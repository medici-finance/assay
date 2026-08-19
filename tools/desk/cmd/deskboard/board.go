package main

// board.go — the data model, GET-only gh fetchers, ACTION classifier, and the six
// read subcommands (prs / actions / reviews / queue / diff / files). Every gh call
// here is a READ (`pr list`, `pr diff`, `gh api` GET, and the trust
// gate's `gh api graphql` QUERY — the one sanctioned -f/-F use; it carries no
// mutation, proven by the shim test). No mutating verb and no `-X POST/PATCH/PUT/
// DELETE` REST flag is ever emitted. The PATH-shim test (deskboard_test.go) proves
// this end to end by enumerating every recorded invocation.
//
// Trust gate (deskkit/trust.go): PRs/issues authored outside the compiled-in trusted
// set (the configured humans and desk Apps) with no CURRENT blessing are
// QUARANTINED — listed under EXTERNAL / UNBLESSED (inert-quoted), counted, never
// given an ACTION. A comment/review from the blessing authority admits an item; content added or edited
// after his latest comment voids the blessing (bless-then-edit).

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The desk posts reviews under this GitHub App login. Only its
// APPROVED / CHANGES_REQUESTED reviews carry flip authority — unforgeable because the
// App is a distinct actor and GitHub blocks self-approval. Ported verbatim from v1.
// isReviewerBot reports whether login is the CONFIGURED reviewer App — whose APPROVED
// review at head is the desk's flip signal. The slug is adopter configuration, not a
// compiled-in constant: the `reviewer=<slug>:<id>` entry of
// ASSAY_TRUSTED_BOT_SLUGS binds it.
//
// This is a MATCHER, not a login accessor, and that is the fix for a review-blocker
// bug. The previous shape returned the bare login and every site compared
// `r.User.Login == reviewerBot()`. An unbound role rendered as "", and the comment here
// claimed "" matches no login — but GitHub renders a DELETED review author as
// `"user": null`, which decodes to `Login: ""`. So "" was a value that matched, and a
// review nobody wrote counted toward the flip. Both conditions below are load-bearing:
// ok rejects the unbound role, and the empty-login guard rejects the author-less actor
// even in the (impossible-by-construction, but not by inspection) case that a binding
// ever rendered empty.
func isReviewerBot(login string) bool {
	want, ok := deskkit.RoleAppLogin("reviewer")
	if !ok || login == "" {
		return false
	}
	return login == want
}

// reviewerBotDisplay is for MESSAGE TEXT only — never a comparison.
func reviewerBotDisplay() string { return deskkit.RoleAppLoginOrEmpty("reviewer") }

// securityPassMarker is the literal line an App security-review verdict must carry for
// a risk-classed PR to be flip-eligible (#216). A line-exact match.
const securityPassMarker = "Security-Review: pass"

// securityFailMarker is the retraction twin of securityPassMarker. Without parsing it
// the board could not see a pass being withdrawn at the same head (#216).
const securityFailMarker = "Security-Review: fail"

// verifyGateLabel selects the awaiting-verification issues for the queue view.
const verifyGateLabel = "verify-gate"

// prListLimit caps `gh pr list` explicitly. Without it gh silently defaults to 30 PRs
// per repo, so a >30-PR board truncates and the desk sweeps an incomplete queue with no
// signal (#80, same silent-truncation class as #79). When a repo returns exactly this
// many open PRs, fetchOpenPRs logs a possible-truncation WARN so widening is never left
// to guesswork.
const prListLimit = 100

// ---------------------------------------------------------------------------
// gh runner (the single choke point every GET goes through)
// ---------------------------------------------------------------------------

// ghConcurrency is the AUTHORITATIVE global cap on how many `gh` subprocesses the real
// runner will have in flight at once. It is enforced inside ghRun itself — the single
// choke point — so it holds no matter how the concurrent sweep nests its fan-out
// (repo-level × per-PR level): the product of the sweep's pool sizes can never exceed this
// number of live subprocesses, which is what keeps a burst of REST reads clear of GitHub's
// secondary rate limits. The desk's outward-WRITE budget (deskkit/ratelimit.go) is a
// separate control for mutations; this read-only board makes none. 6 mirrors
// sweepConcurrency (sweep.go): the safe direction to be wrong in is LOW, because a
// secondary-limit 403/429 fails the whole run closed.
const ghConcurrency = 6

// ghSem bounds concurrent real `gh` executions to ghConcurrency. A buffered channel is a
// concurrency-safe counting semaphore; it introduces no second exec route and no shared
// MUTABLE state (the PATH-shim read-only proof is untouched — it exercises this same real
// path, just never more than ghConcurrency at a time). ghRun never calls itself, so
// acquire→exec→release cannot deadlock.
var ghSem = make(chan struct{}, ghConcurrency)

// ghTimeout is the per-unit deadline on a SINGLE `gh` subprocess, enforced at this same
// choke point. It is the #594 fix: #594 was one `gh` wedging forever on a blocking
// auth/token-refresh, and with no deadline that worker blocks and the concurrent sweep's
// wg.Wait() then blocks the whole board FOREVER with no output (parallelizing the sweep
// without this just lets 6 goroutines wedge instead of 1). A finite per-call budget here
// turns a wedge into a terminable error — the repo then fails closed (exit 6, named) like
// any other unverifiable read, instead of stalling the sweep. It is a package var so the
// wedge test (TestActions_WedgedRead_TimesOut) can shrink it; 120s is far above a healthy
// read (~1–2s) yet finite, so a live-but-slow `gh` is never false-tripped. The safe
// direction to be wrong in is HIGH (a too-short budget could fail a slow-but-live read
// closed) — the opposite of ghConcurrency, and both fail CLOSED, never open.
var ghTimeout = 120 * time.Second

// ghRun shells out to the real `gh` binary and returns stdout. It is a package var so
// the PATH-shim test can exercise the REAL exec path (a fake gh first in PATH) and so
// nothing here can silently swap in a mutating call. A non-zero gh exit becomes an
// error carrying gh's stderr; callers wrap it as Unverifiable (exit 6) naming the repo.
//
// It holds ghSem across the exec so the concurrent sweep's total in-flight subprocess
// count is bounded HERE, at the choke point, rather than depending on every caller to
// respect a budget (#439's lesson: no call site can opt out of a global cap
// if the cap lives at the one point they all pass through). The ghTimeout context is
// created and killed HERE too, so a wedged unit both releases its ghSem slot and becomes
// a terminable error within the budget rather than pinning a slot forever.
var ghRun = func(args ...string) ([]byte, error) {
	ghSem <- struct{}{}
	defer func() { <-ghSem }()
	ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	// WaitDelay bounds how long Output() may block AFTER the deadline kills gh but a
	// surviving child still holds the stdout pipe open (Go 1.20+): the pipe is then
	// force-closed and Wait returns, so a wedged gh cannot keep its ghSem slot — or the
	// whole sweep — past the budget by leaking a grandchild.
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("gh %s: timed out after %s (wedged subprocess killed)", strings.Join(args, " "), ghTimeout)
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %v", strings.Join(args, " "), err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// data model (mirrors the gh JSON shapes)
// ---------------------------------------------------------------------------

// check is ONE entry of the GraphQL statusCheckRollup. The rollup is a UNION: CheckRun
// nodes carry status+conclusion, StatusContext nodes (legacy commit statuses — the
// shape every non-Actions CI still posts) carry `state` and NO status. Decoding only
// the CheckRun half meant every StatusContext fell through `status != "COMPLETED"` and
// counted as PENDING FOREVER, so a repo on commit statuses could never leave WAIT-CI
// while deskpost's REST reducer called the same head green.
type check struct {
	Status     string `json:"status"`     // CheckRun: QUEUED / IN_PROGRESS / COMPLETED
	Conclusion string `json:"conclusion"` // CheckRun: SUCCESS / FAILURE / CANCELLED / SKIPPED / NEUTRAL / ...
	Name       string `json:"name"`       // CheckRun
	State      string `json:"state"`      // StatusContext: SUCCESS / PENDING / FAILURE / ERROR / EXPECTED
	// ^ the five values of GitHub's StatusState enum. statusStateBuckets (inventory_test.go)
	// declares what each MEANS to ciState and fails if this list and that table diverge —
	// the enumeration itself was a subset once (#400 T1: EXPECTED counted as a pass).
	Context  string `json:"context"`    // StatusContext name
	TypeName string `json:"__typename"` // CheckRun | StatusContext (when gh emits it)
}

type prBase struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	// State is gh's OPEN / CLOSED / MERGED. The enumeration below already filters to
	// --state open, so this is belt-and-braces — but #247 is exactly the class of bug
	// where merged-vs-closed stops being load-bearing by accident, and a verb that
	// compares a PR head against a branch must be able to say WHICH state it read
	// rather than assume "it came from the open list, so it is open".
	State   string `json:"state"`
	IsDraft bool   `json:"isDraft"`
	Author  struct {
		Login string `json:"login"` // gh CLI renders App authors as "app/<slug>"
	} `json:"author"`
	CreatedAt string `json:"createdAt"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	HeadRefOid        string  `json:"headRefOid"`
	HeadRefName       string  `json:"headRefName"`
	BaseRefName       string  `json:"baseRefName"` // branch filters in the zero-CI probe (#1652) match against this
	MergeStateStatus  string  `json:"mergeStateStatus"`
	StatusCheckRollup []check `json:"statusCheckRollup"`
}

// labelNames flattens a PR's labels for the human-gate declaration read (#241).
func (p prBase) labelNames() []string {
	out := make([]string, 0, len(p.Labels))
	for _, l := range p.Labels {
		out = append(out, l.Name)
	}
	return out
}

type review struct {
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	Body        string `json:"body"`
	State       string `json:"state"` // COMMENTED / APPROVED / CHANGES_REQUESTED / ...
	CommitID    string `json:"commit_id"`
	SubmittedAt string `json:"submitted_at"`
}

// ---------------------------------------------------------------------------
// GET-only fetchers — every one wraps a gh/parse failure as Unverifiable (exit 6)
// naming the repo, so a single repo failing FAILS the whole run. No fetcher
// ever returns a silent empty result on error.
// ---------------------------------------------------------------------------

// fetchOpenPRs returns a repo's open PRs and whether that population may be TRUNCATED —
// the read came back exactly at the `--limit` cap, so GitHub may be holding more.
//
// #400 T2: truncation used to be a stderr WARNING and nothing else. Every count the board
// prints rides on this population (mergeNowCount, unreviewedCount, the row set itself), and
// the machine consumer reads the JSON, where a truncated population was INDISTINGUISHABLE
// from a complete one — a confident number over an unknown remainder, the same absence-as-
// verdict shape this cluster is about. The flag is returned so callers can state it in-band
// (Header.PRPopulation); the stderr banner stays for the human on the table path.
func fetchOpenPRs(repo string) (prs []prBase, truncated bool, err error) {
	out, err := ghRun("pr", "list", "-R", repo, "--state", "open",
		"--limit", strconv.Itoa(prListLimit), "--json",
		"number,title,body,state,isDraft,author,createdAt,labels,headRefOid,headRefName,baseRefName,mergeStateStatus,statusCheckRollup")
	if err != nil {
		return nil, false, deskkit.Unverifiable("cannot read open PRs for "+repo, err)
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, false, deskkit.Unverifiable("cannot parse PR list for "+repo, err)
	}
	if len(prs) >= prListLimit {
		truncated = true
		fmt.Fprintf(os.Stderr, "deskboard: WARNING %s returned %d open PRs at the --limit %d cap — "+
			"the board may be TRUNCATED; widen prListLimit or paginate (#80)\n", repo, len(prs), prListLimit)
	}
	return prs, truncated, nil
}

// prPopulation is the in-band statement of whether the open-PR set a verb rests on was
// read COMPLETE (#400 T2). Three states, per this cluster's own rule:
//
//   - the field is ABSENT — this verb read no PR list at all (queue, nextup, scope, health);
//     never "the population was complete";
//   - complete=true — every PR list read came back under the cap;
//   - complete=false + truncatedRepos — those repos returned exactly `cap` PRs, so the board
//     may be missing rows, and every count derived from them is a floor, not a total.
type prPopulation struct {
	Complete       bool     `json:"complete"`
	Cap            int      `json:"cap"`
	TruncatedRepos []string `json:"truncatedRepos,omitempty"`
}

// newPRPopulation builds the statement from the repos observed at the cap. A nil/empty
// list is a MEASURED complete read — the caller only calls this when it actually read at
// least one PR list.
func newPRPopulation(truncatedRepos []string) *prPopulation {
	return &prPopulation{
		Complete:       len(truncatedRepos) == 0,
		Cap:            prListLimit,
		TruncatedRepos: truncatedRepos,
	}
}

// renderPopulationLine prints the truncation warning on the table path. Silent when the
// population was complete (the noise budget: a healthy board pays nothing), and silent on a
// nil population — that verb read no PR list, so it has nothing to say on this axis.
func renderPopulationLine(w io.Writer, p *prPopulation) {
	if p == nil || p.Complete {
		return
	}
	fmt.Fprintf(w, "POPULATION TRUNCATED: %s returned open PRs at the --limit %d cap — rows and every "+
		"count below are a FLOOR, not a total; widen prListLimit or paginate (#80)\n",
		strings.Join(p.TruncatedRepos, ", "), p.Cap)
}

// apiPageSize is the maximum GitHub REST page size. The API defaults to THIRTY when
// per_page is omitted, so an unpaginated fetch silently truncates — and truncation here
// is not cosmetic: dropping the newest reviews loses the verdict that governs.
const apiPageSize = 100

// fetchReviews returns ALL reviews for a PR. It pages explicitly: a PR with more than
// 30 reviews used to have its older verdicts silently dropped, and after the #216
// order-sensitive reduction a dropped page can change the answer.
func fetchReviews(repo string, num int) ([]review, error) {
	var all []review
	for page := 1; ; page++ {
		out, err := ghRun("api", fmt.Sprintf("repos/%s/pulls/%d/reviews?per_page=%d&page=%d", repo, num, apiPageSize, page))
		if err != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("cannot read reviews for %s#%d", repo, num), err)
		}
		var chunk []review
		if err := json.Unmarshal(out, &chunk); err != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("cannot parse reviews for %s#%d", repo, num), err)
		}
		all = append(all, chunk...)
		if len(chunk) < apiPageSize {
			return all, nil
		}
	}
}

// maxFilePages bounds the changed-files walk (100/page). Exceeding it leaves the entry
// count below the PR's changed_files, which fetchChangedFiles reports as incomplete.
const maxFilePages = 40 // 4000 entries; GitHub's own /files cap is 3000

// fetchChangedFiles returns the set of paths the PR changes vs its base branch (its own
// contribution), used for the risk-path scan (#216) and the MERGE-CURR check, plus
// whether that set is COMPLETE.
//
// It used to be `gh pr view --json files`, whose GraphQL is files(first: 100) with no
// pagination and no signal that it truncated. Measured on #1618: changedFiles=652,
// `--json files` returned 100 — an ALPHABETICAL window ending inside docs/, so every
// k8s/… and services/… path sorted entirely outside it and the risk-path scan saw none
// of them. This now walks the REST files endpoint and reconciles the entry count
// against GitHub's own changed_files.
//
// Renamed entries contribute BOTH paths: GitHub reports a `git mv` under its NEW name
// only, so a move OUT of a security directory would otherwise leave no trace for the
// triggers to match.
//
// complete=false is not an error — the caller degrades CLOSED per-PR (risk-classed,
// and RE-REVIEW rather than MERGE-CURR) instead of failing the whole board, so one
// enormous PR cannot brick the desk's sweep.
func fetchChangedFiles(repo string, num int) (files map[string]bool, complete bool, err error) {
	// GitHub's own count, for reconciliation. Zero (or an old API without the field)
	// disables the cross-check rather than faking one.
	metaOut, err := ghRun("api", fmt.Sprintf("repos/%s/pulls/%d", repo, num))
	if err != nil {
		return nil, false, deskkit.Unverifiable(fmt.Sprintf("cannot read PR metadata for %s#%d", repo, num), err)
	}
	var meta struct {
		ChangedFiles int `json:"changed_files"`
	}
	if err := json.Unmarshal(metaOut, &meta); err != nil {
		return nil, false, deskkit.Unverifiable(fmt.Sprintf("cannot parse PR metadata for %s#%d", repo, num), err)
	}

	set := map[string]bool{}
	entries := 0
	for page := 1; page <= maxFilePages; page++ {
		out, err := ghRun("api", fmt.Sprintf("repos/%s/pulls/%d/files?per_page=%d&page=%d",
			repo, num, apiPageSize, page))
		if err != nil {
			return nil, false, deskkit.Unverifiable(fmt.Sprintf("cannot read changed files for %s#%d", repo, num), err)
		}
		var chunk []struct {
			Filename         string `json:"filename"`
			PreviousFilename string `json:"previous_filename"`
		}
		if err := json.Unmarshal(out, &chunk); err != nil {
			return nil, false, deskkit.Unverifiable(fmt.Sprintf("cannot parse changed files for %s#%d", repo, num), err)
		}
		entries += len(chunk)
		for _, f := range chunk {
			if f.Filename != "" {
				set[f.Filename] = true
			}
			if f.PreviousFilename != "" {
				set[f.PreviousFilename] = true
			}
		}
		if len(chunk) < apiPageSize {
			break
		}
	}

	complete = meta.ChangedFiles == 0 || entries >= meta.ChangedFiles
	if !complete {
		fmt.Fprintf(os.Stderr, "deskboard: WARNING %s#%d — read %d changed-file entries but GitHub reports %d; "+
			"the diff is TRUNCATED, so this PR degrades to risk-classed / RE-REVIEW\n", repo, num, entries, meta.ChangedFiles)
	}
	return set, complete, nil
}

// changedFilesBetween returns files changed between two commits (compare API), for the
// MERGE-CURR benign-merge check. Unlike v1 this fails CLOSED: a compare error is exit 6.
func changedFilesBetween(repo, base, head string) (map[string]bool, error) {
	if base == "" || head == "" {
		return nil, deskkit.Unverifiable("compare needs both base and head", nil)
	}
	out, err := ghRun("api", fmt.Sprintf("repos/%s/compare/%s...%s", repo, base, head))
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("cannot compare %s %s...%s", repo, short(base), short(head)), err)
	}
	var v struct {
		Files []struct {
			Filename string `json:"filename"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("cannot parse compare for %s", repo), err)
	}
	set := map[string]bool{}
	for _, f := range v.Files {
		if f.Filename != "" {
			set[f.Filename] = true
		}
	}
	return set, nil
}

// prBlessed evaluates the blessing for a PR whose AUTHOR is untrusted: ONE
// bounded `gh api graphql` read (deskkit.PRTrustQuery) covering the PR body's
// lastEditedAt plus all three comment surfaces — conversation comments, reviews, and
// review comments (REVIEWS by the authority count as blessing comments). Bless-then-edit aware:
// content added/edited after the authority's latest comment voids the blessing. Called ONLY
// after deskkit.TrustedAuthor(author) returned false (bounded API growth). An
// incomplete payload (any connection past first:100) is NOT blessed — fail closed to
// quarantine; a fetch/parse error is Unverifiable (exit 6).
func prBlessed(repo string, num int) (bool, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return false, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	out, err := ghRun("api", "graphql",
		"-f", "query="+deskkit.PRTrustQuery,
		"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(num))
	if err != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf("cannot read trust events for %s#%d (trust gate)", repo, num), err)
	}
	bodyEdited, events, complete, perr := deskkit.ParsePRTrustPayload(out)
	if perr != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf("cannot parse trust events for %s#%d (trust gate)", repo, num), perr)
	}
	if !complete {
		return false, nil // overflowed thread — quarantine, never silently admit
	}
	return deskkit.Blessed(bodyEdited, events), nil
}

// ---------------------------------------------------------------------------
// pure helpers (ciState + review reduction + classifier + risk) — unit-tested
// directly, no gh.
// ---------------------------------------------------------------------------

// ciState collapses the check rollup to pass/pending/fail/unknown counts (SKIPPED/
// NEUTRAL ignored). It decodes BOTH rollup node shapes (#268):
//
//   - CheckRun     — `status` + `conclusion`, as before;
//   - StatusContext — `state`, which carries the whole verdict and never has a status.
//
// `unknown` is the fourth state and exists so an entry this reducer cannot interpret is
// COUNTED rather than absorbed. Folding an unrecognised entry into `pending` reads as
// "CI hasn't finished" and never clears; folding it into `pass` would be a fail-open on
// the flip gate. It is reported, and it blocks the green verdict.
func ciState(p prBase) (pass, pending, fail, unknown int) {
	for _, c := range p.StatusCheckRollup {
		switch {
		case c.Status != "": // CheckRun node
			switch {
			case c.Status != "COMPLETED":
				pending++
			case c.Conclusion == "SUCCESS":
				pass++
			case c.Conclusion == "SKIPPED" || c.Conclusion == "NEUTRAL":
				// ignore path-filtered / neutral
			case c.Conclusion == "":
				// #400 N8: COMPLETED with no conclusion at all. This used to fall into
				// `default` and be counted a FAILURE — fail-closed, but the row then said
				// "a check FAILED" about a conclusion nobody could read, which is the same
				// absence-as-verdict shape as R2/R3 with the sign flipped. It is an
				// UNREADABLE entry, and CI-UNKNOWN is what this board calls that; it
				// preempts every approve+green path exactly as a fail does, so nothing
				// gets looser, and the row now names what actually happened.
				unknown++
			default: // FAILURE, CANCELLED, TIMED_OUT, ACTION_REQUIRED, STARTUP_FAILURE
				fail++
			}
		case c.State != "": // StatusContext node (legacy commit status)
			switch strings.ToUpper(c.State) {
			case "SUCCESS":
				pass++
			// #400 T1: `EXPECTED` is NOT a pass. GitHub's StatusState documents it as
			// "Status is expected" — a required legacy status context that has been
			// DECLARED and has not reported yet — as against `SUCCESS`, "Status is
			// successful". Counting it as a pass was this PR's own R2/Q2 defect on the
			// legacy-status axis: one `EXPECTED` context, approved at head, non-draft,
			// CLEAN, rendered `pass=1 ciGreen=true action=MERGE-NOW note="…CI green…"`
			// off a check that had reported nothing. It is a check still owed, which is
			// exactly what `pending` means here — and pending, unlike pass, cannot reach
			// MERGE-NOW or a "CI green" note.
			case "PENDING", "EXPECTED":
				pending++
			case "FAILURE", "ERROR":
				fail++
			default:
				unknown++
			}
		default:
			// Neither shape decoded: a rollup entry we cannot read at all.
			unknown++
		}
	}
	return
}

// reviewState reduces the App's reviews at a given head. Mirrors v1's deskReviewState
// and adds securityPass: whether any App review AT head carries the literal
// Security-Review: pass line (#216), and approvedAt: the timestamp of the latest
// APPROVED review at head (drives the approved-age decay alarm).
type reviewState struct {
	ever         bool
	atHead       bool
	blocking     bool
	approved     bool
	lastSHA      string
	securityPass bool
	approvedAt   time.Time
	// suspectNoOp (#37) is true when the effective verdict AT HEAD reads as
	// blocking specifically because an APPROVED review was SUPPRESSED: it immediately
	// followed a CHANGES_REQUESTED at the SAME commit, with no push in between. See the
	// reduction loop below for the mechanism and deskpost/ready.go's latestAppVerdict for
	// the KEEP IN SYNC twin that gates the actual mutating flip.
	suspectNoOp bool
}

// sameHead reports whether a review's commit sha and the PR's head sha are the SAME
// READ sha (#400 Q3). An empty string on either side is an absence, not a value: the
// review carried no `commit_id`, or `headRefOid` was missing from the payload. Comparing
// them with `==` makes two absences equal and turns "we could not tell" into "yes, at
// head" — the same absence-as-verdict defect as R2 (empty rollup ⇒ "CI green") and R3
// (unread mergeStateStatus ⇒ mergeable), on the review axis. Both must be present.
func sameHead(reviewSHA, head string) bool {
	return reviewSHA != "" && head != "" && reviewSHA == head
}

func reduceReviews(reviews []review, head string) reviewState {
	var st reviewState
	var decisive []review
	lastSec := map[string]secVerdict{}
	var secAuthors []string
	for _, r := range reviews {
		if !isReviewerBot(r.User.Login) {
			continue
		}
		// Security marker: reviews arrive in ascending submitted order, so the LAST
		// security verdict at head governs, per author (#216). A pass
		// retracted by a later fail at the same head is NOT green.
		if sameHead(r.CommitID, head) {
			if v := classifySecurityBody(r.Body); v != secNone {
				if _, seen := lastSec[r.User.Login]; !seen {
					secAuthors = append(secAuthors, r.User.Login)
				}
				lastSec[r.User.Login] = v
			}
		}
		if r.State == "APPROVED" || r.State == "CHANGES_REQUESTED" {
			decisive = append(decisive, r)
		}
	}
	st.securityPass = len(secAuthors) > 0
	for _, a := range secAuthors {
		if lastSec[a] != secPass {
			st.securityPass = false
		}
	}
	if len(decisive) == 0 {
		return st
	}
	st.ever = true

	// #37 ("Reviewer-App gate is forgeable — approve→flip→merge in 14s"): reduce the
	// decisive sequence the same way deskpost's latestAppVerdict does, rather than just
	// taking the chronologically-last review. "Un-forgeable because a PR author cannot
	// approve its own PR" (methodology/brief-17) only defends the AUTHOR account — GitHub's
	// self-approval block has no opinion on a third-party App re-posting a verdict, and any
	// session that can mint the reviewer App's token can post an APPROVED over a standing
	// CHANGES_REQUESTED at an unchanged head. LIVE EVIDENCE: decks#17 and decks#11
	// (2026-07-14) both flipped to APPROVED at an unchanged head, no intervening commit,
	// while a blocking review stood — the board would have reported both as FLIP-eligible.
	// "An approval at an unchanged head cannot be a re-verification." Once a commit carries
	// a CHANGES_REQUESTED, only a NEW commit_id (a genuine push) produces a verdict this
	// reduction accepts as APPROVED; a same-commit APPROVED — including a retried one —
	// keeps reading as the standing CHANGES_REQUESTED, and is flagged suspectNoOp so the
	// board can say so LOUDLY instead of folding it into an ordinary blocking row.
	// KEEP IN SYNC with deskpost/ready.go's latestAppVerdict — with ONE DELIBERATE
	// DIVERGENCE, stated here so the sync claim stays honest (review NOTICE on the #37 PR):
	// latestAppVerdict filters its stream to the CORRECTNESS lane (classifyLane), whereas
	// `decisive` above admits any reviewer-bot APPROVED/CHANGES_REQUESTED, security lane
	// included. So a security pass posted at a head that already carries a standing
	// correctness CHANGES_REQUESTED reads as SUSPECT-APPROVAL on the board while the
	// mutating gate reads it as an ordinary blocked row. That divergence errs toward
	// FLAGGING on an advisory surface a human reads, never toward granting a flip, so it is
	// the safe direction — but it IS a divergence, not an exact mirror. Any change that
	// makes the board's reduction the basis of a mutating decision must close it first.
	var lastCommit string
	var commitBlocked bool
	var effState, effCommit, effSubmittedAt string
	var suspect bool
	for _, r := range decisive {
		if r.CommitID != lastCommit {
			lastCommit = r.CommitID
			commitBlocked = false
		}
		switch r.State {
		case "CHANGES_REQUESTED":
			commitBlocked = true
			effState, effCommit, effSubmittedAt, suspect = r.State, r.CommitID, r.SubmittedAt, false
		case "APPROVED":
			if commitBlocked {
				// No push since the standing CHANGES_REQUESTED at this exact commit —
				// suppress the approval; commitBlocked stays true so a retried forgery
				// at the same commit is refused too.
				effState, effCommit, suspect = "CHANGES_REQUESTED", r.CommitID, true
				continue
			}
			effState, effCommit, effSubmittedAt, suspect = r.State, r.CommitID, r.SubmittedAt, false
		}
	}

	st.lastSHA = effCommit
	// #400 Q3: "at head" is a comparison of two shas, and it is only a statement about
	// the world when BOTH were actually read. A raw `==` made two ABSENCES compare
	// equal: an unread `headRefOid` plus a review carrying no `commit_id` used to
	// produce atHead=true, approved=true, and — with a green rollup — MERGE-NOW, the most
	// consequential verdict this board emits, off two fields nobody could read. Either
	// side empty is could-not-check, and could-not-check is NOT at head — the row falls
	// back to RE-REVIEW/MERGE-CURR, which is the fail-closed direction. sameHead applies
	// that guard on top of the #37 reduction's effCommit.
	st.atHead = sameHead(effCommit, head)
	if st.atHead {
		st.blocking = effState == "CHANGES_REQUESTED"
		st.approved = effState == "APPROVED"
		st.suspectNoOp = suspect
		if st.approved {
			if t, err := time.Parse(time.RFC3339, effSubmittedAt); err == nil {
				st.approvedAt = t
			}
		}
	}
	return st
}

// hasSecurityPassLine reports whether body carries the Security-Review: pass marker.
//
// DELEGATES to deskkit.HasSecurityReviewPass — the canonical, emphasis-tolerant reader.
// This used to be a THIRD, hand-rolled implementation
// (`strings.TrimSpace(ln) == marker`, exact and case-SENSITIVE) that never went through
// bodycheck at all, so it disagreed with deskpost's read path on exactly the bodies that
// widened deskpost to recover (an emphasised `**Security-Review: fail**` retraction). Both
// deskpost's bodycheck.HasSecurityReviewPass and this now read through the same deskkit
// function, so there is one reader and one accepted set.
func hasSecurityPassLine(body string) bool {
	return deskkit.HasSecurityReviewPass(body)
}

// hasSecurityFailLine reports whether body carries the Security-Review: fail marker.
// See hasSecurityPassLine — same delegation to deskkit.
func hasSecurityFailLine(body string) bool {
	return deskkit.HasSecurityReviewFail(body)
}

// secVerdict is one review body's security-review verdict.
type secVerdict int

const (
	secNone secVerdict = iota // the body carries no Security-Review line
	secPass
	secFail
)

// classifySecurityBody reduces one review body to its security verdict. A body carrying
// BOTH markers is ambiguous and classifies as FAIL — the board fails closed, mirroring
// deskpost's ready gate.
func classifySecurityBody(body string) secVerdict {
	pass := hasSecurityPassLine(body)
	fail := hasSecurityFailLine(body)
	switch {
	case fail:
		return secFail
	case pass:
		return secPass
	default:
		return secNone
	}
}

// anyRiskPath delegates to deskkit — the ONE risk-class computation deskpost's ready
// gate also consumes. It used to be a second, hand-rolled copy of the trigger list
// here, which is precisely the drift anti-pattern shape (a parallel list that drifts): riskpath.go's
// own doc already claimed "shared helper in deskkit, not duplicated" while this copy
// existed, and the two had already diverged in their matching rules. Routing through
// deskkit is what makes the repo-aware classification reach the board.
//
// A nil/empty file set fails closed inside deskkit — a board row that could not read
// the diff must never read as "no security review needed".
func anyRiskPath(repo string, files map[string]bool) bool {
	paths := make([]string, 0, len(files))
	for f := range files {
		paths = append(paths, f)
	}
	sort.Strings(paths)
	return deskkit.RiskPathTriggered(repo, paths)
}

// Action verbs. MERGE-NOW ranks above all — an approved-at-head CI-green PR
// whose approval is perishable when main outruns the merge. SECURITY-REVIEW-REQUIRED
// (#216) sits between BLOCKED and CI-RED: it blocks a would-be FLIP for a risk-classed
// PR whose security review has not passed. CI-NEVER-RAN (#1652) ranks with CI-RED: the
// rollup is empty but a workflow at head would have fired on the diff — the PR is
// UNVALIDATED, which is a work item (retrigger/investigate CI), never a flip signal.
const (
	actMergeNow = "MERGE-NOW"
	// actHumanGate (#241) is terminal for a PR carrying a machine-readable human-gate
	// declaration: it replaces MERGE-NOW/FLIP so the board can never print "merge now"
	// for work whose merge is reserved to a human, and it is excluded from the
	// MERGE-NOW count and decay banner.
	actHumanGate = "HUMAN-GATE"
	// actSuspectApproval (#37) ranks above BLOCKED: a no-op APPROVED posted over a
	// standing CHANGES_REQUESTED at an unchanged head (no intervening push) is a
	// forgery-shaped verdict, not a routine "needs work" row — surface it LOUDLY rather
	// than fold it into an ordinary BLOCKED read.
	actSuspectApproval = "SUSPECT-APPROVAL"
	actBlocked         = "BLOCKED"
	actSecReview       = "SECURITY-REVIEW-REQUIRED"
	// actCIUnknown (#268) is the CI three-state's fourth outcome: the rollup carried
	// entries this board cannot interpret, so the CI verdict is not established. It
	// blocks the approve+green path rather than defaulting either way.
	actCIUnknown = "CI-UNKNOWN"
	// actCIUnverified (#400 R2) is the OTHER way the CI verdict fails to exist: not an
	// entry we could not read, but NO entry at all on a repo that runs PR CI (an empty
	// or absent `statusCheckRollup`, or one carrying only SKIPPED/NEUTRAL). ciUnknown
	// counts what was unreadable; this counts what was never there. Both mean "green was
	// not established", and neither may reach a FLIP/MERGE-NOW note that says "CI green".
	actCIUnverified = "CI-UNVERIFIED"
	actCIRed        = "CI-RED"
	// actCINeverRan (#1652) is the rollup being empty for a DIFFERENT reason than
	// actCIUnverified: a workflow that WOULD fire on this diff never ran at head at
	// all (dispatch failure, not "no CI configured"), so the PR is UNVALIDATED rather
	// than CI-green-or-red.
	actCINeverRan = "CI-NEVER-RAN"
	actWaitCI     = "WAIT-CI"
	actConflict   = "CONFLICT"
	// actMergeStateUnknown (#400 R3) is the mergeability four-state's third arm:
	// GitHub returned `UNKNOWN` (mergeability still computing) or nothing at all. The
	// board must not convert that silence into MERGE-NOW.
	actMergeStateUnknown = "MERGE-STATE-UNKNOWN"
	// actMergeBehind (#54) is the mergeability four-state's fourth arm: mergeStateStatus
	// read BEHIND — a MEASURED "main has moved since this head was last synced", not an
	// unmeasured unknown. The review that approved this head verified it against a `main`
	// that is no longer the merge target. MERGE-NOW must not fire off a stale-base review;
	// this is the retro finding #54 exists to close.
	actMergeBehind = "MERGE-BEHIND"
	actFlip        = "FLIP"
	actReReview    = "RE-REVIEW"
	actNeedsReview = "NEEDS-REVIEW"
	actMergeCurr   = "MERGE-CURR"
	actReady       = "READY"
	actCheck       = "CHECK"
)

// guardArmMarker is the distinct phrase the fail-loud guard arm (#400 R2) puts in its
// note, and nothing else in this package may say. It is a named constant rather than an
// inline string because of N6: the tripwire proving that arm is UNREACHABLE keyed on a
// substring of the note the arm itself returns, so a mutation that replaced the return
// with the old `FLIP` + "CI green" note deleted the tripwire along with the arm and the
// whole suite stayed green — a guard that removes itself when it is defeated. Keyed on
// this constant, and with TestClassify_GuardArmIsStillTheGuardArm asserting the arm still
// returns it, the same mutation is now visible.
const guardArmMarker = "NO CI/merge arm of the classifier claimed this row"

// ---------------------------------------------------------------------------
// mergeStateStatus, read as four states (#400 R3, #54)
// ---------------------------------------------------------------------------

// mergeVerdict is the four-state read of GitHub's `mergeStateStatus`. The board used a
// two-state test — `DIRTY || BLOCKED` is a conflict, EVERYTHING ELSE is mergeable — which
// made `UNKNOWN` (GitHub has not finished computing mergeability) and `""` (the field
// could not be resolved at all) both classify as mergeable, and an approved+green PR in
// either state rendered as MERGE-NOW. That is the subset/absence defect on the merge axis:
// a field we could not read became a positive statement about the world.
//
// The `default` arm is the load-bearing one. It catches `UNKNOWN`, `""`, and any value
// GitHub adds to the enum later — a new state this code has never seen must arrive as
// "could not be read", never as "mergeable".
//
// mergeVerdictBehind is split out of the old mergeable bucket. BEHIND
// is not "could not be read" — it is a MEASURED "no": GitHub is reporting that the base
// branch has moved since this PR's branch was last synced. Folding it into mergeVerdictOK
// made the classifier reproduce the exact retro finding #54 documents: a review posted
// against review-time `main` gets read as still-current at merge-time, because nothing
// re-asks the question against the tree the PR would actually land in. An APPROVED verdict
// on a BEHIND head verified only the diff against a `main` that no longer exists.
type mergeVerdict int

const (
	mergeVerdictOK      mergeVerdict = iota // a recognised state that permits a merge
	mergeVerdictBlocked                     // DIRTY / BLOCKED — definitely not mergeable
	mergeVerdictUnknown                     // could-not-check: UNKNOWN, absent, or unrecognised
	// mergeVerdictBehind (#54) — BEHIND: the base has moved since the head was last synced.
	// The review's "correctness" was established against a `main` that is now stale.
	mergeVerdictBehind
)

// readMergeState maps the raw `mergeStateStatus` string to its verdict. The recognised
// mergeable set is GitHub's own enum minus the two blocking values and BEHIND: CLEAN,
// HAS_HOOKS, UNSTABLE (only non-required checks are failing), and DRAFT (the board flips
// drafts, so draft-ness is not a merge-state objection here). BEHIND (base moved on) is its
// own verdict (#54) — a merge is still mechanically possible, but the review that approved
// this head did not verify it against the base it would actually merge into.
func readMergeState(s string) mergeVerdict {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CLEAN", "HAS_HOOKS", "UNSTABLE", "DRAFT":
		return mergeVerdictOK
	case "BEHIND":
		return mergeVerdictBehind
	case "DIRTY", "BLOCKED":
		return mergeVerdictBlocked
	default:
		return mergeVerdictUnknown
	}
}

// classifyInput carries everything the pure classifier needs. ownFilesChanged is only
// consulted when !atHead (MERGE-CURR vs RE-REVIEW): true = PR's own files changed since
// the reviewed sha (RE-REVIEW), false = benign keep-current merge (MERGE-CURR).
// ciGreen: true when CI is confirmed green (pass>0, no pending/fail) or
// vacuously green for CI-less repos.
// mergeConflict: true when MergeStateStatus is DIRTY or BLOCKED —
// an un-mergeable PR must never classify MERGE-NOW regardless of approval+CI state.
// zeroCI (#1652): the probed state of a 0/0/0 rollup — zeroCINoChecks /
// zeroCINeverRan / zeroCIUnverified, or "" when the rollup counted at least one
// check. Never-ran and unverified block the FLIP signal outright; a bare
// `0✓ 0pend 0fail` is never again read as green.
type classifyInput struct {
	ever            bool
	atHead          bool
	blocking        bool
	approvedAtHead  bool
	draft           bool
	pass            int
	pending         int
	fail            int
	ownFilesChanged bool
	riskClassed     bool
	securityPass    bool
	ciGreen         bool
	mergeConflict   bool
	// mergeStateUnknown (#400 R3): the mergeStateStatus could not be read (UNKNOWN,
	// absent, or a value this code does not recognise). Distinct from mergeConflict —
	// that is a measured "no", this is "we did not measure". It blocks MERGE-NOW.
	mergeStateUnknown bool
	// mergeBehind (#54): mergeStateStatus is BEHIND — main has moved since this head was
	// last synced. Distinct from BOTH mergeConflict (a measured "no, it will not merge")
	// and mergeStateUnknown (an unmeasured "we don't know"): this is a measured "the base
	// this review verified is no longer the base a merge would use". It blocks MERGE-NOW
	// the same way mergeStateUnknown does, but for the opposite reason — GitHub read the
	// state fine, and what it read says the review is stale against current main.
	mergeBehind bool
	// mergeStateRaw is the verbatim mergeStateStatus, so the could-not-check note can
	// name what actually came back instead of asserting a cause.
	mergeStateRaw string
	// ciUnknown (#268): rollup entries this board could not interpret. >0 means the CI
	// verdict was not established — never treated as green, never as merely pending.
	ciUnknown int
	// humanGate (#241): the PR carries a machine-readable human-gate declaration.
	humanGate       bool
	humanGateReason string
	// zeroCI (#1652): the probed state of a 0/0/0 rollup — see the type doc above.
	zeroCI string
	// suspectNoOp (#37): the effective blocking state is a suppressed no-op approval —
	// an APPROVED posted over a standing CHANGES_REQUESTED at the same head, with no
	// intervening push. See reduceReviews.
	suspectNoOp bool
}

// classify reproduces deskboard v1's ACTION semantics exactly, with the single #216
// addition, the MERGE-NOW class, and the #1652 zero-CI split: same inputs → same
// ACTION as v1 whenever ciGreen, riskClassed are false and zeroCI is empty.
func classify(in classifyInput) (action, note string) {
	switch {
	case !in.ever:
		return actNeedsReview, "no bot APPROVED/CHANGES_REQUESTED at head — dispatch a reviewer"
	case !in.atHead:
		if !in.ownFilesChanged {
			return actMergeCurr, "keep-current merge; PR's own files unchanged since last review — no re-review"
		}
		return actReReview, "head advanced past last review (PR's own files changed) — delta re-review"
	// #37: an APPROVED that immediately follows the App's own CHANGES_REQUESTED at the
	// SAME head, with no push in between, is not a re-verification — surface it as a
	// suspected forgery/no-op-flip attempt, distinct from an ordinary blocking review, so
	// a human reading the board sees the attempt rather than a routine BLOCKED row. Never
	// FLIP-eligible either way, but which of the two it is matters for the response (chase
	// down who minted the App token vs. wait on the worker).
	case in.blocking && in.suspectNoOp:
		return actSuspectApproval, reviewerBotDisplay() + " posted APPROVED at head immediately after its " +
			"own CHANGES_REQUESTED at the SAME head, with no intervening push — that cannot be a " +
			"re-verification (#37); treating as a suspected forged/no-op flip attempt, never FLIP-eligible, " +
			"until a new commit lands"
	case in.blocking:
		return actBlocked, reviewerBotDisplay() + " requested changes at head — worker must act"
	// A DEFINITE failure outranks an indefinite unknown. When the rollup carries both,
	// reporting CI-UNKNOWN alone hid a red behind "inspect the checks" — and a red check
	// is a work item with a known next step, while an unknown is an instruction to go
	// look. Both are stated: the action names the red, the note carries the unmeasured
	// count so the unknown is not quietly absorbed into a state that was measured.
	case in.fail > 0 && in.ciUnknown > 0:
		return actCIRed, fmt.Sprintf("a check FAILED (%d) — that is a definite work item; %d rollup "+
			"entr%s ALSO could not be interpreted, so the REST of CI is not established either",
			in.fail, in.ciUnknown, plural(in.ciUnknown, "y", "ies"))
	// #268: an uninterpretable check rollup means the CI verdict was NOT established.
	// It has to preempt every approve+green path below, which would otherwise treat
	// "no failures and no pendings" as green and reach FLIP.
	case in.ciUnknown > 0:
		return actCIUnknown, fmt.Sprintf("%d check-rollup entr%s could not be interpreted — "+
			"CI state is NOT established (not green, not merely pending); inspect the checks before any flip",
			in.ciUnknown, plural(in.ciUnknown, "y", "ies"))
	// #400 R2: approved at head, nothing failed, nothing pending, nothing unreadable —
	// and STILL not green. On a CI-required repo that means NOT ONE check reported a
	// verdict at this head: an empty/absent rollup, or one carrying only SKIPPED/NEUTRAL
	// entries. Without this arm the flow fell through to the late `case in.approvedAtHead:`
	// arm, which returns FLIP with a note that ASSERTS "CI green" and never consults
	// ciGreen — the tool printing a conclusion about checks it never saw.
	//
	// deskpost's ready gate refuses the identical state (cmd/deskpost/ready.go, `case
	// ciEmpty` on a CIRequired repo → unverifiable, exit 6). One tool exiting 6 while the
	// other recommends the flip is a live divergence on the CI axis, so the two now agree.
	//
	// The zero-CI disambiguation folds in here rather than beside it: `ciGreen` false with fail==0 && pending==0
	// (and ciUnknown==0, excluded above) is ALWAYS the same zero-rollup shape the zero-CI probe
	// exists to explain, so its three states drive the SPECIFIC reason instead of one flat
	// "unverified" — never-ran (a real dispatch failure) is a different work item than a
	// rollup the board could not probe, which is different again from a probed, CONFIRMED
	// no-checks zero that a CI-required repo's policy still refuses to call green. An empty
	// `zeroCI` (never probed — e.g. no caller-side probe was run) keeps the original R2
	// blanket note as the fallback.
	case in.approvedAtHead && !in.ciGreen && in.fail == 0 && in.pending == 0:
		// A risk-classed PR without a security-review pass at head must stay
		// SEC-REVIEW-REQUIRED regardless of the CI-zero reason.
		if in.riskClassed && !in.securityPass {
			return actSecReview, "risk-classed (touches a security path) and no '" + securityPassMarker +
				"' from " + reviewerBotDisplay() + " at head — security review required before FLIP"
		}
		switch in.zeroCI {
		case zeroCINeverRan:
			return actCINeverRan, "workflow(s) exist that would fire on this diff but NO check ran at head — " +
				"the PR is UNVALIDATED (0 checks is absence of evidence, not green); retrigger/investigate CI " +
				"before any flip"
		case zeroCIUnverified:
			return actCheck, "check rollup is empty and the board could NOT verify why — could-not-check is " +
				"not green; do not flip on an unverified zero"
		case zeroCINoChecks:
			return actCheck, "approved at head and verified NO workflow fires on this diff, but repo CI policy " +
				"treats an empty rollup as unverifiable at the deskpost gate — human call, not a routine flip"
		}
		return actCIUnverified, reviewerBotDisplay() + " APPROVED at head, but NO check reported a verdict there — " +
			"the rollup was empty/absent (or carried only skipped/neutral entries) on a repo that runs PR CI, " +
			"so CI green is NOT established; deskpost `ready` refuses this same state (exit 6). Confirm the " +
			"checks actually ran before any flip"
	// approved at head + CI green → MERGE-NOW (ranks above READY/FLIP).
	// Risk-classed drafts without security pass stay blocked (SEC-REVIEW-REQUIRED).
	case in.approvedAtHead && in.ciGreen && !in.mergeConflict:
		// #400 N9: "CI green" is a VERDICT, and on a repo the policy marks as running no
		// PR CI (deskkit.CIRequired false) with nothing in the rollup, no check ran to
		// return one. ciGreen is VACUOUSLY true there — a policy statement, not a result.
		// The action is policy-backed and unchanged; the board simply stops asserting a
		// result nobody produced, which is this PR's own thesis applied to its wording.
		ciPhrase := "CI green"
		if in.pass == 0 {
			ciPhrase = "no PR CI configured for this repo and nothing ran (not a green verdict)"
		}
		if in.draft && in.riskClassed && !in.securityPass {
			return actSecReview, "risk-classed (touches a security path) and no '" + securityPassMarker +
				"' from " + reviewerBotDisplay() + " at head — security review required before FLIP"
		}
		// #1652: on a CI-less repo with a probed no-checks zero the green is
		// vacuous — say so, so "CI green" never silently includes it.
		checkedZero := ""
		if in.zeroCI == zeroCINoChecks {
			checkedZero = " (CI is a CHECKED zero: verified no workflow fires on this diff, #1652)"
		}
		// #241: a declared human gate never resolves to MERGE-NOW. Security still
		// ranks above it — a risk-classed PR needs its verdict either way.
		if in.humanGate {
			return actHumanGate, humanGateNote(in.draft, in.humanGateReason, ciPhrase+checkedZero)
		}
		// #400 R3: mergeability was never established. MERGE-NOW is a recommendation to
		// merge, and it must rest on a READ merge state, not on the absence of a
		// conflict signal. Flipping a draft ready is still safe here (it does not depend
		// on mergeability), so the note says which half survives.
		if in.mergeStateUnknown {
			if in.draft {
				return actFlip, reviewerBotDisplay() + " APPROVED at head, " + ciPhrase + checkedZero + ", draft — the ready-flip is safe " +
					"(it does not depend on mergeability). MERGE-NOW is WITHHELD: mergeStateStatus came back " +
					mergeStateDisplay(in.mergeStateRaw) + ", so mergeability was NOT established — re-read " +
					"the PR before merging"
			}
			return actMergeStateUnknown, reviewerBotDisplay() + " APPROVED at head and " + ciPhrase + checkedZero + ", but mergeStateStatus " +
				"came back " + mergeStateDisplay(in.mergeStateRaw) + " — GitHub had not finished computing " +
				"mergeability, or the field could not be read. MERGE-NOW is NOT established; re-read the PR " +
				"before merging"
		}
		// #54: mergeStateStatus read BEHIND — a MEASURED "main has moved since this head
		// was last synced", not an unmeasured unknown. The App's APPROVED verdict verified
		// the diff against the `main` this head was based on; it never re-asked the
		// question against CURRENT main. Flipping a draft ready is still safe (it does not
		// depend on mergeability, same as the mergeStateUnknown arm above) — but MERGE-NOW
		// is withheld, because that recommendation would be trusting a review that never
		// saw the tree it would actually land in.
		if in.mergeBehind {
			if in.draft {
				return actFlip, reviewerBotDisplay() + " APPROVED at head, " + ciPhrase + checkedZero + ", draft — the ready-flip is safe " +
					"(it does not depend on mergeability). MERGE-NOW is WITHHELD: mergeStateStatus is " +
					mergeStateDisplay(in.mergeStateRaw) + " — main has moved since this head was last synced, so " +
					"the review did not verify against the CURRENT base (#54). Sync with main and get a fresh " +
					"review before merging"
			}
			return actMergeBehind, reviewerBotDisplay() + " APPROVED at head and " + ciPhrase + checkedZero + ", but mergeStateStatus is " +
				mergeStateDisplay(in.mergeStateRaw) + " — main has moved since this head was last synced. The " +
				"review verified this diff against a base that is no longer current (#54). MERGE-NOW is NOT " +
				"established; sync with main and get a fresh review before merging"
		}
		if !in.draft {
			return actMergeNow, reviewerBotDisplay() + " APPROVED at head, " + ciPhrase + ", ready — merge now" + checkedZero
		}
		return actMergeNow, reviewerBotDisplay() + " APPROVED at head, " + ciPhrase + ", draft — desk flips, then merge now" + checkedZero
	case !in.draft:
		return actReady, "already ready — awaiting human/merge"
	case in.fail > 0:
		return actCIRed, "bot APPROVED but a check failed/cancelled"
	case in.pending > 0:
		return actWaitCI, "bot APPROVED, CI still pending"
	case in.mergeConflict:
		return actConflict, "mergeStateStatus is DIRTY/BLOCKED — PR cannot be merged; flip is unsafe"
	// #400 R2: this arm used to be `case in.approvedAtHead:` returning FLIP with a note
	// that ASSERTED "CI green" while never consulting ciGreen — every FLIP the live board
	// has ever printed came out of here, and every one of them was a row whose CI was
	// never established (the MERGE-NOW arm above already takes the genuinely green case).
	// With CI-UNVERIFIED above and MERGE-NOW/FLIP below it, approvedAtHead is now fully
	// resolved before reaching here. It is kept as a FAIL-LOUD guard rather than deleted:
	// if a later arm change ever routes an approved row down here, the board must say it
	// has no verdict instead of inventing the friendliest one.
	// TestClassify_ActionInventory pins that no reachable input reaches this arm.
	case in.approvedAtHead:
		return actCheck, reviewerBotDisplay() + " APPROVED at head, but " + guardArmMarker +
			" — deskboard cannot state a verdict for it. This is a deskboard bug, not a PR state: " +
			"inspect, and do not read it as a flip"
	default:
		return actCheck, "bot review at head is neither APPROVED nor CHANGES_REQUESTED — inspect"
	}
}

// The action-priority table that used to live here is GONE. It stopped ordering
// anything when the row sort became gate-score-descending with an
// oldest-first tie-break (see cmdActions), but the map stayed — Go does not complain
// about an unused package-level var. A dead table that still reads like live ranking
// policy is exactly the "looks like an answer, is not one" shape this PR is about: a
// reviewer read HUMAN-GATE's rank off it and reasoned about board ordering that no
// longer exists. Deleted rather than re-tuned; the ordering lives in one place.
//
// SUSPECT-APPROVAL (#37) ranking above BLOCKED is NOT this table's job either — it is
// enforced by classify()'s switch itself: `case in.blocking && in.suspectNoOp` is
// checked before the plain `case in.blocking` (BLOCKED) arm, so the row reads
// SUSPECT-APPROVAL, never an ordinary BLOCKED, regardless of what a sort does with the
// resulting action string.

// mergeStateDisplay renders a raw mergeStateStatus for a could-not-check note. An empty
// field is named as ABSENT rather than printed as `""`, which reads like a value.
func mergeStateDisplay(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "ABSENT (no mergeStateStatus in the payload)"
	}
	return strconv.Quote(raw)
}

// buildClassifyInput assembles every classifier input that can be derived from ONE PR
// payload plus its reviews, with no further network read. The three inputs it does not
// carry — ownFilesChanged, riskClassed, and zeroCI — need extra fetches/probes and are
// filled by the caller under the same conditions as before (zeroCI: #1652, the probed
// state of a 0/0/0 rollup — "" when the caller never ran the probe, e.g. because the
// rollup was not a zero, or a genuine could-not-probe result of its own).
//
// It exists as a named function because of #400 R2/R3: while this construction was
// inline in cmdActions' 130-line loop, no test could enumerate the payload shapes it
// maps from, so two whole classes of unreadable field (an empty check rollup, an
// unreadable mergeStateStatus) were silently converted into positive verdicts and
// nothing could see it. TestClassify_ActionInventory drives the real payload shapes
// through THIS function, not through a hand-built classifyInput.
func buildClassifyInput(p prBase, rs reviewState, ciRequired bool, zeroCI string) classifyInput {
	pass, pending, fail, unknownChecks := ciState(p)

	// CI is green when no checks are failing or pending, AND either
	// at least one check passed, or CI is not required for this repo
	// (vacuously green — there are no checks to fail).
	// #268: an uninterpretable rollup entry can never count toward green.
	// #400 R2: a false here (nothing failed, nothing pending, and nothing passed
	// either, on a repo that runs CI) is what stops the board asserting green over
	// an empty rollup — classify's CI-UNVERIFIED arm reads ciGreen directly.
	// #1652 tightens the vacuous (`!ciRequired`) half for a ZERO rollup the probe
	// actually READ something negative about: checks-never-ran and unverified are
	// never green on any repo, whether or not that repo requires CI, because absence
	// of evidence is not evidence. A zero the caller never probed (zeroCI=="", the
	// same value an ordinary non-zero rollup carries) is NOT treated as negative
	// evidence — the pre-#1652 vacuous-green rule for a CI-less repo still applies,
	// same as a probed, CONFIRMED no-checks zero.
	ciGreen := unknownChecks == 0 && pass > 0 && pending == 0 && fail == 0
	if unknownChecks == 0 && pass == 0 && pending == 0 && fail == 0 {
		switch zeroCI {
		case zeroCINeverRan, zeroCIUnverified:
			ciGreen = false
		default: // zeroCINoChecks, or "" (never probed)
			ciGreen = !ciRequired
		}
	}

	// #241: the human-gate declaration is read from the PR itself.
	humanGate, hgReason := humanGateDeclared(p.Title, p.Body, p.labelNames())

	mv := readMergeState(p.MergeStateStatus)

	return classifyInput{
		ever: rs.ever, atHead: rs.atHead, blocking: rs.blocking,
		approvedAtHead: rs.approved, draft: p.IsDraft,
		pass: pass, pending: pending, fail: fail,
		securityPass:      rs.securityPass,
		ciGreen:           ciGreen,
		ciUnknown:         unknownChecks,
		humanGate:         humanGate,
		humanGateReason:   hgReason,
		mergeConflict:     mv == mergeVerdictBlocked,
		mergeStateUnknown: mv == mergeVerdictUnknown,
		mergeBehind:       mv == mergeVerdictBehind,
		mergeStateRaw:     p.MergeStateStatus,
		zeroCI:            zeroCI,
		suspectNoOp:       rs.suspectNoOp,
	}
}

// plural picks a suffix for count-carrying notes.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ---------------------------------------------------------------------------
// reports
// ---------------------------------------------------------------------------

// Header rides on every report (JSON and table). asOf anti-stale-quote header (#209);
// stale/audit fields carry the STALE/audit-age banner signal IN-BAND on the JSON path (see
// main.go for why the machine path never writes banners to stderr).
// StopFlags carries active loop stop-flags so a silently-stopped loop is
// visible on every board read.
// MergeNow* fields carry the approved-idle alarm in-band: count, decay flag,
// threshold, and the PR numbers whose approval has expired past the threshold.
// StaleState (#236) is the three-state drift verdict `stale` alone could not carry:
// in-sync / drift / not-applicable (unpinned build — nothing installed to drift) /
// unknown (the check could not run). `stale` stays the authoritative boolean and now
// fails CLOSED: an unknown reads as stale, never as fresh.
// Scope (#359) states what a SWEEPING verb covered; it is absent on verbs that take an
// explicit repo, so absent means "did not sweep", never "the set was empty".
// Unreviewed* (#359, temporal axis) carry the never-reviewed-and-ageing alarm: a PR the
// review sweep has never picked up does not age back into view on its own.
// MainHealth (#295) rides in-band on the verbs that assess it (actions / health) and is
// omitted elsewhere — an ABSENT field means "this verb did not probe", which is exactly
// the distinction the three-state rule demands; it never means "green".
// PolicyDrift (an internal hardening review) rides in-band on
// `actions` the same way MainHealth rides on actions/health: the standalone `policydrift`
// verb is the fail-closed gate a human types, but nothing periodic ever
// typed it, so a visibility flip could sit undetected same as #295's red-main gap. This
// field is the always-on companion — omitted on every verb but `actions`.
type Header struct {
	AsOf              string                   `json:"asOf"`
	Stale             bool                     `json:"stale"`
	StaleState        string                   `json:"staleState,omitempty"`
	StaleDetail       string                   `json:"staleDetail,omitempty"`
	AuditFirstTS      string                   `json:"auditFirstTS,omitempty"`
	AuditReset        bool                     `json:"auditReset"`
	StopFlags         []deskkit.ActiveStopFlag `json:"stopFlags,omitempty"`
	Scope             *BoardScope              `json:"scope,omitempty"`
	MergeNowCount     int                      `json:"mergeNowCount"`
	MergeNowDecay     bool                     `json:"mergeNowDecay"`
	MergeNowThreshold string                   `json:"mergeNowThreshold"`
	MergeNowDecayPRs  []int                    `json:"mergeNowDecayPRs,omitempty"`
	// MergeNowAgeUnknown* (#400 N7) is the decay alarm's third state, symmetric with the
	// UnreviewedAgeUnknown lane below: a MERGE-NOW row whose approving review carried an
	// unparseable (or absent) `submitted_at`. `approvedAt` is then the zero time and the
	// row used to drop OUT of the decay comparison entirely — no line, no counter, and a
	// blank APPROVED column — so "approved seconds ago" and "approval age unreadable"
	// rendered identically. Kept OUT of MergeNowDecayPRs, because that list means
	// "measured past the threshold"; folding an unmeasured row in would be a different
	// lie. Absent (omitempty) = every MERGE-NOW row had a readable approval age.
	MergeNowAgeUnknownCount int      `json:"mergeNowAgeUnknownCount,omitempty"`
	MergeNowAgeUnknownPRs   []int    `json:"mergeNowAgeUnknownPRs,omitempty"`
	UnreviewedCount         int      `json:"unreviewedCount,omitempty"`
	UnreviewedThreshold     string   `json:"unreviewedThreshold,omitempty"`
	UnreviewedPRs           []string `json:"unreviewedPRs,omitempty"`
	// UnreviewedAgeUnknown* is the fourth state of the same alarm: a PR with NO reviewer
	// verdict at any head whose OPEN AGE could not be read (createdAt missing or
	// unparseable), so it can be neither confirmed nor cleared against the threshold.
	// It is kept OUT of UnreviewedCount — that count means "aged past the threshold",
	// and folding an unmeasured row into it would be a different lie — but it must not
	// be a silence either: without this the row renders as an ordinary NEEDS-REVIEW with
	// a blank OPEN column, and "never reviewed, age unknown" looks exactly like
	// "reviewed and fine". Absent (omitempty) = every never-reviewed PR had a readable
	// age, never "not computed".
	UnreviewedAgeUnknownCount int                 `json:"unreviewedAgeUnknownCount,omitempty"`
	UnreviewedAgeUnknownPRs   []string            `json:"unreviewedAgeUnknownPRs,omitempty"`
	MainHealth                *branchHealthReport `json:"mainHealth,omitempty"`
	// PolicyDrift (#442 review) rides in-band on `actions`
	// the same way MainHealth does: the tracker#1543 visibility-drift check, periodic here
	// rather than a standalone cron nobody yet runs.
	PolicyDrift *policyDriftAlarm `json:"policyDrift,omitempty"`
	// PRPopulation (#400 T2) states whether the open-PR set this verb rests on was read
	// complete, on every verb that reads one. Absent = this verb read no PR list — never
	// "complete". See prPopulation.
	PRPopulation *prPopulation `json:"prPopulation,omitempty"`
}

// Report is what a subcommand hands back to main: a value to marshal as JSON, a table
// renderer for --table, and the audit Detail (the open-PR set for prs/actions, "" else).
type Report struct {
	value  any
	render func(io.Writer)
	detail string
}

// ---- prs ----

type prRow struct {
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Draft      bool   `json:"draft"`
	HeadSHA    string `json:"headSHA"`
	MergeState string `json:"mergeState"`
	CIPass     int    `json:"ciPass"`
	CIPending  int    `json:"ciPending"`
	CIFail     int    `json:"ciFail"`
	CIUnknown  int    `json:"ciUnknown"`
	// CIZero/CIZeroDetail (#1652) name WHICH zero a 0/0/0 rollup is:
	// no-checks / checks-never-ran / unverified. ABSENT means the rollup
	// counted at least one check — it never means green.
	CIZero       string `json:"ciZero,omitempty"`
	CIZeroDetail string `json:"ciZeroDetail,omitempty"`
	OpenAge      string `json:"openAge,omitempty"`
}

// externalRow is an open item quarantined by the trust gate (deskkit/trust.go):
// untrusted author, no blessing comment/review. Quarantined-visible — listed and
// counted, never actionable; one comment from the blessing authority admits it on the next run.
type externalRow struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

func sortExternal(rows []externalRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Repo != rows[j].Repo {
			return rows[i].Repo < rows[j].Repo
		}
		return rows[i].Number > rows[j].Number
	})
}

// renderExternal writes the quarantine section on the --table path (shared by prs /
// actions / queue). Silent when nothing is quarantined. All public-origin text
// (title, author) renders INERT — quoted, control characters escaped — so the
// quarantine listing itself cannot inject into the human/agent reading it.
func renderExternal(w io.Writer, kind string, rows []externalRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(w, "\nEXTERNAL / UNBLESSED — %d %s quarantined (untrusted author, no current blessing; NOT actionable — a comment from %s admits, edits after a blessing re-quarantine)\n", len(rows), kind, blessAuthorityName())
	fmt.Fprintf(w, "%-11s %-6s %-28s %s\n", "REPO", "NUM", "AUTHOR", "TITLE")
	for _, r := range rows {
		author := r.Author
		if author == "" {
			author = "(unknown)"
		}
		fmt.Fprintf(w, "%-11s #%-5d %-28s %s\n", shortRepo(r.Repo), r.Number, inert(author, 26), inert(r.Title, 50))
	}
}

// externalDetail extends the audit detail line with the quarantine count so a desk
// quoting the board also states what was withheld.
func externalDetail(rows []externalRow) string {
	if len(rows) == 0 {
		return ""
	}
	return fmt.Sprintf(" external=%d", len(rows))
}

type prsReport struct {
	Header
	PRs      []prRow       `json:"prs"`
	External []externalRow `json:"external"`
}

func cmdPRs(hdr Header) (*Report, error) {
	hdr.Scope = boardScope() // #359: a sweeping verb states its coverage
	rep := prsReport{Header: hdr, PRs: []prRow{}, External: []externalRow{}}
	now := time.Now()
	var open []string
	var truncatedRepos []string // #400 T2: which repos came back at the cap
	for _, repo := range deskkit.AllowedRepos() {
		prs, truncated, err := fetchOpenPRs(repo)
		if err != nil {
			return nil, err // exit 6, repo named — never a partial board
		}
		if truncated {
			truncatedRepos = append(truncatedRepos, repo)
		}
		for _, p := range prs {
			// Trust gate: untrusted author → the bounded blessing read; unblessed →
			// quarantine (visible, excluded from the actionable list and the open= set).
			if !deskkit.TrustedAuthor(p.Author.Login) {
				blessed, berr := prBlessed(repo, p.Number)
				if berr != nil {
					return nil, berr
				}
				if !blessed {
					rep.External = append(rep.External, externalRow{Repo: repo, Number: p.Number, Title: p.Title, Author: p.Author.Login})
					continue
				}
			}
			pass, pending, fail, unknown := ciState(p)
			row := prRow{
				Repo: repo, Number: p.Number, Title: p.Title, Draft: p.IsDraft,
				HeadSHA: p.HeadRefOid, MergeState: p.MergeStateStatus,
				CIPass: pass, CIPending: pending, CIFail: fail, CIUnknown: unknown,
				OpenAge: openAgeOf(p, now),
			}
			// #1652: a zero rollup is ambiguous until probed — never render it bare.
			// A rollup with unreadable (#268) entries is a DIFFERENT absence and is never
			// probed as a zero — CIUnknown already says the CI verdict was not established.
			if pass == 0 && pending == 0 && fail == 0 && unknown == 0 {
				row.CIZero, row.CIZeroDetail = probeZeroCI(repo, p)
			}
			rep.PRs = append(rep.PRs, row)
			open = append(open, fmt.Sprintf("%s#%d", repo, p.Number))
		}
	}
	sort.SliceStable(rep.PRs, func(i, j int) bool {
		if rep.PRs[i].Repo != rep.PRs[j].Repo {
			return rep.PRs[i].Repo < rep.PRs[j].Repo
		}
		return rep.PRs[i].Number > rep.PRs[j].Number
	})
	sortExternal(rep.External)
	rep.Header.PRPopulation = newPRPopulation(truncatedRepos)
	return &Report{value: rep, detail: openDetail(open) + externalDetail(rep.External), render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s\n", hdr.AsOf)
		renderScopeLine(w, rep.Header.Scope)
		renderPopulationLine(w, rep.Header.PRPopulation)
		fmt.Fprintf(w, "%-11s %-5s %-6s %-10s %-22s %-8s %s\n", "REPO", "PR", "DRAFT", "HEAD", "CI", "OPEN", "TITLE")
		for _, r := range rep.PRs {
			fmt.Fprintf(w, "%-11s #%-4d %-6t %-10s %-22s %-8s %s\n",
				shortRepo(r.Repo), r.Number, r.Draft, short(r.HeadSHA), ciSummary(r.CIPass, r.CIPending, r.CIFail, r.CIUnknown, r.CIZero),
				displayDuration(r.OpenAge), title(r.Title, 46))
		}
		if len(rep.PRs) == 0 {
			fmt.Fprintln(w, "(no open PRs in the watched set — see the scope line above for what that covers)")
		}
		renderExternal(w, "PR(s)", rep.External)
	}}, nil
}

// ---- actions ----

type actionRow struct {
	Repo         string `json:"repo"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Action       string `json:"action"`
	Draft        bool   `json:"draft"`
	CIPass       int    `json:"ciPass"`
	CIPending    int    `json:"ciPending"`
	CIFail       int    `json:"ciFail"`
	CIUnknown    int    `json:"ciUnknown"`
	CIZero       string `json:"ciZero,omitempty"`       // #1652: which zero a 0/0/0 rollup is
	CIZeroDetail string `json:"ciZeroDetail,omitempty"` // absent = not a zero row, never "green"
	RiskClassed  bool   `json:"riskClassed"`
	SecurityPass bool   `json:"securityPass"`
	// HumanGate / HumanGateReason (#241): a machine-readable human-gate declaration on
	// the PR itself. A true here can never coexist with action MERGE-NOW.
	HumanGate       bool   `json:"humanGate,omitempty"`
	HumanGateReason string `json:"humanGateReason,omitempty"`
	Score           int    `json:"score"`
	OwningBrief     string `json:"owningBrief"`
	ApprovedAge     string `json:"approvedAge,omitempty"`
	// OpenAge is how long the PR has been open (#359, temporal axis). Absent when the
	// creation time could not be read.
	OpenAge string `json:"openAge,omitempty"`
	// BaseBranchRed (#295) marks a row whose repo's DEFAULT BRANCH is red. The board
	// annotates, it does not block: deskpost owns the ready-flip gate, and whether a red
	// main should hard-block a flip is the human call left open on #295.
	BaseBranchRed bool   `json:"baseBranchRed,omitempty"`
	Note          string `json:"note"`

	approvedAt time.Time // raw time for accurate decay comparison
	openedAt   time.Time // raw time for the never-reviewed alarm (#359)
}

// tombstone is a PR that left the open set between sweeps (#209). State/Merged carry
// HOW it left (#247): absence proves it is gone, never that it merged. Merged is a
// pointer so the unknown case OMITS it — a consumer making an irreversible decision has
// to handle a missing field and cannot read a zero-value false as "closed unmerged".
type tombstone struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	State  string `json:"state"`
	Merged *bool  `json:"merged,omitempty"`
	Note   string `json:"note"`
}

type actionsReport struct {
	Header
	Rows       []actionRow   `json:"rows"`
	Tombstones []tombstone   `json:"tombstones"`
	External   []externalRow `json:"external"`
}

// ---- gate-score machinery (consuming statusgen's --gate-scores JSON) ----
//
// The `gateScoreRow` shape this consumes is declared in nextup.go — the same
// package already models `statusgen --gate-scores` output there,
// so this file reuses it rather than redeclaring it.

// defaultGateScore is the neutral score assigned to PRs whose owning brief is not in
// the gate-scores output (unowned PRs — register PRs, docs, fix/issue-NN not yet
// claimed by a brief). Zero sorts below every real score (min P2=1000).
const defaultGateScore = 0

// execGateScores returns the raw JSON output of `statusgen --gate-scores`. It checks
// the DESKBOARD_GATE_SCORES_JSON and DESKBOARD_GATE_SCORES_FAIL env vars for test
// fixtures; otherwise it shells out to the real statusgen binary. Package var so
// tests can supply fixtures via env vars without a separate mock.
//
// PORT NOTE (#1016 → here): an earlier version ran `go run <root>/tools/statusgen`.
// That is wrong in this repo twice over — assay's statusgen is at
// ./statusgen, not ./tools/statusgen, and nextup.go's contract is explicit that the
// board runs the PINNED statusgen binary and NEVER `go run`s a local tree. So this
// resolves the binary through the same resolveStatusgen() that `deskboard nextup`
// uses, and invokes it with the same `--gate-scores --root <abs>` argv.
var execGateScores = func() ([]byte, error) {
	if v := os.Getenv("DESKBOARD_GATE_SCORES_JSON"); v != "" {
		return []byte(v), nil
	}
	if fail := os.Getenv("DESKBOARD_GATE_SCORES_FAIL"); fail != "" {
		return nil, fmt.Errorf("statusgen --gate-scores: %s", fail)
	}
	root, err := findRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("statusgen --gate-scores: cannot find repo root: %v", err)
	}
	bin, err := resolveStatusgen()
	if err != nil {
		return nil, fmt.Errorf("statusgen --gate-scores: %v", err)
	}
	cmd := exec.Command(bin, "--gate-scores", "--root", root)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("statusgen --gate-scores: %s",
				strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("statusgen --gate-scores: %v", err)
	}
	return out, nil
}

// findRepoRoot returns the git repo root (absolute path).
func findRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// mapBranchToBrief maps a PR branch name to its owning brief ID ("stream/NN") via
// branch-as-claim. It matches known brief IDs by converting
// "stream/NN" to the branch pattern "stream-NN" and looking for that pattern in the
// branch name. Longer patterns are matched first to disambiguate (e.g.
// "stream-name-11" before "name-11"). Returns "" for no match ("unowned").
func mapBranchToBrief(branch string, knownBriefs []string) string {
	if branch == "" {
		return ""
	}
	// Sort by brief ID length descending — longer = more specific match.
	sorted := make([]string, len(knownBriefs))
	copy(sorted, knownBriefs)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	for _, briefID := range sorted {
		// Convert "stream/NN" → "stream-NN" for branch-name matching.
		pattern := strings.Replace(briefID, "/", "-", 1)
		if strings.Contains(branch, pattern) {
			return briefID
		}
	}
	return ""
}

// fetchAndMapGateScores runs statusgen --gate-scores and returns a brief→score map.
// On failure it returns an error that the caller wraps as Unverifiable (exit 6).
func fetchAndMapGateScores() (map[string]int, error) {
	raw, err := execGateScores()
	if err != nil {
		return nil, err
	}
	var rows []gateScoreRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("statusgen --gate-scores: unparseable JSON: %v", err)
	}
	m := make(map[string]int, len(rows))
	for _, r := range rows {
		m[r.Brief] = r.Score
		// If multiple rows for the same brief (shouldn't happen), keep the last one.
	}
	return m, nil
}

// actionsPartial is one repo's contribution to the `actions` board: its own action
// rows, its own quarantine (external) rows, the ids it saw OPEN (for tombstoning), and
// the ids that are ACTIONABLE (non-quarantined, for the open= audit set). It is built
// entirely inside one repo's sweep worker and never touched by another goroutine, so the
// concurrent sweep shares no mutable state — the merge happens after the sweep, in a
// deterministic order (see cmdActions).
type actionsPartial struct {
	rows        []actionRow
	external    []externalRow
	currentOpen []string // every open PR id seen (quarantined included) — tombstone input
	openList    []string // actionable ids only — the open= audit set
	truncated   bool     // #400 T2: this repo's open-PR read came back at the --limit cap
}

// merge folds another partial into this one, preserving append order. It is the single
// operation the per-repo → product → full hierarchy is built from: repo partials merge
// into a product-level partial, product partials merge into the full one.
func (p *actionsPartial) merge(o actionsPartial) {
	p.rows = append(p.rows, o.rows...)
	p.external = append(p.external, o.external...)
	p.currentOpen = append(p.currentOpen, o.currentOpen...)
	p.openList = append(p.openList, o.openList...)
	p.truncated = p.truncated || o.truncated
}

// prOutcome is one PR's contribution to a repo's actionsPartial: always an id (for the
// tombstone-input set), and EITHER a quarantine row (untrusted, unblessed) OR an action
// row. It is what the per-PR fan-out returns; assembling the partial from the outcomes in
// PR-list order keeps the result byte-identical to the old serial loop.
type prOutcome struct {
	id       string
	external *externalRow
	row      *actionRow
}

// sweepActionsRepo computes one repo's actionsPartial. It is the per-repo unit of work
// the bounded sweep runs concurrently; briefScore, knownBriefs, redBases and now are all
// read-only shared inputs, safe to read from many workers at once.
//
// Within the repo it fans the PER-PR reads out concurrently too (classifyPR under a second
// bounded pool) — that fan-out is what removes the long pole of one PR-heavy repo, and it
// is safe because each PR is classified from independent reads with no shared mutable
// state, and the TOTAL number of concurrent `gh` subprocesses is bounded globally by
// ghRun's ghSem regardless of how repo-level and PR-level pools multiply. It fails CLOSED
// exactly as the old serial body did: any gh/parse error is wrapped-and-repo/PR-named
// and propagates to fail the whole run, deterministically (lowest-index PR first).
func sweepActionsRepo(repo string, briefScore map[string]int, knownBriefs []string, redBases map[string]bool, now time.Time) (actionsPartial, error) {
	var part actionsPartial
	prs, truncated, err := fetchOpenPRs(repo)
	if err != nil {
		return actionsPartial{}, err // fail the whole run, name the repo
	}
	part.truncated = truncated
	ciRequired := deskkit.CIRequired(repo)
	outcomes, err := sweepConcurrent(prs, sweepConcurrency, func(p prBase) (prOutcome, error) {
		return classifyPR(repo, p, ciRequired, briefScore, knownBriefs, redBases, now)
	})
	if err != nil {
		return actionsPartial{}, err
	}
	// Assemble the partial in PR-list order (byte-identical to the serial version).
	for _, oc := range outcomes {
		part.currentOpen = append(part.currentOpen, oc.id)
		if oc.external != nil {
			part.external = append(part.external, *oc.external)
			continue
		}
		part.openList = append(part.openList, oc.id)
		if oc.row != nil {
			part.rows = append(part.rows, *oc.row)
		}
	}
	return part, nil
}

// classifyPR does the per-PR reads and classification for one open PR. It is pure of any
// shared mutable state — it returns a prOutcome — so it is safe to run for many PRs at
// once. All gh reads inside it fail CLOSED, naming the repo/PR.
func classifyPR(repo string, p prBase, ciRequired bool, briefScore map[string]int, knownBriefs []string, redBases map[string]bool, now time.Time) (prOutcome, error) {
	id := fmt.Sprintf("%s#%d", repo, p.Number)

	// Trust gate (deskkit/trust.go): untrusted author + no current blessing → quarantine.
	// Quarantined PRs get NO ACTION row and stay out of the open= audit set (they are not
	// on anyone's work list), but keep their id so a prior run's row tombstones cleanly.
	//
	// On a PUBLIC (risk-classed) repo the author bar is HIGHER (#943): only role Apps
	// (ASSAY_TRUSTED_BOT_SLUGS) and mapped humans (ASSAY_HUMAN_LOGIN_MAP) qualify — NEVER
	// a shared machine account that ASSAY_TRUSTED_LOGINS admits as a human, and never a
	// fork author. Public repos accept fork PRs from any account, so
	// auto-reviewing an untrusted diff would spend the reviewer App's identity on hostile
	// input and blur the fork-PR trust boundary. VisibilityRiskClassed is fail-closed:
	// only a KNOWN-private repo keeps the plain TrustedAuthor bar; public/internal/unknown
	// all get the tighter gate. The blessing authority can still admit any single PR by
	// commenting (the manual override, unchanged on either path).
	authorTrusted := deskkit.TrustedAuthor(p.Author.Login)
	if deskkit.VisibilityRiskClassed(repo) {
		authorTrusted = deskkit.TrustedPublicAuthor(p.Author.Login)
	}
	if !authorTrusted {
		blessed, berr := prBlessed(repo, p.Number)
		if berr != nil {
			return prOutcome{}, berr
		}
		if !blessed {
			return prOutcome{id: id, external: &externalRow{Repo: repo, Number: p.Number, Title: p.Title, Author: p.Author.Login}}, nil
		}
	}

	reviews, err := fetchReviews(repo, p.Number)
	if err != nil {
		return prOutcome{}, err
	}
	rs := reduceReviews(reviews, p.HeadRefOid)

	// #1652: an empty rollup is ambiguous until probed. The probe runs ONLY on a truly
	// zero rollup (pass==pending==fail==unknown==0) and never errors — every failure is a
	// row-local `unverified`, never a dead board (same contract as branch health). A
	// rollup carrying entries this board could not interpret (#268) is a DIFFERENT
	// absence and is never probed as a zero — ciUnknown already says the CI verdict was
	// not established.
	pass, pending, fail, unknownChecks := ciState(p)
	zeroState, zeroDetail := "", ""
	if pass == 0 && pending == 0 && fail == 0 && unknownChecks == 0 {
		zeroState, zeroDetail = probeZeroCI(repo, p)
	}

	in := buildClassifyInput(p, rs, ciRequired, zeroState)
	humanGate, hgReason := in.humanGate, in.humanGateReason

	// MERGE-CURR needs the PR's own files vs the changes since the reviewed
	// sha; both are fetched only when actually needed (head advanced).
	if rs.ever && !rs.atHead {
		own, complete, err := fetchChangedFiles(repo, p.Number)
		if err != nil {
			return prOutcome{}, err
		}
		changed, err := changedFilesBetween(repo, rs.lastSHA, p.HeadRefOid)
		if err != nil {
			return prOutcome{}, err
		}
		// A truncated own-files set cannot prove the intersection is empty, so it
		// degrades to RE-REVIEW (the safe side) rather than the benign MERGE-CURR.
		in.ownFilesChanged = !complete || intersects(own, changed)
	}

	// Risk classification (#216) only matters at the FLIP decision: bot
	// APPROVED at head, CI green, still draft, not blocking. Fetch changed
	// files there to test the path triggers.
	riskClassed := false
	if rs.approved && rs.atHead && !rs.blocking && p.IsDraft && fail == 0 && pending == 0 {
		files, complete, err := fetchChangedFiles(repo, p.Number)
		if err != nil {
			return prOutcome{}, err
		}
		// A diff we could not read in full is risk-classed: the trigger we did not
		// see is exactly the one this gate exists to catch.
		riskClassed = !complete || anyRiskPath(repo, files)
	}
	in.riskClassed = riskClassed

	action, note := classify(in)

	// map PR to its owning brief via branch-as-claim.
	owning := mapBranchToBrief(p.HeadRefName, knownBriefs)
	score := defaultGateScore
	if owning != "" {
		score = briefScore[owning]
	}

	// compute approved-age for MERGE-NOW rows (age since the
	// approving review). Zero-value rs.approvedAt → empty string.
	approvedAge := ""
	if action == actMergeNow && !rs.approvedAt.IsZero() {
		approvedAge = formatDuration(now.Sub(rs.approvedAt))
	}

	// #295: annotate (never re-classify) a row whose base branch is red.
	if redBases[repo] {
		note += " — NOTE: " + repo + " default branch is RED; merging stacks onto a broken main"
	}

	openedAt := time.Time{}
	if p.CreatedAt != "" {
		if t, perr := time.Parse(time.RFC3339, p.CreatedAt); perr == nil {
			openedAt = t
		}
	}

	return prOutcome{id: id, row: &actionRow{
		Repo: repo, Number: p.Number, Title: p.Title, Action: action,
		Draft: p.IsDraft, CIPass: pass, CIPending: pending, CIFail: fail,
		CIUnknown: unknownChecks, CIZero: zeroState, CIZeroDetail: zeroDetail,
		RiskClassed: riskClassed, SecurityPass: rs.securityPass,
		HumanGate: humanGate, HumanGateReason: hgReason,
		Score: score, OwningBrief: owning,
		ApprovedAge: approvedAge, approvedAt: rs.approvedAt,
		OpenAge: openAgeOf(p, now), openedAt: openedAt,
		BaseBranchRed: redBases[repo], Note: note,
	}}, nil
}

func cmdActions(hdr *Header, mergeNowThreshold, unreviewedThreshold time.Duration) (*Report, error) {
	hdr.Scope = boardScope() // #359: a sweeping verb states its coverage
	rep := actionsReport{Header: *hdr, Rows: []actionRow{}, Tombstones: []tombstone{}, External: []externalRow{}}
	currentOpen := map[string]bool{}
	var openList []string
	now := time.Now()

	// fetch gate scores once, fail closed (exit 6) on error.
	briefScore, err := fetchAndMapGateScores()
	if err != nil {
		return nil, deskkit.Unverifiable("gate-scores unavailable", err)
	}
	// Collect known brief IDs for branch→brief mapping.
	knownBriefs := make([]string, 0, len(briefScore))
	for id := range briefScore {
		knownBriefs = append(knownBriefs, id)
	}

	// #295: default-branch health, probed BEFORE the PR sweep so every row can be
	// annotated with the state of the branch it would merge into. Unlike the PR reads
	// this never fails the run — an unreadable branch yields an `unknown` row (loud,
	// never green), because a failed health probe does not make the PR rows wrong.
	mainHealth := assessBranchHealth()
	redBases := mainHealth.redRepoSet()

	// an internal hardening review: the public-repo risk rule visibility-drift check rides
	// the actions header the same way mainHealth does, so the periodic surface the finding
	// asked for is this loop rather than a new cron nobody yet runs.
	policyAlarm := assessPolicyDrift()

	// Bounded-concurrency sweep (per-repo → product → full). Each repo's work runs in
	// one of sweepConcurrency workers and returns its own partial; a repo error fails the
	// whole run, deterministically named. See sweep.go. #1652's zero-CI probe
	// (0/0/0 rollups) runs per-PR inside classifyPR, which sweepActionsRepo calls.
	repos := deskkit.AllowedRepos()
	partials, err := sweepRepos(repos, sweepConcurrency, func(repo string) (actionsPartial, error) {
		return sweepActionsRepo(repo, briefScore, knownBriefs, redBases, now)
	})
	if err != nil {
		return nil, err
	}

	// Merge per-repo partials → product-level → full. The merge order is deterministic
	// (products sorted, repos in roster order within each product), and the full report is
	// re-sorted to a total order below, so the output is byte-identical to the old serial
	// sweep regardless of the order the workers finished in.
	byRepo := make(map[string]actionsPartial, len(repos))
	var truncatedRepos []string // #400 T2: which repos came back at the cap
	for i, repo := range repos {
		byRepo[repo] = partials[i]
		if partials[i].truncated {
			truncatedRepos = append(truncatedRepos, repo)
		}
	}
	prodNames, prodGroups := groupByProduct(repos)
	var full actionsPartial
	for _, pname := range prodNames {
		var prod actionsPartial
		for _, repo := range prodGroups[pname] {
			prod.merge(byRepo[repo])
		}
		full.merge(prod)
	}
	rep.Rows = append(rep.Rows, full.rows...)
	rep.External = append(rep.External, full.external...)
	openList = append(openList, full.openList...)
	for _, id := range full.currentOpen {
		currentOpen[id] = true
	}

	// order by gate score descending, oldest-first tie-break
	// (lowest PR number = oldest). Replaces the prior action-priority sort.
	sort.SliceStable(rep.Rows, func(i, j int) bool {
		if rep.Rows[i].Score != rep.Rows[j].Score {
			return rep.Rows[i].Score > rep.Rows[j].Score
		}
		// Tie-break: oldest-first (PR number ascending).
		if rep.Rows[i].Number != rep.Rows[j].Number {
			return rep.Rows[i].Number < rep.Rows[j].Number
		}
		return rep.Rows[i].Repo < rep.Rows[j].Repo
	})

	// #209 tombstones: any PR a prior deskboard run (within the trailing hour) saw
	// OPEN but which is no longer open (merged/closed) gets a "drop from your list"
	// row, so a desk quoting a stale ready-list is contradicted by the next run.
	rep.Tombstones = tombstonesFor(currentOpen, deskkit.AllowedRepos())
	sortExternal(rep.External)

	// post-sort MERGE-NOW counts + decay alarm. Approval is perishable — a
	// PR whose latest App review is APPROVED at the current head with CI green is
	// MERGE-NOW, and when its approved-age exceeds the threshold, emit a decay banner
	// (both in the JSON header and as a table line). Read-only surface — no merge act.
	// #400 N7: three states, not two — past the threshold, within it, or UNMEASURABLE
	// (the approving review's submitted_at was absent or unparseable, so approvedAt is
	// the zero time). The third used to fall out of the loop silently.
	mergeNowCount := 0
	var decayPRs []int
	var ageUnknownPRs []int
	for _, r := range rep.Rows {
		if r.Action == actMergeNow {
			mergeNowCount++
			switch {
			case r.approvedAt.IsZero():
				ageUnknownPRs = append(ageUnknownPRs, r.Number)
			case now.Sub(r.approvedAt) > mergeNowThreshold:
				decayPRs = append(decayPRs, r.Number)
			}
		}
	}
	rep.Header.MainHealth = &mainHealth
	rep.Header.PolicyDrift = &policyAlarm
	// #400 T2: the population every count below rests on, stated in-band. Set BEFORE the
	// counters so a reader meets the coverage claim before the numbers derived from it.
	rep.Header.PRPopulation = newPRPopulation(truncatedRepos)
	rep.Header.MergeNowCount = mergeNowCount
	rep.Header.MergeNowThreshold = mergeNowThreshold.String()
	if len(decayPRs) > 0 {
		rep.Header.MergeNowDecay = true
		rep.Header.MergeNowDecayPRs = decayPRs
	}
	rep.Header.MergeNowAgeUnknownCount = len(ageUnknownPRs)
	rep.Header.MergeNowAgeUnknownPRs = ageUnknownPRs

	// #359 (temporal axis): a review sweep covers the PRs it sees when it starts and
	// nothing ages an older one back into view — a PR opened before a sweep began can be
	// missed while later ones are reviewed, and nothing brings it back on its own.
	// The board cannot make the sweep backfill, but it CAN stop
	// "never picked up" from looking identical to "picked up and fine": a PR with no
	// reviewer verdict at any head, open past the threshold, gets named.
	//
	// The age itself is a three-state read, not two: past the threshold, within it, or
	// UNMEASURABLE. A row whose createdAt is missing or unparseable used to be skipped
	// outright, which put it back in the silence this alarm exists to break — action
	// NEEDS-REVIEW, blank OPEN column, no line, no counter. It now gets its own lane
	// saying which half is unknown: the verdict is known (there is none), the age is not.
	var unreviewed []string
	var unreviewedRows []actionRow
	var ageUnknown []string
	var ageUnknownRows []actionRow
	for _, r := range rep.Rows {
		if r.Action != actNeedsReview {
			continue
		}
		if r.openedAt.IsZero() {
			ageUnknown = append(ageUnknown, fmt.Sprintf("%s#%d", r.Repo, r.Number))
			ageUnknownRows = append(ageUnknownRows, r)
			continue
		}
		if now.Sub(r.openedAt) > unreviewedThreshold {
			unreviewed = append(unreviewed, fmt.Sprintf("%s#%d", r.Repo, r.Number))
			unreviewedRows = append(unreviewedRows, r)
		}
	}
	rep.Header.UnreviewedThreshold = unreviewedThreshold.String()
	rep.Header.UnreviewedCount = len(unreviewed)
	rep.Header.UnreviewedPRs = unreviewed
	rep.Header.UnreviewedAgeUnknownCount = len(ageUnknown)
	rep.Header.UnreviewedAgeUnknownPRs = ageUnknown

	// #1652: the audit line carries the zero-rollup tally, so a later reader can
	// see the probe RAN and what it found — a silent zero is the defect itself.
	detail := openDetail(openList) + externalDetail(rep.External) + " " + mainHealth.auditDetail() + " " + policyAlarm.auditDetail()
	zeroNever, zeroUnver := 0, 0
	for _, r := range rep.Rows {
		switch r.CIZero {
		case zeroCINeverRan:
			zeroNever++
		case zeroCIUnverified:
			zeroUnver++
		}
	}
	if zeroNever > 0 || zeroUnver > 0 {
		detail += fmt.Sprintf(" zeroCI[checks-never-ran=%d unverified=%d]", zeroNever, zeroUnver)
	}

	return &Report{value: rep, detail: detail, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s\n", hdr.AsOf)
		// #295: branch health prints FIRST — above tombstones, decay and MERGE-NOW —
		// because a red default branch outranks a merge onto it. Policy drift (#442
		// finding 1) prints right after: both are board-wide facts about what a merge
		// would land on, ahead of any individual PR row.
		mainHealth.renderAlarms(w)
		policyAlarm.renderAlarms(w)
		for _, t := range rep.Tombstones {
			fmt.Fprintf(w, "%-11s #%-4d %s\n", shortRepo(t.Repo), t.Number, t.Note)
		}
		// decay banner — approval is perishable when main outruns the merge.
		if len(decayPRs) > 0 {
			for _, n := range decayPRs {
				for _, r := range rep.Rows {
					if r.Number == n && r.Action == actMergeNow {
						fmt.Fprintf(w, "DECAY: #%d approved %s ago (>%s threshold) — approval is perishable; merge it or it re-laps\n",
							n, r.ApprovedAge, mergeNowThreshold)
						break
					}
				}
			}
		}
		// #400 N7: the decay alarm's unmeasurable half, named rather than dropped. Same
		// shape as UNREVIEWED-AGE-UNKNOWN below: the verdict is known (APPROVED, merge
		// now), the AGE is not, and a blank APPROVED column reads exactly like "just
		// approved" — the silence the decay alarm exists to break.
		for _, n := range ageUnknownPRs {
			for _, r := range rep.Rows {
				if r.Number == n && r.Action == actMergeNow {
					fmt.Fprintf(w, "DECAY-AGE-UNKNOWN: %s#%d is MERGE-NOW but its approval age could NOT be "+
						"read (the approving review's submitted_at was absent or unparseable) — it cannot be "+
						"cleared against the %s threshold; check how long it has sat before quoting it as fresh\n",
						shortRepo(r.Repo), n, mergeNowThreshold)
					break
				}
			}
		}
		// #359: PRs the sweep has never reviewed, named before the rows. One line per
		// stranded PR, and NOTHING at all when every open PR has been looked at.
		for _, r := range unreviewedRows {
			fmt.Fprintf(w, "UNREVIEWED: %s#%d has been open %s with NO %s verdict at any head (>%s) — "+
				"a sweep starting now will not backfill it; pick it up explicitly\n",
				shortRepo(r.Repo), r.Number, displayDuration(r.OpenAge), reviewerBotDisplay(), unreviewedThreshold)
		}
		// #359: same alarm, unmeasurable half. Named separately so the reader is never
		// told an age the board does not have, and never told nothing at all.
		for _, r := range ageUnknownRows {
			fmt.Fprintf(w, "UNREVIEWED-AGE-UNKNOWN: %s#%d has NO %s verdict at any head and its open age "+
				"could NOT be read (createdAt missing or unparseable) — it cannot be cleared against the "+
				"%s threshold; check it by hand\n",
				shortRepo(r.Repo), r.Number, reviewerBotDisplay(), unreviewedThreshold)
		}
		renderScopeLine(w, rep.Header.Scope)
		renderPopulationLine(w, rep.Header.PRPopulation)
		fmt.Fprintf(w, "%-11s %-5s %-30s %-22s %-6s %-22s %-46s %s\n", "REPO", "PR", "ACTION", "CI", "SCORE", "BRIEF", "TITLE", "NOTE")
		for _, r := range rep.Rows {
			ci := ciSummary(r.CIPass, r.CIPending, r.CIFail, r.CIUnknown, r.CIZero)
			// MERGE-NOW rows carry approved-age in the ACTION column.
			actionDisplay := r.Action
			if r.ApprovedAge != "" && r.Action == actMergeNow {
				actionDisplay = fmt.Sprintf("%s (%s)", r.Action, displayDuration(r.ApprovedAge))
			}
			brief := r.OwningBrief
			if brief == "" {
				brief = "unowned"
			}
			fmt.Fprintf(w, "%-11s #%-4d %-30s %-22s %-6d %-22s %-46s %s\n",
				shortRepo(r.Repo), r.Number, actionDisplay, ci, r.Score, brief, title(r.Title, 46), r.Note)
		}
		if len(rep.Rows) == 0 && len(rep.Tombstones) == 0 {
			fmt.Fprintln(w, "(no open PRs in the watched set — see the scope line above for what that covers)")
		}
		renderExternal(w, "PR(s)", rep.External)
	}}, nil
}

// tombstonesFor reads prior deskboard audit lines (verb prs/actions) within the last
// hour, unions the open sets they recorded, and returns a tombstone for each id no
// longer in currentOpen. Best-effort: an unreadable/corrupt audit file yields no
// tombstones (the advisory layer never fails the board; corruption surfaces elsewhere).
//
// scope is the repo set the caller actually swept to build currentOpen (#489). The
// inference this function makes is "I saw it before and I do not see it now, so it
// merged", and that inference is only sound if the run could see ANYTHING at all: with
// an empty scope, currentOpen is empty for a reason that has nothing to do with the
// PRs, and EVERY remembered id becomes a fabricated merge. Reproduced against a seeded
// ledger: six remembered PRs, six "MERGED — drop from your list" lines, exit 0, while
// all six were open — one of them the PR carrying this fix.
//
// dispatch already refuses a board-wide verb with an empty set before cmdActions runs,
// so in the shipped binary this branch is unreachable. It is kept because the guard and
// the unsound inference live in different files: the inference must be safe on its own
// terms, not because of a check somewhere else that a later caller may not go through.
func tombstonesFor(currentOpen map[string]bool, scope []string) []tombstone {
	if len(scope) == 0 {
		return nil
	}
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return nil
	}
	cutoff := time.Now().Add(-time.Hour)
	priorOpen := map[string]bool{}
	for _, e := range entries {
		if e.Verb != "prs" && e.Verb != "actions" {
			continue
		}
		ts, perr := time.Parse(time.RFC3339, e.TS)
		if perr != nil || ts.Before(cutoff) {
			continue
		}
		for _, id := range parseOpenDetail(e.Detail) {
			priorOpen[id] = true
		}
	}
	var out []tombstone
	for id := range priorOpen {
		if currentOpen[id] {
			continue
		}
		repo, num, ok := splitID(id)
		if !ok {
			continue
		}
		// #247: absence proves the PR left the open set, never HOW. Ask.
		state, merged, detail := fetchPRState(repo, num)
		out = append(out, tombstone{
			Repo: repo, Number: num, State: state, Merged: merged,
			Note: tombstoneNote(state, detail),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Number > out[j].Number
	})
	return out
}

// ---- reviews ----

type reviewRow struct {
	Author         string `json:"author"`
	State          string `json:"state"`
	CommitID       string `json:"commitId"`
	AtHead         bool   `json:"atHead"`
	IsAppReviewer  bool   `json:"isAppReviewer"`
	SecurityMarker bool   `json:"securityMarker"`
	SubmittedAt    string `json:"submittedAt"`
}

type reviewsReport struct {
	Header
	Repo    string      `json:"repo"`
	Number  int         `json:"number"`
	HeadSHA string      `json:"headSHA"`
	Reviews []reviewRow `json:"reviews"`
}

func cmdReviews(hdr Header, repo string, num int) (*Report, error) {
	// The current head is needed to mark which verdicts land at head.
	prs, truncated, err := fetchOpenPRs(repo) // single repo; still GET-only
	if err != nil {
		return nil, err
	}
	head := ""
	for _, p := range prs {
		if p.Number == num {
			head = p.HeadRefOid
		}
	}
	// #400 T2: this verb reads a PR list too, so it states whether that read was complete.
	// It matters here for a specific reason: a truncated list that does not contain `num`
	// leaves `head` EMPTY, and an empty head makes every row's atHead false — "no verdict
	// at head" produced by a read that simply did not reach the PR.
	var truncatedRepos []string
	if truncated {
		truncatedRepos = []string{repo}
	}
	hdr.PRPopulation = newPRPopulation(truncatedRepos)
	reviews, err := fetchReviews(repo, num)
	if err != nil {
		return nil, err
	}
	rep := reviewsReport{Header: hdr, Repo: repo, Number: num, HeadSHA: head, Reviews: []reviewRow{}}
	for _, r := range reviews {
		rep.Reviews = append(rep.Reviews, reviewRow{
			Author:   r.User.Login,
			State:    r.State,
			CommitID: r.CommitID,
			// #400 Q3: the THIRD site of the same comparison, and the one that hand-rolled
			// it. It happened to be equivalent — but the argument of this PR is that "are
			// these two shas the same READ sha" belongs in one place, so that a later edit
			// cannot re-introduce absence-equals-absence on one site while the other two
			// stay right.
			AtHead:         sameHead(r.CommitID, head),
			IsAppReviewer:  isReviewerBot(r.User.Login),
			SecurityMarker: hasSecurityPassLine(r.Body),
			SubmittedAt:    r.SubmittedAt,
		})
	}
	return &Report{value: rep, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s  %s#%d head=%s\n", hdr.AsOf, shortRepo(repo), num, short(head))
		fmt.Fprintf(w, "%-28s %-18s %-10s %-6s %-6s %s\n", "AUTHOR", "STATE", "COMMIT", "HEAD", "APP", "SEC")
		for _, r := range rep.Reviews {
			fmt.Fprintf(w, "%-28s %-18s %-10s %-6t %-6t %t\n",
				r.Author, r.State, short(r.CommitID), r.AtHead, r.IsAppReviewer, r.SecurityMarker)
		}
		if len(rep.Reviews) == 0 {
			fmt.Fprintln(w, "(no reviews)")
		}
		// #400 U2: this verb reads the open-PR list too (for `head`), so a truncated read
		// gets the same human-facing warning `prs`/`actions` already carry — otherwise an
		// empty `head` from a capped, truncated list looks identical to "no verdict at head"
		// on the table path, exactly where a reviewer is reading it.
		renderPopulationLine(w, rep.Header.PRPopulation)
	}}, nil
}

// ---- queue ----

type issueRow struct {
	Repo   string   `json:"repo"`
	Number int      `json:"number"`
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	Labels []string `json:"labels"`
}

type queueReport struct {
	Header
	Issues   []issueRow    `json:"issues"`
	External []externalRow `json:"external"`
}

// issueBlessed is the issue twin of prBlessed (deskkit.IssueTrustQuery), for the
// verify-gate queue: called only for untrusted-author issues, fail closed.
func issueBlessed(repo string, num int) (bool, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return false, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	out, err := ghRun("api", "graphql",
		"-f", "query="+deskkit.IssueTrustQuery,
		"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(num))
	if err != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf("cannot read trust events for %s#%d (trust gate)", repo, num), err)
	}
	bodyEdited, events, complete, perr := deskkit.ParseIssueTrustPayload(out)
	if perr != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf("cannot parse trust events for %s#%d (trust gate)", repo, num), perr)
	}
	if !complete {
		return false, nil
	}
	return deskkit.Blessed(bodyEdited, events), nil
}

// gateIssue is one row of the verify-gate issue listing (REST /issues). The endpoint
// also returns PRs, distinguished by a non-nil pull_request member.
type gateIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	HTMLURL     string    `json:"html_url"`
	PullRequest *struct{} `json:"pull_request"`
	User        struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func cmdQueue(hdr Header) (*Report, error) {
	hdr.Scope = boardScope() // #359: a sweeping verb states its coverage
	rep := queueReport{Header: hdr, Issues: []issueRow{}, External: []externalRow{}}
	for _, repo := range deskkit.AllowedRepos() {
		// Page explicitly: the REST default of 30/page silently hid every verify-gate
		// issue past the thirtieth, and an invisible queue item is one nobody works.
		var issues []gateIssue
		for page := 1; ; page++ {
			out, err := ghRun("api", fmt.Sprintf("repos/%s/issues?labels=%s&state=open&per_page=%d&page=%d",
				repo, verifyGateLabel, apiPageSize, page))
			if err != nil {
				return nil, deskkit.Unverifiable("cannot read verify-gate issues for "+repo, err)
			}
			var chunk []gateIssue
			if err := json.Unmarshal(out, &chunk); err != nil {
				return nil, deskkit.Unverifiable("cannot parse verify-gate issues for "+repo, err)
			}
			issues = append(issues, chunk...)
			if len(chunk) < apiPageSize {
				break
			}
		}
		for _, is := range issues {
			if is.PullRequest != nil {
				continue // the issues endpoint also returns PRs; keep only real issues
			}
			// Trust gate: REST gives login AND numeric id here — both must match the
			// compiled-in identity (TrustedAuthorID, recycled-login defense).
			if !deskkit.TrustedAuthorID(is.User.Login, is.User.ID) {
				blessed, berr := issueBlessed(repo, is.Number)
				if berr != nil {
					return nil, berr
				}
				if !blessed {
					rep.External = append(rep.External, externalRow{Repo: repo, Number: is.Number, Title: is.Title, Author: is.User.Login})
					continue
				}
			}
			labels := make([]string, 0, len(is.Labels))
			for _, l := range is.Labels {
				labels = append(labels, l.Name)
			}
			rep.Issues = append(rep.Issues, issueRow{
				Repo: repo, Number: is.Number, Title: is.Title, URL: is.HTMLURL, Labels: labels,
			})
		}
	}
	sortExternal(rep.External)
	return &Report{value: rep, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s  (label: %s)\n", hdr.AsOf, verifyGateLabel)
		renderScopeLine(w, rep.Header.Scope)
		fmt.Fprintf(w, "%-11s %-6s %s\n", "REPO", "ISSUE", "TITLE")
		for _, is := range rep.Issues {
			fmt.Fprintf(w, "%-11s #%-5d %s\n", shortRepo(is.Repo), is.Number, title(is.Title, 60))
		}
		if len(rep.Issues) == 0 {
			fmt.Fprintln(w, "(no verify-gate issues in the watched set — see the scope line above)")
		}
		renderExternal(w, "issue(s)", rep.External)
	}}, nil
}

// ---- diff ----

type diffReport struct {
	Header
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Diff   string `json:"diff"`
}

func cmdDiff(hdr Header, repo string, num int) (*Report, error) {
	out, err := ghRun("pr", "diff", fmt.Sprint(num), "-R", repo)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("cannot read diff for %s#%d", repo, num), err)
	}
	rep := diffReport{Header: hdr, Repo: repo, Number: num, Diff: string(out)}
	return &Report{value: rep, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s  %s#%d\n", hdr.AsOf, shortRepo(repo), num)
		io.WriteString(w, rep.Diff)
		if !strings.HasSuffix(rep.Diff, "\n") {
			fmt.Fprintln(w)
		}
	}}, nil
}

// ---- files ----

type filesReport struct {
	Header
	Repo    string   `json:"repo"`
	Number  int      `json:"number"`
	Path    string   `json:"path,omitempty"`
	Files   []string `json:"files,omitempty"`
	Content string   `json:"content,omitempty"`
	// Truncated is true when fewer entries were read than GitHub reports for the PR —
	// a reviewer reading this list must know it is not the whole diff.
	Truncated bool `json:"truncated,omitempty"`
}

func cmdFiles(hdr Header, repo string, num int, path string) (*Report, error) {
	rep := filesReport{Header: hdr, Repo: repo, Number: num, Path: path}
	if path == "" {
		files, complete, err := fetchChangedFiles(repo, num)
		if err != nil {
			return nil, err
		}
		rep.Truncated = !complete
		for f := range files {
			rep.Files = append(rep.Files, f)
		}
		sort.Strings(rep.Files)
		return &Report{value: rep, render: func(w io.Writer) {
			fmt.Fprintf(w, "asOf %s  %s#%d changed files:\n", hdr.AsOf, shortRepo(repo), num)
			for _, f := range rep.Files {
				fmt.Fprintln(w, f)
			}
			if len(rep.Files) == 0 {
				fmt.Fprintln(w, "(no changed files)")
			}
			if rep.Truncated {
				fmt.Fprintln(w, "WARNING: this list is TRUNCATED — fewer entries than GitHub reports for the PR")
			}
		}}, nil
	}

	// Specific file: read its contents at the PR head (GET-only contents API).
	// #400 U1: this branch resolves the head via the same open-PR-list read `reviews`
	// makes, so it owes the same population statement — an absent `prPopulation` here
	// would read as "this verb never looked", which is false on exactly this path.
	head, truncated, err := headOfPR(repo, num)
	var truncatedRepos []string
	if truncated {
		truncatedRepos = []string{repo}
	}
	rep.Header.PRPopulation = newPRPopulation(truncatedRepos)
	if err != nil {
		return nil, err
	}
	out, err := ghRun("api", fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, path, head))
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("cannot read %s at %s#%d", path, repo, num), err)
	}
	var v struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("cannot parse contents of %s (%s#%d)", path, repo, num), err)
	}
	content := v.Content
	if v.Encoding == "base64" {
		dec, derr := base64.StdEncoding.DecodeString(strings.ReplaceAll(v.Content, "\n", ""))
		if derr != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("cannot base64-decode %s (%s#%d)", path, repo, num), derr)
		}
		content = string(dec)
	}
	rep.Content = content
	return &Report{value: rep, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s  %s#%d %s @ %s\n", hdr.AsOf, shortRepo(repo), num, path, short(head))
		io.WriteString(w, rep.Content)
		if !strings.HasSuffix(rep.Content, "\n") {
			fmt.Fprintln(w)
		}
		renderPopulationLine(w, rep.Header.PRPopulation)
	}}, nil
}

// headOfPR resolves a PR's current head sha (open PRs only; a closed/merged PR is not
// a reviewer read target and returns exit 6). The bool return states whether the open-PR
// list this read rests on was truncated at the --limit cap (#400 U1) — the caller reads a
// PR list here exactly as `reviews`/`prs`/`actions` do, and owes the same population
// statement rather than leaving it silently unset on this one path.
func headOfPR(repo string, num int) (string, bool, error) {
	prs, truncated, err := fetchOpenPRs(repo)
	if err != nil {
		return "", truncated, err
	}
	for _, p := range prs {
		if p.Number == num {
			return p.HeadRefOid, truncated, nil
		}
	}
	// #400 T2: absence from a TRUNCATED list proves nothing about the PR. "Not an open PR"
	// is a statement about the world; over a capped read it is a statement about the page
	// we happened to get. Both are exit 6, but they must not say the same thing.
	if truncated {
		return "", truncated, deskkit.Unverifiable(fmt.Sprintf(
			"cannot tell whether %s#%d is open: the open-PR list for %s came back at the --limit %d cap, "+
				"so it may be TRUNCATED and this PR may simply be past the cut (#80)", repo, num, repo, prListLimit), nil)
	}
	return "", truncated, deskkit.Unverifiable(fmt.Sprintf("%s#%d is not an open PR", repo, num), nil)
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

// openAgeOf is how long a PR has been open, "" when createdAt is missing or unparseable
// (an unknown age is reported as absent, never as zero — a zero age reads as "just
// opened", which is the fail-open answer here).
func openAgeOf(p prBase, now time.Time) string {
	if p.CreatedAt == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil {
		return ""
	}
	return formatDuration(now.Sub(t))
}

// ciSummary renders the CI counters, folding in both absence axes: `?unk` for
// rollup entries this board could not interpret (#268), on top of ciColumn's
// zero-rollup naming (#1652) — a healthy, non-zero rollup renders exactly as
// before, so neither axis costs anything on an ordinary board.
func ciSummary(pass, pending, fail, unknown int, zero string) string {
	s := ciColumn(pass, pending, fail, zero)
	if unknown > 0 {
		s += fmt.Sprintf(" %d?unk", unknown)
	}
	return s
}

func intersects(a, b map[string]bool) bool {
	if b == nil {
		return true // unknown ⇒ assume changed (forces RE-REVIEW — the safe side)
	}
	for f := range a {
		if b[f] {
			return true
		}
	}
	return false
}

// ownFilesChanged decides the MERGE-CURR / RE-REVIEW question: did the PR's OWN files
// change between the reviewed sha and the current head?
//
// `complete` is the load-bearing half and the reason this is a named function rather than
// one line inside cmdActions' loop (#400, same move as buildClassifyInput). A truncated
// own-files read cannot prove the intersection is EMPTY — measured on tracker#1618, `--json
// files` returned an alphabetical 100-entry window of 652 — so an incomplete read degrades
// to RE-REVIEW rather than the benign MERGE-CURR, whose note says "PR's own files unchanged
// since last review". That sentence over a partial read is a confident claim about files
// nobody listed. Inline, nothing could reach it: dropping `!complete` left the whole suite
// green.
func ownFilesChanged(own map[string]bool, complete bool, changed map[string]bool) bool {
	if !complete {
		return true
	}
	return intersects(own, changed)
}

func openDetail(ids []string) string {
	sort.Strings(ids)
	return "open=" + strings.Join(ids, " ")
}

func parseOpenDetail(detail string) []string {
	const p = "open="
	i := strings.Index(detail, p)
	if i < 0 {
		return nil
	}
	rest := strings.TrimSpace(detail[i+len(p):])
	if rest == "" {
		return nil
	}
	return strings.Fields(rest)
}

func splitID(id string) (repo string, num int, ok bool) {
	i := strings.LastIndex(id, "#")
	if i < 0 {
		return "", 0, false
	}
	var n int
	if _, err := fmt.Sscanf(id[i+1:], "%d", &n); err != nil {
		return "", 0, false
	}
	return id[:i], n, true
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// shortRepo renders a repo slug as the short label the boards print. It is a thin
// reading of the ONE shared deskkit resolver (deskkit.RepoShortLabel) so deskboard
// and issueboard render through a single mechanism: a GENERIC compiled default
// (short = the repo's last path segment) plus the adopter's ASSAY_REPO_ALIASES
// override, where the house short names now live instead of a switch compiled into
// this tree.
//
// The labels must be INJECTIVE over the census: two repos rendering the same string is
// not cosmetic on a board whose whole purpose is telling a human which repo an item is
// in. Collisions under the generic default are the alias config's job to resolve, and
// TestShortRepoLabelsAreInjective pins the property over the configured set.
func shortRepo(s string) string {
	return deskkit.RepoShortLabel(s)
}

func trunc(s string, n int) string {
	if len([]rune(s)) > n {
		r := []rune(s)
		return string(r[:n-1]) + "…"
	}
	return s
}

// formatDuration returns a Go-parseable duration string for d, rounded to the minute
// (or second if under a minute). Returns "" if d is zero. The standard Go duration
// format is used so callers can time.ParseDuration the result.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	return d.Round(time.Minute).String()
}

// displayDuration strips trailing zero units from a Go duration string for compact
// table display (e.g. "5m0s" → "5m", "1h23m0s" → "1h23m", "2h0m0s" → "2h").
// It only strips "0s" when it follows "m" (Xm0s → Xm), and only strips "0m" when
// it follows "h" (Xh0m → Xh), so it never mangles the "0" inside "10m"/"20m"/"40s".
func displayDuration(s string) string {
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

// inert renders public-origin text (titles, author strings) as quoted terminal-safe
// data — control characters, ANSI escapes, and quotes are escaped — so quarantine
// output is DATA, never an instruction channel.
func inert(s string, n int) string {
	return strconv.Quote(trunc(s, n))
}

// title renders a public-origin title into an ACTIONABLE lane: control characters and
// ANSI escapes are stripped first, then it is truncated to n columns. Stripping before
// truncating is required both ways round — an escape sequence must not survive, and it
// must not consume display columns it never occupies. The quarantine lanes keep inert()
// instead, which QUOTES the payload so an injection attempt stays visible as evidence.
func title(s string, n int) string {
	return trunc(deskkit.StripControl(s), n)
}

// ---- policydrift ----
//
// The desk decides risk-classification from a COMPILED-IN visibility value
// (deskkit/config.go), never a live API read: a gate whose decision needs the network
// is a gate that fails when the network does, and the whole allowed-repo set is
// compiled in for the same reason. The price of a hand-maintained value is that it can
// go stale — which is the drift anti-pattern verbatim, a parallel list drifting until it silently
// disabled a gate. This subcommand is the antidote: it reads each allowed repo's real
// visibility (GET-only) and FAILS LOUD (exit 6) on any disagreement, so the drift is
// discovered by a check rather than by an unreviewed public PR.

type visibilityRow struct {
	Repo       string `json:"repo"`
	CompiledIn string `json:"compiledIn"`
	Observed   string `json:"observed"`
	RiskClass  bool   `json:"riskClassedByVisibility"`
}

type policyDriftReport struct {
	Header
	Repos []visibilityRow `json:"repos"`
	Drift []string        `json:"drift"`
}

// repoMeta is the slice of GET repos/<repo> this check consumes.
type repoMeta struct {
	Visibility string `json:"visibility"`
	Private    bool   `json:"private"`
}

func cmdPolicyDrift(hdr Header) (*Report, error) {
	hdr.Scope = boardScope() // #359: a sweeping verb states its coverage
	observed := make(map[string]string, len(deskkit.AllowedRepos()))
	rep := policyDriftReport{Header: hdr, Repos: []visibilityRow{}, Drift: []string{}}
	var mismatch []string

	for _, repo := range deskkit.AllowedRepos() {
		out, err := ghRun("api", "repos/"+repo)
		if err != nil {
			// a repo we could not read is NOT a pass. Fail the whole run.
			return nil, deskkit.Unverifiable("cannot read repo metadata for "+repo, err)
		}
		var m repoMeta
		if err := json.Unmarshal(out, &m); err != nil {
			return nil, deskkit.Unverifiable("cannot parse repo metadata for "+repo, err)
		}
		// Cross-check the two fields GitHub returns. They should never disagree; if
		// they do, we do not know which to believe, so we say so rather than pick.
		switch {
		case m.Visibility == "public" && m.Private:
			mismatch = append(mismatch, repo+": API self-contradicts — visibility=\"public\" but private=true")
		case m.Visibility == "private" && !m.Private:
			mismatch = append(mismatch, repo+": API self-contradicts — visibility=\"private\" but private=false")
		}
		observed[repo] = m.Visibility
		rep.Repos = append(rep.Repos, visibilityRow{
			Repo:       repo,
			CompiledIn: deskkit.RepoVisibility(repo).String(),
			Observed:   m.Visibility,
			RiskClass:  deskkit.VisibilityRiskClassed(repo),
		})
	}

	rep.Drift = append(deskkit.VisibilityDrift(observed), mismatch...)
	if len(rep.Drift) > 0 {
		return nil, deskkit.Unverifiable("repo visibility policy DRIFT — the compiled-in table no longer "+
			"matches GitHub; the risk-class gate is deciding on stale facts:\n  "+
			strings.Join(rep.Drift, "\n  "), nil)
	}

	return &Report{value: rep, detail: fmt.Sprintf("policydrift ok (%d repos)", len(rep.Repos)),
		render: func(w io.Writer) {
			fmt.Fprintf(w, "asOf %s\n", hdr.AsOf)
			fmt.Fprintf(w, "%-38s %-11s %-11s %s\n", "REPO", "COMPILED-IN", "OBSERVED", "RISK-CLASSED")
			for _, r := range rep.Repos {
				fmt.Fprintf(w, "%-38s %-11s %-11s %t\n", r.Repo, r.CompiledIn, r.Observed, r.RiskClass)
			}
			fmt.Fprintln(w, "no drift — compiled-in visibility matches GitHub")
			renderScopeLine(w, rep.Header.Scope)
		}}, nil
}

// ---- policydrift, riding on `actions` (an internal hardening review) ----
//
// cmdPolicyDrift above is the fail-closed GATE: a human (or a script) has to type
// `deskboard policydrift`, and it exits 6 the moment the compiled-in table disagrees with
// GitHub. The internal review found nothing periodic ever typed it — no workflow, no loop, no
// cron — which is exactly the "control fires when a human remembers, and never otherwise"
// gap that produced the drift anti-pattern incident this check exists to catch. assessPolicyDrift is the
// always-on companion: it rides the Header the same way assessBranchHealth (#295) rides
// `actions`, so every ordinary board read carries the signal without anyone having to ask
// for it separately.
//
// It deliberately does NOT fail the run. `actions` is the desk's primary read — the PR
// sweep that every review/verify loop iteration consumes — and turning one repo's
// unreadable metadata into a dead board is the same false economy #295's doc comment
// already rejected for branch health. A repo whose visibility could not be read is simply
// left out of the `observed` map; VisibilityDrift then reports it "NOT OBSERVED" (an
// unchecked repo is drift, never a pass — see config.go), so the loud failure still
// happens, just as a Drift entry instead of a killed run. The fail-closed exit-6 gate is
// still there for whoever wants it: `deskboard policydrift`.
type policyDriftAlarm struct {
	Scope []string `json:"scope"`
	Drift []string `json:"drift,omitempty"`
}

// assessPolicyDrift probes every allowed repo's real visibility (GET-only) and compares it
// to the compiled-in table, exactly like cmdPolicyDrift, but never returns an error — see
// the doc comment above.
func assessPolicyDrift() policyDriftAlarm {
	scope := deskkit.AllowedRepos()
	alarm := policyDriftAlarm{Scope: scope}
	observed := make(map[string]string, len(scope))
	var mismatch []string

	for _, repo := range scope {
		out, err := ghRun("api", "repos/"+repo)
		if err != nil {
			continue // left out of `observed` → VisibilityDrift reports it NOT OBSERVED
		}
		var m repoMeta
		if err := json.Unmarshal(out, &m); err != nil {
			continue // same as above: unparseable is unobserved, not a guessed pass
		}
		switch {
		case m.Visibility == "public" && m.Private:
			mismatch = append(mismatch, repo+": API self-contradicts — visibility=\"public\" but private=true")
		case m.Visibility == "private" && !m.Private:
			mismatch = append(mismatch, repo+": API self-contradicts — visibility=\"private\" but private=false")
		}
		observed[repo] = m.Visibility
	}

	alarm.Drift = append(deskkit.VisibilityDrift(observed), mismatch...)
	return alarm
}

// renderAlarms writes the POLICY-DRIFT line(s), if any, followed by the always-printed
// summary — same shape as branchHealthReport.renderAlarms, so a healthy board stays one
// line and an unhealthy one cannot be silent.
func (a policyDriftAlarm) renderAlarms(w io.Writer) {
	for _, d := range a.Drift {
		fmt.Fprintf(w, "POLICY-DRIFT: %s — compiled-in visibility table is stale; "+
			"the risk-class gate may be deciding on wrong facts\n", d)
	}
	fmt.Fprintln(w, a.summaryLine())
}

func (a policyDriftAlarm) summaryLine() string {
	return fmt.Sprintf("policy-drift: %d drifted — scope: %d repo(s)", len(a.Drift), len(a.Scope))
}

// auditDetail records the verdict in the audit line, mirroring branchHealthReport's.
func (a policyDriftAlarm) auditDetail() string {
	if len(a.Drift) == 0 {
		return fmt.Sprintf("policyDrift clean scope=%d", len(a.Scope))
	}
	return fmt.Sprintf("policyDrift DRIFT=%d scope=%d", len(a.Drift), len(a.Scope))
}

// blessAuthorityName renders the configured blessing authority for a human-facing
// line. With the roster unconfigured there IS no authority, and the message says so
// rather than naming nobody — an unconfigured gate must read as refused, never as
// quiet.
func blessAuthorityName() string {
	if l := deskkit.BlessAuthorityLogin(); l != "" {
		return l
	}
	return "the (UNCONFIGURED) blessing authority"
}
