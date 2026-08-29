// Structural integration test: verifies verifyloop's dispatchRequirements stay
// in sync with the verifier dispatch KIT,
// tools/desk/cmd/deskdispatch/references/verifier-prompt.md.
//
// WHERE THE REQUIREMENT LIST LIVES, AND WHY IT MOVED. This test used to parse
// plugins/assay/skills/verify-desk/SKILL.md's "Its prompt MUST carry:" list. That list is
// gone: the skill was restructured so the dispatch prompt is no longer prose written in the
// skill at all. The skill now says so in as many words — "The prompt is a KIT, not prose
// written here" — and names its declared source: `deskdispatch --kit verifier` emits the
// common-clauses kit plus the verifier kit VERBATIM, from
// tools/desk/cmd/deskdispatch/references/. So the single statement of what a dispatched
// verifier must be told is the kit, and this test is re-pointed at it rather than deleted:
// the coupling is the point, only its far end moved.
//
// Two consequences worth stating. (1) The source now lives in THIS module, not in a
// documentation tree, so the upward walk terminates inside the checkout that is being
// tested and the "skill missing → silently skip" hole closes — the kit is embedded into the
// deskdispatch binary by //go:embed, so a tree that builds deskdispatch necessarily carries
// it. (2) The clause set the kit states is WIDER than the old bullet list, so the registry
// below is wider too: every clause of the kit is enumerated, and each one records how (or
// whether) verifyloop's own template carries it. Narrowing the source back to a single
// section would silently drop requirements from the gate.
//
// The `consumer` build tag stays DROPPED and this file runs in ordinary CI. The path filter
// that triggers it and the cross-module reader registry row that pins it live in
// tools/desk/internal/deskkit/citrigger_test.go (and .github/workflows/tools.yml).

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// The dispatched-verifier prompt has TWO producers of ONE clause set: the verifier kit that
// `deskdispatch --kit verifier` emits verbatim (the manual desk's path, and the wording the
// house treats as canonical) and the fixed template in dispatch.go that the loop engine emits
// once verifyloop becomes the driver. Nothing kept them in sync, so the canonical text gained
// the name-and-derive step and the Go template silently did not.
//
// These tests are that missing coupling: the kit is parsed at test time and matched against
// the dispatchRequirements registry in BOTH directions.
//
// Bound: enforcement is clause-granular. A requirement ADDED, REMOVED or RENAMED in the kit
// goes red; wording drift strictly INSIDE one clause does not.
//
// This IS a live CI gate in this tree, and it is on the RELEASE path:
// .github/workflows/release.yml runs `cd tools/desk && go test ./... && go vet ./...` as one
// leg of its test matrix, so a divergence here blocks a release rather than merely annoying a
// developer — which is exactly what happened when the requirement list moved and this test
// fataled on its own re-point instruction. .github/workflows/ci.yml additionally builds and
// vets every module on every push; `go vet` compiles test files, so it catches compile
// breakage here even though it does not run the test.

const kitRel = "tools/desk/cmd/deskdispatch/references/verifier-prompt.md"

// kitDirRel is the kit's home. It exists iff the dispatch tool ships in this tree, and it is
// what separates "this checkout does not carry deskdispatch" (skip) from "the kit was
// deleted" (fail closed).
const kitDirRel = "tools/desk/cmd/deskdispatch"

// kitClauseAnchor is the kit's own statement of what it is. Everything below it is clause
// text. It occurs exactly once, on ONE line, and it names the thing the registry couples to —
// a re-word that loses it is exactly the restructuring this test must notice.
const kitClauseAnchor = "load-bearing clauses every dispatched VERIFIER agent receives"

// The kit is markdown with `## ` clause sections. A section that carries top-level bullets
// states one requirement per bullet; a section that carries none (a verbatim quoted block, a
// rule written as prose) states ONE requirement, the section itself. Fenced blocks are inert.
var (
	kitBulletMarker  = regexp.MustCompile("^ {0,3}- ")
	kitSectionHeader = regexp.MustCompile("^## ")
	kitFenceMarker   = regexp.MustCompile("^\\s*```")
)

// repoRoot walks up from the package directory to the checkout root (the directory holding
// the verifier dispatch kit). Not finding it is a failure, not a skip: a silent skip is how a
// coupling test stops coupling anything.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, kitRel)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find %s walking up from the package dir — run these tests from inside the repo checkout", kitRel)
		}
		dir = parent
	}
}

// skipIfDispatchKitAbsent skips this coupling test when the dispatch tool is not in the
// checkout AT ALL — a sliced file set that ships verifyloop without deskdispatch has no kit
// and nothing to couple dispatchRequirements to.
//
// Fail-closed intent is preserved precisely: if the deskdispatch package IS present somewhere
// up the walk but the verifier kit is missing (a real deletion), this does NOT skip — it
// returns and lets repoRoot's Fatal fire. In practice the skip is unreachable in any tree that
// compiles deskdispatch, because kits.go embeds references/*.md into the binary.
func skipIfDispatchKitAbsent(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, kitRel)); err == nil {
			return // kit present — run the test
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(kitDirRel))); err == nil {
			return // dispatch tool present but its kit missing — let repoRoot fail closed
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skipf("%s not present in this checkout", kitRel)
		}
		dir = parent
	}
}

// kitPromptClauses returns one entry per clause of the verifier kit: one per top-level bullet
// in a section that has bullets, and one for the whole section where it has none. Wrapped
// lines are folded and whitespace normalised, so a re-wrap is not a change.
func kitPromptClauses(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, kitRel))
	if err != nil {
		t.Fatalf("read %s: %v", kitRel, err)
	}
	lines := strings.Split(string(raw), "\n")

	start := -1
	for i, ln := range lines {
		if strings.Contains(ln, kitClauseAnchor) {
			start = i + 1
			break
		}
	}
	if start == -1 {
		t.Fatalf("%s: could not find the %q line — the kit was restructured; "+
			"re-point this test (and reconcile dispatchRequirements) rather than deleting it",
			kitRel, kitClauseAnchor)
	}

	var (
		clauses          []string
		bullet           strings.Builder // the bullet currently being folded
		section          strings.Builder // the whole section, used only if it has no bullets
		inSection        bool
		sectionHasBullet bool
		bulletOpen       bool
		inFence          bool
	)
	fold := func(b *strings.Builder) string { return strings.Join(strings.Fields(b.String()), " ") }
	flushBullet := func() {
		if s := fold(&bullet); s != "" {
			clauses = append(clauses, s)
		}
		bullet.Reset()
		bulletOpen = false
	}
	flushSection := func() {
		flushBullet()
		if inSection && !sectionHasBullet {
			if s := fold(&section); s != "" {
				clauses = append(clauses, s)
			}
		}
		section.Reset()
		sectionHasBullet = false
	}

	for _, ln := range lines[start:] {
		if kitFenceMarker.MatchString(ln) {
			inFence = !inFence
			section.WriteString(" ")
			section.WriteString(ln)
			continue
		}
		switch {
		case !inFence && kitSectionHeader.MatchString(ln):
			flushSection()
			inSection = true
			section.WriteString(strings.TrimPrefix(strings.TrimSpace(ln), "## "))
		case !inSection:
			// kit preamble above the first clause section — not a clause
		case !inFence && kitBulletMarker.MatchString(ln):
			flushBullet()
			sectionHasBullet = true
			bulletOpen = true
			bullet.WriteString(strings.TrimPrefix(strings.TrimSpace(ln), "- "))
			section.WriteString(" ")
			section.WriteString(ln)
		default:
			if bulletOpen && !inFence && strings.TrimSpace(ln) != "" {
				bullet.WriteString(" ")
				bullet.WriteString(strings.TrimSpace(ln))
			}
			section.WriteString(" ")
			section.WriteString(ln)
		}
	}
	flushSection()

	if len(clauses) < 4 {
		t.Fatalf("%s: parsed only %d prompt-requirement clauses — the parser lost the kit; fix the parser, "+
			"do not lower this bar", kitRel, len(clauses))
	}
	return clauses
}

func matchesAnchors(bullet string, anchors []string) bool {
	low := strings.ToLower(bullet)
	for _, a := range anchors {
		if !strings.Contains(low, strings.ToLower(a)) {
			return false
		}
	}
	return true
}

// excerpt trims a bullet down to something readable in a failure message.
func excerpt(b string) string {
	if len(b) > 220 {
		return b[:220] + "…"
	}
	return b
}

// TestDispatchRequirements_MatchSkillMD is the divergence gate: every kit prompt
// requirement has a registry entry, and every registry entry still matches a kit clause.
//
// (The name is kept as-is: it is the check whose failure the release gate and the tracking
// issue name. What it reads moved from the skill to the kit the skill declares as the prompt's
// source; the gate it enforces did not move.)
//
// The binding is 1:1 IN BOTH DIRECTIONS, and that is load-bearing rather than tidy. An entry
// allowed to match several clauses can absorb a clause that has no entry of its own, which
// silences the reverse-direction orphan check — the exact defect state (the kit carries
// a requirement, the registry does not) then goes GREEN. That is not hypothetical: with anchors
// {"verify table", "sha"}, `brief-verify-table-and-target-sha` matched BOTH the brief-path
// clause and the name-and-derive clause once the latter was rewritten to contain the words
// "Verify table" and `git show <sha> --stat`, so deleting the name-and-derive entry outright
// left this test passing.
//
// So: 0 matches is red (as before), >1 matches is red, and a clause claimed by more than one
// entry is red. Anchors are kept multi-token and specific for the same reason — a single common
// token is one unrelated edit away from colliding.
func TestDispatchRequirements_MatchSkillMD(t *testing.T) {
	skipIfDispatchKitAbsent(t)

	bullets := kitPromptClauses(t)

	// claimants[i] = IDs of every registry entry matching clause i.
	claimants := make([][]string, len(bullets))
	for _, req := range dispatchRequirements {
		if len(req.Anchors) == 0 {
			t.Errorf("requirement %q declares no Anchors — it cannot be matched against the kit", req.ID)
			continue
		}
		var hits []int
		for i, b := range bullets {
			if matchesAnchors(b, req.Anchors) {
				hits = append(hits, i)
				claimants[i] = append(claimants[i], req.ID)
			}
		}
		switch {
		case len(hits) == 0:
			t.Errorf("requirement %q (anchors %v) matches NO clause of the verifier kit %s.\n"+
				"Either the kit dropped the requirement (delete the entry) or reworded it past its anchors "+
				"(re-anchor the entry AND check the prompt still says the same thing).",
				req.ID, req.Anchors, kitRel)
		case len(hits) > 1:
			var got []string
			for _, i := range hits {
				got = append(got, excerpt(bullets[i]))
			}
			t.Errorf("requirement %q (anchors %v) matches %d clauses of %s, want exactly 1:\n  - %s\n"+
				"Over-matching is not harmless: a multi-clause entry marks a clause as covered that has no "+
				"entry of its own, which silences the orphan check below and lets a genuine divergence "+
				"pass. Tighten the anchors so they identify ONE clause.",
				req.ID, req.Anchors, len(hits), kitRel, strings.Join(got, "\n  - "))
		}
	}

	for i, b := range bullets {
		switch {
		case len(claimants[i]) == 0:
			t.Errorf("%s carries a dispatch-prompt requirement with NO counterpart in dispatchRequirements:\n  %s\n"+
				"Add a dispatchRequirement for it. If the prompt should carry it, carry it (carriedInPrompt + Probes); "+
				"if the engine enforces it structurally, say where (carriedByEngine); if it is a tracked gap, "+
				"record the issue (notYetCarried + Issue). This is the divergence class this gate exists to catch.",
				kitRel, excerpt(b))
		case len(claimants[i]) > 1:
			t.Errorf("%s clause is claimed by %d registry entries (%s), want exactly 1:\n  %s\n"+
				"Two entries cannot both own one requirement — re-anchor them so each identifies its own clause.",
				kitRel, len(claimants[i]), strings.Join(claimants[i], ", "), excerpt(b))
		}
	}
}

// TestDispatchRequirements_CarriageHoldsInRenderedPrompt checks each registry entry against
// the prompt the engine actually emits — carriedInPrompt probes present, notYetCarried probes
// absent (so landing the prose forces the registry update).
func TestDispatchRequirements_CarriageHoldsInRenderedPrompt(t *testing.T) {
	it := loopengine.Item{
		ID:        "fixture/01",
		BriefPath: "docs/streams/fixture/brief-01-x.md",
		TargetSHA: "deadbeef",
		Payload:   map[string]string{"verify_table": "| 1 | `go test ./...` | exit 0 |"},
	}
	prompt := renderDispatchPrompt(it, loopengine.TierLocal)

	seen := map[string]bool{}
	for _, req := range dispatchRequirements {
		if seen[req.ID] {
			t.Errorf("duplicate requirement ID %q", req.ID)
		}
		seen[req.ID] = true

		switch req.How {
		case carriedInPrompt:
			if len(req.Probes) == 0 {
				t.Errorf("requirement %q is carriedInPrompt but declares no Probes — nothing is checked", req.ID)
			}
			for _, p := range req.Probes {
				if !strings.Contains(prompt, p) {
					t.Errorf("requirement %q is declared carriedInPrompt but the rendered prompt does not contain %q.\n"+
						"Either the template lost the requirement (put it back) or the probe is stale (re-probe).", req.ID, p)
				}
			}
		case notYetCarried:
			if req.Issue == 0 {
				t.Errorf("requirement %q is notYetCarried without a tracking Issue — an untracked gap is a silent gap", req.ID)
			}
			for _, p := range req.Probes {
				if strings.Contains(prompt, p) {
					t.Errorf("requirement %q is recorded notYetCarried but the rendered prompt now contains %q.\n"+
						"The gap was closed: flip How to carriedInPrompt (and close issue #%d).", req.ID, p, req.Issue)
				}
			}
		case carriedByEngine:
			if req.Note == "" {
				t.Errorf("requirement %q is carriedByEngine but does not say WHERE the engine enforces it", req.ID)
			}
		default:
			t.Errorf("requirement %q has an unknown carriage %d", req.ID, req.How)
		}
	}
}

// TestDispatchPrompt_TierIsTheLocalSessionModel pins the ONE dispatch requirement that has no
// counterpart clause in the verifier kit.
//
// The tier rule — dispatch on the local session model, never a paid/external or larger tier —
// is a ROUTING decision this loop makes (TierPolicy, tier.go), not something the kit tells a
// dispatched verifier, so it cannot be an entry in a registry that is 1:1 with kit clauses.
// It used to ride along as the `tier-local-session-model-never-opus` registry entry. Losing
// that entry when the registry was re-pointed at the kit would have quietly removed the only
// check that the rendered prompt still states the tier, so the probe moves here rather than
// being dropped: the coupling changed shape, the control did not.
func TestDispatchPrompt_TierIsTheLocalSessionModel(t *testing.T) {
	it := loopengine.Item{ID: "fixture/03", BriefPath: "docs/streams/fixture/brief-03-x.md", TargetSHA: "f00d"}
	const probe = "local session model — never opus/external"
	for _, tier := range []loopengine.Tier{loopengine.TierLocal, loopengine.TierSession} {
		if got := renderDispatchPrompt(it, tier); !strings.Contains(got, probe) {
			t.Errorf("rendered prompt at tier %s does not state %q.\n"+
				"The engine dispatches the local session model only (TierPolicy, tier.go) and the prompt "+
				"has to say so; a bigger tier is not this loop's to reach for.", tier, probe)
		}
	}
}

// TestDispatchPrompt_NoSharedCheckoutMarker keeps the shared-checkout backstop wired to the real render.
func TestDispatchPrompt_NoSharedCheckoutMarker(t *testing.T) {
	it := loopengine.Item{ID: "fixture/02", BriefPath: "docs/streams/fixture/brief-02-x.md", TargetSHA: "cafe"}
	if err := assertNoSharedCheckout(renderDispatchPrompt(it, loopengine.TierLocal)); err != nil {
		t.Fatalf("rendered prompt trips the shared-checkout backstop: %v", err)
	}
}
