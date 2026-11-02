# Draft recipe for termux-packages (https://github.com/termux/termux-packages),
# as packages/gecko/build.sh there. Not submitted yet; before it can be:
#   - the repository needs a license (TERMUX_PKG_LICENSE must name it),
#   - the 0.2.0 release (the first with the Termux fixes) published, with its
#     tarball's sha256 below.
# Test it in a termux-packages checkout: ./scripts/run-docker.sh ./build-package.sh -a aarch64 gecko
TERMUX_PKG_HOMEPAGE=https://github.com/eternaleclipse/gecko
TERMUX_PKG_DESCRIPTION="Terminal for shells, tmux, ssh and coding agents, in a browser or the Gecko app"
TERMUX_PKG_LICENSE="custom"
TERMUX_PKG_MAINTAINER="@termux"
TERMUX_PKG_VERSION="0.3.3"
TERMUX_PKG_SRCURL=https://github.com/eternaleclipse/gecko/archive/refs/tags/v${TERMUX_PKG_VERSION}.tar.gz
TERMUX_PKG_SHA256=SKIP_CHECKSUM
TERMUX_PKG_RECOMMENDS="git, openssh, tmux"
TERMUX_PKG_BUILD_IN_SRC=true
TERMUX_PKG_AUTO_UPDATE=true
# Gecko is a static Go program with its web UI built in: one binary.
TERMUX_PKG_BLACKLISTED_ARCHES="arm, i686"

termux_step_make() {
	termux_setup_golang
	termux_setup_nodejs

	# The web UI, embedded into the binary (web/dist).
	pushd web
	npm install --no-audit --no-fund
	node build.mjs
	popd

	# GOOS=android (termux_setup_golang's default) with cgo: names resolve
	# through Android's resolver, and Gecko knows it runs on Android.
	go build -trimpath \
		-ldflags "-s -w -X main.version=v$TERMUX_PKG_VERSION" \
		-o gecko ./cmd/gecko
}

termux_step_make_install() {
	install -Dm700 -t "$TERMUX_PREFIX/bin" gecko
}
