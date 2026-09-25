package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// livenessRoster configures one human, one bless authority (also a human), and one bot —
// enough for RosterIdentities to enumerate all three sources in one run.
const livenessRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=reviewer=assay-reviewer-app:300000004
`

// livenessGitLabRoster configures a GitHub bless/human authority (Configured() requires
// ASSAY_BLESS_LOGIN regardless of which forge the repo under test is on) plus two GitLab
// service-account bot identities via the forge-qualified ASSAY_TRUSTED_BOT_SLUGS grammar
// (forgeidentity.go: `[role=]<forge>:<slug-or-login>[:<id>]`) — the only source
// GitLabRosterIdentities reads.
const livenessGitLabRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=worker=gitlab:desk-worker:5001,reviewer=gitlab:desk-reviewer:5002
`

// TestLivenessCmdUnconfiguredRosterRefuses — an unconfigured roster must exit refused,
// printing ZERO findings for a reason explicitly named as "unconfigured" — never the same
// shape a clean, fully-configured run with no notices would print.
func TestLivenessCmdUnconfiguredRosterRefuses(t *testing.T) {
	installRosterEnv(t, "") // empty roster: unconfigured

	var out string
	err := runCaptured(t, &out, func() error {
		return cmdLiveness([]string{"--repo", "medici-finance/assay"})
	})
	if err == nil {
		t.Fatal("cmdLiveness passed on an unconfigured roster — must refuse")
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("cmdLiveness err = %v (%T), want Refused/exit 5", err, err)
	}
	if !strings.Contains(err.Error(), "unconfigured") {
		t.Fatalf("refusal %q does not name the reason as unconfigured", err.Error())
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("unconfigured refusal printed findings to stdout: %q", out)
	}
}

// TestLivenessCmdReportsFindingsForConfiguredIdentities — a configured roster against a
// stubbed GitHub forge prints one NOTICE per non-Alive identity, none for Alive ones, exit
// OK.
func TestLivenessCmdReportsFindingsForConfiguredIdentities(t *testing.T) {
	installRosterEnv(t, livenessRoster)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ada"):
			// Bless AND human entry for ada — alive, no notice.
			_, _ = w.Write([]byte(`{"id":2001,"login":"ada"}`))
		case strings.HasSuffix(r.URL.Path, "/shared-agent"):
			// Reclaimed: different id than pinned (2002).
			_, _ = w.Write([]byte(`{"id":424242,"login":"shared-agent"}`))
		case strings.HasSuffix(r.URL.Path, "/assay-reviewer-app[bot]"):
			// Deleted — but ONLY at the account's correct, [bot]-suffixed REST login
			// (assay#1665: GitHub's REST /users/{login} never resolves a GitHub App under its
			// bare slug — only under "<slug>[bot]" — so a real deletion 404s here, not at the
			// bare-slug path below).
			w.WriteHeader(http.StatusNotFound)
		default:
			// assay#1665's defect class: a bot identity must be probed at "<slug>[bot]",
			// never the bare roster-configured slug. A request for the bare
			// "/assay-reviewer-app" path (or anything else unexpected) falls through here and
			// fails the test loudly, same as any other unexpected lookup path.
			t.Errorf("unexpected liveness lookup path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	orig := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return &deskkit.GitHubForge{Token: "t", BaseURL: srv.URL, Client: srv.Client()},
			deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"}, nil
	}
	t.Cleanup(func() { forgeFor = orig })

	var out string
	err := runCaptured(t, &out, func() error {
		return cmdLiveness([]string{"--repo", "medici-finance/assay"})
	})
	if err != nil {
		t.Fatalf("cmdLiveness: %v", err)
	}
	if strings.Contains(out, "ada") {
		t.Errorf("an alive identity (ada) produced output: %q", out)
	}
	if !strings.Contains(out, "RECLAIMED") || !strings.Contains(out, "shared-agent") {
		t.Errorf("missing RECLAIMED notice for shared-agent: %q", out)
	}
	if !strings.Contains(out, "DELETED") || !strings.Contains(out, "assay-reviewer-app") {
		t.Errorf("missing DELETED notice for assay-reviewer-app: %q", out)
	}
	// Exactly two NOTICE lines: reclaimed + deleted, none for the two alive ada entries
	// (human + bless).
	if n := strings.Count(out, "NOTICE:"); n != 2 {
		t.Errorf("got %d NOTICE lines, want 2:\n%s", n, out)
	}
}

// TestLivenessCmdGitLabForgeRunsTheCheck — assay#1667. A GitLab-backed repo now RUNS the
// liveness check (through *deskkit.GitLabForge and GitLabRosterIdentities), instead of
// printing the stale GitHub-only notice. This is what pre-fix code fails: run against the
// merge-base (the old `gf, ok := f.(*deskkit.GitHubForge)` type assertion), a GitLab forge
// fell straight into the "not ok" branch and printed the could-not-check GitHub-only line
// with 0 identities checked — see the PR's "## Fail-first" section for the red run this
// test produces on that code.
func TestLivenessCmdGitLabForgeRunsTheCheck(t *testing.T) {
	installRosterEnv(t, livenessGitLabRoster)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GitLab usernames never get a "[bot]" suffix (assay#1665/#1666 is GitHub-only) —
		// fail loudly if this ever probes a suffixed form.
		switch username := r.URL.Query().Get("username"); username {
		case "desk-worker":
			// Alive: resolves to the pinned id, active state.
			_, _ = w.Write([]byte(`[{"id":5001,"username":"desk-worker","state":"active"}]`))
		case "desk-reviewer":
			// Resolves to the pinned id, but BLOCKED — must not be reported as alive.
			_, _ = w.Write([]byte(`[{"id":5002,"username":"desk-reviewer","state":"blocked"}]`))
		default:
			t.Errorf("unexpected liveness lookup username: %q (path %s)", username, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	orig := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return &deskkit.GitLabForge{Token: "t", BaseURL: srv.URL, Client: srv.Client()},
			deskkit.ForgeRepo{Owner: "example-org", Name: "on-gitlab"}, nil
	}
	t.Cleanup(func() { forgeFor = orig })

	var out string
	err := runCaptured(t, &out, func() error {
		return cmdLiveness([]string{"--repo", "example-org/on-gitlab"})
	})
	if err != nil {
		t.Fatalf("cmdLiveness on a GitLab forge: %v", err)
	}
	if strings.Contains(out, "GitHub-only") {
		t.Fatalf("GitLab forge still printed the stale GitHub-only notice: %q", out)
	}
	if strings.Contains(out, "desk-worker") {
		t.Errorf("an alive GitLab identity (desk-worker) produced output: %q", out)
	}
	if !strings.Contains(out, "SUSPENDED") || !strings.Contains(out, "desk-reviewer") {
		t.Errorf("missing SUSPENDED notice for the blocked desk-reviewer: %q", out)
	}
	if n := strings.Count(out, "NOTICE:"); n != 1 {
		t.Errorf("got %d NOTICE lines, want 1 (only the blocked identity):\n%s", n, out)
	}
}

// TestLivenessCmdUnknownForgeIsCouldNotCheck — a repo served by neither GitHub nor GitLab
// produces an explicit could-not-check NOTICE, never silence and never a refusal (the
// roster itself may be perfectly configured; only THIS repo's forge has no liveness
// implementation). Replaces the pre-#1667 GitHub-only-gap test now that GitLab is a real,
// supported second arm rather than the unsupported case.
func TestLivenessCmdUnknownForgeIsCouldNotCheck(t *testing.T) {
	installRosterEnv(t, livenessRoster)

	orig := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		// Any Forge implementation that is neither *deskkit.GitHubForge nor
		// *deskkit.GitLabForge exercises the default arm. fakeRosterForge (forge_test.go)
		// fits.
		return &fakeRosterForge{}, deskkit.ForgeRepo{Owner: "example-org", Name: "on-something-else"}, nil
	}
	t.Cleanup(func() { forgeFor = orig })

	var out string
	err := runCaptured(t, &out, func() error {
		return cmdLiveness([]string{"--repo", "example-org/on-something-else"})
	})
	if err != nil {
		t.Fatalf("cmdLiveness on an unrecognised forge must not refuse (the roster may be fine): %v", err)
	}
	if !strings.Contains(out, "could-not-check") {
		t.Fatalf("unrecognised forge did not report could-not-check: %q", out)
	}
	if strings.Contains(out, "#933") {
		t.Fatalf("unrecognised-forge notice still cites the stale #933 pointer: %q", out)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("unrecognised forge produced silence — must print exactly one explicit line")
	}
}

// runCaptured runs fn with stdout captured into *out, returning fn's error. A thin wrapper
// over captureStdout (width_test.go) for the commands whose return value this suite also
// needs to assert on.
func runCaptured(t *testing.T, out *string, fn func() error) error {
	t.Helper()
	var err error
	*out = captureStdout(t, func() {
		err = fn()
	})
	return err
}
