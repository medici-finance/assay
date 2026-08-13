package main

// zeroci_test.go — the #1652 zero-CI disambiguation: parser/matcher units, the
// classify contract for probed zero states, and end-to-end runs through the gh
// shim covering the two board cases (a checks-never-ran zero
// and a legitimate path-filtered zero) plus the
// fail-closed edges.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// workflowFixture builds the contents-API response body for one workflow file.
func workflowFixture(yaml string) string {
	return `{"content":"` + base64.StdEncoding.EncodeToString([]byte(yaml)) + `","encoding":"base64"}`
}

// ---------------------------------------------------------------------------
// parser units
// ---------------------------------------------------------------------------

func TestParseWorkflowPRTrigger(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		present  bool
		branches []string
		paths    []string
		pathsIgn []string
		types    []string
		wantErr  string
	}{
		{
			name:    "scalar on push",
			content: "on: push\n",
			present: false,
		},
		{
			name:    "flow list with pull_request",
			content: "on: [push, pull_request]\n",
			present: true,
		},
		{
			name:    "block sequence",
			content: "on:\n  - push\n  - pull_request\n",
			present: true,
		},
		{
			name:    "block sequence at column 0 (legal YAML)",
			content: "on:\n- push\n- pull_request\njobs:\n  t:\n    steps: []\n",
			present: true,
		},
		{
			name:    "block sequence without PR",
			content: "on:\n  - push\n  - workflow_dispatch\n",
			present: false,
		},
		{
			name:    "bare pull_request in map",
			content: "on:\n  pull_request:\n\njobs:\n  t:\n    steps: []\n",
			present: true,
		},
		{
			name:    "on with trailing comment still reads the map",
			content: "on: # triggers\n  pull_request:\n",
			present: true,
		},
		{
			name:    "quoted on key",
			content: "\"on\":\n  pull_request:\n",
			present: true,
		},
		{
			name:    "pull_request_target counts",
			content: "on:\n  pull_request_target:\n    branches: [main]\n",
			present: true, branches: []string{"main"},
		},
		{
			name: "full filter set",
			content: `on:
  pull_request:
    branches: [main]
    types: [opened, synchronize, reopened, labeled]
    paths:
      - "tools/**"
      - 'statusgen/**'   # trailing comment
    paths-ignore:
      - '**.md'
      - 'docs/**'
  push:
    branches: [main]
`,
			present:  true,
			branches: []string{"main"},
			types:    []string{"opened", "synchronize", "reopened", "labeled"},
			paths:    []string{"tools/**", "statusgen/**"},
			pathsIgn: []string{"**.md", "docs/**"},
		},
		{
			name: "non-PR subtrees are skipped untouched",
			content: `on:
  schedule:
    - cron: "0 6 * * *"
  workflow_dispatch:
    inputs:
      tag:
        description: 'Tag to release'
        required: true
  pull_request:
`,
			present: true,
		},
		{
			name:    "empty flow map on the event",
			content: "on:\n  pull_request: {}\n",
			present: true,
		},
		{
			name:    "bare scalar branch filter",
			content: "on:\n  pull_request:\n    branches: main\n",
			present: true, branches: []string{"main"},
		},
		{
			name:    "missing on is an error",
			content: "name: x\njobs:\n  t:\n    steps: []\n",
			wantErr: "no top-level `on:`",
		},
		{
			name:    "unsupported filter key is an error",
			content: "on:\n  pull_request:\n    if: something\n",
			wantErr: `unsupported filter key "if"`,
		},
		{
			name:    "multi-line scalar value is an error",
			content: "on:\n  pull_request:\n    paths: |\n      - x\n",
			wantErr: "multi-line scalar",
		},
		{
			name:    "empty flow list is an error",
			content: "on:\n  pull_request:\n    branches: []\n",
			wantErr: "empty list",
		},
		{
			name:    "duplicate filter key is an error",
			content: "on:\n  pull_request:\n    paths:\n      - a\n    paths:\n      - b\n",
			wantErr: `duplicate filter key "paths"`,
		},
		{
			name:    "list item before any key is an error",
			content: "on:\n  pull_request:\n      - orphan\n",
			wantErr: "before any filter key",
		},
		{
			name:    "inline value on the event is an error",
			content: "on:\n  pull_request: [opened]\n",
			wantErr: "inline value",
		},
		{
			name:    "list item after a flow value is an error",
			content: "on:\n  pull_request:\n    branches: [main]\n      - extra\n",
			wantErr: "follows an inline value",
		},
		{
			name:    "unevenly indented pull_request is an error, never silently missed",
			content: "on:\n  push:\n    pull_request: x\n",
			wantErr: "uneven event indentation",
		},
		{
			name:    "mixed block sequence and mapping is an error",
			content: "on:\n  - push\n  pull_request:\n",
			wantErr: "mixes a block sequence with a mapping",
		},
		// YAML anchors/aliases/merge keys. This parser resolves no node graph,
		// so every one of these is unmodelled — and each was previously read
		// LITERALLY, which is how `paths: *p` became the glob "*p", matched
		// nothing, and answered a confident wrong no-checks.
		{
			name:    "alias as a filter value is an error, never a literal glob",
			content: "x: &p ['a/**']\non:\n  pull_request:\n    paths: *p\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "alias as a filter list item is an error",
			content: "on:\n  pull_request:\n    paths:\n      - *p\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "anchor definition on a filter value is an error",
			content: "on:\n  pull_request:\n    paths: &p a/**\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "alias inside a filter flow list is an error",
			content: "on:\n  pull_request:\n    branches: [main, *rel]\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "alias in a branch filter is an error",
			content: "on:\n  pull_request:\n    branches: *b\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "merge key under a pull_request filter map is an error",
			content: "on:\n  pull_request:\n    <<: *base\n    paths: ['a/**']\n",
			wantErr: "merge/alias key",
		},
		// The same class one level up, at `on:` itself — where a dropped
		// pull_request trigger also reads as "nothing fires".
		{
			name:    "on: is an alias — an error, not an unrecognised event",
			content: "x: &ev [pull_request]\non: *ev\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "alias inside on:'s flow list is an error",
			content: "on: [push, *ev]\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "alias as an on: block-sequence item is an error",
			content: "on:\n  - push\n  - *ev\n",
			wantErr: "alias/anchor",
		},
		{
			name:    "merge key at the event level is an error, not a skipped event",
			content: "on:\n  <<: *base\n  push:\n    branches: [main]\n",
			wantErr: "merge/alias key",
		},
		// Tab indentation. Tabs are illegal YAML indentation and this parser
		// counts leading SPACES, so a tab makes it mis-read the nesting — the
		// on: block below reads as empty and the trigger silently vanishes.
		{
			name:    "tab-indented event is an error, not an empty on: block",
			content: "on:\n\tpull_request:\n\t\tpaths: ['a/**']\n",
			wantErr: "tab-indented",
		},
		{
			name:    "tab-indented filter line is an error",
			content: "on:\n  pull_request:\n\t\tpaths: ['a/**']\n",
			wantErr: "tab-indented",
		},
		{
			name:    "tab-indented on: sequence item is an error",
			content: "on:\n\t- pull_request\n",
			wantErr: "tab-indented",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr, err := parseWorkflowPRTrigger("w.yml", c.content)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("parseWorkflowPRTrigger = ( %+v, %v ), want error containing %q", tr, err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWorkflowPRTrigger error: %v", err)
			}
			if tr.present != c.present {
				t.Errorf("present = %t, want %t", tr.present, c.present)
			}
			if !reflect.DeepEqual(tr.branches, c.branches) {
				t.Errorf("branches = %v, want %v", tr.branches, c.branches)
			}
			if !reflect.DeepEqual(tr.paths, c.paths) {
				t.Errorf("paths = %v, want %v", tr.paths, c.paths)
			}
			if !reflect.DeepEqual(tr.pathsIgnore, c.pathsIgn) {
				t.Errorf("pathsIgnore = %v, want %v", tr.pathsIgnore, c.pathsIgn)
			}
			if !reflect.DeepEqual(tr.types, c.types) {
				t.Errorf("types = %v, want %v", tr.types, c.types)
			}
		})
	}
}

func TestWouldFire(t *testing.T) {
	files := []string{"tools/desk/board.go", "docs/x.md"}
	cases := []struct {
		name string
		tr   prTrigger
		base string
		file []string
		want bool
		err  string
	}{
		{"absent never fires", prTrigger{present: false}, "main", files, false, ""},
		{"no filters fires", prTrigger{present: true}, "main", files, true, ""},
		{"branches match", prTrigger{present: true, branches: []string{"main", "release/*"}}, "main", files, true, ""},
		{"branches mismatch", prTrigger{present: true, branches: []string{"release/*"}}, "main", files, false, ""},
		{"branches-ignore hit", prTrigger{present: true, branchesIgnore: []string{"wip*"}}, "wip-x", files, false, ""},
		{"branches-ignore miss", prTrigger{present: true, branchesIgnore: []string{"wip*"}}, "main", files, true, ""},
		{"types intersect", prTrigger{present: true, types: []string{"opened", "labeled"}}, "main", files, true, ""},
		{"types no intersect", prTrigger{present: true, types: []string{"closed"}}, "main", files, false, ""},
		{"paths match one file", prTrigger{present: true, paths: []string{"tools/**"}}, "main", files, true, ""},
		{"paths match none", prTrigger{present: true, paths: []string{"secrets/**"}}, "main", files, false, ""},
		{"paths ordered negation excludes", prTrigger{present: true, paths: []string{"**", "!docs/**"}}, "main",
			[]string{"docs/x.md"}, false, ""},
		{"paths negation re-includes", prTrigger{present: true, paths: []string{"docs/**", "!docs/skip.md", "docs/skip.md"}}, "main",
			[]string{"docs/skip.md"}, true, ""},
		{"paths-ignore all files ignored", prTrigger{present: true, pathsIgnore: []string{"**.md", "docs/**"}}, "main",
			[]string{"docs/x.md", "README.md"}, false, ""},
		{"paths-ignore one file survives", prTrigger{present: true, pathsIgnore: []string{"**.md", "docs/**"}}, "main",
			[]string{"docs/x.md", "tools/desk/board.go"}, true, ""},
		{"character-class glob errors", prTrigger{present: true, paths: []string{"src/[ab].go"}}, "main", files, false, "character class"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.tr.wouldFire(c.base, c.file)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("wouldFire = (%t, %v), want error containing %q", got, err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("wouldFire error: %v", err)
			}
			if got != c.want {
				t.Errorf("wouldFire(%s, %v) = %t, want %t", c.base, c.file, got, c.want)
			}
		})
	}
}

// TestGlobSemantics pins the matcher against GitHub's documented filter-pattern
// behaviour for the shapes the watched repos actually use.
func TestGlobSemantics(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"tools/**", "tools/desk/board.go", true},
		{"tools/**", "tools/x", true},
		{"tools/**", "statusgen/x", false},
		{"**.md", "README.md", true},
		{"**.md", "docs/deep/x.md", true},
		{"k8s/**/*.yaml", "k8s/dev/app/x.yaml", true},
		{"k8s/**/*.yaml", "k8s/x.yaml", true},
		{"k8s/**/*.yaml", "k8s/x.yml", false},
		{"app.yaml", "app.yaml", true},
		{"go.work", "go.work", true},
		{"release/*", "release/1", true},
		{"release/*", "release/1/2", false},
	}
	for _, c := range cases {
		re, err := wfGlobToRegexp(c.glob)
		if err != nil {
			t.Fatalf("wfGlobToRegexp(%q): %v", c.glob, err)
		}
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("glob %q vs %q = %t, want %t", c.glob, c.path, got, c.want)
		}
	}
}

func TestCIColumn(t *testing.T) {
	if got := ciColumn(2, 1, 0, ""); got != "2✓ 1pend 0fail" {
		t.Errorf("non-zero rollup must keep the classic rendering; got %q", got)
	}
	if got := ciColumn(0, 0, 0, zeroCINoChecks); got != "0 checks (no-checks)" {
		t.Errorf("no-checks rendering; got %q", got)
	}
	if got := ciColumn(0, 0, 0, zeroCINeverRan); got != "0 checks (NEVER-RAN)" {
		t.Errorf("checks-never-ran rendering; got %q", got)
	}
	if got := ciColumn(0, 0, 0, zeroCIUnverified); got != "0 checks (unverified)" {
		t.Errorf("unverified rendering; got %q", got)
	}
	// A caller that skipped the probe renders unverified — never a bare zero.
	if got := ciColumn(0, 0, 0, ""); got != "0 checks (unverified)" {
		t.Errorf("zero rollup without a probe state must render unverified, not a bare 0✓ 0pend 0fail; got %q", got)
	}
}

// TestClassify_ZeroCI pins the flip-side contract (#1652): checks-never-ran and
// unverified block the FLIP signal outright; a verified no-checks zero keeps
// the vacuous-green MERGE-NOW on CI-less repos but says so.
func TestClassify_ZeroCI(t *testing.T) {
	cases := []struct {
		name string
		in   classifyInput
		want string
		note string // substring the note must carry
	}{
		{"never-ran blocks flip", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, zeroCI: zeroCINeverRan},
			actCINeverRan, "UNVALIDATED"},
		{"unverified blocks flip", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, zeroCI: zeroCIUnverified},
			actCheck, "could NOT verify"},
		{"no-checks on a CI-required repo is a human call", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, zeroCI: zeroCINoChecks},
			actCheck, "human call"},
		{"no-checks vacuous green non-draft merges", classifyInput{ever: true, atHead: true, draft: false, approvedAtHead: true, ciGreen: true, zeroCI: zeroCINoChecks},
			actMergeNow, "CHECKED zero"},
		{"no-checks vacuous green draft merges", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, zeroCI: zeroCINoChecks},
			actMergeNow, "CHECKED zero"},
		{"no-checks risk-classed draft without secpass stays blocked", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, ciGreen: true, zeroCI: zeroCINoChecks, riskClassed: true},
			actSecReview, "security review required"},
		// #216: hoisting the security guard above the zero-CI switch
		// means a risk-classed PR without secpass stays SEC-REVIEW-REQUIRED under
		// every probed zero state — the switch no longer shadows the guard.
		{"never-ran risk-classed without secpass stays blocked", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, zeroCI: zeroCINeverRan, riskClassed: true},
			actSecReview, "security review required"},
		{"unverified risk-classed without secpass stays blocked", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, zeroCI: zeroCIUnverified, riskClassed: true},
			actSecReview, "security review required"},
		{"no-checks (!ciGreen) risk-classed without secpass stays blocked", classifyInput{ever: true, atHead: true, draft: true, approvedAtHead: true, zeroCI: zeroCINoChecks, riskClassed: true},
			actSecReview, "security review required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			act, note := classify(c.in)
			if act != c.want {
				t.Errorf("classify(%+v) = %s, want %s", c.in, act, c.want)
			}
			if c.note != "" && !strings.Contains(note, c.note) {
				t.Errorf("note = %q, want substring %q", note, c.note)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// end-to-end through the shim
// ---------------------------------------------------------------------------

// zeroCIEnv drives one zero-rollup PR through `actions` and returns its row.
func zeroCIEnv(t *testing.T, repo string, num int, extra func(t *testing.T)) actionRow {
	t.Helper()
	head := "zeroci0000head"
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":`+strconv.Itoa(num)+`,"title":"zero ci","isDraft":true,"author":{"login":"app/assay-worker-app"},`+
			`"headRefOid":"`+head+`","baseRefName":"main","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":"looks good\n\nSecurity-Review: pass\n","submitted_at":"2026-08-02T00:00:00Z"}]`)
	if extra != nil {
		extra(t)
	}

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	for _, r := range rep.Rows {
		if r.Number == num {
			return r
		}
	}
	t.Fatalf("PR #%d missing from actions report: %+v", num, rep.Rows)
	return actionRow{}
}

// TestZeroCI_ChecksNeverRan_1652 is the checks-never-ran case: a
// workflow at head would fire on this diff, no run exists — the row must say
// checks-never-ran and the action must NOT be a flip signal.
func TestZeroCI_ChecksNeverRan_1652(t *testing.T) {
	row := zeroCIEnv(t, "medici-finance/assay", 332, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
			`[{"name":"tools.yml","type":"file","path":".github/workflows/tools.yml"}]`)
		t.Setenv("DESKBOARD_GH_WORKFLOW_TOOLS_YML", workflowFixture(`name: tools
on:
  pull_request:
    paths:
      - "tools/**"
      - "statusgen/**"
  push:
    branches: [main]
`))
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"tools/desk/cmd/deskboard/board.go"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
	})
	if row.CIZero != zeroCINeverRan {
		t.Errorf("CIZero = %q, want %q (detail: %s)", row.CIZero, zeroCINeverRan, row.CIZeroDetail)
	}
	if !strings.Contains(row.CIZeroDetail, "tools.yml") {
		t.Errorf("CIZeroDetail should name the workflow that would fire; got %q", row.CIZeroDetail)
	}
	if row.Action != actCINeverRan {
		t.Errorf("Action = %s, want %s — a PR whose checks never ran is UNVALIDATED, not flippable", row.Action, actCINeverRan)
	}
	if !strings.Contains(row.Note, "UNVALIDATED") {
		t.Errorf("Note should carry the unvalidated warning; got %q", row.Note)
	}
}

// TestZeroCI_NoChecks_PathFiltered is the issue's examples#24 counter-case:
// the zero is legitimate (the only PR-triggering workflow's path filter
// excludes the diff) — the row says no-checks and the CI-less repo keeps its
// vacuous green, ANNOTATED.
func TestZeroCI_NoChecks_PathFiltered(t *testing.T) {
	row := zeroCIEnv(t, "example-org/examples", 24, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
			`[{"name":"build-go.yml","type":"file","path":".github/workflows/build-go.yml"},`+
				`{"name":"tools.yml","type":"file","path":".github/workflows/tools.yml"}]`)
		t.Setenv("DESKBOARD_GH_WORKFLOW_TOOLS_YML", workflowFixture(`name: tools
on:
  pull_request:
    paths:
      - 'tools/**'
      - 'go.work'
  push:
    branches: [main]
`))
		t.Setenv("DESKBOARD_GH_WORKFLOW_BUILD_GO_YML", workflowFixture(`name: build-go
on:
  push:
    branches: [main]
    paths:
      - 'go/**'
  workflow_dispatch:
`))
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"python/bots/sim.py"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
	})
	if row.CIZero != zeroCINoChecks {
		t.Errorf("CIZero = %q, want %q (detail: %s)", row.CIZero, zeroCINoChecks, row.CIZeroDetail)
	}
	// CI-less repo + verified no-checks → vacuous green stands, but the note
	// must SAY it is a checked zero, not let "CI green" silently cover it.
	if row.Action != actMergeNow {
		t.Errorf("Action = %s, want %s (verified no-checks on a CI-less repo keeps vacuous green)", row.Action, actMergeNow)
	}
	if !strings.Contains(row.Note, "CHECKED zero") {
		t.Errorf("MERGE-NOW note must annotate the checked zero; got %q", row.Note)
	}
}

// TestZeroCI_Unverified_RunsExist covers the suspect-read case: the rollup
// counted zero but check runs DO exist at head — the read is at fault, not
// the CI, and the row must say unverified rather than pick a side.
func TestZeroCI_Unverified_RunsExist(t *testing.T) {
	row := zeroCIEnv(t, "medici-finance/assay", 500, func(t *testing.T) {
		// Shim default: one completed/success check run — the probe must NOT
		// reach the workflow reads at all.
	})
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q (detail: %s)", row.CIZero, zeroCIUnverified, row.CIZeroDetail)
	}
	if row.Action != actCheck {
		t.Errorf("Action = %s, want %s — could-not-check is not a flip signal", row.Action, actCheck)
	}
}

// TestZeroCI_Unverified_AllSkipped: checks technically ran but produced no
// verdict — a flip needs at least one completed successful check (#1652 ask
// 2), so all-SKIPPED is not green.
func TestZeroCI_Unverified_AllSkipped(t *testing.T) {
	row := zeroCIEnv(t, "medici-finance/assay", 501, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON",
			`{"total_count":1,"check_runs":[{"name":"ci","status":"completed","conclusion":"skipped","head_sha":"zeroci0000head"}]}`)
	})
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q (detail: %s)", row.CIZero, zeroCIUnverified, row.CIZeroDetail)
	}
	if !strings.Contains(row.CIZeroDetail, "SKIPPED") {
		t.Errorf("detail should name the all-skipped cause; got %q", row.CIZeroDetail)
	}
	if row.Action != actCheck {
		t.Errorf("Action = %s, want %s", row.Action, actCheck)
	}
}

// TestZeroCI_Unverified_CheckRunsReadError pins funnel K of R3 (PR #428 review
// @ d79aafd1): a transient failure reading check-runs at head must NEVER fall
// through to no-checks. Before this test the funnel was asserted in prose
// ("the fail-closed contract is asserted at 11 call sites") but never
// exercised end-to-end — a K-shaped mutant (treat the read error as
// "no runs exist") survived the suite.
func TestZeroCI_Unverified_CheckRunsReadError(t *testing.T) {
	row := zeroCIEnv(t, "medici-finance/assay", 503, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_FAIL_PATH", "/check-runs")
	})
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q (detail: %s) — a check-runs read failure must never be read as a checked zero",
			row.CIZero, zeroCIUnverified, row.CIZeroDetail)
	}
	if row.CIZero == zeroCINoChecks {
		t.Fatalf("CIZero must never be %q on a read failure", zeroCINoChecks)
	}
	if row.Action != actCheck {
		t.Errorf("Action = %s, want %s — a read failure is could-not-check, never a flip signal", row.Action, actCheck)
	}
}

// TestZeroCI_Unverified_CombinedStatusExists pins funnel N of R3: check runs
// read as empty, but the combined commit status carries >=1 context — external
// CI (statuses, not check runs) is live at head, and the rollup's zero is a
// suspect READ, not a checked absence of CI. N is the most production-reachable
// mutant named in the review: "a repo whose external CI posts commit statuses
// rather than check runs — the mutant reads live external CI as 'no checks are
// configured'."
func TestZeroCI_Unverified_CombinedStatusExists(t *testing.T) {
	row := zeroCIEnv(t, "medici-finance/assay", 504, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_COMBINED_STATUS_JSON",
			`{"total_count":1,"statuses":[{"context":"external-ci/build","state":"success"}]}`)
	})
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q (detail: %s) — a non-empty combined status must never be read as no-checks",
			row.CIZero, zeroCIUnverified, row.CIZeroDetail)
	}
	if row.CIZero == zeroCINoChecks {
		t.Fatalf("CIZero must never be %q when external CI has posted a commit status", zeroCINoChecks)
	}
	if row.Action != actCheck {
		t.Errorf("Action = %s, want %s — could-not-check is not a flip signal", row.Action, actCheck)
	}
}

// TestZeroCI_Unverified_ChangedFilesTruncated pins funnel T of R3: nothing ran,
// a workflow with a `paths:` filter exists, but the changed-file read GitHub
// reconciles against its own PR metadata is truncated — the probe cannot prove
// no path-filtered workflow would have fired, so it must refuse to answer
// no-checks (the truncated-changed-files half of the same fail-closed contract
// changedfiles_test.go pins for the risk-path scan).
func TestZeroCI_Unverified_ChangedFilesTruncated(t *testing.T) {
	row := zeroCIEnv(t, "medici-finance/assay", 505, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
			`[{"name":"tools.yml","type":"file","path":".github/workflows/tools.yml"}]`)
		t.Setenv("DESKBOARD_GH_WORKFLOW_TOOLS_YML", workflowFixture(`name: tools
on:
  pull_request:
    paths:
      - "tools/**"
`))
		// GitHub says 652 changed files; the files endpoint hands back one — the
		// classic changed-files truncation shape (board.go's fetchChangedFiles).
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"tools/desk/cmd/deskboard/board.go"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":652}`)
	})
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q (detail: %s) — a truncated changed-file list must never resolve to no-checks",
			row.CIZero, zeroCIUnverified, row.CIZeroDetail)
	}
	if row.CIZero == zeroCINoChecks {
		t.Fatalf("CIZero must never be %q when the changed-file list is truncated", zeroCINoChecks)
	}
	if !strings.Contains(row.CIZeroDetail, "TRUNCATED") {
		t.Errorf("detail should name the truncation; got %q", row.CIZeroDetail)
	}
	if row.Action != actCheck {
		t.Errorf("Action = %s, want %s — could-not-check is not a flip signal", row.Action, actCheck)
	}
}

// TestZeroCI_NoChecks_NoWorkflowsDir: a repo with no .github/workflows at
// head is a verified no-checks — the 404 is an answer, not an error.
func TestZeroCI_NoChecks_NoWorkflowsDir(t *testing.T) {
	row := zeroCIEnv(t, "example-org/org-slides", 60, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		// No DESKBOARD_GH_WORKFLOWS_DIR_JSON — the shim answers 404.
	})
	if row.CIZero != zeroCINoChecks {
		t.Errorf("CIZero = %q, want %q (detail: %s)", row.CIZero, zeroCINoChecks, row.CIZeroDetail)
	}
}

// TestZeroCI_Unverified_UnparseableWorkflow is the fail-closed half (the
// mutation test): a workflow shape the parser does not model must surface as
// unverified, never silently as no-checks.
func TestZeroCI_Unverified_UnparseableWorkflow(t *testing.T) {
	row := zeroCIEnv(t, "medici-finance/assay", 502, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
			`[{"name":"ci.yml","type":"file","path":".github/workflows/ci.yml"}]`)
		t.Setenv("DESKBOARD_GH_WORKFLOW_CI_YML", workflowFixture("on:\n  pull_request:\n    paths: |\n      - x\n"))
	})
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q — an unmodelled workflow shape must fail closed (detail: %s)",
			row.CIZero, zeroCIUnverified, row.CIZeroDetail)
	}
	if row.Action != actCheck {
		t.Errorf("Action = %s, want %s", row.Action, actCheck)
	}
}

// zeroCIWorkflowRow drives one zero-rollup PR whose head carries a single
// workflow file with the given content, and returns its board row. Nothing ran
// at head, so the probe reaches the workflow parse — the point under test.
func zeroCIWorkflowRow(t *testing.T, num int, workflow string) actionRow {
	t.Helper()
	return zeroCIEnv(t, "medici-finance/assay", num, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
			`[{"name":"ci.yml","type":"file","path":".github/workflows/ci.yml"}]`)
		t.Setenv("DESKBOARD_GH_WORKFLOW_CI_YML", workflowFixture(workflow))
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"a/one.go"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
	})
}

// assertFailedClosed pins the PROPERTY, not a string: an unmodelled workflow
// shape must reach `unverified` and must never be reported as a checked zero,
// because a wrong no-checks waves an unvalidated PR through as vacuous green.
func assertFailedClosed(t *testing.T, row actionRow, shape string) {
	t.Helper()
	if row.CIZero == zeroCINoChecks {
		t.Fatalf("%s produced a CHECKED-ZERO claim (%q) — an unmodelled workflow shape must fail closed, "+
			"never render a vacuous green (detail: %s)", shape, row.CIZero, row.CIZeroDetail)
	}
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q — %s is an unmodelled shape (detail: %s)",
			row.CIZero, zeroCIUnverified, shape, row.CIZeroDetail)
	}
	if row.Action != actCheck {
		t.Errorf("Action = %s, want %s — could-not-check is never a flip signal", row.Action, actCheck)
	}
}

// TestZeroCI_Unverified_AliasWorkflow — a YAML alias in a path filter. The
// parser resolves no node graph, so taking `*p` literally made it a glob that
// matched no file in the diff; every workflow then "would not fire" and the row
// rendered a confident no-checks on a PR whose CI genuinely would have run.
// Anchors and aliases are legal YAML used for DRY workflow configs.
func TestZeroCI_Unverified_AliasWorkflow(t *testing.T) {
	row := zeroCIWorkflowRow(t, 503, "x: &p ['a/**']\non:\n  pull_request:\n    paths: *p\n")
	assertFailedClosed(t, row, "a YAML alias in paths:")
}

// TestZeroCI_Unverified_AliasAtOnLevel — the same class one level up: an alias
// spliced in at `on:` itself. Reading `*ev` as an unrecognised event name drops
// the pull_request trigger entirely and lands on the same wrong no-checks.
func TestZeroCI_Unverified_AliasAtOnLevel(t *testing.T) {
	row := zeroCIWorkflowRow(t, 504, "x: &ev [pull_request]\non: *ev\n")
	assertFailedClosed(t, row, "a YAML alias as the whole on: value")
}

// TestZeroCI_Unverified_MergeKeyWorkflow — a `<<:` merge key at the event level
// splices another map's events in. Skipping it as "some non-PR event" silently
// drops whatever pull_request trigger it carries.
func TestZeroCI_Unverified_MergeKeyWorkflow(t *testing.T) {
	row := zeroCIWorkflowRow(t, 505, "on:\n  <<: *base\n  push:\n    branches: [main]\n")
	assertFailedClosed(t, row, "a YAML merge key under on:")
}

// TestZeroCI_Unverified_TabIndentedWorkflow — tabs are illegal YAML
// indentation, so GitHub rejects the file and no-checks may even be the right
// ANSWER; the parser must not reach it by not seeing the pull_request block.
// It counts leading spaces, so a tab makes the on: block read as empty — the
// same blindness would answer "nothing fires" for any mis-tracked indentation.
func TestZeroCI_Unverified_TabIndentedWorkflow(t *testing.T) {
	row := zeroCIWorkflowRow(t, 506, "on:\n\tpull_request:\n\t\tpaths: ['a/**']\n")
	assertFailedClosed(t, row, "a tab-indented on: block")
}

// TestZeroCI_TableRendersTheState proves the human path carries the state on
// the line — the bare `0✓ 0pend 0fail` is gone for zero rollups.
func TestZeroCI_TableRendersTheState(t *testing.T) {
	head := "zeroci0000head"
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", "medici-finance/assay")
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":600,"title":"zero ci","isDraft":true,"author":{"login":"app/assay-worker-app"},`+
			`"headRefOid":"`+head+`","baseRefName":"main","mergeStateStatus":"CLEAN","statusCheckRollup":[]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON", `[]`)
	t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
	t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
		`[{"name":"tools.yml","type":"file","path":".github/workflows/tools.yml"}]`)
	t.Setenv("DESKBOARD_GH_WORKFLOW_TOOLS_YML", workflowFixture("on:\n  pull_request:\n"))

	var out, errb bytes.Buffer
	if code := run([]string{"actions", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions --table) = exit %d, stderr=%s", code, errb.String())
	}
	body := out.String()
	if !strings.Contains(body, "0 checks (NEVER-RAN)") {
		t.Errorf("table must render the never-ran state on the line; got:\n%s", body)
	}
	if strings.Contains(body, "0✓ 0pend 0fail") {
		t.Errorf("a zero rollup must NEVER render as a bare 0✓ 0pend 0fail again; got:\n%s", body)
	}
}

// TestZeroCI_NeverRan_OnCILessRepo exercises the ciGreen guard:
// a CI-less repo (CIRequired:false) gives a vacuously-green MERGE-NOW only when
// the probe confirms no-checks. checks-never-ran is NOT green on ANY repo.
// Without this test the guard mutation (ciGreen = !ciRequired) survives the full
// suite — every existing e2e test runs against a CI-required repo.
func TestZeroCI_NeverRan_OnCILessRepo(t *testing.T) {
	row := zeroCIEnv(t, "example-org/examples", 901, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
			`[{"name":"ci.yml","type":"file","path":".github/workflows/ci.yml"}]`)
		t.Setenv("DESKBOARD_GH_WORKFLOW_CI_YML",
			workflowFixture("on:\n  pull_request:\n    paths: ['a/**']\n"))
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"a/one.go"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
	})
	if row.CIZero != zeroCINeverRan {
		t.Errorf("CIZero = %q, want %q (detail: %s)", row.CIZero, zeroCINeverRan, row.CIZeroDetail)
	}
	// A CI-less repo must NOT wave a never-ran through as vacuous green.
	if row.Action == actMergeNow || row.Action == actFlip {
		t.Errorf("Action = %s — a CI-less repo's vacuous green must NOT cover checks-never-ran; "+
			"0 checks that should have run is absence of evidence, not green (detail: %s)",
			row.Action, row.CIZeroDetail)
	}
}

// TestZeroCI_Unverified_OnCILessRepo exercises the ciGreen guard for unverified
// on a CI-less repo. An unverified zero (e.g. alias workflow) is also not green.
func TestZeroCI_Unverified_OnCILessRepo(t *testing.T) {
	row := zeroCIEnv(t, "example-org/examples", 902, func(t *testing.T) {
		t.Setenv("DESKBOARD_GH_CHECKRUNS_JSON", `{"total_count":0,"check_runs":[]}`)
		t.Setenv("DESKBOARD_GH_WORKFLOWS_DIR_JSON",
			`[{"name":"ci.yml","type":"file","path":".github/workflows/ci.yml"}]`)
		t.Setenv("DESKBOARD_GH_WORKFLOW_CI_YML",
			workflowFixture("x: &p ['a/**']\non:\n  pull_request:\n    paths: *p\n"))
		t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"a/one.go"}]`)
		t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)
	})
	if row.CIZero != zeroCIUnverified {
		t.Errorf("CIZero = %q, want %q (detail: %s)", row.CIZero, zeroCIUnverified, row.CIZeroDetail)
	}
	if row.Action == actMergeNow || row.Action == actFlip {
		t.Errorf("Action = %s — an unverified zero is not green on any repo (detail: %s)",
			row.Action, row.CIZeroDetail)
	}
}
