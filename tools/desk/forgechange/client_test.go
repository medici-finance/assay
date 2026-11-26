package forgechange

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func jsonResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)),
		Header: http.Header{"Content-Type": {"application/json"}}, Request: req}
}

func TestPublishBindsCommentToChangeAndPolicy(t *testing.T) {
	const report = "Review finished."
	head := strings.Repeat("a", 40)
	for _, forge := range []string{"github", "gitlab"} {
		t.Run(forge, func(t *testing.T) {
			project := "https://github.com/org/repo"
			repoPath := "/repos/org/repo"
			changePath, postPath := repoPath+"/pulls/7", repoPath+"/issues/7/comments"
			if forge == "gitlab" {
				project = "https://gitlab.example/prefix/org/repo"
				repoPath = "/prefix/api/v4/projects/org%2Frepo"
				changePath = repoPath + "/merge_requests/7"
				postPath = changePath + "/notes"
			}
			for _, tc := range []struct {
				name, current, visibility, state, body, wantErr string
				alreadyPosted                                   bool
				posts                                           int
			}{
				{name: "fresh", current: head, visibility: "private", state: "open", body: report, posts: 1},
				{name: "duplicate", current: head, visibility: "private", state: "open", body: report, alreadyPosted: true},
				{name: "stale duplicate", current: strings.Repeat("b", 40), visibility: "private", state: "open",
					body: report, wantErr: "differs from current head", alreadyPosted: true},
				{name: "closed", current: head, visibility: "private", state: "closed", body: report, wantErr: "no longer open"},
				{name: "visibility changed", current: head, visibility: "public", state: "open", body: report, wantErr: "visibility exceeds"},
				{name: "sensitive content", current: head, visibility: "private", state: "open", body: "Contact owner" + "@" + "fixture.company",
					wantErr: "pii.email"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					posts := 0
					client := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
						if forge == "github" && req.Header.Get("Authorization") != "token fixture" ||
							forge == "gitlab" && req.Header.Get("PRIVATE-TOKEN") != "fixture" {
							t.Errorf("wrong credential header for %s", forge)
						}
						switch req.Method + " " + req.URL.EscapedPath() {
						case "GET " + repoPath:
							return jsonResponse(req, 200, fmt.Sprintf(`{"visibility":%q}`, tc.visibility)), nil
						case "GET " + changePath:
							if forge == "gitlab" {
								state := tc.state
								if state == "open" {
									state = "opened"
								}
								return jsonResponse(req, 200, fmt.Sprintf(`{"iid":7,"state":%q,"sha":%q,"changes_count":"0","web_url":%q}`,
									state, tc.current, project+"/-/merge_requests/7")), nil
							}
							return jsonResponse(req, 200, fmt.Sprintf(`{"number":7,"state":%q,"head":{"sha":%q},"html_url":%q}`,
								tc.state, tc.current, project+"/pull/7")), nil
						case "POST " + postPath:
							posts++
							var payload map[string]any
							if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || payload["body"] != report {
								t.Fatalf("wrong comment payload: %v (%v)", payload, err)
							}
							return jsonResponse(req, 201, `{"id":19}`), nil
						default:
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
							return nil, errors.New("unexpected request")
						}
					})}
					c, err := New(Config{Forge: forge, ProjectURL: project, Repository: "org/repo", Token: "fixture", HTTPClient: client})
					if err != nil {
						t.Fatal(err)
					}
					err = c.PublishComment(Publication{Number: 7, Head: head, Body: tc.body, MaxVisibility: "private", AlreadyPosted: tc.alreadyPosted})
					if (tc.wantErr == "" && err != nil) || (tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr))) {
						t.Fatalf("PublishComment error = %v, want %q", err, tc.wantErr)
					}
					if posts != tc.posts {
						t.Fatalf("posted %d comments, want %d", posts, tc.posts)
					}
				})
			}
		})
	}
}

func TestGitLabDiscoveryAvoidsPipelineReads(t *testing.T) {
	calls := 0
	transport := transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.EscapedPath() != "/api/v4/projects/org%2Frepo/merge_requests" || req.URL.Query().Get("state") != "opened" {
			t.Fatalf("unexpected discovery request %s", req.URL)
		}
		if calls == 1 {
			r := jsonResponse(req, 200, `[{"iid":4}]`)
			r.Header.Set("X-Next-Page", "2")
			return r, nil
		}
		return jsonResponse(req, 200, `[{"iid":7}]`), nil
	})
	cfg := Config{Forge: "gitlab", ProjectURL: "https://gitlab.example/org/repo", Repository: "org/repo",
		Token: "fixture", HTTPClient: &http.Client{Transport: transport}}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	numbers, capped, err := c.OpenChanges()
	if !reflect.DeepEqual(numbers, []int{4, 7}) || capped || err != nil || calls != 2 {
		t.Fatalf("OpenChanges = %v, %v, %v after %d calls", numbers, capped, err, calls)
	}
}

func TestMintUsesOnlyConfiguredRepository(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	transport := transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != "POST" || req.URL.String() != "https://api.github.com/app/installations/2/access_tokens" {
			t.Fatalf("unexpected mint request: %s %s", req.Method, req.URL)
		}
		var body struct{ Repositories []string }
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || !reflect.DeepEqual(body.Repositories, []string{"repo"}) {
			t.Fatalf("unexpected mint payload: %v (%v)", body, err)
		}
		return jsonResponse(req, 201, fmt.Sprintf(`{"token":"fixture","expires_at":%q}`, until)), nil
	})
	pemData := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	token, err := MintGitHubToken(&http.Client{Transport: transport}, "1", "2", "repo", pemData)
	if err != nil || token.Value != "fixture" || token.ExpiresAt.Format(time.RFC3339) != until {
		t.Fatalf("unexpected token or expiry: %v", err)
	}
}

func TestGitLabCredentialDoesNotFollowRedirect(t *testing.T) {
	calls := 0
	cfg := Config{Forge: "gitlab", Repository: "org/repo", Token: "fixture", ProjectURL: "https://gitlab.example/org/repo",
		HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			res := jsonResponse(req, http.StatusTemporaryRedirect, "")
			res.Header.Set("Location", "https://other.example/")
			return res, nil
		})}}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Change(7); err == nil || calls != 1 {
		t.Fatalf("redirect should fail after one request, got %d calls and %v", calls, err)
	}
}
