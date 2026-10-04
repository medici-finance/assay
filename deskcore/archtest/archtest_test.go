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
		ForbiddenDirect:   []string{"os", "syscall", "os/signal", "os/user", "io/ioutil", "plugin", "unsafe"},
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
// any transitive path, and import no effect-capable standard package (os, syscall and the rest
// of ForbiddenDirect) themselves.
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
	got := Check(pkgs, deskcorePolicy(mod, mod+"/domain", mod+"/facts", mod+"/policy", mod+"/clean",
		mod+"/direct", mod+"/viahelper"))
	want := map[string]bool{
		mod + "/domain -> " + mod + "/mid -> os/exec": true,
		mod + "/facts -> " + mod + "/witness":         true,
		mod + "/policy -> net/http/httptest":          true,
		// os.StartProcess and syscall.Exec, with no os/exec import anywhere.
		mod + "/direct -> os":      true,
		mod + "/direct -> syscall": true,
		// A clean pure package whose module helper writes a file.
		mod + "/viahelper -> " + mod + "/helper -> os": true,
	}
	for _, v := range got {
		c := strings.Join(v.Chain, " -> ")
		if !want[c] {
			t.Errorf("unexpected violation %s", v)
			continue
		}
		delete(want, c)
	}
	for c := range want {
		t.Errorf("planted reach not caught: %s", c)
	}
}

func TestArchtestCatchesCycleAndLoadErrors(t *testing.T) {
	const mod = "example.com/cycle"
	dir := fixture(t, "cycle")
	got := Check(goList(t, dir, "./..."), deskcorePolicy(mod, mod+"/a"))
	if len(got) == 0 {
		t.Fatal("an import cycle passed the check")
	}
	for _, v := range got {
		if !strings.Contains(v.Why, "import cycle") {
			t.Errorf("the cycle fixture produced a violation that does not name the cycle: %s", v)
		}
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

var pureDirs = []string{"domain", "facts", "policy", "config"}

// deskcoreRules are the call rules the pure packages live by.
func deskcoreRules() []Rule {
	return []Rule{
		{
			Pkg:   "time",
			Funcs: []string{"Now", "Since", "Until", "After", "AfterFunc", "Tick", "NewTimer", "NewTicker", "Sleep"},
			Why:   "a pure package reads no clock; the caller passes the time in",
		},
		{
			Pkg:   "encoding/json",
			Funcs: []string{"Unmarshal", "NewDecoder"},
			Allow: []string{"domain/strict.go:DecodeStrict", "domain/domain.go:OutcomeKind.UnmarshalJSON", "facts/facts.go:peekSchema"},
			Why:   "untrusted JSON is decoded only through domain.DecodeStrict",
		},
		{
			Method: "Sub",
			Allow:  []string{"facts/facts.go:ageAt"},
			Why:    "an age is computed only by facts.ageAt, which refuses a future time",
		},
	}
}

// TestPurePackagesFollowTheCallRules: no clock read in a pure package, one strict JSON decoder,
// and one age computation.
func TestPurePackagesFollowTheCallRules(t *testing.T) {
	found, err := ScanRules(os.DirFS(".."), pureDirs, deskcoreRules())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("%s", f)
	}
}

// TestCallRulesCatchPlantedCalls is the positive control for ScanRules.
func TestCallRulesCatchPlantedCalls(t *testing.T) {
	fsys := os.DirFS(fixture(t, "rules"))
	rules := []Rule{
		deskcoreRules()[0],
		{Pkg: "encoding/json", Funcs: []string{"Unmarshal"}, Allow: []string{"decode/decode.go:Strict"}, Why: "one decoder"},
		{Method: "Sub", Allow: []string{"age/age.go:ageAt"}, Why: "one age"},
	}
	found, err := ScanRules(fsys, []string{"clock", "dotclock", "decode", "age"}, rules)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"clock/clock.go:8: time.Since at package level":       true,
		"clock/clock.go:11: time.Sleep in Wait":               true,
		"clock/clock.go:12: time.Now in Wait":                 true,
		"dotclock/dotclock.go:3: dot-import of time":          true,
		"decode/decode.go:9: encoding/json.Unmarshal in Load": true,
		"age/age.go:10: call of method Sub in window.Fresh":   true,
	}
	for _, f := range found {
		key := ""
		for w := range want {
			if strings.HasPrefix(f.String(), w) {
				key = w
			}
		}
		if key == "" {
			t.Errorf("unexpected finding %s", f)
			continue
		}
		delete(want, key)
	}
	for w := range want {
		t.Errorf("planted call not caught: %s", w)
	}
	if _, err := ScanRules(fsys, []string{"missing"}, rules); err == nil {
		t.Error("a directory that does not exist was scanned as clean")
	}
	stale := []Rule{{Method: "Sub", Allow: []string{"age/age.go:ageAt", "age/age.go:gone"}, Why: "one age"}}
	if _, err := ScanRules(fsys, []string{"age"}, stale); err == nil || !strings.Contains(err.Error(), "age/age.go:gone") {
		t.Errorf("a stale allow entry was not refused: %v", err)
	}
}

// TestNoForgeableConstructedTypes: no pure package has a type whose constructor a composite
// literal could skip.
func TestNoForgeableConstructedTypes(t *testing.T) {
	found, err := ScanForgeable(os.DirFS(".."), pureDirs)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("%s", f)
	}
}

// TestForgeableLintCatchesExportedFields is the positive control for ScanForgeable.
func TestForgeableLintCatchesExportedFields(t *testing.T) {
	found, err := ScanForgeable(os.DirFS(fixture(t, "forgeable")), []string{"open", "ptr", "sealed", "plain"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"open/open.go": "type Grant", "ptr/ptr.go": "type Lease"}
	if len(found) != len(want) {
		t.Errorf("got %d findings, want %d: %v", len(found), len(want), found)
	}
	for _, f := range found {
		if w, ok := want[f.File]; !ok || !strings.HasPrefix(f.What, w) {
			t.Errorf("unexpected finding %s", f)
		}
	}
	if _, err := ScanForgeable(os.DirFS(fixture(t, "forgeable")), []string{"missing"}); err == nil {
		t.Error("a directory that does not exist was scanned as clean")
	}
}
