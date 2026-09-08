package main

import (
	"bytes"
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
	publicRepoGateFn = func(deskkit.RepoInfoFetcher, string, string, int) error { return nil }
	t.Cleanup(func() { publicRepoGateFn = oldGate })

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
	evidencePath := writeRepoFile(t, "docs/brief.md", "# Brief\n\n## Evidence\n| 1 | ... | evidence row |\n")
	f.setFile(evidencePath, "# Brief\n\n## Evidence\n")

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
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

// TestIdempotencyNoop: committing content already on the branch is a noop (exit 0, NO WriteFile).
func TestIdempotencyNoop(t *testing.T) {
	f, _ := setupFake(t)
	content := "# Brief\n\n## Evidence\ncontent already on branch\n"
	evidencePath := writeRepoFile(t, "docs/brief.md", content)
	f.setFile(evidencePath, content) // remote == local

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
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
	evidencePath := writeRepoFile(t, "docs/brief.md", "token: "+secret+"\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitRefused {
		t.Fatalf("secret-scan exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("a secret-scanned refusal still wrote %d time(s)", f.putCalls)
	}
}

func TestSecretScanRefusedNoRemote(t *testing.T) {
	f, _ := setupFake(t)
	secret := "ghp_" + strings.Repeat("b", 36)
	evidencePath := writeRepoFile(t, "docs/new.md", "token: "+secret+"\n")
	// No remote file set → ReadFile 404 (create path); the secret scan must still refuse first.
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitRefused {
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
	evidencePath := writeRepoFile(t, "docs/big.md", big)
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitRefused {
		t.Fatalf("oversize exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if len(f.hits) != 0 {
		t.Fatalf("oversize refusal still reached the forge: %v", f.hits)
	}
}

// --- Attribution: the three states, driven by the author WriteFile reports ---

func TestCommitAttributionToVerifierApp(t *testing.T) {
	f, _ := setupFake(t)
	f.writeAuthor = "assay-verifier-app[bot]"
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
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
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
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
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")

	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitUnverifiable {
		t.Fatalf("wrong-author exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(lastAudit(t).Detail, "some-other-app[bot]") {
		t.Fatalf("audit detail does not name the wrong author: %q", lastAudit(t).Detail)
	}
}

// --- Brief merge (the ReadFile op's consumer) ---

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
	evidencePath := writeRepoFile(t, "docs/brief.md", "row\n")

	stdoutBuf := stdout.(*bytes.Buffer)
	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath})
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
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	// WarnIfUnpinned writes to stderr when the binary is unstamped (the test binary is); its
	// exact wording is deskkit's concern, so this just proves the run completed clean.
}

func TestAuditFields(t *testing.T) {
	f, _ := setupFake(t)
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
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
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"medici-finance/assay", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
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
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
		t.Fatalf("main with sanction exit = %d, want 0", code)
	}
}

func TestNonMainBranchNeedsNoSanction(t *testing.T) {
	f, _ := setupFake(t)
	t.Setenv("VERIFIER_MAIN_OK", "")
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "feat/x", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
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
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "refs/heads/feat/x", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
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
	evidencePath := writeRepoFile(t, "docs/NOTSTATUS.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
		t.Fatalf("non-STATUS file exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
}

// --- Public-repo gate (stubbed at the seam) ---

func TestPublicRepoGateRefusesCommitToPublicRepo(t *testing.T) {
	f, _ := setupFake(t)
	publicRepoGateFn = func(deskkit.RepoInfoFetcher, string, string, int) error {
		return deskkit.Unverifiable("public repo: a file write has no reactions surface", nil)
	}
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitUnverifiable {
		t.Fatalf("public-repo gate exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if f.putCalls != 0 {
		t.Fatalf("public-repo gate refusal still wrote %d time(s)", f.putCalls)
	}
}

func TestPublicRepoGatePassesPrivateRepo(t *testing.T) {
	f, _ := setupFake(t)
	// Default stub returns nil (private/internal passes through).
	evidencePath := writeRepoFile(t, "docs/brief.md", "content\n")
	f.setFile(evidencePath, "old\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath}); code != deskkit.ExitOK {
		t.Fatalf("private-repo exit = %d, want 0", code)
	}
}

// --- --root local resolution ---

func TestRootResolvesEvidenceFileAgainstCheckout(t *testing.T) {
	f, _ := setupFake(t)
	rel := "docs/brief.md"
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
	target := "docs/rows.jsonl"
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
	target := "docs/rows.jsonl"
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
	target := "docs/rows.jsonl"
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
	target := "docs/notes.md"
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
	target := "docs/notes.md"
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
