package deskkit

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestIsAllowedRepo(t *testing.T) {
	cases := []struct {
		repo string
		want bool
	}{
		{"example-org/tracker", true},
		{"example-org/agents", true},
		{"example-org/examples", true},
		{"medici-finance/assay", true},
		{"example-org/example-k8s", true},
		{"example-org/example-reconciler", true},
		{"example-org/org-slides", true},
		{"example-org/proposals", true},
		{"example-org/platform", true}, // private, API-checked
		// One slides repo per product. slides itself is RETAINED above pending an
		// undecided call on whether it is redundant or focused on per-product material.
		{"example-org/demo-slides", true},
		{"example-org/assay-slides", true},
		{"example-org/example-reconciler-slides", true},
		{"example-org/console", true},
		// Negative paths: anything outside the configured set is refused. The
		// fixture roster (rosterfixture_test.go) carries NO owner/* pattern, so this
		// case list still exercises the plain explicit-only path — pattern admission
		// is exercised separately below with its own installed roster.
		{"example-org/some-other-repo", false},
		{"attacker/tracker", false},
		{"tracker", false},
		{"", false},
		{"EXAMPLE-ORG/TRACKER", false},        // case-sensitive
		{"medici-finance/assay-tools", false}, // merged into assay
		{"attacker/assay", false},
		{"attacker/example-k8s", false},
		{"other-org/example-k8s", false}, // right name, wrong org — set is example-org/example-k8s
		// These additions are example-org ONLY. Every one of these slugs is
		// checked against the API and 404s; none may be reachable by typo-adjacency
		// to a real entry.
		{"other-org/platform", false},
		{"other-org/demo-slides", false},
		{"other-org/assay-slides", false},
		{"other-org/example-reconciler-slides", false},
		{"other-org/slides", false},
		// The product name is not the repo name (the repo is example-reconciler-slides). A reader
		// reaching for the product name must not be silently authorised.
		{"example-org/product-slides", false},
		{"attacker/platform", false},
		{"example-org/platforms", false}, // near-miss, not a prefix match
		{"other-org/console", false},     // wrong org
		{"attacker/console", false},
		// example-org/unlisted-repo is not, and has never been, in this fixture's explicit
		// set or matched by any pattern — refused by plain absence, no dedicated
		// exclusion mechanism required (see TestPatternExplicitEntryAndPatternCoexist
		// for a repo refused ALONGSIDE an active owner/* pattern).
		{"example-org/unlisted-repo", false},
	}
	for _, c := range cases {
		t.Run(c.repo, func(t *testing.T) {
			if got := IsAllowedRepo(c.repo); got != c.want {
				t.Fatalf("IsAllowedRepo(%q) = %v, want %v", c.repo, got, c.want)
			}
		})
	}
}

func TestAllowedReposSortedAndComplete(t *testing.T) {
	want := []string{
		"example-org/agents",
		"example-org/assay-slides",
		"example-org/console",
		"example-org/demo-slides",
		"example-org/example-k8s",
		"example-org/example-reconciler",
		"example-org/example-reconciler-slides",
		"example-org/examples",
		"example-org/org-slides",
		"example-org/platform",
		"example-org/proposals",
		"example-org/tracker",
		"medici-finance/assay",
	}
	if got := AllowedRepos(); !reflect.DeepEqual(got, want) {
		t.Fatalf("AllowedRepos() = %v, want %v", got, want)
	}
}

// TestCIRequired — the drift anti-pattern: the CI policy is a column of the allowed-repo table, so it cannot
// drift from it. Every allowed repo answers; anything outside the set fails CLOSED.
func TestCIRequired(t *testing.T) {
	cases := map[string]bool{
		"example-org/tracker":            true,
		"example-org/agents":             true,
		"example-org/examples":           false, // no PR CI (builds on merge-to-main)
		"medici-finance/assay":           true,  // HAS .github/workflows
		"example-org/example-k8s":        true,  // active Validate workflow on PRs
		"example-org/example-reconciler": true,  // ci.yml, `on: pull_request: branches: [main]`; runs complete on PR heads
		"example-org/org-slides":         false, // render-slides.yml exists but is push-only (no pull_request trigger)
		"example-org/proposals":          false, // empty repo
		// Each value checked SEPARATELY against
		// `gh api repos/<repo>/contents/.github/workflows`, not copied across the group:
		// the platform repo runs PR CI, the three slides repos run none.
		"example-org/platform":                  true,  // lint.yml, `on: [pull_request]`
		"example-org/demo-slides":               false, // no .github/workflows — 404
		"example-org/assay-slides":              false, // no .github/workflows — 404
		"example-org/example-reconciler-slides": false, // no .github/workflows — 404
		"example-org/console":                   true,  // lint.yml, `on: [pull_request]`
		// Outside the set → CI-required (fail closed), never an implicit "no CI = green".
		"attacker/slides":            true,
		"example-org/random-repo":    true,
		"":                           true,
		"EXAMPLE-ORG/ORG-SLIDES":     true, // case-sensitive
		"other-org/demo-slides":      true, // wrong org — the slides repos are example-org only
		"example-org/product-slides": true, // product name, not the repo name (example-reconciler-slides)
		"attacker/platform":          true,
		"other-org/platform":         true, // wrong org (404 on the API)
		"other-org/console":          true, // wrong org
	}
	for _, r := range AllowedRepos() {
		if _, ok := cases[r]; !ok {
			t.Fatalf("allowed repo %q has no CIRequired expectation here — the CI policy and the "+
				"allowed set have drifted", r)
		}
	}
	for repo, want := range cases {
		t.Run(repo, func(t *testing.T) {
			if got := CIRequired(repo); got != want {
				t.Fatalf("CIRequired(%q) = %v, want %v", repo, got, want)
			}
		})
	}
}

func TestSplitRepo(t *testing.T) {
	cases := []struct {
		repo      string
		wantOwner string
		wantName  string
	}{
		{"example-org/tracker", "example-org", "tracker"},
		{"medici-finance/assay", "medici-finance", "assay"},
		{"owner/repo", "owner", "repo"},
		{"no-slash", "", ""},
		{"", "", ""},
		// More than one "/" is rejected outright. Tolerating it made IsAllowedRepo
		// answer TRUE for a path-traversal-shaped slug whose owner half reads
		// "medici-finance" and whose remainder names another org.
		{"too/many/slashes", "", ""},
		{"medici-finance/../example-org/x", "", ""},
		{"medici-finance/", "", ""},
		{"/assay", "", ""},
	}
	for _, c := range cases {
		t.Run(c.repo, func(t *testing.T) {
			owner, name := splitRepo(c.repo)
			if owner != c.wantOwner || name != c.wantName {
				t.Fatalf("splitRepo(%q) = (%q, %q), want (%q, %q)", c.repo, owner, name, c.wantOwner, c.wantName)
			}
		})
	}
}

// TestRepoVisibility — same drift-anti-pattern shape as TestCIRequired: visibility is a column of the
// allowed-repo table, every allowed repo must answer, and anything outside the set is
// VisibilityUnknown (which risk-classes). API-checked.
func TestRepoVisibility(t *testing.T) {
	cases := map[string]Visibility{
		"example-org/tracker":            VisibilityPrivate,
		"example-org/agents":             VisibilityPrivate,
		"example-org/examples":           VisibilityPrivate,
		"medici-finance/assay":           VisibilityPrivate,
		"example-org/example-k8s":        VisibilityPublic, // PUBLIC
		"example-org/example-reconciler": VisibilityPrivate,
		"example-org/org-slides":         VisibilityPrivate,
		"example-org/proposals":          VisibilityPublic, // PUBLIC — found while fixing the public-repo risk rule
		// All four checked via `gh api repos/<repo> --jq .visibility`;
		// all four returned "private". If ANY is ever flipped public it becomes
		// risk-classed unconditionally (VisibilityRiskClassed) and this value must change
		// with it — `deskboard policydrift` is what catches the lag.
		"example-org/platform":                  VisibilityPrivate,
		"example-org/demo-slides":               VisibilityPrivate,
		"example-org/assay-slides":              VisibilityPrivate,
		"example-org/example-reconciler-slides": VisibilityPrivate,
		"example-org/console":                   VisibilityPrivate, // `gh repo view ... --json visibility`
		// Outside the set → unknown (fail closed), never an implicit "private".
		"attacker/example-k8s":       VisibilityUnknown,
		"other-org/example-k8s":      VisibilityUnknown,
		"":                           VisibilityUnknown,
		"EXAMPLE-ORG/EXAMPLE-K8S":    VisibilityUnknown, // case-sensitive
		"other-org/platform":         VisibilityUnknown, // wrong org
		"example-org/product-slides": VisibilityUnknown, // product name, repo is example-reconciler-slides
		"EXAMPLE-ORG/PLATFORM":       VisibilityUnknown, // case-sensitive
		"other-org/console":          VisibilityUnknown, // wrong org
	}
	for _, r := range AllowedRepos() {
		if _, ok := cases[r]; !ok {
			t.Fatalf("allowed repo %q has no Visibility expectation here — the visibility policy and "+
				"the allowed set have drifted", r)
		}
		if RepoVisibility(r) == VisibilityUnknown {
			t.Fatalf("allowed repo %q has no compiled-in visibility — state it (it currently fails "+
				"closed, so every PR there is risk-classed)", r)
		}
	}
	for repo, want := range cases {
		t.Run(repo, func(t *testing.T) {
			if got := RepoVisibility(repo); got != want {
				t.Fatalf("RepoVisibility(%q) = %v, want %v", repo, got, want)
			}
		})
	}
}

func TestParseVisibility(t *testing.T) {
	cases := map[string]Visibility{
		"public":   VisibilityPublic,
		"private":  VisibilityPrivate,
		"internal": VisibilityUnknown, // org-visible is NOT private — fail closed, report drift
		"Public":   VisibilityUnknown, // case-sensitive
		"PRIVATE":  VisibilityUnknown,
		"":         VisibilityUnknown,
		"unknown":  VisibilityUnknown,
		"public ":  VisibilityUnknown,
	}
	for in, want := range cases {
		t.Run(strconv.Quote(in), func(t *testing.T) {
			if got := ParseVisibility(in); got != want {
				t.Fatalf("ParseVisibility(%q) = %v, want %v", in, got, want)
			}
		})
	}
}

// TestVisibilityDriftClean — the compiled-in table against a truthful observation is
// silent. This is also the shape `deskboard policydrift` produces on a good day.
func TestVisibilityDriftClean(t *testing.T) {
	observed := map[string]string{}
	for _, r := range AllowedRepos() {
		observed[r] = RepoVisibility(r).String()
	}
	if d := VisibilityDrift(observed); len(d) != 0 {
		t.Fatalf("VisibilityDrift on a truthful observation = %v, want none", d)
	}
}

// TestVisibilityDriftCatchesWrongCompiledInValue — the point of the check. A policy
// table that says example-k8s is private while GitHub says public is exactly the state
// that silently disables the gate, so the check must name it.
func TestVisibilityDriftCatchesWrongCompiledInValue(t *testing.T) {
	wrong := map[string]repoPolicy{
		"example-org/example-k8s": {CIRequired: true, Visibility: VisibilityPrivate}, // LIE
		"medici-finance/assay":    {CIRequired: true, Visibility: VisibilityPrivate}, // truthful
	}
	truth := map[string]string{
		"example-org/example-k8s": "public",
		"medici-finance/assay":    "private",
	}
	got := visibilityDriftAgainst(wrong, truth)
	if len(got) != 1 {
		t.Fatalf("visibilityDriftAgainst = %v, want exactly one drift line", got)
	}
	if !strings.Contains(got[0], "example-org/example-k8s") ||
		!strings.Contains(got[0], "public") || !strings.Contains(got[0], "private") {
		t.Fatalf("drift line %q must name the repo and BOTH values", got[0])
	}
}

// TestVisibilityDriftFailsClosedOnGaps — everything the check cannot positively confirm
// is drift. A silent skip would make the check worthless.
func TestVisibilityDriftFailsClosedOnGaps(t *testing.T) {
	policy := map[string]repoPolicy{
		"o/observed":  {Visibility: VisibilityPrivate},
		"o/missing":   {Visibility: VisibilityPrivate},
		"o/unstated":  {CIRequired: true}, // no visibility stated
		"o/weirdapi":  {Visibility: VisibilityPrivate},
		"o/flippedto": {Visibility: VisibilityPrivate},
	}
	observed := map[string]string{
		"o/observed":   "private",
		"o/unstated":   "private",
		"o/weirdapi":   "internal",
		"o/flippedto":  "public",
		"o/not-in-set": "public",
		// "o/missing" deliberately absent
	}
	got := visibilityDriftAgainst(policy, observed)
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"o/missing",   // never observed
		"o/unstated",  // compiled-in value absent
		"o/weirdapi",  // unrecognised API value
		"o/flippedto", // genuine disagreement
		"o/not-in-set",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("drift report %q does not mention %q", joined, want)
		}
	}
	if strings.Contains(joined, "o/observed:") {
		t.Fatalf("drift report flagged the one clean repo: %q", joined)
	}
	if len(got) != 5 {
		t.Fatalf("got %d drift lines, want 5: %v", len(got), got)
	}
}

// --- owner/* PATTERN support (extended to configuration) -------------
//
// The fixture roster (rosterfixture_test.go, shared TestMain) carries NO pattern
// entry, so every test above exercises the plain explicit-only path unchanged from
// before pattern support was added. Pattern behaviour is exercised here with its OWN installed
// roster (withRoster, per-test HOME + ReloadConfig), so it cannot perturb the shared
// fixture's exact repo set that the whole rest of this package's suite (and several
// cmd/ packages') asserts against.

// patternRoster carries one explicit repo plus one owner/* pattern, and a second
// owner's explicit repo that the pattern must NOT reach.
const patternRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,medici-finance/*,other-org/explicit-repo:no-ci:public
`

// TestPatternWildcardAdmitsAnyRepoUnderOwner is the row that would have caught
// `example-alerts` (invisible under a fixed enumerated census):
// a repo matching ONLY the pattern, never explicitly listed, is allowed with ZERO
// configuration edit beyond the pattern entry itself.
func TestPatternWildcardAdmitsAnyRepoUnderOwner(t *testing.T) {
	installRoster(t, patternRoster)

	for _, repo := range []string{
		"medici-finance/example-alerts",        // was invisible before org-default
		"medici-finance/repo-created-tomorrow", // synthetic — proves NO edit needed for new repos
		"medici-finance/assay",
	} {
		if !IsAllowedRepo(repo) {
			t.Errorf("IsAllowedRepo(%q) = false with medici-finance/* configured — pattern admission broken", repo)
		}
	}
}

// TestPatternExplicitEntryAndPatternCoexist — backward compatibility: an explicit
// owner/name entry alongside a pattern for a DIFFERENT owner still matches exactly as
// it always did, and a repo under neither is still refused.
func TestPatternExplicitEntryAndPatternCoexist(t *testing.T) {
	installRoster(t, patternRoster)

	cases := []struct {
		repo string
		want bool
	}{
		{"example-org/tracker", true},          // explicit entry, unaffected by the pattern
		{"other-org/explicit-repo", true},      // explicit entry under a DIFFERENT owner than the pattern
		{"medici-finance/anything", true},      // the pattern
		{"example-org/some-other-repo", false}, // NOT explicit, and example-org has no pattern
		{"example-org/example-cosmes", false},  // absent from both the explicit set and any pattern
		{"attacker/medici-finance", false},     // wrong owner — the pattern matches the OWNER half only
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.repo, func(t *testing.T) {
			if got := IsAllowedRepo(c.repo); got != c.want {
				t.Fatalf("IsAllowedRepo(%q) = %v, want %v", c.repo, got, c.want)
			}
		})
	}
}

// TestPatternDoesNotWidenCIOrVisibility pins the design boundary stated in config.go's
// allowedRepos doc comment: a pattern widens IsAllowedRepo ONLY. A repo matched by the
// pattern alone (never explicitly configured) must still answer CI-required (fail
// closed) and VisibilityUnknown (risk-classed) — exactly as an org-default repo did
// under the former compiled-in design.
func TestPatternDoesNotWidenCIOrVisibility(t *testing.T) {
	installRoster(t, patternRoster)

	const patternOnly = "medici-finance/pattern-only-repo"
	if !IsAllowedRepo(patternOnly) {
		t.Fatalf("precondition: %q must be allowed by the pattern", patternOnly)
	}
	if !CIRequired(patternOnly) {
		t.Errorf("CIRequired(%q) = false for a pattern-only repo; false is the fail-OPEN answer "+
			"and must come only from an explicit entry", patternOnly)
	}
	if got := RepoVisibility(patternOnly); got != VisibilityUnknown {
		t.Errorf("RepoVisibility(%q) = %v, want VisibilityUnknown — a pattern carries no visibility", patternOnly, got)
	}
	if !VisibilityRiskClassed(patternOnly) {
		t.Errorf("VisibilityRiskClassed(%q) = false — an unknown visibility must risk-class", patternOnly)
	}
}

// TestPatternNeverLeaksIntoAllowedRepos is the regression this design explicitly
// guards against (config.go's AllowedRepos doc comment): every production caller
// passes each AllowedRepos element straight to `gh api repos/<repo>`, so a returned
// "medici-finance/*" element is a live break, not a cosmetic one.
func TestPatternNeverLeaksIntoAllowedRepos(t *testing.T) {
	installRoster(t, patternRoster)

	for _, r := range AllowedRepos() {
		if strings.ContainsAny(r, "*") {
			t.Fatalf("AllowedRepos() returned %q — a pattern must never appear here", r)
		}
	}
	want := []string{"example-org/tracker", "other-org/explicit-repo"}
	if got := AllowedRepos(); !reflect.DeepEqual(got, want) {
		t.Fatalf("AllowedRepos() = %v, want %v (patterns excluded)", got, want)
	}
}

// TestAllowedRepoScopeIncludesPatterns — the DISPLAY set: the explicit census plus
// each configured owner/* pattern, rendered "owner/*" and appended after the sorted
// explicit set. Separate from AllowedRepos precisely because this one is not
// iterable against the API.
func TestAllowedRepoScopeIncludesPatterns(t *testing.T) {
	installRoster(t, patternRoster)

	want := []string{
		"example-org/tracker",
		"other-org/explicit-repo",
		"medici-finance/*",
	}
	if got := AllowedRepoScope(); !reflect.DeepEqual(got, want) {
		t.Fatalf("AllowedRepoScope() = %v, want %v", got, want)
	}
}

// TestAllowedRepoScopeNoPatternConfigured — counterweight: with no pattern configured
// (the shared fixture roster), the scope is byte-identical to AllowedRepos.
func TestAllowedRepoScopeNoPatternConfigured(t *testing.T) {
	if got, want := AllowedRepoScope(), AllowedRepos(); !reflect.DeepEqual(got, want) {
		t.Fatalf("AllowedRepoScope() = %v, want %v (no pattern configured, must equal AllowedRepos)", got, want)
	}
}

// TestPatternMalformedEntriesRefuseTheWholeRoster — a pattern entry carries NO
// ci/visibility policy (a token would apply to ONE repo's actual state, which a
// pattern does not name), and the owner half may not itself contain "*". Both are
// malformed-value refusals, exactly like a bad explicit slug: the WHOLE
// configuration collapses to unconfigured (P1) rather than silently dropping one
// entry and reporting success.
func TestPatternMalformedEntriesRefuseTheWholeRoster(t *testing.T) {
	const preamble = "ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\n"
	cases := map[string]string{
		"policy token on a pattern (ci)":     preamble + "ASSAY_ALLOWED_REPOS=medici-finance/*:ci\n",
		"policy token on a pattern (public)": preamble + "ASSAY_ALLOWED_REPOS=medici-finance/*:public\n",
		"double wildcard owner":              preamble + "ASSAY_ALLOWED_REPOS=medici-*finance/*\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			installRoster(t, body)

			if EffectiveConfig().Configured() {
				t.Fatalf("Configured() = true for a malformed pattern entry — must refuse the whole roster")
			}
			if len(EffectiveConfig().Problems) == 0 {
				t.Fatalf("no Problems recorded for a malformed pattern entry")
			}
		})
	}
}
