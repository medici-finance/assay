package main

// vocabulary.go — the role-ownership table: which label belongs to which role, which
// labels belong to every role, and which belong to none.
//
// THIS TABLE IS THE CONTROL. It is the one thing standing between "a role writes a label"
// and "a role forges another role's marker", so it is closed at build time with no runtime
// mutation path: no flag, no environment variable, no config file extends it. A defect here
// is a code-review question over a handful of lines, not a live surface an operator's
// misconfiguration could widen. Every entry names the code that already APPLIES the label at
// its own call site — ownership is read from that code, never assumed.
//
// TWO CLOSED INPUTS, ONE TABLE. The shared escalation rows are the labels topology.yaml
// declares under labels.decision_owed, read from the topology loader (the desk module's
// compiled-in derivation, itself diffed against the source by TestTopologyDriftRegistry)
// rather than restated here — a second hand copy of a declared set is the drift the
// registry test exists to catch. The role-owned and refused rows are desklabel's own
// vocabulary: no topology category declares them, so they live here as a literal.
//
// A label is matched CASE-INSENSITIVELY against the table (forge labels are
// case-insensitive) and always written in its canonical case, so `Approval-Needed` and
// `approval-needed` are one entry and one write.

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/topology"
)

// The two role names the table keys on. They are the App roles deskkit's loop → role map
// resolves (roletoken.go), spelled once here so the table and the refusal messages agree.
const (
	roleWorker   = "worker"
	roleReviewer = "reviewer"
)

// Owner sentinels. A real role name in the Owner field means that role, and only that role,
// may set or clear the label.
const (
	// ownerShared — ANY resolved role may set or clear it. The escalation vocabulary:
	// only the human-only-CLOSE gate on the decision class is absolute (deskclose's
	// decision-label gate reads the label fresh on its own next call); the LABEL itself is
	// not role-exclusive, because any role may need to escalate.
	ownerShared = "shared"
	// ownerNone — refused for EVERY role, explicitly rather than by omission.
	ownerNone = "no role"
)

// helpWantedLabel is the one shared escalation label that topology.yaml declares in NO
// category — it is neither system_state nor decision_owed — so it is desklabel's own row,
// not a restatement of the declared source. Everything else in the shared set is READ
// from the topology loader (see sharedRows).
const helpWantedLabel = "help wanted"

// vocabEntry is one row of the table.
type vocabEntry struct {
	// Canonical is the label as it is written to the forge.
	Canonical string
	// Owner is roleWorker, roleReviewer, ownerShared or ownerNone.
	Owner string
	// Why cites the code that makes this the owner — the reviewer's cross-check.
	Why string
}

// vocabulary is the closed table, assembled ONCE at init from two closed inputs: the
// topology loader's compiled decision-owed set (the shared escalation rows) and the
// desklabel-owned rows below. Neither input is runtime state — topology.Compiled() is the
// desk module's compiled-in derivation of topology.yaml (bound to the source by
// TestTopologyDriftRegistry), not a file read — so the table is still a build-time
// literal with no flag, environment variable or config file extending it. Order is
// presentation only; lookup is by name.
var vocabulary = buildVocabulary(topology.Compiled())

// buildVocabulary assembles the table from a topology. It takes the topology as a
// parameter so a test can hand it a DIFFERENT declared set and prove the shared rows
// follow it — the derivation is checked, not assumed.
func buildVocabulary(top topology.Topology) []vocabEntry {
	rows := sharedRows(top)
	rows = append(rows, desklabelOwnedRows...)
	return rows
}

// sharedRows derives the shared escalation rows: every label topology.yaml declares under
// labels.decision_owed (the SLA escalation scope — needs-decision, question and the
// human-decision-queue class), read from the loader rather than restated here so a change
// to the declared set cannot leave this table silently disagreeing with it; plus
// helpWantedLabel, the escalation label no topology category carries.
func sharedRows(top topology.Topology) []vocabEntry {
	names := top.DecisionOwedLabelNames() // sorted; a copy
	out := make([]vocabEntry, 0, len(names)+1)
	for _, name := range names {
		out = append(out, vocabEntry{Canonical: name, Owner: ownerShared,
			Why: "escalation: topology.yaml labels.decision_owed, read via the topology loader (any role may " +
				"need to escalate; only deskclose's human-only-CLOSE gate on the decision class is absolute, " +
				"the label itself is not role-exclusive)"})
	}
	out = append(out, vocabEntry{Canonical: helpWantedLabel, Owner: ownerShared,
		Why: "escalation: needs hands (deskfile --label filing convention; declared in no topology category, " +
			"so this is desklabel's own row)"})
	return out
}

// desklabelOwnedRows are the rows that are desklabel's OWN vocabulary — role-owned
// markers and the refused-for-everyone marker. None of them is a topology label; each
// names the code that already applies it at its own call site.
var desklabelOwnedRows = []vocabEntry{
	// --- worker-owned: worker-authored findings ---------------------------------------------
	{Canonical: "superseded?", Owner: roleWorker,
		Why: "deskclose superseded: the WORKER proposes (labelProposed); the reviewer only confirms or disputes"},
	{Canonical: "disposition:superseded", Owner: roleWorker,
		Why: "deskdisposition set records a WORKER's finding (DispositionVerdict.Label)"},
	{Canonical: "disposition:resolved-elsewhere", Owner: roleWorker,
		Why: "deskdisposition set records a WORKER's finding (DispositionVerdict.Label)"},
	{Canonical: "disposition:needs-rebase", Owner: roleWorker,
		Why: "deskdisposition set records a WORKER's finding (DispositionVerdict.Label)"},

	// --- reviewer-owned: the ready-flip queue-state pair ------------------------------------
	{Canonical: "authorization-needed", Owner: roleReviewer,
		Why: "deskflip's labelBeforeFlip; deskdispatch applies it under the REVIEWER role's own credential"},
	{Canonical: "approval-needed", Owner: roleReviewer,
		Why: "deskflip's labelAfterFlip — the reviewer-run ready flip's post-state"},

	// --- refused for every role ---------------------------------------------------------------
	{Canonical: "human-decided", Owner: ownerNone,
		Why: "records that a HUMAN ruled (deskclose's decisionLabels); a role self-applying it is a forgery"},
}

// lookup finds the entry for label, case-insensitively and whitespace-trimmed. ok=false
// means the label has no desklabel vocabulary entry at all.
func lookup(label string) (vocabEntry, bool) {
	want := strings.TrimSpace(label)
	for _, e := range vocabulary {
		if strings.EqualFold(e.Canonical, want) {
			return e, true
		}
	}
	return vocabEntry{}, false
}

// permits answers whether role may set or clear the label e names. It is the single
// ownership decision every write runs through, kept as one small function so a review can
// read the whole rule in one place.
func permits(e vocabEntry, role string) bool {
	switch e.Owner {
	case ownerShared:
		return true
	case ownerNone:
		return false
	default:
		return e.Owner == role
	}
}

// authorize is the ownership check: it returns the CANONICAL label when role may write
// label, and a Refused (exit 5) naming the label, the role that DOES own it (or "no role"),
// and the role the session resolved, otherwise. It runs before any forge read or write, and
// it is the only path to a canonical name — a label that did not pass here has no spelling
// the write path can use.
func authorize(label, role string) (string, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		return "", deskkit.Refused("refused: no session role resolved — the label vocabulary is keyed on the " +
			"session's App role and there is no flag that asserts one")
	}
	e, ok := lookup(label)
	if !ok {
		return "", deskkit.Refused(fmt.Sprintf(
			"refused: label %q has no desklabel vocabulary entry — it belongs to no role this verb serves "+
				"(session role: %s). The vocabulary is closed: shared %s; worker-owned %s; reviewer-owned %s. "+
				"A label some other verb provisions at its own call site is that verb's, not this one's.",
			deskkit.StripControl(label), deskkit.StripControl(role),
			strings.Join(ownedBy(ownerShared), ", "), strings.Join(ownedBy(roleWorker), ", "),
			strings.Join(ownedBy(roleReviewer), ", ")))
	}
	if permits(e, role) {
		return e.Canonical, nil
	}
	return "", deskkit.Refused(fmt.Sprintf(
		"refused: label %q is owned by %s, and this session's App role is %s — a role writes only the "+
			"markers it owns (%s). The role is read from the session (DESK_LOOP), never from a flag.",
		e.Canonical, ownerPhrase(e.Owner), deskkit.StripControl(role), e.Why))
}

// ownerPhrase renders an Owner value for a refusal message.
func ownerPhrase(owner string) string {
	switch owner {
	case ownerShared:
		return "every role (shared)"
	case ownerNone:
		return "no role at all (refused for every role)"
	default:
		return "the " + owner + " role"
	}
}

// ownedBy lists the canonical labels a given Owner value carries, sorted.
func ownedBy(owner string) []string {
	var out []string
	for _, e := range vocabulary {
		if e.Owner == owner {
			out = append(out, e.Canonical)
		}
	}
	sort.Strings(out)
	return out
}

// printVocabulary renders the table — the `vocabulary` verb, so an operator can read the
// control rather than infer it from refusals.
func printVocabulary(w io.Writer) {
	for _, e := range vocabulary {
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.Canonical, e.Owner, e.Why)
	}
}
