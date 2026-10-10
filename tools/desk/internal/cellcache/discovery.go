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

// LoadMarkedPolicy reads the policy a marked root runs under, for a caller
// that found the root by walking a cells root rather than by resolving a
// cell's environment. root is the candidate cache root itself (…/go-cache).
//
// (nil, nil) means NOT a marked root — no ownership marker, so the root is
// not this package's concern and the caller moves on silently. A marked root
// whose policy cannot be read or proven to belong to THIS root is an error:
// the caller reports it could-not-check rather than inventing a policy.
//
// The reads here are advisory pre-checks. Enforcement goes through open(),
// which re-validates the marker, the domain and the path under the root's own
// lock; a race against these reads fails closed there, never opens.
func LoadMarkedPolicy(root string) (*Policy, error) {
	ownerPath := filepath.Join(root, "owner")
	st, err := os.Lstat(ownerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > 1024 {
		return nil, fmt.Errorf("invalid cache owner file")
	}
	b, err := os.ReadFile(ownerPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) != 3 || lines[0] != schema || lines[2] != "" {
		return nil, fmt.Errorf("cache ownership marker is not in this schema")
	}
	domain := lines[1]

	rb, err := os.ReadFile(filepath.Join(root, "report.json"))
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
