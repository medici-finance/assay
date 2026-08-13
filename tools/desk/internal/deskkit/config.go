package deskkit

import (
	"fmt"
	"sort"
	"strings"
)

// repoPolicy is the desk policy for ONE allowed repo. Keeping the policy IN the repo
// entry (rather than in a parallel list elsewhere) is deliberate: the drift anti-pattern was exactly a
// parallel list going stale — deskpost's CI-less set drifted from the allowed set, so
// `deskpost ready` could never flip the CI-less report repos. Configuring a repo now
// FORCES a CI decision alongside it.
type repoPolicy struct {
	// CIRequired is true when the repo runs PR CI, so an EMPTY status/check rollup at a
	// head means "checks have not reported yet" (unverifiable, exit 6) rather than "green".
	// It is false ONLY for repos known to have no PR CI at all, where an empty rollup is
	// all there will ever be.
	CIRequired bool

	// Visibility is the repo's GitHub visibility. A PUBLIC repo is
	// risk-classed unconditionally — see VisibilityRiskClassed. The ZERO value is
	// VisibilityUnknown, so a repo configured without stating a visibility fails
	// CLOSED (risk-classed) instead of silently inheriting "private".
	//
	// It is CONFIGURED, not read from the API at decision time, for the same reason
	// the whole allowed set is: a gate that needs the network is a gate that fails open
	// when the network does. The staleness risk that buys is paid for by the drift
	// check (VisibilityDrift + `deskboard policydrift`), which is exactly the drift anti-pattern
	// medicine — a hand-maintained value with no drift check IS the drift anti-pattern.
	Visibility Visibility
}

// Visibility is a repo's GitHub visibility as configured. The zero value is
// VisibilityUnknown on purpose: "not stated" must never read as "private".
type Visibility int

const (
	// VisibilityUnknown — not stated, or an API value this code does not recognise.
	// Fails closed everywhere it is consumed, and is reported as drift. It is the
	// ZERO value so an unstated visibility can never read as "private".
	VisibilityUnknown Visibility = iota
	// VisibilityPublic — world-readable. Risk-classed unconditionally.
	VisibilityPublic
	// VisibilityPrivate — the ONLY value that does not risk-class on visibility alone.
	VisibilityPrivate
)

func (v Visibility) String() string {
	switch v {
	case VisibilityPublic:
		return "public"
	case VisibilityPrivate:
		return "private"
	default:
		return "unknown"
	}
}

// ParseVisibility maps a GitHub API `visibility` string to a Visibility. ONLY the two
// exact strings "public" and "private" parse. Anything else — "internal", "", a future
// value, a truncated read — is VisibilityUnknown, which fails closed at every consumer
// and is reported as drift rather than guessed at.
func ParseVisibility(s string) Visibility {
	switch s {
	case "public":
		return VisibilityPublic
	case "private":
		return VisibilityPrivate
	default:
		return VisibilityUnknown
	}
}

// allowedRepos is the desk tools' WRITE-AUTHORISATION set: the repos
// any tool may act on. Any tool acting on a repo outside it refuses (exit 5).
//
// It is CONFIGURED, not compiled in — but configured from
// OUTSIDE every ref the tools evaluate: the ASSAY_ALLOWED_REPOS repository/organization
// Actions variable in CI, or the config-home file locally. No flag, no PR-supplied
// input, and no environment read on the write path may widen it. Widening this set
// widens the blast radius of every desk tool, so the effective value is echoed on every
// run (P3) — a change appears in run output and CI history rather than only in settings.
//
// The value format is `owner/name[:ci|:no-ci][:public|:private]`, and BOTH policy
// defaults fail closed: a repo configured with no CI token is CI-required (an empty
// status rollup then reads as "not reported yet", never as green), and one with no
// visibility token is VisibilityUnknown, which risk-classes every PR on it.
//
// An entry MAY instead be a PATTERN of the form `owner/*` (extended to
// configuration): it admits every CURRENT AND FUTURE repo under that owner to
// IsAllowedRepo, with NO code edit and NO explicit entry required — this is what let
// a compiled-in org-default census become a configured one without losing the
// property. That census enumerated a SUBSET of the owner's repositories, so a
// newly-created repo stayed invisible to the tools until someone edited and rebuilt
// them; a pattern entry closes that gap without naming any of them. A pattern entry carries NO CI/visibility policy tokens: it
// widens ONLY IsAllowedRepo. CIRequired, RepoVisibility, VisibilityDrift and
// AllowedRepos all read the explicit map alone (see EffectiveConfig().RepoPatterns
// and IsAllowedRepo below) — a repo matched only by a pattern is CI-required (fail
// closed) and VisibilityUnknown (risk-classed) until an explicit entry states
// otherwise, exactly as it was under the former compiled-in design.
//
// CIRequired must reflect each repo's ACTUAL CI and Visibility its ACTUAL visibility;
// re-check with `gh api repos/<repo> --jq .visibility` before changing a value, and run
// `deskboard policydrift` to prove the configured set still matches the world. That
// drift check is the drift anti-pattern medicine: the drift anti-pattern was a hand-maintained parallel list going
// stale and silently disabling a gate.
//
// A private review channel configured by the operator is DELIBERATELY ABSENT from the
// configured set. It used to be a literal entry compiled into this
// map, which meant the channel's repo name shipped inside every desk binary — the
// disclosure was the name, not the comment above it, so stripping prose alone would have
// changed nothing. Converting the whole map to configuration (#455, below) carries that
// property forward rather than reopening it: nothing in this source tree names the
// channel, so there is nothing here to ship. The desk tools have no write authority to
// any such channel; filing into it is a human act. If a channel must become act-allowed
// again it belongs in operator configuration (ASSAY_ALLOWED_REPOS / the config-home
// file), never back in this literal.
func allowedRepos() map[string]repoPolicy {
	return EffectiveConfig().Repos
}

// Carve-out: deskadvisory (tools/desk/cmd/deskadvisory/) operates on repos
// outside this set — but it reaches them only by starting from a repo that IS in
// this set, following its advisory private_fork pointer. The configured set
// is never widened by a caller; the fork is reached only via the authoritative pointer
// on the allowlisted base. See the deskadvisory design notes.
//
// IsAllowedRepo reports whether repo (owner/name form) is in the configured set —
// either an explicit owner/name entry, or matched by an owner/* PATTERN entry (see
// allowedRepos's doc comment and rosterconfig.go's parsing of ASSAY_ALLOWED_REPOS).
// A caller that gets false must refuse with ExitRefused — this function makes NO
// network call and trusts NO caller-supplied override. An UNCONFIGURED set answers
// false for every repo (P1: unset is closed), which refuses rather than admits.
//
// Matching is exact-case on both the explicit map and the pattern owner, consistent
// with the rest of this package's repo matching (see splitRepo and its test) — GitHub
// resolves case variants to the same repo, but this code has never treated that as
// its job to normalise; a caller that needs it must re-case before calling.
func IsAllowedRepo(repo string) bool {
	if _, ok := allowedRepos()[repo]; ok {
		return true
	}
	owner, name := splitRepo(repo)
	if owner == "" || name == "" {
		return false
	}
	for _, pat := range EffectiveConfig().RepoPatterns {
		if owner == pat {
			return true
		}
	}
	return false
}

// CIRequired reports whether an EMPTY CI rollup on repo must be treated as unverifiable
// rather than green. It is the ONE source for that question — no tool may keep a
// second CI-less list. A repo outside the configured EXPLICIT set is CI-required: fail
// closed. This includes a repo matched only by an owner/* PATTERN — patterns widen
// IsAllowedRepo alone (see its doc comment), never this function.
func CIRequired(repo string) bool {
	p, ok := allowedRepos()[repo]
	if !ok {
		return true
	}
	return p.CIRequired
}

// RepoVisibility returns the CONFIGURED visibility of repo. A repo outside the allowed
// set — or one configured with no stated visibility — is VisibilityUnknown. A repo
// matched only by an owner/* PATTERN is VisibilityUnknown for the same reason CIRequired
// fails closed on it: patterns carry no policy, only admission.
// This function makes NO network call; the live value is checked by VisibilityDrift.
func RepoVisibility(repo string) Visibility {
	p, ok := allowedRepos()[repo]
	if !ok {
		return VisibilityUnknown
	}
	return p.Visibility
}

// VisibilityRiskClassed reports whether repo is risk-classed on VISIBILITY ALONE,
// independent of which files a PR touches.
//
// The rule is stated in the fail-closed direction on purpose: everything EXCEPT a
// known-private repo is risk-classed. So an unknown repo, a repo whose policy entry
// omits the visibility, and a public repo all answer true; only VisibilityPrivate
// answers false. There is no input on which this can waive the gate by accident.
//
// POLICY CALL (the public-repo risk rule suggested-fix, decided in the secure direction pending
// the owner): a PUBLIC repo is risk-classed for EVERY PR, whatever it touches. Before this,
// the path triggers were all tracker-shaped, so the public infrastructure repo — the one
// whose diffs an outsider can read — could never be risk-classed at all, and got
// strictly LESS mandatory security scrutiny than the private application repo. This
// inverts that. Relaxing it later is a one-line change; the reverse mistake ships
// un-reviewed Identity diffs to a public repo.
func VisibilityRiskClassed(repo string) bool {
	return RepoVisibility(repo) != VisibilityPrivate
}

// VisibilityDrift compares the configured visibility of every allowed repo against
// values OBSERVED from the GitHub API (repo -> the API's `visibility` string) and
// returns one human-readable line per disagreement. An empty result means the table
// matches the world.
//
// This is the drift anti-pattern medicine. The drift anti-pattern was a hand-maintained parallel list going stale and
// silently disabling a gate; a hand-maintained visibility field with no drift check
// would repeat it exactly. The check is DELIBERATELY not part of the gate decision:
// deskpost/deskboard read the configured value (no network on the hot path, so the
// gate cannot fail open when GitHub is down), and this runs separately and loudly.
//
// A repo the caller did not observe is drift, not a pass — a check that silently
// skips what it could not read verifies nothing.
func VisibilityDrift(observed map[string]string) []string {
	return visibilityDriftAgainst(allowedRepos(), observed)
}

// visibilityDriftAgainst is VisibilityDrift against an explicit policy table, so a test
// can point it at a deliberately-wrong table and prove the check catches it.
func visibilityDriftAgainst(policy map[string]repoPolicy, observed map[string]string) []string {
	var out []string
	repos := make([]string, 0, len(policy))
	for r := range policy {
		repos = append(repos, r)
	}
	sort.Strings(repos)
	for _, repo := range repos {
		want := policy[repo].Visibility
		raw, seen := observed[repo]
		if !seen {
			out = append(out, fmt.Sprintf("%s: NOT OBSERVED — configured %q could not be checked "+
				"(an unchecked repo is drift, never a pass)", repo, want))
			continue
		}
		got := ParseVisibility(raw)
		if want == VisibilityUnknown {
			out = append(out, fmt.Sprintf("%s: configured visibility is UNSET (fails closed, so every PR "+
				"is risk-classed); the API says %q — state it in ASSAY_ALLOWED_REPOS", repo, raw))
			continue
		}
		if got == VisibilityUnknown {
			out = append(out, fmt.Sprintf("%s: API returned unrecognised visibility %q (configured %q) — "+
				"only \"public\"/\"private\" are understood", repo, raw, want))
			continue
		}
		if got != want {
			out = append(out, fmt.Sprintf("%s: DRIFT — configured %q, API says %q", repo, want, got))
		}
	}
	// An observation for a repo outside the table is a caller bug, and silently
	// dropping it is how a stale set hides. Report it.
	extra := make([]string, 0)
	for repo := range observed {
		if _, ok := policy[repo]; !ok {
			extra = append(extra, repo)
		}
	}
	sort.Strings(extra)
	for _, repo := range extra {
		out = append(out, fmt.Sprintf("%s: observed but NOT in the allowed repo set — "+
			"the caller and the configured set disagree", repo))
	}
	return out
}

// AllowedRepos returns the configured EXPLICIT repo set as a sorted slice of CONCRETE
// "owner/name" slugs (for help text, banners and the P3 echo). Every element is a real
// repo, so callers may pass each straight to `gh api repos/<repo>` — deskboard,
// deskroster and `deskboard policydrift` all do.
//
// It deliberately EXCLUDES owner/* pattern entries: IsAllowedRepo
// also admits any repo matched by a configured pattern, and that set cannot be
// enumerated without an API call. Use IsAllowedRepo to ASK about a repo; use this only
// to ITERATE the repos whose policy is explicitly configured.
//
// A pattern MUST NOT be returned here. It was, briefly, in the compiled-in predecessor
// of this design, and it is a live break rather than a cosmetic one: every production
// caller treats each element as a slug, so an "owner/*" element becomes
// `gh api repos/owner/*`, which errors — and an unreadable repo fails the
// WHOLE board run. Patterns are display-only and live in AllowedRepoScope.
func AllowedRepos() []string {
	repos := allowedRepos()
	out := make([]string, 0, len(repos))
	for r := range repos {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// AllowedRepoScope returns the human-readable SCOPE for help text and banners: the
// configured explicit repo set (AllowedRepos) plus each configured owner/* pattern,
// rendered "owner/*" and appended after the sorted explicit set. Separate from
// AllowedRepos precisely because a pattern is not iterable against the API — never pass
// an element of this slice to `gh api repos/<repo>`.
func AllowedRepoScope() []string {
	out := AllowedRepos()
	pats := append([]string(nil), EffectiveConfig().RepoPatterns...)
	sort.Strings(pats)
	for _, p := range pats {
		out = append(out, p+"/*")
	}
	return out
}

// splitRepo splits "owner/name" into its components. Returns empty strings unless the
// input contains EXACTLY one "/".
//
// The exactly-one requirement is defence in depth, not pedantry. IsAllowedRepo's
// pattern match works on the owner half alone, so a SplitN(…, 2) that tolerated extra
// segments answered TRUE for "medici-finance/../example-org/x" — the owner reads as
// medici-finance and the rest is ignored. Not reachable from deskpost today (its own
// splitRepo already rejects it, and an installation token would not authorise the other
// org anyway), but the allow-check must not depend on a caller upstream getting it right.
func splitRepo(repo string) (owner, name string) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

// ScanRepos returns the configured intake SCAN scope (ASSAY_SCAN_REPOS) in the
// order it was declared: the owned-repo set the issue lane READS. It is the
// desk-side half of the owned-repo roster this externalises out of source; the
// scanner half (statusgen's scanRepos) reads the identical key, so a single roster
// value now feeds both and they cannot drift. Unset yields an empty slice — the
// read surface is then empty (fail safe), never a fallback to the write boundary.
func ScanRepos() []string {
	return EffectiveConfig().ScanRepos
}

// BoardScopeError is the loud refusal every READ path emits when it cannot name a
// single repo to read (#489). It is the read-side twin of the write-side
// IsAllowedRepo refusal, and it exists because those two boundaries failed
// DIFFERENTLY on the same empty set: a write verb handed an unknown repo refused at
// exit 5, while a board verb handed NO repos swept nothing and reported the empty
// sweep as a clean result at exit 0.
//
// It checks the two axes in order, because they are genuinely independent and the
// first has the better message:
//
//  1. RosterUnconfiguredError — nothing configured at all. This is that function's
//     first caller in tools/desk (#489 item 4); it was documented as "the loud
//     refusal every trust-gated caller emits" while being called from nowhere but
//     its own tests, so the refusal existed only on statusgen's --scan-issues path.
//
//  2. An EMPTY repo set on an otherwise-usable roster. Configured() deliberately
//     does not consult Repos — it answers a question about TRUST (who may be
//     believed), not about SCOPE (what may be read) — so a roster carrying a
//     blessing authority and trusted logins but no ASSAY_ALLOWED_REPOS answers
//     configured=true with nothing to read. That partial state is what actually
//     reproduced #489, and RosterUnconfiguredError returns nil in it, so this
//     second check is the load-bearing one rather than a belt on axis 1's braces.
//
// The exit code is 6 (unverifiable), not 5 (refused), and the distinction is the
// whole point: the caller asked a question this process could not answer, which is
// not the same as asking one it is not allowed to answer. An empty sweep is
// COULD-NOT-CHECK, never green-and-empty.
//
// It returns nil the moment one repo is configured — it says nothing about whether
// that repo could be REACHED, which stays each caller's obligation.
func BoardScopeError() error {
	if err := RosterUnconfiguredError(); err != nil {
		return err
	}
	if len(allowedRepos()) == 0 {
		// A pattern-only configuration (owner/* with no explicit entries) does NOT
		// satisfy this check, on purpose: patterns widen IsAllowedRepo (the write
		// boundary) but cannot be enumerated without an API call, so a board READ
		// still has zero concrete repos to sweep. See AllowedRepos's doc comment for
		// the same distinction on the write side.
		c := EffectiveConfig()
		return Unverifiable(fmt.Sprintf(
			"COULD-NOT-CHECK: the allowed-repo set is EMPTY, so this read swept ZERO repos "+
				"and knows nothing (#489).\n"+
				"  An empty sweep is not an empty board: reporting it as one turns \"I read nothing\" "+
				"into \"there is nothing\", and a tombstone rule built on that difference then "+
				"announces every remembered PR as merged.\n"+
				"  The rest of the roster IS loaded (bless/trusted-logins are set), which is why "+
				"nothing else complained — %s answers a question about TRUST, not about SCOPE.\n"+
				"Source consulted: %s (class %s).\n"+
				"CI: set the repository or organization Actions variable %s.\n"+
				"Locally: add it to %s, mode 0600 — write-class tools do NOT read the environment.\n"+
				"Format: owner/name[:ci|:no-ci][:public|:private], or owner/* to admit every repo "+
				"under owner (IsAllowedRepo only — see allowedRepos's doc comment), comma-separated.",
			EnvBlessLogin, c.Source, c.Class, EnvAllowedRepos, ConfigHomePath()), nil)
	}
	return nil
}
