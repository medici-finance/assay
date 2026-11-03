// Package mirrortest provides synthetic Store conformance support. ReferenceStore
// is not production storage, durable persistence or a private-data retention policy.
package mirrortest

import (
	"context"
	"strconv"
	"sync"

	"github.com/medici-finance/assay/forgeread/mirror"
)

type ReferenceStore struct {
	mu        sync.Mutex
	snapshots map[mirror.Scope]mirror.Snapshot
	version   uint64
}

func NewStore() *ReferenceStore {
	return &ReferenceStore{snapshots: map[mirror.Scope]mirror.Snapshot{}}
}

// storeAdmission deliberately does not call the library's query check: the store
// has its own enforcement point when QueryScope is bypassed.
func storeAdmission(ctx context.Context, a mirror.Admission, scope mirror.Scope) error {
	if a == nil || a.ID() == "" || a.ID() != scope.AdmissionID {
		return mirror.ErrPermission
	}
	if err := a.Authorize(ctx, scope.Repository); err != nil {
		return mirror.ErrPermission
	}
	return nil
}
func (s *ReferenceStore) Read(ctx context.Context, a mirror.Admission, scope mirror.Scope) (mirror.Snapshot, error) {
	if err := scope.Validate(); err != nil {
		return mirror.Snapshot{}, err
	}
	if err := storeAdmission(ctx, a, scope); err != nil {
		return mirror.Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap, ok := s.snapshots[scope]; ok {
		return mirror.CloneSnapshot(snap), nil
	}
	return mirror.Snapshot{Scope: scope, Coverage: mirror.Coverage{State: mirror.NeverRead}, Records: []mirror.Observation{}}, nil
}
func (s *ReferenceStore) Apply(ctx context.Context, a mirror.Admission, u mirror.ScanUpdate) (mirror.Snapshot, error) {
	if err := u.Validate(); err != nil {
		return mirror.Snapshot{}, err
	}
	if err := storeAdmission(ctx, a, u.Scope); err != nil {
		return mirror.Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.snapshots[u.Scope]
	if !ok {
		old = mirror.Snapshot{Scope: u.Scope, Coverage: mirror.Coverage{State: mirror.NeverRead}}
	}
	if u.ExpectedVersion != old.Version {
		return mirror.Snapshot{}, mirror.ErrConflict
	}
	if u.Coverage.LastComplete.After(old.Coverage.LastComplete) && (!u.CompleteScan || u.Coverage.State != mirror.Complete) {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	if u.Coverage.LastInventory.After(old.Coverage.LastInventory) && (!u.CompleteScan || !u.FullInventory) {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	if u.Coverage.LastComplete.Before(old.Coverage.LastComplete) || u.Coverage.LastAttempted.Before(old.Coverage.LastAttempted) || u.Coverage.LastInventory.Before(old.Coverage.LastInventory) || u.Coverage.ObservedUpTo.Before(old.Coverage.ObservedUpTo) {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	if u.Coverage.State == mirror.Complete && !u.CompleteScan {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	if len(u.NotInInventory) > 0 && (!u.CompleteScan || !u.FullInventory) {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	if u.RetentionDecisionRequired && (u.Coverage.State != mirror.Unavailable || u.Coverage.Reason != mirror.Permission || len(u.Upsert) > 0 || len(u.NotInInventory) > 0) {
		return mirror.Snapshot{}, mirror.ErrInvalid
	}
	rows := mirror.CloneSnapshot(old).Records
	for _, next := range mirror.CloneSnapshot(mirror.Snapshot{Records: u.Upsert}).Records {
		found := false
		for i := range rows {
			if rows[i].ID == next.ID {
				rows[i] = next
				found = true
				break
			}
		}
		if !found {
			rows = append(rows, next)
		}
	}
	for _, id := range u.NotInInventory {
		for i := range rows {
			if rows[i].ID == id {
				rows[i].InInventory = false
			}
		}
	}
	s.version++
	snap := mirror.Snapshot{Scope: u.Scope, Coverage: u.Coverage, Records: rows, Version: strconv.FormatUint(s.version, 10)}
	s.snapshots[u.Scope] = mirror.CloneSnapshot(snap)
	return mirror.CloneSnapshot(snap), nil
}

var _ mirror.Store = (*ReferenceStore)(nil)
