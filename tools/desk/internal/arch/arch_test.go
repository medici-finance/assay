package arch_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/arch"
)

// registerHint closes every real-tree failure: the rule's row in the register explains
// why the rule exists and what a change to it needs.
const registerHint = "see the row of that id in docs/contracts.md's rule register"

// --- the real tree -------------------------------------------------------------------

// TestDependencyDirection: no internal/ package imports a command and no command imports
// another command (R-dep-direction). It relies on the compiler for import cycles and for
// cross-tree internal/ imports and does not re-check either. Ceiling 0: nothing is
// grandfathered.
func TestDependencyDirection(t *testing.T) {
	g := realGraph(t)
	failAll(t, arch.Direction(g))
}

// hubLanding is hub-allow.txt as it landed. Every entry added after landing carries a
// "# grow <url>" line directly above it; the test checks the line is there, and review
// checks that the url is the driver's grow reply.
var hubLanding = map[string]bool{"gitcore": true, "topology": true}

// TestHubAllowList: internal/deskkit imports exactly the internal/ packages on
// hub-allow.txt — nothing unlisted, and nothing listed that it no longer imports — and
// every entry past the landing set carries its "# grow" line (R-hub-allowlist).
func TestHubAllowList(t *testing.T) {
	g := realGraph(t)
	allow := readList(t, "hub-allow.txt")
	failAll(t, arch.HubAllow(g, arch.Hub, allow))
	raw, err := os.ReadFile("hub-allow.txt")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	for _, a := range allow {
		if !hubLanding[a] && !arch.GrowAnnotated(lines, a) {
			t.Errorf("%s: hub-allow.txt adds %s with no \"# grow <url>\" line directly above it (%s)", arch.RuleHubAllow, a, registerHint)
		}
	}
}

// TestOneImplementationPerMeaning: every declared implementation of a registered meaning
// sits in its owner or a listed duplicate, the count per meaning stays under markers.txt's
// ceiling, and an owner inside tools/desk declares itself (R-one-implementation). In a
// checkout of tools/desk alone the semantic index is absent, and the test says
// could-not-check rather than passing.
func TestOneImplementationPerMeaning(t *testing.T) {
	modRoot := moduleRoot(t)
	repo, ok := repoRoot(modRoot)
	if !ok {
		t.Skipf("could-not-check (no semantic index): %s is not tools/desk inside a repository carrying docs/contracts.md", modRoot)
	}
	f, err := os.Open(filepath.Join(repo, "docs", "contracts.md"))
	if err != nil {
		t.Fatalf("docs/contracts.md: %v", err)
	}
	defer f.Close()
	index, err := arch.ParseIndex(f)
	if err != nil {
		t.Fatalf("docs/contracts.md: %v", err)
	}
	fsys := os.DirFS(repo)
	ms, vs, err := arch.Markers(fsys, "tools/desk", index)
	if err != nil {
		t.Fatalf("Markers: %v", err)
	}
	cf, err := os.Open("markers.txt")
	if err != nil {
		t.Fatalf("markers.txt: %v", err)
	}
	defer cf.Close()
	ceilings, err := arch.ParseCeilings(cf)
	if err != nil {
		t.Fatalf("markers.txt: %v", err)
	}
	rv, skipped := arch.Ratchet(ms, index, ceilings, "tools/desk")
	t.Logf("markers=%d over %d index rows; owner half could-not-check (owner outside tools/desk or undeclared): %s",
		len(ms), len(index), strings.Join(skipped, " "))
	for _, r := range index {
		if n, c := countOf(ms, r.ID), ceilings[r.ID]; n < c {
			t.Logf("NOTE: %s has %d markers under its ceiling of %d; lower markers.txt to lock in the gain", r.ID, n, c)
		}
	}
	failAll(t, append(vs, rv...))
}

// TestMissingIndexIsCouldNotCheck: a module root that is not tools/desk inside a
// repository resolves to no index, so rule 3 skips as could-not-check instead of walking
// an empty index and passing (the three-state rule).
func TestMissingIndexIsCouldNotCheck(t *testing.T) {
	d := t.TempDir()
	mod := filepath.Join(d, "desk")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := repoRoot(mod); ok {
		t.Fatalf("a bare copy at %s resolved to a repository root; the could-not-check branch would never fire", mod)
	}
	// tools/desk inside a directory that has no docs/contracts.md is also could-not-check.
	mod = filepath.Join(d, "tools", "desk")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := repoRoot(mod); ok {
		t.Fatalf("%s resolved to a repository root with no docs/contracts.md", mod)
	}
	// And the positive control: with the index present it resolves.
	if err := os.MkdirAll(filepath.Join(d, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "docs", "contracts.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r, ok := repoRoot(mod); !ok || r != d {
		t.Fatalf("repoRoot(%s) = %q, %v; want %q, true", mod, r, ok, d)
	}
	// An index that parses to nothing is an error, never a clean pass.
	if _, err := arch.ParseIndex(strings.NewReader("# contracts\n\n## Semantic owners\n\nno table\n")); err == nil {
		t.Fatal("ParseIndex accepted an index with no S- rows")
	}
	if _, err := arch.ParseIndex(strings.NewReader("# contracts\n")); err == nil {
		t.Fatal("ParseIndex accepted a file with no Semantic owners section")
	}
}

// TestReportCloneDensity reports clone density when dupl is on PATH, and could-not-check
// otherwise. Reported, never ratcheted: it never fails.
func TestReportCloneDensity(t *testing.T) {
	if _, err := exec.LookPath("dupl"); err != nil {
		t.Log("clones: could-not-check (no dupl)")
		return
	}
	modRoot := moduleRoot(t)
	var files []string
	_ = filepath.WalkDir(modRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dupl", "-t", "60", "-files")
	cmd.Stdin = strings.NewReader(strings.Join(files, "\n") + "\n")
	out, err := cmd.CombinedOutput()
	m := regexp.MustCompile(`Found total ([0-9]+) clone groups`).FindSubmatch(out)
	if err != nil || m == nil {
		t.Logf("clones: could-not-check (dupl run unreadable: %v)", err)
		return
	}
	t.Logf("clones: %s groups at dupl -t 60 (reported, not ratcheted)", m[1])
}

// --- the fixture ---------------------------------------------------------------------

// TestRulesFixture runs every rule over testdata/tree, a fixture repository whose
// tools/desk module carries one violation of each shape and four decoys: a _test.go
// importing a command (ignored), a marker in a listed duplicate (allowed), a marker in a
// listed package directory (allowed) and a cmd/x/internal import (allowed).
// internal/clean is the clean control.
func TestRulesFixture(t *testing.T) {
	fsys := fixtureFS(t)
	const mod = "tools/desk"
	g, err := arch.Imports(fsys, mod)
	if err != nil {
		t.Fatalf("Imports: %v", err)
	}

	t.Run("direction", func(t *testing.T) {
		want := []string{
			"R-dep-direction: tools/desk/cmd/beta/main.go:4: cmd/beta imports cmd/alpha: a command never imports another command; move the shared code under internal/",
			"R-dep-direction: tools/desk/internal/leaky/leaky.go:4: internal/leaky imports cmd/alpha: a package under internal/ never imports a command",
		}
		assertSet(t, arch.Direction(g), want)
	})

	t.Run("hub", func(t *testing.T) {
		allow := readListAt(t, "testdata/tree/tools/desk/internal/arch/hub-allow.txt")
		want := []string{
			"R-hub-allowlist: hub-allow.txt lists topology but internal/deskkit no longer imports internal/topology; remove the line to lock in the gain",
			"R-hub-allowlist: tools/desk/internal/deskkit/hub.go:5: internal/deskkit imports internal/extra, which is not on hub-allow.txt; everything the hub imports is a dependency of every package that imports it",
		}
		assertSet(t, arch.HubAllow(g, arch.Hub, allow), want)
		if vs := arch.HubAllow(g, "internal/nosuchhub", nil); len(vs) != 1 {
			t.Errorf("a missing hub must be one violation, got %v", vs)
		}
	})

	index := fixtureIndex(t)
	ms, vs, err := arch.Markers(fsys, mod, index)
	if err != nil {
		t.Fatalf("Markers: %v", err)
	}

	// S-wide's cells name the walked root, its cmd/ and internal/ in prose and list one
	// package directory. Only that package's own files are sites: the marker in its
	// subpackage and the marker reachable only through the prose spans are both red.
	t.Run("markers", func(t *testing.T) {
		want := []string{
			"R-one-implementation: tools/desk/internal/stray/stray.go:4: S-thing implemented outside its owner at tools/desk/internal/stray/stray.go:4; add it to the row's duplicates with a design-fit finding, or move it to the owner",
			"R-one-implementation: tools/desk/internal/pkg/sub/sub.go:4: S-wide implemented outside its owner at tools/desk/internal/pkg/sub/sub.go:4; add it to the row's duplicates with a design-fit finding, or move it to the owner",
			"R-one-implementation: tools/desk/internal/wide/wide.go:4: S-wide implemented outside its owner at tools/desk/internal/wide/wide.go:4; add it to the row's duplicates with a design-fit finding, or move it to the owner",
		}
		assertSet(t, vs, want)
		if len(ms) != 6 {
			t.Errorf("markers = %v, want 6 (S-thing: owner, listed duplicate, stray; S-wide: listed package, subpackage, prose-only)", ms)
		}
	})

	t.Run("ratchet-at-ceiling", func(t *testing.T) {
		rv, skipped := arch.Ratchet(ms, index, map[string]int{"S-thing": 3, "S-wide": 3}, mod)
		assertSet(t, rv, nil)
		if !reflect.DeepEqual(skipped, []string{"S-far", "S-wide"}) {
			t.Errorf("skipped = %v, want [S-far S-wide] (their owners are outside the walked tree)", skipped)
		}
	})

	t.Run("ratchet-over-ceiling", func(t *testing.T) {
		rv, _ := arch.Ratchet(ms, index, map[string]int{"S-thing": 2, "S-gone": 1}, mod)
		assertSet(t, rv, []string{
			"R-one-implementation: S-thing has 3 declared implementations, over its markers.txt ceiling of 2",
			"R-one-implementation: S-wide has 3 declared implementations, over its markers.txt ceiling of 0",
			"R-one-implementation: markers.txt has a ceiling for S-gone, which is not a row of the semantic index",
		})
	})

	// An owner cell that names only the walked root or its containers declares no
	// in-scope owner, so the owner half skips it rather than reading the tree as its owner.
	t.Run("ratchet-container-owner", func(t *testing.T) {
		wide := []arch.SRow{{ID: "S-wide", Owners: []string{"tools/desk", "tools/desk/cmd", "tools/desk/internal", "tools"}}}
		rv, skipped := arch.Ratchet(nil, wide, nil, mod)
		assertSet(t, rv, nil)
		if !reflect.DeepEqual(skipped, []string{"S-wide"}) {
			t.Errorf("skipped = %v, want [S-wide]", skipped)
		}
	})

	t.Run("grow-annotation", func(t *testing.T) {
		lines := []string{
			"# header",
			"gitcore",
			"# grow https://example.org/issues/1#c2",
			"# why this one",
			"forgeban",
			"# grow https://example.org/issues/1#c3",
			"",
			"blanked",
			"bare",
		}
		for entry, want := range map[string]bool{"gitcore": false, "forgeban": true, "blanked": false, "bare": false, "absent": false} {
			if got := arch.GrowAnnotated(lines, entry); got != want {
				t.Errorf("GrowAnnotated(%s) = %v, want %v", entry, got, want)
			}
		}
	})

	t.Run("ratchet-owner-undeclared", func(t *testing.T) {
		var notOwner []arch.Marker
		for _, m := range ms {
			if !strings.Contains(m.File, "/owner/") {
				notOwner = append(notOwner, m)
			}
		}
		rv, _ := arch.Ratchet(notOwner, index, map[string]int{"S-thing": 3, "S-wide": 3}, mod)
		assertSet(t, rv, []string{
			"R-one-implementation: S-thing's owner (tools/desk/internal/owner/owner.go) declares no implementation; mark the function that computes the meaning",
		})
	})

	t.Run("marker-placement", func(t *testing.T) {
		mfs := fstest.MapFS{
			"m/a.go": {Data: []byte("package m\n\n// semantic: S-thing\nvar x = 1\n\n// semantic: S-nope\nfunc F() {}\n")},
		}
		_, pv, err := arch.Markers(mfs, "m", index)
		if err != nil {
			t.Fatal(err)
		}
		assertSet(t, pv, []string{
			"R-one-implementation: m/a.go:3: marker for S-thing is not directly above a func declaration",
			"R-one-implementation: m/a.go:6: marker names S-nope, which is not a row of the semantic index",
		})
	})

	t.Run("index", func(t *testing.T) {
		want := []arch.SRow{
			{ID: "S-thing", Owners: []string{"tools/desk/internal/owner/owner.go"}, Duplicates: []string{"tools/desk/internal/dup/dup.go"}},
			{ID: "S-far", Owners: []string{"statusgen/far.go"}},
			{ID: "S-wide", Owners: []string{"statusgen/wide.go"}, Duplicates: []string{"tools/desk/internal/pkg", "tools/desk", "tools/desk/cmd", "tools/desk/internal"}},
		}
		if !reflect.DeepEqual(index, want) {
			t.Errorf("ParseIndex = %+v, want %+v", index, want)
		}
	})
}

// --- helpers -------------------------------------------------------------------------

// fixtureFS serves testdata/tree with its tools/desk/go.mod added in memory. The fixture
// deliberately carries no go.mod on disk: CI builds, vets and tests every tracked go.mod
// as a module, and the fixture's planted violations are not meant to compile.
func fixtureFS(t *testing.T) fs.FS {
	t.Helper()
	m := fstest.MapFS{
		"tools/desk/go.mod": {Data: []byte("module example.com/fixture/tools/desk\n\ngo 1.25.0\n")},
	}
	src := os.DirFS("testdata/tree")
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(src, p)
		if rerr != nil {
			return rerr
		}
		m[p] = &fstest.MapFile{Data: b}
		return nil
	})
	if err != nil {
		t.Fatalf("reading testdata/tree: %v", err)
	}
	return m
}

func fixtureIndex(t *testing.T) []arch.SRow {
	t.Helper()
	f, err := os.Open("testdata/tree/docs/contracts.md")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	index, err := arch.ParseIndex(f)
	if err != nil {
		t.Fatalf("ParseIndex(fixture): %v", err)
	}
	return index
}

// realGraph reads the import graph of the tools/desk module this test runs in. Inside the
// repository the file paths it reports are repository-relative (tools/desk/...), in a bare
// copy module-relative.
func realGraph(t *testing.T) arch.Graph {
	t.Helper()
	modRoot := moduleRoot(t)
	fsys, dir := os.DirFS(modRoot), "."
	if repo, ok := repoRoot(modRoot); ok {
		fsys, dir = os.DirFS(repo), "tools/desk"
	}
	g, err := arch.Imports(fsys, dir)
	if err != nil {
		t.Fatalf("Imports: %v", err)
	}
	return g
}

// moduleRoot walks up from the package directory (the test's working directory) to the
// nearest go.mod: the tools/desk module root.
func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", wd)
		}
		dir = parent
	}
}

// repoRoot returns the repository root when modRoot is <repo>/tools/desk and
// <repo>/docs/contracts.md exists; ok is false otherwise (a consumer copy of tools/desk).
func repoRoot(modRoot string) (string, bool) {
	if filepath.Base(modRoot) != "desk" || filepath.Base(filepath.Dir(modRoot)) != "tools" {
		return "", false
	}
	repo := filepath.Dir(filepath.Dir(modRoot))
	if _, err := os.Stat(filepath.Join(repo, "docs", "contracts.md")); err != nil {
		return "", false
	}
	return repo, true
}

func readList(t *testing.T, name string) []string {
	t.Helper()
	return readListAt(t, name)
}

func readListAt(t *testing.T, p string) []string {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	l, err := arch.ParseList(f)
	if err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	return l
}

func countOf(ms []arch.Marker, id string) int {
	n := 0
	for _, m := range ms {
		if m.ID == id {
			n++
		}
	}
	return n
}

// failAll reports every violation, each naming its rule, file and register row.
func failAll(t *testing.T, vs []arch.Violation) {
	t.Helper()
	for _, v := range vs {
		t.Errorf("%s (%s)", v, registerHint)
	}
}

func assertSet(t *testing.T, got []arch.Violation, want []string) {
	t.Helper()
	gs := make([]string, 0, len(got))
	for _, v := range got {
		gs = append(gs, v.String())
	}
	sort.Strings(gs)
	w := append([]string(nil), want...)
	sort.Strings(w)
	if len(gs) == 0 && len(w) == 0 {
		return
	}
	if !reflect.DeepEqual(gs, w) {
		t.Errorf("violations:\n  got  %q\n  want %q", gs, w)
	}
}
