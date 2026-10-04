package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verifyflip_test.go — red-first tests for `statusgen verifyflip` (#2074).
// Every refusal is a case here, and each asserts the README is left
// byte-identical: a refusal that wrote anything is a failure.

const (
	vfRunner = "assay-verifier-app[bot]"
	vfSHA    = "b988d175ab12"
	vfStamp  = "2026-08-13 " + vfRunner + " @ " + vfSHA
	vfRow1   = "| 1 | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | " + vfRunner + " @ " + vfSHA + " |"
	vfRow2   = "| 2 | `false` | pass exit=1 | sha256:aa11bb22cc33 | 2026-08-13 | " + vfRunner + " @ " + vfSHA + " |"
	vfPass   = "**VERIFY: PASS** — both rows green."
	vfRisk   = "{regulatory: no, customer: no, irreversible: no, sensitive-data: no}"
	vfReview = "2026-08-01 human:pat"
	vfVerify = "| # | Command | Expect |\n|---|---------|--------|\n" +
		"| 1 | `true` | exit 0 |\n| 2 | `false` | exit 1 |\n"

	// Commit authors, matched against the package fixture roster.
	vfMailVerifier = "300000005+assay-verifier-app[bot]@users.noreply.github.com"
	vfMailWorker   = "300000006+assay-worker-app[bot]@users.noreply.github.com"
	vfMailHuman    = "100001+ada@users.noreply.github.com"
)

type vfOpts struct {
	gate, risk, status, verified, evidence, verify, author string
}

func vfDefaults() vfOpts {
	return vfOpts{gate: "model", risk: vfRisk, status: "implemented", verified: "—",
		evidence: witnessTableFor(vfRow1, vfRow2) + "\n\n" + vfPass, verify: vfVerify,
		author: vfMailVerifier}
}

// vfFixture lays a one-brief stream on disk, commits it as o.author (the
// verifier by default) in a fresh git repo, and returns the root and the
// README path.
func vfFixture(t *testing.T, o vfOpts) (root, readme string) {
	t.Helper()
	root = t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "vf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "---\nbrief: vf/01\ntitle: A flip fixture\nwave: 0\ndepends: []\nunblocks: []\neffort: S\n" +
		"gate: " + o.gate + "\nrisk: " + o.risk + "\nissues: []\nschema: brief-v1\n" +
		"authored: 2026-08-13 by fixture\nsources: [\"fixture\"]\n---\n\n# Brief 01\n\n" +
		"## Verify\n" + o.verify + "\n## Evidence\n" + o.evidence +
		"\n\n## Review\nGate: model.\n"
	if err := os.WriteFile(filepath.Join(dir, "brief-01-flip.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	readme = filepath.Join(dir, "README.md")
	body := "---\nstream: vf\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# VF\n\n## Briefs\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [t](./brief-01-flip.md) | 0 | S | " + o.status + " | " + o.verified + " | " + vfReview + " |\n"
	if err := os.WriteFile(readme, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	vfGit(t, root, o.author, "init", "-q")
	vfCommit(t, root, o.author)
	return root, readme
}

// vfGit runs git in root with author and committer pinned to mail.
func vfGit(t *testing.T, root, mail string, args ...string) {
	t.Helper()
	name, _, _ := strings.Cut(mail, "@")
	runGitEnv(t, root, []string{"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + mail,
		"GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + mail},
		append([]string{"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
}

// vfCommit commits every change in root as mail.
func vfCommit(t *testing.T, root, mail string) {
	t.Helper()
	vfGit(t, root, mail, "add", "-A")
	vfGit(t, root, mail, "commit", "-q", "-m", "fixture")
}

// vfBrief is the fixture brief's path.
func vfBrief(root string) string {
	return filepath.Join(root, "docs", "streams", "vf", "brief-01-flip.md")
}

// vfAppend appends text to the fixture brief's Evidence section.
func vfAppend(t *testing.T, root, text string) {
	t.Helper()
	raw, err := os.ReadFile(vfBrief(root))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(raw), "\n\n## Review\n", text+"\n\n## Review\n", 1)
	if err := os.WriteFile(vfBrief(root), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runVF(t *testing.T, root string, extra ...string) (int, string, string) {
	t.Helper()
	args := append([]string{"--root", root, "--brief", "vf/01", "--sha", vfSHA, "--runner", vfRunner}, extra...)
	var out, errb bytes.Buffer
	code := runVerifyflip(args, &out, &errb)
	return code, out.String(), errb.String()
}

// vfRefuses asserts exit 1, a stderr naming want, and an untouched README.
func vfRefuses(t *testing.T, o vfOpts, want string, extra ...string) {
	t.Helper()
	root, readme := vfFixture(t, o)
	before, _ := os.ReadFile(readme)
	code, out, errOut := runVF(t, root, extra...)
	if code != verifyflipExitRefused {
		t.Fatalf("exit = %d, want %d (refused); out=%q err=%q", code, verifyflipExitRefused, out, errOut)
	}
	if !strings.Contains(errOut, want) {
		t.Fatalf("stderr = %q, want it to contain %q", errOut, want)
	}
	after, _ := os.ReadFile(readme)
	if !bytes.Equal(before, after) {
		t.Fatalf("a refusal wrote the README:\n%s", after)
	}
}

func TestVflipHappyPath(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	before, _ := os.ReadFile(readme)
	code, out, errOut := runVF(t, root)
	if code != verifyflipExitOK {
		t.Fatalf("exit = %d, want 0; out=%q err=%q", code, out, errOut)
	}
	if !strings.Contains(out, "verified-stamp: "+vfStamp) {
		t.Fatalf("stdout = %q, want the derived stamp %q", out, vfStamp)
	}
	after, _ := os.ReadFile(readme)
	want := strings.Replace(string(before),
		"| implemented | — | "+vfReview+" |", "| verified | "+vfStamp+" | "+vfReview+" |", 1)
	if string(after) != want {
		t.Fatalf("README after flip:\n%s\nwant (only Status and Verified changed, Reviewed untouched):\n%s", after, want)
	}
}

func TestVflipDryRunNoWrite(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	before, _ := os.ReadFile(readme)
	code, out, errOut := runVF(t, root, "--dry-run")
	if code != verifyflipExitOK || !strings.Contains(out, "verified-stamp: "+vfStamp) {
		t.Fatalf("exit = %d out=%q err=%q", code, out, errOut)
	}
	after, _ := os.ReadFile(readme)
	if !bytes.Equal(before, after) {
		t.Fatal("--dry-run wrote the README")
	}
}

// TestVflipLatestRunOnly: an earlier run at another sha, closed by a BLOCKED
// verdict, does not count against the later PASS — only the PASS's own rows
// are read.
func TestVflipLatestRunOnly(t *testing.T) {
	o := vfDefaults()
	old := strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, "0123456789ab")
	o.evidence = old + "\n\nVERIFY: BLOCKED. row 2 did not execute.\n\n" + o.evidence
	root, _ := vfFixture(t, o)
	if code, out, errOut := runVF(t, root, "--dry-run"); code != verifyflipExitOK {
		t.Fatalf("exit = %d, want 0; out=%q err=%q", code, out, errOut)
	}
}

func TestVflipVerdictFailAfter(t *testing.T) {
	o := vfDefaults()
	o.evidence += "\n\n**VERIFY: FAIL** — row 1 regressed."
	vfRefuses(t, o, "verdict mismatch")
}

func TestVflipVerdictBlocked(t *testing.T) {
	o := vfDefaults()
	o.evidence += "\n\nVERIFY: BLOCKED — row 2 could not run."
	vfRefuses(t, o, "verdict mismatch")
}

func TestVflipNoVerdict(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2)
	vfRefuses(t, o, "verdict mismatch")
}

func TestVflipLenientPass(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2) + "\n\nVERIFY: PASS. Both rows green."
	vfRefuses(t, o, "non-strict PASS")
}

func TestVflipLooseBoldPass(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2) + "\n\n**Non-implementer run — VERIFY: PASS**"
	vfRefuses(t, o, "non-strict PASS")
}

// TestVflipQuotedPass: a strict marker quoted in inline code after a FAIL is
// a quotation, not a verdict — the FAIL stays the latest verdict.
func TestVflipQuotedPass(t *testing.T) {
	o := vfDefaults()
	o.evidence += "\n\n**VERIFY: FAIL** — fix, then record `**VERIFY: PASS**`."
	vfRefuses(t, o, "verdict mismatch")
}

func TestVflipShaMismatch(t *testing.T) {
	vfRefuses(t, vfDefaults(), "sha mismatch", "--sha", "deadbeef0000")
}

func TestVflipRowShaDisagree(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, strings.Replace(vfRow2, vfSHA, "0123456789ab", 1)) + "\n\n" + vfPass
	vfRefuses(t, o, "sha mismatch")
}

func TestVflipRunnerMismatch(t *testing.T) {
	vfRefuses(t, vfDefaults(), "runner mismatch", "--runner", "assay-worker-app[bot]")
}

func TestVflipRowRunnerDisagree(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, strings.Replace(vfRow2, vfRunner, "human:alex", 1)) + "\n\n" + vfPass
	vfRefuses(t, o, "runner mismatch")
}

func TestVflipRunnerNoSha(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(strings.Replace(vfRow1, " @ "+vfSHA, "", 1),
		strings.Replace(vfRow2, " @ "+vfSHA, "", 1)) + "\n\n" + vfPass
	vfRefuses(t, o, "sha mismatch")
}

func TestVflipGateHuman(t *testing.T) {
	o := vfDefaults()
	o.gate = "human"
	vfRefuses(t, o, "gate is human")
}

func TestVflipRiskYes(t *testing.T) {
	o := vfDefaults()
	o.risk = "{regulatory: no, customer: yes, irreversible: no, sensitive-data: no}"
	vfRefuses(t, o, "risk.customer")
}

func TestVflipIrreversible(t *testing.T) {
	o := vfDefaults()
	o.risk = "{regulatory: no, customer: no, irreversible: yes, sensitive-data: no}"
	vfRefuses(t, o, "irreversible")
}

func TestVflipNotImplemented(t *testing.T) {
	o := vfDefaults()
	o.status, o.verified = "verified", vfStamp
	vfRefuses(t, o, "not implemented")
}

func TestVflipStampPresent(t *testing.T) {
	o := vfDefaults()
	o.verified = vfStamp
	vfRefuses(t, o, "never overwrites a stamp")
}

func TestVflipUnwitnessed(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, strings.Replace(vfRow2, "pass exit=1", "fail exit=0", 1)) + "\n\n" + vfPass
	vfRefuses(t, o, "passing execution witness")
}

func TestVflipHeldRow(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2) + "\n\nRow 2 is HELD pending a rerun.\n\n" + vfPass
	vfRefuses(t, o, "HELD")
}

// TestVflipDiffGuard: the post-condition refuses a rewrite that touches the
// Reviewed cell, or any cell beyond the header's Status and an empty Verified.
func TestVflipDiffGuard(t *testing.T) {
	hdr := "x\n| # | Brief | Status | Verified | Reviewed |\n|---|---|---|---|---|\n"
	before := hdr + "| 01 | t | implemented | — | — |\ny"
	good := hdr + "| 01 | t | verified | " + vfStamp + " | — |\ny"
	if err := flipRowDiffCheck(before, good, "01"); err != nil {
		t.Fatalf("a Status+Verified rewrite must pass: %v", err)
	}
	for name, after := range map[string]string{
		"reviewed":       hdr + "| 01 | t | verified | " + vfStamp + " | " + vfStamp + " |\ny",
		"empty-reviewed": hdr + "| 01 | t | verified | — | " + vfStamp + " |\ny",
		"status-only":    hdr + "| 01 | t | verified | — | — |\ny",
		"title":          hdr + "| 01 | u | verified | " + vfStamp + " | — |\ny",
		"two-line":       strings.Replace(good, "x\n", "z\n", 1),
	} {
		if err := flipRowDiffCheck(before, after, "01"); err == nil {
			t.Errorf("%s: a rewrite beyond Status and Verified must be refused", name)
		}
	}
	// No header row above the row: the columns cannot be resolved, so refuse.
	bare := "x\n| 01 | t | implemented | — | — |\ny"
	if err := flipRowDiffCheck(bare, "x\n| 01 | t | verified | "+vfStamp+" | — |\ny", "01"); err == nil {
		t.Error("no-header: a row whose columns cannot be resolved must be refused")
	}
}

// TestVflipWitnessOldRun (F1): rows 1+2 at sha X, BLOCKED, then a run at the
// stamped sha that records row 1 only, then PASS. Row 2 was never run at the
// stamped sha, so the flip refuses — witnesses come from the PASS run alone.
func TestVflipWitnessOldRun(t *testing.T) {
	o := vfDefaults()
	old := strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, "0123456789ab")
	o.evidence = old + "\n\nVERIFY: BLOCKED. row 2 did not execute.\n\n" +
		witnessTableFor(vfRow1) + "\n\n" + vfPass
	vfRefuses(t, o, "passing execution witness")
}

// TestVflipRowsAfterPass (A1): a verdict-first layout puts a run's rows BELOW
// its verdict; Date/Runner rows after the latest PASS refuse rather than let
// the PASS take the earlier (FAIL) run's rows.
func TestVflipRowsAfterPass(t *testing.T) {
	o := vfDefaults()
	runX := strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, "0123456789ab")
	o.evidence = "**VERIFY: FAIL** — run at X.\n\n" + runX + "\n\n" + vfPass + "\n\n" +
		witnessTableFor(vfRow1, vfRow2)
	vfRefuses(t, o, "follow the latest PASS", "--sha", "0123456789ab")
}

// TestVflipDiffWired (ADV-6): planVerifyFlip itself runs the diff guard — a
// rewrite that also writes the Reviewed cell is refused before any write.
func TestVflipDiffWired(t *testing.T) {
	orig := flipRewriteFn
	t.Cleanup(func() { flipRewriteFn = orig })
	flipRewriteFn = func(raw, num, stamp string) (string, error) {
		out, err := orig(raw, num, stamp)
		return strings.Replace(out, "| "+vfReview+" |", "| "+stamp+" |", 1), err
	}
	root, readme := vfFixture(t, vfDefaults())
	before, _ := os.ReadFile(readme)
	code, out, errOut := runVF(t, root)
	if code == verifyflipExitOK || !strings.Contains(errOut, "neither Status") {
		t.Fatalf("exit = %d, want the diff guard's refusal; out=%q err=%q", code, out, errOut)
	}
	if after, _ := os.ReadFile(readme); !bytes.Equal(before, after) {
		t.Fatalf("a bad rewrite reached the README:\n%s", after)
	}
}

// vfDated swaps every row's Date cell for date.
func vfDated(date string) vfOpts {
	o := vfDefaults()
	o.evidence = strings.ReplaceAll(o.evidence, "| 2026-08-13 |", "| "+date+" |")
	return o
}

// TestVflipDateNotIso (ADV-5): the Date cell is copied into the stamp, so it
// must be a real YYYY-MM-DD date.
func TestVflipDateNotIso(t *testing.T) {
	for _, d := range []string{"2026-8-13", "13 Aug 2026", "2026-02-30", "soon"} {
		t.Run(d, func(t *testing.T) { vfRefuses(t, vfDated(d), "bad date") })
	}
}

// TestVflipDateFuture (ADV-5): a run dated after today refuses.
func TestVflipDateFuture(t *testing.T) {
	vfRefuses(t, vfDated("2999-01-01"), "bad date")
}

// TestVflipAmbiguousFile (ADV-4): two brief-01 files are a resolution failure
// (could-not-check), as `statusgen brief --check-verified` reports it — never
// a silent pick of the first file.
func TestVflipAmbiguousFile(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	dir := filepath.Dir(readme)
	src, err := os.ReadFile(filepath.Join(dir, "brief-01-flip.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-aa.md"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	vfCouldNotCheck(t, root, readme, "vf/01", "multiple brief files")
}

// TestVflipStreamNameKey (ADV-4): a key naming the stream by its frontmatter
// name rather than its directory resolves nowhere under --check-verified, so
// the flip cannot check it either.
func TestVflipStreamNameKey(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	raw, _ := os.ReadFile(readme)
	if err := os.WriteFile(readme, []byte(strings.Replace(string(raw), "stream: vf\n", "stream: other\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCouldNotCheck(t, root, readme, "other/01", "no brief file")
}

// TestVflipStreamDirClash (ADV-4): a board in another directory that names
// itself `vf` must not have its README flipped from the vf/ brief's record.
func TestVflipStreamDirClash(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	other := filepath.Join(root, "docs", "streams", "aa")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "brief-01-flip.md"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(readme), name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(other, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	otherReadme := filepath.Join(other, "README.md")
	vfCouldNotCheck(t, root, otherReadme, "vf/01", "outside the board")
}

// vfCouldNotCheck asserts exit 2 for key, a stderr naming want, and an
// untouched README.
func vfCouldNotCheck(t *testing.T, root, readme, key, want string) {
	t.Helper()
	before, _ := os.ReadFile(readme)
	var out, errb bytes.Buffer
	code := runVerifyflip([]string{"--root", root, "--brief", key, "--sha", vfSHA, "--runner", vfRunner}, &out, &errb)
	if code != verifyflipExitCouldNotCheck || !strings.Contains(errb.String(), want) {
		t.Fatalf("exit = %d, want %d naming %q; out=%q err=%q", code, verifyflipExitCouldNotCheck, want, out.String(), errb.String())
	}
	if after, _ := os.ReadFile(readme); !bytes.Equal(before, after) {
		t.Fatalf("a could-not-check wrote the README:\n%s", after)
	}
}

// TestVflipSymlinkReadme (ADV-7): a README that is a symlink is refused, and
// the link target is never written.
func TestVflipSymlinkReadme(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	target := filepath.Join(t.TempDir(), "target.md")
	raw, _ := os.ReadFile(readme)
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(readme); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, readme); err != nil {
		t.Skipf("symlink unsupported here: %v", err)
	}
	code, out, errOut := runVF(t, root)
	if code != verifyflipExitRefused || !strings.Contains(errOut, "not a regular file") {
		t.Fatalf("exit = %d, want %d naming a non-regular README; out=%q err=%q", code, verifyflipExitRefused, out, errOut)
	}
	if after, _ := os.ReadFile(target); !bytes.Equal(raw, after) {
		t.Fatalf("the flip wrote through the symlink:\n%s", after)
	}
}

// TestVflipStruckBoundary: a struck (retracted) BLOCKED verdict is not the
// latest verdict, but it still ends the earlier run — that run's rows at
// another sha are not read as the PASS's rows.
func TestVflipStruckBoundary(t *testing.T) {
	o := vfDefaults()
	old := strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, "0123456789ab")
	o.evidence = old + "\n\n~~VERIFY: BLOCKED. row 2 did not execute.~~\n\n" + o.evidence
	root, _ := vfFixture(t, o)
	if code, out, errOut := runVF(t, root, "--dry-run"); code != verifyflipExitOK {
		t.Fatalf("exit = %d, want 0; out=%q err=%q", code, out, errOut)
	}
}

// TestVflipStruckPassOnly: a struck strict PASS is a retraction, not a verdict.
func TestVflipStruckPassOnly(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2) + "\n\n~~" + vfPass + "~~"
	vfRefuses(t, o, "verdict mismatch")
}

// vfQualified swaps every row's runner for runner + quals.
func vfQualified(quals string) vfOpts {
	o := vfDefaults()
	o.evidence = strings.ReplaceAll(o.evidence, vfRunner+" @ "+vfSHA+" |", vfRunner+" @ "+vfSHA+quals+" |")
	return o
}

// TestVflipOnBehalfDropped: the Evidence row's `on-behalf-of human:<name>`
// qualifier stays out of the Verified stamp; the other qualifiers are kept.
func TestVflipOnBehalfDropped(t *testing.T) {
	root, readme := vfFixture(t, vfQualified(" (claude-x) (on-behalf-of human:pat) (forge-identity)"))
	code, out, errOut := runVF(t, root)
	want := vfStamp + " (claude-x) (forge-identity)"
	if code != verifyflipExitOK || !strings.Contains(out, "verified-stamp: "+want+"\n") {
		t.Fatalf("exit = %d out=%q err=%q, want stamp %q", code, out, errOut, want)
	}
	after, _ := os.ReadFile(readme)
	if !strings.Contains(string(after), "| verified | "+want+" | "+vfReview+" |") {
		t.Fatalf("README after flip:\n%s", after)
	}
}

// TestVflipHumanTokenRefused: a `human:` token outside a clean qualifier
// list cannot be dropped, so the flip refuses rather than write it.
func TestVflipHumanTokenRefused(t *testing.T) {
	vfRefuses(t, vfQualified(" by human:pat"), "human-stamp")
}

// ---------------------------------------------------------------------------
// flip-verdict-provenance: git, not the Runner text, says who wrote the PASS.
// ---------------------------------------------------------------------------

// vfExpect runs the verb on root and wants exit code, stderr naming want, and
// — unless the verb succeeded — an untouched README.
func vfExpect(t *testing.T, root, readme string, code int, want string) {
	t.Helper()
	before, _ := os.ReadFile(readme)
	got, out, errOut := runVF(t, root)
	if got != code || !strings.Contains(errOut+out, want) {
		t.Fatalf("exit = %d, want %d naming %q; out=%q err=%q", got, code, want, out, errOut)
	}
	if after, _ := os.ReadFile(readme); code != verifyflipExitOK && !bytes.Equal(before, after) {
		t.Fatalf("a refusal wrote the README:\n%s", after)
	}
}

// TestVflipWorkerPassRun: the verifier recorded a FAIL; another App appended
// a PASS run whose Runner text names the verifier. The rows read right, so
// only blame tells them apart.
func TestVflipWorkerPassRun(t *testing.T) {
	o := vfDefaults()
	o.evidence = strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, "0123456789ab") +
		"\n\n**VERIFY: FAIL** — row 2 failed at 0123456789ab."
	root, readme := vfFixture(t, o)
	vfAppend(t, root, "\n\n"+witnessTableFor(vfRow1, vfRow2)+"\n\n"+vfPass)
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipWorkerMarker: the verifier's rows, another App's PASS marker.
func TestVflipWorkerMarker(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2)
	root, readme := vfFixture(t, o)
	vfAppend(t, root, "\n\n"+vfPass)
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipWorkerRow: the verifier's PASS, one row rewritten by another App.
func TestVflipWorkerRow(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	raw, _ := os.ReadFile(vfBrief(root))
	edited := strings.Replace(string(raw), "sha256:6f1a0b3c9d22", "sha256:6f1a0b3c9d23", 1)
	if err := os.WriteFile(vfBrief(root), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipFoldedPassLine: a comment folds two file lines into the PASS line;
// the second one is another App's, so it is blamed too.
func TestVflipFoldedPassLine(t *testing.T) {
	o := vfDefaults()
	o.evidence += " <!-- note\nend -->"
	root, readme := vfFixture(t, o)
	raw, _ := os.ReadFile(vfBrief(root))
	edited := strings.Replace(string(raw), "\nend -->", "\nend --> and row 3 too", 1)
	if err := os.WriteFile(vfBrief(root), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipUncommittedPass: a PASS nobody committed is owned by nobody.
func TestVflipUncommittedPass(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2)
	root, readme := vfFixture(t, o)
	vfAppend(t, root, "\n\n"+vfPass)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipHumanOwnedPass: a roster human may verify.
func TestVflipHumanOwnedPass(t *testing.T) {
	o := vfDefaults()
	o.author = vfMailHuman
	root, readme := vfFixture(t, o)
	vfExpect(t, root, readme, verifyflipExitOK, "flipped")
}

// TestVflipNoRosterCannotCheck: no roster, no accepted actor — could-not-check.
func TestVflipNoRosterCannotCheck(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	scanWithNoRoster(t)
	vfExpect(t, root, readme, verifyflipExitCouldNotCheck, "provenance")
}

// TestVflipShallowCannotCheck: blame in a shallow clone cannot reach the
// commits behind the lines — could-not-check, never a pass.
func TestVflipShallowCannotCheck(t *testing.T) {
	root, _ := vfFixture(t, vfDefaults())
	if err := os.WriteFile(filepath.Join(root, "later.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailVerifier)
	clone := filepath.Join(t.TempDir(), "c")
	vfGit(t, root, vfMailVerifier, "clone", "-q", "--depth", "1", "file://"+root, clone)
	vfExpect(t, clone, filepath.Join(clone, "docs", "streams", "vf", "README.md"),
		verifyflipExitCouldNotCheck, "shallow")
}

// TestVflipNotARepo: outside git there is no provenance — could-not-check.
func TestVflipNotARepo(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	vfExpect(t, root, readme, verifyflipExitCouldNotCheck, "provenance")
}

// TestVflipMarkerTwoComments: the verifier's rows, another App's PASS marker
// carrying two trailing comments. The marker line is content; its author must
// be judged, however many comments follow the text on the same line.
func TestVflipMarkerTwoComments(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2)
	root, readme := vfFixture(t, o)
	vfAppend(t, root, "\n\n"+vfPass+" <!-- first --> <!-- second -->")
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// vfBlameEntry is one --line-porcelain record.
func vfBlameEntry(n int, name, email, content string) string {
	s := fmt.Sprintf("aaaa %d %d 1\n", n, n)
	if name != "" || email != "" {
		s += "author " + name + "\nauthor-mail <" + email + ">\n"
	}
	return s + "\t" + content + "\n"
}

// TestVflipBlameCoverage pins that the provenance judgement accounts for every
// PASS line it asked about: a missing record or a record with no author is
// could-not-check, never a pass on the authors that remain.
func TestVflipBlameCoverage(t *testing.T) {
	p := evidenceActorPolicyFromRoster()
	if p.Unavailable != "" {
		t.Fatalf("fixture roster unavailable: %s", p.Unavailable)
	}
	v := vfBlameEntry(1, fixtureVerifierName, fixtureVerifierEmail, vfPass)
	w := vfBlameEntry(2, fixtureWorkerName, fixtureWorkerEmail, vfRow1)
	cases := []struct {
		name, out string
		want      int
		refused   bool // false: could-not-check (a plain error)
	}{
		{"one record for two lines", v, 2, false},
		{"no records", "", 1, false},
		{"record with no author", v + vfBlameEntry(2, "", "", vfRow1), 2, false},
		{"worker line", v + w, 2, true},
		{"worker comment-only line", v + vfBlameEntry(2, fixtureWorkerName, fixtureWorkerEmail, "<!-- x -->"), 2, true},
	}
	for _, c := range cases {
		err := flipJudgeBlame(p, c.out, c.want, "brief.md")
		var r *flipRefusal
		switch {
		case err == nil:
			t.Errorf("%s: judged clean, want a refusal or could-not-check", c.name)
		case c.refused != errors.As(err, &r):
			t.Errorf("%s: err = %v (refusal=%v), want refusal=%v", c.name, err, errors.As(err, &r), c.refused)
		}
	}
	if err := flipJudgeBlame(p, v+vfBlameEntry(2, fixtureVerifierName, fixtureVerifierEmail, vfRow1), 2, "brief.md"); err != nil {
		t.Fatalf("every line verifier-owned: err = %v, want nil", err)
	}
}

// vfBlameCallers returns every function in src that calls name.
func vfBlameCallers(t *testing.T, fset *token.FileSet, file string, src any, name string) []string {
	t.Helper()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == name {
					out = append(out, fn.Name.Name)
				}
			}
			return true
		})
	}
	return out
}

// TestVflipBlameCallers is the class guard for flip-verdict-provenance: the
// content-filtered, deduplicated author set may answer only the Evidence-actor
// lint's any-line question. A caller that turns blame into authority must
// count per-line records (blamePorcelainLines), so any new caller of
// blamePorcelainAuthors outside the allow-list fails here. A planted caller
// is the positive control: a matcher that stopped matching would fail it.
func TestVflipBlameCallers(t *testing.T) {
	allowed := map[string]bool{"blamePorcelainAuthors": true, "blameEvidenceAuthors": true}
	fset := token.NewFileSet()
	plant := "package main\nfunc plantedJudge(out string) { blamePorcelainAuthors(out) }\n"
	if got := vfBlameCallers(t, fset, "plant.go", plant, "blamePorcelainAuthors"); len(got) != 1 || got[0] != "plantedJudge" {
		t.Fatalf("positive control: callers = %v, want [plantedJudge]", got)
	}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no package sources found (err=%v)", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		for _, caller := range vfBlameCallers(t, fset, f, nil, "blamePorcelainAuthors") {
			if !allowed[caller] {
				t.Errorf("%s: %s calls blamePorcelainAuthors — a filtered author set cannot establish per-line provenance; use blamePorcelainLines and count the records", f, caller)
			}
		}
	}
}

// TestVflipLintTwoComments is the lower-layer pair for the shared parser fix:
// the Evidence-actor lint reads a line with text before two comments as
// content, so a verifier who owns only such a line still backs the closure.
func TestVflipLintTwoComments(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2)
	o.author = vfMailWorker
	root, readme := vfFixture(t, o)
	vfAppend(t, root, "\n\nverifier note <!-- a --> <!-- b -->")
	raw, _ := os.ReadFile(readme)
	flipped := strings.Replace(string(raw), "| implemented | — |", "| verified | "+vfStamp+" |", 1)
	if err := os.WriteFile(readme, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailVerifier)
	withBaseClosures(t, map[string]bool{}, true)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if problems, _ := evidenceActorGate(root, streams); len(problems) != 0 {
		t.Fatalf("lint problems = %v, want none: the verifier's line before two comments is content", problems)
	}
}

// TestVflipStrippedSpans pins the stripped-line → file-line map.
func TestVflipStrippedSpans(t *testing.T) {
	spans, ok := flipStrippedSpans("a\nb <!-- x\ny -->c\nd <!-- z -->\ne")
	want := [][2]int{{0, 0}, {1, 2}, {3, 3}, {4, 4}}
	if !ok || len(spans) != len(want) {
		t.Fatalf("spans = %v ok=%v, want %v", spans, ok, want)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Fatalf("spans = %v, want %v", spans, want)
		}
	}
	if _, ok := flipStrippedSpans("a <!-- open"); ok {
		t.Fatal("an unterminated comment must not map")
	}
}

// TestVflipLintLowerLayer is the adversarial pair for provenance. The verb
// refuses an Evidence section no verifier wrote; with the verb bypassed (the
// row flipped by hand), the Evidence-actor lint the deskevidence flip runs
// after its commit still names the closure.
func TestVflipLintLowerLayer(t *testing.T) {
	o := vfDefaults()
	o.author = vfMailWorker
	root, readme := vfFixture(t, o)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")

	raw, _ := os.ReadFile(readme)
	flipped := strings.Replace(string(raw), "| implemented | — |", "| verified | "+vfStamp+" |", 1)
	if err := os.WriteFile(readme, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	withBaseClosures(t, map[string]bool{}, true)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := evidenceActorGate(root, streams)
	if len(problems) != 1 || !strings.Contains(problems[0], "vf/01") {
		t.Fatalf("lint problems = %v, want one naming vf/01", problems)
	}
}

// ---------------------------------------------------------------------------
// flip-gate-class-coverage: a gate:human Verify row is a person's call.
// ---------------------------------------------------------------------------

func vfClassed(c1, c2 string) string {
	return "| # | Class | Command | Expect |\n|---|---|---|---|\n" +
		"| 1 | " + c1 + " | `true` | exit 0 |\n| 2 | " + c2 + " | `false` | exit 1 |\n"
}

func TestVflipGateHumanRow(t *testing.T) {
	o := vfDefaults()
	o.verify = vfClassed("check", "gate:human")
	vfRefuses(t, o, "gate-class: Verify row 2 is classed gate:human")
}

func TestVflipGateHumanCompound(t *testing.T) {
	o := vfDefaults()
	o.verify = vfClassed("`Gate:Human` +flow", "check")
	vfRefuses(t, o, "gate-class: Verify row 1 is classed gate:human")
}

func TestVflipUnknownRowClass(t *testing.T) {
	o := vfDefaults()
	o.verify = vfClassed("check", "gate:humn")
	vfRefuses(t, o, "unknown class")
}

func TestVflipClassedRowsFlip(t *testing.T) {
	o := vfDefaults()
	o.verify = vfClassed("check:ci", "check")
	root, readme := vfFixture(t, o)
	vfExpect(t, root, readme, verifyflipExitOK, "flipped")
}

// TestVflipCheckVerifiedLayer is the adversarial pair for the row class. The
// verb refuses; with the verb bypassed (the row flipped by hand to a model
// stamp), `statusgen brief --check-verified` — the closure check the
// deskevidence flip runs after the write — refuses the same tree, and
// accepts it once the Verified cell names a human.
func TestVflipCheckVerifiedLayer(t *testing.T) {
	o := vfDefaults()
	o.verify = vfClassed("check", "gate:human")
	root, readme := vfFixture(t, o)
	vfExpect(t, root, readme, verifyflipExitRefused, "gate-class")

	for _, tc := range []struct {
		cell string
		code int
	}{{vfStamp, briefInfoExitInvalid}, {"2026-08-13 human:alex", briefInfoExitOK}} {
		raw, _ := os.ReadFile(readme)
		row := strings.Index(string(raw), "| 01 |")
		flipped := string(raw)[:row] + "| 01 | [t](./brief-01-flip.md) | 0 | S | verified | " + tc.cell + " | " + vfReview + " |\n"
		if err := os.WriteFile(readme, []byte(flipped), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		code := runBriefInfo([]string{"--root", root, "--check-verified", "vf/01"}, &out, &errb)
		if code != tc.code || (code != briefInfoExitOK && !strings.Contains(errb.String(), "gate:human")) {
			t.Fatalf("Verified %q: check-verified exit = %d, want %d; err=%q", tc.cell, code, tc.code, errb.String())
		}
	}
}

// ---------------------------------------------------------------------------
// flip-verdict-provenance, round 3: every line the verb reads, and every
// change made after the PASS was recorded.
// ---------------------------------------------------------------------------

// vfEdit replaces the first old with new in the fixture brief.
func vfEdit(t *testing.T, root, old, new string) {
	t.Helper()
	raw, err := os.ReadFile(vfBrief(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), old) {
		t.Fatalf("fixture brief does not contain %q", old)
	}
	if err := os.WriteFile(vfBrief(root), []byte(strings.Replace(string(raw), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// vfNoDateHdr is a witness table whose header names neither Date nor Runner:
// the witness reader still takes its rows (last two cells), the Date/Runner
// reader does not.
const vfNoDateHdr = "| # | Command | Result | Output | When | Who |\n|---|---|---|---|---|---|\n"

// TestVflipRunRowNoDateHdr: a witness row in a table without Date/Runner
// columns, committed by another App inside the PASS run BEFORE the verifier
// recorded the PASS. Only blame of the whole run sees it: no commit follows
// the PASS, and the marker and the Date/Runner row are the verifier's.
func TestVflipRunRowNoDateHdr(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1)
	root, readme := vfFixture(t, o)
	vfAppend(t, root, "\n\n"+vfNoDateHdr+vfRow2)
	vfCommit(t, root, vfMailWorker)
	vfAppend(t, root, "\n\n"+vfPass)
	vfCommit(t, root, vfMailVerifier)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipRunRowAfterPass is the reviewer's shape: the same table inserted
// above the verifier's PASS by a later commit of another App.
func TestVflipRunRowAfterPass(t *testing.T) {
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1) + "\n\n" + vfPass
	root, readme := vfFixture(t, o)
	if code, _, _ := runVF(t, root, "--dry-run"); code != verifyflipExitRefused {
		t.Fatalf("baseline: exit = %d, want the unwitnessed row 2 refused", code)
	}
	vfEdit(t, root, "\n"+vfPass, "\n"+vfNoDateHdr+vfRow2+"\n\n"+vfPass)
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// vfLaterFail is a verifier FAIL recorded after the verifier's PASS.
const vfLaterFail = "**VERIFY: FAIL** — row 2 regressed on re-check at " + vfSHA + "."

// vfFailAfterPass commits the verifier's PASS, then the verifier's later
// FAIL, and returns the fixture.
func vfFailAfterPass(t *testing.T) (root, readme string) {
	t.Helper()
	root, readme = vfFixture(t, vfDefaults())
	vfAppend(t, root, "\n\n"+vfLaterFail)
	vfCommit(t, root, vfMailVerifier)
	if code, _, errOut := runVF(t, root, "--dry-run"); code != verifyflipExitRefused || !strings.Contains(errOut, "verdict mismatch") {
		t.Fatalf("control: exit = %d err=%q, want the later FAIL refused", code, errOut)
	}
	return root, readme
}

// TestVflipLaterFailEdited: another App neutralises the verifier's later FAIL
// — deleting it, striking it, quoting it — so the earlier PASS reads as the
// latest verdict again. Each refuses.
func TestVflipLaterFailEdited(t *testing.T) {
	for _, c := range []struct{ name, repl string }{
		{"deleted", ""},
		{"struck", "~~" + vfLaterFail + "~~"},
		{"blockquoted", "> " + vfLaterFail},
		{"blanked", " "},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, readme := vfFailAfterPass(t)
			vfEdit(t, root, "\n\n"+vfLaterFail, "\n\n"+c.repl)
			vfCommit(t, root, vfMailWorker)
			vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
		})
	}
}

// TestVflipLaterRunDeleted: the verifier recorded a whole later FAIL run (a
// new witness table, then the FAIL); another App deleted the block.
func TestVflipLaterRunDeleted(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	later := "\n\n" + strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, "0123456789ab") +
		"\n\n**VERIFY: FAIL** — row 2 failed at 0123456789ab."
	vfAppend(t, root, later)
	vfCommit(t, root, vfMailVerifier)
	vfEdit(t, root, later, "")
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipUncommittedDelete: a deletion nobody committed has no blame record
// and no commit; the working tree's Evidence must match HEAD's.
func TestVflipUncommittedDelete(t *testing.T) {
	root, readme := vfFailAfterPass(t)
	vfEdit(t, root, "\n\n"+vfLaterFail, "")
	vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
}

// TestVflipEveryRunLine is the class guard for flip-verdict-provenance: every
// line of the PASS run, re-authored by another App before the verifier
// recorded the PASS, refuses. The edit adds one trailing space, which no
// reader sees, so provenance is the only thing that can refuse it. The run's
// lines are enumerated from the fixture, so a new line the verb reads is
// covered without editing this list.
func TestVflipEveryRunLine(t *testing.T) {
	run := strings.Split(witnessTableFor(vfRow1, vfRow2), "\n")
	for i, line := range run {
		t.Run(fmt.Sprintf("line%d", i), func(t *testing.T) {
			o := vfDefaults()
			o.evidence = witnessTableFor(vfRow1, vfRow2)
			root, readme := vfFixture(t, o)
			vfEdit(t, root, "\n"+line+"\n", "\n"+line+" \n")
			vfCommit(t, root, vfMailWorker)
			vfAppend(t, root, "\n\n"+vfPass)
			vfCommit(t, root, vfMailVerifier)
			vfExpect(t, root, readme, verifyflipExitRefused, "provenance")
		})
	}
	// Positive control: the same history with the verifier making the edit
	// flips, so the refusals above are about who edited, not what.
	o := vfDefaults()
	o.evidence = witnessTableFor(vfRow1, vfRow2)
	root, readme := vfFixture(t, o)
	vfEdit(t, root, "\n"+run[0]+"\n", "\n"+run[0]+" \n")
	vfCommit(t, root, vfMailVerifier)
	vfAppend(t, root, "\n\n"+vfPass)
	vfCommit(t, root, vfMailVerifier)
	vfExpect(t, root, readme, verifyflipExitOK, "flipped")
}

// TestVflipMergeNotJudged: a merge commit another App made, whose Evidence is
// one parent's verbatim, changed nothing in Evidence and is not judged; the
// verifier's own post-PASS note passes. The template comment the brief's
// author wrote before the run is not content and is not judged either.
func TestVflipMergeNotJudged(t *testing.T) {
	o := vfDefaults()
	o.evidence = "<!-- appended at implementation time -->"
	o.author = vfMailWorker
	root, readme := vfFixture(t, o)
	runGitEnv(t, root, nil, "checkout", "-q", "-b", "side")
	vfAppend(t, root, "\n\n"+witnessTableFor(vfRow1, vfRow2)+"\n\n"+vfPass)
	vfCommit(t, root, vfMailVerifier)
	runGitEnv(t, root, nil, "checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	vfGit(t, root, vfMailWorker, "merge", "-q", "--no-ff", "--no-edit", "side")
	vfAppend(t, root, "\n\nverifier note: re-read at "+vfSHA+", nothing changed.")
	vfCommit(t, root, vfMailVerifier)
	vfExpect(t, root, readme, verifyflipExitOK, "flipped")
}

// TestBlameLinesCommit pins the commit id the porcelain reader records per
// line: the history layer walks from the PASS marker's commit, so a reader that
// dropped or misread it would walk from the wrong place. A content line that
// happens to look like a header (it is TAB-prefixed in porcelain) must not
// overwrite it.
func TestBlameLinesCommit(t *testing.T) {
	a := strings.Repeat("a", 40)
	b := strings.Repeat("b", 64)
	out := a + " 3 3 1\nauthor V\nauthor-mail <v@x>\nfilename f\n\t" + strings.Repeat("c", 40) + " 1 1\n" +
		b + " 4 4\nauthor W\nauthor-mail <w@x>\nfilename f\n\tsecond\n"
	got := blamePorcelainLines(out)
	if len(got) != 2 || got[0].Commit != a || got[1].Commit != b {
		t.Fatalf("commits = %+v, want %s then %s", got, a[:8], b[:8])
	}
	if got[0].Content != strings.Repeat("c", 40)+" 1 1" {
		t.Fatalf("content = %q", got[0].Content)
	}
}
