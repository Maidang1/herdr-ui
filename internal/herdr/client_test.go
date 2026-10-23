package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCallFraming(t *testing.T) {
	for _, tt := range []struct {
		name, reply string
		bad         bool
	}{
		{"large", `{"id":"m0-1","result":{"text":"` + strings.Repeat("x", 70000) + `"}}` + "\n", false},
		{"wrong id", `{"id":"wrong","result":{}}` + "\n", true},
		{"remote error", `{"id":"m0-1","error":{"code":"busy","message":"occupied"}}` + "\n", true},
		{"truncated", `{"id":"m0-1","result":{}}`, true},
		{"over limit", strings.Repeat("x", MaxLineBytes+1) + "\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "s")
			l, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			go func() {
				c, e := l.Accept()
				if e != nil {
					return
				}
				defer c.Close()
				bufio.NewReader(c).ReadBytes('\n')
				c.Write([]byte(tt.reply))
			}()
			var out json.RawMessage
			err = Call(context.Background(), Target{Socket: path}, "ping", map[string]any{}, &out)
			if (err != nil) != tt.bad {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestCallCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		<-done
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	var out any
	if err := Call(ctx, Target{Socket: path}, "ping", nil, &out); err == nil {
		t.Fatal("missing cancellation")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation did not interrupt read")
	}
}
