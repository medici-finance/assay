package main

// prompt.go — the assembled agent prompt.
//
// THE SHAPE, AND WHY IT IS THIS SHAPE. A dispatch prompt has two halves that must not be
// mixed: the ASSIGNMENT (which item, which repo, which worktree, which tier — different
// every time) and the CLAUSES (the rules every agent of that class receives — identical
// every time). Prose dispatch mixed them, which is how a clause got dropped whenever a
// dispatcher was in a hurry and how the wording drifted whenever one was thorough.
//
// So the assignment is COMPUTED from the invocation and the clauses are QUOTED from the
// kit, and the boundary between them is a heading a reader can see. Two sessions
// dispatching the same item produce byte-identical text, which is the repeatability aim:
// identical behaviour because the machinery is computed, not re-interpreted.
//
// FOUR THINGS GO IN VERBATIM, FROM THE INVOCATION, and no agent infers any of them: the
// target repo, the repo root the worktree is cut from, the literal instruction to isolate
// in an owned worktree OF THAT REPO with the command that produces one, and the
// instruction to open the draft PR IN THAT REPO. An agent handed an item from one repo and
// a worktree of another will helpfully recreate the work in the wrong place, and the
// result reads as real work on a repo that never asked for it.
//
// NO SHARED-CHECKOUT PATH EVER APPEARS HERE AS A PLACE TO WORK. A path carried in a
// prompt overrides every isolation layer beneath it, because the agent simply uses the
// path it was given. The only absolute path this prompt states as somewhere to BE is the
// agent's OWN worktree. One narrow exception is a TOOL path: with --claim-root the claim
// script does not exist in the agent's worktree at all, so the release command must name
// the script where it actually is — invoked, never worked in — or the instruction is one
// the agent cannot follow.

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// tierClause is the pickup-time STOP a strong-tier item carries. It is emitted VERBATIM
// and only for the strong tier: an item that does not demand a strong implementer must
// not carry a line telling a cheap-tier agent to hand back, or every dispatch becomes a
// negotiation.
const tierClause = "If you are a fast/cheap-tier model, STOP — this item requires a strong " +
	"implementer. Report which model you are and hand back."

// homeUnknown is what the prompt shows for the agent's worktree on a --dry-run, where no
// worktree was created and therefore none can be named.
//
// It is NOT a predicted path. deskwt owns where a worktree lands, and a second prediction
// of that location here would be a second source of truth for the one value the isolation
// floor rests on — so the dry-run prompt says plainly that the path is not yet known
// rather than printing a guess an operator might paste into a real dispatch.
const homeUnknown = "<not created: --dry-run; the real dispatch names the path `deskwt add` printed>"

// assemblePrompt builds the full prompt: assignment, then the kit's clauses verbatim.
// home is the agent's worktree path as REPORTED by deskwt, or "" on a dry run. The plan
// carries the values validation derived — the repo, the branch, and the resolved claim
// script — so the prompt states what was checked, never a re-derivation of it.
func assemblePrompt(o dispatchOpts, plan dispatchPlan, home string) (string, error) {
	repo, branch := plan.repo, plan.branch
	common, err := commonKitText()
	if err != nil {
		return "", err
	}
	kit, err := kitText(o.kit)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		home = homeUnknown
	}
	// The checkout base is stated ABSOLUTE. A relative path means nothing to the agent:
	// it resolves against whatever directory that process happens to start in, which is
	// exactly the ambiguity the isolation clauses exist to remove. If it cannot be made
	// absolute, the caller's spelling is used as given rather than silently dropped.
	base := o.root
	if abs, err := filepath.Abs(base); err == nil {
		base = abs
	}

	review := reviewKit(o.kit)

	var b strings.Builder
	fmt.Fprintf(&b, "# Assignment — %s\n\n", o.item)
	fmt.Fprintf(&b, "- **Item key:** `%s`\n", o.item)
	if review {
		// A reviewer opens no PR, so the "the PR opens THERE" framing is not just noise here —
		// it is the very scaffold that leads a reviewer to open a spurious draft PR. The repo
		// is named as the tree the PR under review belongs to, which is the value the review
		// clauses require path claims to be resolved against.
		fmt.Fprintf(&b, "- **Target repo:** `%s` — the repository the PR under review belongs to; resolve every path claim there.\n", repo)
	} else {
		fmt.Fprintf(&b, "- **Target repo:** `%s` — the PR opens THERE, not anywhere else.\n", repo)
	}
	fmt.Fprintf(&b, "- **Checkout base:** `%s` — the `git -C` source your worktree is cut FROM. It is not your writable root.\n", base)
	fmt.Fprintf(&b, "- **Your home worktree:** `%s` — every file operation stays under it.\n", home)
	// The auto-cut worktree branch is the IMPLEMENTER's output surface. A reviewer produces
	// no branch and must review the PR's HEAD, not this fresh branch off main, so naming it
	// here would only invite a reviewer to work on the wrong tree.
	if !review {
		fmt.Fprintf(&b, "- **Branch:** `%s`\n", branch)
	}
	fmt.Fprintf(&b, "- **Execution tier:** `%s`\n", o.tier)
	if strings.TrimSpace(o.brief) != "" {
		fmt.Fprintf(&b, "- **Specification:** `%s` — implement to its contract; do not expand scope.\n", o.brief)
	}
	if plan.gateHuman {
		b.WriteString("- **Human-gated item:** a decision issue is open for it. Do not pre-empt the decision; " +
			"implement what is already settled and stop where the decision begins.\n")
	}
	b.WriteString("\n## Isolate first\n\n")
	b.WriteString("Work in an owned worktree OF THAT REPO. It already exists; if you must recreate it:\n\n")
	fmt.Fprintf(&b, "```\ngit -C %s worktree add %s refs/remotes/origin/main --detach\n```\n\n", base, home)
	b.WriteString("Check `git rev-parse --show-toplevel` before your first write and ABORT if it resolves " +
		"anywhere but your home worktree.\n\n")
	// The assignment's action half is the ONE thing that differs by class: an implementer
	// OPENS a PR and stops at `implemented`; a reviewer opens nothing, produces a VERDICT,
	// and reviews the PR's head. Emitting the implementer scaffold to a reviewer is the
	// defect this split fixes — a reviewer handed "open the draft PR" opens a spurious one.
	if review {
		writeReviewAssignment(&b, o, plan, repo, home)
	} else {
		writeWorkerAssignment(&b, o, plan, repo)
	}

	if strings.EqualFold(o.tier, "strong") {
		fmt.Fprintf(&b, "\n%s\n", tierClause)
	}

	// The COMMON clauses come first, then the class kit. Order matters for a reader: the
	// isolation floor is what every later clause assumes, so it is the first thing after
	// the assignment rather than something the agent meets halfway down.
	b.WriteString("\n---\n\n# Standing clauses — common (quoted verbatim, not paraphrased)\n\n")
	b.WriteString(common)
	fmt.Fprintf(&b, "\n\n---\n\n# Standing clauses — %s (quoted verbatim, not paraphrased)\n\n", o.kit)
	b.WriteString(kit)
	b.WriteString("\n")
	return b.String(), nil
}

// reviewKit reports whether this dispatch is a REVIEW dispatch. A reviewer's assignment is
// read-only and PR-head-shaped rather than the implementer's open-a-PR scaffold, so the one
// place the assignment differs by class turns on this.
func reviewKit(kit string) bool { return strings.EqualFold(strings.TrimSpace(kit), "review") }

// writeWorkerAssignment emits the IMPLEMENTER's action half: open the draft PR in the target
// repo, self-register the instant it opens, and release the dispatch claim once the branch is
// pushed so branch-as-claim takes over. This is the scaffold an agent that PRODUCES a change
// needs; a reviewer, which produces a verdict and no branch, gets writeReviewAssignment. The
// text here is byte-for-byte what every worker dispatch has always carried.
func writeWorkerAssignment(b *strings.Builder, o dispatchOpts, plan dispatchPlan, repo string) {
	b.WriteString("## Open the draft PR in that repo\n\n")
	fmt.Fprintf(b, "Run `deskpr create` from INSIDE your worktree, so the PR lands against `%s`'s own main. "+
		"Stop at `implemented`: never set verified/done and never flip a PR ready.\n\n", repo)

	// Every desk WRITE verb (`deskpr create`, `deskfile`, `deskreply`) refuses with
	// $DESK_LOOP unset — the kill switch's per-loop `STOP.<loop>` flag has nothing to
	// match, so a stop a human is holding would silently fail. A dispatched worker must
	// NOT inherit the dispatching desk's DESK_LOOP (that resolves to the desk's App and
	// mints the WRONG identity for this worker's PR and comments); its OWN loop is
	// `worker-desk`, which resolves to the worker App. State it here, at the first write,
	// so the worker sets it now rather than meeting the refusal at the PR ceremony.
	b.WriteString("Before `deskpr create` — and before any desk write verb (`deskfile`, `deskreply`) — " +
		"set your OWN loop identity in this shell (do NOT inherit the dispatching desk's):\n\n")
	b.WriteString("```\nexport DESK_LOOP=worker-desk\n```\n\n")

	if n, ok := issueNumFromItem(o.item); ok {
		// An issue-only item carries an `Issue:` trailer, not a `Brief:` one — and
		// `deskpr create` REFUSES a body with neither. The emitted key never named which
		// line to add, so every issue-only worker discovered the refusal at the PR
		// ceremony; name the exact trailer here.
		fmt.Fprintf(b, "This is ISSUE-ONLY work: your `deskpr create` body MUST carry the trailer line "+
			"`Issue: #%d` (an issue-only PR carries `Issue: #<N>`, never a `Brief:` line).\n\n", n)
		// The sanctioned verb for a comment on the dispatched-from ISSUE — a
		// BLOCKED-ON-HUMAN report, a could-not-check note. `deskreply` is PR-only; a
		// hand-rolled `gh` write bypasses deskfile's dedupe/budget/self-containment gates.
		fmt.Fprintf(b, "To post on the ISSUE you were dispatched from (a `BLOCKED-ON-HUMAN` report, a "+
			"could-not-check note), the sanctioned verb is `deskfile attach -R %s --to %d --body-file F` "+
			"(with DESK_LOOP set, above). `deskreply` is for your OWN open PR only, and a hand-rolled `gh` "+
			"write on the issue bypasses deskfile's dedupe, budget and self-containment gates.\n\n", repo, n)
	}

	b.WriteString("Self-register the instant your draft PR opens:\n\n")
	fmt.Fprintf(b, "```\nDESK_SESSION=<your-session> deskroster set --repo %s --pr <N> --what %q\n```\n\n",
		shortRepo(repo), o.item)
	b.WriteString("Release the dispatch claim once your branch is pushed — branch-as-claim takes over:\n\n")
	writeReleaseClaim(b, o, plan, repo)
}

// writeReviewAssignment emits the REVIEWER's action half. A reviewer is READ-ONLY: it opens
// no PR, runs no `deskpr create`, pushes no branch, and does not stop at `implemented` —
// those are the implementer's scaffold, and a reviewer handed them can open a spurious draft
// PR for a PR that is already open or waste a round pushing a branch it must never touch. A
// reviewer reviews the PULL REQUEST's HEAD, so the assignment fetches pull/<N>/head into the
// worktree rather than leaving it on the fresh branch off main the worktree was cut on; a
// reviewer that does not fetch the head reviews the wrong tree. Its output is a VERDICT posted
// via `deskpost` per the review clauses below — never a PR, a merge, or a ready-flip.
func writeReviewAssignment(b *strings.Builder, o dispatchOpts, plan dispatchPlan, repo, home string) {
	b.WriteString("## Review the open PR — READ-ONLY\n\n")
	fmt.Fprintf(b, "You are REVIEWING %s in `%s`. This is a REVIEW, not an implementation: you produce a "+
		"VERDICT, never a change. Do not create a pull request, do not push a branch, and do not advance "+
		"the item past review. A reviewer posts its verdict via `deskpost` (per the standing review "+
		"clauses below); it never implements, merges, or flips a PR ready.\n\n", prRef(o.pr), repo)
	b.WriteString("Review the PULL REQUEST's HEAD, not the fresh branch your worktree was cut on. Fetch the " +
		"head into your worktree and check it out first:\n\n")
	// The server-side ref the change's HEAD is advertised under is FORGE-specific — GitHub
	// publishes it at refs/pull/<N>/head, GitLab at refs/merge-requests/<iid>/head (the MR's
	// own pipeline.ref). The refspec MUST follow the resolved forge of the target repo, or a
	// GitLab reviewer that follows this prompt verbatim fetches a GitHub-shaped coordinate that
	// does not exist and cannot check out the head it was dispatched to verdict (#773). The
	// forge was resolved pre-claim (validateCallerPreconditions) and carried on the plan.
	head := reviewHeadRefPrefix(plan.forgeKind)
	if o.pr > 0 {
		fmt.Fprintf(b, "```\ngit -C %s fetch origin %s/%d/head && git -C %s checkout FETCH_HEAD\n```\n\n",
			home, head, o.pr, home)
	} else {
		fmt.Fprintf(b, "```\ngit -C %s fetch origin %s/<N>/head && git -C %s checkout FETCH_HEAD\n```\n\n",
			home, head, home)
	}
	b.WriteString("Release the dispatch claim once your verdict is posted:\n\n")
	writeReleaseClaim(b, o, plan, repo)
}

// writeReleaseClaim emits the dispatch-claim release command. The release names the CLAIM key
// — the one the acquire was taken under (translated from the plan item key when they differ)
// — or the agent would release a key nobody holds and the real claim would sit until its TTL.
// With no --claim-root the script sits in the agent's own worktree, so the stable
// repo-relative spelling is kept (it is also machine-independent, which keeps two dispatchers'
// prompts byte-identical). With --claim-root the worktree does NOT carry the script, so the
// resolved path is stated — a tool to invoke, not a place to work.
func writeReleaseClaim(b *strings.Builder, o dispatchOpts, plan dispatchPlan, repo string) {
	// The release hint was resolved once (validateCallerPreconditions) alongside the claim
	// tool itself: the repo-relative script path when the script sits in the agent's own
	// worktree, the resolved absolute path under --claim-root, or the bare goClaimBinary name
	// when the pure-Go fallback is in use (a tool on PATH, not a place in the tree).
	releaseTool := plan.claimReleaseHint
	if releaseTool == "" {
		releaseTool = claimScriptRel
	}
	fmt.Fprintf(b, "```\n%s release %q --repo %s\n```\n", releaseTool, plan.claimKey, repo)
}

// issueNumFromItem extracts the GitHub issue number from an ISSUE-ONLY item key. The
// worker's dispatch key for issue-only work is `issue-<N>` (also carried in the
// repo-qualified plan-key form `<owner>/<name>:issue-<N>` and the claim-key form
// `<repo>--issue-<N>`); this reduces any of them to <N>. ok=false for a brief-based item —
// which carries a `Brief:` trailer, not an `Issue:` one — so the issue-shaped assignment
// lines are emitted for issue work only, mirroring briefIDFromItem's inverse case.
func issueNumFromItem(item string) (int, bool) {
	s := strings.TrimSpace(item)
	// Drop a `<owner>/<name>:` repo qualifier, then a `<repo>--` claim-key prefix, so the
	// tail is the bare item segment in every form deskdispatch is handed.
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "--"); i >= 0 {
		s = s[i+2:]
	}
	s = strings.Trim(s, "/")
	const pfx = "issue-"
	if !strings.HasPrefix(s, pfx) {
		return 0, false
	}
	n, err := strconv.Atoi(s[len(pfx):])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// reviewHeadRefPrefix is the forge-specific server-side ref NAMESPACE under which a change's
// HEAD commit is advertised, so a reviewer can fetch the reviewed head: GitHub publishes it at
// refs/pull/<N>/head, GitLab at refs/merge-requests/<iid>/head (the MR's own pipeline.ref).
// The prefix follows the resolved forge because a GitHub `pull/<N>/head` fetch against a GitLab
// MR is the wrong coordinate — the reviewer cannot check out the head it was dispatched to
// verdict (#773). GitHub is the fallback shape: a review dispatch always resolves the forge
// pre-claim (an unresolvable one refuses the dispatch), so an empty kind here is only reachable
// on a code path that bypassed that resolution, where the historical GitHub shape is the safe
// default rather than an empty ref.
func reviewHeadRefPrefix(kind deskkit.ForgeKind) string {
	if kind == deskkit.ForgeGitLab {
		return "merge-requests"
	}
	return "pull"
}

// prRef names the PR under review for the assignment prose. --pr is optional on a review
// dispatch, so when it is absent the prose points at "the PR named in your dispatch" rather
// than printing a wrong number.
func prRef(pr int) string {
	if pr > 0 {
		return fmt.Sprintf("PR #%d", pr)
	}
	return "the PR named in your dispatch"
}
