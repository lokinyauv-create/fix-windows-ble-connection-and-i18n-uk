# mod: power-buttons-ui — explicit on/off controls that don't lie or misfire

**Branch:** `mod/power-buttons-ui` · **Base:** upstream `main` · **Depends on:** nothing
**Platforms:** all (frontend only) · **Commits:** 6 (2026-08-03 … 08-05)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

- Launching SteamVR sometimes sent "sleep" instead of "wake". The app also slept the
  stations right after it started.
- The group power button did nothing.
- Stations that can't report power state showed as asleep, and the single toggle
  could never turn them on (or never off).
- A second click during the ~30 s spin-up reversed the command and stopped the boot.
- Failed commands were silently dropped.

## Causes and fixes

| Cause | Fix |
|---|---|
| `previousSteamVRState` doubled as a flag for the manual toggle; the effect also ran on mount | Separate state; react only to real SteamVR transitions |
| Group `updatePowerState` was an empty loop | Wire it like the per-station control |
| Power state `-1` (unknown) treated as "off" | Separate **unknown** state with a neutral dot (`lib/powerState.ts`) |
| One toggle has to guess the direction when the state is unknown | **Separate on and off buttons** everywhere (station, group, header) |
| A flat 15 s cooldown dropped legitimate presses | Guard only while the command is in flight |
| The `ChangeBaseStationPowerStatus` wrapper didn't return its result | Return it. Manual buttons show errors, the SteamVR automation only logs |

## Portable recipe (any UI controlling devices with write-only state)

- If you can't read a device's state, **don't build a toggle**: use two explicit commands.
- Keep "unknown" as a first-class state, and never map it to "off".
- Automation that reacts to an external process (SteamVR) must fire only on **transitions**
  of that process, never on mount or on user actions.
- Block re-entry while a command is running rather than with a fixed timer.
- Always return backend errors to the caller, and decide per caller whether to show or log.

## Files

`frontend/src/components/{TitleBar,BaseStation,BaseStationGroups}.tsx`,
`frontend/src/lib/powerState.ts`, `frontend/src/lib/native/index.ts`, `locales/*.json`
("Turn on"/"Turn off"; the Ukrainian text is in mod/i18n-uk).

## Apply

```bash
git merge mod/power-buttons-ui
git am mods/power-buttons-ui/power-buttons-ui.patch   # on top of upstream main
```

## Verify

Three presses of "on" a few seconds apart all reach the stations (log: three
`Power command requested`). SteamVR start and stop wakes and sleeps the stations exactly once.
