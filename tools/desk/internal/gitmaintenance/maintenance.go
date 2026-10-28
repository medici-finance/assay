// Package gitmaintenance owns bounded, offline maintenance of one shared Git
// object store. It never expires reflogs or unreachable objects, fetches, runs gc,
// or changes branches. Native incremental maintenance retains every object ID,
// including objects needed only by an unknown reverse-alternate borrower.
package gitmaintenance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/gitcore"
)

const (
	DefaultInterval = 24 * time.Hour
	DefaultTimeout  = 2 * time.Minute
	MaxTimeout      = 15 * time.Minute
	lockName        = "assay-maintenance.lock"
	stampName       = "assay-maintenance.json"
	attemptName     = "assay-maintenance-attempt.json"
	activeName      = "assay-maintenance-active.json"
)

type Options struct {
	DryRun      bool          `json:"dry_run"`
	MinInterval time.Duration `json:"min_interval"`
	Timeout     time.Duration `json:"timeout"`
}

type ObjectCounts struct {
	Loose      int64 `json:"loose"`
	LooseBytes int64 `json:"loose_bytes"`
	Packed     int64 `json:"packed"`
	PackBytes  int64 `json:"pack_bytes"`
	Packs      int64 `json:"packs"`
	Garbage    int64 `json:"garbage"`
}

type Worktree struct {
	Path     string `json:"path"`
	HEAD     string `json:"head"`
	Branch   string `json:"branch,omitempty"`
	Locked   bool   `json:"locked"`
	Prunable bool   `json:"prunable"`
}

type Inventory struct {
	Repo           string            `json:"repo"`
	CommonDir      string            `json:"common_dir"`
	Objects        ObjectCounts      `json:"objects"`
	Refs           int               `json:"refs"`
	RefNamespaces  map[string]int    `json:"ref_namespaces"`
	DispatchRefs   []string          `json:"dispatch_refs"`
	Worktrees      []Worktree        `json:"worktrees"`
	Alternates     []string          `json:"alternates"`
	LastSuccess    *time.Time        `json:"last_success,omitempty"`
	LastAttempt    *time.Time        `json:"last_attempt,omitempty"`
	Recovery       string            `json:"recovery"`
	ReflogRecovery map[string]string `json:"reflog_recovery"`
	TemporaryRefs  []TempRefResult   `json:"temporary_refs"`
}

type Task struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
}

type Report struct {
	Before        Inventory       `json:"before"`
	After         *Inventory      `json:"after,omitempty"`
	Tasks         []Task          `json:"tasks"`
	Skipped       string          `json:"skipped,omitempty"`
	DryRun        bool            `json:"dry_run"`
	TemporaryRefs []TempRefResult `json:"temporary_refs"`
}

type successStamp struct {
	Version   int       `json:"version"`
	SuccessAt time.Time `json:"success_at"`
}

type attemptStamp struct {
	Version     int       `json:"version"`
	AttemptedAt time.Time `json:"attempted_at"`
}

// Inspect writes nothing: no lock, stamp, config, task registration or maintenance.
// Incomplete state is an error, never an empty inventory used to authorize work.
func Inspect(ctx context.Context, repo string) (Inventory, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	if err := CheckHooks(ctx, repo); err != nil {
		return Inventory{}, err
	}
	return inspect(ctx, repo)
}

// Run coalesces all linked worktrees by their canonical common directory. The OS
// lock lives through every child and the final stamp; a crashed process releases
// it without stale-lock deletion. Native Git also takes its own object/ref locks.
func Run(ctx context.Context, repo string, opts Options) (report Report, err error) {
	if opts.MinInterval < 0 || opts.Timeout < 0 {
		return report, errors.New("maintenance: intervals and timeouts must not be negative")
	}
	if opts.MinInterval == 0 {
		opts.MinInterval = DefaultInterval
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Timeout > MaxTimeout {
		return report, fmt.Errorf("maintenance: timeout exceeds %s", MaxTimeout)
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return report, err
	}
	report.DryRun = opts.DryRun
	if err := CheckHooks(ctx, repo); err != nil {
		return report, err
	}
	if opts.DryRun {
		report.Before, err = inspect(ctx, repo)
		if err != nil {
			return report, err
		}
		if pendingTempRefs(report.Before.TemporaryRefs) {
			report.Skipped = "prior temporary reference cleanup requires operator confirmation"
			return report, nil
		}
		if len(report.Before.Alternates) != 0 {
			report.Skipped = "borrowed object stores require a separate maintenance window"
			return report, nil
		}
		for _, task := range maintenanceTasks {
			report.Tasks = append(report.Tasks, Task{Name: task, Status: "planned"})
		}
		return report, nil
	}
	// Resolve/stamp preflight writes nothing. A malformed/future cadence stamp
	// cannot cause even a lock-file creation. Heavy inventory follows the lock,
	// so it cannot race another coordinator's temporary pack writes.
	report.Before.CommonDir, err = commonDir(repo)
	if err != nil {
		return report, err
	}
	if err := CheckCustody(ctx, repo); err != nil {
		return report, err
	}
	if _, err = readStamp(report.Before.CommonDir); err != nil {
		return report, err
	}
	if _, err = readAttempt(report.Before.CommonDir); err != nil {
		return report, err
	}
	if err = checkCommonHooks(ctx, report.Before.CommonDir); err != nil {
		return report, err
	}
	// Local object counts cannot bound history reachable through an alternate.
	// Refuse borrowed stores before creating even coordinator lock files, while
	// preserving their complete read-only inventory. Reverse borrowers do not
	// constrain a provider: its local inventory still bounds its own tasks.
	alternates, err := inspectAlternates(ctx, filepath.Join(report.Before.CommonDir, "objects"))
	if err != nil {
		return report, err
	}
	if len(alternates) != 0 {
		report.Before, err = inspect(ctx, repo)
		report.Skipped = "borrowed object stores require a separate maintenance window"
		return report, err
	}
	f, err := openLock(filepath.Join(report.Before.CommonDir, lockName))
	if err != nil {
		return report, err
	}
	defer f.Close()
	if err = deskkit.TryLockExclusive(f); err != nil {
		if errors.Is(err, deskkit.ErrLockBusy) {
			report.Skipped = "another coordinator is running"
			return report, nil
		}
		return report, fmt.Errorf("maintenance: lock could not be acquired: %w", err)
	}
	defer deskkit.UnlockFile(f)
	// Hold ref-child custody through all tasks. A standalone producer cannot
	// start a CAS and leave uncertain children after this run's preflight.
	tempLock, err := OpenLock(filepath.Join(report.Before.CommonDir, tempRefLockName))
	if err != nil {
		return report, err
	}
	defer tempLock.Close()
	if err = deskkit.TryLockExclusive(tempLock); err != nil {
		if errors.Is(err, deskkit.ErrLockBusy) {
			report.Skipped = "another temporary reference cleanup is running"
			return report, nil
		}
		return report, err
	}
	defer deskkit.UnlockFile(tempLock)
	ctx = context.WithValue(ctx, tempRefLockKey{}, tempRefLockCustody{Common: report.Before.CommonDir})
	// The inventory and cadence are re-read AFTER locking: concurrent callers
	// cannot both use a pre-lock absent/stale stamp to launch duplicate work.
	report.Before, err = inspect(ctx, repo)
	if err != nil {
		return report, err
	}
	if pendingTempRefs(report.Before.TemporaryRefs) {
		return report, errCleanupUnknown
	}
	if len(report.Before.Alternates) != 0 {
		report.Skipped = "borrowed object stores require a separate maintenance window"
		return report, nil
	}
	if report.Before.LastSuccess != nil && time.Since(*report.Before.LastSuccess) < opts.MinInterval {
		report.Skipped = "minimum interval has not elapsed"
		return report, nil
	}
	if report.Before.LastAttempt != nil && time.Since(*report.Before.LastAttempt) < opts.MinInterval {
		report.Skipped = "minimum attempt interval has not elapsed"
		return report, nil
	}
	// A task budget bounds time and memory, and caps the inventory on which a
	// reachable commit-graph walk operates. Large stores require an explicit
	// separate maintenance window rather than an unbounded background sweep.
	if report.Before.Objects.Loose+report.Before.Objects.Packed > 500000 || report.Before.Objects.PackBytes > 2<<30 {
		report.Skipped = "object store exceeds the bounded maintenance window"
		return report, nil
	}
	// Failed runs consume the cadence too: an automatic prune loop must not
	// repeat earlier expensive tasks whenever a later task fails. Record only
	// an eligible, fully inspected attempt, after fresh control-policy checks.
	if err = CheckHooks(ctx, repo); err != nil {
		return report, err
	}
	if err = checkCommonHooks(ctx, report.Before.CommonDir); err != nil {
		return report, err
	}
	if err = deskkit.Guard(); err != nil {
		return report, err
	}
	if err = writeAttempt(report.Before.CommonDir, time.Now().UTC()); err != nil {
		return report, err
	}
	// Establish durable custody before any mutation. The OS lock ends when a
	// coordinator crashes, but its children may survive; an outstanding marker
	// requires operator-confirmed cleanup, never an age/PID guess.
	if err = beginActive(report.Before.CommonDir); err != nil {
		return report, err
	}
	clean := true
	defer func() {
		if clean {
			err = errors.Join(err, os.Remove(filepath.Join(report.Before.CommonDir, activeName)))
		}
	}()
	// Receipt custody and compare-and-delete are the only ref retirement path.
	// Arbitrary old ref names, dispatch namespaces and live producer receipts
	// are inventory-only, regardless of their age or object reachability.
	if err := deskkit.Guard(); err != nil {
		return report, err
	}
	report.TemporaryRefs, err = RecoverTempRefs(ctx, repo, tempRefLimit)
	if err != nil {
		clean = !errors.Is(err, errCleanupUnknown)
		return report, err
	}
	if pendingTempRefs(report.TemporaryRefs) {
		clean = false
		return report, errCleanupUnknown
	}
	for _, name := range maintenanceTasks {
		if err := CheckHooks(ctx, repo); err != nil {
			return report, err
		}
		if err := deskkit.Guard(); err != nil {
			return report, err
		}
		start := time.Now()
		_, taskErr := native(ctx, report.Before.CommonDir, name)
		status := "completed"
		if taskErr != nil {
			status = "failed"
		}
		report.Tasks = append(report.Tasks, Task{Name: name, Status: status, Duration: time.Since(start)})
		if taskErr != nil {
			clean = !errors.Is(taskErr, errCleanupUnknown)
			return report, taskErr
		}
	}
	if err = os.Remove(filepath.Join(report.Before.CommonDir, activeName)); err != nil {
		clean = false
		return report, err
	}
	clean = false // custody already released after confirmed cleanup
	after, err := inspect(ctx, repo)
	if err != nil {
		return report, err
	}
	report.After = &after
	completed := time.Now().UTC()
	if err := writeStamp(report.Before.CommonDir, completed); err != nil {
		return report, err
	}
	report.After.LastSuccess = &completed
	return report, nil
}

func pendingTempRefs(rows []TempRefResult) bool {
	for _, row := range rows {
		if row.CleanupPending {
			return true
		}
	}
	return false
}

// CheckCustody is a read-only metadata gate for both maintenance coordinators.
// It never reads refs or objects, acquires locks, or launches a Git command.
func CheckCustody(ctx context.Context, repo string) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	common, err := commonDir(repo)
	if err != nil {
		return err
	}
	if err := checkActive(common); err != nil {
		return err
	}
	dir := filepath.Join(common, "assay-maintenance", "temporary-refs")
	f, err := openMaintenanceDirectory(dir)
	if os.IsNotExist(err) {
		// Missing receipt storage is normal; a broken symlink in its parent is
		// not proof of absence and must not conceal an old pending child.
		st, statErr := os.Lstat(filepath.Dir(dir))
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return errors.New("maintenance: receipt parent is not a real directory")
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if err := noReceiptSymlink(common); err != nil {
		return err
	}
	entries, err := f.ReadDir(tempRefLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) > tempRefLimit {
		return errors.New("maintenance: receipt custody exceeds inventory budget")
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".receipt-") {
			continue // unpublished atomic-write scratch cannot authorize a child
		}
		data, err := readMaintenanceFile(filepath.Join(dir, entry.Name()), 4096)
		if err != nil {
			return err
		}
		var receipt tempReceipt
		if json.Unmarshal(data, &receipt) != nil || !validTempReceipt(entry.Name(), receipt) {
			return errors.New("maintenance: receipt custody is unreadable or unregistered")
		}
		if receipt.CleanupPending {
			return errCleanupUnknown
		}
	}
	return nil
}

func beginActive(common string) error {
	f, err := os.OpenFile(filepath.Join(common, activeName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	data := struct {
		Version   int       `json:"version"`
		StartedAt time.Time `json:"started_at"`
	}{1, time.Now().UTC()}
	return errors.Join(json.NewEncoder(f).Encode(data), f.Sync(), f.Close())
}

func checkActive(common string) error {
	if _, err := os.Lstat(filepath.Join(common, activeName)); err == nil {
		return errors.New("maintenance: prior run process cleanup is unconfirmed; an operator must confirm its children have stopped before removing the active marker")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

var maintenanceTasks = []string{"pack-refs", "commit-graph", "loose-objects", "incremental-repack"}

func openLock(path string) (*os.File, error) {
	return OpenLock(path)
}

// OpenLock is the shared no-follow regular-file primitive used by index-only
// and object-store coordinators; both lock the same persistent inode.
func OpenLock(path string) (*os.File, error) { return openMaintenanceLock(path) }

func readStamp(common string) (*time.Time, error) {
	path := filepath.Join(common, stampName)
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Size() > 1024 {
		return nil, errors.New("maintenance: invalid success stamp file")
	}
	f, err := openMaintenanceRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1025))
	decoder.DisallowUnknownFields()
	var stamp successStamp
	if err := decoder.Decode(&stamp); err != nil {
		return nil, fmt.Errorf("maintenance: invalid success stamp: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("maintenance: trailing success stamp data")
	}
	if stamp.Version != 1 || stamp.SuccessAt.IsZero() || stamp.SuccessAt.After(time.Now()) {
		return nil, errors.New("maintenance: invalid or future success timestamp")
	}
	return &stamp.SuccessAt, nil
}

func writeStamp(common string, at time.Time) error {
	f, err := os.CreateTemp(common, ".assay-maintenance-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := json.NewEncoder(f).Encode(successStamp{Version: 1, SuccessAt: at}); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(common, stampName))
}

func readAttempt(common string) (*time.Time, error) {
	path := filepath.Join(common, attemptName)
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Size() > 1024 {
		return nil, errors.New("maintenance: invalid attempt stamp file")
	}
	f, err := openMaintenanceRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1025))
	decoder.DisallowUnknownFields()
	var stamp attemptStamp
	if err := decoder.Decode(&stamp); err != nil {
		return nil, fmt.Errorf("maintenance: invalid attempt stamp: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("maintenance: trailing attempt stamp data")
	}
	if stamp.Version != 1 || stamp.AttemptedAt.IsZero() || stamp.AttemptedAt.After(time.Now()) {
		return nil, errors.New("maintenance: invalid or future attempt timestamp")
	}
	return &stamp.AttemptedAt, nil
}

func writeAttempt(common string, at time.Time) error {
	f, err := os.CreateTemp(common, ".assay-maintenance-attempt-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := json.NewEncoder(f).Encode(attemptStamp{Version: 1, AttemptedAt: at}); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(common, attemptName))
}

func commonDir(repo string) (string, error) {
	common, err := gitcore.CommonDir(repo)
	if err != nil {
		return "", err
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return "", fmt.Errorf("maintenance: common directory is unreadable: %w", err)
	}
	return filepath.Abs(common)
}
