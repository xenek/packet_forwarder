// Copyright © 2017 The Things Network. Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package util

import "time"

// TXTimestamp allows to wrap a router.DownlinkMessage.GatewayConfiguration.Timestamp
type TXTimestamp uint32

func (ts TXTimestamp) GetAsDuration() time.Duration {
	return time.Duration(ts) * time.Microsecond
}

func TXTimestampFromDuration(d time.Duration) TXTimestamp {
	return TXTimestamp(d.Nanoseconds() / 1000.0)
}
