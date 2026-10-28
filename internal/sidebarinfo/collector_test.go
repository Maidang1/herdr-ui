package sidebarinfo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
)

func TestLatestSourceCancelsOldCollectionAndOwnsInput(t *testing.T) {
	oldPath, newPath, mutatedPath := t.TempDir(), t.TempDir(), t.TempDir()
	entered := make(chan struct{})
	canceled := make(chan struct{})
	var once sync.Once
	notifications := make(chan Publication, 4)
	run := func(ctx context.Context, name string, args ...string) (commandOutput, error) {
		switch name {
		case "ps":
			return commandOutput{data: []byte(processFixture)}, nil
		case "lsof":
			return commandOutput{}, nil
		case "git":
			if args[1] == oldPath {
				once.Do(func() { close(entered) })
				<-ctx.Done()
				select {
				case <-canceled:
				default:
					close(canceled)
				}
				return commandOutput{}, ctx.Err()
			}
			if args[2] == "symbolic-ref" {
				return commandOutput{data: []byte("feature\n")}, nil
			}
			return commandOutput{}, errors.New("no upstream/origin")
		}
		panic("unexpected command")
	}
	c := newCollector(herdr.Target{}, func(p Publication) { publishForTest(t, notifications, p) }, run, func(context.Context, herdr.Target, string) (int, error) { return 0, nil })
	defer closeCollectorForTest(t, c)
	original := Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "w", Path: oldPath}}}
	c.updateSource(original)
	awaitCollectorSignal(t, entered, "collection did not start")
	c.updateSource(original)
	c.mu.Lock()
	generation := c.generation
	c.mu.Unlock()
	if generation != 1 {
		t.Fatal("identical source scheduled new generation")
	}
	original.Workspaces[0].Path = mutatedPath
	c.mu.Lock()
	path := c.source.Workspaces[0].Path
	c.mu.Unlock()
	if path != oldPath {
		t.Fatal("caller mutated collector source")
	}
	c.updateSource(Source{TargetID: 2, Workspaces: []WorkspaceSource{{ID: "w", Path: newPath}}})
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("old command not canceled")
	}
	select {
	case p := <-notifications:
		if p.TargetID != 2 || p.Entries["w"].Info.Branch != "feature" {
			t.Fatal("old source published", p)
		}
	case <-time.After(time.Second):
		t.Fatal("latest source not published")
	}
}
func TestCloseCancelsCommandsAndPreventsLateNotification(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	published := make(chan Publication, 2)
	c := newCollector(herdr.Target{}, func(p Publication) { publishForTest(t, published, p) }, func(ctx context.Context, _ string, _ ...string) (commandOutput, error) {
		once.Do(func() { close(entered) })
		<-ctx.Done()
		return commandOutput{}, ctx.Err()
	}, func(context.Context, herdr.Target, string) (int, error) { return 0, nil })
	defer closeCollectorForTest(t, c)
	c.updateSource(Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "w", Path: t.TempDir()}}})
	awaitCollectorSignal(t, entered, "collection did not start")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Close(ctx); err != nil {
		t.Fatal(err)
	}
	c.updateSource(Source{TargetID: 2})
	select {
	case p := <-published:
		t.Fatal("closed collector published", p)
	default:
	}
}

func TestEmptyDesiredStateClearsRootsWithoutLocalCommands(t *testing.T) {
	c := &Collector{roots: map[string]shellRoot{"old": {pid: 1}}, run: func(context.Context, string, ...string) (commandOutput, error) {
		t.Fatal("empty service executed local command")
		return commandOutput{}, nil
	}}
	p := c.collect(context.Background(), Source{TargetID: 2})
	if p.TargetID != 2 || len(p.Entries) != 0 || len(c.roots) != 0 {
		t.Fatal("old state retained", p, c.roots)
	}
}

// Every coordination wait fails locally, rather than leaving a broken fixture
// blocked until the package-wide timeout. Notification callbacks must also be
// nonblocking so Close can finish even when an assertion fails before draining.
func awaitCollectorSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal(message)
	}
}

func publishForTest(t *testing.T, publications chan<- Publication, p Publication) {
	t.Helper()
	select {
	case publications <- p:
	default:
		t.Error("unexpected publication overflow")
	}
}

func closeCollectorForTest(t *testing.T, c *Collector) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Close(ctx); err != nil {
		t.Errorf("collector cleanup: %v", err)
	}
}
