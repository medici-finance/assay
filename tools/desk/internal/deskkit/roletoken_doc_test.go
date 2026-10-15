package deskkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// deskShellDoc is the bundle reference that carries the `gh`-as-the-role recipe
// (§Forge CLI as the role, #3055), relative to this package directory.
var deskShellDoc = filepath.Join("..", "..", "..", "..", "plugins", "assay", "references", "desk-shell.md")

// recipeCaseArm matches one arm of the recipe's `case "${DESK_LOOP:-}"` mapping:
//
//	worker-desk|batch-fanout) role=worker ;;
var recipeCaseArm = regexp.MustCompile(`^\s*([a-z][a-z0-9|-]*)\)\s+role=([a-z-]*)\s*;;\s*$`)

// TestDeskShellRecipeMatchesLoopTokenRoles holds the shell recipe's loop-to-role
// mapping in step with loopTokenRoles. The recipe is copied into adopter shells,
// so a loop added or renamed here and not there sends that desk to the refusal
// branch, and a stale arm mints the wrong App's token. Every canonical loop and
// every retired alias (canonicalLoopNames) must map to the same App role the code
// uses, and the recipe must name no loop the code does not know.
func TestDeskShellRecipeMatchesLoopTokenRoles(t *testing.T) {
	raw, err := os.ReadFile(deskShellDoc)
	if err != nil {
		t.Skipf("could-not-check: %s is not in this checkout (%v)", deskShellDoc, err)
	}
	const begin, end = "<!-- BEGIN forge cli as the role -->", "<!-- END forge cli as the role -->"
	s := string(raw)
	i, j := strings.Index(s, begin), strings.Index(s, end)
	if i < 0 || j < i {
		t.Fatalf("desk-shell.md has no %q ... %q section; the recipe this test guards is gone or its markers moved", begin, end)
	}

	got := map[string]string{}
	inCase, sawCatchAll := false, false
	for _, ln := range strings.Split(s[i:j], "\n") {
		switch {
		case strings.Contains(ln, `case "${DESK_LOOP:-}" in`):
			inCase = true
			continue
		case inCase && strings.TrimSpace(ln) == "esac":
			inCase = false
			continue
		}
		if !inCase {
			continue
		}
		m := recipeCaseArm.FindStringSubmatch(ln)
		if m == nil {
			if strings.HasPrefix(strings.TrimSpace(ln), "*)") {
				sawCatchAll = true
				if !strings.Contains(ln, "role= ;;") {
					t.Errorf("catch-all arm must leave role empty (a refusal), got %q", strings.TrimSpace(ln))
				}
				continue
			}
			t.Errorf("unparsed line inside the recipe's case block: %q", strings.TrimSpace(ln))
			continue
		}
		for _, loop := range strings.Split(m[1], "|") {
			if prev, dup := got[loop]; dup {
				t.Errorf("loop %q mapped twice (%q and %q)", loop, prev, m[2])
			}
			got[loop] = m[2]
		}
	}
	if len(got) == 0 {
		t.Fatal("parsed zero case arms from the recipe; a check that read nothing proved nothing")
	}
	if !sawCatchAll {
		t.Error("recipe's case block has no `*)` catch-all; an unknown loop must refuse, never fall through")
	}

	want := map[string]string{}
	for loop, role := range loopTokenRoles {
		want[loop] = role
		for _, alias := range canonicalLoopNames[loop] {
			want[alias] = role
		}
	}

	var names []string
	for k := range want {
		names = append(names, k)
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, loop := range names {
		w, inCode := want[loop]
		g, inDoc := got[loop]
		switch {
		case !inDoc:
			t.Errorf("desk-shell.md recipe is missing loop %q (code maps it to App role %q)", loop, w)
		case !inCode:
			t.Errorf("desk-shell.md recipe maps loop %q, which loopTokenRoles does not know", loop)
		case g != w:
			t.Errorf("desk-shell.md recipe maps loop %q to %q; code maps it to %q", loop, g, w)
		}
	}
}

// TestDeskShellRecipeRefusesBlankToken runs the recipe's bash block against a
// stub `desktoken` and a stub `gh`. A custody file that holds no token (empty, a
// bare newline, only whitespace) must stop the recipe before `gh` runs: real `gh`
// treats an empty GH_TOKEN as unset and falls back to the host's stored login,
// which is a human's, and exits 0. A size check alone passes a newline-only file.
func TestDeskShellRecipeRefusesBlankToken(t *testing.T) {
	raw, err := os.ReadFile(deskShellDoc)
	if err != nil {
		t.Skipf("could-not-check: %s is not in this checkout (%v)", deskShellDoc, err)
	}
	const begin, end = "<!-- BEGIN forge cli as the role -->", "<!-- END forge cli as the role -->"
	s := string(raw)
	i, j := strings.Index(s, begin), strings.Index(s, end)
	if i < 0 || j < i {
		t.Fatalf("desk-shell.md has no %q ... %q section", begin, end)
	}
	sec := s[i:j]
	k := strings.Index(sec, "```bash\n")
	if k < 0 {
		t.Fatal("no ```bash block in the recipe section")
	}
	body := sec[k+len("```bash\n"):]
	body = body[:strings.Index(body, "```")]

	shells := []string{}
	for _, sh := range []string{"bash", "zsh"} {
		if _, err := exec.LookPath(sh); err == nil {
			shells = append(shells, sh)
		}
	}
	if len(shells) == 0 {
		t.Skip("could-not-check: neither bash nor zsh is on PATH")
	}

	cases := []struct {
		name, content string
		wantGH        bool
	}{
		{"empty", "", false},
		{"newline", "\n", false},
		{"whitespace", " \t\n \n", false},
		{"token", "ghs_stubtoken\n", true},
	}
	for _, sh := range shells {
		for _, c := range cases {
			t.Run(sh+"/"+c.name, func(t *testing.T) {
				dir := t.TempDir()
				bin := filepath.Join(dir, "bin")
				if err := os.Mkdir(bin, 0o755); err != nil {
					t.Fatal(err)
				}
				tok := filepath.Join(dir, "token")
				ghLog := filepath.Join(dir, "gh.log")
				if err := os.WriteFile(tok, []byte(c.content), 0o600); err != nil {
					t.Fatal(err)
				}
				stubs := map[string]string{
					"desktoken": "#!/bin/sh\nprintf '%s\\n' '" + tok + "'\n",
					"gh":        "#!/bin/sh\nprintf 'GH_TOKEN=[%s]\\n' \"$GH_TOKEN\" >> '" + ghLog + "'\n",
				}
				for name, src := range stubs {
					if err := os.WriteFile(filepath.Join(bin, name), []byte(src), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"-c", body}
				if sh == "bash" {
					args = append([]string{"--noprofile", "--norc"}, args...)
				} else {
					args = append([]string{"-f"}, args...)
				}
				cmd := exec.Command(sh, args...)
				cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "DESK_LOOP=worker-desk", "HOME=" + dir}
				out, runErr := cmd.CombinedOutput()
				logged, _ := os.ReadFile(ghLog)
				if !c.wantGH {
					if len(logged) != 0 {
						t.Fatalf("gh ran with a blank custody file (%q): %s", c.content, logged)
					}
					if runErr == nil {
						t.Errorf("recipe exited 0 on a blank custody file; output: %s", out)
					}
					if !strings.Contains(string(out), "desk-auth: unauthenticated") {
						t.Errorf("refusal does not name desk-auth: unauthenticated; output: %s", out)
					}
					return
				}
				if runErr != nil || string(logged) != "GH_TOKEN=[ghs_stubtoken]\n" {
					t.Fatalf("gh did not run with the token: err=%v log=%q output=%s", runErr, logged, out)
				}
			})
		}
	}
}
