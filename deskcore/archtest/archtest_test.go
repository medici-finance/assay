package archtest

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/config"
)

const module = "github.com/medici-finance/assay/deskcore"

// deskcorePolicy is the rule this module lives by.
func deskcorePolicy(mod string, pure ...string) Policy {
	return Policy{
		Module:            mod,
		Pure:              pure,
		ForbiddenStd:      []string{"os/exec", "net", "net/http/..."},
		ForbiddenInternal: []string{"ports", "collect", "adapters", "custody", "identity", "runtime", "witness"},
	}
}

// goList runs "go list -e -json -deps" in dir, offline: no proxy, no workspace file.
func goList(t *testing.T, dir string, pkgs ...string) []Package {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list", "-e", "-json", "-deps"}, pkgs...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOWORK=off", "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list in %s: %v\n%s", dir, err, stderr.String())
	}
	got, err := Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// fixture copies testdata/<name> to a temporary directory, dropping the ".fixture" suffix the
// files carry so that neither the go tool nor a repository-wide grep mistakes them for real
// source.
func fixture(t *testing.T, name string) string {
	t.Helper()
	dst := t.TempDir()
	src := filepath.Join("testdata", name)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, strings.TrimSuffix(rel, ".fixture"))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

var purePackages = []string{module + "/domain", module + "/facts", module + "/policy", module + "/config"}

// TestPurePackagesHaveNoEffectfulTransitiveImports: domain, facts, policy and config reach no
// process, network or HTTP package, no effectful module package and no third-party package, by
// any transitive path.
func TestPurePackagesHaveNoEffectfulTransitiveImports(t *testing.T) {
	pkgs := goList(t, "..", "./domain", "./facts", "./policy", "./config")
	for _, v := range Check(pkgs, deskcorePolicy(module, purePackages...)) {
		t.Errorf("%s", v)
	}
	// The import direction: domain is the leaf; facts, policy and config build on it only.
	imports := map[string][]string{}
	for _, p := range pkgs {
		imports[p.ImportPath] = p.Imports
	}
	for _, p := range purePackages {
		for _, imp := range imports[p] {
			if strings.HasPrefix(imp, module+"/") && imp != module+"/domain" {
				t.Errorf("%s imports %s; pure packages may import only %s/domain from this module", p, imp, module)
			}
			if p == module+"/domain" && strings.HasPrefix(imp, module) {
				t.Errorf("domain imports %s; domain is the leaf", imp)
			}
		}
	}
}

// TestArchtestCatchesPlantedImport is the positive control: a fixture module plants each kind of
// forbidden reach, and Check reports every one with its chain while passing a clean package.
func TestArchtestCatchesPlantedImport(t *testing.T) {
	const mod = "example.com/planted"
	dir := fixture(t, "planted")
	pkgs := goList(t, dir, "./...")
	got := Check(pkgs, deskcorePolicy(mod, mod+"/domain", mod+"/facts", mod+"/policy", mod+"/clean"))
	want := map[string]string{
		mod + "/domain": mod + "/domain -> " + mod + "/mid -> os/exec",
		mod + "/facts":  mod + "/facts -> " + mod + "/witness",
		mod + "/policy": mod + "/policy -> net/http/httptest",
	}
	seen := map[string]bool{}
	for _, v := range got {
		c := strings.Join(v.Chain, " -> ")
		if want[v.Pure] != c {
			t.Errorf("unexpected violation %s", v)
			continue
		}
		seen[v.Pure] = true
	}
	for p, c := range want {
		if !seen[p] {
			t.Errorf("planted reach not caught: %s", c)
		}
	}
}

func TestArchtestCatchesCycleAndLoadErrors(t *testing.T) {
	const mod = "example.com/cycle"
	dir := fixture(t, "cycle")
	got := Check(goList(t, dir, "./..."), deskcorePolicy(mod, mod+"/a"))
	if len(got) == 0 {
		t.Fatal("an import cycle passed the check")
	}
	// A graph assembled by hand, which the go tool never vetted.
	pkgs := []Package{
		{ImportPath: "m/a", Imports: []string{"m/b"}},
		{ImportPath: "m/b", Imports: []string{"m/a"}},
	}
	got = Check(pkgs, deskcorePolicy("m", "m/a"))
	if len(got) != 1 || got[0].Why != "import cycle" || strings.Join(got[0].Chain, " ") != "m/a m/b m/a" {
		t.Errorf("hand-built cycle: %v", got)
	}
	if got := Check(nil, deskcorePolicy("m", "m/missing")); len(got) != 1 {
		t.Errorf("a missing pure package passed: %v", got)
	}
}

func TestArchtestCatchesThirdParty(t *testing.T) {
	pkgs := []Package{
		{ImportPath: "m/a", Imports: []string{"github.com/other/lib", "strings", "runtime"}},
		{ImportPath: "github.com/other/lib", Module: &struct{ Path string }{"github.com/other/lib"}},
		{ImportPath: "strings", Standard: true},
		{ImportPath: "runtime", Standard: true},
	}
	got := Check(pkgs, deskcorePolicy("m", "m/a"))
	if len(got) != 1 || got[0].Bad != "github.com/other/lib" {
		t.Errorf("third-party reach: %v", got)
	}
}

func knobs() []Knob {
	var out []Knob
	for _, k := range config.Table() {
		out = append(out, Knob{Name: k.Name, EnvName: k.EnvName(), V1: k.V1})
	}
	return out
}

// TestNoKnobReadsOutsideConfig: outside the config package, no deskcore source reads the
// process environment or hard-codes a knob's earlier literal.
func TestNoKnobReadsOutsideConfig(t *testing.T) {
	found, err := ScanKnobReads(os.DirFS(".."), knobs(), []string{"config"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("%s", f)
	}
}

// TestKnobReadLintCatchesStrayRead is the positive control for the lint.
func TestKnobReadLintCatchesStrayRead(t *testing.T) {
	found, err := ScanKnobReads(os.DirFS(fixture(t, "knobread")), knobs(), []string{"config"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"stray/stray.go":     "os.Getenv",
		"alias/alias.go":     "os.LookupEnv",
		"literal/literal.go": "write.rate.per_pr_hour",
		"dot/dot.go":         "dot-import of os",
		"sc/sc.go":           "syscall.Getenv",
	}
	if len(found) != len(want) {
		t.Errorf("got %d findings, want %d: %v", len(found), len(want), found)
	}
	for _, f := range found {
		if w, ok := want[f.File]; !ok || !strings.Contains(f.What, w) {
			t.Errorf("unexpected finding %s", f)
		}
		delete(want, f.File)
	}
	for file, w := range want {
		t.Errorf("missed %s in %s", w, file)
	}
}
