package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// writeTestKey generates a fresh RSA key, writes its PKCS#1 PEM to a temp file, and returns
// the path plus the matching public key for verification.
func writeTestKey(t *testing.T) (pemPath string, pub *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	pemPath = filepath.Join(t.TempDir(), "verifier-app.pem")
	if err := os.WriteFile(pemPath, pemBytes, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return pemPath, &key.PublicKey
}

// --- Row 1: batch window flush logic ----------------------------------------

func TestBatchDueToFlush(t *testing.T) {
	start := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)
	window := 5 * time.Minute

	// Empty batch never flushes, drained or not.
	var empty batch
	if empty.dueToFlush(start.Add(time.Hour), window, true) {
		t.Fatalf("empty batch flushed — a flush must produce a non-empty payload")
	}

	b := batch{}
	b.add(rowResult{Exit: 0}, start)

	// Inside the window, not drained → hold.
	if b.dueToFlush(start.Add(4*time.Minute), window, false) {
		t.Fatalf("flushed before window elapsed and not drained")
	}
	// Window elapsed → flush.
	if !b.dueToFlush(start.Add(5*time.Minute), window, false) {
		t.Fatalf("did not flush after the window elapsed")
	}
	// Drained before window → flush (whichever first).
	if !b.dueToFlush(start.Add(time.Minute), window, true) {
		t.Fatalf("did not flush on queue drain inside the window")
	}
}

// --- Row 1: payload composition + FAIL row inclusion + provenance digest -----

func TestComposePayloadIncludesFailRowsAndStampsProvenance(t *testing.T) {
	ts := time.Date(2026, 8, 17, 20, 30, 0, 0, time.UTC)
	meta := sessionMeta{ID: "wd-demo", Runner: "assay-verifier-app[bot]"}
	rows := []rowResult{
		{BriefPath: "docs/streams/demo/brief-01.md", Row: 1, Class: "check:ci", Command: "go test ./...", Exit: 0, Output: "ok\n"},
		{BriefPath: "docs/streams/demo/brief-02.md", Row: 2, Class: "check", Command: "false", Exit: 1, Output: "FAIL: boom\n"},
	}
	p := composePayload("medici-finance/assay", "deadbeef", ts, meta, rows)

	if p.Schema != deskkit.VerdictSchemaVersion {
		t.Fatalf("schema = %q, want %q", p.Schema, deskkit.VerdictSchemaVersion)
	}
	if len(p.Entries) != 2 {
		t.Fatalf("entries = %d, want 2 (FAIL rows are INCLUDED, not dropped)", len(p.Entries))
	}
	if p.Entries[0].Result != loopengine.VerdictPass {
		t.Fatalf("row 1 result = %q, want PASS", p.Entries[0].Result)
	}
	if p.Entries[1].Result != loopengine.VerdictFail {
		t.Fatalf("row 2 result = %q, want FAIL (the verdict carries the failure)", p.Entries[1].Result)
	}
	// Provenance digest is the engine's sha256 of the RAW transcript it received.
	want := sha256.Sum256([]byte("FAIL: boom\n"))
	if got := p.Entries[1].Session.TranscriptSHA256; got != hex.EncodeToString(want[:]) {
		t.Fatalf("transcript_sha256 = %q, want %q", got, hex.EncodeToString(want[:]))
	}
	if p.Entries[1].Session.Runner != meta.Runner {
		t.Fatalf("session runner = %q, want %q", p.Entries[1].Session.Runner, meta.Runner)
	}
}

// --- Row 2: dry-run body verifies; one flipped byte then refuses -------------

func TestSignedBodyVerifiesAndTamperRefuses(t *testing.T) {
	pemPath, pub := writeTestKey(t)
	p := composePayload("medici-finance/assay", "cafe1234", time.Now(),
		sessionMeta{ID: "s", Runner: "r"},
		[]rowResult{{BriefPath: "b.md", Row: 1, Class: "check:ci", Command: "true", Exit: 0, Output: "ok"}})

	body, err := signPayload(p, pemPath)
	if err != nil {
		t.Fatalf("signPayload: %v", err)
	}
	if state, msg := deskkit.VerifyVerdictBody(body, pub); state != deskkit.VerdictVerified {
		t.Fatalf("VerifyVerdictBody(untampered) = %v (%s), want Verified", state, msg)
	}

	// Flip one byte of the logical payload (PASS -> FAIL) and the signature must refuse.
	tampered := strings.Replace(body, `"result":"PASS"`, `"result":"FAIL"`, 1)
	if tampered == body {
		t.Fatalf("test setup: could not find the result field to flip in the body")
	}
	if state, msg := deskkit.VerifyVerdictBody(tampered, pub); state != deskkit.VerdictRefused {
		t.Fatalf("VerifyVerdictBody(tampered) = %v (%s), want Refused", state, msg)
	}
}

// --- Row 2: a DRY-RUN body verifies with the deskverdict-verify primitive; a
// flipped byte then refuses. This is the Verify-table row-2 check named exactly so
// `go test -run 'DryRunSignVerify'` selects it (a `-run` filter that matches nothing
// exits 0 — a false green — so the row is only a real assertion when a test carries
// this name). It exercises the actual `--dry-run` emit path (runVerdict dryRun),
// extracts the emitted signed body, and verifies it through deskkit.VerifyVerdictBody
// — the same body-level verify `deskverdict verify` calls — so signing and verifying
// share one canonical form end-to-end.
func TestDryRunSignVerify(t *testing.T) {
	root := demoRoot(t)
	pemPath, pub := writeTestKey(t)

	var out bytes.Buffer
	err := runVerdict(verdictRunConfig{
		root: root, repo: "medici-finance/assay", head: "cafe1234",
		pem: pemPath, dryRun: true, window: time.Hour, exec: fakeExec, out: &out,
	})
	if err != nil {
		t.Fatalf("runVerdict dry-run: %v", err)
	}

	// The emitted signed body is everything before the trailing "signed verdict …" summary.
	s := out.String()
	body := s
	if i := strings.Index(s, "\nsigned verdict "); i >= 0 {
		body = s[:i]
	}
	if !strings.Contains(body, verdictFenceTagLiteral) {
		t.Fatalf("dry-run body carries no verdict-payload block; got:\n%s", s)
	}

	// The untampered dry-run body verifies with deskverdict's body-level verify.
	if state, msg := deskkit.VerifyVerdictBody(body, pub); state != deskkit.VerdictVerified {
		t.Fatalf("VerifyVerdictBody(dry-run body) = %v (%s), want Verified", state, msg)
	}

	// Flip one byte of the logical payload (PASS -> FAIL) and the signature must refuse:
	// the digest no longer matches the canonical bytes that were signed.
	tampered := strings.Replace(body, `"result":"PASS"`, `"result":"FAIL"`, 1)
	if tampered == body {
		t.Fatalf("test setup: no PASS result to flip in the dry-run body:\n%s", body)
	}
	if state, msg := deskkit.VerifyVerdictBody(tampered, pub); state != deskkit.VerdictRefused {
		t.Fatalf("VerifyVerdictBody(one flipped byte) = %v (%s), want Refused", state, msg)
	}
}

// --- Row 3: missing-PEM env → non-zero, names the envelope, files nothing ----

func TestRunVerdictMissingPEMFailsClosedAndFilesNothing(t *testing.T) {
	root := demoRoot(t)
	// VERIFIER_PEM points at a nonexistent file: the runner resolves it, runs the rows, then
	// the sign step fails closed — nothing signed, nothing emitted.
	t.Setenv("VERIFIER_PEM", filepath.Join(t.TempDir(), "does-not-exist.pem"))

	var out bytes.Buffer
	err := runVerdict(verdictRunConfig{
		root: root, repo: "medici-finance/assay", head: "sha",
		dryRun: true, window: time.Hour, exec: fakeExec, out: &out,
	})
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("runVerdict with missing PEM = %v, want Unverifiable (exit 6)", err)
	}
	if !strings.Contains(err.Error(), "verifier") {
		t.Fatalf("envelope error must name the verifier key, got: %v", err)
	}
	if strings.Contains(out.String(), verdictFenceTagLiteral) || strings.Contains(out.String(), "signed verdict") {
		t.Fatalf("a fail-closed envelope must file/emit NOTHING; got:\n%s", out.String())
	}
}

// --- End-to-end: dry-run composes + signs + prints, no filing ----------------

func TestRunVerdictDryRunEndToEnd(t *testing.T) {
	root := demoRoot(t)
	pemPath, pub := writeTestKey(t)

	var out bytes.Buffer
	err := runVerdict(verdictRunConfig{
		root: root, repo: "medici-finance/assay", head: "sha",
		pem: pemPath, dryRun: true, window: time.Hour, exec: fakeExec, out: &out,
	})
	if err != nil {
		t.Fatalf("runVerdict dry-run: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "signed verdict for 2 row(s) across 1 brief(s)") {
		t.Fatalf("summary line missing/wrong; got:\n%s", s)
	}
	if !strings.Contains(s, verdictFenceTagLiteral) {
		t.Fatalf("dry-run body must carry the verdict-payload block; got:\n%s", s)
	}
	// The emitted body (everything before the trailing summary line) must verify.
	body := s
	if i := strings.Index(s, "\nsigned verdict "); i >= 0 {
		body = s[:i]
	}
	if state, msg := deskkit.VerifyVerdictBody(body, pub); state != deskkit.VerdictVerified {
		t.Fatalf("emitted dry-run body did not verify: %v (%s)", state, msg)
	}
	if !strings.Contains(body, `"result":"FAIL"`) {
		t.Fatalf("dry-run payload must include the FAIL row; got body:\n%s", body)
	}
}

// verdictFenceTagLiteral is the fenced-block opener the signed body carries.
const verdictFenceTagLiteral = "```verdict-payload"

// fakeExec is a deterministic rowExec: `true`/anything-0 → exit 0, `false` → exit 1. No real
// shell, so the batch/payload/sign path is exercised hermetically.
func fakeExec(root, command string) (int, string) {
	if strings.TrimSpace(command) == "false" {
		return 1, "boom\n"
	}
	return 0, "ok\n"
}

// demoRoot builds a controlled repo tree: one stream with an implemented brief whose Verify
// table has a check:ci row, a check row, and a gate:model row (the last skipped by the runner).
func demoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "---\nstream: demo\n---\n\n" +
		"| # | status | verified | reviewed |\n" +
		"|---|--------|----------|----------|\n" +
		"| 01 | implemented | — | — |\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\n" +
		"## Verify\n\n" +
		"| # | Class | Command | Expect |\n" +
		"|---|-------|---------|--------|\n" +
		"| 1 | check:ci | true | exit 0 |\n" +
		"| 2 | check | false | exit non-zero |\n" +
		"| 3 | gate:model | a model judges | ok |\n\n" +
		"## Evidence\n<!-- appended at implementation time -->\n"
	if err := os.WriteFile(filepath.Join(dir, "brief-01-demo.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
