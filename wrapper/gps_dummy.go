// +build dummy

package wrapper

import (
	"time"

	"github.com/TheThingsNetwork/go-utils/log"
)

func LoRaGPSEnable(TTYPath string) error {
	return nil
}

func GetGPSCoordinates() (GPSCoordinates, error) {
	return GPSCoordinates{}, nil
}

func UpdateGPSData(ctx log.Interface) error {
	return nil
}

func GetPacketTime(ts uint32) (time.Time, error) {
	return time.Now().Add(time.Duration(-30) * time.Second), nil
}
