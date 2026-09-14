package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- pure helpers -----------------------------------------------------------

func TestContainsBackfillForm(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		stream    string
		num       string
		wantMatch bool
	}{
		{"slash form in branch", "feat/derived-board/07-rollout", "derived-board", "07", true},
		{"dash form in branch", "feat/derived-board-07-rollout", "derived-board", "07", true},
		{"slash form in body", "This closes derived-board/07 for good.", "derived-board", "07", true},
		{"unrelated text", "This closes derived-board/03 for good.", "derived-board", "07", false},
		{"empty text", "", "derived-board", "07", false},
		{"substring but wrong brief", "derived-board/071", "derived-board", "07", true}, // documents: a literal substring match, not word-boundary — declared, reviewable, and the report surfaces every match for a human to check
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := containsBackfillForm(c.text, c.stream, c.num)
			if got != c.wantMatch {
				t.Errorf("containsBackfillForm(%q, %q, %q) = %v, want %v", c.text, c.stream, c.num, got, c.wantMatch)
			}
		})
	}
}

func TestBriefStreamNum(t *testing.T) {
	if stream, num, ok := briefStreamNum("assay:assay:derived-board:07"); !ok || stream != "derived-board" || num != "07" {
		t.Fatalf("brief-v2 id: got (%q, %q, %v), want (derived-board, 07, true)", stream, num, ok)
	}
	if stream, num, ok := briefStreamNum("derived-board/07"); !ok || stream != "derived-board" || num != "07" {
		t.Fatalf("legacy id: got (%q, %q, %v), want (derived-board, 07, true)", stream, num, ok)
	}
	if _, _, ok := briefStreamNum("not-an-id"); ok {
		t.Fatal("garbage id: want ok=false")
	}
}

func TestMatchBackfillPR(t *testing.T) {
	pulls := []ghPull{
		{Number: 10, MergedAt: "", Body: "Brief: derived-board/07 in body but never merged"},                  // open — must not count
		{Number: 20, MergedAt: "2026-08-01T00:00:00Z", Body: "unrelated work"},                                // merged, no match
		{Number: 30, MergedAt: "2026-08-05T00:00:00Z", Body: "fixes stuff for derived-board/07 historically"}, // merged, body match
		{Number: 25, MergedAt: "2026-08-03T00:00:00Z", Body: "unrelated"},
	}
	match, ok := matchBackfillPR(pulls, "derived-board", "07")
	if !ok {
		t.Fatal("want a match")
	}
	if match.Number != 30 {
		t.Fatalf("want PR #30 (the merged body match), got #%d", match.Number)
	}

	// Highest-numbered merged match wins when more than one merged PR matches.
	pulls = append(pulls, ghPull{Number: 40, MergedAt: "2026-08-06T00:00:00Z", Head: struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	}{Ref: "feat/derived-board-07-followup"}})
	match, ok = matchBackfillPR(pulls, "derived-board", "07")
	if !ok || match.Number != 40 {
		t.Fatalf("want the higher-numbered merged match #40, got #%d ok=%v", match.Number, ok)
	}

	if _, ok := matchBackfillPR(pulls, "derived-board", "99"); ok {
		t.Fatal("want no match for a brief nothing references")
	}
}

func TestApplyReconcileBackfill(t *testing.T) {
	cells := []BriefCell{
		{ID: "assay:assay:derived-board:01", Cell: "implemented", Source: "pr", Witness: "PR #1 (merged aaa1111)"}, // already witnessed — untouched
		{ID: "assay:assay:derived-board:02", Cell: "todo", Source: "pr", Reason: "PR search ran; no open or merged PR carries this brief's trailer"},
		{ID: "assay:assay:derived-board:03", Cell: "todo", Source: "pr", Reason: "PR search ran; no open or merged PR carries this brief's trailer"},
		{ID: "assay:assay:derived-board:04", Cell: "todo", Source: "pr", Reason: "PR search ran; no open or merged PR carries this brief's trailer"},
	}
	pulls := []ghPull{
		{Number: 50, MergedAt: "2026-08-10T00:00:00Z", MergeCommitSHA: "deadbeef00", Body: "Brief-free merge for derived-board/02 back in the day"},
	}
	lookup := func(stream, num string) (string, string, bool) {
		switch num {
		case "03":
			return "implemented", "cafef00d00", true // no PR match, but hand-said advanced — must NOT stay a silent todo
		case "04":
			return "todo", "cafef00d01", true // hand-said also todo — leave alone
		}
		return "", "", false // 02 is answered by the PR match, not by history
	}

	out := applyReconcileBackfill(cells, pulls, true, lookup)

	if out[0].Cell != "implemented" || out[0].Source != "pr" {
		t.Fatalf("01: an already-witnessed cell must be untouched, got %+v", out[0])
	}
	if out[1].Cell != "implemented" || out[1].Source != "backfill" || !strings.Contains(out[1].Witness, "PR #50") {
		t.Fatalf("02: want a backfill PR-match promotion to implemented, got %+v", out[1])
	}
	if out[2].Cell != "unknown" || out[2].Source != "backfill" || !strings.Contains(out[2].Reason, "hand-asserted implemented at cafef00") {
		t.Fatalf("03: want unknown with the hand-asserted reason (never a silent todo), got %+v", out[2])
	}
	if out[3].Cell != "todo" {
		t.Fatalf("04: hand-said todo too — must stay todo, got %+v", out[3])
	}
}

func TestApplyReconcileBackfill_PullFetchFailed(t *testing.T) {
	// The raw pull fetch itself failing must never manufacture a guess on top of
	// a failure — the row is left exactly as the (already honest) normal fold
	// reported it.
	cells := []BriefCell{
		{ID: "assay:assay:derived-board:02", Cell: "todo", Source: "pr", Reason: "PR search ran; no open or merged PR carries this brief's trailer"},
	}
	lookup := func(stream, num string) (string, string, bool) { return "done", "sha1", true }
	out := applyReconcileBackfill(cells, nil, false /* pullsLookedAt */, lookup)
	// pullsLookedAt=false skips the PR-match arm entirely, but the hand-said
	// fallback still runs off the (independently git-read) lookup.
	if out[0].Cell != "unknown" || !strings.Contains(out[0].Reason, "hand-asserted done") {
		t.Fatalf("want the hand-said fallback still applied despite the failed pull fetch, got %+v", out[0])
	}
}

func TestBuildDriftRows(t *testing.T) {
	cells := []BriefCell{
		{ID: "assay:assay:derived-board:01", Cell: "implemented", Witness: "PR #1 (merged aaa1111)"},
		{ID: "assay:assay:derived-board:02", Cell: "unknown", Source: "backfill", Reason: "no witness — hand-asserted done at cafef00"},
		{ID: "assay:assay:derived-board:03", Cell: "implemented", Witness: "PR #9 (merged bbb2222)"},
	}
	lookup := func(stream, num string) (string, string, bool) {
		switch num {
		case "01":
			return "implemented", "sha01", true // agrees — no row
		case "02":
			return "done", "cafef00d", true // disagrees — a row, rendered via the unknown(...) form
		case "03":
			return "verified", "sha03", true // disagrees — a row
		}
		return "", "", false
	}
	rows := buildDriftRows(cells, lookup)
	if len(rows) != 2 {
		t.Fatalf("want 2 drift rows (agreement on 01 excluded), got %d: %+v", len(rows), rows)
	}
	if rows[0].ID != "assay:assay:derived-board:02" || rows[0].HandSaid != "done" || rows[0].Derived != "unknown (no witness — hand-asserted done at cafef00)" {
		t.Fatalf("row 0 (02) mismatch: %+v", rows[0])
	}
	if rows[1].ID != "assay:assay:derived-board:03" || rows[1].HandSaid != "verified" || rows[1].Derived != "implemented" {
		t.Fatalf("row 1 (03) mismatch: %+v", rows[1])
	}
}

func TestRenderDriftReport(t *testing.T) {
	rows := []driftRow{
		{ID: "assay:assay:derived-board:02", HandSaid: "done", Derived: "unknown (no witness — hand-asserted done at cafef00)", Witness: "no PR carries a trailer; last hand edit cafef00"},
	}
	out := renderDriftReport("medici-finance/assay", time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), rows)
	if !strings.Contains(out, "# Board drift report — medici-finance/assay") {
		t.Fatalf("missing repo header: %s", out)
	}
	if !strings.Contains(out, "| assay:assay:derived-board:02 | done | unknown (no witness — hand-asserted done at cafef00) |") {
		t.Fatalf("missing drift row: %s", out)
	}
	if strings.Count(out, "\n|") == 0 {
		t.Fatalf("want at least one markdown table row line: %s", out)
	}

	empty := renderDriftReport("medici-finance/assay", time.Now(), nil)
	if !strings.Contains(empty, "_none_") {
		t.Fatalf("an empty drift set must still render a readable table, got: %s", empty)
	}
}

// --- git-history mining ------------------------------------------------------

// initHandSaidRepo builds a git repo whose stream README is hand-edited across
// three commits and then flips to the generated shape at a fourth — the exact
// shape brief-04's migration produces. Returns the repo root.
func initHandSaidRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	rel := filepath.Join("docs", "streams", "demo", "README.md")
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(content, msg, date string) {
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", rel)
		runGitEnv(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date},
			"commit", "-q", "-m", msg)
	}
	write(streamREADME("demo", "todo"), "todo", "2026-07-01T10:00:00Z")
	write(streamREADME("demo", "implemented"), "implemented", "2026-07-08T10:00:00Z")
	write(streamREADME("demo", "done"), "hand-said done", "2026-07-15T10:00:00Z")
	generated := "---\nstream: demo\nstatus: active\npriority: P1\nboard: generated\n---\n\n" +
		"<!-- statusgen:briefs:begin -->\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | Do the thing | 0 | M | implemented | — | — |\n" +
		"<!-- statusgen:briefs:end -->\n"
	write(generated, "migrate to generated table", "2026-07-20T10:00:00Z")
	return dir
}

func TestHandSaidBeforeGeneration(t *testing.T) {
	dir := initHandSaidRepo(t)
	state, sha, found := handSaidBeforeGeneration(dir, "demo", "01")
	if !found {
		t.Fatal("want found=true")
	}
	if state != "done" {
		t.Fatalf("want the LAST hand-said state (done, from the 3rd commit) — the generated 4th commit's implemented must not win — got %q", state)
	}
	// The reported sha must be the hand-edited commit, not the generated one.
	out := gitLogHashes(t, dir, rel(t))
	if len(out) < 3 || sha != out[2] {
		t.Fatalf("want the 3rd (hand-said done) commit sha %q, got %q (all: %v)", out[2], sha, out)
	}

	if _, _, found := handSaidBeforeGeneration(dir, "demo", "99"); found {
		t.Fatal("a brief that never appeared in any snapshot must not be found")
	}
	if _, _, found := handSaidBeforeGeneration(dir, "no-such-stream", "01"); found {
		t.Fatal("a stream with no README history must not be found")
	}
}

func rel(t *testing.T) string {
	t.Helper()
	return filepath.Join("docs", "streams", "demo", "README.md")
}

// gitLogHashes returns the oldest-first commit SHAs that touched rel.
func gitLogHashes(t *testing.T, dir, relPath string) []string {
	t.Helper()
	commits, err := streamReadmeCommits(dir, filepath.ToSlash(relPath))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range commits {
		out = append(out, c.SHA)
	}
	return out
}
