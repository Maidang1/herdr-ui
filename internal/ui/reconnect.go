package ui

import (
	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The window keeps the picture of the session while the connection to the
// daemon is broken.
//
// The daemon may be restarting, or the machine may have been asleep; the
// endpoint tries again on its own, and the window shows why it is waiting
// and offers to try at once.

const (
	// reconnectCardWidth is how wide the card is.
	reconnectCardWidth = 420
	// reconnectCardHeight is how tall the card is.
	reconnectCardHeight = 116
	// reconnectButtonWidth is how wide the button is.
	reconnectButtonWidth = 140
	// reconnectButtonHeight is how tall the button is.
	reconnectButtonHeight = 28
)

// watchOutage tells the window when the connection breaks or comes back.
func (a *App) watchOutage(endpoint *herdr.Endpoint) {
	go func() {
		for range endpoint.OutageChanged() {
			if outage := endpoint.Outage(); outage != nil {
				a.setStatus("the connection to the daemon broke: %s", outage.Reason)
			} else {
				a.setStatus("the connection to the daemon is back")
			}
			a.invalidate()
		}
	}()
}

// outageOf returns the break in the connection, when there is one.
func (a *App) outageOf() *herdr.Outage {
	a.mu.Lock()
	endpoint := a.endpoint
	a.mu.Unlock()
	if endpoint == nil {
		return nil
	}
	return endpoint.Outage()
}

// reconnectButtonRect says where the button that tries again stands.
func (a *App) reconnectButtonRect(r ui.Rect) (ui.Rect, bool) {
	if a.outageOf() == nil {
		return ui.Rect{}, false
	}
	x := r.X + (r.W-reconnectCardWidth)/2 + 16
	y := r.Y + (r.H-reconnectCardHeight)/2 + 74
	return ui.Rect{X: x, Y: y, W: reconnectButtonWidth, H: reconnectButtonHeight}, true
}

// drawReconnecting shows the card over the picture of the session.
func (a *App) drawReconnecting(p *ui.Painter, r ui.Rect) {
	outage := a.outageOf()
	if outage == nil {
		return
	}
	// The picture of the session stays, dimmed.
	p.Fill(r, ui.RGBA(0, 0, 0, 0.36), 0)

	x := r.X + (r.W-reconnectCardWidth)/2
	y := r.Y + (r.H-reconnectCardHeight)/2
	card := ui.Rect{X: x, Y: y, W: reconnectCardWidth, H: reconnectCardHeight}
	p.Fill(card, ui.RGBA(28, 32, 42, 0.97), 10)
	p.Fill(ui.Rect{X: card.X, Y: card.Y, W: card.W, H: 1}, ui.RGBA(120, 170, 255, 0.8), 0)

	p.Text(card.X+16, card.Y+30, "Reconnecting to the daemon", 14, ui.RGBA(235, 238, 245, 1))
	reason := outage.Reason
	if len(reason) > 90 {
		reason = reason[:90] + "…"
	}
	p.Text(card.X+16, card.Y+54, reason, 12, ui.RGBA(150, 158, 170, 1))

	button := ui.Rect{X: card.X + 16, Y: card.Y + 74, W: reconnectButtonWidth, H: reconnectButtonHeight}
	p.Fill(button, ui.RGBA(38, 46, 62, 1), 6)
	p.Fill(ui.Rect{X: button.X, Y: button.Y, W: button.W, H: 1}, ui.RGBA(120, 170, 255, 0.7), 0)
	p.Text(button.X+12, button.Y+19, "Reconnect now", 12, ui.RGBA(220, 230, 245, 1))
}

// reconnectNow asks the endpoint to try again at once.
func (a *App) reconnectNow() {
	a.mu.Lock()
	endpoint := a.endpoint
	a.mu.Unlock()
	if endpoint == nil {
		return
	}
	endpoint.ReconnectNow()
	a.setStatus("trying the daemon again")
}
