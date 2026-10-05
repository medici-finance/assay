package deskkit

// selfcontain_briefid_test.go — the session-id arm's ONE exemption (#2022): a brief-v2
// frontmatter `id:` line whose whole value is a lowercase UUID, in a
// `docs/streams/**/brief-*.md` file, is not a session id. Everything else the arm refused it
// still refuses.
//
// Every UUID below is ASSEMBLED AT RUN TIME (bfUUID): no line of this file is itself a
// UUID, because the push-path check this file tests reads the added lines of the change
// that carries it, and the exemption deliberately does not cover a Go test file.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// bfUUID assembles a lowercase 8-4-4-4-12 UUID from a distinguishing prefix and suffix.
func bfUUID(head, tail string) string {
	return head + "-" + "7b4d" + "-" + "4e21" + "-" + "9a8c" + "-" + tail
}

var (
	bfID   = bfUUID("3f2a9c1e", "5d6e7f801234") // the brief's own id
	bfSess = bfUUID("c0ffee42", "a1b2c3d4e5f6") // a planted session id
)

const (
	bfPath   = "docs/streams/demo/brief-01-demo.md"
	bfIDRule = RuleSelfContainPrefix + "session-id"
)

// bfBrief is a minimal brief-v2 file: frontmatter holding idLine, then a body.
func bfBrief(idLine, body string) string {
	return "---\n" +
		"brief: example:example:demo:01\n" +
		"title: \"a demo brief\"\n" +
		"schema: brief-v2\n" +
		idLine + "\n" +
		"---\n\n# Demo\n\n" + body + "\n"
}

// bfCheck runs the outbound check on one kind-`file` field, with or without the file's full
// content as its FileSources entry.
func bfCheck(t *testing.T, path, text, src string, withSrc bool) error {
	t.Helper()
	w := OutboundWrite{Role: "worker", Repo: obPublic, Kind: OutboundKindFile,
		Fields: []OutboundField{{Name: path, Text: text}}}
	if withSrc {
		w.FileSources = map[string]string{path: src}
	}
	return OutboundCheck(w)
}

func bfSetup(t *testing.T) {
	t.Helper()
	obRoster(t)
	t.Cleanup(SetOutboundNoticeWriter(&bytes.Buffer{}))
	SetOutboundContext(OutboundContext{Tool: "briefid", Verb: "test"})
}

func bfWantRefused(t *testing.T, err error, span string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ADMITTED; want a %s refusal naming %q", bfIDRule, span)
	}
	if !IsRefused(err) || !obRuleMatches(err.Error(), bfIDRule) {
		t.Fatalf("want a %s refusal, got %v", bfIDRule, err)
	}
	if !strings.Contains(err.Error(), span) {
		t.Fatalf("refusal does not name %q: %v", span, err)
	}
}

// TestBriefIDFrontmatterAdmitted is (c): the brief-v2 frontmatter `id:` line — bare and
// double-quoted, the two spellings the brief files carry — passes the outbound check.
func TestBriefIDFrontmatterAdmitted(t *testing.T) {
	bfSetup(t)
	for _, idLine := range []string{"id: " + bfID, `id: "` + bfID + `"`} {
		src := bfBrief(idLine, "Plain prose with no identifiers.")
		if err := bfCheck(t, bfPath, src, src, true); err != nil {
			t.Fatalf("%q in a brief's frontmatter was REFUSED: %v", idLine, err)
		}
		// Deeper stream directories match `docs/streams/**/brief-*.md` too.
		deep := "docs/streams/demo/phase-2/brief-07-x.md"
		if err := bfCheck(t, deep, src, src, true); err != nil {
			t.Fatalf("%q in %s was REFUSED: %v", idLine, deep, err)
		}
	}
}

// TestBriefIDProseStillRefused is (a): a session UUID in a brief's PROSE refuses, even with
// the frontmatter id exempt in the same file — the arm reports the first NON-exempt match.
func TestBriefIDProseStillRefused(t *testing.T) {
	bfSetup(t)
	for name, body := range map[string]string{
		"prose":                "The run under session " + bfSess + " recorded it.",
		"id line in the body":  "id: " + bfSess,
		"same id in the body":  "id: " + bfID,
		"quoted id in body":    `id: "` + bfSess + `"`,
		"bare uuid in a table": "| run | " + bfSess + " |",
		"same id in prose":     "Recorded as " + bfID + " on the board.",
	} {
		t.Run(name, func(t *testing.T) {
			src := bfBrief("id: "+bfID, body)
			err := bfCheck(t, bfPath, src, src, true)
			span := bfSess
			if strings.Contains(body, bfID) {
				span = bfID
			}
			bfWantRefused(t, err, span)
		})
	}

	// The span is the ONE id line, not the frontmatter and not the value: a UUID on any
	// other frontmatter line — above or below the id, a different value or the same one —
	// still refuses.
	for name, c := range map[string]struct{ fm, span string }{
		"other key above the id":     {"run: " + bfSess + "\nid: " + bfID, bfSess},
		"other key below the id":     {"id: " + bfID + "\nrun: " + bfSess, bfSess},
		"same value under other key": {"id: " + bfID + "\nparent: " + bfID, bfID},
	} {
		t.Run(name, func(t *testing.T) {
			src := bfBrief(c.fm, "prose")
			bfWantRefused(t, bfCheck(t, bfPath, src, src, true), c.span)
		})
	}
}

// TestBriefIDOtherArmsArmed: the exemption is on the session-id arm only. A brief whose
// frontmatter id IS exempt still refuses an agent id, a machine path or a worktree name in
// its body. Each span is assembled at run time, like the UUIDs, so no line of this file
// carries one.
func TestBriefIDOtherArmsArmed(t *testing.T) {
	bfSetup(t)
	for _, c := range []struct{ rule, span string }{
		{"agent-id", "agent-" + "0a1b2c3d" + "4e5f"},
		{"absolute-machine-path", "/Us" + "ers/someone/notes.md"},
		{"scratch-worktree-name", "trac" + "ker-demo-item"},
	} {
		t.Run(c.rule, func(t *testing.T) {
			src := bfBrief("id: "+bfID, "See "+c.span+" for the run.")
			err := bfCheck(t, bfPath, src, src, true)
			if err == nil || !obRuleMatches(err.Error(), RuleSelfContainPrefix+c.rule) ||
				!strings.Contains(err.Error(), c.span) {
				t.Fatalf("want a %s%s refusal naming %q, got %v", RuleSelfContainPrefix, c.rule, c.span, err)
			}
		})
	}
}

// TestBriefIDOutsideShapeRefused is (b) and the fail-closed conditions: every shape that is
// not EXACTLY the one exempted refuses, whether or not the full content is supplied.
func TestBriefIDOutsideShapeRefused(t *testing.T) {
	bfSetup(t)
	good := bfBrief("id: "+bfID, "prose")
	noFM := "# Demo\n\nprose\n\nid: " + bfID + "\n"
	// The id-less frontmatter closes at its FIRST fence; the body's id line sits ahead of a
	// later `---` rule, so a predicate that took the LAST fence would put it "inside".
	idlessFM := "---\nschema: brief-v2\n---\n\n# Demo\n\nid: " + bfID + "\n\n---\n\nmore\n"
	// No fence on line 1, but an id line ahead of a `---`: only the first-line test refuses.
	noOpenFence := "# Demo\nid: " + bfID + "\n---\nprose\n"
	dup := func(second string) string { return bfBrief("id: "+bfID+"\n"+second, "prose") }
	cases := []struct {
		name, path, text, src string
		withSrc               bool
	}{
		{"no frontmatter, id line in body", bfPath, noFM, noFM, true},
		{"unterminated frontmatter", bfPath, "---\nschema: brief-v2\nid: " + bfID + "\n\nprose\n",
			"---\nschema: brief-v2\nid: " + bfID + "\n\nprose\n", true},
		{"blank line 1, fence on line 2", bfPath, "\n" + good, "\n" + good, true},
		{"first line not a fence", bfPath, noOpenFence, noOpenFence, true},
		{"id-less fm, body id, later rule", bfPath, idlessFM, idlessFM, true},
		{"stream README", "docs/streams/demo/README.md", good, good, true},
		{"non-brief file in a stream", "docs/streams/demo/notes.md", good, good, true},
		{"brief name outside docs/streams", "docs/brief-01-demo.md", good, good, true},
		{"docs/streams nested elsewhere", "vendor/docs/streams/demo/brief-01.md", good, good, true},
		{"leading slash", "/docs/streams/demo/brief-01-demo.md", good, good, true},
		{"prefix case changed", "Docs/Streams/demo/brief-01-demo.md", good, good, true},
		{"brief name case changed", "docs/streams/demo/Brief-01-demo.md", good, good, true},
		{"extension case changed", "docs/streams/demo/brief-01-demo.MD", good, good, true},
		{"name prefixed before brief-", "docs/streams/demo/xbrief-01-demo.md", good, good, true},
		{"dot-dot segment", "docs/streams/../brief-01-demo.md", good, good, true},
		{"dot segment", "docs/streams/./brief-01-demo.md", good, good, true},
		{"empty segment", "docs/streams//brief-01-demo.md", good, good, true},
		{"backslash in the name", `docs/streams/demo/brief-x\..\..\notes.md`, good, good, true},
		{"brief named file, wrong extension", "docs/streams/demo/brief-01-demo.txt", good, good, true},
		{"Go test file", "docs/streams/demo/brief_test.go", good, good, true},
		{"no full content supplied", bfPath, good, "", false},
		{"text not line-aligned with source", bfPath, "id: " + bfID + "\n", good, true},
		{"text line differs from source line", bfPath, bfBrief("id: "+bfSess, "prose"), good, true},
		// Both lines carry the UUID here, so the second line refuses whatever the key count
		// says; the cases below are the ones that pin the count itself.
		{"duplicate id key, both uuid", bfPath, dup("id: " + bfID), dup("id: " + bfID), true},
		{"duplicate id key, non-uuid", bfPath, dup("id: second"), dup("id: second"), true},
		{"duplicate key double-quoted", bfPath, dup(`"id": second`), dup(`"id": second`), true},
		{"duplicate key single-quoted", bfPath, dup(`'id': second`), dup(`'id': second`), true},
		{"duplicate key, space before :", bfPath, dup("id : second"), dup("id : second"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := bfCheck(t, c.path, c.text, c.src, c.withSrc)
			span := bfID
			if strings.Contains(c.text, bfSess) {
				span = bfSess
			}
			bfWantRefused(t, err, span)
		})
	}
}

// TestBriefIDVariantsUnchanged is (d): a frontmatter line that is not exactly
// `id: <lowercase uuid>` (or its double-quoted form) gets EXACTLY the verdict it got before
// the exemption existed — the verdict with the full content supplied equals the verdict
// without it. The refusing variants are also asserted to refuse.
func TestBriefIDVariantsUnchanged(t *testing.T) {
	bfSetup(t)
	upper := strings.ToUpper(bfID)
	undashed := strings.ReplaceAll(bfID, "-", "")
	cases := []struct {
		line    string
		refused bool
	}{
		{"id: " + bfID + " extra", true},
		{"id: " + bfID + " # comment", true},
		{"id: prefix id: " + bfID, true}, // text BEFORE the value: pins the `^` anchor
		{`id: "` + bfID, true},           // unbalanced: opening quote only
		{"id: " + bfID + `"`, true},      // unbalanced: closing quote only
		{"id: " + bfID + "\r", true},
		{"id:  " + bfID, true},
		{"id:" + bfID, true},
		{"  id: " + bfID, true},
		{"id: '" + bfID + "'", true},
		{"id : " + bfID, true},
		{"brief_id: " + bfID, true},
		{"session: " + bfID, true},
		{"id: [" + bfID + "]", true},
		{"id: " + upper, false},    // the arm is lowercase-only: never refused, before or after
		{"id: " + undashed, false}, // not the 8-4-4-4-12 shape: never refused, before or after
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			src := bfBrief(c.line, "prose")
			with := bfCheck(t, bfPath, src, src, true)
			without := bfCheck(t, bfPath, src, src, false)
			if (with == nil) != (without == nil) || (with != nil && with.Error() != without.Error()) {
				t.Fatalf("the exemption CHANGED the verdict on %q:\n with source: %v\n without:     %v",
					c.line, with, without)
			}
			if c.refused {
				bfWantRefused(t, with, bfID)
			} else if with != nil && obRuleMatches(with.Error(), bfIDRule) {
				t.Fatalf("%q refused as a session id: %v", c.line, with)
			}
		})
	}
}

// TestBriefIDBodiesStillRefused is (e): the same `id: <uuid>` text in a commit message, a PR
// body, an issue body or a comment refuses. Only a kind-`file` field reads FileSources, so a
// body write that carries one anyway gets no exemption.
func TestBriefIDBodiesStillRefused(t *testing.T) {
	bfSetup(t)
	brief := bfBrief("id: "+bfID, "prose")
	if err := SelfContainCheck("PR body", []byte(brief),
		SelfContainOpts{Repo: obPublic, NumberHint: 9000, Notices: &bytes.Buffer{}}); err == nil ||
		!strings.Contains(err.Error(), bfID) {
		t.Fatalf("PR body carrying a brief's frontmatter id: want a session-id refusal, got %v", err)
	}
	for _, kind := range []string{OutboundKindChange, OutboundKindIssue, OutboundKindComment,
		OutboundKindReview, OutboundKindCommit} {
		t.Run(kind, func(t *testing.T) {
			err := OutboundCheck(OutboundWrite{Role: "worker", Repo: obPublic, Kind: kind,
				Fields:      []OutboundField{{Name: bfPath, Text: brief}},
				FileSources: map[string]string{bfPath: brief}})
			bfWantRefused(t, err, bfID)
		})
	}
}

// TestBriefIDPushPath runs the exemption where #2022 bit: OutboundCheckPush over a real
// branch diff, which reads only ADDED lines and must establish the frontmatter from the
// file's head content.
func TestBriefIDPushPath(t *testing.T) {
	bfSetup(t)
	p := newObPushRepo(t)
	push := func(name string, files map[string]string, msg string) error {
		t.Helper()
		p.git("checkout", "-q", "-b", name, "main")
		keys := make([]string, 0, len(files))
		for k := range files {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p.write(k, files[k])
			p.git("add", k)
		}
		p.git("commit", "-q", "-m", msg)
		p.git("checkout", "-q", "main")
		return OutboundCheckPush(OutboundPush{Dir: p.dir, Repo: obPublic, Base: "main", Head: name,
			Branch: name, Role: "worker"})
	}

	if err := push("new-brief", map[string]string{bfPath: bfBrief("id: "+bfID, "prose")},
		"add a brief"); err != nil {
		t.Fatalf("a new brief with a frontmatter id was REFUSED on the push path: %v", err)
	}

	// A MODIFIED brief: main carries it without an id; the branch adds only the id line, so
	// both fences are unchanged context — blank in the added-lines view.
	old := "docs/streams/demo/brief-02-old.md"
	p.write(old, bfBrief("wave: 1", "prose"))
	p.git("add", old)
	p.git("commit", "-q", "-m", "seed an older brief")
	if err := push("add-id", map[string]string{old: bfBrief("wave: 1\nid: "+bfID, "prose")},
		"give the brief an id"); err != nil {
		t.Fatalf("adding the id line to an existing brief's frontmatter was REFUSED: %v", err)
	}

	bfWantRefused(t, push("prose-sess", map[string]string{
		"docs/streams/demo/brief-03-p.md": bfBrief("id: "+bfID, "session "+bfSess+" ran it"),
	}, "add a brief with prose"), bfSess)

	bfWantRefused(t, push("readme-id", map[string]string{
		"docs/streams/demo/README.md": bfBrief("id: "+bfID, "prose"),
	}, "add a readme"), bfID)

	err := push("commit-id", map[string]string{"docs/streams/demo/brief-04-c.md": bfBrief("wave: 1", "prose")},
		"add a brief\n\nid: "+bfID)
	bfWantRefused(t, err, bfID)
	if !strings.Contains(err.Error(), "(commit write to") {
		t.Fatalf("the commit-message refusal is not a commit write: %v", err)
	}
}

// TestBriefIDForgeWriteFile runs the exemption on the Forge file-write seam, which carries
// the write's full content (deskevidence's pre-flight hands the same evidence).
func TestBriefIDForgeWriteFile(t *testing.T) {
	bfSetup(t)
	write := func(file, content, msg string) (*outboundRecordingForge, error) {
		fake := &outboundRecordingForge{}
		_, err := OutboundChecked(fake, "worker").WriteFile(obRepo(obPublic), WriteFileInput{
			File: file, Branch: "main", Content: []byte(content), Message: msg})
		return fake, err
	}
	fake, err := write(bfPath, bfBrief("id: "+bfID, "prose"), "add a brief")
	if err != nil || len(fake.calls) != 1 {
		t.Fatalf("a brief with a frontmatter id was refused at WriteFile (calls %d): %v", len(fake.calls), err)
	}
	_, err = write(bfPath, bfBrief("id: "+bfID, "session "+bfSess), "add a brief")
	bfWantRefused(t, err, bfSess)
	_, err = write(bfPath, bfBrief("wave: 1", "prose"), "id: "+bfID)
	bfWantRefused(t, err, bfID)
	_, err = write("docs/streams/demo/notes.md", bfBrief("id: "+bfID, "prose"), "add notes")
	bfWantRefused(t, err, bfID)
}

// fileKindOffenders is the CLASS GUARD's matcher. The defect class #2022 is: a kind-`file`
// outbound check that reaches the self-containment scan WITHOUT the file's full content, so
// a brief's frontmatter id cannot be told from a session id and the write is falsely
// refused. It reports, by position:
//
//   - every use of OutboundKindFile that is not the Kind of an OutboundWrite literal that
//     also sets FileSources (a bare `o.check(repo, OutboundKindFile, …)` call is exactly the
//     shape the WriteFile seam had before #2022);
//   - every use of SelfContainOpts.FileSource outside outbound.go and selfcontain.go, so the
//     exemption's evidence can only come from the outbound check's kind-`file` arm;
//   - every use of SelfContainOpts.InFile outside those two files (#2217), so the
//     synthetic-fixture exemption likewise reaches only a kind-`file` write, never a body.
func fileKindOffenders(fset *token.FileSet, f *ast.File, base string) []string {
	allowed := map[*ast.Ident]bool{}
	identOf := func(e ast.Expr) *ast.Ident {
		switch v := e.(type) {
		case *ast.Ident:
			return v
		case *ast.SelectorExpr:
			return v.Sel
		}
		return nil
	}
	// A comparison or a case label only READS the kind (`w.Kind == OutboundKindFile`); it
	// runs no check, so it is not a site of the class.
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			for _, e := range []ast.Expr{x.X, x.Y} {
				if id := identOf(e); id != nil {
					allowed[id] = true
				}
			}
		case *ast.CaseClause:
			for _, e := range x.List {
				if id := identOf(e); id != nil {
					allowed[id] = true
				}
			}
		}
		return true
	})
	ast.Inspect(f, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok || compositeLitTypeName(cl.Type) != "OutboundWrite" {
			return true
		}
		var kind *ast.Ident
		hasSources := false
		for _, e := range cl.Elts {
			kv, ok := e.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			k, _ := kv.Key.(*ast.Ident)
			if k == nil {
				continue
			}
			switch k.Name {
			case "Kind":
				kind = identOf(kv.Value)
			case "FileSources":
				hasSources = true
			}
		}
		if kind != nil && kind.Name == "OutboundKindFile" && hasSources {
			allowed[kind] = true
		}
		return true
	})
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			for _, nm := range x.Names {
				allowed[nm] = true // the declaration itself
			}
		case *ast.Ident:
			switch {
			case x.Name == "OutboundKindFile" && !allowed[x]:
				out = append(out, fset.Position(x.Pos()).String()+": OutboundKindFile check without FileSources")
			case x.Name == "FileSource" && !allowed[x] && base != "outbound.go" && base != "selfcontain.go":
				out = append(out, fset.Position(x.Pos()).String()+": SelfContainOpts.FileSource set outside the outbound check")
			case x.Name == "InFile" && !allowed[x] && base != "outbound.go" && base != "selfcontain.go":
				out = append(out, fset.Position(x.Pos()).String()+": SelfContainOpts.InFile set outside the outbound check")
			}
		}
		return true
	})
	return out
}

// TestFileKindSitesCarrySources is the class guard over the whole tools/desk tree (non-test
// Go files), with a positive control so a matcher that silently stops matching fails here
// instead of reporting clean.
func TestFileKindSitesCarrySources(t *testing.T) {
	const planted = `package p
func f() {
	_ = deskkit.OutboundCheck(deskkit.OutboundWrite{Kind: deskkit.OutboundKindFile, Fields: nil})
	_ = o.check(repo, OutboundKindFile, x)
	_ = SelfContainOpts{FileSource: "x"}
	_ = SelfContainOpts{InFile: true}
	_ = OutboundCheck(OutboundWrite{Kind: OutboundKindFile, FileSources: m})
	_ = k == OutboundKindFile
}
`
	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, "planted.go", planted, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileKindOffenders(fset, pf, "planted.go"); len(got) != 4 {
		t.Fatalf("positive control: the guard flagged %d planted sites, want 4 (the compliant "+
			"literal and the comparison must pass): %v", len(got), got)
	}

	var offenders []string
	sites := 0
	werr := filepath.Walk(deskTreeRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "testdata", ".git", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fs := token.NewFileSet()
		f, perr := parser.ParseFile(fs, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "OutboundKindFile" {
				sites++
			}
			return true
		})
		offenders = append(offenders, fileKindOffenders(fs, f, filepath.Base(path))...)
		return nil
	})
	if werr != nil {
		t.Fatal(werr)
	}
	if sites < 4 { // the declaration + the push path + the WriteFile seam + deskevidence
		t.Fatalf("the walk saw %d OutboundKindFile uses, want at least 4 — is it walking the tree?", sites)
	}
	if len(offenders) > 0 {
		t.Fatalf("kind-`file` outbound checks without the file's full content (#2022 — supply "+
			"OutboundWrite.FileSources):\n  %s", strings.Join(offenders, "\n  "))
	}
}
