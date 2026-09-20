package main

// rowderive_test.go — FAIL-FIRST coverage for per-row, command-cell-only derivation (#1309 item 4).
//
// THE DEFECT. deriveOnlineLane / deriveBlockedUntil substring-matched the WHOLE `## Verify`
// section with no row or cell awareness, so one row's Expect wording deferred the entire brief.
// Two live false positives: a brief whose Verify prose EXPLAINS that `kubectl` is refused was
// bucketed online-lane; a brief whose row-1 expectation says "shadow window" was deferred though
// its command is `git ls-files … | wc -l`. These pin: derive per ROW from the Command cell only;
// defer rows, not briefs (a brief with any runnable row stays DISPATCH and names the deferred
// rows for the verifier to record as unrun); a brief whose EVERY row is non-runnable is
// bucketed; an explicit frontmatter marker still wins.

import (
	"strings"
	"testing"
)

// The first live false positive: the Expect prose explains kubectl is refused; the command is
// an offline grep. Must be DISPATCH with no deferred rows.
func TestRowDerive_KubectlInExpectProseIsNotAnOnlineLane(t *testing.T) {
	brief := fixtureBrief("",
		"| 1 | `grep -n 'refuse' tools/guard.sh` | the guard refuses any `kubectl` invocation — kubectl never runs in this lane |")
	disp, reason := classifyFixture(t, brief)
	if disp != dispDispatch {
		t.Fatalf("brief whose Expect prose mentions kubectl classified %v (%q); want dispatch", disp, reason)
	}
	root := t.TempDir()
	writeFixtureStream(t, root, "example-stream", oneRowTable, map[string]string{"01": brief})
	if dr := payloadValue(selectOne(t, root, "example-stream/01"), "deferred_rows"); dr != "" {
		t.Fatalf("no row should be deferred, got deferred_rows=%q", dr)
	}
}

// Rows are deferred, not briefs: row 2's COMMAND is a live-cluster kubectl, rows 1 and 3 run
// offline. The brief stays DISPATCH, deferred_rows names row 2, and the rendered prompt tells
// the verifier to record it as explicitly unrun.
func TestRowDerive_DefersRowsNotBriefs(t *testing.T) {
	brief := fixtureBrief("",
		"| 1 | `go test ./pkg/...` | exit 0 |\n"+
			"| 2 | `kubectl get pods -n example-ns` | pod Running |\n"+
			"| 3 | `git ls-files 'reports/daily/*' \\| wc -l` | >= 14 — the shadow window's dated capture trees exist on main |")
	root := t.TempDir()
	writeFixtureStream(t, root, "example-stream", oneRowTable, map[string]string{"01": brief})
	it := selectOne(t, root, "example-stream/01")
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	if disp, reason := classifyItem(it, tier); disp != dispDispatch {
		t.Fatalf("mixed brief classified %v (%q); want dispatch (rows defer, briefs do not)", disp, reason)
	}
	if got := payloadValue(it, "deferred_rows"); got != "2: online lane (cluster)" {
		t.Fatalf("deferred_rows = %q, want \"2: online lane (cluster)\"", got)
	}
	prompt := renderDispatchPrompt(it, tier)
	if !strings.Contains(prompt, "Rows to record as explicitly UNRUN (not runnable offline; run every other row): 2: online lane (cluster)") {
		t.Fatalf("dispatch prompt does not name the deferred row:\n%s", prompt)
	}
}

// A brief whose EVERY row is non-runnable is still bucketed as a whole — online lane when a
// command names one, deferred when a command names a window (blocked-until wins when both).
func TestRowDerive_AllRowsNonRunnableBucketsTheBrief(t *testing.T) {
	online := fixtureBrief("",
		"| 1 | `kubectl get pods -n example-ns` | Running |\n"+
			"| 2 | `kubectl get cronjob -n example-ns` | scheduled |")
	if disp, _ := classifyFixture(t, online); disp != dispAwaitingOnlineLane {
		t.Fatalf("all-online brief classified %v; want awaiting-online-lane", disp)
	}
	mixed := fixtureBrief("",
		"| 1 | `kubectl get pods -n example-ns` | Running |\n"+
			"| 2 | wait for the observation window to elapse, then compare | window elapsed |")
	if disp, _ := classifyFixture(t, mixed); disp != dispDeferred {
		t.Fatalf("all-non-runnable (online + window) brief classified %v; want deferred (blocked-until precedes lane)", disp)
	}
}

// An explicit frontmatter marker wins over derivation in BOTH directions: verify-lane set on a
// brief whose commands are all offline buckets it; blocked-until likewise.
func TestRowDerive_ExplicitMarkerWinsOverDerivation(t *testing.T) {
	brief := fixtureBrief("verify-lane: cluster\n", "| 1 | `go test ./...` | exit 0 |")
	if disp, reason := classifyFixture(t, brief); disp != dispAwaitingOnlineLane || reason != "cluster" {
		t.Fatalf("explicit verify-lane on offline rows classified %v (%q); want awaiting-online-lane/cluster", disp, reason)
	}
	brief = fixtureBrief("blocked-until: 2026-12-01 (authored)\n", "| 1 | `go test ./...` | exit 0 |")
	if disp, reason := classifyFixture(t, brief); disp != dispDeferred || !strings.Contains(reason, "authored") {
		t.Fatalf("explicit blocked-until on offline rows classified %v (%q); want deferred with the authored reason", disp, reason)
	}
}

// Derivation reads the Command cell by header name, whatever the column order.
func TestRowDerive_ReadsCommandColumnByName(t *testing.T) {
	brief := "---\nbrief: x\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n---\n\n# Brief\n\n" +
		realVerifyHeading + "\n\n| # | Class | Expect | Command |\n|---|-------|--------|---------|\n" +
		"| 1 | check | kubectl is refused here | `grep -c refuse guard.sh` |\n\n## Evidence\n<!-- x -->\n"
	if disp, reason := classifyFixture(t, brief); disp != dispDispatch {
		t.Fatalf("reordered-column brief classified %v (%q); want dispatch", disp, reason)
	}
}
