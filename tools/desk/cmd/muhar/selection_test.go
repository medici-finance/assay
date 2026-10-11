package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeSelector returns a fixed Selection per mutation name.
type fakeSelector struct {
	prepErr error
	by      map[string]Selection
}

func (f *fakeSelector) Prepare(string) error { return f.prepErr }
func (f *fakeSelector) For(m Mutation, _ []byte) Selection {
	return f.by[m.Name]
}

// changedHarness: the full suite is red whenever core.txt differs from src, so
// every planted mutation is Caught without selection.
func changedHarness(t *testing.T) (*Harness, string) {
	t.Helper()
	const src = "invariant: n == p\nguard: breaker.active\n"
	dir, path := writeSrc(t, "core.txt", src)
	return &Harness{
		Root: dir,
		Test: func(string) bool {
			b, _ := os.ReadFile(path)
			return string(b) != src
		},
		Control: Mutation{Name: "control", File: "core.txt", Old: "n == p", New: "n != p"},
		Mutations: []Mutation{
			{Name: "guard", File: "core.txt", Old: "breaker.active", New: "true"},
			{Name: "absent", File: "core.txt", Old: "no such text", New: "x"},
		},
	}, src
}

// The safety property at the harness: a Skip is NOT CAUGHT, never Caught, and
// a selection never replaces the could-not-mutate state.
//
// regression: #2636
func TestSelectSkipNeverCaught(t *testing.T) {
	h, _ := changedHarness(t)
	h.Selector = &fakeSelector{by: map[string]Selection{
		"guard":  {Skip: true},
		"absent": {Skip: true},
	}}
	rep := h.Run()
	if rep.Broken {
		t.Fatalf("run broken: %s", rep.BrokenReason)
	}
	if got := rep.Results[0].Result; got != NotCaught {
		t.Fatalf("skipped mutation: want NOT_CAUGHT (selection only demotes), got %s", got)
	}
	if got := rep.Results[1].Result; got != CouldNotMutate {
		t.Fatalf("unlandable mutation under Skip: want COULD_NOT_MUTATE, got %s", got)
	}
}

// The control runs UNDER selection: a narrowed suite that cannot fail breaks
// the run, even though the full suite would have caught the control.
//
// regression: #2636
func TestSelectControlNarrowed(t *testing.T) {
	h, _ := changedHarness(t)
	h.Selector = &fakeSelector{by: map[string]Selection{
		"control": {Test: func(string) bool { return false }}, // narrowed suite stays green
	}}
	rep := h.Run()
	if !rep.Broken || !strings.Contains(rep.BrokenReason, "positive control was NOT CAUGHT") {
		t.Fatalf("control not caught under selection must break the run; got broken=%v %q", rep.Broken, rep.BrokenReason)
	}

	h, _ = changedHarness(t)
	h.Selector = &fakeSelector{by: map[string]Selection{"control": {Skip: true}}}
	if rep := h.Run(); !rep.Broken {
		t.Fatal("a control the selector skips must break the run")
	}
}

func TestSelectPrepareErrorBroken(t *testing.T) {
	h, _ := changedHarness(t)
	h.Selector = &fakeSelector{prepErr: errors.New("boom")}
	rep := h.Run()
	if !rep.Broken || !strings.Contains(rep.BrokenReason, "boom") {
		t.Fatalf("a failed coverage pass must break the run; got broken=%v %q", rep.Broken, rep.BrokenReason)
	}
}

// Every spec the workflows run must be one -select can model.
func TestParseGoTestSpecs(t *testing.T) {
	for _, sp := range []string{
		"internal/deskkit/mutations.json",
		"cmd/desksourceguard/mutations.json",
		"internal/loopengine/mutations.json",
		"internal/loopengine/mutations-retry.json",
		"cmd/deskclose/mutations.json",
		"cmd/reviewloop/mutations.json",
		"cmd/deskmerge/mutations.json",
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", sp))
		if err != nil {
			t.Fatalf("read %s: %v", sp, err)
		}
		var s spec
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatalf("parse %s: %v", sp, err)
		}
		g, err := parseGoTest(s.Test)
		if err != nil {
			t.Errorf("%s: %v", sp, err)
			continue
		}
		if len(g.pkgs) == 0 {
			t.Errorf("%s: no packages parsed from %q", sp, s.Test)
		}
	}

	g, err := parseGoTest(`go test ./a/ -count=1 -run 'TestX|TestY' ./b/...`)
	if err != nil {
		t.Fatal(err)
	}
	if g.run != "TestX|TestY" || strings.Join(g.pkgs, " ") != "./a/ ./b/..." || strings.Join(g.flags, " ") != "-count=1" {
		t.Fatalf("parsed %+v", g)
	}
	for _, bad := range []string{
		"go test ./... && ./verify.sh",
		"go test ./... | tee x",
		"go test -run 'TestA/sub' ./a/",
		"go test -tags=x ./a/",
		"go test -count=1",
		"make test",
		`go test -run "$X" ./a/`,
	} {
		if _, err := parseGoTest(bad); err == nil {
			t.Errorf("parseGoTest(%q) accepted a command selection cannot model", bad)
		}
	}
}

func TestFuncLines(t *testing.T) {
	const src = `package p

var limit = 3

func A(x int) bool {
	return x > 1
}
`
	if l1, l2, ok := funcLines("p.go", []byte(src), "x > 1"); !ok || l1 != 5 || l2 != 7 {
		t.Fatalf("in-function mutation: got %d-%d ok=%v, want 5-7", l1, l2, ok)
	}
	if _, _, ok := funcLines("p.go", []byte(src), "limit = 3"); ok {
		t.Fatal("a package-level declaration must not attribute to a function")
	}
	if _, _, ok := funcLines("p.go", []byte(src), "func A(x int) bool {"); ok {
		t.Fatal("a signature must not attribute to its function: an edit there can plant a new declaration")
	}
	if _, _, ok := funcLines("p.go", []byte(src), "absent"); ok {
		t.Fatal("absent text must not attribute")
	}
}

// selModule is a module whose tests catch mutants in four different ways.
var selModule = map[string]string{
	"go.mod": "module example.com/sel\n\ngo 1.21\n",
	"p/p.go": `package p

var Limit = 3

func A(x int) bool { return x > 1 }

func B(x int) bool { return x < 10 }

func C(x int) int { return x * 2 }
`,
	"p/p_test.go": `package p

import (
	"bytes"
	"os"
	"testing"
)

func TestA(t *testing.T) {
	if !A(2) || A(1) {
		t.Fatal("A")
	}
}

func TestB(t *testing.T) {
	if !B(3) || B(10) {
		t.Fatal("B")
	}
}

func TestLimit(t *testing.T) {
	if Limit != 3 {
		t.Fatal("Limit")
	}
}

// TestSource catches a change to C, or a planted function, without ever
// calling anything.
func TestSource(t *testing.T) {
	b, err := os.ReadFile("p.go")
	if err != nil || !bytes.Contains(b, []byte("x * 2")) {
		t.Fatal("C changed")
	}
	if n := bytes.Count(b, []byte("func ")); n != 3 {
		t.Fatalf("p.go declares %d functions, want 3", n)
	}
}
`,
}

func selSpec() spec {
	return spec{
		Test:    "go test ./p/ -count=1",
		Control: Mutation{Name: "control", File: "p/p.go", Old: "x > 1", New: "x >= 1"},
		Mutations: []Mutation{
			{Name: "A", File: "p/p.go", Old: "x > 1", New: "x > 0"},
			{Name: "B", File: "p/p.go", Old: "x < 10", New: "x <= 10"},
			{Name: "C", File: "p/p.go", Old: "x * 2", New: "x * 3"},
			{Name: "Limit", File: "p/p.go", Old: "Limit = 3", New: "Limit = 4"},
			// Plant edits a signature, Inject closes C's body and opens another
			// function: both add a declaration only TestSource sees.
			{Name: "Plant", File: "p/p.go", Old: "func C(x int) int {", New: "func D() {}\n\nfunc C(x int) int {"},
			{Name: "Inject", File: "p/p.go", Old: "x * 2", New: "x * 2 }\n\nfunc D() int { return 1"},
		},
	}
}

func runGo(root string, args []string) bool {
	c := exec.Command("go", args...)
	c.Dir = root
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	return c.Run() != nil
}

func selHarness(t *testing.T, root string, s spec, sel Selector) Report {
	t.Helper()
	h := &Harness{
		Root:      root,
		Control:   s.Control,
		Mutations: s.Mutations,
		Selector:  sel,
		Test: func(r string) bool {
			args, err := shellWords(s.Test)
			if err != nil {
				t.Fatal(err)
			}
			return runGo(r, args[1:])
		},
	}
	return h.Run()
}

func verdicts(rep Report) map[string]Result {
	out := map[string]Result{}
	for _, r := range rep.Results {
		out[r.Mutation.Name] = r.Result
	}
	return out
}

// End to end on a real module: selection narrows each mutant to the tests that
// execute its function, gives the same verdicts as the full sweep once the
// source-reading test is on the always-run list, and — without that list —
// can only DEMOTE the mutant no test executes, never promote one.
//
// regression: #2636
func TestCoverSelectorEndToEnd(t *testing.T) {
	needGo(t)
	root := t.TempDir()
	writeFiles(t, root, selModule)
	s := selSpec()
	cmd, err := parseGoTest(s.Test)
	if err != nil {
		t.Fatal(err)
	}

	full := selHarness(t, root, s, nil)
	if full.Broken {
		t.Fatalf("full sweep broken: %s", full.BrokenReason)
	}
	for name, v := range verdicts(full) {
		if v != Caught {
			t.Fatalf("full sweep: %s is %s, want CAUGHT", name, v)
		}
	}

	var mu sync.Mutex
	ran := map[string]string{} // mutation-run regex, keyed by the -run value
	record := func(r string, args []string) bool {
		mu.Lock()
		for i, a := range args {
			if a == "-run" {
				ran[args[i+1]] = strings.Join(args, " ")
			}
		}
		mu.Unlock()
		return runGo(r, args)
	}

	s.Select = &selectSpec{Always: []string{"TestSource"}}
	var log bytes.Buffer
	withAlways := selHarness(t, root, s, newCoverSelector(s, cmd, 2, os.Environ(), &log, record))
	if withAlways.Broken {
		t.Fatalf("selected sweep broken: %s\n%s", withAlways.BrokenReason, log.String())
	}
	if got, want := withAlways.Summary(), full.Summary(); got != want {
		t.Fatalf("selection changed the report:\nfull:\n%s\nselected:\n%s\nlog:\n%s", want, got, log.String())
	}
	// A's run is TestA plus the always-run test, nothing else; B's likewise.
	if _, ok := ran["^(TestA|TestSource)$"]; !ok {
		t.Fatalf("mutant A was not narrowed to TestA+TestSource; runs: %v\n%s", ran, log.String())
	}
	if _, ok := ran["^(TestB|TestSource)$"]; !ok {
		t.Fatalf("mutant B was not narrowed to TestB+TestSource; runs: %v", ran)
	}
	if !strings.Contains(log.String(), `"Limit": full suite (mutation is outside any function body)`) {
		t.Fatalf("package-level mutation must run the full suite; log:\n%s", log.String())
	}
	if !strings.Contains(log.String(), `"Plant": full suite (mutation is outside any function body)`) {
		t.Fatalf("a signature mutation must run the full suite; log:\n%s", log.String())
	}
	if !strings.Contains(log.String(), `"Inject": full suite (mutation adds or removes a top-level declaration)`) {
		t.Fatalf("a mutation that adds a declaration must run the full suite; log:\n%s", log.String())
	}

	// Without the always-run list, C (executed by no test, caught only by a
	// source read) is demoted to NOT CAUGHT — and nothing else moves: Plant and
	// Inject run the full suite, so the same source read still catches them.
	s.Select = nil
	bare := selHarness(t, root, s, newCoverSelector(s, cmd, 2, os.Environ(), &log, runGo))
	if bare.Broken {
		t.Fatalf("bare selected sweep broken: %s", bare.BrokenReason)
	}
	v := verdicts(bare)
	if v["C"] != NotCaught {
		t.Fatalf("C without the always-run list: want NOT_CAUGHT without a run, got %s", v["C"])
	}
	for _, name := range []string{"A", "B", "Limit", "Plant", "Inject"} {
		if v[name] != Caught {
			t.Fatalf("%s: selection must not move a covered mutant, got %s", name, v[name])
		}
	}
}

// A test that only passes after another has run would make a narrowed run red
// for a reason that is not the mutation (a false CAUGHT). The coverage pass
// runs every test alone, and one that fails alone breaks the run.
//
// regression: #2636
func TestSelectAloneFailBroken(t *testing.T) {
	needGo(t)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod": "module example.com/dep\n\ngo 1.21\n",
		"p/p.go": "package p\n\nvar ready bool\n\nfunc Set() { ready = true }\n\nfunc Ready() bool { return ready }\n",
		"p/p_test.go": `package p

import "testing"

func TestASet(t *testing.T) { Set() }

func TestBUse(t *testing.T) {
	if !Ready() {
		t.Fatal("not ready")
	}
}
`,
	})
	s := spec{
		Test:      "go test ./p/ -count=1",
		Control:   Mutation{Name: "control", File: "p/p.go", Old: "ready = true", New: "ready = false"},
		Mutations: []Mutation{{Name: "m", File: "p/p.go", Old: "return ready", New: "return true"}},
	}
	cmd, err := parseGoTest(s.Test)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	rep := selHarness(t, root, s, newCoverSelector(s, cmd, 1, os.Environ(), &log, runGo))
	if !rep.Broken || !strings.Contains(rep.BrokenReason, "TestBUse fails when run alone") {
		t.Fatalf("an order-dependent test must break a selected run; got broken=%v %q", rep.Broken, rep.BrokenReason)
	}
}

// The always-run list is checked against the suite: a misspelt name breaks the
// run instead of silently adding nothing.
func TestSelectAlwaysUnknown(t *testing.T) {
	needGo(t)
	root := t.TempDir()
	writeFiles(t, root, selModule)
	s := selSpec()
	s.Select = &selectSpec{Always: []string{"TestSourse"}}
	cmd, _ := parseGoTest(s.Test)
	var log bytes.Buffer
	rep := selHarness(t, root, s, newCoverSelector(s, cmd, 1, os.Environ(), &log, runGo))
	if !rep.Broken || !strings.Contains(rep.BrokenReason, `"TestSourse"`) {
		t.Fatalf("unknown always-run name must break the run; got broken=%v %q", rep.Broken, rep.BrokenReason)
	}
}

// A narrowed set can be red on pristine source even when every test passes
// alone and the full order passes: here TestT2 cleans up after TestT1, and
// selection drops TestT2 for a mutant in F. Using that set would report an
// equivalent mutant (one the full suite cannot see) as CAUGHT. Prepare runs
// each distinct set on pristine source, and a red one runs the full command.
//
// regression: #2636
func TestSelectSubsetRedOnPristine(t *testing.T) {
	needGo(t)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod": "module example.com/subset\n\ngo 1.21\n",
		"p/p.go": `package p

var polluted bool

func Pollute() { polluted = true }

func Reset() { polluted = false }

func Polluted() bool { return polluted }

func F(x int) int {
	y := x + 1
	return y
}

func G(x int) int { return x * 2 }
`,
		"p/p_test.go": `package p

import "testing"

func TestT1(t *testing.T) {
	Pollute()
	if F(1) != 2 {
		t.Fatal("F")
	}
}

func TestT2(t *testing.T) {
	Reset()
	if G(2) != 4 {
		t.Fatal("G")
	}
}

func TestT3(t *testing.T) {
	if Polluted() {
		t.Fatal("state leaked from an earlier test")
	}
	if F(2) != 3 {
		t.Fatal("F")
	}
}
`,
	})
	s := spec{
		Test:      "go test ./p/ -count=1",
		Control:   Mutation{Name: "control", File: "p/p.go", Old: "x * 2", New: "x * 3"},
		Mutations: []Mutation{{Name: "equivalent-F", File: "p/p.go", Old: "y := x + 1", New: "y := x + 1 + 0"}},
	}
	cmd, err := parseGoTest(s.Test)
	if err != nil {
		t.Fatal(err)
	}
	full := selHarness(t, root, s, nil)
	if full.Broken || verdicts(full)["equivalent-F"] != NotCaught {
		t.Fatalf("full sweep: want healthy with equivalent-F NOT_CAUGHT, got broken=%v %v", full.Broken, verdicts(full))
	}
	var log bytes.Buffer
	sel := selHarness(t, root, s, newCoverSelector(s, cmd, 2, os.Environ(), &log, runGo))
	if sel.Broken {
		t.Fatalf("selected sweep broken: %s\n%s", sel.BrokenReason, log.String())
	}
	if got, want := sel.Summary(), full.Summary(); got != want {
		t.Fatalf("a set red on pristine source changed the report:\nfull:\n%s\nselected:\n%s\nlog:\n%s", want, got, log.String())
	}
	if !strings.Contains(log.String(), "-run ^(TestT1|TestT3)$ example.com/subset/p` is RED on pristine source") {
		t.Fatalf("the red set must be named in the log:\n%s", log.String())
	}
	if !strings.Contains(log.String(), `"equivalent-F": full suite (narrowed set not proven green on pristine source)`) {
		t.Fatalf("the mutant on the red set must run the full suite:\n%s", log.String())
	}
}

// The coverage pass runs each test under the spec's own -race, -short and
// -timeout, so "passes alone" is checked in the narrowed runs' configuration.
func TestCoverFlags(t *testing.T) {
	cmd, err := parseGoTest("go test -race -short -count=1 -timeout 90s -v ./p/")
	if err != nil {
		t.Fatal(err)
	}
	build, run := (&coverSelector{cmd: cmd}).coverFlags()
	if got := strings.Join(build, " "); got != "-race" {
		t.Fatalf("build flags = %q, want -race", got)
	}
	if got := strings.Join(run, " "); got != "-test.short -test.timeout=90s" {
		t.Fatalf("run flags = %q", got)
	}
	cmd, _ = parseGoTest("go test -count=1 ./p/")
	if _, run := (&coverSelector{cmd: cmd}).coverFlags(); strings.Join(run, " ") != "-test.timeout=10m" {
		t.Fatalf("default run flags = %q", run)
	}
}
