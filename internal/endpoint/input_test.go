package endpoint

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"
	"testing"
)

// Full ClientShellPaneInput payloads produced by Rust herdr-protocol's
// encode_message with bincode 2 standard; envelope prefix is tag13,"pane",1.
func TestSemanticInputRustGoldens(t *testing.T) {
	cases := []struct {
		name, golden string
		event        func() (PaneEvent, error)
	}{
		{"commit", "0d0470616e65010106e4bda0e5a5bd", func() (PaneEvent, error) { return TextEvent("你好", false) }},
		{"paste", "0d0470616e65010303610a62", func() (PaneEvent, error) { return TextEvent("a\nb", true) }},
		{"enter", "0d0470616e650100010000010000000000", func() (PaneEvent, error) { return KeyEvent(1, 0, 0, 0, false) }},
		{"unicode-repeat", "0d0470616e6501000fe7958c0301010000000000", func() (PaneEvent, error) { return KeyEvent(15, '界', 0, 3, true) }},
		{"right-click", "0d0470616e650102000100fb2c0105000200", func() (PaneEvent, error) { return MouseEvent(0, 1, 300, 5, 2, 0) }},
		{"scroll", "0d0470616e650102040002030000fbf401", func() (PaneEvent, error) { return MouseEvent(4, 0, 2, 3, 0, 500) }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ev, err := tt.event()
			if err != nil {
				t.Fatal(err)
			}
			want, err := hex.DecodeString(tt.golden)
			if err != nil {
				t.Fatal(err)
			}
			got := append([]byte{13, 4, 'p', 'a', 'n', 'e', 1}, ev...)
			if !bytes.Equal(got, want) {
				t.Fatalf("semantic payload\n got %x\nwant %x", got, want)
			}
		})
	}
}
func TestSemanticInputRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		event func() (PaneEvent, error)
	}{
		{"text-utf8", func() (PaneEvent, error) { return TextEvent(string([]byte{255}), false) }},
		{"text-limit", func() (PaneEvent, error) { return TextEvent(strings.Repeat("x", 1024*1024+1), true) }},
		{"key-enum", func() (PaneEvent, error) { return KeyEvent(18, 0, 0, 0, false) }},
		{"surrogate", func() (PaneEvent, error) { return KeyEvent(15, 0xd800, 0, 0, false) }},
		{"function-zero", func() (PaneEvent, error) { return KeyEvent(16, 0, 0, 0, false) }},
		{"function-limit", func() (PaneEvent, error) { return KeyEvent(16, 0, 25, 0, false) }},
		{"key-mods", func() (PaneEvent, error) { return KeyEvent(1, 0, 0, 128, false) }},
		{"mouse-enum", func() (PaneEvent, error) { return MouseEvent(8, 0, 0, 0, 0, 0) }},
		{"mouse-button", func() (PaneEvent, error) { return MouseEvent(0, 3, 0, 0, 0, 0) }},
		{"mouse-mods", func() (PaneEvent, error) { return MouseEvent(0, 0, 0, 0, 128, 0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.event(); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	c := NewSocket("unused")
	c.state.Snapshot = &Snapshot{BootID: "current"}
	if err := c.SendPaneEvent(context.Background(), "old", "pane", PaneEvent{1, 0}, false); err == nil {
		t.Fatal("stale boot accepted")
	}
}
