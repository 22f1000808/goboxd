#!/bin/sh
# Install C compiler (GCC). Already installed via the runtime apt block.
set -e
apt-get install -y --no-install-recommends gcc libc-dev
gcc --version
