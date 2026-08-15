package deskkit

// The P1–P4 property tests for the roster/authority configuration.
//
// Each property in the brief is expressed HERE as a test, not as a claim in a
// comment. A refactor that reads the roster from a file inside the repository, or
// that degrades an unset roster to "no filtering", fails one of these.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// goldenRoster is the golden roster fixture this package's tests compile against.
func goldenRoster() map[string]string {
	return map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedLogins:   "ada:2001,shared-agent:2002",
		EnvTrustedBotSlugs: "desk=assay-desk-app:300000001,intake-loop=assay-intake-loop-app:300000002,issue-loop=assay-issue-loop-app:300000003,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006",
	}
}

// ---- P1: unset is CLOSED, not open -------------------------------------------

// TestTrustUnconfiguredFailsClosed is P1. With every roster variable unset,
// TrustedAuthor is false for EVERY input — including the logins this tree used to
// compile in — and the package surfaces a refusal rather than an empty pass.
//
// The failure this catches is not a crash. It is a tool that runs, finds no
// roster, and treats "nothing configured" as "nothing to filter" — which admits
// arbitrary external issue and PR text into an agent's work queue. That is the
// exact exposure the compiled-in roster was introduced to close.
func TestTrustUnconfiguredFailsClosed(t *testing.T) {
	withNoRoster(t)

	for _, login := range []string{
		"ada", "example-org", "assay-reviewer-app[bot]", "app/assay-desk-app",
		"ada", "external-user", "",
	} {
		if TrustedAuthor(login) {
			t.Errorf("TrustedAuthor(%q) = true with NO roster configured — unset must trust nobody", login)
		}
		if TrustedAuthorID(login, 2001) {
			t.Errorf("TrustedAuthorID(%q, 2001) = true with NO roster configured", login)
		}
		if IsBlessAuthority(login) {
			t.Errorf("IsBlessAuthority(%q) = true with NO roster configured — unset means nobody can bless", login)
		}
		if IsBlessAuthorityID(login, 2001) {
			t.Errorf("IsBlessAuthorityID(%q, 2001) = true with NO roster configured", login)
		}
	}

	// The blessing path itself must be shut, not merely unpopulated.
	now := time.Now()
	if Blessed(time.Time{}, []ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: now}}) {
		t.Error("Blessed(...) = true with NO roster configured — an unconfigured roster has no blessing authority")
	}
	if ItemTrustedEvents("external-user", time.Time{}, []ContentEvent{{Author: "ada", AuthorID: 2001, CreatedAt: now}}) {
		t.Error("ItemTrustedEvents admitted an external item with NO roster configured")
	}

	// The write-authorisation set is the same rule: unset refuses.
	if IsAllowedRepo("example-org/tracker") {
		t.Error("IsAllowedRepo = true with NO roster configured — the write boundary must fail closed")
	}
	if !CIRequired("example-org/tracker") {
		t.Error("CIRequired = false for an unconfigured repo — an empty CI rollup would read as green")
	}

	// …and it must SAY SO. A silent unconfigured state is the defect.
	err := RosterUnconfiguredError()
	if err == nil {
		t.Fatal("RosterUnconfiguredError() = nil with NO roster configured — the refusal must be loud")
	}
	for _, want := range []string{"NOT CONFIGURED", EnvBlessLogin, EnvTrustedLogins} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the unconfigured refusal does not mention %q, so a reader cannot fix it:\n%s", want, err)
		}
	}
}

// TestUnconfiguredRosterIsNotSilentlyPartial: a roster that names trusted humans
// but NO blessing authority is not "half configured", it is unconfigured. A
// half-parsed roster that trusts some logins while nobody can bless is the
// configured-but-empty shape this design exists to prevent.
func TestUnconfiguredRosterIsNotSilentlyPartial(t *testing.T) {
	withRoster(t, map[string]string{EnvTrustedLogins: "ada:1001"})
	if EffectiveConfig().Configured() {
		t.Fatal("a roster with no blessing authority reported itself CONFIGURED")
	}
	if TrustedAuthor("ada") {
		t.Error("TrustedAuthor(ada) = true from a roster with no blessing authority — partial config must not partially apply")
	}
}

// ---- P2: the source of truth lies outside every ref --------------------------

// TestRosterIsNotSettableFromTheWorkTree is P2. Plausible config files planted in
// a work tree — the shapes a pull request could actually author — must not change
// any verdict. The value lives in repository settings (CI) or the config home
// (local); neither is reachable from a diff.
func TestRosterIsNotSettableFromTheWorkTree(t *testing.T) {
	work := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	withNoRoster(t)

	verdict := func() string {
		var b bytes.Buffer
		for _, l := range EffectiveConfig().EffectiveConfigLines() {
			b.WriteString(l)
			b.WriteString("\n")
		}
		for _, login := range []string{"attacker", "ada", "ada"} {
			b.WriteString(login)
			b.WriteString(":")
			if TrustedAuthor(login) {
				b.WriteString("trusted")
			} else {
				b.WriteString("untrusted")
			}
			if IsBlessAuthority(login) {
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

	// Every shape a PR could plausibly add to the tree, each naming an attacker.
	planted := []string{
		".assay-trust",
		"assay.yml",
		".github/.assay-trust",
		".github/assay.yml",
		".assay/.assay-trust",
		".assay/assay.yml",
		// The config-home file's own name, planted in the WORK TREE rather than the
		// config home — the near-miss most likely to be accepted by a careless
		// implementation that resolves the path relatively.
		"roster.env",
		".config/assay/roster.env",
	}
	payload := []byte(EnvBlessLogin + "=attacker:999\n" +
		EnvTrustedLogins + "=attacker:999\n" +
		EnvTrustedBotSlugs + "=attacker-app:999\n" +
		EnvAllowedRepos + "=attacker/repo:no-ci:private\n")
	for _, rel := range planted {
		p := filepath.Join(work, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ReloadConfig()

	if after := verdict(); after != before {
		t.Fatalf("planting config files IN THE WORK TREE changed the verdict — the roster is reachable from a diff, "+
			"so the pull request being judged could add its own author to it.\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if TrustedAuthor("attacker") || IsBlessAuthority("attacker") {
		t.Fatal("an in-tree config file named an attacker and the loader accepted it")
	}
	if IsAllowedRepo("attacker/repo") {
		t.Fatal("an in-tree config file widened the WRITE-authorisation set")
	}
}

// TestWriteToolsRefuseEnvRoster is the split-by-action-class ruling.
// With the roster set in the ENVIRONMENT and no config-home file, a write-class
// load path reports UNCONFIGURED: the env value was not consulted.
//
// The threat is specific: a locally-run tool that ACTS is often driven by an agent
// reading untrusted text, and an env override is one injected sentence away. The
// config-home file is not — setting it takes a prior, separately-authorised write.
func TestWriteToolsRefuseEnvRoster(t *testing.T) {
	withNoRoster(t) // config home exists but is empty

	t.Setenv(EnvBlessLogin, "attacker:999")
	t.Setenv(EnvTrustedLogins, "attacker:999,ada:2001")
	t.Setenv(EnvTrustedBotSlugs, "attacker-app:999")
	t.Setenv(EnvAllowedRepos, "attacker/repo:no-ci:private")
	t.Setenv(EnvHumanLoginMap, "alex:attacker")
	ReloadConfig()

	cfg := EffectiveConfig()
	if cfg.Class != ClassWrite {
		t.Fatalf("the default tool class is %v, want ClassWrite — the restrictive class must be the ZERO value", cfg.Class)
	}
	if cfg.Configured() {
		t.Fatal("a write-class load consulted the ENVIRONMENT — the split-by-action-class ruling forbids it")
	}
	for _, login := range []string{"attacker", "ada"} {
		if TrustedAuthor(login) || IsBlessAuthority(login) {
			t.Errorf("write-class load trusted %q from an environment variable", login)
		}
	}
	if IsAllowedRepo("attacker/repo") {
		t.Error("write-class load took the WRITE-authorisation set from an environment variable")
	}
	if RosterUnconfiguredError() == nil {
		t.Error("with only an env roster present, a write-class tool must report UNCONFIGURED")
	}

	// The read-only class is the one place env-first survives — and no current tool
	// is in it. Asserted so the two classes cannot silently collapse into one.
	if ro := LoadConfig(ClassReadOnly); !ro.Configured() {
		t.Error("ClassReadOnly did not read the environment — the two classes have collapsed, " +
			"and TestWriteToolsRefuseEnvRoster would then pass for the wrong reason")
	}
}

// TestReadOnlyPrecedenceIsEnvFirstPerKey pins ClassReadOnly's ORDERING, which is the
// only place in this loader where two sources can both supply a value.
//
// The documented rule is "env-first with a config-home fallback", merged PER KEY:
// a key present in the environment wins outright, and the file supplies only the
// keys the environment does not carry. Nothing asserted it, so inverting the merge
// to file-wins survived the whole suite — and the two orderings are not equally
// safe in the same direction for every key. Env-first means a variable in the
// process environment SHADOWS the reviewed, permission-checked file (the class's
// accepted residual, which is why no acting tool is in this class); file-first
// would instead let a stale on-disk roster silently override a deliberate
// per-invocation value. Whichever is intended, it must be the one that is pinned:
// the first ClassReadOnly tool will inherit this ordering without re-deciding it.
func TestReadOnlyPrecedenceIsEnvFirstPerKey(t *testing.T) {
	// A complete, valid roster on disk — INCLUDING a repo set, which is the key the
	// environment overrides below. goldenRoster() carries only the three trust
	// variables, so it has to be stated here or the override has nothing to beat.
	fileRoster := goldenRoster()
	fileRoster[EnvAllowedRepos] = "file-org/file-repo:ci:private"
	withRoster(t, fileRoster)
	// ...and an environment that overrides exactly ONE key.
	t.Setenv(EnvAllowedRepos, "env-org/env-repo:ci:private")
	ReloadConfig()
	t.Cleanup(ReloadConfig)

	cfg := LoadConfig(ClassReadOnly)
	if !cfg.Configured() {
		t.Fatal("the merged read-only configuration did not load — the assertions below prove nothing")
	}

	// The overridden key comes from the ENVIRONMENT.
	if _, ok := cfg.Repos["env-org/env-repo"]; !ok {
		t.Errorf("ClassReadOnly did not take %s from the environment (repos=%v) — the documented "+
			"precedence is env-first, and a silent flip to file-first changes which source a "+
			"read-only tool obeys", EnvAllowedRepos, cfg.Repos)
	}
	if _, ok := cfg.Repos["file-org/file-repo"]; ok {
		t.Errorf("the FILE's %s survived alongside the environment's (repos=%v) — the merge is "+
			"per KEY, not per entry: a union here would let the weaker source WIDEN the "+
			"write-authorisation set the stronger one meant to replace", EnvAllowedRepos, cfg.Repos)
	}
	// Keys the environment does NOT carry still come from the file.
	if cfg.Bless.Login == "" {
		t.Errorf("%s was dropped: a key absent from the environment must still fall back to the "+
			"config-home file, or one env override would blank the rest of the roster", EnvBlessLogin)
	}
}

// TestConfigHomePermissionsEnforced is the sshd rule. A config file (or its
// directory) that is group- or world-writable is REFUSED, naming the problem; the
// same file at 0600 loads.
func TestConfigHomePermissionsEnforced(t *testing.T) {
	home := withRoster(t, goldenRoster())
	if !EffectiveConfig().Configured() {
		t.Fatal("the fixture roster did not load at 0600/0700 — the positive control is broken, " +
			"so the negative cases below would prove nothing")
	}
	dir := filepath.Join(home, ".config", "assay")
	file := filepath.Join(dir, "roster.env")

	// A flat loop, NOT subtests: an external check counts `--- PASS:` lines
	// for this test name and pins the count at exactly 2 (one per module). A
	// t.Run subtest prints its own `--- PASS: <Test>/<case>` line and would
	// silently inflate that count past the pin.
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
		ReloadConfig()
		cfg := EffectiveConfig()
		if cfg.Configured() {
			t.Errorf("the loader accepted a roster from a %s — anything that can write it can name "+
				"the accounts this tool trusts", tc.name)
		}
		if !strings.Contains(strings.Join(cfg.Problems, " "), "writable") {
			t.Errorf("the %s refusal does not name the permission problem, so a user cannot fix it: %v",
				tc.name, cfg.Problems)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(file, 0o600); err != nil {
			t.Fatal(err)
		}
		ReloadConfig()
	}

	// Positive control restored: the same file at 0600 loads again.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o600); err != nil {
		t.Fatal(err)
	}
	ReloadConfig()
	if !EffectiveConfig().Configured() {
		t.Fatal("the roster did not reload after permissions were corrected — the check is not a permission check, " +
			"it is a permanent refusal")
	}
}

// ---- P3: change is admin-only and VISIBLE, in both directions ----------------

// TestEffectiveRosterIsEchoed is P3 for the roster. The echo must carry the
// logins, and it must CHANGE when an entry is REMOVED from a non-empty roster —
// not only when the set goes from empty to non-empty.
func TestEffectiveRosterIsEchoed(t *testing.T) {
	withRoster(t, goldenRoster())
	var wide bytes.Buffer
	EchoEffectiveConfig(&wide)
	for _, want := range []string{"ada:2001", "shared-agent:2002", "assay-reviewer-app:300000004"} {
		if !strings.Contains(wide.String(), want) {
			t.Errorf("the run echo does not carry %q — a roster change would not appear in run output:\n%s", want, wide.String())
		}
	}

	// Remove ONE entry from the non-empty roster. The echoed line must differ.
	narrowed := goldenRoster()
	narrowed[EnvTrustedLogins] = "ada:2001"
	withRoster(t, narrowed)
	var narrow bytes.Buffer
	EchoEffectiveConfig(&narrow)
	if narrow.String() == wide.String() {
		t.Fatal("removing an entry from a non-empty roster did not change the echo — a narrowing " +
			"(the dangerous direction for a trigger list, and an audit hole for a roster) would be invisible")
	}
	if strings.Contains(narrow.String(), "shared-agent:2002") {
		t.Error("the echo still names a removed identity — it is not the EFFECTIVE value")
	}
}

// TestEffectiveConfigIsEchoed extends P3 to EVERY converted surface. Each line
// must change when an entry is ADDED or REMOVED. The allowed-repo set matters most
// here: it is the blast radius of every desk tool, and a widening must never be
// invisible in run output.
func TestEffectiveConfigIsEchoed(t *testing.T) {
	surfaces := []struct {
		name          string
		key           string
		base, widened string
	}{
		{"allowedRepos", EnvAllowedRepos,
			"one/a:ci:private,two/b:no-ci:public",
			"one/a:ci:private,two/b:no-ci:public,three/c:ci:private"},
		{"humanLoginMap", EnvHumanLoginMap, "ada:ada-gh", "ada:ada-gh,bo:bo-gh"},
		{"riskPathExtra", EnvRiskPathTriggersExtra, "infra/", "infra/,secrets/"},
	}
	for _, s := range surfaces {
		t.Run(s.name, func(t *testing.T) {
			base := goldenRoster()
			base[s.key] = s.base
			withRoster(t, base)
			var narrow bytes.Buffer
			EchoEffectiveConfig(&narrow)
			if !strings.Contains(narrow.String(), s.key+"=") {
				t.Fatalf("the run echo carries no %s line at all", s.key)
			}

			wide := goldenRoster()
			wide[s.key] = s.widened
			withRoster(t, wide)
			var widened bytes.Buffer
			EchoEffectiveConfig(&widened)

			if widened.String() == narrow.String() {
				t.Fatalf("ADDING an entry to %s did not change the run echo — a widening of this surface "+
					"would leave no trace in run output or CI history", s.key)
			}
			// …and the reverse. Removal must be equally visible.
			withRoster(t, base)
			var again bytes.Buffer
			EchoEffectiveConfig(&again)
			if again.String() == widened.String() {
				t.Fatalf("REMOVING an entry from %s did not change the run echo", s.key)
			}
		})
	}
}

// ---- P4: numeric-id pinning survives the move --------------------------------

// TestRecycledLoginRejected is P4. A trusted login carrying a MISMATCHED numeric
// id is untrusted: a deleted login can be re-registered by an attacker, and a
// config format that carried only logins would silently discard that defence.
func TestRecycledLoginRejected(t *testing.T) {
	withRoster(t, goldenRoster())

	if !TrustedAuthorID("ada", 2001) {
		t.Fatal("the positive control failed: the configured login+id pair is not trusted, so the negative cases below prove nothing")
	}
	for _, tc := range []struct {
		login string
		id    int64
	}{
		{"ada", 999999},
		{"example-org", 1},
		{"assay-reviewer-app[bot]", 42},
	} {
		if TrustedAuthorID(tc.login, tc.id) {
			t.Errorf("TrustedAuthorID(%q, %d) = true — a trusted login with the WRONG id must be untrusted", tc.login, tc.id)
		}
	}
	if IsBlessAuthorityID("ada", 999999) {
		t.Error("the blessing authority's login carrying a foreign id was accepted as the authority")
	}
	if !IsBlessAuthorityID("ada", 2001) {
		t.Error("the blessing authority's login with its pinned id was refused")
	}
}

// TestBlessLoginRequiresID: the id is MANDATORY on ASSAY_BLESS_LOGIN, unlike its
// siblings. A bare login is refused at load — never silently downgraded to
// login-only trust — because the blessing authority is the single highest-value
// target for login recycling.
func TestBlessLoginRequiresID(t *testing.T) {
	r := goldenRoster()
	r[EnvBlessLogin] = "ada"
	withRoster(t, r)

	cfg := EffectiveConfig()
	if cfg.Configured() {
		t.Fatal("a bare-login ASSAY_BLESS_LOGIN was accepted — the id is mandatory for the blessing authority")
	}
	if IsBlessAuthority("ada") {
		t.Error("a bare-login blessing authority was accepted as the authority")
	}
	if !strings.Contains(strings.Join(cfg.Problems, " "), "numeric id") {
		t.Errorf("the refusal does not say the id is missing: %v", cfg.Problems)
	}
	// It must fail EXACTLY like the unset case, not partially.
	if TrustedAuthor("example-org") {
		t.Error("a refused bless login left the rest of the roster live — validation failure must " +
			"collapse the whole configuration, not admit a partial roster")
	}
}

// TestBlessLoginRejectsMultipleValues: ASSAY_BLESS_LOGIN is a SINGLE identity. Its
// siblings accept comma-separated lists, and that looser rule must not leak onto
// it — taking the first entry, or blessing every entry, would silently grant the
// authority this variable exists to restrict.
func TestBlessLoginRejectsMultipleValues(t *testing.T) {
	for _, raw := range []string{
		"ada:2001,attacker:999",
		"ada:2001;attacker:999",
		"ada:2001 attacker:999",
	} {
		r := goldenRoster()
		r[EnvBlessLogin] = raw
		withRoster(t, r)
		cfg := EffectiveConfig()
		if cfg.Configured() {
			t.Errorf("ASSAY_BLESS_LOGIN=%q was accepted — a list must be refused, exactly like an unset roster", raw)
		}
		if IsBlessAuthority("ada") {
			t.Errorf("ASSAY_BLESS_LOGIN=%q blessed the FIRST entry of a list", raw)
		}
		if IsBlessAuthority("attacker") {
			t.Errorf("ASSAY_BLESS_LOGIN=%q blessed a second entry of a list", raw)
		}
	}
}

// TestBlessLoginRejectsBotAccount: the blessing is the AUTHORISATION half of the
// trust gate, and the authorisation half of a two-factor mechanism has to be a
// human act (looksLikeBot's argument, verbatim). An App that could
// bless would admit external text into its own queue.
func TestBlessLoginRejectsBotAccount(t *testing.T) {
	for _, raw := range []string{
		"assay-desk-app[bot]:300000001",
		"app/assay-desk-app:300000001",
		"some-app:12345",
		"some-bot:12345",
		// Present in the CONFIGURED bot-slug set, in bare form: the shape the
		// rendering checks above do not catch.
		"assay-reviewer-app:300000004",
	} {
		r := goldenRoster()
		r[EnvBlessLogin] = raw
		withRoster(t, r)
		cfg := EffectiveConfig()
		if cfg.Configured() {
			t.Errorf("ASSAY_BLESS_LOGIN=%q was accepted — a bot/App/shared-agent account must never be the blessing authority", raw)
		}
		if BlessAuthorityLogin() != "" {
			t.Errorf("ASSAY_BLESS_LOGIN=%q produced a live blessing authority %q", raw, BlessAuthorityLogin())
		}
	}
}

// TestHumanLoginMapRejectsBotAccount is the twin of statusgen's test of the same
// name. This module has no gate-clearing consumer of the map, but the check is
// carried here so the two duplicated readers stay byte-identical on the same file:
// a bot/App/shared-agent LOGIN in ASSAY_HUMAN_LOGIN_MAP collapses the whole
// configuration, mirroring the ASSAY_BLESS_LOGIN rule.
func TestHumanLoginMapRejectsBotAccount(t *testing.T) {
	// Anti-vacuity control: the golden roster's well-formed map is live.
	withRoster(t, goldenRoster())
	if !EffectiveConfig().Configured() {
		t.Fatal("control: the golden roster is not configured — the bot-shaped cases below prove nothing")
	}

	for _, raw := range []string{
		"alex:assay-desk-app[bot]",
		"alex:app/assay-desk-app",
		"alex:some-app",
		"alex:some-bot",
	} {
		r := goldenRoster()
		r[EnvHumanLoginMap] = raw
		withRoster(t, r)
		cfg := EffectiveConfig()
		if cfg.Configured() {
			t.Errorf("deskkit accepted ASSAY_HUMAN_LOGIN_MAP=%q — a bot/App value must not resolve a human token", raw)
		}
		if len(cfg.HumanLogins) != 0 {
			t.Errorf("ASSAY_HUMAN_LOGIN_MAP=%q produced a live human mapping %v", raw, cfg.HumanLogins)
		}
	}
}

// TestMalformedIDNeverDegradesToLoginOnlyTrust pins splitIdentity's stated
// fail-closed rule: "ok is false when an id is present but not a number — a typo'd
// id must never degrade to login-only trust."
//
// The rule was stated in the source and nothing tested it. Replacing the refusal
// with `return l, 0, true` in BOTH modules left the whole suite green, which is why
// this test exists: the mutation is a WIDENING one, not a stylistic one. Under it
// `ASSAY_TRUSTED_LOGINS=ada:2O959` (letter O for zero) stops refusing the
// configuration and instead configures `ada` as a trusted login with no pin, so
// TrustedAuthor — the login-only board surface — answers true for a login whose id
// was never checked. That is precisely the login-recycling exposure the id pins
// close.
//
// The bless-login row is included deliberately even though it is caught one branch
// later by the mandatory-id rule: it survives the mutation by LUCK, not by design,
// so the assertion is on WHICH refusal fires, not merely that one does.
func TestMalformedIDNeverDegradesToLoginOnlyTrust(t *testing.T) {
	// Anti-vacuity control: the same roster with a well-formed id must be live,
	// so a green result below cannot come from a roster that is broken anyway.
	withRoster(t, goldenRoster())
	if !EffectiveConfig().Configured() || !TrustedAuthor("ada") {
		t.Fatal("control: the well-formed golden roster is not live — the malformed cases below prove nothing")
	}

	// Letter O for zero, a trailing dot, and a signed value: three shapes that
	// ParseInt rejects but a login-only degrade would silently accept.
	for _, bad := range []string{"2O959", "2001.", "+2001x"} {
		t.Run("trusted_login/"+bad, func(t *testing.T) {
			r := goldenRoster()
			r[EnvTrustedLogins] = "ada:" + bad + ",shared-agent:2002"
			withRoster(t, r)

			cfg := EffectiveConfig()
			if cfg.Configured() {
				t.Errorf("ASSAY_TRUSTED_LOGINS with the unparseable id %q was accepted — a typo'd id "+
					"must refuse the configuration, never degrade to login-only trust", bad)
			}
			if TrustedAuthor("ada") {
				t.Errorf("id %q degraded to login-only trust: TrustedAuthor(\"ada\") is true with no "+
					"id ever checked — this is the login-recycling exposure the pin exists to close", bad)
			}
			// The collapse is whole-config, not per-entry.
			if TrustedAuthor("example-org") {
				t.Errorf("id %q left the REST of the roster live — a validation failure must collapse "+
					"the whole configuration, not admit a partial roster", bad)
			}
			if !strings.Contains(strings.Join(cfg.Problems, " "), "cannot parse entry") {
				t.Errorf("the refusal for id %q does not name the unparseable entry: %v", bad, cfg.Problems)
			}
		})
	}

	// The same rule on the bot-slug reader, whose entries carry an extra `role=`
	// prefix and whose logins are derived into two rendered forms.
	t.Run("bot_slug", func(t *testing.T) {
		r := goldenRoster()
		r[EnvTrustedBotSlugs] = "worker=assay-worker-app:3O6480234"
		withRoster(t, r)

		cfg := EffectiveConfig()
		if cfg.Configured() {
			t.Error("ASSAY_TRUSTED_BOT_SLUGS with an unparseable id was accepted")
		}
		for _, rendering := range []string{"assay-worker-app[bot]", "app/assay-worker-app"} {
			if TrustedAuthor(rendering) {
				t.Errorf("an unparseable bot id degraded to login-only trust: TrustedAuthor(%q) is true "+
					"with no id ever checked", rendering)
			}
		}
	})

	// The blessing authority. Under the widening mutation this row still refuses,
	// but for the WRONG reason — the id arrives as 0 and the mandatory-id branch
	// catches it. Pin the refusal that the parse rule owns.
	t.Run("bless_login", func(t *testing.T) {
		r := goldenRoster()
		r[EnvBlessLogin] = "ada:2O959"
		withRoster(t, r)

		cfg := EffectiveConfig()
		if cfg.Configured() {
			t.Error("ASSAY_BLESS_LOGIN with an unparseable id was accepted")
		}
		if IsBlessAuthority("ada") {
			t.Error("an unparseable id produced a live blessing authority")
		}
		if !strings.Contains(strings.Join(cfg.Problems, " "), "cannot be parsed") {
			t.Errorf("the bless-login refusal did not come from the PARSE rule — an unparseable id "+
				"reached a later branch and was refused by luck: %v", cfg.Problems)
		}
	})
}

// ---- the golden roster --------------------------------------------

// TestTrustGoldenRoster proves BEHAVIOURAL NEUTRALITY: the golden fixture
// roster, supplied as configuration, reproduces the verdicts the compiled-in
// constants produced, for all eight identities and in both GitHub renderings.
// A conversion that quietly changed who is trusted fails here.
func TestTrustGoldenRoster(t *testing.T) {
	withRoster(t, goldenRoster())

	humans := map[string]int64{"ada": 2001, "shared-agent": 2002}
	bots := map[string]int64{
		"assay-desk-app":        300000001,
		"assay-intake-loop-app": 300000002,
		"assay-issue-loop-app":  300000003,
		"assay-reviewer-app":    300000004,
		"assay-verifier-app":    300000005,
		"assay-worker-app":      300000006,
	}
	if len(humans)+len(bots) != 8 {
		t.Fatalf("the golden roster covers %d identities, want 8", len(humans)+len(bots))
	}

	for login, id := range humans {
		if !TrustedAuthor(login) || !TrustedAuthor(strings.ToUpper(login)) {
			t.Errorf("%s is not a trusted author under the configured roster", login)
		}
		if !TrustedAuthorID(login, id) {
			t.Errorf("%s:%d is not trusted id-aware", login, id)
		}
		if TrustedAuthorID(login, id+1) {
			t.Errorf("%s with a foreign id was trusted", login)
		}
	}
	for slug, id := range bots {
		for _, rendered := range []string{slug + "[bot]", "app/" + slug} {
			if !TrustedAuthor(rendered) {
				t.Errorf("%s is not a trusted author under the configured roster", rendered)
			}
			if !TrustedAuthorID(rendered, id) {
				t.Errorf("%s:%d is not trusted id-aware", rendered, id)
			}
		}
		if TrustedAuthor(slug) {
			t.Errorf("the BARE slug %q was trusted — slugs and usernames are separate GitHub namespaces, so that is squattable", slug)
		}
	}

	// Exactly ONE blessing authority, and it is a human.
	blessed := 0
	for _, login := range append(keysOf(humans), renderedBots(bots)...) {
		if IsBlessAuthority(login) {
			blessed++
		}
	}
	if blessed != 1 {
		t.Errorf("%d configured identities may bless, want exactly 1 — trusted AUTHOR and may-BLESS are distinct capabilities", blessed)
	}
	if !IsBlessAuthority("ada") {
		t.Error("the configured blessing authority is not the blessing authority")
	}

	// Non-identities stay out.
	for _, login := range []string{"", "external-user", "dependabot[bot]", "github-actions[bot]", "ada2", "app/ada"} {
		if TrustedAuthor(login) {
			t.Errorf("%q was trusted under the configured roster", login)
		}
	}
}

func keysOf(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func renderedBots(m map[string]int64) []string {
	out := make([]string, 0, len(m)*2)
	for slug := range m {
		out = append(out, slug+"[bot]", "app/"+slug)
	}
	return out
}
