package server_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/client"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
	"github.com/medici-finance/assay/tools/desk/internal/forgeservice/server"
)

func TestDecodeMirrorQuery(t *testing.T) {
	repo := mirror.RepositoryID{Provider: mirror.GitHub, Host: "github.com", Path: "example/project"}
	scope := service.Scope{
		Repositories: []mirror.RepositoryID{repo}, AdmissionID: "fixture-admission",
		AccessEpoch: "fixture-epoch", CallerID: "fixture-reader", RepresentedActor: "fixture-actor",
	}
	query := service.Request{Schema: service.Schema, ID: "fixture-query", Scope: scope, Repository: repo,
		Operation: service.MirrorQuery, Mirror: &service.MirrorRequest{Kind: mirror.Issues, MaxAgeMS: 60000,
			Minimum: &service.Token{Scope: scope, Repository: repo, Kind: mirror.Issues, Value: "fixture-minimum"}}}
	encode := func(q service.Request) []byte {
		t.Helper()
		wire, err := service.EncodeRequest(q)
		if err != nil {
			t.Fatal(err)
		}
		return wire
	}
	request := func(wire []byte) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(wire))
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	wire := encode(query)
	got, err := server.DecodeMirrorQuery(request(wire))
	if err != nil || !reflect.DeepEqual(got, query) {
		t.Fatalf("valid mirror claims changed: got %#v, err %v", got, err)
	}

	t.Run("client transport uses the same envelope", func(t *testing.T) {
		calls := 0
		c, err := client.New(client.Config{
			Endpoint: "https://example.test/query", Scope: scope, Timeout: time.Second,
			Transport: localTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				got, err := server.DecodeMirrorQuery(r)
				if err != nil || !reflect.DeepEqual(got, query) {
					t.Fatalf("client envelope changed: got %#v, err %v", got, err)
				}
				// This fixture has no authority or data source, so it returns an empty hold.
				out, err := service.EncodeResponse(service.Held(got, mirror.Unknown), got)
				if err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(bytes.NewReader(out)),
				}, nil
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		out, report, err := c.Do(context.Background(), query)
		if !errors.Is(err, service.ErrInvalid) || out.Metadata.State != service.Unavailable || out.Mirror != nil || calls != 1 || report.TransportAttempts != 1 {
			t.Fatalf("unexpected unqualified wire hold: state %s, attempts %d, err %v", out.Metadata.State, calls, err)
		}
	})

	t.Run("known operations outside the mirror surface", func(t *testing.T) {
		read := query
		read.Operation, read.Mirror = service.FileAt, nil
		read.Source = &service.SourceRequest{Revision: strings.Repeat("a", 40), Path: "README.md"}
		write := query
		write.Operation, write.Mirror = service.CreateIssue, nil
		write.Effect = &service.EffectRequest{ID: service.OperationID{Key: "fixture", PayloadDigest: strings.Repeat("a", 64)}, Title: "fixture"}
		for _, other := range []service.Request{read, write} {
			got, err := server.DecodeMirrorQuery(request(encode(other)))
			if !errors.Is(err, service.ErrSchema) || !reflect.DeepEqual(got, service.Request{}) {
				t.Errorf("operation %s was not refused: %v", other.Operation, err)
			}
		}
	})

	for _, test := range []struct {
		name   string
		change func(*http.Request)
	}{
		{"method", func(r *http.Request) { r.Method = http.MethodGet }},
		{"media type", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }},
		{"content encoding", func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }},
		{"foreign field", func(r *http.Request) {
			r.Body = io.NopCloser(strings.NewReader(`{"foreign":true,` + string(wire[1:])))
		}},
		{"duplicate operation", func(r *http.Request) {
			r.Body = io.NopCloser(strings.NewReader(`{"operation":"mirror-query",` + string(wire[1:])))
		}},
		{"conflicting minimum scope", func(r *http.Request) {
			body := bytes.Replace(wire, []byte(`"admissionId":"fixture-admission"`), []byte(`"admissionId":"foreign-admission"`), 1)
			r.Body = io.NopCloser(bytes.NewReader(body))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := request(wire)
			test.change(r)
			got, err := server.DecodeMirrorQuery(r)
			if err == nil || !reflect.DeepEqual(got, service.Request{}) {
				t.Fatalf("invalid request returned claims: %#v, err %v", got, err)
			}
		})
	}

	t.Run("unknown content length is bounded", func(t *testing.T) {
		r := request(wire)
		body := strings.NewReader(strings.Repeat(" ", service.MaxMessageBytes+2))
		r.Body, r.ContentLength = io.NopCloser(body), -1
		got, err := server.DecodeMirrorQuery(r)
		if !errors.Is(err, service.ErrInvalid) || !reflect.DeepEqual(got, service.Request{}) || body.Len() != 1 {
			t.Fatalf("oversized body must refuse after at most limit+1 bytes: err %v, unread %d", err, body.Len())
		}
	})
}

// localTransport connects the real client to the decoder without a listener.
type localTransport func(*http.Request) (*http.Response, error)

func (f localTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
