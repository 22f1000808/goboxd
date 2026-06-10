#!/bin/sh
# Install Ruby.
set -e
apt-get install -y --no-install-recommends ruby
ruby --version
