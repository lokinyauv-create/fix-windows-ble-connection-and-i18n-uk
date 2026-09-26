# mod: about-links — working "Made by / Design by / Source code" links

**Branch:** `mod/about-links` · **Base:** upstream `main` · **Depends on:** nothing
**Platforms:** all · **Commits:** 1 (2026-08-03)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

The "Made by", "Design by" and "Source code" links at the bottom of Settings did nothing.
Their `window.runtime.BrowserOpenURL(...)` calls were commented out during the TypeScript
migration (bf383c0) and never restored, so the links had been dead since June 2025.

## Fix

The calls are restored (with `//@ts-ignore`, because the Wails runtime isn't typed on
`window`), so each link opens in the system browser. A plain `<a href>` would navigate the
webview itself.

## Portable recipe (Wails and other webview apps)

Open external URLs with the host runtime (`BrowserOpenURL` in Wails, `shell.openExternal`
in Electron, `open()` from the Tauri shell plugin) instead of `<a href>`.

## Apply

```bash
git merge mod/about-links
git am mods/about-links/about-links.patch
```
