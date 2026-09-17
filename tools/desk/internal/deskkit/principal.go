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
// WHY BOTH RESOLVED STATES NAME THE SAME LOGIN TODAY. Per the ruling this brief relays
// (medici-finance/assay-toolkit#2454, 2026-09-12, C3=C): until a second human joins the
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

import (
	"encoding/json"
	"os"
	"strings"
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

// Principal is the resolved on-behalf-of identity for the current process.
type Principal struct {
	// Login is the lowercased human login, non-empty only when State.Resolved().
	Login string
	State PrincipalState
}

// Value renders the trailer VALUE (everything after "On-behalf-of: "): `human:<login>`,
// with ` mode:unattended` appended for an unattended run. Empty when unresolved.
func (p Principal) Value() string {
	if !p.State.Resolved() {
		return ""
	}
	v := "human:" + p.Login
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

// sessionIsAttended reports whether the named session carries a live roster beacon with
// a non-empty role — the file deskroster writes for an interactive desk/worker session.
// A missing, empty, or unparsable beacon reads as NOT attended (never an error): a cron
// loop with no beacon is exactly the unattended case this resolver exists to tag, and a
// malformed beacon is no stronger a signal than a missing one for this purpose (the
// statusgen witness/lint paths are the ones that police malformed state, not this file).
func sessionIsAttended(session string) bool {
	if session == "" {
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

// ResolvePrincipal resolves the on-behalf-of principal for the current process.
//
// ORDER: the session's roster beacon (deskroster's own read path, reused read-only —
// never this file's env vars alone) decides ATTENDED vs UNATTENDED; ASSAY_BLESS_LOGIN,
// read exclusively through EffectiveConfig() (config-home file for a ClassWrite tool,
// which is every caller of this function), supplies the login. An environment variable
// is NEVER consulted for the login itself — see the file comment.
//
// session, when non-empty, is used only if neither $DESK_SESSION nor $CLAUDE_SESSION_ID
// is set (mirrors deskroster's own resolution order); pass "" to use the ambient session
// only.
//
// Returns a *DeskError (Refused, exit 5) when no principal can be resolved — the roster
// is unconfigured, or the bless login is somehow absent from the human map (defensive;
// see the file comment on why parseConfig makes this unreachable in practice).
func ResolvePrincipal(session string) (Principal, error) {
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
	sess := resolveSessionName(session)
	state := PrincipalUnattended
	if sessionIsAttended(sess) {
		state = PrincipalAttended
	}
	return Principal{Login: login, State: state}, nil
}

// OnBehalfOfLine is the convenience form of ResolvePrincipal for a caller that only wants
// the rendered trailer line (or the refusal). session is passed through unchanged.
func OnBehalfOfLine(session string) (string, error) {
	p, err := ResolvePrincipal(session)
	if err != nil {
		return "", err
	}
	return p.Line(), nil
}

// AppendOnBehalfOf appends the on-behalf-of trailer to body as a new, blank-line-
// separated final line — the shape a git trailer block and a PR/issue comment's closing
// line share. Refuses (does not modify body) when the principal cannot be resolved, so a
// caller that ignores the error can never post a body silently missing its trailer.
func AppendOnBehalfOf(body []byte, session string) ([]byte, error) {
	line, err := OnBehalfOfLine(session)
	if err != nil {
		return nil, err
	}
	s := strings.TrimRight(string(body), "\n")
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
// Refuses when the principal cannot be resolved.
func OnBehalfOfCommitSuffix(session string) (string, error) {
	line, err := OnBehalfOfLine(session)
	if err != nil {
		return "", err
	}
	return "\n\n" + line, nil
}
