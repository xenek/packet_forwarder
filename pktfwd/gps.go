// Copyright © 2017 The Things Network. Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package pktfwd

import (
	"context"
	"sync"
	"time"

	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/packet_forwarder/wrapper"
	"github.com/TheThingsNetwork/ttn/api/gateway"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/dotpy3/go-gpsd"
	"github.com/pkg/errors"
)

/*
	GPS workflow with the TTN back-end:
	- if gps available, send that in the status message (no need to send it with every uplink)
	- if nothing available or set, get the coordinates from account server and use that in the status message
*/

type GPS interface {
	Start() error
	Stop()
	GetCoordinates() (*gateway.GPSMetadata, error)
	PacketTime(uplink router.UplinkMessage) (time.Time, error)
}

type halGPS struct {
	ctx     log.Interface
	BgCtx   context.Context
	Cancel  context.CancelFunc
	GPSPath string
}

func NewHalGPS(ctx log.Interface, gpsPath string) GPS {
	bgCtx, cancel := context.WithCancel(context.Background())
	return &halGPS{
		ctx:     ctx,
		BgCtx:   bgCtx,
		Cancel:  cancel,
		GPSPath: gpsPath,
	}
}

func (g *halGPS) GetCoordinates() (*gateway.GPSMetadata, error) {
	coord, err := wrapper.GetGPSCoordinates()
	if err != nil {
		return nil, err
	}

	return &gateway.GPSMetadata{
		Altitude:  int32(coord.Altitude),
		Latitude:  float32(coord.Latitude),
		Longitude: float32(coord.Longitude),
	}, nil
}

func (g *halGPS) Start() error {
	err := wrapper.LoRaGPSEnable(g.GPSPath)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-g.BgCtx.Done():
				return
			case <-time.After(gpsUpdateRate):
				// The GPS time reference and coordinates are updated at `gpsUpdateRate`
				err := wrapper.UpdateGPSData(g.ctx)
				if err != nil {
					g.ctx.WithError(err).Warn("GPS update error")
				}
			}
		}
	}()

	return nil
}

func (g *halGPS) Stop() {
	g.Cancel()
	return
}

func (g *halGPS) PacketTime(uplink router.UplinkMessage) (time.Time, error) {
	ts := uplink.GetGatewayMetadata().GetTimestamp()
	return wrapper.GetPacketTime(ts)
}

type gpsdGPS struct {
	sync.Mutex
	ctx                      log.Interface
	GPSDAddress              string
	stop                     context.CancelFunc
	latestReport             *gpsd.TPVReport
	latestReportTime         *time.Time
	concentratorBootTime     *time.Time
	concentratorBootTimeLock sync.Mutex
}

func (g *gpsdGPS) Start() error {
	sess, err := gpsd.Dial(g.GPSDAddress)
	if err != nil {
		return errors.Wrap(err, "Couldn't open GPSD")
	}

	sess.AddFilter("TPV", func(r interface{}) {
		report, ok := r.(*gpsd.TPVReport)
		if !ok {
			return
		}

		g.Lock()
		g.latestReport = report
		currentTime := time.Now()
		g.latestReportTime = &currentTime
		g.Unlock()
	})
	ctx, cancel := context.WithCancel(context.Background())
	sess.Watch(ctx)
	g.stop = cancel

	sess.OnError(func(err error) {
		g.ctx.WithError(err).Error("GPS error")
	})

	return nil
}

func (g *gpsdGPS) Stop() {
	g.stop()
	return
}

func (g *gpsdGPS) GetCoordinates() (*gateway.GPSMetadata, error) {
	g.Lock()
	defer g.Unlock()
	if g.latestReport == nil {
		return nil, errors.New("No gpsd data available")
	}

	return &gateway.GPSMetadata{
		Latitude:  float32(g.latestReport.Lat),
		Longitude: float32(g.latestReport.Lon),
		Altitude:  int32(g.latestReport.Alt),
		Time:      int64(g.latestReport.Time.UnixNano() / 1000),
	}, nil
}

func (g *gpsdGPS) SetBootTime(t time.Time) {
	g.concentratorBootTimeLock.Lock()
	g.concentratorBootTime = &t
	g.concentratorBootTimeLock.Unlock()
}

func (g *gpsdGPS) PacketTime(uplink router.UplinkMessage) (time.Time, error) {
	g.Lock()
	defer g.Unlock()
	g.concentratorBootTimeLock.Lock()
	defer g.concentratorBootTimeLock.Unlock()
	if g.latestReportTime == nil {
		return time.Time{}, errors.New("No GPS data yet")
	}
	if g.concentratorBootTime == nil {
		return time.Time{}, errors.New("Concentrator boot time not computed yet")
	}

	// Difference between GPS time and systime
	sysGPSDiff := g.latestReport.Time.Sub(*g.latestReportTime)
	// Add it to concentrator boot time, to get the real concentrator boot time
	realConcentratorBootTime := g.concentratorBootTime.Add(sysGPSDiff)
	// Add concentrator uptime value of the packet
	return realConcentratorBootTime.Add(time.Duration(uplink.GatewayMetadata.Timestamp) * time.Microsecond), nil
}

func NewGPSDGPS(ctx log.Interface, address string) *gpsdGPS {
	return &gpsdGPS{
		ctx:                      ctx,
		GPSDAddress:              address,
		concentratorBootTimeLock: sync.Mutex{},
	}
}
