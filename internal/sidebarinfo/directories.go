package sidebarinfo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type workspaceDirectory struct {
	id, path    string
	descendants bool
}

func canonicalDirectory(path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return filepath.Clean(canonical), true
}
func workspaceDirectories(source Source, repoRoots map[string]string) []workspaceDirectory {
	var directories []workspaceDirectory
	home, _ := os.UserHomeDir()
	home, _ = canonicalDirectory(home)
	for _, workspace := range source.Workspaces {
		path := workspace.Path
		descendants := false
		if repoRoots[workspace.ID] != "" {
			path = repoRoots[workspace.ID]
			descendants = true
		}
		canonical, ok := canonicalDirectory(path)
		if !ok {
			continue
		}
		// Generic shell workspaces do not claim every server below HOME or /tmp.
		if canonical == home || filepath.Dir(canonical) == canonical {
			descendants = false
		}
		directories = append(directories, workspaceDirectory{id: workspace.ID, path: canonical, descendants: descendants})
	}
	return directories
}
func workspaceForDirectory(path string, candidates []workspaceDirectory) string {
	canonical, ok := canonicalDirectory(path)
	if !ok {
		return ""
	}
	best, depth, count := "", -1, 0
	for _, candidate := range candidates {
		matches := canonical == candidate.path
		if !matches && candidate.descendants {
			relative, err := filepath.Rel(candidate.path, canonical)
			matches = err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
		}
		if !matches {
			continue
		}
		candidateDepth := len(strings.Split(candidate.path, string(filepath.Separator)))
		if candidateDepth > depth {
			best, depth, count = candidate.id, candidateDepth, 1
		} else if candidateDepth == depth {
			count++
		}
	}
	if count != 1 {
		return ""
	}
	return best
}
func parseWorkingDirectories(data []byte) (map[int]string, error) {
	paths := map[int]string{}
	pid := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			value, err := strconv.Atoi(line[1:])
			if err != nil || value < 1 {
				return nil, fmt.Errorf("invalid CWD process")
			}
			pid = value
		case 'n':
			if pid == 0 || !filepath.IsAbs(line[1:]) {
				return nil, fmt.Errorf("invalid process working directory")
			}
			if previous, exists := paths[pid]; exists && previous != line[1:] {
				return nil, fmt.Errorf("ambiguous process working directory")
			}
			paths[pid] = line[1:]
		}
	}
	return paths, nil
}
func (c *Collector) listenerWorkingDirectories(ctx context.Context, listeners map[int][]int) (map[int]string, bool) {
	if len(listeners) == 0 {
		return map[int]string{}, true
	}
	if len(listeners) > 4096 {
		return nil, false
	}
	pids := make([]int, 0, len(listeners))
	for pid := range listeners {
		pids = append(pids, pid)
	}
	slices.Sort(pids)
	ids := make([]string, len(pids))
	for i, pid := range pids {
		ids[i] = strconv.Itoa(pid)
	}
	result, err := c.run(ctx, "lsof", "-a", "-p", strings.Join(ids, ","), "-d", "cwd", "-Fpn")
	complete := err == nil && !result.diagnostic
	if !complete && len(result.data) == 0 {
		return nil, false
	}
	paths, parseErr := parseWorkingDirectories(result.data)
	return paths, complete && parseErr == nil
}
