// Package cellscratch is the sole owner of disposable task scratch, never Git worktrees.
package cellscratch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const Schema = "assay-scratch-v1"
const recordLimit = 16384
const TailLimit = 64 * 1024

type Record struct {
	Schema     string    `json:"schema"`
	Owner      string    `json:"owner"`
	ID         string    `json:"id"`
	Session    string    `json:"session"`
	Task       string    `json:"task"`
	Revision   string    `json:"revision"`
	Created    time.Time `json:"created"`
	Finished   time.Time `json:"finished"`
	State      string    `json:"state"`
	Evidence   string    `json:"evidence"`
	ExitCode   int       `json:"exit_code"`
	ChildGroup int       `json:"child_group"`
	Reaped     bool      `json:"reaped"`
}

type Store struct {
	root  *os.Root
	Path  string
	owner string
	// cell and role label the sweep lock's owner record so a busy launch can name
	// its holder; lockWait bounds how long a sibling's sweep may delay this one.
	cell, role string
	lockWait   time.Duration
	lockLog    io.Writer // late sweep-lock acquisitions; nil writes nothing
	waited     time.Duration
	walks      uint64
	scanLimit  int
	// Tests inject deletion failure at this single destruction boundary.
	remove func(string, pins) error
	// ctx, when bound, stops a periodic sweep between filesystem steps; due and
	// ran carry SweepIfDue's interval through Sweep's maintenance lease.
	ctx context.Context
	due time.Duration
	ran bool
}

// sweepLockName is the cell-wide lock that serialises Open, Begin, Acknowledge
// and the short metadata transactions of Sweep across roles sharing one root.
const sweepLockName = "sweep.lock"
const maintenanceLockName = "maintenance.lock"

// maintenanceStampName records when periodic maintenance last finished cleanly, so
// the supervisors of every role sharing one root coalesce onto one finished sweep per
// interval.
const maintenanceStampName = "maintenance.stamp"

// maintenanceAttemptName records the last periodic sweep that ran but failed or ended
// partial, and how many did so in a row, so siblings also coalesce onto one retry.
// Its name shares the stamp's prefix, which keeps it out of the inventory.
const maintenanceAttemptName = maintenanceStampName + ".attempt"

// MaintenanceRetry is how long after a failed or partial periodic sweep the root waits
// before a sweep may run again, whichever supervisor runs it. Each further failure in a
// row doubles it, never above the sweep interval; a value that is not positive means
// the whole interval.
var MaintenanceRetry = 10 * time.Minute

// SweepLockWait bounds how long one launch waits for a sibling launch's scratch
// metadata transaction. Recursive maintenance never holds this lock.
var SweepLockWait = 2 * time.Minute

// MaintenanceLockWait bounds how long a periodic sweep waits for a sibling's scratch
// transaction before giving up until its next turn.
var MaintenanceLockWait = 10 * time.Second

// OpenMaintenance opens a handle for the supervisor's periodic sweep. Its maintenance
// stops at ctx's cancellation (Bind), each sweep-lock wait is bounded by
// MaintenanceLockWait so stopping the supervisor never waits on a sibling's launch,
// and it never writes lock-wait lines: the supervisor's stderr may be the child's
// interactive terminal, and the maintenance log records any failure.
func OpenMaintenance(ctx context.Context, path string) (*Store, error) {
	s, err := Open(path, func(s *Store) { s.lockWait, s.lockLog = MaintenanceLockWait, nil })
	if err != nil {
		return nil, err
	}
	s.Bind(ctx)
	return s, nil
}

// openOption adjusts a handle before Open takes its first sweep lock; only this
// package supplies one.
type openOption func(*Store)

// Open is the launch and operator form: a busy sweep lock waits up to SweepLockWait
// and a late acquisition is reported on stderr.
func Open(path string, opts ...openOption) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("scratch root must be absolute")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("scratch root must be a private real directory (0700)")
	}
	r, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	s := &Store{root: r, Path: path, lockWait: SweepLockWait, lockLog: os.Stderr}
	for _, opt := range opts {
		opt(s)
	}
	s.remove = s.removeConfined
	lock, err := s.sweepLock()
	if err != nil {
		r.Close()
		return nil, err
	}
	defer lock.Release()
	b, err := s.read("owner", 128)
	if os.IsNotExist(err) {
		token := make([]byte, 16)
		if _, err = rand.Read(token); err == nil {
			b = []byte(hex.EncodeToString(token))
			err = s.atomic("owner", b)
		}
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	if len(b) != 32 {
		r.Close()
		return nil, errors.New("invalid scratch owner record")
	}
	s.owner = string(b)
	return s, nil
}

func (s *Store) Close() error            { return s.root.Close() }
func (s *Store) LockWait() time.Duration { return s.waited }

// Traversals reports this handle's explicit maintenance walks; admission stays zero.
func (s *Store) Traversals() uint64 { return s.walks }

// Bind makes this handle's maintenance stop at ctx's cancellation. A stopped sweep
// reports partial accounting and reclaims nothing further; it never half-decides.
func (s *Store) Bind(ctx context.Context) { s.ctx = ctx }

func (s *Store) stopped() error {
	if s.ctx == nil {
		return nil
	}
	return s.ctx.Err()
}

// Label names the cell and role this store serves; they go into the sweep lock's
// owner record so a sibling that waits on it can name the holder.
func (s *Store) Label(cell, role string) { s.cell, s.role = cell, role }
func (s *Store) lock(name string) (*os.File, error) {
	f, err := s.openLock(name)
	if err != nil {
		return nil, err
	}
	if err = deskkit.TryLockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// sweepLock waits, with backoff, for a sibling's scratch transaction and names
// the lock and its holder when the wait runs out.
func (s *Store) sweepLock() (*deskkit.HeldLock, error) {
	f, err := s.openLock(sweepLockName)
	if err != nil {
		return nil, err
	}
	began := time.Now()
	held, err := deskkit.AcquireHeld(s.root, sweepLockName, f, deskkit.LockSpec{Name: "cell scratch sweep", Cell: s.cell, Role: s.role, Wait: s.lockWait, Log: s.lockLog})
	s.waited += time.Since(began)
	return held, err
}

func (s *Store) openLock(name string) (*os.File, error) {
	st, err := s.root.Lstat(name)
	if err == nil && !st.Mode().IsRegular() {
		return nil, errors.New("scratch lock is not regular")
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := s.root.OpenFile(name, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil || st != nil && !os.SameFile(st, opened) {
		f.Close()
		return nil, errors.New("scratch lock identity changed")
	}
	return f, nil
}

var errRecordChanged = errors.New("scratch record identity changed")

func (s *Store) read(name string, limit int64) ([]byte, error) {
	for attempt := 0; attempt < 3; attempt++ {
		b, err := s.readOnce(name, limit)
		if !errors.Is(err, errRecordChanged) {
			return b, err
		}
	}
	return nil, errRecordChanged
}

func (s *Store) readOnce(name string, limit int64) ([]byte, error) {
	st, err := s.root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, errors.New("invalid or oversized scratch record")
	}
	f, err := s.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	got, err := f.Stat()
	if err != nil || !os.SameFile(st, got) {
		return nil, errRecordChanged
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if len(b) > int(limit) {
		return nil, errors.New("oversized scratch record")
	}
	return b, err
}
func (s *Store) atomic(name string, b []byte) error {
	token := make([]byte, 8)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	tmp := name + "." + hex.EncodeToString(token) + ".tmp"
	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove(tmp)
	_, err = f.Write(b)
	err = errors.Join(err, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return s.root.Rename(tmp, name)
}
func validID(id string) bool {
	kind, nonce, ok := strings.Cut(id, "-")
	if !ok || kind != "task" || len(nonce) != 16 {
		return false
	}
	_, err := hex.DecodeString(nonce)
	return err == nil
}
func (s *Store) record(id string) (Record, error) {
	var r Record
	if !validID(id) {
		return r, errors.New("invalid scratch id")
	}
	st, err := s.root.Lstat(id)
	if err != nil {
		return r, err
	}
	if !st.IsDir() {
		return r, errors.New("scratch entry is not a real directory")
	}
	b, err := s.read(id+"/record.json", recordLimit)
	if err != nil {
		return r, err
	}
	// Only this owner's canonical records authorize lifecycle actions. Reject duplicate,
	// missing and case-aliased fields rather than decoding optimistic zero values.
	if err = json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	canonical, _ := json.Marshal(r)
	if string(b) != string(canonical) || r.Schema != Schema || r.Owner != s.owner || r.ID != id || r.Session == "" || r.Task == "" || r.Revision == "" || r.Created.IsZero() {
		return r, errors.New("unverified ownership record")
	}
	switch r.State {
	case "running", "pending", "failed", "complete", "recovered", "resumable":
	default:
		return r, errors.New("unknown lifecycle state")
	}
	return r, nil
}
func (s *Store) save(r Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(b) > recordLimit {
		return errors.New("scratch record exceeds limit")
	}
	return s.atomic(r.ID+"/record.json", b)
}

type Run struct {
	Store  *Store
	Record Record
	lease  *os.File
}

func (s *Store) Begin(session, task, revision string) (*Run, error) {
	if session == "" || task == "" || revision == "" || len(session)+len(task)+len(revision) > 2048 {
		return nil, errors.New("session, task and source revision are required (2048 bytes maximum)")
	}
	lock, err := s.sweepLock()
	if err != nil {
		return nil, err
	}
	defer lock.Release()
	nonce := make([]byte, 8)
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	id := fmt.Sprintf("task-%s", hex.EncodeToString(nonce))
	if err = s.root.Mkdir(id, 0700); err != nil {
		return nil, err
	}
	lease, err := s.lock(id + "/owner.lock")
	if err != nil {
		return nil, err
	}
	r := Record{Schema: Schema, Owner: s.owner, ID: id, Session: session, Task: task, Revision: revision, Created: time.Now().UTC(), State: "running", Evidence: "pending", ExitCode: -1}
	if err = s.save(r); err == nil {
		err = s.root.Mkdir(id+"/work", 0700)
	}
	if err != nil {
		lease.Close()
		return nil, err
	}
	return &Run{Store: s, Record: r, lease: lease}, nil
}
func (r *Run) Work() string { return filepath.Join(r.Store.Path, r.Record.ID, "work") }
func (r *Run) Close() error {
	if r.lease == nil {
		return nil
	}
	err := r.lease.Close()
	r.lease = nil
	return err
}
func (r *Run) Started(pid int) error { r.Record.ChildGroup = pid; return r.Store.save(r.Record) }
func (r *Run) Finish(code int, reaped bool) error {
	r.Record.ExitCode = code
	r.Record.Reaped = reaped
	r.Record.Finished = time.Now().UTC()
	if code == 0 {
		r.Record.State = "pending"
	} else {
		r.Record.State = "failed"
	}
	if _, err := r.Store.receipt(r.Record.ID); err == nil {
		r.Record.Evidence = "acknowledged"
		if code == 0 {
			r.Record.State = "complete"
		}
	}
	return r.Store.save(r.Record)
}
func (s *Store) receipt(id string) (string, error) {
	b, err := s.read(id+"/receipt", 4096)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("empty evidence receipt")
	}
	return string(b), nil
}

// Acknowledge records a durable destination only AFTER the caller has read it back.
// It never treats a process exit or elapsed time as evidence handoff.
func (s *Store) Acknowledge(id, receipt string, resumable bool) error {
	if strings.TrimSpace(receipt) == "" || len(receipt) > 4096 {
		return errors.New("a bounded canonical evidence receipt is required")
	}
	lock, err := s.sweepLock()
	if err != nil {
		return err
	}
	defer lock.Release()
	r, err := s.record(id)
	if err != nil {
		return err
	}
	if resumable {
		lease, err := s.lock(id + "/owner.lock")
		if err != nil {
			return err
		}
		defer lease.Close()
		r.State = "resumable"
		if err = s.save(r); err != nil {
			return err
		}
	}
	return s.atomic(id+"/receipt", []byte(receipt))
}

type Policy struct {
	MaxAge   time.Duration
	MaxBytes int64
}

func DefaultPolicy() Policy { return Policy{7 * 24 * time.Hour, 256 * 1024 * 1024} }

type Entry struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	Bytes     int64  `json:"bytes"`
	Reclaimed int64  `json:"reclaimed"`
}
type Report struct {
	Apply      bool    `json:"apply"`
	Entries    []Entry `json:"entries"`
	Bytes      int64   `json:"bytes"`
	Reclaimed  int64   `json:"reclaimed"`
	Partial    bool    `json:"partial"`
	OverBudget bool    `json:"over_budget"`
}

// SweepIfDue is the periodic form of Sweep. It skips, reporting ran=false, when a
// sibling holds the maintenance lease, finished a sweep less than interval ago, or
// ran one that failed less than its retry delay ago (retryDelay); otherwise it sweeps
// exactly as Sweep does. Under the same lease, a sweep that completes without error
// or partial accounting stamps its end; one that fails or ends partial, and was not
// stopped, records the failed attempt instead. A zero interval never sweeps.
func (s *Store) SweepIfDue(p Policy, interval time.Duration, apply bool) (Report, bool, error) {
	if interval <= 0 {
		return Report{Apply: apply, Entries: []Entry{}}, false, nil
	}
	s.due, s.ran = interval, false
	defer func() { s.due, s.ran = 0, false }()
	rep, err := s.Sweep(p, apply)
	if !s.ran && errors.Is(err, deskkit.ErrLockBusy) {
		return rep, false, nil
	}
	return rep, s.ran, err
}

// lastSweep reads the periodic maintenance stamp; a missing or damaged stamp reads as due.
func (s *Store) lastSweep() (time.Time, error) {
	b, err := s.read(maintenanceStampName, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, string(b))
}

// lastAttempt reads the failed-sweep record: when the last one ran and how many failed
// in a row. A missing or damaged record reads as no failure.
func (s *Store) lastAttempt() (time.Time, int, error) {
	b, err := s.read(maintenanceAttemptName, 64)
	if err != nil {
		return time.Time{}, 0, err
	}
	stamp, count, _ := strings.Cut(string(b), " ")
	n, err := strconv.Atoi(count)
	if err != nil || n < 1 {
		return time.Time{}, 0, errors.New("damaged maintenance attempt record")
	}
	at, err := time.Parse(time.RFC3339Nano, stamp)
	return at, n, err
}

// retryDelay is how long after the nth failed sweep in a row no supervisor may start
// another: MaintenanceRetry, doubled for each failure after the first, never above
// interval. The doubling saturates at interval before it can overflow, so no failure
// count or interval yields a zero, negative or spinning delay.
func retryDelay(n int, interval time.Duration) time.Duration {
	wait := MaintenanceRetry
	if wait <= 0 {
		return interval
	}
	for i := 1; i < n && wait < interval; i++ {
		if wait > interval/2 {
			return interval
		}
		wait *= 2
	}
	return min(wait, interval)
}

// Sweep holds a root lease and each candidate's execution lease across inspection and
// deletion. OpenRoot confines destructive operations to the scratch root, and
// removeConfined to the directories the sweep pinned while planning: a path that
// became a link, or names a different directory or file since, fails the removal.
// Age is retention policy, NEVER liveness proof. Missing child enrollment fails closed.
func (s *Store) Sweep(p Policy, apply bool) (rep Report, err error) {
	rep = Report{Apply: apply, Entries: []Entry{}}
	if p.MaxAge < 0 || p.MaxBytes < 0 {
		return rep, errors.New("negative scratch budget")
	}
	if s.scanLimit == 0 {
		limit, err := deskkit.MaintenanceEntryLimit()
		if err != nil {
			rep.Partial = true
			return rep, err
		}
		s.scanLimit = limit
		defer func() { s.scanLimit = 0 }()
	}
	// Maintenance serializes other maintenance, never admission. Task leases
	// protect candidates while recursive accounting runs outside the root lock.
	maintenance, err := s.lock(maintenanceLockName)
	if err != nil {
		return rep, err
	}
	defer maintenance.Close()
	if s.due > 0 {
		last, lastErr := s.lastSweep()
		if lastErr == nil && !last.After(time.Now()) && time.Since(last) < s.due {
			return rep, nil
		}
		// A failed attempt older than the last finished sweep is stale: the stamp
		// supersedes it even if its removal below did not complete.
		failures := 0
		if at, n, attemptErr := s.lastAttempt(); attemptErr == nil && !at.After(time.Now()) && (lastErr != nil || at.After(last)) {
			if time.Since(at) < retryDelay(n, s.due) {
				return rep, nil
			}
			failures = n
		}
		s.ran = true
		// The stamp records a finished sweep, written under the same lease. A failed
		// or partial sweep leaves the previous stamp and records the attempt instead,
		// so no supervisor sweeps again before the retry delay, which grows with each
		// failure in a row up to the interval, and siblings never retry all at once.
		// The supervisor that ran the failing sweep itself waits a whole interval;
		// a sibling whose own wait ends first runs the retry. A stopped sweep records nothing: stopping is not a
		// failure of the tree, and the next supervisor's check simply finds it due.
		defer func() {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if err == nil && !rep.Partial {
				if err = s.atomic(maintenanceStampName, []byte(now)); err == nil {
					_ = s.root.Remove(maintenanceAttemptName)
				}
			} else if s.stopped() == nil {
				err = errors.Join(err, s.atomic(maintenanceAttemptName, []byte(now+" "+strconv.Itoa(failures+1))))
			}
		}()
	}
	lock, err := s.sweepLock()
	if err != nil {
		return rep, err
	}
	f, err := s.root.Open(".")
	var entries []fs.DirEntry
	if err == nil {
		entries, err = deskkit.ReadDirBounded(f, s.inventoryLimit())
		f.Close()
	}
	lock.Release()
	if err != nil {
		rep.Partial = true
		return rep, err
	}
	type candidate struct {
		r          Record
		observed   Record
		entry      int
		size, work int64
		growth     int64
		lease      *os.File
		// build lists the disposable build output of an abandoned unacknowledged
		// task; such a candidate loses only these paths, never its record or tail.
		build []string
		// seen pins the identities planning observed at every path deletion opens.
		seen pins
	}
	var candidates []candidate
	var errs []error
	// unmeasured names each entry too large to walk inside the inventory bound. Such
	// an entry is kept, and the sweep reports partial, but it is contained: unlike
	// any other failed inspection it does not withdraw authority from the rest of the
	// plan, so one oversized task cannot stop every other task being reclaimed.
	var unmeasured []error
	for _, e := range entries {
		if e.Name() == "owner" || e.Name() == "preparation.json" || e.Name() == maintenanceLockName || strings.HasPrefix(e.Name(), maintenanceStampName) || e.Name() == sweepLockName || strings.HasPrefix(e.Name(), sweepLockName+".holder") {
			continue
		}
		// Identities are pinned before anything is measured, so deletion can prove it
		// opens the very directories this plan inspected.
		seen := pins{}
		pinErr := s.pin(seen, e.Name())
		size, allGit, sizeErr := s.measure(e.Name())
		// Only the bound running out on a pinned entry is contained; a pin failure,
		// an identity change or a stopped walk still fails the whole plan closed.
		overLimit := pinErr == nil && errors.Is(sizeErr, deskkit.ErrScanLimit)
		sizeErr = errors.Join(sizeErr, pinErr)
		// An over-limit entry's bytes are what the walk counted before it stopped: a
		// lower bound, so the budget arithmetic below can only under-reclaim.
		row := Entry{ID: e.Name(), Action: "keep", Bytes: size}
		rep.Bytes += size
		r, readErr := s.record(e.Name())
		if readErr != nil {
			row.Reason = "unverified ownership; inventory only"
			if overLimit {
				row.Reason = overLimitReason
				unmeasured = append(unmeasured, fmt.Errorf("%s: %w", e.Name(), sizeErr))
			} else if sizeErr != nil {
				row.Reason = "could-not-check unverified tree"
				errs = append(errs, sizeErr)
			}
		} else {
			lease, lockErr := s.lock(e.Name() + "/owner.lock")
			if lockErr != nil {
				row.Reason = "live or unavailable execution lease"
			} else {
				var work int64
				var git bool
				walkErr := s.pin(seen, r.ID+"/work")
				if r.State != "resumable" {
					if _, e := s.receipt(r.ID); e == nil && !allGit && sizeErr == nil {
						var measureErr error
						work, git, measureErr = s.measure(r.ID + "/work")
						walkErr = errors.Join(walkErr, measureErr)
					}
				}
				_, receiptErr := s.receipt(e.Name())
				// An abandoned task's Git data is judged per work entry by
				// abandonedBuild, so build output beside a worktree can go while
				// the worktree stays; every other task keeps its whole tree.
				abandoned := receiptErr != nil && r.State != "resumable"
				switch {
				case overLimit && walkErr == nil:
					row.Reason = overLimitReason
					unmeasured = append(unmeasured, fmt.Errorf("%s: %w", e.Name(), sizeErr))
				case sizeErr != nil || walkErr != nil:
					row.Reason = "could-not-check tree"
					errs = append(errs, errors.Join(sizeErr, walkErr))
				case (git || allGit) && !abandoned:
					row.Reason = "possible Git data present; retained for its owner"
				case r.State == "resumable":
					row.Reason = "resumable task"
				case receiptErr != nil:
					row.Reason = "pending evidence handoff"
					build, n, buildErr := s.abandonedBuild(r, p, seen)
					if errors.Is(buildErr, errNotRealDir) {
						row.Reason = "workspace is not a real directory; retained"
					} else if buildErr != nil {
						row.Reason = "could-not-check abandoned build output"
						errs = append(errs, buildErr)
					} else if len(build) > 0 {
						candidates = append(candidates, candidate{r: r, observed: r, entry: len(rep.Entries), size: size, work: n, lease: lease, build: build, seen: seen})
						rep.Entries = append(rep.Entries, row)
						continue
					}
				default:
					inactive := r.Reaped
					if !inactive && r.ChildGroup > 0 {
						inactive, err = groupEmpty(r.ChildGroup)
						if err != nil {
							row.Reason = "could-not-check child group"
						}
					}
					if !inactive {
						if row.Reason == "" {
							row.Reason = "child inactivity unproved"
						}
					} else {
						observed := r
						before, _ := json.Marshal(r)
						r.Reaped = true
						r.Evidence = "acknowledged"
						if r.State == "running" {
							r.State = "recovered"
							r.Finished = time.Now().UTC().Truncate(time.Second)
						}
						after, _ := json.Marshal(r)
						candidates = append(candidates, candidate{r: r, observed: observed, entry: len(rep.Entries), size: size, work: work, growth: int64(len(after) - len(before)), lease: lease, seen: seen})
						rep.Entries = append(rep.Entries, row)
						continue
					}
				}
				lease.Close()
			}
		}
		rep.Entries = append(rep.Entries, row)
	}
	defer func() {
		for _, c := range candidates {
			c.lease.Close()
		}
	}()
	// Incomplete accounting grants no reclamation authority, even for earlier
	// fully observed candidates. Their leases close through the defer above. The one
	// contained exception is an entry over the inventory bound (unmeasured): it is
	// itself kept and never a candidate, and the bytes it hides only lower the totals
	// the budget rules read, so the remaining candidates may still be reclaimed.
	if len(errs) > 0 {
		rep.Partial = true
		return rep, errors.Join(append(errs, unmeasured...)...)
	}
	// Oldest retained diagnostics leave first. Successful acknowledged tasks need no
	// local evidence; failed/recovered records retain exit status, receipt and bounded tail.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].r.Created.Before(candidates[j].r.Created) })
	remaining := rep.Bytes
	for _, c := range candidates {
		remaining -= c.work
		if c.build != nil {
			continue
		}
		if c.r.ExitCode != 0 {
			remaining += c.growth
		}
		if c.r.ExitCode == 0 {
			remaining -= c.size - c.work
		}
	}
	var growthWritten int64
	for _, c := range candidates {
		row := &rep.Entries[c.entry]
		build := c.build != nil
		entire := !build && (c.r.ExitCode == 0 || time.Since(c.r.Created) > p.MaxAge || remaining > p.MaxBytes)
		target := c.r.ID + "/work"
		planned := c.work
		row.Action = "compact"
		row.Reason = "acknowledged failure; retain bounded diagnostics"
		if build {
			row.Action = "reclaim-build"
			row.Reason = "abandoned unacknowledged task; disposable build output only"
		}
		if entire {
			target = c.r.ID
			planned = c.size
			row.Action = "remove"
			row.Reason = "acknowledged success or expired/over-budget diagnostics"
			if c.r.ExitCode != 0 {
				remaining -= c.size - c.work + c.growth
			}
		}
		if !apply {
			continue
		}
		if err = s.stopped(); err != nil {
			row.Action, row.Reason = "keep", "maintenance stopped"
			errs = append(errs, err)
			continue
		}
		// Revalidate Git retention under the task lease, outside admission's
		// root lock. No legitimate writer can acquire this task meanwhile. An
		// abandoned task is rechecked per planned entry, as it was planned; any
		// planned entry now holding Git data, or uninspectable, keeps the task.
		recheck := []string{c.r.ID}
		if build {
			recheck = c.build
		}
		var git bool
		var checkErr error
		for _, next := range recheck {
			_, found, e := s.measure(next)
			git, checkErr = git || found, errors.Join(checkErr, e)
		}
		if checkErr != nil || git {
			row.Action, row.Reason = "keep", "Git retention or unknown tree at deletion"
			if checkErr != nil {
				errs = append(errs, checkErr)
			}
			continue
		}
		lock, lockErr := s.sweepLock()
		if lockErr != nil {
			errs = append(errs, lockErr)
			row.Action = "error"
			row.Reason = lockErr.Error()
			continue
		}
		current, readErr := s.record(c.r.ID)
		_, receiptErr := s.receipt(c.r.ID)
		owner, ownerErr := s.read("owner", 128)
		// An acknowledged candidate needs its receipt; an abandoned build candidate
		// is admitted only while it still has none (an ack hands it to the rule above).
		acknowledged := receiptErr == nil
		if readErr != nil || current != c.observed || current.State == "resumable" || acknowledged == build || ownerErr != nil || string(owner) != s.owner {
			lock.Release()
			row.Action, row.Reason = "keep", "ownership or evidence changed during maintenance"
			continue
		}
		// Every directory above a deletion target must still be real: a task whose
		// work became a link to a sibling's work would otherwise delete the sibling's
		// files under this task's lease. Every target and every directory above it must
		// also still be the one planning pinned, so a sibling's real directory renamed
		// into place is not deleted either. Both are checked before any record write or
		// removal, so such a task is kept with a reason. The lock is released before
		// removal; removeConfined repeats the identity proof on the handles it deletes
		// through, so a link or rename after this check fails the removal instead of
		// redirecting it.
		targets := []string{c.r.ID + "/work"}
		if entire {
			targets = append(targets, target)
		}
		if build {
			targets = c.build
		}
		var linkErr error
		for _, next := range targets {
			linkErr = errors.Join(linkErr, s.realDir(filepath.Dir(next)))
		}
		if linkErr != nil {
			lock.Release()
			row.Action, row.Reason = "keep", "workspace is not a real directory at deletion"
			continue
		}
		var movedErr error
		for _, next := range targets {
			movedErr = errors.Join(movedErr, s.pin(c.seen, next))
		}
		if movedErr != nil {
			lock.Release()
			row.Action, row.Reason = "keep", "workspace changed since the sweep planned it"
			errs = append(errs, movedErr)
			continue
		}
		if !entire && !build {
			if err = s.save(c.r); err != nil {
				row.Action = "error"
				row.Reason = err.Error()
				errs = append(errs, err)
				lock.Release()
				continue
			}
			growthWritten += c.growth
		}
		lock.Release()
		// Remove the workspace before retiring its ownership record and receipt.
		// RemoveAll(task) may otherwise unlink record.json first and fail on a
		// workspace file, leaving a large tree that no later sweep may authorize.
		for _, next := range targets {
			if err = s.remove(next, c.seen); err != nil {
				break
			}
		}
		if err != nil {
			var left int64
			var measureErr error
			for _, rest := range c.build {
				n, _, e := s.measure(rest)
				left, measureErr = left+n, errors.Join(measureErr, e)
			}
			if !build {
				left, _, measureErr = s.measure(target)
			}
			if measureErr == nil {
				row.Reclaimed = planned - left
				if row.Reclaimed < 0 {
					row.Reclaimed = 0
				}
			}
			row.Action = "error"
			row.Reason = err.Error()
			errs = append(errs, err)
		} else {
			row.Reclaimed = planned
		}
		rep.Reclaimed += row.Reclaimed
	}
	errs = append(errs, unmeasured...)
	rep.Partial = len(errs) > 0
	if apply {
		rep.OverBudget = rep.Bytes+growthWritten-rep.Reclaimed > p.MaxBytes
	} else {
		rep.OverBudget = remaining > p.MaxBytes
	}
	return rep, errors.Join(errs...)
}
func (s *Store) measure(name string) (int64, bool, error) {
	s.walks++
	st, err := s.root.Lstat(name)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return 0, false, nil
	}
	var n int64
	git := false
	err = deskkit.BoundedWalkDir(s.root.FS(), name, s.inventoryLimit(), func(p string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) && p == name {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.stopped(); err != nil {
			return err
		}
		marked, err := s.gitMarker(p, d.Name())
		if err != nil {
			return err
		}
		if marked {
			git = true
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		if st.Mode().IsRegular() {
			n += st.Size()
		}
		return nil
	})
	return n, git, err
}

// abandonedBuild names the disposable build output of a task that never reached an
// evidence receipt and can no longer get one. The caller already holds the task's
// execution lease, so its owner is dead; the child group must be reaped or proved
// empty, and its last activity older than the retention age. Only entries directly
// under work/ whose name buildOutput accepts qualify, and each one only when nothing
// inside it is Git data: build output beside a worktree goes, the worktree and any
// build-named entry holding Git data stay. An entry that cannot be inspected fails
// the whole plan closed. Each entry's identity is pinned in seen before it is measured,
// and work's identity is verified against seen. The record, the output tail and every
// other work file stay for the owner's handoff.
func (s *Store) abandonedBuild(r Record, p Policy, seen pins) ([]string, int64, error) {
	last := r.Created
	if r.Finished.After(last) {
		last = r.Finished
	}
	if time.Since(last) <= p.MaxAge {
		return nil, 0, nil
	}
	quiet := r.Reaped
	if !quiet && r.ChildGroup > 0 {
		empty, err := groupEmpty(r.ChildGroup)
		quiet = err == nil && empty
	}
	if !quiet {
		return nil, 0, nil
	}
	// os.Root follows a link that stays inside the root: a work that is a link to a
	// sibling's work must not list, measure or plan the sibling's build output.
	err := s.realDir(r.ID + "/work")
	if os.IsNotExist(err) {
		return nil, 0, nil
	}
	if err == nil {
		err = s.pin(seen, r.ID+"/work")
	}
	if err != nil {
		return nil, 0, err
	}
	f, err := s.root.Open(r.ID + "/work")
	if err != nil {
		return nil, 0, err
	}
	entries, err := deskkit.ReadDirBounded(f, s.inventoryLimit())
	f.Close()
	if err != nil {
		return nil, 0, err
	}
	var build []string
	var total int64
	for _, e := range entries {
		if !buildOutput(e.Name()) || e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		if err := s.pin(seen, r.ID+"/work/"+e.Name()); err != nil {
			return nil, 0, err
		}
		n, git, err := s.measure(r.ID + "/work/" + e.Name())
		if err != nil {
			return nil, 0, err
		}
		if git {
			continue
		}
		build = append(build, r.ID+"/work/"+e.Name())
		total += n
	}
	return build, total, nil
}

// overLimitReason keeps an entry whose walk ran past the inventory bound, either the
// entry count or the fixed directory depth. Its size and Git content are unknown, so it
// is never reclaimed by this sweep. The remedy differs by limit, so both are named.
const overLimitReason = "over the maintenance inventory limit (entries or depth); not measured, retained (ASSAY_MAINTENANCE_ENTRY_LIMIT raises the entry bound; a tree past the depth limit needs owner-assisted layout correction)"

var errNotRealDir = errors.New("scratch path crosses a link or non-directory")

// realDir proves dir and every directory above it inside the scratch root is a real
// directory, one Lstat per component. os.Root confines a path to the root but follows
// a link that stays inside it, so without this one task's path can name a sibling's
// files. Abandoned-build planning and every deletion target's parent pass through it
// under the sweep lock; removeConfined repeats the proof on open handles after it,
// against the identities pin recorded.
func (s *Store) realDir(dir string) error {
	if dir == "." {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")
	for i := range parts {
		st, err := s.root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return err
		}
		if !st.IsDir() {
			return errNotRealDir
		}
	}
	return nil
}

// buildOutput is the closed set of compiler and binary output names an abandoned task
// may lose. Widening it needs its own retention decision. It is the Go toolchain's
// work directory, go-build plus decimal digits, and any <tool>-bin-<suffix> name with
// both parts non-empty, the shape of an os.MkdirTemp("", "<tool>-bin-") directory for
// built test or tool binaries. go-builder/, go-build-notes/, cellctl-bin-/ and
// -bin-x/ are ordinary work and stay. Git data inside a matching entry still keeps it.
func buildOutput(name string) bool {
	if digits, ok := strings.CutPrefix(name, "go-build"); ok && digits != "" && strings.Trim(digits, "0123456789") == "" {
		return true
	}
	tool, suffix, ok := strings.Cut(name, "-bin-")
	return ok && tool != "" && suffix != ""
}

// removeConfined is the single destruction boundary. It reaches target's parent one
// proved directory at a time from the scratch root (descend), each open handle
// required to be the directory seen pinned for it while planning, and removes the last
// component relative to that open directory, so it holds after the sweep lock is
// released. A component swapped for a link, or a different directory renamed onto a
// pinned name, fails the removal instead of redirecting it. The last component must
// be the file seen pinned: a directory is opened, proved by its handle and emptied
// through it, then removed only while empty; anything else, a link included, is
// unlinked itself, never followed. What the code does not prove: a file renamed onto
// the last name between its identity check and the unlink, or an empty directory
// renamed there before the final empty-only removal, is removed in its place, and
// entries moved into a pinned directory are emptied with it.
func (s *Store) removeConfined(target string, seen pins) error {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(target)), "/")
	want := make([]os.FileInfo, len(parts))
	for i := range parts {
		st, ok := seen[strings.Join(parts[:i+1], "/")]
		if !ok {
			return errors.New("removal target was not pinned while planning")
		}
		want[i] = st
	}
	r, err := s.root.OpenRoot(".")
	if err != nil {
		return err
	}
	if r, err = descend(r, parts[:len(parts)-1], want[:len(want)-1]); err != nil {
		return err
	}
	parent := r
	defer parent.Close()
	leaf, pinned := parts[len(parts)-1], want[len(want)-1]
	st, err := parent.Lstat(leaf)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if pinned == nil || !os.SameFile(pinned, st) {
		return errChanged
	}
	if !st.IsDir() {
		return parent.Remove(leaf)
	}
	if r, err = parent.OpenRoot("."); err != nil {
		return err
	}
	if r, err = descend(r, []string{leaf}, []os.FileInfo{pinned}); err != nil {
		return err
	}
	f, err := r.Open(".")
	var entries []fs.DirEntry
	if err == nil {
		entries, err = deskkit.ReadDirBounded(f, s.inventoryLimit())
		f.Close()
	}
	for _, e := range entries {
		err = errors.Join(err, r.RemoveAll(e.Name()))
	}
	r.Close()
	if err != nil {
		return err
	}
	return removeEmptyDir(parent, leaf)
}

// gitMarker is a conservative filesystem classifier, not repository admission.
// Partial/damaged repositories still contain required data. Do not run Git,
// follow metadata links or require config parsing to decide whether to retain it.
func (s *Store) gitMarker(path, name string) (bool, error) {
	if metadataPath(name) {
		return true, nil
	}
	if !strings.EqualFold(name, "HEAD") {
		return false, nil
	}
	// HEAD plus repository storage/reference markers identifies bare repositories
	// and detached Git directories independently of their enclosing directory name.
	for _, sibling := range []string{"objects", "refs", "packed-refs", "reftable", "commondir"} {
		_, err := s.root.Lstat(filepath.Join(filepath.Dir(path), sibling))
		if err == nil {
			return true, nil
		}
		if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

// Inventory is intentionally read-only and has no adoption or deletion flag. Legacy
// directories have no enrollment proof and cannot be promoted by age or a matching name.
func Inventory(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	limit, err := deskkit.MaintenanceEntryLimit()
	if err != nil {
		return nil, err
	}
	entries, err := deskkit.ReadDirBounded(f, limit)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, e := range entries {
		out = append(out, Entry{ID: e.Name(), Action: "keep", Reason: "legacy/unmarked; verified ownership required"})
	}
	return out, nil
}

func (s *Store) inventoryLimit() int {
	if s.scanLimit > 0 {
		return s.scanLimit
	}
	return deskkit.MaintenanceEntries
}
