package deskkit

import (
	"net/http"
	"strings"
	"testing"
)

// forge_gitlab_trust_test.go — the GitLab trust-events brief: the Verify rows for the GitLab trust read and
// the commit author-login resolution. The golden corpus (forge_gitlab_test.go) pins the WIRE
// footprint of each operation; these tests pin the VERDICT: that deskkit.Blessed draws the
// same conclusion from a GitLab payload as from the GitHub payload on equivalent content —
// the single-point-of-failure control the brief names, because a mapping that fail-opens
// (a body edit that does not move BodyEdited, a bot login the id guard cannot pin, a
// truncated thread reported Complete) survives every happy-path golden.

const (
	// The GitLab reviewer service account the parity roster binds. Its username is its ONE
	// rendering (BotIdentity.AcceptedLogins); its numeric user id is what the strict content
	// check pins on.
	glTrustBotLogin       = "assay-reviewer-bot"
	glTrustBotID    int64 = 41987965
	// The GitHub twin on the same roster, so a parity case can put "a trusted bot's comment"
	// on both forges: GitHub renders it decorated, GitLab bare.
	ghTrustBotSlug       = "assay-worker-app"
	ghTrustBotID   int64 = 300000006

	trustT1 = "2026-09-01T10:00:00Z" // the untrusted author's comment
	trustT2 = "2026-09-02T10:00:00Z" // ada's blessing
	trustT3 = "2026-09-03T10:00:00Z" // after the blessing
)

// withTrustParityRoster installs a roster carrying the blessing authority (ada:2001, the
// fixture value), one GitLab-qualified bot and one GitHub-qualified bot, through the REAL
// loader.
func withTrustParityRoster(t *testing.T) {
	t.Helper()
	withRoster(t, forgeRoster(
		"reviewer=gitlab:"+glTrustBotLogin+":41987965,worker=github:"+ghTrustBotSlug+":300000006",
		map[string]string{EnvTrustedLogins: "ada:2001"}))
}

func glTrustEvents(t *testing.T, s *glServer, gql map[string]any) *TrustPayload {
	t.Helper()
	s.gql = gql
	tp, err := s.forge().PRTrustEvents(glRepo, 7)
	if err != nil {
		t.Fatalf("PRTrustEvents: %v", err)
	}
	return tp
}

// ghTrustPayload builds the GitHub payload for equivalent content — the PRTrustQuery
// envelope the GitHub backend decodes — and reduces it through the same reader the GitHub
// backend uses (trustFromEnvelope), so the parity comparison is backend-reader vs
// backend-reader, not a hand-rolled expectation.
func ghTrustPayload(t *testing.T, bodyEdited string, comments []gqlComment, more bool) TrustPayload {
	t.Helper()
	var env gqlEnvelope
	env.Data.Repository.PullRequest = &gqlItem{
		LastEditedAt: bodyEdited,
		Comments:     gqlConn{PageInfo: gqlPageInfo{HasNextPage: more}, Nodes: comments},
	}
	tp, err := trustFromEnvelope(env, true)
	if err != nil {
		t.Fatalf("GitHub reader: %v", err)
	}
	return tp
}

func ghUser(login string, id int64, created, edited string) gqlComment {
	return gqlComment{CreatedAt: created, LastEditedAt: edited,
		Author: &gqlActor{Login: login, Typename: "User", DatabaseID: id}}
}

func ghBot(slug string, id int64, created, edited string) gqlComment {
	return gqlComment{CreatedAt: created, LastEditedAt: edited,
		Author: &gqlActor{Login: slug, Typename: "Bot", DatabaseID: id}}
}

// TestForgeGitlabTrustEvents — Verify row 2. PRTrustEvents on a GitLab MR fixture returns a
// REAL TrustPayload — events carrying a rendered login AND a non-zero numeric AuthorID, a
// BodyEdited time, and Complete — not a could-not-check stub; the IssueTrustEvents twin
// returns a real payload on an issue fixture.
func TestForgeGitlabTrustEvents(t *testing.T) {
	t.Run("merge_request", func(t *testing.T) {
		s := newGLServer(t)
		tp := glTrustEvents(t, s, glGQLTrust("mergeRequest",
			[]map[string]any{
				glGQLNoteNode("external-user", 501, false, trustT1, ""),
				glGQLNoteNode(glTrustBotLogin, glTrustBotID, true, trustT3, "2026-09-03T11:00:00Z"),
			},
			[]map[string]any{
				glGQLActivityNode("changed the description", "2026-08-31T09:00:00Z"),
				glGQLActivityNode("added ~bug label", trustT3),
			}, false, false))
		if !tp.Complete {
			t.Fatal("a single-page read must be Complete")
		}
		if len(tp.Events) != 2 {
			t.Fatalf("events = %d, want 2 (both non-system notes): %+v", len(tp.Events), tp.Events)
		}
		if e := tp.Events[0]; e.Author != "external-user" || e.AuthorID != 501 || e.CreatedAt != ts(t, trustT1) || !e.EditedAt.IsZero() {
			t.Fatalf("event 0 = %+v, want external-user/501 created %s never edited", e, trustT1)
		}
		if e := tp.Events[1]; e.Author != glTrustBotLogin || e.AuthorID != glTrustBotID || e.EditedAt.IsZero() {
			t.Fatalf("event 1 = %+v, want the bot's BARE username, its numeric id and its edit time", e)
		}
		if strings.HasSuffix(tp.Events[1].Author, "[bot]") {
			t.Fatalf("a GitLab service account must render as its bare username, never GitHub bot clothing: %q", tp.Events[1].Author)
		}
		if want := ts(t, "2026-08-31T09:00:00Z"); !tp.BodyEdited.Equal(want) {
			t.Fatalf("BodyEdited = %s, want the description-change note's time %s (the label note must not move it)", tp.BodyEdited, want)
		}
	})
	t.Run("issue_twin", func(t *testing.T) {
		s := newGLServer(t)
		s.gql = glGQLTrust("issue",
			[]map[string]any{glGQLNoteNode("ada", fixtureBlessID, false, trustT2, "")},
			[]map[string]any{glGQLActivityNode("changed the description", trustT1)}, false, false)
		tp, err := s.forge().IssueTrustEvents(glRepo, 12)
		if err != nil {
			t.Fatalf("IssueTrustEvents: %v", err)
		}
		if !tp.Complete || len(tp.Events) != 1 || tp.Events[0].AuthorID != fixtureBlessID || !tp.BodyEdited.Equal(ts(t, trustT1)) {
			t.Fatalf("issue payload = %+v, want one ada event with id %d, body edited %s, Complete", tp, fixtureBlessID, trustT1)
		}
		if got := string(s.requests[0].Body); !strings.Contains(got, "issue(iid:$iid)") {
			t.Fatalf("the issue twin must query the issue noteable; body=%s", got)
		}
	})
}

// TestForgeGitlabTrustParity — Verify row 3. deskkit.Blessed on the GitLab payload draws the
// SAME verdict as on the GitHub payload for equivalent content: a note edited by an
// untrusted author AFTER the blessing re-quarantines; a label/assignee event does NOT
// (no spurious re-quarantine from the noteable's updatedAt); a description edit after the
// blessing does; a trusted bot's later comment does not — and only with the RIGHT numeric id.
func TestForgeGitlabTrustParity(t *testing.T) {
	withTrustParityRoster(t)

	// Every case is one piece of content expressed twice: as GitLab notes (the backend under
	// test reads them over the fake instance) and as the GitHub GraphQL envelope (reduced by
	// the GitHub backend's reader). The verdict must agree, and `want` says which way.
	cases := []struct {
		name     string
		glNotes  []map[string]any // non-system notes
		glAct    []map[string]any // system notes
		ghBody   string           // GitHub lastEditedAt on the body
		ghEvents []gqlComment
		want     bool
	}{
		{
			name: "blessed: ada commented after the untrusted author, body never edited",
			glNotes: []map[string]any{
				glGQLNoteNode("external-user", 501, false, trustT1, ""),
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
			},
			ghEvents: []gqlComment{ghUser("external-user", 501, trustT1, ""), ghUser("ada", fixtureBlessID, trustT2, "")},
			want:     true,
		},
		{
			name: "re-quarantined: the untrusted note was EDITED after the blessing",
			glNotes: []map[string]any{
				glGQLNoteNode("external-user", 501, false, trustT1, trustT3),
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
			},
			ghEvents: []gqlComment{ghUser("external-user", 501, trustT1, trustT3), ghUser("ada", fixtureBlessID, trustT2, "")},
			want:     false,
		},
		{
			name: "re-quarantined: an untrusted note was ADDED after the blessing",
			glNotes: []map[string]any{
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
				glGQLNoteNode("external-user", 501, false, trustT3, ""),
			},
			ghEvents: []gqlComment{ghUser("ada", fixtureBlessID, trustT2, ""), ghUser("external-user", 501, trustT3, "")},
			want:     false,
		},
		{
			// The spurious-re-quarantine hole: a label and an assignee event after the blessing
			// move the noteable's updatedAt (planted at 2026-09-09 by glGQLTrust) and add system
			// notes — neither is a content edit. GitHub's lastEditedAt stays null for the same.
			name: "stays blessed: label + assignee events after the blessing are not content edits",
			glNotes: []map[string]any{
				glGQLNoteNode("external-user", 501, false, trustT1, ""),
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
			},
			glAct: []map[string]any{
				glGQLActivityNode("added ~bug label", trustT3),
				glGQLActivityNode("assigned to @ada", "2026-09-04T10:00:00Z"),
				glGQLActivityNode("changed title from **a** to **b**", "2026-09-05T10:00:00Z"),
			},
			ghEvents: []gqlComment{ghUser("external-user", 501, trustT1, ""), ghUser("ada", fixtureBlessID, trustT2, "")},
			want:     true,
		},
		{
			name: "re-quarantined: the DESCRIPTION was edited after the blessing",
			glNotes: []map[string]any{
				glGQLNoteNode("external-user", 501, false, trustT1, ""),
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
			},
			glAct:    []map[string]any{glGQLActivityNode("changed the description", trustT3)},
			ghBody:   trustT3,
			ghEvents: []gqlComment{ghUser("external-user", 501, trustT1, ""), ghUser("ada", fixtureBlessID, trustT2, "")},
			want:     false,
		},
		{
			// A description edit BEFORE the blessing is covered by it (ada saw the edit); only
			// the LATEST edit counts, and the latest one here predates the blessing.
			name: "stays blessed: description edited before the blessing",
			glNotes: []map[string]any{
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
			},
			glAct:    []map[string]any{glGQLActivityNode("changed the description", trustT1), glGQLActivityNode("changed the description", "2026-08-30T10:00:00Z")},
			ghBody:   trustT1,
			ghEvents: []gqlComment{ghUser("ada", fixtureBlessID, trustT2, "")},
			want:     true,
		},
		{
			// A TRUSTED bot's later comment is trusted content in its own right on both forges.
			// On GitLab that needs the strict content check to pin the bot's numeric id off the
			// forge-qualified roster entry (its login has no [bot] decoration to key on).
			name: "stays blessed: a trusted bot commented after the blessing",
			glNotes: []map[string]any{
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
				glGQLNoteNode(glTrustBotLogin, glTrustBotID, true, trustT3, ""),
			},
			ghEvents: []gqlComment{ghUser("ada", fixtureBlessID, trustT2, ""), ghBot(ghTrustBotSlug, ghTrustBotID, trustT3, "")},
			want:     true,
		},
		{
			// The lower layer: the bot LOGIN is right but the numeric id is not — a recycled or
			// squatted username. Untrusted content after the blessing on both forges.
			name: "re-quarantined: the trusted bot login carries the WRONG numeric id",
			glNotes: []map[string]any{
				glGQLNoteNode("ada", fixtureBlessID, false, trustT2, ""),
				glGQLNoteNode(glTrustBotLogin, glTrustBotID+1, true, trustT3, ""),
			},
			ghEvents: []gqlComment{ghUser("ada", fixtureBlessID, trustT2, ""), ghBot(ghTrustBotSlug, ghTrustBotID+1, trustT3, "")},
			want:     false,
		},
		{
			name: "not blessed: the blessing login carries the wrong numeric id",
			glNotes: []map[string]any{
				glGQLNoteNode("ada", fixtureBlessID+1, false, trustT2, ""),
			},
			ghEvents: []gqlComment{ghUser("ada", fixtureBlessID+1, trustT2, "")},
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newGLServer(t)
			gl := glTrustEvents(t, s, glGQLTrust("mergeRequest", tc.glNotes, orEmpty(tc.glAct), false, false))
			gh := ghTrustPayload(t, tc.ghBody, tc.ghEvents, false)
			if !gl.Complete || !gh.Complete {
				t.Fatalf("both single-page reads must be Complete (gl=%v gh=%v)", gl.Complete, gh.Complete)
			}
			glVerdict := Blessed(gl.BodyEdited, gl.Events)
			ghVerdict := Blessed(gh.BodyEdited, gh.Events)
			if glVerdict != ghVerdict {
				t.Fatalf("PARITY BREAK: GitLab verdict %v, GitHub verdict %v\n gitlab payload: %+v\n github payload: %+v", glVerdict, ghVerdict, gl, gh)
			}
			if glVerdict != tc.want {
				t.Fatalf("both backends agree on %v, want %v\n gitlab payload: %+v", glVerdict, tc.want, gl)
			}
		})
	}

	t.Run("incomplete thread is never blessed on either forge", func(t *testing.T) {
		s := newGLServer(t)
		gl := glTrustEvents(t, s, glGQLTrust("mergeRequest",
			[]map[string]any{glGQLNoteNode("ada", fixtureBlessID, false, trustT2, "")}, []map[string]any{}, true, false))
		gh := ghTrustPayload(t, "", []gqlComment{ghUser("ada", fixtureBlessID, trustT2, "")}, true)
		if gl.Complete || gh.Complete {
			t.Fatalf("an overflowed comment page must be INCOMPLETE on both forges (gl=%v gh=%v)", gl.Complete, gh.Complete)
		}
	})
	t.Run("overflowed activity with no description-change seen is incomplete", func(t *testing.T) {
		s := newGLServer(t)
		gl := glTrustEvents(t, s, glGQLTrust("mergeRequest",
			[]map[string]any{glGQLNoteNode("ada", fixtureBlessID, false, trustT2, "")},
			[]map[string]any{glGQLActivityNode("added ~bug label", trustT3)}, false, true))
		if gl.Complete {
			t.Fatal("the latest description edit may be older than the activity page: must be INCOMPLETE, never assumed unedited")
		}
	})
	t.Run("overflowed activity WITH a description-change seen is complete", func(t *testing.T) {
		s := newGLServer(t)
		gl := glTrustEvents(t, s, glGQLTrust("mergeRequest",
			[]map[string]any{glGQLNoteNode("ada", fixtureBlessID, false, trustT2, "")},
			[]map[string]any{glGQLActivityNode("changed the description", trustT1)}, false, true))
		if !gl.Complete || !gl.BodyEdited.Equal(ts(t, trustT1)) {
			t.Fatalf("the newest page carries the LATEST edit by construction: want Complete with BodyEdited %s, got %+v", trustT1, gl)
		}
	})
}

func orEmpty(m []map[string]any) []map[string]any {
	if m == nil {
		return []map[string]any{}
	}
	return m
}

// TestForgeGitlabGetCommitAuthorLogin — Verify row 4. GetCommit on a commit whose author and
// committer addresses resolve to instance accounts returns NON-EMPTY logins; a fixture whose
// address resolves to no account leaves the field EMPTY — per-field could-not-check, never a
// login built from the address or taken from a fuzzy search hit.
func TestForgeGitlabGetCommitAuthorLogin(t *testing.T) {
	commit := func(author, committer string) map[string]any {
		return map[string]any{
			"id": "abc123", "committed_date": "2026-09-01T10:00:00Z",
			"author_email": author, "committer_email": committer,
		}
	}
	t.Run("both resolve", func(t *testing.T) {
		s := newGLServer(t)
		s.commit = commit("a@example.com", "c@example.com")
		s.userSearch["a@example.com"] = []map[string]any{{"id": 11, "username": "a-dev", "public_email": "A@Example.com"}}
		s.userSearch["c@example.com"] = []map[string]any{{"id": 13, "username": "c-dev"}} // list shape without email
		s.users["13"] = map[string]any{"id": 13, "username": "c-dev", "public_email": "c@example.com"}
		rc, err := s.forge().GetCommit(glRepo, "abc123")
		if err != nil {
			t.Fatalf("GetCommit: %v", err)
		}
		if rc.AuthorLogin != "a-dev" || rc.CommitterLogin != "c-dev" {
			t.Fatalf("logins = %q/%q, want a-dev/c-dev (exact email match, case-insensitive; detail read for the email-less list entry)", rc.AuthorLogin, rc.CommitterLogin)
		}
	})
	t.Run("no account resolves", func(t *testing.T) {
		s := newGLServer(t)
		s.commit = commit("service_account_group_9619193_x@noreply.gitlab.example", "c@example.com")
		// A fuzzy hit: the search matched the name, but the account's email is a different address.
		s.userSearch["c@example.com"] = []map[string]any{{"id": 13, "username": "c-dev", "name": "c@example.com", "public_email": "other@example.com"}}
		rc, err := s.forge().GetCommit(glRepo, "abc123")
		if err != nil {
			t.Fatalf("GetCommit: %v", err)
		}
		if rc.AuthorLogin != "" || rc.CommitterLogin != "" {
			t.Fatalf("logins = %q/%q, want both EMPTY (unresolved address; fuzzy hit is not an attribution)", rc.AuthorLogin, rc.CommitterLogin)
		}
		if rc.CommittedDate != "2026-09-01T10:00:00Z" {
			t.Fatalf("the committed date must still be served: %q", rc.CommittedDate)
		}
	})
	t.Run("two accounts claiming one address is ambiguous, not a coin flip", func(t *testing.T) {
		s := newGLServer(t)
		s.commit = commit("a@example.com", "")
		s.userSearch["a@example.com"] = []map[string]any{
			{"id": 11, "username": "a-dev", "public_email": "a@example.com"},
			{"id": 12, "username": "a-dev-2", "public_email": "a@example.com"},
		}
		rc, err := s.forge().GetCommit(glRepo, "abc123")
		if err != nil {
			t.Fatalf("GetCommit: %v", err)
		}
		if rc.AuthorLogin != "" || rc.CommitterLogin != "" {
			t.Fatalf("logins = %q/%q, want EMPTY on an ambiguous match and on an empty address", rc.AuthorLogin, rc.CommitterLogin)
		}
	})
	t.Run("resolution is memoised per address", func(t *testing.T) {
		s := newGLServer(t)
		s.commit = commit("a@example.com", "a@example.com")
		s.userSearch["a@example.com"] = []map[string]any{{"id": 11, "username": "a-dev", "public_email": "a@example.com"}}
		f := s.forge()
		if _, err := f.GetCommit(glRepo, "abc123"); err != nil {
			t.Fatal(err)
		}
		if _, err := f.GetCommit(glRepo, "abc123"); err != nil {
			t.Fatal(err)
		}
		searches := 0
		for _, r := range s.requests {
			if lUserList.MatchString(r.Path) {
				searches++
			}
		}
		if searches != 1 {
			t.Fatalf("users searches = %d, want 1 (one address, author and committer, two reads)", searches)
		}
	})
}

// TestForgeGitlabTrustReadTierErrors — Verify row 5. A permission/transport failure on the
// trust or commit read surfaces could-not-check, DISTINCT from an empty read (no notes; an
// unresolved address) and from a real payload; a read failure is never a clean empty trust
// set.
func TestForgeGitlabTrustReadTierErrors(t *testing.T) {
	mustCouldNotCheck := func(t *testing.T, err error, want string) {
		t.Helper()
		if err == nil {
			t.Fatal("expected a could-not-check error, got a payload — a read that failed must never read as a clean empty trust set")
		}
		if !strings.Contains(err.Error(), "could-not-check") || !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want a could-not-check naming %q", err, want)
		}
	}
	t.Run("graphql 403 is a tier/permission could-not-check", func(t *testing.T) {
		s := newGLServer(t)
		s.forceStatus["/api/graphql"] = http.StatusForbidden
		_, err := s.forge().PRTrustEvents(glRepo, 7)
		mustCouldNotCheck(t, err, "HTTP 403")
	})
	t.Run("graphql 401 is a credential could-not-check", func(t *testing.T) {
		s := newGLServer(t)
		s.forceStatus["/api/graphql"] = http.StatusUnauthorized
		_, err := s.forge().IssueTrustEvents(glRepo, 7)
		mustCouldNotCheck(t, err, "HTTP 401")
	})
	t.Run("graphql-level errors are could-not-check", func(t *testing.T) {
		s := newGLServer(t)
		s.gql = map[string]any{"data": nil, "errors": []map[string]any{{"message": "insufficient scope"}}}
		_, err := s.forge().PRTrustEvents(glRepo, 7)
		mustCouldNotCheck(t, err, "insufficient scope")
	})
	t.Run("invisible project is a not-found could-not-check", func(t *testing.T) {
		s := newGLServer(t)
		s.gql = map[string]any{"data": map[string]any{"project": nil}}
		_, err := s.forge().PRTrustEvents(glRepo, 7)
		mustCouldNotCheck(t, err, "returned no mergeRequest")
		if !IsForgeNotFound(err) {
			t.Fatalf("a null project classifies as not-visible: %v", err)
		}
	})
	t.Run("an unparseable actor id is could-not-check, never a defaulted 0", func(t *testing.T) {
		s := newGLServer(t)
		s.gql = glGQLTrust("mergeRequest", []map[string]any{{
			"createdAt": trustT2, "lastEditedAt": nil,
			"author": map[string]any{"id": "gid://gitlab/User/not-a-number", "username": "ada", "bot": false},
		}}, []map[string]any{}, false, false)
		_, err := s.forge().PRTrustEvents(glRepo, 7)
		if err == nil || !strings.Contains(err.Error(), "not a global user id") {
			t.Fatalf("want a refusal naming the bad id, got %v", err)
		}
	})
	t.Run("no notes at all is a REAL empty payload", func(t *testing.T) {
		s := newGLServer(t)
		tp, err := s.forge().PRTrustEvents(glRepo, 7)
		if err == nil {
			t.Fatal("no canned response means the fake instance answers 404: must be could-not-check")
		}
		s.gql = glGQLTrust("mergeRequest", []map[string]any{}, []map[string]any{}, false, false)
		tp, err = s.forge().PRTrustEvents(glRepo, 7)
		if err != nil {
			t.Fatalf("an empty thread is a real read: %v", err)
		}
		if len(tp.Events) != 0 || !tp.Complete || !tp.BodyEdited.IsZero() {
			t.Fatalf("empty payload = %+v, want zero events, Complete, never edited", tp)
		}
	})
	t.Run("commit read 403 is could-not-check for the whole read", func(t *testing.T) {
		s := newGLServer(t)
		s.forceStatus["/repository/commits/abc123"] = http.StatusForbidden
		_, err := s.forge().GetCommit(glRepo, "abc123")
		mustCouldNotCheck(t, err, "HTTP 403")
	})
	t.Run("users lookup 403 leaves the login EMPTY but serves the commit", func(t *testing.T) {
		s := newGLServer(t)
		s.commit = map[string]any{"id": "abc123", "committed_date": "2026-09-01T10:00:00Z", "author_email": "a@example.com"}
		s.forceStatus["/users"] = http.StatusForbidden
		rc, err := s.forge().GetCommit(glRepo, "abc123")
		if err != nil {
			t.Fatalf("a users-route failure must not blank the committed date: %v", err)
		}
		if rc.AuthorLogin != "" || rc.CommittedDate == "" {
			t.Fatalf("got %+v, want an EMPTY login (UNKNOWN attribution) with the date served", rc)
		}
	})
}

// TestGitLabBotIdentityPinsNumericID — the trust.go half of the parity: a GitLab-qualified
// roster bot resolves its pinned USER id from its BARE username (its one rendering), so the
// strict content check accepts it with the right id and refuses it with a wrong one; a
// GitHub App's bare slug stays untrusted on every path (username-squatting fail-close).
func TestGitLabBotIdentityPinsNumericID(t *testing.T) {
	withTrustParityRoster(t)
	if id, ok := expectedID(glTrustBotLogin); !ok || id != glTrustBotID {
		t.Fatalf("expectedID(%q) = (%d,%v), want (%d,true)", glTrustBotLogin, id, ok, glTrustBotID)
	}
	if !trustedContentAuthor(glTrustBotLogin, glTrustBotID) {
		t.Fatal("a GitLab bot with its pinned id is trusted content")
	}
	if trustedContentAuthor(glTrustBotLogin, glTrustBotID+1) {
		t.Fatal("a GitLab bot login with the wrong id must be untrusted (recycled-login defence)")
	}
	if trustedContentAuthor(glTrustBotLogin, 0) {
		t.Fatal("a GitLab bot event with no id must fail closed: its login has no [bot] decoration to trust on alone")
	}
	if !TrustedAuthorID(glTrustBotLogin, glTrustBotID) || TrustedAuthorID(glTrustBotLogin, glTrustBotID+1) {
		t.Fatal("TrustedAuthorID must pin the GitLab bot's id the same way")
	}
	if _, ok := expectedID(ghTrustBotSlug); ok {
		t.Fatalf("a GitHub App's BARE slug %q must not resolve an id", ghTrustBotSlug)
	}
	if id, ok := expectedID(ghTrustBotSlug + "[bot]"); !ok || id != ghTrustBotID {
		t.Fatalf("the GitHub twin's decorated rendering must still pin: (%d,%v)", id, ok)
	}
	if TrustedAuthor(glTrustBotLogin + "[bot]") {
		t.Fatal("a GitLab username in GitHub bot clothing is not an accepted rendering")
	}
}
