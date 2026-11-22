package cellcadence

import (
	"context"
	"errors"
	"os"
	"testing"
)

// RecordStopped finishes only the unfinished interactive record of the owner it names (#6116); the
// primitive is new, so these are unit tests, not fail-first ones.
func TestRecordStoppedFinishesOnlyTheNamedOwner(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	if err := l.RunInteractive(context.Background(), "sample", "worker-desk", func(context.Context) Result { return Result{Uncertain: true} }); !errors.Is(err, ErrUnfinished) {
		t.Fatal(err)
	}
	rec, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	mine := Generation{PID: os.Getpid(), Start: "owner-start", Sequence: rec.Sequence, LastStart: rec.LastStart}
	stale := mine
	stale.Sequence++
	for name, g := range map[string]Generation{
		"another owner":    {PID: os.Getpid() + 1, Start: "owner-start", Sequence: rec.Sequence, LastStart: rec.LastStart},
		"no owner":         {},
		"unnamed pid only": {PID: os.Getpid()},
		"stale sequence":   stale,
	} {
		if err := l.RecordStopped(g, "stopped"); err == nil {
			t.Fatalf("%s: finished a record it did not write", name)
		}
		if s, err := Read(dir); err != nil || !s.Running {
			t.Fatalf("%s: record changed: %+v %v", name, s, err)
		}
	}
	if err := l.RecordStopped(mine, "stopped by a teardown"); err != nil {
		t.Fatal(err)
	}
	s, err := Read(dir)
	if err != nil || s.Running || s.Outcome != "could-not-check" || s.Summary != "stopped by a teardown" || s.LastFinish.IsZero() || s.ExitCode != -1 {
		t.Fatalf("stopped record: %+v %v", s, err)
	}
	if err := l.RecordStopped(mine, "again"); err == nil {
		t.Fatal("a finished record was finished again")
	}
	pass := false
	if err := l.RunInteractive(context.Background(), "sample", "worker-desk", func(context.Context) Result { pass = true; return Result{} }); err != nil || !pass {
		t.Fatalf("the stopped record refused the next owner: %v %v", pass, err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := l.RecordStopped(mine, "closed"); err == nil {
		t.Fatal("a closed lease recorded a stop")
	}
}

// A cadence record is never finished as an interactive stop.
func TestRecordStoppedRefusesCadenceRecord(t *testing.T) {
	dir := t.TempDir()
	l := take(t, dir)
	_ = l.Run(context.Background(), testConfig(), func(context.Context) Result { return Result{Uncertain: true} })
	if s, err := Read(dir); err != nil || !s.Running {
		t.Fatalf("setup: %+v %v", s, err)
	}
	if err := l.RecordStopped(Generation{PID: os.Getpid(), Start: "owner-start"}, "stopped"); err == nil {
		t.Fatal("a cadence pass's unfinished record was finished")
	}
}
