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
		case strings.HasSuffix(r.URL.Path, "/assay-reviewer-app"):
			// Deleted.
			w.WriteHeader(http.StatusNotFound)
		default:
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

// TestLivenessCmdNonGitHubForgeNamesTheGap — a non-GitHub-backed repo produces the explicit
// "GitHub-only" line, never silence and never a refusal (the roster itself may be perfectly
// configured; only THIS repo's forge is unsupported).
func TestLivenessCmdNonGitHubForgeNamesTheGap(t *testing.T) {
	installRosterEnv(t, livenessRoster)

	orig := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		// Any Forge implementation that is not *deskkit.GitHubForge exercises the type
		// assertion's failure path. fakeRosterForge (forge_test.go) fits.
		return &fakeRosterForge{}, deskkit.ForgeRepo{Owner: "example-org", Name: "on-gitlab"}, nil
	}
	t.Cleanup(func() { forgeFor = orig })

	var out string
	err := runCaptured(t, &out, func() error {
		return cmdLiveness([]string{"--repo", "example-org/on-gitlab"})
	})
	if err != nil {
		t.Fatalf("cmdLiveness on a non-GitHub forge must not refuse (the roster may be fine): %v", err)
	}
	if !strings.Contains(out, "GitHub-only") {
		t.Fatalf("non-GitHub forge did not name the gap: %q", out)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("non-GitHub forge produced silence — must print exactly one explicit line")
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
