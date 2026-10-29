package endpoint

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

const SurfaceFrameTag = 13
const SurfacePatchTag = 19
const maxSurfaceCells = 262144

// Surface values are immutable after publication. Clone returns an independent copy.
type SurfaceCell struct {
	Symbol    string
	FG, BG    uint32
	Modifier  uint16
	Skip      bool
	Hyperlink *uint32
}
type SurfaceCursor struct {
	X, Y    uint16
	Visible bool
	Shape   uint8
}
type SurfaceRect struct{ X, Y, Width, Height uint16 }
type SurfaceScroll struct{ OffsetFromBottom, MaxOffsetFromBottom, ViewportRows uint64 }
type SurfaceGrid struct {
	Cells         []SurfaceCell
	Width, Height uint16
	Cursor        *SurfaceCursor
	Hyperlinks    []string
	Graphics      []byte
}
type SurfacePane struct {
	PaneID                                                        string
	ContentRevision                                               uint64
	Rect, InnerRect                                               SurfaceRect
	ScrollbarRect                                                 *SurfaceRect
	Scroll                                                        *SurfaceScroll
	Focused, MouseReporting, SGRPixelMouse, AlternateScreenActive bool
	PixelWidth, PixelHeight                                       uint32
}
type SurfaceSplit struct {
	Direction     uint8
	Pos           uint16
	Area, HitRect SurfaceRect
	Path          []bool
}
type SurfacePopupSize struct {
	Kind  uint8
	Value uint16
}
type SurfacePopup struct {
	TerminalID, Title             string
	Width, Height                 *SurfacePopupSize
	Frame                         SurfaceGrid
	MouseReporting, SGRPixelMouse bool
	PixelWidth, PixelHeight       uint32
}
type SurfaceGraphicsSource struct {
	Kind, TargetKind            uint8
	PaneID, TerminalID, LayerID string
	ImageID                     uint32
}
type SurfaceGraphicsAssetKey struct {
	Source                   SurfaceGraphicsSource
	ImageWidth, ImageHeight  uint32
	Format                   uint8
	DataLen, DataFingerprint uint64
}
type SurfaceGraphicsAsset struct {
	Key  SurfaceGraphicsAssetKey
	Data []byte
}
type SurfaceGraphicsPlacement struct {
	Asset                                                                     SurfaceGraphicsAssetKey
	LogicalPlacementID                                                        uint32
	X, Y                                                                      uint16
	Cols, Rows, SourceX, SourceY, SourceWidth, SourceHeight, XOffset, YOffset uint32
	Z                                                                         int32
	ScrollbackOffset                                                          uint32
}
type SurfaceGraphicsScene struct {
	Assets         []SurfaceGraphicsAsset
	Placements     []SurfaceGraphicsPlacement
	RetainedAssets []SurfaceGraphicsAssetKey
}
type SurfaceFrame struct {
	BootID                              string
	ProjectionRevision, SurfaceRevision uint64
	Frame                               SurfaceGrid
	Panes                               []SurfacePane
	Splits                              []SurfaceSplit
	Popup                               *SurfacePopup
	Graphics                            SurfaceGraphicsScene
}
type SurfacePatchRow struct {
	X, Y  uint16
	Cells []SurfaceCell
}
type SurfacePatch struct {
	BootID                                                   string
	ProjectionRevision, BaseSurfaceRevision, SurfaceRevision uint64
	Rows                                                     []SurfacePatchRow
	Panes                                                    []SurfacePane
	Cursor                                                   *SurfaceCursor
}

func (d *decoder) bounded(max uint64) uint64 {
	n := d.uint()
	if n > max && d.err == nil {
		d.err = errors.New("surface integer exceeds bounds")
		return 0
	}
	return n
}
func (d *decoder) u16() uint16 { return uint16(d.bounded(math.MaxUint16)) }
func (d *decoder) u32() uint32 { return uint32(d.bounded(math.MaxUint32)) }
func (d *decoder) byteValue() uint8 {
	b := d.take(1)
	if b == nil {
		return 0
	}
	return b[0]
}
func (d *decoder) boolean() bool {
	n := d.byteValue()
	if n > 1 {
		d.err = errors.New("invalid surface boolean")
	}
	return n == 1
}
func (d *decoder) count(max uint64) int {
	n := d.bounded(max)
	if n > uint64(len(d.b)) && d.err == nil {
		d.err = errors.New("surface collection exceeds remaining payload")
	}
	if d.err != nil {
		return 0
	}
	return int(n)
}
func (d *decoder) bytes() []byte {
	n := d.bounded(MaxFrameSize)
	return append([]byte(nil), d.take(n)...)
}
func (d *decoder) rect() SurfaceRect { return SurfaceRect{d.u16(), d.u16(), d.u16(), d.u16()} }
func (d *decoder) cursor() *SurfaceCursor {
	if !d.option() {
		return nil
	}
	c := &SurfaceCursor{d.u16(), d.u16(), d.boolean(), d.byteValue()}
	if c.Shape > 6 {
		d.err = errors.New("unknown cursor shape")
	}
	return c
}
func (d *decoder) cells() []SurfaceCell {
	out := make([]SurfaceCell, d.count(maxSurfaceCells))
	for i := range out {
		c := &out[i]
		c.Symbol = d.str()
		c.FG = d.u32()
		c.BG = d.u32()
		c.Modifier = d.u16()
		c.Skip = d.boolean()
		if d.option() {
			n := d.u32()
			c.Hyperlink = &n
		}
	}
	return out
}
func (d *decoder) grid() SurfaceGrid {
	g := SurfaceGrid{}
	g.Cells = d.cells()
	g.Width = d.u16()
	g.Height = d.u16()
	g.Cursor = d.cursor()
	g.Hyperlinks = make([]string, d.count(65536))
	for i := range g.Hyperlinks {
		g.Hyperlinks[i] = d.str()
	}
	g.Graphics = d.bytes()
	if err := g.Validate(); err != nil && d.err == nil {
		d.err = err
	}
	return g
}
func (d *decoder) panes() []SurfacePane {
	out := make([]SurfacePane, d.count(4096))
	for i := range out {
		p := &out[i]
		p.PaneID = d.str()
		p.ContentRevision = d.uint()
		p.Rect = d.rect()
		p.InnerRect = d.rect()
		if d.option() {
			r := d.rect()
			p.ScrollbarRect = &r
		}
		if d.option() {
			p.Scroll = &SurfaceScroll{d.uint(), d.uint(), d.uint()}
		}
		p.Focused = d.boolean()
		p.MouseReporting = d.boolean()
		p.SGRPixelMouse = d.boolean()
		p.AlternateScreenActive = d.boolean()
		p.PixelWidth = d.u32()
		p.PixelHeight = d.u32()
	}
	return out
}
func (d *decoder) popupSize() *SurfacePopupSize {
	if !d.option() {
		return nil
	}
	n := d.bounded(1)
	s := &SurfacePopupSize{Kind: uint8(n)}
	if n == 0 {
		s.Value = d.u16()
	} else {
		s.Value = uint16(d.byteValue())
	}
	return s
}
func (d *decoder) assetKey() SurfaceGraphicsAssetKey {
	k := SurfaceGraphicsAssetKey{}
	k.Source.Kind = uint8(d.bounded(1))
	if k.Source.Kind == 0 {
		k.Source.TargetKind = uint8(d.bounded(1))
		if k.Source.TargetKind == 0 {
			k.Source.PaneID = d.str()
		} else {
			k.Source.TerminalID = d.str()
		}
		k.Source.ImageID = d.u32()
	} else {
		k.Source.PaneID = d.str()
		k.Source.LayerID = d.str()
	}
	k.ImageWidth = d.u32()
	k.ImageHeight = d.u32()
	k.Format = uint8(d.bounded(2))
	k.DataLen = d.bounded(MaxImageSize)
	k.DataFingerprint = d.uint()
	return k
}
func (d *decoder) scene() SurfaceGraphicsScene {
	s := SurfaceGraphicsScene{}
	s.Assets = make([]SurfaceGraphicsAsset, d.count(4096))
	for i := range s.Assets {
		a := &s.Assets[i]
		a.Key = d.assetKey()
		a.Data = d.bytes()
		if uint64(len(a.Data)) != a.Key.DataLen {
			d.err = errors.New("surface asset length mismatch")
		}
	}
	s.Placements = make([]SurfaceGraphicsPlacement, d.count(65536))
	for i := range s.Placements {
		p := &s.Placements[i]
		p.Asset = d.assetKey()
		p.LogicalPlacementID = d.u32()
		p.X = d.u16()
		p.Y = d.u16()
		p.Cols = d.u32()
		p.Rows = d.u32()
		p.SourceX = d.u32()
		p.SourceY = d.u32()
		p.SourceWidth = d.u32()
		p.SourceHeight = d.u32()
		p.XOffset = d.u32()
		p.YOffset = d.u32()
		z := d.bounded(math.MaxUint32)
		p.Z = int32(z>>1) ^ -int32(z&1)
		p.ScrollbackOffset = d.u32()
	}
	s.RetainedAssets = make([]SurfaceGraphicsAssetKey, d.count(4096))
	for i := range s.RetainedAssets {
		s.RetainedAssets[i] = d.assetKey()
	}
	return s
}

func surfaceDecoder(b []byte, tag uint64) decoder {
	d := decoder{b: b}
	if len(b) > MaxFrameSize {
		d.err = errors.New("surface payload exceeds limit")
		return d
	}
	if d.uint() != tag && d.err == nil {
		d.err = errors.New("unexpected surface message tag")
	}
	return d
}

// DecodeSurfaceFrame decodes a complete bincode ServerMessage payload, including its tag.
func DecodeSurfaceFrame(b []byte) (*SurfaceFrame, error) {
	d := surfaceDecoder(b, SurfaceFrameTag)
	s := &SurfaceFrame{}
	s.BootID = d.str()
	s.ProjectionRevision = d.uint()
	s.SurfaceRevision = d.uint()
	s.Frame = d.grid()
	s.Panes = d.panes()
	s.Splits = make([]SurfaceSplit, d.count(4096))
	for i := range s.Splits {
		p := &s.Splits[i]
		p.Direction = uint8(d.bounded(1))
		p.Pos = d.u16()
		p.Area = d.rect()
		p.HitRect = d.rect()
		p.Path = make([]bool, d.count(256))
		for j := range p.Path {
			p.Path[j] = d.boolean()
		}
	}
	if d.option() {
		s.Popup = &SurfacePopup{}
		p := s.Popup
		p.TerminalID = d.str()
		p.Title = d.str()
		p.Width = d.popupSize()
		p.Height = d.popupSize()
		p.Frame = d.grid()
		p.MouseReporting = d.boolean()
		p.SGRPixelMouse = d.boolean()
		p.PixelWidth = d.u32()
		p.PixelHeight = d.u32()
	}
	s.Graphics = d.scene()
	if err := d.finish(); err != nil {
		return nil, fmt.Errorf("surface frame: %w", err)
	}
	return s, nil
}

// DecodeSurfacePatch decodes a complete bincode ServerMessage payload, including its tag.
func DecodeSurfacePatch(b []byte) (*SurfacePatch, error) {
	d := surfaceDecoder(b, SurfacePatchTag)
	p := &SurfacePatch{}
	p.BootID = d.str()
	p.ProjectionRevision = d.uint()
	p.BaseSurfaceRevision = d.uint()
	p.SurfaceRevision = d.uint()
	p.Rows = make([]SurfacePatchRow, d.count(maxSurfaceCells))
	total := 0
	for i := range p.Rows {
		r := &p.Rows[i]
		r.X = d.u16()
		r.Y = d.u16()
		r.Cells = d.cells()
		total += len(r.Cells)
		if total > maxSurfaceCells {
			d.err = errors.New("surface patch cells exceed limit")
		}
	}
	p.Panes = d.panes()
	p.Cursor = d.cursor()
	if err := d.finish(); err != nil {
		return nil, fmt.Errorf("surface patch: %w", err)
	}
	return p, nil
}
func (g SurfaceGrid) Validate() error {
	if len(g.Cells) > maxSurfaceCells || len(g.Cells) != int(g.Width)*int(g.Height) {
		return errors.New("surface cell count mismatch")
	}
	for _, c := range g.Cells {
		if c.Hyperlink != nil && uint64(*c.Hyperlink) >= uint64(len(g.Hyperlinks)) {
			return errors.New("surface hyperlink outside table")
		}
	}
	if g.Cursor != nil && g.Cursor.Visible && (g.Cursor.X >= g.Width || g.Cursor.Y >= g.Height) {
		return errors.New("surface cursor outside grid")
	}
	return nil
}

func cloneCells(c []SurfaceCell) []SurfaceCell {
	out := slices.Clone(c)
	for i := range out {
		if out[i].Hyperlink != nil {
			v := *out[i].Hyperlink
			out[i].Hyperlink = &v
		}
	}
	return out
}
func cloneCursor(c *SurfaceCursor) *SurfaceCursor {
	if c == nil {
		return nil
	}
	v := *c
	return &v
}
func cloneGrid(g SurfaceGrid) SurfaceGrid {
	g.Cells = cloneCells(g.Cells)
	g.Cursor = cloneCursor(g.Cursor)
	g.Hyperlinks = slices.Clone(g.Hyperlinks)
	g.Graphics = slices.Clone(g.Graphics)
	return g
}
func clonePanes(p []SurfacePane) []SurfacePane {
	out := slices.Clone(p)
	for i := range out {
		if out[i].Scroll != nil {
			v := *out[i].Scroll
			out[i].Scroll = &v
		}
		if out[i].ScrollbarRect != nil {
			v := *out[i].ScrollbarRect
			out[i].ScrollbarRect = &v
		}
	}
	return out
}
func (s *SurfaceFrame) Clone() *SurfaceFrame {
	if s == nil {
		return nil
	}
	v := *s
	v.Frame = cloneGrid(s.Frame)
	v.Panes = clonePanes(s.Panes)
	v.Splits = slices.Clone(s.Splits)
	for i := range v.Splits {
		v.Splits[i].Path = append([]bool(nil), v.Splits[i].Path...)
	}
	if s.Popup != nil {
		p := *s.Popup
		p.Frame = cloneGrid(p.Frame)
		if p.Width != nil {
			w := *p.Width
			p.Width = &w
		}
		if p.Height != nil {
			h := *p.Height
			p.Height = &h
		}
		v.Popup = &p
	}
	v.Graphics.Assets = slices.Clone(s.Graphics.Assets)
	for i := range v.Graphics.Assets {
		v.Graphics.Assets[i].Data = append([]byte(nil), v.Graphics.Assets[i].Data...)
	}
	v.Graphics.Placements = slices.Clone(s.Graphics.Placements)
	v.Graphics.RetainedAssets = slices.Clone(s.Graphics.RetainedAssets)
	return &v
}

// Apply validates the entire patch before returning an independent next baseline.
// Stale or out-of-order patches are errors: the upstream client closes on these.
func (s *SurfaceFrame) Apply(p *SurfacePatch) (*SurfaceFrame, error) {
	if s == nil || p == nil || p.BootID != s.BootID || p.ProjectionRevision != s.ProjectionRevision || p.BaseSurfaceRevision != s.SurfaceRevision || s.SurfaceRevision == math.MaxUint64 || p.SurfaceRevision != s.SurfaceRevision+1 || s.Popup != nil {
		return nil, errors.New("surface patch identity mismatch")
	}
	if err := s.Frame.Validate(); err != nil {
		return nil, err
	}
	for _, r := range p.Rows {
		if r.Y >= s.Frame.Height || int(r.X)+len(r.Cells) > int(s.Frame.Width) {
			return nil, errors.New("surface patch row outside grid")
		}
		for _, c := range r.Cells {
			if c.Hyperlink != nil && uint64(*c.Hyperlink) >= uint64(len(s.Frame.Hyperlinks)) {
				return nil, errors.New("surface patch hyperlink outside table")
			}
		}
	}
	for _, pane := range p.Panes {
		found := false
		for _, old := range s.Panes {
			if old.PaneID == pane.PaneID && old.Rect == pane.Rect && old.InnerRect == pane.InnerRect {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("surface patch changed pane geometry")
		}
	}
	if p.Cursor != nil && p.Cursor.Visible && (p.Cursor.X >= s.Frame.Width || p.Cursor.Y >= s.Frame.Height) {
		return nil, errors.New("surface patch cursor outside grid")
	}
	v := s.Clone()
	for _, r := range p.Rows {
		start := int(r.Y)*int(v.Frame.Width) + int(r.X)
		copy(v.Frame.Cells[start:], cloneCells(r.Cells))
	}
	for _, pane := range clonePanes(p.Panes) {
		for i := range v.Panes {
			if v.Panes[i].PaneID == pane.PaneID {
				v.Panes[i] = pane
				break
			}
		}
	}
	v.Frame.Cursor = cloneCursor(p.Cursor)
	v.SurfaceRevision = p.SurfaceRevision
	return v, nil
}
