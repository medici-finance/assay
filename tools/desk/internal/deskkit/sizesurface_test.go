package deskkit

import (
	"reflect"
	"testing"
)

func TestSizeClassLabelBoundaries(t *testing.T) {
	cases := []struct {
		lines int
		want  string
	}{
		{0, "size:S"},
		{1, "size:S"},
		{99, "size:S"},   // just under the S/M boundary
		{100, "size:M"},  // AT the S/M boundary → M (threshold is <100 for S)
		{101, "size:M"},
		{399, "size:M"},  // just under the M/L boundary
		{400, "size:L"},  // AT the M/L boundary → L (threshold is <400 for M)
		{401, "size:L"},
		{5000, "size:L"},
	}
	for _, c := range cases {
		if got := SizeClassLabel(c.lines); got != c.want {
			t.Errorf("SizeClassLabel(%d) = %q, want %q", c.lines, got, c.want)
		}
	}
}

func TestChangedLineCountExcludesGenerated(t *testing.T) {
	files := []FileDelta{
		{Path: "tools/desk/cmd/deskpost/label.go", Changed: 60}, // counts
		{Path: "internal/deskkit/sizesurface.go", Changed: 30},  // counts
		{Path: "vendor/github.com/x/y/z.go", Changed: 5000},     // generated → excluded
		{Path: "tools/desk/go.sum", Changed: 800},               // lockfile → excluded
		{Path: "web/app.min.js", Changed: 2000},                 // minified → excluded
		{Path: "api/service.pb.go", Changed: 1200},              // protobuf → excluded
	}
	got := ChangedLineCount(files)
	if got != 90 {
		t.Fatalf("ChangedLineCount = %d, want 90 (only the two non-generated files)", got)
	}
	// And the class it drives must be S, not the L the raw churn would have produced.
	if lbl := SizeClassLabel(got); lbl != "size:S" {
		t.Fatalf("size label = %q, want size:S — generated churn must not inflate the class", lbl)
	}
}

func TestClassifySurfaceAbsentConfigIsUnknown(t *testing.T) {
	// cfgPresent=false → Unknown, no matched globs, NEVER guessed as std, even when a path
	// would obviously have matched a typical surface glob.
	state, matched := ClassifySurface(false, nil, []string{".github/workflows/ci.yml"})
	if state != SurfaceUnknown {
		t.Fatalf("state = %v, want SurfaceUnknown for absent config", state)
	}
	if matched != nil {
		t.Fatalf("matched = %v, want nil for absent config", matched)
	}
	if lbl, ok := state.Label(); ok {
		t.Fatalf("SurfaceUnknown.Label() = %q, ok=true; want no label", lbl)
	}
}

func TestClassifySurfaceCoreNamesMatchedGlobs(t *testing.T) {
	globs := []string{
		".github/workflows/**",
		".claude/guardrails/**",
		"tools/desk/cmd/*guard*/**",
		"tools/desk/hooks/**",
	}
	files := []string{
		"docs/readme.md",
		".github/workflows/leaksweep-control.yml", // matches workflows glob
		"tools/desk/cmd/writeguard/main.go",       // matches *guard* glob
	}
	state, matched := ClassifySurface(true, globs, files)
	if state != SurfaceCore {
		t.Fatalf("state = %v, want SurfaceCore", state)
	}
	want := []string{".github/workflows/**", "tools/desk/cmd/*guard*/**"}
	if !reflect.DeepEqual(matched, want) {
		t.Fatalf("matched = %v, want %v (distinct, sorted)", matched, want)
	}
	if lbl, ok := state.Label(); !ok || lbl != "surface:core" {
		t.Fatalf("Label() = %q,%v; want surface:core,true", lbl, ok)
	}
}

func TestClassifySurfaceStdWhenPresentButNoMatch(t *testing.T) {
	globs := []string{".github/workflows/**", "tools/desk/cmd/*guard*/**"}
	files := []string{"docs/a.md", "docs/b.md", "README.md"}
	state, matched := ClassifySurface(true, globs, files)
	if state != SurfaceStd {
		t.Fatalf("state = %v, want SurfaceStd", state)
	}
	if matched != nil {
		t.Fatalf("matched = %v, want nil", matched)
	}
	if lbl, ok := state.Label(); !ok || lbl != "surface:std" {
		t.Fatalf("Label() = %q,%v; want surface:std,true", lbl, ok)
	}
}

func TestClassifySurfaceSkipsBlankPaths(t *testing.T) {
	// A malformed listing (blank entries) degrades to std, not a fail-closed core.
	globs := []string{".github/workflows/**"}
	state, _ := ClassifySurface(true, globs, []string{"", "  ", "docs/x.md"})
	if state != SurfaceStd {
		t.Fatalf("state = %v, want SurfaceStd for blank+docs paths", state)
	}
}

func TestParseSurfaceGlobs(t *testing.T) {
	data := []byte(`# .assay-surfaces — declared risk-surface globs
# comment line

.github/workflows/**
   tools/desk/cmd/*guard*/**

.claude/guardrails/**
`)
	got := ParseSurfaceGlobs(data)
	want := []string{
		".github/workflows/**",
		"tools/desk/cmd/*guard*/**",
		".claude/guardrails/**",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseSurfaceGlobs = %v, want %v", got, want)
	}
}

func TestParseSurfaceGlobsEmptyIsNoGlobs(t *testing.T) {
	// An empty-but-present file parses to zero globs; ClassifySurface(true, ...) then yields
	// SurfaceStd (present, nothing matched), NOT SurfaceUnknown — the presence of the file
	// is the signal, and it is the caller (not the parser) that tracks presence.
	got := ParseSurfaceGlobs([]byte("# only comments\n\n"))
	if len(got) != 0 {
		t.Fatalf("ParseSurfaceGlobs(comments only) = %v, want empty", got)
	}
	state, _ := ClassifySurface(true, got, []string{".github/workflows/ci.yml"})
	if state != SurfaceStd {
		t.Fatalf("state = %v, want SurfaceStd for present-but-empty config", state)
	}
}

func TestMatchSurfaceGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		// trailing ** matches any depth, including the directory's direct children
		{".github/workflows/**", ".github/workflows/ci.yml", true},
		{".github/workflows/**", ".github/workflows/nested/x.yml", true},
		{".github/workflows/**", ".github/other.yml", false},
		// ** matches zero segments too
		{"**/go.sum", "go.sum", true},
		{"**/go.sum", "tools/desk/go.sum", true},
		{"**/go.sum", "go.summary", false},
		// in-segment wildcard
		{"tools/desk/cmd/*guard*/**", "tools/desk/cmd/writeguard/main.go", true},
		{"tools/desk/cmd/*guard*/**", "tools/desk/cmd/deskpushguard/x.go", true},
		{"tools/desk/cmd/*guard*/**", "tools/desk/cmd/deskpost/x.go", false},
		// leading **
		{"**/vendor/**", "a/b/vendor/c/d.go", true},
		{"**/vendor/**", "vendor/c/d.go", true},
		{"**/vendor/**", "src/main.go", false},
		// literal exact
		{"README.md", "README.md", true},
		{"README.md", "docs/README.md", false},
	}
	for _, c := range cases {
		if got := MatchSurfaceGlob(c.pattern, c.path); got != c.want {
			t.Errorf("MatchSurfaceGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
