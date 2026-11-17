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
	return phantomPartialTree(t, t.TempDir(), delivery...)
}

// phantomPartialTree plants the partial-delivery fixture brief and the alias registry under root and
// returns the brief's path.
func phantomPartialTree(t *testing.T, root string, delivery ...string) string {
	t.Helper()
	plantRegistry(t, root)
	return phantomBriefAs(t, root, "brief: example:example:example-partial:15\n", delivery...)
}

// phantomBriefAs is phantomPartialTree with the brief file's own `brief:` line given by the caller
// ("" plants a file with no id), still at the example-partial/15 path the item key resolves to.
func phantomBriefAs(t *testing.T, root, idLine string, delivery ...string) string {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "example-partial")
	var b strings.Builder
	b.WriteString("---\n" + idLine + "title: Partial fixture\neffort: L\ngate: model\n")
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

// TestPhantomKeyOnlyPartialAdmits: the documented key-only dispatch (no --brief) reads the brief the
// item key names, so a row whose merged PRs are all declared partial is admitted there too, not
// refused as "already DELIVERED" while fanoutloop offers it.
//
// regression: #6751
func TestPhantomKeyOnlyPartialAdmits(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	phantomPartialTree(t, root, `{in: "example#5127", covers: partial, note: "first slice"}`)
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"}}, nil
	})
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	rc, stderr, _ := bkRun(t, root, "assay--example-partial--15")
	if rc != deskkit.ExitOK || strings.Contains(stderr, "already DELIVERED") {
		t.Fatalf("a key-only dispatch must read the key-named brief's delivery: list (#6751): rc=%d\n%s", rc, stderr)
	}
	if !strings.Contains(stderr, "declares it a partial delivery") {
		t.Errorf("the set-aside NOTICE is missing on the key-only path:\n%s", stderr)
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

// TestPhantomBriefIDBound: the --brief file's `delivery:` list relaxes the phantom check only for the
// brief that file IS. A file whose own `brief:` id differs from the dispatched item, names no id, or
// carries an id that does not parse sets nothing aside: the item's merged PR still refuses the fresh
// dispatch, exactly as it does with no --brief, and a could-not-check NOTICE names the reason.
//
// regression: #6751
func TestPhantomBriefIDBound(t *testing.T) {
	cases := []struct{ name, idLine, notice string }{
		{"different brief", "brief: example:example:example-other:02\n", "example-other/02, not the dispatched item example-partial/15"},
		{"no id", "", "carries no `brief:` id"},
		{"unparseable id", "brief: \"not an id\"\n", "does not parse"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			brief := phantomBriefAs(t, phantomRegistry(t), c.idLine,
				`{in: "example#5127", covers: partial, note: "first slice"}`)
			withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
				return []deskkit.PRRef{{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"}}, nil
			})
			var err error
			stderr := captureStderr(t, func() {
				_, err = phantomCheck(dispatchOpts{item: "assay--example-partial--15", kit: "worker", brief: brief}, allowedRepo)
			})
			if err == nil || !strings.Contains(err.Error(), "already DELIVERED") {
				t.Fatalf("a --brief file that is not the item's own brief must not relax the check: %v", err)
			}
			if !strings.Contains(stderr, "could-not-check") || !strings.Contains(stderr, c.notice) {
				t.Errorf("the could-not-check NOTICE must name why (%q):\n%s", c.notice, stderr)
			}
		})
	}
}

// phantomRegistry plants only the alias registry under a fresh root and returns the root.
func phantomRegistry(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	plantRegistry(t, root)
	return root
}

// plantRegistry writes the fixture alias registry (example → allowedRepo) and the brief directory.
func plantRegistry(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "docs", "streams", "example-partial"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := "schema: graph-repos-v1\ncell: example\nrepos:\n  example: {cell: example, repo: " + allowedRepo + "}\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
}
