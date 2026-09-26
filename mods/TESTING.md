# How the mods were tested

[Українською](TESTING.uk.md) · [All mods](README.md)

Everything below comes from real logs: the app's `log.txt`, SteamVR's `vrserver.txt`,
`basestation_wireless.txt`, `coredumpctl`, and the Room Setup `Player.log`. For the August
Windows tests, the source is the commit messages themselves. Legend:
- **live**: exercised on real hardware;
- **build**: compiled / type-checked / patch-applied only;
- **not yet**: not exercised.

## Test machine (Linux)

| | |
|---|---|
| OS | Fedora Linux 44 Workstation, kernel 7.2.6-200.fc44 |
| Bluetooth | BlueZ 5.87, USB adapter IMC Networks 13d3:3617 |
| GPU / driver | AMD Radeon RX 7700 XT / 7800 XT (Navi 32), Mesa 26.2.2 (freeworld RADV) |
| VR | Valve Index HMD (28de:2300) + Valve VR Radio (28de:2102), SteamVR build 25330290 (lighthouse driver 2.17.10) |
| Base stations | 3× Valve LHB 2.0: LHB-83D4ADEA (radio 2.9.2004771 from the start), LHB-624077D2 and LHB-C27FF789 (radio 2.2.9259382 / MCU 1.2 → flashed to 2.9.2004771 / 1.8 on 2026-09-26) |
| Toolchain | Go 1.26.8, Wails CLI 2.16.0 (module v2.10.2), Node 22.23.1, Python 3.14.7 + dbus_next 0.2.3 |

The August commits were also tested on a Windows PC against the same stations,
including a dual-OS test with Windows and Linux on one pair of stations. The commit
messages don't record the Windows version, so it isn't given here.

## Every mod: build checks (2026-09-26)

- `go vet` for **linux** and **windows** (GOOS=windows) on every branch and on `all-mods`;
  `tsc --noEmit` on every branch with frontend changes.
- Every `mods/<name>/<name>.patch` applied with `git am` onto its base; the result's code
  equals the branch (10/10).
- `all-mods` = the merge of every mod. Its code equals the maintainer's working tree
  (`snapshot/2026-09-26`) except the intended portability changes (play-area helpers in
  `tools/`, `.gitignore`) and the later play-area retry fix. `wails build -tags webkit2_41`
  succeeds.

## Per mod

### ble-connection — live (Windows + Linux, 2026-08-03…06), build (09-26)
- Dual-OS: Windows and Linux on the same two stations, then Windows shut down. The Linux
  build logged `Giving up connecting … after 6 attempts` instead of retrying forever, and
  normal connects succeeded right away on both.
- Race fix: one preload per station. A station that used to churn through six failed
  attempts connected on the first.
- Idle-station scan: a probe confirmed "not found" → 8 s scan sees it (rssi −36) → connects
  and enumerates 6 services. In the app, a station that used to exhaust all attempts
  connected on the first.
- 2026-09-26 logs: every session connected all 3 stations on the first pass (19:08, 19:09,
  22:01).

### power-state — live (Linux, 2026-09-26)
- **Bug reproduced (18:54):** LHB-624077D2 acked two wake writes and stayed dark;
  LHB-C27FF789 failed with `In Progress`, and the reconnect dropped the wake. The UI showed
  all three green. SteamVR tracked only LHB-83D4ADEA.
- **Root cause confirmed (19:04):** the same `0x01` re-sent by hand (`busctl … WriteValue`)
  woke LHB-624077D2 within seconds (`vrserver.txt`: "finished adding tracked device …
  LHB-624077D2"); LHB-C27FF789 the same at 19:05.
- **New build (19:08–19:09):** write-only stations got re-sends at +2/+5/+10/+15 s. A
  `Not connected` failure on LHB-83D4ADEA led to 3 retries, then a reconnect, then
  **re-applied pending state**, then `confirmed (state 1)`.
- **After the firmware update (22:01):** all 3 stations report feedback. Wake was confirmed
  on each (`Power state 1 confirmed … (state 9)`) within 2–3 s, and all three were tracked.
- **Sleep (22:05):** `Power state 0 confirmed` on all 3 within 2 s.
- **not yet (any more):** the write-only path can't be re-tested live, because no station
  here runs radio 2.2 now.

### power-buttons-ui — live (2026-08-05, per commits), build (09-26)
- Three presses of "on" three seconds apart all reached the stations, where before only
  the first did.
- A live log showed a second click aborting a boot (06:46:35 wake → 06:46:41 sleep),
  which led to the explicit on/off buttons.
- 2026-09-26: `tsc`. The header on/off buttons were used in the sessions above.

### steamvr-lifecycle — live
- Release on exit: verified on both platforms in August. Before the fix `bluetoothctl`
  still showed `Connected: yes` after the app was gone; after it, the list was empty.
- **2026-09-26 22:05:** SteamVR stopped → sleep sent → confirmed on all 3 → `Disconnecting
  … before exit` ×3 → process gone. The pending-command wait was short because feedback
  came in 2 s.
- 19:16: an earlier build showed the reason for the wait: LHB-C27FF789 got a single sleep
  write before the app disconnected.

### linux-steamvr — live (Linux)
- 2026-09-25: stop detection tested with a fake `vrserver` process.
- 2026-09-26: **real** SteamVR stops at 19:16 and 22:05 were detected (`steamvr.status
  false`), and the stations were put to sleep.
- SteamVR auto-launched the app at 22:01 (`vrserver.txt`: "Starting process …
  base-station-manager").

### i18n-uk — live
The Ukrainian UI is in daily use (all screenshots in this work are the Ukrainian UI).
`tsc` on 09-26.

### about-links — build
`tsc` only on 09-26. The live check happened when it was written in August.

### play-area — live (partly), build
- **Backend (go test harness, SteamVR off):**
  - `GetRoomStatus` finds both helpers;
  - `ListZones` reads the existing zone "Кімната" (4.0 × 2.2 m, auto);
  - universe IDs parse exactly (`json.Number`);
  - SteamVR-off errors are clear;
  - rename/auto round-trip leaves `zones.json` semantically unchanged, and the old `vr-zones
    list` still reads it.
- **UI:** page layout checked through `wails dev` in a browser. The bindings aren't
  available there, so data was checked through the backend test.
- **Live session 22:01:**
  - the watcher started with the app, and the auto zone was applied `(спроба 5)` ("attempt
    5") after 4 × "wrong universe?". **That led to the retry fix** (up to 2 min). The fix
    itself is **not yet** live-tested;
  - the Room Setup button launched the wizard twice (22:02:19, 22:02:52) with the layer
    active (`Player.log`: `[mipclamp] vkCreateImage extent/mips->max: 50 32 7 6`), and it
    didn't crash;
  - a crash at 22:02:05 (SIGTRAP in `Addr2ComputeSurfaceInfo`) came from SteamVR's own Room
    Setup button: its stack has SteamVR's envextensions layer but no `mipclamp`. That is the
    documented unfixed path.
- **Helpers:** `tools/vrchap-io` built from the vendored source (fetching `openvr.h`
  v2.15.6), and `check` returns exit 3 without SteamVR. `tools/room-setup-fix` builds with gcc.
- **not yet:** re-apply after `CALIBRATED base` hasn't been observed in this build (the
  standalone vr-zones watcher with the same logic was used on 2026-09-25).

### lighthouse-dfu — live (Linux, 2026-09-26)
- `info` read all 3 stations (Device Information 0x2A26).
- The buttonless path failed as described (write timed out, the station came back
  unchanged, no damage).
- **Manual `LHB-DFU` mode:**
  - LHB-624077D2 flashed at 20:25, LHB-C27FF789 at 20:34 (297 956 B each, every 4 KiB object
    CRC-checked);
  - both rebooted on their own and read `R: 2.9.2004771 | M: 1.8.2004742`;
  - the power characteristic changed from `write` to `read write notify`;
  - SteamVR tracked all 3 at 22:01.

### wails-tooling — build
Generated files only. `wails build` passes.

## Known gaps
- Windows was not re-tested for the 2026-09-26 changes (power-state confirmation loop,
  exit wait); `go vet` for Windows passes.
- macOS: never tested.
- The play-area retry fix and the recalibration re-apply are waiting for the next SteamVR
  session.
