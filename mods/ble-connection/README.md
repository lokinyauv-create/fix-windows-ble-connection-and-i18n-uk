# mod: ble-connection — base stations that connect, and stay connected

**Branch:** `mod/ble-connection` · **Base:** upstream `main` (24a59b6) · **Depends on:** nothing
**Platforms:** Windows, Linux (BlueZ) · **Commits:** 6 (2026-08-03 … 08-06)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

Base stations hang on "Connecting…" forever, connect and drop straight away, or are
never picked up again after another PC releases them.

## Causes

1. **Scanning while enumerating GATT.** Startup ran a 10 s advertisement scan at the same
   time as it connected to known stations. On Windows service discovery hangs indefinitely
   while the radio scans.
2. **`defer conn.Disconnect()`** in the connect path closed every link the moment the
   function returned, which was right after it succeeded. This happened on both platforms.
3. **No timeouts.** `adapter.Connect()` (BlueZ D-Bus signal / WinRT async) and Windows
   `GetGattServicesWithCacheModeAsync` can block forever.
4. **Unbounded retries.** On Linux the retry loop ran once a second, forever. On Windows
   `FindService`/`ScanCharacteristics` reconnected from inside discovery, which caused a storm
   of overlapping reconnects.
5. **Two connection chains racing for one station.** If a power command arrived during the
   startup preload, a second preload started. A station accepts one link at a time, so both
   chains failed ("Mode: false, Power: false, ID: false").
6. **"Device not found" for an idle station.** Both OSes refuse to connect by address to a
   device they haven't seen advertise recently, even though it is on and advertising.

## Portable recipe (for any app that talks to Lighthouse 2.0 stations)

- Keep **scanning and connecting mutually exclusive**. Hold a mutex around scans, keep scans
  short, and wait about 500 ms after `StopScan` before connecting.
- Never `defer Disconnect()` in a function that hands the connection to someone else.
- Wrap **every** blocking BLE call in a timeout (connect about 10 s, service discovery
  about 8 s), and treat a timeout as a normal failure.
- Retry the **whole connect** (connect + discover) with a cap (5 attempts). Never reconnect
  from inside discovery.
- Track "connection in flight" per device (a `sync.Map` of IDs) and skip devices that are
  already connecting or connected when a second pass starts.
- On "not found" / missing BlueZ object, **scan until that address shows up**, then retry.
- Keep a real log file open for the life of the process, with the file first in
  `io.MultiWriter`. A GUI app launched by SteamVR has no console.

## Files

`app.go` (EnableBluetooth vs scanning, `preloadBaseStations` skip, logging),
`bs.go` (`connectingBaseStations`, `discoverBaseStation`, `scanMutex`),
`bs_other.go` / `bs_windows.go` (timeouts, retry caps, discovery),
`frontend/wailsjs/go/main/App.*` (regenerated bindings).

## Apply

```bash
git merge mod/ble-connection                   # or
git am mods/ble-connection/ble-connection.patch  # on top of upstream main
```

## Verify

Start the app with 2+ known stations. In `log.txt` there is one connect per station, no
"Mode: false…", and a station that was idle for hours connects on the first attempt
(preceded by "Scanning so the adapter can resolve …").
