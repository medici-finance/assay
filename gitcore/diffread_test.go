package gitcore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #2395
func TestDiffReadGaps(t *testing.T) {
	for _, mode := range []string{"unrelated-missing-tree", "changed-missing-tree", "control-tree-name"} {
		t.Run(mode, func(t *testing.T) {
			f := gittest.NewFixture(t)
			dir := "sub"
			if mode == "control-tree-name" {
				dir = "sub\tname"
			}
			if err := os.MkdirAll(filepath.Join(f.Dir, dir), 0755); err != nil {
				t.Fatal(err)
			}
			f.CommitFile(t, dir+"/f", "first\n", "base subtree")
			before := mustGitOutput(t, f, "rev-parse", "HEAD")
			rel := "seed.txt"
			if mode != "unrelated-missing-tree" {
				rel = dir + "/f"
			}
			f.CommitFile(t, rel, "changed\n", "change")
			after := mustGitOutput(t, f, "rev-parse", "HEAD")
			if mode != "control-tree-name" {
				id := mustGitOutput(t, f, "rev-parse", before+":"+dir)
				if err := os.Remove(filepath.Join(f.Dir, ".git", "objects", id[:2], id[2:])); err != nil {
					t.Fatal(err)
				}
			}
			native, nativeErr := exec.Command("git", "-C", f.Dir, "diff", "--name-only", "--no-renames", "-z", before, after, "--").Output()
			r, err := Open(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			a, err := r.treeAt(before)
			if err != nil {
				t.Fatal(err)
			}
			b, err := r.treeAt(after)
			if err != nil {
				t.Fatal(err)
			}
			_, upstreamErr := object.DiffTreeWithOptions(context.Background(), a, b, &object.DiffTreeOptions{DetectRenames: false})
			t.Logf("native error=%v; direct pinned DiffTree error=%v", nativeErr, upstreamErr)
			switch mode {
			case "unrelated-missing-tree":
				if nativeErr != nil || upstreamErr != nil {
					t.Fatal("unchanged-subtree positive control changed")
				}
			case "changed-missing-tree":
				if nativeErr == nil || upstreamErr != nil {
					t.Fatal("selected missing-subtree success gap no longer demonstrated")
				}
			case "control-tree-name":
				if nativeErr != nil || upstreamErr == nil {
					t.Fatal("selected control-name failure gap no longer demonstrated")
				}
			}
			t.Logf("native changed names: %q", native)

		})
	}
}
