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
type RepoInfoFetcher interface {
	// RepoVisibility returns the .visibility field from GET /repos/{owner}/{repo}.
	RepoVisibility(owner, repo string) (string, error)
	// IssueReactions returns reactions on an issue/PR.
	IssueReactions(owner, repo string, issueNumber int) ([]Reaction, error)
}

// Reaction is one reaction on a GitHub issue or PR comment.
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

// PublicRepoGate enforces the public-repo trust gate.
//
// It is called before ANY write-capable desk tool acts on a repo. The gate:
//  1. Reads the LIVE visibility (fetcher.RepoVisibility, never the compiled-in census)
//     — if it fails, exit 6 (fail closed).
//  2. Allowlist, not a denylist: ONLY "private" (case-insensitively, trimmed) returns
//     nil (gate does not apply). "public" and "internal" both require the +1 below;
//     anything else — empty, unrecognised, a future value — exit 6 (fail closed).
//  3. If the repo is public/internal AND issueNumber <= 0, exit 6 (no reactions surface
//     — commands without an issue/PR number cannot act on such repos).
//  4. If the repo is public/internal AND issueNumber > 0, fetches reactions and requires
//     a +1 from the CONFIGURED blessing authority — login AND permanent numeric id, via
//     IsBlessAuthorityIDStrict, which (unlike IsBlessAuthorityID) refuses a missing id.
//  5. If the reactions lookup fails, exit 6.
//  6. If no qualifying reaction is found, exit 5 (refused by constraint).
func PublicRepoGate(fetcher RepoInfoFetcher, owner, repo string, issueNumber int) error {
	visibility, err := fetcher.RepoVisibility(owner, repo)
	if err != nil {
		return Unverifiable(fmt.Sprintf("public-repo gate: cannot determine repo visibility for %s/%s — refusing rather than guessing", owner, repo), err)
	}

	// Allowlist, not a denylist: ONLY "private" (case-insensitively, trimmed) skips the
	// gate. Everything else — "public", "internal", a re-cased or padded read, or a value
	// this code has never seen — either requires the +1 below or fails closed outright.
	// A denylist here ("gate only when == public") is a bypass by spelling: security
	// review on #310 drove the live gate over every visibility string GitHub's API can
	// return and found "PUBLIC", "public " (whitespace), and "internal" all fell through
	// ungated under `visibility != "public"`. `internal` is org-visible, not private — the
	// gate's stated premise is untrusted eyes, so it is gated exactly like `public` here,
	// not treated as a pass. This mirrors config.go's ParseVisibility, which fails closed
	// (VisibilityUnknown, risk-classed) on the identical set of unrecognised inputs; the
	// two visibility readers in this package must agree, not diverge.
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case "private":
		return nil
	case "public", "internal":
		// fall through to the +1 requirement below
	default:
		return Unverifiable(fmt.Sprintf("public-repo gate: repo %s/%s returned unrecognised visibility %q — refusing rather than treating it as private", owner, repo, visibility), nil)
	}

	// Public repo: must have an issue/PR number to consult the reactions surface.
	if issueNumber <= 0 {
		return Unverifiable("public-repo gate: no issue/PR number — a repo-level action on a public repo has no reactions surface to check", nil)
	}

	reactions, err := fetcher.IssueReactions(owner, repo, issueNumber)
	if err != nil {
		return Unverifiable(fmt.Sprintf("public-repo gate: cannot fetch reactions for %s/%s#%d", owner, repo, issueNumber), err)
	}

	for _, r := range reactions {
		// IsBlessAuthorityIDStrict, not IsBlessAuthorityID: login and id come from the
		// SAME reactions object, so a zero id is a failed read, never "this surface has
		// no ids". Admitting it would degrade the pin back to a login-only check
		// and let a recycled blessing-authority login satisfy the gate.
		if r.Content == "+1" && r.User.Type == "User" && IsBlessAuthorityIDStrict(r.User.Login, r.User.ID) {
			return nil // gate satisfied
		}
	}

	return Refused(fmt.Sprintf(
		"public-repo gate: %s/%s is public and issue #%d carries no qualifying +1 from an authorized human (%s)",
		owner, repo, issueNumber, humanList()))
}

// humanList returns the authorized human login(s) as a readable string, for the
// refusal message only. With the roster unconfigured this is "(unconfigured)" rather
// than an empty string, so the refusal never reads as if a blank identity would
// satisfy the gate.
func humanList() string {
	if login := BlessAuthorityLogin(); login != "" {
		return login
	}
	return "(unconfigured — see " + EnvBlessLogin + ")"
}

// --- Default HTTP implementation ---

// HTTPRepoInfoFetcher implements RepoInfoFetcher with direct REST calls to the GitHub
// API using a bearer token. It is the production implementation used by desk commands.
type HTTPRepoInfoFetcher struct {
	Token   string
	BaseURL string // defaults to "https://api.github.com" when empty
	Client  *http.Client
}

func (f *HTTPRepoInfoFetcher) baseURL() string {
	if f.BaseURL != "" {
		return f.BaseURL
	}
	return "https://api.github.com"
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

// IssueReactions calls GET /repos/{owner}/{repo}/issues/{number}/reactions and returns
// the reaction list.
//
// SINGLE PAGE, by decision: per_page=100 with no Link-header follow.
// A genuine ada +1 past the hundredth reaction on the SAME issue is therefore
// invisible to the gate and reads as a mystery refusal. The direction is safe (fail
// closed, never a false pass) and the shape is bounded — 100 reactions on one issue is
// far outside anything this desk has seen — so pagination is deliberately not
// implemented. If it ever bites, the symptom is a refusal that a visible ada +1
// does not clear; the fix is a Link-header walk, not a per_page bump.
func (f *HTTPRepoInfoFetcher) IssueReactions(owner, repo string, issueNumber int) ([]Reaction, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/reactions?per_page=100", f.baseURL(), owner, repo, issueNumber)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+f.Token)
	req.Header.Set("Accept", "application/vnd.github.squirrel-girl-preview+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var reactions []Reaction
	if err := json.Unmarshal(body, &reactions); err != nil {
		return nil, fmt.Errorf("cannot parse reactions response: %w", err)
	}
	return reactions, nil
}

// stubRepoInfoFetcher is a test-only implementation of RepoInfoFetcher.
type stubRepoInfoFetcher struct {
	visibility    string
	visibilityErr error
	reactions     []Reaction
	reactionsErr  error
}

func (s *stubRepoInfoFetcher) RepoVisibility(owner, repo string) (string, error) {
	return s.visibility, s.visibilityErr
}

func (s *stubRepoInfoFetcher) IssueReactions(owner, repo string, issueNumber int) ([]Reaction, error) {
	return s.reactions, s.reactionsErr
}

// Ensure the stub type is referenced (package-level check).
var _ RepoInfoFetcher = (*stubRepoInfoFetcher)(nil)
