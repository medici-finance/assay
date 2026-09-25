package deskkit

import (
	"regexp"
	"strings"
)

// noticelane.go — which `needs-decision` filings may leave the driver's queue. deskfile's
// fork-test gate files a two-option item with a held catching gate on the NOTICE LANE
// (label desk-decided, off the queue, listed in the weekly digest with a veto date). That
// takes a decision away from the human, so the admission test FAILS CLOSED:
//
//  1. a caller label that marks the item one-way (OneWayLabels) keeps it on the queue;
//  2. any one-way term — HumanOnlySignals, or the broader OneWayPatterns below — keeps it;
//  3. only then, a POSITIVE match on R-3's ReversibleSignals admits it.
//
// An item that matches nothing is NOT admitted: absence of a one-way term is not evidence
// that the item is reversible, and a substring list that fails open would let every
// one-way item the list forgot leave the queue on the filer's own `caught-by` claim.
//
// The same one-way check (steps 1–2) guards deskfile's other off-queue routes — the
// fewer-than-two-options re-route message and `--no-fork` — so no route the tool offers
// steers a one-way item off the driver's queue.

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
}

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
	hay := strings.ToLower(title + "\n" + body)
	if s := FirstHumanOnlySignal(hay); s != nil {
		return OneWayHit{Match: s.Needle, Category: s.Category}, true
	}
	for _, p := range OneWayPatterns {
		if m := p.Re.FindString(hay); m != "" {
			return OneWayHit{Match: m, Category: p.Category}, true
		}
	}
	return OneWayHit{}, false
}

// NoticeLaneVerdict decides whether a structurally valid, two-plus-option filing with a held
// catching gate may take the notice lane. admit is true ONLY when the filing is not one-way
// AND carries a positive R-3 reversible signal; why names the deciding signal either way.
func NoticeLaneVerdict(title, body string, labels []string) (admit bool, why string) {
	if hit, ok := OneWay(title, body, labels); ok {
		return false, "one-way: " + hit.String()
	}
	if s := FirstReversibleSignal(strings.ToLower(title + "\n" + body)); s != nil {
		return true, "reversible: " + s.Category + " (`" + s.Needle + "`)"
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
// (heading through the next heading or EOF) when that section carries DeskDecidedMarker.
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
			if strings.Contains(strings.Join(lines[i:end], "\n"), DeskDecidedMarker) {
				i = end - 1
				continue
			}
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}
