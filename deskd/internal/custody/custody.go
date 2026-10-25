// Package custody holds a human credential for one request. MCP and its
// transitive imports cannot reach this package; the architecture tests assert
// that separation with both negative and positive controls.
//
// The human supplies a credential with the named action. Credential has no
// exported fields, setters, serialization, filesystem access, or process-wide
// store. It travels only through a request context, never into persistent
// service state. The caller supplies a server-validated login; FromToken checks
// it against the explicit startup principal before constructing custody.
package custody

import (
	"context"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"strings"
)

// Actor returns the explicitly configured human principal. Empty disables
// credential construction; this policy holds no token and performs no I/O.
func Actor() string { return humanid.Login() }

// MinimumScope is the least OAuth scope a supplied credential needs so a operator
// write can comment on, react to, and close an escalation issue across the
// cell's private repos. It is `repo` because GitHub's classic OAuth flows have
// no finer grain for issue writes on private repos; a public-only cell may
// narrow it to `public_repo`. Under model (b) the scope is the CLIENT's to
// obtain — the service never mints, refreshes, or widens a human credential —
// so this constant documents the requirement the client's session must meet.
const MinimumScope = "repo"

// ErrNoCredential is returned when no human credential rode in with the
// request. It is a first-class state, never an empty string masquerading as a
// token: a write attempted with no credential must fail loudly, so the answer
// path returns could-not-write rather than silently authoring as nobody.
var ErrNoCredential = fmt.Errorf("no human credential accompanied this request — under custody model (b) the person supplies the credential with the named action; the service holds none (#1819)")

// Credential is a human credential supplied WITH ONE REQUEST and held only for
// its lifetime.
//
// Every field is unexported and there is no setter, no MarshalJSON, and no
// String() that could carry the token: the value can be passed and read through
// the methods below and nothing else. It satisfies the answer.Credential
// interface, which is the import edge that makes the credential reachable from
// the write path and (asserted separately, internal/mcp) unreachable from the
// agent surface.
type Credential struct {
	token string
	login string
}

// FromToken builds a per-request credential from a token the caller supplied
// and the login it authenticates as (resolved server-side from the caller's
// live session or a validated bearer — never from a request field the caller
// could name).
//
// A blank token is [ErrNoCredential]; a login other than [Actor] is refused
// here rather than at write time, because a credential that would author under
// the wrong name must never reach the writer at all.
func FromToken(token, login string) (*Credential, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrNoCredential
	}
	if !humanid.Matches(login) {
		return nil, fmt.Errorf("the supplied credential authenticates as %q, not %q — refusing to author under the wrong identity", login, Actor())
	}
	return &Credential{token: token, login: Actor()}, nil
}

// Token returns the supplied token, or [ErrNoCredential] on the zero value. It
// is the method the write path calls immediately before authoring; the caller
// never caches it, so the credential's lifetime is the request's.
func (c *Credential) Token() (string, error) {
	if c == nil || c.token == "" {
		return "", ErrNoCredential
	}
	return c.token, nil
}

// TokenActor is the login this credential authors as. It is always [Actor] —
// [FromToken] refuses to construct anything else — and the method exists so the
// write path can assert it matches the identity it intends to stamp, closing
// the gap between "the token we hold" and "the human we claim to write as".
func (c *Credential) TokenActor() string {
	if c == nil {
		return ""
	}
	return c.login
}

// Present reports whether a credential rode in with this request.
func (c *Credential) Present() bool { return c != nil && c.token != "" }

// contextKey is unexported, so no package outside this one can put a value at
// this key or read it without going through [FromContext].
type contextKey struct{}

// NewContext returns a context carrying the per-request credential. A request
// context is the ONLY vessel this credential travels in: it is created by the
// handler that received the caller's token and it is cancelled and collected
// when that request returns, so the credential is in no struct that outlives
// the request — the persistence-audit property, expressed as a lifetime rather
// than a promise.
func NewContext(ctx context.Context, c *Credential) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext returns the credential carried by the request context, or
// [ErrNoCredential]. Fail-closed: an absent credential is an error, never a
// zero value that would author as nobody.
func FromContext(ctx context.Context) (*Credential, error) {
	if ctx == nil {
		return nil, ErrNoCredential
	}
	c, ok := ctx.Value(contextKey{}).(*Credential)
	if !ok || !c.Present() {
		return nil, ErrNoCredential
	}
	return c, nil
}

// Status is the non-secret view of the request's credential for a status
// surface. It NEVER includes the token itself — only whether one rode in with
// this request and which identity it authors as. There is no at-rest path to
// report any more, which is the point.
type Status struct {
	Present bool   `json:"present"`
	Actor   string `json:"actor"`
	// AtRest is always false and is serialised anyway: a reader of the status
	// surface should see the not-at-rest guarantee stated, not inferred from a
	// missing field.
	AtRest bool `json:"at_rest"`
	// MinimumScope is the scope the CLIENT's session must have obtained for the
	// supplied credential to be able to author the named actions.
	MinimumScope string `json:"minimum_scope"`
}

// StatusOf reports the non-secret state of a per-request credential.
func StatusOf(c *Credential) Status {
	return Status{Present: c.Present(), Actor: c.TokenActor(), AtRest: false, MinimumScope: MinimumScope}
}
