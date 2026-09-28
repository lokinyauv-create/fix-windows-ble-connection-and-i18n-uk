# mod: steamvr-lifecycle — release the stations and quit with SteamVR

**Branch:** `mod/steamvr-lifecycle` · **Base:** merge of `mod/power-state` + `mod/power-buttons-ui`
**Depends on:** power-state (PowerCommandPending), power-buttons-ui (TitleBar sleep flow)
**Platforms:** all · **Commits:** 6 (2026-08-04 … 09-28)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

- After the app closed, the stations stayed "claimed". Another PC couldn't use them, which
  looked like dead hardware. On Linux, BlueZ keeps the link even after the process exits.
- With SteamVR auto-launching the app, it stayed in the tray after the session ended.
- The app quit right after sending "sleep", so write-only stations got only one attempt.
- A SteamVR restart (Steam's "quit all" and relaunch, ~8 s without vrserver) counted as the
  end of the session. The app slept the stations and was still quitting when the new session
  tried to launch it (`VRApplicationError_ApplicationAlreadyRunning`), so the new session ran
  with one station woken halfway and nothing to wake the rest.

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
5. **Survive a SteamVR restart.** A stop only counts once vrserver has been gone for 20 s
   (`steamVRStopGrace`), so a quick restart changes nothing. If SteamVR still comes back while
   the app is releasing the stations, `Shutdown()` re-checks the process after disconnecting
   and stays up instead of exiting. Power commands wait for that decision (`shutdownMu`), so
   the new session's wake reconnects the stations instead of racing the teardown.

## Portable recipe

- A BLE central that can be relaunched **must disconnect explicitly on every exit path**.
  BlueZ doesn't drop the link when the process dies.
- "Quit when the host app quits": act on the transition, do the work (sleep), **re-verify
  the host is still gone**, and only then exit.
- Debounce the host's exit: a restart looks like a short stop. Re-check once more right
  before the irreversible step, and serialize it against work that assumes you stay.
- Never exit while a device command is unconfirmed. Wait with a hard cap.

## Files

`app.go` (`disconnectAllBaseStations`, `Shutdown`, signal handling, tray hooks),
`websockets.go` (`waitForSteamVR` stop grace), `main.go`
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

Restart SteamVR (stop and start again within 20 s): nothing is put to sleep, the app stays up.
Between 20 s and the end of the teardown: the log shows "SteamVR is running again, cancelling
exit" and the stations wake again.
