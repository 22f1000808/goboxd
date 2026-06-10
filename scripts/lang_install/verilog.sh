#!/bin/sh
# Install Icarus Verilog (iverilog + vvp).
set -e
apt-get install -y --no-install-recommends iverilog
iverilog -V
vvp -v 2>/dev/null || true
