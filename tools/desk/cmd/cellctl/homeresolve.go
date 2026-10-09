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

// cellOwnedConfig reports whether the absolute candidate p is the cell's config
// link under any spelling, or resolves to a resource inside the cell home. It
// follows one link at a time, so an alias that points at the cell link is caught
// on the link node itself (os.SameFile on Lstat) before the link's own target is
// reached. A candidate that does not exist is not cell-owned here; the house
// check reports a missing target on its own.
func cellOwnedConfig(p, cellHome, cellConfig string) bool {
	if cellLink, err := os.Lstat(cellConfig); cellConfig != "" && err == nil {
		for hop, q := 0, p; hop < 40; hop++ {
			node, err := os.Lstat(q)
			if err != nil {
				break
			}
			if os.SameFile(node, cellLink) {
				return true
			}
			target, err := os.Readlink(q)
			if err != nil {
				break
			}
			if !filepath.IsAbs(target) {
				dir := filepath.Dir(q)
				if r, err := filepath.EvalSymlinks(dir); err == nil {
					dir = r
				}
				target = filepath.Join(dir, target)
			}
			q = target
		}
	}
	home, err := filepath.EvalSymlinks(cellHome)
	if cellHome == "" || err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(home, resolved)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
