package trust

import (
	"fmt"
	"sort"
	"strings"
)

// Human-name assertions (ASSAY_ASSERTED_HUMANS).
//
// LooksLikeBot is a string test. Two of its clauses are forge-STRUCTURAL: a `[bot]` suffix
// and an `app/` prefix are renderings the forge itself produces for an App account, so no
// configuration can say otherwise. The other two (`-app`, `-bot` suffixes) are NAME SHAPE:
// a convention for naming service accounts, which a human's own username can also match.
//
// An assertion is the one roster statement that a name-shaped login is a human. It is
// deliberately narrow:
//
//   - it is PINNED: `login:id`, both halves mandatory, the id a positive number. A bare
//     login is refused. The waiver applies only where the roster entry carries the SAME
//     login AND the SAME id; the same login with another id, or the id with another
//     login, does not pass.
//   - it waives ONLY the name-shape clauses. A `[bot]` / `app/` login is refused as an
//     assertion, and so is a login the name check never refused (there is nothing to
//     waive), a login the roster registers as a bot account, a repeated login and a
//     repeated id.
//   - it is consumed by ASSAY_TRUSTED_LOGINS and ASSAY_BLESS_LOGIN only. Every other key
//     that refuses a bot-shaped login keeps refusing it.
//   - every use is echoed (the effective-config block, and so its digest).
//
// WHERE THE FORGE'S OWN BOT FLAG IS READABLE. The roster loader is offline: it reads the
// roster file or the CI variables and never calls a forge, so at LOAD the assertion plus
// its pinned id is all that is checked. At RUN time a forge-marked bot wins over the
// assertion wherever the read carries the forge's actor kind for that identity — see
// ForgeMarkedBot and AssertionOverruled for the kinds and the sites. Where a read carries
// only a login (and at most an id), the pinned id is the control.

// HumanAssertionsKey is the roster key. It is an ASSAY_ (methodology) key and is listed in
// KnownRosterKeys.
const HumanAssertionsKey = "ASSAY_ASSERTED_HUMANS"

// HumanAssertions maps a lowercased login to its asserted, pinned numeric id.
type HumanAssertions map[string]int64

// NameShapeBot reports whether login trips LooksLikeBot through a name-shape clause only
// (`-app` / `-bot` suffix) and through neither forge-structural clause.
func NameShapeBot(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	if strings.HasSuffix(l, "[bot]") || strings.HasPrefix(l, "app/") {
		return false
	}
	return strings.HasSuffix(l, "-app") || strings.HasSuffix(l, "-bot")
}

// ParseHumanAssertions parses the key's value. botLogins is the set of logins the roster
// already registers for bot accounts (every forge's rendering). Any problem refuses the
// whole roster, like every other trust-surface problem; each problem starts with the key.
func ParseHumanAssertions(raw string, botLogins map[string]bool) (HumanAssertions, []string) {
	out := HumanAssertions{}
	var problems []string
	bad := func(format string, a ...any) {
		problems = append(problems, HumanAssertionsKey+": "+fmt.Sprintf(format, a...))
	}
	byID := map[int64]string{}
	for _, entry := range SplitList(raw) {
		login, id, ok := SplitIdentity(entry)
		if !ok || id <= 0 {
			bad("entry %q is not a pinned login:id pair — an assertion needs both the login and "+
				"its positive numeric forge id; a bare login is never accepted", entry)
			continue
		}
		if IsBot(login) {
			bad("entry %q names a forge-structural bot rendering (a [bot] suffix or an app/ prefix). "+
				"The forge itself renders that account as an App; no assertion waives it", entry)
			continue
		}
		if !NameShapeBot(login) {
			bad("entry %q asserts nothing — the login does not end in -bot or -app, so the name "+
				"check never refused it. Strike the entry", entry)
			continue
		}
		if botLogins[login] {
			bad("entry %q names a login the roster registers as an App/bot account. An assertion "+
				"never turns a configured bot into a human", entry)
			continue
		}
		if _, dup := out[login]; dup {
			bad("login %q is asserted more than once — one pinned id per login", login)
			continue
		}
		if other, dup := byID[id]; dup {
			bad("id %d is asserted for both %q and %q — one forge id is one account", id, other, login)
			continue
		}
		out[login] = id
		byID[id] = login
	}
	return out, problems
}

// Waives reports whether the assertion set exempts this exact login:id pair from the
// name-shape check. The id must be present and equal to the pin; a forge-structural login
// is never waived.
func (a HumanAssertions) Waives(login string, id int64) bool {
	if id <= 0 || !NameShapeBot(login) {
		return false
	}
	want, ok := a[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(login), "@")))]
	return ok && want == id
}

// RefusalHint is appended to a name-shape refusal so the operator can see the one way a
// human's name-shaped login loads.
func RefusalHint(login string) string {
	if !NameShapeBot(login) {
		return ""
	}
	return fmt.Sprintf(" (no pinned %s entry for that exact login:id; a human whose username ends "+
		"in -bot or -app loads only through one, on a login:id entry carrying the same id)", HumanAssertionsKey)
}

// UnusedProblems returns one problem per assertion that no consuming entry used. An unused
// assertion is refused so a stale one cannot sit waiting for a later entry to match it.
func UnusedProblems(a HumanAssertions, uses map[string][]string, consumers ...string) []string {
	var out []string
	logins := make([]string, 0, len(a))
	for l := range a {
		logins = append(logins, l)
	}
	sort.Strings(logins)
	for _, l := range logins {
		if len(uses[l]) == 0 {
			out = append(out, fmt.Sprintf("%s: %q is asserted but no %s entry carries that pinned "+
				"login:id pair. An unused assertion is refused", HumanAssertionsKey, l, strings.Join(consumers, " or ")))
		}
	}
	return out
}

// EchoAssertions renders the key's echo value: every assertion and the keys that used it,
// sorted, so a reviewer can see each waiver in the run.
func EchoAssertions(a HumanAssertions, uses map[string][]string) string {
	out := make([]string, 0, len(a))
	for l, id := range a {
		u := append([]string(nil), uses[l]...)
		sort.Strings(u)
		out = append(out, fmt.Sprintf("%s:%d (name check waived for %s)", l, id, strings.Join(u, ",")))
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// ForgeMarkedBot reports whether a forge actor-kind word marks the account as a bot:
// "Bot" (GitHub GraphQL __typename and REST user.type) or "GitLabBot" (the desk tools'
// GitLab trust read, from the GitLab `bot` field). Empty is could-not-check and answers
// false: where the kind is not carried, the pinned id is the control.
func ForgeMarkedBot(kind string) bool {
	k := strings.ToLower(strings.TrimSpace(kind))
	return k == "bot" || k == "gitlabbot"
}

// AssertionOverruled reports whether a forge-marked bot is presenting an asserted login.
// When it is, the read must treat that identity as untrusted: the forge's bot flag wins
// over the roster's assertion.
func AssertionOverruled(a HumanAssertions, login string, forgeBot bool) bool {
	if !forgeBot || len(a) == 0 {
		return false
	}
	_, ok := a[strings.ToLower(strings.TrimSpace(login))]
	return ok
}

// DroppedKeys lists every ASSAY_ key with a value in a roster that was refused as a whole,
// with the reason it was not applied. A refused roster loads nothing (fail closed, P1);
// this only makes the discard visible, so an operator can see why a well-formed key reads
// empty. A key whose own value is named in a problem is reported as refused; every other
// key as discarded with the roster.
func DroppedKeys(vals map[string]string, problems []string) []string {
	if len(problems) == 0 {
		return nil
	}
	keys := make([]string, 0, len(vals))
	for k, v := range vals {
		if strings.HasPrefix(k, "ASSAY_") && strings.TrimSpace(v) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		own := false
		for _, p := range problems {
			if strings.HasPrefix(p, k+":") || strings.HasPrefix(p, k+" ") || strings.HasPrefix(p, k+"=") {
				own = true
				break
			}
		}
		if own {
			out = append(out, k+" — refused: its own value is named in a REFUSED line")
		} else {
			out = append(out, k+" — not applied: discarded with the refused roster (its own value "+
				"was not refused; it loads once every REFUSED line is fixed)")
		}
	}
	return out
}
