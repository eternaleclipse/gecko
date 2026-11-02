VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# Package version: the latest release tag without its "v".
PKG_VERSION ?= $(or $(patsubst v%,%,$(shell git describe --tags --abbrev=0 2>/dev/null)),0.0.0)
# Build id: the commit (with -dirty for uncommitted changes) and when it
# was built, in the web UI, gecko and the Android app alike.
BUILD_COMMIT := $(or $(BUILD_COMMIT),$(shell c=$$(git rev-parse --short=7 HEAD 2>/dev/null || echo unknown); [ -n "$$(git status --porcelain 2>/dev/null)" ] && c="$$c-dirty"; echo $$c))
BUILD_TIME := $(or $(BUILD_TIME),$(shell date -u +%Y-%m-%dT%H:%M:%SZ))
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.buildCommit=$(BUILD_COMMIT) -X main.buildTime=$(BUILD_TIME)
GO ?= go
export GOTOOLCHAIN ?= local

PREFIX ?= $(HOME)/.local
DESKTOP_DIR := $(PREFIX)/share/gecko/desktop
.PHONY: all web build test test-pwa dist packages package-mac package-appimage package-windows install install-desktop desktop desktop-sandbox clean dev android android-gecko android-test ios-web ios ios-test

all: web build

web:
	cd web && npm install --no-audit --no-fund && GECKO_BUILD_COMMIT=$(BUILD_COMMIT) GECKO_BUILD_TIME=$(BUILD_TIME) node build.mjs

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/gecko ./cmd/gecko

test:
	$(GO) vet ./...
	$(GO) test ./...
	node --test web/src/gestures.test.mjs web/src/build.test.mjs web/src/ime.test.mjs web/src/themes.test.mjs web/src/transparency.test.mjs web/src/renderer.test.mjs web/src/anchor.test.mjs web/src/viewport.test.mjs

# The web app (PWA) on phones and tablets: Playwright with WebKit (iPhone)
# and Chromium (Android) profiles against a throwaway daemon (web/e2e).
# The browsers, once: cd web/e2e && npx playwright install --with-deps chromium webkit
# One test or project: make test-pwa PWA_ARGS="--project=iphone-15 -g pinch"
test-pwa: web build
	cd web/e2e && npm ci --no-audit --no-fund && npx playwright test $(PWA_ARGS)

# Cross-compiled binaries; `gecko host install` picks the right one, and
# the Termux installer downloads the Linux ones.
#
# The Linux binaries are static-PIE: they run on any Linux, and also where
# a program has to be started by Android's linker (/system/bin/linker64,
# as Termux from Google Play does), which loads only position-independent
# programs. Go's own linker can't make a static PIE, so these are linked by
# zig (https://ziglang.org, 0.13.0) against musl. Only the startup code and
# the linker come from there: net and os/user stay pure Go (netgo,
# osusergo), as in every other build. internal/staticpie (tag staticpie)
# has the entry point that lets Android's linker start them, and zigcc, the
# C compiler for Go. ZIG names the zig to use.
ZIG ?= zig
dist: web
	@command -v $(ZIG) >/dev/null 2>&1 || { echo "make dist needs zig 0.13.0 for the static-PIE Linux binaries: get it from https://ziglang.org/download/ and put it on PATH, or set ZIG=/path/to/zig" >&2; exit 1; }
	@mkdir -p dist
	for t in linux/amd64:x86_64 linux/arm64:aarch64; do \
		arch=$${t%:*}; arch=$${arch#*/}; \
		ZIG="$(ZIG)" CGO_ENABLED=1 GOOS=linux GOARCH=$$arch CC="$(CURDIR)/internal/staticpie/zigcc $${t#*:}-linux-musl" \
			$(GO) build -trimpath -tags netgo,osusergo,staticpie -buildmode=pie \
			-ldflags "$(LDFLAGS) -linkmode=external '-extldflags=-static-pie -Wl,-e,gecko_start -Wl,--strip-all'" \
			-o dist/gecko-linux-$$arch ./cmd/gecko || exit 1; \
	done
	for t in darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64; do \
		os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/gecko-$$os-$$arch$$ext ./cmd/gecko || exit 1; \
	done

# Installers: .deb/.rpm, a macOS app (zip) and Windows setup.exe, in
# dist/packages. One platform: make packages PLATFORMS=linux
packages: web
	GO="$(GO)" VERSION="$(VERSION)" PKG_VERSION="$(PKG_VERSION)" LDFLAGS="$(LDFLAGS)" packaging/build.sh $(PLATFORMS)

# Self-contained desktop apps (Electron window plus the gecko binary), in
# dist/desktop: see docs/desktop-packages.md. The dmg needs macOS.
package-mac package-appimage package-windows: web
	GO="$(GO)" VERSION="$(VERSION)" PKG_VERSION="$(PKG_VERSION)" LDFLAGS="$(LDFLAGS)" packaging/desktop/build.sh $(@:package-%=%)

# gecko goes to $(PREFIX)/bin/gecko (~/.local/bin/gecko by default), the
# first place Gecko's SSH connections look (see docs/install.md). DESTDIR
# stages the files for a package instead and leaves this machine alone.
install: all
	install -d $(DESTDIR)$(PREFIX)/bin
	@# Under another name, then renamed over the old one: a running gecko
	@# keeps its file (no "text file busy"), and nothing ever starts a
	@# half-copied binary.
	install -m 755 bin/gecko $(DESTDIR)$(PREFIX)/bin/.gecko.new
	mv -f $(DESTDIR)$(PREFIX)/bin/.gecko.new $(DESTDIR)$(PREFIX)/bin/gecko
	@# One gecko on this machine: every other copy becomes a link to this
	@# one, and a running service restarts with it, keeping its sessions.
	@if [ -z "$(DESTDIR)" ]; then $(PREFIX)/bin/gecko upgrade --installed || true; fi
	@if [ "$$(uname -s)" = Linux ]; then $(MAKE) --no-print-directory install-desktop; fi

# App-menu launcher (freedesktop: GNOME, KDE, ...), runs `gecko open`.
install-desktop:
	install -d $(DESTDIR)$(PREFIX)/share/applications $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps
	sed 's|@BIN@|$(PREFIX)/bin/gecko|' packaging/linux/gecko.desktop > $(DESTDIR)$(PREFIX)/share/applications/gecko.desktop
	install -m 644 packaging/linux/gecko.svg $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/gecko.svg
	@if [ -z "$(DESTDIR)" ]; then update-desktop-database $(PREFIX)/share/applications 2>/dev/null || true; fi
	@# GNOME reads icons through this cache; a stale one hides new or changed icons.
	@if [ -z "$(DESTDIR)" ] && { [ -f $(PREFIX)/share/icons/hicolor/icon-theme.cache ] || command -v gtk-update-icon-cache >/dev/null; }; then \
		gtk-update-icon-cache -f -t -q $(PREFIX)/share/icons/hicolor 2>/dev/null || true; fi

# Android app (android/): a debug APK at android/app/build/outputs/apk/debug.
# Needs JDK 17 and the Android SDK (ANDROID_HOME); see docs/android.md.
# The APK carries Gecko itself (the daemon and web UI) for the phone's own
# terminal, as lib/<abi>/libgecko.so: a static linux build runs on Android.
android: android-gecko
	cd android && ./gradlew --no-daemon assembleDebug
	@ls -l android/app/build/outputs/apk/debug/app-debug.apk

ANDROID_JNI := android/app/src/main/jniLibs
android-gecko: web
	@for t in arm64-v8a/arm64 x86_64/amd64; do \
		abi=$${t%/*}; arch=$${t#*/}; mkdir -p $(ANDROID_JNI)/$$abi; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(ANDROID_JNI)/$$abi/libgecko.so ./cmd/gecko || exit 1; \
	done
	@# The web UI, which the app serves for every machine (BundledUi.kt).
	@rm -rf android/app/src/main/assets/web && mkdir -p android/app/src/main/assets && cp -r web/dist android/app/src/main/assets/web && rm -f android/app/src/main/assets/web/.keep
	@# The same build id for the app (BuildConfig, see app/build.gradle.kts).
	@printf 'commit=%s\ntime=%s\n' '$(BUILD_COMMIT)' '$(BUILD_TIME)' > android/app/build-id.properties

# Its unit tests; `./gradlew connectedDebugAndroidTest` runs the device tests.
android-test:
	cd android && ./gradlew --no-daemon testDebugUnitTest

# iOS app (ios/): the web UI as one page for it, with the build id; any OS.
ios-web: web
	node ios/scripts/bundle-web.mjs web/dist ios/Gecko/Generated '$(BUILD_COMMIT)' '$(BUILD_TIME)'

# The app for the iOS Simulator (macOS with Xcode and XcodeGen; see docs/ios.md).
IOS_SIM ?= iPhone 16
ios: ios-web
	cd ios && xcodegen generate
	xcodebuild build -project ios/Gecko.xcodeproj -scheme Gecko -destination 'generic/platform=iOS Simulator' \
		-derivedDataPath ios/build
	@ls -d ios/build/Build/Products/Debug-iphonesimulator/Gecko.app

ios-test: ios-web
	cd ios && xcodegen generate
	xcodebuild test -project ios/Gecko.xcodeproj -scheme Gecko -destination 'platform=iOS Simulator,name=$(IOS_SIM)' \
		-derivedDataPath ios/build -only-testing:GeckoTests

# Native desktop app (Electron): see-through window with blur on macOS and
# Windows. `gecko open` uses it automatically once installed.
desktop:
	cd desktop && npm install --no-audit --no-fund && node node_modules/electron/install.js
	install -d $(DESKTOP_DIR) $(PREFIX)/bin
	cp packaging/linux/gecko-256.png desktop/icon.png
	rm -rf $(DESKTOP_DIR) && cp -a desktop $(DESKTOP_DIR)
	printf '#!/bin/sh\n[ "$$GECKO_DESKTOP_NO_SANDBOX" = 1 ] && set -- --no-sandbox "$$@"\nexec "%s/node_modules/electron/dist/electron" "%s" "$$@"\n' "$(DESKTOP_DIR)" "$(DESKTOP_DIR)" > $(PREFIX)/bin/gecko-desktop
	chmod +x $(PREFIX)/bin/gecko-desktop
	@echo "installed gecko-desktop; \`gecko open\` now uses it"
	@if [ "$$(cat /proc/sys/kernel/apparmor_restrict_unprivileged_userns 2>/dev/null)" = 1 ] && [ ! -f /etc/apparmor.d/gecko-desktop ]; then \
		echo "this system restricts the sandbox Electron needs; run once: make desktop-sandbox (uses sudo)"; fi

# One-time, needs sudo: lets the desktop app's sandbox work on Ubuntu 23.10+.
desktop-sandbox:
	sed 's|@ELECTRON@|$(DESKTOP_DIR)/node_modules/electron/dist/electron|' packaging/linux/apparmor-gecko-desktop | sudo tee /etc/apparmor.d/gecko-desktop >/dev/null
	sudo apparmor_parser -r /etc/apparmor.d/gecko-desktop
	@echo "done: gecko-desktop can now use its sandbox"

dev:
	cd web && node build.mjs --watch

clean:
	rm -rf bin dist web/dist/*.js web/dist/*.css web/dist/assets
