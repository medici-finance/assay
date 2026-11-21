package evaluation

// outcome.go — the verification-outcome protocol: the ONE owner of how a verify-outcome record
// is named, parsed, classified and reduced (contracts S-evidence-coverage and
// S-verification-domain).
//
// Before this file the same rules were copied by hand into four places: the desk-tools record
// layer (tools/desk/internal/deskkit/verifyoutcomes.go), statusgen's own copy of it, statusgen's
// outcome classifier (awaiting_bucket.go), and the held-row reduction that the outcome writer
// (deskkit's verifyoutcomejoin.go) and the verifyloop queue reader (briefscan.go) each carried.
// The copies disagreed in the way copies do: the writer accepted any outcome word while the
// readers recognised six, and the closure guard compared one exact spelling while the readers
// compared case-insensitively. A record the writer admitted could therefore be one a reader
// could not interpret, or one that skipped the verified-closure check the readers would have
// run. This file owns the shared meaning; deskkit and statusgen keep only source and
// publication adapters (a filesystem root, a git tree, a forge write) that feed it explicit
// observations.
//
// PURITY. Nothing here opens a file, decodes JSON, runs a process or reads a clock or the
// environment: this file holds only the vocabulary, the brief-key grammar and the reductions.
// The record codec (JSON decode, the io/fs.FS reader, clock skew) lives in the outcomerecord
// subpackage, which builds on these rules. A value the caller cannot supply is
// reported as unavailable, never rounded to a pass. Reduction grants nothing: a record that
// classifies as a pass is a closure CANDIDATE, accepted only by the protected-closure check at
// the state-changing boundary, and no outcome ever demotes an accepted delivery.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// --- outcome vocabulary ------------------------------------------------------------------

// OutcomeClass is the meaning of a verify-outcome record's `outcome` value.
type OutcomeClass int

const (
	// OutcomeUnrecognised is a value no list names. A reader treats it as could-not-check and a
	// writer refuses to create it.
	OutcomeUnrecognised OutcomeClass = iota
	// OutcomePass is a verified outcome: a closure candidate, never an acceptance by itself.
	OutcomePass
	// OutcomeFail is a failed verification.
	OutcomeFail
	// OutcomeHold is a blocked verification, or one that needs context.
	OutcomeHold
)

// The wire spellings of the outcome field. Readers and the writer check also accept their case
// and surrounding-space variants (ClassifyOutcome), which older records carry.
const (
	OutcomeWordVerified     = "verified"
	OutcomeWordVerifyFail   = "verify-fail"
	OutcomeWordFail         = "fail"
	OutcomeWordBlocked      = "blocked"
	OutcomeWordNeedsContext = "needs_context"
	OutcomeWordNeedsDash    = "needs-context"
)

// outcomeWords is the closed wire vocabulary and the class each spelling means.
var outcomeWords = map[string]OutcomeClass{
	OutcomeWordVerified:     OutcomePass,
	OutcomeWordVerifyFail:   OutcomeFail,
	OutcomeWordFail:         OutcomeFail,
	OutcomeWordBlocked:      OutcomeHold,
	OutcomeWordNeedsContext: OutcomeHold,
	OutcomeWordNeedsDash:    OutcomeHold,
}

// OutcomeWords returns the closed wire vocabulary in a stable order.
func OutcomeWords() []string {
	out := make([]string, 0, len(outcomeWords))
	for w := range outcomeWords {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// String names the class for diagnostics.
func (c OutcomeClass) String() string {
	switch c {
	case OutcomePass:
		return "pass"
	case OutcomeFail:
		return "fail"
	case OutcomeHold:
		return "hold"
	}
	return "unrecognised"
}

// ClassifyOutcome is the READ-side classifier: case-insensitive, surrounding space ignored, so
// every record already on disk keeps the class it has always had. A value no list names is
// OutcomeUnrecognised.
func ClassifyOutcome(outcome string) OutcomeClass {
	return outcomeWords[strings.ToLower(strings.TrimSpace(outcome))]
}

// ParseOutcomeWord is the WRITE-side check. It accepts exactly what the readers recognise
// (ClassifyOutcome: a wire spelling, ignoring case and surrounding space) and refuses anything
// else, so a writer never creates a record a reader would classify as unrecognised. The record's
// bytes are kept as submitted; a recognised `verified` spelling still reaches the closure checks.
func ParseOutcomeWord(outcome string) (OutcomeClass, error) {
	if c := ClassifyOutcome(outcome); c != OutcomeUnrecognised {
		return c, nil
	}
	return OutcomeUnrecognised, fmt.Errorf("outcome %q must be one of %s (case and surrounding space ignored; a row result such as pass belongs in a row's result field, not the overall outcome)",
		outcome, strings.Join(OutcomeWords(), ", "))
}

// IsFailedOrHeld reports whether the value is a failed or held outcome — the ones a brief's
// wake scheduling governs. A verified outcome is the closure lane's concern.
func IsFailedOrHeld(outcome string) bool {
	c := ClassifyOutcome(outcome)
	return c == OutcomeFail || c == OutcomeHold
}

// IsNonDecisive reports whether the value is a held outcome (blocked or needs-context), the
// only kind that may be recorded for an already-verified delivery without Evidence.
func IsNonDecisive(outcome string) bool { return ClassifyOutcome(outcome) == OutcomeHold }

// NonDecisiveVerdictWord maps a held outcome to the Evidence verdict word a run with that outcome
// records (BLOCKED, NEEDS-CONTEXT); "" for a decisive or unrecognised outcome.
func NonDecisiveVerdictWord(outcome string) string {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case OutcomeWordBlocked:
		return "BLOCKED"
	case OutcomeWordNeedsContext, OutcomeWordNeedsDash:
		return "NEEDS-CONTEXT"
	}
	return ""
}

// Lifecycle is the accepted state of a delivery as the board holds it. Outcome records never
// change it; only the protected-closure rules at the state-changing boundary do.
type Lifecycle int

const (
	LifecycleOpen     Lifecycle = iota // not accepted
	LifecycleAccepted                  // verified or done
)

// Role is what an appended outcome observation is FOR.
type Role int

const (
	// RoleDiagnostic: an append-only observation. It never changes accepted lifecycle.
	RoleDiagnostic Role = iota
	// RoleClosureCandidate: a pass. It must clear the current protected-closure checks at the
	// boundary that would act on it; the record alone authorizes nothing.
	RoleClosureCandidate
	// RoleUnreadable: an outcome no reader can interpret; it is refused or surfaced as
	// could-not-check, never acted on.
	RoleUnreadable
)

// Disposition is the reduction of one observation against the delivery's accepted lifecycle.
type Disposition struct {
	Class OutcomeClass
	Role  Role
	// Next is the lifecycle after the observation. It always equals the prior lifecycle: a later
	// BLOCKED diagnostic neither demotes an accepted delivery nor, being a diagnostic, authorizes
	// a new verified flip; a pass is only a candidate until the closure check accepts it.
	Next Lifecycle
	// NeedsClosure is true exactly when the observation must be checked against the protected
	// closure before anything acts on it.
	NeedsClosure bool
}

// Dispose classifies one appended outcome against the prior accepted lifecycle.
func Dispose(prior Lifecycle, outcome string) Disposition {
	d := Disposition{Class: ClassifyOutcome(outcome), Next: prior}
	switch d.Class {
	case OutcomePass:
		d.Role, d.NeedsClosure = RoleClosureCandidate, true
	case OutcomeFail, OutcomeHold:
		d.Role = RoleDiagnostic
	default:
		d.Role = RoleUnreadable
	}
	return d
}

// --- row identity and row results --------------------------------------------------------

// Row result vocabulary of a verify-wake receipt's row_results entries.
const (
	RowPass        = "pass"
	RowFail        = "fail"
	RowCouldNotRun = "could-not-run"
	RowDeferred    = "deferred"
)

// ValidRowResult reports whether s is in the closed row-result set.
func ValidRowResult(s string) bool {
	switch s {
	case RowPass, RowFail, RowCouldNotRun, RowDeferred:
		return true
	}
	return false
}

var (
	rowNumberPattern = regexp.MustCompile(`^[1-9][0-9]*[a-zA-Z]?$`)
	rowLabelPattern  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)
)

// ParseRowID validates a Verify row identity: a positive integer with an optional letter suffix
// (6, 6a) or a letter-led label (R, S1). Anything else (0, 06, R-1, prose) is refused.
func ParseRowID(s string) (string, bool) {
	if rowLabelPattern.MatchString(s) {
		return s, true
	}
	if !rowNumberPattern.MatchString(s) {
		return "", false
	}
	digits := s
	if last := s[len(s)-1]; last < '0' || last > '9' {
		digits = s[:len(s)-1]
	}
	if _, err := strconv.Atoi(digits); err != nil {
		return "", false
	}
	return s, true
}

// RowResultEntry is one row_results entry: an exact row identity and its result.
type RowResultEntry struct {
	Row    string
	Result string
}

// RowSet is the explicit observation the row reduction reads. Presence matters: a count that is
// absent is not a count of zero.
type RowSet struct {
	Held       []string // the receipt's `rows`: the rows it holds
	HasTotal   bool
	Total      int // rows_total
	HasPassed  bool
	Passed     int // rows_passed
	HasResults bool
	Results    []RowResultEntry // row_results; covers ALL rows of the brief
	// BoundToInventory makes the reduction also reconcile the receipt with the brief's CURRENT
	// Verify table (a queue reader's question). A writer has no table and leaves it false.
	BoundToInventory bool
	Inventory        []string // the current Verify table's row identities, in order
}

// RowsState says how much row evidence a receipt carries.
type RowsState int

const (
	// RowsUnavailable: no counts and no row_results. Nothing is known about any row.
	RowsUnavailable RowsState = iota
	// RowsCountOnly: counts without per-row results.
	RowsCountOnly
	// RowsExact: per-row results for every row.
	RowsExact
)

// RowReduction is the typed result of reducing a RowSet.
type RowReduction struct {
	State RowsState
	// Held is the declared held set, exactly as written.
	Held []string
	// Passed and NonPass are the OBSERVED row results, by exact identity and in entry order.
	// NonPass is never derived from Held: an empty Held cannot erase a failed, could-not-run or
	// deferred row, and a held row is not assumed to have failed.
	Passed  []string
	NonPass []string
	// Runnable is the current table's rows that are neither held nor already passed. Set only
	// when the set was bound to an inventory.
	Runnable []string
	// WholeBrief: an empty held set names the whole brief (wake-v1 receipts do).
	WholeBrief bool
	// Complete: every row is observed passing and none is held. False for any unavailable,
	// count-only-short or non-pass state.
	Complete bool
}

// FaultKind classifies why a RowSet cannot be reduced.
type FaultKind int

const (
	FaultPassedExceedsTotal FaultKind = iota + 1
	FaultHeldRepeats
	FaultPassedExceedsNonHeld
	FaultResultsNeedTotal
	FaultResultsCount
	FaultResultRepeats
	FaultResultInvalid
	FaultHeldClaimsPass
	FaultPassCount
	FaultHeldAbsent
	FaultHeldOutsideInventory
	FaultInventoryUnavailable
	FaultInventoryInvalid
	FaultHeldUnknown
	FaultCoverage
	FaultResultUnknown
	FaultCountsDisagree
	FaultCountOnlyNeedsResults
)

// RowFault is a refusal naming the first thing wrong. Text is the single wording writers prefix
// with "refused: " and readers surface as could-not-check.
type RowFault struct {
	Kind        FaultKind
	Row         string
	Index       int
	Got, Wanted int
}

// Error implements error.
func (f *RowFault) Error() string { return f.Text() }

// Text is the fault's one wording.
func (f *RowFault) Text() string {
	switch f.Kind {
	case FaultPassedExceedsTotal:
		return fmt.Sprintf("rows_passed %d exceeds rows_total %d", f.Got, f.Wanted)
	case FaultHeldRepeats:
		return "rows repeats row " + f.Row
	case FaultPassedExceedsNonHeld:
		return fmt.Sprintf("rows_passed %d exceeds non-held rows %d", f.Got, f.Wanted)
	case FaultResultsNeedTotal:
		return "row_results needs rows_total (it covers ALL rows of the brief)"
	case FaultResultsCount:
		return fmt.Sprintf("row_results lists %d rows but rows_total is %d — it covers ALL rows", f.Got, f.Wanted)
	case FaultResultRepeats:
		return fmt.Sprintf("row_results[%d].row %s repeats", f.Index, f.Row)
	case FaultResultInvalid:
		return fmt.Sprintf("row_results[%d].result must be one of pass, fail, could-not-run, deferred", f.Index)
	case FaultHeldClaimsPass:
		return "held row " + f.Row + " also claims PASS"
	case FaultPassCount:
		return fmt.Sprintf("row_results holds %d pass rows but rows_passed is %d", f.Got, f.Wanted)
	case FaultHeldAbsent:
		return "held row " + f.Row + " is absent from row_results inventory"
	case FaultHeldOutsideInventory:
		return "held row " + f.Row + " is outside count-only inventory 1..rows_total"
	case FaultInventoryUnavailable:
		return "Verify row identities unavailable"
	case FaultInventoryInvalid:
		return fmt.Sprintf("invalid or duplicate Verify row %q", f.Row)
	case FaultHeldUnknown:
		return fmt.Sprintf("held row %q is not in the current Verify table", f.Row)
	case FaultCoverage:
		return "row_results does not cover the current Verify table"
	case FaultResultUnknown:
		return fmt.Sprintf("unknown, duplicate or invalid result row %q", f.Row)
	case FaultCountsDisagree:
		return "receipt counts disagree with the current Verify table"
	case FaultCountOnlyNeedsResults:
		return "non-contiguous, suffixed or label rows require exact row_results"
	}
	return "row evidence cannot be reduced"
}

// ReduceRows is the ONE reduction of row_results, held rows, counts and (optionally) the
// current Verify table. A writer calls it unbound to refuse a record no reader could use; a
// queue reader calls it bound to pick the rows still runnable. Both therefore agree on what a
// receipt means.
func ReduceRows(in RowSet) (RowReduction, *RowFault) {
	red := RowReduction{Held: append([]string(nil), in.Held...)}
	if !in.BoundToInventory && in.HasPassed && in.HasTotal && in.Passed > in.Total {
		return red, &RowFault{Kind: FaultPassedExceedsTotal, Got: in.Passed, Wanted: in.Total}
	}
	held := map[string]bool{}
	for _, r := range in.Held {
		if held[r] {
			return red, &RowFault{Kind: FaultHeldRepeats, Row: r}
		}
		held[r] = true
	}
	switch {
	case in.HasResults:
		red.State = RowsExact
	case in.HasTotal || in.HasPassed:
		red.State = RowsCountOnly
	}
	if !in.BoundToInventory && in.HasPassed && in.HasTotal && in.Passed > in.Total-len(held) {
		return red, &RowFault{Kind: FaultPassedExceedsNonHeld, Got: in.Passed, Wanted: in.Total - len(held)}
	}

	known := map[string]bool{}
	if in.BoundToInventory {
		// An empty held set names the whole brief: nothing to reconcile, and nothing runnable
		// is withheld. Observed non-pass rows stay visible on the reduction regardless.
		if len(in.Held) == 0 {
			red.WholeBrief = true
			for _, e := range in.Results {
				if e.Result == RowPass {
					red.Passed = append(red.Passed, e.Row)
				} else {
					red.NonPass = append(red.NonPass, e.Row)
				}
			}
			return red, nil
		}
		for _, id := range in.Inventory {
			if _, ok := ParseRowID(id); !ok || known[id] {
				return red, &RowFault{Kind: FaultInventoryInvalid, Row: id}
			}
			known[id] = true
		}
		if len(known) == 0 {
			return red, &RowFault{Kind: FaultInventoryUnavailable}
		}
		for _, id := range in.Held {
			if !known[id] {
				return red, &RowFault{Kind: FaultHeldUnknown, Row: id}
			}
		}
	}

	passedSet := map[string]bool{}
	if in.HasResults {
		if in.BoundToInventory {
			if in.Total != len(in.Inventory) || len(in.Results) != len(in.Inventory) {
				return red, &RowFault{Kind: FaultCoverage}
			}
		} else {
			if !in.HasTotal {
				return red, &RowFault{Kind: FaultResultsNeedTotal}
			}
			if len(in.Results) != in.Total {
				return red, &RowFault{Kind: FaultResultsCount, Got: len(in.Results), Wanted: in.Total}
			}
		}
		seen := map[string]bool{}
		for i, e := range in.Results {
			if in.BoundToInventory {
				if !known[e.Row] || seen[e.Row] || !ValidRowResult(e.Result) {
					return red, &RowFault{Kind: FaultResultUnknown, Row: e.Row}
				}
			} else {
				if seen[e.Row] {
					return red, &RowFault{Kind: FaultResultRepeats, Index: i, Row: e.Row}
				}
				if !ValidRowResult(e.Result) {
					return red, &RowFault{Kind: FaultResultInvalid, Index: i}
				}
			}
			seen[e.Row] = true
			if e.Result == RowPass {
				if held[e.Row] {
					return red, &RowFault{Kind: FaultHeldClaimsPass, Row: e.Row}
				}
				passedSet[e.Row] = true
				red.Passed = append(red.Passed, e.Row)
			} else {
				red.NonPass = append(red.NonPass, e.Row)
			}
		}
		if (in.HasPassed || in.BoundToInventory) && len(passedSet) != in.Passed {
			return red, &RowFault{Kind: FaultPassCount, Got: len(passedSet), Wanted: in.Passed}
		}
		if !in.BoundToInventory {
			for _, r := range in.Held {
				if !seen[r] {
					return red, &RowFault{Kind: FaultHeldAbsent, Row: r}
				}
			}
		}
	} else if in.HasTotal && !in.BoundToInventory {
		// Without exact results only the contiguous numeric inventory 1..total is known. Never
		// infer a suffix or a foreign held identity.
		for _, r := range in.Held {
			n, err := strconv.Atoi(r)
			if err != nil || n > in.Total {
				return red, &RowFault{Kind: FaultHeldOutsideInventory, Row: r}
			}
		}
	}

	if in.BoundToInventory {
		for _, id := range in.Inventory {
			if !held[id] {
				red.Runnable = append(red.Runnable, id)
			}
		}
		if in.HasResults {
			var rest []string
			for _, id := range red.Runnable {
				if !passedSet[id] {
					rest = append(rest, id)
				}
			}
			red.Runnable = rest
		} else if in.HasTotal || in.HasPassed {
			// Older numeric-only receipts prove completion only when ALL non-held rows passed,
			// with exact totals over a contiguous numeric table.
			if in.Total != len(in.Inventory) || in.Passed < 0 || in.Passed > len(red.Runnable) {
				return red, &RowFault{Kind: FaultCountsDisagree}
			}
			for i, id := range in.Inventory {
				if id != strconv.Itoa(i+1) {
					return red, &RowFault{Kind: FaultCountOnlyNeedsResults}
				}
			}
			if in.Passed == len(red.Runnable) {
				red.Runnable = nil
			}
		}
	}

	switch red.State {
	case RowsExact:
		red.Complete = len(red.NonPass) == 0 && len(in.Held) == 0 && len(red.Passed) > 0
	case RowsCountOnly:
		red.Complete = len(in.Held) == 0 && in.HasTotal && in.HasPassed && in.Total > 0 && in.Passed == in.Total
	}
	return red, nil
}

// RunVerdict is the overall reading of one verification run: its outcome class against its rows.
type RunVerdict int

const (
	// VerdictUnreadable: the outcome is no recognised value. Could-not-check; never acted on.
	VerdictUnreadable RunVerdict = iota
	// VerdictCandidate: a pass with no held or observed non-pass row. A closure candidate only.
	VerdictCandidate
	// VerdictContradiction: a pass that holds a row or carries a non-pass row. Row evidence
	// contradicts the overall outcome, so it is refused rather than reduced to either side.
	VerdictContradiction
	// VerdictRowBlocked: a fail or hold that names at least one held or non-pass row.
	VerdictRowBlocked
	// VerdictClosureOnly: a fail or hold whose rows are all observed passing. The blocker is the
	// closure, not a row; "all rows passed" is not a verified outcome.
	VerdictClosureOnly
	// VerdictRowsUnavailable: a fail or hold with no row evidence at all.
	VerdictRowsUnavailable
)

// RunReduction is the typed reduction of one run.
type RunReduction struct {
	Class   OutcomeClass
	Rows    RowReduction
	Verdict RunVerdict
}

// ReduceRun reduces an outcome word and its row evidence together. A row fault is returned as
// the fault; it is never folded into a verdict.
func ReduceRun(outcome string, rows RowSet) (RunReduction, *RowFault) {
	red, fault := ReduceRows(rows)
	run := RunReduction{Class: ClassifyOutcome(outcome), Rows: red}
	if fault != nil {
		return run, fault
	}
	nonPass := len(red.NonPass) > 0 || len(red.Held) > 0
	switch run.Class {
	case OutcomePass:
		if nonPass {
			run.Verdict = VerdictContradiction
		} else {
			run.Verdict = VerdictCandidate
		}
	case OutcomeFail, OutcomeHold:
		switch {
		case nonPass:
			run.Verdict = VerdictRowBlocked
		case red.State == RowsUnavailable:
			run.Verdict = VerdictRowsUnavailable
		default:
			run.Verdict = VerdictClosureOnly
		}
	default:
		run.Verdict = VerdictUnreadable
	}
	return run, nil
}

// --- brief-key grammar -------------------------------------------------------------------

var (
	outcomeStreamRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	outcomeNumRe    = regexp.MustCompile(`^[0-9]+$`)
)

// SplitBriefKey splits a "<stream>/<NN>" brief key, refusing (never sanitising) any other shape.
func SplitBriefKey(brief string) (stream, num string, err error) {
	parts := strings.SplitN(brief, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid brief key %q: want <stream>/<NN>", brief)
	}
	stream, num = parts[0], parts[1]
	if !outcomeStreamRe.MatchString(stream) {
		return "", "", fmt.Errorf("invalid brief stream %q in key %q", stream, brief)
	}
	if !outcomeNumRe.MatchString(num) {
		return "", "", fmt.Errorf("invalid brief number %q in key %q", num, brief)
	}
	return stream, num, nil
}
