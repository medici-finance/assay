// Package runner is the shared loop-admin runner contract: the caller API
// (Client) and the adapter API (Adapter) that every launch of a model-backed
// agent goes through, whether the caller is a standing desk or a workflow
// stage.
//
// The contract is protocol and checking only. It launches nothing itself, holds
// no credential, selects no work, advances no graph node, approves no result and
// offers no operator administration. A concrete adapter wraps a harness; the
// supervisor (a later unit) journals launches. The module deliberately has no
// dependency on the workflow module or the instance store.
//
// Four rules run through every file here:
//
//   - Unknown stays unknown. A lost launch acknowledgment, a missing usage
//     figure and an unconfirmed cancellation are each reported as such, never
//     rounded to failure, zero or stopped.
//   - Process exit is not acceptance. An adapter reports a terminal observation;
//     only Client.AcceptResult, after the generation fence and the result checks,
//     turns it into an Accepted value, and even that is not the caller's
//     decision to take the work.
//   - No silent fallback. The profile pins the model and the tool list; an
//     observation from another model, or a tool request outside the list, is
//     refused whatever the model's own text says.
//   - A mandatory field or capability the consumer does not understand refuses
//     the request instead of downgrading it.
package runner
