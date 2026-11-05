package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the delivery half of the decision-gate hold and the design-approval
// gate (decisiongatedelivery.go, #5507): a pull request whose body names a brief
// in a Brief: trailer is judged as the move to implemented that reconcile makes
// for it on merge. Each test writes a pull_request event payload and points
// GITHUB_EVENT_NAME / GITHUB_EVENT_PATH at it, as the Actions runner does, so
// the real reader is under test.

const dvPR = 77

// dvEvent writes a pull_request event whose body is body, for repo.
func dvEvent(t *testing.T, repo, body string) {
	t.Helper()
	ev := map[string]any{
		"action":       "opened",
		"pull_request": map[string]any{"number": dvPR, "body": body},
		"repository":   map[string]any{"full_name": repo},
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_EVENT_PATH", p)
}

// dvBase is a gate: human brief at todo on base and head; the change writes no
// move. extra is added to its frontmatter.
func dvBase(t *testing.T, extra string) string {
	t.Helper()
	return dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", extra),
	})
}

// dvV1Lines make the brief a schema'd brief-v1, so its delivery: list is read.
const dvV1Lines = "schema: brief-v1\ndepends: []\nunblocks: []\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: []\nauthored: \"2026-08-01\"\nsources: []\n"

const dvBody = "Delivers the brief.\n\nBrief: " + dhBoardID + "\n"

// The defect: a delivering PR with no decision keys writes no move, so before
// the fix it passed. It is now refused, at the brief file, naming the PR.
func TestDecisionGateDeliveryUnkeyedRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dvBase(t, "")
	dvEvent(t, rlRepo, dvBody)
	p := dhLayerOne(t, root)
	if len(p) != 1 {
		t.Fatalf("layer one: %d problems on an unkeyed delivery, want 1: %v", len(p), p)
	}
	wantContains(t, "delivery refusal", p[0], dhBriefPath+": decision-gate hold", dhBoardID,
		"PR #77's body names it in a Brief: trailer", "from todo to implemented", "no decision issue is recorded")

	// A decision issue with no ruling is refused too, naming the issue.
	root = dvBase(t, "decision-issue: 41\n")
	if p := dhLayerOne(t, root); len(p) != 1 || !strings.Contains(p[0], "decision issue #41 has no recorded ruling") {
		t.Fatalf("an unruled delivery: %v", p)
	}
}

// With both keys recorded the delivery passes.
func TestDecisionGateDeliveryKeyedPasses(t *testing.T) {
	dhSeams(t, nil)
	root := dvBase(t, dhRuledLines())
	dvEvent(t, rlRepo, dvBody)
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("a delivery with decision-issue: and ruling: was refused: %v", p)
	}
}

// Not delivered: a body naming another brief, an Authors:-only body, no
// trailer at all, and no pull_request event (push, schedule, local) all leave
// the todo brief alone.
func TestDecisionGateDeliveryNotDeliveredClean(t *testing.T) {
	dhSeams(t, nil)
	for _, body := range []string{
		"Brief: sdlc/19\n",
		"Authors: " + dhBoardID + "\n",
		"No trailer here.\n",
		"",
	} {
		root := dvBase(t, "")
		dvEvent(t, rlRepo, body)
		if p := dhLayerOne(t, root); len(p) != 0 {
			t.Errorf("body %q: refused a brief it does not deliver: %v", body, p)
		}
	}
	root := dvBase(t, "")
	t.Setenv("GITHUB_EVENT_NAME", "push")
	t.Setenv("GITHUB_EVENT_PATH", filepath.Join(t.TempDir(), "absent.json"))
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Errorf("a push event was judged as a delivery: %v", p)
	}
}

// A brief already at implemented or later at the base (or withdrawn) is not
// judged again: its row has landed, so a PR naming it turns nothing red.
func TestDecisionGateDeliveryLandedRowLeftAlone(t *testing.T) {
	dhSeams(t, nil)
	for _, st := range []string{"implemented", "verified", "done", "withdrawn"} {
		root := dhFixture(t, map[string]string{
			dhReadmePath: dhReadme(dhRow("18", st)),
			dhBriefPath:  dhFM("gate: human", ""),
		})
		dvEvent(t, rlRepo, dvBody)
		if p := dhLayerOne(t, root); len(p) != 0 {
			t.Errorf("%s at the base: a delivery naming it was refused: %v", st, p)
		}
	}
}

// The brief's delivery: list declares this PR a partial delivery in this repo:
// reconcile does not move the row, so neither does the hold. The same claim
// for another repository does not lift it.
func TestDecisionGateDeliveryPartialClaimClean(t *testing.T) {
	dhSeams(t, nil)
	reg := "schema: graph-repos-v1\ncell: ex\nrepos:\n  ex: {cell: ex, repo: " + rlRepo + "}\n  other: {cell: ex, repo: example-org/other-repo}\n"
	for _, tc := range []struct {
		alias string
		want  int
	}{{"ex", 0}, {"other", 1}} {
		root := dhFixture(t, map[string]string{
			dhReadmePath:                    dhReadme(dhRow("18", "todo")),
			dhBriefPath:                     dhFM("gate: human", dvV1Lines+"delivery:\n  - {in: \""+tc.alias+"#77\", covers: partial, note: \"first slice\"}\n"),
			"docs/streams/graph-repos.yaml": reg,
		})
		dvEvent(t, rlRepo, dvBody)
		if p := dhLayerOne(t, root); len(p) != tc.want {
			t.Errorf("partial claim in %s#77: %d problems, want %d: %v", tc.alias, len(p), tc.want, p)
		}
	}
}

// A pull_request event whose payload cannot be read fails closed.
func TestDecisionGateDeliveryUnreadableEventFailsClosed(t *testing.T) {
	dhSeams(t, nil)
	root := dvBase(t, dhRuledLines())
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_EVENT_PATH", filepath.Join(t.TempDir(), "absent.json"))
	p := dhLayerOne(t, root)
	if len(p) != 1 || !strings.Contains(p[0], "refused (fail-closed)") {
		t.Fatalf("an unreadable pull_request payload: %v, want one fail-closed refusal", p)
	}
}

// The delivery reaches --lint itself.
func TestDecisionGateDeliveryLintWiring(t *testing.T) {
	dhSeams(t, nil)
	root := dvBase(t, "")
	dvEvent(t, rlRepo, dvBody)
	var code int
	stdout, stderr := captureBoth(t, func() { code = run(root, "lint", nil, nil, "") })
	if code == 0 {
		t.Fatalf("--lint passed an unkeyed delivery:\n%s\n%s", stdout, stderr)
	}
	wantContains(t, "--lint output", stdout+stderr, "decision-gate hold (lifecycle-v1 §4.5)", "PR #77")
}

// §4.4: a delivered post-cutover risk-gated brief with no design: record is
// flagged as at implemented; the same brief not delivered (still todo) is not,
// and a pre-cutover one is grandfathered.
func TestDecisionGateDeliveryDesignGate(t *testing.T) {
	load := func(t *testing.T, todo04 bool) (string, []*Stream) {
		t.Helper()
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/designgate")); err != nil {
			t.Fatal(err)
		}
		if todo04 {
			rm := filepath.Join(root, "docs/streams/dg/README.md")
			raw, err := os.ReadFile(rm)
			if err != nil {
				t.Fatal(err)
			}
			s := strings.Replace(string(raw), "| 0 | S | in-progress | — | — |\n| 05", "| 0 | S | todo | — | — |\n| 05", 1)
			if s == string(raw) {
				t.Fatal("fixture: could not set dg/04 to todo")
			}
			if err := os.WriteFile(rm, []byte(s), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		return root, streams
	}

	root, streams := load(t, false)
	if hasProblem(designGateProblems(root, streams), "dg/brief-05") {
		t.Fatal("dg/05 at todo was gated with no delivery")
	}
	dvEvent(t, rlRepo, "Brief: dg/05\n")
	p := designGateProblems(root, streams)
	if !hasProblem(p, "dg/brief-05", "no design: record", "PR #77's Brief: trailer delivers it") {
		t.Fatalf("a delivered post-cutover risk-gated brief with no design: was not flagged:\n%s", strings.Join(p, "\n"))
	}
	// The incremental form the reconcile hold uses agrees.
	e := &lintEnv{root: root, streams: streams}
	if got, _ := designGateIncremental(e, statusKeyedLintRule{}); !hasProblem(got, "dg/brief-05", "PR #77") {
		t.Fatalf("designGateIncremental missed the delivery:\n%s", strings.Join(got, "\n"))
	}

	root, streams = load(t, true)
	dvEvent(t, rlRepo, "Brief: dg/04\n")
	if p := designGateProblems(root, streams); hasProblem(p, "dg/brief-04") {
		t.Fatalf("a delivered pre-cutover brief was gated (the cutover must hold):\n%s", strings.Join(p, "\n"))
	}
}

// TestDecisionGateDeliveryEventCleared pins TestMain's clearing of the
// runner's event variables: a child test binary started with them set sees
// neither.
func TestDecisionGateDeliveryEventCleared(t *testing.T) {
	if os.Getenv("STATUSGEN_EVENT_PROBE") == "1" {
		for _, k := range []string{"GITHUB_EVENT_NAME", "GITHUB_EVENT_PATH"} {
			if v, ok := os.LookupEnv(k); ok {
				t.Fatalf("the test binary inherited %s=%q", k, v)
			}
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDecisionGateDeliveryEventCleared$", "-test.count=1")
	cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME=pull_request", "GITHUB_EVENT_PATH=/nonexistent/event.json", "STATUSGEN_EVENT_PROBE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("a pull_request event set by the caller reached the tests: %v\n%s", err, out)
	}
}
