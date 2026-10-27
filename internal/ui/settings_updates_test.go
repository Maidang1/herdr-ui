package ui

import (
	"testing"
	"time"

	native "github.com/egoist/mygo/ui"
)

func TestUpdateSettingsSharePreferencesAndManualCheck(t *testing.T) {
	w := settingsWindow(t)
	state := UpdateSettings{Version: "0.0.7", Enabled: true, AutomaticChecks: true}
	checks := 0
	w.SetActionHooks(ActionHooks{
		UpdateSettings:           func() UpdateSettings { return state },
		CheckForUpdates:          func() { checks++ },
		AutomaticUpdateChecks:    func(on bool) { state.AutomaticChecks = on },
		AutomaticUpdateDownloads: func(on bool) { state.AutomaticDownloads = on },
	})
	tester := native.NewTester(w.SettingsView, 720, 1000)
	for _, label := range []string{"Updates", "Automatically check for updates", "Automatically download and install", "Check for Updates"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if state.AutomaticChecks || !state.AutomaticDownloads || checks != 1 {
		t.Fatalf("update hooks: state=%+v checks=%d", state, checks)
	}
	// Changes made by the update window appear on the next settings frame.
	state.LastCheck = time.Date(2026, 10, 27, 12, 34, 0, 0, time.Local)
	tester.Frame()
	if _, ok := tester.Find("2026-10-27 12:34"); !ok {
		t.Fatal("last check did not refresh")
	}
	if err := tester.Click("Automatically check for updates"); err != nil {
		t.Fatal(err)
	}
	if !state.AutomaticChecks {
		t.Fatal("automatic checks did not re-enable")
	}
}

func TestUpdateSettingsDevelopmentBuildCanCheckManually(t *testing.T) {
	w := settingsWindow(t)
	checked := false
	w.SetActionHooks(ActionHooks{CheckForUpdates: func() { checked = true }})
	tester := native.NewTester(w.SettingsView, 720, 1000)
	if err := tester.Click("Updates"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tester.Find("Automatic updates unavailable"); !ok {
		t.Fatal("missing unavailable explanation")
	}
	if _, ok := tester.Find("Automatically check for updates"); ok {
		t.Fatal("disabled updater offers automatic checks")
	}
	if err := tester.Click("Check for Updates"); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("manual check unavailable for disabled build")
	}
}
