package drainloop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// FileClaim is a stand-in Claimer backed by exclusive-create claim files in a directory.
// One file per claimed ID; O_EXCL makes "create the claim" the atomic single-winner step,
// so two callers racing on the same ID cannot both win.
//
// It is deliberately simple. A production claim store would add stale-claim reclaim, a
// durable cross-machine lock, and an evidence probe (has this item already been done
// elsewhere?) before granting the claim. Those are the seams your infrastructure has opinions
// on: the reclaim/lock stays in your Claimer implementation; the evidence probe is offered
// here as the deskkit-free WorkEvidence hook (see evidence.go), so you can wire it without
// replacing FileClaim.
type FileClaim struct {
	Dir string
}

// NewFileClaim returns a FileClaim rooted at dir, creating the directory if needed.
func NewFileClaim(dir string) (*FileClaim, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileClaim{Dir: dir}, nil
}

func (f *FileClaim) path(id string) string {
	// Sanitise the ID into a single path segment so an ID containing a slash cannot escape
	// the claims dir or collide across separators.
	safe := strings.NewReplacer("/", "_", "\\", "_", "..", "__").Replace(id)
	return filepath.Join(f.Dir, safe+".claim")
}

// Claim atomically creates the claim file. os.O_EXCL is the single-winner primitive: the
// first caller creates it and holds the claim; a racing caller gets ErrExist and is told the
// item is already claimed. Any other error is could-not-check.
func (f *FileClaim) Claim(id string) (bool, error) {
	fh, err := os.OpenFile(f.path(id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil // already claimed — do not dispatch
		}
		return false, err // could-not-check — never "assume free"
	}
	_ = fh.Close()
	return true, nil
}

// Release removes the claim file. It is idempotent: releasing an unheld claim is not an
// error, so a double release cannot fail the drain.
func (f *FileClaim) Release(id string) error {
	err := os.Remove(f.path(id))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
