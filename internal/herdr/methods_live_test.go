package herdr

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveMethods(t *testing.T) {
	if os.Getenv("HERDR_UI_LIVE") == "" {
		t.Skip("set HERDR_UI_LIVE=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	endpoint, err := Connect(ctx, EndpointOptions{CellWidthPx: 8, CellHeightPx: 17, Cols: 120, Rows: 40, SurfaceActive: true})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer endpoint.Close()
	w := endpoint.Welcome()
	for _, m := range w.Methods {
		t.Logf("method: %s", m)
	}
	t.Logf("capabilities: %v", w.Capabilities)
}
