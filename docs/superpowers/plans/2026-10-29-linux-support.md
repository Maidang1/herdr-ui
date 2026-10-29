# Linux Support Implementation Plan

**Goal:** Ship Linux amd64/arm64 desktop builds using the existing Unix attach terminal.

**Architecture:** Keep the pinned mygo GTK backend, Unix socket transport and PTY terminal. Use mygo's native updater, Debian packager and signed portable archive; no WebKit dependency. Keep all three platforms behind the same release publication gate.

**Tech Stack:** Go, mygo, GTK 3, Python standard library, GitHub Actions.

## Tasks

- [x] Add release-contract tests for Linux packaging configuration, matrix and publication gate; verify failure before configuration changes.
- [x] Configure `mygo.json` with Linux native-only packaging and `herdr-ui` command. Add `make linux` for both CPU targets.
- [x] Add a checksum-pinned Linux Herdr downloader with bounded downloads, atomic publication and no destination overwrite; test with local fixture responses, including failure cases.
- [x] Enable the existing isolated Unix integration suite on Linux. Require an explicit Linux test executable; keep macOS discovery unchanged. Exercise real PTY attach ownership, resizing, reconnect, event snapshots, layout and window lifecycle under Xvfb.
- [x] Add native Ubuntu matrix jobs: test, vet, build, isolated integration, Debian/portable packaging, bundled terminal-library checks and signed update artifacts. Publish only after macOS, Windows and both Linux jobs pass. Preserve original update archive names/URLs and publish one identical `install.sh`.
- [x] Document GTK dependencies, user-local installation versus package-manager updates, supported platforms and outstanding X11/Wayland desktop checks.
- [x] Run local Go/Python tests, vet, amd64/arm64 cross-builds and package inspection. Linux native execution is CI-only on this macOS host unless a Linux runtime becomes available; do not claim it passed locally.

## Validation commands

- `python3 -m unittest discover -s scripts -p 'test_*.py' -v`
- `make test`
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ./scripts/go.sh build -o /tmp/herdr-ui-linux-amd64 ./cmd/herdr-ui`
- `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 ./scripts/go.sh build -o /tmp/herdr-ui-linux-arm64 ./cmd/herdr-ui`
- Compile tagged integration tests for both architectures without executing them.
- `make linux`: inspect `.deb` control dependencies, archive resources and installer script; unsigned local packages do not validate production signing.

## Verification results

- Nine Python downloader/release-contract checks passed (initially failed before implementation).
- `make test` passed with Go tests and vet outside the sandbox; the sandbox initially prevented the existing TCP-listener test.
- `make linux` produced both architectures' executable, `.deb`, portable archive and installer. Terminal libraries were downloaded and verified by the pinned mygo packager.
- Inspected both Debian control/data archives: correct architecture, GTK-only dependency, bundled terminal library, desktop entry and command link. Portable archives include the terminal library and licenses; both install scripts match byte-for-byte.
- Linux tagged integration binaries compiled for amd64 and arm64. Linux execution and signed release verification remain CI checks, not locally observed results.
- `actionlint .github/workflows/release.yml` and `git diff --check` passed.
