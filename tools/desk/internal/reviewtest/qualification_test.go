package reviewtest_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
	q "github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

var binaries struct {
	sync.Once
	dir string
	err error
}

func TestMain(m *testing.M) {
	code := gitquiet.Run(m)
	if binaries.dir != "" {
		_ = os.RemoveAll(binaries.dir)
	}
	os.Exit(code)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	return root
}

func build(t *testing.T) string {
	t.Helper()
	binaries.Do(func() {
		binaries.dir, binaries.err = os.MkdirTemp("", "review-qualification-")
		if binaries.err != nil {
			return
		}
		for _, tool := range []string{"deskboard", "reviewloop", "deskdispatch", "issueboard", "deskinbox"} {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			cmd := exec.CommandContext(ctx, "go", "test", "-c", "-o", filepath.Join(binaries.dir, tool+".test"), "./cmd/"+tool)
			cmd.Dir = moduleRoot(t)
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
			b, e := goquiet.Cmd(cmd).CombinedOutput()
			cancel()
			if e != nil {
				binaries.err = fmt.Errorf("build %s: %w\n%s", tool, e, b)
				return
			}
		}
	})
	if binaries.err != nil {
		t.Fatal(binaries.err)
	}
	return binaries.dir
}

// CPU values belong to this process only. Inclusive/descendant coverage remains
// unknown until independent child attribution is supplied by desk-tools/38.
type invocation struct {
	Tool            string      `json:"tool"`
	Start           time.Time   `json:"start"`
	End             time.Time   `json:"end"`
	OwnCPUMs        *float64    `json:"ownCpuMs"`
	DescendantCPUMs *float64    `json:"descendantCpuMs"`
	InclusiveCPUMs  *float64    `json:"inclusiveCpuMs"`
	Exit            int         `json:"exit"`
	Observation     *q.Response `json:"observation,omitempty"`
}

func isolatedEnv(home string) []string {
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "USERPROFILE=" + home, "TMPDIR=" + os.TempDir(), "KUBECONFIG=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "ASSAY_TELEMETRY=0"}
}

func probe(t *testing.T, bin, tool string, request q.Request, root string) (q.Response, invocation) {
	t.Helper()
	dir, e := os.MkdirTemp(root, "probe-")
	if e != nil {
		t.Fatal(e)
	}
	request.Root = dir
	request.Home = filepath.Join(root, tool+"-home")
	b, e := json.Marshal(request)
	if e != nil {
		t.Fatal(e)
	}
	in, out := filepath.Join(dir, "request.json"), filepath.Join(dir, "response.json")
	if e = os.WriteFile(in, b, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	probeName := "TestQualificationProbe"
	if strings.HasPrefix(request.Case, "queue-") {
		probeName = "TestQueueBridgeProbe"
	}
	cmd := exec.CommandContext(ctx, filepath.Join(bin, tool+".test"), "-test.run=^"+probeName+"$", "-test.v")
	cmd.Dir = dir
	cmd.Env = append(isolatedEnv(dir), q.RequestEnv+"="+in, q.ResponseEnv+"="+out)
	v := invocation{Tool: tool, Start: time.Now()}
	log, e := cmd.CombinedOutput()
	v.End = time.Now()
	if cmd.ProcessState != nil {
		cpu := float64((cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()).Nanoseconds()) / 1e6
		v.OwnCPUMs = &cpu
		v.Exit = cmd.ProcessState.ExitCode()
	}
	if e != nil || !strings.Contains(string(log), "--- PASS: "+probeName+" ") {
		t.Fatalf("%s/%s probe: %v\n%s", tool, request.Case, e, log)
	}
	b, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var response q.Response
	if e = json.Unmarshal(b, &response); e != nil {
		t.Fatal(e)
	}
	v.Observation = &response
	return response, v
}

type oracleCase struct {
	Name    string                     `json:"name"`
	Tool    string                     `json:"tool"`
	Request q.Request                  `json:"request"`
	Want    map[string]json.RawMessage `json:"want"`
}

func fixture(t *testing.T, name string, v any) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, v); e != nil {
		t.Fatal(e)
	}
	return b
}

// compare checks literal, separately authored observations. It never derives an
// expected action, head, lane, hold, order or depth using production policy.
func compare(got q.Response, want map[string]json.RawMessage) error {
	b, _ := json.Marshal(got)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(b, &fields)
	for key, raw := range want {
		if key == "outputContains" {
			var tokens []string
			if err := json.Unmarshal(raw, &tokens); err != nil {
				return err
			}
			for _, token := range tokens {
				if !strings.Contains(got.Output, token) {
					return fmt.Errorf("command output missing %q", token)
				}
			}
			continue
		}
		if key == "outputRows" {
			var expected []string
			if err := json.Unmarshal(raw, &expected); err != nil {
				return err
			}
			actual := printedRows(got.Output)
			if !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("command output rows got %q; want %q", actual, expected)
			}
			continue
		}
		if key == "contains" {
			var tokens []string
			_ = json.Unmarshal(raw, &tokens)
			for _, token := range tokens {
				if !strings.Contains(got.Facts["output"], token) {
					return fmt.Errorf("missing output %q", token)
				}
			}
			continue
		}
		var actual, expected any
		if e := json.Unmarshal(fields[key], &actual); e != nil {
			return e
		}
		if e := json.Unmarshal(raw, &expected); e != nil {
			return e
		}
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("%s got %s; want %s", key, fields[key], raw)
		}
	}
	return nil
}

// printedRows reads only the actual command stream. It normalizes column padding,
// never derives a disposition/head/score from internal board state. Comparing the
// whole ordered slice rejects missing, extra and reordered visible rows.
func printedRows(output string) []string {
	rows := []string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.Contains(fields[0], "/") || !strings.Contains(fields[0], "#") {
			continue
		}
		if len(fields) > 5 {
			fields = fields[:5]
		}
		rows = append(rows, strings.Join(fields, " "))
	}
	return rows
}

func TestWorkflowOracle(t *testing.T) {
	bin := build(t)
	var cases []oracleCase
	fixture(t, "oracle.json", &cases)
	if len(cases) < 15 {
		t.Fatal("oracle population unexpectedly narrowed")
	}
	var mixed q.Response
	var mixedWant map[string]json.RawMessage
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			got, _ := probe(t, bin, c.Tool, c.Request, t.TempDir())
			if e := compare(got, c.Want); e != nil {
				t.Fatal(e)
			}
			if c.Name == "planner-mixed" {
				mixed = got
				mixedWant = c.Want
			}
		})
	}
	// Negative controls run the SAME comparator used above, not a second assertion.
	if len(mixed.Rows) != 5 {
		t.Fatal("mixed control did not run")
	}
	for _, mutation := range []string{"wrong-head", "missing-WAIT", "changed-order", "wrong-depth", "missing-printed-WAIT", "reordered-printed-rows", "wrong-printed-depth", "missing-printed-proposal", "wrong-printed-idle"} {
		t.Run(mutation, func(t *testing.T) {
			b, _ := json.Marshal(mixed)
			var changed q.Response
			_ = json.Unmarshal(b, &changed)
			switch mutation {
			case "wrong-head":
				changed.Rows[0].Head = strings.Repeat("a", 39) + "b"
			case "missing-WAIT":
				changed.Rows = append(changed.Rows[:1], changed.Rows[2:]...)
			case "changed-order":
				changed.Rows[0], changed.Rows[1] = changed.Rows[1], changed.Rows[0]
			case "wrong-depth":
				changed.Depth["DISPATCH"] = 1
			case "missing-printed-WAIT":
				changed.Output = withoutPrintedRow(changed.Output, "example-org/tracker#2")
			case "reordered-printed-rows":
				lines := strings.Split(changed.Output, "\n")
				first, second := -1, -1
				for i, line := range lines {
					if strings.HasPrefix(line, "example-org/tracker#1 ") {
						first = i
					}
					if strings.HasPrefix(line, "example-org/tracker#2 ") {
						second = i
					}
				}
				if first < 0 || second < 0 {
					t.Fatal("missing printed row control")
				}
				lines[first], lines[second] = lines[second], lines[first]
				changed.Output = strings.Join(lines, "\n")
			case "wrong-printed-depth":
				changed.Output = strings.Replace(changed.Output, "dispositions: 2 DISPATCH", "dispositions: 1 DISPATCH", 1)
			case "missing-printed-proposal":
				changed.Output = strings.ReplaceAll(changed.Output, "example-org/tracker#1@aaaaaaaaaaaa:review", "removed-proposal")
			case "wrong-printed-idle":
				changed.Output = strings.ReplaceAll(changed.Output, "NOT-IDLE — 1 NEEDS-REVIEW, 1 RE-REVIEW", "IDLE — incorrectly quiet")
			}
			if compare(changed, mixedWant) == nil {
				t.Fatal("oracle accepted planted mutation")
			}
		})
	}
	// Mutate the production rendering path, leaving every internal observation
	// intact. These overlays never edit the checkout and must fail the same oracle.
	for _, mutation := range []struct{ tool, path, old, new, fixture, diagnostic string }{
		{"reviewloop", "cmd/reviewloop/plan.go", "renderRows(board, stdout)", `fmt.Fprintln(stdout, "rows omitted")`, "planner-mixed", "command output rows"},
		{"deskdispatch", "cmd/deskdispatch/reviewround.go", "Scope: DELTA (clause 19)", "Scope: OMITTED (clause 19)", "dispatch-delta", "command output missing"},
	} {
		t.Run("production-"+mutation.tool, func(t *testing.T) {
			var selected *oracleCase
			for i := range cases {
				if cases[i].Name == mutation.fixture {
					selected = &cases[i]
					break
				}
			}
			if selected == nil {
				t.Fatal("production mutation has no literal fixture")
			}
			mutantBin := buildMutant(t, mutation.tool, mutation.path, mutation.old, mutation.new)
			got, _ := probe(t, mutantBin, mutation.tool, selected.Request, t.TempDir())
			if err := compare(got, selected.Want); err == nil || !strings.Contains(err.Error(), mutation.diagnostic) {
				t.Fatalf("production rendering mutation escaped its output assertion: %v", err)
			}
		})
	}
	// Retain the existing production-entrypoint regressions for applicability and
	// body-only edits. These have independent literal assertions, not golden output.
	for tool, pattern := range map[string]string{
		"deskboard":    "TestBodyEdit_BoardActionsEndToEnd|TestSweepDegradesOneRowNotTheBoard|TestCIState_LatestRunPerName|TestZeroCI_NoChecks_PathFiltered|TestZeroCI_ChecksNeverRan_1652|TestMergeNow_ApprovedThenPush_ReReview",
		"deskdispatch": "TestRoundReadComparesTheHeadAVerdictNamesWithTheForges|TestLaneVerdictsAreTheLanesOwn|TestGateHoldsOnEachCondition",
	} {
		cmd := exec.Command(filepath.Join(bin, tool+".test"), "-test.v", "-test.run=^("+pattern+")$")
		cmd.Env = isolatedEnv(t.TempDir())
		b, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("%s applicability: %v\n%s", tool, e, b)
		}
		for _, name := range strings.Split(pattern, "|") {
			if !strings.Contains(string(b), "--- PASS: "+name+" ") {
				t.Errorf("named production case %s did not pass", name)
			}
		}
	}
}

func buildMutant(t *testing.T, tool, relative, old, replacement string) string {
	t.Helper()
	root := moduleRoot(t)
	original := filepath.Join(root, filepath.FromSlash(relative))
	source, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(source), old) != 1 {
		t.Fatal("production mutation target must match exactly once")
	}
	dir := t.TempDir()
	changed := filepath.Join(dir, "mutant.go")
	if err = os.WriteFile(changed, []byte(strings.Replace(string(source), old, replacement, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{original: changed}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err = os.WriteFile(path, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", path, "-c", "-o", filepath.Join(dir, tool+".test"), "./cmd/"+tool)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	if b, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
		t.Fatalf("build production mutation: %v\n%s", err, b)
	}
	return dir
}

func withoutPrintedRow(output, id string) string {
	var kept []string
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, id+" ") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// Expect is the literal, separately authored result for the whole cohort.
type cohort struct {
	Name, Cache, BoardCase, IssueCase, Expect string
	Size, Concurrency, Cycles                 int
}

const partialResult = "failed-or-partial"

// cohortResult reads every invocation's own exit and, for board observations,
// both partial-population signals. A board observation that omits either signal
// is an error, never a clean read; any nonzero count is partial, not only one.
func cohortResult(events []invocation) (string, error) {
	result := "ok"
	for i, v := range events {
		if v.Exit != 0 {
			result = partialResult
		}
		o := v.Observation
		if o == nil {
			continue
		}
		if o.Exit != 0 {
			result = partialResult
		}
		if v.Tool != "deskboard" {
			continue
		}
		degraded, err := strconv.Atoi(o.Facts["degradedRows"])
		if err != nil || degraded < 0 {
			return "", fmt.Errorf("invocation %d (deskboard): degradedRows unreadable: %q", i, o.Facts["degradedRows"])
		}
		if degraded > 0 {
			result = partialResult
		}
		switch o.Facts["truncated"] {
		case "true":
			result = partialResult
		case "false":
		default:
			return "", fmt.Errorf("invocation %d (deskboard): truncated unreadable: %q", i, o.Facts["truncated"])
		}
	}
	return result, nil
}

type manifest struct {
	Schema         string            `json:"schema"`
	Revision       string            `json:"revision"`
	Dirty          bool              `json:"dirty"`
	FixtureSHA256  string            `json:"fixtureSha256"`
	ToolVersion    string            `json:"toolVersion"`
	BinarySHA256   map[string]string `json:"binarySha256"`
	PluginVersion  string            `json:"pluginVersion"`
	Host           string            `json:"host"`
	Method         string            `json:"method"`
	Seed           int               `json:"seed"`
	Arm            string            `json:"arm"`
	Supervision    string            `json:"supervision"`
	Cohort         cohort            `json:"cohort"`
	Result         string            `json:"result"`
	Start          time.Time         `json:"start"`
	End            time.Time         `json:"end"`
	ParentOwnCPUMs *int64            `json:"parentOwnCpuMs"`
	Invocations    []invocation      `json:"invocations"`
	Coverage       map[string]string `json:"coverage"`
}

// unionWall avoids adding overlapping child wall intervals. Cost comparisons must
// use the cycle envelope for total wall and exclusive own CPU for known CPU sums.
func unionWall(events []invocation) time.Duration {
	x := append([]invocation(nil), events...)
	sort.Slice(x, func(i, j int) bool { return x[i].Start.Before(x[j].Start) })
	var total time.Duration
	var lo, hi time.Time
	for _, v := range x {
		if lo.IsZero() {
			lo, hi = v.Start, v.End
			continue
		}
		if v.Start.After(hi) {
			total += hi.Sub(lo)
			lo, hi = v.Start, v.End
		} else if v.End.After(hi) {
			hi = v.End
		}
	}
	if !lo.IsZero() {
		total += hi.Sub(lo)
	}
	return total
}

func gitFixture(t *testing.T, root string) (string, string, string) {
	t.Helper()
	dir := filepath.Join(root, "git")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "core.hooksPath=" + os.DevNull, "-c", "commit.gpgsign=false", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(isolatedEnv(root), "GIT_AUTHOR_DATE=2026-10-17T12:00:00Z", "GIT_COMMITTER_DATE=2026-10-17T12:00:00Z", "GIT_CONFIG_COUNT=0", "GIT_TEMPLATE_DIR=")
		b, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("git fixture %v: %v\n%s", args, e, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "--quiet")
	for _, body := range []string{"before\n", "after\n"} {
		if e := os.WriteFile(filepath.Join(dir, "fixture.txt"), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		git("add", "fixture.txt")
		git("commit", "--quiet", "-m", "fixture")
	}
	return dir, git("rev-parse", "HEAD^"), git("rev-parse", "HEAD")
}

// regression: #3768, #3871
func TestCostCohort(t *testing.T) {
	bin := build(t)
	var cohorts []cohort
	raw := fixture(t, "cohorts.json", &cohorts)
	oracle, e := os.ReadFile(filepath.Join("testdata", "oracle.json"))
	if e != nil {
		t.Fatal(e)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(append(raw, oracle...)))
	root := moduleRoot(t)
	gitRead := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		b, e := cmd.Output()
		if e != nil {
			t.Fatal(e)
		}
		return strings.TrimSpace(string(b))
	}
	revision, dirty := gitRead("rev-parse", "HEAD"), gitRead("status", "--porcelain") != ""
	binaryHashes := map[string]string{}
	for _, tool := range []string{"deskboard", "reviewloop", "deskdispatch", "issueboard"} {
		b, e := os.ReadFile(filepath.Join(bin, tool+".test"))
		if e != nil {
			t.Fatal(e)
		}
		binaryHashes[tool] = fmt.Sprintf("%x", sha256.Sum256(b))
	}
	var reports []manifest
	var partialRuns [][]invocation
	for _, c := range cohorts {
		t.Run(c.Name, func(t *testing.T) {
			if c.Size < 1 || c.Concurrency < 1 || c.Cycles < c.Concurrency || (c.Expect != "ok" && c.Expect != partialResult) {
				t.Fatal("invalid cohort")
			}
			fixtureRoot := t.TempDir()
			gitDir, base, head := gitFixture(t, fixtureRoot)
			before, beforeOK := perfrec.CPUMillis()
			m := manifest{Schema: "review-cost-fixture-v1", Revision: revision, Dirty: dirty, FixtureSHA256: digest, ToolVersion: "source test binaries; " + runtime.Version(), BinarySHA256: binaryHashes, PluginVersion: "not invoked; caller-only/combined arm unqualified", Host: fmt.Sprintf("%s/%s CPUs=%d GOMAXPROCS=%d", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.GOMAXPROCS(0)),
				Method: "prebuilt command test adapters; literal oracle; local transport; fixture setup/build excluded", Seed: 37, Arm: "tool-only", Supervision: "fixed: board + plan + dispatch for each review candidate + local git agent + issue scan", Cohort: c, Start: time.Now(), Coverage: map[string]string{
					"ownCpu":        "OS ProcessState user+system for direct subprocesses; parent RUSAGE_SELF where supported",
					"descendantCpu": "unknown; direct-child measurements are not an inclusive process tree",
					"http":          "issue enumeration: actual production HTTP attempts at transport; other typed fixtures bypass HTTP, coverage partial",
					"retries":       "unknown: repeated endpoint attempts retained; no retry lineage signal yet",
					"points":        "unknown; fixtures cannot establish billable GitHub REST/GraphQL cost",
					"agent":         "local git diff fixture; no model/launcher CPU measured, production workflow coverage partial",
					"cache":         "fresh processes; steady cycles reuse per-slot command homes and Git objects; typed fake reads leave token/audit/HTTP cache behavior unqualified",
					"wall":          "cycle envelope; child interval union available; never sum overlapping durations",
				}}
			var mu sync.Mutex
			var wg sync.WaitGroup
			slots := make(chan int, c.Concurrency)
			for slot := 0; slot < c.Concurrency; slot++ {
				slots <- slot
			}
			for cycle := 0; cycle < c.Cycles; cycle++ {
				slot := <-slots
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { slots <- slot }()
					cycleRoot := filepath.Join(fixtureRoot, fmt.Sprintf("slot-%d", slot))
					if e := os.MkdirAll(cycleRoot, 0700); e != nil {
						t.Error(e)
						return
					}
					var events []invocation
					board, ev := probe(t, bin, "deskboard", q.Request{Case: c.BoardCase, Size: c.Size, Head: head}, cycleRoot)
					events = append(events, ev)
					if c.BoardCase == "access-failure" {
						if board.Exit != 6 || len(board.Rows) != 0 {
							t.Error("failed enumeration became a clean/partial board")
						}
					} else if board.Exit != 0 || len(board.Rows) != c.Size {
						t.Errorf("board scope changed: exit=%d rows=%d want=%d", board.Exit, len(board.Rows), c.Size)
					}
					if board.Facts["truncated"] != fmt.Sprint(c.Size >= 100) {
						t.Error("population completeness was lost")
					}
					a, p := board.Actions, board.PRs
					if board.Exit != 0 {
						a = []byte(`{}`)
						p = []byte(`{"prs":[]}`)
					}
					planner, event := probe(t, bin, "reviewloop", q.Request{Case: "pipeline", Size: c.Size, Actions: a, PRs: p, Now: "2026-10-17T12:01:00Z"}, cycleRoot)
					ev = event
					if len(planner.Rows) != len(board.Rows) {
						t.Error("planner dropped board rows")
					}
					if len(printedRows(planner.Output)) != len(board.Rows) {
						t.Error("planner output dropped board rows")
					}
					if (c.Size >= 100 || board.Exit != 0) && planner.Exit != 6 {
						t.Error("failed/capped population was reported as complete")
					}

					events = append(events, ev)
					for _, row := range board.Rows {
						if row.Action != "NEEDS-REVIEW" && row.Action != "RE-REVIEW" {
							continue
						}
						dispatch, event := probe(t, bin, "deskdispatch", q.Request{Case: "delta", Size: 1, Head: row.Head, Base: base}, cycleRoot)
						ev = event
						if dispatch.Facts["head"] != row.Head || dispatch.Exit != 0 {
							t.Error("dispatcher did not bind board head")
						}
						if !strings.Contains(dispatch.Output, "`git diff "+base+" "+row.Head+"`") {
							t.Error("dispatcher output lost review endpoints")
						}
						events = append(events, ev)
						cmd := exec.Command("git", "--no-pager", "diff", "--no-ext-diff", "--no-textconv", "--numstat", base, head)
						cmd.Dir = gitDir
						cmd.Env = isolatedEnv(cycleRoot)
						x := invocation{Tool: "fixture-agent/git-diff", Start: time.Now()}
						b, e := cmd.CombinedOutput()
						x.End = time.Now()
						if e != nil || strings.TrimSpace(string(b)) != "1\t1\tfixture.txt" {
							t.Errorf("agent diff: %v %s", e, b)
						}
						if cmd.ProcessState != nil {
							cpu := float64((cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()).Nanoseconds()) / 1e6
							x.OwnCPUMs = &cpu
							x.Exit = cmd.ProcessState.ExitCode()
						}
						events = append(events, x)
					}
					issues, event := probe(t, bin, "issueboard", q.Request{Case: c.IssueCase, Size: c.Size}, cycleRoot)
					ev = event
					if c.IssueCase == "open" && (issues.Exit != 0 || strings.Count(issues.Facts["output"], "CREATE-PLACEHOLDER") != c.Size) {
						t.Error("issue cohort lost work")
					}
					if c.IssueCase == "access-failure" && issues.Exit != 6 {
						t.Error("issue access failure became clean")
					}
					events = append(events, ev)
					mu.Lock()
					m.Invocations = append(m.Invocations, events...)
					mu.Unlock()
				}()
			}
			wg.Wait()
			m.End = time.Now()
			after, afterOK := perfrec.CPUMillis()
			if beforeOK && afterOK {
				delta := after - before
				m.ParentOwnCPUMs = &delta
			}
			expectedInvocations := 3 * c.Cycles
			if c.BoardCase == "needs-review" || c.BoardCase == "partial" {
				expectedInvocations += 2 * c.Size * c.Cycles
			}
			if len(m.Invocations) != expectedInvocations {
				t.Fatal("missing cycle invocations")
			}
			attempts := 0
			for i, v := range m.Invocations {
				if v.Observation != nil {
					// Each enumeration records its own attempts; another cycle's cannot stand in.
					if v.Tool == "issueboard" && len(v.Observation.Attempts) == 0 {
						t.Fatalf("invocation %d: issue enumeration recorded no HTTP attempt", i)
					}
					for _, a := range v.Observation.Attempts {
						attempts++
						if a.Points != nil {
							t.Fatal("invented fixture quota points")
						}
					}
				}
			}
			if attempts == 0 {
				t.Fatal("production HTTP boundary was never exercised")
			}
			result, err := cohortResult(m.Invocations)
			if err != nil {
				t.Fatal(err)
			}
			m.Result = result
			if m.Result != c.Expect {
				t.Fatalf("cohort result=%s; want %s", m.Result, c.Expect)
			}
			if c.Expect == partialResult {
				partialRuns = append(partialRuns, m.Invocations)
			}
			if unionWall(m.Invocations) > m.End.Sub(m.Start) {
				t.Fatal("overlapping child wall double-counted")
			}
			reports = append(reports, m)
			t.Logf("%s: cycles=%d invocations=%d HTTP attempts=%d result=%s; CPU/HTTP coverage partial", c.Name, c.Cycles, len(m.Invocations), attempts, m.Result)
		})
	}
	// Plant partial signals on a real degraded run, reusing the classifier above.
	// The zeroed control proves the count is that run's only partial signal.
	var degradedRun []invocation
	for _, run := range partialRuns {
		for _, v := range run {
			if v.Tool == "deskboard" && v.Observation.Facts["degradedRows"] != "0" {
				degradedRun = run
			}
		}
	}
	if degradedRun == nil {
		t.Fatal("no cohort observed a degraded board row")
	}
	for _, plant := range []struct{ degraded, truncated, want string }{
		{"0", "false", "ok"},
		{"2", "false", partialResult},
		{"0", "true", partialResult},
		{"", "false", ""},
		{"0", "", ""},
	} {
		changed := append([]invocation(nil), degradedRun...)
		for i, v := range changed {
			if v.Tool != "deskboard" {
				continue
			}
			o := *v.Observation
			o.Facts = map[string]string{}
			for k, x := range v.Observation.Facts {
				o.Facts[k] = x
			}
			for k, x := range map[string]string{"degradedRows": plant.degraded, "truncated": plant.truncated} {
				if x == "" {
					delete(o.Facts, k)
				} else {
					o.Facts[k] = x
				}
			}
			changed[i].Observation = &o
		}
		got, err := cohortResult(changed)
		if (plant.want == "") != (err != nil) || got != plant.want {
			t.Fatalf("planted degradedRows=%q truncated=%q: result=%q err=%v; want %q", plant.degraded, plant.truncated, got, err, plant.want)
		}
	}
	// A null measurement must survive serialization; unknown is never encoded as 0.
	b, e := json.MarshalIndent(reports, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if err := checkCostUnknowns(b); err != nil {
		t.Fatal(err)
	}
	// Exercise the same report check with missing and fabricated values at both
	// ends of the real output. An unrelated null must never mask either one.
	for _, key := range []string{"descendantCpuMs", "inclusiveCpuMs", "points", "retry"} {
		needle := `"` + key + `": null`
		for _, at := range []int{strings.Index(string(b), needle), strings.LastIndex(string(b), needle)} {
			if at < 0 {
				t.Fatalf("missing unknown-cost control field %s", key)
			}
			for _, replacement := range []string{`"` + key + `": 0`, `"removed-` + key + `": null`} {
				changed := string(b[:at]) + replacement + string(b[at+len(needle):])
				if err := checkCostUnknowns([]byte(changed)); err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("unknown-cost control %s escaped its assertion: %v", key, err)
				}
			}
		}
	}
	if path := os.Getenv("ASSAY_REVIEW_REPORT"); path != "" {
		if e := os.WriteFile(path, append(b, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	// Independent overlap control: summing these would wrongly report 4 seconds.
	t0 := time.Unix(0, 0)
	if unionWall([]invocation{{Start: t0, End: t0.Add(2 * time.Second)}, {Start: t0.Add(time.Second), End: t0.Add(3 * time.Second)}}) != 3*time.Second {
		t.Fatal("wall interval union is wrong")
	}
	t.Run("attempts-304-and-unknown", func(t *testing.T) {
		statuses := []int{200, 304, 503}
		i := 0
		transport := &q.Transport{Answer: func(*http.Request) (int, string) { status := statuses[i]; i++; return status, `{}` }}
		client := http.Client{Transport: transport}
		for range statuses {
			r, e := client.Get("https://fixture.invalid/repos/example-org/tracker/issues?page=1")
			if e != nil {
				t.Fatal(e)
			}
			_ = r.Body.Close()
		}
		attempts := transport.Snapshot()
		if len(attempts) != 3 {
			t.Fatal("repeated HTTP attempts were collapsed")
		}
		for i, a := range attempts {
			if a.Status != statuses[i] || a.Points != nil || a.Retry != nil || a.Query != "page=1" {
				t.Fatalf("lost attempt/304/unknown attribution: %+v", a)
			}
		}
	})
}

// checkCostUnknowns checks the serialized boundary, where a missing field must
// remain distinguishable from explicit null. Typed pointer decoding loses that
// distinction. Every invocation and attempt is checked, including failed cohorts
// and cohorts without a Git agent; a null elsewhere cannot cover an invented cost.
func checkCostUnknowns(raw []byte) error {
	var reports []struct {
		Cohort      cohort                       `json:"cohort"`
		Invocations []map[string]json.RawMessage `json:"invocations"`
	}
	if err := json.Unmarshal(raw, &reports); err != nil {
		return err
	}
	if len(reports) == 0 {
		return fmt.Errorf("missing cost reports")
	}
	for _, report := range reports {
		if len(report.Invocations) == 0 {
			return fmt.Errorf("%s: missing cost invocations", report.Cohort.Name)
		}
		for i, invocation := range report.Invocations {
			where := fmt.Sprintf("%s invocation %d (%s)", report.Cohort.Name, i, invocation["tool"])
			for _, key := range []string{"descendantCpuMs", "inclusiveCpuMs"} {
				if strings.TrimSpace(string(invocation[key])) != "null" {
					return fmt.Errorf("%s: %s must be explicit null, got %q", where, key, invocation[key])
				}
			}
			if observation, ok := invocation["observation"]; ok {
				var response struct {
					Attempts []map[string]json.RawMessage `json:"attempts"`
				}
				if err := json.Unmarshal(observation, &response); err != nil {
					return err
				}
				for j, attempt := range response.Attempts {
					for _, key := range []string{"points", "retry"} {
						if strings.TrimSpace(string(attempt[key])) != "null" {
							return fmt.Errorf("%s attempt %d: %s must be explicit null, got %q", where, j, key, attempt[key])
						}
					}
				}
			}
		}
	}
	return nil
}
