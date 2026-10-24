package main

import (
	"errors"
	"log"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/session"
	"github.com/egoist/mygo"
)

// agentEntry is one agent pane as the sidebar lists it, with the status in
// its lowercase wire form.
type agentEntry struct {
	workspaceID, workspaceName, paneID, agent, status string
}

// notifier reports agent status transitions as system notifications and the
// Dock badge. Every method runs on the window's UI thread; the platform
// actions are fields so tests can collect calls without a desktop.
type notifier struct {
	// active reports whether the user is watching the window; delivery stays
	// quiet while they are, as the in-app overlay already has their eyes.
	active func() bool
	// jump selects an agent's workspace and pane, as a notification click does.
	jump func(workspaceID, paneID string)
	// supported reports whether the desktop can show notifications at all;
	// when it cannot, only the badge remains.
	supported func() bool
	// show delivers one notification for group; click runs when it is opened.
	show func(group, title, body string, click func())
	// badge publishes how many agents currently need the user.
	badge func(count int)
	// clearAll removes the notifications this app has delivered.
	clearAll func()

	last map[string]agentEntry
}

func newNotifier(active func() bool, jump func(workspaceID, paneID string)) *notifier {
	return &notifier{
		active:    active,
		jump:      jump,
		supported: mygo.NotificationsSupported,
		badge:     mygo.App.SetBadgeCount,
		clearAll:  mygo.ClearNotifications,
		show: func(group, title, body string, click func()) {
			// One slot per agent, so a newer event replaces its older banner.
			note := mygo.NewNotification(mygo.NotificationOptions{ID: "agent/" + group, Group: group, Title: title, Body: body})
			note.OnClick(func() {
				mygo.App.Focus()
				click()
				note.Close()
			})
			// Show waits for the system, which may first ask for permission;
			// the UI thread must not wait for that.
			go func() {
				if err := note.Show(); err != nil && !errors.Is(err, mygo.ErrNotificationsDenied) {
					log.Printf("notification: %v", err)
				}
			}()
		},
		last: make(map[string]agentEntry),
	}
}

// Step reports transitions since the previous publication. An agent seen for
// the first time only starts a baseline, so an attaching client never storms;
// one that vanished is forgotten, so its return is a first sight again.
func (n *notifier) Step(latest session.PublishedSnapshot) {
	entries := agentEntries(latest)
	quiet := !n.supported() || n.active()
	live := make(map[string]bool, len(entries))
	blocked := 0
	for _, entry := range entries {
		live[entry.paneID] = true
		if entry.status == "blocked" {
			blocked++
		}
		prev, seen := n.last[entry.paneID]
		n.last[entry.paneID] = entry
		if quiet || !seen || prev.status == entry.status {
			continue
		}
		var title string
		switch {
		case entry.status == "blocked":
			title = entry.agent + " needs you"
		case prev.status == "working" && entry.status == "done":
			title = entry.agent + " finished"
		default:
			continue
		}
		n.show(entry.paneID, title, entry.workspaceName, func() { n.jump(entry.workspaceID, entry.paneID) })
	}
	for id := range n.last {
		if !live[id] {
			delete(n.last, id)
		}
	}
	n.badge(blocked)
}

// Clear treats what was delivered as seen, as the window coming forward
// means the user is looking; the badge still counts what needs them.
func (n *notifier) Clear() {
	n.clearAll()
	blocked := 0
	for _, entry := range n.last {
		if entry.status == "blocked" {
			blocked++
		}
	}
	n.badge(blocked)
}

// agentEntries lists one entry per agent pane, as the sidebar's agent list
// does, in the snapshot's authoritative order.
func agentEntries(p session.PublishedSnapshot) []agentEntry {
	if p.Snapshot == nil {
		return nil
	}
	var entries []agentEntry
	for _, w := range p.Snapshot.Workspaces() {
		for _, t := range p.Snapshot.Tabs(w.ID) {
			for _, pane := range p.Snapshot.Panes(t.ID) {
				if pane.Agent == "" {
					continue
				}
				entries = append(entries, agentEntry{workspaceID: w.ID, workspaceName: w.Label, paneID: pane.ID, agent: pane.Agent, status: strings.ToLower(pane.Status)})
			}
		}
	}
	return entries
}
