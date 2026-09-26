# chore: wails-tooling — regenerated Wails files

**Branch:** `chore/wails-tooling` · **Base:** upstream `main` · **Depends on:** nothing
**Commits:** 2 (2026-09-12, 2026-09-26) · **No functional change**

[Українською](README.uk.md) · [All mods](../README.md)

Output of a newer Wails CLI:
- `frontend/wailsjs/runtime/*`;
- `build/windows/installer/wails_tools.nsh`;
- `frontend/package.json.md5`;
- `github.com/getlantern/systray` promoted to a direct dependency in `go.mod`.

It exists only so that merging every mod reproduces the maintainer's working tree byte for
byte. Don't send it upstream. Other projects don't need it either: running `wails build`
regenerates these files.
