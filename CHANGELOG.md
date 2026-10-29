# Changelog

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
