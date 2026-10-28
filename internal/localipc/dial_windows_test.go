package localipc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestNamedPipeRoundTripAndDeadline(t *testing.T) {
	path := fmt.Sprintf("herdr-ui-test-%d-%d.sock", os.Getpid(), time.Now().UnixNano())
	listener, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if listener.Addr().String() != path {
		t.Fatalf("logical address = %q", listener.Addr())
	}
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := DialContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var peer net.Conn
	select {
	case peer = <-accepted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	defer peer.Close()
	go func() { _, _ = peer.Write([]byte("hello\n")) }()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 6)
	if _, err := io.ReadFull(conn, b); err != nil || string(b) != "hello\n" {
		t.Fatalf("read = %q, %v", b, err)
	}
	conn.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	if _, err := conn.Read(b); err == nil {
		t.Fatal("idle pipe ignored read deadline")
	} else {
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("read deadline = %v", err)
		}
	}
}

func TestNamedPipeDialCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DialContext(ctx, fmt.Sprintf("herdr-ui-missing-%d.sock", os.Getpid()))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled pipe dial = %v", err)
	}
}
