package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A launch hands a cell to a later cellctl process in several places: the model-policy hook, the
// native container console, the comms and role pane commands, a scratch run's environment. Each
// must name the cells root the launch itself loaded the cell from.

// defaultsPolicyCell adds a house cell named demo to the fixture's cells root. Its cell.env
// names no model policy: the policy comes from the cells root's defaults.env alone.
func defaultsPolicyCell(t *testing.T, f *policyFixture) (cellDir, policy string) {
	t.Helper()
	cellDir = filepath.Join(f.cellsRoot, "demo")
	cfg := filepath.Join(cellDir, "home", ".config", "assay")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "roster.env"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := "CELL=demo\nCELL_KIND=house\n" +
		"CELL_ROOTS=example-org/example-repo=" + f.repoDir + "\n" +
		"CELL_REPO=" + f.repoDir + "\n"
	if err := os.WriteFile(filepath.Join(cellDir, "cell.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	// The policy lives outside every cell directory, named by an absolute path.
	policy = filepath.Join(filepath.Dir(f.cellsRoot), "machine-policy.json")
	src, err := os.Open(f.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.cellsRoot, cellDefaultsFile), []byte("CELL_MODEL_POLICY="+policy+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return cellDir, policy
}

// regression: PR #2971 (cell names narrowed to [a-z0-9-]; renamed from the
// nested-cell form of this test)
//
// TestBinaryCellHookReadsTheLaunchDefaults drives the whole path for a cell named demo whose
// model policy is named ONLY by the cells root's defaults.env: a real launch installs the hook,
// and the hook command that launch wrote — run as Claude Code runs it, with no CELLS_ROOT in its
// environment — must find the same policy and so allow what the policy allows and refuse what it
// refuses.
func TestBinaryCellHookReadsTheLaunchDefaults(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	cellDir, policy := defaultsPolicyCell(t, f)

	r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "demo", "pr-review-desk")
	if r.code != 0 {
		t.Fatalf("launch: exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	settings := ""
	argv := launchArgv(r.stdout)
	for i, a := range argv {
		if a == "--settings" && i+1 < len(argv) {
			settings = argv[i+1]
		}
	}
	command := launchHookCommand(t, settings)
	if want := "--cells-root " + bashQuote(f.cellsRoot) + " "; !strings.Contains(command, want) {
		t.Fatalf("the hook command does not name the cells root the launch used (%s):\n%s", want, command)
	}
	if !strings.Contains(command, bashQuote(cellDir)) {
		t.Fatalf("the hook command does not name the cell directory:\n%s", command)
	}

	if code, stderr := runHook(t, f, command, agentEvent("opus")); code != 0 {
		t.Errorf("a child the machine-wide policy allows: exit %d, want 0; stderr: %s", code, stderr)
	}
	if code, stderr := runHook(t, f, command, agentEvent("unknown-model")); code != wantBlock || !strings.Contains(stderr, "model-policy:") {
		t.Errorf("a child the machine-wide policy refuses: exit %d, want %d; stderr: %s", code, wantBlock, stderr)
	}
	// The root is the one on the command line, whatever the window's environment carries.
	if code, stderr := runHookEnv(t, f, command, agentEvent("opus"), "CELLS_ROOT="+filepath.Join(f.cellsRoot, "team")); code != 0 {
		t.Errorf("with another CELLS_ROOT in the hook's environment: exit %d, want 0; stderr: %s", code, stderr)
	}

	// A command that names a root the cell is not under is refused, not re-rooted.
	sha := policySHA(t, policy)
	hook := func(pre []string, dir string) string {
		parts := append([]string{cellctlBinary(t)}, pre...)
		parts = append(parts, "model-policy", "hook", dir, "pr-review-desk", "anthropic", "", "claude", sha)
		for i, p := range parts {
			parts[i] = bashQuote(p)
		}
		return strings.Join(parts, " ")
	}
	elsewhere := t.TempDir()
	if code, stderr := runHook(t, f, hook([]string{"--cells-root", elsewhere}, cellDir), agentEvent("opus")); code != wantBlock || !strings.Contains(stderr, "is not under the cells root") {
		t.Errorf("a cell directory outside the named root: exit %d, want %d; stderr: %s", code, wantBlock, stderr)
	}
	// The hook's OWN refusal, by its text: the loader refuses these shapes too (a cell is never
	// named `.` or by a path that leaves the root), so the exit code alone would not show that
	// the hook's check is there.
	notUnder := func(dir, root string) string {
		return "cell directory " + dir + " is not under the cells root " + root + " this window launched from"
	}
	// The root itself is not a cell under it.
	if code, stderr := runHook(t, f, hook([]string{"--cells-root", cellDir}, cellDir), agentEvent("opus")); code != wantBlock || !strings.Contains(stderr, notUnder(cellDir, cellDir)) {
		t.Errorf("the cell directory named as its own root: exit %d, want %d and %q; stderr: %s", code, wantBlock, notUnder(cellDir, cellDir), stderr)
	}
	// A directory whose path merely BEGINS with the root's path is not under it: `<root>-other`
	// is the root's sibling. A containment test on the strings would let it through.
	sibling := filepath.Join(f.cellsRoot+"-other", "demo")
	if err := os.MkdirAll(sibling, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cellDir, "cell.env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, "cell.env"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sibling, f.cellsRoot) {
		t.Fatalf("the sibling fixture %s does not share the root's path as a prefix (%s)", sibling, f.cellsRoot)
	}
	if code, stderr := runHook(t, f, hook([]string{"--cells-root", f.cellsRoot}, sibling), agentEvent("opus")); code != wantBlock || !strings.Contains(stderr, notUnder(sibling, f.cellsRoot)) {
		t.Errorf("a cell directory beside the root, sharing its path as a prefix: exit %d, want %d and %q; stderr: %s", code, wantBlock, notUnder(sibling, f.cellsRoot), stderr)
	}
	// The same explicit command with the right root is the control for the two refusals above.
	if code, stderr := runHook(t, f, hook([]string{"--cells-root", f.cellsRoot}, cellDir), agentEvent("opus")); code != 0 {
		t.Errorf("the hand-built command with the launch root: exit %d, want 0; stderr: %s", code, stderr)
	}
	// A command with no --cells-root at all (settings an older cellctl wrote) takes the cell
	// directory's parent, which for a cell named by one directory IS the cells root.
	if code, stderr := runHook(t, f, hook(nil, cellDir), agentEvent("opus")); code != 0 {
		t.Errorf("a flagless hook command: exit %d, want 0; stderr: %s", code, stderr)
	}
}

// launchHookCommand is the PreToolUse hook command in a launch's --settings JSON.
func launchHookCommand(t *testing.T, settings string) string {
	t.Helper()
	var s launchSettings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		t.Fatalf("--settings is not JSON: %v\n%s", err, settings)
	}
	pre := s.Hooks["PreToolUse"]
	if len(pre) == 0 || len(pre[0].Hooks) == 0 {
		t.Fatalf("no PreToolUse hook in the launch's --settings: %s", settings)
	}
	return pre[0].Hooks[0].Command
}
