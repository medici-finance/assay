package main

// cluster_test.go — the deskboard defect cluster: #236 (stale fails open), #247
// (merged vs closed), #241 (no human-gate concept), #268 (CI reducer + security-marker
// divergence with deskpost), #321 (nextup returns the wrong population), #359 (the
// board cannot say it is a subset — by repo and by time).
//
// Every fix is asserted TWICE: firing on the condition it exists for, and SILENT on an
// ordinary board. A signal never observed staying quiet is a signal that will be
// trained away, which is the same outcome as not having it.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// actionsJSON runs `actions` through the fake gh and decodes the report.
func actionsJSON(t *testing.T, args ...string) actionsReport {
	t.Helper()
	var out, errb bytes.Buffer
	argv := append([]string{"actions"}, args...)
	if code := run(argv, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(%v) = exit %d, stderr=%s", argv, code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	return rep
}

// actionsTable runs `actions --table` and returns stdout.
func actionsTable(t *testing.T, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	argv := append([]string{"actions", "--table"}, args...)
	if code := run(argv, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(%v) = exit %d, stderr=%s", argv, code, errb.String())
	}
	return out.String()
}

// onePR installs a single-PR fixture on the tracker repo.
//
// It also installs a BENIGN changed-file set (one doc file, reconciling against
// changed_files:1). That is not decoration: since #382 the risk-path scan fails CLOSED on
// an unreadable diff — `RiskPathTriggered` returns true when `len(changedFiles) == 0` —
// so a fixture that says nothing about the diff classifies every PR
// SECURITY-REVIEW-REQUIRED and no other action can ever be observed. A test that wants
// the risk lane overrides DESKBOARD_GH_PRFILES_JSON with a security path.
func onePR(t *testing.T, prJSON string) {
	t.Helper()
	t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", "["+prJSON+"]")
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`)
	t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
}

// approvedReview is an App APPROVED review at head carrying body.
func approvedReview(head, body string) string {
	b, _ := json.Marshal(body)
	return `[{"user":{"login":"` + reviewerBotDisplay() + `"},"state":"APPROVED","commit_id":"` + head +
		`","submitted_at":"` + time.Now().UTC().Format(time.RFC3339) + `","body":` + string(b) + `}]`
}

// findRow returns the row for a PR number.
func findRow(t *testing.T, rep actionsReport, num int) actionRow {
	t.Helper()
	for _, r := range rep.Rows {
		if r.Number == num {
			return r
		}
	}
	t.Fatalf("no row for PR #%d in %+v", num, rep.Rows)
	return actionRow{}
}

// ---------------------------------------------------------------------------
// #236 — the drift detector must never answer "fresh" when it did not look
// ---------------------------------------------------------------------------

func TestStale_ThreeStates_236(t *testing.T) {
	oldS, oldB := deskkit.SourceSHA, deskkit.BuiltAt
	t.Cleanup(func() { deskkit.SourceSHA, deskkit.BuiltAt = oldS, oldB })
	// Force the #185 primary source (the `.assay-versions` desk-tools pin) OFF so
	// these subtests deterministically reach the in-tree fallback and its
	// could-not-check, regardless of any pin file above the test's working dir.
	oldPin := deskToolsPin
	t.Cleanup(func() { deskToolsPin = oldPin })
	deskToolsPin = func() (string, string, bool) { return "", "", false }

	t.Run("could-not-check fails CLOSED", func(t *testing.T) {
		// A pinned binary whose sourceSHA does not resolve in git: the check cannot
		// run. This is the exact shape the issue observed shipping as "stale": false.
		deskkit.SourceSHA, deskkit.BuiltAt = "0000000000000000000000000000000000000000", "2026-07-31T00:00:00Z"
		state, stale, detail := staleState()
		if state != staleStateUnknown {
			t.Errorf("state = %q, want %q", state, staleStateUnknown)
		}
		if !stale {
			t.Errorf("stale = false on a check that could not run — that is the #236 fail-open (detail: %s)", detail)
		}
		if !strings.Contains(detail, "COULD-NOT-CHECK") {
			t.Errorf("detail must say the check could not run; got %q", detail)
		}
		var banner bytes.Buffer
		printBanners(&banner, Header{StaleState: state, Stale: stale, StaleDetail: detail})
		if !strings.Contains(banner.String(), "STALE-UNKNOWN:") {
			t.Errorf("banner must distinguish could-not-check from measured drift; got %q", banner.String())
		}
	})

	t.Run("unpinned is not-applicable, not a failed check", func(t *testing.T) {
		deskkit.SourceSHA, deskkit.BuiltAt = "", ""
		state, stale, detail := staleState()
		if state != staleStateNotApplicable {
			t.Errorf("state = %q, want %q", state, staleStateNotApplicable)
		}
		if stale {
			t.Error("an unpinned build has no installed binary to drift; reporting STALE here would be noise, not safety")
		}
		if strings.Contains(detail, "COULD-NOT-CHECK") {
			t.Errorf("not-applicable must not masquerade as a failed measurement; got %q", detail)
		}
		var banner bytes.Buffer
		printBanners(&banner, Header{StaleState: state, Stale: stale, StaleDetail: detail})
		if strings.Contains(banner.String(), "STALE") {
			t.Errorf("the ordinary unpinned case must stay quiet; got %q", banner.String())
		}
	})

	t.Run("header carries the state end to end", func(t *testing.T) {
		installFakeGH(t)
		deskkit.SourceSHA, deskkit.BuiltAt = "0000000000000000000000000000000000000000", "2026-07-31T00:00:00Z"
		rep := actionsJSON(t)
		if rep.Header.StaleState != staleStateUnknown || !rep.Header.Stale {
			t.Errorf("header = {state:%q stale:%t}, want {unknown true}", rep.Header.StaleState, rep.Header.Stale)
		}
	})
}

// ---------------------------------------------------------------------------
// #247 — absence proves a PR is gone, never HOW it went
// ---------------------------------------------------------------------------

func TestTombstone_MergedVsClosed_247(t *testing.T) {
	seed := func(t *testing.T, stateJSON string) tombstone {
		t.Helper()
		installFakeGH(t) // no PR list → the seeded PR is gone from the open set
		if stateJSON != "" {
			t.Setenv("DESKBOARD_GH_PRSTATE_JSON", stateJSON)
		}
		if err := deskkit.Log(deskkit.Entry{
			Tool: "deskboard", Verb: "actions", Result: deskkit.ResultOK,
			Detail: "open=example-org/tracker#1580",
			TS:     time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("seeding audit: %v", err)
		}
		rep := actionsJSON(t)
		if len(rep.Tombstones) != 1 {
			t.Fatalf("want exactly one tombstone, got %+v", rep.Tombstones)
		}
		return rep.Tombstones[0]
	}

	t.Run("merged says MERGED", func(t *testing.T) {
		ts := seed(t, `{"state":"closed","merged":true,"merged_at":"2026-07-31T14:01:35Z"}`)
		if ts.State != prStateMerged || ts.Merged == nil || !*ts.Merged {
			t.Fatalf("state=%q merged=%v, want merged/true", ts.State, ts.Merged)
		}
		if !strings.Contains(ts.Note, "MERGED") {
			t.Errorf("note should say MERGED; got %q", ts.Note)
		}
	})

	// #400 N1: every fixture that said merged ALSO set merged_at, so the second merged
	// arm — `merged:true` with merged_at absent or null — was a branch of the function
	// the brief asked to enumerate that nothing reached. Mutating it to `closed` left
	// the suite green: a genuinely merged PR could have been tombstoned as "content did
	// NOT land", which is the #247 error with its sign flipped.
	t.Run("merged=true with no merged_at is still MERGED", func(t *testing.T) {
		for _, payload := range []string{
			`{"state":"closed","merged":true}`,
			`{"state":"closed","merged":true,"merged_at":null}`,
			`{"state":"closed","merged":true,"merged_at":""}`,
		} {
			ts := seed(t, payload)
			if ts.State != prStateMerged || ts.Merged == nil || !*ts.Merged {
				t.Fatalf("%s: state=%q merged=%v, want merged/true — `merged:true` is the API saying it "+
					"landed; a missing timestamp is not evidence that it did not", payload, ts.State, ts.Merged)
			}
			if !strings.Contains(ts.Note, "MERGED") {
				t.Errorf("%s: note should say MERGED; got %q", payload, ts.Note)
			}
		}
	})

	t.Run("closed unmerged never says MERGED", func(t *testing.T) {
		// The live instance: tracker#1580, mergedAt null, printed as "MERGED — drop from
		// your list" and relayed to a human as fact.
		ts := seed(t, `{"state":"closed","merged":false,"merged_at":null}`)
		if ts.State != prStateClosed {
			t.Fatalf("state=%q, want %q", ts.State, prStateClosed)
		}
		if ts.Merged == nil || *ts.Merged {
			t.Fatalf("merged=%v, want an explicit false", ts.Merged)
		}
		if strings.Contains(ts.Note, "MERGED") {
			t.Errorf("a closed-unmerged PR must never be labelled MERGED; got %q", ts.Note)
		}
		if !strings.Contains(ts.Note, "did NOT land") {
			t.Errorf("note must say the content did not land; got %q", ts.Note)
		}
	})

	t.Run("unreadable state is unknown, not merged", func(t *testing.T) {
		ts := seed(t, "") // shim default: an unusable payload
		if ts.State != prStateUnknown {
			t.Fatalf("state=%q, want %q", ts.State, prStateUnknown)
		}
		if ts.Merged != nil {
			t.Errorf("the merged field must be OMITTED when it could not be established; got %v", *ts.Merged)
		}
		if strings.Contains(ts.Note, "MERGED —") {
			t.Errorf("a could-not-check must not read as merged; got %q", ts.Note)
		}
		if !strings.Contains(ts.Note, "COULD-NOT-CHECK") {
			t.Errorf("note must name the could-not-check; got %q", ts.Note)
		}
		// The JSON must not carry a `merged` key at all — a consumer deciding an
		// irreversible action has to handle its absence.
		b, _ := json.Marshal(ts)
		if strings.Contains(string(b), `"merged"`) {
			t.Errorf("unknown tombstone JSON must omit `merged`; got %s", b)
		}
	})
}

// ---------------------------------------------------------------------------
// #241 — a declared human gate can never read as MERGE-NOW
// ---------------------------------------------------------------------------

func TestHumanGateDeclaration_241(t *testing.T) {
	cases := []struct {
		name, title, body string
		labels            []string
		want              bool
	}{
		{"title marker", "[HUMAN GATE] statusgen anti-falsification surface", "", nil, true},
		{"title marker hyphen lower", "fix: [human-gate] integrity check", "", nil, true},
		{"label", "ordinary title", "", []string{"human-gate"}, true},
		{"label mixed case", "ordinary title", "", []string{"Human-Gate"}, true},
		{"body key", "ordinary title", "## Summary\n\nGate: human\n\nmore", nil, true},
		{"body key no space", "ordinary title", "gate:human", nil, true},
		{"prose is NOT a declaration", "ordinary title",
			"this is a human gate in my opinion and the gate is human", nil, false},
		{"unrelated label", "ordinary title", "", []string{"bug", "gate"}, false},
		{"nothing", "ordinary title", "body", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason := humanGateDeclared(c.title, c.body, c.labels)
			if got != c.want {
				t.Fatalf("humanGateDeclared = %t (%s), want %t", got, reason, c.want)
			}
			if got && reason == "" {
				t.Error("a declared gate must name the form that declared it")
			}
		})
	}
}

func TestHumanGate_NeverMergeNow_241(t *testing.T) {
	head := "cafebabe1234"
	risky := func(t *testing.T, title string) actionsReport {
		installFakeGH(t)
		onePR(t, `{"number":223,"title":`+mustJSON(title)+`,"body":"","isDraft":true,`+
			`"author":{"login":"shared-agent"},"createdAt":"`+time.Now().Add(-10*time.Minute).UTC().Format(time.RFC3339)+`",`+
			`"labels":[],"headRefOid":"`+head+`","headRefName":"fix/statusgen","mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview(head, "## Review\n\nVerdict: approve\n"))
		return actionsJSON(t)
	}

	t.Run("fires — declared gate is terminal", func(t *testing.T) {
		rep := risky(t, "[HUMAN GATE] statusgen anti-falsification surface")
		row := findRow(t, rep, 223)
		if row.Action != actHumanGate {
			t.Fatalf("action = %s, want %s (the #241 live instance was MERGE-NOW)", row.Action, actHumanGate)
		}
		if !row.HumanGate || row.HumanGateReason == "" {
			t.Error("row must carry the declaration and its source")
		}
		if strings.Contains(strings.ToLower(row.Note), "merge now") {
			t.Errorf("a human-gate row must never instruct a merge; note = %q", row.Note)
		}
		if !strings.Contains(row.Note, "MERGE is the human's") {
			t.Errorf("note must say who owns the merge; got %q", row.Note)
		}
		if rep.Header.MergeNowCount != 0 || rep.Header.MergeNowDecay {
			t.Errorf("a human-gate PR must not feed the MERGE-NOW count/decay alarm: count=%d decay=%t",
				rep.Header.MergeNowCount, rep.Header.MergeNowDecay)
		}
	})

	t.Run("silent — no declaration, still MERGE-NOW", func(t *testing.T) {
		rep := risky(t, "feat: an ordinary change")
		row := findRow(t, rep, 223)
		if row.Action != actMergeNow {
			t.Fatalf("action = %s, want %s — the gate must not fire on ordinary PRs", row.Action, actMergeNow)
		}
		if row.HumanGate {
			t.Error("humanGate set on a PR carrying no declaration")
		}
	})
}

// ---------------------------------------------------------------------------
// #268 — the CI reducer's two blind spots, and the marker the two tools read
// differently
// ---------------------------------------------------------------------------

func TestCIState_StatusContextAndUnknown_268(t *testing.T) {
	mk := func(rollup string) prBase {
		var p prBase
		if err := json.Unmarshal([]byte(`{"statusCheckRollup":`+rollup+`}`), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name                              string
		rollup                            string
		pass, pending, fail, unknownCount int
	}{
		{"CheckRun success (unchanged)", `[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]`, 1, 0, 0, 0},
		{"CheckRun running (unchanged)", `[{"__typename":"CheckRun","status":"IN_PROGRESS","conclusion":"","name":"ci"}]`, 0, 1, 0, 0},
		{"CheckRun failure (unchanged)", `[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"FAILURE","name":"ci"}]`, 0, 0, 1, 0},
		{"StatusContext success was PENDING FOREVER", `[{"__typename":"StatusContext","context":"legacy","state":"SUCCESS"}]`, 1, 0, 0, 0},
		{"StatusContext pending", `[{"__typename":"StatusContext","context":"legacy","state":"PENDING"}]`, 0, 1, 0, 0},
		{"StatusContext failure", `[{"__typename":"StatusContext","context":"legacy","state":"FAILURE"}]`, 0, 0, 1, 0},
		{"StatusContext error", `[{"__typename":"StatusContext","context":"legacy","state":"ERROR"}]`, 0, 0, 1, 0},
		{"unrecognised state is counted, not absorbed", `[{"__typename":"StatusContext","context":"x","state":"MOONBEAM"}]`, 0, 0, 0, 1},
		{"undecodable entry is counted", `[{"__typename":"FutureNode"}]`, 0, 0, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, pe, f, u := ciState(mk(c.rollup))
			if p != c.pass || pe != c.pending || f != c.fail || u != c.unknownCount {
				t.Errorf("ciState = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
					p, pe, f, u, c.pass, c.pending, c.fail, c.unknownCount)
			}
		})
	}
}

func TestCIUnknown_BlocksTheFlip_268(t *testing.T) {
	head := "abc123abc123"
	board := func(t *testing.T, rollup string) actionsReport {
		installFakeGH(t)
		onePR(t, `{"number":9,"title":"t","body":"","isDraft":true,"author":{"login":"shared-agent"},`+
			`"createdAt":"`+time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339)+`","labels":[],`+
			`"headRefOid":"`+head+`","headRefName":"b","mergeStateStatus":"CLEAN","statusCheckRollup":`+rollup+`}`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview(head, "## Review\n\nVerdict: approve\n"))
		return actionsJSON(t)
	}

	t.Run("fires — an uninterpretable rollup is not green and not pending", func(t *testing.T) {
		rep := board(t, `[{"__typename":"StatusContext","context":"x","state":"MOONBEAM"}]`)
		row := findRow(t, rep, 9)
		if row.Action != actCIUnknown {
			t.Fatalf("action = %s, want %s (before the fix this was FLIP: fail=0, pending=0 read as green)", row.Action, actCIUnknown)
		}
		if row.CIUnknown != 1 {
			t.Errorf("ciUnknown = %d, want 1", row.CIUnknown)
		}
	})

	t.Run("fires — a StatusContext-only repo can finally go green", func(t *testing.T) {
		rep := board(t, `[{"__typename":"StatusContext","context":"legacy-ci","state":"SUCCESS"}]`)
		row := findRow(t, rep, 9)
		if row.Action != actMergeNow {
			t.Fatalf("action = %s, want %s — a legacy commit status used to count as pending forever", row.Action, actMergeNow)
		}
	})

	t.Run("silent — an ordinary CheckRun board says nothing about unknowns", func(t *testing.T) {
		installFakeGH(t)
		onePR(t, `{"number":9,"title":"t","body":"","isDraft":true,"author":{"login":"shared-agent"},`+
			`"createdAt":"`+time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339)+`","labels":[],`+
			`"headRefOid":"`+head+`","headRefName":"b","mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview(head, "## Review\n\nVerdict: approve\n"))
		table := actionsTable(t)
		if strings.Contains(table, "?unk") || strings.Contains(table, actCIUnknown) {
			t.Errorf("the fourth CI state must be invisible on a healthy board; got:\n%s", table)
		}
	})
}

// TestSecurityMarker_DivergenceIsReported_268 and its mirror-spelling pin used to live
// here (at#268 divergence 2: this board's accepted Security-Review spelling was a strict
// SUBSET of deskpost's, and this reported the disagreement rather than silently refusing
// forever). PR #584 (merged after this branch last synced) fixed the
// divergence STRUCTURALLY instead: deskboard now calls deskkit.HasSecurityReviewPass /
// HasSecurityReviewFail directly (tools/desk/internal/deskkit/verdictmarker.go) — the
// same emphasis-tolerant, case-insensitive reader deskpost's bodycheck delegates to. The
// two tools' accepted sets are now IDENTICAL, not a subset relationship, so there is
// nothing left for a divergence-mirror to detect: a body this board and deskpost would
// ever have disagreed about no longer exists. Removed rather than re-targeted; see
// securityorder_test.go for the coverage of the unified reader.

// ---------------------------------------------------------------------------
// #359 — the board says what it covers, by repo and by time
// ---------------------------------------------------------------------------

// countBoardScopeLines counts the board's OWN scope footer — a line that IS the scope
// statement, not any line mentioning the word. #295's `main-health:` summary carries its
// own mid-line `— scope: N watched repo(s), lookback …` clause describing what the branch
// probe covered; that is a different instrument's coverage statement and counting it as a
// second board scope line would fail the noise budget for the wrong reason (and, worse,
// a substring count would let the real footer go missing while a health clause kept the
// tally at one).
func countBoardScopeLines(s string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.HasPrefix(ln, "scope: ") {
			n++
		}
	}
	return n
}

func TestScope_HeaderPresentOnSweeps_AbsentElsewhere_359(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":1,"title":"t","body":"","isDraft":true,"author":{"login":"shared-agent"},"createdAt":"`+
			time.Now().UTC().Format(time.RFC3339)+`","labels":[],"headRefOid":"abc123","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)

	sweeps := [][]string{{"prs"}, {"actions"}, {"queue"}}
	for _, argv := range sweeps {
		var out, errb bytes.Buffer
		if code := run(argv, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(%v) = exit %d: %s", argv, code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatalf("%v: %v", argv, err)
		}
		scope, ok := m["scope"].(map[string]any)
		if !ok {
			t.Fatalf("%v: a sweeping verb must state its scope; got %v", argv, m["scope"])
		}
		if int(scope["count"].(float64)) != len(deskkit.AllowedRepos()) {
			t.Errorf("%v: scope count %v disagrees with the set the loop iterates", argv, scope["count"])
		}
	}

	// Verbs that take an explicit repo swept NOTHING: the field must be ABSENT, so
	// absent can never be read as "the set was empty" (the #377 rule).
	repo := "example-org/tracker"
	for _, argv := range [][]string{{"reviews", repo, "1"}, {"files", repo, "1"}} {
		var out, errb bytes.Buffer
		if code := run(argv, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(%v) = exit %d: %s", argv, code, errb.String())
		}
		if strings.Contains(out.String(), `"scope"`) {
			t.Errorf("%v: a non-sweeping verb must OMIT scope entirely; got:\n%s", argv, out.String())
		}
	}

	// The table path states it in one line — always, and only one.
	table := actionsTable(t)
	if n := countBoardScopeLines(table); n != 1 {
		t.Errorf("scope line appears %d times, want exactly 1:\n%s", n, table)
	}
}

func TestScope_ReconcilesAgainstTheOwners_359(t *testing.T) {
	pr := func(repo string, num int, age time.Duration) string {
		return `{"number":` + itoa(num) + `,"title":"t","createdAt":"` +
			time.Now().Add(-age).UTC().Format(time.RFC3339) + `","repository":{"nameWithOwner":"` + repo + `"}}`
	}

	t.Run("fires — an unwatched repo with open PRs is named", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_SEARCH_JSON", `[`+
			pr("medici-finance/assay", 359, time.Hour)+`,`+
			pr("unwatched-org/never-watched", 3, 12*24*time.Hour)+`,`+
			pr("example-org/example-reconciler", 51, 2*time.Hour)+`]`)
		var out, errb bytes.Buffer
		if code := run([]string{"scope", "--table"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("scope exited %d: %s", code, errb.String())
		}
		if !strings.Contains(out.String(), "UNWATCHED: unwatched-org/never-watched") {
			t.Errorf("the 12-day-old approved PR's repo must be named; got:\n%s", out.String())
		}
		if strings.Contains(out.String(), "UNWATCHED: medici-finance/assay") {
			t.Errorf("a WATCHED repo must not be reported as a gap; got:\n%s", out.String())
		}
	})

	t.Run("silent — no gap, one summary line and no alarm", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_SEARCH_JSON", `[`+pr("medici-finance/assay", 359, time.Hour)+`]`)
		var out, errb bytes.Buffer
		if code := run([]string{"scope", "--table"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("scope exited %d: %s", code, errb.String())
		}
		if strings.Contains(out.String(), "UNWATCHED:") {
			t.Errorf("no gap must print no alarm; got:\n%s", out.String())
		}
		if !strings.Contains(out.String(), "scope-check:") {
			t.Errorf("the summary line must always print; got:\n%s", out.String())
		}
	})

	t.Run("a search it could not run is exit 6, never 'no gaps'", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_FAIL_REPO", "medici-finance") // the owner arg of the search
		var out, errb bytes.Buffer
		code := run([]string{"scope"}, &out, &errb)
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("scope = exit %d, want %d — an unperformed reconciliation must never render as clean",
				code, deskkit.ExitUnverifiable)
		}
		if strings.Contains(out.String(), `"unwatched"`) {
			t.Errorf("a failed run must emit no report; got:\n%s", out.String())
		}
	})

	t.Run("an unattributable search row fails the run", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_SEARCH_JSON", `[{"number":1,"title":"t","createdAt":"","repository":{}}]`)
		var out, errb bytes.Buffer
		if code := run([]string{"scope"}, &out, &errb); code != deskkit.ExitUnverifiable {
			t.Fatalf("scope = exit %d, want %d", code, deskkit.ExitUnverifiable)
		}
	})
}

func TestUnreviewed_TemporalSubset_359(t *testing.T) {
	board := func(t *testing.T, age time.Duration) (actionsReport, string) {
		installFakeGH(t)
		onePR(t, `{"number":350,"title":"another window's work","body":"","isDraft":false,`+
			`"author":{"login":"shared-agent"},"createdAt":"`+time.Now().Add(-age).UTC().Format(time.RFC3339)+`",`+
			`"labels":[],"headRefOid":"aaa111","headRefName":"b","mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`)
		// No reviews at all — the measured case: opened before the sweep window, never
		// picked up, and nothing ages it back into view.
		return actionsJSON(t), actionsTable(t)
	}

	t.Run("fires — a 12-hour-old never-reviewed PR is named", func(t *testing.T) {
		rep, table := board(t, 12*time.Hour)
		if rep.Header.UnreviewedCount != 1 {
			t.Fatalf("unreviewedCount = %d, want 1 (header: %+v)", rep.Header.UnreviewedCount, rep.Header)
		}
		if len(rep.Header.UnreviewedPRs) != 1 || !strings.HasSuffix(rep.Header.UnreviewedPRs[0], "#350") {
			t.Errorf("the stranded PR must be named: %v", rep.Header.UnreviewedPRs)
		}
		if !strings.Contains(table, "UNREVIEWED:") {
			t.Errorf("the table must name it too; got:\n%s", table)
		}
		if row := findRow(t, rep, 350); row.OpenAge == "" {
			t.Error("every row should carry its open age")
		}
	})

	t.Run("silent — a fresh unreviewed PR raises nothing", func(t *testing.T) {
		rep, table := board(t, 3*time.Minute)
		if rep.Header.UnreviewedCount != 0 || len(rep.Header.UnreviewedPRs) != 0 {
			t.Errorf("a PR opened minutes ago must not trip the alarm: %+v", rep.Header.UnreviewedPRs)
		}
		if strings.Contains(table, "UNREVIEWED:") {
			t.Errorf("no alarm expected on a fresh board; got:\n%s", table)
		}
	})

	t.Run("silent — an old PR that WAS reviewed raises nothing", func(t *testing.T) {
		installFakeGH(t)
		onePR(t, `{"number":350,"title":"reviewed and waiting","body":"","isDraft":false,`+
			`"author":{"login":"shared-agent"},"createdAt":"`+time.Now().Add(-12*time.Hour).UTC().Format(time.RFC3339)+`",`+
			`"labels":[],"headRefOid":"aaa111","headRefName":"b","mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview("aaa111", "## Review\n\nVerdict: approve\n"))
		rep := actionsJSON(t)
		if rep.Header.UnreviewedCount != 0 {
			t.Errorf("'reviewed and waiting' is not 'never seen'; count = %d", rep.Header.UnreviewedCount)
		}
	})
}

// ---------------------------------------------------------------------------
// #321 — the verb named for the dispatch queue returns the verification backlog
// ---------------------------------------------------------------------------

func TestAwaiting_PopulationIsStated_321(t *testing.T) {
	t.Run("canonical verb states its population", func(t *testing.T) {
		installFakeStatusgen(t)
		twoRoots(t)
		var out, errb bytes.Buffer
		if code := run([]string{"awaiting"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("awaiting exited %d: %s", code, errb.String())
		}
		var rep nextupReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("not JSON: %v", err)
		}
		if rep.Population != populationAwaiting {
			t.Errorf("population = %q, want %q", rep.Population, populationAwaiting)
		}
		if !strings.Contains(rep.PopulationNote, "NOT the dispatch queue") {
			t.Errorf("the note must rule out the dispatch reading; got %q", rep.PopulationNote)
		}
		if rep.AliasUsed != "" {
			t.Errorf("aliasUsed set on the canonical verb: %q", rep.AliasUsed)
		}
	})

	t.Run("the nextup alias still works and says it is misnamed", func(t *testing.T) {
		installFakeStatusgen(t)
		twoRoots(t)
		var out, errb bytes.Buffer
		if code := run([]string{"nextup", "--table"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("nextup exited %d: %s", code, errb.String())
		}
		if !strings.Contains(out.String(), "AWAITING-VERIFICATION") {
			t.Errorf("the table must name the population it renders; got:\n%s", out.String())
		}
		if !strings.Contains(out.String(), "deprecated alias") {
			t.Errorf("the alias must announce itself; got:\n%s", out.String())
		}
	})

	t.Run("the dispatch queue is a SERVED verb, distinct population from awaiting", func(t *testing.T) {
		// #321: dispatch (and its aliases) is no longer refused — it is served by
		// statusgen --next-up (todo/in-progress, unclaimed), a different population
		// from awaiting (implemented/verified). It states that population in-band.
		for _, verb := range []string{"dispatch", "todo", "next", "next-up"} {
			installFakeStatusgen(t)
			twoRoots(t)
			var out, errb bytes.Buffer
			code := run([]string{verb}, &out, &errb)
			if code != deskkit.ExitOK {
				t.Fatalf("%s = exit %d, want %d (dispatch is now served): %s", verb, code, deskkit.ExitOK, errb.String())
			}
			var rep dispatchReport
			if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
				t.Fatalf("%s: not JSON: %v\n%s", verb, err, out.String())
			}
			if rep.Population != populationDispatch {
				t.Errorf("%s: population = %q, want %q", verb, rep.Population, populationDispatch)
			}
			// The dispatch population must be todo/in-progress — NEVER the
			// awaiting-verification statuses, or the two verbs would collapse.
			for _, st := range rep.PopulationStatuses {
				if st == "implemented" || st == "verified" {
					t.Errorf("%s: dispatch population includes awaiting status %q: %v", verb, st, rep.PopulationStatuses)
				}
			}
			// The fake --next-up emits one todo row per root — proof it is the
			// dispatch population, not the implemented rows --gate-scores emits.
			for _, r := range rep.Rows {
				if r.Status != "todo" && r.Status != "in-progress" {
					t.Errorf("%s: dispatch row %s has non-dispatch status %q", verb, r.Brief, r.Status)
				}
			}
		}
	})
}

// ---------------------------------------------------------------------------
// small test helpers
// ---------------------------------------------------------------------------

func mustJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// ---------------------------------------------------------------------------
// Review follow-ups (review): the survivals.
//
// Every test below exists because a mutation to correct code passed the suite in
// SILENCE. A guarantee nothing can break is not a guarantee — it is a comment that
// happens to compile — and on this PR in particular an unpinned fail-closed branch is
// the same defect class the PR is about, one level up.
// ---------------------------------------------------------------------------

// withStaleSeams drives staleState()'s two MEASURING branches. Production calls
// deskkit.IsPinned and real git; these seams exist so drift-vs-in-sync — the branches a
// pinned install actually runs, and the ones #236 was about — can be asserted at all.
func withStaleSeams(t *testing.T, pinned bool, trees map[string]string, fail bool) {
	t.Helper()
	oldPinned, oldTree, oldPin := isPinned, gitTree, deskToolsPin
	t.Cleanup(func() { isPinned, gitTree, deskToolsPin = oldPinned, oldTree, oldPin })
	// Default the #185 primary source OFF so these tests deterministically exercise
	// the in-tree ref FALLBACK (the branch they were written for), independent of
	// whether any `.assay-versions` happens to sit above the test's working dir.
	// A pin: not found here.
	deskToolsPin = func() (string, string, bool) { return "", "", false }
	isPinned = func() bool { return pinned }
	gitTree = func(ref string) (string, error) {
		if fail {
			return "", fmt.Errorf("simulated: git unavailable / ref missing")
		}
		if v, ok := trees[ref]; ok {
			return v, nil
		}
		return trees["*"], nil
	}
}

func TestStale_MeasuringStates_236(t *testing.T) {
	oldS, oldB := deskkit.SourceSHA, deskkit.BuiltAt
	t.Cleanup(func() { deskkit.SourceSHA, deskkit.BuiltAt = oldS, oldB })
	deskkit.SourceSHA, deskkit.BuiltAt = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "2026-07-31T00:00:00Z"

	t.Run("fires — a MEASURED drift is stale and says so", func(t *testing.T) {
		// This is #236 verbatim on the live path: the installed tree really does
		// differ from origin/main. Answering in-sync/fresh here is the whole bug, and
		// before this test that mutation passed the entire suite.
		withStaleSeams(t, true, map[string]string{
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef": "tree-installed",
			// FULLY-QUALIFIED remote-tracking ref: staleState resolves
			// refs/remotes/origin/main, not the bare short name a stray local
			// `refs/heads/origin/main` decoy could shadow (#885).
			"refs/remotes/origin/main": "tree-origin",
		}, false)
		state, stale, detail := staleState()
		if state != staleStateDrift {
			t.Fatalf("state = %q, want %q", state, staleStateDrift)
		}
		if !stale {
			t.Fatalf("stale = false on a MEASURED drift — the authoritative boolean must agree with the state (detail: %s)", detail)
		}
		if !strings.Contains(detail, "reinstall") {
			t.Errorf("detail must say what to do about the drift; got %q", detail)
		}
		var banner bytes.Buffer
		printBanners(&banner, Header{StaleState: state, Stale: stale, StaleDetail: detail})
		if !strings.Contains(banner.String(), "STALE") {
			t.Errorf("a measured drift must raise the banner; got %q", banner.String())
		}
		if strings.Contains(banner.String(), "STALE-UNKNOWN") {
			t.Errorf("a MEASURED drift must not render as could-not-check; got %q", banner.String())
		}
	})

	t.Run("silent — a matching tree is in-sync, fresh, and quiet", func(t *testing.T) {
		withStaleSeams(t, true, map[string]string{"*": "same-tree"}, false)
		state, stale, detail := staleState()
		if state != staleStateInSync {
			t.Fatalf("state = %q, want %q (detail %q)", state, staleStateInSync, detail)
		}
		if stale {
			t.Error("a measured match must report stale=false — a detector that alarms when it agrees gets trained away")
		}
		var banner bytes.Buffer
		printBanners(&banner, Header{StaleState: state, Stale: stale, StaleDetail: detail})
		if strings.Contains(banner.String(), "STALE") {
			t.Errorf("the healthy case must print nothing; got %q", banner.String())
		}
	})

	t.Run("a pinned build whose git read FAILS is unknown, never in-sync", func(t *testing.T) {
		withStaleSeams(t, true, nil, true)
		state, stale, _ := staleState()
		if state != staleStateUnknown || !stale {
			t.Fatalf("state=%q stale=%t, want unknown/true", state, stale)
		}
	})
}

// ---------------------------------------------------------------------------
// #185 — the drift check keys on the consumer's .assay-versions pin, so a
// consumer checkout (no in-tree tools/desk) is no longer permanently
// STALE-UNKNOWN. The PRIMARY source is the running binary's releaseTag vs the
// desk-tools tag the consumer pins; the in-tree git ref is a fallback.
// ---------------------------------------------------------------------------

// withPinSeams drives staleState()'s #185 PRIMARY branch: it stubs the
// `.assay-versions` desk-tools pin lookup and the running binary's releaseTag, and
// forces the in-tree git fallback to FAIL — exactly the consumer-checkout shape
// (`origin/main:tools/desk` does not resolve) the issue reported. A run that reaches
// the fallback here is a run whose primary branch did not answer.
func withPinSeams(t *testing.T, pinFound bool, pinTag, runningTag string) {
	t.Helper()
	oldPinned, oldTree, oldPin := isPinned, gitTree, deskToolsPin
	oldRelease := deskkit.ReleaseTag
	t.Cleanup(func() {
		isPinned, gitTree, deskToolsPin = oldPinned, oldTree, oldPin
		deskkit.ReleaseTag = oldRelease
	})
	isPinned = func() bool { return true }
	gitTree = func(string) (string, error) {
		return "", fmt.Errorf("simulated consumer checkout: origin/main:tools/desk does not resolve")
	}
	deskToolsPin = func() (string, string, bool) {
		if !pinFound {
			return "", "", false
		}
		return "/consumer/repo", pinTag, true
	}
	deskkit.ReleaseTag = runningTag
}

func TestStale_ConsumerPin_185(t *testing.T) {
	oldS, oldB := deskkit.SourceSHA, deskkit.BuiltAt
	t.Cleanup(func() { deskkit.SourceSHA, deskkit.BuiltAt = oldS, oldB })
	deskkit.SourceSHA, deskkit.BuiltAt = "aaf6d8a", "2026-08-25T22:57:16Z"

	t.Run("matching pin is in-sync, NOT STALE-UNKNOWN (the #185 regression)", func(t *testing.T) {
		// The issue verbatim: a pinned binary run from a consumer checkout where
		// `git rev-parse origin/main:tools/desk` does not resolve. Before #185 this
		// went STALE-UNKNOWN forever; now the running releaseTag matches the pinned
		// desk-tools tag, so it is in-sync and quiet.
		withPinSeams(t, true, "v0.18.0", "v0.18.0")
		state, stale, detail := staleState()
		if state != staleStateInSync {
			t.Fatalf("state = %q, want %q (detail %q)", state, staleStateInSync, detail)
		}
		if stale {
			t.Errorf("a matching consumer pin must report stale=false; detail %q", detail)
		}
		var banner bytes.Buffer
		printBanners(&banner, Header{StaleState: state, Stale: stale, StaleDetail: detail})
		if strings.Contains(banner.String(), "STALE") {
			t.Errorf("a matched pin must not raise STALE-UNKNOWN (the #185 bug); got %q", banner.String())
		}
	})

	t.Run("differing pin is a MEASURED drift, not could-not-check", func(t *testing.T) {
		withPinSeams(t, true, "v0.18.0", "v0.17.0")
		state, stale, detail := staleState()
		if state != staleStateDrift {
			t.Fatalf("state = %q, want %q (detail %q)", state, staleStateDrift, detail)
		}
		if !stale {
			t.Error("a measured pin mismatch must report stale=true")
		}
		if !strings.Contains(detail, "v0.17.0") || !strings.Contains(detail, "v0.18.0") {
			t.Errorf("drift detail must name both the running and pinned tags; got %q", detail)
		}
		var banner bytes.Buffer
		printBanners(&banner, Header{StaleState: state, Stale: stale, StaleDetail: detail})
		if strings.Contains(banner.String(), "STALE-UNKNOWN") {
			t.Errorf("a MEASURED pin drift must not render as could-not-check; got %q", banner.String())
		}
	})

	t.Run("component-prefixed and plain tags normalize equal", func(t *testing.T) {
		// The running stamp and the consumer pin may each be written plain (`v0.18.0`)
		// or component-prefixed (`desk-tools/v0.18.0`); they name the same release and
		// must not read as drift.
		withPinSeams(t, true, "desk-tools/v0.18.0", "v0.18.0")
		state, _, detail := staleState()
		if state != staleStateInSync {
			t.Fatalf("state = %q, want %q (detail %q) — tag normalization failed", state, staleStateInSync, detail)
		}
	})

	t.Run("no pin AND no in-tree ref is could-not-check, naming both sources", func(t *testing.T) {
		// A consumer with no desk-tools pin and no in-tree tools/desk: the honest
		// third state. Fails CLOSED (stale=true), and the reason names which sources
		// were missing rather than rounding up to fresh.
		withPinSeams(t, false, "", "v0.18.0")
		state, stale, detail := staleState()
		if state != staleStateUnknown || !stale {
			t.Fatalf("state=%q stale=%t, want unknown/true (detail %q)", state, stale, detail)
		}
		if !strings.Contains(detail, "COULD-NOT-CHECK") {
			t.Errorf("detail must announce it could not check; got %q", detail)
		}
		if !strings.Contains(detail, ".assay-versions") || !strings.Contains(detail, "tools/desk") {
			t.Errorf("could-not-check reason must name BOTH missing sources (the pin and the in-tree ref); got %q", detail)
		}
	})

	t.Run("pin present but binary carries no releaseTag stamp falls back", func(t *testing.T) {
		// An older stamped binary is pinned (sourceSHA/builtAt) but has no releaseTag,
		// so the tag comparison cannot run. It must fall through to the in-tree ref —
		// which the consumer lacks — and land on could-not-check, never a false match.
		withPinSeams(t, true, "v0.18.0", "")
		state, stale, detail := staleState()
		if state != staleStateUnknown || !stale {
			t.Fatalf("state=%q stale=%t, want unknown/true (detail %q)", state, stale, detail)
		}
	})
}

// ---------------------------------------------------------------------------
// #247 — the tombstone states the suite used to skip
// ---------------------------------------------------------------------------

// seedTombstone plants a prior-sweep audit line for tracker#1580 (the live instance) and
// returns the one tombstone the next sweep produces.
func seedTombstone(t *testing.T) tombstone {
	t.Helper()
	if err := deskkit.Log(deskkit.Entry{
		Tool: "deskboard", Verb: "actions", Result: deskkit.ResultOK,
		Detail: "open=example-org/tracker#1580",
		TS:     time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seeding audit: %v", err)
	}
	rep := actionsJSON(t)
	if len(rep.Tombstones) != 1 {
		t.Fatalf("want exactly one tombstone, got %+v", rep.Tombstones)
	}
	return rep.Tombstones[0]
}

func TestTombstone_ReopenedIsNotMerged_247(t *testing.T) {
	t.Run("fires — a PR reopened between sweeps is back ON the list", func(t *testing.T) {
		installFakeGH(t) // no PR list → #1580 is gone from the open set…
		t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `{"state":"open","merged":false,"merged_at":null}`)
		ts := seedTombstone(t) // …but the API says it is open again
		if ts.State != prStateOpen {
			t.Fatalf("state=%q, want %q — a reopened PR reported as MERGED is #247 verbatim", ts.State, prStateOpen)
		}
		if strings.Contains(ts.Note, "MERGED") {
			t.Errorf("a reopened PR must never be labelled MERGED; got %q", ts.Note)
		}
		if !strings.Contains(ts.Note, "REOPENED") {
			t.Errorf("note must name the state; got %q", ts.Note)
		}
		// `merged` is omitted: it is not false-and-known, it is inapplicable.
		b, _ := json.Marshal(ts)
		if strings.Contains(string(b), `"merged"`) {
			t.Errorf("a reopened tombstone must omit `merged` rather than answer it; got %s", b)
		}
	})

	t.Run("silent — an ordinary merged PR is unaffected", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `{"state":"closed","merged":true,"merged_at":"2026-07-31T14:01:35Z"}`)
		ts := seedTombstone(t)
		if ts.State != prStateMerged {
			t.Fatalf("state=%q, want %q — the reopen lane must not swallow a real merge", ts.State, prStateMerged)
		}
		if strings.Contains(ts.Note, "REOPENED") {
			t.Errorf("no reopen text may appear on a merged PR; got %q", ts.Note)
		}
	})
}

func TestTombstone_ReadFailureFailsClosed_247(t *testing.T) {
	// The PR-state read failing is the case a tombstone lane MUST fail closed on: the
	// board knows the PR left the open set and knows nothing about how. Both failure
	// branches (transport and parse) previously mutated to `merged` in silence — and a
	// wrong MERGED here is what the intake-desk makes an irreversible write off.
	for _, c := range []struct{ name, setup string }{
		{"the API call FAILED", "fail"},
		{"the payload did not PARSE", "garbage"},
	} {
		t.Run("fires — "+c.name+" is unknown, never merged", func(t *testing.T) {
			installFakeGH(t)
			if c.setup == "fail" {
				t.Setenv("DESKBOARD_GH_FAIL_MATCH", "/pulls/1580")
			} else {
				t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `<!DOCTYPE html><html>rate limited</html>`)
			}
			ts := seedTombstone(t)
			if ts.State != prStateUnknown {
				t.Fatalf("state=%q, want %q", ts.State, prStateUnknown)
			}
			if ts.Merged != nil {
				t.Fatalf("merged=%v — an unknown state must carry NO boolean; a consumer must be forced to handle its absence", *ts.Merged)
			}
			if !strings.Contains(ts.Note, "COULD-NOT-CHECK") {
				t.Errorf("note must name the could-not-check; got %q", ts.Note)
			}
			if strings.Contains(ts.Note, "MERGED —") {
				t.Errorf("an unreadable state must never be labelled MERGED; got %q", ts.Note)
			}
		})
	}

	t.Run("silent — a readable payload is unaffected by the failure lane", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `{"state":"closed","merged":false,"merged_at":null}`)
		ts := seedTombstone(t)
		if ts.State != prStateClosed {
			t.Fatalf("state=%q, want %q", ts.State, prStateClosed)
		}
		if strings.Contains(ts.Note, "COULD-NOT-CHECK") {
			t.Errorf("a read that succeeded must not claim it could not check; got %q", ts.Note)
		}
	})
}

// TestPRPayload_OneEndpointServesBothReaders pins the #382 merge resolution. GET
// repos/{o}/{r}/pulls/{n} has TWO readers in this binary — fetchChangedFiles'
// changed_files reconciliation and fetchPRState's merged-vs-closed read. The two shim
// arms that arrived from the two branches matched the same endpoint, where the first
// wins for both; a naive resolution starves one reader in silence, and the starved one
// FAILS CLOSED, so the suite would still be green while the board stopped measuring.
func TestPRPayload_OneEndpointServesBothReaders(t *testing.T) {
	installFakeGH(t)
	// ONE payload carrying BOTH readers' fields, as real GitHub returns.
	t.Setenv("DESKBOARD_GH_PRSTATE_JSON",
		`{"state":"closed","merged":false,"merged_at":null,"changed_files":1}`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`)
	// An open PR whose head advanced past the review, so the changed-file read runs.
	head, reviewed := "aaaa1111", "bbbb2222"
	t.Setenv("DESKBOARD_GH_PR_REPO", "example-org/tracker")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", `[{"number":1,"title":"t","body":"","isDraft":true,`+
		`"author":{"login":"shared-agent"},"createdAt":"`+time.Now().UTC().Format(time.RFC3339)+`",`+
		`"labels":[],"headRefOid":"`+head+`","headRefName":"b","mergeStateStatus":"CLEAN",`+
		`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview(reviewed, "## Review\n\nVerdict: approve\n"))
	// …and a DIFFERENT PR seeded as gone, so the state read runs in the same sweep.
	if err := deskkit.Log(deskkit.Entry{
		Tool: "deskboard", Verb: "actions", Result: deskkit.ResultOK,
		Detail: "open=example-org/tracker#1580",
		TS:     time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seeding audit: %v", err)
	}

	rep := actionsJSON(t)

	// Reader 1: the tombstone lane got a usable state.
	if len(rep.Tombstones) != 1 || rep.Tombstones[0].State != prStateClosed {
		t.Fatalf("the PR-STATE reader was starved by the shared endpoint: %+v", rep.Tombstones)
	}
	// Reader 2: the changed-file reconciliation got its count and did NOT degrade to
	// truncated (which is how a starved meta read shows up — fail closed, still green).
	row := findRow(t, rep, 1)
	if strings.Contains(row.Note, "TRUNCATED") {
		t.Errorf("the CHANGED-FILES reader was starved by the shared endpoint; note = %q", row.Note)
	}
}

// ---------------------------------------------------------------------------
// #359 — the scope statement, on every surface that claims coverage
// ---------------------------------------------------------------------------

// TestScope_TableLineOnEverySweep_359 pins the table line on EVERY sweeping verb, not
// just `actions`. Deleting renderScopeLine from `prs` or `queue` used to pass the whole
// suite: two of the three human-readable surfaces could lose their coverage statement
// with nothing going red — a coverage claim that is itself uncovered.
func TestScope_TableLineOnEverySweep_359(t *testing.T) {
	sweeps := [][]string{{"prs", "--table"}, {"actions", "--table"}, {"queue", "--table"},
		{"health", "--table"}, {"policydrift", "--table"}}
	for _, argv := range sweeps {
		t.Run(argv[0], func(t *testing.T) {
			installFakeGH(t)
			t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
			t.Setenv("DESKBOARD_GH_PRLIST_JSON",
				`[{"number":1,"title":"t","body":"","isDraft":true,"author":{"login":"shared-agent"},"createdAt":"`+
					time.Now().UTC().Format(time.RFC3339)+`","labels":[],"headRefOid":"abc123",`+
					`"mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
			var out, errb bytes.Buffer
			if code := run(argv, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("run(%v) = exit %d: %s", argv, code, errb.String())
			}
			if n := countBoardScopeLines(out.String()); n != 1 {
				t.Fatalf("%v: scope line appears %d times, want exactly 1 (one line is the whole noise budget):\n%s",
					argv, n, out.String())
			}
			if !strings.Contains(out.String(), "repos OUTSIDE this set are not read by this board") {
				t.Errorf("%v: the scope line must say what it EXCLUDES, not just what it covers:\n%s", argv, out.String())
			}
		})
	}

	// The JSON header of the fourth sweeping verb, for the same reason as the other three.
	t.Run("policydrift JSON carries scope", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
		var out, errb bytes.Buffer
		if code := run([]string{"policydrift"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("policydrift = exit %d: %s", code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		scope, ok := m["scope"].(map[string]any)
		if !ok {
			t.Fatalf("policydrift iterates AllowedRepos — it owes a scope; got %v", m["scope"])
		}
		if int(scope["count"].(float64)) != len(deskkit.AllowedRepos()) {
			t.Errorf("scope count %v disagrees with the set the loop iterates", scope["count"])
		}
	})
}

// TestScope_PageCapIsFlagged_359 pins the ONE guarantee against a silent truncation of
// the reconciliation. `>=` mutating to `>` passed the suite; at exactly the cap the
// result set is indistinguishable from a truncated one, so the boundary is the case.
func TestScope_PageCapIsFlagged_359(t *testing.T) {
	rows := func(n int) string {
		parts := make([]string, 0, n)
		for i := 1; i <= n; i++ {
			parts = append(parts, `{"number":`+itoa(i)+`,"title":"t","createdAt":"`+
				time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)+
				`","repository":{"nameWithOwner":"medici-finance/assay"}}`)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}

	t.Run("fires — a result set AT the cap is reported as possibly incomplete", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_SEARCH_JSON", rows(searchLimit))
		var out, errb bytes.Buffer
		if code := run([]string{"scope", "--table"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("scope = exit %d: %s", code, errb.String())
		}
		if !strings.Contains(out.String(), "may be INCOMPLETE") {
			t.Fatalf("a search AT the --limit cap must be flagged — at the boundary, complete and truncated look identical:\n%s", out.String())
		}
		var jout, jerr bytes.Buffer
		if code := run([]string{"scope"}, &jout, &jerr); code != deskkit.ExitOK {
			t.Fatalf("scope = exit %d: %s", code, jerr.String())
		}
		var m map[string]any
		if err := json.Unmarshal(jout.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if m["truncated"] != true {
			t.Errorf("the machine path must carry the truncation flag too; got %v", m["truncated"])
		}
	})

	t.Run("silent — a result set BELOW the cap raises nothing", func(t *testing.T) {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_SEARCH_JSON", rows(searchLimit-1))
		var out, errb bytes.Buffer
		if code := run([]string{"scope", "--table"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("scope = exit %d: %s", code, errb.String())
		}
		if strings.Contains(out.String(), "INCOMPLETE") {
			t.Errorf("a complete reconciliation must not warn; got:\n%s", out.String())
		}
	})
}

// TestScope_StatesItsOwnBlindSpot_359 — the reconciliation verb owes the same honesty it
// enforces. `gh search prs` returns no rows AND no error for a repo the caller's token
// cannot read, so gap=false is "no gap OBSERVED", never "no gap EXISTS".
func TestScope_StatesItsOwnBlindSpot_359(t *testing.T) {
	installFakeGH(t)
	// The blind-spot case itself: a token that can see nothing returns an empty set,
	// which without the bound renders as a perfectly clean reconciliation.
	t.Setenv("DESKBOARD_GH_SEARCH_JSON", `[]`)

	var out, errb bytes.Buffer
	if code := run([]string{"scope", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("scope = exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "CALLER'S token") {
		t.Fatalf("the table must state the token bound — an empty search is not evidence of no gap:\n%s", out.String())
	}

	var jout, jerr bytes.Buffer
	if code := run([]string{"scope"}, &jout, &jerr); code != deskkit.ExitOK {
		t.Fatalf("scope = exit %d: %s", code, jerr.String())
	}
	var m map[string]any
	if err := json.Unmarshal(jout.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["gap"] != false {
		t.Fatalf("precondition: this fixture has no observable gap; got %v", m["gap"])
	}
	// #400 R4: `scope` sweeps AllowedRepos to build its watched list, so it owes the same
	// header statement the other four sweeping verbs carry. Deleting `hdr.Scope =
	// boardScope()` from cmdScope left the whole suite green — the verb whose entire job
	// is stating coverage was the one verb whose own coverage statement nothing pinned.
	// (TestDispatch_VerbInventory now enforces this over the WHOLE dispatcher; this
	// assertion stays because this is the verb the omission actually happened on.)
	scope, ok := m["scope"].(map[string]any)
	if !ok {
		t.Fatalf("`scope` sweeps the watched set and must state it in its own header; got %v", m["scope"])
	}
	if int(scope["count"].(float64)) != len(deskkit.AllowedRepos()) {
		t.Errorf("scope count %v disagrees with the set the verb reconciles", scope["count"])
	}
	bound, _ := m["observabilityBound"].(string)
	if !strings.Contains(bound, "NO GAP WAS OBSERVED") {
		t.Errorf("the machine path must carry the bound beside `gap`, or a consumer reads gap=false as proof; got %q", bound)
	}
	// #400 N2: `Contains(bound, "owner")` was satisfied by the earlier "listed in `owners`"
	// clause, so the SUBSTANTIVE sentence — the one saying an unlisted owner is never asked
	// about at all — could be deleted with this test green. Pin the phrase, not the word.
	if !strings.Contains(bound, "under any other owner is not asked about at all") {
		t.Errorf("the bound must state the OWNER axis in full — a repo under an owner the board does not "+
			"watch is never even queried, so it can never appear in `unwatched`; got %q", bound)
	}
}

// TestUnreviewed_AgeUnknownIsItsOwnState_359 — the third state of the temporal alarm. A
// never-reviewed PR whose createdAt cannot be read was skipped entirely: NEEDS-REVIEW,
// blank OPEN column, unreviewedCount 0, no line anywhere. Silence read as "reviewed".
func TestUnreviewed_AgeUnknownIsItsOwnState_359(t *testing.T) {
	board := func(t *testing.T, createdAt string) (actionsReport, string) {
		installFakeGH(t)
		onePR(t, `{"number":391,"title":"never picked up","body":"","isDraft":false,`+
			`"author":{"login":"shared-agent"},"createdAt":`+mustJSON(createdAt)+`,"labels":[],`+
			`"headRefOid":"abc123","headRefName":"b","mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", `[]`) // NO verdict at any head
		return actionsJSON(t), actionsTable(t)
	}

	for _, bad := range []struct{ name, value string }{
		{"createdAt missing", ""},
		{"createdAt unparseable", "not-a-time"},
	} {
		t.Run("fires — "+bad.name, func(t *testing.T) {
			rep, table := board(t, bad.value)
			row := findRow(t, rep, 391)
			if row.Action != actNeedsReview {
				t.Fatalf("precondition: want %s, got %s", actNeedsReview, row.Action)
			}
			if rep.Header.UnreviewedAgeUnknownCount != 1 {
				t.Fatalf("unreviewedAgeUnknownCount = %d, want 1 — an unmeasurable age must be a STATE, not a silence",
					rep.Header.UnreviewedAgeUnknownCount)
			}
			if len(rep.Header.UnreviewedAgeUnknownPRs) != 1 ||
				!strings.HasSuffix(rep.Header.UnreviewedAgeUnknownPRs[0], "#391") {
				t.Errorf("the PR must be NAMED, not just counted; got %v", rep.Header.UnreviewedAgeUnknownPRs)
			}
			if rep.Header.UnreviewedCount != 0 {
				t.Errorf("an unmeasured row must not join the MEASURED count (`aged past the threshold`); got %d",
					rep.Header.UnreviewedCount)
			}
			if !strings.Contains(table, "UNREVIEWED-AGE-UNKNOWN: tracker#391") {
				t.Fatalf("the table must carry a could-not-check marker for this row:\n%s", table)
			}
			if strings.Contains(table, "UNREVIEWED: tracker#391") {
				t.Errorf("an unmeasured age must not be asserted as aged-past-threshold:\n%s", table)
			}
		})
	}

	t.Run("silent — a readable age raises no could-not-check", func(t *testing.T) {
		rep, table := board(t, time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339))
		if rep.Header.UnreviewedAgeUnknownCount != 0 || rep.Header.UnreviewedAgeUnknownPRs != nil {
			t.Fatalf("a measurable age must raise nothing; got count=%d prs=%v",
				rep.Header.UnreviewedAgeUnknownCount, rep.Header.UnreviewedAgeUnknownPRs)
		}
		if strings.Contains(table, "UNREVIEWED-AGE-UNKNOWN") {
			t.Errorf("the could-not-check line must be invisible on a healthy board:\n%s", table)
		}
		// And the field is ABSENT from JSON, never a zero that reads as an answer.
		var out, errb bytes.Buffer
		if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("actions = exit %d: %s", code, errb.String())
		}
		if strings.Contains(out.String(), "unreviewedAgeUnknown") {
			t.Errorf("the field must be omitted when nothing was unmeasurable; got:\n%s", out.String())
		}
	})
}

// TestCIRed_BeatsCIUnknown_268 — a DEFINITE failure is a work item; an indefinite unknown
// is an instruction to go look. Reporting CI-UNKNOWN for a rollup carrying both hid the
// red behind the weaker statement. Both are stated: the action names the red, the note
// carries the count that was never established.
func TestCIRed_BeatsCIUnknown_268(t *testing.T) {
	head := "c0ffee001122"
	board := func(t *testing.T, rollup string) actionRow {
		installFakeGH(t)
		onePR(t, `{"number":12,"title":"t","body":"","isDraft":true,"author":{"login":"shared-agent"},`+
			`"createdAt":"`+time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339)+`","labels":[],`+
			`"headRefOid":"`+head+`","headRefName":"b","mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":`+rollup+`}`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview(head, "## Review\n\nVerdict: approve\n"))
		return findRow(t, actionsJSON(t), 12)
	}

	t.Run("fires — a real FAILURE alongside an undecodable entry reports the red", func(t *testing.T) {
		row := board(t, `[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"FAILURE","name":"go-test"},`+
			`{"__typename":"SomethingNew","weird":true}]`)
		if row.Action != actCIRed {
			t.Fatalf("action = %s, want %s — a definite red must not hide behind an indefinite unknown", row.Action, actCIRed)
		}
		if row.CIFail != 1 || row.CIUnknown != 1 {
			t.Errorf("both counts must survive to the row: fail=%d unknown=%d", row.CIFail, row.CIUnknown)
		}
		if !strings.Contains(row.Note, "could not be interpreted") {
			t.Errorf("the unknown must still be stated — the red does not make it green; got %q", row.Note)
		}
	})

	t.Run("silent — an undecodable entry with NO failure is still CI-UNKNOWN", func(t *testing.T) {
		row := board(t, `[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"go-test"},`+
			`{"__typename":"SomethingNew","weird":true}]`)
		if row.Action != actCIUnknown {
			t.Fatalf("action = %s, want %s — the CI-RED lane must not swallow the unknown lane", row.Action, actCIUnknown)
		}
	})

	t.Run("silent — an ordinary red board is unchanged", func(t *testing.T) {
		row := board(t, `[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"FAILURE","name":"go-test"}]`)
		if row.CIUnknown != 0 {
			t.Fatalf("no unknowns here; got %d", row.CIUnknown)
		}
		if strings.Contains(row.Note, "could not be interpreted") {
			t.Errorf("no unknown-entry text may appear when every entry decoded; got %q", row.Note)
		}
	})
}

// ---------------------------------------------------------------------------
// #400 round 3 — the three remaining absence instances (N7, N8, N9)
// ---------------------------------------------------------------------------

// TestDecay_AgeUnknownIsItsOwnState_400N7 — the decay alarm was a two-state read on a
// three-state world. A MERGE-NOW row whose approving review carried an unparseable (or
// absent) `submitted_at` has a zero `approvedAt`, so it fell OUT of the comparison
// entirely: no DECAY line, not in `mergeNowDecayPRs`, and a BLANK approved-age column —
// rendering identically to "approved seconds ago". Same shape as the createdAt lane on
// the UNREVIEWED alarm, on the alarm that gates merges.
func TestDecay_AgeUnknownIsItsOwnState_400N7(t *testing.T) {
	const repo = "example-org/tracker"
	const head = "deadbeefcafe"

	run400 := func(t *testing.T, submittedAt string, args ...string) (actionsReport, string) {
		t.Helper()
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":42,"title":"t","isDraft":false,"author":{"login":"shared-agent"},"createdAt":"`+
				time.Now().UTC().Format(time.RFC3339)+`","headRefOid":"`+head+
				`","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
			`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+
				`","body":"ok","submitted_at":"`+submittedAt+`"}]`)
		var out, errb bytes.Buffer
		argv := append([]string{"actions", "--merge-now-threshold", "10m"}, args...)
		if code := run(argv, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(%v) = exit %d, stderr=%s", argv, code, errb.String())
		}
		var rep actionsReport
		if len(args) == 0 {
			if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
				t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
			}
		}
		return rep, out.String()
	}

	for _, unreadable := range []string{"", "not-a-timestamp", "2026-13-45T99:99:99Z"} {
		t.Run("unreadable="+unreadable, func(t *testing.T) {
			rep, _ := run400(t, unreadable)
			r := findRow(t, rep, 42)
			if r.Action != actMergeNow {
				t.Fatalf("action = %s, want %s (the VERDICT is readable; only the age is not)", r.Action, actMergeNow)
			}
			if rep.Header.MergeNowAgeUnknownCount != 1 ||
				len(rep.Header.MergeNowAgeUnknownPRs) != 1 || rep.Header.MergeNowAgeUnknownPRs[0] != 42 {
				t.Errorf("an unreadable approval age must get its OWN lane; got count=%d prs=%v",
					rep.Header.MergeNowAgeUnknownCount, rep.Header.MergeNowAgeUnknownPRs)
			}
			// It must NOT be folded into the measured list — that would claim a
			// measurement, which is the opposite lie.
			for _, n := range rep.Header.MergeNowDecayPRs {
				if n == 42 {
					t.Error("an unmeasured row was folded into mergeNowDecayPRs, which means 'measured past the threshold'")
				}
			}
			if r.ApprovedAge != "" {
				t.Errorf("approvedAge = %q — the board must not print an age it could not read", r.ApprovedAge)
			}

			_, table := run400(t, unreadable, "--table")
			if !strings.Contains(table, "DECAY-AGE-UNKNOWN:") {
				t.Errorf("--table must NAME the unmeasurable row; got:\n%s", table)
			}
		})
	}

	// Positive controls: neither readable state leaks into the new lane.
	t.Run("readable and fresh", func(t *testing.T) {
		rep, _ := run400(t, time.Now().UTC().Add(-1*time.Minute).Format(time.RFC3339))
		if rep.Header.MergeNowAgeUnknownCount != 0 {
			t.Errorf("a readable age must not enter the unknown lane; got %d", rep.Header.MergeNowAgeUnknownCount)
		}
		if rep.Header.MergeNowDecay {
			t.Error("a 1m-old approval is not decayed against a 10m threshold")
		}
	})
	t.Run("readable and decayed", func(t *testing.T) {
		rep, table := run400(t, time.Now().UTC().Add(-30*time.Minute).Format(time.RFC3339))
		if rep.Header.MergeNowAgeUnknownCount != 0 {
			t.Errorf("a readable age must not enter the unknown lane; got %d", rep.Header.MergeNowAgeUnknownCount)
		}
		if !rep.Header.MergeNowDecay {
			t.Error("a 30m-old approval IS decayed against a 10m threshold — the measured lane must still fire")
		}
		_ = table
	})
}

// TestCIState_CompletedWithNoConclusion_400N8 — a COMPLETED check run carrying no
// `conclusion` used to be counted a FAILURE. That failed closed, but the row then said
// "a check FAILED" about a conclusion nobody could read: the absence-as-verdict defect
// with the sign flipped. It is an UNREADABLE entry, and CI-UNKNOWN preempts every
// approve+green path exactly as a fail does, so nothing gets looser.
func TestCIState_CompletedWithNoConclusion_400N8(t *testing.T) {
	var p prBase
	payload := `{"number":1,"headRefOid":"abc","mergeStateStatus":"CLEAN",` +
		`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","name":"ci"}]}`
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	pass, pending, fail, unknown := ciState(p)
	if fail != 0 {
		t.Errorf("fail = %d — a conclusion nobody could read is not a failure the board may report", fail)
	}
	if unknown != 1 {
		t.Errorf("unknown = %d, want 1 (COMPLETED with no conclusion is UNREADABLE)", unknown)
	}
	if pass != 0 || pending != 0 {
		t.Errorf("pass=%d pending=%d, want 0/0", pass, pending)
	}

	// Fail-closed is preserved end to end: approved at head still does not reach green.
	in := buildClassifyInput(p, reviewState{ever: true, atHead: true, approved: true}, true, "")
	action, note := classify(in)
	if action != actCIUnknown {
		t.Errorf("action = %s, want %s; note: %s", action, actCIUnknown, note)
	}
	if strings.Contains(note, "FAILED") {
		t.Errorf("the note reports a FAILURE over an unread conclusion: %s", note)
	}
}

// TestNote_NoCIConfiguredIsNotAGreenVerdict_400N9 — on a repo the policy marks as running
// no PR CI, an empty rollup makes ciGreen VACUOUSLY true. The action is policy-backed and
// stays MERGE-NOW; the NOTE said "CI green", asserting a verdict where nothing ran.
func TestNote_NoCIConfiguredIsNotAGreenVerdict_400N9(t *testing.T) {
	var p prBase
	if err := json.Unmarshal([]byte(`{"number":1,"title":"t","isDraft":false,"headRefOid":"abc",`+
		`"mergeStateStatus":"CLEAN","statusCheckRollup":[]}`), &p); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	rs := reviewState{ever: true, atHead: true, approved: true}

	in := buildClassifyInput(p, rs, false, "") // ciRequired = false
	if !in.ciGreen {
		t.Fatal("precondition: an empty rollup on a CI-less repo is vacuously green")
	}
	action, note := classify(in)
	if action != actMergeNow {
		t.Fatalf("action = %s, want %s — the policy call is unchanged", action, actMergeNow)
	}
	if strings.Contains(note, "CI green") {
		t.Errorf("the note asserts a CI verdict on a repo where no check ran:\n%s", note)
	}
	if !strings.Contains(note, "no PR CI configured") {
		t.Errorf("the note must SAY why there is no verdict, not merely omit one:\n%s", note)
	}

	// Positive control: a real passing check still earns the words.
	var q prBase
	if err := json.Unmarshal([]byte(`{"number":1,"title":"t","isDraft":false,"headRefOid":"abc",`+
		`"mergeStateStatus":"CLEAN","statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`), &q); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if _, note := classify(buildClassifyInput(q, rs, true, "")); !strings.Contains(note, "CI green") {
		t.Errorf("a genuinely green head must still be called green:\n%s", note)
	}
}

// ---------------------------------------------------------------------------
// #400 T1 / T2 — the two the third re-review required
// ---------------------------------------------------------------------------

// TestCIState_ExpectedIsNotAPass_400T1 — GitHub's StatusState `EXPECTED` ("Status is
// expected") was mapped into `pass` alongside `SUCCESS` ("Status is successful"). That is
// R2/Q2 verbatim on the legacy-status axis: `EXPECTED` is what a required status context
// that has NOT REPORTED looks like, and counting it green let one unreported context carry
// an approved, non-draft, CLEAN PR to MERGE-NOW with "CI green" in the note — measured, at
// the head this test was written against.
//
// It is `pending`: a check still owed, which is exactly what pending means here. The
// assertion runs through the real builder and the real classifier, and reads the ACTION and
// the NOTE — never `in.ciGreen`, the boolean a mutant flips.
func TestCIState_ExpectedIsNotAPass_400T1(t *testing.T) {
	mk := func(state string) prBase {
		var p prBase
		payload := `{"number":1,"title":"t","isDraft":false,"headRefOid":"abc","mergeStateStatus":"CLEAN",` +
			`"statusCheckRollup":[{"__typename":"StatusContext","state":"` + state + `","context":"legacy/ci"}]}`
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			t.Fatalf("fixture %s: %v", state, err)
		}
		return p
	}
	approvedAtHead := reviewState{ever: true, atHead: true, approved: true}

	t.Run("EXPECTED is a check still owed, not a pass", func(t *testing.T) {
		pass, pending, fail, unknown := ciState(mk("EXPECTED"))
		if pass != 0 {
			t.Errorf("pass = %d — `EXPECTED` means the status is EXPECTED, i.e. declared and not "+
				"yet reported; a context that reported nothing is not a passing context", pass)
		}
		if pending != 1 || fail != 0 || unknown != 0 {
			t.Errorf("pending=%d fail=%d unknown=%d, want 1/0/0", pending, fail, unknown)
		}
	})

	t.Run("and it cannot reach MERGE-NOW or a CI-green note", func(t *testing.T) {
		in := buildClassifyInput(mk("EXPECTED"), approvedAtHead, true, "")
		action, note := classify(in)
		if action == actMergeNow || action == actFlip {
			t.Errorf("action = %s off one UNREPORTED status context — this is #400 T1 verbatim\nnote: %s", action, note)
		}
		if strings.Contains(note, "CI green") {
			t.Errorf("the note asserts CI green over a context that never reported:\n%s", note)
		}
	})

	// Positive control, so "EXPECTED is not green" is not satisfiable by a reducer that
	// calls everything pending.
	t.Run("positive control: SUCCESS still passes and still merges", func(t *testing.T) {
		if pass, _, _, _ := ciState(mk("SUCCESS")); pass != 1 {
			t.Fatalf("pass = %d for a SUCCESS context, want 1", pass)
		}
		if action, note := classify(buildClassifyInput(mk("SUCCESS"), approvedAtHead, true, "")); action != actMergeNow {
			t.Errorf("a genuinely successful legacy status must still reach MERGE-NOW; got %s (%s)", action, note)
		}
	})
}

// TestPRPopulation_TruncationIsInBand_400T2 — the open-PR population is what every count
// on this board rests on, and a read at the `--limit` cap may be missing rows. Truncation
// used to be a stderr WARNING and nothing else: the machine consumer reads the JSON, where
// a truncated population was indistinguishable from a complete one, so `mergeNowCount`,
// `unreviewedCount` and the row set itself were confident numbers over an unknown
// remainder. Both halves are asserted — fires at the cap, SILENT below it.
func TestPRPopulation_TruncationIsInBand_400T2(t *testing.T) {
	const cappedRepo = "example-org/tracker"
	prList := func(n int) string {
		parts := make([]string, 0, n)
		for i := 1; i <= n; i++ {
			parts = append(parts, `{"number":`+itoa(i)+`,"title":"t","body":"","isDraft":true,`+
				`"author":{"login":"shared-agent"},"createdAt":"`+time.Now().UTC().Format(time.RFC3339)+
				`","labels":[],"headRefOid":"abc123","headRefName":"b","mergeStateStatus":"CLEAN",`+
				`"statusCheckRollup":[]}`)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	install := func(t *testing.T, n int) {
		t.Helper()
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", cappedRepo)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON", prList(n))
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
	}

	for _, verb := range []string{"prs", "actions"} {
		t.Run(verb+" — fires at the cap, in the JSON a machine reads", func(t *testing.T) {
			install(t, prListLimit)
			var out, errb bytes.Buffer
			if code := run([]string{verb}, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("run(%s) = exit %d: %s", verb, code, errb.String())
			}
			var m map[string]any
			if err := json.Unmarshal(out.Bytes(), &m); err != nil {
				t.Fatalf("parsing %s JSON: %v", verb, err)
			}
			pop, ok := m["prPopulation"].(map[string]any)
			if !ok {
				t.Fatalf("%s read a PR list and its header carries NO prPopulation — the population every "+
					"count rests on is unstated (#400 T2); header keys: %v", verb, keysOf(m))
			}
			if pop["complete"] != false {
				t.Errorf("%s: prPopulation.complete = %v over a read AT the %d cap — at the boundary a "+
					"complete population and a truncated one look identical", verb, pop["complete"], prListLimit)
			}
			repos, _ := pop["truncatedRepos"].([]any)
			if len(repos) != 1 || repos[0] != cappedRepo {
				t.Errorf("%s: truncatedRepos = %v, want [%s] — the reader is owed WHICH repo may be short",
					verb, pop["truncatedRepos"], cappedRepo)
			}
		})

		t.Run(verb+" --table says it too", func(t *testing.T) {
			install(t, prListLimit)
			var out, errb bytes.Buffer
			if code := run([]string{verb, "--table"}, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("run(%s --table) = exit %d: %s", verb, code, errb.String())
			}
			if !strings.Contains(out.String(), "POPULATION TRUNCATED") {
				t.Errorf("%s --table: the human surface must state the truncation too:\n%s", verb, out.String())
			}
		})

		t.Run(verb+" — silent below the cap", func(t *testing.T) {
			install(t, 2)
			var out, errb bytes.Buffer
			if code := run([]string{verb}, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("run(%s) = exit %d: %s", verb, code, errb.String())
			}
			var m map[string]any
			if err := json.Unmarshal(out.Bytes(), &m); err != nil {
				t.Fatal(err)
			}
			pop, ok := m["prPopulation"].(map[string]any)
			if !ok {
				t.Fatalf("%s: prPopulation must be present and say complete=true — absent means "+
					"'this verb read no PR list', which would be false here", verb)
			}
			if pop["complete"] != true {
				t.Errorf("%s: a read BELOW the cap is a measured-complete population; got %v", verb, pop["complete"])
			}
			if _, present := pop["truncatedRepos"]; present {
				t.Errorf("%s: truncatedRepos present on a complete read: %v", verb, pop["truncatedRepos"])
			}
			var tout, terrb bytes.Buffer
			if code := run([]string{verb, "--table"}, &tout, &terrb); code != deskkit.ExitOK {
				t.Fatalf("run(%s --table) = exit %d: %s", verb, code, terrb.String())
			}
			if strings.Contains(tout.String(), "POPULATION TRUNCATED") {
				t.Errorf("%s --table warns on a complete read — an alarm that fires when nothing is "+
					"wrong is an alarm that gets trained away:\n%s", verb, tout.String())
			}
		})
	}

	// #400 U2: `reviews` reads the open-PR list too (to find `head`), and the standing
	// review found the inventory above never exercised it — deleting its PRPopulation
	// assignment left the suite green. Pinned directly, same shape as the loop above.
	t.Run("reviews — fires at the cap, in the JSON a machine reads", func(t *testing.T) {
		install(t, prListLimit)
		var out, errb bytes.Buffer
		if code := run([]string{"reviews", cappedRepo, "1"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(reviews) = exit %d: %s", code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatalf("parsing reviews JSON: %v", err)
		}
		pop, ok := m["prPopulation"].(map[string]any)
		if !ok {
			t.Fatalf("reviews read a PR list and its header carries NO prPopulation — the population every "+
				"count rests on is unstated (#400 U2); header keys: %v", keysOf(m))
		}
		if pop["complete"] != false {
			t.Errorf("reviews: prPopulation.complete = %v over a read AT the %d cap", pop["complete"], prListLimit)
		}
		repos, _ := pop["truncatedRepos"].([]any)
		if len(repos) != 1 || repos[0] != cappedRepo {
			t.Errorf("reviews: truncatedRepos = %v, want [%s]", pop["truncatedRepos"], cappedRepo)
		}
	})

	t.Run("reviews --table says it too", func(t *testing.T) {
		install(t, prListLimit)
		var out, errb bytes.Buffer
		if code := run([]string{"reviews", cappedRepo, "1", "--table"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(reviews --table) = exit %d: %s", code, errb.String())
		}
		if !strings.Contains(out.String(), "POPULATION TRUNCATED") {
			t.Errorf("reviews --table: the human surface must state the truncation too:\n%s", out.String())
		}
	})

	t.Run("reviews — silent below the cap", func(t *testing.T) {
		install(t, 2)
		var out, errb bytes.Buffer
		if code := run([]string{"reviews", cappedRepo, "1"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(reviews) = exit %d: %s", code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		pop, ok := m["prPopulation"].(map[string]any)
		if !ok {
			t.Fatalf("reviews: prPopulation must be present and say complete=true — absent means " +
				"'this verb read no PR list', which would be false here")
		}
		if pop["complete"] != true {
			t.Errorf("reviews: a read BELOW the cap is a measured-complete population; got %v", pop["complete"])
		}
		if _, present := pop["truncatedRepos"]; present {
			t.Errorf("reviews: truncatedRepos present on a complete read: %v", pop["truncatedRepos"])
		}
	})

	// #400 U1: `files <repo> <n> <path>` resolves its head via the same open-PR-list read,
	// on the path branch only — `files <repo> <n>` with no path reads a per-PR files
	// endpoint instead and must stay silent (asserted separately below).
	t.Run("files <path> — fires at the cap, in the JSON a machine reads", func(t *testing.T) {
		install(t, prListLimit)
		var out, errb bytes.Buffer
		if code := run([]string{"files", cappedRepo, "1", "README.md"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(files <path>) = exit %d: %s", code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatalf("parsing files JSON: %v", err)
		}
		pop, ok := m["prPopulation"].(map[string]any)
		if !ok {
			t.Fatalf("files <path> resolved its head via the open-PR list and carries NO prPopulation "+
				"(#400 U1); header keys: %v", keysOf(m))
		}
		if pop["complete"] != false {
			t.Errorf("files <path>: prPopulation.complete = %v over a read AT the %d cap", pop["complete"], prListLimit)
		}
	})

	t.Run("files <path> — silent below the cap", func(t *testing.T) {
		install(t, 2)
		var out, errb bytes.Buffer
		if code := run([]string{"files", cappedRepo, "1", "README.md"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(files <path>) = exit %d: %s", code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		pop, ok := m["prPopulation"].(map[string]any)
		if !ok || pop["complete"] != true {
			t.Errorf("files <path>: a read BELOW the cap is a measured-complete population; got %v", pop)
		}
	})

	t.Run("files (no path) omits the field entirely", func(t *testing.T) {
		install(t, prListLimit)
		var out, errb bytes.Buffer
		if code := run([]string{"files", cappedRepo, "1"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(files) = exit %d: %s", code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if _, present := m["prPopulation"]; present {
			t.Errorf("files with no path reads a per-PR files endpoint, not the open-PR list — a "+
				"population claim there would be about a read it never made; got %v", m["prPopulation"])
		}
	})

	// The verbs that read NO PR list must OMIT the field. Absent is the third state:
	// "this verb read no population", never "the population was complete".
	t.Run("queue omits the field entirely", func(t *testing.T) {
		install(t, prListLimit)
		var out, errb bytes.Buffer
		if code := run([]string{"queue"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(queue) = exit %d: %s", code, errb.String())
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if _, present := m["prPopulation"]; present {
			t.Errorf("queue reads issues, not a PR list — a population claim there would be a coverage "+
				"statement about a read it never made; got %v", m["prPopulation"])
		}
	})

	// And the fetcher itself, directly: the flag is a MEASURED property of the read.
	t.Run("fetchOpenPRs reports the cap to its caller", func(t *testing.T) {
		for _, c := range []struct {
			n    int
			want bool
		}{{prListLimit, true}, {prListLimit - 1, false}, {0, false}} {
			stubGHFunc(t, func(args ...string) ([]byte, error) {
				return []byte(prList(c.n)), nil
			})
			prs, truncated, err := fetchOpenPRs("o/r")
			if err != nil {
				t.Fatalf("n=%d: %v", c.n, err)
			}
			if len(prs) != c.n {
				t.Fatalf("n=%d: got %d PRs", c.n, len(prs))
			}
			if truncated != c.want {
				t.Errorf("n=%d (cap %d): truncated = %v, want %v", c.n, prListLimit, truncated, c.want)
			}
		}
	})

	// headOfPR's absence answer is only a statement about the world when the list was
	// COMPLETE. Over a capped read, "not an open PR" is a statement about the page.
	t.Run("headOfPR distinguishes absent-from-a-complete-list from absent-from-a-capped-one", func(t *testing.T) {
		stubGHFunc(t, func(args ...string) ([]byte, error) { return []byte(prList(prListLimit)), nil })
		_, truncated, err := headOfPR("o/r", 999999)
		if err == nil {
			t.Fatal("a PR missing from a capped list must not resolve")
		}
		if !truncated {
			t.Errorf("a read AT the cap must report truncated=true even on the not-found path")
		}
		if !strings.Contains(err.Error(), "TRUNCATED") {
			t.Errorf("over a capped read the error must say the list may be short, not assert the PR is "+
				"not open: %v", err)
		}
		stubGHFunc(t, func(args ...string) ([]byte, error) { return []byte(prList(2)), nil })
		_, truncated2, err := headOfPR("o/r", 999999)
		if err == nil || !strings.Contains(err.Error(), "is not an open PR") {
			t.Errorf("over a COMPLETE read the definite answer must survive: %v", err)
		}
		if truncated2 {
			t.Errorf("a read below the cap must report truncated=false")
		}
	})
}

// ---------------------------------------------------------------------------
// #400 — the silent survivals the third re-review listed as non-blocking
// ---------------------------------------------------------------------------

// TestReviews_AtHeadIsTheSharedComparison_400 — the `reviews` verb hand-rolled the
// at-head test instead of calling sameHead. It happened to be equivalent; the point of Q3
// is that this comparison lives in ONE place, so a later edit cannot re-introduce
// absence-equals-absence on one site while the other two stay right. Nothing exercised
// this verb's atHead column at all, so the hand-rolled copy could be mutated freely.
func TestReviews_AtHeadIsTheSharedComparison_400(t *testing.T) {
	const repo = "example-org/tracker"
	run1 := func(t *testing.T, headSHA, reviewSHA string) reviewsReport {
		t.Helper()
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON", `[{"number":7,"title":"t","body":"","isDraft":true,`+
			`"author":{"login":"shared-agent"},"createdAt":"`+time.Now().UTC().Format(time.RFC3339)+
			`","labels":[],"headRefOid":"`+headSHA+`","headRefName":"b","mergeStateStatus":"CLEAN",`+
			`"statusCheckRollup":[]}]`)
		t.Setenv("DESKBOARD_GH_REVIEWS_JSON", approvedReview(reviewSHA, "looks fine"))
		var out, errb bytes.Buffer
		if code := run([]string{"reviews", repo, "7"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(reviews) = exit %d: %s", code, errb.String())
		}
		var rep reviewsReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("parsing reviews JSON: %v\n%s", err, out.String())
		}
		if len(rep.Reviews) != 1 {
			t.Fatalf("want exactly one review row, got %d", len(rep.Reviews))
		}
		return rep
	}

	// The defect shape: two ABSENT shas are not a match.
	if got := run1(t, "", "").Reviews[0].AtHead; got {
		t.Error("atHead = true with BOTH the PR head and the review's commit_id absent — two " +
			"could-not-checks compared equal (#400 Q3)")
	}
	if got := run1(t, "abc123", "").Reviews[0].AtHead; got {
		t.Error("atHead = true off a review carrying no commit_id")
	}
	if got := run1(t, "", "abc123").Reviews[0].AtHead; got {
		t.Error("atHead = true off a PR whose headRefOid could not be read")
	}
	// Positive control.
	if got := run1(t, "abc123", "abc123").Reviews[0].AtHead; !got {
		t.Error("positive control: a review AT a read head must report atHead")
	}
}

// TestOwnFilesChanged_TruncatedReadForcesReReview_400 — MERGE-CURR's note says "PR's own
// files unchanged since last review". A truncated own-files read cannot support that
// sentence: measured on tracker#1618, `--json files` returned an alphabetical 100-entry window
// of 652 changed files, so an empty intersection meant nothing. While the rule was one
// inline expression in cmdActions' loop, dropping the `!complete` term left the suite
// green.
func TestOwnFilesChanged_TruncatedReadForcesReReview_400(t *testing.T) {
	own := map[string]bool{"docs/a.md": true}
	changedElsewhere := map[string]bool{"other/b.go": true}

	if !ownFilesChanged(own, false, changedElsewhere) {
		t.Error("an INCOMPLETE own-files read reported 'unchanged' — the intersection was computed " +
			"over a partial list, so it proves nothing; the safe side is RE-REVIEW")
	}
	if ownFilesChanged(own, true, changedElsewhere) {
		t.Error("a COMPLETE read with a disjoint change set must stay MERGE-CURR — a brake that " +
			"never releases is not a brake")
	}
	if !ownFilesChanged(own, true, map[string]bool{"docs/a.md": true}) {
		t.Error("a complete read whose sets intersect must force RE-REVIEW")
	}
	// An unread `changed` set (nil) is unknown, and unknown forces the safe side.
	if !ownFilesChanged(own, true, nil) {
		t.Error("a nil changed-set is a could-not-check and must force RE-REVIEW")
	}

	// And the consequence the note makes: RE-REVIEW, never the confident MERGE-CURR line.
	in := classifyInput{ever: true, atHead: false, ownFilesChanged: ownFilesChanged(own, false, changedElsewhere)}
	if action, note := classify(in); action != actReReview {
		t.Errorf("action = %s over a truncated own-files read, want %s\nnote: %s", action, actReReview, note)
	}
}

// TestFetchOpenPRs_ParseFailureFailsLoud_400 — a fetcher never returns a silent empty
// result on error. An unparseable PR list rendering as an EMPTY BOARD is the worst possible
// output of this tool: no rows, no error, and a desk concluding there is nothing to do.
func TestFetchOpenPRs_ParseFailureFailsLoud_400(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) { return []byte(`{"not":"an array"}`), nil })
	prs, truncated, err := fetchOpenPRs("o/r")
	if err == nil {
		t.Fatalf("an unparseable PR list returned no error (%d PRs, truncated=%v) — the board would "+
			"render EMPTY and read as 'nothing open'", len(prs), truncated)
	}
	if prs != nil {
		t.Errorf("a failing fetcher must return no rows, got %d", len(prs))
	}
	if !strings.Contains(err.Error(), "cannot parse PR list") {
		t.Errorf("the error must name what could not be read: %v", err)
	}
	// The read failure half of the same contract.
	stubGHFunc(t, func(args ...string) ([]byte, error) { return nil, fmt.Errorf("gh: 401") })
	if _, _, err := fetchOpenPRs("o/r"); err == nil {
		t.Error("a gh failure must fail the run, never yield an empty board")
	}
}

// TestHumanGate_LabelChannelIsRead_241 — the declaration has three channels (label, title,
// body) and only two were exercised anywhere. A PR declaring the gate ONLY by label
// classified MERGE-NOW under a mutant that dropped the label argument, silently: the board
// telling a desk to merge work whose merge is reserved to a human.
func TestHumanGate_LabelChannelIsRead_241(t *testing.T) {
	mk := func(labelsJSON string) prBase {
		var p prBase
		if err := json.Unmarshal([]byte(`{"number":1,"title":"an ordinary title","body":"nothing here",`+
			`"isDraft":false,"headRefOid":"abc","mergeStateStatus":"CLEAN","labels":`+labelsJSON+`,`+
			`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`), &p); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		return p
	}
	rs := reviewState{ever: true, atHead: true, approved: true}

	for _, label := range []string{"human-gate", "Human Gate", "gate: human"} {
		b, _ := json.Marshal(label)
		in := buildClassifyInput(mk(`[{"name":`+string(b)+`}]`), rs, true, "")
		action, note := classify(in)
		if action != actHumanGate {
			t.Errorf("label %q declared the gate and the board said %s — the label channel is read by "+
				"humanGateDeclared and by nothing else\nnote: %s", label, action, note)
		}
		if !strings.Contains(note, "label") {
			t.Errorf("label %q: the note must name the channel that stopped the desk: %s", label, note)
		}
	}
	// Silent on an ordinary PR: an unrelated label declares nothing.
	if action, _ := classify(buildClassifyInput(mk(`[{"name":"documentation"}]`), rs, true, "")); action != actMergeNow {
		t.Errorf("an ordinary labelled PR classified %s — a gate that fires on everything gates nothing", action)
	}
}

// TestReduceReviews_CommentedIsNotDecisive_400 — `ever` means "the App returned a VERDICT
// at some head". A COMMENTED review is not a verdict, and admitting it turns "never
// reviewed" into READY/inspect — the NEEDS-REVIEW row, and the #359 never-reviewed alarm
// built on it, both disappear for a PR the reviewer only commented on.
func TestReduceReviews_CommentedIsNotDecisive_400(t *testing.T) {
	mk := func(state string) review {
		var r review
		r.User.Login = reviewerBotDisplay()
		r.State = state
		r.CommitID = "abc123"
		r.SubmittedAt = time.Now().UTC().Format(time.RFC3339)
		return r
	}
	for _, state := range []string{"COMMENTED", "DISMISSED", "PENDING"} {
		st := reduceReviews([]review{mk(state)}, "abc123")
		if st.ever {
			t.Errorf("a %s review counted as a verdict — `ever` is what NEEDS-REVIEW and the "+
				"never-reviewed alarm are computed from", state)
		}
		if action, _ := classify(classifyInput{ever: st.ever, atHead: st.atHead,
			blocking: st.blocking, approvedAtHead: st.approved, pass: 1, ciGreen: true}); action != actNeedsReview {
			t.Errorf("a PR with only a %s review classified %s, want %s", state, action, actNeedsReview)
		}
	}
	// Positive control, both decisive states.
	for _, state := range []string{"APPROVED", "CHANGES_REQUESTED"} {
		if st := reduceReviews([]review{mk(state)}, "abc123"); !st.ever {
			t.Errorf("%s is a verdict and must count", state)
		}
	}
}

// TestOpenAge_UnreadableIsAbsent_400 — an age the board could not read is reported ABSENT,
// never as a duration. Fabricating one from the zero time renders "just opened", which is
// the fail-open answer for every age-based alarm on this board.
func TestOpenAge_UnreadableIsAbsent_400(t *testing.T) {
	now := time.Now()
	for _, raw := range []string{"", "not-a-timestamp", "2026-13-45T99:99:99Z"} {
		if got := openAgeOf(prBase{CreatedAt: raw}, now); got != "" {
			t.Errorf("openAgeOf(%q) = %q — an unreadable createdAt must be an ABSENT age, not a "+
				"fabricated one", raw, got)
		}
	}
	if got := openAgeOf(prBase{CreatedAt: now.Add(-48 * time.Hour).UTC().Format(time.RFC3339)}, now); got == "" {
		t.Error("positive control: a readable createdAt must yield an age")
	}
}

// TestMergeStateDisplay_AbsentIsNamed_400 — the could-not-check note names what came back.
// An empty field printed as `""` reads like a value GitHub returned; ABSENT says nobody
// read one, which is the whole distinction R3 exists for.
func TestMergeStateDisplay_AbsentIsNamed_400(t *testing.T) {
	for _, raw := range []string{"", "   "} {
		got := mergeStateDisplay(raw)
		if !strings.Contains(got, "ABSENT") {
			t.Errorf("mergeStateDisplay(%q) = %q — an absent field must be NAMED absent", raw, got)
		}
	}
	if got := mergeStateDisplay("UNKNOWN"); got != `"UNKNOWN"` {
		t.Errorf(`mergeStateDisplay("UNKNOWN") = %s, want the verbatim value quoted`, got)
	}
	// And it reaches the note, so the rendering is not merely a helper nobody consults.
	var p prBase
	if err := json.Unmarshal([]byte(`{"number":1,"title":"t","isDraft":false,"headRefOid":"abc",`+
		`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}`), &p); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	action, note := classify(buildClassifyInput(p, reviewState{ever: true, atHead: true, approved: true}, true, ""))
	if action != actMergeStateUnknown {
		t.Fatalf("action = %s, want %s", action, actMergeStateUnknown)
	}
	if !strings.Contains(note, "ABSENT (no mergeStateStatus in the payload)") {
		t.Errorf("the could-not-check note must name the absence:\n%s", note)
	}
}
