package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// ---------------------------------------------------------------------------
// Fixtures shared by the three Verify rows (verdict-lane/03).
//
// The tree: an `issue-flow` stream carrying rulings.md (the R-6 sign-off line),
// and an `example` stream with one model-gate brief (brief-01) at `implemented`,
// a Class-columned Verify table with a check:ci scripted row, and an Evidence
// section to append into. A generated RSA keypair signs the verdict payload — no
// committed key material, no deskverdict-binary dependency, so the bad-signature
// clause is proven against real RS256.
// ---------------------------------------------------------------------------

const testVerifierSlug = "assay-verifier-app"
const testVerifierID = 500000123
const testHomeRepo = "example-org/tracker"

// verifyRoster arms the R-6 verify lane for tests: bless authority ada (User),
// the verifier App bound by its role slug + id, and the home repo.
func verifyRoster() map[string]string {
	return map[string]string{
		scanEnvBlessLogin:      "ada:100001",
		scanEnvTrustedLogins:   "ada:100001",
		scanEnvTrustedBotSlugs: "verifier=" + testVerifierSlug + ":" + strconv.Itoa(testVerifierID),
		scanEnvHumanLoginMap:   "alex:ada",
		scanEnvHomeRepo:        testHomeRepo,
	}
}

// r6BlessResolver arms the enactment gate: resolves any URL to the bless
// authority (ada:100001, User) with a body naming R-6.
func r6BlessResolver(url string) (authorIdentity, string, error) {
	return authorIdentity{Login: "ada", ID: 100001, Type: "User"}, "the blessing authority accepts R-6 as drafted", nil
}

const r6Empty = "**Sign-off:** _(empty — the blessing authority fills with an acceptance URL)_"
const r6Armed = "**Sign-off:** https://github.com/example-org/tracker/pull/999#issuecomment-5319580160 — the blessing authority"

// writeVerifyTree builds the fixture tree under root with the given R-6 sign-off
// line. It returns the absolute brief-01 path.
func writeVerifyTree(t *testing.T, root, r6signoff string) string {
	t.Helper()

	// issue-flow stream + rulings.md (R-6 line, R-7 empty to bound the section).
	ifDir := filepath.Join(root, "docs", "streams", "issue-flow")
	mustMkdir(t, ifDir)
	writeFileT(t, filepath.Join(ifDir, "README.md"),
		"---\nstream: issue-flow\nstatus: active\npriority: P0\ntrack: platform\n---\n\n"+
			"# Issue Flow\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n"+
			"|---|-------|------|--------|----------|----------|\n")
	writeFileT(t, filepath.Join(ifDir, "rulings.md"),
		"# Rulings\n\n## R-6 Verify verdict-transcription lane\n\nStuff.\n\n"+r6signoff+"\n\n"+
			"## R-7 Scan-transcription lane\n\n**Sign-off:** _(empty)_\n")

	// example stream: README with a model-gate brief-01 row at `implemented`.
	exDir := filepath.Join(root, "docs", "streams", "example")
	mustMkdir(t, exDir)
	writeFileT(t, filepath.Join(exDir, "README.md"),
		"---\nstream: example\nstatus: active\npriority: P1\ntrack: product\nrepo: "+testHomeRepo+"\n---\n\n"+
			"# Example\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n"+
			"|---|-------|------|--------|----------|----------|\n"+
			"| 01 | A thing | 0 | implemented |  |  |\n")

	// The check:ci scripted row must exist + be executable on the candidate tree.
	vdDir := filepath.Join(exDir, "verify.d", "brief-01")
	mustMkdir(t, vdDir)
	scriptRel := "docs/streams/example/verify.d/brief-01/row-1.sh"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(scriptRel)), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	briefPath := filepath.Join(exDir, "brief-01-a-thing.md")
	writeFileT(t, briefPath,
		"---\n"+
			"brief: example/01\n"+
			"title: \"A thing\"\n"+
			"wave: 0\n"+
			"depends: []\n"+
			"unblocks: []\n"+
			"effort: M\n"+
			"gate: model\n"+
			"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n"+
			"issues: []\n"+
			"schema: brief-v1\n"+
			"authored: 2026-08-17 by test\n"+
			"sources: []\n"+
			"---\n\n"+
			"# Brief 01\n\n## Verify\n\n"+
			"| # | Class | Command | Expect |\n"+
			"|---|-------|---------|--------|\n"+
			"| 1 | check:ci | "+scriptRel+" | exit 0 |\n\n"+
			"## Evidence\n\n"+
			"<!-- appended at implementation time -->\n")
	return briefPath
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFileT(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// signedVerdictBody builds a signed verdict issue body for one entry, signed with
// key. When result/evidence/class differ, callers exercise the per-entry clauses.
func signedVerdictBody(t *testing.T, key *rsa.PrivateKey, e verdictEntry) string {
	t.Helper()
	pl := verdictPayload{
		Schema:  "verdict-v1",
		Repo:    testHomeRepo,
		TS:      "2026-08-17T20:30:00Z",
		Head:    "3f500360abc1234567890def",
		Entries: []verdictEntry{e},
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := vvCanonicalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := vvSignCanonical(canonical, key)
	if err != nil {
		t.Fatal(err)
	}
	return vvAssembleBody(canonical, sig)
}

// validEntry is a clean PASS entry for brief-01 row 1 (check:ci).
func validEntry() verdictEntry {
	return verdictEntry{
		Brief:    "docs/streams/example/brief-01-a-thing.md",
		Row:      1,
		Class:    "check:ci",
		Result:   "PASS",
		Evidence: "| 1 | check:ci | `row-1.sh` | exit 0 | 2026-08-17 | assay-verifier-app[bot] |",
	}
}

// verifierAuthorResolver returns the bound verifier App identity for the home
// repo issue, and errFixture for anything else (fail closed).
func verifierAuthorResolver(issue int) authorResolver {
	return func(repo string, n int) (authorIdentity, error) {
		if repo == testHomeRepo && n == issue {
			return authorIdentity{Login: testVerifierSlug + "[bot]", ID: testVerifierID, Type: "Bot"}, nil
		}
		return authorIdentity{}, errFixture
	}
}

// notEditedResolver reports every body as un-edited.
func notEditedResolver(repo string, issue int) (bool, error) { return false, nil }

// passRowExec is a rowExecutor that reports a green re-execution.
func passRowExec(root, command string) runResult { return runResult{exit: 0} }

// realSigVerifier verifies against pub — the real RS256 path.
func realSigVerifier(pub *rsa.PublicKey) verdictSigVerifier {
	return func(body string) (vvVerifyState, string) { return vvVerifyBody(body, pub) }
}

// singleIssueLister lists exactly one open verdict issue (number 42) with body.
func singleIssueLister(body string) verdictIssueLister {
	return func(repo string) ([]verdictIssue, error) {
		return []verdictIssue{{Number: 42, Title: "verify verdict batch", Body: body, CreatedAt: "2026-08-17T20:30:05Z"}}, nil
	}
}

func genKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// ---------------------------------------------------------------------------
// Enactment gate (R-6) — quick invariants.
// ---------------------------------------------------------------------------

func TestR6EnactmentEmptyIsInert(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Empty)
	armed, reason := transcribeVerifyEnactmentGate(root, r6BlessResolver)
	if armed {
		t.Fatalf("empty R-6 sign-off must be INERT, got armed (%s)", reason)
	}
	if !strings.Contains(reason, "empty") {
		t.Errorf("reason should name the empty sign-off, got %q", reason)
	}
}

func TestR6EnactmentArmedByBless(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Armed)
	armed, reason := transcribeVerifyEnactmentGate(root, r6BlessResolver)
	if !armed {
		t.Fatalf("a bless-authority sign-off naming R-6 must arm the lane, got INERT (%s)", reason)
	}
}

func TestR6EnactmentNonBlessRefused(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Armed)
	cases := []struct {
		name string
		res  commentResolver
	}{
		{"recycled-id", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "ada", ID: 999, Type: "User"}, "R-6 ok", nil
		}},
		{"non-user", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "ada", ID: 100001, Type: "Bot"}, "R-6 ok", nil
		}},
		{"wrong-login", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "mallory", ID: 100001, Type: "User"}, "R-6 ok", nil
		}},
		{"body-omits-r6", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "ada", ID: 100001, Type: "User"}, "looks good to me", nil
		}},
	}
	for _, tc := range cases {
		if armed, reason := transcribeVerifyEnactmentGate(root, tc.res); armed {
			t.Errorf("%s: must be INERT, got armed (%s)", tc.name, reason)
		}
	}
}

func TestFindR6SignoffScopedToR6(t *testing.T) {
	md := "## R-5 x\n**Sign-off:** https://example/5\n\n" +
		"## R-6 y\n**Sign-off:** https://example/6\n\n" +
		"## R-7 z\n**Sign-off:** https://example/7\n"
	line, ok := findR6SignoffLine(md)
	if !ok {
		t.Fatal("expected to find the R-6 sign-off line")
	}
	if !strings.Contains(line, "/6") {
		t.Errorf("R-6 line should carry the R-6 URL, not a neighbour's: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Verify Row 1: a VALID signed verdict → all clauses pass, planned diff matches
// the golden (Evidence appended, README status flipped implemented→verified,
// frontmatter + Verify byte-identical).
// ---------------------------------------------------------------------------

func TestRow1ValidVerdictTranscribes(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	briefPath := writeVerifyTree(t, root, r6Armed)
	key := genKey(t)
	body := signedVerdictBody(t, key, validEntry())

	beforeBrief := readFile(t, briefPath)
	readmePath := filepath.Join(root, "docs", "streams", "example", "README.md")

	got := captureRun(t, func() int {
		return runTranscribeVerify(root, false,
			singleIssueLister(body), verifierAuthorResolver(42), notEditedResolver,
			realSigVerifier(&key.PublicKey), r6BlessResolver, passRowExec)
	})
	if got.code != 0 {
		t.Fatalf("valid verdict run exit %d, stderr=%s", got.code, got.err)
	}
	if !strings.Contains(got.log, "flip implemented→verified") {
		t.Errorf("expected an implemented→verified flip in the report, got:\n%s", got.log)
	}

	// Golden: Evidence appended.
	afterBrief := readFile(t, briefPath)
	if !strings.Contains(afterBrief, "assay-verifier-app[bot]") || strings.Count(afterBrief, "| 1 | check:ci |") < 2 {
		t.Errorf("Evidence row was not appended to the brief:\n%s", afterBrief)
	}
	// Golden: frontmatter + Verify byte-identical (cl.4).
	if fmBefore, _ := frontmatterBytes(beforeBrief); fmBefore != mustFrontmatter(t, afterBrief) {
		t.Errorf("cl.4 violated: frontmatter changed")
	}
	if sectionBody(beforeBrief, "Verify") != sectionBody(afterBrief, "Verify") {
		t.Errorf("cl.4 violated: Verify section changed")
	}
	// Golden: README status flipped to verified with a Verified stamp.
	readme := readFile(t, readmePath)
	if !strings.Contains(readme, "| verified |") && !strings.Contains(readme, "verified") {
		t.Errorf("README status was not flipped to verified:\n%s", readme)
	}
	if !strings.Contains(readme, "verdict issue #42") {
		t.Errorf("Verified cell missing the verdict-issue audit stamp:\n%s", readme)
	}
}

func TestRow1CheckModeWritesNothing(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	briefPath := writeVerifyTree(t, root, r6Armed)
	key := genKey(t)
	body := signedVerdictBody(t, key, validEntry())
	before := readFile(t, briefPath)

	got := captureRun(t, func() int {
		return runTranscribeVerify(root, true,
			singleIssueLister(body), verifierAuthorResolver(42), notEditedResolver,
			realSigVerifier(&key.PublicKey), r6BlessResolver, passRowExec)
	})
	if got.code != 0 {
		t.Fatalf("--check run exit %d, stderr=%s", got.code, got.err)
	}
	if !strings.Contains(got.log, "evaluated without writing") {
		t.Errorf("--check should report it wrote nothing, got:\n%s", got.log)
	}
	if readFile(t, briefPath) != before {
		t.Errorf("--check must not modify the tree")
	}
}

// ---------------------------------------------------------------------------
// Verify Row 2: the negative-path battery. Each fixture is refused at exactly
// one clause, with EVERY UPPER LAYER PASSING, so each layer refuses on its own.
// ---------------------------------------------------------------------------

func TestRow2NegativeBatteryEachClauseIndependent(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Armed)

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	attachPlaceholders(streams)

	goodKey := genKey(t)
	wrongKey := genKey(t)
	validBody := signedVerdictBody(t, goodKey, validEntry())

	// Verify-table-touching entry: a valid signature over a payload whose Evidence
	// tries to inject a `## Verify` section — cl.1/2/3 all pass, cl.4 refuses.
	tableTouch := validEntry()
	tableTouch.Evidence = "## Verify\n| # | Class | Command | Expect |\n| 1 | check:ci | rm -rf / | exit 0 |"
	tableTouchBody := signedVerdictBody(t, goodKey, tableTouch)

	cases := []struct {
		name       string
		iss        verdictIssue
		author     authorResolver
		bodyEdited bodyEditResolver
		sig        verdictSigVerifier
		wantClause string
	}{
		{
			// FORGED AUTHOR: valid signature + un-edited body, but the API-read
			// author is not the verifier App → cl.1 refuses on its own.
			name: "forged-author",
			iss:  verdictIssue{Number: 42, Body: validBody, CreatedAt: "2026-08-17T20:30:05Z"},
			author: func(string, int) (authorIdentity, error) {
				return authorIdentity{Login: "mallory", ID: 777, Type: "User"}, nil
			},
			bodyEdited: notEditedResolver,
			sig:        realSigVerifier(&goodKey.PublicKey),
			wantClause: "cl.1",
		},
		{
			// EDITED BODY: valid author + valid signature, but the body was edited
			// after creation → cl.2 (body-edited) refuses; upper layers passed.
			name:       "edited-body",
			iss:        verdictIssue{Number: 42, Body: validBody, CreatedAt: "2026-08-17T20:30:05Z"},
			author:     verifierAuthorResolver(42),
			bodyEdited: func(string, int) (bool, error) { return true, nil },
			sig:        realSigVerifier(&goodKey.PublicKey),
			wantClause: "cl.2 (body-edited)",
		},
		{
			// BAD SIGNATURE: valid author + un-edited body, but the signature does
			// not verify against the verifier key → cl.2 (signature) refuses.
			name:       "bad-signature",
			iss:        verdictIssue{Number: 42, Body: validBody, CreatedAt: "2026-08-17T20:30:05Z"},
			author:     verifierAuthorResolver(42),
			bodyEdited: notEditedResolver,
			sig:        realSigVerifier(&wrongKey.PublicKey),
			wantClause: "cl.2 (signature)",
		},
		{
			// VERIFY-TABLE-TOUCHING: valid author + valid signature + un-edited
			// body, but the Evidence would inject a `## Verify` section → cl.4.
			name:       "verify-table-touching",
			iss:        verdictIssue{Number: 42, Body: tableTouchBody, CreatedAt: "2026-08-17T20:30:05Z"},
			author:     verifierAuthorResolver(42),
			bodyEdited: notEditedResolver,
			sig:        realSigVerifier(&goodKey.PublicKey),
			wantClause: "cl.4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			applies, skip := evaluateVerdict(root, tc.iss, streams, testHomeRepo,
				tc.author, tc.bodyEdited, tc.sig, passRowExec)
			if applies != nil {
				t.Fatalf("%s: must transcribe NOTHING, got %d applies", tc.name, len(applies))
			}
			if skip == nil {
				t.Fatalf("%s: expected a refusal, got none", tc.name)
			}
			if !strings.Contains(skip.Clause, tc.wantClause) {
				t.Errorf("%s: refused at %q, want clause %q — reason: %s", tc.name, skip.Clause, tc.wantClause, skip.Reason)
			}
		})
	}
}

// TestRow2CIReExecMismatchRefuses proves cl.6: a check:ci row claiming PASS whose
// re-execution comes back non-zero refuses the whole verdict, independently of
// the author/signature layers (which pass here).
func TestRow2CIReExecMismatchRefuses(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Armed)
	streams, _, _ := loadStreams(root)
	attachPlaceholders(streams)

	key := genKey(t)
	body := signedVerdictBody(t, key, validEntry())
	iss := verdictIssue{Number: 42, Body: body, CreatedAt: "2026-08-17T20:30:05Z"}

	failExec := func(root, command string) runResult { return runResult{exit: 1} }
	applies, skip := evaluateVerdict(root, iss, streams, testHomeRepo,
		verifierAuthorResolver(42), notEditedResolver, realSigVerifier(&key.PublicKey), failExec)
	if applies != nil || skip == nil || !strings.Contains(skip.Clause, "cl.6") {
		t.Fatalf("a check:ci PASS that re-executes to non-zero must refuse at cl.6; got applies=%v skip=%+v", applies, skip)
	}

	// couldNotRun (no hermetic sandbox) is ALSO a refusal — never a silent pass.
	cnrExec := func(root, command string) runResult {
		return runResult{exit: -1, couldNotRun: true, reason: "no sandbox"}
	}
	applies2, skip2 := evaluateVerdict(root, iss, streams, testHomeRepo,
		verifierAuthorResolver(42), notEditedResolver, realSigVerifier(&key.PublicKey), cnrExec)
	if applies2 != nil || skip2 == nil || !strings.Contains(skip2.Clause, "cl.6") {
		t.Fatalf("a check:ci that could-not-run must refuse at cl.6; got applies=%v skip=%+v", applies2, skip2)
	}
}

// ---------------------------------------------------------------------------
// Verify Row 3: an empty / non-bless sign-off → the lane is INERT and evaluates
// NO clause (the lister/verifier are never even consulted).
// ---------------------------------------------------------------------------

func TestRow3EmptySignoffIsInertEvaluatesNoClause(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Empty)
	key := genKey(t)
	body := signedVerdictBody(t, key, validEntry())

	// A lister that FAILS the test if it is ever called — an inert lane must not
	// list issues at all.
	trap := func(repo string) ([]verdictIssue, error) {
		t.Errorf("INERT lane must not list issues")
		return []verdictIssue{{Number: 42, Body: body}}, nil
	}
	got := captureRun(t, func() int {
		return runTranscribeVerify(root, true,
			trap, verifierAuthorResolver(42), notEditedResolver,
			realSigVerifier(&key.PublicKey), r6BlessResolver, passRowExec)
	})
	if got.code != 0 {
		t.Fatalf("inert run exit %d, stderr=%s", got.code, got.err)
	}
	if !strings.Contains(got.log, "INERT") || !strings.Contains(got.log, "evaluating no clause") {
		t.Errorf("expected an INERT report evaluating no clause, got:\n%s", got.log)
	}
}

// ---------------------------------------------------------------------------
// Run-level clause gates: cl.9 (unconsumed-verdict tripwire), cl.10 (ci-failure
// hold). Both armed, both refusing before any transcription.
// ---------------------------------------------------------------------------

func TestCl9UnconsumedTripwireRefuses(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Armed)
	key := genKey(t)
	body := signedVerdictBody(t, key, validEntry())

	many := func(repo string) ([]verdictIssue, error) {
		var out []verdictIssue
		for i := 0; i < transcribeUnconsumedCap+1; i++ {
			out = append(out, verdictIssue{Number: 100 + i, Body: body, CreatedAt: "2026-08-17T20:30:05Z"})
		}
		return out, nil
	}
	got := captureRun(t, func() int {
		return runTranscribeVerify(root, true,
			many, verifierAuthorResolver(42), notEditedResolver,
			realSigVerifier(&key.PublicKey), r6BlessResolver, passRowExec)
	})
	if got.code != 3 {
		t.Fatalf("cl.9 tripwire must exit 3, got %d (stdout=%s)", got.code, got.log)
	}
	if !strings.Contains(got.log, "TRIPWIRE (cl.9)") {
		t.Errorf("expected a cl.9 tripwire message, got:\n%s", got.log)
	}
}

func TestCl10CIFailureHoldsLane(t *testing.T) {
	scanWithRoster(t, verifyRoster())
	root := t.TempDir()
	writeVerifyTree(t, root, r6Armed)
	key := genKey(t)
	body := signedVerdictBody(t, key, validEntry())

	// One verdict issue plus a fresh ci-failure issue → the lane holds.
	withCIFailure := func(repo string) ([]verdictIssue, error) {
		return []verdictIssue{
			{Number: 42, Body: body, CreatedAt: "2026-08-17T20:30:05Z"},
			{Number: 7, Title: "ci-failure: main is red", CreatedAt: nowRFC3339()},
		}, nil
	}
	got := captureRun(t, func() int {
		return runTranscribeVerify(root, false,
			withCIFailure, verifierAuthorResolver(42), notEditedResolver,
			realSigVerifier(&key.PublicKey), r6BlessResolver, passRowExec)
	})
	if got.code != 0 {
		t.Fatalf("cl.10 hold should exit 0 (held, not error), got %d", got.code)
	}
	if !strings.Contains(got.log, "HELD (cl.10") {
		t.Errorf("expected a cl.10 main-health hold, got:\n%s", got.log)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustFrontmatter(t *testing.T, s string) string {
	t.Helper()
	fm, err := frontmatterBytes(s)
	if err != nil {
		t.Fatal(err)
	}
	return fm
}
