package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// originresolution_test.go — #1623: deskmerge's origin identity checks decide on what GIT
// resolves for the fetch and the push, not on go-git's read of the repository config file.
//
// Each case plants a real config shape the file read cannot see (a global-scope insteadOf, a
// multi-valued url list, a pushurl) and points it at a MIRROR of the fixture's remote filed under
// another project's name. The mirror carries every ref the real remote does, so on the unfixed
// code the run does not fail for want of a ref: it answers, or merges and pushes, against the
// wrong project — which is the defect, observed rather than inferred.

const otherRepo = "someone/else"

// mirrorAsOther clones w.remote (all refs, refs/pull/* included) to a bare repo whose path names
// otherRepo, and returns that path.
func (w *world) mirrorAsOther(t *testing.T) string {
	t.Helper()
	other := filepath.Join(w.dir, filepath.FromSlash(otherRepo)+".git")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, w.dir, "clone", "-q", "--mirror", w.remote, other)
	return other
}

// withGlobalGitConfig writes body as $HOME/.gitconfig for this test. HOME is the one config
// location the scrubbed git environment (internal/gitexec drops every GIT_* variable) still
// honours, so a global-scope rule has to be planted there to reach deskmerge's own git calls.
func withGlobalGitConfig(t *testing.T, body string) {
	t.Helper()
	p := filepath.Join(os.Getenv("HOME"), ".gitconfig")
	if _, err := os.Stat(p); err == nil {
		t.Fatalf("refusing to overwrite an existing %s", p)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(p) })
}

// FETCH side, global scope. The root's own config names the repo; a global insteadOf rewrites it
// to the other project. Unfixed: the file read passes and `check` answers from the wrong
// project's refs (exit 0). Fixed: refused (exit 5), naming the URL git resolved.
func TestResolveRepoRootGatesOnGitsFetchURL(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	other := w.mirrorAsOther(t)
	withGlobalGitConfig(t, "[url \""+other+"\"]\n\tinsteadOf = "+w.remote+"\n")
	w.install(t, defaultPR(), false)

	code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 when git resolves origin to another project, got %d\n%s", code, out)
	}
	if !strings.Contains(out, otherRepo) {
		t.Fatalf("the refusal must name the URL git resolved:\n%s", out)
	}
	for _, c := range *w.gitAll {
		if len(c) > 1 && c[1] == "fetch" {
			t.Fatalf("a fetch ran before the refusal: %v", c)
		}
	}
}

// FETCH side, multi-valued url list: no single URL names the project. Unfixed: go-git's first
// value was checked and the run went on. Fixed: refused.
func TestResolveRepoRootRefusesMultiOrigin(t *testing.T) {
	withScratchTemp(t)
	w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
	git(t, w.root, "config", "--add", "remote.origin.url", w.mirrorAsOther(t))
	w.install(t, defaultPR(), false)

	code, out := cli(verbCheck, "-R", testRepo, "7", "--repo-root", w.root)
	if code != deskkit.ExitRefused {
		t.Fatalf("want exit 5 for a multi-valued origin url list, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "2 fetch URLs") {
		t.Fatalf("the refusal must say the list is multi-valued:\n%s", out)
	}
}

// PUSH side. The fetch URL names the repo; remote.origin.pushurl sends the push to another
// project. Unfixed: deskmerge merged and PUSHED to the other project (exit 0). Fixed: refused
// before the budget and the commit, in the dry run as in the real run, and nothing lands on
// either remote.
func TestPushDestinationMustNameTheRepo(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pushurl func(w *world, other string) []string
		want    string
	}{
		{"a pushurl naming another project", func(w *world, other string) []string { return []string{other} },
			"does not name " + testRepo},
		{"a multi-valued pushurl (git pushes to every value)", func(w *world, other string) []string {
			return []string{w.remote, other}
		}, "2 destinations"},
	} {
		for _, dry := range []bool{false, true} {
			name := tc.name
			if dry {
				name += " (dry run)"
			}
			t.Run(name, func(t *testing.T) {
				withScratchTemp(t)
				w := newWorld(t, map[string]string{"pr.txt": "a\n"}, map[string]string{"main.txt": "b\n"})
				other := w.mirrorAsOther(t)
				otherBefore := git(t, w.dir, "-C", other, "rev-parse", "refs/heads/pr-branch")
				for _, u := range tc.pushurl(w, other) {
					git(t, w.root, "config", "--add", "remote.origin.pushurl", u)
				}
				w.install(t, defaultPR(), true)
				rul := w.rulingsFile(t, signOffURL)

				args := []string{verbMerge, "-R", testRepo, "7", "--repo-root", w.root, "--rulings", rul}
				if dry {
					args = append(args, "--dry-run")
				}
				code, out := cli(args...)
				if code != deskkit.ExitRefused {
					t.Fatalf("want exit 5, got %d\n%s", code, out)
				}
				if !strings.Contains(out, tc.want) {
					t.Fatalf("refusal does not say %q:\n%s", tc.want, out)
				}
				w.assertNoPush(t)
				if got := git(t, w.dir, "-C", other, "rev-parse", "refs/heads/pr-branch"); got != otherBefore {
					t.Fatalf("the OTHER project's branch moved (%s -> %s)", otherBefore, got)
				}
			})
		}
	}
}
