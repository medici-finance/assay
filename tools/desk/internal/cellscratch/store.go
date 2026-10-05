// Package cellscratch is the sole owner of disposable task scratch, never Git worktrees.
package cellscratch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	// Tests inject deletion failure at this single destruction boundary.
	remove func(string) error
}

func Open(path string) (*Store, error) {
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
	s := &Store{root: r, Path: path}
	s.remove = r.RemoveAll
	lock, err := s.lock("sweep.lock")
	if err != nil {
		r.Close()
		return nil, err
	}
	defer lock.Close()
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

func (s *Store) Close() error { return s.root.Close() }
func (s *Store) lock(name string) (*os.File, error) {
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
	if err = deskkit.TryLockExclusive(f); err != nil {
		f.Close()
		return nil, err
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
	if !strings.HasPrefix(id, "task-") || len(id) != 21 {
		return false
	}
	_, err := hex.DecodeString(id[5:])
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
	lock, err := s.lock("sweep.lock")
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	nonce := make([]byte, 8)
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	id := "task-" + hex.EncodeToString(nonce)
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
	lock, err := s.lock("sweep.lock")
	if err != nil {
		return err
	}
	defer lock.Close()
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

// Sweep holds a root lease and each candidate's execution lease across inspection and
// deletion. OpenRoot confines destructive operations even if a path becomes a symlink.
// Age is retention policy, NEVER liveness proof. Missing child enrollment fails closed.
func (s *Store) Sweep(p Policy, apply bool) (Report, error) {
	rep := Report{Apply: apply, Entries: []Entry{}}
	if p.MaxAge < 0 || p.MaxBytes < 0 {
		return rep, errors.New("negative scratch budget")
	}
	lock, err := s.lock("sweep.lock")
	if err != nil {
		return rep, err
	}
	defer lock.Close()
	entries, err := fs.ReadDir(s.root.FS(), ".")
	if err != nil {
		return rep, err
	}
	type candidate struct {
		r          Record
		entry      int
		size, work int64
		growth     int64
		lease      *os.File
	}
	var candidates []candidate
	var errs []error
	for _, e := range entries {
		if e.Name() == "owner" || e.Name() == "sweep.lock" {
			continue
		}
		size, allGit, sizeErr := s.measure(e.Name())
		row := Entry{ID: e.Name(), Action: "keep", Bytes: size}
		rep.Bytes += size
		r, readErr := s.record(e.Name())
		if readErr != nil {
			row.Reason = "unverified ownership; inventory only"
			if sizeErr != nil {
				row.Reason = "could-not-check unverified tree"
				errs = append(errs, sizeErr)
			}
		} else {
			lease, lockErr := s.lock(e.Name() + "/owner.lock")
			if lockErr != nil {
				row.Reason = "live or unavailable execution lease"
			} else {
				work, git, walkErr := s.measure(e.Name() + "/work")
				_, receiptErr := s.receipt(e.Name())
				switch {
				case sizeErr != nil || walkErr != nil:
					row.Reason = "could-not-check tree"
					errs = append(errs, errors.Join(sizeErr, walkErr))
				case git || allGit:
					row.Reason = "Git checkout/worktree present; deskwt owns pruning"
				case r.State == "resumable":
					row.Reason = "resumable task"
				case receiptErr != nil:
					row.Reason = "pending evidence handoff"
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
						before, _ := json.Marshal(r)
						r.Reaped = true
						r.Evidence = "acknowledged"
						if r.State == "running" {
							r.State = "recovered"
							r.Finished = time.Now().UTC().Truncate(time.Second)
						}
						after, _ := json.Marshal(r)
						candidates = append(candidates, candidate{r: r, entry: len(rep.Entries), size: size, work: work, growth: int64(len(after) - len(before)), lease: lease})
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
	// Oldest retained diagnostics leave first. Successful acknowledged tasks need no
	// local evidence; failed/recovered records retain exit status, receipt and bounded tail.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].r.Created.Before(candidates[j].r.Created) })
	remaining := rep.Bytes
	for _, c := range candidates {
		remaining -= c.work
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
		entire := c.r.ExitCode == 0 || time.Since(c.r.Created) > p.MaxAge || remaining > p.MaxBytes
		target := c.r.ID + "/work"
		planned := c.work
		row.Action = "compact"
		row.Reason = "acknowledged failure; retain bounded diagnostics"
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
		if !entire {
			if err = s.save(c.r); err != nil {
				row.Action = "error"
				row.Reason = err.Error()
				errs = append(errs, err)
				continue
			}
			growthWritten += c.growth
		}
		if err = s.remove(target); err != nil {
			left, _, measureErr := s.measure(target)
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
	rep.Partial = len(errs) > 0
	if apply {
		rep.OverBudget = rep.Bytes+growthWritten-rep.Reclaimed > p.MaxBytes
	} else {
		rep.OverBudget = remaining > p.MaxBytes
	}
	return rep, errors.Join(errs...)
}
func (s *Store) measure(name string) (int64, bool, error) {
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
	err = fs.WalkDir(s.root.FS(), name, func(p string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) && p == name {
			return nil
		}
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
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

// Inventory is intentionally read-only and has no adoption or deletion flag. Legacy
// directories have no enrollment proof and cannot be promoted by age or a matching name.
func Inventory(path string) ([]Entry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, e := range entries {
		out = append(out, Entry{ID: e.Name(), Action: "keep", Reason: "legacy/unmarked; verified ownership required"})
	}
	return out, nil
}
