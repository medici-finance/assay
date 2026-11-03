package cellscratch

import (
	"encoding/hex"
	"encoding/json"
	"os"
)

// Prepare coalesces bounded shared initialization for one configuration and
// binary generation. This receipt records work only. It grants no lease,
// pressure allowance, role or permission; admission always checks those anew.
func (s *Store) Prepare(binding string) (bool, error) {
	digest, err := hex.DecodeString(binding)
	if err != nil || len(digest) != 32 {
		return false, os.ErrInvalid
	}
	lock, err := s.sweepLock()
	if err != nil {
		return false, err
	}
	defer lock.Release()
	want, _ := json.Marshal(struct{ Owner, Binding string }{s.owner, binding})
	before, err := s.read("preparation.json", 1024)
	if err == nil && string(before) == string(want) {
		return false, nil
	}
	// Corrupt and stale receipts cannot be reused. Re-running this bounded
	// initialization is safe; no authority was cached in the receipt.
	if err = s.atomic("preparation.json", want); err != nil {
		return false, err
	}
	return true, nil
}
