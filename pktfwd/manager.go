// Copyright © 2017 The Things Network. Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package pktfwd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/packet_forwarder/util"
	"github.com/TheThingsNetwork/packet_forwarder/wrapper"
	"github.com/pkg/errors"
)

const (
	initUplinkPollingRate   = 100 * time.Microsecond
	stableUplinkPollingRate = 5 * time.Millisecond
	statusRoutineSleepRate  = 15 * time.Second
	gpsUpdateRate           = 5 * time.Millisecond
)

/* Manager struct manages the routines during runtime, once the gateways and network
configuration have been set up. It startes a routine, that it only stopped when the
users wants to close the program or that an error occurs. */
type Manager struct {
	ctx               log.Interface
	conf              util.Config
	netClient         NetworkClient
	statusMgr         StatusManager
	uplinkPollingRate time.Duration
	// Concentrator boot time
	bootTimeSetters     multipleBootTimeSetter
	foundBootTime       bool
	isGPS               bool
	downlinksSendMargin time.Duration
}

func NewManager(ctx log.Interface, conf util.Config, netClient NetworkClient, gpsPath string, runConfig TTNConfig) Manager {
	isGPS := gpsPath != ""
	statusMgr := NewStatusManager(ctx, netClient.FrequencyPlan(), runConfig.GatewayDescription, isGPS)
	bootTimeSetters := NewMultipleBootTimeSetter()
	bootTimeSetters.Add(statusMgr)
	return Manager{
		ctx:             ctx,
		conf:            conf,
		netClient:       netClient,
		statusMgr:       statusMgr,
		bootTimeSetters: bootTimeSetters,
		isGPS:           isGPS,
		// At the beginning, until we get our first uplinks, we keep a high polling rate to the concentrator
		uplinkPollingRate:   initUplinkPollingRate,
		downlinksSendMargin: runConfig.DownlinksSendMargin,
	}
}

func (m *Manager) run() error {
	runStart := time.Now()
	m.ctx.WithField("DateTime", runStart).Info("Starting concentrator...")
	err := wrapper.StartLoRaGateway()
	if err != nil {
		return err
	}

	m.ctx.WithField("DateTime", time.Now()).Info("Concentrator started, packets can now be received and sent")
	err = m.handler(runStart)
	if shutdownErr := m.shutdown(); shutdownErr != nil {
		m.ctx.WithError(shutdownErr).Error("Couldn't stop concentrator gracefully")
	}
	return err
}

func (m *Manager) handler(runStart time.Time) (err error) {
	// First, we'll handle the case when the user wants to end the program
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGABRT)

	// We'll start the routines, and attach them a context
	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var routinesErr = make(chan error)
	go m.startRoutines(bgCtx, routinesErr, runStart)

	// Finally, we'll listen to the different issues
	select {
	case sig := <-c:
		m.ctx.WithField("Signal", sig.String()).Info("Stopping packet forwarder")
	case err = <-routinesErr:
		m.ctx.Error("Program ended after one of the network links failed")
	}

	return err
}

func (m *Manager) findConcentratorBootTime(packets []wrapper.Packet, runStart time.Time) error {
	currentTime := time.Now()
	highestTimestamp := uint32(0)
	for _, p := range packets {
		if p.CountUS > highestTimestamp {
			highestTimestamp = p.CountUS
		}
	}
	if highestTimestamp == 0 {
		return nil
	}

	// Estimated boot time: highest timestamp (closest to current time) substracted to the current time
	highestTimestampDuration := time.Duration(highestTimestamp) * time.Microsecond
	bootTime := currentTime.Add(-highestTimestampDuration)
	if runStart.After(bootTime) || bootTime.After(time.Now()) {
		// Absurd timestamp
		return errors.New("Absurd uptime received by concentrator")
	}
	m.ctx.WithField("BootTime", bootTime).Info("Determined concentrator boot time")
	m.setBootTime(bootTime)
	return nil
}

func (m *Manager) setBootTime(bootTime time.Time) {
	m.bootTimeSetters.SetBootTime(bootTime)
	m.foundBootTime = true
	m.uplinkPollingRate = stableUplinkPollingRate
}

func (m *Manager) uplinkRoutine(bgCtx context.Context, errc chan error, runStart time.Time) {
	m.ctx.Info("Waiting for uplink packets")
	for {
		packets, err := wrapper.Receive()
		if err != nil {
			errc <- errors.Wrap(err, "Uplink packets retrieval error")
			return
		}
		if len(packets) == 0 { // Empty payload => we sleep, then reiterate.
			time.Sleep(m.uplinkPollingRate)
			continue
		}

		m.ctx.WithField("NbPackets", len(packets)).Info("Received uplink packets")
		if !m.foundBootTime {
			// First packets received => find concentrator boot time
			err = m.findConcentratorBootTime(packets, runStart)
			if err != nil {
				m.ctx.WithError(err).Warn("Error when computing concentrator boot time - using packet forwarder run start time")
				m.setBootTime(runStart)
			}
		}

		validPackets, err := wrapUplinkPayload(packets, m.netClient.GatewayID())
		if err != nil {
			continue
		}
		m.statusMgr.HandledRXBatch(len(validPackets), len(packets))
		if len(validPackets) == 0 {
			m.ctx.Warn("Packets received, but with invalid CRC - ignoring")
			time.Sleep(m.uplinkPollingRate)
			continue
		}

		m.ctx.WithField("NbValidPackets", len(validPackets)).Info("Received valid packets - sending them to the back-end")
		m.netClient.SendUplinks(validPackets)

		select {
		case <-bgCtx.Done():
			errc <- nil
			return
		default:
			continue
		}
	}
}

func (m *Manager) gpsRoutine(bgCtx context.Context, errC chan error) {
	m.ctx.Info("Starting GPS update routine")
	for {
		select {
		case <-bgCtx.Done():
			return
		default:
			// The GPS time reference and coordinates are updated at `gpsUpdateRate`
			err := wrapper.UpdateGPSData(m.ctx)
			if err != nil {
				errC <- errors.Wrap(err, "GPS update error")
			}
		}
	}
}

func (m *Manager) downlinkRoutine(bgCtx context.Context) {
	m.ctx.Info("Waiting for downlink messages")
	downlinkQueue := m.netClient.Downlinks()
	dManager := NewDownlinkManager(bgCtx, m.ctx, m.conf, m.statusMgr, m.downlinksSendMargin)
	m.bootTimeSetters.Add(dManager)
	for {
		select {
		case downlink := <-downlinkQueue:
			m.ctx.Info("Scheduling newly-received downlink packet")
			m.statusMgr.ReceivedTX()
			dManager.ScheduleDownlink(downlink)
		case <-bgCtx.Done():
			return
		}
	}
}

func (m *Manager) statusRoutine(bgCtx context.Context, errC chan error) {
	for {
		select {
		case <-time.After(statusRoutineSleepRate):
			rtt, err := m.netClient.Ping()
			if err != nil {
				errC <- errors.Wrap(err, "Network server health check error")
				continue
			}

			status, err := m.statusMgr.GenerateStatus(rtt)
			if err != nil {
				errC <- errors.Wrap(err, "Gateway status computation error")
				return
			}

			err = m.netClient.SendStatus(*status)
			if err != nil {
				errC <- errors.Wrap(err, "Gateway status transmission error")
				return
			}
		case <-bgCtx.Done():
			return
		}
	}
}

func (m *Manager) startRoutines(bgCtx context.Context, err chan error, runTime time.Time) {
	var errC = make(chan error)
	upCtx, upCancel := context.WithCancel(bgCtx)
	downCtx, downCancel := context.WithCancel(bgCtx)
	statsCtx, statsCancel := context.WithCancel(bgCtx)
	gpsCtx, gpsCancel := context.WithCancel(bgCtx)

	go m.uplinkRoutine(upCtx, errC, runTime)
	go m.downlinkRoutine(downCtx)
	go m.statusRoutine(statsCtx, errC)
	if m.isGPS {
		go m.gpsRoutine(gpsCtx, errC)
	}
	select {
	case routineErr := <-errC:
		err <- routineErr
		close(errC)
	case <-bgCtx.Done():
		err <- nil
	}
	upCancel()
	gpsCancel()
	downCancel()
	statsCancel()
}

func (m *Manager) shutdown() error {
	m.netClient.Stop()
	return stopGateway(m.ctx)
}

func stopGateway(ctx log.Interface) error {
	err := wrapper.StopLoRaGateway()
	if err != nil {
		return err
	}

	ctx.Info("Concentrator stopped gracefully")
	return nil
}
