package main

import (
	"github.com/medici-finance/assay/statusgen/evaluation"

	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// verifiedCellRe matches a dated runner in a Verified column: "YYYY-MM-DD <token...>".
var verifiedCellRe = regexp.MustCompile(evaluation.VerifiedStampPattern)

// verifiedTokenRe captures the first whitespace-token immediately after the
// leading date in a Verified cell (e.g. "2026-07-08 sonnet-verifier" -> "sonnet-verifier").
var verifiedTokenRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\s+(\S+)`)

// separatorRowRe matches a markdown table separator row ("|---|:--|--:|"),
// i.e. a row containing only pipes, dashes, colons and whitespace.
var separatorRowRe = regexp.MustCompile(`^[\s:|-]+$`)

// belowFloorModels is the SINGLE, explicit list of MODEL FAMILIES that sit
// below the risk-keyed verifier floor — the runners a
// risk-flagged brief may NOT be verified by.
//
// The criterion is CAPABILITY, NOT PRICE. This distinction is the whole point
// of the list and the reason it is an explicit named set rather than a pattern:
// the floor exists to stop a model that is not strong enough to actually
// re-run a Verify table from rubber-stamping a risk-flagged brief. A model that
// is merely inexpensive to call is not thereby weak, and must not be listed
// here. Concretely: `glm-5.2` is cheap on PRICE and strong on CAPABILITY, so it
// is deliberately absent — an earlier `glm` entry in this list conflated the
// two and produced false rejections of legitimate verifications.
//
// The keys are model-family names, matched against the NAME SEGMENTS of the
// runner token (see belowFloorRunner), never as bare substrings. Substring
// matching is what made the old list unsafe: `mini` silently swallows
// `gemini-*`, `lite` swallows `elite-*`, and `glm` swallowed every `glm-5.2-*`
// verifier. `mini`/`flash`/`lite` stay because they are real family names of
// small models (`gpt-4o-mini`, `gemini-2.5-flash`, `…-lite`), and segment
// matching now makes them mean exactly that.
//
// To CHANGE the floor, add or remove a family here — this is the one place the
// list lives. Add a family only if you are claiming it is too WEAK to verify a
// risk-flagged brief; never because it is cheap to run.
var belowFloorModels = map[string]bool{
	"deepseek": true,
	"sonnet":   true,
	"haiku":    true,
	"mini":     true,
	"flash":    true,
	"lite":     true,
}

// belowFloorRunner reports whether a runner token names a model family from
// belowFloorModels. The token is lowercased and split into name segments on
// any non-alphanumeric separator, and each segment has its trailing version
// digits stripped, so "sonnet-verifier", "via-deepseek", "claude-haiku-4.5"
// and "sonnet5" all match their family while "glm-5.2-verifier",
// "gemini-verifier" and "elite-verifier" do not.
func belowFloorRunner(token string) bool {
	segs := strings.FieldsFunc(strings.ToLower(token), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, seg := range segs {
		if belowFloorModels[strings.TrimRight(seg, "0123456789")] {
			return true
		}
	}
	return false
}

// humanRunnerName returns the <name> of a runner token of the form
// "human:<name>", and ok=false when the token is not a human stamp at all.
//
// The prefix test is exact and at the START of the token, matching
// hasHumanReviewer and the anchored humanStampRe in corroborate.go: neither
// "superhuman:alex" nor "non-human:alex" is a human stamp.
// The name is the leading ASCII-word run after the colon, so trailing
// punctuation ("human:alex,") is tolerated while a confusable/homoglyph name
// ("human:іan", Cyrillic і) yields an EMPTY name — which then fails the
// known-human lookup below rather than passing unexamined.
func humanRunnerName(token string) (name string, ok bool) {
	const prefix = "human:"
	if !strings.HasPrefix(strings.ToLower(token), prefix) {
		return "", false
	}
	rest := token[len(prefix):]
	end := 0
	for end < len(rest) {
		c := rest[end]
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' {
			end++
			continue
		}
		break
	}
	return rest[:end], true
}

// looksLikeHumanLogin reports whether name has the SHAPE of a real human login:
// a non-empty ASCII login token of GitHub-plausible length. humanRunnerName has
// already restricted the captured name to [0-9A-Za-z_] and yields "" for a
// confusable/homoglyph name (e.g. "human:іan", Cyrillic і), so this predicate's
// whole job is to separate a well-formed login from the empty / absurd-length
// residue. GitHub caps a login at 39 characters; a longer run is not a login.
//
// It is NOT the human-token exemption predicate — that is confirmedHumanEver
// (current-or-historical roster membership). Shape ALONE never clears the floor: a
// well-formed but never-confirmed login is a REJECT, not an accept. This predicate
// only sorts the two REJECT reasons apart, so verifierFloorFailure can tell an
// operator whether they wrote a name that was never a confirmed human (well-formed
// shape) or a token that names no login at all (empty/homoglyph name).
func looksLikeHumanLogin(name string) bool {
	return name != "" && len(name) <= 39
}

// confirmedHumanEver reports whether name was EVER a confirmed human: present in the
// CURRENT human-login map (ASSAY_HUMAN_LOGIN_MAP, HumanLogin) or recorded in the
// FORMER-humans map (ASSAY_FORMER_HUMAN_LOGIN_MAP, FormerHumanLogin) as a departed
// one. This is the verifier floor's human-exemption predicate: the floor clears for a
// human confirmed now OR historically, and rejects a name confirmed by neither. It is
// DELIBERATELY not used by the corroboration/register identity gates, which resolve
// through the current map ONLY — a departed human cannot approve today.
func confirmedHumanEver(name string) bool {
	if _, known := HumanLogin(name); known {
		return true
	}
	_, former := FormerHumanLogin(name)
	return former
}

// verifierFloorFailure reports whether a Verified cell FAILS the verifier floor
// and if so returns a self-contained reason clause the caller
// can splice into its problem line. A cell with no dated runner returns
// ("", false): that gap is attributionProblems' to report, not the floor's.
//
// THE FIX — an unbacked `human:` token no longer buys the exemption.
// The previous implementation opened with `if hasHumanReviewer(verified) { return
// ..., false }` — a blanket early return on a "human:" token appearing ANYWHERE in
// the cell. That is defensible on its face (if a human ran the table, tier does not
// apply) and was load-bearing in exactly the wrong way: appending " human:alex" to a
// Verified cell silently switched the whole control off, whether or not a human ran
// anything. `2026-07-31 sonnet-verifier human:alex` cleared the floor. The desk found
// a false `human:alex` in the Verified cell on four separate PRs in one day
// the false attribution was not sitting
// next to a passing gate, it WAS the thing keeping the gate from firing, and
// correcting the cell to the honest runner turned CI red immediately. The perverse
// incentive that creates — the cheapest way to clear the new red is to put
// `human:<name>` back — is why this needs a mechanism and not vigilance.
//
// The exemption is scoped to the one thing it was ever meant to mean: the
// RUNNER token itself — the first token after the date, i.e. WHO RAN THE TABLE —
// is a human. That preserves the legitimate accept (a genuine `2026-07-31
// human:alex` still clears, and so does `human:alex (sonnet-assisted)`) while the
// forgery (`sonnet-verifier human:alex`) is caught, because the human token is no
// longer standing where the runner goes.
//
// THE LEAVER PRINCIPLE — confirmed NOW or confirmed HISTORICALLY, never merely
// login-SHAPED. A Verified cell like `2026-07-18 human:ian` records who signed off
// THEN, so a later roster change must not retroactively red every board that person
// ran: dropping `ian` from the current map when he leaves cannot be allowed to fail
// his historical stamps. But the answer is NOT "clear for any well-formed login" —
// that would let `human:<any-plausible-name>` clear the floor whether or not a human
// was ever behind it, which is the forgery this gate must reject. So the exemption
// clears for a CONFIRMED human and only a confirmed one: a name present in the CURRENT
// roster (ASSAY_HUMAN_LOGIN_MAP, via HumanLogin) OR recorded in the FORMER-humans
// roster (ASSAY_FORMER_HUMAN_LOGIN_MAP, via FormerHumanLogin) as a departed human who
// WAS confirmed at some past point. A name in NEITHER — one that was never a confirmed
// human — FAILS, exactly as it did on main before the shape-based widening.
// confirmedHumanEver is that current-∪-former predicate. This is a tightening back
// toward main's map-membership check, carving out only the leaver case main got wrong
// (main red-lined departed humans by consulting the current map alone).
//
// WHAT STILL STOPS FABRICATION, and what this consciously accepts. The floor is a
// MODEL-CAPABILITY gate: it exists to stop a model too weak to re-run a Verify table
// from rubber-stamping a risk-flagged brief. A CONFIRMED `human:<login>` — current or
// historical — is by definition not a weak-model rubber-stamp, so admitting it does
// not weaken what the floor is FOR (a below-floor model in the runner slot, `human:`
// suffix or not, is still caught). Establishing that THIS named human actually ACTED
// on THIS PR is a different job and not this check's: it belongs to `--corroborate`
// (corroborateStamps) and the register human-authorisation key
// (authorizedByVerifiedHuman), BOTH of which resolve the name through the CURRENT map
// ONLY and require an APPROVED review / approval comment — a departed human cannot
// approve today, and a fabricated `human:whoever` resolves to nothing there. Those
// gates are UNCHANGED: "only confirmed humans can approve" is theirs, and the former
// map deliberately does NOT widen them. The residual this accepts is the same one the
// current map already carries (see rosterconfig.go): a name written into either map by
// whoever supplies the roster is trusted to have been a human; the id-pinning and
// corroboration gates, not the floor, are where identity is established downstream.
//
// A name that was NEVER a confirmed human FAILS LOUD — the correctness the shape-based
// form dropped and this restores. So does a MALFORMED human token: `human:` with no
// name, `human:@@@`, or a homoglyph name all yield an empty name from humanRunnerName
// and are not a login at all. The two are reported with DIFFERENT reasons
// (looksLikeHumanLogin sorts them), but both red-line: a token that names no confirmed
// human is not "gate satisfied". Note a malformed token would otherwise pass by
// accident — belowFloorRunner("human:") is false — so it must be rejected explicitly.
//
// The Verified/Reviewed split is deliberately NOT enforced here. The convention
// is that Verified names who ran the table and Reviewed carries the human sign-off,
// so a `human:` token in Verified is arguably always misplaced — but a human really
// can run a Verify table, and flagging that would be a new false rejection. This
// change only stops the token from SUPPRESSING a check it does not speak to.
func verifierFloorFailure(verified string) (reason string, failed bool) {
	m := verifiedTokenRe.FindStringSubmatch(verified)
	if m == nil {
		return "", false
	}
	runner := m[1]

	if name, isHuman := humanRunnerName(runner); isHuman {
		if _, known := HumanLogin(name); known {
			return "", false // a currently-confirmed human ran it — the floor does not apply
		}
		if _, former := FormerHumanLogin(name); former {
			// A human confirmed HISTORICALLY (in the roster at some past point, now
			// recorded in ASSAY_FORMER_HUMAN_LOGIN_MAP) but no longer current. The
			// leaver case: a stamp records who signed off THEN, and dropping a
			// departed human from the current map must not retroactively red the
			// boards they ran. Still a confirmed human, so the floor does not apply.
			return "", false
		}
		if looksLikeHumanLogin(name) {
			// A well-formed login SHAPE that was NEVER a confirmed human — in neither
			// the current nor the former map. Shape is not proof a human acted;
			// admitting it would launder exactly the forgery this floor must reject.
			return fmt.Sprintf("the runner token %q claims a human, but %q was never a confirmed human "+
				"(not in "+scanEnvHumanLoginMap+" now, nor recorded in "+scanEnvFormerHumanLoginMap+" as a "+
				"departed human) — only a currently- or historically-confirmed human clears the floor; name "+
				"the runner that actually ran the table, or add the human to the appropriate map (itself a "+
				"reviewed change)",
				runner, name), true
		}
		return fmt.Sprintf("the runner token %q claims a human, but %q is not a well-formed human login "+
			"(an empty or confusable name) — a human token that names no real login does not satisfy the floor; "+
			"name the runner that actually ran the table (a login of the form human:<name>)",
			runner, name), true
	}

	if belowFloorRunner(runner) {
		return fmt.Sprintf("runner %q is a model family the floor rejects on capability, not price", runner), true
	}
	return "", false
}

// runnerClearsFloor reports whether a SINGLE runner token satisfies the verifier
// floor on its own — the same admissibility verifierFloorFailure applies to the
// Verified cell's runner, factored out so the Evidence read below can reuse it.
// A `human:<login>` clears ONLY when the login was EVER a confirmed human — present
// in the current roster (ASSAY_HUMAN_LOGIN_MAP) or recorded in the former-humans
// roster (ASSAY_FORMER_HUMAN_LOGIN_MAP), the current-∪-former predicate
// confirmedHumanEver. A never-confirmed name (a well-formed login shape in neither
// map) does NOT clear, and neither does a MALFORMED human token (empty/confusable
// name) — the same "a token that names no confirmed human is not gate-satisfied"
// stance verifierFloorFailure takes. A plain, above-floor model (`opus-verifier`,
// `glm-5.2-verifier`) clears.
func runnerClearsFloor(token string) bool {
	if name, isHuman := humanRunnerName(token); isHuman {
		return confirmedHumanEver(name)
	}
	return !belowFloorRunner(token)
}

// evidenceFloorFailure reads the `## Evidence` section — the record of who
// ACTUALLY ran each row — and reports a floor violation the Verified cell alone
// conceals. This is the complete signal the risk-keyed floor is meant to read:
// the Verified cell names one runner and an agent can edit it in a single line,
// while Evidence records, per row, who ran the check the floor protects.
//
// The gap this closes: a risk-flagged brief whose
// Verified cell names an above-floor runner PASSED the cell-only floor even when
// its Evidence recorded rows genuinely run at a below-floor tier — "the floor is
// satisfied by a runner who did not run the rows the floor is protecting". Once
// Evidence is read, such a brief FAILS.
//
// A row is evaluated across EVERY Evidence table (parseEvidenceRows unions rows
// by ID, so an implementer run plus an independent re-run both count). The floor
// is satisfied PER ROW: a row run below the floor and then genuinely RE-RUN by an
// above-floor (or human) runner is CURED and does not fail — this is exactly the
// legitimate cheap-then-strong-re-run shape the issue calls out, and keeping it a
// pass is what stops this from becoming a new false rejection. Only a row whose
// completed Evidence runners are ALL below the floor (no curing re-run) poisons
// the gate. Rows with no completed Evidence are the UNRUN derivation's concern
// (unrun.go), not the floor's, so they are skipped here.
//
// Only a runner read from a DECLARED `Runner` column participates
// (evidenceRow.RunnerFromColumn). parseEvidenceRows also derives a runner from
// the last-two-cells fallback when a table names no columns; that value is a
// free-text output cell, not an attribution, and reading it here would break the
// floor both ways — a stray family word in an output cell false-rejects a strong
// run, and a below-floor row is laundered clear by a later fallback cell. The
// fallback stays available to the UNRUN derivation, where it is safe.
func evidenceFloorFailure(evidence string) (reason string, failed bool) {
	rows := parseEvidenceRows(evidence)
	var poisoned []string
	for id, ers := range rows {
		cleared := false
		belowRunner := ""
		for _, er := range ers {
			if !evidenceRowComplete(er) {
				continue // an unrun / dateless / runnerless row proves nothing either way
			}
			if !er.RunnerFromColumn {
				// The runner was read from parseEvidenceRows' last-two-cells
				// fallback, not a declared `Runner` column — i.e. it is a
				// free-text output cell, not an attribution. Letting it
				// participate breaks the floor in BOTH directions: a stray
				// family word in an output cell (`... sonnet ...`, `flash`)
				// false-REJECTS a strong run, and a below-floor row is
				// false-CLEARED ("laundered") by a later fallback cell that
				// reads as an above-floor token. So a fallback-derived runner
				// neither poisons nor cures the floor.
				continue
			}
			if runnerClearsFloor(er.Runner) {
				cleared = true
				break
			}
			if belowFloorRunner(er.Runner) && belowRunner == "" {
				belowRunner = er.Runner
			}
		}
		if !cleared && belowRunner != "" {
			poisoned = append(poisoned, fmt.Sprintf("row %s run only by %q", id, belowRunner))
		}
	}
	if len(poisoned) == 0 {
		return "", false
	}
	sort.Strings(poisoned)
	return strings.Join(poisoned, ", "), true
}

// attributionProblems is the brief-16 runner-attribution check: a `verified`/
// `done` brief-v1 brief's verification must be attributable to a runner
// distinct from the implementer. Scope is brief-v1 files only (the same
// schema opt-in gate as checkBriefFiles) whose README-row status is
// `verified` or `done` — legacy no-frontmatter briefs and todo/in-progress/
// implemented rows are exempt. It reuses parseBriefFile/briefFilePaths/
// expectedBriefID from brieffile.go rather than re-parsing brief files.
//
// This is BEST-EFFORT and says so in its checks, not just this comment:
// `authored:` names the brief's AUTHOR, not necessarily who later ran the
// verification, and sessions in a repo may share one git identity — a
// file-level check cannot see true cross-session independence. It catches
// the detectable cases (missing dated runner, a verifier token matching the
// author or reading as "the implementer"/"self", and Evidence rows that are
// all implementer-attributed) and nothing more (brief-16 scope-honesty note).
//
// The TOKEN checks above compare free strings an agent writes at will; the
// verifier/author tokens carry no binding to who actually committed anything
// (security-hardening ID-2, hole-2). This function therefore layers a SECOND,
// independent signal — a committer-identity cross-check read from git — that
// fails on different inputs than the token checks: it compares the git identity
// of the brief's authoring commit against that of the commit that most recently
// touched it (the Evidence-adding / status-flip commit).
//
// SEVERITY (security-hardening/27 Task 2, assay#1116): same identity +
// self-labeled independence = the selfVerification hard error — but ONLY when
// identity is actually DISCRIMINATING in this repo, i.e. more than one git
// identity appears across the checked briefs' history. A same-identity brief
// is escalated to a hard PROBLEM when (a) this repo has more than one
// committer identity in its brief history (so "same identity" is a genuine
// signal, not an artifact of everything sharing one identity) and (b) the
// brief's own tokens SELF-LABEL as independent — selfVerificationReason found
// nothing and evidenceHasIndependentRow is true — meaning the token layer
// alone would have passed it; the identity layer is precisely what catches
// this case (hole-2: "authored: … by fable" then "verified: … opus" defeats
// selfVerificationReason with no cross-check). A brief the token checks
// already flagged (reason != "" or no independent Evidence row) is not
// double-flagged here — it is already a hard PROBLEM from that layer.
//
// The single-identity-repo case stays a NOTICE, deliberately, and is not
// escalated: when EVERY checked brief's authoring and Evidence commits sit
// under one shared git identity (the common shape for a repo that commits
// everything under one bot/App identity), commit metadata cannot discriminate
// author from verifier at all — escalating there would fail closed against a
// legitimate, common operating shape (a solo-maintainer or single-bot-identity
// repo) rather than against a real independence gap, which is exactly the new
// false-positive class this change must not introduce. That case remains a
// loud, honest degradation: token-level attribution is the only independence
// signal commit metadata can offer there.
//
// THE OVERRIDE PATH (measured-status/05). The hard PROBLEM above stays the default.
// The one sanctioned way through it for a legitimate same-identity re-touch is a
// recorded `independence-override:` frontmatter value naming a human that
// independenceOverrideRefusal accepts — a current human, not the author, committed
// under a git author identity other than the one excused, and still covering the
// Evidence as it stands. An accepted override is reported as a NOTICE, never silently; a refused
// one leaves the PROBLEM standing and says why. It excuses this committer-identity
// PROBLEM only: the token-level checks above are untouched by it.
//
// It must still degrade LOUDLY, never silently, when git cannot answer at all
// (no .git, or a brief's own history unreadable) — those paths are unchanged.
//
// Returns (problems, notices): hard problems change the exit code; notices are
// printed and do not.
func attributionProblems(streams []*Stream) (problems, notices []string) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	addNotice := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	// Committer-identity cross-check accumulators (hole-2), resolved after the
	// scan once we know whether this repo's brief history uses a single git
	// identity (commit metadata then cannot corroborate independence — one
	// aggregate NOTICE) or several (a brief whose authoring and Evidence commits
	// share one identity is not independently verified — surfaced, still a
	// NOTICE, so an honest shared-identity re-run is never red-lined).
	type identPair struct {
		label, id string
		// selfLabeledIndependent is true when this brief's TOKEN-level checks
		// (selfVerificationReason, evidenceHasIndependentRow) found nothing wrong
		// — i.e. the tokens alone read as independent. That is precisely the
		// "self-labeled independence" half of the escalation condition: a brief
		// the token layer already flagged is already a hard PROBLEM from that
		// layer and is not re-flagged here.
		selfLabeledIndependent bool
		// root and rel locate this brief's file for the precision refinement pass
		// below (evidenceSectionTouchedByOtherIdentity) — only run on escalation
		// candidates, so it costs nothing for the common case of a distinct
		// authoring/last-toucher identity.
		root, rel string
		// override and authored carry the brief's `independence-override:` and
		// `authored:` values for the recorded-override pass (measured-status/05),
		// consulted ONLY for a confirmed same-identity PROBLEM.
		override, authored string
		// forced is true when the brief is in the precise pass ONLY because it
		// carries an override: its whole-file authoring and most-recent commit are
		// under DIFFERENT identities, so it is not a same-identity candidate itself.
		forced bool
	}
	var sameIdentity []identPair
	idents := map[string]bool{}
	gitReadable := false // at least one brief yielded readable commit identity

	// ONE history walk per root, built lazily on the first root that needs it, replacing the
	// per-brief `git log` PAIR this loop used to run. A root whose walk fails simply has no
	// index and every brief under it falls back to the per-path reads — the answer is the same
	// either way, which is the property that lets this be a cost change and not a behaviour one.
	authorIdx := map[string]map[string]pathAuthors{}

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				// Malformed files are reported by checkBriefFiles; legacy/
				// opted-out files are exempt here as everywhere else.
				continue
			}
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
				continue // no row, or a status this check does not apply to
			}

			label := fmt.Sprintf("%s/brief-%s", s.Name, num)

			if !verifiedCellRe.MatchString(row.Verified) {
				add(`%s: Verified cell must name a dated runner ("YYYY-MM-DD <runner>")`, label)
				continue // no dated runner to extract a token from
			}

			reason := selfVerificationReason(bf.Authored, row.Verified)
			if reason != "" {
				add("%s: Verified runner looks like self-verification (%s)", label, reason)
			}

			hasIndependentRow := evidenceHasIndependentRow(bf.Evidence)
			if !hasIndependentRow {
				add("%s: verified requires an independent (non-implementer) Evidence row", label)
			}

			// self-labeled independence: the token layer found nothing wrong —
			// used below to gate the committer-identity escalation (security-
			// hardening/27 Task 2, assay#1116).
			selfLabeledIndependent := reason == "" && hasIndependentRow

			// --- committer-identity cross-check (security-hardening ID-2, hole-2) ---
			// A `git archive` export has no .git and nothing git can answer about
			// this tree — there is no adversary to fail closed against, only an
			// absence of data (see hasNoGitDir) — so skip it SILENTLY: a per-brief
			// notice on every export would be pure noise, not a degradation worth
			// announcing.
			if hasNoGitDir(s.Root) {
				continue
			}
			rel, relErr := filepath.Rel(s.Root, path)
			if relErr != nil {
				rel = path
			}
			idx, built := authorIdx[s.Root]
			if !built {
				idx, _ = buildPathAuthorIndex(s.Root, filepath.FromSlash("docs/streams"))
				authorIdx[s.Root] = idx // nil on failure: cached so the walk is attempted once
			}
			authoringID, evidenceID, idok := lookupPathAuthors(idx, s.Root, rel)
			if !idok {
				// .git EXISTS but this brief's own history is unreadable — an
				// untracked / just-added file, or a shallow clone that truncated
				// it. Degrade LOUDLY so a reader knows the identity layer did not
				// run for this brief; never a silent pass.
				addNotice("%s: could not cross-check verification independence against commit history "+
					"(brief file untracked or its history unavailable) — token-level attribution is the only "+
					"independence signal for this brief", label)
				continue
			}
			gitReadable = true
			idents[authoringID] = true
			idents[evidenceID] = true
			// A brief that CARRIES an independence-override is always judged by the
			// precise Evidence-section pass below, even when its whole-file last
			// toucher differs from its author (the override's own commit usually
			// makes it differ): the override is then reported as honored or
			// refused, never silently moot (measured-status/05).
			if authoringID == evidenceID || (bf.IndependenceOverride != "" && selfLabeledIndependent) {
				sameIdentity = append(sameIdentity, identPair{label, authoringID, selfLabeledIndependent, s.Root, rel,
					bf.IndependenceOverride, bf.Authored, authoringID != evidenceID})
			}
		}
	}

	// Resolve the gathered committer-identity signal into at most one NOTICE.
	if gitReadable {
		switch {
		case len(idents) <= 1:
			// One git identity behind every checked brief's authoring AND latest
			// commit: commit metadata cannot tell author from verifier here (all
			// sessions share one identity). Say so ONCE, loudly — the token checks
			// are the only independence signal — rather than emit a non-signal per
			// brief, and never let the layer pass silently.
			var only string
			for id := range idents {
				only = id
			}
			addNotice("verification-independence: committer-identity cross-check is inconclusive for this "+
				"repository — every checked brief's authoring and Evidence commits are under one git identity "+
				"(%q), so commit metadata cannot corroborate cross-session independence; token-level "+
				"attribution is the only independence signal here", only)
		case len(sameIdentity) > 0:
			// Multiple identities exist in this repo, so identity IS discriminating
			// — yet these briefs' authoring and most-recent (Evidence-adding)
			// commits are under ONE identity: the same actor both wrote and last
			// touched the brief. Split by whether the TOKEN layer already caught
			// it (security-hardening/27 Task 2, assay#1116):
			//
			//   - selfLabeledIndependent == true: the tokens alone read as
			//     independent (selfVerificationReason found nothing, an
			//     independent Evidence row exists) — exactly hole-2, the case the
			//     token layer cannot see. With identity actually discriminating
			//     in this repo, same authoring/Evidence identity is now positive
			//     evidence of non-independence, not just an inconclusive
			//     coincidence, so this escalates to a hard PROBLEM: the
			//     selfVerification error the brief's Task 2/4(b) requires.
			//   - selfLabeledIndependent == false: the token layer already
			//     flagged this brief (self-verification reason, or no
			//     independent Evidence row) — already a hard PROBLEM from that
			//     layer. Not re-flagged as a second problem; surfaced instead as
			//     a bounded corroborating NOTICE so the identity signal is still
			//     visible without duplicating the failure.
			var escalated, corroborating []identPair
			for _, p := range sameIdentity {
				if p.selfLabeledIndependent {
					escalated = append(escalated, p)
				} else {
					corroborating = append(corroborating, p)
				}
			}
			sort.Slice(escalated, func(i, j int) bool { return escalated[i].label < escalated[j].label })

			// Precision refinement (assay#1277 review findings, two rounds): the fast
			// index's "most recent commit touching the path" is a whole-FILE,
			// most-recent-only signal, and BOTH of those properties can be wrong.
			// Whole-file: a later, unrelated, repo-wide mechanical commit (a
			// formatting pass, a brief-schema migration) that never went near the
			// Evidence section resets it back to the authoring identity, masking a
			// genuinely distinct identity that landed the real Evidence commit in
			// between. Most-recent-only: even scoped to the Evidence section alone, a
			// later SAME-identity commit that adds a caveat/addendum next to an
			// already-independently-verified row would read as "the last toucher is
			// the author" even though independence was genuinely established earlier
			// in that same section's history. evidenceSectionTouchedByOtherIdentity
			// answers the question that is actually at stake — did an identity OTHER
			// than the author EVER touch the Evidence section, not only most recently.
			// Only escalation CANDIDATES need this refinement — a brief already
			// reading as distinct-identity from the fast index is unaffected either
			// way, and a brief the token layer already flagged (the `corroborating`
			// bucket below) is already a hard PROBLEM regardless of this refinement.
			var confirmed, unconfirmed, unjudgedOverride []identPair
			history := evidenceHistoryChecks(len(escalated), func(i int) (bool, bool) {
				p := escalated[i]
				return evidenceSectionTouchedByOtherIdentity(p.root, p.rel, p.id)
			})
			for i, p := range escalated {
				touchedByOther, pok := history[i].touchedByOther, history[i].known
				switch {
				case !pok:
					// Could not determine the Evidence section's own history (no
					// "## Evidence" heading found, unreadable history, git
					// unavailable) — fail OPEN to advisory here: never turn an
					// undetermined case into a false hard PROBLEM. A brief that is
					// here only because it carries an override is not a same-identity
					// whole-file candidate, so it is reported as an override that could
					// not be judged, not under the same-identity notice below.
					if p.forced {
						unjudgedOverride = append(unjudgedOverride, p)
					} else {
						unconfirmed = append(unconfirmed, p)
					}
				case touchedByOther:
					// An identity other than the author touched the Evidence section at
					// some point in its history — independence was genuinely
					// established; not a problem, and not even worth a NOTICE.
				default:
					// Confirmed: every commit that ever touched the Evidence section
					// shares the author's identity — a real selfVerification, not an
					// artifact of a later commit resetting a most-recent-only signal.
					confirmed = append(confirmed, p)
				}
			}
			for _, p := range confirmed {
				// The recorded override naming a human (measured-status/05): the
				// hard failure stays the default, and only an override that passes
				// EVERY condition of independenceOverrideRefusal turns it into a
				// visible NOTICE. An absent override is the default path, unchanged.
				overrideNote := ""
				if p.override != "" {
					ov, sha, why := independenceOverrideRefusal(p.root, p.rel, p.id, p.override, p.authored)
					if why == "" {
						addNotice("%s: committer-identity cross-check FAILS (authoring and every Evidence commit "+
							"share one git identity, %q) but is OVERRIDDEN by a recorded independence-override naming "+
							"human:%s, dated %s, committed under a different git author identity in %s (lint does not "+
							"corroborate that the named human authorized it); reason: %s",
							p.label, p.id, ov.name, ov.date, shortSHA(sha), ov.reason)
						continue
					}
					overrideNote = "; its recorded independence-override is NOT honored: " + why
				}
				add("%s: verification independence FAILS the committer-identity cross-check — the authoring "+
					"commit and every commit that ever touched this brief's Evidence section share one git "+
					"identity (%q), even though the Verified/Evidence tokens self-label as independent; this "+
					"repo's brief history carries more than one git identity, so identity IS discriminating "+
					"here — this is the security-hardening/27 selfVerification failure (same identity + "+
					"self-labeled independence): confirm the Evidence was genuinely run by someone other than "+
					"the author, or correct the runner/author token, or have a human record an "+
					"independence-override (measured-status/05)%s", p.label, p.id, overrideNote)
			}
			if len(unconfirmed) > 0 {
				labels := make([]string, 0, len(unconfirmed))
				for _, p := range unconfirmed {
					labels = append(labels, fmt.Sprintf("%s (%s)", p.label, p.id))
				}
				sort.Strings(labels)
				addNotice("verification-independence: committer-identity cross-check — could not precisely "+
					"identify the commit that landed the Evidence content for %d brief(s) whose whole-file "+
					"authoring and most-recent commit share one identity (no \"## Evidence\" heading found, or "+
					"its history was unreadable); token-level attribution is the only independence signal for "+
					"these until that resolves: %s", len(unconfirmed), strings.Join(labels, ", "))
			}
			if len(unjudgedOverride) > 0 {
				labels := make([]string, 0, len(unjudgedOverride))
				for _, p := range unjudgedOverride {
					labels = append(labels, p.label)
				}
				sort.Strings(labels)
				addNotice("verification-independence: committer-identity cross-check — the recorded "+
					"independence-override of %d brief(s) could not be judged: each is in the precise "+
					"Evidence-section pass only because it carries an override (its whole-file authoring and "+
					"most-recent commit are under different identities), and its Evidence section's own history "+
					"could not be read (no \"## Evidence\" heading found, or its history was unreadable); the "+
					"override is neither honored nor refused until that resolves: %s",
					len(unjudgedOverride), strings.Join(labels, ", "))
			}
			if len(corroborating) > 0 {
				labels := make([]string, 0, len(corroborating))
				for _, p := range corroborating {
					labels = append(labels, fmt.Sprintf("%s (%s)", p.label, p.id))
				}
				sort.Strings(labels)
				const maxShown = 8
				shown := labels
				suffix := ""
				if len(labels) > maxShown {
					shown = labels[:maxShown]
					suffix = fmt.Sprintf(", +%d more", len(labels)-maxShown)
				}
				addNotice("verification-independence: committer-identity cross-check — %d already-flagged "+
					"verified/done brief(s) also have their authoring and most-recent (Evidence-adding) commit "+
					"under one git identity, corroborating the token-level finding already reported as a "+
					"PROBLEM above: %s%s", len(corroborating), strings.Join(shown, ", "), suffix)
			}
		}
	}

	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// independenceOverrideRe is the grammar of an `independence-override:` value:
// `human:<name> YYYY-MM-DD <reason>`. The human token must OPEN the value — an
// on-behalf-of relay (`on-behalf-of human:<name>`) or a prose mention is not the
// human's own authorization and does not parse — and the name is the same ASCII
// word run humanRunnerName accepts, so a confusable spelling does not match.
var independenceOverrideRe = regexp.MustCompile(`^human:([0-9A-Za-z_]+)\s+(\d{4}-\d{2}-\d{2})\s+(.*)$`)

// independenceOverride is one parsed `independence-override:` value.
type independenceOverride struct {
	name, date, reason string
}

// parseIndependenceOverride parses raw against independenceOverrideRe. why is ""
// on success, else the reason the value is malformed. The date must be a real
// calendar date (2026-02-30 is malformed). The reason must say
// something: leading separators ("—", "-", ":") are trimmed and an empty
// remainder is malformed, so a bare `human:<name> <date>` authorizes nothing.
func parseIndependenceOverride(raw string) (ov independenceOverride, why string) {
	m := independenceOverrideRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return ov, fmt.Sprintf("the value %q does not read as \"human:<name> YYYY-MM-DD <reason>\"", raw)
	}
	if _, err := time.Parse("2006-01-02", m[2]); err != nil {
		return ov, fmt.Sprintf("the date %q is not a calendar date", m[2])
	}
	reason := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(m[3]), "—–-: "))
	if reason == "" {
		return ov, fmt.Sprintf("the value %q gives no reason", raw)
	}
	return independenceOverride{name: strings.ToLower(m[1]), date: m[2], reason: reason}, ""
}

// independenceOverrideRefusal judges a brief's recorded `independence-override:`
// against the committer-identity cross-check PROBLEM it would excuse
// (measured-status/05; the ruling on that brief's decision issue: hard failure,
// but also an explicit override path). It returns the parsed override and the
// commit that recorded it, and why == "" ONLY when every condition holds:
//
//  1. GRAMMAR — the value is `human:<name> YYYY-MM-DD <reason>` with a real
//     calendar date and a non-empty reason (parseIndependenceOverride).
//  2. A CURRENT HUMAN — <name> resolves through the CURRENT human-login map
//     (HumanLogin), the same map the approval gates use. A name in the
//     former-humans map only, or in neither, authorizes nothing: a departed human
//     cannot authorize today, and an unconfigured map honors no override at all.
//  3. NOT THE AUTHOR — <name> is not the brief's own `authored:` token: the human
//     who wrote the work does not excuse its verification.
//  4. NOT IN A FIXTURE CORPUS — the brief does not lie in a declared fixture corpus
//     (isExcludedFixturePath). `--corroborate` skips the human:<name> stamps there,
//     so an override in one could never be corroborated online.
//  5. A DIFFERENT RECORDING IDENTITY — the frontmatter line carrying the override,
//     as committed at HEAD, was last written by a commit whose git AUTHOR identity
//     (the commit object's own email, mailmap off — frontmatterKeyLineProvenance)
//     differs from the identity the PROBLEM names. An uncommitted override, or a
//     working-tree value that differs from the committed line, has no recorder and
//     is refused.
//  6. NOT POSTDATED — the override's date is not later than the committer date of
//     the commit that recorded it (one day of slack for timezones). There is no
//     lower bound: a human may rule days before the line is committed.
//  7. NOT STALE — the brief's current Evidence section (working tree) says exactly
//     what it said in the commit that recorded the override. The override covers
//     the Evidence the human saw; any later Evidence edit, committed or not,
//     re-opens the PROBLEM until a human records the override again. This is
//     judged by CONTENT, never by commit ancestry (#2026).
//
// WHAT THIS DOES NOT PROVE. Conditions 5 and 6 read commit metadata, and a git
// author identity and committer date are whatever the committer set. So this
// check stops the excused identity recording an override under its OWN author
// email; it does not stop an actor who commits under a second email or as
// another automation identity, and nothing here ties <name> to the person who
// made the commit. The control that would bind <name> to that human is ONLINE:
// `--corroborate` treats the `human:<name>` token on an ADDED override line like
// every other added human stamp (stampsInDiff), so on a PR it is corroborated only
// by that human's own approval on the PR. That control binds only where
// `--corroborate` actually runs on the PR that adds the override; it is not part
// of this offline check.
//
// Every git read that cannot answer is a refusal, never a pass.
func independenceOverrideRefusal(root, rel, excusedID, raw, authored string) (ov independenceOverride, sha, why string) {
	ov, why = parseIndependenceOverride(raw)
	if why != "" {
		return ov, "", why
	}
	if _, current := HumanLogin(ov.name); !current {
		if _, former := FormerHumanLogin(ov.name); former {
			return ov, "", fmt.Sprintf("human:%s is a former human, not in the current human-login map — a departed human cannot authorize an override today", ov.name)
		}
		return ov, "", fmt.Sprintf("human:%s does not resolve through the current human-login map, so no human is named", ov.name)
	}
	if tok, ok := authorToken(authored); ok && tok == ov.name {
		return ov, "", fmt.Sprintf("human:%s is the brief's own author — the author does not excuse the verification of their own work", ov.name)
	}
	if isExcludedFixturePath(root, filepath.ToSlash(rel)) {
		return ov, "", "the brief lies in a declared fixture corpus, whose human:<name> stamps --corroborate skips, so the override could never be corroborated online"
	}
	value, recSHA, recID, recDay, ok := frontmatterKeyLineProvenance(root, rel, "independence-override")
	if !ok {
		return ov, "", "the override line is not committed at HEAD (or git cannot attribute it), so who recorded it is unknown"
	}
	if value != strings.TrimSpace(raw) {
		return ov, "", "the committed override is not a single-line value matching the working tree, so the line git attributes is not the override being judged"
	}
	if recID == excusedID {
		return ov, "", fmt.Sprintf("it was recorded by the same git identity it would excuse (%q) — the override line must be committed under a different git author identity", recID)
	}
	authorizedOn, _ := time.Parse("2006-01-02", ov.date) // validated by parseIndependenceOverride
	recordedOn, err := time.Parse("2006-01-02", recDay)
	if err != nil {
		return ov, "", fmt.Sprintf("the date of the commit that recorded the override (%s) could not be read", shortSHA(recSHA))
	}
	if authorizedOn.After(recordedOn.AddDate(0, 0, 1)) {
		return ov, "", fmt.Sprintf("it is dated %s, later than the commit that recorded it (%s, %s) — an authorization cannot postdate its own record", ov.date, shortSHA(recSHA), recDay)
	}
	seen, ok := evidenceSectionAtCommit(root, recSHA, rel)
	if !ok {
		return ov, "", fmt.Sprintf("the Evidence section as committed with the override (%s) could not be read, so the override cannot be shown to cover today's Evidence", shortSHA(recSHA))
	}
	cur, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ov, "", "the brief's current Evidence section could not be read, so the override cannot be shown to cover it"
	}
	if now, found := evidenceSectionText(cur); !found || now != seen {
		return ov, "", fmt.Sprintf("it is stale — the Evidence section differs from what it said when the override was recorded in %s; a human must record it again", shortSHA(recSHA))
	}
	return ov, recSHA, ""
}

type evidenceHistoryResult struct {
	touchedByOther bool
	known          bool
}

// Evidence histories are independent Git reads. Bound process pressure like
// the Evidence-actor blame pool, then consume indexed results in the original
// order so concurrency never changes verdict ordering or unknown handling.
const evidenceHistoryWorkers = 8

func evidenceHistoryChecks(count int, check func(int) (bool, bool)) []evidenceHistoryResult {
	results := make([]evidenceHistoryResult, count)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < min(count, evidenceHistoryWorkers); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i].touchedByOther, results[i].known = check(i)
			}
		}()
	}
	for i := 0; i < count; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

// principalAttributionCutoverDate is the date the on-behalf-of write path landed
// (this PR's own head, tools/desk/internal/deskkit/principal.go's six writers). It is
// the ONE place that boundary is spelled — never re-derived from a commit sha or a
// feature flag — because the question the cutover answers ("could this row's write
// path have stamped an annotation at all") is decided by calendar time, not by which
// commit a particular checkout happens to have.
//
// WHY A CUTOVER, NOT A HARD PROBLEM FROM DAY ONE (reviewer findings, both lanes:
// "the new check is retroactive with no grandfathering"). A repo's `## Evidence`
// history predates this feature by construction — no write path before this landing
// could have stamped an annotation — so treating every missing-annotation row as a
// hard PROBLEM condemns history the moment the check ships, with the only greens
// being "hand-edit closed briefs" (manufacturing the exact attribution the annotation
// exists to record — rejected, same reasoning as the EXECUTION WITNESS NOTICE below)
// or "weaken/drop the check" (rejected explicitly by the security lane: that erases
// the enforcement this PR exists to add). A cutover is the third option neither
// review rejected: a row from BEFORE the write path existed could not have carried an
// annotation and is reported as a NOTICE (visible, not silently dropped); a row from
// AT OR AFTER the cutover COULD have gone through a stamping write path, so a missing
// annotation there is a hard PROBLEM — exactly the enforcement the feature is for,
// scoped to the window it can actually apply to.
//
// This mirrors witnessNotices' (main.go) own transition rule for the identical shape
// ("a brief closed before the witness existed cannot have one, and hand-writing
// witnesses into closed briefs to green the gate would manufacture exactly the
// evidence the witness exists to replace") — NOT a blanket downgrade to NOTICE, which
// the security lane's review explicitly named as a route that weakens the control.
const principalAttributionCutoverDate = "2026-09-17"

// principalAttributionProblems is multi-principal/01's lint half (Task item 4): an
// App-authored Evidence row carries the on-behalf-of annotation verifyrun.go's row()
// renders (`on-behalf-of human:<login-or-neutral-name>`, inside the Runner cell's
// trailing parenthetical), or the row is flagged.
//
// TWO shapes. The unknown-principal shape is always a hard PROBLEM (an annotation that
// names an unrecognised login was written by a stamping path and is simply wrong — no
// cutover question applies, because the row IS annotated). The missing-annotation shape
// is cutover-gated (principalAttributionCutoverDate, above): a PROBLEM for a row dated
// at or after the cutover (the write path could have stamped it and did not), a NOTICE
// for a row dated before it (no write path could have). A row with no readable Date cell
// is treated as pre-cutover (NOTICE, not PROBLEM) — a malformed date is a different
// check's concern (attributionProblems' verifiedCellRe family), not a reason to fail
// this one closed against a row this feature could not have touched.
//
// A HUMAN Runner (`human:<name>`) needs no annotation at all and is never checked here
// — the runner already names the acting human directly, which is the whole property
// on-behalf-of exists to recover for a SHARED App identity.
func principalAttributionProblems(streams []*Stream) (problems, notices []string) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	addNotice := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }
	cfg := scanEffectiveConfig()
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			label := fmt.Sprintf("%s/brief-%s", s.Name, num)
			rows := parseEvidenceRows(bf.Evidence)
			for _, id := range sortedEvidenceRowIDs(rows) {
				for _, r := range rows[id] {
					if isHumanRunnerToken(r.Runner) || !isAppRunnerToken(r.Runner) {
						continue // not an App/bot runner — nothing for this check to say
					}
					login, hasAnnotation := onBehalfOfPrincipalOf(r.Runner)
					if !hasAnnotation {
						msg := fmt.Sprintf("%s: Evidence row #%s was run by an App identity (%s) with no "+
							"on-behalf-of principal — every App-authored Evidence row must carry the "+
							"on-behalf-of annotation the write path stamps (see docs/on-behalf-of.md)",
							label, id, r.Runner)
						if strings.TrimSpace(r.Date) != "" && r.Date >= principalAttributionCutoverDate {
							problems = append(problems, msg)
						} else {
							addNotice("%s", msg+fmt.Sprintf(" — NOTICE, not a PROBLEM: dated %q, before the "+
								"on-behalf-of write path landed (%s); no write path could have stamped this "+
								"row, so it is grandfathered rather than condemned (flip to PROBLEM applies "+
								"automatically once the row's own date reaches the cutover)",
								strings.TrimSpace(r.Date), principalAttributionCutoverDate))
						}
						continue
					}
					if cfg.Configured() {
						// The principal is the login on a known-private repo and the roster's
						// neutral name elsewhere (principal.go's visibility split), so either
						// form is accepted — the name only through ASSAY_HUMAN_LOGIN_MAP, onto a
						// login the roster recognises as human.
						if !onBehalfOfPrincipalRecognised(cfg, login) {
							add("%s: Evidence row #%s's on-behalf-of principal %q is not in this repo's "+
								"roster human map — a principal must be a login the roster recognises as "+
								"human, or the neutral name the human-login map carries for one", label, id, login)
						}
					}
				}
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// sortedEvidenceRowIDs returns rows' keys sorted numerically-then-lexically, so
// principalAttributionProblems reports in a stable, readable order rather than Go's
// randomised map iteration.
func sortedEvidenceRowIDs(rows map[string][]evidenceRow) []string {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ni, ei := strconv.Atoi(ids[i])
		nj, ej := strconv.Atoi(ids[j])
		if ei == nil && ej == nil {
			return ni < nj
		}
		return ids[i] < ids[j]
	})
	return ids
}

// authorToken extracts the author's identifying token from an `authored:`
// frontmatter value of the form "YYYY-MM-DD by <author> ...". ok is false
// when there is no " by " marker to key off (e.g. a bare-date authored value)
// — the self-verification comparison is then skipped rather than guessed at.
func authorToken(authored string) (tok string, ok bool) {
	_, after, found := strings.Cut(authored, " by ")
	if !found {
		return "", false
	}
	fields := strings.Fields(after)
	if len(fields) == 0 {
		return "", false
	}
	return strings.ToLower(fields[0]), true
}

// selfVerificationReason reports why a Verified cell reads as self-verification,
// or "" if it doesn't. Caller must have already confirmed verifiedCellRe matches.
func selfVerificationReason(authored, verified string) string {
	m := verifiedTokenRe.FindStringSubmatch(verified)
	if m == nil {
		return ""
	}
	verifierTok := strings.ToLower(m[1])

	if authorTok, ok := authorToken(authored); ok && verifierTok == authorTok {
		return fmt.Sprintf("verifier %q matches the brief's author %q", verifierTok, authorTok)
	}
	if implementerAttributed(verifierTok) {
		return fmt.Sprintf("verifier %q contains the token \"implementer\" — it names the implementer (a self-labelled \"non-implementer\" is still a self-assertion), not an independent runner", verifierTok)
	}
	if verifierTok == "self" {
		return fmt.Sprintf("verifier %q is literally \"self\"", verifierTok)
	}
	return ""
}

// implementerAttributed reports whether a Runner cell names the implementer —
// i.e. is NOT an independent runner.
//
// A cell containing the token "implementer" in ANY spelling — the plain
// "implementer (Opus 4.8)" AND the self-asserted "worker (non-implementer)" —
// names the implementer and does not count as independent
// (security-hardening ID-2, hole-1). The earlier form stripped the literal
// "non-implementer" before the substring test, so a cell reading
// "sonnet verifier (non-implementer)" passed the independence check outright.
// That turned an implementer's own affirmative self-certification ("I ran this
// as a non-implementer") into a pass — exactly the self-assertion this gate
// exists to deny: the tool would be trusting the claim text to certify the very
// independence it is meant to establish. An honest independent verifier names
// the runner that actually ran the table (e.g. "opus-verifier",
// "sonnet-verifier") — a token that does not contain "implementer" at all — and
// still reads as independent; only a token that spells out "implementer" (with
// or without a "non-" prefix) is now treated as implementer-attributed.
//
// The match is case-insensitive so the predicate holds "in ANY spelling"
// regardless of whether a caller pre-lowercased the cell.
func implementerAttributed(runnerCell string) bool {
	return strings.Contains(strings.ToLower(runnerCell), "implementer")
}

// evidenceHasIndependentRow reports whether an Evidence section contains at
// least one markdown table content row (outside HTML comments, beyond a
// header/separator pair) whose last (Runner) cell is not implementer-
// attributed. A section may hold more than one table (e.g. an
// "implementer run" table followed by an "independent re-run" table); each
// table's own header+separator pair is skipped in turn.
//
// Cells are split with splitRowEscaped, the local escape-aware variant. A
// Command cell legitimately contains an escaped pipe (e.g.
// “ `grep -ciE "arm64\|amd64"` “); a splitter that does not know about the
// escape cuts the row there too, so a row that names a "Runner" column by index
// off the header reads the WRONG cell as Runner for every row after the
// escape — one that need not contain "implementer" at all. That silently
// flips an all-implementer Evidence table into one that reads as
// independently backed: the gate goes green with zero independent rows. This
// is the same escape-aware splitter parseEvidenceRows (unrun.go) already
// uses for the identical by-header-column read.
//
// A named Runner column is only trustworthy when the row's cell count
// matches the header's. splitRowEscaped correctly
// treats an UNESCAPED pipe as a real delimiter, so a Command cell holding one
// (e.g. an unescaped shell pipe in a backticked snippet — the common mistake
// an author makes when they forget to escape) still shifts every cell after
// it, same as the escaped-pipe bug this function was fixed for. A row whose
// cell count disagrees with the header's is therefore unreadable at that
// index and is skipped rather than trusted, so it can never count as
// independent — a ragged row fails the gate closed, not open.
func evidenceHasIndependentRow(evidence string) bool {
	stripped, _ := stripRowComments(evidence)
	lines := strings.Split(stripped, "\n")
	runnerIdx := -1   // -1 = no header seen yet for this table: use the last cell
	headerCells := -1 // -1 = no header seen yet for this table: skip the alignment check
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			runnerIdx, headerCells = -1, -1 // table ended; the next table names its own columns
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue // an orphan separator row
		}
		// A header row is immediately followed by a separator row; note where
		// its "Runner" column sits (if named) and how many cells it has, then
		// skip both rows.
		if i+1 < len(lines) && separatorRowRe.MatchString(strings.Trim(strings.TrimSpace(lines[i+1]), "|")) {
			runnerIdx = -1
			headerRow := splitRowEscaped(line)
			headerCells = len(headerRow)
			for j, c := range headerRow {
				if strings.EqualFold(strings.TrimSpace(c), "runner") {
					runnerIdx = j
				}
			}
			i++
			continue
		}
		cells := splitRowEscaped(line)
		if len(cells) == 0 {
			continue
		}
		// A named Runner column's index only identifies the right cell when
		// this row's cell count matches the header's; a ragged row (an
		// unescaped pipe, a missing or extra cell) would otherwise silently
		// mis-locate it. Skip it — never guess.
		if runnerIdx >= 0 && headerCells >= 0 && len(cells) != headerCells {
			continue
		}
		idx := len(cells) - 1
		if runnerIdx >= 0 && runnerIdx < len(cells) {
			idx = runnerIdx
		}
		runner := strings.ToLower(strings.TrimSpace(cells[idx]))
		if runner == "" {
			continue
		}
		if !implementerAttributed(runner) {
			return true
		}
	}
	return false
}
