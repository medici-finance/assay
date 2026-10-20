//go:build !windows

package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A terminal Ctrl-C signals muhar's foreground process group. Each suite runs
// in a group of its own, so the interrupt handler must kill those groups:
// otherwise the suite outlives muhar, orphaned.
// regression: #2639
func TestInterruptKillsSuite(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(root, "pkg", "p.go"), "package pkg\nfunc Control() int { return 1 }\nfunc Mutant() int { return 2 }\n")
	pidFile := filepath.Join(t.TempDir(), "suite.pid")
	spec := map[string]any{
		"root":      root,
		"test":      "if grep -q 'return 0' pkg/p.go; then exit 1; fi; if grep -q 'return 3' pkg/p.go; then sleep 120 & echo $! > " + shellQuote(pidFile) + "; wait; fi",
		"build":     "true",
		"control":   Mutation{Name: "positive-control", File: "pkg/p.go", Old: "return 1", New: "return 0"},
		"mutations": []Mutation{{Name: "slow-mutant", File: "pkg/p.go", Old: "return 2", New: "return 3"}},
		"timeout":   map[string]any{"floor": "10m"},
	}
	body, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(root, "spec.json")
	writeFixture(t, specPath, string(body))

	cmd := exec.Command(muharTestBinary(t), "-spec", specPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // muhar leads its own group, as in a terminal
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }()

	pid := 0
	for end := time.Now().Add(60 * time.Second); pid == 0 && time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		raw, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
	}
	if pid == 0 {
		t.Fatal("the slow mutant's suite never started")
	}
	defer func() { _ = syscall.Kill(pid, syscall.SIGKILL) }()

	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 130 {
			t.Fatalf("muhar exited %v after SIGINT; want exit 130", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("muhar did not exit after SIGINT")
	}
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
	}
	t.Fatalf("the suite's child (pid %d) is still running after muhar exited on SIGINT", pid)
}
