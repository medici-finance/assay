package deskkit

// selfcontain.go — the PUBLIC-REPO SELF-CONTAINMENT scan (#203).
//
// WHAT IT IS FOR, and why the secret scan does not already cover it. bodycheck.go asks one
// question of every outward body: does this text carry a CREDENTIAL. That question is
// answered identically whatever repo the body is going to, and it is the right question for
// a private repo, where the readership is already inside the house.
//
// A PUBLIC repo has a second failure mode the credential question cannot see. A body can be
// entirely free of secrets and still not be SELF-CONTAINED: it can carry a private
// repository's name, a cross-repo issue reference that resolves only inside the house, an
// absolute path off the author's own machine, a dispatch session id, a scratch worktree
// name, or an identifier out of a register that is deliberately not published. None of that
// is a credential; all of it is disclosure, and every category has reached a public surface
// at least once. Until now the only control was a sentence in a skill body that each worker
// had to remember — which is the shape of control that has already failed here repeatedly
// (#685's hand-maintained counts, #592, #627). #203 is the ask to move it into the tool.
//
// THE SPLIT BETWEEN REFUSE AND NOTICE is the three-state instrument rule, not a softness.
// A category REFUSES when the span it matches is unambiguous — a full `owner/name` slug of a
// repo the roster says is private, an absolute path under a machine-local root, a session
// UUID. A category NOTICES when the check could not actually decide: a bare `#N` with no
// reference number to compare against, a bare short name that is also an ordinary English
// word, a withheld-identifier set the deployment never configured. A notice says what was
// NOT checked, in its own words, rather than rounding a could-not-check up to a pass or down
// to a refusal the author cannot act on. The expensive direction here is the false refusal
// (#209, #1255: refusing legitimate `file:line` references starved the desk's write budget),
// so the ambiguous half deliberately warns.
//
// WHAT IS NEVER COMPILED IN. The private repo names, the aliases and the withheld register
// identifiers are all DEPLOYMENT VALUES read at run time from the roster
// (ASSAY_ALLOWED_REPOS, ASSAY_REPO_ALIASES) and from EnvWithheldIdentifiers. Embedding any
// of them in this file would make the shipped binary the leak the scan exists to prevent —
// the same reasoning sweepconfig.go records for the disclosure sweep's routed-away set. An
// adopter with none of it configured gets a complete, usable configuration: the machine-
// shaped categories (absolute paths, session ids, worktree names) still work, and the
// register category degrades to a NOTICE.

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SurfaceBodyPublic is the surface name for the public-repo self-containment scan, named
// alongside SurfaceBody (bodycheck.go) so a refusal says which scan produced it. Callers
// that scan a named surface pass their own noun phrase ("PR body", "comment body") and this
// is the default.
const SurfaceBodyPublic = "public body"

// minShortNameNotice is the shortest private-repo SHORT NAME that earns a notice. See the
// noise-floor note at its only use: shorter tokens are ordinary English and a notice keyed
// on one fires on nearly every body. The full `owner/name` slug refuses at any length, so
// this bounds only the advisory half.
const minShortNameNotice = 4

// EnvWithheldIdentifiers names the environment variable carrying the register identifiers a
// PUBLIC body must not name — stream slugs and brief ids that live in a register the house
// does not publish, comma-separated:
//
//	ASSAY_WITHHELD_IDENTIFIERS=<slug>,<other-slug>
//
// It is HOUSE configuration and is never compiled in, for the reason recorded at the top of
// this file. UNSET is not an error and not a refusal: an adopter with no withheld register
// gets a NOTICE saying the category was not checked, and every other category still runs.
//
// A configured slug matches both the bare slug and any `<slug>/<NN>` brief id under it.
//
// KEEP IN SYNC with parseConfig's `known` map in rosterconfig.go: an unrecognised key in the
// ASSAY_ namespace REFUSES the whole roster, so a house that sets this without the key being
// recognised would collapse every desk tool's configuration at once.
const EnvWithheldIdentifiers = "ASSAY_WITHHELD_IDENTIFIERS"

// SelfContainOpts is what the scan needs to know about the write it is guarding.
type SelfContainOpts struct {
	// Repo is the owner/name the body will be posted to. The scan never flags the target
	// repo's own name or short label.
	Repo string
	// NumberHint is a number known to exist on Repo — the PR being replied to, the issue a
	// trailer resolves to. It is the only offline evidence available for the bare-`#N`
	// heuristic: a body's own repo cannot own a number far past one that already exists on
	// it. Zero means "no hint", and the bare-`#N` category then reports itself unchecked
	// rather than guessing. It is never a network read: the offline envelope forbids one,
	// and a scan that reaches the network fails open when the network is down.
	NumberHint int
	// Notices is where NOTICE lines are written. nil means os.Stderr. Desks run silent on
	// stdout, so notices go to stderr exactly as PublicRepoGate's bless notice does.
	Notices io.Writer
}

var (
	// reAbsMachinePath matches an absolute path under a machine-local root. These roots are
	// GENERIC (every unix developer machine and every CI runner has them) rather than
	// house-specific, so naming them here discloses nothing. `/tmp/tracker-` and
	// `/private/tmp/tracker-` are the sanctioned worktree prefix deskwt mints under.
	//
	// The trailing `+` — at least ONE character past the root — is load-bearing and is the
	// #380 lesson applied before it can bite. A guard that fires on its own documentation
	// gets routed around rather than fixed, and the ROOT ALONE is what documentation of this
	// check is made of: `deskpr --help`, the worker kit clause, this file's own comments and
	// the PR body that introduces it all have to spell `/Users/` to say which roots are
	// covered. A bare root with nothing after it also discloses nothing — `/Users/` names no
	// user, no machine and no directory — while a real leaked path always carries at least
	// one component. So the one shape the scan must tolerate is exactly the one that carries
	// no information.
	reAbsMachinePath = regexp.MustCompile(`(?:/Users/|/home/|/private/tmp/|/private/var/folders/|/tmp/tracker-)[^\s"'` + "`" + `)\]>,;]+`)
	// reWorktreeName matches a scratch worktree directory name written WITHOUT its leading
	// path — `tracker-<item>` — which is how it most often reaches a body (a command line, a
	// "my worktree is …" sentence). deskwt mints exactly this shape (cmd/deskwt).
	reWorktreeName = regexp.MustCompile(`\btracker-[A-Za-z0-9][A-Za-z0-9._-]*`)
	// reSessionUUID matches the session id shape the agent tooling mints (a lowercase hex
	// UUID). Anchored on the full 8-4-4-4-12 grouping so an ordinary hyphenated word or a
	// git SHA cannot match it.
	reSessionUUID = regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	// reAgentID matches the dispatch tooling's agent-directory id (`agent-<hex>`).
	reAgentID = regexp.MustCompile(`\bagent-[0-9a-f]{8,}\b`)
	// reQualifiedRef matches an `owner/name` slug, optionally with a `#N` issue reference.
	// Group 1 is the slug, group 2 the number (empty when absent).
	reQualifiedRef = regexp.MustCompile(`\b([A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*)(?:#([0-9]+))?`)
	// reShortRef matches a bare short-name issue reference — `alias#N`.
	reShortRef = regexp.MustCompile(`\b([A-Za-z0-9][A-Za-z0-9._-]*)#([0-9]+)`)
	// reBareRef matches a bare `#N` that is NOT preceded by a name (reShortRef's case) and
	// not part of a fragment or a markdown heading.
	reBareRef = regexp.MustCompile(`(^|[\s(\[,;])#([0-9]+)\b`)
	// reBriefID matches a `<slug>/<NN>` brief id.
	reBriefID = regexp.MustCompile(`\b([a-z0-9][a-z0-9-]*)/([0-9]{1,3})\b`)
)

// WithheldIdentifiers returns the house-configured withheld register identifiers from
// EnvWithheldIdentifiers, lowercased, split on commas and trimmed, with empties dropped.
// A nil return is the UNSET case and is a complete adopter configuration — see the const.
func WithheldIdentifiers() []string {
	raw := strings.TrimSpace(os.Getenv(EnvWithheldIdentifiers))
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SelfContainApplies reports whether the public-repo self-containment scan applies to a
// write targeting repo.
//
// Two conditions, and each is deliberately the conservative reading of a boundary #203 drew:
//
//   - the roster must be CONFIGURED. An adopter with no roster gets no new refusals out of
//     nowhere: every repo would otherwise read as VisibilityUnknown and the scan would run
//     on writes the tool has never gated before.
//   - the repo must not be KNOWN-PRIVATE. This is VisibilityRiskClassed's rule, reused
//     rather than restated: everything except an explicitly-private repo is in scope, so a
//     repo whose entry omits its visibility is scanned (fail closed) rather than skipped.
//     #203's boundary "Private repos: unchanged" is exactly the private arm of that.
//
// It reads the CONFIGURED visibility, never the live API. That is the same split
// repovis.go documents in the opposite direction and for the same reason: a gate that has to
// reach the network fails open when the network is down, and this one runs before any write.
// PublicRepoGate still does the live read for the +1 authorization it owns.
func SelfContainApplies(repo string) bool {
	if !EffectiveConfig().Configured() {
		return false
	}
	return VisibilityRiskClassed(repo)
}

// SelfContainCheck runs the public-repo self-containment scan over content and returns a
// Refused error (exit 5) naming the first offending span and its category, or nil.
//
// It is a NO-OP when SelfContainApplies(o.Repo) is false, so callers may call it
// unconditionally; the visibility decision lives in one place rather than at each call site.
//
// NOTICE lines are written to o.Notices (default stderr) whether or not the scan refuses —
// a could-not-check is reported as itself, never suppressed by a later verdict.
//
// surface is the caller's own noun phrase ("PR body", "comment body"); empty means
// SurfaceBodyPublic.
func SelfContainCheck(surface string, content []byte, o SelfContainOpts) error {
	if !SelfContainApplies(o.Repo) {
		return nil
	}
	if surface == "" {
		surface = SurfaceBodyPublic
	}
	w := o.Notices
	if w == nil {
		w = os.Stderr
	}
	refusal, notices := selfContainScan(surface, string(content), o)
	for _, n := range notices {
		fmt.Fprintf(w, "NOTICE: %s\n", n)
	}
	if refusal != "" {
		return Refused(refusal)
	}
	return nil
}

// selfContainScan is the pure half: it returns the refusal message (empty when clean) and
// the notice lines, with no I/O and no configuration reads of its own beyond the roster
// accessors. Split out so tests can assert on both halves without capturing streams.
func selfContainScan(surface, s string, o SelfContainOpts) (refusal string, notices []string) {
	priv, privShort := privateRepoNames(o.Repo)
	refuse := func(category, span, why string) {
		if refusal == "" {
			refusal = fmt.Sprintf("refused: %s is not self-contained — %s %q %s. "+
				"A public body must stand alone for a reader outside this house; reword the span "+
				"(see `deskpr --help`, PUBLIC-REPO SELF-CONTAINMENT, for the categories)",
				surface, category, span, why)
		}
	}

	// --- category: machine-local absolute paths, worktree names, session and agent ids ---
	//
	// These are the unambiguous half of the scan: every shape here is minted by tooling and
	// none of them can be a legitimate part of a public body. They are checked FIRST so the
	// refusal a worker sees names the span that is easiest to fix.
	for _, m := range [...]struct {
		re       *regexp.Regexp
		category string
		why      string
	}{
		{reAbsMachinePath, "absolute machine path", "resolves only on the machine that wrote it"},
		{reWorktreeName, "scratch worktree name", "names a throwaway directory nobody else has"},
		{reSessionUUID, "session id", "identifies an agent session, not anything a reader can look up"},
		{reAgentID, "agent id", "identifies an agent session, not anything a reader can look up"},
	} {
		if loc := m.re.FindString(s); loc != "" {
			refuse(m.category, loc, m.why)
		}
	}

	// --- category: cross-repo references to a private repo ---
	//
	// A QUALIFIED `owner/name` slug is unambiguous, so it refuses whether or not it carries
	// a `#N`. The slug alone is the disclosure — #203's report is about a body naming a
	// house repo, not only about the issue number hanging off it.
	for _, m := range reQualifiedRef.FindAllStringSubmatch(s, -1) {
		slug := strings.ToLower(m[1])
		if !priv[slug] {
			continue
		}
		if m[2] != "" {
			refuse("cross-repo reference", m[0], "points into a repository the roster marks PRIVATE")
			continue
		}
		refuse("private repository name", m[0], "names a repository the roster marks PRIVATE")
	}
	// An `alias#N` cross-repo reference resolves through the roster's own alias map, so it
	// refuses on exactly the aliases the deployment configured and on nothing else.
	for _, m := range reShortRef.FindAllStringSubmatch(s, -1) {
		if privShort[strings.ToLower(m[1])] {
			refuse("cross-repo reference", m[0], "points into a repository the roster marks PRIVATE")
		}
	}

	// --- category: withheld register identifiers (house config only) ---
	//
	// REFUSES when the deployment configured a withheld set and the body names one of its
	// slugs or a brief id under it. NOTICES — never refuses — when the key is absent, so an
	// adopter with no withheld register is not blocked by a check it cannot satisfy.
	withheld := WithheldIdentifiers()
	if len(withheld) == 0 {
		notices = append(notices, fmt.Sprintf(
			"%s: withheld register identifiers NOT CHECKED — %s is unset, so this scan cannot "+
				"know which stream slugs or brief ids this deployment withholds. Every other "+
				"category ran.", surface, EnvWithheldIdentifiers))
	} else {
		// Brief ids FIRST, so a `<slug>/<NN>` span is reported whole. The bare-slug arm
		// below would otherwise fire on the slug half and name a shorter span than the one
		// the author has to edit — a refusal that under-reports its own span costs a round
		// trip, which is #328's lesson about surface naming applied one level down.
		lower := strings.ToLower(s)
		for _, m := range reBriefID.FindAllStringSubmatch(s, -1) {
			for _, id := range withheld {
				if strings.ToLower(m[1]) == id {
					refuse("withheld register identifier", m[0],
						"names a brief in a register this deployment does not publish")
				}
			}
		}
		for _, id := range withheld {
			if !containsToken(lower, id) {
				continue
			}
			refuse("withheld register identifier", id,
				"names an entry in a register this deployment does not publish")
		}
	}

	// --- notice-only: bare `#N`, and bare private short names ---
	//
	// Both are HEURISTIC, and #203 asks for a warning rather than a refusal on exactly this
	// ground: a bare `#N` is the correct spelling for this repo's OWN issues (refusing it
	// would refuse the normal case), and a repo's short label is frequently an ordinary
	// word. They are reported so a human can look, never so a write is blocked.
	notices = append(notices, bareRefNotices(surface, s, o.NumberHint)...)
	for short := range privShort {
		// A short name under minShortNameNotice characters is not noticed at all. This is
		// not squeamishness about false positives — it is the console noise floor. A
		// deployment is free to alias a repo `at` or `it`, and a notice keyed on a token
		// that short fires on ordinary English in nearly every body; measured on this
		// change's own PR body, a two-letter alias produced a notice about the word "at".
		// A channel that warns on everything is a channel nobody reads, and the FULL
		// `owner/name` slug still REFUSES at any length, so nothing is lost but noise.
		if len(short) < minShortNameNotice {
			continue
		}
		if containsToken(strings.ToLower(s), short) {
			notices = append(notices, fmt.Sprintf(
				"%s: the word %q is the configured short name of a PRIVATE repository. It is not "+
					"refused (a short name is indistinguishable from ordinary prose), but check it "+
					"is not a reference a public reader cannot resolve.", surface, short))
		}
	}
	sort.Strings(notices)
	return refusal, notices
}

// bareRefNotices reports bare `#N` references the body's own repo cannot plausibly own.
//
// The heuristic is deliberately weak and says so. With a hint (the PR being replied to, the
// issue a trailer names), a number ABOVE the hint cannot yet exist on this repo, which is
// the one thing a body's own numbering makes checkable offline. Without a hint nothing is
// checkable at all, and the notice says that rather than passing silently: an instrument
// that did not look has not cleared anything.
func bareRefNotices(surface, s string, hint int) []string {
	m := reBareRef.FindAllStringSubmatch(s, -1)
	if len(m) == 0 {
		return nil
	}
	if hint <= 0 {
		return []string{fmt.Sprintf(
			"%s: %d bare `#N` reference(s) NOT CHECKED — no reference number for this repo was "+
				"available, so the scan cannot tell a local issue from a cross-repo one. A bare "+
				"`#N` on a public body resolves against THIS repo for every reader.", surface, len(m))}
	}
	var out []string
	seen := map[string]bool{}
	for _, g := range m {
		n, err := strconv.Atoi(g[2])
		if err != nil || n <= hint || seen[g[2]] {
			continue
		}
		seen[g[2]] = true
		out = append(out, fmt.Sprintf(
			"%s: bare `#%d` is above #%d, a number known to exist on this repo — it probably names "+
				"an issue in ANOTHER repository, where a public reader will silently resolve it "+
				"against this one. Qualify it as `owner/repo#N`, or drop it.", surface, n, hint))
	}
	return out
}

// privateRepoNames returns the roster's PRIVATE repositories as a lowercased set of
// `owner/name` slugs, and the lowercased set of their short labels (basename, plus any
// configured ASSAY_REPO_ALIASES short name).
//
// self — the repo being written to — is excluded from both: a body on a repo may of course
// name that repo, and a deployment that runs a desk on an internal repo would otherwise
// refuse every body it writes.
//
// A repo whose configured visibility is UNKNOWN is NOT included. That is the one place this
// file does not fail closed, and it is deliberate: unknown is the DEFAULT for any repo the
// roster does not state a visibility for, so treating unknown as private would refuse on
// every repo name in the census the moment a deployment left one field blank. The
// fail-closed direction is already carried by SelfContainApplies, which puts unknown-
// visibility repos IN scope as targets; the cost of missing one as a SUBJECT is a
// disclosure the leak sweep and review still see, while the cost of the other direction is
// the write-budget starvation #209/#1255 record.
func privateRepoNames(self string) (slugs map[string]bool, shorts map[string]bool) {
	slugs, shorts = map[string]bool{}, map[string]bool{}
	selfLower := strings.ToLower(self)
	_, selfName := splitRepo(self)
	for repo, pol := range EffectiveConfig().Repos {
		if pol.Visibility != VisibilityPrivate {
			continue
		}
		if strings.EqualFold(repo, selfLower) {
			continue
		}
		slugs[strings.ToLower(repo)] = true
		if _, name := splitRepo(repo); name != "" && !strings.EqualFold(name, selfName) {
			shorts[strings.ToLower(name)] = true
			if lbl := RepoShortLabel(repo); lbl != "" && !strings.EqualFold(lbl, selfName) {
				shorts[strings.ToLower(lbl)] = true
			}
		}
	}
	return slugs, shorts
}

// containsToken reports whether lowered carries tok as a whole token — bounded on both
// sides by something other than a letter, digit, `-` or `_`. A substring match would fire on
// any word that happens to contain a short name.
func containsToken(lowered, tok string) bool {
	if tok == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(lowered[i:], tok)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(tok)
		if !isTokenByte(lowered, start-1) && !isTokenByte(lowered, end) {
			return true
		}
		i = start + 1
		if i >= len(lowered) {
			return false
		}
	}
}

func isTokenByte(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return c == '-' || c == '_' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
