package deskkit

import "strings"

// AuthorIsRoleApp reports whether login renders as one of the roster's role Apps
// (ASSAY_TRUSTED_BOT_SLUGS), using the SAME `<slug>[bot]` / `app/<slug>` resolution the
// public-author trust gate (TrustedPublicAuthor) uses — never a hard-coded login. It is the
// App half of that gate WITHOUT the accountable-human fallback: the question here is "is the
// author one of our role Apps", not "is the author trusted to be auto-reviewed".
//
// Fail closed on the unknown: an empty login or an unconfigured roster is NOT asserted to be
// an App. That is the safe direction for THIS predicate — it never invents App-ness — and
// the caller supplies the other half (a caller that cannot resolve authorship still refuses
// by its own path/visibility terms). Membership in c.Bots (not id != 0) is the test, so an
// unpinned bot slug still resolves; a bare slug with no `[bot]`/`app/` rendering never does.
func AuthorIsRoleApp(login string) bool {
	c := EffectiveConfig()
	if !c.Configured() {
		return false
	}
	l := strings.ToLower(strings.TrimSpace(login))
	switch {
	case strings.HasSuffix(l, "[bot]"):
		_, ok := c.Bots[strings.TrimSuffix(l, "[bot]")]
		return ok
	case strings.HasPrefix(l, "app/"):
		_, ok := c.Bots[strings.TrimPrefix(l, "app/")]
		return ok
	}
	return false
}

// TrailerAbsentAppAnomaly reports the #587 anomaly: a role-App-authored PR carrying NO
// `Brief:` / `Issue:` link trailer. deskpr makes the trailer MANDATORY for App-authored PRs
// (exit 5), so a trailer-less App-authored PR is an anomaly by construction — an unverifiable
// change that must FAIL CLOSED. A caller (deskflip / deskboard) treats such a PR as
// RiskClassed + Unverifiable: the flip refuses until a `Security-Review: pass` stands at head,
// and the board row says why. A trailer-less HUMAN-authored PR is NOT this anomaly — it keeps
// today's path / label / visibility risk terms, so no new cost lands on maintainer PRs.
//
// It reuses AuthorIsRoleApp (roster resolution) and ParseTrailers (the one shared trailer
// grammar) — never a hard-coded login and never a second trailer parser. Presence, not
// validity, is what this gate turns on: a body whose trailers are malformed-but-present (a
// multiplicity error from ParseTrailers) is NOT trailer-absent, so it is deskpr's refusal to
// make, not a reason to fire the App fail-closed here. Only genuine ABSENCE — zero trailers,
// no parse error — is the anomaly.
func TrailerAbsentAppAnomaly(authorLogin string, prBody []byte) bool {
	if !AuthorIsRoleApp(authorLogin) {
		return false
	}
	trs, err := ParseTrailers(prBody)
	if err != nil {
		// A parse error is a multiplicity error: trailers are PRESENT (just too many), so
		// the row is not trailer-absent. deskpr owns that refusal; this gate does not fire.
		return false
	}
	return len(trs) == 0
}
