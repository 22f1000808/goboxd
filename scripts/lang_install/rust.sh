#!/bin/sh
# Install Rust (rustc + cargo).
set -e
apt-get install -y --no-install-recommends rustc cargo
rustc --version
cargo --version
