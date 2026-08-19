package main

// evidenceactor.go — the EVIDENCE-ACTOR check (desk-apps/07).
//
// THE HOLE IT CLOSES. `F-verify-self-attest` (open since 2026-07-17):
// "Model-gated verified/done is fully self-attestable: 'independent verification'
// and verifier-tier are substring matches on free-text the verifying session
// writes about itself." Every existing control on a `verified` row reads text the
// verifying session authored — attribution.go's verifier-floor reads the Verified
// cell's runner token, evidenceHasIndependentRow reads the Evidence table's Runner
// column, and both are prose in a file the same session committed. A session that
// wants a green row writes `2026-07-20 opus-verifier` and an Evidence row whose
// Runner cell says something other than "implementer", and every one of those
// checks passes.
//
// THE MECHANISM. A `## Evidence` section is a set of LINES IN GIT. Who committed
// each of those lines is not free text — it is commit metadata, written by
// whatever identity pushed, not by the sentence being asserted. This check reads
// it: for every README row at `verified` or `done`, `git blame` the brief file's
// Evidence section and ask whether ANY of its current lines is owned by an
// ACCEPTED VERIFIER ACTOR. A section owned entirely by the implementing identity
// is the F-verify-self-attest shape, and now says so out loud.
//
// DERIVE-OR-DIFF. One declared source: the accepted actor is the roster's
// `verifier=<slug>:<id>` role binding in ASSAY_TRUSTED_BOT_SLUGS (rosterconfig.go),
// which topology.yaml names as "the identity whose Evidence commits count as
// PROVEN attribution". Nothing is hardcoded here and no new roster key is added —
// a second key would have to be recognised by BOTH readers of one roster.env, and
// an unknown ASSAY_ key collapses the other reader's whole configuration.
//
// ── WHICH SIGNAL IS TRUSTED, AND WHY (this is the load-bearing decision) ───────
//
// TRUSTED: the commit's AUTHOR EMAIL, in GitHub's noreply form
// `<numeric-id>+<login>@users.noreply.github.com`, matched on the NUMERIC ID
// against the roster binding. The id is GitHub's permanent handle for an account;
// a login is not (a deleted login can be re-registered by someone else, which is
// the argument rosterconfig.go already makes for requiring an id on
// ASSAY_BLESS_LOGIN). When the roster binding carries no id the match degrades to
// login-only, and the check SAYS SO in its own output rather than quietly
// accepting the weaker key.
//
// NOT TRUSTED: the author DISPLAY NAME. `git -c user.name='<verifier>[bot]' commit`
// sets it to anything. It is read here only to NAME a mismatch (see the impostor
// case below), never to accept one.
//
// NOT TRUSTED: the GPG signature, and this is measured rather than assumed. Every
// App commit in this repo — verifier and worker alike — carries `%G? = G` signed by
// ONE operator key (`%GS` resolves to the same single signer for both roles),
// because the signing happens on the operator's machine after the App identity has
// already been chosen. The signature therefore attests WHICH MACHINE produced the
// commit, not WHICH ACTOR authored it, so keying acceptance on it would accept a
// worker-authored Evidence section. The human-authored commits in the same range
// report `%G? = E` (key not in the verifying keyring), so a signature-keyed check
// would additionally reject the strongest actor in the fleet. Signature is a
// machine-provenance signal here, not an actor signal.
//
// NOT CONSULTED AT ALL: any GitHub API field, including a review's `commit_id`. A
// live case on 2026-08-13 had a review's `commit_id` reporting the current head
// while the same review's body said it had read an earlier commit — a check that
// pins "who verified what, at which head" to one API field inherits that
// disagreement. This check does not pin a head at all: it asks only "did an
// accepted actor commit these Evidence lines", which the local object database
// answers. It is also why --lint stays offline and network-free.
//
// WHAT THIS IS: TAMPER-EVIDENT, NOT TAMPER-PROOF. Whoever holds the working copy
// can `git -c user.email='<id>+<slug>[bot]@users.noreply.github.com' commit` and
// mint an accepted-looking author. The check does not claim otherwise, and the
// honest framing is verifyrun.go's, for the same reason: the value is that the
// forgery becomes a SPECIFIC, NAMED, DIFFABLE ACT recorded in commit metadata
// rather than an unremarkable sentence in a markdown table, and that the same
// commit is visible on GitHub's push events under the identity that actually
// pushed it. It raises the cost and the visibility of a false `verified`; it does
// not make one impossible. Closing that residual needs the desk-apps/08 server-side
// ruleset (who may push what to main), which has not landed.
//
// ── THREE-STATE (desk-hardening/01) ───────────────────────────────────────────
//
//	checked-clean      an accepted actor owns at least one current Evidence line
//	checked-failed     the section exists and NO accepted actor owns any of it
//	could-not-check    the accepted-actor set is unknown, or blame could not run
//
// could-not-check is load-bearing and is never rendered as either of the others.
// The two shapes that produce it:
//
//   - THE ROSTER IS UNCONFIGURED, or configures no `verifier=` role. There is then
//     no accepted actor to compare against, and "no accepted actor committed this"
//     would be trivially true for every row in the corpus — a checked-failed
//     verdict derived from the checker's own ignorance. One could-not-check NOTICE,
//     zero findings.
//
//   - GIT BLAME FAILED for a row (untracked file, no .git, a tree exported with
//     `git archive`). Reported as its own could-not-check line naming the count;
//     those rows are excluded from the clean AND the flagged tally, never folded
//     into either.
//
// A tamper-evidence check that fails open is worse than no check, because it
// launders an unverified row as checked. So every path that cannot establish the
// actor says so by name.
//
// ── SEVERITY: NOTICE, AND THE NUMBER THAT DECIDED IT ──────────────────────────
//
// MEASURED before choosing, against this repo at 1910eff7 on 2026-08-13:
// 141 README rows sit at `verified` or `done`. 92 of them (65%) have an Evidence
// section that no accepted actor owns — in almost every case the section is owned
// end-to-end by the WORKER App, i.e. the implementing identity wrote its own
// verification evidence, which is F-verify-self-attest observed rather than
// hypothesised. 48 are backed by the verifier App, all of them post-dating the
// deskevidence cutover (desk-apps/04). One (issue-flow/04) came back
// UNRESOLVED-IDENTITY: its Evidence commit carries the verifier LOGIN with the
// App's numeric App-id in the noreply address where the bot USER id belongs — the
// same mint defect that leaves `.author.login` null on GitHub, and 245 commits in
// this repo's history carry that malformed address. That row is why the notice for
// this class refuses to call it forgery: the misconfiguration and the forgery look
// identical from here, and only a human reading the commit can tell them apart.
//
// Arming a PROBLEM against a corpus that is 65% drifted would red every unrelated
// PR on day one and the fleet would learn to route around the gate. So this ships
// at NOTICE with a named reconciliation backlog, which is exactly the precedent
// mergedstatus.go set for the same situation. Promotion to PROBLEM is a later
// ruling, once the backlog is reconciled or explicitly grandfathered — and the
// grandfathering has to be a DATE CUTOFF recorded here, not a silent allowlist.
//
// ── WHAT THIS CHECK DOES NOT COVER (no silent caps) ───────────────────────────
//
//  1. IT PROVES WHO COMMITTED, NEVER WHAT WAS RUN. An accepted actor can commit an
//     Evidence section full of fiction. Execution is verifyrun.go's question; the
//     two are complements and neither subsumes the other.
//  2. IT IS DEAD IN CI TODAY. `--lint` under GitHub Actions loads the roster from
//     the environment, and .github/workflows/tools.yml sets no ASSAY_ variables, so
//     every CI run reports could-not-check. The check is live only where the roster
//     is — the desk's local pre-push lint. Supplying the roster to CI is a human
//     act (Actions variables) and is named in the hand-back, not worked around.
//  3. BLAME IS ABOUT CURRENT LINES. An Evidence section the verifier wrote and the
//     implementer later rewrote END TO END flips to flagged — correct. One the
//     implementer partially edited stays backed on the lines it did not touch —
//     deliberate: an editorial pass (a name scrub, a link fix) must not invalidate
//     a real verification. Measured cost of the alternative: attributing the
//     section to its LAST-TOUCHING COMMIT instead of blaming it makes a single
//     repo-wide scrub commit re-attribute 93 of the 133 rows that measurement covered to the
//     scrubber, which is
//     noise, not signal.
//  4. IT READS `verified`/`done` ROWS ONLY, and only where a `schema: brief-v1`
//     brief file exists with an exact `## Evidence` heading. Legacy briefs, rows
//     with no brief file, and the `Reviewed` column are all out of scope — the
//     review gate is a separate mechanism with its own actor.
//  5. UNCOMMITTED EVIDENCE IS NOT BACKED, on purpose. Blame runs against the
//     WORKING TREE, so lines not yet committed are owned by nobody and cannot
//     satisfy the check. A row cannot be greened by editing a file.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// noreplyEmailRe matches GitHub's noreply commit address,
// `<numeric-id>+<login>@users.noreply.github.com`. The numeric id is the whole
// point of parsing it — see the trusted-signal note in the file header.
var noreplyEmailRe = regexp.MustCompile(`^(\d+)\+([^@]+)@users\.noreply\.github\.com$`)

// githubIdentityFromEmail resolves a commit author email to the GitHub identity it
// pins. ok is false for any address that is not the noreply form: a plain address
// carries no GitHub identity at all, so it can never satisfy the check.
//
// The `[bot]` suffix is stripped from the login so a bot address compares against
// the roster's bare App slug, which is the form ASSAY_TRUSTED_BOT_SLUGS stores.
func githubIdentityFromEmail(email string) (login string, id int64, ok bool) {
	m := noreplyEmailRe.FindStringSubmatch(strings.TrimSpace(email))
	if m == nil {
		return "", 0, false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return strings.TrimSuffix(m[2], "[bot]"), n, true
}

// actorRef is one accepted identity: a login, and the permanent numeric id when
// the roster pinned one. ID == 0 means UNPINNED — see matches.
type actorRef struct {
	Login string
	ID    int64
}

// matches reports whether a commit identity is this actor.
//
// When the ref is id-pinned, the ID IS THE COMPARISON and the login is ignored:
// that is what makes a login re-registered by somebody else fail to match. Only an
// UNPINNED ref falls back to comparing logins, which is the weaker key the policy
// summary states in its output.
func (a actorRef) matches(login string, id int64) bool {
	if a.ID != 0 {
		return id == a.ID
	}
	return a.Login != "" && strings.EqualFold(a.Login, login)
}

// evidenceActorPolicy is the accepted-actor set, derived from the roster.
//
// Unavailable is the could-not-check state: non-empty means the policy could not
// be established and NO row may be judged against it.
type evidenceActorPolicy struct {
	Verifier    actorRef
	Humans      []actorRef
	Unavailable string
}

// idPinned reports whether the verifier binding carries a numeric id — i.e.
// whether acceptance is keyed on the permanent identity or only on the login.
func (p evidenceActorPolicy) idPinned() bool { return p.Verifier.ID != 0 }

// evidenceActorPolicyFromRoster derives the accepted-actor set from the effective
// roster configuration.
//
// It fails closed in three distinguishable ways, each with its own message,
// because "the gate did not run" and "the gate ran and found nothing" have to stay
// separable for whoever reads the NOTICE.
func evidenceActorPolicyFromRoster() evidenceActorPolicy {
	cfg := scanEffectiveConfig()

	if len(cfg.Problems) > 0 {
		return evidenceActorPolicy{Unavailable: fmt.Sprintf(
			"the trust roster (%s) is absent or invalid, so no accepted verifier identity could be "+
				"resolved (%d configuration problem(s), first: %s)",
			scanConfigHomePath(), len(cfg.Problems), cfg.Problems[0])}
	}

	slug := strings.TrimSpace(cfg.RoleBots["verifier"])
	if slug == "" {
		return evidenceActorPolicy{Unavailable: fmt.Sprintf(
			"no verifier role is bound: %s carries no `verifier=<slug>[:<id>]` entry in %s. Without a "+
				"declared verifier identity there is nothing to compare an Evidence commit against, and "+
				"reporting every row unbacked would be a verdict about this checker's own ignorance",
			scanEnvTrustedBotSlugs, cfg.Source)}
	}

	p := evidenceActorPolicy{Verifier: actorRef{Login: slug, ID: cfg.Bots[slug]}}
	for login, id := range cfg.Humans {
		p.Humans = append(p.Humans, actorRef{Login: login, ID: id})
	}
	sort.Slice(p.Humans, func(i, j int) bool { return p.Humans[i].Login < p.Humans[j].Login })
	return p
}

// actorVerdict is what the policy makes of one commit identity.
type actorVerdict int

const (
	// actorRejected — not an accepted identity. The ordinary flagged case.
	actorRejected actorVerdict = iota
	// actorVerifier — the bound verifier App.
	actorVerifier
	// actorHuman — a roster-known human.
	actorHuman
	// actorImpostor — the author claims the verifier identity but the pinned id
	// disagrees, or the display name claims it while the address pins nobody.
	// Rejected exactly like actorRejected; separated because it is a TAMPER
	// SIGNAL rather than a backlog row and is reported on its own line.
	actorImpostor
)

// classify judges one commit author (display name + email) against the policy.
// reason is a short human-readable clause naming what was decided and why.
func (p evidenceActorPolicy) classify(name, email string) (actorVerdict, string) {
	login, id, pinned := githubIdentityFromEmail(email)

	if !pinned {
		// No GitHub identity in the address. If the DISPLAY NAME nonetheless
		// claims the verifier, that is the local-spoof shape and is named.
		if p.claimsVerifierByName(name) {
			return actorImpostor, fmt.Sprintf(
				"author name %q claims the verifier identity but the commit address %q is not a GitHub "+
					"noreply address, so it pins no account — a display name is free text",
				name, email)
		}
		return actorRejected, fmt.Sprintf("address %q pins no GitHub account", email)
	}

	if p.Verifier.matches(login, id) {
		if p.idPinned() {
			return actorVerifier, fmt.Sprintf("committed by the bound verifier App (id %d)", id)
		}
		return actorVerifier, fmt.Sprintf(
			"committed by login %q, which matches the bound verifier slug — LOGIN-ONLY match, because "+
				"the %s entry for the verifier role carries no numeric id", login, scanEnvTrustedBotSlugs)
	}

	// Right login, wrong id: the pinned identity is what refuses this.
	if p.idPinned() && strings.EqualFold(login, p.Verifier.Login) {
		return actorImpostor, fmt.Sprintf(
			"address %q carries the verifier login but GitHub id %d, and the roster pins the verifier "+
				"role to id %d — a login that does not resolve to the pinned account is not that account",
			email, id, p.Verifier.ID)
	}

	for _, h := range p.Humans {
		if h.matches(login, id) {
			return actorHuman, fmt.Sprintf("committed by human:%s", h.Login)
		}
	}

	return actorRejected, fmt.Sprintf("committed by %s (id %d), which the roster does not accept as a verifier", login, id)
}

// claimsVerifierByName reports whether a display name reads as the verifier App.
// Used ONLY to name an impostor, never to accept one.
func (p evidenceActorPolicy) claimsVerifierByName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	slug := strings.ToLower(p.Verifier.Login)
	return slug != "" && (n == slug || n == slug+"[bot]" || n == "app/"+slug)
}

// ---------------------------------------------------------------------------
// Reading the Evidence section's line range and its blame
// ---------------------------------------------------------------------------

// evidenceLineRange locates the `## Evidence` section in a RAW brief file and
// returns its body's 1-based inclusive line range.
//
// The heading test is `strings.TrimSpace(line) == "## Evidence"`, character for
// character what extractEvidence uses, so the range this returns is the range of
// the text the rest of statusgen calls Evidence. A decorated heading
// (`## Evidence (notes)`) is not the section, here as there.
//
// ok is false when there is no heading or the section body is empty.
func evidenceLineRange(raw string) (start, end int, ok bool) {
	lines := strings.Split(raw, "\n")
	head := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			head = i
			break
		}
	}
	if head < 0 {
		return 0, 0, false
	}
	last := len(lines) - 1
	for i := head + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			last = i - 1
			break
		}
	}
	if last <= head {
		return 0, 0, false
	}
	return head + 2, last + 1, true // 1-based, body only (heading excluded)
}

// blameAuthor is one distinct (name, email) pair owning at least one blamed line.
type blameAuthor struct {
	Name  string
	Email string
}

// blamePorcelainAuthors parses `git blame --line-porcelain` output into the set of
// distinct authors owning the CONTENT-BEARING lines of the blamed range.
//
// CONTENT-BEARING IS THE WHOLE POINT, and it is a fail-open this check had before
// the positive control caught it: a blank line inside an Evidence section is
// structure, not evidence, and it survives an edit that replaces every real row.
// Attributing it made a section whose entire table had been rewritten by the
// implementer still read as verifier-backed, on the strength of one unchanged
// empty line. Boilerplate inside HTML comments is excluded for the same reason —
// the brief template ships the Evidence contract comment, so whoever created the
// file owns it, and owning a template line is not having verified anything.
//
// This is why the parse is --line-porcelain rather than --porcelain: the cheaper
// form emits author fields once per distinct commit with no way to tell which
// LINES they cover, so it cannot answer "who owns the non-blank ones".
func blamePorcelainAuthors(out string) []blameAuthor {
	var authors []blameAuthor
	seen := map[string]bool{}
	name, mail := "", ""
	inComment := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "author "):
			name = strings.TrimSpace(strings.TrimPrefix(line, "author "))
		case strings.HasPrefix(line, "author-mail "):
			mail = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "author-mail ")), "<>")
		case strings.HasPrefix(line, "\t"):
			// The blamed line's own content, verbatim after the leading TAB.
			content := strings.TrimSpace(line[1:])
			bearing := content != "" && !inComment
			// Track HTML-comment spans across lines. A line that OPENS a comment
			// carries no evidence past the marker, and one that closes it may.
			for rest := content; ; {
				if inComment {
					_, after, found := strings.Cut(rest, "-->")
					if !found {
						break
					}
					inComment = false
					rest = after
					if strings.TrimSpace(rest) != "" {
						bearing = true
					}
					continue
				}
				before, after, found := strings.Cut(rest, "<!--")
				if !found {
					break
				}
				if strings.TrimSpace(before) == "" {
					bearing = false
				}
				inComment = true
				rest = after
			}
			if !bearing {
				name, mail = "", ""
				continue
			}
			key := name + "\x00" + mail
			if !seen[key] {
				seen[key] = true
				authors = append(authors, blameAuthor{Name: name, Email: mail})
			}
			name, mail = "", ""
		}
	}
	return authors
}

// blameEvidenceAuthors blames one Evidence line range and returns its authors.
//
// It deliberately blames the WORKING TREE (no revision argument): the line range
// was computed from the file on disk, so blaming a revision instead would apply
// on-disk line numbers to a different version of the file, and uncommitted lines
// would be attributed to whoever last touched that region rather than to nobody.
// Uncommitted lines come back owned by git's `not.committed.yet` address, which
// pins no GitHub account and therefore cannot back a row.
func blameEvidenceAuthors(root, rel string, start, end int) ([]blameAuthor, error) {
	cmd := exec.Command("git", "-C", root, "blame", "--line-porcelain",
		"-L", fmt.Sprintf("%d,%d", start, end), "--", rel)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame -L %d,%d %s: %w", start, end, rel, err)
	}
	return blamePorcelainAuthors(string(out)), nil
}

// ---------------------------------------------------------------------------
// The check
// ---------------------------------------------------------------------------

// evidenceActorRow is one judged row.
type evidenceActorRow struct {
	ID       string // "<stream>/<NN>"
	Status   string // verified | done
	Verified string // the README Verified cell, for the message
	Verdict  actorVerdict
	Reason   string
	Err      error // non-nil => could-not-check for this row
}

// evidenceActorBlameLimit bounds how many blames run at once. `git blame` is
// process-per-file and the corpus is ~130 rows; serial it measures ~2.2s, which
// would roughly double --lint's wall time. Bounded concurrency keeps it well
// inside the noise while staying deterministic (results are indexed, then sorted).
const evidenceActorBlameLimit = 8

// evidenceActorJudge is the testable core: given the policy and the already-read
// rows, it produces one verdict each. Blame is injected so tests can drive it
// without a git repo — but the shipped positive control uses a REAL repo, because
// a check whose only proof of failure runs against a stub has not been shown to
// read git correctly.
func evidenceActorJudge(p evidenceActorPolicy, rows []evidenceActorRow,
	blame func(row int) ([]blameAuthor, error)) []evidenceActorRow {

	out := make([]evidenceActorRow, len(rows))
	copy(out, rows)

	var wg sync.WaitGroup
	sem := make(chan struct{}, evidenceActorBlameLimit)
	for i := range out {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			authors, err := blame(i)
			if err != nil {
				out[i].Err = err
				return
			}
			if len(authors) == 0 {
				out[i].Err = fmt.Errorf("the Evidence section has no content-bearing lines " +
					"(blank lines and HTML-comment boilerplate only), so there is no authored " +
					"evidence to attribute — checkBriefFiles reports that emptiness as a PROBLEM")
				return
			}
			// An ACCEPT anywhere in the section wins; otherwise the sharpest
			// rejection is reported, impostor outranking a plain reject.
			best, bestReason := actorRejected, ""
			for _, a := range authors {
				v, reason := p.classify(a.Name, a.Email)
				if v == actorVerifier || v == actorHuman {
					best, bestReason = v, reason
					break
				}
				if bestReason == "" || (v == actorImpostor && best != actorImpostor) {
					best, bestReason = v, reason
				}
			}
			out[i].Verdict, out[i].Reason = best, bestReason
		}(i)
	}
	wg.Wait()
	return out
}

// evidenceActorNotices is the --lint entry point. It returns NOTICE strings only —
// see the severity note in the file header for why this is not a PROBLEM.
func evidenceActorNotices(root string, streams []*Stream) []string {
	p := evidenceActorPolicyFromRoster()
	if p.Unavailable != "" {
		return []string{fmt.Sprintf(
			"could-not-check: Evidence-actor (desk-apps/07, F-verify-self-attest) did not run — %s. "+
				"No `verified`/`done` row is reported clean or unbacked by this run.", p.Unavailable)}
	}

	type pending struct {
		row        evidenceActorRow
		rel        string
		start, end int
	}
	var work []pending
	var skipped []string

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			var row *Brief
			for i := range s.Briefs {
				if s.Briefs[i].Num == num {
					row = &s.Briefs[i]
					break
				}
			}
			if row == nil || (row.Status != "verified" && row.Status != "done") {
				continue
			}
			id := s.Name + "/" + num

			raw, err := os.ReadFile(path)
			if err != nil {
				skipped = append(skipped, fmt.Sprintf("%s (unreadable: %v)", id, err))
				continue
			}
			// brief-v1 opt-in, the same schema gate every other brief-file check uses.
			if _, ok, perr := parseBriefFile(path); perr != nil || !ok {
				continue
			}
			start, end, ok := evidenceLineRange(string(raw))
			if !ok {
				// checkBriefFiles already makes an empty/absent Evidence section on a
				// verified row a hard PROBLEM; re-reporting it here would double-count
				// somebody else's finding.
				continue
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				skipped = append(skipped, fmt.Sprintf("%s (path not under --root: %v)", id, rerr))
				continue
			}
			work = append(work, pending{
				row:   evidenceActorRow{ID: id, Status: row.Status, Verified: row.Verified},
				rel:   filepath.ToSlash(rel),
				start: start, end: end,
			})
		}
	}

	if len(work) == 0 {
		return nil
	}

	rows := make([]evidenceActorRow, len(work))
	for i := range work {
		rows[i] = work[i].row
	}
	judged := evidenceActorJudge(p, rows, func(i int) ([]blameAuthor, error) {
		return blameEvidenceAuthors(root, work[i].rel, work[i].start, work[i].end)
	})

	var flagged, impostors, unreadable []string
	clean := 0
	for _, r := range judged {
		switch {
		case r.Err != nil:
			unreadable = append(unreadable, fmt.Sprintf("%s (%v)", r.ID, r.Err))
		case r.Verdict == actorVerifier || r.Verdict == actorHuman:
			clean++
		case r.Verdict == actorImpostor:
			// The per-row line carries the cell it contradicts: this class needs a
			// human to read one commit, and the Verified cell is what they are
			// deciding whether to keep believing.
			impostors = append(impostors, fmt.Sprintf("%s (row is %q, Verified cell %q) — %s",
				r.ID, r.Status, r.Verified, r.Reason))
		default:
			flagged = append(flagged, r.ID)
		}
	}
	sort.Strings(flagged)
	sort.Strings(impostors)
	sort.Strings(unreadable)

	var notices []string

	// The tamper signals first — one line each. These are not backlog.
	for _, im := range impostors {
		notices = append(notices, fmt.Sprintf(
			"Evidence-actor UNRESOLVED-IDENTITY: %s. This is a different class from an unbacked row and "+
				"needs a human to read the commit, because it has two readings and they prescribe "+
				"opposite actions: a MISCONFIGURED committer (the App's numeric App-id used in the "+
				"noreply address where the bot's USER id belongs — the same defect that leaves "+
				"`.author.login` null on GitHub), or an identity deliberately dressed as the verifier. "+
				"Neither backs the `verified` cell, and the check does not guess which it is "+
				"(desk-apps/07).", im))
	}

	if len(flagged) > 0 {
		pin := "id-pinned"
		if !p.idPinned() {
			pin = fmt.Sprintf("LOGIN-ONLY (the %s verifier entry carries no numeric id, so acceptance "+
				"rests on a re-registerable login)", scanEnvTrustedBotSlugs)
		}
		notices = append(notices, fmt.Sprintf(
			"Evidence-actor: %d of %d judged `verified`/`done` rows carry an Evidence section that no "+
				"accepted verifier actor committed — the `verified` claim rests on text the implementing "+
				"identity wrote about itself (F-verify-self-attest). %d row(s) are backed. Matching is %s. "+
				"NOTICE this phase, not a PROBLEM: the backlog predates the verifier-App Evidence cutover "+
				"(desk-apps/04), and arming a hard gate against it would red every unrelated PR. A row "+
				"clears by re-verification whose Evidence the verifier App commits. Rows: %s",
			len(flagged), clean+len(flagged)+len(impostors), clean, pin, strings.Join(flagged, ", ")))
	}

	if len(unreadable) > 0 {
		notices = append(notices, fmt.Sprintf(
			"could-not-check: Evidence-actor could not read git blame for %d row(s), which are counted "+
				"neither clean nor unbacked — %s", len(unreadable), strings.Join(unreadable, "; ")))
	}
	if len(skipped) > 0 {
		notices = append(notices, fmt.Sprintf(
			"could-not-check: Evidence-actor skipped %d row(s) before blame — %s",
			len(skipped), strings.Join(skipped, "; ")))
	}

	return notices
}
