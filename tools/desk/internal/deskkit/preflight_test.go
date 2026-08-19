package deskkit

// Property tests for the operating-envelope preflight.
//
// Every check here ships with a POSITIVE CONTROL: a scenario that makes it go
// red. A check with no proof that it can fail is indistinguishable from a check
// that always passes, and this preflight exists precisely because five different
// silent-pass conditions each cost a live desk a whole pass.
//
// The suite is hermetic: every probe is injected, so nothing here mints a token,
// touches a git remote, or reads a real credential.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture roster's verifier binding (rosterfixture_test.go):
//
//	verifier=assay-verifier-app:300000005
//
// The App id below is a DIFFERENT number on purpose — that difference is the
// entire content of #638, and a test that used the same number for both could
// not tell the bug from the fix.
const (
	pfRole       = "verifier"
	pfSlug       = "assay-verifier-app"
	pfBotUserID  = "300000005"
	pfAppID      = "400000005"
	pfGoodEmail  = pfBotUserID + "+" + pfSlug + "[bot]@users.noreply.github.com"
	pfAppIDEmail = pfAppID + "+" + pfSlug + "[bot]@users.noreply.github.com"
)

// okProbes is an all-green probe set. Each test darkens exactly ONE probe, so a
// red result is attributable to the check under test and nothing else.
func okProbes() PreflightProbes {
	return PreflightProbes{
		ColdMint: func(string, string) (string, error) { return "/tmp/fake-token", nil },
		GrantedScopes: func(string, string) (map[string]string, error) {
			return map[string]string{"pull_requests": "write", "issues": "write", "contents": "write"}, nil
		},
		WriteTransport: func(Landing) (ProbeVerdict, string, error) { return ProbePermitted, "up to date", nil },
		CommitEmail:    func(string) (string, error) { return pfGoodEmail, nil },
		AppIDFor:       func(string) (string, error) { return pfAppID, nil },
		QueuedSiblings: func(string) ([]SiblingReq, error) { return nil, nil },
		DirExists:      func(string) (bool, error) { return true, nil },
	}
}

func runPF(t *testing.T, p PreflightProbes) PreflightReport {
	t.Helper()
	return PreflightRequest{Role: pfRole, Root: t.TempDir(), Probes: p}.Run()
}

func pfCheck(t *testing.T, rep PreflightReport, name string) Check {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("check %q is not in the report (checks: %v)", name, pfNames(rep))
	return Check{}
}

func pfNames(rep PreflightReport) []string {
	var out []string
	for _, c := range rep.Checks {
		out = append(out, c.Name)
	}
	return out
}

// ---- the three-state contract ---------------------------------------------

// TestPreflightZeroValueStateIsCouldNotCheck is the load-bearing one. A Check
// built by a future author who forgets to set State must read RED. If the zero
// value were CheckedClean, every unwritten code path in this file would be a
// silent pass — which is the class of bug the whole preflight exists to catch.
func TestPreflightZeroValueStateIsCouldNotCheck(t *testing.T) {
	var c Check
	if c.State != CouldNotCheck {
		t.Fatalf("zero-value CheckState = %v, want CouldNotCheck", c.State)
	}
	if c.State.Green() {
		t.Fatal("the zero-value state reads GREEN — a check that never looked would pass")
	}
	if got := c.State.String(); got != "could-not-check" {
		t.Fatalf("zero-value renders %q, want could-not-check", got)
	}
	// An out-of-range value must also fail closed rather than render an unknown token.
	if got := CheckState(99).String(); got != "could-not-check" {
		t.Fatalf("out-of-range state renders %q, want could-not-check", got)
	}
}

// TestPreflightThreeStateVocabularyIsFixed pins the exact words the desk reports
// in. Consumers (skills, CI greps) match on them; drifting one is a silent break.
func TestPreflightThreeStateVocabulary(t *testing.T) {
	for state, want := range map[CheckState]string{
		CheckedClean:  "checked-clean",
		CheckedFailed: "checked-failed",
		CouldNotCheck: "could-not-check",
	} {
		if got := state.String(); got != want {
			t.Errorf("state %d renders %q, want %q", state, got, want)
		}
	}
}

// TestPreflightAllCleanIsGreen — the baseline. Five checks, all clean, no error.
func TestPreflightAllCleanIsGreen(t *testing.T) {
	withRoster(t, goldenRoster())
	rep := runPF(t, okProbes())
	if len(rep.Checks) != 5 {
		t.Fatalf("ran %d checks, want 5: %v", len(rep.Checks), pfNames(rep))
	}
	for _, c := range rep.Checks {
		if c.State != CheckedClean {
			t.Errorf("%s = %s (%s); want checked-clean", c.Name, c.State, c.Detail)
		}
		if c.Remediation != "" {
			t.Errorf("%s is clean but carries a remediation %q", c.Name, c.Remediation)
		}
	}
	if !rep.Green() {
		t.Fatal("all-clean report is not Green()")
	}
	if err := rep.Err(); err != nil {
		t.Fatalf("all-clean report returned an error: %v", err)
	}
	if !strings.Contains(rep.SummaryLine(), "GREEN 5/5") {
		t.Fatalf("summary %q does not report GREEN 5/5", rep.SummaryLine())
	}
}

// TestPreflightEmptyReportIsNotGreen — a preflight that ran nothing proved
// nothing. "No checks failed" must never be reported as "the envelope is fine".
func TestPreflightEmptyReportIsNotGreen(t *testing.T) {
	var rep PreflightReport
	if rep.Green() {
		t.Fatal("an EMPTY report reads Green() — a preflight that ran no checks would pass")
	}
	if rep.Err() == nil {
		t.Fatal("an empty report returned no error")
	}
}

// TestPreflightNoRoleIsRefusedNotAssumed — the envelope is checked FOR a role.
// Guessing one is how a desk passes a preflight for an identity it is not using.
func TestPreflightNoRole(t *testing.T) {
	rep := PreflightRequest{Role: "  ", Probes: okProbes()}.Run()
	if rep.Green() {
		t.Fatal("a preflight with no role reported GREEN")
	}
}

// ---- check 1: cold token mint (#794, #567) --------------------------------

// TestPreflightColdMintFailureIsRed is the #794 positive control: on a fresh
// shell the minter cannot find the key, and the desk must learn that at BOOT
// rather than an hour later when the warm cache lapses mid-pass.
func TestPreflightColdMintFailureIsRed(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.ColdMint = func(string, string) (string, error) {
		return "", Unverifiable("cold mint refused: no App private key verifier-app.pem found. Searched: /h/.config/assay", nil)
	}
	rep := runPF(t, p)
	c := pfCheck(t, rep, CheckColdMint)
	if c.State != CheckedFailed {
		t.Fatalf("%s = %s, want checked-failed", c.Name, c.State)
	}
	if c.Remediation == "" {
		t.Fatal("a failed cold mint carries no remediation")
	}
	if !strings.Contains(c.Refs, "#794") {
		t.Fatalf("refs %q do not name #794", c.Refs)
	}
	if rep.Err() == nil {
		t.Fatal("a red cold mint did not make the whole pass could-not-run")
	}
}

// TestPreflightColdMintSilentSuccessIsNotGreen — a mint that reports success but
// names no token file has not proven anything. It must read could-not-check, not
// clean: "exit 0" is not evidence a credential exists.
func TestPreflightColdMintSilentSuccessIsNotGreen(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.ColdMint = func(string, string) (string, error) { return "   ", nil }
	rep := runPF(t, p)
	if got := pfCheck(t, rep, CheckColdMint).State; got != CouldNotCheck {
		t.Fatalf("%s = %s, want could-not-check", CheckColdMint, got)
	}
}

// TestPreflightColdMintScrubbedEnvIsAnAllowlist — "cold" means the probe cannot
// inherit an ambient credential. If it could, the probe would pass on the warm
// token that #794 says is about to expire, which is the failure exactly.
func TestPreflightColdMintScrubbedEnvIsAllowlist(t *testing.T) {
	t.Setenv("VERIFIER_TOKEN", "/warm/token")
	t.Setenv("VERIFIER_PEM", "/warm/key.pem")
	t.Setenv("GH_TOKEN", "ghs_warm")
	t.Setenv(EnvConfigHome, "/cfg")
	env := scrubbedEnv()
	joined := strings.Join(env, "\n")
	for _, banned := range []string{"VERIFIER_TOKEN=", "VERIFIER_PEM=", "GH_TOKEN="} {
		if strings.Contains(joined, banned) {
			t.Errorf("the cold probe inherits %s — it would pass on an ambient credential", banned)
		}
	}
	if !strings.Contains(joined, EnvConfigHome+"=/cfg") {
		t.Error("the cold probe drops the config-home knob — it would search the wrong directory")
	}
}

// TestPreflightRepoSlugFromEveryRemoteForm — the cold mint is against an
// INSTALLATION, so the probe must resolve the same owner the pass lands on. The
// ssh HOST-ALIAS form (`host-alias:owner/repo.git`) is the one a naive parser
// drops; dropping it sends the probe at the minter's built-in default owner, and
// the check then reports on an installation the pass will never use.
func TestPreflightRepoSlugFromEveryRemoteForm(t *testing.T) {
	for _, url := range []string{
		"https://github.com/example-org/tracker.git",
		"https://github.com/example-org/tracker",
		"git@github.com:example-org/tracker.git",
		"ssh://git@github.com/example-org/tracker.git",
		"github-alias:example-org/tracker.git",
	} {
		m := remoteSlugRe.FindStringSubmatch(url)
		if m == nil {
			t.Errorf("remote %q yielded no owner/name — the probe would mint against the default owner", url)
			continue
		}
		if got := m[1] + "/" + m[2]; got != "example-org/tracker" {
			t.Errorf("remote %q resolved to %q", url, got)
		}
	}
}

// ---- check 2: App scopes vs rostered duties (#571) ------------------------

// TestPreflightScopeGapIsRed is the #571 positive control: the installation can
// write issues but not pull_requests, so the desk goes silent on PR threads —
// discovered, in the live incident, at comment time.
func TestPreflightScopeGapIsRed(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.GrantedScopes = func(string, string) (map[string]string, error) {
		return map[string]string{"issues": "write", "contents": "write", "pull_requests": "read"}, nil
	}
	rep := runPF(t, p)
	c := pfCheck(t, rep, CheckAppScopes)
	if c.State != CheckedFailed {
		t.Fatalf("%s = %s, want checked-failed", c.Name, c.State)
	}
	if !strings.Contains(c.Detail, "pull_requests") {
		t.Fatalf("detail %q does not name the missing scope", c.Detail)
	}
	if strings.Contains(c.Detail, "issues=") || strings.Contains(c.Detail, "contents=") {
		t.Fatalf("detail %q reports scopes that ARE granted as missing", c.Detail)
	}
}

// TestPreflightScopesUnreadableIsCouldNotCheck — no recorded grant means the
// check could not LOOK. AGENTS.md: a permission listing is neither grant nor bar;
// an ABSENT listing is certainly not a grant.
func TestPreflightScopesUnreadableIsCouldNotCheck(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.GrantedScopes = func(string, string) (map[string]string, error) {
		return nil, errors.New("no recorded grant at /tmp/fake-token.perms")
	}
	rep := runPF(t, p)
	c := pfCheck(t, rep, CheckAppScopes)
	if c.State != CouldNotCheck {
		t.Fatalf("%s = %s, want could-not-check", c.Name, c.State)
	}
	if c.State.Green() {
		t.Fatal("an unreadable grant reported green")
	}
}

// TestPreflightScopesNotReadWhenNoTokenMinted — the scope check must not invent a
// verdict when check 1 produced nothing to read.
func TestPreflightScopesNotReadWhenNoTokenMinted(t *testing.T) {
	withRoster(t, goldenRoster())
	called := false
	p := okProbes()
	p.ColdMint = func(string, string) (string, error) { return "", Unverifiable("cold mint refused: no key", nil) }
	p.GrantedScopes = func(string, string) (map[string]string, error) { called = true; return nil, nil }
	rep := runPF(t, p)
	if called {
		t.Error("the scope probe ran with no minted token")
	}
	if got := pfCheck(t, rep, CheckAppScopes).State; got != CouldNotCheck {
		t.Fatalf("%s = %s, want could-not-check", CheckAppScopes, got)
	}
}

// TestPreflightLevelSatisfiesOrdering pins the read < write < admin ordering and
// its fail-closed edges: an unknown or absent level never satisfies anything.
func TestPreflightLevelSatisfies(t *testing.T) {
	cases := []struct {
		granted, want string
		ok            bool
	}{
		{"write", "write", true},
		{"admin", "write", true},
		{"read", "write", false},
		{"", "write", false},
		{"maintain", "write", false}, // unknown level: fail closed
		{"read", "read", true},
		{"write", "maintain", false}, // unknown requirement: fail closed
	}
	for _, c := range cases {
		if got := levelSatisfies(c.granted, c.want); got != c.ok {
			t.Errorf("levelSatisfies(%q, %q) = %v, want %v", c.granted, c.want, got, c.ok)
		}
	}
}

// TestPreflightPermsSidecarParse — the grant sidecar is parsed strictly: a
// malformed file is an ERROR (could-not-check), never a silently empty map that
// would read as "the installation grants nothing".
func TestPreflightPermsSidecarParse(t *testing.T) {
	got, err := parsePermsJSON(`{"contents":"write","issues":"write"}`)
	if err != nil {
		t.Fatalf("well-formed grant failed to parse: %v", err)
	}
	if got["contents"] != "write" || got["issues"] != "write" {
		t.Fatalf("parsed grant = %v", got)
	}
	for _, bad := range []string{``, `not json`, `{"contents"}`, `[{"a":"b"}]`, `{"":"write"}`} {
		if _, err := parsePermsJSON(bad); err == nil {
			t.Errorf("malformed grant %q parsed without error — it would read as an empty grant", bad)
		}
	}
}

// ---- check 3: write transport (#823) --------------------------------------

// TestPreflightWriteTransportRejectionIsRed is the #823 positive control: a pass
// that verifies two briefs to a clean PASS and can land neither.
func TestPreflightWriteTransportRejectionIsRed(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.WriteTransport = func(Landing) (ProbeVerdict, string, error) {
		return ProbeRejected, "remote: Permission to example-org/tracker.git denied", nil
	}
	rep := runPF(t, p)
	c := pfCheck(t, rep, CheckWriteTransport)
	if c.State != CheckedFailed {
		t.Fatalf("%s = %s, want checked-failed", c.Name, c.State)
	}
	if !strings.Contains(c.Remediation, "STOP") {
		t.Fatalf("a rejection's remediation %q does not say STOP", c.Remediation)
	}
	if !strings.Contains(strings.ToLower(c.Remediation), "do not retry under another identity") {
		t.Fatalf("remediation %q omits the AGENTS.md scope-rejection rule", c.Remediation)
	}
}

// TestPreflightProbeRejectionIsNotRetried — AGENTS.md, "Scope rejections": a
// rejection is a STOP, never re-attempted under a different identity. The probe
// is invoked EXACTLY ONCE. A retry loop here would be the desk quietly
// escalating its own privileges.
func TestPreflightProbeRejectionIsNotRetried(t *testing.T) {
	withRoster(t, goldenRoster())
	calls := 0
	p := okProbes()
	p.WriteTransport = func(Landing) (ProbeVerdict, string, error) {
		calls++
		return ProbeRejected, "403", nil
	}
	runPF(t, p)
	if calls != 1 {
		t.Fatalf("the write-transport probe ran %d times after a REJECTION; want exactly 1", calls)
	}
}

// TestPreflightWriteTransportInconclusiveIsCouldNotCheck — an unreachable remote
// or a detached HEAD is not a pass.
func TestPreflightWriteTransportInconclusiveIsCouldNotCheck(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.WriteTransport = func(Landing) (ProbeVerdict, string, error) {
		return ProbeInconclusive, "detached HEAD: no landing branch to probe", nil
	}
	rep := runPF(t, p)
	if got := pfCheck(t, rep, CheckWriteTransport).State; got != CouldNotCheck {
		t.Fatalf("%s = %s, want could-not-check", CheckWriteTransport, got)
	}
}

// TestPreflightRejectionVocabulary — the classifier must read the harness
// permission-denial wording (#823) as a REJECTION, not as inconclusive. From the
// desk's seat "blocked by the permission classifier" and "403" are the same fact:
// this identity has no permitted write transport.
func TestPreflightRejectionVocabulary(t *testing.T) {
	rejections := []string{
		"remote: Permission to org/repo.git denied to assay-verifier-app[bot]",
		"fatal: Authentication failed for 'https://github.com/org/repo.git/'",
		"remote: 403 Forbidden",
		"GraphQL: Resource not accessible by integration (addComment)",
		"blocked by the auto-mode permission classifier",
		"remote: error: GH006: Protected branch update failed",
	}
	for _, r := range rejections {
		if !rejectionRe.MatchString(r) {
			t.Errorf("transport answer %q is NOT read as a rejection — it would report inconclusive", r)
		}
	}
	for _, ok := range []string{"Everything up-to-date", "To github.com:org/repo.git"} {
		if rejectionRe.MatchString(ok) {
			t.Errorf("benign transport output %q is read as a rejection", ok)
		}
	}
}

// ---- check 4: commit identity (#638) --------------------------------------

// TestPreflightCommitIdentityBotUserIDIsClean — the correct shape.
func TestPreflightCommitIdentityBotUserIDIsClean(t *testing.T) {
	withRoster(t, goldenRoster())
	rep := runPF(t, okProbes())
	if got := pfCheck(t, rep, CheckCommitIdentity).State; got != CheckedClean {
		t.Fatalf("%s = %s with the bot-user-id email, want checked-clean", CheckCommitIdentity, got)
	}
}

// TestPreflightCommitIdentityAppIDIsRed is the #638 positive control, in the
// exact shape the live incident had: the email is well-formed, names the right
// App, and its numeric prefix is the APP id — so every commit lands with
// author.login=null and is attributed to nobody. Locally invisible; remotely total.
func TestPreflightCommitIdentityAppIDIsRed(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.CommitEmail = func(string) (string, error) { return pfAppIDEmail, nil }
	rep := runPF(t, p)
	c := pfCheck(t, rep, CheckCommitIdentity)
	if c.State != CheckedFailed {
		t.Fatalf("%s = %s with the APP-id email, want checked-failed", c.Name, c.State)
	}
	if !strings.Contains(c.Detail, "APP id") {
		t.Fatalf("detail %q does not say the prefix is the App id", c.Detail)
	}
	if !strings.Contains(c.Remediation, pfGoodEmail) {
		t.Fatalf("remediation %q does not name the corrected email %q", c.Remediation, pfGoodEmail)
	}
}

// TestPreflightCommitIdentityWrongAppIsRed — the right numeric shape under
// another App's name is still the wrong identity.
func TestPreflightCommitIdentityWrongAppIsRed(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.CommitEmail = func(string) (string, error) {
		return pfBotUserID + "+assay-worker-app[bot]@users.noreply.github.com", nil
	}
	rep := runPF(t, p)
	if got := pfCheck(t, rep, CheckCommitIdentity).State; got != CheckedFailed {
		t.Fatalf("%s = %s for another App's login, want checked-failed", CheckCommitIdentity, got)
	}
}

// TestPreflightCommitIdentityUnsetIsRed — no identity at all is a failure, not an
// absence: the commits would land under whatever ambient git config exists.
func TestPreflightCommitIdentityUnsetIsRed(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.CommitEmail = func(string) (string, error) { return "", nil }
	rep := runPF(t, p)
	if got := pfCheck(t, rep, CheckCommitIdentity).State; got != CheckedFailed {
		t.Fatalf("%s = %s with no configured email, want checked-failed", CheckCommitIdentity, got)
	}
}

// TestPreflightCommitIdentityUnboundRoleIsCouldNotCheck — with no roster binding
// the check cannot know which bot user id is right, so it must NOT guess and must
// NOT pass. This is the same fail-closed shape RoleAppLogin's ok return enforces.
func TestPreflightCommitIdentityUnboundRoleIsCouldNotCheck(t *testing.T) {
	withNoRoster(t)
	p := okProbes()
	rep := PreflightRequest{Role: pfRole, Root: t.TempDir(), Probes: p}.Run()
	c := pfCheck(t, rep, CheckCommitIdentity)
	if c.State != CouldNotCheck {
		t.Fatalf("%s = %s with an unconfigured roster, want could-not-check", c.Name, c.State)
	}
	if c.Remediation == "" {
		t.Fatal("the unbound-role result names no remediation")
	}
}

// ---- check 5: sibling checkouts (#679) ------------------------------------

// TestPreflightMissingSiblingIsRed is the #679 positive control: a queued brief
// declares an out-of-repo checkout that is not there, and the rows that need it
// would be discovered unrunnable mid-pass.
func TestPreflightMissingSiblingIsRed(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.QueuedSiblings = func(string) ([]SiblingReq, error) {
		return []SiblingReq{{Brief: "docs/streams/example-stream/brief-43-x.md", Rel: "../tracker"}}, nil
	}
	p.DirExists = func(string) (bool, error) { return false, nil }
	rep := runPF(t, p)
	c := pfCheck(t, rep, CheckSiblings)
	if c.State != CheckedFailed {
		t.Fatalf("%s = %s with a missing sibling, want checked-failed", c.Name, c.State)
	}
	if !strings.Contains(c.Detail, "../tracker") || !strings.Contains(c.Detail, "brief-43") {
		t.Fatalf("detail %q does not name both the missing checkout and the brief that declared it", c.Detail)
	}
}

// TestPreflightSiblingStatErrorIsCouldNotCheck — an unreadable path is not a
// missing one, and reporting it as "absent" would send the reader to the wrong fix.
func TestPreflightSiblingStatErrorIsCouldNotCheck(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.QueuedSiblings = func(string) ([]SiblingReq, error) {
		return []SiblingReq{{Brief: "b.md", Rel: "../tracker"}}, nil
	}
	p.DirExists = func(string) (bool, error) { return false, errors.New("permission denied") }
	rep := runPF(t, p)
	if got := pfCheck(t, rep, CheckSiblings).State; got != CouldNotCheck {
		t.Fatalf("%s = %s on a stat error, want could-not-check", CheckSiblings, got)
	}
}

// TestPreflightQueuedSiblingsReadsDeclarationsOnly exercises the REAL scanner
// against a temp streams tree. Three properties at once:
//
//   - a QUEUED brief's out-of-repo declaration is collected;
//   - a NON-queued brief's declaration is not (a sibling nobody is about to need
//     is not an envelope failure, and counting it would red the desk for work it
//     is not doing);
//   - a ../<repo> mentioned in PROSE (an Evidence path) is not a declaration.
func TestPreflightQueuedSiblingsReadsDeclarationsOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := "| # | status |\n|---|---|\n| 01 | implemented |\n| 02 | done |\n| 03 | verified |\n"
	write(t, filepath.Join(dir, "README.md"), readme)
	write(t, filepath.Join(dir, "brief-01-a.md"), "files:\n- `x.go`\nout-of-repo files:\n- `../tracker/docs/y.md`\n")
	write(t, filepath.Join(dir, "brief-02-b.md"), "out-of-repo files:\n- `../not-queued/z.md`\n")
	write(t, filepath.Join(dir, "brief-03-c.md"),
		"out-of-repo files: none\n\n## Evidence\nran against ../some-sibling/report.md\n")

	got, err := QueuedSiblings(root)
	if err != nil {
		t.Fatalf("QueuedSiblings: %v", err)
	}
	var rels []string
	for _, r := range got {
		rels = append(rels, r.Rel)
	}
	if len(rels) != 1 || rels[0] != "../tracker" {
		t.Fatalf("declared siblings = %v, want exactly [../tracker]", rels)
	}
	if got[0].Brief == "" {
		t.Fatal("a declared sibling does not name the brief that declared it")
	}
}

// TestPreflightNoDeclaredSiblingsIsClean — "none" is the overwhelmingly common
// declaration and must be GREEN, not could-not-check. A check that is red for
// everybody gets switched off by everybody.
func TestPreflightNoDeclaredSiblingsIsClean(t *testing.T) {
	withRoster(t, goldenRoster())
	rep := runPF(t, okProbes())
	if got := pfCheck(t, rep, CheckSiblings).State; got != CheckedClean {
		t.Fatalf("%s = %s with no declared siblings, want checked-clean", CheckSiblings, got)
	}
}

// TestPreflightUnreadableQueueIsCouldNotCheck — no docs/streams/ means the queue
// could not be read, which is not the same as "no brief needs a sibling".
func TestPreflightUnreadableQueueIsCouldNotCheck(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.QueuedSiblings = nil // use the real scanner against an empty temp root
	rep := PreflightRequest{Role: pfRole, Root: t.TempDir(), Probes: p}.Run()
	if got := pfCheck(t, rep, CheckSiblings).State; got != CouldNotCheck {
		t.Fatalf("%s = %s with no docs/streams/, want could-not-check", CheckSiblings, got)
	}
}

// ---- the report contract ---------------------------------------------------

// TestPreflightEveryNonGreenCheckNamesARemediation — the reader must be able to
// ACT. A red line with no fix is a line that gets read as noise.
func TestPreflightEveryNonGreenCheckNamesARemediation(t *testing.T) {
	withRoster(t, goldenRoster())
	scenarios := map[string]func(*PreflightProbes){
		"mint": func(p *PreflightProbes) {
			p.ColdMint = func(string, string) (string, error) { return "", errors.New("x") }
		},
		"scopes": func(p *PreflightProbes) {
			p.GrantedScopes = func(string, string) (map[string]string, error) { return nil, errors.New("x") }
		},
		"transport": func(p *PreflightProbes) {
			p.WriteTransport = func(Landing) (ProbeVerdict, string, error) { return ProbeRejected, "403", nil }
		},
		"identity": func(p *PreflightProbes) { p.CommitEmail = func(string) (string, error) { return pfAppIDEmail, nil } },
		"siblings": func(p *PreflightProbes) {
			p.QueuedSiblings = func(string) ([]SiblingReq, error) { return []SiblingReq{{Brief: "b", Rel: "../x"}}, nil }
			p.DirExists = func(string) (bool, error) { return false, nil }
		},
	}
	for name, darken := range scenarios {
		p := okProbes()
		darken(&p)
		rep := runPF(t, p)
		blocking := rep.Blocking()
		if len(blocking) == 0 {
			t.Errorf("%s: darkening the probe produced a GREEN report", name)
			continue
		}
		for _, c := range blocking {
			if strings.TrimSpace(c.Remediation) == "" {
				t.Errorf("%s: non-green check %s carries no remediation", name, c.Name)
			}
		}
	}
}

// TestPreflightSummaryIsExactlyOneLine — the contract is "report ONE line and
// stop". A probe's multi-line stderr pasted into the boot message is how one line
// becomes forty and the desk's operator stops reading it.
func TestPreflightSummaryIsExactlyOneLine(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.WriteTransport = func(Landing) (ProbeVerdict, string, error) {
		return ProbeRejected, "remote: denied\nhint: line two\nhint: line three\n", nil
	}
	p.ColdMint = func(string, string) (string, error) {
		return "", errors.New("cold mint refused: line one\nline two\nline three")
	}
	rep := runPF(t, p)
	line := rep.SummaryLine()
	if strings.ContainsAny(line, "\n\r") {
		t.Fatalf("the summary is not one line:\n%s", line)
	}
	if err := rep.Err(); err == nil || strings.ContainsAny(err.Error(), "\n\r") {
		t.Fatalf("the refusal is not one line: %v", err)
	}
}

// TestPreflightRedIsCouldNotRunExitSix — both non-green states map to exit 6.
// From the pass's point of view "the envelope is broken" and "we could not tell"
// are the same fact: the pass did not run. Giving checked-failed a softer code is
// how a red envelope becomes a retried one.
func TestPreflightRedIsCouldNotRunExitSix(t *testing.T) {
	withRoster(t, goldenRoster())
	for name, darken := range map[string]func(*PreflightProbes){
		"checked-failed": func(p *PreflightProbes) {
			p.WriteTransport = func(Landing) (ProbeVerdict, string, error) { return ProbeRejected, "403", nil }
		},
		"could-not-check": func(p *PreflightProbes) {
			p.GrantedScopes = func(string, string) (map[string]string, error) { return nil, errors.New("no grant recorded") }
		},
	} {
		p := okProbes()
		darken(&p)
		err := runPF(t, p).Err()
		if err == nil {
			t.Fatalf("%s: no refusal", name)
		}
		if got := ExitCodeOf(err); got != ExitUnverifiable {
			t.Errorf("%s: exit %d, want %d (could-not-run)", name, got, ExitUnverifiable)
		}
	}
}

// TestPreflightRunsEveryCheckEvenAfterAFailure — the desk gets ONE line listing
// EVERY problem, not the first one. Reporting one failure at a time is how a
// broken envelope costs five boots instead of one.
func TestPreflightRunsEveryCheckEvenAfterAFailure(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.ColdMint = func(string, string) (string, error) { return "", errors.New("no key") }
	p.CommitEmail = func(string) (string, error) { return pfAppIDEmail, nil }
	rep := runPF(t, p)
	if len(rep.Checks) != 5 {
		t.Fatalf("stopped after %d checks; the desk must see the whole envelope in one line", len(rep.Checks))
	}
	line := rep.SummaryLine()
	for _, want := range []string{CheckColdMint, CheckCommitIdentity} {
		if !strings.Contains(line, want) {
			t.Errorf("summary omits the failed check %s: %s", want, line)
		}
	}
}

// TestPreflightSummaryNamesTheOwningIssue — a red envelope must NOT prompt a new
// issue: each failure already has one. The refs in the line are how the reader
// knows that without going to look.
func TestPreflightSummaryNamesTheOwningIssue(t *testing.T) {
	withRoster(t, goldenRoster())
	p := okProbes()
	p.CommitEmail = func(string) (string, error) { return pfAppIDEmail, nil }
	if line := runPF(t, p).SummaryLine(); !strings.Contains(line, "#638") {
		t.Fatalf("summary does not name the issue that already owns this failure: %s", line)
	}
}

// ---- the App-credential search path (#794) --------------------------------

// TestPreflightConfigHomeUnsetIsCompleteConfiguration — the shipped default alone
// is a working configuration. An adopter must not have to set a knob to mint.
func TestPreflightConfigHomeUnsetIsCompleteConfiguration(t *testing.T) {
	t.Setenv("HOME", "/h")
	os.Unsetenv(EnvConfigHome)
	dirs := ConfigHomeDirs()
	if len(dirs) != 1 || dirs[0] != filepath.Join("/h", ".config", "assay") {
		t.Fatalf("unset search path = %v, want just the shipped default", dirs)
	}
}

// TestPreflightConfigHomePrependsNeverReplaces — the knob adds a directory at the
// head; it never LOSES a file that resolves today. A knob that replaced the
// default would turn one misconfiguration into a fleet-wide outage, which is
// exactly what an unknown roster key did once already.
func TestPreflightConfigHomePrependsNeverReplaces(t *testing.T) {
	t.Setenv("HOME", "/h")
	t.Setenv(EnvConfigHome, "/h/.config/other")
	dirs := ConfigHomeDirs()
	if len(dirs) != 2 || dirs[0] != "/h/.config/other" || dirs[1] != filepath.Join("/h", ".config", "assay") {
		t.Fatalf("search path = %v, want [knob, shipped default]", dirs)
	}
}

// TestPreflightConfigHomeFindsAcrossThePath — the file is READ from wherever it
// is, and a NEW file is WRITTEN to the head, so read and write converge on one
// directory (#794's closing requirement).
func TestPreflightConfigHomeFindsAcrossThePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	other := filepath.Join(home, ".config", "other")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(other, "apps.env"), "VERIFIER_APP_ID=999\n")
	t.Setenv(EnvConfigHome, other)

	path, searched, found := FindConfigFile("apps.env")
	if !found {
		t.Fatalf("apps.env not found across %v", searched)
	}
	if path != filepath.Join(other, "apps.env") {
		t.Fatalf("resolved %q, want the knob directory's copy", path)
	}
	if got := ConfigHomeWritePath("verifier-token-1"); got != filepath.Join(other, "verifier-token-1") {
		t.Fatalf("write path %q is not at the head of the search path", got)
	}
	// The refusal must be able to name EVERY directory it looked in — a bare
	// "not found at <one path>" is the #794 symptom.
	if _, searched, found := FindConfigFile("nothing-here.pem"); found || len(searched) < 2 {
		t.Fatalf("a missing file resolved (%v) or named too few directories: %v", found, searched)
	}
}

// TestPreflightAppIDNamesEveryDirectorySearched — the diagnostic property, on the
// real resolver.
func TestPreflightAppIDNamesEveryDirectorySearched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfigHome, filepath.Join(home, ".config", "elsewhere"))
	os.Unsetenv("VERIFIER_APP_ID")
	_, err := AppID("verifier")
	if err == nil {
		t.Fatal("AppID resolved with no apps.env anywhere")
	}
	for _, want := range []string{"elsewhere", ".config/assay", "VERIFIER_APP_ID"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err.Error(), want)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
