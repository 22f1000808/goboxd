#!/bin/sh
# Install Go language toolchain.
# Debian bookworm ships golang-go 1.19; we install from system packages
# which is sufficient for sandbox execution.
set -e
apt-get install -y --no-install-recommends golang-go
# golang-go on Debian installs to /usr/bin/go
/usr/bin/go version
