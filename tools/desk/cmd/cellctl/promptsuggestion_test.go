package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file exercises the BUILT cellctl binary for assay#1436: Claude Code's next-prompt
// suggestions must be disabled by DEFAULT on every claude-arm launch this tool composes — a
// house/k8s cell, a scrubbed desk/smoke session, and (via the image defaults documented in
// images/assay-harness/Dockerfile and containers/base/Dockerfile) a container launch — because
// setting the var in an interactive shell covers none of those. Codex is a different harness and
// keeps whatever it already had: this repo touches only the claude-only knob.
//
// Every fixture cell is a "house" kind with NO model policy — the smallest cell that reaches a
// REAL (non-dry-run) launch. Git operations use only a temporary filesystem-origin repo; the stub
// `claude`/`codex` binaries print selected env keys instead of contacting any model endpoint, so
// no live infrastructure is touched (C3).

// promptSuggestionFixture is a minimal house cell wired for a real (non-dry-run) launch.
type promptSuggestionFixture struct {
	cellsRoot string
	cellDir   string
	cfgDir    string
	binDir    string
	repoDir   string
}

func newPromptSuggestionFixture(t *testing.T) *promptSuggestionFixture {
	t.Helper()
	root := t.TempDir()
	f := &promptSuggestionFixture{
		cellsRoot: filepath.Join(root, "cells"),
		cfgDir:    filepath.Join(root, "claude-config"),
		binDir:    filepath.Join(root, "bin"),
		repoDir:   filepath.Join(root, "repo"),
	}
	f.cellDir = filepath.Join(f.cellsRoot, "example")
	for _, d := range []string{
		filepath.Join(f.cellDir, "home", ".config", "assay"),
		f.cfgDir, f.binDir, f.repoDir,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.cellDir, "home", ".config", "assay", "roster.env"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := "CELL=example\nCELL_KIND=house\n" +
		"CELL_ROOTS=example-org/example-repo=" + f.repoDir + "\n" +
		"CELL_REPO=" + f.repoDir + "\n"
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", f.repoDir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(f.cellDir, "absent-gitconfig"), "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v %s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture")
	git("remote", "add", "origin", f.repoDir)

	// The stub `claude`: answers --version/plugin enable the same way the real CLI's exit-1
	// "already enabled" NOTICE path is tolerated (deskLaunch), then prints the ONE env key this
	// brief cares about, so the test asserts on the child's actual environment, not on cellctl's
	// own composed slice.
	claudeScript := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  --version) echo '2.1.278 (stub)'; exit 0;;\n" +
		"  plugin) exit 0;;\n" +
		"esac\n" +
		"printf 'PROMPT_SUGGESTION=%s\\n' \"${CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION-<unset>}\"\n"
	if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(claudeScript), 0o755); err != nil {
		t.Fatal(err)
	}
	codexScript := "#!/bin/sh\nprintf 'PROMPT_SUGGESTION=%s\\n' \"${CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION-<unset>}\"\n"
	if err := os.WriteFile(filepath.Join(f.binDir, "codex"), []byte(codexScript), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *promptSuggestionFixture) run(t *testing.T, extraEnv []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(cellctlBinary(t), args...)
	base := []string{
		"CELLS_ROOT=" + f.cellsRoot,
		"CLAUDE_CONFIG_DIR=" + f.cfgDir,
		"PATH=" + f.binDir + ":" + os.Getenv("PATH"),
		"HOME=" + f.cellDir,
		"KUBECONFIG=/dev/null",
		// deskwt role-init is not on this fixture's PATH; forcing the fallback path keeps this
		// launch filesystem-only, same precedent as provider_defaults_test.go's own launch fixture.
		"CELLCTL_DESKWT=0",
	}
	cmd.Env = append(base, extraEnv...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running cellctl %v: %v", args, err)
		}
	}
	return runResult{out.String(), errb.String(), code}
}

// TestBinaryPromptSuggestionInheritedTrueOverriddenToFalse is the acceptance test assay#1436
// asks for: exercise the BUILT launcher with a stub claude process, with
// CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=true INHERITED in the parent (launching-shell) environment
// before invoking the launcher — the child must receive "false" anyway. The launcher's own
// explicit default wins over an inherited value; an inherited true is exactly what an interactive
// shell that once `export`ed it, or a scheduler's ambient environment, would carry.
func TestBinaryPromptSuggestionInheritedTrueOverriddenToFalse(t *testing.T) {
	f := newPromptSuggestionFixture(t)
	r := f.run(t, []string{"CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=true"}, "desk", "example", "worker-desk")
	if r.code != 0 {
		t.Fatalf("launch: %+v", r)
	}
	if !strings.Contains(r.stdout, "PROMPT_SUGGESTION=false\n") {
		t.Errorf("child did not receive the disabled default over an inherited true:\n%s", r.stdout)
	}
}

// TestBinaryPromptSuggestionUnsetParentStillDisabled proves this is a real DEFAULT, not merely an
// override of an inherited value: with nothing set in the parent at all, the child still gets
// false.
func TestBinaryPromptSuggestionUnsetParentStillDisabled(t *testing.T) {
	f := newPromptSuggestionFixture(t)
	r := f.run(t, nil, "desk", "example", "worker-desk")
	if r.code != 0 {
		t.Fatalf("launch: %+v", r)
	}
	if !strings.Contains(r.stdout, "PROMPT_SUGGESTION=false\n") {
		t.Errorf("child did not default to disabled:\n%s", r.stdout)
	}
}

// TestBinaryPromptSuggestionCodexArmUntouched retains Codex behavior: codex is a different
// harness with no equivalent knob, so this repo must not touch its env var — whatever the parent
// carries reaches the codex child unchanged.
func TestBinaryPromptSuggestionCodexArmUntouched(t *testing.T) {
	f := newPromptSuggestionFixture(t)
	r := f.run(t, []string{"CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=true"}, "desk", "example", "worker-desk", "--harness", "codex")
	if r.code != 0 {
		t.Fatalf("codex launch: %+v", r)
	}
	if !strings.Contains(r.stdout, "PROMPT_SUGGESTION=true\n") {
		t.Errorf("the codex arm must retain whatever it inherited untouched:\n%s", r.stdout)
	}
}

// TestBinaryPromptSuggestionShownInDryRun covers the issue's "show the setting in dry-run output"
// ask: a DRY_RUN=1 boot must display the composed default so an operator can see it without a
// live launch.
func TestBinaryPromptSuggestionShownInDryRun(t *testing.T) {
	f := newPromptSuggestionFixture(t)
	r := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
	if r.code != 0 {
		t.Fatalf("dry-run: %+v", r)
	}
	if !strings.Contains(r.stdout, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false") {
		t.Errorf("dry-run does not show the disabled default composed into the launch:\n%s", r.stdout)
	}
}

// TestBinaryPromptSuggestionScrubbedLaunchDisabled covers the scrubbed-launch path the issue
// singles out (a scrubbed desk/smoke session has no inheritable shell to `export` into at all):
// the composed `env -i` launch must carry the disabled default on the claude arm.
func TestBinaryPromptSuggestionScrubbedLaunchDisabled(t *testing.T) {
	f := newPromptSuggestionFixture(t)
	scrubbedHome := filepath.Join(f.cellDir, "home")
	if err := os.MkdirAll(filepath.Join(scrubbedHome, ".config", "assay"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scrubbedHome, ".config", "assay", "roster.env"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	scrubbedEnv := "CELL=example\nCELL_KIND=scrubbed\nCELL_REPO_SLUG=example-org/example-repo\n" +
		"CELL_REPO=" + f.repoDir + "\n"
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(scrubbedEnv), 0o644); err != nil {
		t.Fatal(err)
	}
	// A scrubbed dry run prints its own [plan] block, composed through scrubbedComposeEnv, which
	// is what this test checks — no tmux/live launch needed.
	r := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk")
	if r.code != 0 {
		t.Fatalf("scrubbed dry-run: %+v", r)
	}
	if !strings.Contains(r.stdout, "[plan] env CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false") {
		t.Errorf("scrubbed launch plan does not carry the disabled default:\n%s", r.stdout)
	}
}
