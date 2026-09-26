# Mods for go-steamvr-lighthouse-manager

[Українською](README.uk.md)

Each fix or feature from the `lokinyauv-create` fork is split out as its own branch on top
of upstream `main` (DHCPCD9/go-steamvr-lighthouse-manager @ 24a59b6). Each branch has:
- the original commits, with their original dates;
- `mods/<name>/README.md` (+ `README.uk.md`): problem → cause → portable recipe → files → apply → verify;
- `mods/<name>/<name>.patch`: the mod as a `git am`-able series against its base.

The recipes are written so the same fix can be carried into a **different** program (another
Lighthouse manager, another Wails/webview app, another OpenVR tool), not just this one.

**How each mod was tested, on what hardware, and what hasn't been tested yet:
[TESTING.md](TESTING.md).**

| Mod | Branch | Base / depends on | What it solves |
|---|---|---|---|
| [ble-connection](ble-connection/README.md) | `mod/ble-connection` | main | stations stuck on "Connecting…", dropped links, never reconnected |
| [power-state](power-state/README.md) | `mod/power-state` | ble-connection | commands that don't arrive; green in the UI but dark in reality |
| [power-buttons-ui](power-buttons-ui/README.md) | `mod/power-buttons-ui` | main | explicit on/off, group button, SteamVR automation flips, lost errors |
| [steamvr-lifecycle](steamvr-lifecycle/README.md) | `mod/steamvr-lifecycle` | power-state + power-buttons-ui | stations left claimed; quit with SteamVR; wait for sleep to land |
| [linux-steamvr](linux-steamvr/README.md) | `mod/linux-steamvr` | main | SteamVR auto-launch/state/paths on Linux |
| [i18n-uk](i18n-uk/README.md) | `mod/i18n-uk` | main | Ukrainian translation |
| [about-links](about-links/README.md) | `mod/about-links` | main | dead About/Source code links |
| [play-area](play-area/README.md) | `mod/play-area` | main | saved play areas, drift auto-fix, Room Setup crash on Linux/AMD |
| [lighthouse-dfu](lighthouse-dfu/README.md) | `mod/lighthouse-dfu` | main | base station firmware update from Linux |
| [wails-tooling](wails-tooling/README.md) | `chore/wails-tooling` | main | regenerated Wails files (no functional change) |

```
main ─┬─ ble-connection ── power-state ─┐
      ├─ power-buttons-ui ──────────────┴─ steamvr-lifecycle
      ├─ linux-steamvr   ├─ i18n-uk   ├─ about-links
      ├─ play-area       ├─ lighthouse-dfu
      └─ chore/wails-tooling
```

## Putting everything back together

Branch `all-mods` = upstream `main` + every mod, merged in this order:

1. `mod/steamvr-lifecycle` (brings ble-connection, power-state, power-buttons-ui)
2. `mod/linux-steamvr`: **conflict in `app.go`** (the "app launched again" handler). Keep the
   comment from linux-steamvr and the `a.ShowFromTray()` call from steamvr-lifecycle.
3. `mod/i18n-uk`
4. `mod/about-links`
5. `mod/play-area`: **conflicts in `TitleBar.tsx`, `en.json` and the generated
   `wailsjs/go/main/App.*`**. Keep both sides (the power buttons *and* the map icon, all
   keys); regenerate the bindings with `wails generate module`.
6. `mod/lighthouse-dfu`
7. `chore/wails-tooling`

The result is identical to the maintainer's working tree of 2026-09-26 (branch
`snapshot/2026-09-26`), except:
- the play-area helpers now live in `tools/` (with `.gitignore` entries) instead of the
  maintainer's home directory, and `playarea_linux.go` looks them up there;
- the later play-area fix that gives SteamVR up to 2 min to settle before the auto zone is
  applied.

Check with `git diff snapshot/2026-09-26 all-mods -- . ':!mods'`.

## Upstream

The maintainer asked for fixes to come as separate pull requests
([PR #25](https://github.com/DHCPCD9/go-steamvr-lighthouse-manager/pull/25)). Every
`mod/*` branch except play-area, lighthouse-dfu and chore/wails-tooling (personal tooling)
is a self-contained PR candidate.
