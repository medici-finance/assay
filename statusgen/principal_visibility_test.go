package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The witness-side half of the on-behalf-of visibility split. The desk writers already
// name the roster's NEUTRAL form of the human on any target the roster does not state is
// `:private`; the witness annotation statusgen writes into a brief's `## Evidence` table
// — a file in the repository, as world-readable as a posted review on a public repo —
// named the login whatever the target. These tests pin that it now follows the same split.

// witnessVisibilityRoster's blessing authority has a LOGIN that differs from the neutral
// name the human-login map carries for it, and configures one public and one private repo.
// Both halves are what make the split observable: with name == login (the other fixtures'
// shape) a leaked login and a correct substitution would render identically.
func witnessVisibilityRoster() map[string]string {
	return map[string]string{
		scanEnvBlessLogin:      "a-private-handle:100001",
		scanEnvTrustedLogins:   "a-private-handle:100001",
		scanEnvTrustedBotSlugs: "verifier=assay-verifier-app:300000005",
		scanEnvHumanLoginMap:   "ada:a-private-handle",
		scanEnvAllowedRepos:    "example-org/pub:ci:public,example-org/one:ci:private",
	}
}

const (
	witnessFixtureLogin = "a-private-handle"
	witnessFixtureName  = "ada"
)

func witnessRowFor(repo string) string {
	w := witness{ID: "1", Command: "true", State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-09-23", Runner: "assay-verifier-app[bot]", Tree: "abc1234", Repo: repo}
	return w.row()
}

// TestWitnessOnBehalfOfPublicTargetNamesNeutralName is the defect: a witness row written
// into a public repo named the roster LOGIN. It must name the neutral form instead, and the
// login must appear nowhere in the row. Every target that is not stated private takes the
// neutral form — an unconfigured repo, and an unstated one ("", the brief's stream declares
// no repo) — which is the fail-closed direction the writer side takes too.
func TestWitnessOnBehalfOfPublicTargetNamesNeutralName(t *testing.T) {
	scanWithRoster(t, witnessVisibilityRoster())
	for _, repo := range []string{"example-org/pub", "", "some-org/never-configured"} {
		row := witnessRowFor(repo)
		if strings.Contains(row, witnessFixtureLogin) {
			t.Fatalf("witness row for repo %q = %q — it carries the roster login; want the neutral name %q",
				repo, row, witnessFixtureName)
		}
		if want := "(on-behalf-of human:" + witnessFixtureName + ")"; !strings.Contains(row, want) {
			t.Fatalf("witness row for repo %q = %q, want it to carry %q", repo, row, want)
		}
	}
}

// TestWitnessOnBehalfOfPrivateTargetUnchanged: a repo the roster states is `:private` keeps
// today's behaviour exactly — the login. The split must not re-anonymise the private record.
func TestWitnessOnBehalfOfPrivateTargetUnchanged(t *testing.T) {
	scanWithRoster(t, witnessVisibilityRoster())
	row := witnessRowFor("example-org/one")
	if want := "(on-behalf-of human:" + witnessFixtureLogin + ")"; !strings.Contains(row, want) {
		t.Fatalf("witness row for the private repo = %q, want it to carry %q (unchanged)", row, want)
	}
}

// TestWitnessOnBehalfOfPublicTargetWithNoNeutralNameSaysNothing: with no name:login entry
// for the blessing authority there is no neutral form to name, and the witness must NOT fall
// back to the login. statusgen never blocks a witness write, so it says nothing; the
// attribution lint then reports the unannotated row like any other.
func TestWitnessOnBehalfOfPublicTargetWithNoNeutralNameSaysNothing(t *testing.T) {
	r := witnessVisibilityRoster()
	delete(r, scanEnvHumanLoginMap)
	scanWithRoster(t, r)
	row := witnessRowFor("example-org/pub")
	if strings.Contains(row, "on-behalf-of") || strings.Contains(row, witnessFixtureLogin) {
		t.Fatalf("witness row for a public repo with no neutral name configured = %q, want no annotation "+
			"and no login", row)
	}
	// Perturb: the same roster still annotates the private target, so the silence above is
	// the visibility split's, not a roster-loading failure.
	if row := witnessRowFor("example-org/one"); !strings.Contains(row, "on-behalf-of human:"+witnessFixtureLogin) {
		t.Fatalf("witness row for the private repo = %q, want the login annotation", row)
	}
}

// TestWitnessTargetRepoReadsStreamFrontmatter: the row's target is the `repo:` the brief's
// own stream README declares — and "" (the public form) when it declares none.
func TestWitnessTargetRepoReadsStreamFrontmatter(t *testing.T) {
	dir := t.TempDir()
	readme := "---\nstream: ex\nstatus: active\npriority: P1\ntrack: platform\nrepo: example-org/pub\n---\n\n# EX\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [One](./brief-01-one.md) | 0 | S | implemented | — | — |\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(dir, "brief-01-one.md")
	if got := witnessTargetRepo(brief); got != "example-org/pub" {
		t.Fatalf("witnessTargetRepo = %q, want %q", got, "example-org/pub")
	}
	noRepo := strings.Replace(readme, "repo: example-org/pub\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(noRepo), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := witnessTargetRepo(brief); got != "" {
		t.Fatalf("witnessTargetRepo with no repo: frontmatter = %q, want \"\"", got)
	}
	if got := witnessTargetRepo(filepath.Join(t.TempDir(), "brief-01.md")); got != "" {
		t.Fatalf("witnessTargetRepo with no README = %q, want \"\"", got)
	}
}

// TestPrincipalAttributionAcceptsNeutralName: the lint must accept the form the writers now
// stamp on a public target — a neutral name the human-login map carries for a recognised
// human — or every public-repo Evidence row would read as an unrecognised principal. It is
// not a widening to "any token": the unknown-principal fixture (TestPrincipalAttribution-
// UnknownPrincipalIsProblem) still reds with the same roster.
func TestPrincipalAttributionAcceptsNeutralName(t *testing.T) {
	scanWithRoster(t, scanExampleRoster()) // human map alex:ada; ada is the human
	streams := loadPrincipalFixture(t, "evidence-unknown-principal")
	path := filepath.Join(streams[0].Dir, "brief-01-unknown-principal.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(b), "(on-behalf-of human:ghost)", "(on-behalf-of human:alex)", 1)
	if patched == string(b) {
		t.Fatal("fixture text did not carry the expected annotation — fixture drifted")
	}
	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	streams2, _, err := loadStreams(streams[0].Root)
	if err != nil {
		t.Fatal(err)
	}
	problems, _ := principalAttributionProblems(streams2)
	if hasProblem(problems, "mp/brief-01", "not in this repo's roster human map") {
		t.Fatalf("a neutral name the human-login map carries must be a recognised principal; got:\n%s",
			strings.Join(problems, "\n"))
	}
}

// TestStripOnBehalfOfAcceptsNeutralName: the corroboration lanes strip the on-behalf-of
// MARKER before they look for sign-off vocabulary. A principal in the neutral-name form
// (what a public-target trailer or witness cell now carries) must be stripped too, or the
// stamp lane reads `human:<name>` as an uncorroborated sign-off on every such row. The
// anchors stay: an undeclared name and a non-login-shaped token are left for the stamp lane.
func TestStripOnBehalfOfAcceptsNeutralName(t *testing.T) {
	scanWithRoster(t, scanExampleRoster()) // human map alex:ada
	for in, want := range map[string]string{
		"On-behalf-of: human:alex":          "alex",
		"(on-behalf-of human:alex)":         "(alex)",
		"On-behalf-of: human:alex-approved": "On-behalf-of: human:alex-approved",
		"on-behalf-of human:alex_x)":        "on-behalf-of human:alex_x)",
	} {
		if got := stripOnBehalfOf(in); got != want {
			t.Errorf("stripOnBehalfOf(%q) = %q, want %q", in, got, want)
		}
	}
	cell := "| 1 | `x` | pass exit=0 | sha256:1 | 2026-09-23 | assay-verifier-app[bot] @ b988d175ab12 (on-behalf-of human:alex) |"
	diff := "diff --git a/docs/streams/example/brief-01.md b/docs/streams/example/brief-01.md\n" +
		"--- a/docs/streams/example/brief-01.md\n+++ b/docs/streams/example/brief-01.md\n@@ -1,0 +1,1 @@\n+" + cell + "\n"
	if stamps := stampsInDiff("", diff); len(stamps) != 0 {
		t.Fatalf("a neutral-name on-behalf-of annotation must yield no STAMP: %+v", stamps)
	}
	// A bare human:<name> with no marker is still a stamp — the exemption is the marker only.
	bare := strings.Replace(diff, "(on-behalf-of human:alex)", "human:alex", 1)
	if stamps := stampsInDiff("", bare); len(stamps) != 1 || stamps[0].Name != "alex" {
		t.Fatalf("a bare human:alex stamp must still be gated, got %+v", stamps)
	}
}
