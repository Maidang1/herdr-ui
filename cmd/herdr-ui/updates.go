package main

import (
	"context"
	"log"
	"sync"
	"time"

	view "github.com/Maidang1/herdr-ui/internal/ui"
)

const (
	versionCheckFirst = 8 * time.Second
	versionCheckEvery = 24 * time.Hour
	versionCheckRetry = time.Hour
)

// versionWatch remembers the newer version a background check found.
type versionWatch struct {
	mu        sync.Mutex
	available string
	kick      chan struct{}
}

func newVersionWatch() *versionWatch {
	return &versionWatch{kick: make(chan struct{}, 1)}
}

func (v *versionWatch) notice() view.UpdateNotice {
	v.mu.Lock()
	defer v.mu.Unlock()
	return view.UpdateNotice{Available: v.available}
}

// poke asks for a check without waiting out the daily interval.
func (v *versionWatch) poke() {
	select {
	case v.kick <- struct{}{}:
	default:
	}
}

func (v *versionWatch) set(available string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.available == available {
		return false
	}
	v.available = available
	return true
}

// run checks while allowed reports that automatic checks are on. The first
// check waits so launch stays quiet. A failure retries sooner than a day.
func (v *versionWatch) run(ctx context.Context, allowed func() bool, check func(context.Context) (string, error), refresh func()) {
	timer := time.NewTimer(versionCheckFirst)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-v.kick:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
		delay := versionCheckEvery
		if allowed() {
			available, err := check(ctx)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				log.Printf("update check: %v", err)
				delay = versionCheckRetry
			} else if v.set(available) {
				refresh()
			}
		}
		timer.Reset(delay)
	}
}
