package deskkit

import (
	"reflect"
	"strings"
	"testing"
)

// The repos the table below leans on, named once so a policy change breaks compilation
// of the intent rather than silently re-labelling a case.
const (
	trackerRepo    = "example-org/tracker"     // private, CI-required
	exampleK8sRepo = "example-org/example-k8s" // PUBLIC
	toolkitRepo    = "medici-finance/assay"    // private, holds the gate itself
	agentsRepo     = "example-org/agents"      // private, no repo-specific triggers
	proposalsRepo  = "example-org/proposals"   // PUBLIC (second public repo)
	notInSetRepo   = "attacker/tracker"        // outside the fixed set
	slidesRepo     = "example-org/org-slides"  // private, no repo-specific triggers
)

// TestRiskPathTriggered — the whole classification, front door. Every "false" row is a
// claim that the gate may be waived, so each one names a KNOWN-PRIVATE, in-set repo
// with a readable, clean changed-file list. Nothing else is allowed to answer false.
func TestRiskPathTriggered(t *testing.T) {
	cases := []struct {
		name  string
		repo  string
		files []string
		want  bool
	}{
		// --- tracker: every compiled base trigger fires on a private in-set repo ---
		{"tracker secrets dir", trackerRepo, []string{"secrets/prod/token.yaml"}, true},
		{"tracker secrets top file", trackerRepo, []string{"secrets/api.key"}, true},
		{"tracker github workflow", trackerRepo, []string{".github/workflows/ci.yml"}, true},
		{"tracker github workflow nested", trackerRepo, []string{".github/workflows/sub/deploy.yml"}, true},
		{"tracker k8s dev rbac", trackerRepo, []string{"k8s/dev/rbac.yaml"}, true},
		{"tracker k8s prod rbac", trackerRepo, []string{"k8s/prod/rbac.yaml"}, true},
		{"tracker risk among clean", trackerRepo, []string{"README.md", "frontend/src/App.tsx", "secrets/db.pw"}, true},

		// --- tracker: near-misses must NOT fire (the waiver still works where intended) ---
		{"tracker docs only", trackerRepo, []string{"docs/streams/desk-tools/brief-03-deskpost.md"}, false},
		{"tracker frontend only", trackerRepo, []string{"frontend/src/lib/client.ts"}, false},
		{"tracker secrets prefix not segment", trackerRepo, []string{"secretsmanager/config.go"}, false},
		{"tracker workflows sibling not segment", trackerRepo, []string{".github/workflowsx/y.yml"}, false},
		{"tracker github non-workflows", trackerRepo, []string{".github/CODEOWNERS"}, false},
		{"tracker k8s rbac wrong depth", trackerRepo, []string{"k8s/dev/base/rbac.yaml"}, false},
		{"tracker k8s rbac too shallow", trackerRepo, []string{"k8s/rbac.yaml"}, false},
		{"tracker k8s non-rbac file", trackerRepo, []string{"k8s/dev/deployment.yaml"}, false},
		{"tracker secret elsewhere", trackerRepo, []string{"config/secrets.md"}, false},

		// --- public repos are risk-classed unconditionally (visibility alone). example-k8s
		// also has repo-specific triggers, exercised directly in
		// TestPathTriggersReachablePastVisibility. ---
		{"example-k8s manifest under base", exampleK8sRepo, []string{"base/app/config.yaml"}, true},
		{"example-k8s admin script", exampleK8sRepo, []string{"admin/scripts/rotate.sh"}, true},
		{"example-k8s deploy script", exampleK8sRepo, []string{"deploy/scripts/apply.sh"}, true},
		{"example-k8s validate workflow", exampleK8sRepo, []string{".github/workflows/validate.yml"}, true},
		// Public means public: even a README-only PR on a public repo is risk-classed.
		{"example-k8s readme only", exampleK8sRepo, []string{"README.md"}, true},
		{"example-k8s docs only", exampleK8sRepo, []string{"docs/architecture.md"}, true},
		{"proposals readme only", proposalsRepo, []string{"README.md"}, true},

		// --- the desk's own gate code (this toolkit) ---
		{"toolkit deskkit", toolkitRepo, []string{"tools/desk/internal/deskkit/riskpath.go"}, true},
		{"toolkit deskpost", toolkitRepo, []string{"tools/desk/cmd/deskpost/ready.go"}, true},
		{"toolkit desktoken", toolkitRepo, []string{"tools/desk/cmd/desktoken/main.go"}, true},
		{"toolkit statusgen not a trigger", toolkitRepo, []string{"statusgen/main.go"}, false},
		{"toolkit docs not a trigger", toolkitRepo, []string{"docs/adopting-assay.md"}, false},

		// --- a private repo with no repo-specific triggers still gets the base list ---
		{"agents base trigger fires", agentsRepo, []string{"secrets/x.pem"}, true},
		{"agents clean", agentsRepo, []string{"agents/oracle-agent/index.ts"}, false},
		{"slides clean", slidesRepo, []string{"notes/2026-08-01.md"}, false},

		// --- fail-closed inputs: every one of these answers TRUE ---
		{"unknown repo", notInSetRepo, []string{"README.md"}, true},
		{"empty repo string", "", []string{"README.md"}, true},
		{"case-mangled repo", "EXAMPLE-ORG/EXAMPLE-PRODUCT", []string{"README.md"}, true},
		{"empty file list private repo", trackerRepo, nil, true},
		{"empty file list public repo", exampleK8sRepo, nil, true},
		{"zero-length file slice", trackerRepo, []string{}, true},
		{"blank path entry", trackerRepo, []string{""}, true},
		{"whitespace path entry", trackerRepo, []string{"   "}, true},
		{"blank among clean", trackerRepo, []string{"README.md", ""}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RiskPathTriggered(c.repo, c.files); got != c.want {
				t.Fatalf("RiskPathTriggered(%q, %v) = %v, want %v", c.repo, c.files, got, c.want)
			}
		})
	}
}

// TestHouseTriggersRideConfigNotCompiledBase is brief-04's coverage guarantee. The
// compiled base was genericized: a deployment's REAL security paths no longer live in
// this shared tree — they are supplied by the adopter through
// ASSAY_RISK_PATH_TRIGGERS_EXTRA. This test proves the interim mechanism actually
// carries that coverage: paths shaped like an adopter's real triggers ARE risk-classed
// when the variable supplies them, and are NOT risk-classed when it is unset — so the
// coverage genuinely flows from the config, which is exactly why setting the variable
// is the merge precondition (a run against an unset variable would silently under-class
// those paths). It also re-asserts the union only ever WIDENS and stays fail-closed —
// the properties the removed old-base monotonicity test used to guard.
func TestHouseTriggersRideConfigNotCompiledBase(t *testing.T) {
	repo := "one/private-repo"
	// Placeholder paths standing in for whatever real trigger paths an adopter sets;
	// the real values live in adopter/house config, never in this tree (Verify row 2).
	houseTriggers := "product/internal/auth/,product/internal/api/,k8s/*/product.yaml"
	houseHits := []string{
		"product/internal/auth/token.go",
		"product/internal/api/handler.go",
		"k8s/dev/product.yaml",
	}
	rosterWith := func(extra string) map[string]string {
		r := goldenRoster()
		r[EnvAllowedRepos] = repo + ":ci:private"
		if extra != "" {
			r[EnvRiskPathTriggersExtra] = extra
		}
		return r
	}

	// (1) UNSET: the house-shaped paths are NOT risk-classed. If they were, coverage
	// would not depend on the config and the merge precondition would be theatre.
	withRoster(t, rosterWith(""))
	for _, f := range houseHits {
		if RiskPathTriggered(repo, []string{f}) {
			t.Fatalf("house path %q was risk-classed with %s UNSET — coverage must flow from the "+
				"config, not from a compiled house path lingering in the base", f, EnvRiskPathTriggersExtra)
		}
	}

	// (2) SET: the same paths ARE risk-classed — the configured triggers union onto the
	// compiled set, so genericizing the base did not drop coverage (Verify row 4).
	withRoster(t, rosterWith(houseTriggers))
	for _, f := range houseHits {
		if !RiskPathTriggered(repo, []string{f}) {
			t.Fatalf("house path %q was NOT risk-classed with %s=%q — the configured triggers must "+
				"union onto the compiled set", f, EnvRiskPathTriggersExtra, houseTriggers)
		}
	}

	// (3) additive-only: the config only WIDENS. The generic compiled base still fires
	// in BOTH states, and a genuinely clean file is classed in NEITHER.
	for _, st := range []string{"", houseTriggers} {
		withRoster(t, rosterWith(st))
		if !RiskPathTriggered(repo, []string{"secrets/prod.key"}) {
			t.Fatalf("a compiled base trigger stopped firing with %s=%q", EnvRiskPathTriggersExtra, st)
		}
		if RiskPathTriggered(repo, []string{"docs/notes/scratch.md"}) {
			t.Fatalf("a clean file was risk-classed with %s=%q — positive control inverted", EnvRiskPathTriggersExtra, st)
		}
	}

	// (4) fail-closed is not configurable: a public repo and an empty diff still class
	// regardless of the trigger config.
	pub := "one/public-repo"
	rpub := rosterWith(houseTriggers)
	rpub[EnvAllowedRepos] = repo + ":ci:private," + pub + ":ci:public"
	withRoster(t, rpub)
	if !RiskPathTriggered(pub, []string{"docs/notes/scratch.md"}) {
		t.Fatal("a PUBLIC repo stopped being risk-classed — visibility must not be configurable")
	}
	if !RiskPathTriggered(repo, nil) {
		t.Fatal("an empty changed-file list stopped being risk-classed — fail-closed must not be configurable")
	}
}

// TestRiskPathTriggeredNeverFalseOutsideTheSet — a repo outside the fixed set can never
// be waived, whatever it changes. Same fail-closed shape as CIRequired.
func TestRiskPathTriggeredNeverFalseOutsideTheSet(t *testing.T) {
	for _, repo := range []string{"attacker/example-k8s", "example-org/example-k8s", "", "no-slash", "a/b/c"} {
		if !RiskPathTriggered(repo, []string{"README.md"}) {
			t.Fatalf("RiskPathTriggered(%q, clean files) = false; a repo with no compiled-in policy must fail closed", repo)
		}
	}
}

// TestUnsetVisibilityFailsClosed — a repo added to the policy table WITHOUT stating a
// visibility must risk-class, not silently inherit "private". The zero value carries
// this property, so the test asserts on the zero value directly.
func TestUnsetVisibilityFailsClosed(t *testing.T) {
	var unset Visibility
	if unset != VisibilityUnknown {
		t.Fatal("the zero Visibility must be VisibilityUnknown — otherwise an unstated policy defaults to a waiver")
	}
	p := repoPolicy{CIRequired: true} // visibility deliberately omitted
	if p.Visibility != VisibilityUnknown {
		t.Fatal("a repoPolicy with no stated visibility must be VisibilityUnknown")
	}
	// And the consumer must treat unknown as risk-classed.
	for _, v := range []Visibility{VisibilityUnknown, VisibilityPublic, Visibility(99)} {
		if v == VisibilityPrivate {
			continue
		}
		if v.String() == "private" {
			t.Fatalf("Visibility(%d).String() renders as private", v)
		}
	}
	if VisibilityRiskClassed("example-org/example-k8s") != true {
		t.Fatal("a public repo must be risk-classed on visibility alone")
	}
	// Every PUBLIC census repo, not just example-k8s. Adding a public repo to the census
	// must not be a way to LOWER its scrutiny: an org-default repo absent from the census
	// answers Unknown and is risk-classed, so a new row that got its visibility wrong
	// would silently take a world-readable repo from every-PR-risk-classed to none.
	// (This is the census-add hazard scoping.md records under review N3, applied to the
	// visibility field specifically.) medici-finance/assay is here because #310 R2 added
	// its row; TestCensusMatchesLive proves the value still matches the live API.
	for _, repo := range AllowedRepos() {
		if RepoVisibility(repo) != VisibilityPublic {
			continue
		}
		if !VisibilityRiskClassed(repo) {
			t.Fatalf("%s is a PUBLIC census repo but is not risk-classed on visibility alone", repo)
		}
	}
	if VisibilityRiskClassed("nobody/nothing") != true {
		t.Fatal("an unknown repo must be risk-classed on visibility alone")
	}
	if VisibilityRiskClassed(trackerRepo) != false {
		t.Fatal("a known-private repo must NOT be risk-classed on visibility alone (paths still apply)")
	}
}

// TestPathTriggersReachablePastVisibility — example-k8s is risk-classed by visibility, so
// RiskPathTriggered short-circuits before its path triggers. Assert the PATH half
// separately, or the repo-specific list would be untested (and could rot unnoticed the
// day someone makes the repo private).
func TestPathTriggersReachablePastVisibility(t *testing.T) {
	cases := []struct {
		repo  string
		file  string
		want  bool
		label string
	}{
		{exampleK8sRepo, "base/app/config.yaml", true, "manifest under base"},
		{exampleK8sRepo, "deploy/scripts/apply.sh", true, "deploy script"},
		{exampleK8sRepo, "admin/scripts/rotate.sh", true, "admin script"},
		{exampleK8sRepo, "hack/port-forward.sh", true, "hack script"},
		{exampleK8sRepo, ".github/workflows/validate.yml", true, "workflow (also a base trigger)"},
		{exampleK8sRepo, "docs/architecture.md", false, "docs are not a security path"},
		{exampleK8sRepo, "README.md", false, "readme is not a security path"},
		{exampleK8sRepo, "examples/demo.yaml", false, "examples are not a security path"},
		// The base list is universal — it applies to example-k8s too.
		{exampleK8sRepo, "secrets/x.pem", true, "base trigger applies everywhere"},
		// Repo-specific triggers do NOT leak to other repos (base/ admin/ hack/ are
		// example-k8s-only, unlike .github/workflows/ which is now a universal base trigger).
		{trackerRepo, "base/app/config.yaml", false, "example-k8s trigger must not apply to tracker"},
		{trackerRepo, "admin/scripts/rotate.sh", false, "example-k8s trigger must not apply to tracker"},
		{agentsRepo, "hack/port-forward.sh", false, "example-k8s trigger must not apply to agents"},
	}
	for _, c := range cases {
		t.Run(c.label+" ("+c.repo+" "+c.file+")", func(t *testing.T) {
			if got := pathTriggerHit(c.repo, []string{c.file}); got != c.want {
				t.Fatalf("pathTriggerHit(%q, %q) = %v, want %v", c.repo, c.file, got, c.want)
			}
		})
	}
}

// riskReasonFileSets is the changed-file input space both coupling tests walk.
var riskReasonFileSets = [][]string{
	nil, {}, {""}, {"README.md"}, {"secrets/x.pem"},
	{"base/app/config.yaml"}, {"tools/desk/internal/deskkit/riskpath.go"},
}

// assertReasonAgrees drives both accessors over every file set and fails on any
// disagreement. RiskClassReason's contract is EXACT: it says "not risk-classed"
// exactly when RiskPathTriggered answers false.
func assertReasonAgrees(t *testing.T, repos []string) {
	t.Helper()
	for _, repo := range repos {
		for _, files := range riskReasonFileSets {
			triggered := RiskPathTriggered(repo, files)
			reason := RiskClassReason(repo, files)
			if triggered == (reason == "not risk-classed") {
				t.Fatalf("RiskPathTriggered(%q,%v)=%v but RiskClassReason said %q", repo, files, triggered, reason)
			}
		}
	}
}

// TestRiskClassReasonAgreesWithTheGate — the message a refusal prints must never
// disagree with the decision it explains.
func TestRiskClassReasonAgreesWithTheGate(t *testing.T) {
	assertReasonAgrees(t, append(AllowedRepos(), "attacker/example-k8s", ""))
}

// TestRiskClassReasonAgreesOnUnstatedVisibility drives the SAME coupling over the
// one repo state the test above structurally cannot reach.
//
// RiskPathTriggered decides visibility through VisibilityRiskClassed (everything
// EXCEPT known-private classes); RiskClassReason re-derives it as two separate
// arms, VisibilityPublic and VisibilityUnknown. Three of the four fail-closed
// preconditions are therefore expressed twice, in two different shapes, and only
// one of the two is the gate.
//
// The gap was in the INPUT SPACE, not in the assertion. Every repo in the fixture
// roster states a visibility, so AllowedRepos() never yields a VisibilityUnknown
// entry, and the two out-of-set repos above are decided by precondition 1 before
// visibility is ever consulted. Deleting RiskClassReason's VisibilityUnknown arm
// therefore left the whole suite green while the accessor answered "not
// risk-classed" for a repo the gate was risk-classing — a refusal message
// contradicting its own refusal.
//
// Unstated visibility is not a hypothetical: `owner/name` and `owner/name:ci` are
// both legal ASSAY_ALLOWED_REPOS entries, and the fail-closed default that makes
// them safe is the very thing this arm reports. The C2 surface makes the state
// reachable by omission, so the coupling has to be asserted in it.
func TestRiskClassReasonAgreesOnUnstatedVisibility(t *testing.T) {
	r := goldenRoster()
	r[EnvAllowedRepos] = "acme/unstated,acme/ci-only:ci,acme/known:ci:private,acme/open:ci:public"
	withRoster(t, r)

	// Guard the fixture: if these ever start parsing with a stated visibility the
	// test below silently stops covering the arm it exists for.
	for _, repo := range []string{"acme/unstated", "acme/ci-only"} {
		if got := RepoVisibility(repo); got != VisibilityUnknown {
			t.Fatalf("fixture no longer exercises the arm: RepoVisibility(%q) = %v, want unknown", repo, got)
		}
	}
	assertReasonAgrees(t, []string{"acme/unstated", "acme/ci-only", "acme/known", "acme/open"})
}

func TestRiskPathTriggersImmutable(t *testing.T) {
	a := RiskPathTriggers()
	if len(a) == 0 {
		t.Fatal("expected a non-empty trigger list")
	}
	a[0] = "mutated/"
	b := RiskPathTriggers()
	if b[0] == "mutated/" {
		t.Fatal("RiskPathTriggers returned a slice aliasing internal state")
	}

	c := RiskPathTriggersFor(exampleK8sRepo)
	if len(c) <= len(b) {
		t.Fatalf("RiskPathTriggersFor(%s) = %v; expected the base list PLUS repo-specific triggers", exampleK8sRepo, c)
	}
	c[0] = "mutated/"
	if reflect.DeepEqual(RiskPathTriggersFor(exampleK8sRepo), c) {
		t.Fatal("RiskPathTriggersFor returned a slice aliasing internal state")
	}
}

// TestRiskPathTriggersForContainsBase — every repo, in the set or not, gets at least the
// universal base list. A repo-specific list may only ADD.
func TestRiskPathTriggersForContainsBase(t *testing.T) {
	base := RiskPathTriggers()
	for _, repo := range append(AllowedRepos(), "attacker/whatever") {
		got := RiskPathTriggersFor(repo)
		for _, want := range base {
			found := false
			for _, g := range got {
				if g == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("RiskPathTriggersFor(%q) dropped base trigger %q — repo lists may only widen", repo, want)
			}
		}
	}
}

// TestNoEnvOrFlagNarrowsTheGate — the gate reads NO configuration. A knob that could
// narrow it is a waiver waiting to be set, so assert the file mentions no env lookup.
func TestNoEnvOrFlagNarrowsTheGate(t *testing.T) {
	t.Setenv("DESK_RISK_PATHS", "")
	t.Setenv("RISK_PATH_TRIGGERS", "")
	t.Setenv("DESK_SKIP_SECURITY_REVIEW", "1")
	if !RiskPathTriggered(trackerRepo, []string{"secrets/x.pem"}) {
		t.Fatal("an environment variable narrowed the risk-path gate")
	}
	if !RiskPathTriggered(exampleK8sRepo, []string{"README.md"}) {
		t.Fatal("an environment variable narrowed the visibility gate")
	}
	if strings.Contains(strings.Join(RiskPathTriggersFor(trackerRepo), ","), "$") {
		t.Fatal("a trigger looks interpolated — triggers are literal, compiled-in patterns")
	}
}
