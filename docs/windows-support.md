# Windows native surface support

The Windows frontend keeps the existing mygo interface and renders the real Herdr daemon's native endpoint surface. It sends semantic key, Unicode text, paste, mouse and resize events to that endpoint. It does not start a terminal attach process or feed rendered screen characters back into a shell.

The implementation targets Windows x64 and Herdr 0.9.5. The Windows job in the [Release workflow](https://github.com/Maidang1/herdr-ui/actions/workflows/release.yml) runs on `windows-2025`. Native tests, vet, executable compilation, the real-daemon integration test, installer packaging and portable ZIP creation first passed together in [run 40747889447](https://github.com/Maidang1/herdr-ui/actions/runs/40747889447). This validates the automated Windows transport and terminal path; physical desktop UI and IME checks remain listed below.

## Build and test

Use a Go installation matching `go.mod` and Python 3.10 or newer. Run the Python wrapper so native Windows builds use the same pinned mygo patches as Unix builds:

```powershell
python scripts/go.py test ./...
python scripts/go.py vet ./...
python scripts/go.py build -trimpath -o artifacts/herdr-ui.exe ./cmd/herdr-ui
python scripts/go.py run github.com/egoist/mygo/cmd/mygo build -platform windows/amd64
```

The pinned mygo CLI has no `-skip-installer` option. Its supported build command packages the executable and resources in `dist/windows-amd64`, and downloads NSIS when needed to produce an installer. CI also archives a portable ZIP and creates signed update archives and a manifest using `MYGO_UPDATER_PRIVATE_KEY`. Pushing a `vX.Y.Z` tag matching `mygo.json` builds macOS, Windows and Linux in parallel; only after all platform jobs succeed does one publish job upload both platforms to the same GitHub Release. Windows assets include `Hergo-<version>-windows-amd64-setup.exe`, `Hergo-<version>-windows-amd64.zip`, a platform-specific SHA-256 checksum file, and signed update files. Manual runs only upload workflow artifacts, even when dispatched on a tag.

## Isolated real-daemon test

The integration test is opt-in and requires an explicit executable path. It never discovers or connects to your normal Herdr session:

```powershell
python scripts/install-herdr-windows.py --destination "$env:TEMP/herdr-ui-test-runtime"
$env:HERDR_TEST_BINARY = "$env:TEMP/herdr-ui-test-runtime/herdr.exe"
python scripts/go.py test -tags integration ./integration -run '^TestWindowsNativeSurface$' -count=1 -v -timeout 3m
```

The downloader prints the exact installed executable path; use that path if the archive contains a subdirectory. The destination must not already exist. In GitHub Actions the downloader also writes `HERDR_TEST_BINARY` to `GITHUB_ENV` for subsequent steps.

The downloader uses the official [Herdr 0.9.5 Windows x64 release](https://github.com/herdrdev/herdr/releases/tag/v0.9.5), verifies SHA-256 `d2c6c9c971c43a0bfdde8c692a9a1a7a45c3e4807cd01ae45662b4734fac1cbf`, limits download and extraction sizes, and rejects unsafe archive paths and symlinks.

The integration test creates its own temporary config, state directories, logical socket paths and daemon process. It removes inherited Herdr identity and home/config variables from the child environment. It verifies the actual Windows named pipe, negotiates the native surface endpoint, creates a PowerShell pane, sends CJK and emoji through semantic TextCommit followed by semantic Enter, checks the resulting output cells, resizes the surface, and reconnects while retaining the same daemon boot and output. Cleanup stops only the daemon at the test's explicit temporary target. No process-name kill or user-session stop command is used.

## Shortcuts and paths

On Windows, app commands use Ctrl+Shift so ordinary terminal Ctrl+C, Ctrl+V, Ctrl+A, Ctrl+D and Ctrl+T remain available. Vertical split uses Ctrl+Shift+D and horizontal split uses Ctrl+Shift+E. Tabs use Ctrl+PageUp/PageDown; tab numbers use Ctrl+Shift+1 through Ctrl+Shift+9. Pane focus uses Ctrl+Shift+arrows, pane cycling uses Ctrl+Shift+PageUp/PageDown, previous search match uses Ctrl+Shift+H, and Zen mode uses Ctrl+Shift+Z. Windows global app accelerators avoid Ctrl+Alt because Windows reports AltGr text input with those modifiers. They also avoid the Win/Super key.

Linux packaging and validation are documented in [Linux support](linux-support.md). On Linux, horizontal split uses Ctrl+Alt+D, tab numbers use Ctrl+Alt+1 through Ctrl+Alt+9, and pane focus uses Ctrl+Alt+arrows. Herdr's configured prefix chords remain available on both platforms. macOS retains its existing Command shortcuts.

Configuration follows the daemon's path rules: `HERDR_CONFIG_PATH` overrides the file; otherwise `XDG_CONFIG_HOME` wins, followed on Windows by `APPDATA`, then `USERPROFILE/AppData/Roaming`, with the shared fallback rules used by the application. Session and socket overrides are resolved independently of the config file override.

## Manual checks and known limits

Automated semantic Unicode input does not validate a physical Windows IME. Before calling Windows support complete, manually check Microsoft Pinyin/Japanese composition, candidate-window placement, commit and cancel, AltGr and keyboard-layout handling, CJK wide glyphs, emoji, clipboard paste, drag selection, context menus, focus switching, DPI scaling, and window resizing with a real desktop session.

Native cell rendering supports packed terminal colors, text modifiers, cursor shapes, local selection and screen search. Search covers the published screen rather than independently downloading scrollback. Inline image graphics are not rendered; the view displays an explicit unsupported-graphics message. Graphics payloads are never interpreted as shell text. Validate PowerShell and full-screen application behavior, mouse reporting, and popup focus on the target Windows machine.
