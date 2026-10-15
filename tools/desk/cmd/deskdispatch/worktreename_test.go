package main

// worktreename_test.go — coverage for the session-scoped worktree name when the item-derived
// base plus the session suffix is longer than the worktree-name bound.
//
// THE DEFECT. The worker and verifier paths built `<base>-<session>` and, when that ran past
// the 64-character grammar, silently fell back to the bare base. Two sessions dispatching the
// same long-keyed item then resolved to one worktree name, so session isolation was lost and
// nothing reported it. The fix shortens the BASE (keeping a digest tag of it) and keeps the
// session suffix whole, or refuses when no session-unique name fits.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Long enough that base + "-" + session overflows the bound on both the worker base
// ("example-stream-with-a-rather-long-name-06", 41) and the verifier base ("verify-" + that).
const (
	longItem  = "example-stream-with-a-rather-long-name/06"
	sessAlpha = "session-alpha-0123456789abcdef"
	sessBravo = "session-bravo-0123456789abcdef"
)

// dispatchedWtName runs one dispatch of longItem under session sess and returns the worktree
// name the verb passed to `deskwt add`.
func dispatchedWtName(t *testing.T, kit, sess string) string {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	t.Setenv("DESK_SESSION", sess)
	s.replies = happyReplies("/private/tmp/long-item-home")
	args := []string{longItem, "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")}
	if kit == "verifier" {
		args = append(args, "--kit", "verifier", "--model", "gpt-6-astra", "--brief", "spec.md")
	}
	if rc := run(args); rc != deskkit.ExitOK {
		t.Fatalf("%s dispatch under %s: rc = %d, want 0", kit, sess, rc)
	}
	for _, c := range s.calls {
		if len(c) > 2 && filepath.Base(c[0]) == "deskwt" && c[1] == "add" {
			return c[2]
		}
	}
	t.Fatalf("%s dispatch ran no `deskwt add`; deskwt calls: %v", kit, s.deskwtCalls())
	return ""
}

// regression: #2829
func TestLongKeyKeepsSessionSuffix(t *testing.T) {
	for _, kit := range []string{"worker", "verifier"} {
		t.Run(kit, func(t *testing.T) {
			a := dispatchedWtName(t, kit, sessAlpha)
			b := dispatchedWtName(t, kit, sessBravo)
			if a == b {
				t.Fatalf("two sessions got the same %s worktree name %q: session isolation is lost", kit, a)
			}
			for sess, name := range map[string]string{sessAlpha: a, sessBravo: b} {
				if !worktreeNameRe.MatchString(name) {
					t.Errorf("%s worktree name %q (%d chars) is outside the worktree-name grammar", kit, name, len(name))
				}
				if !strings.HasSuffix(name, "-"+sess) {
					t.Errorf("%s worktree name %q dropped the session suffix %q", kit, name, sess)
				}
			}
			if kit == "verifier" && !strings.HasPrefix(a, "verify-") {
				t.Errorf("verifier worktree name %q lost its own verify- prefix", a)
			}
		})
	}
}

// regression: #2829
func TestShortenedNamesKeepItemsApart(t *testing.T) {
	// Two items whose bases share every character the cut keeps: the digest tag of the full
	// base is what keeps them apart under one session.
	shared := strings.Repeat("x", 50)
	a, errA := sessionScopedName(shared+"-01", sessAlpha)
	b, errB := sessionScopedName(shared+"-02", sessAlpha)
	if errA != nil || errB != nil {
		t.Fatalf("unexpected refusal: %v / %v", errA, errB)
	}
	if a == b {
		t.Fatalf("two items share the shortened name %q under one session", a)
	}
	for _, n := range []string{a, b} {
		if len(n) > worktreeNameMax || !worktreeNameRe.MatchString(n) {
			t.Errorf("shortened name %q (%d chars) is outside the grammar", n, len(n))
		}
	}
}

// regression: #2829
func TestScopedNameUnchangedWhenItFits(t *testing.T) {
	if got, _ := sessionScopedName("example-stream-06", "sess-xyz"); got != "example-stream-06-sess-xyz" {
		t.Errorf("a name that fits must be base-session unchanged; got %q", got)
	}
	if got, _ := sessionScopedName("example-stream-06", ""); got != "example-stream-06" {
		t.Errorf("no session must keep the bare base; got %q", got)
	}
	// Exactly at the bound: base + "-" + sess is 64 characters and must not be shortened.
	base := strings.Repeat("b", worktreeNameMax-1-len(sessAlpha))
	if got, _ := sessionScopedName(base, sessAlpha); got != base+"-"+sessAlpha {
		t.Errorf("a 64-character name was shortened: %q", got)
	}
}

// regression: #2829
func TestOverlongSessionRefuses(t *testing.T) {
	sess := strings.Repeat("s", worktreeNameMax-baseTagLen-2)
	name, err := sessionScopedName(strings.Repeat("b", 40), sess)
	if err == nil {
		t.Fatalf("a session with no room for the item must refuse, not return %q", name)
	}
	if !strings.Contains(err.Error(), "no session-unique name fits") {
		t.Errorf("the refusal must say no session-unique name fits; got %v", err)
	}
}

// regression: #2829
func TestOverlongSessionRefusesDispatch(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	t.Setenv("DESK_SESSION", strings.Repeat("s", worktreeNameMax-baseTagLen-2))
	rc, stderr := runCapturingStderr(t, []string{"example-stream/06", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "no session-unique name fits") {
		t.Fatalf("rc = %d (want %d), stderr:\n%s", rc, deskkit.ExitRefused, stderr)
	}
	for _, c := range s.calls {
		if len(c) > 1 && filepath.Base(c[0]) == "deskwt" && c[1] == "add" {
			t.Fatalf("a refused name must create no worktree; ran %v", c)
		}
		if strings.Contains(strings.Join(c, " "), "acquire") {
			t.Fatalf("the name refusal must come before the claim; ran %v", c)
		}
	}
}
