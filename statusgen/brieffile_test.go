package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// briefSchemaProblems copies the shared fixture tree into a temp root, loads the
// streams, and returns the brief-file validation problems (hard, exit-1).
func briefSchemaProblems(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/briefschema")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := checkBriefFiles(streams)
	return problems
}

// briefSchemaChecks is briefSchemaProblems returning both the hard problems and
// the non-fatal notices (e.g. the gate-why NOTICE).
func briefSchemaChecks(t *testing.T) (problems, notices []string) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/briefschema")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return checkBriefFiles(streams)
}

// hasProblem reports whether some problem line contains every given substring.
func hasProblem(problems []string, subs ...string) bool {
	for _, p := range problems {
		all := true
		for _, s := range subs {
			if !strings.Contains(p, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func TestBriefFileValid(t *testing.T) {
	problems := briefSchemaProblems(t)
	if hasProblem(problems, "brief-01-valid.md") {
		t.Errorf("valid brief should raise no problems; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileMissingField(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-03-missing-sources.md", "missing required field", "sources") {
		t.Errorf("want a missing-required-field problem for sources; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestGateWhyProblem covers the gate-why-rationale PHASE 3 lint: a risk-gated
// brief (gate: human OR any risk answer yes) with no gate-why is a hard PROBLEM
// (exit 1); a brief that records a gate-why emits none. Table-driven.
func TestGateWhyProblem(t *testing.T) {
	problems := briefSchemaProblems(t)

	tests := []struct {
		name        string
		file        string
		wantProblem bool
	}{
		// gate: human, no gate-why → hard problem.
		{"human-gated without gate-why", "brief-31-human-done-model.md", true},
		// risk answer yes (gate model), no gate-why → hard problem via the anyYes path.
		{"risk-yes without gate-why", "brief-04-risk-gate.md", true},
		// gate: human WITH a recorded gate-why → no problem.
		{"gate-why present", "brief-36-gatewhy-present.md", false},
		// all-no risk, gate model (not risk-gated) → no problem.
		{"not risk-gated", "brief-01-valid.md", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasProblem(problems, tc.file, "gate-why"); got != tc.wantProblem {
				t.Errorf("problem for %s = %v, want %v; problems:\n%s", tc.file, got, tc.wantProblem, strings.Join(problems, "\n"))
			}
		})
	}
}

// TestWhyNotice covers the PHASE 1 lint: every brief-v1 brief
// SHOULD carry a why: — a brief without one emits a NON-FATAL NOTICE (never a
// hard problem this phase); a brief with one emits none.
func TestWhyNotice(t *testing.T) {
	problems, notices := briefSchemaChecks(t)

	tests := []struct {
		name       string
		file       string
		wantNotice bool
	}{
		// No why: → notice.
		{"without why", "brief-60-no-why.md", true},
		// With why: → no notice.
		{"with why", "brief-61-with-why.md", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasProblem(notices, tc.file, "why:"); got != tc.wantNotice {
				t.Errorf("notice for %s = %v, want %v; notices:\n%s", tc.file, got, tc.wantNotice, strings.Join(notices, "\n"))
			}
			// The why: lint is NEVER a hard problem this phase.
			if hasProblem(problems, tc.file, "why:") {
				t.Errorf("why: must be a NOTICE, not a hard problem, for %s; problems:\n%s", tc.file, strings.Join(problems, "\n"))
			}
		})
	}
}

// TestWhySubstanceFloor covers the substance floor on a PRESENT
// why: — presence alone let a title-paste, ".", or "TODO" score as fully
// compliant (zero information); the floor requires a minimum length and
// rejects a why that is a substring of, or near-duplicate of, the title.
// Still a NOTICE, same severity as the presence check (the hard-error flip is
// a separate future step).
func TestWhySubstanceFloor(t *testing.T) {
	problems, notices := briefSchemaChecks(t)

	tests := []struct {
		name       string
		file       string
		wantNotice bool
	}{
		// why: is a verbatim (or near-verbatim) copy of the title → notice.
		{"title-paste why", "brief-62-why-title-paste.md", true},
		// why: "." → far under the length floor → notice.
		{"dot why", "brief-63-why-dot.md", true},
		// why: "TODO" → far under the length floor → notice.
		{"TODO why", "brief-64-why-todo.md", true},
		// why: a genuine multi-line rationale distinct from the title → no notice.
		{"genuine why", "brief-65-why-genuine.md", false},
		// a real why (from TestWhyNotice's fixture) still clears the floor.
		{"with why (pre-existing fixture)", "brief-61-with-why.md", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasProblem(notices, tc.file, "substance floor"); got != tc.wantNotice {
				t.Errorf("substance-floor notice for %s = %v, want %v; notices:\n%s", tc.file, got, tc.wantNotice, strings.Join(notices, "\n"))
			}
			// The substance-floor lint is NEVER a hard problem this phase —
			// same non-fatal severity as the presence-only check it augments.
			if hasProblem(problems, tc.file, "substance floor") {
				t.Errorf("substance floor must be a NOTICE, not a hard problem, for %s; problems:\n%s", tc.file, strings.Join(problems, "\n"))
			}
		})
	}
}

// TestBriefFileInvalidValue covers the optional brief-v1 `value:` field:
// a present-but-unrecognized value is a hard PROBLEM
// naming the bad token; a valid `value: high` raises none; and an absent value
// (brief-01-valid) is never flagged because the field is optional.
func TestBriefFileInvalidValue(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-50-bad-value.md", "invalid value", "urgent") {
		t.Errorf("want an invalid-value problem for brief-50; got:\n%s", strings.Join(problems, "\n"))
	}
	if hasProblem(problems, "brief-51-good-value.md", "invalid value") {
		t.Errorf("valid value: high should raise no problem; got:\n%s", strings.Join(problems, "\n"))
	}
	if hasProblem(problems, "brief-01-valid.md", "invalid value") {
		t.Errorf("absent value must never be flagged (field is optional); got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileGateRiskMismatch(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-04-risk-gate.md", "risk answer is yes", "gate") {
		t.Errorf("want a risk/gate mismatch problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileUnresolvableDepends(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-05-bad-depends.md", "depends", "alpha/99", "unknown brief") {
		t.Errorf("want an unresolvable-depends problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileWaveMismatch(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-06-wave-mismatch.md", "wave", "!=") {
		t.Errorf("want a wave-mismatch problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestBriefFileLegacyExempt is the required legacy-exemption case: a brief file
// with no frontmatter must be left completely alone.
func TestBriefFileLegacyExempt(t *testing.T) {
	problems := briefSchemaProblems(t)
	if hasProblem(problems, "brief-01-legacy.md") {
		t.Errorf("legacy brief (no frontmatter) must be exempt; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileBadEffort(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-07-bad-effort.md", "invalid effort") {
		t.Errorf("want an invalid-effort problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileBadGate(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-08-bad-gate.md", "invalid gate") {
		t.Errorf("want an invalid-gate problem; got:\n%s", strings.Join(problems, "\n"))
	}
	// gate:robot with all-no risk must NOT also trip the risk/gate rule.
	if hasProblem(problems, "brief-08-bad-gate.md", "risk answer is yes") {
		t.Errorf("all-no risk should not trip the risk/gate rule; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileEmptySources(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-09-empty-sources.md", "sources must be non-empty") {
		t.Errorf("want an empty-sources problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileIDMismatch(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-10-id-mismatch.md", "does not match filename-derived id") {
		t.Errorf("want a brief-id/filename mismatch problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestBriefFileParseExemptions covers the two exempt return paths directly.
func TestBriefFileParseExemptions(t *testing.T) {
	dir := t.TempDir()

	noFM := writeTemp(t, dir, "brief-01-legacy.md", "# Brief 01 — no frontmatter\n")
	if bf, ok, err := parseBriefFile(noFM); bf != nil || ok || err != nil {
		t.Errorf("no-frontmatter file: want (nil,false,nil); got (%v,%v,%v)", bf, ok, err)
	}

	otherSchema := writeTemp(t, dir, "brief-02-other.md", "---\nschema: something-else\n---\n# Brief\n")
	if bf, ok, err := parseBriefFile(otherSchema); bf != nil || ok || err != nil {
		t.Errorf("non-brief-v1 schema: want (nil,false,nil); got (%v,%v,%v)", bf, ok, err)
	}
}

// TestBriefFileParseMissingField confirms the (nil,false,err) contract with a
// path-prefixed, aggregated message.
func TestBriefFileParseMissingField(t *testing.T) {
	dir := t.TempDir()
	p := writeTemp(t, dir, "brief-01-x.md", "---\nschema: brief-v1\nbrief: x/01\n---\n# Brief\n")
	bf, ok, err := parseBriefFile(p)
	if bf != nil || ok {
		t.Fatalf("want (nil,false,err); got (%v,%v,%v)", bf, ok, err)
	}
	if err == nil || !strings.Contains(err.Error(), "missing required field") {
		t.Fatalf("want a missing-required-field error; got %v", err)
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("error should be path-prefixed with %q; got %v", p, err)
	}
}

func TestBriefFileExpectedID(t *testing.T) {
	cases := []struct {
		path    string
		id, num string
		ok      bool
	}{
		{"docs/streams/methodology/brief-01-statusgen.md", "methodology/01", "01", true},
		{"docs/streams/operator/brief-12a-domain.md", "operator/12a", "12a", true},
		{"docs/streams/alpha/brief-07.md", "alpha/07", "07", true},
		{"docs/streams/alpha/notes.md", "", "", false},
	}
	for _, c := range cases {
		id, num, ok := expectedBriefID(c.path)
		if id != c.id || num != c.num || ok != c.ok {
			t.Errorf("expectedBriefID(%q) = (%q,%q,%v), want (%q,%q,%v)", c.path, id, num, ok, c.id, c.num, c.ok)
		}
	}
}

// validBriefFM is a well-formed brief-v1 file; parse-level tests mutate one line.
const validBriefFM = "---\n" +
	"schema: brief-v1\n" +
	"brief: x/01\n" +
	"title: t\n" +
	"wave: 0\n" +
	"depends: []\n" +
	"unblocks: []\n" +
	"effort: S\n" +
	"gate: model\n" +
	"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
	"issues: []\n" +
	"authored: 2026-07-08 by t\n" +
	"sources: [\"s\"]\n" +
	"---\n# Brief\n"

// A leading `---` with the opt-in marker but no closing fence is a real error,
// NOT an exemption (regression guard for the silent-exempt-on-malformed bug).
func TestBriefFileUnterminatedFrontmatter(t *testing.T) {
	p := writeTemp(t, t.TempDir(), "brief-01-x.md",
		"---\nschema: brief-v1\nbrief: x/01\ntitle: t\n# no closing fence\n")
	bf, ok, err := parseBriefFile(p)
	if bf != nil || ok || err == nil {
		t.Fatalf("unterminated frontmatter must error, not exempt; got (%v,%v,%v)", bf, ok, err)
	}
}

// A quoted schema value still opts in (the gate reads the parsed value), so a
// missing field is reported rather than the file being silently skipped.
func TestBriefFileQuotedSchemaOptsIn(t *testing.T) {
	fm := strings.Replace(validBriefFM, "schema: brief-v1", "schema: \"brief-v1\"", 1)
	fm = strings.Replace(fm, "sources: [\"s\"]\n", "", 1) // omit sources
	_, ok, err := parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", fm))
	if ok || err == nil || !strings.Contains(err.Error(), "sources") {
		t.Fatalf("quoted schema should opt in and report missing sources; got ok=%v err=%v", ok, err)
	}
}

// CRLF line endings must not exempt an opted-in brief.
func TestBriefFileCRLFOptsIn(t *testing.T) {
	fm := strings.Replace(validBriefFM, "sources: [\"s\"]\n", "", 1) // omit sources
	fm = strings.ReplaceAll(fm, "\n", "\r\n")
	_, ok, err := parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", fm))
	if ok || err == nil || !strings.Contains(err.Error(), "sources") {
		t.Fatalf("CRLF brief should opt in and report missing sources; got ok=%v err=%v", ok, err)
	}
}

// A bare (unquoted) date in authored decodes to time.Time; it must be accepted,
// not rejected as "not a string".
func TestBriefFileBareDateAuthored(t *testing.T) {
	fm := strings.Replace(validBriefFM, "authored: 2026-07-08 by t", "authored: 2026-07-08", 1)
	bf, ok, err := parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", fm))
	if !ok || err != nil {
		t.Fatalf("bare-date authored must be accepted; got (%v,%v,%v)", bf, ok, err)
	}
}

// A bare boolean risk value must be rejected — otherwise it would slip past the
// risk→human-gate rule (which only fires on the string "yes").
func TestBriefFileBareBoolRisk(t *testing.T) {
	fm := strings.Replace(validBriefFM,
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}",
		"risk: {regulatory: true, customer: no, irreversible: no, sensitive-data: no}", 1)
	_, ok, err := parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", fm))
	if ok || err == nil || !strings.Contains(err.Error(), "regulatory must be yes or no") {
		t.Fatalf("bare-bool risk must be rejected; got ok=%v err=%v", ok, err)
	}
}

func TestBriefFileWaveNotInt(t *testing.T) {
	fm := strings.Replace(validBriefFM, "wave: 0", "wave: \"zero\"", 1)
	_, ok, err := parseBriefFile(writeTemp(t, t.TempDir(), "brief-01-x.md", fm))
	if ok || err == nil || !strings.Contains(err.Error(), "wave must be an integer") {
		t.Fatalf("string wave must be rejected; got ok=%v err=%v", ok, err)
	}
}

func TestBriefFileNoReadmeRow(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-14-no-row.md", "no row") {
		t.Errorf("want a no-README-row problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileUnresolvableUnblocks(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-15-bad-unblocks.md", "unblocks", "alpha/97", "unknown brief") {
		t.Errorf("want an unresolvable-unblocks problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileBadRefs(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-16-bad-refs.md", "unknown stream") {
		t.Errorf("want an unknown-stream ref problem; got:\n%s", strings.Join(problems, "\n"))
	}
	if !hasProblem(problems, "brief-16-bad-refs.md", "not a <stream>/<NN> id") {
		t.Errorf("want a malformed-ref problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

func TestBriefFileBadFilename(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-zz-bad-name.md", "filename must match") {
		t.Errorf("want a filename-mismatch problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

// --- Evidence enforcement at the verified gate ---

// A verified brief whose Evidence section holds only the contract comment fails.
func TestEvidenceVerifiedEmptyFails(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-20-verified-empty.md", "Evidence") {
		t.Errorf("verified brief with empty Evidence must be flagged; got:\n%s", strings.Join(problems, "\n"))
	}
}

// A verified brief with a real content row in Evidence passes.
func TestEvidenceVerifiedFilledPasses(t *testing.T) {
	problems := briefSchemaProblems(t)
	if hasProblem(problems, "brief-21-verified-filled.md") {
		t.Errorf("verified brief with filled Evidence must pass; got:\n%s", strings.Join(problems, "\n"))
	}
}

// A todo brief is not subject to the Evidence requirement.
func TestEvidenceTodoEmptyPasses(t *testing.T) {
	problems := briefSchemaProblems(t)
	if hasProblem(problems, "brief-22-todo-empty.md") {
		t.Errorf("todo brief with empty Evidence must pass; got:\n%s", strings.Join(problems, "\n"))
	}
}

// A legacy (no-frontmatter) brief is exempt even when its README row is done.
func TestEvidenceLegacyDoneExempt(t *testing.T) {
	problems := briefSchemaProblems(t)
	if hasProblem(problems, "brief-02-legacy-done.md") {
		t.Errorf("legacy done brief must be exempt from Evidence enforcement; got:\n%s", strings.Join(problems, "\n"))
	}
}

// The check ORs verified/done; this exercises the `done` fail path explicitly
// (an earlier gap: only `verified` had a failing fixture before).
func TestEvidenceDoneEmptyFails(t *testing.T) {
	problems := briefSchemaProblems(t)
	if !hasProblem(problems, "brief-23-done-empty.md", "Evidence") {
		t.Errorf("done brief with empty Evidence must be flagged; got:\n%s", strings.Join(problems, "\n"))
	}
}

// Unit-level check of the content detector: the contract comment alone is empty.
func TestEvidenceContentDetection(t *testing.T) {
	if evidenceHasContent("<!-- only a contract comment\n     spanning lines -->\n\n") {
		t.Error("comment-only Evidence should have no content")
	}
	if !evidenceHasContent("<!-- comment -->\n| 1 | cmd | 0 | ok |\n") {
		t.Error("a table row is content")
	}
	// An unterminated comment consumes the rest of the section — not content.
	if evidenceHasContent("<!-- unterminated, no closing marker\nnot really a row\n") {
		t.Error("an unterminated comment (and everything after it) must not count as content")
	}
	if extractEvidence("# B\n\n## Evidence\nrow\n\n## Review\nx\n") != "row\n" {
		t.Errorf("extractEvidence should return the section body, got %q", extractEvidence("# B\n\n## Evidence\nrow\n\n## Review\nx\n"))
	}
}

// TestBriefFileRiskKeys covers amendment item (b): the risk block
// must contain exactly the four canonical keys — a missing question can never
// fire the human gate, and unknown keys are schema drift.
func TestBriefFileRiskKeys(t *testing.T) {
	dir := t.TempDir()
	missing := writeTemp(t, dir, "brief-01-a.md", "---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no}\nissues: []\nschema: brief-v1\nauthored: 2026-07-08\nsources: [\"s\"]\n---\n# B\n")
	if _, ok, err := parseBriefFile(missing); ok || err == nil || !strings.Contains(err.Error(), "missing canonical risk key") {
		t.Errorf("2-of-4 risk keys must error; got ok=%v err=%v", ok, err)
	}
	extra := writeTemp(t, dir, "brief-02-b.md", "---\nbrief: x/02\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no, vibes: no}\nissues: []\nschema: brief-v1\nauthored: 2026-07-08\nsources: [\"s\"]\n---\n# B\n")
	if _, ok, err := parseBriefFile(extra); ok || err == nil || !strings.Contains(err.Error(), "unknown risk key") {
		t.Errorf("extra risk key must error; got ok=%v err=%v", ok, err)
	}
}

// --- risk-gate enforcement at the done gate ---
//
// (a) risk-yes + gate:model is a self-consistency error (already from brief-01);
// (b) a human-gated brief at done must have a Reviewed column naming a human.
func TestRiskGate(t *testing.T) {
	problems := briefSchemaProblems(t)

	t.Run("human-gated done with a human reviewer passes", func(t *testing.T) {
		if hasProblem(problems, "brief-30-human-done-ok.md") {
			t.Errorf("got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("human-gated done with only a model sign-off fails", func(t *testing.T) {
		if !hasProblem(problems, "brief-31-human-done-model.md", "names no human") {
			t.Errorf("got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("model-gated done is not subject to the human-reviewer rule", func(t *testing.T) {
		if hasProblem(problems, "brief-32-model-done.md", "names no human") {
			t.Errorf("got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("risk yes with gate model fails (self-consistency)", func(t *testing.T) {
		if !hasProblem(problems, "brief-04-risk-gate.md", "risk answer is yes") {
			t.Errorf("got:\n%s", strings.Join(problems, "\n"))
		}
	})
	// A token that merely contains the "human:" substring
	// (e.g. "superhuman:x") must NOT satisfy the human-reviewer rule.
	t.Run("a superhuman: reviewer tag does not count as a human reviewer", func(t *testing.T) {
		if !hasProblem(problems, "brief-33-human-done-superhuman.md", "names no human") {
			t.Errorf("got:\n%s", strings.Join(problems, "\n"))
		}
	})
	// Irreversible-gate tightening: an irreversible:yes brief pulls the human gate
	// one step earlier — it may not be marked `verified` on a model-only sign-off,
	// not just `done`. (A cheap-tier model cannot pre-close an on-ledger change.)
	t.Run("irreversible verified with a human reviewer passes", func(t *testing.T) {
		if hasProblem(problems, "brief-34-irreversible-verified-human.md", "names no human") {
			t.Errorf("got:\n%s", strings.Join(problems, "\n"))
		}
	})
	t.Run("irreversible verified on a model-only sign-off fails", func(t *testing.T) {
		if !hasProblem(problems, "brief-35-irreversible-verified-model.md", "names no human") {
			t.Errorf("got:\n%s", strings.Join(problems, "\n"))
		}
	})
}

// TestExecTier covers the optional exec-tier frontmatter field.
func TestExecTier(t *testing.T) {
	problems, notices := briefSchemaChecks(t)

	t.Run("invalid exec-tier value is a PROBLEM", func(t *testing.T) {
		if !hasProblem(problems, "brief-70-bad-exec-tier.md", "invalid exec-tier", "turbo", "any or strong") {
			t.Errorf("want invalid exec-tier problem; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("exec-tier strong without why is a NOTICE", func(t *testing.T) {
		if hasProblem(problems, "brief-71-exec-strong-no-why.md", "exec-tier") {
			t.Errorf("strong without why should be a NOTICE, not a PROBLEM; got problem:\n%s", strings.Join(problems, "\n"))
		}
		found := false
		for _, n := range notices {
			if strings.Contains(n, "brief-71-exec-strong-no-why.md") && strings.Contains(n, "exec-tier-why") {
				found = true
			}
		}
		if !found {
			t.Errorf("want a missing exec-tier-why NOTICE; notices:\n%s", strings.Join(notices, "\n"))
		}
	})

	t.Run("exec-tier strong with why is clean", func(t *testing.T) {
		if hasProblem(problems, "brief-72-exec-strong-with-why.md") {
			t.Errorf("strong with why should be clean; got:\n%s", strings.Join(problems, "\n"))
		}
		for _, n := range notices {
			if strings.Contains(n, "brief-72-exec-strong-with-why.md") {
				t.Errorf("strong with why should not emit a NOTICE; got: %s", n)
			}
		}
	})

	t.Run("exec-tier any is clean", func(t *testing.T) {
		if hasProblem(problems, "brief-73-exec-any.md") {
			t.Errorf("exec-tier any should be clean; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("valid brief parses exec-tier strong", func(t *testing.T) {
		bf, ok, err := parseBriefFile("testdata/briefschema/docs/streams/alpha/brief-72-exec-strong-with-why.md")
		if err != nil || !ok {
			t.Fatalf("parse failed: err=%v ok=%v", err, ok)
		}
		if bf.ExecTier != "strong" {
			t.Errorf("ExecTier = %q, want strong", bf.ExecTier)
		}
		if !strings.Contains(bf.ExecTierWhy, "cross-component") {
			t.Errorf("ExecTierWhy = %q, want containing 'cross-component'", bf.ExecTierWhy)
		}
	})
}

// TestExecTierMarker covers the [exec:strong] marker rendering in emit.
func TestExecTierMarker(t *testing.T) {
	s := mkStream("alpha", "active", "P1")
	s.Track = "product"
	b := Brief{Num: "72", Title: "exec-tier strong with why", Wave: 0, Status: "implemented", ExecTier: "strong"}
	s.Briefs = append(s.Briefs, b)

	// Next-up rendering
	nu := nextUp([]*Stream{s}, nil, nil)
	out := emit([]*Stream{s}, nil, nu, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "[exec:strong]") {
		t.Errorf("Next-up output missing [exec:strong] marker:\n%s", out)
	}

	// Awaiting-verification rendering
	if !strings.Contains(out, "72 [exec:strong]") {
		t.Errorf("Awaiting output missing [exec:strong] marker on the brief row:\n%s", out)
	}

	// exec-tier: any should NOT render a marker
	b2 := Brief{Num: "73", Title: "exec-tier any", Wave: 0, Status: "implemented", ExecTier: "any"}
	s2 := mkStream("beta", "active", "P1")
	s2.Track = "platform"
	s2.Briefs = append(s2.Briefs, b2)
	nu2 := nextUp([]*Stream{s2}, nil, nil)
	out2 := emit([]*Stream{s2}, nil, nu2, nil, IntakeAlarmResult{}, nil, "")
	if strings.Contains(out2, "[exec:strong]") {
		t.Errorf("exec-tier any must not render [exec:strong]:\n%s", out2)
	}
}

func TestSameWaveDepLint(t *testing.T) {
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams", "samewave")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "---\n" +
		"stream: samewave\n" +
		"status: active\n" +
		"priority: P1\n" +
		"track: test\n" +
		"---\n" +
		"# Samewave\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [One](./brief-01.md) | 1 | S | todo | \u2014 | \u2014 |\n" +
		"| 02 | [Two](./brief-02.md) | 1 | S | todo | \u2014 | \u2014 |\n"
	if err := os.WriteFile(filepath.Join(sdir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	brief01 := "---\n" +
		"brief: samewave/01\n" +
		"title: Brief One\n" +
		"wave: 1\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-07-08\n" +
		"sources: [test]\n" +
		"---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(sdir, "brief-01.md"), []byte(brief01), 0o644); err != nil {
		t.Fatal(err)
	}

	brief02 := "---\n" +
		"brief: samewave/02\n" +
		"title: Brief Two\n" +
		"wave: 1\n" +
		"depends: [samewave/01]\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-07-08\n" +
		"sources: [test]\n" +
		"---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(sdir, "brief-02.md"), []byte(brief02), 0o644); err != nil {
		t.Fatal(err)
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	_, notices := checkBriefFiles(streams)
	if !hasProblem(notices, "depends on samewave/01", "wave 1") {
		t.Errorf("expected same-wave-dep notice, got notices=%v", notices)
	}
}

func TestStrictlyEarlierDepPasses(t *testing.T) {
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams", "earlierdep")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}

	readme := "---\n" +
		"stream: earlierdep\n" +
		"status: active\n" +
		"priority: P1\n" +
		"track: test\n" +
		"---\n" +
		"# Earlierdep\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [Base](./brief-01.md) | 0 | S | todo | \u2014 | \u2014 |\n" +
		"| 02 | [Depends](./brief-02.md) | 1 | S | todo | \u2014 | \u2014 |\n"
	if err := os.WriteFile(filepath.Join(sdir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	brief01 := "---\n" +
		"brief: earlierdep/01\n" +
		"title: Base\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-07-08\n" +
		"sources: [test]\n" +
		"---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(sdir, "brief-01.md"), []byte(brief01), 0o644); err != nil {
		t.Fatal(err)
	}

	brief02 := "---\n" +
		"brief: earlierdep/02\n" +
		"title: Depends\n" +
		"wave: 1\n" +
		"depends: [earlierdep/01]\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-07-08\n" +
		"sources: [test]\n" +
		"---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(sdir, "brief-02.md"), []byte(brief02), 0o644); err != nil {
		t.Fatal(err)
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := checkBriefFiles(streams)
	if hasProblem(problems, "strictly-earlier") {
		t.Errorf("strictly-earlier dep should NOT trigger lint; got problems=%v", problems)
	}
}

// TestSecurityReviewAtDone covers the security-review-at-done rule: a risk-classed brief (frontmatter
// only — gate:human OR any risk answer yes) at `done` whose Reviewed cell lacks the
// literal substring "security-review" emits a NOTICE. Risk-clear briefs (all risk no,
// gate model) are exempt. Severity is NOTICE this phase — the current tree has
// risk-classed done rows with no such token, so a hard error cannot gate until
// backfill lands.
func TestSecurityReviewAtDone(t *testing.T) {
	_, notices := briefSchemaChecks(t)

	tests := []struct {
		name       string
		file       string
		wantNotice bool
	}{
		// Risk-classed (gate:human + irreversible:yes), done, no token → NOTICE.
		{"risk-classed done without token", "brief-74-risk-done-no-review.md", true},
		// Risk-classed (gate:human + irreversible:yes), done, WITH token → clean.
		{"risk-classed done with token", "brief-75-risk-done-with-review.md", false},
		// Risk-clear (gate:model, all risk no), done, no token → clean (exempt).
		{"risk-clear done without token", "brief-76-clear-done-no-review.md", false},
		// Gate:human-only (all risk no), done, no token → NOTICE (gate:human alone
		// is risk-classed).
		{"gate-human-only done without token", "brief-77-human-only-done-no-review.md", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasProblem(notices, tc.file, "security-review"); got != tc.wantNotice {
				t.Errorf("security-review notice for %s = %v, want %v; notices:\n%s", tc.file, got, tc.wantNotice, strings.Join(notices, "\n"))
			}
		})
	}
}
