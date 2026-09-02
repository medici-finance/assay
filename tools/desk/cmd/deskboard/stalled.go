package main

// stalled.go — `deskboard stalled`: the stalled-draft detector + shepherd dispatch list
// It surfaces the disease the review loop's persistence assumption
// produces: a draft that the reviewer requested changes on, and whose author then went
// silent. Sessions die; nothing notices. stalled turns "silent for 48h" into a row on a
// dispatch list so abandonment is a routed event, not a hidden state (#160).
//
// Read-only: every gh call here is a GET (pr list, reviews, a commit read, an
// issue-comments read, a compare). No mutation — the PATH-shim test enumerates it.
//
// Stall definition (the brief's tuned starting point):
//
//	OPEN + DRAFT + latest reviewer-App verdict AT HEAD == CHANGES_REQUESTED
//	         + head commit older than --min-age-hours (the "no push" clock)
//	         + no PR comment by the PR author in that window (the "no responder" clock)
//
// The App verdict comes from the ONE deskkit reduction (#268 — deskkit.ReduceAppVerdict),
// never a fourth ad-hoc reducer. "No push" is measured from the head commit's committer
// date; "no responder activity" from the PR conversation comments by the author.
//
// Fail-LABELED, not fail-closed, at the per-PR granularity (Task 3 / #236): an API error
// on ONE PR's signals yields an `unassessable` row and exit 0 for the rest — partial
// data is labeled, never fabricated as a clean board. A whole-repo PR-list fetch failure
// still fails the run (exit 6): if a repo's open PRs cannot be enumerated, "no stalled
// drafts there" is a claim nothing verified, which is the #236 fail-open shape exactly.
//
// Disposition is ADVISORY. PR ownership is invisible (never auto-adopt); `shepherd`
// is the default, `close-candidate` flags a PR >200 commits
// behind main or whose owning brief is done. The desk/human decides.

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// behindMainCloseThreshold: a stalled draft this far behind main is effectively
// abandoned on a diverged branch — flagged `close-candidate` (advisory). The brief's
// threshold.
const behindMainCloseThreshold = 200

// stalledMinAgeDefault is the tuned starting-point stall window (hours).
const stalledMinAgeDefault = 48

type stalledRow struct {
	Repo         string  `json:"repo"`
	Number       int     `json:"number"`
	Title        string  `json:"title"`
	HeadSHA      string  `json:"headSHA"`
	DaysStalled  float64 `json:"daysStalled"`
	LastReviewAt string  `json:"lastReviewAt,omitempty"`
	LastPushAt   string  `json:"lastPushAt"`
	// LastPushBy is the GitHub login the head commit is attributed to, omitted when
	// GitHub attributes it to no account. It is reported because lastPushAt alone is
	// ambiguous: a push by someone OTHER than the author is merge-currency traffic, not
	// the author working, and it does not stop the stall clock (see PushIsAuthors).
	LastPushBy string `json:"lastPushBy,omitempty"`
	// PushIsAuthors states whether the head commit counted as author activity. False
	// means the newest push on this branch was somebody else's — the row is stalled
	// DESPITE a recent push, and a consumer eyeballing lastPushAt would otherwise read
	// the row as a contradiction.
	PushIsAuthors       bool   `json:"pushIsAuthors"`
	LastAuthorCommentAt string `json:"lastAuthorCommentAt,omitempty"`
	// OpenFindings is the unresolved review-thread count. nil = not cheaply available:
	// computing it needs a per-PR review-thread resolution GraphQL read, which is not a
	// cheap read, so the field is omitted rather than fabricated (the #236 principle
	// applied to a count, not just a boolean).
	OpenFindings *int   `json:"openFindings,omitempty"`
	BehindMain   int    `json:"behindMain"`  // -1 = not assessed (advisory hint only)
	Disposition  string `json:"disposition"` // shepherd | close-candidate
	Note         string `json:"note,omitempty"`
}

type unassessableRow struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	// Reason names WHICH signal could not be assessed (reviews / head-commit / comments),
	// so a desk triaging the row knows what to re-check by hand.
	Reason string `json:"reason"`
}

type stalledReport struct {
	Header
	MinAgeHours int          `json:"minAgeHours"`
	Stalled     []stalledRow `json:"stalled"`
	// Truncated names the repos whose open-PR enumeration came back AT the listing cap,
	// so some open PRs were never examined. fetchOpenPRs already WARNs about this — but
	// on stderr, where the `--json` consumer cannot see it, and this verb's whole premise
	// is that silence must be distinguishable from blindness. An empty list is a positive
	// statement that every repo enumerated completely.
	Truncated    []string          `json:"truncated"`
	Unassessable []unassessableRow `json:"unassessable"`
}

func cmdStalled(hdr Header, minAgeHours int) (*Report, error) {
	if minAgeHours <= 0 {
		minAgeHours = stalledMinAgeDefault
	}
	minAge := time.Duration(minAgeHours) * time.Hour
	now := time.Now()
	hdr.Scope = boardScope() // #359 / #400: a sweeping verb states its coverage
	rep := stalledReport{
		Header:       hdr,
		MinAgeHours:  minAgeHours,
		Stalled:      []stalledRow{},
		Truncated:    []string{},
		Unassessable: []unassessableRow{},
	}
	var stalledIDs []string

	// The reviewer App login is CONFIGURATION, not a compiled-in constant
	// — `stalled` arrived from main still using the old
	// `reviewerBot` const, and this is the merge port of those three call sites.
	//
	// An unbound role resolves to "", which is the right value to pass on: SameActor
	// guards `an != ""`, so "" matches NO review author — including the `"user": null`
	// deleted-author case that decodes to an empty login. A PR then contributes no
	// decisive App review and is simply not reported as blocked-on-worker, rather than
	// every review counting as the reviewer's. That is the same direction isReviewerBot
	// takes for the flip decision.
	reviewerLogin, _ := deskkit.RoleAppLogin("reviewer")

	// Brief→status map is best-effort for the close-candidate hint. Unlike `actions`,
	// stalled does NOT gate on statusgen: a missing/unparseable gate-scores output yields
	// an empty map and the hint stays "shepherd". Never let the advisory layer fail the board.
	briefStatus := fetchBriefStatusBestEffort()
	knownBriefs := knownBriefIDs(briefStatus)

	for _, repo := range deskkit.AllowedRepos() {
		prs, _, err := fetchOpenPRs(repo)
		if err != nil {
			// Whole-repo enumeration failure: fail-closed (exit 6). "No stalled drafts in
			// repo X" cannot be claimed when repo X's open PRs could not be read (#236).
			return nil, err
		}
		if len(prs) >= prListLimit {
			// At the cap: some open PRs in this repo were never enumerated, so a clean
			// board for it means "nothing stalled among the ones we saw", not "nothing
			// stalled". Say so in the report shape both audiences read.
			rep.Truncated = append(rep.Truncated, repo)
		}
		for _, p := range prs {
			// #247: MERGED / CLOSED / OPEN are distinguished HERE, before any branch
			// comparison runs. A merged or closed PR is never a stalled draft — it is
			// finished — and its head compared against main reads "behind by N" exactly
			// like a live branch, which is the misread #247 names.
			kind := deskkit.ParsePRKind(p.State)
			switch kind {
			case deskkit.PRMerged, deskkit.PRClosed:
				continue // finished; never a dispatch row
			case deskkit.PRUnknown:
				// The enumeration asked for --state open, so an absent/garbled state means
				// the read did not tell us what we asked for. Label it — do NOT assume
				// open and do NOT drop it silently (#236 both ways).
				rep.Unassessable = append(rep.Unassessable, unassessableRow{
					Repo: repo, Number: p.Number, Title: p.Title,
					Reason: "open/merged/closed state not stated by the PR list (got " +
						strconv.Quote(p.State) + ")",
				})
				continue
			}

			// Cheap filters next: not-a-draft cannot be stalled; never listed.
			if !p.IsDraft {
				continue
			}
			// Review verdict (the #268 deskkit reduction). A fetch failure on one PR is
			// labeled unassessable; the rest of the board still renders (Task 3).
			reviews, rerr := fetchReviews(repo, p.Number)
			if rerr != nil {
				rep.Unassessable = append(rep.Unassessable, unassessableRow{
					Repo: repo, Number: p.Number, Title: p.Title,
					Reason: "cannot read reviewer-App verdict (" + errShort(rerr) + ")",
				})
				continue
			}
			appReviews := toAppReviews(reviews)
			// The ONE shared snapshot (#268) — stalled reads its fields, it does not
			// re-derive them. CIVerdict rides along free (the rollup came with the PR
			// list); stalled does not gate on it, but the snapshot is the shape every
			// consumer gets, not a stalled-shaped subset.
			st := deskkit.PRState{
				HeadSHA:    p.HeadRefOid,
				Draft:      p.IsDraft,
				State:      kind,
				AppVerdict: deskkit.ReduceAppVerdict(reviewerLogin, appReviews, p.HeadRefOid),
				CIVerdict:  deskkit.ReduceCIVerdict(toCIChecks(p.StatusCheckRollup), deskkit.CIRequired(repo)),
			}
			if !st.AppVerdict.IsBlockingAtHead() {
				continue // latest decisive App review at head is not CHANGES_REQUESTED
			}

			// Candidate: open draft blocking at head. Now the two stall clocks. If EITHER
			// signal is unassessable, the row is `unassessable` — never silently dropped,
			// never reported stalled/clean on a signal we did not read (#236).
			headTime, headBy, herr := fetchHeadCommit(repo, p.HeadRefOid)
			if herr != nil {
				rep.Unassessable = append(rep.Unassessable, unassessableRow{
					Repo: repo, Number: p.Number, Title: p.Title,
					Reason: "cannot read head-commit push date (" + errShort(herr) + ")",
				})
				continue
			}
			lastComment, cerr := fetchLastAuthorComment(repo, p.Number, p.Author.Login)
			if cerr != nil {
				rep.Unassessable = append(rep.Unassessable, unassessableRow{
					Repo: repo, Number: p.Number, Title: p.Title,
					Reason: "cannot read author comments (" + errShort(cerr) + ")",
				})
				continue
			}

			// lastActivity = the most recent activity BY THE AUTHOR, across both channels
			// (a push, a reply). A PR is stalled when BOTH clocks are past the window —
			// the author neither pushed a fix NOR replied in minAge. If only one is past,
			// the author is still active on the other channel and the PR is not stalled.
			//
			// The push clock counts ONLY a push attributable to the author. A
			// merge-currency push by anyone else — `prsync --push` merges origin/main and
			// pushes, and the desk's loop tells workers to merge main on conflict — would
			// otherwise reset the clock on a branch its author abandoned months ago,
			// hiding it from the list forever. That is the one FALSE-QUIET direction in
			// this verb, so it is closed rather than merely noted.
			//
			// Unknown attribution (`headBy == ""`, a commit GitHub maps to no account) is
			// NOT read as "somebody else": that would invent a stall from missing data.
			// It keeps the push on the author's clock and the row, if any, says so.
			pushIsAuthors := headBy == "" || deskkit.SameActor(headBy, p.Author.Login)

			var lastActivity time.Time
			if pushIsAuthors {
				lastActivity = headTime
			}
			authorCommented := !lastComment.IsZero()
			if authorCommented && lastComment.After(lastActivity) {
				lastActivity = lastComment
			}
			if lastActivity.IsZero() {
				// Nothing at all is attributable to the author: the only push was somebody
				// else's and there is no reply. The clock then runs from the moment the
				// ball entered the author's court — the decisive review — because that is
				// the last thing that provably happened, and dating the stall from a
				// stranger's merge commit would understate it. No parseable review
				// timestamp falls back to the head commit rather than to the zero time,
				// which would report a stall measured in millennia.
				lastActivity = headTime
				if last, ok := deskkit.LastAppDecisiveReview(reviewerLogin, appReviews); ok {
					if t, perr := time.Parse(time.RFC3339, last.SubmittedAt); perr == nil {
						lastActivity = t
					}
				}
			}
			if !isStalled(now, lastActivity, minAge) {
				continue // active inside the window on at least one channel
			}

			// Stalled. Disposition is advisory and best-effort: a fetch failure degrades
			// to "shepherd" (the safe default), never a fabricated close-candidate.
			behind, _ := fetchBehindMain(repo, st.HeadSHA)
			disposition, note := stallDisposition(repo, p, behind, briefStatus, knownBriefs)

			row := stalledRow{
				Repo:          repo,
				Number:        p.Number,
				Title:         p.Title,
				HeadSHA:       st.HeadSHA,
				DaysStalled:   daysRounded(now.Sub(lastActivity)),
				LastPushAt:    headTime.UTC().Format(time.RFC3339),
				LastPushBy:    headBy,
				PushIsAuthors: pushIsAuthors,
				Disposition:   disposition,
				BehindMain:    behind,
				Note:          note,
			}
			if authorCommented {
				row.LastAuthorCommentAt = lastComment.UTC().Format(time.RFC3339)
			}
			if last, ok := deskkit.LastAppDecisiveReview(reviewerLogin, appReviews); ok {
				row.LastReviewAt = last.SubmittedAt
			}
			rep.Stalled = append(rep.Stalled, row)
			stalledIDs = append(stalledIDs, fmt.Sprintf("%s#%d", repo, p.Number))
		}
	}

	sort.SliceStable(rep.Stalled, func(i, j int) bool {
		if rep.Stalled[i].DaysStalled != rep.Stalled[j].DaysStalled {
			return rep.Stalled[i].DaysStalled > rep.Stalled[j].DaysStalled // most stalled first
		}
		if rep.Stalled[i].Repo != rep.Stalled[j].Repo {
			return rep.Stalled[i].Repo < rep.Stalled[j].Repo
		}
		return rep.Stalled[i].Number > rep.Stalled[j].Number
	})
	sortUnassessable(rep.Unassessable)

	sort.Strings(rep.Truncated)

	detail := fmt.Sprintf("stalled=%d unassessable=%d", len(rep.Stalled), len(rep.Unassessable))
	if len(rep.Truncated) > 0 {
		detail += " truncated=" + strings.Join(rep.Truncated, ",")
	}
	if len(stalledIDs) > 0 {
		sort.Strings(stalledIDs)
		detail += " " + strings.Join(stalledIDs, " ")
	}
	return &Report{value: rep, detail: detail, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s  (stall window %dh; stalled %d, unassessable %d)\n",
			hdr.AsOf, minAgeHours, len(rep.Stalled), len(rep.Unassessable))
		for _, repo := range rep.Truncated {
			fmt.Fprintf(w, "TRUNCATED %s hit the %d-PR listing cap — open PRs beyond it were never examined\n",
				shortRepo(repo), prListLimit)
		}
		if len(rep.Stalled) == 0 {
			fmt.Fprintln(w, "(no stalled drafts)")
		} else {
			fmt.Fprintf(w, "%-8s %-5s %-6s %-22s %-10s %-16s %s\n",
				"REPO", "PR", "DAYS", "LAST-REVIEW", "HEAD", "DISPOSITION", "TITLE")
			for _, r := range rep.Stalled {
				last := r.LastReviewAt
				if last == "" {
					last = "?"
				}
				fmt.Fprintf(w, "%-8s #%-4d %-6.1f %-22s %-10s %-16s %s\n",
					shortRepo(r.Repo), r.Number, r.DaysStalled, trunc(last, 22),
					short(r.HeadSHA), r.Disposition, title(r.Title, 44))
			}
		}
		for _, r := range rep.Unassessable {
			fmt.Fprintf(w, "UNASSESSABLE %-8s #%-4d %s — %s\n",
				shortRepo(r.Repo), r.Number, title(r.Title, 40), r.Reason)
		}
	}}, nil
}

// stallDisposition returns the advisory disposition + a reason note. "shepherd" is the
// safe default; "close-candidate" requires POSITIVE evidence (behind main > threshold, or
// owning brief done). A signal that could not be assessed never produces close-candidate
// — fabrication risk runs one way here.
func stallDisposition(repo string, p prBase, behindMain int, briefStatus map[string]string, knownBriefs []string) (string, string) {
	if behindMain > behindMainCloseThreshold {
		return "close-candidate", fmt.Sprintf("behind main by %d commits (>%d) — branch likely diverged past recovery",
			behindMain, behindMainCloseThreshold)
	}
	if brief := mapBranchToBrief(p.HeadRefName, knownBriefs); brief != "" {
		if st := lookupBriefStatus(briefStatus, repo, brief); st == "done" || st == "superseded" {
			return "close-candidate", fmt.Sprintf("owning brief %s is %s — work already finished; this draft is orphaned",
				brief, st)
		}
	}
	return "shepherd", ""
}

// ---- per-PR reads (every one GET-only; the PATH-shim test enumerates them) ----

// fetchHeadCommit returns the committer date of repo's commit sha — the "no push" stall
// clock — and the GitHub login the commit is attributed to. The committer date is when
// the head commit landed on the branch, so it is the honest "last push" timestamp
// (author date can predate the push).
//
// `by` is the commit's GitHub committer login, falling back to its author login, in the
// REST rendering ("<slug>[bot]" for an App). It is "" when GitHub attributes the commit
// to no account at all — a commit whose email maps to no user carries a null `committer`
// AND a null `author`. Callers must treat "" as UNKNOWN attribution, never as "not the
// author": see the stall-clock comment at the call site.
func fetchHeadCommit(repo, sha string) (when time.Time, by string, err error) {
	if sha == "" {
		return time.Time{}, "", deskkit.Unverifiable("head sha empty for "+repo, nil)
	}
	out, gerr := ghRun("api", fmt.Sprintf("repos/%s/commits/%s", repo, sha))
	if gerr != nil {
		return time.Time{}, "", deskkit.Unverifiable(fmt.Sprintf("cannot read head commit for %s @ %s", repo, short(sha)), gerr)
	}
	var v struct {
		// Top-level author/committer are the GITHUB ACCOUNTS GitHub resolved the commit
		// to; `commit.author`/`commit.committer` are the raw git name/email. Only the
		// former is an identity comparable to a PR author login.
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
	if err := json.Unmarshal(out, &v); err != nil {
		return time.Time{}, "", deskkit.Unverifiable(fmt.Sprintf("cannot parse head commit for %s @ %s", repo, short(sha)), err)
	}
	if v.Commit.Committer.Date == "" {
		return time.Time{}, "", deskkit.Unverifiable(fmt.Sprintf("head commit %s @ %s has no committer date", repo, short(sha)), nil)
	}
	t, perr := time.Parse(time.RFC3339, v.Commit.Committer.Date)
	if perr != nil {
		return time.Time{}, "", deskkit.Unverifiable(fmt.Sprintf("head commit date for %s @ %s is not RFC3339: %s",
			repo, short(sha), v.Commit.Committer.Date), perr)
	}
	switch {
	case v.Committer != nil && v.Committer.Login != "":
		by = v.Committer.Login
	case v.Author != nil && v.Author.Login != "":
		by = v.Author.Login
	}
	return t, by, nil
}

// isStalled reports whether the last activity is outside the stall window at `now`.
//
// The boundary is exact and deliberate: a stall requires STRICTLY MORE than minAge of
// silence, so activity exactly minAge old is still ACTIVE. That matches the brief's
// "silent for > 48h" and keeps the tie in the direction that does not dispatch a worker
// at a PR someone is holding. It is a one-character difference from the wrong answer, so
// it is pinned by exact-value cases rather than by wall-clock fixtures, which can never
// land on the tick.
func isStalled(now, lastActivity time.Time, minAge time.Duration) bool {
	return now.Sub(lastActivity) > minAge
}

// fetchLastAuthorComment returns the created_at of the most recent PR conversation
// comment by authorLogin, or the zero time if the author has never commented. The
// conversation-comments endpoint (/issues/<n>/comments) is where an author responds to a
// CHANGES_REQUESTED review; review-comment threads are a separate channel and are NOT
// counted here (documented scope — the dispatch consumer reads this as "no reply on the
// main thread").
//
// authorLogin arrives in the gh CLI's rendering ("app/<slug>" for an App) while this
// endpoint renders the same App as "<slug>[bot]", so the match runs through
// deskkit.SameActor. Plain string equality here would answer "not the author" for EVERY
// App-authored PR, silently killing the responder clock on most of the watched repos and
// omitting lastAuthorCommentAt — which a dispatch consumer reads as the established
// fact "the author never replied".
func fetchLastAuthorComment(repo string, num int, authorLogin string) (time.Time, error) {
	var last time.Time
	for page := 1; ; page++ {
		out, err := ghRun("api", fmt.Sprintf("repos/%s/issues/%d/comments?per_page=%d&page=%d",
			repo, num, apiPageSize, page))
		if err != nil {
			return time.Time{}, deskkit.Unverifiable(fmt.Sprintf("cannot read comments for %s#%d", repo, num), err)
		}
		var chunk []struct {
			User struct {
				Login string `json:"login"`
			} `json:"user"`
			CreatedAt string `json:"created_at"`
		}
		if err := json.Unmarshal(out, &chunk); err != nil {
			return time.Time{}, deskkit.Unverifiable(fmt.Sprintf("cannot parse comments for %s#%d", repo, num), err)
		}
		for _, c := range chunk {
			if !deskkit.SameActor(c.User.Login, authorLogin) || c.CreatedAt == "" {
				continue
			}
			if t, perr := time.Parse(time.RFC3339, c.CreatedAt); perr == nil && t.After(last) {
				last = t
			}
		}
		if len(chunk) < apiPageSize {
			return last, nil
		}
	}
}

// stalledDefaultBranch is the branch a PR head is compared against. Every repo in the
// compiled-in set targets `main`; deskkit's repoPolicy states no per-repo default branch,
// so this is a named constant rather than a value smuggled into a format string.
const stalledDefaultBranch = "main"

// fetchBehindMain returns how many commits repo's default branch is ahead of headSHA
// (the PR's "behind by" count), for the advisory close-candidate hint. Best-effort: any
// error returns -1 and the disposition stays "shepherd". Open-PR only by construction
// (stalled calls this solely for open stalled PRs), so a MERGED PR's head never reaches
// the compare (#247 — merged-vs-closed is load-bearing exactly where compare is involved).
func fetchBehindMain(repo, headSHA string) (int, error) {
	if headSHA == "" {
		return -1, deskkit.Unverifiable("behind-main compare needs a head sha for "+repo, nil)
	}
	out, err := ghRun("api", fmt.Sprintf("repos/%s/compare/%s...%s", repo, stalledDefaultBranch, headSHA))
	if err != nil {
		return -1, err
	}
	var v struct {
		BehindBy int    `json:"behind_by"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return -1, err
	}
	// A compare with no `status` is a body this code did not understand. Returning its
	// zero `behind_by` would read as "0 commits behind — perfectly current", which is the
	// #236 fail-open shape: report NOT ASSESSED instead, and the disposition stays
	// "shepherd" rather than becoming a fabricated close-candidate either way.
	if v.Status == "" {
		return -1, deskkit.Unverifiable(fmt.Sprintf("compare %s %s...%s returned no status field",
			repo, stalledDefaultBranch, short(headSHA)), nil)
	}
	return v.BehindBy, nil
}

// fetchBriefStatusBestEffort returns a brief→lifecycle-status map from statusgen
// --gate-scores, for the close-candidate hint. Best-effort: any error (no statusgen on
// PATH, unparseable JSON, no roots) yields an empty map and the caller falls back to the
// "shepherd" default. Unlike `actions`, stalled does NOT fail on gate-scores — the hint
// is advisory and must not couple board availability to a statusgen release being live.
//
// Keys are REPO-QUALIFIED where the row states a repo ("<repo>|<brief>"), because brief
// IDs are only unique within a repo root: multi-root statusgen emits a brief like
// `stream/04` from one repo and could emit the same stream name from another root. An
// unqualified key would let one repo's finished brief mark another repo's live PR
// close-candidate. Rows from older statusgen releases carry no `repo` and land under the
// bare brief ID, which the lookup consults only as a fallback.
func fetchBriefStatusBestEffort() map[string]string {
	raw, err := execGateScores()
	if err != nil {
		return nil
	}
	var rows []gateScoreRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.Repo != "" {
			m[r.Repo+"|"+r.Brief] = r.Status
			continue
		}
		m[r.Brief] = r.Status
	}
	return m
}

// lookupBriefStatus resolves a brief's lifecycle status for repo: the repo-qualified key
// first, then the unqualified fallback for pre-multi-root gate-scores output.
func lookupBriefStatus(briefStatus map[string]string, repo, brief string) string {
	if st, ok := briefStatus[repo+"|"+brief]; ok {
		return st
	}
	return briefStatus[brief]
}

// knownBriefIDs returns the brief IDs present in a (possibly repo-qualified) status map,
// for mapBranchToBrief's branch-as-claim match.
func knownBriefIDs(briefStatus map[string]string) []string {
	seen := make(map[string]bool, len(briefStatus))
	out := make([]string, 0, len(briefStatus))
	for k := range briefStatus {
		brief := k
		if i := strings.Index(k, "|"); i >= 0 {
			brief = k[i+1:]
		}
		if brief == "" || seen[brief] {
			continue
		}
		seen[brief] = true
		out = append(out, brief)
	}
	sort.Strings(out)
	return out
}

// toCIChecks lifts the PR-list status-check rollup into the deskkit shape so the shared
// snapshot's CI verdict comes from the ONE reducer (#268) rather than a local re-walk.
func toCIChecks(rollup []check) []deskkit.CICheck {
	out := make([]deskkit.CICheck, 0, len(rollup))
	for _, c := range rollup {
		// Name/Context and the recency stamps are carried through so ReduceCIVerdict can
		// run the latest-run-per-name reduction. Dropping them here (the prior shape lifted
		// only Status/Conclusion) is what left this path structurally unable to ignore a
		// superseded run — the deskkit half of the #282/#289 board/flip divergence.
		out = append(out, deskkit.CICheck{
			Name:        c.Name,
			Context:     c.Context,
			Status:      c.Status,
			Conclusion:  c.Conclusion,
			StartedAt:   c.StartedAt,
			CompletedAt: c.CompletedAt,
			CreatedAt:   c.CreatedAt,
		})
	}
	return out
}

// ---- small helpers ----

func toAppReviews(reviews []review) []deskkit.AppReview {
	out := make([]deskkit.AppReview, 0, len(reviews))
	for _, r := range reviews {
		out = append(out, deskkit.AppReview{
			AuthorLogin: r.User.Login,
			State:       r.State,
			CommitID:    r.CommitID,
			SubmittedAt: r.SubmittedAt,
		})
	}
	return out
}

func sortUnassessable(rows []unassessableRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Repo != rows[j].Repo {
			return rows[i].Repo < rows[j].Repo
		}
		return rows[i].Number > rows[j].Number
	})
}

// daysRounded reports d as a number of days, with the duration snapped to the nearest
// 0.1 hour first so two rows measured a few seconds apart do not differ in the tail
// digits. The table renders it at 1 decimal place; the JSON keeps the finer value,
// because a consumer sorting a dispatch list wants the order, not the rounding.
func daysRounded(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(d.Round(time.Hour/10)) / float64(24*time.Hour)
}

// errShort reduces an error to a short reason string for an unassessable row.
func errShort(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Keep the row readable; the full cause rides in the audit/exit path if it bubbles up.
	if i := strings.Index(msg, ":"); i > 0 && i < 60 {
		return msg[:i]
	}
	if len(msg) > 60 {
		return msg[:60]
	}
	return msg
}
