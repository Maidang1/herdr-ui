The JSON fixtures are copied unchanged from Herdr's frozen generation-one
fixtures in tests/fixtures/endpoint-{welcome,snapshot}-v1.json.

The binary fixtures were generated using Rust serde and bincode 2's
bincode::config::standard(), with the field and variant order frozen in
Herdr src/protocol/wire.rs. EndpointControl is enum tag 20 followed by
kind and data strings. notification-v1.bin reproduces Herdr's
semantic_notification_roundtrip fixture: NeedsAttention, "codex needs
attention", body "repo · 1", Request sound, agent codex, workspace w1,
tab w1:t1, pane w1:p1, TopRight position. Notification is enum tag 14.
These are payloads; the transport adds the four-byte little-endian length.
