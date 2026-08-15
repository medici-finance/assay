package main

import (
	"fmt"
	"io"
)

// UnreadSurface is one board surface this reactor does NOT read, stated in-band.
//
// The rule is no silent caps: a bound that only exists in the author's head is
// indistinguishable, from the output, from no bound at all. deskboard already applies this
// to itself (its scope footer, its POPULATION TRUNCATED line, its observability bound), and
// the reactor is a second consumer that can narrow coverage again — so it states its own
// narrowing the same way, on every run, rather than in a comment.
type UnreadSurface struct {
	Name        string
	Consequence string
}

// unreadSurfaces is the complete roster of what the reactor is blind to. Each entry is a
// MEASURED gap in the payloads it consumes, not a guess.
var unreadSurfaces = []UnreadSurface{
	{
		Name: "deskboard `actions` tombstones[] — PRs that left the open set between sweeps",
		Consequence: "a PR merged or closed since the last sweep is not reconciled by this reactor; " +
			"the human/desk still sees it on the board's own table output",
	},
	{
		Name: "deskboard `actions` external[] — the trust gate's EXTERNAL / UNBLESSED quarantine",
		Consequence: "quarantined items are never dispatched, flipped or counted here, which is the " +
			"intended trust-gate behaviour; they remain quarantined-VISIBLE on the board, never on this reactor's",
	},
	{
		Name: "the header alarms: unreviewedCount / unreviewedAgeUnknownCount (neglect), " +
			"mergeNowDecay / mergeNowAgeUnknownCount (decay), mainHealth (red default branch), policyDrift",
		Consequence: "these are standing ALARMS about the trigger path's own health, not per-row work; " +
			"the reactor neither reads nor clears them, so a dead trigger path is still detected only by the board and the desk",
	},
	{
		Name: "deskboard `scope` — the unwatched-repo reconciliation (open PRs under a watched owner in a repo the board does not watch)",
		Consequence: "a repo outside deskkit.AllowedRepos() is invisible to this reactor exactly as it is to the board; " +
			"and per deskboard's own observabilityBound, gap=false means NO GAP WAS OBSERVED, never NO GAP EXISTS",
	},
	{
		Name: "issue-loop's board surfaces — issueboard's blocked-placeholder unblock scanner and the needs-decision watch",
		Consequence: "named as the follow-up SEAM, not wired: those are a separate consumer of this driver " +
			"(a follow-up brief), and until then this reactor speaks only for PRs",
	},
	{
		Name: "GitHub itself — this reactor makes NO API calls",
		Consequence: "every fact it reports comes from a deskboard payload handed to it; it can be no fresher, " +
			"no wider and no more complete than that sweep, and the freshness bound is enforced by the idle gate rather than assumed",
	},
	{
		Name: "the loopengine claims dir and its WorkEvidence probe (loop-engine/10)",
		Consequence: "deliberately NOT a consumer: Claim() dedupes brief-shaped drain items, and this reactor's items " +
			"are long-lived PRs whose idempotency is the audit-ledger (repo,pr,head,verb) key. loop-engine/10 names its consumers " +
			"as loop-engine/02 and /03; this driver is not a fourth one, and says so rather than leaving the omission to be read as an oversight",
	},
}

// RenderUnreadSurfaces prints the roster. It runs on EVERY plan, not behind a verbose
// flag: a cap that has to be asked for is a cap nobody sees.
func RenderUnreadSurfaces(w io.Writer) {
	fmt.Fprintf(w, "\nBOARD SURFACES THIS REACTOR DOES NOT READ (%d) — stated every run, never inferred from silence:\n", len(unreadSurfaces))
	for _, s := range unreadSurfaces {
		fmt.Fprintf(w, "  - %s\n      => %s\n", s.Name, s.Consequence)
	}
}
