#!/bin/sh
# Install Node.js (JavaScript runtime).
set -e
apt-get install -y --no-install-recommends nodejs
node --version
