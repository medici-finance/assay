package process

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

// recordingCustody admits every launch and records each Finish result. The
// moved runner tests launch through it so they exercise the same public
// custody boundary every production caller uses.
type recordingCustody struct {
	admitted int
	finished []bool
	refuse   error
	finish   error
	nilAdmit bool
}

type recordingAdmission struct{ c *recordingCustody }

func (c *recordingCustody) Admit([]string) (Admission, error) {
	if c.refuse != nil {
		return nil, c.refuse
	}
	if c.nilAdmit {
		return nil, nil
	}
	c.admitted++
	return recordingAdmission{c}, nil
}

func (a recordingAdmission) Finish(clean bool) error {
	a.c.finished = append(a.c.finished, clean)
	return a.c.finish
}

func testRunner() *Runner {
	r, err := New(&recordingCustody{})
	if err != nil {
		panic(err)
	}
	return r
}

// Package-level shims keep the moved tests' call sites unchanged.
func Run(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer) (int, bool, error) {
	return testRunner().Run(ctx, argv, env, dir, stdout, stderr)
}

func RunInteractive(ctx context.Context, argv, env []string, dir string, stdin *os.File, stdout, stderr io.Writer) (int, bool, error) {
	return testRunner().RunInteractive(ctx, argv, env, dir, stdin, stdout, stderr)
}

func RunObserved(ctx context.Context, argv, env []string, dir string, stdout, stderr io.Writer, started func(int) error) (int, bool, error) {
	return testRunner().RunObserved(ctx, argv, env, dir, stdout, stderr, started)
}

// regression: graph-execution/26 — the moved runner must never launch without
// a custodian, must not launch when custody refuses, and must report an
// uncertain cleanup to the custodian as not clean.
func TestRunnerCustodyBoundary(t *testing.T) {
	if _, err := New(nil); !errors.Is(err, ErrNoCustody) {
		t.Fatalf("New(nil) = %v, want ErrNoCustody", err)
	}
	var zero *Runner
	argv, env := fixture(t, "record", "0")
	if _, uncertain, err := zero.Run(context.Background(), argv, env, t.TempDir(), io.Discard, io.Discard); uncertain || !errors.Is(err, ErrNoCustody) {
		t.Fatalf("nil runner launched or was uncertain: %v %v", uncertain, err)
	}
	if _, uncertain, err := (&Runner{}).RunObserved(context.Background(), argv, env, t.TempDir(), io.Discard, io.Discard, func(int) error {
		t.Fatal("zero runner started a child")
		return nil
	}); uncertain || !errors.Is(err, ErrNoCustody) {
		t.Fatalf("zero runner: %v %v", uncertain, err)
	}

	refusal := errors.New("custody refused")
	refusing := &recordingCustody{refuse: refusal}
	r, _ := New(refusing)
	called := false
	if _, uncertain, err := r.RunObserved(context.Background(), argv, env, t.TempDir(), io.Discard, io.Discard, func(int) error { called = true; return nil }); uncertain || !errors.Is(err, refusal) || called {
		t.Fatalf("refused custody launched: called=%v uncertain=%v err=%v", called, uncertain, err)
	}

	// An admit with no Admission to finish is a contract error before launch:
	// Finish is owed exactly once, so the launch must not run unfinished.
	r, _ = New(&recordingCustody{nilAdmit: true})
	called = false
	if _, uncertain, err := r.RunObserved(context.Background(), argv, env, t.TempDir(), io.Discard, io.Discard, func(int) error { called = true; return nil }); uncertain || !errors.Is(err, ErrNoAdmission) || called {
		t.Fatalf("nil admission launched: called=%v uncertain=%v err=%v", called, uncertain, err)
	}

	c := &recordingCustody{}
	r, _ = New(c)
	if code, uncertain, err := r.Run(context.Background(), argv, env, t.TempDir(), io.Discard, io.Discard); code != 0 || uncertain || err != nil {
		t.Fatalf("clean run: %d %v %v", code, uncertain, err)
	}
	if c.admitted != 1 || len(c.finished) != 1 || !c.finished[0] {
		t.Fatalf("clean run custody record: admitted=%d finished=%v", c.admitted, c.finished)
	}

	// An uncertain cleanup is reported to the custodian as not clean, so it
	// keeps its dirty ownership record.
	c.finished = nil
	if _, uncertain, err := r.withCustody(env, func() (int, bool, error) { return 0, true, nil }); !uncertain || err != nil {
		t.Fatalf("uncertain child: %v %v", uncertain, err)
	}
	if len(c.finished) != 1 || c.finished[0] {
		t.Fatalf("uncertain cleanup reported clean to custody: %v", c.finished)
	}

	c.finish = errors.New("finish failed")
	if _, uncertain, err := r.Run(context.Background(), argv, env, t.TempDir(), io.Discard, io.Discard); !uncertain || !errors.Is(err, c.finish) {
		t.Fatalf("finish failure must be uncertain: %v %v", uncertain, err)
	}
}
