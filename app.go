package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"path"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	_ "embed"

	"github.com/gen2brain/beeep"
	cmap "github.com/orcaman/concurrent-map/v2"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter
var WAKE_UP_CHANNEL = make(chan interface{})
var knownBaseStations = cmap.New[*BaseStation]()

type JsonBaseStation struct {
	Name         string     `json:"name"`
	Channel      int        `json:"channel"`
	PowerState   int        `json:"power_state"`
	LastUpdated  *time.Time `json:"last_updated"`
	Version      int        `json:"version"`
	Status       string     `json:"status"`
	ManagedFlags int        `json:"managed_flags"`
	Id           string     `json:"id"`
}

//go:embed VERSION
var BINARY_VERSION string

// App struct
type App struct {
	ctx                   context.Context
	bluetoothInitFinished bool
	windowHidden          atomic.Bool
	reconnecting          atomic.Bool
}

func NewApp() *App {
	return &App{
		bluetoothInitFinished: false,
	}
}

func (a *App) UpdateConfigValue(name string, value interface{}) {
	config.UpdateValue(name, value)
}
func (a *App) startup(ctx context.Context) {

	// Always keep a log file, and deliberately never close it. This used to be
	// skipped entirely for DEBUG builds and, worse, the handle was closed as
	// soon as startup returned - so log.txt only ever held the first few lines
	// and a problem that showed up later left no trace at all. That matters
	// most when SteamVR auto-launches us, because then there is no console to
	// read either.
	if f, err := os.OpenFile(path.Join(GetConfigFolder(), "log.txt"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666); err != nil {
		log.Printf("Failed to open log file, logging to stdout only: %v\n", err)
	} else {
		// File first: MultiWriter stops at the first writer that errors, and a
		// GUI process launched without a console (SteamVR does exactly that)
		// has no usable stdout - putting it first swallowed the whole log.
		log.SetOutput(io.MultiWriter(f, os.Stdout))
	}

	log.Printf("Version flags: %s\n", VERSION_FLAGS)

	a.ctx = ctx

	go func() {
		for {
			<-WAKE_UP_CHANNEL

			running, _ := isProcRunning("vrserver.exe")

			if !running && config.IsSteamVRManaged {
				a.ShowFromTray()
			}

			WEBSOCKET_BROADCAST.Broadcast(preparePacket("steamvr.status", map[string]interface{}{
				"status": running,
			}))
		}
	}()

	config = GetConfiguration()

	// Only bring the adapter up here. Starting an advertisement scan at this
	// point (as InitBluetooth does) makes the radio scan while
	// preloadBaseStations is enumerating GATT services on the known base
	// stations, and on Windows that makes service discovery hang forever.
	// Scanning is started on demand instead - see StartScanFor10Seconds.
	a.EnableBluetooth()

	go a.preloadBaseStations()

	if config.IsSteamVRManaged && runtime.GOOS == "windows" {
		if running, _ := isProcRunning("vrserver.exe"); running {
			// A session is already live, so keep the links and just get out of
			// the way - HideToTray would hand the stations back mid-session.
			a.windowHidden.Store(true)
			wruntime.WindowHide(a.ctx)
		}
	}

	go initializeSystray(a)
	go StartHttp()

	a.watchForTermination()
	a.releaseWhenIdle()
}

// disconnectAllBaseStations drops every live BLE link we hold. A base station
// only accepts one connection at a time, so a link we leave open keeps it
// unusable from any other machine. On Linux this matters even more: BlueZ owns
// the connection at the daemon level, so it survives the process exiting and
// the station stays claimed until something explicitly disconnects it.
func disconnectAllBaseStations() {
	for name, bs := range knownBaseStations.Items() {
		if bs == nil {
			continue
		}

		log.Printf("Disconnecting from base station %s before exit\n", name)
		(*bs).Disconnect()
	}
}

// anyBaseStationConnected reports whether we currently hold at least one link.
func anyBaseStationConnected() bool {
	for _, bs := range knownBaseStations.Items() {
		if bs != nil && (*bs).GetStatus() == "ready" {
			return true
		}
	}
	return false
}

// HideToTray hides the window and lets go of the base stations. Sitting in the
// tray holding them would keep them unusable from any other machine, and we
// don't need the links until the window comes back or a VR session starts.
func (a *App) HideToTray() {
	a.windowHidden.Store(true)
	wruntime.WindowHide(a.ctx)

	if running, _ := isProcRunning("vrserver.exe"); running {
		// A session is live - the automation still needs the links.
		return
	}

	log.Println("Hidden to tray with no VR session - releasing base stations")
	disconnectAllBaseStations()
}

// ShowFromTray brings the window back and re-establishes any link we dropped
// while idling in the tray.
func (a *App) ShowFromTray() {
	a.windowHidden.Store(false)
	wruntime.WindowShow(a.ctx)
	a.ReconnectBaseStations()
}

// ReconnectBaseStations re-runs the preload pass, rebuilding links we released.
// It is a no-op while a previous pass is still in flight or everything is
// already connected.
func (a *App) ReconnectBaseStations() {
	if anyBaseStationConnected() {
		return
	}

	if !a.reconnecting.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer a.reconnecting.Store(false)

		log.Println("Reconnecting to base stations...")
		a.preloadBaseStations()

		// preloadBaseStations only kicks off the connection goroutines, so give
		// them a moment before another caller is allowed to retry.
		time.Sleep(10 * time.Second)
	}()
}

// releaseWhenIdle drops the BLE links whenever the app is sitting in the tray
// with no VR session running - e.g. after SteamVR exits and the stations have
// been put to sleep. Without this the app would keep them claimed for as long
// as it stays in the tray.
func (a *App) releaseWhenIdle() {
	go func() {
		for {
			time.Sleep(15 * time.Second)

			if !a.windowHidden.Load() {
				continue
			}

			if running, _ := isProcRunning("vrserver.exe"); running {
				continue
			}

			if !anyBaseStationConnected() {
				continue
			}

			log.Println("Idle in tray with no VR session - releasing base stations")
			disconnectAllBaseStations()
		}
	}()
}

// watchForTermination releases the base stations when the process is asked to
// quit from outside the UI (Ctrl+C, `kill`, a logout). SIGKILL can't be caught,
// so a `kill -9` still leaves the stations claimed on Linux.
func (a *App) watchForTermination() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-signals
		log.Printf("Received %v, releasing base stations before exit\n", sig)
		disconnectAllBaseStations()
		os.Exit(0)
	}()
}

// shutdown is wired to Wails' OnShutdown so closing the window releases the
// base stations. Without it the app exits still holding them.
func (a *App) shutdown(ctx context.Context) {
	log.Println("Shutting down, releasing base stations...")
	disconnectAllBaseStations()
}

func (a *App) CreateGroup(name string, baseStations []string) string {
	if config.Groups[name] != nil {
		return "error: Already exists"
	}
	id, group := config.CreateGroup(name)

	for _, v := range baseStations {
		config.Groups[id].BaseStationIDs = append(config.Groups[id].BaseStationIDs, v)
	}

	config.Save()

	WEBSOCKET_BROADCAST.Broadcast(map[string]interface{}{
		"event": "group.create",
		"data":  group,
		"id":    id,
	})
	return id
}

func (a *App) AddBaseStationToGroup(id string, station string) string {
	if config.Groups[id] == nil {
		return "error: Unknown group"
	}

	bs, found := knownBaseStations.Get(station)

	if !found {
		return "error: Unknown base station"
	}

	baseStation := *bs

	if slices.Contains(config.Groups[id].BaseStationIDs, baseStation.GetId()) {
		return "error: already in collection"
	}

	config.UpdateGroupValue(id, "base_stations", append(config.Groups[id].BaseStationIDs, baseStation.GetId()))
	WEBSOCKET_BROADCAST.Broadcast(preparePacket("groups.lighthouses.added", map[string]interface{}{
		"id":    baseStation.GetId(),
		"group": id,
	}))

	return "ok"
}

func (a *App) RenameGroup(id string, newName string) string {

	if config.Groups[id] == nil {
		return "error: Unknown group"
	}

	config.Groups[id].Name = newName

	WEBSOCKET_BROADCAST.Broadcast(preparePacket("group.rename", map[string]interface{}{
		"id":   id,
		"name": newName,
	}))

	config.Save()

	return "ok"
}

func (a *App) RemoveGroup(id string) {

	if config.Groups[id] == nil {
		return
	}

	delete(config.Groups, id)
	WEBSOCKET_BROADCAST.Broadcast(preparePacket("group.delete", map[string]interface{}{
		"id": id,
	}))

	config.Save()

}

func (a *App) UpdateGroupManagedFlags(id string, managed_flags int) string {

	log.Println(config.Groups)
	if config.Groups[id] == nil {
		return "error: Unknown group"
	}

	config.Groups[id].ManagedFlags = managed_flags
	config.Save()

	WEBSOCKET_BROADCAST.Broadcast(preparePacket("group.update.flags", map[string]interface{}{
		"id":    id,
		"flags": managed_flags,
	}))

	return "ok"
}

func (a *App) preloadBaseStations() {

	steamVrRunning, _ := isProcRunning("vrserver.exe")
	for name, baseStation := range config.KnownBaseStations {
		// Don't start a second attempt on a station that is already connected
		// or already being connected to. A power command arriving while the
		// startup pass is still running used to trigger a whole second pass
		// through ReconnectBaseStations, and the two chains then fought over
		// the station's single BLE link: one got the characteristics, the other
		// got an empty list, and both churned until they gave up.
		if existing, ok := knownBaseStations.Get(name); ok && existing != nil && (*existing).GetStatus() == "ready" {
			continue
		}

		if BaseStationIsConnecting(baseStation.Id) {
			continue
		}

		log.Printf("Preload base station: %s %+v\n", name, baseStation)
		preloadedBaseStation := PreloadBaseStation(*baseStation, steamVrRunning && ((baseStation.ManagedFlags&2) > 0))
		knownBaseStations.Set(name, &preloadedBaseStation)
	}
}

func (a *App) ForgetBaseStation(name string) {
	station, found := knownBaseStations.Get(name)

	if !found {
		return
	}

	bs := *station
	bs.Disconnect()

	config.ForgetBaseStation(name)

	knownBaseStations.Remove(name)
}

func (a *App) Notify(title string, text string) {
	beeep.Notify(title, text, "")
}

func (a *App) GetFoundBaseStations() map[string]JsonBaseStation {
	var result = make(map[string]JsonBaseStation)
	for name, v := range knownBaseStations.Items() {

		if v == nil {
			log.Printf("Base station %s in nil\n", name)
			continue
		}

		bs := *v

		configBaseStation := config.KnownBaseStations[name]

		managed := 0

		if configBaseStation != nil {
			managed = configBaseStation.ManagedFlags
		}

		result[bs.GetId()] = JsonBaseStation{
			Name:         bs.GetName(),
			Channel:      bs.GetChannel(),
			PowerState:   bs.GetPowerState(),
			LastUpdated:  nil,
			Version:      bs.GetVersion(),
			Status:       bs.GetStatus(),
			ManagedFlags: managed,
			Id:           bs.GetId(),
		}

	}

	return result
}

func (a *App) UpdateBaseStationParam(name string, param string, value interface{}) {

	bs, found := knownBaseStations.Get(name)

	if !found {
		return
	}

	baseStation := *bs

	if param == "name" {
		baseStation.SetName(value.(string))
		config.UpdateBaseStationValue(name, "nickname", value)
		return

	}

	config.UpdateBaseStationValue(name, param, value)

}

func (a *App) GetConfiguration() *Configuration {
	return config
}

func (a *App) GetVersion() string {
	return BINARY_VERSION
}

func (a *App) ForceUpdate() {
	ForceUpdate()
}

func (a *App) IsUpdatingSupported() bool {
	return IsUpdatingSupported()
}

func (a *App) ToggleSteamVRManagement() *Configuration {
	config.IsSteamVRManaged = !config.IsSteamVRManaged

	config.Save()

	return config
}

func (a *App) ToggleTray() Configuration {
	config.AllowTray = !config.AllowTray

	config.Save()

	return *config
}

func (a *App) bluetoothCallback(adapter *bluetooth.Adapter, sr bluetooth.ScanResult) {

	if !strings.HasPrefix(sr.LocalName(), "LHB-") {
		return
	}

	log.Println(sr.LocalName())

	go ScanCallback(a, adapter, sr)
}

func (a *App) StartScanFor10Seconds() {
	WEBSOCKET_BROADCAST.Broadcast(preparePacket("client.scan", map[string]interface{}{
		"status": true,
	}))

	go adapter.Scan(a.bluetoothCallback)
	time.Sleep(time.Second * 10)
	adapter.StopScan()
	WEBSOCKET_BROADCAST.Broadcast(preparePacket("client.scan", map[string]interface{}{
		"status": false,
	}))
}

// EnableBluetooth brings up the adapter without starting a scan.
func (a *App) EnableBluetooth() bool {

	if a.bluetoothInitFinished {
		return true
	}

	if err := adapter.Enable(); err != nil {
		log.Printf("Failed to enable bluetooth adapter: %+v\n", err)
		return false
	}

	a.bluetoothInitFinished = true
	return true
}

func (a *App) InitBluetooth() bool {
	return a.EnableBluetooth()
}

func ScanCallback(app *App, a *bluetooth.Adapter, sr bluetooth.ScanResult) {

	_, found := knownBaseStations.Get(sr.LocalName())

	// var baseStation BaseStation
	if !found {
		knownBaseStations.Set(sr.LocalName(), nil)
		//Base station not found, we need to store it in out config then

		conn, err := a.Connect(sr.Address, bluetooth.ConnectionParams{})

		if err != nil {
			log.Printf("Failed to connect to bluetooth device: %s\n", sr.LocalName())

			return
		}

		bs, ok := InitBaseStation(&conn, a, sr.LocalName())

		if !ok {
			knownBaseStations.Remove(sr.LocalName())
			return
		}
		defer conn.Disconnect()

		knownBaseStations.Set(sr.LocalName(), &bs)

		//Saving base station
		config.SaveBaseStation(&bs)

		log.Println("New base station discovered and saved")

		if config.IsSteamVRManaged && runtime.GOOS == "windows" {

			running, err := isProcRunning("vrserver.exe")

			if err != nil {
				//whoops
				return
			}
			//Powering on base station
			if running {
				bs.SetPowerState(0x01)
			}
		}

		return
	}
}

func (a *App) ChangeBaseStationPowerStatus(baseStationMac string, status string) string {
	log.Printf("Power command requested: %s -> %s\n", baseStationMac, status)

	baseStation, found := knownBaseStations.Get(baseStationMac)

	if !found {
		log.Printf("Power command failed, %s is not a known base station\n", baseStationMac)
		return "Unknown base station"
	}

	// The link may have been released while idling in the tray, so bring it
	// back before trying to write. This is what lets the SteamVR automation
	// still work after we've handed the stations back to the other machine.
	if (*baseStation).GetStatus() != "ready" {
		a.ReconnectBaseStations()

		for i := 0; i < 30; i++ {
			time.Sleep(time.Second)
			if refreshed, ok := knownBaseStations.Get(baseStationMac); ok && (*refreshed).GetStatus() == "ready" {
				baseStation = refreshed
				break
			}
		}

		if (*baseStation).GetStatus() != "ready" {
			log.Printf("Could not reconnect to %s in time to change power state\n", baseStationMac)
			return "error: base station not connected"
		}
	}

	bs := *baseStation
	switch status {
	case "standingby":
		bs.SetPowerState(0x02)
	case "sleep":
		bs.SetPowerState(0x00)
	case "awake":
		bs.SetPowerState(0x01)
	default:
		return "unknown status"
	}

	return "ok"
}

func (a *App) ChangeBaseStationChannel(baseStationMac string, channel int) string {
	baseStation, found := knownBaseStations.Get(baseStationMac)

	if !found {
		return "Unknown base station"
	}

	for _, v := range knownBaseStations.Items() {
		bs := *v
		if bs.GetChannel() == channel {
			return "error: This channel conflicts with another base station"
		}
	}

	if channel < 1 || channel > 16 {
		return "error: Channel exceeds limit"
	}

	bs := *baseStation
	bs.SetChannel(channel)
	bs.SetPowerState(byte(BS_POWERSTATE_AWAKE))

	return "ok"
}

func (a *App) IdentitifyBaseStation(baseStationMac string) string {
	baseStation, found := knownBaseStations.Get(baseStationMac)

	if !found {
		return "Unknown base station"
	}

	bs := *baseStation
	bs.Identitfy()

	return "ok"
}

func (a *App) Shutdown() {
	disconnectAllBaseStations()

	shutdownSystray()
	os.Exit(0)
}

func (a *App) IsSteamVRConnectivityAvailable() bool {
	return runtime.GOOS == "windows" // No linux & macos support for now
}

func (a *App) IsSteamVRConnected() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	running, err := isProcRunning("vrserver.exe")

	if err != nil {
		log.Println("Failed to obtain process.")
		return false
	}

	return running
}

func (a *App) WakeUpAllBaseStations() {
	for _, c := range knownBaseStations.Items() {

		bs := *c
		if bs.GetPowerState() == BS_POWERSTATE_AWAKE {
			continue
		}
		bs.SetPowerState(0x01)
	}
}

func (a *App) SleepAllBaseStations() {
	for _, c := range knownBaseStations.Items() {
		bs := *c

		if bs.GetPowerState() != BS_POWERSTATE_AWAKE && bs.GetPowerState() != BS_POWERSTATE_AWAKE_2 {
			continue
		}

		bs.SetPowerState(0x01)
		bs.SetPowerState(0x00)
	}
}
