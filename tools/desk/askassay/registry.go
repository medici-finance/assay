package askassay

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Source is the ONE declared origin of a number — derive-or-diff: one number,
// one source. All four fields are mandatory. A source that cannot state its
// limit is a silent cap by definition, so an empty Limit is invalid rather
// than merely undocumented; a source with no cap says so in words.
type Source struct {
	// Cmd is the command line the figure came from, verbatim, with the
	// placeholders the caller substitutes left in angle brackets.
	Cmd string
	// Probe is how the number is extracted from that command's output.
	Probe string
	// Window is the span the number covers.
	Window string
	// Limit is the cap the probe method imposes on the answer — the paging
	// cap, the scope of the walk, the horizon of the log. "none — <why>" is a
	// valid limit; "" is not.
	Limit string
}

// Validate reports why a source cannot back a rendered figure.
func (s Source) Validate() error {
	var missing []string
	for _, f := range []struct{ name, val string }{
		{"command", s.Cmd}, {"probe", s.Probe}, {"window", s.Window}, {"limit", s.Limit},
	} {
		if strings.TrimSpace(f.val) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("source declares no %s", strings.Join(missing, ", no "))
	}
	return nil
}

// Class groups questions by the way their numbers lie. Each class carries
// mandatory caveats (see requiredCaveats) that every question in it inherits,
// so a new question cannot be added without them.
type Class string

const (
	// ClassInventory counts live objects on the forge. They move under you.
	ClassInventory Class = "inventory"
	// ClassBoard counts rows on the declared brief board. A row's status is
	// not the state of the work.
	ClassBoard Class = "board"
	// ClassCompletion answers "how much is done". The board's own
	// verified/done marks are self-attributed.
	ClassCompletion Class = "completion"
	// ClassFlow answers throughput/dwell questions off the transition log.
	ClassFlow Class = "flow"
	// ClassPRState answers PR questions off the two board verbs, neither of
	// which alone forms a (repo, pr, head, verb) key.
	ClassPRState Class = "pr-state"
)

// The standing caveats. Each is a measured property of this system, not a
// disclaimer: every one of them was observed producing a wrong answer.
const (
	// CaveatDrift — measured: a live open-issue count moved 244 -> 242 inside
	// 7.5 minutes, and a board-drift figure was substantially wrong within the
	// hour it was computed.
	CaveatDrift = "counts move while you read them — this figure is true only at the as-of stamp on the line above; a live open-issue count was measured moving 244 to 242 inside 7.5 minutes"

	// CaveatBlindNotIdle — measured: ~16+ concurrent operations on one shared
	// token trip the forge's secondary rate limit and return 403.
	CaveatBlindNotIdle = "an empty result from this probe is BLIND, not idle — concurrent operations on one token trip a secondary rate limit and return 403, and a throttled probe renders could-not-check rather than 0"

	// CaveatSelfVerified — measured: of 141 board rows at verified or done,
	// 92 (65%) carried Evidence written by the identity that implemented them.
	CaveatSelfVerified = "verified and done on this board are SELF-ATTRIBUTED — a measured 92 of 141 such rows (65%) carried Evidence written by the identity that implemented the row. Corroboration is a separate probe (statusgen --corroborate --pr <N>); until it runs this figure is claimed, not confirmed"

	// CaveatBoardLies — measured: rows read todo while their PR had merged,
	// and one row read todo while the work had landed under a differently
	// named branch.
	CaveatBoardLies = "board status is not work state — rows have been measured reading todo while their PR had merged, and reading todo while the work had landed on a differently named branch. This is a count of ROWS, not of unstarted work"

	// CaveatEvidencePresence — measured: an Evidence table asserted a result
	// for a Verify row that could not execute (a shredded markdown cell
	// truncated the command; an escaped alternation matched nothing).
	CaveatEvidencePresence = "the presence of an Evidence row is not evidence — rows have been measured asserting a result for a command that cannot execute, because the markdown cell truncated it or its alternation matched nothing"

	// CaveatNoHeadSHA — measured at cmd/deskboard/board.go, the actionRow
	// struct: no head SHA field.
	CaveatNoHeadSHA = "the action rows behind this figure carry NO head SHA, so no action can be keyed to the commit it was computed against; a head SHA exists only after a join against the PR inventory, and rows that will not join are reported UNRESOLVED, never dropped and never counted as clean"

	// CaveatNoReviewState — measured at cmd/deskboard/board.go, the prRow
	// struct: no review-state field.
	CaveatNoReviewState = "the PR rows behind this figure carry NO review state, so this number says nothing about whether a review exists, passed, or has gone stale against the current head"

	// CaveatScopedSet — the board verbs walk a watched repo set, not the world.
	CaveatScopedSet = "this figure covers the watched repo set only — a PR or issue outside that set is not counted as zero, it is unscoped, and the scope line of the source command is part of the answer"
)

// requiredCaveats is the per-class floor. A question in a class WITHOUT its
// class's caveats is a registry error, enforced by
// TestEveryQuestionCarriesItsClassCaveats.
var requiredCaveats = map[Class][]string{
	ClassInventory:  {CaveatDrift, CaveatBlindNotIdle},
	ClassBoard:      {CaveatBoardLies},
	ClassCompletion: {CaveatSelfVerified, CaveatBoardLies, CaveatEvidencePresence},
	ClassFlow:       {CaveatDrift},
	ClassPRState:    {CaveatBlindNotIdle, CaveatScopedSet},
}

// Question is one thing an operator can ask, bound to exactly one source.
type Question struct {
	// ID is the registry key and the token that appears in a rendered answer.
	ID string
	// Text is the operator's question in words — what the pane matches on.
	Text string
	// Class selects the mandatory caveats.
	Class Class
	// Source is the ONE declared origin of this number.
	Source Source

	// SaturatesAt is the cap at which this question's count stops being a
	// count. When a probe returns a value at or above it, [Computed]
	// downgrades to could-not-check, because a full page and a truncated page
	// are the same bytes. 0 means the source has no row cap — which is a
	// claim the Limit field has to justify in words.
	SaturatesAt int64

	// EmptyMeansZero declares that an empty payload from THIS source is a
	// genuine zero. The default is false, because an empty result is blind,
	// not idle. Setting it true requires EmptyRationale.
	EmptyMeansZero bool
	// EmptyRationale is why an empty payload here is trustworthy.
	EmptyRationale string

	// Caveats are qualifications specific to this question, on top of its
	// class's mandatory set.
	Caveats []string
}

func (q Question) allCaveats() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range append(append([]string(nil), requiredCaveats[q.Class]...), q.Caveats...) {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// Validate reports why a question cannot be served.
func (q Question) Validate() error {
	if strings.TrimSpace(q.ID) == "" {
		return errors.New("question has no ID")
	}
	if strings.TrimSpace(q.Text) == "" {
		return fmt.Errorf("%s: question has no operator-facing text", q.ID)
	}
	if _, ok := requiredCaveats[q.Class]; !ok {
		return fmt.Errorf("%s: class %q is not a declared class", q.ID, q.Class)
	}
	if err := q.Source.Validate(); err != nil {
		return fmt.Errorf("%s: %w", q.ID, err)
	}
	if q.EmptyMeansZero && strings.TrimSpace(q.EmptyRationale) == "" {
		return fmt.Errorf("%s: declares an empty result to be a real zero but gives no rationale — an empty result is blind unless something says why it is not", q.ID)
	}
	if !q.EmptyMeansZero && strings.TrimSpace(q.EmptyRationale) != "" {
		return fmt.Errorf("%s: carries an empty-result rationale but does not declare EmptyMeansZero", q.ID)
	}
	have := map[string]bool{}
	for _, c := range q.allCaveats() {
		have[c] = true
	}
	for _, c := range requiredCaveats[q.Class] {
		if !have[c] {
			return fmt.Errorf("%s: class %s requires a caveat it does not carry", q.ID, q.Class)
		}
	}
	return nil
}

// registry is the closed set of question classes this pane can answer. A
// question that is not here has no source, and a figure with no source does
// not render — so the way to make the pane answer something new is to declare
// where the number comes from, not to let the model estimate it.
var registry = map[string]Question{

	"open-issue-count": {
		ID:    "open-issue-count",
		Text:  "how many issues are open in <owner>/<repo>?",
		Class: ClassInventory,
		Source: Source{
			Cmd:    `gh api graphql -f query='query($o:String!,$r:String!){repository(owner:$o,name:$r){issues(states:OPEN){totalCount}}}' -F o=<owner> -F r=<repo>`,
			Probe:  "data.repository.issues.totalCount, a server-side count",
			Window: "instantaneous — open at the moment the call returned, and only then",
			Limit:  "none — a server-side totalCount is not a paged list, so it cannot truncate. This is why the count question does NOT use the list probe below",
		},
		EmptyMeansZero: false,
	},

	"open-issue-list": {
		ID:    "open-issue-list",
		Text:  "which issues are open in <owner>/<repo>?",
		Class: ClassInventory,
		Source: Source{
			Cmd:    `gh issue list -R <owner>/<repo> --state open --limit 1000 --json number,title`,
			Probe:  "the length of the returned array",
			Window: "instantaneous",
			Limit:  "1000 rows, applied SILENTLY — this call truncates at --limit and reports no truncation of any kind. A measured pull of this shape returned exactly 500 rows against a true total of 958. len(rows) at the cap is indistinguishable from 'more than the cap', so it is refused rather than reported",
		},
		SaturatesAt: 1000,
		Caveats: []string{
			"this list is for enumerating, never for totalling — ask open-issue-count for a total, which reads a server-side count with no paging cap",
		},
	},

	"brief-status-count": {
		ID:    "brief-status-count",
		Text:  "how many briefs are at <status>?",
		Class: ClassBoard,
		Source: Source{
			Cmd:    `statusgen --root <root> --json`,
			Probe:  "count of brief entries whose status field equals <status>",
			Window: "the working tree at <sha> — a board file, not the forge",
			Limit:  "none on rows — every brief file under docs/streams is walked, with no server paging. The limit is one of CURRENCY, not of count: the board is only as current as its last regeneration",
		},
		EmptyMeansZero: false,
		Caveats: []string{
			"a count of todo rows is NOT a count of unstarted work, and this is the specific figure most likely to be quoted as if it were",
		},
	},

	"completion-count": {
		ID:    "completion-count",
		Text:  "how much of <stream> is finished?",
		Class: ClassCompletion,
		Source: Source{
			Cmd:    `statusgen --root <root> --json`,
			Probe:  "count of brief entries whose status field is verified or done",
			Window: "the working tree at <sha>",
			Limit:  "none on rows — the limit here is one of MEANING, not of paging: the board records who stamped a row, not whether anyone independent checked it",
		},
		EmptyMeansZero: false,
	},

	"flow-throughput": {
		ID:    "flow-throughput",
		Text:  "how much is moving through the system, and how fast?",
		Class: ClassFlow,
		Source: Source{
			Cmd:    `statusgen --root <root> --dora --json --since <YYYY-MM-DD>`,
			Probe:  "the named metric field of the emitted object",
			Window: "<since> to now, from the status-transition log",
			Limit:  "the transition log's own horizon — a transition that happened before the log's first entry is INVISIBLE to this probe and is not a zero. The log is append-only and written by main CI, so anything that moved without CI recording it is outside the window",
		},
	},

	"alarm-count": {
		ID:    "alarm-count",
		Text:  "how many alarms are standing?",
		Class: ClassFlow,
		Source: Source{
			Cmd:    `statusgen --root <root> --alarms`,
			Probe:  "the active-unresolved count from the alarm KPI block",
			Window: "the findings register at <sha>",
			Limit:  "none on rows — the register is read whole. Bounded to findings that were WRITTEN DOWN; an unrecorded problem is not a zero alarm",
		},
	},

	// WITHDRAWN: "bottleneck-stage" — "where is the constraint right now?"
	//
	// This question was declared against `statusgen --root <root> --bottleneck`
	// on the belief that it was a diagnostic read. It is not. Its runner writes
	// a dated report file unconditionally, with no flag that suppresses the
	// write, and the directory it writes into carries no publication-manifest
	// row — so the probe both breaks this pane's read-only property and leaves
	// an unclassified artefact behind. The mode is now in statusgenWriteModes.
	//
	// It is WITHDRAWN rather than re-sourced because there is no read-only
	// command that answers it today. That is the honest end state under the
	// numbers rule: a question with no read-only source has no source, so it
	// has no figure. Asking it now routes through [Unanswerable] and renders
	// could-not-check with a stated reason, which is the correct answer — a
	// figure quietly recomputed from a stale report file would not be.
	//
	// Re-declaring it needs a statusgen mode that computes the report without
	// writing it (or a manifest-classified read of an existing report), plus
	// the caveat that a report file is only as current as the run that wrote it.

	"code-efficiency": {
		ID:    "code-efficiency",
		Text:  "how much code changed, and how concentrated was it?",
		Class: ClassFlow,
		Source: Source{
			Cmd:    `statusgen --root <root> --code --json --since <YYYY-MM-DD>`,
			Probe:  "the named metric field of the emitted object",
			Window: "<since> to now, over the git history reachable from the checked-out ref",
			Limit:  "the clone's history — a shallow clone, or work on an unfetched branch, reads as no churn rather than as unknown churn. Check the clone depth before quoting this",
		},
	},

	"gate-score": {
		ID:    "gate-score",
		Text:  "what should be picked up next?",
		Class: ClassBoard,
		Source: Source{
			Cmd:    `statusgen --root <root> --gate-scores`,
			Probe:  "the score field of the emitted awaiting-queue entries",
			Window: "the working tree at <sha>",
			Limit:  "the awaiting queue only — a brief that is not eligible has no score, which is not the same as a score of zero",
		},
	},

	"pr-action-count": {
		ID:    "pr-action-count",
		Text:  "how many PRs are waiting on an action?",
		Class: ClassPRState,
		Source: Source{
			Cmd:    `deskboard actions --json`,
			Probe:  "count of action rows whose action field equals <action>",
			Window: "instantaneous, over the watched repo set",
			Limit:  "the watched repo set, and the forge's own paging behind the verb. A 403 from the secondary rate limit surfaces as an empty or short set with a non-zero exit — which is why an empty payload here is could-not-check",
		},
		Caveats: []string{CaveatNoHeadSHA},
	},

	"pr-inventory-count": {
		ID:    "pr-inventory-count",
		Text:  "how many PRs are open?",
		Class: ClassPRState,
		Source: Source{
			Cmd:    `deskboard prs --json`,
			Probe:  "the length of the returned PR array",
			Window: "instantaneous, over the watched repo set",
			Limit:  "the watched repo set. This verb DOES carry a head SHA per row; it is the only place one comes from",
		},
		Caveats: []string{CaveatNoReviewState},
	},

	"awaiting-count": {
		ID:    "awaiting-count",
		Text:  "what is waiting on a human?",
		Class: ClassPRState,
		Source: Source{
			Cmd:    `deskboard awaiting --json`,
			Probe:  "the length of the returned awaiting array",
			Window: "instantaneous, over the watched repo set",
			Limit:  "the watched repo set. An item waiting on a human OUTSIDE that set is not counted and is not zero",
		},
	},

	"health-state": {
		ID:    "health-state",
		Text:  "is the board healthy?",
		Class: ClassPRState,
		Source: Source{
			Cmd:    `deskboard health --json`,
			Probe:  "the emitted health verdict and its failing-check count",
			Window: "instantaneous, over the watched repo set",
			Limit:  "the checks the verb implements — a class of failure it does not model reads as healthy, which is the reason this answer names the check set rather than only the verdict",
		},
	},
}

// Lookup returns a declared question. A question that is not declared has no
// source, and a figure with no source does not render — so the miss is
// reported, never guessed at.
func Lookup(id string) (Question, bool) {
	q, ok := registry[id]
	return q, ok
}

// Questions returns every declared question, ID-sorted.
func Questions() []Question {
	out := make([]Question, 0, len(registry))
	for _, q := range registry {
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Unanswerable builds the answer for a question the registry does not
// declare. It exists so that "I do not have a source for that" is a rendered
// could-not-check with a stamp, rather than the model filling the gap.
func Unanswerable(id, asked string, st Stamp) Answer {
	return Answer{
		question: id,
		state:    CouldNotCheck,
		stamp:    st,
		reason:   fmt.Sprintf("no declared source for %q — this pane answers only from the question registry, and a figure with no query behind it does not render", asked),
	}
}
