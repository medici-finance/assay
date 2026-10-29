//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The forge endpoint keys a launched role window must carry from its own cell.env (#4841).
// desktoken reads GITLAB_API_BASE from its process environment and has no default, so a role
// window that does not carry the cell's base cannot cold-mint, however the cell file is set.
const (
	forgeFixtureBase      = "https://gitlab.example.com/api/v4"
	forgeFixtureInherited = "https://gitlab.inherited.example/api/v4"
)

// forgeEnvStub prints whether each forge key is set in the harness process and its value, as
// `KEY=<set-or-empty>|<value>`, so "absent" and "set to empty" stay distinguishable.
const forgeEnvStub = "#!/bin/sh\ncase \"$1\" in\n--version) echo '2.1.278 (stub)'; exit 0;;\nplugin) exit 0;;\nesac\n" +
	"printf 'GITLAB_API_BASE=%s|%s\\n' \"${GITLAB_API_BASE+set}\" \"${GITLAB_API_BASE-}\"\n" +
	"printf 'FORGE_API_BASE=%s|%s\\n' \"${FORGE_API_BASE+set}\" \"${FORGE_API_BASE-}\"\n" +
	"exit 0\n"

func forgeLine(key, value string, set bool) string {
	if !set {
		return key + "=|\n"
	}
	return key + "=set|" + value + "\n"
}

func writeForgeCellEnv(t *testing.T, f *policyFixture, extra string) {
	t.Helper()
	env := "CELL=example\nCELL_KIND=house\n" +
		"CELL_ROOTS=example-org/example-repo=" + f.repoDir + "\n" +
		"CELL_REPO=" + f.repoDir + "\n" + extra
	must(t, os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(env), 0o600))
}

type forgeEnvCase struct {
	name, cellEnv string
	shell         []string
	gitlab, forge string
	gitlabSet     bool
	forgeSet      bool
}

var forgeEnvCases = []forgeEnvCase{
	// regression: #4841 — a non-default base the cell holds reaches the role window.
	{name: "cell base reaches child",
		cellEnv: "CELL_FORGE=gitlab\nGITLAB_API_BASE=" + forgeFixtureBase + "\nFORGE_API_BASE=" + forgeFixtureBase + "\n",
		gitlab:  forgeFixtureBase, gitlabSet: true, forge: forgeFixtureBase, forgeSet: true},
	// regression: #4841 — precedence: the cell's own value wins over one the launching shell carries.
	{name: "cell base wins over inherited",
		cellEnv: "CELL_FORGE=gitlab\nGITLAB_API_BASE=" + forgeFixtureBase + "\nFORGE_API_BASE=" + forgeFixtureBase + "\n",
		shell:   []string{"GITLAB_API_BASE=" + forgeFixtureInherited, "FORGE_API_BASE=" + forgeFixtureInherited},
		gitlab:  forgeFixtureBase, gitlabSet: true, forge: forgeFixtureBase, forgeSet: true},
	// A cell without the keys leaves the child exactly as inherited: the shell's value passes
	// through untouched and the compiled FORGE_API_BASE derivation is never added.
	{name: "cell without keys leaves child unchanged",
		shell:  []string{"GITLAB_API_BASE=" + forgeFixtureInherited},
		gitlab: forgeFixtureInherited, gitlabSet: true},
	// A GitLab cell with no base configured: cellctl's compiled gitlab.com fallback is NOT
	// forwarded, so desktoken's deliberate no-default refusal still stands.
	{name: "compiled gitlab default not forwarded",
		cellEnv: "CELL_FORGE=gitlab\n"},
	// An empty cell value is unset to desktoken; it must not clobber an inherited value.
	{name: "empty cell value does not clobber inherited",
		cellEnv: "GITLAB_API_BASE=\n",
		shell:   []string{"GITLAB_API_BASE=" + forgeFixtureInherited},
		gitlab:  forgeFixtureInherited, gitlabSet: true},
}

func checkForgeEnv(t *testing.T, tc forgeEnvCase, stdout string) {
	t.Helper()
	for _, want := range []string{
		forgeLine("GITLAB_API_BASE", tc.gitlab, tc.gitlabSet),
		forgeLine("FORGE_API_BASE", tc.forge, tc.forgeSet),
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("role window lacks %q:\n%s", strings.TrimSpace(want), stdout)
		}
	}
}

// regression: #4841
// TestLaunchForwardsCellForgeBase drives the real binary's claude launch with a stub harness that
// prints the forge keys it received.
func TestLaunchForwardsCellForgeBase(t *testing.T) {
	for _, tc := range forgeEnvCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPolicyFixture(t, "2.1.278")
			f.prepareLocalLaunch(t)
			must(t, os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(forgeEnvStub), 0o755))
			writeForgeCellEnv(t, f, tc.cellEnv)
			r := f.run(t, append([]string{"CELLCTL_DESKWT=0"}, tc.shell...), "desk", "example", "worker-desk", "--cadence", "off")
			if r.code != 0 {
				t.Fatalf("launch exit=%d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
			}
			checkForgeEnv(t, tc, r.stdout)
		})
	}
}

// regression: #4841
// TestCursorLaunchForwardsForgeBase is the same contract on the cursor arm, whose environment
// filter (cursorLaunchEnv) runs after the keys are composed and must keep them.
func TestCursorLaunchForwardsForgeBase(t *testing.T) {
	for _, tc := range forgeEnvCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPolicyFixture(t, "")
			f.prepareLocalLaunch(t)
			writeCursorWorkspace(t, f.repoDir, "worker-desk")
			git := exec.Command("git", "-C", f.repoDir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid",
				"-c", "commit.gpgsign=false", "commit", "-q", "-m", "cursor workspace")
			git.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(f.cellDir, "absent-gitconfig"), "GIT_CONFIG_NOSYSTEM=1")
			add := exec.Command("git", "-C", f.repoDir, "add", "-A")
			add.Env = git.Env
			if out, err := add.CombinedOutput(); err != nil {
				t.Fatalf("git add: %v %s", err, out)
			}
			if out, err := git.CombinedOutput(); err != nil {
				t.Fatalf("git commit: %v %s", err, out)
			}
			must(t, os.WriteFile(filepath.Join(f.binDir, "agent"), []byte(forgeEnvStub), 0o755))
			writeForgeCellEnv(t, f, "CELL_HARNESS=cursor\nCURSOR_MODEL_default=fixture-cursor-model\n"+tc.cellEnv)
			r := f.run(t, append([]string{"CELLCTL_DESKWT=0"}, tc.shell...), "desk", "example", "worker-desk", "--cadence", "off")
			if r.code != 0 {
				t.Fatalf("launch exit=%d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
			}
			checkForgeEnv(t, tc, r.stdout)
		})
	}
}

// TestCursorLaunchEnvKeepsForgeKeys pins the filter itself: it drops Claude's endpoint and model
// overrides, never the forge endpoint the desk verbs mint against.
func TestCursorLaunchEnvKeepsForgeKeys(t *testing.T) {
	in := []string{"GITLAB_API_BASE=" + forgeFixtureBase, "FORGE_API_BASE=" + forgeFixtureBase, "ANTHROPIC_BASE_URL=https://example.invalid", "CLAUDE_CONFIG_DIR=/example/cfg"}
	out := cursorLaunchEnv(in)
	if envValue(out, "GITLAB_API_BASE") != forgeFixtureBase || envValue(out, "FORGE_API_BASE") != forgeFixtureBase {
		t.Fatalf("cursorLaunchEnv dropped a forge key: %v", out)
	}
	if _, ok := envLookup(out, "ANTHROPIC_BASE_URL"); ok {
		t.Fatalf("cursorLaunchEnv kept a Claude endpoint: %v", out)
	}
}
