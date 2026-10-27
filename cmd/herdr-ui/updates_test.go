package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestVersionWatchReportsANewerVersionAndClearsIt(t *testing.T) {
	watch := newVersionWatch()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	found := make(chan string, 2)
	versions := []string{"0.0.8", ""}
	go watch.run(ctx, func() bool { return true }, func(context.Context) (string, error) {
		next := versions[0]
		versions = versions[1:]
		return next, nil
	}, func() { found <- watch.notice().Available })
	watch.poke()

	wait := time.NewTimer(2 * time.Second)
	defer wait.Stop()
	select {
	case got := <-found:
		if got != "0.0.8" {
			t.Fatalf("notice %q", got)
		}
	case <-wait.C:
		t.Fatal("background check did not report")
	}
	watch.poke()
	select {
	case got := <-found:
		if got != "" {
			t.Fatalf("cleared notice %q", got)
		}
	case <-wait.C:
		t.Fatal("a later check did not clear the notice")
	}
}

func TestVersionWatchSkipsWhileAutomaticChecksAreOff(t *testing.T) {
	watch := newVersionWatch()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checked := make(chan struct{}, 1)
	asked := make(chan struct{}, 1)
	go watch.run(ctx, func() bool {
		select {
		case asked <- struct{}{}:
		default:
		}
		return false
	}, func(context.Context) (string, error) {
		checked <- struct{}{}
		return "9", errors.New("should not check")
	}, func() {})
	watch.poke()
	select {
	case <-asked:
	case <-time.After(2 * time.Second):
		t.Fatal("the watch never consulted automatic checks")
	}
	select {
	case <-checked:
		t.Fatal("checked while automatic checks were off")
	default:
	}
}
