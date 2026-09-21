#!/bin/bash
# Builds the Vulkan layer used by vr-room-setup. Needs gcc and vulkan-headers
# (/usr/include/vulkan/vk_layer.h).
set -e
cd "$(dirname "$(readlink -f "$0")")"
gcc -O2 -fPIC -shared -Wall -Wextra -fvisibility=hidden -o libVkLayer_vrfix_mipclamp.so mipclamp_layer.c
echo "built: $(pwd)/libVkLayer_vrfix_mipclamp.so"
