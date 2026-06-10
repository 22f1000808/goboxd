#!/bin/sh
# Install Python 3. Already installed via the runtime apt block.
set -e
apt-get install -y --no-install-recommends python3
python3 --version
