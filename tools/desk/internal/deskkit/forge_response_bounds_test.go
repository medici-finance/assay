package deskkit

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
	"github.com/medici-finance/assay/perfrec"
)

const nativeTestLimit = 8 << 20

type nativeCountBody struct {
	io.Reader
	read   int
	closed bool
}

type nativeRequestSink struct{ requests []perfrec.RequestObservation }

func (s *nativeRequestSink) ObserveRequest(r perfrec.RequestObservation) bool {
	s.requests = append(s.requests, r)
	return true
}

func (b *nativeCountBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}
func (b *nativeCountBody) Close() error { b.closed = true; return nil }

type nativeFixtureTransport struct {
	payload  string
	length   int64
	status   int
	encoding string
	chunked  bool
	redirect string
	hits     int
	body     *nativeCountBody
}

func (f *nativeFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.hits++
	payload := []byte(f.payload)
	if f.encoding == "gzip" {
		var buf bytes.Buffer
		z := gzip.NewWriter(&buf)
		_, _ = z.Write(payload)
		_ = z.Close()
		payload = buf.Bytes()
	}
	f.body = &nativeCountBody{Reader: bytes.NewReader(payload)}
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	if f.length >= 0 {
		h.Set("Content-Length", strconv.FormatInt(f.length, 10))
	}
	if f.redirect != "" {
		h.Set("Location", f.redirect)
	}
	if f.encoding != "" {
		h.Set("Content-Encoding", f.encoding)
	}
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	resp := &http.Response{StatusCode: status, Header: h, Body: f.body, ContentLength: f.length, Request: r}
	if f.chunked {
		resp.TransferEncoding = []string{"chunked"}
	}
	return resp, nil
}

// nativeAccess bypasses adapter aggregate accounting but still uses the actual
// admitted backend's mirror-specific transport and native SDK page reader.
func nativeAccess(f Forge, ctx context.Context) adapters.Access {
	if g, ok := f.(*GitHubForge); ok {
		return g.mirrorAccess(ctx)
	}
	return f.(*GitLabForge).mirrorAccess(ctx)
}

func ordinaryAccess(f Forge) adapters.Access {
	if g, ok := f.(*GitHubForge); ok {
		return g.sharedAccess()
	}
	return f.(*GitLabForge).sharedAccess()
}

type nativeCancelTransport struct {
	started chan struct{}
	body    *nativeCancelBody
}
type nativeCancelBody struct {
	ctx     context.Context
	started chan struct{}
	closed  bool
}

func (b *nativeCancelBody) Read([]byte) (int, error) {
	close(b.started)
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (b *nativeCancelBody) Close() error { b.closed = true; return nil }
func (f *nativeCancelTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.body = &nativeCancelBody{ctx: r.Context(), started: f.started}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: f.body, Request: r}, nil
}

// regression: F-mirror-native-allocation
func TestMirrorResponseBoundIsolation(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		t.Run(string(provider), func(t *testing.T) {
			q := url.Values{"page": {"1"}}
			payload := `[]` + strings.Repeat(" ", nativeTestLimit+512)
			rt := &nativeFixtureTransport{payload: payload, length: -1}
			// This admitted client already counts native sends. Context and bound
			// composition must preserve that leaf, not count it a second time.
			f := nativeBackend(provider, countRoundTrips(rt))
			sink := &nativeRequestSink{}
			observer := RequestObserver{Sink: sink, Origin: perfrec.OriginPoll}
			if g, ok := f.(*GitHubForge); ok {
				g.Observer = observer
			} else {
				f.(*GitLabForge).Observer = observer
			}
			before := perfForgeCalls.Load()
			got, err := nativeAccess(f, context.Background()).GetPage(context.Background(), "o/r", "issues", q)
			var limit interface{ LimitBytes() int64 }
			if err == nil || !errors.As(err, &limit) || len(got.Body) != 0 || rt.body.read != nativeTestLimit+1 || !rt.body.closed {
				t.Fatalf("native overflow escaped without aggregate accounting: bytes=%d body=%+v err=%v", len(got.Body), rt.body, err)
			}
			if perfForgeCalls.Load()-before != 1 || rt.hits != 1 {
				t.Fatalf("attempts counted more than once: %d/%d", perfForgeCalls.Load()-before, rt.hits)
			}
			if len(sink.requests) != 1 || sink.requests[0].Bytes == nil || *sink.requests[0].Bytes != nativeTestLimit+1 {
				t.Fatalf("overflow response bytes were lost or duplicated: %+v", sink.requests)
			}
			beforeHits := rt.hits
			_, err = nativeAccess(f, context.Background()).GetPage(context.Background(), "other/repo", "issues", q)
			if !IsRefused(err) || rt.hits != beforeHits {
				t.Fatalf("scope refusal sent a request: %v %d", err, rt.hits)
			}
			if _, err := ReadOnly(f).FileIssue(ForgeRepo{Owner: "o", Name: "r"}, IssueInput{Title: "example"}); !IsRefused(err) || rt.hits != beforeHits {
				t.Fatalf("readonly boundary changed: %v", err)
			}
			// Reuse the ORIGINAL backend after mirror reads. Its ordinary page is
			// deliberately above the mirror cap and must retain its prior behavior.
			pg, err := ordinaryAccess(f).GetPage(context.Background(), "o/r", "issues", q)
			if err != nil || len(pg.Body) != 2 || rt.body.read != len(payload) || !rt.body.closed {
				t.Fatalf("mirror cap changed ordinary native reads: len=%d read=%d err=%v", len(pg.Body), rt.body.read, err)
			}
			for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
				errorsRT := &nativeFixtureTransport{payload: `{"message":"denied"}`, length: -1, status: status}
				_, err := nativeAccess(nativeBackend(provider, errorsRT), context.Background()).GetPage(context.Background(), "o/r", "issues", q)
				var api *ForgeAPIError
				if !errors.As(err, &api) || api.Status != status || !errorsRT.body.closed {
					t.Fatalf("ordinary error classification changed: %v", err)
				}
			}
			cancelRT := &nativeCancelTransport{started: make(chan struct{})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := nativeAccess(nativeBackend(provider, cancelRT), ctx).GetPage(ctx, "o/r", "issues", q)
				done <- err
			}()
			select {
			case <-cancelRT.started:
			case <-time.After(time.Second):
				t.Fatal("read did not start")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) || !cancelRT.body.closed {
					t.Fatalf("cancellation lost cause/close: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("native body ignored cancellation")
			}
		})
	}
	t.Run("gitlab-redirect", func(t *testing.T) {
		rt := &nativeFixtureTransport{payload: `{"message":"redirect"}`, length: -1, status: http.StatusTemporaryRedirect, redirect: "https://other.example.invalid/target"}
		_, err := nativeAccess(nativeBackend(ForgeGitLab, rt), context.Background()).GetPage(context.Background(), "o/r", "issues", nil)
		if err == nil || rt.hits != 1 || !rt.body.closed {
			t.Fatalf("GitLab mirror followed redirect: %v hits=%d", err, rt.hits)
		}
	})
}

func nativeBackend(provider ForgeKind, rt http.RoundTripper) Forge {
	c := &http.Client{Transport: rt}
	if provider == ForgeGitHub {
		return &GitHubForge{Token: ciFixtureToken, Client: c, onlyRepo: "o/r"}
	}
	return &GitLabForge{Token: glTestToken, Client: c, onlyRepo: "o/r"}
}

func nativePayload(provider ForgeKind, kind MirrorRecordKind) string {
	if provider == ForgeGitHub {
		extra := ""
		if kind == MirrorChanges {
			extra = `,"head":{"sha":"head"},"base":{"sha":"base"}`
		}
		return `[{"number":1,"state":"open","title":"useful","updated_at":"2026-10-01T12:00:00Z"` + extra + `}]`
	}
	extra := ""
	if kind == MirrorChanges {
		extra = `,"sha":"head","source_branch":"topic","target_branch":"main"`
	}
	return `[{"iid":1,"state":"opened","title":"useful","updated_at":"2026-10-01T12:00:00Z"` + extra + `}]`
}

// regression: F-mirror-native-allocation
func TestMirrorNativeResponseBounds(t *testing.T) {
	for _, provider := range []ForgeKind{ForgeGitHub, ForgeGitLab} {
		for _, kind := range []MirrorRecordKind{MirrorIssues, MirrorChanges} {
			for _, mode := range []string{"small", "exact", "gzip-small", "gzip-exact", "known", "absent", "false", "chunked", "gzip", "error", "error-gzip", "error-rate", "error-rate-gzip", "error-server", "error-server-gzip", "prefix"} {
				t.Run(string(provider)+"/"+string(kind)+"/"+mode, func(t *testing.T) {
					payload := nativePayload(provider, kind)
					rt := &nativeFixtureTransport{length: -1}
					oversize := !strings.HasSuffix(mode, "small") && !strings.HasSuffix(mode, "exact")
					if mode == "prefix" {
						payload = `[]`
					}
					if strings.HasPrefix(mode, "error") {
						payload = `{"message":"denied"}`
						rt.status = http.StatusForbidden
						if strings.Contains(mode, "rate") {
							rt.status = http.StatusTooManyRequests
						}
						if strings.Contains(mode, "server") {
							rt.status = http.StatusServiceUnavailable
						}
					}
					if !strings.HasSuffix(mode, "small") {
						size := nativeTestLimit
						if oversize {
							size += 1024
						}
						payload += strings.Repeat(" ", size-len(payload))
					}
					rt.payload = payload
					if mode == "known" {
						rt.length = int64(len(payload))
					}
					if mode == "false" {
						rt.length = 2
					}
					if mode == "chunked" {
						rt.chunked = true
					}
					if strings.Contains(mode, "gzip") {
						rt.encoding = "gzip"
					}
					f := nativeBackend(provider, rt)
					source, err := NewMirrorReadSource(ReadOnly(f), ForgeResolution{Kind: provider, Repo: ForgeRepo{Owner: "o", Name: "r"}})
					if err != nil {
						t.Fatal(err)
					}
					got, err := source.Read(context.Background(), kind, adapters.UpdateQuery{})
					if oversize {
						var limit interface{ LimitBytes() int64 }
						if err == nil || !errors.As(err, &limit) || limit.LimitBytes() != nativeTestLimit || ExitCodeOf(err) != ExitUnverifiable {
							t.Fatalf("overflow lacks typed limit refusal: %v", err)
						}
						if len(got.Issues)+len(got.Changes) != 0 || !got.ObservedUpTo.IsZero() {
							t.Fatalf("overflow exposed observations: %+v", got)
						}
						var api *ForgeAPIError
						if strings.HasPrefix(mode, "error") {
							if !errors.As(err, &api) || api.Status != rt.status || api.Body != "" || api.Message != "" || (rt.status == http.StatusForbidden && !IsForgeForbidden(err)) || (rt.status == http.StatusTooManyRequests && !IsForgeRateLimited(err)) {
								t.Fatalf("limit lost known native error status: %v", err)
							}
						} else if errors.As(err, &api) {
							t.Fatalf("successful HTTP response acquired API failure: %+v", api)
						}
					} else if err != nil || len(got.Issues)+len(got.Changes) != 1 || got.Incomplete || got.ObservedUpTo.IsZero() {
						t.Fatalf("useful exact/below-bound population refused: %+v %v", got, err)
					}
					if rt.hits != 1 || rt.body == nil || !rt.body.closed {
						t.Fatalf("request/body lifecycle: hits=%d body=%+v", rt.hits, rt.body)
					}
					if rt.encoding == "" && rt.body.read > nativeTestLimit+1 {
						t.Fatalf("native read consumed %d bytes", rt.body.read)
					}
				})
			}
		}
	}
}
