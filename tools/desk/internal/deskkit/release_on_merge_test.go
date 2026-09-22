package deskkit

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Release by merge (iso-9001/07) source-coupling test.
//
// WHY IT READS THE STAGED FILES, NOT THE LIVE ONES. No App in this project may
// push a .github/workflows/* file (GitHub hard-rejects it), so the two release-
// by-merge workflow files land under ci/staged-workflows/ first and a maintainer
// promotes them. Until promotion the LIVE .github/workflows/release.yml carries
// none of this wiring, so a coupling check over the live file would redden the
// suite the moment this brief merged. This check therefore pins the wiring in the
// STAGED edit surface — the reviewable artifact — and reddens if a later edit to
// those staged files drops it. When a maintainer promotes (and, per
// desk-supervision/12, the staged-copy landing is eventually retired), the guard
// that the LIVE release.yml carries the tag-push authorizer belongs to
// TestReleaseAuthorizerStampedFromReleaseWorkflow, which already reads the live
// file; this check's home moves to the live paths at that point.
//
// COVERAGE BOUNDARY (docs/mistake-proofing.md D6). This pins PRESENCE of the
// wiring in the staged files, not its live behaviour (which cannot exist until a
// maintainer promotes), and not its adequacy. The release-by-merge control is the
// release-PR marker plus the merger's identity; the tag immutability ruleset,
// release.yml's refusal of an unmarked tag, and the gated release environment are
// the independent layers behind it. This check proves none of those fire; it
// proves the source that wires them was not silently deleted.

const (
	stagedReleaseOnMerge = "release-on-merge.yml"
	stagedReleaseTwin    = "release.yml"
)

func stagedWorkflowPath(name string) string {
	// internal/deskkit sits at tools/desk/internal/deskkit; the repo root is four
	// levels up, and the staged workflows live at ci/staged-workflows/.
	return filepath.Join("..", "..", "..", "..", "ci", "staged-workflows", name)
}

// releaseOnMergeGuards are the strings whose presence in
// ci/staged-workflows/release-on-merge.yml proves each half of the merge-detect-
// and-tag design is wired. Each is ASCII so no guard carries a unicode literal.
var releaseOnMergeGuards = []struct{ want, problem string }{
	{"branches: [main]", "release-on-merge.yml no longer triggers on push to main — the merge that cuts the release would never be detected"},
	{"actions/create-github-app-token@", "release-on-merge.yml no longer mints an App token — a GITHUB_TOKEN-created tag does NOT trigger release.yml (recursion guard), so the release would build nothing"},
	{"app-id: ${{ secrets.RELEASE_APP_ID }}", "release-on-merge.yml no longer names the release-cutter App secret RELEASE_APP_ID — the reviewer cannot see which credential must exist before promotion"},
	{`title_re = re.compile(r'^release: (v[0-9]+\.[0-9]+\.[0-9]+)$')`, "release-on-merge.yml no longer anchors the release-PR title marker — a PR that merely mentions a version could be mistaken for a release PR"},
	{`re.search(r'(?m)^RELEASE: '`, "release-on-merge.yml no longer requires the anchored RELEASE body marker — the machine-readable half of the marker is gone"},
	{`pr.get("merge_commit_sha") != sha`, "release-on-merge.yml no longer ties the release PR to THIS pushed commit — it could tag a commit that is not the merge of the release PR"},
	{`create_tag "assay/${TAG}"`, "release-on-merge.yml no longer creates the umbrella assay/vX.Y.Z tag"},
	{`create_tag "${TAG}"`, "release-on-merge.yml no longer creates the plain vX.Y.Z tag — the tag that triggers release.yml's build"},
	{"records no merged_by login", "release-on-merge.yml no longer refuses a release PR with no recorded merger — it could cut a release with no authorizer"},
}

// releaseTwinGuards are the strings whose presence in
// ci/staged-workflows/release.yml proves the tag-push path resolves the authorizer
// from the merged release PR and refuses an unmarked tag (closing iso-9001/04's
// tag-push gap). Chosen to be UNIQUE to this brief's additive change: the live
// release.yml already carries `pull-requests: read` on the release job, so that
// string alone would not prove the addition.
var releaseTwinGuards = []struct{ want, problem string }{
	{"commits/${SHA}/pulls", "the staged release.yml tag-push path no longer resolves the merged PR from the tag's commit"},
	{`TAG_WANT="$tag" python3`, "the staged release.yml tag-push path no longer runs the marker/authorizer resolution helper"},
	{`emit authorizer "$login (merged release PR #$number)"`, "the staged release.yml tag-push path no longer emits the authorizer from the merged PR's merged_by.login — iso-9001/04's tag-push gap is reopened"},
	{"is not a merged release PR (title", "the staged release.yml tag-push path no longer refuses a v* tag whose commit is not a merged release PR — a stray or hand-cut tag would build a release"},
}

func releaseByMergeStagedProblems(onMerge, twin string) []string {
	var problems []string
	for _, c := range releaseOnMergeGuards {
		if !strings.Contains(onMerge, c.want) {
			problems = append(problems, c.problem)
		}
	}
	for _, c := range releaseTwinGuards {
		if !strings.Contains(twin, c.want) {
			problems = append(problems, c.problem)
		}
	}
	sort.Strings(problems)
	return problems
}

func readStagedWorkflow(t *testing.T, name string) string {
	t.Helper()
	p := stagedWorkflowPath(name)
	skipIfFixtureAbsent(t, p, "ci/staged-workflows/ is not part of this repository's published file set")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("staged workflow not readable at %s: %v", p, err)
	}
	return string(raw)
}

// TestReleaseByMergeWiringStaged reddens if either staged workflow drops the
// release-by-merge wiring. It also asserts release-on-merge.yml's own
// GITHUB_TOKEN is not granted a write scope — the tag write is the App token, and
// a contents:write / actions:write grant here would be a broadening the design
// forbids.
func TestReleaseByMergeWiringStaged(t *testing.T) {
	onMerge := readStagedWorkflow(t, stagedReleaseOnMerge)
	twin := readStagedWorkflow(t, stagedReleaseTwin)

	for _, p := range releaseByMergeStagedProblems(onMerge, twin) {
		t.Errorf("staged release-by-merge wiring dropped: %s", p)
	}

	// release-on-merge.yml's own GITHUB_TOKEN must stay read-only: the tag write
	// rides on the App token, and it never dispatches a workflow, so it needs
	// neither contents:write nor actions:write. Checked against real YAML keys,
	// not comment prose (the header explains WHY these are withheld and names
	// them, which a substring match would misread as a grant).
	if grantsWriteScope(onMerge, "contents") {
		t.Error("release-on-merge.yml grants contents: write to GITHUB_TOKEN — the tag write must be the App token, not GITHUB_TOKEN")
	}
	if grantsWriteScope(onMerge, "actions") {
		t.Error("release-on-merge.yml grants actions: write — it creates a tag, it does not dispatch a workflow; actions:write also cancels runs and deletes logs repo-wide")
	}
}

// grantsWriteScope reports whether the workflow YAML has a real permissions key
// granting `<scope>: write`, ignoring comment lines (a `#`-led line that merely
// names the scope in prose is not a grant).
func grantsWriteScope(wf, scope string) bool {
	for _, ln := range strings.Split(wf, "\n") {
		s := strings.TrimSpace(ln)
		if strings.HasPrefix(s, "#") {
			continue
		}
		if s == scope+": write" {
			return true
		}
	}
	return false
}

// TestReleaseByMergeWiringMissingIsCaught is the mutation control (rule 16): each
// guarded string is removed from a copy of the file it belongs to, and EVERY
// removal must make releaseByMergeStagedProblems report a problem. A coupling
// check that still passes with the wiring deleted guards nothing.
func TestReleaseByMergeWiringMissingIsCaught(t *testing.T) {
	onMerge := readStagedWorkflow(t, stagedReleaseOnMerge)
	twin := readStagedWorkflow(t, stagedReleaseTwin)

	// Positive control on the intact tree: nothing reported before any mutation,
	// or the mutations below prove nothing.
	if problems := releaseByMergeStagedProblems(onMerge, twin); len(problems) != 0 {
		t.Fatalf("the intact staged files already report problems (%v) — fix the wiring before proving the check can catch its absence", problems)
	}

	for _, guarded := range releaseOnMergeGuards {
		if !strings.Contains(onMerge, guarded.want) {
			t.Errorf("guard %q absent from release-on-merge.yml — the coupling check would never fire", guarded.want)
			continue
		}
		mutated := strings.Replace(onMerge, guarded.want, "REMOVED-BY-MUTATION-CONTROL", 1)
		if problems := releaseByMergeStagedProblems(mutated, twin); len(problems) == 0 {
			t.Errorf("removing %q from release-on-merge.yml was NOT caught — the check guards nothing there", guarded.want)
		}
	}
	for _, guarded := range releaseTwinGuards {
		if !strings.Contains(twin, guarded.want) {
			t.Errorf("guard %q absent from staged release.yml — the coupling check would never fire", guarded.want)
			continue
		}
		mutated := strings.Replace(twin, guarded.want, "REMOVED-BY-MUTATION-CONTROL", 1)
		if problems := releaseByMergeStagedProblems(onMerge, mutated); len(problems) == 0 {
			t.Errorf("removing %q from staged release.yml was NOT caught — the check guards nothing there", guarded.want)
		}
	}
}
