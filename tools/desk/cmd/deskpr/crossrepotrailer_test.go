package main

// crossrepotrailer_test.go — deskpr's half of the `Cross-repo-brief:` link (medici-finance/assay#2748).
// A PR whose brief is homed in another repo cannot carry a `Brief:` that resolves under --root, so
// create AND edit (both gate on requireTrailer) refused it. The new kind is validated on its own
// grammar and against --root's alias registry, never resolved to a brief file here.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const xrRegistry = `schema: graph-repos-v1
cell: example-cell
self: example-tool
repos:
  example-tool:   {cell: example-cell, repo: medici-finance/assay}
  example-trk:    {cell: example-cell, repo: null, unpublished: true}
`

func xrRoot(t *testing.T, registry string) string {
	t.Helper()
	root := t.TempDir()
	if registry != "" {
		p := filepath.Join(root, "docs", "streams", "graph-repos.yaml")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(registry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// regression: #2748
// TestCrossRepoTrailerGate — a registered, non-self alias with the item-key grammar is accepted with
// no brief file under --root; every near-miss is an exit-5 refusal.
func TestCrossRepoTrailerGate(t *testing.T) {
	root := xrRoot(t, xrRegistry)
	for _, tc := range []struct {
		body, root, want string
	}{
		{"Cross-repo-brief: example-trk:example-stream/05", root, ""},
		{"Cross-repo-brief: example-nope:example-stream/05", root, "not registered"},
		{"Cross-repo-brief: example-tool:example-stream/05", root, "own alias"},
		{"Cross-repo-brief: example-stream/05", root, "<alias>:<stream>/<NN>"},
		{"Cross-repo-brief: example-trk:example-stream/05", xrRoot(t, ""), "no alias registry"},
		{"Cross-repo-brief: example-trk:example-stream/05\nBrief: example-stream/05", root, "exactly one link"},
	} {
		n, err := requireTrailer([]byte("body\n"+tc.body+"\n"), tc.root, tc.root)
		if tc.want == "" {
			if err != nil || n != 0 {
				t.Errorf("%q: got (%d, %v), want (0, nil)", tc.body, n, err)
			}
			continue
		}
		if !deskkit.IsRefused(err) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v, want an exit-5 refusal containing %q", tc.body, err, tc.want)
		}
	}
}

// regression: #2748
// TestCrossRepoTrailerLink — edit's trailer-immutability compare renders the new kind as itself,
// so a Cross-repo-brief: body can never compare equal to (or be re-pointed at) a Brief: one.
func TestCrossRepoTrailerLink(t *testing.T) {
	got, ok := trailerLink([]byte("x\nCross-repo-brief: example-trk:example-stream/05\n"))
	if !ok || got != "Cross-repo-brief: example-trk:example-stream/05" {
		t.Fatalf("trailerLink(Cross-repo-brief) = (%q, %v)", got, ok)
	}
}
