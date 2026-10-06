package deskkit

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// verifiersource.go: the verifier home's source closure.
//
// THE CLASS. Admission must bind the dispatched commit to the bytes a Verify row
// actually reads. Asking git whether the worktree differs from the commit does
// not establish that: index flags (assume-unchanged, skip-worktree), replacement
// objects and clean filters each change git's answer without changing the files.
// So admission reads the commit's tree with replacements disabled and compares
// every path under the home, byte for byte, against it. Nothing in the index,
// the replace namespace or a filter driver participates in the comparison.

// verifierNoReplacements refuses a home whose repository carries replacement
// objects or grafts. Admission itself ignores them, but a Verify row's own git
// reads would not, so the attested commit would not be what the row inspects.
func verifierNoReplacements(home string) error {
	refs, err := verifierGit(home, "for-each-ref", "--format=%(refname)", "refs/replace/")
	if err != nil {
		return err
	}
	if refs != "" {
		return Refused("verifier home carries replacement objects (refs/replace/); a Verify row would not read the attested commit")
	}
	grafts, err := verifierGit(home, "rev-parse", "--path-format=absolute", "--git-path", "info/grafts")
	if err != nil {
		return err
	}
	if _, err := os.Lstat(grafts); err == nil {
		return Refused("verifier home carries replacement objects (info/grafts); a Verify row would not read the attested commit")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Unverifiable("cannot inspect verifier grafts", err)
	}
	return nil
}

// verifierTree lists commit's full tree as path -> "<mode> <object>", read with
// replacement objects disabled.
func verifierTree(home, commit string) (map[string]string, error) {
	out, err := verifierGit(home, "ls-tree", "-r", "--full-tree", "-z", commit)
	if err != nil {
		return nil, err
	}
	tree := map[string]string{}
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		meta, name, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 {
			return nil, Unverifiable("cannot read attested source tree", fmt.Errorf("malformed entry %q", rec))
		}
		tree[name] = f[0] + " " + f[2]
	}
	return tree, nil
}

func verifierObjectHash(home string) (func() hash.Hash, error) {
	format, err := verifierGit(home, "rev-parse", "--show-object-format")
	if err != nil {
		return nil, err
	}
	switch format {
	case "sha1":
		return sha1.New, nil
	case "sha256":
		return sha256.New, nil
	}
	return nil, Unverifiable("cannot establish verifier source", fmt.Errorf("unknown object format %q", format))
}

func verifierBlobID(newHash func() hash.Hash, b []byte) string {
	h := newHash()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// verifierSourceClosure refuses unless every file under home is a path of
// commit's tree holding that path's attested bytes and mode, and every tree path
// is present. Paths in allowed (the brief and its stream index) are checked by
// their own rules instead.
func verifierSourceClosure(home, commit string, allowed map[string]bool) error {
	tree, err := verifierTree(home, commit)
	if err != nil {
		return err
	}
	newHash, err := verifierObjectHash(home)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	walkErr := filepath.WalkDir(home, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(home, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entry, tracked := tree[rel]
		if d.IsDir() {
			if tracked {
				// A gitlink: its content is another repository's commit, which
				// this closure does not attest.
				return Refused("verifier home has a submodule, whose content is not attested: " + rel)
			}
			return nil
		}
		if !tracked {
			return Refused("unattested worktree file: " + rel)
		}
		seen[rel] = true
		if allowed[rel] {
			return nil
		}
		return verifierSameEntry(home, rel, p, entry, newHash)
	})
	if walkErr != nil {
		return walkErr
	}
	var missing []string
	for name := range tree {
		if !seen[name] && !allowed[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return Refused("source files changed since verifier dispatch: " + missing[0] + " (missing)")
	}
	return nil
}

func verifierSameEntry(home, rel, p, entry string, newHash func() hash.Hash) error {
	mode, object, _ := strings.Cut(entry, " ")
	changed := Refused("source files changed since verifier dispatch: " + rel)
	info, err := os.Lstat(p)
	if err != nil {
		return Unverifiable("cannot read verifier source file "+rel, err)
	}
	var data []byte
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		if mode != "120000" {
			return changed
		}
		target, err := os.Readlink(p)
		if err != nil {
			return Unverifiable("cannot read verifier source link "+rel, err)
		}
		data = []byte(filepath.ToSlash(target))
	case info.Mode().IsRegular():
		data, err = os.ReadFile(p)
		if err != nil {
			return Unverifiable("cannot read verifier source file "+rel, err)
		}
		// Windows has no executable bit, and a link may be checked out as a
		// file holding its target; the content comparison below still binds.
		want := "100644"
		if info.Mode()&0o111 != 0 {
			want = "100755"
		}
		if runtime.GOOS != "windows" && mode != want {
			return changed
		}
	default:
		return changed
	}
	if verifierBlobID(newHash, data) == object {
		return nil
	}
	if mode == "120000" {
		return changed
	}
	// The checkout may legitimately convert the blob (eol, ident, working-tree
	// encoding). Render the attested blob through that conversion and compare,
	// but never through a filter driver: a driver's output is not the attested
	// bytes, so a filtered path that differs from its blob is refused.
	attr, err := verifierGit(home, "check-attr", "-z", "filter", "--", rel)
	if err != nil {
		return err
	}
	if f := strings.Split(attr, "\x00"); len(f) < 3 || (f[2] != "unspecified" && f[2] != "unset") {
		return Refused("source files changed since verifier dispatch: " + rel + " (filter driver)")
	}
	expect, err := verifierGitBytes(home, "cat-file", "--filters", "--path="+rel, object)
	if err != nil {
		return Unverifiable("cannot render attested source file "+rel, err)
	}
	if !bytes.Equal(expect, data) {
		return changed
	}
	return nil
}
