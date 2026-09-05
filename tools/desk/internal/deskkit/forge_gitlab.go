package deskkit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// forge_gitlab.go — the GitLab implementation of Forge, seated on the OFFICIAL GitLab Go
// client (gitlab.com/gitlab-org/api/client-go, the successor to xanzy/go-gitlab and the
// client GitLab itself maintains and ships `glab` on top of) rather than a hand-rolled
// net/http stack or a shelled `glab` binary. The GitHub backend is seated the same way on
// go-gh (forge_github.go), so both forges are library-behind-one-interface and neither
// requires a vendor CLI on PATH.
//
// WHAT MAKES THIS FILE HARD is not the transport — it is the CONCEPT MAPPING. GitHub and
// GitLab agree on the shape of "a change with reviews and checks" and disagree on almost
// every detail underneath it. The stream spec (§6) fixes the mapping; the golden corpus in
// forge_gitlab_test.go pins it per operation so a plausible-but-wrong mapping fails a
// fixture rather than a pilot. The mappings that are NOT one-to-one, and what this backend
// does about each, are documented at their method — read those comments before changing a
// method, because several of them are fail-closed decisions, not conveniences:
//
//   - GetIssue: GitHub serves issues and PRs from ONE number sequence; GitLab keeps two.
//     A bare number is therefore AMBIGUOUS on GitLab, and this backend probes both and
//     REFUSES when both resolve (rather than silently picking one).
//   - ReviewsAtHead: a GitLab approval carries no per-approval SHA or timestamp, so
//     "at head" has to be established from other facts (approval-reset policy, diff-version
//     timestamps) and is left UNSET — never guessed — where it cannot be established.
//   - IssueReactions: the award-emoji payload carries no bot/human discriminator, and the
//     house admission gate (repovis.go) requires one. This backend resolves it from the
//     users API rather than defaulting to "User", which would silently let a bot's 👍
//     satisfy a gate that on GitHub only a human can satisfy.
//   - PullRequest.ChangedFiles: GitLab reports a TRUNCATABLE count ("1000+"); a truncated
//     count is reported so the caller's reconciliation can never be satisfied.
//
// Auth: a personal/group/project access token over the `PRIVATE-TOKEN` header
// (gitlab.AccessTokenAuthSource). The token VALUE is injected — minting, rotation and file
// custody are the identity layer (spec §2/§5), deliberately outside this seam. An
// empty token is REFUSED rather than resolved from any ambient source, mirroring the GitHub
// backend's posture.

// GitLabAPIBase is the single home of the GitLab host literal this backend defaults to.
// It lives here rather than in forge.go (which holds the FROZEN interface and its shared
// types) so the interface file stays backend-agnostic; GitHubAPIBase predates that split.
//
// It is the INSTANCE root, not the API root: the client library appends `api/v4/` itself,
// and a self-managed EE instance is configured by pointing BaseURL at its own root (the
// profile's normal case — spec §1 targets self-managed EE as much as gitlab.com).
const GitLabAPIBase = "https://gitlab.com"

// GitLabForge implements Forge against GitLab REST v4. Same construction shape as
// GitHubForge: BaseURL defaults to GitLabAPIBase, Client defaults to the library's own
// transport, so a test points BaseURL at an httptest server and supplies its Client.
type GitLabForge struct {
	// Token is an already-minted access token (see the file header: this seam never mints).
	Token string
	// BaseURL is the GitLab instance root; empty means GitLabAPIBase.
	BaseURL string
	// Client, when non-nil, is the HTTP client the backend issues requests through.
	Client *http.Client

	// cl caches the client built from Token/BaseURL. Lazily constructed by client() so a
	// bare struct literal (the golden test's construction shape) still works.
	cl *gitlab.Client
}

var _ Forge = (*GitLabForge)(nil)

func (g *GitLabForge) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return GitLabAPIBase
}

// gitlabNoLimiter satisfies gitlab.RateLimiter without ever waiting. See client() for why
// the library's own adaptive limiter is displaced rather than left on: pacing belongs to the
// wrapper around a Forge call, not inside the implementation.
type gitlabNoLimiter struct{}

func (gitlabNoLimiter) Wait(_ context.Context) error { return nil }

// client returns the GitLab client for this forge, building it on first use.
//
// Three construction choices are load-bearing:
//
//  1. An empty Token is REFUSED here. The library would otherwise happily build a client
//     that sends an empty PRIVATE-TOKEN header and reads whatever the instance exposes
//     anonymously — a silent downgrade from "acts as the minted service account" to "acts
//     as nobody", which is exactly the unminted-identity hole the GitHub backend closes.
//  2. Retries are DISABLED (WithoutRetries). forge.go's contract puts budgets, rate
//     limiting and breakers OUTSIDE this interface — they wrap a Forge call, never live
//     inside an implementation. The library's default retryablehttp policy (5xx/429 with
//     exponential backoff) is precisely such a policy; leaving it on would put a second,
//     hidden retry budget inside the seam that the desk's own ratelimit.go cannot see or
//     account for. A transport failure is surfaced to the caller, which owns the retry.
//  3. The library's adaptive rate limiter is displaced by a no-op for the same reason: it
//     reconfigures itself from RateLimit-* response headers and then SLEEPS inside the
//     call. That is the wrapper's job.
func (g *GitLabForge) client() (*gitlab.Client, error) {
	if g.Token == "" {
		return nil, Unverifiable("refusing to reach the GitLab forge without an explicitly minted token — "+
			"this backend never falls back to an anonymous or ambient identity", nil)
	}
	if g.cl != nil {
		return g.cl, nil
	}
	opts := []gitlab.ClientOptionFunc{
		gitlab.WithBaseURL(g.baseURL()),
		gitlab.WithoutRetries(),
		gitlab.WithCustomLimiter(gitlabNoLimiter{}),
	}
	if g.Client != nil {
		opts = append(opts, gitlab.WithHTTPClient(g.Client))
	}
	cl, err := gitlab.NewClient(g.Token, opts...)
	if err != nil {
		return nil, Unverifiable("cannot build GitLab API client", err)
	}
	g.cl = cl
	return cl, nil
}

// project renders a ForgeRepo as the URL-encoded project path GitLab addresses projects by
// ("owner/name" → "owner%2Fname"). Returned as a plain string because the library escapes
// it itself; this value is only used for diagnostics paths.
func (g *GitLabForge) projectPath(repo ForgeRepo) string {
	return url.PathEscape(repo.Slug())
}

// --- Error mapping (the three-state surface) ---

// gitlabStatusReason renders WHY a status is a could-not-check, in the three tiers the
// brief names. The distinction matters operationally: a 401 says re-mint, a 403 says the
// role or the instance TIER does not expose this endpoint (approval rules and external
// status checks are Premium/Ultimate features — spec §3), and a 404 says the token cannot
// SEE the object. None of the three is an empty result.
func gitlabStatusReason(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "credential rejected (HTTP 401): the token is invalid, expired, or was rotated away"
	case http.StatusForbidden:
		return "permission or tier gate (HTTP 403): the token's role, or the instance's GitLab tier, " +
			"does not expose this endpoint"
	case http.StatusNotFound:
		return "not visible (HTTP 404): the object does not exist, or the token cannot see it"
	case http.StatusTooManyRequests:
		return "rate limited (HTTP 429) by the instance"
	default:
		return fmt.Sprintf("HTTP %d", status)
	}
}

// mapErr converts a library error into the forge's stable error shape.
//
// EVERY non-nil error becomes a could-not-check refusal (ExitUnverifiable) whose message
// carries the literal token `could-not-check`, wrapping a *ForgeAPIError so that status
// classification — and IsForgeNotFound, which both backends share — keeps working through
// errors.As. The rule this encodes is the brief's task 3 and clause C4: an instrument that
// could not look has not cleared anything, so a permission or tier failure must never reach
// a caller as an empty-but-nil-error result.
//
// gitlab.ErrNotFound is a SENTINEL — the library discards the response for 404s — so it is
// re-inflated to a 404 *ForgeAPIError here rather than falling into the statusless branch.
func (g *GitLabForge) mapErr(method, path string, err error) error {
	if err == nil {
		return nil
	}
	status := 0
	switch {
	case errors.Is(err, gitlab.ErrNotFound):
		status = http.StatusNotFound
	default:
		var er *gitlab.ErrorResponse
		if errors.As(err, &er) && er.Response != nil {
			status = er.Response.StatusCode
		}
	}
	if status == 0 {
		// No HTTP status ever came back (transport failure, marshal error, cancelled
		// context). Still a could-not-check, but with nothing to classify.
		return Unverifiable(fmt.Sprintf("could-not-check: %s %s did not complete: %v", method, path, err), err)
	}
	return Unverifiable(
		fmt.Sprintf("could-not-check: %s %s — %s", method, path, gitlabStatusReason(status)),
		&ForgeAPIError{Status: status, Method: method, Path: path},
	)
}

// --- Value mapping helpers ---

// gitlabState normalises GitLab's lifecycle vocabulary to the interface's `open | closed`.
//
// GitLab's states are opened / closed / merged / locked. The interface carries only the two
// GitHub states, so `merged` collapses to `closed` (a merged MR is not open) and `locked`
// — a transient state during a merge — collapses to `open`. Callers that need the merged
// distinction read it from the forge-specific layer, not from this seam.
func gitlabState(s string) string {
	switch s {
	case "opened", "locked":
		return "open"
	case "closed", "merged":
		return "closed"
	default:
		return s
	}
}

// gitlabAccount maps a GitLab user to the interface's Account. Login is the USERNAME (the
// handle, GitLab's analog of a GitHub login) and ID is the permanent numeric id — both are
// required by trust.go, which pins on the id precisely so a recycled username cannot
// inherit an authority.
func gitlabAccount(id int64, username string) Account {
	return Account{Login: username, ID: id}
}

// gitlabTime renders a GitLab timestamp in the RFC3339 shape the interface's string
// timestamps carry (GitHub's submitted_at). A nil timestamp renders empty — absent, not
// epoch-zero, which would read as a real 1970 verdict.
func gitlabTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// gitlabMergeableState maps GitLab's `detailed_merge_status` to the three-value
// PullRequest.Mergeable vocabulary.
//
// GitLab reports a DOZEN detailed statuses where GitHub reports a tri-state, and the mapping
// is deliberately narrow in both directions:
//
//   - only `mergeable` maps to MERGEABLE. Everything the forge has not positively cleared
//     stays out of that bucket.
//   - only the two statuses that describe the CHANGE ITSELF being un-mergeable —
//     `broken_status` (the source cannot be merged into the target) and `conflict` — map to
//     CONFLICTING, because CONFLICTING is what the consuming gate treats as a decided
//     refusal rather than a retry.
//   - EVERYTHING ELSE maps to UNKNOWN. That includes `checking`/`unchecked` (not computed
//     yet), the policy holds (`not_approved`, `blocked_status`, `discussions_not_resolved`,
//     `draft_status`, `ci_still_running`, …) and any status this table has never seen.
//
// The last clause is the load-bearing one. A GitLab release that adds a new detailed status
// must not fall into MERGEABLE by default, and it must not fall into CONFLICTING either — a
// policy hold is not a conflict, and reporting it as one would tell a caller the change needs
// rebasing when it needs an approval. UNKNOWN is the honest answer for a status this tree has
// not been taught, and the consuming gate reads UNKNOWN as could-not-check.
//
// An EMPTY detailed_merge_status (an older instance that does not send the field) is UNKNOWN
// for the same reason: absence is not a clearance.
func gitlabMergeableState(detailed string) string {
	switch strings.ToLower(strings.TrimSpace(detailed)) {
	case "mergeable":
		return Mergeable
	case "broken_status", "conflict":
		return MergeableConflicting
	default:
		return MergeableUnknown
	}
}

// gitlabChangedFileCount maps GitLab's `changes_count` to the interface's exact
// ChangedFiles int, FAILING CLOSED on truncation.
//
// GitHub reports an exact `changed_files`; GitLab reports a STRING that is either an exact
// decimal ("3") or a truncated lower bound ("1000+"). PullRequest.ChangedFiles exists so a
// caller can reconcile it against len(ListChangedFiles) and refuse to trust a short walk
// (forge.go: "the reconciliation partner for ListChangedFiles"). Reporting 1000 for "1000+"
// would let a walk that ALSO truncated at 1000 reconcile clean — a false pass on exactly
// the risk-path gate the count protects. Reporting 1001 instead makes the reconciliation
// unsatisfiable, so a truncated change set can only ever fail closed.
//
// An unparseable value yields 0 and ok=false, which the caller surfaces as could-not-check.
func gitlabChangedFileCount(changesCount string) (int, bool) {
	s := strings.TrimSpace(changesCount)
	if s == "" {
		return 0, false
	}
	if truncated := strings.HasSuffix(s, "+"); truncated {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "+"))
		if err != nil {
			return 0, false
		}
		return n + 1, true
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// gitlabDraftPrefix is the title prefix GitLab uses to mark a merge request as a draft. It
// is the ONLY draft mechanism GitLab has — there is no boolean to set — so opening a draft
// change means prefixing the title, and flipping ready means removing the prefix.
const gitlabDraftPrefix = "Draft: "

// gitlabDraftPrefixes are every prefix GitLab itself recognises as marking a draft,
// including the legacy `WIP:` spelling still honoured on older EE instances. Matching is
// case-insensitive because GitLab's own matcher is.
var gitlabDraftPrefixes = []string{"draft:", "wip:"}

// gitlabStripDraftPrefix removes a leading draft marker from an MR title, reporting whether
// one was present. Only ONE marker is stripped: a title that genuinely begins "Draft: Draft:
// …" is the author's, and silently eating both would rewrite their text.
func gitlabStripDraftPrefix(title string) (string, bool) {
	lower := strings.ToLower(title)
	for _, p := range gitlabDraftPrefixes {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(title[len(p):]), true
		}
	}
	return title, false
}

// --- Opaque change ids ---

// gitlabNodeIDScheme is the prefix of the opaque id this backend mints for a change.
//
// forge.go's PullRequest.NodeID is "the opaque id the flip-draft mutation needs" — opaque
// being the operative word: on GitHub it is a GraphQL global id, and on GitLab there is no
// such thing at all. MarkReadyForReview takes ONLY that id (the interface is frozen, so the
// signature cannot grow a repo argument), and clearing a `Draft:` prefix needs the project
// AND the MR iid. This backend therefore mints its own opaque id carrying both, in the
// shape `gitlab:<owner>/<name>!<iid>` — GitLab's own `!` MR-reference notation, so the id
// is legible in a log line rather than an unreadable blob.
const gitlabNodeIDScheme = "gitlab:"

func gitlabNodeID(repo ForgeRepo, iid int) string {
	return fmt.Sprintf("%s%s!%d", gitlabNodeIDScheme, repo.Slug(), iid)
}

// parseGitLabNodeID reverses gitlabNodeID. It REFUSES anything it did not mint rather than
// guessing: a GitHub node id handed to the GitLab backend is a wiring bug, and acting on a
// half-parsed id would target the wrong project.
func parseGitLabNodeID(nodeID string) (ForgeRepo, int, error) {
	fail := func() (ForgeRepo, int, error) {
		return ForgeRepo{}, 0, Refused(fmt.Sprintf(
			"not a GitLab change id: %q (expected %s<owner>/<name>!<iid>, as minted by the GitLab forge backend)",
			StripControl(nodeID), gitlabNodeIDScheme))
	}
	rest, ok := strings.CutPrefix(nodeID, gitlabNodeIDScheme)
	if !ok {
		return fail()
	}
	slug, iidStr, ok := strings.Cut(rest, "!")
	if !ok {
		return fail()
	}
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || name == "" {
		return fail()
	}
	iid, err := strconv.Atoi(iidStr)
	if err != nil || iid <= 0 {
		return fail()
	}
	return ForgeRepo{Owner: owner, Name: name}, iid, nil
}

// --- Pagination ---

// The per-page and page-cap constants mirror the GitHub backend's (forge_github.go): GitLab
// caps per_page at 100 too, and the same walk-to-exhaustion-then-reconcile discipline
// applies. GitLab signals continuation with the `X-Next-Page` HEADER rather than Link
// relations (brief fact 4); the client library parses it into Response.NextPage, and a walk
// stops when it comes back 0 — which is authoritative, unlike inferring the end from a
// short page.
const (
	gitlabPerPage     = 100
	gitlabMaxFilePage = 40 // 4000 entries, matching forgeMaxFilePages
	gitlabMaxCIPage   = 25 // 2500 entries, matching forgeMaxCIPages
	gitlabMaxNotePage = 25
)

// gitlabTotal reads the forge's OWN asserted total for a listing.
//
// GitLab sends X-Total (parsed into Response.TotalItems) for most offset-paginated
// endpoints but OMITS it on very large or unindexed sets. When it is absent the honest
// answer is the walked length: inventing a larger number would fabricate a failure, and
// there is no third field to report "unknown" through. The walk itself is bounded by
// NextPage, which is present either way, so an omitted total does not hide a short read.
func gitlabTotal(resp *gitlab.Response, walked int) int {
	if resp != nil && resp.TotalItems > 0 {
		return int(resp.TotalItems)
	}
	return walked
}

// --- Reads ---

// GetPullRequest reads a merge request. GitLab addresses an MR within a project by its
// per-project IID (the number a human sees, `!7`), which is what every desk tool means by a
// change number — not the instance-global `id`, which is never shown and never used here.
func (g *GitLabForge) GetPullRequest(repo ForgeRepo, number int) (*PullRequest, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", g.projectPath(repo), number)
	mr, _, err := cl.MergeRequests.GetMergeRequest(repo.Slug(), int64(number), nil)
	if err != nil {
		return nil, g.mapErr(http.MethodGet, path, err)
	}
	changed, ok := gitlabChangedFileCount(mr.ChangesCount)
	if !ok {
		// Not fatal to the whole read — but the count is the reconciliation partner for
		// ListChangedFiles, so an unreadable one must not silently become 0 (which would
		// reconcile clean against ANY walk). Refuse instead.
		return nil, Unverifiable(fmt.Sprintf(
			"could-not-check: %s %s — changes_count %q is neither an exact count nor a truncated one, "+
				"so the changed-file reconciliation cannot be performed",
			http.MethodGet, path, StripControl(mr.ChangesCount)), nil)
	}
	out := &PullRequest{
		Number:       int(mr.IID),
		State:        gitlabState(mr.State),
		Draft:        mr.Draft,
		NodeID:       gitlabNodeID(repo, int(mr.IID)),
		ChangedFiles: changed,
		HeadSHA:      mr.SHA,
		Mergeable:    gitlabMergeableState(mr.DetailedMergeStatus),
		Labels:       append([]string(nil), mr.Labels...),
		URL:          mr.WebURL,
		HeadRef:      mr.SourceBranch,
	}
	if mr.Author != nil {
		out.Author = gitlabAccount(mr.Author.ID, mr.Author.Username)
	}
	if mr.UpdatedAt != nil {
		out.UpdatedAt = mr.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return out, nil
}

// GetIssue resolves what a bare number IS, and refuses when GitLab cannot say.
//
// This is the mapping with no GitHub analog. GitHub serves issues and pull requests from a
// SINGLE per-repo number sequence, so `GET /issues/{n}` always resolves and its
// `pull_request` sub-object is a reliable discriminator. GitLab keeps issues and merge
// requests in SEPARATE per-project IID sequences, so `#7` and `!7` routinely both exist and
// are different objects. A backend that probed only the issues endpoint would answer
// IsPullRequest:false for every number — quietly correct on the happy path and wrong
// exactly when a desk tool asks "is this number a change?" about a number that is both.
//
// So both are probed, and:
//   - exactly one resolves  → that is the answer;
//   - neither resolves      → the 404 is returned (IsForgeNotFound licenses the caller's
//     own re-resolution, as on GitHub);
//   - BOTH resolve          → REFUSED. There is no correct answer to give, and picking one
//     would route a comment or a verdict at the wrong object. The
//     caller must use a typed operation instead.
//
// A non-404 error from either probe (401/403) is returned as-is: a tier or credential
// failure must not be read as "this kind does not exist".
func (g *GitLabForge) GetIssue(repo ForgeRepo, number int) (*Issue, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	issuePath := fmt.Sprintf("/projects/%s/issues/%d", g.projectPath(repo), number)
	mrPath := fmt.Sprintf("/projects/%s/merge_requests/%d", g.projectPath(repo), number)

	iss, _, issErr := cl.Issues.GetIssue(repo.Slug(), int64(number))
	if issErr != nil {
		if mapped := g.mapErr(http.MethodGet, issuePath, issErr); !IsForgeNotFound(mapped) {
			return nil, mapped
		}
		iss = nil
	}
	mr, _, mrErr := cl.MergeRequests.GetMergeRequest(repo.Slug(), int64(number), nil)
	if mrErr != nil {
		if mapped := g.mapErr(http.MethodGet, mrPath, mrErr); !IsForgeNotFound(mapped) {
			return nil, mapped
		}
		mr = nil
	}

	switch {
	case iss != nil && mr != nil:
		return nil, Unverifiable(fmt.Sprintf(
			"could-not-check: %s carries BOTH issue #%d and merge request !%d — GitLab numbers issues and "+
				"merge requests in separate sequences, so a bare number cannot be resolved to one kind; "+
				"use the typed operation for the kind you mean",
			repo.Slug(), number, number), nil)
	case iss != nil:
		out := &Issue{Number: int(iss.IID), State: gitlabState(iss.State), IsPullRequest: false}
		if iss.Author != nil {
			out.Author = gitlabAccount(iss.Author.ID, iss.Author.Username)
		}
		return out, nil
	case mr != nil:
		out := &Issue{Number: int(mr.IID), State: gitlabState(mr.State), IsPullRequest: true}
		if mr.Author != nil {
			out.Author = gitlabAccount(mr.Author.ID, mr.Author.Username)
		}
		return out, nil
	default:
		// Neither kind exists (or is visible). Surface the issue-side 404 so
		// IsForgeNotFound holds.
		return nil, g.mapErr(http.MethodGet, issuePath, issErr)
	}
}

// ReviewsAtHead returns the verdicts on a merge request, WITH the head each is provably
// pinned to — and with an EMPTY CommitID wherever GitLab cannot establish one.
//
// This is the mapping the stream spec calls "review-at-head ↔ approvals + notes filtered by
// head SHA", and it is the one that most rewards being got right, because the naive version
// is convincing: GitLab's approvals endpoint returns `approved_by`, the MR carries `sha`, so
// stamping every approval with the MR's current head "obviously" works. It does not. A
// GitLab approval carries NO sha and NO timestamp, and whether it survives a push is a
// PROJECT SETTING (`reset_approvals_on_push`). With that setting off, an approval read today
// may have been given against a head three pushes ago — and stamping it with the current
// head manufactures exactly the at-head evidence the desk's flip gate exists to require.
//
// So the head is established from facts, never assumed:
//
//   - APPROVALS are stamped with the MR's head sha only when the project actually resets
//     approvals on push, which makes "this approval exists" and "it was given at this head"
//     the same statement. Otherwise CommitID is left EMPTY — the approval is real and is
//     reported, but it is not claimed to be at head, and an at-head filter drops it.
//     Fail-closed: an unpinned approval never counts as an at-head approval.
//   - NOTES (the verdict bodies; GitLab approvals carry no body) are stamped with the head
//     sha when they were created at or after the current diff VERSION arrived — the
//     versions endpoint records exactly when each head landed, so this is a comparison of
//     recorded timestamps, not an inference. Older notes are reported with an empty
//     CommitID.
//
// System notes are excluded from the returned verdicts (they are GitLab's own timeline
// entries, not a reviewer's) but ARE read: the "approved this merge request" system note is
// the only place GitLab records WHEN an approval was given, so it supplies SubmittedAt for
// the matching approver.
//
// Cost: this is four reads plus the note pages. Each is golden-pinned, and the alternative
// is a cheaper answer that is sometimes false.
func (g *GitLabForge) ReviewsAtHead(repo ForgeRepo, number int) ([]Review, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	proj := g.projectPath(repo)

	mrPath := fmt.Sprintf("/projects/%s/merge_requests/%d", proj, number)
	mr, _, err := cl.MergeRequests.GetMergeRequest(repo.Slug(), int64(number), nil)
	if err != nil {
		return nil, g.mapErr(http.MethodGet, mrPath, err)
	}
	headSHA := mr.SHA

	// When the current head landed. versions[0] is the newest (GitLab returns them id-desc).
	versionsPath := fmt.Sprintf("/projects/%s/merge_requests/%d/versions", proj, number)
	versions, _, err := cl.MergeRequests.GetMergeRequestDiffVersions(repo.Slug(), int64(number),
		&gitlab.GetMergeRequestDiffVersionsOptions{ListOptions: gitlab.ListOptions{PerPage: 1}})
	if err != nil {
		return nil, g.mapErr(http.MethodGet, versionsPath, err)
	}
	var headArrivedAt *time.Time
	if len(versions) > 0 && versions[0].HeadCommitSHA == headSHA {
		headArrivedAt = versions[0].CreatedAt
	}

	// Does an approval survive a push on this project? Only if not, an approval cannot be
	// attributed to the current head.
	approvalCfgPath := fmt.Sprintf("/projects/%s/approvals", proj)
	cfg, _, err := cl.Projects.GetApprovalConfiguration(repo.Slug())
	if err != nil {
		// Approval configuration is a Premium+ surface (spec §3). A tier or permission
		// failure here is could-not-check for the WHOLE read, not a licence to fall back to
		// "assume approvals are head-pinned".
		return nil, g.mapErr(http.MethodGet, approvalCfgPath, err)
	}
	approvalsArePinned := cfg.ResetApprovalsOnPush

	approvalsPath := fmt.Sprintf("/projects/%s/merge_requests/%d/approvals", proj, number)
	approvals, _, err := cl.MergeRequests.GetMergeRequestApprovals(repo.Slug(), int64(number))
	if err != nil {
		return nil, g.mapErr(http.MethodGet, approvalsPath, err)
	}

	notesPath := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", proj, number)
	var notes []*gitlab.Note
	for page := 1; page <= gitlabMaxNotePage; page++ {
		chunk, resp, nerr := cl.Notes.ListMergeRequestNotes(repo.Slug(), int64(number),
			&gitlab.ListMergeRequestNotesOptions{
				ListOptions: gitlab.ListOptions{PerPage: gitlabPerPage, Page: int64(page)},
			})
		if nerr != nil {
			return nil, g.mapErr(http.MethodGet, notesPath, nerr)
		}
		notes = append(notes, chunk...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}

	// GitLab records the moment of an approval only in its system-note timeline. Collect the
	// LATEST such note per approver so an approval can carry a real SubmittedAt.
	approvedAt := map[int64]*time.Time{}
	for _, n := range notes {
		if n == nil || !n.System || !gitlabIsApprovalSystemNote(n.Body) {
			continue
		}
		if prev, ok := approvedAt[n.Author.ID]; !ok || gitlabAfter(n.CreatedAt, prev) {
			approvedAt[n.Author.ID] = n.CreatedAt
		}
	}

	var out []Review
	for _, a := range approvals.ApprovedBy {
		if a == nil || a.User == nil {
			continue
		}
		r := Review{
			// GitLab mints no per-approval id; the approver's user id is the only stable
			// identifier the approvals payload carries.
			ID:          a.User.ID,
			Author:      gitlabAccount(a.User.ID, a.User.Username),
			State:       "APPROVED",
			SubmittedAt: gitlabTime(approvedAt[a.User.ID]),
		}
		if approvalsArePinned {
			r.CommitID = headSHA
		}
		out = append(out, r)
	}
	for _, n := range notes {
		if n == nil || n.System {
			continue
		}
		r := Review{
			ID:          n.ID,
			Author:      gitlabAccount(n.Author.ID, n.Author.Username),
			State:       "COMMENTED",
			Body:        n.Body,
			SubmittedAt: gitlabTime(n.CreatedAt),
		}
		if headArrivedAt != nil && n.CreatedAt != nil && !n.CreatedAt.Before(*headArrivedAt) {
			r.CommitID = headSHA
		}
		out = append(out, r)
	}
	return out, nil
}

// gitlabApprovalSystemNoteBodies are the system-note bodies GitLab writes when an approval
// is recorded. Matched exactly (after trimming) rather than by substring: a REVIEWER's note
// that merely quotes the phrase must not be mistaken for the timeline entry.
var gitlabApprovalSystemNoteBodies = map[string]bool{
	"approved this merge request": true,
}

func gitlabIsApprovalSystemNote(body string) bool {
	return gitlabApprovalSystemNoteBodies[strings.TrimSpace(strings.ToLower(body))]
}

// gitlabAfter reports whether a is strictly later than b, treating a nil b as "earlier than
// anything" and a nil a as "not later".
func gitlabAfter(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return a.After(*b)
}

// ListChangedFiles returns the MR's file entries, rename-aware.
//
// GitLab's diffs endpoint reports old_path and new_path on every entry plus explicit
// new_file / renamed_file / deleted_file flags, so both halves of a rename survive — which
// is what the risk-path gate needs (forge.go: "a `git mv` of a security path plus edits
// must still surface the pre-rename path"). PreviousFilename is set ONLY on a rename, as on
// GitHub, so a caller cannot mistake an unchanged path for a rename source.
func (g *GitLabForge) ListChangedFiles(repo ForgeRepo, number int) ([]ChangedFile, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/diffs", g.projectPath(repo), number)
	var all []ChangedFile
	for page := 1; page <= gitlabMaxFilePage; page++ {
		chunk, resp, derr := cl.MergeRequests.ListMergeRequestDiffs(repo.Slug(), int64(number),
			&gitlab.ListMergeRequestDiffsOptions{
				ListOptions: gitlab.ListOptions{PerPage: gitlabPerPage, Page: int64(page)},
			})
		if derr != nil {
			return nil, g.mapErr(http.MethodGet, path, derr)
		}
		for _, d := range chunk {
			if d == nil {
				continue
			}
			cf := ChangedFile{Filename: d.NewPath, Status: gitlabDiffStatus(d)}
			if d.RenamedFile {
				cf.PreviousFilename = d.OldPath
			}
			if d.DeletedFile {
				// A deleted entry's new_path echoes the old path; keep the path that was
				// removed so a risk-path gate still sees it.
				cf.Filename = d.OldPath
			}
			all = append(all, cf)
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return all, nil
}

// gitlabDiffStatus maps GitLab's three booleans to GitHub's status vocabulary. The order is
// load-bearing: GitLab sets renamed_file together with new_file/deleted_file in some
// rename-with-edit cases, and `renamed` is the status that carries the extra path a risk
// gate must see, so it wins.
func gitlabDiffStatus(d *gitlab.MergeRequestDiff) string {
	switch {
	case d.RenamedFile:
		return "renamed"
	case d.NewFile:
		return "added"
	case d.DeletedFile:
		return "removed"
	default:
		return "modified"
	}
}

// ChecksAtHead returns the CI rollups at a commit.
//
// The spec's mapping is "required check ↔ pipeline status (+ external status check at
// Ultimate)". GitLab has no single combined-status document, so the two GitHub rollups are
// filled from the two GitLab surfaces that actually correspond:
//
//   - Statuses  ← the commit-statuses endpoint (`/statuses`), GitLab's own direct analog of
//     GitHub's legacy commit statuses: externally-posted, named, per-commit.
//   - CheckRuns ← the JOBS of the commit's last pipeline. A GitHub required check is a named
//     check-run; a GitLab required check is a named pipeline job. Enumerating
//     jobs is what makes "is check X green at this head" answerable at all —
//     the pipeline's own status is a single rollup with no names in it.
//   - CombinedState ← the commit's build status, which IS GitLab's rollup over both.
//
// External status checks (Ultimate) are deliberately NOT read here: that endpoint is
// MR-scoped (`/merge_requests/:iid/status_checks`) and this operation is addressed by SHA
// alone, so there is no MR to ask about. Wiring them belongs to the stream's Ultimate-tier
// lane, which adds the consuming tool alongside — the interface's freeze rule.
//
// A commit with no pipeline yields empty CheckRuns and a zero count, which is a truthful
// "no jobs", distinct from the could-not-check an error produces.
func (g *GitLabForge) ChecksAtHead(repo ForgeRepo, sha string) (*ChecksAtHead, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	proj := g.projectPath(repo)
	out := &ChecksAtHead{}

	commitPath := fmt.Sprintf("/projects/%s/repository/commits/%s", proj, sha)
	commit, _, err := cl.Commits.GetCommit(repo.Slug(), sha, nil)
	if err != nil {
		return nil, g.mapErr(http.MethodGet, commitPath, err)
	}
	if commit.Status != nil {
		out.CombinedState = gitlabBuildState(string(*commit.Status))
	}

	statusPath := fmt.Sprintf("/projects/%s/repository/commits/%s/statuses", proj, sha)
	var lastResp *gitlab.Response
	for page := 1; page <= gitlabMaxCIPage; page++ {
		chunk, resp, serr := cl.Commits.GetCommitStatuses(repo.Slug(), sha,
			&gitlab.GetCommitStatusesOptions{
				ListOptions: gitlab.ListOptions{PerPage: gitlabPerPage, Page: int64(page)},
			})
		if serr != nil {
			return nil, g.mapErr(http.MethodGet, statusPath, serr)
		}
		lastResp = resp
		for _, s := range chunk {
			if s == nil {
				continue
			}
			out.Statuses = append(out.Statuses, StatusContext{
				State:     gitlabBuildState(s.Status),
				Context:   s.Name,
				CreatedAt: gitlabTime(s.CreatedAt),
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	out.StatusTotalCount = gitlabTotal(lastResp, len(out.Statuses))

	if commit.LastPipeline == nil {
		return out, nil
	}
	jobsPath := fmt.Sprintf("/projects/%s/pipelines/%d/jobs", proj, commit.LastPipeline.ID)
	lastResp = nil
	for page := 1; page <= gitlabMaxCIPage; page++ {
		chunk, resp, jerr := cl.Jobs.ListPipelineJobs(repo.Slug(), commit.LastPipeline.ID,
			&gitlab.ListJobsOptions{
				ListOptions: gitlab.ListOptions{PerPage: gitlabPerPage, Page: int64(page)},
			})
		if jerr != nil {
			return nil, g.mapErr(http.MethodGet, jobsPath, jerr)
		}
		lastResp = resp
		for _, j := range chunk {
			if j == nil {
				continue
			}
			status, conclusion := gitlabJobStatus(j.Status)
			out.CheckRuns = append(out.CheckRuns, CheckRun{
				Name: j.Name, Status: status, Conclusion: conclusion,
				// GitLab's finished_at is the job's true end — the same fact GitHub's
				// completed_at carries — so the latest-run-per-name reduction orders both
				// forges' rollups by the same kind of stamp rather than by list position.
				StartedAt:   gitlabTime(j.StartedAt),
				CompletedAt: gitlabTime(j.FinishedAt),
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	out.CheckRunsTotalCount = gitlabTotal(lastResp, len(out.CheckRuns))
	return out, nil
}

// gitlabBuildState maps a GitLab build state to GitHub's combined-status vocabulary
// (success | pending | failure | error).
//
// The two that are NOT obvious, and are deliberate:
//
//   - `canceled` → failure. A cancelled required check did not pass, and the desk's flip
//     gate must treat "not green" as not-green. Calling it pending would leave a PR
//     forever "still running"; calling it success would flip on a check that never ran.
//   - `manual` → pending. A manual job is awaiting an action that may still come, which is
//     the same state a queued job is in from the gate's point of view.
//
// `skipped` maps to success, matching GitHub, where a skipped check does not block.
// An unrecognised state maps to `error` — fail-closed, never silently to success.
func gitlabBuildState(s string) string {
	switch s {
	case "success":
		return "success"
	case "skipped":
		return "success"
	case "failed":
		return "failure"
	case "canceled", "canceling":
		return "failure"
	case "created", "waiting_for_resource", "preparing", "pending", "running", "scheduled", "manual":
		return "pending"
	case "":
		return ""
	default:
		return "error"
	}
}

// gitlabJobStatus splits a GitLab job state into GitHub's two check-run fields: a lifecycle
// `status` (queued | in_progress | completed) and, once completed, a `conclusion`. A job
// that has not finished has NO conclusion — the empty string, not a guessed one.
func gitlabJobStatus(s string) (status, conclusion string) {
	switch s {
	case "created", "pending", "waiting_for_resource", "preparing", "scheduled", "manual":
		return "queued", ""
	case "running":
		return "in_progress", ""
	case "success":
		return "completed", "success"
	case "failed":
		return "completed", "failure"
	case "canceled", "canceling":
		return "completed", "cancelled"
	case "skipped":
		return "completed", "skipped"
	default:
		// Unknown lifecycle: report it as completed with no successful conclusion so a
		// caller that requires `success` refuses rather than waits forever.
		return "completed", "neutral"
	}
}

// IssueReactions returns the award emoji on an issue, mapped onto the reaction vocabulary
// the house admission gate speaks — INCLUDING the human/bot discriminator that gate needs.
//
// Two mappings here are security-relevant, and both exist because the naive version silently
// WEAKENS a control that GitHub enforces:
//
//  1. NAMES. The gate in repovis.go admits on `Content == "+1"`. GitLab names the same
//     emoji `thumbsup`. Left unmapped, no GitLab 👍 would ever satisfy the gate; mapped
//     wrongly, the wrong emoji would. gitlabAwardToReaction is the exact correspondence for
//     the eight reactions GitHub supports; any other award passes through under its own
//     name, so it is visible but matches no gate.
//  2. HUMAN vs BOT. That same gate requires `User.Type == "User"` — a bot's 👍 must not
//     authorize anything. GitLab's award payload carries only a BasicUser, which has NO bot
//     flag, so defaulting Type to "User" would let a service account's award satisfy a gate
//     that on GitHub only a human can. Instead each DISTINCT awarding user is resolved once
//     through the users API, whose `bot` field is the real discriminator. If that lookup
//     fails, Type is left EMPTY — which the gate reads as not-a-human and refuses. Never
//     defaulted to "User".
//
// Like the GitHub backend this walks a SINGLE page: past 100 awards on one issue it reports
// what it read, and the gate — which requires a POSITIVE match — cannot be opened by an
// unread award, only closed.
func (g *GitLabForge) IssueReactions(repo ForgeRepo, number int) ([]Reaction, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/projects/%s/issues/%d/award_emoji", g.projectPath(repo), number)
	awards, _, err := cl.AwardEmoji.ListIssueAwardEmoji(repo.Slug(), int64(number),
		&gitlab.ListAwardEmojiOptions{ListOptions: gitlab.ListOptions{PerPage: gitlabPerPage}})
	if err != nil {
		return nil, g.mapErr(http.MethodGet, path, err)
	}
	kind := map[int64]string{}
	out := make([]Reaction, 0, len(awards))
	for _, a := range awards {
		if a == nil {
			continue
		}
		t, seen := kind[a.User.ID]
		if !seen {
			t = g.gitlabActorType(cl, a.User.ID)
			kind[a.User.ID] = t
		}
		out = append(out, Reaction{
			Content: gitlabReactionContent(a.Name),
			User:    ReactionUser{Login: a.User.Username, ID: a.User.ID, Type: t},
		})
	}
	return out, nil
}

// gitlabActorType resolves whether a user id is a human or a bot, returning "User", "Bot",
// or "" when it could not be established. The empty answer is the fail-closed one: every
// house gate that consults it requires the literal "User", so an unresolved actor authorizes
// nothing. A lookup failure is deliberately NOT propagated as an error — one unreadable
// actor must not blank the whole reaction list, which would hide the awards that DID
// resolve.
func (g *GitLabForge) gitlabActorType(cl *gitlab.Client, id int64) string {
	u, _, err := cl.Users.GetUser(id, gitlab.GetUsersOptions{})
	if err != nil || u == nil {
		return ""
	}
	if u.Bot {
		return "Bot"
	}
	return "User"
}

// gitlabAwardToReaction maps GitLab award-emoji names to GitHub reaction content values.
// GitHub supports exactly these eight; the pairs are by emoji, not by name similarity
// (GitLab's `tada` is GitHub's `hooray`, and GitLab's `laughing` is GitHub's `laugh`).
var gitlabAwardToReaction = map[string]string{
	"thumbsup":   "+1",
	"thumbsdown": "-1",
	"laughing":   "laugh",
	"confused":   "confused",
	"heart":      "heart",
	"tada":       "hooray",
	"rocket":     "rocket",
	"eyes":       "eyes",
}

// gitlabReactionContent maps one award name, passing an unmapped name through unchanged so
// it stays visible to a human reading the list while matching no gate.
func gitlabReactionContent(name string) string {
	if c, ok := gitlabAwardToReaction[name]; ok {
		return c
	}
	return name
}

// RepoVisibility returns the project's visibility. GitLab's three values are
// private | internal | public — `internal` (visible to any signed-in instance user) has no
// GitHub analog and is passed through unchanged rather than folded into either neighbour:
// the public-repo gate must decide about it explicitly, not inherit a guess made here.
func (g *GitLabForge) RepoVisibility(repo ForgeRepo) (string, error) {
	cl, err := g.client()
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/projects/%s", g.projectPath(repo))
	p, _, err := cl.Projects.GetProject(repo.Slug(), nil)
	if err != nil {
		return "", g.mapErr(http.MethodGet, path, err)
	}
	if p.Visibility == "" {
		return "", Unverifiable(fmt.Sprintf("project %s has no .visibility field in the API response", repo.Slug()), nil)
	}
	return string(p.Visibility), nil
}

// --- Writes ---

// CreateDraftChange opens a merge request as a DRAFT.
//
// GitLab has no draft boolean: draft-ness IS the `Draft:` title prefix, which GitLab parses
// back out into the `draft` field it returns. The prefix is applied only when the caller's
// title does not already carry one, so a caller that spelled it itself does not end up with
// "Draft: Draft: …". Opening as a draft is the frozen property of this seam — it opens
// changes as drafts, never ready.
func (g *GitLabForge) CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	title := in.Title
	if _, already := gitlabStripDraftPrefix(title); !already {
		title = gitlabDraftPrefix + title
	}
	path := fmt.Sprintf("/projects/%s/merge_requests", g.projectPath(repo))
	mr, _, err := cl.MergeRequests.CreateMergeRequest(repo.Slug(), &gitlab.CreateMergeRequestOptions{
		Title:        gitlab.Ptr(title),
		Description:  gitlab.Ptr(in.Body),
		SourceBranch: gitlab.Ptr(in.Head),
		TargetBranch: gitlab.Ptr(in.Base),
	})
	if err != nil {
		return nil, g.mapErr(http.MethodPost, path, err)
	}
	if !mr.Draft {
		// The prefix is the only draft mechanism; if GitLab did not read it back as a
		// draft, the change is OPEN FOR REVIEW when it was meant to be a draft. Refuse
		// rather than hand back a ref that misrepresents the change's state.
		return nil, Unverifiable(fmt.Sprintf(
			"could-not-check: created merge request !%d in %s did not come back marked draft (title %q) — "+
				"the Draft: prefix is GitLab's only draft mechanism and it did not take",
			mr.IID, repo.Slug(), StripControl(mr.Title)), nil)
	}
	return &PullRef{
		Number: int(mr.IID),
		NodeID: gitlabNodeID(repo, int(mr.IID)),
		URL:    mr.WebURL,
	}, nil
}

// PostComment posts a note on an issue or merge request.
//
// GitLab has no shared comments endpoint: notes live under `/issues/:iid/notes` or
// `/merge_requests/:iid/notes`, so the kind must be resolved before the write. It is
// resolved by GetIssue, which carries the both-kinds-exist refusal — deliberately, because
// posting a comment at the wrong object is exactly the failure that ambiguity produces.
// The reference it returns carries the note's numeric id and the opaque id EditComment
// takes. It carries NO url: GitLab's notes API publishes no per-note location, and composing
// one would be this tree inventing an address the forge never asserted (see Comment.URL).
func (g *GitLabForge) PostComment(repo ForgeRepo, number int, body string) (*CommentRef, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	kind, err := g.GetIssue(repo, number)
	if err != nil {
		return nil, err
	}
	if kind.IsPullRequest {
		path := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", g.projectPath(repo), number)
		note, _, perr := cl.Notes.CreateMergeRequestNote(repo.Slug(), int64(number),
			&gitlab.CreateMergeRequestNoteOptions{Body: gitlab.Ptr(body)})
		if perr != nil {
			return nil, g.mapErr(http.MethodPost, path, perr)
		}
		return &CommentRef{ID: gitlabNoteID(repo, number, note.ID), DatabaseID: note.ID}, nil
	}
	path := fmt.Sprintf("/projects/%s/issues/%d/notes", g.projectPath(repo), number)
	note, _, perr := cl.Notes.CreateIssueNote(repo.Slug(), int64(number),
		&gitlab.CreateIssueNoteOptions{Body: gitlab.Ptr(body)})
	if perr != nil {
		return nil, g.mapErr(http.MethodPost, path, perr)
	}
	// An ISSUE note's id is deliberately NOT wrapped in a merge-request-shaped opaque id:
	// EditComment addresses merge-request notes, and handing back an id that would route an
	// edit at the wrong endpoint is worse than handing back none. The numeric id is reported
	// so the write is still answerable.
	return &CommentRef{DatabaseID: note.ID}, nil
}

// PostReview submits a head-pinned verdict on a merge request.
//
// GitHub's review is ONE atomic call carrying a body, a verdict and the reviewed commit.
// GitLab splits all three apart: an approval is a separate endpoint, it carries no body, and
// there is no "request changes" verdict at all. This backend therefore composes the verdict,
// and the ORDER is the safety property:
//
//	the note (the reasoning) is posted FIRST, the approval second.
//
// Either half can fail. Note-then-approve fails to "a verdict comment with no approval",
// which grants nothing and is visible to a human. Approve-then-note would fail to "an
// approval with no stated reasoning" — a merge-enabling grant with its justification
// missing. Only one of those two is safe to fail into.
//
// The verbs map as:
//
//   - APPROVE         → note (when a body is given) + POST /approve, pinned with `sha`.
//     GitLab VALIDATES that sha against the MR head server-side and
//     rejects a mismatch, so the head-pin is enforced by the forge, not
//     merely asserted by us — strictly stronger than GitHub's commit_id,
//     which is recorded rather than checked.
//   - REQUEST_CHANGES → note + revoke this actor's own approval. GitLab has no
//     changes-requested verdict; the closest true statement is "this
//     reviewer's approval does not stand at this head", which is what
//     unapprove records. A 404 from unapprove means there was no approval
//     to revoke — already the desired state, so it is not an error.
//   - COMMENT         → note only.
//
// An unrecognised event is REFUSED. Silently degrading an unknown verb to a comment would
// turn a failed approval into a successful-looking no-op.
func (g *GitLabForge) PostReview(repo ForgeRepo, number int, in ReviewInput) error {
	cl, err := g.client()
	if err != nil {
		return err
	}
	event := strings.ToUpper(strings.TrimSpace(in.Event))
	switch event {
	case "APPROVE", "REQUEST_CHANGES", "COMMENT":
	default:
		return Refused(fmt.Sprintf("unsupported review event %q — expected APPROVE, REQUEST_CHANGES or COMMENT",
			StripControl(in.Event)))
	}
	proj := g.projectPath(repo)

	if in.Body != "" {
		notePath := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", proj, number)
		_, _, nerr := cl.Notes.CreateMergeRequestNote(repo.Slug(), int64(number),
			&gitlab.CreateMergeRequestNoteOptions{Body: gitlab.Ptr(in.Body)})
		if nerr != nil {
			return g.mapErr(http.MethodPost, notePath, nerr)
		}
	}

	switch event {
	case "APPROVE":
		path := fmt.Sprintf("/projects/%s/merge_requests/%d/approve", proj, number)
		var opt *gitlab.ApproveMergeRequestOptions
		if in.HeadSHA != "" {
			opt = &gitlab.ApproveMergeRequestOptions{SHA: gitlab.Ptr(in.HeadSHA)}
		}
		_, _, aerr := cl.MergeRequestApprovals.ApproveMergeRequest(repo.Slug(), int64(number), opt)
		return g.mapErr(http.MethodPost, path, aerr)
	case "REQUEST_CHANGES":
		path := fmt.Sprintf("/projects/%s/merge_requests/%d/unapprove", proj, number)
		_, uerr := cl.MergeRequestApprovals.UnapproveMergeRequest(repo.Slug(), int64(number))
		if uerr != nil {
			mapped := g.mapErr(http.MethodPost, path, uerr)
			if IsForgeNotFound(mapped) {
				// Nothing to revoke — the post-condition ("this reviewer does not approve
				// at this head") already holds.
				return nil
			}
			return mapped
		}
		return nil
	default: // COMMENT
		if in.Body == "" {
			return Refused("a COMMENT review with an empty body would post nothing")
		}
		return nil
	}
}

// MarkReadyForReview clears the `Draft:` prefix — GitLab's only ready transition.
//
// The nodeID is the opaque id this backend minted (see gitlabNodeIDScheme); anything else is
// refused rather than guessed at. Clearing the prefix needs the CURRENT title, so the MR is
// read first: rewriting the title from a stale copy would silently revert an edit made in
// between.
//
// It is IDEMPOTENT — an MR that is already not a draft returns success without a write.
// GitHub's mutation errors in that case, and the divergence is deliberate: the desk's ready
// verb is a verify-then-retry loop, and turning "already in the desired state" into a
// failure makes a successful flip look like a failed one on the retry. The post-condition
// this operation promises is "the change is not a draft", and that post-condition holds.
func (g *GitLabForge) MarkReadyForReview(nodeID string) error {
	repo, iid, err := parseGitLabNodeID(nodeID)
	if err != nil {
		return err
	}
	cl, cerr := g.client()
	if cerr != nil {
		return cerr
	}
	proj := g.projectPath(repo)
	getPath := fmt.Sprintf("/projects/%s/merge_requests/%d", proj, iid)
	mr, _, gerr := cl.MergeRequests.GetMergeRequest(repo.Slug(), int64(iid), nil)
	if gerr != nil {
		return g.mapErr(http.MethodGet, getPath, gerr)
	}
	stripped, had := gitlabStripDraftPrefix(mr.Title)
	if !had {
		return nil
	}
	if stripped == "" {
		// The title was nothing but the marker. Removing it would leave an MR with an empty
		// title, which GitLab rejects — and inventing one would put words in the author's
		// mouth.
		return Refused(fmt.Sprintf(
			"merge request !%d in %s has no title beyond its draft marker, so clearing the marker would "+
				"leave it untitled — retitle it before flipping it ready", iid, repo.Slug()))
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", proj, iid)
	updated, _, uerr := cl.MergeRequests.UpdateMergeRequest(repo.Slug(), int64(iid),
		&gitlab.UpdateMergeRequestOptions{Title: gitlab.Ptr(stripped)})
	if uerr != nil {
		return g.mapErr(http.MethodPut, path, uerr)
	}
	if updated.Draft {
		return Unverifiable(fmt.Sprintf(
			"could-not-check: merge request !%d in %s is still marked draft after clearing its title marker",
			iid, repo.Slug()), nil)
	}
	return nil
}

// FileIssue files a new issue.
func (g *GitLabForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/projects/%s/issues", g.projectPath(repo))
	iss, _, err := cl.Issues.CreateIssue(repo.Slug(), &gitlab.CreateIssueOptions{
		Title:       gitlab.Ptr(in.Title),
		Description: gitlab.Ptr(in.Body),
	})
	if err != nil {
		return nil, g.mapErr(http.MethodPost, path, err)
	}
	return &IssueRef{Number: int(iss.IID), URL: iss.WebURL}, nil
}

// CloseIssue closes an issue via `state_event=close`.
//
// GitHub's stateReason (completed / not_planned) has NO GitLab equivalent: GitLab records
// only that an issue is closed. Rather than drop the distinction silently — a caller that
// asked for "not planned" would get a close indistinguishable from "completed" and never
// know — the reason is recorded as a note before the close, so the information survives
// where a human and the API can both still read it. An empty reason writes no note.
func (g *GitLabForge) CloseIssue(repo ForgeRepo, number int, stateReason string) error {
	cl, err := g.client()
	if err != nil {
		return err
	}
	proj := g.projectPath(repo)
	if stateReason != "" {
		notePath := fmt.Sprintf("/projects/%s/issues/%d/notes", proj, number)
		_, _, nerr := cl.Notes.CreateIssueNote(repo.Slug(), int64(number),
			&gitlab.CreateIssueNoteOptions{
				Body: gitlab.Ptr(fmt.Sprintf("closed as: %s", stateReason)),
			})
		if nerr != nil {
			return g.mapErr(http.MethodPost, notePath, nerr)
		}
	}
	path := fmt.Sprintf("/projects/%s/issues/%d", proj, number)
	_, _, uerr := cl.Issues.UpdateIssue(repo.Slug(), int64(number),
		&gitlab.UpdateIssueOptions{StateEvent: gitlab.Ptr("close")})
	return g.mapErr(http.MethodPut, path, uerr)
}

// DeleteRef deletes one git ref. This is the mapping that does NOT reach across cleanly, and
// the refusal below is the honest half of it.
//
// GitHub exposes a general git-data ref API (`DELETE /git/refs/<anything>`), so it can delete
// a ref in any namespace — including the `refs/dispatch/*` namespace the desk's claim
// mechanism uses. GitLab Community Edition exposes no general ref-delete endpoint at all: the
// Branches API (`DELETE /projects/:id/repository/branches/:branch`, Tier: Free/Premium/
// Ultimate — https://docs.gitlab.com/api/branches/) deletes a BRANCH, and tags have their own
// endpoint. There is no CE endpoint, at any tier, for a ref outside those namespaces.
//
// So this backend serves "heads/<branch>" from the Branches API and REFUSES every other
// namespace as could-not-check, naming the gap. It does not silently succeed, and it does not
// invent a ref-shaped call the instance would answer with an HTML 404 — either would report a
// claim as released when it is still held. A profile that needs claim refs on GitLab uses a
// branch-namespaced claim; that is a workflow decision, not something a backend may paper over.
func (g *GitLabForge) DeleteRef(repo ForgeRepo, ref string) error {
	clean, err := ValidateRefPath(ref)
	if err != nil {
		return err
	}
	branch, ok := strings.CutPrefix(clean, "heads/")
	if !ok {
		return Unverifiable(fmt.Sprintf(
			"could-not-check: GitLab exposes no general ref-delete endpoint, so DeleteRef cannot serve %q — "+
				"only the \"heads/<branch>\" namespace maps (the Branches API); a claim held outside refs/heads "+
				"has no CE equivalent and is NOT reported released", ref), nil)
	}
	cl, cerr := g.client()
	if cerr != nil {
		return cerr
	}
	path := fmt.Sprintf("/projects/%s/repository/branches/%s", g.projectPath(repo), url.PathEscape(branch))
	_, derr := cl.Branches.DeleteBranch(repo.Slug(), branch)
	return g.mapErr(http.MethodDelete, path, derr)
}

// ListLabelEvents returns the merge request's label-application events with the user that
// applied each one.
//
// GitLab's RESOURCE LABEL EVENTS endpoint is the exact analog of the GitHub timeline read
// this replaces: it records add/remove per label with the acting user, which is the one fact
// the model-capability floor needs and the label LIST cannot give. Events whose action is
// not `add` are dropped, matching the GitHub side's filter to `labeled` — a removal is not an
// attestation.
//
// An event whose label GitLab has since DELETED comes back with an empty label name (the
// documented shape: the event survives, the label record does not). It is dropped rather than
// emitted as an unnamed attestation, because a stamp nobody can name attests to nothing.
func (g *GitLabForge) ListLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/resource_label_events",
		g.projectPath(repo), number)
	var out []LabelEvent
	for page := 1; page <= gitlabMaxNotePage; page++ {
		chunk, resp, lerr := cl.ResourceLabelEvents.ListMergeRequestsLabelEvents(repo.Slug(), int64(number),
			&gitlab.ListLabelEventsOptions{
				ListOptions: gitlab.ListOptions{PerPage: gitlabPerPage, Page: int64(page)},
			})
		if lerr != nil {
			return nil, g.mapErr(http.MethodGet, path, lerr)
		}
		for _, e := range chunk {
			if e == nil || e.Action != "add" || strings.TrimSpace(e.Label.Name) == "" {
				continue
			}
			out = append(out, LabelEvent{Name: e.Label.Name, AppliedBy: e.User.Username})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return out, nil
}

// gitlabNoteIDScheme prefixes the opaque comment id this backend mints. Like
// gitlabNodeIDScheme it carries the coordinates the update endpoint needs — project, MR iid
// and note id — because GitLab's note update is addressed by all three while the interface's
// EditComment takes only an opaque id. The shape is legible on purpose:
// `gitlab:<owner>/<name>!<iid>#note<id>`.
const gitlabNoteIDScheme = "gitlab:"

func gitlabNoteID(repo ForgeRepo, iid int, noteID int64) string {
	return fmt.Sprintf("%s%s!%d#note%d", gitlabNoteIDScheme, repo.Slug(), iid, noteID)
}

// parseGitLabNoteID reverses gitlabNoteID, REFUSING anything it did not mint. A GitHub
// GraphQL node id handed to this backend is a wiring bug; half-parsing it would edit a note
// in the wrong project.
func parseGitLabNoteID(id string) (ForgeRepo, int, int64, error) {
	fail := func() (ForgeRepo, int, int64, error) {
		return ForgeRepo{}, 0, 0, Refused(fmt.Sprintf(
			"not a GitLab comment id: %q (expected %s<owner>/<name>!<iid>#note<id>, as minted by the "+
				"GitLab forge backend's ListComments)", StripControl(id), gitlabNoteIDScheme))
	}
	rest, ok := strings.CutPrefix(id, gitlabNoteIDScheme)
	if !ok {
		return fail()
	}
	changeRef, noteRef, ok := strings.Cut(rest, "#note")
	if !ok {
		return fail()
	}
	slug, iidStr, ok := strings.Cut(changeRef, "!")
	if !ok {
		return fail()
	}
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || name == "" {
		return fail()
	}
	iid, ierr := strconv.Atoi(iidStr)
	if ierr != nil || iid <= 0 {
		return fail()
	}
	noteID, nerr := strconv.ParseInt(noteRef, 10, 64)
	if nerr != nil || noteID <= 0 {
		return fail()
	}
	return ForgeRepo{Owner: owner, Name: name}, iid, noteID, nil
}

// ListComments returns a merge request's notes, oldest first.
//
// SYSTEM NOTES ARE DROPPED. GitLab records its own activity ("changed the description",
// "added label X") as notes with `system: true` in the SAME list as human comments. GitHub
// keeps those in a separate timeline, so a backend that passed them through would hand a
// caller a comment list containing entries no account ever wrote — and the consuming
// upsert filters on authorship, which a system note technically satisfies (it carries the
// acting user as its author).
//
// Comment.Minimized is false for every GitLab note, and that is EXACT rather than a default:
// GitLab has no minimise/hide-comment feature, so on a GitLab instance no comment is hidden.
func (g *GitLabForge) ListComments(repo ForgeRepo, number int) ([]Comment, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", g.projectPath(repo), number)
	var out []Comment
	for page := 1; page <= gitlabMaxNotePage; page++ {
		chunk, resp, nerr := cl.Notes.ListMergeRequestNotes(repo.Slug(), int64(number),
			&gitlab.ListMergeRequestNotesOptions{
				ListOptions: gitlab.ListOptions{PerPage: gitlabPerPage, Page: int64(page)},
				// GitLab's default note order is newest-first; the interface promises
				// oldest-first (GitHub's order), and the consuming newest-wins rule depends
				// on it, so the order is REQUESTED rather than reversed after the fact —
				// a local reversal would only reorder the page that was fetched.
				OrderBy: gitlab.Ptr("created_at"),
				Sort:    gitlab.Ptr("asc"),
			})
		if nerr != nil {
			return nil, g.mapErr(http.MethodGet, path, nerr)
		}
		for _, n := range chunk {
			if n == nil || n.System {
				continue
			}
			out = append(out, Comment{
				ID:         gitlabNoteID(repo, number, n.ID),
				DatabaseID: n.ID,
				Author:     gitlabAccount(n.Author.ID, n.Author.Username),
				Body:       n.Body,
				Minimized:  false,
				CreatedAt:  gitlabTime(n.CreatedAt),
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return out, nil
}

// EditComment replaces a note's body. The repo argument is carried for the error message
// only: the note's coordinates travel inside the opaque id (see gitlabNoteID), and a
// MISMATCH between the two is refused rather than resolved in favour of either — a caller
// that passes one project's id with another project's coordinate has a wiring bug, and
// silently trusting one half would write to whichever the backend happened to prefer.
func (g *GitLabForge) EditComment(repo ForgeRepo, commentID, body string) error {
	idRepo, iid, noteID, err := parseGitLabNoteID(commentID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(idRepo.Slug(), repo.Slug()) {
		return Refused(fmt.Sprintf(
			"comment id %q names project %s but the call names %s — refusing to guess which is meant",
			StripControl(commentID), idRepo.Slug(), repo.Slug()))
	}
	cl, cerr := g.client()
	if cerr != nil {
		return cerr
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/notes/%d", g.projectPath(repo), iid, noteID)
	_, _, uerr := cl.Notes.UpdateMergeRequestNote(repo.Slug(), int64(iid), noteID,
		&gitlab.UpdateMergeRequestNoteOptions{Body: gitlab.Ptr(body)})
	return g.mapErr(http.MethodPut, path, uerr)
}

// ApplyLabels reconciles a merge request's labels.
//
// The GitLab mapping is 1:1 with GitHub's in effect but not in shape, and the difference is
// where the care goes:
//
//   - Labels are PROJECT-scoped on both forges, and both require a label to exist before it
//     can be applied — so the ensure step is `POST /projects/:id/labels`, with an
//     already-exists response treated as the success case for an ensure exactly as GitHub's
//     422 is.
//   - GitLab has NO per-label add/remove endpoints on an MR. Instead ONE `PUT
//     /merge_requests/:iid` carries `add_labels` and `remove_labels` together, which is
//     strictly better for this operation: the whole reconciliation lands atomically, where
//     the GitHub backend has to issue one request per removal.
//   - GitLab colors REQUIRE a leading `#`; GitHub forbids one. LabelSpec carries the bare
//     hex digits and each backend renders its own form, so a caller never has to know which
//     forge it is talking to.
//
// The current label set comes from the MR read the family reconciliation needs anyway, so a
// change with no RemoveFamilies issues no extra read.
func (g *GitLabForge) ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	proj := g.projectPath(repo)
	out := &LabelOutcome{}
	adding := map[string]bool{}
	for _, l := range change.Add {
		if strings.TrimSpace(l.Name) == "" {
			return nil, Refused("refusing to apply an unnamed label — the label NAME is the load-bearing part")
		}
		adding[l.Name] = true
	}

	labelPath := fmt.Sprintf("/projects/%s/labels", proj)
	for _, l := range change.Add {
		opts := &gitlab.CreateLabelOptions{Name: gitlab.Ptr(l.Name)}
		if l.Color != "" {
			opts.Color = gitlab.Ptr(gitlabLabelColor(l.Color))
		}
		if l.Description != "" {
			opts.Description = gitlab.Ptr(l.Description)
		}
		_, _, cerr := cl.Labels.CreateLabel(repo.Slug(), opts)
		if cerr == nil {
			continue
		}
		var er *gitlab.ErrorResponse
		if errors.As(cerr, &er) && er.Response != nil && er.Response.StatusCode == http.StatusConflict {
			continue // already exists — the success case for an ensure
		}
		if errors.As(cerr, &er) && er.Response != nil && er.Response.StatusCode == http.StatusBadRequest {
			// GitLab answers a duplicate label name with 400 + "already exists" on some
			// versions and 409 on others. Both mean the post-condition already holds.
			continue
		}
		return nil, g.mapErr(http.MethodPost, labelPath, cerr)
	}

	remove := map[string]bool{}
	for _, n := range change.Remove {
		remove[n] = true
	}
	if len(change.RemoveFamilies) > 0 {
		mrPath := fmt.Sprintf("/projects/%s/merge_requests/%d", proj, number)
		mr, _, gerr := cl.MergeRequests.GetMergeRequest(repo.Slug(), int64(number), nil)
		if gerr != nil {
			return nil, g.mapErr(http.MethodGet, mrPath, gerr)
		}
		for _, cur := range mr.Labels {
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
	removeList := make([]string, 0, len(remove))
	for _, name := range sortedKeys(remove) {
		if adding[name] {
			continue
		}
		removeList = append(removeList, name)
	}

	addList := make([]string, 0, len(change.Add))
	for _, l := range change.Add {
		addList = append(addList, l.Name)
	}
	if len(addList) == 0 && len(removeList) == 0 {
		return out, nil
	}
	opts := &gitlab.UpdateMergeRequestOptions{}
	if len(addList) > 0 {
		opts.AddLabels = (*gitlab.LabelOptions)(&addList)
	}
	if len(removeList) > 0 {
		opts.RemoveLabels = (*gitlab.LabelOptions)(&removeList)
	}
	updPath := fmt.Sprintf("/projects/%s/merge_requests/%d", proj, number)
	if _, _, uerr := cl.MergeRequests.UpdateMergeRequest(repo.Slug(), int64(number), opts); uerr != nil {
		return nil, g.mapErr(http.MethodPut, updPath, uerr)
	}
	out.Added = addList
	out.Removed = removeList
	return out, nil
}

// gitlabLabelColor renders a bare 6-hex-digit LabelSpec color in the form GitLab requires
// (a leading `#`). A value that already carries one is passed through, and a value that is
// not hex at all is passed through unchanged so GitLab's own validation — not a silent local
// rewrite — is what rejects it.
func gitlabLabelColor(color string) string {
	c := strings.TrimSpace(color)
	if c == "" || strings.HasPrefix(c, "#") {
		return c
	}
	return "#" + c
}

// --- Identity / transport ---

// PushTransportHint returns GitLab's push-transport shape: a personal/group/project access
// token authenticates an https push as the username "oauth2" (GitLab's documented username
// for token-authenticated git over https, the analog of GitHub's "x-access-token"), supplied
// through an inline credential.helper that reads the 0600 token file — never a token-in-URL,
// which leaks the secret into argv and the reflog and is classifier-blocked besides.
//
// The host is derived from the configured instance root, because a self-managed EE
// deployment (the profile's normal case) does not push to gitlab.com. Pure function, no
// network call, and it carries NO secret — only the shape.
func (g *GitLabForge) PushTransportHint(repo ForgeRepo) PushTransport {
	host := "gitlab.com"
	if u, err := url.Parse(g.baseURL()); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	return PushTransport{
		RemoteHost:    host,
		TokenUsername: "oauth2",
		CredentialHelperHint: "supply the token via an inline credential.helper that reads the 0600 token " +
			"file; never embed it in the remote URL",
	}
}
