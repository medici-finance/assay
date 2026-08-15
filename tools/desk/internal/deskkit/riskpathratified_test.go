package deskkit

// C8, the ratified condition.
//
// The C1–C12 disposition table was ratified by the repository owner; the dated
// ratification record (the issue and the review-comment IDs) lives in the private
// project tracker. C8 was
// ratified NOT as a bare "repo/org variable" but with a condition attached, and
// the ratification says so in terms:
//
//	C8 riskPathTriggers — repo/org variable, ADDITIVE-ONLY
//	(ASSAY_RISK_PATH_TRIGGERS_EXTRA). No knob may narrow the set, and the four
//	fail-closed preconditions stay non-configurable. This is the security gate;
//	additive-only is the condition of the ratification, not a detail.
//
// riskpathconfig_test.go already pins the additive-only half against three
// benign values. The two tests below exist because the condition is now a term
// of the ratification rather than an internal design preference, and a term of a
// ratification should fail something when it stops holding:
//
//   - the first sweeps VALUE SHAPES an admin or an implementer would reach for
//     in order to subtract — negation prefixes, wildcards, sentinels, explicit
//     removal syntaxes. None of these is currently meaningful to the parser,
//     which is exactly the property under test: "there is no removal syntax" is
//     a claim about values that do NOT parse specially, so it can only be held
//     by trying them.
//
//   - the second pins the OTHER half of the condition, which no existing test
//     covers: the four fail-closed preconditions in RiskPathTriggered must not
//     become reachable-past through configuration.
//
// Neither test asserts anything about the C2 roster variable's own reach — see
// the note on TestRatifiedPreconditionsStayNonConfigurable.

import "testing"

// ratifiedCompiledFloor is every compiled trigger that applies to this repo:
// the universal base list plus the assay cell's own additions. The union of the
// two IS the floor the ratification puts under adopter configuration.
// cleanControlPath is a path no compiled trigger covers AND that no widening
// value in these tables can reach: it is three segments deep, so a bare "*"
// (a legal single-segment wildcard) does not match it. Every positive control
// below uses it, so a control failure means a genuine inversion rather than a
// value that merely widened.
const cleanControlPath = "docs/notes/scratch.md"

func ratifiedCompiledFloor() []string {
	out := append([]string{}, baseRiskPathTriggers...)
	return append(out, repoRiskPathTriggersFor("medici-finance/assay")...)
}

// TestRatifiedNoRemovalSyntaxNarrowsTheSet is the "no knob may narrow the set"
// half. It sweeps the value shapes that would express subtraction if any of them
// were meaningful, and asserts the compiled floor survives every one — both in
// the reported set and, more importantly, in what actually classifies.
func TestRatifiedNoRemovalSyntaxNarrowsTheSet(t *testing.T) {
	repo := "medici-finance/assay"

	// Paths covered by the compiled floor. Each must stay risk-classed no matter
	// what the adopter variable says.
	compiledHits := []string{
		"secrets/prod/token.yaml",
		".github/workflows/release.yml",
		"k8s/dev/rbac.yaml",
		"k8s/prod/rbac.yaml",
		"tools/desk/internal/deskkit/riskpath.go",
		"tools/desk/cmd/deskpost/main.go",
		"tools/desk/cmd/deskboard/board.go",
		"tools/desk/cmd/desktoken/main.go",
		"tools/desk/cmd/deskpushguard/main.go",
		"tools/desk/cmd/writeguard/main.go",
	}

	// Shapes that express, or try to express, REMOVAL. If the parser ever grows
	// a subtraction syntax, one of these is what it will look like.
	subtractionShapes := []string{
		"-secrets/",
		"!secrets/",
		"^secrets/",
		"~secrets/",
		"secrets/-",
		"secrets/=",
		"secrets/:remove",
		"remove:secrets/",
		"-tools/desk/internal/deskkit/",
		"-secrets/,-k8s/*/rbac.yaml,-tools/desk/internal/deskkit/",
		"NONE",
		"none",
		"null",
		"[]",
		"",
		"   ",
		"*",
		"*/*",
		".",
		".*",
		"-*",
	}

	for _, extra := range subtractionShapes {
		t.Run(extra, func(t *testing.T) {
			r := goldenRoster()
			r[EnvAllowedRepos] = repo + ":ci:private"
			r[EnvRiskPathTriggersExtra] = extra
			withRoster(t, r)

			// (a) the floor is still reported in the effective set — by BOTH
			// accessors. RiskPathTriggersFor is the per-repo view; RiskPathTriggers
			// is the universal one, and its doc comment makes the same promise
			// ("Nothing narrows the compiled list: there is no removal syntax").
			// It has no non-test caller today, so only this assertion stands
			// between that promise and a silent regression: a mutation letting a
			// configured value REPLACE the base list in RiskPathTriggers alone
			// left the whole suite green before this row was added.
			for _, acc := range []struct {
				name string
				got  []string
				// floor is per-accessor: RiskPathTriggersFor carries the repo's
				// compiled additions too, RiskPathTriggers is the universal set.
				floor []string
			}{
				{"RiskPathTriggersFor(" + repo + ")", RiskPathTriggersFor(repo), ratifiedCompiledFloor()},
				{"RiskPathTriggers()", RiskPathTriggers(), baseRiskPathTriggers},
			} {
				for _, want := range acc.floor {
					found := false
					for _, g := range acc.got {
						if g == want {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("NARROWED: %s=%q dropped compiled trigger %q from %s (%v). "+
							"C8's ratification is conditional on additive-only semantics; a value that "+
							"subtracts a compiled entry breaks the term the owner ratified.",
							EnvRiskPathTriggersExtra, extra, want, acc.name, acc.got)
					}
				}
			}

			// (b) and, the part that matters, the floor still CLASSIFIES. A
			// trigger present in a reported list but not consulted by the gate
			// would satisfy (a) while waiving the gate.
			for _, f := range compiledHits {
				if !RiskPathTriggered(repo, []string{f}) {
					t.Errorf("NARROWED: %s=%q made %q stop being risk-classed",
						EnvRiskPathTriggersExtra, extra, f)
				}
			}

			// (c) positive control — a clean file is still NOT classed, so the
			// assertions above are not being satisfied by a blanket true.
			//
			// The control path is deliberately MULTI-SEGMENT. A bare "*" is a
			// legal single-segment wildcard, so EXTRA="*" genuinely classifies
			// every top-level file — that is WIDENING, the safe direction, and
			// a control at "README.md" would misreport it as a defect.
			if RiskPathTriggered(repo, []string{cleanControlPath}) {
				t.Errorf("positive control inverted with %s=%q: a clean file was risk-classed, "+
					"so (b) proves nothing", EnvRiskPathTriggersExtra, extra)
			}
		})
	}
}

// TestRatifiedPreconditionsStayNonConfigurable is the second half of the
// condition: "the four fail-closed preconditions stay non-configurable". The
// four, in the order RiskPathTriggered decides them, are
//
//  1. a repo outside the allowed set,
//  2. a repo that is public or of unstated visibility,
//  3. an empty changed-file list,
//  4. a blank path entry,
//
// each of which returns true before, or instead of, consulting the trigger
// paths. This test drives all four under hostile roster states and asserts none
// of them can be reached past.
//
// SCOPE, stated because the boundary is easy to blur. This pins the four
// PREDICATES against configuration. It does not, and should not, assert that
// their INPUTS are unconfigurable: preconditions 1 and 2 read the repo set and
// the visibility marker from ASSAY_ALLOWED_REPOS, which is C2's separately
// ratified surface. That transitive reach is a real property of the design and
// is recorded on the PR for the owner; it is not something this test can decide.
func TestRatifiedPreconditionsStayNonConfigurable(t *testing.T) {
	hostile := []struct {
		name  string
		extra map[string]string
	}{
		{"nothing set beyond the base roster", nil},
		{"extra tries to subtract everything", map[string]string{EnvRiskPathTriggersExtra: "-*"}},
		{"extra is a wildcard, trust widened", map[string]string{
			EnvRiskPathTriggersExtra: "*",
			EnvTrustedLogins:         "mallory:1",
			EnvBlessLogin:            "mallory:1",
		}},
		{"extra is a sentinel", map[string]string{EnvRiskPathTriggersExtra: "NONE"}},
		{"extra empty", map[string]string{EnvRiskPathTriggersExtra: ""}},
		{"extra is a match-everything regex-looking value", map[string]string{EnvRiskPathTriggersExtra: ".*"}},
	}

	for _, h := range hostile {
		t.Run(h.name, func(t *testing.T) {
			roster := func(repos string) map[string]string {
				r := goldenRoster()
				if repos != "" {
					r[EnvAllowedRepos] = repos
				}
				for k, v := range h.extra {
					r[k] = v
				}
				return r
			}

			// (1) repo outside the allowed set — no policy is never a waiver.
			withRoster(t, roster("one/private-repo:ci:private"))
			if !RiskPathTriggered("acme/widget", []string{cleanControlPath}) {
				t.Error("PRECONDITION 1 DEFEATED: a repo outside the allowed set was not risk-classed")
			}

			// (2) public repo — the public-repo risk rule's unconditional classing.
			withRoster(t, roster("acme/widget:ci:public"))
			if !RiskPathTriggered("acme/widget", []string{cleanControlPath}) {
				t.Error("PRECONDITION 2 DEFEATED: a PUBLIC repo was not risk-classed")
			}

			// (2b) visibility unstated — the zero value fails closed.
			withRoster(t, roster("acme/widget:ci"))
			if !RiskPathTriggered("acme/widget", []string{cleanControlPath}) {
				t.Error("PRECONDITION 2b DEFEATED: an unstated-visibility repo was not risk-classed")
			}

			// (3) and (4) need a repo that clears 1 and 2, so the gate actually
			// reaches them rather than short-circuiting earlier.
			withRoster(t, roster("acme/widget:ci:private"))
			if !RiskPathTriggered("acme/widget", nil) {
				t.Error("PRECONDITION 3 DEFEATED: a nil changed-file list was not risk-classed")
			}
			if !RiskPathTriggered("acme/widget", []string{}) {
				t.Error("PRECONDITION 3 DEFEATED: an empty changed-file list was not risk-classed")
			}
			if !RiskPathTriggered("acme/widget", []string{""}) {
				t.Error("PRECONDITION 4 DEFEATED: an empty path entry was not risk-classed")
			}
			if !RiskPathTriggered("acme/widget", []string{"   "}) {
				t.Error("PRECONDITION 4 DEFEATED: a blank path entry was not risk-classed")
			}
			if !RiskPathTriggered("acme/widget", []string{cleanControlPath, ""}) {
				t.Error("PRECONDITION 4 DEFEATED: a blank entry among clean entries was not risk-classed")
			}

			// Positive control: this repo, with clean files and no blank entry,
			// is NOT classed — so the seven assertions above are discriminating.
			if RiskPathTriggered("acme/widget", []string{cleanControlPath}) {
				t.Error("positive control inverted: a private repo with a clean file was risk-classed, " +
					"so the precondition assertions prove nothing")
			}
		})
	}
}
