package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #3460
func TestRederiveHeadingCells(t *testing.T) {
	for _, heading := range []string{"## Verify", "## Verify (executable checks)", "  ## Verify — offline", "## Verify: checks", "## VerifyChecks"} {
		t.Run(heading, func(t *testing.T) {
			base := strings.Replace(baseTableMD, "## Verify", heading, 1)
			base = strings.Replace(base, "go test ./foo/...", "`printf a \\| grep 'a\\|b'`", 1)
			base = strings.Replace(base, "| pass |", "| output is `a\\|b` |", 1)
			head := strings.Replace(base, "`printf a \\| grep 'a\\|b'`", "`true`", 1)
			head = strings.Replace(head, "output is `a\\|b`", "exit 0", 1)
			out, err := fixtureRederive(t, base, head)
			if err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("row 1: from-base class=%q command=%q expect=%q", "check", "`printf a \\| grep 'a\\|b'`", "output is `a\\|b`")
			if !strings.Contains(out, want) {
				t.Fatalf("lost original cells: want %s; got %s", want, out)
			}
		})
	}
}

// regression: #3460
func TestRederiveMalformed(t *testing.T) {
	cases := map[string]string{
		"missing":          "# Brief\n## Evidence\n| Command | Expect |\n|---|---|\n| true | pass |",
		"empty file":       "",
		"empty":            "## Verify\n",
		"no rows":          "## Verify\n| Command | Expect |\n|---|---|\n",
		"no command":       strings.Replace(baseTableMD, "Command", "Action", 1),
		"no expect":        strings.Replace(baseTableMD, "Expect", "Result", 1),
		"no separator":     strings.Replace(baseTableMD, "|---|-------|---------|--------|\n", "", 1),
		"bad separator":    strings.Replace(baseTableMD, "|---|-------|---------|--------|", "|---|bad|---|---|", 1),
		"short row":        strings.Replace(baseTableMD, "| 1 | check | go test ./foo/... | pass |", "| 1 | check | go test ./foo/... |", 1),
		"long row":         strings.Replace(baseTableMD, "go test ./foo/...", "printf a | cat", 1),
		"empty command":    strings.Replace(baseTableMD, "go test ./foo/...", "", 1),
		"empty expect":     strings.Replace(baseTableMD, "| pass |", "| |", 1),
		"duplicate id":     strings.Replace(baseTableMD, "| 2 |", "| 1 |", 1),
		"empty id":         strings.Replace(baseTableMD, "| 1 |", "| |", 1),
		"duplicate column": strings.Replace(baseTableMD, "Class", "Command", 1),
	}
	for name, bad := range cases {
		for _, side := range []string{"base", "head"} {
			t.Run(name+"/"+side, func(t *testing.T) {
				base, head := baseTableMD, headTableMD
				if side == "base" {
					base = bad
				} else {
					head = bad
				}
				out, err := fixtureRederive(t, base, head)
				if err == nil || !strings.Contains(err.Error(), "could-not-check") || out != "" {
					t.Fatalf("malformed %s accepted: err=%v output=%q", side, err, out)
				}
			})
		}
	}
}

// fixtureRederive exercises the real merge-base and git-show path offline.
func fixtureRederive(t *testing.T, base, head string) (string, error) {
	t.Helper()
	return fixtureRederivePath(t, base, head, "brief.md", false)
}

func fixtureRederivePath(t *testing.T, base, head, brief string, nested bool) (string, error) {
	t.Helper()
	root := t.TempDir()
	if nested {
		brief = "sub/" + brief
		if err := os.MkdirAll(filepath.Join(root, "sub"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.hooksPath=" + filepath.Join(root, "no-hooks"), "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		// Fixtures never inherit an outer repository's index/worktree selection.
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "GIT_") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "--quiet")
	if base != "absent-fixture" {
		if err := os.WriteFile(filepath.Join(root, brief), []byte(base), 0600); err != nil {
			t.Fatal(err)
		}
		git("add", brief)
	}
	git("commit", "--quiet", "--allow-empty", "-m", "base")
	git("branch", "base")
	if err := os.WriteFile(filepath.Join(root, brief), []byte(head), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", brief)
	git("commit", "--quiet", "--allow-empty", "-m", "candidate")
	var out bytes.Buffer
	if nested {
		root = filepath.Join(root, "sub")
		brief = strings.TrimPrefix(brief, "sub/")
	}
	err := cmdRederive([]string{"--root", root, "--brief", brief, "--main", "base"}, &out)
	return out.String(), err
}

// regression: #3460. Second entry point: changed diff rows must use the same heading grammar.
func TestDecoratedVerifyDiff(t *testing.T) {
	for _, h := range []string{"## Verify (executable checks)", "## Verify: offline", "## VerifyChecks"} {
		diff := "diff --git a/brief.md b/brief.md\n@@ -1,2 +1,2 @@ " + h + "\n-old\n+new\n"
		if !verifySectionTouched(diff, "brief.md") {
			t.Errorf("missed %q", h)
		}
	}
}

// regression: #3460
func TestRederiveReadFailure(t *testing.T) {
	old := runGitFn
	t.Cleanup(func() { runGitFn = old })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "brief.md"), []byte(headTableMD), 0600); err != nil {
		t.Fatal(err)
	}
	runGitFn = func(_ string, args ...string) (string, error) {
		if args[0] == "rev-parse" {
			return "", nil
		}
		if args[0] == "merge-base" {
			return "base", nil
		}
		return "", fmt.Errorf("unreadable base object")
	}
	var out bytes.Buffer
	err := cmdRederive([]string{"--root", root, "--brief", "brief.md"}, &out)
	if err == nil || out.Len() != 0 {
		t.Fatalf("unreadable base became author-added: err=%v out=%q", err, out.String())
	}
}

// Existing new-brief behavior remains supported through real local Git.
func TestRederiveNewBrief(t *testing.T) {
	out, err := fixtureRederive(t, "absent-fixture", headTableMD)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, ": author-added") != 3 || strings.Contains(out, "from-base") {
		t.Fatalf("new brief provenance: %s", out)
	}
}

// Compatibility controls: column lookup is by name and ID-less tables use ordinals.
func TestRederiveColumnOrder(t *testing.T) {
	base := "## Verify\n| Expect | Command |\n|---|---|\n| old result | old command |\n"
	head := "## Verify\n| Command | Expect |\n|---|---|\n| new command | new result |\n"
	out, err := fixtureRederive(t, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `row 1: from-base class="" command="old command" expect="old result"`) {
		t.Fatalf("column/ordinal provenance: %s", out)
	}
}

// regression: #3460
func TestRederiveExistingReadGap(t *testing.T) {
	old := runGitFn
	t.Cleanup(func() { runGitFn = old })
	runGitFn = func(root string, args ...string) (string, error) {
		if args[0] == "show" {
			return "", fmt.Errorf("simulated blob read failure")
		}
		return runGit(root, args...)
	}
	// The merge-base and ls-tree are real: this path EXISTS in the base tree.
	out, err := fixtureRederive(t, baseTableMD, headTableMD)
	if err == nil || out != "" {
		t.Fatalf("existing unreadable brief became author-added: err=%v out=%q", err, out)
	}
}
