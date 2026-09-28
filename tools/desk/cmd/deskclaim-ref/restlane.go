package main

// restlane.go — the ONE bounded second lane a claim create or release may take when the git
// transport's receive-pack comes back with the forge's bare, reasonless refusal.
//
// WHY A SECOND LANE AT ALL. On github.com a refs/dispatch/<key> create or delete over
// git-smart-HTTP has been observed to come back as a per-command `ng <ref> failure` — no named
// cause, no compare-and-swap loss (GitHub names that one: "cannot lock ref …: reference already
// exists"), not tied to one credential (App installation tokens of several roles and an
// operator login all hit it), and gone again minutes later with the SAME credential. The claim
// flow fails closed on it, correctly, which stranded dispatches and releases behind a manual
// retry. The REST refs API is a different front end onto the same repository and the same
// installation grant, so when — and only when — the git lane returns that generic word, the
// SAME token is offered to it once.
//
// WHAT IT PRESERVES.
//
//   - Create-if-absent. The git create is a server-side compare-and-swap (old=zero). The REST
//     create (POST git/refs) is create-only by construction: 422 "Reference already exists" is
//     the same answer, and is read as HELD — never as free, never retried as an overwrite.
//   - Release only of what was read. The git delete CAS'd against the value the advertisement
//     showed. The REST DELETE has no old-value, so the lane first re-reads the ref (GET git/ref)
//     and deletes only if it still holds that same value; anything else (moved, unreadable) is
//     could-not-check and nothing is deleted. A ref already gone is a released no-op, as on the
//     git lane.
//   - Fail closed. Every answer the lane cannot positively classify — a non-2xx other than the
//     two named 422s, a transport error, an unparseable body — is could-not-check. One attempt,
//     no retry loop: the lane is a fallback for one refusal, not a way to hammer a forge that is
//     refusing writes.
//   - Scope. Only create (old=zero) and delete fall back. An advance or steal (old≠zero) is a CAS
//     from a specific tag object the REST API cannot express without force=true, which is exactly
//     the clobbering race the git CAS replaced — those stay fail-closed with the server's words.
//   - Host. The lane exists only for a GitHub-resolved repo on github.com, whose REST base is
//     deskkit.GitHubAPIBase. Any other forge or host gets no lane (nil) and keeps today's
//     fail-closed answer.
//
// The token is sent only in the Authorization header of requests to that base; it is never
// placed in a URL, an error, or a log line.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// restAPIBase is the REST base the lane calls. A package var ONLY so a test can point it at an
// httptest server; production is deskkit.GitHubAPIBase, the one home of the GitHub API host.
var restAPIBase = deskkit.GitHubAPIBase

// restLane holds what one repo's REST fallback needs. It is built by newRESTLane or not at all.
type restLane struct {
	base        string
	owner, name string
	token       string
	client      *http.Client
}

// newRESTLane returns the lane for a GitHub-resolved repo on github.com, or nil for any other
// forge/host (no lane: the git lane's refusal stands).
func newRESTLane(kind deskkit.ForgeKind, host, owner, name, token string) *restLane {
	if kind != deskkit.ForgeGitHub || !strings.EqualFold(strings.TrimSpace(host), "github.com") {
		return nil
	}
	return &restLane{base: strings.TrimRight(restAPIBase, "/"), owner: owner, name: name, token: token, client: &http.Client{}}
}

// restCreateOutcome is the REST lane's answer to a create-if-absent.
type restCreateOutcome int

const (
	restCreated restCreateOutcome = iota // the ref was created and holds the new claim tag
	restHeld                             // 422 "Reference already exists": someone holds it
)

// restError is every lane failure: could-not-check, never "free" or "released".
type restError struct{ msg string }

func (e *restError) Error() string { return "REST fallback: " + e.msg }

func restErrorf(format string, a ...any) error { return &restError{msg: fmt.Sprintf(format, a...)} }

// maxRESTErrorBody bounds how much of a refusal body is quoted in an error.
const maxRESTErrorBody = 512

// do sends one JSON request and decodes a JSON response into out (when non-nil and 2xx). It
// returns the status code and, for a non-2xx, the server's `message` field (bounded).
func (r *restLane) do(ctx context.Context, method, path string, body, out any) (int, string, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, "", restErrorf("encode %s %s: %v", method, path, err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.base+path, rd)
	if err != nil {
		return 0, "", restErrorf("build %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		// net/http's error carries the URL (no credential is ever in it); keep only the cause.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return 0, "", restErrorf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var m struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &m)
		msg := strings.TrimSpace(m.Message)
		if len(msg) > maxRESTErrorBody {
			msg = msg[:maxRESTErrorBody]
		}
		return resp.StatusCode, msg, nil
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, "", restErrorf("decode %s %s: %v", method, path, err)
		}
	}
	return resp.StatusCode, "", nil
}

func (r *restLane) repoPath(suffix string) string {
	return "/repos/" + url.PathEscape(r.owner) + "/" + url.PathEscape(r.name) + suffix
}

// refAPIPath renders "refs/dispatch/<id>" as the path the git/ref(s) endpoints take after their
// own prefix: "dispatch/<id>", each segment escaped.
func refAPIPath(ref plumbing.ReferenceName) string {
	segs := strings.Split(strings.TrimPrefix(ref.String(), "refs/"), "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// createClaim writes the claim through the REST lane: the empty blob (idempotent), an annotated
// tag carrying msg over it with the same fixed tagger a Go-minted claim carries, then the ref —
// create-only. 422 "Reference already exists" is HELD.
func (r *restLane) createClaim(ctx context.Context, ref plumbing.ReferenceName, tagName, msg string, when time.Time) (restCreateOutcome, error) {
	var blob struct {
		SHA string `json:"sha"`
	}
	code, m, err := r.do(ctx, http.MethodPost, r.repoPath("/git/blobs"), map[string]string{"content": "", "encoding": "utf-8"}, &blob)
	if err != nil {
		return 0, err
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return 0, restErrorf("create empty blob: HTTP %d %s", code, m)
	}
	if blob.SHA != gitcore.EmptyBlobHash {
		return 0, restErrorf("create empty blob: server answered sha %q, want %s", blob.SHA, gitcore.EmptyBlobHash)
	}

	var tag struct {
		SHA string `json:"sha"`
	}
	code, m, err = r.do(ctx, http.MethodPost, r.repoPath("/git/tags"), map[string]any{
		"tag":     tagName,
		"message": msg,
		"object":  gitcore.EmptyBlobHash,
		"type":    "blob",
		"tagger": map[string]string{
			"name":  gitcore.ClaimTaggerName,
			"email": gitcore.ClaimTaggerEmail,
			"date":  when.UTC().Format(time.RFC3339),
		},
	}, &tag)
	if err != nil {
		return 0, err
	}
	if code != http.StatusCreated {
		return 0, restErrorf("create claim tag: HTTP %d %s", code, m)
	}
	if !plumbing.IsHash(tag.SHA) {
		return 0, restErrorf("create claim tag: server answered no object id")
	}

	code, m, err = r.do(ctx, http.MethodPost, r.repoPath("/git/refs"), map[string]string{"ref": ref.String(), "sha": tag.SHA}, nil)
	if err != nil {
		return 0, err
	}
	switch {
	case code == http.StatusCreated:
		return restCreated, nil
	case code == http.StatusUnprocessableEntity && strings.Contains(strings.ToLower(m), "reference already exists"):
		return restHeld, nil
	default:
		return 0, restErrorf("create %s: HTTP %d %s", ref, code, m)
	}
}

// deleteClaim deletes ref through the REST lane ONLY if it still holds expect — the value the
// git lane's delete was compare-and-swapped against. existed=false means the ref was already
// gone (a released no-op).
func (r *restLane) deleteClaim(ctx context.Context, ref plumbing.ReferenceName, expect plumbing.Hash) (existed bool, err error) {
	var cur struct {
		Ref    string `json:"ref"`
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	code, m, err := r.do(ctx, http.MethodGet, r.repoPath("/git/ref/"+refAPIPath(ref)), nil, &cur)
	if err != nil {
		return false, err
	}
	switch {
	case code == http.StatusNotFound:
		return false, nil
	case code != http.StatusOK:
		return false, restErrorf("read %s before delete: HTTP %d %s", ref, code, m)
	case cur.Ref != ref.String():
		return false, restErrorf("read %s before delete: server answered ref %q", ref, cur.Ref)
	case cur.Object.SHA != expect.String():
		return false, restErrorf("%s moved to %s since it was read at %s — not deleting a claim this release did not read", ref, cur.Object.SHA, expect)
	}

	code, m, err = r.do(ctx, http.MethodDelete, r.repoPath("/git/refs/"+refAPIPath(ref)), nil, nil)
	if err != nil {
		return false, err
	}
	switch {
	case code == http.StatusNoContent || code == http.StatusOK:
		return true, nil
	case code == http.StatusUnprocessableEntity && strings.Contains(strings.ToLower(m), "reference does not exist"):
		return false, nil
	default:
		return false, restErrorf("delete %s: HTTP %d %s", ref, code, m)
	}
}
