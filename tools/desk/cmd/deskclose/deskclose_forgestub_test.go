package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	Number      int                     `json:"number"`
	Title       string                  `json:"title"`
	State       string                  `json:"state"`
	Body        string                  `json:"body"`
	Labels      []struct{ Name string } `json:"labels"`
	PullRequest *struct {
		MergedAt *string `json:"merged_at"`
	} `json:"pull_request"`
}

type stubPullWire struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Merged bool   `json:"merged"`
	Draft  bool   `json:"draft"`
	User   struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	} `json:"user"`
	Labels []struct{ Name string } `json:"labels"`
}

type stubCommentWire struct {
	HTMLURL   string `json:"html_url"`
	IssueURL  string `json:"issue_url"`
	Body      string `json:"body"`
	Minimized bool   `json:"minimized"`
	User      struct {
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

// stubKey resolves the fixture key for the object of one kind at a number.
//
// A fixture that models a project with SEPARATE number sequences (the GitLab shape)
// registers its change under "repo!N" alongside the issue at "repo#N"; a fixture that models
// ONE sequence (the GitHub shape) registers only "repo#N" and both kinds resolve to it. That
// is what lets one stub serve both forges' numbering without modelling two APIs.
func (s *stubRemote) stubKey(slug string, n int, kind deskkit.TargetKind) string {
	if kind == deskkit.TargetChange {
		if k := slug + "!" + strconv.Itoa(n); s.items[k] != "" {
			return k
		}
	}
	return slug + "#" + strconv.Itoa(n)
}

// stubCollides reports whether the fixture carries BOTH kinds at one number — the case the
// untyped read cannot resolve.
func (s *stubRemote) stubCollides(slug string, n int) bool {
	return s.items[slug+"#"+strconv.Itoa(n)] != "" && s.items[slug+"!"+strconv.Itoa(n)] != ""
}

// stubIssueAt decodes one fixture object into the forge-neutral Issue.
func (s *stubRemote) stubIssueAt(key string) (*deskkit.Issue, error) {
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

// GetIssue is the UNTYPED read, and it reproduces the one behaviour that matters here: a
// number that names two different objects has no correct answer, so it is a could-not-check
// refusal worded as the GitLab backend words it — never one of the two picked.
func (s *stubRemote) GetIssue(fr deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	slug := fr.Slug()
	s.calls = append(s.calls, []string{"api", "repos/" + slug + "/issues/" + strconv.Itoa(n)})
	s.untypedGets = append(s.untypedGets, fmt.Sprintf("%s#%d", slug, n))
	if s.stubCollides(slug, n) {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s carries BOTH issue #%d and merge request !%d — GitLab numbers issues and "+
				"merge requests in separate sequences, so a bare number cannot be resolved to one kind; "+
				"use the typed operation for the kind you mean", slug, n, n), nil)
	}
	return s.stubIssueAt(slug + "#" + strconv.Itoa(n))
}

// GetIssueTyped records the STATED kind on the call trail and then answers from the same
// fixture map, so a test can assert which kind deskclose asked for without the fixture having
// to model two number sequences. A stated kind that contradicts the fixture's own kind is an
// error, the way both real backends report it — a typed read is an assertion, not a hint.
func (s *stubRemote) GetIssueTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	s.typedGets = append(s.typedGets, fmt.Sprintf("%s#%d:%s", fr.Slug(), n, string(kind)))
	if kind != deskkit.TargetIssue && kind != deskkit.TargetChange {
		return nil, deskkit.Refused(fmt.Sprintf("refused: unknown target kind %q", string(kind)))
	}
	key := s.stubKey(fr.Slug(), n, kind)
	s.calls = append(s.calls, []string{"api", "repos/" + fr.Slug() + "/issues/" + strconv.Itoa(n)})
	iss, err := s.stubIssueAt(key)
	if err != nil {
		return nil, err
	}
	switch {
	case iss.IsPullRequest && kind == deskkit.TargetIssue:
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d is a pull request, not an issue", fr.Slug(), n), nil)
	case !iss.IsPullRequest && kind == deskkit.TargetChange:
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d is an issue, not a pull request", fr.Slug(), n), nil)
	case kind != deskkit.TargetIssue && kind != deskkit.TargetChange:
		return nil, deskkit.Refused(fmt.Sprintf("refused: unknown target kind %q", string(kind)))
	}
	return iss, nil
}

// ListCommentsTyped records the kind and serves the same thread fixture ListComments does.
//
// KIND-ACCURATE, matching production (forge_github.go: listCommentsGQL): a number that names
// the OTHER kind resolves to a null noteable, reported as could-not-check — never as an empty
// thread. That check applies only when the item at the resolved key IS modelled: a number with
// no modelled issue/PR object (a bare sign-off / manifest comment host) is not a mismatch, it is
// simply not modelled either way — the untyped ListComments stub already treats it the same way.
func (s *stubRemote) ListCommentsTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind) ([]deskkit.Comment, error) {
	if kind != deskkit.TargetIssue && kind != deskkit.TargetChange {
		return nil, deskkit.Refused(fmt.Sprintf("refused: unknown target kind %q", string(kind)))
	}
	key := s.stubKey(fr.Slug(), n, kind)
	if j, ok := s.items[key]; ok {
		var w stubIssueWire
		if err := json.Unmarshal([]byte(j), &w); err == nil {
			isPR := w.PullRequest != nil
			switch {
			case isPR && kind == deskkit.TargetIssue:
				return nil, deskkit.Unverifiable(fmt.Sprintf(
					"could-not-check: %s carries no issue at number %d, so its comment thread could not be read",
					fr.Slug(), n), nil)
			case !isPR && kind == deskkit.TargetChange:
				return nil, deskkit.Unverifiable(fmt.Sprintf(
					"could-not-check: %s carries no pull request at number %d, so its comment thread could not be read",
					fr.Slug(), n), nil)
			}
		}
	}
	s.typedThreads = append(s.typedThreads, fmt.Sprintf("%s#%d:%s", fr.Slug(), n, string(kind)))
	return s.listCommentsAt(fr, n, key)
}

func (s *stubRemote) GetPullRequest(fr deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	// A change read is kind-implied: there is no ambiguity to resolve, so the change fixture
	// wins wherever a project registers one under the separate-sequence key.
	key := s.stubKey(fr.Slug(), n, deskkit.TargetChange)
	if _, ok := s.pulls[key]; !ok {
		key = fr.Slug() + "#" + strconv.Itoa(n)
	}
	s.calls = append(s.calls, []string{"api", "repos/" + fr.Slug() + "/pulls/" + strconv.Itoa(n)})
	j, ok := s.pulls[key]
	if !ok {
		return nil, deskkit.Unverifiable("HTTP 404: not a pull request "+key, nil)
	}
	var w stubPullWire
	if err := json.Unmarshal([]byte(j), &w); err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(w.Labels))
	for _, l := range w.Labels {
		labels = append(labels, l.Name)
	}
	return &deskkit.PullRequest{
		Number: w.Number, Title: w.Title, State: w.State, Merged: w.Merged, Draft: w.Draft,
		Author: deskkit.Account{Login: stubRenderLogin(w.User.Login, w.User.Type), ID: w.User.ID},
		Labels: labels,
	}, nil
}

// ReopenIssue records the reopen as a gh-shaped `issue reopen` argv — counted by writes(),
// so a refusal path that reopened anything shows up as a write — and flips the fixture back
// to open, so the re-close that follows in the same invocation sees an open item.
func (s *stubRemote) ReopenIssue(fr deskkit.ForgeRepo, n int) error {
	key := fr.Slug() + "#" + strconv.Itoa(n)
	if s.stubIsPRAt(key) {
		return fmt.Errorf("reopen addressed %s, which is a pull request — ReopenIssue is the issue sequence only", key)
	}
	s.calls = append(s.calls, []string{"issue", "reopen", strconv.Itoa(n), "-R", fr.Slug()})
	if v, ok := s.items[key]; ok {
		s.items[key] = strings.Replace(v, `"state":"closed"`, `"state":"open"`, 1)
	}
	return nil
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
	// KIND-ACCURATE, matching production: GitHub's untyped ListComments uses the
	// pullRequest(number:) selection (forge_github.go: ListComments → listCommentsGQL(...,
	// TargetChange)), so at a number that names an ISSUE the noteable is null and the read is
	// could-not-check — never the issue's own thread. Reproduce that here: a MODELLED
	// issue-item at n comes back could-not-check through the untyped read, so reaching an
	// issue's thread REQUIRES ListCommentsTyped(TargetIssue). A change, or a number with no
	// modelled item (a bare sign-off / manifest host), serves the thread as before — which is
	// why the ruling and manifest gates keep working while the human-decided lane, which reads
	// an issue's own thread, does not until it routes through the typed call.
	if _, ok := s.items[key]; ok && !s.stubIsPRAt(key) {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s names an issue; the untyped ListComments uses the change/pullRequest "+
				"selection, so an issue's own thread is unreachable through it — use ListCommentsTyped(issue)",
			key), nil)
	}
	return s.listCommentsAt(fr, n, key)
}

// listCommentsAt serves the thread registered under one fixture key. The key — not the
// number — is what distinguishes an issue's thread from the thread of a change sharing its
// number, which is the whole property the typed read exists to hold.
func (s *stubRemote) listCommentsAt(fr deskkit.ForgeRepo, n int, key string) ([]deskkit.Comment, error) {
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
			Minimized:  w.Minimized,
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
	return s.stubIsPRAt(fr.Slug() + "#" + strconv.Itoa(n))
}

func (s *stubRemote) stubIsPRAt(key string) bool {
	j, ok := s.items[key]
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

// PostCommentTyped records the kind on the write trail and then emits the same gh-shaped argv
// PostComment does, so the existing writes() assertions read unchanged.
func (s *stubRemote) PostCommentTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind, body string) (*deskkit.CommentRef, error) {
	if kind != deskkit.TargetIssue && kind != deskkit.TargetChange {
		return nil, deskkit.Refused(fmt.Sprintf("refused: unknown target kind %q", string(kind)))
	}
	s.typedComments = append(s.typedComments, fmt.Sprintf("%s#%d:%s", fr.Slug(), n, string(kind)))
	return s.PostComment(fr, n, body)
}

// CloseIssueTyped is the close deskclose actually calls. The stub asserts the stated kind
// against the fixture's own kind, which is what makes a close routed at the wrong kind of
// object a TEST FAILURE here rather than a silently wrong write on a real project.
func (s *stubRemote) CloseIssueTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind, reason string) error {
	key := s.stubKey(fr.Slug(), n, kind)
	want := deskkit.TargetIssue
	if s.stubIsPRAt(key) {
		want = deskkit.TargetChange
	}
	if kind != want {
		return fmt.Errorf("close targeted kind %s on %s#%d, which is a %s", kind, fr.Slug(), n, want)
	}
	if kind == deskkit.TargetChange && strings.TrimSpace(reason) != "" {
		return fmt.Errorf("close of the change %s!%d carried state reason %q, which no forge records",
			fr.Slug(), n, reason)
	}
	s.typedCloses = append(s.typedCloses, fmt.Sprintf("%s#%d:%s", fr.Slug(), n, string(kind)))
	return s.closeAt(fr, n, key, reason)
}

func (s *stubRemote) CloseIssue(fr deskkit.ForgeRepo, n int, reason string) error {
	return s.closeAt(fr, n, fr.Slug()+"#"+strconv.Itoa(n), reason)
}

func (s *stubRemote) closeAt(fr deskkit.ForgeRepo, n int, key, reason string) error {
	var argv []string
	if s.stubIsPRAt(key) {
		argv = []string{"pr", "close", strconv.Itoa(n), "-R", fr.Slug()}
	} else {
		argv = []string{"issue", "close", strconv.Itoa(n), "-R", fr.Slug()}
		if reason != "" {
			argv = append(argv, "--reason", reason)
		}
	}
	s.calls = append(s.calls, argv)
	// Reflect the close so a resumed run sees the item as already closed.
	if v, ok := s.items[key]; ok {
		s.items[key] = strings.Replace(v, `"state":"open"`, `"state":"closed"`, 1)
	}
	return nil
}

func (s *stubRemote) ApplyLabels(fr deskkit.ForgeRepo, n int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	// The kind recorded on the write comes from the CALLER's stated target, not from the
	// stub's own item lookup — so a caller that labels a PR as an issue (or leaves the target
	// unset, which the real backends refuse) shows up as the wrong argv / an error here.
	var kind string
	switch change.Target {
	case deskkit.TargetIssue:
		kind = "issue"
	case deskkit.TargetChange:
		kind = "pr"
	default:
		return nil, errors.New("refusing to apply labels with no target kind")
	}
	if want := "issue"; s.stubIsPRAt(s.stubKey(fr.Slug(), n, change.Target)) {
		want = "pr"
		if kind != want {
			return nil, fmt.Errorf("label target %s on %s#%d, which is a %s", kind, fr.Slug(), n, want)
		}
	} else if kind != want {
		return nil, fmt.Errorf("label target %s on %s#%d, which is an %s", kind, fr.Slug(), n, want)
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
