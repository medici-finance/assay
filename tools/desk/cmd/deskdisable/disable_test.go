package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- Verify row 2: inside-only install -> disable -> byte-identical tree ---

func TestDisable_InsideOnly_ByteIdenticalAfterDisable(t *testing.T) {
	root := t.TempDir()
	gitInitFixture(t, root)

	// Adopter content placed BEFORE install, in the same directories Assay's
	// own scaffolds touch — proves the inverses are surgical, not a blast
	// radius (threat model: "an inverse deletes a file Assay did not create").
	placeAdopterContent(t, root, "docs/streams/adopter-notes.md", "pre-existing adopter content\n")
	placeAdopterContent(t, root, "README.md", "adopter's own top-level readme\n")

	writeManifest(t, root, "components/streams-scaffold", streamsScaffoldManifest)
	writeManifest(t, root, "components/main-guard", mainGuardManifest)

	before := treeSnapshot(t, root)

	installStreamsScaffold(t, root)
	installMainGuard(t, root)

	if _, err := os.Stat(filepath.Join(root, "docs", "streams", streamsReadmeName)); err != nil {
		t.Fatalf("fixture install did not create the streams README: %v", err)
	}

	// Verify row 2's literal order: main-guard, then streams-scaffold.
	if _, err := runDisable(root, "assay/main-guard", false, true); err != nil {
		t.Fatalf("disable assay/main-guard: %v", err)
	}
	if _, err := runDisable(root, "assay/streams-scaffold", false, true); err != nil {
		t.Fatalf("disable assay/streams-scaffold: %v", err)
	}

	after := treeSnapshot(t, root)
	assertTreeUnchanged(t, before, after)
}

// --- Verify row 3: ledger line count after install, before disable --------

func TestFixture_LedgerLineAfterOutsideInstall(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "components/labels", labelsManifest)

	installLabels(t, root, "assay-desk-app")

	entries, err := deskkit.ReadLedger(root)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d ledger line(s), want 1 (one outside step executed)", len(entries))
	}
	if entries[0].Kind != "label" {
		t.Errorf(`entries[0].Kind = %q, want "label"`, entries[0].Kind)
	}

	raw, err := os.ReadFile(deskkit.LedgerPath(root))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d raw ledger line(s), want 1", len(lines))
	}
	labelCount := 0
	for _, l := range lines {
		if strings.Contains(l, `"kind":"label"`) {
			labelCount++
		}
	}
	if labelCount != 1 {
		t.Fatalf(`grep -c '"kind":"label"' = %d, want 1`, labelCount)
	}
}

// --- Verify row 4: labels --dry-run lists for human, deletes nothing ------

func TestDisable_LabelsDryRun_ListsForHuman(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "components/labels", labelsManifest)
	installLabels(t, root, "assay-desk-app")

	before := treeSnapshot(t, root)
	beforeLedger, err := os.ReadFile(deskkit.LedgerPath(root))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := runDisable(root, "assay/labels", true /* dry-run */, false)
	if err != nil {
		t.Fatalf("dry-run must exit clean; got error: %v (plan:\n%s)", err, fmtSteps(plan))
	}
	if len(plan.Steps) == 0 {
		t.Fatalf("expected at least one plan step")
	}
	for _, s := range plan.Steps {
		if !strings.HasPrefix(s.Detail, "list-for-human:") {
			t.Errorf("step %s/%s detail = %q, want prefix \"list-for-human:\"", s.Component, s.StepID, s.Detail)
		}
	}
	mustPlanContains(t, plan, "review-request") // named from the ledger

	after := treeSnapshot(t, root)
	assertTreeUnchanged(t, before, after) // dry-run touches nothing

	afterLedger, err := os.ReadFile(deskkit.LedgerPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeLedger) != string(afterLedger) {
		t.Errorf("dry-run must not touch the ledger")
	}
}

// --- Verify row 5: refuses to strand an active dependent -------------------

const deskToolsManifest = `component: assay/desk-tools
version: 0.28.0
provides:
  - assay.desk.verbs
inject:
  required: []
apply:
  - id: desk-binaries
    effect: install the pinned desk-tools binaries on PATH and pin them in .assay-versions
    boundary: inside
    inverse: remove the desk-tools binaries and their entries from .assay-versions
`

const prReviewDeskManifest = `component: assay/pr-review-desk
version: 0.28.0
provides:
  - assay.desk.role.pr-review-desk
inject:
  required:
    - key: assay.desk.verbs
      range: ">=0.20.0 <1.0.0"
apply:
  - id: skill-body
    effect: install the pr-review-desk skill directory into the harness plugin cache
    boundary: inside
    inverse: remove the skill directory from the harness plugin cache
`

func TestDisable_RefusesActiveDependent(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "tools/desk", deskToolsManifest)
	writeManifest(t, root, "plugins/assay/skills/pr-review-desk", prReviewDeskManifest)

	before := treeSnapshot(t, root)

	_, err := runDisable(root, "assay/desk-tools", false, true)
	if err == nil {
		t.Fatalf("expected a refusal: assay/pr-review-desk still requires assay.desk.verbs")
	}
	if !strings.Contains(err.Error(), "assay/pr-review-desk") {
		t.Errorf("refusal must name the dependent; got: %v", err)
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Errorf("exit code = %d, want ExitRefused (%d)", deskkit.ExitCodeOf(err), deskkit.ExitRefused)
	}

	after := treeSnapshot(t, root)
	assertTreeUnchanged(t, before, after) // nothing changed
}

// A --cascade listing the dependent first must succeed, and must disable the
// dependent (whose only step here has a registered executor) before the
// target — proving cascade actually reorders rather than just suppressing
// the refusal.
const worktreeSkillManifest = `component: assay/pr-review-desk
version: 0.28.0
provides:
  - assay.desk.role.pr-review-desk
inject:
  required:
    - key: assay.desk.verbs
apply:
  - id: skill-body
    effect: create docs/streams/ under a second root (fixture-only stand-in for a skill directory)
    boundary: inside
    inverse: remove docs/streams/
`

func TestDisable_CascadeDisablesDependentFirst(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "components/streams-scaffold", streamsScaffoldManifest)
	writeManifest(t, root, "fixture/pr-review-desk", worktreeSkillManifest)
	installStreamsScaffold(t, root)

	// Register a temporary executor for the fixture-only dependent's step, the
	// same way the mutation test (below) temporarily swaps one out — this
	// test exists to prove ORDERING, not to add a new production component,
	// so reusing streamsScaffoldInverse's (idempotent) behaviour is enough.
	key := execKey("assay/pr-review-desk", "skill-body")
	insideExecutors[key] = streamsScaffoldInverse
	defer delete(insideExecutors, key)

	plan, err := runDisable(root, "assay/streams-scaffold", false, true, "assay/pr-review-desk")
	if err != nil {
		t.Fatalf("cascade disable: %v (plan:\n%s)", err, fmtSteps(plan))
	}
	if len(plan.Cascade) != 1 || plan.Cascade[0] != "assay/pr-review-desk" {
		t.Errorf("plan.Cascade = %v, want [assay/pr-review-desk]", plan.Cascade)
	}
	// The cascade component's step must appear BEFORE the target's step.
	firstComponent := plan.Steps[0].Component
	if firstComponent != "assay/pr-review-desk" {
		t.Errorf("first step's component = %q, want the cascaded dependent first", firstComponent)
	}
}

// --- Verify row 7 (mutation test): a no-op inverse is caught by the tree diff, never silently passed

func TestDisable_MutationNoOpInverseIsCaughtByTreeDiff(t *testing.T) {
	root := t.TempDir()
	gitInitFixture(t, root)
	writeManifest(t, root, "components/main-guard", mainGuardManifest)

	before := treeSnapshot(t, root)
	installMainGuard(t, root)

	// Mutation: swap the registered executor for a no-op, exactly the failure
	// mode named in the threat model ("a no-op inverse passes silently").
	// Restored via defer so no other test in this package observes it.
	key := execKey("assay/main-guard", "hooks-path")
	original := insideExecutors[key]
	insideExecutors[key] = func(string) error { return nil }
	defer func() { insideExecutors[key] = original }()

	if _, err := runDisable(root, "assay/main-guard", false, true); err != nil {
		t.Fatalf("a no-op executor must not itself error: %v", err)
	}

	after := treeSnapshot(t, root)
	diffs := treeDiff(before, after)
	if len(diffs) == 0 {
		t.Fatalf("a no-op inverse must be CAUGHT by the tree-diff check (it silently passed instead)")
	}
	// Restore for real and prove the genuine executor closes the gap the
	// mutation opened — the fail-first half of this test's story.
	insideExecutors[key] = original
	if _, err := runDisable(root, "assay/main-guard", false, true); err != nil {
		t.Fatalf("restored executor: disable failed: %v", err)
	}
	after2 := treeSnapshot(t, root)
	assertTreeUnchanged(t, before, after2)
}

// --- Verify row 10: full reverse then re-install is idempotent -------------

func TestDisable_FullReverseThenReinstallIsIdempotent(t *testing.T) {
	root := t.TempDir()
	gitInitFixture(t, root)
	writeManifest(t, root, "components/streams-scaffold", streamsScaffoldManifest)
	writeManifest(t, root, "components/main-guard", mainGuardManifest)

	installStreamsScaffold(t, root)
	installMainGuard(t, root)
	firstInstall := treeSnapshot(t, root)

	if _, err := runDisable(root, "assay/streams-scaffold", false, true); err != nil {
		t.Fatalf("disable streams-scaffold: %v", err)
	}
	if _, err := runDisable(root, "assay/main-guard", false, true); err != nil {
		t.Fatalf("disable main-guard: %v", err)
	}

	installStreamsScaffold(t, root)
	installMainGuard(t, root)
	secondInstall := treeSnapshot(t, root)

	if diffs := treeDiff(firstInstall, secondInstall); len(diffs) > 0 {
		t.Fatalf("second install differs from first after a full reverse:\n  %s", strings.Join(diffs, "\n  "))
	}
}

// --- Missing manifest / missing cascade name / unregistered inside step ----

func TestDisable_UnknownComponentRefuses(t *testing.T) {
	root := t.TempDir()
	if _, err := runDisable(root, "assay/does-not-exist", false, true); err == nil {
		t.Fatalf("expected a refusal for an unknown component")
	}
}

// TestDisable_MisclassifiedOutsideStepRefusesRatherThanExecute proves the
// brief's single-point-of-failure claim: "an outside step mis-labelled inside
// gets a destructive inverse instead of a listed compensation... deskdisable
// refuses to act on any forge object it cannot find in the ledger, so a
// mis-classified forge step has no id to act on and falls through to the
// human checklist" (component-model.md §4's boundary classification is the
// SPOF; the registered-executor allowlist is the layer behind it). A fixture
// component here claims `boundary: inside` for a step that is really a forge
// effect (creating something on the forge). Since insideExecutors is a
// closed allowlist of components this implementation has specifically
// verified are safe, adopter-repo-relative file operations, the
// mis-classified step has no entry — deskdisable refuses rather than execute
// a destructive "inverse" against something it never actually created
// locally.
func TestDisable_MisclassifiedOutsideStepRefusesRatherThanExecute(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "fixture/mislabelled", `component: assay/fixture-mislabelled
version: 0.28.0
provides:
  - assay.fixture.mislabelled
inject:
  required: []
apply:
  - id: create-forge-thing
    effect: create something on the forge (MIS-LABELLED below — this is really an outside effect)
    boundary: inside
    inverse: delete the forge thing
`)
	before := treeSnapshot(t, root)
	_, err := runDisable(root, "assay/fixture-mislabelled", false, true)
	if err == nil {
		t.Fatalf("a mis-classified (outside-as-inside) step must be refused, not executed")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Errorf("exit code = %d, want ExitRefused", deskkit.ExitCodeOf(err))
	}
	after := treeSnapshot(t, root)
	assertTreeUnchanged(t, before, after) // the allowlist caught it before any write
}

func TestDisable_UnregisteredInsideStepRefusesBeforeTouchingAnything(t *testing.T) {
	root := t.TempDir()
	// A skill component's inverse target is harness-defined (a later brief
	// not landed) — no executor is registered for it.
	writeManifest(t, root, "plugins/assay/skills/pdfingest", `component: assay/pdfingest
version: 0.28.0
provides:
  - assay.skill.pdfingest
inject:
  required: []
apply:
  - id: skill-body
    effect: install the pdfingest skill directory into the harness plugin cache
    boundary: inside
    inverse: remove the skill directory from the harness plugin cache
`)
	before := treeSnapshot(t, root)
	_, err := runDisable(root, "assay/pdfingest", false, true)
	if err == nil {
		t.Fatalf("expected a refusal: no registered executor for this inside step")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Errorf("exit code = %d, want ExitRefused", deskkit.ExitCodeOf(err))
	}
	after := treeSnapshot(t, root)
	assertTreeUnchanged(t, before, after)
}
