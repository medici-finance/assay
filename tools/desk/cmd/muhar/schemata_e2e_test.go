package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fxSrc is a small package with one function of each shape schemata handles:
// plain results, named results, a void function, an untested function a
// source-reading test pins, and a package-level constant (not in a function).
const fxSrc = `package fx

import "strings"

// Limit is package-level: a mutant of it is not inside a function body.
const Limit = 10

func Clamp(n int) int {
	if n > Limit {
		return Limit
	}
	if n < 0 {
		return 0
	}
	return n
}

func Join(parts []string) string {
	return strings.Join(parts, ",")
}

func Count(s string) (n int) {
	for _, r := range s {
		if r == 'a' {
			n++
		}
	}
	return
}

func Touch(m map[string]int, k string) {
	m[k]++
}

func Double(n int) int { return n * 2 }
`

const fxTest = `package fx

import (
	"os"
	"strings"
	"testing"
)

func TestClamp(t *testing.T) {
	for in, want := range map[int]int{11: 10, 10: 10, -1: 0, 5: 5} {
		if got := Clamp(in); got != want {
			t.Fatalf("Clamp(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestJoin(t *testing.T) {
	if got := Join([]string{"a", "b"}); got != "a,b" {
		t.Fatal(got)
	}
}

func TestCount(t *testing.T) {
	if got := Count("aab"); got != 2 {
		t.Fatal(got)
	}
}

func TestTouch(t *testing.T) {
	m := map[string]int{}
	Touch(m, "k")
	if m["k"] != 1 {
		t.Fatal(m)
	}
}

// Double has no behaviour test; this one reads the source from disk, so only
// a mutant written to disk can trip it.
func TestDoubleSource(t *testing.T) {
	raw, err := os.ReadFile("fx.go")
	if err != nil || !strings.Contains(string(raw), "return n * 2") {
		t.Fatal("Double's body changed on disk")
	}
}
`

// fxMutations returns the fixture's mutations and their classic verdicts.
func fxMutations() ([]Mutation, []Result) {
	type mv struct {
		old, new string
		want     Result
	}
	list := []mv{
		{"n > Limit {", "n > Limit+1 {", Caught},
		{"return Limit", "return Limit - 1", Caught},
		{"n < 0", "n < -1", Caught},
		{"return 0", "return 1", Caught},
		{"return n\n}", "return n + 1\n}", Caught},
		{`parts, ","`, `parts, ";"`, Caught},
		{"Join(parts,", "Join(parts[1:],", Caught},
		{"r == 'a'", "r != 'a'", Caught},
		{"n++", "n += 2", Caught},
		{"\treturn\n}", "\tn = 0\n\treturn\n}", Caught},
		{"m[k]++", "m[k]--", Caught},
		{"m[k]++", "m[k] = 0", Caught},
		{"n * 2", "n * 3", Caught},                                 // only the source-reading test sees it
		{"Limit = 10", "Limit = 11", Caught},                       // not in a function: classic
		{"\treturn strings", "\tx := 1\n\treturn strings", Caught}, // does not compile
		{"range s {", "range s + \"\" {", NotCaught},               // an equivalent mutant
	}
	var ms []Mutation
	var want []Result
	for i, m := range list {
		ms = append(ms, Mutation{Name: "m" + string(rune('a'+i)), File: "fx.go", Old: m.old, New: m.new})
		want = append(want, m.want)
	}
	return ms, want
}

// The cost (#2635): every mutant recompiled its package. With schemata the
// eligible mutants share one build per worker, so a sweep compiles the
// mutated package far fewer times than it has mutants, and the verdicts are
// exactly the classic ones: the mutant only a source-reading test can see,
// the one that does not compile and the one outside any function all still
// come out as before, as does the one no test catches.
//
// regression: #2635
func TestSchemataOneBuildPerWorker(t *testing.T) {
	needGo(t)
	if runtime.GOOS == "windows" {
		t.Skip("the compile counter is a shell script")
	}
	work := t.TempDir()
	bin := filepath.Join(work, "muhar")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build muhar: %v\n%s", err, out)
	}
	root := filepath.Join(work, "fx")
	writeFiles(t, root, map[string]string{
		"go.mod":     "module example.com/fx\n\ngo 1.21\n",
		"fx.go":      fxSrc,
		"fx_test.go": fxTest,
	})
	logPath := filepath.Join(work, "compiles.log")
	counter := filepath.Join(work, "count.sh")
	script := "#!/bin/sh\ncase \"$1\" in */compile) for a in \"$@\"; do [ \"$a\" = example.com/fx ] && echo x >> " + logPath + "; done;; esac\nexec \"$@\"\n"
	if err := os.WriteFile(counter, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ms, want := fxMutations()
	sp := spec{
		Root:      root,
		Test:      "go test -count=1 -toolexec=" + counter + " ./...",
		Control:   Mutation{Name: "control", File: "fx.go", Old: "return 0", New: "return -5"},
		Mutations: ms,
	}
	raw, _ := json.Marshal(sp)
	specPath := filepath.Join(work, "spec.json")
	if err := os.WriteFile(specPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-j", "2", "-spec", specPath)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GOFLAGS=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("muhar: %v\nstdout:\n%s\nstderr:\n%s", err, out, stderr.String())
	}
	for i, m := range ms {
		line := ""
		for _, l := range strings.Split(string(out), "\n") {
			if strings.HasSuffix(l, " "+m.Name) {
				line = l
			}
		}
		if !strings.Contains(line, want[i].String()+" ") {
			t.Errorf("mutation %s (%q -> %q): got line %q, want %s", m.Name, m.Old, m.New, line, want[i])
		}
	}
	if !strings.Contains(string(out), "Totals: 15 caught, 1 NOT CAUGHT, 0 could-not-mutate.") {
		t.Errorf("Totals line changed:\n%s", out)
	}
	logRaw, _ := os.ReadFile(logPath)
	compiles := strings.Count(string(logRaw), "x")
	if compiles >= len(ms) {
		t.Errorf("the sweep compiled the mutated package %d times for %d mutations: still a build per mutant", compiles, len(ms))
	}
	t.Logf("%d compiles of the mutated package for %d mutations", compiles, len(ms))
}
