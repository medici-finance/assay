package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// unrunStreams copies the UNRUN fixture tree into a temp root and loads it.
func unrunStreams(t *testing.T) []*Stream {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/unrun")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

// withBase substitutes closedAtBase for the duration of a test. grandfathered
// lists the "<stream>/<NN>" briefs that were ALREADY closed at the merge-base;
// everything else counts as closed by this branch.
func withBase(t *testing.T, ok bool, grandfathered ...string) {
	t.Helper()
	prev := closedAtBase
	set := map[string]bool{}
	for _, g := range grandfathered {
		set[g] = true
	}
	closedAtBase = func(string, []*Stream) (map[string]bool, bool) { return set, ok }
	t.Cleanup(func() { closedAtBase = prev })
}

func briefIn(t *testing.T, streams []*Stream, num string) (*Stream, *Brief) {
	t.Helper()
	s, br := findBrief(streams, num)
	if br == nil {
		t.Fatalf("fixture brief %s not found", num)
	}
	return s, br
}

// ---------------------------------------------------------------------------
// The four Task-1 cases
// ---------------------------------------------------------------------------

// TestUnrunRiskBearingUnroutedBlocksDone is the gate's teeth: a brief this
// branch closes over an UNRUN live row with no routed follow-up is a PROBLEM.
func TestUnrunRiskBearingUnroutedBlocksDone(t *testing.T) {
	withBase(t, true) // nothing grandfathered: every closure is this branch's
	streams := unrunStreams(t)
	problems, _ := unrunGateChecks(t.TempDir(), streams)
	if !containsSub(problems, "ur/brief-01") {
		t.Fatalf("ur/01 (unrouted UNRUN live row) must be a PROBLEM; got %v", problems)
	}
	if !containsSub(problems, "#2") {
		t.Errorf("the PROBLEM must name the offending Verify row; got %v", problems)
	}
	s, br := briefIn(t, streams, "01")
	if got := qualityToken(s, br); !strings.HasSuffix(got, "‡") {
		t.Errorf("qualityToken = %q, want a trailing ‡", got)
	}
}

// TestUnrunRoutedFollowUpIsCleanButStillMarked: routing the row to a named
// follow-up clears the PROBLEM (the transition is allowed) but the row is still
// unrun, so the board token stays.
func TestUnrunRoutedFollowUpIsCleanButStillMarked(t *testing.T) {
	withBase(t, true)
	streams := unrunStreams(t)
	problems, notices := unrunGateChecks(t.TempDir(), streams)
	if containsSub(problems, "ur/brief-02") {
		t.Fatalf("a routed UNRUN row must NOT be a PROBLEM; got %v", problems)
	}
	if !containsSub(notices, "ur/brief-02") {
		t.Errorf("a routed UNRUN row must still be NOTICEd; got %v", notices)
	}
	s, br := briefIn(t, streams, "02")
	if got := qualityToken(s, br); !strings.HasSuffix(got, "‡") {
		t.Errorf("a routed-but-unrun row must still render ‡; got %q", got)
	}
}

// TestUnrunNonRiskBearingRowIsSilent: an UNRUN row that is not a live/mutating
// check is neither a PROBLEM nor a board token. The gate is scoped to the row
// that matters, not to every skipped `wc -l`.
func TestUnrunNonRiskBearingRowIsSilent(t *testing.T) {
	withBase(t, true)
	streams := unrunStreams(t)
	problems, notices := unrunGateChecks(t.TempDir(), streams)
	if containsSub(problems, "ur/brief-03") || containsSub(notices, "ur/brief-03") {
		t.Fatalf("a non-risk-bearing UNRUN row must not fire; problems=%v notices=%v", problems, notices)
	}
	s, br := briefIn(t, streams, "03")
	if got := qualityToken(s, br); strings.Contains(got, "‡") {
		t.Errorf("qualityToken = %q, want no ‡", got)
	}
}

// TestUnrunFullyRunBriefUnchanged: a brief whose every Verify row has a
// completed Evidence row is untouched by all of this.
func TestUnrunFullyRunBriefUnchanged(t *testing.T) {
	withBase(t, true)
	streams := unrunStreams(t)
	problems, notices := unrunGateChecks(t.TempDir(), streams)
	if containsSub(problems, "ur/brief-04") || containsSub(notices, "ur/brief-04") {
		t.Fatalf("a fully-run brief must not fire; problems=%v notices=%v", problems, notices)
	}
	s, br := briefIn(t, streams, "04")
	if got := qualityToken(s, br); got != "done" {
		t.Errorf("qualityToken = %q, want %q", got, "done")
	}
	if fs := briefUnrunFindings(s, br); len(fs) != 0 {
		t.Errorf("fully-run brief has %d unrun findings, want 0: %+v", len(fs), fs)
	}
}

// ---------------------------------------------------------------------------
// Derived, not declared — the property the whole state rests on
// ---------------------------------------------------------------------------

// TestUnrunDerivedFromSilentOmission is the anti-self-report test. ur/05 never
// writes "UNRUN", "deferred" or any other marker anywhere in the file: its
// Evidence table simply stops before the live row. If UNRUN were a token an
// author sets, this brief would render as a clean `done`. It must not.
func TestUnrunDerivedFromSilentOmission(t *testing.T) {
	withBase(t, true)
	streams := unrunStreams(t)
	s, br := briefIn(t, streams, "05")

	raw, err := os.ReadFile(s.Dir + "/brief-05-silent-omission.md")
	if err != nil {
		t.Fatal(err)
	}
	// Guard the fixture itself: the test is only meaningful while the file
	// contains no marker outside its own explanatory prose.
	if strings.Count(strings.ToUpper(string(raw)), "UNRUN") != 1 {
		t.Fatal(`fixture drift: brief-05 must mention "UNRUN" exactly once (in its prose), never as a row marker`)
	}

	fs := unrunRiskBearing(briefUnrunFindings(s, br))
	if len(fs) != 1 || fs[0].ID != "2" {
		t.Fatalf("silent omission of the live row must derive as UNRUN; got %+v", fs)
	}
	if fs[0].Reason != "no Evidence row" {
		t.Errorf("reason = %q, want %q", fs[0].Reason, "no Evidence row")
	}
	if fs[0].Routed {
		t.Error("a row with no Evidence row at all can never read as routed — there is nowhere to say where it went")
	}
	problems, _ := unrunGateChecks(t.TempDir(), streams)
	if !containsSub(problems, "ur/brief-05") {
		t.Errorf("silent omission must block the done transition; got %v", problems)
	}
}

// TestUnrunEvidenceRowWithoutDatedRunnerIsUnrun: an Evidence row exists and
// claims exit 0, but names no date and no runner. An unattributed claim is not
// a run — clearing the state requires a positive, attributed artifact.
func TestUnrunEvidenceRowWithoutDatedRunnerIsUnrun(t *testing.T) {
	streams := unrunStreams(t)
	s, br := briefIn(t, streams, "09")
	fs := unrunRiskBearing(briefUnrunFindings(s, br))
	if len(fs) != 1 || fs[0].ID != "2" {
		t.Fatalf("an Evidence row with no dated runner must not count as run; got %+v", fs)
	}
}

// TestUnrunBriefWideRiskTagsEveryRow: an untagged row in a brief whose own risk
// block answers yes is risk-bearing by definition.
func TestUnrunBriefWideRiskTagsEveryRow(t *testing.T) {
	streams := unrunStreams(t)
	s, br := briefIn(t, streams, "08")
	fs := briefUnrunFindings(s, br)
	if len(fs) != 1 {
		t.Fatalf("want 1 unrun row, got %+v", fs)
	}
	if !fs[0].RiskBearing {
		t.Errorf("risk.irreversible=yes must make an untagged unrun row risk-bearing; got %+v", fs[0])
	}
}

// ---------------------------------------------------------------------------
// Transition scoping — the grandfathered backlog
// ---------------------------------------------------------------------------

// TestUnrunGrandfatheredClosureIsNoticeNotProblem: the same offending shape on a
// brief already closed at the merge-base stays visible but does not red-gate —
// the gate is on the transition, not on history.
func TestUnrunGrandfatheredClosureIsNoticeNotProblem(t *testing.T) {
	withBase(t, true, "ur/01", "ur/05", "ur/08", "ur/09")
	streams := unrunStreams(t)
	problems, notices := unrunGateChecks(t.TempDir(), streams)
	if len(problems) != 0 {
		t.Fatalf("a pre-existing closure must not be a PROBLEM; got %v", problems)
	}
	if !containsSub(notices, "grandfathered") {
		t.Errorf("the backlog must still be NOTICEd; got %v", notices)
	}
}

// TestUnrunUnresolvableBaseGrandfathersWithNotice: with no resolvable base
// there is no observable transition, so nothing is gated — and the run says so
// rather than reporting a misleading all-clear.
func TestUnrunUnresolvableBaseGrandfathersWithNotice(t *testing.T) {
	withBase(t, false)
	streams := unrunStreams(t)
	problems, notices := unrunGateChecks(t.TempDir(), streams)
	if len(problems) != 0 {
		t.Fatalf("unresolvable base must grandfather everything; got %v", problems)
	}
	if !containsSub(notices, "running degraded") {
		t.Errorf("the degraded run must name its cause; got %v", notices)
	}
}

// TestUnrunRealMergeBaseResolves exercises the REAL closedAtBase (the git
// merge-base → git show README.md → parseBriefTable path) against a purpose-built
// fixture git repo whose merge-base carries a `done` brief, so the default
// implementation is not left untested behind the fake in withBase.
//
// Re-baselined for the post-dehouse topology (assay#92): the old form pointed at
// the live repo ROOT (`..`) and asserted its merge-base carried `done` briefs —
// true only when statusgen lived in assay-toolkit alongside a rich docs/streams/.
// Now statusgen is dehoused into `assay`, whose root carries no `done`-stage
// briefs at the merge-base (they relocated to assay-toolkit), so that assertion
// is false against reality and the test red-failed. Building the fixture keeps
// the check MEANINGFUL — it still proves closedAtBase resolves a real
// merge-base and grandfathers the briefs `done`/`verified` there — while
// matching the current reality that the statusgen repo root has no such briefs.
func TestUnrunRealMergeBaseResolves(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")

	sdir := filepath.Join(root, "docs", "streams", "grandfathered")
	mustMkdirAll(t, sdir)
	readme := "---\n" +
		"stream: grandfathered\n" +
		"status: active\n" +
		"priority: P1\n" +
		"---\n\n# Grandfathered\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [Closed at base](./brief-01-closed-at-base.md) | 0 | M | done | 2026-08-01 | human:x |\n" +
		"| 02 | [Still open](./brief-02-still-open.md) | 1 | M | todo | — | — |\n"
	writeTemp(t, sdir, "README.md", readme)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "grandfathered: brief-01 done at base")
	// The merge-base of HEAD and origin/main is this commit — closedAtBase reads
	// the README table there and grandfathers its done/verified rows.
	gitRun(t, root, "update-ref", remoteMainRef, "HEAD")

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatalf("fixture root not loadable: %v", err)
	}

	set, ok := closedAtBase(root, streams)
	if !ok {
		t.Fatal("closedAtBase must resolve a base in a fixture with refs/remotes/origin/main")
	}
	if len(set) == 0 {
		t.Fatal("a repo with a done brief at the merge-base must yield a non-empty grandfathered set")
	}
	if !set["grandfathered/01"] {
		t.Errorf("the `done` brief at the base must be grandfathered; got set %v", set)
	}
	if set["grandfathered/02"] {
		t.Errorf("a `todo` brief must NOT be grandfathered (only done/verified are); got set %v", set)
	}
}

// ---------------------------------------------------------------------------
// The `implemented` stage (F-impl-claims-unproven)
// ---------------------------------------------------------------------------

// TestUnrunStaleImplementedFiresOnZeroCoverage: an `implemented` brief with none
// of its Verify rows corroborated, aged past the threshold.
func TestUnrunStaleImplementedFiresOnZeroCoverage(t *testing.T) {
	streams := unrunStreams(t)
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	touch := map[string]time.Time{
		"ur/06": now.Add(-5 * 24 * time.Hour),
		"ur/07": now.Add(-5 * 24 * time.Hour),
	}
	notices := staleImplementedNotices(streams, touch, now)
	if !containsSub(notices, "ur/brief-06") {
		t.Fatalf("a fully-uncorroborated implemented brief must fire; got %v", notices)
	}
	if !containsSub(notices, "0 of 2") {
		t.Errorf("the notice must state the coverage; got %v", notices)
	}
	if containsSub(notices, "ur/brief-07") {
		t.Errorf("a partially-corroborated implemented brief must stay silent; got %v", notices)
	}
}

// TestUnrunStaleImplementedSilentBeforeThreshold: sitting at `implemented`
// awaiting the verify-desk is the normal state — only the AGE makes it a signal.
func TestUnrunStaleImplementedSilentBeforeThreshold(t *testing.T) {
	streams := unrunStreams(t)
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	touch := map[string]time.Time{"ur/06": now.Add(-2 * time.Hour)}
	if notices := staleImplementedNotices(streams, touch, now); len(notices) != 0 {
		t.Errorf("a freshly-implemented brief must not fire; got %v", notices)
	}
}

// TestUnrunStaleImplementedSkipsUnknownAge: no historian entry and no stream
// git touch means the age is unknown — render nothing rather than guess.
func TestUnrunStaleImplementedSkipsUnknownAge(t *testing.T) {
	streams := unrunStreams(t)
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	if notices := staleImplementedNotices(streams, nil, now); len(notices) != 0 {
		t.Errorf("unknown age must not fire; got %v", notices)
	}
}

// TestUnrunStaleImplementedFiresOnRealBoardWhenAged runs the `implemented` leg
// against THIS repo's real streams with the clock wound forward past the
// threshold. It is the "does this state ever actually fire?" check: on today's
// board every implemented brief's fallback clock (the stream's git touch) is
// fresh, so the alarm is legitimately silent — this proves the silence is the
// threshold doing its job and not a check that can never speak.
func TestUnrunStaleImplementedFiresOnRealBoardWhenAged(t *testing.T) {
	streams, _, err := loadStreams("..")
	if err != nil {
		t.Skipf("repo root not loadable: %v", err)
	}
	for _, s := range streams {
		s.LastTouch = gitLastTouch("..", "docs/streams/"+s.Name)
	}
	uncovered := 0
	for _, s := range streams {
		for i := range s.Briefs {
			br := &s.Briefs[i]
			if br.Status != "implemented" {
				continue
			}
			art, ok := loadBriefArtifacts(s, br.Num)
			if !ok || len(parseVerifyItems(art.Verify)) == 0 {
				continue
			}
			if len(unrunFindings(art.Risk, art.Verify, art.Evidence)) == len(parseVerifyItems(art.Verify)) {
				uncovered++
			}
		}
	}
	if uncovered == 0 {
		t.Skip("no fully-uncorroborated implemented brief on the board right now")
	}
	now := time.Now().UTC()
	if got := staleImplementedNotices(streams, nil, now); len(got) != 0 {
		t.Logf("briefs past threshold at the current clock (pre-existing board condition, not a test defect): %v", got)
	}
	aged := now.Add(400 * 24 * time.Hour)
	got := staleImplementedNotices(streams, nil, aged)
	if len(got) != uncovered {
		t.Errorf("wound forward, want %d stale-implemented notices, got %d: %v", uncovered, len(got), got)
	}
}

// TestUnrunDoneGateIgnoresImplementedRows keeps the two stages' severities
// apart: an uncorroborated `implemented` brief is never a PROBLEM.
func TestUnrunDoneGateIgnoresImplementedRows(t *testing.T) {
	withBase(t, true)
	streams := unrunStreams(t)
	problems, notices := unrunGateChecks(t.TempDir(), streams)
	if containsSub(problems, "ur/brief-06") || containsSub(notices, "ur/brief-06") {
		t.Errorf("the done gate must not touch implemented rows; problems=%v notices=%v", problems, notices)
	}
}

// ---------------------------------------------------------------------------
// Parsing units
// ---------------------------------------------------------------------------

func TestUnrunParseVerifyItemsHandlesEscapedPipes(t *testing.T) {
	section := "| # | Command | Expect |\n" +
		"|---|---------|--------|\n" +
		"| 1 | `ls x \\| grep -c y` | 3 |\n"
	items := parseVerifyItems(section)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d: %+v", len(items), items)
	}
	if items[0].ID != "1" || !strings.Contains(items[0].Text, "grep") {
		t.Errorf("escaped pipe shredded the row: %+v", items[0])
	}
}

func TestUnrunParseVerifyItemsFallsBackToOrdinal(t *testing.T) {
	section := "| Command | Expect |\n|---|---|\n| `a` | ok |\n| `b` | ok |\n"
	items := parseVerifyItems(section)
	if len(items) != 2 || items[0].ID != "1" || items[1].ID != "2" {
		t.Errorf("a table with no # column must number rows by ordinal; got %+v", items)
	}
}

// TestUnrunEvidenceTablesUnion: an Evidence section may hold several tables (an
// implementer pass, then a re-verify at a later SHA). A row run by ANY of them
// counts as run.
func TestUnrunEvidenceTablesUnion(t *testing.T) {
	verify := "| # | Command | Expect |\n|---|---|---|\n| 1 | live migrate | applied |\n"
	evidence := "First pass:\n\n" +
		"| # | Command | Exit | Date | Runner |\n|---|---|---|---|---|\n" +
		"| 1 | live migrate | UNRUN | 2026-07-08 | a |\n\n" +
		"Re-verify:\n\n" +
		"| # | Command | Exit | Date | Runner |\n|---|---|---|---|---|\n" +
		"| 1 | live migrate | 0 | 2026-07-16 | b |\n"
	if fs := unrunFindings(nil, verify, evidence); len(fs) != 0 {
		t.Errorf("a later completed pass must clear the row; got %+v", fs)
	}
}

// TestUnrunRoutingNeedsBothKeywordAndRef: a bare issue number in an output
// excerpt must not read as routing, and a bare "follow-up" naming nothing must
// not either. Both are required, and both are things the author must ADD.
func TestUnrunRoutingNeedsBothKeywordAndRef(t *testing.T) {
	verify := "| # | Command | Expect |\n|---|---|---|\n| 1 | live migrate | applied |\n"
	cases := []struct {
		name       string
		cell       string
		wantRouted bool
	}{
		{"bare ref in output", "observed on #123/#124 (read-only)", false},
		{"keyword, no ref", "deferred, follow-up to come", false},
		{"keyword and ref", "UNRUN — routed to follow-up #123", true},
		{"keyword and typed brief id", "UNRUN — tracked by example-app/12", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			evidence := "| # | Command | Exit | Date | Runner |\n|---|---|---|---|---|\n" +
				"| 1 | live migrate | UNRUN | " + c.cell + " | 2026-07-08 | a |\n"
			fs := unrunFindings(nil, verify, evidence)
			if len(fs) != 1 {
				t.Fatalf("row must still be unrun; got %+v", fs)
			}
			if fs[0].Routed != c.wantRouted {
				t.Errorf("Routed = %v, want %v (cell %q)", fs[0].Routed, c.wantRouted, c.cell)
			}
		})
	}
}

// TestUnrunMarkerIgnoresVerbatimCodeSpanContent: the unrun
// marker regex used to scan the WHOLE raw Evidence row, so verbatim
// command/output content that happens to contain a marker word tripped it even
// though the row was genuinely run, dated, and passing. A `grep -n 'DO NOT
// RUN' <file>` command cell ("NOT RUN") and a tool's own `0 failed, 0
// skipped` counters ("skipped") must NOT read as a run-status declaration —
// both are backtick-quoted verbatim content, not prose the author wrote about
// the row's own status.
func TestUnrunMarkerIgnoresVerbatimCodeSpanContent(t *testing.T) {
	verify := "| # | Command | Expect |\n|---|---|---|\n| 1 | `go test ./...` | exit 0 |\n"
	cases := []struct {
		name     string
		evidence string
	}{
		{
			"verbatim grep command for a retired banner contains 'NOT RUN'",
			"| # | Command | Exit | Output | Date | Runner |\n|---|---|---|---|---|---|\n" +
				"| 1 | `grep -n 'DO NOT RUN' legacy.go` | 0 | no matches (banner retired) | 2026-08-08 | tester |\n",
		},
		{
			"verbatim tool counters contain 'skipped'",
			"| # | Command | Exit | Output | Date | Runner |\n|---|---|---|---|---|---|\n" +
				"| 1 | `go test ./...` | 0 | `0 failed, 0 skipped` | 2026-08-08 | tester |\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if fs := unrunFindings(nil, verify, c.evidence); len(fs) != 0 {
				t.Errorf("verbatim code-span content must not read as an unrun declaration; got %+v", fs)
			}
		})
	}
}

// TestUnrunMarkerStillMatchesProseStatusDeclaration: the fix must not
// blind the marker to a REAL "I did not run this" declaration — the marker
// still has to fire when a verifier writes it as plain, unquoted prose (the
// convention every existing fixture uses), whether in the Exit cell or
// elsewhere in the row.
func TestUnrunMarkerStillMatchesProseStatusDeclaration(t *testing.T) {
	verify := "| # | Command | Expect |\n|---|---|---|\n| 1 | `go test ./...` | exit 0 |\n"
	cases := []struct {
		name     string
		evidence string
	}{
		{
			"UNRUN in the Exit cell",
			"| # | Command | Exit | Output | Date | Runner |\n|---|---|---|---|---|---|\n" +
				"| 1 | `go test ./...` | UNRUN | not attempted | 2026-08-08 | tester |\n",
		},
		{
			"deferred as unquoted prose in the Output cell",
			"| # | Command | Exit | Output | Date | Runner |\n|---|---|---|---|---|---|\n" +
				"| 1 | `go test ./...` | - | deferred, no staging environment | 2026-08-08 | tester |\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fs := unrunFindings(nil, verify, c.evidence)
			if len(fs) != 1 {
				t.Errorf("a genuine prose status declaration must still derive as unrun; got %+v", fs)
			}
		})
	}
}

// TestUnrunNoVerifyTableIsSilent: a brief with no Verify table claims nothing
// this check can derive from. Its missing table is verifySectionProblems'
// business, not this one's.
func TestUnrunNoVerifyTableIsSilent(t *testing.T) {
	if fs := unrunFindings(nil, "", "anything"); fs != nil {
		t.Errorf("no Verify table must derive nothing; got %+v", fs)
	}
}

// TestUnrunRiskTagWordBoundaries pins the tagging against the obvious
// substring trap: "deliver" contains "live".
func TestUnrunRiskTagWordBoundaries(t *testing.T) {
	if riskBearingRowRe.MatchString("deliverable check for the whole delivery") {
		t.Error(`"deliver" must not read as the "live" tag`)
	}
	if !riskBearingRowRe.MatchString("live end-to-end run") {
		t.Error(`"live" must read as the tag`)
	}
}

// containsSub reports whether any element of xs contains sub.
func containsSub(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
