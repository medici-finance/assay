package main

// forgeidentity.go — the FORGE dimension of a trusted bot identity, statusgen's half
// (forge-neutral/07).
//
// statusgen and the desk tools (deskkit) are separate Go modules that deliberately
// share no code (the documented-duplicate pattern rosterconfig.go's header states),
// so this is a cross-tree mirror of deskkit.BotIdentity / splitBotEntry. The grammar,
// the per-forge rendering set and the per-forge commit-address shape are documented
// ONCE, in docs/streams/forge-neutral/identity.md; briefs 07/08 cite it rather than
// restating it. A change to either copy must be made in both. The roster-grammar
// coupling is bound by TestRosterGrammarParity over the shared vector file.
//
// The grammar an entry speaks is `[role=]<forge>:<slug-or-login>[:<numeric id>]`. An
// entry with NO `<forge>` segment is read as `github` (the backward-compatibility rule)
// and the default is RECORDED (ForgeInferred), so a caller can tell a legacy entry from
// a deliberate `github:` one.
//
// A NOTE ON THE GITLAB COMMIT ADDRESS, because it is the shape here that is
// validate-only. A GitLab service account commits under
// `service_account_group_<group-id>_<per-account-suffix>@noreply.<host>` (forge-gitlab
// pilot §0/§3 row 13). Neither the group id nor the per-account suffix is derivable
// from a roster entry — the entry carries only the account's username and its numeric
// USER id, and the group id is a different number from every user id. So the GitLab
// address is matched by SHAPE, never constructed, and the acting GitLab account is
// identified by its USERNAME (the git author name, which the pilot's `%an` read
// confirms is the service-account username) PLUS the address shape. That is a
// LOGIN-ONLY match — weaker than GitHub's id-pinned one, because the numeric id that
// would pin it is absent from the commit metadata — and it is recorded as weaker
// wherever it is used.

import (
	"regexp"
	"strconv"
	"strings"
)

// scanBotIdentity is one parsed, forge-qualified ASSAY_TRUSTED_BOT_SLUGS entry —
// statusgen's mirror of deskkit.BotIdentity.
type scanBotIdentity struct {
	// Forge is the forge this identity lives on: forgeGitHub, forgeGitLab, or
	// forgeUnknown for an entry that spelled a forge this build does not recognise
	// (never rounded to github — see ForgeRaw and evidenceActorPolicyFromRoster's
	// third could-not-check reason).
	Forge forgeKind
	// ForgeRaw is the forge token AS WRITTEN, lowercased, so an unrecognised forge can
	// be NAMED in a could-not-check message rather than silently dropped.
	ForgeRaw string
	// ForgeInferred is true when the entry carried NO `<forge>` segment and so was
	// DEFAULTED to github (the backward-compatibility rule); false when the entry
	// spelled a forge explicitly.
	ForgeInferred bool
	// Slug is the slug-or-login field, lowercased.
	Slug string
	// ID is the numeric id (a GitHub bot USER id, a GitLab user id); 0 when unpinned.
	ID int64
}

// scanGitlabServiceAccountRe matches a GitLab service-account commit noreply address,
// `service_account_group_<group-id>_<per-account-suffix>@noreply.<host>`. The group id
// and suffix are NOT in a roster entry (see the file header), so this is a SHAPE that
// is matched, never a value that is constructed. The host is left general so a
// self-hosted instance's `@noreply.<instance-host>` matches as well as gitlab.com's.
// KEEP IN SYNC with deskkit.gitlabServiceAccountRe.
var scanGitlabServiceAccountRe = regexp.MustCompile(`^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$`)

// scanLooksLikeExplicitForge reports whether an entry's leading colon-segment is an
// EXPLICIT (though possibly unrecognised) forge qualifier rather than a legacy slug.
//
// The discriminator is structural: a legacy entry is at most `slug:id` (two segments),
// and a slug never contains a colon, so any entry with a THIRD colon-segment is
// forge-qualified. `head` is that leading segment (already lowercased); `rest` is
// everything after the first colon. It qualifies iff head is a non-numeric token (a
// forge name, never a numeric id) AND rest still carries a `:` (the `<forge>:<slug>:<id>`
// shape). This is what lets `bitbucket:some-slug:12345` be recognised as naming a forge
// this build does not understand — and turned into could-not-check-naming-the-forge —
// instead of failing the whole roster as an unparseable id.
func scanLooksLikeExplicitForge(head, rest string) bool {
	if head == "" {
		return false
	}
	if _, err := strconv.ParseInt(head, 10, 64); err == nil {
		return false // a numeric head is the id half of a legacy slug:id, not a forge
	}
	return strings.Contains(rest, ":")
}

// scanSplitBotEntry parses one ASSAY_TRUSTED_BOT_SLUGS entry (the optional `role=`
// prefix already stripped by the caller). ok is false when the slug-or-login is empty
// or an id is present but not a positive number — the same fail-closed rule
// scanSplitIdentity applies, so a typo'd id never degrades to login-only trust.
//
// The forge is detected by the FIRST colon-separated segment: a leading `github:` or
// `gitlab:` qualifies the entry to that forge; a leading `<other>:<slug>:<id>` qualifies
// it to an UNRECOGNISED forge (forgeUnknown, its raw name kept); anything else is read
// as a legacy `slug[:id]` on github. A GitHub App slug or a GitLab username is never
// literally `github`/`gitlab`, so the discriminator does not collide with a real
// identity. KEEP IN SYNC with deskkit.splitBotEntry (which REFUSES an unrecognised
// forge; statusgen instead records it so the Evidence-actor lint can report
// could-not-check naming the forge, per forge-neutral/07 task 3 — this is the one
// deliberate divergence, and it never rounds an unknown forge up to a pass).
func scanSplitBotEntry(entry string) (scanBotIdentity, bool) {
	b := scanBotIdentity{Forge: forgeGitHub, ForgeRaw: forgeGitHub.String(), ForgeInferred: true}
	head, rest, found := strings.Cut(entry, ":")
	h := strings.ToLower(strings.TrimSpace(head))
	if found {
		switch h {
		case forgeGitHub.String():
			b.Forge, b.ForgeRaw, b.ForgeInferred, entry = forgeGitHub, h, false, rest
		case forgeGitLab.String():
			b.Forge, b.ForgeRaw, b.ForgeInferred, entry = forgeGitLab, h, false, rest
		default:
			if scanLooksLikeExplicitForge(h, rest) {
				b.Forge, b.ForgeRaw, b.ForgeInferred, entry = forgeUnknown, h, false, rest
			}
		}
	}
	slug, id, ok := scanSplitIdentity(entry)
	if !ok {
		return scanBotIdentity{}, false
	}
	b.Slug = slug
	b.ID = id
	return b, true
}

// acceptedLogins is the per-forge set of login renderings this identity is recognised
// under, lowercased. GitHub accepts the two decorated forms and never the bare slug
// (App slugs and usernames share no namespace, so a plain user named after a slug could
// otherwise spoof a desk identity). GitLab accepts the service-account USERNAME
// verbatim — the identity the API attributes to — and mints no `[bot]`/`app/`
// decoration for it. An unrecognised forge has no trustable rendering, so it accepts
// none. KEEP IN SYNC with deskkit.BotIdentity.AcceptedLogins.
func (b scanBotIdentity) acceptedLogins() []string {
	switch b.Forge {
	case forgeGitLab:
		return []string{b.Slug}
	case forgeGitHub:
		return []string{b.Slug + "[bot]", "app/" + b.Slug}
	default:
		return nil
	}
}
