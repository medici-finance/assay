// server.go — the loopback HTTP server (design.md §2, §3): Screen 0 (/tier), Screen 1
// (/setup), Screen 2 (/run), and the manifest-flow callback (/callback). Binds 127.0.0.1
// ONLY — bind_test.go asserts 0.0.0.0 and :: never appear on the listener.
//
// Every state change is one console line AND one audit line, both carrying `app=`/`state=`
// and NEVER key material (secrets_test.go). The state nonce (records.go's newStateNonce) is
// the control that keeps a callback from being accepted for a row it was not issued for
// (callback_test.go: TestCallbackBadState). The genuinely independent second layer is the
// owner check on the conversion result: the App's real owner as GitHub reports it must match
// the operator's gh login (personal) or --org (org-owned), which trips on a different signal
// than the nonce and catches a valid state carrying a FOREIGN App's code (pem_test.go:
// TestPemNeverWrittenOnMismatch / TestPemNeverWrittenOnOrgOwnerMismatch). The loopback bind is
// a precondition, not an independent layer — GET /run serves each pending row's live nonce to
// any local process, so "reached the listener" and "knows the nonce" are one capability, not
// two.
package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

//go:embed page/*.html page/*.css
var pageFS embed.FS

var pageCSS = mustReadCSS()

func mustReadCSS() string {
	b, err := pageFS.ReadFile("page/style.css")
	if err != nil {
		panic(err) // embedded at build time — cannot fail at runtime
	}
	return string(b)
}

var pageTemplates = template.Must(template.ParseFS(pageFS, "page/*.html"))

// throttleTimeout is the design.md §4 "posted → paused" window: no callback within this
// long is GitHub's creation throttle, not an error. A var so tests can shrink it.
var throttleTimeout = 10 * time.Minute

// deskappsServer serves the loopback page and drives the per-App state machine. All state
// mutation goes through its mutex so the HTTP handlers and the throttle watcher never race
// on the same StateFile.
type deskappsServer struct {
	mu        sync.Mutex
	port      int
	tier      string
	prefix    string
	org       string
	ownerKind string // "org" or "me"
	specs     []AppSpec
	state     *StateFile
	identity  ghUser
	mismatch  bool

	// out is a test hook for the console writer; nil means os.Stdout.
	out io.Writer
}

func newServer(port int, tier, prefix, org, ownerKind string, specs []AppSpec, state *StateFile) *deskappsServer {
	return &deskappsServer{port: port, tier: tier, prefix: prefix, org: org, ownerKind: ownerKind, specs: specs, state: state}
}

// console prints one line to s.out (os.Stdout in production).
func (s *deskappsServer) console(format string, args ...any) {
	w := s.out
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

// logState records ONE state change, on the console AND the audit log, in the same call —
// design.md facts: "every state change is one line on stdout"; brief 02's Ground rules:
// tests assert stdout and the audit line carry app=/state=, never key material.
func (s *deskappsServer) logState(app string, state AppRowState, detail string) {
	line := fmt.Sprintf("app=%s state=%s", app, state)
	if detail != "" {
		line += " · " + detail
	}
	s.console("%s → %s", app, line)
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "deskapps",
		Verb:       "state",
		ArgsDigest: deskkit.ArgsDigest([]string{app, string(state)}),
		Result:     deskkit.ResultOK,
		Detail:     line,
	})
}

func (s *deskappsServer) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleTier)
	mux.HandleFunc("GET /tier", s.handleTier)
	mux.HandleFunc("GET /setup", s.handleSetup)
	mux.HandleFunc("GET /run", s.handleRun)
	mux.HandleFunc("GET /callback", s.handleCallback)
	mux.HandleFunc("POST /mark-posted", s.handleMarkPosted)
	return mux
}

// --- Screen 0 — /tier ------------------------------------------------------------------

func (s *deskappsServer) handleTier(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	data := struct {
		CSS       template.CSS
		Tier      string
		Org       string
		OwnerKind string
		Prefix    string
		Specs     []AppSpec
	}{template.CSS(pageCSS), s.tier, s.org, s.ownerKind, s.prefix, s.specs}
	s.mu.Unlock()
	s.render(w, "tier.html", data)
}

// --- Screen 1 — /setup ------------------------------------------------------------------

func (s *deskappsServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	data := struct {
		CSS       template.CSS
		Identity  ghUser
		OwnerKind string
		Org       string
		Tier      string
		Prefix    string
		Specs     []AppSpec
		Mismatch  bool
	}{template.CSS(pageCSS), s.identity, s.ownerKind, s.org, s.tier, s.prefix, s.specs, s.mismatch}
	s.mu.Unlock()
	s.render(w, "setup.html", data)
}

// --- Screen 2 — /run --------------------------------------------------------------------

type runRow struct {
	App          string
	State        string
	ManifestJSON string
	NewAppURL    string
	StateNonce   string
}

func (s *deskappsServer) handleRun(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	rows := make([]runRow, 0, len(s.state.Apps))
	anyPaused := false
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", s.port)
	for i := range s.state.Apps {
		row := &s.state.Apps[i]
		if row.State == StatePaused {
			anyPaused = true
		}
		spec := specFor(s.specs, row.App)
		manifestJSON, _ := BuildManifestJSON(spec, redirectURL)
		rows = append(rows, runRow{
			App:          row.App,
			State:        string(row.State),
			ManifestJSON: string(manifestJSON),
			NewAppURL:    newAppURL(s.ownerKind, s.org),
			StateNonce:   row.StateNonce,
		})
	}
	s.mu.Unlock()

	data := struct {
		CSS       template.CSS
		Rows      []runRow
		AnyPaused bool
	}{template.CSS(pageCSS), rows, anyPaused}
	s.render(w, "run.html", data)
}

// --- /mark-posted -----------------------------------------------------------------------
//
// The Create button navigates the browser to GitHub in a new tab; this fetch (fired
// onsubmit, before that navigation) is how the run board learns a form was actually sent,
// so it can flip pending → posted and start this row's throttle window. It is scoped by the
// SAME state-nonce match as the callback: a request naming a foreign state changes nothing.
func (s *deskappsServer) handleMarkPosted(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	state := r.URL.Query().Get("state")

	s.mu.Lock()
	row := s.state.rowByNonce(state)
	if row == nil || row.App != app || row.State != StatePending {
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	row.State = StatePosted
	row.UpdatedAt = time.Now().UTC()
	_ = saveState(s.state)
	s.mu.Unlock()

	s.logState(app, StatePosted, "manifest form sent")
	w.WriteHeader(http.StatusNoContent)
}

// --- /callback ----------------------------------------------------------------------------

func (s *deskappsServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	s.mu.Lock()
	row := s.state.rowByNonce(state)
	s.mu.Unlock()

	// THE single control (brief 02's single-point-of-failure line): a callback whose state
	// does not match a pending row's nonce is refused before anything else happens — no
	// conversion attempted, no row touched. This is the check mutations.json's first mutant
	// removes, and callback_test.go's TestCallbackBadState is the guard that catches it.
	if row == nil {
		http.Error(w, "unknown or foreign state — refusing", http.StatusForbidden)
		return
	}
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// State-machine guard (S-2): a callback is only meaningful for a row still awaiting its
	// conversion — pending (armed) or posted (Create clicked, throttle running). A row already
	// keyed (or paused/further along) must never be re-converted: without this, a replayed
	// /callback?code=<fresh>&state=<same nonce> would re-run writePEM/writeAppRecords/
	// writeBindings and overwrite the stored key. handleMarkPosted already guards its own
	// transition on row.State; this closes that asymmetry. (A keyed row also has its nonce
	// consumed below, so rowByNonce would not even find it — this catches the paused/other
	// states whose nonce is still live.)
	s.mu.Lock()
	notConvertible := row.State != StatePending && row.State != StatePosted
	s.mu.Unlock()
	if notConvertible {
		http.Error(w, "app already keyed — refusing to re-convert", http.StatusConflict)
		return
	}

	cr, err := convertCodeFn(code)
	if err != nil {
		if err == errConversionExpired {
			s.mu.Lock()
			row.State = StatePosted
			row.UpdatedAt = time.Now().UTC()
			_ = saveState(s.state)
			s.mu.Unlock()
			s.logState(row.App, StatePosted, "conversion code expired — Create again")
			http.Redirect(w, r, "/run", http.StatusFound)
			return
		}
		http.Error(w, "conversion failed", http.StatusBadGateway)
		return
	}

	// Owner mismatch (design.md §8, S-1): the conversion's owner.login must match the owner the
	// operator actually named — their gh login on the personal path, the --org value on the org
	// path. This is a genuinely independent layer from the state nonce: it trips on the App's
	// real owner as GitHub reports it in the conversion response (a different signal, in a
	// different component, than the local state record the nonce matches), and it catches
	// exactly the fault the nonce cannot — a callback carrying a valid state and a FOREIGN App's
	// code. Without the org branch the default (org-owned) path had no owner check at all, so a
	// foreign App's PEM/webhook-secret could be written into the operator's credential plane and
	// the role bindings repointed at an App they do not own. Nothing is written on a mismatch;
	// the row is re-armed to pending.
	var wantOwner string
	switch s.ownerKind {
	case "me":
		wantOwner = s.identity.Login
	case "org":
		wantOwner = s.org
	}
	if wantOwner != "" && cr.Owner.Login != "" && !strings.EqualFold(cr.Owner.Login, wantOwner) {
		s.mu.Lock()
		row.State = StatePending
		row.UpdatedAt = time.Now().UTC()
		s.mismatch = true
		_ = saveState(s.state)
		s.mu.Unlock()
		s.logState(row.App, StatePending, "owner mismatch — nothing written")
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}

	if err := writePEM(row.App, cr.PEM); err != nil {
		http.Error(w, "writing key failed", http.StatusInternalServerError)
		return
	}
	if err := writeAppRecords(row.App, fmt.Sprintf("%d", cr.ID), cr.ClientID, cr.WebhookSecret); err != nil {
		http.Error(w, "writing records failed", http.StatusInternalServerError)
		return
	}
	spec := specFor(s.specs, row.App)
	if err := writeBindings(spec); err != nil {
		http.Error(w, "writing bindings failed", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	row.State = StateKeyed
	row.AppID = fmt.Sprintf("%d", cr.ID)
	row.ClientID = cr.ClientID
	// Consume the nonce (S-2): once a row is keyed its state nonce is spent, so a replayed
	// /callback carrying it finds no row (rowByNonce ignores an empty nonce) and cannot
	// re-run the write path. rowByApp still finds the row for status/resume by name.
	row.StateNonce = ""
	row.UpdatedAt = time.Now().UTC()
	saveErr := saveState(s.state)
	s.mu.Unlock()
	if saveErr != nil {
		http.Error(w, "writing state failed", http.StatusInternalServerError)
		return
	}

	s.logState(row.App, StateKeyed, "PEM written 0600")
	http.Redirect(w, r, "/run", http.StatusFound)
}

// render executes tmpl with data, buffering nothing extra: html/template auto-escapes every
// field, so a value that somehow reached this call (it never should — secrets never enter
// these data structs) would still not render as raw HTML, but the real guarantee is
// structural: PEM/client-secret/webhook-secret fields simply do not exist on runRow,
// setup's template data, or tier's.
func (s *deskappsServer) render(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplates.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// --- throttle watcher ---------------------------------------------------------------------

// watchThrottle periodically flips a "posted" row whose UpdatedAt is older than
// throttleTimeout to "paused" — design.md §4: "posted → paused | no callback within 10
// minutes (the throttle)". It stops when stop is closed.
func (s *deskappsServer) watchThrottle(stop <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.pauseStaleRows()
		}
	}
}

func (s *deskappsServer) pauseStaleRows() {
	now := time.Now().UTC()
	var toLog []string
	s.mu.Lock()
	for i := range s.state.Apps {
		row := &s.state.Apps[i]
		if row.State == StatePosted && now.Sub(row.UpdatedAt) >= throttleTimeout {
			row.State = StatePaused
			row.UpdatedAt = now
			toLog = append(toLog, row.App)
		}
	}
	if len(toLog) > 0 {
		_ = saveState(s.state)
	}
	s.mu.Unlock()
	for _, app := range toLog {
		s.logState(app, StatePaused, "no callback within 10 minutes — GitHub's throttle, not an error")
	}
}

// --- loopback bind ---------------------------------------------------------------------

// listenLoopback binds 127.0.0.1:port; on failure (port in use) it takes the next free
// loopback port (design.md §8: "Port in use"). It NEVER binds 0.0.0.0 or :: — bind_test.go
// asserts this directly on the returned listener's address.
func listenLoopback(port int) (net.Listener, int, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		return ln, ln.Addr().(*net.TCPAddr).Port, nil
	}
	ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	return ln, ln.Addr().(*net.TCPAddr).Port, nil
}
