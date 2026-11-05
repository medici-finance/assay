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
// precise cause, even when its Expect wanted that exit; a row that fails for
// its own reasons stays fail; a passing row passes. The unshare lane never
// reclassifies, and with no manifest nothing is reclassified.
func TestContainerRowStoppedOnMissingModule(t *testing.T) {
	stubIfaces(t, netnoneIfaces)
	root, cache := t.TempDir(), t.TempDir()
	writeGapFile(t, root, "tools/desk/go.mod", "module example.com/desk\n\ngo 1.27.0\n\nrequire golang.org/x/text v0.43.0\n")
	bakeModule(t, cache, "golang.org/x/text", "v0.41.0")
	writeGapFile(t, root, "goerr.txt", goOfflineMissing+"\n")
	stubBakedRoots(t, "tools/desk\nstatusgen\n", cache)

	rows := []verifyRow{
		{ID: "1", Command: "cat goerr.txt; exit 1", Expect: "exit 0"},
		{ID: "2", Command: "echo '--- FAIL: TestX'; exit 1", Expect: "exit 0"},
		{ID: "3", Command: "true", Expect: "exit 0"},
		{ID: "4", Command: "cat goerr.txt; exit 1", Expect: "exit 1"},
	}
	ws := runWitnessesSandboxed(resolveShellPlan(), sandboxContainerNetns, root, rows, "human:tester", "", "0000", "2026-10-28", 30*time.Second, false)
	want := []string{stateCouldNotRun, stateFail, statePass, stateCouldNotRun}
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

	// The unshare lane is not the image: its rows keep their own verdict.
	ws = runWitnessesSandboxed(resolveShellPlan(), sandboxUnshare, root, rows[:1], "human:tester", "", "0000", "2026-10-28", 30*time.Second, false)
	if ws[0].State != stateFail {
		t.Errorf("unshare lane row: state %q (%s), want fail", ws[0].State, ws[0].Note)
	}

	// No manifest: a runner that is not the image checks nothing.
	bakedModuleRootsFile = filepath.Join(t.TempDir(), "absent")
	ws = runWitnessesSandboxed(resolveShellPlan(), sandboxContainerNetns, root, rows[:1], "human:tester", "", "0000", "2026-10-28", 30*time.Second, false)
	if ws[0].State != stateFail {
		t.Errorf("no-manifest row: state %q (%s), want fail", ws[0].State, ws[0].Note)
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
