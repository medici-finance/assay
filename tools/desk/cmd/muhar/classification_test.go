package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestTimeoutPolicyUsesMeasuredBaseline(t *testing.T) {
	policy, err := (TimeoutSpec{}).policy()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := policy.mutantLimit(5*time.Second), 25*time.Second; got != want {
		t.Fatalf("default timeout = %s, want baseline × 5 = %s", got, want)
	}
	policy, err = (TimeoutSpec{Multiplier: 2, Floor: "7s"}).policy()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := policy.mutantLimit(2*time.Second), 7*time.Second; got != want {
		t.Fatalf("floor timeout = %s, want %s", got, want)
	}
}

// regression: #2639
func TestBadBuildIsNotCaught(t *testing.T) {
	runCompileFixture(t, false, "go test ./pkg -count=1")
}

// regression: #2639
func TestSchemataFallbackCompile(t *testing.T) {
	runCompileFixture(t, true, "go test ./pkg -count=1")
}

// An environment prefix on the suite command must stay an assignment in the
// derived build command; quoted whole, sh runs it as a command and every
// mutant, the control included, fails to "build".
// regression: #2639
func TestBuildKeepsEnvPrefix(t *testing.T) {
	runCompileFixture(t, false, "GOWORK=off go test ./pkg -count=1")
}

// A value flag given with a space must keep its value, not swallow the
// mutant's package.
// regression: #2639
func TestBuildKeepsSpacedFlagValue(t *testing.T) {
	runCompileFixture(t, false, "go test -covermode atomic ./pkg -count=1")
}

// regression: #2639
func TestGoBuildFromTestFlags(t *testing.T) {
	dn := shellQuote(os.DevNull)
	for _, tc := range []struct {
		test, want string
	}{
		{"GOWORK=off go test ./... -count=1", "GOWORK='off' 'go' 'test' '-vet=off' '-c' '-o' " + dn + " './pkg'"},
		{"A=1 B='x y' go test ./...", "A='1' B='x y' 'go' 'test' '-vet=off' '-c' '-o' " + dn + " './pkg'"},
		{"go test -covermode atomic ./... -count 1", "'go' 'test' '-vet=off' '-c' '-o' " + dn + " '-covermode' 'atomic' './pkg'"},
		{"go test -bench X -benchtime=2s ./...", "'go' 'test' '-vet=off' '-c' '-o' " + dn + " './pkg'"},
		{"go test -o out -tags=t1 -race ./... -run X", "'go' 'test' '-vet=off' '-c' '-o' " + dn + " '-tags=t1' '-race' './pkg'"},
		{"go test --tags t1 ./...", "'go' 'test' '-vet=off' '-c' '-o' " + dn + " '-tags' 't1' './pkg'"},
	} {
		got, ok := goBuildFromTest(tc.test, "./pkg")
		if !ok || got != tc.want {
			t.Errorf("goBuildFromTest(%q) = %q, %v; want %q", tc.test, got, ok, tc.want)
		}
	}
	// A command it cannot model exactly falls back to the mutated package alone.
	for _, test := range []string{
		"go test -fakevalue v ./...",
		"go test -C sub ./...",
		"1BAD=x go test ./...",
		"go vet ./...",
		"go test ./... | tee log",
	} {
		if got, ok := goBuildFromTest(test, "./pkg"); ok {
			t.Errorf("goBuildFromTest(%q) = %q; want the mutated-package fallback", test, got)
		}
	}
}

// regression: #2639
func TestMutantTimeoutIsReported(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.21\n")
	writeFixture(t, filepath.Join(pkg, "p.go"), "package pkg\nfunc Control() int { return 1 }\nfunc Mutant() int { return 2 }\n")
	spec := map[string]any{
		"root":      root,
		"test":      "if grep -q 'return 0' pkg/p.go; then exit 1; fi; if grep -q 'return 3' pkg/p.go; then sleep 5; fi",
		"build":     "true",
		"control":   Mutation{Name: "positive-control", File: "pkg/p.go", Old: "return 1", New: "return 0"},
		"mutations": []Mutation{{Name: "slow-mutant", File: "pkg/p.go", Old: "return 2", New: "return 3"}},
		// The limit covers the build as well as the suite, so the floor must
		// leave room for a loaded host to run the build; the slow suite still
		// overruns it by seconds.
		"timeout": map[string]any{"multiplier": 0.01, "floor": "1500ms", "baseline": "30s"},
	}
	body, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(root, "spec.json")
	writeFixture(t, specPath, string(body))
	bin := muharTestBinary(t)
	cmd := exec.Command(bin, "-spec", specPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("muhar returned %v; want a healthy report\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "CAUGHT           slow-mutant  (TIMEOUT after ") || !strings.Contains(text, "Totals: 1 caught, 0 NOT CAUGHT, 0 could-not-mutate.") {
		t.Fatalf("timed-out mutant was not flagged as caught while preserving Totals\n%s", output)
	}
}

// runShellSpec runs muhar on a one-mutant spec over a trivial package, with the
// given suite, build and timeout, and returns its combined output and exit code.
func runShellSpec(t *testing.T, test, build string, timeout map[string]any) (string, int) {
	t.Helper()
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(pkg, "p.go"), "package pkg\nfunc Control() int { return 1 }\nfunc Mutant() int { return 2 }\n")
	spec := map[string]any{
		"root":      root,
		"test":      test,
		"build":     build,
		"control":   Mutation{Name: "positive-control", File: "pkg/p.go", Old: "return 1", New: "return 0"},
		"mutations": []Mutation{{Name: "survivor", File: "pkg/p.go", Old: "return 2", New: "return 3"}},
		"timeout":   timeout,
	}
	body, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(root, "spec.json")
	writeFixture(t, specPath, string(body))
	output, err := exec.Command(muharTestBinary(t), "-spec", specPath).CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("muhar did not run: %v\n%s", err, output)
		}
		code = exitErr.ExitCode()
	}
	return string(output), code
}

// The build and the suite share no deadline: a survivor whose suite finishes
// well inside the limit is NOT CAUGHT even when the build took most of it.
//
// regression: #2639
func TestSuiteCutShortByBuildIsNotCaught(t *testing.T) {
	out, code := runShellSpec(t,
		"if grep -q 'return 0' pkg/p.go; then exit 1; fi; sleep 1.5",
		"sleep 2",
		map[string]any{"multiplier": 0.01, "floor": "3s", "baseline": "30s"})
	if code != 0 || !regexp.MustCompile(`NOT_CAUGHT +survivor`).MatchString(out) || !strings.Contains(out, "Totals: 0 caught, 1 NOT CAUGHT, 0 could-not-mutate.") {
		t.Fatalf("a suite cut short by the build's share of the limit was counted as caught (exit %d)\n%s", code, out)
	}
}

// A suite that asserts nothing cannot pass the positive-control gate by being
// killed after a slow build.
//
// regression: #2639
func TestNullSuiteIsHarnessBroken(t *testing.T) {
	out, code := runShellSpec(t, "sleep 1.5", "sleep 2",
		map[string]any{"multiplier": 0.01, "floor": "3s", "baseline": "30s"})
	if code != 2 || !strings.Contains(out, "HARNESS BROKEN") || !strings.Contains(out, "positive control was NOT CAUGHT") {
		t.Fatalf("a suite that asserts nothing passed the positive-control gate (exit %d)\n%s", code, out)
	}
}

// A positive control the suite only times out on validates nothing: the run is
// HARNESS BROKEN, even though the same timeout is CAUGHT for a mutant.
//
// regression: #2639
func TestTimedOutControlIsHarnessBroken(t *testing.T) {
	out, code := runShellSpec(t, "if grep -q 'return 0' pkg/p.go; then sleep 6; fi", "true",
		map[string]any{"multiplier": 0.01, "floor": "1500ms", "baseline": "30s"})
	if code != 2 || !strings.Contains(out, "HARNESS BROKEN") || !strings.Contains(out, "positive control was caught only by a TIMEOUT") {
		t.Fatalf("a positive control caught only by a timeout passed the gate (exit %d)\n%s", code, out)
	}
}

// A mutant that survives its schema run is re-run the classic way with the
// whole limit, not what the schema run left of it: a fallback suite cut short
// would be a TIMEOUT, so CAUGHT, for a survivor.
//
// regression: #2639
func TestSchemataFallbackGetsWholeLimit(t *testing.T) {
	needGo(t)
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.21\n")
	// Schemata needs at least eight eligible mutants: seven the suite catches
	// at once, and the survivor whose fallback is under test.
	source := "package pkg\nfunc Value() int { return 1 }\nfunc Other() int { return 2 }\n"
	check := ""
	mutations := []Mutation{{Name: "survivor", File: "pkg/p.go", Old: "return 2", New: "return 3"}}
	for i := 0; i < 7; i++ {
		source += fmt.Sprintf("func V%d() int { return %d }\n", i, 10+i)
		check += fmt.Sprintf("\tif V%d() != %d { t.Fatal(\"V%d\") }\n", i, 10+i, i)
		mutations = append(mutations, Mutation{Name: fmt.Sprintf("caught-%d", i), File: "pkg/p.go", Old: fmt.Sprintf("return %d", 10+i), New: fmt.Sprintf("return %d", 20+i)})
	}
	writeFixture(t, filepath.Join(pkg, "p.go"), source)
	writeFixture(t, filepath.Join(pkg, "p_test.go"), "package pkg\nimport (\n\t\"testing\"\n\t\"time\"\n)\nfunc TestValue(t *testing.T) {\n\tif Value() != 1 { t.Fatal(\"value\") }\n"+check+"}\nfunc TestSlow(t *testing.T) { if Other() == 3 { time.Sleep(3 * time.Second) } }\n")
	spec := map[string]any{
		"root":      root,
		"test":      "go test -count=1 ./...",
		"control":   Mutation{Name: "positive-control", File: "pkg/p.go", Old: "return 1", New: "return 0"},
		"mutations": mutations,
		"schemata":  true,
		// Only the survivor's suite is slow. One run of it (about 3s) fits
		// the 5s limit; a schema run plus a fallback build and suite on what
		// is left of the same limit does not.
		"timeout": map[string]any{"multiplier": 0.01, "floor": "5s", "baseline": "120s"},
	}
	body, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(root, "spec.json")
	writeFixture(t, specPath, string(body))
	output, err := exec.Command(muharTestBinary(t), "-j", "2", "-spec", specPath).CombinedOutput()
	if err != nil {
		t.Fatalf("muhar returned %v; want a healthy report\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "8 of 8 mutations share one build per worker") {
		t.Fatalf("fixture did not exercise the schemata path\n%s", output)
	}
	if !regexp.MustCompile(`NOT_CAUGHT +survivor`).MatchString(text) || !strings.Contains(text, "Totals: 7 caught, 1 NOT CAUGHT, 0 could-not-mutate.") {
		t.Fatalf("a survivor's classic fallback ran on the schema run's leftover limit and was counted as caught\n%s", output)
	}
}

func runCompileFixture(t *testing.T, schemata bool, test string) {
	t.Helper()
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.21\n")
	source := "package pkg\nfunc Value() int { return 1 }\nfunc Other() int { return 3 }\nfunc Another() int { return 4 }\n"
	mutations := []Mutation{
		{Name: "bad-build", File: "pkg/p.go", Old: "return 3", New: "return missingValue"},
		{Name: "bad-build-second", File: "pkg/p.go", Old: "return 4", New: "return secondMissingValue"},
	}
	for i := 0; i < 8; i++ {
		source += fmt.Sprintf("func Extra%d() int { return %d }\n", i, 10+i)
		mutations = append(mutations, Mutation{Name: fmt.Sprintf("extra-%d", i), File: "pkg/p.go", Old: fmt.Sprintf("return %d", 10+i), New: fmt.Sprintf("return %d", 20+i)})
	}
	writeFixture(t, filepath.Join(pkg, "p.go"), source)
	writeFixture(t, filepath.Join(pkg, "p_test.go"), "package pkg\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"value\") } }\n")
	spec := map[string]any{
		"root":      root,
		"test":      test,
		"control":   Mutation{Name: "positive-control", File: "pkg/p.go", Old: "return 1", New: "return 2"},
		"mutations": mutations,
		"schemata":  schemata,
	}
	body, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(root, "spec.json")
	writeFixture(t, specPath, string(body))

	bin := muharTestBinary(t)
	cmd := exec.Command(bin, "-spec", specPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("muhar returned %v; want a healthy report\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "COULD_NOT_MUTATE bad-build") || !strings.Contains(text, "COULD_NOT_MUTATE bad-build-second") || !regexp.MustCompile(`Totals: [0-9]+ caught, [0-9]+ NOT CAUGHT, 2 could-not-mutate\.`).MatchString(text) {
		t.Fatalf("a non-compiling mutation was counted as caught; want COULD_NOT_MUTATE\n%s", output)
	}
	if schemata && !strings.Contains(text, "8 of 10 mutations share one build per worker") {
		t.Fatalf("fixture did not exercise the schemata path and its classic fallback\n%s", output)
	}
}

func writeFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func muharTestBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "muhar")
	build := exec.Command("go", "build", "-o", bin, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build muhar: %v\n%s", err, output)
	}
	return bin
}

// cdFixture lays out a checkout with two modules: desk (the spec's root) and
// other (the module a `cd ../other && go test …` suite runs in), mirroring a
// tools/desk spec whose suite runs in a sibling module. It returns the
// checkout and the spec root.
func cdFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	for _, dir := range []string{".git", "desk", "other"} {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(t, filepath.Join(repo, "desk", "go.mod"), "module desk\n\ngo 1.21\n")
	writeFixture(t, filepath.Join(repo, "desk", "d.go"), "package desk\n")
	writeFixture(t, filepath.Join(repo, "other", "go.mod"), "module other\n\ngo 1.21\n")
	writeFixture(t, filepath.Join(repo, "other", "p.go"), "package other\nfunc Value() int { return 1 }\nfunc Other() int { return 2 }\nfunc Third() int { return 4 }\n")
	writeFixture(t, filepath.Join(repo, "other", "p_test.go"), "package other\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 || Other() != 2 { t.Fatal(\"value\") } }\n")
	return repo, filepath.Join(repo, "desk")
}

func runCdSpec(t *testing.T, root, test string) (string, int) {
	t.Helper()
	spec := map[string]any{
		"root":    root,
		"test":    test,
		"control": Mutation{Name: "positive-control", File: "../other/p.go", Old: "return 1", New: "return 0"},
		"mutations": []Mutation{
			{Name: "other-mutant", File: "../other/p.go", Old: "return 2", New: "return 3"},
			// Proves the build compiles the mutated package itself: a build
			// of any other package would pass it, and its suite would then
			// count it CAUGHT.
			{Name: "bad-build", File: "../other/p.go", Old: "return 4", New: "return missingValue"},
		},
	}
	body, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(t.TempDir(), "spec.json")
	writeFixture(t, specPath, string(body))
	output, err := exec.Command(muharTestBinary(t), "-j", "1", "-spec", specPath).CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("muhar did not run: %v\n%s", err, output)
		}
		code = exitErr.ExitCode()
	}
	return string(output), code
}

// A suite that starts `cd <dir> &&` runs in <dir>, and so must each mutant's
// build: built from the spec root instead, a mutant in another module is
// "outside main module", the positive control cannot be validated, and the
// whole run is discarded as HARNESS BROKEN.
//
// regression: #4117
func TestBuildFollowsLeadingCd(t *testing.T) {
	needGo(t)
	for _, test := range []string{
		"cd ../other && go test -count=1 ./...",
		"cd ../other && GOWORK=off go test -count=1 .",
		// The shape of the spec that found the defect: an assignment whose
		// value is a command substitution, and a quoted test filter.
		"cd ../other && HOME=$(mktemp -d) KUBECONFIG=/dev/null go test -count=1 -timeout 60s -run '^(TestValue|TestNone)$' .",
	} {
		_, root := cdFixture(t)
		out, code := runCdSpec(t, root, test)
		if code != 0 || !strings.Contains(out, "Harness healthy: baseline GREEN, positive control CAUGHT.") || !strings.Contains(out, "COULD_NOT_MUTATE bad-build") || !strings.Contains(out, "Totals: 1 caught, 0 NOT CAUGHT, 1 could-not-mutate.") {
			t.Errorf("suite %q: the mutant build did not follow the leading cd (exit %d)\n%s", test, code, out)
		}
	}
}

// A compound that is not exactly `cd <dir> && go test …` is not followed: the
// build runs from the spec root as before, so the cross-module control does
// not build and the run is discarded, never judged from the wrong directory.
//
// regression: #4117
func TestBuildRefusesCompoundCd(t *testing.T) {
	needGo(t)
	_, root := cdFixture(t)
	out, code := runCdSpec(t, root, "cd ../other; go test -count=1 ./...")
	if code != 2 || !strings.Contains(out, "positive control could not be validated") {
		t.Fatalf("a `;` compound was followed (exit %d)\n%s", code, out)
	}
}

// regression: #4117
func TestLeadingCd(t *testing.T) {
	repo, base := cdFixture(t)
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	other := realPath(t, filepath.Join(repo, "other"))
	escapeRel, err := filepath.Rel(base, outside)
	if err != nil {
		t.Fatal(err)
	}
	escapeRel = filepath.ToSlash(escapeRel)
	for _, tc := range []struct{ test, dir, rest string }{
		{"cd ../other && go test ./...", other, "go test ./..."},
		{"cd\t../other\t&&\tgo test .", other, "go test ."},
		{"cd sub && A=1 B='x y' go test -run '^(X|Y)$' ./...", realPath(t, filepath.Join(base, "sub")), "A=1 B='x y' go test -run '^(X|Y)$' ./..."},
		{"cd ../other && HOME=$(mktemp -d) go test .", other, "HOME=$(mktemp -d) go test ."},
		{"cd . && go test ./...", realPath(t, base), "go test ./..."},
	} {
		dir, rest, ok := leadingCd(base, tc.test, nil)
		if !ok || dir != tc.dir || rest != tc.rest {
			t.Errorf("leadingCd(%q) = %q, %q, %v; want %q, %q", tc.test, dir, rest, ok, tc.dir, tc.rest)
		}
	}
	// The derived build keeps the suite's own environment and flags.
	if _, rest, ok := leadingCd(base, "cd ../other && GOWORK=off go test -tags=t1 ./... -count=1", nil); !ok {
		t.Fatal("cd with an environment prefix was refused")
	} else if got, ok := goBuildFromTest(rest, "."); !ok || got != "GOWORK='off' 'go' 'test' '-vet=off' '-c' '-o' "+shellQuote(os.DevNull)+" '-tags=t1' '.'" {
		t.Errorf("derived build after cd = %q, %v", got, ok)
	}
	refused := []string{
		// The shell would leave for /sub; joined to the root it would name the
		// root's own sub, so an absolute directory is never followed.
		"cd /sub && go test ./...",
	}
	// A directory whose name the shell would expand is never followed, even
	// when a directory of that literal name exists.
	if err := os.MkdirAll(filepath.Join(base, "a$b"), 0o755); err == nil {
		refused = append(refused, "cd a$b && go test ./...")
	}
	for _, test := range append(refused,
		"go test ./...",                         // no cd: unchanged behaviour
		"GOWORK=off go test ./...",              // no cd
		"cd "+other+" && go test ./...",         // absolute
		"cd "+escapeRel+" && go test ./...",     // escapes the checkout
		"cd ../escape && go test ./...",         // escapes through a symlink
		"cd ../missing && go test ./...",        // does not exist
		"cd ../other/p.go && go test ./...",     // not a directory
		"cd ~ && go test ./...",                 // tilde
		"cd $HOME && go test ./...",             // expansion
		"cd '../other' && go test ./...",        // quoted
		"cd ../ot* && go test ./...",            // glob
		"cd ../other;go test ./...",             // not &&
		"cd ../other; go test ./...",            // not &&
		"cd ../other &&go test ./...",           // malformed
		"cd ../other || go test ./...",          // not &&
		"cd ../other && go vet ./...",           // not go test
		"cd ../other && cd .. && go test ./...", // a second cd
		"cd ../other && go test ./... && rm x",  // chained
		"cd ../other && go test ./... ; rm x",   // chained
		"cd ../other && go test ./... | tee l",  // piped
		"cd ../other && go test ./... > log",    // redirected
		"cd ../other && go test ./... &",        // backgrounded
		"cd ../other && go test `pwd`",          // backtick
		"cd ../other && go test $(cd /; pwd)",   // chain inside a substitution
		"cd ../other && go test \"$(pwd)\"",     // substitution in double quotes
		"cd ../other && (go test ./...)",        // subshell
		"cd ../other && go test (x)",            // subshell
		"cd ../other && go test 'unterminated",  // bad quote
		"cd ../other && go test $(pwd",          // unbalanced
		"cd ../other && go test \\; x",          // escape
		"cd ../other && go test ./...\nrm x",    // newline
		"cd ../other\n&& go test ./...",         // newline
		"cd ../other && \"go\" test ./...",      // quoted command word
	) {
		if dir, rest, ok := leadingCd(base, test, nil); ok {
			t.Errorf("leadingCd(%q) = %q, %q; want refused", test, dir, rest)
		}
	}
	// With no checkout above it (a parallel worker's copy), the bound is the
	// spec root itself.
	bare := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bare, "root", "in"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bare, "sibling"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := leadingCd(filepath.Join(bare, "root"), "cd in && go test .", nil); !ok {
		t.Error("cd into the root's own subdirectory was refused")
	}
	if checkoutRoot(bare) == bare {
		if _, _, ok := leadingCd(filepath.Join(bare, "root"), "cd ../sibling && go test .", nil); ok {
			t.Error("cd out of a root with no checkout above it was followed")
		}
	}
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// A cd word starting with `-` is the previous directory, the end of options
// or an option to the shell, never base/word, even when a directory of that
// literal name exists.
//
// regression: #6793
func TestLeadingCdRefusesDash(t *testing.T) {
	_, base := cdFixture(t)
	for _, name := range []string{"-", "--", "-L", "-P", "-sub"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if dir, _, ok := leadingCd(base, "cd "+name+" && go test .", nil); ok {
			t.Errorf("cd %s was followed to %q; the shell does not go there", name, dir)
		}
	}
	if _, _, ok := leadingCd(base, "cd ./-sub && go test .", nil); !ok {
		t.Error("cd ./-sub was refused; the shell does go there")
	}
}

// With a non-empty CDPATH the shell searches it before base for a word whose
// first component is not `.` or `..`, so such a word is not followed.
//
// regression: #6793
func TestLeadingCdRefusesCdpath(t *testing.T) {
	repo, base := cdFixture(t)
	for _, name := range []string{"sub", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cdpath := []string{"CDPATH=" + repo}
	for _, test := range []string{"cd sub && go test .", "cd .hidden && go test ."} {
		if dir, _, ok := leadingCd(base, test, cdpath); ok {
			t.Errorf("%q followed to %q under CDPATH", test, dir)
		}
	}
	for _, test := range []string{"cd ./sub && go test .", "cd ../other && go test .", "cd . && go test ."} {
		if _, _, ok := leadingCd(base, test, cdpath); !ok {
			t.Errorf("%q refused under CDPATH; CDPATH is not searched for it", test)
		}
	}
	for _, env := range [][]string{nil, {"CDPATH="}, {"CDPATH=" + repo, "CDPATH="}} {
		if _, _, ok := leadingCd(base, "cd sub && go test .", env); !ok {
			t.Errorf("cd sub refused with env %q; no CDPATH applies", env)
		}
	}
}

// With no .git above the spec root (a source archive or export), a cd out of
// the root is not followed: the cross-module control cannot build and the run
// is HARNESS BROKEN. A .git at the export root makes the same spec pass.
//
// regression: #6793
func TestBuildNoCheckoutFailsClosed(t *testing.T) {
	needGo(t)
	repo, root := cdFixture(t)
	if err := os.Remove(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	if got := checkoutRoot(root); got != root {
		t.Skipf("a .git above the temporary directory (%s) bounds the fixture", got)
	}
	test := "cd ../other && go test -count=1 ."
	out, code := runCdSpec(t, root, test)
	if code != 2 || !strings.Contains(out, "positive control could not be validated") {
		t.Fatalf("a cd out of a root with no checkout was followed (exit %d)\n%s", code, out)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, code := runCdSpec(t, root, test); code != 0 || !strings.Contains(out, "Totals: 1 caught, 0 NOT CAUGHT, 1 could-not-mutate.") {
		t.Fatalf("with a .git at the export root the cd was not followed (exit %d)\n%s", code, out)
	}
}

// The followed directory is returned resolved, so swapping a symlink on the
// joined path after the containment check cannot move the build outside the
// checkout.
//
// regression: #6793
func TestLeadingCdReturnsResolved(t *testing.T) {
	repo, base := cdFixture(t)
	link := filepath.Join(base, "link")
	if err := os.Symlink(filepath.Join(repo, "other"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	dir, _, ok := leadingCd(base, "cd link && go test .", nil)
	if !ok {
		t.Fatal("cd through a symlink inside the checkout was refused")
	}
	outside := t.TempDir()
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	now, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !within(realPath(t, repo), now) {
		t.Fatalf("after the swap the build directory %q names %q, outside the checkout", dir, now)
	}
}

// The followed directory is resolved, so the mutated file is too: a spec root
// reached through a symlink still builds each mutant in its own package.
//
// regression: #6793
func TestBuildCdUnderSymlinkRoot(t *testing.T) {
	needGo(t)
	repo, _ := cdFixture(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	out, code := runCdSpec(t, filepath.Join(alias, "desk"), "cd ../other && go test -count=1 .")
	if code != 0 || !strings.Contains(out, "COULD_NOT_MUTATE bad-build") || !strings.Contains(out, "Totals: 1 caught, 0 NOT CAUGHT, 1 could-not-mutate.") {
		t.Fatalf("a root reached through a symlink did not build the followed mutant (exit %d)\n%s", code, out)
	}
}
