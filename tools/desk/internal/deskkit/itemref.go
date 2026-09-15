package deskkit

// itemref.go — the ONE parser for a reference to a numbered forge object.
//
// THE PROBLEM IT EXISTS FOR. GitHub numbers issues and pull requests in a SINGLE per-repo
// sequence, so `#7` addresses exactly one object and a bare number is unambiguous. GitLab
// numbers issues and merge requests in SEPARATE per-project sequences, so `#7` and `!7`
// routinely both exist and are different objects. Every desk verb that takes a number
// therefore has to be told which kind is meant on a GitLab project, and the fail-closed
// refusals the read layer raises (GetIssue's both-kinds refusal) point the caller at "the
// typed operation for the kind you mean" — an operation each verb has had to grow its own
// spelling of.
//
// Four verbs hit that wall in turn, and the fix belongs HERE rather than in each of them: a
// typed reference the desk verbs accept uniformly, parsed in one place, so a reference that
// resolves for one verb resolves for all of them and the grammar cannot drift between them.
//
// THE GRAMMAR, and why `#` does NOT mean "issue":
//
//	!N              a CHANGE  — a merge request (GitLab) or pull request (GitHub)
//	owner/repo!N    the same, in another repository
//	N  ·  #N        kind UNSTATED — the forge resolves it
//	owner/repo#N    the same, in another repository
//	<web URL>       the kind the URL's own path states: `…/issues/N` is an issue,
//	                `…/pull/N` and `…/-/merge_requests/N` are changes
//
// `#N` is deliberately NEUTRAL rather than "an issue". On GitHub — the forge most adopters
// run — `#7` is the ordinary way to write a PULL REQUEST reference, and reading the sigil as
// "issue" would refuse a reference that resolves correctly today and has resolved correctly
// for every caller that ever wrote one. So `#N` keeps exactly the meaning it already has: the
// number, kind unsaid. Where that is not enough — a GitLab project carrying both — the caller
// states the kind, either with `!` for a change or with the verb's `--kind issue` flag.
//
// A reference that states a kind is an ASSERTION, not a hint: the typed read validates it
// (GetIssueTyped refuses when the object at that number is the other kind) rather than
// quietly handing back whatever was there.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ItemRef is one parsed reference to a numbered object on a forge.
//
// Kind is EMPTY when the reference did not state one — the zero value means "unstated",
// never "issue". A caller resolves an empty Kind by asking the forge (the untyped read,
// which is correct wherever the forge has one number sequence and refuses where it does
// not), or by taking the kind from a flag the operator set.
type ItemRef struct {
	// Repo is "owner/name" when the reference carried one, else empty — the caller defaults
	// an empty Repo to whatever repository it is operating on.
	Repo string
	// Number is the object's number, always positive.
	Number int
	// Kind is the kind the reference STATED, or "" when it stated none.
	Kind TargetKind
}

// KindStated reports whether the reference said which kind of object it addresses.
func (r ItemRef) KindStated() bool { return r.Kind != "" }

// String renders the reference in its canonical short form — `!N` for a change, `#N`
// otherwise, prefixed with the repository when one is carried. A reference whose kind is
// unstated renders with `#`, which is the neutral sigil, not a claim that it is an issue.
func (r ItemRef) String() string {
	sigil := "#"
	if r.Kind == TargetChange {
		sigil = "!"
	}
	if r.Repo == "" {
		return fmt.Sprintf("%s%d", sigil, r.Number)
	}
	return fmt.Sprintf("%s%s%d", r.Repo, sigil, r.Number)
}

// itemRefShortRe matches the short forms: an optional `owner/repo`, an optional `#`/`!`
// sigil, and the number.
var itemRefShortRe = regexp.MustCompile(`^(?:([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+))?([#!]?)([0-9]+)$`)

// itemRefURLRe matches a forge web URL for one numbered object: everything between the host
// and the kind segment is the project path, and the kind segment names the kind. GitLab
// spells a project-scoped path with a `/-/` separator; GitHub does not.
var itemRefURLRe = regexp.MustCompile(`^https?://[^/]+/(.+?)/(?:-/)?(issues|pull|pulls|merge_requests)/([0-9]+)/?$`)

// ParseItemRef parses one item reference. It is the single home of the grammar this file's
// header documents; no desk verb re-implements it.
//
// An unparseable reference is REFUSED naming the accepted forms, never resolved to a guess:
// a verb that guesses which object a reference names acts on the wrong one, which is the
// whole failure the typed forms exist to prevent.
func ParseItemRef(s string) (ItemRef, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ItemRef{}, Refused("refused: an empty item reference names nothing — " + TypedRefForms())
	}

	if m := itemRefURLRe.FindStringSubmatch(trimmed); m != nil {
		n, err := parseItemRefNumber(trimmed, m[3])
		if err != nil {
			return ItemRef{}, err
		}
		kind := TargetChange
		if m[2] == "issues" {
			kind = TargetIssue
		}
		// The project path between the host and the kind segment must reduce to the
		// `owner/name` coordinate a ForgeRepo carries. A GitLab project nested more than one
		// group deep does not, and picking two of its segments would address a DIFFERENT
		// project that may well exist — so it is a refusal naming the shortfall rather than a
		// silent re-target.
		segs := strings.Split(strings.Trim(m[1], "/"), "/")
		switch len(segs) {
		case 2:
			return ItemRef{Repo: segs[0] + "/" + segs[1], Number: n, Kind: kind}, nil
		default:
			return ItemRef{}, Refused(fmt.Sprintf(
				"refused: cannot read an owner/repo coordinate out of %s — its project path is %q, which is "+
					"not the two segments a repository coordinate carries. Address it as owner/repo%s%d, or as "+
					"a bare %s%d against the repository the verb was given.",
				StripControl(trimmed), StripControl(m[1]), sigilFor(kind), n, sigilFor(kind), n))
		}
	}

	m := itemRefShortRe.FindStringSubmatch(trimmed)
	if m == nil {
		return ItemRef{}, Refused(fmt.Sprintf(
			"refused: cannot read %s as an item reference — %s", StripControl(trimmed), TypedRefForms()))
	}
	n, err := parseItemRefNumber(trimmed, m[3])
	if err != nil {
		return ItemRef{}, err
	}
	out := ItemRef{Repo: m[1], Number: n}
	if m[2] == "!" {
		out.Kind = TargetChange
	}
	return out, nil
}

// requireNoReasonOnChange is the shared precondition both backends' CloseIssueTyped applies:
// an unknown kind is refused, and a state reason asked for on a CHANGE is refused rather
// than dropped. Neither forge records a state reason on a change — GitHub's `state_reason`
// is an issue field and GitLab has no state reason at all — so a caller that passed one
// would otherwise be told the close succeeded while the distinction it asked for went
// nowhere. The lane that a close belongs to is carried by the comment written before it.
func requireNoReasonOnChange(repo ForgeRepo, number int, kind TargetKind, stateReason string) error {
	switch kind {
	case TargetIssue:
		return nil
	case TargetChange:
		if strings.TrimSpace(stateReason) != "" {
			return Refused(fmt.Sprintf(
				"refused: CloseIssueTyped was asked to close the change %s!%d with state reason %q, but no "+
					"forge records a state reason on a change — close it with an empty reason and let the "+
					"comment posted before the close carry the lane",
				repo.Slug(), number, StripControl(stateReason)))
		}
		return nil
	default:
		return Refused(fmt.Sprintf("refused: CloseIssueTyped: unknown target kind %q for %s#%d",
			string(kind), repo.Slug(), number))
	}
}

// kindNoun renders a kind as the forge-neutral noun a message names it by. An unstated kind
// renders as "object of an unstated kind" rather than defaulting to either, so a message
// built from it cannot claim a kind the caller never gave.
func kindNoun(kind TargetKind) string {
	switch kind {
	case TargetIssue:
		return "issue"
	case TargetChange:
		return "change (pull request / merge request)"
	default:
		return "object of an unstated kind"
	}
}

// sigilFor renders the short-form sigil a kind is written with.
func sigilFor(kind TargetKind) string {
	if kind == TargetChange {
		return "!"
	}
	return "#"
}

// parseItemRefNumber turns the digits out of a reference into a positive number. Zero is
// refused: no forge numbers an object 0, so a `#0` is a malformed reference rather than an
// object nobody has filed yet.
func parseItemRefNumber(ref, digits string) (int, error) {
	n, err := strconv.Atoi(digits)
	if err != nil || n <= 0 {
		return 0, Refused(fmt.Sprintf(
			"refused: %s names item number %q, which is not a positive number",
			StripControl(ref), StripControl(digits)))
	}
	return n, nil
}

// TypedRefForms is the ONE wording of the accepted reference forms. Every refusal that has
// to tell an operator what it would have accepted quotes this, so the grammar is described
// in exactly one place and a verb cannot advertise a form the parser does not take.
func TypedRefForms() string {
	return "the accepted forms are `!N` / `owner/repo!N` (a merge request or pull request), " +
		"`N`, `#N` / `owner/repo#N` (kind unstated — the forge resolves it, and refuses where a " +
		"project carries both an issue and a merge request at that number), or the object's web " +
		"URL (`…/issues/N`, `…/pull/N`, `…/-/merge_requests/N`). Where the number alone is " +
		"ambiguous, state the kind — `!N` for a change, or the verb's `--kind issue` flag"
}

// ResolveStatedKind reconciles the kind a REFERENCE stated with the kind a FLAG stated and
// returns the one kind both agree on, or "" when neither stated anything.
//
// A disagreement is REFUSED rather than resolved in favour of either. The two spellings mean
// the same thing when they agree, and when they do not the caller has said two different
// things about one object — picking the flag would ignore the reference the operator typed,
// and picking the reference would ignore the flag they set. `what` names the argument in the
// refusal (`--by`, `the item number`) so the message says which reference disagreed.
func ResolveStatedKind(what string, ref ItemRef, flag TargetKind) (TargetKind, error) {
	switch {
	case !ref.KindStated():
		return flag, nil
	case flag == "":
		return ref.Kind, nil
	case flag == ref.Kind:
		return ref.Kind, nil
	default:
		return "", Refused(fmt.Sprintf(
			"refused: %s names %s, which states kind %q, but the kind flag states %q. The reference and the "+
				"flag disagree about which object is meant; this is not resolved in favour of either.",
			StripControl(what), StripControl(ref.String()), string(ref.Kind), string(flag)))
	}
}
