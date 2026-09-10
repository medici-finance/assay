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
type ForgeAPIError struct {
	Status int
	Method string
	Path   string
}

func (e *ForgeAPIError) Error() string {
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
	rc, err := g.restClient()
	if err != nil {
		return err
	}
	var bodyReader io.Reader
	if in != nil {
		b, merr := json.Marshal(in)
		if merr != nil {
			return Unverifiable("cannot marshal request body", merr)
		}
		bodyReader = bytes.NewReader(b)
	}
	resp, rerr := rc.Request(method, g.baseURL()+path, bodyReader)
	if rerr != nil {
		var he *ghapi.HTTPError
		if errors.As(rerr, &he) {
			return &ForgeAPIError{Status: he.StatusCode, Method: method, Path: path}
		}
		return Unverifiable(fmt.Sprintf("%s %s failed", method, path), rerr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, out); uerr != nil {
			return Unverifiable(fmt.Sprintf("cannot parse %s %s response", method, path), uerr)
		}
	}
	return nil
}

// --- REST wire shapes (only the fields consumed) ---

type ghPullWire struct {
	Number       int    `json:"number"`
	State        string `json:"state"`
	Draft        bool   `json:"draft"`
	NodeID       string `json:"node_id"`
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
	User   struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
	HTMLURL string `json:"html_url"`
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
	labels := make([]string, 0, len(w.Labels))
	for _, l := range w.Labels {
		labels = append(labels, l.Name)
	}
	return &PullRequest{
		Number:       w.Number,
		State:        w.State,
		Draft:        w.Draft,
		NodeID:       w.NodeID,
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
	}, nil
}

func (g *GitHubForge) GetIssue(repo ForgeRepo, number int) (*Issue, error) {
	var w ghIssueWire
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", repo.Owner, repo.Name, number)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	return &Issue{
		Number:        w.Number,
		Title:         w.Title,
		State:         w.State,
		Author:        Account{Login: w.User.Login, ID: w.User.ID},
		IsPullRequest: w.PullRequest != nil,
	}, nil
}

// forgeIssuePerPage / forgeOpenChangesCap bound the two bulk board reads. The issue read
// paginates to exhaustion (per_page=100); the open-change read is a SINGLE bounded page
// whose cap is reported (OpenChanges.Cap) so a read that came back exactly full signals a
// possibly-truncated population rather than a confident count over an unknown remainder.
const (
	forgeIssuePerPage   = 100
	forgeOpenChangesCap = 100
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

func (g *GitHubForge) ListOpenIssues(repo ForgeRepo) ([]IssueSummary, error) {
	var out []IssueSummary
	for page := 1; ; page++ {
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
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
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
		if len(chunk) < forgeIssuePerPage {
			break
		}
	}
	return out, nil
}

func (g *GitHubForge) PRTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	return g.trustEvents(repo, number, PRTrustQuery, true)
}

func (g *GitHubForge) IssueTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	return g.trustEvents(repo, number, IssueTrustQuery, false)
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
// App token. Instead the read re-resolves through two endpoints the same token CAN read (see
// requiredChecksAdminFree), and only if BOTH of those also fail does it stay could-not-check.
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
//     nothing gates the merge on a check, so the required set is empty (⇒ green).
//  2. GET /repos/{o}/{r}/rules/branches/{b} → the EFFECTIVE rules (classic protection AND
//     rulesets); the union of every `required_status_checks` rule's contexts is the required
//     set. This also closes the ruleset gap the legacy endpoint never covered.
//
// The rules endpoint is consulted whenever the branch is (or may be) protected — i.e. when
// step 1 reports protected, OR when step 1 itself could not be read (a 403/5xx there does not
// prove the branch unprotected, so it is not read as empty). Only when BOTH admin-free reads
// fail is the result could-not-check; legacyErr is threaded into that refusal for context.
func (g *GitHubForge) requiredChecksAdminFree(repo ForgeRepo, branch string, legacyErr error) ([]string, error) {
	var bp ghBranchProtectedWire
	bpath := fmt.Sprintf("/repos/%s/%s/branches/%s", repo.Owner, repo.Name, url.PathEscape(branch))
	brErr := g.doJSON(http.MethodGet, bpath, nil, &bp)
	if brErr == nil && !bp.Protected {
		// The branch is not protected: nothing GitHub enforces gates the merge on a check, so
		// the required set is empty. No need to read the rules.
		return nil, nil
	}
	// The branch is protected, or its protection flag could not be read. In both cases any
	// required contexts live in the effective-rules endpoint, which the same token can read.
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
	return dedupContexts(ctxs), nil
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
          author { login __typename }
        }
      }
    }
  }
}`

func (g *GitHubForge) ListComments(repo ForgeRepo, number int) ([]Comment, error) {
	in := map[string]any{
		"query": ghCommentsQuery,
		"variables": map[string]any{
			"owner": repo.Owner, "name": repo.Name, "number": number,
		},
	}
	var out struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					Comments struct {
						Nodes []struct {
							ID          string `json:"id"`
							DatabaseID  int64  `json:"databaseId"`
							Body        string `json:"body"`
							IsMinimized bool   `json:"isMinimized"`
							CreatedAt   string `json:"createdAt"`
							URL         string `json:"url"`
							Author      struct {
								Login    string `json:"login"`
								Typename string `json:"__typename"`
							} `json:"author"`
						} `json:"nodes"`
					} `json:"comments"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
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
	nodes := out.Data.Repository.PullRequest.Comments.Nodes
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
			Author:     Account{Login: login},
			Body:       n.Body,
			Minimized:  n.IsMinimized,
			CreatedAt:  n.CreatedAt,
			URL:        n.URL,
		})
	}
	return res, nil
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
func (g *GitHubForge) ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error) {
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
