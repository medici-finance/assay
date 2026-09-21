package deskkit

// principal.go — the `On-behalf-of: human:<login>` composite-identity trailer, and the
// resolver every writing desk verb calls before it acts.
//
// WHAT THIS IS. Every desk role shares one GitHub App across every session that ever
// runs it — a review posted by the reviewer App says nothing about whose fleet drove
// it. This file declares the ONE resolver that answers "which human is this App action
// on behalf of", and the ONE trailer format every writer stamps with the answer:
// GitLab's composite-identity model ("service account on behalf of @human"), generalised
// to a fleet that has not yet grown a second human.
//
// THE SINGLE-POINT-OF-FAILURE, stated rather than taken silently (the brief's own
// framing): the session roster read. If a session could set its own principal from an
// environment variable, the trailer would be a self-report — exactly the failure mode
// `rosterconfig.go`'s file-only-for-ClassWrite rule already exists to close for the rest
// of the roster. So this resolver reuses that closed door rather than opening a new one:
// it reads EffectiveConfig() (which, for a ClassWrite tool — every write verb this stamps
// — is the config-home file ONLY, never the environment) and the session's own roster
// beacon file (deskroster's `<StateDir>/roster/<session>.json>`, also a file, never an
// env var). No code path in this file calls os.Getenv for the principal itself; the two
// session-identifying env vars it DOES read ($DESK_SESSION, $CLAUDE_SESSION_ID) name
// WHICH beacon file to open, they never supply the login.
//
// THREE STATES, and it is the load-bearing part, same shape as raisedby.go /
// modelstamp.go:
//
//	PrincipalUnresolved  the ZERO VALUE. No roster is configured (ASSAY_BLESS_LOGIN
//	                     unset or invalid in the config-home file) — there is no login
//	                     to stamp. Every write verb refuses (exit 5) on this state; it
//	                     is never a placeholder.
//	PrincipalAttended    resolved to the bless login, AND the calling session has a
//	                     live roster beacon (an interactive desk/worker session).
//	PrincipalUnattended  resolved to the SAME bless login, but no live session beacon
//	                     was found — an unattended run (a cron loop: scanloop,
//	                     verifyloop, …). The trailer carries `mode:unattended` so a
//	                     reader can tell the two apart without a second lookup.
//
// WHY BOTH RESOLVED STATES NAME THE SAME LOGIN TODAY. Per the 2026-09-12 driver ruling
// this brief relays: until a second human joins the
// roster, the principal resolves to the roster's SINGLE blessing authority regardless of
// who or what is driving the session. The session-beacon read exists so that when a
// second human's attach-time handshake lands (deferred to roster v2), ATTENDED sessions
// already have the plumbing to resolve to the session's own human rather than the bless
// login — this file's job today is only to tell attended and unattended runs apart, not
// yet to pick between two humans.
//
// VALIDATION AGAINST THE HUMAN MAP. Config.Humans already contains the bless login by
// construction — parseConfig adds it there the moment ASSAY_BLESS_LOGIN validates — so
// this resolver's own output can never itself name an unmapped login. The membership
// check below is defensive documentation of that invariant (and the same test a statusgen
// lint independently runs over trailers already landed in history, which THIS resolver
// cannot have written unmapped in the first place — see attribution.go).
//
// WRITERS. The six verbs (deskpost, deskreply, deskpr, deskevidence, deskfile, deskflip)
// call ResolvePrincipal (directly or through OnBehalfOfLine / AppendOnBehalfOf below)
// before their write, and refuse (exit 5) rather than write without a resolved principal.
// See docs/on-behalf-of.md for the full format and what the stamp does and does not
// prove.
//
// WHICH FORM OF THE HUMAN THE TRAILER NAMES depends on the TARGET REPO'S VISIBILITY, and
// that is why every entry point below takes a repo. A trailer stamped onto a write to a
// PUBLIC repo is world-readable forever — a posted review cannot be edited afterwards —
// so on a public target the trailer names the roster's NEUTRAL form of the human (the
// human-login map's KEY, the given name the map already carries as that human's public
// form) rather than the mapped forge login. On a known-PRIVATE target it names the login,
// exactly as before. The choice is made ONCE, here, so no verb can get it individually
// wrong; the repo argument is mandatory rather than optional for the same reason.
//
// The split FAILS CLOSED towards the neutral form: everything except a repo the roster
// states is `:private` takes the public form, matching VisibilityRiskClassed's shape (an
// unstated visibility must never read as "private"). An unknown repo, a repo admitted
// only by an `owner/*` pattern, and an empty repo argument therefore all get the neutral
// name — the direction whose failure mode is a less precise audit trail rather than a
// disclosure that cannot be withdrawn.

import (
	"encoding/json"
	"os"
	"strings"
	"unicode"
)

// PrincipalState is the three-state answer to "on behalf of which human is this write?".
type PrincipalState int

const (
	// PrincipalUnresolved is the ZERO VALUE: no principal could be resolved. Never a
	// placeholder — every write verb refuses rather than stamp this state.
	PrincipalUnresolved PrincipalState = iota
	// PrincipalAttended: resolved, and the calling session carries a live roster beacon.
	PrincipalAttended
	// PrincipalUnattended: resolved, but no live session beacon was found (a cron loop).
	PrincipalUnattended
)

func (s PrincipalState) String() string {
	switch s {
	case PrincipalAttended:
		return "attended"
	case PrincipalUnattended:
		return "unattended"
	default:
		return "unresolved"
	}
}

// Resolved reports whether s names an actual login. Mirrors RaisedByState.Answered() /
// ModelState's analogous accessor — a consumer writes `if !p.State.Resolved()` rather
// than comparing against one branch and forgetting the other.
func (s PrincipalState) Resolved() bool {
	return s == PrincipalAttended || s == PrincipalUnattended
}

// OnBehalfOfPrefix is the ONE spelling of the trailer key. Writers build lines through
// Principal.Line / OnBehalfOfLine and readers match this prefix, so no consumer needs to
// restate the string.
const OnBehalfOfPrefix = "On-behalf-of:"

// Principal is the resolved on-behalf-of identity for the current process, for ONE
// target repo.
//
// NOTHING MAY RENDER Login DIRECTLY. The form the trailer names is visibility-dependent
// (see the file comment), and Subject / Value / Line are the only places that decision is
// made. A caller that prints p.Login is the exact defect this type's Public field exists
// to prevent.
type Principal struct {
	// Login is the lowercased human login, non-empty only when State.Resolved(). It is
	// the form stamped on a known-PRIVATE target only.
	Login string
	// Name is the roster's NEUTRAL form of the same human — the human-login map's key
	// (ASSAY_HUMAN_LOGIN_MAP `name:login`). It is the form stamped on a PUBLIC target,
	// and is non-empty whenever Public is true (resolution refuses rather than resolve a
	// public-target principal with no neutral form to name).
	Name string
	// Public is true when the target repo is NOT known-private, i.e. when the trailer
	// must name Name rather than Login. Fails closed towards true — see the file comment.
	Public bool
	State  PrincipalState
}

// Subject is the form of the human this principal's trailer names on ITS target: the
// neutral name on a public target, the login on a known-private one. Empty when
// unresolved.
func (p Principal) Subject() string {
	if !p.State.Resolved() {
		return ""
	}
	if p.Public {
		return p.Name
	}
	return p.Login
}

// Value renders the trailer VALUE (everything after "On-behalf-of: "): `human:<subject>`,
// with ` mode:unattended` appended for an unattended run. Empty when unresolved.
func (p Principal) Value() string {
	subject := p.Subject()
	if subject == "" {
		return ""
	}
	v := "human:" + subject
	if p.State == PrincipalUnattended {
		v += " mode:unattended"
	}
	return v
}

// Line renders the full trailer line — `On-behalf-of: human:<login>[ mode:unattended]`
// — suitable as a git commit trailer, a final line on an issue/PR comment, or (with the
// prefix stripped by the caller) the `principal` field of a verify-outcomes.jsonl row.
// Empty when unresolved.
func (p Principal) Line() string {
	v := p.Value()
	if v == "" {
		return ""
	}
	return OnBehalfOfPrefix + " " + v
}

// beaconRole is the minimal shape this file reads from a session's roster beacon
// (<StateDir>/roster/<session>.json, the same file ackbeacon.go appends receipts to and
// deskroster stores open-work in). It intentionally reads only the one field this
// resolver needs and ignores every other key — see ackbeacon.go's file comment for why a
// partial read-only view here can never be the thing that drops another writer's field
// (this file never writes the beacon).
type beaconRole struct {
	Role string `json:"role,omitempty"`
}

// resolveSessionName returns the session name from the two identifying env vars, in the
// order deskroster's own resolveSession uses ($DESK_SESSION then $CLAUDE_SESSION_ID),
// falling back to the explicit session argument. It answers "" rather than an error when
// none is set: for THIS resolver, no identifiable session is a legitimate answer
// (PrincipalUnattended), never a refusal — unlike deskroster's own resolveSession, which
// refuses because ITS callers have no other identity to fall back to.
func resolveSessionName(session string) string {
	if s := strings.TrimSpace(os.Getenv("DESK_SESSION")); s != "" {
		return s
	}
	if s := strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID")); s != "" {
		return s
	}
	return strings.TrimSpace(session)
}

// sessionNameSafeRe is the character set a session name must stay within before this
// file will pass it to AckBeaconPath, which joins it directly into a filesystem path.
// On THIS path the name is attacker-influenced in a way AckBeaconPath's other callers'
// names are not: it comes straight from $DESK_SESSION / $CLAUDE_SESSION_ID, two
// environment variables any process sharing this one's environment controls, with no
// prior validation. A traversing value ("../../etc/passwd" and friends) would otherwise
// reach os.ReadFile via an ordinary path.Join. The impact is already bounded — the
// result feeds only a bool, and this process already has whatever filesystem access its
// own environment implies — but the identity path is the wrong place to accept an
// unvalidated path component, so the name is constrained to the shape deskroster itself
// ever writes (an ASCII session id: letters, digits, `-`, `_`, `.`) before it is used.
func sessionNameSafe(session string) bool {
	if session == "" {
		return false
	}
	for _, r := range session {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// sessionIsAttended reports whether the named session carries a live roster beacon with
// a non-empty role — the file deskroster writes for an interactive desk/worker session.
// A missing, empty, unparsable, or unsafely-named (sessionNameSafe) beacon reads as NOT
// attended (never an error): a cron loop with no beacon is exactly the unattended case
// this resolver exists to tag, and a malformed or suspicious name is no stronger a
// signal than a missing beacon for this purpose (the statusgen witness/lint paths are
// the ones that police malformed state, not this file).
func sessionIsAttended(session string) bool {
	if !sessionNameSafe(session) {
		return false
	}
	path, err := AckBeaconPath(session)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var b beaconRole
	if err := json.Unmarshal(data, &b); err != nil {
		return false
	}
	return strings.TrimSpace(b.Role) != ""
}

// publicFormRequired reports whether a trailer stamped onto a write to repo must name the
// human's NEUTRAL form rather than the roster login.
//
// Stated in the fail-closed direction on purpose, the same way VisibilityRiskClassed
// states its own rule: everything EXCEPT a repo the roster explicitly configures
// `:private` takes the public form. A repo the roster does not name, one admitted only by
// an `owner/*` pattern (patterns carry no policy — see RepoVisibility), one configured
// with no visibility token, and an empty repo argument all answer true. There is no input
// on which this can select the login form by accident, which is the property that matters:
// the failure mode of a wrong `true` is a coarser audit trail, the failure mode of a wrong
// `false` is a world-readable disclosure that cannot be withdrawn.
func publicFormRequired(repo string) bool {
	return RepoVisibility(strings.TrimSpace(repo)) != VisibilityPrivate
}

// HumanNeutralName returns the roster's NEUTRAL form of login — the key under which
// ASSAY_HUMAN_LOGIN_MAP's `name:login` entry maps to it — and whether one is configured.
//
// The map is stored name→login, so this is its reverse. When more than one name maps to
// the same login (a roster may legitimately carry an alias), the lexicographically first
// is returned, so the answer is deterministic rather than map-iteration-order dependent.
// Both the input and the answer are in the lowercased form the roster parser stores.
func HumanNeutralName(login string) (string, bool) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" {
		return "", false
	}
	best := ""
	for name, mapped := range EffectiveConfig().HumanLogins {
		if mapped != login {
			continue
		}
		if best == "" || name < best {
			best = name
		}
	}
	return best, best != ""
}

// ResolvePrincipal resolves the on-behalf-of principal for the current process, for the
// write about to be made to repo (owner/name form; "" when the caller genuinely has no
// repo, which resolves to the PUBLIC form — see publicFormRequired).
//
// ORDER: the session's roster beacon (deskroster's own read path, reused read-only —
// never this file's env vars alone) decides ATTENDED vs UNATTENDED; ASSAY_BLESS_LOGIN,
// read exclusively through EffectiveConfig() (config-home file for a ClassWrite tool,
// which is every caller of this function), supplies the login; the TARGET REPO'S
// CONFIGURED VISIBILITY decides whether the trailer names that login or the human's
// neutral name. An environment variable is NEVER consulted for the login itself — see the
// file comment.
//
// session, when non-empty, is used only if neither $DESK_SESSION nor $CLAUDE_SESSION_ID
// is set (mirrors deskroster's own resolution order); pass "" to use the ambient session
// only.
//
// Returns a *DeskError (Refused, exit 5) when no principal can be resolved — the roster
// is unconfigured, the bless login is somehow absent from the human map (defensive; see
// the file comment on why parseConfig makes this unreachable in practice), or the target
// is public and the roster carries no neutral name for the bless login. That last refusal
// is deliberate: the alternatives are stamping the login (the disclosure this split
// exists to prevent) or writing with no trailer at all (retiring the presence guarantee
// every downstream attribution check is built on), and neither is acceptable, so the
// write does not happen until the roster states the name.
func ResolvePrincipal(session, repo string) (Principal, error) {
	c := EffectiveConfig()
	login := strings.ToLower(strings.TrimSpace(c.Bless.Login))
	if !c.Configured() || login == "" {
		return Principal{}, Refused("on-behalf-of principal: no roster is configured (" +
			EnvBlessLogin + " unset, or the roster otherwise failed to load) at " + ConfigHomePath() +
			" — an environment variable is NEVER consulted for the principal, so exporting " +
			EnvBlessLogin + " (or anything else) in the shell has no effect here; configure the " +
			"roster's config-home file, or run as a session with no bless login and expect every " +
			"write verb to refuse rather than stamp an unresolvable principal")
	}
	if _, ok := c.Humans[login]; !ok {
		return Principal{}, Refused("on-behalf-of principal: the bless login " + login +
			" is not in the roster's human map — the principal must be a login the roster " +
			"recognises as human; fix the roster's " + EnvBlessLogin + " / " + EnvHumanLoginMap +
			" entries at " + ConfigHomePath())
	}
	public := publicFormRequired(repo)
	name, named := HumanNeutralName(login)
	if public && !named {
		where := strings.TrimSpace(repo)
		if where == "" {
			where = "an unnamed target"
		}
		return Principal{}, Refused("on-behalf-of principal: the write targets " + where +
			", which the roster does not state is private, so the trailer must name the blessing " +
			"authority's NEUTRAL form rather than its login — but " + EnvHumanLoginMap + " at " +
			ConfigHomePath() + " carries no name:login entry for that login, so there is no neutral " +
			"form to name. Add the entry (or, if this repo really is private, state it as " +
			"`owner/name:ci:private` in " + EnvAllowedRepos + "): stamping the login onto a " +
			"world-readable write cannot be undone, and writing with no trailer at all would retire " +
			"the guarantee every attribution check downstream depends on, so this verb refuses instead")
	}
	sess := resolveSessionName(session)
	state := PrincipalUnattended
	if sessionIsAttended(sess) {
		state = PrincipalAttended
	}
	return Principal{Login: login, Name: name, Public: public, State: state}, nil
}

// OnBehalfOfLine is the convenience form of ResolvePrincipal for a caller that only wants
// the rendered trailer line (or the refusal). session and repo are passed through
// unchanged.
func OnBehalfOfLine(session, repo string) (string, error) {
	p, err := ResolvePrincipal(session, repo)
	if err != nil {
		return "", err
	}
	return p.Line(), nil
}

// stripPlantedOnBehalfOfLines removes every line in s whose trimmed text starts with
// OnBehalfOfPrefix, wherever it sits — not just the exact trailing shape
// StripOnBehalfOfSuffix matches. It exists solely so AppendOnBehalfOf can guarantee the
// body it posts carries AT MOST ONE On-behalf-of line: the resolver's own genuine one.
//
// WHY THIS IS NEEDED (security-lane finding, multi-principal/01 review). Every write
// verb that appends this trailer takes a CALLER-supplied body (`--body-file`), and
// AppendOnBehalfOf previously appended unconditionally, never inspecting or rejecting a
// caller body that already contained an `On-behalf-of:` line. A caller could therefore
// plant an arbitrary `On-behalf-of: human:<anyone>` line, and the posted result carried
// TWO such lines — the planted one and the genuine appended one — with nothing marking
// which was authoritative. A first-match reader (as onBehalfOfPrincipalOf's witness-cell
// counterpart on the statusgen side used to be) would resolve to the planted one. This
// function removes the ambiguity at the source: whatever the caller wrote, at most one
// On-behalf-of line survives into the posted body, and it is always the one this
// resolver just computed.
func stripPlantedOnBehalfOfLines(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), OnBehalfOfPrefix) {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// AppendOnBehalfOf appends the on-behalf-of trailer to body as a new, blank-line-
// separated final line — the shape a git trailer block and a PR/issue comment's closing
// line share. Refuses (does not modify body) when the principal cannot be resolved, so a
// caller that ignores the error can never post a body silently missing its trailer.
//
// Any On-behalf-of line the caller-supplied body already contains — anywhere in it, not
// only at the end — is stripped first (stripPlantedOnBehalfOfLines), so the posted
// result never carries more than the one this call resolves. A caller cannot pre-seed or
// shadow the annotation the later author-never-flipper / segregation-of-duties checks
// are built to trust.
//
// repo names the target the body is about to be posted to, and decides which form of the
// human the appended trailer carries (see the file comment and publicFormRequired).
func AppendOnBehalfOf(body []byte, session, repo string) ([]byte, error) {
	line, err := OnBehalfOfLine(session, repo)
	if err != nil {
		return nil, err
	}
	s := strings.TrimRight(stripPlantedOnBehalfOfLines(string(body)), "\n")
	if s == "" {
		return []byte(line + "\n"), nil
	}
	return []byte(s + "\n\n" + line + "\n"), nil
}

// StripOnBehalfOfSuffix removes a TRAILING on-behalf-of trailer block — the exact shape
// AppendOnBehalfOf adds (a blank line, then the trailer line) — from body, returning the
// text as it stood before that trailer was appended. It is a no-op (returns body
// unchanged) when body does not end in that exact shape.
//
// WHY THIS EXISTS. A verb that compares a REPLACEMENT body against the CURRENT live
// body for idempotency (`deskpr edit`) would otherwise never see a noop: the live body
// carries a PRIOR call's trailer, the caller's replacement does not yet carry one, and
// the two would never compare equal even for a byte-for-byte-intended-identical edit.
// Stripping the trailer before the compare (both sides) restores the noop.
func StripOnBehalfOfSuffix(body string) string {
	trimmed := strings.TrimRight(body, "\n")
	idx := strings.LastIndex(trimmed, "\n\n"+OnBehalfOfPrefix+" ")
	if idx < 0 {
		return body
	}
	rest := trimmed[idx+len("\n\n"):]
	// Confirm the suffix really is ONE trailer line (no embedded blank line / further
	// content after it) before stripping — a body that legitimately contains this text
	// mid-way through, followed by more prose, must be left alone.
	if strings.Contains(rest, "\n") {
		return body
	}
	return trimmed[:idx]
}

// OnBehalfOfCommitSuffix renders the trailer as a commit-message suffix: a blank line
// then the trailer, with no trailing newline — the shape `message + this` wants when the
// caller already owns the final newline handling (deskevidence's WriteFileInput.Message).
// Refuses when the principal cannot be resolved. repo is the repo the commit lands in —
// its history is as world-readable as a comment body is, so the same visibility split
// applies.
func OnBehalfOfCommitSuffix(session, repo string) (string, error) {
	line, err := OnBehalfOfLine(session, repo)
	if err != nil {
		return "", err
	}
	return "\n\n" + line, nil
}
