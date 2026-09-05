package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// newbrief tests (mistake-proofing/05). Top-level functions are named TestNewBrief*
// so Verify row 5 (`-run 'NewBrief'`) counts them all; the refusal battery is
// TestNewBriefRefuses* (row 6); the atomicity test is TestNewBriefInverseEdgeAtomic
// (row 7); the positive control is TestNewBriefOutputLintsClean (row 8).

// nbStreamReadme is a minimal stream README with a Briefs table the generator can
// append a row to.
const nbStreamReadme = `---
stream: demo
repo: medici-finance/assay
status: active
priority: P2
track: platform
---

# demo

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [First](./brief-01-first.md) | 0 | S | todo | — | — |

## Notes
Prose after the table so the inserter must stop at the table's last row.
`

// nbBrief01 is the wave-0 brief a dependent brief points at; its unblocks: is where
// the inverse edge is written.
const nbBrief01 = `---
brief: demo/01
title: First
why: A worked wave-0 brief so a dependent has something real to point at.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-26 fixture
sources: ["fixture: first"]
---

# Brief 01 — First

## Context
files:
facts:
- nothing
`

// nbTree writes the demo stream (README + brief 01) into a fresh temp root and
// returns the root. offline runs pin newBriefNow and force the freshness fetch to
// fail unless a test overrides it.
func nbTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(nbStreamReadme), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-first.md"), []byte(nbBrief01), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// nbRun runs the generator. It pins the authored: date, and on cleanup resets ALL
// seams to their package defaults — so a test may set newBriefFreshness /
// newBriefWriteFile BEFORE calling nbRun (active during the run) without leaking the
// override into the next test.
func nbRun(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	newBriefNow = func() time.Time { return time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		newBriefNow = time.Now
		newBriefFreshness = defaultNewBriefFreshness
		newBriefWriteFile = os.WriteFile
	})
	var out, errb bytes.Buffer
	code = runNewBrief(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func TestNewBriefEmitsEveryRequiredKey(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, _, _ := nbRun(t,
		"--root", root, "--stream", "demo", "--title", "A generated brief",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	raw, err := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-02-a-generated-brief.md"))
	if err != nil {
		t.Fatalf("brief not created: %v", err)
	}
	body := string(raw)
	// Every required brief-v1 key present — a missing key and an empty key are
	// different states, and the generator emits the key either way.
	for _, key := range []string{"brief:", "title:", "why:", "wave:", "depends:", "unblocks:", "effort:", "gate:", "risk:", "issues:", "schema:", "authored:", "sources:"} {
		if !strings.Contains(body, "\n"+key) && !strings.HasPrefix(body, "---\n"+key) {
			t.Errorf("emitted brief is missing required key %q", key)
		}
	}
	// The required body sections, in order.
	for _, sec := range []string{"## Context", "## Ground rules", "## Task", "## Verify", "## Evidence", "## Review"} {
		if !strings.Contains(body, sec) {
			t.Errorf("emitted brief is missing body section %q", sec)
		}
	}
	if !strings.Contains(body, "gate: model") {
		t.Errorf("all-no risk must derive gate: model; body:\n%s", body)
	}
}

func TestNewBriefDerivesGateHumanFromRisk(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, _, _ := nbRun(t,
		"--root", root, "--stream", "demo", "--title", "Risky",
		"--regulatory", "no", "--customer", "yes", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-02-risky.md"))
	body := string(raw)
	if !strings.Contains(body, "gate: human") {
		t.Errorf("any risk yes must derive gate: human; body:\n%s", body)
	}
	// A risk-gated brief owes a written rationale and a decision section —
	// emitted empty-but-present, never omitted.
	if !strings.Contains(body, "gate-why:") {
		t.Errorf("risk-gated brief must emit gate-why (empty-but-present)")
	}
	if !strings.Contains(body, "## Human decision") {
		t.Errorf("human-gated brief must emit a decision section")
	}
}

func TestNewBriefDerivesWaveFromDeps(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, _, se := nbRun(t,
		"--root", root, "--stream", "demo", "--title", "Second wave", "--depends", "demo/01",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("want exit 0, got %d; stderr=%s", code, se)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-02-second-wave.md"))
	body := string(raw)
	if !strings.Contains(body, "wave: 1") {
		t.Errorf("dep in wave 0 must derive wave 1; body:\n%s", body)
	}
	if !strings.Contains(body, `depends: ["demo/01"]`) {
		t.Errorf("depends not rendered; body:\n%s", body)
	}
}

func TestNewBriefStampsFreshnessFromFetch(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "abc1234", "2026-09-04", nil }
	code, _, _ := nbRun(t,
		"--root", root, "--stream", "demo", "--title", "Stamped",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-02-stamped.md"))
	if want := "freshness-checked 2026-09-04 @ abc1234 (origin/main)"; !strings.Contains(string(raw), want) {
		t.Errorf("want freshness stamp %q from the fetch; body:\n%s", want, raw)
	}
}

func TestNewBriefAutoNumbersNextFree(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	// brief 01 exists, so the next free number is 02.
	nbRun(t, "--root", root, "--stream", "demo", "--title", "Two",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if _, err := os.Stat(filepath.Join(root, "docs", "streams", "demo", "brief-02-two.md")); err != nil {
		t.Fatalf("expected auto-numbered brief-02: %v", err)
	}
}

func TestNewBriefNeverOverwrites(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	// An explicit number colliding with brief 01's file must refuse — the
	// never-overwrite convention inherited from init.go.
	code, _, se := nbRun(t, "--root", root, "--stream", "demo", "--number", "01", "--title", "first",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code == newBriefExitOK {
		t.Fatalf("want refusal on an existing file; stderr=%s", se)
	}
	// The original brief 01 is untouched.
	raw, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-01-first.md"))
	if string(raw) != nbBrief01 {
		t.Errorf("brief 01 was modified by a refused run")
	}
}

// --- The refusal battery (Verify row 6). ---

func TestNewBriefRefusesSuppliedGate(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, out, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "X", "--gate", "model",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	nbAssertRefused(t, root, code, out, se, "gate is not accepted")
}

func TestNewBriefRefusesUnansweredRiskNonInteractive(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	// sensitive-data omitted → refusal, never a defaulted "no".
	code, out, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "X",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no")
	nbAssertRefused(t, root, code, out, se, "unanswered")
}

func TestNewBriefRefusesNonexistentDependency(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, out, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "X", "--depends", "demo/99",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	nbAssertRefused(t, root, code, out, se, "does not exist")
}

func TestNewBriefRefusesUntokenizableVerifyCommand(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	// A placeholder the tokenizer/metavar detector flags is not a Verify row.
	code, out, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "X",
		"--verify-command", "run <the thing>",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	nbAssertRefused(t, root, code, out, se, "usable Verify row")
}

// TestNewBriefRefusesStampOnFailedFetch is the fetch half of the refusal battery:
// a failed fetch produces NO stamp (the tool refuses to invent one) and reports
// could-not-check — an absent stamp is honest, an invented one is the defect. The
// brief file is still emitted (a missing optional stamp does not block a valid
// brief); what is refused is the unverified stamp, never a stamp with a made-up value.
func TestNewBriefRefusesStampOnFailedFetch(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrPermission }
	code, _, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "No stamp",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("a failed fetch still emits a valid brief (only the stamp is refused); got exit %d", code)
	}
	if !strings.Contains(se, "could-not-check") {
		t.Errorf("failed fetch must report could-not-check; stderr=%s", se)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-02-no-stamp.md"))
	if strings.Contains(string(raw), "freshness-checked") {
		t.Errorf("failed fetch must produce NO stamp, not an invented one; body:\n%s", raw)
	}
}

// nbAssertRefused checks a refusal exited non-zero, wrote no brief-02 file, left
// brief 01's unblocks empty, and named the reason.
func nbAssertRefused(t *testing.T, root string, code int, _, stderr, wantMsg string) {
	t.Helper()
	if code == newBriefExitOK {
		t.Fatalf("want a refusal (non-zero exit); got 0; stderr=%s", stderr)
	}
	if !strings.Contains(stderr, wantMsg) {
		t.Errorf("refusal message %q missing %q", stderr, wantMsg)
	}
	// No output: no brief-02 file was written.
	entries, _ := os.ReadDir(filepath.Join(root, "docs", "streams", "demo"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "brief-02") {
			t.Errorf("a refusal wrote output: %s", e.Name())
		}
	}
	// The dependency's inverse edge was NOT written on a refusal.
	raw, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-01-first.md"))
	if strings.Contains(string(raw), "demo/02") {
		t.Errorf("a refusal wrote an inverse edge into the dependency")
	}
}

// --- Inverse edge + atomicity (Verify row 7). ---

func TestNewBriefInverseEdgeAtomic(t *testing.T) {
	// Two dependencies: the inverse edge is written into BOTH.
	root := nbTree(t)
	dir := filepath.Join(root, "docs", "streams", "demo")
	brief02 := strings.Replace(strings.Replace(nbBrief01, "demo/01", "demo/02", 1), "# Brief 01 — First", "# Brief 02", 1)
	if err := os.WriteFile(filepath.Join(dir, "brief-02-second.md"), []byte(brief02), 0o644); err != nil {
		t.Fatal(err)
	}
	// Add a README row for brief 02 so it resolves.
	rd, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	updated, err := insertBriefRow(string(rd), "02", "Second", "second", 0, "S")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(updated), 0o644)

	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, _, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "Third",
		"--depends", "demo/01", "--depends", "demo/02",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("want exit 0, got %d; stderr=%s", code, se)
	}
	for _, dep := range []string{"brief-01-first.md", "brief-02-second.md"} {
		raw, _ := os.ReadFile(filepath.Join(dir, dep))
		if !strings.Contains(string(raw), "demo/03") {
			t.Errorf("inverse edge not written into %s; unblocks unchanged:\n%s", dep, raw)
		}
	}
}

func TestNewBriefInverseEdgeAtomicRollback(t *testing.T) {
	// A mid-write failure leaves NO partial graph: neither dependency's unblocks,
	// nor the README, nor the brief file, is changed.
	root := nbTree(t)
	dir := filepath.Join(root, "docs", "streams", "demo")

	before01, _ := os.ReadFile(filepath.Join(dir, "brief-01-first.md"))
	beforeReadme, _ := os.ReadFile(filepath.Join(dir, "README.md"))

	// Fail the write of the dependency edit (the LAST planned write), so the brief
	// file and the README have already been written when the failure hits — the
	// rollback must undo both.
	savedWrite := newBriefWriteFile
	newBriefWriteFile = func(name string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(name, "brief-01-first.md") {
			return os.ErrPermission
		}
		return savedWrite(name, data, perm)
	}
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, _, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "Doomed",
		"--depends", "demo/01",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code == newBriefExitOK {
		t.Fatalf("want a write failure; got exit 0; stderr=%s", se)
	}
	// No partial graph: the brief file is gone, the README is restored, the dep is
	// unchanged.
	if _, err := os.Stat(filepath.Join(dir, "brief-02-doomed.md")); !os.IsNotExist(err) {
		t.Errorf("a failed write left the brief file behind")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "README.md")); !bytes.Equal(got, beforeReadme) {
		t.Errorf("a failed write left the README modified:\n%s", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "brief-01-first.md")); !bytes.Equal(got, before01) {
		t.Errorf("a failed write left the dependency's unblocks modified:\n%s", got)
	}
}

// --- Positive control: the emitted brief passes the lint clean (Verify row 8). ---

func TestNewBriefOutputLintsClean(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "cafe123", "2026-09-04", nil }
	code, _, se := nbRun(t, "--root", root, "--stream", "demo", "--title", "Lints clean", "--depends", "demo/01",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("generation failed: exit %d; stderr=%s", code, se)
	}
	briefPath := filepath.Join(root, "docs", "streams", "demo", "brief-02-lints-clean.md")

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatalf("loadStreams: %v", err)
	}
	// checkBriefFiles is the reference validator — the frontmatter-shape and
	// cross-file methodology rules. A generated brief must produce ZERO PROBLEMs.
	problems, _ := checkBriefFiles(streams, streams)
	if len(problems) != 0 {
		t.Errorf("generated brief tripped checkBriefFiles PROBLEMs:\n%s", strings.Join(problems, "\n"))
	}
	// The three checks this brief's stream depends on must not fire a genuine HIT
	// on the emitted document — deriving the gate/wave/edge correctly is the whole
	// point. A COULD-NOT-CHECK on the risk×files cross-read is EXPECTED and honest:
	// a freshly-generated skeleton has not declared its `files:` yet, and the
	// three-state instrument reports that as could-not-check, never a pass. What
	// would be a defect is a HIT — a declared risky path with all-no answers — which
	// cannot happen with no declared paths.
	if p, n := riskFilesCrossRead(streams); len(p) != 0 {
		t.Errorf("risk×files cross-read (mistake-proofing/01) raised PROBLEMs on the generated brief:\n%v", p)
	} else {
		for _, ln := range n {
			if strings.Contains(ln, "brief-02-lints-clean") && !strings.Contains(ln, "COULD-NOT-CHECK") {
				t.Errorf("risk×files cross-read raised a genuine HIT on the generated brief: %s", ln)
			}
		}
	}
	if p, _ := identifierDereferenceCheck(root, []string{briefPath}); len(p) != 0 {
		t.Errorf("identifier dereference (mistake-proofing/02) fired on the generated brief:\n%s", strings.Join(p, "\n"))
	}
}

// TestNewBriefTitleWithYAMLMetacharacters proves that a realistic single-line
// title carrying YAML metacharacters — a colon+space, a leading bracket, and an
// embedded double quote — generates a brief whose frontmatter reparses cleanly and
// lints clean, rather than refusing with an opaque internal YAML parse error.
// Regression for review 5121184611: the `title:` frontmatter scalar was written
// unescaped, so unremarkable titles like `[urgent] fix thing` or
// `Quote " and colon: value` made the front door refuse (`did not find expected
// key` / `mapping values are not allowed in this context`) instead of generating.
func TestNewBriefTitleWithYAMLMetacharacters(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "cafe123", "2026-09-04", nil }
	const trickyTitle = `[urgent] Fix: the "thing"`
	code, _, se := nbRun(t,
		"--root", root, "--stream", "demo", "--title", trickyTitle, "--depends", "demo/01",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code != newBriefExitOK {
		t.Fatalf("a title with :, \" and [ must generate, not refuse; got exit %d, stderr=%s", code, se)
	}

	// Locate the generated brief (its slug is derived from the title).
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", "demo", "brief-02-*.md"))
	if len(matches) != 1 {
		t.Fatalf("want exactly one generated brief-02, got %v", matches)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("brief not created: %v", err)
	}
	body := string(raw)

	// The frontmatter must reparse as YAML with the title recovered byte-for-byte —
	// no corruption, no truncation, no injected key. Both the `title:` scalar and
	// the `sources:` entry embed the title and must survive.
	fmStart := strings.Index(body, "---\n")
	fmEnd := strings.Index(body, "\n---\n")
	if fmStart != 0 || fmEnd <= 0 {
		t.Fatalf("no frontmatter block found; body:\n%s", body)
	}
	front := body[len("---\n"):fmEnd]
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(front), &fm); err != nil {
		t.Fatalf("emitted frontmatter does not reparse as YAML: %v\nfrontmatter:\n%s", err, front)
	}
	if got, _ := fm["title"].(string); got != trickyTitle {
		t.Errorf("title did not round-trip through the frontmatter:\n got  %q\n want %q", got, trickyTitle)
	}

	// And the whole document must lint clean through the reference validator — the
	// same bar TestNewBriefOutputLintsClean holds the ordinary case to.
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatalf("loadStreams: %v", err)
	}
	if problems, _ := checkBriefFiles(streams, streams); len(problems) != 0 {
		t.Errorf("generated brief tripped checkBriefFiles PROBLEMs:\n%s", strings.Join(problems, "\n"))
	}
}

// TestNewBriefTitleWithNewlineRefusesCleanly documents the fourth character from
// the review — an embedded newline. A brief title spanning lines is nonsensical
// (it cannot inhabit the README's single-line Briefs-table row), so the correct
// behavior is a CLEAN refusal caught by the validate-before-write stage, never a
// partial write. This is the behavior the review already found acceptable; the
// frontmatter fix must not regress it into a silent corruption.
func TestNewBriefTitleWithNewlineRefusesCleanly(t *testing.T) {
	root := nbTree(t)
	newBriefFreshness = func(string) (string, string, error) { return "", "", os.ErrNotExist }
	code, out, se := nbRun(t,
		"--root", root, "--stream", "demo", "--title", "Evil\nsecond line", "--depends", "demo/01",
		"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no")
	if code == newBriefExitOK {
		t.Fatalf("a title with an embedded newline must refuse; got exit 0, stderr=%s", se)
	}
	// No partial write: no brief-02 file, and the dependency's inverse edge untouched.
	entries, _ := os.ReadDir(filepath.Join(root, "docs", "streams", "demo"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "brief-02") {
			t.Errorf("a refusal wrote output: %s", e.Name())
		}
	}
	dep, _ := os.ReadFile(filepath.Join(root, "docs", "streams", "demo", "brief-01-first.md"))
	if strings.Contains(string(dep), "demo/02") {
		t.Errorf("a refusal wrote an inverse edge into the dependency")
	}
	_ = out
}

func nbMentions(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
