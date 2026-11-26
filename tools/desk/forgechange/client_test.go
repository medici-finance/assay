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
				maxVisibility                                   string
				withheld                                        []string
				alreadyPosted, wantNotice                       bool
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
				{name: "mixed-case withheld identifier", current: head, visibility: "public", state: "open",
					body: "Reference to restricted-fixture.", maxVisibility: "public",
					withheld: []string{"  ResTRicted-Fixture  "}, wantErr: "withheld.identifier"},
				{name: "missing withheld policy", current: head, visibility: "internal", state: "open",
					body: report, maxVisibility: "internal", withheld: []string{" "}, wantNotice: true, posts: 1},
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
							if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || payload["body"] != tc.body {
								t.Fatalf("wrong comment payload: %v (%v)", payload, err)
							}
							return jsonResponse(req, 201, `{"id":19}`), nil
						default:
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
							return nil, errors.New("unexpected request")
						}
					})}
					var notices []string
					c, err := New(Config{Forge: forge, ProjectURL: project, Repository: "org/repo", Token: "fixture",
						HTTPClient: client, PublicationNotice: func(message string) { notices = append(notices, message) }})
					if err != nil {
						t.Fatal(err)
					}
					maxVisibility := tc.maxVisibility
					if maxVisibility == "" {
						maxVisibility = "private"
					}
					err = c.PublishComment(Publication{Number: 7, Head: head, Body: tc.body, MaxVisibility: maxVisibility,
						AlreadyPosted: tc.alreadyPosted, WithheldIdentifiers: tc.withheld})
					if tc.wantNotice && (len(notices) != 1 || !strings.Contains(notices[0], "withheld identifiers could not be checked")) {
						t.Errorf("missing safe withheld-policy notice: %v", notices)
					}
					if !tc.wantNotice && len(notices) != 0 {
						t.Errorf("unexpected notices: %v", notices)
					}
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
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	pemData := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	for _, tc := range []struct {
		name, expiry string
		status       int
		wantErr      string
	}{
		{name: "created", status: http.StatusCreated, expiry: future},
		{name: "other success status", status: http.StatusOK, expiry: future, wantErr: "did not grant"},
		{name: "expired", status: http.StatusCreated, expiry: "2000-01-01T00:00:00Z", wantErr: "no valid expiry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := transportFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "POST" || req.URL.String() != "https://api.github.com/app/installations/2/access_tokens" {
					t.Fatalf("unexpected mint request: %s %s", req.Method, req.URL)
				}
				var body struct{ Repositories []string }
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil || !reflect.DeepEqual(body.Repositories, []string{"repo"}) {
					t.Fatalf("unexpected mint payload: %v (%v)", body, err)
				}
				return jsonResponse(req, tc.status, fmt.Sprintf(`{"token":"fixture","expires_at":%q}`, tc.expiry)), nil
			})
			result, err := MintGitHubToken(&http.Client{Transport: transport}, "1", "2", "repo", pemData)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || result.Value != "" {
					t.Fatalf("mint result should be empty with %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil || result.Value != "fixture" || result.ExpiresAt.Format(time.RFC3339) != future {
				t.Fatalf("unexpected token or expiry: %v", err)
			}
		})
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

func TestGitLabOpenChangesReportsCap(t *testing.T) {
	var rows []map[string]int
	for number := 1; number <= 100; number++ {
		rows = append(rows, map[string]int{"iid": number})
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Forge: "gitlab", ProjectURL: "https://gitlab.example/org/repo", Repository: "org/repo",
		Token: "fixture", HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(req, 200, string(raw)), nil
		})}}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	numbers, capped, err := c.OpenChanges()
	if len(numbers) != len(rows) || !capped || err != nil {
		t.Fatalf("OpenChanges returned %d rows, capped %v, error %v", len(numbers), capped, err)
	}
}

func TestGitHubChangeReads(t *testing.T) {
	const project = "https://github.com/org/repo"
	head := strings.Repeat("a", 40)
	current := head
	client, err := New(Config{Forge: "github", ProjectURL: project, Repository: "org/repo", Token: "fixture",
		HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			switch req.Method + " " + req.URL.Path {
			case "POST /graphql":
				var query struct {
					Query     string         `json:"query"`
					Variables map[string]any `json:"variables"`
				}
				if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
					t.Fatal(err)
				}
				if query.Variables["owner"] != "org" || query.Variables["name"] != "repo" {
					t.Fatalf("wrong repository: %v", query.Variables)
				}
				if strings.Contains(query.Query, "pullRequests(") {
					return jsonResponse(req, 200, `{"data":{"repository":{"pullRequests":{"nodes":[{"number":7}]}}}}`), nil
				}
				if query.Variables["number"] != float64(7) || !strings.Contains(query.Query, "comments(") {
					t.Fatalf("wrong comment query: %v", query.Variables)
				}
				return jsonResponse(req, 200, `{"data":{"repository":{"pullRequest":{"comments":{"nodes":[{"databaseId":21,"author":{"login":"fixture","__typename":"User","databaseId":4},"body":"/review","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}}}}}`), nil
			case "GET /repos/org/repo/pulls/7":
				return jsonResponse(req, 200, fmt.Sprintf(`{"number":7,"state":"open","head":{"sha":%q},"html_url":%q,"changed_files":2}`, current, project+"/pull/7")), nil
			case "GET /repos/org/repo/pulls/7/files":
				return jsonResponse(req, 200, `[{"filename":"image.png","status":"added"}]`), nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				return nil, errors.New("unexpected request")
			}
		})}})
	if err != nil {
		t.Fatal(err)
	}
	numbers, capped, err := client.OpenChanges()
	if !reflect.DeepEqual(numbers, []int{7}) || capped || err != nil {
		t.Fatalf("open changes: %v, %v, %v", numbers, capped, err)
	}
	comments, err := client.Comments(7)
	if err != nil || len(comments) != 1 || comments[0].AuthorID != 4 || comments[0].AuthorType != "User" || comments[0].DatabaseID != 21 {
		t.Fatalf("unexpected comments: %v, %v", comments, err)
	}
	files, err := client.ChangedFiles(7)
	if err != nil || files.Complete || files.ChangedFiles != 2 || files.HeadSHA != head ||
		len(files.Files) != 1 || files.Files[0].Filename != "image.png" || !files.Files[0].PatchAbsent {
		t.Fatalf("unexpected incomplete file list: %v, %v", files, err)
	}
}

func TestChangedFilesRefusesMovingHead(t *testing.T) {
	head := strings.Repeat("a", 40)
	transport := transportFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/repos/org/repo/pulls/7":
			return jsonResponse(req, 200, fmt.Sprintf(`{"number":7,"state":"open","head":{"sha":%q},"html_url":"https://github.com/org/repo/pull/7","changed_files":1}`, head)), nil
		case "/repos/org/repo/pulls/7/files":
			head = strings.Repeat("b", 40)
			return jsonResponse(req, 200, `[{"filename":"example.txt"}]`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL)
			return nil, errors.New("unexpected request")
		}
	})
	c, err := New(Config{Forge: "github", ProjectURL: "https://github.com/org/repo", Repository: "org/repo",
		Token: "fixture", HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ChangedFiles(7); err == nil || !strings.Contains(err.Error(), "change moved") {
		t.Fatalf("file read accepted a moving change: %v", err)
	}
}
