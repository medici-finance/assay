package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The scheduled reconcile tick — the reconcile step's own `run:` text, read from
// the staged workflow — exercised end to end against a throwaway bare origin, with stub `statusgen` and `gh` on PATH — hermetic: no
// network, no token, no real repo.

const jobReadmeMain = "# s\n\n| # | Brief | Status |\n|---|---|---|\n| 01 | one | todo |\n| 02 | two | todo |\n"

// stubStatusgen flips row $STUB_FLIP (if set) from todo to implemented in the
// fixture README on `reconcile`, prints the --apply report, and exits
// $STUB_RECONCILE_RC; `regen` is a no-op and the lint exits $STUB_LINT_RC.
const stubStatusgen = `#!/usr/bin/env bash
set -eu
f=docs/streams/s/README.md
case "${1:-}" in
  reconcile)
    if [ "${STUB_RECONCILE_RC:-0}" != 0 ]; then echo "reconcile --apply: could-not-check — HTTP 401" >&2; exit "$STUB_RECONCILE_RC"; fi
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

const stubGh = `#!/usr/bin/env bash
echo "$*" >> "$STUB_LOG/gh.log"
case "$1 $2" in
  "pr list") [ "${STUB_GH_LIST_RC:-0}" = 0 ] || exit "$STUB_GH_LIST_RC"; echo "${STUB_OPEN:-0}" ;;
  "pr create") echo "https://example.invalid/pr/1" ;;
esac
`

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
	r.env = append(os.Environ(),
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

// reconcileJobWorkflow is the staged workflow whose reconcile step is under test.
const reconcileJobWorkflow = "../ci/staged-workflows/assay-statusgen.yml"

// reconcileJobStepName names that step; the test fails if it is renamed away.
const reconcileJobStepName = "Reconcile and regenerate the stream README tables"

// reconcileJobStep extracts the reconcile step's `run:` text from the staged
// workflow, exactly as the runner would execute it.
func reconcileJobStep(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(reconcileJobWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	var wf struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parsing %s: %v", reconcileJobWorkflow, err)
	}
	for _, st := range wf.Jobs["reconcile"].Steps {
		if st.Name == reconcileJobStepName {
			if strings.TrimSpace(st.Run) == "" {
				t.Fatalf("%s: step %q has no run: text", reconcileJobWorkflow, reconcileJobStepName)
			}
			return st.Run
		}
	}
	t.Fatalf("%s: jobs.reconcile has no step named %q", reconcileJobWorkflow, reconcileJobStepName)
	return ""
}

// jobScript writes the step under test to a file outside the work clone and
// returns its path; RECONCILEJOB_SCRIPT points the same rig at another body
// (the pre-fix inline step, for the fail-first record).
func jobScript(t *testing.T) string {
	t.Helper()
	if s := os.Getenv("RECONCILEJOB_SCRIPT"); s != "" {
		return s
	}
	p := filepath.Join(t.TempDir(), "reconcile-step.sh")
	if err := os.WriteFile(p, []byte(reconcileJobStep(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// tick runs one job tick in a fresh clone (as the workflow's checkout gives it)
// with extra env, returning the exit code and combined output.
func (r *jobRig) tick(extra ...string) (int, string) {
	r.t.Helper()
	work := filepath.Join(r.dir, "work")
	_ = os.RemoveAll(work)
	r.git("", "clone", "-q", r.origin, work)
	cmd := exec.Command("bash", jobScript(r.t))
	cmd.Dir = work
	cmd.Env = append(append([]string{}, r.env...), extra...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		r.t.Fatal(err)
	}
	return code, string(out)
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

	code, out := r.tick("STUB_FLIP=01", "STUB_OPEN=1")
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

	code, out := r.tick("STUB_FLIP=01", "STUB_OPEN=1")
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
	code, out := r.tick("STUB_FLIP=02", "STUB_OPEN=1", "PATH="+wrap+string(os.PathListSeparator)+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"))
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

	code, out := r.tick("STUB_FLIP=01", "STUB_OPEN=1")
	if code != 0 {
		t.Fatalf("the desk-side writer's merge commit must not read as foreign (exit %d):\n%s", code, out)
	}
	if after := r.remoteRef("refs/heads/board/reconcile"); after != before {
		t.Fatalf("nothing changed state, so the branch must not move (%s -> %s)", before, after)
	}
}
