package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// writeGapFile writes body to root/rel, creating its directories.
func writeGapFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bakeModule plants path@version in a module cache the way `go mod download`
// leaves it: the .mod and .zip under cache/download.
func bakeModule(t *testing.T, cache, path, version string) {
	t.Helper()
	dl := "cache/download/" + escapeModulePath(path) + "/@v/" + escapeModulePath(version)
	writeGapFile(t, cache, dl+".mod", "module "+path+"\n")
	writeGapFile(t, cache, dl+".zip", "zip")
}

const gapTreeGoMod = `module example.com/desk

go 1.27.0

require (
	example.com/local v0.0.0-00010101000000-000000000000
	github.com/BurntSushi/toml v1.4.0 // a comment
	golang.org/x/text v0.43.0 // indirect
)

require golang.org/x/sys v0.49.0

replace example.com/local => ../local

replace (
	example.com/old v1.0.0 => example.com/new v1.2.0
)

require example.com/old v1.0.0
`

// TestGoModRequirementsParse: block and single-line requires, comments
// stripped, a local replace dropped, a module replace followed.
func TestGoModRequirementsParse(t *testing.T) {
	root := t.TempDir()
	writeGapFile(t, root, "go.mod", gapTreeGoMod)
	reqs, err := goModRequirements(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, q := range reqs {
		got = append(got, q.Path+"@"+q.Version)
	}
	want := "github.com/BurntSushi/toml@v1.4.0 golang.org/x/text@v0.43.0 golang.org/x/sys@v0.49.0 example.com/new@v1.2.0"
	if strings.Join(got, " ") != want {
		t.Fatalf("requirements\n got %s\nwant %s", strings.Join(got, " "), want)
	}
}

// TestModuleCacheGap: a requirement the cache holds at the wanted version is no
// gap; one the cache holds only at an older version is a gap naming both; the
// case-encoded path is found; a module the cache never had has no versions.
func TestModuleCacheGap(t *testing.T) {
	root, cache := t.TempDir(), t.TempDir()
	writeGapFile(t, root, "tools/desk/go.mod", gapTreeGoMod)
	bakeModule(t, cache, "github.com/BurntSushi/toml", "v1.4.0")
	bakeModule(t, cache, "golang.org/x/text", "v0.41.0")
	bakeModule(t, cache, "golang.org/x/text", "v0.9.0")
	bakeModule(t, cache, "golang.org/x/sys", "v0.49.0")
	// a .mod without its source is not enough to build against
	writeGapFile(t, cache, "cache/download/example.com/new/@v/v1.2.0.mod", "module example.com/new\n")

	gaps := moduleCacheGap(root, []string{"tools/desk", "statusgen"}, cache)
	if len(gaps) != 2 {
		t.Fatalf("gaps = %+v, want golang.org/x/text and example.com/new", gaps)
	}
	if g := gaps[0]; g.Path != "golang.org/x/text" || g.Want != "v0.43.0" || strings.Join(g.Have, ",") != "v0.9.0,v0.41.0" || g.Root != "tools/desk" {
		t.Errorf("first gap = %+v", g)
	}
	if g := gaps[1]; g.Path != "example.com/new" || len(g.Have) != 0 {
		t.Errorf("second gap = %+v", g)
	}

	// An extracted tree stands in for the zip.
	writeGapFile(t, cache, "example.com/new@v1.2.0/go.mod", "module example.com/new\n")
	if gaps := moduleCacheGap(root, []string{"tools/desk"}, cache); len(gaps) != 1 {
		t.Errorf("with the extracted tree present, gaps = %+v, want only golang.org/x/text", gaps)
	}
}

// TestModCacheGapNoteWording: the cause names module, wanted and held version;
// "predates" only when every held version is older than the wanted one.
func TestModCacheGapNoteWording(t *testing.T) {
	newer := []modGap{{Root: "tools/desk", Path: "golang.org/x/text", Want: "v0.43.0", Have: []string{"v0.41.0"}}}
	note := modCacheGapNote(newer)
	if !strings.HasPrefix(note, "image module cache predates go.sum (golang.org/x/text v0.43.0 vs v0.41.0)") {
		t.Errorf("note = %q", note)
	}
	if !strings.Contains(note, "tools/desk go.mod and go.sum") || strings.Contains(note, "|") {
		t.Errorf("note %q does not name the root, or carries a table separator", note)
	}
	older := []modGap{{Root: "statusgen", Path: "golang.org/x/text", Want: "v0.39.0", Have: []string{"v0.41.0"}}}
	if c := modCacheGapCause(older); c != "image module cache does not match go.sum (golang.org/x/text v0.39.0 vs v0.41.0)" {
		t.Errorf("older-tree cause = %q", c)
	}
	never := []modGap{{Path: "example.com/x", Want: "v1.0.0"}}
	if c := modCacheGapCause(never); c != "image module cache does not match go.sum (example.com/x v1.0.0 vs none)" {
		t.Errorf("never-baked cause = %q", c)
	}
	many := append(append(append(append([]modGap{}, newer...), newer...), newer...), newer...)
	if c := modCacheGapCause(many); !strings.HasSuffix(c, "; and 1 more)") {
		t.Errorf("four gaps cause = %q", c)
	}
	for _, tc := range [][2]string{{"v0.9.0", "v0.41.0"}, {"v1.0.0-rc1", "v1.0.0"}, {"v0.0.0-20211028202545-6944b10bf410", "v0.47.0"}} {
		if !semverLess(tc[0], tc[1]) || semverLess(tc[1], tc[0]) {
			t.Errorf("semverLess order wrong for %s < %s", tc[0], tc[1])
		}
	}
}

// goOfflineMissing is the shape go prints when a read-only module cache lacks a
// version and GOPROXY=off (#4683).
const goOfflineMissing = `go: downloading golang.org/x/text v0.43.0
/go/pkg/mod/github.com/cli/go-gh/v2@v2.13.0/pkg/asciisanitizer/sanitizer.go:14:2: open /go/pkg/mod/cache/download/golang.org/x/text/@v/v0.43.0.lock: read-only file system`

// TestGapsNamedIn: each of go's three ways of naming the module counts; an
// output that names another module, or another version, does not.
func TestGapsNamedIn(t *testing.T) {
	gaps := []modGap{{Path: "golang.org/x/text", Want: "v0.43.0"}, {Path: "github.com/BurntSushi/toml", Want: "v1.4.0"}}
	for _, out := range []string{
		goOfflineMissing,
		"go: golang.org/x/text@v0.43.0: module lookup disabled by GOPROXY=off",
		"open /go/pkg/mod/cache/download/golang.org/x/text/@v/v0.43.0.mod: no such file",
	} {
		if hit := gapsNamedIn([]byte(out), gaps); len(hit) != 1 || hit[0].Path != "golang.org/x/text" {
			t.Errorf("gapsNamedIn(%q) = %+v", out, hit)
		}
	}
	if hit := gapsNamedIn([]byte("open /go/pkg/mod/cache/download/github.com/!burnt!sushi/toml/@v/v1.4.0.lock: read-only file system"), gaps); len(hit) != 1 {
		t.Errorf("case-encoded cache path not matched: %+v", hit)
	}
	for _, out := range []string{"--- FAIL: TestX", "golang.org/x/text@v0.41.0", "golang.org/x/text@v0.43.01", "example.org/golang.org/x/text@v0.43.0-rc1"} {
		if hit := gapsNamedIn([]byte(out), gaps); len(hit) != 0 {
			t.Errorf("gapsNamedIn(%q) = %+v, want none", out, hit)
		}
	}
}

// stubBakedRoots points the runner's baked-roots manifest and module cache at
// fixtures for one test.
func stubBakedRoots(t *testing.T, roots, cache string) {
	t.Helper()
	manifest := filepath.Join(t.TempDir(), "go-module-roots.txt")
	if err := os.WriteFile(manifest, []byte(roots), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := bakedModuleRootsFile
	bakedModuleRootsFile = manifest
	t.Cleanup(func() { bakedModuleRootsFile = orig })
	t.Setenv("GOMODCACHE", cache)
}

// TestContainerRowStoppedOnMissingModule: in container-netns mode a row that
// exits non-zero naming a module the cache lacks is could-not-run with the
// precise cause, even when its Expect wanted that exit; a row that exits
// non-zero with NO captured output under a gap (#5766) is could-not-run too,
// with a cause that says the output was not captured; a row that fails for its
// own reasons (noisy, no module named) stays fail; a passing row passes, and no
// row moves toward pass. The unshare lane never reclassifies, and with no
// manifest, or no gap, nothing is reclassified.
func TestContainerRowStoppedOnMissingModule(t *testing.T) {
	stubIfaces(t, netnoneIfaces)
	root, cache := t.TempDir(), t.TempDir()
	writeGapFile(t, root, "tools/desk/go.mod", "module example.com/desk\n\ngo 1.27.0\n\nrequire golang.org/x/text v0.43.0\n")
	bakeModule(t, cache, "golang.org/x/text", "v0.41.0")
	writeGapFile(t, root, "goerr.txt", goOfflineMissing+"\n")
	stubBakedRoots(t, "tools/desk\nstatusgen\n", cache)

	run := func(sandbox string, rows []verifyRow) []witness {
		t.Helper()
		return runWitnessesSandboxed(resolveShellPlan(), sandbox, root, rows, "human:tester", "", "0000", "2026-10-28", 30*time.Second, false)
	}
	rows := []verifyRow{
		{ID: "1", Command: "cat goerr.txt; exit 1", Expect: "exit 0"},
		{ID: "2", Command: "echo '--- FAIL: TestX'; exit 1", Expect: "exit 0"},
		{ID: "3", Command: "true", Expect: "exit 0"},
		{ID: "4", Command: "cat goerr.txt; exit 1", Expect: "exit 1"},
		// The silent-output hole (#5766): the row's output went to a file or to
		// /dev/null, so nothing was captured to name the missing module.
		{ID: "5", Command: "exit 1", Expect: "exit 0"},
		{ID: "6", Command: "exit 1", Expect: "exit 1"},
		{ID: "7", Command: ": > /dev/null; exit 0", Expect: "exit 0"},
		{ID: "8", Command: "echo '--- FAIL: TestX' ; echo more; exit 2", Expect: "exit 0"},
		// A second silent shape at a site the first fix shape does not name: a
		// different exit code, output sent to a file, and to /dev/null.
		{ID: "9", Command: "echo boom > out.log 2>&1; exit 2", Expect: "exit 0"},
		{ID: "10", Command: "sh -c 'echo boom' >/dev/null 2>&1; exit 137", Expect: "exit 0"},
		// No move toward pass: a silent exit 0 whose Expect wanted another exit
		// stays fail, and a noisy row that exited as Expect wanted stays pass.
		{ID: "11", Command: "exit 0", Expect: "exit 1"},
		{ID: "12", Command: "echo ok", Expect: "exit 0"},
	}
	ws := run(sandboxContainerNetns, rows)
	want := []string{
		stateCouldNotRun, stateFail, statePass, stateCouldNotRun,
		stateCouldNotRun, stateCouldNotRun, statePass, stateFail,
		stateCouldNotRun, stateCouldNotRun, stateFail, statePass,
	}
	for i, w := range ws {
		if w.State != want[i] {
			t.Errorf("row %s: state %q (%s), want %q", w.ID, w.State, w.Note, want[i])
		}
	}
	cause := regexp.MustCompile(`could-not-run exit=1 sandbox=container-netns — image module cache predates go\.sum \(golang\.org/x/text v0\.43\.0 vs v0\.41\.0\)`)
	for _, i := range []int{0, 3} {
		if !cause.MatchString(ws[i].row()) {
			t.Errorf("row %s witness %q does not carry the module cause", ws[i].ID, ws[i].row())
		}
	}
	// A silent row's cause names the gap and says why the module cause could
	// not be settled; a row whose output named the module does not say that.
	silent := regexp.MustCompile(`could-not-run exit=\d+ sandbox=container-netns — image module cache predates go\.sum \(golang\.org/x/text v0\.43\.0 vs v0\.41\.0\): ` +
		`the row's output was not captured, so a stop on module resolution could be neither confirmed nor excluded`)
	for _, i := range []int{4, 5, 8, 9} {
		if !silent.MatchString(ws[i].row()) {
			t.Errorf("row %s witness %q does not carry the silent-row cause", ws[i].ID, ws[i].row())
		}
		if strings.Contains(ws[i].Note, "stopped on module resolution before its check ran") {
			t.Errorf("row %s note %q asserts a module stop it did not observe", ws[i].ID, ws[i].Note)
		}
	}
	if strings.Contains(ws[0].Note, "not captured") {
		t.Errorf("row 1 named the module in its output, but its note says the output was not captured: %q", ws[0].Note)
	}

	// The unshare lane is not the image: its rows keep their own verdict, silent or not.
	// Row 6 expected its exit 1, so its own verdict is pass.
	for i, wantState := range map[int]string{0: stateFail, 4: stateFail, 5: statePass} {
		if ws := run(sandboxUnshare, []verifyRow{rows[i]}); ws[0].State != wantState {
			t.Errorf("unshare lane row %s: state %q (%s), want %q", rows[i].ID, ws[0].State, ws[0].Note, wantState)
		}
	}

	// A manifest with no gap (the cache holds what the tree wants): the silent
	// non-zero row has nothing to be blamed on and stays fail, as does a noisy one.
	full := t.TempDir()
	bakeModule(t, full, "golang.org/x/text", "v0.43.0")
	stubBakedRoots(t, "tools/desk\nstatusgen\n", full)
	ws = run(sandboxContainerNetns, rows)
	wantNoGap := []string{
		stateFail, stateFail, statePass, statePass,
		stateFail, statePass, statePass, stateFail,
		stateFail, stateFail, stateFail, statePass,
	}
	for i, w := range ws {
		if w.State != wantNoGap[i] {
			t.Errorf("no gap, row %s: state %q (%s), want %q", w.ID, w.State, w.Note, wantNoGap[i])
		}
	}

	// No manifest: a runner that is not the image checks nothing, silent or not.
	bakedModuleRootsFile = filepath.Join(t.TempDir(), "absent")
	for i, wantState := range map[int]string{0: stateFail, 4: stateFail, 5: statePass} {
		if ws := run(sandboxContainerNetns, []verifyRow{rows[i]}); ws[0].State != wantState {
			t.Errorf("no-manifest row %s: state %q (%s), want %q", rows[i].ID, ws[0].State, ws[0].Note, wantState)
		}
	}
}

// TestModGapReclassChokePoint: the one decision every container-netns row goes
// through. A non-zero row is reclassified when its output names a missing
// module or is empty; nothing else is, and a row that exits 0, is already
// could-not-run, or runs with no gap is left alone.
func TestModGapReclassChokePoint(t *testing.T) {
	gaps := []modGap{{Root: "tools/desk", Path: "golang.org/x/text", Want: "v0.43.0", Have: []string{"v0.41.0"}}}
	for _, tc := range []struct {
		name string
		res  runResult
		gaps []modGap
		want string // "" none, "named" or "silent"
	}{
		{"named", runResult{exit: 1, output: []byte(goOfflineMissing)}, gaps, "named"},
		{"silent", runResult{exit: 1}, gaps, "silent"},
		{"silent exit 2", runResult{exit: 2, output: []byte{}}, gaps, "silent"},
		{"noisy other", runResult{exit: 1, output: []byte("--- FAIL: TestX")}, gaps, ""},
		{"whitespace is output", runResult{exit: 1, output: []byte("\n")}, gaps, ""},
		{"silent exit 0", runResult{exit: 0}, gaps, ""},
		{"named exit 0", runResult{exit: 0, output: []byte(goOfflineMissing)}, gaps, ""},
		{"silent no gap", runResult{exit: 1}, nil, ""},
		{"already could-not-run", runResult{exit: 1, couldNotRun: true, reason: "x"}, gaps, ""},
	} {
		reason, ok := modGapReclass(tc.res, tc.gaps)
		got := ""
		switch {
		case ok && strings.Contains(reason, "not captured"):
			got = "silent"
		case ok:
			got = "named"
		}
		if got != tc.want {
			t.Errorf("%s: got %q (%q), want %q", tc.name, got, reason, tc.want)
		}
	}
}

// TestRunnerDockerfileWritesBakedRoots: the image writes the manifest at the
// path verifyrun reads, and lists exactly the roots whose go.mod and go.sum it
// copies in for `go mod download`.
func TestRunnerDockerfileWritesBakedRoots(t *testing.T) {
	raw, err := os.ReadFile("../images/verify-runner/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	df := string(raw)
	m := regexp.MustCompile(`printf '%s\\n' ([^>]+) > (\S+);`).FindStringSubmatch(df)
	if m == nil {
		t.Fatal("the Dockerfile writes no baked-roots manifest")
	}
	if m[2] != bakedModuleRootsFile {
		t.Errorf("the Dockerfile writes %s, verifyrun reads %s", m[2], bakedModuleRootsFile)
	}
	listed := strings.Fields(m[1])
	var copied []string
	for _, c := range regexp.MustCompile(`(?m)^COPY (\S+)/go\.mod (\S+)/go\.sum /tmp/modsrc/`).FindAllStringSubmatch(df, -1) {
		copied = append(copied, c[1])
	}
	if strings.Join(listed, " ") != strings.Join(copied, " ") {
		t.Errorf("manifest lists %v, the image downloads modules for %v", listed, copied)
	}
}
