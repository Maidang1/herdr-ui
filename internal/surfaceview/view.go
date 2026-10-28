// Package surfaceview paints immutable Herdr endpoint cells in native mygo UI.
// It does not run a terminal emulator or interpret screen glyphs as shell input.
package surfaceview

import (
	"math"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/rivo/uniseg"
)

type Config struct {
	// OnPaste receives clipboard text as semantic paste, without terminal escaping.
	OnPaste          func(string) bool
	Font             terminal.Font
	Theme, DarkTheme *terminal.Theme
	CopyOnSelect     terminal.CopyOnSelect
	MinimumContrast  float32
	OnInput          func(ui.InputEvent) bool
	Copy             func(string)
	Paste            func() string
	OnSearch         func(terminal.SearchStatus)
}

type View struct {
	lastSearch                   terminal.SearchStatus
	searchReported               bool
	baseGrid                     endpoint.SurfaceGrid
	baseRect                     endpoint.SurfaceRect
	popup                        bool
	offsetX, offsetY             float32
	cfg                          Config
	frame                        endpoint.SurfaceFrame
	pane                         endpoint.SurfacePane
	present                      bool
	services                     ui.Services
	theme                        *terminal.Theme
	focused, selecting, selected bool
	reportingPointer             bool
	anchor, head                 int
	preedit                      string
	preeditCaret                 int
	query                        string
	matches                      []span
	match                        int
	glyphs                       map[glyphKey][]ui.Glyph
}
type span struct{ start, end int }
type glyphKey struct {
	text    string
	variant uint16
}

func New(cfg Config) *View { return &View{cfg: cfg, glyphs: make(map[glyphKey][]ui.Glyph)} }
func (v *View) SetFrame(f endpoint.SurfaceFrame, paneID string) {
	old := v.pane.InnerRect
	if v.frame.BootID != f.BootID || v.pane.PaneID != paneID {
		v.selected = false
		v.preedit = ""
		v.reportingPointer = false
	}
	v.frame = f
	v.present = false
	for _, p := range f.Panes {
		if p.PaneID == paneID {
			v.pane = p
			v.present = true
			break
		}
	}
	if !v.present {
		v.pane = endpoint.SurfacePane{}
	}
	v.popup = v.present && v.pane.Focused && f.Popup != nil
	if v.popup {
		v.baseGrid = f.Frame
		v.baseRect = v.pane.InnerRect
		v.frame.Frame = f.Popup.Frame
		v.pane.InnerRect = endpoint.SurfaceRect{Width: f.Popup.Frame.Width, Height: f.Popup.Frame.Height}
		v.pane.MouseReporting = f.Popup.MouseReporting
	}
	if old != v.pane.InnerRect {
		v.selected = false
	}
	v.research()
}
func (v *View) SetFont(f terminal.Font)                 { v.cfg.Font = f; clear(v.glyphs) }
func (v *View) SetCopyOnSelect(c terminal.CopyOnSelect) { v.cfg.CopyOnSelect = c }
func (v *View) SetMinimumContrast(r float32)            { v.cfg.MinimumContrast = r }
func (v *View) Composing() bool                         { return v.preedit != "" }

// UnsupportedGraphics reports assets this native cell renderer cannot display.
func (v *View) UnsupportedGraphics() bool {
	return len(v.frame.Graphics.Placements) > 0 || len(v.frame.Frame.Graphics) > 0
}
func fontOf(f terminal.Font, variant uint16) ui.Font {
	if f.Size <= 0 {
		f.Size = 13
	}
	if f.Family == "" {
		f.Family = "monospace"
	}
	if f.Weight <= 0 {
		f.Weight = 400
	}
	if variant&1 != 0 {
		f.Weight = min(900, max(700, f.Weight+300))
	}
	features := make([]string, 0, len(f.Features))
	for _, s := range f.Features {
		if strings.HasPrefix(s, "-") {
			s = s[1:] + "=0"
		}
		features = append(features, strings.TrimPrefix(s, "+"))
	}
	return ui.Font{Family: f.Family, Size: f.Size, Weight: f.Weight, Italic: variant&4 != 0, Features: strings.Join(features, ","), Thicken: f.Thicken}
}

// CellMetrics returns the actual monospace advance and row height in DIPs.
func CellMetrics(f terminal.Font) (float32, float32) {
	font := fontOf(f, 0)
	g := ui.Shape("M", font)
	w := font.Size * .6
	if len(g) > 0 {
		w = g[0].Advance
	}
	m := font.Metrics()
	lh := f.LineHeight
	if lh <= 0 {
		lh = 1
	}
	return max(1, w), max(1, (m.Ascent+m.Descent+m.LineGap)*lh)
}
func (v *View) View(c *ui.Context) ui.Element {
	v.services = c.Services()
	v.theme = v.cfg.Theme
	if c.Theme().Dark && v.cfg.DarkTheme != nil {
		v.theme = v.cfg.DarkTheme
	}
	if v.theme == nil {
		if c.Theme().Dark {
			v.theme = terminal.DarkTheme()
		} else {
			v.theme = terminal.LightTheme()
		}
	}
	e := ui.Box(c).Focusable().FocusRing(false).Clip().Label("Terminal").Cursor(ui.CursorText)
	if v.focused && !e.Focused() {
		v.preedit = ""
		v.selecting = false
	}
	v.focused = e.Focused()
	e.HandleInput(v.input).TextCaret(v.caret()).Draw(v.paint).ContextMenu(v.menu)
	return e
}
func (v *View) cell(x, y int) endpoint.SurfaceCell {
	r := v.pane.InnerRect
	xx, yy := int(r.X)+x, int(r.Y)+y
	g := v.frame.Frame
	if !v.present || x < 0 || y < 0 || x >= int(r.Width) || y >= int(r.Height) || xx >= int(g.Width) || yy >= int(g.Height) {
		return endpoint.SurfaceCell{}
	}
	i := yy*int(g.Width) + xx
	if i >= len(g.Cells) {
		return endpoint.SurfaceCell{}
	}
	return g.Cells[i]
}

// cellColumns derives continuation cells from grapheme display width. Skip is
// Herdr's render-diff flag and carries no character-width information.
func (v *View) cellColumns(x, y int) int {
	return min(max(1, uniseg.StringWidth(v.cell(x, y).Symbol)), int(v.pane.InnerRect.Width)-x)
}
func (v *View) leadingColumn(col, row int) int {
	for x := 0; x < int(v.pane.InnerRect.Width); {
		columns := v.cellColumns(x, row)
		if col < x+columns {
			return x
		}
		x += columns
	}
	return col
}
func (v *View) caret() ui.Rect {
	cw, ch := CellMetrics(v.cfg.Font)
	r := v.pane.InnerRect
	if cur := v.frame.Frame.Cursor; cur != nil {
		x := v.offsetX + float32(int(cur.X)-int(r.X))*cw
		if v.preedit != "" {
			runes := []rune(v.preedit)
			caret := min(max(0, v.preeditCaret), len(runes))
			for _, g := range v.shape(string(runes[:caret]), 0) {
				x += g.Advance
			}
		}
		return ui.Rect{X: x, Y: v.offsetY + float32(int(cur.Y)-int(r.Y))*ch, W: cw, H: ch}
	}
	return ui.Rect{W: cw, H: ch}
}
func (v *View) invalidate() {
	if v.theme != nil {
		v.services.Invalidate()
	}
}
func (v *View) forward(ev ui.InputEvent) bool { return v.cfg.OnInput != nil && v.cfg.OnInput(ev) }
func (v *View) copy() {
	s := v.SelectedText()
	if s == "" {
		return
	}
	if v.cfg.Copy != nil {
		v.cfg.Copy(s)
	} else if v.theme != nil {
		v.services.WriteClipboard(s)
	}
}
func (v *View) paste() {
	s := ""
	if v.cfg.Paste != nil {
		s = v.cfg.Paste()
	} else if v.theme != nil {
		s = v.services.ReadClipboard()
	}
	if s != "" {
		if v.cfg.OnPaste != nil {
			v.cfg.OnPaste(s)
		} else {
			v.forward(ui.InputEvent{Kind: ui.InputText, Text: s})
		}
	}
}
func (v *View) menu(m *ui.Menu) {
	if m.Item("Copy").Disabled(v.SelectedText() == "").Chosen() {
		v.copy()
	}
	if m.Item("Paste").Chosen() {
		v.paste()
	}
	if m.Item("Select All").Chosen() {
		v.selectAll()
	}
}
func (v *View) selectAll() {
	v.anchor = 0
	v.head = int(v.pane.InnerRect.Width)*int(v.pane.InnerRect.Height) - 1
	v.selected = v.head >= 0
	v.invalidate()
	if v.cfg.CopyOnSelect != terminal.CopyOnSelectNone {
		v.copy()
	}
}
func (v *View) point(x, y float32) int {
	cw, ch := CellMetrics(v.cfg.Font)
	w, h := int(v.pane.InnerRect.Width), int(v.pane.InnerRect.Height)
	if w == 0 || h == 0 {
		return 0
	}
	col := max(0, min(w-1, int(math.Floor(float64(x/cw)))))
	row := max(0, min(h-1, int(math.Floor(float64(y/ch)))))
	col = v.leadingColumn(col, row)
	return row*w + col
}
func (v *View) input(ev ui.InputEvent) bool {
	ev.X -= v.offsetX
	ev.Y -= v.offsetY
	switch ev.Kind {
	case ui.InputCompose:
		v.preedit = ev.Text
		v.preeditCaret = ev.Caret
		v.invalidate()
		return true
	case ui.InputText:
		v.preedit = ""
		v.selected = false
		return v.forward(ev)
	case ui.InputCommand:
		switch ev.Text {
		case "copy":
			v.copy()
			return true
		case "paste":
			v.paste()
			return true
		case "selectAll":
			v.selectAll()
			return true
		}
	case ui.InputKeyDown:
		mods := ui.Ctrl | ui.Shift
		if runtime.GOOS == "darwin" {
			mods = ui.Super
		}
		if ev.Mods == mods {
			if ev.Key == ui.KeyC {
				v.copy()
				return true
			}
			if ev.Key == ui.KeyV {
				v.paste()
				return true
			}
		}
	case ui.InputPointerDown, ui.InputPointerMove, ui.InputPointerUp:
		if v.reportingPointer && ev.Kind != ui.InputPointerDown {
			taken := v.forward(ev)
			if ev.Kind == ui.InputPointerUp {
				v.reportingPointer = false
			}
			return taken
		}
		if v.pane.MouseReporting && ev.Mods&ui.Shift == 0 && !v.selecting {
			taken := v.forward(ev)
			if taken && ev.Kind == ui.InputPointerDown {
				v.reportingPointer = true
			}
			return taken
		}
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			v.anchor = v.point(ev.X, ev.Y)
			v.head = v.anchor
			v.selecting = true
			v.selected = false
			return true
		case ui.InputPointerMove:
			if !v.selecting {
				return false
			}
			v.head = v.point(ev.X, ev.Y)
			v.selected = v.head != v.anchor
			v.invalidate()
			return true
		case ui.InputPointerUp:
			if !v.selecting || ev.Button != 0 {
				return false
			}
			v.head = v.point(ev.X, ev.Y)
			v.selecting = false
			v.selected = v.head != v.anchor
			v.invalidate()
			if v.cfg.CopyOnSelect != terminal.CopyOnSelectNone {
				v.copy()
			}
			return true
		}
	}
	return v.forward(ev)
}
func (v *View) SelectedText() string {
	if !v.selected {
		return ""
	}
	w := int(v.pane.InnerRect.Width)
	if w == 0 {
		return ""
	}
	a, b := min(v.anchor, v.head), max(v.anchor, v.head)
	var out strings.Builder
	for row := a / w; row <= b/w; row++ {
		var line strings.Builder
		for x := 0; x < w; {
			columns := v.cellColumns(x, row)
			i := row*w + x
			cell := v.cell(x, row)
			x += columns
			if i+columns-1 < a || i > b {
				continue
			}
			symbol := cell.Symbol
			if symbol == "" {
				symbol = " "
			}
			line.WriteString(symbol)
		}
		out.WriteString(strings.TrimRight(line.String(), " "))
		if row < b/w {
			out.WriteByte('\n')
		}
	}
	return out.String()
}
func (v *View) Search(s string) { v.query = s; v.match = 0; v.research(); v.invalidate() }
func (v *View) SearchNext() {
	if len(v.matches) > 0 {
		v.match = (v.match + 1) % len(v.matches)
	}
	v.reportSearch()
	v.invalidate()
}
func (v *View) SearchPrevious() {
	if len(v.matches) > 0 {
		v.match = (v.match + len(v.matches) - 1) % len(v.matches)
	}
	v.reportSearch()
	v.invalidate()
}
func (v *View) EndSearch() { v.Search("") }
func (v *View) reportSearch() {
	if v.cfg.OnSearch != nil {
		selected := 0
		if len(v.matches) > 0 {
			selected = v.match + 1
		}
		status := terminal.SearchStatus{Text: v.query, Total: len(v.matches), Selected: selected}
		if !v.searchReported || status != v.lastSearch {
			v.searchReported = true
			v.lastSearch = status
			v.cfg.OnSearch(status)
		}
	}
}
func (v *View) research() {
	v.matches = nil
	w, h := int(v.pane.InnerRect.Width), int(v.pane.InnerRect.Height)
	if v.query != "" {
		for y := 0; y < h; y++ {
			var text strings.Builder
			var positions, ends []int
			for x := 0; x < w; {
				cell := v.cell(x, y)
				columns := v.cellColumns(x, y)
				symbol := cell.Symbol
				if symbol == "" {
					symbol = " "
				}
				text.WriteString(symbol)
				for range len(symbol) {
					positions = append(positions, y*w+x)
					ends = append(ends, y*w+x+columns)
				}
				x += columns
			}
			s := text.String()
			for off := 0; off < len(s); {
				i := strings.Index(s[off:], v.query)
				if i < 0 {
					break
				}
				a := off + i
				b := a + len(v.query) - 1
				v.matches = append(v.matches, span{positions[a], ends[b]})
				off = b + 1
			}
		}
	}
	if v.match >= len(v.matches) {
		v.match = 0
	}
	v.reportSearch()
}
func (v *View) shape(s string, mod uint16) []ui.Glyph {
	k := glyphKey{s, mod & 5}
	if g, ok := v.glyphs[k]; ok {
		return g
	}
	if len(v.glyphs) >= 4096 {
		clear(v.glyphs)
	}
	g := ui.Shape(s, fontOf(v.cfg.Font, mod))
	v.glyphs[k] = g
	return g
}
func (v *View) paint(p *ui.Painter, r ui.Rect) {
	if v.theme == nil {
		return
	}
	if v.popup {
		base := *v
		base.popup = false
		base.frame.Frame = v.baseGrid
		base.pane.InnerRect = v.baseRect
		base.preedit = ""
		base.selected = false
		base.matches = nil
		base.paint(p, r)
	} else {
		p.Fill(r, v.theme.Background, 0)
	}
	if !v.present {
		return
	}
	cw, ch := CellMetrics(v.cfg.Font)
	v.offsetX, v.offsetY = 0, 0
	if v.popup {
		v.offsetX = max(0, (r.W-float32(v.pane.InnerRect.Width)*cw)/2)
		v.offsetY = max(0, (r.H-float32(v.pane.InnerRect.Height)*ch)/2)
	}
	r.X += v.offsetX
	r.Y += v.offsetY
	r.W -= v.offsetX
	r.H -= v.offsetY
	font := fontOf(v.cfg.Font, 0)
	m := font.Metrics()
	baseline := (ch-m.Ascent-m.Descent)/2 + m.Ascent
	w, h := int(v.pane.InnerRect.Width), int(v.pane.InnerRect.Height)
	h = min(h, int(math.Ceil(float64(r.H/ch))))
	w = min(w, int(math.Ceil(float64(r.W/cw))))
	for y := 0; y < h; y++ {
		for col := 0; col < w; {
			x := col
			columns := v.cellColumns(x, y)
			col += columns
			cell := v.cell(x, y)
			fg, bg := v.colors(cell)
			i := y*int(v.pane.InnerRect.Width) + x
			n := sort.Search(len(v.matches), func(n int) bool { return v.matches[n].end > i })
			if n < len(v.matches) && i >= v.matches[n].start {
				bg = ui.RGB(230, 190, 70)
				fg = ui.RGB(0, 0, 0)
				if n == v.match {
					bg = ui.RGB(255, 145, 65)
				}
			}
			if v.selected && i+columns-1 >= min(v.anchor, v.head) && i <= max(v.anchor, v.head) {
				bg = v.theme.Selection
				if bg.A == 0 {
					bg = ui.RGB(60, 100, 160)
				}
				if v.theme.SelectionText.A != 0 {
					fg = v.theme.SelectionText
				}
			}
			box := ui.Rect{X: r.X + float32(x)*cw, Y: r.Y + float32(y)*ch, W: cw, H: ch}
			if cell.Skip {
				continue
			}
			box.W = float32(columns) * cw
			p.Fill(box, bg, 0)
			blink := cell.Modifier&(16|32) != 0
			if blink {
				period := 500 * time.Millisecond
				if cell.Modifier&32 != 0 {
					period = 250 * time.Millisecond
				}
				p.After(period)
				if (p.Now().UnixNano()/int64(period))%2 != 0 {
					continue
				}
			}
			if cell.Modifier&128 != 0 {
				continue
			}
			width := float32(columns) * cw
			box.W = width
			p.Clip(box, 0, func() { p.Glyphs(v.shape(cell.Symbol, cell.Modifier), box.X, box.Y+baseline, fg) })
			if cell.Modifier&8 != 0 {
				style := cell.Modifier >> 12
				switch style {
				case 2:
					p.Line(box.X, box.Y+ch-4, box.X+width, box.Y+ch-4, 1, fg)
					p.Line(box.X, box.Y+ch-2, box.X+width, box.Y+ch-2, 1, fg)
				case 3:
					for dx := float32(0); dx < width; dx += 2 {
						dy := float32(0)
						if int(dx)%4 == 0 {
							dy = 1
						}
						p.Fill(ui.Rect{X: box.X + dx, Y: box.Y + ch - 3 + dy, W: min(2, width-dx), H: 1}, fg, 0)
					}
				case 4, 5:
					step := float32(3)
					length := float32(1)
					if style == 5 {
						step = 5
						length = 3
					}
					for dx := float32(0); dx < width; dx += step {
						p.Line(box.X+dx, box.Y+ch-2, box.X+min(dx+length, width), box.Y+ch-2, 1, fg)
					}
				default:
					p.Line(box.X, box.Y+ch-2, box.X+width, box.Y+ch-2, 1, fg)
				}
			}
			if cell.Modifier&256 != 0 {
				p.Line(box.X, box.Y+ch/2, box.X+width, box.Y+ch/2, 1, fg)
			}
		}
	}
	if v.UnsupportedGraphics() {
		p.Text(r.X+4, r.Y+4, "Image graphics are not supported", 11, v.theme.Foreground)
	}
	cur := v.frame.Frame.Cursor
	if cur != nil && cur.Visible && cur.Shape%2 == 1 {
		p.After(500 * time.Millisecond)
	}
	if cur != nil && cur.Visible && (cur.Shape%2 == 0 || (p.Now().UnixNano()/int64(500*time.Millisecond))%2 == 0) {
		cr := v.caret()
		if cr.X >= 0 && cr.Y >= 0 && cr.X-v.offsetX < float32(w)*cw && cr.Y-v.offsetY < float32(h)*ch {
			cr.X += r.X - v.offsetX
			cr.Y += r.Y - v.offsetY
			color := v.theme.Cursor
			if color.A == 0 {
				color = v.theme.Foreground
			}
			if cur.Shape == 3 || cur.Shape == 4 {
				cr.Y += ch - 2
				cr.H = 2
			} else if cur.Shape == 5 || cur.Shape == 6 {
				cr.W = 2
			} else {
				cr.H = ch
				color = color.Alpha(.4)
			}
			p.Fill(cr, color, 0)
		}
	}
	if v.preedit != "" {
		text := v.preedit
		v.preedit = ""
		cr := v.caret()
		v.preedit = text
		cr.X += r.X - v.offsetX
		cr.Y += r.Y - v.offsetY
		g := v.shape(v.preedit, 0)
		p.Fill(ui.Rect{X: cr.X, Y: cr.Y, W: cw * float32(uniseg.StringWidth(v.preedit)), H: ch}, v.theme.Background, 0)
		p.Glyphs(g, cr.X, cr.Y+baseline, v.theme.Foreground)
		p.Line(cr.X, cr.Y+ch-1, cr.X+cw*float32(uniseg.StringWidth(v.preedit)), cr.Y+ch-1, 1, v.theme.Foreground)
	}
}
