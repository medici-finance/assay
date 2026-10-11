package gitmaintenance

// Temporary references are capabilities recorded by their producer, never guessed
// from a name or age. In particular dispatch claims and legacy deskmerge/pr-N
// references have no completion receipt and are not recovery candidates.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

const tempRefLimit = 256

type tempReceipt struct {
	Version        int    `json:"version"`
	Owner          string `json:"owner"`
	Ref            string `json:"ref"`
	Hash           string `json:"hash,omitempty"`
	Complete       bool   `json:"complete"`
	CleanupPending bool   `json:"cleanup_pending,omitempty"`
}

// TempRef is an operation's private fetch destination. Complete may be called
// only after the producer has stopped consuming the reference. A process crash
// before that declaration remains held, regardless of elapsed time.
type TempRef struct {
	Name    string
	root    string
	path    string
	receipt tempReceipt
	mu      sync.Mutex
}

// TempRefResult distinguishes planned/deleted references from live or unproven
// receipts. It is safe to embed in the maintenance JSON inventory.
type TempRefResult struct {
	Ref            string `json:"ref"`
	State          string `json:"state"`
	Reason         string `json:"reason"`
	CleanupPending bool   `json:"cleanup_pending,omitempty"`
}

const tempRefLockName = "assay-maintenance-temprefs.lock"

type tempRefLockKey struct{}
type tempRefLockCustody struct{ Common string }

// BeginTempRef reserves a unique destination before a producer fetches into it.
// The positive owner registry deliberately admits only the implemented producer.
func BeginTempRef(root, owner string) (*TempRef, error) {
	if owner != "deskmerge" {
		return nil, fmt.Errorf("temporary ref owner %q is not registered", owner)
	}
	common, err := gitcore.CommonDir(root)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(common, "assay-maintenance", "temporary-refs")
	if err := makeReceiptDir(common); err != nil {
		return nil, err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	key := hex.EncodeToString(id[:])
	r := tempReceipt{Version: 1, Owner: owner, Ref: "refs/deskmerge/tmp/" + key}
	t := &TempRef{Name: r.Ref, root: root, path: filepath.Join(dir, key+".json"), receipt: r}
	f, err := os.OpenFile(t.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	err = json.NewEncoder(f).Encode(r)
	if syncErr := f.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Observe binds cleanup to the exact fetched hash. A reference changed after this
// point is held by recovery and protected by the native compare-and-delete.
func (t *TempRef) Observe() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.receipt.Complete {
		return "", fmt.Errorf("completed temporary reference cannot be observed again")
	}
	r, err := gitcore.Open(t.root)
	if err != nil {
		return "", err
	}
	h, exists, err := r.LocalRefHash(t.Name)
	if err != nil {
		return "", err
	}
	if !exists || !validRefHash(h) {
		return "", fmt.Errorf("temporary fetch reference is absent or invalid")
	}
	t.receipt.Hash = h
	if err := t.save(); err != nil {
		return "", err
	}
	return h, nil
}

// Complete records the producer's completion before cleanup. If cleanup cannot
// prove safe, the completed receipt remains available to a later bounded sweep.
func (t *TempRef) Complete(ctx context.Context) (TempRefResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r, err := t.declareComplete(ctx)
	if err != nil {
		return TempRefResult{Ref: t.Name, State: "held", Reason: "producer completion could not be declared"}, err
	}
	return consumeTempReceipt(ctx, t.root, t.path, r, false)
}

// Only one opaque producer handle owns this UUID. Its first completion is a
// monotonic promotion while recovery still holds the incomplete receipt. Once
// complete or pending, no producer snapshot may overwrite durable CAS custody.
func (t *TempRef) declareComplete(ctx context.Context) (tempReceipt, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return tempReceipt{}, err
	}
	if err := deskkit.Guard(); err != nil {
		return tempReceipt{}, err
	}
	common, err := commonDir(t.root)
	if err != nil {
		return tempReceipt{}, err
	}
	if err := noReceiptSymlink(common); err != nil {
		return tempReceipt{}, err
	}
	body, err := readMaintenanceFile(t.path, 4096)
	if os.IsNotExist(err) {
		t.receipt.Complete = true
		return t.receipt, nil // an earlier cleanup already drained it
	}
	if err != nil {
		return tempReceipt{}, err
	}
	var fresh tempReceipt
	if json.Unmarshal(body, &fresh) != nil || !validTempReceipt(filepath.Base(t.path), fresh) || fresh.Ref != t.Name {
		return tempReceipt{}, fmt.Errorf("temporary completion receipt changed or is invalid")
	}
	if fresh.Hash != t.receipt.Hash {
		return tempReceipt{}, fmt.Errorf("temporary completion receipt observation changed")
	}
	t.receipt.Complete = true
	if fresh.Complete || fresh.CleanupPending {
		return fresh, nil
	}
	fresh.Complete = true
	if err := saveTempReceipt(t.path, fresh); err != nil {
		return tempReceipt{}, err
	}
	return fresh, nil
}

func (t *TempRef) save() error {
	f, err := os.CreateTemp(filepath.Dir(t.path), ".receipt-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	err = json.NewEncoder(f).Encode(t.receipt)
	if syncErr := f.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, t.path)
}

// InspectTempRefs is bounded and read-only; active/unknown receipts never become
// eligible solely through age. Legacy refs without receipts are inventoried by
// the repository's general reference inventory, not adopted here.
func InspectTempRefs(ctx context.Context, root string) ([]TempRefResult, error) {
	return walkTempReceipts(ctx, root, tempRefLimit, true)
}

// RecoverTempRefs consumes at most limit receipts (capped at 256). Only producer
// completion, registered ownership, no holder, and exact hash permit deletion.
func RecoverTempRefs(ctx context.Context, root string, limit int) ([]TempRefResult, error) {
	if limit <= 0 || limit > tempRefLimit {
		return nil, fmt.Errorf("temporary ref limit must be between 1 and %d", tempRefLimit)
	}
	return walkTempReceipts(ctx, root, limit, false)
}

func walkTempReceipts(ctx context.Context, root string, limit int, dryRun bool) ([]TempRefResult, error) {
	common, err := gitcore.CommonDir(root)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(common, "assay-maintenance", "temporary-refs")
	f, err := openMaintenanceDirectory(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := noReceiptSymlink(common); err != nil {
		return nil, err
	}
	entries, err := f.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > limit {
		return nil, fmt.Errorf("temporary receipt directory exceeds inventory budget %d", limit)
	}
	var results []TempRefResult
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if strings.HasPrefix(entry.Name(), ".receipt-") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		row := TempRefResult{Ref: entry.Name(), State: "held", Reason: "unreadable or unregistered completion receipt"}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			results = append(results, row)
			continue
		}
		file, err := openMaintenanceRegular(path)
		if err != nil {
			results = append(results, row)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(file, 4097))
		closeErr := file.Close()
		var receipt tempReceipt
		if err != nil || closeErr != nil || len(body) > 4096 || json.Unmarshal(body, &receipt) != nil || !validTempReceipt(entry.Name(), receipt) {
			results = append(results, row)
			continue
		}
		row, err = cleanTempReceipt(ctx, root, path, receipt, dryRun)
		results = append(results, row)
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

func validTempReceipt(filename string, r tempReceipt) bool {
	key := strings.TrimSuffix(filename, ".json")
	id, err := hex.DecodeString(key)
	return err == nil && len(id) == 16 && filename == key+".json" && r.Version == 1 && r.Owner == "deskmerge" && r.Ref == "refs/deskmerge/tmp/"+key && (r.Hash == "" || validRefHash(r.Hash))
}

func validRefHash(hash string) bool {
	h := plumbing.NewHash(hash)
	return !h.IsZero() && h.String() == hash
}

func cleanTempReceipt(ctx context.Context, root, path string, r tempReceipt, dryRun bool) (TempRefResult, error) {
	return consumeTempReceipt(ctx, root, path, r, dryRun)
}

// One persistent lock serializes all receipt state transitions and CAS children.
// Run takes the maintenance lock first; standalone producers take only this one.
// No per-operation lock inode accumulates in the common directory.
func consumeTempReceipt(ctx context.Context, root, path string, r tempReceipt, dryRun bool) (TempRefResult, error) {
	if dryRun {
		return cleanTempReceiptLocked(ctx, root, path, r, true)
	}
	row := TempRefResult{Ref: r.Ref, State: "held", CleanupPending: r.CleanupPending}
	if err := ctx.Err(); err != nil {
		return row, err
	}
	if err := deskkit.Guard(); err != nil {
		return row, err
	}
	common, err := commonDir(root)
	if err != nil {
		return row, err
	}
	if err := noReceiptSymlink(common); err != nil {
		return row, err
	}
	owned, _ := ctx.Value(tempRefLockKey{}).(tempRefLockCustody)
	if owned.Common != common {
		f, err := OpenLock(filepath.Join(common, tempRefLockName))
		if err != nil {
			return row, err
		}
		defer f.Close()
		if err := deskkit.TryLockExclusive(f); err != nil {
			if errors.Is(err, deskkit.ErrLockBusy) {
				row.Reason = "another temporary reference cleanup is running; completed receipt retained"
				return row, nil
			}
			return row, err
		}
		defer deskkit.UnlockFile(f)
	}
	if err := ctx.Err(); err != nil {
		return row, err
	}
	// Never trust the caller's pre-lock snapshot: another producer or recovery
	// may have completed, drained, or left durable uncertain custody meanwhile.
	body, err := readMaintenanceFile(path, 4096)
	if os.IsNotExist(err) {
		row.State, row.Reason = "absent", "completion receipt already drained"
		return row, nil
	}
	if err != nil {
		return row, err
	}
	var fresh tempReceipt
	if json.Unmarshal(body, &fresh) != nil || !validTempReceipt(filepath.Base(path), fresh) || fresh.Ref != r.Ref {
		return row, fmt.Errorf("temporary completion receipt changed or is invalid")
	}
	if fresh.CleanupPending {
		row.CleanupPending = true
		row.Reason = "prior cleanup process custody unconfirmed; operator confirmation required"
		return row, nil
	}
	return cleanTempReceiptLocked(ctx, root, path, fresh, false)
}

func saveTempReceipt(path string, r tempReceipt) error {
	t := &TempRef{path: path, receipt: r}
	return t.save()
}

func cleanTempReceiptLocked(ctx context.Context, root, path string, r tempReceipt, dryRun bool) (TempRefResult, error) {
	row := TempRefResult{Ref: r.Ref, State: "held", CleanupPending: r.CleanupPending}
	if r.CleanupPending {
		row.Reason = "prior cleanup process custody unconfirmed; operator confirmation required"
		return row, nil
	}
	if !r.Complete {
		row.Reason = "producer has not declared completion"
		return row, nil
	}
	gr, err := gitcore.Open(root)
	if err != nil {
		row.Reason = "could-not-check: repository unreadable"
		return row, err
	}
	hash, exists, err := gr.LocalRefHash(r.Ref)
	if err != nil {
		row.Reason = "could-not-check: reference is unreadable or symbolic"
		return row, err
	}
	if !exists {
		row.State = "absent"
		row.Reason = "completed operation has no remaining reference"
		if !dryRun {
			return row, removeReceipt(path)
		}
		return row, nil
	}
	if r.Hash == "" || hash != r.Hash {
		row.Reason = "fetched hash absent or reference changed after observation"
		return row, nil
	}
	common, err := gitcore.CommonDir(root)
	if err != nil {
		row.Reason = "could-not-check: shared directory unreadable"
		return row, err
	}
	held, err := tempRefHeld(common, gr, r)
	if err != nil {
		row.Reason = "could-not-check: worktree holders unreadable"
		return row, err
	}
	if held {
		row.Reason = "reference or object is held by a worktree HEAD"
		return row, nil
	}
	row.State = "planned"
	row.Reason = "registered producer completed; exact hash; no worktree holder"
	if dryRun {
		return row, nil
	}
	if err := CheckHooks(ctx, root); err != nil {
		row.State = "held"
		row.Reason = "ref transaction hook policy refused cleanup"
		return row, err
	}
	if err := deskkit.Guard(); err != nil {
		row.State = "held"
		row.Reason = "runtime guard refused cleanup"
		return row, err
	}
	// Publish uncertain custody BEFORE starting a child. A coordinator crash or
	// unknown tree cleanup leaves this receipt permanently held, never aged out.
	r.CleanupPending = true
	if err := saveTempReceipt(path, r); err != nil {
		return row, err
	}
	if err := deleteTempRef(ctx, root, r.Ref, r.Hash); err != nil {
		row.State = "held"
		// A successful-looking ref state does not prove a canceled process tree
		// stopped. Preserve its receipt and coordinator custody before any reread.
		if errors.Is(err, errCleanupUnknown) {
			row.CleanupPending = true
			row.State = "held"
			row.Reason = "compare-and-delete process cleanup could not be confirmed"
			return row, err
		}
		// Every ordinary failure confirms that the child tree has stopped. Clear
		// the pending claim durably before any absent-ref fallback or next retry.
		r.CleanupPending = false
		if saveErr := saveTempReceipt(path, r); saveErr != nil {
			return row, errors.Join(err, saveErr)
		}
		if errors.Is(err, ErrRefDiagnostic) {
			row.State = "held"
			row.Reason = "compare-and-delete emitted an uncertain diagnostic"
			return row, err
		}
		// Producer completion and maintenance recovery can meet on the same
		// receipt. An already-absent destination is a drained operation, while
		// every changed or unreadable reference remains held.
		fresh, openErr := gitcore.Open(root)
		if openErr == nil {
			_, exists, readErr := fresh.LocalRefHash(r.Ref)
			if readErr == nil && !exists {
				row.State = "absent"
				return row, removeReceipt(path)
			}
		}
		row.State = "held"
		row.Reason = "compare-and-delete refused; reference may have moved"
		return row, err
	}
	row.State = "deleted"
	return row, removeReceipt(path)
}

func removeReceipt(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func noReceiptSymlink(common string) error {
	for _, suffix := range []string{"assay-maintenance", "assay-maintenance/temporary-refs"} {
		st, err := os.Lstat(filepath.Join(common, filepath.FromSlash(suffix)))
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return fmt.Errorf("temporary receipt directory is not a real directory")
		}
	}
	return nil
}

func makeReceiptDir(common string) error {
	for _, suffix := range []string{"assay-maintenance", "assay-maintenance/temporary-refs"} {
		path := filepath.Join(common, filepath.FromSlash(suffix))
		if err := os.Mkdir(path, 0o700); err != nil && !os.IsExist(err) {
			return err
		}
		st, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return fmt.Errorf("temporary receipt directory is not a real directory")
		}
	}
	return nil
}

func tempRefHeld(common string, gr *gitcore.Repo, r tempReceipt) (bool, error) {
	heads := []string{filepath.Join(common, "HEAD")}
	entries, err := readWorktreeEntries(common)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("unreadable worktree registration")
		}
		heads = append(heads, filepath.Join(common, "worktrees", entry.Name(), "HEAD"))
	}
	for _, path := range heads {
		body, err := readMaintenanceFile(path, 1<<20)
		if err != nil {
			return false, err
		}
		head := strings.TrimSpace(string(body))
		if name, ok := strings.CutPrefix(head, "ref: "); ok {
			if name == r.Ref {
				return true, nil
			}
			h, err := gr.Resolve(name)
			if err != nil {
				return false, err
			}
			head = h.String()
		}
		if !validRefHash(head) {
			return false, fmt.Errorf("worktree HEAD is not a full object id")
		}
		if head == r.Hash {
			return true, nil
		}
	}
	return false, nil
}
