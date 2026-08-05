package main

import (
	"log"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	BS_POWERSTATE_SLEEP    = 0
	BS_POWERSTATE_STAND_BY = 2
	BS_POWERSTATE_AWAKE    = 9
	BS_POWERSTATE_AWAKE_2  = 11 // Some weird things is happening here
)

var LIGHTHOUSE_SERVICE_UUID = "00001523-1212-EFDE-1523-785FEABCD124"

var (
	LIGHTHOUSE_CHARACTERISTIC_MODE      = "00001524-1212-EFDE-1523-785FEABCD124"
	LIGHTHOUSE_CHARACTERISTIC_IDENTITFY = "00008421-1212-EFDE-1523-785FEABCD124"
	LIGHTHOUSE_CHARACTERISTIC_POWER     = "00001525-1212-EFDE-1523-785FEABCD124"
)

type BaseStation interface {
	ScanCharacteristics() bool
	GetChannel() int
	SetChannel(channel int)
	GetPowerState() int
	SetPowerState(state byte)
	Identitfy()
	GetName() string
	SetName(string)
	GetVersion() int
	Disconnect()
	GetStatus() string
	GetId() string
	GetMAC() string
	IsOutdated() bool
}

type LighthouseV2 struct {
	modeCharacteristic       *bluetooth.DeviceCharacteristic
	identifyCharacteristic   *bluetooth.DeviceCharacteristic
	powerStateCharacteristic *bluetooth.DeviceCharacteristic
	p                        *bluetooth.Device
	adapter                  *bluetooth.Adapter
	service                  *bluetooth.DeviceService
	Name                     string
	Id                       string
	CachedPowerState         int
	CachedChannel            int
	ValidLighthouse          bool
	Status                   string
	mac                      string
	updateAvailable          bool
}

// connectingBaseStations holds the ids of the stations with a connection
// attempt currently in flight, so a second pass can tell the difference between
// "not connected yet" and "nobody is working on it". A base station accepts one
// connection at a time, so two chains racing on the same one make GATT
// discovery come back empty for both.
var connectingBaseStations sync.Map

func BaseStationIsConnecting(id string) bool {
	_, connecting := connectingBaseStations.Load(id)
	return connecting
}

// scanMutex keeps discovery passes from overlapping each other. Scanning while
// another station is enumerating its GATT services is what made discovery hang
// on Windows, so a scan is kept as short as it can be and never runs twice at
// once.
var scanMutex sync.Mutex

// discoverBaseStation scans until the given address shows up, so the OS can
// resolve it. Both backends refuse to connect by address to a device they
// haven't observed recently - Windows fails outright with "device with the
// given address was not found", BlueZ with a missing D-Bus object - and a
// station that was simply idle long enough falls into that state even though
// it is powered on and advertising normally.
func discoverBaseStation(mac string, timeout time.Duration) bool {
	scanMutex.Lock()
	defer scanMutex.Unlock()

	log.Printf("Scanning so the adapter can resolve %s...\n", mac)

	found := make(chan struct{})
	var once sync.Once

	go adapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		if strings.EqualFold(sr.Address.String(), mac) {
			once.Do(func() { close(found) })
		}
	})

	seen := false
	select {
	case <-found:
		seen = true
	case <-time.After(timeout):
	}

	adapter.StopScan()

	// Let the radio settle before the connect attempt - overlapping the two is
	// exactly what breaks service discovery.
	time.Sleep(500 * time.Millisecond)

	log.Printf("Scan for %s finished, found: %v\n", mac, seen)
	return seen
}

func PreloadBaseStation(config BaseStationConfiguration, wakeUp bool) BaseStation {
	lh := &LighthouseV2{
		Name:             config.Name,
		Id:               config.Id,
		Status:           "preloaded",
		CachedPowerState: -1,
		CachedChannel:    config.LastChannel,
		ValidLighthouse:  false,
	}

	connectingBaseStations.Store(config.Id, struct{}{})
	go func() {
		defer connectingBaseStations.Delete(config.Id)
		connectToPreloadedBaseStation(lh, config, wakeUp, 0)
	}()

	return lh
}

func (lv *LighthouseV2) FindService() bool {
	services, err := lv.p.DiscoverServices(nil)
	if err != nil {
		lv.Reconnect()
		log.Printf("Failed to find service: %+v\n", err)
		return false
	}

	var foundService *bluetooth.DeviceService
	for i := range services {
		service := &services[i]
		uuid := strings.ToUpper(service.UUID().String())
		if uuid == LIGHTHOUSE_SERVICE_UUID {
			log.Printf("Found lighthouse service on base station %s\n", lv.Id)
			foundService = service
			break
		}
	}

	lv.service = foundService

	return true
}

func InitBaseStation(connection *bluetooth.Device, adapter *bluetooth.Adapter, name string) (BaseStation, bool) {
	bs := &LighthouseV2{
		p:                        connection,
		adapter:                  adapter,
		service:                  nil,
		modeCharacteristic:       nil,
		identifyCharacteristic:   nil,
		powerStateCharacteristic: nil,
		Name:                     name,
		CachedPowerState:         -1,
		CachedChannel:            -1,
		Id:                       name,
		Status:                   "scanning",
	}

	go bs.PostInit(false)
	return bs, true
}

func (lighthouse *LighthouseV2) PostInit(wakeUp bool) {
	statusChannel := make(chan bool, 1)
	go lighthouse.InitStack(statusChannel)

	go func() {
		select {
		case status := <-statusChannel:
			if !status {
				lighthouse.Disconnect()
				go lighthouse.Reconnect()
				return
			}

			if wakeUp {
				lighthouse.SetPowerState(byte(0x01))
			}
		case <-time.After(time.Second * 3):
			go lighthouse.Reconnect()
			return
		}
	}()
}

func (lighthouse *LighthouseV2) InitStack(status chan bool) {

	if !lighthouse.FindService() {
		status <- false //Failed
	}

	//Adding deadline
	lighthouse.ValidLighthouse = lighthouse.ScanCharacteristics()

	if !lighthouse.ValidLighthouse {
		status <- false // Failed as well
	}
	go lighthouse.StartCaching()

	lighthouse.CachedPowerState = lighthouse.readPowerState()

	WEBSOCKET_BROADCAST.Broadcast(preparePacket("lighthouse.found", JsonBaseStation{
		Name:         lighthouse.GetName(),
		Channel:      lighthouse.GetChannel(),
		PowerState:   lighthouse.GetPowerState(),
		LastUpdated:  nil,
		Version:      lighthouse.GetVersion(),
		Status:       lighthouse.GetStatus(),
		ManagedFlags: 6,
		Id:           lighthouse.GetId(),
	}))
	//Everything is fine, telling in status that we don't need to wait anymore

	status <- true
}

func (lv *LighthouseV2) ScanCharacteristics() bool {
	if lv.service == nil {
		log.Printf("Lighthouse service on base station %s not found, reconnecting...\n", lv.Name)
		lv.Reconnect()
		return false
	}

	characteristics, err := lv.service.DiscoverCharacteristics(nil)
	if err != nil {
		lv.p.Disconnect()
		return false
	}

	for i := range characteristics {
		char := &characteristics[i]
		uuid := char.UUID().String()

		switch strings.ToUpper(uuid) {
		case LIGHTHOUSE_CHARACTERISTIC_MODE:
			lv.modeCharacteristic = char
		case LIGHTHOUSE_CHARACTERISTIC_IDENTITFY:
			lv.identifyCharacteristic = char
		case LIGHTHOUSE_CHARACTERISTIC_POWER:
			lv.powerStateCharacteristic = char
		}
	}

	log.Printf("Finished finding characteristics on base station %s", lv.Name)
	log.Printf("Mode: %v, Power: %v, ID: %v",
		lv.modeCharacteristic != nil,
		lv.powerStateCharacteristic != nil,
		lv.identifyCharacteristic != nil)

	lv.ValidLighthouse = lv.modeCharacteristic != nil && lv.powerStateCharacteristic != nil

	if lv.ValidLighthouse {
		log.Println("Sending ready...")
		lv.Status = "ready"
		WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.status", "status", "ready"))
		WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.channel", "channel", lv.readChannel()))
		WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.power_state", "power_state", lv.readPowerState()))
	}

	lv.mac = lv.p.Address.String()
	go lv.StartCaching()
	return lv.ValidLighthouse
}

func (lv *LighthouseV2) GetChannel() int {

	return lv.CachedChannel
}

func (lv *LighthouseV2) SetChannel(channel int) {

	if !lv.ValidLighthouse {
		return
	}

	if lv.modeCharacteristic == nil {
		log.Printf("ModeCharacteristic on %s was nil, rescanning characteristics and trying again...\n", lv.Name)
		lv.ScanCharacteristics()
		lv.SetChannel(channel)
		return
	}

	_, err := lv.Write(lv.modeCharacteristic, []byte{byte(channel)})

	if err != nil {
		log.Printf("Failed to write bytes on lighthouse, reconnecting...; lighthouse=%s, err=%+v;\n", lv.Id, err)

		lv.Reconnect()
	}

	lv.CachedChannel = channel
}

func (lv *LighthouseV2) GetPowerState() int {
	return lv.CachedPowerState
}

func (lv *LighthouseV2) SetPowerState(state byte) {

	if !lv.ValidLighthouse {
		return
	}

	if lv.powerStateCharacteristic == nil {
		log.Printf("PowerStateCharacteristic on %s was nil, rescanning characteristics and trying again...\n", lv.Name)
		lv.ScanCharacteristics()
		lv.SetPowerState(state)
		return
	}

	// _, err := lv.powerStateCharacteristic.Write([]byte{state})
	_, err := lv.Write(lv.powerStateCharacteristic, []byte{state})

	if err != nil {
		log.Printf("Failed to write bytes on lighthouse, reconnecting...; lighthouse=%s, err=%+v;\n", lv.Id, err)

		lv.Reconnect()
		return
	}

	// Base stations that support neither reading nor notifying on the power
	// characteristic would otherwise stay at -1 forever, so track what we just
	// asked for - it is the only power state information available on them.
	switch state {
	case 0x00:
		lv.CachedPowerState = BS_POWERSTATE_SLEEP
	case 0x01:
		lv.CachedPowerState = BS_POWERSTATE_AWAKE
	case 0x02:
		lv.CachedPowerState = BS_POWERSTATE_STAND_BY
	}

	WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.power_state", "power_state", lv.CachedPowerState))
}

func (lv *LighthouseV2) Identitfy() {

	if !lv.ValidLighthouse {
		return
	}

	if lv.identifyCharacteristic == nil {
		log.Printf("IdentifyCharacteristic on %s was nil, rescanning characteristics and trying again...\n", lv.Name)
		lv.ScanCharacteristics()
		lv.Identitfy()
		return
	}

	// _, err := lv.identifyCharacteristic.Write([]byte{0x01})
	_, err := lv.Write(lv.identifyCharacteristic, []byte{0x01})

	if err != nil {
		log.Printf("Failed to write bytes on lighthouse, reconnecting...; lighthouse=%s, err=%+v;\n", lv.Id, err)

		lv.Reconnect()
	}
}

func (lv *LighthouseV2) GetName() string {

	return lv.Name
}

func (lv *LighthouseV2) Disconnect() {

	if lv.p == nil {
		return
	}

	device := lv.p

	// Drop every handle to the link before disconnecting. Leaving them in place
	// meant the app still believed it was connected: writes went to a dead GATT
	// session and silently "succeeded" on Windows, so power commands did nothing
	// while the UI happily reported them as applied. It also let Disconnect run
	// twice on the same released WinRT session, which crashes the process.
	lv.p = nil
	lv.service = nil
	lv.identifyCharacteristic = nil
	lv.modeCharacteristic = nil
	lv.powerStateCharacteristic = nil
	lv.ValidLighthouse = false
	lv.Status = "preloaded"
	WEBSOCKET_BROADCAST.Broadcast(prepareIdWithFieldPacket(lv.Id, "lighthouse.update.status", "status", "preloaded"))

	err := device.Disconnect()

	if err != nil {
		log.Println("Failed to disconnect.")
		log.Println(err)
		return
	}
	log.Println("Device has been disconnected")
}

func (lv *LighthouseV2) GetVersion() int {
	return 2 // stands for 2.0
}

func (lv *LighthouseV2) GetId() string {
	return lv.Id
}

func (lv *LighthouseV2) GetMAC() string {
	if lv.p != nil {
		return lv.p.Address.String()
	}

	return ""
}

func (lv *LighthouseV2) GetStatus() string {
	return lv.Status
}

func (lv *LighthouseV2) SetName(name string) {
	lv.Name = name
}

func (lv *LighthouseV2) IsOutdated() bool {
	return lv.updateAvailable
}

func (lv *LighthouseV2) readPowerState() int {
	if lv.powerStateCharacteristic == nil {
		return -1
	}

	var data []byte = make([]byte, 1)
	_, err := lv.powerStateCharacteristic.Read(data)

	if err != nil {
		// On Windows the power characteristic often doesn't advertise the read
		// property, so Read fails with "read not supported". That is not a
		// broken connection - reconnecting here tears down a working session,
		// nils the characteristics and loops forever. The power state still
		// arrives through the notifications set up in StartCaching.
		log.Printf("Failed to read state on %s: %+v\n", lv.Id, err)
		return lv.CachedPowerState
	}

	lv.CachedPowerState = int(data[0])
	return int(data[0])
}

func (lv *LighthouseV2) readChannel() int {
	if lv.modeCharacteristic == nil {
		return -1
	}

	var data []byte = make([]byte, 1)
	_, err := lv.modeCharacteristic.Read(data)

	if err != nil {
		// Same reasoning as readPowerState: a failed read is not a reason to
		// drop an otherwise healthy connection.
		log.Printf("Failed to read channel on %s: %+v\n", lv.Id, err)
		return lv.CachedChannel
	}

	lv.CachedChannel = int(data[0])

	if config != nil && config.KnownBaseStations[lv.Id] != nil && config.KnownBaseStations[lv.Id].LastChannel != lv.CachedChannel {
		config.KnownBaseStations[lv.Id].LastChannel = lv.CachedChannel
		config.Save()
	}
	return int(data[0])
}
