package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyFixture materialises a fixture repo into a fresh temp dir so a test can
// write STATUS.md into it without dirtying the checkout.
func copyFixture(t *testing.T, fixture string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	return root
}

// renameStream rewrites a copied fixture's stream directory AND its frontmatter
// `stream:` field, so the copy is a valid stream under a different name. Used to
// build two roots that legitimately differ and, for the duplicate test, one that
// deliberately does not.
func renameStream(t *testing.T, root, from, to string) {
	t.Helper()
	streams := filepath.Join(root, "docs", "streams")
	if err := os.Rename(filepath.Join(streams, from), filepath.Join(streams, to)); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(streams, to, "README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Replace(string(raw), "stream: "+from, "stream: "+to, 1)
	if err := os.WriteFile(readme, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setFrontmatter inserts a `key: value` line into a stream README's frontmatter.
func setFrontmatter(t *testing.T, root, stream, line string) {
	t.Helper()
	readme := filepath.Join(root, "docs", "streams", stream, "README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Replace(string(raw), "---\n", "---\n"+line+"\n", 1)
	if err := os.WriteFile(readme, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestMultiRootDuplicateStreamNameIsHardError pins the ownership rule: a stream
// lives in exactly ONE repo. Two roots defining the same stream name is an
// authoring error, and it must FAIL rather than silently merge — a silent merge
// means one repo's board is quietly overwritten by the other's rows, which is
// precisely the invisible failure multi-root has to rule out.
func TestMultiRootDuplicateStreamNameIsHardError(t *testing.T) {
	a := copyFixture(t, "testdata/goodrepo")
	b := copyFixture(t, "testdata/goodrepo") // same stream name "alpha" in both

	problems := crossRootProblems([]string{a, b})
	if len(problems) == 0 {
		t.Fatal("duplicate stream name across roots produced no problem — a stream must live in exactly one repo")
	}
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, `stream "alpha" is defined under two roots`) {
		t.Errorf("problem does not name the duplicated stream: %q", joined)
	}

	// And it must be fatal end to end, not merely reported.
	if code := runRoots([]string{a, b}, "lint", nil, nil, ""); code != 1 {
		t.Errorf("multi-root lint with a duplicate stream exited %d, want 1", code)
	}
}

// TestMultiRootDistinctStreamsPass is the control for the test above: the same
// two roots, differing only in stream name, must pass. Without it, a change that
// made crossRootProblems always fail would still look "correct".
func TestMultiRootDistinctStreamsPass(t *testing.T) {
	a := copyFixture(t, "testdata/goodrepo")
	b := copyFixture(t, "testdata/goodrepo")
	renameStream(t, b, "alpha", "beta")

	if problems := crossRootProblems([]string{a, b}); len(problems) != 0 {
		t.Fatalf("distinct stream names across roots reported problems: %v", problems)
	}
	if code := runRoots([]string{a, b}, "lint", nil, nil, ""); code != 0 {
		t.Errorf("multi-root lint over distinct roots exited %d, want 0", code)
	}
}

// TestMultiRootEmitsOnePerRootFiltered is the core behavior: one STATUS.md per
// root, each carrying ONLY that root's streams. The negative assertions are the
// load-bearing half — a generator that wrote the union to both files would pass
// every positive check.
func TestMultiRootEmitsOnePerRootFiltered(t *testing.T) {
	a := copyFixture(t, "testdata/goodrepo")
	b := copyFixture(t, "testdata/goodrepo")
	renameStream(t, b, "alpha", "beta")

	if code := runRoots([]string{a, b}, "write", nil, nil, ""); code != 0 {
		t.Fatalf("multi-root write exited %d", code)
	}

	boardA, err := os.ReadFile(filepath.Join(a, "STATUS.md"))
	if err != nil {
		t.Fatalf("root A board: %v", err)
	}
	boardB, err := os.ReadFile(filepath.Join(b, "STATUS.md"))
	if err != nil {
		t.Fatalf("root B board: %v", err)
	}

	if !strings.Contains(string(boardA), "alpha") {
		t.Error("root A board is missing its own stream alpha")
	}
	if strings.Contains(string(boardA), "beta") {
		t.Error("root A board carries root B's stream beta — roots bled")
	}
	if !strings.Contains(string(boardB), "beta") {
		t.Error("root B board is missing its own stream beta")
	}
	if strings.Contains(string(boardB), "alpha") {
		t.Error("root B board carries root A's stream alpha — roots bled")
	}

	// Per-root totals: each board counts ONE stream, not the union of two.
	for name, board := range map[string]string{"A": string(boardA), "B": string(boardB)} {
		if !strings.Contains(board, "**1** streams") {
			t.Errorf("root %s board does not total exactly its own 1 stream — counts are not per-root", name)
		}
	}
}

// TestSingleRootBackCompatByteEquivalence pins the load-bearing promise: a 1-arg
// run through the multi-root driver produces the SAME bytes as calling the
// single-root pipeline directly. The board is consumed by humans and tools alike,
// so "mostly the same" is not good enough.
func TestSingleRootBackCompatByteEquivalence(t *testing.T) {
	direct := copyFixture(t, "testdata/goodrepo")
	viaDriver := copyFixture(t, "testdata/goodrepo")

	if code := run(direct, "write", nil, nil, ""); code != 0 {
		t.Fatalf("direct single-root write exited %d", code)
	}
	if code := runRoots([]string{viaDriver}, "write", nil, nil, ""); code != 0 {
		t.Fatalf("driver single-root write exited %d", code)
	}

	want, err := os.ReadFile(filepath.Join(direct, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(viaDriver, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("single-root output differs when routed through runRoots\n--- direct ---\n%s\n--- via driver ---\n%s", want, got)
	}

	// The no-`repo:` case must render no repo banner at all: adding the field to
	// the model must not add a line to every existing board.
	if strings.Contains(string(want), "_Repo:") {
		t.Error("a board whose streams declare no repo: rendered a repo banner")
	}
}

// captureBoth runs fn with BOTH standard streams redirected and returns
// (stdout, stderr). Back-compat is a claim about the whole observable surface,
// not just the written file — a regression that decorates a single-root run
// shows up on stderr, where a STATUS.md comparison cannot see it.
func captureBoth(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, fn)
	})
	return stdout, stderr
}

// TestSingleRootEmitsNoMultiRootDecoration is the half of back-compat that the
// STATUS.md comparison above CANNOT pin.
//
// Because `runRoots` short-circuits a 1-root call straight into `run`, comparing
// the two through the same build is close to tautological: it proves the fast
// path exists but not that it is REQUIRED. Delete the `len(roots) == 1`
// short-circuit — a genuine regression, since single-root stderr then gains a
// `statusgen: === root X ===` banner and its verdict starts routing through
// verdictAccum — and the file comparison stays perfectly green.
//
// So this pins the observable surface directly, in every gate mode: a single-root
// run's stdout and stderr must be byte-identical to the direct call (modulo the
// two roots' own paths), must carry no root banner, and must still end with
// exactly one verdict line.
func TestSingleRootEmitsNoMultiRootDecoration(t *testing.T) {
	for _, mode := range []string{"write", "lint", "check"} {
		t.Run(mode, func(t *testing.T) {
			direct := copyFixture(t, "testdata/goodrepo")
			viaDriver := copyFixture(t, "testdata/goodrepo")

			var directCode, driverCode int
			directOut, directErr := captureBoth(t, func() {
				// Exactly what the documented single-root call site does.
				directCode = run(direct, mode, effectiveBudgetSpecs(mode, nil, direct), nil, "")
			})
			driverOut, driverErr := captureBoth(t, func() {
				driverCode = runRoots([]string{viaDriver}, mode, nil, nil, "")
			})

			if directCode != driverCode {
				t.Errorf("exit codes differ: direct=%d driver=%d", directCode, driverCode)
			}

			// The only legitimate difference is each run's own root path.
			norm := func(s, root string) string { return strings.ReplaceAll(s, root, "<ROOT>") }
			if got, want := norm(driverOut, viaDriver), norm(directOut, direct); got != want {
				t.Errorf("single-root stdout differs through runRoots\n--- direct ---\n%s\n--- driver ---\n%s", want, got)
			}
			if got, want := norm(driverErr, viaDriver), norm(directErr, direct); got != want {
				t.Errorf("single-root stderr differs through runRoots\n--- direct ---\n%s\n--- driver ---\n%s", want, got)
			}

			// Stated independently of the comparison, so the intent survives even
			// if someone later decorates BOTH paths identically.
			if strings.Contains(driverErr, "=== root ") {
				t.Errorf("a single-root run emitted a multi-root root banner:\n%s", driverErr)
			}
			if mode != "write" {
				finalStdoutLine(t, driverOut) // asserts exactly one verdict line
			}
		})
	}
}

// TestMultiRootBudgetResolvesPerRoot pins brief Task 3 for the word budget: the
// default `CLAUDE.md:2850` check is resolved against EACH root, never against
// roots[0].
//
// It matters because the default spec is skipped when the file is absent. Resolve
// it once from the first root and a second root that simply has no CLAUDE.md gets
// red-gated by the first root's convention — an adopter repo failing a check for a
// file it never agreed to keep. That is the fail-open-by-borrowed-context shape
// the brief forbids, and it is invisible in single-root runs.
func TestMultiRootBudgetResolvesPerRoot(t *testing.T) {
	withClaude := copyFixture(t, "testdata/goodrepo")
	if err := os.WriteFile(filepath.Join(withClaude, "CLAUDE.md"),
		[]byte("# conventions\n\nshort enough to be well inside the 2850-word budget.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	noClaude := copyFixture(t, "testdata/goodrepo")
	renameStream(t, noClaude, "alpha", "beta") // distinct ownership; see the duplicate test

	var code int
	stderr := captureStderr(t, func() {
		code = runRoots([]string{withClaude, noClaude}, "lint", nil, nil, "")
	})
	if code != 0 {
		t.Errorf("multi-root lint exited %d, want 0\n%s", code, stderr)
	}
	if strings.Contains(stderr, "budgeted file is missing") {
		t.Errorf("a root with no CLAUDE.md was budget-checked against another root's convention "+
			"— the budget spec is not resolved per root:\n%s", stderr)
	}

	// Control: the budget check is LIVE, not merely absent. Blow the first root's
	// CLAUDE.md past the cap and the same two-root run must go red on THAT root.
	over := strings.Repeat("word ", 3000)
	if err := os.WriteFile(filepath.Join(withClaude, "CLAUDE.md"), []byte(over), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr = captureStderr(t, func() {
		code = runRoots([]string{withClaude, noClaude}, "lint", nil, nil, "")
	})
	if code == 0 {
		t.Errorf("an over-budget CLAUDE.md in root 1 passed a two-root lint — the check is inert:\n%s", stderr)
	}
	if !strings.Contains(stderr, "CLAUDE.md") {
		t.Errorf("the over-budget failure does not name CLAUDE.md:\n%s", stderr)
	}
}

// TestMultiRootPrintsExactlyOneVerdict pins the output contract across roots.
// finalVerdict runs once per root pass, so without verdictAccum an N-root
// --lint prints N+1 verdict lines and a caller reading "the last line" (the
// documented way to consume a gate run) sees only the final root's outcome —
// silently green whenever the last root happens to be clean.
func TestMultiRootPrintsExactlyOneVerdict(t *testing.T) {
	for _, mode := range []string{"lint", "check"} {
		t.Run(mode, func(t *testing.T) {
			a := copyFixture(t, "testdata/goodrepo")
			b := copyFixture(t, "testdata/goodrepo")
			renameStream(t, b, "alpha", "beta")
			// --check compares against the generated boards, so generate them
			// first; otherwise it legitimately reports drift and the PASS
			// assertion below would be testing the fixture, not the contract.
			if code := runRoots([]string{a, b}, "write", nil, nil, ""); code != 0 {
				t.Fatalf("two-root write exited %d", code)
			}

			var code int
			stdout := captureStdout(t, func() {
				code = runRoots([]string{a, b}, mode, nil, nil, "")
			})
			if code != 0 {
				t.Fatalf("two-root %s exited %d:\n%s", mode, code, stdout)
			}
			// finalStdoutLine fails when the verdict count is not exactly 1.
			want := map[string]string{"lint": "LINT: PASS", "check": "CHECK: PASS"}[mode]
			if got := finalStdoutLine(t, stdout); got != want {
				t.Errorf("final stdout line = %q, want %q\n%s", got, want, stdout)
			}
		})
	}

	// The accumulator must also survive a root that FAILS: the single line has to
	// report the whole run's problem count, not the last root's.
	a := copyFixture(t, "testdata/goodrepo")
	b := copyFixture(t, "testdata/goodrepo")
	renameStream(t, b, "alpha", "beta")
	// Break root A only; root B stays clean and is linted LAST.
	if err := os.WriteFile(filepath.Join(a, "docs", "streams", "alpha", "README.md"),
		[]byte("---\nstream: alpha\n---\n\n[dead](./nope-does-not-exist.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stdout := captureStdout(t, func() { code = runRoots([]string{a, b}, "lint", nil, nil, "") })
	if code == 0 {
		t.Fatalf("a broken first root passed a two-root lint:\n%s", stdout)
	}
	line := finalStdoutLine(t, stdout) // still exactly one verdict
	if !strings.HasPrefix(line, "LINT: FAIL") {
		t.Errorf("final verdict = %q, want a LINT: FAIL — a clean LAST root must not mask an earlier failure\n%s",
			line, stdout)
	}
}

// TestMultiRootReportsCoverage pins the coverage line. The per-root banners are
// printed BEFORE each root is attempted, so they enumerate CONFIGURED roots — a
// root that then fails still gets one. Without a post-loop count, a partially
// failed run is legible only by matching banners against PROBLEM blocks, and
// "this board is missing a repo" is the failure the brief exists to surface.
func TestMultiRootReportsCoverage(t *testing.T) {
	a := copyFixture(t, "testdata/goodrepo")
	b := copyFixture(t, "testdata/goodrepo")
	renameStream(t, b, "alpha", "beta")

	stderr := captureStderr(t, func() { runRoots([]string{a, b}, "lint", nil, nil, "") })
	if !strings.Contains(stderr, "2/2 root(s) completed without error") {
		t.Errorf("a clean two-root run does not report full coverage:\n%s", stderr)
	}

	// A failing root must be visible in the count, not only in the noise above it.
	if err := os.WriteFile(filepath.Join(a, "docs", "streams", "alpha", "README.md"),
		[]byte("---\nstream: alpha\n---\n\n[dead](./nope-does-not-exist.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr = captureStderr(t, func() { runRoots([]string{a, b}, "lint", nil, nil, "") })
	if !strings.Contains(stderr, "1/2 root(s) completed without error") {
		t.Errorf("a partially failed run reports full coverage — the count is inert:\n%s", stderr)
	}

	// Single-root runs must NOT gain the line (byte-equivalence); asserted here
	// too so the reason survives next to the feature.
	solo := copyFixture(t, "testdata/goodrepo")
	stderr = captureStderr(t, func() { runRoots([]string{solo}, "lint", nil, nil, "") })
	if strings.Contains(stderr, "completed without error") {
		t.Errorf("a single-root run emitted the multi-root coverage line:\n%s", stderr)
	}
}

// TestSingleRootOnlySubcommandIsWiredToExit2 closes the gap between the pure
// helper (tested in TestSingleRootOnlySubcommand) and main's use of it: deleting
// the refusal block while leaving the helper intact would go undetected, and the
// sub-command would then silently run against roots[0] — the exact "first root
// and hope" behaviour multi-root exists to remove.
func TestSingleRootOnlySubcommandIsWiredToExit2(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "singleRootOnlySubcommand(") {
		t.Fatal("main.go never calls singleRootOnlySubcommand — a multi-root run would silently use the first root")
	}
	// The refusal must be a usage error (exit 2), not a PROBLEM or a warning.
	idx := strings.Index(src, "singleRootOnlySubcommand(")
	if idx < 0 {
		t.Fatal("unreachable")
	}
	// Window sized to span the whole single-root-only map (which grows as
	// self-contained sub-commands are added — e.g. --drive-issues, mm drives
	// phase 2) plus the refusal block that follows it. It only needs to be large
	// enough to reach the os.Exit(2); the assertion's intent is that the refusal
	// exists and exits 2, not that it sits within any exact byte count.
	window := src[idx:min(idx+1800, len(src))]
	if !strings.Contains(window, "os.Exit(2)") && !strings.Contains(window, "return 2") {
		t.Errorf("the singleRootOnlySubcommand refusal does not exit 2:\n%s", window)
	}
	// And it must be reached only for genuine multi-root runs.
	if !strings.Contains(src, "if len(resolvedRoots) > 1 {") {
		t.Error("the refusal is not gated on len(resolvedRoots) > 1")
	}
}

// TestResolveRoots covers the flag surface: absent --root keeps the historical
// default, and a repeated root is a usage error (running one root twice would
// write its board twice and double-count it).
func TestResolveRoots(t *testing.T) {
	got, err := resolveRoots(nil)
	if err != nil {
		t.Fatalf("no --root: %v", err)
	}
	if len(got) != 1 || got[0] != "." {
		t.Errorf("no --root resolved to %v, want [.]", got)
	}

	got, err = resolveRoots(rootFlags{"a", "b"})
	if err != nil {
		t.Fatalf("two roots: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("two roots resolved to %v, want [a b]", got)
	}

	if _, err := resolveRoots(rootFlags{"a", "a"}); err == nil {
		t.Error("a repeated --root was accepted; it must be a usage error")
	}
}

// TestRepoFrontmatterParsedAndValidated pins `repo:` as LIVE metadata: parsed off
// the stream README, form-validated, and disagreement within one root rejected.
// A field that parses but is never checked is dead metadata that rots (a dead-metadata hazard).
func TestRepoFrontmatterParsedAndValidated(t *testing.T) {
	t.Run("parsed off the README", func(t *testing.T) {
		root := copyFixture(t, "testdata/goodrepo")
		setFrontmatter(t, root, "alpha", "repo: example-org/example")
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(streams) != 1 || streams[0].Repo != "example-org/example" {
			t.Fatalf("repo: not parsed; got %+v", streams)
		}
		if streams[0].Root != root {
			t.Errorf("stream Root = %q, want %q — streams must be tagged with their root", streams[0].Root, root)
		}
	})

	t.Run("malformed value is a problem", func(t *testing.T) {
		streams := []*Stream{{Name: "alpha", Repo: "not-a-repo"}}
		repo, problems := rootRepo(streams)
		if repo != "" {
			t.Errorf("malformed repo returned %q, want empty", repo)
		}
		if len(problems) != 1 || !strings.Contains(problems[0], "invalid repo") {
			t.Errorf("malformed repo problems = %v", problems)
		}
	})

	t.Run("one root, one repo", func(t *testing.T) {
		streams := []*Stream{
			{Name: "alpha", Repo: "owner/one"},
			{Name: "beta", Repo: "owner/two"},
		}
		repo, problems := rootRepo(streams)
		if repo != "" {
			t.Errorf("conflicting declarations returned repo %q, want empty", repo)
		}
		if len(problems) != 1 || !strings.Contains(problems[0], "conflicting repo:") {
			t.Errorf("conflicting repo problems = %v", problems)
		}
	})

	t.Run("agreeing declarations resolve", func(t *testing.T) {
		streams := []*Stream{
			{Name: "alpha", Repo: "owner/one"},
			{Name: "beta", Repo: "owner/one"},
			{Name: "gamma"}, // undeclared is always allowed
		}
		repo, problems := rootRepo(streams)
		if repo != "owner/one" {
			t.Errorf("agreeing declarations resolved to %q, want owner/one", repo)
		}
		if len(problems) != 0 {
			t.Errorf("agreeing declarations reported problems: %v", problems)
		}
	})

	t.Run("invalid repo fails lint", func(t *testing.T) {
		root := copyFixture(t, "testdata/goodrepo")
		setFrontmatter(t, root, "alpha", "repo: nope")
		if code := run(root, "lint", nil, nil, ""); code != 1 {
			t.Errorf("lint with a malformed repo: exited %d, want 1", code)
		}
	})

	t.Run("declared repo is surfaced on the board", func(t *testing.T) {
		root := copyFixture(t, "testdata/goodrepo")
		setFrontmatter(t, root, "alpha", "repo: example-org/example")
		if code := run(root, "write", nil, nil, ""); code != 0 {
			t.Fatalf("write exited %d", code)
		}
		board, err := os.ReadFile(filepath.Join(root, "STATUS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(board), "_Repo: `example-org/example`") {
			t.Error("declared repo: is not surfaced on the generated board — it would ship as dead metadata")
		}
	})
}

// TestCrossRootDuplicateRepoDeclaration pins the other half of the ownership
// rule: two roots are two repos, so two roots claiming the same repo id is an
// authoring error even when their stream names differ.
func TestCrossRootDuplicateRepoDeclaration(t *testing.T) {
	a := copyFixture(t, "testdata/goodrepo")
	b := copyFixture(t, "testdata/goodrepo")
	renameStream(t, b, "alpha", "beta")
	setFrontmatter(t, a, "alpha", "repo: owner/same")
	setFrontmatter(t, b, "beta", "repo: owner/same")

	problems := crossRootProblems([]string{a, b})
	if len(problems) == 0 {
		t.Fatal("two roots declaring the same repo produced no problem")
	}
	if !strings.Contains(strings.Join(problems, "\n"), `repo "owner/same" is declared by two roots`) {
		t.Errorf("problem does not name the duplicated repo: %v", problems)
	}
}

// TestPerRootRegistersDoNotBleed pins the findings half of "per-root derived
// data": a finding in one root's register must not reach the other root's board.
// It is checked in BOTH directions and, critically, via a finding whose affects:
// names a stream that exists only in root B — statusgen hard-errors on an
// unresolvable affects:, so a register that bled would turn root A red rather
// than fail quietly.
func TestPerRootRegistersDoNotBleed(t *testing.T) {
	a := copyFixture(t, "testdata/goodrepo")
	b := copyFixture(t, "testdata/goodrepo")
	renameStream(t, b, "alpha", "beta")

	findingsDir := filepath.Join(b, "docs", "streams", "findings")
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := "---\nid: F-root-b-only-item\ndate: 2026-07-25\ntitle: Root B only\naffects: [\"beta\"]\nresolved: false\n---\n\nBelongs to root B.\n"
	if err := os.WriteFile(filepath.Join(findingsDir, "F-root-b-only-item.md"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := runRoots([]string{a, b}, "write", nil, nil, ""); code != 0 {
		t.Fatalf("multi-root write exited %d — root A likely saw root B's finding", code)
	}
	boardA, err := os.ReadFile(filepath.Join(a, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	boardB, err := os.ReadFile(filepath.Join(b, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(boardA), "F-root-b-only-item") {
		t.Error("root A's board lists root B's finding — findings bled across roots")
	}
	if !strings.Contains(string(boardB), "F-root-b-only-item") {
		t.Error("root B's board is missing its own finding")
	}
}

// TestSingleRootOnlySubcommand pins the refusal used to keep sub-commands from
// silently narrowing to the first root. Deterministic ordering matters: a caller
// enabling two must get a stable message, not a map-iteration lottery.
func TestSingleRootOnlySubcommand(t *testing.T) {
	if got := singleRootOnlySubcommand(map[string]bool{"--dora": false, "--trend": false}); got != "" {
		t.Errorf("no sub-command enabled returned %q, want empty", got)
	}
	if got := singleRootOnlySubcommand(map[string]bool{"--dora": false, "--trend": true}); got != "--trend" {
		t.Errorf("enabled sub-command returned %q, want --trend", got)
	}
	for i := 0; i < 20; i++ {
		if got := singleRootOnlySubcommand(map[string]bool{"--alarms": true, "--trend": true, "--dora": true}); got != "--alarms" {
			t.Fatalf("multiple enabled returned %q, want the lowest-sorting --alarms (deterministic)", got)
		}
	}
}

// TestStubRootFixtureIsUsable guards the committed synthetic fixture itself: it
// must stay lint-clean and keep the unique stream name the verify procedure
// greps for. A fixture that silently rots makes every Verify row that leans on it
// meaningless.
func TestStubRootFixtureIsUsable(t *testing.T) {
	root := copyFixture(t, "testdata/stubroot")
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("stub fixture root does not lint clean (exit %d)", code)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].Name != "multiroot-fixture" {
		t.Fatalf("stub fixture streams = %+v, want exactly one named multiroot-fixture", streams)
	}
	if streams[0].Repo == "" {
		t.Error("stub fixture stream declares no repo: — the fixture is meant to exercise that field")
	}
	if len(streams[0].Briefs) < 3 {
		t.Errorf("stub fixture has %d briefs, want a populated stream (>=3) — an under-populated stub under-tests multi-root", len(streams[0].Briefs))
	}
}
