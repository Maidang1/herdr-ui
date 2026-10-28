package ui

import (
	"errors"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	native "github.com/egoist/mygo/ui"
)

func TestIntegrationsLoadInstallAndRefresh(t *testing.T) {
	w := settingsWindow(t)
	var loaded func([]herdr.Integration, error)
	var changed func([]string, error)
	var action, target string
	changes := 0
	w.SetActionHooks(ActionHooks{
		Integrations: func(done func([]herdr.Integration, error)) { loaded = done },
		IntegrationChange: func(a, agent string, done func([]string, error)) {
			changes++
			action, target, changed = a, agent, done
		},
	})
	tester := native.NewTester(w.SettingsView, 720, 520)
	if err := tester.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	if loaded == nil || !w.settings.integrations.busy {
		t.Fatal("integration page did not request status")
	}
	loaded([]herdr.Integration{{Target: "codex", Label: "codex", Available: true, State: "not_installed"}}, nil)
	tester.Frame()
	if err := tester.Click("Install codex integration"); err != nil {
		t.Fatal(err)
	}
	if action != "install" || target != "codex" || changed == nil {
		t.Fatal(action, target)
	}
	_ = tester.Click("Install codex integration")
	if changes != 1 || !w.settings.integrations.busy {
		t.Fatal("busy operation allowed a duplicate mutation", changes)
	}
	changed([]string{"codex hooks installed"}, nil)
	if !w.settings.integrations.busy {
		t.Fatal("successful install did not refresh authoritative status")
	}
	loaded([]herdr.Integration{{Target: "codex", Label: "codex", Available: true, State: "current"}}, nil)
	tester.Frame()
	if !tester.HasText("codex hooks installed") || !tester.HasText("Installed · Up to date") {
		t.Fatal(tester.Texts())
	}
	if err := tester.Click("Remove codex integration"); err != nil {
		t.Fatal(err)
	}
	if action != "uninstall" || target != "codex" {
		t.Fatal(action, target)
	}
	changed(nil, errors.New("permission denied"))
	tester.Frame()
	if w.settings.integrations.busy || !tester.HasText("Could not change codex integration. permission denied") {
		t.Fatal("failed operation did not recover with an error")
	}
}

func TestIntegrationsUnavailableCommandAndOutdatedStates(t *testing.T) {
	for _, tc := range []struct {
		state, action, label string
		available            bool
	}{
		{"not_installed", "", "", false},
		{"not_installed", "install", "Install", true},
		{"current", "uninstall", "Remove", false},
		{"outdated", "install", "Update", false},
		{"future_state", "", "", true},
	} {
		_, action, label := integrationPresentation(herdr.Integration{State: tc.state, Available: tc.available})
		if action != tc.action || label != tc.label {
			t.Fatalf("%+v: %q %q", tc, action, label)
		}
	}
}

func TestIntegrationsLoadErrorCanRetry(t *testing.T) {
	w := settingsWindow(t)
	var loaded func([]herdr.Integration, error)
	w.SetActionHooks(ActionHooks{Integrations: func(done func([]herdr.Integration, error)) { loaded = done }})
	w.settings.page = indexOfPage("Integrations")
	tester := native.NewTester(w.SettingsView, 720, 520)
	loaded(nil, errors.New("server unavailable"))
	tester.Frame()
	if !tester.HasText("Could not refresh integrations. server unavailable") {
		t.Fatal(tester.Texts())
	}
	if err := tester.Click("Refresh integrations"); err != nil {
		t.Fatal(err)
	}
	if !w.settings.integrations.busy || w.settings.integrations.error != "" {
		t.Fatal("retry did not reset error state")
	}
}

func TestIntegrationActionsFitNarrowSettingsWindow(t *testing.T) {
	w := settingsWindow(t)
	w.settings.page = indexOfPage("Integrations")
	w.SetActionHooks(ActionHooks{
		Integrations: func(done func([]herdr.Integration, error)) {
			done([]herdr.Integration{{Target: "antigravity_cli", Label: "antigravity-cli", Available: true, State: "outdated"}}, nil)
		},
		IntegrationChange: func(string, string, func([]string, error)) {},
	})
	tester := native.NewTester(w.SettingsView, 560, 420)
	for _, label := range []string{"Update antigravity-cli integration", "Remove antigravity-cli integration"} {
		bounds, ok := tester.Find(label)
		if !ok || bounds.X < 180 || bounds.X+bounds.W > 560 {
			t.Fatal("integration control overflowed settings", label, bounds)
		}
	}
}

func TestIntegrationRefreshFailureDoesNotAllowStaleMutation(t *testing.T) {
	w := settingsWindow(t)
	var loaded func([]herdr.Integration, error)
	var changed func([]string, error)
	changes := 0
	w.SetActionHooks(ActionHooks{
		Integrations: func(done func([]herdr.Integration, error)) { loaded = done },
		IntegrationChange: func(_, _ string, done func([]string, error)) {
			changes++
			changed = done
		},
	})
	entry := herdr.Integration{Target: "codex", Label: "codex", Available: true, State: "not_installed"}
	w.loadIntegrations()
	loaded([]herdr.Integration{entry}, nil)
	w.changeIntegration(entry, "install")
	changed([]string{"Installed"}, nil)
	loaded(nil, errors.New("disconnected"))
	w.changeIntegration(entry, "install")
	if changes != 1 || w.settings.integrations.fresh {
		t.Fatal("stale installation state allowed a repeated mutation", changes)
	}
	w.loadIntegrations()
	entry.State = "current"
	loaded([]herdr.Integration{entry}, nil)
	if !w.settings.integrations.fresh {
		t.Fatal("successful refresh did not restore actions")
	}
}

func TestIntegrationReopenDuringLoadRequestsFreshStatus(t *testing.T) {
	w := settingsWindow(t)
	loads := 0
	var loaded func([]herdr.Integration, error)
	w.SetActionHooks(ActionHooks{Integrations: func(done func([]herdr.Integration, error)) {
		loads++
		loaded = done
	}})
	w.loadIntegrations()
	w.settings.integrations.result = "Old operation succeeded"
	w.RefreshIntegrations()
	loaded(nil, nil)
	if loads != 2 || !w.settings.integrations.busy || w.settings.integrations.result != "" {
		t.Fatal("reopening lost refresh intent or retained an old result", loads)
	}
	loaded(nil, nil)
	if !w.settings.integrations.fresh || w.settings.integrations.busy {
		t.Fatal("follow-up refresh did not complete")
	}
}
