# Linux packages

Gecko ships as two packages wherever the format allows:

- **gecko**: the `gecko` command and daemon (one static Go binary), the app
  menu entry, icons, AppStream metadata and a systemd user unit.
- **gecko-desktop**: the native window (Electron, about 120 MB) and
  `/usr/bin/gecko-desktop`. `gecko` recommends it, so `apt install gecko` and
  `dnf install gecko` bring it along by default. Without it, `gecko open`
  uses a browser app window, as before.

| Channel | Package | Built from | Status |
| --- | --- | --- | --- |
| Gecko apt repository (GitHub Pages) | gecko, gecko-desktop (amd64, arm64) | nfpm, `make packages PLATFORMS=linux` | ready, needs a signing key |
| Launchpad PPA | gecko (CLI and daemon only) | `packaging/debian`, offline source | ready, needs a Launchpad account and a license |
| Snap Store | gecko (CLI, daemon and window), classic | `packaging/snap` | ready, needs the name and a classic review |
| Fedora COPR | gecko, gecko-desktop (x86_64) | `packaging/rpm/gecko.spec`, `.copr/Makefile` | ready, needs a COPR project |
| AUR | gecko-bin (prebuilt), gecko (source, system electron) | `packaging/arch` | ready, needs an AUR account |
| AppImage | everything in one file | `make package-appimage` | see [desktop-packages.md](desktop-packages.md) |

Official inclusion (Debian/Ubuntu archive, Fedora, Arch [extra]) is a later
step: see the end of this page.

## Before publishing anywhere: a license

The repository has no license yet, so every package says
`LicenseRef-Unspecified`. Launchpad PPAs require a free license, Fedora and
Arch only take free software, and the Snap Store and AUR expect one. Pick one
(MIT and Apache-2.0 are common for tools like this), add `LICENSE` at the
root, and replace `LicenseRef-Unspecified` in:
`packaging/linux/nfpm-gecko.yaml`, `packaging/linux/nfpm-gecko-desktop.yaml`,
`packaging/linux/copyright`, `packaging/debian/copyright`,
`packaging/rpm/gecko.spec` (and add `%license LICENSE`),
`packaging/arch/*/PKGBUILD` and `.SRCINFO` (and install the license file),
`packaging/snap/snapcraft.yaml` (add `license: <SPDX id>`).

## Building

```sh
make packages PLATFORMS=linux     # dist/packages: .deb and .rpm (gecko, gecko-desktop),
                                  # gecko-<ver>-linux-{x86_64,aarch64}.tar.gz
LINUX_ARCHS=amd64 make packages PLATFORMS=linux     # one arch
PKG_VERSION=0.3.3 packaging/linux/source-tarball.sh # gecko-<ver>-src.tar.gz (offline source)
packaging/snap/build.sh 0.3.3     # gecko_<ver>_amd64.snap (Ubuntu 24.04)
```

The version is the latest `v*` tag. CI (`.github/workflows/linux-packages.yml`)
builds all of it on every push to `feature/desktop-apps`, and installs and
smoke-tests each package on its distro: Ubuntu 22.04 and 24.04, Debian 12 and
13 (from a signed apt repository, so the `Recommends` and signatures are
tested too), Fedora (rpmbuild from the spec, rpmlint), Arch (makepkg,
namcap), the PPA source package (built offline, lintian) and the snap
(snapcraft, installed with snapd). The smoke test (`packaging/linux/smoke.sh`)
runs the CLI, a daemon, a session, and the window under Xvfb.

### What the packages install

```
/usr/bin/gecko                          CLI and daemon
/usr/bin/gecko-desktop                  the window (gecko-desktop)
/usr/lib/gecko-desktop/                 Electron and the app (gecko-desktop)
/usr/lib/systemd/user/gecko.service     the daemon as a user service, not enabled
/usr/share/applications/gecko.desktop   runs `gecko open`
/usr/share/icons/hicolor/.../gecko.*, /usr/share/metainfo/dev.gecko.app.metainfo.xml
/etc/apparmor.d/gecko-desktop-package   (deb) lets the window use Chromium's
                                        sandbox on Ubuntu 23.10+
```

Start at login works as it does without packages: the first plain `gecko`
writes `~/.config/systemd/user/gecko.service` and enables it, and that file
takes precedence over the packaged unit of the same name, so nothing is
registered twice. Enabling the packaged one by hand works too:
`systemctl --user enable --now gecko`. Opening Gecko from the app menu runs
`gecko open`, which starts the daemon if needed but never sets up login
start.

## Gecko apt repository (GitHub Pages)

A signed apt repository (`dists/stable`, amd64 and arm64) served from the
repository's GitHub Pages, built by `packaging/linux/apt-repo.sh`.

You need:
1. A signing key, made once on your machine, never committed:
   ```sh
   gpg --quick-gen-key "Gecko apt repository <you@example.com>" ed25519 sign 3y
   gpg --armor --export-secret-keys <fingerprint>   # paste into the secret below
   ```
2. A repository secret `APT_SIGNING_KEY` with that armored private key
   (Settings > Secrets and variables > Actions).
3. GitHub Pages enabled with source "GitHub Actions" (Settings > Pages).

Publish (builds, tests, signs and deploys):

```sh
gh workflow run linux-packages.yml --ref <branch> -f publish-apt=true
```

Users then follow the page at `https://eternaleclipse.github.io/gecko/`:

```sh
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://eternaleclipse.github.io/gecko/gecko-archive-keyring.gpg | sudo tee /etc/apt/keyrings/gecko.gpg >/dev/null
echo "deb [signed-by=/etc/apt/keyrings/gecko.gpg] https://eternaleclipse.github.io/gecko stable main" | sudo tee /etc/apt/sources.list.d/gecko.list
sudo apt update && sudo apt install gecko
```

Note that a Pages deploy replaces the whole site; if the repository later
serves other pages, put the apt repository in a subdirectory.

## Launchpad PPA (Ubuntu)

Launchpad builds from a source package, without network. The orig tarball
from `packaging/linux/source-tarball.sh` has the Go modules vendored and the
web client already built, and `packaging/debian` builds `gecko` with
`golang-1.24-go` (in Ubuntu 24.04's updates and later series; 24.04's default
Go, 1.22.2, is older than go.mod's 1.22.5).

The trade-off: the PPA carries only **gecko** (CLI and daemon). Electron
can't be built on Launchpad (its build needs network and many hours), and
uploading a prebuilt Electron in a source package is against PPA policy in
spirit and huge. So on a PPA install, `gecko open` uses a browser app window;
for the native window, add the Gecko apt repository (its `gecko-desktop`
depends on `gecko`, which either source satisfies) or use the snap. Same
package name and file layout as the apt repository's `gecko`, so the two mix.

You need:
1. A Launchpad account with your OpenPGP key registered and the Ubuntu Code of
   Conduct signed (Launchpad > your profile).
2. A PPA, e.g. `ppa:<you>/gecko` (Launchpad > Create a new PPA).
3. `devscripts`, `debhelper`, `golang-1.24-go` and `dput` on an Ubuntu machine.
4. A license (above): PPAs are for free software.

Publish (per Ubuntu series, here noble; repeat with `oracular` etc. and a
new `~<series>1` suffix):

```sh
v=0.3.3 series=noble
make web && PKG_VERSION=$v packaging/linux/source-tarball.sh
mkdir -p ppa && cd ppa && cp ../dist/packages/gecko-$v-src.tar.gz gecko_$v.orig.tar.gz
tar -xzf gecko_$v.orig.tar.gz && cp -a ../packaging/debian gecko-$v/debian && cd gecko-$v
dch -v "$v-1~${series}1" -D $series "Gecko $v for $series." && debuild -S -sa -k<your key id>
cd .. && dput ppa:<you>/gecko gecko_$v-1~${series}1_source.changes
```

## Snap Store

`packaging/snap/snapcraft.yaml`: the CLI, daemon and window in one snap, apps
`gecko` and `gecko.desktop` (the app menu entry).

**Classic confinement** is needed. Gecko is a terminal: the shells it starts
must see the user's real system (their tools in `/usr/bin`, `~/.ssh`, git,
compilers, coding agents like Claude Code), run anything the user runs, and
reach other machines over SSH. A strictly confined snap would run those
shells inside the snap's own read-only root with only the snap's tools, which
makes it a different, much less useful product. Classic snaps need a manual
review: after the first upload, request classic confinement on the Snapcraft
forum (category "store-requests") and explain the above; terminal emulators
and multiplexers are an accepted use case.

Notes: the window runs Electron's sandbox where the kernel allows user
namespaces, and `--no-sandbox` (with a one-time note) where Ubuntu restricts
them, as in the AppImage; a snap can't carry Chromium's setuid helper. Shells
started by a snap-installed daemon see `$SNAP/bin` on `PATH` (only
`gecko-desktop` is there) and snapd's `SNAP_*` variables.

You need:
1. An Ubuntu One account, and `snapcraft login` on your machine.
2. The name: `snapcraft register gecko` (if taken, pick another name and
   change `name:` in snapcraft.yaml).
3. The classic confinement approval (above).

Publish (from Ubuntu 24.04, or take the snap from the CI run's `snap`
artifact):

```sh
make packages PLATFORMS=linux LINUX_ARCHS=amd64 && packaging/snap/build.sh 0.3.3
snapcraft upload --release=stable dist/packages/gecko_0.3.3_amd64.snap
```

## Fedora COPR

`packaging/rpm/gecko.spec` builds `gecko` from source (offline, vendored
modules) and `gecko-desktop` from the release tarball's prebuilt Electron, as
Fedora has no Electron to build against. COPR builds from `.copr/Makefile`,
which makes the source tarball from the git checkout and downloads the release
tarball for that version (so the GitHub release must exist first, with
`gecko-<ver>-linux-x86_64.tar.gz` attached). x86_64 only for now.

You need:
1. A Fedora account (accounts.fedoraproject.org) and a COPR API token in
   `~/.config/copr` (copr.fedorainfracloud.org > API).
2. `copr-cli` (`dnf install copr-cli`).
3. A project: `copr-cli create gecko --chroot fedora-43-x86_64 --chroot fedora-42-x86_64`.

Publish:

```sh
copr-cli buildscm gecko --clone-url https://github.com/eternaleclipse/gecko.git --commit v0.3.3 --method make_srpm --spec packaging/rpm/gecko.spec
```

Users: `sudo dnf copr enable <you>/gecko && sudo dnf install gecko`.

## AUR (Arch Linux)

Two PKGBUILDs in `packaging/arch`, each with its `.SRCINFO`:

- **gecko-bin**: the release tarball, prebuilt gecko and bundled Electron.
- **gecko**: built from the source tag with Go and npm; the window runs on
  Arch's `electron` package.

They need the GitHub release with the tarballs; `packaging/arch/update.sh
<version>` sets the version and real checksums (the committed ones say SKIP
until a release exists) and regenerates `.SRCINFO` (needs makepkg).

You need:
1. An AUR account (aur.archlinux.org) with your SSH public key.
2. The names `gecko-bin` and `gecko` free on the AUR (check before the first
   push; rename the package if one is taken).

Publish (once per package; the AUR repository is created on first push):

```sh
git clone ssh://aur@aur.archlinux.org/gecko-bin.git aur-gecko-bin
cp packaging/arch/gecko-bin/PKGBUILD packaging/arch/gecko-bin/.SRCINFO aur-gecko-bin/
cd aur-gecko-bin && git add PKGBUILD .SRCINFO && git commit -m "gecko-bin 0.3.3-1" && git push
```

## Official distro repositories

- **Debian / Ubuntu archive**: needs a Debian Developer to sponsor the upload
  (or Ubuntu's MOTU via Debian), a DFSG license, and every vendored Go module
  packaged separately; the web client must build from source in Debian. The
  CLI package is a realistic candidate; the Electron window is not (Debian
  has no maintained Electron).
- **Fedora**: a package review (bugzilla "Review Request") by a packager
  sponsor, with Go modules unbundled per the Go packaging guidelines. Again
  the CLI only; bundled Electron is not allowed.
- **Arch [extra]**: only Package Maintainers add packages, so one has to
  adopt Gecko (usually after it is popular on the AUR).
