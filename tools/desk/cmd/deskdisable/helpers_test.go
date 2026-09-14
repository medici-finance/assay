package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// writeManifest drops one component.yaml at <root>/<dir>/component.yaml — the
// same fixture shape deskmanifest_test.go uses, so the two tools' tests read
// as one family.
func writeManifest(t *testing.T, root, dir, body string) {
	t.Helper()
	d := filepath.Join(root, dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "component.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitInitFixture git-inits a throwaway directory so main-guard's
// `git config core.hooksPath` has a repo to act on — this is the ONLY git
// repo any deskdisable test ever touches (ground rule: deskdisable never runs
// against any repo other than a throwaway fixture during implementation and
// verification).
func gitInitFixture(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v: %s", dir, err, out)
	}
}

// treeSnapshot walks root and records every regular file's content by
// root-relative path, plus a marker for every EMPTY directory (so a directory
// Assay's install created and disable failed to clean up is still visible as
// a difference, the way an empty directory shows up as its own entry in a
// tar archive).
func treeSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	snap := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				snap[rel+"/"] = nil
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snap[rel] = data
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotting %s: %v", root, err)
	}
	return snap
}

// treeDiff is the Go-native equivalent of Verify row 2's `tar cf`/`tar df`
// byte-identical check: every path present before must be present after with
// identical content, and nothing new may have appeared. Returns a sorted,
// human-readable diff list; empty means byte-identical.
func treeDiff(before, after map[string][]byte) []string {
	var diffs []string
	for k, v := range before {
		v2, ok := after[k]
		if !ok {
			diffs = append(diffs, "missing after disable: "+k)
			continue
		}
		if !bytes.Equal(v, v2) {
			diffs = append(diffs, "changed: "+k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			diffs = append(diffs, "leftover after disable: "+k)
		}
	}
	sort.Strings(diffs)
	return diffs
}

// assertTreeUnchanged fails the test with the full diff list unless the tree
// is byte-identical to the pre-install snapshot.
func assertTreeUnchanged(t *testing.T, before, after map[string][]byte) {
	t.Helper()
	if diffs := treeDiff(before, after); len(diffs) > 0 {
		t.Fatalf("tree not byte-identical to pre-install snapshot:\n  %s", strings.Join(diffs, "\n  "))
	}
}

// --- scripted "install" helpers -------------------------------------------
//
// These exist ONLY to fabricate the "already installed" precondition
// deskdisable's real executors then reverse. No compiled installer exists yet
// for these components (component-model.md §12: "install is idempotent and
// refuse-not-clobber" today, done by hand or by an agent following
// docs/adopting-assay.md); this is exactly what Task 5's "scripted install"
// asks the implementer to supply, and it deliberately mirrors — never
// duplicates — the executors' own target paths (executors.go), the same way
// a unit test's setUp mirrors the code under test without becoming a second
// implementation of it.

const streamsScaffoldManifest = `component: assay/streams-scaffold
version: 0.28.0
provides:
  - assay.streams
inject:
  required: []
apply:
  - id: streams-tree
    effect: create docs/streams/ with a README and the per-stream layout
    boundary: inside
    inverse: remove docs/streams/ (the README and the per-stream layout it created)
`

const mainGuardManifest = `component: assay/main-guard
version: 0.28.0
provides:
  - assay.main-guard
inject:
  required: []
apply:
  - id: hooks-path
    effect: install .githooks/pre-commit and set core.hooksPath
    boundary: inside
    inverse: unset core.hooksPath and remove .githooks/pre-commit
`

const labelsManifest = `component: assay/labels
version: 0.28.0
provides:
  - assay.labels
inject:
  required: []
apply:
  - id: label-set
    effect: create the review-request / raised-by:* label set on the forge
    boundary: outside
    ledger: label
    compensation: list-for-human — never delete unattended (a label may still be carried by an open issue); a human checks and deletes
`

// installStreamsScaffold fabricates assay/streams-scaffold's one effect.
func installStreamsScaffold(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, streamsReadmeName), []byte("# streams/\n\nScaffolded by assay/streams-scaffold.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// installMainGuard fabricates assay/main-guard's one effect. root MUST already
// be a git repo (gitInitFixture).
func installMainGuard(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".githooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "config", "core.hooksPath", ".githooks")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config core.hooksPath: %v: %s", err, out)
	}
}

// installLabels fabricates assay/labels' one OUTSIDE effect: since deskdisable
// never talks to a real forge (offline envelope; no live cluster/prod
// contact), "creating the label" is recorded the same way a real installer's
// outside step would — a ledger line — without any network call.
func installLabels(t *testing.T, root string, by string) {
	t.Helper()
	if err := deskkit.AppendLedger(root, deskkit.LedgerEntry{
		Component: "assay/labels",
		Version:   "0.28.0",
		Step:      "label-set",
		Kind:      "label",
		ID:        "review-request",
		By:        by,
	}); err != nil {
		t.Fatalf("installLabels: AppendLedger: %v", err)
	}
}

// placeAdopterContent drops a file the fixture "adopter" placed before install
// — content Assay must never delete. Used to prove row 2's byte-identical
// check is actually exercising the surgical (not blast-radius) inverses.
func placeAdopterContent(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runDisable is a small wrapper so tests read as "run the CLI" rather than
// calling Disable() directly, matching how the Verify table's rows are
// phrased as commands.
func runDisable(root, target string, dryRun, yes bool, cascade ...string) (*Plan, error) {
	return Disable(Options{Root: root, Target: target, DryRun: dryRun, Cascade: cascade})
}

func mustPlanContains(t *testing.T, plan *Plan, substr string) {
	t.Helper()
	for _, s := range plan.Steps {
		if strings.Contains(s.Detail, substr) {
			return
		}
	}
	t.Fatalf("plan detail never contains %q; steps: %+v", substr, plan.Steps)
}

func fmtSteps(plan *Plan) string {
	if plan == nil {
		return "<nil plan>"
	}
	var b strings.Builder
	for _, s := range plan.Steps {
		fmt.Fprintf(&b, "[%s/%s] %s: %s\n", s.Component, s.StepID, s.Kind, s.Detail)
	}
	return b.String()
}
