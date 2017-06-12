// Copyright © 2017 The Things Network. Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package pktfwd

import (
	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/packet_forwarder/util"
	"github.com/pkg/errors"
)

// Init initiates the configuration, the network connection, and handles the manager
func Run(ctx log.Interface, conf util.Config, ttnConfig TTNConfig, gpsPath string) error {
	networkCli, err := CreateNetworkClient(ctx, ttnConfig)
	if err != nil {
		return errors.Wrap(err, "Network configuration failure")
	}

	if err := configureBoard(ctx, conf); err != nil {
		return errors.Wrap(err, "Board configuration failure")
	}

	return NewManager(ctx, conf, networkCli, ttnConfig).run()
}
