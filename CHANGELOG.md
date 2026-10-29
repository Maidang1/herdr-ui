# Changelog

## 0.0.11

- Add native Linux desktop packages for amd64 and arm64, including Debian packages, portable archives, a user-local installer, and signed in-app updates.
- Test Linux daemon, PTY, native GTK window integration, and packaged terminal startup in the multi-platform Release workflow.
- Publish compatible architecture-specific macOS update manifests alongside the universal update manifest.

Physical Linux X11/Wayland desktop and IME validation remains outstanding, as does physical Windows desktop and IME validation.

## 0.0.10

- Publish macOS universal and Windows x64 packages together through one multi-platform Release workflow.
- Include the Windows installer, portable ZIP, SHA-256 checksums, and signed Windows update archives and manifest in GitHub Releases.
- Run both platforms' tests and packaging in parallel, and publish only after both succeed. Manual workflow runs build artifacts without publishing.

Physical Windows desktop and IME validation remains outstanding.

## 0.0.9

- Add agent integration settings to inspect, install, and remove integrations managed by Herdr.
- Show contextual agent notifications without numeric location labels, match terminal backgrounds to the window theme, and add a compact sidebar settings shortcut.
- Add native Windows named-pipe transport and terminal surface rendering with semantic keyboard, Unicode text, paste, mouse, and resize input.
- Improve native terminal split-pane geometry, wide grapheme rendering, and reconnect history handling, with automated Windows tests and packaging artifacts.

Windows packages remain available as workflow artifacts; this release publishes the universal macOS app and signed updates. Physical Windows desktop and IME validation remains outstanding.

## 0.0.8

- Draw agent marks in their brand colors in the sidebar, tabs, split cards, pane list, and command palette.
- Add marks for Pi, Devin, Kilo, Qwen, Antigravity, and Grok. GitHub Copilot uses its multicolor mark. Black ink turns white in a dark window.
- Keep the content panel's corner radius on terminal cards that meet those corners.
- Keep font changes in the settings preview until Apply. Keyboard zoom still applies immediately.
- Replace the sidebar connection chip with an Update to x.y.z button when a newer release is available.

## 0.0.7

- Add native automatic updates: check after launch and daily while the app runs.
- Add Check for Updates to the application menu and an Updates page in Settings.
- Support release notes, download progress, skipping a version, and optional automatic installation on the next launch.
- Verify update archives with Ed25519 signatures and publish signed universal macOS update archives and manifests.

Users of v0.0.6 and earlier must install this version manually once to enable future in-app updates.
