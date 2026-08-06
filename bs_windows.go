//go:build windows
// +build windows

package main

import (
	"log"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

func (lighthouse *LighthouseV2) Write(characteristic *bluetooth.DeviceCharacteristic, value []byte) (int, error) {
	bytes, err := characteristic.Write(value)
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

func (lv *LighthouseV2) StartCaching() {
	if lv.powerStateCharacteristic != nil {

		err := lv.powerStateCharacteristic.EnableNotificationsWithMode(bluetooth.NotificationModeNotify, func(buf []byte) {
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
		err := lv.modeCharacteristic.EnableNotificationsWithMode(bluetooth.NotificationModeNotify, func(buf []byte) {
			lv.CachedChannel = int(buf[0])
			WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.channel", "channel", int(buf[0])))

		})

		if err != nil {
			log.Printf("Failed to receive notifications on power state, base station firmware probably outdated; lighthouse=%s; err=%+v", lv.Id, err)
			lv.updateAvailable = true
		}
	}
}

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

	// adapter.Connect has no built-in timeout on Windows either, so a base
	// station that never answers the WinRT connect request would otherwise
	// hang this goroutine forever with no further retries.
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

			if strings.Contains(res.err.Error(), "not found") {
				// Windows won't connect by address to a device it hasn't seen
				// recently, even one sitting right there advertising. A scan
				// makes it resolvable again.
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
	bs.mac = config.MacAddress

	log.Printf("Connected to base station: %s, wake up: %+v; discovering GATT services...\n", config.Id, wakeUp)

	// Discover the service inline instead of going through FindService, which
	// kicks off a Reconnect() on every failure and turns a single unresponsive
	// station into a storm of overlapping connection attempts.
	// GetGattServicesWithCacheModeAsync also has no built-in timeout on Windows
	// and hangs indefinitely if the device never answers, so bound it here.
	discoveryDone := make(chan bool, 1)
	go func() {
		services, err := conn.DiscoverServices(nil)
		if err != nil {
			log.Printf("Failed to discover services on %s: %+v\n", config.Id, err)
			discoveryDone <- false
			return
		}

		var foundService *bluetooth.DeviceService
		for i := range services {
			service := &services[i]
			if strings.ToUpper(service.UUID().String()) == LIGHTHOUSE_SERVICE_UUID {
				log.Printf("Found lighthouse service on base station %s\n", config.Id)
				foundService = service
				break
			}
		}

		if foundService == nil {
			log.Printf("Lighthouse service not found on base station %s\n", config.Id)
			discoveryDone <- false
			return
		}

		bs.service = foundService
		discoveryDone <- bs.ScanCharacteristics()
	}()

	select {
	case ok := <-discoveryDone:
		if !ok {
			log.Printf("Discovery failed on %s, retrying (attempt %d)...\n", config.Id, attemp+1)
			conn.Disconnect()
			time.Sleep(time.Second)
			connectToPreloadedBaseStation(bs, config, wakeUp, attemp+1)
			return
		}
		log.Printf("Finished discovering GATT services/characteristics on %s\n", config.Id)
	case <-time.After(8 * time.Second):
		log.Printf("Timed out discovering GATT services on %s after 8s, disconnecting and retrying (attempt %d)...\n", config.Id, attemp+1)
		conn.Disconnect()
		time.Sleep(time.Second)
		connectToPreloadedBaseStation(bs, config, wakeUp, attemp+1)
		return
	}

	if wakeUp {
		bs.SetPowerState(byte(0x01))
	}

	go bs.StartCaching()
}
