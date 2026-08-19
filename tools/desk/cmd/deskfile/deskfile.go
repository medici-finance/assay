package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
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
//	label-missing        the role is valid but the repo has no such label, so
//	                     `gh issue create --label` would have FAILED the whole filing.
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
	// labelListLimit bounds the label-existence probe. A repo with MORE labels than this
	// can have its stamp label paged out of the answer — in which case the probe reports
	// it missing and the filing lands UNSTAMPED with a NOTICE. That is the safe
	// direction: the bound can cost a stamp, never invent one.
	labelListLimit = "500"

	stampOutcomeStamped   = "raised-by=%s"
	stampOutcomeOmitted   = "raised-by=UNSTAMPED:not-requested"
	stampOutcomeNoLabel   = "raised-by=UNSTAMPED:label-missing"
	stampOutcomeUnchecked = "raised-by=UNSTAMPED:could-not-check"
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
// The budget cannot be reset by varying $CLAUDE_SESSION_ID without a trace: every `new`
// audit line carries the sessionTag it charged (deskkit.SessionTag()), so a caller that
// rotates the env var to reset its bucket leaves a forensic trail of which sessions filed
// what. Rotating the ID does reset the bucket (a new session is a new session) — the audit
// trace is the control, not a hard block.
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
				fmt.Sprintf("deskfile budget: audit entry has an unparseable ts %q — move file aside to audit.jsonl.corrupt-<ts>", e.TS), perr)
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
			"new ones, or wait for the 24h window to roll. DO NOT retry-loop by varying CLAUDE_SESSION_ID: each "+
			"`new` audit line records the sessionTag it charged, so rotating the ID leaves a trail, it does not "+
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
		cands, serr := dedupeSearch(*repo, *title)
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

	// Create the issue. argv is built literally; no caller flag reaches gh.
	bodyPath, cleanup, terr := writeTempBody(body)
	if terr != nil {
		return deskkit.Unverifiable("cannot stage issue body", terr)
	}
	defer cleanup()

	// Resolve the provenance stamp. This NEVER returns an error: every way it can fail
	// yields an unstamped filing plus a NOTICE, because the stamp is a metric annotation
	// and a metric must not be able to stop a filing. See the raised-by block above.
	stampApply, stampNote, stampNotice := resolveRaisedByStamp(*repo, stampLabel)
	ac.raisedBy = stampNote
	if stampNotice != "" {
		fmt.Fprintln(os.Stderr, stampNotice)
	}

	ghArgs := []string{"issue", "create", "--repo", *repo, "--title", *title, "--body-file", bodyPath}
	for _, l := range labels {
		ghArgs = append(ghArgs, "--label", l)
	}
	if stampApply != "" {
		ghArgs = append(ghArgs, "--label", stampApply)
	}
	// From here on the create HAS been sent, so every outcome charges session budget —
	// including an unconfirmable one. Set before the call, not after: an error return must
	// carry the marker too. See createSentMarker.
	ac.createSent = true
	out, cerr := gh(ghArgs...)
	if cerr != nil {
		return deskkit.Unverifiable("gh issue create failed", cerr)
	}
	url := deskkit.StripControl(strings.TrimSpace(out))
	if num := issueNumberFromURL(url); num > 0 {
		n := num
		ac.target = &n
	}
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
	view, verr := viewIssue(*repo, target)
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

	bodyPath, cleanup, terr := writeTempBody(body)
	if terr != nil {
		return deskkit.Unverifiable("cannot stage comment body", terr)
	}
	defer cleanup()

	out, cerr := gh("issue", "comment", strconv.Itoa(target), "--repo", *repo, "--body-file", bodyPath)
	if cerr != nil {
		return deskkit.Unverifiable("gh issue comment failed", cerr)
	}
	url := deskkit.StripControl(strings.TrimSpace(out))
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

	cands, serr := dedupeSearch(*repo, *title)
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
// The label-existence probe is not decoration. `gh issue create --label <x>` FAILS
// outright when x does not exist on the repo, so applying an unverified stamp would
// convert a missing metric label into a failed filing — the annotation taking down the
// thing it annotates. And no repo has these labels yet: they must be created outside this
// tool (deskfile's mutating vocabulary is `issue create` and `issue comment`, and widening
// it to `label create` for a metric is not a trade this file makes). So the probe reads,
// and a missing label produces a NOTICE naming the exact create command.
//
// THREE-STATE on the probe itself: present / absent / could-not-ask. An unanswered probe
// is NOT treated as "absent" in the message even though both drop the stamp, because the
// remedies differ and a caller told "create the label" during an API outage will create a
// label that already exists and still not be stamped.
func resolveRaisedByStamp(repo, stampLabel string) (apply, note, notice string) {
	if stampLabel == "" {
		return "", stampOutcomeOmitted, "NOTICE: no --" + raisedByFlag + " given — this issue is filed with " +
			"UNKNOWN provenance and no by-desk metric can attribute it. Unknown is NOT 'human-raised'; " +
			"it is the absence of an answer. Pass --" + raisedByFlag + " <role> to record which desk raised it."
	}
	present, perr := labelExists(repo, stampLabel)
	switch {
	case perr != nil:
		return "", stampOutcomeUnchecked, "NOTICE: could not check whether label " + stampLabel +
			" exists on " + repo + " (" + perr.Error() + ") — filing UNSTAMPED rather than risking a " +
			"failed `gh issue create --label`. This issue reads as UNKNOWN provenance; it is could-not-check, " +
			"not 'the label is absent'."
	case !present:
		return "", stampOutcomeNoLabel, "NOTICE: label " + stampLabel + " does not exist on " + repo +
			" — filing UNSTAMPED (applying it would have failed the whole `gh issue create`). " +
			"This issue reads as UNKNOWN provenance. Create the label once, then re-run:\n" +
			"  gh label create " + stampLabel + " --repo " + repo +
			" --description \"filed by the " + strings.TrimPrefix(stampLabel, deskkit.RaisedByPrefix) +
			" desk\" --force"
	default:
		return stampLabel, fmt.Sprintf(stampOutcomeStamped, strings.TrimPrefix(stampLabel, deskkit.RaisedByPrefix)), ""
	}
}

// labelExists reports whether label is defined on repo. An error return is the
// could-not-check third state — the caller must not read it as "absent".
//
// Bounded by labelListLimit: a repo carrying more labels than that can page the stamp
// label out of the answer, which reports absent and costs a stamp. It can never invent
// one, which is the direction that matters for a provenance claim.
func labelExists(repo, label string) (bool, error) {
	out, err := gh("label", "list", "--repo", repo, "--limit", labelListLimit, "--json", "name")
	if err != nil {
		return false, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		// Exit 0 with no stdout is an UNANSWERED probe, not an empty label set. The same
		// distinction dedupeSearch draws for its own empty output.
		return false, fmt.Errorf("gh label list returned no data")
	}
	var got []struct {
		Name string `json:"name"`
	}
	if perr := parseJSON(out, &got); perr != nil {
		return false, perr
	}
	for _, g := range got {
		// GitHub label names are case-insensitive for uniqueness, so an existing
		// `Raised-By:reviewer` would collide with a create of `raised-by:reviewer`.
		// Matching the same way is what keeps the probe agreeing with the API.
		if strings.EqualFold(strings.TrimSpace(g.Name), label) {
			return true, nil
		}
	}
	return false, nil
}

// --- helpers -----------------------------------------------------------------------

type ghIssueView struct {
	State string `json:"state"`
	URL   string `json:"url"`
}

// viewIssue reads an issue's state and url via the ambient gh identity.
func viewIssue(repo string, number int) (*ghIssueView, error) {
	out, err := gh("issue", "view", strconv.Itoa(number), "--repo", repo, "--json", "state,url")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, fmt.Errorf("gh issue view returned no data")
	}
	var v ghIssueView
	if err := parseJSON(out, &v); err != nil {
		return nil, err
	}
	if v.State == "" {
		return nil, fmt.Errorf("gh issue view JSON missing state")
	}
	// Sanitize at ingest: both fields are remote-authored text and both are rendered with
	// %s into the CLOSED-target refusal. See dedupeSearch for the same treatment of titles.
	v.State = deskkit.StripControl(v.State)
	v.URL = deskkit.StripControl(v.URL)
	return &v, nil
}

func parseJSON(s string, v any) error {
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("cannot parse gh JSON: %w", err)
	}
	return nil
}

// issueNumberFromURL extracts the trailing issue number from a GitHub issue URL. Returns 0
// when the URL does not match the expected shape (the create still succeeded; the number
// is forensic, not load-bearing for the budget).
var issueURLRe = regexp.MustCompile(`/issues/(\d+)$`)

func issueNumberFromURL(url string) int {
	m := issueURLRe.FindStringSubmatch(url)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
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

func writeTempBody(body []byte) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "deskfile-body-*.md")
	if err != nil {
		return "", func() {}, err
	}
	name := f.Name()
	if _, err := f.Write(body); err != nil {
		f.Close()
		os.Remove(name)
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", func() {}, err
	}
	return name, func() { os.Remove(name) }, nil
}
