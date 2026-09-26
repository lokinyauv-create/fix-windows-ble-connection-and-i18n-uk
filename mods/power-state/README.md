# mod: power-state — power commands that really arrive, and an honest status

**Branch:** `mod/power-state` · **Base:** `mod/ble-connection` · **Depends on:** ble-connection
**Platforms:** all (confirmation loop tuned on Linux/BlueZ) · **Commits:** 3 (2026-08-03 … 09-26)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

The UI shows a station as on (green) while it is dark, or stations churn between
"ready" and "preloaded" forever.

## Causes

1. **A failed read was treated as a broken link.** Many stations don't allow reading the
   power characteristic, so `readPowerState` failed, called `Reconnect()`, and repeated
   this forever.
2. **`Disconnect()` left every handle in place.** Writes then went to a dead session. On
   Windows that "succeeds", so the app believed commands landed.
3. **Old radio firmware (2.2) is write-only.** There is no notify, and a read only echoes
   the last value written. A wake sent right after connecting (while notifications were
   still being set up by 2–3 parallel `StartCaching` calls) was acked and ignored, or
   failed with BlueZ `In Progress`. The failure path reconnected with `wakeUp=false` and
   dropped the request. Meanwhile the UI had already been told "awake" (state 9).

## Portable recipe

- A failed **read** is not a disconnect: return the cached value.
- On disconnect, **nil every handle** (device, service, characteristics, status) and publish
  the status, so the normal reconnect path runs.
- **Serialise GATT operations per device** (one mutex around write, StartNotify, …).
- Set up notifications **once per connection**, and **before** the first command.
- Retry a failed write a couple of times (500 ms) before reconnecting, and keep the command
  **pending** so it is re-applied after the reconnect.
- **Confirm every command:**
  - stations that notify: wait for the state (wake: `0x01` accepted, `0x09` booting, `0x0b`
    on; sleep `0x00`; standby `0x02`) and re-send only if it doesn't come;
  - write-only stations: re-send at +2/+5/+10/+15 s (the commands are idempotent);
  - a newer command cancels the older confirmation loop (generation counter).
- Report an **assumed** state only for stations that can't report their own.

Lighthouse 2.0 GATT: service `00001523-1212-efde-1523-785feabcd124`, power
`00001525-…` (write `0x00` sleep / `0x01` on / `0x02` standby), mode/channel `00001524-…`.
Firmware version is in Device Information `0x2A26`. Radio 2.9+ makes power read/notify.

## Files

`bs.go` (`SetPowerState`, `confirmPowerState`, `powerStateReached`, `PowerCommandPending`,
`Disconnect`), `bs_other.go` (`gattMu`, `StartCaching` once, `Write`), `websockets.go`
(no crash on a plain HTTP request).

## Apply

```bash
git merge mod/power-state      # brings mod/ble-connection with it
git am mods/power-state/power-state.patch   # on top of mod/ble-connection
```

## Verify

Log lines `Power state 1 written to … (feedback: false)`, then
`Re-sending power state 1 to write-only …` or `Power state 1 confirmed on …`. Every station
shows up in SteamVR (`grep "adding tracked device.*LHB" vrserver.txt`).
