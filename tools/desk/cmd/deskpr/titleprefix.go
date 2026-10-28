package main

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// titleprefix.go — `deskpr create` names the brief a PR delivers in the PR title.
//
// The `Brief:` / `Cross-repo-brief:` trailer is already required and resolved before a PR
// opens, but the title never carried that id, so a reader of the PR list could not tell which
// brief a PR delivers without opening it. prefixTitleWithBrief closes that: once the body's
// link trailer has been parsed, a delivery trailer's id is put in front of the title unless
// the title already names it. `Authors:` and `Issue:`/`Fixes:` PRs keep their title.

// titleIDChar reports whether c can be part of a brief id token: a title character that
// would make an id match a longer, different id (`x/4` inside `x/46`, `my-x/4`, `a/x/4`).
func titleIDChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '-' || c == '_' || c == '/'
}

// titleNamesID reports whether title contains id as a whole token (case-insensitively). The
// character before a match must not extend the id leftwards (an id char or ':', so
// `other:x/4` is not `x/4`), and the character after it must not extend it rightwards.
func titleNamesID(title, id string) bool {
	t, want := strings.ToLower(title), strings.ToLower(id)
	for from := 0; from < len(t); {
		i := strings.Index(t[from:], want)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(want)
		beforeOK := i == 0 || !(titleIDChar(t[i-1]) || t[i-1] == ':')
		afterOK := end == len(t) || !titleIDChar(t[end])
		if beforeOK && afterOK {
			return true
		}
		from = i + 1
	}
	return false
}

// briefTitleID returns the id a PR body's link trailer asks the title to carry: the
// `<stream>/<NN>` of a `Brief:` trailer, or the full value of a `Cross-repo-brief:` trailer.
// It returns "" for every other body (no trailer, `Authors:`, `Issue:`/`Fixes:`, the scan
// carrier). requireTrailer has already validated the trailer by the time this runs; a body it
// cannot parse yields "" rather than a second refusal.
func briefTitleID(body []byte) string {
	trs, err := deskkit.ParseTrailers(body)
	if err != nil || len(trs) == 0 {
		return ""
	}
	switch trs[0].Kind {
	case deskkit.TrailerBrief:
		stream, nn, ok := splitBriefTrailer(trs[0].Value)
		if !ok {
			return ""
		}
		return stream + "/" + nn
	case deskkit.TrailerCrossRepoBrief:
		return strings.TrimSpace(trs[0].Value)
	}
	return ""
}

// prefixTitleWithBrief returns the title `deskpr create` uses: `<id>: <title>` for a
// delivery trailer whose id the title does not already name, else title unchanged.
func prefixTitleWithBrief(title string, body []byte) string {
	id := briefTitleID(body)
	if id == "" || titleNamesID(title, id) {
		return title
	}
	return id + ": " + title
}
