package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSettingsJSONWiresWriteguard is a regression guard for the
// worktree-isolation backstop: prompt-carried absolute paths can override
// worktree isolation, so prompt discipline alone is not enough — a mechanical
// pre-write backstop is needed. guard.go and guard_test.go prove the DECISION
// LOGIC is correct; this test proves the compiled binary is actually WIRED
// into a live PreToolUse hook for sessions working in this repo's own shared
// checkout. Absent a wired .claude/settings.json fixture, writeguard is
// compiled and unit-tested here but not actually armed for a session homed in
// this checkout; this test closes that gap whenever the fixture is present in
// the published subset.
//
// SCOPE — WHAT THIS WIRING DOES NOT COVER. Stated explicitly so the hook is
// not mistaken for blanket coverage. A PreToolUse hook is consulted only by a
// Claude Code session that loads THIS repo's .claude/settings.json, and only
// for the matched tool surfaces. Measured against the built binary:
//
//   - REFUSED (checked): a redirect into the shared checkout; an
//     ABSOLUTE-PATH invocation of a write tool (`/usr/bin/tee <shared>/x`) —
//     unlike a PATH shim, a hook sees the whole command string, so absolute
//     paths are NOT a bypass here; a `cd` into the shared checkout; and any
//     Edit/Write/MultiEdit/NotebookEdit whose target lands there.
//   - NOT REFUSED (checked): an interpreter carrying the write inside a
//     quoted literal — `/usr/bin/python3 -c "open('<shared>/x','w')"`,
//     `perl -e`, and the like. maskLiterals deliberately blanks quoted spans
//     (so prose and awk programs are not read as redirects), and only `sh -c`
//     / heredoc bodies are re-exposed as commands, so a non-shell
//     interpreter's program text is never scanned. This is writeguard's
//     documented decision model, not a wiring defect — but it IS a hole.
//   - NOT SEEN AT ALL (structural): HTTP/SDK contact (curl, gh, client-go) —
//     no local path to key on; a credentialed MCP or browser tool, which does
//     not route through Bash/Edit at all; an ssh or docker hop, where the
//     write happens on another host or container; and ANY non-Claude-Code
//     runner, for which a committed settings.json is simply inert.
//
// On every surface in the last two groups the layer count is ONE — whatever
// the callee itself checks — not three. This test asserts only that the
// wiring is present; it does not, and cannot, assert coverage of those paths.
//
// It intentionally does NOT re-test decision logic (that's the rest of this
// package) — only that the repo's own settings.json still (a) parses, and
// (b) still points hooks.PreToolUse's Edit/Write/MultiEdit/NotebookEdit and
// Bash matchers at a command invoking this binary, so an editor cannot
// silently drop the wiring without a test going red.
func TestSettingsJSONWiresWriteguard(t *testing.T) {
	// tools/desk/cmd/writeguard -> repo root is four levels up.
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
	// .claude/settings.json is not part of every checkout of this repository, so
	// there may be no settings.json to wire. Skip when it is absent; where it is
	// present this wiring regression guard runs in full. Only os.ErrNotExist
	// skips — a settings.json that exists but cannot be read still fails.
	if _, err := os.Stat(settingsPath); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present in this tree", settingsPath)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading %s: %v (writeguard is compiled but not wired into any live "+
			"PreToolUse hook for sessions working in this repo's own shared checkout)", settingsPath, err)
	}

	var doc struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s is not valid JSON: %v", settingsPath, err)
	}

	// The command the wiring must invoke, derived from the Makefile's
	// INSTALL_DIR rather than hard-coded here, so relocating the install
	// prefix without re-pointing settings.json goes red instead of leaving a
	// hook whose command silently does not exist. Substring-matching
	// "writeguard" in the command is NOT enough: a hook whose command names a
	// path that is not there cannot run, and writeguard FAILS OPEN, so a
	// mistyped or stale path disarms every surface with nothing failing
	// loudly — measured, see the mutation note on matcherCovers below.
	wantCommand := filepath.Join(installDir(t, repoRoot), "writeguard")

	// matcherCovers reports whether a PreToolUse `matcher` NAMES `want` as one
	// of its own alternatives.
	//
	// This is deliberately an allowlist of exact alternatives and NOT
	// strings.Contains(matcher, want). Contains is wrong in both directions
	// and both were measured against the built binary:
	//
	//   - FALSE NEGATIVE (the dangerous one). "Edit" is a substring of
	//     "MultiEdit" and "NotebookEdit", so a matcher narrowed to
	//     "Write|MultiEdit|NotebookEdit" still Contains "Edit" and the check
	//     stays green — while the harness, matching the tool name against the
	//     matcher, selects NO hook for the plain Edit tool and the write
	//     proceeds unguarded. The guard would be off for Edit with every test
	//     passing, which is worse than no guard because it manufactures
	//     confidence.
	//   - FALSE POSITIVE. A matcher of only "NotebookEdit" Contains "Edit" and
	//     would be reported as covering the Edit surface it does not cover.
	//
	// Requiring the tool to appear as its own alternative is also the reading
	// that holds whether the harness anchors the matcher regex or not, so the
	// guard does not depend on that detail. A wildcard matcher (".*",
	// "Edit.*") is intentionally not accepted: this is a safety instrument
	// and the surfaces it arms are named explicitly.
	matcherCovers := func(matcher, want string) bool {
		for _, alt := range strings.Split(matcher, "|") {
			if strings.TrimSpace(alt) == want {
				return true
			}
		}
		return false
	}

	// matcherWiresWriteguard reports whether ANY PreToolUse entry whose
	// matcher names `want` invokes the installed writeguard binary.
	matcherWiresWriteguard := func(want string) bool {
		for _, entry := range doc.Hooks.PreToolUse {
			if !matcherCovers(entry.Matcher, want) {
				continue
			}
			for _, h := range entry.Hooks {
				if h.Type == "command" && h.Command == wantCommand {
					return true
				}
			}
		}
		return false
	}

	// The four file-editing tool surfaces (guard.go's own CheckFilePath
	// covers exactly these) must share a hook whose matcher lists all four —
	// or each be covered individually. Check each is covered by SOME
	// matcher, rather than requiring one specific combined string, so a
	// reasonable re-split of the matcher entries doesn't false-fail this
	// test.
	for _, tool := range []string{"Edit", "Write", "MultiEdit", "NotebookEdit"} {
		if !matcherWiresWriteguard(tool) {
			t.Errorf("%s: no PreToolUse matcher names %q with a hook invoking %s — "+
				"a file-write from a shared-checkout-homed session would go unguarded",
				settingsPath, tool, wantCommand)
		}
	}
	if !matcherWiresWriteguard("Bash") {
		t.Errorf("%s: no PreToolUse matcher names %q with a hook invoking %s — "+
			"a Bash write/cd into the shared checkout would go unguarded",
			settingsPath, "Bash", wantCommand)
	}
}

// installDir returns the Makefile's INSTALL_DIR — the one place that decides
// where `sudo make desk-install` puts the guard binary. Reading it (rather
// than repeating the literal here) is what makes the wiring test notice an
// install-prefix move: change INSTALL_DIR without re-pointing
// .claude/settings.json and this test goes red, instead of leaving a
// PreToolUse hook pointed at a path with no binary behind it. tools.yml's
// paths: filter carries a "Makefile" entry so this read actually triggers the
// job, and the row is registered in deskkit's citrigger registry.
func installDir(t *testing.T, repoRoot string) string {
	t.Helper()
	makefilePath := filepath.Join(repoRoot, "Makefile")
	data, err := os.ReadFile(makefilePath)
	if err != nil {
		t.Fatalf("reading %s: %v", makefilePath, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "INSTALL_DIR")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		for _, op := range []string{":=", "="} {
			if v, ok := strings.CutPrefix(rest, op); ok {
				if v = strings.TrimSpace(v); v != "" {
					return v
				}
			}
		}
	}
	// Not found is a FAILURE, never a skip: silently falling back to a
	// literal would restore exactly the blindness this read exists to remove.
	t.Fatalf("%s: no INSTALL_DIR assignment found — the wiring test cannot "+
		"establish where the guard binary is installed", makefilePath)
	return ""
}
