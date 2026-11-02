#!/bin/sh
# Sets the version and checksums of both PKGBUILDs for a release and
# regenerates .SRCINFO (needs makepkg, i.e. Arch or an archlinux container).
# The release's tarballs must be on GitHub already.
#   packaging/arch/update.sh 0.3.3
set -eu
cd "$(dirname "$0")"
ver=${1:?usage: update.sh <version>}
url=https://github.com/eternaleclipse/gecko
sum() { curl -fsSL "$1" | sha256sum | cut -d' ' -f1; }
for p in gecko-bin gecko; do
	sed -i -e "s/^pkgver=.*/pkgver=$ver/" -e "s/^pkgrel=.*/pkgrel=1/" $p/PKGBUILD
done
sed -i -e "s/^sha256sums_x86_64=.*/sha256sums_x86_64=('$(sum "$url/releases/download/v$ver/gecko-$ver-linux-x86_64.tar.gz")')/" \
	-e "s/^sha256sums_aarch64=.*/sha256sums_aarch64=('$(sum "$url/releases/download/v$ver/gecko-$ver-linux-aarch64.tar.gz")')/" gecko-bin/PKGBUILD
sed -i "s/^sha256sums=.*/sha256sums=('$(sum "$url/archive/v$ver.tar.gz")')/" gecko/PKGBUILD
for p in gecko-bin gecko; do (cd $p && makepkg --printsrcinfo >.SRCINFO); done
echo "updated to $ver; review with git diff, then push each directory to its AUR repo"
