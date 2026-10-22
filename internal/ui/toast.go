package ui

import (
	"time"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// What the daemon tells the window about.
//
// The daemon sends a notice when an agent finishes or asks for attention,
// and gives the window a title. The notices are shown as cards in the
// corner of the terminal, and go away by themselves.

const (
	// toastLifetime is how long a notice is shown.
	toastLifetime = 7 * time.Second
	// toastWidth is how wide a notice is.
	toastWidth = 320
	// toastGap is the space between two notices.
	toastGap = 8
)

// toast is a notice the daemon gave.
type toast struct {
	// notice is what the daemon said.
	notice herdr.Notification
	// until is when the notice goes away.
	until time.Time
}

// watchNotices shows what the daemon tells the window about.
func (a *App) watchNotices(endpoint *herdr.Endpoint) {
	go func() {
		for title := range endpoint.Titles() {
			a.setWindowTitle(title)
		}
	}()
	go func() {
		for notice := range endpoint.Notifications() {
			a.addToast(*notice)
		}
	}()
	go func() {
		for message := range endpoint.Errors() {
			a.setStatus("the daemon says: %s", message)
		}
	}()
	go func() {
		for range endpoint.Bells() {
			a.ringBell()
		}
	}()
}

// ringBell does what the settings say a ringing pane should do.
func (a *App) ringBell() {
	a.mu.Lock()
	sound := a.bellSound
	attention := a.bellAttention
	a.mu.Unlock()
	if sound {
		playSystemSound(soundBell, 100)
	}
	if attention {
		askForAttention()
	}
}

// setWindowTitle gives the window the title the daemon named.
func (a *App) setWindowTitle(title string) {
	a.mu.Lock()
	win := a.win
	a.mu.Unlock()
	if win == nil {
		return
	}
	if title == "" {
		title = "Herdr"
	}
	win.SetTitle(title)
}

// addToast shows a notice of the daemon.
//
// The settings say which notices are shown, and the sound of the notice
// plays with them.
func (a *App) addToast(notice herdr.Notification) {
	a.mu.Lock()
	enabled := a.notificationsEnabled
	finished := a.notificationsFinished
	volume := a.soundVolume
	a.mu.Unlock()

	if !enabled {
		return
	}
	if notice.Kind == "finished" && !finished {
		return
	}
	switch notice.Kind {
	case "finished":
		playSystemSound(soundDone, volume)
	case "needs attention":
		playSystemSound(soundRequest, volume)
	}

	a.mu.Lock()
	a.toasts = append(a.toasts, toast{notice: notice, until: time.Now().Add(toastLifetime)})
	if len(a.toasts) > 4 {
		a.toasts = a.toasts[len(a.toasts)-4:]
	}
	until := a.toasts[0].until
	a.mu.Unlock()
	a.invalidate()
	// The notice goes away by itself.
	time.AfterFunc(time.Until(until), a.invalidate)
}

// toastsOf returns the notices that are still shown.
func (a *App) toastsOf() []toast {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	shown := []toast{}
	for _, item := range a.toasts {
		if item.until.After(now) {
			shown = append(shown, item)
		}
	}
	return shown
}

// drawToasts shows the notices of the daemon in the corner of the terminal.
func (a *App) drawToasts(p *ui.Painter, r ui.Rect) {
	notices := a.toastsOf()
	if len(notices) == 0 {
		return
	}
	y := r.Y + r.H - 16
	for index := len(notices) - 1; index >= 0; index-- {
		notice := notices[index].notice
		title := notice.Title
		if title == "" {
			title = notice.Kind
		}
		height := float32(44)
		if notice.Body != "" {
			height = 64
		}
		y -= height
		card := ui.Rect{X: r.X + r.W - toastWidth - 16, Y: y, W: toastWidth, H: height}
		p.Fill(card, ui.RGBA(28, 32, 42, 0.96), 8)
		p.Fill(ui.Rect{X: card.X, Y: card.Y, W: 3, H: card.H}, ui.RGBA(120, 170, 255, 0.9), 0)
		p.Text(card.X+12, card.Y+20, title, 13, ui.RGBA(235, 238, 245, 1))
		if notice.Body != "" {
			body := notice.Body
			if len(body) > 44 {
				body = body[:44] + "…"
			}
			p.Text(card.X+12, card.Y+40, body, 12, ui.RGBA(160, 168, 180, 1))
		}
		y -= toastGap
	}
}
