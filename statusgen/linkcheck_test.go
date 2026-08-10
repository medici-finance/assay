package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkProblems(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "docs", "streams", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, sub, "real.md", "target")
	content := "See [ok](./real.md) and [ok2](real.md#anchor) and [bad](./missing.md).\n" +
		"Backtick good: `docs/streams/x/real.md`. Backtick bad: `docs/streams/x/gone.md`.\n" +
		"Skips: [url](https://example.com), [anchor](#section), `k8s/identity/<env>.yaml`, `../sibling/file.md`.\n"
	p := writeTemp(t, sub, "README.md", content)
	problems := linkProblems(root, []string{p})
	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2: %v", len(problems), problems)
	}
	for _, want := range []string{"missing.md", "gone.md"} {
		found := false
		for _, pr := range problems {
			if strings.Contains(pr, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem mentioning %q: %v", want, problems)
		}
	}
}

func TestLinkProblemsWithPlanned(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "docs", "streams", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// backticked paths followed by the planned-deliverable markers are skipped
	content := "Reference: `docs/streams/x/future.md` (planned).\n" +
		"Also fine: `docs/streams/x/coming.md` (new) and `docs/streams/x/later.md` (future path).\n" +
		"But this should fail: `docs/streams/x/also-missing.md`.\n" +
		"Marker not adjacent, still fails: `docs/streams/x/gap.md`  (planned).\n"
	p := writeTemp(t, sub, "README.md", content)
	problems := linkProblems(root, []string{p})
	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2 (also-missing + gap): %v", len(problems), problems)
	}
	for _, want := range []string{"also-missing.md", "gap.md"} {
		found := false
		for _, pr := range problems {
			if strings.Contains(pr, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem mentioning %q: %v", want, problems)
		}
	}
	// the failure message must teach both escapes
	if !strings.Contains(problems[0], "(planned)") || !strings.Contains(problems[0], "../<repo>/") {
		t.Errorf("message should teach the escapes, got: %v", problems[0])
	}
}

func TestDocFiles(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "docs", "streams", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, root, "CLAUDE.md", "x")
	writeTemp(t, sub, "README.md", "x")
	writeTemp(t, sub, "notes.txt", "x") // not .md → excluded
	files := docFiles(root)
	if len(files) != 2 {
		t.Errorf("got %d files, want 2 (CLAUDE.md + README.md): %v", len(files), files)
	}
}

// docFiles must walk ALL of docs/**, not just docs/streams/ — the outbound
// docs/articles|integrations|design dirs were previously invisible to the
// linkcheck (assay-toolkit#59).
func TestDocFilesCoversOutboundDirs(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{
		filepath.Join("docs", "articles"),
		filepath.Join("docs", "integrations"),
		filepath.Join("docs", "design"),
		filepath.Join("docs", "streams", "x"),
	} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTemp(t, root, "CLAUDE.md", "x")
	writeTemp(t, filepath.Join(root, "docs", "articles"), "a.md", "x")
	writeTemp(t, filepath.Join(root, "docs", "integrations"), "b.md", "x")
	writeTemp(t, filepath.Join(root, "docs", "design"), "c.md", "x")
	writeTemp(t, filepath.Join(root, "docs", "streams", "x"), "README.md", "x")
	files := docFiles(root)
	// CLAUDE.md + 4 markdown files under docs/**.
	if len(files) != 5 {
		t.Errorf("got %d files, want 5: %v", len(files), files)
	}
	for _, want := range []string{"articles", "integrations", "design", "streams"} {
		found := false
		for _, f := range files {
			if strings.Contains(f, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("docFiles did not cover a file under docs/%s: %v", want, files)
		}
	}
}

// Outbound narrative docs (outside docs/streams/**) get full markdown-link
// checking, but are exempt from the stricter backticked-path heuristic — which
// is a brief-authoring discipline gate and would false-positive on prose that
// mentions planned/cross-repo/absent paths (assay-toolkit#59).
func TestBacktickScopeOutboundDocs(t *testing.T) {
	root := t.TempDir()
	articles := filepath.Join(root, "docs", "articles")
	streams := filepath.Join(root, "docs", "streams", "s")
	for _, d := range []string{articles, streams} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// An outbound article: a dead MARKDOWN link IS flagged; a dead backticked
	// path is NOT (prose, exempt).
	article := "Dead md link: [x](./missing.md). Prose path: `docs/design/nonexistent.md`.\n"
	ap := writeTemp(t, articles, "piece.md", article)
	aProblems := linkProblems(root, []string{ap})
	if len(aProblems) != 1 || !strings.Contains(aProblems[0], "missing.md") {
		t.Fatalf("outbound doc: want 1 problem (dead md link only), got: %v", aProblems)
	}

	// A brief under docs/streams/**: the backticked-path heuristic still applies.
	brief := "Prose path: `docs/design/nonexistent.md`.\n"
	bp := writeTemp(t, streams, "brief.md", brief)
	bProblems := linkProblems(root, []string{bp})
	if len(bProblems) != 1 || !strings.Contains(bProblems[0], "nonexistent.md") {
		t.Fatalf("brief doc: want 1 problem (backticked path), got: %v", bProblems)
	}
}

// A leading `/` is a route on the published docs site, not a path in this
// checkout, so it is skipped — while every OTHER skip rule and every genuinely
// broken relative link of the same shape keeps being reported.
func TestSiteAbsoluteLinksSkipped(t *testing.T) {
	root := t.TempDir()
	articles := filepath.Join(root, "docs", "articles")
	if err := os.MkdirAll(articles, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "Site routes, skipped: [a](/contracts), [b](/llms-full.txt), " +
		"[c](/strategy-engine#templates), [d](/docs/brand-guide.md).\n" +
		// The paired negative: the same targets WITHOUT the leading slash are
		// ordinary relative links and must still be reported.
		"Relative, still checked: [e](contracts.md), [f](./llms-full.txt), [g](docs/brand-guide.md).\n"
	p := writeTemp(t, articles, "piece.md", content)
	problems := linkProblems(root, []string{p})
	if len(problems) != 3 {
		t.Fatalf("got %d problems, want 3 (the relative ones only): %v", len(problems), problems)
	}
	for _, want := range []string{"contracts.md", "./llms-full.txt", "docs/brand-guide.md"} {
		found := false
		for _, pr := range problems {
			if strings.Contains(pr, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem mentioning %q: %v", want, problems)
		}
	}
}

// skippable's contract, pinned directly: http / # / site-absolute / placeholder
// shapes are skipped; ordinary relative targets (including `../`, which
// linkProblems resolves and root-checks itself) are NOT.
func TestSkippable(t *testing.T) {
	for _, target := range []string{
		"", "http://x/y", "https://x/y", "#section",
		"/contracts", "/llms-full.txt", "/docs/brand-guide.md",
		"k8s/identity/<env>.yaml", "docs/*.md", "some path.md",
	} {
		if !skippable(target) {
			t.Errorf("skippable(%q) = false, want true", target)
		}
	}
	for _, target := range []string{
		"./real.md", "real.md", "docs/design/real.md",
		"../design/real.md", "../../other-repo/x.md",
	} {
		if skippable(target) {
			t.Errorf("skippable(%q) = true, want false", target)
		}
	}
}

// Markdown link syntax inside a fenced code block or an inline code span is
// quoted text, not a link — a format example (`[Title](URL)`) or a pasted test
// fixture has no resolvable target. Real links on the same lines, and real
// broken links outside code, must still be reported.
func TestLinksInsideCodeAreNotLinks(t *testing.T) {
	root := t.TempDir()
	articles := filepath.Join(root, "docs", "articles")
	if err := os.MkdirAll(articles, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, articles, "real.md", "target")
	content := "Format example, not a link: `- [Title](URL): Description.`\n" +
		"Inline span plus a real dead link on one line: `[x](path)` and [bad](./gone.md).\n" +
		"A live link next to a span survives: `code` [ok](./real.md) `more`.\n" +
		"```go\n" +
		"content := \"See [ok](./real.md) and [bad](./missing.md).\\n\"\n" +
		"```\n" +
		"~~~\n" +
		"[tilde-fenced](./also-missing.md)\n" +
		"~~~\n" +
		"After the fences, still checked: [bad2](./absent.md).\n"
	p := writeTemp(t, articles, "piece.md", content)
	problems := linkProblems(root, []string{p})
	if len(problems) != 2 {
		t.Fatalf("got %d problems, want 2 (./gone.md + ./absent.md): %v", len(problems), problems)
	}
	for _, want := range []string{"./gone.md", "./absent.md"} {
		found := false
		for _, pr := range problems {
			if strings.Contains(pr, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem mentioning %q: %v", want, problems)
		}
	}
	for _, notWant := range []string{"URL", "missing.md", "also-missing.md"} {
		for _, pr := range problems {
			if strings.Contains(pr, notWant) {
				t.Errorf("code-quoted %q must not be reported: %v", notWant, problems)
			}
		}
	}
}

// stripCode must not swallow a fence's surroundings, and an unterminated fence
// ends at EOF rather than leaking back into checked text.
func TestStripCode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string // substrings that must survive
		gone []string
	}{
		{
			name: "closing fence longer than opening",
			in:   "a [keep](./k.md)\n```\ndrop [x](./d.md)\n````\nb [keep2](./k2.md)\n",
			want: []string{"./k.md", "./k2.md"},
			gone: []string{"./d.md"},
		},
		{
			name: "indented fence",
			in:   "  ```\n  drop [x](./d.md)\n  ```\nkeep [k](./k.md)\n",
			want: []string{"./k.md"},
			gone: []string{"./d.md"},
		},
		{
			name: "info string on opener only",
			in:   "```bash\ndrop [x](./d.md)\n```\nkeep [k](./k.md)\n",
			want: []string{"./k.md"},
			gone: []string{"./d.md"},
		},
		{
			name: "unterminated fence runs to EOF",
			in:   "keep [k](./k.md)\n```\ndrop [x](./d.md)\n",
			want: []string{"./k.md"},
			gone: []string{"./d.md"},
		},
		{
			name: "unpaired backtick does not swallow the line",
			in:   "an ` unpaired tick and [k](./k.md)\n",
			want: []string{"./k.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripCode(tc.in)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("stripCode removed %q: %q", w, got)
				}
			}
			for _, g := range tc.gone {
				if strings.Contains(got, g) {
					t.Errorf("stripCode kept %q: %q", g, got)
				}
			}
		})
	}
}

// The backticked-path heuristic reads RAW content, so stripCode must not have
// disarmed it: a dead backticked path in a brief is still reported, and the
// `(planned)` / `../<repo>/` escapes still work.
func TestBacktickCheckUnaffectedByStripCode(t *testing.T) {
	root := t.TempDir()
	streams := filepath.Join(root, "docs", "streams", "s")
	if err := os.MkdirAll(streams, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "Dead backticked path: `docs/design/nope.md`.\n" +
		"Escapes still hold: `docs/design/soon.md` (planned), `../sibling/x.md`.\n"
	p := writeTemp(t, streams, "brief.md", content)
	problems := linkProblems(root, []string{p})
	if len(problems) != 1 || !strings.Contains(problems[0], "nope.md") {
		t.Fatalf("want 1 backticked-path problem (nope.md), got: %v", problems)
	}
}

// `../`-relative markdown links that resolve INSIDE the repo must be verified
// (they were previously skipped wholesale), while a link that escapes the repo
// root — the sibling-repo `../<repo>/…` convention — stays unchecked.
func TestRelativeParentLinks(t *testing.T) {
	root := t.TempDir()
	articles := filepath.Join(root, "docs", "articles")
	design := filepath.Join(root, "docs", "design")
	for _, d := range []string{articles, design} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTemp(t, design, "real.md", "target")
	content := "In-repo up-link that exists: [ok](../design/real.md).\n" +
		"In-repo up-link that is dead: [bad](../design/missing.md).\n" +
		"Sibling-repo link escapes root, unchecked: [sib](../../../other-repo/x.md).\n"
	p := writeTemp(t, articles, "piece.md", content)
	problems := linkProblems(root, []string{p})
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1 (only ../design/missing.md): %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "missing.md") {
		t.Errorf("problem should name the dead in-repo target: %v", problems[0])
	}
}
