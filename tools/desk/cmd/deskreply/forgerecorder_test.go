package main

// forgerecorder_test.go — the fake FORGE this package's tests drive the verb through, and the
// successor to the fake `gh` binary they used before the transport migration.
//
// WHY IT REPLACES A FAKE CLI. deskreply reached the forge by launching `gh`, so its tests put
// a compiled stand-in first on PATH and asserted on the argv it recorded. Every forge read and
// write now goes through the resolved deskkit.Forge, so there is no CLI to stand in for. The
// successor is an httptest server that records METHOD, PATH, BODY and the Authorization header
// of every request the verb emits — the same facts one layer down, and two of them (the body,
// the credential actually presented) that an argv recorder could not see at all.
//
// It also carries the #562 assertion the fake CLI used to: the minted token's value encodes
// the installation owner, and a request whose token owner does not match the repo it addresses
// is answered the way GitHub answers an installation token scoped to the wrong org.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// forgeReq is one request the verb emitted.
type forgeReq struct {
	Method string
	Path   string
	Body   string
	Auth   string
}

func (r forgeReq) String() string { return r.Method + " " + r.Path }

// forgeRecorder is the fake forge instance plus the knobs the cases turn. The PR-state knobs
// keep the FAKEGH_* environment names the fake CLI used, so a case that sets
// `FAKEGH_PR_STATE=MERGED` still reads the same.
type forgeRecorder struct {
	srv      *httptest.Server
	requests []forgeReq

	// comments is what ListComments serves (the --workpad path). Each entry is one comment
	// node: id, databaseId, body, isMinimized, author login.
	comments []map[string]any
}

func newForgeRecorder(t *testing.T) *forgeRecorder {
	t.Helper()
	f := &forgeRecorder{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(strings.Builder)
		if r.Body != nil {
			b := make([]byte, 1<<16)
			n, _ := r.Body.Read(b)
			body.Write(b[:n])
		}
		rec := forgeReq{Method: r.Method, Path: r.URL.Path, Body: body.String(),
			Auth: r.Header.Get("Authorization")}
		f.requests = append(f.requests, rec)

		// #562, moved down a layer: the minted token's value names the installation owner it
		// was minted for. A token from the wrong installation cannot see this repo, and the
		// forge says so in the words GitHub uses.
		const prefix = "fake-worker-installation-token-for-"
		tokenOwner := ""
		if i := strings.Index(rec.Auth, prefix); i >= 0 {
			tokenOwner = strings.TrimSpace(rec.Auth[i+len(prefix):])
		}
		pathOwner := ""
		if parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/"); len(parts) >= 2 && parts[0] == "repos" {
			pathOwner = parts[1]
		}
		if pathOwner != "" && tokenOwner != "" && pathOwner != tokenOwner {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "Could not resolve to a Repository with the name '" + pathOwner + "'."})
			return
		}

		enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/graphql":
			if strings.Contains(rec.Body, "updateIssueComment") {
				enc(map[string]any{"data": map[string]any{"updateIssueComment": map[string]any{
					"issueComment": map[string]any{"databaseId": 123}}}})
				return
			}
			enc(map[string]any{"data": map[string]any{"repository": map[string]any{
				"pullRequest": map[string]any{"comments": map[string]any{"nodes": f.comments}}}}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/comments"):
			w.WriteHeader(http.StatusCreated)
			enc(map[string]any{
				"id": 123, "node_id": "IC_new",
				"html_url": "https://github.com/" + env("FAKEGH_PR_REPO", "example-org/tracker") +
					"/pull/7#issuecomment-123",
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pulls/"):
			enc(map[string]any{
				"number": 7, "state": strings.ToLower(env("FAKEGH_PR_STATE", "OPEN")),
				"draft": false, "node_id": "PR_node", "changed_files": 1,
				"user": map[string]any{"login": "worker[bot]", "id": 99},
				"head": map[string]any{
					"sha": env("FAKEGH_PR_OID", "1111111111111111111111111111111111111111"),
					"ref": env("FAKEGH_PR_HEAD", "feature/test-branch"),
				},
				"html_url": "https://github.com/" + env("FAKEGH_PR_REPO", "example-org/tracker") + "/pull/7",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)

	oldBase := forgeAPIBase
	forgeAPIBase = f.srv.URL
	t.Cleanup(func() { forgeAPIBase = oldBase })
	return f
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// writes returns every state-changing request the verb made. It is the successor to the argv
// recorder's `ghCalls` filter and is BROADER: it catches any non-GET request at all, including
// ones no argv fragment was ever written for.
func (f *forgeRecorder) writes() []forgeReq {
	var out []forgeReq
	for _, r := range f.requests {
		if r.Method == http.MethodGet {
			continue
		}
		out = append(out, r)
	}
	return out
}

// posted reports whether the ONE mutating operation deskreply may perform — a comment post —
// was made. Successor to `anyCall(ghCalls, "pr", "comment")`.
func (f *forgeRecorder) posted() bool {
	for _, r := range f.writes() {
		if r.Method == http.MethodPost && strings.HasSuffix(r.Path, "/comments") {
			return true
		}
	}
	return false
}

// edited reports whether the workpad edit mutation was issued.
func (f *forgeRecorder) edited() bool {
	for _, r := range f.writes() {
		if r.Path == "/graphql" && strings.Contains(r.Body, "updateIssueComment") {
			return true
		}
	}
	return false
}
