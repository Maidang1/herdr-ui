// Package session owns event-driven authoritative state. No UI or terminal
// plugin is imported here. One coordinator serializes all snapshot requests.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/herdr-ui/internal/herdr"
)

type State string

const (
	Disconnected  State = "Disconnected"
	Connecting    State = "Connecting"
	Subscribing   State = "Subscribing"
	Bootstrapping State = "Bootstrapping"
	Ready         State = "Ready"
	Resyncing     State = "Resyncing"
	Reconnecting  State = "Reconnecting"
	Incompatible  State = "Incompatible"
	Closed        State = "Closed"
)

type PublishedSnapshot struct {
	TargetID uint64
	Epoch    uint64
	Revision uint64
	State    State
	Stale    bool
	Error    string
	Updated  time.Time
	Snapshot *Snapshot
}

var targets atomic.Uint64

type Session struct {
	target                       herdr.Target
	targetID                     uint64
	mu                           sync.RWMutex
	latest                       PublishedSnapshot
	started, closed              bool
	cancel                       context.CancelFunc
	notify                       func()
	done                         chan struct{}
	refresh                      chan struct{}
	wake                         chan struct{}
	unknown                      sync.Map
	operationMu                  sync.Mutex
	nextOperation                uint64
	pendingOperations            map[operationKey]*queuedOperation
	reservedOperations           map[string]bool
	operationOrder               []operationKey
	operationWake                chan struct{}
	visibleMu                    sync.Mutex
	visiblePane, visibleTerminal string
}

func New(target herdr.Target) *Session {
	id := targets.Add(1)
	return &Session{target: target, targetID: id, latest: PublishedSnapshot{TargetID: id, State: Disconnected, Stale: true}, done: make(chan struct{}), refresh: make(chan struct{}, 1), wake: make(chan struct{}, 1), pendingOperations: make(map[operationKey]*queuedOperation), operationWake: make(chan struct{}, 1)}
}
func (s *Session) Start(ctx context.Context, notify func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.closed {
		return
	}
	s.started = true
	s.notify = notify
	ctx, s.cancel = context.WithCancel(ctx)
	go s.run(ctx)
}
func (s *Session) Latest() PublishedSnapshot { s.mu.RLock(); defer s.mu.RUnlock(); return s.latest }
func (s *Session) Refresh() {
	select {
	case s.refresh <- struct{}{}:
	default:
	}
}
func (s *Session) Close(ctx context.Context) error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		if s.started {
			s.cancel()
		} else {
			s.latest.State = Closed
			close(s.done)
		}
	}
	s.mu.Unlock()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Session) publish(epoch uint64, state State, snapshot *Snapshot, err error) {
	s.mu.Lock()
	p := s.latest
	p.Epoch = epoch
	p.State = state
	p.Stale = state != Ready
	p.Error = ""
	if err != nil {
		p.Error = err.Error()
	}
	if snapshot != nil {
		p.Snapshot = snapshot
		p.Revision++
		p.Updated = time.Now()
	}
	s.latest = p
	notify := s.notify
	s.mu.Unlock()
	if notify != nil {
		notify()
	}
}

func (s *Session) run(ctx context.Context) {
	var epoch uint64
	defer func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.finishPending(OperationClosed, "session closed")
		s.publish(epoch, Closed, nil, nil)
		close(s.done)
	}()
	delay := 250 * time.Millisecond
	for ctx.Err() == nil {
		epoch++
		s.publish(epoch, Connecting, nil, nil)
		err, wasReady := s.connected(ctx, epoch)
		s.finishPending(StaleTarget, "connection epoch ended")
		if ctx.Err() != nil {
			return
		}
		var incompatible *compatibilityError
		if errors.As(err, &incompatible) {
			s.publish(epoch, Incompatible, nil, err)
			select {
			case <-ctx.Done():
				return
			case <-s.refresh:
			}
			continue
		}
		s.publish(epoch, Reconnecting, nil, err)
		if wasReady {
			delay = 250 * time.Millisecond
		}
		timer := time.NewTimer(min(delay+time.Duration(rand.Int64N(int64(delay/4)+1)), 5*time.Second))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.refresh:
			timer.Stop()
		case <-timer.C:
		}
		delay = min(delay*2, 5*time.Second)
	}
}

type compatibilityError struct{ error }
type readFailure struct{ err error }
type watcher struct {
	stream *herdr.Stream
	dirty  atomic.Bool
	failed atomic.Pointer[readFailure]
	done   chan struct{}
}

func (s *Session) watch(ctx context.Context, subscriptions []herdr.Subscription) (*watcher, error) {
	stream, err := herdr.Subscribe(ctx, s.target, subscriptions)
	if err != nil {
		return nil, err
	}
	w := &watcher{stream: stream, done: make(chan struct{})}
	go func() {
		defer close(w.done)
		for {
			event, err := stream.Next()
			if err != nil {
				w.failed.Store(&readFailure{err})
				signal(s.wake)
				return
			}
			if !knownEvents[event.Name] {
				if _, seen := s.unknown.LoadOrStore(event.Name, true); !seen {
					log.Printf("session target=%d unknown event=%.128s; refreshing", s.targetID, event.Name)
				}
			}
			w.dirty.Store(true)
			signal(s.wake)
		}
	}()
	return w, nil
}
func (w *watcher) close() {
	if w != nil {
		w.stream.Close()
		<-w.done
	}
}
func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

var lifecycleTypes = []string{
	"workspace.created", "workspace.updated", "workspace.metadata_updated", "workspace.renamed", "workspace.moved", "workspace.reordered", "workspace.closed", "workspace.focused",
	"worktree.created", "worktree.opened", "worktree.removed",
	"tab.created", "tab.closed", "tab.focused", "tab.renamed", "tab.moved",
	"pane.created", "pane.closed", "pane.updated", "pane.focused", "pane.moved", "pane.exited", "pane.agent_detected", "layout.updated",
}

var knownEvents = func() map[string]bool {
	names := map[string]bool{"pane.agent_status_changed": true, "pane.scroll_changed": true}
	for _, name := range lifecycleTypes {
		names[name] = true
		names[strings.ReplaceAll(name, ".", "_")] = true
	}
	return names
}()

func (s *Session) connected(ctx context.Context, epoch uint64) (error, bool) {
	client, err := herdr.Dial(ctx, s.target)
	if err != nil {
		return err, false
	}
	defer client.Close()
	// Parent cancellation must interrupt a request even if no caller is waiting.
	stop := context.AfterFunc(ctx, func() { client.Close() })
	defer stop()
	var ping struct {
		Type string `json:"type"`
	}
	if err := client.Call(ctx, "ping", nil, &ping); err != nil {
		return classify(err), false
	}
	if ping.Type != "pong" {
		return &compatibilityError{fmt.Errorf("ping did not return pong")}, false
	}
	s.publish(epoch, Subscribing, nil, nil)
	subscriptions := make([]herdr.Subscription, len(lifecycleTypes))
	for i, name := range lifecycleTypes {
		subscriptions[i].Type = name
	}
	lifecycle, err := s.watch(ctx, subscriptions)
	if err != nil {
		return classify(err), false
	}
	defer lifecycle.close()
	var agents *watcher
	defer func() { agents.close() }()
	var scroll *watcher
	defer func() { scroll.close() }()
	scrollPane, scrollTerminal := "", ""
	var agentIDs []string
	bound := false
	needSnapshot := true
	first := true
	wasReady := false
	lastWasOperation := false
	referenceRetries := 0
	failedBindings := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err(), wasReady
		}
		for _, w := range []*watcher{lifecycle, agents, scroll} {
			if w != nil {
				if failure := w.failed.Load(); failure != nil {
					return classify(failure.err), wasReady
				}
			}
		}
		select {
		case <-s.refresh:
			needSnapshot = true
		default:
		}
		for _, w := range []*watcher{lifecycle, agents, scroll} {
			if w != nil && w.dirty.Load() {
				needSnapshot = true
			}
		}
		if wasReady && s.Latest().State == Ready && (!needSnapshot || !lastWasOperation) {
			if q := s.takeOperation(); q != nil {
				if err := s.execute(ctx, client, q); err != nil {
					return classify(err), wasReady
				}
				needSnapshot = true
				lastWasOperation = true
				continue
			}
		}
		if !needSnapshot {
			select {
			case <-ctx.Done():
				return ctx.Err(), wasReady
			case <-s.refresh:
				needSnapshot = true
			case <-s.wake:
			case <-s.operationWake:
			}
			continue
		}
		// Only clear signals observed before the request. Readers can set dirty
		// again while Call is in flight, and those signals survive installation.
		for _, w := range []*watcher{lifecycle, agents, scroll} {
			if w != nil {
				w.dirty.Swap(false)
			}
		}
		needSnapshot = false
		if !wasReady {
			s.publish(epoch, Bootstrapping, nil, nil)
		} else {
			s.publish(epoch, Resyncing, nil, nil)
		}
		var result json.RawMessage
		if err := client.Call(ctx, "session.snapshot", nil, &result); err != nil {
			return classify(err), wasReady
		}
		snapshot, err := DecodeSnapshot(result)
		if err != nil {
			var reference *ReferenceError
			if errors.As(err, &reference) && referenceRetries == 0 {
				referenceRetries++
				needSnapshot = true
				continue
			}
			return &compatibilityError{err}, wasReady
		}
		referenceRetries = 0
		for _, w := range []*watcher{lifecycle, agents, scroll} {
			if w != nil {
				if failure := w.failed.Load(); failure != nil {
					return classify(failure.err), wasReady
				}
			}
		}
		ids := snapshot.PaneIDs()
		slices.Sort(ids)
		if !bound || !slices.Equal(ids, agentIDs) {
			nextSubscriptions := make([]herdr.Subscription, len(ids))
			for i, id := range ids {
				nextSubscriptions[i] = herdr.Subscription{Type: "pane.agent_status_changed", PaneID: id}
			}
			var next *watcher
			if len(ids) > 0 {
				next, err = s.watch(ctx, nextSubscriptions)
				if err != nil {
					var remote *herdr.RemoteError
					if errors.As(err, &remote) && (remote.Code == "pane_not_found" || remote.Code == "not_found" || remote.Code == "invalid_target") {
						failedBindings++
						if failedBindings > 8 {
							return fmt.Errorf("pane set kept changing while subscribing: %w", err), wasReady
						}
						needSnapshot = true
						continue
					}
					return classify(err), wasReady
				}
			}
			old := agents
			agents = next
			agentIDs = ids
			bound = true
			failedBindings = 0
			old.close()
			// Install complete but stale data while the confirmed new subscription's
			// second authoritative snapshot closes the handshake race.
			s.publish(epoch, Bootstrapping, snapshot, nil)
			needSnapshot = true
			first = false
			continue
		}
		if first {
			first = false
			needSnapshot = true
			continue
		}
		visiblePane, visibleTerminal := s.visibleScroll()
		if pane, ok := snapshot.Pane(visiblePane); !ok || pane.TerminalID != visibleTerminal {
			visiblePane, visibleTerminal = "", ""
		}
		if visiblePane != scrollPane || visibleTerminal != scrollTerminal {
			var next *watcher
			if visiblePane != "" {
				next, err = s.watch(ctx, []herdr.Subscription{{Type: "pane.scroll_changed", PaneID: visiblePane}})
				if err != nil {
					var remote *herdr.RemoteError
					if errors.As(err, &remote) && remote.Code == "pane_not_found" {
						needSnapshot = true
						continue
					}
					return classify(err), wasReady
				}
			}
			old := scroll
			scroll = next
			scrollPane, scrollTerminal = visiblePane, visibleTerminal
			old.close()
			needSnapshot = true
			s.publish(epoch, Resyncing, snapshot, nil)
			continue
		}
		wasReady = true
		lastWasOperation = false
		s.publish(epoch, Ready, snapshot, nil)
	}
}

func classify(err error) error {
	var protocol *herdr.ProtocolError
	var remote *herdr.RemoteError
	if errors.As(err, &protocol) || errors.As(err, &remote) {
		return &compatibilityError{err}
	}
	if errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("Herdr connection closed: %w", err)
	}
	return err
}
