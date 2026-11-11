package regression

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// cellctlSuiteFloor runs one cellctl shell suite against the shell oracle (the suite's default
// CELLCTL), from a throwaway working directory, and requires the suite's own "<name>: OK" line.
// No workflow ran harness.test.sh or provider.test.sh, so their expectations drifted from the
// oracle and stayed red on a development host unnoticed (#6175). The working directory is a
// fresh t.TempDir(), never the package directory: a suite whose scratch-directory setup fails
// must not be able to delete the tree it runs from.
func cellctlSuiteFloor(t *testing.T, relative string) {
	t.Helper()
	skipWindowsShellFixture(t)
	bash := requireBash(t)
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	suite := filepath.Join(root, filepath.FromSlash(relative))
	t.Chdir(t.TempDir())
	out, err := runShellFixtureWithBash(t, bash, suite, t.TempDir(), shellBudget)
	if want := "\n" + path.Base(relative) + ": OK\n"; err != nil || !strings.Contains(string(out), want) {
		t.Fatalf("fixture suite %s: %v\n%s", relative, err, out)
	}
	t.Log(string(out))
}

// TestReg6175HarnessSuite holds harness.test.sh to the oracle's `check` rows, which read the
// Codex multi_agent state from `codex features list` since ea4f9842d.
//
// regression: #6175
func TestReg6175HarnessSuite(t *testing.T) {
	cellctlSuiteFloor(t, "tools/cellctl/tests/harness.test.sh")
}

// TestReg6175ProviderSuite holds provider.test.sh to the oracle. The suite's "no provider" rows
// must hold on a host whose own shell already runs claude against a provider.
//
// regression: #6175
func TestReg6175ProviderSuite(t *testing.T) {
	cellctlSuiteFloor(t, "tools/cellctl/tests/provider.test.sh")
}

// cellctlSuiteRunners names, for each tools/cellctl/tests suite a `go test` runs, every test that
// runs it, as "<package dir>:<TestName>". The named function must exist in that package and its
// file must name the suite.
var cellctlSuiteRunners = map[string][]string{
	"down.test.sh":               {"tools/desk/cmd/cellctl:TestDownShellSuite"},
	"gen-shims-gh-token.test.sh": {"tools/desk/internal/regression:TestReg1145ShimCredential"},
	"harness.test.sh":            {"tools/desk/internal/regression:TestReg6175HarnessSuite", "tools/desk/cmd/cellctl:TestHarnessShellSuite"},
	"provider.test.sh":           {"tools/desk/internal/regression:TestReg6175ProviderSuite"},
	"role-context.test.sh":       {"tools/desk/cmd/cellctl:TestRoleContextShellSuite"},
	"scrubbed-cell.test.sh":      {"tools/desk/cmd/cellctl:TestScrubbedCellShellSuite"},
}

// unrunCellctlSuites is every suite no `go test` runs yet. It only shrinks: a suite leaves it in
// the PR that gives it a runner, and a new suite gets a runner instead of an entry. The
// fourteen below, and the red rows a first run of them showed, are tracked in #6185.
var unrunCellctlSuites = map[string]bool{
	"cell-set.test.sh": true, "cockpit.test.sh": true, "container-cell.test.sh": true,
	"desk-worktree-merge.test.sh": true, "gitlab-fetch-cred.test.sh": true,
	"herdr-orca-launch.test.sh": true, "house-cell.test.sh": true, "model-namespace.test.sh": true,
	"model-override.test.sh": true, "model-pin.test.sh": true, "parity.test.sh": true,
	"postgres-verify.test.sh": true, "root-drift.test.sh": true, "version-stamp.test.sh": true,
}

// cellctlSuiteFaults returns one line per suite in suites that no runner covers or that sits in
// both tables, per runner entry that does not resolve to a test naming its suite, and per stale
// entry in either table. testFiles maps a package dir to its *_test.go sources by file name.
func cellctlSuiteFaults(suites []string, runners map[string][]string, unrun map[string]bool, testFiles map[string]map[string][]byte) []string {
	var faults []string
	present := map[string]bool{}
	for _, s := range suites {
		present[s] = true
		_, run := runners[s]
		switch {
		case run && unrun[s]:
			faults = append(faults, fmt.Sprintf("%s: has a runner, so drop it from unrunCellctlSuites", s))
		case !run && !unrun[s]:
			faults = append(faults, fmt.Sprintf("%s: no go test runs this suite; add a runner and list it in cellctlSuiteRunners", s))
		}
	}
	for s, names := range runners {
		if !present[s] {
			faults = append(faults, fmt.Sprintf("%s: in cellctlSuiteRunners but not in tools/cellctl/tests", s))
			continue
		}
		for _, entry := range names {
			pkg, name, ok := strings.Cut(entry, ":")
			if !ok || !runnerNamesSuite(testFiles[pkg], name, s) {
				faults = append(faults, fmt.Sprintf("%s: runner %s is not a test in a file that names the suite", s, entry))
			}
		}
	}
	for s := range unrun {
		if !present[s] {
			faults = append(faults, fmt.Sprintf("%s: in unrunCellctlSuites but not in tools/cellctl/tests", s))
		}
	}
	sort.Strings(faults)
	return faults
}

// runnerNamesSuite reports whether a top-level func name is declared in one of files whose
// source also names suite.
func runnerNamesSuite(files map[string][]byte, name, suite string) bool {
	for file, src := range files {
		f, err := parser.ParseFile(token.NewFileSet(), file, src, 0)
		if err != nil || !strings.Contains(string(src), `"`+suite+`"`) && !strings.Contains(string(src), "/"+suite+`"`) {
			continue
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
				return true
			}
		}
	}
	return false
}

// TestCellctlSuitesHaveRunners is the class guard for #6175: a cellctl shell suite that no `go
// test` runs drifts from both implementations unseen. Every suite has a named runner or an entry
// in the shrinking unrun list. Planted controls keep the matcher honest.
//
// regression: #6175
func TestCellctlSuitesHaveRunners(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "tools", "cellctl", "tests", "*.test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	var suites []string
	for _, p := range paths {
		suites = append(suites, filepath.Base(p))
	}
	if len(suites) < 20 {
		t.Fatalf("found %d suites under tools/cellctl/tests, want at least 20", len(suites))
	}
	testFiles := map[string]map[string][]byte{}
	for _, names := range cellctlSuiteRunners {
		for _, entry := range names {
			pkg, _, _ := strings.Cut(entry, ":")
			if testFiles[pkg] != nil {
				continue
			}
			testFiles[pkg] = map[string][]byte{}
			files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pkg), "*_test.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				src, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				testFiles[pkg][filepath.Base(file)] = src
			}
		}
	}
	if faults := cellctlSuiteFaults(suites, cellctlSuiteRunners, unrunCellctlSuites, testFiles); len(faults) > 0 {
		t.Errorf("%s", strings.Join(faults, "\n"))
	}

	plantedFiles := map[string]map[string][]byte{"p": {
		"a_test.go": []byte("package p\nfunc TestRunsA(t *T) { run(\"a.test.sh\") }\n"),
		"b_test.go": []byte("package p\nfunc TestRunsNothing(t *T) {}\n"),
	}}
	got := cellctlSuiteFaults(
		[]string{"a.test.sh", "b.test.sh", "plant.test.sh"},
		map[string][]string{"a.test.sh": {"p:TestRunsA"}, "b.test.sh": {"p:TestRunsNothing"}},
		map[string]bool{"gone.test.sh": true},
		plantedFiles)
	want := []string{
		"b.test.sh: runner p:TestRunsNothing is not a test in a file that names the suite",
		"gone.test.sh: in unrunCellctlSuites but not in tools/cellctl/tests",
		"plant.test.sh: no go test runs this suite; add a runner and list it in cellctlSuiteRunners",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("planted controls:\ngot  %q\nwant %q", got, want)
	}
}
