package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// couplingVectors is the shared cross-tree vector file — ONE roster, both readers.
type couplingVectors struct {
	Roster map[string]string `json:"roster"`
	Cases  []struct {
		Why   string `json:"why"`
		Login string `json:"login"`
		ID    int64  `json:"id"`
		// TrustedAuthorID drives the EXPORTED, general-purpose id check. It is a
		// separate column from TrustedContent because the two deliberately differ
		// on one input — a human login with id 0 — and collapsing them would either
		// tighten the general-purpose path against gh-CLI surfaces that carry no id,
		// or loosen the strict content path. Added because a correctness review
		// found mutation A (relaxing the exported check to
		// `want == 0 => trusted`) SURVIVING: every other test of it uses a pinned
		// identity, so the configured-but-UNPINNED login with a non-zero id — the
		// one input the conversion changed — was never exercised.
		TrustedAuthorID bool   `json:"trustedAuthorID"`
		WhyAuthorID     string `json:"whyAuthorID"`
		TrustedAuthor   bool   `json:"trustedAuthor"`
		TrustedContent  bool   `json:"trustedContent"`
		MayBless        bool   `json:"mayBless"`
	} `json:"cases"`
	RepoCases []struct {
		Why        string `json:"why"`
		Repo       string `json:"repo"`
		Allowed    bool   `json:"allowed"`
		CIRequired bool   `json:"ciRequired"`
		Visibility string `json:"visibility"`
	} `json:"repoCases"`
	RoleCases []struct {
		Why   string `json:"why"`
		Role  string `json:"role"`
		Bound bool   `json:"bound"`
		Login string `json:"login"`
	} `json:"roleCases"`
	RejectedRosters []struct {
		Why             string            `json:"why"`
		Roster          map[string]string `json:"roster"`
		ProblemContains string            `json:"problemContains"`
	} `json:"rejectedRosters"`
}

// couplingVectorPath is the ONE path both trees read. It lives in the STATUSGEN
// tree on purpose: statusgen then reads it in-module (so no CI-trigger registry
// entry is owed on that side), while THIS module's read is cross-module and is
// already covered by tools.yml's `statusgen/**` filter and the scancoupling
// registry row. It is referenced by name in TestScanIssuesTrustGateEnforced's
// assertion that statusgen's twin still consumes it, so a rename must be made in
// both places or the coupling guard fires.
const couplingVectorPath = "../../../../statusgen/testdata/roster_coupling.json"

func loadCouplingVectors(t *testing.T) couplingVectors {
	t.Helper()
	raw, err := os.ReadFile(couplingVectorPath)
	if err != nil {
		t.Fatalf("cannot read the shared cross-tree roster vectors at %s: %v", couplingVectorPath, err)
	}
	var v couplingVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s does not parse: %v", couplingVectorPath, err)
	}
	if len(v.Cases) == 0 {
		t.Fatalf("%s carries no cases — an empty vector file is a coupling guard that cannot fail", couplingVectorPath)
	}
	return v
}

// TestScanIssuesTrustGateEnforced is the ENFORCED coupling from the PR #1070
// review blocker, re-expressed over the CONFIG READERS.
//
// HISTORY:
//   - The original test pinned statusgen's scanRepos to three PRIVATE repos. Its
//     premise: planScan creates durable desk work items (placeholder files) from
//     GitHub issues and carried NO author trust gate, so the path was safe ONLY
//     while no external author could reach it.
//   - statusgen/trustgate.go landed a compiled-in trusted-author set plus a
//     single-authority blessing, ported from this package's trust.go.
//   - A later change (#152) then widened scanRepos to all owned repos,
//     including PUBLIC ones, relying on that gate.
//   - The roster move took BOTH copies of the roster out of source and into
//     adopter configuration. That removed the very literals this test used to
//     assert on, so the assertions moved with them — from TEXT to BEHAVIOUR.
//
// WHAT IT ASSERTS NOW. Every property the literal form expressed, in a form that
// survives configuration:
//
//	(1) SAME ROSTER, BOTH TREES. One roster and one expectation table
//	    (testdata/roster_coupling.json) is fed through this package's reader here,
//	    and through statusgen's reader by its own test over the SAME file. That is
//	    a strictly stronger binding than grepping a sibling file for a constant.
//	(2) LOGIN **AND** NUMERIC ID. The blessing path consults both; a reader that
//	    accepts a login without its pinned id re-opens login recycling.
//	(3) EXACTLY ONE BLESSING AUTHORITY. The old `n != 1` count on `Login = "`
//	    constants becomes behaviour: with several trusted identities configured,
//	    exactly one may bless and every other is refused. Trusted-AUTHOR and
//	    may-BLESS stay distinct capabilities — an agent-driven account that could
//	    bless would re-open the hole this gate closed.
//	(4) UNSET IS CLOSED. With no roster, nobody can bless. The literal form could
//	    not express this at all.
//	(5) statusgen's half still routes through the config reader, and scanissues.go's
//	    fail-closed assertions are UNCHANGED by this brief.
//
// Reading a sibling module's source from a test remains unusual on purpose: a
// comment is not an enforcement (the review's exact objection) — a failing test
// is. The path is ../../../../statusgen/ because statusgen lives at the repo root.
//
// If this test fires, do NOT relax the assertion to green it: the scanner reads
// PUBLIC repos, so a missing gate means arbitrary external issue text can create
// desk work items. Rewriting an assertion to be config-aware is not relaxing it;
// deleting one, widening it so an EMPTY roster satisfies it, or t.Skip-ing the
// test IS relaxing it and is forbidden.
func TestScanIssuesTrustGateEnforced(t *testing.T) {
	base := filepath.Join("..", "..", "..", "..", "statusgen")

	read := func(name string) string {
		p := filepath.Join(base, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("cannot read %s — the --scan-issues trust gate must exist and stay "+
				"readable from here; if statusgen moved, re-point this coupling test, do NOT "+
				"delete it (PR #1070 review blocker): %v", p, err)
		}
		return string(data)
	}

	// ---- (1)+(2)+(3): behaviour, over the SHARED vector file ----
	//
	// This half runs the vectors through THIS tree's reader. statusgen's twin runs
	// the identical file through its own reader (TestRosterCouplingVectors there),
	// and both are pinned at count 2 across the two
	// packages. One roster in, identical verdicts out.
	vec := loadCouplingVectors(t)
	withRoster(t, vec.Roster)
	if !EffectiveConfig().Configured() {
		t.Fatalf("the shared coupling roster did not load: %v — the positive control is broken, so "+
			"every assertion below would prove nothing", EffectiveConfig().Problems)
	}

	blessCount := 0
	for _, c := range vec.Cases {
		if got := TrustedAuthor(c.Login); got != c.TrustedAuthor {
			t.Errorf("TrustedAuthor(%q) = %t, want %t — %s", c.Login, got, c.TrustedAuthor, c.Why)
		}
		if got := trustedContentAuthor(c.Login, c.ID); got != c.TrustedContent {
			t.Errorf("trustedContentAuthor(%q, %d) = %t, want %t — %s", c.Login, c.ID, got, c.TrustedContent, c.Why)
		}
		// The EXPORTED id check, gated on by deskboard/board.go and deskpost/github.go.
		// Driven by the same vector so the two id-aware paths cannot silently diverge
		// again (mutation A).
		if got := TrustedAuthorID(c.Login, c.ID); got != c.TrustedAuthorID {
			why := c.Why
			if c.WhyAuthorID != "" {
				why = c.WhyAuthorID
			}
			t.Errorf("TrustedAuthorID(%q, %d) = %t, want %t — %s", c.Login, c.ID, got, c.TrustedAuthorID, why)
		}
		if got := IsBlessAuthorityID(c.Login, c.ID); got != c.MayBless {
			t.Errorf("IsBlessAuthorityID(%q, %d) = %t, want %t — %s", c.Login, c.ID, got, c.MayBless, c.Why)
		}
		if c.MayBless {
			blessCount++
		}
	}
	// (3) restated as an invariant of the VECTORS themselves, so a future edit that
	// adds a second blessing identity to the fixture fails here rather than
	// quietly teaching both trees that two accounts may bless.
	if blessCount == 0 {
		t.Error("no vector case may bless — the fixture cannot prove the blessing authority exists")
	}
	blessers := map[string]bool{}
	for _, c := range vec.Cases {
		if IsBlessAuthorityID(c.Login, c.ID) {
			blessers[strings.ToLower(c.Login)] = true
		}
	}
	if len(blessers) != 1 {
		t.Errorf("%d DISTINCT logins may bless under the shared roster (%v), want exactly 1 — trusted "+
			"AUTHOR and may-BLESS are different capabilities, and a second blessing authority re-opens "+
			"the hole this gate closed", len(blessers), blessers)
	}

	// ---- (4): unset is closed ----
	withNoRoster(t)
	for _, c := range vec.Cases {
		if IsBlessAuthorityID(c.Login, c.ID) {
			t.Errorf("%q could bless with NO roster configured — unset must mean nobody blesses", c.Login)
		}
		if TrustedAuthor(c.Login) {
			t.Errorf("%q was trusted with NO roster configured", c.Login)
		}
	}
	withRoster(t, vec.Roster) // restore for anything below

	// ---- (5a): statusgen's twin reads CONFIG, not a compiled roster ----
	tg := read("trustgate.go")

	// The conversion's structural residue: there is no longer ANY compiled-in
	// `<name>Login = "..."` constant in the twin. The old form asserted exactly
	// one; under configuration the correct count is zero, and a non-zero count
	// means someone re-introduced a hardcoded authority alongside the config
	// reader — which is how a "configurable" roster quietly keeps a back door.
	if n := len(regexp.MustCompile(`\w*Login\s*=\s*"`).FindAllString(tg, -1)); n != 0 {
		t.Errorf("trustgate.go: found %d compiled-in `Login = \"...\"` constant(s), want 0 — the blessing "+
			"authority is CONFIGURATION now; a hardcoded login beside the config reader is a second, "+
			"unreviewable authority", n)
	}
	if regexp.MustCompile(`scanBlesser\w*`).MatchString(tg) {
		t.Error("trustgate.go still carries a compiled-in single-organisation identity constant")
	}

	blessIdx := strings.Index(tg, "func evalIssueBlessing(")
	if blessIdx < 0 {
		t.Fatal("trustgate.go: evalIssueBlessing is gone — nothing evaluates the blessing")
	}
	body := tg[blessIdx:]
	if !strings.Contains(body, "isBlessAuthorityID") {
		t.Error("trustgate.go: evalIssueBlessing no longer selects the blessing comment through the " +
			"configured blessing-authority check — the blessing is no longer restricted to one " +
			"configured human")
	}
	if !strings.Contains(body, "scanRosterUnconfiguredError") {
		t.Error("trustgate.go: evalIssueBlessing does not refuse on an UNCONFIGURED roster — an unset " +
			"roster would silently make every issue merely 'unblessed' instead of stopping the scan")
	}
	if !strings.Contains(body, "trustedAuthorID") {
		t.Error("trustgate.go: evalIssueBlessing no longer re-checks untrusted content with the " +
			"numeric-id-aware form — login-only content trust is vulnerable to login recycling")
	}

	// The twin's LOADER must exist and must be the write class: statusgen's roster
	// consumer is --scan-issues, which WRITES placeholder files.
	rc := read("rosterconfig.go")
	if !strings.Contains(rc, "scanClassWrite") {
		t.Error("statusgen/rosterconfig.go: no write class — --scan-issues WRITES durable work items, " +
			"so its roster must not be reachable from the environment")
	}
	if !strings.Contains(rc, "NO ENVIRONMENT READ") {
		t.Error("statusgen/rosterconfig.go: the write-class arm no longer states that it does not read " +
			"the environment; check it still does not")
	}

	// The twin must still CONSUME the shared vector file. Without this, the two
	// readers could drift apart with both suites green: this package's half would
	// keep passing over vectors statusgen no longer runs.
	tw := read("rosterconfig_test.go")
	// The two trees spell the path differently (one is in-module, one is not), so
	// the assertion is on the file NAME: both must still consume the same vectors.
	vectorFile := filepath.Base(couplingVectorPath)
	if !strings.Contains(tw, vectorFile) {
		t.Errorf("statusgen's roster test no longer reads the shared vector file %q — the cross-tree "+
			"binding is gone, and the two duplicated readers can now disagree with both suites green",
			vectorFile)
	}
	if !strings.Contains(tw, "TestRosterCouplingVectors") {
		t.Error("statusgen no longer runs TestRosterCouplingVectors — the twin half of this coupling is gone")
	}

	// ---- (5b): scanissues.go's fail-closed assertions, UNCHANGED by this brief ----
	si := read("scanissues.go")

	if !strings.Contains(si, "!trustedAuthor(iss.Author.Login)") {
		t.Fatal("scanissues.go: planScan no longer calls !trustedAuthor(iss.Author.Login) — the " +
			"trust gate has been removed or bypassed, and scanRepos includes PUBLIC repos " +
			"(PR #1070 review blocker)")
	}
	if !strings.Contains(si, "bless(repo, iss.Number)") {
		t.Error("scanissues.go: the blessing checker is no longer invoked for untrusted authors — " +
			"untrusted issues would be admitted unblessed")
	}
	if !strings.Contains(si, "scanRosterUnconfiguredError") {
		t.Error("scanissues.go: runScanIssues no longer REFUSES on an unconfigured roster — a scan " +
			"with no roster would create placeholders from arbitrary external issue text")
	}

	// Fail-closed assertions are WINDOWED: scanissues.go contains many `continue`
	// statements, so an unbounded search from the branch index would pass even with
	// the branch's own `continue` deleted.
	const window = 400
	mustContinue := func(branch, why string) {
		t.Helper()
		i := strings.Index(si, branch)
		if i < 0 {
			t.Errorf("scanissues.go: the %q branch is gone — %s", branch, why)
			return
		}
		end := i + window
		if end > len(si) {
			end = len(si)
		}
		if !strings.Contains(si[i:end], "continue") {
			t.Errorf("scanissues.go: the %q branch no longer skips placeholder creation "+
				"(no `continue` within %d bytes) — %s", branch, window, why)
		}
	}
	mustContinue("berr != nil",
		"an unverifiable blessing read must fail CLOSED (no placeholder, loud NOTICE)")
	mustContinue("if !blessed",
		"an unblessed untrusted-author issue must never become a desk work item")
}

// TestCouplingRepoRolesAndRejections is the deskkit half of the extended
// cross-tree binding (a correctness review).
//
// The vector originally carried only the three TRUST variables, so the repo-policy
// parse sat structurally OUTSIDE the coupling — and the two readers HAD diverged
// there: this copy validated the ci|no-ci / public|private tokens and collapsed the
// whole configuration on a bad one, while statusgen's stored the token unvalidated
// and reported configured=true. One roster file, one typo, opposite verdicts.
//
// The REJECTION cases are the ones that bind the two validators: an acceptance case
// only proves the readers agree about a GOOD file, and they never disagreed there.
//
// statusgen runs the identical table against its own reader
// (TestCouplingRepoRolesAndRejections in statusgen/rosterconfig_test.go). Both must
// exist — a table run through one reader is not a coupling.
func TestCouplingRepoRolesAndRejections(t *testing.T) {
	vec := loadCouplingVectors(t)
	if len(vec.RepoCases) == 0 || len(vec.RoleCases) == 0 || len(vec.RejectedRosters) == 0 {
		t.Fatalf("%s carries repoCases=%d roleCases=%d rejectedRosters=%d — an empty section is a "+
			"coupling guard that cannot fail", couplingVectorPath,
			len(vec.RepoCases), len(vec.RoleCases), len(vec.RejectedRosters))
	}

	withRoster(t, vec.Roster)
	cfg := EffectiveConfig()
	if !cfg.Configured() {
		t.Fatalf("the shared coupling roster did not load into deskkit's reader: %v", cfg.Problems)
	}

	for _, rc := range vec.RepoCases {
		if got := IsAllowedRepo(rc.Repo); got != rc.Allowed {
			t.Errorf("IsAllowedRepo(%q) = %t, want %t — %s", rc.Repo, got, rc.Allowed, rc.Why)
			continue
		}
		if !rc.Allowed {
			// Outside the set, the fail-closed answers still have to hold.
			if !CIRequired(rc.Repo) {
				t.Errorf("CIRequired(%q) must be TRUE for a repo outside the set — %s", rc.Repo, rc.Why)
			}
			continue
		}
		if got := CIRequired(rc.Repo); got != rc.CIRequired {
			t.Errorf("CIRequired(%q) = %t, want %t — %s (a mutation flipped this default "+
				"and survived)", rc.Repo, got, rc.CIRequired, rc.Why)
		}
		if got := cfg.Repos[rc.Repo].Visibility.String(); got != rc.Visibility {
			t.Errorf("visibility(%q) = %q, want %q — %s (a mutation flipped the UNSTATED "+
				"default from unknown to private, un-risk-classing the repo, and survived)",
				rc.Repo, got, rc.Visibility, rc.Why)
		}
	}

	for _, rc := range vec.RoleCases {
		login, bound := RoleAppLogin(rc.Role)
		if bound != rc.Bound {
			t.Errorf("RoleAppLogin(%q) bound = %t, want %t — %s", rc.Role, bound, rc.Bound, rc.Why)
			continue
		}
		if login != rc.Login {
			t.Errorf("RoleAppLogin(%q) = %q, want %q — %s", rc.Role, login, rc.Login, rc.Why)
		}
	}

	for _, rr := range vec.RejectedRosters {
		withRoster(t, rr.Roster)
		c := EffectiveConfig()
		if c.Configured() {
			t.Errorf("a roster that MUST be refused loaded as configured — %s\n  roster: %v",
				rr.Why, rr.Roster)
			continue
		}
		joined := strings.Join(c.Problems, "\n")
		if !strings.Contains(joined, rr.ProblemContains) {
			t.Errorf("the refusal does not name %q — %s\n  problems: %s",
				rr.ProblemContains, rr.Why, joined)
		}
	}
}
