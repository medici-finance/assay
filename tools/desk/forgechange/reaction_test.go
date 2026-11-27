package forgechange

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// reactionFixture is one forge's answers for the guard reads and the writes.
type reactionFixture struct {
	visibility       string
	visibilityStatus int
	state            string
	current          string
	changeStatus     int
	changeURL        string
	postStatus       int
	postBody         string
	userID           int64
	awards           string
}

type reactionForge struct {
	forge, project, repoPath, changePath, commentPath, reactionPath string
}

func reactionForges() []reactionForge {
	return []reactionForge{
		{forge: "github", project: "https://github.com/org/repo", repoPath: "/repos/org/repo",
			changePath: "/repos/org/repo/pulls/7", commentPath: "/repos/org/repo/issues/7/comments",
			reactionPath: "/repos/org/repo/issues/7/reactions"},
		{forge: "gitlab", project: "https://gitlab.example/prefix/org/repo", repoPath: "/prefix/api/v4/projects/org%2Frepo",
			changePath: "/prefix/api/v4/projects/org%2Frepo/merge_requests/7", commentPath: "/prefix/api/v4/projects/org%2Frepo/merge_requests/7/notes",
			reactionPath: "/prefix/api/v4/projects/org%2Frepo/merge_requests/7/award_emoji"},
	}
}

// reactionClient serves the fixture and records every request and write payload.
func reactionClient(t *testing.T, f reactionForge, fx reactionFixture, requests *[]string, writes *[]map[string]any) *Client {
	t.Helper()
	transport := transportFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.EscapedPath()
		*requests = append(*requests, key)
		switch key {
		case "GET " + f.repoPath:
			if fx.visibilityStatus != 0 {
				return jsonResponse(req, fx.visibilityStatus, `{"message":"fixture"}`), nil
			}
			return jsonResponse(req, 200, fmt.Sprintf(`{"visibility":%q}`, fx.visibility)), nil
		case "GET " + f.changePath:
			if fx.changeStatus != 0 {
				return jsonResponse(req, fx.changeStatus, `{"message":"fixture"}`), nil
			}
			state, url := fx.state, fx.changeURL
			if f.forge == "gitlab" {
				if state == "open" {
					state = "opened"
				}
				if url == "" {
					url = f.project + "/-/merge_requests/7"
				}
				return jsonResponse(req, 200, fmt.Sprintf(`{"iid":7,"state":%q,"sha":%q,"changes_count":"0","web_url":%q}`,
					state, fx.current, url)), nil
			}
			if url == "" {
				url = f.project + "/pull/7"
			}
			return jsonResponse(req, 200, fmt.Sprintf(`{"number":7,"state":%q,"head":{"sha":%q},"html_url":%q}`, state, fx.current, url)), nil
		case "POST " + f.commentPath, "POST " + f.reactionPath:
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("undecodable write payload: %v", err)
			}
			*writes = append(*writes, payload)
			if strings.HasSuffix(key, f.commentPath) {
				return jsonResponse(req, 201, `{"id":19}`), nil
			}
			status, body := fx.postStatus, fx.postBody
			if status == 0 {
				status = 201
			}
			if body == "" {
				field, value := "content", payload["content"]
				if f.forge == "gitlab" {
					field, value = "name", payload["name"]
				}
				body = fmt.Sprintf(`{"id":31,%q:%q}`, field, value)
			}
			return jsonResponse(req, status, body), nil
		case "GET /prefix/api/v4/user":
			return jsonResponse(req, 200, fmt.Sprintf(`{"id":%d,"username":"fixture"}`, fx.userID)), nil
		case "GET " + f.reactionPath:
			return jsonResponse(req, 200, fx.awards), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
			return nil, errors.New("unexpected request")
		}
	})
	c, err := New(Config{Forge: f.forge, ProjectURL: f.project, Repository: "org/repo", Token: "fixture",
		HTTPClient: &http.Client{Transport: transport}, PublicationNotice: func(string) {}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAddReactionRequestShape(t *testing.T) {
	head := strings.Repeat("a", 40)
	gitlabNames := map[string]string{"laugh": "laughing", "confused": "confused",
		"heart": "heart", "hooray": "tada", "rocket": "rocket", "eyes": "eyes"}
	for _, f := range reactionForges() {
		for content, award := range gitlabNames {
			t.Run(f.forge+"/"+content, func(t *testing.T) {
				var requests []string
				var writes []map[string]any
				c := reactionClient(t, f, reactionFixture{visibility: "private", state: "open", current: head}, &requests, &writes)
				if err := c.AddReaction(Reaction{Number: 7, Head: head, Content: content, MaxVisibility: "private"}); err != nil {
					t.Fatal(err)
				}
				want := map[string]any{"content": content}
				if f.forge == "gitlab" {
					want = map[string]any{"name": award}
				}
				if len(writes) != 1 || fmt.Sprint(writes[0]) != fmt.Sprint(want) {
					t.Fatalf("reaction payload %v, want %v", writes, want)
				}
				if got := requests[len(requests)-1]; got != "POST "+f.reactionPath {
					t.Fatalf("last request %s, want the reaction write", got)
				}
			})
		}
	}
}

func TestAddReactionRefusesUnlistedContent(t *testing.T) {
	head := strings.Repeat("a", 40)
	for _, f := range reactionForges() {
		for _, content := range []string{"", "+1", "-1", "thumbsup", "thumbsdown", "EYES", " eyes", ":eyes:", "tada", "laughing", "party", "eyes\n"} {
			t.Run(f.forge+"/"+content, func(t *testing.T) {
				var requests []string
				var writes []map[string]any
				c := reactionClient(t, f, reactionFixture{visibility: "private", state: "open", current: head}, &requests, &writes)
				err := c.AddReaction(Reaction{Number: 7, Head: head, Content: content, MaxVisibility: "private"})
				if err == nil || !strings.Contains(err.Error(), "reaction must be one of") || len(requests) != 0 {
					t.Fatalf("content %q: error %v after requests %v", content, err, requests)
				}
			})
		}
	}
}

func TestAddReactionDuplicateIsSuccess(t *testing.T) {
	head := strings.Repeat("a", 40)
	const taken = `{"message":"404 Award Emoji Name has already been taken"}`
	for _, tc := range []struct {
		name, forge string
		fx          reactionFixture
		wantErr     string
		wantReads   bool
	}{
		{name: "existing reaction", forge: "github", fx: reactionFixture{postStatus: 200, postBody: `{"id":30,"content":"eyes"}`}},
		{name: "mismatched confirmation", forge: "github", fx: reactionFixture{postStatus: 200, postBody: `{"id":30,"content":"heart"}`},
			wantErr: "could not be verified"},
		{name: "existing award", forge: "gitlab", wantReads: true, fx: reactionFixture{postStatus: 404, postBody: taken, userID: 5,
			awards: `[{"id":2,"name":"eyes","user":{"id":9}},{"id":3,"name":"eyes","user":{"id":5}}]`}},
		{name: "another user's award", forge: "gitlab", wantReads: true, wantErr: "HTTP 404", fx: reactionFixture{postStatus: 404,
			postBody: taken, userID: 5, awards: `[{"id":2,"name":"eyes","user":{"id":9}}]`}},
		{name: "own award of another name", forge: "gitlab", wantReads: true, wantErr: "HTTP 404", fx: reactionFixture{postStatus: 404,
			postBody: taken, userID: 5, awards: `[{"id":3,"name":"heart","user":{"id":5}}]`}},
		// The caller's own matching award is present, so a 403 that reached the
		// duplicate confirmation would wrongly succeed.
		{name: "forbidden award", forge: "gitlab", wantErr: "HTTP 403", fx: reactionFixture{postStatus: 403,
			postBody: `{"message":"403 Forbidden"}`, userID: 5, awards: `[{"id":3,"name":"eyes","user":{"id":5}}]`}},
	} {
		t.Run(tc.forge+"/"+tc.name, func(t *testing.T) {
			var f reactionForge
			for _, candidate := range reactionForges() {
				if candidate.forge == tc.forge {
					f = candidate
				}
			}
			tc.fx.visibility, tc.fx.state, tc.fx.current = "private", "open", head
			var requests []string
			var writes []map[string]any
			c := reactionClient(t, f, tc.fx, &requests, &writes)
			err := c.AddReaction(Reaction{Number: 7, Head: head, Content: "eyes", MaxVisibility: "private"})
			if (tc.wantErr == "" && err != nil) || (tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr))) {
				t.Fatalf("AddReaction error = %v, want %q", err, tc.wantErr)
			}
			readBack := strings.Contains(strings.Join(requests, "\n"), "GET "+f.reactionPath)
			if len(writes) != 1 || readBack != tc.wantReads {
				t.Fatalf("writes %d, award read %v; requests %v", len(writes), readBack, requests)
			}
		})
	}
}

// TestReactionGuardMatchesPublishComment drives both write verbs through every
// refusal PublishComment enforces. A verb that skipped a step would write.
func TestReactionGuardMatchesPublishComment(t *testing.T) {
	head := strings.Repeat("a", 40)
	open := reactionFixture{visibility: "private", state: "open", current: head}
	with := func(edit func(*reactionFixture)) reactionFixture {
		fx := open
		edit(&fx)
		return fx
	}
	for _, f := range reactionForges() {
		for _, tc := range []struct {
			name, head, maxVisibility, wantErr string
			fx                                 reactionFixture
		}{
			{name: "short head", head: "abc", wantErr: "full reviewed head", fx: open},
			{name: "uppercase head", head: strings.Repeat("A", 40), wantErr: "full reviewed head", fx: open},
			{name: "visibility unreadable", wantErr: "read repository visibility failed (HTTP 404)",
				fx: with(func(fx *reactionFixture) { fx.visibilityStatus = 404 })},
			{name: "visibility widened", wantErr: "visibility exceeds", fx: with(func(fx *reactionFixture) { fx.visibility = "public" })},
			{name: "internal above private scope", wantErr: "visibility exceeds", fx: with(func(fx *reactionFixture) { fx.visibility = "internal" })},
			{name: "unknown live visibility", wantErr: "visibility exceeds", maxVisibility: "public",
				fx: with(func(fx *reactionFixture) { fx.visibility = "Public" })},
			{name: "missing scope", maxVisibility: "none", wantErr: "visibility exceeds", fx: open},
			{name: "change unreadable", wantErr: "read change failed (HTTP 500)", fx: with(func(fx *reactionFixture) { fx.changeStatus = 500 })},
			{name: "change from another URL", wantErr: "mismatched or incomplete change",
				fx: with(func(fx *reactionFixture) { fx.changeURL = "https://other.example/pull/7" })},
			{name: "closed", wantErr: "no longer open", fx: with(func(fx *reactionFixture) { fx.state = "closed" })},
			{name: "merged", wantErr: "no longer open", fx: with(func(fx *reactionFixture) { fx.state = "merged" })},
			{name: "head moved", wantErr: "differs from current head", fx: with(func(fx *reactionFixture) { fx.current = strings.Repeat("b", 40) })},
		} {
			t.Run(f.forge+"/"+tc.name, func(t *testing.T) {
				reviewed := tc.head
				if reviewed == "" {
					reviewed = head
				}
				scope := tc.maxVisibility
				if scope == "" {
					scope = "private"
				}
				verbs := map[string]func(*Client) error{
					"PublishComment": func(c *Client) error {
						return c.PublishComment(Publication{Number: 7, Head: reviewed, Body: "Review started.", MaxVisibility: scope})
					},
					"AddReaction": func(c *Client) error {
						return c.AddReaction(Reaction{Number: 7, Head: reviewed, Content: "eyes", MaxVisibility: scope})
					},
				}
				reads := map[string][]string{}
				for name, verb := range verbs {
					var requests []string
					var writes []map[string]any
					err := verb(reactionClient(t, f, tc.fx, &requests, &writes))
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) || len(writes) != 0 {
						t.Fatalf("%s: error %v, want %q; writes %v", name, err, tc.wantErr, writes)
					}
					reads[name] = requests
				}
				if fmt.Sprint(reads["AddReaction"]) != fmt.Sprint(reads["PublishComment"]) {
					t.Fatalf("guard reads differ: reaction %v, comment %v", reads["AddReaction"], reads["PublishComment"])
				}
			})
		}
	}
}
