// Copyright © 2017 The Things Network. Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package pktfwd

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/TheThingsNetwork/go-account-lib/account"
	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/ttn/api/discovery"
	"github.com/TheThingsNetwork/ttn/api/fields"
	"github.com/TheThingsNetwork/ttn/api/gateway"
	"github.com/TheThingsNetwork/ttn/api/health"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const uplinksBufferSize = 32

type TTNConfig struct {
	ID                  string
	Key                 string
	AuthServer          string
	DiscoveryServer     string
	Router              string
	Version             string
	GatewayDescription  string
	DownlinksSendMargin time.Duration
}

type TTNClient struct {
	currentRouterConn *grpc.ClientConn
	ctx               log.Interface
	uplinkStream      router.UplinkStream
	uplinkMutex       sync.Mutex
	downlinkStream    router.DownlinkStream
	statusStream      router.GatewayStatusStream
	account           *account.Account
	id                string
	connected         bool
	streamsMutex      sync.Mutex
	token             string
	frequencyPlan     string
	// Communication between internal goroutines
	stopDownlinkQueue          chan bool
	stopUplinkQueue            chan bool
	stopMainRouterReconnection chan bool
	downlinkStreamChange       chan bool
	downlinkQueue              chan *router.DownlinkMessage
	uplinkQueue                chan *router.UplinkMessage
}

type NetworkClient interface {
	SendStatus(status gateway.Status) error
	SendUplinks(messages []router.UplinkMessage)
	FrequencyPlan() string
	Downlinks() <-chan *router.DownlinkMessage
	GatewayID() string
	Ping() (time.Duration, error)
	Stop()
}

func (c *TTNClient) GatewayID() string {
	return c.id
}

type RouterHealthCheck struct {
	Conn     *grpc.ClientConn
	Duration time.Duration
	Err      error
}

func connectionHealthCheck(conn *grpc.ClientConn) (time.Duration, error) {
	timeBefore := time.Now()
	ok, err := health.Check(conn)
	if !ok {
		err = errors.New("Health check with the router failed")
	}
	return time.Now().Sub(timeBefore), err
}

func connectToRouter(ctx log.Interface, discoveryClient discovery.Client, router string) (*grpc.ClientConn, error) {
	routerAccess, err := discoveryClient.Get("router", router)
	if err != nil {
		return nil, err
	}

	var announcement = *routerAccess

	ctx.Info("Connecting to router...")
	return announcement.Dial()
}

func (c *TTNClient) FrequencyPlan() string {
	return c.frequencyPlan
}

func reconnectionDelay(tries uint) time.Duration {
	return time.Duration(math.Exp(float64(tries)/2.0)) * time.Second
}

func (c *TTNClient) tryMainRouterReconnection(gw account.Gateway, discoveryClient discovery.Client, gatewayID string) {
	tries := uint(0)
	for {
		select {
		case <-c.stopMainRouterReconnection:
			return
		case <-time.After(reconnectionDelay(tries)):
			break
		}
		c.ctx.Info("Trying to reconnect to main router")
		routerConn, err := connectToRouter(c.ctx, discoveryClient, gw.Router.ID)
		if err != nil {
			c.ctx.WithError(err).Warn("Couldn't connect to the main router")
			tries = tries + 1
			continue
		}

		c.ctx.Info("Connection to main router successful")
		c.uplinkMutex.Lock()
		c.connectToStreams(router.NewRouterClientForGateway(router.NewRouterClient(routerConn), gatewayID, c.token), true)
		c.uplinkMutex.Unlock()
		c.currentRouterConn = routerConn
		c.downlinkStreamChange <- true
		break
	}
}

func (c *TTNClient) Ping() (time.Duration, error) {
	return connectionHealthCheck(c.currentRouterConn)
}

func (c *TTNClient) getLowestLatencyRouter(discoveryClient discovery.Client, fallbackRouters []account.GatewayRouter) (*grpc.ClientConn, error) {
	routerAnnouncements := make([]*discovery.Announcement, 0)
	for _, router := range fallbackRouters {
		routerAnnouncement, err := discoveryClient.Get("router", router.ID)
		if err != nil {
			continue
		}
		routerAnnouncements = append(routerAnnouncements, routerAnnouncement)
	}
	return c.getLowestLatencyRouterFromAnnouncements(discoveryClient, routerAnnouncements)
}

func (c *TTNClient) getLowestLatencyRouterFromAnnouncements(discoveryClient discovery.Client, routerAnnouncements []*discovery.Announcement) (*grpc.ClientConn, error) {
	var routerConn *grpc.ClientConn
	routerHealthChannel := make(chan RouterHealthCheck)
	for _, routerAnnouncement := range routerAnnouncements {
		announcement := routerAnnouncement
		go func() {
			conn, err := announcement.Dial()
			if err != nil {
				routerHealthChannel <- RouterHealthCheck{Err: err}
				return
			}
			duration, err := connectionHealthCheck(conn)
			routerHealthChannel <- RouterHealthCheck{
				Err:      err,
				Duration: duration,
				Conn:     conn,
			}
		}()
	}

	lowestPing := time.Duration(math.MaxInt64)
	routersChecked := 0
	for routerHealth := range routerHealthChannel {
		if routerHealth.Err == nil && routerHealth.Duration < lowestPing {
			if routerConn != nil {
				routerConn.Close()
			}
			routerConn = routerHealth.Conn
		}
		routersChecked++
		if routersChecked == len(routerAnnouncements) {
			break
		}
	}
	if routerConn == nil {
		return nil, errors.New("Packet forwarder couldn't establish a healthy connection with any router")
	}
	c.ctx.Info("Identified the lowest latency router")
	return routerConn, nil
}

func (c *TTNClient) getRouterClient(ctx log.Interface, ttnConfig TTNConfig) (router.RouterClient, error) {
	ctx.WithField("Address", ttnConfig.DiscoveryServer).Info("Connecting to TTN discovery server")
	discoveryClient, err := discovery.NewClient(ttnConfig.DiscoveryServer, &discovery.Announcement{
		ServiceName:    "ttn-packet-forwarder",
		ServiceVersion: ttnConfig.Version,
		Id:             c.id,
	}, func() string { return "" })
	if err != nil {
		return nil, err
	}
	ctx.Info("Connected to discovery server - getting router address")

	defer discoveryClient.Close()

	var routerConn *grpc.ClientConn
	if ttnConfig.Router == "" {
		gw, err := c.account.FindGateway(c.GatewayID())
		if err != nil {
			return nil, err
		}

		routerConn, err = connectToRouter(c.ctx, discoveryClient, gw.Router.ID)
		if err != nil {
			ctx.WithError(err).WithField("RouterID", gw.Router.ID).Warn("Couldn't connect to main router - trying to connect to fallback routers")
			fallbackRouters := gw.FallbackRouters
			if len(fallbackRouters) == 0 {
				ctx.Warn("No fallback routers in memory for this gateway - loading all routers")
				routers, err := discoveryClient.GetAll("router")
				if err != nil {
					ctx.WithError(err).Error("Couldn't retrieve routers")
					return nil, err
				}
				routerConn, err = c.getLowestLatencyRouterFromAnnouncements(discoveryClient, routers)
				if err != nil {
					return nil, err
				}
			} else {
				routerConn, err = c.getLowestLatencyRouter(discoveryClient, fallbackRouters)
				if err != nil {
					return nil, err
				}
			}
			defer func() {
				// Wait for the function to be finished, to protect `c.currentRouterConn`
				go c.tryMainRouterReconnection(gw, discoveryClient, c.GatewayID())
			}()
		}
	} else {
		routerConn, err = connectToRouter(ctx, discoveryClient, ttnConfig.Router)
		if err != nil {
			return nil, err
		}
		ctx.Info("Connected to router")
	}

	c.currentRouterConn = routerConn
	return router.NewRouterClient(routerConn), nil
}

func (c *TTNClient) Downlinks() <-chan *router.DownlinkMessage {
	return c.downlinkQueue
}

func (c *TTNClient) queueUplinks() {
	for {
		select {
		case <-c.stopUplinkQueue:
			c.ctx.Info("Closing uplinks queue")
			close(c.uplinkQueue)
			return
		case uplink := <-c.uplinkQueue:
			ctx := c.ctx.WithFields(fields.Get(uplink))
			if err := c.uplinkStream.Send(uplink); err != nil {
				ctx.WithError(err).Warn("Uplink message transmission to the back-end failed.")
			} else {
				ctx.Info("Uplink message transmission successful.")
			}
		}
	}
}

func (c *TTNClient) queueDownlinks() {
	c.ctx.Info("Downlinks queuing routine started")
	downlinkStreamChannel := c.downlinkStream.Channel()
	for {
		select {
		case <-c.stopDownlinkQueue:
			c.ctx.Info("Closing downlinks queue")
			close(c.downlinkQueue)
			return
		case downlink := <-downlinkStreamChannel:
			c.ctx.Info("Received downlink packet")
			c.downlinkQueue <- downlink
		case <-c.downlinkStreamChange:
			downlinkStreamChannel = c.downlinkStream.Channel()
		}
	}
}

func (c *TTNClient) fetchAccountServerInfo(gatewayID string) error {
	gw, err := c.account.FindGateway(gatewayID)
	if err != nil {
		return errors.Wrap(err, "Account server error")
	}
	c.token = gw.Token.AccessToken
	c.frequencyPlan = gw.FrequencyPlan
	return nil
}

func CreateNetworkClient(ctx log.Interface, ttnConfig TTNConfig) (NetworkClient, error) {
	var client = &TTNClient{
		account:           account.NewWithKey(ttnConfig.AuthServer, ttnConfig.Key),
		ctx:               ctx,
		id:                ttnConfig.ID,
		downlinkQueue:     make(chan *router.DownlinkMessage),
		uplinkQueue:       make(chan *router.UplinkMessage, uplinksBufferSize),
		stopDownlinkQueue: make(chan bool),
		stopUplinkQueue:   make(chan bool),
	}

	err := client.fetchAccountServerInfo(ttnConfig.ID)
	if err != nil {
		return nil, err
	}

	// Getting a RouterClient object
	routerClient, err := client.getRouterClient(ctx, ttnConfig)
	if err != nil {
		return nil, err
	}

	client.connectToStreams(router.NewRouterClientForGateway(routerClient, ttnConfig.ID, client.token), false)

	go client.queueDownlinks()
	go client.queueUplinks()

	return client, nil
}

func (c *TTNClient) connectToStreams(routerClient router.RouterClientForGateway, force bool) {
	c.streamsMutex.Lock()
	defer c.streamsMutex.Unlock()
	if c.connected {
		if !force {
			// If the client is already connected and the new routerClient doesn't want to force
			// connection change, the function is dropped
			return
		}
		c.disconnectOfStreams()
	}
	c.uplinkStream = router.NewMonitoredUplinkStream(routerClient)
	c.downlinkStream = router.NewMonitoredDownlinkStream(routerClient)
	c.statusStream = router.NewMonitoredGatewayStatusStream(routerClient)
	c.connected = true
}

func (c *TTNClient) disconnectOfStreams() {
	c.uplinkStream.Close()
	c.downlinkStream.Close()
	c.statusStream.Close()
	c.connected = false
}

func (c *TTNClient) SendUplinks(messages []router.UplinkMessage) {
	for _, message := range messages {
		c.uplinkQueue <- &message
	}
}

func (c *TTNClient) SendStatus(status gateway.Status) error {
	var uptimeString string
	status.Region = c.frequencyPlan
	uptimeDuration, err := time.ParseDuration(fmt.Sprintf("%dus", status.GetTimestamp()))
	if err == nil {
		uptimeString = uptimeDuration.String()
	} else {
		uptimeString = fmt.Sprintf("%fs", float32(status.GetTimestamp())/1000000.0)
	}
	c.ctx.WithFields(log.Fields{
		"TXPacketsReceived": status.GetTxIn(),
		"TXPacketsValid":    status.GetTxOk(),
		"RXPacketsReceived": status.GetRxIn(),
		"RXPacketsValid":    status.GetRxOk(),
		"FrequencyPlan":     status.GetRegion(),
		"Uptime":            uptimeString,
		"Load1":             status.GetOs().GetLoad_1(),
		"Load5":             status.GetOs().GetLoad_5(),
		"Load15":            status.GetOs().GetLoad_15(),
		"CpuPercentage":     status.GetOs().GetCpuPercentage(),
		"MemoryPercentage":  status.GetOs().GetMemoryPercentage(),
	}).Info("Sending status to the network server")
	err = c.statusStream.Send(&status)
	if err != nil {
		return errors.Wrap(err, "Status stream error")
	}
	return nil
}

// Stop a running network client
func (c *TTNClient) Stop() {
	c.stopDownlinkQueue <- true
	c.stopUplinkQueue <- true
	select {
	case c.stopMainRouterReconnection <- true:
		break
	default:
		break
	}
}
