package deskkit

import (
	"regexp"
	"slices"
	"strings"
)

// noticelane.go — which `needs-decision` filings may leave the driver's queue. deskfile's
// fork-test gate files a two-option item with a held catching gate on the NOTICE LANE
// (label desk-decided, off the queue, listed in the weekly digest with a veto date). That
// takes a decision away from the human, so the admission test FAILS CLOSED:
//
//  1. a caller label that marks the item one-way (OneWayLabels) keeps it on the queue;
//  2. any one-way term — HumanOnlySignals, or the broader OneWayPatterns below — keeps it;
//  3. only then, a POSITIVE match on a CONTENT-BEARING R-3 reversible signal admits it —
//     ReversibleSignals minus the shape-only needles (NoticeLaneShapeOnlyNeedles) — read from
//     the filing's declared SUBJECT alone (deskfile's `### Fork test` block's optional
//     `subject:` line), never from the title or body prose.
//
// Step 3 used to scan title+body directly. Round 4 (security review sec-1688-S1) found the
// gap that leaves open: a title routinely carries more than one clause — "Tool default: let
// the desk commit to main when CI is green? Fix the help-text wording too." is a one-way
// governance question PLUS an incidental "wording" fix tacked on — and a scan over the whole
// string admits on whichever clause happens to carry a reversible needle, not on what the
// filing is actually ABOUT. Binding the read to a filer-declared subject line closes that:
// the filer states, once, what is being decided, and only that line is read for admission. An
// item with no `subject:` line simply never admits (fails closed, same as every other
// unrecognised shape); the fork-test gate's structural requirements are unchanged — a
// `subject:` line is never required, only consulted when present.
//
// The same round found a narrower shape the subject-only read does not fix by itself: a
// subject can be genuinely, single-clause about a lint level, a lint severity or a
// notice-vs-error choice — and STILL not be reversible, because what it classifies is a named
// CI check or job ("lint level for the control-sweep check: notice or error?", "port-or-drop
// the pattern-sweep job?"). Round 4 closed that with ciCheckOrJobRe, a check/job noun scan
// applied to the subject — but the noun scan only recognises the generic nouns
// ("check"/"job"/"workflow"/"pipeline" and the "<word>-sweep"/"<word> check" compounds this
// codebase's own CI surfaces use), never a CI check's OWN NAME. A subject that names a real
// check by name and nothing else ("lint level for pin-consistency: notice or error?",
// "govulncheck findings: notice or error?") still admitted, and chasing it with a longer
// noun list only ever catches the names the list happened to enumerate (security review
// sec-1688-S1, round 5). The fix is narrower than a bigger list: `lint level`, `lint
// severity`, `notice or error` and `port-or-drop`/`port or drop` are, BY CONSTRUCTION, always
// a classification question about SOME check or job, named or not — a lint level is the level
// of some check, a port-or-drop is the disposition of some job. So none of the four ever
// admits on its own, named check or not; they moved into NoticeLaneShapeOnlyNeedles below,
// alongside the shape-only needles round 3 found. ciCheckOrJobRe remains as an independent
// backstop for a subject that pairs a DIFFERENT admitting needle (docs wording, a typo, a
// table column) with an explicit check/job noun — but it is no longer what makes the four
// lint-level/port-or-drop needles refuse; they refuse unconditionally now.
//
// Round 5 also found the subject read itself was not as narrow as "the block's `subject:`
// line" implied: a `>`-quoted line still matched the same pattern (so a quoted line of prose
// after the real subject could override it), and when more than one `subject:` line appeared
// the LAST one won — so a first, honest subject followed by an incidental second line (or a
// leftover template placeholder) admitted on whichever one happened to be last, not on what
// the filer actually declared. The fix (forktest.go, parseForkTest): a fork-test section's
// declared subject is read only when there is EXACTLY ONE `subject:` line in the section and
// it is not `>`-quoted. Two or more — quoted, unquoted, or a mix — means no declared subject
// at all, the same fail-closed default as a missing one.
//
// A reversible needle never outranks a one-way term, which is why step 2 runs first and is
// broad. But step 2 is a keyword list, and a keyword list only catches the phrasings it
// names. The shape-only needles — "tool default", "default value", "flag default", "rename
// the" — name the SHAPE of a change and nothing about what it governs ("tool default: build
// untrusted fork heads"), so as an admission signal they admitted every one-way act the
// list had not named (security review sec-1688-S1, three rounds of fresh probes). They are
// therefore NOT admission signals here: an item whose only reversible signal is one of them
// stays with the human. They remain in ReversibleSignals for deskdigest's display classifier,
// where a miss costs nothing but a row's class.
//
// An item that matches nothing is NOT admitted: absence of a one-way term is not evidence
// that the item is reversible, and a substring list that fails open would let every
// one-way item the list forgot leave the queue on the filer's own `caught-by` claim.
//
// The same one-way check (steps 1–2) guards deskfile's other off-queue routes — the
// fewer-than-two-options re-route message and `--no-fork` — so none of those routes takes
// an item the one-way check recognises off the driver's queue. The check is a keyword floor:
// it recognises the phrasings it lists, not every possible one-way wording.

// OneWayLabels are caller labels that mark a filing one-way by construction. A filing
// carrying any of them never takes the notice lane or a `--no-fork` re-route.
var OneWayLabels = []string{"human-only", "security", "gate:human", "gate: human"}

// OneWayPattern is one fail-closed one-way matcher: a word-bounded RE2 pattern over the
// lower-cased title+body, the short name it is reported as, and the one-way class it
// evidences.
type OneWayPattern struct {
	Re       *regexp.Regexp
	Name     string
	Category string
}

func owp(expr, name, category string) OneWayPattern {
	return OneWayPattern{Re: regexp.MustCompile(expr), Name: name, Category: category}
}

// OneWayPatterns widen HumanOnlySignals to the full one-way set the desk skills name — merge,
// ready-flip, main push, tag/release, weakening or disabling a security control or its CI
// assertion, secrets/credentials/keys/PII, money/funds, identity/auth, deleting or
// overwriting durable data, sending or publishing outside, live-infrastructure mutation, App
// permissions and approval authority — plus the `gate:human` spellings. Word boundaries keep
// a stem from matching inside an unrelated word ("tag" never matches "stage", "key" never
// matches "monkey"). The list is deliberately broad: a false one-way costs one item staying
// on the driver's queue; a false miss costs a decision taken without them. Every pattern is
// Go RE2 (linear time, no backtracking) over a body deskfile has already capped.
var OneWayPatterns = []OneWayPattern{
	// merge / ready-flip / main push — the human-held gates themselves
	owp(`\bmerg(e|es|ed|ing)\b`, "merge", "merge authority"),
	owp(`\bready[- ]?flip\w*|\bgh pr ready\b|\bready for (review|merge|human)\b|\bmark\w* (it |the pr |this pr )?(as )?ready\b`, "ready-flip", "ready-flip authority"),
	owp(`\bpush\w* (\w+ ){0,4}to (main|master|the default branch)\b|\b(main|master) push\b|\bforce[- ]push\w*`, "main push", "main push"),
	// tag / release
	owp(`\breleas(e|es|ed|ing)\b|\btag(s|ged|ging)?\b|\bv\d+\.\d+\b|\bship(s|ped|ping)?\b`, "tag/release", "tag/release"),
	// weakening / disabling a security control or its CI assertion
	owp(`\bdisabl\w*|\bweaken\w*|\bbypass\w*|\bloosen\w*|\bbranch[- ]protection\b|\brulesets?\b|\bleak[- ]?sweep\b|\bguardrails?\b|\bvulnerab\w*|\bexploit\w*`, "security control", "security control"),
	// secrets / credentials / keys / PII
	owp(`\bkeys?\b|\bsigning\b|\bcustody\b|\bpii\b|\bpersonal (data|information)\b|\bpasswords?\b|\bcertificates?\b|\bpem\b|\brotat\w*|\bencrypt\w*|\bdecrypt\w*`, "secret/key/PII", "secrets, credentials, keys or PII"),
	// money / funds
	owp(`\bmoney\b|\bfund(s|ed|ing)?\b|\bpay(s|ing|ment|ments)?\b|\bpaid\b|\bvaults?\b|\bsettle\w*|\bpric(e|es|ing)\b|\binvoic\w*|\bfees?\b|\bcommissions?\b|\brefund\w*|\$\s?\d`, "money", "money or funds"),
	// identity / auth
	owp(`\bidentit(y|ies)\b|\bauth(n|z)?\b|\bauthenticat\w*|\bauthori[sz]\w*|\blog ?ins?\b|\bsign[- ]?ins?\b|\bsso\b|\boauth\w*|\boidc\b|\bsaml\b|\brealms?\b|\bidp\b|\bimpersonat\w*`, "identity/auth", "identity or auth"),
	// delete / overwrite durable data
	owp(`\bdelet\w*|\boverwrit\w*|\bwip(e|es|ed|ing)\b|\bpurg\w*|\btruncat\w*|\bdestr(oy|uct)\w*|\berase\w*|\bdrop (the |a |an )?(table|database|db|schema|column|index|bucket|volume|data)\w*`, "delete/overwrite", "deleting or overwriting durable data"),
	// sending or publishing outside the repo
	owp(`\bpublic\w*|\bpublish\w*|\bexternal\w*|\bvendors?\b|\bthird[- ]part(y|ies)\b|\bsen(d|ds|ding|t)\b|\be-?mail\w*|\bannounc\w*|\bcustomers?\b|\bpartners?\b|\bupload\w*|\bexport\w*|\bblog\b`, "external send/publish", "sending or publishing outside"),
	// live-infrastructure mutation
	owp(`\binfra\w*|\bdeploy\w*|\bprod\b|\blive\b|\bclusters?\b|\bdns\b|\bdatabases?\b|\bmigrat\w*|\bservers?\b|\brunners?\b|\bkubernetes\b|\bk8s\b|\bkubectl\b|\bterraform\b`, "live infrastructure", "live-infrastructure mutation"),
	// App permissions / approval authority
	owp(`\bgithub apps?\b|\bapps? permissions?\b|\bactions:\s*(write|read)\b|\b(read|write|admin) access\b|\bgrant\w*|\binstallations?\b|\badmins?\b|\bprivileg\w*|\bapprov\w*|\bself[- ]review\w*|\bsign[- ]?off\w*|\bratif\w*|\bveto\w*|\bcodeowners?\b`, "permission/approval", "App permissions or approval authority"),
	// gate:human in every spelling
	owp(`\bgate:\s*human\b|\bhuman[- ]gated?\b|\bhuman[- ]only\b`, "gate:human", "human gate"),

	// Round 2 (security review at 834c4f8d5): phrasings of the classes above the first set
	// missed. Each is a class the README names; the probes that found them are negative
	// tests (TestNoticeLaneVerdictRefusesControlPhrasings, and deskfile's
	// TestNoticeLaneRefusesReversibleSubjectOneWay).
	//
	// a direct write to main, or any change landing without a PR
	owp(`\b(commit|write|land|push|merg)\w* (\w+ ){0,4}(straight|directly|direct) (to|on|onto|into) (main|master|the default branch)\b|\b(straight|directly|direct) (to|on|onto|into) (main|master|the default branch)\b|\bwithout (a |an |the |any )?(pr|prs|pull requests?|merge requests?|mrs?|reviews?)\b`, "direct to main", "main push"),
	// a draft change taken to ready
	owp(`\bout of draft\b|\bundraft\w*|\bdraft (to|->|→) ready\b|\bfrom draft\b`, "out of draft", "ready-flip authority"),
	// review requirements and dismissal
	owp(`\brequired[- _]?(approving[- _])?reviews?\w*|\breviews? required\b|\breviewers? required\b|\bdismiss\w*`, "required reviews", "App permissions or approval authority"),
	// second factors
	owp(`\b2fa\b|\bmfa\b|\btwo[- ]factor\b|\bmulti[- ]factor\b|\btotp\b|\bpasskeys?\b`, "2FA/MFA", "identity or auth"),
	// hooks, --no-verify, commit signatures
	owp(`\bno[- ]verify\b|\bhooks?\b|\bunsigned\b|\bsignatures?\b|\bgpg\b|\bsigned[- ]commits?\b|\bcommit signing\b`, "hooks/signatures", "security control"),
	// the trust boundary: who the desk acts on
	owp(`\btrust\w*|\brosters?\b|\bany(one|body)\b|\bany (commenter|author|user|login|account)s?\b|\bcommenters?\b|\ballow[- ]?lists?\b|\bdeny[- ]?lists?\b`, "trust boundary", "identity or auth"),
	// org roles and ownership
	owp(`\bowner\w*|\broles?\b|\bmaintainers?\b|\bcollaborators?\b|\bmembers?(hip)?\b|\bteams? (access|membership|permissions?)\b`, "owner/role", "App permissions or approval authority"),
	// repository lifecycle: archive, transfer, visibility
	owp(`\barchiv\w*|\btransfer\w*|\bvisib\w*|\bprivate\b|\b(other|another|different|new|separate|sibling) org(s|ani[sz]ations?)?\b|\borgani[sz]ations?\b|\bmov\w* (\w+ ){0,4}(under|to|into|between) (\w+ ){0,2}orgs?\b|\brenam\w* (the |this |a )?(repo|repository|org|organi[sz]ation)\b`, "archive/transfer/visibility", "deleting or overwriting durable data"),
	// closing items the driver owns
	owp(`\bauto[- ]?(clos|resolv)\w*|\bclos(e|es|ed|ing) (\w+ ){0,4}(needs-decision|human-only|gate)\b|\bneeds-decision (issues? )?(\w+ ){0,3}(clos\w*|older|stale|expir\w*)\b`, "auto-close", "human-only-close authority"),
	// charges
	owp(`\bcharg(e|es|ed|ing)\b|\bcards?\b|\bcredit\b|\bpurchas\w*|\bbuy(s|ing)?\b|\bbought\b|\bsubscri\w*|\bbilling\b`, "charge", "money or funds"),
	// the control-verb class: turning a control off, near a control noun (either order)
	owp(controlVerbExpr, "control off", "security control"),
}

// controlVerbExpr is the control-verb class: a verb that turns a control off ("turn off",
// "switch off", "stop requiring", "no longer require", "skip", "opt out", "remove", "relax",
// "waive", "drop … requirement", "allow … without") within a few words of a control noun
// ("check", "scan", "gate", "guard", "hook", "review", "requirement", "protection", …), in
// either order ("the scan was turned off"). `drop` is paired only with "requirement": the
// R-3 example "port-or-drop" drops scripts, not controls.
const controlNouns = `(checks?|scans?|scanners?|gates?|guards?|hooks?|reviews?|reviewers?|requirements?|protections?|lints?|linters?|tests?|ci|verification|verif(y|ies)|assertions?|signing|policy|policies|rules?|controls?|alerts?)`

var controlVerbExpr = `\b(turn(s|ed|ing)? off|switch(es|ed|ing)? off|stop(s|ped|ping)? (requir|enforc|check|run)\w*|no longer (requir|enforc|check|run)\w*|skip\w*|opt(s|ed|ing)? out( of)?|remov\w*|relax\w*|waiv\w*|suppress\w*|silenc\w*|allow\w* (\S+ ){0,6}?without)\W+(\S+\W+){0,5}?` + controlNouns + `\b` +
	`|\b` + controlNouns + `\W+(\S+\W+){0,4}?(turned off|switched off|skipped|waived|removed|relaxed|suppressed|dropped|no longer (required|enforced|run))\b` +
	`|\bdrop\w* (the |a |an )?(\S+ ){0,3}?requirements?\b`

// OneWayHit names why an item is one-way: the label, needle or pattern that matched, and
// its class. The zero value means "no one-way signal found".
type OneWayHit struct {
	Match    string
	Category string
}

// String renders the hit for a refusal message or audit line.
func (h OneWayHit) String() string { return h.Category + " (`" + h.Match + "`)" }

// OneWay reports whether a filing is one-way: a one-way caller label, a HumanOnlySignals
// needle, or a OneWayPatterns match anywhere in title+body. Labels are checked first, then
// the digest's own list, then the broader patterns, so the reported reason is the most
// specific one available.
func OneWay(title, body string, labels []string) (OneWayHit, bool) {
	for _, l := range labels {
		for _, ow := range OneWayLabels {
			if strings.EqualFold(strings.TrimSpace(l), ow) {
				return OneWayHit{Match: "label " + ow, Category: "one-way label"}, true
			}
		}
	}
	return OneWayExempting(title + "\n" + body)
}

// OneWayExempting is OneWay's text half — a HumanOnlySignals needle or a OneWayPatterns match
// anywhere in text — with the named HumanOnlySignals needles exempt. Every OneWayPatterns
// entry and every other needle is still read. deskfile uses it for the fork-test block's
// `ruled-check:` line: that line records the search for an existing ruling, so its natural
// wording ("no prior ruling found") would trip the `ruling` needle on every filing — but it is
// also where a filer names the SUBJECT of the search, so the line is read against everything
// else (security review sec-1688-S1, round 2).
func OneWayExempting(text string, exempt ...string) (OneWayHit, bool) {
	hay := strings.ToLower(text)
	for i := range HumanOnlySignals {
		sig := &HumanOnlySignals[i]
		if slices.Contains(exempt, sig.Needle) {
			continue
		}
		if strings.Contains(hay, sig.Needle) {
			return OneWayHit{Match: sig.Needle, Category: sig.Category}, true
		}
	}
	for _, p := range OneWayPatterns {
		if m := p.Re.FindString(hay); m != "" {
			return OneWayHit{Match: m, Category: p.Category}, true
		}
	}
	return OneWayHit{}, false
}

// NoticeLaneShapeOnlyNeedles are the ReversibleSignals needles that never admit the notice
// lane on their own (see the file comment); every other ReversibleSignals needle does. Two
// different reasons put a needle on this list:
//
//   - `tool default`, `default value`, `flag default`, `rename the` name only the SHAPE of a
//     change (a default, a rename), not its content (round 3).
//   - `lint level`, `lint severity`, `notice or error`, `port-or-drop`, `port or drop` name a
//     classification question that is, by construction, always ABOUT some check or job,
//     named in the subject or not — so an admission rule keyed on the check/job noun (round 4)
//     only ever caught the names it enumerated (round 5).
//
// deskdigest's display classifier still reads every one of them (FirstReversibleSignal, from
// title+body, unchanged) — a miss there costs a row's display class, never a decision taken
// without the driver.
var NoticeLaneShapeOnlyNeedles = []string{
	"tool default", "default value", "flag default", "rename the",
	"lint level", "lint severity", "notice or error", "port-or-drop", "port or drop",
}

// hyphenVariantReplacer maps the Unicode hyphen/dash characters most likely to be typed or
// pasted in place of an ASCII hyphen (U+2010 HYPHEN, U+2011 NON-BREAKING HYPHEN, U+2012
// FIGURE DASH, U+2013 EN DASH, U+2014 EM DASH, U+2212 MINUS SIGN, plus, since round 7,
// U+FF0D FULLWIDTH HYPHEN-MINUS, U+FE63 SMALL HYPHEN-MINUS, U+00AD SOFT HYPHEN, and U+2043
// HYPHEN BULLET) to plain "-". It is applied once, in NoticeLaneVerdict, to the subject before
// any needle or regex test in this file runs against it: `ciCheckOrJobRe`'s "<word>-sweep"
// compounds and the hyphenated needles (`port-or-drop`) are ASCII-hyphen literals, so a check
// name or phrase typed with a "fancy" hyphen — a smart-quote editor's autocorrect, a pasted em
// dash — would otherwise silently miss both (security review sec-1688-S1, round 6 advisory:
// "fix the typo in the pattern‑sweep message", U+2011, got past `ciCheckOrJobRe`; round 7 widens
// the set with further look-alikes named in the withheld review detail, including the soft
// hyphen, which renders as no visible character at all).
var hyphenVariantReplacer = strings.NewReplacer(
	"‐", "-",
	"‑", "-",
	"‒", "-",
	"–", "-",
	"—", "-",
	"−", "-",
	"－", "-",
	"﹣", "-",
	"\u00ad", "-", // U+00AD SOFT HYPHEN — renders as no visible character at all
	"⁃", "-",
)

// normalizeHyphens rewrites every Unicode hyphen/dash look-alike hyphenVariantReplacer lists
// to the ASCII hyphen.
func normalizeHyphens(s string) string { return hyphenVariantReplacer.Replace(s) }

// hasNonASCIIByte reports whether s contains any byte >= 0x80 — true for any UTF-8 encoded
// non-ASCII rune (a Unicode hyphen/dash look-alike beyond the four normalizeHyphens widens, a
// non-breaking space, a zero-width character, or anything else outside plain ASCII).
func hasNonASCIIByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}

// isWordByte reports whether b is an ASCII word character (letter, digit or underscore) — the
// same class regexp's `\w`/`\b` use, applied by hand so wordBoundaryContains needs no
// per-needle regexp compilation.
func isWordByte(b byte) bool {
	return b == '_' ||
		('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') ||
		('0' <= b && b <= '9')
}

// wordBoundaryContains reports whether needle occurs in hay bounded by a non-word character
// (or the string's own start/end) on BOTH ends — never as a bare substring run inside a
// larger word. Needle may itself contain internal spaces or hyphens ("notice or error",
// "port-or-drop"); only its two ends are boundary-checked, so it still matches as written.
//
// Without this, `strings.Contains` alone let the "wording" needle match inside "rewording"
// (security review sec-1688-S1, round 6 advisory) — "rewording" is not "wording", but the
// substring is there regardless of what word it sits inside.
func wordBoundaryContains(hay, needle string) bool {
	if needle == "" {
		return false
	}
	from := 0
	for {
		i := strings.Index(hay[from:], needle)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(needle)
		beforeOK := start == 0 || !isWordByte(hay[start-1])
		afterOK := end == len(hay) || !isWordByte(hay[end])
		if beforeOK && afterOK {
			return true
		}
		from = start + 1
	}
}

// separatorRunRe matches a RUN of one or more whitespace, underscore, dot, slash or hyphen
// characters — the separator alphabet a subject's own spelling of a multi-word needle might
// use in place of the plain space every ReversibleSignals/NoticeLaneShapeOnlyNeedles entry is
// written with ("lint_level", "lint-level", "lint.level", "lint  level" for "lint level").
var separatorRunRe = regexp.MustCompile(`[\s_./-]+`)

// collapseSeparators rewrites every RUN of separatorRunRe's alphabet to a single space, so a
// needle and a hay written with different (or doubled) separators still compare equal. Round
// 7.1 (security review sec-1688-S1 advisory, veto evasion): before this, "port-or-drop" and
// "port or drop" had to be enumerated as two SEPARATE NoticeLaneShapeOnlyNeedles entries (and
// still missed "port_or_drop", a doubled space, or any other separator spelling) — collapsing
// once, on both sides of the comparison, closes the whole class instead of one spelling at a
// time.
func collapseSeparators(s string) string {
	return separatorRunRe.ReplaceAllString(s, " ")
}

// subjectContainsNeedle is wordBoundaryContains with collapseSeparators applied to BOTH sides
// first — used only for matching a fork-test subject against the shape-only veto and
// content-bearing needle lists in FirstNoticeLaneSignal, never for the broader HumanOnlySignals
// scan over title+body prose (OneWay), which keeps ordinary wordBoundaryContains: collapsing
// separators there would risk widening what counts as a one-way TERM in free-form prose, which
// is not this fix's target.
func subjectContainsNeedle(subject, needle string) bool {
	return wordBoundaryContains(collapseSeparators(subject), collapseSeparators(needle))
}

// ciCheckOrJobRe names a CI check or job by the nouns this codebase's own CI surfaces use
// (check/checks, job/jobs, workflow/workflows, pipeline/pipelines) and the "<word>-sweep" /
// "<word> check" compounds those surfaces are actually named with (leak-sweep, control-sweep,
// pattern-sweep, "the leak check", …). A subject that names one alongside an OTHER admitting
// reversible needle (docs wording, a typo, a table column, …) is asking a classification
// question about that check or job rather than making the edit the needle would otherwise
// suggest, so FirstNoticeLaneSignal never admits on it either. This is now a backstop, not the
// only guard on the round-4 lint-level/port-or-drop shape: those four needles never admit at
// all any more (NoticeLaneShapeOnlyNeedles, round 5) because a longer noun list here only ever
// catches the check names it enumerates, never one named by itself
// ("lint level for pin-consistency: notice or error?" names no noun this regexp lists).
// Deliberately broad, per this file's own fail-closed direction: a false refusal costs one
// item staying on the driver's queue.
var ciCheckOrJobRe = regexp.MustCompile(`(?i)\b\w+[- ](?:sweep|check)\b|\b(?:checks?|jobs?|workflows?|pipelines?)\b`)

// FirstNoticeLaneSignal returns the first ReversibleSignals entry that may admit the notice
// lane — a content-bearing needle, never one of NoticeLaneShapeOnlyNeedles, and never when
// subject names a CI check or job (ciCheckOrJobRe) — whose needle occurs in subject, or nil.
// subject must already be lower-cased and hyphen-normalised (NoticeLaneVerdict does both
// before calling this). subject is the filing's declared subject alone (a `### Fork test`
// block's `subject:` line), never title+body — see the file comment and NoticeLaneVerdict.
//
// A NoticeLaneShapeOnlyNeedles match is a VETO, checked BEFORE any content-bearing needle,
// never a skip: round 5 made the four lint-level/port-or-drop needles never admit ON THEIR
// OWN, by excluding them from the content-needle scan below — but excluding them from that
// scan is not the same as refusing the subject outright, so a subject that ALSO carried an
// unrelated content-bearing needle ("wording of the pin-consistency lint level: notice or
// error" — "wording" is a real ReversibleSignals needle) still admitted through it, silently
// outvoting the shape-only phrase (correctness re-review cor-1688-C7, residual; security
// review sec-1688-S1, round 6). Checking the veto first, and returning nil the instant one
// matches, means a shape-only phrase can never be outvoted by a second needle in the same
// subject.
func FirstNoticeLaneSignal(subject string) *Signal {
	if ciCheckOrJobRe.MatchString(subject) {
		return nil
	}
	for _, n := range NoticeLaneShapeOnlyNeedles {
		if subjectContainsNeedle(subject, n) {
			return nil
		}
	}
	for i := range ReversibleSignals {
		s := &ReversibleSignals[i]
		if subjectContainsNeedle(subject, s.Needle) {
			return s
		}
	}
	return nil
}

// NoticeLaneVerdict decides whether a structurally valid, two-plus-option filing with a held
// catching gate may take the notice lane. The one-way check still reads the whole title and
// body (unchanged: OneWay's fail-closed floor is not what round 4 found narrow — see the file
// comment). admit is true ONLY when the filing is not one-way AND subject — the filing's
// declared subject alone, never title or body prose — carries a positive, content-bearing R-3
// reversible signal (FirstNoticeLaneSignal); why names the deciding signal either way. subject
// is read exactly as given (case-folded here, so callers pass it unfolded); an empty subject
// (no `subject:` line in the fork-test block) never admits.
func NoticeLaneVerdict(title, body, subject string, labels []string) (admit bool, why string) {
	if hit, ok := OneWay(title, body, labels); ok {
		return false, "one-way: " + hit.String()
	}
	subj := normalizeHyphens(strings.ToLower(strings.TrimSpace(subject)))
	if subj == "" {
		return false, "no declared subject (fails closed: the reversible signal is read only from the fork-test " +
			"block's `subject:` line, and none was given)"
	}
	// Round 7.1 (security review sec-1688-S1 advisory): normalizeHyphens above closes the four
	// Unicode hyphen/dash look-alikes round 7 named, but that is inherently a finite list —
	// eleven further hyphen/dash code points, plus a non-breaking space, a zero-width character
	// or any other non-ASCII look-alike, still slip past a check/needle scan built on ASCII
	// literals. Rather than enumerate more code points, ANY non-ASCII byte in the subject fails
	// closed here, before ciCheckOrJobRe or FirstNoticeLaneSignal ever run: a subject the driver
	// typed in plain ASCII never trips this, and one that did not is exactly the shape a
	// look-alike or invisible-character evasion needs.
	if hasNonASCIIByte(subj) {
		return false, "the subject contains a non-ASCII character (fails closed: a look-alike or " +
			"invisible character could otherwise change what a check/keyword scan reads — stays with the human)"
	}
	if s := FirstNoticeLaneSignal(subj); s != nil {
		return true, "reversible: " + s.Category + " (`" + s.Needle + "`)"
	}
	if ciCheckOrJobRe.MatchString(subj) {
		return false, "the subject names a CI check or job: an R-3 reversible example applied to it is a " +
			"classification decision about that check, not a reversible edit to it (fails closed: stays with the human)"
	}
	if s := FirstReversibleSignal(subj); s != nil {
		return false, "only a shape-only R-3 signal (`" + s.Needle + "`), which names the shape of the change, " +
			"not what it governs (fails closed: stays with the human)"
	}
	return false, "no R-3 reversible signal in the subject (fails closed: an item the lists cannot place stays with the human)"
}

// DeskDecidedMarker (decided.go) is the machine-readable marker a desk R-3 decision carries
// — the one deskdigest's veto surface reads, written by hand in a comment, by deskpr into a
// PR body, or by deskfile's notice lane into the filed issue's own body. One const, declared
// once, beside the block's shared parse/render.

// deskDecidedHeadingRe matches the tool-written `## Desk-decided` heading, any level.
var deskDecidedHeadingRe = regexp.MustCompile(`(?i)^\s*#{1,6}\s*Desk-decided\s*$`)

var anyMDHeadingRe = regexp.MustCompile(`^\s*#{1,6}(\s|$)`)

// StripDeskDecidedBlock returns body without the tool-written `## Desk-decided` section
// (heading through the next heading or EOF) when that section carries the marker, in any
// spelling DeskDecidedMarkerRe (the digest's own reader) accepts.
// The block is the TOOL's text, not the filer's: a classifier that read it would classify
// every notice on the tool's own field names (`cost:` is an R-3 spend needle).
func StripDeskDecidedBlock(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		if deskDecidedHeadingRe.MatchString(lines[i]) {
			end := len(lines)
			for j := i + 1; j < len(lines); j++ {
				if anyMDHeadingRe.MatchString(lines[j]) {
					end = j
					break
				}
			}
			if DeskDecidedMarkerRe.MatchString(strings.Join(lines[i:end], "\n")) {
				i = end - 1
				continue
			}
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}
