# mod: lighthouse-dfu — update Lighthouse 2.0 base station firmware from Linux

**Branch:** `mod/lighthouse-dfu` · **Base:** upstream `main` · **Depends on:** nothing (standalone tool)
**Platforms:** Linux with BlueZ, Python 3 + `dbus_next` · **Commits:** 1 (2026-09-26)

[Українською](README.uk.md) · [All mods](../README.md)

## Problem

Stations on radio firmware 2.2 expose the power characteristic as write-only, so no app can
tell whether they really turned on (see mod/power-state). Updating them from Linux fails:

- SteamVR's updater (through the Index headset radio) **crashes vrmonitor** with
  `free(): invalid pointer` ([#653](https://github.com/ValveSoftware/SteamVR-for-Linux/issues/653),
  [#709](https://github.com/ValveSoftware/SteamVR-for-Linux/issues/709)).
- A generic Nordic *buttonless* DFU can't get in: Valve authenticates first ("LHB-Unlock"),
  so the enter-bootloader write just hangs.

## Solution (verified 2026-09-26 on two stations: radio 2.2 → 2.9.2004771, MCU 1.2 → 1.8)

1. Unplug the station's power, **hold the button on the back**, and plug the power back
   in. The LED stays off. The station now advertises as **`LHB-DFU`** at MAC+1.
2. Run `python3 tools/lighthouse-dfu/lh_dfu.py dfu`. It flashes the signed package SteamVR
   ships (`tools/lighthouse/firmware/lighthouse_tx_vader/archive/428/
   lighthouse_tx_vader_radio_full_dfu.v2_9_2004771.zip`) in about 2 min.
3. The station reboots on its own. Check with `lh_dfu.py info <MAC>`.

Only one station at a time. The bootloader verifies the signature. If the transfer is
interrupted, the station stays in `LHB-DFU`, and running `dfu` again finishes the job.
Close other BLE apps (e.g. this manager) first, because a station accepts one link at a time.

## Portable recipe

`lh_dfu.py` is a complete, dependency-light Nordic Secure DFU client over BlueZ D-Bus
(`dbus_next`): select/create/CRC/execute for the init packet and 4 KiB data objects, with a
per-object retry. It works for **any** nRF5 SDK device whose bootloader you can reach.
Change `DEFAULT_PACKAGE` and the name match in `cmd_dfu`.

## Apply

It's a standalone tool: copy `tools/lighthouse-dfu/lh_dfu.py`, or `git merge mod/lighthouse-dfu`.
