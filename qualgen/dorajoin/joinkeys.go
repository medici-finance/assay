package dorajoin

import "fmt"

// joinkeys.go — the join-key resolver (spec §3.2, §8): resolves a
// delivery-metrics record to a quality record on the keys the flow board
// already uses (PR number + merge SHA + stream/task ID), so the DORA join
// never invents a new correlation scheme. A squash or history-rewriting merge
// can break the merge-SHA half of a key; the resolver reports that as its own
// three-state outcome rather than silently dropping the record.

// JoinKey identifies one delivery/quality record pair. Any subset of the
// three fields may be set; ResolveJoin matches on the strongest field both
// sides carry (PR number, then merge SHA, then stream/task ID) so a partial
// key from either side still has a chance to resolve.
type JoinKey struct {
	PRNumber     int    `json:"pr_number,omitempty"`
	MergeSHA     string `json:"merge_sha,omitempty"`
	StreamTaskID string `json:"stream_task_id,omitempty"`
}

// MatchState is the three-state join outcome (spec §3.2). Every delivery
// record resolves to exactly one; none are ever silently dropped.
type MatchState string

const (
	// MatchMatched — a quality record was found for this delivery record's
	// key.
	MatchMatched MatchState = "matched"
	// MatchNoMatch — the key itself resolved cleanly (its merge SHA, when
	// present, is reachable in the mined history) but no quality record
	// carries a matching key. A real, looked-and-found-nothing answer.
	MatchNoMatch MatchState = "no-match"
	// MatchCouldNotJoin — the key could not be resolved at all: an
	// unreachable merge SHA (squash or history-rewriting merge), or the
	// reachability check itself failed. Never counted as a match, never
	// dropped.
	MatchCouldNotJoin MatchState = "could-not-join"
)

// JoinResult is the outcome of resolving one delivery record's key.
type JoinResult struct {
	DeliveryKey JoinKey    `json:"delivery_key"`
	QualityKey  *JoinKey   `json:"quality_key,omitempty"`
	State       MatchState `json:"state"`
	// Reason is set for MatchCouldNotJoin, naming why the key could not be
	// resolved.
	Reason string `json:"reason,omitempty"`
}

// ShaReachable reports whether sha is reachable in the mined quality history.
// A squash merge or a history rewrite (rebase, filter-branch, a shallow /
// grafted clone's floor) collapses the original merge SHA away; the caller
// supplies this check (typically a git plumbing lookup) rather than dorajoin
// assuming access to a repository.
type ShaReachable func(sha string) (bool, error)

// keysEqual matches on the strongest field both keys carry: PR number when
// both set it, else merge SHA when both set it, else stream/task ID. An
// empty field on either side never counts as a match on that field.
func keysEqual(a, b JoinKey) bool {
	if a.PRNumber != 0 && b.PRNumber != 0 {
		return a.PRNumber == b.PRNumber
	}
	if a.MergeSHA != "" && b.MergeSHA != "" {
		return a.MergeSHA == b.MergeSHA
	}
	return a.StreamTaskID != "" && a.StreamTaskID == b.StreamTaskID
}

// ResolveJoin resolves one delivery record's key against the set of quality
// keys already known. When delivery carries a merge SHA, reachable is
// consulted first: an unreachable SHA (or a reachability-check error) is
// could-not-join before any key comparison runs, because a delivery record
// whose own merge SHA cannot be found in the mined history has no reliable
// key to match on at all — a coincidental PR-number or task-ID match against
// the wrong quality record would be a false positive, not a resolved join.
func ResolveJoin(delivery JoinKey, quality []JoinKey, reachable ShaReachable) JoinResult {
	if delivery.MergeSHA != "" && reachable != nil {
		ok, err := reachable(delivery.MergeSHA)
		if err != nil {
			return JoinResult{
				DeliveryKey: delivery, State: MatchCouldNotJoin,
				Reason: fmt.Sprintf("checking merge-SHA reachability: %v", err),
			}
		}
		if !ok {
			return JoinResult{
				DeliveryKey: delivery, State: MatchCouldNotJoin,
				Reason: "merge SHA unreachable in mined history (squash or history-rewriting merge)",
			}
		}
	}

	for i := range quality {
		if keysEqual(delivery, quality[i]) {
			matched := quality[i]
			return JoinResult{DeliveryKey: delivery, QualityKey: &matched, State: MatchMatched}
		}
	}
	return JoinResult{DeliveryKey: delivery, State: MatchNoMatch}
}

// ResolveJoins resolves every delivery key against the quality-key set,
// preserving input order. Every delivery record produces exactly one
// JoinResult — none are ever dropped, matched or not.
func ResolveJoins(deliveries []JoinKey, quality []JoinKey, reachable ShaReachable) []JoinResult {
	out := make([]JoinResult, 0, len(deliveries))
	for _, d := range deliveries {
		out = append(out, ResolveJoin(d, quality, reachable))
	}
	return out
}
