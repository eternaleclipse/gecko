#!/bin/sh
# A source tarball that builds without network: the committed tree, Go
# modules vendored and the web client already built (web/dist). For the
# Launchpad PPA (debian/), Fedora COPR (packaging/rpm) and anyone packaging
# Gecko offline. Needs `make web` first.
#   PKG_VERSION=0.3.3 packaging/linux/source-tarball.sh   -> dist/packages/gecko-0.3.3-src.tar.gz
set -eu
cd "$(dirname "$0")/../.."
GO=${GO:-go}
ver=${PKG_VERSION:?set PKG_VERSION}
name=gecko-$ver
[ -f web/dist/index.html ] || { echo "web/dist is empty: run make web first" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
git archive --prefix="$name/" HEAD | tar -x -C "$tmp"
(cd "$tmp/$name" && "$GO" mod vendor)
cp -a web/dist/. "$tmp/$name/web/dist/"
mkdir -p dist/packages
tar -C "$tmp" --owner=0 --group=0 --sort=name -czf "dist/packages/$name-src.tar.gz" "$name"
echo "created package: dist/packages/$name-src.tar.gz"
