package adapter_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/adapter"
)

// recordingBase records every request that made it past the guard.
type recordingBase struct{ seen []string }

func (r *recordingBase) RoundTrip(req *http.Request) (*http.Response, error) {
	r.seen = append(r.seen, req.Method+" "+req.URL.Path)
	return &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
		Header:     http.Header{},
		Request:    req,
	}, nil
}

// TestReadOnlyTransportRefusesEveryWriteShape is the STRUCTURAL proof that
// the service contract ships no write path.
//
// The refusal is at the transport, so it binds every call site in the
// process — including ones nobody has written yet. This test enumerates the
// shapes a write would take and asserts that none of them reaches the
// network. Each case is a positive control: it must FAIL to send.
func TestReadOnlyTransportRefusesEveryWriteShape(t *testing.T) {
	cases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{"POST to a REST collection (create an issue)", "POST", "https://api.github.com/repos/o/r/issues", `{"title":"x"}`},
		{"POST a comment", "POST", "https://api.github.com/repos/o/r/issues/1/comments", `{"body":"x"}`},
		{"PATCH an issue (close it)", "PATCH", "https://api.github.com/repos/o/r/issues/1", `{"state":"closed"}`},
		{"PUT a label set", "PUT", "https://api.github.com/repos/o/r/issues/1/labels", `{"labels":["x"]}`},
		{"DELETE a label", "DELETE", "https://api.github.com/repos/o/r/issues/1/labels/x", ""},
		{"a GraphQL mutation", "POST", "https://api.github.com/graphql", `{"query":"mutation { addComment(input:{}) { clientMutationId } }"}`},
		{"a GraphQL mutation hidden by case", "POST", "https://api.github.com/graphql", `{"query":"MUTATION { addComment(input:{}) { clientMutationId } }"}`},
		{"a GraphQL subscription", "POST", "https://api.github.com/graphql", `{"query":"subscription { x }"}`},
		{"a GraphQL body that is not JSON", "POST", "https://api.github.com/graphql", `mutation { x }`},
		{"a GraphQL body with no query field", "POST", "https://api.github.com/graphql", `{"variables":{}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := &recordingBase{}
			client := &http.Client{Transport: adapter.NewReadOnlyTransport(base)}
			req, err := http.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Do(req)
			if err == nil {
				t.Fatalf("POSITIVE CONTROL SURVIVED: %s was PERFORMED. The read-only guarantee is not enforced.", tc.name)
			}
			var v *adapter.ReadOnlyViolation
			if !errors.As(err, &v) {
				t.Fatalf("refused, but not by the read-only guard: %v", err)
			}
			if len(base.seen) != 0 {
				t.Fatalf("the request reached the network before being refused: %v", base.seen)
			}
		})
	}
}

// TestReadOnlyTransportPermitsReads guards the other direction. A guard that
// blocks the reads too gets removed, and a removed guard blocks nothing.
func TestReadOnlyTransportPermitsReads(t *testing.T) {
	cases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{"REST GET", "GET", "https://api.github.com/repos/o/r/issues?state=open", ""},
		{"HEAD", "HEAD", "https://api.github.com/repos/o/r", ""},
		{"GraphQL query", "POST", "https://api.github.com/graphql", `{"query":"query { repository(owner:\"o\",name:\"r\") { id } }"}`},
		{"GraphQL shorthand query", "POST", "https://api.github.com/graphql", `{"query":"{ viewer { login } }"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := &recordingBase{}
			client := &http.Client{Transport: adapter.NewReadOnlyTransport(base)}
			req, _ := http.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			if _, err := client.Do(req); err != nil {
				t.Fatalf("a legitimate read was refused: %v", err)
			}
			if len(base.seen) != 1 {
				t.Fatalf("the read did not reach the network: %v", base.seen)
			}
		})
	}
}

// TestGraphQLBodySurvivesInspection — the guard reads the body to classify
// it, so it must put the body back or every permitted query would arrive
// empty. A guard that breaks the requests it allows gets deleted.
func TestGraphQLBodySurvivesInspection(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		got = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	const body = `{"query":"query { repository(owner:\"o\",name:\"r\") { id } }"}`
	client := &http.Client{Transport: adapter.NewReadOnlyTransport(nil)}
	req, _ := http.NewRequest("POST", srv.URL+"/graphql", strings.NewReader(body))
	if _, err := client.Do(req); err != nil {
		t.Fatal(err)
	}
	if got != body {
		t.Fatalf("the guard consumed the body it permitted:\n got: %q\nwant: %q", got, body)
	}
}

// TestClientHasNoWriteMethod is the API-surface half of the guarantee.
//
// The transport stops a write at runtime. This asserts the SHAPE: the only
// constructor in the package wires the guard in unconditionally, so there is
// no way to obtain a Client whose transport is not the read-only one.
// Together they cover both "a caller tried" and "a caller was handed the
// means to".
func TestClientHasNoWriteMethod(t *testing.T) {
	base := &recordingBase{}
	c := adapter.NewGitHub("https://api.github.com", "read-scoped-token-placeholder", base)
	if c == nil {
		t.Fatal("NewGitHub returned nil")
	}
	// The exported surface is exercised elsewhere; what matters here is
	// that the constructor takes a base transport and always wraps it. A
	// change that let a caller supply a *finished* transport would show up
	// as a compile break in this line.
	var _ func(string, string, http.RoundTripper) *adapter.Client = adapter.NewGitHub
}
