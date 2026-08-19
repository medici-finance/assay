// Structural integration test: verifies verifyloop's dispatchRequirements stay
// in sync with .claude/skills/verify-desk/SKILL.md's "Its prompt MUST carry:"
// list.
//
// The `consumer` build tag was DROPPED: verify-desk's
// SKILL.md moved into THIS repo as its single home, so the skill file is now at
// the assay checkout root and repoRoot's upward walk terminates here
// (before, the path existed only in a separate repo, so this test hid
// behind the tag and — run with the tag from an assay checkout — walked
// past the root to $HOME and read the ~/.claude thin pointer). This file now
// runs in ordinary CI; the .claude/** path filter that triggers it and the
// cross-module reader registry row that pins it live in
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

// The dispatched-verifier prompt has TWO consumers of ONE requirement list: the prose the
// manual desk follows (`.claude/skills/verify-desk/SKILL.md` loop step 2, "Its prompt MUST
// carry:") and the fixed template in dispatch.go that the loop engine emits once verifyloop
// becomes the driver. Nothing kept them in sync, so SKILL.md gained the name-and-derive
// step and the Go template silently did not.
//
// These tests are that missing coupling: the skill file is parsed at test time and matched
// against the dispatchRequirements registry in BOTH directions.
//
// Bound: enforcement is bullet-granular. A requirement ADDED, REMOVED or RENAMED in SKILL.md
// goes red; wording drift strictly INSIDE one bullet does not.
//
// This IS a live CI gate: scripts/go-check-workspace.sh runs `go test -count=1
// ./...` over every workspace module, /tools/desk among them (REQUIRE_MODULES), from the
// "Build, vet and unit-test every Go module in the workspace" step of the Checks workflow.
// The earlier caveat here — that no CI job ran tools/desk tests — was true when this
// file was written and is not true on the tree it merges into.

const skillRel = ".claude/skills/verify-desk/SKILL.md"

// skillBulletMarker matches a top-level bullet of the "Its prompt MUST carry:" list. The list
// sits inside numbered loop step 2, so its bullets are indented; continuation lines are
// indented further and are appended to the bullet they belong to.
var (
	skillBulletMarker = regexp.MustCompile(`^ {1,4}- `)
	numberedStep      = regexp.MustCompile(`^\d+\. `)
)

// repoRoot walks up from the package directory to the checkout root (the directory holding
// the verify-desk skill). Not finding it is a failure, not a skip: a silent skip is how a
// coupling test stops coupling anything.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, skillRel)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find %s walking up from the package dir — run these tests from inside the repo checkout", skillRel)
		}
		dir = parent
	}
}

// skipIfVerifyDeskSkillAbsent skips this coupling test when the verify-desk
// SKILL.md is not in the checkout AT ALL. This repository's published file set
// does not always carry a .claude/skills tree, so there may be no verify-desk
// skill and nothing to couple dispatchRequirements to.
//
// Fail-closed intent is preserved precisely: if a .claude/skills tree IS present
// somewhere up the walk but the specific verify-desk SKILL.md is missing (a real
// deletion), this does NOT skip — it returns and lets repoRoot's Fatal fire. It
// skips only when no .claude/skills tree exists at all.
func skipIfVerifyDeskSkillAbsent(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, skillRel)); err == nil {
			return // skill present — run the test
		}
		if _, err := os.Stat(filepath.Join(dir, ".claude", "skills")); err == nil {
			return // skills tree present but this skill missing — let repoRoot fail closed
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skipf("%s not present in this checkout", skillRel)
		}
		dir = parent
	}
}

// skillPromptBullets returns one entry per top-level bullet of SKILL.md loop step 2's
// "Its prompt MUST carry:" list, continuation lines folded in.
func skillPromptBullets(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, skillRel))
	if err != nil {
		t.Fatalf("read %s: %v", skillRel, err)
	}
	lines := strings.Split(string(raw), "\n")

	start := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Its prompt MUST carry:") {
			start = i + 1
			break
		}
	}
	if start == -1 {
		t.Fatalf("%s: could not find the 'Its prompt MUST carry:' list — the skill was restructured; "+
			"re-point this test (and reconcile dispatchRequirements) rather than deleting it", skillRel)
	}

	var bullets []string
	var cur strings.Builder
	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			bullets = append(bullets, strings.TrimSpace(cur.String()))
		}
		cur.Reset()
	}
	for _, ln := range lines[start:] {
		if numberedStep.MatchString(ln) { // next numbered loop step: list is over
			break
		}
		switch {
		case skillBulletMarker.MatchString(ln):
			flush()
			cur.WriteString(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), "- ")))
		case strings.TrimSpace(ln) == "":
			// blank line inside the list: keep folding, the list has no blank separators today
		default:
			cur.WriteString(" ")
			cur.WriteString(strings.TrimSpace(ln))
		}
	}
	flush()

	if len(bullets) < 4 {
		t.Fatalf("%s: parsed only %d prompt-requirement bullets — the parser lost the list; fix the parser, "+
			"do not lower this bar", skillRel, len(bullets))
	}
	return bullets
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

// TestDispatchRequirements_MatchSkillMD is the divergence gate: every SKILL.md prompt
// requirement has a registry entry, and every registry entry still matches a SKILL.md bullet.
//
// The binding is 1:1 IN BOTH DIRECTIONS, and that is load-bearing rather than tidy. An entry
// allowed to match several bullets can absorb a bullet that has no entry of its own, which
// silences the reverse-direction orphan check — the exact defect state (SKILL.md carries
// a requirement, the registry does not) then goes GREEN. That is not hypothetical: with anchors
// {"verify table", "sha"}, `brief-verify-table-and-target-sha` matched BOTH the brief-path
// bullet and the name-and-derive bullet once the latter was rewritten to contain the words
// "Verify table" and `git show <sha> --stat`, so deleting the name-and-derive entry outright
// left this test passing.
//
// So: 0 matches is red (as before), >1 matches is red, and a bullet claimed by more than one
// entry is red. Anchors are kept multi-token and specific for the same reason — a single common
// token is one unrelated edit away from colliding.
func TestDispatchRequirements_MatchSkillMD(t *testing.T) {
	skipIfVerifyDeskSkillAbsent(t)

	bullets := skillPromptBullets(t)

	// claimants[i] = IDs of every registry entry matching bullet i.
	claimants := make([][]string, len(bullets))
	for _, req := range dispatchRequirements {
		if len(req.Anchors) == 0 {
			t.Errorf("requirement %q declares no Anchors — it cannot be matched against SKILL.md", req.ID)
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
			t.Errorf("requirement %q (anchors %v) matches NO bullet of %s 'Its prompt MUST carry:'.\n"+
				"Either the skill dropped the requirement (delete the entry) or reworded it past its anchors "+
				"(re-anchor the entry AND check the prompt still says the same thing).",
				req.ID, req.Anchors, skillRel)
		case len(hits) > 1:
			var got []string
			for _, i := range hits {
				got = append(got, excerpt(bullets[i]))
			}
			t.Errorf("requirement %q (anchors %v) matches %d bullets of %s, want exactly 1:\n  - %s\n"+
				"Over-matching is not harmless: a multi-bullet entry marks a bullet as covered that has no "+
				"entry of its own, which silences the orphan check below and lets a genuine divergence "+
				"pass. Tighten the anchors so they identify ONE bullet.",
				req.ID, req.Anchors, len(hits), skillRel, strings.Join(got, "\n  - "))
		}
	}

	for i, b := range bullets {
		switch {
		case len(claimants[i]) == 0:
			t.Errorf("%s carries a dispatch-prompt requirement with NO counterpart in dispatchRequirements:\n  %s\n"+
				"Add a dispatchRequirement for it. If the prompt should carry it, carry it (carriedInPrompt + Probes); "+
				"if the engine enforces it structurally, say where (carriedByEngine); if it is a tracked gap, "+
				"record the issue (notYetCarried + Issue). This is the divergence class this gate exists to catch.",
				skillRel, excerpt(b))
		case len(claimants[i]) > 1:
			t.Errorf("%s bullet is claimed by %d registry entries (%s), want exactly 1:\n  %s\n"+
				"Two entries cannot both own one requirement — re-anchor them so each identifies its own bullet.",
				skillRel, len(claimants[i]), strings.Join(claimants[i], ", "), excerpt(b))
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

// TestDispatchPrompt_NoSharedCheckoutMarker keeps the shared-checkout backstop wired to the real render.
func TestDispatchPrompt_NoSharedCheckoutMarker(t *testing.T) {
	it := loopengine.Item{ID: "fixture/02", BriefPath: "docs/streams/fixture/brief-02-x.md", TargetSHA: "cafe"}
	if err := assertNoSharedCheckout(renderDispatchPrompt(it, loopengine.TierLocal)); err != nil {
		t.Fatalf("rendered prompt trips the shared-checkout backstop: %v", err)
	}
}
