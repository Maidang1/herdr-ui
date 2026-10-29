package endpoint

import (
	"context"
	"errors"
	"unicode/utf8"
)

// PaneEvent is already encoded semantic input, not outer-terminal VT bytes.
// The daemon encodes it for the target child's negotiated keyboard protocol.
type PaneEvent []byte

func TextEvent(text string, paste bool) (PaneEvent, error) {
	if len(text) > 1024*1024 || !utf8.ValidString(text) {
		return nil, errors.New("terminal text exceeds bounds or is not UTF-8")
	}
	tag := byte(1)
	if paste {
		tag = 3
	}
	return appendString([]byte{tag}, text), nil
}

// KeyCode uses the frozen ClientKeyCode discriminants; character keys carry
// one UTF-8 scalar, and F keys one byte in the range 1..24.
func KeyEvent(code uint8, char rune, function uint8, modifiers uint8, repeat bool) (PaneEvent, error) {
	if code > 17 || modifiers & ^uint8(15) != 0 {
		return nil, errors.New("invalid semantic key")
	}
	b := []byte{0, code}
	if code == 15 {
		if !utf8.ValidRune(char) {
			return nil, errors.New("invalid key codepoint")
		}
		b = utf8.AppendRune(b, char)
	} else if code == 16 {
		if function < 1 || function > 24 {
			return nil, errors.New("invalid function key")
		}
		b = append(b, function)
	}
	b = append(b, modifiers)
	kind := byte(0)
	if repeat {
		kind = 1
	}
	b = append(b, kind, 1)               // repeat_count=1
	return append(b, 0, 0, 0, 0, 0), nil // shifted, text, release, physical, Windows record
}

func MouseEvent(kind, button uint8, col, row uint16, modifiers uint8, lines uint16) (PaneEvent, error) {
	if kind > 7 || button > 2 || modifiers & ^uint8(15) != 0 {
		return nil, errors.New("invalid semantic mouse event")
	}
	b := []byte{2, kind}
	if kind < 3 {
		b = append(b, button)
	}
	b = append(b, 0) // Cell position
	b = appendUint(appendUint(b, uint64(col)), uint64(row))
	b = append(b, 0, modifiers) // no pixel geometry
	return appendUint(b, uint64(lines)), nil
}

func (c *Client) SendPaneEvent(ctx context.Context, boot, pane string, event PaneEvent, popup bool) error {
	c.mu.Lock()
	valid := c.state.Snapshot != nil && c.state.Snapshot.BootID == boot
	c.mu.Unlock()
	if !valid || pane == "" || len(pane) > 128 || len(event) == 0 {
		return errors.New("terminal input target is stale or invalid")
	}
	tag := byte(13)
	if popup {
		tag = 14
	}
	b := appendString([]byte{tag}, pane)
	b = append(b, 1) // one semantic event
	return c.send(ctx, append(b, event...))
}

func (c *Client) FocusTab(ctx context.Context, tab string) error {
	if tab == "" {
		return errors.New("explicit tab required")
	}
	return c.request(ctx, "tab.focus", map[string]string{"tab_id": tab})
}
func (c *Client) FocusPane(ctx context.Context, pane string) error {
	if pane == "" {
		return errors.New("explicit pane required")
	}
	return c.request(ctx, "pane.focus", map[string]string{"pane_id": pane})
}

func (c *Client) SetSurfaceActive(ctx context.Context, active bool) error {
	return c.request(ctx, "client_shell.surface.set", map[string]bool{"active": active})
}

func (c *Client) ResizeSurface(ctx context.Context, cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return errors.New("empty surface dimensions")
	}
	b := appendUint(appendUint([]byte{12, 0, 0}, uint64(cols)), uint64(rows))
	return c.send(ctx, append(b, 0)) // pixel_mouse=false
}

func (c *Client) FocusSurface(ctx context.Context, focused bool) error {
	b := byte(0)
	if focused {
		b = 1
	}
	return c.send(ctx, []byte{18, b})
}
