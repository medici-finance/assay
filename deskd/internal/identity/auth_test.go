package identity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ── the bind gate ───────────────────────────────────────────────────────────

// TestRefusedStartOffLoopback is P3's core rule at the unit level: a
// non-loopback bind with token validation off is refused, and the refusal names
// the flag that would fix it. The binary-level twin lives in cmd/deskd.
func TestRefusedStartOffLoopback(t *testing.T) {
	offLoopback := []string{"0.0.0.0:8787", ":8787", "10.0.0.4:8787", "[::]:8787", "deskd.internal:8787"}
	for _, addr := range offLoopback {
		if err := CheckBind(addr, AuthLoopback, false); err == nil {
			t.Errorf("CheckBind(%q, loopback) allowed the bind — an unauthenticated surface must never reach a network address", addr)
		} else if !strings.Contains(err.Error(), "--auth") {
			t.Errorf("CheckBind(%q) refused without naming --auth: %v", addr, err)
		}
	}
	// A hostname that is not an IP literal cannot be PROVEN loopback without
	// resolving it, and resolution is attacker-influenceable — so it is refused.
	if err := CheckBind("deskd.internal:8787", AuthBearer, true); err != nil {
		t.Errorf("a non-loopback bind WITH bearer validation was refused: %v", err)
	}
}

// TestLoopbackBindNeedsNoValidator is the paired positive control: the default
// laptop shape is untouched, so the gate cannot be satisfied by refusing
// everything.
func TestLoopbackBindNeedsNoValidator(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8787", "localhost:8787", "[::1]:8787"} {
		if err := CheckBind(addr, AuthLoopback, false); err != nil {
			t.Errorf("CheckBind(%q, loopback) refused the loopback default: %v", addr, err)
		}
	}
}

// TestBearerModeNeedsTheViewerApp: asking for bearer validation without the
// viewer App credentials is a refused start, not a mode that accepts every
// token because it has nothing to check against.
func TestBearerModeNeedsTheViewerApp(t *testing.T) {
	err := CheckBind("127.0.0.1:8787", AuthBearer, false)
	if err == nil {
		t.Fatal("--auth=bearer with no viewer App configured was allowed — it would accept every bearer token as valid")
	}
	if !strings.Contains(err.Error(), ViewerClientIDEnv) {
		t.Fatalf("the refusal does not name the env var that would fix it: %v", err)
	}
}

// TestParseAuthModeRefusesUnknown: an unrecognised --auth value is an error, not
// a silent fallback to the permissive mode.
func TestParseAuthModeRefusesUnknown(t *testing.T) {
	if _, err := ParseAuthMode("none"); err == nil {
		t.Fatal("--auth=none parsed — an unknown mode must never fall back to loopback")
	}
	if m, err := ParseAuthMode(""); err != nil || m != AuthLoopback {
		t.Fatalf("the empty --auth value = %q, %v; want the loopback default", m, err)
	}
}

// ── the bearer validator ────────────────────────────────────────────────────

// appTokenServer stands in for GitHub's POST /applications/{id}/token. It
// answers 404 for a token this App did not mint (the foreign-App case) and 200
// with the grant otherwise.
func appTokenServer(t *testing.T, grants map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/applications/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			AccessToken string `json:"access_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		grant, ok := grants[body.AccessToken]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(grant)
	}))
}

func grant(login string, scopes ...string) map[string]any {
	return map[string]any{"scopes": scopes, "user": map[string]any{"login": login}}
}

// TestValidTokenResolvesItsCaller is the positive control every refusal below
// leans on: a viewer-App token within the read ceiling resolves to its login.
func TestValidTokenResolvesItsCaller(t *testing.T) {
	srv := appTokenServer(t, map[string]any{"tok_ok": grant("some-viewer", "read:user")})
	defer srv.Close()
	v := NewValidator("client-id", "client-secret", srv.URL)
	caller, err := v.Validate(context.Background(), "tok_ok")
	if err != nil {
		t.Fatalf("a valid read token was refused: %v", err)
	}
	if caller.Login != "some-viewer" {
		t.Fatalf("caller login = %q, want some-viewer", caller.Login)
	}
}

// TestOverScopedTokenRefused: a token carrying more than read:user is refused
// outright rather than downgraded. The service must never hold, even for one
// request, a credential that can do more than it was presented for.
func TestOverScopedTokenRefused(t *testing.T) {
	srv := appTokenServer(t, map[string]any{
		"tok_repo":  grant("some-viewer", "read:user", "repo"),
		"tok_admin": grant("some-viewer", "admin:org"),
	})
	defer srv.Close()
	v := NewValidator("client-id", "client-secret", srv.URL)
	for _, tok := range []string{"tok_repo", "tok_admin"} {
		if _, err := v.Validate(context.Background(), tok); err == nil {
			t.Errorf("%s was accepted — its scopes exceed the %s ceiling", tok, ScopeCeiling)
		}
	}
}

// TestScopeSetIsTheCeiling pins the ruling on the authorization contract: the accepted
// scope set is exactly {read:user}, the stated ceiling. A token carrying only
// read:user (or no classic scope at all) resolves; a token carrying user:email
// or read:org, alone or beside read:user, is refused by the same over-scope
// rule as any other scope outside the ceiling — one error, ErrTokenRejected,
// naming the scope that broke it. Neither scope has a consumer in deskd.
//
// regression: #247
func TestScopeSetIsTheCeiling(t *testing.T) {
	if len(allowedScopes) != 1 || !allowedScopes[ScopeCeiling] {
		t.Errorf("allowedScopes = %v, want exactly {%s}", allowedScopes, ScopeCeiling)
	}

	srv := appTokenServer(t, map[string]any{
		"tok_read_user":  grant("some-viewer", "read:user"),
		"tok_none":       grant("some-viewer"),
		"tok_email":      grant("some-viewer", "read:user", "user:email"),
		"tok_org":        grant("some-viewer", "read:user", "read:org"),
		"tok_email_bare": grant("some-viewer", "user:email"),
		"tok_org_bare":   grant("some-viewer", "read:org"),
	})
	defer srv.Close()
	v := NewValidator("client-id", "client-secret", srv.URL)

	for _, tok := range []string{"tok_read_user", "tok_none"} {
		caller, err := v.Validate(context.Background(), tok)
		if err != nil {
			t.Errorf("%s was refused, but its scopes are within {%s}: %v", tok, ScopeCeiling, err)
			continue
		}
		if caller.Login != "some-viewer" {
			t.Errorf("%s resolved to %q, want some-viewer", tok, caller.Login)
		}
	}

	refused := map[string]string{
		"tok_email": "user:email", "tok_email_bare": "user:email",
		"tok_org": "read:org", "tok_org_bare": "read:org",
	}
	for tok, scope := range refused {
		_, err := v.Validate(context.Background(), tok)
		if err == nil {
			t.Errorf("%s was accepted — %s exceeds the %s ceiling", tok, scope, ScopeCeiling)
			continue
		}
		if !errors.Is(err, ErrTokenRejected) {
			t.Errorf("%s: refusal is not ErrTokenRejected: %v", tok, err)
		}
		if !strings.Contains(err.Error(), strconv.Quote(scope)) {
			t.Errorf("%s: refusal does not name the over-scope %q: %v", tok, scope, err)
		}
	}
}

// TestCallerHoldsNoEmailOrOrg: a validated caller is a login and a scope
// string, nothing more. Even when the grant GitHub returns carries an email
// and org data, none of it reaches the Caller, so nothing downstream can read
// a viewer's email or org membership from a bearer.
//
// regression: #247
func TestCallerHoldsNoEmailOrOrg(t *testing.T) {
	want := []string{"Login", "Scope"}
	rt := reflect.TypeOf(Caller{})
	var got []string
	for i := 0; i < rt.NumField(); i++ {
		got = append(got, rt.Field(i).Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Caller fields = %v, want %v — a new field may carry viewer data the read ceiling does not cover", got, want)
	}

	srv := appTokenServer(t, map[string]any{"tok_rich": map[string]any{
		"scopes": []string{"read:user"},
		"user": map[string]any{
			"login": "some-viewer", "email": "viewer@example.invalid",
			"organizations_url": "https://api.example.invalid/users/some-viewer/orgs",
		},
	}})
	defer srv.Close()
	v := NewValidator("client-id", "client-secret", srv.URL)
	caller, err := v.Validate(context.Background(), "tok_rich")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	raw, _ := json.Marshal(caller)
	for _, leak := range []string{"viewer@example.invalid", "orgs"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("the validated caller carries grant data %q: %s", leak, raw)
		}
	}
}

// TestForeignAppTokenRefused: a token the viewer App did not mint (GitHub
// answers 404) is refused. Without this the service would accept any valid
// GitHub token from anywhere as a read identity.
func TestForeignAppTokenRefused(t *testing.T) {
	srv := appTokenServer(t, map[string]any{"tok_ours": grant("some-viewer", "read:user")})
	defer srv.Close()
	v := NewValidator("client-id", "client-secret", srv.URL)
	if _, err := v.Validate(context.Background(), "tok_from_another_app"); err == nil {
		t.Fatal("a token the viewer App does not recognise was accepted")
	}
	if _, err := v.Validate(context.Background(), ""); err == nil {
		t.Fatal("an empty bearer token was accepted")
	}
}

// TestUnreachableValidatorRefuses: could-not-check is a REFUSAL here, never a
// pass. A validator that cannot reach GitHub knows nothing about the token it
// was handed, and "we could not check, so we allowed it" is the failure mode
// the whole three-state discipline exists to prevent.
func TestUnreachableValidatorRefuses(t *testing.T) {
	srv := appTokenServer(t, map[string]any{})
	srv.Close() // nothing is listening
	v := NewValidator("client-id", "client-secret", srv.URL)
	if _, err := v.Validate(context.Background(), "tok_anything"); err == nil {
		t.Fatal("a token was accepted while the validator could not reach the viewer App")
	}
}

// TestValidatorCachesTheVerdictNotTheToken: the cache holds a hash key and a
// resolved login, and it expires. Nothing in it is a credential.
func TestValidatorCachesTheVerdictNotTheToken(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(grant("some-viewer", "read:user"))
	}))
	defer srv.Close()

	now := time.Now().UTC()
	v := NewValidator("client-id", "client-secret", srv.URL)
	v.Now = func() time.Time { return now }

	for i := 0; i < 4; i++ {
		if _, err := v.Validate(context.Background(), "tok_cached"); err != nil {
			t.Fatalf("validate %d: %v", i, err)
		}
	}
	if calls != 1 {
		t.Fatalf("%d validation calls for 4 reads of one token, want 1 (the verdict is cached)", calls)
	}

	// The TOKEN is not a key anyone can read back: the cache is keyed by hash.
	v.mu.Lock()
	for key := range v.cache {
		if strings.Contains(key, "tok_cached") {
			v.mu.Unlock()
			t.Fatalf("the cache key contains the token value: %q", key)
		}
	}
	v.mu.Unlock()

	// Past the TTL the verdict is re-checked, so a revoked token stops working.
	now = now.Add(validationTTL + time.Second)
	if _, err := v.Validate(context.Background(), "tok_cached"); err != nil {
		t.Fatalf("post-TTL validate: %v", err)
	}
	if calls != 2 {
		t.Fatalf("%d validation calls after the TTL elapsed, want 2 (the verdict expires)", calls)
	}
}

// TestCallerCarriesNoToken: the validated identity is a login and a scope
// string. There is no field on it a later commit could author with.
func TestCallerCarriesNoToken(t *testing.T) {
	srv := appTokenServer(t, map[string]any{"tok_secret_value": grant("some-viewer", "read:user")})
	defer srv.Close()
	v := NewValidator("client-id", "client-secret", srv.URL)
	caller, err := v.Validate(context.Background(), "tok_secret_value")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	raw, err := json.Marshal(caller)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "tok_secret_value") {
		t.Fatalf("the validated caller carries the token: %s", raw)
	}
}

// TestRemoteIsLoopbackReadsTheConnection: the machine boundary is read from the
// connection, never from a header a caller could set. X-Forwarded-For is the
// classic way this check is quietly defeated.
func TestRemoteIsLoopbackReadsTheConnection(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.RemoteAddr = "198.51.100.7:5555"
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	if RemoteIsLoopback(r) {
		t.Fatal("a header made an off-loopback request look local")
	}
	r.RemoteAddr = "127.0.0.1:5555"
	if !RemoteIsLoopback(r) {
		t.Fatal("a genuine loopback request was not recognised")
	}
}

// ── the guard-ordering change (/mcp joins the protected table) ──────────────

type gatedPolicy struct{}

func (gatedPolicy) AnswerAsForRepo(owner, name string) string    { return "operator" }
func (gatedPolicy) AnonymousReadAllowed(owner, name string) bool { return false }
func (gatedPolicy) SurfaceLoginGated() bool                      { return true }

// TestRouteOwnedAuthenticationAdmitsAnAgentCaller: an App caller has no viewer
// OAuth session and must not need one, so a surface route may declare its own
// authentication. This is the ONLY reason the field exists — before it, /mcp
// was mounted outside the table to dodge the session requirement, and outside
// the table is where the route-boundary review found it authenticating nothing.
func TestRouteOwnedAuthenticationAdmitsAnAgentCaller(t *testing.T) {
	served := false
	g := &Guard{Policy: gatedPolicy{}}
	route := Route{
		Pattern: "/mcp", Min: Commenter, ItemScoped: false,
		Handler:       http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true }),
		Authenticated: func(r *http.Request) bool { return r.Header.Get("X-Test-Authenticated") == "yes" },
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("X-Test-Authenticated", "yes")
	g.Handler(route).ServeHTTP(rec, req)
	if !served {
		t.Fatalf("an authenticated agent caller was refused on a login-gated surface: %d", rec.Code)
	}

	// The refusal side: no route-owned authentication, no session, no service.
	served = false
	rec = httptest.NewRecorder()
	g.Handler(route).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if served {
		t.Fatal("an unauthenticated caller reached the handler")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated caller got %d, want 401", rec.Code)
	}
}

// TestRouteOwnedAuthenticationIsRefusedBesideAnonymousRead: "authenticated by
// another gate" and "reachable with no gate" are opposites, and a route
// claiming both does not validate.
func TestRouteOwnedAuthenticationIsRefusedBesideAnonymousRead(t *testing.T) {
	err := ValidateRoutes([]Route{{
		Pattern: "/mcp", Min: Commenter, AllowAnonymousRead: true,
		Handler:       http.NotFoundHandler(),
		Authenticated: func(*http.Request) bool { return true },
	}})
	if err == nil {
		t.Fatal("a route claiming both anonymous read and its own authentication validated")
	}
}
