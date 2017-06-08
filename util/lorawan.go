// Copyright © 2017 The Things Network. Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package util

import (
	"errors"

	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/TheThingsNetwork/ttn/core/types"
	"github.com/brocaar/lorawan"
)

func GetDevAddr(uplink *router.UplinkMessage) (types.DevAddr, error) {
	var devAddr [4]byte
	if uplink == nil {
		return devAddr, errors.New("Invalid uplink")
	}

	var phyPayload lorawan.PHYPayload
	err := phyPayload.UnmarshalBinary(uplink.Payload)
	if err != nil {
		return devAddr, err
	}

	macPayload, ok := phyPayload.MACPayload.(*lorawan.MACPayload)
	if !ok {
		return devAddr, errors.New("The uplink doesn't contain a MAC payload")
	}

	devAddr = types.DevAddr(macPayload.FHDR.DevAddr)
	return devAddr, nil
}
