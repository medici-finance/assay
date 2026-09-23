package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the design-decision RULING-LINK lane of --corroborate
// (decisionruling.go, registers-v1 §7.5). The forge is FAKED: every test serves
// the two REST reads the lane makes (GET /issues/comments/{id}, GET /issues/{n})
// from an httptest server and injects a *ghClient pointed at it — no test reaches
// the network. Fixture identities are example-* only, and the human-login map is
// passed in as a parameter, so no test depends on the shared fixture roster.
//
// Every refusal path has its own test, each asserting the NAMED reason, and each
// is paired with an entry in decisionruling-mutations.json that disables exactly
// that check (the fail-first evidence: run the spec through tools/desk/cmd/muhar).

const (
	rlRepo     = "example-org/example-repo"
	rlRecordID = "DR-example-ruling-rec"
	rlFile     = "docs/streams/decisions/" + rlRecordID + ".md"
	rlIssue    = 41
	rlComment  = 9001
	rlMarker   = "<!-- decision-gate: " + rlRecordID + " -->"
	rlHuman    = "example-human" // a mapped human's login
	rlOther    = "example-other" // a DIFFERENT mapped human's login
)

var rlLogins = map[string]string{
	"example_human": rlHuman,
	"example_other": rlOther,
}

func rlURL(repo string, issue, comment int) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%d", repo, issue, comment)
}

// rlForge is the faked forge: one comment and one issue, each overridable.
type rlForge struct {
	commentStatus int
	commentBody   string
	issueStatus   int
	issueBody     string
}

// rlPosted is the default comment timestamp: created_at == updated_at, i.e. a
// comment never edited after it was posted.
const rlPosted = "2026-09-01T10:00:00Z"

// rlRulingText is the default ruling comment text: it names the record by its id,
// which is what ties the comment to THIS record in text the human wrote.
const rlRulingText = "Approved: " + rlRecordID + " as briefed."

func rlCommentJSON(login, typ, repo string, issue int) string {
	return rlCommentJSONWith(login, typ, repo, issue, rlRulingText, rlPosted, rlPosted)
}

// rlCommentJSONWith is rlCommentJSON with the comment text and both timestamps
// chosen by the test. An empty created/updated value omits that field.
func rlCommentJSONWith(login, typ, repo string, issue int, body, created, updated string) string {
	ts := ""
	if created != "" {
		ts += fmt.Sprintf(`,"created_at":%q`, created)
	}
	if updated != "" {
		ts += fmt.Sprintf(`,"updated_at":%q`, updated)
	}
	return fmt.Sprintf(`{"id":%d,"user":{"login":%q,"type":%q},"issue_url":"https://api.github.com/repos/%s/issues/%d","html_url":%q,"body":%q%s}`,
		rlComment, login, typ, repo, issue, rlURL(repo, issue, rlComment), body, ts)
}

func rlIssueJSON(body string) string {
	return fmt.Sprintf(`{"number":%d,"state":"closed","body":%q}`, rlIssue, body)
}

// defaultRLForge is the all-conditions-hold forge: a mapped human's comment on
// the record's own decision issue, which carries the record's marker.
func defaultRLForge() *rlForge {
	return &rlForge{
		commentStatus: 200,
		commentBody:   rlCommentJSON(rlHuman, "User", rlRepo, rlIssue),
		issueStatus:   200,
		issueBody:     rlIssueJSON("A human decision is needed.\n\n" + rlMarker + "\n"),
	}
}

func (f *rlForge) client(t *testing.T) *ghClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case fmt.Sprintf("/repos/%s/issues/comments/%d", rlRepo, rlComment):
			w.WriteHeader(f.commentStatus)
			fmt.Fprint(w, f.commentBody)
		case fmt.Sprintf("/repos/%s/issues/%d", rlRepo, rlIssue):
			w.WriteHeader(f.issueStatus)
			fmt.Fprint(w, f.issueBody)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return &ghClient{doer: srv.Client(), base: srv.URL, token: "x"}
}

// rlRun drives the lane end to end for one record: the record's stamps are
// synthesised (addDecisionRecordStamps), its ruling resolved against the faked
// forge (resolveRuling), and the verdict decided by corroborateStampsRuled with
// the given PR data. It returns the single result for the record's stamp.
func rlRun(t *testing.T, c *ghClient, rec decisionRecordStamp, data *ghPRData, citing []string) corroborateResult {
	t.Helper()
	stamps := addDecisionRecordStamps(nil, []decisionRecordStamp{rec})
	if len(stamps) != 1 {
		t.Fatalf("addDecisionRecordStamps produced %d stamps, want 1 — the record's decided-by must be visible to the stamp lane", len(stamps))
	}
	outcomes := rulingOutcomes{rec.File: resolveRuling(c, rlRepo, rec, rlLogins, citing)}
	res := corroborateStampsRuled(stamps, data, rlRepo, 7, nil, outcomes)
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	return res[0]
}

func rlPlaceholder(ruling string) decisionRecordStamp {
	return decisionRecordStamp{File: rlFile, ID: rlRecordID, DecidedBy: "human:<name>", Ruling: ruling}
}

func rlNamed(name, ruling string) decisionRecordStamp {
	return decisionRecordStamp{File: rlFile, ID: rlRecordID, DecidedBy: "human:" + name, Names: []string{name}, Ruling: ruling}
}

func rlWantMissing(t *testing.T, r corroborateResult, reason rulingReason) {
	t.Helper()
	if r.Verdict != verdictMissing {
		t.Fatalf("verdict = %v, want MISSING-CORROBORATION (%s) — evidence: %s", r.Verdict, reason, r.Evidence)
	}
	if !strings.Contains(r.Evidence, "ruling "+string(reason)+":") {
		t.Fatalf("evidence does not name the reason %q: %s", reason, r.Evidence)
	}
}

// ---- positive controls ---------------------------------------------------------

// POSITIVE: a placeholder decided-by with a ruling link to a mapped human's
// comment on the record's own decision issue corroborates.
func TestRuling_Positive_PlaceholderWithRuling(t *testing.T) {
	r := rlRun(t, defaultRLForge().client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil)
	if r.Verdict != verdictCorroborated {
		t.Fatalf("verdict = %v, want CORROBORATED — evidence: %s", r.Verdict, r.Evidence)
	}
	if r.Login != rlHuman || !strings.Contains(r.Evidence, "example_human") {
		t.Fatalf("evidence should name the ruling author and the mapped human, got login %q: %s", r.Login, r.Evidence)
	}
}

// POSITIVE (brief marker form): the decision issue names a BRIEF whose design:
// cites the record, in the colon work-item form, rather than the record itself.
func TestRuling_Positive_CitingBriefMarker(t *testing.T) {
	f := defaultRLForge()
	f.issueBody = rlIssueJSON("<!-- decision-gate: example-org:example-repo:example-stream:7 -->\n")
	r := rlRun(t, f.client(t), rlNamed("example_human", rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, []string{"example-stream/07"})
	if r.Verdict != verdictCorroborated {
		t.Fatalf("verdict = %v, want CORROBORATED — evidence: %s", r.Verdict, r.Evidence)
	}
}

// ---- refusal paths (each names its reason) -------------------------------------

// The hole this lane closes: a placeholder decided-by with NO ruling link is a
// PROBLEM. Before this lane the placeholder was not a stamp at all.
func TestRuling_PlaceholderWithoutRuling(t *testing.T) {
	rlWantMissing(t, rlRun(t, defaultRLForge().client(t), rlPlaceholder(""), &ghPRData{}, nil), rulingPlaceholderUnratified)
}

// A placeholder can never be corroborated by a PR anchor — it names nobody.
func TestRuling_PlaceholderNotRescuedByPRApproval(t *testing.T) {
	data := &ghPRData{Reviews: []ghReview{{Author: ghAuthor{Login: rlHuman}, State: "APPROVED"}}}
	rlWantMissing(t, rlRun(t, defaultRLForge().client(t), rlPlaceholder(""), data, nil), rulingPlaceholderUnratified)
}

func TestRuling_MalformedLink(t *testing.T) {
	bad := fmt.Sprintf("https://github.com/%s/pull/%d#issuecomment-%d", rlRepo, rlIssue, rlComment)
	rlWantMissing(t, rlRun(t, defaultRLForge().client(t), rlPlaceholder(bad), &ghPRData{}, nil), rulingMalformed)
}

func TestRuling_LinkIntoAnotherRepo(t *testing.T) {
	rlWantMissing(t, rlRun(t, defaultRLForge().client(t), rlPlaceholder(rlURL("example-org/elsewhere", rlIssue, rlComment)), &ghPRData{}, nil), rulingUnrelatedIssue)
}

func TestRuling_Unresolvable(t *testing.T) {
	f := defaultRLForge()
	f.commentStatus = http.StatusInternalServerError
	f.commentBody = `{"message":"Server Error"}`
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingUnresolvable)
}

func TestRuling_UnresolvableTransport(t *testing.T) {
	c := &ghClient{doer: &fakeDoer{err: fmt.Errorf("dial tcp: connection refused")}, base: githubAPIBase, token: "x"}
	rlWantMissing(t, rlRun(t, c, rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingUnresolvable)
}

func TestRuling_DeletedComment(t *testing.T) {
	f := defaultRLForge()
	f.commentStatus = http.StatusNotFound
	f.commentBody = `{"message":"Not Found"}`
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingDeleted)
}

// A bot is refused by its TYPE even when its login is (mis)mapped as a human.
func TestRuling_BotAuthor(t *testing.T) {
	f := defaultRLForge()
	f.commentBody = rlCommentJSON("example-bot[bot]", "Bot", rlRepo, rlIssue)
	logins := map[string]string{"example_human": "example-bot[bot]"}
	stamps := addDecisionRecordStamps(nil, []decisionRecordStamp{rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment))})
	o := resolveRuling(f.client(t), rlRepo, rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), logins, nil)
	res := corroborateStampsRuled(stamps, &ghPRData{}, rlRepo, 7, nil, rulingOutcomes{rlFile: o})
	rlWantMissing(t, res[0], rulingBotAuthor)
}

// Placeholder form: the author is a human account, but not one in the map.
func TestRuling_WrongAuthorUnmapped(t *testing.T) {
	f := defaultRLForge()
	f.commentBody = rlCommentJSON("example-stranger", "User", rlRepo, rlIssue)
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingWrongAuthor)
}

// Named form: the author IS a mapped human, but not the one decided-by names.
func TestRuling_WrongAuthorOtherHuman(t *testing.T) {
	f := defaultRLForge()
	f.commentBody = rlCommentJSON(rlOther, "User", rlRepo, rlIssue)
	rec := rlNamed("example_human", rlURL(rlRepo, rlIssue, rlComment))
	rlWantMissing(t, rlRun(t, f.client(t), rec, &ghPRData{}, nil), rulingWrongAuthor)
	// Pin the resolver's own refusal too: the stamp verdict alone is backed by a
	// second layer (rulingOutcome.rulesFor), so it would stay MISSING even if the
	// resolver passed a comment by the wrong human.
	if o := resolveRuling(f.client(t), rlRepo, rec, rlLogins, nil); o.Reason != rulingWrongAuthor {
		t.Fatalf("resolveRuling reason = %q, want %q — detail: %s", o.Reason, rulingWrongAuthor, o.Detail)
	}
}

// The link pairs issue #41 with a comment id that actually sits on issue #99.
func TestRuling_CommentOnDifferentIssue(t *testing.T) {
	f := defaultRLForge()
	f.commentBody = rlCommentJSON(rlHuman, "User", rlRepo, 99)
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingUnrelatedIssue)
}

// The comment is genuine and on the linked issue, but that issue is not a
// decision issue for THIS record (its marker names another record).
func TestRuling_IssueForAnotherRecord(t *testing.T) {
	f := defaultRLForge()
	f.issueBody = rlIssueJSON("<!-- decision-gate: DR-example-other-rec -->\n")
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingUnrelatedIssue)
}

// A brief marker only counts for a brief that CITES the record.
func TestRuling_BriefMarkerNotCiting(t *testing.T) {
	f := defaultRLForge()
	f.issueBody = rlIssueJSON("<!-- decision-gate: example-stream/07 -->\n")
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, []string{"example-stream/08"}), rulingUnrelatedIssue)
}

// A colon-form brief marker naming ANOTHER repository does not count.
func TestRuling_BriefMarkerOtherRepo(t *testing.T) {
	f := defaultRLForge()
	f.issueBody = rlIssueJSON("<!-- decision-gate: example-org:elsewhere:example-stream:7 -->\n")
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, []string{"example-stream/07"}), rulingUnrelatedIssue)
}

func TestRuling_PullRequestNotIssue(t *testing.T) {
	f := defaultRLForge()
	f.issueBody = fmt.Sprintf(`{"number":%d,"body":%q,"pull_request":{"url":"x"}}`, rlIssue, rlMarker)
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingUnrelatedIssue)
}

// A ruling corroborates ONLY the human who wrote it. A record whose decided-by
// names two humans, with a ruling by one of them, must not corroborate the other —
// mapped or not. Each other name is MISSING (wrong-author), never passed on
// someone else's comment and never handed to the PR anchors (the link is present).
func TestRuling_CosignerNotCorroboratedByAnothersRuling(t *testing.T) {
	for _, second := range []string{"example_other", "example_ghost"} { // mapped, unmapped
		t.Run(second, func(t *testing.T) {
			rec := decisionRecordStamp{File: rlFile, ID: rlRecordID,
				DecidedBy: "human:example_human, human:" + second,
				Names:     []string{"example_human", second},
				Ruling:    rlURL(rlRepo, rlIssue, rlComment)}
			stamps := addDecisionRecordStamps(nil, []decisionRecordStamp{rec})
			if len(stamps) != 2 {
				t.Fatalf("addDecisionRecordStamps produced %d stamps, want 2", len(stamps))
			}
			// The second name even approved the PR: a present link still decides it.
			data := &ghPRData{Reviews: []ghReview{{Author: ghAuthor{Login: rlOther}, State: "APPROVED"}}}
			out := rulingOutcomes{rec.File: resolveRuling(defaultRLForge().client(t), rlRepo, rec, rlLogins, nil)}
			got := map[string]corroborateResult{}
			for _, r := range corroborateStampsRuled(stamps, data, rlRepo, 7, nil, out) {
				got[r.Stamp.Name] = r
			}
			if r := got["example_human"]; r.Verdict != verdictCorroborated {
				t.Fatalf("the ruling author's own name: verdict = %v, want CORROBORATED — evidence: %s", r.Verdict, r.Evidence)
			}
			r := got[second]
			rlWantMissing(t, r, rulingWrongAuthor)
			if !strings.Contains(r.Evidence, rlHuman) || !strings.Contains(r.Evidence, second) {
				t.Fatalf("evidence should name the ruling author and the uncorroborated name: %s", r.Evidence)
			}
		})
	}
}

// A comment edited after it was posted does not corroborate: the text the reviewer
// reads must be the text the human wrote.
func TestRuling_EditedComment(t *testing.T) {
	f := defaultRLForge()
	f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, rlRulingText, rlPosted, "2026-12-01T10:00:00Z")
	rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingEditedComment)
}

// A comment whose timestamps cannot be read cannot be shown unedited: that is a
// could-not-check (unresolvable-link), never a pass.
func TestRuling_CommentTimestampsUnreadable(t *testing.T) {
	for name, ts := range map[string][2]string{
		"no-created": {"", rlPosted},
		"no-updated": {rlPosted, ""},
		"garbled":    {"yesterday", "yesterday"},
	} {
		t.Run(name, func(t *testing.T) {
			f := defaultRLForge()
			f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, rlRulingText, ts[0], ts[1])
			rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingUnresolvable)
		})
	}
}

// The tie between the comment and THIS record must be text the human wrote: the
// comment itself names the record id. The issue body's marker alone is editable
// after the ruling, so a comment that never names the record does not corroborate,
// even on an issue that (now) carries the record's marker. A longer id that merely
// starts with the record's id does not count.
func TestRuling_CommentDoesNotNameRecord(t *testing.T) {
	for name, text := range map[string]string{
		"no-id":     "Approved as briefed.",
		"other-id":  "Approved: DR-example-other-rec as briefed.",
		"prefix-id": "Approved: " + rlRecordID + "-v2 as briefed.",
		"infix-id":  "Approved: x" + rlRecordID + " as briefed.",
	} {
		t.Run(name, func(t *testing.T) {
			f := defaultRLForge()
			f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, text, rlPosted, rlPosted)
			rlWantMissing(t, rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil), rulingRecordNotNamed)
		})
	}
}

// The record id may sit anywhere in the comment, next to punctuation or markup.
func TestRuling_CommentNamesRecordInContext(t *testing.T) {
	for _, text := range []string{
		rlRecordID,
		"`" + rlRecordID + "`: approved.",
		"Approve (" + rlRecordID + ").\n\nThanks.",
	} {
		f := defaultRLForge()
		f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, text, rlPosted, rlPosted)
		r := rlRun(t, f.client(t), rlPlaceholder(rlURL(rlRepo, rlIssue, rlComment)), &ghPRData{}, nil)
		if r.Verdict != verdictCorroborated {
			t.Fatalf("comment %q: verdict = %v, want CORROBORATED — evidence: %s", text, r.Verdict, r.Evidence)
		}
	}
}

func TestRuling_RecordUnreadable(t *testing.T) {
	rec := loadDecisionRecordStamp(t.TempDir(), rlFile) // no such file
	if rec.LoadErr == "" {
		t.Fatal("loading a missing record must carry LoadErr")
	}
	rlWantMissing(t, rlRun(t, defaultRLForge().client(t), rec, &ghPRData{}, nil), rulingRecordUnreadable)
}

// A PRESENT-but-failing ruling link fails the stamp even though the named human
// approved the PR: the record asserts a provenance that is false.
func TestRuling_FailingLinkBeatsPRApproval(t *testing.T) {
	rlWithRoster(t) // the PR anchor resolves names through the roster's map
	data := &ghPRData{Reviews: []ghReview{{Author: ghAuthor{Login: rlHuman}, State: "APPROVED"}}}

	// Control: with NO ruling link, the same stamp IS corroborated by the PR
	// approval — so the refusal below is the ruling lane's, not a roster gap.
	noLink := rlNamed("example_human", "")
	ctl := corroborateStampsRuled(addDecisionRecordStamps(nil, []decisionRecordStamp{noLink}), data, rlRepo, 7, nil,
		rulingOutcomes{rlFile: resolveRuling(nil, rlRepo, noLink, rlLogins, nil)})
	if ctl[0].Verdict != verdictCorroborated {
		t.Fatalf("control: a named stamp with no ruling link and an APPROVED review must corroborate via the PR anchor, got %v: %s", ctl[0].Verdict, ctl[0].Evidence)
	}

	f := defaultRLForge()
	f.commentStatus = http.StatusNotFound
	rec := rlNamed("example_human", rlURL(rlRepo, rlIssue, rlComment))
	res := corroborateStampsRuled(addDecisionRecordStamps(nil, []decisionRecordStamp{rec}), data, rlRepo, 7, nil,
		rulingOutcomes{rlFile: resolveRuling(f.client(t), rlRepo, rec, rlLogins, nil)})
	rlWantMissing(t, res[0], rulingDeleted)
}

// rlWithRoster installs a test roster whose human-login map carries the example
// identities, for the tests that go through the shared HumanLogin lookup (the PR
// anchors). The reload cleanup is registered BEFORE HOME is overridden, so it runs
// after HOME is restored (cleanups are LIFO) and reloads the suite's roster.
func rlWithRoster(t *testing.T) {
	t.Helper()
	t.Cleanup(scanReloadConfig)
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := strings.Replace(fixtureRoster+darFixtureExtra(), "ASSAY_HUMAN_LOGIN_MAP=alex:ada",
		"ASSAY_HUMAN_LOGIN_MAP=example_human:"+rlHuman+",example_other:"+rlOther, 1)
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	scanReloadConfig()
	if l, ok := HumanLogin("example_human"); !ok || l != rlHuman {
		t.Fatalf("test roster not in effect: example_human -> %q, %v", l, ok)
	}
}

// ---- plumbing ------------------------------------------------------------------

// A real-name record with no ruling link is NOT decided by the ruling lane — the
// pre-existing PR anchors still decide it, unchanged.
func TestRuling_NamedWithoutLinkFallsThrough(t *testing.T) {
	o := rulingOutcome{Present: false, Names: []string{"example_human"}}
	if o.appliesTo("example_human") {
		t.Fatal("a real-name stamp with no ruling link must fall through to the PR anchors")
	}
	if !o.appliesTo(decisionPlaceholderName) {
		t.Fatal("the placeholder stamp must always be decided by the ruling lane")
	}
}

// Only DR records in the register directory, with an ADDED line, are gated.
func TestRuling_RecordsInDiffScope(t *testing.T) {
	diff := strings.Join([]string{
		"+++ b/docs/streams/decisions/DR-example-ruling-rec.md",
		"+decided-by: \"human:<name>\"",
		"+++ b/docs/streams/decisions/README.md",
		"+prose",
		"+++ b/docs/elsewhere/DR-example-ruling-rec.md",
		"+decided-by: \"human:<name>\"",
		"+++ b/docs/streams/decisions/DR-example-untouched.md",
		"-decided-by: \"human:<name>\"",
		"",
	}, "\n")
	got := decisionRecordsInDiff("", diff)
	if len(got) != 1 || got[0] != rlFile {
		t.Fatalf("decisionRecordsInDiff = %v, want exactly [%s]", got, rlFile)
	}
}

// The record loader parses decided-by names and the ruling from frontmatter.
func TestRuling_LoadRecord(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(rlFile))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	link := rlURL(rlRepo, rlIssue, rlComment)
	body := "---\nid: " + rlRecordID + "\ndecided-by: \"human:<name>\"\nruling: \"" + link + "\"\n---\nbody\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := loadDecisionRecordStamp(root, rlFile)
	if rec.LoadErr != "" || rec.ID != rlRecordID || rec.Ruling != link || len(rec.Names) != 0 {
		t.Fatalf("loaded %+v", rec)
	}
}

// The offline --lint checks the ruling: grammar when present, and only then.
func TestRuling_LintGrammar(t *testing.T) {
	write := func(ruling string) string {
		root := t.TempDir()
		dir := filepath.Join(root, "docs", "streams", "decisions")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		fm := "---\nid: " + rlRecordID + "\ndate: \"2026-09-23\"\ntitle: \"t\"\nconsequence: minor\ndecided-by: \"human:<name>\"\n"
		if ruling != "" {
			fm += "ruling: \"" + ruling + "\"\n"
		}
		fm += "alternatives:\n  - \"a — b\"\naccepted:\n  - \"c\"\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, rlRecordID+".md"), []byte(fm), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	for _, tc := range []struct {
		ruling string
		bad    bool
	}{
		{"", false},
		{rlURL(rlRepo, rlIssue, rlComment), false},
		{"https://github.com/" + rlRepo + "/issues/41", true},
		{"https://example.com/" + rlRepo + "/issues/41#issuecomment-9001", true},
		{"see the ruling on #41", true},
	} {
		probs := decisionRegisterProblems(write(tc.ruling))
		hit := false
		for _, p := range probs {
			hit = hit || strings.Contains(p, "ruling ")
		}
		if hit != tc.bad {
			t.Errorf("ruling %q: lint flagged=%v, want %v (problems: %v)", tc.ruling, hit, tc.bad, probs)
		}
	}
}
