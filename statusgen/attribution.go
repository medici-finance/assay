package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// verifiedCellRe matches a dated runner in a Verified column: "YYYY-MM-DD <token...>".
var verifiedCellRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \S`)

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
// verification, and every session in this repo shares one git identity — a
// file-level check cannot see true cross-session independence. It catches
// the detectable cases (missing dated runner, a verifier token matching the
// author or reading as "the implementer"/"self", and Evidence rows that are
// all implementer-attributed) and nothing more (brief-16 scope-honesty note).
func attributionProblems(streams []*Stream) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

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

			if reason := selfVerificationReason(bf.Authored, row.Verified); reason != "" {
				add("%s: Verified runner looks like self-verification (%s)", label, reason)
			}

			if !evidenceHasIndependentRow(bf.Evidence) {
				add("%s: verified requires an independent (non-implementer) Evidence row", label)
			}
		}
	}
	sort.Strings(problems)
	return problems
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
		return fmt.Sprintf("verifier %q names the implementer, not an independent runner", verifierTok)
	}
	if verifierTok == "self" {
		return fmt.Sprintf("verifier %q is literally \"self\"", verifierTok)
	}
	return ""
}

// implementerAttributed reports whether a Runner cell names the implementer.
// "non-implementer" asserts the opposite and must not match — strip it before
// the substring test so `sonnet verifier (non-implementer)` reads as
// independent while `implementer (Opus 4.8)` still reads as implementer-run.
func implementerAttributed(runnerCell string) bool {
	return strings.Contains(strings.ReplaceAll(runnerCell, "non-implementer", ""), "implementer")
}

// evidenceHasIndependentRow reports whether an Evidence section contains at
// least one markdown table content row (outside HTML comments, beyond a
// header/separator pair) whose last (Runner) cell is not implementer-
// attributed. A section may hold more than one table (e.g. an
// "implementer run" table followed by an "independent re-run" table); each
// table's own header+separator pair is skipped in turn.
//
// Cells are split with splitRowEscaped, NOT the naive splitRow. A
// Command cell legitimately contains an escaped pipe (e.g.
// “ `grep -ciE "arm64\|amd64"` “); splitRow does not know about the escape
// and cuts the row there too, so a row that names a "Runner" column by index
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
	stripped := htmlCommentRe.ReplaceAllString(evidence, "")
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
