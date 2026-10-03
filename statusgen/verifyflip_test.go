package main

import (
	"bytes"
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
