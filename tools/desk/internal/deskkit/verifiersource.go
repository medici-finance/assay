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
	"os/exec"
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
// every path under the home, byte for byte, against it.
//
// Every input to that comparison is the attested commit or the immutable
// binding. A file may differ from its blob only by the checkout conversion, and
// the conversion is rendered from the commit's own .gitattributes files and the
// line-ending settings pinned at dispatch. The attribute source is pinned to the
// attested commit (attr.tree, so a tree the home names, or its index, is never
// read; on a git without attr.tree the worktree files read are themselves
// compared). The home's own attributes (info/attributes refuses; the global and
// system files and the GIT_ATTR_SOURCE environment are switched off) and its
// current conversion config never take part, and a filter driver's output is
// never accepted as attested bytes. Nothing in the replace namespace
// participates either.
//
// Out of scope, stated so it is not overclaimed: the brief's own Evidence
// section (written by the witness runner during the run, bound only by the plan
// digest), and repository configuration that changes how a Verify row's own git
// commands PRESENT content (diff drivers, pagers, aliases). The binding covers
// the bytes on disk, not what a command chooses to print about them.

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

// verifierNoLocalAttributes refuses a home whose repository carries its own
// attributes file. Those attributes are not attested, are shared by every
// worktree of the repository, and change how git converts a checkout, so the
// bytes a Verify row reads would no longer follow from the attested commit.
func verifierNoLocalAttributes(home string) error {
	attrs, err := verifierGit(home, "rev-parse", "--path-format=absolute", "--git-path", "info/attributes")
	if err != nil {
		return err
	}
	if _, err := os.Lstat(attrs); err == nil {
		return Refused("verifier home carries unattested attributes (info/attributes); a Verify row would not read the attested bytes")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Unverifiable("cannot inspect verifier attributes", err)
	}
	return nil
}

// verifierCheckout is the line-ending conversion a home was checked out with.
// It is read once at dispatch, pinned in the immutable binding, and is the only
// conversion config admission renders with afterwards. source is the attested
// commit the attributes are read from; it is set by the reader, never pinned.
type verifierCheckout struct{ autocrlf, eol, source string }

func (c verifierCheckout) String() string { return "autocrlf=" + c.autocrlf + " eol=" + c.eol }

// args pins the conversion and the attribute source for one git call and
// switches off the global attributes file; system attributes are off in
// verifierEnv. Command-line values outrank every config file.
func (c verifierCheckout) args(rest ...string) []string {
	return append([]string{"-c", "core.autocrlf=" + c.autocrlf, "-c", "core.eol=" + c.eol, "-c", "core.attributesFile=" + os.DevNull, "-c", "attr.tree=" + c.source}, rest...)
}

func parseVerifierCheckout(s string) (verifierCheckout, error) {
	a, e, _ := strings.Cut(s, " ")
	c := verifierCheckout{autocrlf: strings.TrimPrefix(a, "autocrlf="), eol: strings.TrimPrefix(e, "eol=")}
	okA := c.autocrlf == "true" || c.autocrlf == "false" || c.autocrlf == "input"
	okE := c.eol == "lf" || c.eol == "crlf" || c.eol == "native"
	if !okA || !okE || c.String() != s {
		return c, Refused("invalid verifier binding: checkout conversion")
	}
	return c, nil
}

// verifierConfig reads one config value; set is false when the key is absent.
func verifierConfig(home string, args ...string) (value string, set bool, err error) {
	out, err := verifierGitBytes(home, append([]string{"config"}, args...)...)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(out)), true, nil
}

// readVerifierCheckout reads the home's effective conversion settings. An
// unrecognised value refuses rather than guessing what the checkout did.
func readVerifierCheckout(home string) (verifierCheckout, error) {
	c := verifierCheckout{autocrlf: "false", eol: "native"}
	if b, set, err := verifierConfig(home, "--type=bool", "--get", "core.autocrlf"); err == nil && set {
		c.autocrlf = b
	} else if err != nil {
		raw, _, rerr := verifierConfig(home, "--get", "core.autocrlf")
		if rerr != nil || !strings.EqualFold(raw, "input") {
			return c, Refused("verifier home has an unrecognised core.autocrlf; cannot pin its checkout conversion")
		}
		c.autocrlf = "input"
	}
	raw, set, err := verifierConfig(home, "--get", "core.eol")
	if err != nil {
		return c, Unverifiable("cannot read verifier checkout conversion", err)
	}
	if set {
		c.eol = strings.ToLower(raw)
	}
	return parseVerifierCheckout(c.String())
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
// is present. Paths in allowed (the brief, and at Evidence time its stream
// index) are checked by their own rules instead.
func verifierSourceClosure(home, commit string, checkout verifierCheckout, allowed map[string]bool) error {
	checkout.source = commit
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
		return verifierSameEntry(home, rel, p, entry, newHash, checkout)
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

// verifierEntryBytes returns the bytes a Verify row reads at p (a link's
// target for a link), refusing a missing path or a type or mode change.
func verifierEntryBytes(rel, p, mode string) ([]byte, error) {
	changed := Refused("source files changed since verifier dispatch: " + rel)
	info, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, Refused("source files changed since verifier dispatch: " + rel + " (missing)")
	}
	if err != nil {
		return nil, Unverifiable("cannot read verifier source file "+rel, err)
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		if mode != "120000" {
			return nil, changed
		}
		target, err := os.Readlink(p)
		if err != nil {
			return nil, Unverifiable("cannot read verifier source link "+rel, err)
		}
		return []byte(filepath.ToSlash(target)), nil
	case info.Mode().IsRegular():
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, Unverifiable("cannot read verifier source file "+rel, err)
		}
		// Windows has no executable bit, and a link may be checked out as a
		// file holding its target; the content comparison still binds.
		want := "100644"
		if info.Mode()&0o111 != 0 {
			want = "100755"
		}
		if runtime.GOOS != "windows" && mode != want {
			return nil, changed
		}
		return data, nil
	}
	return nil, changed
}

func verifierSameEntry(home, rel, p, entry string, newHash func() hash.Hash, checkout verifierCheckout) error {
	mode, object, _ := strings.Cut(entry, " ")
	data, err := verifierEntryBytes(rel, p, mode)
	if err != nil {
		return err
	}
	if verifierBlobID(newHash, data) == object {
		return nil
	}
	if mode == "120000" {
		return Refused("source files changed since verifier dispatch: " + rel)
	}
	expect, err := verifierRender(home, rel, object, checkout)
	if err != nil {
		return err
	}
	if !bytes.Equal(expect, data) {
		return Refused("source files changed since verifier dispatch: " + rel)
	}
	return nil
}

// verifierRender renders the attested blob as a checkout would write it (eol,
// ident, working-tree encoding), with the attributes of the commit's own
// .gitattributes files and the pinned conversion only, and never through a
// filter driver: a driver's output is not the attested bytes, so a filtered
// path that differs from its blob is refused.
func verifierRender(home, rel, object string, checkout verifierCheckout) ([]byte, error) {
	if checkout.source == "" {
		return nil, Unverifiable("cannot render attested source file "+rel, errors.New("no attested attribute source"))
	}
	attr, err := verifierGit(home, checkout.args("check-attr", "-z", "filter", "--", rel)...)
	if err != nil {
		return nil, err
	}
	if f := strings.Split(attr, "\x00"); len(f) < 3 || (f[2] != "unspecified" && f[2] != "unset") {
		return nil, Refused("source files changed since verifier dispatch: " + rel + " (filter driver)")
	}
	expect, err := verifierGitBytes(home, checkout.args("cat-file", "--filters", "--path="+rel, object)...)
	if err != nil {
		return nil, Unverifiable("cannot render attested source file "+rel, err)
	}
	return expect, nil
}

// verifierIndexEdit admits the stream index at Evidence time only as its
// attested bytes, or as those bytes with the attested brief's own row's
// lifecycle cells changed (nn is that row; empty when the brief has no row, and
// then nothing may change). It is the same property a row-scoped landing binds.
func verifierIndexEdit(home, commit, index, nn string, checkout verifierCheckout) error {
	checkout.source = commit
	entry, err := verifierGit(home, "ls-tree", "-z", commit, "--", index)
	if err != nil {
		return err
	}
	if entry == "" {
		return nil // never attested: the closure refuses the path if present
	}
	meta, name, _ := strings.Cut(strings.TrimSuffix(entry, "\x00"), "\t")
	f := strings.Fields(meta)
	if len(f) != 3 || name != index {
		return Unverifiable("cannot read attested stream index", fmt.Errorf("malformed entry %q", entry))
	}
	mode, object := f[0], f[2]
	data, err := verifierEntryBytes(index, filepath.Join(home, filepath.FromSlash(index)), mode)
	if err != nil {
		return err
	}
	newHash, err := verifierObjectHash(home)
	if err != nil {
		return err
	}
	if id := verifierBlobID(newHash, data); id == object {
		return nil
	}
	changed := Refused("source files changed since verifier dispatch: " + index + " (beyond the attested brief's own row lifecycle cells)")
	if mode == "120000" {
		return changed
	}
	rendered, err := verifierRender(home, index, object, checkout)
	if err != nil {
		return err
	}
	if bytes.Equal(rendered, data) {
		return nil
	}
	if nn == "" {
		return changed
	}
	raw, err := verifierGitBytes(home, "cat-file", "blob", object)
	if err != nil {
		return Unverifiable("cannot read attested stream index", err)
	}
	for _, base := range [][]byte{raw, rendered} {
		if rebased, _, _, err := RebaseNamedRows(base, data, []string{nn}); err == nil && bytes.Equal(rebased, data) {
			return nil
		}
	}
	return changed
}
