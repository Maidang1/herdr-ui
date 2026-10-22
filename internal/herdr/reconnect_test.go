package herdr

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTheClientComesBackWhenTheDaemonDropsIt says the window keeps its
// session when the daemon drops the connection: the break is reported, and
// the endpoint opens the connection again on its own.
func TestTheClientComesBackWhenTheDaemonDropsIt(t *testing.T) {
	socket := filepath.Join("/tmp", "herdr-ui-drop-"+strings.ReplaceAll(t.Name(), "/", "-")+".sock")
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		listener.Close()
		_ = os.Remove(socket)
	})

	accepted := make(chan int, 4)
	go func() {
		count := 0
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			count++
			accepted <- count
			go func(conn net.Conn, first bool) {
				defer conn.Close()
				payload, err := readFrame(conn)
				if err != nil {
					return
				}
				control, err := decodeEndpointControl(payload)
				if err != nil || control.Kind != "endpoint.hello.v1" {
					return
				}
				welcome, _ := json.Marshal(map[string]any{
					"generation":     1,
					"server_version": "0.0.0",
					"methods":        []string{"pane.focus"},
					"capabilities":   []string{},
				})
				if err := writeFrame(conn, encodeEndpointControl("endpoint.welcome.v1", string(welcome))); err != nil {
					return
				}
				if first {
					// The daemon goes away.
					time.Sleep(150 * time.Millisecond)
					return
				}
				// The second connection stays.
				for {
					if _, err := readFrame(conn); err != nil {
						return
					}
				}
			}(conn, count == 1)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	endpoint, err := Connect(ctx, EndpointOptions{
		SocketPath: socket, CellWidthPx: 8, CellHeightPx: 17, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer endpoint.Close()

	// The daemon drops the connection: the break is reported.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if endpoint.Outage() != nil {
			break
		}
		select {
		case <-endpoint.OutageChanged():
		case <-time.After(100 * time.Millisecond):
		}
	}
	outage := endpoint.Outage()
	if outage == nil {
		t.Fatal("the break was not reported")
	}
	if outage.Reason == "" {
		t.Fatal("the break has no reason")
	}

	// The endpoint opens the connection again on its own.
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if endpoint.Outage() == nil {
			break
		}
		select {
		case <-endpoint.OutageChanged():
		case <-time.After(100 * time.Millisecond):
		}
	}
	if endpoint.Outage() != nil {
		t.Fatalf("the connection did not come back: %v", endpoint.Outage().Reason)
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case count := <-accepted:
			if count >= 2 {
				return
			}
		case <-time.After(200 * time.Millisecond):
		}
	}
	t.Fatal("the daemon saw no second connection")
}
