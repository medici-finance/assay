package gitcore

import (
	"bytes"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #2395
func TestTypedHeaderParity(t *testing.T) {
	f := gittest.NewFixture(t)
	tree := mustGitOutput(t, f, "rev-parse", "HEAD^{tree}")
	parent := mustGitOutput(t, f, "rev-parse", "HEAD")
	sig := "author A <a@example.invalid> 1 +0000\ncommitter C <c@example.invalid> 1 +0000\n"
	headers := map[string]string{
		"ordinary":         "tree " + tree + "\nparent " + parent + "\n" + sig,
		"short-tree":       "tree ab\n" + sig,
		"invalid-tree":     "tree z" + tree[1:] + "\n" + sig,
		"long-tree":        "tree " + tree + "00\n" + sig,
		"short-parent":     "tree " + tree + "\nparent ab\n" + sig,
		"invalid-parent":   "tree " + tree + "\nparent bad-parent\n" + sig,
		"long-parent":      "tree " + tree + "\nparent " + parent + "00\n" + sig,
		"late-parent":      "tree " + tree + "\n" + sig + "parent " + parent + "\n",
		"duplicate-author": "tree " + tree + "\nparent " + parent + "\nauthor A <a@example.invalid> 1 +0000\n" + sig,
		"duplicate-tree":   "tree " + tree + "\nparent " + parent + "\ntree " + tree + "\n" + sig,
		"no-identities":    "tree " + tree + "\nparent " + parent + "\n",
		"crlf-tree":        "tree " + tree + "\r\nparent " + parent + "\n" + sig,
	}
	for name, head := range headers {
		t.Run(name, func(t *testing.T) {
			raw := []byte(head + "\nsubject\n")
			cmd := exec.Command("git", "-C", f.Dir, "hash-object", "-t", "commit", "--literally", "-w", "--stdin")
			cmd.Stdin = bytes.NewReader(raw)
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			id := strings.TrimSpace(string(out))
			r, err := Open(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := r.RawCommit(id); err != nil || !bytes.Equal(got, raw) {
				t.Fatalf("raw-byte path changed: %q %v", got, err)
			}
			native, nativeErr := exec.Command("git", "--no-replace-objects", "-C", f.Dir, "show", "-s", "--format=%P", id).Output()
			got, gotErr := r.ParentHashes(id)
			if (nativeErr == nil) != (gotErr == nil) || (nativeErr == nil && !reflect.DeepEqual(got, strings.Fields(string(native)))) {
				t.Errorf("parents=(%v,%v), native=(%q,%v)", got, gotErr, native, nativeErr)
			}
			native, nativeErr = exec.Command("git", "--no-replace-objects", "-C", f.Dir, "show", id+":seed.txt").Output()
			blobText, blobErr := r.FileAt(id, "seed.txt")
			blob := []byte(blobText)
			gap := name == "invalid-parent" || name == "short-parent" || name == "long-parent" || name == "long-tree" || name == "crlf-tree"
			if gap {
				if nativeErr != nil || blobErr == nil {
					t.Fatalf("retained generic treeish gap not demonstrated: native %v, FileAt %v", nativeErr, blobErr)
				}
				t.Logf("retained native generic blob: native succeeds; selected FileAt errors %v", blobErr)
			} else if (nativeErr == nil) != (blobErr == nil) || (nativeErr == nil && !bytes.Equal(blob, native)) {
				t.Errorf("blob control=(%q,%v), native=(%q,%v)", blob, blobErr, native, nativeErr)
			}

			native, nativeErr = exec.Command("git", "--no-replace-objects", "-C", f.Dir, "ls-tree", "-r", "-z", "--name-only", "--full-tree", id).Output()
			entries, entryErr := r.TreeEntries(id, true, false)
			var names []string
			for _, e := range entries {
				names = append(names, e.Path)
			}
			var wantNames []string
			if len(native) > 0 {
				wantNames = strings.Split(strings.TrimSuffix(string(native), "\x00"), "\x00")
			}
			if (nativeErr == nil) != (entryErr == nil) || (nativeErr == nil && !reflect.DeepEqual(names, wantNames)) {
				t.Errorf("tree names=(%q,%v), native=(%q,%v)", names, entryErr, wantNames, nativeErr)
			}
			native, nativeErr = exec.Command("git", "--no-replace-objects", "-C", f.Dir, "rev-parse", "--verify", id+"^{commit}").Output()
			resolved, resolveErr := r.Resolve(id)
			if (nativeErr == nil) != (resolveErr == nil) || (nativeErr == nil && resolved.String() != strings.TrimSpace(string(native))) {
				t.Errorf("resolve=(%q,%v), native=(%q,%v)", resolved, resolveErr, native, nativeErr)
			}
			nativeErr = exec.Command("git", "--no-replace-objects", "-C", f.Dir, "merge-base", "--is-ancestor", parent, id).Run()
			nativeState := "unknown"
			if nativeErr == nil {
				nativeState = "yes"
			} else if ee, ok := nativeErr.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				nativeState = "no"
			}
			related, ancestorErr := r.IsAncestor(parent, id)
			candidateState := "unknown"
			if ancestorErr == nil {
				if related {
					candidateState = "yes"
				} else {
					candidateState = "no"
				}
			}
			if candidateState != nativeState {
				t.Errorf("ancestry state=%s (%v), native=%s (%v)", candidateState, ancestorErr, nativeState, nativeErr)
			}

		})
	}
}
