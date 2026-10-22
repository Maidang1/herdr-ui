package herdr

import (
	"encoding/hex"
	"testing"
)

// The bytes the Rust client writes for a key press, taken from the
// herdr-protocol model encoded with bincode 2's standard configuration.
func TestPaneKeyMatchesTheRustBytes(t *testing.T) {
	cases := []struct {
		name string
		key  PaneKey
		want string
	}{
		{
			name: "a control letter",
			key:  PaneKey{Code: KeyChar, Value: 'c', HasValue: true, Modifiers: KeyModControl},
			want: "0d0577313a703101000f630200010000000000",
		},
		{
			name: "enter",
			key:  PaneKey{Code: KeyEnter},
			want: "0d0577313a70310100010000010000000000",
		},
		{
			name: "a function key held down",
			key:  PaneKey{Code: KeyFunction, Value: 5, HasValue: true, Repeat: true, RepeatCount: 3},
			want: "0d0577313a7031010010050001030000000000",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := hex.EncodeToString(encodePaneKey("w1:p1", test.key))
			if got != test.want {
				t.Fatalf("the message came out as %s, want %s", got, test.want)
			}
		})
	}
}

func TestPaneTextMatchesTheRustBytes(t *testing.T) {
	got := hex.EncodeToString(encodePaneInput("w1:p1", "hi"))
	want := "0d0577313a70310101026869"
	if got != want {
		t.Fatalf("the message came out as %s, want %s", got, want)
	}
}
