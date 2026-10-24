// v1_terminal_actions.go — the terminal named actions on the `/v1` seam
// (the service contract): `send` (POST /v1/desks/{id}/terminal/send) and
// `session` (POST /v1/sessions, DELETE /v1/sessions/{id}).
//
// These are the most powerful writes the product carries — a person's words
// into a live agent session — so every request passes, in order:
//
//  0. origin: a browser request from any origin but deskd's own is refused
//     before identity is read ([refuseCrossSite], review pr192-S1) — the
//     viewer cookie proves WHO the viewer is, never that the viewer meant
//     this request. A query string is then refused ([refuseQuery],
//     the service contract's task-workflow amendment): no terminal write takes
//     one, so a workflow id or control verb cannot ride beside a send;
//  1. identity: the per-request VIEWER identity (session cookie on loopback,
//     a viewer-App bearer off-loopback). No identity is 401; an Authorization
//     header the viewer App does not validate is 400 — the answerer's GitHub
//     write credential is never accepted as identity ([TerminalAuthority]);
//  2. tier: Answerer on the desk's cell, decided by the identity package
//     BEFORE the registry is touched (layer c) — a Commenter gets 403;
//  3. shape: a strict body (unknown fields — a `container`, a `pod`, a
//     `command` — are 400; the client never names a target). One exception
//     to the 2-before-3 order: on POST /v1/sessions the cell comes from the
//     body's `on`, so the strict decode runs after identity and BEFORE tier,
//     and a Commenter's malformed open gets 400, not 403 (review pr192-N6).
//     Nothing is reached either way: the registry is touched only after tier;
//  4. the registry's own refusals (layer d): the desk must be in the caller's
//     cell, writable, at the epoch the input was composed against; a close is
//     refused on a configured desk; an open is refused (503) while deskd's
//     attach lifecycle is off or not running, for a pod desk, and for a kind
//     the host's terminal.CapabilityFor does not report supported, each before
//     any driver is built (the service contract's combining merge, obligations
//     (vi) and (vii)).
//
// Nothing here is a GitHub write and nothing here holds a credential: this
// file imports neither internal/answer nor internal/custody (the authority
// test pins the import set), so it cannot author a review, ratify a gate, mint
// a role or reach a forge-write proxy. Text a desk prints is scrollback and is
// never read as approval.
package serve

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// TerminalAuthority is the per-request authority decision the terminal
// actions consult (deskd wires identity.TerminalGate). Principal resolves the
// viewer; Authorize resolves the viewer AND requires the Answerer tier on the
// cell. Errors carrying an HTTPStatus() are mapped to that status; any other
// error is a 403 (fail closed).
type TerminalAuthority interface {
	Principal(r *http.Request) (login string, err error)
	Authorize(r *http.Request, cell string) (login string, err error)
}

// TerminalActions is the wiring the terminal named actions need. nil (or a
// partial wiring) leaves the routes UNMOUNTED — a 404, never a route that
// exists and cannot work.
type TerminalActions struct {
	Registry  *terminal.Registry
	Authority TerminalAuthority
	// Driver resolves the driver for a custom session's target kind (read from
	// cells.yaml by the registry). nil = [terminal.RegisteredDriver].
	Driver func(kind schema.TargetKind) (terminal.Target, error)
}

func (t *TerminalActions) ready() bool {
	return t != nil && t.Registry != nil && t.Authority != nil
}

// maxSessionBody bounds a session-open body; maxSendBody a send body.
const (
	maxSendBody    = 2*terminal.MaxSendBytes + 4096
	maxSessionBody = 2*terminal.MaxSendBytes + 4096
	maxTitleBytes  = 200
)

// mountTerminalActions registers the terminal named-action routes by walking
// [contract.Routes] — exactly like [mountNamedActions] — so a route exists here
// only if the contract's closed allow-list declares it. A desk or session id
// carries slashes ("<cell>/<role>", "<cell>/custom/<id>"), so a route whose
// path has an {id} is mounted at the subtree before it and the handler parses
// the id; a route with no {id} is mounted exactly.
func mountTerminalActions(mux *http.ServeMux, actions *NamedActions) {
	if actions == nil || !actions.Terminal.ready() {
		return
	}
	t := actions.Terminal
	for _, r := range contract.Routes {
		if r.Kind != contract.KindNamedAction {
			continue
		}
		prefix, suffix, hasID := strings.Cut(r.Path, "{id}")
		pattern := string(r.Method) + " " + r.Path
		if hasID {
			pattern = string(r.Method) + " " + prefix
		}
		switch {
		case r.Action == contract.ActionSend && r.Method == contract.POST && hasID:
			mux.HandleFunc(pattern, t.handleSend(prefix, suffix))
		case r.Action == contract.ActionSession && r.Method == contract.POST && !hasID:
			mux.HandleFunc(pattern, t.handleOpen)
		case r.Action == contract.ActionSession && r.Method == contract.DELETE && hasID && suffix == "":
			mux.HandleFunc(pattern, t.handleClose(prefix))
		}
	}
}

// handleSend serves POST /v1/desks/{id}/terminal/send. It is mounted on the
// /v1/desks/ subtree for POST, so a POST to any OTHER path under it (the read
// routes /terminal and /terminal/stream) keeps the 405 the read tree gave it.
func (t *TerminalActions) handleSend(prefix, suffix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
		if !ok || id == "" || !strings.HasPrefix(r.URL.Path, prefix) {
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed: the only write under /v1/desks/ is POST /v1/desks/{id}/terminal/send"})
			return
		}
		if refuseCrossSite(w, r, true) || refuseQuery(w, r) {
			return
		}
		cell := cellOfDesk(id)
		login, err := t.Authority.Authorize(r, cell)
		if err != nil {
			writeAuthorityErr(w, err)
			return
		}
		var req contract.SendRequest
		if !decodeStrict(w, r, maxSendBody, &req) {
			return
		}
		timeout := time.Duration(req.TimeoutMS) * time.Millisecond
		if req.TimeoutMS < 0 {
			timeout = -1
		}
		if err := terminal.ValidateSend([]byte(req.Input), req.WaitFor, timeout, req.RequestID); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, terminal.ErrInputTooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			writeSendRefusal(w, status, err, id, req.RequestID, req.Epoch, "")
			return
		}
		if req.Epoch == "" {
			writeSendRefusal(w, http.StatusBadRequest, terminal.ErrEpochRequired, id, req.RequestID, "", "")
			return
		}
		rid := req.RequestID
		if rid == "" {
			rid = newRequestID()
		}
		rc, err := t.Registry.WaitThenSend(r.Context(), id, []byte(req.Input), req.WaitFor, timeout, terminal.SendBinding{
			Cell:       cell,
			Epoch:      req.Epoch,
			RequestID:  rid,
			Revalidate: t.revalidate(r, cell, login),
		})
		if err != nil {
			writeSendRefusal(w, sendErrStatus(err), err, id, rid, req.Epoch, string(rc.StateAfter))
			return
		}
		writeJSON(w, http.StatusOK, rc.Result())
	}
}

// handleOpen serves POST /v1/sessions: open a custom session on a configured
// desk's own target.
func (t *TerminalActions) handleOpen(w http.ResponseWriter, r *http.Request) {
	if refuseCrossSite(w, r, true) || refuseQuery(w, r) {
		return
	}
	if _, err := t.Authority.Principal(r); err != nil {
		writeAuthorityErr(w, err)
		return
	}
	// The cell is read from the body's `on`, so shape runs before tier here
	// (the package comment's step 3 exception, pr192-N6).
	var req contract.SessionRequest
	if !decodeStrict(w, r, maxSessionBody, &req) {
		return
	}
	if req.On == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "on is required: the configured desk id the session opens on"})
		return
	}
	cell := cellOfDesk(req.On)
	login, err := t.Authority.Authorize(r, cell)
	if err != nil {
		writeAuthorityErr(w, err)
		return
	}
	switch {
	case req.Skill != "" && req.Prompt != "":
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "set skill OR prompt, not both"})
		return
	case req.Skill != "":
		// The skill allow-list is to be read from the bundled plugin manifest;
		// this build carries none, so a skill session is could-not-check rather
		// than an argv assembled from a guessed list. A prompt session covers
		// the same need with the operator's own words.
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "skill sessions are could-not-check on this build: the skill allow-list is read from the bundled plugin manifest, which this build does not carry — open the session with a prompt instead"})
		return
	case len(req.Prompt) > terminal.MaxSendBytes:
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": terminal.ErrInputTooLarge.Error()})
		return
	case len(req.Title) > maxTitleBytes:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is too long"})
		return
	}
	driver := t.Driver
	if driver == nil {
		driver = terminal.RegisteredDriver
	}
	row, err := t.Registry.OpenSession(r.Context(), req.On, cell, login, req.Title, driver, terminal.Size{Cols: 80, Rows: 24})
	if err != nil {
		writeJSON(w, sessionErrStatus(err), map[string]string{"error": err.Error()})
		return
	}
	if req.Prompt != "" {
		input := req.Prompt
		if !strings.HasSuffix(input, "\n") {
			input += "\n"
		}
		rc, _ := t.Registry.WaitThenSend(r.Context(), row.ID, []byte(input), contract.SendWaitPrompt, 0, terminal.SendBinding{
			Cell:       cell,
			Epoch:      t.Registry.Epoch(row.ID),
			RequestID:  newRequestID(),
			Revalidate: t.revalidate(r, cell, login),
		})
		row.PromptOutcome = rc.Outcome
	}
	writeJSON(w, http.StatusCreated, row)
}

// handleClose serves DELETE /v1/sessions/{id}: close a custom session; a
// configured desk is refused.
func (t *TerminalActions) handleClose(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// A DELETE carries no body and is never a CORS "simple" request, so it
		// takes the origin floor but not the content-type requirement.
		if refuseCrossSite(w, r, false) || refuseQuery(w, r) {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, prefix)
		if id == "" || id == r.URL.Path {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no session id in the path"})
			return
		}
		cell := cellOfDesk(id)
		if _, err := t.Authority.Authorize(r, cell); err != nil {
			writeAuthorityErr(w, err)
			return
		}
		if err := t.Registry.CloseSession(id, cell); err != nil {
			msg := err.Error()
			if errors.Is(err, terminal.ErrConfiguredDesk) {
				msg = "configured desk"
			}
			writeJSON(w, sessionErrStatus(err), map[string]string{"error": msg})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// refuseQuery refuses (400), and reports true for, a terminal write carrying a
// query string. No terminal write takes one — the target is the path and the
// payload is the strict body — so a query is never read; refusing it keeps a
// workflow work/node/attempt id or a start/resume/handback/decision verb from
// riding beside an otherwise-valid send or session (the service contract's
// task-workflow amendment: terminal chat and lifecycle control stay distinct).
func refuseQuery(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.RawQuery == "" && !r.URL.ForceQuery {
		return false
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a terminal write takes no query string: the target is the path, the input is the body"})
	return true
}

// refuseCrossSite is the browser-request floor under the three terminal writes
// (review pr192-S1). The viewer cookie is SameSite=Strict, and Strict does not
// separate ports: a page on another port of the same host is same-site, so the
// browser attaches the cookie to its cross-origin request. It refuses, and
// reports true for, any request that:
//
//   - carries a Sec-Fetch-Site other than "same-origin" or "none" (403);
//   - carries an Origin other than deskd's own — the Origin's host[:port] must
//     equal the request's Host; an opaque "null" Origin is foreign (403);
//   - when needJSON, does not declare its body application/json (415). A JSON
//     content type makes the browser send a CORS preflight, and deskd answers
//     no preflight with a grant, so a cross-origin page cannot send one.
//
// A request with neither browser header (a CLI or the contract client) is
// judged by its content type alone. It runs before identity is read, so a
// refused request never reaches the authority, the driver or the registry.
func refuseCrossSite(w http.ResponseWriter, r *http.Request, needJSON bool) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin terminal write refused: Sec-Fetch-Site is " + site + "; a terminal write is accepted only from deskd's own origin"})
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" && !isOwnOrigin(origin, r.Host) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin terminal write refused: the request's Origin is not deskd's own origin"})
		return true
	}
	if needJSON {
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "a terminal write's body must be sent as Content-Type: application/json"})
			return true
		}
	}
	return false
}

// isOwnOrigin reports whether an Origin header names the host[:port] the
// request was addressed to. The scheme is not compared — behind a TLS proxy
// deskd sees plain HTTP for an https origin — because one host:port serves one
// scheme; the port is compared, because that is the boundary SameSite misses.
//
// host is always the Host deskd itself received, never X-Forwarded-Host or
// Forwarded (review pr192-S7): deskd has no trusted-proxy configuration, so
// it cannot tell a proxy's forwarded header from a caller's own. Behind a
// reverse proxy that rewrites Host, a browser terminal write therefore fails
// closed with 403 — an availability limit, not a gap; serving the terminal
// actions through such a proxy needs a configured public origin.
func isOwnOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return host != "" && strings.EqualFold(u.Host, host)
}

// revalidate is the post-wait authority re-check: the same decision as the
// pre-check, and the same viewer.
func (t *TerminalActions) revalidate(r *http.Request, cell, login string) func() error {
	return func() error {
		again, err := t.Authority.Authorize(r, cell)
		if err != nil {
			return err
		}
		if !strings.EqualFold(again, login) {
			return errors.New("the request's viewer changed during the wait")
		}
		return nil
	}
}

// cellOfDesk is the cell a desk or session id belongs to ("" if the id has no
// cell segment — which then authorizes nothing).
func cellOfDesk(id string) string {
	cell, _, ok := strings.Cut(id, "/")
	if !ok {
		return ""
	}
	return cell
}

// decodeStrict decodes exactly one JSON object with no unknown fields and no
// trailing data. A field the contract does not declare — a target selector
// among them — is a 400, never ignored.
func decodeStrict(w http.ResponseWriter, r *http.Request, limit int64, out any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		status := http.StatusBadRequest
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			status = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, status, map[string]string{"error": "request body did not decode as the contract type: " + err.Error()})
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body carries data after the JSON object"})
		return false
	}
	return true
}

// writeAuthorityErr maps an authority refusal to its status; an error with no
// status is a 403 (fail closed).
func writeAuthorityErr(w http.ResponseWriter, err error) {
	status := http.StatusForbidden
	var hs interface{ HTTPStatus() int }
	if errors.As(err, &hs) {
		status = hs.HTTPStatus()
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeSendRefusal(w http.ResponseWriter, status int, err error, id, rid, epoch, state string) {
	outcome := contract.SendNotSent
	if errors.Is(err, terminal.ErrOutcomeUnknown) {
		outcome = contract.SendOutcomeUnknown
	}
	body := map[string]any{"error": err.Error(), "outcome": outcome, "desk_id": id}
	if rid != "" {
		body["request_id"] = rid
	}
	if epoch != "" {
		body["epoch"] = epoch
	}
	if state != "" {
		body["state"] = state
	}
	writeJSON(w, status, body)
}

// sendErrStatus maps a registry refusal to its status.
func sendErrStatus(err error) int {
	switch {
	case errors.Is(err, terminal.ErrNoSuchDesk):
		return http.StatusNotFound
	case errors.Is(err, terminal.ErrWrongCell), errors.Is(err, terminal.ErrRevoked):
		return http.StatusForbidden
	case errors.Is(err, terminal.ErrTooManyPending):
		return http.StatusTooManyRequests
	case errors.Is(err, terminal.ErrInputTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, terminal.ErrInputEmpty), errors.Is(err, terminal.ErrBadWait),
		errors.Is(err, terminal.ErrBadTimeout), errors.Is(err, terminal.ErrBadRequestID),
		errors.Is(err, terminal.ErrEpochRequired):
		return http.StatusBadRequest
	case errors.Is(err, terminal.ErrOutcomeUnknown):
		return http.StatusBadGateway
	default:
		// timeout, epoch changed, not writable, duplicate in flight, canceled:
		// the desk's state conflicts with the request; nothing was written.
		return http.StatusConflict
	}
}

// sessionErrStatus maps a session-store refusal to its status.
func sessionErrStatus(err error) int {
	switch {
	case errors.Is(err, terminal.ErrNoSuchDesk), errors.Is(err, terminal.ErrNoSuchSession):
		return http.StatusNotFound
	case errors.Is(err, terminal.ErrConfiguredDesk), errors.Is(err, terminal.ErrWrongCell):
		return http.StatusForbidden
	case errors.Is(err, terminal.ErrNotConfiguredDesk):
		return http.StatusBadRequest
	case errors.Is(err, terminal.ErrTooManySessions):
		return http.StatusTooManyRequests
	case errors.Is(err, terminal.ErrNoTarget), errors.Is(err, terminal.ErrNoDriver), errors.Is(err, terminal.ErrUnreachable),
		errors.Is(err, terminal.ErrClusterUnauthorized), errors.Is(err, terminal.ErrAttachOff), errors.Is(err, terminal.ErrKindUnsupported):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// newRequestID mints a request id for a send that named none, so every
// result carries the key its receipt is recorded under.
func newRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "t" + strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", "")
	}
	return hex.EncodeToString(b[:])
}
