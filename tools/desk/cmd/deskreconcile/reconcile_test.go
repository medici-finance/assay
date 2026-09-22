package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// runNow is a fixed clock so the commit message is deterministic.
var runNow = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// dry-run / step-sequence tests (fake Exec, records the command shapes)
// ---------------------------------------------------------------------------

// recordingExec records every command and returns the canned reply keyed by the FIRST
// arg (verb). A statusgen reply drives the "did anything change" decision through the
// git status reply, so both are scriptable.
type recordingExec struct {
	steps   []string
	replies map[string]string // key: name+" "+firstArg, value: stdout
	deskpr  []string          // deskpr subcommands seen, in order
}

func (r *recordingExec) fn(dir, name string, args ...string) (string, error) {
	r.steps = append(r.steps, renderStep(dir, name, args))
	// Simulate the one filesystem effect a later step depends on: `git worktree add
	// <path>` creates the worktree directory, which the PR-body write lands in.
	if name == "git" && len(args) >= 3 && args[0] == "worktree" && args[1] == "add" {
		_ = os.MkdirAll(args[len(args)-2], 0o755)
	}
	if name == "deskpr" && len(args) > 0 {
		r.deskpr = append(r.deskpr, args[0])
	}
	// The origin-derive read (not a recorded step) resolves --repo when none is passed.
	if name == "git" && len(args) >= 2 && args[0] == "config" && args[1] == "--get" {
		return "git@github.com:example-org/example-repo.git\n", nil
	}
	key := name
	if len(args) > 0 {
		key = name + " " + args[0]
	}
	if v, ok := r.replies[key]; ok {
		return v, nil
	}
	return "", nil
}

func absWT(t *testing.T) (root, wt string) {
	t.Helper()
	base := t.TempDir()
	return filepath.Join(base, "target"), filepath.Join(base, "recon-wt")
}

func TestDryRun_ReconcilesButNeverCommitsOrOpensAPR(t *testing.T) {
	root, wt := absWT(t)
	rec := &recordingExec{replies: map[string]string{
		// one flip in the throwaway worktree, and a dirty README to prove it.
		"statusgen reconcile": `{"applied":[{"id":"alpha/01","from":"todo","to":"implemented","readme":"docs/streams/alpha/README.md","witness":"#100"}]}`,
		"git status":          " M docs/streams/alpha/README.md\n",
	}}
	res, err := Run(Options{Root: root, Worktree: wt, Now: runNow, DryRun: true, exec: rec.fn})
	if err != nil {
		t.Fatalf("dry-run refused: %v", err)
	}
	joined := strings.Join(rec.steps, "\n")
	for _, want := range []string{
		"git fetch origin",
		"git worktree add --detach " + wt + " refs/remotes/origin/main",
		"git checkout -B board/reconcile",
		"statusgen reconcile --backfill --apply --root . --json",
		"git status --porcelain",
		"git worktree remove --force " + wt,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("dry-run sequence missing %q:\n%s", want, joined)
		}
	}
	// The whole point of a dry-run: it discards. No commit, no deskpr, no ls-remote probe.
	for _, forbidden := range []string{"git commit", "deskpr", "git add", "git ls-remote"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("dry-run must not run %q:\n%s", forbidden, joined)
		}
	}
	if len(res.Flipped) != 1 || res.Flipped[0].ID != "alpha/01" {
		t.Fatalf("dry-run did not report the row it would flip: %+v", res.Flipped)
	}
	if res.NoOp {
		t.Fatalf("dry-run with a flip should not be a no-op")
	}
}

func TestRealRun_FreshOpensADraftPR(t *testing.T) {
	root, wt := absWT(t)
	rec := &recordingExec{replies: map[string]string{
		"git ls-remote":       "", // branch does not exist -> fresh
		"statusgen reconcile": `{"applied":[{"id":"alpha/01","from":"todo","to":"implemented","readme":"docs/streams/alpha/README.md","witness":"#100"}]}`,
		"git status":          " M docs/streams/alpha/README.md\n",
		"deskpr create":       "created https://example.invalid/pr/1\n",
	}}
	_, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: rec.fn})
	if err != nil {
		t.Fatalf("fresh run refused: %v", err)
	}
	joined := strings.Join(rec.steps, "\n")
	for _, want := range []string{
		"git ls-remote --heads origin board/reconcile",
		"git add -- docs/streams/alpha/README.md",
		"git commit -m " + prTitle + " 2026-09-22",
		"deskpr create --title " + prTitle,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fresh sequence missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "git merge") {
		t.Fatalf("a fresh run must not merge — it cuts from origin/main:\n%s", joined)
	}
	// exactly one PR: create, not create-then-a-second-create.
	if got := strings.Join(rec.deskpr, ","); got != "create" {
		t.Fatalf("deskpr calls = %q, want a single create", got)
	}
}

func TestRealRun_CreateNoopFallsBackToUpdate(t *testing.T) {
	// deskpr create noops (a PR already raced onto the branch) WITHOUT pushing our commit,
	// so the follow-up push verb must run to carry it.
	root, wt := absWT(t)
	rec := &recordingExec{replies: map[string]string{
		"git ls-remote":       "",
		"statusgen reconcile": `{"applied":[{"id":"alpha/01","from":"todo","to":"implemented","readme":"docs/streams/alpha/README.md","witness":"#100"}]}`,
		"git status":          " M docs/streams/alpha/README.md\n",
		"deskpr create":       "noop: open PR already exists for board/reconcile: https://example.invalid/pr/9\n",
	}}
	if _, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: rec.fn}); err != nil {
		t.Fatalf("run refused: %v", err)
	}
	if got := strings.Join(rec.deskpr, ","); got != "create,update" {
		t.Fatalf("deskpr calls = %q, want create then update", got)
	}
}

func TestRealRun_CoalesceUpdatesTheOpenPR(t *testing.T) {
	root, wt := absWT(t)
	rec := &recordingExec{replies: map[string]string{
		"git ls-remote":       "abc123\trefs/heads/board/reconcile\n", // branch exists -> coalesce
		"statusgen reconcile": `{"applied":[{"id":"beta/02","from":"todo","to":"implemented","readme":"docs/streams/beta/README.md","witness":"#200"}]}`,
		"git status":          " M docs/streams/beta/README.md\n",
	}}
	if _, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: rec.fn}); err != nil {
		t.Fatalf("coalesce run refused: %v", err)
	}
	joined := strings.Join(rec.steps, "\n")
	for _, want := range []string{
		"git worktree add --detach " + wt + " refs/remotes/origin/board/reconcile",
		"git merge --no-edit refs/remotes/origin/main",
		"deskpr update",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("coalesce sequence missing %q:\n%s", want, joined)
		}
	}
	if got := strings.Join(rec.deskpr, ","); got != "update" {
		t.Fatalf("deskpr calls = %q, want a single update (never a second PR)", got)
	}
}

func TestRealRun_NothingToFlipIsANoOp(t *testing.T) {
	root, wt := absWT(t)
	rec := &recordingExec{replies: map[string]string{
		"git ls-remote":       "",
		"statusgen reconcile": `{"applied":[]}`,
		"git status":          "", // nothing changed on disk
	}}
	res, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: rec.fn})
	if err != nil {
		t.Fatalf("no-op run refused: %v", err)
	}
	if !res.NoOp {
		t.Fatalf("a run that changed nothing must be a no-op: %+v", res)
	}
	joined := strings.Join(rec.steps, "\n")
	for _, forbidden := range []string{"git commit", "git add", "deskpr"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("a no-op must not %q:\n%s", forbidden, joined)
		}
	}
}

func TestRun_RefusesForeignChange(t *testing.T) {
	// A reconcile writes ONLY stream README Status cells. Anything else in the diff is not
	// reconcile output and must never be committed under the reconcile message.
	root, wt := absWT(t)
	rec := &recordingExec{replies: map[string]string{
		"git ls-remote":       "",
		"statusgen reconcile": `{"applied":[{"id":"alpha/01","from":"todo","to":"implemented","readme":"docs/streams/alpha/README.md","witness":"#100"}]}`,
		"git status":          " M docs/streams/alpha/README.md\n M STATUS.md\n M tools/desk/go.mod\n",
	}}
	_, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: rec.fn})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal on a foreign changed path", err, deskkit.ExitCodeOf(err))
	}
	if !strings.Contains(err.Error(), "STATUS.md") || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("the refusal must name the foreign paths: %v", err)
	}
	// It must refuse BEFORE committing anything.
	if strings.Contains(strings.Join(rec.steps, "\n"), "git commit") {
		t.Fatalf("refused but still committed")
	}
}

func TestRun_RefusesMissingWorktree(t *testing.T) {
	if _, err := Run(Options{Root: "/tmp/x", Worktree: "", Now: runNow}); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("empty worktree: err = %v, want refusal", err)
	}
	if _, err := Run(Options{Root: "/tmp/x", Worktree: "recon-wt", Now: runNow}); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("relative worktree: err = %v, want refusal", err)
	}
}

func TestRun_RefusesNestedWorktree(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(Options{Root: root, Worktree: filepath.Join(root, "inner", "wt"), Now: runNow}); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("nested worktree: err = %v, want refusal", err)
	}
}

func TestRealRun_RefusesWithoutIssue(t *testing.T) {
	root, wt := absWT(t)
	if _, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 0}); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("real run with no --issue: err = %v, want refusal", err)
	}
}

// ---------------------------------------------------------------------------
// end-to-end behaviour tests: real git fixture + a fake statusgen that WRITES,
// deskpr stubbed (no forge/token in a unit test).
// ---------------------------------------------------------------------------

// fixtureRepo builds a target checkout with an `origin` remote carrying `main`. The board
// has four briefs across four streams: alpha (todo, witnessed), bravo (todo, no witness),
// gamma (verified), delta (done). It returns the root checkout path.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	root := filepath.Join(base, "target")
	mustGit(t, base, "init", "--bare", origin)
	mustGit(t, base, "init", "-b", "main", root)
	gitConfig(t, root)

	writeReadme(t, root, "alpha", "01", "todo")
	writeReadme(t, root, "bravo", "01", "todo")
	writeReadme(t, root, "gamma", "01", "verified")
	writeReadme(t, root, "delta", "01", "done")

	mustGit(t, root, "add", "-A")
	mustGit(t, root, "commit", "-m", "fixture board")
	mustGit(t, root, "remote", "add", "origin", origin)
	mustGit(t, root, "push", "origin", "main")
	return root
}

func writeReadme(t *testing.T, root, stream, num, status string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A one-row generated briefs table. The Status cell is the ONLY `| <status> |` on the row.
	body := "# " + stream + "\n\n<!-- statusgen:briefs:begin -->\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| " + num + " | [t](brief-" + num + ".md) | 0 | M | " + status + " | x | x |\n" +
		"<!-- statusgen:briefs:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func gitConfig(t *testing.T, root string) {
	t.Helper()
	// A throwaway fixture repo, never the shared checkout: local identity is fine here.
	mustGit(t, root, "config", "user.name", "fixture")
	mustGit(t, root, "config", "user.email", "fixture@example.invalid")
	mustGit(t, root, "config", "commit.gpgsign", "false")
}

// fakeStatusgen writes a shell shim that flips the Status cell (todo -> implemented) of each
// stream named in $DR_FLIP (space-separated) whose README still reads todo, and emits the
// matching `applied` JSON. With $DR_FOREIGN set it also writes a non-README file, to drive
// the foreign-change refusal end to end. It points STATUSGEN_BIN at itself.
func fakeStatusgen(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "statusgen")
	shim := `#!/bin/sh
applied=""
for s in $DR_FLIP; do
  f="docs/streams/$s/README.md"
  if [ -f "$f" ] && grep -q '| todo |' "$f"; then
    sed 's/| todo |/| implemented |/' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
    row="{\"id\":\"$s/01\",\"from\":\"todo\",\"to\":\"implemented\",\"readme\":\"$f\",\"witness\":\"#100\"}"
    if [ -z "$applied" ]; then applied="$row"; else applied="$applied,$row"; fi
  fi
done
if [ -n "$DR_FOREIGN" ]; then echo tampered > STATUS.md; fi
printf '{"applied":[%s]}\n' "$applied"
`
	if err := os.WriteFile(bin, []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskkit.StatusgenBinEnv, bin)
}

// e2eExec runs git + statusgen for real (statusgen via STATUSGEN_BIN) and STUBS deskpr,
// recording its subcommands. The real forge write is exercised by deskpr's own tests; here
// the target is deskreconcile's own orchestration on a real tree.
func e2eExec(deskpr *[]string) Exec {
	return func(dir, name string, args ...string) (string, error) {
		if name == "deskpr" {
			if len(args) > 0 {
				*deskpr = append(*deskpr, args[0])
			}
			return "created https://example.invalid/pr/1\n", nil
		}
		return RealExec(dir, name, args...)
	}
}

func TestE2E_FlipsExactlyTheWitnessedTodoRow(t *testing.T) {
	fakeStatusgen(t)
	t.Setenv("DR_FLIP", "alpha") // only alpha (todo + witness) flips
	root := fixtureRepo(t)
	wt := filepath.Join(t.TempDir(), "recon-wt")
	var deskpr []string
	res, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: e2eExec(&deskpr)})
	if err != nil {
		t.Fatalf("run refused: %v", err)
	}
	if len(res.Changed) != 1 || res.Changed[0] != "docs/streams/alpha/README.md" {
		t.Fatalf("changed = %v, want exactly alpha's README", res.Changed)
	}
	if len(res.Flipped) != 1 || res.Flipped[0].ID != "alpha/01" || res.Flipped[0].To != "implemented" {
		t.Fatalf("flipped = %+v, want alpha/01 -> implemented", res.Flipped)
	}
	if got := strings.Join(deskpr, ","); got != "create" {
		t.Fatalf("deskpr = %q, want a single create", got)
	}
	// The commit on board/reconcile contains ONLY alpha's README — gamma (verified) and
	// delta (done) and bravo (todo, no witness) are untouched and uncommitted.
	files := mustGit(t, root, "show", "--name-only", "--format=", "board/reconcile")
	changed := strings.Fields(strings.TrimSpace(files))
	if len(changed) != 1 || changed[0] != "docs/streams/alpha/README.md" {
		t.Fatalf("commit touched %v, want only alpha's README", changed)
	}
	// origin/main is untouched (nothing pushed there): every row still as seeded.
	for _, s := range []string{"bravo", "gamma", "delta"} {
		got := showOnBranch(t, root, "main", "docs/streams/"+s+"/README.md")
		if strings.Contains(got, "implemented") {
			t.Fatalf("%s README on main was mutated: %s", s, got)
		}
	}
	// exactly one commit on the branch (one flip = one commit).
	if n := strings.TrimSpace(mustGit(t, root, "rev-list", "--count", "main..board/reconcile")); n != "1" {
		t.Fatalf("board/reconcile has %s commits ahead of main, want 1", n)
	}
}

func TestE2E_NoWitnessedTodoIsANoOp(t *testing.T) {
	fakeStatusgen(t)
	t.Setenv("DR_FLIP", "") // statusgen flips nothing (idempotent re-run shape)
	root := fixtureRepo(t)
	wt := filepath.Join(t.TempDir(), "recon-wt")
	var deskpr []string
	res, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: e2eExec(&deskpr)})
	if err != nil {
		t.Fatalf("run refused: %v", err)
	}
	if !res.NoOp {
		t.Fatalf("nothing witnessed -> must be a no-op, got %+v", res)
	}
	if len(deskpr) != 0 {
		t.Fatalf("a no-op opened/updated a PR: %v", deskpr)
	}
	// No board/reconcile commit was made.
	if err := exec.Command("git", "-C", root, "rev-parse", "--verify", "board/reconcile").Run(); err == nil {
		if n := strings.TrimSpace(mustGit(t, root, "rev-list", "--count", "main..board/reconcile")); n != "0" {
			t.Fatalf("a no-op left %s commits on board/reconcile", n)
		}
	}
}

func TestE2E_ForeignChangeRefusedOnARealTree(t *testing.T) {
	fakeStatusgen(t)
	t.Setenv("DR_FLIP", "alpha")
	t.Setenv("DR_FOREIGN", "1") // statusgen also dirties STATUS.md
	root := fixtureRepo(t)
	wt := filepath.Join(t.TempDir(), "recon-wt")
	var deskpr []string
	_, err := Run(Options{Root: root, Worktree: wt, Now: runNow, Issue: 1175, exec: e2eExec(&deskpr)})
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, deskkit.ExitCodeOf(err))
	}
	if len(deskpr) != 0 {
		t.Fatalf("refused but still called deskpr: %v", deskpr)
	}
}

func TestOwnerNameFromRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:example-org/example-repo.git":       "example-org/example-repo",
		"ssh://git@github.com/example-org/example-repo.git": "example-org/example-repo",
		"https://github.com/example-org/example-repo.git":   "example-org/example-repo",
		"https://github.com/example-org/example-repo":       "example-org/example-repo",
		"  git@github.com:example-org/example-repo.git\n":   "example-org/example-repo",
		"not-a-remote": "",
		"":             "",
	}
	for in, want := range cases {
		if got := ownerNameFromRemote(in); got != want {
			t.Errorf("ownerNameFromRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func showOnBranch(t *testing.T, root, branch, path string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "show", branch+":"+path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git show %s:%s: %v: %s", branch, path, err, out)
	}
	return string(out)
}
