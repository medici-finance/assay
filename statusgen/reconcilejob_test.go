package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The scheduled reconcile tick — the reconcile job's own clone, compute and
// publish `run:` texts, read from the staged workflow — exercised end to end
// against a throwaway bare origin, with stub `statusgen` and `gh` on PATH —
// hermetic: no network, no token, no real repo.

const jobReadmeMain = "# s\n\n| # | Brief | Status |\n|---|---|---|\n| 01 | one | todo |\n| 02 | two | todo |\n"

// Sentinels the rig substitutes for the workflow's token expressions, so a test
// can tell which step held which credential.
const (
	readTokenSentinel  = "READ-TOKEN-SENTINEL"
	writeTokenSentinel = "WRITE-TOKEN-SENTINEL"
)

// stubStatusgen records its arguments, one invocation per line, in
// $STUB_LOG/statusgen.log. It flips row $STUB_FLIP (if set) from todo to
// implemented in the fixture README on `reconcile`, or with $STUB_HOLD reports
// one held row (in the real report's shape, statusgen/reconcile.go's
// printReconcileTable) and writes nothing; prints the --apply report and exits
// $STUB_RECONCILE_RC. `regen` writes a non-README file when $STUB_STRAY is set
// and is otherwise a no-op; the lint exits $STUB_LINT_RC. With
// $STUB_CHECK_WRITE set, every invocation fails if its environment holds the
// write token.
const stubStatusgen = `#!/usr/bin/env bash
set -eu
echo "$*" >> "$STUB_LOG/statusgen.log"
if [ -n "${STUB_CHECK_WRITE:-}" ]; then
  case "$(env)" in *WRITE-TOKEN-SENTINEL*) echo "statusgen ran holding the write token" >&2; exit 97 ;; esac
fi
f=docs/streams/s/README.md
case "${1:-}" in
  reconcile)
    if [ "${STUB_RECONCILE_RC:-0}" != 0 ]; then echo "reconcile --apply: could-not-check — HTTP 401" >&2; exit "$STUB_RECONCILE_RC"; fi
    if [ -n "${STUB_HOLD:-}" ]; then
      echo "reconcile: o/r (2 brief(s))"
      echo "reconcile --apply: held 1 row(s) — writing them would add a lint PROBLEM; not written"
      echo "  s/${STUB_HOLD}                                     todo -> implemented   [PR #7 (merged abc1234)]"
      echo "      would add: risk-gated brief at \"implemented\" has no design: record"
    fi
    if [ -n "${STUB_FLIP:-}" ]; then
      sed "s/^| ${STUB_FLIP} | \(.*\) | todo |\$/| ${STUB_FLIP} | \1 | implemented |/" "$f" > "$f.tmp" && mv "$f.tmp" "$f"
      echo "reconcile: o/r (2 brief(s))"
      echo "reconcile --apply: wrote 1 row(s)"
      echo "  s/${STUB_FLIP} todo -> implemented   $f   [PR #7 (merged abc1234)]"
    fi ;;
  regen)
    if [ -n "${STUB_STRAY:-}" ]; then echo stray > NOTES.md; fi ;;
  *) exit "${STUB_LINT_RC:-0}" ;;
esac
`

// stubGh logs every call, and the GH_CONFIG_DIR it saw in $STUB_LOG/gh-env.log.
// `api` GETs print $STUB_PULLS: TSV lines of number, head repository, head ref
// and base ref — what the step's --jq projection prints — and the stub returns
// EVERY entry whatever the query says, the worst case of a head filter that
// also matches forks, so the step's own selection is what is tested (the
// projection itself is pinned by TestReconcileJobPRProjection). A call made
// holding the write token logs `token=write`.
const stubGh = `#!/usr/bin/env bash
echo "$*" >> "$STUB_LOG/gh.log"
echo "GH_CONFIG_DIR=${GH_CONFIG_DIR:-}" >> "$STUB_LOG/gh-env.log"
[ "${GH_TOKEN:-}" = WRITE-TOKEN-SENTINEL ] && echo "token=write" >> "$STUB_LOG/gh.log"
case "$1" in
  api)
    case " $* " in *" --method PATCH "*) exit 0 ;; esac
    [ "${STUB_GH_LIST_RC:-0}" = 0 ] || exit "$STUB_GH_LIST_RC"
    [ -z "${STUB_PULLS:-}" ] || printf '%s\n' "$STUB_PULLS" ;;
  pr)
    case "$2" in
      create) echo "https://example.invalid/pr/1" ;;
    esac ;;
esac
`

// ownPR is this repository's own open PR from the carried branch into main, as
// the step's --jq projection prints it.
const ownPR = "5\to/r\tboard/reconcile\tmain"

type jobRig struct {
	t         *testing.T
	dir       string
	forge     string // GITHUB_SERVER_URL's directory: origin is <forge>/o/r.git
	origin    string
	seed      string
	bin       string
	workspace string // the job's default working directory (GITHUB_WORKSPACE)
	userCfg   string // the runner user's git configuration
	env       []string
	ticks     int
	lastTemp  string // the last tick's RUNNER_TEMP
	// Hooks a test uses to move the forge between steps, as a concurrent
	// writer would.
	beforeCompute func()
	beforePublish func()
	// When set, the tick skips the compute step and hands this value to the
	// publish step as the compute step's `commit` output.
	forceCommit string
}

func requireJobTools(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the reconcile job runs on a Linux runner; its shell is not exercised on Windows")
	}
	for _, tool := range []string{"bash", "git", "sed"} {
		requireTool(t, tool)
	}
}

// requireTool skips a test whose tool is missing on a developer machine, but
// fails it under CI: on a runner a skip would read as a pass of a test that
// never ran.
func requireTool(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s not on PATH: under CI this test must run, not skip", tool)
		}
		t.Skipf("%s not on PATH", tool)
	}
}

// scrubbedEnv is the caller's environment minus any credential, runner or git
// variable a step could act on: the rig supplies each step's env itself.
func scrubbedEnv() []string {
	drop := map[string]bool{
		"GH_TOKEN": true, "GITHUB_TOKEN": true, "GH_ENTERPRISE_TOKEN": true,
		"GITHUB_ENTERPRISE_TOKEN": true, "GITHUB_OUTPUT": true, "GITHUB_ENV": true,
		"GITHUB_PATH": true, "COMMIT": true, "GH_CONFIG_DIR": true,
		"GITHUB_SERVER_URL": true, "RUNNER_TEMP": true, "GIT_CEILING_DIRECTORIES": true,
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_TEMPLATE_DIR": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); !drop[k] && !strings.HasPrefix(k, "GIT_CONFIG") {
			env = append(env, kv)
		}
	}
	return env
}

func newJobRig(t *testing.T) *jobRig {
	t.Helper()
	requireJobTools(t)
	r := &jobRig{t: t, dir: t.TempDir()}
	r.forge = filepath.Join(r.dir, "forge")
	r.origin = filepath.Join(r.forge, "o", "r.git")
	r.seed = filepath.Join(r.dir, "seed")
	r.bin = filepath.Join(r.dir, "bin")
	r.workspace = filepath.Join(r.dir, "workspace")
	for _, d := range []string{r.bin, r.workspace} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(r.bin, "gh"), []byte(stubGh), 0o755); err != nil {
		t.Fatal(err)
	}
	r.userCfg = filepath.Join(r.dir, "gitconfig")
	if err := os.WriteFile(r.userCfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r.env = append(scrubbedEnv(),
		"PATH="+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		// The runner user's git configuration: what a job inherits unless it
		// replaces it. The residue test plants hooks and rewrites here.
		"GIT_CONFIG_GLOBAL="+r.userCfg, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=job", "GIT_AUTHOR_EMAIL=job@example.invalid",
		"GIT_COMMITTER_NAME=job", "GIT_COMMITTER_EMAIL=job@example.invalid",
		"GITHUB_REPOSITORY=o/r", "GITHUB_SERVER_URL=file://"+r.forge,
		"STUB_LOG="+r.dir, "TMPDIR="+r.dir,
	)
	r.git("", "init", "-q", "--bare", "-b", "main", r.origin)
	r.git("", "init", "-q", "-b", "main", r.seed)
	r.write(r.seed, jobReadmeMain)
	r.git(r.seed, "add", "-A")
	r.git(r.seed, "commit", "-q", "-m", "seed")
	r.git(r.seed, "remote", "add", "origin", r.origin)
	r.git(r.seed, "push", "-q", "origin", "main")
	return r
}

func (r *jobRig) git(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *jobRig) write(repo, readme string) {
	r.t.Helper()
	p := filepath.Join(repo, "docs", "streams", "s", "README.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(readme), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commitOnMain lands a commit on origin/main from the seed clone.
func (r *jobRig) commitOnMain(readme, msg string) {
	r.t.Helper()
	r.git(r.seed, "pull", "-q", "--ff-only", "origin", "main")
	r.write(r.seed, readme)
	r.git(r.seed, "commit", "-q", "-am", msg)
	r.git(r.seed, "push", "-q", "origin", "main")
}

// reconcileJobWorkflow is the staged workflow whose reconcile job is under
// test. RECONCILEJOB_WORKFLOW points the tests at another copy of it — a
// mutant, in TestReconcileJobMutantsFail.
const reconcileJobWorkflow = "../ci/staged-workflows/assay-statusgen.yml"

func reconcileWorkflowPath() string {
	if p := os.Getenv("RECONCILEJOB_WORKFLOW"); p != "" {
		return p
	}
	return reconcileJobWorkflow
}

// The steps the tick runs; the tests fail if any is renamed away.
const (
	cloneStepName        = "Clone main into a fresh job-local directory"
	reconcileJobStepName = "Reconcile and regenerate the stream README tables"
	publishStepName      = "Push board/reconcile and open or refresh its draft PR"
	appTokenExpr         = "${{ steps.app-token.outputs.token }}"
	commitGateExpr       = "${{ steps.compute.outputs.commit != '' }}"
	runnerTempExpr       = "${{ runner.temp }}"
)

type wfStep struct {
	ID               string            `yaml:"id"`
	Name             string            `yaml:"name"`
	If               string            `yaml:"if"`
	Uses             string            `yaml:"uses"`
	With             map[string]any    `yaml:"with"`
	Env              map[string]string `yaml:"env"`
	WorkingDirectory string            `yaml:"working-directory"`
	Run              string            `yaml:"run"`
}

type wfJob struct {
	If          string            `yaml:"if"`
	Permissions any               `yaml:"permissions"`
	Env         map[string]string `yaml:"env"`
	Steps       []wfStep          `yaml:"steps"`
}

type workflow struct {
	On          map[string]any    `yaml:"on"`
	Concurrency map[string]any    `yaml:"concurrency"`
	Env         map[string]string `yaml:"env"`
	Jobs        map[string]wfJob  `yaml:"jobs"`
}

func parseWorkflow(t *testing.T, raw []byte, name string) workflow {
	t.Helper()
	var wf workflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return wf
}

func stagedWorkflow(t *testing.T) workflow {
	t.Helper()
	p := reconcileWorkflowPath()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return parseWorkflow(t, raw, p)
}

// reconcileStep returns the reconcile job's step of that name, exactly as the
// runner would execute it.
func reconcileStep(t *testing.T, name string) wfStep {
	t.Helper()
	for _, st := range stagedWorkflow(t).Jobs["reconcile"].Steps {
		if st.Name == name {
			if strings.TrimSpace(st.Run) == "" {
				t.Fatalf("%s: step %q has no run: text", reconcileWorkflowPath(), name)
			}
			return st
		}
	}
	t.Fatalf("%s: jobs.reconcile has no step named %q", reconcileWorkflowPath(), name)
	return wfStep{}
}

// stepEnv resolves a step's `env:` map the way the runner would, with the
// rig's sentinels for the token expressions and the compute step's outputs.
// An expression the rig does not know fails the test rather than pass blank.
func stepEnv(t *testing.T, st wfStep, outputs map[string]string) []string {
	t.Helper()
	known := map[string]string{
		"${{ github.token }}":                 readTokenSentinel,
		appTokenExpr:                          writeTokenSentinel,
		"${{ steps.compute.outputs.commit }}": outputs["commit"],
	}
	var env []string
	for k, v := range st.Env {
		if strings.Contains(v, "${{") {
			sub, ok := known[strings.TrimSpace(v)]
			if !ok {
				t.Fatalf("step %q env %s: the rig cannot resolve %q", st.Name, k, v)
			}
			v = sub
		}
		env = append(env, k+"="+v)
	}
	sort.Strings(env)
	return env
}

// scriptFile writes a step's run text to a file outside every repository.
func scriptFile(t *testing.T, run string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "step.sh")
	if err := os.WriteFile(p, []byte(run), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func (r *jobRig) runStep(dir, script string, env []string) (int, string) {
	r.t.Helper()
	cmd := exec.Command("bash", script)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, r.env...), env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		r.t.Fatal(err)
	}
	return code, string(out)
}

// readOutputs parses a GITHUB_OUTPUT (or GITHUB_ENV) file of key=value lines.
func readOutputs(t *testing.T, path string) map[string]string {
	t.Helper()
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			m[k] = v
		}
	}
	return m
}

// tick runs one job tick as the runner would: a fresh RUNNER_TEMP holding the
// statusgen binary (the stub); the clone step; the compute step; then — only
// when compute set its `commit` output, as the publish step's `if:` says — the
// publish step. Each step runs in its `working-directory:` (the workspace when
// it names none), with what earlier steps wrote to GITHUB_ENV, its own `env:`,
// and the test's extra env. Returns the exit code and combined output.
func (r *jobRig) tick(extra ...string) (int, string) {
	r.t.Helper()
	r.ticks++
	temp := filepath.Join(r.dir, fmt.Sprintf("runner-temp-%d", r.ticks))
	if err := os.MkdirAll(temp, 0o755); err != nil {
		r.t.Fatal(err)
	}
	r.lastTemp = temp
	if err := os.WriteFile(filepath.Join(temp, "statusgen"), []byte(stubStatusgen), 0o755); err != nil {
		r.t.Fatal(err)
	}
	ghEnv := filepath.Join(r.dir, fmt.Sprintf("github-env-%d", r.ticks))
	ghOut := filepath.Join(r.dir, fmt.Sprintf("github-output-%d", r.ticks))
	for _, f := range []string{ghEnv, ghOut} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
	run := func(name string, outputs map[string]string) (int, string) {
		st := reconcileStep(r.t, name)
		dir := r.workspace
		if wd := st.WorkingDirectory; wd != "" {
			dir = strings.ReplaceAll(wd, runnerTempExpr, temp)
			if strings.Contains(dir, "${{") {
				r.t.Fatalf("step %q working-directory: the rig cannot resolve %q", name, wd)
			}
		}
		env := []string{"RUNNER_TEMP=" + temp, "GITHUB_ENV=" + ghEnv, "GITHUB_OUTPUT=" + ghOut}
		for k, v := range readOutputs(r.t, ghEnv) {
			env = append(env, k+"="+v)
		}
		env = append(append(env, stepEnv(r.t, st, outputs)...), extra...)
		return r.runStep(dir, scriptFile(r.t, st.Run), env)
	}
	code, out := run(cloneStepName, nil)
	if code != 0 {
		return code, out
	}
	outputs := map[string]string{"commit": r.forceCommit}
	if r.forceCommit == "" {
		if r.beforeCompute != nil {
			r.beforeCompute()
		}
		var o string
		code, o = run(reconcileJobStepName, nil)
		out += o
		outputs = readOutputs(r.t, ghOut)
		if code != 0 || outputs["commit"] == "" {
			return code, out
		}
	}
	if r.beforePublish != nil {
		r.beforePublish()
	}
	code, o := run(publishStepName, outputs)
	return code, out + o
}

func (r *jobRig) remoteRef(ref string) string {
	r.t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", "-q", ref)
	cmd.Dir = r.origin
	cmd.Env = r.env
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func (r *jobRig) remoteFile(ref string) string {
	return r.git(r.origin, "show", ref+":docs/streams/s/README.md")
}

func (r *jobRig) ghLog() string {
	b, _ := os.ReadFile(filepath.Join(r.dir, "gh.log"))
	return string(b)
}

func (r *jobRig) statusgenLog() []string {
	b, _ := os.ReadFile(filepath.Join(r.dir, "statusgen.log"))
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// TestReconcileJobFreshBranch: no carried branch yet; a witnessed flip is
// committed on a branch cut from main and a DRAFT PR into main is opened.
func TestReconcileJobFreshBranch(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_FLIP=01")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(r.remoteFile("refs/heads/board/reconcile"), "| 01 | one | implemented |") {
		t.Fatalf("flip not carried:\n%s", r.remoteFile("refs/heads/board/reconcile"))
	}
	if !strings.Contains(r.ghLog(), "pr create --repo o/r --draft --base main --head board/reconcile") {
		t.Fatalf("a draft PR into main was not opened; gh log:\n%s", r.ghLog())
	}
	msg := r.git(r.origin, "log", "-1", "--format=%B", "refs/heads/board/reconcile")
	if !strings.Contains(msg, "PR #7") {
		t.Fatalf("the commit must name each flip's witness:\n%s", msg)
	}
}

// TestReconcileJobStatusgenArgs is review B6's trailer-only half at the job
// level: the step runs the reconcile WITHOUT --backfill, then the README
// re-render, then the full lint — exactly these invocations, in this order.
func TestReconcileJobStatusgenArgs(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	want := []string{"reconcile --apply --root . --repo o/r", "regen --readmes --root .", "--root . --lint"}
	if got := r.statusgenLog(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("statusgen invocations:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestReconcileJobSurvivesConflictingMainChange is review B3: a routine change
// on main touching the same README lines the branch flipped used to make the
// tick's `git merge` conflict and exit 1, every hour, until a human cleared the
// branch. The tick now recomputes on main's tree and carries the result as a
// fast-forward of the branch.
func TestReconcileJobSurvivesConflictingMainChange(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	oldTip := r.remoteRef("refs/heads/board/reconcile")
	// Main renames brief 01 on the very line the branch flipped.
	r.commitOnMain(strings.Replace(jobReadmeMain, "| 01 | one | todo |", "| 01 | one, renamed | todo |", 1), "docs: rename brief 01")

	code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+ownPR)
	if code != 0 {
		t.Fatalf("a routine main change wedged the tick (exit %d):\n%s", code, out)
	}
	newTip := r.remoteRef("refs/heads/board/reconcile")
	if newTip == oldTip {
		t.Fatal("the branch was not updated")
	}
	if got := r.remoteFile("refs/heads/board/reconcile"); !strings.Contains(got, "| 01 | one, renamed | implemented |") {
		t.Fatalf("the branch must carry main's change AND the flip:\n%s", got)
	}
	// Fast-forward (never forced), and main is now an ancestor (PR mergeable).
	r.git(r.origin, "merge-base", "--is-ancestor", oldTip, newTip)
	r.git(r.origin, "merge-base", "--is-ancestor", "refs/heads/main", newTip)
}

// TestReconcileJobComputesOnFetchedMain pins the compute step's detached
// checkout of the main it just fetched: main moves after the clone step, and
// the carried tree is still computed from main as fetched by the compute step.
func TestReconcileJobComputesOnFetchedMain(t *testing.T) {
	r := newJobRig(t)
	r.beforeCompute = func() {
		r.commitOnMain(strings.Replace(jobReadmeMain, "| 01 | one | todo |", "| 01 | one, renamed | todo |", 1), "docs: rename brief 01")
	}
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if got := r.remoteFile("refs/heads/board/reconcile"); !strings.Contains(got, "| 01 | one, renamed | implemented |") {
		t.Fatalf("the tree must be computed on the fetched main, not the clone's checkout:\n%s", got)
	}
}

// TestReconcileJobNeverForces is review B6's "never forced": a writer that moves
// the branch between this tick's compute and publish steps wins; the tick's
// push is rejected, the tick fails, and the other writer's commit stays.
func TestReconcileJobNeverForces(t *testing.T) {
	r := newJobRig(t)
	var racer string
	r.beforePublish = func() {
		other := filepath.Join(r.dir, "racer")
		r.git("", "clone", "-q", r.origin, other)
		r.write(other, strings.Replace(jobReadmeMain, "| 02 | two | todo |", "| 02 | two | implemented |", 1))
		r.git(other, "commit", "-q", "-am", "chore(board): reconcile (desk)")
		r.git(other, "push", "-q", "origin", "HEAD:refs/heads/board/reconcile")
		racer = r.git(other, "rev-parse", "HEAD")
	}
	code, out := r.tick("STUB_FLIP=01")
	if code == 0 {
		t.Fatalf("a push that lost the race must fail the tick:\n%s", out)
	}
	if got := r.remoteRef("refs/heads/board/reconcile"); got != racer {
		t.Fatalf("the other writer's commit was overwritten: branch at %s, racer %s\n%s", got, racer, out)
	}
	if strings.Contains(r.ghLog(), "pr create") {
		t.Fatalf("no PR may be opened after a rejected push:\n%s", r.ghLog())
	}
}

// TestReconcileJobRefusesForeignCommit: a commit on the branch that this job
// did not make is never overwritten — the tick fails loudly and pushes nothing.
func TestReconcileJobRefusesForeignCommit(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	before := r.handCommit("NOTES.md", "wip: my notes")
	r.commitOnMain(jobReadmeMain+"\nmore\n", "docs: main moves")

	code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+ownPR)
	if code == 0 {
		t.Fatalf("a foreign commit on the branch must fail the tick:\n%s", out)
	}
	if !strings.Contains(out, "wip: my notes") {
		t.Fatalf("the refusal must name the foreign commit:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != before {
		t.Fatal("the branch carrying a foreign commit was overwritten")
	}
}

// TestReconcileJobRefusesNonReadmeCommit is review B6's README-only half: a
// commit with this job's own subject that touches more than stream READMEs is
// refused — the subject alone never makes a commit this job's. The boundary is
// a stream's own README: a path outside docs/streams/, a brief file beside the
// README, and a README one directory further down are each refused.
func TestReconcileJobRefusesNonReadmeCommit(t *testing.T) {
	for _, file := range []string{"NOTES.md", "docs/streams/s/brief-01.md", "docs/streams/s/sub/README.md"} {
		t.Run(file, func(t *testing.T) {
			r := newJobRig(t)
			if code, out := r.tick("STUB_FLIP=01"); code != 0 {
				t.Fatalf("first tick exit %d:\n%s", code, out)
			}
			before := r.handCommit(file, "chore(board): reconcile 2026-10-08")
			r.commitOnMain(jobReadmeMain+"\nmore\n", "docs: main moves")

			code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+ownPR)
			if code == 0 {
				t.Fatalf("a same-subject commit touching %s must fail the tick:\n%s", file, out)
			}
			if !strings.Contains(out, "(touches more than stream READMEs)") {
				t.Fatalf("the refusal must say the commit touches more than stream READMEs:\n%s", out)
			}
			if r.remoteRef("refs/heads/board/reconcile") != before {
				t.Fatalf("the branch carrying a commit touching %s was overwritten", file)
			}
		})
	}
}

// TestReconcileJobForeignSubject is review B6's subject half, one parent: a
// commit whose change is the stream README only, but whose subject is not this
// job's, is refused and named — the change alone never makes a commit this
// job's, so a hand edit of the table is never silently replaced.
func TestReconcileJobForeignSubject(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	before := r.handCommit("docs/streams/s/README.md", "wip: hand edit of the table")
	r.commitOnMain(jobReadmeMain+"\nmore\n", "docs: main moves")

	code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+ownPR)
	if code == 0 {
		t.Fatalf("a README-only commit with another subject must fail the tick:\n%s", out)
	}
	if !strings.Contains(out, "wip: hand edit of the table") {
		t.Fatalf("the refusal must name the commit:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != before {
		t.Fatal("the branch carrying a hand edit of the table was overwritten")
	}
}

// TestReconcileJobForeignMerge is review B6's subject half, two parents: a
// merge of main whose change against main is the stream README only, but
// whose subject is neither this job's nor git's default `Merge ...`, is
// refused and named. TestReconcileJobAcceptsDeskSideMerge is its other side.
func TestReconcileJobForeignMerge(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	r.git(r.seed, "pull", "-q", "--ff-only", "origin", "main")
	if err := os.WriteFile(filepath.Join(r.seed, "CODE.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git(r.seed, "add", "-A")
	r.git(r.seed, "commit", "-q", "-m", "feat: unrelated code")
	r.git(r.seed, "push", "-q", "origin", "main")
	hand := filepath.Join(r.dir, "hand-merge")
	r.git("", "clone", "-q", "-b", "board/reconcile", r.origin, hand)
	r.git(hand, "merge", "-q", "-m", "wip: hand merge of main", "origin/main")
	r.git(hand, "push", "-q", "origin", "board/reconcile")
	before := r.remoteRef("refs/heads/board/reconcile")

	code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+ownPR)
	if code == 0 {
		t.Fatalf("a merge with another subject must fail the tick:\n%s", out)
	}
	if !strings.Contains(out, "wip: hand merge of main") {
		t.Fatalf("the refusal must name the merge:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != before {
		t.Fatal("the branch carrying a foreign merge was overwritten")
	}
}

// handCommit pushes a commit writing file onto the carried branch, as another
// writer would, and returns the new branch tip.
func (r *jobRig) handCommit(file, subject string) string {
	r.t.Helper()
	hand := filepath.Join(r.dir, "hand")
	_ = os.RemoveAll(hand)
	r.git("", "clone", "-q", "-b", "board/reconcile", r.origin, hand)
	p := filepath.Join(hand, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("hand edit\n"), 0o644); err != nil {
		r.t.Fatal(err)
	}
	r.git(hand, "add", "-A")
	r.git(hand, "commit", "-q", "-m", subject)
	r.git(hand, "push", "-q", "origin", "board/reconcile")
	return r.remoteRef("refs/heads/board/reconcile")
}

// TestReconcileJobRefusesStrayPath is review B6's "only stream READMEs may
// change": a statusgen run that writes any other path fails the tick, naming
// it, and nothing is pushed.
func TestReconcileJobRefusesStrayPath(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_FLIP=01", "STUB_STRAY=1")
	if code == 0 {
		t.Fatalf("a run that changed NOTES.md must fail the tick:\n%s", out)
	}
	if !strings.Contains(out, "changed paths other than stream READMEs") || !strings.Contains(out, "NOTES.md") {
		t.Fatalf("the refusal must name the stray path:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != "" || r.ghLog() != "" {
		t.Fatalf("nothing may be pushed or opened; gh log:\n%s", r.ghLog())
	}
}

// TestReconcileJobCouldNotCheckFails is review A1 at the job level: a reconcile
// that could not read the PRs fails the tick and pushes nothing.
func TestReconcileJobCouldNotCheckFails(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_FLIP=01", "STUB_RECONCILE_RC=3")
	if code == 0 {
		t.Fatalf("a could-not-check reconcile must fail the tick:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != "" {
		t.Fatal("nothing may be pushed on a could-not-check")
	}
}

// TestReconcileJobPRListFailureFails: a PR list that could not be read is a
// could-not-check, never "a PR is already open".
func TestReconcileJobPRListFailureFails(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_FLIP=01", "STUB_GH_LIST_RC=1")
	if code == 0 {
		t.Fatalf("an unreadable PR list must fail the tick:\n%s", out)
	}
}

// TestReconcileJobRejectsBadCommitOutput pins the publish step's check of the
// compute step's output: a value that is not an object name is refused before
// git sees it.
func TestReconcileJobRejectsBadCommitOutput(t *testing.T) {
	r := newJobRig(t)
	r.forceCommit = "zz"
	code, out := r.tick()
	if code == 0 || !strings.Contains(out, "is not an object name") {
		t.Fatalf("a commit output that is not an object name must be refused by the step's own check (exit %d):\n%s", code, out)
	}
}

// TestReconcileJobNoChangeNoPush: nothing changed state — no branch, no PR.
func TestReconcileJobNoChangeNoPush(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick()
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != "" || r.ghLog() != "" {
		t.Fatalf("a no-change tick must push and open nothing; gh log:\n%s", r.ghLog())
	}
}

// TestReconcileJobLintFailureFails: the lint step is never bypassed — a tree
// the lint rejects is not committed.
func TestReconcileJobLintFailureFails(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_FLIP=01", "STUB_LINT_RC=1")
	if code == 0 {
		t.Fatalf("a lint failure must fail the tick:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != "" {
		t.Fatal("a tree the lint rejected was pushed")
	}
}

// TestReconcileJobRemoteListingFails is review A1's transport half: the old
// step ran `git ls-remote --exit-code ... >/dev/null 2>&1`, so a transport
// error read as "branch absent" — the tick re-cut the branch from main and
// failed later, if at all, on an unrelated-looking push rejection with the real
// cause discarded. A failed listing now fails the tick AT the listing, with
// git's own error in the log, and nothing is pushed.
func TestReconcileJobRemoteListingFails(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	before := r.remoteRef("refs/heads/board/reconcile")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrap := filepath.Join(r.dir, "gitwrap")
	if err := os.MkdirAll(wrap, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := "#!/usr/bin/env bash\nif [ \"${1:-}\" = ls-remote ]; then echo 'fatal: unable to access remote: Could not resolve host' >&2; exit 128; fi\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(wrap, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	r.commitOnMain(jobReadmeMain+"\nmore\n", "docs: main moves")
	code, out := r.tick("STUB_FLIP=02", "STUB_PULLS="+ownPR, "PATH="+wrap+string(os.PathListSeparator)+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if code == 0 {
		t.Fatalf("a remote that could not be listed must fail the tick:\n%s", out)
	}
	if !strings.Contains(out, "Could not resolve host") {
		t.Fatalf("the listing's own error must reach the log, not be silenced:\n%s", out)
	}
	if strings.Contains(out, "rejected") {
		t.Fatalf("the tick must stop at the failed listing, not go on to a push:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != before {
		t.Fatal("the branch moved on a tick that could not list the remote")
	}
}

// TestReconcileJobAcceptsDeskSideMerge: the desk-side writer on the same branch
// merges main with git's default "Merge ..." commit. That commit, whose change
// against main is stream READMEs only, is this writer family's own — the tick
// carries on from it rather than refusing.
func TestReconcileJobAcceptsDeskSideMerge(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	// Main moves on a non-README path, and the desk-side writer merges it in.
	r.git(r.seed, "pull", "-q", "--ff-only", "origin", "main")
	if err := os.WriteFile(filepath.Join(r.seed, "CODE.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git(r.seed, "add", "-A")
	r.git(r.seed, "commit", "-q", "-m", "feat: unrelated code")
	r.git(r.seed, "push", "-q", "origin", "main")
	desk := filepath.Join(r.dir, "desk")
	r.git("", "clone", "-q", "-b", "board/reconcile", r.origin, desk)
	r.git(desk, "merge", "-q", "--no-edit", "origin/main")
	r.git(desk, "push", "-q", "origin", "board/reconcile")
	before := r.remoteRef("refs/heads/board/reconcile")

	code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+ownPR)
	if code != 0 {
		t.Fatalf("the desk-side writer's merge commit must not read as foreign (exit %d):\n%s", code, out)
	}
	if after := r.remoteRef("refs/heads/board/reconcile"); after != before {
		t.Fatalf("nothing changed state, so the branch must not move (%s -> %s)", before, after)
	}
}

// TestReconcileJobIgnoresForkPR is review SEC-2372-1: the old step asked the
// forge for open PRs by bare head branch name, which also returns a fork's PR
// from a same-named branch. A look-alike made every tick push the branch, open
// no PR, report "an open PR already carries" it and exit 0. Only this
// repository's own branch counts as our PR now; a look-alike — including one
// whose fork was deleted (no head repository), and one from this repository's
// own branch of another name — is ignored and our PR is opened.
func TestReconcileJobIgnoresForkPR(t *testing.T) {
	r := newJobRig(t)
	lookalikes := "9\toutsider/r\tboard/reconcile\tmain\n10\t-\tboard/reconcile\tmain\n11\to/r\tboard/reconcile-x\tmain"
	code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+lookalikes)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(r.ghLog(), "pr create") {
		t.Fatalf("a look-alike PR was read as ours — no PR opened:\n%s\ngh log:\n%s", out, r.ghLog())
	}
	if strings.Contains(r.ghLog(), "--method PATCH") {
		t.Fatalf("a look-alike PR was edited:\n%s", r.ghLog())
	}
	for _, n := range []string{"#9", "#10", "#11"} {
		if !strings.Contains(out, "ignoring open PR "+n+": its head is not") {
			t.Fatalf("the ignored look-alike %s must be named in the log:\n%s", n, out)
		}
	}
}

// TestReconcileJobPRIdentity is review SEC-2372-1's second round: anyone can
// open a PR FROM this repository's own board/reconcile into any base. One into
// another base is never adopted (named and skipped, and our own PR into main is
// opened); two PRs into main from this head fail the tick rather than pick one.
func TestReconcileJobPRIdentity(t *testing.T) {
	t.Run("other base", func(t *testing.T) {
		r := newJobRig(t)
		code, out := r.tick("STUB_FLIP=01", "STUB_PULLS=12\to/r\tboard/reconcile\trelease")
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "ignoring open PR #12: it targets release, not main") {
			t.Fatalf("a PR from our head into another base must be named and skipped:\n%s", out)
		}
		if strings.Contains(r.ghLog(), "--method PATCH") || !strings.Contains(r.ghLog(), "pr create --repo o/r --draft --base main") {
			t.Fatalf("a PR into another base was adopted:\n%s", r.ghLog())
		}
	})
	t.Run("two into main", func(t *testing.T) {
		r := newJobRig(t)
		code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+ownPR+"\n13\to/r\tboard/reconcile\tmain")
		if code == 0 {
			t.Fatalf("two PRs carrying the branch into main must fail the tick:\n%s", out)
		}
		if !strings.Contains(out, "#5 and #13 both carry") {
			t.Fatalf("the refusal must name both PRs:\n%s", out)
		}
		if strings.Contains(r.ghLog(), "--method PATCH") || strings.Contains(r.ghLog(), "pr create") {
			t.Fatalf("no PR may be edited or opened when two are selected:\n%s", r.ghLog())
		}
	})
}

// TestReconcileJobPRQuery pins the list call: paginated, open PRs only,
// owner-qualified head, and the four-field projection the selection reads.
func TestReconcileJobPRQuery(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	want := `api --paginate repos/o/r/pulls?state=open&per_page=100&head=o:board/reconcile --jq ` + prProjection
	if !strings.Contains(r.ghLog(), want) {
		t.Fatalf("the PR list call must be\n  %s\ngh log:\n%s", want, r.ghLog())
	}
}

// prProjection is the publish step's --jq projection: number, head repository
// ("-" when the head repository is gone, so the tab-split read keeps four
// fields), head ref and base ref.
const prProjection = `.[] | [.number, (.head.repo.full_name // "-"), .head.ref, .base.ref] | @tsv`

// TestReconcileJobPRProjection runs the projection with real jq over a forge-
// shaped list: our PR, a fork's, a deleted fork's and one into another base
// each come out as the four fields the selection reads. Skipped without jq on
// a developer machine; under CI a missing jq fails it.
func TestReconcileJobPRProjection(t *testing.T) {
	requireTool(t, "jq")
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Fatal(err)
	}
	const list = `[
 {"number":5,"head":{"repo":{"full_name":"o/r"},"ref":"board/reconcile"},"base":{"ref":"main"}},
 {"number":9,"head":{"repo":{"full_name":"outsider/r"},"ref":"board/reconcile"},"base":{"ref":"main"}},
 {"number":10,"head":{"repo":null,"ref":"board/reconcile"},"base":{"ref":"main"}},
 {"number":12,"head":{"repo":{"full_name":"o/r"},"ref":"board/reconcile"},"base":{"ref":"release"}}
]`
	cmd := exec.Command(jq, "-r", prProjection)
	cmd.Stdin = strings.NewReader(list)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "5\to/r\tboard/reconcile\tmain\n9\toutsider/r\tboard/reconcile\tmain\n10\t-\tboard/reconcile\tmain\n12\to/r\tboard/reconcile\trelease\n"
	if string(out) != want {
		t.Fatalf("projection output:\n%q\nwant:\n%q", out, want)
	}
}

// TestReconcileJobRefreshesBody is review SEC-2372-4's second half: the PR text
// was written once, at creation, and never refreshed. Each push to the branch
// now rewrites our PR's title and body from that tick's report.
func TestReconcileJobRefreshesBody(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	r.commitOnMain(jobReadmeMain+"\nmore\n", "docs: main moves")
	code, out := r.tick("STUB_FLIP=02", "STUB_PULLS="+ownPR)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(r.ghLog(), "api --method PATCH repos/o/r/pulls/5 -f title=chore(board): reconcile -F body=@") {
		t.Fatalf("our open PR's title and body were not rewritten:\n%s", r.ghLog())
	}
	if n := strings.Count(r.ghLog(), "pr create"); n != 1 {
		t.Fatalf("pr create ran %d times; once, on the first tick, is right:\n%s", n, r.ghLog())
	}
	body, err := os.ReadFile(filepath.Join(r.lastTemp, "reconcile-pr-body.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "s/02 todo -> implemented") {
		t.Fatalf("the refreshed body must carry this tick's report:\n%s", body)
	}
}

// TestReconcileJobShowsHeldRows is review SEC-2372-4's first half: on a tick
// where every witnessed row is held, the report went only to a temp file and
// the log said only that nothing changed. The report now reaches the log.
func TestReconcileJobShowsHeldRows(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_HOLD=01")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "held 1 row(s)") || !strings.Contains(out, "s/01") || !strings.Contains(out, "would add: risk-gated brief") {
		t.Fatalf("a held row and its reason must reach the job log:\n%s", out)
	}
	if r.remoteRef("refs/heads/board/reconcile") != "" || r.ghLog() != "" {
		t.Fatalf("a held-only tick carries nothing; gh log:\n%s", r.ghLog())
	}
}

// TestReconcileJobTokenScope is review SEC-2372-2's credential half, run: no
// statusgen invocation holds the write token, and the PR calls do.
func TestReconcileJobTokenScope(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_FLIP=01", "STUB_CHECK_WRITE=1")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(r.ghLog(), "token=write") {
		t.Fatalf("the publish step's gh calls must hold the write token:\n%s", r.ghLog())
	}
}

// TestReconcileJobIgnoresRunnerResidue is review SEC-2372-2's second round:
// git state an earlier job could leave on a reused runner — a repository in the
// workspace with hooks and a push address, and the runner user's own git
// configuration with a hooks path and URL rewrites — takes no effect on the
// tick. No hook runs, the push reaches the real origin and not the decoy, and
// gh reads a configuration directory of the job's own.
func TestReconcileJobIgnoresRunnerResidue(t *testing.T) {
	r := newJobRig(t)
	marker := filepath.Join(r.dir, "hook-ran")
	hook := "#!/usr/bin/env bash\necho \"$0 ${GH_TOKEN:-}\" >> " + marker + "\n"
	writeHooks := func(dir string) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, h := range []string{"pre-push", "post-checkout", "reference-transaction", "pre-commit", "post-commit", "post-merge"} {
			if err := os.WriteFile(filepath.Join(dir, h), []byte(hook), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	decoyForge := filepath.Join(r.dir, "decoy")
	decoy := filepath.Join(decoyForge, "o", "r.git")
	r.git("", "init", "-q", "--bare", "-b", "main", decoy)
	forgeURL := "file://" + r.forge + "/"
	decoyURL := "file://" + decoyForge + "/"

	// The workspace repository an earlier job left behind.
	r.git("", "clone", "-q", r.origin, r.workspace)
	writeHooks(filepath.Join(r.workspace, ".git", "hooks"))
	r.git(r.workspace, "config", "remote.origin.pushurl", decoyURL+"o/r.git")
	r.git(r.workspace, "config", "url."+decoyURL+".pushInsteadOf", forgeURL)

	// The runner user's git configuration. Planted after the rig's own setup,
	// which reads it too.
	userHooks := filepath.Join(r.dir, "user-hooks")
	writeHooks(userHooks)
	r.git("", "config", "--file", r.userCfg, "core.hooksPath", userHooks)
	r.git("", "config", "--file", r.userCfg, "url."+decoyURL+".pushInsteadOf", forgeURL)
	r.git("", "config", "--file", r.userCfg, "remote.origin.pushurl", decoyURL+"o/r.git")

	code, out := r.tick("STUB_FLIP=01")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Fatalf("a hook left by an earlier job ran during the tick:\n%s", b)
	}
	if got := r.remoteRef("refs/heads/board/reconcile"); got == "" {
		t.Fatalf("the push did not reach the real origin:\n%s", out)
	}
	cmd := exec.Command("git", "rev-parse", "--verify", "-q", "refs/heads/board/reconcile")
	cmd.Dir = decoy
	cmd.Env = r.env
	if o, _ := cmd.Output(); len(strings.TrimSpace(string(o))) != 0 {
		t.Fatal("the push was sent to the decoy a planted push address names")
	}
	envLog, _ := os.ReadFile(filepath.Join(r.dir, "gh-env.log"))
	for _, line := range strings.Split(strings.TrimSpace(string(envLog)), "\n") {
		if !strings.HasPrefix(line, "GH_CONFIG_DIR="+r.lastTemp+string(filepath.Separator)) {
			t.Fatalf("gh must read a configuration directory under the job's RUNNER_TEMP, got %q", line)
		}
	}
}

// TestReconcileJobInstallVerifies runs the job's two download steps (Go and gh)
// from the YAML with stub curl, sha256sum and tar: a tarball whose digest is
// not the pin is refused and never extracted; the pinned digest is extracted.
// The isolation guard reads the same gate as text; this pins what it does.
func TestReconcileJobInstallVerifies(t *testing.T) {
	requireJobTools(t)
	wf := stagedWorkflow(t)
	ghPin := regexp.MustCompile(`want="([0-9a-f]{64})"`)
	for _, c := range []struct{ step, pin string }{
		{"Install Go (job-local toolchain and caches, checksum-verified)", wf.Env["GO_LINUX_AMD64_SHA256"]},
		{"Install gh CLI (pinned, checksum-verified)", ""},
	} {
		st := reconcileStep(t, c.step)
		pin := c.pin
		if pin == "" {
			m := ghPin.FindStringSubmatch(st.Run)
			if m == nil {
				t.Fatalf("step %q pins no 64-hex digest", c.step)
			}
			pin = m[1]
		}
		for _, sum := range []string{strings.Repeat("0", 64), pin} {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			temp := filepath.Join(dir, "temp")
			for _, d := range []string{bin, temp} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stubs := map[string]string{
				"curl":      "#!/usr/bin/env bash\nwhile [ $# -gt 0 ]; do if [ \"$1\" = -o ]; then echo tarball > \"$2\"; fi; shift; done\n",
				"sha256sum": "#!/usr/bin/env bash\necho \"$STUB_SUM  $1\"\n",
				"tar":       "#!/usr/bin/env bash\necho \"$*\" >> \"$STUB_LOG/tar.log\"\n",
			}
			for name, body := range stubs {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", scriptFile(t, st.Run))
			cmd.Dir = dir
			cmd.Env = append(scrubbedEnv(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"RUNNER_TEMP="+temp, "GITHUB_PATH="+filepath.Join(dir, "path"), "GITHUB_ENV="+filepath.Join(dir, "env"),
				"GO_VERSION="+wf.Env["GO_VERSION"], "GO_LINUX_AMD64_SHA256="+wf.Env["GO_LINUX_AMD64_SHA256"],
				"STUB_SUM="+sum, "STUB_LOG="+dir)
			out, err := cmd.CombinedOutput()
			_, tarErr := os.Stat(filepath.Join(dir, "tar.log"))
			extracted := tarErr == nil
			if sum == pin {
				if err != nil || !extracted {
					t.Errorf("step %q with the pinned digest: err %v, extracted %v\n%s", c.step, err, extracted, out)
				}
				continue
			}
			if err == nil || extracted || !strings.Contains(string(out), "refusing to extract") {
				t.Errorf("step %q with a digest that is not the pin: err %v, extracted %v — a mismatch must exit nonzero before tar\n%s", c.step, err, extracted, out)
			}
		}
	}
}

// TestReconcileJobTriggerAndScope pins the job's guards, read from the YAML:
// the schedule-and-repository `if:` (review A5), the read-only job token, and
// the mint narrowed to the publish step's two grants. And the workflow-level
// wiring the job depends on: the hourly schedule, the scheduled runs' own
// concurrency group (one reconcile at a time, never queued with a push regen),
// the PR lint step that runs these tests, and the pull_request path entries
// that trigger it on a change to statusgen or to this staged file.
func TestReconcileJobTriggerAndScope(t *testing.T) {
	wf := stagedWorkflow(t)
	if got := fmt.Sprint(wf.On["schedule"]); got != "[map[cron:17 * * * *]]" {
		t.Errorf("on.schedule: %s, want exactly one cron \"17 * * * *\"", got)
	}
	if g := fmt.Sprint(wf.Concurrency["group"]); !strings.Contains(g, "github.event_name == 'schedule' && 'reconcile' ||") {
		t.Errorf("concurrency group %q gives scheduled runs no group of their own", g)
	}
	if c := fmt.Sprint(wf.Concurrency["cancel-in-progress"]); c != "false" {
		t.Errorf("concurrency cancel-in-progress: %s, want false", c)
	}
	pr, _ := wf.On["pull_request"].(map[string]any)
	paths := fmt.Sprint(pr["paths"])
	for _, need := range []string{"statusgen/**", "ci/staged-workflows/assay-statusgen.yml"} {
		if !strings.Contains(" "+strings.Trim(paths, "[]")+" ", " "+need+" ") {
			t.Errorf("on.pull_request.paths %s lacks %q: a change there would not run the job tests", paths, need)
		}
	}
	const lintRun = `go test -count=1 -timeout 600s -run '^(TestReconcileJob|TestCredentialedJobIsolation|TestIsolationGuardFlagsPlant|TestStatusKeyedLintRulesRegistered)' .`
	ran := false
	for _, st := range wf.Jobs["lint"].Steps {
		ran = ran || strings.Contains(st.Run, lintRun)
	}
	if !ran {
		t.Errorf("no lint job step runs\n  %s", lintRun)
	}

	job := wf.Jobs["reconcile"]
	if want := "${{ github.event_name == 'schedule' && github.repository == 'medici-finance/assay' }}"; job.If != want {
		t.Errorf("reconcile if: %q, want %q", job.If, want)
	}
	perms, _ := job.Permissions.(map[string]any)
	want := map[string]any{"contents": "read", "pull-requests": "read", "issues": "read"}
	if fmt.Sprint(perms) != fmt.Sprint(want) {
		t.Errorf("reconcile permissions: %v, want exactly %v", job.Permissions, want)
	}
	minted := false
	for _, st := range job.Steps {
		if !strings.HasPrefix(st.Uses, "actions/create-github-app-token@") {
			continue
		}
		minted = true
		grants := map[string]string{}
		for k, v := range st.With {
			if strings.HasPrefix(k, "permission-") {
				grants[k] = fmt.Sprint(v)
			}
			if k == "owner" || k == "repositories" {
				t.Errorf("the mint must cover this repository only; it sets %s: %v", k, v)
			}
		}
		if fmt.Sprint(grants) != fmt.Sprint(map[string]string{"permission-contents": "write", "permission-pull-requests": "write"}) {
			t.Errorf("mint grants %v, want exactly permission-contents: write and permission-pull-requests: write", grants)
		}
	}
	if !minted {
		t.Fatal("the reconcile job mints no App token")
	}
}

// TestReconcileJobStepWiring is review SEC-2372-2's credential half, read from
// the YAML: the job checks nothing out into the workspace; the App token is
// minted after every step that touches statusgen, only when the compute step
// produced a commit, and is referenced by the publish step alone, which runs no
// statusgen.
func TestReconcileJobStepWiring(t *testing.T) {
	job := stagedWorkflow(t).Jobs["reconcile"]
	mint, compute, publish, lastStatusgen := -1, -1, -1, -1
	for i, st := range job.Steps {
		switch {
		case strings.HasPrefix(st.Uses, "actions/checkout@"):
			t.Errorf("step %d uses actions/checkout, which keeps a workspace repository an earlier job left", i)
		case strings.HasPrefix(st.Uses, "actions/create-github-app-token@"):
			mint = i
			if st.If != commitGateExpr {
				t.Errorf("mint step if: %q, want %q", st.If, commitGateExpr)
			}
		case st.Name == reconcileJobStepName:
			compute = i
			if st.ID != "compute" {
				t.Errorf("compute step id %q, want compute", st.ID)
			}
		case st.Name == publishStepName:
			publish = i
			if st.If != commitGateExpr {
				t.Errorf("publish step if: %q, want %q", st.If, commitGateExpr)
			}
		}
		if strings.Contains(st.Run, "statusgen") && st.Name != publishStepName {
			lastStatusgen = i
		}
		holds := false
		for _, v := range st.Env {
			holds = holds || strings.Contains(v, "steps.app-token.outputs.token")
		}
		for _, v := range st.With {
			holds = holds || strings.Contains(fmt.Sprint(v), "steps.app-token.outputs.token")
		}
		if holds && st.Name != publishStepName {
			t.Errorf("step %q references the App token; only %q may", st.Name, publishStepName)
		}
		if _, ok := st.Env["GH_TOKEN"]; ok && st.Name != publishStepName {
			t.Errorf("step %q sets GH_TOKEN; only %q may", st.Name, publishStepName)
		}
	}
	if mint < 0 || compute < 0 || publish < 0 {
		t.Fatalf("reconcile job steps not found (mint %d, compute %d, publish %d)", mint, compute, publish)
	}
	if mint <= lastStatusgen || mint <= compute || publish <= mint {
		t.Errorf("order: mint (%d) must follow every statusgen step (last %d) and compute (%d), and precede publish (%d)", mint, lastStatusgen, compute, publish)
	}
	if strings.Contains(job.Steps[publish].Run, "statusgen") {
		t.Errorf("the publish step holds the write token and must run no statusgen")
	}
	if job.Steps[publish].Env["GH_TOKEN"] != appTokenExpr {
		t.Errorf("publish GH_TOKEN = %q, want %q", job.Steps[publish].Env["GH_TOKEN"], appTokenExpr)
	}
}

// Class guard for SEC-2372-2 — the defect class: a job that mints the
// board-writer App token (a bypass actor on the default branch) on the shared
// runner label, without the corroborate job's toolchain isolation and without
// git and gh state of its own. knownUnisolated lists the jobs that are live
// today with that shape; each is a maintainer follow-up on the live workflow
// and outside this change. The list may only shrink: a listed job that is now
// isolated, or no longer exists, fails the guard too. The guard reads the
// YAML's text; what that text does is pinned by the job tests above, which run
// the reconcile job's steps.
var knownUnisolated = map[string]bool{
	"regen":          true,
	"model-autoflip": true,
}

// invokes reports whether a run text calls tool as a command on some line.
func invokes(run, tool string) bool {
	re := regexp.MustCompile(`(^|[;&|(]\s*|\$\(\s*|\b(if|then|do|exec)\s+)` + regexp.QuoteMeta(tool) + `\s`)
	for _, line := range strings.Split(run, "\n") {
		if re.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

// mismatchRefusal is a step's checksum gate: the digest taken with sha256sum,
// compared with the pin, and a mismatch exiting nonzero before anything else
// runs (only echo lines between the comparison and the exit).
var mismatchRefusal = regexp.MustCompile(`(?s)sha256sum[^\n]*\n(?:[^\n]*\n)*?[ \t]*if \[ "[^"\n]+" != "\$\{?got\}?" \]; then\n(?:[ \t]*echo[^\n]*\n)*[ \t]*exit 1\n[ \t]*fi\n`)

// refusesMismatchFirst reports whether a download step refuses a checksum
// mismatch before its first tar extraction.
func refusesMismatchFirst(run string) bool {
	gate := mismatchRefusal.FindStringIndex(run)
	tar := regexp.MustCompile(`(?m)^\s*tar\s`).FindStringIndex(run)
	return gate != nil && (tar == nil || tar[0] >= gate[1])
}

// isolationProblems names every way a credentialed job departs from the
// isolation the reconcile job is held to. A job that mints no App token
// returns nil.
func isolationProblems(wf workflow, job wfJob) []string {
	credentialed := false
	for _, st := range job.Steps {
		credentialed = credentialed || strings.HasPrefix(st.Uses, "actions/create-github-app-token@")
	}
	if !credentialed {
		return nil
	}
	var p []string
	if job.Env["GOENV"] != "off" {
		p = append(p, "job env GOENV is not off")
	}
	if job.Env["GOTOOLCHAIN"] != "local" {
		p = append(p, "job env GOTOOLCHAIN is not local")
	}
	if wf.Env["GO_LINUX_AMD64_SHA256"] == "" {
		p = append(p, "workflow env pins no GO_LINUX_AMD64_SHA256")
	}
	callsGh, pinnedGh, ownGitCfg, gitBeforeCfg := false, false, false, false
	for _, st := range job.Steps {
		run := st.Run
		if strings.HasPrefix(st.Uses, "actions/checkout@") {
			p = append(p, "uses actions/checkout, which keeps a workspace repository (its hooks and local configuration) an earlier job left")
			if fmt.Sprint(st.With["persist-credentials"]) != "false" {
				p = append(p, "checkout persists a credential")
			}
		}
		if (strings.Contains(run, "go.dev/dl") || strings.Contains(run, "cli/cli/releases")) && !refusesMismatchFirst(run) {
			p = append(p, fmt.Sprintf("step %q extracts a download without refusing a checksum mismatch first", st.Name))
		}
		if strings.Contains(run, "go.dev/dl") {
			for _, need := range []string{"sha256sum", "$GO_LINUX_AMD64_SHA256", "${RUNNER_TEMP}/go-toolchain",
				"GOCACHE=${RUNNER_TEMP}", "GOMODCACHE=${RUNNER_TEMP}", "GOPATH=${RUNNER_TEMP}", "GOFLAGS="} {
				if !strings.Contains(run, need) {
					p = append(p, fmt.Sprintf("step %q installs Go without %s", st.Name, need))
				}
			}
			if strings.Contains(run, "HOME") {
				p = append(p, fmt.Sprintf("step %q installs Go under the runner home", st.Name))
			}
		}
		if strings.Contains(run, "cli/cli/releases") {
			pinnedGh = refusesMismatchFirst(run) && regexp.MustCompile(`want="[0-9a-f]{64}"`).MatchString(run)
			if strings.Contains(run, "command -v gh") {
				p = append(p, fmt.Sprintf("step %q accepts a gh already on PATH", st.Name))
			}
		}
		usesGit, usesGh := invokes(run, "git"), invokes(run, "gh")
		callsGh = callsGh || usesGh
		// The step that gives the job its own git and gh configuration: a
		// global config file of its own with hooks off, the system config
		// off, repository discovery stopped at RUNNER_TEMP, and a gh config
		// directory of its own — exported for every later step.
		if strings.Contains(run, "GIT_CONFIG_GLOBAL=") && strings.Contains(run, "GITHUB_ENV") {
			for _, need := range []string{"hooksPath = /dev/null", `echo "GIT_CONFIG_GLOBAL=`, `echo "GIT_CONFIG_NOSYSTEM=1"`,
				`echo "GIT_CEILING_DIRECTORIES=${RUNNER_TEMP}"`, `echo "GH_CONFIG_DIR=${RUNNER_TEMP}/`, "clone", "--template= "} {
				if !strings.Contains(run, need) {
					p = append(p, fmt.Sprintf("step %q sets up the job's git without %s", st.Name, need))
				}
			}
			ownGitCfg = true
		} else if (usesGit || usesGh) && !ownGitCfg {
			gitBeforeCfg = true
		}
		if (usesGit || usesGh || invokes(run, "go")) && !strings.HasPrefix(st.WorkingDirectory, runnerTempExpr) {
			p = append(p, fmt.Sprintf("step %q runs git, gh or go outside a directory under RUNNER_TEMP", st.Name))
		}
		if regexp.MustCompile(`\bpush\s+(-\S+\s+)*origin\b`).MatchString(run) {
			p = append(p, fmt.Sprintf("step %q pushes to a configured remote, not the forge URL", st.Name))
		}
		if strings.Contains(run, "credential.helper=!") || (strings.Contains(run, "credential.") && !strings.Contains(run, `credential.${GITHUB_SERVER_URL}.helper=`)) {
			p = append(p, fmt.Sprintf("step %q gives a credential helper that is not bound to the forge host", st.Name))
		}
		// A host-bound helper is added to whatever helper list git already
		// holds; the empty value just before it clears that list first.
		if strings.Contains(run, "credential.") && !strings.Contains(run, `-c credential.helper= -c "credential.${GITHUB_SERVER_URL}.helper=`) {
			p = append(p, fmt.Sprintf("step %q does not clear inherited credential helpers before its own", st.Name))
		}
	}
	if !ownGitCfg {
		p = append(p, "no step gives the job git and gh configuration of its own (GIT_CONFIG_GLOBAL, GIT_CONFIG_NOSYSTEM, GH_CONFIG_DIR via GITHUB_ENV)")
	}
	if gitBeforeCfg {
		p = append(p, "git or gh runs before the job's own git configuration is in place")
	}
	if callsGh && !pinnedGh {
		p = append(p, "calls gh with no pinned, checksum-verified install")
	}
	return p
}

func TestCredentialedJobIsolation(t *testing.T) {
	wf := stagedWorkflow(t)
	seen := map[string]bool{}
	names := make([]string, 0, len(wf.Jobs))
	for name := range wf.Jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		problems := isolationProblems(wf, wf.Jobs[name])
		seen[name] = true
		switch {
		case knownUnisolated[name] && len(problems) == 0:
			t.Errorf("job %q is now isolated — drop it from knownUnisolated", name)
		case !knownUnisolated[name] && len(problems) > 0:
			t.Errorf("job %q mints the App token without the reconcile job's isolation:\n  %s", name, strings.Join(problems, "\n  "))
		}
	}
	for name := range knownUnisolated {
		if !seen[name] {
			t.Errorf("knownUnisolated names %q, which is not a job in %s", name, reconcileWorkflowPath())
		}
	}
}

// TestIsolationGuardFlagsPlant is the guard's positive control: a planted
// credentialed job with every departure is flagged on each, so a matcher that
// silently stopped matching fails here instead of reporting clean.
func TestIsolationGuardFlagsPlant(t *testing.T) {
	const planted = `
env:
  GO_LINUX_AMD64_SHA256: "x"
jobs:
  planted:
    steps:
      - uses: actions/create-github-app-token@0000000000000000000000000000000000000000
      - uses: actions/checkout@0000000000000000000000000000000000000000
      - name: Install Go
        run: |
          curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o "${RUNNER_TEMP}/go.tar.gz"
          tar -C "${HOME}/go-toolchain" -xzf "${RUNNER_TEMP}/go.tar.gz"
      - name: gh
        run: |
          if command -v gh >/dev/null 2>&1; then exit 0; fi
          curl -fsSL https://github.com/cli/cli/releases/download/v2.63.2/gh.tar.gz -o gh.tar.gz
          got="$(sha256sum gh.tar.gz | cut -d' ' -f1)"
          tar -xzf gh.tar.gz
          if [ "$want" != "$got" ]; then
            exit 1
          fi
      - name: use
        run: |
          gh pr list
          git -c 'credential.helper=!f() { echo password=x; }; f' push origin HEAD
`
	wf := parseWorkflow(t, []byte(planted), "planted")
	got := strings.Join(isolationProblems(wf, wf.Jobs["planted"]), "\n")
	for _, want := range []string{"GOENV is not off", "GOTOOLCHAIN is not local", "uses actions/checkout", "checkout persists a credential",
		"without sha256sum", "under the runner home", "accepts a gh already on PATH", "no pinned, checksum-verified install",
		"no step gives the job git and gh configuration of its own", "runs before the job's own git configuration",
		`step "use" runs git, gh or go outside a directory under RUNNER_TEMP`, "pushes to a configured remote",
		"not bound to the forge host", `step "Install Go" extracts a download without refusing a checksum mismatch first`,
		`step "gh" extracts a download without refusing a checksum mismatch first`,
		`step "use" does not clear inherited credential helpers`} {
		if !strings.Contains(got, want) {
			t.Errorf("planted job not flagged for %q; got:\n%s", want, got)
		}
	}
	// And the staged reconcile job is flagged on none of them.
	if p := isolationProblems(stagedWorkflow(t), stagedWorkflow(t).Jobs["reconcile"]); len(p) != 0 {
		t.Errorf("reconcile job: %v", p)
	}
}

// The reconcile job's two checksum gates, anchored on the comments around them
// (the same install text recurs in other jobs of the file), with the gate
// removed or moved after the extraction.
const (
	ghGateLines = `          if [ "$want" != "$got" ]; then
            echo "::error::gh tarball sha256 mismatch — want ${want}, got ${got}; refusing to extract"
            exit 1
          fi
`
	ghTarLines = `          tar -C "${RUNNER_TEMP}" -xzf "${RUNNER_TEMP}/gh.tar.gz"
          echo "${RUNNER_TEMP}/gh_2.63.2_linux_amd64/bin" >> "${GITHUB_PATH}"
      # Compute: read-only.`
	ghGate     = ghGateLines + ghTarLines
	ghTar      = ghTarLines
	ghTarFirst = `          tar -C "${RUNNER_TEMP}" -xzf "${RUNNER_TEMP}/gh.tar.gz"` + "\n" + ghGateLines + `          echo "${RUNNER_TEMP}/gh_2.63.2_linux_amd64/bin" >> "${GITHUB_PATH}"
      # Compute: read-only.`
	goHead = `never $HOME; the tarball checksum-pinned.
      - name: Install Go (job-local toolchain and caches, checksum-verified)
        run: |
          set -euo pipefail
          url="https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
          echo "fetching $url"
          curl -fsSL "$url" -o "${RUNNER_TEMP}/go.tar.gz"
          got="$(sha256sum "${RUNNER_TEMP}/go.tar.gz" | cut -d' ' -f1)"
`
	goGate = goHead + `          if [ "$GO_LINUX_AMD64_SHA256" != "$got" ]; then
            echo "::error::Go tarball sha256 mismatch — want ${GO_LINUX_AMD64_SHA256}, got ${got}; refusing to extract"
            exit 1
          fi
`
	goNoGate = goHead
)

// reconcileMutants are edits to the staged job, each of which one named test
// must catch. TestReconcileJobMutantsFail applies each to a copy of the file
// and runs that test against the copy: a mutant the test survives fails the
// suite. Each `from` must occur exactly once, so the list stays bound to the
// file's text.
var reconcileMutants = []struct {
	name, from, to, test string
}{
	{"backfill on the schedule",
		`"$statusgen" reconcile --apply --root .`, `"$statusgen" reconcile --backfill --apply --root .`, "TestReconcileJobStatusgenArgs"},
	{"forced push (+refspec)",
		`"${COMMIT}:refs/heads/${branch}"`, `"+${COMMIT}:refs/heads/${branch}"`, "TestReconcileJobNeverForces"},
	{"forced push (--force)",
		`push "${GITHUB_SERVER_URL}/${repo}.git"`, `push --force "${GITHUB_SERVER_URL}/${repo}.git"`, "TestReconcileJobNeverForces"},
	{"README-only refusal removed",
		`"$readme_only" || foreign=`, `true || foreign=`, "TestReconcileJobRefusesNonReadmeCommit"},
	{"README pattern admits all of docs/streams",
		`readme_re='^docs/streams/[^/]+/README\.md$'`, `readme_re='^docs/streams/'`, "TestReconcileJobRefusesNonReadmeCommit"},
	{"README pattern admits nested READMEs",
		`readme_re='^docs/streams/[^/]+/README\.md$'`, `readme_re='^docs/streams/.+/README\.md$'`, "TestReconcileJobRefusesNonReadmeCommit"},
	{"one-parent subject check removed",
		`echo "$subj" | grep -q -E "$subject_re" || {`, `true || {`, "TestReconcileJobForeignSubject"},
	{"subject pattern matches anything",
		`subject_re='^chore\(board\): reconcile( |$)'`, `subject_re=''`, "TestReconcileJobForeignSubject"},
	{"merge subject check removed",
		`echo "$subj" | grep -q -E "${subject_re}|^Merge " || {`, `true || {`, "TestReconcileJobForeignMerge"},
	{"merge subject admits any subject",
		`"${subject_re}|^Merge "`, `"${subject_re}|^"`, "TestReconcileJobForeignMerge"},
	{"stray-path guard removed",
		`if [ -n "$stray" ]; then`, `if false; then`, "TestReconcileJobRefusesStrayPath"},
	{"repository guard dropped",
		` && github.repository == 'medici-finance/assay' }}`, ` }}`, "TestReconcileJobTriggerAndScope"},
	{"contents widened",
		"      contents: read\n      pull-requests: read\n      issues: read", "      contents: write\n      pull-requests: read\n      issues: read", "TestReconcileJobTriggerAndScope"},
	{"mint not narrowed",
		"          permission-contents: write\n", "", "TestReconcileJobTriggerAndScope"},
	{"detached checkout of fetched main removed",
		`git checkout -q --detach "$base"`, `true`, "TestReconcileJobComputesOnFetchedMain"},
	{"commit-output check removed",
		`''|*[!0-9a-f]*) echo`, `__never__) echo`, "TestReconcileJobRejectsBadCommitOutput"},
	{"list not paginated",
		`gh api --paginate "repos/`, `gh api "repos/`, "TestReconcileJobPRQuery"},
	{"projection reads the base repository",
		`(.head.repo.full_name // "-")`, `(.base.repo.full_name // "-")`, "TestReconcileJobPRQuery"},
	{"PR opened as non-draft",
		`--draft --base "$default_branch"`, `--base "$default_branch"`, "TestReconcileJobFreshBranch"},
	{"head repository not compared",
		`[ "$head_repo" != "$repo" ] || `, ``, "TestReconcileJobIgnoresForkPR"},
	{"head ref not compared",
		` || [ "$head_ref" != "$branch" ]`, ``, "TestReconcileJobIgnoresForkPR"},
	{"base not compared",
		`elif [ "$base_ref" != "$default_branch" ]; then`, `elif false; then`, "TestReconcileJobPRIdentity"},
	{"second selected PR not refused",
		`elif [ -n "$own" ]; then`, `elif false; then`, "TestReconcileJobPRIdentity"},
	{"title not rewritten",
		`-f "title=${title}" `, ``, "TestReconcileJobRefreshesBody"},
	{"runner user's git configuration kept",
		`echo "GIT_CONFIG_GLOBAL=${cfg}"`, `echo "UNUSED_CONFIG=${cfg}"`, "TestReconcileJobIgnoresRunnerResidue"},
	{"gh configuration directory not the job's own",
		`echo "GH_CONFIG_DIR=${RUNNER_TEMP}/board-gh-config"`, `echo "UNUSED_DIR=${RUNNER_TEMP}/board-gh-config"`, "TestReconcileJobIgnoresRunnerResidue"},
	{"publish runs in the workspace",
		"        working-directory: ${{ runner.temp }}/board-src\n        env:\n          # The WRITE token", "        env:\n          # The WRITE token", "TestReconcileJobIgnoresRunnerResidue"},
	{"compute runs in the workspace",
		"        working-directory: ${{ runner.temp }}/board-src\n        env:\n          # The READ token", "        env:\n          # The READ token", "TestReconcileJobIgnoresRunnerResidue"},
	{"hooks not disabled",
		`printf '[core]\n\thooksPath = /dev/null\n' > "$cfg"`, `: > "$cfg"`, "TestIsolationGuardFlagsPlant"},
	{"clone takes the template directory",
		`--branch main --template= \`, `--branch main \`, "TestIsolationGuardFlagsPlant"},
	{"push to a configured remote",
		`push "${GITHUB_SERVER_URL}/${repo}.git" "${COMMIT}`, `push origin "${COMMIT}`, "TestIsolationGuardFlagsPlant"},
	{"credential helper for any host",
		`-c "credential.${GITHUB_SERVER_URL}.helper=${helper}"`, `-c "credential.helper=${helper}"`, "TestIsolationGuardFlagsPlant"},
	{"schedule moved off the hourly tick",
		`- cron: "17 * * * *"`, `- cron: "17 3 * * *"`, "TestReconcileJobTriggerAndScope"},
	{"scheduled runs share the ref's group",
		`github.event_name == 'schedule' && 'reconcile' || `, ``, "TestReconcileJobTriggerAndScope"},
	{"lint step no longer runs the job tests",
		`-run '^(TestReconcileJob|TestCredentialedJobIsolation|TestIsolationGuardFlagsPlant|TestStatusKeyedLintRulesRegistered)'`,
		`-run '^(TestStatusKeyedLintRulesRegistered)'`, "TestReconcileJobTriggerAndScope"},
	{"staged copy not a PR trigger path",
		`      - "ci/staged-workflows/assay-statusgen.yml"`, ``, "TestReconcileJobTriggerAndScope"},
	{"statusgen not a PR trigger path",
		"      - \"STATUS.md\"\n      - \"statusgen/**\"\n", "      - \"STATUS.md\"\n", "TestReconcileJobTriggerAndScope"},
	{"inherited credential helpers kept",
		`git -c credential.helper= -c "credential.`, `git -c "credential.`, "TestIsolationGuardFlagsPlant"},
	{"gh checksum gate removed (guard)", ghGate, ghTar, "TestIsolationGuardFlagsPlant"},
	{"gh checksum gate removed (effect)", ghGate, ghTar, "TestReconcileJobInstallVerifies"},
	{"gh extracted before its checksum gate", ghGate, ghTarFirst, "TestIsolationGuardFlagsPlant"},
	{"Go checksum gate removed (guard)", goGate, goNoGate, "TestIsolationGuardFlagsPlant"},
	{"Go checksum gate removed (effect)", goGate, goNoGate, "TestReconcileJobInstallVerifies"},
	{"workspace checkout re-added",
		"      - name: Clone main into a fresh job-local directory\n", "      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1\n        with:\n          persist-credentials: false\n      - name: Clone main into a fresh job-local directory\n", "TestReconcileJobStepWiring"},
}

// TestReconcileJobMutantsFail is review B6's class guard: each property the job
// comment and the README say is tested is shown red when its line is reverted.
// It re-runs this test binary against each mutated copy of the staged file.
func TestReconcileJobMutantsFail(t *testing.T) {
	if os.Getenv("RECONCILEJOB_WORKFLOW") != "" {
		t.Skip("running against a mutant")
	}
	requireJobTools(t)
	raw, err := os.ReadFile(reconcileJobWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range reconcileMutants {
		m := m
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			if n := strings.Count(string(raw), m.from); n != 1 {
				t.Fatalf("mutant text occurs %d times in %s, want 1: %q", n, reconcileJobWorkflow, m.from)
			}
			p := filepath.Join(t.TempDir(), "assay-statusgen.yml")
			if err := os.WriteFile(p, []byte(strings.Replace(string(raw), m.from, m.to, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run", "^"+m.test+"$", "-test.count=1")
			cmd.Env = append(os.Environ(), "RECONCILEJOB_WORKFLOW="+p)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("mutant survived: %s stays green\n%s", m.test, out)
			}
			if !strings.Contains(string(out), "--- FAIL: "+m.test) {
				t.Fatalf("mutant run failed, but not in %s:\n%s", m.test, out)
			}
		})
	}
}
