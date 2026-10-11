package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// manyRowBoard lays a synthetic board of nStreams streams × nBriefs briefs
// under a fresh root and returns it with the reconcile cells a backfill would
// derive for it: the first nWitnessed rows of every stream read todo on the
// board and carry a merged-PR witness, so each is a candidate move.
func manyRowBoard(tb testing.TB, nStreams, nBriefs, nWitnessed int) (string, []BriefCell) {
	tb.Helper()
	root := tb.TempDir()
	var cells []BriefCell
	for s := 0; s < nStreams; s++ {
		name := fmt.Sprintf("mr%02d", s)
		dir := filepath.Join(root, "docs", "streams", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatal(err)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "---\nstream: %s\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# %s\n\n## Briefs\n\n", name, name)
		b.WriteString("| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n")
		for i := 1; i <= nBriefs; i++ {
			num := fmt.Sprintf("%02d", i)
			status := "implemented"
			if i <= nWitnessed {
				status = "todo"
				cells = append(cells, BriefCell{ID: name + "/" + num, Cell: "implemented", Source: "pr", Witness: fmt.Sprintf("PR #%d", s*100+i)})
			} else if i > nBriefs-2 {
				status = "todo"
			}
			file := fmt.Sprintf("brief-%s-b.md", num)
			fmt.Fprintf(&b, "| %s | [b%s](./%s) | 0 | S | %s | — | — |\n", num, num, file, status)
			deps := "[]"
			if s > 0 {
				deps = fmt.Sprintf("[mr%02d/%s]", s-1, num)
			}
			brief := fmt.Sprintf("---\nschema: brief-v1\nbrief: %s/%s\ntitle: synthetic %s/%s\nwave: 0\ndepends: %s\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nauthored: 2026-09-10 by fixture\nsources: [\"fixture\"]\n---\n\n# Brief %s\n\n## Verify\n\n| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | rc 0 |\n", name, num, name, num, deps, num)
			if err := os.WriteFile(filepath.Join(dir, file), []byte(brief), 0o644); err != nil {
				tb.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(b.String()), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return root, cells
}

func BenchmarkReconcilePlanManyRow(b *testing.B) {
	root, cells := manyRowBoard(b, 50, 10, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, held, err := planReconcileWrites(root, cells)
		if err != nil {
			b.Fatal(err)
		}
		if i == 0 {
			b.Logf("rows=%d held=%d", len(rows), len(held))
		}
	}
}

// regression: #2615
func TestHoldPassesBoundedByRows(t *testing.T) {
	// One witnessed row per stream, then four: the hold's whole-board work must
	// not grow with the number of rows it moves.
	for _, perStream := range []int{1, 4} {
		root, cells := manyRowBoard(t, 30, 8, perStream)
		var st holdStats
		rows, held, err := reconcileWritesWith(root, cells, false, holdOptions{stats: &st})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 30*perStream || len(held) != 0 {
			t.Fatalf("fixture drift: want %d rows and no holds, got rows=%d held=%+v", 30*perStream, len(rows), held)
		}
		if bound := len(statusKeyedLintRules); st.FullRuleEvals > bound {
			t.Fatalf("%d moved rows took %d whole-board rule evaluations; want at most %d (one baseline pass of the %d rules) — the hold re-ran the board once per row", len(rows), st.FullRuleEvals, bound, len(statusKeyedLintRules))
		}
	}
}
