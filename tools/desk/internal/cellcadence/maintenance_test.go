package cellcadence

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBetweenPassMaintenanceCustody(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	cfg := testConfig()
	entered, release := make(chan struct{}), make(chan struct{})
	cfg.BetweenPasses = func(ctx context.Context) error {
		s, err := Read(dir)
		if err != nil || s.Running || s.LastFinish.IsZero() || s.Sequence != 1 {
			t.Errorf("maintenance entered before completed child checkpoint: %+v %v", s, err)
		}
		close(entered)
		<-release
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx, cfg, func(context.Context) Result { return good() }) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("maintenance not reached")
	}
	first, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, dir, func(s State) bool { return !s.Running && s.Heartbeat.After(first.Heartbeat) })
	cancel()
	if _, err := Acquire(dir); !errors.Is(err, ErrBusy) {
		t.Fatalf("lost role custody during maintenance: %v", err)
	}
	select {
	case <-done:
		t.Fatal("returned before maintenance released custody")
	default:
	}
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("maintenance did not release")
	}
	s, err := Read(dir)
	if err != nil || s.Maintenance != "could-not-check" {
		t.Fatalf("maintenance failure not recorded: %+v %v", s, err)
	}
}

func TestUncertainPassDoesNotMaintain(t *testing.T) {
	l := take(t, t.TempDir())
	cfg := testConfig()
	called := false
	cfg.BetweenPasses = func(context.Context) error { called = true; return nil }
	err := l.Run(context.Background(), cfg, func(context.Context) Result { return Result{Uncertain: true} })
	if !errors.Is(err, ErrUnfinished) || called {
		t.Fatalf("uncertain child authorized maintenance: err=%v called=%v", err, called)
	}
}
