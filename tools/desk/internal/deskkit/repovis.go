package deskkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RepoInfoFetcher is the minimal GitHub API surface the public-repo gate needs.
// Commands provide the real (token-authenticated) implementation; tests provide a stub.
//
// It is deliberately just the live-visibility read: the gate no longer consults a
// per-item reaction surface (see PublicRepoGate), so an issue/PR number and a
// reactions probe are no longer part of this interface.
type RepoInfoFetcher interface {
	// RepoVisibility returns the .visibility field from GET /repos/{owner}/{repo}.
	RepoVisibility(owner, repo string) (string, error)
}

// Reaction is one reaction (GitHub) or award emoji (GitLab) on an issue or PR.
//
// These types are NOT used by PublicRepoGate any more — the public-repo write gate
// stopped reading reactions when the per-item +1 was replaced by the repository-scoped
// :public / :internal authorization (PublicRepoGate below). They remain because the Forge
// abstraction's reaction/award admission surface (forge.go, with the GitHub and GitLab
// backends) consumes them; that surface is a separate control and out of scope here.
type Reaction struct {
	User    ReactionUser `json:"user"`
	Content string       `json:"content"`
}

// ReactionUser is the actor who added a reaction.
type ReactionUser struct {
	Login string `json:"login"`
	Type  string `json:"type"` // "User" or "Bot"
	ID    int64  `json:"id"`   // numeric user id — login-recycling defence (trust.go IsBlessAuthorityID)
}

// FetchRepoVisibility calls GET /repos/{owner}/{repo} and returns the LIVE .visibility
// field. It requires a RepoInfoFetcher; use HTTPRepoInfoFetcher for production.
//
// NOT to be confused with config.go's RepoVisibility(repo) Visibility, which returns the
// COMPILED-IN census value and makes no network call. The two are deliberately separate
// (the public-repo risk rule) and must stay so:
//
//   - risk-classing reads the compiled-in value, so the gate cannot fail OPEN when
//     GitHub is unreachable;
//   - the write gate here reads the LIVE value, so a repo flipped to public after the
//     census was written is still gated. It fails CLOSED on any read error, so using
//     the network here cannot open a hole either.
//
// Never route PublicRepoGate through the compiled-in map: an org-default repo absent
// from the census answers VisibilityUnknown, which would skip the gate's public branch.
func FetchRepoVisibility(fetcher RepoInfoFetcher, owner, repo string) (string, error) {
	v, err := fetcher.RepoVisibility(owner, repo)
	if err != nil {
		return "", fmt.Errorf("cannot determine repo visibility for %s/%s: %w", owner, repo, err)
	}
	return v, nil
}

// PublicRepoGateVerbs names the desk commands that call PublicRepoGate before an outward
// write, in the order the gate's refusal lists them: deskpr (create, update, edit),
// deskpost (comment, ready, review), deskreply, deskevidence and deskrelease (cut). The
// refusal names exactly these, so it never claims to cover a write the gate does not see.
// Tracker writes are not among them: deskfile, for one, never calls the gate
// (publicRepoGateUngated). TestPublicRepoGateVerbsMatchCallers in cmd/deskevidence walks
// the command tree and fails when a command starts or stops calling the gate without this
// list following it (#5436).
var PublicRepoGateVerbs = []string{"deskpr", "deskpost", "deskreply", "deskevidence", "deskrelease"}

// publicRepoGateUngated is the issue-tracker command the refusal names as one the gate
// does not cover. It must never appear in PublicRepoGateVerbs (TestGateRefusalNamesGatedVerbs).
const publicRepoGateUngated = "deskfile"

// gateScopeClause renders the refusal's scope sentence from PublicRepoGateVerbs.
func gateScopeClause() string {
	return "the authorization is repository-scoped and covers the desk commands that run this gate (" +
		strings.Join(PublicRepoGateVerbs, ", ") + "); tracker writes such as " + publicRepoGateUngated +
		" do not run it; merge remains the human's."
}

// PublicRepoGate enforces the public-repo write gate.
//
// It is called before the outward write of each command in PublicRepoGateVerbs — not
// before every write-capable desk tool: tracker writes such as deskfile do not call it.
// The authorization unit is the REPOSITORY, not the item: a repo listed in the allowed-repos
// configuration with a visibility token at least as exposed as its live visibility — a
// public repo listed `:public`, an internal repo listed `:internal` — is a place the desk may
// write, decided once by a human out-of-band, and every gated write on it passes. (The gate
// also tolerates an internal repo mislabelled `:public`; see step 3.)
// This replaces the former per-item `+1` reaction check, which was unsatisfiable for the
// write that matters most — opening the first pull request, which has no issue/PR number
// yet — and expressed the human decision in the wrong unit. A draft PR is inert until a
// human merges it, so the repository-level decision plus the merge gate buy everything
// the per-item ceremony did.
//
// The gate:
//  1. Reads the LIVE visibility (fetcher.RepoVisibility, never the configured census) —
//     if it fails, exit 6 (fail closed). The live read is load-bearing: a repo flipped
//     to public AFTER the set was written is still gated, and a stale roster claim never
//     authorizes a repo the forge reports as MORE exposed than the claim (row 5 of the
//     brief's Verify table).
//  2. Allowlist, not a denylist: ONLY "private" (case-insensitively, trimmed) returns
//     nil (gate does not apply). "public" and "internal" require an explicit allowed-repos
//     entry (step 3); anything else — empty, unrecognised, a future value — exit 6 (fail
//     closed).
//  3. For "public"/"internal", the repo MUST carry an EXPLICIT allowed-repos entry whose
//     CONFIGURED visibility is non-private and ranks at least as exposed as the live read,
//     on the exposure rank private < internal < public (visibilityExposure, the same rank
//     the verifier cross-repo check uses). So live public needs VisibilityPublic, and live
//     internal needs VisibilityInternal. The gate TOLERATES VisibilityPublic on a live-
//     internal repo, because the write reaches no reader the entry did not admit, but that
//     entry is a mislabel, not an equal choice: `deskboard policydrift` reports it as drift,
//     and it over-ranks the repo wherever the configured visibility is ranked alone (the
//     verifier cross-repo source check, where a public tracking repo could then publish its
//     binding). So no refusal ever names `:public` as a live-internal remedy, and operators
//     change such an entry to `:internal`. Nothing passes that is more exposed than its
//     entry states. If the
//     entry does not authorize — absent, matched only by an `owner/*` pattern (patterns
//     carry no policy), tagged `:private`, carrying no visibility token, or tagged
//     `:internal` on a live-public repo — return Refused (exit 5) naming the repo, what was
//     read live, what the set says and why that does not authorize, and the exact remedy.
//     Both reads stay load-bearing: the live read bounds what the configured claim must
//     cover.
//
// `internal` is instance-visible, not private — the gate's premise is untrusted eyes, so
// it is gated like `public`, not treated as a pass, and VisibilityInternal is not private
// anywhere else either (risk-classing, the self-containment scan, the public-form trailer).
// This mirrors config.go's ParseVisibility, which fails closed on the identical set of
// unrecognised inputs (#310); the two visibility readers in this package must agree, not
// diverge.
func PublicRepoGate(fetcher RepoInfoFetcher, owner, repo string) error {
	visibility, err := fetcher.RepoVisibility(owner, repo)
	if err != nil {
		return Unverifiable(fmt.Sprintf("public-repo gate: cannot determine repo visibility for %s/%s — refusing rather than guessing", owner, repo), err)
	}

	// Allowlist, not a denylist: ONLY "private" (case-insensitively, trimmed) skips the
	// gate. Everything else — "public", "internal", a re-cased or padded read, or a value
	// this code has never seen — either requires an authorizing allowed-repos entry below
	// or fails closed outright. A denylist here ("gate only when == public") is a bypass by
	// spelling: security review on #310 drove the live gate over every visibility string
	// GitHub's API can return and found "PUBLIC", "public " (whitespace), and "internal"
	// all fell through ungated under `visibility != "public"`.
	live := strings.ToLower(strings.TrimSpace(visibility))
	switch live {
	case "private":
		return nil
	case "public", "internal":
		// The single control standing between a desk tool and an outward write to a
		// public/internal repo: an EXPLICIT allowed-repos entry whose visibility token is
		// not private and ranks at least as exposed as the live value. The configured value
		// is read here (RepoVisibility, no network) AND ranked against the LIVE read above.
		// A repo absent from the set, matched only by an `owner/*` pattern (patterns widen
		// IsAllowedRepo alone; they carry no visibility policy), tagged `:private`, carrying
		// no visibility token, or tagged `:internal` on a live-public repo ranks below the
		// live value or is unranked, and refuses. The set is configured out-of-band, so no
		// pull request can add its own repository to it.
		slug := owner + "/" + repo
		configured := RepoVisibility(slug)
		liveRank, entryRank := visibilityExposure(ParseVisibility(live)), visibilityExposure(configured)
		if configured != VisibilityPrivate && liveRank > 0 && entryRank >= liveRank {
			return nil
		}
		// The remedy names the live value only. The gate tolerates a `:public` entry on a
		// live-internal repo, but that is a mislabel: it over-ranks the repo elsewhere
		// (the verifier cross-repo source check ranks configured visibility alone), so the
		// refusal never recommends it.
		remedy := slug + ":" + live
		return Refused(fmt.Sprintf(
			"public-repo gate: %s reads live-%s but the allowed-repos set does not authorize outward writes to it "+
				"(%s). Add %s to the allowed-repos configuration (%s) — %s",
			slug, live, gateEntryCause(slug, configured, live), remedy, EnvAllowedRepos, gateScopeClause()))
	default:
		return Unverifiable(fmt.Sprintf("public-repo gate: repo %s/%s returned unrecognised visibility %q — refusing rather than treating it as private", owner, repo, visibility), nil)
	}
}

// gateEntryCause names why the configured entry for slug does not authorize a write at the
// live visibility: no explicit entry (absent, or matched only by an owner/* pattern), an
// entry with no visibility token, a `:private` entry, or a token less exposed than the live
// read. It only words the refusal; the decision is PublicRepoGate's rank comparison.
func gateEntryCause(slug string, configured Visibility, live string) string {
	if _, explicit := allowedRepos()[slug]; !explicit {
		if IsAllowedRepo(slug) {
			return "matched only by an owner/* pattern, which carries no visibility policy"
		}
		return "no allowed-repos entry names it"
	}
	switch configured {
	case VisibilityPrivate:
		return "configured visibility: private; a :private entry authorizes no outward write to a non-private repo"
	case VisibilityUnknown:
		return "configured visibility: unknown; the entry carries no visibility token"
	default:
		return fmt.Sprintf("configured visibility: %s, less exposed than live-%s", configured.String(), live)
	}
}

// --- Default HTTP implementation ---

// HTTPRepoInfoFetcher implements RepoInfoFetcher with direct REST calls to the GitHub
// API using a bearer token. It is the production implementation used by desk commands.
type HTTPRepoInfoFetcher struct {
	Token   string
	BaseURL string // defaults to GitHubAPIBase when empty
	Client  *http.Client
}

func (f *HTTPRepoInfoFetcher) baseURL() string {
	if f.BaseURL != "" {
		return f.BaseURL
	}
	return GitHubAPIBase
}

func (f *HTTPRepoInfoFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

// RepoVisibility calls GET /repos/{owner}/{repo} and returns the .visibility field.
func (f *HTTPRepoInfoFetcher) RepoVisibility(owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", f.baseURL(), owner, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+f.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := countingClient(f.client()).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var repoInfo struct {
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(body, &repoInfo); err != nil {
		return "", fmt.Errorf("cannot parse repo response: %w", err)
	}
	if repoInfo.Visibility == "" {
		return "", fmt.Errorf("repo %s/%s has no .visibility field in API response (unrecognised)", owner, repo)
	}
	return repoInfo.Visibility, nil
}

// ForgeRepoInfoFetcher adapts an ALREADY-RESOLVED Forge backend — whichever one
// deskkit.ForgeFor picked for the repo (GitHubForge, GitLabForge, or a test fake) — to the
// (owner, repo string) RepoInfoFetcher signature PublicRepoGate consumes.
//
// It exists because a caller that already resolved the forge for its OTHER operations
// (OpenChangeForBranch, CreateDraftChange, GetPullRequest, …) must ask the public-repo gate's
// visibility read through the SAME backend, not a second, independently-constructed client.
// Before this type existed, cmd/deskpr built `&HTTPRepoInfoFetcher{Token: ghToken}` for the
// gate regardless of which forge the resolver had actually picked, so a GitLab-resolved repo's
// visibility read went out over GitHub's REST API — which cannot answer for a project that
// does not live there, and the gate failed closed (could-not-check) reporting a GitHub error
// for a repo GitHub has never heard of (assay#1054). The GitLab backend's OWN RepoVisibility
// (forge_gitlab.go) was already correct and already reachable through Forge — nothing
// downstream of the resolver was broken; the gate simply never asked it.
//
// This is a THIN adapter, not a second implementation: it delegates to fg.RepoVisibility
// (part of the frozen Forge interface, backed by the golden-pinned per-forge reads), so it
// adds no fall-open path of its own — a read error propagates unchanged, and the gate still
// fails closed on an unreadable visibility surface, exactly as it did before.
type ForgeRepoInfoFetcher struct {
	Forge Forge
}

// RepoVisibility delegates to the resolved backend's ForgeRepo-signature read.
func (f ForgeRepoInfoFetcher) RepoVisibility(owner, repo string) (string, error) {
	return f.Forge.RepoVisibility(ForgeRepo{Owner: owner, Name: repo})
}

// ForgeRepoInfoFetcher satisfies the public-repo gate's fetcher contract.
var _ RepoInfoFetcher = ForgeRepoInfoFetcher{}

// stubRepoInfoFetcher is a test-only implementation of RepoInfoFetcher.
type stubRepoInfoFetcher struct {
	visibility    string
	visibilityErr error
}

func (s *stubRepoInfoFetcher) RepoVisibility(owner, repo string) (string, error) {
	return s.visibility, s.visibilityErr
}

// Ensure the stub type is referenced (package-level check).
var _ RepoInfoFetcher = (*stubRepoInfoFetcher)(nil)
