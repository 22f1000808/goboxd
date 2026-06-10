#!/bin/sh
# Install C++ compiler (G++). Already installed via the runtime apt block.
set -e
apt-get install -y --no-install-recommends g++
g++ --version
