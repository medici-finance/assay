package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// phantompartial_test.go — #6751, the deskdispatch site of the same defect: the phantom check read a
// MERGED PR's `Brief:` trailer as "already DELIVERED" even when the brief's own `delivery:` list
// declares that PR `covers: partial`. statusgen offers such a row; the fresh dispatch must not refuse
// it. Slugs and aliases are synthetic; the registry maps the fixture alias to allowedRepo.

func phantomPartialBrief(t *testing.T, delivery ...string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "example-partial")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	reg := "schema: graph-repos-v1\ncell: example\nrepos:\n  example: {cell: example, repo: " + allowedRepo + "}\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("---\nbrief: example:example:example-partial:15\ntitle: Partial fixture\neffort: L\ngate: model\n")
	b.WriteString("risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\ndelivery:\n")
	for _, d := range delivery {
		b.WriteString("  - " + d + "\n")
	}
	b.WriteString("schema: brief-v2\nversion: 1\n---\n\n# Partial fixture\n")
	path := filepath.Join(dir, "brief-15-x.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestPhantomPartialMergeAdmits: the brief's only merged PRs are declared partial deliveries, so the
// fresh dispatch is admitted (no refusal, not a follow-up).
//
// regression: #6751
func TestPhantomPartialMergeAdmits(t *testing.T) {
	brief := phantomPartialBrief(t,
		`{in: "example#5127", covers: partial, note: "first slice"}`,
		`{in: "example#6001", covers: partial, note: "second slice"}`)
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{
			{Number: 6001, State: "MERGED", Body: "Brief: example-partial/15"},
			{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"},
		}, nil
	})
	fu, err := phantomCheck(dispatchOpts{item: "assay--example-partial--15", kit: "worker", brief: brief}, allowedRepo)
	if err != nil {
		t.Fatalf("a brief whose merged PRs are all declared covers: partial must be dispatchable (#6751): %v", err)
	}
	if fu.pr != 0 {
		t.Errorf("a fresh dispatch over partial deliveries is not a follow-up: %+v", fu)
	}
}

// TestPhantomUndeclaredRefuses: an undeclared merged PR still delivers the brief.
func TestPhantomUndeclaredRefuses(t *testing.T) {
	brief := phantomPartialBrief(t, `{in: "example#5127", covers: partial, note: "first slice"}`)
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{
			{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"},
			{Number: 7000, State: "MERGED", Body: "Brief: example-partial/15"},
		}, nil
	})
	_, err := phantomCheck(dispatchOpts{item: "assay--example-partial--15", kit: "worker", brief: brief}, allowedRepo)
	if err == nil || !strings.Contains(err.Error(), "#7000") {
		t.Fatalf("an undeclared merged PR must still refuse the fresh dispatch, naming it: %v", err)
	}
}

// TestPhantomUnresolvedRefuses: a partial claim on an alias the registry does not carry is
// could-not-check, so nothing is set aside and the merged PR still refuses.
func TestPhantomUnresolvedRefuses(t *testing.T) {
	brief := phantomPartialBrief(t, `{in: "missing#5127", covers: partial, note: "first slice"}`)
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"}}, nil
	})
	if _, err := phantomCheck(dispatchOpts{item: "assay--example-partial--15", kit: "worker", brief: brief}, allowedRepo); err == nil {
		t.Fatal("an unresolvable partial claim must not admit the dispatch")
	}
}

// TestPhantomPartialReworkFollows: --rework keeps following up the newest merged PR, partial or not,
// so the worktree never re-cuts a merged branch's name.
func TestPhantomPartialReworkFollows(t *testing.T) {
	brief := phantomPartialBrief(t, `{in: "example#5127", covers: partial, note: "first slice"}`)
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"}}, nil
	})
	fu, err := phantomCheck(dispatchOpts{item: "assay--example-partial--15", kit: "worker", brief: brief, rework: true}, allowedRepo)
	if err != nil || fu.pr != 5127 {
		t.Fatalf("--rework over a partial merge must follow up #5127: %+v %v", fu, err)
	}
}
