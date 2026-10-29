package ui

import (
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func TestSelectTabNumberCountsFromOneAndNineIsLast(t *testing.T) {
	w := populatedChromeWindow(t)
	for _, step := range []struct {
		n    int
		want string
	}{{2, "other"}, {1, "t"}, {9, "other"}, {5, "other"}} {
		w.SelectTabNumber(step.n)
		if got := w.currentView().TabID; got != step.want {
			t.Fatalf("⌘%d selected %q, want %q", step.n, got, step.want)
		}
	}
}

func TestClosePaneKeyFindsTheFocusedAgent(t *testing.T) {
	w := referenceWindow(t)
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w","label":"Work","active_tab_id":"t"}],"tabs":[{"workspace_id":"w","tab_id":"t","number":1,"label":"Dev"}],"panes":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","title":"Shell"}],"agents":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","agent":"claude","agent_status":"working"}],"layouts":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	w.app.Update(session.PublishedSnapshot{State: session.Ready, Snapshot: snapshot})
	if got := focusedAgent(w.currentView()); got != "claude" {
		t.Fatalf("focused agent = %q", got)
	}
	if got := focusedAgent(populatedChromeWindow(t).currentView()); got != "" {
		t.Fatalf("a plain shell reported agent %q", got)
	}
}

func TestDirectKeyLabelsUseMenuOrder(t *testing.T) {
	wantLabels := map[string]string{"split_horizontal": "⇧⌘D", "new_tab": "⌘T", "focus_pane_left": "⌥⌘←", "next_tab": "⇧⌘]", "zen": "⌃⌘Z", "rename_tab": ""}
	if runtime.GOOS != "darwin" {
		wantLabels = map[string]string{"split_horizontal": "Ctrl+Alt+D", "new_tab": "Ctrl+Shift+T", "focus_pane_left": "Ctrl+Alt+Left", "next_tab": "Ctrl+PageDown", "zen": "Ctrl+Shift+Z", "rename_tab": ""}
	}
	if runtime.GOOS == "windows" {
		wantLabels["split_horizontal"] = "Ctrl+Shift+E"
		wantLabels["focus_pane_left"] = "Ctrl+Shift+Left"
	}
	for id, want := range wantLabels {
		if got := directKeyLabel(id); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}

func TestCopyConfirmationShowsThenFades(t *testing.T) {
	w := populatedChromeWindow(t)
	tester := native.NewTester(w.View, 1000, 700)
	if _, ok := tester.Find("Copied to clipboard"); ok {
		t.Fatal("confirmation showed before a copy")
	}
	w.ShowCopied()
	tester.Frame()
	if _, ok := tester.Find("Copied to clipboard"); !ok {
		t.Fatal("copy was not confirmed")
	}
	tester.Advance(copyShown)
	if _, ok := tester.Find("Copied to clipboard"); ok {
		t.Fatal("confirmation outstayed its time")
	}
}

func TestDirectKeyProfilesPreserveTerminalControlKeys(t *testing.T) {
	mac := directKeysForPlatform("darwin")
	if mac["new_tab"] != "CmdOrCtrl+T" || mac["split_horizontal"] != "CmdOrCtrl+Shift+D" {
		t.Fatalf("Mac profile changed: %v", mac)
	}
	for _, platform := range []string{"windows", "linux"} {
		keys := directKeysForPlatform(platform)
		if len(keys) != len(mac) {
			t.Fatalf("%s missing commands", platform)
		}
		seen := map[string]string{}
		for action, key := range keys {
			if key == "Ctrl+C" || key == "Ctrl+V" || key == "Ctrl+A" || key == "Ctrl+D" || key == "Ctrl+T" {
				t.Errorf("%s steals terminal control key %s for %s", platform, key, action)
			}
			if strings.Contains(key, "Cmd") || strings.Contains(key, "Super") || strings.Contains(key, "Win") {
				t.Errorf("%s uses system key %s", platform, key)
			}
			if previous := seen[key]; previous != "" {
				t.Errorf("%s: %s and %s share %s", platform, previous, action, key)
			}
			seen[key] = action
			if acceleratorLabel(key, platform) != key {
				t.Errorf("%s label uses Mac symbols: %s", platform, key)
			}
		}
		wantKeys := map[string]string{"new_tab": "Ctrl+Shift+T", "split_vertical": "Ctrl+Shift+D", "split_horizontal": "Ctrl+Alt+D", "previous_tab": "Ctrl+PageUp", "next_tab": "Ctrl+PageDown", "focus_pane_left": "Ctrl+Alt+Left"}
		if platform == "windows" {
			wantKeys["split_horizontal"] = "Ctrl+Shift+E"
			wantKeys["focus_pane_left"] = "Ctrl+Shift+Left"
		}
		for action, want := range wantKeys {
			if keys[action] != want {
				t.Errorf("%s %s=%q", platform, action, keys[action])
			}
		}
	}
}
func TestTabNumberAcceleratorMatchesPlatform(t *testing.T) {
	for _, platform := range []string{"windows", "linux", "darwin"} {
		for _, n := range []int{0, 1, 9, 10} {
			got := tabNumberAccelerator(platform, n)
			if n < 1 || n > 9 {
				if got != "" {
					t.Fatal(got)
				}
				continue
			}
			prefix := "Ctrl+Alt+"
			if platform == "darwin" {
				prefix = "Cmd+"
			} else if platform == "windows" {
				prefix = "Ctrl+Shift+"
			}
			if !strings.HasPrefix(got, prefix) {
				t.Fatalf("%s tab %d: %s", platform, n, got)
			}
		}
	}
}

func TestWindowsGlobalAcceleratorsLeaveAltGrTextAvailable(t *testing.T) {
	keys := directKeysForPlatform("windows")
	for action, want := range map[string]string{
		"split_horizontal": "Ctrl+Shift+E", "cycle_pane_previous": "Ctrl+Shift+PageUp", "cycle_pane_next": "Ctrl+Shift+PageDown",
		"focus_pane_left": "Ctrl+Shift+Left", "focus_pane_right": "Ctrl+Shift+Right", "focus_pane_up": "Ctrl+Shift+Up", "focus_pane_down": "Ctrl+Shift+Down",
		"find_previous": "Ctrl+Shift+H", "zen": "Ctrl+Shift+Z",
	} {
		if keys[action] != want {
			t.Errorf("%s=%s, want %s", action, keys[action], want)
		}
	}
	for n := 1; n <= 9; n++ {
		keys["tab-number-"+strconv.Itoa(n)] = tabNumberAccelerator("windows", n)
	}
	seen := map[string]string{}
	for action, key := range keys {
		parts := strings.Split(key, "+")
		ctrl, alt := false, false
		for _, part := range parts[:len(parts)-1] {
			ctrl = ctrl || part == "Ctrl"
			alt = alt || part == "Alt"
		}
		if ctrl && alt {
			t.Errorf("%s intercepts AltGr with %s", action, key)
		}
		if previous := seen[key]; previous != "" {
			t.Errorf("%s collides with %s at %s", action, previous, key)
		}
		seen[key] = action
	}
}
