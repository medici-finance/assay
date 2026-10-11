package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// newWorkspace returns the provisioner a parallel sweep installs as
// Harness.Workspace: one throwaway copy of srcRoot per worker, made by
// copyModuleTree so the copy builds exactly as the source tree does.
func newWorkspace(srcRoot string) func(id int) (string, func(), error) {
	return func(id int) (string, func(), error) {
		dir, err := os.MkdirTemp("", fmt.Sprintf("muhar-%d-", id))
		if err != nil {
			return "", nil, err
		}
		cleanup := func() { _ = os.RemoveAll(dir) }
		if err := copyModuleTree(srcRoot, dir); err != nil {
			cleanup()
			return "", nil, err
		}
		return dir, cleanup, nil
	}
}

// copyModuleTree copies src to dst (copyTree) and then re-points every
// filesystem `replace` directive the copy cannot resolve on its own.
//
// Why (#2550). A go.mod `replace X => ../../sibling` is resolved relative to
// the go.mod's own directory. The copy lives in a fresh temp directory, so a
// relative target that escapes the copied tree points at nothing there: every
// suite run in the copy fails at setup, the baseline goes red, and the whole
// sweep is discarded as broken before a single mutation runs. The copy is not
// a faithful copy of the tree unless its module graph resolves the same way.
//
// So, for each go.mod in the copy, a filesystem replacement (no version on
// the right-hand side, relative path) whose target lies OUTSIDE src is
// rewritten to the absolute path of that target in the source tree — the
// directory the source go.mod already resolves to. The replaced module is only
// read there; mutations still resolve against the copy's root, so they never
// touch it. A relative target INSIDE src is left alone: it was copied too, and
// resolving it inside the copy is what keeps a mutation to it isolated. Module
// replacements (a versioned right-hand side) and absolute paths already resolve
// from anywhere and are left alone.
func copyModuleTree(src, dst string) error {
	if err := copyTree(src, dst); err != nil {
		return err
	}
	return relinkLocalReplaces(src, dst)
}

// goModReplace is the slice of `go mod edit -json` output this file reads.
type goModReplace struct {
	Old struct{ Path, Version string }
	New struct{ Path, Version string }
}

// relinkLocalReplaces applies copyModuleTree's rewrite to every go.mod under
// dst that the go command would treat as a module root — it skips the
// directories the go command itself ignores (testdata, and names starting
// with "." or "_"), so a fixture go.mod is copied byte-for-byte.
func relinkLocalReplaces(src, dst string) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	return filepath.WalkDir(dst, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dst && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dst, filepath.Dir(path))
		if err != nil {
			return err
		}
		return relinkGoMod(path, filepath.Join(absSrc, rel), absSrc)
	})
}

// relinkGoMod rewrites the escaping filesystem replacements of the go.mod at
// path (a copy of the go.mod in origDir), then re-reads it to PROVE each
// rewrite landed — the same no-silent-no-op rule the mutations themselves are
// held to. An error here fails workspace provisioning, which the harness
// reports as a broken run, never as a clean sweep.
func relinkGoMod(path, origDir, absSrc string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Contains(raw, []byte("replace")) {
		return nil // no replace directive at all: nothing to resolve
	}
	reps, err := readReplaces(path)
	if err != nil {
		return err
	}
	want := map[string]string{} // old module path, plus "@" and its version when set -> new absolute target
	var args []string
	for _, r := range reps {
		if r.New.Version != "" || filepath.IsAbs(r.New.Path) {
			continue // a module replacement, or a path that resolves from anywhere
		}
		target := filepath.Join(origDir, filepath.FromSlash(r.New.Path))
		if within(absSrc, target) {
			continue // copied alongside; resolves (isolated) inside the copy
		}
		old := r.Old.Path
		if r.Old.Version != "" {
			old += "@" + r.Old.Version
		}
		want[old] = target
		args = append(args, "-replace="+old+"="+target)
	}
	if len(args) == 0 {
		return nil
	}
	cmd := exec.Command("go", append(append([]string{"mod", "edit"}, args...), path)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("re-point relative replace directives in %s: %v: %s", path, err, strings.TrimSpace(string(out)))
	}
	after, err := readReplaces(path)
	if err != nil {
		return err
	}
	for _, r := range after {
		old := r.Old.Path
		if r.Old.Version != "" {
			old += "@" + r.Old.Version
		}
		if target, ok := want[old]; ok {
			if r.New.Path != target {
				return fmt.Errorf("%s: replace %s still points at %q after re-pointing it to %q", path, old, r.New.Path, target)
			}
			delete(want, old)
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("%s: replace directive(s) not re-pointed: %v", path, want)
	}
	return nil
}

// readReplaces parses the replace directives of the go.mod at path with the go
// command's own parser, so no hand-rolled go.mod grammar can drift from it.
func readReplaces(path string) ([]goModReplace, error) {
	out, err := exec.Command("go", "mod", "edit", "-json", path).Output()
	if err != nil {
		var stderr string
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("read replace directives of %s: %v %s", path, err, stderr)
	}
	var mod struct{ Replace []goModReplace }
	if err := json.Unmarshal(out, &mod); err != nil {
		return nil, fmt.Errorf("parse `go mod edit -json` for %s: %w", path, err)
	}
	return mod.Replace, nil
}

// within reports whether target is root itself or lies under it. Both are
// cleaned absolute paths built from the same root, so a lexical check suffices.
func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
