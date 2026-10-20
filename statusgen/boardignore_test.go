package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Ignored board paths (#2915). The regen job statusgen init scaffolds stages
// the board with a path-scoped add; when the adopter's .gitignore matches
// STATUS.md (or docs/streams) git refuses it, and before this fix the first
// run on main failed with git's generic "Use -f" hint on every push. These
// tests reach the behaviour only through entry points that predate the fix
// (runInitForge, run, the template text), so they compile, and fail, on the
// pre-fix code. The shell-vs-Go oracle is in boardoracle_test.go.

// isolateGit points git at an empty global config and no system config, so a
// developer's own excludesFile cannot change what these tests see.
func isolateGit(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GITHUB_ACTIONS", "")
}

// gitRepo is a fresh `git init` tree with the given files written.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	requireTool(t, "git")
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// scaffoldPaths lists every path under dir outside .git.
func scaffoldPaths(t *testing.T, dir string) []string {
	t.Helper()
	var got []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	return got
}

// TestInitRefusesIgnoredBoard: init on a repo whose .gitignore matches
// STATUS.md refuses, names the rule and the fix, and writes nothing — in a
// real run and in a dry run, for either forge's template.
//
// regression: #2915
func TestInitRefusesIgnoredBoard(t *testing.T) {
	isolateGit(t)
	for _, forge := range []forgeKind{forgeGitHub, forgeGitLab} {
		for _, dry := range []bool{false, true} {
			dir := gitRepo(t, map[string]string{".gitignore": "STATUS.md\n"})
			var code int
			errOut := captureStderr(t, func() {
				_ = captureStdout(t, func() { code = runInitForge(dir, forge, true, "", dry) })
			})
			if code == 0 {
				t.Errorf("forge %v dry=%v: init exited 0 on a tree that ignores STATUS.md:\n%s", forge, dry, errOut)
			}
			want := `STATUS.md is ignored by .gitignore:1:STATUS.md; add "!STATUS.md" to .gitignore`
			if !strings.Contains(errOut, want) {
				t.Errorf("forge %v dry=%v: refusal does not name the rule and fix %q:\n%s", forge, dry, want, errOut)
			}
			if got := scaffoldPaths(t, dir); len(got) != 1 || got[0] != ".gitignore" {
				t.Errorf("forge %v dry=%v: a refused init wrote files: %v", forge, dry, got)
			}
		}
	}
}

// TestInitIgnoredParentDir: an ignored docs/ hides docs/streams from the add
// too; init names the parent rule and says a `!` line alone cannot fix it.
//
// regression: #2915
func TestInitIgnoredParentDir(t *testing.T) {
	isolateGit(t)
	dir := gitRepo(t, map[string]string{".gitignore": "docs/\n"})
	var code int
	errOut := captureStderr(t, func() {
		_ = captureStdout(t, func() { code = runInitForge(dir, forgeGitHub, true, "", false) })
	})
	if code == 0 {
		t.Fatalf("init exited 0 on a tree that ignores docs/:\n%s", errOut)
	}
	for _, want := range []string{"docs/streams is ignored by .gitignore:1:docs/", "narrow that rule"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("refusal lacks %q:\n%s", want, errOut)
		}
	}
}

// TestInitCleanRepoUnchanged: a git repo with no rule over the board paths
// (and one whose rule is negated) scaffolds exactly as before.
func TestInitCleanRepoUnchanged(t *testing.T) {
	isolateGit(t)
	for name, ignore := range map[string]string{
		"no-gitignore": "",
		"unrelated":    "node_modules/\n*.log\n",
		"negated":      "*.md\n!STATUS.md\n",
	} {
		files := map[string]string{}
		if ignore != "" {
			files[".gitignore"] = ignore
		}
		dir := gitRepo(t, files)
		var code int
		errOut := captureStderr(t, func() {
			_ = captureStdout(t, func() { code = runInitForge(dir, forgeGitHub, true, "", false) })
		})
		if code != 0 {
			t.Errorf("%s: init exited %d:\n%s", name, code, errOut)
		}
		if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "assay-statusgen.yml")); err != nil {
			t.Errorf("%s: no workflow written: %v", name, err)
		}
	}
}

// TestLintIgnoredBoard: --lint on an init-scaffolded repo whose .gitignore
// then matches STATUS.md is a PROBLEM naming the rule; without the rule it is
// clean.
//
// regression: #2915
func TestLintIgnoredBoard(t *testing.T) {
	isolateGit(t)
	for _, ignored := range []bool{false, true} {
		dir := gitRepo(t, nil)
		_ = captureStderr(t, func() {
			_ = captureStdout(t, func() {
				if code := runInitForge(dir, forgeGitHub, true, "", false); code != 0 {
					t.Fatalf("init exit %d", code)
				}
			})
		})
		if ignored {
			if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("STATUS.md\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var code int
		errOut := captureStderr(t, func() {
			_ = captureStdout(t, func() { code = run(dir, "lint", nil, nil, "") })
		})
		want := `PROBLEM: STATUS.md is ignored by .gitignore:1:STATUS.md; add "!STATUS.md" to .gitignore`
		if ignored {
			if code == 0 || !strings.Contains(errOut, want) {
				t.Errorf("ignored STATUS.md: lint exit %d, want 1 with %q:\n%s", code, want, errOut)
			}
		} else if code != 0 || strings.Contains(errOut, "is ignored by") {
			t.Errorf("clean repo: lint exit %d or an ignore PROBLEM:\n%s", code, errOut)
		}
	}
}

// workflowStep returns the run: text of one step in a workflow template.
func workflowStep(t *testing.T, raw, job, step string) string {
	t.Helper()
	for _, st := range parseWorkflow(t, []byte(raw), "initWorkflow").Jobs[job].Steps {
		if st.Name == step {
			return st.Run
		}
	}
	t.Fatalf("no step %q in job %q", step, job)
	return ""
}

const initCommitStep = "Commit generated board surfaces if they changed"

// TestRegenStepNamesRule: the scaffolded GitHub commit step, run on a repo
// whose .gitignore matches STATUS.md, fails with an ::error:: naming the rule
// and the fix — not git's "Use -f" hint — commits and pushes nothing, and
// never force-adds. Without the rule the same step lands the board.
//
// regression: #2915
func TestRegenStepNamesRule(t *testing.T) {
	step := workflowStep(t, initWorkflow, "regen", initCommitStep)
	if strings.Contains(step, "add -f") || strings.Contains(step, "--force") {
		t.Fatalf("the commit step force-adds over ignore rules:\n%s", step)
	}
	for _, ignored := range []bool{false, true} {
		r := newPushRig(t)
		if err := plantFile("STATUS.md", "board 1\n")(r.ws); err != nil {
			t.Fatal(err)
		}
		if ignored {
			if err := plantFile(".gitignore", "STATUS.md\n")(r.ws); err != nil {
				t.Fatal(err)
			}
		}
		head := r.git(r.ws, "rev-parse", "HEAD")
		code, out := r.run(step, "SHIM_PUSH=ok")
		if !ignored {
			if code != 0 {
				t.Errorf("clean repo: the step failed:\n%s", out)
			}
			continue
		}
		want := `::error::STATUS.md is ignored by .gitignore:1:STATUS.md; add "!STATUS.md" to .gitignore`
		if code == 0 || !strings.Contains(out, want) {
			t.Errorf("ignored STATUS.md: exit %d, want non-zero with %q:\n%s", code, want, out)
		}
		if strings.Contains(out, "Use -f") {
			t.Errorf("git's generic hint reached the log instead of the rule:\n%s", out)
		}
		if got := r.git(r.ws, "rev-parse", "HEAD"); got != head {
			t.Errorf("a board commit was written past the refusal")
		}
		if n := r.count("pushes"); n != 0 {
			t.Errorf("pushed %d time(s) past the refusal", n)
		}
	}
}

// TestGitlabRegenNamesRule: the same for the scaffolded GitLab regen job.
//
// regression: #2915
func TestGitlabRegenNamesRule(t *testing.T) {
	r := newPushRig(t)
	if err := plantFile(".gitignore", "STATUS.md\n")(r.ws); err != nil {
		t.Fatal(err)
	}
	code, out := r.run(gitlabRegenScript(t, initGitlabCI), gitlabEnv("200:live", "SHIM_PUSH=ok")...)
	want := `ERROR: STATUS.md is ignored by .gitignore:1:STATUS.md; add "!STATUS.md" to .gitignore`
	if code == 0 || !strings.Contains(out, want) {
		t.Errorf("exit %d, want non-zero with %q:\n%s", code, want, out)
	}
	if strings.Contains(out, "Use -f") {
		t.Errorf("git's generic hint reached the log instead of the rule:\n%s", out)
	}
	if n := r.count("pushes"); n != 0 {
		t.Errorf("pushed %d time(s) past the refusal", n)
	}
}

// rawBoardAdd matches a `git add` whose arguments name a board path directly —
// the defect class: a stage made without asking git whether it would refuse
// the path for an ignore rule. Templates stage board paths with board_add.
var rawBoardAdd = regexp.MustCompile(`\bgit\s+add\b[^\n]*(STATUS\.md|docs/streams)`)

// TestNoRawBoardAdd: no template statusgen init scaffolds stages a board path
// with a raw `git add`, and every template that calls board_add defines it.
// A planted raw add proves the matcher fires.
//
// regression: #2915
func TestNoRawBoardAdd(t *testing.T) {
	if !rawBoardAdd.MatchString("      git add STATUS.md docs/streams\n") ||
		!rawBoardAdd.MatchString("git add -- STATUS.md\n") {
		t.Fatal("the matcher misses a planted raw board add")
	}
	for name, body := range map[string]string{"initWorkflow": initWorkflow, "initGitlabCI": initGitlabCI} {
		if m := rawBoardAdd.FindString(body); m != "" {
			t.Errorf("%s stages a board path without the ignore check: %q", name, m)
		}
		if strings.Contains(body, "board_add ") && !strings.Contains(body, "board_add() {") {
			t.Errorf("%s calls board_add but never defines it", name)
		}
	}
}
