# Changelog

## 0.0.15

- Detect whether Herdr is installed (PATH and the usual install directories). Without Herdr, the window explains how to install it and offers a plain terminal.
- Add a plain terminal mode: local shells in tabs, a quiet one-line strip, and a Herdr status chip that leads back to Herdr once it is installed. Settings → Herdr chooses Automatic, Herdr, or Plain terminal at startup; `-plain` and `-connect` override it once.
- Reorder workspaces and tabs by dragging them, or with Move Up/Down in the workspace menu (`workspace.move`, `tab.move`).
- Move a pane to a new tab, a new workspace, or another tab (`pane.move`).
- Send a prompt to an agent, get notified once when it stops, and start an agent in a pane (`agent.prompt`, `agent.wait`, `agent.start`).
- Filter the sidebar's agents by view (All, Needs attention, Working, This workspace) and share the view with Herdr's clients (`agent.view.set`).
- Save a tab's layout and open it again as a new tab (`layout.export`, `layout.apply`); saved layouts keep splits, ratios, names and directories, not environments.
- Optionally repeat Hergo's reminders in Herdr's terminal clients (`notification.show`).
- Settings → Herdr shows the installation, agent manifests with reload, saved layouts, and live handoff to a newly installed Herdr. The plugins page links and unlinks local plugins.
- Name the window after the selected workspace and tab.

## 0.0.14

- Add agent and process details, detection diagnostics, and independent agent naming.
- Add Herdr plugin management, session restore settings, saved SSH machines, and local session entry points.
- Add cancellable output matching reminders with substring and regular expression matching.
- Add official agent marks for Prime Agent, OMP, Droid, Qoder CLI, Hermes, Letta, and Mastra Code, with light and dark theme support.

Live SSH access, installed-plugin mutations, successful naming of a real coding agent, and Windows command surfaces remain unverified on physical systems. Physical Linux X11/Wayland desktop and IME validation remains outstanding, as does physical Windows desktop and IME validation.

## 0.0.13

- Follow Herdr theme configuration automatically, including light/dark switching and custom color overrides. Apply configuration edits to windows and terminals while the app is running.
- Show all themes as preview cards, with System, Hergo Light and Hergo Dark alongside Herdr built-in themes. Use the Follow Herdr switch to choose automatic or manual colors.
- Split Spaces and Agents into equal-height sidebar sections with fixed headers and independent scrolling, and hide their scrollbars.
- Improve default Herdr text contrast.

Physical Linux X11/Wayland desktop and IME validation remains outstanding, as does physical Windows desktop and IME validation.

## 0.0.12

- Rebrand the desktop application and platform packages as Hergo.
- Adopt a new ram app icon inspired by Herdr and mygo.
- Rewrite the README with installation, development, and release guidance, and adopt the MIT license.

Physical Linux X11/Wayland desktop and IME validation remains outstanding, as does physical Windows desktop and IME validation.

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
