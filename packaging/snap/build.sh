#!/bin/sh
# Builds the snap from the release tarball made by `make packages
# PLATFORMS=linux` (dist/packages/gecko-<ver>-linux-x86_64.tar.gz):
#   packaging/snap/build.sh 0.3.3   -> dist/packages/gecko_0.3.3_amd64.snap
# Uses snapcraft --destructive-mode, so run it on Ubuntu 24.04 (the core24
# base), in CI or a VM. Without it, snapcraft builds in an LXD container.
set -eu
cd "$(dirname "$0")/../.."
ver=${1:?usage: build.sh <version>}
dir=dist/build/snap
rm -rf "$dir" && mkdir -p "$dir/snap"
sed "s/@VERSION@/$ver/" packaging/snap/snapcraft.yaml >"$dir/snap/snapcraft.yaml"
cp -a packaging/snap/launchers "$dir/launchers"
mkdir -p "$dir/payload"
tar -xzf "dist/packages/gecko-$ver-linux-x86_64.tar.gz" -C "$dir/payload" --strip-components=1
(cd "$dir" && snapcraft pack ${SNAPCRAFT_ARGS:---destructive-mode})
mv "$dir"/gecko_*.snap dist/packages/
ls -l dist/packages/gecko_*.snap
