package main

// labelremovalhold_test.go — the label-removal hold (#2414): the pure decision, the deskread
// reader, the default checker, and both scanners' use of it.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// labelHoldClearAll is the clear-everything checker the pre-existing scan tests pass, so their
// assertions keep reading exactly what they read before the hold existed. It is defined in a
// test file ONLY: production code must never carry a checker that clears everything.
func labelHoldClearAll(string, int) (labelHoldVerdict, error) { return labelHoldVerdict{}, nil }

// --- fixtures -----------------------------------------------------------------

// holdBase is the clock the hold fixtures count hours from.
var holdBase = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// hr renders "n hours after the base" as the forge would.
func hr(n float64) string {
	return holdBase.Add(time.Duration(n * float64(time.Hour))).Format(time.RFC3339)
}

const (
	appLogin   = "assay-worker-app[bot]"
	appID      = int64(300000006)
	ratifier   = "ada"
	ratifierID = int64(100001)
	otherHuman = "dana"
	otherID    = int64(200002)
)

func evApply(label string, at float64, login string, id int64) labelEvent {
	return labelEvent{Label: label, Removed: false, At: hr(at), Login: login, ID: id}
}

func evRemove(label string, at float64, login string, id int64) labelEvent {
	return labelEvent{Label: label, Removed: true, At: hr(at), Login: login, ID: id}
}

func ratifying(at float64) issueComment {
	return issueComment{User: issueCommentUser{Login: ratifier, ID: ratifierID, Type: "User"}, CreatedAt: hr(at)}
}

func commentBy(login string, id int64, at float64) issueComment {
	return issueComment{User: issueCommentUser{Login: login, ID: id, Type: "User"}, CreatedAt: hr(at)}
}

// holdRoster arms a roster with a blessing authority (ada), one other authorized human (dana)
// and the worker App.
func holdRoster(t *testing.T) {
	t.Helper()
	scanWithRoster(t, transcribeRoster())
}

// --- the pure decision -----------------------------------------------------------

func TestLabelHoldDecision(t *testing.T) {
	holdRoster(t)
	const nd = "needs-decision"
	type want int
	const (
		clear want = iota
		held
		cnc
	)
	cases := []struct {
		name        string
		events      []labelEvent
		incomplete  bool
		comments    []issueComment
		commentsErr bool
		want        want
		label       string // expected label on a hold ("" = not asserted)
		noRead      bool   // the comments must not be read at all
	}{
		{name: "removed by an App, no comment",
			events: []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			want:   held, label: nd},
		{name: "removed by the ratifying identity",
			events: []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, ratifier, ratifierID)},
			want:   clear, noRead: true},
		{name: "removed by another rostered human",
			events: []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, otherHuman, otherID)},
			want:   clear, noRead: true},
		{name: "App removal, ratifying comment between application and removal",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{ratifying(2)}, want: clear},
		{name: "App removal, ratifying comment after the removal",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{ratifying(4)}, want: clear},
		{name: "the only ratifying comment predates a later re-application",
			events: []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID),
				evApply(nd, 5, appLogin, appID), evRemove(nd, 6, appLogin, appID)},
			comments: []issueComment{ratifying(2)}, want: held, label: nd},
		{name: "ratifying login with a different id",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{commentBy(ratifier, 31337, 4)}, want: held},
		{name: "comment by another rostered human",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{commentBy(otherHuman, otherID, 4)}, want: held},
		{name: "trusted desk identity rendered app/<slug> (no [bot] suffix)",
			events: []labelEvent{evApply(nd, 1, "app/assay-worker-app", appID),
				evRemove(nd, 3, "app/assay-worker-app", appID)},
			want: held, label: nd},
		{name: "removing actor has the ratifying login and id 0",
			events: []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, ratifier, 0)},
			want:   held},
		{name: "removing actor has the ratifying login and a different id",
			events: []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, ratifier, 31337)},
			want:   held},
		{name: "same creation time as the latest application",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{ratifying(1)}, want: held},
		{name: "same creation time as the removal where no application is recorded",
			events:   []labelEvent{evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{ratifying(3)}, want: held},
		{name: "same creation time as the latest unnamed event",
			events:   []labelEvent{{Label: "", Removed: true, At: hr(3), Login: appLogin, ID: appID}},
			comments: []issueComment{ratifying(3)}, want: held},
		{name: "no application recorded, ratifying comment before the removal",
			events:   []labelEvent{evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{ratifying(2)}, want: held},
		{name: "no application recorded, ratifying comment after the removal",
			events:   []labelEvent{evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{ratifying(4)}, want: clear},
		{name: "last event is an application and the label is absent",
			events: []labelEvent{evApply(nd, 1, appLogin, appID)},
			want:   held, label: nd},
		{name: "last event is an application, ratifying comment after it does not release",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID)},
			comments: []issueComment{ratifying(5)}, want: held, label: nd},
		{name: "a later removal by a rostered human releases the application",
			events: []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 2, otherHuman, otherID)},
			want:   clear},
		{name: "unnamed event, no comment",
			events: []labelEvent{{Label: "", Removed: true, At: hr(3), Login: appLogin, ID: appID}},
			want:   held},
		{name: "unnamed event, ratifying comment before it",
			events:   []labelEvent{{Label: "", Removed: true, At: hr(3), Login: appLogin, ID: appID}},
			comments: []issueComment{ratifying(2)}, want: held},
		{name: "unnamed event, ratifying comment after it",
			events:   []labelEvent{{Label: "", Removed: true, At: hr(3), Login: appLogin, ID: appID}},
			comments: []issueComment{ratifying(4)}, want: clear},
		{name: "two unnamed events with the comment between them",
			events: []labelEvent{{Label: "", Removed: true, At: hr(3), Login: appLogin, ID: appID},
				{Label: "", Removed: false, At: hr(5), Login: appLogin, ID: appID}},
			comments: []issueComment{ratifying(4)}, want: held},
		{name: "history marked incomplete, otherwise clear, with a ratifying comment",
			events:     []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, ratifier, ratifierID)},
			incomplete: true, comments: []issueComment{ratifying(4)}, want: cnc, noRead: true},
		{name: "no event for any excluded label and no unnamed event",
			events: []labelEvent{evApply("bug", 1, appLogin, appID), evRemove("bug", 2, appLogin, appID)},
			want:   clear, noRead: true},
		{name: "empty history",
			want: clear, noRead: true},
		{name: "two excluded labels, one clear and one not",
			events: []labelEvent{
				evApply(nd, 1, appLogin, appID), evRemove(nd, 2, otherHuman, otherID),
				evApply("verify-gate", 1, appLogin, appID), evRemove("verify-gate", 2, appLogin, appID)},
			want: held, label: "verify-gate"},
		{name: "label names compare case-insensitively",
			events: []labelEvent{evApply("Needs-Decision", 1, appLogin, appID), evRemove("Needs-Decision", 2, appLogin, appID)},
			want:   held, label: nd},
		{name: "an unparsable event time",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID), {Label: nd, Removed: true, At: "garbage", Login: appLogin, ID: appID}},
			comments: []issueComment{ratifying(4)}, want: held},
		{name: "an unparsable ratifying comment time",
			events:   []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			comments: []issueComment{{User: issueCommentUser{Login: ratifier, ID: ratifierID}, CreatedAt: "garbage"}}, want: held},
		{name: "comments unreadable when a comparison needs them",
			events:      []labelEvent{evApply(nd, 1, appLogin, appID), evRemove(nd, 3, appLogin, appID)},
			commentsErr: true, want: cnc},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reads := 0
			read := func() ([]issueComment, error) {
				reads++
				if c.commentsErr {
					return nil, errFixture
				}
				return c.comments, nil
			}
			v := decideLabelHold(scanExcludedLabelSet(), c.events, !c.incomplete, read)
			got := clear
			switch {
			case v.CouldNotCheck:
				got = cnc
			case v.Held:
				got = held
			}
			if got != c.want {
				t.Fatalf("verdict = %+v, want state %d (0 clear, 1 held, 2 could-not-check)", v, c.want)
			}
			if c.want != clear && !v.IsHold() {
				t.Errorf("a hold must report IsHold: %+v", v)
			}
			if c.want == held && c.label != "" && v.Label != c.label {
				t.Errorf("hold names label %q, want %q", v.Label, c.label)
			}
			if c.want == held && v.Detail == "" {
				t.Error("a hold carries the sentence saying what holds it and what releases it")
			}
			if c.noRead && reads != 0 {
				t.Errorf("the comments were read %d time(s); this case must not read them", reads)
			}
			if reads > 1 {
				t.Errorf("the comments were read %d times, want at most one", reads)
			}
		})
	}
}

// TestLabelHoldBotRenderings: one App id rendered three ways as the removing actor is held in
// every rendering, the hold message carries the login exactly as rendered, and the same three
// with a rostered human's id (under a login that is not that human's) are held too. The
// decision never classifies by login shape.
func TestLabelHoldBotRenderings(t *testing.T) {
	holdRoster(t)
	for _, login := range []string{"assay-worker-app", "assay-worker-app[bot]", "app/assay-worker-app"} {
		for _, id := range []int64{appID, otherID} {
			t.Run(fmt.Sprintf("%s/%d", login, id), func(t *testing.T) {
				events := []labelEvent{
					evApply("needs-decision", 1, login, id),
					evRemove("needs-decision", 3, login, id),
				}
				v := decideLabelHold(scanExcludedLabelSet(), events, true, func() ([]issueComment, error) { return nil, nil })
				if !v.Held || v.CouldNotCheck {
					t.Fatalf("verdict = %+v, want held", v)
				}
				if v.Actor != login || !strings.Contains(v.Detail, login) {
					t.Errorf("hold actor %q / detail %q must carry the login exactly as rendered (%q)", v.Actor, v.Detail, login)
				}
			})
		}
	}
}

// --- the deskread reader -------------------------------------------------------------

// labelStub is a deskread stand-in for the label-events and comments kinds (and, for the
// end-to-end tests, issues). It is driven by environment variables so one script serves every
// scenario:
//
//	STUB_HISTORY  good-none | bot | human | unnamed | incomplete | nocomplete | partial | error | rejected | empty | malformed
//	STUB_COMMENTS none | ratified | error
//	STUB_LOG      file each invoked kind is appended to
const labelStub = `#!/bin/sh
kind="$1"
arg="$3"
if [ -n "$STUB_LOG" ]; then echo "$kind" >> "$STUB_LOG"; fi
repo="${arg%#*}"; num="${arg##*#}"
case "$kind" in
issues)
  cat <<EOF
{"schema":1,"kind":"issues","repos":[{"repo":"$arg","issues":[
 {"number":610,"title":"a report","state":"open","authorLogin":"ada","labels":["bug"],"url":"https://example.test/610"}
]}],"partial":[]}
EOF
  ;;
label-events)
  case "$STUB_HISTORY" in
  error) echo "boom" >&2; exit 1 ;;
  rejected) echo "deskread: unknown kind label-events" >&2; exit 2 ;;
  partial)
    cat <<EOF
{"schema":1,"kind":"label-events","items":[],"partial":[{"repo":"$repo","number":$num,"reason":"the App installation cannot read it"}]}
EOF
    exit 6 ;;
  empty) echo '{"schema":1,"kind":"label-events","items":[],"partial":[]}' ;;
  malformed) echo 'not json' ;;
  nocomplete)
    cat <<EOF
{"schema":1,"kind":"label-events","items":[{"repo":"$repo","number":$num,"labelEvents":{"events":[]}}],"partial":[]}
EOF
    ;;
  incomplete)
    cat <<EOF
{"schema":1,"kind":"label-events","items":[{"repo":"$repo","number":$num,"labelEvents":{"complete":false,"events":[
 {"label":"needs-decision","removed":false,"createdAt":"2026-10-01T01:00:00Z","actorLogin":"ada","actorId":100001},
 {"label":"needs-decision","removed":true,"createdAt":"2026-10-01T03:00:00Z","actorLogin":"ada","actorId":100001}
]}}],"partial":[]}
EOF
    ;;
  good-none)
    cat <<EOF
{"schema":1,"kind":"label-events","items":[{"repo":"$repo","number":$num,"labelEvents":{"complete":true,"events":[
 {"label":"bug","removed":false,"createdAt":"2026-10-01T01:00:00Z","actorLogin":"assay-worker-app[bot]","actorId":300000006}
]}}],"partial":[]}
EOF
    ;;
  bot)
    cat <<EOF
{"schema":1,"kind":"label-events","items":[{"repo":"$repo","number":$num,"labelEvents":{"complete":true,"events":[
 {"label":"needs-decision","removed":false,"createdAt":"2026-10-01T01:00:00Z","actorLogin":"assay-worker-app[bot]","actorId":300000006},
 {"label":"needs-decision","removed":true,"createdAt":"2026-10-01T03:00:00Z","actorLogin":"assay-worker-app[bot]","actorId":300000006}
]}}],"partial":[]}
EOF
    ;;
  human)
    cat <<EOF
{"schema":1,"kind":"label-events","items":[{"repo":"$repo","number":$num,"labelEvents":{"complete":true,"events":[
 {"label":"needs-decision","removed":false,"createdAt":"2026-10-01T01:00:00Z","actorLogin":"assay-worker-app[bot]","actorId":300000006},
 {"label":"needs-decision","removed":true,"createdAt":"2026-10-01T03:00:00Z","actorLogin":"ada","actorId":100001}
]}}],"partial":[]}
EOF
    ;;
  unnamed)
    cat <<EOF
{"schema":1,"kind":"label-events","items":[{"repo":"$repo","number":$num,"labelEvents":{"complete":true,"events":[
 {"label":"","removed":true,"createdAt":"2026-10-01T03:00:00Z","actorLogin":"assay-worker-app[bot]","actorId":300000006}
]}}],"partial":[]}
EOF
    ;;
  esac
  ;;
comments)
  case "$STUB_COMMENTS" in
  error) echo "boom" >&2; exit 1 ;;
  ratified)
    cat <<EOF
{"schema":1,"kind":"comments","items":[{"repo":"$repo","number":$num,"comments":[
 {"authorLogin":"ada","authorId":100001,"createdAt":"2026-10-01T04:00:00Z","body":"go ahead"}
]}],"partial":[]}
EOF
    ;;
  *)
    cat <<EOF
{"schema":1,"kind":"comments","items":[{"repo":"$repo","number":$num,"comments":[]}],"partial":[]}
EOF
    ;;
  esac
  ;;
*) echo "unexpected kind $kind" >&2; exit 5 ;;
esac
`

// useLabelStub puts the stub first on PATH (with a failing gh), picks the scenarios and
// returns the path of the invocation log.
func useLabelStub(t *testing.T, history, comments string) string {
	t.Helper()
	bin := t.TempDir()
	writeStubBin(t, bin, "gh", "#!/bin/sh\necho 'gh: stubbed to fail' >&2\nexit 1\n")
	writeStubBin(t, bin, "deskread", labelStub)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	log := filepath.Join(t.TempDir(), "stub.log")
	t.Setenv("STUB_LOG", log)
	t.Setenv("STUB_HISTORY", history)
	t.Setenv("STUB_COMMENTS", comments)
	return log
}

func stubKinds(t *testing.T, log string) []string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		return nil
	}
	return strings.Fields(string(b))
}

func TestLabelHistoryUnreadShapes(t *testing.T) {
	const repo = "example-org/tracker"
	// A good envelope decodes with login, id, time and the completeness flag.
	useLabelStub(t, "human", "none")
	h, err := newDeskreadReader().IssueLabelHistory(repo, 7)
	if err != nil {
		t.Fatalf("good envelope: %v", err)
	}
	if !h.Complete || len(h.Events) != 2 {
		t.Fatalf("good envelope decoded as %+v", h)
	}
	last := h.Events[1]
	if last.Label != "needs-decision" || !last.Removed || last.Login != "ada" || last.ID != 100001 || last.At != "2026-10-01T03:00:00Z" {
		t.Errorf("event decoded as %+v", last)
	}

	// An item marked incomplete decodes as incomplete, not as an error and not as complete.
	useLabelStub(t, "incomplete", "none")
	h, err = newDeskreadReader().IssueLabelHistory(repo, 7)
	if err != nil || h.Complete {
		t.Errorf("incomplete item: %+v, %v — want decoded as incomplete without an error", h, err)
	}

	// An unnamed event keeps its empty label.
	useLabelStub(t, "unnamed", "none")
	h, err = newDeskreadReader().IssueLabelHistory(repo, 7)
	if err != nil || len(h.Events) != 1 || h.Events[0].Label != "" {
		t.Errorf("unnamed event: %+v, %v", h, err)
	}

	for _, bad := range []string{"partial", "empty", "nocomplete", "error", "rejected", "malformed"} {
		t.Run(bad, func(t *testing.T) {
			useLabelStub(t, bad, "none")
			h, err := newDeskreadReader().IssueLabelHistory(repo, 7)
			if err == nil {
				t.Fatalf("%s: read %+v with no error — an unread history must never look like an empty one", bad, h)
			}
			if len(h.Events) != 0 {
				t.Errorf("%s: an error must not carry events: %+v", bad, h)
			}
		})
	}
}

// --- the default checker ----------------------------------------------------------------

func TestLabelHoldDefaultChecker(t *testing.T) {
	holdRoster(t)
	const repo = "example-org/tracker"
	for _, tc := range []struct {
		name, history, comments string
	}{
		{"an erroring label-history read", "error", "none"},
		{"an issue listed in partial", "partial", "none"},
		{"a history marked incomplete", "incomplete", "ratified"},
		{"a deskread that rejects the kind", "rejected", "none"},
		{"a history with no completeness field", "nocomplete", "none"},
		{"a good history then an unreadable comment read", "bot", "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useLabelStub(t, tc.history, tc.comments)
			v, err := defaultScanLabelHoldChecker(repo, 7)
			if err != nil {
				t.Fatalf("the default checker reports a read failure as a verdict, not an error: %v", err)
			}
			if !v.CouldNotCheck || !v.IsHold() || v.Clause() != labelHoldClauseCNC {
				t.Errorf("verdict = %+v, want held could-not-check", v)
			}
			if v.Detail == "" {
				t.Error("a could-not-check hold names why")
			}
		})
	}

	t.Run("a history with no excluded-label event reads no comments", func(t *testing.T) {
		log := useLabelStub(t, "good-none", "error")
		v, err := defaultScanLabelHoldChecker(repo, 7)
		if err != nil || v.IsHold() {
			t.Fatalf("verdict = %+v, %v — want clear", v, err)
		}
		for _, k := range stubKinds(t, log) {
			if k == "comments" {
				t.Errorf("a comment read happened for an issue with no excluded-label event; kinds = %v", stubKinds(t, log))
			}
		}
	})

	t.Run("history is read before comments", func(t *testing.T) {
		log := useLabelStub(t, "bot", "ratified")
		v, err := defaultScanLabelHoldChecker(repo, 7)
		if err != nil || v.IsHold() {
			t.Fatalf("verdict = %+v, %v — want clear (ratified after the application)", v, err)
		}
		kinds := stubKinds(t, log)
		if len(kinds) != 2 || kinds[0] != "label-events" || kinds[1] != "comments" {
			t.Errorf("read order = %v, want [label-events comments]", kinds)
		}
	})

	t.Run("a bot removal with no ratifying comment is held, not could-not-check", func(t *testing.T) {
		useLabelStub(t, "bot", "none")
		v, err := defaultScanLabelHoldChecker(repo, 7)
		if err != nil || !v.Held || v.CouldNotCheck || v.Label != "needs-decision" || v.Actor != "assay-worker-app[bot]" {
			t.Errorf("verdict = %+v, %v", v, err)
		}
	})
}

// --- the gate -----------------------------------------------------------------------------

func TestLabelHoldGateFoldsEveryShape(t *testing.T) {
	if held, clause, _ := labelHoldGate(nil, "r/x", 1); !held || clause != labelHoldClauseCNC {
		t.Errorf("nil checker: held=%v clause=%q, want held could-not-check", held, clause)
	}
	errCheck := func(string, int) (labelHoldVerdict, error) { return labelHoldVerdict{}, errFixture }
	if held, clause, _ := labelHoldGate(errCheck, "r/x", 1); !held || clause != labelHoldClauseCNC {
		t.Errorf("erroring checker: held=%v clause=%q", held, clause)
	}
	if held, _, _ := labelHoldGate(labelHoldClearAll, "r/x", 1); held {
		t.Error("a clear verdict must not hold")
	}
	// A could-not-check verdict that forgot to set Held is still a hold.
	cncOnly := func(string, int) (labelHoldVerdict, error) {
		return labelHoldVerdict{CouldNotCheck: true, Detail: "x"}, nil
	}
	if held, clause, _ := labelHoldGate(cncOnly, "r/x", 1); !held || clause != labelHoldClauseCNC {
		t.Errorf("could-not-check verdict: held=%v clause=%q", held, clause)
	}
}

// --- scan-level: planScan ------------------------------------------------------------------

// holdChecker serves a fixed verdict per "repo#issue" and counts calls.
func holdChecker(v labelHoldVerdict) labelHoldChecker {
	return func(string, int) (labelHoldVerdict, error) { return v, nil }
}

var heldVerdict = labelHoldVerdict{
	Held: true, Label: "needs-decision", Actor: "assay-worker-app[bot]",
	Detail: `excluded label "needs-decision" was removed by assay-worker-app[bot], who is not a declared human; it is released when the ratifying identity comments on the issue`,
}

// holdPaths builds one fixture per admission path for planScan, each with a single issue (no
// excluded label now) whose state would, absent the hold, produce a plan.
type holdPath struct {
	name   string
	issue  int
	setup  func(t *testing.T) ([]*Stream, string)
	plan   func(plans []scanPlan, closeOuts []closeOutPlan) bool // true = the admitting plan exists
	action string
}

func archivedHoldFixture(t *testing.T, issue int) ([]*Stream, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "docs/streams", scanStreamName)
	done := filepath.Join(dir, archiveDirName)
	if err := os.MkdirAll(done, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, dir, "README.md", scanStreamREADME)
	writeTemp(t, done, placeholderFileName(scanHomeRepo(), issue), closedPlaceholderFile(scanStreamName, issue, "bug"))
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	attachPlaceholders(streams)
	return streams, root
}

func holdPaths() []holdPath {
	return []holdPath{
		{name: "create", issue: 700, action: "created",
			setup: func(t *testing.T) ([]*Stream, string) { return scanFixtureRepo(t, nil) },
			plan: func(plans []scanPlan, _ []closeOutPlan) bool {
				return findPlan(plans, scanHomeRepo(), 700) != nil
			}},
		{name: "root reactivate", issue: 701, action: "reactivated",
			setup: func(t *testing.T) ([]*Stream, string) {
				return scanFixtureRepo(t, map[string]string{"issue-701.md": labelExcludedPlaceholderFile("alpha", 701, "bug, needs-decision")})
			},
			plan: func(_ []scanPlan, closeOuts []closeOutPlan) bool {
				c := findCloseOut(closeOuts, scanHomeRepo(), 701)
				return c != nil && c.Action == "reactivate"
			}},
		{name: "archive reactivate", issue: 702, action: "reactivated",
			setup: func(t *testing.T) ([]*Stream, string) { return archivedHoldFixture(t, 702) },
			plan: func(_ []scanPlan, closeOuts []closeOutPlan) bool {
				c := findCloseOut(closeOuts, scanHomeRepo(), 702)
				return c != nil && c.Action == "reactivate"
			}},
	}
}

func holdIssueData(issue int) map[string][]ghIssue {
	return map[string][]ghIssue{
		scanHomeRepo(): {{Number: issue, Title: "an issue", Labels: lbl("bug"), Author: ghAuthor{Login: "ada"}}},
	}
}

func holdNoticeCount(ss []string, sub string) int {
	n := 0
	for _, s := range ss {
		if strings.Contains(s, sub) {
			n++
		}
	}
	return n
}

func TestScanLabelHold(t *testing.T) {
	holdRoster(t)
	for _, p := range holdPaths() {
		t.Run(p.name+"/held", func(t *testing.T) {
			streams, root := p.setup(t)
			plans, closeOuts, notices, _ := planScan(root, streams, fixtureLister(holdIssueData(p.issue), ""), blessAll, holdChecker(heldVerdict))
			if p.plan(plans, closeOuts) {
				t.Fatalf("a held issue produced its admitting plan: plans=%+v closeOuts=%+v", plans, closeOuts)
			}
			if findPlan(plans, scanHomeRepo(), p.issue) != nil || findCloseOut(closeOuts, scanHomeRepo(), p.issue) != nil {
				t.Errorf("a held issue produced a plan or close-out: plans=%+v closeOuts=%+v", plans, closeOuts)
			}
			if n := holdNoticeCount(notices, fmt.Sprintf("%s#%d", scanHomeRepo(), p.issue)); n != 1 {
				t.Fatalf("want exactly one NOTICE naming the issue, got %d in %v", n, notices)
			}
			for _, n := range notices {
				if strings.Contains(n, fmt.Sprintf("#%d", p.issue)) {
					for _, want := range []string{labelHoldClause, "needs-decision", "NOT " + p.action} {
						if !strings.Contains(n, want) {
							t.Errorf("NOTICE %q must carry %q", n, want)
						}
					}
				}
			}
		})
		t.Run(p.name+"/clear", func(t *testing.T) {
			streams, root := p.setup(t)
			plans, closeOuts, notices, _ := planScan(root, streams, fixtureLister(holdIssueData(p.issue), ""), blessAll, holdChecker(labelHoldVerdict{}))
			if !p.plan(plans, closeOuts) {
				t.Fatalf("a clear issue must get the plan it gets today: plans=%+v closeOuts=%+v", plans, closeOuts)
			}
			if n := holdNoticeCount(notices, labelHoldClause); n != 0 {
				t.Errorf("a clear issue draws no hold NOTICE, got %v", notices)
			}
		})
	}
}

func TestScanLabelHoldFailsClosed(t *testing.T) {
	holdRoster(t)
	errChecker := func(string, int) (labelHoldVerdict, error) { return labelHoldVerdict{}, errFixture }
	for _, p := range holdPaths() {
		for _, c := range []struct {
			name  string
			check labelHoldChecker
		}{{"error", errChecker}, {"nil", nil}} {
			t.Run(p.name+"/"+c.name, func(t *testing.T) {
				streams, root := p.setup(t)
				plans, closeOuts, notices, _ := planScan(root, streams, fixtureLister(holdIssueData(p.issue), ""), blessAll, c.check)
				if p.plan(plans, closeOuts) || findPlan(plans, scanHomeRepo(), p.issue) != nil || findCloseOut(closeOuts, scanHomeRepo(), p.issue) != nil {
					t.Fatalf("a %s checker must produce no plan: plans=%+v closeOuts=%+v", c.name, plans, closeOuts)
				}
				if n := holdNoticeCount(notices, "could-not-check"); n != 1 {
					t.Errorf("want one could-not-check NOTICE, got %d in %v", n, notices)
				}
			})
		}
	}
}

// TestScanLabelHoldKeepsDone: a held placeholder stays `done` through plan and apply; once the
// check clears, the same sequence leaves it `todo`.
func TestScanLabelHoldKeepsDone(t *testing.T) {
	holdRoster(t)
	const issue = 710
	run := func(t *testing.T, check labelHoldChecker) string {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(root, "docs/streams", scanStreamName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, dir, "README.md", scanStreamREADME)
		writeTemp(t, dir, "issue-710.md", labelExcludedPlaceholderFile(scanStreamName, issue, "bug, needs-decision"))
		var code int
		captureRun(t, func() int {
			code = runScanIssues(root, false, fixtureLister(holdIssueData(issue), ""), nilCommentLister, blessAll, check)
			return code
		})
		if code != 0 {
			t.Fatalf("scan exited %d", code)
		}
		ph, ok, err := parsePlaceholderFile(filepath.Join(dir, "issue-710.md"))
		if err != nil || !ok {
			t.Fatalf("re-parse: ok=%v err=%v", ok, err)
		}
		return ph.Status
	}
	if got := run(t, holdChecker(heldVerdict)); got != "done" {
		t.Errorf("held: placeholder status = %q, want it to still read done", got)
	}
	if got := run(t, holdChecker(labelHoldVerdict{})); got != "todo" {
		t.Errorf("clear: placeholder status = %q, want todo", got)
	}
}

// --- scan-level: planTranscribeScan ----------------------------------------------------------

func TestTranscribeLabelHold(t *testing.T) {
	holdRoster(t)
	home := "example-org/tracker"
	authors := func(n int) authorResolver {
		return fixtureAuthorResolver(map[string]authorIdentity{
			fmt.Sprintf("%s#%d", home, n): {Login: "ada", ID: 100001, Type: "User"},
		})
	}
	paths := []struct {
		name  string
		issue int
		setup func(t *testing.T) ([]*Stream, string)
		plan  func(d transcribeDelta, issue int) bool
	}{
		{"create", 800, func(t *testing.T) ([]*Stream, string) { return scanFixtureRepo(t, nil) },
			func(d transcribeDelta, n int) bool {
				for _, p := range d.Creates {
					if p.Issue == n {
						return true
					}
				}
				return false
			}},
		{"root reactivate", 801, func(t *testing.T) ([]*Stream, string) {
			return scanFixtureRepo(t, map[string]string{"issue-801.md": labelExcludedPlaceholderFile("alpha", 801, "bug, needs-decision")})
		}, func(d transcribeDelta, n int) bool {
			c := findCloseOut(d.CloseOuts, home, n)
			return c != nil && c.Action == "reactivate"
		}},
		{"archive reactivate", 802, func(t *testing.T) ([]*Stream, string) { return archivedHoldFixture(t, 802) },
			func(d transcribeDelta, n int) bool {
				c := findCloseOut(d.CloseOuts, home, n)
				return c != nil && c.Action == "reactivate"
			}},
	}
	errChecker := func(string, int) (labelHoldVerdict, error) { return labelHoldVerdict{}, errFixture }
	for _, p := range paths {
		list := fixtureLister(holdIssueData(p.issue), "")
		plan := func(t *testing.T, check labelHoldChecker) transcribeDelta {
			streams, root := p.setup(t)
			d, err := planTranscribeScan(root, streams, home, list, nilCommentLister, authors(p.issue), blessNone, check)
			if err != nil {
				t.Fatal(err)
			}
			return d
		}
		t.Run(p.name+"/held", func(t *testing.T) {
			d := plan(t, holdChecker(heldVerdict))
			if p.plan(d, p.issue) {
				t.Fatalf("a held issue was admitted: %+v", d)
			}
			if len(d.Skips) != 1 || d.Skips[0].Issue != p.issue || d.Skips[0].Clause != "label-removal-hold" {
				t.Fatalf("skips = %+v, want one label-removal-hold skip for #%d", d.Skips, p.issue)
			}
			if !strings.Contains(d.Skips[0].Reason, "needs-decision") {
				t.Errorf("skip reason %q must name the label", d.Skips[0].Reason)
			}
		})
		t.Run(p.name+"/clear", func(t *testing.T) {
			d := plan(t, holdChecker(labelHoldVerdict{}))
			if !p.plan(d, p.issue) {
				t.Fatalf("a clear issue must be admitted as today: %+v", d)
			}
			if len(d.Skips) != 0 {
				t.Errorf("a clear issue draws no skip: %+v", d.Skips)
			}
		})
		for _, c := range []struct {
			name  string
			check labelHoldChecker
		}{{"error", errChecker}, {"nil", nil}} {
			t.Run(p.name+"/"+c.name, func(t *testing.T) {
				d := plan(t, c.check)
				if p.plan(d, p.issue) {
					t.Fatalf("a %s checker admitted the issue: %+v", c.name, d)
				}
				if len(d.Skips) != 1 || d.Skips[0].Clause != "label-removal-hold (could-not-check)" {
					t.Errorf("skips = %+v, want one could-not-check skip", d.Skips)
				}
			})
		}
	}
}

// --- end to end ------------------------------------------------------------------------------

func TestScanIssuesLabelHoldE2E(t *testing.T) {
	scanOnlyHomeRepo(t)
	repo := scanHomeRepo()
	const issue = 610
	root := func(t *testing.T) (string, string) {
		r := t.TempDir()
		if err := os.CopyFS(r, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(r, "docs/streams", scanStreamName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, dir, "README.md", scanStreamREADME)
		return r, filepath.Join(dir, placeholderFileName(repo, issue))
	}
	scan := func(t *testing.T, history string) (string, string, bool) {
		useLabelStub(t, history, "none")
		r, ph := root(t)
		out := captureRun(t, func() int {
			return runScanIssues(r, false, defaultScanIssueLister, defaultScanCommentLister, defaultScanBlessChecker, defaultScanLabelHoldChecker)
		})
		_, err := os.Stat(ph)
		return out.log, out.err, err == nil
	}

	log, errOut, wrote := scan(t, "bot")
	if wrote {
		t.Errorf("a label removed by an App with no ratifying comment wrote a placeholder; stdout:\n%s", log)
	}
	if !strings.Contains(errOut, labelHoldClause) || !strings.Contains(errOut, "needs-decision") {
		t.Errorf("want the hold NOTICE naming the label, got stderr:\n%s", errOut)
	}

	if log, errOut, wrote = scan(t, "human"); !wrote {
		t.Errorf("a label removed by a rostered human must write the placeholder; stdout:\n%s\nstderr:\n%s", log, errOut)
	}

	log, errOut, wrote = scan(t, "rejected")
	if wrote {
		t.Errorf("a deskread that rejects the kind wrote a placeholder; stdout:\n%s", log)
	}
	if !strings.Contains(errOut, "could-not-check") {
		t.Errorf("want a could-not-check NOTICE, got stderr:\n%s", errOut)
	}
}

func TestTranscribeLabelHoldE2E(t *testing.T) {
	holdRoster(t)
	home := "example-org/tracker"
	const issue = 811
	list := fixtureLister(holdIssueData(issue), "")
	authors := fixtureAuthorResolver(map[string]authorIdentity{
		fmt.Sprintf("%s#%d", home, issue): {Login: "ada", ID: 100001, Type: "User"},
	})
	run := func(t *testing.T) (capturedRun, bool) {
		_, root := scanFixtureRepo(t, nil)
		writeRulings(t, root, r7Armed)
		out := captureRun(t, func() int {
			return runTranscribeScan(root, false, list, nilCommentLister, authors, blessNone, defaultScanLabelHoldChecker, blessAuthorityResolver)
		})
		_, err := os.Stat(filepath.Join(root, "docs/streams", scanStreamName, placeholderFileName(home, issue)))
		return out, err == nil
	}

	useLabelStub(t, "bot", "none")
	out, wrote := run(t)
	if wrote || !strings.Contains(out.log, "SKIP "+home+"#811 — label-removal-hold:") {
		t.Errorf("a bot-removed label must hold (wrote=%v):\n%s", wrote, out.log)
	}

	useLabelStub(t, "human", "none")
	if out, wrote = run(t); !wrote {
		t.Errorf("a human-removed label must create:\n%s", out.log)
	}

	// No deskread on PATH at all: nothing is created, and the skip is the could-not-check clause.
	t.Setenv("PATH", t.TempDir())
	out, wrote = run(t)
	if wrote || !strings.Contains(out.log, "label-removal-hold (could-not-check)") {
		t.Errorf("with no deskread on PATH nothing may be created and the skip must be could-not-check (wrote=%v):\n%s", wrote, out.log)
	}
}

// TestLabelHoldMainWiring pins both entry-point calls in main.go to the default checker, and
// the clear-everything checker to a test file.
func TestLabelHoldMainWiring(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, call := range []string{"runScanIssues(", "runTranscribeScan("} {
		i := strings.Index(text, call)
		if i < 0 {
			t.Fatalf("main.go: no %s call", call)
		}
		j := strings.Index(text[i:], "))")
		if j < 0 {
			t.Fatalf("main.go: unterminated %s call", call)
		}
		if !strings.Contains(text[i:i+j], "defaultScanLabelHoldChecker") {
			t.Errorf("main.go: the %s call does not pass defaultScanLabelHoldChecker: %s", call, text[i:i+j+2])
		}
	}
}
