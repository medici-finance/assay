package main

import (
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// classify.go — the R-3 classification engine. Pure: it takes a collected item and
// returns a verdict plus the reason for it. No I/O, no clock, no network.
//
// R-3 as drafted (docs/streams/issue-flow/rulings.md):
//
//	"A needs-decision item whose reversal cost is one commit — docs wording, lint
//	 levels, port-or-drop, tool defaults — is decided by the desk … Irreversible,
//	 mechanism, security, and spend decisions remain human-only."
//
// The engine implements that split and refuses to guess across it. THE ASYMMETRY IS
// DELIBERATE: reversible is entered only on a positive one-commit-reversal signal with
// no human-only signal present; human-only is entered on any human-only signal; and an item
// that matches nothing lands in `unclassified`, which is a request for a human look and
// not a decision. There is no arm that returns `reversible` because nothing else fired.

// class is the verdict. Four values, THREE epistemic states: reversible and humanOnly are
// both "read it and placed it"; unclassified is "read it and could not place it";
// couldNotRead is "did not read it". The third must never be rendered as either of the
// first two, and the fourth must never be rendered as "nothing to report".
type class string

const (
	classReversible   class = "reversible"
	classHumanOnly    class = "human-only"
	classUnclassified class = "unclassified"
	classCouldNotRead class = "could-not-read"
)

// classSource names WHICH input produced the verdict, so a reader of the digest can
// audit the precedence rather than take it on trust.
type classSource string

const (
	srcBlessComment classSource = "bless-comment"
	srcHumanLabel   classSource = "human-only-label"
	srcBodyOverride classSource = "body-override"
	srcHeuristic    classSource = "r3-criteria"
	srcNoSignal     classSource = "no-signal"
	srcUnread       classSource = "unread"
)

// verdict is one classification with its provenance and, when a signal fired, the
// literal that fired it.
type verdict struct {
	Class  class
	Source classSource
	Why    string
}

// decisionClassRe matches the override line in an issue body or a comment. Anchored to
// a line start so it cannot be matched out of the middle of a sentence, and the value is
// a closed set — an unrecognised value is a defect the digest REPORTS, never a value it
// rounds to the nearest lane.
var decisionClassRe = regexp.MustCompile(`(?im)^[ \t>*_-]*decision-class:[ \t]*([A-Za-z-]+)`)

// deskRecommendationRe and defaultIfNoAnswerRe read the two fields the digest renders
// per item but must never invent. A recommendation deskdigest made up would read exactly
// like one a human wrote, so the generator lifts them from the item or says they are
// absent.
var (
	deskRecommendationRe = regexp.MustCompile(`(?im)^[ \t>*_-]*desk-recommendation:[ \t]*(.+)$`)
	defaultIfNoAnswerRe  = regexp.MustCompile(`(?im)^[ \t>*_-]*default-if-no-answer:[ \t]*(.+)$`)
)

// humanOnlyLabel marks an infra ask only the human can perform — App permissions,
// token rotation, key custody. The label is introduced by this brief and applied during
// a later sweep, so it does not yet exist in either repo; that absence is
// REPORTED per repo rather than read as an empty set (see collect.go).
const humanOnlyLabel = "human-only"

// needsDecisionLabel is the standing decision-queue label.
const needsDecisionLabel = "needs-decision"

// humanOnlySignals are R-3's human-only side: irreversible, mechanism, security, spend.
// Each entry is a substring matched case-insensitively against title+body. The list is
// deliberately BROADER than the reversible list: a false human-only costs one item staying
// in the human's queue that could have left it, and a false reversible costs a decision
// taken without them. Those are not the same mistake.
var humanOnlySignals = []signal{
	// irreversible
	{"irreversible", "irreversible"},
	{"cannot be undone", "irreversible"},
	{"one-way door", "irreversible"},
	{"delete the", "irreversible"},
	{"force-push", "irreversible"},
	{"publish", "irreversible (publication is not recallable)"},
	{"public repo", "irreversible (publication is not recallable)"},
	{"make it public", "irreversible (publication is not recallable)"},
	{"open source", "irreversible (publication is not recallable)"},
	{"rewrite history", "irreversible"},
	// mechanism
	{"mechanism", "mechanism"},
	{"who may", "mechanism (authority boundary)"},
	{"authority", "mechanism (authority boundary)"},
	{"delegat", "mechanism (authority boundary)"},
	{"merge to main", "mechanism"},
	{"auto-merge", "mechanism"},
	{"ruling", "mechanism"},
	// "human gate" / "gate question", NOT a bare "gate". Measured against the live queue on
	// 2026-08-13, the bare needle classified 4 of 8 items `mechanism` on incidental uses
	// ("the release guard gate", "the consumer-enumeration gate") — a confident wrong REASON
	// attached to a defensible verdict, which is the shape that teaches a reader to stop
	// trusting the Why column. Those items now land unclassified, which is what the engine
	// actually knows about them, and unclassified is a request for a look rather than a
	// claim.
	{"human gate", "mechanism"},
	{"gate question", "mechanism"},
	{"gate: human", "mechanism"},
	// security
	{"security", "security"},
	{"token", "security"},
	{"credential", "security"},
	{"secret", "security"},
	{"permission", "security"},
	{"sops", "security"},
	{"private key", "security"},
	{"rotation", "security"},
	{"scope", "security"},
	// spend
	{"spend", "spend"},
	{"cost", "spend"},
	{"billing", "spend"},
	{"licence", "spend"},
	{"license", "spend"},
	{"subscription", "spend"},
	{"budget", "spend"},
	// production blast radius — an irreversible-in-practice class
	{"production", "irreversible in practice (production)"},
	{"prod deploy", "irreversible in practice (production)"},
	{"on-chain", "irreversible in practice (on-chain)"},
	{"mainnet", "irreversible in practice (on-chain)"},
}

// reversibleSignals are R-3's own four examples of a one-commit reversal and their
// immediate neighbours. Nothing is added here on a hunch: the list stays close to the
// ruling's text because widening it silently widens what the desk may decide without
// asking.
var reversibleSignals = []signal{
	{"docs wording", "docs wording (R-3 example)"},
	{"wording", "docs wording (R-3 example)"},
	{"typo", "docs wording (R-3 example)"},
	{"phrasing", "docs wording (R-3 example)"},
	{"lint level", "lint level (R-3 example)"},
	{"lint severity", "lint level (R-3 example)"},
	{"notice or error", "lint level (R-3 example)"},
	{"port-or-drop", "port-or-drop (R-3 example)"},
	{"port or drop", "port-or-drop (R-3 example)"},
	{"tool default", "tool default (R-3 example)"},
	{"default value", "tool default (R-3 example)"},
	{"flag default", "tool default (R-3 example)"},
	{"rename the", "one-commit reversal (rename)"},
	{"table column", "one-commit reversal (report layout)"},
}

// signal is one matcher plus the R-3 category it evidences.
type signal struct {
	needle   string
	category string
}

// firstSignal returns the first matching signal, or nil.
func firstSignal(hay string, sigs []signal) *signal {
	for i := range sigs {
		if strings.Contains(hay, sigs[i].needle) {
			return &sigs[i]
		}
	}
	return nil
}

// classifyItem applies the precedence chain. The order is load-bearing and is stated in
// --help in the same order it is executed here.
func classifyItem(it *item) verdict {
	// 0. COULD-NOT-READ OUTRANKS EVERYTHING. The highest-precedence input is a comment
	//    by the blessing authority. If the comment thread did not come back, that input
	//    was not consulted — so no verdict below it can be sound, whatever the body or
	//    the keywords say. An engine that classified anyway would be reporting a verdict
	//    it had no basis for, which is the exact failure the three-state rule exists to
	//    stop. This arm is FIRST for that reason, not last.
	if !it.CommentsRead {
		return verdict{classCouldNotRead, srcUnread,
			"the comment thread could not be read (" + it.CommentsErr + "), so a the blessing authority " +
				"decision-class comment — the input that outranks every other — could not be " +
				"ruled in or out. Unread is not unclassified and is not reversible."}
	}

	// 1. A BLESSING-AUTHORITY COMMENT ALWAYS WINS. Latest one wins, so a human can change their mind
	//    in the thread without editing anything. The author check is the strict
	//    id-pinned form: a login is a claim about a name, and the desk, the workers and
	//    the human share automation identities that also report type=User.
	if v, ok := blessCommentClass(it); ok {
		return v
	}

	// 2. THE human-only LABEL. An infra ask only the human can perform is human-only by
	//    construction; a body line cannot argue it out of that lane, which is why this
	//    sits ABOVE the body override and below the blessing-authority comment.
	if hasLabel(it.Labels, humanOnlyLabel) {
		return verdict{classHumanOnly, srcHumanLabel,
			"labelled `" + humanOnlyLabel + "` — an infra ask only the human can perform"}
	}

	// 3. THE BODY OVERRIDE, honoured only for a TRUSTED author. The body of an issue is
	//    written by whoever opened it. Honouring an untrusted body would let an outside
	//    author move their own item into the lane the desk decides without asking — a
	//    self-service authority grant. Refusing it is not a technicality; it is the
	//    difference between an override and an escalation of privilege.
	if m := decisionClassRe.FindStringSubmatch(it.Body); m != nil {
		raw := strings.ToLower(strings.TrimSpace(m[1]))
		if !deskkit.TrustedAuthorID(it.AuthorLogin, it.AuthorID) {
			return verdict{classUnclassified, srcNoSignal,
				"the body carries `decision-class: " + deskkit.StripControl(raw) + "` but its author (" +
					deskkit.StripControl(it.AuthorLogin) + ") is outside the trust roster — an untrusted body " +
					"may not move an item into a lane the desk decides. Needs a human look."}
		}
		switch class(raw) {
		case classReversible:
			return verdict{classReversible, srcBodyOverride, "body override `decision-class: reversible` by a trusted author"}
		case classHumanOnly:
			return verdict{classHumanOnly, srcBodyOverride, "body override `decision-class: human-only` by a trusted author"}
		default:
			return verdict{classUnclassified, srcNoSignal,
				"the body carries `decision-class: " + deskkit.StripControl(raw) + "`, which is not one of " +
					"`reversible` / `human-only`. An unrecognised override is reported, never rounded to the nearest lane."}
		}
	}

	// 4. R-3's OWN CRITERIA, conservatively. human-only is checked first: an item that is
	//    both a docs-wording change and a security change is a security change.
	hay := strings.ToLower(it.Title + "\n" + it.Body)
	if s := firstSignal(hay, humanOnlySignals); s != nil {
		return verdict{classHumanOnly, srcHeuristic, "R-3 " + s.category + " signal (`" + s.needle + "`)"}
	}
	if s := firstSignal(hay, reversibleSignals); s != nil {
		return verdict{classReversible, srcHeuristic, "R-3 one-commit-reversal signal: " + s.category}
	}

	// 5. NO SIGNAL. This is the arm that would be a bug if it returned anything else.
	return verdict{classUnclassified, srcNoSignal,
		"no R-3 signal fired either way — the reversal cost of this item is not derivable from its " +
			"text, so it is NOT assumed cheap. Add `decision-class: reversible|human-only` to the body, " +
			"or rule on it in a comment."}
}

// blessCommentClass finds the LATEST decision-class line written by the roster's
// blessing authority. Precedence within the thread is chronological, so the human's most
// recent word is the operative one.
func blessCommentClass(it *item) (verdict, bool) {
	var best *comment
	for i := range it.Comments {
		c := &it.Comments[i]
		// An App or Bot artifact is never a human ruling, whatever permissions it
		// holds; and type=User alone is not enough, because shared automation accounts
		// report type=User exactly as a person does.
		if !strings.EqualFold(c.AuthorType, "User") {
			continue
		}
		if !deskkit.IsBlessAuthorityIDStrict(c.AuthorLogin, c.AuthorID) {
			continue
		}
		if decisionClassRe.FindStringSubmatch(c.Body) == nil {
			continue
		}
		if best == nil || c.CreatedAt.After(best.CreatedAt) {
			best = c
		}
	}
	if best == nil {
		return verdict{}, false
	}
	raw := strings.ToLower(strings.TrimSpace(decisionClassRe.FindStringSubmatch(best.Body)[1]))
	switch class(raw) {
	case classReversible:
		return verdict{classReversible, srcBlessComment,
			"the blessing authority ruled `decision-class: reversible` in the thread"}, true
	case classHumanOnly:
		return verdict{classHumanOnly, srcBlessComment,
			"the blessing authority ruled `decision-class: human-only` in the thread"}, true
	default:
		// A blessing-authority comment that names an unknown class still WINS over the heuristic —
		// the human spoke, and the engine must not talk over them with a keyword match.
		// It resolves to unclassified because the value is not one deskdigest knows,
		// which is a question for the human, not a licence to pick.
		return verdict{classUnclassified, srcBlessComment,
			"the blessing authority wrote `decision-class: " + deskkit.StripControl(raw) + "`, which is not " +
				"one of `reversible` / `human-only`. The human's word still outranks the heuristic, so no " +
				"keyword classification is applied — this needs one more word from them."}, true
	}
}

// hasLabel is a case-insensitive label membership test.
func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}

// recommendation extracts the desk's recommendation for an item. deskdigest NEVER
// invents one: an absent recommendation renders as absent, because a generated
// recommendation is indistinguishable in the rendered table from one a person reasoned
// about.
func recommendation(it *item) (string, bool) {
	if m := deskRecommendationRe.FindStringSubmatch(it.Body); m != nil {
		return oneLine(m[1]), true
	}
	for i := len(it.Comments) - 1; i >= 0; i-- {
		if m := deskRecommendationRe.FindStringSubmatch(it.Comments[i].Body); m != nil {
			return oneLine(m[1]), true
		}
	}
	return "", false
}

// declaredDefault extracts a stated default-if-no-answer. Same rule as recommendation:
// a safe default EXISTS only when someone stated one. Absence renders as
// `no default — blocks until answered`, per the brief, and that is the honest reading:
// if nobody has written down what happens when the question goes unanswered, then what
// happens is that it stays unanswered.
func declaredDefault(it *item) (string, bool) {
	if m := defaultIfNoAnswerRe.FindStringSubmatch(it.Body); m != nil {
		return oneLine(m[1]), true
	}
	for i := len(it.Comments) - 1; i >= 0; i-- {
		if m := defaultIfNoAnswerRe.FindStringSubmatch(it.Comments[i].Body); m != nil {
			return oneLine(m[1]), true
		}
	}
	return "", false
}

// oneLine flattens remote-authored text into one safe markdown table cell: control
// sequences stripped, newlines folded, pipes escaped so a crafted title cannot forge
// extra columns in the rendered table.
func oneLine(s string) string {
	s = deskkit.StripControl(s)
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
