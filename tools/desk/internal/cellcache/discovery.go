package cellcache

// discovery.go — what a SWEEP needs beyond Check: find marked roots without a
// cell.env in hand, and remove a root whose owning cell is provably gone.
//
// The package doc (policy.go) owns the trust model. The two additions here
// hold it the same way open() does: a root is only ever addressed through its
// ownership marker, the policy a sweep enforces is the one the root's own
// latest report recorded (never an invented or ambient one), and the lease
// set read under the root's lock at act time — never a snapshot taken earlier
// in a sweep — is the only authority on active consumers.
//
// The policy source is report.json, written by every live Check and by every
// consumer enrollment (Acquire). It is SELF-CONVERGING: an operator who
// changes CELL_GO_CACHE_BYTES gets the new budget enforced at the cell's next
// launch, which records it, and the next sweep enforces it. Between the change
// and that launch a sweep enforces the previously recorded policy — the
// safe direction in both cases: a lowered new budget is still enforced at
// admission, and enforcing a stale LOWER budget only ever reclaims disposable
// inactive cache earlier than the new policy requires.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DomainFor is the trust-domain derivation Resolve binds a cell's cache root
// to: the hash of the canonical cell directory. It is exported so a caller
// holding an ALREADY-canonical directory can ask "does this marked root belong
// to that cell" without re-walking the filesystem — the answer must be the
// same derivation Resolve used, or the comparison proves nothing.
func DomainFor(canonicalCellDir string) string {
	sum := sha256.Sum256([]byte(canonicalCellDir))
	return hex.EncodeToString(sum[:])
}

// betweenCheckAndOpen is the test seam readMetaBounded calls between the
// metadata entry's checks and its open — the window a swap to a FIFO must not
// be able to turn into a stalled read. See TestLoadMarkedPolicyBoundary's
// swapped-FIFO fixture. Production code never sets it.
var betweenCheckAndOpen = func(name string) {}

// readMetaBounded reads name relative to the verified root, under the
// package's one metadata discipline. os.Root deliberately resolves in-root
// links (O_NOFOLLOW is no discipline there), so the layers are:
//
//   - Lstat first: a symlink, FIFO or device at a metadata path is refused
//     before any open, so a planted FIFO cannot stall the read and a link
//     cannot redirect it;
//   - O_NONBLOCK on the open: a swap to a FIFO between the checks and the
//     open still cannot stall;
//   - the opened DESCRIPTOR's own fstat — never a path stat — proving a
//     same-device, singly-owned, not other-writable regular file, and the
//     descriptor must BE the entry the checks passed (device+inode), so a
//     swap between Lstat and open loses the race;
//   - a byte bound on the read, verified on both the entry and the stream.
func readMetaBounded(r *os.Root, dev uint64, name string, max int64) ([]byte, error) {
	lst, err := r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if err := checkEntry(lst, dev); err != nil || !lst.Mode().IsRegular() || lst.Size() > max {
		return nil, fmt.Errorf("invalid cache metadata file %s", name)
	}
	// A swap between the checks and the open is the window O_NONBLOCK exists
	// for; the seam lets a test put a swap there deterministically.
	betweenCheckAndOpen(name)
	f, err := r.OpenFile(name, os.O_RDONLY|noBlock, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fst, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if err := checkEntry(fst, dev); err != nil || !fst.Mode().IsRegular() {
		return nil, fmt.Errorf("invalid cache metadata file %s", name)
	}
	ldev, lino, err := identity(lst)
	if err != nil {
		return nil, err
	}
	fdev, fino, err := identity(fst)
	if err != nil {
		return nil, err
	}
	if ldev != fdev || lino != fino {
		return nil, fmt.Errorf("cache metadata file %s changed during the read", name)
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("oversized cache metadata file %s", name)
	}
	return b, nil
}

// LoadMarkedPolicy reads the policy a marked root runs under, for a caller
// that found the root by walking a cells root rather than by resolving a
// cell's environment. root is the candidate cache root itself (…/go-cache).
//
// (nil, nil) means NOT a marked root — no ownership marker, so the root is
// not this package's concern and the caller moves on silently. A marked root
// whose policy cannot be read or proven to belong to THIS root is an error:
// the caller reports it could-not-check rather than inventing a policy.
//
// The candidate's path discipline runs BEFORE any content read: a cells root
// can be shared or sticky, so a candidate directory can belong to another
// principal, and reading its "metadata" before the boundary check would let
// that candidate stall the reader (a FIFO at report.json) or flood it (a link
// to an endless source). The reads then go through readMetaBounded. These are
// still advisory pre-checks: enforcement goes through open(), which
// re-validates the marker, the domain and the path under the root's own lock.
func LoadMarkedPolicy(root string) (*Policy, error) {
	if err := validatePath(root); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer r.Close()
	rst, err := r.Stat(".")
	if err != nil {
		return nil, err
	}
	dev, _, err := identity(rst)
	if err != nil {
		return nil, err
	}
	b, err := readMetaBounded(r, dev, "owner", 1024)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) != 3 || lines[0] != schema || lines[2] != "" {
		return nil, fmt.Errorf("cache ownership marker is not in this schema")
	}
	domain := lines[1]

	rb, err := readMetaBounded(r, dev, "report.json", 64<<10)
	if err != nil {
		return nil, fmt.Errorf("marked cache root has no readable report of the policy it runs under: %v", err)
	}
	var rep Report
	if err := json.Unmarshal(rb, &rep); err != nil {
		return nil, fmt.Errorf("cache report is not parseable: %v", err)
	}
	if rep.Schema != schema {
		return nil, fmt.Errorf("cache report is not in this schema")
	}
	p := rep.Policy
	if p.Domain != domain {
		return nil, fmt.Errorf("recorded policy's trust domain does not match the root's ownership marker")
	}
	// The recorded root must name THIS root: a report copied or carried over
	// from another root must not redirect enforcement there (or pull this
	// root's marker onto another root's policy).
	discovered, derr := filepath.EvalSymlinks(root)
	recorded, rerr := filepath.EvalSymlinks(p.Root)
	if derr != nil || rerr != nil || filepath.Clean(discovered) != filepath.Clean(recorded) {
		return nil, fmt.Errorf("recorded policy names a different cache root")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Reap removes an ORPHANED marked root: one whose owning cell directory the
// caller has already removed under its own safety gates, so no launch can
// ever compose this root's policy again. That removal is the external proof
// Recover asks an operator for; Reap is the recovery's orphan half.
//
// The lease set is re-read under the root's own lock at act time, and a root
// with ANY active consumer record is refused — a sweep that learned "no
// consumers" earlier holds no proof now. A root that no longer exists is not
// an error: the common case is a root that lived INSIDE the removed cell
// directory and went with it, which is the best possible outcome.
//
// The removal runs while the root's lock is held, so a consumer racing the
// reap fails its bounded lock wait loudly instead of building into a tree
// that is halfway deleted. Go's read-only module directories are made
// writable first, through the same verified open descriptors as inactive
// cleanup.
func Reap(p Policy) (bool, error) {
	if _, err := os.Lstat(p.Root); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	m, err := open(p, false)
	if err != nil {
		return false, err
	}
	active, aerr := m.active()
	if aerr == nil && len(active) > 0 {
		aerr = fmt.Errorf("cache root has %d active consumer record(s); reaping an orphaned root requires none", len(active))
	}
	if aerr != nil {
		m.close()
		return false, aerr
	}
	for _, name := range []string{"build", "mod", "gopath"} {
		if err := m.removeTree(name); err != nil {
			m.close()
			return false, err
		}
	}
	err = os.RemoveAll(m.p.Root)
	m.close()
	if err != nil {
		return false, err
	}
	return true, nil
}
