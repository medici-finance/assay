package main

import (
	"fmt"
	"io"
)

// UnreadSurface is one inbound surface this consumer does NOT read, stated in-band.
//
// The rule is no silent caps: a bound that exists only in the author's head is indistinguishable,
// from the output, from no bound at all. This drain narrows coverage in several honest ways, so it
// states its own narrowing on every run rather than in a comment nobody runs.
type UnreadSurface struct {
	Name        string
	Consequence string
}

var unreadSurfaces = []UnreadSurface{
	{
		Name: "pull requests and their reviews",
		Consequence: "this drain's inbound surface is ISSUES and their comments. A PR arriving on a " +
			"watched repo is another loop's queue, and this one neither sees nor counts it",
	},
	{
		Name: "the intake register's raw entries (ideas filed as text, not as issues)",
		Consequence: "the register lane is worked by hand today; its untriaged count and its age " +
			"threshold are reported by the board tool, not by this drain, so an aged register entry is " +
			"invisible here",
	},
	{
		Name: "items suppressed by the poller's burst cap",
		Consequence: "a mass update collapses to one burst line with no per-item keys, so those items " +
			"cannot be queued this pass. The pass reports itself BLIND rather than reporting the " +
			"remainder as the whole",
	},
	{
		Name: "repos whose poll DEGRADED this cycle",
		Consequence: "their baselines were retained, so nothing is lost — but this pass cannot speak " +
			"for them, and says so instead of returning a partial queue as the queue",
	},
	{
		Name: "the quarantined lane's contents",
		Consequence: "items from untrusted authors with no standing blessing are listed and counted " +
			"and never routed. That is the trust gate working, not a gap — but it does mean the queue " +
			"length is not the inbound length",
	},
	{
		Name: "the register write itself (the finding and rejected/watching exits)",
		Consequence: "those two exits are recorded in this drain's ledger but produced by the model " +
			"tier's own register edit, not by a lane here. The ledger proves the item left; it does not " +
			"prove the register file was written",
	},
	{
		Name: "cross-machine dispatch state",
		Consequence: "the claims dir this drain consults is machine-local. It serialises dispatchers " +
			"sharing a machine and says nothing about a desk running elsewhere",
	},
}

// RenderUnreadSurfaces prints the roster. It runs on EVERY plan, not behind a verbose flag: a cap
// that has to be asked for is a cap nobody sees.
func RenderUnreadSurfaces(w io.Writer) {
	fmt.Fprintf(w, "\nINBOUND SURFACES THIS DRAIN DOES NOT READ (%d) — stated every run, never inferred from silence:\n", len(unreadSurfaces))
	for _, s := range unreadSurfaces {
		fmt.Fprintf(w, "  - %s\n      => %s\n", s.Name, s.Consequence)
	}
}
