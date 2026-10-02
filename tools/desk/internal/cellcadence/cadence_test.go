package cellcadence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var errStop = errors.New("test stopped")

func testConfig() Config {
	return Config{Cell: "sample", Role: "worker-desk", Interval: 5 * time.Millisecond, Budget: time.Second, Heartbeat: 5 * time.Millisecond}
}
func take(t *testing.T, dir string) *Lease {
	t.Helper()
	l, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}
func good() Result {
	return Result{Outcome: "noop", Summary: "tick role=worker-desk outcome=noop swept=1 acted=0 filed=0 duration=1"}
}
func waitState(t *testing.T, dir string, accept func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s, err := Read(dir)
		if err == nil && accept(s) {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	s, err := Read(dir)
	t.Fatalf("checkpoint wait expired: %#v %v", s, err)
	return State{}
}

func TestSerialCadence(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	cfg := testConfig()
	var count, active, max atomic.Int32
	var starts, finishes []time.Time
	cfg.Guard = func() error {
		s, err := Read(dir)
		if err == nil && s.Sequence == 3 && !s.Running {
			return errStop
		}
		return nil
	}
	err := l.Run(context.Background(), cfg, func(context.Context) Result {
		n := active.Add(1)
		if n > max.Load() {
			max.Store(n)
		}
		starts = append(starts, time.Now())
		count.Add(1)
		time.Sleep(15 * time.Millisecond)
		finishes = append(finishes, time.Now())
		active.Add(-1)
		return good()
	})
	if !errors.Is(err, errStop) || count.Load() != 3 || max.Load() != 1 {
		t.Fatalf("run=%v count=%d overlap=%d", err, count.Load(), max.Load())
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(finishes[i-1]) < cfg.Interval {
			t.Fatal("interval measured before prior completion")
		}
	}
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Running || s.Sequence != 3 || s.Outcome != "noop" || !s.NextDue.After(s.LastFinish) {
		t.Fatalf("wrong completion %#v", s)
	}
}

func TestRestartCheckpoint(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	cfg := testConfig()
	cfg.Interval = time.Hour
	var calls atomic.Int32
	cfg.Guard = func() error {
		if calls.Load() > 0 {
			return errStop
		}
		return nil
	}
	if err := l.Run(context.Background(), cfg, func(context.Context) Result { calls.Add(1); return good() }); !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l = take(t, dir)
	cfg.Guard = nil
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := l.Run(ctx, cfg, func(context.Context) Result { t.Error("restarted ahead of stored due time"); return good() })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.NextDue = time.Now().Add(-24 * time.Hour)
	if err = write(dir, s); err != nil {
		t.Fatal(err)
	}
	calls.Store(0)
	cfg.Guard = func() error {
		if calls.Load() > 0 {
			return errStop
		}
		return nil
	}
	if err = l.Run(context.Background(), cfg, func(context.Context) Result { calls.Add(1); return good() }); !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	s, err = Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || s.Sequence != 2 {
		t.Fatal("restart replayed missed intervals")
	}
}

func TestRoleLeaseExclusion(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	if other, err := Acquire(dir); !errors.Is(err, ErrBusy) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("duplicate owner: %v", err)
	}
	cfg := testConfig()
	cfg.Interval = time.Hour
	started := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- l.Run(ctx, cfg, func(ctx context.Context) Result {
			close(started)
			<-ctx.Done()
			<-release
			return Result{Err: ctx.Err()}
		})
	}()
	<-started
	if err := l.Close(); err == nil {
		t.Fatal("released active lease")
	}
	cancel()
	if other, err := Acquire(dir); !errors.Is(err, ErrBusy) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("cancellation released before reap: %v", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	_ = take(t, dir)
}

func TestCrashRefusesOverlap(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	cfg := testConfig()
	cfg.Guard = func() error {
		s, err := Read(dir)
		if err == nil && s.Sequence > 0 && !s.Running {
			return errStop
		}
		return nil
	}
	err := l.Run(context.Background(), cfg, func(context.Context) Result { return Result{Outcome: "noop", Summary: good().Summary, Uncertain: true} })
	if !errors.Is(err, ErrUnfinished) {
		t.Fatal(err)
	}
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Running || s.Outcome != "could-not-check" {
		t.Fatalf("forgot surviving child %#v", s)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	l = take(t, dir)
	err = l.Run(context.Background(), cfg, func(context.Context) Result { t.Error("crash caused overlapping launch"); return good() })
	if !errors.Is(err, ErrUnfinished) {
		t.Fatal(err)
	}
}

func TestHeartbeatAndCancel(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	cfg := testConfig()
	cfg.Interval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- l.Run(ctx, cfg, func(ctx context.Context) Result {
			<-ctx.Done()
			time.Sleep(10 * time.Millisecond)
			return Result{Err: ctx.Err()}
		})
	}()
	first := waitState(t, dir, func(s State) bool { return s.Running })
	_ = waitState(t, dir, func(s State) bool { return s.Running && s.Heartbeat.After(first.Heartbeat) })
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Running || s.Outcome != "could-not-check" {
		t.Fatalf("cancellation healthy %#v", s)
	}
}

func TestBudgetAndGuard(t *testing.T) {
	for _, mode := range []string{"budget", "guard"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			l := take(t, dir)
			cfg := testConfig()
			cfg.Interval = time.Hour
			cfg.Budget = 20 * time.Millisecond
			var running atomic.Bool
			if mode == "guard" {
				cfg.Guard = func() error {
					if running.Load() {
						return errStop
					}
					return nil
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- l.Run(ctx, cfg, func(ctx context.Context) Result { running.Store(true); <-ctx.Done(); return Result{Err: ctx.Err()} })
			}()
			s := waitState(t, dir, func(s State) bool { return s.Sequence == 1 && !s.Running })
			if s.Outcome != "could-not-check" {
				t.Fatalf("failed pass healthy %#v", s)
			}
			cancel()
			err := <-done
			if mode == "guard" && !errors.Is(err, errStop) {
				t.Fatal(err)
			}
		})
	}
}

func TestMissingSummaryDegrades(t *testing.T) {
	for _, result := range []Result{{Outcome: "ok"}, {Outcome: "noop", Summary: good().Summary, ExitCode: 1}, {Outcome: "ok", Summary: good().Summary, Err: errors.New("secret-must-not-persist")}} {
		dir := t.TempDir()
		l := take(t, dir)
		cfg := testConfig()
		var count atomic.Int32
		cfg.Guard = func() error {
			s, err := Read(dir)
			if err == nil && s.Sequence > 0 && !s.Running {
				return errStop
			}
			return nil
		}
		if err := l.Run(context.Background(), cfg, func(context.Context) Result { count.Add(1); return result }); !errors.Is(err, errStop) {
			t.Fatal(err)
		}
		s, err := Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		if s.Outcome != "could-not-check" || s.Summary != "" {
			t.Fatalf("false health %#v", s)
		}
		b, _ := os.ReadFile(filepath.Join(dir, "checkpoint.json"))
		if strings.Contains(string(b), "secret-must-not-persist") {
			t.Fatal("raw error persisted")
		}
	}
}

func TestCorruptCheckpointRefused(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	cfg := testConfig()
	var calls atomic.Int32
	cfg.Guard = func() error {
		if calls.Load() > 0 {
			return errStop
		}
		return nil
	}
	if err := l.Run(context.Background(), cfg, func(context.Context) Result { calls.Add(1); return good() }); !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "checkpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	variants := []string{"{}", string(b) + "{}", strings.Replace(string(b), `"running":false`, `"running":true,"running":false`, 1), strings.Replace(string(b), `"running":false`, `"Running":false`, 1), strings.Replace(string(b), `"running":false`, `"running":null`, 1), strings.Replace(string(b), `"cell":"sample"`, `"cell":"foreign"`, 1)}
	cfg.Guard = nil
	for _, raw := range variants {
		if err = os.WriteFile(filepath.Join(dir, "checkpoint.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if err = l.Run(context.Background(), cfg, func(context.Context) Result {
			t.Error("launched from corrupt/foreign checkpoint")
			return Result{Uncertain: true}
		}); err == nil {
			t.Fatal("corrupt checkpoint accepted")
		}
		got, _ := os.ReadFile(filepath.Join(dir, "checkpoint.json"))
		if string(got) != raw {
			t.Fatal("corrupt checkpoint overwritten")
		}
	}
}
