package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stopSnippetSkills are the desk skills whose loop runs an iteration-boundary stop check of
// its own (the skill layer). intake-desk and the-desk have none and rely on the Guard alone.
var stopSnippetSkills = []string{"pr-review-desk", "verify-desk", "worker-desk"}

// extractStopSnippet returns the first ```bash fence under the skill's
// "### Stop-flag check" heading — the exact text the loop runs at each iteration boundary.
func extractStopSnippet(t *testing.T, skill string) string {
	t.Helper()
	path := filepath.Join("../../../../plugins/assay/skills", skill, "SKILL.md")
	skipIfFixtureAbsent(t, path, "the desk skill bodies are not part of this published file set")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := string(raw)
	i := strings.Index(s, "### Stop-flag check")
	if i < 0 {
		t.Fatalf("%s: no \"### Stop-flag check\" heading — the skill layer is gone", path)
	}
	s = s[i:]
	j := strings.Index(s, "```bash\n")
	if j < 0 {
		t.Fatalf("%s: no ```bash fence under the stop-flag heading", path)
	}
	s = s[j+len("```bash\n"):]
	k := strings.Index(s, "```")
	if k < 0 {
		t.Fatalf("%s: unterminated stop-flag fence", path)
	}
	return s[:k]
}

// runStopSnippet runs snippet in a clean environment: HOME is a temp dir, PATH holds only
// the system directories (no desk tool, so no Guard can be reached), and the marker is
// ASSAY_CELL=cell with DESK_LOOP=loop. A trailing sentinel line shows whether the snippet
// fell through (loop continues) or exited (loop stops).
func runStopSnippet(t *testing.T, snippet, home, cell, loop string) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available: the skill snippet is bash")
	}
	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", snippet+"\necho LOOP-CONTINUES\n")
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "ASSAY_CELL=" + cell, "DESK_LOOP=" + loop}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("snippet failed: %v\n%s", err, out)
	}
	return string(out)
}

// regression: #2971
func TestSkillStopSnippetGuardBypassed(t *testing.T) {
	for _, skill := range stopSnippetSkills {
		t.Run(skill, func(t *testing.T) {
			snippet := extractStopSnippet(t, skill)
			home := t.TempDir()
			state := filepath.Join(home, ".config", "assay")
			if err := os.MkdirAll(state, 0o700); err != nil {
				t.Fatal(err)
			}

			// No flag: the loop continues (the snippet is not a blanket exit).
			if out := runStopSnippet(t, snippet, home, "a", skill); !strings.Contains(out, "LOOP-CONTINUES") {
				t.Fatalf("no flag present, yet the loop exited:\n%s", out)
			}

			// STOP.cell.a with the marker cell=a: the loop exits and names the flag.
			flag := filepath.Join(state, "STOP.cell.a")
			if err := os.WriteFile(flag, []byte("drain a\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			out := runStopSnippet(t, snippet, home, "a", skill)
			t.Logf("%s with STOP.cell.a: %q", skill, out)
			if strings.Contains(out, "LOOP-CONTINUES") || !strings.Contains(out, "STOP.cell.a active") {
				t.Fatalf("STOP.cell.a present, want the loop to exit naming it; got:\n%s", out)
			}

			// A session of another cell is not stopped by it.
			if out := runStopSnippet(t, snippet, home, "b", skill); !strings.Contains(out, "LOOP-CONTINUES") {
				t.Fatalf("STOP.cell.a stopped a session of cell b:\n%s", out)
			}
			if err := os.Remove(flag); err != nil {
				t.Fatal(err)
			}

			// STOP.cell.a.<loop> stops this loop of cell a.
			if err := os.WriteFile(filepath.Join(state, "STOP.cell.a."+skill), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			out = runStopSnippet(t, snippet, home, "a", skill)
			if strings.Contains(out, "LOOP-CONTINUES") || !strings.Contains(out, "STOP.cell.a."+skill+" active") {
				t.Fatalf("STOP.cell.a.%s present, want the loop to exit naming it; got:\n%s", skill, out)
			}

			// A marker that fails the cell-name rule builds no path: a flag planted where a
			// naive join would look is not read.
			if err := os.MkdirAll(filepath.Join(state, "STOP.cell.x"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(state, "STOP.cell.x", "y"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if out := runStopSnippet(t, snippet, home, "x/y", skill); !strings.Contains(out, "LOOP-CONTINUES") {
				t.Fatalf("invalid marker x/y built a flag path:\n%s", out)
			}
		})
	}
}
