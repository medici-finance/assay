package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Command deskfile is the filing-gate desk tool. It
// encodes the filing-discipline ruling — dedupe BEFORE filing, attach
// instances to class issues, budget filings — as a binding gate rather than desk memory.
// An issue that should have been a comment on a class issue never gets minted.
//
// deskfile gates WHETHER and WHERE an issue is filed, never WHO: the
// caller's standing gh credential is the filing identity, unchanged. It NEVER mints an
// App token. It has NO merge/close/reopen/edit capability: the only mutating gh
// verbs it can emit are `issue create` (via `new`) and `issue comment` (via `attach`).
//
// Exit codes (deskkit contract): 0 success/noop · 3 disabled ·
// 4 rate-limited · 5 refused · 6 unverifiable. See deskkit/exitcodes.go.

const maxBodyBytes = 16 * 1024 // body cap (16 KiB)

// --- the blocker-evidence gate + correction capture (brief 07 of its tracking stream) ---
//
// Two behaviours land here, both the TOOL layer of a rule whose SKILL layer lives in the
// desk skills:
//
//   - Blocker-evidence gate: a `new` filing labelled with an escalation label (below) is a
//     blocker CLAIM. A blocker claim with nothing to quote is not a blocker claim, exactly
//     as an idle claim with no sweep is not an idle claim — so the filing REFUSES (exit 5)
//     unless its body carries an `### Evidence` heading followed by a fenced block. The two
//     layers fail on different signals in different components: a desk that skips the skill
//     clause still cannot file an evidence-less escalation here.
//   - Correction capture: `deskfile new --label skill-bug --correction "<msg>"` composes the
//     skill-bug body from this session's last receipt (brief 02 of its tracking stream). The desk detects
//     the correction (a model reads the message); the record's SHAPE and its
//     recent-receipt precondition are the tool's, so every capture reads the same.
const (
	// skillBugLabel is the label a correction-capture filing carries. --correction composes
	// the body for a filing so labelled; it is an ordinary user --label (deskfile never mints
	// it — the NOTICE prints the one-off create), so the repo must define it.
	skillBugLabel = "skill-bug"

	// skillBugReceiptWindow bounds how recently a receipt must have been recorded for
	// --correction to compose a skill-bug from it. A correction that follows no recent
	// receipt has nothing to correct, so the composition REFUSES (exit 5).
	skillBugReceiptWindow = 30 * time.Minute
)

// needsDecisionLabel names the standing decision-queue label, restated once so the fork-test
// gate (forktest.go) and the evidence gate below cannot drift on the
// literal.
const needsDecisionLabel = "needs-decision"

// escalationLabels are the labels whose `new` filings MUST carry evidence: such a filing is
// a blocker claim, and a blocker claim with nothing to quote is not a blocker claim. The gate
// is the tool half of the two-layer blocker-evidence rule. `human-only` is deliberately NOT
// here — it marks an ACT, not a claim (brief 05 of its tracking stream) — and the gate binds `new` only,
// so `attach` observations (not fresh claims) are unaffected.
var escalationLabels = map[string]bool{
	needsDecisionLabel: true,
	"help wanted":      true,
	"question":         true,
}

// escalationLabelIn returns the first label in labels that is an escalation label (matched
// case-insensitively, trimmed) and whether one was found.
func escalationLabelIn(labels []string) (string, bool) {
	for _, l := range labels {
		if escalationLabels[strings.ToLower(strings.TrimSpace(l))] {
			return strings.TrimSpace(l), true
		}
	}
	return "", false
}

// withoutLabel returns labels with every case-insensitive match of want dropped.
func withoutLabel(labels []string, want string) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if !strings.EqualFold(strings.TrimSpace(l), want) {
			out = append(out, l)
		}
	}
	return out
}

// hasLabel reports whether want is among labels (case-insensitive, trimmed).
func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}

// bodyHasEvidenceBlock reports whether body carries an `### Evidence` heading followed,
// anywhere after it, by a fenced code block (a line opening with ```). The fence is what
// makes a blocker claim re-runnable — the verbatim output of a command run this tick. A
// heading with no fence under it, or a fence with no heading before it, does not satisfy the
// gate.
func bodyHasEvidenceBlock(body string) bool {
	seenHeading := false
	for _, ln := range strings.Split(body, "\n") {
		t := strings.TrimSpace(ln)
		if isEvidenceHeading(t) {
			seenHeading = true
			continue
		}
		if seenHeading && strings.HasPrefix(t, "```") {
			return true
		}
	}
	return false
}

// isEvidenceHeading reports whether a trimmed line is a Markdown heading whose text begins
// with "Evidence" (any heading level, case-insensitive) — `### Evidence`, `## Evidence
// (this tick)`. The gate names `### Evidence`; accepting other levels avoids refusing a
// well-intentioned filing over a `#` count while still requiring the labelled section.
func isEvidenceHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	rest := strings.TrimSpace(strings.TrimLeft(line, "#"))
	return strings.HasPrefix(strings.ToLower(rest), "evidence")
}

// composeSkillBugTitle renders the deterministic skill-bug title from the section the desk
// was following. A title is required for the dedupe gate; keying it on the section groups
// repeat corrections about the same skill section toward the same issue.
func composeSkillBugTitle(section string) string {
	return "skill-bug: " + section
}

// composeSkillBugBody renders the skill-bug issue body from the last receipt and the desk's
// --correction / --section / --reading inputs. The body is composed by the TOOL, never by the
// desk (brief 07 of its tracking stream): the desk detects the correction, but the record's SHAPE — the
// five fields, in this order — is fixed here so every capture reads the same. The
// caller-controlled strings (the human's message and the desk's own text) are covered by the
// surface scan the caller runs on the composed body before filing.
func composeSkillBugBody(rec deskkit.AckRecord, loop, correction, section, reading string) string {
	var b strings.Builder
	b.WriteString("## Correction captured\n\n")
	b.WriteString("A human correction followed a desk receipt. The desk obeyed the correction; this " +
		"issue captures it as a skill-bug so the fix outlives the session.\n\n")
	b.WriteString("**Receipt:** " + rec.Line() + "\n\n")
	b.WriteString("**Correction (verbatim):**\n\n")
	for _, ln := range strings.Split(correction, "\n") {
		b.WriteString("> " + ln + "\n")
	}
	b.WriteString("\n**Desk loop:** " + loop + "\n\n")
	b.WriteString("**Skill / section followed:** " + section + "\n\n")
	b.WriteString("**What the skill should have said (desk's reading):** " + reading + "\n")
	return b.String()
}

// --- the raised-by provenance stamp -------------------------------------------------
//
// deskfile is the ONE choke point every gated filing passes through, so it is where the
// `raised-by:<role>` stamp belongs: a second stamping path would be a second place for the
// convention to be forgotten. The vocabulary and the reader contract are declared once, in
// deskkit/raisedby.go; nothing about the label is spelled out here.
//
// THE STAMP NEVER BLOCKS THE FILING. This is the deliberate half. `raised-by:` is a
// METRIC annotation, not a safety gate, and the labels do not exist in any repo yet — 0 of
// 421 issues on the home repo carry one at the time this shipped. A hard gate against an
// already-drifted corpus reds everything on day one and teaches the fleet to route around
// the verb; the fleet precedent is statusgen/mergedstatus.go, which shipped its
// reconciliation at NOTICE severity for exactly this reason and recorded promotion as a
// later ruling. So an unstampable filing is filed UNSTAMPED with a loud NOTICE, and the
// issue reads as UNKNOWN provenance — which is a true statement about it.
//
// What IS refused (exit 5) is a role the roster does not bind. That is a caller error with
// a fix in hand, not a state of the world, and stamping it would mint a metric category
// nothing else will ever populate.
//
// FOUR OUTCOMES, all audited, none silent (see resolveRaisedByStamp):
//
//	stamped              the label exists in the repo and was applied.
//	not-requested        no --raised-by was given. Provenance UNKNOWN by omission.
//	label-missing        the role is valid but the repo has no such label; deskfile never
//	                     mints one, and applying an unverified label could have failed
//	                     the whole filing.
//	could-not-check      the label-existence probe could not be answered (API error,
//	                     unparseable output). Three-state: not "absent", not "present".
//
// The last three all land the issue as UNKNOWN. They are kept DISTINCT in the audit line
// because they need different remedies — create the label, pass the flag, or investigate
// an outage — and collapsing them into one "unstamped" would hide which.
const (
	// raisedByFlag is the flag name, restated once so the NOTICE text and the usage
	// string cannot drift from the flag registration.
	raisedByFlag = "raised-by"
	// toFlag is the addressee flag name, restated once so its NOTICE text and usage
	// string cannot drift from the flag registration.
	toFlag = "to"

	stampOutcomeStamped   = "raised-by=%s"
	stampOutcomeOmitted   = "raised-by=UNSTAMPED:not-requested"
	stampOutcomeNoLabel   = "raised-by=UNSTAMPED:label-missing"
	stampOutcomeUnchecked = "raised-by=UNSTAMPED:could-not-check"
)

// --- the desk-inbox addressee stamp (`--to <role>` → `to:<role>`) -------------------
//
// `--to <role>` addresses a filing TO a desk, so the addressee's own sweep leads with it
// (fanoutloop/issueboard). It reuses the `--raised-by` role resolver (one resolver, two
// flags — deskkit.AddressedToLabel over the same bound vocabulary), and it degrades the
// SAME way the raised-by stamp does: an unbound role is REFUSED (exit 5), but a role that
// is valid where the repo simply has no `to:<role>` label yet files UNSTAMPED with a
// NOTICE carrying the one-off label-create command for the repo's FORGE (`gh label
// create` on GitHub, `glab label create` on GitLab — labelCreateHint), because applying an
// unverified label could fail the whole filing.
//
// ONE DIFFERENCE FROM raised-by, deliberate: omitting `--to` is the NORMAL, common case
// (most filings are not addressed to a desk), so it is SILENT — no NOTICE, just the
// UNADDRESSED audit token. Omitting `--raised-by`, by contrast, is a metric gap worth a
// NOTICE. The audit token is `to=<role>` when applied, else `to=UNADDRESSED:<reason>`.
const (
	toOutcomeAddressed = "to=%s"
	toOutcomeOmitted   = "to=UNADDRESSED:not-requested"
	toOutcomeNoLabel   = "to=UNADDRESSED:label-missing"
	toOutcomeUnchecked = "to=UNADDRESSED:could-not-check"
)

// --- per-session new-issue budget (NEW accounting, NOT the deskkit limiter) --
//
// The deskkit outward-write limiter (RateLimitPerPRPerHour etc.) has NO session dimension
// and NO 24h window, so it cannot express "3 new issues per session per rolling 24h". This
// budget is computed separately over the audit log's sessionTag+tool+verb+repo fields and
// counts ONLY successful (or sent-but-unconfirmed) `new` writes. Attach comments are NOT
// budgeted — directing filers to attach is the very motion the gate encourages, so it must
// never be the path the budget refuses.
//
// WHOSE session. The tag is deskkit.SessionTag(), which names the agent DOING THE FILING —
// $DESK_SESSION ahead of the harness's own session id, because a dispatched agent is a
// child process and inherits the harness id of the session that dispatched it. Keyed on the
// inherited id the budget would cover a whole FAN-OUT rather than an agent: the first agent
// to file three would exhaust every sibling's budget, and the others would be refused
// having filed nothing. The cap is per ACTOR — one agent still gets 3, and gets no more by
// being dispatched alongside others.
//
// The rate cannot be reset by varying the session id without a trace: every `new` audit
// line carries the sessionTag it charged (deskkit.SessionTag()), so a caller that rotates
// the env var to reset its bucket leaves a forensic trail of which sessions filed what.
// Rotating the ID does reset the bucket (a new session is a new session) — the audit trace
// is the control, not a hard block.
//
// assay#1204 reframed the cap from a per-session TALLY to a per-window RATE (N filings per
// window). The counting fields are UNCHANGED (session+tool+verb+repo over the audit log),
// so the anti-evasion property above is preserved verbatim; what changed is that the KNOB is
// the window, both are env-fixable (envNewRate / envNewWindow), and a `--force-file --reason`
// override raises the rate for one filing (see cmdNew) — while still recording a charged,
// session-tagged audit line, so the override can never erase the trail either.
const (
	// defaultNewRate is the shipped fallback for the per-session, per-repo cap on `new`
	// writes within one window: at most this many filings per defaultNewWindow. 3 is
	// enough for a productive session, low enough to stop a runaway filer. Overridable at
	// runtime with envNewRate.
	defaultNewRate = 3
	// defaultNewWindow is the shipped fallback for the rolling window the rate counts over.
	// Overridable at runtime with envNewWindow.
	defaultNewWindow = 24 * time.Hour

	// envNewRate / envNewWindow make the pace env-fixable with no recompile: an integer
	// rate and a Go time.ParseDuration string respectively. Unset → the shipped fallback,
	// silently. SET-but-unparseable → the shipped fallback AND a NOTICE to stderr naming the
	// bad value; an unparseable value must never silently DISABLE the cap. See newBudgetConfig.
	envNewRate   = "ASSAY_DESKFILE_NEW_RATE"
	envNewWindow = "ASSAY_DESKFILE_NEW_WINDOW"
)

// newBudgetConfig resolves the new-issue filing rate and window, reading envNewRate /
// envNewWindow with the shipped defaults (defaultNewRate / defaultNewWindow) as the
// fallback. An unset var takes the fallback silently. A SET var that does not parse to a
// POSITIVE value takes the fallback AND prints a NOTICE to stderr naming the bad value: the
// cap must never be silently disabled by a typo. It is resolved ONCE per invocation in
// cmdNew so the NOTICE prints at most once.
func newBudgetConfig() (rate int, window time.Duration) {
	rate, window = defaultNewRate, defaultNewWindow
	if v := strings.TrimSpace(os.Getenv(envNewRate)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rate = n
		} else {
			fmt.Fprintf(os.Stderr, "NOTICE: %s=%q is not a positive integer — using the shipped default rate of %d filings per window\n",
				envNewRate, v, defaultNewRate)
		}
	}
	if v := strings.TrimSpace(os.Getenv(envNewWindow)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			window = d
		} else {
			fmt.Fprintf(os.Stderr, "NOTICE: %s=%q is not a valid positive Go duration (e.g. 24h) — using the shipped default window of %s\n",
				envNewWindow, v, defaultNewWindow)
		}
	}
	return rate, window
}

// createSentMarker is stamped at the head of the audit detail of every `new` line whose
// `gh issue create` was ACTUALLY INVOKED. It is set on auditCtx immediately before the
// exec call and is therefore present on every outcome of that call — success, error, or a
// crash that still unwinds through the deferred finalize.
//
// It exists because "may have created an issue" is not a property of the audit RESULT
// alone. ResultUnverifiable is emitted both by a create whose outcome could not be
// confirmed (which must charge budget) and by pre-write failures — a dedupe search outage,
// an unreadable --body-file — which provably sent nothing and must not. Without a
// discriminator the budget charges both, and three search outages lock a session out for
// 24h having filed nothing. See chargedNewEntry.
const createSentMarker = "create-sent | "

// chargedNewEntry reports whether a deskfile `new` audit entry represents an issue that
// MAY HAVE BEEN CREATED, and so consumes session budget. It mirrors deskkit's private
// chargesBudget semantics, restated here because deskfile owns THIS budget and deskkit's
// is not exported:
//   - ResultOK charges: the issue was created.
//   - ResultUnverifiable charges ONLY IF the entry carries createSentMarker: the create
//     call was SENT and its outcome could not be confirmed, which is fail-open not to
//     charge against a flaky API. An Unverifiable WITHOUT the marker is a pre-write
//     failure — the create was never reached, no issue can exist, and charging it makes
//     the outage consume the budget that the escape hatch (--force-new --reason) needs in
//     order to be usable DURING that outage. That is not a fail-open loosening: the marker
//     is written by the same process that decided to call gh, before the call, so the only
//     entries it omits are ones this process can prove never reached the remote.
//   - ResultRefused/Noop/RateLimited/Disabled/DryRun do NOT charge: nothing reached the
//     remote, and counting RateLimited/Refused re-creates the livelock deskkit's design
//     exists to avoid (a budget refusal must not inflate the budget).
//   - Anything unclassified charges (fail closed).
//
// assay#955 asked whether a REFUSED `new` — the pre-write gates (BodyCheck's secret scan,
// the dedupe search finding a likely duplicate, or a self-containment-style refusal) —
// still consumes this budget. It does not: every one of those gates returns a
// *deskkit.DeskError with Code == ExitRefused, cmdNew's finalize maps that to
// ResultRefused (never reaching the createSent-marking line, since all of them run BEFORE
// checkSessionBudget in cmdNew's flow), and the ResultRefused case above excludes it from
// the count. The refusal is still logged — chargedNewEntry only decides what COUNTS, not
// what gets audited — so it is audited but free, per the ruling. See
// TestBudgetBodyCheckRefusalDoesNotConsumeSlot and TestBudgetDedupeRefusalDoesNotConsumeSlot
// for the end-to-end regression proof (three consecutive refusals, then a clean `new` that
// must still succeed).
func chargedNewEntry(e deskkit.Entry) bool {
	switch e.Result {
	case deskkit.ResultRefused, deskkit.ResultNoop,
		deskkit.ResultRateLimited, deskkit.ResultDisabled, deskkit.ResultDryRun:
		return false
	case deskkit.ResultUnverifiable:
		return strings.HasPrefix(e.Detail, createSentMarker)
	default: // ResultOK and anything unclassified (fail closed)
		return true
	}
}

// checkSessionBudget applies the per-window new-issue filing rate. It returns RateLimited
// (exit 4) when this session+repo has already charged `rate` `new` writes within `window`,
// Unverifiable (exit 6) on a corrupt/unreadable audit file (fail closed — corruption must
// not masquerade as an empty count), and nil when one more `new` is within the rate. The
// retry-after is the expiry of the oldest charged write in the window (ts + window + 1s), so
// a caller waking on it is certainly past the boundary. rate/window are resolved by
// newBudgetConfig (env-fixable, shipped defaults otherwise); the counting fields
// (session+tool+verb+repo) are unchanged, so the anti-evasion trail is preserved.
func checkSessionBudget(repo, session string, now time.Time, rate int, window time.Duration) error {
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return err // already an Unverifiable *DeskError (exit 6)
	}
	cutoff := now.Add(-window)
	var charged []time.Time
	for _, e := range entries {
		if e.Tool != "deskfile" || e.Verb != "new" {
			continue
		}
		if e.Repo != repo || e.SessionTag != session {
			continue
		}
		if !chargedNewEntry(e) {
			continue
		}
		ts, perr := time.Parse(time.RFC3339, e.TS)
		if perr != nil {
			return deskkit.Unverifiable(
				fmt.Sprintf("deskfile budget: audit entry has an unparseable ts %q — run `deskaudit recover` (quarantines the bad line and carries good entries forward; a plain move resets the budget + idempotency)", e.TS), perr)
		}
		if ts.Before(cutoff) {
			continue
		}
		charged = append(charged, ts)
	}
	if len(charged) < rate {
		return nil
	}
	// The oldest charged write's expiry is when the count drops to rate-1, admitting one
	// more. Sort oldest-first (stable on RFC3339 ts) to find it deterministically.
	sortTimesAscending(charged)
	freeAt := charged[0].Add(window).Add(time.Second)
	retryAfter := freeAt.Sub(now)
	if retryAfter <= 0 {
		retryAfter = time.Second
	}
	return deskkit.RateLimitedAfter(fmt.Sprintf(
		"refused: deskfile new-issue rate exhausted (%d `new` on %s within the last %s for session %q; "+
			"rate %d per %s) — retry-after: %ds (free at %s). Attach further observations to an existing "+
			"issue instead of filing new ones, or wait for the window to roll. This is YOUR agent's rate, not "+
			"the whole fan-out's. Raise the pace with %s / %s, or file one issue now with `--force-file "+
			"--reason <r>` (audit-logged, does not reset the count). DO NOT retry-loop by varying $DESK_SESSION "+
			"(or the harness session id): each `new` audit line records the sessionTag it charged, so rotating "+
			"the ID leaves a trail, it does not erase one.",
		len(charged), repo, window, session, rate, window,
		int(retryAfter/time.Second), freeAt.UTC().Format(time.RFC3339),
		envNewRate, envNewWindow),
		retryAfter)
}

func sortTimesAscending(ts []time.Time) {
	// Insertion sort — the slice is tiny (cap 3 in normal use, bounded by audit history).
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Before(ts[j-1]); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

// --- audit -------------------------------------------------------------------------

// auditCtx accumulates the fields for the ONE audit line every invocation emits
// ("audit every path"). finalize is deferred so exactly one line is written no matter which
// branch returns. Verb is set by each cmd; repo/title/bodyDigest/target/detail are filled
// in as the flow progresses so a refusal mid-flow still records what was attempted.
type auditCtx struct {
	verb            string
	repo            string
	title           string
	bodyDigest      string
	target          *int
	detail          string
	forceNewReason  string // non-empty when --force-new bypassed the dedupe search
	forceFileReason string // non-empty when --force-file raised the new-issue rate for this filing
	successResult   string // ResultOK unless a noop set it otherwise

	// raisedBy records WHICH of the four stamp outcomes this filing took (see the
	// raised-by block at the head of this file). It is APPENDED to the audit detail,
	// never prepended: chargedNewEntry discriminates on createSentMarker being the
	// PREFIX of Detail, so a note in front of it would silently un-charge the budget.
	//
	// It is on the audit line rather than only on stderr because "how many filings went
	// out unstamped, and why" is the question the backfill decision turns on, and a
	// NOTICE scrolls past.
	raisedBy string

	// addressedTo records WHICH `to:` outcome the filing took (applied / not-requested /
	// label-missing / could-not-check). Appended to the audit detail AFTER raisedBy, for
	// the same reason raisedBy is appended not prepended: chargedNewEntry keys on
	// createSentMarker being the PREFIX of Detail, so nothing may go in front of it.
	addressedTo string

	// lane records which route a `new` filing took through the fork-test gate —
	// `lane=desk-decided (<why>)`, `lane=needs-decision (<why>)`, or `no-fork=<value>` — so
	// "the tool took this off the human queue" is on the LOCAL audit trail, not only on the
	// forge. Appended after addressedTo, never prepended (see raisedBy for why).
	lane string

	// createSent is set immediately BEFORE the `gh issue create` exec and stamps
	// createSentMarker onto the audit detail. It is the discriminator the per-session
	// budget reads to tell "the create was sent and we cannot confirm it" (charges) from
	// "we never got as far as the create" (does not). See createSentMarker.
	createSent bool

	// readOnly marks a verb that performs NO outward write on ANY path, so every one of
	// its audit lines is logged as ResultDryRun regardless of outcome. Only `check`
	// qualifies, and it qualifies by CONSTRUCTION: cmdCheck builds no write argv at all
	// (proved by TestCheckDryRunNoWrites / TestCheckNeverCallsAMutatingVerb), which is
	// deskkit's stated precondition for ResultDryRun — "a path that provably performed no
	// outward write" (audit.go). For a dry-run FLAG the requirement is that the flag which
	// selects the result also suppresses the write; here the VERB does both.
	//
	// This is what keeps deskfile's own success case off deskkit's two meters. `check`
	// finding a duplicate is the verb WORKING, but it exits 5 and so logged `refused`,
	// which the breaker counts as non-progress: five correct dedupe hits opened a 15-minute
	// breaker against `attach` — the exact motion the refusal message tells the caller to
	// make. Symmetrically, a `check` whose search failed logged `unverifiable`, which
	// deskkit's chargesBudget counts, so a READ verb consumed deskfile's outward-WRITE
	// budget. ResultDryRun is invisible to both meters (it can neither trip the breaker nor
	// reset it, and charges nothing), which is exactly right for a verb that writes
	// nothing. The refusal reason is still on the line, in Detail.
	readOnly bool
}

func (a *auditCtx) log(result, detail string) {
	if a.createSent {
		detail = createSentMarker + detail
	}
	if a.raisedBy != "" {
		detail = strings.TrimSpace(detail + " | " + a.raisedBy)
	}
	if a.addressedTo != "" {
		detail = strings.TrimSpace(detail + " | " + a.addressedTo)
	}
	if a.lane != "" {
		detail = strings.TrimSpace(detail + " | " + deskkit.StripControl(a.lane))
	}
	e := deskkit.Entry{
		Tool:       "deskfile",
		Verb:       a.verb,
		Result:     result,
		Detail:     detail,
		Repo:       a.repo,
		PR:         a.target,
		HeadSHA:    nil, // deskfile has no head-pinning concept
		BodyDigest: a.bodyDigest,
		Title:      a.title,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	}
	_ = deskkit.Log(e)
}

// finalize maps the terminal error (or success) to exactly one audit result.
func (a *auditCtx) finalize(err error) {
	// A help screen is not an invocation of the verb, so it appends NO row. The ledger this
	// would land in is append-only, never rotated, and counted per tool for the write budget
	// and the circuit breaker (deskkit/audit.go, ratelimit.go) — see helprequest.go.
	if deskkit.IsHelpRequest(err) {
		return
	}
	// A read-only verb's every outcome is a dry run — see auditCtx.readOnly. The EXIT CODE
	// is unaffected (check still exits 5 on a duplicate, 6 on an unanswered search); only
	// the meters' view of the line changes, and they are write meters.
	if a.readOnly {
		detail := a.detail
		if err != nil {
			detail = err.Error()
		}
		a.log(deskkit.ResultDryRun, detail)
		return
	}
	if err == nil {
		result := a.successResult
		if result == "" {
			result = deskkit.ResultOK
		}
		a.log(result, a.detail)
		return
	}
	var result string
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitDisabled:
		result = deskkit.ResultDisabled
	case deskkit.ExitRateLimited:
		result = deskkit.ResultRateLimited
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}

// --- flags -------------------------------------------------------------------------

// stringSlice is a repeatable flag.Value (for --label).
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("empty value")
	}
	*s = append(*s, v)
	return nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // suppress flag's own output; we craft messages
	return fs
}

// --- verbs -------------------------------------------------------------------------

// cmdNew implements `deskfile new -R <repo> --title <t> --body-file <f> [--label ...]
// [--force-new --reason <r>] [--force-file --reason <r>]`. Flow: repo allowed → body+title
// scan → dedupe search (refuse exit 5 on a likely dup; fail closed exit 6 on a search
// API error unless --force-new) → new-issue rate (exit 4 over, env-fixable) → outward-write
// budget → `gh issue create` → audit. --force-new bypasses the dedupe search entirely; the
// DISTINCT --force-file raises the per-window rate for this one filing (without touching
// dedupe). Both require --reason and are audit-logged with it (the escape hatches for urgent
// filings during API outages / a spent rate); --force-file's line is still charged, so the
// override never resets or erases the rate count.
func cmdNew(args []string) (err error) {
	ac := &auditCtx{verb: "new"}
	defer func() { ac.finalize(err) }()

	fs := newFlagSet("deskfile new")
	repo := fs.String("R", "", "target repo, owner/name (required, must be in the desk-tools set)")
	title := fs.String("title", "", "issue title (required)")
	bodyFile := fs.String("body-file", "", "path to a file containing the issue body (required)")
	var labels stringSlice
	fs.Var(&labels, "label", "label to apply (repeatable)")
	raisedBy := fs.String(raisedByFlag, "", "desk role that RAISED this issue — stamps `raised-by:<role>` "+
		"(vocabulary derived from the roster's role-bindings; omitting it files with UNKNOWN provenance)")
	toRole := fs.String(toFlag, "", "desk role this issue is ADDRESSED TO — stamps `to:<role>` so that desk's "+
		"sweep leads with it (same role vocabulary as --"+raisedByFlag+"; omitting it is normal and silent). "+
		"NOTE: on `new` --to takes a ROLE; on `attach` --to takes an issue NUMBER")
	correction := fs.String("correction", "", "the human's correction message, VERBATIM — switches `new` into "+
		"skill-bug composition mode: the tool composes the title and body from this session's last receipt "+
		"(requires --label "+skillBugLabel+", --section and --reading; --title/--body-file are composed, not "+
		"passed; refuses if no receipt was recorded in the last "+skillBugReceiptWindow.String()+")")
	section := fs.String("section", "", "the skill + section the desk was following (composed into the skill-bug body; requires --correction)")
	reading := fs.String("reading", "", "the desk's one-line reading of what the skill should have said (composed into the skill-bug body; requires --correction)")
	forceNew := fs.Bool("force-new", false, "bypass the DEDUPE search AND the blocker-evidence gate (escape hatch; requires --reason)")
	forceFile := fs.Bool("force-file", false, "raise the new-issue RATE for this ONE filing so it files even when the "+
		"rate is spent (escape hatch; requires --reason). Distinct from --force-new, which bypasses dedupe; "+
		"--force-file does NOT weaken dedupe and does NOT reset the rate count (the filing is still audited and charged).")
	reason := fs.String("reason", "", "stated reason for --force-new / --force-file (required with either)")
	noFork := fs.String("no-fork", "", "re-routes a filing that turned out to have fewer than two workable options "+
		"(the `deskfile new` fork-test gate's refusal names this flag): one of "+noForkBriefContradicts+" | "+
		noForkWrongRepo+" | "+noForkToolFalsePositive+". Files WITHOUT the needs-decision label, and refuses a one-way item (deskkit.OneWay) — "+
		"that stays on the driver's queue")
	if perr := fs.Parse(args); perr != nil {
		// TIER TWO: `-h`/`--help` in any spelling reaches flag.Parse as flag.ErrHelp.
		// A help screen is not a refusal and writes no audit row — the finalizer
		// skips it (deskkit/helprequest.go).
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: unexpected extra arguments")
	}
	if strings.TrimSpace(*repo) == "" {
		return deskkit.Refused("refused: -R <repo> is required")
	}
	// Correction-capture (skill-bug) mode: the tool composes the title and body from this
	// session's last receipt, so --title/--body-file are refused here and --label skill-bug,
	// --section and --reading are required. Ordinary mode requires --title/--body-file and
	// forbids the correction-only flags.
	composeMode := strings.TrimSpace(*correction) != ""
	if composeMode {
		if strings.TrimSpace(*title) != "" {
			return deskkit.Refused("refused: --title is composed by --correction (skill-bug mode) — drop --title")
		}
		if strings.TrimSpace(*bodyFile) != "" {
			return deskkit.Refused("refused: --body-file is composed by --correction (skill-bug mode) — drop --body-file")
		}
		if !hasLabel(labels, skillBugLabel) {
			return deskkit.Refused("refused: --correction (skill-bug mode) requires --label " + skillBugLabel)
		}
		if strings.TrimSpace(*section) == "" {
			return deskkit.Refused("refused: --correction requires --section (the skill + section the desk was following)")
		}
		if strings.TrimSpace(*reading) == "" {
			return deskkit.Refused("refused: --correction requires --reading (the desk's one-line reading of what the skill should have said)")
		}
	} else {
		if strings.TrimSpace(*section) != "" || strings.TrimSpace(*reading) != "" {
			return deskkit.Refused("refused: --section/--reading are only valid with --correction (skill-bug composition mode)")
		}
		if strings.TrimSpace(*title) == "" {
			return deskkit.Refused("refused: --title is required")
		}
		if strings.TrimSpace(*bodyFile) == "" {
			return deskkit.Refused("refused: --body-file is required (no stdin/inline body)")
		}
	}
	if (*forceNew || *forceFile) && strings.TrimSpace(*reason) == "" {
		return deskkit.Refused("refused: --force-new/--force-file require a non-empty --reason (the escape hatch is audit-logged)")
	}
	if !deskkit.IsAllowedRepo(*repo) {
		return deskkit.Refused("refused: " + *repo + " is not in the desk-tools repo set")
	}

	// --no-fork re-routes a fork-test refusal: the filer re-runs a
	// filing that had fewer than two workable options with exactly one of the three closed
	// values, and the tool composes the shape each re-route requires — title prefix and
	// addressee for the two that name one, content requirements for all three — rather than
	// trusting free text to carry it. It NEVER coexists with --label needs-decision: the
	// whole point of the flag is filing WITHOUT that label.
	noForkVal := strings.TrimSpace(*noFork)
	if noForkVal != "" {
		switch noForkVal {
		case noForkBriefContradicts, noForkWrongRepo, noForkToolFalsePositive:
		default:
			return deskkit.Refused("refused: --no-fork must be one of " + noForkBriefContradicts + " | " +
				noForkWrongRepo + " | " + noForkToolFalsePositive + ", got " + noForkVal)
		}
		if hasLabel(labels, needsDecisionLabel) {
			return deskkit.Refused("refused: --no-fork files WITHOUT the " + needsDecisionLabel +
				" label — drop --label " + needsDecisionLabel)
		}
		var requiredPrefix, requiredTo string
		switch noForkVal {
		case noForkBriefContradicts:
			requiredPrefix, requiredTo = "amend brief:", "desk"
		case noForkWrongRepo:
			// "worker" is the ROSTER's role name for the worker-desk window (the vocabulary
			// --to shares with --raised-by, RaisedByRoles) — NOT the skill file name
			// "worker-desk", which boundRole would refuse as unbound.
			requiredPrefix, requiredTo = "re-dispatch:", "worker"
		}
		if requiredTo != "" {
			if strings.TrimSpace(*toRole) != "" && !strings.EqualFold(strings.TrimSpace(*toRole), requiredTo) {
				return deskkit.Refused(fmt.Sprintf(
					"refused: --no-fork %s addresses the filing --to %s — drop --to or pass --to %s",
					noForkVal, requiredTo, requiredTo))
			}
			*toRole = requiredTo
		}
		if requiredPrefix != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(*title)), strings.ToLower(requiredPrefix)) {
			*title = requiredPrefix + " " + strings.TrimSpace(*title)
		}
		if noForkVal == noForkToolFalsePositive && !hasLabel(labels, "bug") {
			labels = append(labels, "bug")
		}
	}
	ac.repo = *repo
	ac.title = *title

	// Resolve the forge that serves this repo under the session-role App's custody (write-verbs-C).
	// ForgeFor RETAINS the could-not-check refusal on an unresolvable forge (no ASSAY_REPO_FORGES
	// entry and an absent/unmapped origin), and now SERVES GitLab through the backend — the #691
	// interim named-refusal is superseded. Minting the token here is the identity change the #781
	// ruling confirmed; --raised-by stays a body/label attribution below.
	fg, fr, kind, ferr := forgeForFn(*repo, false)
	if ferr != nil {
		return ferr
	}

	// Validate the raised-by ROLE before anything is written. An unbound role is a
	// caller error with a fix in hand (exit 5), and it is the one raised-by condition
	// that refuses: everything else about the stamp degrades to UNKNOWN rather than
	// blocking the filing. Resolution of whether the LABEL exists happens later, after
	// the dedupe and budget gates, so a filing that was going to be refused anyway does
	// not spend an API call proving it.
	stampLabel := ""
	if strings.TrimSpace(*raisedBy) != "" {
		l, lerr := deskkit.RaisedByLabel(*raisedBy)
		if lerr != nil {
			return lerr
		}
		stampLabel = l
	}

	// Validate the --to ROLE the same way and for the same reason: an unbound addressee
	// is a caller error with a fix in hand (exit 5, bound set named), refused BEFORE any
	// write. Whether the `to:<role>` LABEL exists on the repo is resolved later, after the
	// dedupe/budget gates, so a filing that would be refused anyway spends no API call.
	toLabel := ""
	if strings.TrimSpace(*toRole) != "" {
		l, lerr := deskkit.AddressedToLabel(*toRole)
		if lerr != nil {
			return lerr
		}
		toLabel = l
	}

	// Body: composed by the tool in skill-bug mode, else read from --body-file (file only,
	// 16 KiB cap). Either way the composed/read body and the title are surface-scanned before
	// anything is written.
	var body []byte
	if composeMode {
		// The record's precondition is the tool's (brief 07 of its tracking stream): read this session's
		// last receipt within the window. A corrupt/unreadable beacon is could-not-check
		// (exit 6, propagated); a beacon with no recent receipt is a refusal (exit 5) — a
		// correction with nothing to correct is not a skill-bug.
		sess := deskkit.SessionTag()
		rec, aerr := deskkit.LastAckWithin(sess, skillBugReceiptWindow, time.Now())
		if aerr != nil {
			return aerr
		}
		if rec == nil {
			return deskkit.Refused(fmt.Sprintf(
				"refused: no receipt recorded in the last %s for session %q — a correction with nothing to "+
					"correct is not a skill-bug. Print a receipt with `deskack \"<your one-line reading>\"` on "+
					"the human message BEFORE composing the skill-bug.",
				skillBugReceiptWindow, sess))
		}
		loop := strings.TrimSpace(os.Getenv("DESK_LOOP"))
		*title = composeSkillBugTitle(*section)
		ac.title = *title
		body = []byte(composeSkillBugBody(*rec, loop, *correction, *section, *reading))
	} else {
		b, berr := readBody(*bodyFile)
		if berr != nil {
			return berr
		}
		body = b
	}

	// --no-fork content requirements: each re-route names what its body must carry, checked
	// against the shape rather than trusting free text. FIRST, the one-way check: every
	// --no-fork value files WITHOUT needs-decision, so a one-way item (a one-way caller label,
	// or a one-way term anywhere in title+body — deskkit.OneWay) is refused here rather than
	// steered off the driver's queue. The only way forward for it is a needs-decision filing.
	if noForkVal != "" {
		ac.lane = "no-fork=" + noForkVal
		if hit, oneWay := deskkit.OneWay(*title, oneWayHay(string(body)), labels); oneWay {
			return deskkit.Refused("refused: --no-fork files WITHOUT the " + needsDecisionLabel + " label, and this " +
				"item is one-way — " + hit.String() + ". A one-way item stays on the driver's queue: file it " +
				"with --label " + needsDecisionLabel + " (and a `### Fork test` block), or, if it genuinely has one " +
				"workable option, with --label " + needsDecisionLabel + " --force-new --reason \"<why>\".")
		}
		switch noForkVal {
		case noForkWrongRepo:
			if !repoShapeRe.Match(body) {
				return deskkit.Refused("refused: --no-fork wrong-repo requires the body to name the repo the " +
					"work belongs in (an `owner/repo` token)")
			}
		case noForkBriefContradicts:
			if !briefIDShapeRe.Match(body) {
				return deskkit.Refused("refused: --no-fork brief-contradicts-artifact requires the body to name " +
					"the brief id it amends (a `<stream>/<NN>` or `assay:at:<stream>:<NN>` token)")
			}
			if !artifactPathShapeRe.Match(body) {
				return deskkit.Refused("refused: --no-fork brief-contradicts-artifact requires the body to name " +
					"the artifact it contradicts (a backtick-quoted path with an extension)")
			}
		case noForkToolFalsePositive:
			if !bodyHasFence(string(body)) {
				return deskkit.Refused("refused: --no-fork tool-false-positive requires the body to carry the " +
					"tool's refusal text in a fenced block")
			}
		}
	}

	// The fork-test gate: a filing labelled needs-decision must carry a well-formed
	// `### Fork test` block naming the workable options, the default, the gate that catches
	// a wrong guess, and the search that showed the question was not already ruled. Runs
	// BEFORE dedupe, and before the blocker-evidence gate below so the evidence requirement
	// (an escalation-label property) sees the label set. --no-fork and this gate are mutually
	// exclusive (enforced above: --no-fork refuses if --label needs-decision is also given),
	// so a --no-fork filing never reaches this block. --force-new bypasses it as it bypasses
	// dedupe and the evidence gate — the audited escape hatch every refusal in this tool
	// shares, and one that FILES AS needs-decision, so it never takes an item off the queue.
	//
	// Every route off the driver's queue this gate offers FAILS CLOSED (deskkit/noticelane.go):
	// the fewer-than-two-options refusal names the --no-fork re-routes only for an item that
	// is not one-way, and the notice lane admits only a positive R-3 reversible signal with no
	// one-way term and no one-way caller label.
	deskDecidedApply := false
	if !*forceNew && noForkVal == "" && hasLabel(labels, needsDecisionLabel) {
		res := parseForkTest(string(body))
		if !res.Structural() {
			return deskkit.Refused(forkTestErrorMessage(res))
		}
		if !res.Workable() {
			if hit, oneWay := deskkit.OneWay(*title, oneWayHay(string(body)), labels); oneWay {
				return deskkit.Refused(forkTestOneWayMessage(res, hit))
			}
			return deskkit.Refused(forkTestRerouteMessage(res))
		}
		def := res.DefaultOption() // non-nil: Structural() already proved it names a counted option
		ac.lane = "lane=" + needsDecisionLabel
		if res.CaughtByKind == caughtByNothing {
			// No gate catches a wrong guess, so this is a genuine decision: stays
			// needs-decision, filed as today.
			ac.lane += " (caught-by: nothing)"
		} else if admit, why := deskkit.NoticeLaneVerdict(*title, oneWayHay(string(body)), labels); admit {
			// Notice lane (R-3): two-plus workable options, a gate the driver still holds, a
			// positive reversible signal and no one-way term. desk-decided is ADDED alongside
			// needs-decision at label time, and needs-decision comes off only in a second
			// write (see the label block below), so a failed write never leaves the filing on
			// neither label. The appended block carries the shared marker, so the filing IS
			// the desk's R-3 act — no separate comment for deskdigest to wait for.
			body = append(body, []byte(renderDeskDecidedBlock(*def, res.CountedOptions(), res.CaughtByKind, res.CaughtByDetail))...)
			deskDecidedApply = true
			ac.lane = "lane=" + deskDecidedLabel + " (" + why + ")"
		} else {
			// One-way, or nothing positive to go on: stays needs-decision, filed as today.
			ac.lane += " (" + why + ")"
		}
	}

	if serr := deskkit.ScanSurface("issue body", body); serr != nil {
		return serr
	}
	if serr := deskkit.ScanSurface("issue title", []byte(*title)); serr != nil {
		return serr
	}
	ac.bodyDigest = deskkit.Sha256Hex(body)

	// Blocker-evidence gate (brief 07 of its tracking stream): a `new` filing labelled with an escalation
	// label is a blocker CLAIM, and a blocker claim with nothing to quote is not a blocker
	// claim. Refuse (exit 5) unless the body carries an `### Evidence` heading followed by a
	// fenced block. --force-new --reason bypasses it as it bypasses dedupe — audited — for the
	// case where the evidence genuinely cannot be produced. human-only is not in the set (an
	// act, not a claim); attach is a separate verb and unaffected. A notice-lane filing is
	// judged WITHOUT needs-decision: it is filed as a desk decision taken, not a blocker
	// claim (needs-decision stays on its label list only so the label writes can be
	// add-first — see the notice lane above).
	evidenceLabels := labels
	if deskDecidedApply {
		evidenceLabels = withoutLabel(labels, needsDecisionLabel)
	}
	if !*forceNew {
		if lbl, ok := escalationLabelIn(evidenceLabels); ok && !bodyHasEvidenceBlock(string(body)) {
			return deskkit.Refused(fmt.Sprintf(
				"refused: a filing labelled %q is a blocker claim and must carry a `### Evidence` section with "+
					"at least one fenced block that is the verbatim output of a command run this tick (its "+
					"command line as the first line of the fence). None was found. A blocker claim with nothing "+
					"to quote is not a blocker claim: re-check first — if the re-check produces a fence, file "+
					"with it; if it produces a success, proceed. Override with --force-new --reason only if the "+
					"evidence genuinely cannot be produced (audited).", lbl))
		}
	}

	// Dedupe gate. --force-new bypasses the search entirely (the operator vouches the
	// title is unique; the reason is audit-logged). Otherwise a search API failure fails
	// CLOSED (exit 6): minting a possibly-duplicate issue is the expensive direction, and
	// `check`-style certainty about absence of duplicates cannot be bought with a guess.
	if !*forceNew {
		cands, serr := dedupeSearch(fg, fr, *title)
		if errors.Is(serr, errNoScorableTokens) {
			// Not an outage: this title can never match anything, so the gate cannot run
			// on it at all. Refuse (exit 5) with the fix in hand rather than pass a
			// guaranteed-empty dedupe off as a clean one.
			return deskkit.Refused(
				"refused: --title normalises to no scorable tokens (only stopwords, single characters " +
					"or punctuation), so the dedupe matcher cannot compare it against anything and would " +
					"pass it unconditionally. Give the issue a title with at least one substantive word.")
		}
		if serr != nil {
			return deskkit.Unverifiable(
				"dedupe search failed — refuse rather than mint a possible duplicate (override with --force-new --reason)", serr)
		}
		if m := matchesAbove(cands); len(m) > 0 {
			top := m[0]
			// deskkit.DedupeRefusalPrefix is the ONE exit-5 text a caller (deskboot's alarm) may
			// read as "already filed"; formatting from it keeps that match honest.
			return deskkit.Refused(fmt.Sprintf(
				deskkit.DedupeRefusalPrefix+"%d %q (score %.2f%s). Attach your observation there instead:\n"+
					"  deskfile attach -R %s --to %d --body-file <f>\n"+
					"Candidates at/above threshold %.2f:\n%s"+
					"Override with --force-new --reason only if you can justify why this is not a duplicate.",
				top.Number, top.Title, top.Score, classMarker(top.HasClassLabel),
				*repo, top.Number, matchThreshold, formatCandidates(m)))
		}
	} else {
		ac.forceNewReason = *reason
	}

	// Per-window new-issue filing rate (this tool's own accounting; see checkSessionBudget).
	// The rate/window are env-fixable (newBudgetConfig, resolved once so its NOTICE prints at
	// most once). --force-file raises the rate for THIS one filing: it skips the gate but is
	// still charged below, so it never resets or erases the count. It is audit-logged with its
	// reason and the filing session so the override leaves a trail (anti-evasion is preserved).
	rate, window := newBudgetConfig()
	if *forceFile {
		ac.forceFileReason = *reason
	} else {
		if berr := checkSessionBudget(*repo, deskkit.SessionTag(), time.Now(), rate, window); berr != nil {
			return berr
		}
	}

	// Standard outward-write budget. `new` creates a target whose number is not
	// known in advance, so AllowWriteRepoWide is the scope whose bucket its writes land in
	// (the same reasoning as deskpr create — see deskkit.AllowWriteRepoWide).
	if werr := deskkit.AllowWriteRepoWide("deskfile", *repo); werr != nil {
		return werr
	}

	// Resolve the provenance stamp. This NEVER returns an error: every way it can fail
	// yields an unstamped filing plus a NOTICE, because the stamp is a metric annotation
	// and a metric must not be able to stop a filing. See the raised-by block above.
	stampApply, stampNote, stampNotice := resolveRaisedByStamp(fg, fr, kind, stampLabel)
	ac.raisedBy = stampNote
	if stampNotice != "" {
		fmt.Fprintln(os.Stderr, stampNotice)
	}

	// Resolve the addressee stamp the same way. Like resolveRaisedByStamp it NEVER errors:
	// an unappliable `to:` label degrades to UNADDRESSED + a NOTICE rather than blocking
	// the filing.
	toApply, toNote, toNotice := resolveAddressedToStamp(fg, fr, kind, toLabel)
	ac.addressedTo = toNote
	if toNotice != "" {
		fmt.Fprintln(os.Stderr, toNotice)
	}

	// User --label labels are pre-checked for existence BEFORE the filing: applying a label
	// that does not exist would, on the old `gh issue create --label` path, fail the whole
	// create. deskfile's mutating vocabulary is issue create + issue comment + the label
	// RECONCILE (never a label CREATE for a metric), so a missing user label is a refusal here,
	// not a silent mint — and the refusal comes before FileIssue so no orphan issue is left.
	applyLabels := make([]deskkit.LabelSpec, 0, len(labels)+2)
	for _, l := range labels {
		present, perr := labelExists(fg, fr, l)
		if perr != nil {
			return deskkit.Unverifiable("cannot check whether label "+l+" exists on "+*repo, perr)
		}
		if !present {
			return deskkit.Refused("refused: label " + l + " does not exist on " + *repo +
				" — create it once (a human/label action) or drop --label " + l + "; deskfile never mints a label")
		}
		applyLabels = append(applyLabels, deskkit.LabelSpec{Name: l})
	}
	if stampApply != "" {
		applyLabels = append(applyLabels, deskkit.LabelSpec{Name: stampApply})
	}
	if toApply != "" {
		applyLabels = append(applyLabels, deskkit.LabelSpec{Name: toApply})
	}
	if deskDecidedApply {
		// UNLIKE the user labels above, desk-decided is never pre-checked-and-refused: the
		// tool applies it itself, so it goes through the SAME ensure-exists path
		// deskflip/deskpost's mechanical labels use (ApplyLabels creates a missing label on
		// first use, with this colour and description) rather than requiring a human/admin
		// label-create step before the first notice-lane filing can succeed. needs-decision is
		// still in applyLabels here: it comes off in a SECOND write, after this one lands.
		applyLabels = append(applyLabels, deskkit.LabelSpec{Name: deskDecidedLabel, Color: deskDecidedColor,
			Description: "filed on the R-3 notice lane — see the weekly decision digest for its veto date"})
	}

	// On-behalf-of trailer (multi-principal/01), appended to the filed body only — every
	// gate above (dedupe/title checks, the secret/self-contain scans) already ran against
	// the caller-supplied body.
	fileBody, oerr := deskkit.AppendOnBehalfOf(body, "", *repo)
	if oerr != nil {
		return oerr
	}

	// From here on the create HAS been sent, so every outcome charges session budget —
	// including an unconfirmable one. Set before the call, not after: an error return must
	// carry the marker too. See createSentMarker.
	ac.createSent = true
	ref, cerr := fg.FileIssue(fr, deskkit.IssueInput{Title: *title, Body: string(fileBody)})
	if cerr != nil {
		return deskkit.Unverifiable("file issue failed", cerr)
	}
	n := ref.Number
	ac.target = &n
	// Apply the resolved labels. Every caller/stamp/to label in applyLabels was confirmed to
	// EXIST above (user labels refuse if missing; stamp/to labels resolve to "" if missing), so
	// ApplyLabels' ensure step no-ops for them (the create returns already-exists). The ONE
	// label this write may mint is the tool's own desk-decided, on a notice-lane filing's
	// first use.
	// The target is the ISSUE just filed, stated explicitly: on GitLab the same number also
	// names an unrelated merge request, and a write that left the kind implicit stamped that
	// MR and left the issue unaddressed (no to:<role>, no dedupe key).
	if len(applyLabels) > 0 {
		change := deskkit.LabelChange{Target: deskkit.TargetIssue, Add: applyLabels}
		if _, lerr := fg.ApplyLabels(fr, ref.Number, change); lerr != nil {
			return deskkit.Unverifiable("apply labels to the filed issue failed", lerr)
		}
	}
	// Notice lane, second write: only now that desk-decided is ON the issue does
	// needs-decision come off. Add-first ordering is the fail-closed one — a failure here
	// leaves the issue on BOTH labels (still on the driver's queue, exit 6), never on neither.
	if deskDecidedApply {
		change := deskkit.LabelChange{Target: deskkit.TargetIssue, Remove: []string{needsDecisionLabel}}
		if _, lerr := fg.ApplyLabels(fr, ref.Number, change); lerr != nil {
			return deskkit.Unverifiable("the filing is labelled "+deskDecidedLabel+" AND still "+needsDecisionLabel+
				" (it stays on the driver's queue): removing "+needsDecisionLabel+" failed", lerr)
		}
	}
	url := deskkit.StripControl(ref.URL)
	// Record any override(s) ahead of the created-URL, so the audit line names WHICH escape
	// hatch was used, its reason, and (for --force-file) the identity that raised the rate.
	// The SessionTag field on the entry also carries that identity; naming it inline makes the
	// override self-describing in the detail too. chargedNewEntry keys on createSentMarker
	// being the PREFIX of the FINAL detail (added by log()), so these lead the string but not
	// the whole line — the override still CHARGES the rate, it does not un-charge it.
	//
	// The caller-controlled strings that land in Detail (the --reason and the SessionTag) are
	// StripControl'd the same way the URL and Title are: they must not carry control bytes that
	// could corrupt or forge the audit line they are appended to.
	var parts []string
	if ac.forceNewReason != "" {
		parts = append(parts, "force-new: "+deskkit.StripControl(ac.forceNewReason))
	}
	if ac.forceFileReason != "" {
		parts = append(parts, "force-file (rate override) by "+deskkit.StripControl(deskkit.SessionTag())+": "+deskkit.StripControl(ac.forceFileReason))
	}
	parts = append(parts, "created "+url)
	// When the rate/window were RAISED (or otherwise changed) from the shipped defaults by the
	// env knobs, the effective values are APPENDED to the audit Detail. Without this an entry
	// filed under ASSAY_DESKFILE_NEW_RATE=100 is byte-identical to one filed under the shipped 3,
	// so the env path would launder over-filing as ordinary activity and defeat the anti-evasion
	// property that IS the control. Appended (never prepended): chargedNewEntry keys on
	// createSentMarker being the PREFIX of Detail, so nothing may go in front of it.
	if rate != defaultNewRate || window != defaultNewWindow {
		parts = append(parts, fmt.Sprintf("rate-config: %d per %s (env)", rate, window))
	}
	ac.detail = strings.Join(parts, " | ")
	fmt.Println(url)
	return nil
}

// cmdAttach implements `deskfile attach -R <repo> --to <N> --body-file <f> [--kind issue|mr]`.
// Posts the observation as a comment on issue N (a class issue or a duplicate target). Never
// budgeted (attach is the motion the gate encourages). Refuses (exit 5) if N is CLOSED
// with the reopen-or-new guidance. Flow: repo allowed → body scan → verify target
// OPEN (fail closed exit 6 on an API error; refuse exit 5 if closed) → outward-write
// budget → `gh issue comment` → audit.
//
// --kind states WHICH object N names. It exists for GitLab, which numbers issues and merge
// requests in SEPARATE sequences: `#4` and `!4` routinely both exist, and the bare-number
// read (GetIssue) refuses that case rather than pick one — so without a stated kind every
// low number an adopter's project carries in both sequences was un-attachable. The default
// is `issue`, because attach is by definition an observation on an issue; `mr` (alias
// `pr`) is for the rarer observation on a change. The kind drives BOTH the target read and
// the note's endpoint (GetIssueTyped / PostCommentTyped), so the state check and the write
// address the same object. On GitHub the kind is validated against what N is, nothing more.
func cmdAttach(args []string) (err error) {
	ac := &auditCtx{verb: "attach"}
	defer func() { ac.finalize(err) }()

	fs := newFlagSet("deskfile attach")
	repo := fs.String("R", "", "target repo, owner/name (required, must be in the desk-tools set)")
	to := fs.Int("to", 0, "target issue number (required)")
	bodyFile := fs.String("body-file", "", "path to a file containing the comment body (required)")
	kindFlag := fs.String("kind", "issue", "which object --to names: issue (default) or mr (pr is an alias)")
	if perr := fs.Parse(args); perr != nil {
		// TIER TWO: `-h`/`--help` in any spelling reaches flag.Parse as flag.ErrHelp.
		// A help screen is not a refusal and writes no audit row — the finalizer
		// skips it (deskkit/helprequest.go).
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	kind, kerr := deskkit.ParseTargetKind(*kindFlag)
	if kerr != nil {
		return kerr
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: unexpected extra arguments")
	}
	if strings.TrimSpace(*repo) == "" {
		return deskkit.Refused("refused: -R <repo> is required")
	}
	if *to <= 0 {
		return deskkit.Refused(fmt.Sprintf("refused: --to <N> must be a positive issue number, got %d", *to))
	}
	if strings.TrimSpace(*bodyFile) == "" {
		return deskkit.Refused("refused: --body-file is required (no stdin/inline body)")
	}
	if !deskkit.IsAllowedRepo(*repo) {
		return deskkit.Refused("refused: " + *repo + " is not in the desk-tools repo set")
	}
	ac.repo = *repo
	target := *to
	ac.target = &target

	// Resolve the forge under the session-role App's custody (write-verbs-C). Retains the
	// could-not-check refusal on an unresolvable forge; serves GitLab (the #691 refusal is
	// superseded). attach WRITES a comment, so it takes the ordinary (rotating) mint.
	fg, fr, _, ferr := forgeForFn(*repo, false)
	if ferr != nil {
		return ferr
	}

	body, berr := readBody(*bodyFile)
	if berr != nil {
		return berr
	}
	if serr := deskkit.ScanSurface("comment body", body); serr != nil {
		return serr
	}
	ac.bodyDigest = deskkit.Sha256Hex(body)

	// Verify the target is OPEN before posting. An API/parse failure is unverifiable
	// (exit 6); a non-OPEN target is refused (exit 5) with reopen-or-new guidance.
	view, verr := viewIssue(fg, fr, target, kind)
	if verr != nil {
		return deskkit.Unverifiable("cannot read issue state — refuse rather than guess", verr)
	}
	if !strings.EqualFold(view.State, "OPEN") {
		return deskkit.Refused(fmt.Sprintf(
			"refused: issue #%d is %s, not OPEN — reopen it first (a human action) or file a new issue via "+
				"`deskfile new -R %s --title ... --body-file ...`. Target: %s",
			target, view.State, *repo, view.URL))
	}

	// Outward-write budget. Attach is NOT subject to the per-session new-issue
	// budget (this tool's accounting counts `new` only), only the standard outward-write
	// gate. The target number is known, so the per-issue scope is correct.
	if werr := deskkit.AllowWrite("deskfile", *repo, target); werr != nil {
		return werr
	}

	// On-behalf-of trailer (multi-principal/01), appended to the posted body only —
	// ac.bodyDigest above (audit-only here; attach has no body-keyed idempotency gate)
	// stays keyed on the caller-supplied body.
	attachBody, oerr := deskkit.AppendOnBehalfOf(body, "", *repo)
	if oerr != nil {
		return oerr
	}
	ref, cerr := fg.PostCommentTyped(fr, target, kind, string(attachBody))
	if cerr != nil {
		return deskkit.Unverifiable("post comment failed", cerr)
	}
	// A forge whose note reference carries no page URL (a GitLab issue note reports only its
	// numeric id) still gets the TARGET's URL printed, so the caller can find what it wrote.
	url := deskkit.StripControl(ref.URL)
	if url == "" {
		url = view.URL
	}
	ac.detail = "commented " + url + " kind=" + string(kind)
	fmt.Println(url)
	return nil
}

// cmdCheck implements `deskfile check -R <repo> --title <t>`: a dry-run dedupe. Prints the
// candidates and exits 0/5 the same as `new` would, writing nothing. This is the verb
// skills embed in authoring loops (try `check` before composing a `new`). A search API
// failure fails closed (exit 6) — `check` cannot promise "no duplicate" on an unanswered
// search. check is a READ (no outward write), so it is not rate-limited and consumes no
// budget; it still takes the audit line.
func cmdCheck(args []string) (err error) {
	// readOnly: check writes NOTHING on any path, so all its audit lines are ResultDryRun
	// and it feeds neither of deskkit's write meters. See auditCtx.readOnly.
	ac := &auditCtx{verb: "check", readOnly: true}
	defer func() { ac.finalize(err) }()

	fs := newFlagSet("deskfile check")
	repo := fs.String("R", "", "target repo, owner/name (required, must be in the desk-tools set)")
	title := fs.String("title", "", "title to check (required)")
	if perr := fs.Parse(args); perr != nil {
		// TIER TWO: `-h`/`--help` in any spelling reaches flag.Parse as flag.ErrHelp.
		// A help screen is not a refusal and writes no audit row — the finalizer
		// skips it (deskkit/helprequest.go).
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: unexpected extra arguments")
	}
	if strings.TrimSpace(*repo) == "" {
		return deskkit.Refused("refused: -R <repo> is required")
	}
	if strings.TrimSpace(*title) == "" {
		return deskkit.Refused("refused: --title is required")
	}
	if !deskkit.IsAllowedRepo(*repo) {
		return deskkit.Refused("refused: " + *repo + " is not in the desk-tools repo set")
	}
	ac.repo = *repo
	ac.title = *title

	// Resolve the forge under the session-role App's custody (write-verbs-C). check is a READ:
	// it reaches the forge to run the dedupe search, but it files nothing. So it asks for
	// READ-ONLY custody (--no-rotate) rather than the ordinary mint. On the GitLab custody
	// path the ordinary mint is a destructive self-rotation, and a window running several
	// checks in parallel raced its own rotations — the loser got 401 invalid_token and the
	// custody file could be left holding a dead value only a group owner can replace. A verb
	// that writes nothing has no business spending a credential rotation. The
	// unresolvable-forge could-not-check refusal is retained, GitLab is served (#691 superseded).
	fg, fr, _, ferr := forgeForFn(*repo, true)
	if ferr != nil {
		return ferr
	}

	cands, serr := dedupeSearch(fg, fr, *title)
	if errors.Is(serr, errNoScorableTokens) {
		return deskkit.Refused(
			"refused: --title normalises to no scorable tokens (only stopwords, single characters " +
				"or punctuation), so the dedupe matcher cannot compare it against anything and would " +
				"pass it unconditionally. Give the issue a title with at least one substantive word.")
	}
	if serr != nil {
		return deskkit.Unverifiable("dedupe search failed — cannot confirm absence of duplicates", serr)
	}
	if m := matchesAbove(cands); len(m) > 0 {
		top := m[0]
		fmt.Printf("likely duplicate of #%d %q (score %.2f%s)\nattach with:\n  deskfile attach -R %s --to %d --body-file <f>\n",
			top.Number, top.Title, top.Score, classMarker(top.HasClassLabel), *repo, top.Number)
		return deskkit.Refused(fmt.Sprintf("refused: likely duplicate of #%d %q (score %.2f%s)",
			top.Number, top.Title, top.Score, classMarker(top.HasClassLabel)))
	}
	fmt.Printf("no duplicates above threshold %.2f (scored %d candidate(s), max score %.2f)\n",
		matchThreshold, len(cands), topScore(cands))
	return nil
}

// --- the raised-by stamp resolver ---------------------------------------------------

// resolveRaisedByStamp decides whether the provenance label can actually be applied, and
// returns (label-to-apply, audit note, NOTICE for stderr). It NEVER returns an error:
// see the raised-by block at the head of this file for why a metric annotation must not
// be able to refuse a filing.
//
// stampLabel is "" when no --raised-by was given; it has already been validated against
// the roster by the caller when it is not.
//
// The label-existence probe is not decoration. On the original `gh issue create --label
// <x>` path the create FAILED outright when x did not exist on the repo, so applying an
// unverified stamp would convert a missing metric label into a failed filing — the
// annotation taking down the thing it annotates. The forge-backend path keeps that posture
// rather than relying on any backend's create-on-the-fly behaviour. And no repo has these
// labels by default: they must be created outside this tool (deskfile's mutating vocabulary
// is `issue create` and `issue comment`, and widening it to `label create` for a metric is
// not a trade this file makes). So the probe reads, and a missing label produces a NOTICE
// naming the exact create command FOR THE REPO'S FORGE (labelCreateHint) — a `gh` command
// printed on a GitLab repo cannot be run there, so the label never gets created and every
// later filing stays UNSTAMPED (#887 item 2).
//
// THREE-STATE on the probe itself: present / absent / could-not-ask. An unanswered probe
// is NOT treated as "absent" in the message even though both drop the stamp, because the
// remedies differ and a caller told "create the label" during an API outage will create a
// label that already exists and still not be stamped.
func resolveRaisedByStamp(fg deskkit.Forge, fr deskkit.ForgeRepo, kind deskkit.ForgeKind, stampLabel string) (apply, note, notice string) {
	repo := fr.Slug()
	if stampLabel == "" {
		return "", stampOutcomeOmitted, "NOTICE: no --" + raisedByFlag + " given — this issue is filed with " +
			"UNKNOWN provenance and no by-desk metric can attribute it. Unknown is NOT 'human-raised'; " +
			"it is the absence of an answer. Pass --" + raisedByFlag + " <role> to record which desk raised it."
	}
	present, perr := labelExists(fg, fr, stampLabel)
	switch {
	case perr != nil:
		return "", stampOutcomeUnchecked, "NOTICE: could not check whether label " + stampLabel +
			" exists on " + repo + " (" + perr.Error() + ") — filing UNSTAMPED rather than risking a " +
			"failed filing on an unverified label. This issue reads as UNKNOWN provenance; it is could-not-check, " +
			"not 'the label is absent'."
	case !present:
		return "", stampOutcomeNoLabel, "NOTICE: label " + stampLabel + " does not exist on " + repo +
			" — filing UNSTAMPED (deskfile never mints labels, and applying an unverified one could have " +
			"failed the whole filing). This issue reads as UNKNOWN provenance. Create the label once, then re-run:\n" +
			"  " + labelCreateHint(kind, stampLabel, repo,
			"filed by the "+strings.TrimPrefix(stampLabel, deskkit.RaisedByPrefix)+" desk")
	default:
		return stampLabel, fmt.Sprintf(stampOutcomeStamped, strings.TrimPrefix(stampLabel, deskkit.RaisedByPrefix)), ""
	}
}

// resolveAddressedToStamp decides whether the `to:<role>` addressee label can actually be
// applied, and returns (label-to-apply, audit note, NOTICE for stderr). Like
// resolveRaisedByStamp it NEVER returns an error — an addressee stamp that could refuse a
// filing would be an annotation with veto power over the thing it annotates.
//
// toLabel is "" when no --to was given; it has already been validated against the roster
// by the caller when it is not.
//
// The one behavioural difference from resolveRaisedByStamp: the OMITTED case is SILENT
// (no NOTICE). Addressing a filing to a desk is the rare, deliberate case; NOT addressing
// one is the overwhelming default, so a NOTICE on every unaddressed filing would be noise,
// unlike the raised-by metric where an omission is a gap worth flagging. The other three
// outcomes (label present / label missing / probe outage) mirror the raised-by resolver
// exactly, because an unverified label could fail the whole filing the same way.
func resolveAddressedToStamp(fg deskkit.Forge, fr deskkit.ForgeRepo, kind deskkit.ForgeKind, toLabel string) (apply, note, notice string) {
	repo := fr.Slug()
	if toLabel == "" {
		return "", toOutcomeOmitted, ""
	}
	present, perr := labelExists(fg, fr, toLabel)
	switch {
	case perr != nil:
		return "", toOutcomeUnchecked, "NOTICE: could not check whether label " + toLabel +
			" exists on " + repo + " (" + perr.Error() + ") — filing UNADDRESSED rather than risking a " +
			"failed filing on an unverified label. This is could-not-check, not 'the label is absent'."
	case !present:
		return "", toOutcomeNoLabel, "NOTICE: label " + toLabel + " does not exist on " + repo +
			" — filing UNADDRESSED (deskfile never mints labels, and applying an unverified one could have " +
			"failed the whole filing). " +
			"The addressee's sweep will NOT lead with this issue until it is labelled. Create the label " +
			"once, then re-run:\n" +
			"  " + labelCreateHint(kind, toLabel, repo,
			"addressed to the "+strings.TrimPrefix(toLabel, deskkit.AddressedToPrefix)+" desk")
	default:
		return toLabel, fmt.Sprintf(toOutcomeAddressed, strings.TrimPrefix(toLabel, deskkit.AddressedToPrefix)), ""
	}
}

// labelCreateHint renders the ONE-OFF label-create command an operator runs on the repo's
// forge before re-filing. It is forge-SELECTED, never a `gh` literal: on a GitLab-resolved
// repo `gh label create` cannot work, so printing it leaves the label uncreated and every
// later filing UNSTAMPED (#887 item 2). The kind comes from the same resolution that picked
// the backend (forgeFor → deskkit.ResolveForge), so the hint and the backend cannot drift.
// An unknown kind — a forge this tree has no CLI literal for — gets a neutral instruction
// naming the label and repo rather than a guessed command.
func labelCreateHint(kind deskkit.ForgeKind, label, repo, description string) string {
	switch kind {
	case deskkit.ForgeGitHub:
		return "gh label create " + label + " --repo " + repo + " --description \"" + description + "\" --force"
	case deskkit.ForgeGitLab:
		return "glab label create --name " + label + " --repo " + repo + " --description \"" + description + "\""
	default:
		return "create the label " + label + " on " + repo + " (description: \"" + description +
			"\") with your forge's label tool"
	}
}

// labelExists reports whether label is defined on repo. An error return is the
// could-not-check third state — the caller must not read it as "absent".
//
// ListLabels reads the repo's labels; a paging bound in the backend can in principle page a
// label out (reporting absent and costing a stamp), which is the safe direction — it can never
// invent one, the direction that matters for a provenance claim.
func labelExists(fg deskkit.Forge, fr deskkit.ForgeRepo, label string) (bool, error) {
	// ListLabels READS the repo's label names and never creates one — the deliberate opposite of
	// ApplyLabels' ensure step, which is what preserves the "file UNSTAMPED, never mint the
	// label" behaviour. A backend error is the could-not-check THIRD state (the caller must not
	// read it as "absent"); a successful read that does not carry the name is a definite absence.
	names, err := fg.ListLabels(fr)
	if err != nil {
		return false, err
	}
	for _, n := range names {
		// GitHub/GitLab label names are case-insensitive for uniqueness, so an existing
		// `Raised-By:reviewer` would collide with a `raised-by:reviewer`. Matching the same way
		// keeps the probe agreeing with the forge.
		if strings.EqualFold(strings.TrimSpace(n), label) {
			return true, nil
		}
	}
	return false, nil
}

// --- helpers -----------------------------------------------------------------------

type ghIssueView struct {
	State string
	URL   string
}

// viewIssue reads the target's state and url through the resolved forge (GetIssueTyped, which
// carries the URL — #691 — and reads exactly the STATED kind, so a GitLab number that is both
// an issue and a merge request resolves to the one the caller meant). The state comes back in
// the forge-neutral open|closed vocabulary; the caller compares it case-insensitively against
// "OPEN". Both fields are remote-authored text rendered into the CLOSED-target refusal, so they
// are control-stripped at ingest.
func viewIssue(fg deskkit.Forge, fr deskkit.ForgeRepo, number int, kind deskkit.TargetKind) (*ghIssueView, error) {
	iss, err := fg.GetIssueTyped(fr, number, kind)
	if err != nil {
		return nil, err
	}
	if iss.State == "" {
		return nil, fmt.Errorf("forge GetIssueTyped returned no state for %s#%d", fr.Slug(), number)
	}
	return &ghIssueView{
		State: deskkit.StripControl(iss.State),
		URL:   deskkit.StripControl(iss.URL),
	}, nil
}

func readBody(bodyFile string) ([]byte, error) {
	b, err := os.ReadFile(bodyFile)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read --body-file", err)
	}
	if len(b) > maxBodyBytes {
		return nil, deskkit.Refused(fmt.Sprintf("refused: body exceeds %d bytes (%d)", maxBodyBytes, len(b)))
	}
	return b, nil
}
