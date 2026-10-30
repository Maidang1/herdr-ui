# Herdr management UI validation

Implemented Agent/process details and agent naming, plugin catalog/actions/logs/install/update entry points, restore settings, output matching reminders, and saved-machine/local-session entry points. Pane movement and agent launching were excluded per user preference.

Validation on macOS with Herdr 0.9.5:

- `make test`: 12 Python tests, all Go packages and `go vet` passed after final fixes.
- `make build`: application and probe built after final fixes.
- Targeted race checks: `./scripts/go.sh test -race ./internal/herdr ./internal/ui ./internal/app ./cmd/herdr-ui`.
- Existing isolated integration suite passed, including native-window lifecycle; these tests use temporary config/socket paths.
- Temporary overlay tests against a disposable real daemon passed for shell process details, empty plugin catalog, substring/regex output matching and cancellation without blocking ping.
- Eight light/dark renders of Details, Plugins, Session restore and Machines were generated. Settings were also inspected at the supported 560×420 minimum; new control rows stack at narrow widths.

Final ordinary review findings were fixed: TOML edits now target parsed boolean byte ranges and preserve multiline strings; output reminders from Details retain the displayed pane target; Details and command surfaces are mutually exclusive. Regression checks cover multiline configuration content and reminder targeting.

Remote access currently opens Herdr's interactive terminal client within the main window. It does not render remote workspaces in the native sidebar. Live SSH access, installed-plugin mutations, successful naming of a real coding agent and Windows command surfaces were not physically exercised. Restore session references indicate available metadata, not a guarantee of successful restore.

No real user configuration, machine profiles or existing daemon sessions were modified during validation. No commit or release was created.
