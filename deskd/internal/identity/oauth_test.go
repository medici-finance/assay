package identity

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeGitHub stands in for github.com's OAuth token endpoint and the REST /user
// endpoint. It lets the callback flow run with no network.
func fakeGitHub(t *testing.T, login string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"user-token-123"}`))
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		// The identity is whatever GET /user returns for the token — the server
		// is the sole source of the login (Verify 5).
		if r.Header.Get("Authorization") != "Bearer user-token-123" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"` + login + `"}`))
	})
	return httptest.NewServer(mux)
}

func testAuth(t *testing.T, gh *httptest.Server) *Auth {
	cfg := Config{
		ClientID:     "cid",
		ClientSecret: "csecret",
		RedirectURL:  "http://127.0.0.1:8787/auth/callback",
		AuthBase:     gh.URL,
		APIBase:      gh.URL,
	}
	a := NewAuth(cfg, NewSessionStore(nil), nil)
	// The OAuth flow tests are about the FLOW; the rollout gate that decides
	// whether the flow is served at all is exercised in rollout_test.go.
	a.Rollout = allOn{}
	return a
}

// TestCallbackServerDerivesLogin (Verify 5): a completed OAuth flow mints a
// session whose login is the one GET /user returned — never a client-supplied
// value. The callback ignores everything but the code + the state nonce.
func TestCallbackServerDerivesLogin(t *testing.T) {
	gh := fakeGitHub(t, "octocat")
	defer gh.Close()
	a := testAuth(t, gh)

	// Drive /auth/login to obtain the state cookie.
	loginRec := httptest.NewRecorder()
	a.Handler().ServeHTTP(loginRec, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	var stateCk *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == stateCookie {
			stateCk = c
		}
	}
	if stateCk == nil || stateCk.Value == "" {
		t.Fatal("no state cookie set by /auth/login")
	}

	// Callback with a matching state and a code. A body field claiming a
	// different login would be ignored — the callback reads only code + state.
	cbReq := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state="+stateCk.Value+"&login=mallory", nil)
	cbReq.AddCookie(stateCk)
	cbRec := httptest.NewRecorder()
	a.Handler().ServeHTTP(cbRec, cbReq)

	if cbRec.Code != http.StatusFound {
		t.Fatalf("callback got %d, want a redirect: %s", cbRec.Code, cbRec.Body.String())
	}
	var sessCk *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == SessionCookie {
			sessCk = c
		}
	}
	if sessCk == nil {
		t.Fatal("no session cookie minted")
	}
	sess, err := a.Sessions.Validate(sessCk.Value)
	if err != nil {
		t.Fatalf("minted session invalid: %v", err)
	}
	if sess.Login != "octocat" {
		t.Fatalf("session login %q — must be the login GET /user returned, not a client-supplied value", sess.Login)
	}
	if sess.Token() != "user-token-123" {
		t.Fatalf("session did not carry the user token")
	}
}

// TestCallbackRejectsStateMismatch: a callback whose state does not match the
// cookie is a rejected CSRF, not a minted session.
func TestCallbackRejectsStateMismatch(t *testing.T) {
	gh := fakeGitHub(t, "octocat")
	defer gh.Close()
	a := testAuth(t, gh)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=forged", nil)
	req.AddCookie(&http.Cookie{Name: stateCookie, Value: "real-nonce"})
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("state mismatch got %d, want 400", rr.Code)
	}
	// No session cookie should be set.
	for _, c := range rr.Result().Cookies() {
		if c.Name == SessionCookie && c.Value != "" {
			t.Fatal("a session was minted despite a state mismatch")
		}
	}
}

// TestLoginRefusedWhenUnconfigured: with no OAuth client, sign-in is 503.
func TestLoginRefusedWhenUnconfigured(t *testing.T) {
	a := NewAuth(Config{}, NewSessionStore(nil), nil)
	a.Rollout = allOn{}
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured login got %d, want 503", rr.Code)
	}
}

func TestSanitizeReturn(t *testing.T) {
	for in, want := range map[string]string{
		"/inbox":              "/inbox",
		"/item/x?a=1":         "/item/x?a=1",
		"//evil.com":          "",
		"https://evil.com":    "",
		"http://x":            "",
		"":                    "",
		"relative":            "",
		`/\example.invalid`:   "",
		"/%5cexample.invalid": "",
		"/%2fexample.invalid": "",
		"/\t/example.invalid": "",
		"/":                   "/",
	} {
		if got := sanitizeReturn(in); got != want {
			t.Fatalf("sanitizeReturn(%q)=%q want %q", in, got, want)
		}
	}
}

// The callback must validate the cookie again, even if login initially validated
// return_to. A modified return cookie must never produce an external redirect.
func TestCallbackRejectsExternalReturnCookie(t *testing.T) {
	gh := fakeGitHub(t, "octocat")
	defer gh.Close()
	a := testAuth(t, gh)
	for _, target := range []string{`/\example.invalid/path`, `//example.invalid/path`, `/%5cexample.invalid/path`, `/%2fexample.invalid/path`} {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=nonce", nil)
			req.AddCookie(&http.Cookie{Name: stateCookie, Value: "nonce"})
			req.Header.Add("Cookie", returnCookie+"="+target)
			rr := httptest.NewRecorder()
			a.Handler().ServeHTTP(rr, req)
			if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/inbox" {
				t.Fatalf("callback status=%d Location=%q", rr.Code, rr.Header().Get("Location"))
			}
		})
	}
}

func TestOAuthCookieAttributes(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprint("local=", local), func(t *testing.T) {
			gh := fakeGitHub(t, "octocat")
			defer gh.Close()
			a := testAuth(t, gh)
			browserURL := "https://console.example.invalid"
			requestURL := "http://console.example.invalid"
			if local {
				browserURL = "http://127.0.0.1:8787"
				requestURL = browserURL
			}
			a.cfg.RedirectURL = browserURL + "/auth/callback"
			request := func(path string) *http.Request {
				r := httptest.NewRequest(http.MethodGet, requestURL+path, nil)
				r.RemoteAddr = "127.0.0.1:5000"
				return r
			}
			check := func(rr *httptest.ResponseRecorder, want map[string]http.SameSite, deleted bool) {
				t.Helper()
				cookies := rr.Result().Cookies()
				if len(cookies) != len(want) {
					t.Fatalf("cookies=%d want %d", len(cookies), len(want))
				}
				for _, c := range cookies {
					ss, ok := want[c.Name]
					if !ok || c.Secure == local || !c.HttpOnly || c.SameSite != ss || c.Path != "/" || c.Domain != "" {
						t.Fatalf("unexpected cookie attributes: %+v", c)
					}
					if deleted && (c.Value != "" || c.MaxAge != -1) {
						t.Fatalf("cookie was not expired: %+v", c)
					}
				}
			}
			login := httptest.NewRecorder()
			a.Handler().ServeHTTP(login, request("/auth/login?return_to=%2Finbox"))
			check(login, map[string]http.SameSite{stateCookie: http.SameSiteLaxMode, returnCookie: http.SameSiteLaxMode}, false)
			var nonce string
			for _, c := range login.Result().Cookies() {
				if c.Name == stateCookie {
					nonce = c.Value
				}
			}
			req := request("/auth/callback?code=abc&state=" + nonce)
			for _, c := range login.Result().Cookies() {
				req.AddCookie(c)
			}
			callback := httptest.NewRecorder()
			a.Handler().ServeHTTP(callback, req)
			check(callback, map[string]http.SameSite{stateCookie: http.SameSiteLaxMode, returnCookie: http.SameSiteLaxMode, SessionCookie: http.SameSiteStrictMode}, false)
			var session *http.Cookie
			for _, c := range callback.Result().Cookies() {
				if c.Name == SessionCookie {
					session = c
				}
			}
			req = request("/auth/logout")
			req.AddCookie(session)
			logout := httptest.NewRecorder()
			a.Handler().ServeHTTP(logout, req)
			check(logout, map[string]http.SameSite{SessionCookie: http.SameSiteStrictMode}, true)
			if _, err := a.Sessions.Validate(session.Value); err == nil {
				t.Fatal("logout left session valid")
			}
		})
	}
}

func FuzzSanitizeReturnSameOrigin(f *testing.F) {
	for _, s := range []string{"/inbox", `/\example.invalid`, "//example.invalid", "/%5cexample.invalid", "/%2fexample.invalid", "/\t/example.invalid"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := sanitizeReturn(s)
		if got == "" {
			return
		}
		u, err := url.Parse(strings.ReplaceAll(got, `\`, "/"))
		if err != nil || u.Host != "" || u.Scheme != "" || !strings.HasPrefix(got, "/") {
			t.Fatalf("unsafe return accepted: %q", got)
		}
	})
}
