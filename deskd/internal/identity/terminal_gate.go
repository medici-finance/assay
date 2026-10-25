package identity

import (
	"errors"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"net/http"
	"strings"
)

// terminal_gate.go — layer (c) of the service contract's single point of failure:
// the per-request authority decision for the terminal named actions (`send`,
// `session`). It runs before the action touches the terminal registry, on
// signals the request cannot forge, and fails CLOSED.
//
// # Identity: the VIEWER, per request, never stored
//
// The terminal actions are not GitHub writes, so their identity is the viewer
// identity every /v1 read uses — the OAuth viewer session cookie on loopback,
// or an `Authorization: Bearer` token the VIEWER App validates ([Validator],
// read:user ceiling) off-loopback. The answerer's GitHub write credential is
// neither required nor accepted: an Authorization header that is not a bearer
// the viewer App validates is a 400, never "tried as a write token" (custody
// is not widened). Nothing here stores anything; the session store and the
// validator's hash-keyed verdict cache are the existing ones.
//
// # Tier: Answerer on the desk's cell
//
// A Commenter may watch a desk and never type into it. The Answerer tier on a
// cell is: the per-user rollout gate is ON for EVERY repo the cell declares
// (an OFF cell derives nobody above Anonymous — the same rule [Guard.derive]
// applies), AND the viewer's login equals the cell's answer_as. An unknown
// cell, an empty answer_as, or an unresolvable repo all fail closed.

// TerminalCell is what the gate needs to know about a cell.
type TerminalCell struct {
	Name     string
	Repos    []string
	AnswerAs string
}

// GateError is a refusal with the HTTP status it maps to. The serve layer
// reads HTTPStatus without importing this package.
type GateError struct {
	Status int
	Reason string
}

func (e *GateError) Error() string   { return e.Reason }
func (e *GateError) HTTPStatus() int { return e.Status }

// The gate's refusals.
var (
	ErrTerminalNoIdentity = &GateError{http.StatusUnauthorized,
		"no viewer identity accompanied this request — sign in (session cookie) or present a viewer bearer"}
	ErrTerminalUnvalidatedBearer = &GateError{http.StatusBadRequest,
		"the Authorization header is not a bearer the viewer App validates — the terminal actions take the VIEWER identity only; a GitHub write credential is never accepted as identity (custody is not widened)"}
	ErrTerminalConflictingIdentity = &GateError{http.StatusBadRequest,
		"the session cookie and the bearer name different viewers — one request carries one identity"}
	ErrTerminalNotAnswerer = &GateError{http.StatusForbidden,
		"this action needs \"answerer\" on the desk's cell — a commenter may watch a desk, never type into it"}
)

// TerminalGate decides who is asking and whether they are the answerer on a
// cell. It is safe for concurrent use (its parts are).
type TerminalGate struct {
	// Sessions is the OAuth viewer session store (cookie [SessionCookie]).
	Sessions *SessionStore
	// Validator validates a viewer bearer. nil means no viewer App is
	// configured, and then ANY Authorization header is refused (400).
	Validator *Validator
	// Rollout is the per-cell per-user identity gate; nil fails closed (OFF).
	Rollout RolloutGate
	// Cell resolves a cell by name from the loaded config.
	Cell func(name string) (TerminalCell, bool)
}

// Principal resolves the request's viewer login, or a [GateError].
func (g *TerminalGate) Principal(r *http.Request) (string, error) {
	if g == nil || r == nil {
		return "", ErrTerminalNoIdentity
	}
	var bearerLogin string
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		tok := BearerFrom(r)
		if tok == "" || g.Validator == nil {
			return "", ErrTerminalUnvalidatedBearer
		}
		caller, err := g.Validator.Validate(r.Context(), tok)
		if err != nil || caller.Login == "" {
			return "", ErrTerminalUnvalidatedBearer
		}
		bearerLogin = caller.Login
	}
	var cookieLogin string
	if c, err := r.Cookie(SessionCookie); err == nil && g.Sessions != nil {
		sess, verr := g.Sessions.Validate(c.Value)
		if verr != nil {
			// A presented-but-dead session is not silently replaced by the
			// bearer: the caller is told to sign in again.
			return "", &GateError{http.StatusUnauthorized, verr.Error()}
		}
		cookieLogin = sess.Login
	}
	switch {
	case bearerLogin != "" && cookieLogin != "" && !strings.EqualFold(bearerLogin, cookieLogin):
		return "", ErrTerminalConflictingIdentity
	case bearerLogin != "":
		return bearerLogin, nil
	case cookieLogin != "":
		return cookieLogin, nil
	}
	return "", ErrTerminalNoIdentity
}

// Authorize resolves the viewer and requires the Answerer tier on cell. It
// returns the login on success. Identity is decided first (401/400), the tier
// second (403), so an unauthenticated caller learns nothing about cells.
func (g *TerminalGate) Authorize(r *http.Request, cell string) (string, error) {
	login, err := g.Principal(r)
	if err != nil {
		return "", err
	}
	if !g.answererOn(login, cell) {
		return "", ErrTerminalNotAnswerer
	}
	return login, nil
}

// answererOn is the tier decision: rollout ON for every repo of the cell, and
// login == the cell's answer_as. Fail-closed on every unknown.
func (g *TerminalGate) answererOn(login, cell string) bool {
	if !humanid.Enabled() || login == "" || cell == "" || g.Cell == nil {
		return false
	}
	c, ok := g.Cell(cell)
	if !ok || c.AnswerAs == "" || len(c.Repos) == 0 {
		return false
	}
	for _, slug := range c.Repos {
		owner, name, ok := strings.Cut(slug, "/")
		if !ok || !rolloutEnabledFor(g.Rollout, owner, name) {
			return false
		}
	}
	return strings.EqualFold(login, c.AnswerAs)
}

// IsGateError reports whether err is a [GateError], and returns it.
func IsGateError(err error) (*GateError, bool) {
	var ge *GateError
	if errors.As(err, &ge) {
		return ge, true
	}
	return nil, false
}
