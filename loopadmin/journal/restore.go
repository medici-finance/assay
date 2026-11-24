package journal

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"sort"
)

// ErrRestoreHeld refuses a backup or a restored journal whose referenced
// artifact inventory does not verify: an artifact is missing, corrupt, or
// absent from one side of the inventory. The journal stays held.
var ErrRestoreHeld = errors.New("loopadmin/journal: restore held: the referenced-artifact inventory does not verify")

// ArtifactStore opens a referenced artifact by its reference. It is the
// operator's read of the artifact store; the journal only reads through it.
type ArtifactStore interface {
	Open(ctx context.Context, ref string) (io.ReadCloser, error)
}

// Manifest is a backup's passive inventory: the journal schema version and
// every referenced artifact with its digest, taken beside the database copy.
// The database copy itself is made and restored by the operator's separately
// authorized adapter, never by this package.
type Manifest struct {
	Version   int64
	Artifacts []Ref
}

// Inventory is the operator's backup step. It reads the journal's schema
// version and referenced-artifact inventory and verifies every artifact's
// digest against store, so a backup never records an inventory that does not
// verify. It writes nothing.
func Inventory(ctx context.Context, c Config, store ArtifactStore) (Manifest, error) {
	db, err := connect(ctx, c)
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = db.Close() }() // Preserve the inventory result.
	if err = checkSchema(ctx, db); err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if m.Version, err = schemaVersion(ctx, db); err != nil {
		return Manifest{}, err
	}
	if m.Artifacts, err = artifacts(ctx, db); err != nil {
		return Manifest{}, err
	}
	if err = verifyArtifacts(ctx, store, m.Artifacts); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// CheckRestore is the operator's passive check of a restored journal. It
// first pauses the cell's activation record, so the restored journal is held
// whatever the check finds, and never launches or activates. It then requires
// the restored schema version and referenced-artifact inventory to equal the
// backup manifest and every artifact to match its digest in store. A nil
// result means only that the inventory verifies: reopening the journal still
// takes a rotated incarnation and a confirmed group stop (Activate).
func CheckRestore(ctx context.Context, c Config, cell string, m Manifest, store ArtifactStore) error {
	if cell == "" || store == nil {
		return ErrInvalid
	}
	db, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }() // Preserve the restore-check result.
	if err = checkSchema(ctx, db); err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, "UPDATE loopadmin.activation SET paused = true, recorded_at = now() WHERE cell_id = $1", cell); err != nil {
		return classify(ctx, "restore", err)
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if m.Version != version {
		return ErrRestoreHeld
	}
	got, err := artifacts(ctx, db)
	if err != nil {
		return err
	}
	want := append([]Ref(nil), m.Artifacts...)
	sortRefs(want)
	if len(got) != len(want) {
		return ErrRestoreHeld
	}
	for i := range got {
		if got[i] != want[i] {
			return ErrRestoreHeld
		}
	}
	return verifyArtifacts(ctx, store, got)
}

// schemaVersion is this build's target migration version; checkSchema has
// already required the database to be at it.
func schemaVersion(ctx context.Context, db *sql.DB) (int64, error) {
	p, err := provider(db)
	if err != nil {
		return 0, err
	}
	_, target, err := p.GetVersions(ctx)
	if err != nil {
		return 0, failure(ctx, "inventory", Incompatible)
	}
	return target, nil
}

func artifacts(ctx context.Context, db *sql.DB) ([]Ref, error) {
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT ref, digest FROM loopadmin.artifact")
	if err != nil {
		return nil, classify(ctx, "inventory", err)
	}
	defer func() { _ = rows.Close() }() // Rows.Err reports iteration and close errors.
	var out []Ref
	for rows.Next() {
		var r Ref
		if err = rows.Scan(&r.Ref, &r.Digest); err != nil {
			return nil, classify(ctx, "inventory", err)
		}
		out = append(out, r)
	}
	if err = classify(ctx, "inventory", rows.Err()); err != nil {
		return nil, err
	}
	sortRefs(out)
	return out, nil
}

func sortRefs(refs []Ref) {
	sort.Slice(refs, func(i, k int) bool {
		if refs[i].Ref != refs[k].Ref {
			return refs[i].Ref < refs[k].Ref
		}
		return refs[i].Digest < refs[k].Digest
	})
}

func verifyArtifacts(ctx context.Context, store ArtifactStore, refs []Ref) error {
	if store == nil {
		return ErrRestoreHeld
	}
	for _, r := range refs {
		f, err := store.Open(ctx, r.Ref)
		if err != nil {
			return ErrRestoreHeld
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return ErrRestoreHeld
		}
		if "sha256:"+hex.EncodeToString(h.Sum(nil)) != r.Digest {
			return ErrRestoreHeld
		}
	}
	return nil
}
