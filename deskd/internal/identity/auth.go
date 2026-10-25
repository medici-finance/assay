// auth.go — the BIND GATE and the viewer-App bearer validator
// (the service contract increment 2, property P3).
//
// # What was wrong
//
// The custody review of service origin/main d218068 found exactly
// one listen site and no guard in front of it: `--addr` defaulted to
// 127.0.0.1:8787 (cmd/deskd/main.go:93) and was handed straight to
// srv.ListenAndServe() (:376). `deskd --addr 0.0.0.0:8787` bound, and nothing
// refused it. The capability middleware (middleware.go) fronts the data routes,
// but a middleware is a per-request answer to "may THIS viewer read that?" — it
// is not an answer to "should this process be on the network at all?", and
// /mcp was mounted outside it entirely.
//
// # The rule, stated once
//
// A NON-LOOPBACK bind requires token validation to be configured. That is the
// whole of P3, and it is enforced BEFORE any listener exists ([CheckBind]), so
// the failure mode is a refused start naming the flag that would fix it rather
// than an open port nobody meant to open. The default is [AuthLoopback]: the
// laptop dogfood shape is untouched.
//
// # Why the ceiling is read:user
//
// The bearer token a caller presents identifies WHO is reading; it is never the
// credential anything is written with (that is custody model (b): the human
// credential rides with the named action and is dropped when the request
// returns). A read identity therefore needs no write scope, and a token that
// carries one is refused rather than downgraded — accepting an over-scoped
// token would mean the service holds, for the length of a request, a
// credential that could do more than it was presented for.
package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AuthMode is how the service authenticates a caller.
type AuthMode string

const (
	// AuthLoopback trusts the machine boundary: only a caller on the loopback
	// interface is authenticated, and only a loopback --addr may be bound. It is
	// the default and the laptop dogfood shape.
	AuthLoopback AuthMode = "loopback"
	// AuthBearer validates the /mcp caller's Authorization: Bearer token
	// against the viewer OAuth App. It does NOT gate /v1 or the un-versioned
	// data routes: those stay gated by the viewer session (the OAuth cookie)
	// through the capability guard (contract doc §10.2). It is REQUIRED before
	// a non-loopback --addr may be bound.
	AuthBearer AuthMode = "bearer"
)

// AuthModes lists the accepted --auth values for the flag's usage string and
// the parse error.
var AuthModes = []AuthMode{AuthLoopback, AuthBearer}

// ParseAuthMode accepts an --auth flag value. An unknown value is an error, not
// a silent fallback to the permissive mode.
func ParseAuthMode(s string) (AuthMode, error) {
	switch AuthMode(strings.ToLower(strings.TrimSpace(s))) {
	case "", AuthLoopback:
		return AuthLoopback, nil
	case AuthBearer:
		return AuthBearer, nil
	default:
		return "", fmt.Errorf("--auth=%q is not one of %v", s, AuthModes)
	}
}

// IsLoopbackAddr reports whether a listen address binds only the loopback
// interface. An empty host (":8787") or a wildcard ("0.0.0.0", "::") binds every
// interface and is NOT loopback. A hostname that is not an IP literal cannot be
// proven loopback without resolving it, and an unprovable bind is treated as
// non-loopback — fail closed.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		host = strings.TrimSpace(addr)
	}
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// CheckBind is the refused-start rule (P3). It returns a non-nil error — which
// the caller turns into exit 1 — when the service would put an
// UNAUTHENTICATED surface on the network, or when bearer mode is asked for
// without the viewer App credentials that make validation possible.
//
// validatorConfigured is whether the viewer-App client id and secret are set.
func CheckBind(addr string, mode AuthMode, validatorConfigured bool) error {
	if mode == AuthBearer && !validatorConfigured {
		return fmt.Errorf("--auth=bearer needs the viewer OAuth App credentials: set %s and %s. "+
			"Refusing to start rather than accept every bearer token as valid", ViewerClientIDEnv, ViewerClientSecretEnv)
	}
	if IsLoopbackAddr(addr) {
		return nil
	}
	if mode != AuthBearer {
		return fmt.Errorf("refusing to bind the non-loopback address %q with --auth=%s: a network-reachable surface must validate caller tokens. "+
			"Bind a loopback address (the default 127.0.0.1:8787), or set --auth=bearer with %s and %s configured",
			addr, mode, ViewerClientIDEnv, ViewerClientSecretEnv)
	}
	return nil
}

// The viewer OAuth App credentials the bearer validator authenticates its
// validation call with. They are an APP credential (like the read-App key),
// never a human one: validating "is this token one of ours, and what may it
// read?" is the service's own question to GitHub.
const (
	ViewerClientIDEnv     = "DESKD_VIEWER_CLIENT_ID"
	ViewerClientSecretEnv = "DESKD_VIEWER_CLIENT_SECRET"
)

// ScopeCeiling is the most a READ bearer token may carry. A token with any
// broader scope is refused: a read identity needs no write authority, and the
// service must never hold one it was not presented for.
const ScopeCeiling = "read:user"

// allowedScopes is the set a validated read token's scopes must be a subset of.
// An empty scope string (a fine-grained token, or a classic token with none) is
// allowed — it can do less, not more.
//
// # Derivation, per scope (the authorization contract)
//
// Where deskd spends a validated bearer: the token goes to exactly ONE GitHub
// call, the validation POST /applications/{client_id}/token in [Validator.check].
// That call authenticates with the viewer App's own client id and secret, so the
// token's scopes do not decide whether it succeeds. From the grant, deskd reads
// only user.login, in mcpCallerAuth (cmd/deskd/main.go) and
// [TerminalGate.Principal]. [Caller.Scope] is filled and never read. The validator
// keeps no token, so nothing calls GitHub with it afterwards. A viewer's tier comes
// from deskd's own read identity, never from this token ([PermissionQuerier]).
//
//   - read:user: the stated ceiling ([ScopeCeiling]), the scope a read identity
//     is defined to carry. No call site consumes it either. It is admitted as
//     the ceiling itself, not because a call needs it.
//   - user:email: NO CONSUMER, so NOT ADMITTED. No /user/emails call exists,
//     and no email is read from a viewer bearer.
//   - read:org: NO CONSUMER, so NOT ADMITTED. No org-membership call is made
//     with a viewer bearer. Org and repo standing come from deskd's own read
//     identity.
//
// The set was narrowed to the ceiling alone by ruling on the authorization contract; a
// token carrying user:email or read:org is now refused like any other
// over-scoped token. The set is spelled from [ScopeCeiling] so the two cannot
// drift apart. Widening it is an identity/auth change, and that is
// human-gated.
var allowedScopes = map[string]bool{ScopeCeiling: true}

// validationTTL bounds how long a validation result is reused. Short enough
// that a revoked token stops working promptly, long enough that a page of reads
// does not make one API call per request.
const validationTTL = 120 * time.Second

// Caller is a validated read identity: who is asking, and nothing else. It
// carries NO token — the validator keeps none — so nothing downstream can
// author with a caller's read credential.
type Caller struct {
	Login string
	Scope string
}

// Validator validates a read bearer token against the viewer OAuth App
// (POST /applications/{client_id}/token) and caches the VERDICT — never the
// token — for [validationTTL], keyed by SHA-256 of the token.
//
// # What is cached, precisely
//
// The map key is a hash, so the token value itself is in no long-lived
// structure; the value is the resolved login and scope, which are not secrets.
// A cache of tokens would be a credential store by another name, and this
// service holds none.
type Validator struct {
	// ClientID / clientSecret are the viewer App's own credentials.
	ClientID     string
	clientSecret string
	// APIBase is the REST base (injectable for tests).
	APIBase string
	// HTTP is the client used for the validation call.
	HTTP *http.Client
	// Now is injectable so a test can age the cache.
	Now func() time.Time

	mu    sync.Mutex
	cache map[string]cachedVerdict
}

type cachedVerdict struct {
	caller Caller
	err    error
	at     time.Time
}

// NewValidator builds the bearer validator. A blank client id or secret returns
// nil: the caller ([CheckBind]) refuses to start in bearer mode rather than
// running with a validator that would accept everything.
func NewValidator(clientID, clientSecret, apiBase string) *Validator {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return nil
	}
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &Validator{
		ClientID:     clientID,
		clientSecret: clientSecret,
		APIBase:      strings.TrimSuffix(apiBase, "/"),
		HTTP:         &http.Client{Timeout: 15 * time.Second},
		Now:          func() time.Time { return time.Now().UTC() },
		cache:        map[string]cachedVerdict{},
	}
}

// ErrTokenRejected is the single verdict a caller sees for every refusal
// reason. The specific reason is wrapped for the service's own logs; a caller
// learns only that the token was not accepted, so the surface is not an oracle
// for "is this token real but over-scoped" versus "is this token ours".
var ErrTokenRejected = fmt.Errorf("bearer token rejected")

// Validate resolves a bearer token to its [Caller], or an error. Every refusal
// path — a token the viewer App does not recognise (GitHub answers 404), a
// token whose scopes exceed [ScopeCeiling], a validation call that could not be
// made — returns an error, never a permissive default.
func (v *Validator) Validate(ctx context.Context, token string) (Caller, error) {
	if v == nil {
		return Caller{}, fmt.Errorf("%w: no viewer App is configured to validate against", ErrTokenRejected)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Caller{}, fmt.Errorf("%w: no token presented", ErrTokenRejected)
	}
	key := hashToken(token)
	if c, ok := v.lookup(key); ok {
		return c.caller, c.err
	}
	caller, err := v.check(ctx, token)
	v.store(key, cachedVerdict{caller: caller, err: err, at: v.now()})
	return caller, err
}

func (v *Validator) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now().UTC()
}

func (v *Validator) lookup(key string) (cachedVerdict, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.cache[key]
	if !ok {
		return cachedVerdict{}, false
	}
	if v.now().Sub(c.at) > validationTTL {
		delete(v.cache, key)
		return cachedVerdict{}, false
	}
	return c, true
}

func (v *Validator) store(key string, c cachedVerdict) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cache == nil {
		v.cache = map[string]cachedVerdict{}
	}
	v.cache[key] = c
}

// check performs the validation call: POST /applications/{client_id}/token with
// the App's own basic auth and the token in the body. GitHub answers 200 with
// the grant (user + scopes) for a token this App minted, and 404 for anything
// else — including a valid token belonging to a DIFFERENT App, which is exactly
// the foreign-App case that must be refused.
func (v *Validator) check(ctx context.Context, token string) (Caller, error) {
	endpoint := v.APIBase + "/applications/" + url.PathEscape(v.ClientID) + "/token"
	payload, err := json.Marshal(map[string]string{"access_token": token})
	if err != nil {
		return Caller{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return Caller{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(v.ClientID, v.clientSecret)

	client := v.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		// Could-not-check is a REFUSAL here, never a pass: a validator that
		// cannot reach GitHub knows nothing about the token it was handed.
		return Caller{}, fmt.Errorf("%w: the viewer App could not be reached to validate it (%v)", ErrTokenRejected, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Caller{}, fmt.Errorf("%w: the viewer App does not recognise this token (it was minted by another App, or revoked)", ErrTokenRejected)
	}
	if resp.StatusCode != http.StatusOK {
		return Caller{}, fmt.Errorf("%w: validation returned HTTP %d", ErrTokenRejected, resp.StatusCode)
	}
	var grant struct {
		Scopes []string `json:"scopes"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&grant); err != nil {
		return Caller{}, fmt.Errorf("%w: the validation response did not decode (%v)", ErrTokenRejected, err)
	}
	if over := overScoped(grant.Scopes); over != "" {
		return Caller{}, fmt.Errorf("%w: its scopes exceed the %s ceiling (%q) — a read identity is never a write credential", ErrTokenRejected, ScopeCeiling, over)
	}
	if grant.User.Login == "" {
		return Caller{}, fmt.Errorf("%w: the grant names no user", ErrTokenRejected)
	}
	return Caller{Login: grant.User.Login, Scope: strings.Join(grant.Scopes, ",")}, nil
}

// overScoped returns the first scope outside the read ceiling, or "".
func overScoped(scopes []string) string {
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !allowedScopes[s] {
			return s
		}
	}
	return ""
}

// hashToken is the cache key: the token's SHA-256, so the cache holds no token
// value.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// BearerFrom extracts the token from an Authorization: Bearer header.
func BearerFrom(r *http.Request) string {
	if r == nil {
		return ""
	}
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// RemoteIsLoopback reports whether a request arrived over the loopback
// interface. It is the machine-boundary authentication [AuthLoopback] relies
// on, and it is read from the connection's remote address — never from a
// header a caller could set.
func RemoteIsLoopback(r *http.Request) bool {
	if r == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}
