package main

// decisiontransitiongate.go — sdlc/18: make the gate:human contract BINDING.
//
// THE DEFECT (F-gate-merged-first). The gate:human contract says the human decides
// BEFORE the irreversible act, but nothing mechanically stopped a gate:human brief from
// reaching implemented/verified while its decision issue sat open and unruled. Three
// confirmed instances landed anyway — sdlc/05 (3 days early), agentic-metrics/08
// (verified before the gate was answered), drain-harness/08 (10 days early, ~3,200 lines
// of skill prose deleted). All three were ratified retrospectively; none could have been
// stopped by anything that existed at the time.
//
// RULED — medici-finance/assay-toolkit#2063, 2026-09-09 (Ian):
//   - ADOPTED: a gate:human brief may not flip to implemented or verified while its
//     decision issue is open and unruled. Enforcement is at the STATUS TRANSITION.
//   - ADOPTED: do not file a decision-gate issue for a brief whose human review is
//     already recorded (the inverse defect — distribution/04's shape — is a separate fix,
//     tools/decision-issue.sh's own concern, out of THIS repo's reach; see the PR body).
//   - CONSIDERED AND REJECTED, deliberately: widening the refusal into deskflip's
//     ready-flip. Decision-issue latency is the queue's current bottleneck; a flip-block
//     would wedge it. Nothing in this file, and nothing this file is wired into, touches
//     deskflip — see brief-18 Verify row 8, a NEGATIVE row that fails if a later change
//     "improves" on this boundary.
//
// WHERE THIS LIVES, AND WHY NOT --lint. Whether a decision issue carries a RULING is a
// LIVE GitHub read (the issue's comment history) — the same reason decisiongateanchor.go's
// live-fetch plumbing is kept OUT of the offline --lint envelope (see that file's header).
// --lint stays deterministic and network-free (it gates STATUS.md generation); a gh flake
// must never turn into a spurious hard PROBLEM there. This check is instead wired into
// `statusgen --corroborate <pr>` (corroborate.go), which is ALREADY the network-capable,
// per-PR, hard-fail (exit 1) verb an adopter's own CI chooses to run on a PR — exactly the
// "transition path" a status-cell edit rides. checkBriefFiles/brieffile.go already carries
// a SOFT (NOTICE) precursor of part of this ("brief is gate:human at implemented/verified
// but has no decision-issue" — the offline half of the same fact); this file is the HARD,
// online half that actually refuses the transition.
//
// THE PURE CORE VS THE LIVE PLUMBING. decisionRuled / decisionTransitionRefusal /
// decisionTransitionGateProblems consume PRE-FETCHED decisionIssueState values, so the
// "is this ruled" question is offline-testable against fixtures (brief-18 Verify rows
// 1-5 and 9, class check:ci) exactly the way decisionGateCorroboration is.
// fetchDecisionIssueState is the untested live-fetch plumbing, kept separate for the same
// reason fetchDecisionGateIssue is.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// decisionTransitionGateStatuses are the two lifecycle positions brief-18 Task 1 guards:
// a gate:human brief may not reach either while its decision issue is open and unruled,
// or absent. `todo`, `in-progress`, `done` and `blocked` are untouched — `done` already
// carries its own stricter human-review requirement (lifecycle-v1.md §2.5/§4.3), and
// gating `in-progress` is a DIFFERENT, already-shipped control (the design-approval gate,
// designgate.go, sdlc/05) that this file does not duplicate.
var decisionTransitionGateStatuses = map[string]bool{
	"implemented": true,
	"verified":    true,
}

// decisionIssueComment is one comment on a decision issue, pre-fetched.
type decisionIssueComment struct {
	Author string // GitHub login
	Body   string
}

// decisionIssueState is one decision issue's pre-fetched state — open/closed plus its
// comment history — consumed by the pure core below. A nil *decisionIssueState paired
// with unreadable=false means "no decision issue exists at all" (bf.DecisionIssue == 0);
// paired with unreadable=true it means "one is on record but could not be read" — the
// three-state distinction decisionTransitionRefusal's caller must keep honest (C4: a
// could-not-check read is never a pass).
type decisionIssueState struct {
	Ref      string // "owner/repo#N", for messages
	Open     bool
	Comments []decisionIssueComment
}

// decisionRuled reports whether iss carries a RECORDED DRIVER RULING (brief-18 Task 2):
// a comment authored by the blessed human login — rosterconfig.go's ASSAY_BLESS_LOGIN,
// the single highest-authority login in the trust roster, the same identity
// decisionGateCorroboration's third anchor already keys its own "closed by the blessed
// human" condition on (decisiongateanchor.go). Reusing it here means one roster identity
// answers "who is the driver" everywhere this codebase asks the question, rather than a
// second, divergent notion of "human" growing up beside it.
//
// A comment from any OTHER login — most importantly the desk's own bot/App account
// relaying what the driver said elsewhere ("Ian said in standup: ship it") — does NOT
// count. Every one of the three confirmed F-gate-merged-first instances had exactly such
// a relay standing in for a ruling; this is the distinction Task 2 asks for.
//
// Closing the issue is deliberately NOT required: Ian's ruling scopes the block to "open
// ... with no recorded ruling" (Task 1), so a driver comment on a STILL-OPEN issue already
// satisfies it — the driver may leave the paper trail open while downstream work proceeds.
func decisionRuled(iss *decisionIssueState) (evidence string, ruled bool) {
	if iss == nil {
		return "", false
	}
	blessLogin := strings.TrimSpace(scanEffectiveConfig().Bless.Login)
	if blessLogin == "" {
		return "", false // no blessed driver configured — fail closed, same direction decisionGateCorroboration takes
	}
	for _, c := range iss.Comments {
		if strings.EqualFold(strings.TrimSpace(c.Author), blessLogin) {
			return fmt.Sprintf("%s: driver-authored comment by %s", iss.Ref, blessLogin), true
		}
	}
	return "", false
}

// decisionTransitionRefusal is the pure core of brief-18 Tasks 1+2: given a brief's
// frontmatter, the status it is being moved to (or is sitting at), and its decision
// issue's pre-fetched state, decide whether the transition must be REFUSED.
//
// iss == nil && !unreadable means "bf.DecisionIssue == 0" — no decision issue on record
// at all. Task 1 is explicit that this is its OWN refusal, not a free pass: "An absent
// decision issue is its own refusal — silence must not read as permission" (brief-18
// Verify row 3).
//
// iss == nil && unreadable means a decision issue IS on record but its live state could
// not be fetched — a could-not-check, never rounded up to a pass (C4). The transition is
// refused with a message that says so honestly, distinct from "issue open, unruled".
func decisionTransitionRefusal(bf *BriefFile, status string, iss *decisionIssueState, unreadable bool) (message string, refuse bool) {
	if bf.Gate != "human" {
		return "", false // scoped to gate: human only — brief-18 Verify row 5
	}
	if !decisionTransitionGateStatuses[status] {
		return "", false
	}
	if bf.DecisionIssue == 0 {
		return fmt.Sprintf("%s: gate:human brief has status %q with NO decision issue on record — "+
			"silence is not permission; file the decision issue and get a driver ruling before this brief "+
			"may reach %q", bf.Brief, status, status), true
	}
	if unreadable || iss == nil {
		return fmt.Sprintf("%s: decision issue #%d's live state could not be read — refusing (a "+
			"could-not-check read is never treated as a pass); the transition to %q is blocked until it "+
			"can be confirmed ruled", bf.Brief, bf.DecisionIssue, status), true
	}
	if _, ruled := decisionRuled(iss); ruled {
		return "", false
	}
	openWord := "open"
	if !iss.Open {
		openWord = "closed"
	}
	return fmt.Sprintf("%s: decision issue %s is %s with NO recorded driver ruling (a desk/bot relay "+
		"does not count) — the transition to %q is refused until the driver comments on the issue",
		bf.Brief, iss.Ref, openWord, status), true
}

// decisionTransitionGateProblems walks every stream's brief files and returns a hard
// PROBLEM for every gate:human brief that decisionTransitionRefusal refuses.
//
// statusOverride, when non-nil, restricts the scan to the brief IDs it names and uses ITS
// status value rather than the README row's current one — this is how the live wiring in
// corroborate.go scopes the check to transitions THIS PR's diff actually introduces
// (transitionsInDiff), rather than re-flagging a brief that was already sitting at
// implemented/verified before the PR touched anything. Scoping matters: brief-18's own
// exec-tier-why warns that a version of this check broad enough to fire on every PR that
// merely TOUCHES a board file (rather than the one making the move) would wedge the
// queue, which is the fleet's current bottleneck.
//
// statusOverride == nil is the fixture-test shape (brief-18 Verify rows 1-5, 9): the
// current README row status IS the "moved to" state under test, with no diff involved.
//
// fetch resolves one brief's decision issue to its pre-fetched state and an unreadable
// flag; nil fetch (or a brief with bf.DecisionIssue == 0, which never calls fetch at all)
// exercises the "no issue on record" branch only.
func decisionTransitionGateProblems(streams []*Stream, statusOverride map[string]string, fetch func(bf *BriefFile) (*decisionIssueState, bool)) []string {
	var problems []string
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed reported elsewhere; legacy exempt
			}
			id, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			var status string
			if statusOverride != nil {
				st, present := statusOverride[id]
				if !present {
					continue // this PR's diff does not move this brief — not this PR's business
				}
				status = st
			} else {
				row := findRow(s, num)
				if row == nil {
					continue
				}
				status = row.Status
			}
			var iss *decisionIssueState
			unreadable := false
			if bf.DecisionIssue != 0 && fetch != nil {
				iss, unreadable = fetch(bf)
			}
			if msg, refuse := decisionTransitionRefusal(bf, status, iss, unreadable); refuse {
				problems = append(problems, fmt.Sprintf("%s: %s", path, msg))
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// transitionsInDiff parses a unified diff (the same form stampsInDiff consumes) and
// returns canonical brief-id ("<stream>/<NN>") -> new Status-cell value for every ADDED
// status-table row it finds — i.e. the transitions THIS diff introduces. It is pure (no
// git, no network) and hence directly fixture-testable on a literal diff string, unlike
// the gh-backed plumbing below.
//
// briefRowKey (corroborate.go) returns only the bare row NUMBER, not a canonical id — it
// is scoped per-file by its own caller (markPreExisting compares within one file). This
// combines that number with the STREAM name derived from the file's own directory (the
// same "<dirname>" expectedBriefID uses for a brief-<NN>.md file), so the result matches
// the id shape decisionTransitionGateProblems' statusOverride keys on.
//
// The Status column is located by its FIXED position in the canonical status table —
// `| # | Brief | Wave | Effort | Status | Verified | Reviewed |` (briefRowKey's own doc
// comment) — rather than by re-deriving the header from the diff: a single-cell edit's
// unified diff typically reprints only the changed ROW, not the unchanged header line
// above it, so a header-tracking approach (as stampsInDiff needs for its OPTIONAL
// pre-existing-cell comparison) would see no header at all for the common case and never
// fire. The canonical column order is a documented, load-bearing invariant of the status
// table format elsewhere in this file already; this reuses it rather than re-deriving it.
func transitionsInDiff(diff string) map[string]string {
	const statusCellIndex = 4
	out := map[string]string{}
	curFile := ""
	for _, line := range strings.Split(diff, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "diff --git ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 4 {
				curFile = strings.TrimPrefix(fields[3], "b/")
			}
			continue
		}
		if strings.HasPrefix(trimmed, "+++ ") {
			curFile = strings.TrimPrefix(trimmed, "+++ b/")
			continue
		}
		if !strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "+++") {
			continue
		}
		content := strings.TrimPrefix(trimmed, "+")
		num := briefRowKey(content)
		if num == "" {
			continue
		}
		stream := filepath.Base(filepath.Dir(curFile))
		if stream == "" || stream == "." || stream == string(filepath.Separator) {
			continue
		}
		cells := splitTableCells(content)
		if len(cells) <= statusCellIndex {
			continue
		}
		out[stream+"/"+num] = strings.TrimSpace(cells[statusCellIndex])
	}
	return out
}

// ---- live-fetch plumbing (untested, like fetchPRData / fetchDecisionGateIssue) --------

// fetchDecisionIssueState reads one decision issue's open/closed state and every
// comment's author via `gh api`, for the "ruled" question decisionRuled answers. Untested
// plumbing: any failure returns (nil, true) — unreadable, never a fabricated verdict —
// so the caller reports could-not-check rather than silently treating a network hiccup as
// either "ruled" or "not ruled".
func fetchDecisionIssueState(repo string, number int) (*decisionIssueState, bool) {
	out, err := exec.Command("gh", "api", fmt.Sprintf("repos/%s/issues/%d", repo, number)).Output()
	if err != nil {
		return nil, true
	}
	var payload struct {
		State string `json:"state"`
	}
	if json.Unmarshal(out, &payload) != nil {
		return nil, true
	}
	cout, cerr := exec.Command("gh", "api", fmt.Sprintf("repos/%s/issues/%d/comments", repo, number), "--paginate").Output()
	if cerr != nil {
		return nil, true
	}
	var raw []struct {
		Body string   `json:"body"`
		User ghAuthor `json:"user"`
	}
	if json.Unmarshal(cout, &raw) != nil {
		return nil, true
	}
	iss := &decisionIssueState{
		Ref:  fmt.Sprintf("%s#%d", repo, number),
		Open: !strings.EqualFold(payload.State, "closed"),
	}
	for _, c := range raw {
		iss.Comments = append(iss.Comments, decisionIssueComment{Author: c.User.Login, Body: c.Body})
	}
	return iss, false
}
