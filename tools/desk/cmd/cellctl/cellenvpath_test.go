package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// loadLine writes one `KEY=<rhs>` line and reads it back through the loader for goos.
func loadLine(t *testing.T, goos, key, rhs string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cell.env")
	if err := os.WriteFile(p, []byte(key+"="+rhs+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	if err := parseCellEnvFor(goos, e, p); err != nil {
		t.Fatal(err)
	}
	return e.Get(key)
}

// TestWinPathRoundTrip: a native `C:\…` path handed to `cellctl new` is written the way the
// scaffold writes it, loaded back, and comes out as the SAME path — still absolute, still
// accepted by the CELL_ROOTS path check. The written form also loads unchanged under bash rules
// (goos linux), which is what a Git Bash `source cell.env` would see.
func TestWinPathRoundTrip(t *testing.T) {
	for _, in := range []string{
		`C:\src\assay-tests\Test`,
		`C:\Users\op\.local\share\assay\cells\test`,
		`d:\a`,
		`C:/already/forward`,
		`C:\mixed/seps\x`,
	} {
		want := strings.ReplaceAll(in, `\`, "/")
		repoLine := cellEnvPathFor("windows", in)
		rootsLine := cellEnvRootsFor("windows", "example-org/example-repo="+in)
		for _, reader := range []string{"windows", "linux"} {
			if got := loadLine(t, reader, "CELL_REPO", repoLine); got != want {
				t.Errorf("CELL_REPO %q (read as %s) = %q, want %q", in, reader, got, want)
			}
			got := loadLine(t, reader, "CELL_ROOTS", rootsLine)
			if got != "example-org/example-repo="+want {
				t.Errorf("CELL_ROOTS %q (read as %s) = %q", in, reader, got)
			}
		}
		if !deskkit.IsAbsFor("windows", want) || cellPathCheck("windows", want) != nil {
			t.Errorf("%q round-tripped to %q, which is no longer an absolute windows path", in, want)
		}
		if strings.ReplaceAll(want, "/", `\`) != strings.ReplaceAll(in, "/", `\`) {
			t.Errorf("%q round-tripped to a different path %q", in, want)
		}
	}
}

// TestCellEnvBackslashRules pins how the loader reads a `\` in each quoting form, per goos.
// The linux rows ARE bash: they pin the defect shape (`C:srcx`) the scaffold now avoids.
func TestCellEnvBackslashRules(t *testing.T) {
	quoted := bashQuote(`C:\Program Files\x.exe`)
	for _, tc := range []struct{ goos, rhs, want string }{
		{"linux", `C:\src\x`, `C:srcx`},
		{"windows", `C:\src\x`, `C:\src\x`},
		{"linux", `"C:\src\x"`, `C:\src\x`},
		{"windows", `"C:\src\x"`, `C:\src\x`},
		{"linux", `'C:\src\x'`, `C:\src\x`},
		{"windows", `'C:\src\x'`, `C:\src\x`},
		{"linux", `C:\\src\\x`, `C:\src\x`},
		{"windows", `C:\\src\\x`, `C:\src\x`},
		{"linux", quoted, `C:\Program Files\x.exe`},
		{"windows", quoted, `C:\Program Files\x.exe`},
		{"linux", `C:\`, `C:`},
		{"windows", `C:\`, `C:\`},
		{"windows", `a\$HOME`, `a$HOME`},
		{"windows", `a\ b`, `a b`},
		{"windows", `\\srv\share`, `\srv\share`},
		{"windows", `C:\~x\#y\_z\9\é`, `C:\~x\#y\_z\9\é`},
		{"windows", `\~x`, `~x`},
		{"windows", `C:\{guid}`, `C:{guid}`},
		{"windows", `'C:\{guid}'`, `C:\{guid}`},
		{"windows", `C:/{guid}`, `C:/{guid}`},
	} {
		if got := loadLine(t, tc.goos, "K", tc.rhs); got != tc.want {
			t.Errorf("%s: K=%s loads as %q, want %q", tc.goos, tc.rhs, got, tc.want)
		}
	}
}

// TestCellEnvBashQuoteRoundTrip: whatever this package's own %q writer (bashQuote) emits loads
// back as the value it quoted, on Windows as on bash — the Windows separator rule may only keep a
// `\` the writer never produces as an escape. Covers every printable ASCII byte mid-value, the
// word-initial `~` and `#` bashQuote escapes, and the container-scaffold shapes that carry them.
func TestCellEnvBashQuoteRoundTrip(t *testing.T) {
	vals := []string{
		`a/b=C:/x,c/d=D:/y`,
		`C:\Program Files (x86)\tool\launcher.exe`,
		`C:/Program Files (x86)/tool/launcher.exe`,
		`~/cells/x`,
		`#not-a-comment`,
		`C:\x\`,
		`\\srv\share`,
	}
	for c := byte(0x20); c < 0x7f; c++ {
		vals = append(vals, "a"+string(c)+"b", `C:\`+string(c)+`x`)
	}
	for _, v := range vals {
		w := bashQuote(v)
		for _, goos := range []string{"windows", "linux"} {
			if got := loadLine(t, goos, "K", w); got != v {
				t.Errorf("%s: bashQuote(%q) = %s loads as %q", goos, v, w, got)
			}
		}
	}
}

// TestCellEnvRootsFor: only the path half of each entry is rewritten, separators survive, a
// network path stays refused, and every non-windows goos is the identity.
func TestCellEnvRootsFor(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`a/b=C:\x,c/d=D:\y`, `a/b=C:/x,c/d=D:/y`},
		{`a/b=C:\x c/d=D:\y`, `a/b=C:/x c/d=D:/y`},
		{`owner\repo=C:\x`, `owner\repo=C:/x`},
		{`a/b=C:\x=y`, `a/b=C:/x=y`},
		{"", ""},
	} {
		if got := cellEnvRootsFor("windows", tc.in); got != tc.want {
			t.Errorf("cellEnvRootsFor(windows, %q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	for _, unc := range []string{`\\srv\share\a`, `\\?\C:\a`} {
		if cellPathCheck("windows", cellEnvPathFor("windows", unc)) == nil {
			t.Errorf("%q became an accepted path after normalising", unc)
		}
	}
	for _, goos := range []string{"linux", "darwin"} {
		in := `a/b=/x\y`
		if cellEnvRootsFor(goos, in) != in || cellEnvPathFor(goos, `/x\y`) != `/x\y` {
			t.Errorf("%s: the cell.env path helpers must be the identity", goos)
		}
	}
}

// TestNewScrubbedRoundTrip runs the REAL scaffold (cmdNew) and loads what it wrote: CELL_REPO
// and CELL_ROOTS come back as the paths it was given. On a Windows host t.TempDir() is a native
// `C:\…` path, so there this is the end-to-end check of both halves.
func TestNewScrubbedRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	t.Setenv("CELLS_ROOT", t.TempDir())
	if code := inproc("new", "rt", "--kind", "scrubbed", "--repo", repo, "--repo-slug", "o/r", "--roots", "o/r="+repo); code != 0 {
		t.Fatalf("new exited %d", code)
	}
	envfile := filepath.Join(os.Getenv("CELLS_ROOT"), "rt", "cell.env")
	raw, err := os.ReadFile(envfile)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && strings.Contains(string(raw), `\`) {
		t.Errorf("cell.env carries a backslash on windows:\n%s", raw)
	}
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	if err := parseCellEnv(e, envfile); err != nil {
		t.Fatal(err)
	}
	if got := e.Get("CELL_REPO"); filepath.Clean(got) != filepath.Clean(repo) {
		t.Errorf("CELL_REPO = %q, want %q", got, repo)
	}
	roots := rootEntries(e.Get("CELL_ROOTS"))
	if len(roots) != 1 || roots[0][0] != "o/r" || filepath.Clean(roots[0][1]) != filepath.Clean(repo) {
		t.Errorf("CELL_ROOTS = %q, want o/r=%s", e.Get("CELL_ROOTS"), repo)
	}
}

// TestKindChangeReadsLoaderValue: `cellctl set CELL_KIND=container` judges the launcher the
// file already names by what loadCell will read — here a %q-escaped path with a space, which a
// raw file-level read sees as `my\ launcher` and wrongly refuses.
func TestKindChangeReadsLoaderValue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs an executable-bit launcher")
	}
	dir := t.TempDir()
	launcher := filepath.Join(dir, "my launcher")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "cell.env")
	if err := os.WriteFile(p, []byte("CELL=demo\nCELL_CONTAINER_LAUNCHER="+bashQuote(launcher)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	validateKindChange(p, "container", []string{"CELL_KIND=container"})
}

// cellEnvReaders inventories every call that turns cell.env TEXT into a value: the unquote
// functions and the raw line reader. Keyed `callee@file:func`.
func cellEnvReaders(name string, src any) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	callees := map[string]bool{"unquoteShellValueFor": true, "unquoteShellValue": true, "envFileValue": true}
	var sites []string
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := c.Fun.(*ast.Ident); ok && callees[id.Name] {
				sites = append(sites, id.Name+"@"+filepath.Base(name)+":"+fd.Name.Name)
			}
			return true
		})
	}
	return sites, nil
}

// cellEnvReadersAllowed is the closed set. A cell.env value that decides anything (a path
// check, a kind precondition, a launch) is read through the loader; envFileValue answers
// "is the key in the file" for `show` and nothing else.
var cellEnvReadersAllowed = map[string]string{
	"unquoteShellValueFor@cell.go:parseCellEnvFor":   "the loader",
	"unquoteShellValueFor@cell.go:unquoteShellValue": "host-goos wrapper",
	"unquoteShellValue@set.go:effectiveCellEnv":      "set's own KEY=VALUE overlay, read as the loader will",
	"envFileValue@show.go:cmdShow":                   "presence/display only",
}

// TestCellEnvValueReaders is the class guard: a cell.env value interpreted anywhere but the
// loader reads a Windows path (or a %q-quoted one) differently from loadCell.
func TestCellEnvValueReaders(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		sites, err := cellEnvReaders(f.Name(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range sites {
			seen[s] = true
			if _, ok := cellEnvReadersAllowed[s]; !ok {
				t.Errorf("cell.env value read outside the loader: %s — read it through parseCellEnv/effectiveCellEnv", s)
			}
		}
	}
	var missing []string
	for s := range cellEnvReadersAllowed {
		if !seen[s] {
			missing = append(missing, s)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("allow-listed reader not found (stale list or broken matcher): %v", missing)
	}
	planted := `package main
func validateRootsRaw(p string) { v, _ := envFileValue(p, "CELL_ROOTS"); _ = v }`
	sites, err := cellEnvReaders("planted.go", planted)
	if err != nil || len(sites) != 1 || sites[0] != "envFileValue@planted.go:validateRootsRaw" {
		t.Fatalf("positive control: %v %v", sites, err)
	}
}
