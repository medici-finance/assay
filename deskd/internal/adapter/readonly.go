package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ReadOnlyViolation is returned INSTEAD OF PERFORMING a request that could
// mutate GitHub. It is an error, not a log line: the request never reaches
// the network, and the caller sees a failure rather than a silent no-op.
type ReadOnlyViolation struct {
	Method string
	URL    string
	Why    string
}

func (e *ReadOnlyViolation) Error() string {
	return fmt.Sprintf("READ-ONLY TRANSPORT REFUSED %s %s: %s — the service contract ships no write path of any kind; "+
		"the human-gate answer path is the service contract and the agent write path is the service contract",
		e.Method, e.URL, e.Why)
}

// ReadOnlyTransport is how "no write path" is enforced STRUCTURALLY rather
// than by convention.
//
// # Why a transport and not a code review
//
// "This brief has no write path" is a claim about every line of code in the
// process, forever. Reviewing call sites proves it about the lines that
// exist today. A sibling decision on this stream refused to ship a
// dormant-but-built write path on the grounds that *the gap between a flag
// and a grant is one commit* — the same reasoning applies to a convention:
// the gap between "we don't call the mutation" and "we call the mutation" is
// one commit, and no test that enumerates today's call sites would notice.
//
// So the refusal lives at the ONE place every outbound request must pass:
// the RoundTripper. There is no client in this package that is not built on
// it ([NewGitHub] is the only constructor and it wraps unconditionally), and
// the transport has no field, option or env var that relaxes it. Adding a
// write would mean deleting this file, which is a visible act in a diff.
//
// # What it permits
//
//   - GET and HEAD to anything.
//   - POST to a GraphQL endpoint whose document contains no `mutation` and
//     no `subscription` operation. GraphQL reads are POSTs, so a blanket
//     method rule would ban the very thing the index needs — the aliased
//     by-number lookups that let a count avoid a list-call cap.
//
// # What it refuses
//
// Everything else, by method: PUT, PATCH, DELETE, and any POST that is not a
// verified-read GraphQL document. The refusal is on the SHAPE of the
// request, so a future caller does not have to know the rule to be bound by
// it.
type ReadOnlyTransport struct {
	Base http.RoundTripper
	// GraphQLPath is the path treated as the GraphQL endpoint. Requests to
	// any other path may not be POSTed at all.
	GraphQLPath string
}

// NewReadOnlyTransport wraps base. A nil base uses http.DefaultTransport.
func NewReadOnlyTransport(base http.RoundTripper) *ReadOnlyTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &ReadOnlyTransport{Base: base, GraphQLPath: "/graphql"}
}

// RoundTrip refuses anything that could write.
func (t *ReadOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.Method {
	case http.MethodGet, http.MethodHead:
		return t.Base.RoundTrip(req)

	case http.MethodPost:
		if !strings.HasSuffix(req.URL.Path, t.GraphQLPath) {
			return nil, &ReadOnlyViolation{req.Method, req.URL.String(),
				"POST is permitted only to the GraphQL read endpoint"}
		}
		body, err := drainBody(req)
		if err != nil {
			return nil, &ReadOnlyViolation{req.Method, req.URL.String(),
				"GraphQL body could not be inspected (" + err.Error() + "), so it could not be proven read-only"}
		}
		if why := writesGraphQL(body); why != "" {
			return nil, &ReadOnlyViolation{req.Method, req.URL.String(), why}
		}
		return t.Base.RoundTrip(req)

	default:
		return nil, &ReadOnlyViolation{req.Method, req.URL.String(),
			"only GET, HEAD, and read-only GraphQL POST are reachable from this process"}
	}
}

// drainBody reads the request body and puts it back, so the refusal check
// does not consume the request it may go on to permit.
func drainBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	raw, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(raw))
	req.ContentLength = int64(len(raw))
	return raw, nil
}

// writesGraphQL returns a non-empty reason when the document could mutate.
//
// The check is LEXICAL and deliberately over-strict: after string literals
// and comments are removed, the words `mutation` and `subscription` may not
// appear anywhere. A read query that legitimately contained a field called
// `mutation` would be refused — and that is the correct trade, because the
// alternative is a hand-rolled GraphQL parser standing between this process
// and an unintended write. A refusal costs one rename; a miss costs a
// mutation nobody authorised.
func writesGraphQL(body []byte) string {
	if len(body) == 0 {
		return "empty GraphQL body — nothing to prove read-only"
	}
	var payload struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "GraphQL body is not JSON, so its operation type could not be determined"
	}
	if strings.TrimSpace(payload.Query) == "" {
		return "GraphQL body carries no `query` field"
	}
	doc := stripGraphQLLiterals(payload.Query)
	for _, banned := range []string{"mutation", "subscription"} {
		if strings.Contains(strings.ToLower(doc), banned) {
			return "GraphQL document contains a `" + banned + "` operation"
		}
	}
	return ""
}

// stripGraphQLLiterals removes string literals and # comments so the word
// scan cannot be defeated by, or fire on, quoted prose.
func stripGraphQLLiterals(q string) string {
	var out strings.Builder
	inStr, inComment, escaped := false, false, false
	for _, r := range q {
		switch {
		case inComment:
			if r == '\n' {
				inComment = false
				out.WriteRune(r)
			}
		case inStr:
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				inStr = false
			}
		case r == '"':
			inStr = true
		case r == '#':
			inComment = true
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
