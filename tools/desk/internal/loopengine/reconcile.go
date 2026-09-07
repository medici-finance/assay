package loopengine

import (
	"errors"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reconcile.go — the ELIGIBILITY reconciliation the observer runs for every in-flight
// dispatch claim each tick, before the liveness evaluation.
//
// The motivating failure (the observer stream's eligibility-reconcile brief): "a merged or closed PR is DONE,
// stop, never push its branch again" was PROSE a model had to remember, and it leaked — a
// worker whose PR a human merged mid-run kept working, re-pushing to a merged branch and
// resuming an already-merged PR. This turns that sentence into a mechanical check that fires
// within one observer interval.
//
// Eligibility is PURE over three INJECTED readers, so it tests fully offline: the tick wires
// fixture-backed readers for the Verify table and forge/git-backed readers in production, and
// this function never knows which. It joins three reads and short-circuits on the first
// INELIGIBLE reading, cheapest source first (claim, board row, PR) — a defense-in-depth order
// the single-point-of-failure note in the brief depends on: the board row is read from
// origin/main BEFORE the (most expensive, most failure-prone) forge PR read, so a brief that
// flipped to implemented/verified/done is caught as ineligible even when the PR read fails,
// and the claim record is read first of all, so a claim someone released or stole is caught
// for THIS holder regardless of the PR.
//
// THREE-STATE. A read that cannot be performed (could-not-check) is NEVER rounded to eligible
// or to ineligible: Eligibility returns a *BlindError naming the source, and the tick keeps the
// run running, logs BLIND(<source>), and retries next tick — Symphony's "if state refresh
// fails, keep workers running" rule, which is also this house's three-state-instrument rule.

// Status is one brief row's lifecycle token as it stands on origin/main
// (todo / in-progress / implemented / verified / done / blocked).
type Status string

// activeStatuses are the ONLY two statuses under which a dispatched run is still eligible: a
// brief still to-do or in progress. Any other status — implemented, verified, done, blocked,
// or an unknown token a read could not classify — means the item is no longer the run's to
// work, so the run is ineligible-terminal. "not stated" deliberately does NOT read as active
// (the fail-open shape three-state exists to prevent): an empty/unrecognised status is a
// verdict of ineligible, not a pass-through to eligible.
func activeStatus(s Status) bool {
	switch Status(strings.ToLower(strings.TrimSpace(string(s)))) {
	case "todo", "in-progress":
		return true
	default:
		return false
	}
}

// PRState is the reconcile-relevant slice of one PR's forge state: its open/merged/closed
// kind, any terminal worker DISPOSITION recorded on it (SUPERSEDED / RESOLVED-ELSEWHERE), and
// its current labels (the held signals question / needs-decision). It reuses deskkit's
// canonical PRKind and DispositionVerdict vocabularies rather than re-deriving them.
type PRState struct {
	Kind        deskkit.PRKind
	Disposition deskkit.DispositionVerdict
	Labels      []string
}

// ClaimRecord is the identity of one dispatch claim the reconciliation reasons over: its key,
// the item it dispatched, the repo/root those reads are keyed on, the PR number to read, and
// the Holder the claim named at dispatch. The Claim reader re-reads the CURRENT holder; a
// mismatch means the claim was released or stolen underneath this run.
type ClaimRecord struct {
	Key    string
	Item   string
	Root   string
	Repo   string
	PR     int
	Holder string
}

// EligibilityReaders are the three injected reads Eligibility joins. Each returns its value
// or an error; an error is could-not-check for that source, which Eligibility wraps as a
// *BlindError and the caller treats as keep-the-run, never as a verdict.
type EligibilityReaders struct {
	// Claim reads the CURRENT claim record for a key (its live holder). Cheapest source.
	Claim func(key string) (ClaimRecord, error)
	// BoardRow reads the item's lifecycle status on origin/main under root.
	BoardRow func(root, item string) (Status, error)
	// PR reads the reconcile-relevant slice of a PR's forge state. Most expensive; the SPOF.
	PR func(repo string, n int) (PRState, error)
}

// eligibilityKind is the closed set of reconciliation verdicts.
type eligibilityKind int

const (
	kindEligible           eligibilityKind = iota // still the run's to work — leave it running
	kindIneligibleTerminal                        // stop + release + journal SUPERSEDED
	kindIneligibleHeld                            // stop, do NOT release — a human/other desk owns the next move
)

// EligibilityVerdict is one claim's reconciliation outcome. Terminal and Held both mean STOP;
// they differ only in whether the observer also RELEASES the claim (frees it for re-dispatch).
type EligibilityVerdict struct {
	kind   eligibilityKind
	Reason string
}

// Eligible is the verdict for a claim still eligible to run.
var Eligible = EligibilityVerdict{kind: kindEligible}

// IneligibleTerminal builds a terminal-ineligible verdict: the item is finished for this run,
// so the observer stops the run, releases the claim (making the item re-dispatchable), and
// journals SUPERSEDED. reason is a short machine-readable tag. Terminal is used ONLY when the
// item is genuinely FINISHED for this run and no other party owns its claim: pr-merged,
// pr-closed, a board row at implemented/verified/done, or claim-released (the ref is already
// gone, so the delete is a no-op). A claim that moved to a DIFFERENT live holder, or a blocked
// row, is Held, not Terminal — see IneligibleHeld.
func IneligibleTerminal(reason string) EligibilityVerdict {
	return EligibilityVerdict{kind: kindIneligibleTerminal, Reason: reason}
}

// IneligibleHeld builds a held-ineligible verdict: the run must stop, but the claim is NOT
// released — a human or another desk/holder owns the next move. Releasing it would either let a
// fresh worker re-dispatch straight into the held state, or (for claim-reassigned) DELETE a
// claim ref a DIFFERENT live holder now owns, re-freeing an item that holder is actively
// working — a double-dispatch. reason is one of superseded, resolved-elsewhere, needs-decision,
// question, claim-reassigned, board-row-blocked.
func IneligibleHeld(reason string) EligibilityVerdict {
	return EligibilityVerdict{kind: kindIneligibleHeld, Reason: reason}
}

// Eligible reports the still-run verdict.
func (v EligibilityVerdict) Eligible() bool { return v.kind == kindEligible }

// Ineligible reports whether the run must stop (either terminal or held).
func (v EligibilityVerdict) Ineligible() bool { return v.kind != kindEligible }

// Terminal reports the stop+release verdict.
func (v EligibilityVerdict) Terminal() bool { return v.kind == kindIneligibleTerminal }

// Held reports the stop-without-release verdict.
func (v EligibilityVerdict) Held() bool { return v.kind == kindIneligibleHeld }

// BlindError names the reconciliation source that could not be read this tick. The caller
// pulls Source out with BlindSource to log BLIND(<source>) and map the tick to exit 6 — the
// reading is incomplete even though the tick ran. It wraps the underlying read error so a
// caller that wants the cause can errors.Unwrap it.
type BlindError struct {
	Source string
	Err    error
}

func (e *BlindError) Error() string {
	if e.Err == nil {
		return "could-not-check: " + e.Source
	}
	return "could-not-check: " + e.Source + ": " + e.Err.Error()
}

func (e *BlindError) Unwrap() error { return e.Err }

// BlindSource reports the source name if err is (or wraps) a *BlindError. A caller uses it to
// distinguish a keep-the-run could-not-check (ok true) from a hard config error (ok false,
// abort the tick).
func BlindSource(err error) (source string, ok bool) {
	var be *BlindError
	if errors.As(err, &be) {
		return be.Source, true
	}
	return "", false
}

// Eligibility joins the three reads and returns the claim's reconciliation verdict, or a
// *BlindError for the first source that could not be read. Reads run cheapest first (claim,
// board row, PR) and short-circuit on the first INELIGIBLE reading — a defense-in-depth order
// the SPOF note relies on (the board row, read from origin/main, is checked before the forge
// PR read, so a flipped brief is caught even when the PR read would fail).
func Eligibility(c ClaimRecord, r EligibilityReaders) (EligibilityVerdict, error) {
	// 1. Claim (cheapest): does the claim ref still name this holder? A claim released or
	//    stolen underneath the run is ineligible for THIS holder regardless of the PR.
	cur, err := r.Claim(c.Key)
	if err != nil {
		return EligibilityVerdict{}, &BlindError{Source: "claim", Err: err}
	}
	if strings.TrimSpace(cur.Holder) == "" {
		// The ref is gone/empty: our claim is already released. STOP + release, but the
		// release is a no-op (nothing to delete), so there is no other holder to harm.
		return IneligibleTerminal("claim-released"), nil
	}
	if cur.Holder != c.Holder {
		// The ref has moved to a DIFFERENT live holder — the "stolen underneath the run"
		// case. STOP this run, but HELD, not Terminal: releasing here is an unconditional
		// delete-by-key (doReclaim/DeleteRef has no expected-holder guard), which would
		// delete the NEW holder's live claim and re-free an item they are working (a
		// double-dispatch). The next move is the new holder's, not ours.
		return IneligibleHeld("claim-reassigned"), nil
	}

	// 2. Board row (read from origin/main): a brief no longer in an active status
	//    (implemented / verified / done / blocked) is finished for this run — caught here,
	//    BEFORE the failure-prone PR read.
	st, err := r.BoardRow(c.Root, c.Item)
	if err != nil {
		return EligibilityVerdict{}, &BlindError{Source: "board-row", Err: err}
	}
	if !activeStatus(st) {
		stNorm := strings.ToLower(strings.TrimSpace(string(st)))
		if stNorm == "blocked" {
			// blocked is a human-HOLD state: a human owns the next move, so STOP without
			// releasing (Held). implemented/verified/done — and any unknown status a read
			// could not classify — are FINISHED for this run: Terminal (stop + release).
			return IneligibleHeld("board-row-blocked"), nil
		}
		return IneligibleTerminal("board-row-" + stNorm), nil
	}

	// 3. PR (the single point of failure): merged / closed is terminal; a recorded
	//    disposition or a held label is non-terminal (stop, do not release).
	pr, err := r.PR(c.Repo, c.PR)
	if err != nil {
		return EligibilityVerdict{}, &BlindError{Source: "pr", Err: err}
	}
	switch pr.Kind {
	case deskkit.PRMerged:
		return IneligibleTerminal("pr-merged"), nil
	case deskkit.PRClosed:
		return IneligibleTerminal("pr-closed"), nil
	}
	switch pr.Disposition {
	case deskkit.DispositionSuperseded:
		return IneligibleHeld("superseded"), nil
	case deskkit.DispositionResolvedElsewhere:
		return IneligibleHeld("resolved-elsewhere"), nil
	}
	if hasLabel(pr.Labels, "needs-decision") {
		return IneligibleHeld("needs-decision"), nil
	}
	if hasLabel(pr.Labels, "question") {
		return IneligibleHeld("question"), nil
	}
	return Eligible, nil
}

// hasLabel reports whether want is present in labels, case-insensitively and space-trimmed —
// the forge's own label casing must not decide whether a held run is caught.
func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}

// ParseBriefRowStatus reads one brief row's Status cell out of a stream board README, for the
// item "<stream>/<NN>". It is the PURE core of the live BoardRow reader (the git read that
// fetches the README at origin/main wraps it); keeping it pure is what lets the board-row
// parsing be unit-tested offline even though the read around it cannot be. The board table is
// `| # | Brief | Wave | Effort | Status | Verified | Reviewed |`, so the status is the 5th
// pipe-delimited cell of the row whose first cell equals NN. A row that cannot be found, or a
// malformed table, is an error the caller treats as could-not-check — never a guessed status.
func ParseBriefRowStatus(readme, item string) (Status, error) {
	nn := item
	if i := strings.LastIndex(item, "/"); i >= 0 {
		nn = item[i+1:]
	}
	nn = strings.TrimSpace(nn)
	if nn == "" {
		return "", errors.New("item " + item + " has no NN component")
	}
	for _, line := range strings.Split(readme, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitRowCells(line)
		if len(cells) < 5 {
			continue
		}
		if cells[0] != nn {
			continue
		}
		return Status(strings.ToLower(strings.TrimSpace(cells[4]))), nil
	}
	return "", errors.New("no brief row for " + nn + " in the board README")
}

// splitRowCells splits a Markdown table row on unescaped pipes and trims each cell. The
// leading and trailing empty cells produced by the border pipes are dropped, so cell[0] is
// the first real column.
func splitRowCells(line string) []string {
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	// Drop the empty leading/trailing cells from the border pipes.
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}
