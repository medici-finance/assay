package main

// Model-path verified → done auto-flip (methodology-metrics/39).
//
// WHAT THIS REMOVES FROM A HUMAN'S DAY, AND WHAT IT DOES NOT.
// A `gate: model` brief that reaches `verified` still needs its Reviewed cell
// stamped before it can read `done`. That stamp was being transcribed by hand
// from a review that already exists on GitHub — the reviewer App's APPROVED
// review on the PR that merged the work. Transcription is not judgement, and
// the lag was measured in days. This file derives the stamp instead.
//
// It does NOT weaken any gate:
//
//   - `gate: human` briefs are not candidates. They are filtered out before any
//     review is fetched, so the human path cannot be reached from here at all.
//     (verify-gate-close.yml remains the sole writer of a human:<name> stamp.)
//   - The evidence is a REVIEW OBJECT, not text. Not a comment, not a body
//     string, not a marker someone can type into a brief. A forged text marker
//     is precisely how a brief was once closed on a review that never happened.
//   - The approval must REPORT itself at the PR's MERGED HEAD. A review whose
//     `commit_id` names an earlier commit does not flip anything. State the
//     strength of that correctly: `commit_id` is not an established staleness
//     signal (rule 40 in docs/brief-rules.md — one observed disagreement with
//     the head named in a review body, direction and frequency unmeasured), so
//     this comparison is a best-effort filter that is trustworthy in the
//     REFUSING direction and proves nothing in the passing one. A matching SHA
//     does not establish that the approval is current; it only fails to
//     contradict it.
//   - Every failure is a NON-FLIP. A refusal (no App approval / a stale one)
//     and a could-not-check (no reviewer App configured, no merged PR resolves,
//     gh unreadable) both leave the brief at `verified` and both are reported.
//     "Could not confirm" is never recorded as "confirmed".
//   - There is no override. No flag, no environment variable, no fixture path
//     skips the corroboration — TestAutoFlipNoOverride pins it.
//
// What a human still signs off after this lands is unchanged: every
// `gate: human` brief, every irreversible brief, and the reviewer App's review
// itself — which is a review of the CODE, made before the merge, by a
// participant this brief does not touch. This step only transcribes a verdict
// that was already rendered.
//
// WHAT THE FLIP DEPENDS ON, AND WHAT IT DOES NOT RE-CHECK.
// The precondition is exactly two facts: the README row currently reads
// `verified`, and the merge PR carries an APPROVED review OBJECT from the
// roster's `reviewer=` App reporting itself at that PR's merged head. Nothing
// else is consulted.
//
// In particular the flip does NOT re-check the `verified` stamp it promotes.
// It does not ask who ran the Verify table, whether that identity differs from
// the implementer, or whether the Evidence names a real run. That matters
// concretely rather than theoretically: a corpus sweep on 2026-08-13 measured
// 141 README rows at `verified`/`done` and flagged 92 of them (65%) as resting
// on text the implementing identity wrote about itself, with 1 more of
// unresolved identity — the standing `F-verify-self-attest` finding.
//
// So state the trade rather than implying it away: this flip WILL promote an
// unbacked `verified` to `done` whenever the merge PR carries the App
// approval. What it adds to such a row is real and tamper-evident — an App
// review OBJECT of the code, which no free-text stamp provides — but it is a
// review of the CODE, not of the verification. `done` after this change means
// "someone stamped verified, AND the reviewer App approved the merge PR", not
// "the verification was independent". Closing the self-attestation gap is
// F-verify-self-attest's job and is not attempted here.
//
// ROW CLASSES THIS FLIP DOES NOT COVER (no silent caps — each stays put and is
// reported, none is silently skipped):
//
//   - `gate: human` briefs — filtered before any fetch, by design.
//   - briefs with no `gate:` at all (legacy, no brief-v1 frontmatter) — not
//     `model`, so not candidates.
//   - rows at any status other than exactly `verified` — `implemented`,
//     `in-progress`, `todo` and already-`done` rows are never touched.
//   - briefs whose stream resolves no owning repo (`repo:` frontmatter or
//     ASSAY_HOME_REPO) — COULD-NOT-CHECK.
//   - briefs whose last commitScanDepth commits resolve no merged PR — the
//     desk commits Evidence straight to main, so a brief closed that way may
//     have no merge PR within the window at all. COULD-NOT-CHECK.
//   - PRs whose reviews cannot be read, or that report no head commit —
//     COULD-NOT-CHECK.
//   - approvals whose `commit_id` does not match the merged head — REFUSED,
//     with the caveat about that signal's strength recorded above.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ---- the seam ----------------------------------------------------------------------

// prReviewState is one pull request reduced to the facts the flip judges: was
// it merged, what commit was its head when it merged, and what reviews does it
// carry (each with the commit it was submitted against).
type prReviewState struct {
	Merged  bool
	HeadSHA string
	Reviews []ghReview
	// GitLab-only (forge-neutral/02's corroboration rule). On GitLab CE approvals
	// do NOT reset on push, so the approval flag alone is not proof of an at-head
	// verdict — the desk writes the head SHA into a verdict NOTE, and that pinned
	// SHA read back is what carries the at-head property (identity.md §corroboration;
	// pilot steps A9/B4). Approvals lists the logins that approved the MR; Notes
	// carries the MR discussion notes scanned for a reviewer-authored head-pin. Both
	// are empty on the GitHub path, where CommitOID on a review carries the head tie.
	Approvals []string
	Notes     []glNote
}

// glNote is one GitLab merge-request note reduced to what corroboration reads: who
// authored it and its body (scanned for the pinned head SHA).
type glNote struct {
	Author string
	Body   string
}

// modelFlipSource supplies those facts. The live implementation shells out to
// git and gh (ghModelFlipSource below); tests substitute a fake, so no test
// ever touches the network or a real brief.
type modelFlipSource interface {
	// CommitsTouching returns commit SHAs that touched relPath, newest first,
	// at most limit of them.
	CommitsTouching(root, relPath string, limit int) ([]string, error)
	// MergedPRForCommit returns the merged pull request that sha BELONGS TO —
	// i.e. whose merge commit or head commit IS sha, OR that genuinely
	// INTRODUCED sha as one of its own branch commits (the merge-committed
	// intermediate-commit case; see mergedPRForCommit). ok is false when none is.
	MergedPRForCommit(repo, sha string) (int, bool, error)
	// ReviewState returns the PR's merge state, merged head and reviews.
	ReviewState(repo string, pr int) (prReviewState, error)
}

// ---- outcomes ----------------------------------------------------------------------

type flipOutcome int

const (
	// flipUnchecked — the corroboration could not be MADE (no reviewer App in
	// the roster, no merged PR resolves, gh/git unreadable). Not a flip, and
	// deliberately distinct from flipRefused: an unanswered question must never
	// be filed as an answered one.
	flipUnchecked flipOutcome = iota
	// flipRefused — the corroboration was made and FAILED. Not a flip.
	flipRefused
	// flipDone — an App APPROVED review exists at the merged head. Flipped.
	flipDone
)

func (o flipOutcome) String() string {
	switch o {
	case flipDone:
		return "FLIPPED"
	case flipRefused:
		return "REFUSED"
	default:
		return "COULD-NOT-CHECK"
	}
}

// modelFlipResult is the per-candidate record. One line of the run's report.
type modelFlipResult struct {
	Brief   string
	Outcome flipOutcome
	PR      int
	SHA     string
	Stamp   string
	Reason  string
	// Misconfig marks a COULD-NOT-CHECK that a human can FIX by correcting the
	// setup (no `reviewer=` App bound / no owning repo resolves), as opposed to a
	// structurally-unresolvable one (no merge PR in the window, a transient read).
	// Only the former keeps the run fatal — see reportAutoFlipModel (Fix B).
	Misconfig bool
}

// ---- the stamp ---------------------------------------------------------------------

// modelReviewedStamp renders the Reviewed cell. Date first (the repo-wide
// Reviewed-cell convention, and what the `done`-shape lint requires), then the
// reviewing App, then the PR number and the FULL head SHA the approval was
// submitted against — so any auto-flip is auditable back to the exact review
// object, not merely to "an approval on that PR". The SHA is not abbreviated:
// this cell is the audit record, and a short SHA is a lookup, not a fact.
func modelReviewedStamp(now time.Time, reviewer string, pr int, sha string) string {
	return fmt.Sprintf("%s %s (approved PR #%d @ %s)", now.Format("2006-01-02"), reviewer, pr, sha)
}

// reviewerIdentity is the accepted reviewer, resolved forge-aware from the trust
// roster's `reviewer=` role binding (forge-neutral/02, /07). Logins is the set of
// login renderings the reviewer is recognised under ON ITS FORGE — the GitHub
// `<slug>[bot]` / `app/<slug>` pair, or the bare GitLab service-account username —
// derived from the forge-qualified roster entry rather than by appending a literal
// `[bot]` regardless of forge. That literal was the #349 defect: on GitLab no
// `<slug>[bot]` account exists, so the flip could never match and silently never
// fired. Forge selects which corroboration rule decideModelFlip applies.
type reviewerIdentity struct {
	Logins  []string
	Forge   forgeKind
	Display string // the primary accepted login — the Reviewed stamp and report header
	// Unresolved, when non-empty, is a could-not-check reason: the reviewer role is
	// bound but to a forge this build does not understand, so no approval can be
	// matched. It is a MISCONFIGURATION (a human fixes the roster), distinct from
	// "no reviewer bound at all" (Logins empty AND Unresolved "").
	Unresolved string
}

// configured reports whether an accepted reviewer identity was resolved. A false
// return with Unresolved set is the unrecognised-forge could-not-check; with
// Unresolved empty it is "no reviewer= App bound".
func (r reviewerIdentity) configured() bool { return len(r.Logins) > 0 }

// modelReviewer resolves the accepted reviewer identity from the effective roster,
// forge-aware. It mirrors evidenceactor.go's verifier resolution: the reviewer's
// forge comes from ITS roster entry (BotIdents), the accepted login renderings from
// that entry's acceptedLogins(). A legacy roster with no forge-qualified ident for
// the slug defaults to a GitHub entry (the backward-compatibility rule). A reviewer
// bound to a forge this build does not understand is could-not-check naming the
// forge, never a pass (forge-neutral/07) — nothing flips.
func modelReviewer() reviewerIdentity {
	cfg := scanEffectiveConfig()
	slug := strings.TrimSpace(cfg.RoleBots["reviewer"])
	if slug == "" {
		return reviewerIdentity{}
	}
	// Default to a GitHub entry when the roster carries no forge-qualified ident for
	// the slug (a legacy roster), mirroring evidenceActorPolicyFromRoster.
	ident := scanBotIdentity{Forge: forgeGitHub, Slug: strings.ToLower(slug)}
	if got, ok := cfg.BotIdents[strings.ToLower(slug)]; ok {
		ident = got
	}
	if ident.Forge == forgeUnknown {
		return reviewerIdentity{Unresolved: fmt.Sprintf(
			"the reviewer role is bound to slug %q on forge %q, which this build does not understand "+
				"(it recognises github and gitlab) — no approval can be matched to the reviewer, so no "+
				"gate:model row is flipped by this run; an unrecognised forge is could-not-check, never a pass",
			slug, ident.ForgeRaw)}
	}
	logins := ident.acceptedLogins()
	if len(logins) == 0 {
		return reviewerIdentity{} // defensive: no trustable rendering on this forge
	}
	return reviewerIdentity{Logins: logins, Forge: ident.Forge, Display: logins[0]}
}

// loginInSet reports whether login (case-insensitively) is one of the accepted
// reviewer renderings.
func loginInSet(accepted []string, login string) bool {
	for _, l := range accepted {
		if strings.EqualFold(l, login) {
			return true
		}
	}
	return false
}

// noteBodyPinsSHA reports whether a GitLab note body pins headSHA — the FULL head
// SHA appears in the body (case-insensitive). It is the GitLab at-head signal: the
// desk writes the head SHA into the verdict note, and a note pinning an EARLIER SHA
// (a stale verdict) does not contain the current head and so does not corroborate.
// The full SHA is required, never a prefix — the same strength as approvedReviewAt's
// GitHub commit_id compare.
func noteBodyPinsSHA(body, headSHA string) bool {
	if headSHA == "" {
		return false
	}
	return strings.Contains(strings.ToLower(body), strings.ToLower(headSHA))
}

// commitScanDepth is how far back the brief file's history is walked looking
// for a commit that belongs to a merged PR. The desk commits Evidence straight
// to main, so the newest commits touching a brief usually have no PR at all;
// this walks past them to the merge that landed the work.
const commitScanDepth = 25

// ---- the decision ------------------------------------------------------------------

// decideModelFlip judges ONE candidate brief. It never writes.
//
// Order matters: the reviewer identity and the repo are resolved BEFORE any
// fetch, so an unconfigured roster costs nothing and reaches nothing.
func decideModelFlip(root string, s *Stream, path string, briefID string, src modelFlipSource, rev reviewerIdentity) modelFlipResult {
	res := modelFlipResult{Brief: briefID, Outcome: flipUnchecked}

	if !rev.configured() {
		res.Misconfig = true
		if rev.Unresolved != "" {
			// The reviewer role IS bound, but to a forge this build cannot match —
			// could-not-check naming the forge, a fixable misconfiguration.
			res.Reason = rev.Unresolved
			return res
		}
		res.Reason = "no `reviewer=` App bound in ASSAY_TRUSTED_BOT_SLUGS — there is no identity whose approval could be corroborated"
		return res
	}
	repo := s.Repo
	if repo == "" {
		repo = scanEffectiveConfig().HomeRepo
	}
	if repo == "" {
		res.Misconfig = true
		res.Reason = "no owning repo for this stream (`repo:` frontmatter / ASSAY_HOME_REPO) — no PR to read reviews from"
		return res
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	commits, err := src.CommitsTouching(root, filepath.ToSlash(rel), commitScanDepth)
	if err != nil {
		res.Reason = fmt.Sprintf("could not read the brief file's history: %v", err)
		return res
	}

	pr := 0
	for _, sha := range commits {
		n, ok, err := src.MergedPRForCommit(repo, sha)
		if err != nil {
			res.Reason = fmt.Sprintf("could not resolve commit %s to a pull request: %v", sha, err)
			return res
		}
		if ok {
			pr = n
			break
		}
	}
	if pr == 0 {
		res.Reason = fmt.Sprintf("no merged pull request resolves from the last %d commits touching %s", commitScanDepth, filepath.ToSlash(rel))
		return res
	}
	res.PR = pr

	st, err := src.ReviewState(repo, pr)
	if err != nil {
		res.Reason = fmt.Sprintf("could not read the reviews of PR #%d: %v", pr, err)
		return res
	}
	if !st.Merged {
		res.Reason = fmt.Sprintf("PR #%d is not merged", pr)
		return res
	}
	if st.HeadSHA == "" {
		res.Reason = fmt.Sprintf("PR #%d reports no head commit — nothing to corroborate an approval against", pr)
		return res
	}
	res.SHA = st.HeadSHA

	// From here the check WAS made, so every failure is a REFUSAL, not a
	// could-not-check. The corroboration rule is per forge (forge-neutral/02's
	// identity.md §corroboration): on GitHub the at-head tie is a review's own
	// commit_id; on GitLab CE — where approvals persist across a push — it is a
	// note by the reviewer pinning the head SHA, the approval flag alone being
	// insufficient.
	if rev.Forge == forgeGitLab {
		return decideGitLabFlip(res, pr, st, rev)
	}
	return decideGitHubFlip(res, pr, st, rev)
}

// decideGitHubFlip applies the GitHub corroboration rule: an APPROVED review by an
// accepted reviewer login whose commit_id equals the merged head. A stale approval
// (against an earlier commit) is REFUSED and names the signed commit.
func decideGitHubFlip(res modelFlipResult, pr int, st prReviewState, rev reviewerIdentity) modelFlipResult {
	for _, login := range rev.Logins {
		if _, ok := approvedReviewAt(st.Reviews, login, st.HeadSHA); ok {
			res.Outcome = flipDone
			return res
		}
	}
	// Distinguish "never approved" from "approved something else" — the second is
	// the stale-approval case and the operator needs to see which commit was signed.
	for _, login := range rev.Logins {
		if r, any := approvedReviewAt(st.Reviews, login, ""); any {
			res.Outcome = flipRefused
			res.Reason = fmt.Sprintf("PR #%d merged at %s but the %s approval is against %s — a stale approval does not close a brief",
				pr, st.HeadSHA, rev.Display, r.CommitOID)
			return res
		}
	}
	res.Outcome = flipRefused
	res.Reason = fmt.Sprintf("PR #%d carries no APPROVED review from %s", pr, rev.Display)
	return res
}

// decideGitLabFlip applies the GitLab CE corroboration rule: an approval by an
// accepted reviewer identity PLUS a note by that identity whose body pins the head
// SHA. The approval flag alone does NOT close the brief — on CE it persists across
// a push, so a verdict recorded against an older head would read as current; the
// note's pinned SHA is what ties the verdict to the head (identity.md; pilot A9/B4).
func decideGitLabFlip(res modelFlipResult, pr int, st prReviewState, rev reviewerIdentity) modelFlipResult {
	approved := false
	for _, a := range st.Approvals {
		if loginInSet(rev.Logins, a) {
			approved = true
			break
		}
	}
	if !approved {
		res.Outcome = flipRefused
		res.Reason = fmt.Sprintf("MR #%d carries no approval from %s", pr, rev.Display)
		return res
	}
	for _, n := range st.Notes {
		if loginInSet(rev.Logins, n.Author) && noteBodyPinsSHA(n.Body, st.HeadSHA) {
			res.Outcome = flipDone
			return res
		}
	}
	res.Outcome = flipRefused
	res.Reason = fmt.Sprintf("MR #%d is approved by %s but no note by that identity pins the head %s — on GitLab CE an approval persists across a push, so the at-head verdict lives in the note's pinned SHA, not the approval flag",
		pr, rev.Display, st.HeadSHA)
	return res
}

// ---- the run -----------------------------------------------------------------------

// autoFlipModel walks every stream, judges every `gate: model` brief whose
// README row reads exactly `verified`, and — for the ones a live App approval
// at the merged head corroborates — stamps the Reviewed cell and flips the row
// to `done` in the same write. Returns one result per CANDIDATE, so a refusal
// and a could-not-check are both reported rather than being an absence.
//
// dryRun makes the identical decisions and writes nothing.
//
// It never touches STATUS.md (single-writer rule).
func autoFlipModel(root string, streams []*Stream, src modelFlipSource, rev reviewerIdentity, now time.Time, dryRun bool) ([]modelFlipResult, error) {
	var results []modelFlipResult

	for _, s := range streams {
		// Per-stream, so one README is read once and written once even when
		// several of its rows flip in the same run.
		readmePath := filepath.Join(s.Dir, "README.md")
		raw := ""
		changed := false

		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed (reported elsewhere) or legacy/opted-out
			}
			// THE HUMAN-GATE FILTER. Everything below this line is unreachable
			// for a gate:human brief — no PR is resolved, no review is fetched,
			// no result is even recorded. A brief with no gate: at all (legacy)
			// is not `model` either and is excluded by the same test.
			if bf.Gate != "model" {
				continue
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			row := findRow(s, num)
			if row == nil || row.Status != "verified" {
				continue
			}

			res := decideModelFlip(root, s, path, bf.Brief, src, rev)
			if res.Outcome != flipDone {
				results = append(results, res)
				continue
			}

			res.Stamp = modelReviewedStamp(now, rev.Display, res.PR, res.SHA)
			if dryRun {
				results = append(results, res)
				continue
			}
			if raw == "" {
				b, err := os.ReadFile(readmePath)
				if err != nil {
					return results, err
				}
				raw = string(b)
			}
			updated, err := flipRowToDone(raw, num, res.Stamp, "")
			if err != nil {
				// The decision stood but the row could not be rewritten. That
				// is a could-not-check on the WRITE, not a silent success.
				res.Outcome = flipUnchecked
				res.Reason = fmt.Sprintf("%s: %v", readmePath, err)
				results = append(results, res)
				continue
			}
			raw = updated
			changed = true
			results = append(results, res)
		}

		if changed && !dryRun {
			if err := os.WriteFile(readmePath, []byte(raw), 0o644); err != nil {
				return results, err
			}
		}
	}
	return results, nil
}

// ---- the live source ---------------------------------------------------------------

// ghModelFlipSource is the production seam: git for history, gh for GitHub.
// Read-only in both directions — it lists commits and reads pull requests, and
// creates, comments on and mutates nothing.
type ghModelFlipSource struct{}

func (ghModelFlipSource) CommitsTouching(root, relPath string, limit int) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "log",
		fmt.Sprintf("-%d", limit), "--format=%H", "--", relPath).Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", relPath, err)
	}
	var shas []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			shas = append(shas, l)
		}
	}
	return shas, nil
}

// ghCommitPR is the subset of the commit→pulls association the resolver reads.
type ghCommitPR struct {
	Number         int    `json:"number"`
	MergedAt       string `json:"merged_at"`
	MergeCommitSHA string `json:"merge_commit_sha"`
	Head           struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func (g ghModelFlipSource) MergedPRForCommit(repo, sha string) (int, bool, error) {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/commits/%s/pulls", repo, sha)).Output()
	if err != nil {
		return 0, false, fmt.Errorf("gh api commits/%s/pulls: %w", sha, err)
	}
	var prs []ghCommitPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return 0, false, fmt.Errorf("unmarshal commits/%s/pulls: %w", sha, err)
	}
	// The introduced-commits oracle is passed as a lazy callback so it is only
	// paid for the slow path — a PR resolved by the head/merge fast path makes
	// no extra call.
	return mergedPRForCommit(sha, prs, func(n int) ([]string, error) {
		return g.prBranchCommits(repo, n)
	})
}

// mergedPRForCommit resolves sha to the merged PR that OWNS it, given the
// commit→pulls association (prs) and an oracle listing the commits a PR actually
// INTRODUCED (prCommits — its own branch commits, base-excluded). It is split
// out from the gh transport above so this — the integrity-load-bearing decision
// — is unit-tested directly, with no network.
//
// A commit legitimately belongs to a PR in two ways; a third case is REFUSED:
//
//   - FAST PATH — sha IS the PR's head or merge commit. Unchanged; no extra call.
//   - INTERMEDIATE-COMMIT PATH — the PR merged as a REAL merge commit (branch
//     preserved, not squashed) and sha is one of the NON-head commits on its
//     branch. That commit was genuinely introduced by this PR, so the App's
//     approval on the PR is an approval of that commit's work. Before this path
//     such a commit matched no PR (it is neither the head nor the merge commit),
//     the brief resolved no merged PR, and the run came back COULD-NOT-CHECK on
//     every push — the persistent main-red across 8 briefs this repairs.
//   - REFUSED (integrity intent preserved) — a PR that merely CONTAINS an
//     already-landed commit on its branch. The association endpoint returns such
//     PRs too, but an already-landed commit is an ANCESTOR of their base and so
//     is ABSENT from prCommits, so it is never credited. Crediting it would let
//     an unrelated PR's review close a brief — exactly what the old strict guard
//     was protecting against, and still is.
//
// Membership in the introduced-commits list is the clone-free equivalent of
// "reachable from merge_commit_sha but NOT from base.sha": both name precisely
// the set of commits the PR merged, and nothing else.
func mergedPRForCommit(sha string, prs []ghCommitPR, prCommits func(n int) ([]string, error)) (int, bool, error) {
	// Fast path across ALL associated PRs first, so the cheap exact match wins
	// before any introduced-commits call is made.
	for _, p := range prs {
		if p.MergedAt == "" {
			continue
		}
		if strings.EqualFold(p.MergeCommitSHA, sha) || strings.EqualFold(p.Head.SHA, sha) {
			return p.Number, true, nil
		}
	}
	// Intermediate-commit path: credit a PR only if sha is one of the commits IT
	// introduced (base-excluded), never merely one its branch contains.
	for _, p := range prs {
		if p.MergedAt == "" {
			continue
		}
		commits, err := prCommits(p.Number)
		if err != nil {
			return 0, false, err
		}
		for _, c := range commits {
			if strings.EqualFold(c, sha) {
				return p.Number, true, nil
			}
		}
	}
	return 0, false, nil
}

// prBranchCommits lists the commits PR n INTRODUCED — the commits on its branch
// ahead of base, which is exactly what GET pulls/{n}/commits returns. A commit an
// earlier PR already landed is an ancestor of this PR's base and so is absent
// from this list; that is what lets mergedPRForCommit credit a genuine
// intermediate commit while still refusing a PR that merely CONTAINS one. (The
// endpoint caps at 250 commits per PR — far beyond any brief-touching branch.)
func (ghModelFlipSource) prBranchCommits(repo string, n int) ([]string, error) {
	out, err := exec.Command("gh", "api", "--paginate",
		fmt.Sprintf("repos/%s/pulls/%d/commits", repo, n)).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api pulls/%d/commits: %w", n, err)
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(out, &commits); err != nil {
		return nil, fmt.Errorf("unmarshal pulls/%d/commits: %w", n, err)
	}
	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.SHA)
	}
	return shas, nil
}

// ghRESTReview is the REST reviews-endpoint shape. It is used instead of
// `gh pr view --json reviews` for one reason: only this endpoint returns
// `commit_id`, the sole per-review commit reference either transport offers.
// It is the best available filter, not a sound staleness signal — see the
// CommitOID note in corroborate.go and rule 40 in docs/brief-rules.md.
type ghRESTReview struct {
	User struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
	State    string `json:"state"`
	CommitID string `json:"commit_id"`
	HTMLURL  string `json:"html_url"`
	Body     string `json:"body"`
}

type ghPRHead struct {
	HeadRefOID string `json:"headRefOid"`
	State      string `json:"state"`
	MergedAt   string `json:"mergedAt"`
}

func (ghModelFlipSource) ReviewState(repo string, pr int) (prReviewState, error) {
	head, err := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", pr),
		"--repo", repo, "--json", "headRefOid,state,mergedAt").Output()
	if err != nil {
		return prReviewState{}, fmt.Errorf("gh pr view %d: %w", pr, err)
	}
	var h ghPRHead
	if err := json.Unmarshal(head, &h); err != nil {
		return prReviewState{}, fmt.Errorf("unmarshal PR %d head: %w", pr, err)
	}

	out, err := exec.Command("gh", "api", "--paginate",
		fmt.Sprintf("repos/%s/pulls/%d/reviews", repo, pr)).Output()
	if err != nil {
		return prReviewState{}, fmt.Errorf("gh api pulls/%d/reviews: %w", pr, err)
	}
	var rest []ghRESTReview
	if err := json.Unmarshal(out, &rest); err != nil {
		return prReviewState{}, fmt.Errorf("unmarshal PR %d reviews: %w", pr, err)
	}
	st := prReviewState{
		Merged:  h.State == "MERGED" && h.MergedAt != "",
		HeadSHA: h.HeadRefOID,
	}
	for _, r := range rest {
		st.Reviews = append(st.Reviews, ghReview{
			Author:    ghAuthor{Login: r.User.Login},
			Body:      r.Body,
			State:     r.State,
			CommitOID: r.CommitID,
		})
	}
	return st, nil
}

// ---- entry point -------------------------------------------------------------------

// runAutoFlipModel is the `statusgen --auto-flip-model` entrypoint. It is a
// self-contained sub-command: it rewrites stream README rows and never reads or
// writes STATUS.md. It gathers the decisions and hands the exit policy to
// reportAutoFlipModel (see there for the exit codes).
func runAutoFlipModel(root string, dryRun bool) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	rev := modelReviewer()
	results, err := autoFlipModel(root, streams, liveModelFlipSource(rev.Forge), rev, time.Now(), dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	return reportAutoFlipModel(os.Stdout, os.Stderr, results, rev.Display, dryRun)
}

// liveModelFlipSource selects the production read seam for the reviewer's forge.
// GitHub reads through `gh` (ghModelFlipSource). A GitLab reviewer selects
// gitlabModelFlipUnavailable: statusgen has no GitLab merge-request read path yet —
// the forge-aware LIVE read lands with the conformance round trip
// (forge-neutral/10), where a real GitLab deployment exists — so the GitLab live
// read reports an honest could-not-check rather than shelling `gh`, which cannot
// read a GitLab MR. The corroboration DECISION for GitLab is implemented and
// unit-tested (decideGitLabFlip); only the live reader that would feed it real MR
// state is deferred. forgeUnknown keeps GitHub (a legacy roster with no
// forge-qualified reviewer ident resolves to GitHub, per modelReviewer).
func liveModelFlipSource(forge forgeKind) modelFlipSource {
	if forge == forgeGitLab {
		return gitlabModelFlipUnavailable{}
	}
	return ghModelFlipSource{}
}

// errGitLabFlipReadUnavailable is the honest could-not-check a GitLab auto-flip
// reports until the forge-aware live merge-request read lands with the conformance
// round trip (forge-neutral/10). It is structural, not a misconfiguration: no
// operator setting resolves it, so it is a non-fatal NOTICE (reportAutoFlipModel),
// never a fatal error and never a silent flip.
var errGitLabFlipReadUnavailable = errors.New(
	"statusgen has no GitLab merge-request read path yet — the forge-aware live read lands with the conformance round trip (forge-neutral/10); a GitLab auto-flip is could-not-check until then, never a flip")

// gitlabModelFlipUnavailable is the live source for a GitLab reviewer. Commit
// history is plain git and works on any forge, so CommitsTouching is served; the
// merge-request reads report errGitLabFlipReadUnavailable, which decideModelFlip
// surfaces as a structural could-not-check.
type gitlabModelFlipUnavailable struct{}

func (gitlabModelFlipUnavailable) CommitsTouching(root, relPath string, limit int) ([]string, error) {
	return ghModelFlipSource{}.CommitsTouching(root, relPath, limit)
}

func (gitlabModelFlipUnavailable) MergedPRForCommit(repo, sha string) (int, bool, error) {
	return 0, false, errGitLabFlipReadUnavailable
}

func (gitlabModelFlipUnavailable) ReviewState(repo string, pr int) (prReviewState, error) {
	return prReviewState{}, errGitLabFlipReadUnavailable
}

// reportAutoFlipModel prints the per-candidate report and returns the process
// exit code. It is split from runAutoFlipModel so the exit POLICY — which is
// integrity-relevant — is unit-tested without a network, a roster, or real
// streams. It writes nothing to any brief; a brief stays `verified` in every
// non-flip branch regardless of the code returned.
//
// EXIT POLICY (Fix B — defense in depth). A COULD-NOT-CHECK is split into two
// kinds, because they are not the same question and only one is an operator's to
// fix:
//
//   - MISCONFIGURATION (r.Misconfig): no `reviewer=` App is bound, or a stream
//     resolves no owning repo. These are a broken SETUP — the corroboration
//     cannot be attempted at all, and a human fixes the roster / `repo:`
//     frontmatter to unblock it. This stays FATAL (exit 1): it is the operator-
//     fixable case the original exit-1 was designed to surface, and hiding it
//     would leave the whole flip silently dead.
//
//   - STRUCTURALLY UNRESOLVABLE (every other could-not-check): no merged PR
//     resolves from the brief's recent history (the desk commits Evidence
//     straight to main, so a brief legitimately may have no merge PR in the
//     window at all), a transient gh/git read, or a write that could not be
//     applied. Nothing an operator can "fix" makes these resolve, so reddening
//     `model-autoflip` on main on EVERY push forever is the wrong severity —
//     that was the standing main-red. These now emit a loud NOTICE and leave the
//     exit code at 0.
//
// This mirrors statusgen's OWN precedent for a structurally-unresolvable,
// pre-existing condition: evidenceactor.go (the 65%-drifted Evidence-actor
// backlog) and archivecheck.go (the retired-placeholder archive candidate) both
// ship such a condition as a non-fatal NOTICE with a named backlog rather than a
// reddening error, so an unrelated PR is never blocked by a standing condition
// it did not introduce. A NOTICE changes the EXIT code, never the flip decision.
func reportAutoFlipModel(w, errw io.Writer, results []modelFlipResult, reviewer string, dryRun bool) int {
	fmt.Fprintln(w, "# model-path verified→done auto-flip")
	fmt.Fprintln(w, "# Scope: gate:model briefs at `verified` whose merge PR carries an APPROVED")
	fmt.Fprintf(w, "# review from %q at the merged head. gate:human briefs are not candidates.\n",
		cmpOrNone(reviewer))
	if dryRun {
		fmt.Fprintln(w, "# --dry-run: decisions only, nothing written.")
	}
	misconfig, structural := 0, 0
	for _, r := range results {
		switch {
		case r.Outcome == flipDone:
			fmt.Fprintf(w, "%s FLIPPED verified→done — %s\n", r.Brief, r.Stamp)
		case r.Outcome == flipRefused:
			fmt.Fprintf(w, "%s REFUSED (stays verified) — %s\n", r.Brief, r.Reason)
		case r.Misconfig:
			misconfig++
			fmt.Fprintf(w, "%s COULD-NOT-CHECK (stays verified) — %s\n", r.Brief, r.Reason)
		default:
			structural++
			fmt.Fprintf(w, "%s NOTICE COULD-NOT-CHECK (stays verified) — %s\n", r.Brief, r.Reason)
		}
	}
	if len(results) == 0 {
		fmt.Fprintln(w, "(no gate:model briefs at `verified`)")
	}
	if structural > 0 {
		fmt.Fprintf(errw,
			"statusgen: NOTICE: %d candidate(s) are structurally unresolvable (no merged PR in the brief's window, or a transient read) — none was flipped, each stays `verified`; non-fatal, mirroring evidenceactor.go / archivecheck.go\n",
			structural)
	}
	if misconfig > 0 {
		fmt.Fprintf(errw,
			"statusgen: %d candidate(s) could not be checked because of a fixable MISCONFIGURATION (no `reviewer=` App bound / no owning repo) — fix the roster or `repo:` frontmatter; a brief stays `verified` until an App approval at its merged head can be read\n",
			misconfig)
		return 1
	}
	return 0
}

// cmpOrNone renders an unset reviewer identity readably in the scope header.
func cmpOrNone(s string) string {
	if s == "" {
		return "(no reviewer App configured — nothing can flip)"
	}
	return s
}
