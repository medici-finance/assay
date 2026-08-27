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
	"strings"
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

	var b strings.Builder
	fmt.Fprintf(&b, "# Assignment — %s\n\n", o.item)
	fmt.Fprintf(&b, "- **Item key:** `%s`\n", o.item)
	fmt.Fprintf(&b, "- **Target repo:** `%s` — the PR opens THERE, not anywhere else.\n", repo)
	fmt.Fprintf(&b, "- **Checkout base:** `%s` — the `git -C` source you branch FROM. It is not your writable root.\n", base)
	fmt.Fprintf(&b, "- **Your home worktree:** `%s` — every file operation stays under it.\n", home)
	fmt.Fprintf(&b, "- **Branch:** `%s`\n", branch)
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
	b.WriteString("## Open the draft PR in that repo\n\n")
	fmt.Fprintf(&b, "Run `deskpr create` from INSIDE your worktree, so the PR lands against `%s`'s own main. "+
		"Stop at `implemented`: never set verified/done and never flip a PR ready.\n\n", repo)
	b.WriteString("Self-register the instant your draft PR opens:\n\n")
	fmt.Fprintf(&b, "```\nDESK_SESSION=<your-session> deskroster set --repo %s --pr <N> --what %q\n```\n\n",
		shortRepo(repo), o.item)
	b.WriteString("Release the dispatch claim once your branch is pushed — branch-as-claim takes over:\n\n")
	// With no --claim-root the script sits in the agent's own worktree, so the stable
	// repo-relative spelling is kept (it is also machine-independent, which keeps two
	// dispatchers' prompts byte-identical). With --claim-root the worktree does NOT carry
	// the script, so the resolved path is stated — a tool to invoke, not a place to work.
	releaseTool := claimScriptRel
	if strings.TrimSpace(o.claimRoot) != "" {
		releaseTool = plan.claimScript
	}
	// The release names the CLAIM key — the one the acquire was taken under (translated
	// from the plan item key when they differ) — or the agent would release a key nobody
	// holds and the real claim would sit until its TTL.
	fmt.Fprintf(&b, "```\n%s release %q --repo %s\n```\n", releaseTool, plan.claimKey, repo)

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
