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
// :public authorization (PublicRepoGate below). They remain because the Forge
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

// PublicRepoGate enforces the public-repo write gate.
//
// It is called before ANY write-capable desk tool acts on a repo. The authorization
// unit is the REPOSITORY, not the item: a public/internal repo listed in the
// allowed-repos configuration with the `:public` visibility token is a place the desk
// may write, decided once by a human out-of-band, and every write verb on it passes.
// This replaces the former per-item `+1` reaction check, which was unsatisfiable for the
// write that matters most — opening the first pull request, which has no issue/PR number
// yet — and expressed the human decision in the wrong unit. A draft PR is inert until a
// human merges it, so the repository-level decision plus the merge gate buy everything
// the per-item ceremony did.
//
// The gate:
//  1. Reads the LIVE visibility (fetcher.RepoVisibility, never the configured census) —
//     if it fails, exit 6 (fail closed). The live read is load-bearing: a repo flipped
//     to public AFTER the set was written is still gated, and a stale roster claiming
//     `:public` for a repo the forge reports otherwise never authorizes on the stale
//     claim (row 5 of the brief's Verify table).
//  2. Allowlist, not a denylist: ONLY "private" (case-insensitively, trimmed) returns
//     nil (gate does not apply). "public" and "internal" require an explicit `:public`
//     allowed-repos entry (step 3); anything else — empty, unrecognised, a future value
//     — exit 6 (fail closed).
//  3. For "public"/"internal", the repo MUST carry an EXPLICIT allowed-repos entry whose
//     CONFIGURED visibility is public (RepoVisibility(owner/repo) == VisibilityPublic).
//     If it does, return nil. If it does not — absent, matched only by an `owner/*`
//     pattern (patterns carry no policy), tagged `:private`, or carrying no visibility
//     token — return Refused (exit 5) naming the repo, what was read live, what the set
//     says, and the exact remedy. This is why BOTH reads are load-bearing at once: the
//     live read AND the configured claim must AGREE.
//
// `internal` is org-visible, not private — the gate's premise is untrusted eyes, so it is
// gated exactly like `public`, not treated as a pass. This mirrors config.go's
// ParseVisibility, which fails closed on the identical set of unrecognised inputs (#310);
// the two visibility readers in this package must agree, not diverge.
func PublicRepoGate(fetcher RepoInfoFetcher, owner, repo string) error {
	visibility, err := fetcher.RepoVisibility(owner, repo)
	if err != nil {
		return Unverifiable(fmt.Sprintf("public-repo gate: cannot determine repo visibility for %s/%s — refusing rather than guessing", owner, repo), err)
	}

	// Allowlist, not a denylist: ONLY "private" (case-insensitively, trimmed) skips the
	// gate. Everything else — "public", "internal", a re-cased or padded read, or a value
	// this code has never seen — either requires the `:public` allowed-repos entry below or
	// fails closed outright. A denylist here ("gate only when == public") is a bypass by
	// spelling: security review on #310 drove the live gate over every visibility string
	// GitHub's API can return and found "PUBLIC", "public " (whitespace), and "internal"
	// all fell through ungated under `visibility != "public"`.
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case "private":
		return nil
	case "public", "internal":
		// The single control standing between a desk tool and an outward write to a
		// public/internal repo: an EXPLICIT allowed-repos entry tagged `:public`. The
		// configured value is read here (RepoVisibility, no network) AND compared against
		// the LIVE read above — a repo absent from the set, matched only by an `owner/*`
		// pattern (patterns widen IsAllowedRepo alone; they carry no visibility policy),
		// tagged `:private`, or carrying no visibility token all answer something other
		// than VisibilityPublic and refuse. The set is configured out-of-band, so no pull
		// request can add its own repository to it.
		if RepoVisibility(owner+"/"+repo) == VisibilityPublic {
			return nil
		}
		return Refused(fmt.Sprintf(
			"public-repo gate: %s/%s reads live-%s but the allowed-repos set does not authorize outward writes to it "+
				"(configured visibility: %s, not :public). Add %s/%s:public to the allowed-repos configuration (%s) — "+
				"the authorization is repository-scoped and covers every write verb; merge remains the human's.",
			owner, repo, strings.ToLower(strings.TrimSpace(visibility)),
			RepoVisibility(owner+"/"+repo).String(), owner, repo, EnvAllowedRepos))
	default:
		return Unverifiable(fmt.Sprintf("public-repo gate: repo %s/%s returned unrecognised visibility %q — refusing rather than treating it as private", owner, repo, visibility), nil)
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

	resp, err := f.client().Do(req)
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
