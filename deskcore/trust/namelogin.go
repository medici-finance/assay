package trust

import (
	"strconv"
	"strings"
)

// Name-to-login maps (ASSAY_HUMAN_LOGIN_MAP and the former-humans twin ASSAY_FORMER_HUMAN_LOGIN_MAP).
//
// Both config readers split every list-shaped roster value with SplitList, which treats a space as a
// separator the same way it treats a comma. A name:login entry whose NAME half carries a space — a
// display name, which is what a GitLab account shows — was therefore shredded: "Ada Lovelace:ada"
// became the colon-less word "Ada", which refused the WHOLE roster, plus the fragment
// "Lovelace:ada", a mapping the operator never wrote. SplitNameLoginList keeps such an entry in one
// piece, so the reader can refuse that ONE entry by name and apply the rest.
//
// A name with whitespace is refused, never accepted: the name half is the neutral form stamped into
// an `On-behalf-of: human:<name>` trailer, whose value is space-delimited (` mode:unattended` follows
// it), so a spaced name cannot be stamped faithfully. Refusing the entry leaves that human with no
// mapping: a public-target write refuses for want of a neutral name, the trust checks do not admit
// that login through this map, and the ruling-impersonation guard, which matches the CONFIGURED
// names, does not recognise that human's name. The NOTICE says all three. Nothing here ever ADDS a
// mapping that SplitList's split would not have produced.

// SplitNameLoginList splits a name:login map value into entries. Separators are SplitList's — comma,
// semicolon, CR, LF, space and tab, and no other character — and each piece is trimmed and an empty
// one dropped, as SplitList does, with one difference: a run of colon-less words directly followed by
// a word that carries ':' AFTER at least one other character is ONE entry, its words joined by a
// single space ("Ada Lovelace:ada"). A word that BEGINS with ':' is never a join target — the run
// before it and the word itself stay separate entries, as SplitList returns them — because joining
// "ada" onto ":ada-l" would make a name half ("ada ") whose only space is the one the join inserted,
// and a reader's trim would then accept an entry SplitList's split refused (an empty name). So every
// joined entry's name half carries a space BETWEEN two of its words. A value with no such run splits
// exactly as SplitList splits it. Colon-less words with no join target after them in the same
// comma/semicolon/CR/LF segment are returned one per entry, as SplitList returns them, so the
// caller's parse check refuses them as before.
func SplitNameLoginList(raw string) []string {
	segments := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	out := []string{}
	for _, seg := range segments {
		var pending []string
		for _, w := range strings.FieldsFunc(seg, func(r rune) bool { return r == ' ' || r == '\t' }) {
			if w = strings.TrimSpace(w); w == "" {
				continue
			}
			colon := strings.Index(w, ":")
			if colon < 0 {
				pending = append(pending, w)
				continue
			}
			if colon > 0 && len(pending) > 0 {
				w = strings.Join(append(pending, w), " ")
			} else {
				out = append(out, pending...)
			}
			pending = nil
			out = append(out, w)
		}
		out = append(out, pending...)
	}
	return out
}

// NameHasWhitespace reports whether the name half of a name:login entry carries any whitespace rune
// (unicode.IsSpace: a space or tab, the shape SplitNameLoginList keeps whole, and also a no-break
// space, line separator or vertical tab, which neither splitter breaks on). Both readers refuse such
// an entry on its own. Readers pass the UNTRIMMED half (as strings.Cut returns it), so a space next
// to the colon is seen, not trimmed away. strings.Fields splits on exactly the unicode.IsSpace set,
// so rejoining its fields reproduces the name only when the name has no whitespace rune; this keeps
// the package off the unicode import its pure-package allow list does not carry.
func NameHasWhitespace(name string) bool {
	return strings.Join(strings.Fields(name), "") != name
}

// SpacedNameNotice is the NOTICE both readers record for an entry refused because its name half
// carries whitespace. One wording, so the two binaries cannot drift.
func SpacedNameNotice(key, entry string) string {
	return key + ": entry " + strconv.Quote(entry) + " is NOT applied — its name half contains a space. " +
		"The name is the neutral form stamped into an `On-behalf-of: human:<name>` trailer, which " +
		"cannot carry a space, so this one entry gives that human NO mapping: a write that needs " +
		"the neutral name refuses, and the ruling-impersonation guard does not recognise that name. " +
		"It does not refuse the rest of the roster. Write the name without spaces, e.g. " +
		"`ada-lovelace:<login>`"
}
