package main

// statusgen's half of the P1–P4 property suite (assay-selfcontain/05).
//
// The test NAMES here match tools/desk/internal/deskkit's deliberately: the
// brief's Verify rows run one `-run` token across BOTH packages and pin the
// `--- PASS:` count at exactly 2, so a test that exists in only one module cannot
// half-pass. The two modules share no code, so the twin's load path has to be
// exercised in its own right — and it is the half most likely to be skipped.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// couplingVectorPath is the SHARED cross-tree vector file: one roster, one
// expectation table, both readers. It lives here so THIS module reads it
// in-module; deskkit reads the same file at
// ../../../../statusgen/testdata/roster_coupling.json, and the coupling guard in
// that package asserts this file still names it, so a rename must happen in both
// trees or the guard fires.
const couplingVectorPath = "testdata/roster_coupling.json"

type couplingVectors struct {
	Roster map[string]string `json:"roster"`
	Cases  []struct {
		Why            string `json:"why"`
		Login          string `json:"login"`
		ID             int64  `json:"id"`
		TrustedAuthor  bool   `json:"trustedAuthor"`
		TrustedContent bool   `json:"trustedContent"`
		MayBless       bool   `json:"mayBless"`
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

// scanWithRoster installs an explicit roster into a private config home for one
// test, through the REAL loader (file, permissions, parsing).
func scanWithRoster(t *testing.T, vals map[string]string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + vals[k] + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	scanReloadConfig()
	t.Cleanup(scanReloadConfig)
	return home
}

func scanWithNoRoster(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	scanReloadConfig()
	t.Cleanup(scanReloadConfig)
	return home
}

func scanMediciRoster() map[string]string {
	return map[string]string{
		scanEnvBlessLogin:      "ada:20959",
		scanEnvTrustedLogins:   "ada:20959,shared-agent:2002",
		scanEnvTrustedBotSlugs: "reviewer=assay-reviewer-app:306477694,worker=assay-worker-app:306480234",
		scanEnvHumanLoginMap:   "ian:ada",
	}
}

// TestRosterCouplingVectors is the twin half of deskkit's
// TestScanIssuesTrustGateEnforced: the SAME roster and the SAME expectation table,
// through THIS module's independent reader. If the two duplicated readers drift,
// one of the two halves goes red.
func TestRosterCouplingVectors(t *testing.T) {
	raw, err := os.ReadFile(couplingVectorPath)
	if err != nil {
		t.Fatalf("cannot read the shared cross-tree roster vectors at %s: %v — this file is the binding "+
			"that keeps statusgen's duplicated reader honest; if desk-tools moved, re-point this test, "+
			"do NOT delete it", couplingVectorPath, err)
	}
	var vec couplingVectors
	if err := json.Unmarshal(raw, &vec); err != nil {
		t.Fatalf("%s does not parse: %v", couplingVectorPath, err)
	}
	if len(vec.Cases) == 0 {
		t.Fatalf("%s carries no cases — an empty vector file is a coupling guard that cannot fail", couplingVectorPath)
	}

	scanWithRoster(t, vec.Roster)
	if !scanEffectiveConfig().Configured() {
		t.Fatalf("the shared coupling roster did not load into statusgen's reader: %v",
			scanEffectiveConfig().Problems)
	}

	blessers := map[string]bool{}
	for _, c := range vec.Cases {
		if got := trustedAuthor(c.Login); got != c.TrustedAuthor {
			t.Errorf("trustedAuthor(%q) = %t, want %t — %s", c.Login, got, c.TrustedAuthor, c.Why)
		}
		if got := trustedAuthorID(c.Login, c.ID); got != c.TrustedContent {
			t.Errorf("trustedAuthorID(%q, %d) = %t, want %t — %s", c.Login, c.ID, got, c.TrustedContent, c.Why)
		}
		if got := isBlessAuthorityID(c.Login, c.ID); got != c.MayBless {
			t.Errorf("isBlessAuthorityID(%q, %d) = %t, want %t — %s", c.Login, c.ID, got, c.MayBless, c.Why)
		}
		if isBlessAuthorityID(c.Login, c.ID) {
			blessers[strings.ToLower(c.Login)] = true
		}
	}
	if len(blessers) != 1 {
		t.Errorf("%d DISTINCT logins may bless under the shared roster (%v), want exactly 1", len(blessers), blessers)
	}
}

// TestTrustUnconfiguredFailsClosed is P1 in this tree.
func TestTrustUnconfiguredFailsClosed(t *testing.T) {
	scanWithNoRoster(t)
	for _, login := range []string{"ada", "shared-agent", "assay-reviewer-app[bot]", "app/assay-desk-app", "ada", ""} {
		if trustedAuthor(login) {
			t.Errorf("trustedAuthor(%q) = true with NO roster configured — unset must trust nobody", login)
		}
		if trustedAuthorID(login, 20959) {
			t.Errorf("trustedAuthorID(%q, 20959) = true with NO roster configured", login)
		}
		if isBlessAuthorityID(login, 20959) {
			t.Errorf("isBlessAuthorityID(%q, 20959) = true with NO roster configured", login)
		}
	}
	err := scanRosterUnconfiguredError()
	if err == nil {
		t.Fatal("scanRosterUnconfiguredError() = nil with NO roster — --scan-issues would scan ungated")
	}
	if !strings.Contains(err.Error(), "NOT CONFIGURED") {
		t.Errorf("the refusal does not say the roster is unconfigured:\n%v", err)
	}
	// The blessing evaluator itself must refuse, not merely answer "unblessed":
	// a quiet false would look identical to a real quarantine decision.
	if _, berr := evalIssueBlessing([]byte(`{"data":{"repository":{"issue":{"lastEditedAt":"","comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`)); berr == nil {
		t.Error("evalIssueBlessing returned no error with NO roster configured — an unconfigured " +
			"blessing authority must be an error, not a silent 'not blessed'")
	}
}

// TestCorroborationUnconfiguredIsMissing is P1 for C7. An unconfigured human-login
// map keeps meaning MISSING-CORROBORATION — it must never start PASSING
// corroboration checks when unset.
func TestCorroborationUnconfiguredIsMissing(t *testing.T) {
	scanWithNoRoster(t)
	if login, ok := HumanLogin("ian"); ok {
		t.Errorf("HumanLogin(ian) resolved to %q with NO map configured — an unknown name has no "+
			"GitHub login to check, so it is MISSING-CORROBORATION by definition", login)
	}
	if _, ok := HumanLogin("anybody"); ok {
		t.Error("HumanLogin resolved an arbitrary name with NO map configured")
	}
	// The verifier floor is the surface where a resolvable name WAIVES a check, so
	// an unconfigured map must make it stricter, never looser.
	if reason, failed := verifierFloorFailure("2026-08-04 human:ian"); !failed {
		t.Errorf("the verifier floor accepted a human runner token with NO map configured (reason %q) — "+
			"an unverifiable human token does not satisfy the floor", reason)
	}

	// Positive control: with the map configured, the same token clears.
	scanWithRoster(t, scanMediciRoster())
	if login, ok := HumanLogin("ian"); !ok || login != "ada" {
		t.Errorf("HumanLogin(ian) = %q,%t with the map configured, want ada,true — the negative "+
			"cases above would otherwise prove nothing", login, ok)
	}
	if _, failed := verifierFloorFailure("2026-08-04 human:ian"); failed {
		t.Error("the verifier floor rejected a CONFIGURED human runner token")
	}
}

// TestRosterIsNotSettableFromTheWorkTree is P2 in this tree.
func TestRosterIsNotSettableFromTheWorkTree(t *testing.T) {
	work := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scanWithNoRoster(t)

	verdict := func() string {
		var b bytes.Buffer
		for _, l := range scanEffectiveConfig().EffectiveConfigLines() {
			b.WriteString(l + "\n")
		}
		for _, login := range []string{"attacker", "ada", "ada"} {
			b.WriteString(login)
			if trustedAuthor(login) {
				b.WriteString(":trusted")
			} else {
				b.WriteString(":untrusted")
			}
			if isBlessAuthorityID(login, 999) {
				b.WriteString(":may-bless")
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	before := verdict()
	payload := []byte(scanEnvBlessLogin + "=attacker:999\n" +
		scanEnvTrustedLogins + "=attacker:999\n" +
		scanEnvHumanLoginMap + "=ian:attacker\n")
	for _, rel := range []string{
		".assay-trust", "assay.yml",
		".github/.assay-trust", ".github/assay.yml",
		".assay/.assay-trust", ".assay/assay.yml",
		"roster.env", ".config/assay/roster.env",
	} {
		p := filepath.Join(work, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	scanReloadConfig()

	if after := verdict(); after != before {
		t.Fatalf("planting config files IN THE WORK TREE changed statusgen's verdict — the roster is "+
			"reachable from a diff.\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if trustedAuthor("attacker") {
		t.Fatal("an in-tree config file named an attacker and statusgen's loader accepted it")
	}
	if login, ok := HumanLogin("ian"); ok {
		t.Fatalf("an in-tree config file supplied a human-login mapping (ian -> %q)", login)
	}
}

// TestWriteToolsRefuseEnvRoster is the 2026-08-04 split-by-action-class ruling in
// this tree. --scan-issues is WRITE class — it creates placeholder files that
// reach Next-up, where a worker acts on them — so the environment must not reach
// its roster at all.
func TestWriteToolsRefuseEnvRoster(t *testing.T) {
	scanWithNoRoster(t)
	t.Setenv(scanEnvBlessLogin, "attacker:999")
	t.Setenv(scanEnvTrustedLogins, "attacker:999,ada:20959")
	t.Setenv(scanEnvTrustedBotSlugs, "attacker-app:999")
	t.Setenv(scanEnvHumanLoginMap, "ian:attacker")
	scanReloadConfig()

	cfg := scanEffectiveConfig()
	if cfg.Class != scanClassWrite {
		t.Fatalf("statusgen's default tool class is %v, want the write class", cfg.Class)
	}
	if cfg.Configured() {
		t.Fatal("statusgen's write-class load consulted the ENVIRONMENT — --scan-issues WRITES work items, " +
			"so a steered local invocation must not be able to widen its roster through an env var")
	}
	for _, login := range []string{"attacker", "ada"} {
		if trustedAuthor(login) {
			t.Errorf("statusgen trusted %q from an environment variable", login)
		}
	}
	if login, ok := HumanLogin("ian"); ok {
		t.Errorf("statusgen took the human-login map from an environment variable (ian -> %q)", login)
	}
	if scanRosterUnconfiguredError() == nil {
		t.Error("with only an env roster present, --scan-issues must report UNCONFIGURED")
	}
	if ro := scanLoadConfig(scanClassReadOnly); !ro.Configured() {
		t.Error("scanClassReadOnly did not read the environment — the classes have collapsed, and this " +
			"test would then pass for the wrong reason")
	}
}

// TestConfigHomePermissionsEnforced is the sshd rule in this tree.
func TestConfigHomePermissionsEnforced(t *testing.T) {
	home := scanWithRoster(t, scanMediciRoster())
	if !scanEffectiveConfig().Configured() {
		t.Fatal("the fixture roster did not load at 0600/0700 — the positive control is broken")
	}
	dir := filepath.Join(home, ".config", "assay")
	file := filepath.Join(dir, "roster.env")

	// A flat loop, NOT subtests: the brief's Verify row pins the `--- PASS:` count
	// for this test name at exactly 2 (one per module), and a t.Run subtest would
	// print its own line and inflate it past the pin.
	for _, tc := range []struct {
		name string
		path string
		mode os.FileMode
	}{
		{"group-writable file", file, 0o660},
		{"world-writable file", file, 0o606},
		{"group-writable directory", dir, 0o770},
		{"world-writable directory", dir, 0o707},
	} {
		if err := os.Chmod(tc.path, tc.mode); err != nil {
			t.Fatal(err)
		}
		scanReloadConfig()
		cfg := scanEffectiveConfig()
		if cfg.Configured() {
			t.Errorf("statusgen accepted a roster from a %s", tc.name)
		}
		if !strings.Contains(strings.Join(cfg.Problems, " "), "writable") {
			t.Errorf("the %s refusal does not name the permission problem: %v", tc.name, cfg.Problems)
		}
		_ = os.Chmod(dir, 0o700)
		_ = os.Chmod(file, 0o600)
		scanReloadConfig()
	}

	_ = os.Chmod(dir, 0o700)
	_ = os.Chmod(file, 0o600)
	scanReloadConfig()
	if !scanEffectiveConfig().Configured() {
		t.Fatal("the roster did not reload after permissions were corrected")
	}
}

// TestBlessLoginRequiresID, TestBlessLoginRejectsMultipleValues and
// TestBlessLoginRejectsBotAccount are the three extra bless-login format rules,
// bound in THIS tree as well: the twin must not accept a shape deskkit refuses.
func TestBlessLoginRequiresID(t *testing.T) {
	r := scanMediciRoster()
	r[scanEnvBlessLogin] = "ada"
	scanWithRoster(t, r)
	cfg := scanEffectiveConfig()
	if cfg.Configured() {
		t.Fatal("statusgen accepted a bare-login ASSAY_BLESS_LOGIN — the id is mandatory for the blessing authority")
	}
	if isBlessAuthorityID("ada", 20959) {
		t.Error("a bare-login blessing authority was accepted")
	}
	if !strings.Contains(strings.Join(cfg.Problems, " "), "numeric id") {
		t.Errorf("the refusal does not say the id is missing: %v", cfg.Problems)
	}
	if trustedAuthor("shared-agent") {
		t.Error("a refused bless login left the rest of the roster live — validation failure must " +
			"collapse the whole configuration")
	}
}

func TestBlessLoginRejectsMultipleValues(t *testing.T) {
	for _, raw := range []string{
		"ada:20959,attacker:999",
		"ada:20959;attacker:999",
		"ada:20959 attacker:999",
	} {
		r := scanMediciRoster()
		r[scanEnvBlessLogin] = raw
		scanWithRoster(t, r)
		if scanEffectiveConfig().Configured() {
			t.Errorf("statusgen accepted ASSAY_BLESS_LOGIN=%q — it is a SINGLE identity, never a list", raw)
		}
		if isBlessAuthorityID("ada", 20959) {
			t.Errorf("ASSAY_BLESS_LOGIN=%q blessed the FIRST entry of a list", raw)
		}
		if isBlessAuthorityID("attacker", 999) {
			t.Errorf("ASSAY_BLESS_LOGIN=%q blessed a second entry of a list", raw)
		}
	}
}

func TestBlessLoginRejectsBotAccount(t *testing.T) {
	for _, raw := range []string{
		"assay-desk-app[bot]:306483193",
		"app/assay-desk-app:306483193",
		"some-app:12345",
		"some-bot:12345",
		"assay-reviewer-app:306477694", // present in the configured bot-slug set
	} {
		r := scanMediciRoster()
		r[scanEnvBlessLogin] = raw
		scanWithRoster(t, r)
		if scanEffectiveConfig().Configured() {
			t.Errorf("statusgen accepted ASSAY_BLESS_LOGIN=%q — the blessing has to be a HUMAN act", raw)
		}
	}
}

// TestHumanLoginMapRejectsBotAccount pins the login-half validation added so the
// human-login map cannot point a `human:<name>` token at a bot/App/shared-agent
// identity. The three gates the map clears (verifier floor, register human-auth,
// --corroborate) exist to establish that a HUMAN acted, so a bot-shaped value must
// collapse the whole configuration exactly as ASSAY_BLESS_LOGIN does.
func TestHumanLoginMapRejectsBotAccount(t *testing.T) {
	// Anti-vacuity control: the golden roster's well-formed map is live.
	scanWithRoster(t, scanMediciRoster())
	if !scanEffectiveConfig().Configured() {
		t.Fatal("control: the golden roster is not configured — the bot-shaped cases below prove nothing")
	}
	if _, ok := HumanLogin("ian"); !ok {
		t.Fatal("control: HumanLogin(ian) is not known in the golden roster")
	}

	for _, raw := range []string{
		"ian:assay-desk-app[bot]",
		"ian:app/assay-desk-app",
		"ian:some-app",
		"ian:some-bot",
	} {
		r := scanMediciRoster()
		r[scanEnvHumanLoginMap] = raw
		scanWithRoster(t, r)
		if scanEffectiveConfig().Configured() {
			t.Errorf("statusgen accepted ASSAY_HUMAN_LOGIN_MAP=%q — a bot/App value must not resolve a human token", raw)
		}
		if _, ok := HumanLogin("ian"); ok {
			t.Errorf("ASSAY_HUMAN_LOGIN_MAP=%q produced a live human mapping for ian", raw)
		}
	}
}

// TestMalformedIDNeverDegradesToLoginOnlyTrust is the twin of deskkit's test of the
// same name, bound in THIS tree as well because scanSplitIdentity is a duplicated
// reader: the rule "a typo'd id must never degrade to login-only trust" was stated
// in both copies and tested in neither, and the widening mutation
// (`return l, 0, true`) left BOTH suites green.
func TestMalformedIDNeverDegradesToLoginOnlyTrust(t *testing.T) {
	// Anti-vacuity control.
	scanWithRoster(t, scanMediciRoster())
	if !scanEffectiveConfig().Configured() || !trustedAuthor("ada") {
		t.Fatal("control: the well-formed golden roster is not live — the malformed cases below prove nothing")
	}

	for _, bad := range []string{"2O959", "20959.", "+20959x"} {
		r := scanMediciRoster()
		r[scanEnvTrustedLogins] = "ada:" + bad + ",shared-agent:2002"
		scanWithRoster(t, r)

		cfg := scanEffectiveConfig()
		if cfg.Configured() {
			t.Errorf("statusgen accepted ASSAY_TRUSTED_LOGINS with the unparseable id %q", bad)
		}
		if trustedAuthor("ada") {
			t.Errorf("id %q degraded to login-only trust in statusgen: trustedAuthor(\"ada\") is true "+
				"with no id ever checked", bad)
		}
		if trustedAuthor("shared-agent") {
			t.Errorf("id %q left the rest of the roster live — the collapse must be whole-config", bad)
		}
		if !strings.Contains(strings.Join(cfg.Problems, " "), "cannot parse entry") {
			t.Errorf("the refusal for id %q does not name the unparseable entry: %v", bad, cfg.Problems)
		}
	}

	// Bot slugs, both rendered forms.
	r := scanMediciRoster()
	r[scanEnvTrustedBotSlugs] = "worker=assay-worker-app:3O6480234"
	scanWithRoster(t, r)
	if scanEffectiveConfig().Configured() {
		t.Error("statusgen accepted ASSAY_TRUSTED_BOT_SLUGS with an unparseable id")
	}
	for _, rendering := range []string{"assay-worker-app[bot]", "app/assay-worker-app"} {
		if trustedAuthor(rendering) {
			t.Errorf("an unparseable bot id degraded to login-only trust: trustedAuthor(%q) is true", rendering)
		}
	}

	// The blessing authority: pin the refusal that the PARSE rule owns, not the
	// mandatory-id branch that catches it one step later by luck.
	r = scanMediciRoster()
	r[scanEnvBlessLogin] = "ada:2O959"
	scanWithRoster(t, r)
	cfg := scanEffectiveConfig()
	if cfg.Configured() {
		t.Error("statusgen accepted ASSAY_BLESS_LOGIN with an unparseable id")
	}
	if isBlessAuthorityID("ada", 20959) {
		t.Error("an unparseable id produced a live blessing authority in statusgen")
	}
	if !strings.Contains(strings.Join(cfg.Problems, " "), "cannot be parsed") {
		t.Errorf("the bless-login refusal did not come from the PARSE rule: %v", cfg.Problems)
	}
}

// TestMalformedOwnedRepoSlugRefusesWholeConfig pins the malformed-slug refusal
// path for the two owned-repo roster keys this PR adds (ASSAY_HOME_REPO and
// ASSAY_SCAN_REPOS). The generic malformed-config machinery is exercised
// elsewhere, but the new validSlug branch these two keys go through had no
// dedicated control (review of #600). A half-parsed repo roster is the
// configured-but-wrong shape the design exists to prevent, so a bad slug on
// either key must collapse the WHOLE configuration — not silently drop the key
// and leave the trust roster live.
func TestMalformedOwnedRepoSlugRefusesWholeConfig(t *testing.T) {
	// Anti-vacuity control: the golden roster (no owned-repo keys set) is live,
	// so the refusals below are proved against a config that otherwise loads.
	scanWithRoster(t, scanMediciRoster())
	if !scanEffectiveConfig().Configured() || !trustedAuthor("ada") {
		t.Fatal("control: the well-formed golden roster is not live — the malformed cases below prove nothing")
	}

	// Each bad value violates validSlug (needs exactly one '/', no leading/
	// trailing '/'). For the list-valued ASSAY_SCAN_REPOS a single bad entry
	// among good ones must still collapse the whole config.
	cases := []struct {
		key, val string
	}{
		{scanEnvHomeRepo, "no-slash"},
		{scanEnvHomeRepo, "too/many/slashes"},
		{scanEnvHomeRepo, "/leading"},
		{scanEnvHomeRepo, "trailing/"},
		{scanEnvScanRepos, "no-slash"},
		{scanEnvScanRepos, "owner/good,bad-entry"},
		{scanEnvScanRepos, "owner/good,too/many/slashes"},
	}
	for _, tc := range cases {
		r := scanMediciRoster()
		r[tc.key] = tc.val
		scanWithRoster(t, r)

		cfg := scanEffectiveConfig()
		if cfg.Configured() {
			t.Errorf("statusgen accepted %s=%q — a malformed owned-repo slug must refuse", tc.key, tc.val)
		}
		if trustedAuthor("ada") {
			t.Errorf("%s=%q left the trust roster live — the collapse must be whole-config", tc.key, tc.val)
		}
		joined := strings.Join(cfg.Problems, " ")
		if !strings.Contains(joined, tc.key) || !strings.Contains(joined, "not an owner/name repo slug") {
			t.Errorf("the refusal for %s=%q does not name the key and the slug problem: %v", tc.key, tc.val, cfg.Problems)
		}
	}
}

// TestCouplingRepoRolesAndRejections is the half of the cross-tree binding that the
// correctness review at bf1244c9 found missing.
//
// The vector originally carried only the three TRUST variables, so the repo-policy
// parse sat structurally OUTSIDE the coupling — and the two readers had in fact
// diverged there: deskkit validated the ci|no-ci / public|private tokens and
// collapsed the whole configuration on a bad one, while this copy stored the token
// unvalidated and reported configured=true. One file, one typo, opposite verdicts.
//
// The REJECTION cases are the ones that actually bind the two validators. An
// acceptance case only proves the readers agree about a good file; a refusal case
// proves they agree about a bad one, which is where they drifted.
//
// deskkit runs the identical table against its own reader.
func TestCouplingRepoRolesAndRejections(t *testing.T) {
	raw, err := os.ReadFile(couplingVectorPath)
	if err != nil {
		t.Fatalf("cannot read the shared cross-tree roster vectors at %s: %v", couplingVectorPath, err)
	}
	var vec couplingVectors
	if err := json.Unmarshal(raw, &vec); err != nil {
		t.Fatalf("%s does not parse: %v", couplingVectorPath, err)
	}
	if len(vec.RepoCases) == 0 || len(vec.RoleCases) == 0 || len(vec.RejectedRosters) == 0 {
		t.Fatalf("%s carries repoCases=%d roleCases=%d rejectedRosters=%d — an empty section is a "+
			"coupling guard that cannot fail", couplingVectorPath,
			len(vec.RepoCases), len(vec.RoleCases), len(vec.RejectedRosters))
	}

	scanWithRoster(t, vec.Roster)
	cfg := scanEffectiveConfig()
	if !cfg.Configured() {
		t.Fatalf("the shared coupling roster did not load: %v", cfg.Problems)
	}

	for _, rc := range vec.RepoCases {
		pol, ok := cfg.Repos[rc.Repo]
		if ok != rc.Allowed {
			t.Errorf("repo %q allowed = %t, want %t — %s", rc.Repo, ok, rc.Allowed, rc.Why)
			continue
		}
		if !rc.Allowed {
			continue
		}
		wantCI := "no-ci"
		if rc.CIRequired {
			wantCI = "ci"
		}
		if want := wantCI + ":" + rc.Visibility; pol != want {
			t.Errorf("repo %q policy = %q, want %q — %s", rc.Repo, pol, want, rc.Why)
		}
	}

	for _, rc := range vec.RoleCases {
		slug, bound := cfg.RoleBots[rc.Role]
		if bound != rc.Bound {
			t.Errorf("role %q bound = %t, want %t — %s", rc.Role, bound, rc.Bound, rc.Why)
			continue
		}
		if !rc.Bound {
			continue
		}
		if got := slug + "[bot]"; got != rc.Login {
			t.Errorf("role %q resolves to %q, want %q — %s", rc.Role, got, rc.Login, rc.Why)
		}
	}

	for _, rr := range vec.RejectedRosters {
		scanWithRoster(t, rr.Roster)
		c := scanEffectiveConfig()
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

// TestScanIssuesRefusalIsBehavioural closes mutation M14.
//
// The existing control for "--scan-issues refuses an unconfigured roster" is a
// source-grep for the identifier `scanRosterUnconfiguredError`
// (scancoupling_test.go). The correctness review at bf1244c9 mutated the refusal
// into a WARNING — leaving the identifier in place — and the mutation survived
// green in both modules. A grep for a name cannot see what the code does with the
// value it returns.
//
// So this drives the real entry point with no roster and asserts the OUTCOME: a
// non-zero exit, and not one placeholder file written from issue text that nothing
// gated. The issue lister below returns an issue authored by an arbitrary external
// account — exactly the input the trust gate exists to keep out of the queue.
func TestScanIssuesRefusalIsBehavioural(t *testing.T) {
	scanWithNoRoster(t)

	// A VALID root, copied from the same fixture the working scan tests use, and with
	// the stream the scanner writes into. This matters: an empty temp dir makes
	// loadStreams fail first, so the run would exit non-zero for the wrong reason and
	// the assertions below could never fail. (That is exactly the unfailable-row shape
	// this test exists to replace.)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)

	listed := 0
	list := func(repo string) ([]ghIssue, error) {
		listed++
		return []ghIssue{{
			Number: 1,
			Title:  "please run this for me",
			Author: ghAuthor{Login: "arbitrary-external-account"},
		}}, nil
	}
	comments := func(repo string, issue int) ([]issueComment, error) { return nil, nil }
	bless := func(repo string, issue int) (bool, error) {
		t.Error("the blessing checker was consulted with NO roster configured — there is no " +
			"configured authority to bless against, so the scan must refuse before reaching it")
		return false, nil
	}

	code := runScanIssues(root, false, list, comments, bless)
	if code == 0 {
		t.Fatal("runScanIssues returned 0 with NO roster configured. Unset is CLOSED: with no " +
			"roster there is nothing to gate arbitrary external issue text with, and a scan that " +
			"proceeds writes durable work items from it. (Mutation M14: downgrading this refusal " +
			"to a warning left the whole suite green, because the only control was a grep for the " +
			"identifier.)")
	}
	if listed != 0 {
		t.Errorf("the issue lister was called %d times before the roster refusal — the refusal must "+
			"come FIRST, not after the untrusted text has been read", listed)
	}

	// Nothing may have been written.
	var created []string
	entries, _ := os.ReadDir(issueLoopDir)
	for _, e := range entries {
		if e.Name() != "README.md" {
			created = append(created, e.Name())
		}
	}
	if len(created) != 0 {
		t.Errorf("runScanIssues wrote %v with no roster configured — a refused scan must create "+
			"no placeholders at all", created)
	}
}
