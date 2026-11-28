package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
)

func TestPGMirrorSnapshotPages(t *testing.T) {
	ctx := context.Background()
	f, config, rawDB := fixture(t)
	if err := Migrate(ctx, config); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	limits := MirrorLimits{MaxRecords: 1024, MaxBytes: 4 << 20}
	writer, err := db.Mirror(limits)
	if err != nil {
		t.Fatal(err)
	}
	roles := f.NewRoles(t)
	if err := f.CheckRoles(ctx, roles); err != nil {
		t.Fatal(err)
	}
	grants, err := (RefreshRoleProfile{Database: config.Database, Operator: roles.Operator.User, Runtime: roles.Runtime.User}).SQL()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rawDB.ExecContext(ctx, grants); err != nil {
		t.Fatal("canonical fixture grants failed", err)
	}
	runtime, err := Open(ctx, testConfig(roles.Runtime))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	s, err := runtime.Mirror(limits)
	if err != nil {
		t.Fatal(err)
	}
	admission, update := mirrorInput()
	row := update.Upsert[0]
	for i := int64(2); i <= 620; i++ {
		row.ID.Number = i
		update.Upsert = append(update.Upsert, row)
	}
	full := applyMirror(t, writer, admission, update)
	var after int64
	for _, want := range []int{256, 256, 108} {
		page, err := s.ReadPage(ctx, admission, full.Scope, full.Version, full.Coverage.LastComplete, after, 256, 1<<20)
		if err != nil || len(page.Snapshot.Records) != want || page.More != (want == 256) ||
			page.Snapshot.Version != full.Version || page.Snapshot.Coverage != full.Coverage {
			t.Fatalf("unexpected page of %d rows after %d: %v", want, after, err)
		}
		for _, row := range page.Snapshot.Records {
			after++
			if row.ID.Number != after {
				t.Fatalf("want ordered record %d, got %d", after, row.ID.Number)
			}
		}
	}
	raw, err := encodeFact(full.Records[0], 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.ReadPage(ctx, admission, full.Scope, full.Version, full.Coverage.LastComplete, 0, 256, len(raw))
	if err != nil || len(page.Snapshot.Records) != 1 || !page.More {
		t.Fatal("byte budget did not hold a fitting prefix", err)
	}
	if _, err := s.ReadPage(ctx, admission, full.Scope, full.Version, full.Coverage.LastComplete, 0, 256, len(raw)-1); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatal("unrepresentable first row was not refused", err)
	}
	if _, err := s.ReadPage(ctx, admission, full.Scope, full.Version, full.Coverage.LastComplete.Add(time.Second), 256, 256, 1<<20); !errors.Is(err, mirror.ErrConflict) {
		t.Fatal("different observation time was accepted", err)
	}
	update.ExpectedVersion = full.Version
	if next := applyMirror(t, writer, admission, update); next.Version == full.Version {
		t.Fatal("fixture did not advance the version")
	}
	page, err = s.ReadPage(ctx, admission, full.Scope, full.Version, full.Coverage.LastComplete, 256, 256, 1<<20)
	if !errors.Is(err, mirror.ErrConflict) || page.Snapshot.Records != nil {
		t.Fatal("continuation crossed a concurrent refresh", err)
	}
	admission.Allowed = false
	_, err = s.ReadPage(ctx, admission, full.Scope, full.Version, full.Coverage.LastComplete, 256, 256, 1<<20)
	if !errors.Is(err, mirror.ErrPermission) {
		t.Fatal("revoked admission reached the data", err)
	}
}
