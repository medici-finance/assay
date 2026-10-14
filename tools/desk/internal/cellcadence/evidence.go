package cellcadence

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Holder reads the role lease's owner record without taking the lease. Taking
// the lease (even to probe it) rewrites that record, so read it first.
func Holder(dir string) (*deskkit.LockHolder, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return deskkit.ReadLockHolder(root, "owner.lock")
}

// Probe reports whether a live process holds the lease right now. Unlike
// Acquire it neither creates the lock file nor writes or removes an owner
// record, so the evidence a crashed owner left behind survives the probe.
func Probe(dir string) (busy bool, err error) {
	p := filepath.Join(dir, "owner.lock")
	st, err := os.Lstat(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !st.Mode().IsRegular() {
		return false, errors.New("cadence lock is not a regular file")
	}
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if err := deskkit.TryLockExclusive(f); err != nil {
		if errors.Is(err, deskkit.ErrLockBusy) {
			return true, nil
		}
		return false, err
	}
	return false, deskkit.UnlockFile(f)
}
