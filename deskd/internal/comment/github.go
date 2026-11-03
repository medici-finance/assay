package comment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// This file holds UserWriter — the SINGLE outbound-request builder of the user
// comment path. It constructs ONE kind of request: a POST to
// .../issues/{number}/comments, authored with the VIEWER'S token. There is no
// PATCH here (no close), no reaction, no other verb — TestOneUserWritePath
// asserts exactly one request-building file in this package, and
// TestUserWriterConstructsNoClose asserts this file builds no issue-state PATCH.
// "A comment can never carry a close" is therefore a property a diff would break.

// UserWriter posts issue comments as the authenticated viewer, over REST, with
// the viewer's own session token supplied per call (never cached).
type UserWriter struct {
	http    *http.Client
	apiBase string
}

// NewUserWriter builds the writer. apiBase defaults to GitHub's.
func NewUserWriter(apiBase string) *UserWriter {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &UserWriter{
		http:    &http.Client{Timeout: 30 * time.Second},
		apiBase: strings.TrimSuffix(apiBase, "/"),
	}
}

// Comment posts body as an issue comment on ref, authored with token. The token
// is the VIEWER'S — so GitHub attributes the comment to the viewer, and a
// viewer with no write access to a private repo is refused by GitHub itself
// (the layer behind the deskd middleware).
func (w *UserWriter) Comment(ctx context.Context, token string, ref Ref, body string) error {
	owner, name, ok := strings.Cut(ref.Repo, "/")
	if !ok {
		return fmt.Errorf("repo %q is not owner/name", ref.Repo)
	}
	if token == "" {
		return ErrNoAuthor
	}
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", w.apiBase, owner, name, ref.Number)
	payload, err := json.Marshal(map[string]any{"body": body})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return fmt.Errorf("POST %s → HTTP %d: %s", url, resp.StatusCode, snippet)
	}
	return nil
}

// compile-time assertion: UserWriter satisfies the Service's writer contract.
var _ userWriter = (*UserWriter)(nil)
