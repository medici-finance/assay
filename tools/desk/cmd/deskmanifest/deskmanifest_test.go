package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifest drops one component.yaml at <dir>/<name>/component.yaml.
func writeManifest(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "component.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cleanTree writes a small resolvable, acyclic, in-range set of manifests and
// returns the root. forge-github provides assay.forge (a root); roster provides
// assay.roster.trust; desk provides assay.desk.verbs and requires both in range.
func cleanTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeManifest(t, root, "forge", `component: assay/forge-github
version: 0.28.0
provides:
  - assay.forge
inject:
  required: []
  optional: []
apply: []
`)
	writeManifest(t, root, "roster", `component: assay/roster
version: 0.28.0
provides:
  - assay.roster.trust
inject:
  required: []
apply: []
`)
	writeManifest(t, root, "desk", `component: assay/desk-tools
version: 0.28.0
provides:
  - assay.desk.verbs
inject:
  required:
    - key: assay.forge
      range: ">=0.20.0 <1.0.0"
    - key: assay.roster.trust
  optional:
    - key: assay.harness
apply: []
`)
	return root
}

func TestLint_Clean(t *testing.T) {
	report, code := lint(cleanTree(t))
	if code != exitClean {
		t.Fatalf("clean tree exit = %d, want 0; report:\n%s", code, report)
	}
	lines := strings.Split(strings.TrimRight(report, "\n"), "\n")
	if last := lines[len(lines)-1]; last != "checked-clean" {
		t.Errorf("clean tree last line = %q, want exactly \"checked-clean\"; report:\n%s", last, report)
	}
}

func TestLint_UnresolvedRequired(t *testing.T) {
	root := cleanTree(t)
	// Mutation mirroring Verify row 4: add an unresolved required key.
	writeManifest(t, root, "desk", `component: assay/desk-tools
version: 0.28.0
provides:
  - assay.desk.verbs
inject:
  required:
    - key: assay.forge
    - key: assay.nonexistent
apply: []
`)
	report, code := lint(root)
	if code != exitProblems {
		t.Fatalf("unresolved key exit = %d, want 1; report:\n%s", code, report)
	}
	if !strings.Contains(report, "assay.nonexistent") || !strings.Contains(report, "assay/desk-tools") {
		t.Errorf("report must name the unresolved key and the manifest:\n%s", report)
	}
}

func TestLint_Cycle(t *testing.T) {
	root := t.TempDir()
	// Mutation mirroring Verify row 5: two manifests require each other's key.
	writeManifest(t, root, "a", `component: assay/a
version: 0.28.0
provides:
  - assay.a
inject:
  required:
    - key: assay.b
apply: []
`)
	writeManifest(t, root, "b", `component: assay/b
version: 0.28.0
provides:
  - assay.b
inject:
  required:
    - key: assay.a
apply: []
`)
	report, code := lint(root)
	if code != exitProblems {
		t.Fatalf("cycle exit = %d, want 1; report:\n%s", code, report)
	}
	if !strings.Contains(report, "cycle") || !strings.Contains(report, "assay/a") || !strings.Contains(report, "assay/b") {
		t.Errorf("report must name the cycle with both ids:\n%s", report)
	}
}

func TestLint_OptionalEdgeNeverCycles(t *testing.T) {
	root := t.TempDir()
	// a requires b; b only OPTIONALLY injects a — no cycle should be reported.
	writeManifest(t, root, "a", `component: assay/a
version: 0.28.0
provides:
  - assay.a
inject:
  required:
    - key: assay.b
apply: []
`)
	writeManifest(t, root, "b", `component: assay/b
version: 0.28.0
provides:
  - assay.b
inject:
  required: []
  optional:
    - key: assay.a
apply: []
`)
	report, code := lint(root)
	if code != exitClean {
		t.Fatalf("optional back-edge must not cycle; exit = %d, report:\n%s", code, report)
	}
}

func TestLint_OutOfRange(t *testing.T) {
	root := cleanTree(t)
	// Mutation mirroring Verify row 6: require a range no 0.x provider meets.
	writeManifest(t, root, "desk", `component: assay/desk-tools
version: 0.28.0
provides:
  - assay.desk.verbs
inject:
  required:
    - key: assay.forge
      range: ">=99.0.0"
    - key: assay.roster.trust
apply: []
`)
	report, code := lint(root)
	if code != exitProblems {
		t.Fatalf("out-of-range exit = %d, want 1; report:\n%s", code, report)
	}
	if !strings.Contains(report, "range") || !strings.Contains(report, "assay.forge") {
		t.Errorf("report must name the out-of-range provider:\n%s", report)
	}
}

func TestLint_NamespaceViolation(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "bad", `component: assay/bad
version: 0.28.0
provides:
  - notassay.key
inject:
  required: []
apply: []
`)
	report, code := lint(root)
	if code != exitProblems {
		t.Fatalf("namespace violation exit = %d, want 1; report:\n%s", code, report)
	}
	if !strings.Contains(report, "namespace") || !strings.Contains(report, "notassay.key") {
		t.Errorf("report must flag the out-of-namespace provide:\n%s", report)
	}
}

func TestLint_DuplicateID(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "one", `component: assay/dup
version: 0.28.0
provides: []
inject:
  required: []
apply: []
`)
	writeManifest(t, root, "two", `component: assay/dup
version: 0.28.0
provides: []
inject:
  required: []
apply: []
`)
	report, code := lint(root)
	if code != exitProblems {
		t.Fatalf("duplicate id exit = %d, want 1; report:\n%s", code, report)
	}
	if !strings.Contains(report, "duplicate component id") {
		t.Errorf("report must flag the duplicate id:\n%s", report)
	}
}

func TestLint_MissingRequiredKey(t *testing.T) {
	root := t.TempDir()
	// No `apply:` key at all — §2 requires it even when empty.
	writeManifest(t, root, "x", `component: assay/x
version: 0.28.0
provides: []
inject:
  required: []
`)
	report, code := lint(root)
	if code != exitProblems {
		t.Fatalf("missing key exit = %d, want 1; report:\n%s", code, report)
	}
	if !strings.Contains(report, "apply") || !strings.Contains(report, "missing required key") {
		t.Errorf("report must flag the missing required key:\n%s", report)
	}
}

func TestLint_MissingLedgerOnOutsideStep(t *testing.T) {
	root := t.TempDir()
	// Mutation mirroring the ledger-lint Verify row: an outside step with
	// no ledger: value.
	writeManifest(t, root, "x", `component: assay/x
version: 0.28.0
provides:
  - assay.x
inject:
  required: []
apply:
  - id: outside-step
    effect: create something on the forge
    boundary: outside
    compensation: list-for-human
`)
	report, code := lint(root)
	if code != exitProblems {
		t.Fatalf("missing ledger on outside step exit = %d, want 1; report:\n%s", code, report)
	}
	if !strings.Contains(report, "outside-step") || !strings.Contains(report, "assay/x") || !strings.Contains(report, "ledger") {
		t.Errorf("report must name the step, the component, and mention ledger:\n%s", report)
	}
}

func TestLint_OutsideStepWithLedgerIsClean(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "x", `component: assay/x
version: 0.28.0
provides:
  - assay.x
inject:
  required: []
apply:
  - id: outside-step
    effect: create something on the forge
    boundary: outside
    ledger: thing
    compensation: list-for-human
`)
	_, code := lint(root)
	if code != exitClean {
		t.Fatalf("outside step with a ledger: value must be clean; code=%d", code)
	}
}

func TestLint_InsideStepNeedsNoLedger(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "x", `component: assay/x
version: 0.28.0
provides:
  - assay.x
inject:
  required: []
apply:
  - id: inside-step
    effect: create a local file
    boundary: inside
    inverse: remove the local file
`)
	_, code := lint(root)
	if code != exitClean {
		t.Fatalf("an inside step must never be flagged for a missing ledger:; code=%d", code)
	}
}

func TestLint_CouldNotCheckMissingRoot(t *testing.T) {
	report, code := lint(filepath.Join(t.TempDir(), "does-not-exist"))
	if code != exitCouldNotCheck {
		t.Fatalf("missing root exit = %d, want 2; report:\n%s", code, report)
	}
	if !strings.HasPrefix(report, "could-not-check") {
		t.Errorf("missing root report must start could-not-check:\n%s", report)
	}
}

func TestLint_EmptyTreeIsCleanNotCouldNotCheck(t *testing.T) {
	// A readable tree with zero manifests: we DID look and found nothing —
	// clean, never could-not-check.
	report, code := lint(t.TempDir())
	if code != exitClean {
		t.Fatalf("empty readable tree exit = %d, want 0 (clean); report:\n%s", code, report)
	}
}

func TestSatisfies(t *testing.T) {
	cases := []struct {
		version, rng string
		want         bool
	}{
		{"0.28.0", ">=0.20.0 <1.0.0", true},
		{"0.28.0", ">=99.0.0", false},
		{"1.0.0", ">=0.20.0 <1.0.0", false},
		{"0.28.0", "", true},
		{"v0.28.0", ">=0.28.0", true},
		{"0.28.0-rc1", ">=0.28.0", true}, // pre-release suffix ignored for comparison
		{"0.19.9", ">=0.20.0", false},
	}
	for _, c := range cases {
		got, err := satisfies(c.version, c.rng)
		if err != nil {
			t.Errorf("satisfies(%q,%q) error: %v", c.version, c.rng, err)
			continue
		}
		if got != c.want {
			t.Errorf("satisfies(%q,%q) = %v, want %v", c.version, c.rng, got, c.want)
		}
	}
}

func TestRun_Version(t *testing.T) {
	var out, errb strings.Builder
	if code := run([]string{"--version"}, &out, &errb); code != exitClean {
		t.Fatalf("--version exit = %d, want 0", code)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out, errb strings.Builder
	if code := run([]string{"bogus"}, &out, &errb); code != exitCouldNotCheck {
		t.Fatalf("unknown command exit = %d, want 2", code)
	}
}
