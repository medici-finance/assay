package cellcache

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type Measurement struct {
	At             time.Time `json:"at"`
	LogicalBytes   *int64    `json:"logical_bytes"`
	AvailableBytes *uint64   `json:"available_bytes"`
}
type Report struct {
	Schema        string      `json:"schema"`
	Policy        Policy      `json:"policy"`
	Filesystem    string      `json:"filesystem"`
	Outcome       string      `json:"outcome"`
	Reason        string      `json:"reason,omitempty"`
	DryRun        bool        `json:"dry_run"`
	Before        Measurement `json:"before"`
	After         Measurement `json:"after"`
	SkippedActive []string    `json:"skipped_active"`
	Reclaimed     int64       `json:"reclaimed_logical_bytes"`
	// TrimmedBuild counts compiler-cache entries removed by the age trim, and
	// TrimmedBuildBytes their summed logical sizes. Unlike Reclaimed (a
	// before/after difference that concurrent builds can blur), these are
	// exactly what this pass deleted.
	TrimmedBuild      int      `json:"trimmed_build_entries"`
	TrimmedBuildBytes int64    `json:"trimmed_build_bytes"`
	Failures          []string `json:"failures"`
}
type manager struct {
	p    Policy
	r    *os.Root
	lock *deskkit.HeldLock
	dev  uint64
	// Instance-local seams make unavailable measurements/removal deterministic
	// in fixtures without mutating globals or touching host caches.
	free   func(string) (uint64, error)
	remove func(string) error
}

func open(p Policy, create bool) (*manager, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	created := false
	if create {
		// Only the final directory may be created: no adoption of arbitrary trees.
		if err := os.Mkdir(p.Root, 0700); err == nil {
			created = true
		} else if !os.IsExist(err) {
			return nil, err
		}
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(p.Root)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (*manager, error) { r.Close(); return nil, e }
	st, err := r.Stat(".")
	if err != nil {
		return fail(err)
	}
	dev, _, err := identity(st)
	if err != nil {
		return fail(err)
	}
	if created {
		f, e := r.OpenFile("owner", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return fail(e)
		}
		_, e = f.WriteString(schema + "\n" + p.Domain + "\n")
		e = errors.Join(e, f.Sync(), f.Close())
		if e != nil {
			return fail(e)
		}
	}
	st, err = r.Lstat("owner")
	if err != nil {
		return fail(fmt.Errorf("cache ownership not proven: %w", err))
	}
	if err = checkEntry(st, dev); err != nil || !st.Mode().IsRegular() || st.Size() > 1024 {
		return fail(fmt.Errorf("invalid cache owner file"))
	}
	b, err := r.ReadFile("owner")
	if err != nil || string(b) != schema+"\n"+p.Domain+"\n" {
		return fail(fmt.Errorf("cache root belongs to another trust domain or is unmarked"))
	}
	if st, e := r.Lstat("lock"); e == nil {
		if e = checkEntry(st, dev); e != nil || !st.Mode().IsRegular() {
			return fail(fmt.Errorf("invalid cache lock"))
		}
	} else if !os.IsNotExist(e) {
		return fail(e)
	}
	flags := os.O_RDWR | noFollow
	if create {
		flags |= os.O_CREATE
	}
	lock, err := r.OpenFile("lock", flags, 0600)
	if err != nil {
		return fail(err)
	}
	st, err = lock.Stat()
	if err == nil {
		err = checkEntry(st, dev)
	}
	if err != nil {
		lock.Close()
		return fail(err)
	}
	// Bounded wait, with backoff, for short measurement/cleanup transactions.
	// Busy is unknown, never permission to proceed without custody; a lasting
	// busy read names the lock and its holder and says it is retryable.
	held, err := deskkit.AcquireHeld(r, "lock", lock, deskkit.LockSpec{Name: "cell Go cache", Wait: LockWait, Log: os.Stderr})
	if err != nil {
		return fail(err)
	}
	m := &manager{p: p, r: r, lock: held, dev: dev, free: diskFree}
	m.remove = m.removeTree
	return m, nil
}
func (m *manager) close() { m.lock.Release(); m.r.Close() }

// LockWait bounds how long one cache transaction waits for a sibling's.
var LockWait = time.Minute

func (m *manager) measure() (Measurement, []string) {
	v := Measurement{At: time.Now().UTC()}
	var failures []string
	var total int64
	seen := map[[2]uint64]bool{}
	for _, name := range []string{"build", "mod"} {
		if _, err := m.r.Lstat(name); os.IsNotExist(err) {
			continue
		}
		err := fs.WalkDir(m.r.FS(), name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			st, err := m.r.Lstat(path)
			if err != nil {
				return err
			}
			if err = checkEntry(st, m.dev); err != nil {
				return err
			}
			dev, ino, err := identity(st)
			if err != nil {
				return err
			}
			key := [2]uint64{dev, ino}
			if !seen[key] && st.Mode().IsRegular() {
				total += st.Size()
				if total < 0 {
					return fmt.Errorf("cache size overflow")
				}
			}
			seen[key] = true
			return nil
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("measure %s: %v", name, err))
		}
	}
	if len(failures) == 0 {
		v.LogicalBytes = &total
	}
	free, err := m.free(m.p.Root)
	if err != nil {
		failures = append(failures, "available bytes: "+err.Error())
	} else {
		v.AvailableBytes = &free
	}
	return v, failures
}

func (m *manager) active() ([]string, error) {
	f, err := m.r.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	var active []string
	for _, d := range entries {
		if len(d.Name()) > 7 && d.Name()[:7] == "active-" {
			st, err := m.r.Lstat(d.Name())
			if err != nil {
				return nil, err
			}
			if err := checkEntry(st, m.dev); err != nil || !st.Mode().IsRegular() {
				return nil, fmt.Errorf("invalid active cache record")
			}
			active = append(active, d.Name())
		}
	}
	return active, nil
}
func pressure(v Measurement, p Policy) bool {
	return v.LogicalBytes != nil && v.AvailableBytes != nil && (*v.LogicalBytes > p.Budget || *v.AvailableBytes < p.Floor)
}
func (m *manager) check(dry bool) Report {
	return m.checkMode(dry, false)
}

func (m *manager) checkMode(dry, wipe bool) Report {
	r := Report{Schema: schema, Policy: m.p, Filesystem: filesystemName(m.dev), DryRun: dry, Outcome: "checked-clean"}
	r.Before, r.Failures = m.measure()
	var err error
	r.SkippedActive, err = m.active()
	if err != nil {
		r.Failures = append(r.Failures, "active custody: "+err.Error())
	}
	removed := false
	if len(r.Failures) == 0 && (wipe || pressure(r.Before, m.p)) && len(r.SkippedActive) > 0 && !dry {
		// Live consumers hold both roots, so only the age trim may act: it
		// takes compiler-cache entries no running build can have used within
		// the policy's age, and leaves mod/ and gopath/ to the records.
		n, bytes, err := m.trimBuild(time.Now().Add(-m.p.BuildAge()))
		r.TrimmedBuild, r.TrimmedBuildBytes = n, bytes
		removed = n > 0
		if err != nil {
			r.Failures = append(r.Failures, "trim build: "+err.Error())
		}
	}
	if len(r.Failures) == 0 && (wipe || pressure(r.Before, m.p)) && len(r.SkippedActive) == 0 && !dry {
		// Whole inactive roots only. Consumers are enrolled under this same lock;
		// no Go clean subprocess can race a build or module reader.
		for _, name := range []string{"build", "mod"} {
			if err := m.remove(name); err != nil {
				r.Failures = append(r.Failures, "remove "+name+": "+err.Error())
				break
			}
			removed = true
		}
	}
	var afterErrors []string
	r.After, afterErrors = m.measure()
	r.Failures = append(r.Failures, afterErrors...)
	if removed && r.Before.LogicalBytes != nil && r.After.LogicalBytes != nil && *r.Before.LogicalBytes > *r.After.LogicalBytes {
		r.Reclaimed = *r.Before.LogicalBytes - *r.After.LogicalBytes
	}
	if len(r.Failures) > 0 {
		r.Outcome = "could-not-check"
		r.Reason = "partial cache measurement or cleanup; inspect failures"
	} else if (wipe && len(r.SkippedActive) > 0) || pressure(r.After, m.p) {
		r.Outcome = "storage-deferred"
		r.Reason = fmt.Sprintf("cache uses %d logical bytes (budget %d); filesystem has %d available bytes (floor %d); active consumers %d", *r.After.LogicalBytes, m.p.Budget, *r.After.AvailableBytes, m.p.Floor, len(r.SkippedActive))
	}
	return r
}

// Wipe is explicit disposal, independent of the configured pressure threshold.
// The same custody lock excludes enrollment and protects live consumers.
func Wipe(p Policy, dry bool) (Report, error) {
	m, err := open(p, false)
	if err != nil {
		return Report{Schema: schema, Policy: p, Outcome: "could-not-check", Reason: err.Error(), DryRun: dry}, err
	}
	defer m.close()
	return finish(m, m.checkMode(dry, true))
}

// trimBuild removes compiler-cache entries whose mtime is before cutoff. It
// selects what cmd/go's own trimSubdir selects: names ending "-a" (action) or
// "-d" (output) inside the 256 two-hex-digit subdirectories of build/. A "-d"
// output is a regular file, or, for the executables `go run` and `go tool`
// cache, a directory; cmd/go removes an aged one of either shape, and so does
// this. Nothing else under build/ (README, trim.txt, a foreign file) and
// nothing outside it is ever selected. It is safe while consumers hold
// records because:
//   - cmd/go refreshes an entry's mtime when it looks up one stamped more than
//     an hour ago, so an entry a running build looked up is at most about an
//     hour old, and the policy age is at least MinBuildMaxAge;
//   - an entry missing at lookup is a cache miss that cmd/go rebuilds.
//
// Module directories have neither property (no use stamp, no refetch mid
// build), so mod/ is never trimmed.
//
// Custody: every directory and entry passes checkEntry, the discipline
// measurement uses. A directory entry is walked in full before anything in it
// is removed, and any foreign, linked, cross-device or other-writable member
// refuses the WHOLE directory. A refused entry is skipped, never followed,
// and the trim continues with the next one: each entry's verdict is its own,
// so skipping one cannot widen what another removal touches. Every refusal is
// returned in the error, so the pass still reports could-not-check. Removal
// goes through the verified root: Remove for a file (never follows a final
// symlink) and removeTree, the inactive-cleanup path, for a directory.
func (m *manager) trimBuild(cutoff time.Time) (int, int64, error) {
	const dir = "build"
	st, err := m.r.Lstat(dir)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if err = checkEntry(st, m.dev); err != nil || !st.IsDir() {
		return 0, 0, fmt.Errorf("invalid compiler cache directory")
	}
	n, total := 0, int64(0)
	var refused []error
	for i := 0; i < 256; i++ {
		sub := fmt.Sprintf("%s/%02x", dir, i)
		names, err := m.subdirNames(sub)
		if err != nil {
			refused = append(refused, err)
			continue
		}
		for _, name := range names {
			if !strings.HasSuffix(name, "-a") && !strings.HasSuffix(name, "-d") {
				continue
			}
			size, removed, err := m.trimEntry(sub+"/"+name, cutoff)
			if err != nil {
				refused = append(refused, err)
				continue
			}
			if removed {
				n++
				total += size
			}
		}
	}
	return n, total, errors.Join(refused...)
}

// subdirNames lists one verified build/<xx> subdirectory; a missing one is empty.
func (m *manager) subdirNames(sub string) ([]string, error) {
	st, err := m.r.Lstat(sub)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = checkEntry(st, m.dev); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("invalid compiler cache subdirectory %s", sub)
	}
	f, err := m.r.OpenFile(sub, os.O_RDONLY|noFollow, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

// trimEntry removes one selected entry if it is older than cutoff, returning
// the logical bytes it held. A concurrent removal (cmd/go's own trim) is not
// an error.
func (m *manager) trimEntry(path string, cutoff time.Time) (int64, bool, error) {
	st, err := m.r.Lstat(path)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if err = checkEntry(st, m.dev); err != nil || (!st.Mode().IsRegular() && !st.IsDir()) {
		return 0, false, fmt.Errorf("invalid compiler cache entry %s", path)
	}
	if !st.ModTime().Before(cutoff) {
		return 0, false, nil
	}
	if st.IsDir() {
		size, err := m.verifiedTreeSize(path)
		if err != nil {
			return 0, false, fmt.Errorf("refused compiler cache directory %s: %w", path, err)
		}
		if err = m.removeTree(path); err != nil {
			return 0, false, err
		}
		return size, true, nil
	}
	if err = m.r.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return st.Size(), true, nil
}

// verifiedTreeSize walks a directory entry through the verified root, applying
// checkEntry to every member, and sums its regular files. Any violation
// refuses the whole directory before a byte of it is removed.
func (m *manager) verifiedTreeSize(name string) (int64, error) {
	var total int64
	err := fs.WalkDir(m.r.FS(), name, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		st, err := m.r.Lstat(path)
		if err != nil {
			return err
		}
		if err = checkEntry(st, m.dev); err != nil {
			return err
		}
		if st.Mode().IsRegular() {
			total += st.Size()
		}
		return nil
	})
	return total, err
}

// Go module directories are read-only. Open each verified directory and chmod
// that descriptor, never a path-following chmod. Root prevents symlink escape;
// checkEntry refuses cross-device, foreign and linked material before deletion.
func (m *manager) removeTree(name string) error {
	if _, err := m.r.Lstat(name); os.IsNotExist(err) {
		return nil
	}
	err := fs.WalkDir(m.r.FS(), name, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		st, err := m.r.Lstat(path)
		if err != nil {
			return err
		}
		if err = checkEntry(st, m.dev); err != nil {
			return err
		}
		if st.IsDir() {
			f, err := m.r.OpenFile(path, os.O_RDONLY|noFollow, 0)
			if err != nil {
				return err
			}
			err = f.Chmod(st.Mode().Perm() | 0700)
			return errors.Join(err, f.Close())
		}
		return nil
	})
	if err != nil {
		return err
	}
	return m.r.RemoveAll(name)
}

func (m *manager) record(r Report) error {
	// Reports are bounded to the most recent two passes: disk control must not
	// introduce an unbounded measurement log. Rename makes each JSON complete.
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return err
	}
	name := "report-" + hex.EncodeToString(id) + ".tmp"
	f, err := m.r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|noFollow, 0600)
	if err != nil {
		return err
	}
	defer m.r.Remove(name)
	_, err = f.Write(append(b, '\n'))
	err = errors.Join(err, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	if err = m.r.Rename("report.json", "previous.json"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return m.r.Rename(name, "report.json")
}
func finish(m *manager, r Report) (Report, error) {
	if !r.DryRun {
		if err := m.record(r); err != nil {
			r.Outcome = "could-not-check"
			r.Reason = "could not persist storage measurement"
			r.Failures = append(r.Failures, err.Error())
		}
	}
	if r.Outcome != "checked-clean" {
		return r, &Deferred{r}
	}
	return r, nil
}

// Check measures and, unless dry, reclaims only under storage pressure. A dry
// run does not initialize a root or write a report (missing roots are unknown).
func Check(p Policy, dry bool) (Report, error) {
	m, err := open(p, !dry)
	if err != nil {
		now := time.Now().UTC()
		r := Report{Schema: schema, Policy: p, DryRun: dry, Outcome: "could-not-check", Reason: err.Error(), Failures: []string{err.Error()}, Before: Measurement{At: now}, After: Measurement{At: now}}
		return r, &Deferred{r}
	}
	defer m.close()
	return finish(m, m.check(dry))
}

// recordedPolicy re-reads the root's recorded policy through the verified
// root, under the root's lock, bound to the marker's trust domain and to this
// root's path. It is the under-lock half of CheckRecorded.
func (m *manager) recordedPolicy() (*Policy, error) {
	b, err := readMetaBounded(m.r, m.dev, "report.json", 64<<10)
	if err != nil {
		return nil, fmt.Errorf("no recorded policy readable under the root's lock: %v", err)
	}
	var rep Report
	if err := json.Unmarshal(b, &rep); err != nil || rep.Schema != schema {
		return nil, fmt.Errorf("cache report is not parseable or not in this schema")
	}
	p := rep.Policy
	if p.Domain != m.p.Domain {
		return nil, fmt.Errorf("recorded policy's trust domain does not match the root's ownership marker")
	}
	root, rerr := filepath.EvalSymlinks(m.p.Root)
	recorded, perr := filepath.EvalSymlinks(p.Root)
	if rerr != nil || perr != nil || filepath.Clean(root) != filepath.Clean(recorded) {
		return nil, fmt.Errorf("recorded policy names a different cache root")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// betweenLockAndReread is the test seam CheckRecorded calls between taking
// the root's lock and re-reading the recorded policy. See
// TestCheckRecordedInterleave.
var betweenLockAndReread = func() {}

// CheckRecorded is Check for a caller that DISCOVERED the root (a sweep
// walking a cells root) rather than resolved it from a cell's environment.
// The policy enforced is the one the root itself last recorded, re-read under
// the root's own lock in the same transaction as the check.
//
// Why the re-read is not optional: a caller that reads report.json first and
// checks second enforces a snapshot, and record() then writes the ENFORCED
// policy back — so a launch that recorded a new budget between the two reads
// is silently overwritten with the old one, and every later pass restores it
// again. Reading and enforcing in one transaction keeps the convergence the
// report provides: the newest recorded policy is the one applied, and the one
// written back.
//
// The root is never created: a sweep initializes nothing. A dry run still
// writes no report.
func CheckRecorded(root string, dry bool) (Report, error) {
	advisory, err := LoadMarkedPolicy(root)
	if err == nil && advisory == nil {
		err = fmt.Errorf("no marked cache root at %s", root)
	}
	if err != nil {
		now := time.Now().UTC()
		r := Report{Schema: schema, DryRun: dry, Outcome: "could-not-check", Reason: err.Error(), Failures: []string{err.Error()}, Before: Measurement{At: now}, After: Measurement{At: now}}
		return r, &Deferred{r}
	}
	m, err := open(*advisory, false)
	if err != nil {
		now := time.Now().UTC()
		r := Report{Schema: schema, Policy: *advisory, DryRun: dry, Outcome: "could-not-check", Reason: err.Error(), Failures: []string{err.Error()}, Before: Measurement{At: now}, After: Measurement{At: now}}
		return r, &Deferred{r}
	}
	defer m.close()
	// betweenLockAndReread is a test seam between the lock acquisition and the
	// under-lock policy re-read — the only window in which a launch can record
	// a newer policy that a stale enforcement would then write back over.
	// Production code never sets it.
	betweenLockAndReread()
	fresh, err := m.recordedPolicy()
	if err != nil {
		now := time.Now().UTC()
		r := Report{Schema: schema, Policy: *advisory, Filesystem: filesystemName(m.dev), DryRun: dry, Outcome: "could-not-check", Reason: err.Error(), Failures: []string{err.Error()}, Before: Measurement{At: now}, After: Measurement{At: now}}
		return r, &Deferred{r}
	}
	m.p = *fresh
	return finish(m, m.check(dry))
}

// Lease is durable custody, even if the supervisor crashes: while any record
// is held, mod/ and gopath/ are never reclaimed and no root is removed whole;
// only compiler-cache entries older than the policy's trim age may be trimmed
// under pressure (trimBuild). No PID or age is accepted as proof that
// consumers exited.
type Lease struct {
	policy Policy
	record string
}

func Acquire(env []string) (*Lease, error) {
	p, err := FromEnv(env)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	for _, want := range p.Env() {
		key, _, _ := strings.Cut(want, "=")
		count := 0
		for _, kv := range env {
			k, _, _ := strings.Cut(kv, "=")
			if k == key {
				count++
				if kv != want {
					return nil, fmt.Errorf("cache environment differs from policy: %s", key)
				}
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("cache environment needs exactly one %s", key)
		}
	}
	m, err := open(*p, true)
	if err != nil {
		return nil, err
	}
	defer m.close()
	if _, err = finish(m, m.check(false)); err != nil {
		return nil, err
	}
	for _, name := range []string{"build", "mod", "gopath"} {
		if err = m.r.MkdirAll(name, 0700); err != nil {
			return nil, err
		}
		st, err := m.r.Lstat(name)
		if err != nil {
			return nil, err
		}
		if err = checkEntry(st, m.dev); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("invalid cache directory %s", name)
		}
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return nil, err
	}
	name := "active-" + hex.EncodeToString(id)
	f, err := m.r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	_, err = f.WriteString(time.Now().UTC().Format(time.RFC3339Nano) + "\n")
	if err = errors.Join(err, f.Sync(), f.Close()); err != nil {
		return nil, err
	}
	return &Lease{policy: *p, record: name}, nil
}

// Finish releases only after the process supervisor proves its entire child
// tree has stopped. Uncertain completion deliberately retains the record.
func (l *Lease) Finish(clean bool) error {
	if l == nil || !clean {
		return nil
	}
	m, err := open(l.policy, false)
	if err != nil {
		return err
	}
	defer m.close()
	return m.r.Remove(l.record)
}

// Recover requires an operator's external proof that ALL cache consumers have
// stopped. It never kills a task. It clears crash custody, not cache contents.
func Recover(p Policy, confirmed bool) error {
	if !confirmed {
		return fmt.Errorf("cache recovery requires --confirm-stopped after checking all consumers")
	}
	m, err := open(p, false)
	if err != nil {
		return err
	}
	defer m.close()
	active, err := m.active()
	if err != nil {
		return err
	}
	for _, name := range active {
		if err = m.r.Remove(name); err != nil {
			return err
		}
	}
	return nil
}

// Paths are diagnostic only, never an invitation to adopt host cache locations.
func (p Policy) ReportPath() string { return filepath.Join(p.Root, "report.json") }
