package main

// scanissues_nativetrust_test.go — the #1255 acceptance proof.
//
// #1223 moved the --scan-issues OPEN-issue list read off `gh` and onto the native forge (via the
// `deskread` verb). It left the path's two other forge reads shelling `gh`: the trust-gate bless
// read (`gh api graphql`) and the un-block comment read (`gh api --paginate`). scanloop's scan
// lane runs `statusgen --scan-issues`, so under scanloop's replaced HOME those two reads kept
// 401ing on every rostered repo. #1255 routes both through `deskread` (the `trust` and
// `comments` kinds).
//
// The proof is the whole PRODUCTION --scan-issues read set — the three default* vars main wires —
// driven end to end with `gh` on PATH stubbed to FAIL and `deskread` serving the native envelope:
// an untrusted-author issue carrying a current blessing gets its placeholder, and a blocked
// placeholder whose issue the blessing authority answered is un-blocked.
//
// FAIL-FIRST. On the unfixed tree the production bless and comment reads are ghIssueBlessChecker
// and issueCommentLister. With `gh` failing, the bless read errors (→ "trust gate unverifiable",
// no placeholder) and the comment read errors (→ NOTICE, placeholder stays blocked), so both
// outcome assertions below go red. The mutations that re-redden the fixed tree are reverting
// either default (`defaultScanBlessChecker = ghIssueBlessChecker`,
// `defaultScanCommentLister = issueCommentLister`).

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// nativeDeskreadStub serves all three kinds the --scan-issues path reads:
//
//   - issues:   one open issue (#610) authored by an UNTRUSTED account, plus #611 (the issue
//     behind the blocked placeholder, authored by the trusted "ada" so it asks no bless read);
//   - trust:    #610's thread carries a comment by the blessing authority (ada, id 100001) and
//     nothing after it — a current blessing;
//   - comments: #611's thread carries an answer by the blessing authority after blockedAt.
//
// $1 is the kind; $3 is the --repo / --issue value.
const nativeDeskreadStub = `#!/bin/sh
kind="$1"
arg="$3"
case "$kind" in
issues)
  cat <<EOF
{"schema":1,"kind":"issues","repos":[{"repo":"$arg","issues":[
 {"number":610,"title":"external report","state":"open","authorLogin":"outside-contributor","labels":["bug"],"url":"https://example.test/610"},
 {"number":611,"title":"blocked one","state":"open","authorLogin":"ada","labels":["bug"],"url":"https://example.test/611"}
]}],"partial":[]}
EOF
  ;;
trust)
  repo="${arg%#*}"; num="${arg##*#}"
  cat <<EOF
{"schema":1,"kind":"trust","items":[{"repo":"$repo","number":$num,"trust":{"complete":true,"events":[
 {"authorLogin":"ada","authorId":100001,"createdAt":"2026-09-10T00:00:00Z"}
]}}],"partial":[]}
EOF
  ;;
comments)
  repo="${arg%#*}"; num="${arg##*#}"
  cat <<EOF
{"schema":1,"kind":"comments","items":[{"repo":"$repo","number":$num,"comments":[
 {"authorLogin":"ada","authorId":100001,"createdAt":"2026-07-10T13:00:00Z","body":"answered — go ahead"}
]}],"partial":[]}
EOF
  ;;
*) echo "unexpected kind $kind" >&2; exit 5 ;;
esac
`

// scanOnlyHomeRepo narrows the configured scan set to the home repo for one test, so the stub
// envelope (which answers every repo with the same two issues) creates exactly one placeholder.
func scanOnlyHomeRepo(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := fixtureRoster + darFixtureExtra() + "ASSAY_SCAN_REPOS=example-org/tracker\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	scanReloadConfig()
	t.Cleanup(scanReloadConfig)
}

func TestScanIssuesTrustAndUnblockReadsUseNativeForgeNotGH(t *testing.T) {
	scanOnlyHomeRepo(t)
	if got := scanRepos(); len(got) != 1 || got[0] != scanHomeRepo() {
		t.Fatalf("scan set = %v, want only the home repo %s", got, scanHomeRepo())
	}

	bin := t.TempDir()
	writeStubBin(t, bin, "gh", "#!/bin/sh\necho 'gh: HTTP 401: Requires authentication (stub)' >&2\nexit 1\n")
	writeStubBin(t, bin, "deskread", nativeDeskreadStub)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := scanHomeRepo()

	// Guard: the gh stub is effective — the pre-#1255 readers error. Without this a default
	// that silently fell back to gh could pass on a broken stub.
	if _, err := ghIssueBlessChecker(repo, 610); err == nil {
		t.Fatal("ghIssueBlessChecker succeeded with `gh` stubbed to fail — the stub is not effective")
	}
	if _, err := issueCommentLister(repo, 611); err == nil {
		t.Fatal("issueCommentLister succeeded with `gh` stubbed to fail — the stub is not effective")
	}

	// Unit level: each production default reads with no working gh.
	blessed, err := defaultScanBlessChecker(repo, 610)
	if err != nil {
		t.Fatalf("defaultScanBlessChecker errored with a working deskread and no working gh: %v — "+
			"the trust-gate read still depends on the gh shell-out", err)
	}
	if !blessed {
		t.Error("defaultScanBlessChecker = false for a thread whose only comment is the blessing authority's")
	}
	cs, err := defaultScanCommentLister(repo, 611)
	if err != nil {
		t.Fatalf("defaultScanCommentLister errored with a working deskread and no working gh: %v — "+
			"the un-block read still depends on the gh shell-out", err)
	}
	if len(cs) != 1 || cs[0].User.Login != "ada" || cs[0].User.ID != 100001 {
		t.Fatalf("defaultScanCommentLister = %+v, want ada/100001's one answer", cs)
	}

	// End to end: the exact read set main wires, through runScanIssues.
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams", scanStreamName)
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)
	blockedPath := filepath.Join(issueLoopDir, placeholderFileName(repo, 611))
	blocked := placeholderFile(scanStreamName, 611, "bug",
		"blocked: awaiting-issue-response",
		"blockedAt: 2026-07-10T12:00:00Z",
	)
	writeTemp(t, issueLoopDir, filepath.Base(blockedPath), blocked)

	var code int
	out := captureStdout(t, func() {
		code = runScanIssues(root, false, defaultScanIssueLister, defaultScanCommentLister, defaultScanBlessChecker)
	})
	if code != 0 {
		t.Fatalf("runScanIssues exited %d with no working gh, want 0; stdout:\n%s", code, out)
	}
	created := filepath.Join(issueLoopDir, placeholderFileName(repo, 610))
	if _, err := os.Stat(created); err != nil {
		t.Errorf("no placeholder for the blessed untrusted-author issue #610 (%v) — the trust-gate read did not "+
			"reach the forge; stdout:\n%s", err, out)
	}
	ph, ok, err := parsePlaceholderFile(blockedPath)
	if err != nil || !ok {
		t.Fatalf("blocked placeholder no longer parses: ok=%v err=%v", ok, err)
	}
	if ph.Blocked != "" || ph.BlockedAt != "" {
		t.Errorf("#611 is still blocked (blocked=%q) — the un-block comment read did not reach the forge; stdout:\n%s",
			ph.Blocked, out)
	}
	if !strings.Contains(out, "read 1 of 1 configured repos (0 skipped)") {
		t.Errorf("want a full, clean read of the one configured repo; stdout:\n%s", out)
	}
}

// TestScanIssuesNativeTrustReadCouldNotCheck pins the contract the migrated reads keep: an issue
// deskread could not read is an ERROR from both readers — never "not blessed" and never "nobody
// answered", which would respectively quarantine silently and leave a block standing silently.
func TestScanIssuesNativeTrustReadCouldNotCheck(t *testing.T) {
	bin := t.TempDir()
	writeStubBin(t, bin, "deskread", `#!/bin/sh
arg="$3"; repo="${arg%#*}"; num="${arg##*#}"
cat <<EOF
{"schema":1,"kind":"$1","items":[],"partial":[{"repo":"$repo","number":$num,"reason":"the App installation cannot read it"}]}
EOF
exit 6
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := deskreadIssueBlessChecker("example-org/beta", 5)
	if err == nil || !strings.Contains(err.Error(), "cannot read it") {
		t.Errorf("bless read of an unreadable issue: err=%v, want a could-not-check error carrying the reason", err)
	}
	if _, err := deskreadCommentLister("example-org/beta", 5); err == nil || !strings.Contains(err.Error(), "cannot read it") {
		t.Errorf("comment read of an unreadable issue: err=%v, want a could-not-check error carrying the reason", err)
	}
}

// TestScanIssuesNativeTrustOverflowQuarantines: a thread that overflowed the single bounded page
// (complete=false) is not blessed, with no error — the same fail-closed quarantine the gh reader
// gives an overflowed payload, even when the page it did read carries a blessing.
func TestScanIssuesNativeTrustOverflowQuarantines(t *testing.T) {
	bin := t.TempDir()
	writeStubBin(t, bin, "deskread", `#!/bin/sh
arg="$3"; repo="${arg%#*}"; num="${arg##*#}"
cat <<EOF
{"schema":1,"kind":"trust","items":[{"repo":"$repo","number":$num,"trust":{"complete":false,"events":[
 {"authorLogin":"ada","authorId":100001,"createdAt":"2026-09-10T00:00:00Z"}]}}],"partial":[]}
EOF
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	blessed, err := deskreadIssueBlessChecker("example-org/beta", 6)
	if err != nil || blessed {
		t.Errorf("overflowed thread: blessed=%v err=%v, want false/nil (fail closed to quarantine)", blessed, err)
	}
}

// TestEvalBlessingEventsMatchesGraphQLReader binds the two readers: the same thread, fed once as
// the raw GraphQL payload (evalIssueBlessing) and once as already-read events (the shape the
// deskread envelope yields), must draw the same verdict in every case the rule distinguishes.
func TestEvalBlessingEventsMatchesGraphQLReader(t *testing.T) {
	type c struct {
		login, typ string
		id         int64
		created    string
		edited     string
	}
	mk := func(bodyEdited string, cs ...c) ([]byte, []blessEvent) {
		le := "null"
		if bodyEdited != "" {
			le = `"` + bodyEdited + `"`
		}
		var nodes []string
		var evs []blessEvent
		for _, x := range cs {
			ed := "null"
			if x.edited != "" {
				ed = `"` + x.edited + `"`
			}
			nodes = append(nodes, `{"createdAt":"`+x.created+`","lastEditedAt":`+ed+
				`,"author":{"login":"`+x.login+`","__typename":"`+x.typ+`","databaseId":`+itoa64(x.id)+`}}`)
			login := x.login
			if x.typ == "Bot" {
				login += "[bot]"
			}
			e := blessEvent{login: login, id: x.id, created: mustRFC3339(t, x.created)}
			if x.edited != "" {
				e.edited = mustRFC3339(t, x.edited)
			}
			evs = append(evs, e)
		}
		raw := []byte(`{"data":{"repository":{"issue":{"lastEditedAt":` + le +
			`,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` + strings.Join(nodes, ",") + `]}}}}}`)
		return raw, evs
	}
	ada := c{"ada", "User", 100001, "2026-07-21T10:00:00Z", ""}
	for _, tc := range []struct {
		name       string
		bodyEdited string
		cs         []c
	}{
		{"blessed", "", []c{ada}},
		{"wrong id", "", []c{{"ada", "User", 31337, "2026-07-21T10:00:00Z", ""}}},
		{"body edited after", "2026-07-22T10:00:00Z", []c{ada}},
		{"body edited same instant", "2026-07-21T10:00:00Z", []c{ada}},
		{"untrusted after", "", []c{ada, {"someone", "User", 7, "2026-07-22T10:00:00Z", ""}}},
		{"untrusted edited after", "", []c{{"someone", "User", 7, "2026-07-20T10:00:00Z", "2026-07-22T10:00:00Z"}, ada}},
		{"trusted bot after", "", []c{ada, {"assay-desk-app", "Bot", 300000001, "2026-07-22T10:00:00Z", ""}}},
		{"no comments", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, evs := mk(tc.bodyEdited, tc.cs...)
			bodyEdited := mustRFC3339OrZero(t, tc.bodyEdited)
			want, werr := evalIssueBlessing(raw)
			got, gerr := evalBlessingEvents(bodyEdited, evs, true)
			if werr != nil || gerr != nil {
				t.Fatalf("errors: graphql=%v events=%v", werr, gerr)
			}
			if got != want {
				t.Errorf("events reader = %v, graphql reader = %v — the two readers disagree on one thread", got, want)
			}
		})
	}
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func mustRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad fixture time %q: %v", s, err)
	}
	return v
}

func mustRFC3339OrZero(t *testing.T, s string) time.Time {
	t.Helper()
	if s == "" {
		return time.Time{}
	}
	return mustRFC3339(t, s)
}

// TestScanIssuesMainWiresNativeReads pins main's --scan-issues call to the three default* vars
// the end-to-end test above drives — so the tested read set IS the shipped one. Passing a gh
// reader at the call site (the pre-#1255 wiring) reddens it.
func TestScanIssuesMainWiresNativeReads(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`runScanIssues\(\*root, \*scanDryRun, ([^)]*)\)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("main.go: the --scan-issues runScanIssues call was not found")
	}
	if got, want := string(m[1]), "defaultScanIssueLister, defaultScanCommentLister, defaultScanBlessChecker"; got != want {
		t.Errorf("main.go wires --scan-issues with (%s), want (%s) — every read on the scanloop path goes "+
			"through the native forge", got, want)
	}
}
