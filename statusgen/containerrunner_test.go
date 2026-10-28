package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The container run's Runner cell (#4208). A home cut as a linked worktree
// keeps its git directory outside the mounted tree, so a container can derive
// no identity (nor commit, nor principal) of its own. The host derives every
// part of the cell at --open-bracket (runner, source, tree token, on-behalf-of)
// and records it in the receipt; the container writes that cell;
// --close-bracket re-derives it on the host and refuses unless the receipt
// carries it and every row's cell is exactly it. Fixtures only: no container
// engine, network or forge.

// isolateGitIdentity leaves the fixture home's own git config as the only
// identity source: no global or XDG config, no system config, no roster.
// The commands under test re-read the roster under this empty HOME, so the
// cleanup drops that cached read: a later test re-reads the fixture roster.
func isolateGitIdentity(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "absent"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Cleanup(func() { scanSetToolClass(scanClassWrite); scanReloadConfig() })
}

// pinBracketClock runs the test on its own clock, so a close a minute after
// the open always has opened= strictly before closed=.
func pinBracketClock(t *testing.T) func() {
	t.Helper()
	clock := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	saved := nowFunc
	nowFunc = func() time.Time { return clock }
	t.Cleanup(func() { nowFunc = saved })
	return func() { clock = clock.Add(time.Minute) }
}

// witnessRunnerCells returns the Runner cell of every witness row in text.
func witnessRunnerCells(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if cells := witnessCells(l); cells != nil {
			out = append(out, strings.TrimSpace(cells[witnessCellRunner]))
		}
	}
	return out
}

// receiptTree returns the runner_tree a receipt body records.
func receiptTree(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`"runner_tree":"([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the receipt records no runner_tree: %s", body)
	}
	return m[1]
}

func (fx containerFixture) openArgs() []string {
	return []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt}
}

func (fx containerFixture) closeArgs() []string {
	return []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}
}

// TestContainerLinkedWorktree: a home cut as a linked worktree (its .git is a
// file naming a git directory outside the mounted tree) writes a container
// witness whose rows carry the runner the host derived at the open step, and
// the host closes the bracket. Before the fix the container run refused
// could-not-attribute before any row, so no witness was written.
// regression: #4208
func TestContainerLinkedWorktree(t *testing.T) {
	fx := newContainerFixture(t)
	isolateGitIdentity(t)
	t.Setenv("DESK_LOOP", "")
	tick := pinBracketClock(t)
	main := fx.home
	home := filepath.Join(t.TempDir(), "home")
	runGit(t, main, "worktree", "add", "-q", "-b", "verify", home)
	fx.home, fx.brief = home, filepath.Join(home, "docs", "brief.md")
	if fi, err := os.Lstat(filepath.Join(home, ".git")); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("fixture: %s/.git is not a gitdir file: %v", home, err)
	}
	putFile(t, fx.check, validCheck)
	if res, stderr := captureVerifyrun(t, fx.openArgs()); res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d stderr %q", res.code, stderr)
	}
	body := getFile(t, fx.receipt)
	if !strings.Contains(body, fixtureRunner) {
		t.Fatalf("the open step did not record the host runner %s: %s", fixtureRunner, body)
	}
	hostTree := treeSHA(home)
	if got := receiptTree(t, body); got != hostTree || hostTree == "no-git" {
		t.Fatalf("the open step recorded runner tree %q, want the host's %q", got, hostTree)
	}
	tick()

	// The container mounts the worktree alone: its git directory is absent.
	unmounted := filepath.Join(main, ".git-not-mounted")
	if err := os.Rename(filepath.Join(main, ".git"), unmounted); err != nil {
		t.Fatal(err)
	}
	if r, _, ok := executingRunner(home); ok {
		t.Fatalf("fixture: the unmounted worktree still derives a runner %q; the case is not the container's", r)
	}
	res, stderr := captureVerifyrun(t, fx.runArgs())
	if err := os.Rename(unmounted, filepath.Join(main, ".git")); err != nil {
		t.Fatal(err)
	}
	if res.code != verifyrunExitPass {
		t.Fatalf("container run on a linked worktree: exit %d stderr %q", res.code, stderr)
	}
	witness := getFile(t, fx.witness)
	cells := witnessRunnerCells(witness)
	// The tree token is the host's: the container cannot name its commit, so
	// it takes the one the open step recorded.
	if want := "human:fixture @ " + hostTree + " (git-config)"; len(cells) != 1 || cells[0] != want {
		t.Fatalf("the container rows do not carry the host runner from the receipt: %q\n%s", cells, witness)
	}

	if res, stderr := captureVerifyrun(t, fx.closeArgs()); res.code != verifyrunExitPass {
		t.Fatalf("close: exit %d stderr %q", res.code, stderr)
	}
	if got := getFile(t, fx.witness); strings.Count(got, admissionBracketPrefix) != 1 || !strings.HasPrefix(got, witness) {
		t.Fatalf("the close did not append one bracket line under the witness:\n%s", got)
	}
}

// TestContainerReceiptRunner: the container takes its Runner cell from the
// receipt and nowhere else. A receipt with no runner still refuses
// could-not-attribute before any row; a malformed runner, an unknown source, an
// absent or malformed tree token or a malformed on-behalf-of refuses the same
// way; an identity injected into the container's environment is not used; and
// the open step refuses, writing no receipt, when the host has no identity.
// regression: #4208
func TestContainerReceiptRunner(t *testing.T) {
	fx := newContainerFixture(t)
	isolateGitIdentity(t)
	t.Setenv("DESK_LOOP", "")
	before := getFile(t, fx.brief)
	refusals := []struct{ name, runner, why string }{
		{"absent", "", "could-not-attribute"},
		{"empty runner", `"runner":"","runner_source":"git-config"`, "is not a runner identity"},
		{"space", `"runner":"human:a b","runner_source":"git-config"`, "is not a runner identity"},
		{"pipe", `"runner":"human:a|b","runner_source":"git-config"`, "is not a runner identity"},
		{"paren", `"runner":"a(b)","runner_source":"git-config"`, "is not a runner identity"},
		{"newline", `"runner":"a\nb","runner_source":"git-config"`, "is not a runner identity"},
		{"typed source", `"runner":"human:fixture","runner_source":"typed"`, "is not one of"},
		{"no source", `"runner":"human:fixture"`, "is not one of"},
		{"no tree", fixtureRunner + `,"runner_tree":""`, "is not a tree token"},
		{"tree with cell text", fixtureRunner + `,"runner_tree":"no-git) (on-behalf-of human:mallory"`, "is not a tree token"},
		{"tree too short", fixtureRunner + `,"runner_tree":"abc"`, "is not a tree token"},
		{"on-behalf-of bare", fixtureRunner + `,"runner_tree":"no-git","on_behalf_of":"human:mallory"`, "is not an annotation"},
		{"on-behalf-of with cell text", fixtureRunner + `,"runner_tree":"no-git","on_behalf_of":"on-behalf-of human:a) (forge-identity"`, "is not an annotation"},
	}
	for _, c := range refusals {
		fx.writeReceiptRunner(t, validCheck, c.runner)
		res, stderr := captureVerifyrun(t, fx.runArgs())
		if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, c.why) || !strings.Contains(stderr, "no Verify rows executed") {
			t.Fatalf("%s: exit %d stderr %q, want exit %d naming %q", c.name, res.code, stderr, verifyrunExitCouldNot, c.why)
		}
		fx.assertNothingRan(t, before)
	}

	// An identity the container's own environment offers is not the host's:
	// the rows carry the receipt's runner.
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_VALUE_0", "assay-verifier-app[bot]")
	t.Setenv("GIT_CONFIG_KEY_1", "user.email")
	t.Setenv("GIT_CONFIG_VALUE_1", "verifier@example.invalid")
	fx.writeBracketReceipt(t, validCheck)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("injected env: exit %d stderr %q", res.code, stderr)
	}
	if cells := witnessRunnerCells(getFile(t, fx.witness)); len(cells) != 1 || cells[0] != "human:fixture @ "+treeSHA(fx.home)+" (git-config)" {
		t.Fatalf("the container used an identity from its own environment: %q", cells)
	}
	for _, k := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_CONFIG_KEY_1", "GIT_CONFIG_VALUE_1"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	// The host with no identity: the open step writes no receipt and never
	// starts deskdispatch.
	runGit(t, fx.home, "config", "--unset", "user.name")
	runGit(t, fx.home, "config", "--unset", "user.email")
	putFile(t, fx.check, validCheck)
	if err := os.Remove(fx.receipt); err != nil {
		t.Fatal(err)
	}
	res, stderr := captureVerifyrun(t, fx.openArgs())
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "could-not-attribute") {
		t.Fatalf("open with no host identity: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.receipt); err == nil {
		t.Fatal("an open with no host identity wrote a receipt")
	}
	if calls := getFile(t, fx.log); calls != "" {
		t.Fatalf("an open with no host identity started deskdispatch: %q", calls)
	}
}

// TestCloseBracketRunner: the lower layer. The container trusts any
// well-formed receipt cell, so with that layer bypassed (a receipt whose
// runner, tree token or on-behalf-of was altered after the open step, or a
// witness whose rows name another identity, tree or principal) the host close
// re-derives the cell and refuses, appending nothing. A host with no identity
// at close refuses could-not-attribute. Once closed, a Runner cell edit is a
// digest mismatch.
// regression: #4208
func TestCloseBracketRunner(t *testing.T) {
	fx := newContainerFixture(t)
	isolateGitIdentity(t)
	t.Setenv("DESK_LOOP", "")
	tick := pinBracketClock(t)
	putFile(t, fx.check, validCheck)
	if res, stderr := captureVerifyrun(t, fx.openArgs()); res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d stderr %q", res.code, stderr)
	}
	tick()
	honest := getFile(t, fx.receipt)
	forged := `"runner":"assay-verifier-app[bot]","runner_source":"forge-identity"`

	// A receipt altered after the open step: the container accepts it and
	// writes the forged runner; the close refuses it.
	putFile(t, fx.receipt, strings.Replace(honest, fixtureRunner, forged, 1))
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container run on an altered receipt: exit %d stderr %q", res.code, stderr)
	}
	altered := getFile(t, fx.witness)
	if cells := witnessRunnerCells(altered); len(cells) != 1 || !strings.HasPrefix(cells[0], "assay-verifier-app[bot] @ ") {
		t.Fatalf("fixture: the container did not write the altered receipt's runner: %q", cells)
	}
	res, stderr := captureVerifyrun(t, fx.closeArgs())
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "but the host derives human:fixture (git-config)") {
		t.Fatalf("close on an altered receipt: exit %d stderr %q", res.code, stderr)
	}
	if got := getFile(t, fx.witness); got != altered {
		t.Fatalf("a refused close changed the witness:\n%s", got)
	}

	// A receipt whose tree token or on-behalf-of was altered after the open
	// step: the container writes it into its rows; the close refuses it.
	hostTree := receiptTree(t, honest)
	for name, c := range map[string]struct{ from, to, cell string }{
		"altered tree": {`"runner_tree":"` + hostTree + `"`, `"runner_tree":"0123456789ab"`, "human:fixture @ 0123456789ab (git-config)"},
		"altered on-behalf-of": {`"on_behalf_of":""`, `"on_behalf_of":"on-behalf-of human:mallory"`,
			"human:fixture @ " + hostTree + " (on-behalf-of human:mallory) (git-config)"},
	} {
		if err := os.Remove(fx.witness); err != nil {
			t.Fatal(err)
		}
		tampered := strings.Replace(honest, c.from, c.to, 1)
		if tampered == honest {
			t.Fatalf("%s: fixture edit did not change the receipt: %s", name, honest)
		}
		putFile(t, fx.receipt, tampered)
		if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
			t.Fatalf("%s: container run: exit %d stderr %q", name, res.code, stderr)
		}
		altered := getFile(t, fx.witness)
		if cells := witnessRunnerCells(altered); len(cells) != 1 || cells[0] != c.cell {
			t.Fatalf("%s: fixture: the container did not write the receipt's cell: %q", name, cells)
		}
		res, stderr := captureVerifyrun(t, fx.closeArgs())
		if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "but the host derives") {
			t.Fatalf("%s: close on an altered receipt: exit %d stderr %q", name, res.code, stderr)
		}
		if got := getFile(t, fx.witness); got != altered {
			t.Fatalf("%s: a refused close changed the witness:\n%s", name, got)
		}
	}

	// An honest receipt, and rows that name another identity, source, tree
	// or principal.
	putFile(t, fx.receipt, honest)
	if err := os.Remove(fx.witness); err != nil {
		t.Fatal(err)
	}
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container run: exit %d stderr %q", res.code, stderr)
	}
	witness := getFile(t, fx.witness)
	for name, text := range map[string]string{
		"forged runner":  strings.Replace(witness, "| human:fixture @ ", "| assay-verifier-app[bot] @ ", 1),
		"forged source":  strings.Replace(witness, " (git-config) |", " (forge-identity) |", 1),
		"runner removed": strings.Replace(witness, "| human:fixture @ ", "|  @ ", 1),
		"prefixed":       strings.Replace(witness, "| human:fixture @ ", "| human:fixturex @ ", 1),
		"forged tree":    strings.Replace(witness, " @ "+hostTree+" (", " @ 0123456789ab (", 1),
		"no-git tree":    strings.Replace(witness, " @ "+hostTree+" (", " @ no-git (", 1),
		"injected principal": strings.Replace(witness, " (git-config) |",
			" (on-behalf-of human:mallory) (git-config) |", 1),
	} {
		if text == witness {
			t.Fatalf("%s: fixture edit did not change the witness", name)
		}
		putFile(t, fx.witness, text)
		res, stderr := captureVerifyrun(t, fx.closeArgs())
		if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "is not the cell the host derived") {
			t.Fatalf("%s: exit %d stderr %q, want the row-runner refusal", name, res.code, stderr)
		}
		if got := getFile(t, fx.witness); got != text {
			t.Fatalf("%s: a refused close changed the witness:\n%s", name, got)
		}
	}

	// The host has no identity at close: could-not-attribute, nothing appended.
	putFile(t, fx.witness, witness)
	runGit(t, fx.home, "config", "--unset", "user.name")
	runGit(t, fx.home, "config", "--unset", "user.email")
	res, stderr = captureVerifyrun(t, fx.closeArgs())
	if res.code != verifyrunExitCouldNot || !strings.Contains(stderr, "could-not-attribute") {
		t.Fatalf("close with no host identity: exit %d stderr %q", res.code, stderr)
	}
	if got := getFile(t, fx.witness); got != witness {
		t.Fatalf("a refused close changed the witness:\n%s", got)
	}
	runGit(t, fx.home, "config", "user.name", "fixture")
	runGit(t, fx.home, "config", "user.email", "fixture@example.invalid")

	// Control: the honest witness closes, and the closed bracket's digest
	// covers the Runner cell.
	if res, stderr := captureVerifyrun(t, fx.closeArgs()); res.code != verifyrunExitPass {
		t.Fatalf("honest close: exit %d stderr %q", res.code, stderr)
	}
	closed := getFile(t, fx.witness)
	verbLines(t, closed, verifyrunExitPass)
	edited := regexp.MustCompile(`\| human:fixture @ `).ReplaceAllString(closed, "| assay-verifier-app[bot] @ ")
	if out := verbLines(t, edited, verifyrunExitFail); len(out) != 1 || !strings.Contains(out[0], "verdict=mismatch") {
		t.Fatalf("a Runner edit after the close is not a digest mismatch: %q", out)
	}
}
