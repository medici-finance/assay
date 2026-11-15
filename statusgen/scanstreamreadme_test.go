package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #5895: a scan into a repo with no issue-loop stream opened the directory
// and wrote only placeholders, so the scan tree failed `statusgen --lint` with
// "stream directory issue-loop has no README.md". Every scan lane that creates
// placeholders now opens the stream through openScanStream, which writes the
// stream README when it is absent and never overwrites one.

// scanLoopDir is the scan stream directory under a fixture root.
func scanLoopDir(root string) string {
	return filepath.Join(root, "docs", "streams", scanStreamName)
}

// lintExit runs the offline --lint gate over root and returns its exit code.
func lintExit(t *testing.T, root string) int {
	t.Helper()
	return run(root, "lint", nil, nil, "")
}

// regression: #5895
func TestScanNewStreamLints(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	dir := scanLoopDir(root)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("fixture precondition: %s must not exist (stat err=%v)", dir, err)
	}
	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 801, Title: "plain bug", Labels: lbl("bug")}},
	}
	if code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll, labelHoldClearAll); code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("the scan opened %s without its README: %v", dir, err)
	}
	s, err := parseStreamREADME(filepath.Join(dir, "README.md"))
	if err != nil || s.Name != scanStreamName {
		t.Fatalf("README must parse as stream %q: s=%+v err=%v", scanStreamName, s, err)
	}
	if code := lintExit(t, root); code != 0 {
		t.Fatalf("lint on the scan tree exited %d, want 0", code)
	}

	// A second scan finds the README in place and leaves it byte-identical.
	data[scanHomeRepo()] = append(data[scanHomeRepo()], ghIssue{Number: 802, Title: "another bug", Labels: lbl("bug")})
	if code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll, labelHoldClearAll); code != 0 {
		t.Fatalf("second scan exited %d, want 0", code)
	}
	again, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || string(again) != string(raw) {
		t.Errorf("second scan changed the README (err=%v)", err)
	}
	if code := lintExit(t, root); code != 0 {
		t.Fatalf("lint after the second scan exited %d, want 0", code)
	}
}

// The directory already exists with its own README: the scan writes the
// placeholder exactly as before and never touches the authored README.
func TestScanKeepsAuthoredREADME(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	dir := scanLoopDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, dir, "README.md", scanStreamREADME)
	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 811, Title: "plain bug", Labels: lbl("bug")}},
	}
	out := captureRun(t, func() int {
		return runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll, labelHoldClearAll)
	})
	if out.code != 0 {
		t.Fatalf("scan exited %d, want 0:\n%s", out.code, out.log)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || string(raw) != scanStreamREADME {
		t.Errorf("an existing README must stay byte-identical (err=%v):\n%s", err, raw)
	}
	if strings.Contains(out.log, "README.md") {
		t.Errorf("an existing README must not be reported as created:\n%s", out.log)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "README.md,issue-811.md" {
		t.Errorf("stream directory holds %v, want exactly README.md and issue-811.md", names)
	}
	if code := lintExit(t, root); code != 0 {
		t.Fatalf("lint exited %d, want 0", code)
	}
}

// Dry-run names the README it would write and writes nothing.
func TestScanDryRunNamesREADME(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 831, Title: "plain bug", Labels: lbl("bug")}},
	}
	out := captureRun(t, func() int {
		return runScanIssues(root, true, fixtureLister(data, ""), nilCommentLister, blessAll, labelHoldClearAll)
	})
	if out.code != 0 {
		t.Fatalf("dry-run exited %d, want 0:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.log, "would create docs/streams/issue-loop/README.md") {
		t.Errorf("dry-run must name the README it would write:\n%s", out.log)
	}
	if _, err := os.Stat(scanLoopDir(root)); !os.IsNotExist(err) {
		t.Errorf("dry-run must write nothing (stat err=%v)", err)
	}
}

// regression: #5895 — the transcription lane opens the same directory.
func TestTranscribeNewStreamLints(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)
	list := fixtureLister(map[string][]ghIssue{
		"example-org/tracker": {{Number: 841, Title: "keydown bubbles", Labels: lbl("bug")}},
	}, "")
	authors := fixtureAuthorResolver(map[string]authorIdentity{
		"example-org/tracker#841": {Login: "dana", ID: 200002, Type: "User"},
	})
	out := captureRun(t, func() int {
		return runTranscribeScan(root, false, list, nilCommentLister, authors, blessNone, labelHoldClearAll, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("transcribe-scan exited %d, want 0:\n%s", out.code, out.log)
	}
	if _, err := os.Stat(filepath.Join(scanLoopDir(root), "README.md")); err != nil {
		t.Fatalf("transcribe-scan opened the stream without its README: %v\n%s", err, out.log)
	}
	if code := lintExit(t, root); code != 0 {
		t.Fatalf("lint on the transcribe-scan tree exited %d, want 0", code)
	}
}

// regression: #5895 — the cross-repo delta lane opens the same directory.
func TestScanDeltaNewStreamLints(t *testing.T) {
	scanWithRoster(t, scanDeltaRoster())
	key := scanDeltaTestKey(t)
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)
	e := scanDeltaEntryFixture("example-org/widget", 42, "dana", 200002, "User", "rostered")
	pl := scanDeltaPayload{Schema: scanDeltaSchemaVersion, Entries: []scanDeltaEntry{e}}
	body := signScanDeltaBody(t, key, pl, "issue-loop")
	vi := scanDeltaContainer(body, scanDeltaIssueLoopIdent, false)
	list := fixtureLister(map[string][]ghIssue{"example-org/tracker": {{Number: 900, Title: "scan-delta batch", Labels: lbl("scan-delta")}}}, "")
	resolveIssue := fixtureVerdictResolver(map[string]verdictIssue{"example-org/tracker#900": vi})
	authors := fixtureAuthorResolver(map[string]authorIdentity{"example-org/widget#42": {Login: "dana", ID: 200002, Type: "User"}})
	out := captureRun(t, func() int {
		return runTranscribeScanDelta(root, false, "", list, resolveIssue, authors, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("apply exited %d, want 0:\n%s", out.code, out.log)
	}
	if _, err := os.Stat(filepath.Join(scanLoopDir(root), "README.md")); err != nil {
		t.Fatalf("the delta lane opened the stream without its README: %v\n%s", err, out.log)
	}
	if code := lintExit(t, root); code != 0 {
		t.Fatalf("lint on the delta tree exited %d, want 0", code)
	}
}

// bareScanDirOpens lists the functions in src that both name scanStreamName
// and call os.MkdirAll — a scan lane opening the stream directory by hand,
// which is how the README went missing. openScanStream is the one allowed site.
func bareScanDirOpens(t *testing.T, name string, src any) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var hits []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name == "openScanStream" {
			continue
		}
		namesStream, mkdirs := false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "scanStreamName" {
					namesStream = true
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "os" && x.Sel.Name == "MkdirAll" {
					mkdirs = true
				}
			}
			return true
		})
		if namesStream && mkdirs {
			hits = append(hits, name+": "+fn.Name.Name)
		}
	}
	return hits
}

// regression: #5895 — every lane opens the scan stream through openScanStream.
func TestScanStreamOneOpener(t *testing.T) {
	// Positive control: a planted bare opener must be flagged, so a matcher
	// that silently stopped matching fails here instead of reporting clean.
	plant := "package main\nimport (\"os\"; \"path/filepath\")\nfunc plantedLane(root string) error {\n\treturn os.MkdirAll(filepath.Join(root, \"docs\", \"streams\", scanStreamName), 0o755)\n}\n"
	if hits := bareScanDirOpens(t, "plant.go", plant); len(hits) != 1 {
		t.Fatalf("positive control: the planted opener must be flagged, got %v", hits)
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var hits []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		hits = append(hits, bareScanDirOpens(t, f, nil)...)
	}
	if len(hits) != 0 {
		t.Errorf("open docs/streams/%s through openScanStream, which writes its README; bare openers: %v", scanStreamName, hits)
	}
}
