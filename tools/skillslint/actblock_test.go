package main

import (
	"strings"
	"testing"
)

// actblock_test.go — the act-block lint (actblock.go): every example Act block
// under plugins/ opens with the zsh comment guard and keeps its comment lines
// free of shell metacharacters.

const actFence = "```"

// actSkill wraps one fenced block in a minimal skill file.
func actSkill(info string, lines ...string) string {
	return strings.Join(append(append([]string{
		"---", "name: some-desk", "description: x", "---", "", actFence + info,
	}, lines...), actFence, ""), "\n")
}

var cleanActLines = []string{
	actBlockGuardLine,
	"# 0. the act as one function. Pasting this only prints. Type driver_act live to act.",
	"driver_act() (",
	`  DRY_RUN=1; [ "${1-}" = live ] && DRY_RUN=0`,
	"  # 1. push the parked branch. git push has its own --dry-run.",
	`  if [ "$DRY_RUN" = 1 ]; then git push --dry-run origin b; else git push origin b; fi`,
	")",
	"driver_act",
}

// TestActBlockLintClean — a block that keeps both rules is counted and clean.
func TestActBlockLintClean(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "some-desk", actSkill("sh", cleanActLines...))
	blocks, issues, err := ActBlockIssues(root)
	if err != nil {
		t.Fatalf("ActBlockIssues: %v", err)
	}
	if blocks != 1 || len(issues) != 0 {
		t.Fatalf("blocks=%d issues=%+v, want 1 block and no issue", blocks, issues)
	}
}

// TestActBlockLintFlagsComment — each metacharacter class that runs in a
// comment-as-command is flagged, at the line it sits on.
func TestActBlockLintFlagsComment(t *testing.T) {
	for _, c := range []struct{ name, comment string }{
		{"semicolon then backtick span", "# 0. pasting only prints; " + "`driver_act live`" + " acts"},
		{"command substitution", "# 1. push $(git branch)"},
		{"parentheses", "# 2. push the branch (git push has its own --dry-run)"},
		{"redirect", "# 3. log it > out.txt"},
		{"pipe", "# 4. read it | sh"},
		{"apostrophe", "# 5. don't skip this"},
		{"glob", "# 6. every *.md"},
		{"trailing backslash", `# 7. joins the next line \`},
	} {
		t.Run(c.name, func(t *testing.T) {
			lines := append([]string{}, cleanActLines...)
			lines = append(lines[:2], append([]string{c.comment}, lines[2:]...)...)
			root := t.TempDir()
			writeSkill(t, root, "some-desk", actSkill("sh", lines...))
			_, issues, err := ActBlockIssues(root)
			if err != nil {
				t.Fatalf("ActBlockIssues: %v", err)
			}
			if len(issues) != 1 || !strings.Contains(issues[0].Msg, "line 9:") {
				t.Fatalf("issues=%+v, want exactly one, at line 9 (the planted comment)", issues)
			}
		})
	}
}

// TestActBlockLintFlagsMissingGuard — a block whose first line is not the zsh
// guard is flagged, whatever its first line is.
func TestActBlockLintFlagsMissingGuard(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "some-desk", actSkill("sh", cleanActLines[1:]...))
	_, issues, err := ActBlockIssues(root)
	if err != nil {
		t.Fatalf("ActBlockIssues: %v", err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "zsh comment guard") {
		t.Fatalf("issues=%+v, want exactly one missing-guard issue", issues)
	}
}

// TestActBlockLintScope — only an sh fence that defines the act function is an
// act block: another sh fence, or the act text quoted in a text fence, is not
// checked. With no act block anywhere the lint is could-not-check.
func TestActBlockLintScope(t *testing.T) {
	root := t.TempDir()
	body := actSkill("sh", "# not an act; any text here (fine)", "echo hi") +
		strings.Join(append(append([]string{actFence + "text"}, "driver_act() ( # quoted; not run )"), actFence, ""), "\n")
	writeSkill(t, root, "some-desk", body)
	blocks, issues, err := ActBlockIssues(root)
	if err == nil {
		t.Fatalf("blocks=%d issues=%+v: want could-not-check with no act block in the tree", blocks, issues)
	}
}

// TestActBlockLintRealTree — the plugin tree's own act blocks keep both rules,
// and there is at least one (the ask-decision example).
func TestActBlockLintRealTree(t *testing.T) {
	blocks, issues, err := ActBlockIssues("../..")
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	for _, is := range issues {
		t.Errorf("%s: %s", is.Path, is.Msg)
	}
	if blocks < 1 {
		t.Fatalf("found %d act blocks, want at least the ask-decision example", blocks)
	}
}
