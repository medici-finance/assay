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
	Heartbeat        time.Duration // default five seconds
	Guard            func() error  // invoked before any launch and on every heartbeat
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
	Schema     string        `json:"schema"`
	Mode       string        `json:"mode,omitempty"` // empty is the original cadence schema
	Cell       string        `json:"cell"`
	Role       string        `json:"role"`
	PID        int           `json:"pid"` // diagnostic only; never authorizes teardown/recovery
	Interval   time.Duration `json:"interval"`
	Budget     time.Duration `json:"budget"`
	NextDue    time.Time     `json:"next_due"`
	Heartbeat  time.Time     `json:"heartbeat"`
	LastStart  time.Time     `json:"last_start"`
	LastFinish time.Time     `json:"last_finish"`
	Running    bool          `json:"running"`
	Sequence   uint64        `json:"sequence"`
	Outcome    string        `json:"outcome"`
	Summary    string        `json:"summary,omitempty"`
	ExitCode   int           `json:"exit_code"`
}

// Lease is also used by interactive launches so a scheduled and an interactive
// role cannot overlap. Never unlink its lock file: a second inode is a new lock.
type Lease struct {
	dir     string
	file    *os.File
	mu      sync.Mutex
	running bool
}

func Acquire(dir string) (*Lease, error) {
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
	if err = deskkit.TryLockExclusive(f); err != nil {
		_ = f.Close()
		if errors.Is(err, deskkit.ErrLockBusy) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return &Lease{dir: dir, file: f}, nil
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
	err := deskkit.UnlockFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
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
	}
}

func validOutcome(s string) bool {
	return s == "ok" || s == "noop" || s == "refused" || s == "could-not-check"
}
