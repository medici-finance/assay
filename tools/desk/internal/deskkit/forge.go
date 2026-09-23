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

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
	Number int
	State  string // open | closed
	Draft  bool
	NodeID string // opaque id for the flip-draft mutation
	// Title is the change's title. Consumer: cmd/deskpr edit, whose idempotency noop needs the
	// CURRENT title to tell "the requested title already matches" (no write) from "the title
	// changes" (write + re-review comment) without a second read. EMPTY where the forge did not
	// report one. omitempty keeps a change read that carried no title byte-identical in the
	// forge golden corpus. On GitLab the title carries the `Draft:` prefix verbatim (the forge's
	// own rendering), the same way GetPullRequest surfaces every other field as the forge reports it.
	Title        string `json:",omitempty"`
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
	// MergedAt is the forge's own merge timestamp (RFC3339), set ONLY when the change was
	// merged and EMPTY for an open or closed-unmerged change. State alone cannot carry this:
	// a merged change and a closed-unmerged change both report State=="closed" on GitHub, and
	// the distinction is load-bearing where an irreversible action keys on it (deskboard's
	// #209 tombstone lane: a closed-unmerged PR wearing a MERGED label closes issues whose fix
	// never landed). A caller reads a NON-EMPTY MergedAt as "merged", an empty one on a
	// State=="closed" change as "closed-unmerged", and never infers merge from State.
	// Consumer: cmd/deskboard's fetchPRState (freeze rule: this field lands with its consumer).
	// omitempty keeps an open/closed change byte-identical in the forge golden corpus.
	MergedAt string `json:",omitempty"`
	// Merged is the forge's own merged FLAG, independent of MergedAt: GitHub reports a merged
	// change as merged:true with merged_at USUALLY but not always populated (a merged PR can
	// carry a null merged_at), so a reader keying on MergedAt alone would tombstone a genuinely
	// merged change as closed-unmerged (#400 N1). A caller reads "merged" as `Merged ||
	// MergedAt != ""`. Consumer: cmd/deskboard's fetchPRState. omitempty keeps a non-merged
	// change byte-identical in the forge golden corpus.
	Merged bool `json:",omitempty"`
	// GitLabMergeStatus is the forge's OWN raw merge-status string (GitLab
	// `detailed_merge_status`) where the forge reports one FINER-GRAINED than the tri-state
	// Mergeable above. It exists for exactly one consumer: cmd/deskflip's `mergeable`
	// condition, which on GitLab must tell a genuine "not computed yet" (`checking`,
	// `unchecked`, an empty string) apart from the two NAMED policy holds this brief's
	// merge-hold op set is about to release (`draft_status`, `discussions_not_resolved`) —
	// a distinction the three-value Mergeable enum, BY DESIGN, collapses away
	// (gitlabMergeableState's own doc comment says so, and Verify row 7 pins that mapping
	// unchanged). EMPTY on GitHub, and on every GitLab read this field predates; omitempty
	// keeps every existing golden fixture byte-identical. Consumer: cmd/deskflip's
	// `mergeable` condition (the forge-gitlab merge-hold brief; freeze rule binds METHODS, not fields, so this
	// addition changes no method count).
	GitLabMergeStatus string `json:",omitempty"`
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

// --- Merge-hold marker thread (the forge-gitlab merge-hold brief) ---
//
// On GitLab Free neither half of GitHub's server-side verdict-before-merge gate exists: the
// `Draft:` prefix is a title string any Developer can strip, and required approvals are
// Premium. GitLab DOES enforce, on every tier, that a merge request with an unresolved
// discussion thread cannot be merged (`only_allow_merge_if_all_discussions_are_resolved`,
// a plain project setting). This op set makes a resolvable discussion thread the desk's own
// merge hold: opened with the change, released only by the reviewer's approve verdict at the
// current head, re-armed by a request-changes verdict or a new head. GitHub's twin of this
// control is server-side branch protection, already stronger, so its implementation of every
// op here is the typed not-applicable a caller skips.

// MergeHold is the read of a change's merge-hold marker thread. See Forge.ReadMergeHold.
type MergeHold struct {
	// State is one of the MergeHold* constants below.
	State string
	// ID is the hold's opaque id (the GitLab discussion id) — "" unless State is RESOLVED or
	// UNRESOLVED.
	ID string
	// ResolvedBy is the login the FORGE records as having resolved the marker note itself
	// (GitLab Note.ResolvedBy) — populated only when State is RESOLVED. This is a signal
	// independent of who authored the RELEASED reply below: a Developer who resolves the
	// thread by hand (GitLab lets any Developer resolve any thread) sets THIS field to their
	// own login without ever posting a reply, which is exactly the hand-resolve
	// cmd/deskflip's reviewer-approved condition must refuse.
	ResolvedBy string
	// Head is the full commit sha named on the RELEASED reply's `Head:` line — populated
	// only when State is RESOLVED, and EMPTY when a resolved thread carries no such reply (a
	// hand resolve, never touched by SetMergeHold). Consumer: cmd/deskflip's
	// reviewer-approved condition, which refuses a resolved thread whose Head does not equal
	// the change's CURRENT head — a resolve that never named a head can never equal one.
	Head string
}

const (
	// MergeHoldNotApplicable is ReadMergeHold's answer on a forge whose server-side twin of
	// this control is something else entirely (GitHub: branch protection's required
	// reviewer-App review) — never a lack of data, a genuine typed "there is nothing here to
	// read". Consumer: cmd/deskflip's reviewer-approved condition, which reads this as "run
	// the original note/approval-based correctness lane instead", never as an absent hold on
	// a forge that does have a real one.
	MergeHoldNotApplicable = "NOT_APPLICABLE"
	// MergeHoldAbsent means the forge HAS the concept but this change carries no marker
	// thread — CreateDraftChange's caller never opened one (task 2 makes that a loud,
	// non-zero failure rather than a silent gap), or the change predates this brief.
	MergeHoldAbsent = "ABSENT"
	// MergeHoldUnresolved means the marker thread exists and is still open — no reviewer has
	// released it at the current head.
	MergeHoldUnresolved = "UNRESOLVED"
	// MergeHoldResolved means the marker thread is resolved; ResolvedBy and Head narrow
	// further whether THIS resolution is the one a caller may act on.
	MergeHoldResolved = "RESOLVED"
)

// MergeHoldUpdate is SetMergeHold's argument: release (Resolved:true, at Head) or re-arm
// (Resolved:false, naming Reason) a change's merge-hold, with the reply body text the hold's
// next Read finds again (see the marker-reply shapes in forge_gitlab.go).
type MergeHoldUpdate struct {
	// Resolved selects release (true) or re-arm (false).
	Resolved bool
	// Head is the full commit sha the release names on the reply's `Head:` line. Required
	// (and validated non-empty by the implementation) when Resolved is true; ignored
	// otherwise.
	Head string
	// Reason is the re-arm reply's own second line — "request-changes" or "new head <sha>"
	// (see the brief's marker-reply facts). Required when Resolved is false; ignored
	// otherwise.
	Reason string
}

// ErrMergeHoldNotApplicable is OpenMergeHold's and SetMergeHold's typed not-applicable — the
// write-side twin of MergeHoldNotApplicable, needed because neither returns a MergeHold value
// with a State field to carry it. Test with IsMergeHoldNotApplicable, never errors.Is
// directly — the same indirection every other typed sentinel on this seam uses (IsForgeNotFound,
// IsForgeEmptyRepo).
var ErrMergeHoldNotApplicable = errors.New("merge-hold: not applicable on this forge")

// IsMergeHoldNotApplicable reports whether err is, or wraps, ErrMergeHoldNotApplicable.
func IsMergeHoldNotApplicable(err error) bool { return errors.Is(err, ErrMergeHoldNotApplicable) }

// Issue is the subset of an issue the desk tools read. IsPullRequest is the discriminator:
// GitHub serves issues and PRs from one number sequence and the issues endpoint carries a
// pull_request sub-object exactly when the number is a PR (GitLab keeps issues and MRs in
// separate sequences — the mapping resolves per implementation).
type Issue struct {
	Number        int
	State         string // open | closed
	Author        Account
	IsPullRequest bool
	// Title is the issue's title. Consumer: cmd/issueboard's RETIRE-row state read, which
	// positively reads the State (and this title) of an issue absent from the open list —
	// RETIRE rests on that `closed` read, never on the absence alone (#1032); the title is
	// the only best-effort part (a placeholder string stands in for an empty one). omitempty
	// keeps a change that carries no title byte-identical in the forge golden corpus.
	Title string `json:",omitempty"`
	// URL is the issue's human-facing page. Consumer: cmd/deskfile's attach path, which reads
	// the target issue (GetIssue) and prints/records its location — and refuses attaching to a
	// CLOSED target citing that URL. GetIssue carried no URL before the write-verbs migration
	// (#691), so the attach path had to re-read it separately; the field is added so a single GetIssue
	// answers "which kind, what state, and where". EMPTY where the forge reported none. omitempty
	// keeps an issue that carries no URL byte-identical in the forge golden corpus.
	URL string `json:",omitempty"`
	// Labels are the label NAMES currently on the issue/PR. Consumer: cmd/deskclose's absolute
	// decision-label gate (an item carrying `needs-decision`/`human-decided` is never closeable
	// by a sweep), which reads them from the SAME single GetIssue that answers the item's kind
	// and state — so an unread label set can never be mistaken for an empty one. omitempty keeps
	// an issue that carries no labels byte-identical in the forge golden corpus.
	Labels []string `json:",omitempty"`
	// Body is the issue/PR description text. Consumer: cmd/deskclose's review-request lane, which
	// extracts the single PR reference from a review-request issue's body. omitempty keeps a
	// bodyless issue byte-identical in the forge golden corpus.
	Body string `json:",omitempty"`
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
	// ID is the forge's own identifier for this RUN — GitHub's check-run id, GitLab's
	// pipeline-job id — rendered as a string so the interface stays forge-neutral about
	// how each forge numbers them. It identifies one EXECUTION, not the check: a re-run of
	// the same named check is a different ID, which is exactly the property deskflip's
	// check-only-CR exemption needs when a reviewer cites "the run that turned green".
	// "" when the forge served none.
	ID          string
	Name        string
	Status      string // queued | in_progress | completed
	Conclusion  string // success | failure | neutral | ...
	StartedAt   string // RFC3339, "" when the forge reported none
	CompletedAt string // RFC3339, "" when the forge reported none
}

// checkRunID renders a forge's numeric run id as the interface's string ID.
//
// A ZERO id renders as "" rather than "0", and that is fail-closed on purpose: an absent
// id is not an id. Rendering it as "0" would let a review body citing the literal string
// `0` match every run the forge served no identifier for — the one direction of this
// mapping that could GRANT something (see deskflip's check-only-CR exemption, which
// matches a cited id against this field and treats "" as unmatchable).
func checkRunID(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
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
	// Report, when set, receives a human-readable note about a write that SUCCEEDED by a
	// route other than the plain one — the verdict is in force, nothing is refused, but the
	// caller should say what the forge actually did. It is never called on an error path:
	// a note accompanies a nil return only. The GitLab backend uses it when POST /approve
	// answers 401 for an approval this identity already holds (#1106); the GitHub backend
	// never calls it. A nil Report drops the note.
	Report func(note string)
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

// EditChangeInput is the request to replace a change's OWN title/body text — the change's
// description, not a comment on it. A field left EMPTY is not sent, so a body-only edit
// (deskpr edit's case) does not blank a title, and a title-only edit does not blank a body.
// The fields are a struct rather than positional arguments so the interface method takes no
// parameter that reads as content-vs-coordinate ambiguity, and so a later field (a labels or
// state edit) is an additive struct field rather than an interface-signature change.
type EditChangeInput struct {
	Title string
	Body  string
}

// SearchIssuesInput is a free-text dedupe search over ONE repo's issues. Query is the
// already-tokenised free text (deskfile's matcher strips it to `[a-z0-9]` runs so no token can
// be read as a forge search qualifier); the backend scopes it to the repo — the caller never
// supplies a `repo:` qualifier or any other endpoint-shaping syntax. It is a struct field, not
// an interface method PARAMETER, precisely so the no-passthrough shape check (which keys on the
// interface's own parameter names) does not read a `query` argument as an endpoint address.
type SearchIssuesInput struct {
	Query string
}

// IssueSearchResult is one issue matched by SearchIssues: the fields deskfile's dedupe scores
// and reports. URL is the issue's human-facing page (EMPTY where the forge reported none) — the
// dedupe path prints it as the candidate's location, which is why the search shape carries a URL
// even though GetIssue only gained one in the same change (#691). It is a SEARCH result, never a
// change: a forge that serves issues and changes from one number sequence (GitHub) filters the
// changes out, so a caller de-duping issues never has to.
type IssueSearchResult struct {
	Number int
	Title  string
	State  string // open | closed
	Labels []string
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
	// Target says WHAT is being labelled — an issue or a change (PR/MR) — and is REQUIRED:
	// a change that leaves it unset is refused by every backend before any request is
	// issued. The seam cannot infer it from the number. GitHub numbers issues and pull
	// requests in one sequence and labels both through the issues endpoint, so there the
	// distinction costs nothing; GitLab numbers issues and merge requests in two SEPARATE
	// sequences with two separate endpoints, so a label write that assumed "change" landed
	// on whichever merge request happened to share the new issue's iid — the defect that
	// left every `deskfile new` issue on a GitLab project unstamped. Refusing an unset
	// target on BOTH forges is deliberate: a caller that forgot it would otherwise pass
	// every GitHub test and reproduce that defect only on GitLab. The type is the same
	// TargetKind the typed reads/comments take, so one stated kind serves every op.
	Target TargetKind
	// Add is ensured to exist on the repo/project and to be present on the target.
	Add []LabelSpec
	// Remove is taken off the change when present. A name that is not on the change is not
	// an error: removal is idempotent by construction.
	Remove []string
	// RemoveFamilies are label-name prefixes whose stale members are removed. A label
	// matching one of these prefixes that is ALSO in Add is kept.
	RemoveFamilies []string
}

// requireTarget is the shared refusal every backend issues BEFORE its first request when a
// LabelChange names no target. One helper rather than two copies so the two backends cannot
// drift on which values are accepted.
func (c LabelChange) requireTarget() error {
	switch c.Target {
	case TargetChange, TargetIssue:
		return nil
	default:
		return Refused(fmt.Sprintf("refusing to apply labels with no target kind (LabelChange.Target=%q) — "+
			"say whether the number is an issue or a change; on GitLab the two are separate sequences", string(c.Target)))
	}
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

// RollupNode is one entry of a change's CI status-check rollup, carrying BOTH of the two
// shapes GitHub's statusCheckRollup unions: a CheckRun node (Status+Conclusion, the Actions
// shape) and a StatusContext node (State, the legacy commit-status shape every non-Actions CI
// still posts). Typename says which. It is a UNION carried as one struct rather than two typed
// arms because the consumer (cmd/deskboard's ciState reducer) already switches on which fields
// are populated; forcing an either/or here would only move that switch, not remove it. The
// recency stamps (StartedAt/CompletedAt for a CheckRun, CreatedAt for a StatusContext) are the
// keys the latest-run-per-name reduction orders by — an entry carrying none sorts oldest, the
// fail-safe direction (a stampless queued orphan never supersedes a completed run).
type RollupNode struct {
	Typename    string // "CheckRun" | "StatusContext" (empty when the forge did not tag it)
	Name        string // CheckRun name
	Status      string // CheckRun: queued | in_progress | completed
	Conclusion  string // CheckRun: success | failure | ...
	StartedAt   string // CheckRun recency stamp, RFC3339, "" when none
	CompletedAt string // CheckRun recency stamp, RFC3339, "" when none
	Context     string // StatusContext name
	State       string // StatusContext: success | pending | failure | error | expected
	CreatedAt   string // StatusContext recency stamp, RFC3339, "" when none
}

// OpenChange is one open change (PR ↔ MR) in the bulk board read, carrying the fields the
// desk's cross-repo board classifies on AND its CI status-check rollup, so the board reads a
// whole repo's queue in one call rather than N+1 per-PR reads. Author.Login is the RENDERED
// login the trust set expects (a bot carries the "<slug>[bot]" suffix), matching the
// per-forge rendering GetPullRequest already applies. MergeStateStatus is the forge's own
// merge-readiness enum verbatim (GitHub's mergeStateStatus: BEHIND/BLOCKED/CLEAN/DIRTY/
// UNKNOWN/…) — the consumer reads its four-state mergeVerdict off it, and an empty value is
// could-not-check, never mergeable. Consumer: cmd/deskboard's fetchOpenPRs → prBase.
type OpenChange struct {
	Number           int
	Title            string
	Body             string
	State            string // OPEN | CLOSED | MERGED
	Draft            bool
	CreatedAt        string // RFC3339
	LastEditedAt     string // RFC3339, "" until the title/body is first edited
	Author           Account
	Labels           []string
	HeadSHA          string
	HeadRef          string
	BaseRef          string
	MergeStateStatus string
	Rollup           []RollupNode
}

// OpenChanges is the result of the bulk open-change read: the changes plus whether the read
// came back exactly at the page cap, so the board can state a TRUNCATED population in-band
// rather than printing a confident count over an unknown remainder (an absence that reads
// like an answer). Consumer: cmd/deskboard's fetchOpenPRs, which maps TruncatedAtCap onto its
// Header.PRPopulation.
type OpenChanges struct {
	// Changes are the open changes, newest first, up to Cap of them.
	Changes []OpenChange
	// TruncatedAtCap is true when the read returned exactly Cap changes — the forge may be
	// holding more, so every count derived from Changes is a FLOOR, not a total.
	TruncatedAtCap bool
	// Cap is the page cap the read was bounded to (the `first:`/`per_page` ceiling).
	Cap int
}

// ChangeStates is the set of change lifecycle states a ListChanges read is scoped to. It is a
// struct of booleans rather than a slice so an EMPTY request is a compile-visible zero value
// the op refuses (rather than a nil slice that could read as "all"): a states-less read is a
// caller bug, never a silent whole-repo scan. Consumer: the phantom / already-represented
// reconciliation (deskkit.RepresentedPRRefs), which asks for Open+Merged.
type ChangeStates struct {
	Open   bool
	Merged bool
	Closed bool
}

// OpenAndMerged is the ChangeStates the represented-PR reconciliation uses: a brief is
// represented by an OPEN or a MERGED PR, and a CLOSED-unmerged one represents nothing (its work
// was abandoned), so the closed state is deliberately NOT requested.
func OpenAndMerged() ChangeStates { return ChangeStates{Open: true, Merged: true} }

// Any reports whether at least one state is requested. A read with none requested is refused by
// the backends before any request is built — the whole point of the struct is that the state
// set was STATED.
func (s ChangeStates) Any() bool { return s.Open || s.Merged || s.Closed }

// Want reports whether an uppercased forge state (OPEN | MERGED | CLOSED) is in the requested
// set. The backends filter with it so a forge query that cannot express the exact set (GitLab's
// single-state `state=` param) can over-request and narrow client-side to exactly what was asked.
func (s ChangeStates) Want(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "OPEN":
		return s.Open
	case "MERGED":
		return s.Merged
	case "CLOSED":
		return s.Closed
	}
	return false
}

// ChangeRef is one change (PR ↔ MR) in a ListChanges read, reduced to the facts the
// already-represented / phantom reconciliation and a scheduler's routing need: the NUMBER, the
// lifecycle STATE (OPEN | MERGED | CLOSED — MERGED kept DISTINCT from CLOSED, unlike the board's
// OpenChange which collapses both to a single closed word, because the reconciliation routes a
// merged brief differently from an abandoned one), the head SHA and source BRANCH, the TITLE and
// BODY (the body carries the `Brief:`/`Issue:` link trailers BriefRepresentedPR matches on), and
// MergedAt for a merged change. It deliberately carries NO CI rollup or merge-state enum: those
// are the board's concern (OpenChange), and requesting them here would drag in the actions-scope
// dependency and the per-change pipeline read the reconciliation has no use for.
type ChangeRef struct {
	Number   int
	State    string // OPEN | MERGED | CLOSED
	HeadSHA  string
	HeadRef  string // source branch
	Title    string
	Body     string
	MergedAt string // RFC3339, "" unless State == MERGED
}

// ChangeList is the result of a ListChanges read: the changes plus whether the bounded walk was
// able to reach the end of the population. Incomplete=true means the page-count ceiling
// (PageCap) was hit with the forge still reporting more — so a consumer that reads ABSENCE from
// this list as evidence (the phantom check reads "no representing PR here" as "safe to
// dispatch") must treat the absence as could-not-check, never as a confident negative. The
// changes are ordered MOST-RECENTLY-UPDATED FIRST, so the bounded window is the changes most
// likely to represent a currently-queued brief; an older change beyond the window is exactly the
// one the reconciliation has least reason to find. Incomplete is reported, never hidden: a
// truncated read is stated as truncated, not handed back as if it were the whole population.
type ChangeList struct {
	// Changes are the changes matching the requested states, most-recently-updated first, up to
	// the page-count ceiling.
	Changes []ChangeRef
	// Incomplete is true when the walk hit PageCap pages with the forge still paginating — the
	// population is larger than what Changes holds.
	Incomplete bool
	// PageCap is the page-count ceiling the walk was bounded to.
	PageCap int
}

// IssueSummary is one open issue in the bulk issue-board read: the fields the issue lane
// classifies on. Author.Login is the RENDERED login (a bot carries its "<slug>[bot]" suffix)
// and Author.ID the permanent numeric id the trust gate pins on. CreatedAt is the escalation
// clock's baseline (the question was posed then). Consumer: cmd/issueboard's fetchOpenIssues.
// The read returns ISSUES only, never changes (PRs/MRs): a forge that serves both from one
// number sequence (GitHub) filters the changes out, so the caller never has to.
type IssueSummary struct {
	Number    int
	Title     string
	Author    Account
	Labels    []string
	CreatedAt string // RFC3339
	// URL is the issue's human-facing page, EMPTY where the forge did not report one.
	// Consumer: cmd/deskboard's cmdQueue, which prints the verify-gate issue's location in
	// its JSON row. omitempty keeps a change that carries no URL byte-identical in the forge
	// golden corpus. It is the only field cmdQueue needs beyond what the issue-board summary
	// already carried, so the queue lane reuses ListOpenIssues (label-filtered client-side)
	// rather than growing a redundant label-scoped list op.
	URL string `json:",omitempty"`
}

// TrustPayload is the parsed result of a trust-gate content-events read: the item's
// body-edit time, its content events (comments/reviews with author identity and edit
// times), and whether the read was COMPLETE. Complete=false means a comment/review
// connection overflowed the single bounded page the gate reads — the item is treated as NOT
// blessed (fail closed), never silently admitted off a partial thread. The events carry the
// numeric author id a recycled login cannot fake and the lastEditedAt a REST updated_at could
// not (it moves on unrelated events). Consumers: cmd/deskboard (prBlessed/issueBlessed),
// cmd/issueboard (the trust gate + escalation clock), cmd/scanloop (the queueing trust gate).
type TrustPayload struct {
	BodyEdited time.Time
	Events     []ContentEvent
	Complete   bool
}

// RepoCommit is one commit on a repository's default branch or at a ref: its sha, the
// committed date, and the forge accounts the commit is ATTRIBUTED to. AuthorLogin/
// CommitterLogin are the RENDERED account logins ("<slug>[bot]" for an App) the forge
// resolved the commit's author/committer email to — an identity comparable to a change
// author's login — and are EMPTY where the forge attributes the commit to no account, which
// a caller reads as UNKNOWN attribution, never as "not the author". GitLab commit payloads
// carry the raw git author/committer name+email but do NOT resolve them to an instance
// account, so its backend leaves these EMPTY (a per-field could-not-check, the same honest
// posture ReviewsAtHead takes on an unpinnable CommitID) while filling SHA and CommittedDate.
// Consumers: cmd/deskboard's fetchHeadCommit (the stall clock reads CommittedDate and the
// committer/author login) and fetchRecentCommits (branch-health reads only SHA) — freeze
// rule: these land with their call sites.
type RepoCommit struct {
	SHA            string
	CommittedDate  string // RFC3339, "" when the forge reported none
	AuthorLogin    string // rendered account login, "" when unattributed / not resolved
	CommitterLogin string // rendered account login, "" when unattributed / not resolved
}

// RefComparison is the two-dot/three-dot comparison of two refs: the files that differ, plus
// the divergence counts and the forge's own status vocabulary. BehindBy is how many commits
// `base` holds that `head` does not (the PR's "behind by" count); AheadBy the reverse. Status
// is the forge's own divergence word (GitHub: identical | ahead | behind | diverged) — EMPTY
// where the forge does not report one, which a caller reads as could-not-check rather than
// inventing a verdict. Consumers: cmd/deskboard's changedFilesBetween (the MERGE-CURR
// benign-merge check reads Files) and fetchBehindMain (the close-candidate hint reads BehindBy
// and refuses on an empty Status) — freeze rule: this lands with its call sites.
type RefComparison struct {
	Files    []ChangedFile
	AheadBy  int
	BehindBy int
	Status   string // "" when the forge reported none
}

// ChangeSearchResult is one open change (PR ↔ MR) found by an owner-wide search, carrying the
// repo it belongs to so a caller reconciling an owner's changes against a watched set can
// attribute each row. Consumer: cmd/deskboard's scope-reconciliation verb.
type ChangeSearchResult struct {
	Repo      string // "owner/name"
	Number    int
	Title     string
	CreatedAt string // RFC3339
}

// TargetKind names WHICH kind of numbered object a typed forge operation addresses: an
// ISSUE, or a CHANGE (pull request ↔ merge request). It exists for the forges that number
// the two kinds in separate sequences (GitLab), where a bare number resolves to nothing
// without it; on a single-sequence forge (GitHub) it is validated against what the number
// actually is. The zero value is deliberately NOT a kind: a caller that has not stated one
// gets a refusal from ParseTargetKind, never a default guessed on its behalf.
type TargetKind string

const (
	// TargetIssue addresses an issue.
	TargetIssue TargetKind = "issue"
	// TargetChange addresses a change — a pull request on GitHub, a merge request on GitLab.
	TargetChange TargetKind = "change"
)

// ParseTargetKind reduces a user-facing kind word to a TargetKind. It accepts the
// forge-neutral names (issue, change) and both forges' own words for a change (pr, mr),
// case-insensitively, so a flag can be spelled in whichever vocabulary the operator thinks
// in. Anything else — the empty string included — is refused (ExitRefused) naming the
// accepted set: the whole point of the type is that the kind was STATED, so an unparseable
// one must not quietly become an issue.
func ParseTargetKind(s string) (TargetKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "issue":
		return TargetIssue, nil
	case "change", "pr", "mr":
		return TargetChange, nil
	}
	return "", Refused(fmt.Sprintf("refused: unknown target kind %q (want one of: issue, mr; pr is an alias of mr)", s))
}

// ChangeSearchResults is the result of an owner-wide open-change search: the rows plus whether
// the read came back exactly at the page cap (TruncatedAtCap), so a caller can state a
// possibly-incomplete reconciliation in-band rather than treating a capped read as complete.
// Consumer: cmd/deskboard's cmdScope.
type ChangeSearchResults struct {
	Results        []ChangeSearchResult
	TruncatedAtCap bool
	Cap            int
}

// HardeningReadKind names one closed hardening-read kind for op 40, RepoHardeningRead — the
// enumerated replacement for repohardenguard's former arbitrary `gh api <endpoint>` reads
// (the forge-gitlab guard-read-custody brief). It is a named enum validated BEFORE any request exists
// (ValidateHardeningReadKind), never a path, the same DeleteRef/ValidateRefPath shape applied
// to a fixed vocabulary instead of a ref namespace: a kind the backend does not serve is a
// could-not-check REFUSAL naming the forge and the kind, never a guess and never the other
// forge's document.
type HardeningReadKind string

const (
	// HardeningReadRepo reads the repo document itself: `.visibility`,
	// `.security_and_analysis.*` (admin-visible only — a `null` here is could-not-check,
	// exactly as an unauthenticated/non-admin read reports today).
	HardeningReadRepo HardeningReadKind = "repo"
	// HardeningReadRulesets reads every ruleset's DETAIL document (the list→detail walk
	// happens inside the backend) as an ARRAY, so a caller's `[name=X].field` selector
	// resolves inside the returned array. `bypass_actors` is present only for a caller with
	// write access to the ruleset, so under a read-only identity those rows are
	// could-not-check.
	HardeningReadRulesets HardeningReadKind = "rulesets"
	// HardeningReadActionsWorkflowPermissions reads the default Actions workflow permissions
	// (admin-gated).
	HardeningReadActionsWorkflowPermissions HardeningReadKind = "actions-workflow-permissions"
	// HardeningReadActionsForkPRApproval reads the fork-PR contributor-approval setting
	// (admin-gated).
	HardeningReadActionsForkPRApproval HardeningReadKind = "actions-fork-pr-approval"
	// HardeningReadActionsPrivateForkPR reads the fork-PR-workflows-on-private-repos setting
	// (admin-gated).
	HardeningReadActionsPrivateForkPR HardeningReadKind = "actions-private-fork-pr"
	// HardeningReadVulnerabilityReporting reads the private-vulnerability-reporting setting
	// (admin read).
	HardeningReadVulnerabilityReporting HardeningReadKind = "vulnerability-reporting"

	// --- GitLab kinds (the forge-gitlab GitLab-hardening-reads brief). Each is ONE fixed
	// endpoint literal returning GitLab's OWN settings document — never a synthesised
	// GitHub-shaped view — so a checklist is written per forge. The GitHub backend refuses
	// every one of them by name, exactly as GitLab refuses the GitHub kinds above.

	// HardeningReadProject reads the project document (`GET /projects/:id`, Free):
	// `.visibility`, `.only_allow_merge_if_pipeline_succeeds`,
	// `.only_allow_merge_if_all_discussions_are_resolved`, `.ci_config_path`,
	// `.ci_allow_fork_pipelines_to_run_in_parent_project` (Owner/admin-visible only — absent at
	// any lower role, so a row on it is could-not-check there), `.secret_push_protection_enabled`
	// (Ultimate). It is also the GitLab preflight document (ForgeResolution.HardeningRepoDocumentKind).
	HardeningReadProject HardeningReadKind = "project"
	// HardeningReadProtectedBranches reads the protected-branches LIST
	// (`GET /projects/:id/protected_branches`, Free at role level; `user_id`/`group_id` entries
	// are Premium) as one ARRAY, every page walked, so a caller's `[name=main].field` selector
	// resolves inside it. `.push_access_levels.0.access_level == 0` ("No one") is the
	// CE-expressible form of an empty bypass list.
	HardeningReadProtectedBranches HardeningReadKind = "protected-branches"
	// HardeningReadProtectedTags reads the protected-tags LIST (`GET /projects/:id/protected_tags`,
	// Free at role level) as one ARRAY, every page walked.
	HardeningReadProtectedTags HardeningReadKind = "protected-tags"
	// HardeningReadPushRules reads the project push rule (`GET /projects/:id/push_rule`,
	// PREMIUM). On Community Edition the route answers 404/403, which arrives as a
	// could-not-check carrying a *ForgeAPIError — never an empty document; the CE checklist
	// records the row as `not available — Premium` and makes no request at all. On Premium a
	// project with no push rule configured answers the literal document `null`, handed back
	// as-is (an absence, which the guard's Gated cell classifies).
	HardeningReadPushRules HardeningReadKind = "push-rules"
	// HardeningReadApprovals reads the project approval configuration
	// (`GET /projects/:id/approvals`): `.reset_approvals_on_push`,
	// `.merge_requests_author_approval`, `.merge_requests_disable_committers_approval`. The read
	// answers 200 on gitlab.com Free but 404 on some self-managed CE; ENFORCEMENT of the
	// settings is Premium, so a CE checklist's Required cell records the advisory meaning.
	HardeningReadApprovals HardeningReadKind = "approvals"
)

// hardeningReadKinds is the closed vocabulary op 40 serves, in a stable declared order — the
// order HardeningReadKinds() renders and ValidateHardeningReadKind walks.
var hardeningReadKinds = []HardeningReadKind{
	HardeningReadRepo,
	HardeningReadRulesets,
	HardeningReadActionsWorkflowPermissions,
	HardeningReadActionsForkPRApproval,
	HardeningReadActionsPrivateForkPR,
	HardeningReadVulnerabilityReporting,
	HardeningReadProject,
	HardeningReadProtectedBranches,
	HardeningReadProtectedTags,
	HardeningReadPushRules,
	HardeningReadApprovals,
}

// hardeningKindForge records WHICH forge serves each kind. The vocabulary is one closed set
// and the per-forge halves are disjoint: a backend handed the other forge's kind refuses it
// BY NAME (zero requests), never answers with its own nearest document. Every entry of
// hardeningReadKinds has exactly one row here (TestForgeGitlabGolden's refusal cases and
// TestHardeningKindsPartitionByForge pin the partition).
var hardeningKindForge = map[HardeningReadKind]ForgeKind{
	HardeningReadRepo:                       ForgeGitHub,
	HardeningReadRulesets:                   ForgeGitHub,
	HardeningReadActionsWorkflowPermissions: ForgeGitHub,
	HardeningReadActionsForkPRApproval:      ForgeGitHub,
	HardeningReadActionsPrivateForkPR:       ForgeGitHub,
	HardeningReadVulnerabilityReporting:     ForgeGitHub,
	HardeningReadProject:                    ForgeGitLab,
	HardeningReadProtectedBranches:          ForgeGitLab,
	HardeningReadProtectedTags:              ForgeGitLab,
	HardeningReadPushRules:                  ForgeGitLab,
	HardeningReadApprovals:                  ForgeGitLab,
}

// HardeningReadKinds returns the closed vocabulary op 40 serves, as strings, in a stable
// order — for a caller (repohardenguard's checklist parser, an error message) that needs to
// name every valid kind without restating the enum.
func HardeningReadKinds() []string {
	out := make([]string, len(hardeningReadKinds))
	for i, k := range hardeningReadKinds {
		out[i] = string(k)
	}
	return out
}

// hardeningReadKindsFor returns the half of the vocabulary the named forge serves, as strings,
// in declared order — what a backend's by-name refusal lists so a checklist author sees the
// kinds THIS forge answers rather than the whole set. Unexported on purpose: no exported
// function in this package takes a forge selector (TestForgeForRejectsCallerSuppliedForge) —
// a backend names its OWN kind here, never a caller.
func hardeningReadKindsFor(served ForgeKind) []string {
	var out []string
	for _, k := range hardeningReadKinds {
		if hardeningKindForge[k] == served {
			out = append(out, string(k))
		}
	}
	return out
}

// HardeningReadKindForge reports which forge serves kind. An unknown kind reports "" — a
// caller validates with ValidateHardeningReadKind first.
func HardeningReadKindForge(kind HardeningReadKind) ForgeKind {
	return hardeningKindForge[kind]
}

// HardeningRepoDocumentKind returns the kind that reads the repository's own top-level
// document on the RESOLVED forge — `repo` on GitHub, `project` on GitLab. It is the guard's
// PREFLIGHT read (the read that proves the token can see the repo at all before any
// per-row absence is allowed to mean "unset"), read off the resolution ResolveForge handed
// back rather than hard-coded, so a GitLab-resolved run is not refused at the door by a
// GitHub kind. It hangs off ForgeResolution rather than taking a ForgeKind argument: no
// exported function in this package accepts a forge selector
// (TestForgeForRejectsCallerSuppliedForge), and the backend re-validates the kind by name
// regardless, so a resolution a caller fabricated buys a zero-request refusal from the
// forge that actually answers — never the other forge's document. A forge this package
// cannot name is a could-not-check refusal, never a guessed kind.
func (r ForgeResolution) HardeningRepoDocumentKind() (HardeningReadKind, error) {
	return hardeningRepoDocumentKind(r.Kind)
}

// hardeningRepoDocumentKind is the per-forge table behind
// ForgeResolution.HardeningRepoDocumentKind.
func hardeningRepoDocumentKind(served ForgeKind) (HardeningReadKind, error) {
	switch served {
	case ForgeGitHub:
		return HardeningReadRepo, nil
	case ForgeGitLab:
		return HardeningReadProject, nil
	default:
		return "", Unverifiable(fmt.Sprintf(
			"could-not-check: no hardening preflight document is defined for forge %q", served), nil)
	}
}

// refuseHardeningKindForForge is the by-name refusal both backends share for a kind the
// OTHER forge serves: validated (so an unknown kind still fails on ValidateHardeningReadKind's
// grounds, never confused with this one), then refused naming this forge, the kind, the
// forge that does serve it, and the kinds this forge answers — with ZERO requests emitted.
func refuseHardeningKindForForge(this ForgeKind, kind HardeningReadKind) error {
	serving := hardeningKindForge[kind]
	if serving == this {
		return nil
	}
	return Unverifiable(fmt.Sprintf(
		"could-not-check: %s serves no hardening read of kind %q — it is a %s kind; the %s kinds are: %s",
		this, kind, serving, this, strings.Join(hardeningReadKindsFor(this), ", ")), nil)
}

// ValidateHardeningReadKind checks kind against the closed vocabulary BEFORE any request is
// built — the DeleteRef/ValidateRefPath shape, applied to a fixed enum rather than a ref
// namespace. An unrecognised kind is a could-not-check REFUSAL naming the kind and the
// vocabulary; RepoHardeningRead calls this first on both backends, so an unknown kind emits
// ZERO requests on either.
func ValidateHardeningReadKind(kind string) (HardeningReadKind, error) {
	for _, k := range hardeningReadKinds {
		if string(k) == kind {
			return k, nil
		}
	}
	return "", Unverifiable(fmt.Sprintf(
		"could-not-check: %q is not a known hardening-read kind — the enumerated kinds are: %s",
		kind, strings.Join(HardeningReadKinds(), ", ")), nil)
}

// --- Run and gate-approval ops (RunWorkflow / ApproveGate / RunStatus) ---------------------
//
// These three ops start a CI run and clear a deployment gate on it. They were a human action
// until forge-neutral/14: GitHub ships ONE permission (`actions: write`) for dispatching a
// workflow and approving a pending deployment, and the same permission also cancels runs,
// deletes run logs and disables workflows repo-wide, so no desk App is safely grantable it.
// The ops therefore run ONLY under the per-repo run credential the roster binds
// (ResolveRunCredential, runcredential.go) — never a desk role's App. Their one consumer is
// cmd/deskrun (freeze rule: the ops land with that call site).
//
// `repository_dispatch` is deliberately NOT a trigger here: it fires with `contents: write`,
// a scope most desk Apps already hold, which is a far wider "who can start a release"
// surface than a roster-bound, single-purpose credential. No op in this seam posts to the
// repository dispatches endpoint.

// RunWorkflowInput is RunWorkflow's request.
type RunWorkflowInput struct {
	// Workflow names the workflow to run. On GitHub it is the workflow FILE name under
	// .github/workflows (e.g. "release.yml") or its numeric id — a bare name, never a path.
	// On GitLab a project has exactly one pipeline definition, so it is empty or the literal
	// ".gitlab-ci.yml"; anything else is refused rather than silently ignored.
	Workflow string
	// Ref is the branch or tag the run executes on ("main", "v1.2.0").
	Ref string
	// Inputs are the workflow_dispatch inputs (GitHub) / pipeline variables (GitLab).
	Inputs map[string]string
	// Actor, when non-empty, is the login the forge records as the run's actor (a GitHub App
	// renders as "<slug>[bot]"). GitHub's dispatch returns no run id, so the created run is
	// resolved by a follow-up list read; the actor narrows that read. Empty means the read is
	// narrowed by workflow, event, ref and the pre-dispatch time floor only — the ambiguity
	// refusal still stands. GitLab returns the pipeline directly and does not read it.
	Actor string
}

// RunRef is an OPAQUE handle on one run (a GitHub Actions workflow run ↔ a GitLab
// pipeline). ID is the forge's own run/pipeline id, URL its human-facing page. A caller
// never constructs or parses one; it passes back what RunWorkflow returned (or the id a
// human read off the forge, which the backend validates as a bare id before any request).
type RunRef struct {
	ID  string
	URL string `json:",omitempty"`
}

// GateShape names HOW a deployment gate is cleared. GitHub has one shape (a deployment
// environment with required reviewers); GitLab has two with no unifying endpoint, and which
// one a project uses is a property of that project's CI configuration — so the shape is
// STATED by the caller (from the roster's run-credential binding), never inferred.
type GateShape string

const (
	// GateShapeEnvironment is a protected/deployment environment approval: GitHub's
	// pending_deployments, GitLab's protected-environment deployment approval.
	GateShapeEnvironment GateShape = "environment"
	// GateShapeManualJob is a GitLab `when: manual` job played on the pipeline. GitHub has
	// no such shape and refuses it by name.
	GateShapeManualJob GateShape = "manual-job"
)

// ParseGateShape validates a stated gate shape. Empty is returned as empty (the caller did
// not state one); an unknown value is a could-not-check refusal naming the vocabulary.
func ParseGateShape(s string) (GateShape, error) {
	switch GateShape(strings.TrimSpace(s)) {
	case "":
		return "", nil
	case GateShapeEnvironment:
		return GateShapeEnvironment, nil
	case GateShapeManualJob:
		return GateShapeManualJob, nil
	}
	return "", Unverifiable(fmt.Sprintf("could-not-check: %q is not a gate shape — the shapes are %q and %q",
		s, GateShapeEnvironment, GateShapeManualJob), nil)
}

// ApproveGateInput is ApproveGate's request: the gate's NAME (a GitHub environment name, a
// GitLab environment name or manual job name) and its SHAPE.
type ApproveGateInput struct {
	Gate  string
	Shape GateShape
}

// Forge-neutral run lifecycle vocabulary (RunState.Status).
const (
	RunStatusQueued     = "queued"
	RunStatusInProgress = "in_progress"
	RunStatusWaiting    = "waiting"
	RunStatusCompleted  = "completed"
)

// RunState is a run's lifecycle in the forge-neutral vocabulary. Status is one of the
// RunStatus* values; Conclusion is empty until Status is completed, then the forge's
// outcome (success, failure, cancelled, skipped, and on GitHub its further values such as
// timed_out or neutral).
type RunState struct {
	Status     string
	Conclusion string `json:",omitempty"`
	URL        string `json:",omitempty"`
}

// ValidateRunID checks a RunRef's id is a bare positive integer before it is interpolated
// into any request path — the ValidateRefPath shape applied to a run id. A caller-supplied id
// that is anything else is a could-not-check refusal with zero requests.
func ValidateRunID(run RunRef) (int64, error) {
	id := strings.TrimSpace(run.ID)
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != id {
		return 0, Unverifiable(fmt.Sprintf("could-not-check: run id %q is not a bare positive integer — "+
			"a run is addressed by the forge's own id, never a path", StripControl(run.ID)), nil)
	}
	return n, nil
}

// validateRunRef checks a run's branch/tag name. It accepts a short name ("main",
// "release/1.2") or a fully qualified "refs/heads/…"/"refs/tags/…" and returns the SHORT
// name. Every component passes the same checks ValidateRefPath applies, so a ref cannot
// reshape the request it is placed into.
func validateRunRef(ref string) (string, error) {
	r := strings.TrimSpace(ref)
	r = strings.TrimPrefix(r, "refs/heads/")
	r = strings.TrimPrefix(r, "refs/tags/")
	if r == "" {
		return "", Unverifiable("could-not-check: a run needs a branch or tag to run on — the ref is empty", nil)
	}
	for _, p := range strings.Split(r, "/") {
		if err := validateRefComponent(ref, p); err != nil {
			return "", err
		}
	}
	return r, nil
}

// validateRunInputs refuses an empty input/variable name. Values are free text.
func validateRunInputs(in map[string]string) error {
	for k := range in {
		if strings.TrimSpace(k) == "" || strings.ContainsAny(k, " =[]\n\r\t") {
			return Unverifiable(fmt.Sprintf("could-not-check: run input name %q is not a plain name", StripControl(k)), nil)
		}
	}
	return nil
}

// validateGateName refuses an empty or control-bearing gate name before any request.
func validateGateName(gate string) (string, error) {
	g := strings.TrimSpace(gate)
	if g == "" || StripControl(g) != g {
		return "", Unverifiable(fmt.Sprintf("could-not-check: gate name %q is empty or not printable", StripControl(gate)), nil)
	}
	return g, nil
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
	// GetIssueTyped reads the object of ONE stated kind at `number` (see TargetKind). It
	// exists because a bare number is ambiguous on a forge that numbers issues and changes
	// in SEPARATE sequences: GitLab's `#7` and `!7` routinely both exist, and GetIssue
	// REFUSES that case rather than pick one. A caller that knows which kind it means —
	// `deskfile attach` is an observation on an issue — states it and gets that object, or
	// a 404 (IsForgeNotFound) when that kind does not exist at the number. On a forge with
	// ONE number sequence (GitHub) the kind is VALIDATED, never used to route: an issue
	// number read as a change, or a change read as an issue, is a could-not-check error
	// naming the mismatch, never the other object handed back under the wrong name.
	// Consumer: cmd/deskfile's attach target read (freeze rule).
	GetIssueTyped(repo ForgeRepo, number int, kind TargetKind) (*Issue, error)
	// OpenChangeForBranch resolves the single OPEN change (PR ↔ MR) whose SOURCE branch is
	// `branch`, returning (nil, nil) when NONE is open. It exists because every other change
	// read on this seam is keyed by NUMBER, and deskpr's existing-PR-for-branch check and
	// warnIfConflicting have only the branch NAME in hand — a fact no numeric read can answer.
	// MORE THAN ONE open change on one source branch is a could-not-check REFUSAL naming the
	// ambiguity, never a silent first-match: picking one would route a mergeable read or an edit
	// at whichever the forge happened to list first. The returned change carries what the LIST
	// endpoint serves (number, state, draft, head/base refs); a caller needing the merge verdict
	// follows up with GetPullRequest, which is the single-change read that carries Mergeable.
	// Consumer: cmd/deskpr (freeze rule: this read lands with the call site that consumes it).
	OpenChangeForBranch(repo ForgeRepo, branch string) (*PullRequest, error)
	// SearchIssues runs a free-text dedupe search over ONE repo's ISSUES (never changes),
	// returning number, title, state, labels and URL per match (see IssueSearchResult). The
	// query is scoped to the repo by the backend — the caller supplies free text only, never a
	// forge search qualifier. Consumer: cmd/deskfile's file-time dedupe (freeze rule). A forge
	// whose issue search is not repo-scopable in the shape this needs returns could-not-check
	// naming the gap rather than an owner- or instance-wide result.
	SearchIssues(repo ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error)
	// ListLabels returns the repo's label NAMES. It READS ONLY and never creates — the
	// deliberate opposite of ApplyLabels, whose ensure step creates a missing label — because
	// its one consumer, cmd/deskfile's label-existence probe, files an issue UNSTAMPED when a
	// requested label is absent rather than minting the label. A caller that wanted create-if-
	// absent would use ApplyLabels; this is the read that lets a caller decide NOT to.
	// Consumer: cmd/deskfile (freeze rule).
	ListLabels(repo ForgeRepo) ([]string, error)
	// ListOpenChanges reads a repo's OPEN changes (PRs ↔ MRs) with their CI status-check
	// rollups in one bounded read, reporting whether the population was truncated at the page
	// cap (see OpenChanges). It exists because the cross-repo board reads a whole repo's queue
	// at once and an N+1 per-PR fan-out would multiply into secondary-rate-limit territory.
	// Consumer: cmd/deskboard's fetchOpenPRs (freeze rule: this read lands with the call site
	// that consumes it). A forge whose CI rollup does not map to the two-shape union RollupNode
	// carries returns could-not-check naming the gap rather than an approximation.
	ListOpenChanges(repo ForgeRepo) (*OpenChanges, error)
	// ListChanges reads a repo's changes (PRs ↔ MRs) in the requested lifecycle STATES —
	// distinguishing MERGED from CLOSED, which ListOpenChanges does not — carrying per change
	// the number, state, head sha, source branch, title, body (with its link trailers) and
	// merged-at (see ChangeRef/ChangeStates/ChangeList). It exists because the already-
	// represented / phantom reconciliation must know whether a brief has an OPEN or a MERGED PR,
	// and ListOpenChanges serves OPEN only while collapsing state to a board word — so a raw
	// `gh pr list --state all` was the only alternative, and the closed forge surface
	// (TestNoForgeCLIShellout) forbids it. The read is bounded: it walks MOST-RECENTLY-UPDATED
	// first to a hard page-count ceiling and reports ChangeList.Incomplete when the population
	// exceeds it, so a consumer that reads absence as evidence treats a truncated read as
	// could-not-check rather than a confident negative. A states-less request (ChangeStates.Any
	// false) is REFUSED before any request is built — the state set must be STATED, never
	// defaulted to a whole-repo scan. Consumer: cmd/deskdispatch's phantom check via
	// deskkit.RepresentedPRRefs (freeze rule: this read lands with the call site that consumes
	// it).
	ListChanges(repo ForgeRepo, states ChangeStates) (*ChangeList, error)
	// ListOpenIssues reads a repo's OPEN issues (never changes) as classification summaries —
	// number, title, rendered author, labels, creation time (see IssueSummary). Consumer:
	// cmd/issueboard's fetchOpenIssues (freeze rule).
	ListOpenIssues(repo ForgeRepo) ([]IssueSummary, error)
	// PRTrustEvents reads a change's trust-gate content events: the body-edit time plus every
	// comment, review and review-comment with its author identity and edit time, bounded to a
	// single page and reporting completeness (see TrustPayload). Consumer: cmd/deskboard's
	// prBlessed (freeze rule). A forge whose content-edit / numeric-actor-id semantics do not
	// map 1:1 returns could-not-check naming the gap.
	PRTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error)
	// IssueTrustEvents is PRTrustEvents' issue twin (an issue has no reviews or review
	// threads). Consumers: cmd/deskboard's issueBlessed, cmd/issueboard's trust gate,
	// cmd/scanloop's queueing trust gate (freeze rule).
	IssueTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error)
	// IssueContentEvents reads an issue's comment content events for the ESCALATION CLOCK,
	// paginating the comment connection to exhaustion under a HARD page cap (the returned
	// TrustPayload carries no BodyEdited — the clock reads only Events' author+CreatedAt).
	// It is the escalation-clock twin of IssueTrustEvents, and DELIBERATELY distinct from it:
	// IssueTrustEvents reads ONE bounded page and fails closed to quarantine (an untrusted
	// thread too busy to read in a page is never silently admitted), but the escalation clock
	// is computed only for issues ALREADY past the trust gate and needs the WHOLE thread to
	// find the last human response — so a decision-owed issue with a long thread must still
	// yield an escalation verdict rather than take the board down (the single-page bound did
	// exactly that: one overflowed thread failed the whole board with exit 6). Complete=false
	// means the hard page cap was reached before the thread ended (or the forge advertised a
	// next page with no cursor to advance on); the caller then treats that ONE issue's clock
	// CONSERVATIVELY (escalate, could-not-check) and renders the rest of the board — an
	// overflowed thread is NEVER read as "no escalation owed". Consumer: cmd/issueboard's
	// fetchIssueEvents (freeze rule: this read lands with the call site that consumes it).
	IssueContentEvents(repo ForgeRepo, number int) (*TrustPayload, error)
	// ReviewsAtHead returns every review on a change (paginated to exhaustion), in
	// ASCENDING SUBMITTED ORDER — oldest first.
	//
	// The order is part of the contract, not an incidental property of whichever endpoint
	// a backend happens to read. Every consumer reduces this slice by walking it and
	// letting the LAST decisive verdict win (deskboard's reduceReviews, deskpost's
	// latestAppVerdict, deskflip's ReduceAppVerdict), because that is what "the standing
	// verdict" means. Handed the reversed stream those reductions silently invert: the
	// oldest verdict governs, an approval at a newer head never clears an earlier
	// request-changes, and an ordinary approve-then-reject reads as a forged no-op
	// approval. GitHub's reviews endpoint is chronological and satisfies this for free;
	// GitLab's notes endpoint defaults to newest-first and its backend re-orders (#1124).
	//
	// A review whose submitted time could not be established sorts FIRST — an undatable
	// verdict may be superseded by any dated one and may never supersede one, which is the
	// fail-closed placement.
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
	// ListCommentsTyped is ListComments for a caller that has STATED which kind of object
	// `number` names (see TargetKind, GetIssueTyped). ListComments reads a CHANGE's thread
	// on BOTH backends — GitHub's read is the `pullRequest` GraphQL connection and GitLab's
	// is `/merge_requests/:iid/notes` — so an ISSUE's own thread is unreachable through it
	// on either forge: on GitHub it comes back EMPTY (the `pullRequest` selection resolves
	// to null at an issue's number) and on GitLab it comes back as the notes of whichever
	// merge request shares the number. An empty thread reads as "nobody has commented",
	// which is exactly how an unread precondition becomes a satisfied one. This op routes
	// on the stated kind, so an issue's thread is read from the issue. Consumer:
	// cmd/deskclose's two-role superseded lane (freeze rule: it lands with that call site).
	// An unknown kind is refused rather than defaulted.
	ListCommentsTyped(repo ForgeRepo, number int, kind TargetKind) ([]Comment, error)
	// RepoVisibility returns the repo's visibility (private | public | internal | ...).
	RepoVisibility(repo ForgeRepo) (string, error)
	// ReadFile reads a file's content at a ref (GitHub Contents API ↔ GitLab Repository Files
	// API). A path absent on the ref is a not-found error the caller tests with
	// IsForgeNotFound — the seam does not decide whether "absent" is an error, because for a
	// merge-into-existing it is and for a first-write it is not.
	ReadFile(repo ForgeRepo, in ReadFileInput) (*FileContent, error)
	// ListRecentCommits returns up to limit commits from the head of repo's DEFAULT branch,
	// newest first (GitHub `/repos/{o}/{r}/commits` ↔ GitLab `/projects/:id/repository/
	// commits`). An EMPTY repository is reported as the backend-neutral ErrForgeEmptyRepo
	// sentinel the caller tests with IsForgeEmptyRepo — each backend translates its OWN empty
	// signal (GitHub 409 / GitLab 404) into that sentinel, so a raw status is never tested
	// backend-blind. It is a distinct KNOWN state (no-commits), not a read failure; every OTHER
	// error (a GitHub 404 for a gone/renamed repo or lost token access included) stays a read
	// failure the caller surfaces as could-not-check. Consumer: cmd/deskboard's
	// fetchRecentCommits (freeze rule).
	ListRecentCommits(repo ForgeRepo, limit int) ([]RepoCommit, error)
	// GetCommit reads ONE commit's committed date and attributed author/committer accounts
	// (GitHub `/repos/{o}/{r}/commits/{sha}` ↔ GitLab `/projects/:id/repository/commits/:sha`).
	// The account-login fields are a per-field could-not-check where the forge resolves no
	// account (see RepoCommit). Consumer: cmd/deskboard's fetchHeadCommit (freeze rule).
	GetCommit(repo ForgeRepo, sha string) (*RepoCommit, error)
	// CompareRefs compares two refs and returns the files that differ plus the divergence
	// counts and the forge's own status word (see RefComparison). A forge that does not report
	// the divergence status/counts in the shape GitHub's compare API does returns
	// could-not-check naming the gap rather than approximating a benign-merge verdict.
	// Consumers: cmd/deskboard's changedFilesBetween and fetchBehindMain (freeze rule).
	CompareRefs(repo ForgeRepo, base, head string) (*RefComparison, error)
	// SearchOpenChanges returns every OPEN change under one account (GitHub `search prs
	// --owner`). The account is a single owner name, not a repo — this is the ONE owner-wide
	// read on the seam, used by the scope-reconciliation verb to find open changes in repos the
	// board does not watch. A forge whose search is not owner-wide (GitLab's search is group/
	// project-scoped and paginated differently) returns could-not-check naming the gap.
	// Consumer: cmd/deskboard's cmdScope (freeze rule).
	SearchOpenChanges(owner string) (*ChangeSearchResults, error)
	// ListWorkflowFiles returns the CI workflow file names configured at a ref (GitHub
	// `.github/workflows/*.yml`). A path absent at the ref is a not-found error the caller
	// tests with IsForgeNotFound (a repo with no workflows). It is GitHub-Actions-specific: a
	// forge whose CI configuration is not a per-workflow-file directory (GitLab's single
	// `.gitlab-ci.yml`) returns could-not-check naming the gap. Consumer: cmd/deskboard's
	// listWorkflowFiles, the zero-CI probe (freeze rule).
	ListWorkflowFiles(repo ForgeRepo, ref string) ([]string, error)
	// ChangeDiff returns a change's raw unified diff TEXT (GitHub `pr diff`). It is the human-
	// display read behind `deskboard diff`; the STRUCTURED file list is ListChangedFiles. A
	// forge that does not serve a single raw unified-diff document for a change returns
	// could-not-check naming the gap. Consumer: cmd/deskboard's cmdDiff (freeze rule).
	ChangeDiff(repo ForgeRepo, number int) (string, error)
	// RefExists reports whether one git ref is PRESENT in repo. The ref is a REF PATH inside
	// the repo's own ref namespace ("heads/topic", "heads/dispatch/<key>"), never an API path:
	// it is validated by ValidateRefPath before any request is built, so this op cannot address
	// an arbitrary endpoint — the same bound DeleteRef carries, for the same reason (a path-
	// shaped argument that reaches a URL is an arbitrary-endpoint reach unless something refuses
	// the paths that are not refs).
	//
	// A ref that is ABSENT is reported as (false, nil) — the ANSWER, not a failure: that is the
	// whole point of the read. Every OTHER non-2xx is an error the caller reads as
	// could-not-check — a 403 from a token that cannot see refs can never be mistaken for "the
	// ref is gone". A backend whose forge cannot serve a ref-existence read for the namespace it
	// was handed returns a could-not-check REFUSAL naming the gap (GitLab CE exposes no general
	// ref API, so only the `heads/` namespace maps, via the Branches API — the same limit
	// DeleteRef carries), never a guessed "absent".
	//
	// Only a positive ABSENT ages a stamp out; every uncertain path is could-not-check, which
	// changes nothing (freeze rule: this read lands with the call site that consumes it).
	RefExists(repo ForgeRepo, ref string) (bool, error)
	// MatchingRefs returns the FULLY-QUALIFIED ref paths present in repo whose path STARTS WITH
	// refPrefix (the git prefix listing — GitHub `git/matching-refs/<ref>`). refPrefix is a ref
	// path validated by ValidateRefPath before any request is built, so this op cannot address an
	// arbitrary endpoint — the same bound RefExists carries. An EMPTY match is ([], nil), the
	// ANSWER "no such refs", never a failure; every other non-2xx is a could-not-check error the
	// caller must not read as "none".
	//
	// It exists because RefExists addresses ONE exact ref, but a caller sometimes needs a claim
	// FAMILY: a PR's review-dispatch claims are `refs/dispatch/<short>--pr-<N>` plus, for each
	// re-dispatch, `…--<suffix>` — and the suffix a later reader cannot know, so the family must
	// be listed by prefix rather than probed by exact key.
	//
	// Consumer: cmd/deskpost's claimLiveness — the model-capability-floor stamp age-out for a
	// review-lane authority write (a verdict or a ready-flip) reads whether ANY claim in the PR's
	// review-dispatch family is still held, in the `refs/dispatch/*` namespace those claims are
	// ACTUALLY acquired in today (DispatchClaimActiveRefsPrefix — the reader-side half of the
	// issue-708 namespace divergence; the writer/acquire path is left untouched). A backend whose
	// forge cannot prefix-list refs in that shape returns a could-not-check REFUSAL naming the gap
	// (GitLab CE exposes no general ref listing — only the Branches API — so it cannot serve this),
	// never a guessed empty result (freeze rule: this read lands with the call site that consumes it).
	MatchingRefs(repo ForgeRepo, refPrefix string) ([]string, error)
	// RepoHardeningRead reads ONE closed hardening-read kind's document(s) for repo (see
	// HardeningReadKind) — the enumerated replacement for repohardenguard's former arbitrary
	// `gh api <endpoint>` reads. kind is validated by ValidateHardeningReadKind BEFORE any
	// request is built, so this cannot be steered at an arbitrary endpoint
	// (TestForgeNoPassthrough's no-endpoint-argument check keys on parameter names, and `kind`
	// is a closed enum, never a path). The `rulesets` kind performs the list→detail walk
	// internally and returns the ARRAY of full detail documents, so a caller's `[name=X].field`
	// selector resolves inside the returned array without a second op on this seam. The
	// vocabulary is ONE closed set partitioned per forge (HardeningReadKindForge): GitHub
	// serves `repo`/`rulesets`/the Actions and vulnerability-reporting kinds, GitLab serves
	// `project`/`protected-branches`/`protected-tags`/`push-rules`/`approvals` — each a fixed
	// endpoint literal returning that forge's OWN document. A kind the resolved backend does
	// not serve is a could-not-check REFUSAL naming the forge, the kind and the forge that
	// does serve it, emitting zero requests. A tier-gated kind on an edition without it
	// (`push-rules` on Community Edition) is a could-not-check carrying the *ForgeAPIError,
	// never an empty document. Consumer: cmd/repohardenguard's Checker (freeze rule: this op
	// lands with its consumer).
	RepoHardeningRead(repo ForgeRepo, kind HardeningReadKind) (json.RawMessage, error)
	// ReadMergeHold reads the current state of a change's merge-hold marker thread (see
	// MergeHold; the forge-gitlab merge-hold brief). GitHub returns MergeHoldNotApplicable and issues no
	// request — its twin control is server-side branch protection. Any other error is
	// could-not-check, never a silent ABSENT. Consumer: cmd/deskflip's reviewer-approved
	// condition (freeze rule: this read lands with the call site that consumes it).
	ReadMergeHold(repo ForgeRepo, number int) (*MergeHold, error)

	// --- Writes ---

	// CreateDraftChange opens a draft change (draft PR ↔ Draft: MR).
	CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error)
	// OpenMergeHold opens the desk's merge-hold marker thread on a change, returning the
	// hold's opaque id. GitHub returns ErrMergeHoldNotApplicable (test with
	// IsMergeHoldNotApplicable) and issues no request. Consumer: cmd/deskpr create, which
	// opens the hold immediately after CreateDraftChange succeeds on a GitLab-resolved repo
	// (the forge-gitlab merge-hold brief, task 2; freeze rule).
	OpenMergeHold(repo ForgeRepo, number int) (string, error)
	// SetMergeHold releases (MergeHoldUpdate.Resolved true, at Head) or re-arms (Resolved
	// false, naming Reason) a change's merge-hold, posting the reply body text ReadMergeHold
	// finds again. GitHub returns ErrMergeHoldNotApplicable and issues no request.
	// Consumers: cmd/deskpost review (release on approve at head, re-arm on
	// request-changes or a stale-head resolve) and cmd/deskflip's reviewer-approved
	// condition (re-arm on a resolve found stale at the flip) — the forge-gitlab merge-hold
	// brief's tasks 3-4; freeze rule.
	SetMergeHold(repo ForgeRepo, number int, in MergeHoldUpdate) error
	// EditChange replaces a change's OWN title/body text (see EditChangeInput) — the change's
	// description, not a comment on it, and NOT a lifecycle transition (that is
	// MarkReadyForReview). A field left empty is not written, so a body-only edit does not blank
	// a title. Consumer: cmd/deskpr edit (freeze rule: this write lands with the call site that
	// consumes it).
	EditChange(repo ForgeRepo, number int, in EditChangeInput) error
	// PostComment posts a plain comment on an issue or PR and returns a reference to what
	// it created. The reference is what makes the write ANSWERABLE — a caller can report
	// where the comment landed and, for an upsert, hold the id it will edit next time —
	// which is the same reason CreateDraftChange returns a PullRef and FileIssue an
	// IssueRef. A write whose only output is "no error" leaves its caller re-reading the
	// listing to find out what it just did.
	PostComment(repo ForgeRepo, number int, body string) (*CommentRef, error)
	// PostCommentTyped is PostComment for a caller that has STATED which kind of object
	// `number` names (see TargetKind, GetIssueTyped). PostComment resolves the kind itself
	// and so inherits GetIssue's both-kinds refusal on GitLab; this write takes the kind
	// from the caller and posts to that kind's endpoint (issue notes ↔ merge-request
	// notes) without a resolving read. On GitHub both kinds share one comments endpoint,
	// so the kind changes nothing there. Consumer: cmd/deskfile attach (freeze rule).
	PostCommentTyped(repo ForgeRepo, number int, kind TargetKind, body string) (*CommentRef, error)
	// PostReview submits a head-pinned review/approval on a change.
	PostReview(repo ForgeRepo, number int, in ReviewInput) error
	// MarkReadyForReview flips a draft change to ready (the only transition this seam
	// exposes — there is no un-ready, merge, or edit).
	MarkReadyForReview(nodeID string) error
	// ApplyLabels reconciles the labels of ONE issue or change in one operation (see
	// LabelChange; change.Target says which kind number names, and an unset target is
	// refused before any request). It is idempotent: applying an already-present label and
	// removing an already-absent one are both no-ops, so a re-run REPLACES rather than
	// stacks and never fails for having already succeeded.
	ApplyLabels(repo ForgeRepo, number int, change LabelChange) (*LabelOutcome, error)
	// EditComment replaces the body of ONE existing comment. commentID is the opaque id a
	// prior ListComments returned (Comment.ID) — never a locally composed one.
	EditComment(repo ForgeRepo, commentID, body string) error
	// FileIssue files a new issue.
	FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error)
	// CloseIssue closes an issue with an optional state reason.
	CloseIssue(repo ForgeRepo, number int, stateReason string) error
	// CloseIssueTyped is CloseIssue for a caller that has STATED which kind of object
	// `number` names (see TargetKind). CloseIssue addresses an ISSUE on GitLab
	// (`PUT /issues/:iid`), so a merge request is unreachable through it — and on a project
	// carrying both at one number the close lands on the OTHER object, which is worse than
	// not closing at all. This op routes on the stated kind. A non-empty stateReason with
	// TargetChange is REFUSED rather than dropped: neither forge records a state reason on a
	// change, and silently discarding one would leave a caller believing a distinction it
	// asked for had been recorded. Consumer: cmd/deskclose (freeze rule: it lands with that
	// call site). An unknown kind is refused rather than defaulted.
	CloseIssueTyped(repo ForgeRepo, number int, kind TargetKind, stateReason string) error
	// ReopenIssue reopens ONE closed issue. It is CloseIssue's inverse and mirrors its shape
	// minus the state reason: a state reason is a close-time field on GitHub and has no
	// GitLab field at all, and reopening clears it on both. It addresses the ISSUE sequence
	// only — there is no typed twin, because its single consumer (cmd/deskclose's
	// verify-gate-refire lane) acts on issues carrying the verify-gate label and refuses a
	// change before any write. Consumer: cmd/deskclose (freeze rule: it lands with that call
	// site). Reversible by construction: a close undoes it.
	ReopenIssue(repo ForgeRepo, number int) error
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

	// --- Run and gate-approval (forge-neutral/14; consumer: cmd/deskrun) ---

	// RunWorkflow starts one run of a workflow on a ref and returns the run it created.
	// GitHub: `POST …/actions/workflows/{workflow}/dispatches` answers 204 with NO run id, so
	// the backend resolves the created run by a follow-up list read narrowed by workflow,
	// event, ref, actor and a time floor taken BEFORE the dispatch call. More than one run
	// matching is a could-not-check REFUSAL naming the ambiguity — never a newest-first
	// guess, because a caller that resolved the wrong run would then approve or read a run it
	// did not start. GitLab: `POST /projects/:id/trigger/pipeline` authenticated by the
	// PIPELINE TRIGGER TOKEN the backend holds (a credential that can start pipelines and
	// nothing else) — the narrow default; the pipeline comes back in the response.
	RunWorkflow(repo ForgeRepo, in RunWorkflowInput) (RunRef, error)
	// ApproveGate clears ONE named deployment gate on a run. The gate is resolved against
	// what the run is actually waiting on; a name matching none of it is a could-not-check
	// REFUSAL naming the run and the gate, never an approval of whatever happened to be
	// pending. GitHub: pending_deployments read, then approve that environment's id. GitLab:
	// dispatches on the STATED shape — a manual job played, or a blocked protected-environment
	// deployment approved — and never falls over to the other shape.
	ApproveGate(repo ForgeRepo, run RunRef, in ApproveGateInput) error
	// RunStatus reads one run's lifecycle in the forge-neutral vocabulary (RunState). GitHub:
	// `GET …/actions/runs/{id}`; GitLab: `GET /projects/:id/pipelines/:id`. A state the
	// mapping does not know is could-not-check, never rounded to a known one.
	RunStatus(repo ForgeRepo, run RunRef) (*RunState, error)

	// --- Identity / transport ---

	// PushTransportHint returns how a minted token authenticates a push to this forge. It
	// carries no secret (see PushTransport).
	PushTransportHint(repo ForgeRepo) PushTransport
}
