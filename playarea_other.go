//go:build !linux
// +build !linux

package main

// Play area management relies on the Linux vrchap-io helper and the patched
// Room Setup launcher; other platforms get inert stubs.

type Zone struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	PlayArea  []float64 `json:"play_area"`
	CreatedAt string    `json:"created_at"`
	AutoApply bool      `json:"auto_apply"`
}

type RoomStatus struct {
	Supported      bool `json:"supported"`
	SteamVRRunning bool `json:"steamvr_running"`
	HelperFound    bool `json:"helper_found"`
	RoomSetupFound bool `json:"room_setup_found"`
	WatcherRunning bool `json:"watcher_running"`
}

const playAreaUnsupported = "error: not supported on this platform"

func startZoneWatcher() {}

func (a *App) GetRoomStatus() RoomStatus                { return RoomStatus{} }
func (a *App) ListZones() []Zone                        { return []Zone{} }
func (a *App) GetZoneLog() []string                     { return []string{} }
func (a *App) CaptureZone(name string) string           { return playAreaUnsupported }
func (a *App) ApplyZone(id string) string               { return playAreaUnsupported }
func (a *App) RenameZone(id string, name string) string { return playAreaUnsupported }
func (a *App) DeleteZone(id string) string              { return playAreaUnsupported }
func (a *App) SetAutoZone(id string) string             { return playAreaUnsupported }
func (a *App) LaunchRoomSetup() string                  { return playAreaUnsupported }
