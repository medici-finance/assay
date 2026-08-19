package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// carriage says HOW a verify-desk dispatch requirement reaches the dispatched verifier.
type carriage int

const (
	// carriedInPrompt: rendered into the dispatch prompt. Every Probe MUST appear in the
	// rendered output.
	carriedInPrompt carriage = iota
	// carriedByEngine: enforced structurally elsewhere in the adapter/engine and deliberately
	// NOT prompt prose (a rule the code refuses to break beats a rule the worker is asked to
	// follow). Note names where.
	carriedByEngine
	// notYetCarried: a KNOWN, TRACKED gap — SKILL.md carries the requirement, this template
	// does not yet. Issue is mandatory, and every Probe MUST be ABSENT from the rendered
	// prompt, so the day someone lands the prose the registry has to be updated with it.
	notYetCarried
)

// dispatchRequirement maps ONE bullet of `.claude/skills/verify-desk/SKILL.md` loop step 2
// ("Its prompt MUST carry:") onto this template. Anchors are the substrings that identify the
// SKILL.md bullet; Probes are substrings of the rendered prompt.
type dispatchRequirement struct {
	ID      string   // stable slug, independent of either side's wording
	Anchors []string // ALL must appear (case-insensitively) in the SKILL.md bullet this maps to
	How     carriage
	Probes  []string // must appear in the rendered prompt (carriedInPrompt) / must NOT (notYetCarried)
	Issue   int      // tracking issue; REQUIRED for notYetCarried
	Note    string
}

// dispatchRequirements is the ONE in-code enumeration of the SKILL.md prompt requirements —
// it replaces the prose list that used to live in renderDispatchPrompt's doc-comment and that
// silently fell out of date (SKILL.md gained the name-and-derive step, the template and
// its doc-comment did not).
//
// TestDispatchRequirements_MatchSkillMD binds this table to the skill file in both directions:
// a SKILL.md bullet with no entry here is red, an entry that no longer matches any bullet is
// red, and a carriedInPrompt entry whose Probes are missing from the render is red. The
// enforcement is bullet-granular: it catches a requirement ADDED, REMOVED or RENAMED in
// SKILL.md, not wording drift strictly inside one bullet.
//
// The match is 1:1 — each entry must claim EXACTLY one bullet and each bullet exactly one
// entry. Anchors are therefore multi-token and specific on purpose: a single common token is
// one unrelated SKILL.md edit away from claiming a second bullet, and an entry that claims a
// second bullet marks it covered, silencing the orphan check. See the test's doc comment for
// the live instance this bound was written from.
//
// Bound worth stating plainly: enforcement is bullet-granular, so wording drift strictly
// inside one bullet still passes.
var dispatchRequirements = []dispatchRequirement{
	{
		ID: "brief-verify-table-and-target-sha",
		// "sha" alone collided with the name-and-derive bullet after that bullet gained both
		// "Verify table" and `git show <sha> --stat`; "merged-main sha" is the phrase unique
		// to THIS bullet. See the 1:1 rationale on TestDispatchRequirements_MatchSkillMD.
		Anchors: []string{"brief path", "verify table", "merged-main sha"},
		How:     carriedInPrompt,
		Probes:  []string{"Brief: ", "Target SHA"},
		Note:    "verify table supplied by the board read via Payload[\"verify_table\"]",
	},
	{
		ID:      "isolation-own-private-tmp-worktree",
		Anchors: []string{"isolation", "worktree", "/private/tmp"},
		How:     carriedInPrompt,
		Probes:  []string{"Isolation (MANDATORY)", "/private/tmp"},
		Note:    "assertNoSharedCheckout refuses to emit a prompt naming a shared checkout",
	},
	{
		ID:      "run-every-verify-row-observed-output",
		Anchors: []string{"every verify row", "exit code"},
		How:     carriedInPrompt,
		Probes:  []string{"run EVERY row", "exit code", "unrun"},
	},
	{
		ID:      "name-and-derive-risk-bearing-value",
		Anchors: []string{"name-and-derive", "risk-bearing value"},
		How:     notYetCarried,
		Probes:  []string{"risk-bearing value"},
		Issue:   1267,
		Note: "Residual gap. Deliberately NOT copied yet: the canonical wording is being " +
			"rewritten upstream, and copying today's prose would re-create the drift this " +
			"registry exists to catch. Land it with the merged wording, then flip to " +
			"carriedInPrompt. Reaching it needs the item's risk class in the prompt path " +
			"(Item.Risk is already typed).",
	},
	{
		ID:      "verifier-is-not-the-implementer",
		Anchors: []string{"brief's implementer", "fresh agent"},
		How:     carriedInPrompt,
		Probes:  []string{"NOT this brief's implementer"},
		Note:    "the engine's author!=runner guard refuses structurally; the prompt restates it",
	},
	{
		ID:      "structured-result-evidence-rows-and-verdict",
		Anchors: []string{"evidence rows", "verify: pass"},
		How:     carriedInPrompt,
		Probes:  []string{"STRUCTURED result", "PASS|FAIL|BLOCKED"},
		Note:    "free text never reaches the board: Land consumes only the typed Result",
	},
	{
		ID:      "tier-local-session-model-never-opus",
		Anchors: []string{"local session model", "never opus"},
		How:     carriedInPrompt,
		Probes:  []string{"local session model — never opus/external"},
	},
	{
		ID:      "risk-keyed-floor-upper-rung-is-human",
		Anchors: []string{"risk-keyed floor", "upper rung"},
		How:     carriedByEngine,
		Note: "TierPolicy (tier.go) routes risk-flagged items to TierHuman, which the engine " +
			"never dispatches — a routing decision, not prompt prose",
	},
}

// renderDispatchPrompt builds the per-item verifier prompt from the fixed template + the
// item's typed payload. This template is the ONLY per-loop-authored prose (arch doc §4).
//
// What it MUST carry is NOT re-listed here — a second prose copy of the SKILL.md list is
// exactly what went stale. The enumeration is dispatchRequirements above, checked
// against the skill file by TestDispatchRequirements_MatchSkillMD. Change the prompt below
// and you change the registry with it, or the test says so.
func renderDispatchPrompt(it loopengine.Item, tier loopengine.Tier) string {
	verifyTable := it.Payload["verify_table"]
	if strings.TrimSpace(verifyTable) == "" {
		verifyTable = "(read the brief's ## Verify table and run every row)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "VERIFY %s (tier=%s, local session model — never opus/external)\n\n", it.ID, tier)
	fmt.Fprintf(&b, "Brief: %s\n", it.BriefPath)
	fmt.Fprintf(&b, "Target SHA (merged main to run against): %s\n\n", it.TargetSHA)
	b.WriteString("Isolation (MANDATORY):\n")
	b.WriteString("- Create and work in your OWN temporary worktree under /private/tmp off the target SHA.\n")
	b.WriteString("- Never touch a shared checkout; never `git restore`/`git clean`; path-specific `git add` only.\n\n")
	b.WriteString("Verify table — run EVERY row, record command -> exit code -> key observed output line\n")
	b.WriteString("(real output, never a claim; a row that cannot run is recorded as explicitly unrun):\n")
	b.WriteString(verifyTable)
	b.WriteString("\n\n")
	b.WriteString("You are NOT this brief's implementer (verifier != author). Fresh agent only.\n")
	b.WriteString("Report back a STRUCTURED result: one Evidence row per Verify row (command, exit, output),\n")
	b.WriteString("a clear verdict PASS|FAIL|BLOCKED, and your runner identity. Free-text verdicts are not accepted.\n")
	return b.String()
}

// assertNoSharedCheckout is the in-code backstop: the emitted dispatch instruction must
// never name a shared-checkout path. If the rendered prompt or any payload value contains a
// shared-checkout marker, dispatch refuses (fail closed) rather than emit a leaky instruction.
func assertNoSharedCheckout(prompt string) error {
	// Shared-checkout markers: the canonical worktrees-parent and the main checkout name.
	for _, bad := range []string{
		"/tracker/.claude/worktrees",
		"worktrees/thedesk",
	} {
		if strings.Contains(prompt, bad) {
			return fmt.Errorf("dispatch prompt names a shared-checkout path %q (isolation violation) — refusing", bad)
		}
	}
	return nil
}
