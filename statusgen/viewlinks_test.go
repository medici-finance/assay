package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebaseLinkTarget(t *testing.T) {
	cases := []struct {
		name   string
		target string
		subdir string
		want   string
	}{
		{"bare sibling filename", "2026-07-08-x.md", "findings", "findings/2026-07-08-x.md"},
		{"dot-slash sibling", "./x.md", "findings", "findings/x.md"},
		{"parent into another stream", "../methodology/y.md", "findings", "methodology/y.md"},
		{"intake subdir", "z.md", "intake", "intake/z.md"},
		{"preserves anchor", "x.md#frag", "findings", "findings/x.md#frag"},
		{"http untouched", "https://example.com/a", "findings", "https://example.com/a"},
		{"protocol-relative untouched", "ftp://h/a", "findings", "ftp://h/a"},
		{"bare anchor untouched", "#section", "findings", "#section"},
		{"root-absolute untouched", "/etc/passwd", "findings", "/etc/passwd"},
		{"mailto untouched", "mailto:a@b.c", "findings", "mailto:a@b.c"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rebaseLinkTarget(c.target, c.subdir); got != c.want {
				t.Fatalf("rebaseLinkTarget(%q, %q) = %q, want %q", c.target, c.subdir, got, c.want)
			}
		})
	}
}

func TestRebaseEntryBodyLinks(t *testing.T) {
	body := "See [F-05](2026-07-08-x.md) and [brief](../methodology/09.md).\n" +
		"```\n" +
		"cli [not-a-link](2026-07-08-x.md)\n" +
		"```\n" +
		"Also [ext](https://example.com) stays.\n"
	got := rebaseEntryBodyLinks(body, "findings")

	if !strings.Contains(got, "[F-05](findings/2026-07-08-x.md)") {
		t.Errorf("bare sibling link not re-based: %q", got)
	}
	if !strings.Contains(got, "[brief](methodology/09.md)") {
		t.Errorf("parent link not re-based: %q", got)
	}
	// Content inside a code fence must NOT be rewritten.
	if !strings.Contains(got, "cli [not-a-link](2026-07-08-x.md)") {
		t.Errorf("fenced content was rewritten: %q", got)
	}
	if !strings.Contains(got, "[ext](https://example.com)") {
		t.Errorf("external link was mangled: %q", got)
	}
}

// TestGeneratedFindingsViewLinksResolve is the #827 regression guard: a finding
// entry whose body links a sibling entry by bare filename must produce a
// FINDINGS.md view whose links resolve (relative to docs/streams/) and pass the
// register-reference lint — i.e. no PROBLEMs when the generated view is linted.
func TestGeneratedFindingsViewLinksResolve(t *testing.T) {
	root := t.TempDir()
	fDir := filepath.Join(root, "docs", "streams", "findings")
	if err := os.MkdirAll(fDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// The target entry, referenced by a sibling.
	target := "2026-07-08-target-entry.md"
	targetContent := "---\nid: F-05\ndate: \"2026-07-08\"\ntitle: Target entry\nresolved: true\n---\n\nA target finding.\n"
	if err := os.WriteFile(filepath.Join(fDir, target), []byte(targetContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// The referencing entry — bare sibling link, correct from findings/ but broken
	// once hoisted into docs/streams/FINDINGS.md unless re-based.
	refContent := "---\nid: F-08\ndate: \"2026-07-09\"\ntitle: Referencing entry\nresolved: true\n---\n\n" +
		"See [F-05](2026-07-08-target-entry.md) for the existence proof.\n"
	if err := os.WriteFile(filepath.Join(fDir, "2026-07-09-referencing-entry.md"), []byte(refContent), 0o644); err != nil {
		t.Fatal(err)
	}

	view, err := generateFindingsView(root)
	if err != nil {
		t.Fatalf("generateFindingsView: %v", err)
	}

	// The generated view must carry the re-based link, not the bare filename.
	if !strings.Contains(view, "[F-05](findings/2026-07-08-target-entry.md)") {
		t.Fatalf("view does not contain re-based link:\n%s", view)
	}
	if strings.Contains(view, "[F-05](2026-07-08-target-entry.md)") {
		t.Fatalf("view still contains the un-re-based bare link:\n%s", view)
	}

	// Write the view where the checkers expect it, then lint: no PROBLEMs.
	viewPath := filepath.Join(root, "docs", "streams", "FINDINGS.md")
	if err := os.WriteFile(viewPath, []byte(view), 0o644); err != nil {
		t.Fatal(err)
	}
	if probs := linkProblems(root, []string{viewPath}); len(probs) > 0 {
		t.Errorf("linkProblems on generated view: %v", probs)
	}
	rp, _ := registerRefProblems(root, []string{viewPath})
	if len(rp) > 0 {
		t.Errorf("registerRefProblems on generated view: %v", rp)
	}
}
