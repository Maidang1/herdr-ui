# Linux support

Linux amd64 (x86_64) and arm64 (aarch64) builds use the pinned mygo GTK 3
backend and the same Unix socket / `herdr terminal attach` path as macOS.
The default terminal is the PTY-backed Ghostty terminal plugin, not the
Windows endpoint-cell renderer. Herdr must already be installed and running;
launching the application does not start or take over your daemon.

The validation baseline is Ubuntu 22.04 or newer, with glibc and GTK 3. Other
Linux distributions with these libraries may work, but are not the CI baseline.
Musl-only systems such as stock Alpine, 32-bit CPUs and headless desktop use
are not supported. The application requires a graphical X11 or Wayland session.
It does not need WebKitGTK: both the main interface and updater use native UI.

## Install

Choose the architecture matching your machine (`uname -m`). Release filenames
use `amd64` for x86_64 and `arm64` for aarch64.

### Debian / Ubuntu package

Download the matching package and checksum file from the same GitHub release:

```sh
sha256sum -c Hergo-<version>-linux-<arch>-SHA256SUMS --ignore-missing
sudo apt install ./hergo_<version>_<arch>.deb
herdr-ui
```

The package installs the application and bundled terminal library in
`/opt/hergo`, adds `/usr/bin/herdr-ui`, and registers its icon and desktop
entry. APT installs GTK 3 (`libgtk-3-0` or `libgtk-3-0t64`). This is not an APT
repository: install the new release's `.deb` to update. Normal users cannot
replace the system-owned application, so background in-app updates are disabled
for that installation. Do not run the GUI as root to enable updates.

### User-local / portable installation

On Ubuntu, install GTK 3 if it is not already available:

```sh
sudo apt install libgtk-3-0
```

Ubuntu 24.04+ uses the package name `libgtk-3-0t64`. Other distributions should
install their GTK 3 runtime package. CJK/emoji font packages are recommended.
`lsof` is optional: it enables the sidebar's local listening-port discovery.

Download `install.sh`, the matching `hergo-<version>-linux-<arch>.tar.gz`, and
its checksum file from the same release. Keep the original archive filename;
signed update manifests refer to that exact name.

```sh
sha256sum -c Hergo-<version>-linux-<arch>-SHA256SUMS --ignore-missing
sh install.sh ./hergo-<version>-linux-<arch>.tar.gz
herdr-ui
```

Run the installer as your normal user, not with sudo. It installs under
`~/.local/hergo.app`, adds `~/.local/bin/herdr-ui` and registers a desktop
entry. Ensure `~/.local/bin` is on PATH, or launch from the applications menu.
A writable user-local installation supports the native signed updater using
`update-linux-amd64.json` or `update-linux-arm64.json` and the existing Ed25519
public key. No signing private key is stored in the application.

For portable use, extract the archive into a writable directory and run
`./hergo` there. Keep all resources, particularly `libghostty-vt.so`, beside
the executable. Packaged applications never download that library at startup.

`install-linux-amd64.sh` and `install-linux-arm64.sh` are identical copies of
`install.sh`, retained so each architecture's checksum file can verify its
installer. One architecture-detecting `install.sh` is published after comparing
the two generated scripts.

## Build and test

Use the Go version in `go.mod`, Python 3.10+ and the repository wrappers; these
prepare the pinned mygo patches without changing the module cache.

```sh
make test        # Python release contracts, Go tests and vet
make build       # Executables for the current platform
make linux       # Both Linux architectures: executable, .deb, .tar.gz, install.sh
make window      # /bin/cat fixture for IME / clipboard checks
```

To package only your architecture:

```sh
CGO_ENABLED=0 ./scripts/go.sh run github.com/egoist/mygo/cmd/mygo build -platform linux/amd64
# Or: -platform linux/arm64
```

Outputs are in `dist/linux-amd64` and `dist/linux-arm64`. Cross-compilation and
packaging work from macOS too; they do not validate Linux runtime behavior.
Unsigned local packages omit signed update manifests. Release CI requires
`MYGO_UPDATER_PRIVATE_KEY` and fails if signed update artifacts are absent.

### Isolated integration validation

Integration tests never connect to your existing session. Linux requires an
explicit Herdr executable rather than discovering your normal installation:

```sh
python3 scripts/install-herdr-linux.py --arch amd64 --destination /tmp/herdr-ui-runtime
export HERDR_TEST_BINARY=/tmp/herdr-ui-runtime/herdr
make integration
```

Choose a destination that does not exist. The downloader uses official Herdr
0.9.5 binaries and fixed SHA-256 digests:

- amd64: `14d9e8bc8b4a9e0118d77c32266f76d461f09b40342eb23752960bd89f59929d`
- arm64: `17d7536f2da50859523680ef279dffcac8e4a235f4e46fe8da6c2ec44141897c`

Each integration test creates an isolated socket, configuration and daemon,
removes inherited Herdr pane/session identity from child environments, and stops
only that explicit daemon on cleanup. Tests cover PTY streaming, single-owner
attach rejection, resize, close/reconnect, event snapshots, nested layout
operations, shortcut operations and multi-pane native-window lifecycle.

The Release workflow runs both architectures natively on `ubuntu-22.04` and
`ubuntu-22.04-arm`. It uses Xvfb, Openbox and a private D-Bus session for window
tests, checks Debian dependencies and bundled terminal resources, then smoke
tests the packaged terminal. A version tag publishes only after macOS, Windows
and both Linux jobs pass. Manual workflow runs retain artifacts without
publishing a GitHub release.

## Paths and shortcuts

Linux honors `HERDR_CONFIG_PATH`, `XDG_CONFIG_HOME`, `HERDR_SESSION` and
`HERDR_SOCKET_PATH`; `-socket` takes precedence. By default the socket is
`~/.config/herdr/herdr.sock`. The desktop launcher recovers shell PATH when
necessary, including common `~/.local/bin` and `~/.cargo/bin` installations.

Application commands use Ctrl+Shift so ordinary shell Ctrl+C/D/A/T remain
available. New tab is Ctrl+Shift+T, vertical split Ctrl+Shift+D, horizontal split
Ctrl+Alt+D, tab navigation Ctrl+PageUp/PageDown, numbered tabs Ctrl+Alt+1-9,
pane navigation Ctrl+Alt+arrows, search Ctrl+Shift+F and settings Ctrl+Shift+comma.
Herdr's configured prefix chords remain available. A desktop environment may
reserve Ctrl+Alt+arrows; use the pane menus or Herdr prefix navigation in that
case. macOS and Windows bindings are unchanged.

## Validation status and manual checks

Local macOS validation confirms tests/vet, cross-compilation of the app and
integration suite for both Linux architectures, and actual `.deb`/archive
contents. Linux-native CI and physical desktop results must be observed before
claiming runtime support fully validated; adding a CI job is not a passing run.

Xvfb validates X11 automation only, not physical IME or Wayland. Manually check
GNOME/KDE on X11 and Wayland: IBus/Fcitx CJK composition, candidate placement,
commit/cancel, AltGr and keyboard layouts, CJK/emoji glyphs, clipboard paste,
drag selection, mouse reporting, context menus, focus, fractional scaling,
window resizing, notifications and opening/revealing files. Test full-screen
terminal applications and desktop-reserved shortcuts. Launcher badges and
notification actions depend on the desktop environment's capabilities.
