// Package draft is the composing half of the Ask Assay pane (desk-console-2/02,
// design doc §7.5 phase 2). It turns computed claims into a DRAFT a human
// reads, and it has no way at all to turn a draft into a post.
//
// THE INVARIANT
// -------------
// "Never auto-posts — this is an invariant, not a setting." A draft's only
// exit is a human. That sentence is easy to write and easy to lose, so this
// package spends its whole design on making the loss impossible rather than
// discouraged:
//
//  1. NOTHING HERE CAN PERFORM I/O. The package's entire import set is
//     computation: hashing, formatting, regexp, sorting, string handling.
//     There is no os, no io, no net, no os/exec, no syscall — so there is no
//     subprocess to run, no socket to open and no file to write. This is
//     checked as a closed allow-list over the package's own parsed imports
//     (TestAuthorityImportsAreAClosedComputationOnlySet), so ADDING the
//     capability is a red test in the same commit that adds it. A layer that
//     could post but chooses not to is a flag; the gap between a flag and a
//     grant is one commit. This layer cannot post.
//
//  2. THE HAND-OFF NAMES ITS TARGETS AND REFUSES EVERY TRANSMITTING ONE.
//     See handoff.go. Every destination that would transmit is refused
//     unconditionally, BEFORE any check of whether it exists — so a target
//     becoming real can never turn a refusal into a permission.
//
//  3. A DRAFT CANNOT STATE A NUMBER IT DID NOT CITE. Figures enter a body
//     only through {{figure:<question-id>}}, resolved from a [Claim] the
//     caller attached. A bare numeral anywhere else in the template is
//     REFUSED. An uncitable literal has one declared escape that costs a
//     written reason.
//
//  4. AN UNCOMPUTABLE FIGURE RENDERS could-not-check, NEVER 0. The resolver
//     reads [Claim.Value]'s ok, not its int64. A draft that quotes "0" for a
//     probe that came back blind is a falsehood with a human's name about to
//     go on it.
//
//  5. CAVEATS TRAVEL WITH THE CLAIM, BY CLASS, NOT BY HAND. Every attached
//     claim's own caveats are lifted into the draft mechanically. A claim
//     that carries none is REFUSED, because an unqualified number is the one
//     a reader trusts most.
//
// WHY THERE IS NO KILL SWITCH HERE
// --------------------------------
// A stop flag on a layer that cannot act is decoration, and decoration on a
// safety surface is worse than nothing: it invites the reader to believe an
// arming mechanism exists. There is a live instance of the failure this
// avoids in the neighbouring suite — internal/deskkit/killswitch.go builds its
// per-loop stop-flag name by concatenating an environment variable onto a
// prefix with NO allow-list, so renaming the loop leaves a held stop flag
// inert with nothing failing loudly. This package holds no flag of that shape,
// because it holds no flag: what it refuses, it refuses in source.
//
// THE BOUND ON THE CLAIM, STATED
// ------------------------------
// "This package cannot post" is a statement about THIS package. It is not a
// statement about its caller. A caller that takes [Draft.Render]'s string and
// hands it to a posting tool has posted — this layer cannot prevent that and
// does not claim to. What it guarantees is that no such step is contained
// here, that the hand-off surface names every transmitting target and refuses
// it, and that acquiring the capability is a visible, test-reddening diff.
package draft

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Claim is the ONE thing this package accepts as the origin of a figure. It is
// deliberately the exact method subset of the read-only answer layer's
// rendered answer, and it is an interface rather than an import for a
// load-bearing reason: that layer runs guarded subprocesses, so importing it
// would pull os/exec into this package's transitive closure and destroy the
// one-line proof in rule (1) above. Go satisfies interfaces implicitly, so the
// answer layer needs no adapter and no change to be usable here.
type Claim interface {
	// Question is the registry ID of the thing that was asked.
	Question() string
	// Render is the claim's full provenance line: source command, probe,
	// window, limit and as-of stamp. This is what a provenance chip shows.
	Render() string
	// Caveats are the standing qualifications of the claim's class.
	Caveats() []string
	// Value is the figure and whether one exists. ok=false means no number
	// exists — which is a state, not a zero.
	Value() (int64, bool)
}

// FigureUnavailable is what a figure position renders when no number exists.
// It is a constant here, and it deliberately equals the token the answer layer
// renders, so that a draft and the pane it quotes cannot disagree about how
// "no number" looks. It is duplicated rather than imported for the reason
// given on [Claim]; TestFigureTokenIsNotADigit pins it.
const FigureUnavailable = "could-not-check"

// Kind is what a draft proposes. There are two, from §7.5 phase 2: an answer
// to an escalation, and a priority change. A kind outside this set is refused
// rather than defaulted — defaulting a kind is how a priority change gets
// composed with an answer's provenance rules.
type Kind string

const (
	// KindAnswer proposes an answer to an escalation.
	KindAnswer Kind = "proposed-answer"
	// KindPriority proposes a change to what gets worked next.
	KindPriority Kind = "proposed-priority-change"
)

func (k Kind) valid() bool { return k == KindAnswer || k == KindPriority }

// Target names what a draft is ABOUT. It is structured rather than free text
// so that an object reference — which is mostly digits — never has to appear
// as a bare numeral inside a body, where it would be indistinguishable from an
// uncited figure.
type Target struct {
	// Repo is owner/repo.
	Repo string
	// Kind is the object class: issue, pull-request or stream.
	Kind string
	// Ref is the object's identifier as written.
	Ref string
}

var targetKinds = map[string]bool{"issue": true, "pull-request": true, "stream": true}

// Validate reports why a target cannot be drafted against.
func (t Target) Validate() error {
	if strings.TrimSpace(t.Repo) == "" {
		return fmt.Errorf("%w: target names no repo", ErrMalformed)
	}
	if !targetKinds[t.Kind] {
		return fmt.Errorf("%w: target kind %q is not one of issue, pull-request, stream", ErrMalformed, t.Kind)
	}
	if strings.TrimSpace(t.Ref) == "" {
		return fmt.Errorf("%w: target names no ref", ErrMalformed)
	}
	return nil
}

func (t Target) String() string { return t.Repo + " " + t.Kind + " " + t.Ref }

// The refusal classes. Each is a distinct thing a composer got wrong, because
// one undifferentiated error teaches the caller to retry rather than to fix.
var (
	// ErrMalformed is a draft that is structurally incomplete.
	ErrMalformed = errors.New("refused: malformed draft")

	// ErrUncitedFigure is a numeral in a body with no claim behind it. This is
	// the numbers rule crossing into drafting: the model narrates, the index
	// computes, and a draft is where a narrated number would first acquire a
	// human's authority.
	ErrUncitedFigure = errors.New("refused: a figure with no cited claim behind it")

	// ErrUncaveatedClaim is a cited claim carrying no caveat. Caveats attach
	// by class in the answer layer, so a claim with none has either escaped
	// its class or been hand-assembled — both are reasons not to quote it.
	ErrUncaveatedClaim = errors.New("refused: a cited claim carrying no caveat")

	// ErrUnquotedCitation is a claim attached but never quoted. Provenance
	// chips are read as evidence of rigour, so an unquoted citation inflates
	// how well sourced a draft looks. Citation and quotation are a bijection.
	ErrUnquotedCitation = errors.New("refused: a citation the body never quotes")
)

// Draft is one composed proposal. Every field is unexported and there is no
// setter: a draft is what [Compose] validated, or it does not exist. In
// particular there is no field, option or method by which a draft can be
// marked "approved", "authorized" or "ready to send" — this package ships no
// authorization type at all, deliberately. A satisfied authorization plus a
// transmitting code path is a post; dormancy is acceptable for a mechanism
// whose worst case is a wrong recommendation, and not for one whose worst case
// is an unattended post.
type Draft struct {
	kind    Kind
	target  Target
	title   string
	body    string
	cited   []string
	chips   []string
	caveats []string
}

// placeholder matches the two declared ways content enters a body.
var placeholder = regexp.MustCompile(`\{\{(figure|lit):([^{}]*)\}\}`)

// bareNumeral is what a body may not contain outside a placeholder.
var bareNumeral = regexp.MustCompile(`[0-9]`)

// Compose builds a draft, or refuses and says which rule was broken. It never
// returns a partially valid draft alongside an error: a caller that ignores
// the error gets the zero Draft, which renders nothing.
//
// The template carries prose plus two placeholder forms:
//
//	{{figure:<question-id>}}   resolves from the cited claim of that ID
//	{{lit:<text>|<why>}}       a literal, at the price of a written reason
//
// Any digit outside those forms is refused.
func Compose(k Kind, t Target, title, template string, cites ...Claim) (Draft, error) {
	if !k.valid() {
		return Draft{}, fmt.Errorf("%w: %q is not a declared draft kind (%s, %s)", ErrMalformed, k, KindAnswer, KindPriority)
	}
	if err := t.Validate(); err != nil {
		return Draft{}, err
	}
	if strings.TrimSpace(title) == "" {
		return Draft{}, fmt.Errorf("%w: draft has no title", ErrMalformed)
	}
	if strings.TrimSpace(template) == "" {
		return Draft{}, fmt.Errorf("%w: draft has no body", ErrMalformed)
	}

	byID := map[string]Claim{}
	var order []string
	for i, c := range cites {
		if c == nil {
			return Draft{}, fmt.Errorf("%w: citation %d is nil", ErrMalformed, i+1)
		}
		id := strings.TrimSpace(c.Question())
		if id == "" {
			return Draft{}, fmt.Errorf("%w: citation %d names no question", ErrMalformed, i+1)
		}
		if _, dup := byID[id]; dup {
			return Draft{}, fmt.Errorf("%w: %s is cited twice — one number, one source", ErrMalformed, id)
		}
		if len(c.Caveats()) == 0 {
			return Draft{}, fmt.Errorf("%w: %s carries no caveat, so nothing would qualify it in the draft", ErrUncaveatedClaim, id)
		}
		if strings.TrimSpace(c.Render()) == "" {
			return Draft{}, fmt.Errorf("%w: %s renders no provenance line, so its chip would be blank", ErrMalformed, id)
		}
		byID[id] = c
		order = append(order, id)
	}

	quoted := map[string]bool{}
	var resolveErr error
	body := placeholder.ReplaceAllStringFunc(template, func(m string) string {
		parts := placeholder.FindStringSubmatch(m)
		form, arg := parts[1], parts[2]
		if form == "lit" {
			text, why, ok := strings.Cut(arg, "|")
			if !ok || strings.TrimSpace(why) == "" {
				if resolveErr == nil {
					resolveErr = fmt.Errorf("%w: the literal %q states no reason — a literal with a reason is reviewable, a bare one is not", ErrUncitedFigure, strings.TrimSpace(text))
				}
				return m
			}
			return strings.TrimSpace(text)
		}
		id := strings.TrimSpace(arg)
		c, ok := byID[id]
		if !ok {
			if resolveErr == nil {
				resolveErr = fmt.Errorf("%w: the body quotes %q but no claim of that ID was cited", ErrUncitedFigure, id)
			}
			return m
		}
		quoted[id] = true
		return figureOf(c)
	})
	if resolveErr != nil {
		return Draft{}, resolveErr
	}

	// The numeral scan runs on what is LEFT after the placeholders resolved to
	// their own text, so it is run against the template with every declared
	// form removed — not against the rendered body, which legitimately holds
	// the digits the placeholders produced.
	stripped := placeholder.ReplaceAllString(template, "")
	if loc := bareNumeral.FindStringIndex(stripped); loc != nil {
		return Draft{}, fmt.Errorf("%w: the body states a numeral (%q) outside any citation. Figures enter through {{figure:<question-id>}}; a reference belongs in the Target; anything else needs {{lit:<text>|<why>}} and its reason",
			ErrUncitedFigure, snippet(stripped, loc[0]))
	}

	for _, id := range order {
		if !quoted[id] {
			return Draft{}, fmt.Errorf("%w: %s is attached but never quoted — a provenance chip for a figure the reader cannot find is decoration", ErrUnquotedCitation, id)
		}
	}

	d := Draft{kind: k, target: t, title: strings.TrimSpace(title), body: body, cited: order}
	seen := map[string]bool{}
	for _, id := range order {
		c := byID[id]
		d.chips = append(d.chips, c.Render())
		for _, cv := range c.Caveats() {
			if cv == "" || seen[cv] {
				continue
			}
			seen[cv] = true
			d.caveats = append(d.caveats, cv)
		}
	}
	return d, nil
}

// figureOf is the only place a claim becomes text in a body. It reads ok, not
// the int64 — so an unavailable figure renders the could-not-check token and
// there is no branch that can produce a digit for it.
func figureOf(c Claim) string {
	v, ok := c.Value()
	if !ok {
		return FigureUnavailable
	}
	return fmt.Sprintf("%d", v)
}

func snippet(s string, at int) string {
	lo := at - 12
	if lo < 0 {
		lo = 0
	}
	hi := at + 12
	if hi > len(s) {
		hi = len(s)
	}
	return strings.TrimSpace(s[lo:hi])
}

// Zero reports whether this is the empty draft — what a refused [Compose]
// returns, and what a refused [Draft.HandOff] leaves a caller holding.
func (d Draft) Zero() bool { return d.title == "" && d.body == "" }

// Kind reports what the draft proposes.
func (d Draft) Kind() Kind { return d.kind }

// Target reports what the draft is about.
func (d Draft) Target() Target { return d.target }

// Title reports the draft's title.
func (d Draft) Title() string { return d.title }

// Cited reports the question IDs quoted in the body, in citation order.
func (d Draft) Cited() []string { return append([]string(nil), d.cited...) }

// Caveats reports the union of the cited claims' caveats, deduplicated, in
// first-seen order.
func (d Draft) Caveats() []string { return append([]string(nil), d.caveats...) }

// DraftBanner is the first line of every rendered draft. It is a constant so a
// downstream renderer can key on it rather than pattern-match prose, and so a
// test can assert no render path omits it.
const DraftBanner = "DRAFT — NOT AN ACTION — nothing here has been posted, and this layer cannot post it"

// Render is the draft as a human reads it: the banner, what it proposes, the
// body, one provenance chip per cited claim, the caveats those claims carry,
// and the exit statement. The zero Draft renders a refusal notice rather than
// an empty document, because an empty draft on a screen looks like a draft
// with nothing to say instead of a draft that does not exist.
//
// The separator is " · " and never "|": these lines get pasted into markdown
// tables, and a raw pipe shreds the table it lands in.
func (d Draft) Render() string {
	if d.Zero() {
		return DraftBanner + "\nstate: " + FigureUnavailable +
			"\nreason: no draft was composed — this is the empty value, not an empty draft"
	}
	var b strings.Builder
	b.WriteString(DraftBanner + "\n")
	fmt.Fprintf(&b, "proposes: %s · about: %s\n", d.kind, d.target)
	fmt.Fprintf(&b, "title: %s\n\n%s\n", d.title, d.body)
	b.WriteString("\nprovenance:\n")
	if len(d.chips) == 0 {
		b.WriteString("  (none — this draft quotes no figure)\n")
	}
	for _, c := range d.chips {
		fmt.Fprintf(&b, "  - %s\n", strings.ReplaceAll(c, "\n", "\n    "))
	}
	if len(d.caveats) > 0 {
		b.WriteString("\ncaveats carried by the claims above:\n")
		for _, c := range d.caveats {
			fmt.Fprintf(&b, "  - %s\n", c)
		}
	}
	b.WriteString("\nexit: " + ExitStatement + "\n")
	return b.String()
}

// ExitStatement is the footer every draft carries. It is the §6.2 rule in the
// place a reader is standing when they would otherwise reach for a send
// button.
const ExitStatement = "an agent may propose; only the human commits. This draft leaves this layer " +
	"by being read. Every transmitting hand-off destination is refused unconditionally — see Destinations()."

// Questions returns the cited question IDs sorted, for a caller that needs a
// stable set rather than citation order.
func (d Draft) Questions() []string {
	out := d.Cited()
	sort.Strings(out)
	return out
}
