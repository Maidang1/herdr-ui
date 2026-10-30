# Herdr palette attribution

`herdr_palettes.json` was mechanically extracted from `src/app/state.rs` in
[herdrdev/herdr](https://github.com/herdrdev/herdr) at commit
`ee5bfb5041bb3e4a49d591c72308aae3c81ecdde`. The configuration resolution and
color parser in `herdr.go` are adapted from that revision's
`src/config/theme.rs` and `src/app/mod.rs`.

Upstream Herdr is licensed under Apache-2.0; the full license is retained in
`herdr-LICENSE`. This adaptation converts Ratatui colors into native UI colors,
keeps separate resolved light/dark slots, and maps the original surface and
status tokens to this application's semantic palette. Upstream terminal reset
colors resolve to native defaults because this UI does not inherit a host
terminal palette; named ANSI colors use the conventional 16-color RGB values.

To reproduce the palette extraction, read each of the 18 constructors named by
`THEME_NAMES` in `src/config/theme.rs`, extract their `Color::Rgb(r, g, b)` and
named `Color` field values, convert RGB tuples to lowercase hex strings, and
write the field names unchanged. No palette RGB values are hand-maintained.
