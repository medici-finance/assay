package main

// verifierclaimrepo_test.go — FAIL-FIRST coverage for #5999.
//
// THE DEFECT. A cross-repository tracking brief (the brief in tracking repo T, its deliverable in
// D) had no verifier-kit dispatch shape that both claimed and landed Evidence in T: --root at T
// hard-failed, and the --repo D shape the refusal hinted at took and released its claim in D's
// namespace while the brief, its Evidence and its outcome record live in T. The refusal named no
// --repo at all, and the kit called the tracking side read-only with "implement to its contract"
// while also telling the verifier to edit the brief's Evidence.
//
// WHAT THESE PIN. One invocation — --repo D --root <D> --claim-root <T> --brief <T brief> —
// claims, releases and lands in T; the refusal names that --repo; the kit's Specification line
// is a verifier's, never an implementer's.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func crossVerifierFixture(t *testing.T) (source, tracking, brief string) {
	t.Helper()
	crossDispatchConfig(t, "public")
	source = crossDispatchRepo(t, "example-org/source", map[string]string{"source.txt": "deliverable\n"})
	const rel = "docs/streams/sample/brief-01-check.md"
	body := "---\nhomed-in: example-org/source\n---\n# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n\n## Evidence\n"
	registry := "schema: graph-repos-v1\nself: track\nrepos:\n  track:\n    repo: example-org/tracking\n  source:\n    repo: example-org/source\n"
	tracking = crossDispatchRepo(t, "example-org/tracking", map[string]string{rel: body, "docs/streams/graph-repos.yaml": registry})
	t.Setenv("DESK_LOOP", "verify-desk")
	isolateClaimTool(t, t.TempDir())
	plantScripts(t, tracking)
	return source, tracking, filepath.Join(tracking, rel)
}

// regression: #5999
func TestCrossVerifierClaimsTracking(t *testing.T) {
	source, tracking, brief := crossVerifierFixture(t)
	o := dispatchOpts{root: source, claimRoot: tracking, repo: "example-org/source", brief: brief,
		model: "gpt-6-astra", tier: "strong", kit: "verifier", item: "sample/01", dryRun: true,
		promptFile: filepath.Join(t.TempDir(), "preview.md")}
	var err error
	out := captureStdout(t, func() { err = dispatch(o) })
	if err != nil {
		t.Fatalf("the documented cross-repository verifier shape must dispatch: %v", err)
	}
	if !strings.Contains(out, "repo=example-org/source claim-repo=example-org/tracking ") {
		t.Fatalf("dry-run PLAN does not name the tracking claim namespace:\n%s", out)
	}
	raw, rerr := os.ReadFile(emittedPathFor(o.promptFile))
	if rerr != nil {
		t.Fatal(rerr)
	}
	prompt := string(raw)
	// Every claim-tool command the verifier is handed addresses the TRACKING namespace.
	claimLines := 0
	for _, line := range strings.Split(prompt, "\n") {
		if !strings.Contains(line, claimScriptRel) && !strings.Contains(line, goClaimBinary+" ") {
			continue
		}
		if !strings.Contains(line, " release ") && !strings.Contains(line, " progress ") {
			continue
		}
		claimLines++
		if !strings.Contains(line, "--repo example-org/tracking") || strings.Contains(line, "--repo example-org/source") {
			t.Fatalf("claim command addresses the deliverable, not the tracking repo: %s", line)
		}
	}
	if claimLines == 0 {
		t.Fatalf("no claim release command in the verifier prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "deskevidence example-org/tracking main --outcome-record ") {
		t.Fatal("outcome record does not land in the tracking repo")
	}
	for _, line := range strings.Split(prompt, "\n") {
		if strings.HasPrefix(line, "- **Specification:**") && strings.Contains(line, "implement to its contract") {
			t.Fatalf("verifier Specification line carries the implementer wording: %s", line)
		}
	}
	if !strings.Contains(prompt, "isolated tracking clone") {
		t.Fatal("verifier prompt does not name the isolated tracking clone as the Evidence tree")
	}
}

// regression: #5999
func TestCrossMismatchNamesRepo(t *testing.T) {
	source, tracking, brief := crossVerifierFixture(t)
	_ = source
	for _, root := range []string{tracking, source} {
		o := dispatchOpts{root: root, claimRoot: tracking, repo: "example-org/tracking", brief: brief,
			model: "gpt-6-astra", tier: "strong", kit: "verifier", item: "sample/01", dryRun: true,
			promptFile: filepath.Join(t.TempDir(), "preview.md")}
		err := dispatch(o)
		if !deskkit.IsRefused(err) {
			t.Fatalf("--repo at the tracking repo must still hard-fail: %v", err)
		}
		if !strings.Contains(err.Error(), "Re-run with --repo example-org/source,") {
			t.Fatalf("refusal remedy does not name the --repo that dispatches: %v", err)
		}
	}
}
