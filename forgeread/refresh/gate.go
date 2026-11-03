package refresh

import (
	"context"
	"github.com/medici-finance/assay/forgeread/mirror"
)

// Gate is implemented only by the canonical R2 store in production. Acquire owns
// shared caller/admission accounting, fairness, failure capacity and retry eligibility.
// A missing/unreadable record is held, never a fresh local allowance. Unexpired
// failures cannot be evicted into an allowance. A quota denial creates no fill failure.
// Caller/admission denial (including typed mirror.ErrPermission) must not create
// a shared source-failure cooldown; other eligible callers retain their allowance.
// Each ticket accounts for one waiter; Leave is required on every exit.
//
// Complete must reject a stale generation BEFORE Store.Apply, even if the expected
// snapshot version still matches. It must bind store to its own canonical backend,
// and run ApplyCompletion plus ownership/completion/cooldown in ONE transaction.
// A failed commit is unknown, never permission to replay. The store independently
// enforces admission and completeness. There is deliberately no production backend
// or local fallback in this module; test implementations establish only port behavior.
type Gate interface {
	Acquire(context.Context, Demand) (Ticket, error)
	Wait(context.Context, Ticket) (State, error)
	Complete(context.Context, Ticket, mirror.Store, mirror.Admission, Completion) error
	Leave(context.Context, Ticket) error
}

// ApplyCompletion is the typed shared Store.Apply -> QueryScope seam used inside
// Gate.Complete's transaction. It is not a transaction callback or a second store.
// The canonical adapter supplies its transaction-bound Store, verifies current owner
// generation first, and commits failure/fulfilment accounting with this result.
func ApplyCompletion(ctx context.Context, store mirror.Store, a mirror.Admission, c Completion) (Result, error) {
	if c.AdmissionDenied {
		return held(c.Query, mirror.Permission), mirror.ErrPermission
	}
	if absent(store) {
		return held(c.Query, mirror.Transport), ErrHeld
	}
	if absent(c.Authority) {
		return held(c.Query, mirror.Permission), ErrHeld
	}
	if err := c.Authority.Authorize(ctx, c.Caller, c.Query.Scope); err != nil {
		return held(c.Query, mirror.Permission), err
	}
	if err := authorize(ctx, a, c.Query.Scope); err != nil {
		return held(c.Query, mirror.Permission), err
	}
	if c.Update != nil {
		if c.Update.Scope != c.Query.Scope {
			return held(c.Query, mirror.Unknown), mirror.ErrInvalid
		}
		if _, err := store.Apply(ctx, a, *c.Update); err != nil {
			return held(c.Query, mirror.Unknown), err
		}
	}
	q := c.Query
	q.Now = c.Now
	r, err := mirror.QueryScope(ctx, store, a, q.Query)
	if err != nil {
		return held(q, mirror.Unknown), err
	}
	// Revalidate after the final store I/O, before releasing any rows.
	if err := c.Authority.Authorize(ctx, c.Caller, q.Scope); err != nil {
		return held(q, mirror.Permission), err
	}
	if err := authorize(ctx, a, q.Scope); err != nil {
		return held(q, mirror.Permission), err
	}
	out := present(r, q, State{Reason: c.Failure})
	if c.Failure != "" {
		out.Fulfilled = false
		out.Available = false
		out.Stale = true
		out.Reason = c.Failure
	}
	return out, nil
}
