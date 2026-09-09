package main

// classify.go — THE JUDGEMENT FILE.
//
// Everything in this package that is a heuristic rather than a count lives here, in
// one file, behind one version string. That is the whole point of ClassifierVersion:
// relay-vs-substantive is an opinion encoded in code, so a trend line that moves can
// move for two completely different reasons — the operator's behaviour changed, or the
// ruler changed. Only the emitted version tells those apart. Change ANY rule below and
// you MUST bump ClassifierVersion in the same commit; the accuracy test pins the
// version alongside its measured score so the pairing cannot silently drift.
//
// PRIVACY. This file is the only place message text is looked at, and it is looked at
// to produce a label — never a copy. Nothing here returns text, and nothing here is
// reachable from the emit structs (see emit.go's allowlist test). If a metric can only
// be computed by RETAINING content, it is not computed: it is reported as unmeasured.

import (
	"regexp"
	"strings"
)

// ClassifierVersion is emitted in every day-file. Bump on ANY rule change here.
//
//	1 — first cut: cue families + near-duplicate detection,
//	    measured against cmd/opmetrics/testdata/labelled. See README for the score.
//	2 — attention classes: a SECOND, additive label axis (AttentionFamily) over
//	    the same messages — route/status/toil/correction/decision/idea/ack/other.
//	    The v1 relay-vs-substantive axis and its families are UNCHANGED, so the
//	    relay-ratio trend line is unbroken; v2 only ADDS the attention-family
//	    breakdown. The bump is required because a NEW judgement rule was added,
//	    and the accuracy corpus was re-labelled with the family ground truth.
const ClassifierVersion = "opmetrics-relay/2"

// Class is the label the classifier assigns to one operator message.
type Class string

const (
	// ClassRelay — the message carried STATE rather than a decision: a poke, a
	// status echo, a "sync to main", a re-send of something already said. These are
	// the messages the adoption ladder wants to see go to zero: the operator acting
	// as a message bus between agents that could have read the state themselves.
	ClassRelay Class = "relay"
	// ClassSubstantive — the message contained a decision, a constraint, a
	// correction with content, or new work. These are the messages worth a human.
	ClassSubstantive Class = "substantive"
	// ClassEmpty — nothing left after normalisation (an image-only or
	// whitespace-only turn). Counted separately and kept OUT of the ratio's
	// denominator: it is neither a relay nor a decision, and folding it into either
	// would move the headline number for a reason that has nothing to do with the
	// operator.
	ClassEmpty Class = "empty"
)

// relayMaxWords is the length ceiling for cue-based relay classification. A long
// message is substantive even when it opens with a cue, because length is the single
// most reliable proxy for "this turn carried content" in the labelled corpus.
//
// It is deliberately generous (18). The failure direction that follows is stated in
// the README and measured by the accuracy test: this classifier UNDER-counts relays,
// so the emitted relay ratio is a FLOOR. A diagnostic that flatters the operator is
// the safe direction for a number nobody may use as a scorecard.
const relayMaxWords = 18

// duplicateJaccard is the token-overlap threshold above which a message counts as a
// re-send of an earlier one in the same session. Re-sends were the retro's clearest
// relay shape — the operator repeating himself because the first send did not land.
const duplicateJaccard = 0.9

// Cue families. Each is one FAMILY of relay, kept separate so a future reader can see
// which kind of plumbing dominates without re-deriving it from the text (which is
// gone by then). Patterns run against the NORMALISED form: lower-cased, punctuation
// stripped, whitespace collapsed.
var (
	// cueSync — "get the tree current", the single most-repeated instruction.
	cueSync = regexp.MustCompile(`\b(sync to main|resync|re sync|get current|current with main|merge main|pull main|update your branch|fetch and merge)\b`)
	// cueState — the operator echoing a state an agent could have read itself.
	cueState = regexp.MustCompile(`\b(is merged|has merged|now merged|it is merged|its merged|landed|is green|ci is green|ci green|is approved|has been approved|is ready|pushed|is closed|has landed)\b`)
	// cluePoke — a content-free nudge to continue.
	cuePoke = regexp.MustCompile(`^(ok|okay|k|kk|yes|yep|yeah|y|ta|thanks|thank you|ty|go|go ahead|continue|carry on|proceed|next|resume|keep going|carry|again|and|and now|do it|please continue|status|any update|update)$`)
	// cueLookup — the operator asking for state instead of the state being surfaced.
	cueLookup = regexp.MustCompile(`\b(where is|where are|whats the status|what is the status|status of|which pr|what happened to|did you (push|open|file|land)|is it done|are we done|how many left|whats left|what is left)\b`)
	// cueCorrective — a behavioural correction. NOT a relay axis: a correction can be
	// substantive. It is tracked separately for correction-recurrence candidates.
	cueCorrective = regexp.MustCompile(`^(no|nope|not that|stop|dont|do not|never|wrong|i told you|i said|read the|you should|you were told|not like that)\b`)
)

// cueFamily names, emitted as COUNTS only (fixed keys, no dynamic strings).
const (
	familySync      = "sync"
	familyState     = "state_echo"
	familyPoke      = "poke"
	familyLookup    = "lookup"
	familyDuplicate = "duplicate"
)

// ── v2: the ATTENTION-CLASS axis ────────────────────────────────────────────
//
// A SECOND label, independent of the relay-vs-substantive verdict above. Where
// the relay axis asks "did this turn carry a decision or plumbing?", the
// attention axis asks WHICH KIND of operator load a turn represents — the
// vocabulary the stream's targets are stated in. It is ADDITIVE: it never
// changes a message's Class or relay Family, it only adds one more count.
//
// It is still a JUDGEMENT (hence the ClassifierVersion bump), and it is still
// COUNT-ONLY: AttentionFamily returns one fixed vocabulary string, never text.
// First match wins, in the order the switch runs. Rules match on the NORMALISED
// form, exactly as the relay cues do.
//
// The length ceiling (relayMaxWords) applies to route/status/toil for the same
// reason it applies to the relay cues: a long message that merely MENTIONS a
// routing or status phrase is a decision that mentions plumbing, not plumbing —
// so it falls through to `other`, preserving the "under-count, never over-count"
// direction the README commits to. correction/decision/idea carry no ceiling
// (a correction or a research ask can legitimately be long); ack is bounded by
// its own ≤3-word rule.
const (
	AttnRoute      = "route"      // "tell/ask/find/ping <desk>", "wrong window"
	AttnStatus     = "status"     // "where are we", "what's next", "status of"
	AttnToil       = "toil"       // "walk me through", "step by step", "give me the command"
	AttnCorrection = "correction" // the corrective-cue rule, promoted to a family
	AttnDecision   = "decision"   // option letters/numbers, "ratify", "approve <it/the>"
	AttnIdea       = "idea"       // "investigate", "research", "compare", "evaluate"
	AttnAck        = "ack"        // ≤3 words: yes/ok/go/done/merged/retry/continue
	AttnOther      = "other"      // the substantive / uncued default
)

// AllAttentionFamilies is the closed set, in emit order, exported for the emit
// tally and the vocabulary tests.
var AllAttentionFamilies = []string{
	AttnRoute, AttnStatus, AttnToil, AttnCorrection, AttnDecision, AttnIdea, AttnAck, AttnOther,
}

var (
	// route — the operator hand-carrying a message between desks/windows. The verb
	// is one of the brief's four imperatives (tell/ask/find/ping) AND must actually
	// reach a desk/window word, so ordinary uses ("find the bug") and substantive
	// rules that merely mention routing ("route that verdict to the review desk")
	// do not trip it. Deliberately NOT including "route"/"hand" as verbs: those
	// appear inside substantive governance sentences far more than as an operator's
	// routing instruction, and the under-count direction says leave them `other`.
	attnRoute = regexp.MustCompile(`\bwrong window\b|\b(tell|ask|find|ping)\b[a-z0-9 #]*\b(desk|window)\b`)
	// status — the operator pulling state that an agent could have surfaced.
	attnStatus = regexp.MustCompile(`\b(where are we|what ?s next|what is next|what ?s left|what is left|status of|update the runsheet|any update|hows it going|how is it going|is it done|are we done|is that done|whats the status|what is the status)\b`)
	// toil — the operator asking to be walked through mechanics a routine owns.
	// Only operator-facing "give me / walk me" phrasings; a substantive instruction
	// to DOCUMENT a command ("document the exact command") is not toil, so bare
	// "exact command" is deliberately absent.
	attnToil = regexp.MustCompile(`\b(walk me through|step by step|give me the command|how do i|what ?s the command|what is the command|which command|spell out the steps|paste the command)\b`)
	// decision — a ratify/approve/pick-an-option turn. Deliberately NARROW on
	// "approve": it must be an approving ACTION ("approve it/the/this", "i
	// approve"), never the bare word or a passive state echo ("it is approved"),
	// so a substantive rule ABOUT approvals ("must never self-approve") stays
	// `other`.
	attnDecision = regexp.MustCompile(`\b(ratif\w*|option [0-9a-z]\b|go with option|approve (it|the|this|that)|i approve|lets go with option|the decision is)\b`)
	// idea — a research/compare/evaluate ask: work that opens a question rather
	// than closing one.
	attnIdea = regexp.MustCompile(`\b(investigate|research|compare|evaluate|explore|look into|dig into|assess whether|what can we learn)\b`)
	// ack — a bare acknowledgement. Bounded to ≤3 tokens by AttentionFamily and
	// matched whole here so "ok" acks but "ok, now rewrite the parser" does not.
	attnAck = regexp.MustCompile(`^(yes|yep|yeah|ok|okay|k|kk|go|go ahead|done|merged|retry|continue|proceed|next|resume|carry on|keep going|sure|ta|ty|thanks|thank you|ship it)$`)
)

// attnMaxWords is the length ceiling for the cue-based attention families
// (route/status/toil), reusing relayMaxWords so the two axes cannot drift apart:
// the direction the README commits to is enforced once, for both.
const attnMaxWords = relayMaxWords

// AttentionFamily assigns one attention class to a normalised message. It is a
// pure function of the text (no session state), returns "" for a content-free
// turn (so the empty turns stay OUT of the family denominator, exactly as they
// stay out of the relay ratio), and otherwise returns one of AllAttentionFamilies.
func AttentionFamily(norm string, ntoks int) string {
	if ntoks == 0 {
		return ""
	}
	if ntoks <= attnMaxWords {
		switch {
		case attnRoute.MatchString(norm):
			return AttnRoute
		case attnStatus.MatchString(norm):
			return AttnStatus
		case attnToil.MatchString(norm):
			return AttnToil
		}
	}
	switch {
	case cueCorrective.MatchString(norm):
		return AttnCorrection
	case attnDecision.MatchString(norm):
		return AttnDecision
	case attnIdea.MatchString(norm):
		return AttnIdea
	case ntoks <= 3 && attnAck.MatchString(norm):
		return AttnAck
	}
	return AttnOther
}

// wordRe splits the normalised form into word tokens.
var wordRe = regexp.MustCompile(`[a-z0-9#]+`)

// nonWord is everything normalisation flattens to a space. Markdown punctuation,
// backticks, quotes and PR/issue sigils all go, so `"sync to main"`, `sync to main.`
// and **sync to main** normalise identically.
var nonWord = regexp.MustCompile(`[^a-z0-9#]+`)

// apostrophes are DELETED rather than flattened to a space, so "it's merged" becomes
// "its merged" and matches the same cue as the contraction-free spelling. Measured:
// treating them like other punctuation cost four points of relay recall on the
// labelled corpus, because half the state echoes in real traffic are contractions.
var apostrophes = strings.NewReplacer("'", "", "’", "", "ʼ", "")

// normalise lower-cases, strips punctuation and collapses whitespace. It is the only
// transform applied before matching, and it is intentionally lossy: the classifier
// should not be able to tell two differently-punctuated pokes apart.
func normalise(s string) string {
	s = strings.ToLower(s)
	s = apostrophes.Replace(s)
	s = nonWord.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func tokens(norm string) []string { return wordRe.FindAllString(norm, -1) }

// tokenSet is the bag used for near-duplicate detection.
func tokenSet(toks []string) map[string]struct{} {
	m := make(map[string]struct{}, len(toks))
	for _, t := range toks {
		m[t] = struct{}{}
	}
	return m
}

// jaccard is |A∩B| / |A∪B|. Two empty sets are NOT similar (0), so an empty message
// can never be a "duplicate" of another empty one.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// Label is the classifier's verdict on one message. It carries NO text — by
// construction, so that no caller can accidentally forward content into the emit.
type Label struct {
	Class Class
	// Family is the relay family when Class is ClassRelay, "" otherwise. It is one
	// of the family* constants — a fixed vocabulary, never message-derived.
	Family string
	// Attention is the v2 attention class — one of AllAttentionFamilies, or "" for
	// a content-free turn. A SECOND axis, independent of Class/Family: every
	// non-empty message carries exactly one, so the eight family counts sum to
	// messages_classified.
	Attention string
	// Corrective reports the corrective-cue axis, independent of Class.
	Corrective bool
	// Shape is a content-free fingerprint of the normalised token SET, used only to
	// group correction-recurrence candidates. It is a sorted token list held in
	// memory and never emitted; see correctionCandidates.
	shape map[string]struct{}
}

// Classifier labels operator messages in arrival order, per session. It is stateful
// because near-duplicate detection needs the session's earlier messages — and only
// their token SETS, which is all it retains.
type Classifier struct {
	seen map[string][]map[string]struct{} // session → earlier token sets
}

func NewClassifier() *Classifier {
	return &Classifier{seen: map[string][]map[string]struct{}{}}
}

// Classify labels one message. session scopes duplicate detection; a repeat across
// two different sessions is not a re-send, it is the same instruction to two agents,
// which is a different (and legitimate) thing.
func (c *Classifier) Classify(session, text string) Label {
	norm := normalise(text)
	toks := tokens(norm)
	set := tokenSet(toks)

	if len(toks) == 0 {
		return Label{Class: ClassEmpty, shape: set}
	}

	corrective := cueCorrective.MatchString(norm)
	// The attention class is computed once, on the same normalised form, and rides
	// on every label this call returns. It is independent of the relay verdict —
	// a duplicate re-send still carries its own attention class.
	attn := AttentionFamily(norm, len(toks))

	// Near-duplicate FIRST: a re-send is a relay regardless of how long it is,
	// because its information content is zero — it was already said.
	for _, prev := range c.seen[session] {
		if jaccard(set, prev) >= duplicateJaccard {
			c.seen[session] = append(c.seen[session], set)
			return Label{Class: ClassRelay, Family: familyDuplicate, Attention: attn, Corrective: corrective, shape: set}
		}
	}
	c.seen[session] = append(c.seen[session], set)

	if len(toks) > relayMaxWords {
		return Label{Class: ClassSubstantive, Attention: attn, Corrective: corrective, shape: set}
	}

	switch {
	case cuePoke.MatchString(norm):
		return Label{Class: ClassRelay, Family: familyPoke, Attention: attn, Corrective: corrective, shape: set}
	case cueSync.MatchString(norm):
		return Label{Class: ClassRelay, Family: familySync, Attention: attn, Corrective: corrective, shape: set}
	case cueState.MatchString(norm):
		return Label{Class: ClassRelay, Family: familyState, Attention: attn, Corrective: corrective, shape: set}
	case cueLookup.MatchString(norm):
		return Label{Class: ClassRelay, Family: familyLookup, Attention: attn, Corrective: corrective, shape: set}
	}
	return Label{Class: ClassSubstantive, Attention: attn, Corrective: corrective, shape: set}
}

// correctionRecurrenceJaccard groups two corrective messages as "the same correction".
// It is looser than duplicate detection (0.6 vs 0.9) because the operator rarely
// repeats a correction verbatim — he rephrases it, which is exactly why it recurs.
const correctionRecurrenceJaccard = 0.6

// correctionCandidates counts GROUPS of corrective messages that recur — the same
// behavioural rule corrected more than once in the window. It returns a count, never
// the corrections themselves.
//
// It is deliberately called a CANDIDATE count. The brief says report candidates and
// do not over-claim, and this heuristic cannot distinguish "the same rule, twice"
// from "two different rules that share vocabulary". A reader is meant to treat a
// non-zero here as a prompt to go look, never as a finding.
func correctionCandidates(labels []Label) int {
	var groups []map[string]struct{}
	counts := map[int]int{}
	for _, l := range labels {
		if !l.Corrective || len(l.shape) == 0 {
			continue
		}
		placed := false
		for i, g := range groups {
			if jaccard(l.shape, g) >= correctionRecurrenceJaccard {
				counts[i]++
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, l.shape)
			counts[len(groups)-1] = 1
		}
	}
	n := 0
	for _, c := range counts {
		if c > 1 {
			n++
		}
	}
	return n
}
