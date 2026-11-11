package main

// glnote_test.go — the human-decided lane on a GitLab project (#6061).
//
// `--decision` names the human's ruling comment. On GitLab that is a note link,
// `https://<host>/<group>/<project>/-/issues/<N>#note_<id>`. The lane accepts it only when the
// link's forge, host and project are the ones serving the issue being closed, its number is
// that issue's, the note is FETCHED from the forge, and its author is the roster's blessing
// authority (login and numeric user id). Every refusal row breaks exactly one of those, and
// "refused" is proved by the absence of a write.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const glHost = "gitlab.example.test"

// gitlabWorld is triageWorld served by a GitLab backend on glHost.
func gitlabWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := triageWorld(t)
	s.forge = deskkit.ForgeGitLab
	s.host = glHost
	return s, rul
}

// plantNote plants a note on repo#n the way the GitLab backend reports one: a numeric id, the
// author's username and user id, and NO web URL (GitLab notes carry none).
func plantNote(s *stubRemote, repo string, n int, cid, login string, id int64) {
	issueURL := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repo, n)
	s.comment[cid] = fmt.Sprintf(
		`{"id":%s,"html_url":"","issue_url":%q,"body":%q,"minimized":false,"user":{"login":%q,"id":%d,"type":"User"}}`,
		cid, issueURL, "Decision: not pursuing this; residual work is in #40.\n", login, id)
}

func noteURL(host, repo string, n int, cid string) string {
	return fmt.Sprintf("https://%s/%s/-/issues/%d#note_%s", host, repo, n, cid)
}

func humanDecidedRun(rul, url string) (int, string) {
	return execCLI(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionHumanDecided, "--decision", url, "--tracker", "#40", "--rulings", rul)
}

// TestGitLabNoteDecisionCloses is the fail-first row: on main a GitLab note link is refused
// at parse; with the fix it closes, because the note is fetched and its author is the
// blessing authority.
// regression: #6061
func TestGitLabNoteDecisionCloses(t *testing.T) {
	s, rul := gitlabWorld(t)
	plantNote(s, testRepo, triageIssue, "8001", blessLogin, blessID)
	url := noteURL(glHost, testRepo, triageIssue, "8001")
	code, out := humanDecidedRun(rul, url)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0 (a fetched blessing-authority note on this issue), got %d\n%s", code, out)
	}
	assertClosedWith(t, s, reasonNotPlanned)
	if body := commentBody(t, s); !strings.Contains(body, "note_8001") {
		t.Fatalf("close comment should cite the note link:\n%s", body)
	}
}

// TestGitLabNoteDecisionRefusals holds one row per property the lane checks. Each row's
// world is otherwise the accepted one, so a row that closes shows that property unchecked.
// regression: #6061
func TestGitLabNoteDecisionRefusals(t *testing.T) {
	type row struct {
		name  string
		world func(*testing.T) (*stubRemote, string)
		plant func(*stubRemote)
		url   string
		want  int
	}
	bless := func(repo string, n int, cid string) func(*stubRemote) {
		return func(s *stubRemote) { plantNote(s, repo, n, cid, blessLogin, blessID) }
	}
	rows := []row{
		{
			name: "author is not the blessing authority", world: gitlabWorld,
			plant: func(s *stubRemote) { plantNote(s, testRepo, triageIssue, "8101", sharedLogin, sharedID) },
			url:   noteURL(glHost, testRepo, triageIssue, "8101"), want: deskkit.ExitRefused,
		},
		{
			name: "blessing login with another user id", world: gitlabWorld,
			plant: func(s *stubRemote) { plantNote(s, testRepo, triageIssue, "8102", blessLogin, blessID+7) },
			url:   noteURL(glHost, testRepo, triageIssue, "8102"), want: deskkit.ExitRefused,
		},
		{
			// A genuine ruling, but in another project: never reused here.
			name: "note in another project", world: gitlabWorld,
			plant: bless(otherRepo, triageIssue, "8103"),
			url:   noteURL(glHost, otherRepo, triageIssue, "8103"), want: deskkit.ExitRefused,
		},
		{
			// The note IS on this issue; only the host differs. The host is the one fact the
			// fetch cannot correct, so the link must not be accepted on the forge's say-so.
			name: "link on another host", world: gitlabWorld,
			plant: bless(testRepo, triageIssue, "8104"),
			url:   noteURL("gitlab.other.test", testRepo, triageIssue, "8104"), want: deskkit.ExitRefused,
		},
		{
			name: "note on another issue", world: gitlabWorld,
			plant: bless(testRepo, subjectIssue, "8105"),
			url:   noteURL(glHost, testRepo, subjectIssue, "8105"), want: deskkit.ExitRefused,
		},
		{
			// GitLab numbers merge requests apart from issues: !120 is not #120.
			name: "note on the merge request of that number", world: gitlabWorld,
			plant: bless(testRepo, triageIssue, "8106"),
			url:   fmt.Sprintf("https://%s/%s/-/merge_requests/%d#note_8106", glHost, testRepo, triageIssue),
			want:  deskkit.ExitRefused,
		},
		{
			name: "thread link with no anchor", world: gitlabWorld,
			plant: bless(testRepo, triageIssue, "8107"),
			url:   fmt.Sprintf("https://%s/%s/-/issues/%d", glHost, testRepo, triageIssue), want: deskkit.ExitRefused,
		},
		{
			name: "unknown forge anchor", world: gitlabWorld,
			plant: bless(testRepo, triageIssue, "8108"),
			url:   fmt.Sprintf("https://%s/%s/-/issues/%d#discussion_8108", glHost, testRepo, triageIssue),
			want:  deskkit.ExitRefused,
		},
		{
			name: "GitHub link on a GitLab issue", world: gitlabWorld,
			plant: bless(testRepo, triageIssue, "8109"),
			url:   fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-8109", testRepo, triageIssue),
			want:  deskkit.ExitRefused,
		},
		{
			name: "GitLab link on a GitHub issue", world: triageWorld,
			plant: bless(testRepo, triageIssue, "8110"),
			url:   noteURL("gitlab.com", testRepo, triageIssue, "8110"), want: deskkit.ExitRefused,
		},
		{
			// Same host as the issue's forge, other forge's grammar: only the forge match
			// refuses it, so the host match cannot stand in for that check.
			name: "GitLab-form link on github.com", world: triageWorld,
			plant: bless(testRepo, triageIssue, "8114"),
			url:   noteURL("github.com", testRepo, triageIssue, "8114"), want: deskkit.ExitRefused,
		},
		{
			name: "note link with a port", world: gitlabWorld,
			plant: bless(testRepo, triageIssue, "8111"),
			url:   noteURL(glHost+":8443", testRepo, triageIssue, "8111"), want: deskkit.ExitRefused,
		},
		{
			// An unreadable thread is could-not-check: refuse, never accept the link text.
			name: "the thread read fails", world: gitlabWorld,
			plant: func(s *stubRemote) {
				plantNote(s, testRepo, triageIssue, "8112", blessLogin, blessID)
				s.failThread[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = true
			},
			url: noteURL(glHost, testRepo, triageIssue, "8112"), want: deskkit.ExitUnverifiable,
		},
		{
			name: "the note is not on the issue", world: gitlabWorld,
			plant: bless(testRepo, triageIssue, "8113"),
			url:   noteURL(glHost, testRepo, triageIssue, "8199"), want: deskkit.ExitRefused,
		},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			s, rul := r.world(t)
			r.plant(s)
			code, out := humanDecidedRun(rul, r.url)
			if code != r.want {
				t.Fatalf("want exit %d, got %d\n%s", r.want, code, out)
			}
			assertNoWrites(t, s)
		})
	}
}

// TestGitHubDecisionStillCloses pins that the GitHub form keeps working unchanged next to
// the GitLab one.
// regression: #6061
func TestGitHubDecisionStillCloses(t *testing.T) {
	s, rul := triageWorld(t)
	url := plantDecision(s, triageIssue, "8201")
	code, out := humanDecidedRun(rul, url)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	assertClosedWith(t, s, reasonNotPlanned)
}
