package daemon

import (
	"context"
	"testing"
	"time"
)

// waitUntil must fire promptly once the target has already passed, even
// though normal steps are capped at wakeCheckInterval - this is the case a
// laptop hits on wake from sleep, past the scheduled prayer time.
func TestWaitUntilFiresImmediatelyWhenTargetAlreadyPassed(t *testing.T) {
	d := &Daemon{quit: make(chan struct{}), reload: make(chan struct{}, 1)}

	start := time.Now()
	stopped, reloaded := d.waitUntil(context.Background(), start.Add(-time.Hour))
	elapsed := time.Since(start)

	if stopped || reloaded {
		t.Fatalf("stopped=%v reloaded=%v, want false,false", stopped, reloaded)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("waitUntil took %s for an already-passed target, want near-instant", elapsed)
	}
}

func TestWaitUntilReportsReload(t *testing.T) {
	d := &Daemon{quit: make(chan struct{}), reload: make(chan struct{}, 1)}
	d.reload <- struct{}{}

	stopped, reloaded := d.waitUntil(context.Background(), time.Now().Add(time.Hour))
	if stopped || !reloaded {
		t.Fatalf("stopped=%v reloaded=%v, want false,true", stopped, reloaded)
	}
}

func TestWaitUntilReportsStop(t *testing.T) {
	d := &Daemon{quit: make(chan struct{}), reload: make(chan struct{}, 1)}
	close(d.quit)

	stopped, _ := d.waitUntil(context.Background(), time.Now().Add(time.Hour))
	if !stopped {
		t.Fatalf("stopped=false, want true")
	}
}
