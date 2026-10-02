package cellcadence

import (
	"context"
	"errors"
	"testing"
)

func TestInteractiveOwnershipAndCadenceTransition(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	err := l.RunInteractive(context.Background(), "sample", "worker-desk", func(context.Context) Result {
		s, err := Read(dir)
		if err != nil || !s.Running || s.Mode != "interactive" {
			t.Errorf("child started without dirty checkpoint: %+v %v", s, err)
		}
		if err := l.Close(); err == nil {
			t.Error("released ownership before cleanup")
		}
		return Result{}
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Read(dir)
	if err != nil || s.Running || s.Outcome != "ok" {
		t.Fatalf("clean completion: %+v %v", s, err)
	}
	launched := false
	err = l.Run(context.Background(), testConfig(), func(context.Context) Result { launched = true; return Result{Uncertain: true} })
	if !launched || !errors.Is(err, ErrUnfinished) {
		t.Fatalf("clean interactive prevented cadence: %v %v", launched, err)
	}
}

func TestInteractiveUncertainRefusesBothModes(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	err := l.RunInteractive(context.Background(), "sample", "worker-desk", func(context.Context) Result { return Result{Uncertain: true} })
	if !errors.Is(err, ErrUnfinished) {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l = take(t, dir)
	pass := func(context.Context) Result { t.Error("launched after uncertain cleanup"); return Result{} }
	if err := l.RunInteractive(context.Background(), "sample", "worker-desk", pass); !errors.Is(err, ErrUnfinished) {
		t.Fatal(err)
	}
	if err := l.Run(context.Background(), testConfig(), pass); !errors.Is(err, ErrUnfinished) {
		t.Fatal(err)
	}
}
