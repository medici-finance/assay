package forgechange

import (
	"context"
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
				withheld, privateRepos, humans                  []string
				alreadyPosted                                   bool
				wantNotice                                      string
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
					body: report, maxVisibility: "internal", withheld: []string{" "}, wantNotice: "withheld identifiers could not be checked", posts: 1},
				{name: "padded private repository", current: head, visibility: "public", state: "open",
					body: "Reference to example-org/private-fixture.", maxVisibility: "public",
					privateRepos: []string{"  Example-Org/Private-Fixture  "}, withheld: []string{"withheld-fixture"},
					wantErr: "selfcontain.private-repository-name"},
				{name: "private short reference", current: head, visibility: "public", state: "open",
					body: "Reference to private-fixture#4.", maxVisibility: "public",
					privateRepos: []string{"example-org/private-fixture"}, withheld: []string{"withheld-fixture"},
					wantErr: "selfcontain.cross-repo-reference"},
				{name: "bare private name notice", current: head, visibility: "public", state: "open",
					body: "Reference to private-fixture.", maxVisibility: "public",
					privateRepos: []string{"example-org/private-fixture"}, withheld: []string{"withheld-fixture"},
					wantNotice: "publication scan flagged a possible private reference", posts: 1},
				{name: "padded human login", current: head, visibility: "private", state: "open",
					body: "I (Alex) have decided to publish this.", humans: []string{"  ALEx  "}, wantErr: "voice.ruling-claim"},
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
						AlreadyPosted: tc.alreadyPosted, WithheldIdentifiers: tc.withheld, PrivateRepositories: tc.privateRepos, HumanLogins: tc.humans})
					if tc.wantNotice != "" {
						if len(notices) != 1 || !strings.Contains(notices[0], tc.wantNotice) || strings.Contains(notices[0], "private-fixture") {
							t.Errorf("expected one sanitized notice containing %q, got %v", tc.wantNotice, notices)
						}
					} else if len(notices) != 0 {
						t.Errorf("unexpected publication notice: %v", notices)
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
	head4, head7 := strings.Repeat("4", 40), strings.Repeat("7", 40)
	calls := 0
	transport := transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		// Any pipeline lookup, or any other read, fails the test here.
		if req.URL.EscapedPath() != "/api/v4/projects/org%2Frepo/merge_requests" || req.URL.Query().Get("state") != "opened" {
			t.Fatalf("unexpected discovery request %s", req.URL)
		}
		if req.URL.Query().Get("page") != "2" {
			r := jsonResponse(req, 200, fmt.Sprintf(`[{"iid":4,"state":"opened","draft":true,"author":{"id":52,"username":"contributor"},"sha":%q,"target_branch":"main"}]`, head4))
			r.Header.Set("X-Next-Page", "2")
			return r, nil
		}
		return jsonResponse(req, 200, fmt.Sprintf(`[{"iid":7,"state":"opened","title":"Draft: wip","author":{"id":53,"username":"other"},"sha":%q,"target_branch":"release"}]`, head7)), nil
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
	raw, err := json.Marshal(numbers)
	if err != nil || string(raw) != "[4,7]" {
		t.Fatalf("OpenChanges JSON = %s (%v)", raw, err)
	}
	rows, capped, err := c.OpenChangeRows()
	want := []OpenChange{
		{Number: 4, Author: Account{ID: 52, Login: "contributor"}, Draft: true, State: "open", HeadSHA: head4, BaseRef: "main",
			URL: "https://gitlab.example/org/repo/-/merge_requests/4"},
		{Number: 7, Author: Account{ID: 53, Login: "other"}, Draft: true, State: "open", HeadSHA: head7, BaseRef: "release",
			URL: "https://gitlab.example/org/repo/-/merge_requests/7"},
	}
	if !reflect.DeepEqual(rows, want) || capped || err != nil || calls != 4 {
		t.Fatalf("OpenChangeRows = %+v, %v, %v after %d calls", rows, capped, err, calls)
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

func TestStandaloneProviderMatchesProjectOrigin(t *testing.T) {
	for _, test := range []struct {
		forge, project string
	}{
		{"github", "https://gitlab.example/prefix/org/repo"},
		{"gitlab", "https://github.com/org/repo"},
	} {
		t.Run(test.forge, func(t *testing.T) {
			client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("mismatched project reached the network")
				return nil, nil
			})}
			got, err := New(Config{Forge: test.forge, ProjectURL: test.project, Repository: "org/repo", Token: "fixture", HTTPClient: client})
			if err == nil || got != nil {
				t.Fatalf("accepted %s as a %s project", test.project, test.forge)
			}
		})
	}
}

func TestChangeCarriesAuthorAndDraft(t *testing.T) {
	head := strings.Repeat("a", 40)
	for _, tc := range []struct {
		forge, project, path, body, wantJSON string
	}{
		{"github", "https://github.com/org/repo", "/repos/org/repo/pulls/7",
			fmt.Sprintf(`{"number":7,"state":"open","draft":true,"user":{"login":"contributor","id":41,"type":"User"},"head":{"sha":%q},"base":{"ref":"main"},"html_url":"https://github.com/org/repo/pull/7","changed_files":1}`, head),
			`{"state":"open","headSha":"` + head + `","baseRef":"main","url":"https://github.com/org/repo/pull/7","changedFiles":1,"author":{"id":41,"login":"contributor","type":""},"draft":true}`},
		{"gitlab", "https://gitlab.example/org/repo", "/api/v4/projects/org%2Frepo/merge_requests/7",
			fmt.Sprintf(`{"iid":7,"state":"opened","draft":true,"author":{"id":52,"username":"contributor"},"sha":%q,"target_branch":"main","changes_count":"1","web_url":"https://gitlab.example/org/repo/-/merge_requests/7"}`, head),
			`{"state":"open","headSha":"` + head + `","baseRef":"main","url":"https://gitlab.example/org/repo/-/merge_requests/7","changedFiles":1,"author":{"id":52,"login":"contributor","type":""},"draft":true}`},
	} {
		t.Run(tc.forge, func(t *testing.T) {
			c, err := New(Config{Forge: tc.forge, ProjectURL: tc.project, Repository: "org/repo", Token: "fixture",
				HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.EscapedPath() != tc.path {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					}
					return jsonResponse(req, 200, tc.body), nil
				})}})
			if err != nil {
				t.Fatal(err)
			}
			change, err := c.Change(7)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(change)
			if err != nil || string(raw) != tc.wantJSON {
				t.Fatalf("Change JSON = %s (%v)\nwant %s", raw, err, tc.wantJSON)
			}
		})
	}
}

func TestChangedFilesIgnoresDraftFlip(t *testing.T) {
	head := strings.Repeat("a", 40)
	draft := false
	transport := transportFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/repos/org/repo/pulls/7":
			return jsonResponse(req, 200, fmt.Sprintf(`{"number":7,"state":"open","draft":%t,"user":{"login":"contributor","id":41},"head":{"sha":%q},"html_url":"https://github.com/org/repo/pull/7","changed_files":1}`, draft, head)), nil
		case "/repos/org/repo/pulls/7/files":
			draft = true
			return jsonResponse(req, 200, `[{"filename":"example.txt","status":"modified","patch":"@@"}]`), nil
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
	files, err := c.ChangedFiles(7)
	if err != nil {
		t.Fatalf("a draft flip during the file read failed it: %v", err)
	}
	raw, err := json.Marshal(files)
	want := `{"headSha":"` + head + `","complete":true,"changedFiles":1,"files":[{"filename":"example.txt","status":"modified","patch":"@@","patchAbsent":false}]}`
	if err != nil || string(raw) != want {
		t.Fatalf("ChangedFiles JSON = %s (%v)\nwant %s", raw, err, want)
	}
}

func TestOpenChangeRowsGitHub(t *testing.T) {
	head4, head7 := strings.Repeat("4", 40), strings.Repeat("7", 40)
	calls := 0
	c, err := New(Config{Forge: "github", ProjectURL: "https://github.com/org/repo", Repository: "org/repo", Token: "fixture",
		HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if req.Method != http.MethodPost || req.URL.Path != "/graphql" {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
			}
			var query struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"pullRequests(states:OPEN", "orderBy:{field:CREATED_AT,direction:DESC}", "isDraft", "...on User{databaseId}", "...on Bot{databaseId}", "headRefOid"} {
				if !strings.Contains(query.Query, want) {
					t.Errorf("row query lacks %q: %s", want, query.Query)
				}
			}
			for _, bad := range []string{"statusCheckRollup", "reviews(", "comments("} {
				if strings.Contains(query.Query, bad) {
					t.Errorf("row query requests %q: %s", bad, query.Query)
				}
			}
			if query.Variables["owner"] != "org" || query.Variables["name"] != "repo" {
				t.Fatalf("wrong repository: %v", query.Variables)
			}
			return jsonResponse(req, 200, fmt.Sprintf(`{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":false},"nodes":[`+
				`{"number":7,"state":"OPEN","isDraft":true,"author":{"login":"helper","__typename":"Bot","databaseId":99},"headRefOid":%q,"baseRefName":"main"},`+
				`{"number":4,"state":"OPEN","isDraft":false,"author":{"login":"contributor","__typename":"User","databaseId":41},"headRefOid":%q,"baseRefName":"release"}]}}}}`, head7, head4)), nil
		})}})
	if err != nil {
		t.Fatal(err)
	}
	rows, capped, err := c.OpenChangeRows()
	want := []OpenChange{
		{Number: 7, Author: Account{ID: 99, Login: "helper[bot]", Type: "Bot"}, Draft: true, State: "open", HeadSHA: head7, BaseRef: "main", URL: "https://github.com/org/repo/pull/7"},
		{Number: 4, Author: Account{ID: 41, Login: "contributor", Type: "User"}, State: "open", HeadSHA: head4, BaseRef: "release", URL: "https://github.com/org/repo/pull/4"},
	}
	if err != nil || capped || calls != 1 || !reflect.DeepEqual(rows, want) {
		t.Fatalf("OpenChangeRows = %+v, %v, %v after %d calls", rows, capped, err, calls)
	}
	raw, err := json.Marshal(rows[0])
	wantJSON := `{"number":7,"author":{"id":99,"login":"helper[bot]","type":"Bot"},"draft":true,"state":"open","headSha":"` + head7 + `","baseRef":"main","url":"https://github.com/org/repo/pull/7"}`
	if err != nil || string(raw) != wantJSON {
		t.Fatalf("row JSON = %s (%v)\nwant %s", raw, err, wantJSON)
	}
}

func TestOpenChangeRowsGitHubReportsCap(t *testing.T) {
	page := 0
	c, err := New(Config{Forge: "github", ProjectURL: "https://github.com/org/repo", Repository: "org/repo", Token: "fixture",
		HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			var query struct {
				Variables map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(req.Body).Decode(&query); err != nil {
				t.Fatal(err)
			}
			first := int(query.Variables["first"].(float64))
			nodes := make([]string, 0, first)
			for i := 0; i < first; i++ {
				nodes = append(nodes, fmt.Sprintf(`{"number":%d,"state":"OPEN","headRefOid":%q}`, 1000-page*first-i, strings.Repeat("c", 40)))
			}
			page++
			return jsonResponse(req, 200, fmt.Sprintf(`{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":true,"endCursor":"c%d"},"nodes":[%s]}}}}`,
				page, strings.Join(nodes, ","))), nil
		})}})
	if err != nil {
		t.Fatal(err)
	}
	rows, capped, err := c.OpenChangeRows()
	if err != nil || !capped || len(rows) != 100 || rows[0].Number != 1000 {
		t.Fatalf("OpenChangeRows returned %d rows, capped %v, error %v", len(rows), capped, err)
	}
}

func TestOpenChangeRowsRefusesIncompleteRow(t *testing.T) {
	c, err := New(Config{Forge: "github", ProjectURL: "https://github.com/org/repo", Repository: "org/repo", Token: "fixture",
		HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(req, 200, `{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":false},"nodes":[{"number":7,"state":"OPEN"}]}}}}`), nil
		})}})
	if err != nil {
		t.Fatal(err)
	}
	if rows, _, err := c.OpenChangeRows(); err == nil || rows != nil {
		t.Fatalf("a row without a head was accepted: %v, %v", rows, err)
	}
}

func TestGitLabOpenChangeRowsReportsCap(t *testing.T) {
	var rows []map[string]any
	for number := 1; number <= 100; number++ {
		rows = append(rows, map[string]any{"iid": number, "state": "opened", "sha": strings.Repeat("d", 40)})
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	c, err := New(Config{Forge: "gitlab", ProjectURL: "https://gitlab.example/org/repo", Repository: "org/repo",
		Token: "fixture", HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if req.URL.EscapedPath() != "/api/v4/projects/org%2Frepo/merge_requests" {
				t.Fatalf("unexpected request %s", req.URL)
			}
			return jsonResponse(req, 200, string(raw)), nil
		})}})
	if err != nil {
		t.Fatal(err)
	}
	got, capped, err := c.OpenChangeRows()
	if len(got) != len(rows) || !capped || err != nil || calls != 1 {
		t.Fatalf("OpenChangeRows returned %d rows, capped %v, error %v after %d calls", len(got), capped, err, calls)
	}
}

func TestChangesUpdatedSince(t *testing.T) {
	bound := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		forge, project, path, body string
		wantQuery                  map[string]string
	}{
		{"github", "https://github.com/org/repo", "/repos/org/repo/pulls",
			`[{"number":9,"state":"open","draft":true,"title":"t","updated_at":"2026-10-01T12:00:05Z","user":{"login":"contributor","id":41,"type":"User"},"head":{"sha":"h9","ref":"topic"},"base":{"sha":"b","ref":"main","repo":{"full_name":"org/repo"}}},` +
				`{"number":8,"state":"open","draft":false,"title":"t","updated_at":"2026-10-01T12:00:01Z","user":{"login":"contributor","id":41,"type":"User"},"head":{"sha":"h8","ref":"topic"},"base":{"sha":"b","ref":"main","repo":{"full_name":"org/repo"}}},` +
				`{"number":3,"state":"closed","title":"t","updated_at":"2026-09-30T00:00:00Z","user":{"login":"contributor","id":41,"type":"User"},"head":{"sha":"h3","ref":"topic"},"base":{"sha":"b","ref":"main","repo":{"full_name":"org/repo"}}}]`,
			map[string]string{"state": "all", "sort": "updated", "direction": "desc"}},
		{"gitlab", "https://gitlab.example/org/repo", "/api/v4/projects/org%2Frepo/merge_requests",
			`[{"iid":9,"state":"opened","draft":true,"title":"t","updated_at":"2026-10-01T12:00:05Z","author":{"username":"contributor","id":52,"bot":false},"sha":"h9","source_branch":"topic","target_branch":"main"},` +
				`{"iid":8,"state":"opened","work_in_progress":false,"title":"t","updated_at":"2026-10-01T12:00:01Z","author":{"username":"contributor","id":52,"bot":false},"sha":"h8","source_branch":"topic","target_branch":"main"}]`,
			map[string]string{"state": "all", "order_by": "updated_at", "sort": "desc", "updated_after": "2026-10-01T12:00:00Z"}},
	} {
		t.Run(tc.forge, func(t *testing.T) {
			calls := 0
			c, err := New(Config{Forge: tc.forge, ProjectURL: tc.project, Repository: "org/repo", Token: "fixture",
				HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.Method != http.MethodGet || req.URL.EscapedPath() != tc.path {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					}
					for key, want := range tc.wantQuery {
						if got := req.URL.Query().Get(key); got != want {
							t.Errorf("query %s = %q, want %q", key, got, want)
						}
					}
					return jsonResponse(req, 200, tc.body), nil
				})}})
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.ChangesUpdatedSince(context.Background(), UpdateQuery{Since: bound})
			if err != nil || calls != 1 {
				t.Fatalf("ChangesUpdatedSince: %v after %d calls", err, calls)
			}
			if got.Incomplete || !got.ObservedUpTo.Equal(bound.Add(5*time.Second)) || got.Provider != tc.forge || got.Repository != "org/repo" || !got.Since.Equal(bound) {
				t.Fatalf("coverage = %+v", got.UpdateCoverage)
			}
			if len(got.Records) != 2 || got.Records[0].Number != 9 || !got.Records[0].Draft || got.Records[1].Number != 8 || got.Records[1].Draft ||
				got.Records[0].HeadSHA != "h9" || got.Records[0].Author.Login != "contributor" || got.Records[0].Author.ID == 0 {
				t.Fatalf("records = %+v", got.Records)
			}
		})
	}
}

func TestChangesUpdatedSinceIncompleteKeepsWatermark(t *testing.T) {
	page := 0
	c, err := New(Config{Forge: "gitlab", ProjectURL: "https://gitlab.example/org/repo", Repository: "org/repo", Token: "fixture",
		HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			page++
			r := jsonResponse(req, 200, fmt.Sprintf(`[{"iid":%d,"state":"opened","title":"t","updated_at":"2026-10-01T12:00:00Z","author":{"username":"contributor","id":52},"sha":"h"}]`, page))
			r.Header.Set("X-Next-Page", fmt.Sprint(page+1))
			return r, nil
		})}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.ChangesUpdatedSince(context.Background(), UpdateQuery{})
	if err != nil || !got.Incomplete || !got.ObservedUpTo.IsZero() || got.LimitReason != "page-limit" {
		t.Fatalf("page ceiling: %+v, %v", got.UpdateCoverage, err)
	}
}

func TestChangesUpdatedSinceHonorsCanceledContext(t *testing.T) {
	c, err := New(Config{Forge: "github", ProjectURL: "https://github.com/org/repo", Repository: "org/repo", Token: "fixture",
		HTTPClient: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("a canceled read reached the network: %s", req.URL)
			return nil, errors.New("unexpected request")
		})}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ChangesUpdatedSince(ctx, UpdateQuery{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read returned %v", err)
	}
}

func TestExistingReadJSONUnchanged(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{Comment{AuthorID: 4, AuthorType: "User", DatabaseID: 21, URL: "u", CreatedAt: "c", UpdatedAt: "d", Body: "b"},
			`{"authorId":4,"authorType":"User","databaseId":21,"url":"u","createdAt":"c","updatedAt":"d","body":"b"}`},
		{Files{HeadSHA: "h", ChangedFiles: 1, Files: []File{{Filename: "f", PatchAbsent: true}}},
			`{"headSha":"h","complete":false,"changedFiles":1,"files":[{"filename":"f","status":"","patchAbsent":true}]}`},
	} {
		raw, err := json.Marshal(tc.value)
		if err != nil || string(raw) != tc.want {
			t.Errorf("JSON = %s (%v), want %s", raw, err, tc.want)
		}
	}
}
