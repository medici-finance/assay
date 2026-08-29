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
	// notYetCarried: a KNOWN, TRACKED gap — the dispatch kit carries the requirement, this
	// template does not yet. Issue is mandatory, and every Probe MUST be ABSENT from the
	// rendered prompt, so the day someone lands the prose the registry has to be updated with
	// it.
	notYetCarried
)

// dispatchRequirement maps ONE clause of the verifier dispatch kit,
// `tools/desk/cmd/deskdispatch/references/verifier-prompt.md`, onto this template. Anchors are
// the substrings that identify the kit clause; Probes are substrings of the rendered prompt.
type dispatchRequirement struct {
	ID      string   // stable slug, independent of either side's wording
	Anchors []string // ALL must appear (case-insensitively) in the kit clause this maps to
	How     carriage
	Probes  []string // must appear in the rendered prompt (carriedInPrompt) / must NOT (notYetCarried)
	Issue   int      // tracking issue; REQUIRED for notYetCarried
	Note    string
}

// dispatchRequirements is the ONE in-code enumeration of the verifier dispatch kit's clauses —
// it replaces the prose list that used to live in renderDispatchPrompt's doc-comment and that
// silently fell out of date (the canonical text gained the name-and-derive step, the template
// and its doc-comment did not).
//
// WHY THE KIT AND NOT THE SKILL. The requirement list used to be prose in verify-desk's
// SKILL.md ("Its prompt MUST carry:"). The skill was restructured and now states plainly that
// the prompt is a KIT rather than prose written there, naming
// tools/desk/cmd/deskdispatch/references/verifier-prompt.md (plus the common-clauses kit) as
// its declared source, emitted verbatim by `deskdispatch --kit verifier`. So the kit is the
// canonical statement, and this registry is bound to it. The kit's clause set is WIDER than
// the old bullet list, which is why this table is longer: every clause is enumerated, and each
// records how — or whether — THIS template carries it. Narrowing the source back to one
// section would drop requirements out of the gate silently.
//
// TestDispatchRequirements_MatchSkillMD binds this table to the kit in both directions: a kit
// clause with no entry here is red, an entry that no longer matches any clause is red, and a
// carriedInPrompt entry whose Probes are missing from the render is red. The enforcement is
// clause-granular: it catches a requirement ADDED, REMOVED or RENAMED in the kit, not wording
// drift strictly inside one clause.
//
// The match is 1:1 — each entry must claim EXACTLY one clause and each clause exactly one
// entry. Anchors are therefore multi-token and specific on purpose: a single common token is
// one unrelated kit edit away from claiming a second clause, and an entry that claims a second
// clause marks it covered, silencing the orphan check. See the test's doc comment for the live
// instance this bound was written from.
//
// One requirement is deliberately NOT in this table: the dispatch TIER (the local session
// model, never a paid/external or larger tier). It is a routing decision made by TierPolicy in
// tier.go, not a clause of the verifier kit, so it has no kit clause to be 1:1 with. Its
// carriage in the rendered prompt is pinned directly by
// TestDispatchPrompt_TierIsTheLocalSessionModel instead.
//
// Bound worth stating plainly: enforcement is clause-granular, so wording drift strictly
// inside one clause still passes.
var dispatchRequirements = []dispatchRequirement{
	{
		// Kit § "The common clauses come first".
		ID: "isolation-own-private-tmp-worktree",
		// The kit clause states the isolation floor by reference (the common-clauses kit) and
		// adds the verifier-specific part: a TEMPORARY worktree cut off origin/main at the
		// merged head, never the shared checkout. It does not name /private/tmp; that stays a
		// Probe only (the rendered prompt is stricter than the kit, which is fine).
		Anchors: []string{"isolation floor", "never the shared checkout"},
		How:     carriedInPrompt,
		Probes:  []string{"Isolation (MANDATORY)", "/private/tmp"},
		Note:    "assertNoSharedCheckout refuses to emit a prompt naming a shared checkout",
	},
	{
		// Kit § "Run every row", clause 1.
		ID: "brief-verify-table-and-target-sha",
		// "sha" alone collided with the name-and-derive clause once that clause gained both
		// "Verify table" and a `git show <sha>` example; "merged main sha" plus "item path" is
		// the phrasing unique to THIS clause. See the 1:1 rationale on
		// TestDispatchRequirements_MatchSkillMD.
		Anchors: []string{"item path", "verify table", "merged main sha"},
		How:     carriedInPrompt,
		Probes:  []string{"Brief: ", "Target SHA"},
		Note:    "verify table supplied by the board read via Payload[\"verify_table\"]",
	},
	{
		// Kit § "Run every row", clause 2.
		ID:      "run-every-verify-row-observed-output",
		Anchors: []string{"run every verify row", "exit code"},
		How:     carriedInPrompt,
		Probes:  []string{"run EVERY row", "exit code"},
	},
	{
		// Kit § "Run every row", clause 3. Split out of the entry above when the kit stated it
		// as its own clause: an unrun row is a distinct requirement from running every row, and
		// folding the two would let one of them vanish behind the other's anchors.
		ID:      "unrun-row-recorded-explicitly",
		Anchors: []string{"recorded as explicitly unrun", "never assumed to pass"},
		How:     carriedInPrompt,
		Probes:  []string{"explicitly unrun"},
	},
	{
		// Kit § "Run every row", clause 4.
		ID:      "verifier-is-not-the-implementer",
		Anchors: []string{"item's implementer", "fresh agent"},
		How:     carriedInPrompt,
		Probes:  []string{"NOT this brief's implementer"},
		Note:    "the engine's author!=runner guard refuses structurally; the prompt restates it",
	},
	{
		// Kit § "Run every row", clause 5.
		ID:      "structured-result-evidence-rows-and-verdict",
		Anchors: []string{"evidence rows", "verify: pass"},
		How:     carriedInPrompt,
		Probes:  []string{"STRUCTURED result", "PASS|FAIL|BLOCKED"},
		Note:    "free text never reaches the board: Land consumes only the typed Result",
	},
	{
		// Kit § "Evidence format", clause 1.
		ID:      "evidence-row-dated-and-runner-attributed",
		Anchors: []string{"never a bare check mark", "unattributed row"},
		How:     carriedByEngine,
		Note: "renderEvidence (land.go) builds every row from the typed Result and stamps the " +
			"date and the dispatched runner itself, so a bare check mark is not expressible; " +
			"the prompt asks for the runner identity, never for a hand-typed markdown row",
	},
	{
		// Kit § "Evidence format", clause 2.
		ID:      "unrun-reason-in-the-observed-cell",
		Anchors: []string{"could not run", "observed cell"},
		How:     carriedByEngine,
		Note: "renderEvidence (land.go) copies Result.Rows[i].Output into the observed cell " +
			"verbatim, so whatever the verifier said about an unrun row reaches the board " +
			"unedited; the asking half is the unrun clause carried in the prompt",
	},
	{
		// Kit § "Evidence format", clause 3.
		ID:      "plain-wording-not-read-as-unrun",
		Anchors: []string{"board tooling", "which filter excluded it"},
		How:     notYetCarried,
		Probes:  []string{"board tooling"},
		Issue:   187,
		Note: "Gap, recorded rather than assumed. The kit warns the verifier that the board's " +
			"own lint reads unrun-sounding phrasing in an Evidence cell as an unrun row; this " +
			"template says nothing about it and renderEvidence does not rewrite an Output " +
			"cell, so nothing carries it today. Surfaced by the kit reconciliation.",
	},
	{
		// Kit § "Evidence format", clause 4.
		ID:      "no-backticked-path-in-an-evidence-cell",
		Anchors: []string{"backticked file path", "link checker"},
		How:     notYetCarried,
		Probes:  []string{"link checker"},
		Issue:   187,
		Note: "Gap, recorded rather than assumed. A backticked path in an observed cell trips " +
			"the board's link checker; the verifier is the only party that can avoid writing " +
			"one, and this template does not tell it so. Surfaced by the kit reconciliation.",
	},
	{
		// Kit § "Risk-bearing value — ENUMERATE, then rank, then derive". The kit states this
		// one as a verbatim block to paste, not as bullets, so it is one clause.
		ID:      "name-and-derive-risk-bearing-value",
		Anchors: []string{"risk-bearing value", "enumerate"},
		How:     notYetCarried,
		Probes:  []string{"risk-bearing value"},
		Issue:   1267,
		Note: "Residual gap, and the reason this registry exists. The canonical wording has " +
			"now LANDED — it is the kit's ENUMERATE → rank → derive block, to be pasted " +
			"verbatim — so the old 'wait for the upstream rewrite' reason is spent; what is " +
			"left is the work: paste the block and flip to carriedInPrompt. Reaching it needs " +
			"the item's risk class in the prompt path (Item.Risk is already typed).",
	},
	{
		// Kit § "A FAIL is a result, not an interruption" — prose, one clause.
		ID:      "fail-does-not-advance-file-and-continue",
		Anchors: []string{"does not advance", "continue the drain"},
		How:     carriedByEngine,
		Note: "Land (land.go) routes VerdictFail to Evidence-without-flip plus FileBug and " +
			"returns — the item stays at implemented and the drain continues. The engine files " +
			"the change failure; it is not something the verifier is asked to remember",
	},
	{
		// Kit § "What a verifier may and may not flip", clause 1.
		ID:      "land-evidence-as-produced-never-a-wave",
		Anchors: []string{"land evidence as it is produced", "phantom verification debt"},
		How:     carriedByEngine,
		Note: "the engine lands per Result: Land (land.go) takes ONE Result and writes its " +
			"Evidence before returning, so there is no wave buffer for a pass to accumulate in",
	},
	{
		// Kit § "What a verifier may and may not flip", clause 2.
		ID:      "risk-keyed-floor-upper-rung-is-human",
		Anchors: []string{"risk-flagged item", "human gate"},
		How:     carriedByEngine,
		Note: "TierPolicy (tier.go) routes risk-flagged items to TierHuman, which the engine " +
			"never dispatches — a routing decision, not prompt prose",
	},
	{
		// Kit § "What a verifier may and may not flip", clause 3.
		ID:      "irreversible-evidence-written-status-left",
		Anchors: []string{"irreversible item", "never flips it"},
		How:     carriedByEngine,
		Note: "TierPolicy still dispatches an irreversible item so the Evidence is real, and " +
			"Land writes it with flip=false plus a human checkpoint (tier.go, land.go) — the " +
			"model path structurally cannot flip one",
	},
	{
		// Kit § "What a verifier may and may not flip", clause 4.
		ID:      "ci-owned-flips-are-watched-not-overridden",
		Anchors: []string{"ci owns a status flip", "refusal line"},
		How:     notYetCarried,
		Probes:  []string{"refusal line"},
		Issue:   187,
		Note: "Gap, recorded rather than assumed. Nothing in this template or the engine tells " +
			"a verifier to read a CI run's refusal line instead of flipping a stuck row by " +
			"hand. Surfaced by the kit reconciliation.",
	},
}

// renderDispatchPrompt builds the per-item verifier prompt from the fixed template + the
// item's typed payload. This template is the ONLY per-loop-authored prose (arch doc §4).
//
// What it MUST carry is NOT re-listed here — a second prose copy of the canonical list is
// exactly what went stale. The enumeration is dispatchRequirements above, checked against the
// verifier dispatch kit by TestDispatchRequirements_MatchSkillMD. Change the prompt below and
// you change the registry with it, or the test says so.
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
