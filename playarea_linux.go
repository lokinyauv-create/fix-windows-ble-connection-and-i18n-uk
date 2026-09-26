//go:build linux
// +build linux

package main

// Play area ("Room" page) support: saved SteamVR chaperone zones, their
// automatic re-application, and the patched Room Setup launcher. Ported from
// the standalone vr-zones tool - it keeps the same ~/.config/vr-zones/zones.json
// so profiles saved there carry over. SteamVR is reached through the small
// vrchap-io helper (tools/vrchap-io, OpenVR ExportLiveToBuffer /
// ImportFromBufferToWorking) and Room Setup through tools/room-setup-fix.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Zone struct {
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	Data      string    `json:"data,omitempty"`
	PlayArea  []float64 `json:"play_area"`
	CreatedAt string    `json:"created_at"`
	AutoApply bool      `json:"auto_apply"`
}

type zoneStore struct {
	Zones map[string]*Zone `json:"zones"`
}

type RoomStatus struct {
	Supported      bool `json:"supported"`
	SteamVRRunning bool `json:"steamvr_running"`
	HelperFound    bool `json:"helper_found"`
	RoomSetupFound bool `json:"room_setup_found"`
	WatcherRunning bool `json:"watcher_running"`
}

var (
	zonesMu    sync.Mutex // guards zones.json read-modify-write
	zoneLogMu  sync.Mutex
	zoneLog    []string
	zoneWatchO sync.Once
)

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// findTool looks for a bundled helper: $env first, then next to the
// executable (packaged layout) and in the source tree (build/bin/<exe> ->
// ../../tools/...).
func findTool(env string, rel ...string) string {
	if p := os.Getenv(env); p != "" {
		return p
	}
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(append([]string{dir, "tools"}, rel...)...),
			filepath.Join(append([]string{dir, "..", "..", "tools"}, rel...)...))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(append([]string{wd, "tools"}, rel...)...))
	}
	for _, c := range candidates {
		if fileExists(c) {
			return filepath.Clean(c)
		}
	}
	if len(candidates) > 0 {
		return filepath.Clean(candidates[0])
	}
	return filepath.Join(rel...)
}

func vrchapIOPath() string {
	return findTool("BSM_VRCHAP_IO", "vrchap-io", "vrchap-io")
}

func roomSetupPath() string {
	return findTool("BSM_ROOM_SETUP", "room-setup-fix", "vr-room-setup")
}

func zonesFile() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(dir, "vr-zones", "zones.json")
}

func vrserverLogPath() string {
	return filepath.Join(homeDir(), ".local", "share", "Steam", "logs", "vrserver.txt")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// zoneLogf records a line for the Room page and the app log.
func zoneLogf(format string, args ...interface{}) {
	line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	log.Println("[zones]", line)

	zoneLogMu.Lock()
	defer zoneLogMu.Unlock()
	zoneLog = append(zoneLog, line)
	if len(zoneLog) > 50 {
		zoneLog = zoneLog[len(zoneLog)-50:]
	}
}

// ---------- storage ------------------------------------------------------------------------------

func loadZones() (*zoneStore, error) {
	store := &zoneStore{Zones: map[string]*Zone{}}
	raw, err := os.ReadFile(zonesFile())
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, store); err != nil {
		return nil, err
	}
	if store.Zones == nil {
		store.Zones = map[string]*Zone{}
	}
	return store, nil
}

func saveZones(store *zoneStore) error {
	path := zonesFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmp := strings.TrimSuffix(path, ".json") + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------- SteamVR ------------------------------------------------------------------------------

func steamVRRunning() bool {
	return vrserverPid() != ""
}

func vrserverPid() string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err == nil && strings.TrimSpace(string(comm)) == "vrserver" {
			return e.Name()
		}
	}
	return ""
}

// runHelper runs vrchap-io. VR_Init can hang while SteamVR is still starting,
// hence the timeout; exit code 124 means "SteamVR didn't answer in time".
func runHelper(timeout time.Duration, stdin string, args ...string) (int, string, string) {
	helper := vrchapIOPath()
	if !fileExists(helper) {
		return -1, "", fmt.Sprintf("нема %s: запустіть tools/vrchap-io/build.sh", helper)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, helper, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return 124, "", "SteamVR не відповів вчасно"
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			return -1, "", err.Error()
		}
	}
	return code, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())
}

func exportLive() (string, error) {
	code, out, errOut := runHelper(25*time.Second, "", "export")
	if code != 0 {
		if errOut == "" {
			errOut = "не вдалося отримати поточну зону"
		}
		return "", errors.New(errOut)
	}
	return out, nil
}

type universeInfo struct {
	id       string
	playArea []float64
}

// universes returns (id, play_area) for every universe in a chaperone JSON,
// with or without the "universes" wrapper.
func universes(blob string) []universeInfo {
	// UseNumber keeps big universe IDs exact instead of rounding them to float64.
	var root interface{}
	dec := json.NewDecoder(strings.NewReader(blob))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return nil
	}
	var items []interface{}
	if obj, ok := root.(map[string]interface{}); ok {
		if list, ok := obj["universes"].([]interface{}); ok {
			items = list
		} else {
			items = []interface{}{obj}
		}
	}
	var result []universeInfo
	for _, item := range items {
		u, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		info := universeInfo{id: "?"}
		if id, ok := u["universeID"]; ok {
			info.id = fmt.Sprint(id)
		}
		if pa, ok := u["play_area"].([]interface{}); ok {
			for _, v := range pa {
				if n, ok := v.(json.Number); ok {
					if f, err := n.Float64(); err == nil {
						info.playArea = append(info.playArea, f)
					}
				}
			}
		}
		result = append(result, info)
	}
	return result
}

func formatArea(pa []float64) string {
	if len(pa) < 2 {
		return "розмір невідомий"
	}
	return fmt.Sprintf("%.1f × %.1f м", pa[0], pa[1])
}

func applyBlob(blob string) error {
	code, out, errOut := runHelper(25*time.Second, blob, "import", "-")
	if code == 0 && strings.Contains(out, "applied") {
		return nil
	}
	msg := errOut
	if msg == "" {
		msg = out
	}
	if msg == "" {
		msg = "SteamVR не прийняв зону"
	}
	return errors.New(msg)
}

func applyZoneById(id string) (string, error) {
	zonesMu.Lock()
	store, err := loadZones()
	zonesMu.Unlock()
	if err != nil {
		return "", err
	}
	zone := store.Zones[id]
	if zone == nil {
		return "", errors.New("зону видалено")
	}

	warn := ""
	if live, err := exportLive(); err == nil {
		saved := map[string]bool{}
		for _, u := range universes(zone.Data) {
			saved[u.id] = true
		}
		var liveIds []string
		overlap := false
		for _, u := range universes(live) {
			liveIds = append(liveIds, u.id)
			overlap = overlap || saved[u.id]
		}
		if len(saved) > 0 && len(liveIds) > 0 && !overlap {
			warn = fmt.Sprintf("увага: зона збережена для іншого всесвіту станцій (зараз %s); ", strings.Join(liveIds, ", "))
		}
	}

	if err := applyBlob(zone.Data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%sзастосовано «%s» (%s)", warn, zone.Name, formatArea(zone.PlayArea)), nil
}

func autoZoneId() string {
	zonesMu.Lock()
	defer zonesMu.Unlock()
	store, err := loadZones()
	if err != nil {
		return ""
	}
	for id, z := range store.Zones {
		if z.AutoApply {
			return id
		}
	}
	return ""
}

func newZoneId() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- watcher ------------------------------------------------------------------------------

// startZoneWatcher re-applies the auto zone (1) once per SteamVR session, as
// soon as the room is reachable, and (2) after every base station tilt
// recalibration ("CALIBRATED base" in vrserver.txt), which is when the floor
// and bounds drift. It runs for the life of the app - SteamVR launches us for
// every session and we quit when it stops.
func startZoneWatcher() {
	zoneWatchO.Do(func() { go zoneWatcher() })
}

func zoneWatcher() {
	zoneLogf("стежу за SteamVR")
	for _, tool := range []struct{ name, path string }{
		{"vrchap-io", vrchapIOPath()},
		{"Room Setup", roomSetupPath()},
	} {
		state := "знайдено"
		if !fileExists(tool.path) {
			state = "НЕМАЄ"
		}
		zoneLogf("%s: %s (%s)", tool.name, tool.path, state)
	}
	lastPid := ""
	var logPos int64 = -1
	var pendingAt time.Time

	for {
		pid := vrserverPid()
		switch {
		case pid != "" && pid != lastPid:
			lastPid = pid
			zoneLogf("SteamVR запущений (vrserver %s)", pid)
			sessionApply(pid)
			logPos = logSize()
		case pid == "" && lastPid != "":
			zoneLogf("SteamVR закрився")
			lastPid, logPos, pendingAt = "", -1, time.Time{}
		case pid != "" && logPos >= 0:
			var hit bool
			logPos, hit = scanRecalibrations(logPos)
			if hit {
				// Several stations recalibrate in a row; wait for 3 s of quiet.
				pendingAt = time.Now().Add(3 * time.Second)
			}
		}

		if !pendingAt.IsZero() && time.Now().After(pendingAt) {
			pendingAt = time.Time{}
			if id := autoZoneId(); id != "" {
				zoneLogf("станція наново відкалібрувала нахил — перезастосовую зону")
				if msg, err := applyZoneById(id); err != nil {
					zoneLogf("не вдалося застосувати після перекалібрування: %v", err)
				} else {
					zoneLogf("%s", msg)
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func logSize() int64 {
	info, err := os.Stat(vrserverLogPath())
	if err != nil {
		return 0
	}
	return info.Size()
}

func scanRecalibrations(from int64) (int64, bool) {
	f, err := os.Open(vrserverLogPath())
	if err != nil {
		return from, false
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() < from {
		from = 0 // log was rotated by a new session
	}
	if _, err := f.Seek(from, io.SeekStart); err != nil {
		return from, false
	}
	chunk, err := io.ReadAll(f)
	if err != nil {
		return from, false
	}
	return from + int64(len(chunk)), bytes.Contains(chunk, []byte("CALIBRATED base"))
}

func sessionApply(pid string) {
	id := autoZoneId()
	if id == "" {
		zoneLogf("немає зони з автозастосуванням")
		return
	}

	deadline := time.Now().Add(4 * time.Minute)
	ready := false
	for time.Now().Before(deadline) {
		if vrserverPid() != pid {
			return
		}
		if code, _, _ := runHelper(12*time.Second, "", "check"); code == 0 {
			ready = true
			break
		}
		time.Sleep(3 * time.Second)
	}
	if !ready {
		zoneLogf("SteamVR так і не став готовий за 4 хвилини")
		return
	}

	// Right after startup SteamVR keeps rejecting the import ("wrong
	// universe?") until the base stations have settled which universe this
	// is - about 25 s in a real session, which used up five of the six
	// attempts this loop used to allow. Keep trying for up to 2 minutes and
	// log a failure at most every 30 s so the Room page log stays readable.
	time.Sleep(6 * time.Second)
	deadline = time.Now().Add(2 * time.Minute)
	var lastLogged time.Time
	for attempt := 1; ; attempt++ {
		if vrserverPid() != pid {
			return
		}
		msg, err := applyZoneById(id)
		if err == nil {
			zoneLogf("%s (спроба %d)", msg, attempt)
			return
		}
		if time.Since(lastLogged) >= 30*time.Second {
			zoneLogf("спроба %d не вдалась, SteamVR ще не готовий? %v", attempt, err)
			lastLogged = time.Now()
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Second)
	}
	zoneLogf("не вдалося застосувати зону за 2 хвилини; спробую знову після перекалібрування станцій")
}

// ---------- bindings -----------------------------------------------------------------------------

func (a *App) GetRoomStatus() RoomStatus {
	return RoomStatus{
		Supported:      true,
		SteamVRRunning: steamVRRunning(),
		HelperFound:    fileExists(vrchapIOPath()),
		RoomSetupFound: fileExists(roomSetupPath()),
		WatcherRunning: true,
	}
}

func (a *App) ListZones() []Zone {
	zonesMu.Lock()
	defer zonesMu.Unlock()
	store, err := loadZones()
	if err != nil {
		zoneLogf("не вдалося прочитати зони: %v", err)
		return []Zone{}
	}
	result := make([]Zone, 0, len(store.Zones))
	for id, z := range store.Zones {
		zone := *z
		zone.Id = id
		zone.Data = ""
		result = append(result, zone)
	}
	return result
}

func (a *App) GetZoneLog() []string {
	zoneLogMu.Lock()
	defer zoneLogMu.Unlock()
	return append([]string{}, zoneLog...)
}

func (a *App) CaptureZone(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "error: вкажіть назву"
	}
	blob, err := exportLive()
	if err != nil {
		return "error: " + err.Error()
	}
	us := universes(blob)
	if len(us) == 0 {
		return "error: SteamVR віддав порожні дані кімнати — спершу пройдіть калібрування кімнати"
	}

	zonesMu.Lock()
	defer zonesMu.Unlock()
	store, err := loadZones()
	if err != nil {
		return "error: " + err.Error()
	}
	for _, z := range store.Zones {
		if strings.EqualFold(z.Name, name) {
			return fmt.Sprintf("error: зона «%s» уже є", name)
		}
	}
	id := newZoneId()
	store.Zones[id] = &Zone{
		Id: id, Name: name, Data: blob, PlayArea: us[0].playArea,
		CreatedAt: time.Now().Format("2006-01-02T15:04:05"),
	}
	if err := saveZones(store); err != nil {
		return "error: " + err.Error()
	}
	zoneLogf("збережено «%s» (%s)", name, formatArea(us[0].playArea))
	return "ok"
}

func (a *App) ApplyZone(id string) string {
	msg, err := applyZoneById(id)
	if err != nil {
		zoneLogf("не вдалося застосувати: %v", err)
		return "error: " + err.Error()
	}
	zoneLogf("%s", msg)
	return "ok"
}

func (a *App) RenameZone(id string, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "error: вкажіть назву"
	}
	zonesMu.Lock()
	defer zonesMu.Unlock()
	store, err := loadZones()
	if err != nil {
		return "error: " + err.Error()
	}
	zone := store.Zones[id]
	if zone == nil {
		return "error: зону видалено"
	}
	for otherId, z := range store.Zones {
		if otherId != id && strings.EqualFold(z.Name, name) {
			return fmt.Sprintf("error: зона «%s» уже є", name)
		}
	}
	zone.Name = name
	if err := saveZones(store); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}

func (a *App) DeleteZone(id string) string {
	zonesMu.Lock()
	defer zonesMu.Unlock()
	store, err := loadZones()
	if err != nil {
		return "error: " + err.Error()
	}
	zone := store.Zones[id]
	if zone == nil {
		return "ok"
	}
	delete(store.Zones, id)
	if err := saveZones(store); err != nil {
		return "error: " + err.Error()
	}
	zoneLogf("видалено «%s»", zone.Name)
	return "ok"
}

// SetAutoZone makes id the only auto-applied zone; an empty id turns it off.
func (a *App) SetAutoZone(id string) string {
	zonesMu.Lock()
	defer zonesMu.Unlock()
	store, err := loadZones()
	if err != nil {
		return "error: " + err.Error()
	}
	if id != "" && store.Zones[id] == nil {
		return "error: зону видалено"
	}
	for zid, z := range store.Zones {
		z.AutoApply = zid == id
	}
	if err := saveZones(store); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}

func (a *App) LaunchRoomSetup() string {
	if !steamVRRunning() {
		return "error: спершу запустіть SteamVR"
	}
	path := roomSetupPath()
	if !fileExists(path) {
		return "error: нема " + path
	}
	cmd := exec.Command(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return "error: " + err.Error()
	}
	go cmd.Wait()
	zoneLogf("запущено калібрування кімнати (Room Setup)")
	return "ok"
}
