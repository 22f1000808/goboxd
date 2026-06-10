#!/bin/sh
# Install Java (OpenJDK 17).
set -e
apt-get install -y --no-install-recommends openjdk-17-jdk-headless
java -version
javac -version
