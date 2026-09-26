# mod: steamvr-lifecycle — release the stations and quit with SteamVR

**Branch:** `mod/steamvr-lifecycle` · **Base:** merge of `mod/power-state` + `mod/power-buttons-ui`
**Depends on:** power-state (PowerCommandPending), power-buttons-ui (TitleBar sleep flow)
**Platforms:** all · **Commits:** 5 (2026-08-04 … 09-26)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

- After the app closed, the stations stayed "claimed". Another PC couldn't use them, which
  looked like dead hardware. On Linux, BlueZ keeps the link even after the process exits.
- With SteamVR auto-launching the app, it stayed in the tray after the session ended.
- The app quit right after sending "sleep", so write-only stations got only one attempt.

## What it does

1. **Release on exit.** `Shutdown()` existed but nothing called it. It is now wired into
   Wails `OnShutdown` plus SIGINT/SIGTERM. (SIGKILL can't be caught.)
2. **Tray.** An intermediate commit (`40b4904`) dropped the links while idling in the tray.
   It was reverted (`ecce728`) because on Windows tinygo's `Disconnect` leaves a half-open
   handle, and every later connect found no characteristics. The history keeps both so the
   reasoning isn't lost; the net effect is "keep links while in the tray".
3. **Quit after SteamVR.** When "manage power with SteamVR" is on and SteamVR goes running →
   stopped: sleep all managed stations, re-check the process directly (a new session may
   have started), then quit. SteamVR relaunches the app next session.
4. **Wait for pending commands.** Before disconnecting on exit, wait up to 16 s until every
   station has confirmed or finished re-sending its last command.

## Portable recipe

- A BLE central that can be relaunched **must disconnect explicitly on every exit path**.
  BlueZ doesn't drop the link when the process dies.
- "Quit when the host app quits": act on the transition, do the work (sleep), **re-verify
  the host is still gone**, and only then exit.
- Never exit while a device command is unconfirmed. Wait with a hard cap.

## Files

`app.go` (`disconnectAllBaseStations`, signal handling, tray hooks), `main.go`
(`OnShutdown`), `systray_windows.go`, `frontend/src/components/TitleBar.tsx` (quit after
sleep), `frontend/src/lib/native/index.ts`.

## Apply

```bash
git merge mod/steamvr-lifecycle   # brings power-state, ble-connection, power-buttons-ui
git am mods/steamvr-lifecycle/steamvr-lifecycle.patch   # on top of those three
```

## Verify

Stop SteamVR: the log shows the sleep commands, then re-sends, then "Disconnecting from base
station … before exit", and the process is gone. `bluetoothctl devices Connected` is empty.
