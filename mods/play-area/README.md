# mod: play-area — saved SteamVR play areas, auto re-apply, and a Room Setup that doesn't crash

**Branch:** `mod/play-area` · **Base:** upstream `main` · **Depends on:** nothing
**Platforms:** Linux (other platforms compile with stubs) · **Commits:** 4 (2026-09-21 … 09-26)

[Українською](README.uk.md) · [All mods](../README.md)

## Problems

1. SteamVR's floor and bounds drift. Every time a base station's tilt is re-measured (after
   sleep/wake or a short tracking loss, logged as `CALIBRATED base` in `vrserver.txt`), the
   floor and the play area shift a little.
2. SteamVR Room Setup crashes on Linux with AMD (Mesa RADV), so a drifted room can't be
   fixed the normal way.

## What it adds

- A **Room** page (map icon in the title bar):
  - save the current play area under a name; apply, rename or delete it;
  - mark one zone as **Auto**;
  - a **Room Setup** button;
  - a live log.
- A **watcher** that starts with the app. SteamVR auto-launches the app, so it covers every
  session. It applies the Auto zone once SteamVR is ready (waits up to 4 min for OpenVR, then
  keeps trying for up to 2 min while SteamVR settles the universe; in a real session that
  took ~25 s), and again after every `CALIBRATED base`, 3 s after the last one. When it
  starts, the log shows where the helpers were found.
- `tools/vrchap-io`: a tiny OpenVR client with `check` / `export` / `import` of the **live**
  chaperone (`IVRChaperoneSetup::ExportLiveToBuffer`, `ImportFromBufferToWorking` +
  `CommitWorkingCopy`, the same calls Room Setup makes). It never starts SteamVR itself
  (exit code 3).
- `tools/room-setup-fix`: `vr-room-setup` launcher plus the `VK_LAYER_vrfix_mipclamp`
  Vulkan layer.
  - Room Setup (Unity 5.6) creates a 50×32 image with **7** mip levels, but Vulkan allows 6.
    RADV traps (SIGTRAP in `Addr2ComputeSurfaceInfo`). The layer clamps `mipLevels` in
    `vkCreateImage` / `vkCreateImageView` and stubs the `VK_EXT_debug_utils` functions
    vrclient calls without a NULL check.
  - The launcher also enables SteamVR's `VK_LAYER_LUNARG_envextensions_layer` with the
    external memory, semaphore and timeline extensions SteamVR normally injects when it
    launches Room Setup itself.
- Zones are stored in `~/.config/vr-zones/zones.json`, the same format as the standalone
  vr-zones tool.

## Build the helpers

```bash
tools/vrchap-io/build.sh          # OPENVR_INCLUDE=… or it fetches openvr.h v2.15.6
tools/room-setup-fix/build.sh     # needs gcc + vulkan-headers
```

The app looks for them via `BSM_VRCHAP_IO` / `BSM_ROOM_SETUP`, then next to the
executable (`tools/…`), then in the source tree (`build/bin/../../tools/…`).

## Portable recipe

- **Any app** can save and restore the play area with the two OpenVR calls above. Keep the
  raw JSON blob, compare `universeID` before applying (a different universe means the stations
  were moved), and read big IDs as strings or `json.Number`.
- To fight drift, tail `vrserver.txt` for `CALIBRATED base`, debounce, and re-apply.
- Unity/RADV mip crash: reuse `tools/room-setup-fix` as is for any Vulkan app that asks
  for more mips than `floor(log2(max(w,h)))+1`. Enable it with `VK_ADD_LAYER_PATH` +
  `VK_LOADER_LAYERS_ENABLE`.

## Files

`playarea_linux.go`, `playarea_other.go`, `app.go` (`startZoneWatcher()`),
`frontend/src/views/PlayAreaView.tsx`, `app.tsx` (route `/room`), `TitleBar.tsx` (icon),
`lib/native/index.ts`, `locales/en.json`, `tools/vrchap-io/*`, `tools/room-setup-fix/*`,
`.gitignore`.

## Apply

```bash
git merge mod/play-area
git am mods/play-area/play-area.patch   # on top of upstream main
```

## Verify

Open the Room page with SteamVR running and press "Save". Move the floor, then "Apply now":
the floor comes back. On the next SteamVR start the log shows
`застосовано «…» (спроба N)` ("applied, attempt N"). The Room Setup wizard opens without crashing.
