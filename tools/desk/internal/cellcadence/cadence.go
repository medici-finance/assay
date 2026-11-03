// Package cellcadence supplies the process-owned clock for bounded desk passes.
// It does not invoke a model, validate the model's tick grammar or install an OS
// service. The caller owns child-tree cancellation and must reap before returning.
package cellcadence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const Schema = "cell-cadence-v1"

var (
	ErrBusy       = errors.New("cell role is already owned by another session")
	ErrNoState    = errors.New("no cadence checkpoint")
	ErrUnfinished = errors.New("previous role owner did not finish; inspect surviving children before explicit recovery")
)

type Config struct {
	Cell, Role       string
	Interval, Budget time.Duration
	Heartbeat        time.Duration               // default five seconds
	Guard            func() error                // invoked before any launch and on every heartbeat
	BetweenPasses    func(context.Context) error // maintenance, never initial admission or child finalization
}

// Result must be classified by the caller against the published tick grammar.
// A zero process exit alone is not success. Err and missing summaries override
// optimistic outcomes; raw errors/output are never persisted by this package.
type Result struct {
	Outcome, Summary string
	ExitCode         int
	Err              error
	Uncertain        bool
}

type State struct {
	Schema      string        `json:"schema"`
	Mode        string        `json:"mode,omitempty"` // empty is the original cadence schema
	Cell        string        `json:"cell"`
	Role        string        `json:"role"`
	PID         int           `json:"pid"` // diagnostic only; never authorizes teardown/recovery
	Interval    time.Duration `json:"interval"`
	Budget      time.Duration `json:"budget"`
	NextDue     time.Time     `json:"next_due"`
	Heartbeat   time.Time     `json:"heartbeat"`
	LastStart   time.Time     `json:"last_start"`
	LastFinish  time.Time     `json:"last_finish"`
	Running     bool          `json:"running"`
	Sequence    uint64        `json:"sequence"`
	Outcome     string        `json:"outcome"`
	Summary     string        `json:"summary,omitempty"`
	ExitCode    int           `json:"exit_code"`
	Maintenance string        `json:"maintenance,omitempty"`
}

// Lease is also used by interactive launches so a scheduled and an interactive
// role cannot overlap. Never unlink its lock file: a second inode is a new lock.
type Lease struct {
	dir     string
	file    *os.File
	root    *os.Root
	held    *deskkit.HeldLock
	mu      sync.Mutex
	running bool
}

// RoleLock describes a launch's claim on a role lease: which cell and role it is,
// how long a transient collision may delay it, and whether the operator opted in
// to ending a proven-stale holder (deskkit.LockSpec has the rules).
type RoleLock struct {
	Cell, Role string
	Wait       time.Duration
	EndStale   bool
	OptIn      string
	Log        io.Writer
}

// Acquire takes the lease at once or reports ErrBusy; status probes use it.
func Acquire(dir string) (*Lease, error) { return acquire(dir, nil) }

// AcquireRole takes the lease for a launch. A lasting busy read wraps ErrBusy
// around a *deskkit.BusyError that names the lock, its holder, whether the holder
// is current and the action taken or available.
func AcquireRole(dir string, rl RoleLock) (*Lease, error) { return acquire(dir, &rl) }

func acquire(dir string, rl *RoleLock) (*Lease, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("cadence directory must be absolute")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("cadence directory must be a real directory")
	}
	p := filepath.Join(dir, "owner.lock")
	if st, err = os.Lstat(p); err == nil && !st.Mode().IsRegular() {
		return nil, errors.New("cadence lock is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	spec := deskkit.LockSpec{Name: "role lease", Probe: true}
	if rl != nil {
		spec = deskkit.LockSpec{Name: "role lease", Cell: rl.Cell, Role: rl.Role, Wait: rl.Wait, EndStale: rl.EndStale, OptIn: rl.OptIn, Log: rl.Log}
	}
	held, err := deskkit.AcquireHeld(root, "owner.lock", f, spec)
	if err != nil {
		_ = root.Close()
		if errors.Is(err, deskkit.ErrLockBusy) {
			var busy *deskkit.BusyError
			if errors.As(err, &busy) {
				return nil, fmt.Errorf("%w: %w", ErrBusy, busy)
			}
			return nil, ErrBusy
		}
		return nil, err
	}
	return &Lease{dir: dir, file: f, root: root, held: held}, nil
}

// RecordChild names the role child this lease serves (0 clears it), so a waiter
// can tell a holder with a live role session from one whose child has exited.
func (l *Lease) RecordChild(pid int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held == nil {
		return errors.New("lease already released")
	}
	return l.held.RecordChild(pid)
}

func (l *Lease) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return errors.New("cannot release cadence ownership while Run is active")
	}
	if l.file == nil {
		return nil
	}
	err := l.held.Release()
	closeErr := l.root.Close()
	l.file, l.held, l.root = nil, nil, nil
	return errors.Join(err, closeErr)
}

func Read(dir string) (State, error) {
	var s State
	p := filepath.Join(dir, "checkpoint.json")
	st, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return s, ErrNoState
	}
	if err != nil {
		return s, err
	}
	if !st.Mode().IsRegular() || st.Size() > 64*1024 {
		return s, errors.New("invalid cadence checkpoint file")
	}
	f, err := os.Open(p)
	if err != nil {
		return s, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(st, opened) {
		return s, errors.New("cadence checkpoint identity changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return s, err
	}
	if len(b) > 64*1024 {
		return s, errors.New("cadence checkpoint exceeds limit")
	}
	if !utf8.Valid(b) {
		return s, errors.New("invalid cadence checkpoint encoding")
	}
	// A duplicate/case-alias running key must never turn an unfinished child into
	// permission to launch another. This schema is a flat object of scalars.
	keys := json.NewDecoder(bytes.NewReader(b))
	tok, keyErr := keys.Token()
	if keyErr != nil || tok != json.Delim('{') {
		return s, errors.New("invalid cadence checkpoint object")
	}
	seen := map[string]bool{}
	for keys.More() {
		tok, keyErr = keys.Token()
		k, ok := tok.(string)
		fold := strings.ToLower(k)
		if keyErr != nil || !ok || seen[fold] {
			return s, errors.New("duplicate cadence checkpoint member")
		}
		seen[fold] = true
		var raw json.RawMessage
		if keys.Decode(&raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return s, errors.New("invalid cadence checkpoint value")
		}
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&s); err != nil {
		return s, errors.New("invalid cadence checkpoint JSON")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return s, errors.New("trailing cadence checkpoint JSON")
	}
	canonical, _ := json.Marshal(s)
	var a, bmap any
	if json.Unmarshal(b, &a) != nil || json.Unmarshal(canonical, &bmap) != nil || !reflect.DeepEqual(a, bmap) {
		return s, errors.New("noncanonical cadence checkpoint members")
	}
	if s.Schema != Schema || s.Cell == "" || s.Role == "" || s.PID <= 0 || s.NextDue.IsZero() || s.Heartbeat.IsZero() {
		return s, errors.New("invalid cadence checkpoint fields")
	}
	if s.Mode != "" && s.Mode != "interactive" || s.Mode == "" && (s.Interval <= 0 || s.Budget <= 0) || s.Mode == "interactive" && (s.Interval != 0 || s.Budget != 0 || s.Sequence == 0) {
		return s, errors.New("invalid role owner mode or timing")
	}
	if s.Maintenance != "" && s.Maintenance != "checked-clean" && s.Maintenance != "could-not-check" {
		return s, errors.New("invalid maintenance outcome")
	}
	if s.Sequence == 0 && (s.Running || !s.LastStart.IsZero() || !s.LastFinish.IsZero() || s.Outcome != "never-run") {
		return s, errors.New("invalid unused cadence checkpoint")
	}
	if s.Sequence > 0 && (s.LastStart.IsZero() || !s.Running && (s.LastFinish.Before(s.LastStart) || !validOutcome(s.Outcome))) {
		return s, errors.New("incomplete cadence checkpoint")
	}
	return s, nil
}

func write(dir string, s State) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".checkpoint-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "checkpoint.json"))
}

// Run executes at most one pass at a time. After completion it waits Interval;
// restart resumes the stored due time and coalesces downtime into one pass.
// A checkpoint left Running is never automatically cleared using a PID probe:
// its child could outlive this process. Explicit operator recovery is required.
func (l *Lease) Run(ctx context.Context, c Config, pass func(context.Context) Result) error {
	if c.Cell == "" || c.Role == "" || c.Interval <= 0 || c.Budget <= 0 || pass == nil {
		return errors.New("invalid cadence configuration")
	}
	if c.Heartbeat == 0 {
		c.Heartbeat = 5 * time.Second
	}
	if c.Heartbeat < 0 {
		return errors.New("invalid heartbeat interval")
	}
	l.mu.Lock()
	if l.file == nil || l.running {
		l.mu.Unlock()
		return errors.New("cadence lease closed or already running")
	}
	l.running = true
	l.mu.Unlock()
	defer func() { l.mu.Lock(); l.running = false; l.mu.Unlock() }()
	s, err := Read(l.dir)
	if err != nil && !errors.Is(err, ErrNoState) {
		return err
	}
	if err == nil {
		if s.Cell != c.Cell || s.Role != c.Role {
			return errors.New("cadence checkpoint belongs to another cell or role")
		}
		if s.Running {
			return ErrUnfinished
		}
	} else {
		s = State{Schema: Schema, Cell: c.Cell, Role: c.Role, NextDue: time.Now().UTC(), Outcome: "never-run"}
	}
	s.PID = os.Getpid()
	s.Mode = ""
	s.Interval = c.Interval
	s.Budget = c.Budget
	beat := time.NewTicker(c.Heartbeat)
	defer beat.Stop()
	maintain := func() error {
		if c.BetweenPasses == nil || s.Sequence == 0 {
			return nil
		}
		// The child has been reaped and its finish checkpoint persisted. Keep
		// the existing role lease and heartbeat while the maintenance owner works.
		maintenance, stop := context.WithCancel(ctx)
		defer stop()
		finished := make(chan error, 1)
		go func() { finished <- c.BetweenPasses(maintenance) }()
		var interrupted error
		done := ctx.Done()
		for {
			select {
			case err := <-finished:
				s.Maintenance = "checked-clean"
				if err != nil {
					s.Maintenance = "could-not-check"
				}
				s.Heartbeat = time.Now().UTC()
				return errors.Join(interrupted, write(l.dir, s))
			case <-done:
				interrupted = ctx.Err()
				stop()
				done = nil // retain custody until existing maintenance returns
			case <-beat.C:
				s.Heartbeat = time.Now().UTC()
				if err := write(l.dir, s); err != nil {
					interrupted = err
					stop()
				}
				if c.Guard != nil {
					if err := c.Guard(); err != nil {
						interrupted = err
						stop()
					}
				}
			}
		}
	}
	// A loaded checkpoint is not a maintenance receipt: the first launch keeps
	// its ordinary bounded admission. Only this supervisor's completed passes
	// enter the between-pass lifecycle.
	completedHere := false
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		if c.Guard != nil {
			if err = c.Guard(); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		s.Heartbeat = now
		if err = write(l.dir, s); err != nil {
			return fmt.Errorf("cadence checkpoint: %w", err)
		}
		if now.Before(s.NextDue) {
			due := time.NewTimer(time.Until(s.NextDue))
			select {
			case <-ctx.Done():
				due.Stop()
				return ctx.Err()
			case <-beat.C:
				due.Stop()
				continue
			case <-due.C:
			}
			continue
		}
		// A heartbeat may win the select at the due instant. Put the boundary
		// check here so every subsequent pass, not only the timer case, sees it.
		if completedHere {
			if err = maintain(); err != nil {
				return err
			}
			now = time.Now().UTC()
		}
		s.Sequence++
		s.Running = true
		s.LastStart = now
		s.Outcome = "running"
		s.Summary = ""
		if s.Sequence == 0 {
			return errors.New("cadence sequence exhausted")
		}
		if err = write(l.dir, s); err != nil {
			return fmt.Errorf("cadence start checkpoint: %w", err)
		}
		child, cancel := context.WithTimeout(ctx, c.Budget)
		finished := make(chan Result, 1)
		go func() { finished <- pass(child) }()
		var result Result
		var passErr error
		done := child.Done()
	await:
		for {
			select {
			case result = <-finished:
				break await
			case <-done:
				passErr = child.Err()
				done = nil // cancellation requests cleanup; keep lease until callback has reaped
			case <-beat.C:
				s.Heartbeat = time.Now().UTC()
				if err = write(l.dir, s); err != nil {
					passErr = err
					cancel()
					done = nil
				}
				if c.Guard != nil {
					if err = c.Guard(); err != nil {
						passErr = err
						cancel()
						done = nil
					}
				}
			}
		}
		if child.Err() != nil && passErr == nil {
			passErr = child.Err()
		}
		cancel()
		if result.Uncertain {
			s.Heartbeat = time.Now().UTC()
			s.Outcome = "could-not-check"
			s.Summary = ""
			if err = write(l.dir, s); err != nil {
				return errors.Join(ErrUnfinished, err)
			}
			return ErrUnfinished
		}
		s.Running = false
		s.LastFinish = time.Now().UTC()
		s.Heartbeat = s.LastFinish
		s.NextDue = s.LastFinish.Add(c.Interval)
		s.ExitCode = result.ExitCode
		s.Summary = result.Summary
		s.Outcome = result.Outcome
		if result.Err != nil || passErr != nil || result.ExitCode != 0 || !validOutcome(result.Outcome) || result.Summary == "" || len(result.Summary) > 4096 {
			s.Outcome = "could-not-check"
			s.Summary = ""
		}
		if err = write(l.dir, s); err != nil {
			return fmt.Errorf("cadence finish checkpoint: %w", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if passErr != nil && !errors.Is(passErr, context.DeadlineExceeded) {
			return passErr
		}
		completedHere = true
		if err = maintain(); err != nil {
			return err
		}
	}
}

func validOutcome(s string) bool {
	return s == "ok" || s == "noop" || s == "refused" || s == "could-not-check"
}
