# Мод: linux-steamvr — інтеграція зі SteamVR на Linux

**Гілка:** `mod/linux-steamvr` · **Основа:** `main` автора · **Залежить від:** нічого
**Платформи:** Linux (і переносні шляхи Steam для macOS/Windows) · **Коміти:** 3 (2026-09-23 … 09-25)

[English](README.md) · [Усі моди](../README.md)

## Проблема

На Linux програма не помічала, що SteamVR зупинився: станції лишались увімкненими, а
програма працювала далі. SteamVR не міг її автозапустити, а «SteamVR встановлено» завжди
було `false`.

## Причини

- `isProcRunning()` для не-Windows був заглушкою, що завжди повертала `false`.
- `waitForSteamVR()` і `IsSteamVRConnected()` одразу виходили на будь-якій ОС, крім Windows.
- Шлях до конфігурації Steam був жорстко заданий як `${ProgramFiles(x86)}\Steam\config`.
- У vrmanifest був лише `binary_path_windows`.
- Повторний запуск показував вікно, лише якщо SteamVR не працює. На GNOME/Wayland значка в
  треї немає, тож приховане вікно не повернути.

## Переносний рецепт

- Процеси на Linux шукайте за `/proc/<pid>/comm`. Це дешево й без залежностей, а сервер
  SteamVR зветься `vrserver` (без `.exe`).
- Теку конфігурації Steam визначайте для кожної платформи:
  - Linux: `~/.local/share/Steam/config` (звичайний пакет; у Flatpak і Snap інакше);
  - macOS: `~/Library/Application Support/Steam/config`;
  - Windows: `%ProgramFiles(x86)%\Steam\config`.
- У `.vrmanifest` додайте `binary_path_linux` / `binary_path_osx` поруч із
  `binary_path_windows` (`%EXECUTABLE%`).
- При повторному запуску (single instance) вікно показуйте **завжди**.

## Файли

`process_other.go`, `steam_*.go`, `config.go`, `app.go`, `websockets.go`,
`steamvr/manifest.vrmanifest`.

## Як застосувати

```bash
git merge mod/linux-steamvr
git am mods/linux-steamvr/linux-steamvr.patch   # поверх main автора
```

## Перевірка

Запустіть SteamVR: програма стартує сама, у журналі `steamvr.status true`. Зупиніть SteamVR:
за кілька секунд з'явиться `steamvr.status false`.
