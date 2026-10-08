package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The scheduled reconcile tick — the reconcile job's own compute and publish
// `run:` texts, read from the staged workflow — exercised end to end against a
// throwaway bare origin, with stub `statusgen` and `gh` on PATH — hermetic: no
// network, no token, no real repo.

const jobReadmeMain = "# s\n\n| # | Brief | Status |\n|---|---|---|\n| 01 | one | todo |\n| 02 | two | todo |\n"

// Sentinels the rig substitutes for the workflow's token expressions, so a test
// can tell which step held which credential.
const (
	readTokenSentinel  = "READ-TOKEN-SENTINEL"
	writeTokenSentinel = "WRITE-TOKEN-SENTINEL"
)

// stubStatusgen flips row $STUB_FLIP (if set) from todo to implemented in the
// fixture README on `reconcile`, or with $STUB_HOLD reports one held row and
// writes nothing; prints the --apply report and exits $STUB_RECONCILE_RC.
// `regen` is a no-op and the lint exits $STUB_LINT_RC. With $STUB_CHECK_WRITE
// set, every invocation fails if its environment holds the write token.
const stubStatusgen = `#!/usr/bin/env bash
set -eu
if [ -n "${STUB_CHECK_WRITE:-}" ]; then
  case "$(env)" in *WRITE-TOKEN-SENTINEL*) echo "statusgen ran holding the write token" >&2; exit 97 ;; esac
fi
f=docs/streams/s/README.md
case "${1:-}" in
  reconcile)
    if [ "${STUB_RECONCILE_RC:-0}" != 0 ]; then echo "reconcile --apply: could-not-check — HTTP 401" >&2; exit "$STUB_RECONCILE_RC"; fi
    if [ -n "${STUB_HOLD:-}" ]; then
      echo "reconcile: o/r (2 brief(s))"
      echo "reconcile --apply: wrote 0 row(s), held 1"
      echo "  HELD s/${STUB_HOLD} todo -> implemented: risk-gated brief at \"implemented\" has no design: record"
    fi
    if [ -n "${STUB_FLIP:-}" ]; then
      sed "s/^| ${STUB_FLIP} | \(.*\) | todo |\$/| ${STUB_FLIP} | \1 | implemented |/" "$f" > "$f.tmp" && mv "$f.tmp" "$f"
      echo "reconcile: o/r (2 brief(s))"
      echo "reconcile --apply: wrote 1 row(s)"
      echo "  s/${STUB_FLIP} todo -> implemented   $f   [PR #7 (merged abc1234)]"
    fi ;;
  regen) ;;
  *) exit "${STUB_LINT_RC:-0}" ;;
esac
`

// stubGh logs every call. `api` GETs print $STUB_PULLS: TSV lines of number,
// head repository and head ref — what the step's --jq projection prints — and
// the stub returns EVERY entry whatever the query says, the worst case of a head
// filter that also matches forks, so the step's own selection is what is
// tested. `pr list` is the forge's bare head-name filter (forks included): it
// counts every $STUB_PULLS entry. A call made holding the write token logs
// `token=write`.
const stubGh = `#!/usr/bin/env bash
echo "$*" >> "$STUB_LOG/gh.log"
[ "${GH_TOKEN:-}" = WRITE-TOKEN-SENTINEL ] && echo "token=write" >> "$STUB_LOG/gh.log"
case "$1" in
  api)
    case " $* " in *" --method PATCH "*) exit 0 ;; esac
    [ "${STUB_GH_LIST_RC:-0}" = 0 ] || exit "$STUB_GH_LIST_RC"
    [ -z "${STUB_PULLS:-}" ] || printf '%s\n' "$STUB_PULLS" ;;
  pr)
    case "$2" in
      list)
        [ "${STUB_GH_LIST_RC:-0}" = 0 ] || exit "$STUB_GH_LIST_RC"
        if [ -n "${STUB_PULLS:-}" ]; then printf '%s\n' "$STUB_PULLS" | grep -c .; else echo 0; fi ;;
      create) echo "https://example.invalid/pr/1" ;;
    esac ;;
esac
`

// ownPR is this repository's own open PR on the carried branch, as the step's
// --jq projection prints it.
const ownPR = "5\to/r\tboard/reconcile"

type jobRig struct {
	t      *testing.T
	dir    string
	origin string
	seed   string
	bin    string
	env    []string
}

func requireJobTools(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the reconcile job runs on a Linux runner; its shell is not exercised on Windows")
	}
	for _, tool := range []string{"bash", "git", "sed"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
}

// scrubbedEnv is the caller's environment minus any credential or runner
// variable a step could act on: the rig supplies each step's env itself.
func scrubbedEnv() []string {
	drop := map[string]bool{
		"GH_TOKEN": true, "GITHUB_TOKEN": true, "GH_ENTERPRISE_TOKEN": true,
		"GITHUB_ENTERPRISE_TOKEN": true, "GITHUB_OUTPUT": true, "GITHUB_ENV": true,
		"GITHUB_PATH": true, "COMMIT": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); !drop[k] {
			env = append(env, kv)
		}
	}
	return env
}

func newJobRig(t *testing.T) *jobRig {
	t.Helper()
	requireJobTools(t)
	r := &jobRig{t: t, dir: t.TempDir()}
	r.origin = filepath.Join(r.dir, "origin.git")
	r.seed = filepath.Join(r.dir, "seed")
	r.bin = filepath.Join(r.dir, "bin")
	if err := os.MkdirAll(r.bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"statusgen": stubStatusgen, "gh": stubGh} {
		if err := os.WriteFile(filepath.Join(r.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitcfg := filepath.Join(r.dir, "gitconfig")
	if err := os.WriteFile(gitcfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r.env = append(scrubbedEnv(),
		"PATH="+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GIT_CONFIG_GLOBAL="+gitcfg, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=job", "GIT_AUTHOR_EMAIL=job@example.invalid",
		"GIT_COMMITTER_NAME=job", "GIT_COMMITTER_EMAIL=job@example.invalid",
		// What the runner provides the step: RUNNER_TEMP holds the statusgen
		// binary (the stub here) and the step's scratch files.
		"RUNNER_TEMP="+r.bin, "GITHUB_REPOSITORY=o/r",
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

// reconcileJobWorkflow is the staged workflow whose reconcile job is under test.
const reconcileJobWorkflow = "../ci/staged-workflows/assay-statusgen.yml"

// The two steps the tick runs; the tests fail if either is renamed away.
const (
	reconcileJobStepName = "Reconcile and regenerate the stream README tables"
	publishStepName      = "Push board/reconcile and open or refresh its draft PR"
	appTokenExpr         = "${{ steps.app-token.outputs.token }}"
	commitGateExpr       = "${{ steps.compute.outputs.commit != '' }}"
)

type wfStep struct {
	ID   string            `yaml:"id"`
	Name string            `yaml:"name"`
	If   string            `yaml:"if"`
	Uses string            `yaml:"uses"`
	With map[string]any    `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run"`
}

type wfJob struct {
	Env   map[string]string `yaml:"env"`
	Steps []wfStep          `yaml:"steps"`
}

type workflow struct {
	Env  map[string]string `yaml:"env"`
	Jobs map[string]wfJob  `yaml:"jobs"`
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
	raw, err := os.ReadFile(reconcileJobWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	return parseWorkflow(t, raw, reconcileJobWorkflow)
}

// reconcileStep returns the reconcile job's step of that name, exactly as the
// runner would execute it.
func reconcileStep(t *testing.T, name string) wfStep {
	t.Helper()
	for _, st := range stagedWorkflow(t).Jobs["reconcile"].Steps {
		if st.Name == name {
			if strings.TrimSpace(st.Run) == "" {
				t.Fatalf("%s: step %q has no run: text", reconcileJobWorkflow, name)
			}
			return st
		}
	}
	t.Fatalf("%s: jobs.reconcile has no step named %q", reconcileJobWorkflow, name)
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

// scriptFile writes a step's run text to a file outside the work clone.
func scriptFile(t *testing.T, run string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "step.sh")
	if err := os.WriteFile(p, []byte(run), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func (r *jobRig) runStep(work, script string, env []string) (int, string) {
	r.t.Helper()
	cmd := exec.Command("bash", script)
	cmd.Dir = work
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

// readOutputs parses a GITHUB_OUTPUT file of key=value lines.
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

// tick runs one job tick in a fresh clone (as the workflow's checkout gives it)
// with extra env, returning the exit code and combined output: the compute
// step, then — only when it set its `commit` output, as the publish step's
// `if:` says — the publish step. RECONCILEJOB_SCRIPT points the rig at another
// body run as the whole tick (a pre-fix single-step body, for the fail-first
// record), with the env shape that body had: both tokens in one step.
func (r *jobRig) tick(extra ...string) (int, string) {
	r.t.Helper()
	work := filepath.Join(r.dir, "work")
	_ = os.RemoveAll(work)
	r.git("", "clone", "-q", r.origin, work)
	outFile := filepath.Join(r.dir, "github-output")
	if err := os.WriteFile(outFile, nil, 0o644); err != nil {
		r.t.Fatal(err)
	}
	if s := os.Getenv("RECONCILEJOB_SCRIPT"); s != "" {
		env := append([]string{"GITHUB_TOKEN=" + readTokenSentinel, "GH_TOKEN=" + writeTokenSentinel, "GITHUB_OUTPUT=" + outFile}, extra...)
		return r.runStep(work, s, env)
	}
	compute := reconcileStep(r.t, reconcileJobStepName)
	code, out := r.runStep(work, scriptFile(r.t, compute.Run),
		append(append(stepEnv(r.t, compute, nil), "GITHUB_OUTPUT="+outFile), extra...))
	outputs := readOutputs(r.t, outFile)
	if code != 0 || outputs["commit"] == "" {
		return code, out
	}
	publish := reconcileStep(r.t, publishStepName)
	code, out2 := r.runStep(work, scriptFile(r.t, publish.Run),
		append(stepEnv(r.t, publish, outputs), extra...))
	return code, out + out2
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

// TestReconcileJobFreshBranch: no carried branch yet; a witnessed flip is
// committed on a branch cut from main and the draft PR is opened.
func TestReconcileJobFreshBranch(t *testing.T) {
	r := newJobRig(t)
	code, out := r.tick("STUB_FLIP=01")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(r.remoteFile("refs/heads/board/reconcile"), "| 01 | one | implemented |") {
		t.Fatalf("flip not carried:\n%s", r.remoteFile("refs/heads/board/reconcile"))
	}
	if !strings.Contains(r.ghLog(), "pr create") {
		t.Fatalf("draft PR not opened; gh log:\n%s", r.ghLog())
	}
	msg := r.git(r.origin, "log", "-1", "--format=%B", "refs/heads/board/reconcile")
	if !strings.Contains(msg, "PR #7") {
		t.Fatalf("the commit must name each flip's witness:\n%s", msg)
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

// TestReconcileJobRefusesForeignCommit: a commit on the branch that this job
// did not make is never overwritten — the tick fails loudly and pushes nothing.
func TestReconcileJobRefusesForeignCommit(t *testing.T) {
	r := newJobRig(t)
	if code, out := r.tick("STUB_FLIP=01"); code != 0 {
		t.Fatalf("first tick exit %d:\n%s", code, out)
	}
	// A human pushes a hand edit onto the carried branch.
	hand := filepath.Join(r.dir, "hand")
	r.git("", "clone", "-q", "-b", "board/reconcile", r.origin, hand)
	if err := os.WriteFile(filepath.Join(hand, "NOTES.md"), []byte("hand edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git(hand, "add", "-A")
	r.git(hand, "commit", "-q", "-m", "wip: my notes")
	r.git(hand, "push", "-q", "origin", "board/reconcile")
	before := r.remoteRef("refs/heads/board/reconcile")
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
// whose fork was deleted (no head repository) — is ignored and our PR is opened.
func TestReconcileJobIgnoresForkPR(t *testing.T) {
	r := newJobRig(t)
	lookalikes := "9\toutsider/r\tboard/reconcile\n10\t\tboard/reconcile"
	code, out := r.tick("STUB_FLIP=01", "STUB_PULLS="+lookalikes)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if !strings.Contains(r.ghLog(), "pr create") {
		t.Fatalf("a fork's same-named PR was read as ours — no PR opened:\n%s\ngh log:\n%s", out, r.ghLog())
	}
	if strings.Contains(r.ghLog(), "--method PATCH") {
		t.Fatalf("a look-alike PR's body was edited:\n%s", r.ghLog())
	}
	for _, n := range []string{"#9", "#10"} {
		if !strings.Contains(out, "ignoring open PR "+n) {
			t.Fatalf("the ignored look-alike %s must be named in the log:\n%s", n, out)
		}
	}
	if !strings.Contains(r.ghLog(), "head=o:board/reconcile") {
		t.Fatalf("the PR query must name the owner-qualified head:\n%s", r.ghLog())
	}
}

// TestReconcileJobRefreshesBody is review SEC-2372-4's second half: the PR text
// was written once, at creation, and never refreshed. Each push to the branch
// now rewrites our PR's body from that tick's report.
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
	if !strings.Contains(r.ghLog(), "api --method PATCH repos/o/r/pulls/5") {
		t.Fatalf("our open PR's body was not refreshed:\n%s", r.ghLog())
	}
	if n := strings.Count(r.ghLog(), "pr create"); n != 1 {
		t.Fatalf("pr create ran %d times; once, on the first tick, is right:\n%s", n, r.ghLog())
	}
	body, err := os.ReadFile(filepath.Join(r.bin, "reconcile-pr-body.md"))
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
	if !strings.Contains(out, "HELD s/01") || !strings.Contains(out, "no design: record") {
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

// TestReconcileJobStepWiring is review SEC-2372-2's credential half, read from
// the YAML: the checkout persists no credential; the App token is minted after
// every step that touches statusgen, only when the compute step produced a
// commit, and is referenced by the publish step alone, which runs no statusgen.
func TestReconcileJobStepWiring(t *testing.T) {
	job := stagedWorkflow(t).Jobs["reconcile"]
	mint, compute, publish, lastStatusgen := -1, -1, -1, -1
	for i, st := range job.Steps {
		switch {
		case strings.HasPrefix(st.Uses, "actions/checkout@"):
			if fmt.Sprint(st.With["persist-credentials"]) != "false" {
				t.Errorf("checkout must set persist-credentials: false, got %v", st.With["persist-credentials"])
			}
			if _, ok := st.With["token"]; ok {
				t.Errorf("checkout must not take a token: %v", st.With["token"])
			}
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
// runner label, without the corroborate job's isolation. knownUnisolated lists
// the jobs that are live today with that shape; each is a maintainer follow-up
// on the live workflow and outside this change. The list may only shrink: a
// listed job that is now isolated, or no longer exists, fails the guard too.
var knownUnisolated = map[string]bool{
	"regen":          true,
	"model-autoflip": true,
}

// isolationProblems names every way a credentialed job departs from the
// corroborate job's isolation. A job that mints no App token returns nil.
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
	callsGh, pinnedGh := false, false
	for _, st := range job.Steps {
		run := st.Run
		if strings.HasPrefix(st.Uses, "actions/checkout@") && fmt.Sprint(st.With["persist-credentials"]) != "false" {
			p = append(p, "checkout persists a credential")
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
			pinnedGh = strings.Contains(run, "sha256sum")
			if strings.Contains(run, "command -v gh") {
				p = append(p, fmt.Sprintf("step %q accepts a gh already on PATH", st.Name))
			}
		}
		for _, line := range strings.Split(run, "\n") {
			l := strings.TrimSpace(line)
			callsGh = callsGh || strings.HasPrefix(l, "gh ") || strings.Contains(l, "$(gh ") || strings.Contains(l, "\"$(gh ")
		}
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
			t.Errorf("job %q mints the App token without the corroborate job's isolation:\n  %s", name, strings.Join(problems, "\n  "))
		}
	}
	for name := range knownUnisolated {
		if !seen[name] {
			t.Errorf("knownUnisolated names %q, which is not a job in %s", name, reconcileJobWorkflow)
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
      - name: use
        run: |
          gh pr list
`
	wf := parseWorkflow(t, []byte(planted), "planted")
	got := strings.Join(isolationProblems(wf, wf.Jobs["planted"]), "\n")
	for _, want := range []string{"GOENV is not off", "GOTOOLCHAIN is not local", "checkout persists a credential",
		"without sha256sum", "under the runner home", "accepts a gh already on PATH", "no pinned, checksum-verified install"} {
		if !strings.Contains(got, want) {
			t.Errorf("planted job not flagged for %q; got:\n%s", want, got)
		}
	}
	// And the staged reconcile job is flagged on none of them.
	if p := isolationProblems(stagedWorkflow(t), stagedWorkflow(t).Jobs["reconcile"]); len(p) != 0 {
		t.Errorf("reconcile job: %v", p)
	}
}
