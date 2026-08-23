package main

// health_test.go — tests for the default-branch health probe (#295).
//
// The rule is that a clean report from an
// instrument is worthless until the instrument has been proven to go RED on a broken
// input — the positive control. TestHealth_PositiveControl_RedDefaultBranch is that
// control, and it is driven through the REAL binary path (run(), the real exec, a gh
// shim in PATH) using the ACTUAL check-runs payload GitHub returned for the
// example-org/example-k8s breakage (a `validate` check failing on main)
// that #295 was filed against.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// stubGH swaps the package-level gh runner for a request-matching fake and restores it
// afterwards. Used for the state-machine tests, where per-sha fixtures are needed;
// the read-only proof and the end-to-end demos keep using the PATH shim.
func stubGHFunc(t *testing.T, fn func(args ...string) ([]byte, error)) {
	t.Helper()
	prev := ghRun
	ghRun = fn
	t.Cleanup(func() { ghRun = prev })
}

// ---------------------------------------------------------------------------
// pure classifier
// ---------------------------------------------------------------------------

func TestClassifyCheckRuns(t *testing.T) {
	cases := []struct {
		name        string
		runs        []checkRun
		wantState   string
		wantFailing int
		wantCounted bool
	}{
		{
			name:      "all green",
			runs:      []checkRun{{Name: "a", Status: "completed", Conclusion: "success"}},
			wantState: bhGreen, wantCounted: true,
		},
		{
			name: "one failure is RED even beside successes",
			runs: []checkRun{
				{Name: "a", Status: "completed", Conclusion: "success"},
				{Name: "validate", Status: "completed", Conclusion: "failure"},
			},
			wantState: bhRed, wantFailing: 1, wantCounted: true,
		},
		{
			name:      "timed_out counts as RED",
			runs:      []checkRun{{Name: "slow", Status: "completed", Conclusion: "timed_out"}},
			wantState: bhRed, wantFailing: 1, wantCounted: true,
		},
		{
			// Noise damper: on a default branch a cancelled run is nearly always a
			// concurrency-group supersede by the next push, not a breakage.
			name: "cancelled is NOT red",
			runs: []checkRun{
				{Name: "a", Status: "completed", Conclusion: "success"},
				{Name: "b", Status: "completed", Conclusion: "cancelled"},
			},
			wantState: bhGreen, wantCounted: true,
		},
		{
			name: "in-progress is pending, not green and not red",
			runs: []checkRun{
				{Name: "a", Status: "completed", Conclusion: "success"},
				{Name: "b", Status: "in_progress"},
			},
			wantState: bhPending, wantCounted: true,
		},
		{
			// counted==0 is "no evidence here", which is what makes the lookback walk
			// back rather than declare victory.
			name: "all skipped yields NO verdict (not green)",
			runs: []checkRun{
				{Name: "a", Status: "completed", Conclusion: "skipped"},
				{Name: "b", Status: "completed", Conclusion: "neutral"},
			},
			wantState: "", wantCounted: false,
		},
		{
			name: "empty rollup yields NO verdict (not green)",
			runs: nil, wantState: "", wantCounted: false,
		},
		{
			// REST returns lowercase where the gh PR rollup returns uppercase.
			name:      "uppercase conclusions are handled too",
			runs:      []checkRun{{Name: "a", Status: "COMPLETED", Conclusion: "FAILURE"}},
			wantState: bhRed, wantFailing: 1, wantCounted: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, failing, counted := classifyCheckRuns(c.runs)
			if state != c.wantState {
				t.Errorf("state = %q, want %q", state, c.wantState)
			}
			if len(failing) != c.wantFailing {
				t.Errorf("failing = %v, want %d entries", failing, c.wantFailing)
			}
			if (counted > 0) != c.wantCounted {
				t.Errorf("counted = %d, want counted=%v", counted, c.wantCounted)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// per-repo probe: the three states, kept distinct
// ---------------------------------------------------------------------------

const ciRepo = "example-org/tracker"      // deskkit.CIRequired == true
const noCIRepo = "example-org/org-slides" // deskkit.CIRequired == false

func TestAssessRepoBranch_Red(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		req := strings.Join(args, " ")
		switch {
		case strings.Contains(req, "/check-runs"):
			return []byte(`{"total_count":1,"check_runs":[{"name":"validate","status":"completed","conclusion":"failure","head_sha":"deadbeef"}]}`), nil
		default:
			return []byte(`[{"sha":"deadbeef"}]`), nil
		}
	})
	row := assessRepoBranch(ciRepo)
	if row.State != bhRed {
		t.Fatalf("state = %q, want %q (reason: %s)", row.State, bhRed, row.Reason)
	}
	if len(row.Failing) != 1 || row.Failing[0] != "validate" {
		t.Errorf("failing = %v, want [validate]", row.Failing)
	}
	if !strings.Contains(row.Reason, "RED") {
		t.Errorf("reason must say RED; got %q", row.Reason)
	}
}

// TestAssessRepoBranch_ReadFailureIsUnknownNotGreen is the defect this issue is an
// instance of: a probe that could not look must never report health.
func TestAssessRepoBranch_ReadFailureIsUnknownNotGreen(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		return nil, errString("gh api: HTTP 502 Bad Gateway")
	})
	row := assessRepoBranch(ciRepo)
	if row.State != bhUnknown {
		t.Fatalf("state = %q, want %q — a failed read must never read as green", row.State, bhUnknown)
	}
	if !strings.Contains(row.Reason, "COULD-NOT-CHECK") {
		t.Errorf("reason must be explicit about not having checked; got %q", row.Reason)
	}
	if !strings.Contains(row.Reason, "502") {
		t.Errorf("reason must carry the underlying failure; got %q", row.Reason)
	}
}

func TestAssessRepoBranch_CheckRunReadFailureIsUnknown(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		req := strings.Join(args, " ")
		if strings.Contains(req, "/check-runs") {
			return nil, errString("gh api: HTTP 403 rate limit exceeded")
		}
		return []byte(`[{"sha":"aaaa1111"}]`), nil
	})
	row := assessRepoBranch(ciRepo)
	if row.State != bhUnknown || !strings.Contains(row.Reason, "COULD-NOT-CHECK") {
		t.Fatalf("state = %q reason = %q, want unknown/COULD-NOT-CHECK", row.State, row.Reason)
	}
}

// TestAssessRepoBranch_TruncationIsUnknown — three-state rule sub-rule 2: a truncated
// list is not evidence. A failing run could be on the page we never read.
func TestAssessRepoBranch_TruncationIsUnknown(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		req := strings.Join(args, " ")
		if strings.Contains(req, "/check-runs") {
			return []byte(`{"total_count":250,"check_runs":[{"name":"a","status":"completed","conclusion":"success"}]}`), nil
		}
		return []byte(`[{"sha":"aaaa1111"}]`), nil
	})
	row := assessRepoBranch(ciRepo)
	if row.State != bhUnknown {
		t.Fatalf("state = %q, want unknown on a truncated check-run list (reason %q)", row.State, row.Reason)
	}
	if !strings.Contains(row.Reason, "truncated") {
		t.Errorf("reason must name truncation; got %q", row.Reason)
	}
}

// TestAssessRepoBranch_LookbackSkipsCheckless — the noise damper. A docs/status commit
// carries no check runs; that is not an alarm, it is a reason to look one commit back.
func TestAssessRepoBranch_LookbackSkipsCheckless(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		req := strings.Join(args, " ")
		switch {
		case strings.Contains(req, "/commits/head0000/check-runs"):
			return []byte(`{"total_count":0,"check_runs":[]}`), nil
		case strings.Contains(req, "/commits/prev1111/check-runs"):
			return []byte(`{"total_count":1,"check_runs":[{"name":"ci","status":"completed","conclusion":"success"}]}`), nil
		default:
			return []byte(`[{"sha":"head0000"},{"sha":"prev1111"}]`), nil
		}
	})
	row := assessRepoBranch(ciRepo)
	if row.State != bhGreen {
		t.Fatalf("state = %q, want green from the lookback (reason %q)", row.State, row.Reason)
	}
	if row.BehindHead != 1 || row.Commit != "prev1111" {
		t.Errorf("assessed commit = %s behind=%d, want prev1111 behind=1", row.Commit, row.BehindHead)
	}
	if !strings.Contains(row.Reason, "behind head") {
		t.Errorf("a verdict taken behind head must SAY it is behind head; got %q", row.Reason)
	}
}

// TestAssessRepoBranch_NoChecksAnywhere — the two ends of the same observation. On a
// repo that runs CI, "no check runs anywhere in the window" is could-not-check. On a
// repo that runs none, it is a stated not-applicable — and neither one is green.
func TestAssessRepoBranch_NoChecksAnywhere(t *testing.T) {
	fixture := func(args ...string) ([]byte, error) {
		req := strings.Join(args, " ")
		if strings.Contains(req, "/check-runs") {
			return []byte(`{"total_count":0,"check_runs":[]}`), nil
		}
		return []byte(`[{"sha":"a1"},{"sha":"a2"},{"sha":"a3"}]`), nil
	}
	stubGHFunc(t, fixture)
	if row := assessRepoBranch(ciRepo); row.State != bhUnknown {
		t.Errorf("CI-running repo with no check runs: state = %q, want %q (reason %q)", row.State, bhUnknown, row.Reason)
	}
	if row := assessRepoBranch(noCIRepo); row.State != bhNoCI {
		t.Errorf("no-CI repo: state = %q, want %q (reason %q)", row.State, bhNoCI, row.Reason)
	} else if !strings.Contains(row.Reason, "not the same as green") {
		t.Errorf("the no-ci reason must refuse to read as green; got %q", row.Reason)
	}
}

func TestAssessRepoBranch_EmptyRepo(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		return nil, errString("gh api repos/x/commits: Git Repository is empty. (HTTP 409)")
	})
	row := assessRepoBranch("example-org/proposals")
	if row.State != bhNoCommits {
		t.Fatalf("state = %q, want %q — an empty repo is a KNOWN answer, not a failed read", row.State, bhNoCommits)
	}
}

// ---------------------------------------------------------------------------
// report shape: scope announcement + noise budget
// ---------------------------------------------------------------------------

// TestBranchHealth_AnnouncesItsScope — the durable half. A block that cannot state what
// it covered can have "0 red" read as "nothing is red", which is exactly the confusion
// #295 and #359 are both instances of.
func TestBranchHealth_AnnouncesScopeAndTally(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		req := strings.Join(args, " ")
		if strings.Contains(req, "/check-runs") {
			return []byte(`{"total_count":1,"check_runs":[{"name":"ci","status":"completed","conclusion":"success"}]}`), nil
		}
		return []byte(`[{"sha":"a1"}]`), nil
	})
	rep := assessBranchHealth()
	want := len(deskkit.AllowedRepos())
	if len(rep.Scope) != want || len(rep.Repos) != want {
		t.Fatalf("scope=%d rows=%d, want %d of each", len(rep.Scope), len(rep.Repos), want)
	}
	if rep.Green+rep.Red+rep.Pending+rep.NotAssessed+rep.Unknown != want {
		t.Errorf("counts do not account for every watched repo: %+v", rep)
	}
	line := rep.summaryLine()
	for _, must := range []string{"green", "RED", "COULD-NOT-CHECK", "scope:"} {
		if !strings.Contains(line, must) {
			t.Errorf("summary line must mention %q; got %q", must, line)
		}
	}
}

// TestBranchHealth_QuietWhenHealthy — requirement 4. A signal that fires constantly is
// trained away. A wholly healthy board prints ONE line and no alarm.
func TestBranchHealth_QuietWhenHealthy(t *testing.T) {
	stubGHFunc(t, func(args ...string) ([]byte, error) {
		req := strings.Join(args, " ")
		if strings.Contains(req, "/check-runs") {
			return []byte(`{"total_count":1,"check_runs":[{"name":"ci","status":"completed","conclusion":"success"}]}`), nil
		}
		return []byte(`[{"sha":"a1"}]`), nil
	})
	var buf bytes.Buffer
	assessBranchHealth().renderAlarms(&buf)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("a healthy board must render exactly one main-health line; got %d:\n%s", len(lines), buf.String())
	}
	if strings.Contains(buf.String(), "MAIN-RED") || strings.Contains(buf.String(), "MAIN-UNKNOWN") {
		t.Errorf("no alarm should fire on a healthy board; got %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// end-to-end through run(): the positive control and its wiring
// ---------------------------------------------------------------------------

// realRedBranchPayload is the ACTUAL GitHub check-runs response for
// example-org/example-k8s — a commit that reddened main and that the desk could merge
// a PR on top of without a signal. Re-fetch with:
//
//	gh api repos/example-org/example-k8s/commits/<commit>/check-runs
const realRedBranchPayload = `{"total_count":1,"check_runs":[{"name":"validate","status":"completed","conclusion":"failure","head_sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}]}`

// TestHealth_PositiveControl_RedDefaultBranch is the mutation test the three-state rule
// requires: the instrument is shown going RED on a genuinely broken branch, through the
// real binary path, before any green report from it is trusted.
func TestHealth_PositiveControl_RedDefaultBranch(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_CR_RED_REPO", "example-org/example-k8s")
	t.Setenv("DESKBOARD_GH_CR_RED_JSON", realRedBranchPayload)
	t.Setenv("DESKBOARD_GH_COMMITS_JSON", `[{"sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}]`)

	// JSON path (what the loops consume).
	var out, errb bytes.Buffer
	if code := run([]string{"health"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(health) = exit %d, stderr=%s", code, errb.String())
	}
	var rep healthReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing health JSON: %v\n%s", err, out.String())
	}
	// One key, the same one `actions` uses (N2): health carries the report on the
	// embedded Header, not under a second name.
	if rep.MainHealth == nil {
		t.Fatalf("health JSON carries no mainHealth:\n%s", out.String())
	}
	bh := rep.MainHealth
	if bh.Red != 1 || len(bh.RedRepos) != 1 || bh.RedRepos[0] != "example-org/example-k8s" {
		t.Fatalf("expected exactly example-k8s RED; got red=%d repos=%v", bh.Red, bh.RedRepos)
	}
	var ledger branchHealthRow
	for _, r := range bh.Repos {
		if r.Repo == "example-org/example-k8s" {
			ledger = r
		}
	}
	if ledger.State != bhRed || len(ledger.Failing) != 1 || ledger.Failing[0] != "validate" {
		t.Errorf("ledger row = %+v, want state=red failing=[validate]", ledger)
	}
	if bh.Repos[0].State != bhRed {
		t.Errorf("the RED repo must sort first; got %q first", bh.Repos[0].State)
	}

	// Table path (what a human reads).
	out.Reset()
	errb.Reset()
	if code := run([]string{"health", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(health --table) = exit %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "MAIN-RED: ledger") {
		t.Fatalf("table output must carry a MAIN-RED line for ledger; got:\n%s", out.String())
	}
	t.Logf("positive control (example-k8s red-branch payload):\n%s", out.String())
}

// TestActions_RedDefaultBranchAnnotatesRowsAndBanner — the wiring that closes #295: the
// board that decides what gets worked must carry the signal, at the top, and each row on
// the affected repo must say what it would be merging into.
func TestActions_RedDefaultBranchAnnotatesRowsAndBanner(t *testing.T) {
	repo := "example-org/example-k8s"
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_CR_RED_REPO", repo)
	t.Setenv("DESKBOARD_GH_CR_RED_JSON", realRedBranchPayload)
	t.Setenv("DESKBOARD_GH_COMMITS_JSON", `[{"sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}]`)
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	// Author is a role App: example-org/example-k8s is `:public`, and #943's
	// public-repo author gate quarantines a shared trusted-login account (e.g.
	// `shared-agent`) there. This test is about red-default-branch annotation,
	// orthogonal to the author gate, so it uses a trusted-public author to keep
	// the PR classified rather than quarantined.
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":9,"title":"a PR onto a red main","isDraft":true,"author":{"login":"app/assay-desk-app"},"headRefOid":"abc123","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"validate"}]}]`)

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if rep.Header.MainHealth == nil {
		t.Fatal("actions header must carry mainHealth")
	}
	if rep.Header.MainHealth.Red != 1 {
		t.Errorf("header mainHealth.red = %d, want 1", rep.Header.MainHealth.Red)
	}
	if len(rep.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rep.Rows))
	}
	if !rep.Rows[0].BaseBranchRed {
		t.Error("a row on a repo with a RED default branch must be flagged baseBranchRed")
	}
	if !strings.Contains(rep.Rows[0].Note, "RED") {
		t.Errorf("row note must state the base branch is RED; got %q", rep.Rows[0].Note)
	}

	// Table path: the MAIN-RED line must precede the row table (a red main outranks a
	// merge onto it — #295).
	out.Reset()
	if code := run([]string{"actions", "--table"}, &out, &bytes.Buffer{}); code != deskkit.ExitOK {
		t.Fatalf("run(actions --table) = exit %d", code)
	}
	body := out.String()
	iRed := strings.Index(body, "MAIN-RED")
	iHdr := strings.Index(body, "REPO ")
	if iRed < 0 || iHdr < 0 || iRed > iHdr {
		t.Fatalf("MAIN-RED must appear ABOVE the action rows; got:\n%s", body)
	}
	t.Logf("actions board with a red base branch:\n%s", body)
}

// TestActions_HealthUnknownDoesNotFailTheBoard — a failed probe degrades one line, not
// the whole board, and is reported as could-not-check rather than green.
func TestActions_HealthUnknownDoesNotFailTheBoard(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_COMMITS_JSON", "not json at all")

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d (a health-probe failure must not kill the board), stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if rep.Header.MainHealth == nil {
		t.Fatal("actions header must carry mainHealth even when every probe failed")
	}
	if rep.Header.MainHealth.Unknown != len(deskkit.AllowedRepos()) {
		t.Errorf("unknown = %d, want all %d repos could-not-check", rep.Header.MainHealth.Unknown, len(deskkit.AllowedRepos()))
	}
	if rep.Header.MainHealth.Green != 0 {
		t.Errorf("a failed probe must never be counted green; green = %d", rep.Header.MainHealth.Green)
	}
}

// ---------------------------------------------------------------------------
// absence is the guarantee: a verb that did not probe must SAY NOTHING
// ---------------------------------------------------------------------------

// probingVerbs assess default-branch health, so they MUST carry the field.
var probingVerbs = map[string]bool{"actions": true, "health": true}

// nonProbingVerb is one verb that does not probe, with the arguments and fixtures it
// needs to reach exit 0 under the fake gh.
type nonProbingVerb struct {
	verb  string
	args  []string
	setup func(t *testing.T)
}

// nonProbingVerbs — every subcommand that does NOT assess branch health. The list is
// checked for completeness against dispatch() by TestVerbInventory_Complete, so a verb
// added later cannot quietly escape this guard.
func nonProbingVerbs() []nonProbingVerb {
	const repo = "example-org/tracker"
	prList := func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_PRLIST_JSON",
			`[{"number":1,"title":"t","state":"OPEN","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"abc123","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}]`)
	}
	return []nonProbingVerb{
		{verb: "prs", args: []string{"prs"}, setup: prList},
		{verb: "queue", args: []string{"queue"}},
		{verb: "awaiting", args: []string{"awaiting"}, setup: func(t *testing.T) {
			installFakeStatusgen(t)
			twoRoots(t)
		}},
		{verb: "nextup", args: []string{"nextup"}, setup: func(t *testing.T) {
			installFakeStatusgen(t)
			twoRoots(t)
		}},
		// #321: the dispatch queue and its aliases sweep the configured roots via
		// statusgen --next-up, exactly like awaiting/nextup, and probe no branch health.
		{verb: "dispatch", args: []string{"dispatch"}, setup: func(t *testing.T) {
			installFakeStatusgen(t)
			twoRoots(t)
		}},
		{verb: "todo", args: []string{"todo"}, setup: func(t *testing.T) {
			installFakeStatusgen(t)
			twoRoots(t)
		}},
		{verb: "next", args: []string{"next"}, setup: func(t *testing.T) {
			installFakeStatusgen(t)
			twoRoots(t)
		}},
		{verb: "next-up", args: []string{"next-up"}, setup: func(t *testing.T) {
			installFakeStatusgen(t)
			twoRoots(t)
		}},
		{verb: "scope", args: []string{"scope"}, setup: func(t *testing.T) {
			t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
		}},
		{verb: "policydrift", args: []string{"policydrift"}, setup: func(t *testing.T) {
			t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
		}},
		{verb: "reviews", args: []string{"reviews", repo, "1"}, setup: prList},
		{verb: "diff", args: []string{"diff", repo, "1"}, setup: prList},
		{verb: "files", args: []string{"files", repo, "1"}, setup: prList},
		// stalled: default shim returns a fresh commit (not stalled) → empty board, exit 0.
		// A draft PR is supplied so the review/commit/comments reads are exercised.
		{verb: "stalled", args: []string{"stalled"}, setup: func(t *testing.T) {
			prList(t)
			t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
				`[{"user":{"login":"assay-reviewer-app[bot]"},"state":"CHANGES_REQUESTED","commit_id":"abc123","submitted_at":"2026-08-01T00:00:00Z"}]`)
		}},
	}
}

// healthMarkers are every string by which a mainHealth report makes itself known on
// either output path — the JSON field, and the three lines renderAlarms can emit
// (including the ALWAYS-printed summary, which a zero-valued report still prints as
// "0 green · 0 RED …": the exact "nothing is red" misreading this PR exists to kill).
var healthMarkers = []string{"mainHealth", "main-health:", "MAIN-RED", "MAIN-UNKNOWN"}

// TestNonProbingVerbs_OmitMainHealth — an ABSENT mainHealth field means "this verb did
// not probe", which a consumer must be able to tell apart from a probe that found
// nothing wrong. Asserted for EVERY non-probing verb on BOTH output paths, because the
// claim in README.md, main.go's usage and health.go's doc comment is made about the
// whole set, not about `prs` alone (#377 review, R1).
//
// The positive-control half is TestProbingVerbs_CarryMainHealth below: without it this
// test would also pass if the markers were never emitted anywhere.
func TestNonProbingVerbs_OmitMainHealth(t *testing.T) {
	for _, c := range nonProbingVerbs() {
		for _, mode := range []string{"json", "table"} {
			t.Run(c.verb+"/"+mode, func(t *testing.T) {
				installFakeGH(t)
				if c.setup != nil {
					c.setup(t)
				}
				args := append([]string(nil), c.args...)
				if mode == "table" {
					args = append(args, "--table")
				}
				var out, errb bytes.Buffer
				if code := run(args, &out, &errb); code != deskkit.ExitOK {
					t.Fatalf("run(%v) = exit %d, stderr=%s", args, code, errb.String())
				}
				// --table sends banners to stderr and the table to stdout; check both.
				body := out.String() + errb.String()
				for _, m := range healthMarkers {
					if strings.Contains(body, m) {
						t.Errorf("%v carries %q — it never probed branch health, and a report it did not compute "+
							"reads as \"nothing is red\"; got:\n%s", args, m, body)
					}
				}
			})
		}
	}
}

// TestProbingVerbs_CarryMainHealth is the control for the test above: the verbs that DO
// probe emit the field on the JSON path and the summary line on the table path. If this
// fails, the absence assertions are vacuous.
func TestProbingVerbs_CarryMainHealth(t *testing.T) {
	for verb := range probingVerbs {
		for _, mode := range []string{"json", "table"} {
			t.Run(verb+"/"+mode, func(t *testing.T) {
				installFakeGH(t)
				args := []string{verb}
				want := "mainHealth"
				if mode == "table" {
					args = append(args, "--table")
					want = "main-health:"
				}
				var out, errb bytes.Buffer
				if code := run(args, &out, &errb); code != deskkit.ExitOK {
					t.Fatalf("run(%v) = exit %d, stderr=%s", args, code, errb.String())
				}
				if body := out.String() + errb.String(); !strings.Contains(body, want) {
					t.Errorf("%v must carry %q — it probed; got:\n%s", args, want, body)
				}
			})
		}
	}
}

// TestVerbInventory_Complete keeps the guard above honest as the tool grows: every
// subcommand dispatch() accepts must be classified as probing or non-probing. A new
// verb that threads a Header through without deciding which it is fails HERE, rather
// than shipping a mainHealth nobody computed (or losing one somebody did).
func TestVerbInventory_Complete(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading main.go: %v", err)
	}
	body := string(src)
	start := strings.Index(body, "func dispatch(")
	if start < 0 {
		t.Fatal("dispatch() not found in main.go — this test no longer reads what it claims to")
	}
	end := strings.Index(body[start:], "\n}\n")
	if end < 0 {
		t.Fatal("could not find the end of dispatch() in main.go")
	}
	dispatchSrc := body[start : start+end]

	// Collect every quoted case label inside dispatch().
	var verbs []string
	for _, ln := range strings.Split(dispatchSrc, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "case \"") {
			continue
		}
		for _, part := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(ln, "case "), ":"), ",") {
			if v, err := strconv.Unquote(strings.TrimSpace(part)); err == nil {
				verbs = append(verbs, v)
			}
		}
	}
	if len(verbs) < 5 {
		t.Fatalf("parsed only %d verbs from dispatch() — the parse is broken, not the code: %v", len(verbs), verbs)
	}

	covered := map[string]bool{}
	for _, c := range nonProbingVerbs() {
		covered[c.verb] = true
	}
	for _, v := range verbs {
		if probingVerbs[v] || covered[v] {
			continue
		}
		// A REFUSED verb (#321) produces no report at all, so there is no header to
		// carry or omit `mainHealth` on and neither list can hold it. That exemption is
		// PROVEN here, not asserted: the verb is run and must both refuse and stay
		// silent on every health marker. An exemption nobody checks is exactly the
		// "classified nowhere" hole this test exists to close (#400).
		if verbScopeClass[v] == refusedVerb {
			t.Run("refused/"+v, func(t *testing.T) {
				installFakeGH(t)
				var out, errb bytes.Buffer
				if code := run([]string{v}, &out, &errb); code != deskkit.ExitRefused {
					t.Fatalf("verb %q is exempted from the probing split as REFUSED, but run(%q) = exit %d, want %d",
						v, v, code, deskkit.ExitRefused)
				}
				body := out.String() + errb.String()
				for _, m := range healthMarkers {
					if strings.Contains(body, m) {
						t.Errorf("refused verb %q emitted %q — it computed no health report; got:\n%s", v, m, body)
					}
				}
			})
			continue
		}
		t.Errorf("verb %q is dispatched but classified nowhere: add it to nonProbingVerbs() "+
			"(and to README.md's \"do not probe\" list) or to probingVerbs", v)
	}
	// And nothing stale in the other direction.
	all := map[string]bool{}
	for _, v := range verbs {
		all[v] = true
	}
	for v := range covered {
		if !all[v] {
			t.Errorf("nonProbingVerbs() lists %q, which dispatch() no longer accepts", v)
		}
	}
}

// TestREADME_NamesEveryNonProbingVerb — the README sentence that tells consumers which
// verbs do not probe is load-bearing documentation (a consumer reading it decides
// whether an absent field is meaningful). Keep it in step with the guard above.
func TestREADME_NamesEveryNonProbingVerb(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("reading tools/desk/README.md: %v", err)
	}
	const claim = "do not probe"
	i := strings.Index(string(b), claim)
	if i < 0 {
		t.Fatalf("README.md no longer states which verbs %q — the absence guarantee is undocumented", claim)
	}
	// The sentence: back to the previous blank line, forward to the claim.
	para := string(b)[:i]
	if j := strings.LastIndex(para, "\n\n"); j >= 0 {
		para = para[j:]
	}
	for _, c := range nonProbingVerbs() {
		if !strings.Contains(para, "`"+c.verb+"`") {
			t.Errorf("README.md's %q sentence omits `%s`; it reads: %s", claim, c.verb, strings.TrimSpace(para))
		}
	}
}

// errString is a tiny error type so the stubs can return gh-shaped failures.
type errString string

func (e errString) Error() string { return string(e) }
