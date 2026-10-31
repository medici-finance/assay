package main

import (
	"strings"
	"testing"
)

// regression: #5078
func TestHookGhFilterRefused(t *testing.T) {
	_, wt := gitFixture(t)
	t.Setenv("CLAUDE_PROJECT_DIR", wt)
	ambientEnv(t, "worker-desk", "")
	for _, cmd := range []string{
		`gh api repos/o/r --jq env.GH_X`,
		`gh pr view 1 --json number --jq '$ENV.GH_X'`,
		`gh pr list --json number -q 'env'`,
		`gh release view --json name -qenv.GH_X`,
		`gh pr list --json createdAt --jq 'now'`,
		`gh api x --jq '"\(env.GH_X)"'`,
		`bash -c "gh api x --jq 'env.GH_X'"`,
		`gh pr list --json title | grep -q env`,
	} {
		v, err := ambientRun(t, wt, "Bash", ambientBash(cmd))
		if err != nil {
			t.Fatalf("%q: run: %v", cmd, err)
		}
		if !v.Block {
			t.Fatalf("%q: a gh filter reading beyond its output must be refused", cmd)
		}
		for _, want := range []string{"gh output filter", "$ENV", cmd, "stop"} {
			if !strings.Contains(strings.ToLower(v.Reason), strings.ToLower(want)) {
				t.Fatalf("%q: refusal must contain %q, got:\n%s", cmd, want, v.Reason)
			}
		}
	}
}

// regression: #5078
func TestHookGhFilterAllowed(t *testing.T) {
	_, wt := gitFixture(t)
	t.Setenv("CLAUDE_PROJECT_DIR", wt)
	ambientEnv(t, "worker-desk", "fixture-session")
	for _, cmd := range []string{
		`gh api user --jq .login`,
		`gh pr list --json number,title --jq '.[] | select(.title|test("x";"i")) | .number'`,
		`gh pr view 1 --json reviews --jq '[.reviews[] | {a:.author.login}] | last'`,
		`gh api x --jq '.[] | "\(.number) \(.title)"'`,
		`gh api x --jq 'select(.name=="env") | .env'`,
		`gh api x --template '{{.a}}'`,
		`gh pr view 1`,
		`f='.a'; gh api x --jq "$f"`,
	} {
		ambientAllowed(t, wt, cmd)
	}
}

func TestHookGhFilterSilentUnset(t *testing.T) {
	_, wt := gitFixture(t)
	t.Setenv("CLAUDE_PROJECT_DIR", wt)
	ambientEnv(t, "", "")
	if v, err := ambientRun(t, wt, "Bash", ambientBash(`gh api x --jq env.GH_X`)); err != nil || v.Block {
		t.Fatalf("no desk session: want silence, got block=%v err=%v", v.Block, err)
	}
}
