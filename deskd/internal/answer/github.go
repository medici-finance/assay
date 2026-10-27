package answer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"io"
	"net/http"
	"strings"
	"time"
)

// This file holds HumanWriter — the SINGLE platform-write client of the
// human-gate answer path. It is the one place in the whole process that
// constructs a mutating GitHub request authored as the human (operator), and
// TestExactlyOneWritePath asserts that: no other file in this package builds a
// non-GET request, so "the answer path is the ONE write path" is a
// property a diff would break rather than a claim a reviewer makes.
//
// It gets its token from a Credential (satisfied by internal/custody) at the
// moment of each write — never cached — so a credential the caller stops
// supplying stops authoring. The Credential's actor is checked against
// [humanActor()] before any request goes out, closing the gap between "the token
// we hold" and "the human we claim to write as".
//
// Under custody model (b) (#1819, the service contract) the credential is supplied
// PER REQUEST rather than held by the service: [NewPerRequestHumanWriter]
// builds a writer that reads it from the request context every time it writes,
// so the service holds none between requests. The static [NewHumanWriter] form
// is retained for a caller that legitimately owns a credential for its own
// lifetime (the client host of the service contract) and for the test suite.

// humanActor reads the non-secret startup policy. The shared policy has no
// custody/write imports, so it can also be reached safely from MCP.
func humanActor() string { return humanid.Login() }

// Ref is a platform issue ref the writer acts on. It is the adapter-edge shape
// (owner/name#number), recovered from a served row's platform deep-link; the
// domain never carries it (spec §1.5).
type Ref struct {
	Repo   string // owner/name
	Number int
	URL    string
}

func (r Ref) valid() bool { return r.Repo != "" && r.Number > 0 }

// ParseRef recovers a Ref from a platform deep-link
// (https://github.com/<owner>/<name>/issues/<n>). It is how the web layer turns
// a served row's links.platform into something the writer can act on.
func ParseRef(platformURL string) (Ref, bool) {
	ref := Ref{URL: platformURL}
	const marker = "/issues/"
	i := strings.LastIndex(platformURL, marker)
	if i < 0 {
		return Ref{}, false
	}
	numPart := platformURL[i+len(marker):]
	if j := strings.IndexAny(numPart, "/?#"); j >= 0 {
		numPart = numPart[:j]
	}
	if _, err := fmt.Sscanf(numPart, "%d", &ref.Number); err != nil {
		return Ref{}, false
	}
	segs := strings.Split(strings.TrimPrefix(platformURL[:i], "https://"), "/")
	if len(segs) < 3 {
		return Ref{}, false
	}
	ref.Repo = segs[len(segs)-2] + "/" + segs[len(segs)-1]
	return ref, ref.valid()
}

// Credential is the operator credential the writer authors with. internal/custody
// satisfies it; a test supplies a fake. Keeping it an interface means this
// package does not have to import custody to be built or tested — but the
// deskd wiring does, which is what makes custody reachable from the write path
// and (asserted separately) unreachable from MCP.
type Credential interface {
	// Token returns the held operator token, or an error when none is held.
	Token() (string, error)
	// TokenActor is the login the token authenticates as; MUST be operator.
	TokenActor() string
	// Present reports whether a credential is held at all.
	Present() bool
}

// IssueState is the current platform state the answer-race check reads
// (spec §10 answer-race row): enough to tell whether the escalation the human
// is answering is still the one they saw.
type IssueState struct {
	Closed    bool
	UpdatedAt time.Time
}

// credentialKey is the request-context key the per-request credential travels
// under. It is unexported, so only [WithCredential] can put one there.
type credentialKey struct{}

// WithCredential returns a context carrying the human credential for ONE
// request. The handler that received the caller's credential calls it; the
// per-request writer reads it back at write time. A request context cannot
// outlive its request, which is what makes "the credential is in no struct that
// outlives the request" a lifetime rather than a promise.
func WithCredential(ctx context.Context, cred Credential) context.Context {
	return context.WithValue(ctx, credentialKey{}, cred)
}

// CredentialFrom returns the per-request credential a context carries.
func CredentialFrom(ctx context.Context) (Credential, bool) {
	if ctx == nil {
		return nil, false
	}
	c, ok := ctx.Value(credentialKey{}).(Credential)
	if !ok || c == nil || !c.Present() {
		return nil, false
	}
	return c, true
}

// HumanWriter authors comment / close / reaction writes as operator over REST.
//
// It holds AT MOST ONE credential source, and under custody model (b) it holds
// none: cred is nil and every write resolves the caller's per-request
// credential from the request context instead.
type HumanWriter struct {
	http    *http.Client
	apiBase string
	cred    Credential
	// perRequest makes the context the credential source. It is set by
	// [NewPerRequestHumanWriter] and is the service's mode: nothing about the
	// writer outlives a request except the HTTP client.
	perRequest bool
	// gateway is the BOT credential a verified human-signed intent's write
	// runs under (the authorization contract ruling 2). It is reachable ONLY through a
	// context carrying a verified [SignedAct] (see token); nil means signed
	// writes are refused. It is never a human credential: its token is
	// re-read on every use and must have the App installation-token shape
	// ([CheckGatewayToken] refuses every user-bound shape), so custody model
	// (b) — no HUMAN credential at rest — is unchanged.
	gateway *GatewayCredential
}

// NewHumanWriter builds the writer over a credential the caller owns for its
// own lifetime. apiBase defaults to GitHub's.
func NewHumanWriter(apiBase string, cred Credential) *HumanWriter {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &HumanWriter{
		http:    &http.Client{Timeout: 30 * time.Second},
		apiBase: strings.TrimSuffix(apiBase, "/"),
		cred:    cred,
	}
}

// NewPerRequestHumanWriter builds the SERVICE's writer (custody model (b)): it
// holds no credential at all and resolves one from each request's context at
// write time. A request that carries none cannot write — [ErrNoCredential] —
// which is the correct answer, not a fallback to some ambient token, because
// there is no ambient token to fall back to.
func NewPerRequestHumanWriter(apiBase string) *HumanWriter {
	w := NewHumanWriter(apiBase, nil)
	w.perRequest = true
	return w
}

// Actor is the identity this writer authors as.
func (w *HumanWriter) Actor() string { return humanActor() }

// SetIntentGateway installs the gateway credential signed-intent writes run
// under. deskd calls it once at startup, and only when the enrolled-key file
// and the gateway environment are both present. It installs the SOURCE; the
// token itself is read per write.
func (w *HumanWriter) SetIntentGateway(g *GatewayCredential) { w.gateway = g }

// GatewayActor is the bot identity signed-intent writes are authored as, or ""
// when no gateway is configured.
func (w *HumanWriter) GatewayActor() string { return w.gateway.Login() }

// token fetches and validates the credential immediately before a write. In
// per-request mode the credential comes from the request context and is dropped
// with it; in static mode it is the one the writer was built with.
func (w *HumanWriter) token(ctx context.Context) (string, error) {
	if !humanid.Enabled() {
		return "", humanid.ErrDisabled
	}
	// The signed-intent path (the authorization contract ruling 2). A context carrying
	// a SignedAct takes the gateway credential — and ONLY if the act is one
	// the verifier minted. It never falls through to the human path below: a
	// request that brought an act is judged on the act.
	if act, ok := signedActFrom(ctx); ok {
		if !act.ok() {
			return "", ErrIntentUnverified
		}
		if w.gateway == nil {
			return "", ErrNoGateway
		}
		// Re-read and re-checked on every write: never a token held for the
		// process lifetime, never a user-bound shape (GatewayCredential).
		return w.gateway.current()
	}
	// No act: the human path, unchanged. The gateway credential is not
	// reachable from here, so a bot write with no verified human signature
	// has nothing to author with.
	cred := w.cred
	if w.perRequest {
		c, ok := CredentialFrom(ctx)
		if !ok {
			return "", ErrNoCredential
		}
		cred = c
	}
	if cred == nil {
		return "", ErrNoCredential
	}
	if cred.TokenActor() != humanActor() {
		return "", fmt.Errorf("the credential authenticates as %q, not %q — refusing to author under the wrong identity", cred.TokenActor(), humanActor())
	}
	tok, err := cred.Token()
	if err != nil {
		return "", err
	}
	if tok == "" {
		return "", ErrNoCredential
	}
	return tok, nil
}

// CurrentState reads the escalation's current platform state for the race
// check. It is a GET; it is the only read the writer performs.
func (w *HumanWriter) CurrentState(ctx context.Context, ref Ref) (IssueState, error) {
	owner, name, ok := strings.Cut(ref.Repo, "/")
	if !ok {
		return IssueState{}, fmt.Errorf("repo %q is not owner/name", ref.Repo)
	}
	tok, err := w.token(ctx)
	if err != nil {
		return IssueState{}, err
	}
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d", w.apiBase, owner, name, ref.Number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return IssueState{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := w.http.Do(req)
	if err != nil {
		return IssueState{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return IssueState{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return IssueState{}, httpErr("GET", url, resp.StatusCode, raw)
	}
	var body struct {
		State     string    `json:"state"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return IssueState{}, fmt.Errorf("issue-state response did not decode: %w", err)
	}
	return IssueState{Closed: strings.EqualFold(body.State, "closed"), UpdatedAt: body.UpdatedAt}, nil
}

// Comment posts the decision as a operator-authored comment.
func (w *HumanWriter) Comment(ctx context.Context, ref Ref, body string) error {
	return w.mutate(ctx, http.MethodPost, w.issuePath(ref, "/comments"), map[string]any{"body": body})
}

// Close closes the issue as operator (the answer's second half).
func (w *HumanWriter) Close(ctx context.Context, ref Ref) error {
	return w.mutate(ctx, http.MethodPatch, w.issuePath(ref, ""), map[string]any{"state": "closed"})
}

// React posts a reaction as operator (D7 ack write-through uses "eyes" / 👀). The
// issue is untouched otherwise — a reaction is not a close.
func (w *HumanWriter) React(ctx context.Context, ref Ref, content string) error {
	return w.mutate(ctx, http.MethodPost, w.issuePath(ref, "/reactions"), map[string]any{"content": content})
}

// issuePath builds an issue-scoped API path with an optional suffix.
func (w *HumanWriter) issuePath(ref Ref, suffix string) string {
	owner, name, _ := strings.Cut(ref.Repo, "/")
	return fmt.Sprintf("%s/repos/%s/%s/issues/%d%s", w.apiBase, owner, name, ref.Number, suffix)
}

// mutate is the ONE place a non-GET GitHub request is built in this package.
// Every human-gate platform write funnels through here, authored as operator.
func (w *HumanWriter) mutate(ctx context.Context, method, url string, payload any) error {
	if !strings.Contains(url, "/repos/") {
		return fmt.Errorf("refusing to write to a non-issue path: %s", url)
	}
	tok, err := w.token(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpErr(method, url, resp.StatusCode, body)
	}
	return nil
}

func httpErr(method, url string, status int, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	return fmt.Errorf("%s %s → HTTP %d: %s", method, url, status, snippet)
}

// compile-time assertion: HumanWriter satisfies the writer contract the
// Service depends on.
var _ platformWriter = (*HumanWriter)(nil)
