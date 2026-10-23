package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func serve(t *testing.T, handle func(net.Conn)) Target {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "herdr-wire-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); handle(c) }()
		}
	}()
	return Target{Socket: path}
}

func TestClientSerialCallsUseOneRequestPerConnection(t *testing.T) {
	var mu sync.Mutex
	connections, requests := 0, 0
	ids := map[string]bool{}
	target := serve(t, func(c net.Conn) {
		mu.Lock()
		connections++
		mu.Unlock()
		r := bufio.NewReader(c)
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				return
			}
			var req struct {
				ID string `json:"id"`
			}
			json.Unmarshal(line, &req)
			mu.Lock()
			requests++
			if ids[req.ID] {
				t.Error("duplicate request ID")
			}
			ids[req.ID] = true
			mu.Unlock()
			fmt.Fprintf(c, "{\"id\":%q,\"result\":{\"type\":\"pong\"}}\n", req.ID)
			return // Actual Herdr closes ordinary connections after one RPC.
		}
	})
	client, err := Dial(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var result any
			if err := client.Call(context.Background(), "ping", nil, &result); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if connections != 8 || requests != 8 {
		t.Fatalf("connections=%d requests=%d", connections, requests)
	}
}

func TestSubscribeAcknowledgesBeforeReadingEvents(t *testing.T) {
	target := serve(t, func(c net.Conn) {
		r := bufio.NewReader(c)
		line, _ := r.ReadBytes('\n')
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(line, &req)
		fmt.Fprintf(c, "{\"id\":%q,\"result\":{\"type\":\"subscription_started\"}}\n{\"event\":\"pane.created\",\"data\":{}}\n", req.ID)
		r.ReadByte()
	})
	sub, err := Subscribe(context.Background(), target, []Subscription{{Type: "pane.created"}})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	event, err := sub.Next()
	if err != nil {
		t.Fatal(err)
	}
	if event.Name != "pane.created" {
		t.Fatal(event)
	}
	sub.Close()
	if _, err := sub.Next(); err == nil {
		t.Fatal("read after close succeeded")
	}
}

func TestSubscribeRejectsBadAck(t *testing.T) {
	for _, reply := range []string{`{"id":"wrong","result":{"type":"subscription_started"}}`, `{"event":"pane.created","data":{}}`} {
		t.Run(reply, func(t *testing.T) {
			target := serve(t, func(c net.Conn) { bufio.NewReader(c).ReadBytes('\n'); fmt.Fprintln(c, reply) })
			if sub, err := Subscribe(context.Background(), target, []Subscription{{Type: "pane.created"}}); err == nil {
				sub.Close()
				t.Fatal("bad ack accepted")
			}
		})
	}
}

func TestSubscriptionContextEndsBlockedRead(t *testing.T) {
	target := serve(t, func(c net.Conn) {
		r := bufio.NewReader(c)
		line, _ := r.ReadBytes('\n')
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(line, &req)
		fmt.Fprintf(c, "{\"id\":%q,\"result\":{\"type\":\"subscription_started\"}}\n", req.ID)
		r.ReadByte()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub, err := Subscribe(ctx, target, []Subscription{{Type: "pane.created"}})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	done := make(chan error, 1)
	go func() { _, err := sub.Next(); done <- err }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("missing cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock")
	}
}
