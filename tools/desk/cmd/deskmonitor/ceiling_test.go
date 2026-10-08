package main

// ceiling_test.go — the inbound read CEILING (INBOUND_MONITOR_LIMIT).
//
// The defect class these tests close: a poller whose read window is the same number as its
// truncation threshold. It asked the forge for LIMIT issues and called LIMIT issues TRUNCATED, so
// it could not tell a complete set of exactly LIMIT from a set clipped at LIMIT, and a repo whose
// open set grew past the default window (500) read as could-not-check on every cycle from then on,
// with nothing that could clear it.
//
// The fixed shape, in both inbound pollers (the verb and its bash oracle):
//   - the read walks pages until the open set is exhausted, up to a ceiling (default
//     inboundDefaultLimit) that sits BELOW the verb's forge-client page guard, not at a fixed
//     500-issue window;
//   - the poller asks for ONE row past the ceiling, and a read is TRUNCATED only when that extra
//     row comes back — "more than LIMIT", never "== LIMIT".
//
// TestMonitorCeilingParity drives BOTH pollers through the same recorded answers and pins each
// one's output exactly — parity alone would pass with both still blind. Its default-ceiling cases
// run the verb through the production GitHub forge client's page walk, so a default ceiling the
// forge client cannot read whole (one at or past its page guard) fails there.
//
// TestInboundPageGuardDivergence pins the one place the two pollers may disagree: a listing that
// fills the verb's forge-client page guard. The verb fails closed there; the oracle does not see
// the guard.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seq returns issues lo..hi inclusive.
func seq(lo, hi int) []fixtureIssue {
	var nums []int
	for n := lo; n <= hi; n++ {
		nums = append(nums, n)
	}
	return issues(nums...)
}

// TestMonitorPastOldWindow — a repo with more open issues than the old 500 window arms on its
// seed and reports a genuinely new issue on the next cycle, with every knob at its default.
func TestMonitorPastOldWindow(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	t.Setenv("INBOUND_MONITOR_STATE_DIR", stateDir)
	t.Setenv("INBOUND_MONITOR_LIMIT", "")
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
	plantRoster(t, []string{repoT})
	stubSessionIdentity(t, "worker", "x-minted-token", nil)
	fake := newForgeFake(t)

	out, code := pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 705)}}}, repoT)
	if code != 0 || out != "MONITOR-ARMED: 705\n" {
		t.Fatalf("a 705-issue seed must arm in one pass, got exit %d:\n%s", code, out)
	}
	out, code = pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 706)}}}, repoT)
	if code != 0 || out != "INBOUND: example-org/tracker#706 2026-09-01T00:00:00Z\n" {
		t.Fatalf("the next cycle must report exactly the one new issue, got exit %d:\n%s", code, out)
	}
}

// TestMonitorExactLimitWhole — a set of EXACTLY the ceiling is the whole set (it seeds and
// diffs); one past it is TRUNCATED (retain + go loud), on an established repo and on a seed.
func TestMonitorExactLimitWhole(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	t.Setenv("INBOUND_MONITOR_STATE_DIR", stateDir)
	t.Setenv("INBOUND_MONITOR_LIMIT", "3")
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
	plantRoster(t, []string{repoT, repoA})
	stubSessionIdentity(t, "worker", "x-minted-token", nil)
	fake := newForgeFake(t)

	out, code := pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 3)}}}, repoT)
	if code != 0 || out != "MONITOR-ARMED: 3\n" {
		t.Fatalf("a seed of exactly the ceiling is the whole set and must arm, got exit %d:\n%s", code, out)
	}
	sf := filepath.Join(stateDir, "example-org__tracker.state")
	before, _ := os.ReadFile(sf)

	out, code = pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 4)}}}, repoT)
	want := "MONITOR-DEGRADED: example-org/tracker returned more than --limit 3 — results TRUNCATED, treating as could-not-check; keeping its previous 3 issue(s)\n"
	if code != 2 || out != want {
		t.Fatalf("one past the ceiling must be TRUNCATED, got exit %d:\n%s", code, out)
	}
	if after, _ := os.ReadFile(sf); string(after) != string(before) {
		t.Fatalf("a truncated read advanced the baseline:\nbefore %q\nafter  %q", before, after)
	}

	out, code = pollInbound(t, fake, fixtureCycle{Reads: map[string]fixtureRead{repoA: {Issues: seq(1, 4)}}}, repoA)
	wantSeed := "MONITOR-DEGRADED: example-other/agents seed returned more than --limit 3 — results TRUNCATED, no baseline established; raise INBOUND_MONITOR_LIMIT and retry\n"
	if code != 2 || out != wantSeed {
		t.Fatalf("a seed past the ceiling must establish no baseline, got exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "example-other__agents.state")); !os.IsNotExist(err) {
		t.Fatalf("a truncated seed wrote a baseline (stat err %v)", err)
	}
}

// TestMonitorCeilingParity — the class guard over BOTH inbound pollers. The same recorded answers
// go to the verb and to inbound-monitor.sh; each one's output is pinned exactly, and the two must
// agree byte for byte. A poller that still treats a fixed window as its truncation threshold —
// either one — fails here.
func TestMonitorCeilingParity(t *testing.T) {
	script := requireOracle(t, "inbound-monitor.sh")
	def := inboundDefaultLimit
	cases := []struct {
		name string
		fx   fixture
		want []string // expected stdout per cycle, for BOTH pollers
		code []int
	}{
		{
			// The default ceiling's own boundary, every knob at its default. The verb reads through
			// the production forge client's page walk, so this case fails if the default's sentinel
			// row (LIMIT+1) does not fit inside that client's page guard.
			name: "the default ceiling: exactly LIMIT is whole, one past is truncated",
			fx: fixture{Kind: "inbound", Env: map[string]string{"INBOUND_MONITOR_LIMIT": ""}, Repos: []string{repoT},
				Cycles: []fixtureCycle{
					{Note: "seed exactly the default ceiling", Reads: map[string]fixtureRead{repoT: {Issues: seq(1, def)}}},
					{Note: "one past the default ceiling", Reads: map[string]fixtureRead{repoT: {Issues: seq(1, def+1)}}},
				}},
			want: []string{fmt.Sprintf("MONITOR-ARMED: %d\n", def),
				fmt.Sprintf("MONITOR-DEGRADED: example-org/tracker returned more than --limit %d — results TRUNCATED, treating as could-not-check; keeping its previous %d issue(s)\n", def, def)},
			code: []int{0, 2},
		},
		{
			name: "the default ceiling: a seed one past it establishes no baseline",
			fx: fixture{Kind: "inbound", Env: map[string]string{"INBOUND_MONITOR_LIMIT": ""}, Repos: []string{repoT},
				Cycles: []fixtureCycle{
					{Note: "seed one past the default ceiling", Reads: map[string]fixtureRead{repoT: {Issues: seq(1, def+1)}}},
				}},
			want: []string{fmt.Sprintf("MONITOR-DEGRADED: example-org/tracker seed returned more than --limit %d — results TRUNCATED, no baseline established; raise INBOUND_MONITOR_LIMIT and retry\n", def)},
			code: []int{2},
		},
		{
			name: "past the old 500 window, default knobs",
			fx: fixture{Kind: "inbound", Env: map[string]string{"INBOUND_MONITOR_LIMIT": ""}, Repos: []string{repoT},
				Cycles: []fixtureCycle{
					{Note: "seed 705", Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 705)}}},
					{Note: "one new", Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 706)}}},
				}},
			want: []string{"MONITOR-ARMED: 705\n", "INBOUND: example-org/tracker#706 2026-09-01T00:00:00Z\n"},
			code: []int{0, 0},
		},
		{
			name: "exactly the ceiling is whole, one past is truncated",
			fx: fixture{Kind: "inbound", Env: map[string]string{"INBOUND_MONITOR_LIMIT": "3"}, Repos: []string{repoT},
				Cycles: []fixtureCycle{
					{Note: "seed exactly 3", Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 3)}}},
					{Note: "4 is past the ceiling", Reads: map[string]fixtureRead{repoT: {Issues: seq(1, 4)}}},
				}},
			want: []string{"MONITOR-ARMED: 3\n",
				"MONITOR-DEGRADED: example-org/tracker returned more than --limit 3 — results TRUNCATED, treating as could-not-check; keeping its previous 3 issue(s)\n"},
			code: []int{0, 2},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oracle, verb := runParity(t, script, tc.fx)
			assertParity(t, tc.fx, oracle, verb)
			for i := range tc.fx.Cycles {
				for _, side := range []struct {
					who string
					r   cycleResult
				}{{"inbound-monitor.sh", oracle[i]}, {"deskmonitor inbound", verb[i]}} {
					if side.r.stdout != tc.want[i] || side.r.code != tc.code[i] {
						t.Errorf("cycle %d (%s): %s gave exit %d:\n%s\nwant exit %d:\n%s",
							i+1, tc.fx.Cycles[i].Note, side.who, side.r.code, strings.TrimRight(side.r.stdout, "\n"),
							tc.code[i], strings.TrimRight(tc.want[i], "\n"))
					}
				}
			}
		})
	}
}

// TestInboundPageGuardDivergence — the stated divergence between the two inbound pollers. The verb
// reads through the forge client, whose open-issue walk stops at a local page guard (on GitHub,
// 100 pages of 100 REST rows, and those rows include open pull requests until the client drops
// them). A listing that fills the guard is refused as could-not-check, whatever the monitor
// ceiling, so the verb fails closed there: no baseline on a seed, the previous baseline kept byte
// for byte on an established repo, exit 2. `gh issue list` reads issues only and never meets the
// guard, so the oracle reads the same repo cleanly. Both directions are pinned so the divergence
// can only ever be the verb being MORE conservative.
func TestInboundPageGuardDivergence(t *testing.T) {
	script := requireOracle(t, "inbound-monitor.sh")
	const guardRows = 100 * 100 // the GitHub forge client's page guard, in raw REST rows
	fx := fixture{Kind: "inbound", Env: map[string]string{"INBOUND_MONITOR_LIMIT": ""}, Repos: []string{repoT, repoA},
		Cycles: []fixtureCycle{
			{Note: "tracker seeds cleanly; agents' 100 issues sit behind open PRs that fill the guard", Reads: map[string]fixtureRead{
				repoT: {Issues: seq(1, 100)},
				repoA: {Issues: seq(1, 100), PRRows: guardRows - 100},
			}},
			{Note: "tracker gains one issue while open PRs fill the guard", Reads: map[string]fixtureRead{
				repoT: {Issues: seq(1, 101), PRRows: guardRows - 101},
				repoA: {Issues: seq(1, 100), PRRows: guardRows - 100},
			}},
		}}
	oracle, verb := runParity(t, script, fx)

	// The oracle never sees the PR rows: both repos read cleanly.
	if oracle[0].code != 0 || oracle[0].stdout != "MONITOR-ARMED: 200\n" {
		t.Fatalf("cycle 1: the oracle should arm both repos, got exit %d:\n%s", oracle[0].code, oracle[0].stdout)
	}
	if oracle[1].code != 0 || oracle[1].stdout != "INBOUND: example-org/tracker#101 2026-09-01T00:00:00Z\n" {
		t.Fatalf("cycle 2: the oracle should report the one new issue, got exit %d:\n%s", oracle[1].code, oracle[1].stdout)
	}

	// The verb refuses the guard-filling listing: a failed read, fail-closed.
	guardDiag := "could-not-check: %s still reports more open issues after 100 pages of 100"
	seedLine := lineNaming(t, verb[0].stdout, "example-other/agents")
	if verb[0].code != 2 || !strings.Contains(seedLine, "seed read FAILED (forge: "+fmt.Sprintf(guardDiag, repoA)) ||
		!strings.HasSuffix(seedLine, "— no baseline established, will retry next cycle") {
		t.Fatalf("cycle 1: the verb must refuse the guard-filling seed, got exit %d:\n%s", verb[0].code, verb[0].stdout)
	}
	if _, ok := verb[0].state["example-other__agents.state"]; ok {
		t.Fatalf("cycle 1: a guard-filling seed wrote a baseline: %v", verb[0].state)
	}
	if got := lineNaming(t, verb[0].stdout, "MONITOR-ARMED"); got != "" {
		t.Fatalf("cycle 1: a degraded seed cycle must not print the armed line, got %q", got)
	}
	trackerLine := lineNaming(t, verb[1].stdout, "example-org/tracker")
	if verb[1].code != 2 || !strings.Contains(trackerLine, "read FAILED (forge: "+fmt.Sprintf(guardDiag, repoT)) ||
		!strings.HasSuffix(trackerLine, "— keeping its previous 100 issue(s)") {
		t.Fatalf("cycle 2: the verb must refuse the guard-filling read, got exit %d:\n%s", verb[1].code, verb[1].stdout)
	}
	if strings.Contains(verb[1].stdout, "INBOUND:") {
		t.Fatalf("cycle 2: a refused read fired INBOUND:\n%s", verb[1].stdout)
	}
	if verb[1].state["example-org__tracker.state"] != verb[0].state["example-org__tracker.state"] {
		t.Fatalf("cycle 2: a refused read moved the tracker baseline")
	}
}

// lineNaming returns the one stdout line containing needle ("" if none; fatal if several).
func lineNaming(t *testing.T, stdout, needle string) string {
	t.Helper()
	var hit []string
	for _, l := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if strings.Contains(l, needle) {
			hit = append(hit, l)
		}
	}
	switch len(hit) {
	case 0:
		return ""
	case 1:
		return hit[0]
	}
	t.Fatalf("%d lines name %q:\n%s", len(hit), needle, stdout)
	return ""
}
