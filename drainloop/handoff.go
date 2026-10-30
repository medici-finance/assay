package drainloop

import "context"

// Intent is an outbox operation bound to the claim generation admitted by its
// coordination store. The stable Key excludes AttemptID and Generation.
type Intent struct {
	Event Event
}

// TransactionalJournal is an opt-in Journal extension. Its implementation belongs
// to the coordination store, never a second Git journal or a fallback store.
//
// Generation returns the currently owned claim generation after Claim succeeds.
// CommitIntent must atomically compare that generation to current ownership and
// durably commit EFFECT-INTENT plus the admitted generation before returning nil.
// A stale owner or unavailable store returns an error without admission. Existing
// pending entries must not be overwritten under a newer generation.
// Pending must return all outstanding intents for RunID, or an error, never a
// partial success. Record(EFFECT-DONE) completes only the matching key/generation;
// completion failure leaves the entry pending for authoritative reconciliation.
// The implementation is responsible for transactional semantics; implementing the
// interface alone is not evidence that a real store enforces them.
type TransactionalJournal interface {
	Journal
	Generation(context.Context, string) (string, error)
	CommitIntent(context.Context, Intent) error
	Pending(context.Context, string) ([]Intent, error)
}
