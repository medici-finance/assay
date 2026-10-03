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
)

type vfOpts struct {
	gate, risk, status, verified, evidence string
}

func vfDefaults() vfOpts {
	return vfOpts{gate: "model", risk: vfRisk, status: "implemented", verified: "—",
		evidence: witnessTableFor(vfRow1, vfRow2) + "\n\n" + vfPass}
}

// vfFixture lays a one-brief stream on disk and returns the root and the
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
		"## Verify\n| # | Command | Expect |\n|---|---------|--------|\n" +
		"| 1 | `true` | exit 0 |\n| 2 | `false` | exit 1 |\n\n## Evidence\n" + o.evidence +
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
	return root, readme
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
// Reviewed cell, or any cell beyond Status and an empty Verified.
func TestVflipDiffGuard(t *testing.T) {
	before := "x\n| 01 | t | implemented | — | — |\ny"
	good := "x\n| 01 | t | verified | " + vfStamp + " | — |\ny"
	if err := flipRowDiffCheck(before, good, "01"); err != nil {
		t.Fatalf("a Status+Verified rewrite must pass: %v", err)
	}
	for name, after := range map[string]string{
		"reviewed": "x\n| 01 | t | verified | " + vfStamp + " | " + vfStamp + " |\ny",
		"title":    "x\n| 01 | u | verified | " + vfStamp + " | — |\ny",
		"two-line": "z\n| 01 | t | verified | " + vfStamp + " | — |\ny",
	} {
		if err := flipRowDiffCheck(before, after, "01"); err == nil {
			t.Errorf("%s: a rewrite beyond Status and Verified must be refused", name)
		}
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
