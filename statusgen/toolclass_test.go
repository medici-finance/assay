package main

import (
	"os"
	"strings"
	"testing"
)

// scanClassForMode is statusgen's half of the roster security boundary, and the correctness review found it
// with no test at all. Three mutations survived the whole suite:
//
//	M18  scanClassForMode always returns scanClassWrite — i.e. correctness
//	     finding 1 put back exactly as it was reported: --lint in the consumer's
//	     CI reads no roster, and the verifier floor fails on briefs the PR never
//	     touched.
//	M20  drop `scanIssuesMode ||` from the guard, so --scan-issues becomes
//	     ci-eligible. The reviewer built this one and ran it: the acting mode
//	     took its whole roster from the environment with an attacker as blessing
//	     authority. LIVE, not theoretical.
//	M17d removing scanEchoEffectiveConfig from main() — the P3 echo added so
//	     --lint prints the roster it judges with. The deskkit coverage guard
//	     enumerates the desk-tools commands only, and statusgen is a separate
//	     module, so nothing here was checking.
//
// The two directions are NOT symmetric and the tests must not treat them as such.
// M18 makes --lint stricter-but-blind (a false PROBLEM); M20 hands an acting mode
// to the environment. M20 is the one that matters, which is why it gets an
// end-to-end assertion through the real loader and not just a table row.

func TestScanClassForModeSplitsByAction(t *testing.T) {
	cases := []struct {
		why            string
		scanIssuesMode bool
		inCI           bool
		want           scanToolClass
	}{
		{
			why:            "--lint locally: no runner, so no CI transport",
			scanIssuesMode: false, inCI: false, want: scanClassWrite,
		},
		{
			// M18's target.
			why:            "--lint inside a runner: the reporting mode reads the Actions variables",
			scanIssuesMode: false, inCI: true, want: scanClassCI,
		},
		{
			why:            "--scan-issues locally: the acting mode, file-only",
			scanIssuesMode: true, inCI: false, want: scanClassWrite,
		},
		{
			// M20's target. --scan-issues WRITES placeholder briefs into the work
			// queue from GitHub issue text; it stays file-only ALWAYS, including
			// under GITHUB_ACTIONS.
			why:            "--scan-issues inside a runner: STILL file-only — the acting mode is never ci-eligible",
			scanIssuesMode: true, inCI: true, want: scanClassWrite,
		},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			if c.inCI {
				t.Setenv("GITHUB_ACTIONS", "true")
			} else {
				t.Setenv("GITHUB_ACTIONS", "")
			}
			if got := scanClassForMode(c.scanIssuesMode); got != c.want {
				t.Fatalf("scanClassForMode(scanIssuesMode=%t) with in-CI=%t = %v, want %v — %s",
					c.scanIssuesMode, c.inCI, got, c.want, c.why)
			}
		})
	}
}

// TestScanIssuesRefusesAHostileEnvUnderCI is M20's end-to-end kill: the roster is
// set in the ENVIRONMENT to an attacker identity, the config home is empty, and
// GITHUB_ACTIONS=true. --scan-issues must report UNCONFIGURED — it admits work
// items to the queue from untrusted issue text, so the environment must not be able
// to tell it whom to trust.
//
// The positive control at the end is load-bearing: without it these assertions
// would also pass against a loader that reads nothing from anywhere, which is M18.
func TestScanIssuesRefusesAHostileEnvUnderCI(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // empty config home
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv(scanEnvBlessLogin, "attacker:999")
	t.Setenv(scanEnvTrustedLogins, "attacker:999")
	t.Setenv(scanEnvHumanLoginMap, "alex:attacker")
	t.Cleanup(func() { scanSetToolClass(scanClassWrite) })

	acting := scanLoadConfig(scanClassForMode(true))
	if acting.Configured() {
		t.Fatalf("--scan-issues loaded a CONFIGURED roster from the environment inside a runner "+
			"(source %q). It is the acting mode: it creates work items from issue text, and the "+
			"split-by-action-class ruling makes the environment a refused route for it", acting.Source)
	}
	if acting.HumanLogins["alex"] == "attacker" {
		t.Fatal("the environment's human-login map reached --scan-issues: `alex` now resolves to " +
			"`attacker`, so an attacker-authored Verified cell would clear the verifier floor")
	}

	reporting := scanLoadConfig(scanClassForMode(false))
	if !reporting.Configured() {
		t.Fatalf("the reporting class did not read the environment (%v) — the negative assertions "+
			"above prove nothing unless this transport demonstrably works. This is also M18: a "+
			"scanClassForMode that always answers write makes --lint blind in the consumer's CI",
			reporting.Problems)
	}
}

// TestCIHumanLoginMapResidualBoundary pins the ONE decision the environment can
// widen, and the fact that it widens nothing else.
//
// The package comment on rosterconfig.go used to claim that the ci-eligible modes
// "grant nothing and an empty map is strictly stricter, so the environment cannot
// widen a decision through them". The premise is true and the conclusion is false:
// ASSAY_HUMAN_LOGIN_MAP is consulted only to ACCEPT, and its gate-clearing consumers
// never consult Configured(), so a NON-EMPTY map arriving over the CI transport is
// strictly wider than no map.
//
// The map-widened consumers are now TWO: authorizedByVerifiedHuman (the register
// human-authorisation key) and corroborateStamps (--corroborate). The verifier floor
// USED to be a third, but the leaver-principle change made it clear on human-login
// SHAPE rather than live-map membership (a historical stamp must not be red-lined when
// a human leaves the roster), so the map no longer widens it — that is asserted below
// as a property, not a widening.
//
// This test exists so that residual is WRITTEN DOWN AND EXECUTABLE rather than
// merely absent from a comment. It asserts both halves:
//
//	POSITIVE  the residual is real and deliberate — both map-widened gates do clear.
//	          If a future change closes the residual, this half goes red and the
//	          comment must be updated with it. Closing it is not a free win: it
//	          re-breaks CI lint wherever an adopter sets only this one variable.
//	NEGATIVE  the residual is BOUNDED — the map admits no repo, blesses nobody, and
//	          makes nobody a trusted author. If a future change lets the environment
//	          reach any of those, this half goes red.
func TestCIHumanLoginMapResidualBoundary(t *testing.T) {
	// Exactly the shape a single-variable adopter is in: ONE variable set,
	// no config home, inside a runner.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv(scanEnvHumanLoginMap, "alex:attacker")
	scanSetToolClass(scanClassForMode(false))
	t.Cleanup(func() { scanSetToolClass(scanClassWrite); scanReloadConfig() })
	scanReloadConfig()

	cfg := scanEffectiveConfig()

	// Control: the transport must demonstrably work, or every assertion below is
	// vacuous against a loader that reads nothing from anywhere.
	if cfg.HumanLogins["alex"] != "attacker" {
		t.Fatalf("the CI transport did not deliver the human-login map (source %q, problems %v) — "+
			"the assertions below would pass against a loader that reads nothing", cfg.Source, cfg.Problems)
	}
	// The roster is NOT configured: one variable is not a roster.
	if cfg.Configured() {
		t.Fatalf("a lone %s reported a CONFIGURED roster", scanEnvHumanLoginMap)
	}

	// ---- POSITIVE half: the residual is real ------------------------------
	// NOTE ON THE VERIFIER FLOOR: it IS a map-widened gate. The floor clears a
	// `human:<name>` token only when the name was EVER a confirmed human — in the
	// CURRENT map (here, delivered over the CI transport) or the FORMER-humans map
	// (not set in this test) — and REJECTS a never-confirmed name. So a mapped
	// `human:alex` clears where an unmapped `human:nobody` FAILS: the env-delivered map
	// widens the floor decision, exactly as it widens the two gates below. (This is the
	// tightening back toward main that #104's shape-only form had dropped; the floor's
	// malformed-token boundary is pinned in attribution_test.go.)
	if _, failed := verifierFloorFailure("2026-07-18 human:alex"); failed {
		t.Error("a CURRENTLY-mapped human must clear the verifier floor — the env-delivered map is the " +
			"ACCEPT-widening; if this fails, the floor is not consulting the map at all")
	}
	if _, failed := verifierFloorFailure("2026-07-18 human:nobody"); !failed {
		t.Error("an UNMAPPED, never-confirmed human login must FAIL the floor — it is in neither the " +
			"current nor the former map, so the map is what widens `human:alex` clear here, not shape")
	}
	// Gate 1: the register human-authorisation key.
	if !authorizedByVerifiedHuman([]byte("---\nauthorized-by: human:alex\n---\n")) {
		t.Error("the residual is documented as GRANTING authorizedByVerifiedHuman for a mapped " +
			"name, but it refused — update the RESIDUAL note if this was closed deliberately")
	}
	if authorizedByVerifiedHuman([]byte("---\nauthorized-by: human:nobody\n---\n")) {
		t.Error("an UNMAPPED name satisfied authorizedByVerifiedHuman — the gate is not consulting " +
			"the map at all")
	}
	// Gate 2: `--corroborate`. This row exists because the residual note originally
	// enumerated only the two gates above and dismissed this one as "not widening on
	// its own — a mapping only names a login whose APPROVED review must then be found
	// on the PR". The barrier is real but it does not bind: whoever sets the map also
	// chooses which login the name resolves to, so ANY account already carrying an
	// approval-shaped comment on the PR satisfies it — including a shared agent
	// account, which is the identity a `human:<name>` stamp exists to distinguish from
	// a human. The flip is an exit-status flip: runCorroborate returns 1 only when
	// anyMissing.
	prData := &ghPRData{Comments: []ghComment{
		{Author: ghAuthor{Login: "attacker"}, Body: "lgtm", URL: "https://example.invalid/c/1"},
	}}
	mapped := corroborateStamps([]stamp{{Name: "alex"}}, prData, "example-org/example", 1)
	if len(mapped) != 1 || mapped[0].Verdict != verdictCorroborated {
		t.Errorf("the residual is documented as flipping --corroborate to CORROBORATED for a "+
			"mapped name whose login commented an approval, but it did not: %+v\n\nIf this was "+
			"closed deliberately, update the RESIDUAL note at the head of rosterconfig.go and the "+
			"consumer enumeration in corroborate.go in the same commit", mapped)
	}
	// The same PR data with an UNMAPPED stamp name stays MISSING — so it is the map
	// doing the widening here, not the comment.
	unmapped := corroborateStamps([]stamp{{Name: "nobody"}}, prData, "example-org/example", 1)
	if len(unmapped) != 1 || unmapped[0].Verdict != verdictMissing {
		t.Errorf("an UNMAPPED stamp name was corroborated (%+v) — then the map is not what flips "+
			"this verdict and this row is measuring the wrong thing", unmapped)
	}

	// ---- NEGATIVE half: the residual is bounded ---------------------------
	// The mapped login is a NAME, not a grant. None of these may follow from it.
	for _, login := range []string{"attacker", "alex", "attacker[bot]", "app/attacker"} {
		if trustedAuthor(login) {
			t.Errorf("the human-login map made %q a TRUSTED AUTHOR — the map names a human, it does "+
				"not admit one to the board", login)
		}
	}
	if isBlessAuthorityID("attacker", 999) || isBlessAuthorityID("alex", 999) {
		t.Error("the human-login map produced a blessing authority")
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("the human-login map admitted repos %v — it must admit none", cfg.Repos)
	}
	if cfg.Bless.Login != "" {
		t.Errorf("the human-login map produced a live blessing identity %q", cfg.Bless.Login)
	}
	if len(cfg.Humans) != 0 || len(cfg.Bots) != 0 || len(cfg.Logins) != 0 {
		t.Errorf("the human-login map populated the trust roster: humans=%v bots=%v logins=%v",
			cfg.Humans, cfg.Bots, cfg.Logins)
	}
}

// TestMainWiresClassAndEchoOnce is the M17d guard, and the statusgen twin of
// deskkit's echocoverage_test.go. statusgen is a separate module, so the deskkit
// guard — which enumerates the desk-tools commands — has never covered this main.
//
// A source-text control is the honest instrument: the property is "these calls
// exist in main(), before anything can read the roster", and there is no observable
// behaviour to drive from a unit test without executing the binary. It is paired
// with TestScanClassForModeSplitsByAction above, which pins what the class call
// MEANS, and with the behavioural echo tests that pin what the echo SAYS.
func TestMainWiresClassAndEchoOnce(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("cannot read statusgen/main.go: %v", err)
	}
	text := string(src)

	for _, want := range []struct {
		call string
		why  string
	}{
		{
			call: "scanSetToolClass(scanClassForMode(",
			why: "main() must resolve the roster class FROM THE INVOKED MODE. Without this " +
				"call scanCfgClass keeps its zero value (scanClassWrite) and every invocation " +
				"is file-only — which is correctness finding 1: `statusgen --lint` in the " +
				"consumer's CI, where there is no config home and never will be, reads no " +
				"roster at all and the verifier floor fails on briefs the PR never touched",
		},
		{
			call: "scanEchoEffectiveConfig(",
			why: "main() must echo the effective roster ONCE per run, for EVERY mode (P3). " +
				"It used to be echoed only from runScanIssues, so --lint — the mode that " +
				"actually runs in the consumer's CI — printed nothing about the configuration " +
				"it was judging with. Mutation M17d (deleting this call) survived the whole " +
				"suite before it had a test",
		},
	} {
		if !strings.Contains(text, want.call) {
			t.Errorf("statusgen/main.go no longer calls %s — %s", want.call, want.why)
		}
	}

	// Ordering is the other half of the property: the class must be set BEFORE the
	// echo, because scanEffectiveConfig caches on first use and the echo is a read.
	// An echo placed above the class call would print — and pin the run to — the
	// zero-value class whatever the mode.
	classAt := strings.Index(text, "scanSetToolClass(scanClassForMode(")
	echoAt := strings.Index(text, "scanEchoEffectiveConfig(")
	if classAt >= 0 && echoAt >= 0 && classAt > echoAt {
		t.Error("statusgen/main.go echoes the effective config BEFORE it sets the tool class. " +
			"scanEffectiveConfig caches on first use, so the echo would load — and freeze — the " +
			"zero-value write class, and the later scanSetToolClass would be ignored: --lint " +
			"would silently run file-only in CI while printing a roster line that says so too " +
			"quietly to notice")
	}
}
