package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A complete, offline queue: all tracking and source roots exist and both are
// at fetched main. Legacy, same-repository and cross-repository rows coexist.
// Setup is outside the timed benchmark. No preflight, credentials or forge calls.
func associationQueueFixture(t testing.TB, count int) *VerifyLoop {
	t.Helper()
	associationRoster(t)
	source := associationRepo(t, "example-org/agents", map[string]string{"source.txt": "source\n"})
	files := map[string]string{
		"docs/streams/graph-repos.yaml": "schema: graph-repos-v1\nself: tracker\nrepos:\n  tracker:\n    repo: example-org/tracker\n  source:\n    repo: example-org/agents\n",
	}
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n"
	for i := 1; i <= count; i++ {
		num := fmt.Sprintf("%02d", i)
		table += fmt.Sprintf("| %s | fixture | 0 | S | implemented | — | — |\n", num)
		declaration := ""
		switch i % 3 {
		case 1:
			declaration = "deliverable_repo: source\n"
		case 2:
			declaration = "deliverable_repo: tracker\n"
		}
		files["docs/streams/sample/brief-"+num+"-fixture.md"] = "---\n" + declaration + "gate: model\n---\n# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | succeeds |\n\n## Evidence\n"
	}
	files["docs/streams/sample/README.md"] = table
	tracking := associationRepo(t, "example-org/tracker", files)
	t.Setenv(deskkit.RootsEnv, "example-org/agents="+source+",example-org/tracker="+tracking)
	return &VerifyLoop{Root: tracking, RootRepo: "example-org/tracker", TargetSHA: associationGit(t, tracking, "rev-parse", "HEAD")}
}

func BenchmarkAssociationQueue(b *testing.B) {
	for _, n := range []int{12, 96} {
		b.Run(fmt.Sprintf("briefs-%d", n), func(b *testing.B) {
			v := associationQueueFixture(b, n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				items, err := v.SelectQueue()
				if err != nil || len(items) != n {
					b.Fatalf("queue: %d %v", len(items), err)
				}
				for _, it := range items {
					if it.Payload["could_not_check"] != "" {
						b.Fatal(it.Payload["could_not_check"])
					}
				}
			}
		})
	}
}

// Used by the pass tests to change an immutable input through a new commit.
func associationCommitFile(t testing.TB, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	associationGit(t, root, "add", name)
	associationGit(t, root, "commit", "-qm", "change input")
	associationGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
}
