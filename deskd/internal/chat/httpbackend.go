package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPBackend is the subscription model endpoint, spoken to over the
// OpenAI-compatible chat/completions shape both configured rungs serve.
//
// # The key
//
// The credential is held as a [Credential], whose value is unexported and
// whose every rendering path emits [redaction]. It is set on exactly ONE line
// in this file — the request header — and appears nowhere else: not in a log,
// not in an error, not in a struct this package hands upward. HTTPBackend
// itself carries no exported field that could reach it, and its own String()
// is overridden so that dumping the backend dumps nothing.
//
// # What is sent
//
// The operator's question, the BOUND tool descriptions, and the raw JSON
// results of reads the model asked for. That is desk data leaving the cluster,
// which is why this whole component is gate: human — see the brief. Nothing
// here decides that it may; a client is constructed only when a cell's config
// says so, and a cell says so only when a human edits it.
type HTTPBackend struct {
	name     string
	endpoint string
	cred     Credential
	client   *http.Client
}

// NewHTTPBackend builds a backend against a cell's configured endpoint,
// reading the subscription key from the environment variable the config NAMES.
// It refuses a plaintext endpoint: the key must not travel in clear.
func NewHTTPBackend(name, endpoint, keyEnv string, hc *http.Client) (*HTTPBackend, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("chat: no model endpoint configured")
	}
	if !strings.HasPrefix(endpoint, "https://") && !isLoopbackTestEndpoint(endpoint) {
		return nil, fmt.Errorf("chat: endpoint %q is not https — the subscription key must not travel in clear", endpoint)
	}
	cred, err := CredentialFromEnv(keyEnv)
	if err != nil {
		return nil, err
	}
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	return &HTTPBackend{name: name, endpoint: strings.TrimRight(endpoint, "/"), cred: cred, client: hc}, nil
}

// isLoopbackTestEndpoint admits an http:// endpoint on the loopback interface
// ONLY. That is what an httptest server is, and it is the same machine-boundary
// reasoning internal/mcp's default caller auth uses: a plaintext hop that never
// leaves the host puts nothing on a wire. Any other http:// endpoint is
// refused above.
func isLoopbackTestEndpoint(endpoint string) bool {
	return strings.HasPrefix(endpoint, "http://127.0.0.1:") || strings.HasPrefix(endpoint, "http://[::1]:")
}

// Name reports the backend service name.
func (b *HTTPBackend) Name() string { return b.name }

// String is overridden so that printing the backend cannot print its key,
// whatever a caller does with a %v.
func (b *HTTPBackend) String() string { return "chat.HTTPBackend(" + b.name + ")" }

// completionRequest is the OpenAI-compatible request body.
type completionRequest struct {
	Model    string          `json:"model"`
	Messages []chatMessage   `json:"messages"`
	Tools    []toolAdvertise `json:"tools,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type toolAdvertise struct {
	Type     string            `json:"type"`
	Function toolAdvertiseFunc `json:"function"`
}

type toolAdvertiseFunc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type completionResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete performs one turn against the endpoint.
func (b *HTTPBackend) Complete(ctx context.Context, model string, p Prompt) (Reply, error) {
	body := completionRequest{Model: model, Messages: messagesFor(p)}
	for _, t := range p.Tools {
		body.Tools = append(body.Tools, toolAdvertise{
			Type:     "function",
			Function: toolAdvertiseFunc{Name: t.Name, Description: t.Description},
		})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Reply{}, fmt.Errorf("chat: request did not encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Reply{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	// THE ONE CALL SITE. b.cred.header() is the only reader of the key's
	// value in this package, and this is the only place it is called.
	req.Header.Set("Authorization", b.cred.header())

	resp, err := b.client.Do(req)
	if err != nil {
		// The error is returned as-is from net/http. It names the endpoint,
		// never a header — this package adds nothing to it.
		return Reply{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// The status line only. The response BODY is deliberately not echoed:
		// an endpoint that rejects a request sometimes quotes the request back.
		return Reply{}, fmt.Errorf("chat: %s answered %s", b.name, resp.Status)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Reply{}, err
	}
	var cr completionResponse
	if err := json.Unmarshal(payload, &cr); err != nil {
		return Reply{}, fmt.Errorf("chat: %s returned a body this client does not understand", b.name)
	}
	if len(cr.Choices) == 0 {
		return Reply{}, fmt.Errorf("chat: %s returned no choices", b.name)
	}
	msg := cr.Choices[0].Message
	out := Reply{Text: msg.Content}
	for _, tc := range msg.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, RequestedCall{Tool: tc.Function.Name, Args: []byte(tc.Function.Arguments)})
	}
	return out, nil
}

// messagesFor builds the turn's messages. The system message states the
// numbers rule to the model as well — but note that this is the WEAKEST of the
// four controls and is not relied upon: a model that ignores it produces text
// with no tool call behind it, and the figure layer then renders nothing. The
// instruction is here to make the model's job easier, not to make the system
// safe.
func messagesFor(p Prompt) []chatMessage {
	msgs := []chatMessage{{
		Role: "system",
		Content: "You answer questions about a software delivery desk. " +
			"You may call only the read tools offered to you. " +
			"Every number you state must come from a tool result; if you have not read it, say you do not know. " +
			"Numbers you state in prose are discarded by the renderer — only tool results become cited figures.",
	}, {
		Role:    "user",
		Content: p.Question,
	}}
	for _, r := range p.Results {
		msgs = append(msgs, chatMessage{
			Role:    "user",
			Content: "result of " + r.Tool + ":\n" + string(r.Raw),
		})
	}
	return msgs
}
