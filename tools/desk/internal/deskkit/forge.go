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

import "strings"

// GitHubAPIBase is the single home of the GitHub REST/GraphQL host literal. Desk commands
// that keep a package-level test hook (`var apiBaseURL = GitHubAPIBase`) source their
// default from here so the literal is constructed in exactly one place — the forge module —
// and never in a cmd package (the "no direct API construction outside the forge
// implementation" contract).
const GitHubAPIBase = "https://api.github.com"

// GitHubBaseURLOrDefault resolves an override base to a concrete GitHub host: the override
// when non-empty, else GitHubAPIBase. It is the ONE place the "" → default resolution lives,
// so a desk command that keeps a raw REST reader whose reads have no typed Forge op
// (deskpost's ghClient — contents, commit-author, the trust GraphQL query and the present-
// label set are not interface operations) can hold an EMPTY host override in production and
// still source its concrete host from the forge module, never binding the literal in a cmd
// package. GitHubForge.baseURL() resolves through here too, so the two cannot drift.
func GitHubBaseURLOrDefault(base string) string {
	if base != "" {
		return base
	}
	return GitHubAPIBase
}

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
	// Mergeable is the forge's own merge-conflict verdict, normalised to ONE of three
	// values and never to a bool: MERGEABLE, CONFLICTING, or UNKNOWN. The third is not a
	// tidy-up — it is the forge saying "I have not computed this yet", which is exactly the
	// state a caller must NOT read as mergeable (deskflip's `mergeable` condition refuses
	// could-not-check on it). A bool would have to collapse UNKNOWN into one of the other
	// two, and either collapse is wrong in a way that only shows up under load.
	//
	// Consumer: cmd/deskflip's `mergeable` condition (the freeze rule's same-change
	// requirement — this field lands with the call site that reads it).
	Mergeable string
	// Labels are the label NAMES currently on the change. Consumer: cmd/deskflip's
	// already-flipped no-op path, which must be able to tell "the queue label is already
	// correct" (write nothing) from "the queue label is stale" (re-gate, then write)
	// WITHOUT issuing a write to find out.
	Labels []string
	// URL is the change's human-facing page. Consumer: cmd/deskreply, which prints it as
	// the reply's location and records it in the audit detail.
	URL string
	// HeadRef is the SOURCE branch name (GitHub head.ref ↔ GitLab source_branch), as
	// distinct from HeadSHA. Consumer: cmd/deskreply's preflight, which refuses when the
	// worktree's checked-out branch is not the branch the change is built from.
	HeadRef string
	// BaseRef is the TARGET branch name (GitHub base.ref ↔ GitLab target_branch) — the branch
	// whose protection rules gate the merge. Consumer: cmd/deskflip's checks-green condition,
	// which reads the required status checks configured on THIS branch (RequiredStatusChecks)
	// to tell "no CI rollup because nothing is required" apart from "no rollup because the
	// required checks have not reported". Empty when the forge did not report a base branch,
	// which the consumer treats as could-not-check rather than as "no branch".
	BaseRef string
	// UpdatedAt is the forge's own last-modified timestamp, RFC3339, empty when the forge
	// did not report one. Consumed by the loopengine liveness taxonomy's PR-activity probe
	// (see internal/loopengine/probes.go) — every other existing consumer of PullRequest
	// predates this field and reads none of it, so its addition changes no existing
	// behavior.
	UpdatedAt string
	// Body is the change's description text — the source of the one link trailer
	// (`Brief: <stream>/<NN>` / `Issue: #<N>`). Consumer: cmd/deskflip's security lane,
	// which resolves the owning brief from the trailer and consults the brief's own
	// gate/risk frontmatter (BriefRiskFromBody) as an additive risk-classification term.
	// omitempty keeps a bodyless change byte-identical in the forge golden corpus.
	Body string `json:",omitempty"`
}

// The three values PullRequest.Mergeable takes. They are constants rather than free strings
// because a caller SWITCHES on them, and a switch over free strings falls through to its
// default on a typo — which for this field means "unknown", i.e. a refusal, on a PR that was
// perfectly mergeable.
const (
	Mergeable            = "MERGEABLE"
	MergeableConflicting = "CONFLICTING"
	MergeableUnknown     = "UNKNOWN"
)

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
//
// CreatedAt is the entry's only recency stamp, and it is load-bearing rather than
// decorative: a caller reducing a rollup to the LATEST run per context (branch protection's
// own rule) has nothing else to order two runs of one context by, and a reducer with no
// stamp silently keeps whichever entry the forge happened to list last. Consumer:
// cmd/deskflip's latest-run-per-name reduction.
type StatusContext struct {
	State     string // success | pending | failure | error
	Context   string
	CreatedAt string // RFC3339, "" when the forge reported none
}

// CheckRun is one entry of the check-runs rollup.
//
// StartedAt/CompletedAt are the recency stamps the latest-run-per-name reduction orders by
// (see StatusContext.CreatedAt). A run carrying NEITHER — a freshly queued run the forge has
// not stamped — sorts OLDEST at the consumer, which is the fail-safe direction: a stampless
// queued orphan never supersedes a completed run. Consumer: cmd/deskflip.
type CheckRun struct {
	Name        string
	Status      string // queued | in_progress | completed
	Conclusion  string // success | failure | neutral | ...
	StartedAt   string // RFC3339, "" when the forge reported none
	CompletedAt string // RFC3339, "" when the forge reported none
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

// LabelSpec names a label plus the cosmetic metadata used ONLY when the label has to be
// created. The NAME is the load-bearing part everywhere; Color and Description are
// presentation and may be ignored by a backend whose forge does not carry them.
type LabelSpec struct {
	Name        string
	Color       string // 6 hex digits, no leading "#" (each backend renders its own form)
	Description string
}

// LabelChange is the label reconciliation requested on ONE change. It is a declarative
// request rather than a sequence of primitive calls, and that shape is deliberate: the two
// consuming call sites (deskflip's queue-label swap, deskpost's mechanical verdict labels)
// each need "ensure these exist and are on the change, and take these off" as ONE atomic
// intent, and expressing it as four primitives (create / list / add / remove) would put four
// operations on a frozen interface where the tools consume one.
//
// RemoveFamilies is what keeps the read out of the caller. A caller that had to LIST the
// current labels in order to decide which stale ones to drop would need a list operation of
// its own; instead it names the label-name PREFIXES it owns this run ("size:", "surface:"),
// and the backend removes every label carrying one of them that is not being added. A
// family the caller has no definite value for is simply not named, so nothing in it is
// touched — an absent signal removes nothing.
type LabelChange struct {
	// Add is ensured to exist on the repo/project and to be present on the change.
	Add []LabelSpec
	// Remove is taken off the change when present. A name that is not on the change is not
	// an error: removal is idempotent by construction.
	Remove []string
	// RemoveFamilies are label-name prefixes whose stale members are removed. A label
	// matching one of these prefixes that is ALSO in Add is kept.
	RemoveFamilies []string
}

// LabelOutcome reports what the reconciliation actually changed, so a caller can report the
// difference rather than restating its own intent.
type LabelOutcome struct {
	Added   []string
	Removed []string
}

// Comment is one comment/note on a change or issue.
//
// ID is OPAQUE, in the same sense as PullRequest.NodeID: on GitHub it is the GraphQL node id
// the edit mutation takes, on GitLab it is a backend-minted id carrying the coordinates the
// note-update endpoint needs. A caller passes back what it was given and NEVER builds one.
//
// Minimized is GitHub's "hidden/collapsed" state. GitLab has no minimise feature at all, so
// its backend reports false — which is EXACT rather than an approximation: on an instance
// where nothing can be hidden, nothing is.
// URL is the comment's own permalink where the forge publishes one, and EMPTY where it does
// not. GitHub gives every comment a `url`; GitLab's notes API returns no per-note location at
// all, and a composed `<mr url>#note_<id>` would be this tree inventing an address the forge
// never asserted. An absent optional field reported as absent is the honest answer — a
// consumer that has nothing to print prints nothing, which is the case it already handles.
type Comment struct {
	ID         string
	DatabaseID int64 // the forge's own numeric id, for a human-readable reference
	Author     Account
	Body       string
	Minimized  bool
	CreatedAt  string
	URL        string
}

// CommentRef identifies a comment that was just posted. ID is the SAME opaque id
// ListComments reports and EditComment takes, so an upsert can post once and edit thereafter
// without a second read; URL is empty where the forge publishes no per-comment permalink
// (see Comment.URL).
type CommentRef struct {
	ID         string
	DatabaseID int64
	URL        string
}

// FileContent is a file read from a branch: the decoded bytes plus the forge's own opaque
// content id an update must cite to write safely.
type FileContent struct {
	// Content is the file's decoded bytes.
	Content []byte
	// SHA is the OPAQUE content id an update passes back so the write is refused if the file
	// moved underneath it — GitHub's Contents-API blob `sha`, GitLab's `last_commit_id`,
	// each filling the optimistic-lock role its own forge defines. A caller passes back what
	// it was given and NEVER composes one. Empty when the file does not yet exist on the ref.
	SHA string
	// Exists distinguishes "the path is absent on this ref" (false) from "the file is present
	// and empty" (true, Content == nil). A caller that must merge into an existing document
	// reads Exists, never len(Content).
	Exists bool
}

// ReadFileInput addresses one file at one ref. The fields are a struct rather than positional
// arguments so the interface method takes no parameter that reads as an endpoint address (the
// no-passthrough shape check keys on the interface's own parameter NAMES).
type ReadFileInput struct {
	// File is the repo-relative path ("docs/streams/…/brief.md"), never an API path.
	File string
	// Ref is the branch (or tag/sha) to read the file at.
	Ref string
}

// WriteFileInput is a request to write Content at File on Branch. It folds the properties a
// whole-file write needs to be safe — idempotency, the append-only shrink guard, and the
// branch-creation fallback — into the ONE operation, rather than spreading them across a read
// op, a ref-create op and a write op the freeze rule would each have to justify.
type WriteFileInput struct {
	// File is the repo-relative path to write.
	File string
	// Branch is the branch the write lands on.
	Branch string
	// Content is the whole new file content (a Contents-API PUT / Repository-Files write is
	// whole-file, not a patch).
	Content []byte
	// Message is the commit message.
	Message string
	// AppendOnly, when true, refuses a write that would leave the file with FEWER row-bearing
	// (non-blank) lines than the branch already holds — the stale-base/wrong-file clobber a
	// whole-file write would otherwise land as a success. The backend FETCHES the current file
	// and compares post-fetch; the constraint is passed in, the comparison is the backend's.
	AppendOnly bool
	// AllowShrink overrides AppendOnly when a row reduction is genuinely intended.
	AllowShrink bool
	// StartBranch, when set and Branch does not yet exist, creates Branch off StartBranch as
	// PART of the write (GitLab's Repository-Files `start_branch`). It exists so the
	// Evidence-lane fallback needs no separate CreateRef op on the frozen interface. Empty
	// means "write to Branch, which must already exist".
	StartBranch string
}

// WriteFileResult reports what a WriteFile actually did, so a caller can report the difference
// rather than restating its own intent — the same reason LabelOutcome and PullRef exist.
type WriteFileResult struct {
	// Changed is false when the branch already held byte-identical content: the write was an
	// idempotent noop and nothing was committed. The idempotency read is folded into WriteFile
	// so a caller does not issue its own read to find out.
	Changed bool
	// SHA is the resulting content id — the new blob/commit id when Changed, and the existing
	// one on a noop, so a caller always has an id to report.
	SHA string
	// Author is the git author the forge recorded on the write, when the forge reports one.
	// GitHub's Contents API returns commit.author.name (an App-token write records the App's
	// bot); GitLab's file write returns none, so Author is EMPTY there — which a caller reads
	// as could-not-check, never as a wrong identity. It is not guessed.
	Author string
	// DefaultBranchNotWritable is the sentinel: Branch was the forge's default branch and this
	// forge does not permit a direct write to it (GitLab's protected default — pilot D-8).
	// NOTHING was written and no write call was made. The caller lands the change on a side
	// branch (WriteFile with StartBranch) and opens a draft change instead. GitHub's default
	// branch is directly writable by the verifier App (the direct-main carve-out), so its
	// backend never sets this.
	DefaultBranchNotWritable bool
	// Rows and PriorRows are the row counts the shrink guard compared (0 when AppendOnly was
	// not set), so a caller can name the delta in its own success line.
	Rows      int
	PriorRows int
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

// forgeRowCount counts the row-bearing (non-blank) lines in b — the shrink guard's unit, so a
// file with or without a trailing newline reports the same count. It lives here, shared by both
// backends' WriteFile, so the append-only comparison is one definition rather than two that
// could drift.
func forgeRowCount(b []byte) int {
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
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
	// RequiredStatusChecks returns the status-check CONTEXTS branch protection REQUIRES on
	// the named branch of repo — the checks whose success the forge itself gates a merge on.
	// An EMPTY slice means the branch requires no status checks: nothing is configured to
	// block a merge, so an absent CI rollup is everything there will ever be. A branch with
	// no protection at all is empty, NOT an error — GitHub answers 404 for an unprotected
	// branch and GitLab answers with the pipeline-gating setting off, both meaning "nothing
	// required". A read that cannot DETERMINE the required set — a permission or transport
	// failure, an unreadable response, or a branch the caller could not name — returns an
	// error, which the caller treats as could-not-check and refuses: reading a green verdict
	// off "we could not learn what is required" is the fail-open this read exists to prevent.
	// Consumer: cmd/deskflip's checks-green condition (freeze rule: this read lands with the
	// call site that consumes it).
	RequiredStatusChecks(repo ForgeRepo, branch string) ([]string, error)
	// IssueReactions returns the reactions/awards on an issue or PR (the admission gate
	// surface: reaction ↔ award emoji).
	IssueReactions(repo ForgeRepo, number int) ([]Reaction, error)
	// ListLabelEvents returns the change's label-APPLICATION events — the label name AND
	// the actor that applied it. The applier is the whole point: it is what separates a
	// dispatcher's attestation from a self-applied stamp, so a read that returned only the
	// names would make the model-capability floor unenforceable.
	ListLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error)
	// ListComments returns the comments/notes on a change or issue, oldest first.
	ListComments(repo ForgeRepo, number int) ([]Comment, error)
	// RepoVisibility returns the repo's visibility (private | public | internal | ...).
	RepoVisibility(repo ForgeRepo) (string, error)
	// ReadFile reads a file's content at a ref (GitHub Contents API ↔ GitLab Repository Files
	// API). A path absent on the ref is a not-found error the caller tests with
	// IsForgeNotFound — the seam does not decide whether "absent" is an error, because for a
	// merge-into-existing it is and for a first-write it is not.
	ReadFile(repo ForgeRepo, in ReadFileInput) (*FileContent, error)

	// --- Writes ---

	// CreateDraftChange opens a draft change (draft PR ↔ Draft: MR).
	CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error)
	// PostComment posts a plain comment on an issue or PR and returns a reference to what
	// it created. The reference is what makes the write ANSWERABLE — a caller can report
	// where the comment landed and, for an upsert, hold the id it will edit next time —
	// which is the same reason CreateDraftChange returns a PullRef and FileIssue an
	// IssueRef. A write whose only output is "no error" leaves its caller re-reading the
	// listing to find out what it just did.
	PostComment(repo ForgeRepo, number int, body string) (*CommentRef, error)
	// PostReview submits a head-pinned review/approval on a change.
	PostReview(repo ForgeRepo, number int, in ReviewInput) error
	// MarkReadyForReview flips a draft change to ready (the only transition this seam
	// exposes — there is no un-ready, merge, or edit).
	MarkReadyForReview(nodeID string) error
	// ApplyLabels reconciles a change's labels in one operation (see LabelChange). It is
	// idempotent: applying an already-present label and removing an already-absent one are
	// both no-ops, so a re-run REPLACES rather than stacks and never fails for having
	// already succeeded.
	ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error)
	// EditComment replaces the body of ONE existing comment. commentID is the opaque id a
	// prior ListComments returned (Comment.ID) — never a locally composed one.
	EditComment(repo ForgeRepo, commentID, body string) error
	// FileIssue files a new issue.
	FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error)
	// CloseIssue closes an issue with an optional state reason.
	CloseIssue(repo ForgeRepo, number int, stateReason string) error
	// WriteFile writes a file's whole content at a path on a branch (GitHub Contents API ↔
	// GitLab Repository Files API), as the minted identity the backend holds. It folds three
	// properties into the one op (see WriteFileInput/WriteFileResult): an idempotency read
	// that returns Changed=false on byte-identical content and writes nothing; an append-only
	// shrink guard that refuses a row-reducing write post-fetch; and a default-branch
	// writability probe that returns the DefaultBranchNotWritable sentinel — writing nothing —
	// on a forge whose default branch takes no direct write, with StartBranch as the inline
	// branch-creation fallback so no separate CreateRef op is needed.
	WriteFile(repo ForgeRepo, in WriteFileInput) (*WriteFileResult, error)
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
