package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// evaluation_scenarios_test.go — the frozen inputs and golden verdicts of the
// eligibility and coverage evaluation contract (statusgen/15).
//
// testdata/evaluation/scenarios.json describes trees. Each description is
// materialized into a real tree for the statusgen CLI path, and the same
// description is read directly by the evaluation library in
// evaluation_contract_test.go, so one frozen input feeds both consumers.
// testdata/evaluation/golden.json holds the verdicts and reasons captured from
// the pre-extraction implementation (revisions are normalized to commit
// labels): the extraction must not move a single verdict or reason string.
// Refresh the golden file only with STATUSGEN_UPDATE_GOLDEN=1, and review the
// diff.

type scenarioFile struct {
	Eligibility []eligScenario `json:"eligibility"`
	Coverage    []covScenario  `json:"coverage"`
}

type eligScenario struct {
	Name     string                `json:"name"`
	Registry map[string]eligAlias  `json:"registry"`
	Siblings map[string][]eligStrm `json:"siblings"`
	Streams  []eligStrm            `json:"streams"`
}

type eligAlias struct {
	Repo        string `json:"repo"`
	Unpublished bool   `json:"unpublished"`
}

type eligStrm struct {
	Name   string      `json:"name"`
	Briefs []eligBrief `json:"briefs"`
}

type eligBrief struct {
	Num       string     `json:"num"`
	Status    string     `json:"status"`
	Gate      string     `json:"gate"`
	Depends   []string   `json:"depends"`
	Gates     []eligEdge `json:"gates"`
	Feathers  []string   `json:"feathers"`
	HumanPass bool       `json:"humanPass"`
}

type eligEdge struct {
	On     string `json:"on"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type covScenario struct {
	Name    string       `json:"name"`
	NoGit   bool         `json:"noGit"`
	Steps   []covStep    `json:"steps"`
	Options *covOptsSpec `json:"options"`
}

type covStep struct {
	Write  map[string]string `json:"write"`
	Brief  *covBrief         `json:"brief"`
	Commit string            `json:"commit"`
}

type covBrief struct {
	Files     string       `json:"files"`
	Verify    []covRow     `json:"verify"`
	Witnesses []covWitness `json:"witnesses"`
}

type covRow struct {
	ID      string `json:"id"`
	Class   string `json:"class"`
	Command string `json:"command"`
	Expect  string `json:"expect"`
}

type covWitness struct {
	Row     string `json:"row"`
	Command string `json:"command"`
	State   string `json:"state"`
	Tree    string `json:"tree"`
}

type covOptsSpec struct {
	Bindings     map[string]covBindSpec `json:"bindings"`
	Patterns     map[string]covPatSpec  `json:"patterns"`
	Dependencies map[string]covDepsSpec `json:"dependencies"`
}

type covBindSpec struct {
	Pattern string `json:"pattern"`
	Node    string `json:"node"`
}

type covPatSpec struct {
	Join  string                 `json:"join"`
	Nodes map[string]covNodeSpec `json:"nodes"`
}

type covNodeSpec struct {
	Kind     string        `json:"kind"`
	Role     string        `json:"role"`
	Evidence []covEvidence `json:"evidence"`
}

type covEvidence struct {
	Kind      string `json:"kind"`
	Claim     string `json:"claim"`
	Mandatory bool   `json:"mandatory"`
	Source    string `json:"source"`
}

type covDepsSpec struct {
	Complete bool         `json:"complete"`
	Deps     []covDepSpec `json:"deps"`
}

type covDepSpec struct {
	Kind     string   `json:"kind"`
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Recorded string   `json:"recorded"`
	Current  string   `json:"current"`
	Rows     []string `json:"rows"`
}

func loadScenarios(t *testing.T) scenarioFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "evaluation", "scenarios.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f scenarioFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("scenarios.json: %v", err)
	}
	return f
}

// ---- eligibility materialization ------------------------------------------

func writeScenarioFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func yamlList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// writeEligStream writes one stream directory (README table plus brief files)
// below root.
func writeEligStream(t *testing.T, root string, s eligStrm) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", s.Name)
	var rows strings.Builder
	for _, b := range s.Briefs {
		fmt.Fprintf(&rows, "| %s | [Brief %s](brief-%s-x.md) | 0 | S | %s | — | — |\n", b.Num, b.Num, b.Num, b.Status)
	}
	writeScenarioFile(t, filepath.Join(dir, "README.md"), fmt.Sprintf(`---
stream: %s
repo: example-org/example-app
status: active
priority: P2
track: platform
---

# %s — evaluation scenario stream

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
%s`, s.Name, s.Name, rows.String()))
	for _, b := range s.Briefs {
		gate := b.Gate
		if gate == "" {
			gate = "model"
		}
		var fm strings.Builder
		fmt.Fprintf(&fm, "---\nbrief: example:app:%s:%s\ntitle: \"Scenario brief %s\"\nwhy: >-\n  Evaluation scenario fixture.\nwave: 0\ndepends: %s\nunblocks: []\neffort: S\ngate: %s\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nschema: brief-v2\n",
			s.Name, b.Num, b.Num, yamlList(b.Depends), gate)
		if len(b.Gates) > 0 {
			fm.WriteString("gates:\n")
			for _, g := range b.Gates {
				fmt.Fprintf(&fm, "  - on: %q\n    type: %s\n    reason: %q\n", g.On, g.Type, g.Reason)
			}
		}
		if len(b.Feathers) > 0 {
			fm.WriteString("feathers:\n")
			for _, f := range b.Feathers {
				fmt.Fprintf(&fm, "  - %q\n", f)
			}
		}
		fm.WriteString("authored: 2026-10-11 by statusgen/15 fixture\nsources: [\"fixture: evaluation scenarios\"]\n---\n\n")
		fmt.Fprintf(&fm, "# Brief %s — scenario\n", b.Num)
		if b.HumanPass {
			fm.WriteString("\n## Evidence\n\n**VERIFY: PASS** — recorded by a human verifier.\n")
		}
		writeScenarioFile(t, filepath.Join(dir, "brief-"+b.Num+"-x.md"), fm.String())
	}
}

// materializeEligibility builds the scenario's house tree under a fresh parent
// directory (siblings live next to it) and returns the house root.
func materializeEligibility(t *testing.T, sc eligScenario) string {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "house")
	for _, s := range sc.Streams {
		writeEligStream(t, root, s)
	}
	if sc.Registry != nil {
		var b strings.Builder
		b.WriteString("schema: graph-repos-v1\ncell: example\nrepos:\n")
		aliases := make([]string, 0, len(sc.Registry))
		for a := range sc.Registry {
			aliases = append(aliases, a)
		}
		sort.Strings(aliases)
		for _, a := range aliases {
			e := sc.Registry[a]
			if e.Unpublished {
				fmt.Fprintf(&b, "  %s: {cell: example, repo: null, unpublished: true}\n", a)
			} else {
				fmt.Fprintf(&b, "  %s: {cell: example, repo: %s}\n", a, e.Repo)
			}
		}
		writeScenarioFile(t, filepath.Join(root, "docs", "streams", "graph-repos.yaml"), b.String())
	}
	for name, streams := range sc.Siblings {
		for _, s := range streams {
			writeEligStream(t, filepath.Join(parent, name), s)
		}
	}
	return root
}

// eligRecord is one brief's verdict as the golden file records it, Why
// included (it is not part of the CLI JSON, but it is part of the contract).
type eligRecord struct {
	ID      string    `json:"id"`
	Verdict string    `json:"verdict"`
	Holds   []edgeRec `json:"holds,omitempty"`
	Notices []edgeRec `json:"notices,omitempty"`
}

type edgeRec struct {
	Ref    string `json:"ref"`
	Type   string `json:"type"`
	Reason string `json:"reason,omitempty"`
	State  string `json:"state"`
	Why    string `json:"why,omitempty"`
}

func eligRecords(elig map[string]Eligibility) []eligRecord {
	ids := make([]string, 0, len(elig))
	for id := range elig {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	conv := func(in []EdgeVerdict) []edgeRec {
		var out []edgeRec
		for _, e := range in {
			out = append(out, edgeRec{Ref: e.Ref, Type: e.Type, Reason: e.Reason, State: string(e.State), Why: e.Why})
		}
		return out
	}
	var out []eligRecord
	for _, id := range ids {
		e := elig[id]
		out = append(out, eligRecord{ID: id, Verdict: string(e.Verdict), Holds: conv(e.Holds), Notices: conv(e.Notices)})
	}
	return out
}

// ---- coverage materialization ----------------------------------------------

type builtCoverage struct {
	Root   string
	Labels map[string]string // commit label -> full sha
	Stream *Stream
	Opts   coverageOptions
}

func covVerifyTable(rows []covRow) string {
	if len(rows) == 0 {
		return ""
	}
	hasClass := false
	for _, r := range rows {
		if r.Class != "" {
			hasClass = true
		}
	}
	var b strings.Builder
	if hasClass {
		b.WriteString("| # | Class | Command | Expect |\n|---|-------|---------|--------|\n")
	} else {
		b.WriteString("| # | Command | Expect |\n|---|---------|--------|\n")
	}
	for i, r := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		if hasClass {
			fmt.Fprintf(&b, "| %s | %s | `%s` | %s |", r.ID, r.Class, r.Command, r.Expect)
		} else {
			fmt.Fprintf(&b, "| %s | `%s` | %s |", r.ID, r.Command, r.Expect)
		}
	}
	return b.String()
}

func resolveTree(labels map[string]string, tok string) string {
	base, suffix, _ := strings.Cut(tok, "+")
	if sha, ok := labels[base]; ok {
		tok = sha
		if len(tok) > treeSHALen {
			tok = tok[:treeSHALen]
		}
		if suffix != "" {
			tok += "+" + suffix
		}
	}
	return tok
}

func covOptionsFrom(spec *covOptsSpec) coverageOptions {
	var opts coverageOptions
	if spec == nil {
		return opts
	}
	if spec.Bindings != nil {
		opts.Bindings = map[string]coverageBinding{}
		for id, b := range spec.Bindings {
			opts.Bindings[id] = coverageBinding{Pattern: b.Pattern, Node: b.Node}
		}
	}
	if spec.Patterns != nil {
		opts.Patterns = map[string]patternDoc{}
		for name, p := range spec.Patterns {
			doc := patternDoc{Name: name, Join: p.Join, NodeByID: map[string]patternNode{}}
			for id, n := range p.Nodes {
				node := patternNode{ID: id, Kind: n.Kind, Role: n.Role}
				for _, e := range n.Evidence {
					node.Evidence = append(node.Evidence, patternEvidence{Kind: e.Kind, Claim: e.Claim, Mandatory: e.Mandatory, Source: e.Source})
				}
				doc.NodeByID[id] = node
			}
			opts.Patterns[name] = doc
		}
	}
	if spec.Dependencies != nil {
		opts.Dependencies = map[string]dependencyManifest{}
		for id, d := range spec.Dependencies {
			m := dependencyManifest{Complete: d.Complete}
			for _, dep := range d.Deps {
				m.Deps = append(m.Deps, workInputDep{Kind: dep.Kind, Path: dep.Path, Name: dep.Name, Recorded: dep.Recorded, Current: dep.Current, Rows: dep.Rows})
			}
			opts.Dependencies[id] = m
		}
	}
	return opts
}

func materializeCoverage(t *testing.T, sc covScenario) builtCoverage {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeScenarioFile(t, filepath.Join(s.Dir, "README.md"), `---
stream: cov
repo: example-org/example-app
status: active
priority: P2
track: platform
---

# Cov — evaluation scenario stream

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [One](brief-01.md) | 0 | S | todo | — | — |
`)
	labels := map[string]string{}
	initialised := false
	for _, st := range sc.Steps {
		for rel, content := range st.Write {
			mustWriteFile(t, root, rel, content)
		}
		if st.Brief != nil {
			var evidence string
			if len(st.Brief.Witnesses) > 0 {
				rows := make([]string, 0, len(st.Brief.Witnesses))
				for _, w := range st.Brief.Witnesses {
					rows = append(rows, covWitnessRow(w.Row, w.Command, w.State, resolveTree(labels, w.Tree)))
				}
				evidence = coverageEvidenceTable(rows...)
			}
			filesLine := st.Brief.Files
			writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", filesLine, covVerifyTable(st.Brief.Verify), evidence)
		}
		if st.Commit != "" {
			if !initialised {
				mustGitInit(t, root)
				initialised = true
			} else {
				mustGitCommitAll(t, root, "step "+st.Commit)
			}
			labels[st.Commit] = gitHeadFull(t, root)
		}
	}
	return builtCoverage{Root: root, Labels: labels, Stream: s, Opts: covOptionsFrom(sc.Options)}
}

func gitHeadFull(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// normalizeRevs rewrites every labelled commit's 40- and 12-character spelling
// to <label>, and the scenario's temp directory to <tmp>, so a golden is stable
// across machines and runs.
func normalizeRevs(s string, labels map[string]string, tmp string) string {
	if tmp != "" {
		s = strings.ReplaceAll(s, tmp, "<tmp>")
	}
	keys := make([]string, 0, len(labels))
	for l := range labels {
		keys = append(keys, l)
	}
	sort.Strings(keys)
	for _, l := range keys {
		sha := labels[l]
		s = strings.ReplaceAll(s, sha, "<"+l+">")
		s = strings.ReplaceAll(s, sha[:treeSHALen], "<"+l+">")
	}
	return s
}

// ---- golden file -------------------------------------------------------------

type goldenFile struct {
	Eligibility map[string][]eligRecord `json:"eligibility"`
	Coverage    map[string][]Coverage   `json:"coverage"`
}

const goldenPath = "testdata/evaluation/golden.json"

func readGolden(t *testing.T) goldenFile {
	t.Helper()
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (capture with STATUSGEN_UPDATE_GOLDEN=1)", err)
	}
	var g goldenFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func marshalCanonical(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func normalizedCoverage(t *testing.T, cov map[string]Coverage, labels map[string]string, tmp string) []Coverage {
	t.Helper()
	ids := make([]string, 0, len(cov))
	for id := range cov {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var rows []Coverage
	for _, id := range ids {
		rows = append(rows, cov[id])
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var out []Coverage
	if err := json.Unmarshal([]byte(normalizeRevs(string(raw), labels, tmp)), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestEvaluationGoldenVerdicts pins every frozen scenario's verdicts and
// reasons, as computed through the statusgen evaluation entry points, to the
// golden file captured from the pre-extraction implementation.
func TestEvaluationGoldenVerdicts(t *testing.T) {
	sf := loadScenarios(t)
	got := goldenFile{Eligibility: map[string][]eligRecord{}, Coverage: map[string][]Coverage{}}
	for _, sc := range sf.Eligibility {
		root := materializeEligibility(t, sc)
		streams, _, err := loadHydratedStreams(root)
		if err != nil {
			t.Fatalf("%s: %v", sc.Name, err)
		}
		recs := eligRecords(eligibilityForStreams(streams))
		raw, _ := json.Marshal(recs)
		var norm []eligRecord
		if err := json.Unmarshal([]byte(normalizeRevs(string(raw), nil, filepath.Dir(root))), &norm); err != nil {
			t.Fatal(err)
		}
		got.Eligibility[sc.Name] = norm
	}
	for _, sc := range sf.Coverage {
		b := materializeCoverage(t, sc)
		streams, _, err := loadHydratedStreams(b.Root)
		if err != nil {
			t.Fatalf("%s: %v", sc.Name, err)
		}
		cov := evaluateCoverage(b.Root, streams, b.Opts)
		got.Coverage[sc.Name] = normalizedCoverage(t, cov, b.Labels, b.Root)
	}
	if os.Getenv("STATUSGEN_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, marshalCanonical(t, got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden file rewritten: %s", goldenPath)
		return
	}
	want := readGolden(t)
	for name, g := range want.Eligibility {
		if string(marshalCanonical(t, got.Eligibility[name])) != string(marshalCanonical(t, g)) {
			t.Errorf("eligibility scenario %s drifted from the golden verdicts:\n got: %s\nwant: %s", name, marshalCanonical(t, got.Eligibility[name]), marshalCanonical(t, g))
		}
	}
	for name, g := range want.Coverage {
		if string(marshalCanonical(t, got.Coverage[name])) != string(marshalCanonical(t, g)) {
			t.Errorf("coverage scenario %s drifted from the golden verdicts:\n got: %s\nwant: %s", name, marshalCanonical(t, got.Coverage[name]), marshalCanonical(t, g))
		}
	}
	for name := range got.Eligibility {
		if _, ok := want.Eligibility[name]; !ok {
			t.Errorf("eligibility scenario %s has no golden entry", name)
		}
	}
	for name := range got.Coverage {
		if _, ok := want.Coverage[name]; !ok {
			t.Errorf("coverage scenario %s has no golden entry", name)
		}
	}
}
