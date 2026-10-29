package endpoint

import (
	"encoding/hex"
	"math"
	"reflect"
	"testing"
)

// Golden payloads generated with herdr-protocol::encode_message (bincode 2
// standard) from the sibling authoritative Rust client protocol. They include
// UTF-8 graphemes, uint16/uint32 varints, optional links, cursor and scroll data.
const rustSurfaceGolden = "0d04626f6f7407fbfb000103e7958cfc332211fffb0001fb010100010001010100000106011368747470733a2f2f6578616d706c652e636f6d00010470616e65fb2c010000010100000101000102fbf40101010100000c180000000000"
const rustPatchGolden = "1304626f6f7407fbfb00fbfc00010000010161fc332211fffb0001fb0101000100010470616e65fb2c010000010100000101000102fbf40101010100000c1800"

func goldenBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func goldenSurface(t *testing.T) *SurfaceFrame {
	t.Helper()
	s, err := DecodeSurfaceFrame(goldenBytes(t, rustSurfaceGolden))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestRustSurfaceGolden(t *testing.T) {
	s := goldenSurface(t)
	if s.BootID != "boot" || s.ProjectionRevision != 7 || s.SurfaceRevision != 251 || s.Frame.Width != 1 || s.Frame.Height != 1 {
		t.Fatalf("wrong identity/grid: %+v", s)
	}
	c := s.Frame.Cells[0]
	if c.Symbol != "界" || c.FG != 0xff112233 || c.BG != 256 || c.Modifier != 257 || c.Skip || c.Hyperlink == nil || *c.Hyperlink != 0 {
		t.Fatalf("wrong cell: %+v", c)
	}
	if s.Frame.Cursor == nil || s.Frame.Cursor.Shape != 6 || !s.Frame.Cursor.Visible {
		t.Fatalf("wrong cursor %+v", s.Frame.Cursor)
	}
	p := s.Panes[0]
	if p.PaneID != "pane" || p.ContentRevision != 300 || p.Scroll == nil || p.Scroll.MaxOffsetFromBottom != 500 || !p.Focused || !p.MouseReporting || p.PixelWidth != 12 || p.PixelHeight != 24 {
		t.Fatalf("wrong pane %+v", p)
	}
}
func TestRustPatchGolden(t *testing.T) {
	s := goldenSurface(t)
	p, err := DecodeSurfacePatch(goldenBytes(t, rustPatchGolden))
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	if v.SurfaceRevision != 252 || v.Frame.Cells[0].Symbol != "a" || v.Frame.Cursor != nil {
		t.Fatalf("wrong patch result %+v", v)
	}
	if s.SurfaceRevision != 251 || s.Frame.Cells[0].Symbol != "界" || s.Frame.Cursor == nil {
		t.Fatal("patch mutated baseline")
	}
	*v.Frame.Cells[0].Hyperlink = 9
	v.Panes[0].Scroll.OffsetFromBottom = 99
	if *s.Frame.Cells[0].Hyperlink != 0 || s.Panes[0].Scroll.OffsetFromBottom != 2 {
		t.Fatal("patch result aliases baseline")
	}
}
func TestSurfaceRejectsTruncationAndTrailingData(t *testing.T) {
	for _, fixture := range []struct {
		hex    string
		decode func([]byte) error
	}{{rustSurfaceGolden, func(b []byte) error { _, e := DecodeSurfaceFrame(b); return e }}, {rustPatchGolden, func(b []byte) error { _, e := DecodeSurfacePatch(b); return e }}} {
		b := goldenBytes(t, fixture.hex)
		for i := 0; i < len(b); i++ {
			if fixture.decode(b[:i]) == nil {
				t.Fatalf("accepted prefix %d/%d", i, len(b))
			}
		}
		if fixture.decode(append(b, 0)) == nil {
			t.Fatal("accepted trailing byte")
		}
	}
}
func TestSurfaceBounds(t *testing.T) {
	for _, b := range [][]byte{{SurfaceFrameTag, 253, 255, 255, 255, 255, 255, 255, 255, 255}, {SurfaceFrameTag, 1, 255}, {SurfaceFrameTag, 0, 0, 0, 253, 255, 255, 255, 255, 255, 255, 255, 255}, make([]byte, MaxFrameSize+1)} {
		if _, err := DecodeSurfaceFrame(b); err == nil {
			t.Fatal("accepted invalid surface")
		}
	}
	d := decoder{b: appendUint(nil, 65536)}
	d.u16()
	if d.err == nil {
		t.Fatal("uint16 overflow accepted")
	}
	d = decoder{b: []byte{2}}
	d.boolean()
	if d.err == nil {
		t.Fatal("invalid bool accepted")
	}
	d = decoder{b: []byte{2}}
	d.assetKey()
	if d.err == nil {
		t.Fatal("unknown enum accepted")
	}
	d = decoder{b: []byte{1, 0, 0, 1, 7}}
	d.cursor()
	if d.err == nil {
		t.Fatal("unknown cursor shape accepted")
	}
}
func TestSurfacePatchAtomicFences(t *testing.T) {
	tests := []struct {
		name   string
		change func(*SurfaceFrame, *SurfacePatch)
	}{{"boot", func(s *SurfaceFrame, p *SurfacePatch) { p.BootID = "other" }}, {"projection", func(s *SurfaceFrame, p *SurfacePatch) { p.ProjectionRevision++ }}, {"base", func(s *SurfaceFrame, p *SurfacePatch) { p.BaseSurfaceRevision-- }}, {"stale", func(s *SurfaceFrame, p *SurfacePatch) { p.SurfaceRevision = s.SurfaceRevision }}, {"gap", func(s *SurfaceFrame, p *SurfacePatch) { p.SurfaceRevision++ }}, {"overflow", func(s *SurfaceFrame, p *SurfacePatch) {
		s.SurfaceRevision = math.MaxUint64
		p.BaseSurfaceRevision = math.MaxUint64
		p.SurfaceRevision = 0
	}}, {"popup", func(s *SurfaceFrame, p *SurfacePatch) { s.Popup = &SurfacePopup{} }}, {"row", func(s *SurfaceFrame, p *SurfacePatch) { p.Rows[0].X = 1 }}, {"link", func(s *SurfaceFrame, p *SurfacePatch) { n := uint32(1); p.Rows[0].Cells[0].Hyperlink = &n }}, {"pane", func(s *SurfaceFrame, p *SurfacePatch) { p.Panes[0].InnerRect.Width++ }}, {"cursor", func(s *SurfaceFrame, p *SurfacePatch) { p.Cursor = &SurfaceCursor{X: 1, Visible: true} }}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := goldenSurface(t)
			p, err := DecodeSurfacePatch(goldenBytes(t, rustPatchGolden))
			if err != nil {
				t.Fatal(err)
			}
			tt.change(s, p)
			before := s.Clone()
			if _, err = s.Apply(p); err == nil {
				t.Fatal("invalid patch accepted")
			}
			if !reflect.DeepEqual(before, s) {
				t.Fatal("failed patch mutated baseline")
			}
		})
	}
}
func FuzzSurfaceDecode(f *testing.F) {
	b, _ := hex.DecodeString(rustSurfaceGolden)
	f.Add(b)
	b, _ = hex.DecodeString(rustPatchGolden)
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = DecodeSurfaceFrame(b); _, _ = DecodeSurfacePatch(b) })
}

const rustSurfaceExtraGolden = "0d04626f6f7409fb2c010000000000000001010200000203010001030200010103706f7005506f7075700100fb2c0101014b0000000000021b5b0100fb2c01fb900101000103706f70fb2d0101010104fc40e20100040102030401000103706f70fb2d0101010104fc40e2010005060708090a0b0c0d0e0ffbe7031001000103706f70fb2d0101010104fc40e20100"

func TestRustSurfacePopupGraphicsGolden(t *testing.T) {
	s, err := DecodeSurfaceFrame(goldenBytes(t, rustSurfaceExtraGolden))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Splits) != 1 || s.Splits[0].Direction != 1 || !reflect.DeepEqual(s.Splits[0].Path, []bool{false, true}) {
		t.Fatalf("wrong split %+v", s.Splits)
	}
	p := s.Popup
	if p == nil || p.TerminalID != "pop" || p.Title != "Popup" || p.Width == nil || p.Width.Kind != 0 || p.Width.Value != 300 || p.Height == nil || p.Height.Kind != 1 || p.Height.Value != 75 || !p.MouseReporting || p.PixelHeight != 400 {
		t.Fatalf("wrong popup %+v", p)
	}
	if len(s.Graphics.Assets) != 1 || len(s.Graphics.Placements) != 1 || len(s.Graphics.RetainedAssets) != 1 {
		t.Fatalf("wrong scene %+v", s.Graphics)
	}
	a := s.Graphics.Assets[0]
	if a.Key.Source.Kind != 0 || a.Key.Source.TargetKind != 1 || a.Key.Source.TerminalID != "pop" || a.Key.Source.ImageID != 301 || a.Key.Format != 1 || a.Key.DataFingerprint != 123456 || !reflect.DeepEqual(a.Data, []byte{1, 2, 3, 4}) {
		t.Fatalf("wrong asset %+v", a)
	}
	place := s.Graphics.Placements[0]
	if place.Z != -500 || place.X != 6 || place.Y != 7 || place.Cols != 8 || place.Rows != 9 || place.SourceX != 10 || place.SourceY != 11 || place.SourceWidth != 12 || place.SourceHeight != 13 || place.XOffset != 14 || place.YOffset != 15 || place.ScrollbackOffset != 16 {
		t.Fatalf("wrong placement %+v", place)
	}
	v := s.Clone()
	v.Popup.Width.Value = 1
	v.Popup.Frame.Graphics[0] = 0
	v.Splits[0].Path[0] = true
	v.Graphics.Assets[0].Data[0] = 0
	if s.Popup.Width.Value != 300 || s.Popup.Frame.Graphics[0] != 27 || s.Splits[0].Path[0] || s.Graphics.Assets[0].Data[0] != 1 {
		t.Fatal("clone aliases nested metadata")
	}
	for i := 0; i < len(goldenBytes(t, rustSurfaceExtraGolden)); i++ {
		if _, err := DecodeSurfaceFrame(goldenBytes(t, rustSurfaceExtraGolden)[:i]); err == nil {
			t.Fatalf("accepted extra prefix %d", i)
		}
	}
}
