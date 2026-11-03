package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the ONE HTTP client a client host uses to talk to the
// deskd service over the v1 contract. It performs reads, opens the SSE event
// stream, and invokes the enumerated named actions.
//
// Custody rule (b), client side: a per-request WRITE credential is passed to
// [Client.Answer]/[Client.Ack] as a method argument, placed on the
// Authorization header for that one request, and dropped when the call returns —
// it is NEVER stored on the Client. The read-identity token (viewer-App minted,
// read:user) is the only token the Client holds, and it is a READ token, not a
// write credential. The signed-intent alternative keeps even the write
// credential off this client entirely: the caller signs a [SignedIntent] and
// passes it in the request body.
type Client struct {
	http      *http.Client
	base      string
	readToken string // read-identity bearer; loopback mode leaves it empty
}

// Option configures a Client.
type Option func(*Client)

// WithReadToken sets the read-identity bearer sent as Authorization on reads
// and events. In loopback mode it is omitted and the service trusts the machine
// boundary.
func WithReadToken(tok string) Option { return func(c *Client) { c.readToken = tok } }

// WithHTTPClient overrides the underlying *http.Client (tests inject one).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// NewClient builds a Client for the service at base (e.g. http://127.0.0.1:8787).
func NewClient(base string, opts ...Option) *Client {
	c := &Client{
		http: &http.Client{Timeout: 30 * time.Second},
		base: strings.TrimSuffix(base, "/"),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Inbox reads GET /v1/inbox for a cell ("" = all cells).
func (c *Client) Inbox(ctx context.Context, cell string) (InboxResponse, error) {
	path := Prefix + "/inbox"
	if cell != "" {
		path += "?cell=" + cell
	}
	var out InboxResponse
	meta, err := c.get(ctx, path, &out)
	out.Meta = meta
	return out, err
}

// Escalation reads GET /v1/escalations/{id}.
func (c *Client) Escalation(ctx context.Context, id string) (EscalationResponse, error) {
	var out EscalationResponse
	meta, err := c.get(ctx, Prefix+"/escalations/"+id, &out)
	out.Meta = meta
	return out, err
}

// Get is the generic typed read for the remaining read routes (cells, roadmap,
// portfolio, metrics) whose bodies a caller decodes into its own type.
func (c *Client) Get(ctx context.Context, path string, out any) (Meta, error) {
	return c.get(ctx, path, out)
}

// get performs a read: read-identity bearer on Authorization, decode the JSON
// body into out, and lift the ETag header into Meta.
func (c *Client) get(ctx context.Context, path string, out any) (Meta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return Meta{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.readToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.readToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Meta{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Meta{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Meta{}, statusErr(http.MethodGet, path, resp.StatusCode, body)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return Meta{}, fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return Meta{ETag: resp.Header.Get("ETag")}, nil
}

// Events opens GET /v1/events. When lastEventID is non-empty it is sent as the
// Last-Event-ID header so the service replays from its ring buffer. The caller
// reads with [EventStream.Next] and must Close the stream.
func (c *Client) Events(ctx context.Context, lastEventID string) (*EventStream, error) {
	return c.openStream(ctx, Prefix+"/events", lastEventID)
}

// TerminalStream opens GET /v1/desks/{id}/terminal/stream for one desk
// (the service contract). deskID is "<cell>/<role>"; it is path-escaped here, so a
// caller passes the id exactly as GET /v1/desks returned it. lastEventID is the
// scrollback cursor to resume after — pass [EventStream.LastEventID] from the
// stream that dropped, or "" for the desk's whole retained scrollback. The
// stream yields [EventTerminalOutput] and [EventTerminalState] frames, and one
// [EventResync] when the cursor could not be honoured (the client must discard
// what it rendered and continue from the resync frame's id). It is read-only:
// there is no way to send input through it.
func (c *Client) TerminalStream(ctx context.Context, deskID, lastEventID string) (*EventStream, error) {
	return c.openStream(ctx, Prefix+"/desks/"+url.PathEscape(deskID)+"/terminal/stream", lastEventID)
}

// openStream opens an SSE route: read-identity bearer, Last-Event-ID when
// resuming, and a non-200 rendered as an error before any frame is read.
func (c *Client) openStream(ctx context.Context, path, lastEventID string) (*EventStream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if c.readToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.readToken)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, statusErr(http.MethodGet, path, resp.StatusCode, body)
	}
	return NewEventStream(resp.Body, resp.Body), nil
}

// Read marks an escalation read — a service-LOCAL state write (ruling D7), no
// platform write, no credential.
func (c *Client) Read(ctx context.Context, id string) error {
	return c.postNoCred(ctx, Prefix+"/escalations/"+id+"/read")
}

// Refresh nudges the service to re-fetch the issue now — an App-identity read
// on the service, idempotent, no human credential.
func (c *Client) Refresh(ctx context.Context, id string) error {
	return c.postNoCred(ctx, Prefix+"/escalations/"+id+"/refresh")
}

// Answer invokes POST /v1/escalations/{id}/answer. Custody rule (b): supply the
// person's per-request write credential in writeCred (placed on Authorization
// for this one request and dropped on return) OR set req.Intent to a signed
// intent and leave writeCred empty. Supplying neither is a client-side error —
// the service performs no unauthenticated named action.
func (c *Client) Answer(ctx context.Context, id string, req AnswerRequest, writeCred string) (ActionResult, error) {
	if writeCred == "" && req.Intent == nil {
		return ActionResult{}, fmt.Errorf("answer(%s): custody (b) requires a per-request write credential or a signed intent — neither supplied", id)
	}
	var out ActionResult
	err := c.postAction(ctx, Prefix+"/escalations/"+id+"/answer", req, writeCred, &out)
	return out, err
}

// Ack invokes POST /v1/escalations/{id}/ack — the 👀 reaction, authored as the
// person under the same per-request custody as Answer.
func (c *Client) Ack(ctx context.Context, id string, req AckRequest, writeCred string) (ActionResult, error) {
	if writeCred == "" && req.Intent == nil {
		return ActionResult{}, fmt.Errorf("ack(%s): custody (b) requires a per-request write credential or a signed intent — neither supplied", id)
	}
	var out ActionResult
	err := c.postAction(ctx, Prefix+"/escalations/"+id+"/ack", req, writeCred, &out)
	return out, err
}

// Send is POST /v1/desks/{id}/terminal/send (the service contract): type
// req.Input into the desk's live terminal once req.WaitFor holds. It
// authenticates with the client's VIEWER bearer (WithReadToken) — never a write
// credential: typing into a terminal is not a GitHub write. A refusal (409
// timeout, 403, …) is returned as an error carrying the status and body; a
// 502 is outcome-unknown and must not be retried with a NEW request id.
func (c *Client) Send(ctx context.Context, deskID string, req SendRequest) (SendResult, error) {
	var out SendResult
	err := c.viewerAction(ctx, http.MethodPost, Prefix+"/desks/"+url.PathEscape(deskID)+"/terminal/send", req, &out)
	return out, err
}

// OpenSession is POST /v1/sessions: open a custom terminal session on the
// configured desk req.On (the service contract), as the client's viewer identity.
func (c *Client) OpenSession(ctx context.Context, req SessionRequest) (SessionRow, error) {
	var out SessionRow
	err := c.viewerAction(ctx, http.MethodPost, Prefix+"/sessions", req, &out)
	return out, err
}

// CloseSession is DELETE /v1/sessions/{id}: close a custom session. A
// configured desk id is refused by the service (403).
func (c *Client) CloseSession(ctx context.Context, sessionID string) error {
	return c.viewerAction(ctx, http.MethodDelete, Prefix+"/sessions/"+url.PathEscape(sessionID), nil, nil)
}

// viewerAction sends a terminal named action on the VIEWER identity: the read
// bearer, when the client has one (loopback mode sends none).
func (c *Client) viewerAction(ctx context.Context, method, path string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.readToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.readToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusErr(method, path, resp.StatusCode, raw)
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode %s response: %w", path, err)
		}
	}
	return nil
}

// postNoCred posts to a local-state / nudge route that takes no credential.
func (c *Client) postNoCred(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, nil)
	if err != nil {
		return err
	}
	if c.readToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.readToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return statusErr(http.MethodPost, path, resp.StatusCode, body)
	}
	return nil
}

// postAction posts a named action. The per-request write credential rides the
// Authorization header — never the body — and is not retained after this call.
func (c *Client) postAction(ctx context.Context, path string, payload any, writeCred string, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// The WRITE credential authenticates the named action. It is passed for
	// this request only; the Client holds no reference to it after Do returns.
	if writeCred != "" {
		req.Header.Set("Authorization", "Bearer "+writeCred)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusErr(http.MethodPost, path, resp.StatusCode, body)
	}
	if out != nil && len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode %s response: %w", path, err)
		}
	}
	return nil
}

func statusErr(method, path string, status int, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	return fmt.Errorf("%s %s → HTTP %d: %s", method, path, status, snippet)
}
