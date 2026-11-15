package mirror

import (
	"context"
	"errors"
	"reflect"
)

func admit(ctx context.Context, a Admission, scope Scope) error {
	if a == nil {
		return ErrPermission
	}
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if v.IsNil() {
			return ErrPermission
		}
	}
	if a.ID() == "" || a.ID() != scope.AdmissionID {
		return ErrPermission
	}
	if err := a.Authorize(ctx, scope.Repository); err != nil {
		return ErrPermission
	}
	return nil
}

// QueryScope checks current composition-derived admission before any data read.
// Store errors become explicit unavailable results; malformed contract data is an
// error. Permission refusals expose neither stored records nor stored coverage.
func QueryScope(ctx context.Context, store Store, a Admission, q Query) (Result, error) {
	s, reason, err := ReadScopeSnapshot(ctx, store, a, q)
	if reason != "" {
		return unavailable(q.Scope, reason), err
	}
	if err != nil {
		return Result{}, err
	}
	return EvaluateSnapshot(s, q)
}

// ReadScopeSnapshot acquires an owned snapshot through the existing query and
// admission guards. It does not evaluate timestamps or grant permission to return
// data. Callers must evaluate at their chosen explicit time and enforce current
// caller/admission/deadline checks before release. A nonempty reason is an
// unavailable acquisition and always returns a zero snapshot, never stored fields.
func ReadScopeSnapshot(ctx context.Context, store Store, a Admission, q Query) (Snapshot, Reason, error) {
	if err := validateQuery(q); err != nil {
		return Snapshot{}, "", err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, Transport, err
	}
	if err := admit(ctx, a, q.Scope); err != nil {
		return Snapshot{}, Permission, nil
	}
	if store == nil {
		return Snapshot{}, Transport, nil
	}
	s, err := store.Read(ctx, a, q.Scope)
	if err != nil {
		if errors.Is(err, ErrPermission) {
			return Snapshot{}, Permission, nil
		}
		return Snapshot{}, Transport, nil
	}
	if s.Scope != q.Scope {
		return Snapshot{}, "", invalid("store returned a different scope")
	}
	return CloneSnapshot(s), "", nil
}

func validateQuery(q Query) error {
	if err := q.Scope.Validate(); err != nil {
		return err
	}
	if q.Now.IsZero() || q.MaxAge <= 0 {
		return invalid("query needs explicit time and positive freshness window")
	}
	return nil
}

// EvaluateSnapshot evaluates an already-owned snapshot using explicit query time.
// It performs no I/O and grants no authorization. Callers remain responsible for
// current admission/caller/deadline checks before releasing any result.
func EvaluateSnapshot(s Snapshot, q Query) (Result, error) {
	if err := validateQuery(q); err != nil {
		return Result{}, err
	}
	if s.Scope != q.Scope {
		return Result{}, invalid("store returned a different scope")
	}
	if err := validateSnapshot(s, q.Now); err != nil {
		return Result{}, err
	}
	if s.Coverage.Reason == Permission {
		return unavailable(q.Scope, Permission), nil
	}
	return snapshotResult(s, q), nil
}
