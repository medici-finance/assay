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
//     ReversibleSignals minus the shape-only needles (NoticeLaneShapeOnlyNeedles).
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

// NoticeLaneShapeOnlyNeedles are the ReversibleSignals needles that name only the SHAPE of a
// change (a default, a rename), not its content. They never admit the notice lane (see the
// file comment); every other ReversibleSignals needle does.
var NoticeLaneShapeOnlyNeedles = []string{"tool default", "default value", "flag default", "rename the"}

// FirstNoticeLaneSignal returns the first ReversibleSignals entry that may admit the notice
// lane — a content-bearing needle, never one of NoticeLaneShapeOnlyNeedles — whose needle
// occurs in hay, or nil. hay must already be lower-cased, as for FirstReversibleSignal.
func FirstNoticeLaneSignal(hay string) *Signal {
	for i := range ReversibleSignals {
		s := &ReversibleSignals[i]
		if slices.Contains(NoticeLaneShapeOnlyNeedles, s.Needle) {
			continue
		}
		if strings.Contains(hay, s.Needle) {
			return s
		}
	}
	return nil
}

// NoticeLaneVerdict decides whether a structurally valid, two-plus-option filing with a held
// catching gate may take the notice lane. admit is true ONLY when the filing is not one-way
// AND carries a positive, content-bearing R-3 reversible signal (FirstNoticeLaneSignal); why
// names the deciding signal either way.
func NoticeLaneVerdict(title, body string, labels []string) (admit bool, why string) {
	if hit, ok := OneWay(title, body, labels); ok {
		return false, "one-way: " + hit.String()
	}
	hay := strings.ToLower(title + "\n" + body)
	if s := FirstNoticeLaneSignal(hay); s != nil {
		return true, "reversible: " + s.Category + " (`" + s.Needle + "`)"
	}
	if s := FirstReversibleSignal(hay); s != nil {
		return false, "only a shape-only R-3 signal (`" + s.Needle + "`), which names the shape of the change, " +
			"not what it governs (fails closed: stays with the human)"
	}
	return false, "no R-3 reversible signal (fails closed: an item the lists cannot place stays with the human)"
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
