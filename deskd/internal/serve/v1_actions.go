// v1_actions.go — the ENUMERATED named-action write handlers on the `/v1`
// seam (the service contract increment 2, property P2).
//
// # What this closes
//
// The custody review of origin/main d218068 found the declared contract
// and the mounted route table diverging in both directions:
//
//   - internal/contract declared eleven `/v1` routes of which exactly two are
//     KindNamedAction (answer, ack), with TestNamedActionSetIsClosed proving
//     that set cannot be widened silently — but internal/serve mounted only the
//     six READ routes, so the closed set was "a declaration that no mounted
//     route currently implements".
//   - meanwhile a human-authored, free-text platform write DID exist on the
//     binary, POST /comment/{id}, outside the allow-list entirely.
//
// This file lands the first half: the two named actions become real routes on
// the seam, driven from [contract.Routes] so the mounted set is derived from
// the declaration rather than kept parallel to it. The second half — removing
// the free-text route from the service — is in cmd/deskd, and
// TestServiceExcludesFreeTextCommentPath pins it at the import graph.
//
// # Custody model (b), at the handler
//
// Neither handler holds a credential. Each resolves the person's credential
// from THIS request via [NamedActions.Credential], puts it on the request
// context for the per-request writer, and lets it fall out of scope when the
// request returns. There is nothing to persist and no store to persist it in;
// the persistence audit (internal/custody) is what proves that about the code
// rather than about one execution.
//
// # The body is the contract's JSON (increment 4)
//
// The answer handler used to parse a FORM while the contract (AnswerRequest),
// the generated OpenAPI and contract.Client all send JSON — a conformant
// client's answer arrived with every field empty. Both handlers now decode the
// contract type strictly ([decodeActionBody]).
//
// # Cookie OR signed intent (the authorization contract, gap 3)
//
// A named action passes on EITHER the live-answer session cookie with the
// per-request credential (the browser path, unchanged) OR a verified
// [contract.SignedIntent] whose key is enrolled in the reviewed intent-keys
// file. A request whose body carries an intent is judged on the intent ALONE
// ([NamedActions.handleSigned]): a failed intent is refused, never retried on a
// cookie, because acting while ignoring an intent the client sent is a write the
// client believed was bound by a record. With no verifier wired
// ([NamedActions.Intents] nil) a body carrying an intent is still refused
// ([refuseSignedIntent]), exactly as before.
package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// NamedActions is the wiring the two allow-listed write routes need. It is
// supplied by cmd/deskd; nil leaves the routes unmounted, which is the correct
// shape for a read-only deployment — an absent route is a 404, never a route
// that exists and refuses.
type NamedActions struct {
	// Service executes the answer flow write. Its meaning is unchanged by the
	// relocation: same comment + close, same answer record, same race check.
	Service *answer.Service
	// Sessions validates the live-human session every write is gated on.
	Sessions *answer.SessionStore
	// Lookup resolves an escalation id to its platform ref against the TRUSTED
	// index. The ref never comes from the request.
	Lookup answer.EscalationLookup
	// Credential resolves the PER-REQUEST human credential (custody model (b)).
	// ok=false means the request carried none, which is a 403 — the service has
	// no credential of its own to fall back to.
	Credential func(r *http.Request) (answer.Credential, bool)
	// Intents verifies signed intents against the reviewed intent-keys file
	// (the authorization contract ruling 1). nil — no enrolled key, or no gateway
	// credential to carry the write — refuses every signed intent (400), the
	// pre-gap-3 behaviour.
	Intents *answer.IntentVerifier
	// Terminal wires the terminal named actions, send and session
	// (the service contract, v1_terminal_actions.go). They are mounted from this
	// wiring alone — their identity is the per-request VIEWER identity, not the
	// write credential above — and a nil or partial wiring leaves them
	// unmounted (a 404), independently of answer/ack.
	Terminal *TerminalActions
}

// ready reports whether the wiring can actually execute a write. A partially
// wired NamedActions must not mount: a route that exists but cannot work is
// exactly the "gap between a flag and a grant" this surface refuses.
func (n *NamedActions) ready() bool {
	return n != nil && n.Service != nil && n.Sessions != nil && n.Lookup != nil && n.Credential != nil
}

// mountNamedActions registers the enumerated write routes on the v1 mux. It
// walks [contract.Routes] rather than a second literal list: the ONLY way to
// add a human write route here is to add it to the contract's closed allow-list
// first, which TestNamedActionSetIsClosed already guards.
func mountNamedActions(mux *http.ServeMux, actions *NamedActions) {
	if !actions.ready() {
		return
	}
	for _, r := range contract.Routes {
		if r.Kind != contract.KindNamedAction {
			continue
		}
		switch r.Action {
		case contract.ActionAnswer:
			mux.HandleFunc("POST "+r.Path, actions.handle(contract.ActionAnswer))
		case contract.ActionAck:
			mux.HandleFunc("POST "+r.Path, actions.handle(contract.ActionAck))
		case contract.ActionSend, contract.ActionSession:
			// Mounted by mountTerminalActions (v1_terminal_actions.go) from the
			// Terminal wiring: these take the viewer identity and never the
			// write credential this handler demands.
		default:
			// A contract action with no handler here stays UNMOUNTED (a 404)
			// — never a generic write handler that would make the surface
			// open-ended. TestUnknownNamedActionStaysUnmounted drives this
			// branch with a fifth action injected into the contract.
		}
	}
}

// handle is the shared shape of both named actions: authenticate the live
// session, take the person's per-request credential, resolve the ref
// server-side, execute, drop the credential.
func (n *NamedActions) handle(action contract.Action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Cookie OR intent (ruling 3). A body carrying an intent takes the
		// signed path exclusively; any other body takes the session path below,
		// byte-for-byte as before (peekIntent restores the body).
		if intent := peekIntent(r); intent != nil {
			n.handleSigned(action, w, r)
			return
		}
		sess, ok := n.session(w, r)
		if !ok {
			return
		}
		cred, ok := n.Credential(r)
		if !ok || cred == nil || !cred.Present() {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "no human credential accompanied this request — under custody model (b) the person's credential rides with the named action; the service holds none (#1819)",
			})
			return
		}
		id := domain.EscalationID(r.PathValue("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "escalation id is required in the path"})
			return
		}
		ref, found, err := n.Lookup.Ref(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ref lookup failed: " + err.Error()})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no open escalation with that id in the index"})
			return
		}

		// The credential lives on THIS request's context and nowhere else.
		ctx := answer.WithCredential(r.Context(), cred)

		switch action {
		case contract.ActionAnswer:
			n.answer(ctx, w, r, sess, id, ref)
		case contract.ActionAck:
			n.ack(ctx, w, r, sess, id, ref)
		}
	}
}

// handleSigned is the signed-intent path (rulings 1–3). It needs no session
// and no per-request credential: the verified intent stands in for both, and
// the write runs under the gateway credential. Every refusal happens before
// the Service is called, and the Service and the writer each re-check the act
// on their own.
func (n *NamedActions) handleSigned(action contract.Action, w http.ResponseWriter, r *http.Request) {
	if n.Intents == nil {
		refuseSignedIntent(w)
		return
	}
	id := domain.EscalationID(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "escalation id is required in the path"})
		return
	}
	ref, found, err := n.Lookup.Ref(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ref lookup failed: " + err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no open escalation with that id in the index"})
		return
	}
	switch action {
	case contract.ActionAnswer:
		var body contract.AnswerRequest
		if status, err := decodeActionBody(r, &body, true); err != nil {
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		act, err := n.Intents.Accept(*body.Intent, action, ref.URL)
		if err != nil {
			writeJSON(w, answer.StatusForError(err), map[string]string{"error": err.Error()})
			return
		}
		res, err := n.Service.AnswerSigned(r.Context(), act, answer.AnswerRequest{
			ID:                id,
			Ref:               ref,
			Option:            body.Option,
			OptionsRev:        body.OptionsRev,
			Text:              body.Text,
			LastSeenUpdatedAt: body.LastSeenUpdatedAt,
		})
		if err != nil {
			writeJSON(w, answer.StatusForError(err), map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	case contract.ActionAck:
		var body contract.AckRequest
		if status, err := decodeActionBody(r, &body, true); err != nil {
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		act, err := n.Intents.Accept(*body.Intent, action, ref.URL)
		if err != nil {
			writeJSON(w, answer.StatusForError(err), map[string]string{"error": err.Error()})
			return
		}
		if err := n.Service.AckSigned(r.Context(), act, id, ref); err != nil {
			writeJSON(w, answer.StatusForError(err), map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"acked": true, "reaction": answer.AckContent, "still_open": true, "signed_by": act.Login()})
	}
}

// peekIntent reports the signed intent a named-action body carries, or nil.
// It reads at most maxActionBody bytes and ALWAYS restores r.Body to the full
// original stream, so the handler that runs next decodes exactly what the
// client sent. It is lenient on purpose — it only asks "is there an intent?";
// the strict decode ([decodeActionBody]) is still what the chosen path runs.
func peekIntent(r *http.Request) *contract.SignedIntent {
	if r.Body == nil {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, maxActionBody+1))
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(raw), r.Body), r.Body}
	if len(raw) > maxActionBody {
		return nil
	}
	var probe struct {
		Intent *contract.SignedIntent `json:"intent"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	return probe.Intent
}

// IntentAuthenticated is the capability middleware's stand-in for a viewer
// session on the `/v1/` tree (identity.Route.Authenticated; ruling 3: "the
// the capability model middleware must let a request carrying a verified signed intent
// through with NO viewer session"). It admits ONLY a POST to a named action
// this file mounts (answer, ack) whose body carries an intent that verifies
// against an enrolled key for that action on that escalation. It does not
// consume the nonce — the handler's [answer.IntentVerifier.Accept] re-verifies
// everything and does — so this admission is never the only check.
func (n *NamedActions) IntentAuthenticated(r *http.Request) bool {
	if !n.ready() || n.Intents == nil || r.Method != http.MethodPost {
		return false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, contract.Prefix+"/escalations/")
	if !ok {
		return false
	}
	id, verb, ok := strings.Cut(rest, "/")
	if !ok || id == "" {
		return false
	}
	action := contract.Action(verb)
	if action != contract.ActionAnswer && action != contract.ActionAck {
		return false
	}
	intent := peekIntent(r)
	if intent == nil {
		return false
	}
	ref, found, err := n.Lookup.Ref(domain.EscalationID(id))
	if err != nil || !found {
		return false
	}
	return n.Intents.Check(*intent, action, ref.URL) == nil
}

func (n *NamedActions) session(w http.ResponseWriter, r *http.Request) (*answer.Session, bool) {
	c, err := r.Cookie(answer.SessionCookie)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": answer.ErrNoSession.Error()})
		return nil, false
	}
	sess, err := n.Sessions.Validate(c.Value)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return nil, false
	}
	return sess, true
}

func (n *NamedActions) answer(ctx context.Context, w http.ResponseWriter, r *http.Request, sess *answer.Session, id domain.EscalationID, ref answer.Ref) {
	var body contract.AnswerRequest
	if status, err := decodeActionBody(r, &body, true); err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	if body.Intent != nil {
		refuseSignedIntent(w)
		return
	}
	req := answer.AnswerRequest{
		ID:         id,
		Ref:        ref,
		Option:     body.Option,
		OptionsRev: body.OptionsRev,
		Text:       body.Text,
		// A zero time is "the client did not say what it last saw", exactly
		// as an absent form field was before: the race check is skipped the
		// same way, so the answer's CONDITION is unchanged by the re-encode.
		LastSeenUpdatedAt: body.LastSeenUpdatedAt,
	}
	res, err := n.Service.Answer(ctx, sess, req)
	if err != nil {
		writeJSON(w, answer.StatusForError(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (n *NamedActions) ack(ctx context.Context, w http.ResponseWriter, r *http.Request, sess *answer.Session, id domain.EscalationID, ref answer.Ref) {
	// The ack body is optional ([contract.AckRequest] carries only the
	// signed-intent alternative); an empty body is the credential-in-request
	// default. A non-empty one must be the contract's JSON.
	var body contract.AckRequest
	if status, err := decodeActionBody(r, &body, false); err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	if body.Intent != nil {
		refuseSignedIntent(w)
		return
	}
	if err := n.Service.Ack(ctx, sess, id, ref); err != nil {
		writeJSON(w, answer.StatusForError(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"acked": true, "reaction": answer.AckContent, "still_open": true})
}

// maxActionBody bounds a named-action body. An AnswerRequest is a decision and
// a short text; nothing legitimate comes near this.
const maxActionBody = 64 << 10

// decodeActionBody decodes a named action's body as the contract's JSON
// request type (the service contract increment 4). The contract, the generated
// OpenAPI and contract.Client all speak JSON; the handler used to parse a FORM,
// so a contract-conformant client's answer arrived with every field empty.
//
// It is strict on purpose: the body must be declared application/json, must
// decode into the contract type with NO unknown field (a body that carries a
// field the contract does not declare — a credential-shaped one included — is
// refused, never silently dropped), and must hold exactly one JSON value.
// required=false admits an empty body (the ack's default shape).
func decodeActionBody(r *http.Request, out any, required bool) (int, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, maxActionBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return http.StatusRequestEntityTooLarge, fmt.Errorf("request body is over %d bytes", maxActionBody)
		}
		return http.StatusBadRequest, fmt.Errorf("request body unreadable: %v", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		if required {
			return http.StatusBadRequest, errors.New("a named action needs its JSON request body (contract.AnswerRequest)")
		}
		return 0, nil
	}
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/json" {
		return http.StatusUnsupportedMediaType, fmt.Errorf("named-action bodies are application/json (the /v1 contract); got %q", r.Header.Get("Content-Type"))
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return http.StatusBadRequest, fmt.Errorf("request body is not the contract's JSON shape: %v", err)
	}
	// Decoder.More is not enough here: it reports false before a closing
	// `}` or `]`, so `{...}}` would pass. The only acceptable next token after
	// the one value is end of input.
	if _, err := dec.Token(); err != io.EOF {
		return http.StatusBadRequest, errors.New("request body holds more than one JSON value (or trailing tokens)")
	}
	return 0, nil
}

// refuseSignedIntent answers a named action whose body carries a signed intent
// when no verifier is wired (no key enrolled in the reviewed intent-keys file,
// or no gateway credential to carry the write). ACTING while ignoring the
// intent would perform a write the client believed was bound by a record
// nobody checked. It is refused, fail-closed, before anything is written.
func refuseSignedIntent(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": "signed intents are not accepted by this deployment — no key is enrolled in the reviewed intent-keys file, or no gateway credential is configured, and the service never acts on an intent it has not verified; " +
			"send the per-request credential instead (custody model (b), #1819)",
	})
}
