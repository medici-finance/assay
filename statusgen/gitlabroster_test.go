package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// gitlabRosterAnchor returns the scaffolded .statusgen-roster script exactly as a
// GitLab runner receives it (YAML-decoded, so the literal block's indentation is
// gone).
func gitlabRosterAnchor(t *testing.T) string {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(initGitlabCI), &doc); err != nil {
		t.Fatalf("parsing the GitLab template: %v", err)
	}
	items, ok := doc[".statusgen-roster"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("the GitLab template's .statusgen-roster is %T, want one script item", doc[".statusgen-roster"])
	}
	script, ok := items[0].(string)
	if !ok {
		t.Fatalf(".statusgen-roster item is %T, want a string", items[0])
	}
	return script
}

// rosterlessEnv is the caller's environment minus every roster source (the
// ASSAY_* keys, the Actions transport switch, HOME and the GitLab roster
// variable), with HOME pointed at home: the state of a fresh GitLab runner.
func rosterlessEnv(home string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "ASSAY_") || strings.HasPrefix(k, "GITHUB_") || k == "HOME" ||
			k == "USERPROFILE" || k == "DESK_ROOTS" || k == "STATUSGEN_ROSTER_ENV" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+home, "USERPROFILE="+home)
}

// TestGitLabRosterUnsetClaim holds what the scaffolded GitLab roster step SAYS
// happens with STATUSGEN_ROSTER_ENV unset to what statusgen DOES on that runner.
// The step used to claim every job carried on with could-not-check checks; the
// file-class loader refuses a missing roster file, so `statusgen --lint` exited 5
// under a NOTICE that promised otherwise. The step and the adopter doc now state
// each job's outcome, and this test derives both outcomes from the built binary:
// the lint's exit code and its stdout line, and that the regen still writes the
// board. Any change to either outcome turns this red until the step's wording and
// docs/adopting-assay-gitlab.md section 2e are brought back in line.
//
// regression: #4978
func TestGitLabRosterUnsetClaim(t *testing.T) {
	requireJobTools(t)
	home := t.TempDir()
	env := rosterlessEnv(home)

	// 1. The step itself, as the runner runs it, with the variable unset.
	step := exec.Command("bash", "-e", scriptFile(t, gitlabRosterAnchor(t)))
	step.Env = env
	stepOut, err := step.CombinedOutput()
	if err != nil {
		t.Fatalf("the roster step must not fail with the variable unset (statusgen owns the refusal): %v\n%s", err, stepOut)
	}
	notice := string(stepOut)
	if _, err := os.Stat(filepath.Join(home, ".config", "assay", "roster.env")); !os.IsNotExist(err) {
		t.Fatalf("the unset branch wrote a roster file (stat err %v) — the refusal below would no longer be the fresh-runner state", err)
	}

	// 2. What statusgen does on that runner: lint, then the regen.
	bin := buildStatusgenBinary(t)
	run := func(args ...string) (int, string, string) {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, append([]string{"--root", root}, args...)...)
		cmd.Env = env
		var out, diag bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &diag
		code := 0
		if err := cmd.Run(); err != nil {
			ee, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("run statusgen %v: %v", args, err)
			}
			code = ee.ExitCode()
		}
		if _, err := os.Stat(filepath.Join(root, "STATUS.md")); err == nil {
			return code, strings.TrimSpace(out.String()), "board written"
		}
		return code, strings.TrimSpace(out.String()), diag.String()
	}
	lintCode, lintOut, _ := run("--lint")
	regenCode, _, regenBoard := run()

	// 3. The step's NOTICE must state exactly those outcomes.
	if lintCode == 0 {
		t.Fatalf("statusgen --lint passed with no roster file (stdout %q): the refusal the step and doc describe is gone — re-word both", lintOut)
	}
	for _, want := range []string{
		"NOTICE: STATUSGEN_ROSTER_ENV is not set",
		"Merge-request job: statusgen --lint refuses it",
		"'" + lintOut + "'",
		fmt.Sprintf("and exits %d, so the merge-request job FAILS", lintCode),
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("the roster step's NOTICE does not say %q, which is what statusgen --lint does with no roster (exit %d, stdout %q):\n%s", want, lintCode, lintOut, notice)
		}
	}
	if regenCode != 0 || regenBoard != "board written" {
		t.Errorf("the regen with no roster exited %d without a board (%s), but the step says the board is still generated", regenCode, regenBoard)
	}
	if !strings.Contains(notice, "Default-branch regen job: the board is still generated") {
		t.Errorf("the roster step's NOTICE does not say the regen still writes the board:\n%s", notice)
	}
	if strings.Contains(notice, "job stays green") || strings.Contains(notice, "the board below is generated without it") {
		t.Errorf("the roster step still claims one outcome for both jobs:\n%s", notice)
	}

	// 4. The adopter doc quotes the NOTICE verbatim (runner HOME shown as /root).
	docBytes, err := os.ReadFile(filepath.Join("..", "docs", "adopting-assay-gitlab.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Only section 2e counts: a copy of these lines elsewhere in the guide must not
	// satisfy the check.
	_, doc, ok := strings.Cut(string(docBytes), "\n## 2e. ")
	if !ok {
		t.Fatal("docs/adopting-assay-gitlab.md has no section 2e heading")
	}
	doc, _, _ = strings.Cut(doc, "\n## ")
	for _, line := range strings.Split(strings.ReplaceAll(notice, home, "/root"), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.Contains(doc, line) {
			t.Errorf("docs/adopting-assay-gitlab.md section 2e does not quote the roster step's line verbatim:\n%s", line)
		}
	}
}
