package sidebarinfo

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/herdr"
)

const processFixture = "101 1 Fri Oct 23 12:00:00 2026\n201 101 Fri Oct 23 12:01:00 2026\n202 201 Fri Oct 23 12:02:00 2026\n102 1 Fri Oct 23 13:00:00 2026\n302 102 Fri Oct 23 13:01:00 2026\n999 1 Fri Oct 23 14:00:00 2026\n"
const listenerFixture = "p201\nn127.0.0.1:3000\nn[::1]:3000\np202\nn*:8000\np302\nn*:4000\np999\nn*:9999\n"

func TestProcessParsingAndPortAssociationIncludeBackgroundDescendantsOnly(t *testing.T) {
	calls := 0
	run := func(_ context.Context, name string, args ...string) (commandOutput, error) {
		if name == "ps" {
			return commandOutput{data: []byte(processFixture)}, nil
		}
		if len(args) > 0 && args[0] == "-a" {
			return commandOutput{}, nil
		}
		return commandOutput{data: []byte(listenerFixture)}, nil
	}
	info := func(_ context.Context, _ herdr.Target, pane string) (int, error) {
		calls++
		if pane == "a" {
			return 101, nil
		}
		return 102, nil
	}
	c := &Collector{run: run, processInfo: info, roots: map[string]shellRoot{}}
	source := Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "first", Panes: []PaneSource{{"a", "ta"}}}, {ID: "second", Panes: []PaneSource{{"b", "tb"}}}}}
	ports, known := c.collectPorts(context.Background(), source)
	if !known["first"] || !known["second"] || !reflect.DeepEqual(ports["first"], []int{3000, 8000}) || !reflect.DeepEqual(ports["second"], []int{4000}) {
		t.Fatal(ports, known)
	}
	c.collectPorts(context.Background(), source)
	if calls != 2 {
		t.Fatal("unchanged shell ownership was queried again", calls)
	}
}
func TestReusedShellPIDCannotClaimUnrelatedListeners(t *testing.T) {
	processes, _ := parseProcesses([]byte(processFixture))
	pane := PaneSource{"a", "ta"}
	key := rootKey(1, pane)
	c := &Collector{processInfo: func(context.Context, herdr.Target, string) (int, error) { return 101, nil }, roots: map[string]shellRoot{key: {pid: 101, parent: 1, birth: "earlier birth", known: true, expires: time.Now().Add(time.Hour)}}}
	if pid, known := c.resolveRoot(context.Background(), 1, pane, processes); known || pid != 0 {
		t.Fatal("PID reuse accepted", pid, known)
	}
	if pid, known := c.resolveRoot(context.Background(), 1, pane, processes); known || pid != 0 {
		t.Fatal("PID reuse accepted after cache failure", pid, known)
	}
}
func TestPartialListenerFailureIsUnknownButNoListenersIsKnownEmpty(t *testing.T) {
	for _, diagnostic := range []bool{false, true} {
		c := &Collector{roots: map[string]shellRoot{}, processInfo: func(context.Context, herdr.Target, string) (int, error) { return 101, nil }, run: func(_ context.Context, name string, _ ...string) (commandOutput, error) {
			if name == "ps" {
				return commandOutput{data: []byte(processFixture)}, nil
			}
			if diagnostic {
				return commandOutput{code: 0, diagnostic: true}, nil
			}
			return commandOutput{code: 1}, errors.New("lsof exit1")
		}}
		_, known := c.collectPorts(context.Background(), Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "w", Panes: []PaneSource{{"a", "t"}}}}})
		if known["w"] == diagnostic {
			t.Fatal("partial failure/no-listeners not distinguished", diagnostic, known)
		}
	}
}
func TestChangingProcessAncestryIsRejected(t *testing.T) {
	before, _ := parseProcesses([]byte(processFixture))
	after, _ := parseProcesses([]byte(processFixture))
	p := after[201]
	p.birth = "reused child"
	after[201] = p
	if stableAncestry(202, 101, before, after) {
		t.Fatal("reused intermediary accepted")
	}
	before[101] = process{pid: 101, parent: 202, birth: "cycle"}
	if descendsFrom(202, 555, before) {
		t.Fatal("cyclic parent chain matched absent root")
	}
	for _, data := range []string{"invalid row", "1 x Fri Oct 23 12:00:00 2026"} {
		if _, err := parseProcesses([]byte(data)); err == nil {
			t.Fatal("bad ps data accepted")
		}
	}
	for _, data := range []string{"n*:3000\n", "p3\nn*:bad\n", "p-1\n"} {
		if _, err := parseListeners([]byte(data)); err == nil {
			t.Fatal("bad socket data accepted")
		}
	}
}

func TestPartialListenerDiagnosticRetainsConfirmedOwnedPorts(t *testing.T) {
	c := &Collector{roots: map[string]shellRoot{}, processInfo: func(context.Context, herdr.Target, string) (int, error) { return 101, nil }, run: func(_ context.Context, name string, args ...string) (commandOutput, error) {
		if name == "ps" {
			return commandOutput{data: []byte(processFixture)}, nil
		}
		if args[0] == "-a" {
			return commandOutput{}, nil
		}
		return commandOutput{data: []byte(listenerFixture), diagnostic: true}, nil
	}}
	ports, known := c.collectPorts(context.Background(), Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "w", Panes: []PaneSource{{"a", "t"}}}}})
	if known["w"] || !reflect.DeepEqual(ports["w"], []int{3000, 8000}) {
		t.Fatal("partial failure discarded proof or claimed complete", ports, known)
	}
}
