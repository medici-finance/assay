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
// mapping, which is the fail-closed state every consumer already handles (a public-target write
// refuses for want of a neutral name).

// SplitNameLoginList splits a name:login map value into entries. Separators are SplitList's — comma,
// semicolon, newline, space and tab — with one difference: a run of colon-less words directly
// followed by a word that carries ':' is ONE entry, its words joined by a single space
// ("Ada Lovelace:ada"). A value whose every entry is a well-formed name:login with no space in the
// name splits exactly as SplitList splits it. Colon-less words with no ':' word after them in the
// same comma/semicolon/newline segment are returned one per entry, as SplitList returns them, so the
// caller's parse check refuses them as before.
func SplitNameLoginList(raw string) []string {
	segments := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	out := []string{}
	for _, seg := range segments {
		var pending []string
		for _, w := range strings.Fields(seg) {
			if !strings.Contains(w, ":") {
				pending = append(pending, w)
				continue
			}
			if len(pending) > 0 {
				w = strings.Join(append(pending, w), " ")
				pending = nil
			}
			out = append(out, w)
		}
		out = append(out, pending...)
	}
	return out
}

// NameHasWhitespace reports whether the name half of a name:login entry carries whitespace, the
// shape SplitNameLoginList keeps whole and both readers refuse entry-by-entry.
func NameHasWhitespace(name string) bool {
	return strings.ContainsAny(name, " \t")
}

// SpacedNameNotice is the NOTICE both readers record for an entry refused because its name half
// carries whitespace. One wording, so the two binaries cannot drift.
func SpacedNameNotice(key, entry string) string {
	return key + ": entry " + strconv.Quote(entry) + " is NOT applied — its name half contains a space. " +
		"The name is the neutral form stamped into an `On-behalf-of: human:<name>` trailer, which " +
		"cannot carry a space, so this one entry gives that human NO mapping (a write that needs " +
		"the neutral name refuses). It does not refuse the rest of the roster. Write the name without " +
		"spaces, e.g. `ada-lovelace:<login>`"
}
