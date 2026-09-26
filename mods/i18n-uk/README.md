# mod: i18n-uk — Ukrainian translation

**Branch:** `mod/i18n-uk` · **Base:** upstream `main` · **Depends on:** nothing
**Platforms:** all · **Commits:** 2 (2026-08-03, 2026-09-26)

[Українською](README.uk.md) · [All mods](../README.md)

## What it does

Adds `frontend/src/localization/locales/ua.json` and registers it in `i18n.js` (the
language picker already had a name for `ua`, just no translation behind it). The file also carries the Ukrainian text for strings
that other mods add ("Turn on"/"Turn off" from power-buttons-ui, the Room page from
play-area). This way none of those mods needs this one, and unused keys are harmless.

## Portable recipe (react-i18next / i18next)

1. Copy `ua.json` next to the other locale files.
2. `import Ukrainian from './locales/ua.json'` and add `ua: { translation: Ukrainian }` to
   `resources`.
3. If your language picker has a name map, add `ua: "Українська"`.
4. Keys are the English source strings, so any app using the same keys can reuse the file
   as is.

## Apply

```bash
git merge mod/i18n-uk
git am mods/i18n-uk/i18n-uk.patch   # on top of upstream main
```
