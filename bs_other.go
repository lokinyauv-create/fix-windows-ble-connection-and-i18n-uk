//go:build linux
// +build linux

package main

import (
	"log"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

func connectToPreloadedBaseStation(bs *LighthouseV2, config BaseStationConfiguration, wakeUp bool, attemp int) {
	if attemp > 5 {
		log.Printf("Giving up connecting to base station %s after %d attempts\n", config.Id, attemp)
		return
	}

	parsedMac, err := bluetooth.ParseMAC(config.MacAddress)

	if err != nil {
		log.Printf("Failed to parse mac: %s for lighthouse %s (%+v)\n", config.MacAddress, config.Id, err)
		return
	}

	// adapter.Connect blocks on a BlueZ D-Bus PropertiesChanged signal with no
	// built-in timeout, so a base station that never answers (or a stale/
	// unregistered BlueZ device object) hangs this goroutine forever with no
	// further retries. Bound it so we always fall back to a retry.
	type connectResult struct {
		conn bluetooth.Device
		err  error
	}
	connectDone := make(chan connectResult, 1)
	go func() {
		conn, err := adapter.Connect(bluetooth.Address{
			MACAddress: bluetooth.MACAddress{
				MAC: parsedMac,
			},
		}, bluetooth.ConnectionParams{})
		connectDone <- connectResult{conn, err}
	}()

	var conn bluetooth.Device
	select {
	case res := <-connectDone:
		if res.err != nil {
			log.Printf("Failed to connect to base station: %s %+v", config.Id, res.err)

			if strings.Contains(res.err.Error(), "not found") || strings.Contains(res.err.Error(), "doesn't exist") {
				// BlueZ only exposes a D-Bus object for devices it has seen, so
				// connecting by address to a station it hasn't observed fails
				// until a scan registers it.
				discoverBaseStation(config.MacAddress, 8*time.Second)
			} else {
				time.Sleep(time.Second)
			}

			connectToPreloadedBaseStation(bs, config, wakeUp, attemp+1)
			return
		}
		conn = res.conn
	case <-time.After(10 * time.Second):
		log.Printf("Timed out connecting to base station %s after 10s, retrying (attempt %d)...\n", config.Id, attemp+1)
		time.Sleep(time.Second)
		connectToPreloadedBaseStation(bs, config, wakeUp, attemp+1)
		return
	}

	bs.adapter = adapter

	bs.p = &conn

	log.Printf("Connected to base station: %s, wake up: %+v\n", config.Id, wakeUp)

	bs.FindService()
	bs.ScanCharacteristics()

	if wakeUp {
		bs.SetPowerState(byte(0x01))
	}

	go bs.StartCaching()
}

func (lv *LighthouseV2) StartCaching() {
	if lv.powerStateCharacteristic != nil {

		err := lv.powerStateCharacteristic.EnableNotifications(func(buf []byte) {
			lv.CachedPowerState = int(buf[0])
			WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.power_state", "power_state", int(buf[0])))
			log.Printf("Power state on %s changed: %+v", lv.Id, buf)
		})

		if err != nil {
			log.Printf("Failed to receive notifications on power state, base station firmware probably outdated; lighthouse=%s; err=%+v", lv.Id, err)
			lv.updateAvailable = true
		}
	}

	if lv.modeCharacteristic != nil {
		err := lv.modeCharacteristic.EnableNotifications(func(buf []byte) {
			lv.CachedChannel = int(buf[0])
			WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.channel", "channel", int(buf[0])))

		})

		if err != nil {
			log.Printf("Failed to receive notifications on power state, base station firmware probably outdated; lighthouse=%s; err=%+v", lv.Id, err)
			lv.updateAvailable = true
		}
	}
}

func (lighthouse *LighthouseV2) Write(characteristic *bluetooth.DeviceCharacteristic, value []byte) (int, error) {
	bytes, err := characteristic.WriteWithoutResponse(value)
	return bytes, err
}

func (lv *LighthouseV2) Reconnect() {

	WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.status", "status", "preloaded"))

	lv.identifyCharacteristic = nil
	lv.modeCharacteristic = nil
	lv.powerStateCharacteristic = nil

	log.Println("Reconnecting...")
	parsedMac, err := bluetooth.ParseMAC(lv.mac)

	if err != nil {
		log.Printf("Failed to parse MAC: %+v\n", err)
		return
	}

	conn, err := adapter.Connect(bluetooth.Address{
		MACAddress: bluetooth.MACAddress{
			MAC: parsedMac,
		},
	}, bluetooth.ConnectionParams{})

	if err != nil {
		log.Printf("Failed to reconnect to lighthouse %s: %+v\n", lv.Name, err)
		time.Sleep(time.Second)
		lv.Reconnect()
		return
	}

	lv.p = &conn

	go lv.PostInit(false)
}
