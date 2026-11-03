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
	if err := validateQuery(q); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return unavailable(q.Scope, Transport), err
	}
	if err := admit(ctx, a, q.Scope); err != nil {
		return unavailable(q.Scope, Permission), nil
	}
	if store == nil {
		return unavailable(q.Scope, Transport), nil
	}
	s, err := store.Read(ctx, a, q.Scope)
	if err != nil {
		if errors.Is(err, ErrPermission) {
			return unavailable(q.Scope, Permission), nil
		}
		return unavailable(q.Scope, Transport), nil
	}
	return EvaluateSnapshot(s, q)
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
