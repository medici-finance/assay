package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/statusgen/domain"
	"github.com/medici-finance/assay/statusgen/evaluation"
	"gopkg.in/yaml.v3"
)

const floorDir = "testdata/regressionfloor"

type floorEntry struct {
	File, Test, Contract, Source, Disposition, Reason string
	BodySHA256                                        string `yaml:"body_sha256"`
	Cases                                             []string
}
type floorInventory struct {
	Entries []floorEntry
	Added   []string `yaml:"added_cases"`
}
type floorResult struct {
	Released    bool     `json:"released"`
	Claim       string   `json:"claim"`
	Eligibility string   `json:"eligibility"`
	Holds       []string `json:"hold_states" yaml:"hold_states"`
	Notices     []string `json:"notice_states" yaml:"notice_states"`
}
type floorCase struct {
	ID, Verify, Evidence, Edge string
	Expected                   floorResult
}
type floorDifference struct {
	Case, Reason string
	Old, New     floorResult
}

func floorRead(path string, into any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	return d.Decode(into)
}
func floorLoad(dir string) (floorInventory, []floorCase, []floorDifference, error) {
	var inv floorInventory
	var diffs []floorDifference
	if err := floorRead(filepath.Join(dir, "inventory.yaml"), &inv); err != nil {
		return inv, nil, nil, err
	}
	if err := floorRead(filepath.Join(dir, "differences.yaml"), &diffs); err != nil {
		return inv, nil, nil, err
	}
	paths, err := filepath.Glob(filepath.Join(dir, "cases", "*.json"))
	if err != nil {
		return inv, nil, nil, err
	}
	selected := map[string]bool{}
	for _, e := range inv.Entries {
		for _, id := range e.Cases {
			selected[id] = true
		}
	}
	for _, id := range inv.Added {
		selected[id] = true
	}
	var cases []floorCase
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return inv, nil, nil, err
		}
		var c floorCase
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		if err = d.Decode(&c); err != nil {
			return inv, nil, nil, err
		}
		if c.ID == "" || filepath.Base(p) != c.ID+".json" {
			return inv, nil, nil, fmt.Errorf("invalid case identity: %s", p)
		}
		if !selected[c.ID] {
			return inv, nil, nil, fmt.Errorf("unselected case %s", c.ID)
		}
		delete(selected, c.ID)
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		return inv, nil, nil, fmt.Errorf("zero selected cases")
	}
	if len(selected) > 0 {
		return inv, nil, nil, fmt.Errorf("selected case missing: %v", selected)
	}
	known := map[string]bool{}
	for _, c := range cases {
		known[c.ID] = true
	}
	for _, d := range diffs {
		if !known[d.Case] {
			return inv, nil, nil, fmt.Errorf("difference names unselected case %s", d.Case)
		}
	}
	return inv, cases, diffs, nil
}
func floorResolve(inv floorInventory) error {
	contracts, err := os.ReadFile("../docs/contracts.md")
	if err != nil {
		return err
	}
	if len(inv.Entries) == 0 {
		return fmt.Errorf("empty inventory")
	}
	seen := map[string]bool{}
	for _, e := range inv.Entries {
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(e.Source) {
			return fmt.Errorf("unqualified source for %s", e.Test)
		}
		if !strings.Contains(string(contracts), "| "+e.Contract+" |") {
			return fmt.Errorf("unknown contract %s", e.Contract)
		}
		switch e.Disposition {
		case "adapt harness", "reuse unchanged", "replace", "add":
		default:
			return fmt.Errorf("invalid disposition %s", e.Disposition)
		}
		if e.Reason == "" {
			return fmt.Errorf("missing disposition reason %s", e.Test)
		}
		if filepath.IsAbs(e.File) || strings.Contains(e.File, "..") {
			return fmt.Errorf("invalid inventory path")
		}
		key := e.File + ":" + e.Test
		if seen[key] {
			return fmt.Errorf("duplicate entry %s", key)
		}
		seen[key] = true
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", e.File), nil, 0)
		if err != nil {
			return err
		}
		found := false
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == e.Test && fn.Type.Params.NumFields() == 1 {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("inventory test does not resolve: %s", key)
		}
		body := func(file *ast.File) string {
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == e.Test {
					var out bytes.Buffer
					if err := format.Node(&out, token.NewFileSet(), fn.Body); err == nil {
						return out.String()
					}
				}
			}
			return ""
		}
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(e.BodySHA256) || fmt.Sprintf("%x", sha256.Sum256([]byte(body(f)))) != e.BodySHA256 {
			return fmt.Errorf("retained baseline test changed: %s; review the test change and its preservation obligations, update this entry to the approved immutable source, then run go run testdata/regressionfloor/freeze.go from statusgen and review the digest diff", key)
		}
	}
	return nil
}

// A new historical read panics through the nil port rather than inventing facts.
type floorRepo struct {
	evaluation.Repo
	Tree string
}

func (r floorRepo) InRepository() bool { return false }
func (r floorRepo) VerifyExpectAt(rev, path, row string) (string, bool) {
	if rev == r.Tree && path == "docs/streams/build/brief-07.md" && row == "1" {
		return "exit 0", true
	}
	return "", false
}
func floorProject(c evaluation.Coverage, e evaluation.Eligibility) floorResult {
	out := floorResult{Released: c.Released, Eligibility: string(e.Verdict), Holds: []string{}, Notices: []string{}}
	if len(c.Claims) != 1 {
		out.Claim = fmt.Sprintf("claim-count:%d", len(c.Claims))
	} else {
		out.Claim = c.Claims[0].Result
	}
	for _, h := range e.Holds {
		out.Holds = append(out.Holds, string(h.State))
		if h.Why == "" {
			out.Holds = append(out.Holds, "missing-explanation")
		}
	}
	for _, n := range e.Notices {
		out.Notices = append(out.Notices, string(n.State))
		if n.Why == "" {
			out.Notices = append(out.Notices, "missing-explanation")
		}
	}
	return out
}
func floorAdapters(t *testing.T, c floorCase) (floorResult, floorResult) {
	t.Helper()
	var r domain.Record
	b, err := os.ReadFile(filepath.Join(floorDir, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "build")
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	writeCoverageBrief(t, dir, "07", "build", c.Verify, c.Evidence)
	path := filepath.Join(dir, "brief-07.md")
	bf, ok, err := parseBriefFile(path)
	if err != nil || !ok {
		t.Fatalf("parse frozen brief: %v %v", ok, err)
	}
	obs := observeBrief(root, path, "build/07", bf)
	brief := &Brief{Num: "07", Status: "implemented"}
	edge := GraphEdge{Ref: "unpub:example-a/01", Type: "ordering-gate", Reason: "required frozen target"}
	if c.Edge == "gate" {
		brief.Gates = []GraphEdge{edge}
	} else if c.Edge == "feather" {
		brief.Feathers = []GraphEdge{edge}
	}
	streams := []*Stream{{Name: "build", Briefs: []Brief{*brief}}}
	reg := &graphRepos{Aliases: map[string]graphRepoEntry{"unpub": {Unpublished: true}}}
	input := evaluation.CoverageInput{Revision: r.Candidate.Tree, Briefs: []evaluation.BriefObservation{obs}}
	repo := floorRepo{Tree: r.Candidate.Tree}
	// Both existing observation adapters and the domain reuse the one evaluator owner.
	old := floorProject(evaluation.EvaluateCoverage(input, repo)["build/07"], evaluateEligibility(streams, reg, root)["build/07"])
	a, err := domain.Assess(r, domain.Observations{Work: r.Work, Candidate: r.Candidate, Generation: r.Generation.Recorded, AcceptanceVersion: r.Acceptance.Version, AcceptanceDigest: r.Acceptance.Digest, Eligibility: evaluation.EligibilityInput{Streams: evaluationStreams(streams), Registry: evaluationRegistry(reg)}, Coverage: input, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if a.Coverage == nil || a.Eligibility == nil {
		t.Fatalf("domain omitted complete observation: %+v", a)
	}
	newer := floorProject(*a.Coverage, *a.Eligibility)
	if (a.State == "supported") != (newer.Released && newer.Eligibility != "held") {
		t.Fatalf("domain state disagrees: %+v", a)
	}
	return old, newer
}
func floorCompare(id string, old, newer, expected floorResult, diffs []floorDifference) error {
	var match *floorDifference
	for i := range diffs {
		d := &diffs[i]
		if d.Case == id {
			if match != nil {
				return fmt.Errorf("duplicate difference %s", id)
			}
			match = d
		}
	}
	equal := reflect.DeepEqual(old, newer)
	if equal && match != nil {
		return fmt.Errorf("stale difference %s", id)
	}
	if !equal {
		if match == nil {
			return fmt.Errorf("unlisted difference %s", id)
		}
		if strings.TrimSpace(match.Reason) == "" || !reflect.DeepEqual(old, match.Old) || !reflect.DeepEqual(newer, match.New) {
			return fmt.Errorf("difference outcomes/reason mismatch %s", id)
		}
		if !reflect.DeepEqual(old, expected) {
			return fmt.Errorf("old expectation mismatch %s: %+v", id, old)
		}
		return nil
	}
	if !reflect.DeepEqual(old, expected) {
		return fmt.Errorf("expectation mismatch %s: got %+v want %+v", id, old, expected)
	}
	return nil
}

// regression: graph-execution/29
func TestRegressionFloorInventoryResolves(t *testing.T) {
	inv, _, _, err := floorLoad(floorDir)
	if err != nil {
		t.Fatal(err)
	}
	if err = floorResolve(inv); err != nil {
		t.Fatal(err)
	}
	bad := inv
	bad.Entries = append([]floorEntry(nil), inv.Entries...)
	for _, digest := range []string{"", "not-a-digest", strings.Repeat("0", 64)} {
		bad.Entries[0].BodySHA256 = digest
		if floorResolve(bad) == nil {
			t.Fatal("missing, invalid or mismatched baseline digest accepted")
		}
	}
	inv.Entries[0].Test += "Misspelled"
	if floorResolve(inv) == nil {
		t.Fatal("misspelled inventory accepted")
	}
}

// regression: graph-execution/29
func TestRegressionFloorOldNewAgree(t *testing.T) {
	inv, cases, diffs, err := floorLoad(floorDir)
	if err != nil {
		t.Fatal(err)
	}
	if err = floorResolve(inv); err != nil {
		t.Fatal(err)
	}
	head, err := coverageGit("..", "rev-parse", "HEAD").Output()
	if err != nil {
		head = []byte("unavailable (source export)")
	}
	t.Logf("checkout-head=%s (adapters execute this working source, baseline denotes inventory provenance)", strings.TrimSpace(string(head)))
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			old, newer := floorAdapters(t, c)
			for name, result := range map[string]floorResult{"package-main-observation-adapter": old, "domain.Assess": newer} {
				t.Logf("implementation=%s evaluator=%+v case=%s baseline=%s result=%+v", name, evaluation.CurrentIdentity(), c.ID, inv.Entries[0].Source, result)
			}
			if err := floorCompare(c.ID, old, newer, c.Expected, diffs); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// regression: graph-execution/29
func TestRegressionFloorUnlistedDifferenceFails(t *testing.T) {
	floorAllowanceGuards(t)
	_, cases, _, err := floorLoad(floorDir)
	if err != nil {
		t.Fatal(err)
	}
	a := cases[0].Expected
	b := a
	b.Released = !b.Released
	d := floorDifference{Case: "control", Reason: "synthetic approved control", Old: a, New: b}
	if floorCompare("control", a, b, a, nil) == nil {
		t.Fatal("unlisted difference accepted")
	}
	if err := floorCompare("control", a, b, a, []floorDifference{d}); err != nil {
		t.Fatal(err)
	}
	if floorCompare("control", a, a, a, []floorDifference{d}) == nil {
		t.Fatal("stale difference accepted")
	}
	if floorCompare("control", a, a, b, nil) == nil {
		t.Fatal("shared expected outcome guard did not refuse")
	}
	if floorCompare("control", a, b, b, []floorDifference{d}) == nil {
		t.Fatal("allowed old expectation guard did not refuse")
	}
	bad := d
	bad.Old = b
	if floorCompare("control", a, b, a, []floorDifference{bad}) == nil {
		t.Fatal("different old outcome accepted")
	}
	bad = d
	bad.New = a
	if floorCompare("control", a, b, a, []floorDifference{bad}) == nil {
		t.Fatal("different new outcome accepted")
	}
}

// regression: graph-execution/29
func TestRegressionFloorZeroSelectedFails(t *testing.T) {
	floorCaseGuards(t)
	for _, mode := range []string{"empty-selection", "empty-directory"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			inv := floorInventory{}
			if mode == "empty-directory" {
				inv.Added = []string{"missing"}
			}
			b, err := yaml.Marshal(inv)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "inventory.yaml"), b, 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "differences.yaml"), []byte("[]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, _, err = floorLoad(dir); err == nil {
				t.Fatal("zero selected cases accepted")
			}
		})
	}
}

// floorFixture writes only synthetic control data, never production expectations.
func floorFixture(t *testing.T, selected, files []string, diffs []floorDifference) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "cases"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"inventory.yaml": floorInventory{Added: selected}, "differences.yaml": diffs} {
		b, err := yaml.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range files {
		b, err := json.Marshal(floorCase{ID: id})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cases", id+".json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// regression: graph-execution/29
func floorCaseGuards(t *testing.T) {
	t.Helper()
	dir := floorFixture(t, []string{"known", "second"}, []string{"known", "second"}, nil)
	if _, cases, _, err := floorLoad(dir); err != nil || len(cases) != 2 {
		t.Fatalf("valid case selection refused: %v", err)
	}
	for _, id := range []string{"", "different"} {
		dir := floorFixture(t, []string{"known"}, []string{"known"}, nil)
		b, err := json.Marshal(floorCase{ID: id})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cases", "known.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := floorLoad(dir); err == nil || !strings.Contains(err.Error(), "invalid case identity") {
			t.Fatalf("case identity guard did not refuse: %v", err)
		}
	}
	for _, control := range []struct {
		name            string
		selected, files []string
		want            string
	}{
		{"missing-selected", []string{"known", "missing"}, []string{"known"}, "selected case missing"},
		{"unselected-file", []string{"known"}, []string{"known", "extra"}, "unselected case extra"},
	} {
		t.Run(control.name, func(t *testing.T) {
			dir := floorFixture(t, control.selected, control.files, nil)
			if _, _, _, err := floorLoad(dir); err == nil || !strings.Contains(err.Error(), control.want) {
				t.Fatalf("%s guard did not refuse: %v", control.name, err)
			}
		})
	}
}

// regression: graph-execution/29
func floorAllowanceGuards(t *testing.T) {
	t.Helper()
	old := floorResult{Claim: "could-not-check"}
	newer := floorResult{Claim: "pass", Released: true}
	d := floorDifference{Case: "known", Reason: "synthetic approved control", Old: old, New: newer}
	if err := floorCompare("known", old, newer, old, []floorDifference{d}); err != nil {
		t.Fatal(err)
	}
	if err := floorCompare("known", old, newer, old, []floorDifference{d, d}); err == nil || !strings.Contains(err.Error(), "duplicate difference") {
		t.Fatalf("duplicate allowance guard did not refuse: %v", err)
	}
	for _, reason := range []string{"", "  "} {
		bad := d
		bad.Reason = reason
		if err := floorCompare("known", old, newer, old, []floorDifference{bad}); err == nil || !strings.Contains(err.Error(), "difference outcomes/reason mismatch") {
			t.Fatalf("empty reason guard did not refuse: %v", err)
		}
	}
	dir := floorFixture(t, []string{"known"}, []string{"known"}, []floorDifference{d})
	if _, _, _, err := floorLoad(dir); err != nil {
		t.Fatalf("known allowance refused: %v", err)
	}
	d.Case = "unknown"
	dir = floorFixture(t, []string{"known"}, []string{"known"}, []floorDifference{d})
	if _, _, _, err := floorLoad(dir); err == nil || !strings.Contains(err.Error(), "difference names unselected case unknown") {
		t.Fatalf("unknown allowance guard did not refuse: %v", err)
	}
}

// regression: graph-execution/29
func TestFloorFreezeAuthoring(t *testing.T) {
	cmd := exec.Command("go", "test", "-timeout", "45s", "-count=1", "./testdata/regressionfloor")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "KUBECONFIG=/dev/null")
	out, err := goquiet.Cmd(cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("authoring helper controls: %v\n%s", err, out)
	}
}
