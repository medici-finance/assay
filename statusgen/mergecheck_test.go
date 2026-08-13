package main

// mergecheck_test.go — PROOF THAT THE CHECK CAN FAIL (desk-hardening/05).
//
// Every test here is paired: a POSITIVE control that constructs the defect and
// asserts the check goes red, and the mutation of that same fixture with the
// defect removed, asserting it goes green. A check only asserted against a clean
// corpus has never been shown to be connected to anything.
//
// The headline fixture is a faithful reproduction of the collision measured on
// 2026-08-13: statusgen/emit.go's emit() grew an extra parameter on main while
// another open PR still called it with the old arity. Both branches compiled.
// git merged them without a conflict. main went red on merge. TestMergecheck…
// ArityCollision builds exactly that shape in a throwaway repo and requires the
// check to catch it — and requires the same fixture WITHOUT the main-side
// signature change to come back clean, so a check that simply always fails
// cannot pass this file.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepo is a throwaway git repo built commit by commit.
type fixtureRepo struct {
	t   *testing.T
	dir string
}

func newFixtureRepo(t *testing.T) *fixtureRepo {
	t.Helper()
	dir := t.TempDir()
	r := &fixtureRepo{t: t, dir: dir}
	r.git("init", "--quiet", "-b", "main")
	r.git("config", "user.email", "fixture@example.invalid")
	r.git("config", "user.name", "fixture")
	r.git("config", "commit.gpgsign", "false")
	return r
}

func (r *fixtureRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *fixtureRepo) write(rel, content string) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *fixtureRepo) commit(msg string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "--quiet", "--no-gpg-sign", "-m", msg)
}

// run invokes the subcommand exactly as main() does and returns code + combined output.
func (r *fixtureRepo) run(args ...string) (int, string) {
	r.t.Helper()
	var out, errb bytes.Buffer
	code := runMergecheck(append([]string{"--root", r.dir}, args...), &out, &errb)
	return code, out.String() + errb.String()
}

// ── the emit() arity collision ───────────────────────────────────────────────

// arityFixture builds the emit() shape. When mainAddsParam is false the fixture is
// MUTATED so no collision exists — same branches, same merge, defect removed.
func arityFixture(t *testing.T, mainAddsParam bool) *fixtureRepo {
	r := newFixtureRepo(t)

	// Commit 0: the common ancestor. One function, one caller, compiles.
	r.write("go.mod", "module fixture\n\ngo 1.22\n")
	r.write("emit.go", "package main\n\nfunc emit(a int) int { return a }\n")
	r.write("main.go", "package main\n\nfunc main() { _ = emit(1) }\n")
	r.commit("base")

	// main: emit() grows a second parameter, and main's OWN caller is updated with
	// it. Green on its own — this is the half that merged first, in the real case.
	if mainAddsParam {
		r.write("emit.go", "package main\n\nfunc emit(a, b int) int { return a + b }\n")
		r.write("main.go", "package main\n\nfunc main() { _ = emit(1, 2) }\n")
		r.commit("main: emit() takes a second parameter")
	} else {
		r.write("doc.go", "package main\n\n// unrelated change on main, no signature move\n")
		r.commit("main: unrelated")
	}

	// feature, cut from commit 0: a NEW caller at the OLD arity, in its own file.
	// Green on its own. Nothing in this diff mentions emit's signature.
	r.git("checkout", "--quiet", "-b", "feature", "HEAD~1")
	r.write("report.go", "package main\n\nfunc report() int { return emit(9) }\n")
	r.write("use.go", "package main\n\nvar _ = report\n")
	r.commit("feature: a new emit() caller")
	return r
}

func TestMergecheckCatchesMergeIntroducedArityCollision(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a fixture module; skipped under -short")
	}
	r := arityFixture(t, true)

	// Both branches build green on their own. If this precondition ever stops
	// holding the positive control below proves nothing, so it is asserted, not
	// assumed.
	for _, ref := range []string{"main", "feature"} {
		tree := r.git("rev-parse", ref+"^{tree}")
		code, out, err := execOverTree(r.dir, tree, "go build ./...")
		if err != nil {
			t.Fatalf("precondition: could not build %s: %v", ref, err)
		}
		if code != 0 {
			t.Fatalf("precondition failed: %s does not build on its own (exit %d)\n%s", ref, code, out)
		}
	}

	// And they merge without a textual conflict — the property that makes this
	// class invisible to every conflict-based check.
	tm, err := trialMerge(r.dir, "main", "feature")
	if err != nil {
		t.Fatalf("trial merge: %v", err)
	}
	if len(tm.Conflicts) != 0 {
		t.Fatalf("precondition failed: the fixture branches conflict textually (%v); the fixture no longer reproduces the class", tm.Conflicts)
	}

	code, out := r.run("--base", "main", "--head", "feature", "--exec", "go build ./...")
	if code != 1 {
		t.Fatalf("positive control: want exit 1 (checked-failed), got %d\n%s", code, out)
	}
	if !strings.Contains(out, "MERGE-INTRODUCED") {
		t.Errorf("positive control: the finding must be labelled MERGE-INTRODUCED, not merely failed\n%s", out)
	}
	if !strings.Contains(out, "exits 0 on your head") {
		t.Errorf("positive control: the finding must state the branch itself is fine\n%s", out)
	}
}

func TestMergecheckCleanWhenTheArityCollisionIsRemoved(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a fixture module; skipped under -short")
	}
	// The mutation: same two branches, same merge, no signature change on main.
	r := arityFixture(t, false)
	code, out := r.run("--base", "main", "--head", "feature", "--exec", "go build ./...")
	if code != 0 {
		t.Fatalf("negative control: want exit 0, got %d — the check fires on a corpus with no collision\n%s", code, out)
	}
	if strings.Contains(out, "MERGE-INTRODUCED") {
		t.Errorf("negative control: nothing may be reported as merge-introduced\n%s", out)
	}
}

func TestMergecheckAttributesPreExistingBreakageToTheBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a fixture module; skipped under -short")
	}
	// The branch is broken on its own. The merge is innocent, and saying so is the
	// whole stale-vs-broken discrimination: a re-check that blames the merge here
	// sends the worker to the wrong file.
	r := newFixtureRepo(t)
	r.write("go.mod", "module fixture\n\ngo 1.22\n")
	r.write("main.go", "package main\n\nfunc main() {}\n")
	r.commit("base")
	r.git("checkout", "--quiet", "-b", "feature")
	r.write("broken.go", "package main\n\nfunc broken() { this is not go }\n")
	r.commit("feature: broken on its own")

	code, out := r.run("--base", "main", "--head", "feature", "--exec", "go build ./...")
	if !strings.Contains(out, "PRE-EXISTING") {
		t.Errorf("want the failure attributed PRE-EXISTING to the branch\n%s", out)
	}
	if code != 0 {
		t.Errorf("a pre-existing branch failure is not a merge-time finding; want exit 0, got %d\n%s", code, out)
	}
}

// ── the numbering-space collision, seen only in the merged tree ──────────────

// rulesDoc renders a brief-rules.md-shaped file with `extra` appended far from the
// head of the file, so two branches editing different regions merge cleanly.
func rulesDoc(head, tail string) string {
	var b strings.Builder
	b.WriteString("# Brief rules\n\n## Structure\n\n")
	b.WriteString("1. **The first rule.** Body.\n\n")
	b.WriteString(head)
	for i := 0; i < 40; i++ {
		b.WriteString("Filler paragraph that keeps the two edit regions far apart.\n\n")
	}
	b.WriteString("## Later section\n\n")
	b.WriteString("2. **The second rule.** Body.\n\n")
	b.WriteString(tail)
	return b.String()
}

func TestMergecheckCatchesMergeIntroducedNumberingCollision(t *testing.T) {
	r := newFixtureRepo(t)
	r.write(briefRulesRelPath, rulesDoc("", ""))
	r.commit("base rules")

	// main allocates 3 near the top.
	r.write(briefRulesRelPath, rulesDoc("3. **A rule main added.** Body.\n\n", ""))
	r.commit("main: rule 3")

	// feature, from the common ancestor, allocates 3 at the bottom. Neither diff
	// contains the other's number; the file merges cleanly.
	r.git("checkout", "--quiet", "-b", "feature", "HEAD~1")
	r.write(briefRulesRelPath, rulesDoc("", "3. **A rule the branch added.** Body.\n\n"))
	r.commit("feature: rule 3")

	tm, err := trialMerge(r.dir, "main", "feature")
	if err != nil {
		t.Fatalf("trial merge: %v", err)
	}
	if len(tm.Conflicts) != 0 {
		t.Fatalf("precondition failed: the fixture conflicts textually (%v) — it no longer reproduces the class this check is for", tm.Conflicts)
	}

	// Neither tree, read alone, shows the collision. That is the point.
	headContent, ok, err := fileInTree(r.dir, "feature", briefRulesRelPath)
	if err != nil || !ok {
		t.Fatalf("reading the head copy: %v", err)
	}
	if got := numberSpaceCollisions(headContent); len(got) != 0 {
		t.Fatalf("precondition failed: the branch's own copy already collides (%v); a --lint-style read would have caught it and the fixture proves nothing", got)
	}

	code, out := r.run("--base", "main", "--head", "feature")
	if code != 1 {
		t.Fatalf("want exit 1 (checked-failed), got %d\n%s", code, out)
	}
	if !strings.Contains(out, "MERGE-INTRODUCED numbering collision") {
		t.Errorf("want a merge-introduced numbering finding\n%s", out)
	}
	if !strings.Contains(out, "rule number 3 is allocated 2 times") {
		t.Errorf("want the colliding number named\n%s", out)
	}
}

func TestMergecheckCleanWhenTheBranchTakesAFreeNumber(t *testing.T) {
	// The mutation: the branch allocates 4 instead of 3. Same merge, no collision.
	r := newFixtureRepo(t)
	r.write(briefRulesRelPath, rulesDoc("", ""))
	r.commit("base rules")
	r.write(briefRulesRelPath, rulesDoc("3. **A rule main added.** Body.\n\n", ""))
	r.commit("main: rule 3")
	r.git("checkout", "--quiet", "-b", "feature", "HEAD~1")
	r.write(briefRulesRelPath, rulesDoc("", "4. **A rule the branch added.** Body.\n\n"))
	r.commit("feature: rule 4")

	code, out := r.run("--base", "main", "--head", "feature")
	if code != 0 {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if strings.Contains(out, "MERGE-INTRODUCED") {
		t.Errorf("nothing may be reported as merge-introduced\n%s", out)
	}
}

func TestMergecheckReportsPreExistingNumberingCollisionWithoutBlaming(t *testing.T) {
	r := newFixtureRepo(t)
	// The collision is already on the branch's own copy. It is a NOTICE — the merge
	// did not create it, and a re-check that reds the merge for it is noise.
	r.write(briefRulesRelPath, rulesDoc("3. **First three.** Body.\n\n", "3. **Second three.** Body.\n\n"))
	r.commit("base rules with a pre-existing collision")
	r.git("checkout", "--quiet", "-b", "feature")
	r.write("unrelated.md", "nothing to do with rules\n")
	r.commit("feature: unrelated")

	code, out := r.run("--base", "main", "--head", "feature")
	if !strings.Contains(out, "PRE-EXISTING numbering collision") {
		t.Errorf("want the collision attributed as pre-existing\n%s", out)
	}
	if code != 0 {
		t.Errorf("a pre-existing collision must not fail the merge-time check; got %d\n%s", code, out)
	}
}

// ── stale base is a currency fact, not a defect ──────────────────────────────

func TestMergecheckLabelsAMissingCIScriptStaleNotBroken(t *testing.T) {
	// PR #898's shape: main gains .github/scripts/verify-inscope.sh and a job that
	// runs it; a branch cut earlier does not have the file and the job exits 127.
	r := newFixtureRepo(t)
	r.write("README.md", "base\n")
	r.commit("base")
	r.git("checkout", "--quiet", "-b", "feature")
	r.write("branch.md", "branch work\n")
	r.commit("feature: unrelated work")
	r.git("checkout", "--quiet", "main")
	r.write(".github/scripts/verify-inscope.sh", "#!/bin/sh\nexit 0\n")
	r.commit("main: add the CI script the job invokes")

	code, out := r.run("--base", "main", "--head", "feature")
	if !strings.Contains(out, "STALE-BASE") {
		t.Fatalf("want a STALE-BASE notice naming the missing CI-invoked path\n%s", out)
	}
	if !strings.Contains(out, ".github/scripts/verify-inscope.sh") {
		t.Errorf("want the missing path named so the worker can see it is currency, not a bug\n%s", out)
	}
	if !strings.Contains(out, "STALE here, not BROKEN") {
		t.Errorf("the whole point is the wording that separates stale from broken\n%s", out)
	}
	if code != 0 {
		t.Errorf("staleness is advisory, not a finding; want exit 0, got %d\n%s", code, out)
	}
}

func TestMergecheckNoStaleBaseWhenTheBranchHasTheScript(t *testing.T) {
	// The mutation: the branch already carries the script.
	r := newFixtureRepo(t)
	r.write("README.md", "base\n")
	r.write(".github/scripts/verify-inscope.sh", "#!/bin/sh\nexit 0\n")
	r.commit("base with the script")
	r.git("checkout", "--quiet", "-b", "feature")
	r.write("branch.md", "branch work\n")
	r.commit("feature: unrelated work")

	code, out := r.run("--base", "main", "--head", "feature")
	if strings.Contains(out, "STALE-BASE") {
		t.Errorf("no CI-invoked path is missing; nothing may be reported stale\n%s", out)
	}
	if code != 0 {
		t.Errorf("want exit 0, got %d\n%s", code, out)
	}
}

// ── three-state: an instrument that cannot look must not answer ──────────────

func TestMergecheckUnresolvableBaseIsCouldNotCheckNotClean(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("README.md", "base\n")
	r.commit("base")

	code, out := r.run("--base", "refs/remotes/origin/does-not-exist")
	if code != 2 {
		t.Fatalf("an unreachable base must be could-not-check (exit 2), got %d\n%s", code, out)
	}
	if !strings.Contains(out, "could-not-check") {
		t.Errorf("the output must say could-not-check in words\n%s", out)
	}
	if strings.Contains(out, "checked-clean") || strings.Contains(out, "VERDICT: checked-clean") {
		t.Errorf("a run that could not resolve its base must never render as clean — this is the fail-open the brief exists to refuse\n%s", out)
	}
}

func TestMergecheckUnresolvableHeadIsCouldNotCheck(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("README.md", "base\n")
	r.commit("base")

	code, out := r.run("--head", "no-such-ref")
	if code != 2 {
		t.Fatalf("want exit 2, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "could-not-check") {
		t.Errorf("want could-not-check in the output\n%s", out)
	}
}

func TestMergecheckNonRepoRootIsCouldNotCheck(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	code := runMergecheck([]string{"--root", dir}, &out, &errb)
	if code != 2 {
		t.Fatalf("a root that is not a git repo must be could-not-check, got %d\n%s%s", code, out.String(), errb.String())
	}
}

// ── the report always names what it did not look at ─────────────────────────

func TestMergecheckAlwaysPrintsItsBlindSpots(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("README.md", "base\n")
	r.commit("base")
	r.git("checkout", "--quiet", "-b", "feature")
	r.write("x.md", "x\n")
	r.commit("feature")

	code, out := r.run("--base", "main", "--head", "feature")
	if code != 0 {
		t.Fatalf("want a clean run for this fixture, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "CANNOT see") {
		t.Fatalf("a clean verdict must ship with the list of shapes it cannot see\n%s", out)
	}
	for _, b := range blindSpots {
		head := strings.SplitN(b, " ", 3)[0]
		if !strings.Contains(out, head) {
			t.Errorf("blind spot %q is declared but not printed\n%s", head, out)
		}
	}
	if !strings.Contains(out, "and only those") {
		t.Errorf("the clean verdict must be scoped to the probes that ran\n%s", out)
	}
}

func TestMergecheckDefaultBaseIsSpelledInFull(t *testing.T) {
	// A bare `origin/main` resolves through refs/heads first when a local branch
	// named `origin/main` exists — which it does in this repo — and lands the whole
	// re-check on a base dozens of commits behind the real remote tip.
	if defaultMergecheckBase != "refs/remotes/origin/main" {
		t.Fatalf("default base must be the full refs/remotes/ spelling, got %q", defaultMergecheckBase)
	}
}
