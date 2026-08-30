package deskkit

// forge.go — the ONE seam the desk tools reach a forge through (the forge-abstraction
// stream). Every desk tool speaks to GitHub today by constructing REST/GraphQL calls
// itself; a second forge would otherwise mean a second fork of every tool. This file
// declares the `Forge` interface — the ~dozen operations a shipping desk tool consumes —
// so a second forge (GitLab, brief 02) is an implementation of this interface, not a
// rewrite of the tools.
//
// The interface is FROZEN at the operations a shipping tool consumes (spec §6): adding a
// method requires a consuming tool in the same change. Two properties live DELIBERATELY
// OUTSIDE it, because they are forge-agnostic and must not fork per implementation:
//
//   - Token minting / identity (App JWT exchange, PAT rotation) — the identity layer
//     (spec §2, §5). A `Forge` is handed an already-minted token; it never mints one.
//   - Budgets, rate limiting, breakers, and body/secret checks — these WRAP the interface
//     (they are applied by the tool around a Forge call), never inside an implementation.
//
// The github implementation (forge_github.go) is an extraction of the current GitHub
// behavior, pinned by the golden corpus (forge_github_golden_test.go) so the extraction
// changed nothing observable at the wire.

// GitHubAPIBase is the single home of the GitHub REST/GraphQL host literal. Desk commands
// that keep a package-level test hook (`var apiBaseURL = GitHubAPIBase`) source their
// default from here so the literal is constructed in exactly one place — the forge module —
// and never in a cmd package (the "no direct API construction outside the forge
// implementation" contract).
const GitHubAPIBase = "https://api.github.com"

// ForgeRepo is a repository coordinate: the two components every forge addresses a repo by.
type ForgeRepo struct {
	Owner string
	Name  string
}

// Slug renders the coordinate as "owner/name".
func (r ForgeRepo) Slug() string { return r.Owner + "/" + r.Name }

// Account is the identity of an actor (author, reviewer). Both the login AND the permanent
// numeric id are carried: a login-only identity is a recycled-login hole (trust.go pins on
// the id), so every forge must surface both.
type Account struct {
	Login string
	ID    int64
}

// PullRequest is the subset of a change (GitHub pull request ↔ GitLab merge request) the
// desk tools read: head commit, lifecycle state, draft flag, the opaque node id the
// flip-draft mutation needs, the forge's own changed-file count (the reconciliation partner
// for ListChangedFiles), and the author identity the trust gate needs.
type PullRequest struct {
	Number       int
	State        string // open | closed
	Draft        bool
	NodeID       string // opaque id for the flip-draft mutation
	ChangedFiles int    // the forge's OWN count — reconcile against ListChangedFiles
	Author       Account
	HeadSHA      string
}

// Issue is the subset of an issue the desk tools read. IsPullRequest is the discriminator:
// GitHub serves issues and PRs from one number sequence and the issues endpoint carries a
// pull_request sub-object exactly when the number is a PR (GitLab keeps issues and MRs in
// separate sequences — the mapping resolves per implementation).
type Issue struct {
	Number        int
	State         string // open | closed
	Author        Account
	IsPullRequest bool
}

// Review is one review/approval on a change (GitHub review ↔ GitLab MR approval). CommitID
// pins the verdict to a reviewed head — the property "read reviews at head" depends on.
type Review struct {
	ID          int64
	Author      Account
	State       string // APPROVED | CHANGES_REQUESTED | COMMENTED | DISMISSED | PENDING
	CommitID    string
	Body        string
	SubmittedAt string
}

// ChangedFile is one entry of a change's file list. PreviousFilename is set only on a
// rename and is load-bearing for the risk-path gate (a `git mv` of a security path plus
// edits must still surface the pre-rename path), so both paths are carried.
type ChangedFile struct {
	Filename         string
	PreviousFilename string
	Status           string // added | modified | removed | renamed | ...
}

// StatusContext is one entry of the legacy combined-status rollup.
type StatusContext struct {
	State   string // success | pending | failure | error
	Context string
}

// CheckRun is one entry of the check-runs rollup.
type CheckRun struct {
	Name       string
	Status     string // queued | in_progress | completed
	Conclusion string // success | failure | neutral | ...
}

// ChecksAtHead is the two CI rollups at a commit, each carrying the forge's asserted total
// count so a caller can fail CLOSED when the walk read fewer entries than the head claims.
// Required CI check ↔ pipeline status; at GitLab Ultimate the external-status-check surface
// maps here too (spec §6).
type ChecksAtHead struct {
	CombinedState       string // the legacy rollup's overall state
	StatusTotalCount    int
	Statuses            []StatusContext
	CheckRunsTotalCount int
	CheckRuns           []CheckRun
}

// DraftChangeInput is the request to open a draft change (draft PR ↔ Draft: MR).
type DraftChangeInput struct {
	Title string
	Body  string
	Head  string // source branch
	Base  string // target branch
}

// PullRef identifies a change that was just created.
type PullRef struct {
	Number int
	NodeID string
	URL    string
}

// ReviewInput is a head-pinned review submission. Event is the forge-neutral verb; the
// implementation maps it to the wire event (APPROVE / REQUEST_CHANGES / COMMENT).
type ReviewInput struct {
	HeadSHA string // pins the verdict to the reviewed head
	Event   string // APPROVE | REQUEST_CHANGES | COMMENT
	Body    string
}

// IssueInput is the request to file an issue.
type IssueInput struct {
	Title string
	Body  string
}

// IssueRef identifies an issue that was just filed.
type IssueRef struct {
	Number int
	URL    string
}

// PushTransport describes how a minted token authenticates a git push to this forge — the
// "push-transport hints" of spec §6. It carries NO secret: only the host and the scheme by
// which a token (supplied out of band, via a 0600 file read by an inline credential helper)
// authenticates. It exists so a tool that pushes as an App/service-account identity does not
// hardcode a forge's transport shape.
type PushTransport struct {
	// RemoteHost is the git host for this forge (e.g. "github.com").
	RemoteHost string
	// TokenUsername is the username half of a token-authenticated https remote — for a
	// GitHub App installation token this is "x-access-token".
	TokenUsername string
	// CredentialHelperHint names the safe way to supply the token: an inline
	// credential.helper reading the token file, NEVER a token-in-URL (which is
	// classifier-blocked and leaks the secret into process argv / reflog).
	CredentialHelperHint string
}

// Forge is the single seam every desk tool reaches a forge through. The method set is the
// operations a shipping tool consumes (stream spec §6), reconciled against the stream's
// per-tool inventory. It is FROZEN: an addition requires a consuming tool in the same
// change.
type Forge interface {
	// --- Reads ---

	// GetPullRequest reads a change's head/state/draft/node-id/changed-file-count/author.
	GetPullRequest(repo ForgeRepo, number int) (*PullRequest, error)
	// GetIssue reads an issue and answers whether the number is in fact a pull request.
	GetIssue(repo ForgeRepo, number int) (*Issue, error)
	// ReviewsAtHead returns every review on a change (paginated to exhaustion).
	ReviewsAtHead(repo ForgeRepo, number int) ([]Review, error)
	// ListChangedFiles returns a change's file entries (paginated, rename-aware). The
	// caller reconciles len against PullRequest.ChangedFiles before trusting it complete.
	ListChangedFiles(repo ForgeRepo, number int) ([]ChangedFile, error)
	// ChecksAtHead returns the CI rollups at a commit, each with its asserted total count.
	ChecksAtHead(repo ForgeRepo, sha string) (*ChecksAtHead, error)
	// IssueReactions returns the reactions/awards on an issue or PR (the admission gate
	// surface: reaction ↔ award emoji).
	IssueReactions(repo ForgeRepo, number int) ([]Reaction, error)
	// RepoVisibility returns the repo's visibility (private | public | internal | ...).
	RepoVisibility(repo ForgeRepo) (string, error)

	// --- Writes ---

	// CreateDraftChange opens a draft change (draft PR ↔ Draft: MR).
	CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error)
	// PostComment posts a plain comment on an issue or PR.
	PostComment(repo ForgeRepo, number int, body string) error
	// PostReview submits a head-pinned review/approval on a change.
	PostReview(repo ForgeRepo, number int, in ReviewInput) error
	// MarkReadyForReview flips a draft change to ready (the only transition this seam
	// exposes — there is no un-ready, merge, or edit).
	MarkReadyForReview(nodeID string) error
	// FileIssue files a new issue.
	FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error)
	// CloseIssue closes an issue with an optional state reason.
	CloseIssue(repo ForgeRepo, number int, stateReason string) error
	// DeleteRef deletes one git ref from a repo. The ref is a REF PATH inside the repo's
	// own ref namespace ("heads/topic", "dispatch/<key>"), never an API path: it is
	// validated by ValidateRefPath before any request is built, so this op cannot be used
	// to address an arbitrary endpoint. Deleting a ref that is already gone is reported as
	// a not-found error the caller may treat as a no-op — the seam does not decide that.
	DeleteRef(repo ForgeRepo, ref string) error

	// --- Identity / transport ---

	// PushTransportHint returns how a minted token authenticates a push to this forge. It
	// carries no secret (see PushTransport).
	PushTransportHint(repo ForgeRepo) PushTransport
}
