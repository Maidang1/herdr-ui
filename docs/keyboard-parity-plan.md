# Herdr keyboard parity

Goal: every action in the local Herdr KeysConfig must resolve and reach a real UI or server operation. Preserve window-local navigation, explicit mutation targets, IME guards, and no mutation retries.

Design: keep native terminal rendering. Resolve normal, navigation, indexed and command bindings in internal/input. Dispatch local selection and modes in internal/ui, and use explicit socket methods for mutations and revision-guarded history extraction. Copy mode owns an immutable local history capture. Native panels handle text entry and confirmations; external editor and clipboard integrations are injected by the executable.

Alternatives considered: forwarding all keys to a TUI would change rendering and focus ownership; implementing only default keys would leave custom configurations broken. Implement native action parity for ordinary actions; use an isolated embedded Herdr client for server-owned popup/plugin presentation.

## Tasks

- [x] Input: compare KeysConfig/keybinds with all defaults and custom forms; preserve numeric indices, commands and navigation bindings; test conflicts, prefix encoding and native keys.
- [x] Session: add typed explicit-target operations for tab/workspace/pane/worktree mutations, reload, copy operations, command and history support. Test scope validation and response handling.
- [x] UI: connect the complete action catalog, local navigation/history, navigation mode, prompts, copy mode, reload/settings/detach and custom commands. Correct direct resize direction names.
- [x] Integration: run ./scripts/go.sh test ./..., build the executable, verify representative actions against an isolated server where available. Review spec coverage and code correctness; document any actual API limitations.

Existing uncommitted changes belong to the user and must be preserved. Do not commit unrelated changes.

## Validation and platform boundary

See `keyboard-parity.md` for the delivered behavior and native-key limits. Verification includes unit/race suites, static checks, real isolated-server mutation/history/worktree/command/image tests, and window lifecycle integration. The native dependency has no F13+ or independent Hyper key events; such bindings are disabled with diagnostics while all other bindings remain usable.
