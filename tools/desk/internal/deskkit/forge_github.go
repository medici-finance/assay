package deskkit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
)

// forge_github.go — the GitHub implementation of Forge, seated on the official go-gh
// library (github.com/cli/go-gh/v2, its pkg/api REST client + pkg/auth resolution) rather
// than a hand-rolled net/http stack or a shelled `gh` binary. It is an EXTRACTION of the
// behavior the desk tools already run against GitHub, pinned by the golden corpus in
// forge_github_golden_test.go so re-seating the transport changed nothing observable at the
// wire: request method/path/query, pagination, and error mapping are captured per operation
// and stay byte-identical.
//
// Why go-gh, and what it buys:
//   - The GitHub backend now talks the forge through the same class of library-backed client
//     the GitLab backend uses, so both are API-behind-one-interface, and the `gh` CLI need no
//     longer be installed/version-matched on a runner for these operations.
//   - Auth binds to the EXPLICITLY minted desk token. The client is constructed with both a
//     Host and an AuthToken set AND an explicit Transport, which makes go-gh's
//     optionsNeedResolution false — so it never consults gh's ambient keyring/config for a
//     token or host. An empty token is REFUSED (restClient below), never silently resolved
//     to an ambient gh-CLI identity. This preserves the refuse-if-unminted posture the desk
//     tools depend on (mirrors #562/#563) at the transport floor.
//
// It is handed an already-minted token (App installation token or PAT) — minting is the
// identity layer (spec §2/§5) and deliberately not part of this seam.

// GitHubForge implements Forge against the GitHub REST/GraphQL API with a bearer token.
// Same shape as HTTPRepoInfoFetcher (repovis.go): BaseURL defaults to GitHubAPIBase, Client
// defaults to go-gh's own transport, so a test points BaseURL at an httptest server (and may
// supply a Client whose Transport reaches it).
type GitHubForge struct {
	Token   string
	BaseURL string
	Client  *http.Client

	// rc caches the go-gh REST client built from Token/BaseURL. Lazily constructed by
	// restClient so a bare struct literal (the golden test's construction shape) still works.
	rc *ghapi.RESTClient
}

var _ Forge = (*GitHubForge)(nil)

func (g *GitHubForge) baseURL() string {
	return GitHubBaseURLOrDefault(g.BaseURL)
}

// restClient returns the go-gh REST client for this forge, building it on first use.
//
// The token is bound EXPLICITLY: an empty Token is refused here rather than allowed to fall
// through to go-gh's ambient resolution (which would read gh's keyring/config). Setting Host,
// AuthToken and Transport all non-empty makes go-gh's optionsNeedResolution false, so no
// ambient lookup ever runs. The host is derived from BaseURL so go-gh's token roundtripper —
// which attaches the Authorization header only when the request host matches Host — sends the
// token to the real API host and to a test server, but never to an unrelated host.
func (g *GitHubForge) restClient() (*ghapi.RESTClient, error) {
	if g.Token == "" {
		return nil, Unverifiable("refusing to reach the GitHub forge without an explicitly minted token — "+
			"the go-gh backend never falls back to an ambient gh-CLI keyring/config identity", nil)
	}
	if g.rc != nil {
		return g.rc, nil
	}
	host := "github.com"
	if u, perr := url.Parse(g.baseURL()); perr == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	// Transport must be non-nil so go-gh does not treat the options as needing ambient
	// resolution. In production that is the default transport; a test may pass a Client whose
	// Transport reaches its httptest server.
	transport := http.DefaultTransport
	if g.Client != nil && g.Client.Transport != nil {
		transport = g.Client.Transport
	}
	rc, err := ghapi.NewRESTClient(ghapi.ClientOptions{
		Host:      host,
		AuthToken: g.Token,
		Transport: transport,
	})
	if err != nil {
		return nil, Unverifiable("cannot build go-gh REST client", err)
	}
	g.rc = rc
	return rc, nil
}

// ForgeAPIError is a non-2xx REST/GraphQL response. A caller maps it to Unverifiable (an
// API error mid-check means the precondition could not be positively verified). A 404 is
// distinguished via IsForgeNotFound — the only status that licenses a kind re-resolution.
//
// Body, when non-empty, is the forge's OWN structured error message for the failure —
// GitLab renders both its response shapes (`{"message": …}` and `{"error": …}`) into one
// readable string, and preserving it here is what turns a bare "HTTP 400" into an
// actionable refusal a caller can act on rather than guess at (issue #1415). It is
// control-stripped at the point it is captured (mapErr), like every other forge-origin
// string this tree renders, and it is OPTIONAL: a backend or status that carries no body
// leaves it empty and Error() falls back to the status-only form. Being a struct field, it
// is also available to programmatic classification via errors.As — the one narrow use is
// gitlabIsTransientMissingSourceBranch, which reads it to tell GitLab's post-push
// "source branch does not exist" race apart from every other 400.
type ForgeAPIError struct {
	Status int
	Method string
	Path   string
	Body   string
}

func (e *ForgeAPIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("forge API %s %s returned HTTP %d: %s", e.Method, e.Path, e.Status, e.Body)
	}
	return fmt.Sprintf("forge API %s %s returned HTTP %d", e.Method, e.Path, e.Status)
}

// IsForgeNotFound reports whether err is a 404 from the forge REST layer. It unwraps, so a
// ForgeAPIError nested in a DeskError is still recognised.
func IsForgeNotFound(err error) bool {
	var ae *ForgeAPIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// IsForgeForbidden reports whether err is a 403 from the forge REST layer. It unwraps, so a
// ForgeAPIError nested in a DeskError is still recognised. A 403 is distinct from a 404: it
// means the token is authenticated but lacks the scope for THIS endpoint (e.g. the legacy
// branch-protection endpoint needs `administration`, which the reviewer/worker App tokens do
// not carry), which licenses an admin-free re-resolution rather than a fail-open empty.
func IsForgeForbidden(err error) bool {
	var ae *ForgeAPIError
	return errors.As(err, &ae) && ae.Status == http.StatusForbidden
}

// ErrForgeEmptyRepo is the canonical, backend-NEUTRAL signal that the forge has positively
// answered "this repository has no commits yet". It is a distinct KNOWN state (no-commits),
// NOT a read failure. Each backend translates ITS OWN empty signal into this sentinel inside
// ListRecentCommits — GitHub answers 409 Conflict ("Git Repository is empty.") on the commits
// endpoint, GitLab answers 404 on its commits list — so the shared IsForgeEmptyRepo predicate
// tests for exactly this sentinel rather than guessing from a raw HTTP status backend-blind.
// That distinction is load-bearing: a bare status is ambiguous across backends, and a GitHub
// 404 means the repo is gone/renamed or the token has lost access — a could-not-check the
// branch-health probe must SURFACE, never fold into "empty".
var ErrForgeEmptyRepo = errors.New("forge repository has no commits (empty repository)")

// IsForgeEmptyRepo reports whether err carries the canonical empty-repository sentinel
// (ErrForgeEmptyRepo), which a backend's ListRecentCommits raises only for the forge's own
// positive "no commits yet" answer. It is a distinct KNOWN state (no-commits), not a read
// failure, so a caller (deskboard's branch-health probe) tests for it explicitly rather than
// folding it into could-not-check. It unwraps, so a wrapped sentinel is still recognised.
func IsForgeEmptyRepo(err error) bool {
	return errors.Is(err, ErrForgeEmptyRepo)
}

// doJSON performs one REST call through the go-gh client, decoding a 2xx body into out (if
// non-nil and non-empty). A non-2xx is mapped to a *ForgeAPIError carrying the status, method
// and path — go-gh surfaces a non-2xx as its own *api.HTTPError, which is translated back to
// the forge's stable error shape so error CLASSIFICATION (401 vs 403 vs 404, and
// IsForgeNotFound) is unchanged from the pre-go-gh backend. A transport/marshal/parse failure
// is Unverifiable. The full URL is built here (baseURL()+path) and passed to go-gh's client,
// whose restURL passes an absolute URL through unchanged — so path, query and body are emitted
// exactly as constructed, which is what the golden corpus pins.
func (g *GitHubForge) doJSON(method, path string, in, out any) error {
	_, err := g.doJSONHeader(method, path, in, out)
	return err
}

// doJSONHeader is doJSON returning the response headers too, for the ONE caller that needs
// the forge's own pagination signal (ListOpenIssues reads `Link: rel="next"`).
//
// A degraded transport never yields a SHORTER answer with a nil error (#1032): the body
// read's error is kept — a connection cut mid-transfer hands back whatever arrived, which
// can still parse (`[]`), so discarding the error turns a truncated page into a complete,
// empty one — and a ZERO-BYTE body where JSON was expected is an error, not an empty
// result: the forge answers an empty collection as `[]`, never as nothing. cmd/issueboard
// reads an issue's absence from the open-issue listing as evidence about that issue, so a
// listing this seam hands back must be the whole listing or an error.
func (g *GitHubForge) doJSONHeader(method, path string, in, out any) (http.Header, error) {
	rc, err := g.restClient()
	if err != nil {
		return nil, err
	}
	var bodyReader io.Reader
	if in != nil {
		b, merr := json.Marshal(in)
		if merr != nil {
			return nil, Unverifiable("cannot marshal request body", merr)
		}
		bodyReader = bytes.NewReader(b)
	}
	resp, rerr := rc.Request(method, g.baseURL()+path, bodyReader)
	if rerr != nil {
		var he *ghapi.HTTPError
		if errors.As(rerr, &he) {
			return nil, &ForgeAPIError{Status: he.StatusCode, Method: method, Path: path}
		}
		return nil, Unverifiable(fmt.Sprintf("%s %s failed", method, path), rerr)
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, Unverifiable(fmt.Sprintf("%s %s response body cut off mid-transfer (%d bytes arrived) — refusing to treat a partial answer as a complete one", method, path, len(raw)), readErr)
	}
	if out != nil {
		if len(raw) == 0 {
			return nil, Unverifiable(fmt.Sprintf("%s %s returned HTTP %d with an empty body where a JSON answer was expected — an absent answer is not an empty one", method, path, resp.StatusCode), nil)
		}
		if uerr := json.Unmarshal(raw, out); uerr != nil {
			return nil, Unverifiable(fmt.Sprintf("cannot parse %s %s response", method, path), uerr)
		}
	}
	return resp.Header, nil
}

// hasNextLink reports whether a REST response's `Link` header advertises another page
// (`rel="next"`) — the forge's own, authoritative more-pages signal, the twin of the GitLab
// client's NextPage.
func hasNextLink(h http.Header) bool {
	for _, v := range h.Values("Link") {
		for _, part := range strings.Split(v, ",") {
			if strings.Contains(part, `rel="next"`) || strings.Contains(part, "rel=next") {
				return true
			}
		}
	}
	return false
}

// --- REST wire shapes (only the fields consumed) ---

type ghPullWire struct {
	Number       int    `json:"number"`
	State        string `json:"state"`
	Draft        bool   `json:"draft"`
	NodeID       string `json:"node_id"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	ChangedFiles int    `json:"changed_files"`
	User         struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	HTMLURL   string `json:"html_url"`
	UpdatedAt string `json:"updated_at"`
	MergedAt  string `json:"merged_at"`
	Merged    bool   `json:"merged"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	// Mergeable is GitHub's THREE-state answer rendered as a JSON tri-state: true, false,
	// or null while the background merge computation is still running. It is decoded as a
	// *bool precisely so null stays distinguishable from false — collapsing the two would
	// report "not yet computed" as "conflicting", which refuses flips that should proceed,
	// and the opposite collapse would report it as mergeable, which is the fail-open half.
	Mergeable *bool `json:"mergeable"`
}

// ghMergeableState maps GitHub's tri-state `mergeable` field onto the forge-neutral
// vocabulary PullRequest.Mergeable carries.
func ghMergeableState(m *bool) string {
	switch {
	case m == nil:
		return MergeableUnknown
	case *m:
		return Mergeable
	default:
		return MergeableConflicting
	}
}

type ghIssueWire struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Body   string `json:"body"`
	User   struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
	HTMLURL string `json:"html_url"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

type ghReviewWire struct {
	ID   int64 `json:"id"`
	User struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	State       string `json:"state"`
	CommitID    string `json:"commit_id"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
}

type ghFileWire struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
}

type ghCombinedStatusWire struct {
	State      string `json:"state"`
	TotalCount int    `json:"total_count"`
	Statuses   []struct {
		State     string `json:"state"`
		Context   string `json:"context"`
		CreatedAt string `json:"created_at"`
	} `json:"statuses"`
}

type ghCheckRunsWire struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Status      string `json:"status"`
		Conclusion  string `json:"conclusion"`
		StartedAt   string `json:"started_at"`
		CompletedAt string `json:"completed_at"`
	} `json:"check_runs"`
}

// ghLabelWire is one entry of the repo/issue label listings.
type ghLabelWire struct {
	Name string `json:"name"`
}

// ghRequiredStatusChecksWire is the branch-protection required-status-checks object. GitHub
// carries the required contexts in TWO shapes for compatibility: the legacy flat `contexts`
// list of names, and the newer `checks` array whose entries pair a context name with an
// optional app id. Both are decoded and unioned so a context named in only one shape is
// never missed — a required check the reader dropped would read as "nothing required", the
// fail-open direction.
type ghRequiredStatusChecksWire struct {
	Contexts []string `json:"contexts"`
	Checks   []struct {
		Context string `json:"context"`
	} `json:"checks"`
}

// ghBranchProtectedWire is the single field the admin-free fallback reads from
// `GET /repos/{o}/{r}/branches/{b}`: whether ANYTHING protects the branch. This endpoint is
// readable by a plain repo token (no `administration` scope), unlike the legacy protection
// endpoint, so it answers "is the required set necessarily empty" without admin rights.
type ghBranchProtectedWire struct {
	Protected bool `json:"protected"`
}

// ghBranchRuleWire is one entry of `GET /repos/{o}/{r}/rules/branches/{b}` — the EFFECTIVE
// rules applying to the branch, including those contributed by rulesets (not just classic
// branch protection). Only the `required_status_checks` rule type carries required contexts,
// under `parameters.required_status_checks[].context`; other rule types leave that slice
// empty and contribute nothing. This endpoint is readable by the same plain repo token, so it
// closes the ruleset gap the admin-only legacy endpoint leaves behind.
type ghBranchRuleWire struct {
	Type       string `json:"type"`
	Parameters struct {
		RequiredStatusChecks []struct {
			Context string `json:"context"`
		} `json:"required_status_checks"`
	} `json:"parameters"`
}

// ghTimelineWire is one entry of the issue/PR timeline. Only `labeled` events matter to the
// applier-aware label-event read, and only the label name plus the actor that applied it.
type ghTimelineWire struct {
	Event     string `json:"event"`
	CreatedAt string `json:"created_at"`
	Label     struct {
		Name string `json:"name"`
	} `json:"label"`
	Actor struct {
		Login string `json:"login"`
	} `json:"actor"`
}

// Pagination constants — extracted from deskpost's github.go, byte-for-byte. GitHub's
// default page size is 30, so an omitted per_page silently truncates a longer rollup; every
// walk here sends per_page=100 and reconciles against the total the head asserts.
const (
	forgeReviewPerPage = 100
	forgeFilePerPage   = 100
	forgeMaxFilePages  = 40 // 4000 entries; GitHub's own /files cap is 3000
	forgeCIPerPage     = 100
	forgeMaxCIPages    = 25 // 2500 items; exceeding it leaves len < TotalCount → fail closed
)

// --- Reads ---

func (g *GitHubForge) GetPullRequest(repo ForgeRepo, number int) (*PullRequest, error) {
	var w ghPullWire
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", repo.Owner, repo.Name, number)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	return ghPullFromWire(w), nil
}

// ghPullFromWire maps a decoded pull object onto the interface's PullRequest. It is shared by
// the single-change read (GetPullRequest) and the branch lookup (OpenChangeForBranch) so the
// two cannot map one wire shape two ways — the list endpoint the branch lookup uses returns the
// SAME object shape, minus the fields (mergeable, changed_files) the list form omits, which
// decode to their zero values (MergeableUnknown, 0) rather than a wrong value.
func ghPullFromWire(w ghPullWire) *PullRequest {
	labels := make([]string, 0, len(w.Labels))
	for _, l := range w.Labels {
		labels = append(labels, l.Name)
	}
	return &PullRequest{
		Number:       w.Number,
		State:        w.State,
		Draft:        w.Draft,
		NodeID:       w.NodeID,
		Title:        w.Title,
		Body:         w.Body,
		ChangedFiles: w.ChangedFiles,
		Author:       Account{Login: w.User.Login, ID: w.User.ID},
		HeadSHA:      w.Head.SHA,
		UpdatedAt:    w.UpdatedAt,
		MergedAt:     w.MergedAt,
		Merged:       w.Merged,
		Mergeable:    ghMergeableState(w.Mergeable),
		Labels:       labels,
		URL:          w.HTMLURL,
		HeadRef:      w.Head.Ref,
		BaseRef:      w.Base.Ref,
	}
}

func (g *GitHubForge) GetIssue(repo ForgeRepo, number int) (*Issue, error) {
	var w ghIssueWire
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", repo.Owner, repo.Name, number)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(w.Labels))
	for _, l := range w.Labels {
		labels = append(labels, l.Name)
	}
	return &Issue{
		Number:        w.Number,
		Title:         w.Title,
		State:         w.State,
		Author:        Account{Login: w.User.Login, ID: w.User.ID},
		IsPullRequest: w.PullRequest != nil,
		URL:           w.HTMLURL,
		Labels:        labels,
		Body:          w.Body,
	}, nil
}

// GetIssueTyped is GetIssue with the caller's stated kind VALIDATED against what the number
// is. GitHub numbers issues and pull requests in ONE sequence, so there is nothing to route
// on — the one read answers both — but a caller that said "issue" and is handed a pull
// request (or the reverse) would go on to act on the wrong kind of object under the right
// number, so the mismatch is a could-not-check error naming both, never a silent hand-back.
// A 404 is returned as-is (IsForgeNotFound holds). An unknown kind is refused.
func (g *GitHubForge) GetIssueTyped(repo ForgeRepo, number int, kind TargetKind) (*Issue, error) {
	switch kind {
	case TargetIssue, TargetChange:
	default:
		return nil, Refused(fmt.Sprintf("refused: GetIssueTyped: unknown target kind %q for %s#%d", string(kind), repo.Slug(), number))
	}
	iss, err := g.GetIssue(repo, number)
	if err != nil {
		return nil, err
	}
	if iss.IsPullRequest && kind == TargetIssue {
		return nil, Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d is a pull request, not an issue — state the kind you mean (--kind pr)",
			repo.Slug(), number), nil)
	}
	if !iss.IsPullRequest && kind == TargetChange {
		return nil, Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d is an issue, not a pull request — state the kind you mean (--kind issue)",
			repo.Slug(), number), nil)
	}
	return iss, nil
}

// OpenChangeForBranch resolves the single OPEN pull request whose HEAD branch is `branch`
// (`GET /repos/{o}/{r}/pulls?head={owner}:{branch}&state=open`). The head filter is spelled
// `owner:branch` — GitHub's own `user:ref` form — so it matches only same-repo branches, which
// is every change this desk opens. NONE open → (nil, nil). MORE THAN ONE → a could-not-check
// REFUSAL: two open PRs on one source branch has no single right answer, and a silent first-
// match would route deskpr's mergeable read or an edit at whichever GitHub listed first.
func (g *GitHubForge) OpenChangeForBranch(repo ForgeRepo, branch string) (*PullRequest, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, Unverifiable("OpenChangeForBranch needs a non-empty source branch for "+repo.Slug(), nil)
	}
	head := fmt.Sprintf("%s:%s", repo.Owner, branch)
	path := fmt.Sprintf("/repos/%s/%s/pulls?head=%s&state=open&per_page=%d",
		repo.Owner, repo.Name, url.QueryEscape(head), forgeFilePerPage)
	var w []ghPullWire
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	switch len(w) {
	case 0:
		return nil, nil
	case 1:
		return ghPullFromWire(w[0]), nil
	default:
		return nil, Unverifiable(fmt.Sprintf(
			"could-not-check: %d open changes share source branch %q in %s — refusing to guess which one is "+
				"meant; a single open change per source branch is the assumption this read is allowed to make, "+
				"and it does not hold here", len(w), branch, repo.Slug()), nil)
	}
}

// SearchIssues runs a repo-scoped free-text search over ISSUES only (never PRs): the query is
// prefixed with `repo:{o}/{r} is:issue`, so the caller supplies free text and the backend owns
// the scope qualifiers — the closed-surface property (no caller-supplied search syntax).
func (g *GitHubForge) SearchIssues(repo ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error) {
	q := strings.TrimSpace(fmt.Sprintf("repo:%s is:issue %s", repo.Slug(), strings.TrimSpace(in.Query)))
	path := fmt.Sprintf("/search/issues?q=%s&per_page=%d", url.QueryEscape(q), forgeSearchPerPage)
	var w ghIssueSearchWire
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	out := make([]IssueSearchResult, 0, len(w.Items))
	for _, it := range w.Items {
		labels := make([]string, 0, len(it.Labels))
		for _, l := range it.Labels {
			labels = append(labels, l.Name)
		}
		out = append(out, IssueSearchResult{
			Number: it.Number, Title: it.Title, State: it.State, Labels: labels, URL: it.HTMLURL,
		})
	}
	return out, nil
}

// ListLabels reads the repo's label NAMES (`GET /repos/{o}/{r}/labels`, paginated). Read-only:
// it never creates a label, which is the whole reason it is a separate op from ApplyLabels's
// ensure step (deskfile's probe files unstamped on a missing label rather than minting it).
func (g *GitHubForge) ListLabels(repo ForgeRepo) ([]string, error) {
	var all []string
	for page := 1; page <= forgeMaxFilePages; page++ {
		var chunk []ghLabelWire
		path := fmt.Sprintf("/repos/%s/%s/labels?per_page=%d&page=%d",
			repo.Owner, repo.Name, forgeFilePerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, l := range chunk {
			all = append(all, l.Name)
		}
		if len(chunk) < forgeFilePerPage {
			break
		}
	}
	return all, nil
}

// forgeIssuePerPage / forgeOpenChangesCap bound the two bulk board reads. The issue read
// paginates to exhaustion (per_page=100); the open-change read is a SINGLE bounded page
// whose cap is reported (OpenChanges.Cap) so a read that came back exactly full signals a
// possibly-truncated population rather than a confident count over an unknown remainder.
const (
	forgeIssuePerPage   = 100
	forgeOpenChangesCap = 100
	// forgeMaxIssuePages bounds the open-issue walk (the GitLab arm's gitlabMaxIssuePage
	// twin): a forge still advertising more pages past it is a could-not-check, never a
	// silently truncated listing.
	forgeMaxIssuePages = 100
	// forgeListChangesPerPage / forgeListChangesMaxPages bound the ListChanges read: up to
	// forgeListChangesMaxPages pages of forgeListChangesPerPage, most-recently-updated first.
	// Unlike the open-issue walk this does NOT refuse at the ceiling — the represented-PR
	// reconciliation would rather work off the recent window than hold every dispatch on a repo
	// whose lifetime merged-PR count exceeds the ceiling — so the ceiling is reported as
	// ChangeList.Incomplete and the consumer decides. The window (500 recent changes) covers the
	// currently-active briefs a phantom check reasons about.
	forgeListChangesPerPage  = 100
	forgeListChangesMaxPages = 5
)

// ghOpenChangesQuery is the bulk open-PR read, hand-authored so it requests EXACTLY the
// fields the board classifies on — and, deliberately, the rollup CONTEXTS without the
// `checkSuite { workflowRun … }` sub-selection gh's built-in `statusCheckRollup` field
// hardcodes. That sub-field is a LINK to the Actions run and needs `actions:read`; under an
// App holding only `checks:read` it 403s and, on a repo with many Actions suites, sinks the
// whole read to an empty non-2xx. Every conclusion the board reads (CheckRun.status/
// conclusion, StatusContext.state) is covered by `checks:read` alone, so requesting the
// contexts ourselves without checkSuite/workflowRun drops the scope dependency entirely.
const ghOpenChangesQuery = `query($owner:String!,$name:String!,$limit:Int!){repository(owner:$owner,name:$name){pullRequests(states:OPEN,first:$limit,orderBy:{field:CREATED_AT,direction:DESC}){nodes{number title body state isDraft createdAt lastEditedAt author{login __typename} mergeStateStatus headRefOid headRefName baseRefName labels(first:100){nodes{name}} commits(last:1){nodes{commit{statusCheckRollup{contexts(first:100){nodes{__typename ...on CheckRun{name status conclusion startedAt completedAt} ...on StatusContext{context state createdAt}}}}}}}}}}}`

// ghListChangesQuery is the states-scoped, cursor-paginated changes read behind ListChanges. It
// requests EXACTLY the ChangeRef fields — number, state, head oid, source branch (headRefName),
// title, body (with its link trailers), mergedAt — and no rollup, so it carries none of
// ghOpenChangesQuery's actions:read/checks:read scope surface. `$states` is a
// `[PullRequestState!]` variable (OPEN | MERGED | CLOSED) built from the ChangeStates the caller
// asked for; ordering is UPDATED_AT DESC so the bounded window is the changes most likely to
// represent a currently-queued brief; pageInfo drives the bounded cursor walk.
const ghListChangesQuery = `query($owner:String!,$name:String!,$first:Int!,$after:String,$states:[PullRequestState!]){repository(owner:$owner,name:$name){pullRequests(states:$states,first:$first,after:$after,orderBy:{field:UPDATED_AT,direction:DESC}){pageInfo{hasNextPage endCursor} nodes{number state headRefOid headRefName title body mergedAt}}}}`

func (g *GitHubForge) ListOpenChanges(repo ForgeRepo) (*OpenChanges, error) {
	in := map[string]any{
		"query": ghOpenChangesQuery,
		"variables": map[string]any{
			"owner": repo.Owner, "name": repo.Name, "limit": forgeOpenChangesCap,
		},
	}
	var out struct {
		Data struct {
			Repository struct {
				PullRequests struct {
					Nodes []struct {
						Number       int    `json:"number"`
						Title        string `json:"title"`
						Body         string `json:"body"`
						State        string `json:"state"`
						IsDraft      bool   `json:"isDraft"`
						CreatedAt    string `json:"createdAt"`
						LastEditedAt string `json:"lastEditedAt"`
						Author       *struct {
							Login    string `json:"login"`
							Typename string `json:"__typename"`
						} `json:"author"`
						MergeStateStatus string `json:"mergeStateStatus"`
						HeadRefOid       string `json:"headRefOid"`
						HeadRefName      string `json:"headRefName"`
						BaseRefName      string `json:"baseRefName"`
						Labels           struct {
							Nodes []struct {
								Name string `json:"name"`
							} `json:"nodes"`
						} `json:"labels"`
						Commits struct {
							Nodes []struct {
								Commit struct {
									StatusCheckRollup *struct {
										Contexts struct {
											Nodes []struct {
												Typename    string `json:"__typename"`
												Name        string `json:"name"`
												Status      string `json:"status"`
												Conclusion  string `json:"conclusion"`
												StartedAt   string `json:"startedAt"`
												CompletedAt string `json:"completedAt"`
												Context     string `json:"context"`
												State       string `json:"state"`
												CreatedAt   string `json:"createdAt"`
											} `json:"nodes"`
										} `json:"contexts"`
									} `json:"statusCheckRollup"`
								} `json:"commit"`
							} `json:"nodes"`
						} `json:"commits"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		msgs := make([]string, 0, len(out.Errors))
		for _, e := range out.Errors {
			msgs = append(msgs, e.Message)
		}
		return nil, Unverifiable("open-changes GraphQL error: "+strings.Join(msgs, "; "), nil)
	}
	nodes := out.Data.Repository.PullRequests.Nodes
	changes := make([]OpenChange, 0, len(nodes))
	for _, n := range nodes {
		oc := OpenChange{
			Number: n.Number, Title: n.Title, Body: n.Body, State: n.State, Draft: n.IsDraft,
			CreatedAt: n.CreatedAt, LastEditedAt: n.LastEditedAt, MergeStateStatus: n.MergeStateStatus,
			HeadSHA: n.HeadRefOid, HeadRef: n.HeadRefName, BaseRef: n.BaseRefName,
		}
		if n.Author != nil {
			// A GraphQL Bot actor carries the BARE slug as login; re-suffix it to
			// "<slug>[bot]" so the trust set sees the same REST rendering it does elsewhere.
			// A null author (deleted account) stays "" — untrusted, fail closed.
			login := n.Author.Login
			if n.Author.Typename == "Bot" {
				login += "[bot]"
			}
			oc.Author = Account{Login: login}
		}
		for _, l := range n.Labels.Nodes {
			oc.Labels = append(oc.Labels, l.Name)
		}
		if len(n.Commits.Nodes) > 0 {
			if r := n.Commits.Nodes[0].Commit.StatusCheckRollup; r != nil {
				for _, c := range r.Contexts.Nodes {
					oc.Rollup = append(oc.Rollup, RollupNode{
						Typename: c.Typename, Name: c.Name, Status: c.Status, Conclusion: c.Conclusion,
						StartedAt: c.StartedAt, CompletedAt: c.CompletedAt,
						Context: c.Context, State: c.State, CreatedAt: c.CreatedAt,
					})
				}
			}
		}
		changes = append(changes, oc)
	}
	return &OpenChanges{
		Changes:        changes,
		Cap:            forgeOpenChangesCap,
		TruncatedAtCap: len(changes) >= forgeOpenChangesCap,
	}, nil
}

// ghChangeStates maps a ChangeStates to the GraphQL PullRequestState enum values, in a stable
// order so the query variable (and the golden corpus) is deterministic.
func ghChangeStates(states ChangeStates) []string {
	var out []string
	if states.Open {
		out = append(out, "OPEN")
	}
	if states.Merged {
		out = append(out, "MERGED")
	}
	if states.Closed {
		out = append(out, "CLOSED")
	}
	return out
}

func (g *GitHubForge) ListChanges(repo ForgeRepo, states ChangeStates) (*ChangeList, error) {
	if !states.Any() {
		return nil, Unverifiable("ListChanges was asked for no states — the state set must be stated "+
			"(open/merged/closed), never defaulted to a whole-repo scan", nil)
	}
	wantStates := ghChangeStates(states)
	out := &ChangeList{PageCap: forgeListChangesMaxPages}
	var after *string
	for page := 1; page <= forgeListChangesMaxPages; page++ {
		vars := map[string]any{
			"owner": repo.Owner, "name": repo.Name,
			"first": forgeListChangesPerPage, "states": wantStates,
		}
		// A nil `after` on page 1 is sent as GraphQL null — the connection's start.
		vars["after"] = after
		in := map[string]any{"query": ghListChangesQuery, "variables": vars}
		var resp struct {
			Data struct {
				Repository struct {
					PullRequests struct {
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []struct {
							Number      int    `json:"number"`
							State       string `json:"state"`
							HeadRefOid  string `json:"headRefOid"`
							HeadRefName string `json:"headRefName"`
							Title       string `json:"title"`
							Body        string `json:"body"`
							MergedAt    string `json:"mergedAt"`
						} `json:"nodes"`
					} `json:"pullRequests"`
				} `json:"repository"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := g.doJSON(http.MethodPost, "/graphql", in, &resp); err != nil {
			return nil, err
		}
		if len(resp.Errors) > 0 {
			msgs := make([]string, 0, len(resp.Errors))
			for _, e := range resp.Errors {
				msgs = append(msgs, e.Message)
			}
			return nil, Unverifiable("list-changes GraphQL error: "+strings.Join(msgs, "; "), nil)
		}
		conn := resp.Data.Repository.PullRequests
		for _, n := range conn.Nodes {
			out.Changes = append(out.Changes, ChangeRef{
				Number: n.Number, State: strings.ToUpper(n.State), HeadSHA: n.HeadRefOid,
				HeadRef: n.HeadRefName, Title: n.Title, Body: n.Body, MergedAt: n.MergedAt,
			})
		}
		if !conn.PageInfo.HasNextPage {
			return out, nil
		}
		if page == forgeListChangesMaxPages {
			// The ceiling was reached with the forge still paginating — report the population
			// as larger than the window rather than hand back a silent partial.
			out.Incomplete = true
			return out, nil
		}
		cursor := conn.PageInfo.EndCursor
		after = &cursor
	}
	return out, nil
}

// ListOpenIssues walks a repo's OPEN issues to exhaustion (per_page=100), PRs dropped.
//
// End-of-walk (#1032): the walk continues while EITHER the page came back full OR the
// forge's own `Link: rel="next"` says there is more, and stops only when both say there is
// not. Inferring the end from a short page alone let a page that came back short under
// load (a slow, rate-limited or hiccuping upstream) end the walk early with a nil error —
// and cmd/issueboard reads an issue's absence from this listing as evidence about it.
func (g *GitHubForge) ListOpenIssues(repo ForgeRepo) ([]IssueSummary, error) {
	var out []IssueSummary
	for page := 1; page <= forgeMaxIssuePages; page++ {
		var chunk []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			User   struct {
				Login string `json:"login"`
				ID    int64  `json:"id"`
			} `json:"user"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
			CreatedAt   string    `json:"created_at"`
			HTMLURL     string    `json:"html_url"`
			PullRequest *struct{} `json:"pull_request"`
		}
		path := fmt.Sprintf("/repos/%s/%s/issues?state=open&per_page=%d&page=%d",
			repo.Owner, repo.Name, forgeIssuePerPage, page)
		hdr, err := g.doJSONHeader(http.MethodGet, path, nil, &chunk)
		if err != nil {
			return nil, err
		}
		for _, is := range chunk {
			// The REST /issues endpoint serves PRs too, distinguished by a non-nil
			// pull_request member — the issue lane wants issues only, so a change is dropped.
			if is.PullRequest != nil {
				continue
			}
			labels := make([]string, 0, len(is.Labels))
			for _, l := range is.Labels {
				labels = append(labels, l.Name)
			}
			out = append(out, IssueSummary{
				Number: is.Number, Title: is.Title,
				Author:    Account{Login: is.User.Login, ID: is.User.ID},
				Labels:    labels,
				CreatedAt: is.CreatedAt,
				URL:       is.HTMLURL,
			})
		}
		if len(chunk) < forgeIssuePerPage && !hasNextLink(hdr) {
			return out, nil
		}
	}
	return nil, Unverifiable(fmt.Sprintf(
		"could-not-check: %s still reports more open issues after %d pages of %d (the open-issue page "+
			"ceiling) — refusing to hand back a PARTIAL open-issue set, because the issue lane reads an "+
			"issue's absence from this list as evidence about it and would retire placeholders for issues "+
			"that are still open",
		repo.Slug(), forgeMaxIssuePages, forgeIssuePerPage), nil)
}

func (g *GitHubForge) PRTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	return g.trustEvents(repo, number, PRTrustQuery, true)
}

func (g *GitHubForge) IssueTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	return g.trustEvents(repo, number, IssueTrustQuery, false)
}

const (
	// forgeMaxEventPages bounds the escalation-clock comment walk: 20 pages of first:100 =
	// 2000 comments. A thread still advertising a next page past it is reported INCOMPLETE
	// (TrustPayload.Complete=false), which the caller treats as one issue's conservative
	// could-not-check (escalate) — never as a whole-board failure and never as "no escalation
	// owed". The bound keeps the read finite regardless of thread length.
	forgeMaxEventPages = 20
)

// ghIssueEventsRespWire decodes ONE page of IssueEventsQuery. A null `issue` (wrong number,
// or no access) is distinguished from an issue with no comments — the pointer is nil in the
// first case, which the caller maps to could-not-check rather than "no events".
type ghIssueEventsRespWire struct {
	Data struct {
		Repository struct {
			Issue *struct {
				Comments struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						CreatedAt string    `json:"createdAt"`
						Author    *gqlActor `json:"author"`
					} `json:"nodes"`
				} `json:"comments"`
			} `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// IssueContentEvents walks an issue's comment thread to exhaustion under forgeMaxEventPages,
// mapping each comment to a ContentEvent (author rendered as the trust set expects — a Bot
// re-suffixed "<slug>[bot]" — plus its creation time) for the escalation clock. It is the
// paginated counterpart of the single-page trustEvents read: see the interface doc on
// IssueContentEvents for why the escalation clock must not share the trust gate's fail-closed
// single-page bound. Complete=false means the hard cap was hit (or the forge advertised a next
// page with no cursor); the caller then treats that issue conservatively without failing the
// board.
func (g *GitHubForge) IssueContentEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	var events []ContentEvent
	after := ""
	for page := 1; page <= forgeMaxEventPages; page++ {
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number}
		if after != "" {
			vars["after"] = after
		}
		in := map[string]any{"query": IssueEventsQuery, "variables": vars}
		var out ghIssueEventsRespWire
		if err := g.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
			return nil, err
		}
		if len(out.Errors) > 0 {
			msgs := make([]string, 0, len(out.Errors))
			for _, e := range out.Errors {
				msgs = append(msgs, e.Message)
			}
			return nil, Unverifiable("issue-events GraphQL error: "+strings.Join(msgs, "; "), nil)
		}
		iss := out.Data.Repository.Issue
		if iss == nil {
			return nil, Unverifiable(fmt.Sprintf(
				"could-not-check: %s carries no issue at number %d, so its comment thread could not be read",
				repo.Slug(), number), nil)
		}
		for i := range iss.Comments.Nodes {
			n := &iss.Comments.Nodes[i]
			if n.CreatedAt == "" {
				continue
			}
			ct, err := parseTrustTime(n.CreatedAt)
			if err != nil {
				return nil, Unverifiable(fmt.Sprintf("cannot read issue-event createdAt for %s#%d", repo.Slug(), number), err)
			}
			var id int64
			if n.Author != nil {
				id = n.Author.DatabaseID
			}
			events = append(events, ContentEvent{Author: n.Author.renderedLogin(), AuthorID: id, CreatedAt: ct})
		}
		if !iss.Comments.PageInfo.HasNextPage {
			return &TrustPayload{Events: events, Complete: true}, nil
		}
		after = iss.Comments.PageInfo.EndCursor
		if after == "" {
			// A next page is advertised but no cursor was returned: the walk cannot advance,
			// so report INCOMPLETE rather than loop the same page. The caller degrades that
			// one issue conservatively.
			break
		}
	}
	return &TrustPayload{Events: events, Complete: false}, nil
}

// trustEvents runs one trust-gate GraphQL query through the backend's own authenticated
// transport and parses the response through the SAME reader (trustFromEnvelope) the CLI
// surfaces use, so the seam and the CLI cannot draw different blessings from one payload.
func (g *GitHubForge) trustEvents(repo ForgeRepo, number int, gql string, pr bool) (*TrustPayload, error) {
	in := map[string]any{
		"query": gql,
		"variables": map[string]any{
			"owner": repo.Owner, "name": repo.Name, "number": number,
		},
	}
	var env gqlEnvelope
	if err := g.doJSON(http.MethodPost, "/graphql", in, &env); err != nil {
		return nil, err
	}
	tp, err := trustFromEnvelope(env, pr)
	if err != nil {
		return nil, Unverifiable(fmt.Sprintf("cannot read trust events for %s#%d", repo.Slug(), number), err)
	}
	return &tp, nil
}

func (g *GitHubForge) ReviewsAtHead(repo ForgeRepo, number int) ([]Review, error) {
	var all []Review
	for page := 1; ; page++ {
		var chunk []ghReviewWire
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews?per_page=%d&page=%d",
			repo.Owner, repo.Name, number, forgeReviewPerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, r := range chunk {
			all = append(all, Review{
				ID:          r.ID,
				Author:      Account{Login: r.User.Login, ID: r.User.ID},
				State:       r.State,
				CommitID:    r.CommitID,
				Body:        r.Body,
				SubmittedAt: r.SubmittedAt,
			})
		}
		if len(chunk) < forgeReviewPerPage {
			break
		}
	}
	return all, nil
}

func (g *GitHubForge) ListChangedFiles(repo ForgeRepo, number int) ([]ChangedFile, error) {
	var all []ChangedFile
	for page := 1; page <= forgeMaxFilePages; page++ {
		var chunk []ghFileWire
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=%d&page=%d",
			repo.Owner, repo.Name, number, forgeFilePerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, f := range chunk {
			all = append(all, ChangedFile{
				Filename:         f.Filename,
				PreviousFilename: f.PreviousFilename,
				Status:           f.Status,
			})
		}
		if len(chunk) < forgeFilePerPage {
			break
		}
	}
	return all, nil
}

func (g *GitHubForge) ChecksAtHead(repo ForgeRepo, sha string) (*ChecksAtHead, error) {
	out := &ChecksAtHead{}
	// Legacy combined-status rollup.
	for page := 1; page <= forgeMaxCIPages; page++ {
		var cs ghCombinedStatusWire
		path := fmt.Sprintf("/repos/%s/%s/commits/%s/status?per_page=%d&page=%d",
			repo.Owner, repo.Name, sha, forgeCIPerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &cs); err != nil {
			return nil, err
		}
		if page == 1 {
			out.CombinedState = cs.State
			out.StatusTotalCount = cs.TotalCount
		}
		for _, s := range cs.Statuses {
			out.Statuses = append(out.Statuses, StatusContext{
				State: s.State, Context: s.Context, CreatedAt: s.CreatedAt,
			})
		}
		if len(cs.Statuses) < forgeCIPerPage {
			break
		}
	}
	// Check-runs rollup.
	for page := 1; page <= forgeMaxCIPages; page++ {
		var cr ghCheckRunsWire
		path := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?per_page=%d&page=%d",
			repo.Owner, repo.Name, sha, forgeCIPerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &cr); err != nil {
			return nil, err
		}
		if page == 1 {
			out.CheckRunsTotalCount = cr.TotalCount
		}
		for _, c := range cr.CheckRuns {
			out.CheckRuns = append(out.CheckRuns, CheckRun{
				ID:   checkRunID(c.ID),
				Name: c.Name, Status: c.Status, Conclusion: c.Conclusion,
				StartedAt: c.StartedAt, CompletedAt: c.CompletedAt,
			})
		}
		if len(cr.CheckRuns) < forgeCIPerPage {
			break
		}
	}
	return out, nil
}

// RequiredStatusChecks reads the branch's required status-check contexts from GitHub branch
// protection (`GET /repos/{o}/{r}/branches/{branch}/protection/required_status_checks`).
//
// The 404 IS the answer, not an error. GitHub returns 404 both for a branch with no
// protection AND for a protected branch that requires no status checks; in either case
// nothing gates the merge on a check, so the required set is EMPTY and no error is returned.
//
// A 403 is NOT the answer, but it is not a dead end either. The legacy protection endpoint
// needs the `administration` scope, which the reviewer/worker App tokens do not carry, so it
// answers 403 on every repo for those identities. Reading a 403 as "nothing required" would
// be fail-open; returning it as-is would leave the flip permanently could-not-check on every
// App token. Instead the read re-resolves through admin-free endpoints (see
// requiredChecksAdminFree), which — being a flip gate — still fail CLOSED: only a positively
// unprotected branch yields empty/green; a protected branch whose required set cannot be
// determined admin-free stays could-not-check.
//
// Every OTHER non-2xx (401, 5xx, a parse failure) is could-not-check and is returned as-is, so
// the caller fails closed — an absent rollup is never read as green off a required-set the
// tool could not actually read.
func (g *GitHubForge) RequiredStatusChecks(repo ForgeRepo, branch string) ([]string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, Unverifiable("cannot read the required status checks without a branch name — "+
			"which branch's protection applies is undetermined, and undetermined is could-not-check", nil)
	}
	var w ghRequiredStatusChecksWire
	path := fmt.Sprintf("/repos/%s/%s/branches/%s/protection/required_status_checks",
		repo.Owner, repo.Name, url.PathEscape(branch))
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		if IsForgeNotFound(err) {
			// No branch protection, or protection with no required checks: nothing is
			// required to merge, so the required set is empty. This is the honest "green" for
			// an absent rollup, and it is distinct from the error return below.
			return nil, nil
		}
		if IsForgeForbidden(err) {
			// The token lacks `administration` for the legacy endpoint. Re-resolve the same
			// required set through the admin-free endpoints rather than fail the whole flip.
			return g.requiredChecksAdminFree(repo, branch, err)
		}
		return nil, err
	}
	return dedupContexts(w.Contexts, contextsOf(w.Checks)), nil
}

// requiredChecksAdminFree re-resolves the branch's required status-check contexts WITHOUT the
// `administration` scope the legacy protection endpoint demands, for tokens that get a 403
// there (every reviewer/worker App). It uses two endpoints a plain repo token can read:
//
//  1. GET /repos/{o}/{r}/branches/{b} → `.protected`. If the branch is NOT protected at all,
//     nothing gates the merge on a check, so the required set is empty (⇒ green). This is the
//     ONLY admin-free path to an empty/green answer.
//  2. GET /repos/{o}/{r}/rules/branches/{b} → the branch's RULESET rules (NOT classic branch
//     protection — the rules API surfaces rulesets only). The union of every
//     `required_status_checks` rule's contexts is the required set from rulesets.
//
// This is a flip GATE, so it fails CLOSED: the only outputs are an empty set (positively
// unprotected), a NON-empty set (rulesets name required contexts), or could-not-check. In
// particular a branch that is `protected: true` but whose rules endpoint returns NO
// required_status_checks contexts is could-not-check, NOT empty: the tell of CLASSIC branch
// protection, which protects the branch (and may require checks) but is invisible to the
// rules API — reading it as empty would let deskflip flip an un-green PR off an absent rollup
// (the pre-fix behaviour failed closed here, and this must too). Every unreadable-or-unknown
// path returns Unverifiable; legacyErr is threaded into the both-failed refusal for context.
func (g *GitHubForge) requiredChecksAdminFree(repo ForgeRepo, branch string, legacyErr error) ([]string, error) {
	var bp ghBranchProtectedWire
	bpath := fmt.Sprintf("/repos/%s/%s/branches/%s", repo.Owner, repo.Name, url.PathEscape(branch))
	brErr := g.doJSON(http.MethodGet, bpath, nil, &bp)
	if brErr == nil && !bp.Protected {
		// The branch is not protected: nothing GitHub enforces gates the merge on a check, so
		// the required set is empty. This is the only admin-free empty/green answer.
		return nil, nil
	}
	// The branch is protected, or its protection flag could not be read. Read the ruleset rules
	// for any required contexts they name.
	var rules []ghBranchRuleWire
	rpath := fmt.Sprintf("/repos/%s/%s/rules/branches/%s", repo.Owner, repo.Name, url.PathEscape(branch))
	if rErr := g.doJSON(http.MethodGet, rpath, nil, &rules); rErr != nil {
		if brErr != nil {
			// BOTH admin-free fallbacks failed: the required set could not be read at all.
			// Fail closed — an absent rollup must never be read as green off a set this tool
			// could not determine.
			return nil, Unverifiable(fmt.Sprintf(
				"cannot read the required status checks for %s@%s: the legacy protection endpoint is "+
					"forbidden (%v) and both admin-free fallbacks failed (branch read: %v; rules read: %v)",
				repo.Slug(), branch, legacyErr, brErr, rErr), nil)
		}
		// The branch IS protected (step 1 succeeded) but its rules could not be read, so the
		// required set is unknown. Fail closed rather than assume none required.
		return nil, Unverifiable(fmt.Sprintf(
			"cannot read the required status checks for %s@%s: %s is protected but its effective rules "+
				"could not be read (%v)", repo.Slug(), branch, branch, rErr), nil)
	}
	var ctxs []string
	for _, rule := range rules {
		for _, c := range rule.Parameters.RequiredStatusChecks {
			ctxs = append(ctxs, c.Context)
		}
	}
	if set := dedupContexts(ctxs); len(set) > 0 {
		return set, nil
	}
	// Protected (or protection-flag unreadable) AND the rules API named no required contexts.
	// This does NOT prove nothing is required: the rules API shows only rulesets, so a branch
	// under CLASSIC protection reads exactly this way while still gating the merge. Fail closed
	// — could-not-check, never an empty/green set off an admin-free read that cannot see
	// classic protection.
	//
	// The message names the exact permission gap rather than leaving it generic. Every read
	// that lands here does so because the legacy endpoint 403'd (no other path reaches this
	// return), which for a GitHub App token means exactly one thing: the token lacks the
	// `administration` repository permission (read is sufficient) the legacy endpoint requires.
	// That is a ONE-TIME permission decision, not a per-PR judgment call — a caller that reads
	// this as an ordinary could-not-check and re-asks a human on every flip attempt is treating
	// a fixed fact as if it might resolve itself. Naming the permission here is what lets an
	// operator or desk recognise "grant this once" instead of "decide this again."
	return nil, Unverifiable(fmt.Sprintf(
		"cannot read the required status checks for %s@%s: %s is protected but the rules API named no "+
			"required status checks — the tell of CLASSIC branch protection, which the rules API cannot "+
			"see, so the required set is undetermined (could-not-check, never read as green). This is a "+
			"PERMISSION gap, not a per-PR judgment call: the calling App token lacks the `administration: "+
			"read` repository permission the legacy branch-protection endpoint (GET "+
			"/repos/{owner}/{repo}/branches/{branch}/protection/required_status_checks) requires to see "+
			"classic protection's required-checks list. Every flip on this branch will stay could-not-check "+
			"until that permission is granted ONCE — escalate it as a permission grant, not as a recurring "+
			"human decision.",
		repo.Slug(), branch, branch), nil)
}

// contextsOf flattens the `checks` shape (context + optional app id) to its context names.
func contextsOf(checks []struct {
	Context string `json:"context"`
}) []string {
	out := make([]string, 0, len(checks))
	for _, c := range checks {
		out = append(out, c.Context)
	}
	return out
}

// dedupContexts unions any number of context-name lists into a single de-duplicated,
// whitespace-trimmed, order-preserving slice (empty entries dropped).
func dedupContexts(lists ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range lists {
		for _, c := range list {
			c = strings.TrimSpace(c)
			if c == "" || seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// IssueReactions is SINGLE PAGE by decision — the same reasoning HTTPRepoInfoFetcher's
// IssueReactions carries in full (repovis.go): fails closed past 100 reactions on one
// issue, never a false pass. Routed through the go-gh client like every other read; the
// squirrel-girl preview accept header the pre-go-gh raw request set is no longer required
// (the reactions API has been GA for years and returns awards under the default accept).
func (g *GitHubForge) IssueReactions(repo ForgeRepo, number int) ([]Reaction, error) {
	var reactions []Reaction
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/reactions?per_page=100", repo.Owner, repo.Name, number)
	if err := g.doJSON(http.MethodGet, path, nil, &reactions); err != nil {
		return nil, err
	}
	return reactions, nil
}

// ListLabelEvents walks the issue/PR TIMELINE and returns its `labeled` events with the
// login that applied each one.
//
// It reads the timeline rather than the current label set on purpose: the current set says
// only WHAT labels are on the change, and the model-capability floor's whole question is WHO
// applied the tier stamp. A dispatcher's attestation and a stamp the PR author applied to
// itself are indistinguishable in the label list and distinguishable only here.
//
// An EMPTY result is a change with no label applications, which the floor reads as
// UNATTESTED. A read FAILURE propagates — the caller refuses could-not-check rather than
// treating an unreadable timeline as an empty one.
func (g *GitHubForge) ListLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error) {
	var out []LabelEvent
	for page := 1; page <= forgeMaxFilePages; page++ {
		var chunk []ghTimelineWire
		path := fmt.Sprintf("/repos/%s/%s/issues/%d/timeline?per_page=%d&page=%d",
			repo.Owner, repo.Name, number, forgeFilePerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, e := range chunk {
			if e.Event != "labeled" {
				continue
			}
			out = append(out, LabelEvent{Name: e.Label.Name, AppliedBy: e.Actor.Login, CreatedAt: e.CreatedAt})
		}
		if len(chunk) < forgeFilePerPage {
			break
		}
	}
	return out, nil
}

// ghCommentsQuery reads a change's comments with the two properties REST does not carry: the
// GraphQL node id an edit targets, and `isMinimized`. A minimised (hidden/collapsed) comment
// must never be picked up and edited — it was hidden deliberately — and REST's issue-comments
// listing has no field for that state at all, so this read is GraphQL by necessity rather
// than by preference.
//
// The author selection carries `__typename` alongside `login` because a GraphQL Bot actor
// reports the BARE slug as its login (`assay-worker-app`), NOT the `<slug>[bot]` REST
// rendering every identity comparison in this house expects — `RoleAppLogin` returns
// `<slug>[bot]`, and `SameActor` folds the two renderings ONLY when both carry the App
// affix. Without the re-suffix below, a worker's own workpad comment never matched its own
// identity, so `deskreply --workpad` never found a candidate and appended a second comment
// every call (#747). This is the same fold `ghOpenChangesQuery`
// already applies to a PR/review author.
//
// `first: 100` is the same bound the call site it replaces used, and the same stated
// residual: a change with more than 100 comments is read as its first 100, never silently
// re-ordered.
const ghCommentsQuery = `query($owner:String!, $name:String!, $number:Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      comments(first: 100) {
        nodes {
          id
          databaseId
          body
          isMinimized
          createdAt
          url
          author { login __typename ... on User { databaseId } ... on Bot { databaseId } ... on Organization { databaseId } ... on Mannequin { databaseId } }
        }
      }
    }
  }
}`

// ghIssueCommentsQuery is ghCommentsQuery's ISSUE half. GitHub's GraphQL schema keeps
// `issue` and `pullRequest` as separate selections on a repository, so one query cannot
// serve both; the node selection below is byte-identical to the pull-request one, so the
// two reads produce the same Comment shape and nothing downstream has to know which ran.
//
// Unlike the change half, the issue half is WALKED: it carries a `pageInfo` and an `$after`
// cursor and listCommentsGQL follows it to the end of the thread under forgeMaxEventPages.
// An issue thread is read for its NEWEST answer (statusgen's un-block lane keys on the newest
// blessing-authority comment, and GitLab's listNotes already walks every page), and the
// newest comments are exactly the ones a first-100 read drops on a long thread — the defect
// the `gh api --paginate` read this replaces was fixed for. A thread still advertising a next
// page at the cap is could-not-check, never a silently truncated thread.
const ghIssueCommentsQuery = `query($owner:String!, $name:String!, $number:Int!, $after:String) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      comments(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          databaseId
          body
          isMinimized
          createdAt
          url
          author { login __typename ... on User { databaseId } ... on Bot { databaseId } ... on Organization { databaseId } ... on Mannequin { databaseId } }
        }
      }
    }
  }
}`

// ghCommentNodeWire is one comment node of either query's `comments` connection.
type ghCommentNodeWire struct {
	ID          string `json:"id"`
	DatabaseID  int64  `json:"databaseId"`
	Body        string `json:"body"`
	IsMinimized bool   `json:"isMinimized"`
	CreatedAt   string `json:"createdAt"`
	URL         string `json:"url"`
	Author      struct {
		Login      string `json:"login"`
		Typename   string `json:"__typename"`
		DatabaseID int64  `json:"databaseId"`
	} `json:"author"`
}

// ghCommentsConnWire is the `comments` connection hanging off one noteable. PageInfo is
// selected only by the ISSUE query (the change query does not ask for it, so it decodes to
// the zero value — no next page — and the change read stays a single request).
type ghCommentsConnWire struct {
	Comments struct {
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
		Nodes []ghCommentNodeWire `json:"nodes"`
	} `json:"comments"`
}

// ghCommentsRespWire decodes either comments query. Exactly one of the two noteables is
// selected by the query that ran, and a noteable that does not exist at the number comes
// back NULL — which is why both are pointers: a nil here is "the forge resolved no object
// of that kind", not "an object with no comments", and the two must not be conflated.
type ghCommentsRespWire struct {
	Data struct {
		Repository struct {
			PullRequest *ghCommentsConnWire `json:"pullRequest"`
			Issue       *ghCommentsConnWire `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// listCommentsGQL runs the comments query for ONE kind and maps its nodes. It is the shared
// body of ListComments (changes) and ListCommentsTyped (either kind); the request it emits
// for a change is byte-identical to the one the golden corpus pins.
//
// An ISSUE thread is walked page by page (see ghIssueCommentsQuery); a change thread is the
// single first-100 request the golden corpus pins.
func (g *GitHubForge) listCommentsGQL(repo ForgeRepo, number int, kind TargetKind) ([]Comment, error) {
	if kind != TargetIssue {
		return g.listCommentsPage(repo, number, kind, "", nil)
	}
	var res []Comment
	after := ""
	for page := 1; page <= forgeMaxEventPages; page++ {
		var next pageCursor
		chunk, err := g.listCommentsPage(repo, number, kind, after, &next)
		if err != nil {
			return nil, err
		}
		res = append(res, chunk...)
		if !next.hasNext {
			if res == nil {
				res = []Comment{}
			}
			return res, nil
		}
		if next.cursor == "" {
			// A next page is advertised with no cursor to advance on: the walk cannot move,
			// so the thread's tail is unread. Refuse rather than loop the same page or hand
			// back the head of the thread as if it were all of it.
			return nil, Unverifiable(fmt.Sprintf(
				"could-not-check: %s#%d advertises more comments but no cursor to read them with — "+
					"refusing to report a partial issue thread as the whole thread", repo.Slug(), number), nil)
		}
		after = next.cursor
	}
	return nil, Unverifiable(fmt.Sprintf(
		"could-not-check: %s#%d still reports more comments after %d pages of 100 — refusing to report a "+
			"partial issue thread as the whole thread (its newest comments are the unread ones)",
		repo.Slug(), number, forgeMaxEventPages), nil)
}

// pageCursor is one comments page's continuation: whether the forge advertised a further page
// and the cursor to request it with.
type pageCursor struct {
	hasNext bool
	cursor  string
}

// listCommentsPage runs ONE request of the comments query for one kind and maps its nodes.
// after is the page cursor ("" for the first page, which is then sent with no `after`
// variable at all, so the first request of an issue walk carries only the three coordinates);
// next, when non-nil, receives the page's continuation.
func (g *GitHubForge) listCommentsPage(repo ForgeRepo, number int, kind TargetKind, after string, next *pageCursor) ([]Comment, error) {
	query := ghCommentsQuery
	if kind == TargetIssue {
		query = ghIssueCommentsQuery
	}
	vars := map[string]any{
		"owner": repo.Owner, "name": repo.Name, "number": number,
	}
	if after != "" {
		vars["after"] = after
	}
	in := map[string]any{
		"query":     query,
		"variables": vars,
	}
	var out ghCommentsRespWire
	if err := g.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
		return nil, err
	}
	// A non-empty top-level `errors` is reported even when `data` came back partly
	// populated — GraphQL's own partial-failure convention. A partial comment list read as
	// a complete one is how a "no existing comment" conclusion gets drawn from a failed read.
	if len(out.Errors) > 0 {
		msgs := make([]string, 0, len(out.Errors))
		for _, e := range out.Errors {
			msgs = append(msgs, e.Message)
		}
		return nil, Unverifiable("comments GraphQL error: "+strings.Join(msgs, "; "), nil)
	}
	conn := out.Data.Repository.PullRequest
	if kind == TargetIssue {
		conn = out.Data.Repository.Issue
	}
	if conn == nil {
		// The noteable resolved to null: there is no object of the stated kind at this
		// number. Reporting that as an empty comment list is the unread-precondition
		// failure — a caller asking "does a proposal stand?" would read it as "no".
		return nil, Unverifiable(fmt.Sprintf(
			"could-not-check: %s carries no %s at number %d, so its comment thread could not be read",
			repo.Slug(), kindNoun(kind), number), nil)
	}
	if next != nil {
		next.hasNext = conn.Comments.PageInfo.HasNextPage
		next.cursor = conn.Comments.PageInfo.EndCursor
	}
	nodes := conn.Comments.Nodes
	res := make([]Comment, 0, len(nodes))
	for _, n := range nodes {
		// A GraphQL Bot actor carries the BARE slug as login; re-suffix it to "<slug>[bot]"
		// so an identity comparison (SameActor against RoleAppLogin's "<slug>[bot]") sees the
		// same REST rendering it does elsewhere — without this the worker's own workpad comment
		// never matches its own identity (#747). A null author (deleted
		// account) stays "" — untrusted, fail closed. Same fold ghOpenChangesQuery applies.
		login := n.Author.Login
		if n.Author.Typename == "Bot" && login != "" {
			login += "[bot]"
		}
		res = append(res, Comment{
			ID:         n.ID,
			DatabaseID: n.DatabaseID,
			Author:     Account{Login: login, ID: n.Author.DatabaseID},
			Body:       n.Body,
			Minimized:  n.IsMinimized,
			CreatedAt:  n.CreatedAt,
			URL:        n.URL,
		})
	}
	return res, nil
}

func (g *GitHubForge) ListComments(repo ForgeRepo, number int) ([]Comment, error) {
	return g.listCommentsGQL(repo, number, TargetChange)
}

// ListCommentsTyped reads the thread of the object of the STATED kind. On GitHub the two
// kinds share a number sequence but NOT a GraphQL selection, so the kind picks the query —
// `issue(number:)` or `pullRequest(number:)` — and a number that names the other kind comes
// back as a null noteable, which is reported as could-not-check rather than as an empty
// thread. An unknown kind is refused rather than defaulted.
func (g *GitHubForge) ListCommentsTyped(repo ForgeRepo, number int, kind TargetKind) ([]Comment, error) {
	switch kind {
	case TargetIssue, TargetChange:
	default:
		return nil, Refused(fmt.Sprintf("refused: ListCommentsTyped: unknown target kind %q for %s#%d",
			string(kind), repo.Slug(), number))
	}
	return g.listCommentsGQL(repo, number, kind)
}

// ghEditCommentMutation replaces an issue comment's body. The target is the comment's
// GraphQL node id, which is why Comment.ID is opaque: a REST numeric id will not address
// this mutation, and composing one locally is not possible.
const ghEditCommentMutation = `mutation($id: ID!, $body: String!) {
  updateIssueComment(input: {id: $id, body: $body}) {
    issueComment { databaseId }
  }
}`

func (g *GitHubForge) EditComment(repo ForgeRepo, commentID, body string) error {
	if strings.TrimSpace(commentID) == "" {
		return Refused("refusing to edit a comment with no id — the id comes from ListComments, " +
			"never from a locally composed value")
	}
	in := map[string]any{
		"query":     ghEditCommentMutation,
		"variables": map[string]any{"id": commentID, "body": body},
	}
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return Unverifiable("updateIssueComment GraphQL error: "+out.Errors[0].Message, nil)
	}
	return nil
}

// sortedKeys renders a set's members in a deterministic order. Label removals are issued one
// request at a time, so an unordered map walk would make the REQUEST SEQUENCE — which the
// golden corpus pins — differ run to run for the same input.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ApplyLabels reconciles a change's labels: it ensures every Add label exists on the repo,
// removes the named and stale-family labels, then applies the Add set.
//
// ORDER IS LOAD-BEARING. Creation comes first because applying a label the repo does not
// carry is not idempotent on GitHub; removal comes before application so a re-run of a
// family REPLACES rather than momentarily stacks. The label list is read ONCE, before any
// write, and the removals are computed from it — so a caller never has to make a listing call
// of its own (which is what would have put a second label operation on the frozen interface).
//
// Every step degrades in the direction the operation is idempotent in: a create that comes
// back 422 (already exists) is the SUCCESS case for an ensure, and a removal that comes back
// 404 (already absent) is the success case for a removal. Anything else propagates.
//
// change.Target is required but does not change the requests here: GitHub numbers issues and
// pull requests in ONE sequence and labels both through `/issues/{n}/labels`, so an issue and
// a change map to the same calls. The unset refusal still stands on this backend so a caller
// that forgot the target is caught by the forge most contributors run, not only on GitLab
// where the two kinds are separate sequences.
func (g *GitHubForge) ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error) {
	if err := change.requireTarget(); err != nil {
		return nil, err
	}
	out := &LabelOutcome{}
	adding := map[string]bool{}
	for _, l := range change.Add {
		if strings.TrimSpace(l.Name) == "" {
			return nil, Refused("refusing to apply an unnamed label — the label NAME is the load-bearing part")
		}
		adding[l.Name] = true
	}

	// 1. Ensure each label exists on the repo (422 = already present = success).
	for _, l := range change.Add {
		body := map[string]any{"name": l.Name, "color": l.Color, "description": l.Description}
		err := g.doJSON(http.MethodPost, fmt.Sprintf("/repos/%s/%s/labels", repo.Owner, repo.Name), body, nil)
		if err != nil {
			var ae *ForgeAPIError
			if errors.As(err, &ae) && ae.Status == http.StatusUnprocessableEntity {
				continue
			}
			return nil, err
		}
	}

	// 2. Work out what to take off: the explicit Remove names, plus every current label in a
	//    named family that is not being re-applied.
	remove := map[string]bool{}
	for _, n := range change.Remove {
		remove[n] = true
	}
	if len(change.RemoveFamilies) > 0 {
		current, err := g.ghChangeLabels(repo, number)
		if err != nil {
			return nil, err
		}
		for _, cur := range current {
			if adding[cur] {
				continue
			}
			for _, fam := range change.RemoveFamilies {
				if fam != "" && strings.HasPrefix(cur, fam) {
					remove[cur] = true
					break
				}
			}
		}
	}
	for _, name := range sortedKeys(remove) {
		if adding[name] {
			// Naming a label in both halves is a caller bug, not an instruction to churn it.
			continue
		}
		path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%s",
			repo.Owner, repo.Name, number, url.PathEscape(name))
		if err := g.doJSON(http.MethodDelete, path, nil, nil); err != nil {
			if IsForgeNotFound(err) {
				continue // already absent — the success case for an idempotent removal
			}
			return nil, err
		}
		out.Removed = append(out.Removed, name)
	}

	// 3. Apply. GitHub's POST /issues/{n}/labels is additive over a SET, so re-applying a
	//    present label never duplicates it.
	if len(change.Add) > 0 {
		names := make([]string, 0, len(change.Add))
		for _, l := range change.Add {
			names = append(names, l.Name)
		}
		path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", repo.Owner, repo.Name, number)
		if err := g.doJSON(http.MethodPost, path, map[string]any{"labels": names}, nil); err != nil {
			return nil, err
		}
		out.Added = names
	}
	return out, nil
}

// ghChangeLabels lists the label names currently on a change (labels live on the ISSUE view
// of the number on GitHub, for both issues and pull requests).
func (g *GitHubForge) ghChangeLabels(repo ForgeRepo, number int) ([]string, error) {
	var all []string
	for page := 1; page <= forgeMaxFilePages; page++ {
		var chunk []ghLabelWire
		path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels?per_page=%d&page=%d",
			repo.Owner, repo.Name, number, forgeFilePerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, l := range chunk {
			all = append(all, l.Name)
		}
		if len(chunk) < forgeFilePerPage {
			break
		}
	}
	return all, nil
}

func (g *GitHubForge) RepoVisibility(repo ForgeRepo) (string, error) {
	var info struct {
		Visibility string `json:"visibility"`
	}
	path := fmt.Sprintf("/repos/%s/%s", repo.Owner, repo.Name)
	if err := g.doJSON(http.MethodGet, path, nil, &info); err != nil {
		return "", err
	}
	if info.Visibility == "" {
		return "", Unverifiable(fmt.Sprintf("repo %s has no .visibility field in API response", repo.Slug()), nil)
	}
	return info.Visibility, nil
}

// forgeSearchPerPage bounds the owner-wide open-change search in one page; a read that comes
// back exactly full is reported as possibly-truncated (ChangeSearchResults.TruncatedAtCap).
const forgeSearchPerPage = 200

// ghCommitWire is the /commits and /commits/{sha} read shape (only the fields consumed). The
// top-level author/committer are the GitHub ACCOUNTS GitHub resolved the commit to (an identity
// comparable to a change author's login); commit.committer.date is the committed date.
type ghCommitWire struct {
	SHA    string `json:"sha"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	Committer *struct {
		Login string `json:"login"`
	} `json:"committer"`
	Commit struct {
		Committer struct {
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func (w ghCommitWire) toRepoCommit() RepoCommit {
	rc := RepoCommit{SHA: w.SHA, CommittedDate: w.Commit.Committer.Date}
	if w.Author != nil {
		rc.AuthorLogin = w.Author.Login
	}
	if w.Committer != nil {
		rc.CommitterLogin = w.Committer.Login
	}
	return rc
}

// ListRecentCommits reads up to limit commits from the head of the default branch. Omitting
// ?sha= makes GitHub use the default branch, so this needs no separate default-branch read. An
// empty repository answers 409 Conflict ("Git Repository is empty."), which is translated HERE
// into the backend-neutral ErrForgeEmptyRepo sentinel the caller tests with IsForgeEmptyRepo.
// EVERY other status stays a read failure the caller surfaces as could-not-check — a GitHub 404
// (repo gone/renamed, or token access lost) is NOT empty and must never be folded into it.
func (g *GitHubForge) ListRecentCommits(repo ForgeRepo, limit int) ([]RepoCommit, error) {
	if limit <= 0 {
		return nil, Unverifiable("ListRecentCommits needs a positive limit", nil)
	}
	var chunk []ghCommitWire
	path := fmt.Sprintf("/repos/%s/%s/commits?per_page=%d", repo.Owner, repo.Name, limit)
	if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
		var ae *ForgeAPIError
		if errors.As(err, &ae) && ae.Status == http.StatusConflict {
			return nil, fmt.Errorf("%s: %w", ae.Error(), ErrForgeEmptyRepo)
		}
		return nil, err
	}
	out := make([]RepoCommit, 0, len(chunk))
	for _, c := range chunk {
		out = append(out, c.toRepoCommit())
	}
	return out, nil
}

// GetCommit reads one commit's committed date and attributed author/committer accounts.
func (g *GitHubForge) GetCommit(repo ForgeRepo, sha string) (*RepoCommit, error) {
	if strings.TrimSpace(sha) == "" {
		return nil, Unverifiable("GetCommit needs a non-empty sha for "+repo.Slug(), nil)
	}
	var w ghCommitWire
	path := fmt.Sprintf("/repos/%s/%s/commits/%s", repo.Owner, repo.Name, sha)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	rc := w.toRepoCommit()
	return &rc, nil
}

// ghCompareWire is the compare-API read shape (only the fields consumed).
type ghCompareWire struct {
	Status   string       `json:"status"` // identical | ahead | behind | diverged
	AheadBy  int          `json:"ahead_by"`
	BehindBy int          `json:"behind_by"`
	Files    []ghFileWire `json:"files"`
}

// CompareRefs compares base...head via the compare API, returning the differing files plus the
// divergence counts and GitHub's own status word.
func (g *GitHubForge) CompareRefs(repo ForgeRepo, base, head string) (*RefComparison, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(head) == "" {
		return nil, Unverifiable("CompareRefs needs both base and head for "+repo.Slug(), nil)
	}
	var w ghCompareWire
	path := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", repo.Owner, repo.Name, base, head)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	out := &RefComparison{Status: w.Status, AheadBy: w.AheadBy, BehindBy: w.BehindBy}
	for _, f := range w.Files {
		out.Files = append(out.Files, ChangedFile{
			Filename: f.Filename, PreviousFilename: f.PreviousFilename, Status: f.Status,
		})
	}
	return out, nil
}

// ghSearchWire is the /search/issues read shape (only the fields consumed). The search API
// serves issues and PRs from one endpoint; a `is:pr is:open` query returns only changes.
type ghSearchWire struct {
	TotalCount        int  `json:"total_count"`
	IncompleteResults bool `json:"incomplete_results"`
	Items             []struct {
		Number        int    `json:"number"`
		Title         string `json:"title"`
		CreatedAt     string `json:"created_at"`
		RepositoryURL string `json:"repository_url"` // .../repos/{owner}/{name}
	} `json:"items"`
}

// ghIssueSearchWire is the issue-dedupe search shape (SearchIssues): the /search/issues items
// carrying the state, labels and html_url the dedupe scores and reports — the fields the
// owner-wide change search (ghSearchWire) does not read.
type ghIssueSearchWire struct {
	Items []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		HTMLURL string `json:"html_url"`
	} `json:"items"`
}

// SearchOpenChanges finds every open change under one owner via the search API
// (`is:pr is:open user:<owner>`), the typed form of `gh search prs --owner`. The repo each row
// belongs to is recovered from repository_url's trailing owner/name.
func (g *GitHubForge) SearchOpenChanges(owner string) (*ChangeSearchResults, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, Unverifiable("SearchOpenChanges needs a non-empty owner", nil)
	}
	q := fmt.Sprintf("is:pr is:open user:%s", owner)
	var w ghSearchWire
	path := fmt.Sprintf("/search/issues?q=%s&per_page=%d", url.QueryEscape(q), forgeSearchPerPage)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	out := &ChangeSearchResults{Cap: forgeSearchPerPage, TruncatedAtCap: len(w.Items) >= forgeSearchPerPage}
	for _, it := range w.Items {
		slug := ""
		if i := strings.Index(it.RepositoryURL, "/repos/"); i >= 0 {
			slug = it.RepositoryURL[i+len("/repos/"):]
		}
		out.Results = append(out.Results, ChangeSearchResult{
			Repo: slug, Number: it.Number, Title: it.Title, CreatedAt: it.CreatedAt,
		})
	}
	return out, nil
}

// ListWorkflowFiles lists the .yml/.yaml workflow file names under .github/workflows at ref
// (GET-only contents API). A repo with no such directory answers 404, surfaced as a
// *ForgeAPIError the caller tests with IsForgeNotFound.
func (g *GitHubForge) ListWorkflowFiles(repo ForgeRepo, ref string) ([]string, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, Unverifiable("ListWorkflowFiles needs a ref for "+repo.Slug(), nil)
	}
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	path := fmt.Sprintf("/repos/%s/%s/contents/.github/workflows?ref=%s",
		repo.Owner, repo.Name, url.QueryEscape(ref))
	if err := g.doJSON(http.MethodGet, path, nil, &entries); err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		if strings.HasSuffix(e.Name, ".yml") || strings.HasSuffix(e.Name, ".yaml") {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// ChangeDiff returns a change's raw unified diff text via the pulls endpoint with the diff
// media type (the REST equivalent of `gh pr diff`). go-gh's RESTClient hardcodes a JSON Accept
// header and exposes no hook to change it, so the diff media type is requested on a request
// this backend builds itself — still inside the forge implementation (the "no API construction
// outside the backend" contract), on the SAME transport and token restClient uses, so no
// second auth path or ambient-identity fallback is introduced.
func (g *GitHubForge) ChangeDiff(repo ForgeRepo, number int) (string, error) {
	if g.Token == "" {
		return "", Unverifiable("refusing to reach the GitHub forge without an explicitly minted token — "+
			"the go-gh backend never falls back to an ambient gh-CLI keyring/config identity", nil)
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", repo.Owner, repo.Name, number)
	req, rerr := http.NewRequest(http.MethodGet, g.baseURL()+path, nil)
	if rerr != nil {
		return "", Unverifiable("cannot build diff request for "+repo.Slug(), rerr)
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	transport := http.DefaultTransport
	if g.Client != nil && g.Client.Transport != nil {
		transport = g.Client.Transport
	}
	resp, derr := (&http.Client{Transport: transport}).Do(req)
	if derr != nil {
		return "", Unverifiable(fmt.Sprintf("GET %s (diff) failed", path), derr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &ForgeAPIError{Status: resp.StatusCode, Method: http.MethodGet, Path: path}
	}
	return string(raw), nil
}

// --- Writes ---

// CreateDraftChange opens a draft pull request (the REST equivalent of `gh pr create
// --draft`, which deskpr runs today — see the inventory delta). draft:true is the frozen
// property: this seam opens changes as drafts, never ready.
func (g *GitHubForge) CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", repo.Owner, repo.Name)
	body := map[string]any{
		"title": in.Title,
		"body":  in.Body,
		"head":  in.Head,
		"base":  in.Base,
		"draft": true,
	}
	var w ghPullWire
	if err := g.doJSON(http.MethodPost, path, body, &w); err != nil {
		return nil, err
	}
	return &PullRef{Number: w.Number, NodeID: w.NodeID, URL: w.HTMLURL}, nil
}

// ghCommentWire is the issue-comment rendering. `node_id` is REST's spelling of the same
// GraphQL global id ListComments reports and EditComment takes, so a comment posted here can
// be edited later without a second read.
type ghCommentWire struct {
	ID      int64  `json:"id"`
	NodeID  string `json:"node_id"`
	HTMLURL string `json:"html_url"`
}

func (g *GitHubForge) PostComment(repo ForgeRepo, number int, body string) (*CommentRef, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", repo.Owner, repo.Name, number)
	var w ghCommentWire
	if err := g.doJSON(http.MethodPost, path, map[string]any{"body": body}, &w); err != nil {
		return nil, err
	}
	return &CommentRef{ID: w.NodeID, DatabaseID: w.ID, URL: w.HTMLURL}, nil
}

// PostCommentTyped is PostComment on GitHub: issues and pull requests share ONE comments
// endpoint (`/issues/{n}/comments` serves both), so the stated kind selects nothing here.
// It is not re-validated against the object either — the caller's preceding GetIssueTyped
// is where a kind mismatch is caught, and a second read per comment would double the
// footprint of every attach for no new information. An unknown kind is still refused.
func (g *GitHubForge) PostCommentTyped(repo ForgeRepo, number int, kind TargetKind, body string) (*CommentRef, error) {
	switch kind {
	case TargetIssue, TargetChange:
	default:
		return nil, Refused(fmt.Sprintf("refused: PostCommentTyped: unknown target kind %q for %s#%d", string(kind), repo.Slug(), number))
	}
	return g.PostComment(repo, number, body)
}

func (g *GitHubForge) PostReview(repo ForgeRepo, number int, in ReviewInput) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", repo.Owner, repo.Name, number)
	body := map[string]any{"commit_id": in.HeadSHA, "event": in.Event, "body": in.Body}
	return g.doJSON(http.MethodPost, path, body, nil)
}

// MarkReadyForReview flips a draft change to ready via the GraphQL mutation (the only
// GitHub API for the transition; `gh pr ready` uses the same). It is issued over the SAME
// authenticated go-gh client as the REST operations, POSTing the mutation to an absolute
// /graphql URL. go-gh's dedicated GraphQLClient targets a host-derived https endpoint it
// gives no absolute-URL override for, so it cannot be pointed at the golden corpus's httptest
// server without changing the observed request; routing the one mutation through the REST
// client keeps the wire byte-identical to what the corpus pins while still binding auth to
// the minted token.
func (g *GitHubForge) MarkReadyForReview(nodeID string) error {
	q := `mutation($id:ID!){markPullRequestReadyForReview(input:{pullRequestId:$id}){pullRequest{isDraft}}}`
	in := map[string]any{"query": q, "variables": map[string]any{"id": nodeID}}
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return Unverifiable("markPullRequestReadyForReview GraphQL error: "+out.Errors[0].Message, nil)
	}
	return nil
}

// FileIssue files a new issue (the REST equivalent of `gh issue create`, which deskfile
// runs today — see the inventory delta).
func (g *GitHubForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues", repo.Owner, repo.Name)
	var w ghIssueWire
	if err := g.doJSON(http.MethodPost, path, map[string]any{"title": in.Title, "body": in.Body}, &w); err != nil {
		return nil, err
	}
	return &IssueRef{Number: w.Number, URL: w.HTMLURL}, nil
}

// CloseIssue closes an issue (the REST equivalent of `gh issue close`, which deskclose/
// deskfile run today — see the inventory delta). A stateReason of "" omits the field.
func (g *GitHubForge) CloseIssue(repo ForgeRepo, number int, stateReason string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", repo.Owner, repo.Name, number)
	body := map[string]any{"state": "closed"}
	if stateReason != "" {
		body["state_reason"] = stateReason
	}
	return g.doJSON(http.MethodPatch, path, body, nil)
}

// CloseIssueTyped closes the object of the STATED kind. On GitHub issues and pull requests
// share ONE number sequence and one state endpoint (`PATCH /issues/{n}` closes either), so
// the kind selects no different request here — what it does is make the caller's intent
// explicit at the seam, so the same call site works unchanged on a forge where the two kinds
// are separate sequences. A state reason on a CHANGE is refused: GitHub records `state_reason`
// on issues only, and accepting one on a pull request would drop it silently.
func (g *GitHubForge) CloseIssueTyped(repo ForgeRepo, number int, kind TargetKind, stateReason string) error {
	if err := requireNoReasonOnChange(repo, number, kind, stateReason); err != nil {
		return err
	}
	return g.CloseIssue(repo, number, stateReason)
}

// ReopenIssue reopens an issue (`PATCH /repos/{o}/{r}/issues/{n}` with `state: open`). No
// state reason travels: GitHub records one at close time only, and reopening clears it.
func (g *GitHubForge) ReopenIssue(repo ForgeRepo, number int) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", repo.Owner, repo.Name, number)
	return g.doJSON(http.MethodPatch, path, map[string]any{"state": "open"}, nil)
}

// EditChange replaces a change's OWN title/body (`PATCH /repos/{o}/{r}/pulls/{n}`) — the change
// description, not a comment. An empty field is not sent, so a body-only edit does not blank the
// title (deskpr edit's case) and vice versa; asking to change NEITHER is a could-not-check
// refusal rather than an empty PATCH.
func (g *GitHubForge) EditChange(repo ForgeRepo, number int, in EditChangeInput) error {
	body := map[string]any{}
	if in.Title != "" {
		body["title"] = in.Title
	}
	if in.Body != "" {
		body["body"] = in.Body
	}
	if len(body) == 0 {
		return Unverifiable(fmt.Sprintf(
			"could-not-check: EditChange was asked to change neither the title nor the body of %s#%d — "+
				"nothing to write", repo.Slug(), number), nil)
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", repo.Owner, repo.Name, number)
	return g.doJSON(http.MethodPatch, path, body, nil)
}

// DeleteRef deletes one git ref (the typed replacement for the `gh api -X DELETE
// repos/<o>/<r>/git/refs/<ref>` passthrough fanoutloop used to release a dispatch claim).
// The ref is validated by ValidateRefPath first, so the only thing this op can address is a
// ref inside the named repo — the arbitrary-endpoint reach of the call it replaces is gone,
// not renamed.
//
// A missing ref surfaces as a *ForgeAPIError the caller can test with IsForgeNotFound; the
// seam does not decide that "already gone" is success, because for a claim release it is and
// for a tag retraction it is not.
func (g *GitHubForge) DeleteRef(repo ForgeRepo, ref string) error {
	clean, err := ValidateRefPath(ref)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/git/refs/%s", repo.Owner, repo.Name, clean)
	return g.doJSON(http.MethodDelete, path, nil, nil)
}

// RefExists reports whether one git ref is present, via the single-reference read
// (`GET /repos/{o}/{r}/git/ref/{ref}` — SINGULAR `ref`, the exact endpoint that returns one
// reference, distinct from the plural `git/refs/` DeleteRef targets). This is the logic that
// was cmd/deskpost's hand-rolled `refExists`, moved onto the seam so a second forge implements
// the ref-existence read rather than a second tool forking its own.
//
// The ref is validated by ValidateRefPath first, so the one path-shaped argument can only
// address a ref inside the named repo — the arbitrary-endpoint bound DeleteRef carries. A 404
// is the ANSWER "absent" (false, nil), not a failure — that is the whole point of the read;
// every other non-2xx stays an error so a 403 from a token that cannot see refs can never be
// mistaken for "the ref is gone".
func (g *GitHubForge) RefExists(repo ForgeRepo, ref string) (bool, error) {
	clean, err := ValidateRefPath(ref)
	if err != nil {
		return false, err
	}
	path := fmt.Sprintf("/repos/%s/%s/git/ref/%s", repo.Owner, repo.Name, clean)
	if err := g.doJSON(http.MethodGet, path, nil, nil); err != nil {
		if IsForgeNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// MatchingRefs lists the refs whose path starts with refPrefix, via the matching-refs read
// (`GET /repos/{o}/{r}/git/matching-refs/{ref}` — the endpoint that returns EVERY reference
// beginning with the given path, distinct from the singular `git/ref/` RefExists targets).
//
// refPrefix is validated by ValidateRefPath first, so the one path-shaped argument can only
// address refs inside the named repo. An empty match is the endpoint's own 200 `[]` (returned
// as (nil, nil), the ANSWER "no such refs"); a 404 is likewise no-match, not a failure. Every
// other non-2xx stays an error, so a 403 from a token that cannot see refs is never mistaken
// for "no refs". The returned slice carries each match's FULLY-QUALIFIED ref (e.g.
// "refs/dispatch/<key>"), verbatim from the `ref` field.
func (g *GitHubForge) MatchingRefs(repo ForgeRepo, refPrefix string) ([]string, error) {
	clean, err := ValidateRefPath(refPrefix)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/git/matching-refs/%s", repo.Owner, repo.Name, clean)
	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := g.doJSON(http.MethodGet, path, nil, &refs); err != nil {
		if IsForgeNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.Ref != "" {
			out = append(out, r.Ref)
		}
	}
	return out, nil
}

// --- Repo-hardening reads (op 40) ---

// hardeningGithubPaths maps every kind but `rulesets` (a two-hop, handled separately) to its
// ONE fixed endpoint literal. There is exactly one literal per kind — no caller-supplied
// segment — so the map itself is the proof this is not a passthrough in a different shape.
var hardeningGithubPaths = map[HardeningReadKind]string{
	HardeningReadRepo:                       "/repos/%s/%s",
	HardeningReadActionsWorkflowPermissions: "/repos/%s/%s/actions/permissions/workflow",
	HardeningReadActionsForkPRApproval:      "/repos/%s/%s/actions/permissions/fork-pr-contributor-approval",
	HardeningReadActionsPrivateForkPR:       "/repos/%s/%s/actions/permissions/fork-pr-workflows-private-repos",
	HardeningReadVulnerabilityReporting:     "/repos/%s/%s/private-vulnerability-reporting",
}

// RepoHardeningRead implements op 40 on GitHub: kind is validated against the closed
// vocabulary before any request exists, so an unknown kind emits ZERO requests, and a
// GitLab kind (`project`, `protected-branches`, …) is refused BY NAME with zero requests —
// the symmetric twin of the GitLab backend's refusal of the GitHub kinds. Every kind
// but `rulesets` is one fixed GET; `rulesets` performs the list→detail walk and returns the
// ARRAY of detail documents (hardeningRulesets).
func (g *GitHubForge) RepoHardeningRead(repo ForgeRepo, kind HardeningReadKind) (json.RawMessage, error) {
	if _, err := ValidateHardeningReadKind(string(kind)); err != nil {
		return nil, err
	}
	if err := refuseHardeningKindForForge(ForgeGitHub, kind); err != nil {
		return nil, err
	}
	if kind == HardeningReadRulesets {
		return g.hardeningRulesets(repo)
	}
	tmpl, ok := hardeningGithubPaths[kind]
	if !ok {
		// Unreachable: ValidateHardeningReadKind above already refused anything not in
		// hardeningReadKinds, and every entry of that slice is handled here or above. Kept as
		// could-not-check, never a panic — a resolver that cannot name a mapping fails closed.
		return nil, Unverifiable(fmt.Sprintf(
			"RepoHardeningRead: kind %q passed validation but has no GitHub path mapping", kind), nil)
	}
	return g.hardeningGET(fmt.Sprintf(tmpl, repo.Owner, repo.Name))
}

// hardeningGET performs one GET and returns the raw response body unparsed — op 40 hands the
// document back as-is so the CALLER'S OWN field selector (repohardenguard's checklist, never
// this package) decides what inside it matters.
func (g *GitHubForge) hardeningGET(path string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := g.doJSON(http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// hardeningRulesets performs the ONE two-hop kind: GET the ruleset list (which deliberately
// omits `rules`/`bypass_actors`), then GET each entry's own detail by id, and returns the
// ARRAY of detail documents. The hop lives here, once, rather than being repeated by every
// caller that wants a named ruleset's field — repohardenguard's `[name=X].field` selector
// resolves inside the returned array.
func (g *GitHubForge) hardeningRulesets(repo ForgeRepo) (json.RawMessage, error) {
	listPath := fmt.Sprintf("/repos/%s/%s/rulesets", repo.Owner, repo.Name)
	var list []struct {
		ID int64 `json:"id"`
	}
	if err := g.doJSON(http.MethodGet, listPath, nil, &list); err != nil {
		return nil, err
	}
	details := make([]json.RawMessage, 0, len(list))
	for _, rs := range list {
		detailPath := fmt.Sprintf("/repos/%s/%s/rulesets/%d", repo.Owner, repo.Name, rs.ID)
		var d json.RawMessage
		if err := g.doJSON(http.MethodGet, detailPath, nil, &d); err != nil {
			return nil, err
		}
		details = append(details, d)
	}
	return json.Marshal(details)
}

// --- Merge-hold marker thread (the forge-gitlab merge-hold brief) ---
//
// GitHub's server-side twin of this control is branch protection's required reviewer-App
// review, already stronger than a discussion-thread hold — so every op here is a typed
// not-applicable, and none issues a request.

// ReadMergeHold returns MergeHoldNotApplicable — GitHub's gate is branch protection, not a
// discussion thread.
func (g *GitHubForge) ReadMergeHold(repo ForgeRepo, number int) (*MergeHold, error) {
	return &MergeHold{State: MergeHoldNotApplicable}, nil
}

// OpenMergeHold returns ErrMergeHoldNotApplicable — there is no hold to open on GitHub.
func (g *GitHubForge) OpenMergeHold(repo ForgeRepo, number int) (string, error) {
	return "", ErrMergeHoldNotApplicable
}

// SetMergeHold returns ErrMergeHoldNotApplicable — there is no hold to release or re-arm on
// GitHub.
func (g *GitHubForge) SetMergeHold(repo ForgeRepo, number int, in MergeHoldUpdate) error {
	return ErrMergeHoldNotApplicable
}

// --- File content (read / write on a branch) ---

// ghContentsWire is the Contents-API read shape (only the fields consumed). `content` is
// base64 with the API's own 60-column line wrapping, stripped before decoding.
type ghContentsWire struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// ghContentsCommitWire is the Contents-API write response: the new blob sha under `content`
// and the git author the created commit carries under `commit.author` (for an App-token write,
// the App's bot — the identity an Evidence commit is supposed to have).
type ghContentsCommitWire struct {
	Content struct {
		SHA string `json:"sha"`
	} `json:"content"`
	Commit struct {
		SHA    string `json:"sha"`
		Author struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commit"`
}

// stripBase64Whitespace removes the newlines GitHub wraps base64 content at (every 60 cols),
// which StdEncoding.DecodeString does not tolerate.
func stripBase64Whitespace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', ' ', '\t':
			return -1
		}
		return r
	}, s)
}

// ReadFile reads a file's content at a ref via the Contents API. A 404 propagates as a
// *ForgeAPIError (IsForgeNotFound true) so a caller can distinguish "absent" from "unreadable".
func (g *GitHubForge) ReadFile(repo ForgeRepo, in ReadFileInput) (*FileContent, error) {
	path := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s",
		repo.Owner, repo.Name, in.File, url.QueryEscape(in.Ref))
	var w ghContentsWire
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	if w.SHA == "" {
		return nil, Unverifiable(fmt.Sprintf("empty sha in contents response for %s@%s", in.File, in.Ref), nil)
	}
	decoded, derr := base64.StdEncoding.DecodeString(stripBase64Whitespace(w.Content))
	if derr != nil {
		return nil, Unverifiable("cannot decode base64 content for "+in.File, derr)
	}
	return &FileContent{Content: decoded, SHA: w.SHA, Exists: true}, nil
}

// WriteFile writes a file's whole content on a branch via the Contents API, as the minted
// (App) identity. GitHub's default branch is directly writable by the verifier App — the
// direct-main carve-out — so this backend never returns the DefaultBranchNotWritable sentinel
// and ignores StartBranch: on GitHub the Evidence row lands on the target branch directly. The
// idempotency read and the append-only shrink guard are folded in per WriteFileInput.
func (g *GitHubForge) WriteFile(repo ForgeRepo, in WriteFileInput) (*WriteFileResult, error) {
	res := &WriteFileResult{}
	var priorSHA string
	var priorContent []byte
	exists := false
	cur, rerr := g.ReadFile(repo, ReadFileInput{File: in.File, Ref: in.Branch})
	if rerr != nil {
		if !IsForgeNotFound(rerr) {
			return nil, rerr
		}
		// Absent on the branch → this is a create.
	} else {
		priorSHA, priorContent, exists = cur.SHA, cur.Content, true
	}

	if in.AppendOnly {
		res.PriorRows = forgeRowCount(priorContent)
		res.Rows = forgeRowCount(in.Content)
	}

	// Idempotency: byte-identical content already on the branch is a noop, no write.
	if exists && bytes.Equal(priorContent, in.Content) {
		res.SHA = priorSHA
		return res, nil
	}

	// Append-only shrink guard, refused post-fetch (see WriteFileInput.AppendOnly).
	if in.AppendOnly && !in.AllowShrink && exists && res.Rows < res.PriorRows {
		return nil, Refused(fmt.Sprintf(
			"refusing an append-only write to %s on %s that would SHRINK it from %d to %d row(s) — "+
				"almost always a stale-base or wrong-file write; pass AllowShrink when the reduction is intended",
			in.File, in.Branch, res.PriorRows, res.Rows))
	}

	body := map[string]any{
		"message": in.Message,
		"content": base64.StdEncoding.EncodeToString(in.Content),
		"branch":  in.Branch,
	}
	if priorSHA != "" {
		body["sha"] = priorSHA
	}
	var w ghContentsCommitWire
	path := fmt.Sprintf("/repos/%s/%s/contents/%s", repo.Owner, repo.Name, in.File)
	if err := g.doJSON(http.MethodPut, path, body, &w); err != nil {
		return nil, err
	}
	res.Changed = true
	res.SHA = w.Content.SHA
	res.Author = w.Commit.Author.Name
	return res, nil
}

// --- Identity / transport ---

// PushTransportHint returns GitHub's push-transport shape: an App installation token
// authenticates an https push as the username "x-access-token", supplied via an inline
// credential.helper reading the token file — never a token-in-URL (classifier-blocked, and
// it leaks the secret into argv/reflog). Pure function, no network call.
func (g *GitHubForge) PushTransportHint(repo ForgeRepo) PushTransport {
	return PushTransport{
		RemoteHost:    "github.com",
		TokenUsername: "x-access-token",
		CredentialHelperHint: "supply the token via an inline credential.helper that reads the 0600 token " +
			"file; never embed it in the remote URL",
	}
}
