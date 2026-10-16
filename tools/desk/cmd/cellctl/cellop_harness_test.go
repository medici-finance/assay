//go:build cellopfixture && (darwin || linux)

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// The harness for the cellopfixture rows (rows 8, 8b and 10): the cellctl
// binary built WITH the fixture, a fixture world per run, and runs of that binary on a real
// pseudo-terminal (openPTY, per OS) or off one. Everything is offline: the world's environment is
// complete (nothing inherited), PATH is the world's stubs plus the system directories, and
// KUBECONFIG is /dev/null.

var (
	fixtureBinOnce sync.Once
	fixtureBin     string
	fixtureBinErr  error
)

// cellopFixtureBinary builds cellctl with -tags cellopfixture once per test process.
func cellopFixtureBinary(t *testing.T) string {
	t.Helper()
	fixtureBinOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			fixtureBinErr = err
			return
		}
		tmp, err := os.MkdirTemp("", "cellctl-cellop-")
		if err != nil {
			fixtureBinErr = err
			return
		}
		out := filepath.Join(tmp, "cellctl")
		cmd := exec.Command("go", "build", "-tags", "cellopfixture", "-o", out, "./cmd/cellctl")
		cmd.Dir = filepath.Join(wd, "..", "..")
		if b, err := cmd.CombinedOutput(); err != nil {
			fixtureBinErr = fmt.Errorf("go build -tags cellopfixture ./cmd/cellctl: %v\n%s", err, b)
			return
		}
		fixtureBin = out
	})
	if fixtureBinErr != nil {
		t.Fatal(fixtureBinErr)
	}
	return fixtureBin
}

// cellopWorld is a cliWorld with an offline PATH, a trace file and helpers for the plan store.
type cellopWorld struct {
	*cliWorld
	trace string
}

func newCellopWorld(t *testing.T) *cellopWorld {
	t.Helper()
	w := &cellopWorld{cliWorld: newCLIWorld(t, "")}
	// Anything that would reach a forge or a cluster answers from a stub that fails.
	for _, h := range []string{"gh", "glab", "kubectl", "curl", "ssh"} {
		if err := os.WriteFile(filepath.Join(w.binDir, h), []byte("#!/bin/sh\necho \"offline fixture: $0 refused\" >&2\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w.trace = filepath.Join(w.root, "apply-trace")
	return w
}

// env is the complete environment of a fixture run: the world's base environment with an
// offline PATH, the trace file, and extra.
func (w *cellopWorld) env(extra ...string) []string {
	var env []string
	for _, kv := range w.baseEnv() {
		if strings.HasPrefix(kv, "PATH=") {
			kv = "PATH=" + w.binDir + ":/usr/bin:/bin"
		}
		env = append(env, kv)
	}
	env = append(env, "CELLOP_FIXTURE_TRACE="+w.trace)
	return append(env, extra...)
}

func (w *cellopWorld) store() cellop.Store { return cellop.StoreFor(w.cellDir) }

// fixturePlan stores a valid plan for the example cell, as `cellop-fixture --plan` would.
func (w *cellopWorld) fixturePlan(t *testing.T, verb string) cellop.Plan {
	t.Helper()
	id, err := cellop.NewID()
	if err != nil {
		t.Fatal(err)
	}
	p, err := cellop.NewPlan(id, verb, []string{"example"}, []cellop.Action{{
		ID: verb, Cell: "example", Summary: "fixture: " + verb,
		Preconditions: []cellop.Precondition{{Key: "fixture/state", Value: "absent"}},
	}}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.store().Save(p); err != nil {
		t.Fatal(err)
	}
	return p
}

// traced is what the trace file recorded: one line per arrival at the apply path.
func (w *cellopWorld) traced(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(w.trace)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
}

// effects are the fixture actions that ran, one per line.
func (w *cellopWorld) effects(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(w.cellDir, "run", "cellop-fixture", "effects"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
}

// storedPlans counts the plan documents in the example cell's store.
func (w *cellopWorld) storedPlans(t *testing.T) int {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(w.store().Dir, "*.json"))
	return len(m)
}

// ttyMode says which of stdin and stdout are the pseudo-terminal.
type ttyMode int

const (
	ttyNone   ttyMode = iota // stdin /dev/null, stdout a pipe
	ttyBoth                  // stdin, stdout and stderr the terminal
	ttyStdin                 // stdin and stderr the terminal, stdout a pipe
	ttyStdout                // stdin /dev/null, stdout and stderr the terminal
)

func (m ttyMode) String() string {
	return [...]string{"stdin and stdout not a terminal", "on a terminal", "stdout not a terminal", "stdin not a terminal"}[m]
}

// cellopRun is one finished run.
type cellopRun struct {
	code     int
	out      string // terminal output (with \r removed), then the piped stdout and stderr
	stdout   string // the piped stdout, or the terminal output when stdout is the terminal
	timedOut bool
}

const confirmPrompt = "Type the confirm code to apply: "

// runCellctl runs the fixture binary with args in mode. On a terminal, answer (when non-nil) is
// asked what to type once the confirm prompt appears; a run that sits idle without the prompt gets
// an empty line now and then, so a prompt of some other verb cannot hang the walk.
func runCellctl(t *testing.T, w *cellopWorld, mode ttyMode, env []string, answer func(out string) string, args ...string) cellopRun {
	t.Helper()
	cmd := exec.Command(cellopFixtureBinary(t), args...)
	cmd.Env = env
	cmd.Dir = w.root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var piped, pipedErr bytes.Buffer
	var master, slave *os.File
	if mode == ttyNone {
		cmd.Stdout, cmd.Stderr = &piped, &pipedErr
	} else {
		m, s, err := openPTY()
		if err != nil {
			t.Skipf("no pseudo-terminal available: %v", err)
		}
		master, slave = m, s
		switch mode {
		case ttyBoth:
			cmd.Stdin, cmd.Stdout, cmd.Stderr = s, s, s
		case ttyStdin:
			cmd.Stdin, cmd.Stdout, cmd.Stderr = s, &piped, s
		default:
			cmd.Stdout, cmd.Stderr = s, s
		}
	}
	err := cmd.Start()
	if slave != nil {
		slave.Close() // the child holds its own copies
	}
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var term bytes.Buffer
	readDone := make(chan struct{})
	if master != nil {
		go func() {
			defer close(readDone)
			buf := make([]byte, 4096)
			for {
				n, err := master.Read(buf)
				mu.Lock()
				term.Write(buf[:n])
				mu.Unlock()
				if err != nil {
					return
				}
			}
		}()
	} else {
		close(readDone)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	deadline := time.After(20 * time.Second)
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	answered, lastLen, idleSince, nudges := false, 0, time.Now(), 0
	var res cellopRun
	var werr error
loop:
	for {
		select {
		case werr = <-waited:
			break loop
		case <-deadline:
			res.timedOut = true
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			werr = <-waited
			break loop
		case <-tick.C:
			if master == nil || cmd.Stdin == nil {
				continue
			}
			mu.Lock()
			seen := term.String()
			mu.Unlock()
			if !answered && answer != nil && strings.Contains(seen, confirmPrompt) {
				answered = true
				_, _ = master.Write([]byte(answer(seen) + "\n"))
				continue
			}
			if len(seen) != lastLen {
				lastLen, idleSince = len(seen), time.Now()
			} else if time.Since(idleSince) > 1500*time.Millisecond && nudges < 4 {
				nudges++
				idleSince = time.Now()
				_, _ = master.Write([]byte("\n"))
			}
		}
	}
	if master != nil {
		select {
		case <-readDone:
		case <-time.After(time.Second):
		}
		master.Close()
		<-readDone
	}
	res.code = 0
	if werr != nil {
		var ee *exec.ExitError
		if !errors.As(werr, &ee) {
			t.Fatalf("running %v: %v", args, werr)
		}
		res.code = ee.ExitCode()
	}
	mu.Lock()
	termOut := strings.ReplaceAll(term.String(), "\r", "")
	mu.Unlock()
	res.out = termOut + piped.String() + pipedErr.String()
	res.stdout = piped.String()
	if mode == ttyBoth || mode == ttyStdout {
		res.stdout = termOut
	}
	return res
}

// lastJSON is the last line of out that is a cell-op document.
func lastJSON(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, `{"schema":"cell-op-v1"`) {
			return l
		}
	}
	return ""
}
