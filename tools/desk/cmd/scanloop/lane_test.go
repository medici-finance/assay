package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func scanItem(repo, lane string) loopengine.Item {
	return loopengine.Item{
		ID:      repo + "#11",
		Payload: map[string]string{"repo": repo, "number": "11", "lane": lane},
	}
}

func laneReq(t *testing.T, it loopengine.Item, open *OpenScanPR) LaneRequest {
	t.Helper()
	base := t.TempDir()
	return LaneRequest{
		Item:     it,
		Tier:     loopengine.TierLocal,
		Root:     filepath.Join(base, "target"),
		Worktree: filepath.Join(base, "scan-wt"),
		Branch:   "chore/intake-scan-2026-08-24-1200",
		Open:     open,
		Policy:   CoalescePolicy{},
		Now:      coalesceNow,
		DryRun:   true,
	}
}

// TestSelectLane_IsTheOneSeam — every dispatch resolves through this function, so a cutover is a
// change here and nowhere else. Judgment always wins over the payload's lane hint.
func TestSelectLane_IsTheOneSeam(t *testing.T) {
	if got := SelectLane(scanItem("medici-finance/assay", string(LaneScanCarrierPR)), loopengine.TierLocal, nil, nil).Name(); got != LaneScanCarrierPR {
		t.Fatalf("lane = %s, want %s", got, LaneScanCarrierPR)
	}
	if got := SelectLane(scanItem("medici-finance/assay", string(LaneIssueFiling)), loopengine.TierLocal, nil, nil).Name(); got != LaneIssueFiling {
		t.Fatalf("lane = %s, want %s", got, LaneIssueFiling)
	}
	// A judgment item routes to the emitting lane REGARDLESS of the payload hint: the tier is the
	// authority on whether a decision has been made, not the queue row.
	if got := SelectLane(scanItem("medici-finance/assay", string(LaneScanCarrierPR)), loopengine.TierSession, nil, nil).Name(); got != LaneRouting {
		t.Fatalf("lane = %s, want %s for a judgment item", got, LaneRouting)
	}
}

// TestRoutingLane_ExecutesNothing — the judgment half is EMITTED, never computed. A routing lane
// that ran a command would be this loop deciding an exit on its own.
func TestRoutingLane_ExecutesNothing(t *testing.T) {
	var ran int
	x := Exec(func(string, string, ...string) (string, error) { ran++; return "", nil })
	lane := SelectLane(scanItem("medici-finance/assay", ""), loopengine.TierSession, x, nil)
	out, err := lane.Execute(laneReq(t, scanItem("medici-finance/assay", ""), nil))
	if err != nil {
		t.Fatal(err)
	}
	if ran != 0 {
		t.Fatalf("the routing lane ran %d command(s) — judgment is emitted, never executed", ran)
	}
	if out.Exit != ExitUnrouted {
		t.Fatalf("exit = %s, want %s — emitting is not an exit", out.Exit, ExitUnrouted)
	}
	joined := strings.Join(out.Steps, "\n")
	for _, want := range exitNames() {
		if !strings.Contains(joined, want) {
			t.Fatalf("the emitted routing instruction does not name the %q exit:\n%s", want, joined)
		}
	}
}

// TestScanCarrierLane_FreshPRStepSequence pins the three non-negotiables: sync fresh, isolate,
// commit-and-carry — plus the derivation of the title and body from the branch's own diff and the
// drift gate over them.
func TestScanCarrierLane_FreshPRStepSequence(t *testing.T) {
	it := scanItem("medici-finance/assay", string(LaneScanCarrierPR))
	lane := scanCarrierPRLane{}
	out, err := lane.Execute(laneReq(t, it, nil))
	if err != nil {
		t.Fatalf("lane refused: %v", err)
	}
	joined := strings.Join(out.Steps, "\n")
	for _, want := range []string{
		"git fetch origin",
		"git worktree add",
		"statusgen --root . --scan-issues",
		"git add " + deskkit.ScanDir,
		"git commit",
		"deskscanbody emit --base origin/main --format title",
		"deskscanbody emit --base origin/main --format body",
		"deskscanbody check",
		"deskpr create",
		"git worktree remove",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the fresh-PR sequence is missing %q:\n%s", want, joined)
		}
	}
	if out.Exit != ExitPlaceholder {
		t.Fatalf("exit = %s, want %s", out.Exit, ExitPlaceholder)
	}
}

// TestScanCarrierLane_CoalescedPushRegeneratesTheBody — the recurring finding this closes is a body
// still stating the FIRST push's counts against a much larger diff. Every coalesced push must
// re-derive both halves.
func TestScanCarrierLane_CoalescedPushRegeneratesTheBody(t *testing.T) {
	it := scanItem("medici-finance/assay", string(LaneScanCarrierPR))
	out, err := scanCarrierPRLane{}.Execute(laneReq(t, it, openPR(3*time.Minute)))
	if err != nil {
		t.Fatalf("lane refused: %v", err)
	}
	joined := strings.Join(out.Steps, "\n")
	if out.Decision != CoalesceInto {
		t.Fatalf("decision = %s, want COALESCE", out.Decision)
	}
	if !strings.Contains(joined, "deskpr update") {
		t.Fatalf("a coalesced push did not use the follow-up push verb:\n%s", joined)
	}
	for _, want := range []string{"--format title", "--format body", "gh pr edit 42"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("a coalesced push did not regenerate %q:\n%s", want, joined)
		}
	}
	// And it must merge the fetched remote head in before scanning: never scan a base behind it.
	if !strings.Contains(joined, "git merge --no-edit") {
		t.Fatalf("a coalesced scan did not merge the remote head into the branch first:\n%s", joined)
	}
}

// TestScanCarrierLane_RefusesANestedWorktree — a worktree inside the checkout being scanned is a
// shared-checkout scan under another name, and that is exactly what once left a pile of
// uncommitted, superseded placeholder files behind.
func TestScanCarrierLane_RefusesANestedWorktree(t *testing.T) {
	root := t.TempDir()
	req := LaneRequest{
		Item:     scanItem("medici-finance/assay", string(LaneScanCarrierPR)),
		Root:     root,
		Worktree: filepath.Join(root, "inner", "scan-wt"),
		Now:      coalesceNow,
		DryRun:   true,
	}
	_, err := scanCarrierPRLane{}.Execute(req)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v (exit %d), want a refusal", err, deskkit.ExitCodeOf(err))
	}
}

// TestScanCarrierLane_RefusesARelativeOrEmptyWorktree — a failed worktree creation must not leave
// the following steps running in whatever directory the process happens to be in.
func TestScanCarrierLane_RefusesARelativeOrEmptyWorktree(t *testing.T) {
	for _, wt := range []string{"", "scan-wt", "./scan-wt"} {
		req := LaneRequest{
			Item:     scanItem("medici-finance/assay", string(LaneScanCarrierPR)),
			Root:     t.TempDir(),
			Worktree: wt,
			Now:      coalesceNow,
			DryRun:   true,
		}
		if _, err := (scanCarrierPRLane{}).Execute(req); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("worktree %q: err = %v, want a refusal", wt, err)
		}
	}
}

// TestScanCarrierLane_RefusesARepoOutsideTheWriteBoundary — the intake READ scope and the write
// boundary are deliberately different sets, and this lane writes.
func TestScanCarrierLane_RefusesARepoOutsideTheWriteBoundary(t *testing.T) {
	it := scanItem("example-org/site", string(LaneScanCarrierPR)) // in the scan scope, not in the allowed set
	if deskkit.IsAllowedRepo("example-org/site") {
		t.Skip("fixture roster changed: this repo is now a write target")
	}
	_, err := scanCarrierPRLane{}.Execute(laneReq(t, it, nil))
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal for a repo outside the write boundary", err)
	}
}

// TestIssueFilingLane_StampsTheRaisingLoopAndTheExitLabel — omitting the raised-by stamp is not
// neutral: the issue lands with unknown provenance, which is the absence of an answer and never
// "a human raised it".
func TestIssueFilingLane_StampsTheRaisingLoopAndTheExitLabel(t *testing.T) {
	it := scanItem("medici-finance/assay", string(LaneIssueFiling))
	it.Payload["exit"] = string(ExitNeedsDecision)
	it.Payload["title"] = "a decision is owed"
	it.Payload["body-file"] = "/tmp/body.md"

	out, err := issueFilingLane{}.Execute(laneReq(t, it, nil))
	if err != nil {
		t.Fatalf("lane refused: %v", err)
	}
	joined := strings.Join(out.Steps, "\n")
	for _, want := range []string{"deskfile new", "--raised-by " + raisedByRole, "--label needs-decision"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the filing does not carry %q:\n%s", want, joined)
		}
	}
	if out.Exit != ExitNeedsDecision {
		t.Fatalf("exit = %s", out.Exit)
	}
}

// TestIssueFilingLane_RefusesAnExitItDoesNotFile — the lane set and the exit set are held together
// rather than left to agree by convention.
func TestIssueFilingLane_RefusesAnExitItDoesNotFile(t *testing.T) {
	it := scanItem("medici-finance/assay", string(LaneIssueFiling))
	it.Payload["exit"] = string(ExitPlaceholder)
	it.Payload["title"] = "t"
	it.Payload["body-file"] = "/tmp/b.md"
	if _, err := (issueFilingLane{}).Execute(laneReq(t, it, nil)); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

// TestLaneSeam_TheCutoverCommentIsGenericAndCarriesNoPrivateReference — this source is published.
// The pending-cutover note must describe the SHAPE of what is coming, never a document path, a
// repo, or a ruling identifier.
func TestLaneSeam_TheCutoverCommentIsGenericAndCarriesNoPrivateReference(t *testing.T) {
	src := readSource(t, "lane.go")
	if !strings.Contains(src, "transcription lane pending a recorded operator ruling; swap here") {
		t.Fatal("lane.go no longer carries the generic pending-cutover note over the seam")
	}
	for _, forbidden := range []string{"docs/streams/", "docs/research/", "SKILL.md", "rulings.md"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("lane.go names %q — this file is published source and carries no internal document reference", forbidden)
		}
	}
}
