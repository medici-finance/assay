package deskkit

// trustliveness.go — a read-only, fail-closed NOTICE surface that asks a trusted login's
// own forge what it currently says about that login, WITHOUT touching TrustedAuthor's
// verdict. The classifier, the RosterIdentity/LivenessClass/LivenessFinding shapes and
// RenderLivenessNotices are forge-agnostic and shared; the GitHub fetcher lives in this
// file (HTTPAccountFetcher) and the GitLab fetcher in trustliveness_gitlab.go
// (HTTPGitLabAccountFetcher, assay#1667) — see GitLabRosterIdentities below for the
// GitLab identity source and cmd/deskroster/liveness.go for the forge-typed dispatch that
// picks between them.
//
// trust.go's TrustedAuthor / TrustedHumanAuthor / Blessed compare a login (and, where
// pinned, its numeric id) against the CONFIGURED roster — a pure string/id comparison that
// never asks whether the GitHub account behind that login is still there. A deleted,
// renamed, or reclaimed trusted login keeps being honored exactly as before, indefinitely,
// because nothing ever re-checks it against the live account. This file adds that check —
// `deskroster liveness` prints a NOTICE when something changed — so a human has a chance to
// notice a stale or reclaimed login before it is abused.
//
// This deliberately changes nothing about who is trusted today: TrustedAuthor and its
// siblings are untouched, and nothing in this file is consulted by any of them (see the
// cross-reference doc-comment in trust.go, and the absence-grep this brief's Verify row 10
// pins). Wiring a liveness finding into that pass/fail verdict is a separate,
// explicitly human-gated decision — not this file.
//
// DEVIATION FROM THE BRIEF, RECORDED HERE (verify-before-applying, worker-kit clause 7).
// The brief specified GetAccount as a plain method directly on *GitHubForge, "modeled on
// RepoInfoFetcher" and deliberately kept off the frozen Forge interface. That placement
// does not compile clean against this tree: forge_surface_test.go's
// TestForgeNoPassthrough/neither_backend_exports_a_method_outside_the_interface (already
// present at the brief's own cited freshness-check commit b35225c6 — a pre-existing
// invariant, not a new one added by this change) asserts that *GitHubForge's and
// *GitLabForge's EXPORTED method sets equal the Forge interface's exactly — no exceptions,
// checked by reflection, not by convention. An exported GetAccount on *GitHubForge trips it
// directly (confirmed by running the suite).
//
// The brief's own cited precedent is actually the fix: RepoVisibility (repovis.go) is NOT
// a method on *GitHubForge either — it lives on HTTPRepoInfoFetcher, a small STANDALONE
// fetcher struct that is not a Forge implementer at all, so the closed-surface test never
// sees it. HTTPAccountFetcher below is that same shape for account liveness: a plain
// struct, not a GitHubForge method, satisfying AccountFetcher without adding anything to
// GitHubForge's method set or the frozen Forge interface — which is exactly what the brief
// asked for ("no interface-freeze churn, no GitLab implementation obligation"), just placed
// where the existing closed-surface invariant actually allows it to live.
//
// The consumer (cmd/deskroster/liveness.go) still resolves the repo's Forge through the
// existing forgeFor seam to (a) decide whether the repo is GitHub-backed at all, and (b)
// read the already-minted Token/BaseURL/Client off the resolved *GitHubForge value — plain
// exported FIELD reads, not a method call, so nothing here reaches through GitHubForge's
// method set — then hands those to HTTPAccountFetcher.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// ErrAccountNotFound is the canonical signal that a login no longer resolves to any account
// on its forge (a 404 from GitHub's GET /users/{login}, or an EMPTY result from GitLab's
// GET /users?username=). It is a distinct KNOWN state — the account was deleted, the login
// is simply gone, OR (GitLab non-admin callers only, see classifyLiveness's DELETED Detail
// for a GitLab identity) the account is blocked/banned/ldap_blocked and hidden from this
// token — never conflated with a transport failure: see HTTPAccountFetcher.GetAccount,
// HTTPGitLabAccountFetcher.GetAccount, and LivenessDeleted vs LivenessCouldNotCheck below.
var ErrAccountNotFound = errors.New("forge account not found")

// AccountFetcher is the minimal GitHub API surface the liveness check needs: one login in,
// one current account (or a classified error) out. Commands provide the real
// (token-authenticated) implementation (HTTPAccountFetcher below); tests provide a stub.
type AccountFetcher interface {
	// GetAccount returns the CURRENT account behind login. On a 404 (login no longer
	// resolves to any account) it returns (nil, ErrAccountNotFound); any other failure
	// (transport, auth, 5xx) is returned as its own error, distinct from
	// ErrAccountNotFound, so a caller can tell "deleted" apart from "could not tell".
	GetAccount(login string) (*Account, error)
}

// HTTPAccountFetcher implements AccountFetcher with a direct REST call to GitHub using a
// bearer token — the production implementation `deskroster liveness` runs on. Same shape as
// HTTPRepoInfoFetcher (repovis.go): BaseURL defaults to GitHubAPIBase, Client defaults to
// http.DefaultClient, so a test points BaseURL at an httptest server.
type HTTPAccountFetcher struct {
	Token   string
	BaseURL string // defaults to GitHubAPIBase when empty
	Client  *http.Client
}

func (f *HTTPAccountFetcher) baseURL() string {
	return GitHubBaseURLOrDefault(f.BaseURL)
}

func (f *HTTPAccountFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

// GetAccount calls GET /users/{login} and returns the CURRENT id/login GitHub reports for
// it. On a 404 it returns (nil, ErrAccountNotFound); any other non-2xx or transport/decode
// failure is returned as its own distinct error — the caller (CheckRosterLiveness) is the
// one that classifies "deleted" apart from "could not tell", not this method.
func (f *HTTPAccountFetcher) GetAccount(login string) (*Account, error) {
	reqURL := fmt.Sprintf("%s/users/%s", f.baseURL(), url.PathEscape(login))
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+f.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrAccountNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", reqURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wire struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("cannot parse GitHub account response for %q: %w", login, err)
	}
	if wire.Login == "" {
		return nil, fmt.Errorf("GitHub account response for %q carries no .login field", login)
	}
	return &Account{Login: wire.Login, ID: wire.ID}, nil
}

var _ AccountFetcher = (*HTTPAccountFetcher)(nil)

// RosterIdentity is one configured identity worth checking for liveness: a login, its
// pinned numeric id (0 = unpinned), and which roster map it came from.
type RosterIdentity struct {
	Login    string
	PinnedID int64
	// Source names which roster map this identity came from: "human", "bless", or "bot".
	Source string
	// Forge names which forge this identity lives on. The zero value ("") is read as
	// ForgeGitHub — every identity RosterIdentities (below) enumerates IS GitHub-scoped, and
	// every existing call site (including every literal in this package's own tests)
	// predates this field, so leaving it unset must keep meaning exactly what it always
	// meant. GitLabRosterIdentities is the one enumerator that sets it explicitly to
	// ForgeGitLab. probeLogin and classifyLiveness are the two readers.
	Forge ForgeKind
}

// isGitLab reports whether id is GitLab-scoped, treating the zero value as GitHub (see the
// Forge field's doc comment above).
func (id RosterIdentity) isGitLab() bool {
	return id.Forge == ForgeGitLab
}

// RosterIdentities enumerates every GitHub-scoped identity worth checking: Config.Humans,
// Config.Bless (when configured), and Config.Bots — nothing else. A GitLab identity lives
// exclusively in Config.BotIdents (filtered to Forge == ForgeGitLab) and is out of scope
// here — see GitLabRosterIdentities below, trustliveness_gitlab.go's account fetcher, and
// cmd/deskroster/liveness.go's forge-typed dispatch — so reading only these three maps
// needs no forge filtering.
//
// The order is deterministic (sorted within each source, humans then bless then bots) so
// RenderLivenessNotices's output is stable across runs for a diff-friendly NOTICE stream.
func RosterIdentities(c Config) []RosterIdentity {
	var out []RosterIdentity

	humanLogins := make([]string, 0, len(c.Humans))
	for login := range c.Humans {
		humanLogins = append(humanLogins, login)
	}
	sort.Strings(humanLogins)
	for _, login := range humanLogins {
		out = append(out, RosterIdentity{Login: login, PinnedID: c.Humans[login], Source: "human", Forge: ForgeGitHub})
	}

	if c.Bless.Login != "" {
		out = append(out, RosterIdentity{Login: c.Bless.Login, PinnedID: c.Bless.ID, Source: "bless", Forge: ForgeGitHub})
	}

	botLogins := make([]string, 0, len(c.Bots))
	for login := range c.Bots {
		botLogins = append(botLogins, login)
	}
	sort.Strings(botLogins)
	for _, login := range botLogins {
		out = append(out, RosterIdentity{Login: login, PinnedID: c.Bots[login], Source: "bot", Forge: ForgeGitHub})
	}

	return out
}

// GitLabRosterIdentities enumerates every GitLab-scoped identity worth checking:
// Config.BotIdents entries whose Forge is ForgeGitLab — GitLab service accounts, the ONLY
// identity class a GitLab roster carries. Config.Humans/Config.Bless are the GitHub human
// roster (RosterIdentities above) and carry no forge tag of their own; Config.Bots is the
// GitHub-slug-keyed flat view (rosterconfig.go: "Bots ... carries GITHUB entries only").
// Neither belongs here, so BotIdents filtered to Forge == ForgeGitLab is the complete,
// correct source — never Config.Bots, and never an unfiltered walk of Config.BotIdents
// (which also carries the GitHub entries the GitHub path already checks).
//
// The result is sorted by login for the same diff-friendly, deterministic-order reason
// RosterIdentities documents.
func GitLabRosterIdentities(c Config) []RosterIdentity {
	var out []RosterIdentity
	for _, b := range c.BotIdents {
		if b.Forge != ForgeGitLab {
			continue
		}
		out = append(out, RosterIdentity{Login: b.Slug, PinnedID: b.ID, Source: "bot", Forge: ForgeGitLab})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Login < out[j].Login })
	return out
}

// LivenessClass classifies what GitHub currently says about one configured identity,
// relative to what the roster has pinned for it.
type LivenessClass string

const (
	// LivenessAlive: the login still resolves, to the pinned id, under the pinned login
	// spelling. Nothing to report — RenderLivenessNotices emits no line for this class.
	LivenessAlive LivenessClass = "alive"
	// LivenessRenamed: the login still resolves to the PINNED id, but GitHub's current
	// canonical login for that id differs from the one configured. Advisory: the account is
	// the same one, its login spelling moved.
	LivenessRenamed LivenessClass = "renamed"
	// LivenessReclaimed: the login resolves, but to a DIFFERENT id than the one pinned — a
	// NEW account now answers to this login. This is one of the two classes the liveness
	// check exists to catch.
	LivenessReclaimed LivenessClass = "reclaimed"
	// LivenessDeleted: the login no longer resolves to any account (a 404). The other class
	// the liveness check exists to catch.
	LivenessDeleted LivenessClass = "deleted"
	// LivenessUnpinned: the login resolves to SOME account, but the roster carries no
	// pinned id for it (PinnedID == 0), so identity continuity cannot be checked at all.
	// Advisory: recommend pinning.
	LivenessUnpinned LivenessClass = "unpinned"
	// LivenessSuspended: the account resolves, but the forge reports it in any non-"active"
	// state — GitLab's "deactivated", "blocked_pending_approval", and so on (assay#1667). In
	// practice this fires for the GitLab states a non-admin token's user search does NOT hide
	// — "blocked"/"banned"/"ldap_blocked" are hidden entirely from that caller and surface as
	// LivenessDeleted instead (pr1669-F1; see the DELETED Detail for a GitLab identity). This
	// class is a structural no-op on the GitHub path: Account.State stays "" on every
	// GitHub-sourced read (forge_github.go's account read exposes no such field), so
	// classifyLiveness never produces it there. Never coerced into LivenessAlive, and checked
	// BEFORE identity-continuity (unpinned/reclaimed/renamed), so an unpinned or reclaimed
	// GitLab identity's non-active state is still surfaced (pr1669-F3) — a non-active account
	// still resolves and may still carry the right id, but it is not a live trusted identity.
	LivenessSuspended LivenessClass = "suspended"
	// LivenessCouldNotCheck: the fetcher failed for any reason OTHER than a clean 404 —
	// transport error, timeout, auth failure, malformed response. NEVER coerced into
	// LivenessAlive, and never conflated with LivenessDeleted: a stuck-open bisector's
	// first question is always "did it not exist, or did we just fail to ask?"
	LivenessCouldNotCheck LivenessClass = "could_not_check"
)

// LivenessFinding is one identity's classification, with a human-readable Detail.
type LivenessFinding struct {
	Identity RosterIdentity
	Class    LivenessClass
	Detail   string
}

// CheckRosterLiveness classifies every identity independently: one finding per identity,
// pure (no I/O beyond the fetcher call), and one identity's fetcher error NEVER suppresses
// or short-circuits the findings for any other identity in the same run.
func CheckRosterLiveness(fetcher AccountFetcher, identities []RosterIdentity) []LivenessFinding {
	findings := make([]LivenessFinding, 0, len(identities))
	for _, id := range identities {
		findings = append(findings, classifyLiveness(fetcher, id))
	}
	return findings
}

// probeLogin returns the login classifyLiveness sends to GetAccount for id — DEFECT CLASS:
// a bot-login comparison/lookup that does not normalize the "[bot]" suffix between a
// bare-slug source and a REST-shaped target. RosterIdentities enumerates bot identities from
// Config.Bots, which is keyed on the App's BARE slug (rosterconfig.go: "Bots maps a
// lowercased GitHub App slug to its BOT USER id"). GitHub's REST GET /users/{login} endpoint
// only resolves a GitHub App's bot account under its "[bot]"-suffixed rendering — GET
// /users/<slug> 404s for every App, GET /users/<slug>[bot] succeeds — so probing the bare
// slug reported every trusted bot as LivenessDeleted (assay#1665).
//
// The suffix is GitHub-only (assay#1667): a GitLab service account has no decorated
// rendering at all (forgeidentity.go's BotIdentity.PrimaryLogin — the username IS the
// identity GitLab's API attributes things to), so GitLabRosterIdentities' entries must be
// probed at their bare login, never suffixed. id.isGitLab() is what tells the two apart —
// every RosterIdentities entry defaults to GitHub (the Forge field's zero value), every
// GitLabRosterIdentities entry sets ForgeGitLab explicitly.
//
// This is the SAME normalization gap forge_github.go's GraphQL comment/PR-state readers
// already guard against on the read side (forge_github.go ~line 1576: "A GraphQL Bot actor
// carries the BARE slug as login; re-suffix it to '<slug>[bot]' so an identity comparison …
// sees the same REST rendering it does elsewhere" — #747). That fix re-suffixes a
// GraphQL-sourced bare login before comparing it against a REST-shaped expectation;
// deskroster's liveness probe had the mirror-image gap on the WRITE/request side: it sent a
// bare, roster-sourced slug to a REST endpoint that needs the "[bot]" suffix to resolve at
// all. Human and Bless identities are untouched — the bug is bot-only, matching the issue's
// "only the 13 bot entries were flagged" observation.
func probeLogin(id RosterIdentity) string {
	if id.Source == "bot" && !id.isGitLab() && !strings.HasSuffix(id.Login, "[bot]") {
		return id.Login + "[bot]"
	}
	return id.Login
}

// forgeDisplayName returns the human-readable forge name for a NOTICE/Detail string. The
// zero value (unset Forge, meaning GitHub — see RosterIdentity.Forge's doc comment) and
// ForgeGitHub both read "GitHub"; ForgeGitLab reads "GitLab"; anything else falls back to
// the raw ForgeKind string rather than guessing (pr1669-F2/pr1669-sec-F3: four strings here
// used to hard-code "GitHub" on what is now a forge-agnostic path).
func forgeDisplayName(k ForgeKind) string {
	switch k {
	case ForgeGitLab:
		return "GitLab"
	case ForgeGitHub, "":
		return "GitHub"
	default:
		return string(k)
	}
}

// classifyAccountState checks acct.State BEFORE classifyLiveness's unpinned/reclaimed/
// renamed branches, so a blocked/suspended account is reported as such even when the
// identity carries no pinned id to compare against (pr1669-F3: on the pre-fix code the
// PinnedID==0 branch ran first and swallowed the state entirely for an unpinned identity).
//
// GitLab's users API always reports a `state` field, so an EMPTY state on a GitLab response
// is a partial/malformed read, never confirmation the account is active (pr1669-sec-F1) —
// that classifies could-not-check. GitHub's account read carries no state field at all
// (forge.go's Account.State doc: never defaulted to active, but also never SET for GitHub),
// so an empty state on a GitHub identity is the ordinary, unremarkable case and this leaves
// it to the existing branches below (matched=false) — the sec-F1 fix is GitLab-scoped only.
func classifyAccountState(id RosterIdentity, probe string, acct *Account) (class LivenessClass, detail string, matched bool) {
	state := strings.ToLower(strings.TrimSpace(acct.State))
	switch {
	case id.isGitLab() && state == "":
		return LivenessCouldNotCheck, fmt.Sprintf(
				"login %q resolved on GitLab with no state field in the response — GitLab's users "+
					"API always reports one, so a missing state is treated as a partial/malformed "+
					"read, never as confirmation the account is active", probe),
			true
	case id.isGitLab() && state != "active":
		return LivenessSuspended, fmt.Sprintf(
				"login %q resolves to id %d, but its current forge-reported state is %q, not active — "+
					"never reported as alive", probe, acct.ID, state),
			true
	case !id.isGitLab() && state != "" && state != "active":
		return LivenessSuspended, fmt.Sprintf(
				"login %q resolves to id %d, but its current forge-reported state is %q, not active — "+
					"never reported as alive", probe, acct.ID, state),
			true
	}
	return "", "", false
}

// classifyLiveness runs the classification decision table for one identity. See the
// LivenessClass constants above for what each outcome means. It probes the identity's forge
// at probeLogin(id) (bot identities get the "[bot]" suffix on GitHub — see probeLogin) but
// keeps Identity.Login as the bare, roster-configured login throughout, so a caller/renderer
// always sees the identity in the shape the roster itself uses.
func classifyLiveness(fetcher AccountFetcher, id RosterIdentity) LivenessFinding {
	probe := probeLogin(id)
	acct, err := fetcher.GetAccount(probe)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			detail := fmt.Sprintf(
				"login %q no longer resolves to any %s account — it was pinned to id %d",
				probe, forgeDisplayName(id.Forge), id.PinnedID)
			if id.isGitLab() {
				// pr1669-F1: GitLab hides blocked/banned/ldap_blocked accounts from a
				// non-admin token's user search entirely (upstream GitLab's
				// UsersFinder#base_scope / FORBIDDEN_SEARCH_STATES) — a desk forge
				// credential (project/group/service-account token) IS a non-admin caller,
				// so an empty result here does NOT unambiguously mean "deleted".
				detail = fmt.Sprintf(
					"login %q returned no matching GitLab account — it was pinned to id %d. "+
						"GitLab hides blocked, banned, and ldap_blocked accounts from a "+
						"non-admin token's user search entirely, so this means the account "+
						"was deleted, OR that it is blocked/banned/ldap_blocked and simply "+
						"hidden from this token — not distinguishable from this read alone",
					probe, id.PinnedID)
			}
			return LivenessFinding{
				Identity: id,
				Class:    LivenessDeleted,
				Detail:   detail,
			}
		}
		return LivenessFinding{
			Identity: id,
			Class:    LivenessCouldNotCheck,
			Detail:   fmt.Sprintf("could not verify login %q against %s: %v", probe, forgeDisplayName(id.Forge), err),
		}
	}

	if class, detail, matched := classifyAccountState(id, probe, acct); matched {
		return LivenessFinding{Identity: id, Class: class, Detail: detail}
	}

	if id.PinnedID == 0 {
		return LivenessFinding{
			Identity: id,
			Class:    LivenessUnpinned,
			Detail: fmt.Sprintf(
				"login %q currently resolves to id %d, but the roster pins no id for it — "+
					"identity continuity cannot be checked", probe, acct.ID),
		}
	}

	if acct.ID != id.PinnedID {
		return LivenessFinding{
			Identity: id,
			Class:    LivenessReclaimed,
			Detail: fmt.Sprintf(
				"login %q now resolves to id %d, not the pinned id %d — a DIFFERENT account now answers to this login",
				probe, acct.ID, id.PinnedID),
		}
	}

	if !strings.EqualFold(acct.Login, probe) {
		return LivenessFinding{
			Identity: id,
			Class:    LivenessRenamed,
			Detail: fmt.Sprintf(
				"id %d (configured as login %q) now has canonical login %q",
				id.PinnedID, probe, acct.Login),
		}
	}

	return LivenessFinding{Identity: id, Class: LivenessAlive}
}

// RenderLivenessNotices renders one "NOTICE: ..." line per finding whose class is NOT
// LivenessAlive — an alive identity produces no output, the same quiet-on-the-happy-path
// shape every other NOTICE in this codebase uses. Each class gets a message shape that
// reads differently from the others, so LivenessReclaimed/LivenessDeleted (the two classes
// the liveness check actually worries about) are visually distinct from the advisory
// LivenessRenamed/LivenessUnpinned, which read differently again from LivenessCouldNotCheck.
func RenderLivenessNotices(findings []LivenessFinding) []string {
	var lines []string
	for _, f := range findings {
		switch f.Class {
		case LivenessAlive:
			continue
		case LivenessDeleted:
			lines = append(lines, fmt.Sprintf(
				"NOTICE: DELETED — trusted %s login %q no longer exists on %s. %s",
				f.Identity.Source, f.Identity.Login, forgeDisplayName(f.Identity.Forge), f.Detail))
		case LivenessReclaimed:
			lines = append(lines, fmt.Sprintf(
				"NOTICE: RECLAIMED — trusted %s login %q now points at a DIFFERENT %s account. %s",
				f.Identity.Source, f.Identity.Login, forgeDisplayName(f.Identity.Forge), f.Detail))
		case LivenessRenamed:
			lines = append(lines, fmt.Sprintf(
				"NOTICE: renamed — trusted %s login %q: %s",
				f.Identity.Source, f.Identity.Login, f.Detail))
		case LivenessUnpinned:
			lines = append(lines, fmt.Sprintf(
				"NOTICE: unpinned — trusted %s login %q: %s",
				f.Identity.Source, f.Identity.Login, f.Detail))
		case LivenessSuspended:
			lines = append(lines, fmt.Sprintf(
				"NOTICE: SUSPENDED — trusted %s login %q is not active on its forge. %s",
				f.Identity.Source, f.Identity.Login, f.Detail))
		case LivenessCouldNotCheck:
			lines = append(lines, fmt.Sprintf(
				"NOTICE: could-not-check — trusted %s login %q: %s",
				f.Identity.Source, f.Identity.Login, f.Detail))
		default:
			lines = append(lines, fmt.Sprintf(
				"NOTICE: unrecognised liveness class %q for trusted %s login %q: %s",
				f.Class, f.Identity.Source, f.Identity.Login, f.Detail))
		}
	}
	return lines
}
