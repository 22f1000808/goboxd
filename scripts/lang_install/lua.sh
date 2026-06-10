#!/bin/sh
# Install Lua 5.4.
set -e
apt-get install -y --no-install-recommends lua5.4
lua5.4 -v
