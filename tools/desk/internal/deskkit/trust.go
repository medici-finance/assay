package deskkit

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Trust gate: desk scanning loops read repos where ARBITRARY
// external users can author issues, PRs, and comments. Unvetted third-party text
// must never enter a desk's work queue or steer its actions. The rule: a desk
// ignores EVERYTHING unless it is authored by a configured trusted identity — OR
// the configured BLESSING AUTHORITY has commented on it (the blessing). Untrusted
// items stay QUARANTINED-VISIBLE (listed, counted, never actionable) so the
// blessing authority can see what awaits; one of their comments admits an item.
//
// WHERE THE ROSTER COMES FROM. It used to be compiled in.
// It is now adopter CONFIGURATION — but configuration read from OUTSIDE every ref
// the tools evaluate (a repository/organization Actions variable in CI, a
// config-home file locally), never from a file a pull request can author, never
// from a flag, and — for the tools that ACT — never from the environment. See
// rosterconfig.go for the mechanism and the properties it preserves.
//
// Matching stays fail-closed in every direction: an unset roster trusts nobody
// and blesses nobody, and empty or unknown logins are untrusted.

// TrustedAuthor reports whether login is one of the CONFIGURED trusted
// identities. Empty and unknown logins are untrusted, and so is EVERY login when
// the roster is unconfigured (P1: unset is closed, not open). Comparison is exact
// on the rendered form (no trimming, no bare-slug acceptance), case-insensitive
// only — GitHub logins and App slugs are case-insensitively unique, so a case
// variant is the same account, never a second one.
func TrustedAuthor(login string) bool {
	if login == "" {
		return false
	}
	c := EffectiveConfig()
	if !c.Configured() {
		return false
	}
	return c.Logins[strings.ToLower(login)]
}

// TrustedPublicAuthor reports whether login may have a PR AUTO-REVIEWED on a
// PUBLIC (risk-classed) repo. The bar is deliberately HIGHER than TrustedAuthor:
// only the role Apps (ASSAY_TRUSTED_BOT_SLUGS, matched on their `<slug>[bot]` /
// `app/<slug>` rendering) and the real, accountable humans in
// ASSAY_HUMAN_LOGIN_MAP qualify. A shared machine account admitted to
// ASSAY_TRUSTED_LOGINS as a human — an org push/token account, not an
// accountable role — confers NO public-review trust, and a bare slug never does
// (username-squatting fail-close, parity with expectedID). Public repos accept
// fork PRs from ANY account, so auto-engaging an untrusted diff would spend the
// reviewer App's identity on hostile input and blur the fork-PR trust boundary
// (#943). Empty/unknown logins and an unconfigured roster are untrusted (fail
// closed). Private-repo review is unaffected — that path keeps using
// TrustedAuthor, which additionally honours ASSAY_TRUSTED_LOGINS.
func TrustedPublicAuthor(login string) bool {
	if login == "" {
		return false
	}
	c := EffectiveConfig()
	if !c.Configured() {
		return false
	}
	l := strings.ToLower(strings.TrimSpace(login))
	if l == "" {
		return false
	}
	// A role App, matched on its rendered form against the configured bot slugs.
	// Map MEMBERSHIP (not id != 0) so an unpinned bot slug still qualifies; the
	// bare slug is never accepted.
	switch {
	case strings.HasSuffix(l, "[bot]"):
		if _, ok := c.Bots[strings.TrimSuffix(l, "[bot]")]; ok {
			return true
		}
	case strings.HasPrefix(l, "app/"):
		if _, ok := c.Bots[strings.TrimPrefix(l, "app/")]; ok {
			return true
		}
	}
	// A mapped, accountable human (ASSAY_HUMAN_LOGIN_MAP `name:login`). The map is
	// name→login; the login is the value. Case-insensitive, like every login match.
	for _, mapped := range c.HumanLogins {
		if strings.EqualFold(mapped, l) {
			return true
		}
	}
	return false
}

// TrustedHumanAuthor reports whether login is an ACCOUNTABLE trusted human — the
// person who owns and merges their OWN PR, as distinct from a role App or a shared
// machine account. It answers one question for the board's neglect axis (#177): a
// PR with no reviewer-App verdict at head is desk-review neglect ONLY when the desk
// is responsible for reviewing it; a maintainer's own un-reviewed PR (the closure
// artifact of a human-gated brief, where the human fills the decision table in their
// own PR and merges it) is theirs to merge, and the review desk deliberately does
// not dispatch a model reviewer on it — a model reviewing the human's ratified
// ruling inverts the gate.
//
// The accountable-human set is the SAME one the public-repo trust bar honours as a
// human (TrustedPublicAuthor, #943): the mapped humans of ASSAY_HUMAN_LOGIN_MAP,
// plus the single blessing authority (an accountable human by construction — the
// loader refuses a bot/App bless login). It deliberately EXCLUDES:
//   - role Apps / bot renderings ([bot], app/, -app, -bot) — never a human, and
//     #177's non-goal keeps App-authored PRs in the review-neglect metric;
//   - a shared machine account admitted to ASSAY_TRUSTED_LOGINS as a plain human
//     (an org push/token account, not an accountable person) — exactly the account
//     TrustedPublicAuthor refuses public-review trust to. Its PRs are NOT "a trusted
//     human's own PRs" and stay in the neglect metric. This is why the check is NOT
//     `TrustedAuthor && !bot`: that would sweep the shared account in too.
//
// Empty/unknown logins and an unconfigured roster are false (fail closed).
func TrustedHumanAuthor(login string) bool {
	if login == "" {
		return false
	}
	l := strings.ToLower(strings.TrimSpace(login))
	if l == "" {
		return false
	}
	// A bot/App/shared-agent rendering is never an accountable human, however
	// trusted its App identity — fail closed against one reaching here.
	if looksLikeBot(l) {
		return false
	}
	c := EffectiveConfig()
	if !c.Configured() {
		return false
	}
	// The blessing authority is an accountable human by construction.
	if c.Bless.Login != "" && strings.EqualFold(l, c.Bless.Login) {
		return true
	}
	// A mapped, accountable human (ASSAY_HUMAN_LOGIN_MAP). Values are lowercased at
	// parse. NOT the broad ASSAY_TRUSTED_LOGINS set (c.Humans) — see the doc above.
	for _, mapped := range c.HumanLogins {
		if strings.EqualFold(mapped, l) {
			return true
		}
	}
	return false
}

// expectedID resolves a trusted login (any accepted rendered form) to its pinned
// numeric id. ok is false for logins outside the trusted set.
func expectedID(login string) (id int64, ok bool) {
	c := EffectiveConfig()
	if !c.Configured() {
		return 0, false
	}
	l := strings.ToLower(login)
	if id, ok := c.Humans[l]; ok {
		return id, true
	}
	slug := l
	switch {
	case strings.HasSuffix(l, "[bot]"):
		slug = strings.TrimSuffix(l, "[bot]")
	case strings.HasPrefix(l, "app/"):
		slug = strings.TrimPrefix(l, "app/")
	default:
		return 0, false // bare slug is never trusted (username-squatting fail-close)
	}
	id, ok = c.Bots[slug]
	return id, ok
}

// TrustedAuthorID is the id-aware form of TrustedAuthor: when the surface has the
// numeric user id (REST user.id / GraphQL databaseId), BOTH the login and the id
// must match the configured identity — defending against login recycling. id 0
// means the surface has no numeric id (gh CLI list JSON) and falls back to
// login-only. A trusted login carrying the WRONG id is untrusted (fail closed).
// An UNPINNED configured identity (the id is optional on ASSAY_TRUSTED_LOGINS /
// ASSAY_TRUSTED_BOT_SLUGS) is trusted only on surfaces that structurally carry no
// id. Where the surface DOES supply one there is nothing to compare it against, so
// this answers false — pinning is what buys trust on an id-bearing surface.
//
// This arm was briefly the other way round. The conversion introduced
// `if want == 0 { return true }`, which made a configured-but-unpinned login
// trusted carrying ANY id — precisely the login-recycling exposure P4 exists to
// close, and a WIDENING against the pre-conversion code: old TrustedAuthorID was
// `return ok && want == id`, under which want==0 could never match a real id. Every
// identity in the roster is pinned, so nothing here changed verdicts — but
// "behaviour-preserving" was the claim being made, the arm was unasserted, and
// mutation A (removing it) survived the whole suite. Restored to the old semantics
// and pinned by the shared coupling vector's unpinned-human cases.
func TrustedAuthorID(login string, id int64) bool {
	if !TrustedAuthor(login) {
		return false
	}
	if id == 0 {
		// The SURFACE has no id (gh CLI list JSON). Login-only, as before.
		return true
	}
	want, ok := expectedID(login)
	if !ok {
		return false
	}
	return want != 0 && want == id
}

// trustedContentAuthor is the STRICT id-aware trust check for surfaces where the
// numeric id is ALWAYS available (GraphQL databaseId) — used inside Blessed's
// untrusted-content loop, the same surface statusgen's evalIssueBlessing guards
// with its own strict trustedAuthorID (statusgen/trustgate.go). Bot accounts stay
// trusted by login alone (Apps cannot be recycled); a HUMAN login with id==0 is
// untrusted here, unlike the general-purpose TrustedAuthorID above, which
// deliberately falls back to login-only for surfaces (gh CLI list JSON) that
// structurally lack a numeric id. Ported to close the divergence the
// #315 review found: before this, a trusted-human GraphQL event
// carrying no id was treated as trusted content and did not re-quarantine, while
// statusgen's analogous check (post-T10) already failed closed on the same input.
// KEEP IN SYNC with statusgen/trustgate.go's trustedAuthorID.
func trustedContentAuthor(login string, id int64) bool {
	if !TrustedAuthor(login) {
		return false
	}
	if id == 0 {
		// Bots stay trusted by login alone even without an id (Apps cannot be
		// recycled) — same as the general-purpose TrustedAuthorID. Human accounts
		// fail closed without a numeric id: this is the ONLY delta from
		// TrustedAuthorID, closing the divergence the #315 review
		// found (statusgen's trustedAuthorID already required id!=0 for human
		// logins; the two copies disagreed on this exact input).
		lower := strings.ToLower(login)
		return strings.HasSuffix(lower, "[bot]") || strings.HasPrefix(lower, "app/")
	}
	// id != 0: identical to TrustedAuthorID — an exact match required for BOTH
	// human and bot logins (a trusted login carrying the wrong id is untrusted).
	want, ok := expectedID(login)
	if !ok {
		return false
	}
	// want == 0 (configured, never pinned) is NOT a match on an id-bearing surface:
	// there is nothing to check the id against, and this is the STRICT path.
	return want != 0 && want == id
}

// IsBlessAuthority reports whether login is the CONFIGURED blessing authority —
// the single human account whose comment admits an externally-authored item.
// Only comment/review AUTHORSHIP by this login blesses an item; the name
// appearing in someone else's comment TEXT is never consulted here. Desk App
// identities can never be the blessing authority (the loader refuses to accept
// one), because that would let the desks admit external items to their own
// queues. With the roster unconfigured this is false for every input.
func IsBlessAuthority(login string) bool {
	if login == "" {
		return false
	}
	c := EffectiveConfig()
	if !c.Configured() {
		return false
	}
	return strings.EqualFold(login, c.Bless.Login)
}

// IsBlessAuthorityID is the id-aware blessing-authority check: with a numeric id
// available, the blessing login carrying any other id is NOT the authority (fail
// closed). The configured bless login always carries a pinned id — the loader
// refuses a bare one — so the id==0 arm here means only "this surface did not
// supply an id", never "no id was configured".
func IsBlessAuthorityID(login string, id int64) bool {
	if !IsBlessAuthority(login) {
		return false
	}
	return id == 0 || id == EffectiveConfig().Bless.ID
}

// IsBlessAuthorityIDStrict is IsBlessAuthorityID with the id==0 compatibility arm
// removed: the id must be present AND correct. Use it wherever the login and the id
// are parsed from the SAME GitHub object (a reactions payload, a review payload), so
// an absent id means the read was wrong rather than that the surface has no ids.
// Fails closed on id==0 — unlike IsBlessAuthorityID, a missing id here is NEVER
// treated as "this surface has no ids" and admitted on login alone (for example,
// a `ada`-login reaction carrying no numeric id must be refused, not
// silently trusted via the general-purpose id==0 fallback).
func IsBlessAuthorityIDStrict(login string, id int64) bool {
	return IsBlessAuthority(login) && id != 0 && id == EffectiveConfig().Bless.ID
}

// ItemTrusted reports whether a GitHub item (issue or PR) may enter a desk work
// queue: its author is trusted, OR the blessing authority authored at least one
// comment on it (issue comments, PR review comments, and PR reviews all count —
// callers pass the union of those author logins). The blessing applies regardless
// of who authored the item. No comment authors and an untrusted author →
// untrusted (fail closed).
//
// This is the timestamp-FREE form. A gate surface that can read edit timestamps
// MUST use ItemTrustedEvents instead — this form cannot see the bless-then-edit
// attack (author edits the body AFTER the blessing comment).
func ItemTrusted(authorLogin string, commentAuthors []string) bool {
	if TrustedAuthor(authorLogin) {
		return true
	}
	for _, c := range commentAuthors {
		if IsBlessAuthority(c) {
			return true
		}
	}
	return false
}

// ContentEvent is one content-bearing event on a GitHub item: an issue/PR comment,
// a PR review, or a PR review comment. CreatedAt is the creation/submission time;
// EditedAt is the content's lastEditedAt (zero when never edited). Author uses the
// same rendered forms TrustedAuthor accepts ("<slug>[bot]" / "app/<slug>" for Apps);
// AuthorID is the numeric user id when the surface has it (0 = unavailable).
type ContentEvent struct {
	Author    string
	AuthorID  int64
	CreatedAt time.Time
	EditedAt  time.Time
}

// Blessed evaluates a blessing for an item whose AUTHOR is untrusted. A blessing
// covers the content AS OF the blessing comment — later edits need a fresh
// comment from the blessing authority (bless-then-edit hole). The
// item is blessed iff:
//
//	(1) the blessing authority authored at least one event; let B = the LATEST
//	    such event's CreatedAt (their own later edits do not move B — their edits
//	    are trusted);
//	(2) the item BODY was not edited after B (bodyEditedAt is the item's
//	    lastEditedAt — zero when never edited, which is always covered: the body
//	    existed when the blessing was given);
//	(3) no UNTRUSTED-authored event was created OR edited after B — an external
//	    user adding or rewriting a comment after the blessing re-quarantines the
//	    item until the authority comments again. Trusted-author events after B are
//	    fine: their content is trusted in its own right.
//
// With the roster UNCONFIGURED there is no blessing authority, so this is false
// for every input (P1).
//
// Note the timestamp source: use GraphQL lastEditedAt (tracks CONTENT edits) —
// REST updated_at moves on unrelated events like labels and would re-quarantine
// spuriously. Title renames are not tracked by lastEditedAt; titles are display
// strings on the boards, never instructions, so the body/comment rule is the gate
// (and the quarantine renderers print all public-origin text inertly quoted).
//
// TODO(hash-pinning): the stronger v2 records a HASH of the blessed body at blessing
// time and re-compares at act time (any content delta voids the blessing even if
// timestamps are somehow off). That needs durable per-item state the stateless board
// reads don't have today — tracked as a follow-up issue; this timestamp comparison
// is the shipped v1.
func Blessed(bodyEditedAt time.Time, events []ContentEvent) bool {
	if !EffectiveConfig().Configured() {
		return false // P1: no roster ⇒ no blessing authority ⇒ nothing is blessed
	}
	var bless time.Time
	for _, e := range events {
		// The id==0 arm IsBlessAuthorityID admits here is dead in practice:
		// trustedContentAuthor (used below) requires id!=0 for a human login, so an
		// id-less blessing event accepted here would immediately void itself as
		// untrusted content at its own timestamp (last == bless, tie fails closed).
		// Kept via IsBlessAuthorityID rather than inlined strict, since that is the
		// general exported blessing-authority check with its own documented
		// id-absent semantics (ported comment from statusgen/trustgate.go's T10).
		if IsBlessAuthorityID(e.Author, e.AuthorID) && e.CreatedAt.After(bless) {
			bless = e.CreatedAt
		}
	}
	if bless.IsZero() {
		return false
	}
	// T3: use !Before rather than After so a tie (same-second edit) voids
	// the blessing — fail-closed on ties. Ported from statusgen/trustgate.go;
	// the two copies are a documented duplicate (KEEP IN SYNC).
	if !bodyEditedAt.Before(bless) {
		return false // body edited at/after the blessing — void
	}
	for _, e := range events {
		if trustedContentAuthor(e.Author, e.AuthorID) {
			continue
		}
		last := e.CreatedAt
		if e.EditedAt.After(last) {
			last = e.EditedAt
		}
		// T3: tie voids the blessing — fail-closed on same-second edits.
		if !last.Before(bless) {
			return false // untrusted content added/edited at/after the blessing
		}
	}
	return true
}

// ItemTrustedEvents is the edit-aware trust decision: trusted authorship admits the
// item outright (no timestamp logic — the author's own edits are trusted); anything
// else needs a CURRENT blessing per Blessed. This is the form every gate surface
// with edit-timestamp access must use.
func ItemTrustedEvents(authorLogin string, bodyEditedAt time.Time, events []ContentEvent) bool {
	if TrustedAuthor(authorLogin) {
		return true
	}
	return Blessed(bodyEditedAt, events)
}

// TrustedLogins returns the full accepted login set, sorted (for help text, the
// README and the P3 echo — never for widening decisions, which go through
// TrustedAuthor).
func TrustedLogins() []string {
	c := EffectiveConfig()
	out := make([]string, 0, len(c.Logins))
	for l := range c.Logins {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// BlessAuthorityLogin returns the configured blessing authority's login, or ""
// when the roster is unconfigured. For messages and the echo only.
func BlessAuthorityLogin() string {
	c := EffectiveConfig()
	if !c.Configured() {
		return ""
	}
	return c.Bless.Login
}

// RoleAppLogin returns the REST rendering ("<slug>[bot]") of the App bound to a
// desk role by the `role=` prefix on ASSAY_TRUSTED_BOT_SLUGS, and whether the
// role is BOUND at all.
//
// The ok return is not decoration, and it is why this function cannot go back to
// returning a bare string. The previous shape answered "" for an unbound role and
// told every caller in a doc comment to treat that as a refusal. No caller did:
// each one compared an actor login against the result, so an unbound role turned
// an identity comparison into `login == ""` — a comparison that a real GitHub
// response can SATISFY. A deleted review author decodes to `Login: ""` (the REST
// shape is `"user": null`), so a review nobody wrote matched the reviewer App and
// satisfied the flip gates; an Evidence commit carrying no author name matched the
// verifier App and was reported as PROVEN attribution, with the could-not-check
// third state unreachable. Both were live in an earlier version and both were reachable
// from a single dropped `role=` prefix — a typo the loader accepted silently.
//
// So the refusal is now in the TYPE. A caller that ignores ok does not compile
// into a working comparison, and the empty-string-as-identity shape is gone.
// Callers must use the package-local matcher helpers (isReviewerBot and friends),
// which answer false for an unbound role AND for an empty actor login.
func RoleAppLogin(role string) (login string, ok bool) {
	c := EffectiveConfig()
	if !c.Configured() {
		return "", false
	}
	slug, bound := c.RoleBots[strings.ToLower(role)]
	if !bound || strings.TrimSpace(slug) == "" {
		return "", false
	}
	return slug + "[bot]", true
}

// RoleAppLoginOrEmpty is the DISPLAY-ONLY rendering of a role binding, for
// message text ("expected <slug>[bot]"). It renders "(unbound)" rather than an
// empty string so a diagnostic never silently reads as a blank identity. It must
// never be used in a comparison — RoleAppLogin's ok return is the only admissible
// basis for an identity decision.
func RoleAppLoginOrEmpty(role string) string {
	login, ok := RoleAppLogin(role)
	if !ok {
		return "(unbound role " + strings.ToLower(role) + ")"
	}
	return login
}

// RoleBotCommitIdentity returns the git commit identity (name, email) a desk role's App must
// author with — "<slug>[bot]" and "<botUserID>+<slug>[bot]@users.noreply.github.com" — with
// BOTH the slug and the deployment-specific numeric bot USER id sourced from the roster
// (ASSAY_TRUSTED_BOT_SLUGS), never a source literal. The bot USER id is a real, per-deployment
// account id: baking it into a source constant leaks it into the publicly-staged tool, which is
// what the tree sweep refuses. (#638 is the separate rule that the email PREFIX must be the bot
// USER id, not the App id, so a commit links to an account; this keeps that id in config.) ok is
// false when the role is unbound or its bot user id is unpinned (0), so the caller refuses
// rather than stamping an account-unlinked or empty identity.
func RoleBotCommitIdentity(role string) (name, email string, ok bool) {
	c := EffectiveConfig()
	if !c.Configured() {
		return "", "", false
	}
	slug, bound := c.RoleBots[strings.ToLower(role)]
	if !bound || strings.TrimSpace(slug) == "" {
		return "", "", false
	}
	id, hasID := c.Bots[slug]
	if !hasID || id <= 0 {
		return "", "", false
	}
	return slug + "[bot]", fmt.Sprintf("%d+%s[bot]@users.noreply.github.com", id, slug), true
}

// RoleBound reports whether a desk role has an App bound to it. Tools that need a
// role to do their job call this at startup and refuse loudly rather than
// discovering the unbound state inside a comparison.
func RoleBound(role string) bool {
	_, ok := RoleAppLogin(role)
	return ok
}

// RequireRole is the loud refusal for a tool whose whole function depends on
// knowing which App's action counts.
func RequireRole(role string) error {
	if RoleBound(role) {
		return nil
	}
	return fmt.Errorf("the desk role %q is NOT BOUND to any App: %s carries no %s= prefix "+
		"(or the roster is unconfigured), so this tool cannot tell which App's action counts. "+
		"Refusing rather than comparing against an empty identity — an empty identity MATCHES a "+
		"deleted GitHub author. Source consulted: %s.",
		strings.ToLower(role), EnvTrustedBotSlugs, strings.ToLower(role), EffectiveConfig().Source)
}
