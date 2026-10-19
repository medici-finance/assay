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
