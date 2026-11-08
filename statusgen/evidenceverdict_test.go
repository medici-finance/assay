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
	if code, out, _ := evRun(t, filepath.Join(t.TempDir(), "missing"), "--verdict", "BLOCKED"); code != evidenceverdictExitCouldNotCheck {
		t.Fatalf("a missing board: exit = %d, want %d (could-not-check, never rounded to either answer); out=%q",
			code, evidenceverdictExitCouldNotCheck, out)
	}
}

// regression: #5759
//
// TestEvVerdictUnresolved: every input whose status or verdict this
// subcommand cannot resolve is could-not-check (exit 2) — never landable, never
// refused. One row per way the answer can go unresolved, so a new rounding at any
// of these sites turns this table red.
func TestEvVerdictUnresolved(t *testing.T) {
	type row struct {
		name  string
		setup func(t *testing.T) (root string, args []string)
		want  string
	}
	plant := func(t *testing.T, rel, text string) string {
		t.Helper()
		root, _ := evAfter(t, "verified", "PASS")
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	rows := []row{
		{"board unreadable", func(t *testing.T) (string, []string) {
			return filepath.Join(t.TempDir(), "missing"), []string{"--brief-path", evBriefRel, "--verdict", "BLOCKED"}
		}, "cannot read the board"},
		{"brief absent at root", func(t *testing.T) (string, []string) {
			root, _ := evAfter(t, "verified", "PASS")
			return root, []string{"--brief-path", "docs/streams/vc/brief-02-absent.md", "--verdict", "BLOCKED"}
		}, "no brief file"},
		{"brief id does not parse", func(t *testing.T) (string, []string) {
			root := plant(t, "docs/streams/vc/brief-x.md", "# no id\n")
			return root, []string{"--brief-path", "docs/streams/vc/brief-x.md", "--verdict", "BLOCKED"}
		}, "id does not parse"},
		{"no board row", func(t *testing.T) (string, []string) {
			root := plant(t, "docs/streams/vc/brief-03-orphan.md", "# orphan\n")
			return root, []string{"--brief-path", "docs/streams/vc/brief-03-orphan.md", "--verdict", "BLOCKED"}
		}, "no board row"},
		{"--after unreadable", func(t *testing.T) (string, []string) {
			root, _ := evAfter(t, "verified", "PASS")
			return root, []string{"--brief-path", evBriefRel, "--after", filepath.Join(t.TempDir(), "gone.md")}
		}, "could-not-check"},
		{"--before unreadable", func(t *testing.T) (string, []string) {
			root, after := evAfter(t, "verified", "BLOCKED")
			return root, []string{"--brief-path", evBriefRel, "--after", after, "--before", filepath.Join(t.TempDir(), "gone.md")}
		}, "could-not-check"},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			root, args := r.setup(t)
			code, out, errOut := captureEvVerdict(t, append([]string{"--root", root}, args...))
			if code != evidenceverdictExitCouldNotCheck {
				t.Fatalf("exit = %d, want %d (could-not-check); out=%q err=%q", code, evidenceverdictExitCouldNotCheck, out, errOut)
			}
			if strings.Contains(out, "landable") {
				t.Fatalf("an unresolved answer printed a landing verdict: %q", out)
			}
			if !strings.Contains(errOut, r.want) {
				t.Fatalf("stderr %q does not name %q", errOut, r.want)
			}
		})
	}
}

// regression: #5759
//
// TestEvVerdictWhitelist: the rule is a whitelist — on a verified brief, every
// verdict word other than PASS and FAIL is refused, PARTIAL and HELD included.
func TestEvVerdictWhitelist(t *testing.T) {
	root, _ := evAfter(t, "verified", "PASS")
	for _, w := range []string{"BLOCKED", "NEEDS-CONTEXT", "PARTIAL", "HELD"} {
		code, out, _ := evRun(t, root, "--verdict", w)
		if code != evidenceverdictExitNotLandable {
			t.Fatalf("--verdict %s: exit = %d, want refused; out=%q", w, code, out)
		}
		if !strings.Contains(out, "other than PASS or FAIL") {
			t.Fatalf("--verdict %s: refusal %q does not state the whitelist rule", w, out)
		}
	}
}

func TestEvVerdictUsage(t *testing.T) {
	if code, _, _ := captureEvVerdict(t, []string{"--brief-path", evBriefRel}); code != evidenceverdictExitCouldNotCheck {
		t.Fatalf("no --after/--verdict: exit = %d, want %d", code, evidenceverdictExitCouldNotCheck)
	}
}
