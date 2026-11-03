package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/httpsecurity"
)

// Cookie names. All are HttpOnly + SameSite=Lax (the OAuth redirect is a
// top-level cross-site navigation back from github.com, which Strict would
// strip; Lax lets the callback read the state cookie while still blocking a
// silent cross-site POST). The session cookie itself is Strict — see [Auth].
const (
	// SessionCookie carries the authenticated session id. HttpOnly so page
	// script cannot read it; SameSite=Strict so no cross-site request rides it.
	SessionCookie = "deskd_user_session"
	// stateCookie carries the OAuth CSRF nonce between /auth/login and the
	// callback. Short-lived and single-use.
	stateCookie = "deskd_oauth_state"
	// returnCookie carries the path to send the viewer back to after login.
	returnCookie = "deskd_oauth_return"
)

// DefaultScope is the OAuth scope a viewer's token is requested with: `repo`,
// so a commenter can add an issue comment as themselves on the cell's private
// repos (GitHub's classic OAuth has no finer grain for private-repo issue
// writes). A public-only deployment may narrow it via [Config.Scope].
const DefaultScope = "repo"

// Config is the second OAuth client — DISTINCT from the operator device-flow
// client (custody). The viewer authenticates through THIS client; the answer
// path still authors with the operator credential.
type Config struct {
	// ClientID / ClientSecret are the DESKD_OAUTH_CLIENT_ID / _SECRET pair.
	ClientID     string
	ClientSecret string
	// RedirectURL is the absolute callback URL registered on the OAuth App,
	// e.g. http://127.0.0.1:8787/auth/callback.
	RedirectURL string
	// Scope defaults to [DefaultScope].
	Scope string
	// AuthBase is github.com's OAuth host (authorize + token). Overridable for
	// tests. Defaults to https://github.com.
	AuthBase string
	// APIBase is the REST host for GET /user. Defaults to https://api.github.com.
	APIBase string
}

// Configured reports whether per-user OAuth is set up. When false, deskd serves
// but no viewer can authenticate — a private-repo cell then shows only the login
// prompt (which itself explains the missing configuration), and never any data.
func (c Config) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

func (c Config) authBase() string {
	if c.AuthBase == "" {
		return "https://github.com"
	}
	return strings.TrimSuffix(c.AuthBase, "/")
}

func (c Config) apiBase() string {
	if c.APIBase == "" {
		return "https://api.github.com"
	}
	return strings.TrimSuffix(c.APIBase, "/")
}

func (c Config) scope() string {
	if c.Scope == "" {
		return DefaultScope
	}
	return c.Scope
}

// Auth drives the per-user OAuth authorization-code flow and owns the session
// store the rest of deskd validates against.
type Auth struct {
	cfg      Config
	Sessions *SessionStore
	http     *http.Client
	now      func() time.Time
	// Rollout is the service contract Task 9 gate. When no cell has opted in,
	// the sign-in routes are NOT SERVED — that is the positive content of an
	// OFF deployment (relayed direction, toolkit 1963, fork 2):
	// login-gated, with no way to sign in, so nobody rises above anonymous.
	// A nil Rollout is an unresolved gate and fails CLOSED — sign-in is not
	// served — so a wiring that forgets the gate loses the flow rather than
	// opening it.
	Rollout RolloutGate
}

// NewAuth builds the authenticator over a config and session store.
func NewAuth(cfg Config, sessions *SessionStore, now func() time.Time) *Auth {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if sessions == nil {
		sessions = NewSessionStore(now)
	}
	return &Auth{cfg: cfg, Sessions: sessions, http: &http.Client{Timeout: 30 * time.Second}, now: now}
}

// Handler registers the auth endpoints. They are the ONLY anonymous-reachable
// data-free routes on a private-repo cell: the login redirect, the callback,
// and logout. Everything with item data sits behind the [Middleware].
//
// Login and callback are additionally behind the rollout gate ([Auth.Rollout])
// — on a deployment where no cell has opted in they refuse rather than begin
// an OAuth flow. Logout is NOT gated: ending a session is always safe, and a
// viewer holding a session from before an opt-in was withdrawn must be able to
// drop it.
func (a *Auth) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/login", a.handleLogin)
	mux.HandleFunc("GET /auth/callback", a.handleCallback)
	mux.HandleFunc("POST /auth/logout", a.handleLogout)
	mux.HandleFunc("GET /auth/logout", a.handleLogout)
	return mux
}

// refuseWhenRolledOff writes the rollout refusal and reports whether it did.
// It is checked FIRST, before the OAuth-configuration check, so an OFF
// deployment never advertises which credentials it is missing: "not enabled
// here" is the whole answer.
func (a *Auth) refuseWhenRolledOff(w http.ResponseWriter) bool {
	if rolloutSignInServed(a.Rollout) {
		return false
	}
	writeAuthError(w, http.StatusServiceUnavailable,
		"per-user sign-in is not enabled on this deskd: no cell opts in (set per_user_identity: enabled on a cell that declares a dev environment — the service contract Task 9). This surface stays login-gated and serves no item data")
	return true
}

func (a *Auth) handleLogin(w http.ResponseWriter, r *http.Request) {
	if a.refuseWhenRolledOff(w) {
		return
	}
	if !a.cfg.Configured() {
		writeAuthError(w, http.StatusServiceUnavailable,
			"per-user sign-in is not configured on this deskd (set DESKD_OAUTH_CLIENT_ID / _SECRET / redirect URL)")
		return
	}
	nonce, err := randToken()
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "could not start sign-in")
		return
	}
	// Stash the CSRF nonce and the return path in HttpOnly cookies.
	a.setCookie(w, r, stateCookie, nonce, 10*time.Minute, http.SameSiteLaxMode)
	ret := sanitizeReturn(r.URL.Query().Get("return_to"))
	a.setCookie(w, r, returnCookie, ret, 10*time.Minute, http.SameSiteLaxMode)

	q := url.Values{}
	q.Set("client_id", a.cfg.ClientID)
	q.Set("redirect_uri", a.cfg.RedirectURL)
	q.Set("scope", a.cfg.scope())
	q.Set("state", nonce)
	http.Redirect(w, r, a.cfg.authBase()+"/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

func (a *Auth) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Gated independently of /auth/login, not merely because login is: a
	// callback is reachable directly, and a flow begun while a cell was opted
	// in must not mint a session after the opt-in was withdrawn.
	if a.refuseWhenRolledOff(w) {
		return
	}
	if !a.cfg.Configured() {
		writeAuthError(w, http.StatusServiceUnavailable, "per-user sign-in is not configured on this deskd")
		return
	}
	// CSRF: the state returned by GitHub must match the nonce we set. A missing
	// or mismatched state is a rejected callback, never a minted session.
	stateParam := r.URL.Query().Get("state")
	stateCk, err := r.Cookie(stateCookie)
	if err != nil || stateParam == "" || subtle.ConstantTimeCompare([]byte(stateParam), []byte(stateCk.Value)) != 1 {
		writeAuthError(w, http.StatusBadRequest, "sign-in state did not match — restart sign-in")
		return
	}
	a.clearCookie(w, r, stateCookie)

	code := r.URL.Query().Get("code")
	if code == "" {
		writeAuthError(w, http.StatusBadRequest, "sign-in returned no authorization code")
		return
	}
	token, err := a.exchange(r.Context(), code)
	if err != nil {
		writeAuthError(w, http.StatusBadGateway, "could not complete sign-in with GitHub")
		return
	}
	// Resolve the login SERVER-SIDE from the token — the only identity source.
	login, err := a.currentLogin(r.Context(), token)
	if err != nil || login == "" {
		writeAuthError(w, http.StatusBadGateway, "could not read your GitHub identity")
		return
	}
	sess, err := a.Sessions.Begin(login, token)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "could not establish a session")
		return
	}
	a.setCookieExpiry(w, r, SessionCookie, sess.ID, sess.ExpiresAt, http.SameSiteStrictMode)

	ret := "/inbox"
	if rc, err := r.Cookie(returnCookie); err == nil {
		if s := sanitizeReturn(rc.Value); s != "" {
			ret = s
		}
	}
	a.clearCookie(w, r, returnCookie)
	http.Redirect(w, r, ret, http.StatusFound)
}

func (a *Auth) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		a.Sessions.End(c.Value)
	}
	a.clearCookie(w, r, SessionCookie)
	http.Redirect(w, r, "/inbox", http.StatusFound)
}

// exchange trades the authorization code for a user access token.
func (a *Auth) exchange(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"client_id":     {a.cfg.ClientID},
		"client_secret": {a.cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {a.cfg.RedirectURL},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.authBase()+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token exchange HTTP %d", resp.StatusCode)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("token response did not decode: %w", err)
	}
	if tr.Error != "" || tr.AccessToken == "" {
		return "", fmt.Errorf("token exchange returned no access token")
	}
	return tr.AccessToken, nil
}

// currentLogin resolves the authenticated login from a token via GET /user. It
// is the SERVER-SIDE identity resolution the whole model rests on (Verify 5).
func (a *Auth) currentLogin(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.apiBase()+"/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /user HTTP %d", resp.StatusCode)
	}
	var u struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(raw, &u); err != nil {
		return "", err
	}
	return u.Login, nil
}

func (a *Auth) setCookie(w http.ResponseWriter, r *http.Request, name, value string, ttl time.Duration, ss http.SameSite) {
	c := httpsecurity.CookiesForURL(a.cfg.RedirectURL).New(r, name, value, ss)
	c.Expires = a.now().Add(ttl)
	c.MaxAge = int(ttl / time.Second)
	http.SetCookie(w, c)
}

func (a *Auth) setCookieExpiry(w http.ResponseWriter, r *http.Request, name, value string, exp time.Time, ss http.SameSite) {
	c := httpsecurity.CookiesForURL(a.cfg.RedirectURL).New(r, name, value, ss)
	c.Expires = exp
	http.SetCookie(w, c)
}

func (a *Auth) clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	ss := http.SameSiteLaxMode
	if name == SessionCookie {
		ss = http.SameSiteStrictMode
	}
	c := httpsecurity.CookiesForURL(a.cfg.RedirectURL).New(r, name, "", ss)
	c.MaxAge = -1
	http.SetCookie(w, c)
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// sanitizeReturn keeps only a same-origin absolute PATH, so the post-login
// redirect can never be steered to another site (open-redirect defence). A
// value that is not a rooted path with no scheme/host is dropped.
func sanitizeReturn(s string) string { return httpsecurity.ReturnPath(s) }

func randToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
