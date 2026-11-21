package main

// GitLab ruling links (#7252): a ruling recorded as a GitLab issue note is
// parsed on the configured host only, bound to the record's own project, and
// read through the stub deskread (autoflip_gitlab_test.go's glWorld) for the
// same author, edit, record-naming and decision-issue checks the GitHub form
// gets. Every test runs offline: the forge is a stub on PATH.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	glrHost    = "gitlab.example.com"
	glrProject = "example-group/example-project"
	glrIssue   = 41
	glrNote    = 9001
	glrPosted  = "2026-09-01T10:00:00Z"
)

func glrURL(host, project string, issue, note int) string {
	return fmt.Sprintf("https://%s/%s/-/issues/%d#note_%d", host, project, issue, note)
}

var glrRuling = glrURL(glrHost, glrProject, glrIssue, glrNote)

// glrRoster binds the review identity to GitLab (as #7246's auto-flip reads
// it) and maps the example humans; the API base names the GitLab host.
func glrRoster(t *testing.T, forge string) {
	t.Helper()
	t.Cleanup(scanReloadConfig)
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := strings.Replace(fixtureRoster+darFixtureExtra(), "ASSAY_HUMAN_LOGIN_MAP=alex:ada",
		"ASSAY_HUMAN_LOGIN_MAP=example_human:"+rlHuman+",example_other:"+rlOther, 1)
	if forge == "gitlab" {
		roster = strings.Replace(roster, "reviewer=assay-reviewer-app:300000004", "reviewer=gitlab:example-reviewer-sa:42", 1)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GITLAB_API_BASE", "https://"+glrHost+"/api/v4")
	scanReloadConfig()
}

func glrItem(kind string, n int, payload string) string {
	return fmt.Sprintf(`{"schema":%d,"kind":%q,"items":[{"repo":%q,"number":%d,%s}],"partial":[]}`, deskreadSchema, kind, glrProject, n, payload)
}

// glrNoteJSON is a note whose author account type reads User — the shape a
// deskread that reports GitLab account types would emit.
func glrNoteJSON(id int, author, body, created, updated string) string {
	return glrNoteTyped(id, author, `"authorType":"User",`, body, created, updated)
}

// glrNoteTyped is glrNoteJSON with the author-type member given raw (with its
// trailing comma), or "" to leave it out as today's GitLab note read does.
func glrNoteTyped(id int, author, typ, body, created, updated string) string {
	return fmt.Sprintf(`{"databaseId":%d,"authorLogin":%q,"authorId":7,%s"createdAt":%q,"updatedAt":%q,"body":%s,"url":""}`,
		id, author, typ, created, updated, glJSON(body))
}

// glrWorld is the all-conditions-hold forge: a mapped human's unedited note
// naming text, on issue #41 whose body carries marker.
func glrWorld(text, marker string, notes ...string) *glWorld {
	w := &glWorld{answers: map[string]string{}, codes: map[string]int{}}
	if notes == nil {
		notes = []string{glrNoteJSON(glrNote, rlHuman, text, glrPosted, glrPosted)}
	}
	target := fmt.Sprintf("%s#%d", glrProject, glrIssue)
	w.set(glrItem("comments", glrIssue, `"comments":[`+strings.Join(notes, ",")+`]`), "comments", "--issue", target)
	w.set(glrItem("issue", glrIssue, fmt.Sprintf(`"issue":{"state":"open","title":"t","body":%s}`, glJSON("A human decision is needed.\n\n"+marker+"\n"))), "issue", "--issue", target)
	return w
}

// glrResolve resolves a decision record whose ruling is link, on glrProject.
func glrResolve(t *testing.T, w *glWorld, link string) rulingOutcome {
	t.Helper()
	w.install(t)
	rec := decisionRecordStamp{File: rlFile, ID: rlRecordID, DecidedBy: "human:<name>", Ruling: link}
	got := resolveRuling(nil, glrProject, rec, rlLogins, nil, nil)
	t.Logf("%s: %s (%s)", link, got.Reason, got.Detail)
	return got
}

func glrRecordWorld(notes ...string) *glWorld {
	return glrWorld("Ruled on "+rlRecordID+": option 1.", "<!-- decision-gate: "+rlRecordID+" -->", notes...)
}

func glrWant(t *testing.T, got rulingOutcome, want rulingReason) {
	t.Helper()
	if got.Reason != want {
		t.Fatalf("reason = %q (%s), want %q", got.Reason, got.Detail, want)
	}
}

// regression: #7252
func TestGLRulingParses(t *testing.T) {
	glrRoster(t, "gitlab")
	for _, c := range []struct{ raw, repo string }{
		{glrRuling, glrProject},
		{glrURL(glrHost, "example-group/sub/example-project", 7, 12), "example-group/sub/example-project"},
		{glrURL("GitLab.Example.com", glrProject, glrIssue, glrNote), glrProject},
	} {
		link, ok := parseRulingURL(c.raw)
		if !ok || link.Repo != c.repo || link.Issue <= 0 || link.CommentID <= 0 {
			t.Errorf("parseRulingURL(%q) = %+v, %v; want project %s", c.raw, link, ok, c.repo)
		}
	}
}

// regression: #7252
func TestGLRulingBoundToHost(t *testing.T) {
	glrRoster(t, "gitlab")
	glrWant(t, glrResolve(t, glrRecordWorld(), glrRuling), rulingOK)
	for _, bad := range []string{
		glrURL("gitlab.com", glrProject, glrIssue, glrNote),
		glrURL("gitlab.example.com.evil.example", glrProject, glrIssue, glrNote),
		fmt.Sprintf("https://%s/%s/-/merge_requests/%d#note_%d", glrHost, glrProject, glrIssue, glrNote),
		fmt.Sprintf("https://%s:8443/%s/-/issues/%d#note_%d", glrHost, glrProject, glrIssue, glrNote),
		fmt.Sprintf("http://%s/%s/-/issues/%d#note_%d", glrHost, glrProject, glrIssue, glrNote),
		glrRuling + " trailing",
		fmt.Sprintf("https://%s/%s/-/issues/%d", glrHost, glrProject, glrIssue),
		fmt.Sprintf("https://%s/example-group/-/x/-/issues/%d#note_%d", glrHost, glrIssue, glrNote),
	} {
		if _, ok := parseRulingURL(bad); ok {
			t.Errorf("parseRulingURL(%q) accepted a link off the configured host or grammar", bad)
		}
	}
}

// regression: #7252
func TestGLRulingOtherProject(t *testing.T) {
	glrRoster(t, "gitlab")
	got := glrResolve(t, glrRecordWorld(), glrURL(glrHost, "example-group/elsewhere", glrIssue, glrNote))
	glrWant(t, got, rulingUnrelatedIssue)
}

// regression: #7252
func TestGLRulingUntrustedWriter(t *testing.T) {
	glrRoster(t, "gitlab")
	w := glrRecordWorld(glrNoteJSON(glrNote, "example-stranger", "Ruled on "+rlRecordID+".", glrPosted, glrPosted))
	glrWant(t, glrResolve(t, w, glrRuling), rulingWrongAuthor)
}

// regression: #7252
func TestGLRulingBotWriter(t *testing.T) {
	glrRoster(t, "gitlab")
	for _, bot := range []string{"example-reviewer-sa", "project_12_bot_0a1b", "group_3_bot"} {
		w := glrRecordWorld(glrNoteJSON(glrNote, bot, "Ruled on "+rlRecordID+".", glrPosted, glrPosted))
		glrWant(t, glrResolve(t, w, glrRuling), rulingBotAuthor)
	}
}

// Condition 3b on GitLab: a note whose author account type was not read is
// could-not-check, never read as User — even from a mapped human whose
// unedited note names the record on its decision issue (every other condition
// holds). Today's GitLab note read reports no type, so this is every note.
// regression: #7252 (review F1)
func TestGLRulingUntypedAuthor(t *testing.T) {
	glrRoster(t, "gitlab")
	text := "Ruled on " + rlRecordID + ": option 1."
	for name, typ := range map[string]string{"absent": "", "empty": `"authorType":"",`, "blank": `"authorType":" ",`} {
		t.Run(name, func(t *testing.T) {
			w := glrRecordWorld(glrNoteTyped(glrNote, rlHuman, typ, text, glrPosted, glrPosted))
			got := glrResolve(t, w, glrRuling)
			glrWant(t, got, rulingUnresolvable)
			wantContains(t, "refusal", got.Detail, "no account type")
		})
	}
}

// A stated account type other than User is a non-human author, whatever the
// login map says about the login. regression: #7252 (review F1)
func TestGLRulingNonUserType(t *testing.T) {
	glrRoster(t, "gitlab")
	text := "Ruled on " + rlRecordID + ": option 1."
	for _, typ := range []string{"Bot", "Organization", "Mannequin"} {
		w := glrRecordWorld(glrNoteTyped(glrNote, rlHuman, `"authorType":"`+typ+`",`, text, glrPosted, glrPosted))
		glrWant(t, glrResolve(t, w, glrRuling), rulingBotAuthor)
	}
}

// Every gap in the note or issue read is could-not-check, never a pass: a note
// with no author, unreadable times, an item with no comment list, an issue
// read that carries no issue. regression: #7252 (review A1)
func TestGLRulingEnvelopeGaps(t *testing.T) {
	glrRoster(t, "gitlab")
	text := "Ruled on " + rlRecordID + ": option 1."
	target := fmt.Sprintf("%s#%d", glrProject, glrIssue)
	cases := map[string]*glWorld{
		"no author":      glrRecordWorld(glrNoteJSON(glrNote, "", text, glrPosted, glrPosted)),
		"bad createdAt":  glrRecordWorld(glrNoteJSON(glrNote, rlHuman, text, "yesterday", glrPosted)),
		"bad updatedAt":  glrRecordWorld(glrNoteJSON(glrNote, rlHuman, text, glrPosted, "")),
		"no comments":    glrRecordWorld(),
		"no issue field": glrRecordWorld(),
	}
	cases["no comments"].set(glrItem("comments", glrIssue, `"title":"t"`), "comments", "--issue", target)
	cases["no issue field"].set(glrItem("issue", glrIssue, `"title":"t"`), "issue", "--issue", target)
	for name, w := range cases {
		t.Run(name, func(t *testing.T) { glrWant(t, glrResolve(t, w, glrRuling), rulingUnresolvable) })
	}
}

// regression: #7252
func TestGLRulingUnreadableNote(t *testing.T) {
	glrRoster(t, "gitlab")
	w := glrRecordWorld()
	k := glArgKey("comments", "--issue", fmt.Sprintf("%s#%d", glrProject, glrIssue))
	w.answers[k] = ""
	w.codes[k] = 6
	glrWant(t, glrResolve(t, w, glrRuling), rulingUnresolvable)

	// An answer missing its issue read is unreadable too, never a pass.
	w = glrRecordWorld()
	delete(w.answers, glArgKey("issue", "--issue", fmt.Sprintf("%s#%d", glrProject, glrIssue)))
	glrWant(t, glrResolve(t, w, glrRuling), rulingUnresolvable)
}

// regression: #7252
func TestGLRulingDeletedOrEdited(t *testing.T) {
	glrRoster(t, "gitlab")
	other := glrNoteJSON(glrNote+1, rlHuman, "Ruled on "+rlRecordID+".", glrPosted, glrPosted)
	glrWant(t, glrResolve(t, glrRecordWorld(other), glrRuling), rulingDeleted)
	edited := glrNoteJSON(glrNote, rlHuman, "Ruled on "+rlRecordID+".", glrPosted, "2026-09-02T10:00:00Z")
	glrWant(t, glrResolve(t, glrRecordWorld(edited), glrRuling), rulingEditedComment)
	unnamed := glrNoteJSON(glrNote, rlHuman, "Option 1.", glrPosted, glrPosted)
	glrWant(t, glrResolve(t, glrRecordWorld(unnamed), glrRuling), rulingRecordNotNamed)
	w := glrWorld("Ruled on "+rlRecordID+".", "<!-- decision-gate: DR-another-record -->")
	glrWant(t, glrResolve(t, w, glrRuling), rulingUnrelatedIssue)
}

// A GitLab note is read only when the roster binds the repo to GitLab.
// regression: #7252
func TestGLRulingNeedsGitLabForge(t *testing.T) {
	glrRoster(t, "github")
	got := glrResolve(t, glrRecordWorld(), glrRuling)
	if got.Reason == rulingOK || got.Reason == rulingMalformed {
		t.Fatalf("reason = %q (%s), want a refusal naming the forge", got.Reason, got.Detail)
	}
	wantContains(t, "refusal", got.Detail, "gitlab")
}

// The decision-gate hold lifts on a GitLab ruling note, as on a GitHub one,
// once deskread reports the note author's account type. The fixture reports
// "User"; the GitLab note read reports no type yet, so today every GitLab
// ruling stops at condition 3b and the hold stays. regression: #7252
func TestGLRulingLiftsGateHold(t *testing.T) {
	glrRoster(t, "gitlab")
	dhSeams(t, &rlForge{commentStatus: 404, issueStatus: 404})
	decisionGateLoginsFn = func() map[string]string { return rlLogins }
	repoFromOriginAtFn = func(string) string { return glrProject }
	glrWorld("Option 1 for "+dhBoardID+", as written.", dhMarker).install(t)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", "decision-issue: 41\nruling: "+glrRuling+"\n"),
	})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("layer one refused a GitLab ruling note: %v", p)
	}
	code, out := dhGate(t, root)
	if code != 0 {
		t.Fatalf("--decision-gate exit %d, want 0:\n%s", code, out)
	}
	wantContains(t, "--decision-gate report", out, dhBoardID+" RULED — ruled by "+rlHuman)
}

// An acceptance transition's approval stays a GitHub comment: a well-formed
// GitLab ruling note on the configured host is no approval URL there, while
// the GitHub form still is. regression: #7252 (review A2)
func TestGLRulingNoApproval(t *testing.T) {
	glrRoster(t, "gitlab")
	at := func(u string) string {
		return "Acceptance-Transition: link=1 amendment=" + strings.Repeat("a", 40) + " approval=" + u
	}
	if _, why := transitionLinks(at(rlURL(rlRepo, rlIssue, rlComment))); why != "" {
		t.Fatalf("a GitHub approval comment was refused: %s", why)
	}
	if _, ok := parseRulingURL(glrRuling); !ok {
		t.Fatal("the GitLab note did not parse, so the refusal below proves nothing")
	}
	_, why := transitionLinks(at(glrRuling))
	wantContains(t, "transition refusal", why, "a valid approval URL is required")
}
