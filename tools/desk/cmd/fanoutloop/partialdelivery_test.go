package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// partialdelivery_test.go — #6751: a MERGED PR the brief's own `delivery:` list declares
// `covers: partial` is work in progress, never the brief's delivery. statusgen already counts it
// that way and offers the row; the planner must offer it too, instead of listing it
// LANDED-UNRECONCILED. Stream slugs, aliases and PR numbers are synthetic.

const partialRegistry = `schema: graph-repos-v1
cell: example
repos:
  example: {cell: example, repo: example-org/example-repo}
  other:   {cell: example, repo: example-org/other-repo}
  hidden:  {cell: example, repo: null, unpublished: true}
`

// partialBrief is a brief-v2 frontmatter in the live shape: a flow-mapping `delivery:` list.
func partialBrief(delivery ...string) string {
	var b strings.Builder
	b.WriteString("---\nbrief: example:example:example-partial:15\ntitle: Partial fixture\n")
	b.WriteString("effort: L\ngate: model\n")
	b.WriteString("risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n")
	if len(delivery) > 0 {
		b.WriteString("delivery:\n")
		for _, d := range delivery {
			b.WriteString("  - " + d + "\n")
		}
	}
	b.WriteString("schema: brief-v2\nversion: 1\n---\n\n# Partial fixture\n")
	return b.String()
}

// partialRoot writes the alias registry and the example-partial/15 brief under a temp root.
func partialRoot(t *testing.T, brief string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "example-partial")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "graph-repos.yaml"), []byte(partialRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-15-x.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func partialPlan(t *testing.T, brief string, prs []deskkit.PRRef) string {
	t.Helper()
	setupDeskHome(t)
	withPlanTransports(t, prs, nil)
	f := planWithRows([]BoardRow{briefRow("example-partial", "15", "L", "", "model", false)})
	f.Root = partialRoot(t, brief)
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	return out.String()
}

const (
	partialA = `{in: "example#5127", covers: partial, note: "the judgement half only; the recorder remains owed"}`
	partialB = `{in: "example#6001", covers: partial, note: "the recorder; rows 14 and 16 remain owed"}`
)

// TestPlan_PartialMergeDispatches is the fail-first proof of #6751: the only PRs naming the queued
// brief are two MERGED PRs its `delivery:` list declares `covers: partial` (the shape a brief takes
// when its work lands in slices). Before the fix the row was listed LANDED-UNRECONCILED and never
// offered; it must be offered for dispatch, as statusgen offers it.
//
// regression: #6751
func TestPlan_PartialMergeDispatches(t *testing.T) {
	s := partialPlan(t, partialBrief(partialA, partialB), []deskkit.PRRef{
		{Number: 6001, State: "MERGED", Body: "Second slice.\n\nBrief: example-partial/15"},
		{Number: 5127, State: "MERGED", Body: "First slice.\n\nBrief: example-partial/15"},
	})
	if strings.Contains(s, "LANDED-UNRECONCILED") {
		t.Errorf("a brief whose merged PRs are all declared covers: partial was listed landed-unreconciled (#6751):\n%s", s)
	}
	if !strings.Contains(s, "=== DISPATCH example-partial/15") {
		t.Errorf("a brief with only partial merged deliveries must be offered for dispatch (#6751):\n%s", s)
	}
}

// TestPlan_PartialThenFullLands: a later `full` entry for the same PR supersedes the partial (the
// list is append-only history), so the PR is the delivery and the row stays landed-unreconciled.
func TestPlan_PartialThenFullLands(t *testing.T) {
	full := `{in: "example#5127", covers: full, note: "the rest landed"}`
	s := partialPlan(t, partialBrief(partialA, full), []deskkit.PRRef{
		{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"},
	})
	if strings.Contains(s, "=== DISPATCH example-partial/15") || !strings.Contains(s, "example-partial/15 — merged PR #5127") {
		t.Errorf("a PR declared partial and then full is the delivery; the row must stay landed:\n%s", s)
	}
}

// TestPlan_UndeclaredMergeLands: a partial claim never hides an undeclared merged PR behind it.
func TestPlan_UndeclaredMergeLands(t *testing.T) {
	s := partialPlan(t, partialBrief(partialA), []deskkit.PRRef{
		{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"},
		{Number: 7000, State: "MERGED", Body: "Brief: example-partial/15"},
	})
	if strings.Contains(s, "=== DISPATCH example-partial/15") || !strings.Contains(s, "example-partial/15 — merged PR #7000") {
		t.Errorf("an undeclared merged PR must keep the row landed-unreconciled:\n%s", s)
	}
}

// TestPlan_PartialPlusOpenResumes: with the partial set aside, an OPEN PR still routes to resume.
func TestPlan_PartialPlusOpenResumes(t *testing.T) {
	s := partialPlan(t, partialBrief(partialA), []deskkit.PRRef{
		{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"},
		{Number: 7100, State: "OPEN", Body: "Brief: example-partial/15"},
	})
	if !strings.Contains(s, "=== DISPATCH resume:pr-7100") || strings.Contains(s, "LANDED-UNRECONCILED") {
		t.Errorf("an OPEN PR behind a partial merge must route to resume:\n%s", s)
	}
}

// TestPlan_PartialOtherRepoLands: a partial claim naming ANOTHER repo's PR of the same number says
// nothing about this repo's PR, so this repo's merged PR still delivers the brief.
func TestPlan_PartialOtherRepoLands(t *testing.T) {
	other := `{in: "other#5127", covers: partial, note: "a sibling slice"}`
	s := partialPlan(t, partialBrief(other), []deskkit.PRRef{
		{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"},
	})
	if strings.Contains(s, "=== DISPATCH example-partial/15") {
		t.Errorf("a partial claim on another repo must not set this repo's merged PR aside:\n%s", s)
	}
}

// TestPlan_PartialUnresolvedLands: a partial claim whose alias cannot be resolved (withheld here)
// is could-not-check. statusgen derives the brief unknown; the planner keeps the conservative
// answer it had before the fix — the merged PR represents the brief and the row is not offered.
func TestPlan_PartialUnresolvedLands(t *testing.T) {
	hidden := `{in: "hidden#5127", covers: partial, note: "a withheld slice"}`
	s := partialPlan(t, partialBrief(hidden), []deskkit.PRRef{
		{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"},
	})
	if strings.Contains(s, "=== DISPATCH example-partial/15") {
		t.Errorf("an unresolvable partial claim must not be guessed into a dispatch:\n%s", s)
	}
}

// TestPlan_PartialFileTransport: the same partial set-aside applies on the production path,
// where the authoring exemption reads each candidate PR's changed files first.
func TestPlan_PartialFileTransport(t *testing.T) {
	setupDeskHome(t)
	code := []deskkit.ChangedFile{{Filename: "tools/x/main.go", Status: "modified"}}
	withPlanTransports(t,
		[]deskkit.PRRef{{Number: 5127, State: "MERGED", Body: "Brief: example-partial/15"}},
		planFiles(t, map[int][]deskkit.ChangedFile{5127: code}))
	f := planWithRows([]BoardRow{briefRow("example-partial", "15", "L", "", "model", false)})
	f.Root = partialRoot(t, partialBrief(partialA))
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	if s := out.String(); !strings.Contains(s, "=== DISPATCH example-partial/15") {
		t.Errorf("a partial merged delivery must be set aside on the file-transport path too (#6751):\n%s", s)
	}
}
