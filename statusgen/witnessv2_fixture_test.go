package main

// witnessv2_fixture_test.go — a real git repository holding one brief whose
// Verify rows carry v2 witnesses landed the way a delivering change request
// lands them (statusgen/36): a base commit, a feature branch adding the
// records and the Evidence rows, and a `Merge pull request #7` merge commit on
// main. The record content is computed through the same port the reader uses
// (coverageRepo.ListTree / CheckAttr at the base) and the library's one
// derivation (ComputeWitnessContent), so a release proves the reader and the
// writer agree on a real tree.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

const (
	gv2Stream  = "wv"
	gv2Num     = "01"
	gv2BriefID = gv2Stream + "/" + gv2Num
	gv2CR      = "7"
	gv2Date    = "2026-10-25"
	gv2Output  = "sha256:0a1b2c3d4e5f"
	gv2Harness = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	gv2Pins    = "statusgen:\n  tag: statusgen/v1.0.59\ndesk-tools:\n  tag: desk-tools/v1.0.59\nharness:\n  digest: " + gv2Harness + "\n"
)

type gv2Row struct{ ID, Class, Command, Expect string }

// gv2Witness is one witness row the fixture writes into Evidence.
type gv2Witness struct {
	Row string
	// Identity is the cell's identity token (the text before `@`).
	Identity string
	// Tail is what follows the tree token in the cell: the on-behalf-of and
	// runner-source parentheticals, with their leading space.
	Tail string
	// V1Tree, when set, writes a v1 cell naming this commit instead of a v2
	// cell (no record).
	V1Tree string
	// EvidenceCommand overrides the Evidence Command cell (default: the Verify
	// row's Command).
	EvidenceCommand string
	// RecordCommand overrides the text command_sha256 digests (default: the
	// Evidence Command cell).
	RecordCommand string
	State         string // default pass
}

type gitV2Fixture struct {
	t      *testing.T
	Root   string
	Inputs string // the `inputs:` value; "" writes no inputs line
	Rows   []gv2Row
	Wits   []gv2Witness
	// AuthorName/AuthorEmail author the feature commit that writes the
	// Evidence rows (default: the fixture verifier App).
	AuthorName, AuthorEmail string
	// AfterMerge edits files on main after the merge (committed as one more
	// first-parent commit) — the Verify row changing after the run.
	AfterMerge map[string]string

	Base, Feature, Merge string
	RecordIDs            map[string]string // row id → record id (v2 rows)
	BriefPath            string
}

func gv2BriefRel() string {
	return filepath.ToSlash(filepath.Join("docs", "streams", gv2Stream, "brief-"+gv2Num+".md"))
}

func gv2Git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(fixtureGitEnv(os.Environ()), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	cmd.Env = append(cmd.Env, fixtureGitIdentity...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// gv2GitAs runs git with the author (and committer) identity forced, over the
// fixture identity gv2Git exports in the environment.
func gv2GitAs(t *testing.T, root, name, email string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(fixtureGitEnv(os.Environ()), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	cmd.Env = append(cmd.Env, fixtureGitIdentity...)
	cmd.Env = append(cmd.Env, "GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func gv2Write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gv2VerifyTable(rows []gv2Row) string {
	var b strings.Builder
	b.WriteString("| # | Class | Command | Expect |\n|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString("| " + r.ID + " | " + r.Class + " | `" + r.Command + "` | " + r.Expect + " |\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func gv2Brief(inputs string, rows []gv2Row, evidence string) string {
	context := "## Context\n\nfacts: none\n"
	if inputs != "" {
		context += "inputs: " + inputs + "\n"
	}
	return `---
brief: ` + gv2BriefID + `
title: witness v2 fixture
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-10-25 by statusgen/36 fixture
sources: ["fixture: statusgen/36 witnessv2_fixture_test.go"]
---

# Brief 01 — witness v2 fixture

` + context + `
## Verify
` + gv2VerifyTable(rows) + `

## Evidence
<!-- contract comment -->

` + evidence + `

## Review
Gate: model.
`
}

func gv2README() string {
	return `---
stream: ` + gv2Stream + `
repo: example-org/example-app
status: active
priority: P2
track: platform
---

# Wv — witness v2 fixture stream

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [One](brief-01.md) | 0 | S | todo | — | — |
`
}

// newGitV2Fixture returns the default fixture: inputs `tools/x/pkg/`, two
// Verify rows, one v2 witness per row by the verifier App.
func newGitV2Fixture(t *testing.T) *gitV2Fixture {
	return &gitV2Fixture{
		t:      t,
		Root:   t.TempDir(),
		Inputs: "`tools/x/pkg/`",
		Rows: []gv2Row{
			{ID: "1", Class: "check:ci", Command: "cat tools/x/pkg/a.go", Expect: "exit 0"},
			{ID: "2", Class: "check:ci", Command: "cat tools/x/pkg/b.go", Expect: "exit 0; prints B"},
		},
		Wits: []gv2Witness{
			{Row: "1", Identity: "verifier-app", Tail: " (forge-identity)"},
			{Row: "2", Identity: "verifier-app", Tail: " (forge-identity)"},
		},
		AuthorName:  fixtureVerifierName,
		AuthorEmail: fixtureVerifierEmail,
	}
}

func (f *gitV2Fixture) verifyRow(id string) gv2Row {
	for _, r := range f.Rows {
		if r.ID == id {
			return r
		}
	}
	f.t.Fatalf("no Verify row %s", id)
	return gv2Row{}
}

// build lays down the base, the feature branch and the merge.
func (f *gitV2Fixture) build() *gitV2Fixture {
	t := f.t
	t.Helper()
	root := f.Root
	gv2Git(t, root, "init", "-q", "-b", "main")
	gv2Git(t, root, "config", "commit.gpgsign", "false")
	gv2Write(t, root, "docs/streams/"+gv2Stream+"/README.md", gv2README())
	gv2Write(t, root, evaluation.WitnessPinFile, gv2Pins)
	gv2Write(t, root, "tools/x/go.mod", "module example.test/x\n\ngo 1.25\n")
	gv2Write(t, root, "tools/x/pkg/a.go", "package pkg\n\nfunc A() int { return 1 }\n")
	gv2Write(t, root, "tools/x/pkg/b.go", "package pkg\n\n// B\nfunc B() int { return 2 }\n")
	gv2Write(t, root, "README.md", "fixture\n")
	f.BriefPath = filepath.Join(root, filepath.FromSlash(gv2BriefRel()))
	gv2Write(t, root, gv2BriefRel(), gv2Brief(f.Inputs, f.Rows, ""))
	gv2Git(t, root, "add", "-A")
	gv2Git(t, root, "commit", "-q", "-m", "fixture: base")
	f.Base = gv2Git(t, root, "rev-parse", "HEAD")

	gv2Git(t, root, "checkout", "-q", "-b", "fixture/branch")
	repo := coverageRepo{root: root}
	listing, ok := repo.ListTree(f.Base)
	if !ok {
		t.Fatal("ls-tree at base failed")
	}
	declared, found, _ := evaluation.ParseInputsDeclaration("inputs: " + f.Inputs)
	if f.Inputs == "" {
		declared, found = nil, false
	}
	var paths []string
	for _, e := range listing {
		paths = append(paths, e.Path)
	}
	attrs, ok := repo.CheckAttr(f.Base, paths)
	if !ok {
		t.Fatal("check-attr at base failed")
	}
	f.RecordIDs = map[string]string{}
	var rows []string
	for _, w := range f.Wits {
		vr := f.verifyRow(w.Row)
		evCmd := vr.Command
		if w.EvidenceCommand != "" {
			evCmd = w.EvidenceCommand
		}
		state := w.State
		if state == "" {
			state = statePass
		}
		result := state + " exit=0"
		if w.V1Tree != "" {
			rows = append(rows, "| "+w.Row+" | `"+evCmd+"` | "+result+" | "+gv2Output+" | "+gv2Date+" | "+w.Identity+" @ "+w.V1Tree+w.Tail+" |")
			continue
		}
		recCmd := evCmd
		if w.RecordCommand != "" {
			recCmd = w.RecordCommand
		}
		content := evaluation.ComputeWitnessContent(evaluation.WitnessClaimSpec(declared, found, nil, w.Row), listing, attrs)
		rec := evaluation.WitnessRecord{
			Schema: 2, Brief: gv2BriefID, Row: w.Row,
			CommandSHA256: evaluation.WitnessTextDigest(recCmd), ExpectSHA256: evaluation.WitnessTextDigest(vr.Expect), ClassSHA256: evaluation.WitnessTextDigest(vr.Class),
			Runner: w.Identity, RunnerSource: "app-token", RunnerClass: evaluation.WitnessRunnerContainer, Network: "none",
			Tools:       map[string]string{"statusgen": "statusgen/v1.0.59", "desk-tools": "desk-tools/v1.0.59", "harness": gv2Harness, "go": "go1.25.3"},
			ToolsSource: "recorded", Origin: "run", Result: result, Exit: "0", OutputSHA256: gv2Output, Date: gv2Date,
			CR: gv2CR, Scope: content.Scope, Inputs: content.Inputs, Implicit: content.Implicit, Attributes: content.Attributes,
			TreeDigest: content.TreeDigest, DirtyPaths: []string{},
			Informational: evaluation.WitnessInformational{Commit: f.Base},
		}
		canonical, id, err := evaluation.EncodeWitnessRecord(rec)
		if err != nil {
			t.Fatalf("encode record: %v", err)
		}
		f.RecordIDs[w.Row] = id
		gv2Write(t, root, evaluation.WitnessRecordPath(gv2BriefID, id), string(canonical))
		rows = append(rows, "| "+w.Row+" | `"+evCmd+"` | "+result+" | "+gv2Output+" | "+gv2Date+" | "+w.Identity+" @ [w2] cr:"+gv2CR+" rec:"+id+w.Tail+" |")
	}
	evidence := ""
	if len(rows) > 0 {
		evidence = witnessHeader + "\n" + strings.Join(rows, "\n")
	}
	gv2Write(t, root, gv2BriefRel(), gv2Brief(f.Inputs, f.Rows, evidence))
	gv2Git(t, root, "add", "-A")
	gv2GitAs(t, root, f.AuthorName, f.AuthorEmail, "commit", "-q", "-m", "Evidence: witness rows")
	f.Feature = gv2Git(t, root, "rev-parse", "HEAD")

	gv2Git(t, root, "checkout", "-q", "main")
	gv2Git(t, root, "merge", "-q", "--no-ff", "-m", "Merge pull request #"+gv2CR+" from example-org/fixture-branch", "fixture/branch")
	f.Merge = gv2Git(t, root, "rev-parse", "HEAD")
	if len(f.AfterMerge) > 0 {
		for rel, content := range f.AfterMerge {
			gv2Write(t, root, rel, content)
		}
		gv2Git(t, root, "add", "-A")
		gv2Git(t, root, "commit", "-q", "-m", "fixture: after merge")
	}
	return f
}

// rewriteVerify rewrites the brief's Verify rows on main (keeping its
// Evidence), as AfterMerge content.
func (f *gitV2Fixture) briefWithRows(rows []gv2Row) string {
	f.t.Helper()
	raw, err := os.ReadFile(f.BriefPath)
	if err != nil {
		f.t.Fatal(err)
	}
	s := string(raw)
	start := strings.Index(s, "## Verify\n")
	end := strings.Index(s, "\n\n## Evidence")
	if start < 0 || end < 0 {
		f.t.Fatal("brief layout")
	}
	return s[:start] + "## Verify\n" + gv2VerifyTable(rows) + s[end:]
}

// coverage evaluates the fixture in-process, as `--coverage` does.
func (f *gitV2Fixture) coverage() Coverage {
	f.t.Helper()
	s := &Stream{Name: gv2Stream, Dir: filepath.Join(f.Root, "docs", "streams", gv2Stream), Root: f.Root}
	cov := evaluateCoverage(f.Root, []*Stream{s}, coverageOptions{})
	c, ok := cov[gv2BriefID]
	if !ok {
		f.t.Fatalf("no coverage for %s: %v", gv2BriefID, cov)
	}
	return c
}

// claimFor returns the claim for Verify row id.
func claimFor(t *testing.T, c Coverage, id string) Claim {
	t.Helper()
	prefix := "Verify row #" + id + ": "
	for _, cl := range c.Claims {
		if strings.HasPrefix(cl.Claim, prefix) {
			return cl
		}
	}
	t.Fatalf("no claim for row %s in %+v", id, c.Claims)
	return Claim{}
}

// cliCoverage runs the built binary's `--coverage --json` over the fixture.
func (f *gitV2Fixture) cliCoverage(bin string) []coverageBriefJSON {
	f.t.Helper()
	out := runCLIJSON(f.t, bin, f.Root, "--coverage")
	var rows []coverageBriefJSON
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		f.t.Fatalf("decode --coverage --json: %v\n%s", err, out)
	}
	return rows
}

func cliClaimFor(t *testing.T, rows []coverageBriefJSON, id string) coverageClaimJSON {
	t.Helper()
	for _, b := range rows {
		if b.BriefID != gv2BriefID {
			continue
		}
		for _, cl := range b.Claims {
			if strings.HasPrefix(cl.Claim.Claim, "Verify row #"+id+": ") {
				return cl
			}
		}
	}
	t.Fatalf("no CLI claim for row %s", id)
	return coverageClaimJSON{}
}

// TestGitV2FixtureReleases is the fixture's own positive control: every v2
// row of the default fixture releases through the real git port.
func TestGitV2FixtureReleases(t *testing.T) {
	f := newGitV2Fixture(t).build()
	c := f.coverage()
	for _, id := range []string{"1", "2"} {
		cl := claimFor(t, c, id)
		if !cl.Released || cl.Result != covPass {
			t.Errorf("row %s: want a release, got %s: %s", id, cl.Result, cl.Reason)
		}
		if cl.Revision != f.Base {
			t.Errorf("row %s: revision %q, want the record's informational.commit %s", id, cl.Revision, f.Base)
		}
	}
}

func mustParseBriefFile(t *testing.T, path string) *BriefFile {
	t.Helper()
	bf, ok, err := parseBriefFile(path)
	if err != nil || !ok {
		t.Fatalf("parse %s: ok=%v err=%v", path, ok, err)
	}
	return bf
}
