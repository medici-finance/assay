package deskkit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPGitLabAccountFetcherAlive drives GetAccount against a stub GET
// /api/v4/users?username=<name> response carrying one matching entry — the ordinary alive
// case — and checks the account fields land untouched (id, username, state).
func TestHTTPGitLabAccountFetcherAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v4/users" {
			t.Errorf("unexpected path %q", got)
		}
		if got := r.URL.Query().Get("username"); got != "desk-worker" {
			t.Errorf("unexpected username query %q", got)
		}
		if got := r.Header.Get("PRIVATE-TOKEN"); got != "tok" {
			t.Errorf("PRIVATE-TOKEN header = %q, want tok", got)
		}
		_, _ = w.Write([]byte(`[{"id":5001,"username":"desk-worker","state":"active"}]`))
	}))
	defer srv.Close()

	// A BaseURL pointed at the test server exercises the self-hosted-instance path: the
	// fetcher must never hardcode gitlab.com.
	f := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: srv.URL, Client: srv.Client()}
	acct, err := f.GetAccount("desk-worker")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.ID != 5001 || acct.Login != "desk-worker" || acct.State != "active" {
		t.Fatalf("got %+v, want id=5001 login=desk-worker state=active", acct)
	}
}

// TestHTTPGitLabAccountFetcherNotFound — an EMPTY result list (200 OK, `[]`) is GitLab's
// "no such account" signal on this endpoint (it never 404s on a username with no match).
// This must classify as ErrAccountNotFound, not a generic transport error.
func TestHTTPGitLabAccountFetcherNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	f := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: srv.URL, Client: srv.Client()}
	_, err := f.GetAccount("ghost")
	if err != ErrAccountNotFound {
		t.Fatalf("GetAccount err = %v, want ErrAccountNotFound", err)
	}
}

// TestHTTPGitLabAccountFetcherServerErrorIsCouldNotCheck — a stubbed 5xx must classify as a
// transport-shaped error DISTINCT from ErrAccountNotFound, so the caller reports
// could-not-check, never "deleted".
func TestHTTPGitLabAccountFetcherServerErrorIsCouldNotCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: srv.URL, Client: srv.Client()}
	_, err := f.GetAccount("desk-worker")
	if err == nil {
		t.Fatal("GetAccount returned no error on a stubbed 5xx")
	}
	if err == ErrAccountNotFound {
		t.Fatal("a 5xx classified as ErrAccountNotFound — must be a distinct, could-not-check-shaped error")
	}
}

// TestHTTPGitLabAccountFetcherAuthError — a stubbed 401 (bad/expired token) is likewise
// could-not-check, never "deleted".
func TestHTTPGitLabAccountFetcherAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	f := &HTTPGitLabAccountFetcher{Token: "bad", BaseURL: srv.URL, Client: srv.Client()}
	_, err := f.GetAccount("desk-worker")
	if err == nil || err == ErrAccountNotFound {
		t.Fatalf("GetAccount on a 401 = %v, want a distinct could-not-check error", err)
	}
}

// TestHTTPGitLabAccountFetcherBlockedState confirms the wire "state" field survives onto
// Account.State untouched, for a blocked account — classifyLiveness is the one that turns
// this into a finding; this test only pins the fetcher's own contract.
func TestHTTPGitLabAccountFetcherBlockedState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":5002,"username":"desk-reviewer","state":"blocked"}]`))
	}))
	defer srv.Close()

	f := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: srv.URL, Client: srv.Client()}
	acct, err := f.GetAccount("desk-reviewer")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.State != "blocked" {
		t.Fatalf("acct.State = %q, want blocked", acct.State)
	}
}

// TestHTTPGitLabAccountFetcherDeactivatedState mirrors the blocked test for "deactivated" —
// the issue names both states explicitly as never-alive.
func TestHTTPGitLabAccountFetcherDeactivatedState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":5003,"username":"desk-bot","state":"deactivated"}]`))
	}))
	defer srv.Close()

	f := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: srv.URL, Client: srv.Client()}
	acct, err := f.GetAccount("desk-bot")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.State != "deactivated" {
		t.Fatalf("acct.State = %q, want deactivated", acct.State)
	}
}

// TestHTTPGitLabAccountFetcherUsesDefaultHost confirms an empty BaseURL falls back to
// GitLabAPIBase (gitlab.com), the same "" -> default resolution the GitHub fetcher and
// GitLabForge.baseURL both apply — this fetcher must never silently target a different
// host. Checked via the constructed request address rather than a live call.
func TestHTTPGitLabAccountFetcherUsesDefaultHost(t *testing.T) {
	f := &HTTPGitLabAccountFetcher{Token: "tok"}
	if got := f.baseURL(); got != GitLabAPIBase {
		t.Fatalf("baseURL() = %q, want %q", got, GitLabAPIBase)
	}
}

// TestHTTPGitLabAccountFetcherHonoursSelfHostedInstance confirms a self-hosted instance
// root (anything other than gitlab.com) is honoured verbatim — the whole point of taking
// BaseURL as a field rather than hardcoding GitLabAPIBase.
func TestHTTPGitLabAccountFetcherHonoursSelfHostedInstance(t *testing.T) {
	f := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: "https://gitlab.example.internal"}
	if got, want := f.baseURL(), "https://gitlab.example.internal"; got != want {
		t.Fatalf("baseURL() = %q, want %q", got, want)
	}
	// A trailing slash on the configured instance root must not double up against the
	// literal "/api/v4/users" this fetcher appends.
	f2 := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: "https://gitlab.example.internal/"}
	if got, want := f2.baseURL(), "https://gitlab.example.internal"; got != want {
		t.Fatalf("baseURL() with trailing slash = %q, want %q", got, want)
	}
}

// TestGitLabRosterIdentitiesFiltersByForge confirms GitLabRosterIdentities reads ONLY
// Config.BotIdents entries whose Forge is ForgeGitLab, in sorted order, and never reads
// Config.Bots (the GitHub-only flat view) or a GitHub-forge BotIdents entry.
func TestGitLabRosterIdentitiesFiltersByForge(t *testing.T) {
	cfg := Config{
		Bots: map[string]int64{"github-only-app": 111}, // must never surface here
		BotIdents: map[string]BotIdentity{
			"zzz-gitlab-bot":    {Forge: ForgeGitLab, Slug: "zzz-gitlab-bot", ID: 900},
			"aaa-gitlab-bot":    {Forge: ForgeGitLab, Slug: "aaa-gitlab-bot", ID: 800},
			"github-only-app":   {Forge: ForgeGitHub, Slug: "github-only-app", ID: 111},
			"inferred-github-x": {Forge: ForgeGitHub, ForgeInferred: true, Slug: "inferred-github-x", ID: 222},
		},
	}
	ids := GitLabRosterIdentities(cfg)
	var logins []string
	for _, id := range ids {
		logins = append(logins, id.Login)
		if id.Forge != ForgeGitLab {
			t.Errorf("identity %q carries Forge %q, want gitlab", id.Login, id.Forge)
		}
		if id.Source != "bot" {
			t.Errorf("identity %q carries Source %q, want bot", id.Login, id.Source)
		}
	}
	want := []string{"aaa-gitlab-bot", "zzz-gitlab-bot"}
	if len(logins) != len(want) {
		t.Fatalf("got %v, want %v", logins, want)
	}
	for i := range want {
		if logins[i] != want[i] {
			t.Fatalf("got %v, want %v", logins, want)
		}
	}
}

// TestClassifyLivenessGitLabBlockedIsNotAlive is the end-to-end classifier test (stub
// fetcher, not the HTTP one) pinning the issue's Verify row: "a stubbed state: blocked is
// not classified alive". Uses a GitLab-forged identity so probeLogin does not suffix the
// login, matching GitLabRosterIdentities' real output shape.
func TestClassifyLivenessGitLabBlockedIsNotAlive(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "desk-reviewer", PinnedID: 5002, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			"desk-reviewer": {Login: "desk-reviewer", ID: 5002, State: "blocked"},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Class == LivenessAlive {
		t.Fatalf("a blocked GitLab account classified Alive — the negative control this test exists for")
	}
	if findings[0].Class != LivenessSuspended {
		t.Fatalf("blocked account classified %q, want LivenessSuspended", findings[0].Class)
	}
	if !strings.Contains(findings[0].Detail, "blocked") {
		t.Fatalf("Detail %q does not name the blocked state", findings[0].Detail)
	}
}

// TestClassifyLivenessGitLabDeactivatedIsNotAlive mirrors the blocked test for
// "deactivated".
func TestClassifyLivenessGitLabDeactivatedIsNotAlive(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "desk-bot", PinnedID: 5003, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			"desk-bot": {Login: "desk-bot", ID: 5003, State: "deactivated"},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if findings[0].Class != LivenessSuspended {
		t.Fatalf("deactivated account classified %q, want LivenessSuspended", findings[0].Class)
	}
}

// TestClassifyLivenessGitLabActiveIsAlive is the positive control: an active GitLab
// account with the right id and login classifies Alive exactly like a GitHub one.
func TestClassifyLivenessGitLabActiveIsAlive(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "desk-worker", PinnedID: 5001, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			"desk-worker": {Login: "desk-worker", ID: 5001, State: "active"},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if findings[0].Class != LivenessAlive {
		t.Fatalf("active account classified %q, want LivenessAlive", findings[0].Class)
	}
}

// TestClassifyLivenessGitLabNotFoundIsDeleted confirms a GitLab 404-equivalent (surfaced
// by the fetcher as ErrAccountNotFound) classifies DELETED, same as GitHub.
func TestClassifyLivenessGitLabNotFoundIsDeleted(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "ghost", PinnedID: 5004, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		errs: map[string]error{"ghost": ErrAccountNotFound},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if findings[0].Class != LivenessDeleted {
		t.Fatalf("not-found GitLab account classified %q, want LivenessDeleted", findings[0].Class)
	}
}

// TestClassifyLivenessGitLabTransportErrorIsCouldNotCheck confirms a transport/auth error
// on a GitLab identity classifies could-not-check, never deleted.
func TestClassifyLivenessGitLabTransportErrorIsCouldNotCheck(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "desk-worker", PinnedID: 5001, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		errs: map[string]error{"desk-worker": errors.New("500 internal server error")},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if findings[0].Class != LivenessCouldNotCheck {
		t.Fatalf("transport error classified %q, want LivenessCouldNotCheck", findings[0].Class)
	}
}

// TestProbeLoginNeverSuffixesGitLab confirms probeLogin leaves a GitLab bot login bare —
// GitLab has no "[bot]"-decorated rendering (assay#1665/#1666 is GitHub-only).
func TestProbeLoginNeverSuffixesGitLab(t *testing.T) {
	got := probeLogin(RosterIdentity{Login: "desk-worker", Source: "bot", Forge: ForgeGitLab})
	if got != "desk-worker" {
		t.Fatalf("probeLogin(GitLab bot) = %q, want the bare login unsuffixed", got)
	}
}
