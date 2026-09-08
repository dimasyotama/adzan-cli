// Package daemon runs the background scheduler: sleep until the next prayer,
// sound the adhan, repeat, while answering CLI commands over the local socket.
package daemon

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dimasyotama/adzan-cli/internal/audio"
	"github.com/dimasyotama/adzan-cli/internal/config"
	"github.com/dimasyotama/adzan-cli/internal/ipc"
	"github.com/dimasyotama/adzan-cli/internal/prayer"
)

const (
	// refreshInterval is how often the rolling window is extended. Computation
	// is local arithmetic, so this only needs to beat the window length.
	refreshInterval = 12 * time.Hour

	// retryInterval applies if a recompute somehow fails.
	retryInterval = 10 * time.Minute

	// wakeCheckInterval bounds how long any single wait for the next prayer
	// runs before re-checking the wall clock. A laptop sleeping through the
	// whole wait can't be trusted to fire a single long timer exactly on
	// wake - the OS defers/coalesces a background process's timers around a
	// sleep, which delayed the adhan by several minutes. Polling in short
	// steps instead means the worst-case lateness is one interval.
	wakeCheckInterval = 20 * time.Second
)

// waitResult reports why a wait for the daemon's channels returned.
type waitResult int

const (
	waitElapsed waitResult = iota
	waitReloaded
	waitStopped
)

// Daemon owns the schedule, the player, and the command socket.
type Daemon struct {
	mu        sync.Mutex
	cfg       *config.Config
	sched     *prayer.Schedule
	player    *audio.Player
	logger    *log.Logger
	startedAt time.Time

	reload chan struct{}
	quit   chan struct{}
	once   sync.Once
}

// New builds a daemon from the saved configuration.
func New(cfg *config.Config, logger *log.Logger) (*Daemon, error) {
	path, err := config.SchedulePath()
	if err != nil {
		return nil, err
	}
	return &Daemon{
		cfg:       cfg,
		sched:     prayer.LoadSchedule(path),
		player:    audio.New(),
		logger:    logger,
		startedAt: time.Now(),
		reload:    make(chan struct{}, 1),
		quit:      make(chan struct{}),
	}, nil
}

// Run blocks until the context is cancelled or a quit command arrives.
func (d *Daemon) Run(ctx context.Context) error {
	srv, err := ipc.Listen()
	if err != nil {
		return err
	}
	defer srv.Close()
	go srv.Serve(d.handle)

	d.logger.Printf("daemon started; location=%s method=%s",
		d.cfg.Location.Label, prayer.MethodName(d.cfg.Method))

	for {
		wait := d.refresh(ctx)

		next, err := d.nextEvent()
		if err != nil {
			d.logger.Printf("no upcoming prayer available: %v; retrying in %s", err, wait)
			if stop := d.sleep(ctx, wait); stop {
				return nil
			}
			continue
		}

		d.logger.Printf("next prayer: %s at %s (in %s)",
			next.Name, next.At.Format("15:04"), time.Until(next.At).Round(time.Second))

		stopped, reloaded := d.waitUntil(ctx, next.At)
		if stopped {
			return nil
		}
		// A reload during the wait can change the schedule, so re-derive the
		// next event rather than trusting the one computed before we waited.
		if reloaded {
			continue
		}
		d.announce(next)

		// Move past this event so the loop does not re-fire on the same minute.
		if stop := d.sleep(ctx, 61*time.Second); stop {
			return nil
		}
	}
}

// refresh makes sure the schedule covers the days ahead. Times are computed
// locally, so this cannot fail for network reasons; the returned duration is
// simply how long to wait before the next routine recompute.
func (d *Daemon) refresh(ctx context.Context) time.Duration {
	d.mu.Lock()
	cfg := *d.cfg
	sched := d.sched
	if sched.Timezone == "" && cfg.Location.Timezone != "" {
		sched.Timezone = cfg.Location.Timezone
	}
	d.mu.Unlock()

	computed, err := sched.Ensure(ctx, cfg.Location.Latitude, cfg.Location.Longitude,
		cfg.Method, cfg.Tune, time.Now())
	if err != nil {
		d.logger.Printf("could not compute prayer times: %v", err)
		return retryInterval
	}

	if computed {
		if path, perr := config.SchedulePath(); perr == nil {
			if serr := sched.Save(path); serr != nil {
				d.logger.Printf("could not cache schedule: %v", serr)
			}
		}
		d.logger.Printf("prayer times computed for the days ahead")
	}
	return refreshInterval
}

func (d *Daemon) nextEvent() (prayer.Event, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sched.Next(time.Now())
}

// announce plays the adhan unless the user has muted it.
func (d *Daemon) announce(ev prayer.Event) {
	d.mu.Lock()
	muted := d.cfg.Muted
	sound := d.cfg.SoundPath
	if ev.Name == "Fajr" && d.cfg.FajrSoundPath != "" {
		sound = d.cfg.FajrSoundPath
	}
	d.mu.Unlock()

	if muted {
		d.logger.Printf("%s is due; muted, staying silent", ev.Name)
		return
	}
	d.logger.Printf("%s - playing adhan", ev.Name)
	if err := d.player.Play(sound); err != nil {
		d.logger.Printf("playback failed: %v", err)
	}
}

// wait blocks for dur, a reload, a quit, or context cancellation, reporting
// which one interrupted it (or that dur simply elapsed).
func (d *Daemon) wait(ctx context.Context, dur time.Duration) waitResult {
	timer := time.NewTimer(dur)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return waitStopped
	case <-d.quit:
		return waitStopped
	case <-d.reload:
		return waitReloaded
	case <-timer.C:
		return waitElapsed
	}
}

// sleep waits for dur, a reload, a quit, or context cancellation.
// It reports true when the daemon should shut down.
func (d *Daemon) sleep(ctx context.Context, dur time.Duration) bool {
	return d.wait(ctx, dur) == waitStopped
}

// waitUntil blocks until target, polling in short steps (see
// wakeCheckInterval) rather than trusting one long timer across it. It
// reports true for stopped if the daemon should shut down, and true for
// reloaded if a reload interrupted the wait (the caller should re-derive
// the schedule rather than assume target is still correct).
func (d *Daemon) waitUntil(ctx context.Context, target time.Time) (stopped, reloaded bool) {
	for {
		remaining := time.Until(target)
		if remaining <= 0 {
			return false, false
		}
		switch d.wait(ctx, min(remaining, wakeCheckInterval)) {
		case waitStopped:
			return true, false
		case waitReloaded:
			return false, true
		}
	}
}

// Shutdown stops playback and unblocks Run.
func (d *Daemon) Shutdown() {
	d.once.Do(func() {
		d.player.Stop()
		close(d.quit)
	})
}

// handle answers one CLI command.
func (d *Daemon) handle(req ipc.Request) ipc.Response {
	switch req.Command {
	case ipc.CmdStatus:
		return ipc.Response{OK: true, Status: d.status()}

	case ipc.CmdStop:
		if d.player.Stop() {
			return ipc.Response{OK: true, Message: "adhan stopped"}
		}
		return ipc.Response{OK: true, Message: "nothing is playing"}

	case ipc.CmdTest:
		d.mu.Lock()
		sound := d.cfg.SoundPath
		d.mu.Unlock()
		if err := d.player.Play(sound); err != nil {
			return ipc.Response{Error: err.Error()}
		}
		return ipc.Response{OK: true, Message: "playing adhan - run `adzan stop` to silence it"}

	case ipc.CmdMute, ipc.CmdUnmute:
		muted := req.Command == ipc.CmdMute
		d.mu.Lock()
		d.cfg.Muted = muted
		cfg := *d.cfg
		d.mu.Unlock()
		if err := cfg.Save(); err != nil {
			return ipc.Response{Error: fmt.Sprintf("could not persist setting: %v", err)}
		}
		if muted {
			d.player.Stop()
			return ipc.Response{OK: true, Message: "muted - the adhan will not sound until you run `adzan unmute`"}
		}
		return ipc.Response{OK: true, Message: "unmuted"}

	case ipc.CmdReload:
		cfg, err := config.Load()
		if err != nil {
			return ipc.Response{Error: err.Error()}
		}
		d.mu.Lock()
		d.cfg = cfg
		d.mu.Unlock()
		select {
		case d.reload <- struct{}{}:
		default:
		}
		return ipc.Response{OK: true, Message: "configuration reloaded"}

	case ipc.CmdQuit:
		go func() {
			time.Sleep(100 * time.Millisecond)
			d.Shutdown()
		}()
		return ipc.Response{OK: true, Message: "daemon stopping"}

	default:
		return ipc.Response{Error: "unknown command: " + req.Command}
	}
}

func (d *Daemon) status() *ipc.Status {
	d.mu.Lock()
	cfg := *d.cfg
	sched := d.sched
	d.mu.Unlock()

	st := &ipc.Status{
		Location:  cfg.Location.Label,
		Coords:    fmt.Sprintf("%.4f, %.4f", cfg.Location.Latitude, cfg.Location.Longitude),
		Method:    prayer.MethodName(cfg.Method),
		Timezone:  sched.Timezone,
		SoundPath: cfg.SoundPath,
		Muted:     cfg.Muted,
		Playing:   d.player.Playing(),
		StartedAt: d.startedAt,
	}
	if st.Timezone == "" {
		st.Timezone = cfg.Location.Timezone
	}

	now := time.Now().In(sched.Location())
	if timings := sched.Timings(now); timings != nil {
		today := make(map[string]string, len(prayer.Order)+1)
		for _, name := range append(append([]string{}, prayer.Order...), "Sunrise") {
			if v, ok := timings[name]; ok {
				today[name] = prayer.CleanTime(v)
			}
		}
		st.Today = today
	}
	if ev, err := sched.Next(time.Now()); err == nil {
		st.NextName = ev.Name
		st.NextAt = ev.At
	}
	return st
}
