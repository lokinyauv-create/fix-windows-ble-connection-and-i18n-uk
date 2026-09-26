# Моди для go-steamvr-lighthouse-manager

[English](README.md)

Кожне виправлення чи функція з форку `lokinyauv-create` винесене в окрему гілку поверх
`main` автора (DHCPCD9/go-steamvr-lighthouse-manager @ 24a59b6). У кожній гілці є:
- оригінальні коміти з оригінальними датами;
- `mods/<назва>/README.uk.md` (+ `README.md`): проблема → причина → переносний рецепт →
  файли → як застосувати → як перевірити;
- `mods/<назва>/<назва>.patch`: мод як серія комітів для `git am` поверх своєї основи.

Рецепти написані так, щоб те саме виправлення можна було перенести в **іншу** програму
(інший менеджер станцій, іншу програму на Wails/webview, інший інструмент для OpenVR), а не
лише в цю.

**Як тестувався кожен мод, на якому обладнанні і що ще не перевірено:
[TESTING.uk.md](TESTING.uk.md).**

| Мод | Гілка | Основа / залежить від | Що вирішує |
|---|---|---|---|
| [ble-connection](ble-connection/README.uk.md) | `mod/ble-connection` | main | станції висять на «Connecting…», відпадають, не перепідключаються |
| [power-state](power-state/README.uk.md) | `mod/power-state` | ble-connection | команди не доходять; в інтерфейсі зелена, насправді темна |
| [power-buttons-ui](power-buttons-ui/README.uk.md) | `mod/power-buttons-ui` | main | окремі «увімк/вимк», кнопка групи, збої автоматики SteamVR, загублені помилки |
| [steamvr-lifecycle](steamvr-lifecycle/README.uk.md) | `mod/steamvr-lifecycle` | power-state + power-buttons-ui | станції лишаються зайнятими; вихід разом зі SteamVR; дочекатися сну |
| [linux-steamvr](linux-steamvr/README.uk.md) | `mod/linux-steamvr` | main | автозапуск, стан і шляхи SteamVR на Linux |
| [i18n-uk](i18n-uk/README.uk.md) | `mod/i18n-uk` | main | український переклад |
| [about-links](about-links/README.uk.md) | `mod/about-links` | main | неробочі посилання «Про програму» |
| [play-area](play-area/README.uk.md) | `mod/play-area` | main | збережені зони, автовиправлення дрейфу, падіння Room Setup на Linux/AMD |
| [lighthouse-dfu](lighthouse-dfu/README.uk.md) | `mod/lighthouse-dfu` | main | оновлення прошивки станцій з Linux |
| [wails-tooling](wails-tooling/README.uk.md) | `chore/wails-tooling` | main | перегенеровані файли Wails (без функціональних змін) |

```
main ─┬─ ble-connection ── power-state ─┐
      ├─ power-buttons-ui ──────────────┴─ steamvr-lifecycle
      ├─ linux-steamvr   ├─ i18n-uk   ├─ about-links
      ├─ play-area       ├─ lighthouse-dfu
      └─ chore/wails-tooling
```

## Як зібрати все назад

Гілка `all-mods` = `main` автора + усі моди, злиті в такому порядку:

1. `mod/steamvr-lifecycle` (разом з ble-connection, power-state, power-buttons-ui)
2. `mod/linux-steamvr`: **конфлікт в `app.go`** (обробник «програму запустили вдруге»).
   Лишіть коментар з linux-steamvr і виклик `a.ShowFromTray()` зі steamvr-lifecycle.
3. `mod/i18n-uk`
4. `mod/about-links`
5. `mod/play-area`: **конфлікти в `TitleBar.tsx`, `en.json` і згенерованих
   `wailsjs/go/main/App.*`**. Лишіть обидві сторони (кнопки живлення *і* іконку мапи, усі
   ключі), а прив'язки перегенеруйте через `wails generate module`.
6. `mod/lighthouse-dfu`
7. `chore/wails-tooling`

Результат збігається з робочою версією від 2026-09-26 (гілка `snapshot/2026-09-26`), крім:
- того, що помічники play-area тепер лежать у `tools/` (з рядками в `.gitignore`), а не в
  домашній теці, і `playarea_linux.go` шукає їх там;
- пізнішого фіксу play-area: SteamVR отримує до 2 хв, щоб визначити всесвіт станцій, перш
  ніж застосувати авто-зону.

Перевірка: `git diff snapshot/2026-09-26 all-mods -- . ':!mods'`.

## Для автора програми

Автор попросив надсилати виправлення окремими PR
([PR #25](https://github.com/DHCPCD9/go-steamvr-lighthouse-manager/pull/25)). Кожна гілка
`mod/*`, крім play-area, lighthouse-dfu і chore/wails-tooling (особисті інструменти), —
самостійний кандидат на PR.
