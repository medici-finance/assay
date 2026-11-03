package adapter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// AutomationDirectory is the join table between a filing identity and a
// configured loop. deskd builds it from cells.yaml: each cell's loops name
// the App they authenticate as, and that App's login is what appears as the
// issue author.
//
// It is an interface so the adapter does not import the config package (and
// so a test can supply a directory with a deliberate hole in it).
type AutomationDirectory interface {
	// Lookup resolves a filing login to an automation. ok=false is the
	// UNJOINABLE case and is a first-class outcome, not an error.
	Lookup(cell domain.CellName, login string) (domain.AutomationRef, bool)
	// Known returns every login the directory can resolve for a cell, so a
	// could-not-check reason can say what it looked for rather than only
	// that it failed.
	Known(cell domain.CellName) []string
}

// labelKind maps the wire label onto the domain kind. `help wanted` carries
// a space on GitHub and a hyphen in the domain — the drift is handled once,
// here.
var labelKind = map[string]domain.EscalationKind{
	"needs-decision": domain.KindNeedsDecision,
	"question":       domain.KindQuestion,
	"help wanted":    domain.KindHelpWanted,
	"help-wanted":    domain.KindHelpWanted,
	"ask":            domain.KindAsk,
}

// KindForLabel resolves a wire label (or the hyphenated domain spelling of one)
// to its escalation kind. It is the ONLY reader of [labelKind] outside this
// file, so the label vocabulary has one owner in this module: the MCP filer
// derives its argument table from here instead of restating it.
func KindForLabel(label string) (domain.EscalationKind, bool) {
	k, ok := labelKind[strings.ToLower(strings.TrimSpace(label))]
	return k, ok
}

// WireLabel is the GitHub label a kind is filed under: the inverse of
// [labelKind] on its wire spelling. Where the table carries both a wire and a
// domain spelling for one kind (`help wanted` / `help-wanted`), the wire one is
// the key that differs from the domain value. ok=false for an unknown kind.
func WireLabel(k domain.EscalationKind) (string, bool) {
	wire, found := "", false
	for label, kind := range labelKind {
		if kind != k {
			continue
		}
		if !found || label != string(k) {
			wire, found = label, true
		}
	}
	return wire, found
}

// kindPrecedence resolves an issue carrying several escalation labels — the
// real corpus does (13 label instances across 10 issues, design §1.1). The
// most human-actionable label wins, so a `needs-decision` that also carries
// `question` is not demoted.
var kindPrecedence = []domain.EscalationKind{
	domain.KindNeedsDecision, domain.KindHelpWanted, domain.KindAsk, domain.KindQuestion,
}

// goldLabels mark an irreversible / human sign-off gate. A gold item always
// requires the full item view (ruling D1) — it is never answerable in-list.
var goldLabels = map[string]bool{
	"gold": true, "irreversible": true, "gate:human": true, "human-gate": true,
}

// urgentLabels and urgentTitleMarkers together derive UrgencyUrgent. The
// title markers are in the corpus verbatim: one `URGENT:` and one `BLOCKER:`
// among the ten real escalations (design §1.1).
var urgentLabels = map[string]bool{"urgent": true, "blocker": true, "p0": true}

var urgentTitleMarkers = []string{"URGENT", "BLOCKER"}

var elevatedLabels = map[string]bool{"p1": true, "priority:high": true, "bug": true}

// provenanceLabel is the wire label for a policy-raised escalation
// (spec §1.4). Its ABSENCE derives `agent-raised`. Ruling D4 puts the policy
// engine in service contract, so nothing in this build produces the other
// value — the field lands, the producer does not.
const provenanceLabel = "policy-raised"

// MapEscalation turns a platform issue into a domain escalation.
//
// Every field it DERIVES (as opposed to copies) gets a domain.Assessment in
// Escalation.Fields. Two of them can genuinely fail:
//
//   - `automation` — the join from filing login to configured loop. A login
//     that is in no cell's loop set is UNRESOLVED: the escalation is still
//     ingested and still served, with automation null and a reason naming
//     the login and the set that was searched. It is NOT dropped (an
//     unattributed escalation is still a human waiting) and NOT guessed
//     (a wrong attribution would make the service contract fold two unrelated
//     filings into one row). It also gets NO collapse key, so folding can
//     never merge it — spec §1.1.3's "unattributable automations never
//     merge", enforced by the key being empty rather than by a downstream
//     check somebody has to remember.
//   - `options` — the dual-encoding body parse. See [ParseResult.Assessment].
//
// mintID is the adapter's platform-ref → domain-id mapping (spec §1.5); the
// domain never sees the platform key. answers is the cell's poll-side answer
// verifier ([DeriveAnswer]).
func MapEscalation(
	cell domain.CellName,
	repo Repo,
	iss RawIssue,
	dir AutomationDirectory,
	mintID func(repo Repo, number int) domain.EscalationID,
	answers AnswerVerifier,
) domain.Escalation {

	fields := domain.FieldConfidence{}

	// ---- kind -------------------------------------------------------
	kind, kindOK := deriveKind(iss.Labels)
	kindProbe := domain.Probe{
		Source: "gh api graphql: repository.issue(number:).labels — the labels on this poll",
		Method: domain.MethodGraphQLAliased,
		Window: "all labels on the issue",
		Limit:  50, // labels(first: 50) in the derivation document
		Capped: len(iss.Labels) >= 50,
	}
	if kindOK && !kindProbe.Capped {
		fields["kind"] = domain.Clean(kindProbe)
	} else if kindProbe.Capped {
		fields["kind"] = domain.Unchecked(kindProbe,
			"the label page returned its full 50-label limit, so an escalation label may be beyond it")
		kind = domain.KindQuestion
	} else {
		fields["kind"] = domain.Unchecked(kindProbe,
			"none of the escalation labels ("+strings.Join(escalationLabels, ", ")+") is present, "+
				"yet the list probe selected this issue by label — the two views disagree")
		kind = domain.KindQuestion
	}

	// ---- provenance -------------------------------------------------
	prov := domain.AgentRaised
	if hasLabel(iss.Labels, provenanceLabel) {
		prov = domain.PolicyRaised
	}
	fields["provenance"] = domain.Clean(domain.Probe{
		Source: "gh api graphql: repository.issue(number:).labels — presence of the `" + provenanceLabel + "` label",
		Method: domain.MethodGraphQLAliased,
		Window: "absence derives agent-raised (spec §1.4); ruling D4 defers the only policy-raised producer to service contract",
		Limit:  50,
		Capped: kindProbe.Capped,
	})

	// ---- automation (the join that can fail) ------------------------
	automation, joined := domain.AutomationRef{}, false
	if dir != nil {
		automation, joined = dir.Lookup(cell, normaliseLogin(iss.Author))
	}
	joinProbe := domain.Probe{
		Source: "join: repository.issue(number:).author.login × cells.yaml loops[].app",
		Method: domain.MethodLocalTable,
		Window: "the configured loops of cell " + string(cell),
		Limit:  0,
	}
	if joined {
		fields["automation"] = domain.Clean(joinProbe)
	} else {
		known := "(no directory)"
		if dir != nil {
			k := dir.Known(cell)
			sort.Strings(k)
			known = strings.Join(k, ", ")
			if known == "" {
				known = "(cell has no configured loops)"
			}
		}
		fields["automation"] = domain.Unchecked(joinProbe, fmt.Sprintf(
			"filing identity %q matches no configured loop of cell %q (searched: %s) — the row is UNRESOLVED: "+
				"served with automation null, given no collapse key, and never folded",
			iss.Author, cell, known))
	}

	// ---- urgency ----------------------------------------------------
	urgency := deriveUrgency(iss.Title, iss.Labels)
	fields["urgency"] = domain.Clean(domain.Probe{
		Source: "derived: repository.issue(number:).{title,labels}",
		Method: domain.MethodGraphQLAliased,
		Window: "gold labels " + keysOf(goldLabels) + "; urgent labels " + keysOf(urgentLabels) +
			"; title markers " + strings.Join(urgentTitleMarkers, "/"),
		Limit:  50,
		Capped: kindProbe.Capped,
	})

	// ---- options (dual-encoding parse) ------------------------------
	parsed := ParseOptions(iss.Body)
	fields["options"] = parsed.Assessment()

	esc := domain.Escalation{
		ID:         mintID(repo, iss.Number),
		Cell:       cell,
		Automation: automation,
		Kind:       kind,
		Provenance: prov,
		Urgency:    urgency,
		Title:      iss.Title,
		Situation:  situationOf(iss.Body),
		Options:    parsed.Options,
		Rev:        parsed.Rev,
		Rationale:  parsed.Rationale,
		FiledAt:    iss.CreatedAt,
		// Repeat is 1 by definition here: this build stores the collapse
		// COLUMN and performs no folding (the service contract). A row that has
		// never been folded has been raised once.
		Repeat: 1,
		Fields: fields,
	}

	if parsed.Encoding == EncodingTypedV2Malformed {
		// Design §10: render raw, never drop.
		esc.RawBody = iss.Body
	}

	// ---- answer (the service contract increment 3, task 6) ----------------
	// Meaningful only once the issue is closed; DeriveAnswer itself refuses
	// an open iss, so this call is a no-op (nil, false) for the common
	// open-inbox case and costs nothing beyond the already-fetched comments.
	// answers is the cell's poll-side verifier (gap-3 item (5)); its zero
	// value trusts no bot-carried record.
	if ans, ok := DeriveAnswer(iss, answers); ok {
		esc.Answer = ans
	}
	return esc
}

// situationOf takes the prose above the options block as the situation.
func situationOf(body string) string {
	if before, _, found := strings.Cut(body, "### Options"); found {
		return strings.TrimSpace(before)
	}
	if before, _, found := strings.Cut(body, OptionsMarker); found {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(body)
}

// normaliseLogin strips the `[bot]` suffix REST appends to App logins, so a
// directory built from cells.yaml (which names Apps without it) joins
// against either API's spelling.
func normaliseLogin(login string) string {
	return strings.TrimSuffix(login, "[bot]")
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}

func deriveKind(labels []string) (domain.EscalationKind, bool) {
	present := map[domain.EscalationKind]bool{}
	for _, l := range labels {
		if k, ok := labelKind[strings.ToLower(strings.TrimSpace(l))]; ok {
			present[k] = true
		}
	}
	for _, k := range kindPrecedence {
		if present[k] {
			return k, true
		}
	}
	return "", false
}

func deriveUrgency(title string, labels []string) domain.Urgency {
	for _, l := range labels {
		if goldLabels[strings.ToLower(strings.TrimSpace(l))] {
			return domain.UrgencyGold
		}
	}
	for _, l := range labels {
		if urgentLabels[strings.ToLower(strings.TrimSpace(l))] {
			return domain.UrgencyUrgent
		}
	}
	for _, m := range urgentTitleMarkers {
		if strings.Contains(title, m) {
			return domain.UrgencyUrgent
		}
	}
	for _, l := range labels {
		if elevatedLabels[strings.ToLower(strings.TrimSpace(l))] {
			return domain.UrgencyElevated
		}
	}
	return domain.UrgencyRoutine
}

func keysOf(m map[string]bool) string {
	var k []string
	for s := range m {
		k = append(k, s)
	}
	sort.Strings(k)
	return strings.Join(k, "/")
}
