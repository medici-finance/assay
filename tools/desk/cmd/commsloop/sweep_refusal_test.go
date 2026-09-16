package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
)

// sweep_refusal_test.go — the finer contracts of the sweep's refusal
// counting (#1165), written through the SHARED writer
// (commsqueue.AppendRefusal) so the reader is proven against the exact bytes
// the gateway produces, not a hand-rendered imitation.

func appendRefusals(t *testing.T, root string, n int, cell string, kind commsqueue.RefusalKind, from comms.SenderID, to comms.Lane, at time.Time) {
	t.Helper()
	for i := 0; i < n; i++ {
		rec := commsqueue.RefusalRecord{
			Time: at.Add(time.Duration(i) * time.Second), Kind: commsqueue.JournalKindRefused,
			ID: fmt.Sprintf("%s-%s-%d", from.Role, kind, i), Cell: cell, Refusal: kind,
			From: from, To: to, Verb: "handoff", RawDigest: "00",
		}
		if err := commsqueue.AppendRefusal(root, rec); err != nil {
			t.Fatalf("AppendRefusal: %v", err)
		}
	}
}

func findingIDs(r SweepReport) []string {
	out := make([]string, 0, len(r.Findings))
	for _, f := range r.Findings {
		out = append(out, string(f.Kind)+" "+f.ID)
	}
	return out
}

// TestSweepRefusalLaneAxis: two senders each under the per-sender threshold
// but converging on ONE lane pair over it — the LANE axis breaches while
// the sender axis stays quiet. Threshold is set explicitly through
// SweepDeps.
func TestSweepRefusalLaneAxis(t *testing.T) {
	root := t.TempDir()
	to := comms.Lane{Cell: "cell-a", Role: "worker-desk"}
	appendRefusals(t, root, 3, "cell-a", commsqueue.RefusalLaneDenied, comms.SenderID{Cell: "cell-a", Role: "the-desk"}, to, sweepTestNow)
	appendRefusals(t, root, 3, "cell-a", commsqueue.RefusalLaneDenied, comms.SenderID{Cell: "cell-a", Role: "intake-desk"}, to, sweepTestNow)

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{RefusalThreshold: 5})

	if report.State != SweepCheckedFailed {
		t.Fatalf("state = %s, want checked-failed: %v / %v", report.State, report.Findings, report.CouldNotCheckReasons)
	}
	if report.Refused != 6 {
		t.Fatalf("Refused = %d, want 6", report.Refused)
	}
	ids := findingIDs(report)
	if len(ids) != 1 || ids[0] != string(FindingRefusalThreshold)+" lane:cell-a/worker-desk" {
		t.Fatalf("want exactly ONE finding, on the destination lane (neither sender reaches the threshold alone), got %v", ids)
	}
	if d := report.Findings[0].Detail; !strings.Contains(d, "6 gateway refusal(s) for lane cell-a/worker-desk") || !strings.Contains(d, "lane-denied=6") {
		t.Fatalf("lane finding detail must carry the count, the lane and the breakdown, got %q", d)
	}
}

// TestSweepRefusalThresholdDisabled: a negative threshold disables the
// check — a flood is still COUNTED (Refused) but reports no finding.
func TestSweepRefusalThresholdDisabled(t *testing.T) {
	root := t.TempDir()
	appendRefusals(t, root, 50, "cell-a", commsqueue.RefusalReplay, comms.SenderID{Cell: "cell-a", Role: "the-desk"}, comms.Lane{Cell: "cell-a", Role: "worker-desk"}, sweepTestNow)

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{RefusalThreshold: -1})

	if report.State != SweepCheckedClean || len(report.Findings) != 0 {
		t.Fatalf("disabled threshold must report no finding, got %s %v", report.State, report.Findings)
	}
	if report.Refused != 50 {
		t.Fatalf("refusals are still counted when the threshold is disabled, Refused=%d", report.Refused)
	}
}

// TestSweepRefusalUnrecognisedKindIsCorrupt: a refused line naming a kind
// outside the closed vocabulary is could-not-check, never silently counted
// under some other label.
func TestSweepRefusalUnrecognisedKindIsCorrupt(t *testing.T) {
	root := t.TempDir()
	writeJournalLines(t, root,
		`{"time":"2026-09-12T12:00:00Z","kind":"refused","id":"weird-1","cell":"cell-a","refusal":"made-up-kind","from":{"cell":"cell-a","role":"the-desk"},"to":{"cell":"cell-a","role":"worker-desk"},"verb":"handoff"}`)

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{})

	if report.State != SweepCouldNotCheck {
		t.Fatalf("state = %s, want could-not-check", report.State)
	}
	if len(report.CouldNotCheckReasons) != 1 || !strings.Contains(report.CouldNotCheckReasons[0], "made-up-kind") {
		t.Fatalf("reason must name the unrecognised kind, got %v", report.CouldNotCheckReasons)
	}
	if report.Refused != 0 {
		t.Fatalf("a corrupt refusal line is not counted, Refused=%d", report.Refused)
	}
}

// TestSweepRefusalUnattributedScopedByGatewayCell: refusals whose bytes
// never presented a from/to (parse failures, unauthenticated peers) carry
// only the GATEWAY cell — they are in scope for THAT cell's sweep and count
// under the "(unattributed)" sender, so a flood of garbage never falls out
// of every cell's scope.
func TestSweepRefusalUnattributedScopedByGatewayCell(t *testing.T) {
	root := t.TempDir()
	appendRefusals(t, root, 4, "cell-a", commsqueue.RefusalEnvelopeParse, comms.SenderID{}, comms.Lane{}, sweepTestNow)

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{RefusalThreshold: 4})
	if report.State != SweepCheckedFailed {
		t.Fatalf("cell-a's sweep must see its own gateway's unattributed refusals, got %s %v %v", report.State, report.Findings, report.CouldNotCheckReasons)
	}
	if ids := findingIDs(report); len(ids) != 2 || !strings.Contains(ids[0], "sender:(unattributed)") || !strings.Contains(ids[1], "lane:(unattributed)") {
		t.Fatalf("want the unattributed sender + lane findings, got %v", ids)
	}

	other := Sweep(root, "cell-b", time.Time{}, SweepDeps{RefusalThreshold: 4})
	if other.State != SweepCheckedClean || other.Refused != 0 {
		t.Fatalf("another cell's sweep must not count cell-a's gateway refusals, got %s refused=%d", other.State, other.Refused)
	}
}

// TestSweepRefusalSinceFloor: refusals older than --since are outside the
// window and neither counted nor breached.
func TestSweepRefusalSinceFloor(t *testing.T) {
	root := t.TempDir()
	from := comms.SenderID{Cell: "cell-a", Role: "the-desk"}
	to := comms.Lane{Cell: "cell-a", Role: "worker-desk"}
	appendRefusals(t, root, 6, "cell-a", commsqueue.RefusalKillSwitch, from, to, sweepTestNow.Add(-48*time.Hour))
	appendRefusals(t, root, 2, "cell-a", commsqueue.RefusalKillSwitch, from, to, sweepTestNow)

	report := Sweep(root, "cell-a", sweepTestNow.Add(-time.Hour), SweepDeps{RefusalThreshold: 5})
	if report.State != SweepCheckedClean || report.Refused != 2 {
		t.Fatalf("only the two in-window refusals count, got %s refused=%d findings=%v", report.State, report.Refused, report.Findings)
	}

	all := Sweep(root, "cell-a", time.Time{}, SweepDeps{RefusalThreshold: 5})
	if all.State != SweepCheckedFailed || all.Refused != 8 {
		t.Fatalf("no floor sweeps the whole history, got %s refused=%d", all.State, all.Refused)
	}
}

// TestSweepRefusalNotLaneRechecked: a refused line whose presented lane is
// illegal is NOT re-checked as a lane violation — it never landed, there is
// no landing to re-verify. Below threshold it is clean; the refusal
// vocabulary is counted, the ACL is not consulted for it.
func TestSweepRefusalNotLaneRechecked(t *testing.T) {
	root := t.TempDir()
	appendRefusals(t, root, 1, "cell-a", commsqueue.RefusalLaneDenied,
		comms.SenderID{Cell: "cell-a", Role: "the-desk"}, comms.Lane{Cell: "cell-a", Role: "janitor"}, sweepTestNow)

	report := Sweep(root, "cell-a", time.Time{}, SweepDeps{})
	if report.State != SweepCheckedClean {
		t.Fatalf("one refused out-of-lane attempt is a counted refusal, not a lane-violation landing: %s %v", report.State, report.Findings)
	}
	if report.Refused != 1 || report.Checked != 1 {
		t.Fatalf("Refused=%d Checked=%d, want 1/1", report.Refused, report.Checked)
	}
}

// TestSweepCLIRefusalThresholdFlag drives `commsloop sweep` end to end with
// the flag: the default breaches at ten, `--refusal-threshold 100` does
// not, and the report line carries the refused count either way.
func TestSweepCLIRefusalThresholdFlag(t *testing.T) {
	root := t.TempDir()
	appendRefusals(t, root, DefaultRefusalThreshold, "cell-a", commsqueue.RefusalBudgetExhausted,
		comms.SenderID{Cell: "cell-a", Role: "the-desk"}, comms.Lane{Cell: "cell-a", Role: "worker-desk"}, time.Now().UTC())
	getenv := func(k string) string {
		if k == EnvQueueDir {
			return root
		}
		return ""
	}

	var out strings.Builder
	err := cmdSweep([]string{"--cell", "cell-a"}, getenv, &out)
	if err == nil || !strings.Contains(err.Error(), "finding(s)") {
		t.Fatalf("default threshold must report checked-failed, got err=%v out=%s", err, out.String())
	}
	if !strings.Contains(out.String(), fmt.Sprintf("refused=%d", DefaultRefusalThreshold)) || !strings.Contains(out.String(), "FINDING refusal-threshold") {
		t.Fatalf("report must carry the refused count and the finding, got:\n%s", out.String())
	}

	out.Reset()
	if err := cmdSweep([]string{"--cell", "cell-a", "--refusal-threshold", "100"}, getenv, &out); err != nil {
		t.Fatalf("raised threshold must be clean, got %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), fmt.Sprintf("refused=%d", DefaultRefusalThreshold)) {
		t.Fatalf("refused count is reported even when clean, got:\n%s", out.String())
	}
}
