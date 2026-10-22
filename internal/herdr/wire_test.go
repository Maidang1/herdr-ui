package herdr

import (
	"encoding/hex"
	"encoding/json"
	"testing"
)

// The hello frame below is what the Rust client sends. The daemon reads it
// as the first message of a connection, so these bytes are a contract.
func TestHelloFrameMatchesTheRustClient(t *testing.T) {
	hello := `{"generation":1,"cell_width_px":8,"cell_height_px":17,"surface_size":{"cols":120,"rows":40},` +
		`"pixel_mouse":false,"direct_graphics":false,"endpoint_keybindings":false,"mouse_capture":false,` +
		`"surface_active":true,"surface_reuse":true,"surface_delta":true,"surface_scroll":true,` +
		`"snapshot_codecs":["shell.snapshot.v1"],"surface_codecs":["shell.surface.v1"],` +
		`"input_codecs":["shell.input.semantic.v1"],"blob_codecs":["shell.blob.v1"]}`
	payload := encodeEndpointControl("endpoint.hello.v1", hello)
	want := "1411656e64706f696e742e68656c6c6f2e7631fbaa01" + hex.EncodeToString([]byte(hello))
	if got := hex.EncodeToString(payload); got != want {
		t.Fatalf("the hello frame came out as\n%s\nwant\n%s", got, want)
	}
}

func TestEndpointControlRoundTrip(t *testing.T) {
	payload := encodeEndpointControl("shell.snapshot.v1", `{"boot_id":"x"}`)
	control, err := decodeEndpointControl(payload)
	if err != nil {
		t.Fatal(err)
	}
	if control.Kind != "shell.snapshot.v1" {
		t.Fatalf("the kind came back as %q", control.Kind)
	}
	if control.Data != `{"boot_id":"x"}` {
		t.Fatalf("the data came back as %q", control.Data)
	}
}

func TestResponseChunkRoundTrip(t *testing.T) {
	encoder := bincodeEncoder()
	encoder.Variant(msgServerClientShellEndpointResponse)
	encoder.String("boot-1")
	encoder.String("req-1")
	encoder.Bool(true)
	encoder.BytesOf([]byte(`{"id":"req-1","result":{}}`))

	chunk, err := decodeResponseChunk(encoder.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if chunk.BootID != "boot-1" || chunk.RequestID != "req-1" || !chunk.FinalChunk {
		t.Fatalf("the chunk came back as %+v", chunk)
	}
	if string(chunk.Data) != `{"id":"req-1","result":{}}` {
		t.Fatalf("the data came back as %q", chunk.Data)
	}
}

func TestPaneInputCarriesTheText(t *testing.T) {
	payload := encodePaneInput("w1:p1", "hello")
	decoder := newVariantReader(payload)
	variant, ok := decoder.variant()
	if !ok {
		t.Fatal("the variant could not be read")
	}
	if variant != msgClientShellPaneInput {
		t.Fatalf("the variant came back as %d", variant)
	}
	// The message holds the pane, then one event: the text.
	dec := newPayloadDecoder(payload)
	pane, err := dec.String()
	if err != nil {
		t.Fatal(err)
	}
	if pane != "w1:p1" {
		t.Fatalf("the pane came back as %q", pane)
	}
	count, err := dec.U64()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("the event count came back as %d", count)
	}
	event, err := dec.Variant()
	if err != nil {
		t.Fatal(err)
	}
	if event != inputTextCommit {
		t.Fatalf("the event came back as %d", event)
	}
	text, err := dec.String()
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Fatalf("the text came back as %q", text)
	}
}

func TestSurfaceFrameDecodesTheCells(t *testing.T) {
	encoder := bincodeEncoder()
	encoder.Variant(msgServerPaneSurface)
	encoder.String("boot-1")
	encoder.U64(7)
	encoder.U64(9)
	encoder.U64(2) // two cells
	encoder.String("A")
	encoder.U32(0x0200FF00)
	encoder.U32(0)
	encoder.U16(0)
	encoder.Bool(false)
	encoder.Option(false)
	encoder.String("B")
	encoder.U32(0)
	encoder.U32(0x020000FF)
	encoder.U16(0)
	encoder.Bool(false)
	encoder.Option(false)
	encoder.U16(2) // width
	encoder.U16(1) // height
	encoder.Option(false)
	encoder.U64(0) // no hyperlinks
	encoder.U64(0) // no graphics
	encoder.U64(1) // one pane
	encoder.String("w1:p1")
	encoder.U64(3)
	encoder.U16(0)
	encoder.U16(0)
	encoder.U16(2)
	encoder.U16(1)
	encoder.U16(0)
	encoder.U16(0)
	encoder.U16(2)
	encoder.U16(1)
	encoder.Option(false)
	encoder.Option(false)
	encoder.Bool(true)
	encoder.Bool(false)
	encoder.Bool(false)
	encoder.Bool(false)
	encoder.U32(160)
	encoder.U32(18)
	encoder.U64(0) // no splits
	// The popup and the graphics scene follow; this client stops before them.

	frame, err := decodeSurfaceFrame(encoder.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if frame.BootID != "boot-1" || frame.SurfaceRevision != 9 {
		t.Fatalf("the frame came back as %+v", frame)
	}
	if len(frame.Grid.Cells) != 2 {
		t.Fatalf("the grid holds %d cells", len(frame.Grid.Cells))
	}
	if frame.Grid.Cells[0].Symbol != "A" || frame.Grid.Cells[1].Symbol != "B" {
		t.Fatalf("the cells came back as %+v", frame.Grid.Cells)
	}
	if frame.Grid.Width != 2 || frame.Grid.Height != 1 {
		t.Fatalf("the grid came back as %dx%d", frame.Grid.Width, frame.Grid.Height)
	}
	if len(frame.Panes) != 1 || frame.Panes[0].PaneID != "w1:p1" || !frame.Panes[0].Focused {
		t.Fatalf("the panes came back as %+v", frame.Panes)
	}
}

func TestSnapshotReadsTheTokensAndThePairs(t *testing.T) {
	raw := `{
		"boot_id": "boot-1",
		"revision": 4,
		"focused_workspace_id": "w1",
		"workspaces": [{
			"workspace_id": "w1", "active_tab_id": "w1:t1", "number": 1, "label": "dsh",
			"git_ahead_behind": [2, 3], "tokens": [["files", "12"]], "focused": true,
			"agent_status": "working"
		}],
		"tabs": [{"tab_id": "w1:t1", "workspace_id": "w1", "number": 1, "label": "1", "focused": true}],
		"panes": [{"pane_id": "w1:p1", "workspace_id": "w1", "tab_id": "w1:t1", "cwd": "/tmp", "focused": true}],
		"agents": [{"pane_id": "w1:p1", "workspace_id": "w1", "tab_id": "w1:t1", "agent": "grok", "agent_status": "idle"}]
	}`
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.BootID != "boot-1" {
		t.Fatalf("the boot id came back as %q", snapshot.BootID)
	}
	workspace, ok := snapshot.Workspace("w1")
	if !ok {
		t.Fatal("the workspace is missing")
	}
	if workspace.GitAheadBehind == nil || workspace.GitAheadBehind.Ahead != 2 || workspace.GitAheadBehind.Behind != 3 {
		t.Fatalf("the pair came back as %+v", workspace.GitAheadBehind)
	}
	if len(workspace.Tokens) != 1 || workspace.Tokens[0].Name != "files" || workspace.Tokens[0].Value != "12" {
		t.Fatalf("the tokens came back as %+v", workspace.Tokens)
	}
	if len(snapshot.TabsOf("w1")) != 1 {
		t.Fatalf("the workspace holds %d tabs", len(snapshot.TabsOf("w1")))
	}
	if len(snapshot.PanesOf("w1:t1")) != 1 {
		t.Fatalf("the tab holds %d panes", len(snapshot.PanesOf("w1:t1")))
	}
	agent, ok := snapshot.AgentOf("w1:p1")
	if !ok {
		t.Fatal("the agent is missing")
	}
	if agent.Agent == nil || *agent.Agent != "grok" {
		t.Fatalf("the agent came back as %+v", agent)
	}
}
