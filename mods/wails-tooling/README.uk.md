# chore: wails-tooling — перегенеровані файли Wails

**Гілка:** `chore/wails-tooling` · **Основа:** `main` автора · **Залежить від:** нічого
**Коміти:** 2 (2026-09-12, 2026-09-26) · **Функціональних змін немає**

[English](README.md) · [Усі моди](../README.md)

Результат роботи новішої версії Wails CLI:
- `frontend/wailsjs/runtime/*`;
- `build/windows/installer/wails_tools.nsh`;
- `frontend/package.json.md5`;
- `github.com/getlantern/systray` став прямою залежністю в `go.mod`.

Цей коміт потрібен лише для того, щоб злиття всіх модів давало вашу робочу версію байт у
байт. Автору його не надсилайте. Іншим проєктам він теж не потрібен: `wails build` сам
перегенерує ці файли.
