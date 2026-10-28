package deskkit

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type contextBlockingTransport struct{}

func (contextBlockingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	<-r.Context().Done()
	return nil, r.Context().Err()
}
func TestForgePreparationCancellation(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			var f Forge
			c := &http.Client{Transport: contextBlockingTransport{}}
			if kind == "github" {
				f = &GitHubForge{Token: "fixture", Client: c}
			} else {
				f = &GitLabForge{Token: "fixture", BaseURL: "https://gitlab.example.invalid", Client: c}
			}
			f = OutboundChecked(f, "reviewer")
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			bounded := WithForgeReadContext(f, ctx)
			if !IsOutboundChecked(bounded) {
				t.Fatal("context removed outbound enforcement")
			}
			start := time.Now()
			_, err := bounded.GetPullRequest(ForgeRepo{Owner: "example-org", Name: "tracker"}, 77)
			if err == nil || time.Since(start) > time.Second {
				t.Fatal("preparation request did not honor cancellation", err)
			}
		})
	}
}

func TestGitLabPreparationBase(t *testing.T) {
	s := newGLServer(t)
	s.mr = glMR(nil)
	const base = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	s.mr["diff_refs"] = map[string]any{"base_sha": base, "head_sha": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	pr, e := s.forge().GetPullRequest(glRepo, 7)
	if e != nil || pr == nil || pr.BaseSHA != base {
		t.Fatalf("GitLab exact base was discarded: %#v %v", pr, e)
	}
}
