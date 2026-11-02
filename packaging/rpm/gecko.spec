# Gecko for Fedora (and COPR, see .copr/Makefile and docs/linux-packages.md).
# gecko: the CLI and daemon, built from source (Go modules vendored and the
# web client prebuilt in Source0, so the build needs no network).
# gecko-desktop: the native window, Electron from the release tarball
# (Source1), since Fedora has no Electron to build it against.

%{!?gecko_version: %global gecko_version 0.3.3}
%global debug_package %{nil}
# Electron carries its own copies of a few libraries; they are not for
# other packages, and need nothing beyond what gecko-desktop requires.
%global __provides_exclude_from ^%{_prefix}/lib/gecko-desktop/.*$
%global __requires_exclude ^lib(ffmpeg|EGL|GLESv2|vk_swiftshader|vulkan)\\.so.*$

Name:           gecko
Version:        %{gecko_version}
Release:        1%{?dist}
Summary:        Terminal multiplexer for coding agents across your machines
# The project has not chosen a license yet (docs/linux-packages.md).
License:        LicenseRef-Unspecified
URL:            https://github.com/eternaleclipse/gecko
Source0:        %{url}/releases/download/v%{version}/gecko-%{version}-src.tar.gz
Source1:        %{url}/releases/download/v%{version}/gecko-%{version}-linux-x86_64.tar.gz
ExclusiveArch:  x86_64

BuildRequires:  golang >= 1.22
BuildRequires:  systemd-rpm-macros
BuildRequires:  desktop-file-utils
BuildRequires:  libappstream-glib
Recommends:     gecko-desktop = %{version}-%{release}
Suggests:       tmux
Suggests:       mosh

%description
Gecko keeps shells, tmux sessions, SSH connections and coding agents running
in a daemon and shows them in a web UI and a native window, on your desktop or
your phone. Sessions survive closing the window, and you can see at a glance
which agent is working and which one is waiting for you.

%package        desktop
Summary:        Native window for the Gecko terminal multiplexer
License:        LicenseRef-Unspecified AND MIT
Requires:       gecko = %{version}-%{release}
# Linked libraries are found automatically; these are loaded at runtime.
Requires:       gtk3
Requires:       nss
Requires:       mesa-libgbm
Requires:       at-spi2-core
Requires:       xdg-utils
Recommends:     libnotify
Recommends:     libsecret
Recommends:     libXScrnSaver
Recommends:     libXtst

%description    desktop
A see-through window around Gecko's web UI, without browser chrome in the
way (Electron). `gecko open` uses it once installed; opened from the app menu
it finds or starts the Gecko daemon.

%prep
%autosetup -n gecko-%{version}
tar -xzf %{SOURCE1}

%build
export CGO_ENABLED=0 GOFLAGS="-mod=vendor -trimpath" GOPROXY=off GOTOOLCHAIN=local
go build -ldflags "-s -w -X main.version=%{version}" -o gecko ./cmd/gecko

%install
install -Dm755 gecko %{buildroot}%{_bindir}/gecko
install -Dm644 packaging/linux/gecko.service %{buildroot}%{_userunitdir}/gecko.service
mkdir -p %{buildroot}%{_datadir}/applications
sed 's|@BIN@|%{_bindir}/gecko|' packaging/linux/gecko.desktop >%{buildroot}%{_datadir}/applications/gecko.desktop
install -Dm644 packaging/linux/gecko.svg %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/gecko.svg
install -Dm644 packaging/linux/gecko-256.png %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/gecko.png
install -Dm644 packaging/linux/gecko.metainfo.xml %{buildroot}%{_metainfodir}/dev.gecko.app.metainfo.xml
install -Dm644 packaging/linux/gecko.1 %{buildroot}%{_mandir}/man1/gecko.1
install -Dm644 packaging/linux/gecko-desktop.1 %{buildroot}%{_mandir}/man1/gecko-desktop.1
# The window, from the release tarball.
bin=gecko-%{version}-linux-x86_64
mkdir -p %{buildroot}%{_prefix}/lib
cp -a $bin/usr/lib/gecko-desktop %{buildroot}%{_prefix}/lib/
chmod 644 %{buildroot}%{_prefix}/lib/gecko-desktop/*.so*
install -Dm755 packaging/linux/gecko-desktop %{buildroot}%{_bindir}/gecko-desktop

%check
desktop-file-validate %{buildroot}%{_datadir}/applications/gecko.desktop
appstream-util validate-relax --nonet %{buildroot}%{_metainfodir}/dev.gecko.app.metainfo.xml
%{buildroot}%{_bindir}/gecko version

%files
%doc README.md docs/linux-packages.md docs/desktop-packages.md
%{_bindir}/gecko
%{_mandir}/man1/gecko.1*
%{_userunitdir}/gecko.service
%{_datadir}/applications/gecko.desktop
%{_datadir}/icons/hicolor/scalable/apps/gecko.svg
%{_datadir}/icons/hicolor/256x256/apps/gecko.png
%{_metainfodir}/dev.gecko.app.metainfo.xml

%files desktop
%license gecko-%{version}-linux-x86_64/usr/lib/gecko-desktop/LICENSE.electron.txt
%{_bindir}/gecko-desktop
%{_mandir}/man1/gecko-desktop.1*
%dir %{_prefix}/lib/gecko-desktop
%{_prefix}/lib/gecko-desktop/*
%exclude %{_prefix}/lib/gecko-desktop/chrome-sandbox
# Chromium's setuid sandbox helper, used where user namespaces are off.
%attr(4755,root,root) %{_prefix}/lib/gecko-desktop/chrome-sandbox

%changelog
* Mon Nov 02 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.3.3-1
- New upstream release 0.3.3

* Tue Oct 20 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.3.2-1
- New upstream release 0.3.2

* Tue Oct 20 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.3.1-1
- New upstream release 0.3.1

* Sun Oct 18 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.3.0-1
- New upstream release 0.3.0

* Sun Oct 18 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.2.2-1
- New upstream release 0.2.2

* Sun Oct 18 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.2.1-1
- New upstream release 0.2.1

* Sat Oct 17 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.2.0-1
- New upstream release 0.2.0

* Sat Oct 17 2026 Gecko contributors <gecko@users.noreply.github.com> - 0.1.0-1
- First package: gecko (CLI and daemon) and gecko-desktop (native window)
