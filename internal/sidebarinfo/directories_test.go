package sidebarinfo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/herdr"
)

func TestExternalListenerCWDMatchesUniqueCanonicalWorkspace(t *testing.T) {
	project := t.TempDir()
	child := filepath.Join(project, "packages", "service")
	if err := os.MkdirAll(child, 0700); err != nil {
		t.Fatal(err)
	}
	source := Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "repo", Path: project}}}
	c := &Collector{roots: map[string]shellRoot{}, processInfo: func(context.Context, herdr.Target, string) (int, error) { return 0, nil }, run: func(_ context.Context, name string, args ...string) (commandOutput, error) {
		if name == "ps" {
			return commandOutput{data: []byte(processFixture)}, nil
		}
		if args[0] == "-a" {
			return commandOutput{data: []byte("p999\nn" + child + "\n")}, nil
		}
		return commandOutput{data: []byte("p999\nn*:9900\n")}, nil
	}}
	ports, known := c.collectPorts(context.Background(), source, map[string]string{"repo": project})
	if !known["repo"] || !reflect.DeepEqual(ports["repo"], []int{9900}) {
		t.Fatal("actual external CWD not associated", ports, known)
	}
	// An arbitrary non-Git shell folder does not claim nested projects.
	ports, _ = c.collectPorts(context.Background(), source)
	if len(ports["repo"]) != 0 {
		t.Fatal("non-Git path claimed descendants", ports)
	}
}
func TestDirectoryBoundaryAmbiguityAndSymlinks(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "repo")
	sibling := filepath.Join(base, "repo-other")
	nested := filepath.Join(project, "nested")
	for _, path := range []string{project, sibling, nested} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	source := Source{Workspaces: []WorkspaceSource{{ID: "root", Path: project}, {ID: "nested", Path: nested}}}
	candidates := workspaceDirectories(source, map[string]string{"root": project, "nested": nested})
	if got := workspaceForDirectory(sibling, candidates); got != "" {
		t.Fatal("sibling prefix falsely matched", got)
	}
	if got := workspaceForDirectory(nested, candidates); got != "nested" {
		t.Fatal("longest unique checkout not selected", got)
	}
	canonicalNested, _ := canonicalDirectory(nested)
	candidates = append(candidates, workspaceDirectory{id: "duplicate", path: canonicalNested, descendants: true})
	if got := workspaceForDirectory(nested, candidates); got != "" {
		t.Fatal("same-path workspaces ambiguously assigned", got)
	}
	alias := filepath.Join(base, "symlink")
	if err := os.Symlink(project, alias); err != nil {
		t.Fatal(err)
	}
	candidates = workspaceDirectories(Source{Workspaces: []WorkspaceSource{{ID: "root", Path: alias}}}, map[string]string{"root": alias})
	if got := workspaceForDirectory(nested, candidates); got != "root" {
		t.Fatal("canonical symlink descendant lost", got)
	}
	escape := filepath.Join(project, "escape")
	if err := os.Symlink(sibling, escape); err != nil {
		t.Fatal(err)
	}
	if got := workspaceForDirectory(escape, candidates); got != "" {
		t.Fatal("symlink escaped checkout boundary", got)
	}
}
func TestExternalCWDRequiresStableProcessIdentity(t *testing.T) {
	path := t.TempDir()
	calls := 0
	c := &Collector{roots: map[string]shellRoot{}, run: func(_ context.Context, name string, args ...string) (commandOutput, error) {
		if name == "ps" {
			calls++
			data := processFixture
			if calls == 2 {
				data = strings.Replace(data, "999 1 Fri Oct 23 14:00:00 2026", "999 1 Fri Oct 23 14:00:01 2026", 1)
			}
			return commandOutput{data: []byte(data)}, nil
		}
		if args[0] == "-a" {
			return commandOutput{data: []byte("p999\nn" + path + "\n")}, nil
		}
		return commandOutput{data: []byte("p999\nn*:9900\n")}, nil
	}}
	ports, _ := c.collectPorts(context.Background(), Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "w", Path: path}}})
	if len(ports["w"]) != 0 {
		t.Fatal("reused external listener PID accepted", ports)
	}
}
func TestExternalCWDRejectsProcessAbsentFromEitherSnapshot(t *testing.T) {
	for _, missing := range []string{"both", "before", "after"} {
		t.Run(missing, func(t *testing.T) {
			path := t.TempDir()
			calls := 0
			c := &Collector{roots: map[string]shellRoot{}, run: func(_ context.Context, name string, args ...string) (commandOutput, error) {
				if name == "ps" {
					calls++
					data := processFixture
					if missing == "both" || (missing == "before" && calls == 1) || (missing == "after" && calls == 2) {
						data = strings.Replace(data, "999 1 Fri Oct 23 14:00:00 2026\n", "", 1)
					}
					return commandOutput{data: []byte(data)}, nil
				}
				if args[0] == "-a" {
					return commandOutput{data: []byte("p999\nn" + path + "\n")}, nil
				}
				return commandOutput{data: []byte("p999\nn*:9900\n")}, nil
			}}
			ports, _ := c.collectPorts(context.Background(), Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "w", Path: path}}})
			if len(ports["w"]) != 0 {
				t.Fatal("listener has no stable process identity", ports)
			}
		})
	}
}
