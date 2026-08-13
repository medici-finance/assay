package main

// delta_test.go — tests for the --delta / --quiet modes (deskquiet).
//
// The fail-dangerous direction (brief exec-tier-why) is a delta mode that HIDES a new
// actionable row behind a bad diff while the tests pass. These tests prove the
// fail-open direction holds: every unassessable snapshot yields FULL output, never an
// empty diff; a partial read never advances the snapshot; changed/new/removed rows are
// detected; and the --quiet line format is pinned.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// prJSON builds a one-PR pr-list fixture for the scoped repo.
func prJSON(repo string, n int, title, head string, draft bool, state string) string {
	return fmt.Sprintf(
		`[{"number":%d,"title":%q,"isDraft":%t,"author":{"login":"shared-agent"},"headRefOid":%q,"mergeStateStatus":%q,"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`,
		n, title, draft, head, state)
}

// twoPRJSON builds a two-PR fixture.
func twoPRJSON(repo string, n1 int, t1, h1 string, d1 bool, s1 string, n2 int, t2, h2 string, d2 bool, s2 string) string {
	return fmt.Sprintf(
		`[{"number":%d,"title":%q,"isDraft":%t,"author":{"login":"shared-agent"},"headRefOid":%q,"mergeStateStatus":%q,"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]},`+
			`{"number":%d,"title":%q,"isDraft":%t,"author":{"login":"shared-agent"},"headRefOid":%q,"mergeStateStatus":%q,"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`,
		n1, t1, d1, h1, s1, n2, t2, d2, h2, s2)
}

const deltaTestRepo = "example-org/tracker"

// deltaRunner binds ONE fake-gh + HOME environment (so the snapshot dir persists
// across multiple run() calls within a single test) to a run function. Call
// newDeltaRunner once per (sub)test; the HOME it sets stays for that test's lifetime.
func newDeltaRunner(t *testing.T) func(prFixture string, flags ...string) (string, string, int) {
	t.Helper()
	installFakeGH(t) // sets a stable temp HOME for this test + the gh shim
	t.Setenv("DESKBOARD_GH_PR_REPO", deltaTestRepo)
	return func(prFixture string, flags ...string) (string, string, int) {
		t.Helper()
		// t.Setenv is scoped to the whole test, so setting it each call is fine: the
		// last write wins and HOME (the snapshot location) is untouched.
		t.Setenv("DESKBOARD_GH_PRLIST_JSON", prFixture)
		var out, errb bytes.Buffer
		code := run(append([]string{"prs"}, flags...), &out, &errb)
		return out.String(), errb.String(), code
	}
}

// runDeltaPRs is the one-shot form: a fresh env + a single run. Each call is a
// fresh HOME, so it is ONLY for first-run / stateless assertions.
func runDeltaPRs(t *testing.T, prFixture string, flags ...string) (string, string, int) {
	t.Helper()
	return newDeltaRunner(t)(prFixture, flags...)
}

// snapshotPathFor returns the on-disk snapshot path for a subcommand (same key the
// code uses), so tests can corrupt / inspect / verify it.
func snapshotPathFor(t *testing.T, sub string) string {
	t.Helper()
	p, err := snapshotFile(sub, deskkit.AllowedRepos())
	if err != nil {
		t.Fatalf("snapshotFile: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// TestDeltaMode (Verify item 1)
// ---------------------------------------------------------------------------

func TestDeltaMode(t *testing.T) {
	run := newDeltaRunner(t)
	// FIRST RUN — no prior snapshot → full output + first-run label.
	out, _, code := run(prJSON(deltaTestRepo, 100, "fix alpha", "aaaaaaa1", true, "CLEAN"), "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("first run exit=%d, want 0", code)
	}
	if !strings.Contains(out, "first run — no prior snapshot") {
		t.Errorf("first run should carry the first-run label; got:\n%s", out)
	}
	// First run must print the FULL table (the PR row is visible), not a delta.
	if !strings.Contains(out, "#100") {
		t.Errorf("first run should print the full table including #100; got:\n%s", out)
	}

	// UNCHANGED RUN — same data → "no change".
	out, _, code = run(prJSON(deltaTestRepo, 100, "fix alpha", "aaaaaaa1", true, "CLEAN"), "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("unchanged run exit=%d", code)
	}
	if !strings.Contains(out, "no change") {
		t.Errorf("unchanged run should print 'no change'; got:\n%s", out)
	}
	if strings.Contains(out, "first run") || strings.Contains(out, "snapshot reset") {
		t.Errorf("unchanged run should NOT carry a first-run/reset label; got:\n%s", out)
	}

	// FIELD-CHANGED — title changes → ~ row.
	out, _, code = run(prJSON(deltaTestRepo, 100, "fix alpha RENAMED", "aaaaaaa1", true, "CLEAN"), "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("field-changed run exit=%d", code)
	}
	if !strings.Contains(out, "~ changed") || !strings.Contains(out, "#100") {
		t.Errorf("field-changed run should show ~ changed #100; got:\n%s", out)
	}
	// Re-run so the snapshot advances past the field change before the next case.
	if _, _, c := run(prJSON(deltaTestRepo, 100, "fix alpha RENAMED", "aaaaaaa1", true, "CLEAN"), "--delta"); c != deskkit.ExitOK {
		t.Fatalf("post-change settle run exit=%d", c)
	}

	// NEW + REMOVED — swap #100 for #200.
	out, _, code = run(prJSON(deltaTestRepo, 200, "new work", "bbbbbbb1", true, "CLEAN"), "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("new+removed run exit=%d", code)
	}
	if !strings.Contains(out, "+ added") || !strings.Contains(out, "#200") {
		t.Errorf("new+removed run should show + added #200; got:\n%s", out)
	}
	if !strings.Contains(out, "- removed") || !strings.Contains(out, "#100") {
		t.Errorf("new+removed run should show - removed #100; got:\n%s", out)
	}
}

// TestDeltaMode_HeadChangeIsFieldChange: a head-SHA advance (push) is a field change,
// not a new item — the PR keeps its number.
func TestDeltaMode_HeadChangeIsFieldChange(t *testing.T) {
	run := newDeltaRunner(t)
	// Seed + settle.
	run(prJSON(deltaTestRepo, 50, "feat", "head0001", true, "CLEAN"), "--delta")
	run(prJSON(deltaTestRepo, 50, "feat", "head0001", true, "CLEAN"), "--delta")
	// Push: head advances to head0002.
	out, _, code := run(prJSON(deltaTestRepo, 50, "feat", "head0002", true, "CLEAN"), "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("head-change exit=%d", code)
	}
	if !strings.Contains(out, "~ changed") {
		t.Errorf("a head-SHA change is a field change (~), got:\n%s", out)
	}
	if strings.Contains(out, "+ added") || strings.Contains(out, "- removed") {
		t.Errorf("same PR number should not show as add/remove; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// TestDeltaFailOpen (Verify item 2)
// ---------------------------------------------------------------------------

func TestDeltaFailOpen(t *testing.T) {
	t.Run("corrupt snapshot → full output + reset, never empty", func(t *testing.T) {
		run := newDeltaRunner(t)
		// Seed a good snapshot first.
		run(prJSON(deltaTestRepo, 10, "seed", "s1", true, "CLEAN"), "--delta")
		// Corrupt the snapshot file.
		path := snapshotPathFor(t, "prs")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
			t.Fatal(err)
		}
		// Run with --delta against the SAME data (an empty diff would be the bug).
		out, _, code := run(prJSON(deltaTestRepo, 10, "seed", "s1", true, "CLEAN"), "--delta")
		if code != deskkit.ExitOK {
			t.Fatalf("corrupt-snapshot exit=%d", code)
		}
		if !strings.Contains(out, "snapshot reset") {
			t.Errorf("corrupt snapshot should yield 'snapshot reset' label; got:\n%s", out)
		}
		// CRITICAL: full output, never an empty diff. The PR row must be visible.
		if !strings.Contains(out, "#10") {
			t.Errorf("corrupt snapshot must print FULL output (the row), not hide it; got:\n%s", out)
		}
	})

	t.Run("missing snapshot → first run, full output, never empty", func(t *testing.T) {
		// No seeding — the snapshot dir is fresh (installFakeGH uses a fresh temp HOME).
		out, _, code := runDeltaPRs(t, prJSON(deltaTestRepo, 20, "fresh", "f1", true, "CLEAN"), "--delta")
		if code != deskkit.ExitOK {
			t.Fatalf("missing-snapshot exit=%d", code)
		}
		if !strings.Contains(out, "first run") {
			t.Errorf("missing snapshot should yield 'first run' label; got:\n%s", out)
		}
		if !strings.Contains(out, "#20") {
			t.Errorf("missing snapshot must print FULL output; got:\n%s", out)
		}
	})

	t.Run("schema-mismatched snapshot → full output + reset", func(t *testing.T) {
		run := newDeltaRunner(t)
		run(prJSON(deltaTestRepo, 30, "schema", "sc1", true, "CLEAN"), "--delta")
		path := snapshotPathFor(t, "prs")
		// Write a snapshot from a future/incompatible schema version.
		if err := os.WriteFile(path, []byte(`{"schema":999,"items":{"x":"y"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		out, _, code := run(prJSON(deltaTestRepo, 30, "schema", "sc1", true, "CLEAN"), "--delta")
		if code != deskkit.ExitOK {
			t.Fatalf("schema-mismatch exit=%d", code)
		}
		if !strings.Contains(out, "snapshot reset") {
			t.Errorf("schema-mismatch should yield 'snapshot reset'; got:\n%s", out)
		}
		if !strings.Contains(out, "#30") {
			t.Errorf("schema-mismatch must print FULL output; got:\n%s", out)
		}
	})

	t.Run("partial read leaves prior snapshot intact", func(t *testing.T) {
		run := newDeltaRunner(t)
		// Seed a good snapshot and capture its bytes.
		run(prJSON(deltaTestRepo, 40, "persist", "p1", true, "CLEAN"), "--delta")
		path := snapshotPathFor(t, "prs")
		priorBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read prior snapshot: %v", err)
		}
		// Now make gh FAIL for one repo mid-sweep → deskboard aborts exit 6.
		// The failed read must NOT overwrite the prior snapshot.
		t.Setenv("DESKBOARD_GH_FAIL_REPO", "example-org/agents") // a repo in the allowed set
		_, _, code := run(prJSON(deltaTestRepo, 40, "persist", "p1", true, "CLEAN"), "--delta")
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("partial read should exit 6 (unverifiable), got %d", code)
		}
		afterBytes, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("read post-fail snapshot: %v", rerr)
		}
		if string(afterBytes) != string(priorBytes) {
			t.Errorf("partial/errored read must NOT overwrite the prior snapshot;\n prior=%s\n after=%s", priorBytes, afterBytes)
		}
	})
}

// ---------------------------------------------------------------------------
// TestQuietLine (Verify item 3)
// ---------------------------------------------------------------------------

func TestQuietLine(t *testing.T) {
	run := newDeltaRunner(t)
	// First run — Δ carries the "first run" label, not a misleading zero.
	out, _, code := run(prJSON(deltaTestRepo, 1, "one", "h1", true, "CLEAN"), "--quiet")
	if code != deskkit.ExitOK {
		t.Fatalf("quiet first-run exit=%d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("--quiet should print exactly ONE line; got %d lines:\n%s", len(lines), out)
	}
	line := lines[0]
	// Pinned format: <sub>: <summary> | Δ ... | N actionable
	if !strings.HasPrefix(line, "prs:") {
		t.Errorf("quiet line should start with 'prs:'; got %q", line)
	}
	if !strings.Contains(line, "1 open (1 draft)") {
		t.Errorf("quiet summary should state '1 open (1 draft)'; got %q", line)
	}
	if !strings.Contains(line, "Δ first run") {
		t.Errorf("first-run quiet line should carry 'Δ first run'; got %q", line)
	}
	// Third segment: a labelled count, not a bare "N actionable" restating the
	// summary (review Minor 1). Nothing is CI-red or conflicting here, so it is 0.
	if !strings.HasSuffix(line, "0 ci-red/conflicting (see `actions` for the ACTION class)") {
		t.Errorf("quiet line should end with the labelled attention count; got %q", line)
	}

	// Settle the snapshot.
	run(prJSON(deltaTestRepo, 1, "one", "h1", true, "CLEAN"), "--delta")

	// Unchanged quiet run — Δ 0.
	out, _, code = run(prJSON(deltaTestRepo, 1, "one", "h1", true, "CLEAN"), "--quiet")
	if code != deskkit.ExitOK {
		t.Fatalf("quiet unchanged exit=%d", code)
	}
	line = strings.Split(strings.TrimRight(out, "\n"), "\n")[0]
	if !strings.Contains(line, "Δ 0") {
		t.Errorf("unchanged quiet line should carry 'Δ 0'; got %q", line)
	}

	// Two PRs total (add #2), keep #1 → Δ +1.
	out, _, code = run(
		twoPRJSON(deltaTestRepo, 1, "one", "h1", true, "CLEAN", 2, "two", "h2", false, "CLEAN"),
		"--quiet")
	if code != deskkit.ExitOK {
		t.Fatalf("quiet add exit=%d", code)
	}
	line = strings.Split(strings.TrimRight(out, "\n"), "\n")[0]
	// 2 open (1 draft) — #1 draft, #2 not draft.
	if !strings.Contains(line, "Δ +1") {
		t.Errorf("one-added quiet line should carry 'Δ +1'; got %q", line)
	}
	if !strings.Contains(line, "2 open (1 draft)") {
		t.Errorf("summary should reflect 2 open 1 draft; got %q", line)
	}
}

// TestQuietComposableWithDelta: --quiet --delta prints the quiet line FIRST, then
// the changed rows.
func TestQuietComposableWithDelta(t *testing.T) {
	run := newDeltaRunner(t)
	// Seed + settle.
	run(prJSON(deltaTestRepo, 7, "seed", "s7", true, "CLEAN"), "--delta")
	run(prJSON(deltaTestRepo, 7, "seed", "s7", true, "CLEAN"), "--delta")
	// Add one.
	out, _, code := run(
		twoPRJSON(deltaTestRepo, 7, "seed", "s7", true, "CLEAN", 8, "new", "s8", true, "CLEAN"),
		"--quiet", "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("quiet+delta exit=%d", code)
	}
	// Line 0 is the quiet summary; later lines carry the delta rows.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("quiet+delta should print the quiet line AND delta rows; got %d lines:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "prs:") || !strings.Contains(lines[0], "Δ +1") {
		t.Errorf("line 0 should be the quiet summary with Δ +1; got %q", lines[0])
	}
	joined := strings.Join(lines[1:], "\n")
	if !strings.Contains(joined, "+ added") || !strings.Contains(joined, "#8") {
		t.Errorf("delta rows should follow the quiet line and show + added #8; got:\n%s", joined)
	}
}

// ---------------------------------------------------------------------------
// pure unit tests (diff + extractors, no gh)
// ---------------------------------------------------------------------------

func TestDiffSets(t *testing.T) {
	cur := deltaSet{Items: []deltaItem{
		{ID: "a", Signature: "1"},  // unchanged
		{ID: "b", Signature: "2x"}, // changed (was "2")
		{ID: "c", Signature: "3"},  // added
	}}
	prev := snapshot{Schema: snapshotSchema, Items: map[string]string{
		"a": "1",
		"b": "2",
		"z": "gone", // removed
	}}
	dr := diffSets(cur, prev, snapOK)
	if len(dr.added) != 1 || dr.added[0].ID != "c" {
		t.Errorf("added = %+v, want [c]", dr.added)
	}
	if len(dr.changed) != 1 || dr.changed[0].ID != "b" {
		t.Errorf("changed = %+v, want [b]", dr.changed)
	}
	if len(dr.removed) != 1 || dr.removed[0].ID != "z" {
		t.Errorf("removed = %+v, want [z]", dr.removed)
	}
	// Unassessable baseline → no diff surfaced (caller prints full output).
	dr = diffSets(cur, prev, snapReset)
	if dr.hasDelta() {
		t.Errorf("reset must yield no delta (full output path), got %+v", dr)
	}
}

func TestSnapshot_AssessmentResetHeals(t *testing.T) {
	// After a reset, the successful full read overwrites the bad file, so the NEXT
	// run diffs cleanly against the healed snapshot.
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	path := filepath.Join(dir, ".config", "assay", "snapshots", "prs-test.json")
	// Corrupt.
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("garbage"), 0o600)
	prev, assess := loadSnapshot(path)
	if assess != snapReset {
		t.Fatalf("corrupt → assess=%v, want snapReset", assess)
	}
	// A save (the post-read heal step) makes the next load clean.
	if err := saveSnapshot(path, snapshot{Items: map[string]string{"a": "1"}}); err != nil {
		t.Fatal(err)
	}
	prev, assess = loadSnapshot(path)
	if assess != snapOK {
		t.Fatalf("after heal → assess=%v, want snapOK (items=%v)", assess, prev.Items)
	}
}

func TestPrsDeltaSetSummary(t *testing.T) {
	rep := prsReport{PRs: []prRow{
		{Repo: "x/y", Number: 1, Draft: true},
		{Repo: "x/y", Number: 2, Draft: false},
	}, External: []externalRow{{Repo: "x/y", Number: 9}}}
	ds, ok := prsDeltaSet(rep)
	if !ok {
		t.Fatal("prsDeltaSet returned false")
	}
	if len(ds.Items) != 3 { // 2 PRs + 1 external
		t.Errorf("items = %d, want 3", len(ds.Items))
	}
	if ds.Summary != "2 open (1 draft)" {
		t.Errorf("summary = %q, want '2 open (1 draft)'", ds.Summary)
	}
	// Neither PR is CI-red or un-mergeable, so the attention count is 0 — it must
	// NOT restate len(rep.PRs) (review Minor 1).
	if ds.Actionable != 0 {
		t.Errorf("actionable = %d, want 0 (no CI-red / conflicting row)", ds.Actionable)
	}
}

// TestPrsAttention_CountsCiRedAndConflicting pins the labelled third segment to the
// subset the `prs` payload can actually prove, so it can never silently degenerate
// back into "len(PRs)" — a count that restates the summary teaches a desk nothing
// and reads as a claim about rows the subcommand never classified.
func TestPrsAttention_CountsCiRedAndConflicting(t *testing.T) {
	rep := prsReport{PRs: []prRow{
		{Repo: "x/y", Number: 1, MergeState: "CLEAN", CIPass: 1},    // fine
		{Repo: "x/y", Number: 2, MergeState: "CLEAN", CIFail: 1},    // CI red
		{Repo: "x/y", Number: 3, MergeState: "DIRTY", CIPass: 1},    // conflicting
		{Repo: "x/y", Number: 4, MergeState: "BLOCKED", CIPass: 1},  // blocked
		{Repo: "x/y", Number: 5, MergeState: "CLEAN", CIPending: 1}, // still running
	}}
	ds, ok := prsDeltaSet(rep)
	if !ok {
		t.Fatal("prsDeltaSet returned false")
	}
	if ds.Actionable != 3 {
		t.Errorf("attention count = %d, want 3 (#2 ci-red, #3 dirty, #4 blocked)", ds.Actionable)
	}
	if !strings.Contains(ds.ActionableLabel, "ci-red/conflicting") {
		t.Errorf("label must name what it counts; got %q", ds.ActionableLabel)
	}
}

func TestNextupDeltaSetSummary(t *testing.T) {
	rep := nextupReport{Rows: []nextupRow{
		{Repo: "x/y", Stream: "s", Brief: "s/1", Status: "todo", Score: 100},
		{Repo: "x/y", Stream: "s", Brief: "s/2", Status: "in-progress", Score: 200},
		{Repo: "x/y", Stream: "s", Brief: "s/3", Status: "implemented", Score: 0},
	}, Roots: []deskkit.RootConfig{{Repo: "x/y"}}}
	ds, ok := nextupDeltaSet(rep)
	if !ok {
		t.Fatal("nextupDeltaSet returned false")
	}
	if ds.Summary != "3 awaiting (1 todo, 1 in-progress)" {
		t.Errorf("summary = %q", ds.Summary)
	}
	if ds.Actionable != 2 { // todo + in-progress
		t.Errorf("actionable = %d, want 2", ds.Actionable)
	}
	if len(ds.RepoSet) != 1 || ds.RepoSet[0] != "x/y" {
		t.Errorf("repoSet = %v", ds.RepoSet)
	}
}

func TestQueueDeltaSet_LabelsInSignature(t *testing.T) {
	rep := queueReport{Issues: []issueRow{
		{Repo: "x/y", Number: 5, Title: "t", Labels: []string{"b", "a"}},
	}}
	ds, ok := queueDeltaSet(rep)
	if !ok {
		t.Fatal("queueDeltaSet returned false")
	}
	// Labels are sorted into the signature so a label change is detected.
	if !strings.Contains(ds.Items[0].Signature, "labels=a,b") {
		t.Errorf("signature should carry sorted labels; got %q", ds.Items[0].Signature)
	}
}

// TestDelta_UnsupportedSubcommandRefused: --delta on a subcommand that does not
// support it is Refused (exit 5), never silently ignored.
func TestDelta_UnsupportedSubcommandRefused(t *testing.T) {
	installFakeGH(t)
	var out, errb bytes.Buffer
	code := run([]string{"diff", "example-org/tracker", "1", "--delta"}, &out, &errb)
	if code != deskkit.ExitRefused {
		t.Fatalf("exit=%d, want 5 (refused)", code)
	}
}

// TestDeltaFailOpen_ReportShapeMismatch: if a subcommand claims delta support but
// its report value drifts away from the shape its extractor asserts, the console must
// go NOISY (full output + a reset-shaped label), never silent. This is the same
// fail-open direction as a corrupt snapshot — the failure mode the brief calls
// fail-dangerous is an empty console, not a redundant one.
func TestDeltaFailOpen_ReportShapeMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rendered := "FULL-TABLE-ROW #4242\n"
	rep := &Report{
		value:  struct{ NotAPRSReport bool }{true}, // deliberately the wrong shape
		render: func(w io.Writer) { fmt.Fprint(w, rendered) },
		detail: "d",
	}
	var out, errb bytes.Buffer
	detail, applied := runDeltaQuiet(&out, &errb, "prs", rep, true, true)
	if !applied {
		t.Fatalf("a supported subcommand must stay applied (fail open), got applied=false")
	}
	if !strings.Contains(out.String(), rendered) {
		t.Errorf("shape mismatch must print the FULL report, got stdout:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "snapshot reset") {
		t.Errorf("shape mismatch must carry a reset-shaped label, got stdout:\n%s", out.String())
	}
	if !strings.Contains(errb.String(), "WARNING") {
		t.Errorf("shape mismatch must warn on stderr, got stderr:\n%s", errb.String())
	}
	if !strings.Contains(detail, "delta=unavailable") {
		t.Errorf("audit detail should record the unavailable delta, got %q", detail)
	}
}

// TestSnapshot_RepoSetKeyOrderIndependent: the same repos in a different order
// hash to the same key, so a reshuffled AllowedRepos() never splits snapshots.
func TestSnapshot_RepoSetKeyOrderIndependent(t *testing.T) {
	a := repoSetKey([]string{"c", "a", "b"})
	b := repoSetKey([]string{"a", "b", "c"})
	if a != b {
		t.Errorf("repoSetKey must be order-independent; got %q vs %q", a, b)
	}
}

// ---------------------------------------------------------------------------
// baseline consumption — PR #483 review, finding 3
//
// A --quiet run prints COUNTS, not rows. If it also advanced the snapshot it would
// consume the delta it just advertised, and the desk's own follow-up --delta would
// answer "no change" — the new row unrecoverable through the delta path. That is
// quiet-by-accident, the failure direction this whole file guards. The baseline is
// therefore "the last state the desk was SHOWN", advanced only by a rendering run.
// ---------------------------------------------------------------------------

// TestQuietOnly_HoldsBaseline is the finding-3 regression: the exact two-invocation
// sequence a desk loop produces — a quiet line reporting Δ +1, then the drill-in.
func TestQuietOnly_HoldsBaseline(t *testing.T) {
	run := newDeltaRunner(t)
	// Seed + settle on one PR.
	run(prJSON(deltaTestRepo, 10, "alpha", "aaaa0001", true, "CLEAN"), "--delta")
	run(prJSON(deltaTestRepo, 10, "alpha", "aaaa0001", true, "CLEAN"), "--delta")

	two := twoPRJSON(deltaTestRepo, 10, "alpha", "aaaa0001", true, "CLEAN", 20, "beta", "bbbb0001", true, "CLEAN")

	// #20 appears. The quiet loop line advertises it.
	quiet, _, code := run(two, "--quiet")
	if code != deskkit.ExitOK {
		t.Fatalf("quiet exit=%d", code)
	}
	if !strings.Contains(quiet, "Δ +1") {
		t.Fatalf("quiet line should advertise Δ +1; got:\n%s", quiet)
	}

	// The desk drills in. The row it was told about MUST still be there.
	out, _, code := run(two, "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("follow-up delta exit=%d", code)
	}
	if strings.Contains(out, "no change") {
		t.Fatalf("a --quiet run must NOT consume the delta: follow-up --delta said 'no change'; got:\n%s", out)
	}
	if !strings.Contains(out, "+ added") || !strings.Contains(out, "#20") {
		t.Errorf("follow-up --delta must still show + added #20; got:\n%s", out)
	}

	// And the rendering run DID consume it — the next --delta is quiet-clean.
	out, _, _ = run(two, "--delta")
	if !strings.Contains(out, "no change") {
		t.Errorf("after a rendering run the baseline must advance; got:\n%s", out)
	}
}

// TestQuietOnly_BadgeAccumulates: repeated quiet-only iterations diff against the
// same held baseline, so the unread badge grows rather than resetting to Δ 0 each
// loop. Without the hold, iteration 2 would report Δ +1 (only the newest row) and
// the earlier one would be lost.
func TestQuietOnly_BadgeAccumulates(t *testing.T) {
	run := newDeltaRunner(t)
	run(prJSON(deltaTestRepo, 10, "alpha", "aaaa0001", true, "CLEAN"), "--delta")
	run(prJSON(deltaTestRepo, 10, "alpha", "aaaa0001", true, "CLEAN"), "--delta")

	out, _, _ := run(twoPRJSON(deltaTestRepo, 10, "alpha", "aaaa0001", true, "CLEAN", 20, "beta", "bbbb0001", true, "CLEAN"), "--quiet")
	if !strings.Contains(out, "Δ +1") {
		t.Fatalf("first quiet iteration should read Δ +1; got %q", out)
	}
	// #20's head advances too: one added (#20) AND one changed (#10 renamed).
	out, _, _ = run(twoPRJSON(deltaTestRepo, 10, "alpha RENAMED", "aaaa0001", true, "CLEAN", 20, "beta", "bbbb0001", true, "CLEAN"), "--quiet")
	if !strings.Contains(out, "Δ +1 ~1") {
		t.Errorf("second quiet iteration should still see BOTH unread changes (Δ +1 ~1); got %q", out)
	}
}

// TestQuietOnly_UntrustedBaselineHeld: with no trusted baseline, a quiet-only run
// must not adopt the unseen board as "already seen". It keeps saying so.
func TestQuietOnly_UntrustedBaselineHeld(t *testing.T) {
	run := newDeltaRunner(t)
	fixture := prJSON(deltaTestRepo, 77, "unseen", "cccc0001", true, "CLEAN")
	for i := 1; i <= 2; i++ {
		out, _, code := run(fixture, "--quiet")
		if code != deskkit.ExitOK {
			t.Fatalf("quiet iteration %d exit=%d", i, code)
		}
		if !strings.Contains(out, "Δ first run") {
			t.Errorf("quiet iteration %d must still label the untrusted baseline; got %q", i, out)
		}
	}
	// A rendering run establishes it — full output, and only then does Δ go numeric.
	out, _, _ := run(fixture, "--delta")
	if !strings.Contains(out, "#77") {
		t.Errorf("the rendering run must show the row; got:\n%s", out)
	}
	out, _, _ = run(fixture, "--quiet")
	if !strings.Contains(out, "Δ 0") {
		t.Errorf("after the rendering run the quiet line should read Δ 0; got %q", out)
	}
}

// TestQuietOnly_AuditRecordsHold: the mandatory audit line records that this
// run held the baseline, so a forensic read can tell a consuming run from a
// non-consuming one.
func TestQuietOnly_AuditRecordsHold(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rep := &Report{
		value:  prsReport{PRs: []prRow{{Repo: "x/y", Number: 1, MergeState: "CLEAN"}}},
		render: func(w io.Writer) { fmt.Fprint(w, "FULL\n") },
		detail: "d",
	}
	var out, errb bytes.Buffer
	held, _ := runDeltaQuiet(&out, &errb, "prs", rep, false, true)
	if !strings.Contains(held, "snapshot=held") {
		t.Errorf("quiet-only audit detail should record the held baseline; got %q", held)
	}
	out.Reset()
	errb.Reset()
	consumed, _ := runDeltaQuiet(&out, &errb, "prs", rep, true, true)
	if strings.Contains(consumed, "snapshot=held") {
		t.Errorf("a rendering run consumes the baseline and must NOT record a hold; got %q", consumed)
	}
}

// ---------------------------------------------------------------------------
// signature field coverage — PR #483 review, finding 6a
//
// The shipped extractors track the right fields, but nothing HELD them there: the
// reviewer reduced each signature (dropping CI/draft/merge-state, replacing one with
// a constant) and the whole package suite stayed green. A signature that stops
// tracking a field drops that PR's transition — going CI-red, flipping draft→ready,
// going CONFLICTING — out of --delta entirely, with a green suite: precisely the
// "quieter console hides a red state" direction. These tests pin each tracked field
// to an actual `~ changed` row.
// ---------------------------------------------------------------------------

// assertFieldTracked requires that mutating one field turns the row into exactly one
// `~ changed` entry (not an add, not a removal, not silence).
func assertFieldTracked(t *testing.T, field string, ex deltaExtractor, base, mutated any, id string) {
	t.Helper()
	baseSet, ok := ex(base)
	if !ok {
		t.Fatalf("%s: extractor rejected the base fixture", field)
	}
	prev := snapshot{Schema: snapshotSchema, Items: map[string]string{}}
	for _, it := range baseSet.Items {
		prev.Items[it.ID] = it.Signature
	}
	curSet, ok := ex(mutated)
	if !ok {
		t.Fatalf("%s: extractor rejected the mutated fixture", field)
	}
	dr := diffSets(curSet, prev, snapOK)
	if len(dr.added) != 0 || len(dr.removed) != 0 {
		t.Errorf("%s: a field change must not add/remove rows; added=%d removed=%d", field, len(dr.added), len(dr.removed))
	}
	if len(dr.changed) != 1 || dr.changed[0].ID != id {
		t.Errorf("%s: must produce exactly one `~ changed` row for %s; got changed=%v", field, id, dr.changed)
	}
}

func prsFixture(mutPR func(*prRow), mutExt func(*externalRow)) prsReport {
	pr := prRow{Repo: "x/y", Number: 1, Title: "t", Draft: true, HeadSHA: "h0", MergeState: "CLEAN", CIPass: 1}
	ext := externalRow{Repo: "x/y", Number: 2, Title: "e", Author: "outsider"}
	if mutPR != nil {
		mutPR(&pr)
	}
	if mutExt != nil {
		mutExt(&ext)
	}
	return prsReport{PRs: []prRow{pr}, External: []externalRow{ext}}
}

func TestPrsSignature_TracksEachField(t *testing.T) {
	base := prsFixture(nil, nil)
	prCases := map[string]func(*prRow){
		"title":      func(r *prRow) { r.Title = "renamed" },
		"draft":      func(r *prRow) { r.Draft = false }, // draft → ready
		"head":       func(r *prRow) { r.HeadSHA = "h1" },
		"mergeState": func(r *prRow) { r.MergeState = "DIRTY" }, // → CONFLICTING
		"ciPass":     func(r *prRow) { r.CIPass = 2 },
		"ciPending":  func(r *prRow) { r.CIPending = 1 },
		"ciFail":     func(r *prRow) { r.CIFail = 1 }, // → CI-red
	}
	for field, mut := range prCases {
		assertFieldTracked(t, "prRow."+field, prsDeltaSet, base, prsFixture(mut, nil), "x/y#1")
	}
	extCases := map[string]func(*externalRow){
		"title":  func(e *externalRow) { e.Title = "renamed" },
		"author": func(e *externalRow) { e.Author = "someone-else" },
	}
	for field, mut := range extCases {
		assertFieldTracked(t, "externalRow."+field, prsDeltaSet, base, prsFixture(nil, mut), "x/y#2")
	}
}

func queueFixture(mut func(*issueRow)) queueReport {
	is := issueRow{Repo: "x/y", Number: 5, Title: "t", Labels: []string{"a", "b"}}
	if mut != nil {
		mut(&is)
	}
	return queueReport{Issues: []issueRow{is}}
}

func TestQueueSignature_TracksEachField(t *testing.T) {
	base := queueFixture(nil)
	cases := map[string]func(*issueRow){
		"title":         func(r *issueRow) { r.Title = "renamed" },
		"labels/added":  func(r *issueRow) { r.Labels = []string{"a", "b", "verify-gate"} },
		"labels/remove": func(r *issueRow) { r.Labels = []string{"a"} },
	}
	for field, mut := range cases {
		assertFieldTracked(t, "issueRow."+field, queueDeltaSet, base, queueFixture(mut), "x/y#5")
	}
	// A reordering is NOT a change (labels are sorted into the signature).
	reordered := queueFixture(func(r *issueRow) { r.Labels = []string{"b", "a"} })
	baseSet, _ := queueDeltaSet(base)
	curSet, _ := queueDeltaSet(reordered)
	if baseSet.Items[0].Signature != curSet.Items[0].Signature {
		t.Errorf("label ORDER must not register as a change; %q vs %q", baseSet.Items[0].Signature, curSet.Items[0].Signature)
	}
}

func nextupFixture(mut func(*nextupRow)) nextupReport {
	r := nextupRow{Repo: "x/y", Root: ".", Stream: "s", Brief: "s/1", Status: "todo", Score: 100, BlockedCount: 0}
	if mut != nil {
		mut(&r)
	}
	return nextupReport{Rows: []nextupRow{r}, Roots: []deskkit.RootConfig{{Repo: "x/y"}}}
}

func TestNextupSignature_TracksEachField(t *testing.T) {
	base := nextupFixture(nil)
	cases := map[string]func(*nextupRow){
		"status":       func(r *nextupRow) { r.Status = "in-progress" }, // the lifecycle flip
		"score":        func(r *nextupRow) { r.Score = 200 },
		"blockedCount": func(r *nextupRow) { r.BlockedCount = 3 },
	}
	for field, mut := range cases {
		assertFieldTracked(t, "nextupRow."+field, nextupDeltaSet, base, nextupFixture(mut), "x/y|s|s/1")
	}
}

// TestPrsDelta_CiRedIsAChangedRow is the end-to-end half of finding 6a: a PR that
// goes CI-red between sweeps must surface as a `~ changed` row through run(), not
// only in a unit signature comparison.
func TestPrsDelta_CiRedIsAChangedRow(t *testing.T) {
	run := newDeltaRunner(t)
	green := prJSONConclusion(deltaTestRepo, 30, "work", "dddd0001", true, "CLEAN", "SUCCESS")
	run(green, "--delta")
	run(green, "--delta")
	red := prJSONConclusion(deltaTestRepo, 30, "work", "dddd0001", true, "CLEAN", "FAILURE")
	out, _, code := run(red, "--delta")
	if code != deskkit.ExitOK {
		t.Fatalf("ci-red run exit=%d", code)
	}
	if !strings.Contains(out, "~ changed") || !strings.Contains(out, "#30") {
		t.Errorf("a PR going CI-red must show as ~ changed #30; got:\n%s", out)
	}
}

// prJSONConclusion is prJSON with the check conclusion under the caller's control.
func prJSONConclusion(repo string, n int, title, head string, draft bool, state, conclusion string) string {
	return fmt.Sprintf(
		`[{"number":%d,"title":%q,"isDraft":%t,"author":{"login":"shared-agent"},"headRefOid":%q,"mergeStateStatus":%q,"statusCheckRollup":[{"status":"COMPLETED","conclusion":%q,"name":"ci"}]}]`,
		n, title, draft, head, state, conclusion)
}

// TestSnapshot_TypeMismatchIsReset pins the json.Unmarshal error branch in
// loadSnapshot on its own (review 6b). The corrupt-snapshot subtest of
// TestDeltaFailOpen does not: its fixture is unparseable, and encoding/json
// validates the whole document before assigning anything, so Schema stays at the
// zero value and the SCHEMA check catches it with the unmarshal check deleted.
//
// A TYPE mismatch is the case that separates them: the document is valid JSON, so
// `schema` is assigned before the decode fails on `items`. Delete the unmarshal
// check and this file reads as snapOK with an empty item map — every current row
// then renders as `+ added` under a delta the desk was told to trust.
func TestSnapshot_TypeMismatchIsReset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prs-deadbeef.json")
	// Valid JSON, right schema, wrong type for items.
	if err := os.WriteFile(path, []byte(`{"schema":1,"items":"not-a-map"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	snap, assess := loadSnapshot(path)
	if assess != snapReset {
		t.Fatalf("a snapshot that fails to decode must be snapReset, got %v", assess)
	}
	if len(snap.Items) != 0 {
		t.Errorf("a reset must hand back NO items to diff against; got %v", snap.Items)
	}
}
