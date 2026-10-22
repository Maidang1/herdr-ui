package herdr

import (
	"encoding/hex"
	"testing"
)

// The bytes the Rust daemon writes for the messages the window reads,
// taken from the herdr-protocol model encoded with bincode 2's standard
// configuration.
func TestTheWindowTitleIsRead(t *testing.T) {
	title, err := decodeWindowTitle(mustHex(t, "06010b7a736820e2809420647368"))
	if err != nil {
		t.Fatalf("the title: %v", err)
	}
	if title != "zsh — dsh" {
		t.Fatalf("the title came out as %q", title)
	}

	title, err = decodeWindowTitle(mustHex(t, "0600"))
	if err != nil {
		t.Fatalf("the title: %v", err)
	}
	if title != "" {
		t.Fatalf("the title came out as %q", title)
	}
}

func TestTheNoticeOfTheDaemonIsRead(t *testing.T) {
	notice, err := decodeNotification(mustHex(t,
		"0e0111546865206167656e7420697320646f6e65011549742077726f74652074687265652066696c65732e01000105636f64657801027731010577313a7431010577313a70310103"))
	if err != nil {
		t.Fatalf("the notice: %v", err)
	}
	if notice.Kind != "finished" {
		t.Fatalf("the kind came out as %q", notice.Kind)
	}
	if notice.Title != "The agent is done" {
		t.Fatalf("the title came out as %q", notice.Title)
	}
	if notice.Body != "It wrote three files." {
		t.Fatalf("the body came out as %q", notice.Body)
	}
	if notice.Agent != "codex" {
		t.Fatalf("the agent came out as %q", notice.Agent)
	}
	if notice.WorkspaceID != "w1" || notice.TabID != "w1:t1" || notice.PaneID != "w1:p1" {
		t.Fatalf("the place came out as %+v", notice)
	}
	if notice.Position != "bottom right" {
		t.Fatalf("the place of the notice came out as %q", notice.Position)
	}
}

func TestANoticeWithNothingElseIsRead(t *testing.T) {
	notice, err := decodeNotification(mustHex(t, "0e000e546865206167656e742061736b7300000000000000"))
	if err != nil {
		t.Fatalf("the notice: %v", err)
	}
	if notice.Kind != "needs attention" {
		t.Fatalf("the kind came out as %q", notice.Kind)
	}
	if notice.Title != "The agent asks" {
		t.Fatalf("the title came out as %q", notice.Title)
	}
	if notice.Body != "" || notice.Agent != "" || notice.Position != "" {
		t.Fatalf("the notice came out as %+v", notice)
	}
}

func TestTheErrorOfTheDaemonIsRead(t *testing.T) {
	message, err := decodeClientError(mustHex(t, "0f107468652070616e6520697320676f6e65"))
	if err != nil {
		t.Fatalf("the error: %v", err)
	}
	if message != "the pane is gone" {
		t.Fatalf("the error came out as %q", message)
	}
}

// mustHex reads bytes written as hex.
func mustHex(t *testing.T, text string) []byte {
	t.Helper()
	data, err := hex.DecodeString(text)
	if err != nil {
		t.Fatalf("the bytes %q: %v", text, err)
	}
	return data
}

func TestTheBellOfATerminalIsRead(t *testing.T) {
	count, err := decodeTerminalBell(mustHex(t, "0901"))
	if err != nil {
		t.Fatalf("the bell: %v", err)
	}
	if count != 1 {
		t.Fatalf("the bell came out as %d", count)
	}
	// A number that does not fit in one byte stands behind its marker.
	count, err = decodeTerminalBell(mustHex(t, "09fb2c01"))
	if err != nil {
		t.Fatalf("the bell: %v", err)
	}
	if count != 300 {
		t.Fatalf("the bell came out as %d", count)
	}
}
