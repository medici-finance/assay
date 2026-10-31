package main

import (
	"fmt"
	"os/exec"
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

// hookFilterMixes: the word's bytes in order, with up to two quoting bytes placed
// around and between them.
func hookFilterMixes(word string) []string {
	mixes, seen := []string{word}, map[string]bool{word: true}
	for from, n := 0, 0; n < 2; n++ {
		end := len(mixes)
		for _, m := range mixes[from:end] {
			for i := 0; i <= len(m); i++ {
				for _, q := range []string{`'`, `"`, `\`} {
					if w := m[:i] + q + m[i:]; !seen[w] {
						seen[w] = true
						mixes = append(mixes, w)
					}
				}
			}
		}
		from = end
	}
	return mixes
}

// regression: #5078
func TestHookGhFilterWordsLikeShell(t *testing.T) {
	// Through run(): each word is generated, and a real shell alone says what it reads as.
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("could not check: sh is not on PATH: %v", err)
	}
	_, wt := gitFixture(t)
	t.Setenv("CLAUDE_PROJECT_DIR", wt)
	ambientEnv(t, "worker-desk", "fixture-session")
	refused := func(cmd string) {
		t.Helper()
		v, err := ambientRun(t, wt, "Bash", ambientBash(cmd))
		if err != nil || !v.Block {
			t.Errorf("%q: want refused, got block=%v err=%v", cmd, v.Block, err)
		}
	}
	for _, row := range []struct {
		word, line string
		reads      bool
	}{
		{"--jq", "gh api x %s env.GH_X", true},
		{"-q", "gh api x %s env.GH_X", true},
		{"-q", "gh api x -H h %s env.GH_X", true},
		{"--jq", "gh api x %s .a", false},
		{"-q", "gh api x %s .a", false},
		{"env.GH_X", "gh api x --jq %s", true},
		{"env.GH_X", "gh api x --jq=%s", true},
		{".a", "gh api x --jq %s", false},
	} {
		seen := 0
		for _, w := range hookFilterMixes(row.word) {
			out, err := exec.Command(sh, "-c", "printf %s "+w+" 2>/dev/null").Output()
			if err != nil || string(out) != row.word {
				continue // an open quote, or a word that reads as something else
			}
			seen++
			cmd := fmt.Sprintf(row.line, w)
			if row.reads {
				refused(cmd)
			} else {
				ambientAllowed(t, wt, cmd)
			}
		}
		if seen < 2 {
			t.Errorf("%q over %q: %d words read as it, so this row proves nothing", row.line, row.word, seen)
		}
	}
	long, short := "--jq", "-q"
	for _, f := range []string{long, short} {
		for _, r := range []string{"2>o", "<o", "9>>o", "&>o", ">o 2>&1"} {
			refused("gh api x " + f + " " + r + " env.GH_X")
			ambientAllowed(t, wt, "gh api x "+f+" "+r+" .a")
		}
		refused("gh api x {" + f + ",.a}")
		refused("gh api x " + f + " {.a,.b}")
	}
	for _, n := range []string{"env", "now", "localtime", "strflocaltime"} {
		for _, form := range []string{`"${%s}"`, `"$(true %s)"`} {
			ambientAllowed(t, wt, "gh api x --jq "+fmt.Sprintf(form, n))
		}
	}
}
