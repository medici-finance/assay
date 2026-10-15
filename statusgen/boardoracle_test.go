package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// boardOracleCases are the ignore shapes the shell and Go halves of the
// ignored-board check must agree on (#2915). commit lists paths committed
// before .gitignore is written (a tracked file a rule later matches stays
// stageable); want is the board paths git refuses, in boardAddPaths order.
var boardOracleCases = []struct {
	name, ignore string
	commit       []string
	want         []string
}{
	{"clean", "", nil, nil},
	{"unrelated", "node_modules/\n", nil, nil},
	{"status-ignored", "STATUS.md\n", nil, []string{"STATUS.md"}},
	{"glob-ignored", "*.md\n", nil, []string{"STATUS.md"}},
	{"anchored", "/STATUS.md\n", nil, []string{"STATUS.md"}},
	{"negated", "STATUS.md\n!STATUS.md\n", nil, nil},
	{"tracked", "STATUS.md\n", []string{"STATUS.md"}, nil},
	{"streams-ignored", "docs/streams/\n", nil, []string{"docs/streams"}},
	{"parent-ignored", "docs/\n", nil, []string{"docs/streams"}},
	{"parent-ignored-tracked", "docs/\n", []string{"docs/streams/s/README.md"}, []string{"docs/streams"}},
	{"both", "STATUS.md\ndocs/\n", nil, []string{"STATUS.md", "docs/streams"}},
}

// TestBoardAddOracle: boardadd.sh's board_add (the text both templates embed)
// and boardIgnoreHits (what init and --lint run) reach the same verdict on
// every case, print byte-identical messages, and board_add stages nothing
// when it refuses and the board when it does not.
//
// regression: #2915
func TestBoardAddOracle(t *testing.T) {
	isolateGit(t)
	requireTool(t, "sh")
	shDir := t.TempDir()
	writeFile(t, shDir, "boardadd.sh", boardAddSh)
	sh := filepath.Join(shDir, "boardadd.sh")
	for _, c := range boardOracleCases {
		t.Run(c.name, func(t *testing.T) {
			dir := gitRepo(t, map[string]string{
				"STATUS.md":                "board\n",
				"docs/streams/s/README.md": "# s\n",
			})
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
				return string(out)
			}
			if len(c.commit) > 0 {
				git(append([]string{"add", "--"}, c.commit...)...)
				git("commit", "-q", "-m", "seed")
			}
			if c.ignore != "" {
				writeFile(t, dir, ".gitignore", c.ignore)
			}

			hits, err := boardIgnoreHits(dir)
			if err != nil {
				t.Fatalf("boardIgnoreHits: %v", err)
			}
			var gotPaths, wantLines []string
			for _, h := range hits {
				gotPaths = append(gotPaths, h.path)
				if h.rule == "" {
					t.Errorf("%s: git refused it but no rule was named", h.path)
				}
				wantLines = append(wantLines, "ERROR: "+h.message())
			}
			if strings.Join(gotPaths, ",") != strings.Join(c.want, ",") {
				t.Errorf("Go verdict %v, want %v", gotPaths, c.want)
			}

			cmd := exec.Command("sh", "-c", `. "$1"; board_add STATUS.md docs/streams`, "sh", sh)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if (code != 0) != (len(c.want) > 0) {
				t.Errorf("board_add exit %d, want refusal=%v:\n%s", code, len(c.want) > 0, out)
			}
			if got := strings.TrimRight(string(out), "\n"); got != strings.Join(wantLines, "\n") {
				t.Errorf("shell and Go messages differ\nshell: %q\ngo:    %q", got, strings.Join(wantLines, "\n"))
			}
			staged := strings.TrimSpace(git("diff", "--cached", "--name-only"))
			if len(c.want) > 0 && staged != "" {
				t.Errorf("a refused board_add staged %q", staged)
			}
			if len(c.want) == 0 && len(c.commit) == 0 && !strings.Contains(staged, "STATUS.md") {
				t.Errorf("board_add staged %q, want the board", staged)
			}
		})
	}
}

// TestBoardIgnoreNotRepo: outside a work tree there is no verdict — an error,
// never an empty (clean) result.
func TestBoardIgnoreNotRepo(t *testing.T) {
	isolateGit(t)
	requireTool(t, "git")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(t.TempDir()))
	if _, err := boardIgnoreHits(t.TempDir()); err != errNotWorkTree {
		t.Errorf("err = %v, want errNotWorkTree", err)
	}
}
