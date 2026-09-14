package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskclose_forgestub_test.go — stubRemote's deskkit.Forge implementation. Since the
// write-verbs-C migration deskclose reaches the forge through forgeForFn; the stub serves the
// seam ops from the SAME JSON fixture maps the old runGH stub used (items/pulls/comment/threads)
// and records gh-shaped argv in s.calls, so the writes()/closes() assertions read unchanged.

// --- fixture parsing ---

type stubIssueWire struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	Body        string `json:"body"`
	Labels      []struct{ Name string } `json:"labels"`
	PullRequest *struct {
		MergedAt *string `json:"merged_at"`
	} `json:"pull_request"`
}

type stubPullWire struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Merged bool   `json:"merged"`
}

type stubCommentWire struct {
	HTMLURL  string `json:"html_url"`
	IssueURL string `json:"issue_url"`
	Body     string `json:"body"`
	User     struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	} `json:"user"`
}

// stubRenderLogin mirrors the forge backends' rendering: a Bot/App author carries the
// `<slug>[bot]` suffix. So a fixture that declares type Bot with a bare login is rendered the
// way ListComments would, which is what deskclose's App/Bot exclusion keys on.
func stubRenderLogin(login, typ string) string {
	if login == "" {
		return ""
	}
	if (strings.EqualFold(typ, "Bot") || strings.EqualFold(typ, "App")) &&
		!strings.HasSuffix(login, "[bot]") && !strings.HasPrefix(login, "app/") {
		return login + "[bot]"
	}
	return login
}

func (s *stubRemote) GetIssue(fr deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	key := fr.Slug() + "#" + strconv.Itoa(n)
	s.calls = append(s.calls, []string{"api", "repos/" + fr.Slug() + "/issues/" + strconv.Itoa(n)})
	if s.failItem[key] {
		return nil, errors.New("HTTP 502: bad gateway")
	}
	j, ok := s.items[key]
	if !ok {
		return nil, deskkit.Unverifiable("HTTP 404: no such issue "+key, nil)
	}
	var w stubIssueWire
	if err := json.Unmarshal([]byte(j), &w); err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(w.Labels))
	for _, l := range w.Labels {
		labels = append(labels, l.Name)
	}
	return &deskkit.Issue{
		Number: w.Number, Title: w.Title, State: w.State, Body: w.Body,
		Labels: labels, IsPullRequest: w.PullRequest != nil,
	}, nil
}

func (s *stubRemote) GetPullRequest(fr deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	key := fr.Slug() + "#" + strconv.Itoa(n)
	s.calls = append(s.calls, []string{"api", "repos/" + fr.Slug() + "/pulls/" + strconv.Itoa(n)})
	j, ok := s.pulls[key]
	if !ok {
		return nil, deskkit.Unverifiable("HTTP 404: not a pull request "+key, nil)
	}
	var w stubPullWire
	if err := json.Unmarshal([]byte(j), &w); err != nil {
		return nil, err
	}
	return &deskkit.PullRequest{Number: w.Number, State: w.State, Merged: w.Merged}, nil
}

// commentItem parses a comment fixture's issue_url into "owner/name#N".
func commentItem(j string) (string, bool) {
	var w stubCommentWire
	if err := json.Unmarshal([]byte(j), &w); err != nil {
		return "", false
	}
	// issue_url: https://api.github.com/repos/<owner>/<name>/issues/<N>
	i := strings.Index(w.IssueURL, "/repos/")
	if i < 0 {
		return "", false
	}
	rest := w.IssueURL[i+len("/repos/"):]
	rest = strings.Replace(rest, "/issues/", "#", 1)
	return rest, true
}

func (s *stubRemote) ListComments(fr deskkit.ForgeRepo, n int) ([]deskkit.Comment, error) {
	key := fr.Slug() + "#" + strconv.Itoa(n)
	s.calls = append(s.calls, []string{"api", "repos/" + fr.Slug() + "/issues/" + strconv.Itoa(n) + "/comments"})
	if s.failThread[key] {
		return nil, errors.New("HTTP 502: bad gateway")
	}
	// A failComment on a comment that lives on THIS item makes the whole listing fail — the
	// authorizing comment could not be read, which is could-not-check.
	for cid := range s.failComment {
		if !s.failComment[cid] {
			continue
		}
		if j, ok := s.comment[cid]; ok {
			if it, ok := commentItem(j); ok && it == key {
				return nil, errors.New("HTTP 403: rate limited by the API")
			}
		}
	}
	var out []deskkit.Comment
	// Authorizing comments registered by cid, attached to this item via their issue_url. The map
	// key IS the comment's database id (the cid the permalink names).
	for cid, j := range s.comment {
		it, ok := commentItem(j)
		if !ok || it != key {
			continue
		}
		var w stubCommentWire
		if err := json.Unmarshal([]byte(j), &w); err != nil {
			return nil, err
		}
		id, _ := strconv.ParseInt(cid, 10, 64)
		out = append(out, deskkit.Comment{
			DatabaseID: id,
			Body:       w.Body,
			URL:        w.HTMLURL,
			Author:     deskkit.Account{Login: stubRenderLogin(w.User.Login, w.User.Type), ID: w.User.ID},
		})
	}
	// Proposal-thread comments (the two-role superseded reader walks these).
	for _, j := range s.threads[key] {
		var w stubCommentWire
		if err := json.Unmarshal([]byte(j), &w); err != nil {
			return nil, err
		}
		out = append(out, deskkit.Comment{
			Body:   w.Body,
			Author: deskkit.Account{Login: stubRenderLogin(w.User.Login, w.User.Type), ID: w.User.ID},
		})
	}
	return out, nil
}

// stubIsPR reports whether the fixture item is a pull request, so the recorded gh-shaped argv
// uses the `pr`/`issue` verb the old code emitted (the confirm/propose assertions key on it).
func (s *stubRemote) stubIsPR(fr deskkit.ForgeRepo, n int) bool {
	j, ok := s.items[fr.Slug()+"#"+strconv.Itoa(n)]
	if !ok {
		return false
	}
	var w stubIssueWire
	if err := json.Unmarshal([]byte(j), &w); err != nil {
		return false
	}
	return w.PullRequest != nil
}

func (s *stubRemote) PostComment(fr deskkit.ForgeRepo, n int, body string) (*deskkit.CommentRef, error) {
	s.calls = append(s.calls, []string{"issue", "comment", strconv.Itoa(n), "-R", fr.Slug(), "--body", body})
	return &deskkit.CommentRef{URL: "https://github.com/" + fr.Slug() + "/issues/" + strconv.Itoa(n) + "#issuecomment-1"}, nil
}

func (s *stubRemote) CloseIssue(fr deskkit.ForgeRepo, n int, reason string) error {
	var argv []string
	if s.stubIsPR(fr, n) {
		argv = []string{"pr", "close", strconv.Itoa(n), "-R", fr.Slug()}
	} else {
		argv = []string{"issue", "close", strconv.Itoa(n), "-R", fr.Slug()}
		if reason != "" {
			argv = append(argv, "--reason", reason)
		}
	}
	s.calls = append(s.calls, argv)
	// Reflect the close so a resumed run sees the item as already closed.
	key := fr.Slug() + "#" + strconv.Itoa(n)
	if v, ok := s.items[key]; ok {
		s.items[key] = strings.Replace(v, `"state":"open"`, `"state":"closed"`, 1)
	}
	return nil
}

func (s *stubRemote) ApplyLabels(fr deskkit.ForgeRepo, n int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	kind := "issue"
	if s.stubIsPR(fr, n) {
		kind = "pr"
	}
	if s.failApply {
		for _, l := range change.Add {
			s.calls = append(s.calls, []string{kind, "edit", strconv.Itoa(n), "-R", fr.Slug(), "--add-label", l.Name})
		}
		return nil, errors.New("could not add label")
	}
	var added []string
	for _, l := range change.Add {
		// The ensure-then-add reconcile: record the create AND the add, matching the argv the old
		// three-call sequence emitted, so writes() sees both.
		s.calls = append(s.calls, []string{"label", "create", l.Name, "-R", fr.Slug()})
		s.calls = append(s.calls, []string{kind, "edit", strconv.Itoa(n), "-R", fr.Slug(), "--add-label", l.Name})
		added = append(added, l.Name)
	}
	return &deskkit.LabelOutcome{Added: added}, nil
}
