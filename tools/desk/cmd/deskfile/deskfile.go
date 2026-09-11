package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
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
// The budget cannot be reset by varying the session id without a trace: every `new` audit
// line carries the sessionTag it charged (deskkit.SessionTag()), so a caller that rotates
// the env var to reset its bucket leaves a forensic trail of which sessions filed what.
// Rotating the ID does reset the bucket (a new session is a new session) — the audit trace
// is the control, not a hard block.
const (
	// defaultNewBudgetPerSession is the per-session, per-repo cap on `new` writes in a
	// rolling 24h window. 3 is the default — enough for a productive session,
	// low enough to stop a runaway filer.
	defaultNewBudgetPerSession = 3
	// budgetWindow is the rolling window the budget counts over.
	budgetWindow = 24 * time.Hour
)

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

// checkSessionBudget applies the per-session new-issue budget. It returns RateLimited
// (exit 4) when this session+repo has already charged defaultNewBudgetPerSession `new`
// writes in the last 24h, Unverifiable (exit 6) on a corrupt/unreadable audit file
// (fail closed — corruption must not masquerade as an empty budget), and nil when one
// more `new` is within budget. The retry-after is the expiry of the oldest charged write
// in the window (ts + 24h + 1s), so a caller waking on it is certainly past the boundary.
func checkSessionBudget(repo, session string, now time.Time) error {
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return err // already an Unverifiable *DeskError (exit 6)
	}
	cutoff := now.Add(-budgetWindow)
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
	if len(charged) < defaultNewBudgetPerSession {
		return nil
	}
	// The oldest charged write's expiry is when the count drops to cap-1, admitting one
	// more. Sort oldest-first (stable on RFC3339 ts) to find it deterministically.
	sortTimesAscending(charged)
	freeAt := charged[0].Add(budgetWindow).Add(time.Second)
	retryAfter := freeAt.Sub(now)
	if retryAfter <= 0 {
		retryAfter = time.Second
	}
	return deskkit.RateLimitedAfter(fmt.Sprintf(
		"refused: deskfile session budget exhausted (%d `new` on %s in the last 24h for session %q; max %d) — "+
			"retry-after: %ds (free at %s). Attach further observations to an existing issue instead of filing "+
			"new ones, or wait for the 24h window to roll. This is YOUR agent's budget, not the whole "+
			"fan-out's. DO NOT retry-loop by varying $DESK_SESSION (or the harness session id): each `new` "+
			"audit line records the sessionTag it charged, so rotating the ID leaves a trail, it does not "+
			"erase one.",
		len(charged), repo, session, defaultNewBudgetPerSession,
		int(retryAfter/time.Second), freeAt.UTC().Format(time.RFC3339)),
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
	verb           string
	repo           string
	title          string
	bodyDigest     string
	target         *int
	detail         string
	forceNewReason string // non-empty when --force-new bypassed the dedupe search
	successResult  string // ResultOK unless a noop set it otherwise

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
// [--force-new --reason <r>]`. Flow: repo allowed → body+title scan
// → dedupe search (refuse exit 5 on a likely dup; fail closed exit 6 on a search
// API error unless --force-new) → session budget (exit 4 over) → outward-write budget
// → `gh issue create` → audit. --force-new bypasses the dedupe search entirely and
// is audit-logged with its reason (the escape hatch for urgent filings during API outages).
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
	forceNew := fs.Bool("force-new", false, "bypass the dedupe search (escape hatch; requires --reason)")
	reason := fs.String("reason", "", "stated reason for --force-new (required with --force-new)")
	if perr := fs.Parse(args); perr != nil {
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
	if strings.TrimSpace(*bodyFile) == "" {
		return deskkit.Refused("refused: --body-file is required (no stdin/inline body)")
	}
	if *forceNew && strings.TrimSpace(*reason) == "" {
		return deskkit.Refused("refused: --force-new requires a non-empty --reason (the escape hatch is audit-logged)")
	}
	if !deskkit.IsAllowedRepo(*repo) {
		return deskkit.Refused("refused: " + *repo + " is not in the desk-tools repo set")
	}
	ac.repo = *repo
	ac.title = *title

	// Resolve the forge that serves this repo under the session-role App's custody (write-verbs-C).
	// ForgeFor RETAINS the could-not-check refusal on an unresolvable forge (no ASSAY_REPO_FORGES
	// entry and an absent/unmapped origin), and now SERVES GitLab through the backend — the #691
	// interim named-refusal is superseded. Minting the token here is the identity change the #781
	// ruling confirmed; --raised-by stays a body/label attribution below.
	fg, fr, kind, ferr := forgeForFn(*repo)
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

	// Body: file only, 16 KiB cap, secret scan. No override flag exists.
	body, berr := readBody(*bodyFile)
	if berr != nil {
		return berr
	}
	if serr := deskkit.ScanSurface("issue body", body); serr != nil {
		return serr
	}
	if serr := deskkit.ScanSurface("issue title", []byte(*title)); serr != nil {
		return serr
	}
	ac.bodyDigest = deskkit.Sha256Hex(body)

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
			return deskkit.Refused(fmt.Sprintf(
				"refused: likely duplicate of #%d %q (score %.2f%s). Attach your observation there instead:\n"+
					"  deskfile attach -R %s --to %d --body-file <f>\n"+
					"Candidates at/above threshold %.2f:\n%s"+
					"Override with --force-new --reason only if you can justify why this is not a duplicate.",
				top.Number, top.Title, top.Score, classMarker(top.HasClassLabel),
				*repo, top.Number, matchThreshold, formatCandidates(m)))
		}
	} else {
		ac.forceNewReason = *reason
	}

	// Per-session new-issue budget (this tool's own accounting; see checkSessionBudget).
	if berr := checkSessionBudget(*repo, deskkit.SessionTag(), time.Now()); berr != nil {
		return berr
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

	// From here on the create HAS been sent, so every outcome charges session budget —
	// including an unconfirmable one. Set before the call, not after: an error return must
	// carry the marker too. See createSentMarker.
	ac.createSent = true
	ref, cerr := fg.FileIssue(fr, deskkit.IssueInput{Title: *title, Body: string(body)})
	if cerr != nil {
		return deskkit.Unverifiable("file issue failed", cerr)
	}
	n := ref.Number
	ac.target = &n
	// Apply the resolved labels. Every label in applyLabels was confirmed to EXIST above (user
	// labels refuse if missing; stamp/to labels resolve to "" if missing), so ApplyLabels'
	// ensure step no-ops (the create returns already-exists) and NO label is minted.
	if len(applyLabels) > 0 {
		if _, lerr := fg.ApplyLabels(fr, ref.Number, deskkit.LabelChange{Add: applyLabels}); lerr != nil {
			return deskkit.Unverifiable("apply labels to the filed issue failed", lerr)
		}
	}
	url := deskkit.StripControl(ref.URL)
	if ac.forceNewReason != "" {
		ac.detail = "force-new: " + ac.forceNewReason + " | created " + url
	} else {
		ac.detail = "created " + url
	}
	fmt.Println(url)
	return nil
}

// cmdAttach implements `deskfile attach -R <repo> --to <N> --body-file <f>`. Posts the
// observation as a comment on issue N (a class issue or a duplicate target). Never
// budgeted (attach is the motion the gate encourages). Refuses (exit 5) if N is CLOSED
// with the reopen-or-new guidance. Flow: repo allowed → body scan → verify target
// OPEN (fail closed exit 6 on an API error; refuse exit 5 if closed) → outward-write
// budget → `gh issue comment` → audit.
func cmdAttach(args []string) (err error) {
	ac := &auditCtx{verb: "attach"}
	defer func() { ac.finalize(err) }()

	fs := newFlagSet("deskfile attach")
	repo := fs.String("R", "", "target repo, owner/name (required, must be in the desk-tools set)")
	to := fs.Int("to", 0, "target issue number (required)")
	bodyFile := fs.String("body-file", "", "path to a file containing the comment body (required)")
	if perr := fs.Parse(args); perr != nil {
		return deskkit.Refused("refused: bad flags: " + perr.Error())
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
	// superseded).
	fg, fr, _, ferr := forgeForFn(*repo)
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
	view, verr := viewIssue(fg, fr, target)
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

	ref, cerr := fg.PostComment(fr, target, string(body))
	if cerr != nil {
		return deskkit.Unverifiable("post comment failed", cerr)
	}
	url := deskkit.StripControl(ref.URL)
	ac.detail = "commented " + url
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

	// Resolve the forge under the session-role App's custody (write-verbs-C). check is a READ,
	// but it reaches the forge, so it mints the session token like the other verbs; the
	// unresolvable-forge could-not-check refusal is retained, GitLab is served (#691 superseded).
	fg, fr, _, ferr := forgeForFn(*repo)
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

// viewIssue reads an issue's state and url through the resolved forge (GetIssue, which now
// carries the URL — #691). The state comes back in the forge-neutral open|closed vocabulary; the
// caller compares it case-insensitively against "OPEN". Both fields are remote-authored text
// rendered into the CLOSED-target refusal, so they are control-stripped at ingest.
func viewIssue(fg deskkit.Forge, fr deskkit.ForgeRepo, number int) (*ghIssueView, error) {
	iss, err := fg.GetIssue(fr, number)
	if err != nil {
		return nil, err
	}
	if iss.State == "" {
		return nil, fmt.Errorf("forge GetIssue returned no state for %s#%d", fr.Slug(), number)
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
