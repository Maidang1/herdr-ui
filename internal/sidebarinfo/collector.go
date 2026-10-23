package sidebarinfo

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
)

const refreshInterval = 5 * time.Second

type Collector struct {
	target      herdr.Target
	notify      func(Publication)
	run         commandFunc
	processInfo processCall
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	wake        chan struct{}
	mu          sync.Mutex
	notifyMu    sync.Mutex
	source      Source
	generation  uint64
	closed      bool
	jobCancel   context.CancelFunc
	// Only the serial background worker accesses root ownership cache.
	roots map[string]shellRoot
}

func New(target herdr.Target, notify func(Publication)) *Collector {
	return newCollector(target, notify, runCommand, readShellPID)
}
func newCollector(target herdr.Target, notify func(Publication), run commandFunc, info processCall) *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Collector{target: target, notify: notify, run: run, processInfo: info, ctx: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1), roots: map[string]shellRoot{}}
	go c.loop()
	return c
}

func (c *Collector) Update(p session.PublishedSnapshot) {
	if p.Snapshot == nil || p.Stale || p.State != session.Ready {
		return
	}
	c.updateSource(BuildSource(p))
}
func (c *Collector) updateSource(source Source) {
	source = cloneSource(source)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || reflect.DeepEqual(source, c.source) {
		return
	}
	c.source = source
	c.generation++
	if c.jobCancel != nil {
		c.jobCancel()
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
func (c *Collector) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.cancel()
	if c.jobCancel != nil {
		c.jobCancel()
	}
	c.mu.Unlock()
	select {
	case <-c.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	// A notification already entering user code may finish; none starts after
	// Close returns. UI assembly additionally checks its closed flag on delivery.
	c.notifyMu.Lock()
	c.notifyMu.Unlock()
	return nil
}
func (c *Collector) loop() {
	defer close(c.done)
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.wake:
		case <-ticker.C:
		}
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		source, generation := cloneSource(c.source), c.generation
		job, cancel := context.WithTimeout(c.ctx, 8*time.Second)
		c.jobCancel = cancel
		c.mu.Unlock()
		if source.TargetID == 0 {
			cancel()
			continue
		}
		publication := c.collect(job, source)
		canceled := errors.Is(job.Err(), context.Canceled)
		cancel()
		c.notifyMu.Lock()
		c.mu.Lock()
		valid := !c.closed && c.generation == generation && !canceled
		c.jobCancel = nil
		c.mu.Unlock()
		if valid && c.notify != nil {
			c.notify(clonePublication(publication))
		}
		c.notifyMu.Unlock()
	}
}

func (c *Collector) collect(ctx context.Context, source Source) Publication {
	publication := Publication{TargetID: source.TargetID, Entries: make(map[string]Entry, len(source.Workspaces))}
	if len(source.Workspaces) == 0 {
		clear(c.roots)
		return publication
	}
	infos := make([]Info, len(source.Workspaces))
	// Git commands have at most four workspace jobs, independent of pane output.
	jobs := make(chan int)
	var workers sync.WaitGroup
	for i := 0; i < min(4, len(source.Workspaces)); i++ {
		workers.Go(func() {
			for index := range jobs {
				infos[index] = collectGit(ctx, source.Workspaces[index].Path, c.run)
			}
		})
	}
	dispatched := make(chan struct{})
	go func() {
		defer close(dispatched)
		defer close(jobs)
		for i := range source.Workspaces {
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	<-dispatched
	workers.Wait()
	repoRoots := map[string]string{}
	for i, w := range source.Workspaces {
		repoRoots[w.ID] = infos[i].RepoRoot
	}
	ports, known := c.collectPorts(ctx, source, repoRoots)
	for index, item := range source.Workspaces {
		info := infos[index]
		if info.Path == "" {
			info.Path = item.Path
		}
		info.Ports = ports[item.ID]
		info.PortsKnown = known[item.ID]
		publication.Entries[item.ID] = Entry{Fingerprint: Fingerprint(item), Info: info}
	}
	return publication
}

func rootKey(target uint64, pane PaneSource) string {
	data, _ := json.Marshal([]any{target, pane.ID, pane.TerminalID})
	return string(data)
}
func (c *Collector) resolveRoot(ctx context.Context, targetID uint64, pane PaneSource, processes map[int]process) (int, bool) {
	key := rootKey(targetID, pane)
	old, exists := c.roots[key]
	now := time.Now()
	if exists && now.Before(old.expires) {
		if old.pid == 0 {
			return 0, old.known
		}
		if observed, ok := processes[old.pid]; ok && observed.parent == old.parent && observed.birth == old.birth {
			return old.pid, old.known
		}
	}
	pid, err := c.processInfo(ctx, c.target, pane.ID)
	if err != nil {
		old.known = false
		old.expires = now.Add(refreshInterval)
		c.roots[key] = old
		return 0, false
	}
	if pid == 0 {
		c.roots[key] = shellRoot{known: true, expires: now.Add(refreshInterval)}
		return 0, true
	}
	observed, ok := processes[pid]
	if !ok {
		return 0, false
	}
	if exists && old.pid == pid && old.birth != "" && (old.birth != observed.birth || old.parent != observed.parent) {
		// The runtime may still report an exited shell PID now owned by somebody
		// else. The same number is not fresh ownership evidence after PID reuse.
		old.known = false
		old.expires = now.Add(refreshInterval)
		c.roots[key] = old
		return 0, false
	}
	c.roots[key] = shellRoot{pid: pid, parent: observed.parent, birth: observed.birth, known: true, expires: now.Add(30 * time.Second)}
	return pid, true
}

func (c *Collector) collectPorts(ctx context.Context, source Source, repositoryRoots ...map[string]string) (map[string][]int, map[string]bool) {
	ports := map[string][]int{}
	known := map[string]bool{}
	active := map[string]bool{}
	for _, workspace := range source.Workspaces {
		for _, pane := range workspace.Panes {
			active[rootKey(source.TargetID, pane)] = true
		}
	}
	for key := range c.roots {
		if !active[key] {
			delete(c.roots, key)
		}
	}
	beforeResult, err := c.run(ctx, "ps", "-axo", "pid=,ppid=,lstart=")
	if err != nil || beforeResult.diagnostic {
		return ports, known
	}
	before, err := parseProcesses(beforeResult.data)
	if err != nil {
		return ports, known
	}
	roots := map[string][]int{}
	for _, workspace := range source.Workspaces {
		known[workspace.ID] = true
		for _, pane := range workspace.Panes {
			root, ok := c.resolveRoot(ctx, source.TargetID, pane, before)
			if !ok {
				known[workspace.ID] = false
			}
			if root > 0 && ok {
				roots[workspace.ID] = append(roots[workspace.ID], root)
			}
		}
	}
	listenersResult, err := c.run(ctx, "lsof", "-nP", "-a", "-iTCP", "-sTCP:LISTEN", "-Fpn")
	listenersComplete := err == nil && !listenersResult.diagnostic || (listenersResult.code == 1 && !listenersResult.diagnostic && len(listenersResult.data) == 0)
	if !listenersComplete && len(listenersResult.data) == 0 {
		return ports, map[string]bool{}
	}
	listeners, err := parseListeners(listenersResult.data)
	if err != nil {
		return ports, map[string]bool{}
	}
	cwds, cwdKnown := c.listenerWorkingDirectories(ctx, listeners)
	afterResult, err := c.run(ctx, "ps", "-axo", "pid=,ppid=,lstart=")
	if err != nil || afterResult.diagnostic {
		return ports, map[string]bool{}
	}
	after, err := parseProcesses(afterResult.data)
	if err != nil {
		return ports, map[string]bool{}
	}
	for _, workspace := range source.Workspaces {
		var workspacePorts []int
		for _, root := range roots[workspace.ID] {
			if before[root] != after[root] {
				known[workspace.ID] = false
				continue
			}
			for pid, values := range listeners {
				if !descendsFrom(pid, root, before) && !descendsFrom(pid, root, after) {
					continue
				}
				if !stableAncestry(pid, root, before, after) {
					known[workspace.ID] = false
					continue
				}
				workspacePorts = append(workspacePorts, values...)
			}
		}
		slices.Sort(workspacePorts)
		ports[workspace.ID] = slices.Compact(workspacePorts)
	}
	// Detached IDE/daemon listeners have no pane-shell ancestor. A real local
	// process CWD can still identify a workspace, without guessing from names.
	repoRoots := map[string]string{}
	if len(repositoryRoots) > 0 {
		repoRoots = repositoryRoots[0]
	}
	candidates := workspaceDirectories(source, repoRoots)
	for pid, values := range listeners {
		owned := false
		for _, workspaceRoots := range roots {
			for _, root := range workspaceRoots {
				if descendsFrom(pid, root, before) || descendsFrom(pid, root, after) {
					owned = true
				}
			}
		}
		if owned {
			continue
		}
		beforeProcess, beforeOK := before[pid]
		afterProcess, afterOK := after[pid]
		if !beforeOK || !afterOK || beforeProcess != afterProcess {
			continue
		}
		cwd, ok := cwds[pid]
		if !ok {
			continue
		}
		workspaceID := workspaceForDirectory(cwd, candidates)
		if workspaceID != "" {
			ports[workspaceID] = append(ports[workspaceID], values...)
		}
	}
	for _, workspace := range source.Workspaces {
		if !cwdKnown || !listenersComplete {
			known[workspace.ID] = false
		}
		slices.Sort(ports[workspace.ID])
		ports[workspace.ID] = slices.Compact(ports[workspace.ID])
	}
	return ports, known
}
func stableAncestry(pid, root int, before, after map[int]process) bool {
	seen := map[int]bool{}
	for pid > 0 && !seen[pid] {
		seen[pid] = true
		a, ok := before[pid]
		if !ok {
			return false
		}
		b, ok := after[pid]
		if !ok || a != b {
			return false
		}
		if pid == root {
			return true
		}
		pid = a.parent
	}
	return false
}
