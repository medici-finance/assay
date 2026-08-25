package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// lane.go — the DISPATCH-LANE SEAM.
//
// Every dispatch this drain makes goes through SelectLane below, and nothing else in this package
// knows which lane ran. That isolation is the point: the loop, the queue, the trust gate, the
// coalesce window and the exit ledger are all lane-agnostic, so replacing a lane is a change to
// this one file and to nothing else.
//
// NOTE FOR THE FUTURE MAINTAINER:
//
//	transcription lane pending a recorded operator ruling; swap here.
//
// The scan-carrier-PR lane below is today's shape: the drain derives the delta locally and
// carries it to the target repo behind a draft PR. A successor lane, in which the target repo
// re-derives and commits the delta itself in response to the inbound event, is drafted but not
// ratified. Until that ruling is recorded and the successor is deployed and armed, this lane
// stands unchanged — do not anticipate the cutover, and when it comes, implement the new Lane and
// change SelectLane. The loop does not move.

// LaneName identifies a dispatch lane in the plan output and the exit ledger.
type LaneName string

const (
	// LaneScanCarrierPR carries the scan delta to the target repo behind a draft PR.
	LaneScanCarrierPR LaneName = "scan-carrier-pr"
	// LaneIssueFiling files an issue through the sanctioned filing verb (the bug, finding and
	// needs-decision exits).
	LaneIssueFiling LaneName = "issue-filing"
	// LaneRouting is the JUDGMENT lane. It executes nothing: it emits the item for a model tier to
	// route through the five tracked exits. The routing test — who told us to do this, and which
	// exit this becomes — is judgment, and a loop that computed it would be confidently wrong in
	// the direction that compounds through every worker the resulting brief spawns.
	LaneRouting LaneName = "routing"
)

// LaneRequest is everything a lane is given. It is a value, not a handle to the loop: a lane cannot
// re-enter the drain, re-read the queue, or change the window.
type LaneRequest struct {
	Item     loopengine.Item
	Tier     loopengine.Tier
	Root     string // the target checkout the scan is rooted at
	Worktree string // the ISOLATED linked worktree the scan runs in — never the shared checkout
	Branch   string // the scan branch cut inside that worktree
	Base     string // the ref the worktree is cut from (a fetched REMOTE head, never a local branch)
	DiffBase string // the ref the derived counts are diffed against
	Open     *OpenScanPR
	Policy   CoalescePolicy
	Now      time.Time
	DryRun   bool
}

// LaneOutcome is what a lane hands back to Land.
type LaneOutcome struct {
	Lane     LaneName
	Exit     Exit
	Artifact string
	Decision CoalesceDecision
	// Steps is the exact command sequence the lane ran, or — under --dry-run — would run. It is
	// the audit surface: a lane that reports an outcome without the steps that produced it is a
	// claim, not evidence.
	Steps []string
}

// Lane is the swappable unit. One method, one direction, no callbacks into the loop.
type Lane interface {
	Name() LaneName
	Execute(LaneRequest) (LaneOutcome, error)
}

// SelectLane is THE seam — the one function that maps a tiered item onto a lane. A cutover swaps
// what this returns; the drain around it is untouched.
func SelectLane(it loopengine.Item, tier loopengine.Tier, x Exec, w WriteFile) Lane {
	if tier == loopengine.TierSession {
		// Judgment: emitted, never executed.
		return routingLane{}
	}
	switch LaneName(it.Payload["lane"]) {
	case LaneIssueFiling:
		return issueFilingLane{exec: x}
	default:
		return scanCarrierPRLane{exec: x, write: w}
	}
}

// WriteFile is the file seam the derived title and body are captured through. The derivation verb
// prints to stdout by design and both the PR verb and the drift gate read files, so the capture is
// part of the lane rather than a shell redirection this binary cannot see.
type WriteFile func(path, content string) error

// Exec is the process seam. Every outward and local command a lane runs goes through it, so the
// lanes' command SHAPES are unit-testable without a checkout, a remote or a token.
type Exec func(dir, name string, args ...string) (string, error)

// RealExec runs commands for real.
func RealExec(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

// ---------------------------------------------------------------------------
// the scan-carrier-PR lane
// ---------------------------------------------------------------------------

// scanCarrierPRLane derives the placeholder delta in an ISOLATED worktree and carries it to the
// target repo as a draft PR.
//
// Three non-negotiables, all learned the same way — a scan rooted at a shared checkout once left a
// dozen stale-modified and a dozen untracked placeholder files behind, uncommitted, resurrecting
// content the default branch had already superseded:
//
//   - ISOLATE. The scan runs in its own linked worktree. It is never rooted at a shared checkout,
//     and no branch is cut inside one. assertIsolatedScanRoot refuses before anything runs, and the
//     scanner enforces the same rule again on its own account.
//   - SYNC FRESH. The worktree is cut from the fetched remote head, never from whatever the local
//     branch happens to be: a stale base makes the scan re-create placeholders the default branch
//     has since enriched.
//   - COMMIT AND CARRY, ALWAYS. The run is not done until the delta is committed and a draft PR is
//     open. Placeholder files left uncommitted in ANY tree are dirt, not output.
type scanCarrierPRLane struct {
	exec  Exec
	write WriteFile
}

func (l scanCarrierPRLane) Name() LaneName { return LaneScanCarrierPR }

func (l scanCarrierPRLane) Execute(req LaneRequest) (LaneOutcome, error) {
	out := LaneOutcome{Lane: LaneScanCarrierPR, Exit: ExitPlaceholder}

	if err := assertIsolatedScanRoot(req.Root, req.Worktree); err != nil {
		return out, err
	}
	repo := strings.TrimSpace(req.Item.Payload["repo"])
	if repo == "" {
		return out, deskkit.Refused("scan-carrier lane: the item carries no target repo")
	}
	if !deskkit.IsAllowedRepo(repo) {
		return out, deskkit.Refused("scan-carrier lane: " + repo + " is outside the configured write boundary — " +
			"the scan READ scope and the write boundary are deliberately different sets, and this lane writes")
	}

	decision, why := req.Policy.Decide(req.Open, req.Now)
	out.Decision = decision

	run := l.exec
	if run == nil {
		run = RealExec
	}
	write := l.write
	if write == nil {
		write = writeTextFile
	}
	capture := func(dir, name string, args ...string) (string, error) {
		out.Steps = append(out.Steps, renderStep(dir, name, args))
		if req.DryRun {
			return "", nil
		}
		return run(dir, name, args...)
	}
	step := func(dir, name string, args ...string) error {
		_, err := capture(dir, name, args...)
		return err
	}

	base := req.Base
	if base == "" {
		base = "refs/remotes/origin/main"
	}
	diffBase := req.DiffBase
	if diffBase == "" {
		diffBase = "origin/main"
	}

	// 1. sync fresh, then isolate.
	if err := step(req.Root, "git", "fetch", "origin"); err != nil {
		return out, err
	}
	cutFrom := base
	if decision.Act() == CoalesceInto {
		// Re-cut from the open scan branch, with the remote head merged in first: never scan a
		// base that is behind the default branch.
		cutFrom = "origin/" + req.Open.Branch
	}
	if err := step(req.Root, "git", "worktree", "add", req.Worktree, "-b", req.Branch, cutFrom); err != nil {
		return out, err
	}
	if decision.Act() == CoalesceInto {
		if err := step(req.Worktree, "git", "merge", "--no-edit", base); err != nil {
			return out, err
		}
	}

	// 2. derive the delta. The scanner is wrapped, never re-implemented, and it refuses a shared
	//    checkout on its own account — this lane's guard is the earlier, cheaper one.
	if err := step(req.Worktree, "statusgen", "--root", ".", "--scan-issues"); err != nil {
		return out, err
	}

	// 3. commit the delta, path-scoped: the run is not done until it is committed.
	if err := step(req.Worktree, "git", "add", deskkit.ScanDir); err != nil {
		return out, err
	}
	if err := step(req.Worktree, "git", "commit", "-m", "chore(intake): scan"); err != nil {
		return out, err
	}

	// 4. carry it. Push and title/body regeneration are ONE call — see PushAndRegenerate.
	//
	// The title and body are DERIVED from the branch's own diff on EVERY push, never hand-written
	// and never carried over from the previous push: the counts describe a diff that grows with
	// each coalesced commit.
	titleFile := filepath.Join(req.Worktree, scanTitleFile)
	bodyFile := filepath.Join(req.Worktree, scanBodyFile)
	var title string
	regenerate := func() error {
		t, err := capture(req.Worktree, "deskscanbody", "emit", "--base", diffBase, "--format", "title")
		if err != nil {
			return err
		}
		title = strings.TrimSpace(t)
		if req.DryRun && title == "" {
			title = derivedTitlePlaceholder
		}
		body, err := capture(req.Worktree, "deskscanbody", "emit", "--base", diffBase, "--format", "body")
		if err != nil {
			return err
		}
		if !req.DryRun {
			if err := write(titleFile, title+"\n"); err != nil {
				return err
			}
			if err := write(bodyFile, body); err != nil {
				return err
			}
		}
		// The gate half: it exits 5 when the stated counts disagree with the diff and 6 when it
		// cannot take the diff at all. A could-not-check is never a pass. It runs on the dry-run
		// path too — as a printed step — so the sequence an operator reviews is the sequence that
		// runs, gate included.
		return step(req.Worktree, "deskscanbody", "check", "--base", diffBase, "--text-file", titleFile)
	}

	switch decision.Act() {
	case CoalesceInto:
		out.Artifact = fmt.Sprintf("%s#%d", repo, req.Open.Number)
		err := PushAndRegenerate(
			func() error { return step(req.Worktree, "deskpr", "update") },
			func() error {
				if err := regenerate(); err != nil {
					return err
				}
				// There is no sanctioned EDIT verb: the PR-opening verb is create-only by
				// construction and there is no ready/edit/close verb beside it. The title/body
				// refresh therefore goes out on the direct path, after the write-boundary check
				// above and under the same kill switch as every other write this binary makes.
				// Move it to a sanctioned verb the day one exists.
				return step(req.Worktree, "gh", "pr", "edit", strconv.Itoa(req.Open.Number),
					"--title", title, "--body-file", bodyFile)
			})
		if err != nil {
			return out, err
		}
	default:
		out.Artifact = repo + " (new draft scan PR)"
		if err := regenerate(); err != nil {
			return out, err
		}
		if err := step(req.Worktree, "deskpr", "create", "--title", title, "--body-file", bodyFile); err != nil {
			return out, err
		}
	}

	// 5. put the isolated worktree back.
	if err := step(req.Root, "git", "worktree", "remove", req.Worktree); err != nil {
		return out, err
	}

	out.Steps = append(out.Steps, "# coalesce: "+string(decision)+" — "+why)
	return out, nil
}

// ---------------------------------------------------------------------------
// the issue-filing lane
// ---------------------------------------------------------------------------

// issueFilingLane files an issue through the sanctioned filing verb, which dedupes first and stamps
// the raising loop. Omitting that stamp is NOT neutral: the issue lands with unknown provenance,
// which is the absence of an answer and never "a human raised it".
type issueFilingLane struct{ exec Exec }

func (l issueFilingLane) Name() LaneName { return LaneIssueFiling }

func (l issueFilingLane) Execute(req LaneRequest) (LaneOutcome, error) {
	out := LaneOutcome{Lane: LaneIssueFiling}

	repo := strings.TrimSpace(req.Item.Payload["repo"])
	if repo == "" {
		return out, deskkit.Refused("issue-filing lane: the item carries no target repo")
	}
	if !deskkit.IsAllowedRepo(repo) {
		return out, deskkit.Refused("issue-filing lane: " + repo + " is outside the configured write boundary")
	}
	exit := Exit(req.Item.Payload["exit"])
	if !exit.Filed() {
		return out, deskkit.Refused("issue-filing lane: " + string(exit) + " is not an exit this lane files; " +
			"the filed exits are " + strings.Join(filedExitNames(), ", "))
	}
	title := strings.TrimSpace(req.Item.Payload["title"])
	bodyFile := strings.TrimSpace(req.Item.Payload["body-file"])
	if title == "" || bodyFile == "" {
		return out, deskkit.Refused("issue-filing lane: a filed exit needs both a title and a body file")
	}

	run := l.exec
	if run == nil {
		run = RealExec
	}
	args := []string{"new", "--repo", repo, "--title", title, "--body-file", bodyFile, "--raised-by", raisedByRole}
	for _, lbl := range exit.Labels() {
		args = append(args, "--label", lbl)
	}
	out.Steps = append(out.Steps, renderStep(req.Root, "deskfile", args))
	if !req.DryRun {
		if _, err := run(req.Root, "deskfile", args...); err != nil {
			return out, err
		}
	}
	out.Exit = exit
	out.Artifact = repo
	return out, nil
}

// raisedByRole is this loop's role in the roster's role bindings — the same binding its App token is
// minted against. The filing verb refuses (exit 5) any role the roster does not bind and prints the
// bound set, so this is a declared value rather than a name spelled after the skill.
const raisedByRole = "issue-loop"

// ---------------------------------------------------------------------------
// the routing lane (judgment — emitted, never executed)
// ---------------------------------------------------------------------------

type routingLane struct{}

func (routingLane) Name() LaneName { return LaneRouting }

// Execute EMITS. It runs no command and makes no write: the item is handed to a model tier with the
// five exits named, and the drain waits for the structured result. Computing this here is exactly
// the mistake the tier split exists to prevent.
func (routingLane) Execute(req LaneRequest) (LaneOutcome, error) {
	return LaneOutcome{
		Lane: LaneRouting,
		Exit: ExitUnrouted,
		Steps: []string{
			"# JUDGMENT — not computed here. Route " + req.Item.ID + " to exactly one tracked exit: " +
				strings.Join(exitNames(), " · "),
			"# The ownership test (was this monitor-fired or human-directed?) and the exit choice are " +
				"the model tier's call; this loop supplies the queue, the trust verdict and the ledger.",
		},
	}, nil
}

// ---------------------------------------------------------------------------
// isolation guard
// ---------------------------------------------------------------------------

// assertIsolatedScanRoot refuses a scan worktree that is not genuinely isolated: an empty or
// relative path (a failed worktree creation would otherwise leak the current directory into every
// step that follows), and — the real defect — a worktree nested INSIDE the checkout being scanned,
// which is the shared-checkout scan wearing a different name.
func assertIsolatedScanRoot(root, worktree string) error {
	if strings.TrimSpace(worktree) == "" {
		return deskkit.Refused("scan-carrier lane: no scan worktree path — refusing to run in the current directory")
	}
	if !filepath.IsAbs(worktree) {
		return deskkit.Refused("scan-carrier lane: the scan worktree path must be ABSOLUTE (" + worktree +
			"); a relative path resolves against whatever directory the step happens to run in")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return deskkit.Unverifiable("scan-carrier lane: cannot resolve the target checkout "+root, err)
	}
	absWT := filepath.Clean(worktree)
	if absWT == absRoot {
		return deskkit.Refused("scan-carrier lane: the scan worktree IS the target checkout — " +
			"the scan runs in its own linked worktree, never rooted at a shared checkout")
	}
	if strings.HasPrefix(absWT, absRoot+string(filepath.Separator)) {
		return deskkit.Refused("scan-carrier lane: the scan worktree " + absWT + " is nested inside the target checkout " +
			absRoot + " — that is a shared-checkout scan under another name")
	}
	return nil
}

func renderStep(dir, name string, args []string) string {
	return "(" + dir + ") " + name + " " + strings.Join(args, " ")
}

// scanTitleFile / scanBodyFile are the derived title and body, captured INSIDE the per-run isolated
// worktree rather than at a fixed shared path. A fixed path under the system temp directory is
// shared machine-wide, so two parallel scans clobber each other's PR body; a path inside a worktree
// that exists only for this run cannot collide.
const (
	scanTitleFile = ".scan-title"
	scanBodyFile  = ".scan-body"
	// derivedTitlePlaceholder stands in for the real title on the dry-run path, where no derivation
	// verb actually ran. It is deliberately not a plausible title: a dry-run step an operator could
	// mistake for the real one is worse than an obviously synthetic one.
	derivedTitlePlaceholder = "<derived from the branch diff at run time>"
)

func writeTextFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
