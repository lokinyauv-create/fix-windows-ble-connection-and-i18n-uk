# mod: linux-steamvr — SteamVR integration on Linux

**Branch:** `mod/linux-steamvr` · **Base:** upstream `main` · **Depends on:** nothing
**Platforms:** Linux (plus portable Steam paths for macOS/Windows) · **Commits:** 3 (2026-09-23 … 09-25)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

On Linux the app never learned that SteamVR had stopped: stations stayed awake and the app
stayed running. SteamVR couldn't auto-launch it, and "is SteamVR installed" was always false.

## Causes

- `isProcRunning()` for non-Windows was a stub that always returned `false`.
- `waitForSteamVR()` and `IsSteamVRConnected()` returned early on anything but Windows.
- Steam's config dir was hard-coded as `${ProgramFiles(x86)}\Steam\config`.
- The vrmanifest had only `binary_path_windows`.
- A second launch showed the window only if SteamVR wasn't running. On GNOME/Wayland there is
  no tray icon, so a hidden window couldn't be brought back.

## Portable recipe

- Detect processes on Linux by scanning `/proc/<pid>/comm`. It's cheap, needs no
  dependencies, and SteamVR's server is `vrserver` (no `.exe`).
- Resolve Steam's config dir per platform:
  - Linux: `~/.local/share/Steam/config` (native package; Flatpak and Snap differ);
  - macOS: `~/Library/Application Support/Steam/config`;
  - Windows: `%ProgramFiles(x86)%\Steam\config`.
- Put `binary_path_linux` / `binary_path_osx` in the `.vrmanifest` next to
  `binary_path_windows` (`%EXECUTABLE%`).
- On single-instance relaunch, **always** show the window.

## Files

`process_other.go`, `steam_{linux,windows,darwin}.go`, `config.go`, `app.go`,
`websockets.go`, `steamvr/manifest.vrmanifest`.

## Apply

```bash
git merge mod/linux-steamvr
git am mods/linux-steamvr/linux-steamvr.patch   # on top of upstream main
```

## Verify

Start SteamVR: the app starts (auto-launch) and the log shows `steamvr.status true`. Stop
SteamVR: `steamvr.status false` follows within seconds.
