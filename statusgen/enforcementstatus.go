package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// enforcement-status (mistake-proofing/04) — derive the authoring guidance's
// enforcement-status claims from the lint itself.
//
// THE PROBLEM. The authoring guidance (plugins/assay/skills/author-brief/SKILL.md)
// tells authors, in its own prose, which of its rules the lint enforces and which
// are only conventions. Those claims were hand-typed and they drifted: the
// guidance stated that the `consumers:` field was enforced by "no lint" and told
// authors to leave entries deliberately non-conforming on that belief — while a
// shipped check (consumers.go) makes one class of `consumers:` claim a hard
// `--lint` PROBLEM. A guidance document that tells authors a live gate is
// decorative manufactures deliberate non-conformance. This is the documented
// error class the house has closed twice: declare the source, generate the copy,
// diff it in CI.
//
// THE SOURCE OF TRUTH — WHY A DECLARED REGISTRY, NOT A LINT RUN. The lint already
// carries a stable [rule-tag] bracket token on some emitted lines, and the
// firing-audit (lintaudit.go) reconstructs a rule set by SCRAPING a lint run. We
// deliberately do NOT reuse that as the source here, and the choice is the
// central design call of this brief:
//
//   - A lint run is a FLOOR, not a total. It can only surface a rule that FIRES
//     on the tree it runs against; a rule that is currently quiet emits no line
//     and is invisible. Worse, the third status this block must carry — "not
//     enforced", an authoring convention the lint checks at all — can NEVER be
//     produced by a run, because a rule that does not exist emits nothing to
//     scrape. Honesty about non-coverage is itself a device (spec §3 D6); a
//     source that structurally cannot represent "not enforced" defeats it.
//   - So the source is an EXPLICIT registry (lintRuleRegistry, below): the
//     declared total of the rules this guidance makes claims about, each with its
//     one-line description and its enforcement status from the three-value set.
//     The emitter renders it in a stable order; skillslint regenerates the copy
//     in the guidance and byte-diffs it in the CI job that already runs on pull
//     requests, so the copy cannot drift silently.
//
// VISIBILITY OF UNREGISTERED RULES. The cost of a declared registry is that a
// tagged lint rule could be ADDED without being registered here, and then read as
// silently absent. Two guards close that: (1) enforcementStatusRegistryCoversVerifyRowRules
// (see the test) asserts every stable rule-tag const the Verify-row lint declares
// appears in the registry, so a new tagged row that forgets to register reddens
// the suite; (2) the generated block's own header states the coverage boundary —
// that it reports what the lint enforces, not what the methodology requires, and
// that a rule enforced by a tool OUTSIDE the lint reads as `not enforced` here
// unless registered. An unregistered rule is therefore never presented as a pass:
// it is either caught by the test or covered by the stated boundary.

// EnforcementStatus is one of exactly three values. Three, not two: "not
// enforced" is a real state, and hiding it is the honesty failure the spec's
// non-coverage rule (D6) exists to prevent.
type EnforcementStatus string

const (
	// StatusFatal — the lint emits a PROBLEM for this, which makes `--lint` exit
	// non-zero. A violation cannot merge past the gate.
	StatusFatal EnforcementStatus = "fatal"
	// StatusAdvisory — the lint emits a NOTICE. It is printed and counted but
	// never moves the exit code; the gate stays green.
	StatusAdvisory EnforcementStatus = "advisory"
	// StatusNotEnforced — no lint rule checks this. It is an authoring convention
	// only, honoured by discipline, not by a gate.
	StatusNotEnforced EnforcementStatus = "not enforced"
)

// enforcementStatusValues is the exhaustive three-value set, used to validate the
// registry and to prove (in the test) that all three are representable.
var enforcementStatusValues = []EnforcementStatus{StatusFatal, StatusAdvisory, StatusNotEnforced}

func (s EnforcementStatus) valid() bool {
	for _, v := range enforcementStatusValues {
		if s == v {
			return true
		}
	}
	return false
}

// LintRule is one declared rule: its stable identity (the rule-tag), what it
// checks in one line, and its enforcement status.
type LintRule struct {
	Tag    string
	Checks string
	Status EnforcementStatus
}

// lintRuleRegistry is the DECLARED SINGLE SOURCE for the enforcement-status
// claims the authoring guidance makes. Every status here is a fact about the
// lint's code, verifiable at the cited site:
//
//   - The Verify-row family (verifyrows.go rules const block) is emitted through
//     unfailableRowNotices — every one is a NOTICE, so every one is `advisory`.
//   - The consumers: family (consumers.go consumersCheck): a `follow-up
//     <stream>/<NN>` whose target is not a brief in any stream README is `add`ed
//     to problems (consumers.go:381) — a `--lint` PROBLEM, so it is `fatal`. The
//     rest of the family is `notice`d — `advisory`.
//   - The flow Verify row (author-brief rule 6) is `not enforced`: the guidance
//     names it as a planned/advisory check, and consumersCheck does not verify
//     that a shared-value brief carries a cross-component flow row.
//
// Adding a rule here is the ONLY way it enters the generated block; the block is
// a pure function of this slice (see renderEnforcementBlock), which is what the
// EnforcementStatusTracksTheLint test proves.
var lintRuleRegistry = []LintRule{
	// Verify-row shape lint (verifyrows.go) — all advisory (unfailable notices).
	{ruleERELiteralPipe, "a `\\|` inside a `grep -E` pattern is a literal pipe, not alternation, so the row matches almost nothing and passes blind", StatusAdvisory},
	{ruleGrepZeroCount, "a `grep -c` whose pass bar is satisfied by a zero count measures nothing", StatusAdvisory},
	{ruleExitSwallowed, "a shell pipeline whose real exit status is sunk by a later stage, so the row cannot fail", StatusAdvisory},
	{ruleRE2LiteralPipe, "a `\\|` inside a `go test -run`/`-bench` selector is a literal pipe in RE2, not alternation", StatusAdvisory},
	{ruleMetavar, "an unsubstituted `<metavar>` placeholder left in the Command cell, so the row cannot be run as written", StatusAdvisory},
	{ruleGoRunExit, "a `go run` in the Command cell flattens the program's exit code, so a non-zero result reads as success", StatusAdvisory},
	{ruleBREAlternation, "a pipe in a basic-regex grep pattern (no `-E`/`-P`) is an ordinary character, so the pattern matches the Verify row itself", StatusAdvisory},
	{ruleShreddedCell, "a raw `|` in the Command cell is read as a table delimiter, truncating the command and shifting every later column", StatusAdvisory},
	{ruleMovingRef, "a diff base pinned to a moving ref (a branch name, not a SHA) makes the row's result drift under it", StatusAdvisory},
	{rulePortability, "a GNU-only shell construct that fails on the BSD/macOS userland a reviewer may run the row on", StatusAdvisory},

	// consumers: routed-consumer lint (consumers.go). One class is fatal.
	{"consumers-followup-missing-brief", "a `consumers: follow-up <stream>/<NN>` whose target is not a brief in any stream README — the routing claim is false", StatusFatal},
	{"consumers-unrouted", "a `consumers:` entry that names no routing token (`fixed-here` / `follow-up` / `out-of-scope`)", StatusAdvisory},
	{"consumers-followup-no-target", "a `follow-up` routing that names no `<stream>/<NN>` target — a deferral with no holder", StatusAdvisory},
	{"consumers-followup-one-way", "a `follow-up` target that exists but never references the deferring brief back — the coverage claim is one-way", StatusAdvisory},
	{"consumers-prose", "a `consumers:` written as a prose paragraph rather than a routed list, so nothing can corroborate it", StatusAdvisory},
	{"consumers-no-verify-row", "a brief carrying `consumers:` but no Verify row that runs `statusgen --consumers` to corroborate the routing", StatusAdvisory},
	{"consumers-missing-list", "a brief whose prose reads as changing a shared surface but enumerates no `consumers:` (a heuristic prompt, never a verdict)", StatusAdvisory},
	{"consumers-out-of-scope-no-reason", "an `out-of-scope` routing with no substantive reason for the reviewer who must weigh the exclusion", StatusAdvisory},

	// Authoring conventions the lint does NOT check — the third status, stated so
	// the block's non-coverage is itself visible (spec §3 D6).
	{"consumers-flow-verify-row", "that a shared-value brief's Verify table carries at least one row exercising the cross-component flow end-to-end — a judgement call no lint decides", StatusNotEnforced},
}

// renderEnforcementBlock renders the registry as the generated block that lives
// in the authoring guidance. It is a PURE function of rules: same input, same
// bytes. Rows are sorted by tag so an unrelated change never reorders the table
// (an unstable order turns every edit into a diff). An invalid status is a
// programming error in the registry and is returned as an error rather than
// emitted, so a malformed registry reddens rather than shipping a false claim.
func renderEnforcementBlock(rules []LintRule) (string, error) {
	seen := map[string]bool{}
	sorted := make([]LintRule, 0, len(rules))
	for _, r := range rules {
		if r.Tag == "" {
			return "", fmt.Errorf("enforcement-status: a registry rule has an empty tag")
		}
		if !r.Status.valid() {
			return "", fmt.Errorf("enforcement-status: rule %q has status %q, which is not one of fatal/advisory/not enforced", r.Tag, r.Status)
		}
		if seen[r.Tag] {
			return "", fmt.Errorf("enforcement-status: rule tag %q is declared twice", r.Tag)
		}
		seen[r.Tag] = true
		sorted = append(sorted, r)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Tag < sorted[j].Tag })

	var b strings.Builder
	// The FIRST line is the region anchor skillslint locates on, and it doubles as
	// the human's "this is generated" signal — no injected marker comment, matching
	// the shared-guardrail generator's first-line-anchor style.
	b.WriteString("## Enforcement status of these rules (generated — do not hand-edit)\n")
	b.WriteString("\n")
	b.WriteString("Regenerate with `statusgen enforcement-status` piped through the skillslint sync\n")
	b.WriteString("(`go run ./tools/skillslint --sync`); the byte-diff gate in the skillslint CI job fails\n")
	b.WriteString("if this block drifts from the lint's rule registry. Hand-edit it and the gate reddens.\n")
	b.WriteString("\n")
	b.WriteString("This table reports what `statusgen --lint` enforces, **not** what the methodology\n")
	b.WriteString("requires. A rule enforced only by a tool outside the lint — a CI workflow, a desk-side\n")
	b.WriteString("guard — reads as `not enforced` here unless it is registered. The three statuses are\n")
	b.WriteString("exact: **fatal** is a `--lint` PROBLEM that makes the run exit non-zero; **advisory** is\n")
	b.WriteString("a NOTICE that is printed but never gates; **not enforced** means no lint rule checks it —\n")
	b.WriteString("it is an authoring convention only.\n")
	b.WriteString("\n")
	b.WriteString("| rule | what it checks | status |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, r := range sorted {
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", r.Tag, r.Checks, r.Status))
	}
	// No trailing newline beyond the last row's: the region is compared line for
	// line, and a stray blank line at the tail would be a perpetual diff.
	return strings.TrimRight(b.String(), "\n"), nil
}

// runEnforcementStatus is the `statusgen enforcement-status` subcommand: it
// prints the generated block to stdout. It is a pure emitter — it reads no files
// and touches no tree, so it never belongs in `--lint` and needs no --root. The
// generator-and-diff half lives in skillslint (it owns the guidance copy and the
// CI byte-diff), which shells out to this emitter for the block's bytes.
func runEnforcementStatus(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "statusgen enforcement-status: unexpected argument %q — the emitter takes none (it renders the compiled-in registry)\n", args[0])
		return 2
	}
	block, err := renderEnforcementBlock(lintRuleRegistry)
	if err != nil {
		fmt.Fprintln(stderr, "statusgen enforcement-status:", err)
		return 2
	}
	fmt.Fprintln(stdout, block)
	return 0
}
