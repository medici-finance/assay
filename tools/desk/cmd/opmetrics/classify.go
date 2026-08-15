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
//	1 — first cut (methodology-metrics/40): cue families + near-duplicate detection,
//	    measured against cmd/opmetrics/testdata/labelled. See README for the score.
const ClassifierVersion = "opmetrics-relay/1"

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

	// Near-duplicate FIRST: a re-send is a relay regardless of how long it is,
	// because its information content is zero — it was already said.
	for _, prev := range c.seen[session] {
		if jaccard(set, prev) >= duplicateJaccard {
			c.seen[session] = append(c.seen[session], set)
			return Label{Class: ClassRelay, Family: familyDuplicate, Corrective: corrective, shape: set}
		}
	}
	c.seen[session] = append(c.seen[session], set)

	if len(toks) > relayMaxWords {
		return Label{Class: ClassSubstantive, Corrective: corrective, shape: set}
	}

	switch {
	case cuePoke.MatchString(norm):
		return Label{Class: ClassRelay, Family: familyPoke, Corrective: corrective, shape: set}
	case cueSync.MatchString(norm):
		return Label{Class: ClassRelay, Family: familySync, Corrective: corrective, shape: set}
	case cueState.MatchString(norm):
		return Label{Class: ClassRelay, Family: familyState, Corrective: corrective, shape: set}
	case cueLookup.MatchString(norm):
		return Label{Class: ClassRelay, Family: familyLookup, Corrective: corrective, shape: set}
	}
	return Label{Class: ClassSubstantive, Corrective: corrective, shape: set}
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
