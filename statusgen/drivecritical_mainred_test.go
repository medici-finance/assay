package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Row 5 of statusgen/05 — the three arms that were not live on merged main:
// main-red (injected input), stamped-security (authority set from roster config),
// and reviewer-finding (reaches the board through the finding's control:). The
// subtests are registered under TestDriveCriticalTierNeverBuried so the brief's own
// Verify row 5 command exercises them.

// withMainHealth installs a main-health input for one test and restores it.
func withMainHealth(t *testing.T, mh MainHealth) {
	t.Helper()
	old := activeMainHealth
	activeMainHealth = mh
	t.Cleanup(func() { activeMainHealth = old })
}

// withStampConfig installs the roster-configured authority state for one test.
func withStampConfig(t *testing.T, set bool, auths ...string) {
	t.Helper()
	oldM, oldSet := criticalStampAuthorities, criticalStampAuthoritiesSet
	m := map[string]bool{}
	for _, a := range auths {
		m[a] = true
	}
	criticalStampAuthorities, criticalStampAuthoritiesSet = m, set
	t.Cleanup(func() { criticalStampAuthorities, criticalStampAuthoritiesSet = oldM, oldSet })
}

// criticalBoard is a fire stream holding one P2 brief (base 1000) and a routine
// P1 stream (base 2000) carried by a SURGE drive (+2500): without the tier the
// surge buries the fire. fire is the brief under test.
func criticalBoard(t *testing.T, fire Brief) (NextUp, []*Stream) {
	t.Helper()
	fs := mkStream("fire", "active", "P2", fire)
	fs.LastTouch = day(0)
	routine := mkStream("routine", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	routine.LastTouch = day(0)
	streams := []*Stream{fs, routine, driveBriefStream("filler", 3)}
	root := t.TempDir()
	makeStreamsDir(t, root)
	writeDrive(t, root, "surge-routine", "declared-by: operator\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: routine\n")
	ds := loadDrives(root, streams, driveTestNow)
	if !ds.applied() {
		t.Fatalf("the surge drive must apply: %+v", ds)
	}
	withDrives(t, ds)
	return nextUp(streams, KnownClaims(map[string]bool{}), nil), streams
}

// assertFireOnTop fails unless fire/01 is critical via arm and ranks above the
// surge-boosted routine pick whose total exceeds it.
func assertFireOnTop(t *testing.T, nu NextUp, arm string) {
	t.Helper()
	iFire, pFire := pickIndex(nu, "fire")
	iRoutine, pRoutine := pickIndex(nu, "routine")
	if pFire == nil || pRoutine == nil {
		t.Fatalf("fire and routine must both be on the board: %+v", nu.Picks)
	}
	if !pFire.CriticalTier || pFire.CriticalArm != arm {
		t.Fatalf("fire/01 must be critical via %q, got critical=%v arm=%q", arm, pFire.CriticalTier, pFire.CriticalArm)
	}
	if pRoutine.Total() <= pFire.Total() {
		t.Fatalf("test premise broken: surge total %d must exceed fire total %d", pRoutine.Total(), pFire.Total())
	}
	if iFire >= iRoutine {
		t.Fatalf("critical fire (idx %d) must rank ABOVE the surge-boosted routine (idx %d)", iFire, iRoutine)
	}
}

func assertFireNotCritical(t *testing.T, nu NextUp, why string) {
	t.Helper()
	_, pFire := pickIndex(nu, "fire")
	if pFire == nil {
		t.Fatalf("fire/01 must be on the board: %+v", nu.Picks)
	}
	if pFire.CriticalTier {
		t.Fatalf("%s: fire/01 must NOT be critical, got arm %q", why, pFire.CriticalArm)
	}
}

const redRef = "example-org/app#77"

// testMainRedFixOutranksSurge: with main injected RED and a fix addressing the
// tracking issue, the fix is critical via main-red and outranks a surge.
func testMainRedFixOutranksSurge(t *testing.T) {
	withFindings(t, nil)
	withMainHealth(t, MainHealth{State: "red", Refs: map[string]bool{redRef: true}})
	nu, _ := criticalBoard(t, Brief{Num: "01", Wave: 0, Status: "todo", IssueRefs: []string{redRef}})
	assertFireOnTop(t, nu, "main-red")
	if nu.MainRedUnknown != "" {
		t.Fatalf("a supplied main-health input must not report could-not-check: %q", nu.MainRedUnknown)
	}
}

// testMainRedNeedsRedAndLinkage: the arm is off when main is green, when the
// brief addresses a different issue, and when the input is absent.
func testMainRedNeedsRedAndLinkage(t *testing.T) {
	withFindings(t, nil)
	fix := Brief{Num: "01", Wave: 0, Status: "todo", IssueRefs: []string{redRef}}
	for _, tc := range []struct {
		name string
		mh   MainHealth
		b    Brief
	}{
		{"green", MainHealth{State: "green"}, fix},
		{"red-but-unlinked", MainHealth{State: "red", Refs: map[string]bool{"example-org/app#78": true}}, fix},
		{"red-but-no-issue-linkage", MainHealth{State: "red", Refs: map[string]bool{redRef: true}}, Brief{Num: "01", Wave: 0, Status: "todo"}},
		{"could-not-check", MainHealth{}, fix},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withMainHealth(t, tc.mh)
			nu, _ := criticalBoard(t, tc.b)
			assertFireNotCritical(t, nu, tc.name)
		})
	}
}

// testMainRedCouldNotCheckIsNamed: with a drive active and no input, the board,
// the --lint NOTICE source and the --next-up JSON all say could-not-check — never a
// silent green. With no drive nothing is added (baseline unchanged).
func testMainRedCouldNotCheckIsNamed(t *testing.T) {
	withFindings(t, nil)
	withMainHealth(t, MainHealth{})
	nu, streams := criticalBoard(t, Brief{Num: "01", Wave: 0, Status: "todo"})
	if nu.MainRedUnknown == "" || nu.MainHealth != "could-not-check" {
		t.Fatalf("drive active + no input must be could-not-check, got health=%q unknown=%q", nu.MainHealth, nu.MainRedUnknown)
	}
	raw, err := json.Marshal(buildDispatchView(nu, streams, "", ClaimSource{Known: true}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"mainHealth":"could-not-check"`) {
		t.Fatalf("the --next-up JSON must carry mainHealth=could-not-check: %s", raw)
	}
	if out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, ""); !strings.Contains(out, "COULD NOT CHECK — main-red arm could not check") {
		t.Fatalf("the board must name the main-red could-not-check:\n%s", out)
	}

	// No drive → no field, no line.
	withDrives(t, DriveSet{})
	nu = nextUp(streams, KnownClaims(map[string]bool{}), nil)
	if nu.MainRedUnknown != "" || nu.MainHealth != "" {
		t.Fatalf("with no drive the main-red state must not be recorded (baseline unchanged): %+v", nu)
	}
}

// testStampedSecurityFromConfig: the authority set is roster configuration. Set
// and listed → the security arm fires and outranks a surge; set but unlisted, or
// UNSET → no grant, and criticalStampNotices names each inert stamp with its reason.
func testStampedSecurityFromConfig(t *testing.T) {
	withFindings(t, nil)
	withMainHealth(t, MainHealth{State: "green"})
	stamped := Brief{Num: "01", Wave: 0, Status: "todo", Reviewed: "2026-09-27 critical-security(sec-authority)"}

	withStampConfig(t, true, "sec-authority")
	nu, streams := criticalBoard(t, stamped)
	assertFireOnTop(t, nu, "security")
	if n := criticalStampNotices(streams); len(n) != 0 {
		t.Fatalf("an honoured stamp must not be reported: %v", n)
	}

	withStampConfig(t, true, "someone-else")
	nu, streams = criticalBoard(t, stamped)
	assertFireNotCritical(t, nu, "unlisted authority")
	if n := criticalStampNotices(streams); len(n) != 1 || !strings.Contains(n[0], "not in the configured critical-stamp authority set") {
		t.Fatalf("an unlisted authority's stamp must be named: %v", n)
	}

	withStampConfig(t, false)
	nu, streams = criticalBoard(t, stamped)
	assertFireNotCritical(t, nu, "unset authority set")
	if n := criticalStampNotices(streams); len(n) != 1 || !strings.Contains(n[0], "could-not-check") || !strings.Contains(n[0], "ASSAY_CRITICAL_STAMP_AUTHORITIES unset") {
		t.Fatalf("with the authority set UNSET each stamp must be reported could-not-check: %v", n)
	}
}

// testReviewerFindingReachesBoardViaControl: the remediation brief a finding names
// in control: is critical on the board; a brief the finding names in affects: is
// StaleRef-excluded from Next-up BY DESIGN (the tier orders eligible picks, it
// never overrides eligibility).
func testReviewerFindingReachesBoardViaControl(t *testing.T) {
	withMainHealth(t, MainHealth{State: "green"})
	withFindings(t, []Finding{{ID: "F-x", Control: "fire/01", Affects: []string{"filler/01"}}})
	nu, streams := criticalBoard(t, Brief{Num: "01", Wave: 0, Status: "todo"})
	assertFireOnTop(t, nu, "reviewer-finding")

	// The affects-named brief: applyFindings stamps StaleRef, and it leaves Next-up.
	applyFindings(streams, activeFindings)
	nu = nextUp(streams, KnownClaims(map[string]bool{}), nil)
	for _, p := range nu.Picks {
		if p.Stream.Name == "filler" && p.Brief.Num == "01" {
			t.Fatalf("an affects-named (StaleRef) brief must stay excluded from Next-up even with a drive active: %+v", p)
		}
	}
	if !strings.Contains(streams[2].Briefs[0].StaleRef, "F-x") {
		t.Fatalf("fixture premise: filler/01 must carry StaleRef from the finding, got %q", streams[2].Briefs[0].StaleRef)
	}
	if _, p := pickIndex(nu, "fire"); p == nil || p.CriticalArm != "reviewer-finding" {
		t.Fatalf("the control-named remediation must remain on the board, critical: %+v", p)
	}
	// A RESOLVED finding lifts nothing.
	if reviewerFindingCritical([]Finding{{ID: "F-x", Control: "fire/01", Resolved: true}}, "fire", "01") {
		t.Fatal("a resolved finding's control must not qualify")
	}
	// The brief-<NN> spelling of control: resolves too; a bare stream never does.
	if !reviewerFindingCritical([]Finding{{ID: "F-y", Control: "fire/brief-01"}}, "fire", "01") {
		t.Fatal("control: fire/brief-01 must resolve to fire/01")
	}
	if reviewerFindingCritical([]Finding{{ID: "F-z", Control: "fire"}}, "fire", "01") {
		t.Fatal("a bare-stream control must never broadcast to a brief")
	}
}

// TestMainHealthParse pins the injected input's grammar: three states, and every
// malformed value refused rather than falling back to either state.
func TestMainHealthParse(t *testing.T) {
	if mh, err := parseMainHealth(""); err != nil || mh.known() {
		t.Fatalf("empty = could-not-check, got %+v %v", mh, err)
	}
	if mh, err := parseMainHealth("green"); err != nil || mh.State != "green" || mh.red() {
		t.Fatalf("green, got %+v %v", mh, err)
	}
	mh, err := parseMainHealth("red:example-org/app#77, example-org/lib#3")
	if err != nil || !mh.red() || !mh.Refs["example-org/app#77"] || !mh.Refs["example-org/lib#3"] {
		t.Fatalf("red with two refs, got %+v %v", mh, err)
	}
	if mh.String() != "red:example-org/app#77,example-org/lib#3" {
		t.Fatalf("String() = %q", mh.String())
	}
	for _, bad := range []string{"red", "red:", "red:#77", "red:app#77", "RED:example-org/app#77", "yes", "unknown"} {
		if _, err := parseMainHealth(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}

// TestIssueRefsWiring: the main-red arm reads Brief.IssueRefs, so pin that both
// producers populate it — a placeholder's own issue, and a brief's `issues:`
// resolved against the stream's `repo:` (and NOT resolved when the stream declares
// no repo — the arm never guesses one).
func TestIssueRefsWiring(t *testing.T) {
	ph := &Placeholder{Path: "x/issue-77.md", Repo: "example-org/app", Issue: 77, Status: "todo"}
	if got := ph.toBrief().IssueRefs; len(got) != 1 || got[0] != "example-org/app#77" {
		t.Fatalf("placeholder IssueRefs = %v", got)
	}
	for _, tc := range []struct {
		repoLine string
		want     []string
	}{
		{"repo: example-org/app\n", []string{"example-org/app#77", "example-org/app#78"}},
		{"", nil},
	} {
		root := t.TempDir()
		sdir := filepath.Join(root, "docs", "streams", "fixes")
		if err := os.MkdirAll(sdir, 0o755); err != nil {
			t.Fatal(err)
		}
		readme := "---\nstream: fixes\nstatus: active\npriority: P1\ntrack: test\n" + tc.repoLine + "---\n# Fixes\n" +
			"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
			"|---|-------|------|--------|--------|----------|----------|\n" +
			"| 01 | [One](./brief-01.md) | 0 | S | todo | — | — |\n"
		brief := "---\nbrief: fixes/01\ntitle: Brief One\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\n" +
			"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nissues: [77, 78]\nschema: brief-v1\n" +
			"authored: 2026-07-08\nsources: [test]\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(sdir, "README.md"), []byte(readme), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sdir, "brief-01.md"), []byte(brief), 0o644); err != nil {
			t.Fatal(err)
		}
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		checkBriefFiles(streams, streams)
		got := streams[0].Briefs[0].IssueRefs
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("repo line %q: IssueRefs = %v, want %v", tc.repoLine, got, tc.want)
		}
	}
}

// TestCriticalStampAuthoritiesConfig pins the roster key end to end: a valid list
// is parsed, echoed and wired; UNSET is an explicit state distinct from a list; a
// malformed entry refuses the whole configuration (and a refused configuration
// wires NOTHING, never a half-parsed set).
func TestCriticalStampAuthoritiesConfig(t *testing.T) {
	base := map[string]string{scanEnvBlessLogin: "ada:1001", scanEnvTrustedLogins: "ada:1001"}
	with := func(v string) map[string]string {
		m := map[string]string{}
		for k, x := range base {
			m[k] = x
		}
		if v != "" {
			m[scanEnvCriticalStampAuthorities] = v
		}
		return m
	}
	t.Cleanup(func() { wireCriticalStampAuthorities(scanConfig{}) })

	cfg := scanParseConfig(scanClassReadOnly, "test", with("sec-authority, review.desk"))
	if len(cfg.Problems) != 0 || !cfg.CriticalStampAuthoritiesSet || strings.Join(cfg.CriticalStampAuthorities, ",") != "review.desk,sec-authority" {
		t.Fatalf("valid list: %+v problems=%v", cfg.CriticalStampAuthorities, cfg.Problems)
	}
	if !strings.Contains(strings.Join(cfg.EffectiveConfigLines(), "\n"), "ASSAY_CRITICAL_STAMP_AUTHORITIES=review.desk,sec-authority") {
		t.Fatalf("the effective value must be echoed: %v", cfg.EffectiveConfigLines())
	}
	wireCriticalStampAuthorities(cfg)
	if !criticalStampAuthoritiesSet || !criticalStampAuthorized("sec-authority") || criticalStampAuthorized("other") {
		t.Fatal("a configured set must wire exactly its listed authorities")
	}

	cfg = scanParseConfig(scanClassReadOnly, "test", with(""))
	if cfg.CriticalStampAuthoritiesSet || !strings.Contains(strings.Join(cfg.EffectiveConfigLines(), "\n"), "ASSAY_CRITICAL_STAMP_AUTHORITIES=(unset") {
		t.Fatalf("unset must be an explicit, echoed state: %v", cfg.EffectiveConfigLines())
	}
	wireCriticalStampAuthorities(cfg)
	if criticalStampAuthoritiesSet || criticalStampAuthorized("sec-authority") {
		t.Fatal("unset must wire an empty, UNSET authority set")
	}

	cfg = scanParseConfig(scanClassReadOnly, "test", with("ok, bad(authority)"))
	if len(cfg.Problems) == 0 {
		t.Fatal("a malformed authority must refuse the configuration")
	}
	wireCriticalStampAuthorities(cfg)
	if criticalStampAuthoritiesSet || criticalStampAuthorized("ok") {
		t.Fatal("a refused configuration must wire nothing")
	}
}
