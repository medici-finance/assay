package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// A schema mutant must report the positions the classic mutated file would:
// the same file, the same line and the same function name, for code after the
// edit in the mutated function itself (the edit here adds two lines).
func TestSchemaPositionsMatchClassic(t *testing.T) {
	needGo(t)
	src := "package fx\n\nimport \"runtime\"\n\nfunc Where() string {\n\tx := 1\n\t_ = x\n" +
		"\tpc, file, line, _ := runtime.Caller(0)\n\treturn file + \":\" + itoa(line) + \" \" + runtime.FuncForPC(pc).Name()\n}\n\n" +
		"func itoa(n int) string {\n\tif n < 10 {\n\t\treturn string(rune('0' + n))\n\t}\n\treturn itoa(n/10) + string(rune('0'+n%10))\n}\n"
	test := "package fx\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestDump(t *testing.T) {\n\tos.WriteFile(os.Getenv(\"DUMP\"), []byte(Where()), 0o644)\n}\n"
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"go.mod": "module example.com/fx\n\ngo 1.21\n", "fx.go": src, "fx_test.go": test})
	m := Mutation{Name: "grow", File: "fx.go", Old: "\t_ = x\n", New: "\t_ = x\n\tx++\n\tx--\n"}

	run := func(env ...string) string {
		dump := filepath.Join(t.TempDir(), "dump")
		cmd := exec.Command("go", "test", "-count=1", ".")
		cmd.Dir = root
		cmd.Env = append(envWithout(os.Environ(), "GOFLAGS", schemaEnv), append(env, "DUMP="+dump)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go test: %v\n%s", err, out)
		}
		raw, _ := os.ReadFile(dump)
		return string(raw)
	}

	tr, err := buildSchemaTree(root, map[int]Mutation{0: m}, false)
	if err != nil || !tr.ids["0"] {
		t.Fatalf("mutant not eligible: %v %v", err, tr.skipped)
	}
	defer tr.cleanup()
	pristine := run()
	schemaBase := run("GOFLAGS=-overlay="+tr.overlay, schemaEnv+"="+schemaNone)
	schema := run("GOFLAGS=-overlay="+tr.overlay, schemaEnv+"=0")

	path := filepath.Join(root, "fx.go")
	orig, _ := os.ReadFile(path)
	mutated, _ := applyEdit(string(orig), m)
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	classic := run()
	_ = os.WriteFile(path, orig, 0o644)

	if schemaBase != pristine {
		t.Errorf("schemata with no mutant selected reports %q, pristine reports %q", schemaBase, pristine)
	}
	if schema != classic {
		t.Errorf("schema mutant reports %q, classic mutant reports %q", schema, classic)
	}
	if classic == pristine {
		t.Fatalf("fixture does not move the line (%q)", classic)
	}
}

func TestSchemaSiteRefusals(t *testing.T) {
	const src = `package p

var V = 1

func A(n int) (err error) {
	x, err := f()
	_ = x
	return err
}

func B() {
loop:
	for {
		break loop
	}
}

func C(n int) int {
	return n + 1
}

func f() (int, error) { return 0, nil }
`
	cases := []struct {
		name, path string
		m          Mutation
		reason     string
	}{
		{"package var", "p.go", Mutation{Old: "V = 1", New: "V = 2"}, "edit is not inside a function body"},
		{"redeclared result", "p.go", Mutation{Old: "_ = x", New: "_ = x + 1"}, "mutated body redeclares a parameter or result with :="},
		{"labels", "p.go", Mutation{Old: "break loop", New: "continue loop"}, "function uses labels"},
		{"not go", "p.txt", Mutation{Old: "n + 1", New: "n + 2"}, "not a .go file"},
		{"absent", "p.go", Mutation{Old: "nope", New: "x"}, "edit does not apply"},
		{"escapes body", "p.go", Mutation{Old: "return n + 1\n", New: "return n\n}\n\nfunc D() int {\n\treturn 0\n"}, "edit changes the declarations"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, why := schemaSiteFor(tc.path, src, tc.m, "0"); why != tc.reason {
				t.Fatalf("reason = %q, want %q", why, tc.reason)
			}
		})
	}
	if _, why := schemaSiteFor("p.go", src, Mutation{Old: "n + 1", New: "n + 2"}, "0"); why != "" {
		t.Fatalf("a plain body edit was refused: %s", why)
	}
	if _, why := schemaSiteFor("p.go", src+"\nvar muharX = 1\n", Mutation{Old: "n + 1", New: "n + 2"}, "0"); why == "" {
		t.Fatal("a file already using the muhar prefix was accepted")
	}
}

// Schemata is trusted for CAUGHT only: a green schemata run is re-run the
// classic way and the classic verdict is reported.
func TestSchemaGreenRerunsClassic(t *testing.T) {
	dir, path := writeSrc(t, "core.go", "package core\n\nfunc F() int {\n\treturn 1\n}\n")
	classicRuns := 0
	h := &Harness{
		Root:      dir,
		Mutations: []Mutation{{Name: "m", File: "core.go", Old: "return 1", New: "return 2"}},
		Test: func(string) bool {
			classicRuns++
			raw, _ := os.ReadFile(path)
			return strings.Contains(string(raw), "return 2") // red only when the edit is on disk
		},
	}
	tr := &schemaTree{ids: map[string]bool{"0": true}}

	h.SchemaTest = func(_, _, _ string) bool { return false }
	if res, _ := h.runMutation(dir, tr, 0); res != Caught || classicRuns != 1 {
		t.Fatalf("green schemata run: got %s after %d classic runs, want CAUGHT after 1", res, classicRuns)
	}
	h.SchemaTest = func(_, _, sel string) bool { return sel == "0" }
	classicRuns = 0
	if res, _ := h.runMutation(dir, tr, 0); res != Caught || classicRuns != 0 {
		t.Fatalf("red schemata run: got %s after %d classic runs, want CAUGHT with none", res, classicRuns)
	}
}

func TestSchemataUsable(t *testing.T) {
	cases := []struct {
		test, goflags, tmp string
		want               bool
	}{
		{"go test ./x/ -count=1 -run 'TestA'", "", "/tmp", true},
		{"go test ./x/", "", "/tmp", false},          // a cached pass could be replayed
		{"go test ./x/ -count=2", "", "/tmp", false}, // only -count=1 is recognised
		{"make test -count=1", "", "/tmp", false},    // not a go test
		{"go test -count=1 -overlay=o.json ./x/", "", "/tmp", false},
		{"go test -count=1 ./x/", "-overlay=o.json", "/tmp", false},
		{"go test -count=1 ./x/", "", "/my tmp", false}, // GOFLAGS splits on spaces
	}
	for _, c := range cases {
		if got := schemataUsable(c.test, c.goflags, c.tmp); got != c.want {
			t.Errorf("schemataUsable(%q, %q, %q) = %v, want %v", c.test, c.goflags, c.tmp, got, c.want)
		}
	}
}

func TestSchemaBlameMapsLines(t *testing.T) {
	tr := &schemaTree{copies: map[string]*schemaCopy{
		"0_a.go": {ids: []string{"1", "2"}, spans: map[string][2]int{"1": {10, 12}, "2": {13, 20}}},
		"1_b.go": {ids: []string{"3"}, spans: map[string][2]int{"3": {5, 6}}},
	}}
	bad, ok := tr.blame("../x/0_a.go:11:3: declared and not used: y\n../x/1_b.go:40:1: oops\n")
	if !ok || !bad["1"] || bad["2"] || !bad["3"] {
		t.Fatalf("blame = %v %v", bad, ok)
	}
	if _, ok := tr.blame("other.go:3:1: broken\n"); ok {
		t.Fatal("a diagnostic outside every schemata copy was blamed on a mutant")
	}
	if _, ok := tr.blame("go: no main packages to build\n"); ok {
		t.Fatal("output with no diagnostic was blamed on a mutant")
	}
}

// guardFixture is a package of nine one-line functions, each mutant one of
// them, so every mutant is eligible and the eligibility floor is met. The
// classic suite stub is red only for the control, so every classic verdict
// for a mutant is NOT CAUGHT: any CAUGHT in a report came from schemata.
func guardFixture(t *testing.T) *Harness {
	t.Helper()
	var src strings.Builder
	src.WriteString("package g\n")
	var ms []Mutation
	for i := 0; i < 9; i++ {
		lit := strconv.Itoa(1000 + i)
		src.WriteString("\nfunc F" + strconv.Itoa(i) + "() int {\n\treturn " + lit + "\n}\n")
		ms = append(ms, Mutation{Name: "f" + strconv.Itoa(i), File: "g.go", Old: "return " + lit, New: "return " + lit + "7"})
	}
	src.WriteString("\nfunc C() int {\n\treturn 5000\n}\n")
	dir, path := writeSrc(t, "g.go", src.String())
	return &Harness{
		Root:      dir,
		Control:   Mutation{Name: "control", File: "g.go", Old: "return 5000", New: "return 5001"},
		Mutations: ms,
		Test: func(string) bool {
			raw, _ := os.ReadFile(path)
			return strings.Contains(string(raw), "return 5001")
		},
	}
}

func wantAllClassic(t *testing.T, rep Report) {
	t.Helper()
	if rep.Broken {
		t.Fatalf("harness broken: %s", rep.BrokenReason)
	}
	for _, r := range rep.Results {
		if r.Result != NotCaught {
			t.Errorf("%s: %s, want NOT_CAUGHT (the classic verdict): a schemata red was trusted without its guard", r.Mutation.Name, r.Result)
		}
	}
}

// A schemata build that is red with no mutant selected proves nothing: its
// reds must never be read as CAUGHT.
func TestSchemaBaselineRedIsClassic(t *testing.T) {
	h := guardFixture(t)
	h.SchemaTest = func(_, _, _ string) bool { return true } // red whatever is selected
	wantAllClassic(t, h.Run())
}

// A schemata build that does not catch the positive control cannot be
// trusted to catch anything, so the run goes classic.
func TestSchemaControlMissIsClassic(t *testing.T) {
	h := guardFixture(t)
	h.SchemaTest = func(_, _, sel string) bool { return sel != schemaNone && sel != schemaControl }
	wantAllClassic(t, h.Run())
}

// Each extra worker proves the schemata baseline in its own copy; a red one
// gets no schemata tree, so all of that worker's mutants run classically.
func TestSchemaWorkerBaselineRed(t *testing.T) {
	h := guardFixture(t)
	other := t.TempDir()
	raw, _ := os.ReadFile(filepath.Join(h.Root, "g.go"))
	if err := os.WriteFile(filepath.Join(other, "g.go"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	h.SchemaTest = func(root, _, sel string) bool {
		if sel == schemaNone {
			return root == other // the baseline is red only in the other copy
		}
		return true
	}
	plan, first := h.schemataSetup(h.Root)
	defer first.cleanup()
	if plan == nil || first == nil {
		t.Fatal("schemata did not engage in the first copy, where its baseline is green")
	}
	if tr := h.schemataFor(other, plan); tr != nil {
		tr.cleanup()
		t.Fatal("a worker copy whose schemata baseline is red got a schemata tree")
	}
	if tr := h.schemataFor(h.Root, plan); tr == nil {
		t.Fatal("a worker copy whose schemata baseline is green got none")
	} else {
		tr.cleanup()
	}
}

// The classic run reports COULD_NOT_MUTATE for a file it cannot write;
// schemata never writes, so such a file must stay classic.
func TestSchemaReadOnlyFileClassic(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file the process cannot open for writing")
	}
	dir, path := writeSrc(t, "p.go", "package p\n\nfunc C(n int) int {\n\treturn n + 1\n}\n")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	tr, err := buildSchemaTree(dir, map[int]Mutation{0: {File: "p.go", Old: "n + 1", New: "n + 2"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.cleanup()
	if tr.ids["0"] || tr.skipped["file not writable"] != 1 {
		t.Fatalf("read-only file: ids=%v skipped=%v, want it skipped as not writable", tr.ids, tr.skipped)
	}
}
