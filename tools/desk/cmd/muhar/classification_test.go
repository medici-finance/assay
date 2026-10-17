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
	runCompileFixture(t, false)
}

// regression: #2639
func TestSchemataFallbackCompile(t *testing.T) {
	runCompileFixture(t, true)
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
	writeFixture(t, filepath.Join(pkg, "p_test.go"), "package pkg\nimport (\"os\"; \"strings\"; \"testing\"; \"time\")\nfunc TestValues(t *testing.T) { b, _ := os.ReadFile(\"p.go\"); s := string(b); if strings.Contains(s, \"return 0\") { t.Fatal(\"control\") }; if strings.Contains(s, \"return 3\") { time.Sleep(2*time.Second) } }\n")
	spec := map[string]any{
		"root":      root,
		"test":      "go test ./pkg -count=1",
		"control":   Mutation{Name: "positive-control", File: "pkg/p.go", Old: "return 1", New: "return 0"},
		"mutations": []Mutation{{Name: "slow-mutant", File: "pkg/p.go", Old: "return 2", New: "return 3"}},
		"timeout":   map[string]any{"multiplier": 1, "floor": "20ms", "baseline": "30s"},
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

func runCompileFixture(t *testing.T, schemata bool) {
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
		"test":      "go test ./pkg -count=1",
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
	if bin := os.Getenv("MUHAR_TEST_BINARY"); bin != "" {
		return bin
	}
	bin := filepath.Join(t.TempDir(), "muhar")
	build := exec.Command("go", "build", "-o", bin, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build muhar: %v\n%s", err, output)
	}
	return bin
}
