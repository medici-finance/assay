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
	files, _ := docFiles(root)
	if len(files) != 2 {
		t.Errorf("got %d files, want 2 (CLAUDE.md + README.md): %v", len(files), files)
	}
}

// docFiles must walk ALL of docs/**, not just docs/streams/ — the outbound
// docs/articles|integrations|design dirs were previously invisible to the
// linkcheck.
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
	files, _ := docFiles(root)
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
// mentions planned/cross-repo/absent paths.
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

// A generator must not grade its own generated artifacts against a
// source-quality rule (#169). The intake boilerplate hardcodes a
// backticked `docs/v-next.md` reference; the link check runs over docs/**,
// including the generated docs/streams/INTAKE.md view. An adopter root with
// intake entries but no docs/v-next.md would therefore fail its own --lint on a
// path statusgen itself wrote. Prove: (1) the generated view is excluded from
// the link-check file set, and (2) genuine adopter-authored breakage in the
// per-entry SOURCE file is STILL caught.
func TestGeneratedRegisterViewsExcludedFromLinkCheck(t *testing.T) {
	root := t.TempDir()
	intakeDir := filepath.Join(root, "docs", "streams", "intake")
	if err := os.MkdirAll(intakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A valid intake source entry — enough for generateIntakeView to emit a view.
	entry := "---\nid: I-adopter-entry\ndate: 2026-08-15\ntitle: An adopter intake entry\ndisposition: watching\n---\n\n" +
		"A synthetic intake entry so the generated view has content.\n"
	if err := os.WriteFile(filepath.Join(intakeDir, "I-adopter-entry.md"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}

	// Render the real INTAKE view and write it where CI commits it. Deliberately
	// do NOT create docs/v-next.md — the adopter never authored that path.
	view, err := generateIntakeView(root)
	if err != nil {
		t.Fatalf("generateIntakeView: %v", err)
	}
	if !strings.Contains(view, "`docs/v-next.md`") {
		t.Fatalf("precondition failed: generated intake boilerplate no longer hardcodes `docs/v-next.md`; update this test:\n%s", view)
	}
	viewPath := filepath.Join(root, "docs", "streams", "INTAKE.md")
	if err := os.WriteFile(viewPath, []byte(view), 0o644); err != nil {
		t.Fatal(err)
	}

	// (1) docFiles must NOT include the generated view.
	files, walkProblems := docFiles(root)
	if len(walkProblems) != 0 {
		t.Fatalf("unexpected walk problems: %v", walkProblems)
	}
	for _, f := range files {
		if filepath.Base(f) == "INTAKE.md" {
			t.Fatalf("docFiles included the generated view %q — statusgen must not link-check its own output", f)
		}
	}
	// The SOURCE entry file stays in the set (its author-authored links matter).
	foundSource := false
	for _, f := range files {
		if strings.HasSuffix(filepath.ToSlash(f), "docs/streams/intake/I-adopter-entry.md") {
			foundSource = true
		}
	}
	if !foundSource {
		t.Fatalf("docFiles dropped the per-entry SOURCE file — adopter-authored links would go unchecked: %v", files)
	}

	// (2) Running the link check over the real file set is clean: the tool no
	// longer fails the adopter on the `docs/v-next.md` path it emitted itself.
	if probs := linkProblems(root, files); len(probs) != 0 {
		t.Fatalf("adopter with intake entries but no docs/v-next.md fails lint on statusgen's own output: %v", probs)
	}

	// (3) NEGATIVE: a genuinely broken backticked path authored in the SOURCE
	// entry is still caught — the exclusion is scoped to generated views only.
	broken := entry + "\nBroken deliverable: `docs/streams/intake/does-not-exist.md`.\n"
	if err := os.WriteFile(filepath.Join(intakeDir, "I-adopter-entry.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	files2, _ := docFiles(root)
	probs2 := linkProblems(root, files2)
	if len(probs2) != 1 || !strings.Contains(probs2[0], "does-not-exist.md") {
		t.Fatalf("adopter-authored dead link in a SOURCE entry must still be caught, got: %v", probs2)
	}
}

// isGeneratedRegisterView is BOTH location-scoped and generation-gated: only
// docs/streams/INTAKE.md and docs/streams/FINDINGS.md are candidate views, AND
// the matching generator must actually emit content for this root (per-entry
// source files must exist). An identically-named file elsewhere in the tree is
// an ordinary authored doc and stays link-checked. This root carries a per-entry
// intake file and a per-entry findings file, so both generators emit.
func TestIsGeneratedRegisterView(t *testing.T) {
	root := t.TempDir()
	intakeDir := filepath.Join(root, "docs", "streams", "intake")
	findingsDir := filepath.Join(root, "docs", "streams", "findings")
	for _, d := range []string{intakeDir, findingsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(intakeDir, "I-x.md"),
		[]byte("---\nid: I-generated-view-test\ndate: 2026-08-15\ntitle: X\ndisposition: watching\n---\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(findingsDir, "2026-08-15-x.md"),
		[]byte("---\nid: F-generated-view-test\ndate: \"2026-08-15\"\ntitle: X\naffects: []\nresolved: true\n---\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		rel  string
		want bool
	}{
		{"docs/streams/INTAKE.md", true},         // view statusgen emits here
		{"docs/streams/FINDINGS.md", true},       // view statusgen emits here
		{"docs/streams/STATUS.md", false},        // STATUS.md is not a docs/ register view
		{"docs/streams/intake/INTAKE.md", false}, // an authored file, not the view
		{"docs/articles/INTAKE.md", false},       // authored doc that happens to share a name
		{"docs/streams/x/README.md", false},      // ordinary brief
		{"INTAKE.md", false},                     // outside docs/streams/
	}
	for _, c := range cases {
		got := isGeneratedRegisterView(root, filepath.Join(root, filepath.FromSlash(c.rel)))
		if got != c.want {
			t.Errorf("isGeneratedRegisterView(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}

// The exclusion is gated on the view ACTUALLY being generated (#169 Blocking 2):
// a hand-authored or scaffolded docs/streams/INTAKE.md in a root with NO
// per-entry intake files is not statusgen's output — the generator emits "" and
// writes nothing there — so it must stay link-checked. This is the fails-to-fire
// regression: keying the exclusion on path alone would silently stop checking a
// real, editable register. `statusgen init` scaffolds exactly this shape (an
// INTAKE.md to edit, no per-entry files), so every fresh adopter starts here.
func TestHandAuthoredRegisterViewStaysChecked(t *testing.T) {
	root := t.TempDir()
	streams := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(streams, 0o755); err != nil {
		t.Fatal(err)
	}
	// A hand-authored INTAKE.md with a dead backticked path, and NO
	// docs/streams/intake/ per-entry files — so generateIntakeView returns "".
	authored := "# Intake\n\nSee `docs/streams/does-not-exist.md` for the queue.\n"
	viewPath := filepath.Join(streams, "INTAKE.md")
	if err := os.WriteFile(viewPath, []byte(authored), 0o644); err != nil {
		t.Fatal(err)
	}

	// Precondition: statusgen would NOT generate a view here.
	if v, err := generateIntakeView(root); err != nil || v != "" {
		t.Fatalf("precondition failed: generateIntakeView emitted content (err=%v, len=%d) — this root must have no per-entry intake files", err, len(v))
	}

	// The predicate must not exclude it.
	if isGeneratedRegisterView(root, viewPath) {
		t.Fatal("isGeneratedRegisterView excluded a hand-authored INTAKE.md with no per-entry source — a real authored document would go unchecked")
	}

	// End-to-end: docFiles keeps it, and its dead link is caught.
	files, walkProblems := docFiles(root)
	if len(walkProblems) != 0 {
		t.Fatalf("unexpected walk problems: %v", walkProblems)
	}
	found := false
	for _, f := range files {
		if strings.HasSuffix(filepath.ToSlash(f), "docs/streams/INTAKE.md") {
			found = true
		}
	}
	if !found {
		t.Fatalf("docFiles dropped the hand-authored INTAKE.md — it would go unchecked: %v", files)
	}
	probs := linkProblems(root, files)
	if len(probs) != 1 || !strings.Contains(probs[0], "does-not-exist.md") {
		t.Fatalf("dead link in a hand-authored register must be caught, got: %v", probs)
	}
}

// A markdown file that lives under a NESTED assay root (a skeleton carrying its
// OWN .assay-versions/STATUS.md — e.g. a tutorial skeleton the learner extracts
// and runs as their repo root) authors its backticked deliverable paths relative
// to THAT nested root, not the outer repo root. The path-existence check must
// resolve against the nested root too, or a path that really exists inside the
// skeleton is falsely reported missing. A genuinely-broken path inside the same
// skeleton must STILL fire — the fix removes the false positive without weakening
// real coverage.
func TestNestedRootBacktickResolution(t *testing.T) {
	root := t.TempDir()
	// The outer repo root markers.
	writeTemp(t, root, ".assay-versions", "statusgen-linux-amd64 v0.0.0 sha256:x\n")
	// A nested assay root two levels below docs/streams/<stream>/, carrying its
	// own root markers — the extracted-repo root the end user runs.
	nested := filepath.Join(root, "docs", "streams", "tutorial", "skeleton", "root")
	answers := filepath.Join(nested, "lessons", "answers")
	if err := os.MkdirAll(answers, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, nested, ".assay-versions", "statusgen-linux-amd64 v0.0.0 sha256:x\n")
	writeTemp(t, nested, "STATUS.md", "# nested board\n")
	writeTemp(t, answers, "1.md", "answer 1")
	// A lesson under the nested root: one backticked path that EXISTS relative to
	// the nested root, and one that is genuinely absent.
	lesson := "The answer lives at `lessons/answers/1.md` in your extracted repo.\n" +
		"Genuinely broken: `lessons/answers/999.md`.\n"
	lp := writeTemp(t, filepath.Join(nested, "lessons"), "lesson-01.md", lesson)

	problems := linkProblems(root, []string{lp})
	// The nested-root-relative path that exists must NOT be flagged.
	for _, pr := range problems {
		if strings.Contains(pr, "lessons/answers/1.md") {
			t.Errorf("nested-root path that exists was falsely flagged: %v", problems)
		}
	}
	// The genuinely-missing path must STILL be flagged (coverage not weakened).
	if len(problems) != 1 || !strings.Contains(problems[0], "999.md") {
		t.Fatalf("want exactly 1 problem (the real missing path 999.md), got: %v", problems)
	}
}

// nestedRootBase returns the nearest ancestor that is itself an assay root
// nested strictly below root, and "" when none applies (so an ordinary brief's
// resolution is untouched). It must never return the outer root and never escape
// above it.
func TestNestedRootBase(t *testing.T) {
	root := t.TempDir()
	writeTemp(t, root, ".assay-versions", "x")
	nested := filepath.Join(root, "docs", "streams", "tutorial", "skeleton", "root")
	deep := filepath.Join(nested, "lessons", "answers")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, nested, "STATUS.md", "x")

	// A file deep under the nested root resolves to the nested root.
	f := filepath.Join(deep, "note.md")
	if got := nestedRootBase(root, f); got != filepath.Clean(nested) {
		t.Errorf("nestedRootBase for a deep file = %q, want %q", got, nested)
	}
	// An ordinary brief with no nested root above it returns "" (outer root is
	// never returned as a nested base — it is already a resolution base).
	plain := filepath.Join(root, "docs", "streams", "ordinary")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := nestedRootBase(root, filepath.Join(plain, "brief.md")); got != "" {
		t.Errorf("nestedRootBase for an ordinary brief = %q, want \"\"", got)
	}
}

// An ordinary brief (no nested root anywhere above it) must resolve EXACTLY as
// before: against the outer root and the containing directory only. This pins
// that the nested-root awareness is additive and does not silently pass a path
// that resolves against neither base.
func TestNestedRootDoesNotAffectOrdinaryBriefs(t *testing.T) {
	root := t.TempDir()
	writeTemp(t, root, ".assay-versions", "x")
	streams := filepath.Join(root, "docs", "streams", "s")
	if err := os.MkdirAll(streams, 0o755); err != nil {
		t.Fatal(err)
	}
	// A dead backticked path in an ordinary brief is still caught.
	content := "Dead: `docs/streams/s/missing.md`.\n"
	p := writeTemp(t, streams, "brief.md", content)
	problems := linkProblems(root, []string{p})
	if len(problems) != 1 || !strings.Contains(problems[0], "missing.md") {
		t.Fatalf("ordinary-brief resolution changed: want 1 problem (missing.md), got: %v", problems)
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
