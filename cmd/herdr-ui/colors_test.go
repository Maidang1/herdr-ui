package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchHerdrColorsAtomicSaveDeleteAndCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[theme]"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchHerdrColors(ctx, path, 5*time.Millisecond, func() { changes <- struct{}{} })
	}()
	next := func() {
		t.Helper()
		select {
		case <-changes:
		case <-time.After(2 * time.Second):
			t.Fatal("configuration change was not detected")
		}
	}
	next()
	replacement := path + ".tmp"
	if err := os.WriteFile(replacement, []byte("[theme]\nname = \"gruvbox\""), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	next()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	next()
	if err := os.WriteFile(path, []byte("[theme]\nname = \"nord\""), 0600); err != nil {
		t.Fatal(err)
	}
	next()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}
}
