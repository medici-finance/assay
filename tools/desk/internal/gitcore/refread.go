package gitcore

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
)

// LocalRefHash reads a direct hash reference without dereferencing a symbolic
// alias; a symbolic reference is refused. The underlying reader can report an
// unopenable loose ref as missing, or malformed text as a zero hash. Callers
// retiring recovery state must validate hashes and independently prove absence.
func (r *Repo) LocalRefHash(name string) (string, bool, error) {
	ref, err := r.repo.Reference(plumbing.ReferenceName(name), false)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if ref.Type() != plumbing.HashReference {
		return "", true, fmt.Errorf("local ref %s is symbolic", name)
	}
	return ref.Hash().String(), true, nil
}
