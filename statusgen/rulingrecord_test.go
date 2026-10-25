package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for `--corroborate --ruling-record` (rulingrecord.go, #2079): one decision
// record corroborated outside any PR, through the same faked forge the ruling-lane
// tests use (rlForge). Fixture identities are example-* only.

// The dispatched brief the ruling must rule on. Its decision issue carries its
// board-id marker, and the human's comment names it as well as the record.
const (
	rrBriefRel    = "docs/streams/example-stream/brief-01-example.md"
	rrBriefID     = "example-stream/01"
	rrBriefMarker = "<!-- decision-gate: " + rrBriefID + " -->"
)

// rrBoundForge is the all-conditions-hold forge for a recorded ruling: the
// record's ruling stands (defaultRLForge), its decision issue also carries the
// brief's marker, and the comment names both the record and the brief.
func rrBoundForge() *rlForge {
	f := defaultRLForge()
	f.issueBody = rlIssueJSON("A human decision is needed.\n\n" + rlMarker + "\n" + rrBriefMarker + "\n")
	f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue,
		"Approved: "+rlRecordID+" for "+rrBriefID+".", rlPosted, rlPosted)
	return f
}

// rrSetup writes the canonical record and the brief under a temp root, pins the
// repository and the forge client, and installs the example roster.
func rrSetup(t *testing.T, f *rlForge) string {
	t.Helper()
	rlWithRoster(t)
	root := t.TempDir()
	rlWriteRecord(t, root, rlFile, rlRecordID)
	bp := filepath.Join(root, filepath.FromSlash(rrBriefRel))
	if err := os.MkdirAll(filepath.Dir(bp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bp, []byte("---\nbrief: "+rrBriefID+"\ngate: human\n---\n# Example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prevRepo, prevClient := repoFromOriginAtFn, rulingRecordClientFn
	t.Cleanup(func() { repoFromOriginAtFn, rulingRecordClientFn = prevRepo, prevClient })
	repoFromOriginAtFn = func(string) string { return rlRepo }
	c := f.client(t)
	rulingRecordClientFn = func() *ghClient { return c }
	return root
}

// regression: #2079
func TestRulingRecord_Corroborates(t *testing.T) {
	root := rrSetup(t, rrBoundForge())
	if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 0 {
		t.Fatalf("a record whose ruling link resolves to a mapped human's unedited comment must corroborate (exit 0), got %d", got)
	}
}

// regression: #2079
func TestRulingRecord_RefusesWrongAuthor(t *testing.T) {
	f := rrBoundForge()
	f.commentBody = rlCommentJSON("example-stranger", "User", rlRepo, rlIssue)
	root := rrSetup(t, f)
	if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 1 {
		t.Fatalf("a ruling comment by an unmapped login must refuse (exit 1), got %d", got)
	}
}

// regression: #2079
func TestRulingRecord_RefusesBotAuthor(t *testing.T) {
	f := rrBoundForge()
	f.commentBody = rlCommentJSON("example-app[bot]", "Bot", rlRepo, rlIssue)
	root := rrSetup(t, f)
	if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 1 {
		t.Fatalf("a bot-authored ruling comment must refuse (exit 1), got %d", got)
	}
}

// regression: #2079
func TestRulingRecord_RefusesNoRulingLink(t *testing.T) {
	root := rrSetup(t, rrBoundForge())
	p := filepath.Join(root, filepath.FromSlash(rlFile))
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, l := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(l, "ruling:") {
			kept = append(kept, l)
		}
	}
	if err := os.WriteFile(p, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 1 {
		t.Fatalf("outside a PR a record with no ruling: link has no anchor and must refuse (exit 1), got %d", got)
	}
}

// regression: #2079
func TestRulingRecord_RefusesMissingRecord(t *testing.T) {
	root := rrSetup(t, rrBoundForge())
	if got := runCorroborateRecord(root, "docs/streams/decisions/DR-example-absent.md", rrBriefRel); got != 1 {
		t.Fatalf("an absent record is unreadable and must refuse (exit 1), got %d", got)
	}
}

func TestRulingRecord_ForgeErrorCouldNotCheck(t *testing.T) {
	f := rrBoundForge()
	f.commentStatus = 500
	f.commentBody = `{"message":"boom"}`
	root := rrSetup(t, f)
	if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 2 {
		t.Fatalf("a forge read that failed is could-not-check (exit 2), got %d", got)
	}
}

func TestRulingRecord_UsageAndRepo(t *testing.T) {
	root := rrSetup(t, rrBoundForge())
	for _, rel := range []string{
		"docs/streams/decisions/README.md",
		"docs/streams/decisions/../x.md",
		"docs/streams/other/DR-x.md",
		"",
	} {
		if got := runCorroborateRecord(root, rel, rrBriefRel); got != 2 {
			t.Errorf("--ruling-record %q is not a decision record path: want exit 2, got %d", rel, got)
		}
	}
	repoFromOriginAtFn = func(string) string { return "" }
	if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 2 {
		t.Fatalf("an undeterminable repository is could-not-check (exit 2), got %d", got)
	}
}

// A record whose ruling stands — for ANOTHER brief — must not open this brief's
// gate. The issue must carry this brief's marker AND the comment must name it:
// the marker alone is editable after the ruling, so it cannot bind the comment.
// regression: #2079
func TestRulingRecord_OtherBrief(t *testing.T) {
	named := "Approved: " + rlRecordID + " for " + rrBriefID + "."
	for name, tc := range map[string]struct{ issue, comment string }{
		"unrelated":   {issue: rlMarker, comment: rlRulingText},
		"marker-only": {issue: rlMarker + "\n" + rrBriefMarker, comment: rlRulingText},
		"named-only":  {issue: rlMarker, comment: named},
		"brief-issue": {issue: rrBriefMarker, comment: named},
		"other-brief": {issue: rlMarker + "\n<!-- decision-gate: example-stream/02 -->", comment: "Approved: " + rlRecordID + " for example-stream/02."},
	} {
		t.Run(name, func(t *testing.T) {
			f := defaultRLForge()
			f.issueBody = rlIssueJSON("A human decision is needed.\n\n" + tc.issue + "\n")
			f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, tc.comment, rlPosted, rlPosted)
			root := rrSetup(t, f)
			if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 1 {
				t.Fatalf("a ruling not bound to %s must refuse (exit 1), got %d", rrBriefID, got)
			}
		})
	}
}

// A zero-padded board id and its unpadded spelling are one brief.
// regression: #2079
func TestRulingRecord_BriefSpelling(t *testing.T) {
	f := rrBoundForge()
	f.issueBody = rlIssueJSON(rlMarker + "\n<!-- decision-gate: example-stream/1 -->\n")
	f.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue,
		"Approved: "+rlRecordID+" for example-stream/1.", rlPosted, rlPosted)
	root := rrSetup(t, f)
	if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != 0 {
		t.Fatalf("example-stream/1 names %s: want exit 0, got %d", rrBriefID, got)
	}
}

// --ruling-brief must name a brief file in this checkout; anything else is a
// usage error (exit 2), never a pass.
// regression: #2079
func TestRulingRecord_BriefUsage(t *testing.T) {
	root := rrSetup(t, rrBoundForge())
	for _, b := range []string{
		"",
		"docs/streams/example-stream/brief-09-absent.md",
		"docs/streams/example-stream/notes.md",
		"docs/streams/example-stream/../../../x/brief-01-x.md",
		"/abs/docs/streams/example-stream/brief-01-example.md",
	} {
		if got := runCorroborateRecord(root, rlFile, b); got != 2 {
			t.Errorf("--ruling-brief %q: want exit 2, got %d", b, got)
		}
	}
}
