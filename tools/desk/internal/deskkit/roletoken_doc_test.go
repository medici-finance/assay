package deskkit

import (
	"os"
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
