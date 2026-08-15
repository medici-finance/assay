package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdPlan is the reactor's whole shipped surface: read the sweep(s), classify every row,
// coalesce the outward verbs, and state the idle verdict. It spawns nothing and writes
// nothing outward — the standing-window cutover is gate: human.
//
// --actions and --prs are FILES (or "-" for stdin on --actions), not a shelled-out
// deskboard: keeping the network out of this binary is what lets the reactor's own
// semantics be tested against fixtures, and it keeps the one process that could dispatch
// reviewers from also being the one that hits the shared token.
func cmdPlan(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	actionsPath := fs.String("actions", "", "path to `deskboard actions` JSON (or - for stdin) — REQUIRED")
	prsPath := fs.String("prs", "", "path to `deskboard prs` JSON — supplies the head SHAs the actions verb omits")
	nowStr := fs.String("now", "", "RFC3339 instant to age the board against (default: wall clock)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("reviewloop plan: bad flags: " + err.Error())
	}
	if strings.TrimSpace(*actionsPath) == "" {
		return deskkit.Refused("reviewloop plan: --actions is required. This tool refuses to run against no board at all: " +
			"a reactor with no sweep is BLIND, and blind is not idle (#79).")
	}

	now := time.Now().UTC()
	if strings.TrimSpace(*nowStr) != "" {
		t, err := time.Parse(time.RFC3339, *nowStr)
		if err != nil {
			return deskkit.Refused("reviewloop plan: --now is not RFC3339: " + err.Error())
		}
		now = t
	}

	actionsJSON, err := readInput(*actionsPath)
	if err != nil {
		return deskkit.Unverifiable("reviewloop plan: cannot read the actions payload", err)
	}
	var prsJSON []byte
	if strings.TrimSpace(*prsPath) != "" {
		prsJSON, err = readInput(*prsPath)
		if err != nil {
			return deskkit.Unverifiable("reviewloop plan: cannot read the prs payload", err)
		}
	}

	board, err := ReadBoard(actionsJSON, prsJSON)
	if err != nil {
		return err // already a typed deskkit error carrying its exit code
	}

	fmt.Fprintf(stdout, "reviewloop plan — board-reactor for pr-review-desk (archetype B; NOT a drain)\n")
	fmt.Fprintf(stdout, "swept %s · %d row(s) · action table watches %d ACTION(s)\n",
		board.AsOf.Format(time.RFC3339), len(board.Rows), len(KnownActions()))
	if board.Scope != nil {
		fmt.Fprintf(stdout, "scope: %d repo(s) — %s\n", board.Scope.Count, board.Scope.Source)
	}

	fmt.Fprintf(stdout, "\nROWS (every row, including the waiting states — a row is never dropped for having no verb):\n")
	board.RenderRows(stdout)

	counts := board.CountByDisposition()
	fmt.Fprintf(stdout, "\ndispositions: %d DISPATCH · %d FLIP-VERB · %d SURFACE · %d WAIT · %d NO-OP\n",
		counts[DispositionDispatch], counts[DispositionFlip], counts[DispositionSurface],
		counts[DispositionWait], counts[DispositionNoOp])

	if u := board.UnresolvedHeads(); len(u) > 0 {
		fmt.Fprintf(stdout, "\nHEAD UNRESOLVED for %d row(s): %s\n"+
			"  => `deskboard actions` carries no head SHA; supply --prs so outward verbs can be keyed and de-duplicated.\n",
			len(u), strings.Join(u, ", "))
	}

	verbs := Coalesce([]*Board{board}, deskkit.AlreadyDone)
	fmt.Fprintf(stdout, "\nOUTWARD VERBS, coalesced on (repo, pr, head, verb):\n")
	if len(verbs) == 0 {
		fmt.Fprintf(stdout, "  (none — no row on this board carries an outward verb)\n")
	}
	for _, v := range verbs {
		state := "emit"
		if v.Suppressed {
			state = "SUPPRESSED"
		}
		fmt.Fprintf(stdout, "  %-10s %s  (deltas coalesced: %d)\n", state, v.Key, v.Observations)
		if v.Suppressed {
			fmt.Fprintf(stdout, "      => %s\n", v.Why)
		}
	}
	fmt.Fprintf(stdout, "  the desk NEVER merges: `ready` flips a MERGE-NOW draft to ready under deskpost's existing gates; the merge stays the human's.\n")

	verdict := Idle(board, now)
	fmt.Fprintf(stdout, "\nIDLE GATE (#79, in code — a model cannot declare all-clear): %s\n", verdict)
	for _, r := range verdict.BlindReasons {
		fmt.Fprintf(stdout, "  blind: %s\n", r)
	}

	RenderUnreadSurfaces(stdout)

	fmt.Fprintf(stdout, "\nSTOP POINT: cutover of the standing review window onto this driver is gate: human — BLOCKED-ON-HUMAN.\n")

	// A board that cannot answer the idle question is a could-not-check RUN, not a
	// successful one. Exit 6 so a caller scripting this cannot read rc=0 as all-clear.
	if verdict.State == IdleCouldNotCheck {
		return deskkit.Unverifiable("reviewloop: the idle gate is COULD-NOT-CHECK — "+
			strings.Join(verdict.BlindReasons, "; "), nil)
	}
	return nil
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
