package ui

import (
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/app"
	native "github.com/egoist/mygo/ui"
)

func toastWindow(t *testing.T) (*Window, *[]app.Notice) {
	t.Helper()
	w := populatedChromeWindow(t)
	w.SetInitialSettings(SettingsSnapshot{Toast: ToastSettings{Delivery: "herdr", Position: "top-right"}})
	var inbox []app.Notice
	w.hooks.Notices = func() []app.Notice {
		fresh := inbox
		inbox = nil
		return fresh
	}
	return w, &inbox
}

func TestNotificationFloatsWithoutResizingOrTakingFocus(t *testing.T) {
	w, inbox := toastWindow(t)
	tester := native.NewTester(w.View, 820, 560)
	before, _ := tester.Find("Terminal content")
	*inbox = append(*inbox, app.Notice{Kind: "NeedsAttention", Title: "codex needs attention", Body: "repo · 2", PaneID: "p2", Received: tester.Now()})
	tester.Frame()
	after, _ := tester.Find("Terminal content")
	toast, ok := tester.Find("Notification toast")
	if !ok || toast.X+toast.W != 804 || toast.Y != 56 || toast.W != 360 {
		t.Fatal("notification is not at the top right", toast)
	}
	if _, ok := tester.Find("repo · 2"); !ok {
		t.Fatal("notification body is missing")
	}
	if before != after || w.modalOpen() || tester.Focused("Open notification") {
		t.Fatal("notification disrupted the workspace", before, after)
	}
	if err := tester.Click("Open notification"); err != nil {
		t.Fatal(err)
	}
	if v := w.currentView(); v.PaneID != "p2" {
		t.Fatal("opening did not select the agent's pane", v.PaneID)
	}
	if _, ok := tester.Find("Notification toast"); ok {
		t.Fatal("opened notification is still visible")
	}
}

func TestNotificationExpiresAfterItsTimeAndCanBeDismissed(t *testing.T) {
	w, inbox := toastWindow(t)
	tester := native.NewTester(w.View, 820, 560)
	*inbox = append(*inbox,
		app.Notice{Kind: "Custom", Title: "first", Received: tester.Now()},
		app.Notice{Kind: "Custom", Title: "second", Received: tester.Now()})
	tester.Frame()
	if _, ok := tester.Find("first"); !ok {
		t.Fatal("first notification not shown")
	}
	if err := tester.Click("Dismiss notification"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tester.Find("second"); !ok {
		t.Fatal("dismissing did not bring the next notification")
	}
	tester.Advance(5 * time.Second)
	if _, ok := tester.Find("Notification toast"); ok {
		t.Fatal("expired notification is still visible")
	}
}

func TestNotificationForTheTabOnScreenStaysQuiet(t *testing.T) {
	w, inbox := toastWindow(t)
	tester := native.NewTester(w.View, 820, 560)
	*inbox = append(*inbox, app.Notice{Kind: "NeedsAttention", Title: "here", PaneID: "p", Received: tester.Now()})
	tester.Frame()
	if _, ok := tester.Find("Notification toast"); ok {
		t.Fatal("a notification for the visible tab was shown")
	}
}

func TestNotificationsSwitchMutesTheToast(t *testing.T) {
	w, inbox := toastWindow(t)
	w.SetInitialSettings(SettingsSnapshot{Toast: ToastSettings{Delivery: "off"}})
	tester := native.NewTester(w.View, 820, 560)
	*inbox = append(*inbox, app.Notice{Kind: "Custom", Title: "muted", Received: tester.Now()})
	tester.Frame()
	if _, ok := tester.Find("Notification toast"); ok {
		t.Fatal("a muted notification was shown")
	}
}

func TestNotificationPositionFollowsTheSetting(t *testing.T) {
	w, inbox := toastWindow(t)
	w.settings.snapshot.Toast.Position = "bottom-left"
	tester := native.NewTester(w.View, 820, 560)
	*inbox = append(*inbox, app.Notice{Kind: "Custom", Title: "corner", Received: tester.Now()})
	tester.Frame()
	toast, ok := tester.Find("Notification toast")
	if !ok || toast.X != 16 || toast.Y+toast.H != 544 {
		t.Fatal("notification is not at the bottom left", toast)
	}
}

func TestSystemDeliveryPostsInsteadOfShowing(t *testing.T) {
	w, inbox := toastWindow(t)
	w.settings.snapshot.Toast.Delivery = "system"
	var posted []app.Notice
	w.hooks.PostNotification = func(n app.Notice) { posted = append(posted, n) }
	tester := native.NewTester(w.View, 820, 560)
	*inbox = append(*inbox, app.Notice{Kind: "NeedsAttention", Title: "codex needs attention", PaneID: "p2", Received: tester.Now()})
	tester.Frame()
	if _, ok := tester.Find("Notification toast"); ok {
		t.Fatal("system delivery showed an in-app toast")
	}
	if len(posted) != 1 || posted[0].Title != "codex needs attention" {
		t.Fatal("system delivery did not post", posted)
	}
	*inbox = append(*inbox, app.Notice{Kind: "Custom", Title: "pumped", Received: time.Now()})
	w.PumpNotifications()
	if len(posted) != 2 {
		t.Fatal("a notice outside a frame was not posted", posted)
	}
}
