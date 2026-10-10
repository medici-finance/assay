package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// homeresolve.go — the ONE place cellctl turns the operator's environment into a home
// directory and the config / harness directories derived from it. Every resolver takes the
// target goos explicitly, so the Windows rules are table-tested on any host
// (TestEnvResolution); production callers pass runtime.GOOS. TestHomeReadClassGuard keeps
// every other non-test file from reading HOME or USERPROFILE for a path of its own.

// operatorHomeFor resolves the operator's home directory for goos.
//
// The primary follows os.UserHomeDir — the resolution every desk tool cellctl launches uses
// to find the roster and the App credentials — so cellctl links and checks the same config
// home those tools then read: %USERPROFILE% on windows, $HOME elsewhere. On windows only,
// $HOME is the fallback when %USERPROFILE% is unset — the precedence the Codex launch already
// used, so a HOME-only Git Bash shell keeps working. Elsewhere there is NO fallback:
// os.UserHomeDir reads $HOME alone there, so a home resolved from %USERPROFILE% would point
// cellctl at a config home every tool it launches fails to find.
//
// An unset home REFUSES, and so does a value that is not an absolute local path or that names
// the filesystem root. Every path derived from such a home is either relative (resolved
// against whatever directory cellctl happened to run in), rooted at the filesystem root, or on
// a network share, and a config home resolved that way would link, check or launch against a
// directory that is not the operator's. The refusal names the variable, never its value.
func operatorHomeFor(goos string, e *Env) (string, error) {
	type candidate struct{ name, v string }
	cands := []candidate{{"HOME", e.Get("HOME")}}
	if goos == "windows" {
		cands = []candidate{{"USERPROFILE", e.Get("USERPROFILE")}, {"HOME", e.Get("HOME")}}
	}
	for _, c := range cands {
		if c.v == "" {
			continue
		}
		if err := homePathCheck(goos, c.v); err != nil {
			return "", fmt.Errorf("cannot resolve the operator home: %s %v "+
				"(refusing to derive a config path from it)", c.name, err)
		}
		return c.v, nil
	}
	unset := "HOME is not set"
	if goos == "windows" {
		unset = "neither USERPROFILE nor HOME is set"
	}
	return "", fmt.Errorf("cannot resolve the operator home: %s "+
		"(refusing to derive a config path from an empty home)", unset)
}

// homePathCheck is the shape a resolved home must have: cellPathCheck's absolute, local path
// (no UNC or device spelling), and not the bare filesystem or drive root. The error text never
// carries the value.
func homePathCheck(goos, v string) error {
	if err := cellPathCheck(goos, v); err != nil {
		return fmt.Errorf("is not usable: %v", err)
	}
	q := v
	if goos == "windows" {
		q = strings.ReplaceAll(v, `\`, "/")
	}
	if c := path.Clean(q); c == "/" || (goos == "windows" && len(c) == 2 && c[1] == ':') {
		return fmt.Errorf("names the filesystem root")
	}
	return nil
}

// configHomeFor is the operator's assay config home: $ASSAY_CONFIG_HOME, else
// <home>/.config/assay. The override needs no home at all.
func configHomeFor(goos string, e *Env) (string, error) {
	if v := e.Get("ASSAY_CONFIG_HOME"); v != "" {
		return v, nil
	}
	return underHome(goos, e, ".config", "assay")
}

// operatorConfigKey is launch context, not the active roster override. Only
// cellctl reads it; desk tools continue to use ASSAY_CONFIG_HOME. A nested launch
// must retain this independently captured target instead of capturing its alias.
const operatorConfigKey = "CELLCTL_OPERATOR_CONFIG_HOME"

func operatorConfigHomeFor(goos string, e *Env) (string, error) {
	if e.IsSet(operatorConfigKey) {
		v := e.Get(operatorConfigKey)
		if err := homePathCheck(goos, v); err != nil {
			return "", fmt.Errorf("%s %v", operatorConfigKey, err)
		}
		return v, nil
	}
	return configHomeFor(goos, e)
}

// Capture before HOME/ASSAY_CONFIG_HOME change. Resolve existing aliases so the
// stored expectation is a resource path independent of the cell's config link.
// A missing config stays a missing path: launch composition does not provision
// it, and check reports it unavailable. Nested context is never recaptured.
//
// With no recorded context, a candidate that reaches the cell's own config link
// (cellConfig) or lies inside the cell home is NOT an operator resource: the
// composing environment is already cell-scoped, and resolving through the link
// would record wherever the link points as the expectation. Capture then returns
// "" and the context stays unset, so the house check falls back to the cell link
// and refuses the self-comparison.
func captureOperatorConfig(e *Env, cellHome, cellConfig string) (string, error) {
	p, err := operatorConfigHomeFor(runtime.GOOS, e)
	if err != nil || e.IsSet(operatorConfigKey) {
		return p, err
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if cellOwnedConfig(p, cellHome, cellConfig) {
		return "", nil
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot resolve operator config: %w", err)
	}
	return p, nil
}

// cellOwnedConfig reports whether the path p is cell-owned: it reaches
// a cell's config link, or it lies inside a cell home. The cells are this one
// and every sibling under the same cells root (<root>/<name>/home). p is walked
// one path element at a time, the way the OS resolves it (resolvedPrefix), and
// every node on the way is compared with each config link by identity. A
// spelling therefore cannot hide the link: an alias, a relative alias, a target
// ending in a separator or a dot element, or a ".." after a link. Containment is
// node identity of an ancestor, not a string prefix, so letter case on a
// case-insensitive filesystem is irrelevant. A candidate that does not exist yet
// is judged by its existing prefix. A walk that cannot finish (the link budget
// is spent, or an element cannot be read) counts as cell-owned: no independent
// expectation is recorded, and the house check refuses.
func cellOwnedConfig(p, cellHome, cellConfig string) bool {
	p, err := filepath.Abs(p)
	if err != nil {
		return true
	}
	homes := []string{cellHome}
	if cellHome != "" {
		siblings, _ := filepath.Glob(filepath.Join(filepath.Dir(filepath.Dir(cellHome)), "*", "home"))
		homes = append(homes, siblings...)
	}
	var links, roots []os.FileInfo
	for _, h := range homes {
		if fi, err := os.Stat(h); h != "" && err == nil {
			roots = append(roots, fi)
		}
		link := filepath.Join(h, ".config", "assay")
		if h == cellHome {
			link = cellConfig
		}
		if fi, err := os.Lstat(link); link != "" && err == nil {
			links = append(links, fi)
		}
	}
	isAny := func(node os.FileInfo, set []os.FileInfo) bool {
		for _, s := range set {
			if os.SameFile(node, s) {
				return true
			}
		}
		return false
	}
	prefix, ok := resolvedPrefix(p, func(node os.FileInfo) bool { return isAny(node, links) })
	if !ok {
		return true
	}
	for d := prefix; ; d = filepath.Dir(d) {
		if fi, err := os.Stat(d); err == nil && isAny(fi, roots) {
			return true
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}

// maxConfigLinks bounds resolvedPrefix. It is at least the limit any supported
// OS applies, so a chain the OS still resolves never outruns the walk.
const maxConfigLinks = 64

// resolvedPrefix resolves the absolute path p element by element, following
// each link where it is met, as the OS does: a relative link target continues
// from the link's resolved directory, and ".." applies to the resolved path so
// far. stop sees every node that exists on the way. It returns the resolved
// path of the longest existing prefix of p, and ok=false when stop matched, the
// link budget was spent, or an element could not be read for a reason other
// than not existing.
func resolvedPrefix(p string, stop func(os.FileInfo) bool) (string, bool) {
	vol := filepath.VolumeName(p)
	dest := vol + string(filepath.Separator)
	pending := p[len(vol):]
	for followed := 0; ; {
		pending = strings.TrimLeftFunc(pending, func(r rune) bool { return r < 0x80 && os.IsPathSeparator(uint8(r)) })
		if pending == "" {
			return dest, true
		}
		elem := pending
		if i := strings.IndexFunc(pending, func(r rune) bool { return r < 0x80 && os.IsPathSeparator(uint8(r)) }); i >= 0 {
			elem, pending = pending[:i], pending[i:]
		} else {
			pending = ""
		}
		switch elem {
		case ".":
			continue
		case "..":
			dest = filepath.Dir(dest)
			continue
		}
		next := filepath.Join(dest, elem)
		node, err := os.Lstat(next)
		if os.IsNotExist(err) {
			return dest, true
		}
		if err != nil || stop(node) {
			return "", false
		}
		if node.Mode()&os.ModeSymlink == 0 {
			dest = next
			continue
		}
		if followed++; followed > maxConfigLinks {
			return "", false
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", false
		}
		if filepath.IsAbs(target) {
			vol = filepath.VolumeName(target)
			dest = vol + string(filepath.Separator)
			target = target[len(vol):]
		}
		pending = target + string(filepath.Separator) + pending
	}
}

// ghConfigDirFor matches the GitHub CLI's own documented precedence: $GH_CONFIG_DIR, then
// $XDG_CONFIG_HOME/gh, then (windows) %APPDATA%\GitHub CLI, then <home>/.config/gh. A native
// Windows login normally lives under AppData, which resolves with no home variable set.
func ghConfigDirFor(goos string, e *Env) (string, error) {
	if v := e.Get("GH_CONFIG_DIR"); v != "" {
		return v, nil
	}
	if v := e.Get("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, filepath.Base(ghConfigRelPath)), nil
	}
	if goos == "windows" {
		if v := e.Get("APPDATA"); v != "" {
			return filepath.Join(v, "GitHub CLI"), nil
		}
	}
	return underHome(goos, e, ghConfigRelPath)
}

// claudeConfigDirFor is the Claude harness home: $CLAUDE_CONFIG_DIR, else <home>/.claude.
func claudeConfigDirFor(goos string, e *Env) (string, error) {
	if v := e.Get("CLAUDE_CONFIG_DIR"); v != "" {
		return v, nil
	}
	return underHome(goos, e, ".claude")
}

// codexHomeFor is the Codex harness home: $CODEX_HOME, else <home>/.codex.
func codexHomeFor(goos string, e *Env) (string, error) {
	if v := e.Get("CODEX_HOME"); v != "" {
		return v, nil
	}
	return underHome(goos, e, ".codex")
}

func underHome(goos string, e *Env, elem ...string) (string, error) {
	home, err := operatorHomeFor(goos, e)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, elem...)...), nil
}

// hostHome is operatorHomeFor on this host, dying with the refusal when no home resolves.
func hostHome(e *Env) string { return mustResolve(operatorHomeFor(runtime.GOOS, e)) }

func mustResolve(p string, err error) string {
	if err != nil {
		die("%v", err)
	}
	return p
}
