package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// siblingmerge_test.go — tests for the sibling-merge-unreconciled phantom
// class. Every fixture uses
// example-org/example-sibling / example-stream: the deliverable repo is
// PUBLIC and no fixture may name a private repo,
// stream or issue.

// siblingExampleRegistry is the graph-repos.yaml every test in this file
// shares: "ex" is a published sibling alias, "hidden" is unpublished (a
// withheld repo, the could-not-check shape).
const siblingExampleRegistry = "schema: graph-repos-v1\ncell: test\nrepos:\n" +
	"  ex:     {cell: test, repo: example-org/example-sibling}\n" +
	"  hidden: {cell: test, repo: null, unpublished: true}\n"

// writeSiblingRegistry writes a docs/streams/graph-repos.yaml under a fresh
// temp root and returns the root. siblingMergeCheck takes []*Stream directly
// (like boardHonestyNotices), so no docs/streams/<name> tree is needed for
// most of these tests — only the registry file loadGraphRepos(root) reads.
func writeSiblingRegistry(t *testing.T, registry string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if registry != "" {
		if err := os.WriteFile(filepath.Join(dir, "graph-repos.yaml"), []byte(registry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// newSiblingGitRepo scaffolds a REAL git repo (git init, real commits) to
// stand in for a sibling checkout — the Task's own instruction ("Tests build
// their sibling as a real temporary git repo").
func newSiblingGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInit(t, dir, "Sibling Bot", "sibling-bot@example.com")
	return dir
}

// commitEmpty makes an --allow-empty commit carrying exactly the given
// message (subject + body together, as a real commit's %B would read) and
// returns nothing — the tests read history back through readSiblingCommits
// itself rather than the sha, keeping the fixtures close to what a genuine
// `git log` would hand the detector.
func commitEmpty(t *testing.T, dir, message string) {
	t.Helper()
	runGit(t, dir, "commit", "--allow-empty", "-m", message)
}

// siblingBriefTree builds a FULL, on-disk board root (registry + one active
// stream + one brief-v1 file) for the tests that must drive the real `run()`/
// `runPhantoms` pipeline rather than call siblingMergeCheck directly.
// extraFrontmatter is inserted into the brief's frontmatter block (e.g.
// "deliverable_repo: ex\n").
func siblingBriefTree(t *testing.T, extraFrontmatter string) string {
	t.Helper()
	root := t.TempDir()
	streamsDir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(streamsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamsDir, "graph-repos.yaml"), []byte(siblingExampleRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	streamDir := filepath.Join(streamsDir, "example-stream")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "---\nstream: example-stream\nstatus: active\npriority: P1\ntrack: platform\n---\n\n" +
		"# Example Stream\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 02 | [Second](brief-02-x.md) | 0 | S | todo | — | — |\n"
	if err := os.WriteFile(filepath.Join(streamDir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := "---\nbrief: example-stream/02\ntitle: A fixture brief for the sibling-merge detector\n" +
		"wave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\n" +
		"schema: brief-v1\nauthored: 2026-09-25 by fixture\nsources: [\"fixture: sibling-merge test\"]\n" +
		extraFrontmatter + "---\n\n# Brief 02 — fixture\n\n" +
		"## Context\nfiles: statusgen/fixture.go\n\n## Verify\n| # | Command | Expect |\n|---|---------|--------|\n" +
		"| 1 | `true` | exit 0 |\n"
	if err := os.WriteFile(filepath.Join(streamDir, "brief-02-x.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// withSiblingRootOverride sets the package-level --sibling-root flag
// accumulator for the duration of the test, restoring it on cleanup — the
// same var main.go's --sibling-root flag.Var call feeds, and the only way
// run()'s (~90 call sites, unchanged signature) pipeline learns of an
// override.
func withSiblingRootOverride(t *testing.T, spec string) {
	t.Helper()
	orig := siblingRootFlagValues
	siblingRootFlagValues = siblingRootFlags{spec}
	t.Cleanup(func() { siblingRootFlagValues = orig })
}

// --- Verify row 1 -----------------------------------------------------------

func TestSiblingMergeSubjectID(t *testing.T) {
	root := writeSiblingRegistry(t, siblingExampleRegistry)
	sib := newSiblingGitRepo(t)
	commitEmpty(t, sib, "feat(overrides): the thing (example-stream/19) (#129)")

	s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
		{Num: "19", Status: "todo", DeliverableRepo: "ex"},
	}}
	overrides := map[string]string{"example-org/example-sibling": sib}
	notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)

	if failed != 1 {
		t.Fatalf("want 1 checked-failed row, got %d; notices=%v", failed, notices)
	}
	if cnc != 0 {
		t.Fatalf("want 0 could-not-check, got %d; notices=%v", cnc, notices)
	}
	joined := strings.Join(notices, "\n")
	for _, want := range []string{phantomSiblingMergeUnreconciled, "example-stream/19", "example-org/example-sibling"} {
		if !strings.Contains(joined, want) {
			t.Errorf("NOTICE must contain %q; got:\n%s", want, joined)
		}
	}
	if s.Briefs[0].MergedInSibling != "example-org/example-sibling" {
		t.Errorf("MergedInSibling = %q, want example-org/example-sibling", s.Briefs[0].MergedInSibling)
	}
}

// --- Verify row 2 -----------------------------------------------------------

func TestSiblingMergeBodyOnlyID(t *testing.T) {
	root := writeSiblingRegistry(t, siblingExampleRegistry)
	sib := newSiblingGitRepo(t)
	// The real shape this class exists for (fact 3): the id names the brief
	// only in the commit BODY (a prose line and a Brief: trailer), never the
	// subject.
	commitEmpty(t, sib, "feat(terminal): container driver — exec into the container (#137)\n\n"+
		"This lands the container driver end to end.\n\nBrief: example-stream/13\n")

	s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
		{Num: "13", Status: "todo", DeliverableRepo: "ex"},
	}}
	overrides := map[string]string{"example-org/example-sibling": sib}
	notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
	if failed != 1 || cnc != 0 {
		t.Fatalf("a body-only id must still be checked-failed; failed=%d cnc=%d notices=%v", failed, cnc, notices)
	}
}

// --- Verify row 3 -----------------------------------------------------------

func TestSiblingMergeCouldNotCheck(t *testing.T) {
	assertCNCOnly := func(t *testing.T, notices []string, failed, cnc int) {
		t.Helper()
		if failed != 0 {
			t.Fatalf("want 0 checked-failed, got %d; notices=%v", failed, notices)
		}
		if cnc != 1 {
			t.Fatalf("want exactly 1 could-not-check, got %d; notices=%v", cnc, notices)
		}
		if len(notices) != 1 {
			t.Fatalf("want exactly one line per sibling per run, got %d: %v", len(notices), notices)
		}
		if !strings.Contains(notices[0], "could-not-check") {
			t.Errorf("want a could-not-check NOTICE; got: %s", notices[0])
		}
		for _, n := range notices {
			if strings.Contains(n, "no phantom") {
				t.Errorf("a could-not-check must never be rounded to a clean conclusion; got: %s", n)
			}
		}
	}

	t.Run("absent root", func(t *testing.T) {
		root := writeSiblingRegistry(t, siblingExampleRegistry)
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
			{Num: "01", Status: "todo", DeliverableRepo: "ex"},
		}}
		overrides := map[string]string{"example-org/example-sibling": filepath.Join(t.TempDir(), "does-not-exist")}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		assertCNCOnly(t, notices, failed, cnc)
	})

	t.Run("shallow clone", func(t *testing.T) {
		root := writeSiblingRegistry(t, siblingExampleRegistry)
		full := newSiblingGitRepo(t)
		commitEmpty(t, full, "feat: x (example-stream/02) (#1)")
		shallow := filepath.Join(t.TempDir(), "shallow")
		cmd := exec.Command("git", "clone", "--depth", "1", "file://"+full, shallow)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git clone --depth 1: %v\n%s", err, out)
		}
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
			{Num: "02", Status: "todo", DeliverableRepo: "ex"},
		}}
		overrides := map[string]string{"example-org/example-sibling": shallow}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		assertCNCOnly(t, notices, failed, cnc)
	})

	t.Run("unpublished alias", func(t *testing.T) {
		root := writeSiblingRegistry(t, siblingExampleRegistry)
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
			{Num: "03", Status: "todo", DeliverableRepo: "hidden"},
		}}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, nil)
		assertCNCOnly(t, notices, failed, cnc)
		if !strings.Contains(notices[0], "unpublished") {
			t.Errorf("want the reason to name 'unpublished'; got: %s", notices[0])
		}
	})
}

// --- Verify row 4 -----------------------------------------------------------

func TestSiblingMergeNeverProblem(t *testing.T) {
	root := siblingBriefTree(t, "deliverable_repo: ex\n")
	sib := newSiblingGitRepo(t)
	commitEmpty(t, sib, "feat: ship it (example-stream/02) (#42)")
	withSiblingRootOverride(t, "example-org/example-sibling="+sib)

	var code int
	stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })

	if code != 0 {
		t.Errorf("lint exited %d, want 0 — sibling-merge-unreconciled must never change the --lint exit code; stderr:\n%s", code, stderr)
	}
	if strings.Contains(stderr, "PROBLEM:") {
		t.Errorf("sibling-merge-unreconciled must never be a PROBLEM; stderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, phantomSiblingMergeUnreconciled) {
		t.Errorf("expected the finding to surface as a NOTICE; stderr:\n%s", stderr)
	}

	if code := run(root, "write", nil, nil, ""); code != 0 {
		t.Fatalf("write exited %d, want 0 — the finding must never block the STATUS.md write", code)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); err != nil {
		t.Errorf("STATUS.md was not written: %v", err)
	}
}

// --- Verify row 5 -----------------------------------------------------------

func TestSiblingMergeHeldFromNextUp(t *testing.T) {
	neighbour := Brief{Num: "02", Title: "neighbour", Wave: 0, Status: "todo"}

	plainHeld := Brief{Num: "01", Title: "held", Wave: 0, Status: "todo"}
	sPlain := mkStream("example-stream", "active", "P1", plainHeld, neighbour)
	sPlain.Track = "platform"
	nuPlain := nextUp([]*Stream{sPlain}, ClaimView{}, nil)

	held := Brief{Num: "01", Title: "held", Wave: 0, Status: "todo", MergedInSibling: "example-org/example-sibling"}
	sHeld := mkStream("example-stream", "active", "P1", held, neighbour)
	sHeld.Track = "platform"
	nuHeld := nextUp([]*Stream{sHeld}, ClaimView{}, nil)

	for _, p := range nuHeld.Picks {
		if p.Brief.Num == "01" {
			t.Errorf("a checked-failed row must be absent from Next-up picks; picks: %+v", nuHeld.Picks)
		}
	}
	if got := nuHeld.MergedElsewhere["example-stream/01"]; got != "example-org/example-sibling" {
		t.Errorf("MergedElsewhere[example-stream/01] = %q, want example-org/example-sibling; map: %v", got, nuHeld.MergedElsewhere)
	}

	var plainScore, heldScore int
	var sawPlain, sawHeld bool
	for _, p := range nuPlain.Picks {
		if p.Brief.Num == "02" {
			plainScore, sawPlain = p.Score, true
		}
	}
	for _, p := range nuHeld.Picks {
		if p.Brief.Num == "02" {
			heldScore, sawHeld = p.Score, true
		}
	}
	if !sawPlain || !sawHeld {
		t.Fatalf("the neighbour row must be a pick in both runs; sawPlain=%v sawHeld=%v", sawPlain, sawHeld)
	}
	if plainScore != heldScore {
		t.Errorf("the neighbour's score must be byte-identical whether or not the sibling merge holds row 01; plain=%d held=%d", plainScore, heldScore)
	}

	// Rendered in the named STATUS.md section.
	out := emit([]*Stream{sHeld}, nil, nuHeld, nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "Merged in a sibling repo — check before dispatch (1)") {
		t.Errorf("STATUS.md must render the named section; got:\n%s", out)
	}
	if !strings.Contains(out, "example-stream/01") || !strings.Contains(out, "example-org/example-sibling") {
		t.Errorf("the rendered section must name the held row and its sibling; got:\n%s", out)
	}
}

// --- Verify row 6 -----------------------------------------------------------

func TestSiblingMergeInProgressNotHeld(t *testing.T) {
	root := writeSiblingRegistry(t, siblingExampleRegistry)
	sib := newSiblingGitRepo(t)
	commitEmpty(t, sib, "feat: x (example-stream/05) (#7)")

	s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
		{Num: "05", Status: "in-progress", DeliverableRepo: "ex"},
	}}
	overrides := map[string]string{"example-org/example-sibling": sib}
	notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
	if failed != 1 {
		t.Fatalf("an in-progress row must still be SURFACED (item 6), got failed=%d notices=%v", failed, notices)
	}
	if cnc != 0 {
		t.Fatalf("want 0 could-not-check, got %d", cnc)
	}
	if s.Briefs[0].MergedInSibling != "" {
		t.Errorf("an in-progress row must NEVER be excluded — MergedInSibling must stay empty; got %q", s.Briefs[0].MergedInSibling)
	}

	nu := nextUp([]*Stream{s}, ClaimView{}, nil)
	found := false
	for _, p := range nu.Picks {
		if p.Brief.Num == "05" {
			found = true
		}
	}
	if !found {
		t.Errorf("the in-progress row must remain a Next-up pick despite the finding")
	}
}

// --- Verify row 7 -----------------------------------------------------------

func TestPhantomsVerbExitCodes(t *testing.T) {
	t.Run("checked-failed exits 1", func(t *testing.T) {
		root := siblingBriefTree(t, "deliverable_repo: ex\n")
		sib := newSiblingGitRepo(t)
		commitEmpty(t, sib, "feat: ship (example-stream/02) (#42)")
		var out, errOut bytes.Buffer
		code := runPhantoms([]string{
			"--root", root, "--class", phantomSiblingMergeUnreconciled,
			"--sibling-root", "example-org/example-sibling=" + sib,
		}, &out, &errOut)
		if code != 1 {
			t.Fatalf("want exit 1, got %d; stdout=%s stderr=%s", code, out.String(), errOut.String())
		}
	})

	t.Run("could-not-check-only exits 2", func(t *testing.T) {
		root := siblingBriefTree(t, "deliverable_repo: ex\n")
		var out, errOut bytes.Buffer
		code := runPhantoms([]string{"--root", root, "--class", phantomSiblingMergeUnreconciled}, &out, &errOut)
		if code != 2 {
			t.Fatalf("want exit 2 (no override, no default-path checkout), got %d; stdout=%s stderr=%s", code, out.String(), errOut.String())
		}
	})

	t.Run("clean exits 0", func(t *testing.T) {
		root := siblingBriefTree(t, "") // no deliverable_repo/homed-in declared at all
		var out, errOut bytes.Buffer
		code := runPhantoms([]string{"--root", root, "--class", phantomSiblingMergeUnreconciled}, &out, &errOut)
		if code != 0 {
			t.Fatalf("want exit 0, got %d; stdout=%s stderr=%s", code, out.String(), errOut.String())
		}
	})
}

// --- Verify row 8 -----------------------------------------------------------

func TestSiblingMergeUnknownStreamIgnored(t *testing.T) {
	t.Run("an id naming no row on this board produces no finding", func(t *testing.T) {
		root := writeSiblingRegistry(t, siblingExampleRegistry)
		sib := newSiblingGitRepo(t)
		commitEmpty(t, sib, "chore: unrelated cleanup (foo/12) (#9)")
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
			{Num: "02", Status: "todo", DeliverableRepo: "ex"},
		}}
		overrides := map[string]string{"example-org/example-sibling": sib}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		if failed != 0 || cnc != 0 {
			t.Fatalf("an id naming no row on this board must produce no finding; failed=%d cnc=%d notices=%v", failed, cnc, notices)
		}
	})

	t.Run("a basename matching no registry entry produces no finding", func(t *testing.T) {
		root := writeSiblingRegistry(t, siblingExampleRegistry)
		dir := t.TempDir()
		body := "---\nbrief: example-stream/03\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\n" +
			"gate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\n" +
			"schema: brief-v1\nauthored: 2026-09-25 by fixture\n" +
			"sources: [\"../not-a-registered-repo/thing.go\"]\n---\n\n# t\n"
		if err := os.WriteFile(filepath.Join(dir, "brief-03-x.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		s := &Stream{Name: "example-stream", Status: "active", Dir: dir, Briefs: []Brief{
			{Num: "03", Status: "todo"},
		}}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, nil)
		if failed != 0 || cnc != 0 {
			t.Fatalf("an unregistered basename must produce no finding, not even could-not-check; failed=%d cnc=%d notices=%v", failed, cnc, notices)
		}
	})
}

// --- Verify row 9 -----------------------------------------------------------

func TestSiblingMergeDeclaredRef(t *testing.T) {
	root := writeSiblingRegistry(t, siblingExampleRegistry)
	sib := newSiblingGitRepo(t)
	// The withheld-identifier shape (fact 4): the brief id appears nowhere;
	// only a same-repo Issue: trailer does.
	commitEmpty(t, sib, "fix: withheld-id shape\n\nA public PR that omits the private brief id.\n\nIssue: #942\n")

	t.Run("a declared tracked-in ref matched by the Issue: trailer is checked-failed", func(t *testing.T) {
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
			{Num: "04", Status: "todo", DeliverableRepo: "ex", TrackedIn: []string{"ex#942"}},
		}}
		overrides := map[string]string{"example-org/example-sibling": sib}
		_, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		if failed != 1 || cnc != 0 {
			t.Fatalf("a declared tracked-in ref matched by the trailer must be checked-failed; failed=%d cnc=%d", failed, cnc)
		}
	})

	t.Run("the same history with no declared ref is checked-clean (a documented miss)", func(t *testing.T) {
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
			{Num: "04", Status: "todo", DeliverableRepo: "ex"},
		}}
		overrides := map[string]string{"example-org/example-sibling": sib}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		if failed != 0 || cnc != 0 {
			t.Fatalf("with no tracked-in declared and no id match, the row must be checked-clean; failed=%d cnc=%d notices=%v", failed, cnc, notices)
		}
	})
}

// --- Verify row 10 ----------------------------------------------------------

func TestSiblingMergeIsPromptNotProof(t *testing.T) {
	root := writeSiblingRegistry(t, siblingExampleRegistry)
	sib := newSiblingGitRepo(t)
	commitEmpty(t, sib, "feat: ship it (example-stream/06) (#55)")
	s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
		{Num: "06", Status: "todo", DeliverableRepo: "ex"},
	}}
	overrides := map[string]string{"example-org/example-sibling": sib}
	notices, failed, _ := siblingMergeCheck([]*Stream{s}, root, overrides)
	if failed != 1 {
		t.Fatalf("want 1 checked-failed, got %d", failed)
	}
	joined := strings.ToLower(strings.Join(notices, "\n"))
	if !strings.Contains(joined, "check") {
		t.Errorf("the NOTICE must contain 'check'; got:\n%s", joined)
	}
	if strings.Contains(joined, "implemented") || strings.Contains(joined, "delivered") {
		t.Errorf("the primary NOTICE must never claim 'implemented' or 'delivered'; got:\n%s", joined)
	}

	// No code path of this class writes the stream README or the brief file.
	fullRoot := siblingBriefTree(t, "deliverable_repo: ex\n")
	readmePath := filepath.Join(fullRoot, "docs", "streams", "example-stream", "README.md")
	briefPath := filepath.Join(fullRoot, "docs", "streams", "example-stream", "brief-02-x.md")
	beforeReadme, _ := os.ReadFile(readmePath)
	beforeBrief, _ := os.ReadFile(briefPath)
	withSiblingRootOverride(t, "example-org/example-sibling="+sib)
	_ = run(fullRoot, "lint", nil, nil, "")
	afterReadme, _ := os.ReadFile(readmePath)
	afterBrief, _ := os.ReadFile(briefPath)
	if string(beforeReadme) != string(afterReadme) {
		t.Errorf("the stream README must never be modified by this class")
	}
	if string(beforeBrief) != string(afterBrief) {
		t.Errorf("the brief file must never be modified by this class")
	}
}

// --- Verify row 11 -----------------------------------------------------------

func TestSiblingMergeDeliveryClaimAck(t *testing.T) {
	root := writeSiblingRegistry(t, siblingExampleRegistry)

	t.Run("an acknowledged partial releases the hold with no finding", func(t *testing.T) {
		sib := newSiblingGitRepo(t)
		commitEmpty(t, sib, "feat: part one (example-stream/07) (#10)")
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{{
			Num: "07", Status: "todo", DeliverableRepo: "ex",
			Delivery: []DeliveryClaim{{In: "ex#10", Covers: "partial", Note: "Task 2 still owed"}},
		}}}
		overrides := map[string]string{"example-org/example-sibling": sib}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		if failed != 0 || cnc != 0 {
			t.Fatalf("an acknowledged partial must release the hold with no finding; failed=%d cnc=%d notices=%v", failed, cnc, notices)
		}
		if s.Briefs[0].MergedInSibling != "" {
			t.Errorf("a released row must not carry MergedInSibling; got %q", s.Briefs[0].MergedInSibling)
		}
	})

	t.Run("an acknowledged full keeps the hold with the cell-not-landed NOTICE", func(t *testing.T) {
		sib := newSiblingGitRepo(t)
		commitEmpty(t, sib, "feat: all done (example-stream/08) (#11)")
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{{
			Num: "08", Status: "todo", DeliverableRepo: "ex",
			Delivery: []DeliveryClaim{{In: "ex#11", Covers: "full"}},
		}}}
		overrides := map[string]string{"example-org/example-sibling": sib}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		if failed != 1 || cnc != 0 {
			t.Fatalf("a full claim with the cell still todo must keep the hold; failed=%d cnc=%d notices=%v", failed, cnc, notices)
		}
		joined := strings.Join(notices, "\n")
		if !strings.Contains(joined, "cell") || !strings.Contains(joined, "landed") {
			t.Errorf("expected a 'cell not landed' NOTICE; got:\n%s", joined)
		}
		if s.Briefs[0].MergedInSibling == "" {
			t.Errorf("a full-but-unlanded claim must still hold the row")
		}
	})

	t.Run("an unacknowledged second PR for the same brief still fires", func(t *testing.T) {
		sib := newSiblingGitRepo(t)
		commitEmpty(t, sib, "feat: part one (example-stream/09) (#20)")
		commitEmpty(t, sib, "feat: part two (example-stream/09) (#21)")
		s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{{
			Num: "09", Status: "todo", DeliverableRepo: "ex",
			Delivery: []DeliveryClaim{{In: "ex#20", Covers: "partial", Note: "more to come"}},
		}}}
		overrides := map[string]string{"example-org/example-sibling": sib}
		notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
		if failed != 1 || cnc != 0 {
			t.Fatalf("an unacknowledged second PR must still fire even with an earlier partial ack; failed=%d cnc=%d notices=%v", failed, cnc, notices)
		}
		if s.Briefs[0].MergedInSibling == "" {
			t.Errorf("the row must still be held by the unacknowledged PR")
		}
	})
}

// --- Review finding C1: the board's own repo is never its own sibling ------

// TestSiblingTargetsExcludeOwnRepo pins the reviewer's C1 finding on PR 1683:
// siblingTargetsForBrief must drop any registry entry naming the board's OWN
// repo, from every one of the three sibling-set derivation paths
// (deliverable_repo:, homed-in:, and the registry walk the ../<basename>/
// heuristic shares with it) — never adding it as a target and never reporting
// it as a could-not-check. ownRepoFor's `self:` alias must win over a
// stream's own `repo:` frontmatter when both are present.
func TestSiblingTargetsExcludeOwnRepo(t *testing.T) {
	registry := "schema: graph-repos-v1\ncell: test\nself: home\nrepos:\n" +
		"  home: {cell: test, repo: example-org/example-home}\n" +
		"  ex:   {cell: test, repo: example-org/example-sibling}\n"
	root := writeSiblingRegistry(t, registry)
	reg, ok, err := loadGraphRepos(root)
	if err != nil || !ok {
		t.Fatalf("loadGraphRepos: ok=%v err=%v", ok, err)
	}

	streams := []*Stream{{Name: "example-stream", Repo: "example-org/other-declared"}}
	if got := ownRepoFor(reg, streams); got != "example-org/example-home" {
		t.Fatalf("ownRepoFor = %q, want example-org/example-home — the registry's self: must win over repo: frontmatter", got)
	}

	// deliverable_repo: naming the own repo — silently dropped, never
	// could-not-check (the declaration resolved fine; it is simply not a
	// sibling).
	b1 := Brief{Num: "01", Status: "todo", DeliverableRepo: "home"}
	targets1, unresolved1 := siblingTargetsForBrief(b1, "", reg, "example-org/example-home")
	if len(targets1) != 0 {
		t.Errorf("deliverable_repo: naming the own repo must yield zero targets, got %+v", targets1)
	}
	if len(unresolved1) != 0 {
		t.Errorf("deliverable_repo: naming the own repo must NEVER be could-not-check, got %v", unresolved1)
	}

	// homed-in: naming the own repo — the registry-walk path.
	b2 := Brief{Num: "02", Status: "todo", HomedIn: "example-org/example-home"}
	targets2, unresolved2 := siblingTargetsForBrief(b2, "", reg, "example-org/example-home")
	if len(targets2) != 0 {
		t.Errorf("homed-in: naming the own repo must yield zero targets, got %+v", targets2)
	}
	if len(unresolved2) != 0 {
		t.Errorf("want 0 unresolved, got %v", unresolved2)
	}

	// The ../<basename>/ heuristic — same registry walk, own-repo basename in
	// the raw body.
	b3 := Brief{Num: "03", Status: "todo"}
	targets3, _ := siblingTargetsForBrief(b3, "see ../example-home/tools/x.go for context", reg, "example-org/example-home")
	if len(targets3) != 0 {
		t.Errorf("a ../<basename>/ hit on the own repo must yield zero targets, got %+v", targets3)
	}

	// A genuine sibling is unaffected by the exclusion.
	b4 := Brief{Num: "04", Status: "todo", DeliverableRepo: "ex"}
	targets4, _ := siblingTargetsForBrief(b4, "", reg, "example-org/example-home")
	if len(targets4) != 1 || targets4[0].Repo != "example-org/example-sibling" {
		t.Errorf("a genuine sibling must still resolve, got %+v", targets4)
	}
}

// TestSiblingMergeOwnRepoEndToEnd is the end-to-end companion: an own-history
// commit that happens to mention a todo row's id (an audit/authoring commit,
// exactly the reviewer's live-reproduction shape) must never hold that row
// once the brief's own deliverable_repo: alias is excluded as the board's own
// repo.
func TestSiblingMergeOwnRepoEndToEnd(t *testing.T) {
	registry := "schema: graph-repos-v1\ncell: test\nself: home\nrepos:\n" +
		"  home: {cell: test, repo: example-org/example-home}\n"
	root := writeSiblingRegistry(t, registry)
	// The "sibling" checkout is a real git repo whose history mentions the
	// brief id in an unrelated audit commit — the live false positive the
	// reviewer observed.
	home := newSiblingGitRepo(t)
	commitEmpty(t, home, "docs: audit mentions example-stream/03 in passing (#5)")

	s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
		{Num: "03", Status: "todo", DeliverableRepo: "home"},
	}}
	overrides := map[string]string{"example-org/example-home": home}
	notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
	if failed != 0 || cnc != 0 {
		t.Fatalf("the board's own repo must never be read as a sibling; failed=%d cnc=%d notices=%v", failed, cnc, notices)
	}
	if s.Briefs[0].MergedInSibling != "" {
		t.Errorf("the row must never be held by its own repo's history; got MergedInSibling=%q", s.Briefs[0].MergedInSibling)
	}
}

// TestSiblingMergePathIsRootBackstop pins pathIsRoot, the path-identity
// backstop finding C1 calls for: even when the name-based ownRepo comparison
// has nothing to compare against (no self:, no repo: frontmatter — ownRepo ==
// ""), a sibling checkout override that resolves to root's OWN directory must
// never be read as an external sibling. Without the backstop this reproduces
// the exact live defect: root's own history (which DOES mention the brief id,
// same as any audit/authoring commit) is read back as "merged in a sibling".
func TestSiblingMergePathIsRootBackstop(t *testing.T) {
	root := newSiblingGitRepo(t) // root is itself a real, non-shallow git checkout
	streamsDir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(streamsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamsDir, "graph-repos.yaml"), []byte(siblingExampleRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	commitEmpty(t, root, "docs: audit mentions example-stream/03 in passing (#5)")

	s := &Stream{Name: "example-stream", Status: "active", Briefs: []Brief{
		{Num: "03", Status: "todo", DeliverableRepo: "ex"},
	}}
	// A registry/name mismatch this run's name-based comparison alone cannot
	// catch (ownRepo is "" here — no self:, no repo: frontmatter) — but the
	// override resolves the registered sibling's checkout path back to root
	// itself.
	overrides := map[string]string{"example-org/example-sibling": root}
	notices, failed, cnc := siblingMergeCheck([]*Stream{s}, root, overrides)
	if failed != 0 || cnc != 0 {
		t.Fatalf("a sibling checkout path that resolves to root's OWN directory must be skipped silently — never checked, never could-not-check; failed=%d cnc=%d notices=%v", failed, cnc, notices)
	}
	if s.Briefs[0].MergedInSibling != "" {
		t.Errorf("a self-referential path must never hold the row; got MergedInSibling=%q", s.Briefs[0].MergedInSibling)
	}
}

// --- Review finding C2: the hold must reach the machine-readable queues too -

// TestNextUpJSONHonorsSiblingMergeHold pins the reviewer's C2 finding on PR
// 1683: runNextUp (the `--next-up` JSON emitter, dispatchqueue.go) must run
// the sibling-merge-unreconciled detector before nextUp(), exactly like
// run()'s STATUS.md path does — otherwise a row STATUS.md holds back as
// "merged in a sibling" is still emitted as a dispatchable todo in the JSON a
// cross-repo dispatcher actually reads.
func TestNextUpJSONHonorsSiblingMergeHold(t *testing.T) {
	root := siblingBriefTree(t, "deliverable_repo: ex\n") // one todo brief: example-stream/02
	sib := newSiblingGitRepo(t)
	commitEmpty(t, sib, "feat: ship it (example-stream/02) (#42)")
	withSiblingRootOverride(t, "example-org/example-sibling="+sib)

	out := captureStdout(t, func() {
		if code := runNextUp(root); code != 0 {
			t.Fatalf("runNextUp returned non-zero exit %d", code)
		}
	})

	var view dispatchView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("--next-up did not emit valid JSON: %v\noutput: %q", err, out)
	}
	for _, r := range view.Rows {
		if r.Brief == "example-stream/02" {
			t.Errorf("example-stream/02 is checked-failed sibling-merge-unreconciled and must be ABSENT from --next-up JSON (same hold STATUS.md renders), got rows: %+v", view.Rows)
		}
	}
}
