package deskkit

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// releaseAuthorizerCouplingProblems is the source-coupling check behind
// TestReleaseAuthorizerStampedFromReleaseWorkflow and its mutation control,
// TestReleaseAuthorizerStampMissingIsCaught. It takes the text of
// .github/workflows/release.yml and returns every way the release-authorizer
// traceability wiring (the release-authorizer brief) is absent from it. An
// empty slice means the wiring is present; a non-empty slice is what the
// coupling test reports as failure.
//
// WHAT IT GUARDS — the two halves the brief requires:
//
//  1. the `resolve` job EMITS an authorizer output: declared in the job's
//     outputs block and written through the single-line-refusing emit helper,
//     with the dispatch path carrying the SAME $ACTOR the tag message already
//     interpolates (one source for one fact, never a second derivation) and
//     the tag-push path an explicit not-recorded value, never blank.
//  2. the create-release step CONSUMES that output through env:
//     (RELEASE_AUTHORIZER), and the python3 notes helper reads it from the
//     environment — the same shape RELEASE_TAG and DESK_IMAGE_REF already
//     use, never a ${{ }} splice inside run:, because a login is
//     attacker-shaped text the tag-format gate does not cover — and the notes
//     body carries the labelled authorized-by line.
//
// COVERAGE BOUNDARY (docs/mistake-proofing.md D6 — stated here so it travels
// with the check). This check pins PRESENCE of the wiring, not its adequacy,
// and the wiring it pins is a readable copy, not a gate: the body line
// records an authorization that happens in the gated `release` environment,
// where a named human approves the run. If the body line and the environment
// approval ever disagree, the environment is the authority. The tag-push path
// records no authorizer BY DESIGN — the tag already exists when the workflow
// starts, so there is no dispatch actor, and the pushing token's identity
// must never be printed as though it were one. The line records who
// authorized the cut, never who or what built the artifact; the pipeline
// carries no signature or provenance attestation, and nothing here may be
// read as claiming one.
func releaseAuthorizerCouplingProblems(wf string) []string {
	var problems []string
	for _, c := range []struct{ want, problem string }{
		{
			want:    "authorizer: ${{ steps.tag.outputs.authorizer }}",
			problem: "the resolve job does not declare the authorizer output",
		},
		{
			want:    `emit authorizer "$ACTOR`,
			problem: `the dispatch path does not emit the dispatch actor as the authorizer — the SAME $ACTOR the tag message already carries; a second, independently derived copy of that fact is the defect this repo's derive-or-diff convention exists to prevent`,
		},
		{
			want:    `emit authorizer "not recorded`,
			problem: `the tag-push path does not emit an explicit not-recorded authorizer — a blank where an authorizer belongs reads as "nobody", a stronger claim than the truth`,
		},
		{
			want:    "RELEASE_AUTHORIZER: ${{ needs.resolve.outputs.authorizer }}",
			problem: "the create-release step does not pass the authorizer through env: — a login is user-controlled and must ride in via the environment, never a ${{ }} splice inside run:",
		},
		{
			want:    `os.environ["RELEASE_AUTHORIZER"]`,
			problem: "the release-notes helper does not read the authorizer from the environment (the shape RELEASE_TAG and DESK_IMAGE_REF already use)",
		},
		{
			want:    "authorized-by: ",
			problem: "the release-notes body does not carry the labelled authorized-by line",
		},
	} {
		if !strings.Contains(wf, c.want) {
			problems = append(problems, c.problem)
		}
	}
	// A blank emit is the one failure mode the presence checks above cannot
	// catch, because it removes no guarded string: refuse it explicitly.
	if strings.Contains(wf, `emit authorizer ""`) {
		problems = append(problems, `the authorizer is emitted blank (emit authorizer "") — never print an empty field where an authorizer belongs`)
	}
	sort.Strings(problems)
	return problems
}

// TestReleaseAuthorizerStampedFromReleaseWorkflow is the source-coupling test
// for the release-authorizer traceability work: it reads
// .github/workflows/release.yml and reddens if the
// release-authorizer traceability wiring is dropped from it — either half,
// the resolve job no longer emitting an authorizer output or the
// create-release step no longer passing it through env: into the notes body.
// It follows TestCellctlPackagedInReleaseWorkflow's shape (a distinct, named
// test reading the workflow source, so no broader -run pattern can pass
// without this wiring ever being present).
func TestReleaseAuthorizerStampedFromReleaseWorkflow(t *testing.T) {
	// internal/deskkit sits at tools/desk/internal/deskkit; the repo root is four
	// levels up.
	path := filepath.Join("..", "..", "..", "..", ".github", "workflows", "release.yml")
	skipIfFixtureAbsent(t, path,
		".github/ is not part of this repository's published file set")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("release workflow not readable at %s: %v", path, err)
	}
	wf := string(raw)
	for _, p := range releaseAuthorizerCouplingProblems(wf) {
		t.Errorf("release.yml dropped the release-authorizer wiring: %s", p)
	}
}

// TestReleaseAuthorizerStampMissingIsCaught is the mutation row (rule 16) for
// the coupling test above: each guarded string is removed from a copy of the
// workflow in turn, and EVERY removal must make
// releaseAuthorizerCouplingProblems report a problem. A coupling test that
// still passes with the guarded text deleted guards nothing; this positive
// control proves the check can fail on every suite execution, not only inside
// a mutation harness.
func TestReleaseAuthorizerStampMissingIsCaught(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", ".github", "workflows", "release.yml")
	skipIfFixtureAbsent(t, path,
		".github/ is not part of this repository's published file set")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("release workflow not readable at %s: %v", path, err)
	}
	wf := string(raw)

	// Positive control on the intact tree: the check must report nothing
	// BEFORE any mutation, or the mutations below prove nothing.
	if problems := releaseAuthorizerCouplingProblems(wf); len(problems) != 0 {
		t.Fatalf("the intact release.yml already reports problems (%v) — fix the wiring before proving the check can catch its absence", problems)
	}

	for _, guarded := range []string{
		"authorizer: ${{ steps.tag.outputs.authorizer }}",
		`emit authorizer "$ACTOR`,
		`emit authorizer "not recorded`,
		"RELEASE_AUTHORIZER: ${{ needs.resolve.outputs.authorizer }}",
		`os.environ["RELEASE_AUTHORIZER"]`,
		"authorized-by: ",
	} {
		if !strings.Contains(wf, guarded) {
			t.Errorf("guard string %q is not present in release.yml — the coupling test would never fire; the workflow and the test have drifted apart", guarded)
			continue
		}
		mutated := strings.Replace(wf, guarded, "REMOVED-BY-MUTATION-CONTROL", 1)
		if problems := releaseAuthorizerCouplingProblems(mutated); len(problems) == 0 {
			t.Errorf("removing %q was NOT caught — the coupling test passes with the authorizer wiring deleted, so it guards nothing", guarded)
		}
	}
}
