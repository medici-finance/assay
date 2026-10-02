package main

import (
	"fmt"
	"path/filepath"
	"runtime"
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
// home those tools then read: %USERPROFILE% on windows, $HOME elsewhere. The other variable is
// the fallback when the primary is unset, the precedence the Codex launch already used, so a
// HOME-only Git Bash shell on windows keeps working.
//
// Neither set REFUSES. Every path derived from an empty home is either relative (resolved
// against whatever directory cellctl happened to run in) or rooted at the filesystem root, and
// a config home resolved that way would link, check or launch against a directory that is not
// the operator's.
func operatorHomeFor(goos string, e *Env) (string, error) {
	if goos == "windows" {
		if v := e.Get("USERPROFILE"); v != "" {
			return v, nil
		}
		if v := e.Get("HOME"); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("cannot resolve the operator home: neither USERPROFILE nor HOME is set " +
			"(refusing to derive a config path from an empty home)")
	}
	if v := e.Get("HOME"); v != "" {
		return v, nil
	}
	if v := e.Get("USERPROFILE"); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("cannot resolve the operator home: neither HOME nor USERPROFILE is set " +
		"(refusing to derive a config path from an empty home)")
}

// configHomeFor is the operator's assay config home: $ASSAY_CONFIG_HOME, else
// <home>/.config/assay. The override needs no home at all.
func configHomeFor(goos string, e *Env) (string, error) {
	if v := e.Get("ASSAY_CONFIG_HOME"); v != "" {
		return v, nil
	}
	return underHome(goos, e, ".config", "assay")
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
