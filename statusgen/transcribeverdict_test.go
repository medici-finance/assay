package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Signing + fixture helpers
// ---------------------------------------------------------------------------

// verdictTestKey generates a throwaway RSA keypair for the test and exports its
// PKIX public PEM into ASSAY_VERIFIER_PUBKEY, so verdictResolvePubkey resolves it
// through the real code path. The private half signs the fixture bodies.
func verdictTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	t.Setenv(verdictPubkeyVar, string(pubPEM))
	return key
}

// signVerdictBody canonicalises pl, signs the canonical bytes with key (RS256), and
// assembles the issue-body block the transcriber reads — the SAME shape deskverdict
// sign emits.
func signVerdictBody(t *testing.T, key *rsa.PrivateKey, pl verdictPayload) string {
	t.Helper()
	raw, err := json.Marshal(pl)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := verdictCanonicalizeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(sig)
	return "```" + verdictFenceTag + "\n" + string(canonical) + "\n```\n\n" +
		"<!-- " + verdictSigMarker + " v1 alg=RS256 sig=" + b64 + " -->\n"
}

const verdictTestRepo = "example-org/tracker"

func verdictRoster() map[string]string {
	return map[string]string{
		scanEnvBlessLogin:      "ada:100001",
		scanEnvTrustedLogins:   "ada:100001",
		scanEnvTrustedBotSlugs: "verifier=assay-verifier-app:300000009,worker=assay-worker-app:300000006",
		scanEnvHomeRepo:        verdictTestRepo,
		scanEnvScanRepos:       verdictTestRepo,
	}
}

// verifierIdent is the API-read identity of the verifier App the roster binds.
var verifierIdent = authorIdentity{Login: "assay-verifier-app[bot]", ID: 300000009, Type: "Bot"}

// blessR6Resolver arms the R-6 enactment gate: any URL resolves to the bless
// authority (ada:100001, User) with a body naming R-6.
func blessR6Resolver(string) (authorIdentity, string, error) {
	return authorIdentity{Login: "ada", ID: 100001, Type: "User"}, "the blessing authority accepts R-6 as drafted", nil
}

// fixtureVerdictResolver returns a verdictIssueResolver keyed by "repo#issue".
func fixtureVerdictResolver(data map[string]verdictIssue) verdictIssueResolver {
	return func(repo string, issue int) (verdictIssue, error) {
		if v, ok := data[repo+"#"+strconv.Itoa(issue)]; ok {
			return v, nil
		}
		return verdictIssue{}, errFixture
	}
}

func passCheckCI(string, string) checkCIResult { return checkCIResult{Passed: true, Reason: "exit 0"} }
func failCheckCI(string, string) checkCIResult {
	return checkCIResult{Passed: false, Reason: "exit 1"}
}
func noHealthHold() (bool, string, error) { return false, "", nil }

const r6Empty = "**Sign-off:** _(empty — kryton fills with an acceptance URL)_"
const r6Armed = "**Sign-off:** https://github.com/example-org/tracker/pull/999#issuecomment-5319580160 — kryton"

// writeVerdictRulings drops an issue-flow stream (README + rulings.md) carrying the
// given R-6 sign-off line. Scoped so a filled R-7 line below cannot arm the lane.
func writeVerdictRulings(t *testing.T, root, r6Signoff string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "issue-flow")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "---\nstream: issue-flow\nstatus: active\npriority: P0\ntrack: platform\n---\n\n" +
		"# Issue Flow\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|----------|----------|\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "# Rulings\n\n## R-6 Verify verdict-transcription lane\n\nStuff.\n\n" + r6Signoff + "\n\n" +
		"## R-7 Scan-transcription lane\n\n**Sign-off:** https://github.com/o/r/pull/1#issuecomment-1 — kryton\n"
	if err := os.WriteFile(filepath.Join(dir, "rulings.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// verdictBrief writes a brief-v1 file + its README row into a `verdict-lane` stream.
// gate and irreversible are parameterised; the brief carries one check:ci Verify row
// and an Evidence section, at README status `implemented`.
func writeVerdictBrief(t *testing.T, root, num, gate string, irreversible bool) string {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "verdict-lane")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	irr := "no"
	if irreversible {
		irr = "yes"
	}
	fname := "brief-" + num + "-fixture.md"
	brief := "---\n" +
		"brief: verdict-lane/" + num + "\n" +
		"title: Fixture brief " + num + "\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: M\n" +
		"gate: " + gate + "\n" +
		"risk: {regulatory: no, customer: no, irreversible: " + irr + ", sensitive-data: no}\n" +
		"issues: []\n" +
		"schema: brief-v1\n" +
		"authored: 2026-08-17 by fixture\n" +
		"sources: [\"fixture\"]\n" +
		"---\n\n" +
		"# Brief " + num + "\n\n" +
		"## Verify\n" +
		"| # | Class | Command | Expect |\n" +
		"|---|-------|---------|--------|\n" +
		"| 1 | check:ci | true | exit 0 |\n\n" +
		"## Evidence\n" +
		"<!-- appended at implementation time -->\n\n" +
		"## Review\nGate: " + gate + ".\n"
	if err := os.WriteFile(filepath.Join(dir, fname), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	// README with a row at `implemented` for this brief.
	readmePath := filepath.Join(dir, "README.md")
	existing, _ := os.ReadFile(readmePath)
	if len(existing) == 0 {
		hdr := "---\nstream: verdict-lane\nstatus: active\npriority: P0\ntrack: platform\n---\n\n" +
			"# Verdict Lane\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n" +
			"|---|-------|------|--------|----------|----------|\n"
		existing = []byte(hdr)
	}
	row := "| " + num + " | [Fixture](" + fname + ") | 0 | implemented | — | — |\n"
	if err := os.WriteFile(readmePath, append(existing, []byte(row)...), 0o644); err != nil {
		t.Fatal(err)
	}
	return "docs/streams/verdict-lane/" + fname
}

// verdictBaseRepo builds a loadable repo: the alpha stream (copied from goodrepo),
// the issue-flow rulings, and returns the root. The caller adds verdict-lane briefs.
func verdictBaseRepo(t *testing.T, r6Signoff string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	writeVerdictRulings(t, root, r6Signoff)
	return root
}

// okEntry builds a PASS entry naming briefRel row 1.
func okEntry(briefRel, evidence string) verdictEntry {
	return verdictEntry{Brief: briefRel, Row: 1, Class: "check:ci", Result: "PASS", Evidence: evidence}
}

func okPayload(entries ...verdictEntry) verdictPayload {
	return verdictPayload{Schema: "verdict-v1", Repo: verdictTestRepo, TS: "2026-08-17T20:30:00Z", Head: "3f500360", Entries: entries}
}

// ---------------------------------------------------------------------------
// Canonicalisation
// ---------------------------------------------------------------------------

func TestVerdictCanonicalizeDeterministicAndIdempotent(t *testing.T) {
	// Key order is normalised, whitespace stripped, `<`/`>`/`&` NOT escaped.
	a := []byte(`{"b":1,"a":"x<y>&z","n":10}`)
	b := []byte(` { "a" : "x<y>&z" , "n" : 10 , "b" : 1 } `)
	ca, err := verdictCanonicalizeJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := verdictCanonicalizeJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ca) != string(cb) {
		t.Errorf("canonical forms differ:\n%s\n%s", ca, cb)
	}
	want := `{"a":"x<y>&z","b":1,"n":10}`
	if string(ca) != want {
		t.Errorf("canonical = %s, want %s (keys sorted, no HTML-escape)", ca, want)
	}
	// Idempotent.
	cc, err := verdictCanonicalizeJSON(ca)
	if err != nil {
		t.Fatal(err)
	}
	if string(cc) != string(ca) {
		t.Errorf("not idempotent: %s vs %s", cc, ca)
	}
	// Trailing data is rejected.
	if _, err := verdictCanonicalizeJSON([]byte(`{}{}`)); err == nil {
		t.Error("trailing data after the JSON value must be rejected")
	}
}

// ---------------------------------------------------------------------------
// Enactment gate (R-6)
// ---------------------------------------------------------------------------

func TestVerdictEnactmentGateEmptyIsInert(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	root := t.TempDir()
	writeVerdictRulings(t, root, r6Empty)
	armed, reason := transcribeVerdictEnactmentGate(root, blessR6Resolver)
	if armed {
		t.Fatalf("empty R-6 sign-off must be INERT, got armed (%s)", reason)
	}
	if !strings.Contains(reason, "empty") {
		t.Errorf("reason should name the empty sign-off, got %q", reason)
	}
}

func TestVerdictEnactmentGateArmedByBless(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	root := t.TempDir()
	writeVerdictRulings(t, root, r6Armed)
	armed, reason := transcribeVerdictEnactmentGate(root, blessR6Resolver)
	if !armed {
		t.Fatalf("a bless-authority sign-off naming R-6 must arm the lane, got INERT (%s)", reason)
	}
}

func TestVerdictEnactmentGateNonBlessRefused(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	root := t.TempDir()
	writeVerdictRulings(t, root, r6Armed)
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
			return authorIdentity{Login: "ada", ID: 100001, Type: "User"}, "looks good", nil
		}},
	}
	for _, tc := range cases {
		if armed, reason := transcribeVerdictEnactmentGate(root, tc.res); armed {
			t.Errorf("%s: must be INERT, got armed (%s)", tc.name, reason)
		}
	}
}

func TestFindR6SignoffLineScopedToR6(t *testing.T) {
	md := "## R-6 x\n**Sign-off:** _(empty)_\n" +
		"## R-7 y\n**Sign-off:** https://github.com/o/r/pull/1#issuecomment-1 — kryton\n"
	line, ok := findR6SignoffLine(md)
	if !ok {
		t.Fatal("expected to find the R-6 sign-off line")
	}
	if firstHTTPSURL(line) != "" {
		t.Errorf("R-6 line is empty; a filled R-7 line must not leak in: %q", line)
	}
}

// ---------------------------------------------------------------------------
// Run: INERT (Verify row 3)
// ---------------------------------------------------------------------------

func TestTranscribeVerdictInertEvaluatesNoClause(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Empty)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	body := signVerdictBody(t, key, okPayload(okEntry(rel, "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |")))
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: 501}}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{
		verdictTestRepo + "#501": {Author: verifierIdent, Body: body},
	})

	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, false, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if !strings.Contains(out.log, "INERT") {
		t.Errorf("expected INERT report, got:\n%s", out.log)
	}
	if out.code != 0 {
		t.Errorf("INERT must exit 0, got %d", out.code)
	}
	// INERT writes nothing — Evidence section unchanged.
	b, _ := os.ReadFile(filepath.Join(root, rel))
	if strings.Contains(string(b), "verifier |") {
		t.Error("INERT lane must not append Evidence")
	}
}

// ---------------------------------------------------------------------------
// Run: valid signed verdict consumed (Verify row 1)
// ---------------------------------------------------------------------------

func TestTranscribeVerdictValidSignatureConsumed(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	evLine := "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |"
	body := signVerdictBody(t, key, okPayload(okEntry(rel, evLine)))
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: 601}}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{
		verdictTestRepo + "#601": {Author: verifierIdent, Body: body},
	})

	// --check: report only, no writes.
	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if out.code != 0 {
		t.Fatalf("armed --check must exit 0, got %d:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.log, "CONSUME "+verdictTestRepo+"#601") {
		t.Errorf("expected a CONSUME for 601, got:\n%s", out.log)
	}
	if !strings.Contains(out.log, "FLIP verdict-lane/01 implemented→verified") {
		t.Errorf("expected a model-tier FLIP, got:\n%s", out.log)
	}
	// --check writes nothing.
	b, _ := os.ReadFile(filepath.Join(root, rel))
	if strings.Contains(string(b), "verifier |") {
		t.Error("--check must not append Evidence")
	}

	// Real run: applies.
	out2 := captureRun(t, func() int {
		return runTranscribeVerdict(root, false, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if out2.code != 0 {
		t.Fatalf("armed run must exit 0, got %d:\n%s", out2.code, out2.log)
	}
	b2, _ := os.ReadFile(filepath.Join(root, rel))
	if !strings.Contains(string(b2), evLine) {
		t.Errorf("Evidence line not appended to the brief:\n%s", b2)
	}
	// Frontmatter + Verify section byte-identical (cl.4): the Verify row survives.
	if !strings.Contains(string(b2), "| 1 | check:ci | true | exit 0 |") {
		t.Error("cl.4: the Verify table must be byte-identical after transcription")
	}
	readme, _ := os.ReadFile(filepath.Join(root, "docs/streams/verdict-lane/README.md"))
	if !strings.Contains(string(readme), "| verified |") {
		t.Errorf("README status must flip to verified:\n%s", readme)
	}
	if !strings.Contains(string(readme), "assay-verifier-app[bot] (verdict "+verdictTestRepo+"#601)") {
		t.Errorf("Verified cell must carry the verifier + verdict ref:\n%s", readme)
	}
}

// gate:human briefs get Evidence appended but are NEVER flipped by this lane (cl.5).
func TestTranscribeVerdictHumanGateEvidenceOnlyNoFlip(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "02", "human", false)

	evLine := "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |"
	body := signVerdictBody(t, key, okPayload(okEntry(rel, evLine)))
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: 602}}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{
		verdictTestRepo + "#602": {Author: verifierIdent, Body: body},
	})

	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, false, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if out.code != 0 {
		t.Fatalf("run must exit 0, got %d:\n%s", out.code, out.log)
	}
	if strings.Contains(out.log, "FLIP") {
		t.Errorf("a gate:human brief must NOT be flipped by this lane:\n%s", out.log)
	}
	b, _ := os.ReadFile(filepath.Join(root, rel))
	if !strings.Contains(string(b), evLine) {
		t.Error("gate:human brief should still get its Evidence appended")
	}
	readme, _ := os.ReadFile(filepath.Join(root, "docs/streams/verdict-lane/README.md"))
	if strings.Contains(string(readme), "| verified |") {
		t.Error("gate:human README row must stay implemented (verify-gate flips it, not this lane)")
	}
}

// ---------------------------------------------------------------------------
// Run: negative-path battery (Verify row 2) — every layer refused independently
// ---------------------------------------------------------------------------

func TestTranscribeVerdictNegativeBattery(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	relModel := writeVerdictBrief(t, root, "01", "model", false)
	relIrrev := writeVerdictBrief(t, root, "03", "model", true)

	goodEv := "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |"

	// 701 — forged author (worker App, not the verifier). Body is validly signed.
	forgedBody := signVerdictBody(t, key, okPayload(okEntry(relModel, goodEv)))
	// 702 — tampered signature: valid body with PASS flipped to FAIL post-signing.
	tampered := strings.Replace(forgedBody, "PASS", "FAIL", 1)
	// 703 — edited body (timeline).
	editedBody := signVerdictBody(t, key, okPayload(okEntry(relModel, goodEv)))
	// 704 — irreversible brief.
	irrevBody := signVerdictBody(t, key, okPayload(okEntry(relIrrev, goodEv)))
	// 705 — human: stamp in the Evidence.
	humanBody := signVerdictBody(t, key, okPayload(okEntry(relModel, "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | human:alex |")))
	// 706 — check:ci re-execution disagrees with the verdict.
	mismatchBody := signVerdictBody(t, key, okPayload(okEntry(relModel, goodEv)))

	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {
		{Number: 701}, {Number: 702}, {Number: 703}, {Number: 704}, {Number: 705}, {Number: 706},
	}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{
		verdictTestRepo + "#701": {Author: authorIdentity{Login: "assay-worker-app[bot]", ID: 300000006, Type: "Bot"}, Body: forgedBody},
		verdictTestRepo + "#702": {Author: verifierIdent, Body: tampered},
		verdictTestRepo + "#703": {Author: verifierIdent, Body: editedBody, Edited: true},
		verdictTestRepo + "#704": {Author: verifierIdent, Body: irrevBody},
		verdictTestRepo + "#705": {Author: verifierIdent, Body: humanBody},
		verdictTestRepo + "#706": {Author: verifierIdent, Body: mismatchBody},
	})

	// 706 needs a FAILING re-execution; the rest never reach cl.6, so a fail runner
	// is safe for all — only 706 gets there.
	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, failCheckCI, noHealthHold, blessR6Resolver)
	})
	if out.code != 0 {
		t.Fatalf("run should complete (exit 0) with per-verdict refusals, got %d:\n%s", out.code, out.log)
	}

	want := map[int]string{
		701: "clause-1 (author)",
		702: "clause-2 (signature)",
		703: "clause-2 (timeline)",
		704: "clause-5 (irreversible)",
		705: "clause-5 (human-stamp)",
		706: "clause-6 (check:ci)",
	}
	for n, clause := range want {
		marker := "REFUSE " + verdictTestRepo + "#" + strconv.Itoa(n) + " — " + clause
		if !strings.Contains(out.log, marker) {
			t.Errorf("issue %d must be REFUSED naming %s; log:\n%s", n, clause, out.log)
		}
	}
	if strings.Contains(out.log, "CONSUME") {
		t.Errorf("no verdict in the negative battery may be consumed:\n%s", out.log)
	}
	if strings.Contains(out.log, "FLIP") {
		t.Errorf("no flip may result from the negative battery:\n%s", out.log)
	}
}

// ---------------------------------------------------------------------------
// Run: refusals (roster / flood / main-health)
// ---------------------------------------------------------------------------

func TestTranscribeVerdictRosterUnconfiguredRefused(t *testing.T) {
	scanWithNoRoster(t)
	root := verdictBaseRepo(t, r6Armed)
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: nil}, "")
	resolve := fixtureVerdictResolver(nil)

	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if out.code != 2 {
		t.Fatalf("an unconfigured roster must REFUSE the run (exit 2), got %d:\n%s", out.code, out.err)
	}
	if !strings.Contains(out.err, "REFUSED") || !strings.Contains(out.err, "roster") {
		t.Errorf("refusal must name the roster; stderr:\n%s", out.err)
	}
}

func TestTranscribeVerdictNoVerifierBoundRefused(t *testing.T) {
	// A configured roster that binds NO verifier App.
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:      "ada:100001",
		scanEnvTrustedLogins:   "ada:100001",
		scanEnvTrustedBotSlugs: "worker=assay-worker-app:300000006",
		scanEnvHomeRepo:        verdictTestRepo,
		scanEnvScanRepos:       verdictTestRepo,
	})
	verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: nil}, "")
	resolve := fixtureVerdictResolver(nil)

	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if out.code != 2 {
		t.Fatalf("no verifier App bound must REFUSE the run (exit 2), got %d:\n%s", out.code, out.err)
	}
	if !strings.Contains(out.err, "verifier") {
		t.Errorf("refusal must name the missing verifier binding; stderr:\n%s", out.err)
	}
}

func TestTranscribeVerdictFloodTripwireRefused(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	body := signVerdictBody(t, key, okPayload(okEntry(rel, "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |")))
	// 21 verdict issues (> the threshold of 20) → clause-9.
	var issues []ghIssue
	data := map[string]verdictIssue{}
	for n := 800; n < 821; n++ {
		issues = append(issues, ghIssue{Number: n})
		data[verdictTestRepo+"#"+strconv.Itoa(n)] = verdictIssue{Author: verifierIdent, Body: body}
	}
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: issues}, "")
	resolve := fixtureVerdictResolver(data)

	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	if out.code != 3 {
		t.Fatalf("21 unconsumed verdict issues must trip the flood tripwire (exit 3), got %d:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.log, "FLOOD") || !strings.Contains(out.log, "clause-9") {
		t.Errorf("flood refusal must name clause-9; log:\n%s", out.log)
	}
}

func TestTranscribeVerdictMainHealthHold(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)

	body := signVerdictBody(t, key, okPayload(okEntry(rel, "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |")))
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: 901}}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{
		verdictTestRepo + "#901": {Author: verifierIdent, Body: body},
	})
	held := func() (bool, string, error) {
		return true, "open ci-failure issue younger than 6h", nil
	}

	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, false, "", list, resolve, passCheckCI, held, blessR6Resolver)
	})
	if out.code != 0 {
		t.Fatalf("a main-health hold must exit 0 (neutral), got %d", out.code)
	}
	if !strings.Contains(out.log, "HELD") {
		t.Errorf("expected a HELD report; log:\n%s", out.log)
	}
	// Held → nothing written.
	b, _ := os.ReadFile(filepath.Join(root, rel))
	if strings.Contains(string(b), "verifier |") {
		t.Error("a held lane must append nothing")
	}
}

// An unreadable main-health signal is fail-closed HELD, never a pass-through.
func TestTranscribeVerdictMainHealthUnreadableHolds(t *testing.T) {
	scanWithRoster(t, verdictRoster())
	verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	writeVerdictBrief(t, root, "01", "model", false)
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: nil}, "")
	resolve := fixtureVerdictResolver(nil)
	errHealth := func() (bool, string, error) { return false, "", errFixture }

	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, true, "", list, resolve, passCheckCI, errHealth, blessR6Resolver)
	})
	if out.code != 0 || !strings.Contains(out.log, "HELD") {
		t.Errorf("an unreadable main-health signal must fail-closed HELD (exit 0); code=%d log:\n%s", out.code, out.log)
	}
}

// Deep unit: the pubkey variable resolver accepts base64-of-PEM too.
func TestVerdictDecodePubkeyVarBase64(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	b64 := base64.StdEncoding.EncodeToString(pubPEM)
	got, derr := verdictDecodePubkeyVar(b64)
	if derr != nil {
		t.Fatalf("base64-of-PEM must decode: %v", derr)
	}
	if !strings.Contains(string(got), "BEGIN PUBLIC KEY") {
		t.Errorf("decoded value is not a PEM: %s", got)
	}
	if _, err := verdictDecodePubkeyVar("   "); err == nil {
		t.Error("an empty pubkey value must error, never a silent pass")
	}
}
