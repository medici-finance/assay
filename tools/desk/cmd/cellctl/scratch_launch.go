package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
)

type scratchLaunch struct {
	store *cellscratch.Store
	run   *cellscratch.Run
	tail  *cellscratch.Tail
}

func (c *Cell) beginScratch(role string, env []string) (*scratchLaunch, []string, error) {
	began := time.Now()
	if _, err := c.launchPrepareLaunch(); err != nil {
		return nil, env, err
	}
	value := func(key string) string {
		for _, kv := range env {
			if k, v, ok := strings.Cut(kv, "="); ok && k == key {
				return v
			}
		}
		return ""
	}
	revision := value("ASSAY_SOURCE_REVISION")
	if revision == "" {
		return nil, env, errors.New("launch lacks source revision for scratch ownership")
	}
	session := value("DESK_SESSION")
	if session == "" {
		session = c.Name + "/" + role
	}
	_, err := c.scratchPolicy()
	if err != nil {
		return nil, env, err
	}
	s, err := cellscratch.Open(filepath.Join(c.Dir, "run", "scratch"))
	if err != nil {
		return nil, env, err
	}
	s.Label(c.Name, role)
	r, err := s.Begin(session, role, revision)
	c.recordLaunchStep("launch-scratch-lock-wait", s.LockWait(), err)
	c.recordLaunchStep("launch-scratch-admission", time.Since(began), err)
	if err != nil {
		s.Close()
		return nil, env, err
	}
	cellsRoot, cellName := c.reenter()
	env = taskTempEnv(env, r.Work())
	for k, v := range map[string]string{"ASSAY_SCRATCH_ID": r.Record.ID, "ASSAY_SCRATCH_ROOT": s.Path, "ASSAY_SCRATCH_CELL": cellName, "CELLS_ROOT": cellsRoot} {
		env = envSet(env, k, v)
	}
	return &scratchLaunch{s, r, r.Tail()}, env, nil
}
func (s *scratchLaunch) finish(result *cellcadence.Result) {
	code := result.ExitCode
	if code == 0 && result.Err != nil {
		code = 1
	} // malformed completion remains a diagnostic failure
	err := s.run.Finish(code, !result.Uncertain && s.tail.Err() == nil)
	err = errors.Join(err, s.run.Close())
	err = errors.Join(err, s.store.Close())
	if err != nil {
		result.Err = errors.Join(result.Err, err)
		result.Outcome = "could-not-check"
	}
}

// taskTempKeys are every temp-root variable a managed task points at its work
// dir. taskTempEnv is the one place a task environment gets them, so a launch
// site cannot set TMPDIR and leave a harness's own root under the system temp dir.
var taskTempKeys = []string{"TMPDIR", "TMP", "TEMP", harnessTempRootKey}

func taskTempEnv(env []string, work string) []string {
	for _, k := range taskTempKeys {
		env = envSet(env, k, work)
	}
	return env
}

// harnessTempRootKey is the claude harness's documented temp-root override
// (code.claude.com/docs/en/env-vars, CLAUDE_CODE_TMPDIR). Codex's configuration
// reference documents no temp-root override; it names $TMPDIR as the temp
// location, which the launch already sets. Its state stays under CODEX_HOME.
// On macOS the claude root defaults to /tmp and ignores TMPDIR, so without the
// key the harness's per-session scratch lands outside every task record (#7274).
const harnessTempRootKey = "CLAUDE_CODE_TMPDIR"

// Append overrides after the original command environment and before the prompt:
// Codex commands must inherit the SAME scratch owner as the harness process.
// The claude temp root crosses too, so a claude run started from a Codex command
// keeps its session scratch inside the same task.
func scratchArgv(harness string, args, env []string) []string {
	if harness != "codex" || len(args) == 0 {
		return args
	}
	out := append([]string{}, args[:len(args)-1]...)
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && (strings.HasPrefix(k, "ASSAY_SCRATCH_") || slices.Contains(taskTempKeys, k) || k == "CELLS_ROOT" || k == "ASSAY_SOURCE_REVISION") {
			b, _ := json.Marshal(v)
			out = append(out, "-c", "shell_environment_policy.set."+k+"="+string(b))
		}
	}
	return append(out, args[len(args)-1])
}

// scratchPolicy reads existing cell.env settings; there is no second config file.
func (c *Cell) scratchPolicy() (cellscratch.Policy, error) {
	p := cellscratch.DefaultPolicy()
	if c.Env == nil {
		return p, nil
	}
	if raw := c.Env.Get("CELL_SCRATCH_MAX_AGE"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value < 0 {
			return p, fmt.Errorf("CELL_SCRATCH_MAX_AGE must be a nonnegative duration")
		}
		p.MaxAge = value
	}
	if raw := c.Env.Get("CELL_SCRATCH_MAX_BYTES"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return p, fmt.Errorf("CELL_SCRATCH_MAX_BYTES must be nonnegative bytes")
		}
		p.MaxBytes = value
	}
	return p, nil
}

// interactiveOutput preserves terminal descriptors. Wrapping a terminal in an
// io.Writer makes os/exec create a pipe, changing the interactive program's ABI.
// Only redirected streams are copied; a bounded marker records absent capture.
func interactiveOutput(file *os.File, tail io.Writer) io.Writer {
	if term.IsTerminal(int(file.Fd())) {
		_, _ = fmt.Fprintln(tail, "interactive terminal stream: output capture unavailable; retain required evidence explicitly")
		return file
	}
	return io.MultiWriter(file, tail)
}
