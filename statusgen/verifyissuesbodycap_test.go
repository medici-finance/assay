package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// vgcForgeLimit is the forge's issue-body limit (65,536 characters). The test
// counts with its own oracle rather than the emitter's counter, so a counter
// that under-counts cannot pass its own check.
const vgcForgeLimit = 65536

// vgcUnits counts s the most expensive way the forge could: UTF-16 code units
// (an astral character is two), plus one more per LF in case the body is
// stored with CRLF line endings.
func vgcUnits(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
		if r == '\n' {
			n++
		}
	}
	return n
}

// loadVGCRoot copies the body-cap fixtures (testdata/verifygatecap) into a temp
// root, replaces each `<!-- oversize: … -->` line with enough table rows to
// push the rendered card far past the forge limit, and commits the tree so the
// emitter can name the commit its brief link points at.
func loadVGCRoot(t *testing.T) (root, sha string) {
	t.Helper()
	root = t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/verifygatecap")); err != nil {
		t.Fatal(err)
	}
	var rows strings.Builder
	for i := 1; i <= 1500; i++ {
		// Multi-byte and astral characters, so a byte or rune count disagrees
		// with the forge's character count.
		fmt.Fprintf(&rows, "| %d | `go test ./pkg/m%d/...` | 0 | ok — résumé 😀 row %d | 2026-10-21 | example-verifier |\n", i, i, i)
	}
	for _, name := range []string{"brief-01-verified-oversize.md", "brief-02-implemented-oversize.md"} {
		p := filepath.Join(root, "docs", "streams", "vgc", name)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "<!-- oversize:") {
				out = append(out, strings.TrimSuffix(rows.String(), "\n"))
				continue
			}
			out = append(out, line)
		}
		if err := os.WriteFile(p, []byte(strings.Join(out, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		base := []string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}
		out, err := exec.Command("git", append(base, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "fixture")
	return root, git("rev-parse", "HEAD")
}

// TestVerifyIssueBodyCapped pins the body cap on BOTH verify-gate renderers
// (the verified→done card and the implemented-with-pass card): a brief whose
// Verify/Evidence sections overrun the forge's 65,536-character body limit
// still yields a body the forge accepts, with the idempotency marker and the
// header intact, the cut stated where it happens, the brief linked at the
// commit, and the closing checklist kept.
//
// regression: #4091
func TestVerifyIssueBodyCapped(t *testing.T) {
	root, sha := loadVGCRoot(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	issues := verifyIssues(root, streams, map[string]bool{})
	byBrief := map[string]verifyIssue{}
	for _, iss := range issues {
		byBrief[iss.Brief] = iss
	}

	for _, tc := range []struct {
		brief, file, heading string
		mustKeep             []string
	}{
		{"vgc/01", "brief-01-verified-oversize.md", "## Human sign-off required — vgc/01", nil},
		{"vgc/02", "brief-02-implemented-oversize.md",
			"## Human sign-off required (verified-by-model at implemented) — vgc/02",
			[]string{"### Trade-offs", "prior state needs no remediation", "### Prior-state remediation",
				"- [ ] Trade-offs understood and accepted"}},
	} {
		t.Run(tc.brief, func(t *testing.T) {
			iss, ok := byBrief[tc.brief]
			if !ok {
				t.Fatalf("%s not emitted (got %d issues) — check the fixture", tc.brief, len(issues))
			}
			body := iss.Body
			if n := vgcUnits(body); n >= vgcForgeLimit {
				t.Fatalf("%s body is %d characters (UTF-16, LF counted as CRLF), not under the forge's %d limit (%d runes, %d bytes)",
					tc.brief, n, vgcForgeLimit, utf8.RuneCountInString(body), len(body))
			}
			if !utf8.ValidString(body) {
				t.Errorf("%s body is not valid UTF-8 — a cut split a character", tc.brief)
			}
			marker := "<!-- verify-gate: " + tc.brief + " -->"
			if !strings.HasPrefix(body, marker+"\n\n") {
				t.Errorf("%s body must open with the marker line %q; got %q", tc.brief, marker, body[:min(len(body), 80)])
			}
			if got := verifyMarkerRe.FindAllString(body, -1); len(got) != 1 || got[0] != marker {
				t.Errorf("%s body markers = %q, want exactly [%q] (dedupe reads it)", tc.brief, got, marker)
			}
			// Header intact and in order, all before the first cut.
			cut := strings.Index(body, "**Cut here:**")
			if cut < 0 {
				t.Fatalf("%s body was over the limit but states no cut", tc.brief)
			}
			prev := 0
			for _, h := range []string{marker, tc.heading, "**Gate reason**", "**Implemented by:**",
				"**Brief:** [" + tc.brief + "](", "### Verify"} {
				at := strings.Index(body, h)
				if at < prev || at > cut {
					t.Errorf("%s header part %q at %d: want after %d and before the first cut at %d", tc.brief, h, at, prev, cut)
				}
				prev = at
			}
			link := "/blob/" + sha + "/docs/streams/vgc/" + tc.file + ")"
			if !strings.Contains(body[cut:], link) {
				t.Errorf("%s cut notice must link the brief at the commit (%q); got %q", tc.brief, link, body[cut:min(len(body), cut+400)])
			}
			for _, keep := range append([]string{"### Recorded results (Evidence)", "### Before you close"}, tc.mustKeep...) {
				if !strings.Contains(body, keep) {
					t.Errorf("%s body lost %q", tc.brief, keep)
				}
			}
			if !strings.HasSuffix(body, "Only a human closer is honored.\n") {
				t.Errorf("%s body lost its closing line; tail: %q", tc.brief, body[max(0, len(body)-120):])
			}
		})
	}
}

// TestGeneratedPromptsKept: the TRADE-OFFS and PRIOR-STATE prompts that
// statusgen itself writes (nothing lifted from the brief) are warnings the
// human must see. They sit outside the cuttable region, so even a card whose
// header leaves almost no room still carries them whole.
//
// regression: #4091
func TestGeneratedPromptsKept(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/verifygatecap")); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "docs", "streams", "vgc", "brief-02-implemented-oversize.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	// No Trade-offs section, so both prompts are generated; and a gate-why
	// that fills nearly the whole card.
	i, j := strings.Index(src, "## Trade-offs"), strings.Index(src, "## Evidence")
	if i < 0 || j < i {
		t.Fatal("fixture shape changed: Trade-offs must precede Evidence")
	}
	src = src[:i] + src[j:]
	src = strings.Replace(src, "sources:", "gate-why: "+strings.Repeat("reason ", 8200)+"\nsources:", 1)
	var rows strings.Builder
	for k := 1; k <= 600; k++ {
		fmt.Fprintf(&rows, "| %d | `go test ./pkg/m%d/...` | 0 | ok | 2026-10-21 | example-verifier |\n", k, k)
	}
	src = strings.Replace(src, "<!-- oversize: evidence -->", strings.TrimSuffix(rows.String(), "\n"), 1)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	var body string
	for _, iss := range verifyIssues(root, streams, map[string]bool{}) {
		if iss.Brief == "vgc/02" {
			body = iss.Body
		}
	}
	if body == "" {
		t.Fatal("vgc/02 not emitted")
	}
	if n := vgcUnits(body); n >= vgcForgeLimit {
		t.Fatalf("body is %d characters, not under the forge's %d limit", n, vgcForgeLimit)
	}
	for _, keep := range []string{"**TRADE-OFFS: not provided**", "### Prior-state remediation", "**PRIOR-STATE: unassessed**",
		"- [ ] Prior-state remediation assessed"} {
		if !strings.Contains(body, keep) {
			t.Errorf("body lost %q", keep)
		}
	}
	if !strings.HasPrefix(body, "<!-- verify-gate: vgc/02 -->\n") {
		t.Error("body must open with the marker")
	}
}
