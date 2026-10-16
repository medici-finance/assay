package cellop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ErrAlreadyConsumed is returned by Consume when the plan's consumed marker already exists.
var ErrAlreadyConsumed = errors.New("plan already consumed")

// ErrNotFound is returned by Load when no plan with the id is stored.
var ErrNotFound = errors.New("plan not found")

// Store is one cell's plan store: <cell dir>/run/plans/<id>.json, mode 0600 in a 0700 directory,
// and the consumed markers under run/plans/consumed/<id>.
type Store struct{ Dir string }

// StoreFor is the plan store of the cell whose directory is cellDir.
func StoreFor(cellDir string) Store { return Store{Dir: filepath.Join(cellDir, "run", "plans")} }

// Path is where the plan with id is stored.
func (s Store) Path(id string) string { return filepath.Join(s.Dir, id+".json") }

// MarkerPath is the consumed marker of the plan with id.
func (s Store) MarkerPath(id string) string { return filepath.Join(s.Dir, "consumed", id) }

func checkID(id string) error {
	if !idRE.MatchString(id) {
		return fmt.Errorf("%w: plan id %q is not 24 lowercase hex characters", ErrInvalid, id)
	}
	return nil
}

// Save writes a verified plan, mode 0600, by exclusive creation: a stored plan is never
// overwritten, so the bytes an operator confirmed are the bytes that were planned.
func (s Store) Save(p Plan) (string, error) {
	if err := p.Verify(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	path := s.Path(p.ID)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, werr := f.Write(append(raw, '\n'))
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(path)
		return "", errors.Join(werr, cerr)
	}
	return path, nil
}

// Load reads and verifies the stored plan with id. A missing plan is ErrNotFound; a plan that does
// not parse, does not verify, or carries another id is ErrInvalid; anything else is the read error.
func (s Store) Load(id string) (Plan, error) {
	if err := checkID(id); err != nil {
		return Plan{}, err
	}
	raw, err := os.ReadFile(s.Path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return Plan{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return Plan{}, err
	}
	p, err := DecodePlan(raw)
	if err != nil {
		return Plan{}, err
	}
	if p.ID != id {
		return Plan{}, fmt.Errorf("%w: the file for %s holds plan %s", ErrInvalid, id, p.ID)
	}
	return p, nil
}

// Consumed reports whether the plan's consumed marker exists.
func (s Store) Consumed(id string) (bool, error) {
	if err := checkID(id); err != nil {
		return false, err
	}
	_, err := os.Lstat(s.MarkerPath(id))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// Consume marks the plan consumed by exclusive creation of its marker. Exactly one of any number
// of concurrent callers succeeds; every other gets ErrAlreadyConsumed. The marker is the whole
// record: once it exists the plan cannot be applied again, unless someone deletes it — the bound
// the spec states (a deleted marker allows another interactive apply within the plan's expiry,
// still subject to the re-measure).
func (s Store) Consume(id string, now time.Time) error {
	if err := checkID(id); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.MarkerPath(id)), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.MarkerPath(id), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %s", ErrAlreadyConsumed, id)
	}
	if err != nil {
		return err
	}
	_, werr := f.WriteString(Stamp(now) + "\n")
	return errors.Join(werr, f.Close())
}
