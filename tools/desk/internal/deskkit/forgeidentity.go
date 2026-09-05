package deskkit

// forgeidentity.go — the FORGE dimension of a trusted bot identity (the forge-qualified-identity brief).
//
// Until this brief a roster bot entry could only ever spell a GitHub App: a slug, a
// numeric id, and the two fixed renderings `<slug>[bot]` / `app/<slug>`. On a GitLab
// deployment every check that asked "who acted?" answered could-not-check, because no
// entry could say WHICH FORGE the identity belonged to (see the stream README and the
// 2026-09-02 pilot's D-3/D-9). This file adds that dimension.
//
// The grammar an entry now speaks is `[role=]<forge>:<slug-or-login>[:<numeric id>]`.
// An entry with NO `<forge>` segment is read as `github` — the backward-compatibility
// rule — and the default is RECORDED (BotIdentity.ForgeInferred) so a caller can tell
// an explicit `github` from an inferred one. The per-forge rendering set, the per-forge
// commit-address shape, and the corroboration rule are all documented once in
// the stream identity.md doc so briefs 07/08/09/11 cite rather than restate.
//
// A NOTE ON THE GITLAB COMMIT ADDRESS, because it is the one shape here that is
// validate-only. A GitLab service account commits under
// `service_account_group_<group-id>_<per-account-suffix>@noreply.<host>` (pilot §0/§3
// row 13). Neither the group id nor the per-account suffix is derivable from a roster
// entry — the entry carries only the account's username and its numeric USER id, and the
// group id (`9619193` on the pilot) is a different number from every user id
// (`41987965…`). So the GitLab address is matched by SHAPE, never CONSTRUCTED from the
// roster, and a tool that must STAMP a commit identity (deskwt) refuses a GitLab entry
// rather than inventing an underivable value or falling back to the GitHub shape.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// BotIdentity is one parsed, forge-qualified ASSAY_TRUSTED_BOT_SLUGS entry.
type BotIdentity struct {
	// Forge is the forge this identity lives on. It is `github` or `gitlab`.
	Forge ForgeKind
	// ForgeInferred is true when the entry carried NO `<forge>` segment and so was
	// DEFAULTED to github (the backward-compatibility rule). It is false when the
	// entry spelled a forge explicitly. The distinction is load-bearing: the
	// forge-agreement check (assertEntryForgeAgrees) refuses an EXPLICIT mismatch but
	// exempts an inferred-github entry, because a legacy roster carrying no forge must
	// keep working — and whether an inferred entry may act on a non-github repo is the
	// question the human gate on this brief is confirming, not one this code pre-empts.
	ForgeInferred bool
	// Slug is the slug-or-login field, lowercased: a GitHub App slug, or a GitLab
	// service-account username.
	Slug string
	// ID is the numeric id (a GitHub bot USER id, a GitLab user id); 0 when unpinned.
	ID int64
}

// gitlabServiceAccountRe matches a GitLab service-account commit noreply address:
// `service_account_group_<group-id>_<per-account-suffix>@noreply.<host>`. The group id
// and suffix are NOT in a roster entry (see the file header), so this is a SHAPE the
// preflight matches, never a value it constructs. The host is left general so a
// self-hosted instance's `@noreply.<instance-host>` matches as well as gitlab.com's.
var gitlabServiceAccountRe = regexp.MustCompile(`^service_account_group_[0-9]+_[0-9a-z]+@noreply\.[a-z0-9.-]+$`)

// splitBotEntry parses one ASSAY_TRUSTED_BOT_SLUGS entry (the optional `role=` prefix
// already stripped by the caller). ok is false when the slug-or-login is empty or an id
// is present but not a positive number — the same fail-closed rule splitIdentity applies,
// so a typo'd id never degrades to login-only trust.
//
// The forge is detected by the FIRST colon-separated segment: a leading `github:` or
// `gitlab:` (with something after it) qualifies the entry; anything else is read as a
// legacy `slug[:id]` on github. A GitHub App slug or a GitLab username is never literally
// `github`/`gitlab`, so the discriminator does not collide with a real identity.
func splitBotEntry(entry string) (BotIdentity, bool) {
	b := BotIdentity{Forge: ForgeGitHub, ForgeInferred: true}
	head, rest, found := strings.Cut(entry, ":")
	switch strings.ToLower(strings.TrimSpace(head)) {
	case string(ForgeGitHub), string(ForgeGitLab):
		if found {
			b.Forge = ForgeKind(strings.ToLower(strings.TrimSpace(head)))
			b.ForgeInferred = false
			entry = rest
		}
	}
	slug, id, ok := splitIdentity(entry)
	if !ok {
		return BotIdentity{}, false
	}
	b.Slug = slug
	b.ID = id
	return b, true
}

// AcceptedLogins is the per-forge set of login renderings this identity is recognised
// under, lowercased. On GitHub the account renders in two decorated forms and the bare
// slug is deliberately absent (App slugs and usernames share no namespace, so a plain
// user named after a slug could otherwise spoof a desk identity). On GitLab the
// service-account username IS the identity the API attributes notes/approvals/commits to
// — there is no decorated form — so the username is the one genuine rendering; the
// GitHub `[bot]`/`app/` decorations are NOT minted for it, so a GitLab username dressed
// in GitHub bot clothing is not accepted.
func (b BotIdentity) AcceptedLogins() []string {
	switch b.Forge {
	case ForgeGitLab:
		return []string{b.Slug}
	default:
		return []string{b.Slug + "[bot]", "app/" + b.Slug}
	}
}

// CommitEmailSpec is the expected commit author address for a forge-qualified identity.
// GitHub yields an EXACT address (derivable from slug+id). GitLab yields a SHAPE (a
// regexp): the service-account noreply address embeds a group id and a per-account
// suffix that a roster entry does not carry, so it can be VALIDATED but not CONSTRUCTED.
type CommitEmailSpec struct {
	Forge ForgeKind
	// Exact is the one acceptable address for a GitHub identity, empty when the id is
	// unpinned (nothing to build the address from) or the forge is GitLab.
	Exact string
	// Shape is the acceptable-address pattern for a GitLab identity, nil for GitHub.
	Shape *regexp.Regexp
	// Derivable reports whether an exact address could be built — true only for a
	// GitHub identity with a pinned bot USER id. A tool that must STAMP an identity
	// refuses when this is false rather than inventing an underivable value.
	Derivable bool
}

// CommitEmailSpec returns the expected commit-address spec for this identity.
func (b BotIdentity) CommitEmailSpec() CommitEmailSpec {
	switch b.Forge {
	case ForgeGitLab:
		return CommitEmailSpec{Forge: ForgeGitLab, Shape: gitlabServiceAccountRe, Derivable: false}
	default:
		if b.ID > 0 {
			return CommitEmailSpec{
				Forge:     ForgeGitHub,
				Exact:     fmt.Sprintf("%d+%s[bot]@users.noreply.github.com", b.ID, b.Slug),
				Derivable: true,
			}
		}
		return CommitEmailSpec{Forge: ForgeGitHub, Derivable: false}
	}
}

// Accepts reports whether email is an acceptable commit author address under this spec.
// GitHub compares the exact address; GitLab matches the service-account shape. Neither
// accepts the other forge's shape — a GitHub noreply address never matches the GitLab
// pattern, and a GitLab address is not the exact GitHub string — which is the property
// that keeps a cross-forge address from passing a check it should fail.
func (s CommitEmailSpec) Accepts(email string) bool {
	email = strings.TrimSpace(email)
	switch s.Forge {
	case ForgeGitLab:
		return s.Shape != nil && s.Shape.MatchString(strings.ToLower(email))
	default:
		return s.Exact != "" && strings.EqualFold(email, s.Exact)
	}
}

// RoleBotIdentity resolves a desk role to its forge-qualified bot identity. ok is false
// for an unbound role or one whose slug has no parsed identity — the same fail-closed
// shape RoleAppLogin's ok return enforces, so a caller cannot turn an unbound role into
// an empty-identity comparison.
func (c Config) RoleBotIdentity(role string) (BotIdentity, bool) {
	slug, bound := c.RoleBots[strings.ToLower(strings.TrimSpace(role))]
	if !bound || strings.TrimSpace(slug) == "" {
		return BotIdentity{}, false
	}
	b, ok := c.BotIdents[strings.ToLower(slug)]
	return b, ok
}

// sortedBotIdents renders the configured bot identities in forge-qualified form
// (`<forge>:<slug>[:<id>]`), sorted, for the P3 effective-config echo. It is forge-first
// so a GitLab identity is as visible in run output as a GitHub one — a bot line that
// showed only the GitHub-slug set would make a GitLab identity invisible in the echo.
func (c Config) sortedBotIdents() string {
	out := make([]string, 0, len(c.BotIdents))
	for _, b := range c.BotIdents {
		s := string(b.Forge) + ":" + b.Slug
		if b.ID != 0 {
			s += ":" + strconv.FormatInt(b.ID, 10)
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// assertEntryForgeAgrees refuses when a role's EXPLICITLY forge-qualified bot entry names
// a forge other than resolved. An INFERRED-github entry (a legacy unqualified roster) is
// exempt — that is the backward-compatibility rule the human gate is confirming, and this
// code does not pre-empt it. An unbound role is not this check's concern (the caller's own
// unbound handling covers it). Silently accepting a mismatched entry is exactly how a
// same-named account on the wrong forge would become trusted, so the mismatch is a Refused
// (exit 5) naming BOTH forges, never a silent drop.
func assertEntryForgeAgrees(role string, resolved ForgeKind) error {
	ident, ok := EffectiveConfig().RoleBotIdentity(role)
	if !ok || ident.ForgeInferred {
		return nil
	}
	if ident.Forge != resolved {
		return Refused(fmt.Sprintf(
			"roster entry for role %q names forge %q, but the repo being acted on resolves to forge %q — "+
				"a forge-mismatched identity is REFUSED, never ignored: silently accepting it is how a "+
				"same-named account on the wrong forge becomes trusted. Fix the %s entry for %q, or the "+
				"repo's %s binding.",
			strings.ToLower(role), ident.Forge, resolved, EnvTrustedBotSlugs, strings.ToLower(role), EnvRepoForges))
	}
	return nil
}

// AssertRoleForgeMatches resolves the forge that serves repo and refuses (Refused, exit 5)
// when role's forge-qualified bot entry names a different one. It is the standalone,
// repo-taking form of the agreement gate ForgeFor applies inline; a forge that cannot be
// resolved is ForgeFor's own could-not-check to raise, not this check's, so this returns
// nil there rather than inventing a second refusal for the same condition.
func AssertRoleForgeMatches(role string, repo ForgeRepo) error {
	ident, ok := EffectiveConfig().RoleBotIdentity(role)
	if !ok || ident.ForgeInferred {
		return nil
	}
	res, err := resolveForgeKind(repo)
	if err != nil {
		return nil
	}
	return assertEntryForgeAgrees(role, res.Kind)
}
