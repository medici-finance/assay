package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// archivestreamedge_test.go — regression for archived-stream edge resolution.
//
// A stream that is finished is moved off the active board into docs/archive/<stream>/
// (streamarchive.go; checks.go rejects a `done` stream still under docs/streams/).
// loadStreams walks only docs/streams/, so before this fix an archived stream was
// absent from the resolution universe entirely — and any ACTIVE brief that still
// `depends:`/`unblocks:` a done brief, or any OPEN finding that still `affects:` the
// completed stream, had its previously-valid edge converted into a hard
// `references unknown stream` PROBLEM the moment the referenced stream was archived.
// Archived work is real work: a brief may depend on a completed brief; a finding may
// concern a completed stream. So docs/archive/<stream> must resolve for an edge
// EXACTLY like docs/streams/<stream>.
//
// The boundary is the other half of the fix: a stream present under NEITHER
// docs/streams/ nor docs/archive/ is genuinely unknown and must STILL PROBLEM — the
// fix adds known targets, it never blanket-suppresses unknown-stream detection.

// writeArchiveEdgeFixture materializes a tree with:
//   - an ACTIVE stream `active-work` whose briefs carry (a) a depends: edge into an
//     ARCHIVED stream, and (b) a CONTROL depends: edge into a genuinely-unknown
//     stream;
//   - an ARCHIVED stream `completed-core` under docs/archive/ with brief 07 in its
//     README table (and its brief file, so the run()-level link check is clean).
func writeArchiveEdgeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// active-work: active, serves example-app.
	awDir := filepath.Join(root, "docs", "streams", "active-work")
	if err := os.MkdirAll(awDir, 0o755); err != nil {
		t.Fatal(err)
	}
	awReadme := "---\n" +
		"stream: active-work\n" +
		"status: active\n" +
		"priority: P1\n" +
		"serves: example-app\n" +
		"---\n\n# Active Work\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [Base](./brief-01-base.md) | 0 | M | todo | — | — |\n" +
		"| 02 | [Depends on archived](./brief-02-dep-archived.md) | 1 | M | todo | — | — |\n" +
		"| 03 | [Depends on nowhere](./brief-03-dep-nowhere.md) | 1 | M | todo | — | — |\n"
	writeFixtureFile(t, filepath.Join(awDir, "README.md"), awReadme)
	writeFixtureFile(t, filepath.Join(awDir, "brief-01-base.md"),
		pausedDepBrief("active-work/01", "Base", 0, nil))
	// brief-02 depends on a brief in an ARCHIVED stream — the edge under repair.
	writeFixtureFile(t, filepath.Join(awDir, "brief-02-dep-archived.md"),
		pausedDepBrief("active-work/02", "Depends on archived", 1, []string{"completed-core/07"}))
	// brief-03 depends on a stream that exists NOWHERE — the control (still unknown).
	writeFixtureFile(t, filepath.Join(awDir, "brief-03-dep-nowhere.md"),
		pausedDepBrief("active-work/03", "Depends on nowhere", 1, []string{"ghost-nowhere/01"}))

	// completed-core: ARCHIVED (moved to docs/archive/), status done, still carrying
	// brief 07 in its README table so a per-brief edge (<stream>/07) resolves.
	ccDir := filepath.Join(root, "docs", "archive", "completed-core")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ccReadme := "---\n" +
		"stream: completed-core\n" +
		"status: done\n" +
		"priority: P1\n" +
		"serves: example-app\n" +
		"---\n\n# Completed Core\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 07 | [Legacy](./brief-07-legacy.md) | 0 | M | done | 2026-01-01 fixture | 2026-01-01 fixture |\n"
	writeFixtureFile(t, filepath.Join(ccDir, "README.md"), ccReadme)
	writeFixtureFile(t, filepath.Join(ccDir, "brief-07-legacy.md"),
		pausedDepBrief("completed-core/07", "Legacy", 0, nil))

	return root
}

// affectsUnknownStream reports whether some problem flags `stream` as an unknown
// Affects target for a finding.
func affectsUnknownStream(problems []string, stream string) bool {
	for _, p := range problems {
		if strings.Contains(p, "Affects references unknown stream") && strings.Contains(p, `"`+stream+`"`) {
			return true
		}
	}
	return false
}

// TestArchivedStreamResolvesAsKnownDependsTarget is the depends:/unblocks: half of
// the fix. An ACTIVE brief's edge into an ARCHIVED stream must resolve; a
// genuinely-unknown stream must still be flagged.
//
// Fail-first (clause 9, committed reproduction): the pre-fix universe (`active`
// alone, without the archived streams) reproduces the false `unknown stream`
// positive the fix removes — the reviewer re-runs this test and observes the
// `old` assertion holding. The fixed universe (`edge`, active + archived) clears it.
func TestArchivedStreamResolvesAsKnownDependsTarget(t *testing.T) {
	root := writeArchiveEdgeFixture(t)
	active, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := loadArchivedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	edge := edgeResolutionUniverse(active, archived)

	// THE FIX: resolve edges against active + archived.
	fixed, _ := checkBriefFiles(active, edge)
	if hasUnknownStream(fixed, "completed-core") {
		t.Errorf("depends: into an archived stream was falsely flagged unknown: %v", filterUnknown(fixed))
	}
	// BOUNDARY: the genuinely-unknown stream is still caught — the fix did not
	// blanket-suppress unknown-stream detection.
	if !hasUnknownStream(fixed, "ghost-nowhere") {
		t.Errorf("a genuinely-unknown stream must still flag unknown-stream: %v", fixed)
	}

	// PRE-FIX REPRODUCTION: without the archived streams in the universe the edge
	// into the archived stream falsely flags unknown — the exact regression the
	// fix removes. If this stops reproducing, the test no longer proves anything.
	old, _ := checkBriefFiles(active, active)
	if !hasUnknownStream(old, "completed-core") {
		t.Fatalf("pre-fix reproduction failed: an active-only universe should flag the archived-stream edge unknown — the red is no longer observable: %v", old)
	}
}

// TestArchivedStreamResolvesAsKnownAffectsTarget is the finding-affects half of the
// fix, with the same fail-first structure. An OPEN finding's affects: into an
// archived stream must resolve; a genuinely-unknown stream must still be flagged.
func TestArchivedStreamResolvesAsKnownAffectsTarget(t *testing.T) {
	root := writeArchiveEdgeFixture(t)
	active, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := loadArchivedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	edge := edgeResolutionUniverse(active, archived)

	findings := []Finding{
		{ID: "F-01", Affects: []string{"completed-core/brief-07"}, Resolved: false}, // into archived
		{ID: "F-02", Affects: []string{"ghost-nowhere/brief-01"}, Resolved: false},  // control: nowhere
	}

	// THE FIX: findings affects: resolves against active + archived.
	fixed, _ := checkScoped(active, edge, findings)
	if affectsUnknownStream(fixed, "completed-core") {
		t.Errorf("an open finding affecting an archived stream was falsely flagged unknown: %v", fixed)
	}
	// BOUNDARY: the genuinely-unknown affects target is still caught.
	if !affectsUnknownStream(fixed, "ghost-nowhere") {
		t.Errorf("a genuinely-unknown affects target must still flag unknown-stream: %v", fixed)
	}

	// PRE-FIX REPRODUCTION: active-only universe falsely flags the archived affects.
	old, _ := checkScoped(active, active, findings)
	if !affectsUnknownStream(old, "completed-core") {
		t.Fatalf("pre-fix reproduction failed: an active-only universe should flag the archived affects unknown — the red is no longer observable: %v", old)
	}
}

// TestLoadArchivedStreams pins the loader contract: it reads docs/archive/<stream>
// README tables (name, briefs, Archived flag), returns (nil, nil) for a root with
// no docs/archive/, and never contributes to the active stream set.
func TestLoadArchivedStreams(t *testing.T) {
	root := writeArchiveEdgeFixture(t)

	archived, err := loadArchivedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 {
		t.Fatalf("want 1 archived stream, got %d: %+v", len(archived), archived)
	}
	cc := archived[0]
	if cc.Name != "completed-core" {
		t.Errorf("archived stream name = %q, want completed-core", cc.Name)
	}
	if !cc.Archived {
		t.Errorf("archived stream must carry Archived=true")
	}
	if len(cc.Briefs) != 1 || cc.Briefs[0].Num != "07" {
		t.Errorf("archived stream README brief table not loaded: %+v", cc.Briefs)
	}

	// The active load path must NOT pick up the archived stream.
	active, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range active {
		if s.Name == "completed-core" {
			t.Fatalf("archived stream leaked into the active stream set")
		}
	}

	// A root with no docs/archive/ is a legitimate empty, not an error.
	bare := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bare, "docs", "streams"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := loadArchivedStreams(bare)
	if err != nil {
		t.Fatalf("absent docs/archive/ must not error: %v", err)
	}
	if got != nil {
		t.Errorf("absent docs/archive/ must return nil, got %+v", got)
	}
}

// TestEdgeResolutionUniverseActiveWinsCollision proves an active stream shadows a
// same-named archived stream (a stream mid-transition may momentarily exist under
// both trees) and that the active `streams` slice is never mutated.
func TestEdgeResolutionUniverseActiveWinsCollision(t *testing.T) {
	act := &Stream{Name: "dup", Briefs: []Brief{{Num: "01"}}}
	arch := &Stream{Name: "dup", Archived: true, Briefs: []Brief{{Num: "99"}}}
	other := &Stream{Name: "other", Archived: true}
	active := []*Stream{act}

	universe := edgeResolutionUniverse(active, []*Stream{arch, other})
	if len(universe) != 2 {
		t.Fatalf("want 2 (active dup wins collision + archived other), got %d", len(universe))
	}
	if len(active) != 1 || active[0] != act {
		t.Fatalf("active slice was mutated: %+v", active)
	}
	byName := map[string]*Stream{}
	for _, s := range universe {
		byName[s.Name] = s
	}
	if byName["dup"] != act {
		t.Errorf("active copy must win a name collision, got archived")
	}
	if byName["other"] != other {
		t.Errorf("archived non-colliding stream must be present")
	}
}

// TestRunLintResolvesArchivedEdges is the end-to-end guard: `statusgen --lint` over
// a tree with active edges into an archived stream must NOT emit an unknown-stream
// PROBLEM for the archived stream, while STILL emitting one for the genuinely-unknown
// control. This exercises the run() wiring (loadArchivedStreams → edgeStreams), which
// the unit tests above stop short of.
//
// Fail-first (clause 9, committed mutation): reverting the run() wiring — passing
// `streams` instead of `edgeStreams` as the allStreams argument of checkScoped and
// checkBriefFiles in main.go — reddens THIS test (the archived-stream assertion
// fires); the control assertion is what proves the wiring did not blanket-suppress.
func TestRunLintResolvesArchivedEdges(t *testing.T) {
	root := writeArchiveEdgeFixture(t)
	var code int
	stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })

	if strings.Contains(stderr, "unknown stream") && strings.Contains(stderr, `"completed-core"`) {
		t.Errorf("--lint flagged an active edge into an archived stream as unknown:\n%s", stderr)
	}
	// The control edge into a nowhere stream must still be a PROBLEM (so exit 1).
	if !(strings.Contains(stderr, "unknown stream") && strings.Contains(stderr, `"ghost-nowhere"`)) {
		t.Errorf("--lint must still flag the genuinely-unknown control edge:\n%s", stderr)
	}
	if code == 0 {
		t.Errorf("the control edge should keep --lint red (exit 1), got exit 0")
	}
}
