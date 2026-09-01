package main

// inventory_test.go — the STRUCTURAL half of the subset/absence cluster (#400 round 2).
//
// Every finding in this cluster is one instance of the same shape: a limited view
// rendered as a complete one. Round 1 pinned the named instances one test at a time,
// and round 2 found more members of the same family — guards that were untested
// BECAUSE THE FIXTURE COULD NOT REPRESENT THE STATE THEY DEFEND AGAINST. Five rows of
// the classifier table were named "approved GREEN" and left `ciGreen` at false; the
// FLIP arm asserting "CI green" was therefore reachable only through those rows, and
// the suite stayed green while the live board printed a CI verdict it had never read.
//
// One test at a time cannot close that. These three enumerate the population instead:
//
//	TestClassify_ActionInventory  — every ACTION verb the classifier defines must be
//	                                produced by at least one REAL payload, driven through
//	                                the same buildClassifyInput the board uses. An action
//	                                nothing can produce, or one produced that the table
//	                                does not know about, fails. This is the positive
//	                                control: an absence assertion here cannot pass
//	                                vacuously, because the same enumeration proves the
//	                                inputs reach every arm.
//	TestDispatch_VerbInventory    — every verb in the dispatcher (parsed from main.go's
//	                                case labels, not hand-listed) must be classified as
//	                                sweeping-repos / sweeping-roots / non-sweeping /
//	                                refused, and is then RUN and checked against that
//	                                classification. A verb classified nowhere fails.
//	TestReadme_VerbParity         — the README's enumerated deskboard verb list is the
//	                                part that rots. It is compared against the dispatcher.

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// 1. the ACTION inventory
// ---------------------------------------------------------------------------

// actionConstsFromSource returns every ACTION verb string the classifier defines, read
// from board.go's `act*` const block. Parsed rather than hand-listed for the reason the
// whole cluster exists: a hand-kept second copy of a list drifts, and the drift is
// invisible — a new action would simply never be asserted about.
func actionConstsFromSource(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "board.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing board.go: %v", err)
	}
	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
			return true
		}
		name := vs.Names[0].Name
		if !strings.HasPrefix(name, "act") {
			return true
		}
		lit, ok := vs.Values[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		out[name] = v
		return true
	})
	if len(out) == 0 {
		t.Fatal("no act* constants found in board.go — the inventory would assert nothing " +
			"(this is the vacuous-pass the test exists to prevent)")
	}
	return out
}

// actionsDeclaredUnproducible names any ACTION the classifier defines that NO PR payload
// can produce, with the reason. It is EMPTY on purpose: an entry here is a standing claim
// that a verb the board can print is unreachable, and it has to be argued, not assumed.
// The FLIP arm's history is why — it was reachable only through a state the board should
// never have called green, and no test could tell.
var actionsDeclaredUnproducible = map[string]string{}

// rollupFixture is one check-rollup shape PLUS the tallies it is DECLARED to produce.
// The declaration is the point (#400 Q2/Q4): round 2 enumerated these shapes but bound
// nothing to their outcome, and every absence assertion was conditioned on a DERIVED
// boolean (`in.ciGreen`) — so counting `SKIPPED`/`NEUTRAL` as a pass left the whole suite
// green while an only-skipped head classified MERGE-NOW with "CI green" in the note. A
// hand-declared tally is an independent statement about what the shape MEANS, and
// TestClassify_ActionInventory checks the real `ciState` against it.
type rollupFixture struct {
	json string // the `"statusCheckRollup":…,` fragment, or "" for no key at all
	// declared tallies. `SKIPPED`/`NEUTRAL` report NO verdict: they are not a pass, and
	// a head carrying only those has established nothing.
	pass, pending, fail, unknown int
}

// rollupFixtures are the check-rollup shapes a real head can carry, INCLUDING the two
// that carry no verdict at all — an absent field and an empty array — which is the R2
// state the board used to render as "CI green".
var rollupFixtures = map[string]rollupFixture{
	"absent": {json: ``}, // no statusCheckRollup key at all
	"empty":  {json: `"statusCheckRollup":[],`},
	"all-success": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}],`,
		pass: 1},
	"one-failure": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"FAILURE","name":"ci"}],`,
		fail: 1},
	"one-pending": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"IN_PROGRESS","name":"ci"}],`,
		pending: 1},
	// Q2: one COMPLETED check that reported no verdict. Zero of everything — a skipped
	// check is not a passing check, and this head has established nothing.
	"only-skipped": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SKIPPED","name":"ci"}],`},
	"only-neutral": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"NEUTRAL","name":"ci"}],`},
	"undecodable": {json: `"statusCheckRollup":[{"__typename":"SomethingNew","weird":true}],`,
		unknown: 1},
	// N8: COMPLETED, no conclusion. Unreadable, not failed — the board must not report
	// "a check FAILED" about a conclusion it never saw.
	"completed-no-conclusion": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","name":"ci"}],`,
		unknown: 1},
	"statuscontext-ok": {json: `"statusCheckRollup":[{"__typename":"StatusContext","state":"SUCCESS","context":"legacy"}],`,
		pass: 1},
	// T1: a legacy status context in state `EXPECTED` — declared and NOT yet reported.
	// This enumeration carried ONE StatusContext shape of five StatusState values, and the
	// one it left out was the one the reducer got wrong: a completeness claim that was a
	// subset, made by the table meant to end that class. statusStateBuckets below now holds
	// the enum itself.
	"only-expected": {json: `"statusCheckRollup":[{"__typename":"StatusContext","state":"EXPECTED","context":"legacy/required"}],`,
		pending: 1},
	"success-plus-fail": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"a"},{"__typename":"CheckRun","status":"COMPLETED","conclusion":"FAILURE","name":"b"}],`,
		pass: 1, fail: 1},
	"skipped-plus-success": {json: `"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SKIPPED","name":"a"},{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"b"}],`,
		pass: 1},
}

// statusStateBuckets is GitHub's StatusState enum in FULL, each value declared into the
// bucket `ciState` must put it in (#400 T1). It is the legacy-status twin of
// mergeStateFixtures: `rollupFixtures` enumerates rollup SHAPES, and a shape enumeration
// cannot say "these are all the values one field can hold" — which is how `EXPECTED`
// ("Status is expected", i.e. a required context that has not reported) spent this whole
// cluster mapped onto `pass` next to `SUCCESS` ("Status is successful"), unnoticed by a
// suite whose author believed the shapes were enumerated.
//
// Buckets, not booleans: the declaration is checked against the real `ciState`.
type ciBucket int

const (
	bucketPass ciBucket = iota
	bucketPending
	bucketFail
	bucketUnknown
)

var statusStateBuckets = map[string]ciBucket{
	"SUCCESS":  bucketPass,
	"PENDING":  bucketPending,
	"EXPECTED": bucketPending, // declared, never reported — a check still owed
	"FAILURE":  bucketFail,
	"ERROR":    bucketFail,
	// Not a StatusState: anything outside the enum is uninterpretable, never a pass.
	"MOONBEAM": bucketUnknown,
}

// TestCIState_EveryStatusStateIsBucketed_400T1 asserts the table above against the real
// reducer, and — the half that would have caught T1 — asserts the table is not itself a
// subset: every value board.go's own `State` field comment lists as one it expects to
// receive must appear here. A reducer and a doc comment that disagree about the value set
// is exactly the gap `EXPECTED` fell through.
// TestCIState_LatestRunPerName is the deskboard half of the #282/#289 fix: a superseded
// run of a check NAME must not count against a PR whose current run for that name is green,
// and a genuinely red LATEST run must still count. Without the reduction the board would
// render a push+pull_request double-triggered PR CI-fail while deskflip flips it ready —
// the board/flip divergence this class set out to end.
func TestCIState_LatestRunPerName(t *testing.T) {
	const tOld, tNew = "2026-01-01T00:00:00Z", "2026-01-01T01:00:00Z"
	cases := map[string]struct {
		rollup                       []check
		pass, pending, fail, unknown int
	}{
		// A cancelled predecessor + the re-triggered success (#289 shape): the newer
		// SUCCESS is the latest run, so the board reads one pass and zero fail.
		"cancelled predecessor + green latest": {
			rollup: []check{
				{Name: "changelog", Status: "COMPLETED", Conclusion: "CANCELLED", CompletedAt: tOld},
				{Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: tNew},
			},
			pass: 1,
		},
		// A stale QUEUED orphan (#282 shape) carries no stamps, so it sorts oldest and
		// the completed SUCCESS wins — no lingering pending.
		"stale-queued orphan + green latest": {
			rollup: []check{
				{Name: "control-sweep", Status: "QUEUED"},
				{Name: "control-sweep", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: tNew},
			},
			pass: 1,
		},
		// Anti-over-loosen: an older SUCCESS superseded by a newer FAILURE must still count
		// as a failure — the reduction ignores superseded runs, never a real red latest.
		"older success superseded by newer failure": {
			rollup: []check{
				{Name: "build", Status: "COMPLETED", Conclusion: "SUCCESS", CompletedAt: tOld},
				{Name: "build", Status: "COMPLETED", Conclusion: "FAILURE", CompletedAt: tNew},
			},
			fail: 1,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			pass, pending, fail, unknown := ciState(prBase{StatusCheckRollup: c.rollup})
			if pass != c.pass || pending != c.pending || fail != c.fail || unknown != c.unknown {
				t.Errorf("ciState = pass=%d pending=%d fail=%d unknown=%d; want pass=%d pending=%d fail=%d unknown=%d",
					pass, pending, fail, unknown, c.pass, c.pending, c.fail, c.unknown)
			}
		})
	}
}

func TestCIState_EveryStatusStateIsBucketed_400T1(t *testing.T) {
	for state, want := range statusStateBuckets {
		var p prBase
		payload := `{"number":1,"statusCheckRollup":[{"__typename":"StatusContext","state":"` + state +
			`","context":"legacy"}]}`
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			t.Fatalf("fixture %s: %v", state, err)
		}
		pass, pending, fail, unknown := ciState(p)
		got := map[ciBucket]int{bucketPass: pass, bucketPending: pending, bucketFail: fail, bucketUnknown: unknown}
		for _, b := range []ciBucket{bucketPass, bucketPending, bucketFail, bucketUnknown} {
			wantN := 0
			if b == want {
				wantN = 1
			}
			if got[b] != wantN {
				t.Errorf("StatusState %q: bucket %d = %d, want %d (pass=%d pending=%d fail=%d unknown=%d) — "+
					"the declared meaning of the value and what ciState does with it have diverged",
					state, b, got[b], wantN, pass, pending, fail, unknown)
			}
		}
	}

	// The enum, read off board.go's own field comment, must be fully declared above.
	src, err := os.ReadFile("board.go")
	if err != nil {
		t.Fatalf("reading board.go: %v", err)
	}
	const marker = "// StatusContext: "
	i := strings.Index(string(src), marker)
	if i < 0 {
		t.Fatal("board.go no longer documents the StatusContext value set on the `State` field — " +
			"that comment is what this parity check reads")
	}
	line := string(src)[i+len(marker):]
	if j := strings.IndexByte(line, '\n'); j >= 0 {
		line = line[:j]
	}
	for _, v := range strings.Split(line, "/") {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := statusStateBuckets[v]; !ok {
			t.Errorf("board.go's `State` comment lists StatusState %q as a value it expects, and this "+
				"table declares no bucket for it — an enumeration that is a subset of the values the "+
				"field can hold is #400 T1 verbatim", v)
		}
	}
	// Positive control: the parity check must actually have read values.
	if n := len(strings.Split(line, "/")); n < 5 {
		t.Errorf("read only %d value(s) from the `State` comment (%q) — the parity check would assert "+
			"almost nothing", n, line)
	}
}

// mergeStateFixtures are the mergeStateStatus values GitHub actually returns, plus the
// two that carry NO verdict (`UNKNOWN` while mergeability is still computing, and an
// absent field) and one value this code has never seen. The last is the R3 sibling: a
// value added to GitHub's enum later must arrive as could-not-check, never as mergeable.
//
// Each carries its DECLARED verdict (#400 Q1/Q4). Round 2 enumerated these values but
// asserted only `if in.mergeStateUnknown && action == actMergeNow` — conditioned on the
// very boolean a mutant flips — so putting `UNKNOWN` and `""` back into the mergeable set
// left the whole suite green and both classified MERGE-NOW. The two states R3 names by
// name were the two nothing pinned. The declaration below is checked against the real
// `readMergeState` through the real `buildClassifyInput`.
var mergeStateFixtures = map[string]mergeVerdict{
	"CLEAN":     mergeVerdictOK,
	"UNSTABLE":  mergeVerdictOK,
	"HAS_HOOKS": mergeVerdictOK,
	"DRAFT":     mergeVerdictOK,
	// #54: BEHIND is its own verdict, not mergeable. It is a MEASURED "main has moved
	// since this head was last synced" — folding it into mergeVerdictOK is the exact
	// defect #54 documents: an APPROVED review read as still-current at merge time when
	// it never verified against the base a merge would actually use.
	"BEHIND":  mergeVerdictBehind,
	"DIRTY":   mergeVerdictBlocked,
	"BLOCKED": mergeVerdictBlocked,
	// The three that carry NO verdict: still computing, absent from the payload, and a
	// value GitHub may add to the enum later. All three are could-not-check.
	"UNKNOWN":        mergeVerdictUnknown,
	"":               mergeVerdictUnknown,
	"SOME_NEW_STATE": mergeVerdictUnknown,
}

// TestReadMergeState_UnknownStatesAreNotMergeable pins `readMergeState` DIRECTLY, over
// every declared value (#400 Q1). The inventory below drives it through the builder; this
// asserts the function itself, because the two states R3 exists for — `UNKNOWN` and the
// absent field — are exactly the ones a re-widened mergeable set would swallow, and a
// test that only reads `in.mergeStateUnknown` cannot see that.
func TestReadMergeState_UnknownStatesAreNotMergeable(t *testing.T) {
	for raw, want := range mergeStateFixtures {
		if got := readMergeState(raw); got != want {
			t.Errorf("readMergeState(%q) = %d, want %d", raw, got, want)
		}
		// Case and surrounding whitespace are normalised, never a new state.
		for _, variant := range []string{strings.ToLower(raw), "  " + raw + "  "} {
			if got := readMergeState(variant); got != want {
				t.Errorf("readMergeState(%q) = %d, want %d (same value, different spelling)", variant, got, want)
			}
		}
	}
	// The positive control: the declaration must actually exercise all three verdicts,
	// or "every value matched" would be satisfiable by a table of one.
	seen := map[mergeVerdict]int{}
	for _, v := range mergeStateFixtures {
		seen[v]++
	}
	for _, v := range []mergeVerdict{mergeVerdictOK, mergeVerdictBlocked, mergeVerdictUnknown, mergeVerdictBehind} {
		if seen[v] == 0 {
			t.Errorf("no fixture declares verdict %d — the table would assert nothing about it", v)
		}
	}
}

// reviewFixtures are the reviewState reductions a PR can present.
var reviewFixtures = map[string]reviewState{
	"none":              {},
	"head-advanced":     {ever: true, atHead: false, lastSHA: "old"},
	"blocking-at-head":  {ever: true, atHead: true, blocking: true},
	"comment-only-head": {ever: true, atHead: true},
	// #37: a no-op APPROVED suppressed over a standing CHANGES_REQUESTED at the SAME
	// head reduces to blocking=true with suspectNoOp=true — the shape reduceReviews
	// produces for the live-evidence forgery. Distinct from "blocking-at-head" (an
	// ordinary CHANGES_REQUESTED with no suppressed approval behind it): the classifier
	// must read this one as SUSPECT-APPROVAL, never BLOCKED.
	"suspect-noop-approval": {ever: true, atHead: true, blocking: true, suspectNoOp: true},
	"approved":              {ever: true, atHead: true, approved: true},
	"approved-secpass":      {ever: true, atHead: true, approved: true, securityPass: true},
	"approved-no-secpass":   {ever: true, atHead: true, approved: true, securityPass: false},
}

// expectedAction is the per-fixture expected-action table (#400 Q4), written from the
// classifier's DOCUMENTED precedence over the fixtures' DECLARED properties — never from
// values classify itself derived. That independence is the whole point: round 2's
// assertions were all conditioned on `in.ciGreen` / `in.mergeStateUnknown`, so a mutant
// that flipped one of those booleans made its own assertion vacuous and Q1 and Q2 both
// walked out through the hole. Here a mutant that mis-reads a rollup or a merge state
// disagrees with a table that never consulted it.
//
// It is a second implementation on purpose. If a classifier arm is added or reordered,
// this must be edited to match — that edit IS the review of the change.
func expectedAction(rf rollupFixture, mv mergeVerdict, rs reviewState,
	draft, ciRequired, humanGate, ownFilesChanged, riskClassed bool, zeroCI string) string {

	// The tallies and the CI verdict, derived ONLY from the declared fixture. #1652:
	// a genuine zero rollup's green depends on WHAT the probe found, not just on
	// ciRequired — a probe that reads real negative evidence (never-ran / unverified)
	// blocks green on every repo; a zero the caller never probed ("", the same value a
	// non-zero rollup carries) falls back to the pre-#1652 vacuous rule.
	pass, pending, fail, unknown := rf.pass, rf.pending, rf.fail, rf.unknown
	ciGreen := unknown == 0 && pass > 0 && pending == 0 && fail == 0
	if unknown == 0 && pass == 0 && pending == 0 && fail == 0 {
		switch zeroCI {
		case zeroCINeverRan, zeroCIUnverified:
			ciGreen = false
		default: // zeroCINoChecks, or "" (never probed)
			ciGreen = !ciRequired
		}
	}
	blocked := mv == mergeVerdictBlocked
	unknownMerge := mv == mergeVerdictUnknown
	behindMerge := mv == mergeVerdictBehind

	switch {
	case !rs.ever:
		return actNeedsReview
	case !rs.atHead:
		if !ownFilesChanged {
			return actMergeCurr
		}
		return actReReview
	// #37: a suppressed no-op APPROVED over a standing CHANGES_REQUESTED at the same
	// head must read SUSPECT-APPROVAL, never an ordinary BLOCKED — checked first since
	// classify()'s own switch checks it before the plain blocking arm.
	case rs.blocking && rs.suspectNoOp:
		return actSuspectApproval
	case rs.blocking:
		return actBlocked
	case fail > 0 && unknown > 0:
		return actCIRed
	case unknown > 0:
		return actCIUnknown
	case rs.approved && !ciGreen && fail == 0 && pending == 0:
		// #216 / #1652: hoisted above the zero-CI-driven return so every probed-zero
		// (and never-probed-zero) branch sees it — the security guard must not be
		// shadowed by any of the more specific CI-absence states.
		if riskClassed && !rs.securityPass {
			return actSecReview
		}
		switch zeroCI {
		case zeroCINeverRan:
			return actCINeverRan
		case zeroCIUnverified, zeroCINoChecks:
			return actCheck
		}
		return actCIUnverified
	case rs.approved && ciGreen && !blocked:
		if draft && riskClassed && !rs.securityPass {
			return actSecReview
		}
		if humanGate {
			return actHumanGate
		}
		if unknownMerge {
			if draft {
				return actFlip
			}
			return actMergeStateUnknown
		}
		// #54: BEHIND withholds MERGE-NOW the same way an unknown merge state does, for
		// the opposite reason — this one WAS measured, and what it measured is that main
		// has moved past the base the review verified.
		if behindMerge {
			if draft {
				return actFlip
			}
			return actMergeBehind
		}
		return actMergeNow
	case !draft:
		return actReady
	case fail > 0:
		return actCIRed
	case pending > 0:
		return actWaitCI
	case blocked:
		return actConflict
	case rs.approved:
		// The fail-loud guard arm. Reaching it is a classifier hole, asserted separately.
		return actCheck
	default:
		return actCheck
	}
}

// TestReadme_SweepingVerbSentence_359 closes N10. README's "Every sweeping verb states
// its scope" sentence enumerated FOUR of the five verbs that carried the header — the
// fifth being `scope`, added by this PR's own R4 fix and not added there. A subset
// presented as complete, in the documentation of the section about subsets presented as
// complete. `TestReadme_VerbParity` covers the tool-reference ROW, not this sentence.
func TestReadme_SweepingVerbSentence_359(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("reading tools/desk/README.md: %v", err)
	}
	const claim = "Every sweeping verb states its scope."
	i := strings.Index(string(b), claim)
	if i < 0 {
		t.Fatalf("README.md no longer carries the %q sentence — the coverage claim it documents "+
			"is what #359 exists to make legible", claim)
	}
	// The sentence runs to the end of its bullet (the next line starting "- **").
	rest := string(b)[i:]
	if j := strings.Index(rest, "\n- **"); j >= 0 {
		rest = rest[:j]
	}
	var sweeping, nonSweep []string
	for v, c := range verbScopeClass {
		switch c {
		case sweepsRepos:
			sweeping = append(sweeping, v)
		case nonSweeping:
			nonSweep = append(nonSweep, v)
		}
	}
	sort.Strings(sweeping)
	sort.Strings(nonSweep)
	if len(sweeping) == 0 || len(nonSweep) == 0 {
		t.Fatal("verbScopeClass declares no sweeping or no non-sweeping verb — this check would assert nothing")
	}
	for _, v := range sweeping {
		if !strings.Contains(rest, "`"+v+"`") {
			t.Errorf("the %q sentence omits `%s`, which verbScopeClass declares repo-sweeping — a subset "+
				"presented as the complete list. It reads:\n%s", claim, v, strings.TrimSpace(rest))
		}
	}
	for _, v := range nonSweep {
		if !strings.Contains(rest, "`"+v+"`") {
			t.Errorf("the %q sentence omits `%s` from the verbs that OMIT the field; it reads:\n%s",
				claim, v, strings.TrimSpace(rest))
		}
	}
}

// TestReduceReviews_AbsentShasAreNeverAtHead closes #400 Q3, the same absence-as-verdict
// defect as R2 and R3 on the review axis. `st.atHead = last.CommitID == head` made two
// ABSENCES compare EQUAL: a payload with no `headRefOid` and a review with no `commit_id`
// produced ever=true, atHead=true, approved=true — and with a green rollup, MERGE-NOW,
// the most consequential verdict this board emits, from two fields nobody could read.
//
// Driven through the real `reduceReviews` and then the real `classify`, because the
// classifier fixtures hand-build `reviewState` and can never see this.
func TestReduceReviews_AbsentShasAreNeverAtHead(t *testing.T) {
	mk := func(state, commitID string) review {
		var r review
		r.User.Login = reviewerBotDisplay()
		r.State = state
		r.CommitID = commitID
		r.SubmittedAt = time.Now().UTC().Format(time.RFC3339)
		return r
	}

	cases := []struct {
		name       string
		head       string
		commitID   string
		wantAtHead bool
	}{
		{"both present and equal", "abc123", "abc123", true},
		{"both present, different", "abc123", "old999", false},
		{"head absent, review carries a sha", "", "abc123", false},
		{"review commit_id absent, head present", "abc123", "", false},
		// The defect verbatim: two absences are not a match.
		{"BOTH absent", "", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := reduceReviews([]review{mk("APPROVED", c.commitID)}, c.head)
			if st.atHead != c.wantAtHead {
				t.Fatalf("atHead = %v, want %v (head=%q reviewSHA=%q) — an unread sha is a "+
					"could-not-check, and could-not-check is never 'at head'",
					st.atHead, c.wantAtHead, c.head, c.commitID)
			}
			if !c.wantAtHead && st.approved {
				t.Errorf("approved = true off a verdict that is not at head (head=%q reviewSHA=%q)",
					c.head, c.commitID)
			}

			// And the consequence at the board level: a green, clean, approved PR whose
			// shas could not be compared must NOT be recommended for merge.
			in := classifyInput{
				ever: st.ever, atHead: st.atHead, blocking: st.blocking, approvedAtHead: st.approved,
				pass: 1, ciGreen: true, ownFilesChanged: true,
			}
			action, _ := classify(in)
			if !c.wantAtHead && action == actMergeNow {
				t.Errorf("head=%q reviewSHA=%q classified MERGE-NOW — this is #400 Q3 verbatim",
					c.head, c.commitID)
			}
			if c.wantAtHead && action != actMergeNow {
				t.Errorf("positive control: a genuinely at-head approval on green CI must reach "+
					"MERGE-NOW; got %s", action)
			}
		})
	}

	// The security-marker read is the same comparison and gets the same rule: a marker
	// on a review whose sha could not be compared is not a marker "at head".
	sec := mk("APPROVED", "")
	sec.Body = "Security-Review: pass"
	if st := reduceReviews([]review{sec}, ""); st.securityPass {
		t.Error("securityPass = true from a review whose sha and the PR head were BOTH absent — " +
			"the security verdict is claimed at a head nobody read")
	}
	// Positive control for that half.
	sec2 := mk("APPROVED", "abc123")
	sec2.Body = "Security-Review: pass"
	if st := reduceReviews([]review{sec2}, "abc123"); !st.securityPass {
		t.Error("positive control: a Security-Review: pass at a READ head must count")
	}
}

// TestClassify_GuardArmIsStillTheGuardArm closes N6 structurally. The inventory proves
// no reachable input REACHES the fail-loud guard arm — but it proved that by looking for
// the arm's own note, so replacing the arm's `return` with the old `FLIP` + "CI green"
// note satisfied the tripwire by deleting it: nothing reached a phrase nothing said any
// more. Unreachability is only worth asserting about an arm that still fails loud, so the
// arm's identity is pinned at the source level, where a swapped return is visible.
func TestClassify_GuardArmIsStillTheGuardArm(t *testing.T) {
	src, err := os.ReadFile("board.go")
	if err != nil {
		t.Fatalf("reading board.go: %v", err)
	}
	body := string(src)
	const arm = "\tcase in.approvedAtHead:\n"
	i := strings.Index(body, arm)
	if i < 0 {
		t.Fatal("the `case in.approvedAtHead:` guard arm is GONE from classify — #400 R2's fail-loud " +
			"backstop is what makes the unreachability claim meaningful; removing it makes a later " +
			"routing change silent again")
	}
	rest := body[i+len(arm):]
	end := strings.Index(rest, "\n\tcase ")
	if j := strings.Index(rest, "\n\tdefault:"); j >= 0 && (end < 0 || j < end) {
		end = j
	}
	if end < 0 {
		t.Fatal("could not find the end of the guard arm in classify")
	}
	armSrc := rest[:end]
	if !strings.Contains(armSrc, "guardArmMarker") {
		t.Errorf("the guard arm no longer returns guardArmMarker — the tripwire that asserts it is "+
			"unreachable keys on that constant, so an arm that says anything else is a tripwire that "+
			"deleted itself (N6). Arm source:\n%s", armSrc)
	}
	if !strings.Contains(armSrc, "actCheck") {
		t.Errorf("the guard arm must return actCheck — a reachable-but-unexplained row is an "+
			"instruction to inspect, never a verdict. Arm source:\n%s", armSrc)
	}
	for _, forbidden := range []string{"actFlip", "actMergeNow", "CI green"} {
		if strings.Contains(armSrc, forbidden) {
			t.Errorf("the guard arm mentions %q — it exists to say the board has NO verdict for the row; "+
				"a friendly verdict here is the #400 R2 defect restored. Arm source:\n%s", forbidden, armSrc)
		}
	}
	// And the phrase is the guard arm's alone: written out a second time anywhere in the
	// package, the inventory's "nothing reached the guard arm" check could fire on an
	// innocent row, or (worse) be satisfied by a note the guard arm no longer emits.
	srcs, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing package sources: %v", err)
	}
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		want := 0
		if f == "board.go" {
			want = 1 // the const declaration; the arm references the identifier
		}
		if n := strings.Count(string(b), guardArmMarker); n != want {
			t.Errorf("the guard-arm phrase is written literally %d time(s) in %s, want %d — it must "+
				"exist once, as the constant, and be referenced by identifier everywhere else", n, f, want)
		}
	}
}

// TestClassify_ActionInventory drives every combination of the payload shapes above
// through the REAL input builder and asserts the classifier's whole vocabulary is
// accounted for, in both directions.
func TestClassify_ActionInventory(t *testing.T) {
	consts := actionConstsFromSource(t)
	produced := map[string]string{} // action -> a describing case, for the failure message

	for rollupName, rf := range rollupFixtures {
		rollup := rf.json
		for ms, wantVerdict := range mergeStateFixtures {
			for revName, rs := range reviewFixtures {
				for _, draft := range []bool{true, false} {
					for _, ciRequired := range []bool{true, false} {
						for _, humanGate := range []bool{true, false} {
							title := "t"
							if humanGate {
								title = "[HUMAN GATE] t"
							}
							payload := `{"number":1,"title":` + mustJSON(title) + `,"body":"","isDraft":` +
								strconv.FormatBool(draft) + `,"author":{"login":"shared-agent"},"createdAt":"` +
								time.Now().UTC().Format(time.RFC3339) + `","labels":[],"headRefOid":"abc123",` +
								`"headRefName":"b","mergeStateStatus":` + mustJSON(ms) + `,` + rollup +
								`"__end":0}`
							var p prBase
							if err := json.Unmarshal([]byte(strings.Replace(payload, `,"__end":0`, ``, 1)), &p); err != nil {
								t.Fatalf("fixture %s/%s: %v\n%s", rollupName, ms, err, payload)
							}

							// The two inputs the caller fills after the extra fetches, under the
							// same conditions cmdActions applies them.
							for _, ownFilesChanged := range []bool{true, false} {
								for _, riskClassed := range []bool{true, false} {
									// #1652: zeroCI is a THIRD caller-filled input, like ownFilesChanged
									// and riskClassed — classifyPR only probes it for a genuine zero
									// rollup (pass==pending==fail==unknown==0), so it is only varied
									// there; every other shape gets the one real value a caller ever
									// passes for it: "" (never probed, because there was something to
									// read). Without this dimension no enumerated payload can reach
									// actCINeverRan, and a guard nothing can reach is a guard nothing
									// tests.
									zeroCIOptions := []string{""}
									if rf.pass == 0 && rf.pending == 0 && rf.fail == 0 && rf.unknown == 0 {
										zeroCIOptions = []string{"", zeroCINoChecks, zeroCINeverRan, zeroCIUnverified}
									}
									for _, zeroCI := range zeroCIOptions {
										in := buildClassifyInput(p, rs, ciRequired, zeroCI)
										in.ownFilesChanged = ownFilesChanged
										in.riskClassed = riskClassed
										action, note := classify(in)

										desc := "rollup=" + rollupName + " mergeState=" + strconv.Quote(ms) +
											" reviews=" + revName + " draft=" + strconv.FormatBool(draft) +
											" ciRequired=" + strconv.FormatBool(ciRequired) +
											" humanGate=" + strconv.FormatBool(humanGate) +
											" zeroCI=" + strconv.Quote(zeroCI)
										if _, ok := produced[action]; !ok {
											produced[action] = desc
										}

										// #400 Q2: the REAL tallies must match what the fixture is
										// DECLARED to mean. This is the assertion that was missing —
										// it consults no derived boolean, so counting SKIPPED/NEUTRAL
										// as a pass fails HERE rather than sliding through as green.
										if in.pass != rf.pass || in.pending != rf.pending ||
											in.fail != rf.fail || in.ciUnknown != rf.unknown {
											t.Fatalf("%s: ciState read pass=%d pending=%d fail=%d unknown=%d, but the "+
												"fixture declares pass=%d pending=%d fail=%d unknown=%d — a check that "+
												"reported no verdict is not a passing check",
												desc, in.pass, in.pending, in.fail, in.ciUnknown,
												rf.pass, rf.pending, rf.fail, rf.unknown)
										}
										// #400 Q1 / #54: same, on the merge axis — checked against the
										// declared verdict, not against `in.mergeStateUnknown` /
										// `in.mergeBehind`.
										if in.mergeConflict != (wantVerdict == mergeVerdictBlocked) ||
											in.mergeStateUnknown != (wantVerdict == mergeVerdictUnknown) ||
											in.mergeBehind != (wantVerdict == mergeVerdictBehind) {
											t.Fatalf("%s: buildClassifyInput read mergeConflict=%v unknown=%v behind=%v, but %q is "+
												"declared verdict %d", desc, in.mergeConflict, in.mergeStateUnknown, in.mergeBehind, ms, wantVerdict)
										}

										// #400 Q4: the per-fixture expected ACTION. Direction 2 below only
										// requires each action to be produced by SOME row; this binds THIS
										// row's outcome, which is what Q1 and Q2 escaped through.
										if want := expectedAction(rf, wantVerdict, rs, draft, ciRequired,
											humanGate, ownFilesChanged, riskClassed, zeroCI); action != want {
											t.Fatalf("%s ownFilesChanged=%v riskClassed=%v: classify = %s, want %s\nnote: %s",
												desc, ownFilesChanged, riskClassed, action, want, note)
										}

										// The fail-loud guard arm (#400 R2) must be unreachable. If a
										// payload lands on it, the classifier has a hole and the board
										// would print a row it cannot explain.
										if strings.Contains(note, guardArmMarker) {
											t.Errorf("classifier hole: %s reached the fail-loud guard arm\nnote: %s", desc, note)
										}

										// The absence half, stated positively: no note may CLAIM green
										// unless green was actually established. Read off the DECLARED
										// tallies, not off `in.ciGreen` — a mutant that flips that
										// boolean must not be able to make its own guard vacuous.
										// #400 N9 strengthens this: the note may say "CI green" only when a
										// check actually PASSED. `ciGreen` is also vacuously true on a repo
										// the policy marks as running no PR CI — a policy statement, not a
										// result — and asserting a verdict there is the same defect with a
										// friendlier cause.
										if rf.pass == 0 && strings.Contains(note, "CI green,") {
											t.Errorf("%s: note asserts CI green over a rollup in which NOT ONE check "+
												"passed — this is #400 R2/N9\nnote: %s", desc, note)
										}
										// ...and none may recommend the merge on an unread merge state.
										if wantVerdict == mergeVerdictUnknown && action == actMergeNow {
											t.Errorf("%s: MERGE-NOW on an unread mergeStateStatus — this is #400 R3 verbatim", desc)
										}
										// ...nor on a base that has moved past the reviewed diff (#54):
										// the review never verified against the tree a merge would use.
										if wantVerdict == mergeVerdictBehind && action == actMergeNow {
											t.Errorf("%s: MERGE-NOW on a BEHIND mergeStateStatus — this is #54 verbatim "+
												"(a review-time verdict trusted as still current at merge-time)", desc)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Direction 1: nothing came out that the vocabulary does not define.
	known := map[string]bool{}
	for _, v := range consts {
		known[v] = true
	}
	for action, desc := range produced {
		if !known[action] {
			t.Errorf("classify produced ACTION %q, which is not one of board.go's act* constants (from %s)", action, desc)
		}
	}

	// Direction 2 — the positive control. Every declared ACTION must be REACHED by this
	// enumeration. An action nothing here produces is either dead vocabulary or an arm
	// whose input shape no fixture can build, and in this cluster that second case is
	// exactly how a guard ends up untested.
	var missing []string
	for name, v := range consts {
		if _, ok := produced[v]; ok {
			continue
		}
		if why, declared := actionsDeclaredUnproducible[v]; declared {
			t.Logf("%s (%s) declared unproducible: %s", name, v, why)
			continue
		}
		missing = append(missing, name+" ("+v+")")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("these ACTIONs are defined but NO enumerated payload produces them: %s\n"+
			"Either the enumeration cannot represent the state that arm defends (extend rollupFixtures / "+
			"mergeStateFixtures / reviewFixtures — an untestable guard is an untested guard), or the arm is "+
			"dead and should be removed, or it is genuinely unproducible and belongs in "+
			"actionsDeclaredUnproducible WITH a reason.", strings.Join(missing, ", "))
	}
}

// ---------------------------------------------------------------------------
// 2. the VERB inventory
// ---------------------------------------------------------------------------

// scopeClass is what a verb owes on the coverage axis.
type scopeClass int

const (
	sweepsRepos scopeClass = iota // iterates deskkit.AllowedRepos — MUST state `scope`
	sweepsRoots                   // iterates the configured ROOTS — states coverage as `roots`
	nonSweeping                   // takes an explicit repo — MUST omit `scope` entirely
	refusedVerb                   // exits 5 by design; produces no report
)

// verbScopeClass is the hand-declared half. It is deliberately the ONLY hand-kept list,
// and the test fails if it disagrees with the dispatcher in either direction — a verb
// added to main.go and classified nowhere is the exact "classified nowhere" hole this
// guard exists to catch.
var verbScopeClass = map[string]scopeClass{
	"prs":         sweepsRepos,
	"actions":     sweepsRepos,
	"queue":       sweepsRepos,
	"health":      sweepsRepos,
	"scope":       sweepsRepos,
	"policydrift": sweepsRepos,
	"awaiting":    sweepsRoots,
	"nextup":      sweepsRoots,
	"stalled":     sweepsRepos,
	// throughput spans BOTH coverage axes: it sweeps the repo set for its review depth and
	// the configured ROOTS for its dispatch/verify depths. It is declared repo-sweeping
	// because that is the obligation with teeth — `scope` must be present and must agree
	// with the set the loop iterates — and it ALSO emits `roots`, which this class does not
	// forbid. Declaring it root-sweeping instead would be worse: that class REQUIRES `scope`
	// to be absent, and a verb that really does sweep the repo set would then be forbidden
	// from stating the coverage it has.
	"throughput": sweepsRepos,
	"reviews":    nonSweeping,
	"diff":       nonSweeping,
	"files":      nonSweeping,
	// #321: the dispatch queue is now SERVED — it iterates the configured ROOTS
	// (statusgen --next-up per root), exactly like awaiting/nextup, and states its
	// coverage as `roots`. It is no longer a refusedVerb.
	"dispatch": sweepsRoots,
	"todo":     sweepsRoots,
	"next":     sweepsRoots,
	"next-up":  sweepsRoots,
}

// dispatchVerbs reads every case label of main.go's dispatch switch. Parsed, never
// hand-listed: the point is to detect the verb that was added without being classified.
func dispatchVerbs(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing main.go: %v", err)
	}
	var verbs []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "dispatch" {
			return true
		}
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			sw, ok := m.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			id, ok := sw.Tag.(*ast.Ident)
			if !ok || id.Name != "sub" {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, e := range cc.List {
					lit, ok := e.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					v, err := strconv.Unquote(lit.Value)
					if err == nil {
						verbs = append(verbs, v)
					}
				}
			}
			return false
		})
		return false
	})
	if len(verbs) == 0 {
		t.Fatal("no verbs parsed out of dispatch() — the inventory would assert nothing")
	}
	sort.Strings(verbs)
	return verbs
}

// TestDispatch_VerbInventory closes finding 2 at the population level rather than one
// verb at a time. Round 1 pinned four of the five sweeping verbs; `scope`'s own header
// `scope` could still be deleted with the whole suite green. Rather than add a fifth
// assertion, this asserts over the dispatcher itself.
func TestDispatch_VerbInventory(t *testing.T) {
	verbs := dispatchVerbs(t)

	// Both directions of the classification, before anything is run.
	for _, v := range verbs {
		if _, ok := verbScopeClass[v]; !ok {
			t.Fatalf("dispatch verb %q is classified NOWHERE — add it to verbScopeClass "+
				"(sweepsRepos / sweepsRoots / nonSweeping / refusedVerb). A verb with no declared "+
				"coverage obligation is a verb whose coverage claim nobody checks.", v)
		}
	}
	inDispatch := map[string]bool{}
	for _, v := range verbs {
		inDispatch[v] = true
	}
	for v := range verbScopeClass {
		if !inDispatch[v] {
			t.Errorf("verbScopeClass declares %q, which dispatch() no longer routes — the table has gone stale", v)
		}
	}

	repo := "example-org/tracker"
	// Positive control: the enumeration must actually exercise each class, or "every
	// verb passed" would be satisfiable by a table that ran nothing.
	seen := map[scopeClass]int{}

	for _, v := range verbs {
		v := v
		t.Run(v, func(t *testing.T) {
			class := verbScopeClass[v]
			seen[class]++

			argv := []string{v}
			switch class {
			case nonSweeping:
				argv = append(argv, repo, "1")
			case sweepsRoots:
				installFakeStatusgen(t)
			}
			// `stalled` inverts the default: its table is the human path and `--json`
			// selects the machine shape every other verb already defaults to. Without it
			// this harness would try to parse a human table as JSON.
			if v == "stalled" {
				argv = append(argv, "--json")
			}

			installFakeGH(t)
			t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
			t.Setenv("DESKBOARD_GH_PRLIST_JSON",
				`[{"number":1,"title":"t","body":"","isDraft":true,"author":{"login":"shared-agent"},"createdAt":"`+
					time.Now().UTC().Format(time.RFC3339)+`","labels":[],"headRefOid":"abc123",`+
					`"headRefName":"b","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
			if class == sweepsRoots {
				twoRoots(t)
			}

			var out, errb bytes.Buffer
			code := run(argv, &out, &errb)

			if class == refusedVerb {
				if code != deskkit.ExitRefused {
					t.Fatalf("run(%v) = exit %d, want %d — a verb that cannot be served must REFUSE, "+
						"never emit an empty board (#321)", argv, code, deskkit.ExitRefused)
				}
				if strings.Contains(out.String(), `"rows"`) {
					t.Errorf("a refused verb must emit no report; got:\n%s", out.String())
				}
				return
			}
			if code != deskkit.ExitOK {
				t.Fatalf("run(%v) = exit %d: %s", argv, code, errb.String())
			}

			var m map[string]any
			if err := json.Unmarshal(out.Bytes(), &m); err != nil {
				t.Fatalf("%v: %v\n%s", argv, err, out.String())
			}
			scope, hasScope := m["scope"].(map[string]any)

			switch class {
			case sweepsRepos:
				if !hasScope {
					t.Fatalf("%q sweeps deskkit.AllowedRepos and MUST state its scope in the header; got %v",
						v, m["scope"])
				}
				if int(scope["count"].(float64)) != len(deskkit.AllowedRepos()) {
					t.Errorf("%q: scope count %v disagrees with the set the loop iterates", v, scope["count"])
				}
			case sweepsRoots:
				if hasScope {
					t.Errorf("%q sweeps ROOTS, not the repo set — a repo-axis `scope` here would claim a "+
						"coverage it does not have; got %v", v, m["scope"])
				}
				if _, ok := m["roots"]; !ok {
					t.Errorf("%q must state which roots it read — otherwise an empty row set is "+
						"indistinguishable from a root that was never read; got keys %v", v, keysOf(m))
				}
			case nonSweeping:
				if strings.Contains(out.String(), `"scope"`) {
					t.Errorf("%q takes an explicit repo and swept nothing — `scope` must be ABSENT so "+
						"absent can never be read as 'the set was empty'; got:\n%s", v, out.String())
				}
			}
		})
	}

	// refusedVerb is intentionally omitted: since #321 no classified verb refuses
	// (the dispatch queue is now served), so there is no refused member to exercise.
	// The class + its handling above are kept for a future verb that cannot be served.
	for _, c := range []scopeClass{sweepsRepos, sweepsRoots, nonSweeping} {
		if seen[c] == 0 {
			t.Errorf("no verb of class %d was exercised — the inventory passed vacuously for it", c)
		}
	}
}

// isVerbToken keeps the backticked tokens that could be a subcommand and drops prose.
func isVerbToken(s string) bool {
	if s == "" || s == "deskboard" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '-' {
			return false
		}
	}
	return true
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// 3. the README parity check
// ---------------------------------------------------------------------------

// TestReadme_VerbParity — the enumerated list is the part that rots. README's tool
// reference names deskboard's verbs; nothing made it agree with the dispatcher, so a
// verb added or renamed leaves the documented list quietly wrong, and the operator
// reading it believes the board covers something it does not.
func TestReadme_VerbParity(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("reading tools/desk/README.md: %v", err)
	}
	var row string
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "| `deskboard` |") {
			row = ln
			break
		}
	}
	if row == "" {
		t.Fatal("no `deskboard` row found in README.md's tool-reference table — the parity check " +
			"would pass vacuously (this failure IS the drift it looks for)")
	}

	// Verbs the README states, read out of the Verb(s) CELL's backticked names. Only
	// verb-shaped tokens count: the cell also carries prose ("(alias `nextup`)").
	cells := strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")
	if len(cells) < 2 {
		t.Fatalf("deskboard README row has no Verb(s) cell: %s", row)
	}
	documented := map[string]bool{}
	for i, part := range strings.Split(cells[1], "`") {
		if i%2 == 0 { // outside the backticks
			continue
		}
		if isVerbToken(part) {
			documented[part] = true
		}
	}
	if len(documented) == 0 {
		t.Fatalf("no verbs parsed out of the README row — the parity check would assert nothing: %s", row)
	}

	// The refused verbs are deliberately NOT in the tool reference — they exist to say
	// no. They are documented in their own README section instead, which this checks.
	readme := string(b)
	for _, v := range dispatchVerbs(t) {
		if verbScopeClass[v] == refusedVerb {
			if !strings.Contains(readme, "`deskboard "+v+"`") && !strings.Contains(readme, "`"+v+"`") {
				t.Errorf("refused verb %q is routed by dispatch() but the README never mentions it — "+
					"an operator learns it exists by being refused", v)
			}
			continue
		}
		if !documented[v] {
			t.Errorf("verb %q is served by dispatch() but is missing from README's deskboard tool-reference "+
				"row — the enumerated list has gone stale:\n%s", v, strings.TrimSpace(row))
		}
	}
	for v := range documented {
		if _, ok := verbScopeClass[v]; !ok {
			t.Errorf("README's deskboard row documents verb %q, which dispatch() does not route", v)
		}
	}
}
