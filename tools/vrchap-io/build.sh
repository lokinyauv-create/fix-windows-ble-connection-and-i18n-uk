#!/bin/bash
# Builds vrchap-io, the helper the Room page uses to export/import the live
# SteamVR play area. Needs g++ and the OpenVR SDK header openvr.h:
#   OPENVR_INCLUDE=/path/to/openvr/headers ./build.sh
# Without OPENVR_INCLUDE the header of OpenVR v2.15.6 is downloaded into
# third_party/. libopenvr_api.so is taken from the SteamVR install
# (STEAMVR_DIR, default: native Steam's library).
set -e
cd "$(dirname "$(readlink -f "$0")")"
SV="${STEAMVR_DIR:-$HOME/.local/share/Steam/steamapps/common/SteamVR}"
LIB="$SV/bin/linux64"
INC="${OPENVR_INCLUDE:-}"
if [ -z "$INC" ]; then
    INC=third_party/openvr
    if [ ! -f "$INC/openvr.h" ]; then
        mkdir -p "$INC"
        curl -fsSL -o "$INC/openvr.h" https://raw.githubusercontent.com/ValveSoftware/openvr/v2.15.6/headers/openvr.h
    fi
fi
g++ -O2 -std=c++17 -Wall -Wextra -I "$INC" vrchap-io.cpp -o vrchap-io -L"$LIB" -lopenvr_api -Wl,-rpath,"$LIB"
echo "built: $(pwd)/vrchap-io"
