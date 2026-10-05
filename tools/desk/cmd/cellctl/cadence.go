package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
	"github.com/medici-finance/assay/tools/desk/internal/tick"
)

type cadenceOptions struct{ Interval, Budget time.Duration }

// Defaults apply only after the role's model policy has selected its harness.
// Explicit durations and "off" retain precedence; other harnesses/kinds are unchanged.
func resolveDeskCadence(kind, harness, interval, budget string) *cadenceOptions {
	if kind == "house" && harness == "codex" && interval == "" {
		interval = "5m"
	}
	return resolveCadence(kind, interval, budget)
}

func resolveCadence(kind, interval, budget string) *cadenceOptions {
	if interval == "" || interval == "off" {
		if budget != "" && interval != "off" {
			die("--tick-budget requires --cadence (or CELL_CADENCE)")
		}
		return nil
	}
	if kind != "house" {
		die("cadence currently requires a house cell")
	}
	d, err := time.ParseDuration(interval)
	if err != nil || d < time.Second || d > 24*time.Hour || d%time.Second != 0 {
		die("cadence must be a whole-second duration between 1s and 24h")
	}
	if budget == "" {
		budget = "20m"
	}
	b, err := time.ParseDuration(budget)
	if err != nil || b < 61*time.Second || b > 24*time.Hour || b%time.Second != 0 {
		die("tick budget must be a whole-second duration between 61s and 24h (includes the exit reserve)")
	}
	return &cadenceOptions{Interval: d, Budget: b}
}

func (c *Cell) cadenceDir(role string) string { return filepath.Join(c.Dir, "run", "cadence", role) }

func (c *Cell) runInteractiveHarness(role string, argv, env []string, wt string) {
	if c.cadenceLease == nil {
		die("interactive launch requires the role ownership lease")
	}
	ctx, stop := cellprocess.NotifyContext(context.Background())
	defer stop()
	code := -1
	err := c.cadenceLease.RunInteractive(ctx, c.Name, role, func(child context.Context) cellcadence.Result {
		var uncertain bool
		var err error
		scratch, childEnv, beginErr := c.beginScratch(role, env)
		if beginErr != nil {
			return cellcadence.Result{ExitCode: -1, Err: beginErr}
		}
		harness := ""
		for _, kv := range env {
			if strings.HasPrefix(kv, "ASSAY_HARNESS=") {
				harness = strings.TrimPrefix(kv, "ASSAY_HARNESS=")
			}
		}
		childArgs := scratchArgv(harness, argv, childEnv)
		code, uncertain, err = cellprocess.RunInteractiveObserved(child, childArgs, childEnv, wt, os.Stdin, io.MultiWriter(os.Stdout, scratch.tail), io.MultiWriter(os.Stderr, scratch.tail), scratch.run.Started)
		result := cellcadence.Result{ExitCode: code, Uncertain: uncertain, Err: err}
		scratch.finish(&result)
		return result
	})
	if errors.Is(err, cellcadence.ErrUnfinished) {
		die("interactive cleanup: %v", err)
	}
	if code < 0 {
		if err != nil {
			die("interactive launch: %v", err)
		}
		code = 1
	}
	if err != nil && code == 0 {
		die("interactive ownership: %v", err)
	}
	exitWith(code)
}

// The supervisor is the foreground cockpit command. Model completion returns here;
// no harness-specific prompt injection or shell timer is needed to start the next pass.
func (c *Cell) runCadencedHarness(role, harness string, argv, env []string, wt string) {
	if c.cadenceLease == nil {
		die("cadence requires the role ownership lease")
	}
	args, env, err := prepareTickLaunch(harness, argv, env, c.Cadence.Budget)
	if err != nil {
		die("cadence: %v", err)
	}
	ctx, stop := cellprocess.NotifyContext(context.Background())
	defer stop()
	fmt.Printf("[cadence] cell=%s role=%s interval=%s budget=%s state=%s; Go supervisor owns subsequent passes\n", c.Name, role, c.Cadence.Interval, c.Cadence.Budget, c.cadenceDir(role))
	err = c.cadenceLease.Run(ctx, cellcadence.Config{
		Cell: c.Name, Role: role, Interval: c.Cadence.Interval, Budget: c.Cadence.Budget,
		Guard: func() error { return c.cadenceGuard(role) },
	}, func(pass context.Context) cellcadence.Result {
		return c.executeCadencePass(pass, role, harness, args, env, wt)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		die("cadence: %v", err)
	}
}

func (c *Cell) cadenceGuard(role string) error {
	for _, p := range []string{filepath.Join(c.cadenceDir(role), "STOP"), filepath.Join(c.Config, "DISABLED"), filepath.Join(c.Config, "STOP"), filepath.Join(c.Config, "STOP."+role)} {
		_, err := os.Lstat(p)
		if err == nil {
			return fmt.Errorf("stopped by %s", p)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot check stop flag %s: %w", p, err)
		}
	}
	return nil
}

func (c *Cell) executeCadencePass(ctx context.Context, role, harness string, args, env []string, wt string) (result cellcadence.Result) {
	scratch, env, scratchErr := c.beginScratch(role, env)
	if scratchErr != nil {
		return cellcadence.Result{Outcome: "could-not-check", ExitCode: -1, Err: scratchErr}
	}
	defer scratch.finish(&result)
	args = scratchArgv(harness, args, env)
	out := &tailWriter{limit: 64 * 1024}
	runArgs := append([]string(nil), args...)
	resultFile := ""
	if harness == "codex" {
		var capacityErr error
		runArgs, capacityErr = c.refreshCodexCapacity(role, runArgs)
		if capacityErr != nil {
			return cellcadence.Result{Outcome: "could-not-check", ExitCode: -1, Err: capacityErr}
		}
		f, err := os.CreateTemp(scratch.run.Work(), "last-message-*")
		if err != nil {
			return cellcadence.Result{Outcome: "could-not-check", ExitCode: -1, Err: err}
		}
		resultFile = f.Name()
		f.Close()
		runArgs = append(runArgs[:len(runArgs)-1], "--output-last-message", resultFile, runArgs[len(runArgs)-1])
	}
	exit, uncertain, err := cellprocess.RunObserved(ctx, runArgs, env, wt, io.MultiWriter(os.Stdout, out, scratch.tail), io.MultiWriter(os.Stderr, scratch.tail), scratch.run.Started)
	result = cellcadence.Result{Outcome: "could-not-check", ExitCode: exit, Uncertain: uncertain, Err: err}
	if err != nil || exit != 0 || uncertain {
		return result
	}
	final := out.String()
	if resultFile != "" {
		f, openErr := os.Open(resultFile)
		if openErr != nil {
			result.Err = openErr
			return result
		}
		b, readErr := io.ReadAll(io.LimitReader(f, 64*1024+1))
		f.Close()
		if readErr != nil || len(b) > 64*1024 {
			result.Err = fmt.Errorf("last message unreadable or exceeds 64 KiB")
			return result
		}
		final = string(b)
	}
	summary, outcome, validationErr := cadenceSummary(final, role)
	if validationErr != nil {
		result.Err = validationErr
		return result
	}
	result.Summary, result.Outcome = summary, outcome
	return result
}

func cadenceSummary(output, role string) (string, string, error) {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	if err := tick.Check(output); err != nil {
		return "", "", fmt.Errorf("tick did not provide valid completion evidence: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	line := lines[len(lines)-1]
	fields := strings.Fields(line)
	if fields[1] != "role="+role {
		return "", "", fmt.Errorf("tick summary belongs to a different role")
	}
	return line, strings.TrimPrefix(fields[2], "outcome="), nil
}

// Bounded capture: harness progress can be arbitrarily large; only its tail is evidence.
type tailWriter struct {
	mu    sync.Mutex
	buf   []byte
	limit int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	if n >= w.limit {
		w.buf = append(w.buf[:0], p[n-w.limit:]...)
		return n, nil
	}
	w.buf = append(w.buf, p...)
	if len(w.buf) > w.limit {
		w.buf = append(w.buf[:0], w.buf[len(w.buf)-w.limit:]...)
	}
	return n, nil
}
func (w *tailWriter) String() string { w.mu.Lock(); defer w.mu.Unlock(); return string(w.buf) }

func cmdCadence(cell string, args []string) {
	c := loadCell(cell)
	if c.Kind != "house" {
		die("cadence is only defined for house cells")
	}
	if len(args) == 0 {
		die("cadence <cell> status|stop|resume|recover [role] [--confirm-stopped]")
	}
	action := args[0]
	roles := knownRoles
	confirmed := false
	if len(args) > 1 {
		if !valueIn(args[1], knownRoles) {
			die("cadence: unknown role %q", args[1])
		}
		roles = []string{args[1]}
	}
	if len(args) > 2 {
		if len(args) != 3 || args[2] != "--confirm-stopped" || action != "recover" {
			die("cadence: unexpected arguments")
		}
		confirmed = true
	}
	for _, role := range roles {
		dir := c.cadenceDir(role)
		switch action {
		case "status":
			c.printCadenceStatus(role)
		case "stop":
			if err := os.MkdirAll(dir, 0700); err != nil {
				die("cadence stop: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "STOP"), []byte("operator requested stop\n"), 0600); err != nil {
				die("cadence stop: %v", err)
			}
			fmt.Printf("[cadence] %s stop requested; active pass cancellation is bounded by the heartbeat\n", role)
		case "resume", "recover":
			if action == "recover" && !confirmed {
				die("cadence recover requires --confirm-stopped after checking that the prior harness and all its children have stopped")
			}
			lease, err := cellcadence.Acquire(dir)
			if err != nil {
				die("cadence: cannot change active role: %v", err)
			}
			func() {
				defer lease.Close()
				if action == "recover" {
					if err := os.Remove(filepath.Join(dir, "checkpoint.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
						die("cadence recover: %v", err)
					}
				}
				if err := os.Remove(filepath.Join(dir, "STOP")); err != nil && !errors.Is(err, os.ErrNotExist) {
					die("cadence resume: %v", err)
				}
			}()
			fmt.Printf("[cadence] %s %s recorded; start the role to run it\n", role, action)
		default:
			die("cadence: unknown action %q", action)
		}
	}
}

func (c *Cell) printCadenceStatus(role string) {
	dir := c.cadenceDir(role)
	s, err := cellcadence.Read(dir)
	if errors.Is(err, cellcadence.ErrNoState) {
		fmt.Printf("[cadence] role=%s not-configured\n", role)
		return
	}
	if err != nil {
		fmt.Printf("[cadence] role=%s could-not-check: %v\n", role, err)
		return
	}
	if s.Cell != c.Name || s.Role != role {
		fmt.Printf("[cadence] role=%s could-not-check: foreign checkpoint scope\n", role)
		return
	}
	health := "stopped"
	lease, lockErr := cellcadence.Acquire(dir)
	if lockErr == nil {
		lease.Close()
		if s.Running {
			health = "unfinished"
		}
	} else if errors.Is(lockErr, cellcadence.ErrBusy) {
		health = "active"
		if mode, modeErr := readRoleMode(dir); modeErr != nil {
			health = "could-not-check"
		} else if mode != "cadence" {
			health = mode
		}
		if health == "active" && time.Since(s.Heartbeat) > 15*time.Second {
			health = "stale"
		}
	} else {
		health = "could-not-check"
	}
	fmt.Printf("[cadence] role=%s supervisor=%s last=%s running=%t next=%s heartbeat=%s\n", role, health, s.Outcome, s.Running, s.NextDue.UTC().Format(time.RFC3339), s.Heartbeat.UTC().Format(time.RFC3339))
}

// Request cancellation before closing cockpit panes, which otherwise could kill
// the supervisor before it reaps its active harness. No PID is used as authority.
func (c *Cell) stopCadences() error {
	if c.Kind != "house" {
		return nil
	}
	for _, role := range knownRoles {
		dir := c.cadenceDir(role)
		_, err := cellcadence.Read(dir)
		if err != nil && !errors.Is(err, cellcadence.ErrNoState) {
			return err
		}
		lease, err := cellcadence.Acquire(dir)
		if err == nil {
			state, readErr := cellcadence.Read(dir)
			lease.Close()
			if readErr == nil && (state.Cell != c.Name || state.Role != role) {
				return fmt.Errorf("foreign cadence checkpoint scope")
			}
			if readErr != nil && !errors.Is(readErr, cellcadence.ErrNoState) {
				return readErr
			}
			if state.Running {
				return cellcadence.ErrUnfinished
			}
			continue
		}
		if !errors.Is(err, cellcadence.ErrBusy) {
			return err
		}
		mode, err := readRoleMode(dir)
		if err != nil {
			return err
		}
		if mode == "interactive" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "STOP"), []byte("cellctl down\n"), 0600); err != nil {
			return err
		}
		deadline := time.Now().Add(20 * time.Second)
		for {
			lease, err := cellcadence.Acquire(dir)
			if err == nil {
				state, readErr := cellcadence.Read(dir)
				lease.Close()
				if readErr == nil && (state.Cell != c.Name || state.Role != role) {
					return fmt.Errorf("foreign cadence checkpoint scope")
				}
				if readErr != nil && !errors.Is(readErr, cellcadence.ErrNoState) {
					return readErr
				}
				if state.Running {
					return cellcadence.ErrUnfinished
				}
				break
			}
			if !errors.Is(err, cellcadence.ErrBusy) {
				return err
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("cadence %s has not stopped; cockpit left open for inspection", role)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func writeRoleMode(dir, mode string) error {
	f, err := os.CreateTemp(dir, ".owner-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.WriteString(mode); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "owner-mode"))
}
func readRoleMode(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "owner-mode"))
	if err != nil {
		return "", err
	}
	if string(b) != "cadence" && string(b) != "interactive" {
		return "", fmt.Errorf("unrecognized role owner mode")
	}
	return string(b), nil
}
