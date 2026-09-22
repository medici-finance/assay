package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskevidence_test.go — the behavioural suite, rewritten onto the forge seam.
//
// SINCE THE FORGE MIGRATION deskevidence performs no JWT exchange and builds no net/http of its
// own: every read and write goes through the resolved deskkit.Forge under the verifier App's
// custody. The tests therefore drive a RECORDING fake Forge (fakeForge below) rather than a fake
// GitHub HTTP server, and assert on the ops it recorded — the same equivalence the golden corpus
// pins at the wire, applied here to deskevidence's own decisions (idempotency, the shrink guard,
// the main-branch guard, attribution, the Evidence-lane fallback). The install-id/JWT tests the
// pre-migration suite carried are gone with the code they pinned; the custody question they were
// about now lives in the resolver (forgeresolve_test.go).

// fakeForge is a recording, scriptable Forge. Only the methods deskevidence calls are
// implemented; the embedded nil interface makes any other method a compile-time member and a
// run-time panic if some future path reaches for it.
type fakeForge struct {
	deskkit.Forge

	mu    sync.Mutex
	files map[string]string // repo-path -> content served by ReadFile / written by WriteFile

	// defaultBranch, when set, makes a WriteFile targeting it return the
	// DefaultBranchNotWritable sentinel (nothing recorded) — the GitLab-shaped closed default.
	defaultBranch string
	// writeAuthor is the author WriteFile reports; "" resolves to the verifier bot display.
	writeAuthor string
	// emptyAuthor forces WriteFile to report NO author (the could-not-check case).
	emptyAuthor bool
	// writeErr, when set, is returned by WriteFile.
	writeErr error
	// commitAuthorLogin is the AuthorLogin GetCommit resolves for a sha — the ONLINE
	// attribution resolution (#1477). Empty means the forge could not resolve the account
	// (could-not-check). commitErr, when set, is GetCommit's error.
	commitAuthorLogin string
	commitErr         error
	getCommitCalls    int
	// prHeadSHA is the HeadSHA GetPullRequest reports for the draft change opened on the GitLab
	// landing path, and prErr its error. Empty HeadSHA leaves the online resolution with no sha.
	prHeadSHA string
	prErr     error
	// onPut runs at the top of WriteFile, inside deskevidence's audit flock — the
	// serialisation tests use it to observe what a concurrent invocation can do.
	onPut func()

	hits    []string
	reads   []deskkit.ReadFileInput
	writes  []deskkit.WriteFileInput
	changes []deskkit.DraftChangeInput

	// compat mirrors of the most-asserted write facts.
	putCalls   int
	putBranch  string
	putContent string

	// visibility is what RepoVisibility reports (the public-repo gate's live-visibility
	// read — assay#1066's regression coverage, see gatewired_test.go). Defaults to
	// "private" (the gate's no-op case) when unset.
	visibility      string
	visibilityCalls int
	visibilityRepo  deskkit.ForgeRepo
}

// RepoVisibility answers the public-repo gate's live-visibility read from THIS fake — the
// resolved forge backend — rather than any hardcoded GitHub-only client.
func (f *fakeForge) RepoVisibility(repo deskkit.ForgeRepo) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.visibilityCalls++
	f.visibilityRepo = repo
	if f.visibility != "" {
		return f.visibility, nil
	}
	return "private", nil
}

func (f *fakeForge) setFile(path, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files == nil {
		f.files = map[string]string{}
	}
	f.files[path] = content
}

func (f *fakeForge) ReadFile(_ deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits = append(f.hits, "GET "+in.File)
	f.reads = append(f.reads, in)
	c, ok := f.files[in.File]
	if !ok {
		return nil, deskkit.Unverifiable("not found: "+in.File,
			&deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: in.File})
	}
	return &deskkit.FileContent{Content: []byte(c), SHA: "sha-" + in.File, Exists: true}, nil
}

func (f *fakeForge) WriteFile(_ deskkit.ForgeRepo, in deskkit.WriteFileInput) (*deskkit.WriteFileResult, error) {
	if f.onPut != nil {
		f.onPut()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits = append(f.hits, "PUT "+in.File+"@"+in.Branch)
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	if f.defaultBranch != "" && in.Branch == f.defaultBranch {
		// The closed default branch: report the sentinel, record NO write.
		return &deskkit.WriteFileResult{DefaultBranchNotWritable: true}, nil
	}
	f.writes = append(f.writes, in)
	f.putCalls++
	f.putBranch = in.Branch
	f.putContent = string(in.Content)
	if f.files == nil {
		f.files = map[string]string{}
	}
	f.files[in.File] = string(in.Content)
	author := f.writeAuthor
	if f.emptyAuthor {
		author = ""
	} else if author == "" {
		author = verifierBotDisplay()
	}
	return &deskkit.WriteFileResult{Changed: true, SHA: "new-sha", Author: author}, nil
}

func (f *fakeForge) CreateDraftChange(_ deskkit.ForgeRepo, in deskkit.DraftChangeInput) (*deskkit.PullRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.changes = append(f.changes, in)
	return &deskkit.PullRef{Number: 4242, URL: "https://forge.example/change/4242"}, nil
}

// GetPullRequest serves the head sha the online attribution resolution reads on the GitLab
// draft-landing path (#1477). Only the HeadSHA field is populated (all deskevidence reads).
func (f *fakeForge) GetPullRequest(_ deskkit.ForgeRepo, number int) (*deskkit.PullRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.prErr != nil {
		return nil, f.prErr
	}
	return &deskkit.PullRequest{Number: number, HeadSHA: f.prHeadSHA}, nil
}

// GetCommit resolves a commit's attributed account login — the ONLINE seam checkAttribution
// uses to map a GitLab commit to the committing account's username (#1477). An empty
// commitAuthorLogin models a forge that could not resolve the account (could-not-check).
func (f *fakeForge) GetCommit(_ deskkit.ForgeRepo, sha string) (*deskkit.RepoCommit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCommitCalls++
	if f.commitErr != nil {
		return nil, f.commitErr
	}
	return &deskkit.RepoCommit{SHA: sha, AuthorLogin: f.commitAuthorLogin}, nil
}

// setupFake wires isolation: a temp HOME with the fixture roster (so trust/write-auth decisions
// answer the same verdicts they always did), the standard verify-desk environment, a recording
// fake Forge behind forgeForFn, a token-mint stub, a no-op public-repo gate, and captured
// stdout/stderr.
func setupFake(t *testing.T) (*fakeForge, *bytes.Buffer) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "deskevidence-test")
	t.Setenv("DESK_LOOP", "verify-desk")
	t.Setenv("VERIFIER_MAIN_OK", "1")

	f := &fakeForge{}

	oldForge := forgeForFn
	forgeForFn = func(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return f, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	t.Cleanup(func() { forgeForFn = oldForge })

	oldMint := mintTokenFn
	mintTokenFn = func(string) error { ghToken = "test-verifier-token"; return nil }
	t.Cleanup(func() { mintTokenFn = oldMint; ghToken = "" })

	oldGate := publicRepoGateFn
	publicRepoGateFn = func(deskkit.RepoInfoFetcher, string, string) error { return nil }
	t.Cleanup(func() { publicRepoGateFn = oldGate })

	// The statusgen PROBLEM-diff check defaults to "introduces nothing" so the whole
	// behavioural suite never shells a real statusgen or touches a real tree merely by
	// calling cmdEvidence. Tests specifically exercising the check override this seam
	// themselves (see the lint-diff tests below).
	oldLintDiff := lintDiffFn
	lintDiffFn = func(string, string, []byte) ([]string, error) { return nil, nil }
	t.Cleanup(func() { lintDiffFn = oldLintDiff })
	oldOutcome := outcomeGuardFn
	outcomeGuardFn = func(string, string, []byte, []byte, deskkit.Forge, deskkit.ForgeRepo, string) error { return nil }
	t.Cleanup(func() { outcomeGuardFn = oldOutcome })

	var errBuf bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout = &bytes.Buffer{}
	stderr = &errBuf
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })

	return f, &errBuf
}

// writeRepoFile creates a local file under a temp dir (a checked-out repo file). Returns the
// absolute path.
func writeRepoFile(t *testing.T, path, content string) string {
	t.Helper()
	base := t.TempDir()
	abs := filepath.Join(base, path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", abs, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
	return abs
}

// rootWithFile creates a checkout root holding relPath with content, and returns the root — for
// the --root resolution and append-only tests, where the repo-relative path deskevidence reads
// and the path it commits must be the same.
func rootWithFile(t *testing.T, relPath, content string) string {
	t.Helper()
	root := t.TempDir()
	abs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", abs, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
	return root
}

func auditEntries(t *testing.T) []deskkit.Entry {
	t.Helper()
	e, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("load audit: %v", err)
	}
	return e
}

func lastAudit(t *testing.T) deskkit.Entry {
	t.Helper()
	e := auditEntries(t)
	if len(e) == 0 {
		t.Fatal("no audit entries")
	}
	return e[len(e)-1]
}

// --- CLI surface ---

func TestVersionOK(t *testing.T) {
	_, _ = setupFake(t)
	if code := run([]string{"--version"}); code != deskkit.ExitOK {
		t.Fatalf("--version exit = %d, want 0", code)
	}
}

func TestHelpOK(t *testing.T) {
	_, _ = setupFake(t)
	if code := run([]string{"--help"}); code != deskkit.ExitOK {
		t.Fatalf("--help exit = %d, want 0", code)
	}
}

func TestNoArgsRefused(t *testing.T) {
	_, _ = setupFake(t)
	if code := run(nil); code != deskkit.ExitRefused {
		t.Fatalf("no args exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestMissingEvidenceFileRefused(t *testing.T) {
	_, _ = setupFake(t)
	if code := run([]string{"example-org/tracker", "main"}); code != deskkit.ExitRefused {
		t.Fatalf("missing --evidence-file exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestKillSwitchExit3 — DESK_TOOLS_DISABLED=1 → exit 3, one disabled audit line, and NO forge call.
func TestKillSwitchExit3(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", "/dev/null"})
	if code != deskkit.ExitDisabled {
		t.Fatalf("disabled exit = %d, want %d", code, deskkit.ExitDisabled)
	}
	if len(f.hits) != 0 {
		t.Fatalf("expected NO forge calls while disabled, got %v", f.hits)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultDisabled {
		t.Fatalf("last audit result = %q, want disabled", got)
	}
}

// --- Happy path + idempotency ---

// TestSuccessfulCommitEndToEnd: local file → BodyCheck → ReadFile → WriteFile through the seam,
// as the verifier App.
func TestSuccessfulCommitEndToEnd(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "# Brief\n\n## Evidence\n| 1 | ... | evidence row |\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("successful commit exit = %d, want 0", code)
	}
	last := lastAudit(t)
	if last.Result != deskkit.ResultOK {
		t.Fatalf("last audit result = %q, want ok", last.Result)
	}
	if last.BodyDigest == "" {
		t.Fatal("expected non-empty bodyDigest in audit")
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if f.putBranch != "main" {
		t.Fatalf("WriteFile branch = %q, want main", f.putBranch)
	}
	if f.writes[0].File != evidencePath {
		t.Fatalf("WriteFile file = %q, want %q", f.writes[0].File, evidencePath)
	}
}

// TestEvidenceCommitCarriesOnBehalfOfTrailer is multi-principal/01's Verify row 6 at the
// unit level: the commit message landed with an Evidence row carries the on-behalf-of
// git trailer naming the roster's bless login (the fixture roster's
// ASSAY_BLESS_LOGIN=ada:2001).
func TestEvidenceCommitCarriesOnBehalfOfTrailer(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "# Brief\n\n## Evidence\n| 1 | ... | evidence row |\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("commit exit = %d, want 0", code)
	}
	if len(f.writes) != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", len(f.writes))
	}
	msg := f.writes[0].Message
	if !strings.HasSuffix(msg, "On-behalf-of: human:ada mode:unattended") {
		t.Fatalf("commit message = %q, want it to end with the on-behalf-of trailer", msg)
	}
	if !strings.HasPrefix(msg, "Evidence: verification row for "+evidencePath) {
		t.Fatalf("commit message = %q, want it to still start with the evidence-row message", msg)
	}
}

// TestIdempotencyNoop: committing content already on the branch is a noop (exit 0, NO WriteFile).
func TestIdempotencyNoop(t *testing.T) {
	f, _ := setupFake(t)
	content := "# Brief\n\n## Evidence\ncontent already on branch\n"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, content)
	f.setFile(evidencePath, content) // remote == local

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("idempotent noop exit = %d, want 0", code)
	}
	if f.putCalls != 0 {
		t.Fatalf("expected 0 WriteFile calls for noop, got %d", f.putCalls)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultNoop {
		t.Fatalf("noop audit result = %q, want %q", got, deskkit.ResultNoop)
	}
}

// --- Secret scan ---

func TestSecretScanRefused(t *testing.T) {
	f, _ := setupFake(t)
	secret := "ghp_" + strings.Repeat("a", 36)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "token: "+secret+"\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitRefused {
		t.Fatalf("secret-scan exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("a secret-scanned refusal still wrote %d time(s)", f.putCalls)
	}
}

func TestSecretScanRefusedNoRemote(t *testing.T) {
	f, _ := setupFake(t)
	secret := "ghp_" + strings.Repeat("b", 36)
	evidencePath := "docs/streams/x/new.md"
	root := rootWithFile(t, evidencePath, "token: "+secret+"\n")
	// No remote file set → ReadFile 404 (create path); the secret scan must still refuse first.
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitRefused {
		t.Fatalf("secret-scan (no remote) exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("secret-scan refusal wrote %d time(s)", f.putCalls)
	}
}

// TestOversizeRefused: a local file over the byte cap is refused before any forge call.
func TestOversizeRefused(t *testing.T) {
	f, _ := setupFake(t)
	big := strings.Repeat("x", maxBytes+1)
	evidencePath := "docs/streams/x/big.md"
	root := rootWithFile(t, evidencePath, big)
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitRefused {
		t.Fatalf("oversize exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("oversize refusal still reached the forge: %v", f.hits)
	}
}

// bigVerifyOutcomesContent returns synthetic verify-outcomes.jsonl content of AT LEAST
// minBytes, built from repeated realistic rows — not a single run of one repeated byte the
// way strings.Repeat("x", n) is. It exercises the sidecar's real shape (many short JSON
// lines) without also tripping BodyCheck's long-high-entropy-run secret heuristic, which a
// giant single unbroken token (over a few hundred bytes) is exactly shaped to trip.
func bigVerifyOutcomesContent(minBytes int) string {
	var b strings.Builder
	for i := 0; b.Len() < minBytes; i++ {
		fmt.Fprintf(&b, `{"ts": "2026-09-07T01:19:23Z", "brief": "desk-tools/%d", "outcome": "verified", "rows_passed": 5, "rows_total": 5, "sha": "67abbac"}`+"\n", i)
	}
	return b.String()
}

// TestVerifyOutcomesSidecarOversizeAllowed is #1338's fail-first case: a sidecar just over the
// general 262144-byte cap — the same size class that filed the issue (whose own trigger was
// 291722 bytes). Before the fix this refused with "evidence file exceeds 262144 bytes (N)" —
// the same shape as the cited "…(291722)" — because verify-outcomes.jsonl was judged against
// the general maxBytes cap like any other evidence file. After the fix the override cap
// (verifyOutcomesMaxBytes, 4 MiB) applies instead and the commit succeeds.
func TestVerifyOutcomesSidecarOversizeAllowed(t *testing.T) {
	f, errBuf := setupFake(t)
	content := bigVerifyOutcomesContent(maxBytes + 1) // over the general cap, #1338's own size class
	evidencePath := "docs/streams/verify-outcomes.jsonl"
	root := rootWithFile(t, evidencePath, content)

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("verify-outcomes.jsonl at %d bytes exit = %d (stderr %q), want %d",
			len(content), code, errBuf.String(), deskkit.ExitOK)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected exactly 1 write to land, got %d (hits %v)", f.putCalls, f.hits)
	}
	if got := len(f.putContent); got != len(content) {
		t.Fatalf("landed content length = %d, want %d", got, len(content))
	}
}

// TestVerifyOutcomesSidecarStillCapped: the override is a raised ceiling, not an exemption
// (#439's "an unscoped exemption fails OPEN" property, restated for this cap) — a
// verify-outcomes.jsonl write past verifyOutcomesMaxBytes still refuses, naming that cap in
// the message rather than the general one.
func TestVerifyOutcomesSidecarStillCapped(t *testing.T) {
	f, errBuf := setupFake(t)
	content := strings.Repeat("x", verifyOutcomesMaxBytes+1)
	evidencePath := "docs/streams/verify-outcomes.jsonl"
	root := rootWithFile(t, evidencePath, content)

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitRefused {
		t.Fatalf("over-the-override-cap exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("oversize refusal still reached the forge: %v", f.hits)
	}
	wantMsg := fmt.Sprintf("evidence file exceeds %d bytes (%d)", verifyOutcomesMaxBytes, len(content))
	if !strings.Contains(errBuf.String(), wantMsg) {
		t.Fatalf("stderr = %q, want it to contain %q", errBuf.String(), wantMsg)
	}
}

// TestVerifyOutcomesShardOversizeAllowed: a future ROTATION shard (verify-outcomes-<tag>.jsonl,
// #1338 part 2) gets the same raised cap as the canonical unsharded file — the write side must
// not silently drop back to the general cap the moment the file is renamed for rotation.
func TestVerifyOutcomesShardOversizeAllowed(t *testing.T) {
	f, errBuf := setupFake(t)
	content := bigVerifyOutcomesContent(maxBytes + 1)
	evidencePath := "docs/streams/verify-outcomes-2026-10.jsonl"
	root := rootWithFile(t, evidencePath, content)

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("rotation shard at %d bytes exit = %d (stderr %q), want %d",
			len(content), code, errBuf.String(), deskkit.ExitOK)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected exactly 1 write to land, got %d", f.putCalls)
	}
}

// TestVerifyOutcomesSidecarNameNotOverridenOutsideDocsStreamsRoot: the override is keyed on
// the file sitting DIRECTLY under docs/streams/ — a same-named file nested one level deeper
// (a different stream's own artifact that happens to share the basename) must NOT inherit the
// raised cap.
func TestVerifyOutcomesSidecarNameNotOverridenOutsideDocsStreamsRoot(t *testing.T) {
	f, _ := setupFake(t)
	content := strings.Repeat("x", maxBytes+1)
	evidencePath := "docs/streams/some-stream/verify-outcomes.jsonl"
	root := rootWithFile(t, evidencePath, content)

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitRefused {
		t.Fatalf("nested same-named file exit = %d, want %d (must NOT inherit the override cap)", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("oversize refusal still reached the forge: %v", f.hits)
	}
}

// --- Attribution: the three states, driven by the author WriteFile reports ---

func TestCommitAttributionToVerifierApp(t *testing.T) {
	f, _ := setupFake(t)
	f.writeAuthor = "assay-verifier-app[bot]"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("verifier-attributed write exit = %d, want 0", code)
	}
	if !strings.Contains(lastAudit(t).Detail, "assay-verifier-app[bot]") {
		t.Fatalf("audit detail does not record the verifier author: %q", lastAudit(t).Detail)
	}
}

// TestMissingAuthorIsCouldNotCheckNotSuccess: a write the forge reports NO author for is
// could-not-check (warned, exit stays 0), never rounded up to proven.
func TestMissingAuthorIsCouldNotCheckNotSuccess(t *testing.T) {
	f, errBuf := setupFake(t)
	f.emptyAuthor = true
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("empty-author exit = %d, want 0 (could-not-check)", code)
	}
	if !strings.Contains(lastAudit(t).Detail, "could-not-check") {
		t.Fatalf("audit detail does not record could-not-check: %q", lastAudit(t).Detail)
	}
	if !strings.Contains(errBuf.String(), "could NOT verify") {
		t.Fatalf("no could-not-check WARNING on stderr: %q", errBuf.String())
	}
}

// TestCommitAttributedToAnotherAppIsRefused: a write attributed to some OTHER login is
// proven-wrong (exit 6), naming what landed.
func TestCommitAttributedToAnotherAppIsRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.writeAuthor = "some-other-app[bot]"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitUnverifiable {
		t.Fatalf("wrong-author exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(lastAudit(t).Detail, "some-other-app[bot]") {
		t.Fatalf("audit detail does not name the wrong author: %q", lastAudit(t).Detail)
	}
}

// --- Brief merge (the ReadFile op's consumer) ---

// TestSecretScanIgnoresPreexistingBriefBody: BodyCheck must scan only the bytes THIS commit
// adds, never the whole merged file. A brief already carrying a
// secret-shaped run in its PRE-EXISTING body (already reviewed and merged through the normal PR
// path) must not permanently block every future Evidence append to that file. Before the fix,
// this scanned commitContent (the merged whole file) and refused; after the fix, it scans
// localContent (the evidence being added) and lands.
func TestSecretScanIgnoresPreexistingBriefBody(t *testing.T) {
	f, _ := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	preexistingSecret := "ghp_" + strings.Repeat("c", 36)
	f.setFile(briefPath, "# Brief\n\ntoken: "+preexistingSecret+"\n\n## Evidence\n| 1 | a | b |\n")
	evidencePath := writeRepoFile(t, "row.md", "| 2 | c | d |\n")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (pre-existing secret-shaped text on the branch must not block a clean append)", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if !strings.Contains(f.putContent, "| 2 | c | d |") {
		t.Fatalf("merged content missing the new row:\n%s", f.putContent)
	}
}

// TestSecretScanStillRefusesNewSecretInBriefMerge: the companion negative-path row — a secret in
// the EVIDENCE ITSELF (the bytes this commit is actually adding) must still refuse, brief-path
// merge or not. Proves the fix narrowed the scan's SCOPE, not its sensitivity.
func TestSecretScanStillRefusesNewSecretInBriefMerge(t *testing.T) {
	f, _ := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	f.setFile(briefPath, "# Brief\n\n## Evidence\n| 1 | a | b |\n")
	newSecret := "ghp_" + strings.Repeat("d", 36)
	evidencePath := writeRepoFile(t, "row.md", "token: "+newSecret+"\n")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (a secret in the NEW evidence must still refuse)", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("a secret-scanned refusal still wrote %d time(s)", f.putCalls)
	}
}

// --- #966: the SAME added-bytes-only scoping, without --brief-path ---
//
// Without --brief-path, --evidence-file IS the whole target file: the caller merged the new
// row into a local working copy itself before calling this tool, so localContent can
// legitimately be almost entirely content that was ALREADY on the branch. Before the fix,
// BodyCheck scanned that whole file — indistinguishable, at the scan, from the --brief-path
// case #901 already fixed — and a brief carrying a secret-shaped run ANYWHERE in its
// pre-existing body could never receive another Evidence append through this tool, by
// anyone, ever. The fix diffs localContent against the remote content fetched up front and
// scans only the lines addedLines reports as new.

// TestSecretScanIgnoresPreexistingSecretWithoutBriefPath isolates the SCOPING half of the
// fix from the two new allowlist rules below: a plain ghp_ token (not covered by ANY
// allowlist rule) already on the branch must not block a clean append made via the
// direct-write flow. Before the fix this scanned commitContent (== localContent in this
// flow, the WHOLE target file) and refused on the pre-existing token; after the fix it
// scans only addedLines(remoteContent, localContent) and lands.
func TestSecretScanIgnoresPreexistingSecretWithoutBriefPath(t *testing.T) {
	f, _ := setupFake(t)
	preexistingSecret := "ghp_" + strings.Repeat("f", 36)
	remote := "# Brief\n\ntoken: " + preexistingSecret + "\n\n## Evidence\n| 1 | a | b |\n"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, remote+"| 2 | c | d |\n")
	f.setFile(evidencePath, remote)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (pre-existing secret-shaped text on the branch must not block a clean append)", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if !strings.Contains(f.putContent, "| 2 | c | d |") {
		t.Fatalf("committed content missing the new row:\n%s", f.putContent)
	}
}

// TestSecretScanIgnoresPreexistingEnumListWithoutBriefPath: a pre-existing ALL-CAPS
// stream-status enum slash-list (#966's own repro shape) already on the branch must not
// block a clean append made via the direct-write flow (no --brief-path).
func TestSecretScanIgnoresPreexistingEnumListWithoutBriefPath(t *testing.T) {
	f, _ := setupFake(t)
	remote := "# Brief\n\nprior states: PENDING/RUNNING/BLOCKED/FAILED/RETRYING/DONE\n\n## Evidence\n| 1 | a | b |\n"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, remote+"| 2 | c | d |\n")
	f.setFile(evidencePath, remote)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (a pre-existing enum slash-list must not block a clean append)", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if !strings.Contains(f.putContent, "| 2 | c | d |") {
		t.Fatalf("committed content missing the new row:\n%s", f.putContent)
	}
}

// TestSecretScanIgnoresPreexistingKubernetesUIDWithoutBriefPath: a pre-existing Kubernetes
// generated PersistentVolume name (#966's other repro shape) already on the branch must not
// block a clean append made via the direct-write flow.
func TestSecretScanIgnoresPreexistingKubernetesUIDWithoutBriefPath(t *testing.T) {
	f, _ := setupFake(t)
	remote := "# Brief\n\nbound volume: pvc-38d9b7efea064f53" + "acd5843296326c99\n\n## Evidence\n| 1 | a | b |\n"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, remote+"| 2 | c | d |\n")
	f.setFile(evidencePath, remote)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0 (a pre-existing k8s generated UID must not block a clean append)", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if !strings.Contains(f.putContent, "| 2 | c | d |") {
		t.Fatalf("committed content missing the new row:\n%s", f.putContent)
	}
}

// TestSecretScanStillRefusesNewSecretWithoutBriefPath: the companion negative-path row for
// the direct-write flow — a secret in the bytes this commit ACTUALLY adds must still refuse.
// Proves #966's fix narrowed the scan's scope, not its sensitivity: addedLines still hands
// the scanner the newly-added line, and BodyCheck still refuses it.
func TestSecretScanStillRefusesNewSecretWithoutBriefPath(t *testing.T) {
	f, _ := setupFake(t)
	remote := "# Brief\n\n## Evidence\n| 1 | a | b |\n"
	newSecret := "ghp_" + strings.Repeat("e", 36)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, remote+"token: "+newSecret+"\n")
	f.setFile(evidencePath, remote)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (a secret in the NEWLY ADDED bytes must still refuse)", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("a secret-scanned refusal still wrote %d time(s)", f.putCalls)
	}
}

func TestMergeEvidenceIntoBrief(t *testing.T) {
	f, _ := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	f.setFile(briefPath, "# Brief\n\n## Evidence\n| 1 | a | b |\n")
	evidencePath := writeRepoFile(t, "row.md", "| 2 | c | d |\n")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitOK {
		t.Fatalf("brief-merge exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if f.writes[0].File != briefPath {
		t.Fatalf("WriteFile targeted %q, want the brief %q", f.writes[0].File, briefPath)
	}
	// The merged content keeps the existing row AND appends the new one.
	if !strings.Contains(f.putContent, "| 1 | a | b |") || !strings.Contains(f.putContent, "| 2 | c | d |") {
		t.Fatalf("merged content missing a row:\n%s", f.putContent)
	}
}

// --- Block-level idempotency: an Evidence block equivalent to one already standing is a no-op ---

// TestEquivalentEvidenceBlockIsNoop: a fresh block byte-equivalent to the block already standing
// under ## Evidence (past a leading placeholder comment) is a no-op — exit 0, NO WriteFile, audit
// noop, and the noop line on stdout.
func TestEquivalentEvidenceBlockIsNoop(t *testing.T) {
	f, _ := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	block := "| 1 | check:ci | go build | 0 |\n| 2 | check:ci | go test | 0 |"
	// The brief already carries a placeholder comment AND this exact block under ## Evidence.
	f.setFile(briefPath, "# Brief\n\n## Evidence\n<!-- appended at implementation time -->\n"+block+"\n")
	evidencePath := writeRepoFile(t, "row.md", block+"\n")

	stdoutBuf := stdout.(*bytes.Buffer)
	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitOK {
		t.Fatalf("equivalent-block noop exit = %d, want 0", code)
	}
	if f.putCalls != 0 {
		t.Fatalf("expected 0 WriteFile for an equivalent block, got %d", f.putCalls)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultNoop {
		t.Fatalf("audit result = %q, want %q", got, deskkit.ResultNoop)
	}
	if !strings.Contains(stdoutBuf.String(), "noop: Evidence block already present") {
		t.Fatalf("stdout missing the block-already-present noop line: %q", stdoutBuf.String())
	}
}

// TestEquivalenceSurvivesLineEndingsAndTrailingSpace: a fresh block that differs from the
// standing one ONLY by CRLF line endings and trailing whitespace is still equivalent — a
// Windows-authored re-run must not defeat the check.
func TestEquivalenceSurvivesLineEndingsAndTrailingSpace(t *testing.T) {
	f, _ := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	standing := "| 1 | check:ci | go build | 0 |\n| 2 | check:ci | go test | 0 |"
	f.setFile(briefPath, "# Brief\n\n## Evidence\n"+standing+"\n")
	// Byte-different from the standing block: CRLF line endings and trailing spaces/tabs.
	fresh := "| 1 | check:ci | go build | 0 |  \r\n| 2 | check:ci | go test | 0 |\t\r\n"
	evidencePath := writeRepoFile(t, "row.md", fresh)

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitOK {
		t.Fatalf("CRLF/trailing-space equivalent exit = %d, want 0", code)
	}
	if f.putCalls != 0 {
		t.Fatalf("CRLF/trailing-space equivalent block should be a noop, got %d PUT(s)", f.putCalls)
	}
	if got := lastAudit(t).Result; got != deskkit.ResultNoop {
		t.Fatalf("audit result = %q, want %q", got, deskkit.ResultNoop)
	}
}

// TestNearEquivalentBlocksStillLand is the NEGATIVE control: a block differing by one character,
// a partial (prefix) re-run, and a superset that adds a new row each LAND (a PUT is recorded) —
// the equivalence check must never swallow genuinely new evidence.
func TestNearEquivalentBlocksStillLand(t *testing.T) {
	standing := "| 1 | check:ci | go build | 0 |\n| 2 | check:ci | go test | 0 |"
	cases := []struct {
		name  string
		fresh string
	}{
		{"one-character-difference", "| 1 | check:ci | go build | 0 |\n| 2 | check:ci | go test | 1 |"},
		{"prefix-partial-rerun", "| 1 | check:ci | go build | 0 |"},
		{"superset-adds-a-row", standing + "\n| 3 | check:ci | go vet | 0 |"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := setupFake(t)
			briefPath := "docs/streams/x/brief.md"
			f.setFile(briefPath, "# Brief\n\n## Evidence\n"+standing+"\n")
			evidencePath := writeRepoFile(t, "row.md", tc.fresh+"\n")

			code := run([]string{"example-org/tracker", "main",
				"--evidence-file", evidencePath, "--brief-path", briefPath})
			if code != deskkit.ExitOK {
				t.Fatalf("%s exit = %d, want 0", tc.name, code)
			}
			if f.putCalls != 1 {
				t.Fatalf("%s: new content must land — expected 1 WriteFile, got %d", tc.name, f.putCalls)
			}
			if got := lastAudit(t).Result; got != deskkit.ResultOK {
				t.Fatalf("%s: audit result = %q, want %q", tc.name, got, deskkit.ResultOK)
			}
		})
	}
}

// --- The Evidence lane on a forge whose default branch takes no direct write (Verify row 10) ---

// TestEvidenceLandsAsChangeWhenDefaultBranchClosed: with the resolved forge reporting the
// default branch not directly writable, the run performs NO direct write to that branch, lands
// the row on a side branch, opens a draft change, and exits 0 with the change named on stdout.
func TestEvidenceLandsAsChangeWhenDefaultBranchClosed(t *testing.T) {
	f, _ := setupFake(t)
	f.defaultBranch = "main" // a WriteFile to main returns the sentinel, records nothing
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "row\n")

	stdoutBuf := stdout.(*bytes.Buffer)
	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("closed-default-branch exit = %d, want 0", code)
	}

	// NO direct write to the default branch landed — every recorded write is to a side branch.
	for _, w := range f.writes {
		if w.Branch == "main" {
			t.Fatalf("a direct write to the closed default branch was recorded: %+v", w)
		}
	}
	if len(f.writes) != 1 {
		t.Fatalf("expected exactly 1 side-branch write, got %d", len(f.writes))
	}
	side := f.writes[0].Branch
	if !strings.HasPrefix(side, "evidence/") {
		t.Fatalf("side branch %q is not an evidence/* branch", side)
	}
	if f.writes[0].StartBranch != "main" {
		t.Fatalf("side-branch write StartBranch = %q, want main (cut from the closed default)", f.writes[0].StartBranch)
	}
	// A draft change was opened from the side branch onto the default branch.
	if len(f.changes) != 1 {
		t.Fatalf("expected exactly 1 draft change, got %d", len(f.changes))
	}
	if f.changes[0].Head != side || f.changes[0].Base != "main" {
		t.Fatalf("draft change head/base = %q/%q, want %q/main", f.changes[0].Head, f.changes[0].Base, side)
	}
	if !strings.Contains(stdoutBuf.String(), "draft") {
		t.Fatalf("stdout does not name the draft change: %q", stdoutBuf.String())
	}
}

// --- Cheap refusals ---

func TestBadRepoRefused(t *testing.T) {
	_, _ = setupFake(t)
	if code := run([]string{"not-a-repo", "main", "--evidence-file", "/dev/null"}); code != deskkit.ExitRefused {
		t.Fatalf("bad repo exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestMissingBranchRefused(t *testing.T) {
	_, _ = setupFake(t)
	if code := run([]string{"example-org/tracker"}); code != deskkit.ExitRefused {
		t.Fatalf("missing branch exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestUnpinnedWarning(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	// WarnIfUnpinned writes to stderr when the binary is unstamped (the test binary is); its
	// exact wording is deskkit's concern, so this just proves the run completed clean.
}

func TestAuditFields(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	last := lastAudit(t)
	if last.Tool != "deskevidence" || last.Verb != "commit" {
		t.Fatalf("audit tool/verb = %q/%q", last.Tool, last.Verb)
	}
	if last.Repo != "example-org/tracker" {
		t.Fatalf("audit repo = %q", last.Repo)
	}
	if last.BodyDigest == "" {
		t.Fatal("audit bodyDigest empty")
	}
}

// --- Repo-set gate ---

func TestRepoNotInSetRefused(t *testing.T) {
	f, _ := setupFake(t)
	if code := run([]string{"random-org/random-repo", "main", "--evidence-file", "/dev/null"}); code != deskkit.ExitRefused {
		t.Fatalf("repo-not-in-set exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("repo-set refusal reached the forge: %v", f.hits)
	}
}

func TestRepoInSetNotRefusedByRepoGate(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"medici-finance/assay", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("in-set repo exit = %d, want 0", code)
	}
}

// --- Main-branch guard ---

func TestMainBranchRefusedWithoutSanction(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "")
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitRefused {
		t.Fatalf("main without sanction exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("main-guard refusal reached the forge: %v", f.hits)
	}
}

func TestMainBranchAllowedWithSanction(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "1")
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("main with sanction exit = %d, want 0", code)
	}
}

func TestNonMainBranchNeedsNoSanction(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "")
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "feat/x", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("non-main branch exit = %d, want 0", code)
	}
}

func TestMainSanctionMustBeExactlyOne(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "0")
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitRefused {
		t.Fatalf("VERIFIER_MAIN_OK=0 exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("main-guard refusal reached the forge: %v", f.hits)
	}
}

func TestFullRefBranchStillGuarded(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "")
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	if code := run([]string{"example-org/tracker", "refs/heads/main", "--evidence-file", evidencePath}); code != deskkit.ExitRefused {
		t.Fatalf("refs/heads/main exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("refs/heads/main refusal reached the forge: %v", f.hits)
	}
}

func TestFullRefNonMainNotGuarded(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "")
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "refs/heads/feat/x", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("refs/heads/feat/x exit = %d, want 0", code)
	}
}

// --- STATUS.md guard ---

func TestStatusMDRefusedAsEvidenceFile(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := writeRepoFile(t, "STATUS.md", "generated\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitRefused {
		t.Fatalf("STATUS.md evidence-file exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("STATUS.md refusal reached the forge: %v", f.hits)
	}
}

func TestStatusMDRefusedAsBriefPath(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := writeRepoFile(t, "row.md", "row\n")
	if code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", "STATUS.md"}); code != deskkit.ExitRefused {
		t.Fatalf("STATUS.md brief-path exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("STATUS.md brief-path refusal reached the forge: %v", f.hits)
	}
}

func TestNonStatusFileStillCommits(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/NOTSTATUS.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("non-STATUS file exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
}

// --- Public-repo gate (stubbed at the seam) ---

func TestPublicRepoGateRefusesCommitToPublicRepo(t *testing.T) {
	f, _ := setupFake(t)
	publicRepoGateFn = func(deskkit.RepoInfoFetcher, string, string) error {
		return deskkit.Refused("public repo: not authorized by a listed :public allowed-repos entry")
	}
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitRefused {
		t.Fatalf("public-repo gate exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("public-repo gate refusal still wrote %d time(s)", f.putCalls)
	}
}

func TestPublicRepoGatePassesPrivateRepo(t *testing.T) {
	f, _ := setupFake(t)
	// Default stub returns nil (private/internal passes through).
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("private-repo exit = %d, want 0", code)
	}
}

// --- --root local resolution ---

func TestRootResolvesEvidenceFileAgainstCheckout(t *testing.T) {
	f, _ := setupFake(t)
	rel := "docs/streams/x/brief.md"
	root := rootWithFile(t, rel, "from the right checkout\n")
	f.setFile(rel, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", rel, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("--root exit = %d, want 0", code)
	}
	if f.putContent != "from the right checkout\n" {
		t.Fatalf("committed content = %q, want the --root checkout's copy", f.putContent)
	}
}

func TestRootWithAbsoluteEvidenceFileRefused(t *testing.T) {
	f, _ := setupFake(t)
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", "/abs/path.md", "--root", t.TempDir()}); code != deskkit.ExitRefused {
		t.Fatalf("--root with absolute --evidence-file exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("contradiction refusal reached the forge: %v", f.hits)
	}
}

// --- Append-only shrink guard ---

func TestAppendOnlyShrinkRefused(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/x/rows.jsonl"
	root := rootWithFile(t, target, "{\"a\":1}\n")         // local has 1 row
	f.setFile(target, "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n") // remote has 3
	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("append-only shrink exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("shrink refusal still wrote %d time(s)", f.putCalls)
	}
}

func TestAppendOnlyShrinkOverride(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/x/rows.jsonl"
	root := rootWithFile(t, target, "{\"a\":1}\n")
	f.setFile(target, "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root, "--allow-shrink"})
	if code != deskkit.ExitOK {
		t.Fatalf("append-only shrink with --allow-shrink exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile with --allow-shrink, got %d", f.putCalls)
	}
}

func TestAppendOnlyGrowthAllowed(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/x/rows.jsonl"
	root := rootWithFile(t, target, "{\"a\":1}\n{\"b\":2}\n")
	f.setFile(target, "{\"a\":1}\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("append-only growth exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile for growth, got %d", f.putCalls)
	}
}

func TestNonJSONLShrinkNotBlockedWithoutFlag(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/x/notes.md"
	root := rootWithFile(t, target, "line1\n")
	f.setFile(target, "line1\nline2\nline3\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("non-jsonl shrink (no flag) exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile for a non-jsonl shrink, got %d", f.putCalls)
	}
}

func TestAppendOnlyFlagBlocksNonJSONLShrink(t *testing.T) {
	f, _ := setupFake(t)
	target := "docs/streams/x/notes.md"
	root := rootWithFile(t, target, "line1\n")
	f.setFile(target, "line1\nline2\nline3\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root, "--append-only"})
	if code != deskkit.ExitRefused {
		t.Fatalf("non-jsonl shrink with --append-only exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("append-only refusal still wrote %d time(s)", f.putCalls)
	}
}

// --- docs/streams/ scoping guard (issue: deskevidence: refuse a landing that adds a
// statusgen PROBLEM or lands outside docs/streams/) ---

// TestTargetOutsideDocsStreamsRefused: a target path at the repo root (the "stray root
// file" main-red shape the issue names) is refused before any network call.
func TestTargetOutsideDocsStreamsRefused(t *testing.T) {
	f, _ := setupFake(t)
	target := "STRAY-ROOT-FILE.md"
	root := rootWithFile(t, target, "content\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("outside-docs/streams exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("outside-docs/streams refusal reached the forge: %v", f.hits)
	}
	if f.putCalls != 0 {
		t.Fatalf("outside-docs/streams refusal still wrote %d time(s)", f.putCalls)
	}
}

// TestBriefPathOutsideDocsStreamsRefused: the SAME guard on --brief-path, since that (not
// --evidence-file) is the path actually committed when both flags are given.
func TestBriefPathOutsideDocsStreamsRefused(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/row.md"
	root := rootWithFile(t, evidencePath, "| 2 | c | d |\n")
	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", "ROOT-BRIEF.md", "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("outside-docs/streams (brief-path) exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("outside-docs/streams (brief-path) refusal reached the forge: %v", f.hits)
	}
}

// TestTraversalEscapeOutsideDocsStreamsRefused: a directory-traversal escape out of
// docs/streams/ is refused on its CLEANED form — path.Clean collapses the ../ segments, so
// the escape shows up as a path failing the prefix check rather than surviving as a literal
// "..".
func TestTraversalEscapeOutsideDocsStreamsRefused(t *testing.T) {
	_, _ = setupFake(t)
	target := "docs/streams/../../etc/passwd"
	// Placed so the local read succeeds (filepath.Join cleans the same ../../ the guard's own
	// path.Clean does, so this lands at exactly where --root resolution would look) — the
	// refusal under test is the docs/streams scoping guard, not an incidental "file not
	// found", so the escape must be reachable to prove the guard (not the local read) is
	// what catches it.
	root := rootWithFile(t, target, "content\n")
	code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("traversal-escape exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

// TestAbsoluteTargetOutsideDocsStreamsRefused: an absolute --evidence-file with no --root
// is refused by the docs/streams guard — a Contents-API repo path is never absolute in real
// use.
func TestAbsoluteTargetOutsideDocsStreamsRefused(t *testing.T) {
	f, _ := setupFake(t)
	code := run([]string{"example-org/tracker", "main", "--evidence-file", "/etc/passwd"})
	if code != deskkit.ExitRefused {
		t.Fatalf("absolute-target exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("absolute-target refusal reached the forge: %v", f.hits)
	}
}

// --- statusgen PROBLEM-diff guard (issue: deskevidence: refuse a landing that adds a
// statusgen PROBLEM or lands outside docs/streams/) ---
//
// These drive cmdEvidence through the top-level lintDiffFn seam — the same seam setupFake
// stubs to "introduces nothing" by default — so they exercise exactly what cmdEvidence does
// with the seam's answer, independent of lintDiffAt's own staging/diff mechanics (covered
// separately in lintdiff_test.go).

// TestLintDiffIntroducedProblemRefused: lintDiffFn reporting an introduced PROBLEM refuses
// the landing (exit 5, naming the PROBLEM line) before any write.
func TestLintDiffIntroducedProblemRefused(t *testing.T) {
	f, _ := setupFake(t)
	const problemLine = "PROBLEM: docs/streams/x/brief.md: backticked path \"../sibling/x\" does not exist — " +
		"for a sibling-repo file, prefix it ../<repo>/../sibling/x"
	lintDiffFn = func(root, target string, content []byte) ([]string, error) {
		return []string{problemLine}, nil
	}
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("introduced-PROBLEM exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("introduced-PROBLEM refusal still wrote %d time(s)", f.putCalls)
	}
	// The audit line carries the refusal's error text (ac.finalize sets detail = err.Error()
	// on every non-nil-error path), so it is where the introduced PROBLEM line's own
	// ../<repo>/ hint — passed through verbatim, never re-summarised — is checked.
	if !strings.Contains(lastAudit(t).Detail, problemLine) {
		t.Fatalf("audit detail does not carry the introduced PROBLEM line verbatim (including its ../<repo>/ hint): %q", lastAudit(t).Detail)
	}
}

// TestLintDiffCleanCommits: lintDiffFn reporting no introduced problems lets a clean
// landing through — the default setupFake stub already proves this for every OTHER test in
// the suite; this test additionally proves lintDiffFn is actually CALLED with the landing's
// own root/target/content, not skipped.
func TestLintDiffCleanCommits(t *testing.T) {
	f, _ := setupFake(t)
	var gotRoot, gotTarget string
	var gotContent []byte
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	lintDiffFn = func(r, target string, content []byte) ([]string, error) {
		gotRoot, gotTarget, gotContent = r, target, content
		return nil, nil
	}
	f.setFile(evidencePath, "old\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("clean lint-diff exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if gotRoot != root {
		t.Fatalf("lintDiffFn root = %q, want %q", gotRoot, root)
	}
	if gotTarget != evidencePath {
		t.Fatalf("lintDiffFn target = %q, want %q", gotTarget, evidencePath)
	}
	if string(gotContent) != "content\n" {
		t.Fatalf("lintDiffFn content = %q, want %q", gotContent, "content\n")
	}
}

// TestLintDiffCouldNotCheckIsUnverifiable: lintDiffFn itself returning an error (statusgen
// not on PATH, in production) is could-not-check — Unverifiable (exit 6), never rounded up
// to a pass and never treated as the specific "introduced a PROBLEM" refusal (exit 5).
func TestLintDiffCouldNotCheckIsUnverifiable(t *testing.T) {
	f, _ := setupFake(t)
	lintDiffFn = func(string, string, []byte) ([]string, error) {
		return nil, deskkit.Unverifiable("statusgen is not on PATH", nil)
	}
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "content\n")
	f.setFile(evidencePath, "old\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("could-not-check exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if f.putCalls != 0 {
		t.Fatalf("could-not-check still wrote %d time(s)", f.putCalls)
	}
}

// TestLintDiffSkippedForNoop: an idempotent noop (content already on the branch) never
// reaches the lint-diff check at all — nothing is landing, so there is nothing to lint.
func TestLintDiffSkippedForNoop(t *testing.T) {
	f, _ := setupFake(t)
	called := false
	lintDiffFn = func(string, string, []byte) ([]string, error) {
		called = true
		return nil, nil
	}
	content := "# Brief\n\n## Evidence\ncontent already on branch\n"
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, content)
	f.setFile(evidencePath, content)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("noop exit = %d, want 0", code)
	}
	if called {
		t.Fatal("lintDiffFn was called for a noop landing — nothing is landing, nothing to lint")
	}
}

// TestDryRunPrintsPlanNoWrite (verify-integrity/04 item 1, Verify row 4): --dry-run on a
// fixture brief prints the commits-API landing plan and performs ZERO writes — every gate
// that can refuse the landing still runs (mint, forge resolution, remote read, secret scan,
// lint-diff), only the write itself is skipped. There is no local `git commit` anywhere in
// this tool's write path (mintTokenFn → forgeForFn → fg.WriteFile is the only path this
// package has ever had), so a plan that never reaches fg.WriteFile is, by construction, a
// plan with no local git commit in it.
func TestDryRunPrintsPlanNoWrite(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "# Brief\n\n## Evidence\n| 1 | ... | evidence row |\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root, "--dry-run"})
	if code != deskkit.ExitOK {
		t.Fatalf("dry-run exit = %d, want %d", code, deskkit.ExitOK)
	}
	if f.putCalls != 0 {
		t.Fatalf("--dry-run must never write: got %d WriteFile call(s)", f.putCalls)
	}
	// The read (mint, forge resolve, remote fetch, secret scan, lint-diff) DID run — a
	// dry-run is a real preview, not a no-op that skips validation too.
	if len(f.reads) == 0 {
		t.Fatal("--dry-run should still resolve the forge and read the remote content")
	}
	out := stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("stdout must print the dry-run plan; got:\n%s", out)
	}
	if !strings.Contains(out, evidencePath) || !strings.Contains(out, "main") {
		t.Fatalf("the plan must name the target path and branch; got:\n%s", out)
	}
}

// TestDryRunRefusesOnMintFailure (Verify row 4's perturbation): with the App token
// unmintable, --dry-run refuses exactly like the non-dry-run path (TestMintFailureAbortsBeforeForge)
// — it never falls back to a local git identity, because dry-run only skips the FINAL write
// step and the mint happens far earlier, unconditionally.
func TestDryRunRefusesOnMintFailure(t *testing.T) {
	f, _ := setupFake(t)
	mintErr := deskkit.Unverifiable("desktoken verifier --repo example-org/tracker: mint refused",
		errors.New("mint boom"))

	oldMint := mintTokenFn
	mintTokenFn = func(string) error { return mintErr }
	t.Cleanup(func() { mintTokenFn = oldMint })

	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath, "# Brief\n\n## Evidence\n| 1 | ... | row |\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root, "--dry-run"})
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("dry-run with mint failure exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if len(f.hits) != 0 {
		t.Fatalf("mint failed but the forge was reached: %v", f.hits)
	}
	if f.putCalls != 0 {
		t.Fatalf("mint failed but %d WriteFile call(s) were made", f.putCalls)
	}
}
