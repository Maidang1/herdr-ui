package main

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/session"
	"github.com/egoist/mygo"
)

// notifier posts Herdr's notifications to the system and keeps the Dock
// badge. Which notices to post, and when, is the toast queue's decision;
// the badge follows the snapshot. Every method runs on the window's UI
// thread; the platform actions are fields so tests can collect calls
// without a desktop.
type notifier struct {
	// open selects the place a notice names, as a notification click does.
	open func(app.Notice)
	// supported reports whether the desktop can show notifications at all;
	// when it cannot, only the badge remains.
	supported func() bool
	// show delivers one notification for group; click runs when it is opened.
	show func(group, title, body string, click func())
	// badge publishes how many agents currently need the user.
	badge func(count int)
	// clearAll removes the notifications this app has delivered.
	clearAll func()

	// enabled is off only when delivery is off, which mutes the badge too,
	// so one setting governs every surface.
	enabled bool
	blocked int
}

func newNotifier(open func(app.Notice)) *notifier {
	return &notifier{
		open:      open,
		enabled:   true,
		supported: mygo.NotificationsSupported,
		badge:     mygo.App.SetBadgeCount,
		clearAll:  mygo.ClearNotifications,
		show: func(group, title, body string, click func()) {
			// One slot per pane, so a newer event replaces its older banner.
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
	}
}

// SetEnabled follows the delivery setting: off clears the badge and
// whatever banners already went out.
func (n *notifier) SetEnabled(enabled bool) {
	n.enabled = enabled
	if !enabled {
		n.clearAll()
		n.badge(0)
		return
	}
	n.badge(n.blocked)
}

// Post shows one notice the toast queue made ready for the system.
func (n *notifier) Post(notice app.Notice) {
	if !n.enabled || !n.supported() {
		return
	}
	group := notice.PaneID
	if group == "" {
		group = "notice/" + strconv.FormatInt(notice.Received.UnixNano(), 10)
	}
	n.show(group, notice.Title, notice.Body, func() { n.open(notice) })
}

// Step recounts the badge from a publication: the agents blocked on the user.
func (n *notifier) Step(latest session.PublishedSnapshot) {
	n.blocked = 0
	for _, entry := range agentEntries(latest) {
		if entry.status == "blocked" {
			n.blocked++
		}
	}
	if n.enabled {
		n.badge(n.blocked)
	}
}

// Clear treats what was delivered as seen, as the window coming forward
// means the user is looking; the badge still counts what needs them.
func (n *notifier) Clear() {
	n.clearAll()
	if n.enabled {
		n.badge(n.blocked)
	}
}

// agentEntry is one agent pane as the sidebar lists it, with the status in
// its lowercase wire form.
type agentEntry struct {
	workspaceID, workspaceName, paneID, agent, status string
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
