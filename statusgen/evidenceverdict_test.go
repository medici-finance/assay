package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const evBriefRel = "docs/streams/vc/brief-01-closure.md"

func captureEvVerdict(t *testing.T, args []string) (code int, out, errOut string) {
	t.Helper()
	dir := t.TempDir()
	of, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer of.Close()
	ef, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer ef.Close()
	code = runEvidenceverdict(args, of, ef)
	ob, _ := os.ReadFile(filepath.Join(dir, "out"))
	eb, _ := os.ReadFile(filepath.Join(dir, "err"))
	return code, string(ob), string(eb)
}

// evAfter is a fixture whose brief Evidence ends with a run recording verdict
// word; the returned path is that brief file (the "after" text).
func evAfter(t *testing.T, status, word string) (root, after string) {
	t.Helper()
	ev := "2026-08-12 verifier run.\n\nVERIFY: PASS\n\n2026-11-01 re-run.\n\nVERIFY: " + word + "\n"
	root = vcFixture(t, status, stamped, ev)
	return root, filepath.Join(root, filepath.FromSlash(evBriefRel))
}

func evRun(t *testing.T, root string, extra ...string) (int, string, string) {
	t.Helper()
	return captureEvVerdict(t, append([]string{"--root", root, "--brief-path", evBriefRel}, extra...))
}

// regression: #5759
func TestEvVerdictVerifiedBlocked(t *testing.T) {
	root, after := evAfter(t, "verified", "BLOCKED")
	code, out, errOut := evRun(t, root, "--after", after)
	if code != evidenceverdictExitNotLandable {
		t.Fatalf("exit = %d, want %d; out=%q err=%q", code, evidenceverdictExitNotLandable, out, errOut)
	}
	for _, want := range []string{"vc/01", `"verified"`, "VERIFY: BLOCKED", "outcome record only"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout %q does not name %q", out, want)
		}
	}
}

// regression: #5759
func TestEvVerdictDoneNeedsContext(t *testing.T) {
	root, after := evAfter(t, "done", "NEEDS-CONTEXT")
	if code, out, _ := evRun(t, root, "--after", after); code != evidenceverdictExitNotLandable {
		t.Fatalf("exit = %d, want refused; out=%q", code, out)
	}
}

// regression: #5759
func TestEvVerdictVerifiedFail(t *testing.T) {
	root, after := evAfter(t, "verified", "FAIL")
	if code, out, _ := evRun(t, root, "--after", after); code != evidenceverdictExitLandable {
		t.Fatalf("exit = %d, want landable; out=%q", code, out)
	}
}

// regression: #5759
func TestEvVerdictVerifiedPass(t *testing.T) {
	root, after := evAfter(t, "verified", "PASS")
	if code, out, _ := evRun(t, root, "--after", after); code != evidenceverdictExitLandable {
		t.Fatalf("exit = %d, want landable; out=%q", code, out)
	}
}

// regression: #5759
func TestEvVerdictImplBlocked(t *testing.T) {
	root, after := evAfter(t, "implemented", "BLOCKED")
	if code, out, _ := evRun(t, root, "--after", after); code != evidenceverdictExitLandable {
		t.Fatalf("exit = %d, want landable; out=%q", code, out)
	}
}

// An unchanged latest verdict (same word, same count) records nothing new:
// an edit elsewhere in a brief whose history already ends BLOCKED still lands.
func TestEvVerdictUnchanged(t *testing.T) {
	root, after := evAfter(t, "verified", "BLOCKED")
	before := filepath.Join(t.TempDir(), "before.md")
	raw, err := os.ReadFile(after)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(before, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := evRun(t, root, "--after", after, "--before", before); code != evidenceverdictExitLandable {
		t.Fatalf("exit = %d, want landable; out=%q", code, out)
	}
}

// A struck re-run is not the latest verdict, exactly as flipLatestPass reads it.
func TestEvVerdictStruckIgnored(t *testing.T) {
	root, after := evAfter(t, "verified", "PASS")
	raw, _ := os.ReadFile(after)
	edited := strings.Replace(string(raw), "## Review", "~~2026-11-02 VERIFY: BLOCKED~~\n\n## Review", 1)
	if err := os.WriteFile(after, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := evRun(t, root, "--after", after); code != evidenceverdictExitLandable {
		t.Fatalf("exit = %d, want landable (struck BLOCKED is not latest); out=%q", code, out)
	}
}

// regression: #5759
func TestEvVerdictWordFlag(t *testing.T) {
	root, _ := evAfter(t, "verified", "PASS")
	if code, out, _ := evRun(t, root, "--verdict", "blocked"); code != evidenceverdictExitNotLandable {
		t.Fatalf("--verdict blocked: exit = %d, want refused; out=%q", code, out)
	}
	if code, out, _ := evRun(t, root, "--verdict", "FAIL"); code != evidenceverdictExitLandable {
		t.Fatalf("--verdict FAIL: exit = %d, want landable; out=%q", code, out)
	}
}

func TestEvVerdictNoBoard(t *testing.T) {
	if code, _, _ := evRun(t, filepath.Join(t.TempDir(), "missing"), "--verdict", "BLOCKED"); code == evidenceverdictExitNotLandable {
		t.Fatalf("a missing board must never read as refused")
	}
}

func TestEvVerdictUsage(t *testing.T) {
	if code, _, _ := captureEvVerdict(t, []string{"--brief-path", evBriefRel}); code != evidenceverdictExitCouldNotCheck {
		t.Fatalf("no --after/--verdict: exit = %d, want %d", code, evidenceverdictExitCouldNotCheck)
	}
}
