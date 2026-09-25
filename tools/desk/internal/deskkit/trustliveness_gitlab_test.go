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

// TestClassifyLivenessGitLabMissingStateIsCouldNotCheck is the CLASS GUARD for
// pr1669-sec-F1: GitLab's users API always reports a `state` field, so a GitLab account
// response carrying an EMPTY state is a partial/malformed read, never confirmation the
// account is active. On the pre-fix code (classifyLiveness's state check reads
// `state != "" && state != "active"`) an empty state falls through every branch and
// classifies Alive — this is the negative control: an empty-state GitLab account must NEVER
// classify Alive, and must classify could-not-check specifically (not some other class).
func TestClassifyLivenessGitLabMissingStateIsCouldNotCheck(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "desk-worker", PinnedID: 5001, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			// id and login both match the pin; State is the zero value — exactly what a
			// partial/malformed GitLab response (or a stub omitting the field) looks like on
			// the wire.
			"desk-worker": {Login: "desk-worker", ID: 5001, State: ""},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Class == LivenessAlive {
		t.Fatalf("a GitLab account with no state field classified Alive — the negative control this test exists for")
	}
	if findings[0].Class != LivenessCouldNotCheck {
		t.Fatalf("GitLab account with no state field classified %q, want LivenessCouldNotCheck", findings[0].Class)
	}
}

// TestClassifyLivenessGitHubEmptyStateStillAlive is the POSITIVE control paired with the
// test above: GitHub's account read carries no state field at all (forge.go's Account.State
// doc: "never defaulted to active", but also never SET for GitHub), so an empty state on a
// GitHub identity must keep classifying Alive exactly as before — the sec-F1 fix is
// GitLab-scoped only, per the finding's own instruction to leave the GitHub path unchanged.
func TestClassifyLivenessGitHubEmptyStateStillAlive(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "alive-human", PinnedID: 1001, Source: "human"}, // Forge zero value = GitHub
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			"alive-human": {Login: "alive-human", ID: 1001, State: ""},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if findings[0].Class != LivenessAlive {
		t.Fatalf("GitHub identity with empty (never-set) state classified %q, want LivenessAlive — "+
			"the sec-F1 fix must not touch the GitHub path", findings[0].Class)
	}
}

// TestClassifyLivenessGitLabUnpinnedBlockedReportsSuspended is the CLASS GUARD for
// pr1669-F3: on the pre-fix code, classifyLiveness's `id.PinnedID == 0` branch returns
// LivenessUnpinned BEFORE the state check ever runs, so an unpinned GitLab identity whose
// account is blocked classifies unpinned (advisory) and the blocked state is never
// surfaced. This is the negative control: an unpinned + blocked GitLab identity must NEVER
// classify Unpinned, and must classify Suspended, with the state named in Detail.
func TestClassifyLivenessGitLabUnpinnedBlockedReportsSuspended(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "desk-unpinned", PinnedID: 0, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			"desk-unpinned": {Login: "desk-unpinned", ID: 77, State: "blocked"},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Class == LivenessUnpinned {
		t.Fatalf("an unpinned GitLab identity whose account is blocked classified Unpinned — " +
			"the negative control this test exists for; the blocked state was never surfaced")
	}
	if findings[0].Class != LivenessSuspended {
		t.Fatalf("unpinned + blocked GitLab account classified %q, want LivenessSuspended", findings[0].Class)
	}
	if !strings.Contains(findings[0].Detail, "blocked") {
		t.Fatalf("Detail %q does not name the blocked state", findings[0].Detail)
	}
}

// TestClassifyLivenessGitLabEmptyResultDeletedNamesHiddenCaveat is the CLASS GUARD for
// pr1669-F1: GitLab hides blocked/banned/ldap_blocked accounts from a non-admin token's user
// search entirely (UsersFinder#base_scope / FORBIDDEN_SEARCH_STATES upstream) — a desk
// forge credential (project/group/service-account token) IS a non-admin caller, so an empty
// `GET /users?username=` result does not unambiguously mean "deleted"; it can equally mean
// "hidden from this token". On the pre-fix code the DELETED Detail claims only "no longer
// resolves to any GitHub/GitLab account", with no such caveat — this test requires the
// caveat text to be present for a GitLab identity.
func TestClassifyLivenessGitLabEmptyResultDeletedNamesHiddenCaveat(t *testing.T) {
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
	detail := strings.ToLower(findings[0].Detail)
	for _, want := range []string{"blocked", "hidden"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("DELETED Detail %q does not name the non-admin-visibility caveat (missing %q) — "+
				"an empty GitLab result can mean deleted OR hidden from this token, and the notice "+
				"must say so, not claim unambiguous deletion", findings[0].Detail, want)
		}
	}
}

// TestRenderLivenessNoticesNamesGitLabForge is the CLASS GUARD for pr1669-F2/pr1669-sec-F3:
// on the pre-fix code, four strings in trustliveness.go hard-coded "GitHub" and "(404)" on
// what is now a forge-agnostic path, so a GitLab DELETED/RECLAIMED/could-not-check notice
// named the wrong forge and a 404 that never happened. This asserts a GitLab-identity
// DELETED and RECLAIMED notice both name GitLab, never GitHub, and never claim "(404)".
func TestRenderLivenessNoticesNamesGitLabForge(t *testing.T) {
	glID := RosterIdentity{Login: "svc2", PinnedID: 9, Source: "bot", Forge: ForgeGitLab}
	findings := []LivenessFinding{
		{Identity: glID, Class: LivenessDeleted, Detail: "d"},
		{Identity: glID, Class: LivenessReclaimed, Detail: "d"},
	}
	lines := RenderLivenessNotices(findings)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	for _, l := range lines {
		if strings.Contains(l, "GitHub") {
			t.Fatalf("GitLab identity notice names GitHub: %q", l)
		}
		if !strings.Contains(l, "GitLab") {
			t.Fatalf("GitLab identity notice does not name GitLab: %q", l)
		}
		if strings.Contains(l, "(404)") {
			t.Fatalf("GitLab identity notice claims a 404 that never happened: %q", l)
		}
	}
}

// TestClassifyLivenessGitLabDeletedDetailNamesGitLabNot404 mirrors the render test above at
// the classifier's own Detail string (before RenderLivenessNotices wraps it), for the
// DELETED and could-not-check branches inside classifyLiveness itself.
func TestClassifyLivenessGitLabDeletedDetailNamesGitLabNot404(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "ghost", PinnedID: 5004, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		errs: map[string]error{"ghost": ErrAccountNotFound},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if strings.Contains(findings[0].Detail, "GitHub") {
		t.Fatalf("GitLab DELETED Detail names GitHub: %q", findings[0].Detail)
	}
	if strings.Contains(findings[0].Detail, "(404)") {
		t.Fatalf("GitLab DELETED Detail claims a 404 that never happened: %q", findings[0].Detail)
	}

	identities2 := []RosterIdentity{
		{Login: "desk-worker", PinnedID: 5001, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher2 := &stubAccountFetcher{
		errs: map[string]error{"desk-worker": errors.New("500 internal server error")},
	}
	findings2 := CheckRosterLiveness(fetcher2, identities2)
	if strings.Contains(findings2[0].Detail, "GitHub") {
		t.Fatalf("GitLab could-not-check Detail names GitHub: %q", findings2[0].Detail)
	}
}

// TestHTTPGitLabAccountFetcherDoesNotFollowRedirect is the CLASS GUARD for pr1669-sec-F2:
// GetAccount sends PRIVATE-TOKEN through f.client(), which in production is
// http.DefaultClient (GitLabForge is constructed with a nil Client). Go's default client
// FOLLOWS 3xx responses and does NOT strip a custom PRIVATE-TOKEN header on a cross-host
// redirect (unlike Authorization, which IS stripped — the GitHub sibling is safe because it
// uses Authorization). A redirecting/misconfigured/hostile instance could otherwise receive
// the token. This drives GetAccount with Client left nil (the production path) against a
// primary server that 302s to a second server, and asserts (a) the second server never
// receives the request at all, and (b) GetAccount returns a non-nil, non-ErrAccountNotFound
// error — the redirect must be treated as could-not-check, never followed and never
// misread as "not found".
func TestHTTPGitLabAccountFetcherDoesNotFollowRedirect(t *testing.T) {
	var secondServerHit bool
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondServerHit = true
		if got := r.Header.Get("PRIVATE-TOKEN"); got != "" {
			t.Errorf("PRIVATE-TOKEN reached the redirect target: %q", got)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer second.Close()

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL+"/api/v4/users?username=desk-worker", http.StatusFound)
	}))
	defer first.Close()

	// Client deliberately left nil so this exercises the SAME client() fallback production
	// uses — a Client explicitly injected by a test (as every other test in this file does
	// via srv.Client()) would not prove anything about the production default.
	f := &HTTPGitLabAccountFetcher{Token: "tok", BaseURL: first.URL}
	_, err := f.GetAccount("desk-worker")
	if secondServerHit {
		t.Fatal("the redirect target was reached — PRIVATE-TOKEN was forwarded across a cross-host redirect")
	}
	if err == nil {
		t.Fatal("GetAccount followed a redirect silently and returned no error")
	}
	if err == ErrAccountNotFound {
		t.Fatal("a redirect classified as ErrAccountNotFound — must be a distinct could-not-check-shaped error, never DELETED")
	}
}

// TestClassifyLivenessGitLabReclaimedTakesPrecedenceOverState is the CLASS GUARD for
// pr1669-F4: the pr1669-F3 fix moved classifyAccountState ahead of BOTH the unpinned branch
// AND the acct.ID != PinnedID (reclaimed) branch, which is too broad — a pinned identity
// whose live account now resolves to a DIFFERENT id (a genuine account reclaim/squat) must
// classify LivenessReclaimed even when that different account's state is non-active, never
// LivenessSuspended. Reported reproduction: identity pinned to id 5001, live account comes
// back as {ID: 9999, State: "deactivated"} — the pre-fix code (state checked before
// id-mismatch) reports Suspended and the notice never mentions the id changed, which is the
// whole point of the Reclaimed class; a GitLab "deactivated" account reactivates when its
// owner signs back in, so "reactivate our bot" would be the wrong response for someone
// else's account. This asserts the result is Reclaimed, never Suspended, and that BOTH the
// pinned id (5001) and the live, mismatched id (9999) appear in the Detail.
func TestClassifyLivenessGitLabReclaimedTakesPrecedenceOverState(t *testing.T) {
	identities := []RosterIdentity{
		{Login: "desk-pinned", PinnedID: 5001, Source: "bot", Forge: ForgeGitLab},
	}
	fetcher := &stubAccountFetcher{
		accounts: map[string]*Account{
			"desk-pinned": {Login: "desk-pinned", ID: 9999, State: "deactivated"},
		},
	}
	findings := CheckRosterLiveness(fetcher, identities)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Class == LivenessSuspended {
		t.Fatalf("a pinned identity with an id-mismatched, non-active-state live account classified " +
			"Suspended — the negative control this test exists for; the id change was swallowed")
	}
	if findings[0].Class != LivenessReclaimed {
		t.Fatalf("id-mismatch + non-active state classified %q, want LivenessReclaimed", findings[0].Class)
	}
	detail := findings[0].Detail
	if !strings.Contains(detail, "5001") {
		t.Fatalf("Detail %q does not name the pinned id 5001", detail)
	}
	if !strings.Contains(detail, "9999") {
		t.Fatalf("Detail %q does not name the mismatched live id 9999", detail)
	}
}
