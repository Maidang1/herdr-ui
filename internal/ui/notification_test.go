package ui

import (
	"testing"
	"time"

	native "github.com/egoist/mygo/ui"
)

func TestNotificationFloatsWithoutResizingOrTakingFocus(t *testing.T) {
	w := populatedChromeWindow(t)
	title := ""
	opened := 0
	w.hooks.NotificationText = func() (string, time.Duration) { return title, time.Second }
	w.hooks.NotificationTarget = func() string { opened++; return "" }
	tester := native.NewTester(w.View, 820, 560)
	before, _ := tester.Find("Terminal content")
	title = "codex finished"
	tester.Frame()
	after, _ := tester.Find("Terminal content")
	toast, ok := tester.Find("Notification toast")
	if !ok || toast.X+toast.W != 804 || toast.Y != 56 || toast.W != 360 {
		t.Fatal("notification is not at the top right", toast)
	}
	if before != after || w.modalOpen() || tester.Focused("Open notification") {
		t.Fatal("notification disrupted the workspace", before, after)
	}
	if err := tester.Click("Open notification"); err != nil {
		t.Fatal(err)
	}
	if opened != 1 {
		t.Fatal("notification action not dispatched", opened)
	}
	title = ""
	tester.Advance(time.Second)
	if _, ok := tester.Find("Notification toast"); ok {
		t.Fatal("expired notification is still visible")
	}
}
