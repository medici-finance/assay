package main

// prstate.go — merged vs closed-unmerged, kept apart.
//
// The board's #209 tombstone lane labelled EVERY vanished PR "MERGED — drop from your
// list", inferring the label from absence. Absence proves the PR left the open set; it
// proves nothing about which way it left. example-org/tracker#1580 was
// CLOSED unmerged (mergedAt null) and the board printed MERGED, which the review desk
// then relayed to a human as fact.
//
// The distinction is load-bearing, not cosmetic: a merged PR's content is on main; a
// closed-unmerged PR's content is NOT, which changes what follow-up work exists — and
// the intake-desk's CLOSE-ON-FIX-LANDED lane makes an irreversible write off exactly
// this bit ("the MERGE is the authorization"). A closed-unmerged PR wearing a MERGED
// label closes issues whose fix never landed.
//
// Three states, never two:
//
//	merged   — the API says merged_at is set
//	closed   — open=false and merged_at is null: content did NOT land
//	unknown  — the state read failed, or the payload did not say: COULD-NOT-CHECK,
//	           reported as such, never resolved to either of the other two
//
// The `merged` boolean is a POINTER and is omitted in the unknown case. A consumer
// deciding an irreversible action must handle a missing field; it can never read a
// zero-value `false` as "closed unmerged" or a stale `true` as "merged".

import (
	"encoding/json"
	"fmt"
)

const (
	prStateMerged  = "merged"
	prStateClosed  = "closed"
	prStateOpen    = "open"
	prStateUnknown = "unknown"
)

// fetchPRState resolves how a PR left the open set. GET-only. A read or parse failure
// is NOT an error the caller must fail on — a tombstone is an advisory lane and killing
// the whole board over one vanished PR would be the noise that gets a board ignored —
// but it MUST degrade to `unknown` carrying the reason, never to a guessed `merged`.
func fetchPRState(repo string, num int) (state string, merged *bool, detail string) {
	out, err := ghRun("api", fmt.Sprintf("repos/%s/pulls/%d", repo, num))
	if err != nil {
		return prStateUnknown, nil, "could not read PR state: " + err.Error()
	}
	var v struct {
		State    string  `json:"state"`
		Merged   *bool   `json:"merged"`
		MergedAt *string `json:"merged_at"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return prStateUnknown, nil, "could not parse PR state: " + err.Error()
	}
	switch {
	case v.MergedAt != nil && *v.MergedAt != "":
		t := true
		return prStateMerged, &t, "merged_at " + *v.MergedAt
	case v.Merged != nil && *v.Merged:
		t := true
		return prStateMerged, &t, "merged=true"
	case v.State == "closed":
		f := false
		return prStateClosed, &f, "state=closed, merged_at null"
	case v.State == "open":
		// It is open again (reopened between sweeps) — say so rather than tombstoning
		// it as gone.
		return prStateOpen, nil, "state=open (reopened since the prior sweep)"
	default:
		return prStateUnknown, nil, "the PR payload carried neither merged_at nor a recognised state"
	}
}

// tombstoneNote renders the drop-from-your-list line for each state. The action ("drop
// it") is the same in all three; the CLAIM is not, and only the claim was wrong.
func tombstoneNote(state, detail string) string {
	switch state {
	case prStateMerged:
		return "MERGED — drop from your list (" + detail + ")"
	case prStateClosed:
		return "CLOSED (unmerged) — drop from your list; its content did NOT land on main, " +
			"so anything it carried is still unlanded work (" + detail + ")"
	case prStateOpen:
		return "REOPENED — it is open again; it is back ON your list (" + detail + ")"
	default:
		return "GONE, merged-or-closed COULD-NOT-CHECK — drop from your list, but do NOT " +
			"record it as merged: " + detail
	}
}
