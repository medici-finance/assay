package main

// Tests for the R-7 clause-4 CROSS-REPO scan-delta verify path
// (the house-private brief's Task 3). Reuses the R-7 enactment-gate fixtures
// (writeRulings, r7Armed, blessAuthorityResolver) from transcribescan_test.go
// and the verdictIssue fixture resolver from transcribeverdict_test.go — same
// package, one fixture vocabulary.

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// scanDeltaIssueLoopIdent is the API-read identity the fixture roster binds to
// the "issue-loop" role.
var scanDeltaIssueLoopIdent = authorIdentity{Login: "assay-issue-loop-app[bot]", ID: 300000010, Type: "Bot"}

// scanDeltaRoster extends transcribeRoster with the issue-loop role bound in
// ASSAY_TRUSTED_BOT_SLUGS, so scanDeltaIssueLoopIdentity() resolves it.
func scanDeltaRoster() map[string]string {
	r := transcribeRoster()
	r[scanEnvTrustedBotSlugs] = r[scanEnvTrustedBotSlugs] + ",issue-loop=assay-issue-loop-app:300000010"
	return r
}

// scanDeltaTestKey generates a throwaway RSA keypair and exports its PKIX
// public PEM into ASSAY_ISSUE_LOOP_PUBKEY, so scanDeltaResolvePubkey resolves
// it through the real code path — the issue-loop twin of verdictTestKey.
func scanDeltaTestKey(t *testing.T) *rsa.PrivateKey {
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
	t.Setenv(scanDeltaPubkeyVar, string(pubPEM))
	return key
}

// signScanDeltaBody canonicalises pl, signs it with key, and assembles the
// issue-body block declaring role — the SAME shape `deskverdict sign --key
// <role>` emits (AssembleVerdictBodyForRole). role is a parameter (not always
// "issue-loop") so a test can construct a wrong-role fixture without a second
// helper.
func signScanDeltaBody(t *testing.T, key *rsa.PrivateKey, pl scanDeltaPayload, role string) string {
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
		"<!-- " + verdictSigMarker + " v1 alg=RS256 role=" + role + " sig=" + b64 + " -->\n"
}

// scanDeltaEntryFixture builds a scanDeltaEntry whose Body is a real
// placeholder-v1 render (renderPlaceholder), so byte-identity assertions are
// meaningful, not a rubber stamp.
func scanDeltaEntryFixture(repo string, issue int, login string, id int64, typ, trustBasis string) scanDeltaEntry {
	gate := derivePlaceholderGate(nil, "an ordinary issue title")
	return scanDeltaEntry{
		Repo: repo, Issue: issue,
		AuthorLogin: login, AuthorID: id, AuthorType: typ,
		TrustBasis: trustBasis, Class: "create",
		Body: renderPlaceholder(repo, issue, gate, nil),
	}
}

// scanDeltaContainer wraps a signed payload body as the verdictIssue the
// container-level checks read: authored by author, edited per edited.
func scanDeltaContainer(body string, author authorIdentity, edited bool) verdictIssue {
	return verdictIssue{Author: author, Body: body, Edited: edited}
}

// --- planScanDelta: the core clause-4 battery, one issue at a time ---------

func TestScanDeltaRoundtripByteIdentical(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	e1 := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	e2 := scanDeltaEntryFixture("example-org/agents", 7, "ada", 100001, "User", "blessed:12345")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, TS: "2026-09-18T00:00:00Z", Entries: []scanDeltaEntry{e1, e2}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	authors := fixtureAuthorResolver(map[string]authorIdentity{
		"example-org/widget#42": {Login: "dana", ID: 200002, Type: "User"},
		"example-org/agents#7":  {Login: "ada", ID: 100001, Type: "User"},
	})

	creates, results, notices, clause, why := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, authors, map[string]bool{})
	if clause != "" {
		t.Fatalf("expected no container-level refusal, got %s: %s", clause, why)
	}
	if len(creates) != 2 {
		t.Fatalf("expected 2 CREATEs, got %d (results=%+v notices=%v)", len(creates), results, notices)
	}
	for _, c := range creates {
		var want scanDeltaEntry
		switch c.Issue {
		case 42:
			want = e1
		case 7:
			want = e2
		default:
			t.Fatalf("unexpected create for issue %d", c.Issue)
		}
		if c.Content != want.Body {
			t.Errorf("issue %d content not byte-identical to the payload's rendered body:\n--- got ---\n%s\n--- want ---\n%s", c.Issue, c.Content, want.Body)
		}
	}
	for _, r := range results {
		if r.Outcome != scanDeltaAccepted {
			t.Errorf("entry %s#%d unexpectedly not accepted: %s (%s)", r.Entry.Repo, r.Entry.Issue, r.Clause, r.Reason)
		}
	}
}

func TestScanDeltaSameRepoEntryRefused(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	// This entry targets the HOME repo itself — must be refused, never boarded
	// via the cross-repo lane (R-7 cl.2a already owns same-repo issues).
	e := scanDeltaEntryFixture("example-org/tracker", 55, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	creates, results, _, clause, why := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	if clause != "" {
		t.Fatalf("container-level should not refuse here, got %s: %s", clause, why)
	}
	if len(creates) != 0 {
		t.Fatalf("same-repo entry must never be created, got %d creates", len(creates))
	}
	if len(results) != 1 || results[0].Outcome != scanDeltaEntryRefused || !strings.Contains(results[0].Clause, "same-repo") {
		t.Fatalf("expected exactly one same-repo refusal, got %+v", results)
	}
}

func TestScanDeltaAuthorContradictedRefused(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	// The API reads a DIFFERENT author than the entry declares.
	authors := fixtureAuthorResolver(map[string]authorIdentity{
		"example-org/widget#42": {Login: "mallory", ID: 999999, Type: "User"},
	})

	creates, results, _, clause, _ := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, authors, map[string]bool{})
	if clause != "" {
		t.Fatalf("container-level should not refuse here")
	}
	if len(creates) != 0 {
		t.Fatal("a contradicted entry must never be created")
	}
	if len(results) != 1 || results[0].Outcome != scanDeltaEntryRefused || !strings.Contains(results[0].Clause, "contradicted") {
		t.Fatalf("expected a per-entry author-contradicted refusal, got %+v", results)
	}
}

// TestScanDeltaAuthorUnreadableStillAccepted is the R-7 cl.4 two-tier honesty
// row: an entry whose repo this box cannot read via the API rests on the
// signature + the producer's own clause-1 check — NOT a refusal, but the
// reason is surfaced as a NOTICE, never silently dropped and never silently
// promoted to "verified via API".
func TestScanDeltaAuthorUnreadableStillAccepted(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/private-repo", 9, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	// fixtureAuthorResolver(nil) errors for EVERY key — "unreadable".
	creates, results, notices, clause, _ := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	if clause != "" {
		t.Fatal("container-level should not refuse here")
	}
	if len(creates) != 1 {
		t.Fatalf("an unreadable-author entry rests on the signature and IS still created, got %d creates", len(creates))
	}
	if len(results) != 1 || results[0].Outcome != scanDeltaAccepted {
		t.Fatalf("expected the entry accepted (two-tier honesty), got %+v", results)
	}
	if len(notices) == 0 {
		t.Fatal("an unreadable-author accept must surface a NOTICE, never silently pass")
	}
}

func TestScanDeltaContainerAuthorNotIssueLoopRefused(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	// The CONTAINER issue is authored by an arbitrary human, not the issue-loop App.
	vi := scanDeltaContainer(body, authorIdentity{Login: "mallory", ID: 5005, Type: "User"}, false)

	creates, _, _, clause, why := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	if clause != "clause-4 (author)" {
		t.Fatalf("expected clause-4 (author) refusal, got clause=%q why=%q", clause, why)
	}
	if len(creates) != 0 {
		t.Fatal("nothing should be created when the container author check fails")
	}
}

func TestScanDeltaWrongRoleSignatureRefused(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	// Signed with the SAME key, but declaring role=verifier instead of issue-loop.
	body := signScanDeltaBody(t, key, pl, "verifier")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	creates, _, _, clause, why := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	if clause != "clause-4 (signature)" {
		t.Fatalf("expected clause-4 (signature) refusal for a wrong-role block, got clause=%q why=%q", clause, why)
	}
	if !strings.Contains(why, "role") {
		t.Errorf("refusal reason should name the role mismatch: %s", why)
	}
	if len(creates) != 0 {
		t.Fatal("nothing should be created on a role-declaration mismatch")
	}
}

// TestScanDeltaWrongKeySignatureRefused is the plain crypto-mismatch row: the
// body correctly declares role=issue-loop, but was signed with a DIFFERENT
// keypair than the one configured in ASSAY_ISSUE_LOOP_PUBKEY — a definite
// cryptographic negative, distinct from the role-declaration mismatch above
// (that one uses the SAME key with the WRONG declared role; this one uses the
// RIGHT declared role with the WRONG key).
func TestScanDeltaWrongKeySignatureRefused(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	configuredKey := scanDeltaTestKey(t) // exports ITS pub into ASSAY_ISSUE_LOOP_PUBKEY
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, otherKey, pl, "issue-loop") // signed with the WRONG key

	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)
	pub := &configuredKey.PublicKey // the key ACTUALLY configured for this role

	creates, _, _, clause, why := planScanDelta(root, 900, vi, pub, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	if clause != "clause-4 (signature)" {
		t.Fatalf("expected clause-4 (signature) refusal for a wrong-key block, got clause=%q why=%q", clause, why)
	}
	if strings.Contains(why, "role") {
		t.Errorf("this refusal is a CRYPTO mismatch, not a role mismatch — the message should not blame the role: %s", why)
	}
	if len(creates) != 0 {
		t.Fatal("nothing should be created when the signature does not verify against the configured key")
	}
}

func TestScanDeltaEditedBodyRefused(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	// The container issue WAS edited since creation.
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, true)

	creates, _, _, clause, why := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	if clause != "clause-4 (body-unedited)" {
		t.Fatalf("expected clause-4 (body-unedited) refusal, got clause=%q why=%q", clause, why)
	}
	if len(creates) != 0 {
		t.Fatal("nothing should be created when the container was edited")
	}
}

func TestScanDeltaNoPubkeyConfiguredCouldNotCheck(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	// Do NOT call scanDeltaTestKey — no key material ever generated, and the
	// variable is explicitly unset so a leaked test-runner env cannot leak in.
	t.Setenv(scanDeltaPubkeyVar, "")
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	// Sign with a throwaway key never exported anywhere — pub is nil below regardless.
	throwaway, _ := rsa.GenerateKey(rand.Reader, 2048)
	body := signScanDeltaBody(t, throwaway, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	creates, _, _, clause, why := planScanDelta(root, 900, vi, nil, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	t.Logf("clause=%s why=%s", clause, why) // printed under -v regardless of pass/fail, so a verify.d row can grep the wording
	if clause != "clause-4 (signature)" {
		t.Fatalf("expected clause-4 (signature) could-not-check, got clause=%q why=%q", clause, why)
	}
	if !strings.Contains(why, "could not check") {
		t.Errorf("a missing key must read as could-not-check, never a pass or a bare refusal: %s", why)
	}
	if len(creates) != 0 {
		t.Fatal("nothing should be created when no key is configured")
	}
}

func TestScanDeltaUnsupportedClassCouldNotCheck(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	e.Class = "retire" // not yet handled by this lane
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	creates, results, _, clause, _ := planScanDelta(root, 900, vi, &key.PublicKey, "example-org/tracker", scanDeltaIssueLoopIdent, fixtureAuthorResolver(nil), map[string]bool{})
	if clause != "" {
		t.Fatal("container-level should not refuse here")
	}
	if len(creates) != 0 {
		t.Fatal("an unsupported class must never be created")
	}
	if len(results) != 1 || results[0].Outcome != scanDeltaEntryCouldNotCheck {
		t.Fatalf("expected could-not-check for an unsupported class, got %+v", results)
	}
}

// --- runTranscribeScanDelta: the sweep + gate + apply/dry-run surface ------

func TestRunTranscribeScanDeltaInertWhenUnarmed(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, "**Sign-off:** _(empty)_") // R-7 UNsigned

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	list := fixtureLister(map[string][]ghIssue{"example-org/tracker": {{Number: 900, Title: "scan-delta batch", Labels: lbl("scan-delta")}}}, "")
	resolveIssue := fixtureVerdictResolver(map[string]verdictIssue{"example-org/tracker#900": vi})

	out := captureRun(t, func() int {
		return runTranscribeScanDelta(root, true, "", list, resolveIssue, fixtureAuthorResolver(nil), blessNoneCommentResolver)
	})
	if out.code != 0 {
		t.Fatalf("unarmed run must exit 0, got %d:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.log, "INERT") {
		t.Errorf("unarmed run must announce INERT: %s", out.log)
	}
	if strings.Contains(out.log, "CREATE") {
		t.Error("an unarmed lane must evaluate no clause and create nothing")
	}
}

// blessNoneCommentResolver never resolves any URL to the bless authority — used
// to keep the enactment gate closed deliberately in the INERT test above (the
// empty sign-off line already keeps it closed before resolve is ever called).
func blessNoneCommentResolver(url string) (authorIdentity, string, error) {
	return authorIdentity{}, "", fmt.Errorf("no resolver configured for %s", url)
}

func TestRunTranscribeScanDeltaDryRunThenApply(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)

	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	list := fixtureLister(map[string][]ghIssue{"example-org/tracker": {{Number: 900, Title: "scan-delta batch", Labels: lbl("scan-delta")}}}, "")
	resolveIssue := fixtureVerdictResolver(map[string]verdictIssue{"example-org/tracker#900": vi})
	authors := fixtureAuthorResolver(map[string]authorIdentity{"example-org/widget#42": {Login: "dana", ID: 200002, Type: "User"}})

	target := filepath.Join(root, "docs", "streams", scanStreamName, placeholderFileName("example-org/widget", 42))

	// --check: report only.
	out := captureRun(t, func() int {
		return runTranscribeScanDelta(root, true, "", list, resolveIssue, authors, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("armed --check must exit 0, got %d:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.log, "CREATE") || !strings.Contains(out.log, "example-org/widget#42") {
		t.Errorf("expected a CREATE for example-org/widget#42 in:\n%s", out.log)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("--check must not write the placeholder")
	}

	// Apply: same run, dryRun=false — writes the file, byte-identical to the
	// entry's rendered body (the producer's render IS the trust primitive).
	out = captureRun(t, func() int {
		return runTranscribeScanDelta(root, false, "", list, resolveIssue, authors, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("apply must exit 0, got %d:\n%s", out.code, out.log)
	}
	got, rerr := os.ReadFile(target)
	if rerr != nil {
		t.Fatalf("apply did not write %s: %v", target, rerr)
	}
	if string(got) != e.Body {
		t.Errorf("written content not byte-identical to the signed entry's body:\n--- got ---\n%s\n--- want ---\n%s", got, e.Body)
	}
}

func TestRunTranscribeScanDeltaFloodTripwireRefusesWholeIssue(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)

	var entries []scanDeltaEntry
	authorsMap := map[string]authorIdentity{}
	for i := 1; i <= transcribeFloodThreshold+1; i++ {
		repo := fmt.Sprintf("example-org/flood%d", i)
		entries = append(entries, scanDeltaEntryFixture(repo, i, "dana", 200002, "User", "rostered"))
		authorsMap[repo+"#"+strconv.Itoa(i)] = authorIdentity{Login: "dana", ID: 200002, Type: "User"}
	}
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: entries}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)

	list := fixtureLister(map[string][]ghIssue{"example-org/tracker": {{Number: 900, Title: "scan-delta batch", Labels: lbl("scan-delta")}}}, "")
	resolveIssue := fixtureVerdictResolver(map[string]verdictIssue{"example-org/tracker#900": vi})
	authors := fixtureAuthorResolver(authorsMap)

	out := captureRun(t, func() int {
		return runTranscribeScanDelta(root, false, "", list, resolveIssue, authors, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("a flood refusal is per-issue, the RUN still completes; got %d:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.log, "FLOOD") {
		t.Errorf("expected a FLOOD line, got:\n%s", out.log)
	}
	if strings.Contains(out.log, "created ") {
		t.Error("a flooded issue's delta must write nothing")
	}
	for i := 1; i <= transcribeFloodThreshold+1; i++ {
		target := filepath.Join(root, "docs", "streams", scanStreamName, placeholderFileName(fmt.Sprintf("example-org/flood%d", i), i))
		if _, err := os.Stat(target); err == nil {
			t.Fatalf("flood entry %d must not have been written", i)
		}
	}
}

func TestRunTranscribeScanDeltaSkipsOrdinaryIssuesSilently(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)

	list := fixtureLister(map[string][]ghIssue{"example-org/tracker": {
		{Number: 1, Title: "an ordinary issue with no payload block", Labels: nil},
	}}, "")
	resolveIssue := fixtureVerdictResolver(map[string]verdictIssue{
		"example-org/tracker#1": {Author: authorIdentity{Login: "someone", ID: 1, Type: "User"}, Body: "just an ordinary issue body, no fenced block"},
	})

	out := captureRun(t, func() int {
		return runTranscribeScanDelta(root, true, "", list, resolveIssue, fixtureAuthorResolver(nil), blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("exit 0 expected, got %d:\n%s", out.code, out.log)
	}
	if strings.Contains(out.log, "REFUSE") || strings.Contains(out.log, "CREATE") {
		t.Errorf("an ordinary issue with no payload block must be skipped SILENTLY (no clause noise): %s", out.log)
	}
	if !strings.Contains(out.log, "0 carried a scan-delta payload block") {
		t.Errorf("expected the sweep summary to report 0 candidates: %s", out.log)
	}
}
