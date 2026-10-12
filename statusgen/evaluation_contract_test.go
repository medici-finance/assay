package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// evaluation_contract_test.go — the contract of the statusgen/evaluation
// library (statusgen/15): the library, fed the frozen inputs of
// testdata/evaluation/scenarios.json directly, returns the same verdicts and
// reasons as the statusgen CLI reading the materialized trees; uncertain
// evidence holds; the library's import graph stays pure; and a module outside
// this repository can build against and exercise both entry points.
//
// Eligibility grants no admission and coverage no publication: every check
// here reads verdicts, none of them flips a status or writes a file.

// ---- hand-built library inputs ---------------------------------------------

// libEligibilityInput builds the library input for a scenario directly from
// its description, never through the statusgen loaders. parent is the
// directory the scenario was materialized under, so a sibling's reported
// location reads like the CLI's.
func libEligibilityInput(sc eligScenario, parent string) evaluation.EligibilityInput {
	toStreams := func(in []eligStrm) []evaluation.Stream {
		var out []evaluation.Stream
		for _, s := range in {
			es := evaluation.Stream{Name: s.Name}
			for _, b := range s.Briefs {
				eb := evaluation.Brief{
					Num:     b.Num,
					Status:  b.Status,
					Depends: b.Depends,
					// A human-gated brief at implemented with a recorded
					// passing verdict satisfies a depends: edge.
					HumanGatePassRecorded: b.HumanPass && b.Gate == "human" && b.Status == "implemented",
				}
				for _, g := range b.Gates {
					eb.Gates = append(eb.Gates, evaluation.Edge{Ref: g.On, Type: g.Type, Reason: g.Reason})
				}
				for _, f := range b.Feathers {
					eb.Feathers = append(eb.Feathers, evaluation.Edge{Ref: f, Type: "build-dep"})
				}
				es.Briefs = append(es.Briefs, eb)
			}
			out = append(out, es)
		}
		return out
	}
	in := evaluation.EligibilityInput{Streams: toStreams(sc.Streams)}
	if sc.Registry != nil {
		reg := &evaluation.Registry{Aliases: map[string]evaluation.RegistryAlias{}}
		for name, a := range sc.Registry {
			reg.Aliases[name] = evaluation.RegistryAlias{Repo: a.Repo, Unpublished: a.Unpublished}
		}
		in.Registry = reg
	}
	in.Siblings = func(repo string) evaluation.Sibling {
		name := repo
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		where := filepath.Join(parent, name)
		streams, ok := sc.Siblings[name]
		if !ok {
			return evaluation.Sibling{Where: where}
		}
		return evaluation.Sibling{Found: true, Where: where, Streams: toStreams(streams)}
	}
	return in
}

// libCoverageInput builds the library input for a materialized coverage
// scenario from its description.
func libCoverageInput(t *testing.T, sc covScenario, b builtCoverage) (evaluation.CoverageInput, evaluation.Repo) {
	t.Helper()
	var brief *covBrief
	for i := range sc.Steps {
		if sc.Steps[i].Brief != nil {
			brief = sc.Steps[i].Brief // the last brief step is the final tree's brief
		}
	}
	in := evaluation.CoverageInput{}
	var repo evaluation.Repo
	if !sc.NoGit {
		in.Revision = gitHeadFull(t, b.Root)[:evaluation.TreeSHALen]
		repo = newCoverageRepo(b.Root)
	}
	obs := evaluation.BriefObservation{
		ID:        "cov/01",
		BriefRel:  "docs/streams/cov/brief-01.md",
		Witnesses: map[string][]evaluation.Witness{},
	}
	if brief != nil {
		if brief.Files != "" {
			obs.DeclaredFilesFound = true
			for _, e := range strings.Split(brief.Files, ",") {
				obs.DeclaredFiles = append(obs.DeclaredFiles, strings.Trim(strings.TrimSpace(e), "`"))
			}
		}
		for _, r := range brief.Verify {
			row := evaluation.VerifyRow{ID: r.ID, Command: r.Command, Expect: r.Expect}
			if strings.Contains(r.Class, "+flow") {
				row.Obligations = []string{evaluation.ObligationFlow}
			}
			obs.VerifyRows = append(obs.VerifyRows, row)
		}
		for _, w := range brief.Witnesses {
			obs.Witnesses[w.Row] = append(obs.Witnesses[w.Row], evaluation.Witness{
				Command: w.Command, State: w.State, Tree: resolveTree(b.Labels, w.Tree),
			})
		}
	}
	in.Briefs = []evaluation.BriefObservation{obs}
	if sc.Options != nil {
		if sc.Options.Bindings != nil {
			in.Bindings = map[string]evaluation.Binding{}
			for id, bd := range sc.Options.Bindings {
				in.Bindings[id] = evaluation.Binding{Pattern: bd.Pattern, Node: bd.Node}
			}
		}
		if sc.Options.Patterns != nil {
			in.Patterns = map[string]evaluation.Pattern{}
			for name, p := range sc.Options.Patterns {
				pat := evaluation.Pattern{Join: p.Join, Nodes: map[string]evaluation.PatternNode{}}
				for id, n := range p.Nodes {
					node := evaluation.PatternNode{}
					for _, e := range n.Evidence {
						node.Evidence = append(node.Evidence, evaluation.PatternEvidence{Kind: e.Kind, Claim: e.Claim, Mandatory: e.Mandatory, Source: e.Source})
					}
					pat.Nodes[id] = node
				}
				in.Patterns[name] = pat
			}
		}
		if sc.Options.Dependencies != nil {
			in.Dependencies = map[string]evaluation.DependencyManifest{}
			for id, d := range sc.Options.Dependencies {
				m := evaluation.DependencyManifest{Complete: d.Complete}
				for _, dep := range d.Deps {
					m.Deps = append(m.Deps, evaluation.WorkInputDep{Kind: dep.Kind, Path: dep.Path, Name: dep.Name, Recorded: dep.Recorded, Current: dep.Current, Rows: dep.Rows})
				}
				in.Dependencies[id] = m
			}
		}
	}
	return in, repo
}

// asEligibilityRows renders a library verdict map the way the CLI's --json
// does: one row per brief, in id order.
func asEligibilityRows(m map[string]evaluation.Eligibility) []evaluation.Eligibility {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows := make([]evaluation.Eligibility, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, m[id])
	}
	return rows
}

func asCoverageRows(m map[string]evaluation.Coverage) []evaluation.Coverage {
	rows := make([]evaluation.Coverage, 0, len(m))
	for _, id := range evaluation.SortedIDs(m) {
		rows = append(rows, m[id])
	}
	return rows
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// buildStatusgenBinary builds this package's CLI once for a test.
func buildStatusgenBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "statusgen")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runCLIJSON(t *testing.T, bin, root, mode string) string {
	t.Helper()
	cmd := exec.Command(bin, "--root", root, mode, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("statusgen %s: %v\n%s", mode, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

// ---- parity ----------------------------------------------------------------

// regression: statusgen/15 — one frozen input, two consumers (the CLI over a
// materialized tree; the library over hand-built observations), one verdict.
func TestEvaluationLibraryCLIParity(t *testing.T) {
	sf := loadScenarios(t)
	golden := readGolden(t)
	bin := buildStatusgenBinary(t)

	for _, sc := range sf.Eligibility {
		root := materializeEligibility(t, sc)
		parent := filepath.Dir(root)
		lib := evaluation.EvaluateEligibility(libEligibilityInput(sc, parent))

		// The CLI, run as a binary over the materialized tree.
		cli := runCLIJSON(t, bin, root, "--eligibility")
		if got := mustJSON(t, asEligibilityRows(lib)); got != cli {
			t.Errorf("eligibility %s: library and CLI disagree\nlibrary: %s\n    cli: %s", sc.Name, got, cli)
		}
		// Reasons (including the could-not-check detail the wire omits) match
		// the verdicts frozen before the extraction.
		raw, _ := json.Marshal(eligRecords(lib))
		var norm []eligRecord
		if err := json.Unmarshal([]byte(normalizeRevs(string(raw), nil, parent)), &norm); err != nil {
			t.Fatal(err)
		}
		if string(marshalCanonical(t, norm)) != string(marshalCanonical(t, golden.Eligibility[sc.Name])) {
			t.Errorf("eligibility %s: library verdicts drifted from the frozen ones:\n got: %s\nwant: %s", sc.Name, marshalCanonical(t, norm), marshalCanonical(t, golden.Eligibility[sc.Name]))
		}
	}

	for _, sc := range sf.Coverage {
		b := materializeCoverage(t, sc)
		in, repo := libCoverageInput(t, sc, b)
		lib := evaluation.EvaluateCoverage(in, repo)
		libRows := normalizedCoverage(t, lib, b.Labels, b.Root)

		want := golden.Coverage[sc.Name]
		if string(marshalCanonical(t, libRows)) != string(marshalCanonical(t, want)) {
			t.Errorf("coverage %s: library verdicts differ from the frozen ones:\n got: %s\nwant: %s", sc.Name, marshalCanonical(t, libRows), marshalCanonical(t, want))
		}
		// The in-process CLI path agrees too (it also carries the option-only
		// scenarios, which the command line has no flag for).
		streams, _, err := loadHydratedStreams(b.Root)
		if err != nil {
			t.Fatalf("%s: %v", sc.Name, err)
		}
		cliRows := normalizedCoverage(t, evaluateCoverage(b.Root, streams, b.Opts), b.Labels, b.Root)
		if string(marshalCanonical(t, libRows)) != string(marshalCanonical(t, cliRows)) {
			t.Errorf("coverage %s: library and statusgen path disagree\nlibrary: %s\n    cli: %s", sc.Name, marshalCanonical(t, libRows), marshalCanonical(t, cliRows))
		}
		// And the binary, for the scenarios the command line can express.
		if sc.Options == nil {
			cli := runCLIJSON(t, bin, b.Root, "--coverage")
			if got := mustJSON(t, asCoverageRows(lib)); got != cli {
				t.Errorf("coverage %s: library and CLI binary disagree\nlibrary: %s\n    cli: %s", sc.Name, got, cli)
			}
		}
	}
}

// ---- uncertain evidence ------------------------------------------------------

// regression: statusgen/15 — evidence that is missing, recorded against
// another revision, stale or unresolved must HOLD; a known state is a
// different verdict from an unknown one. Treating "cannot tell" as satisfied
// would release work on a guess.
func TestEvaluationUncertainEvidenceHolds(t *testing.T) {
	sf := loadScenarios(t)
	covByName := map[string]covScenario{}
	for _, sc := range sf.Coverage {
		covByName[sc.Name] = sc
	}
	soleRow := func(name string) (evaluation.Coverage, evaluation.Claim) {
		sc, ok := covByName[name]
		if !ok {
			t.Fatalf("scenario %s is not in scenarios.json", name)
		}
		b := materializeCoverage(t, sc)
		in, repo := libCoverageInput(t, sc, b)
		c := evaluation.EvaluateCoverage(in, repo)["cov/01"]
		if len(c.Claims) == 0 {
			t.Fatalf("%s: no claims", name)
		}
		return c, c.Claims[0]
	}

	held := []struct{ name, result string }{
		{"missing-witness", evaluation.ResultMissing},
		{"other-path-changed-wrong-revision", evaluation.ResultWrongRevision},
		{"declared-files-without-manifest-conservative", evaluation.ResultWrongRevision},
		{"policy-dependency-changed", evaluation.ResultWrongRevision},
		{"witness-commit-absent-from-clone", evaluation.ResultCouldNotCheck},
		{"witness-token-too-short", evaluation.ResultCouldNotCheck},
		{"witness-without-git-history", evaluation.ResultCouldNotCheck},
		{"witness-could-not-run", evaluation.ResultCouldNotCheck},
		{"environment-dependency-current-unknown", evaluation.ResultCouldNotCheck},
		{"witness-records-failure", evaluation.ResultFail},
		{"command-changed-after-witness", evaluation.ResultError},
	}
	for _, c := range held {
		cov, claim := soleRow(c.name)
		if cov.Released || claim.Released {
			t.Errorf("%s: released on uncertain evidence: %+v", c.name, claim)
		}
		if claim.Result != c.result {
			t.Errorf("%s: result %q, want %q (%s)", c.name, claim.Result, c.result, claim.Reason)
		}
		if claim.Reason == "" {
			t.Errorf("%s: a hold must say why", c.name)
		}
	}
	// A mandatory claim the pattern node demands but the tree cannot
	// corroborate holds the whole brief while its Verify rows pass.
	cov, _ := soleRow("bound-observe-evidence-holds")
	if cov.Released {
		t.Errorf("bound-observe-evidence-holds: released although a mandatory claim is unresolved: %+v", cov)
	}
	var sawUnresolved bool
	for _, cl := range cov.Claims {
		if cl.Result == evaluation.ResultCouldNotCheck {
			sawUnresolved = true
		}
	}
	if !sawUnresolved {
		t.Errorf("bound-observe-evidence-holds: expected an unresolved claim, got %+v", cov.Claims)
	}

	// A passing witness at the item's own revision is still not credited when
	// the Verify row cannot be read as it stood at that revision: the pass may
	// answer a different question than the row asks today.
	exact := evaluation.CoverageInput{
		Revision: "aaaaaaaaaaaa",
		Briefs: []evaluation.BriefObservation{{
			ID:         "s/01",
			BriefRel:   "docs/streams/s/brief-01.md",
			VerifyRows: []evaluation.VerifyRow{{ID: "1", Command: "true", Expect: "exit 0"}},
			Witnesses: map[string][]evaluation.Witness{"1": {
				{Command: "true", State: evaluation.StatePass, Tree: "aaaaaaaaaaaa"},
			}},
		}},
	}
	if c := evaluation.EvaluateCoverage(exact, &memRepo{unreadable: true})["s/01"]; c.Released || c.Claims[0].Result != evaluation.ResultCouldNotCheck {
		t.Errorf("a pass whose row cannot be read at the witness revision was credited: %+v", c)
	}
	if c := evaluation.EvaluateCoverage(exact, &memRepo{})["s/01"]; !c.Released || c.Claims[0].Result != evaluation.ResultPass {
		t.Errorf("control: the same pass with a readable row did not release: %+v", c)
	}

	// The known states are distinct: a witness at the item's revision passes,
	// and a binding that names nothing is ignored (the rows alone decide).
	for _, name := range []string{"witness-at-head-passes", "binding-to-unknown-pattern-is-ignored", "binding-to-unknown-node-is-ignored"} {
		cov, claim := soleRow(name)
		if !cov.Released || claim.Result != evaluation.ResultPass {
			t.Errorf("%s: want a released pass, got %+v", name, cov)
		}
	}

	// Eligibility: an unresolvable or forge-backed edge is could-not-check and
	// holds; a satisfied edge releases; an unsatisfied edge holds under a
	// different state. The three never collapse into one another.
	elig := map[string]evaluation.Eligibility{}
	for _, sc := range sf.Eligibility {
		root := materializeEligibility(t, sc)
		for id, e := range evaluation.EvaluateEligibility(libEligibilityInput(sc, filepath.Dir(root))) {
			elig[sc.Name+":"+id] = e
		}
	}
	wantElig := []struct {
		key     string
		verdict evaluation.EligibilityVerdict
		state   evaluation.EligibilityState
	}{
		{"forge-backed-gates:alpha/02", evaluation.VerdictHeld, evaluation.StateCouldNotCheck},
		{"unresolved-in-repo-gate:alpha/02", evaluation.VerdictHeld, evaluation.StateCouldNotCheck},
		{"absent-sibling-checkout:alpha/02", evaluation.VerdictHeld, evaluation.StateCouldNotCheck},
		{"alias-not-in-registry:alpha/02", evaluation.VerdictHeld, evaluation.StateCouldNotCheck},
		{"in-repo-todo-gate:alpha/02", evaluation.VerdictHeld, evaluation.StateUnsatisfied},
		{"sibling-satisfied-and-missing:alpha/03", evaluation.VerdictHeld, evaluation.StateUnsatisfied},
		{"in-repo-done-gate:alpha/02", evaluation.VerdictEligible, ""},
		{"sibling-satisfied-and-missing:alpha/02", evaluation.VerdictEligible, ""},
	}
	for _, w := range wantElig {
		e, ok := elig[w.key]
		if !ok {
			t.Errorf("%s: no verdict", w.key)
			continue
		}
		if e.Verdict != w.verdict {
			t.Errorf("%s: verdict %q, want %q", w.key, e.Verdict, w.verdict)
		}
		if w.state != "" && (len(e.Holds) == 0 || e.Holds[0].State != w.state) {
			t.Errorf("%s: want the hold in state %q, got %+v", w.key, w.state, e.Holds)
		}
		if w.state != "" && e.Holds[0].Why == "" && w.state == evaluation.StateCouldNotCheck {
			t.Errorf("%s: a could-not-check hold must say why", w.key)
		}
	}
}

// ---- purity ------------------------------------------------------------------

// pureStdAllow is the set of standard-library packages a pure evaluator may
// import directly. A standard package is judged by who imports it, not by what
// IT imports (fmt reaches os), so the boundary is the importer's own list:
// text, collections, hashing and formatting only. Process, network, file,
// environment, clock, randomness, credential and reflection-driven packages
// are absent by construction.
var pureStdAllow = map[string]bool{
	"crypto/sha256": true,
	"encoding/hex":  true,
	"errors":        true,
	"fmt":           true,
	"path":          true,
	"regexp":        true,
	"sort":          true,
	"strconv":       true,
	"strings":       true,
	"unicode/utf8":  true,
}

type listedPackage struct {
	ImportPath string
	Standard   bool
	Imports    []string
}

// goListDeps lists dir's package and every dependency, transitively.
func goListDeps(t *testing.T, dir, pattern string, env ...string) map[string]listedPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-json", pattern)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), append([]string{"GOWORK=off", "GOFLAGS=-mod=mod"}, env...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps %s in %s: %v\n%s", pattern, dir, err, stderr.String())
	}
	pkgs := map[string]listedPackage{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p listedPackage
		if err := dec.Decode(&p); err != nil {
			t.Fatal(err)
		}
		pkgs[p.ImportPath] = p
	}
	return pkgs
}

// boundaryViolations walks the import graph from root and reports every
// non-standard package that directly imports a standard package outside allow,
// with the chain that reaches it. A non-standard package is code someone wrote:
// an indirect import of os/exec through a helper is as impure as a direct one.
func boundaryViolations(pkgs map[string]listedPackage, root string, allow map[string]bool) []string {
	var found []string
	seen := map[string]bool{}
	var walk func(path string, chain []string)
	walk = func(path string, chain []string) {
		if seen[path] {
			return
		}
		seen[path] = true
		p, ok := pkgs[path]
		if !ok || p.Standard {
			return
		}
		chain = append(chain, path)
		for _, imp := range p.Imports {
			ip, known := pkgs[imp]
			if known && ip.Standard {
				if !allow[imp] {
					found = append(found, fmt.Sprintf("%s imports %s (via %s)", path, imp, strings.Join(chain, " -> ")))
				}
				continue
			}
			if imp == "C" || imp == "unsafe" {
				found = append(found, fmt.Sprintf("%s imports %s (via %s)", path, imp, strings.Join(chain, " -> ")))
				continue
			}
			walk(imp, chain)
		}
	}
	walk(root, nil)
	sort.Strings(found)
	return found
}

func writeTreeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		writeScenarioFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
}

// regression: statusgen/15 — the evaluator must stay free of process,
// network, filesystem and credential dependencies, directly and through any
// package it pulls in.
func TestEvaluationPureDependencyBoundary(t *testing.T) {
	// The real library: nothing outside the allowlist, and no package of this
	// repository other than itself.
	pkgs := goListDeps(t, ".", "./evaluation")
	const self = "github.com/medici-finance/assay/statusgen/evaluation"
	if _, ok := pkgs[self]; !ok {
		t.Fatalf("go list did not report %s: %v", self, keys(pkgs))
	}
	if v := boundaryViolations(pkgs, self, pureStdAllow); len(v) > 0 {
		t.Errorf("the evaluation library is not pure:\n  %s", strings.Join(v, "\n  "))
	}
	for path, p := range pkgs {
		if !p.Standard && path != self {
			t.Errorf("the evaluation library depends on %s; it may import the standard library only", path)
		}
	}

	// A planted INDIRECT violation: the library-shaped package imports a
	// helper, and the helper starts a process. The same walker must refuse it.
	planted := t.TempDir()
	writeTreeFiles(t, planted, map[string]string{
		"go.mod":           "module example.test/planted\n\ngo 1.25.0\n",
		"lib/lib.go":       "package lib\n\nimport \"example.test/planted/helper\"\n\nfunc Verdict() string { return helper.Name() }\n",
		"helper/helper.go": "package helper\n\nimport \"example.test/planted/deeper\"\n\nfunc Name() string { return deeper.Name() }\n",
		"deeper/deeper.go": "package deeper\n\nimport \"os/exec\"\n\nfunc Name() string { out, _ := exec.Command(\"git\", \"--version\").Output(); return string(out) }\n",
	})
	v := boundaryViolations(goListDeps(t, planted, "./lib"), "example.test/planted/lib", pureStdAllow)
	if len(v) != 1 || !strings.Contains(v[0], "example.test/planted/deeper imports os/exec") || !strings.Contains(v[0], "lib -> example.test/planted/helper -> example.test/planted/deeper") {
		t.Errorf("a planted indirect os/exec import was not refused with its chain: %v", v)
	}
	for _, std := range []string{"net/http", "os", "syscall", "os/user", "io/ioutil", "net"} {
		dir := t.TempDir()
		writeTreeFiles(t, dir, map[string]string{
			"go.mod":      "module example.test/planted\n\ngo 1.25.0\n",
			"lib/lib.go":  "package lib\n\nimport _ \"example.test/planted/helper\"\n",
			"helper/h.go": "package helper\n\nimport _ \"" + std + "\"\n",
		})
		if v := boundaryViolations(goListDeps(t, dir, "./lib"), "example.test/planted/lib", pureStdAllow); len(v) != 1 {
			t.Errorf("a planted %s import was not refused: %v", std, v)
		}
	}

	// The pure control passes.
	control := t.TempDir()
	writeTreeFiles(t, control, map[string]string{
		"go.mod":         "module example.test/control\n\ngo 1.25.0\n",
		"lib/lib.go":     "package lib\n\nimport (\n\t\"example.test/control/helper\"\n\t\"strings\"\n)\n\nfunc Verdict() string { return strings.ToUpper(helper.Name()) }\n",
		"helper/help.go": "package helper\n\nimport \"sort\"\n\nfunc Name() string { s := []string{\"b\", \"a\"}; sort.Strings(s); return s[0] }\n",
	})
	if v := boundaryViolations(goListDeps(t, control, "./lib"), "example.test/control/lib", pureStdAllow); len(v) != 0 {
		t.Errorf("the pure control was refused: %v", v)
	}
}

func keys(m map[string]listedPackage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// memRepo is an in-memory Repo: frozen observations, no git. Every call is
// recorded so a test can see exactly what the evaluator asked.
type memRepo struct {
	calls []string
	// unreadable makes the historical Verify row unreadable at any revision
	// (the brief or the row did not exist there, or there is no history).
	unreadable bool
}

func (m *memRepo) note(s string) { m.calls = append(m.calls, s) }

func (m *memRepo) InRepository() bool { m.note("InRepository"); return true }
func (m *memRepo) ResolveObject(tok string) (string, bool) {
	m.note("ResolveObject " + tok)
	return "", false
}
func (m *memRepo) ResolveCommit(tok string) (string, bool) {
	m.note("ResolveCommit " + tok)
	return "", false
}
func (m *memRepo) IsShallow() (bool, bool) { m.note("IsShallow"); return false, true }
func (m *memRepo) MergeBase(a, b string) evaluation.History {
	m.note("MergeBase")
	return evaluation.HistoryUnknown
}
func (m *memRepo) MergeBaseAll(a, b string) ([]string, bool) {
	m.note("MergeBaseAll")
	return nil, false
}
func (m *memRepo) DiffNames(a, b string) ([]string, bool) { m.note("DiffNames"); return nil, false }
func (m *memRepo) RevList(tip string, ex []string, n int) ([]string, bool) {
	m.note("RevList")
	return nil, false
}
func (m *memRepo) TreeFiles(rev string) []string { m.note("TreeFiles"); return nil }
func (m *memRepo) TreeModes(rev string) ([]evaluation.TreeEntry, bool) {
	m.note("TreeModes")
	return nil, false
}
func (m *memRepo) ObjectAt(rev, p string) (string, bool) { m.note("ObjectAt"); return "", false }
func (m *memRepo) RootPrefix() (string, bool)            { m.note("RootPrefix"); return "", true }
func (m *memRepo) VerifyExpectAt(rev, rel, row string) (string, bool) {
	m.note("VerifyExpectAt")
	if m.unreadable {
		return "", false
	}
	return "exit 0", true
}
func (m *memRepo) DeclaredFilesAt(rev, rel string) ([]string, bool, bool) {
	m.note("DeclaredFilesAt")
	return nil, false, false
}

// regression: statusgen/15 — an offline flow with every ambient effect
// removed: no PATH (no process can start), no HOME, no credentials, an empty
// working directory. The verdicts are the same, and nothing is written.
func TestEvaluationOfflineFlow(t *testing.T) {
	sf := loadScenarios(t)
	var want []string
	var inputs []evaluation.EligibilityInput
	for _, sc := range sf.Eligibility {
		root := materializeEligibility(t, sc)
		in := libEligibilityInput(sc, filepath.Dir(root))
		inputs = append(inputs, in)
		want = append(want, mustJSON(t, asEligibilityRows(evaluation.EvaluateEligibility(in))))
	}

	work := t.TempDir()
	t.Chdir(work)
	t.Setenv("PATH", "")
	t.Setenv("HOME", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	repo := &memRepo{}
	for i, in := range inputs {
		// Siblings are an observation the caller supplied; nothing is opened.
		got := mustJSON(t, asEligibilityRows(evaluation.EvaluateEligibility(in)))
		if got != want[i] {
			t.Errorf("eligibility scenario %d changed once the ambient environment was removed", i)
		}
	}
	cov := evaluation.EvaluateCoverage(evaluation.CoverageInput{
		Revision: "aaaaaaaaaaaa",
		Briefs: []evaluation.BriefObservation{{
			ID:         "s/01",
			BriefRel:   "docs/streams/s/brief-01.md",
			VerifyRows: []evaluation.VerifyRow{{ID: "1", Command: "true", Expect: "exit 0"}},
			Witnesses: map[string][]evaluation.Witness{"1": {
				{Command: "true", State: evaluation.StatePass, Tree: "aaaaaaaaaaaa"},
			}},
		}},
	}, repo)
	if c := cov["s/01"]; !c.Released || len(c.Claims) != 1 || c.Claims[0].Result != evaluation.ResultPass {
		t.Errorf("a witness at the item's revision did not pass: %+v", c)
	}
	// Coverage may ask the Repo; it asked only typed questions of it.
	entries, err := os.ReadDir(work)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the evaluators wrote to the working directory: %v", entries)
	}
	for _, c := range repo.calls {
		if c != "VerifyExpectAt" {
			t.Errorf("an exact-revision pass asked the repository more than the row's historical Expect: %q", c)
		}
	}
}

// ---- identity ----------------------------------------------------------------

// regression: statusgen/15 — same input, same snapshot, evaluator version and
// policy digest: the evaluator names itself so a verdict can be tied to the
// rules that produced it.
func TestEvaluationIdentityStable(t *testing.T) {
	a, b := evaluation.CurrentIdentity(), evaluation.CurrentIdentity()
	if a != b {
		t.Fatalf("identity differs between calls: %+v vs %+v", a, b)
	}
	if a.Version != evaluation.EvaluatorVersion || a.Version == "" {
		t.Errorf("identity version %q", a.Version)
	}
	if len(a.PolicyDigest) != 64 {
		t.Errorf("policy digest %q is not a sha-256 hex digest", a.PolicyDigest)
	}
	sf := loadScenarios(t)
	sc := sf.Eligibility[0]
	root := materializeEligibility(t, sc)
	in := libEligibilityInput(sc, filepath.Dir(root))
	first := mustJSON(t, asEligibilityRows(evaluation.EvaluateEligibility(in)))
	for i := 0; i < 3; i++ {
		if again := mustJSON(t, asEligibilityRows(evaluation.EvaluateEligibility(in))); again != first {
			t.Fatalf("the same input produced a different verdict on run %d", i+2)
		}
	}
}

// ---- external consumer -------------------------------------------------------

const evalConsumerMain = `package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// frozenRepo is a consumer-supplied Repo over frozen observations: the consumer
// owns how history is read, and here it has none beyond the row's Expect text.
type frozenRepo struct{}

func (frozenRepo) InRepository() bool                           { return false }
func (frozenRepo) ResolveObject(string) (string, bool)          { return "", false }
func (frozenRepo) ResolveCommit(string) (string, bool)          { return "", false }
func (frozenRepo) IsShallow() (bool, bool)                      { return false, false }
func (frozenRepo) MergeBase(string, string) evaluation.History  { return evaluation.HistoryUnknown }
func (frozenRepo) MergeBaseAll(string, string) ([]string, bool) { return nil, false }
func (frozenRepo) DiffNames(string, string) ([]string, bool)    { return nil, false }
func (frozenRepo) RevList(string, []string, int) ([]string, bool) {
	return nil, false
}
func (frozenRepo) TreeFiles(string) []string                            { return nil }
func (frozenRepo) TreeModes(string) ([]evaluation.TreeEntry, bool)      { return nil, false }
func (frozenRepo) ObjectAt(string, string) (string, bool)               { return "", false }
func (frozenRepo) RootPrefix() (string, bool)                           { return "", false }
func (frozenRepo) VerifyExpectAt(string, string, string) (string, bool) { return "exit 0", true }
func (frozenRepo) DeclaredFilesAt(string, string) ([]string, bool, bool) {
	return nil, false, false
}

func main() {
	elig := evaluation.EvaluateEligibility(evaluation.EligibilityInput{
		Streams: []evaluation.Stream{{Name: "alpha", Briefs: []evaluation.Brief{
			{Num: "01", Status: "todo"},
			{Num: "02", Status: "todo", Gates: []evaluation.Edge{{Ref: "alpha/01", Type: "ordering-gate", Reason: "first"}}},
			{Num: "03", Status: "todo", Gates: []evaluation.Edge{{Ref: "#12", Type: "ordering-gate", Reason: "issue"}}},
		}}},
	})
	cov := evaluation.EvaluateCoverage(evaluation.CoverageInput{
		Revision: "aaaaaaaaaaaa",
		Briefs: []evaluation.BriefObservation{{
			ID:         "alpha/01",
			BriefRel:   "docs/streams/alpha/brief-01.md",
			VerifyRows: []evaluation.VerifyRow{{ID: "1", Command: "true", Expect: "exit 0"}},
			Witnesses:  map[string][]evaluation.Witness{"1": {{Command: "true", State: evaluation.StatePass, Tree: "aaaaaaaaaaaa"}}},
		}},
	}, frozenRepo{})
	out := map[string]any{"identity": evaluation.CurrentIdentity(), "eligibility": elig, "coverage": cov}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

// unsupportedConsumerMain expects an API the module does not offer: a verdict
// that grants admission. The build must fail.
const unsupportedConsumerMain = `package main

import "github.com/medici-finance/assay/statusgen/evaluation"

func main() {
	_ = evaluation.Admit(evaluation.EligibilityInput{})
}
`

// moduleZip packs the statusgen module the way a release would publish it:
// the manifest, the checksums and every non-test Go file, under the module
// path and version prefix.
func moduleZip(t *testing.T, root, modulePath, version string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	prefix := modulePath + "@" + version + "/"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if rel, rerr := filepath.Rel(root, path); rerr == nil {
			path = rel
		}
		if info.IsDir() {
			if path == "testdata" || strings.HasPrefix(path, ".") && path != "." {
				return filepath.SkipDir
			}
			return nil
		}
		slash := filepath.ToSlash(path)
		keep := slash == "go.mod" || slash == "go.sum" ||
			(strings.HasSuffix(slash, ".go") && !strings.HasSuffix(slash, "_test.go"))
		if !keep {
			return nil
		}
		w, err := zw.Create(prefix + slash)
		if err != nil {
			return err
		}
		f, err := os.Open(filepath.Join(root, path))
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// regression: statusgen/15 — a module outside this repository consumes both
// entry points through a pinned module version, with the workspace off, no
// relative replace and no network: a local file proxy is the only source.
// regression: #2692 — no ambient module cache is needed to seed that proxy.
func TestEvaluationExternalConsumer(t *testing.T) {
	const modulePath = "github.com/medici-finance/assay/statusgen"
	const version = "v0.0.0-20261011000000-000000000000"
	// An explicitly empty parent cache also catches another fixture helper
	// accidentally reintroducing a dependency on the caller's cache.
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOPROXY", "off")

	// The statusgen manifest names nothing under tools/desk, and any replace
	// is a relative sibling-module path (a dependency's replace directives are
	// ignored by a consumer, which resolves that module from its own sources;
	// the proxy below serves the sibling the way a published release would).
	manifest, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), "tools/desk") {
		t.Fatalf("the statusgen manifest must name nothing under tools/desk:\n%s", manifest)
	}
	for _, line := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "replace") && !strings.Contains(line, "=> ../") {
			t.Fatalf("the only replace statusgen may carry is a relative sibling module:\n%s", line)
		}
	}

	proxy := t.TempDir()
	modDir := filepath.Join(proxy, filepath.FromSlash(modulePath), "@v")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"list":            []byte(version + "\n"),
		version + ".info": []byte(`{"Version":"` + version + `","Time":"2026-10-11T00:00:00Z"}`),
		version + ".mod":  manifest,
		version + ".zip":  moduleZip(t, ".", modulePath, version),
	} {
		if err := os.WriteFile(filepath.Join(modDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The in-repository sibling module the manifest requires, served at the
	// version the manifest names, as a release of it would be.
	const siblingPath = "github.com/medici-finance/assay/forgeread"
	const siblingVersion = "v0.0.0-00010101000000-000000000000"
	siblingDir := filepath.Join(proxy, filepath.FromSlash(siblingPath), "@v")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	siblingMod, err := os.ReadFile(filepath.Join("..", "forgeread", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"list":                   []byte(siblingVersion + "\n"),
		siblingVersion + ".info": []byte(`{"Version":"` + siblingVersion + `","Time":"2026-10-11T00:00:00Z"}`),
		siblingVersion + ".mod":  siblingMod,
		siblingVersion + ".zip":  moduleZip(t, filepath.Join("..", "forgeread"), siblingPath, siblingVersion),
	} {
		if err := os.WriteFile(filepath.Join(siblingDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The evaluation package imports only the standard library. Serve the
	// modules under test, never ambient cached dependencies of the statusgen
	// CLI. TestMain isolates HOME, and normal test builds need not download
	// dependencies used only by a dependency's tests.

	env := func(consumer, source string) []string {
		return append(os.Environ(),
			"GOWORK=off",
			"GOFLAGS=-mod=mod -modcacherw",
			"GOPROXY=file://"+filepath.ToSlash(source),
			"GOSUMDB=off",
			"GONOSUMDB=*",
			"GOPRIVATE=", "GONOPROXY=none",
			"GOTOOLCHAIN=local",
			"GOMODCACHE="+filepath.Join(consumer, ".modcache"),
			"GOCACHE="+filepath.Join(consumer, ".gocache"),
			// Any network attempt fails at once.
			"HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1", "NO_PROXY=",
		)
	}
	runIn := func(dir, source, extraMain string, args ...string) (string, error) {
		if extraMain != "" {
			if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(extraMain), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = env(dir, source)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if err != nil {
			return stdout.String() + stderr.String(), err
		}
		return stdout.String(), nil
	}

	consumer := t.TempDir()
	gomod := "module example.test/consumer\n\ngo 1.25.0\n\nrequire " + modulePath + " " + version + "\n"
	if strings.Contains(gomod, "replace") {
		t.Fatal("the consumer manifest must not replace the module")
	}
	if err := os.WriteFile(filepath.Join(consumer, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := runIn(consumer, proxy, evalConsumerMain, "run", ".")
	if err != nil {
		t.Fatalf("the external consumer failed to build or run:\n%s", got)
	}
	var decoded struct {
		Identity    evaluation.Identity               `json:"identity"`
		Eligibility map[string]evaluation.Eligibility `json:"eligibility"`
		Coverage    map[string]evaluation.Coverage    `json:"coverage"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("consumer output is not JSON: %v\n%s", err, got)
	}
	// The consumer, through the published module, sees this checkout's rules.
	if decoded.Identity != evaluation.CurrentIdentity() {
		t.Errorf("the consumer's evaluator identity %+v differs from the module's %+v", decoded.Identity, evaluation.CurrentIdentity())
	}
	if e := decoded.Eligibility["alpha/01"]; e.Verdict != evaluation.VerdictEligible {
		t.Errorf("alpha/01 through the consumer: %+v", e)
	}
	if e := decoded.Eligibility["alpha/02"]; e.Verdict != evaluation.VerdictHeld || len(e.Holds) != 1 || e.Holds[0].State != evaluation.StateUnsatisfied {
		t.Errorf("alpha/02 through the consumer: %+v", e)
	}
	if e := decoded.Eligibility["alpha/03"]; e.Verdict != evaluation.VerdictHeld || len(e.Holds) != 1 || e.Holds[0].State != evaluation.StateCouldNotCheck {
		t.Errorf("alpha/03 through the consumer: %+v", e)
	}
	if c := decoded.Coverage["alpha/01"]; !c.Released || len(c.Claims) != 1 || c.Claims[0].Result != evaluation.ResultPass {
		t.Errorf("coverage through the consumer: %+v", c)
	}
	// The consumer's own go.mod, after resolution, still carries no replace.
	after, err := os.ReadFile(filepath.Join(consumer, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "replace") {
		t.Errorf("resolution wrote a replace into the consumer manifest:\n%s", after)
	}

	// Fresh consumers must fail when their pinned publication is unavailable,
	// even after this test has built a consumer successfully. Neither a warm
	// parent cache nor another served version can satisfy the missing pin.
	for _, tc := range []struct{ name, source, pin string }{
		{"missing module", t.TempDir(), version},
		{"missing version", proxy, "v0.0.0-20261011000000-000000000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mod := strings.Replace(gomod, version, tc.pin, 1)
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := runIn(dir, tc.source, evalConsumerMain, "run", ".")
			if err == nil || !strings.Contains(out, tc.pin) || !strings.Contains(out, "reading file://") {
				t.Fatalf("missing publication must fail at the file proxy: err=%v\n%s", err, out)
			}
		})
	}

	// An expectation of an API the module does not offer fails to build.
	if out, err := runIn(consumer, proxy, unsupportedConsumerMain, "build", "-o", filepath.Join(consumer, "unsupported"), "."); err == nil {
		t.Errorf("a consumer expecting an unsupported API built:\n%s", out)
	} else if !strings.Contains(out, "undefined: evaluation.Admit") {
		t.Errorf("the unsupported-API build failed for another reason:\n%s", out)
	}
}
