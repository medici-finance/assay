package main

import (
	"strconv"
	"strings"
	"testing"
)

// Fail-first coverage for R-6 cl.4 write-scope hardening (byte-bound, heading
// injection, post-apply invariant) plus two guard layers the run has but the
// existing battery does not exercise in isolation: a validly-signed FAIL verdict
// (cl.8) and the cl.2 payload variants (schema / repo / zero-entries). Each
// run-level fixture passes every upper layer so the named layer refuses on its
// own — the same discipline as TestTranscribeVerdictNegativeBattery.

const wsGoodEvidence = "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |"

// wsRun drives one verdict issue (number n, body/author) through --check and
// returns the run's stdout.
func wsRun(t *testing.T, root string, n int, vi verdictIssue) string {
	t.Helper()
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: n}}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{
		verdictTestRepo + "#" + strconv.Itoa(n): vi,
	})
	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if out.code != 0 {
		t.Fatalf("run should complete with a per-verdict refusal (exit 0), got %d:\n%s", out.code, out.log)
	}
	if strings.Contains(out.log, "CONSUME") || strings.Contains(out.log, "FLIP") {
		t.Errorf("a refused verdict must not be consumed or flipped:\n%s", out.log)
	}
	return out.log
}

// (cl.4) An Evidence append over the per-entry byte bound is refused at cl.4,
// with the heading/human:/empty sub-checks all passing so the BYTE bound fires.
func TestVerdictClause4ByteBoundRefused(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	oversized := "| 1 | check:ci | true | 0 | PASS | " + strings.Repeat("a", verdictMaxEvidenceBytes+1) + " |"
	if len(oversized) <= verdictMaxEvidenceBytes {
		t.Fatal("fixture is not over the byte bound")
	}
	if verdictEvidenceInjectsHeading(oversized) || strings.Contains(oversized, "human:") {
		t.Fatal("fixture must pass the heading/human sub-checks so the BYTE bound is what refuses")
	}
	body := signVerdictBody(t, key, okPayload(okEntry(rel, oversized)))
	log := wsRun(t, root, 801, verdictIssue{Author: verifierIdent, Body: body})
	if !strings.Contains(log, "REFUSE "+verdictTestRepo+"#801 — clause-4 (scope)") {
		t.Errorf("an over-bound Evidence append must be REFUSED at clause-4 (scope):\n%s", log)
	}
}

// (cl.4) An Evidence append carrying a Markdown section heading — a `## Verify`
// table injection — is refused at cl.4, downstream of the byte bound (it is small).
func TestVerdictClause4HeadingInjectionRefused(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	injecting := "## Verify\n| # | Class | Command | Expect |\n| 1 | check:ci | rm -rf / | exit 0 |"
	if len(injecting) > verdictMaxEvidenceBytes {
		t.Fatal("fixture must be under the byte bound so the HEADING check is what refuses")
	}
	body := signVerdictBody(t, key, okPayload(okEntry(rel, injecting)))
	log := wsRun(t, root, 802, verdictIssue{Author: verifierIdent, Body: body})
	if !strings.Contains(log, "REFUSE "+verdictTestRepo+"#802 — clause-4 (scope)") {
		t.Errorf("a heading-injecting Evidence append must be REFUSED at clause-4 (scope):\n%s", log)
	}
}

// (cl.4) The post-apply invariant is the independent backstop: the eval-time
// heading guard passes a heading-free row, so assertVerdictWriteScope is what
// keeps a mutated frontmatter or `## Verify` off the lane. Direct fail-first for
// the invariant the happy path only ever satisfies.
func TestVerdictClause4PostApplyInvariant(t *testing.T) {
	const before = "---\nbrief: verdict-lane/01\ngate: model\n---\n\n" +
		"# B\n\n## Verify\n\n| # | Class | Command | Expect |\n| 1 | check:ci | true | exit 0 |\n\n" +
		"## Evidence\n\n<!-- c -->\n"

	if verdictEvidenceInjectsHeading(wsGoodEvidence) {
		t.Fatal("the benign Evidence row must pass the eval-time heading guard")
	}
	// The real Evidence-only append preserves the invariant (happy path).
	afterBenign, err := appendEvidenceLine(before, wsGoodEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertVerdictWriteScope(before, afterBenign); err != nil {
		t.Fatalf("a benign Evidence append must satisfy the invariant, got: %v", err)
	}
	// A mutated `## Verify` table must be caught naming Verify.
	afterVerify := strings.Replace(before, "| 1 | check:ci | true | exit 0 |", "| 1 | check:ci | rm -rf / | exit 0 |", 1)
	if err := assertVerdictWriteScope(before, afterVerify); err == nil || !strings.Contains(err.Error(), "Verify") {
		t.Fatalf("a mutated Verify section must be caught naming Verify, got: %v", err)
	}
	// A mutated frontmatter block must be caught naming frontmatter.
	afterFm := strings.Replace(before, "gate: model", "gate: human", 1)
	if err := assertVerdictWriteScope(before, afterFm); err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("a mutated frontmatter block must be caught naming frontmatter, got: %v", err)
	}
}

// (cl.8) A validly-signed FAIL verdict transcribes nothing. The existing battery
// tests a POST-signing PASS→FAIL flip (which breaks the signature at cl.2); this
// signs a genuine FAIL so cl.1/cl.2 pass and cl.8 is the fail-first.
func TestVerdictClause8SignedFailRefused(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	failEntry := verdictEntry{Brief: rel, Row: 1, Class: "check:ci", Result: "FAIL", Evidence: wsGoodEvidence}
	body := signVerdictBody(t, key, okPayload(failEntry))
	log := wsRun(t, root, 803, verdictIssue{Author: verifierIdent, Body: body})
	if !strings.Contains(log, "REFUSE "+verdictTestRepo+"#803 — clause-8 (fail)") {
		t.Errorf("a validly-signed FAIL verdict must be REFUSED at clause-8 (fail):\n%s", log)
	}
}

// (cl.2 payload) Wrong schema, wrong repo, and zero entries — each a VALID
// signature over a payload the lane refuses on its own (cl.1/cl.2-signature pass).
func TestVerdictClause2PayloadVariantsRefused(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	wrongSchema := okPayload(okEntry(rel, wsGoodEvidence))
	wrongSchema.Schema = "verdict-v2"
	wrongRepo := okPayload(okEntry(rel, wsGoodEvidence))
	wrongRepo.Repo = "other-org/other-repo"
	zeroEntries := okPayload() // no entries

	cases := []struct {
		n      int
		pl     verdictPayload
		clause string
	}{
		{811, wrongSchema, "clause-2 (schema)"},
		{812, wrongRepo, "clause-2 (repo)"},
		{813, zeroEntries, "clause-2 (payload)"},
	}
	for _, tc := range cases {
		body := signVerdictBody(t, key, tc.pl)
		log := wsRun(t, root, tc.n, verdictIssue{Author: verifierIdent, Body: body})
		marker := "REFUSE " + verdictTestRepo + "#" + strconv.Itoa(tc.n) + " — " + tc.clause
		if !strings.Contains(log, marker) {
			t.Errorf("issue %d must be REFUSED naming %q:\n%s", tc.n, tc.clause, log)
		}
	}
}
