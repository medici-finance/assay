package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
)

// These are real compiled subprocesses: killing the owner must not look like
// clean model completion, even when the platform cleans its job on owner death.
func TestInteractiveOwnerFixture(t *testing.T) {
	mode := os.Getenv("CELL_INTERACTIVE_FIXTURE")
	if mode == "" {
		return
	}
	dir := os.Getenv("CELL_INTERACTIVE_DIR")
	if mode == "child" {
		_ = os.WriteFile(filepath.Join(dir, "child.pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
		for {
			time.Sleep(time.Hour)
		}
	}
	c := &Cell{Name: "sample", Dir: dir}
	l, err := cellcadence.Acquire(c.cadenceDir("worker-desk"))
	if err != nil {
		os.Exit(91)
	}
	c.cadenceLease = l
	defer l.Close()
	env := envSet(os.Environ(), "CELL_INTERACTIVE_FIXTURE", "child")
	env = envSet(env, "ASSAY_SOURCE_REVISION", "fixture-revision")
	c.runInteractiveHarness("worker-desk", []string{os.Args[0], "-test.run=^TestInteractiveOwnerFixture$"}, env, dir)
}

func TestInteractiveOwnerCrashRefusesCadence(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestInteractiveOwnerFixture$")
	cmd.Env = append(os.Environ(), "CELL_INTERACTIVE_FIXTURE=owner", "CELL_INTERACTIVE_DIR="+dir)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	cmd.WaitDelay = 200 * time.Millisecond
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	var pid int
	until := time.Now().Add(10 * time.Second)
	for pid == 0 {
		if b, err := os.ReadFile(filepath.Join(dir, "child.pid")); err == nil {
			pid, _ = strconv.Atoi(string(b))
		}
		if time.Now().After(until) {
			t.Fatal("interactive child never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	})
	stateDir := filepath.Join(dir, "run", "cadence", "worker-desk")
	if l, err := cellcadence.Acquire(stateDir); !errors.Is(err, cellcadence.ErrBusy) {
		if l != nil {
			_ = l.Close()
		}
		t.Fatalf("live owner not exclusive: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	l, err := cellcadence.Acquire(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	launched := false
	err = l.Run(ctx, cellcadence.Config{Cell: "sample", Role: "worker-desk", Interval: time.Second, Budget: time.Second}, func(context.Context) cellcadence.Result { launched = true; return cellcadence.Result{Uncertain: true} })
	if launched || !errors.Is(err, cellcadence.ErrUnfinished) {
		t.Fatalf("owner crash allowed replacement: launched=%v error=%v", launched, err)
	}
	s, err := cellcadence.Read(stateDir)
	if err != nil || !s.Running {
		t.Fatalf("owner crash lost dirty checkpoint: %+v %v", s, err)
	}
}
