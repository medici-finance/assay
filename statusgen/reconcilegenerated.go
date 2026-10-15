package main

// reconcilegenerated.go — generated output a row move changes, judged and
// written together with the move (#2694).
//
// Some Status-keyed lint rules compare a GENERATED artifact against a fresh
// render of the board: a drive-plan page's drive-snapshot region
// (driveRegionLintProblems) is the one live today. Moving a driven row's Status
// cell changes that render, so the README edit on its own always drifts the
// region. The hold used to stage only the README and judge it against the
// region as it sat on disk, so every witnessed move on a driven stream was held
// for drive-region-drift — a PROBLEM the complete proposed output (the README
// edit plus the region it re-renders) does not carry.
//
// generatedOutputs is the registry of such artifacts. With the --regenerate
// opt-in (holdOptions.regenerate), when a move adds the drift PROBLEM of a
// registered artifact, reconcileWrites tries the move once more with the
// artifact re-rendered against the moved board and staged beside the README,
// and lets the lint — unchanged, every rule — judge that complete
// output. Clean: the row is admitted and reports the regenerated region
// (appliedRow.Generated); --apply writes it beside the README, the read-only
// form keeps it staged. Not clean: the row is held with the PROBLEMs of the
// complete output. Only a drift the move ADDS is answered: a region that was
// already drifting (a hand edit inside the fences) is never re-rendered, so the
// writer never launders the edit the drift rule exists to catch.
//
// The opt-in is off by default because the landing callers — the scheduled
// reconcile job's stray-path check and deskreconcile's changedReadmes — commit
// stream READMEs only and refuse a run that changed anything else. Without it,
// such a move is held on its drift exactly as before, and --apply writes
// stream READMEs only.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generatedOutput is one kind of generated artifact a Status-keyed lint rule
// compares against a fresh render of the board.
type generatedOutput struct {
	rule string // the statusKeyedLintRules entry whose drift PROBLEM this answers
	// drifted returns the artifacts under root whose drift PROBLEM is in added.
	drifted func(root string, added []string) []string
	// regenerate re-renders path against the board as currently staged and
	// returns the whole new file; ok is false when it cannot be rendered.
	regenerate func(root, path string) (content string, change generatedChange, ok bool)
}

// generatedOutputs lists every registered generated artifact.
var generatedOutputs = []generatedOutput{
	{"driveRegionLintProblems", driftedDrivePlans, regenerateDriveRegion},
}

// driftedDrivePlans returns the drive-plan pages under root whose
// drive-region-drift PROBLEM is in added.
func driftedDrivePlans(root string, added []string) []string {
	if len(added) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, p := range added {
		want[p] = true
	}
	dir := filepath.Join(root, "docs", "roadmap", "drives")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if want[driveRegionDriftProblem(e.Name(), strings.TrimSuffix(e.Name(), ".md"))] {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// regenerateDriveRegion re-renders the drive-snapshot region of the plan page
// at path from the board as currently staged (the README move included) and
// splices it between the page's fences. The render is the one
// `statusgen --drive-snapshot <slug>` prints and the drift rule compares
// against; nothing outside the fences changes.
func regenerateDriveRegion(root, path string) (string, generatedChange, bool) {
	name := filepath.Base(path)
	slug := strings.TrimSuffix(name, ".md")
	// The drift PROBLEM is only raised past this containment check; re-check it,
	// since the page may be written.
	if drivePlanEscapeProblem(root, path, name) != "" {
		return "", generatedChange{}, false
	}
	raw, err := readStaged(path)
	if err != nil {
		return "", generatedChange{}, false
	}
	before, ok := extractDriveRegion(string(raw), slug)
	if !ok {
		return "", generatedChange{}, false
	}
	fresh, rc := driveLintSection(&driveLintCtx{root: root}, slug, nowFunc())
	if rc != 0 {
		return "", generatedChange{}, false
	}
	content, ok := spliceDriveRegion(string(raw), slug, fresh)
	if !ok {
		return "", generatedChange{}, false
	}
	return content, generatedChange{Path: path, Drive: slug, RegionBefore: before, RegionAfter: fresh}, true
}

// regeneratedStage is the generated output staged for one move attempt.
type regeneratedStage struct {
	changes  []generatedChange
	contents []string // the whole new file, parallel to changes
}

// driftedOutputs returns, per registered artifact kind, the artifacts whose
// drift PROBLEM a move added.
func driftedOutputs(root string, added []string) (kinds []generatedOutput, paths [][]string) {
	for _, g := range generatedOutputs {
		if ps := g.drifted(root, added); len(ps) > 0 {
			kinds = append(kinds, g)
			paths = append(paths, ps)
		}
	}
	return kinds, paths
}

// stageRegenerated re-renders every drifted artifact against the board as now
// staged and stages the result, returning what it staged and the undo for it.
// An artifact that cannot be rendered is left as it is: the lint then still
// reports its drift and the move is held.
func stageRegenerated(root string, staged *readmeStage, kinds []generatedOutput, paths [][]string) (*regeneratedStage, func() error) {
	out := &regeneratedStage{}
	var undos []func() error
	for i, g := range kinds {
		for _, p := range paths[i] {
			content, change, ok := g.regenerate(root, p)
			if !ok {
				continue
			}
			undos = append(undos, staged.put(p, content))
			out.changes = append(out.changes, change)
			out.contents = append(out.contents, content)
		}
	}
	return out, func() error {
		var first error
		for i := len(undos) - 1; i >= 0; i-- {
			if err := undos[i](); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
}

// writeRegenerated writes each staged artifact to disk (the --apply form).
func writeRegenerated(root string, rs *regeneratedStage) error {
	for i, c := range rs.changes {
		if drivePlanEscapeProblem(root, c.Path, filepath.Base(c.Path)) != "" {
			return fmt.Errorf("%s: refusing to write a generated region outside the tree", c.Path)
		}
		if err := os.WriteFile(c.Path, []byte(rs.contents[i]), 0o644); err != nil {
			return fmt.Errorf("%s: %w", c.Path, err)
		}
	}
	return nil
}
