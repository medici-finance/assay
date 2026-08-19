package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// capturedRun captures stdout, stderr and the process exit code of a run.
type capturedRun struct {
	log  string // stdout
	err  string // stderr
	code int
}

// captureRun runs fn with stdout+stderr redirected to pipes, returning both plus
// the exit code fn produced. Pipes are drained concurrently so a large write
// cannot deadlock on the pipe buffer.
func captureRun(t *testing.T, fn func() int) capturedRun {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() { b, _ := io.ReadAll(rOut); outCh <- string(b) }()
	go func() { b, _ := io.ReadAll(rErr); errCh <- string(b) }()

	code := fn()
	wOut.Close()
	wErr.Close()
	return capturedRun{log: <-outCh, err: <-errCh, code: code}
}

// --- fixtures -----------------------------------------------------------------

// fixtureAuthorResolver returns a canned authorResolver keyed by "repo#issue".
// A missing key returns errFixture (an unreadable author — fail closed).
func fixtureAuthorResolver(data map[string]authorIdentity) authorResolver {
	return func(repo string, issue int) (authorIdentity, error) {
		if a, ok := data[repo+"#"+strconv.Itoa(issue)]; ok {
			return a, nil
		}
		return authorIdentity{}, errFixture
	}
}

// blessAuthorityResolver arms the enactment gate: it resolves any URL to the
// configured bless authority (ada:100001, User) with a body naming R-7.
func blessAuthorityResolver(url string) (authorIdentity, string, error) {
	return authorIdentity{Login: "ada", ID: 100001, Type: "User"}, "the blessing authority accepts R-7 as drafted", nil
}

// transcribeRoster is a roster that arms the same-repo lane for tests: bless
// authority ada, home repo example-org/tracker, an authorized author (dana) that
// is NOT the bless authority, and the worker App in the bot set.
func transcribeRoster() map[string]string {
	return map[string]string{
		scanEnvBlessLogin:        "ada:100001",
		scanEnvTrustedLogins:     "ada:100001,shared-agent:100002",
		scanEnvTrustedBotSlugs:   "worker=assay-worker-app:300000006",
		scanEnvHomeRepo:          "example-org/tracker",
		scanEnvScanRepos:         "example-org/tracker",
		scanEnvAuthorizedAuthors: "dana:200002",
	}
}

// writeRulings drops a rulings.md into root carrying the given R-7 sign-off line.
func writeRulings(t *testing.T, root, signoffLine string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", "issue-flow")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// issue-flow is a real stream in-tree, so loadStreams requires a README here.
	readme := "---\nstream: issue-flow\nstatus: active\npriority: P0\ntrack: platform\n---\n\n" +
		"# Issue Flow\n\n| # | Brief | Wave | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|----------|----------|\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "# Rulings\n\n## R-6 Verify lane\n\n**Sign-off:** _(empty)_\n\n" +
		"## R-7 Scan-transcription lane\n\nStuff.\n\n" + signoffLine + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rulings.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const r7Empty = "**Sign-off:** _(empty — the blessing authority fills with an acceptance URL)_"
const r7Armed = "**Sign-off:** https://github.com/example-org/tracker/pull/999#issuecomment-5319580160 — the blessing authority"

// --- enactment gate -----------------------------------------------------------

func TestTranscribeEnactmentGateEmptyIsInert(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	root := t.TempDir()
	writeRulings(t, root, r7Empty)
	armed, reason := transcribeEnactmentGate(root, blessAuthorityResolver)
	if armed {
		t.Fatalf("empty R-7 sign-off must be INERT, got armed (%s)", reason)
	}
	if !strings.Contains(reason, "empty") {
		t.Errorf("reason should name the empty sign-off, got %q", reason)
	}
}

func TestTranscribeEnactmentGateArmedByBlessAuthority(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	root := t.TempDir()
	writeRulings(t, root, r7Armed)
	armed, reason := transcribeEnactmentGate(root, blessAuthorityResolver)
	if !armed {
		t.Fatalf("a bless-authority sign-off naming R-7 must arm the lane, got INERT (%s)", reason)
	}
}

func TestTranscribeEnactmentGateNonBlessRefused(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	root := t.TempDir()
	writeRulings(t, root, r7Armed)

	// wrong id (recycled login), a Bot type, a wrong login, and a body not naming
	// R-7 — each must leave the lane INERT.
	cases := []struct {
		name string
		res  commentResolver
	}{
		{"recycled-id", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "ada", ID: 999, Type: "User"}, "R-7 ok", nil
		}},
		{"non-user", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "ada", ID: 100001, Type: "Bot"}, "R-7 ok", nil
		}},
		{"wrong-login", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "mallory", ID: 100001, Type: "User"}, "R-7 ok", nil
		}},
		{"body-omits-r7", func(string) (authorIdentity, string, error) {
			return authorIdentity{Login: "ada", ID: 100001, Type: "User"}, "looks good to me", nil
		}},
	}
	for _, tc := range cases {
		if armed, reason := transcribeEnactmentGate(root, tc.res); armed {
			t.Errorf("%s: must be INERT, got armed (%s)", tc.name, reason)
		}
	}
}

func TestFindR7SignoffLineScopedToR7(t *testing.T) {
	md := "## R-6 x\n**Sign-off:** https://github.com/o/r/pull/1#issuecomment-1 — the blessing authority\n" +
		"## R-7 y\n**Sign-off:** _(empty)_\n"
	line, ok := findR7SignoffLine(md)
	if !ok {
		t.Fatal("expected to find the R-7 sign-off line")
	}
	if firstHTTPSURL(line) != "" {
		t.Errorf("R-7 line is empty; a filled R-6 line must not leak in: %q", line)
	}
}

func TestParseCommentURL(t *testing.T) {
	repo, id, ok := parseCommentURL("https://github.com/example-org/tracker/pull/999#issuecomment-5319580160")
	if !ok || repo != "example-org/tracker" || id != "5319580160" {
		t.Errorf("parse pull URL = (%q,%q,%v)", repo, id, ok)
	}
	repo, id, ok = parseCommentURL("https://github.com/o/r/issues/297#issuecomment-42")
	if !ok || repo != "o/r" || id != "42" {
		t.Errorf("parse issues URL = (%q,%q,%v)", repo, id, ok)
	}
	if _, _, ok := parseCommentURL("https://github.com/o/r/pull/1"); ok {
		t.Error("a URL with no #issuecomment- must not parse")
	}
}

// --- run: INERT (row 3) -------------------------------------------------------

func TestTranscribeScanInertEvaluatesNoClause(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Empty)

	// A trusted author with an unhandled issue — which the lane would normally
	// CREATE. Because the lane is INERT it must create nothing and report INERT.
	list := fixtureLister(map[string][]ghIssue{
		"example-org/tracker": {{Number: 501, Title: "a bug", Labels: lbl("bug")}},
	}, "")
	authors := fixtureAuthorResolver(map[string]authorIdentity{
		"example-org/tracker#501": {Login: "ada", ID: 100001, Type: "User"},
	})

	out := captureRun(t, func() int {
		return runTranscribeScan(root, false, list, nilCommentLister, authors, blessAll, func(string) (authorIdentity, string, error) {
			return authorIdentity{}, "", errFixture
		})
	})
	if !strings.Contains(out.log, "INERT") {
		t.Errorf("expected INERT report, got:\n%s", out.log)
	}
	if out.code != 0 {
		t.Errorf("INERT must exit 0, got %d", out.code)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/streams/issue-loop/issue-501.md")); err == nil {
		t.Error("INERT lane must not create any placeholder")
	}
}

// --- run: trusted-author CREATE (row 1) --------------------------------------

func TestTranscribeScanTrustedAuthorCreateByteIdentical(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)

	list := fixtureLister(map[string][]ghIssue{
		"example-org/tracker": {
			{Number: 601, Title: "keydown bubbles", Labels: lbl("bug")},          // dana (authorized human)
			{Number: 602, Title: "worker retry loop", Labels: lbl("bug")},        // worker App (authorized bot)
			{Number: 603, Title: "seed via bless authority", Labels: lbl("bug")}, // ada (seeded authorized author)
		},
	}, "")
	authors := fixtureAuthorResolver(map[string]authorIdentity{
		"example-org/tracker#601": {Login: "dana", ID: 200002, Type: "User"},
		"example-org/tracker#602": {Login: "assay-worker-app[bot]", ID: 300000006, Type: "Bot"},
		"example-org/tracker#603": {Login: "ada", ID: 100001, Type: "User"},
	})

	// dry-run (--check): report only, no writes.
	out := captureRun(t, func() int {
		return runTranscribeScan(root, true, list, nilCommentLister, authors, blessNone, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("armed --check must exit 0, got %d:\n%s", out.code, out.log)
	}
	for _, n := range []int{601, 602, 603} {
		if !strings.Contains(out.log, "CREATE") || !strings.Contains(out.log, "#"+strconv.Itoa(n)) {
			t.Errorf("expected a CREATE for issue %d in:\n%s", n, out.log)
		}
		// --check writes nothing.
		if _, err := os.Stat(filepath.Join(root, "docs/streams/issue-loop/issue-"+strconv.Itoa(n)+".md")); err == nil {
			t.Errorf("--check must not write issue-%d.md", n)
		}
	}

	// Re-derivation IS the trust: the planned CREATE Content is byte-identical to
	// the scanner's own render (R-7 cl.2a).
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	attachPlaceholders(streams)
	delta, err := planTranscribeScan(root, streams, "example-org/tracker", list, nilCommentLister, authors, blessNone)
	if err != nil {
		t.Fatal(err)
	}
	if len(delta.Creates) != 3 {
		t.Fatalf("expected 3 CREATEs, got %d", len(delta.Creates))
	}
	for _, p := range delta.Creates {
		want := renderPlaceholder("example-org/tracker", p.Issue, p.Gate, p.Labels)
		if p.Content != want {
			t.Errorf("issue %d CREATE not byte-identical to renderPlaceholder:\n--- got ---\n%s\n--- want ---\n%s", p.Issue, p.Content, want)
		}
	}
}

// --- run: negative-path battery (row 2) --------------------------------------

func TestTranscribeScanUntrustedAuthorRefused(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)

	list := fixtureLister(map[string][]ghIssue{
		"example-org/tracker": {
			{Number: 701, Title: "external ask", Labels: lbl("bug")},      // untrusted human
			{Number: 702, Title: "recycled id", Labels: lbl("bug")},       // authorized login, WRONG id
			{Number: 703, Title: "author unreadable", Labels: lbl("bug")}, // resolver has no entry
		},
	}, "")
	authors := fixtureAuthorResolver(map[string]authorIdentity{
		"example-org/tracker#701": {Login: "external-user", ID: 5005, Type: "User"},
		"example-org/tracker#702": {Login: "dana", ID: 999999, Type: "User"}, // recycled
		// 703 intentionally absent → errFixture (unreadable author)
	})

	out := captureRun(t, func() int {
		// blessNone: no blessing rescues the untrusted authors.
		return runTranscribeScan(root, true, list, nilCommentLister, authors, blessNone, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("run should complete (exit 0) with per-issue skips, got %d", out.code)
	}
	for _, n := range []int{701, 702, 703} {
		marker := "SKIP example-org/tracker#" + strconv.Itoa(n) + " — clause-1 (trust)"
		if !strings.Contains(out.log, marker) {
			t.Errorf("issue %d must be REFUSED naming clause-1; log:\n%s", n, out.log)
		}
		if strings.Contains(out.log, "CREATE") && strings.Contains(out.log, "#"+strconv.Itoa(n)+",") {
			t.Errorf("issue %d must not be boarded", n)
		}
	}
}

func TestTranscribeScanRosterUnreadableRefused(t *testing.T) {
	scanWithNoRoster(t) // config home with no roster.env → unconfigured
	root := t.TempDir()
	writeRulings(t, root, r7Armed)
	list := fixtureLister(map[string][]ghIssue{"example-org/tracker": nil}, "")
	authors := fixtureAuthorResolver(nil)

	out := captureRun(t, func() int {
		return runTranscribeScan(root, true, list, nilCommentLister, authors, blessAll, blessAuthorityResolver)
	})
	if out.code != 2 {
		t.Fatalf("an unconfigured roster must REFUSE the run (exit 2), got %d:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.err, "REFUSED") || !strings.Contains(out.err, "roster") {
		t.Errorf("refusal must name the roster; stderr:\n%s", out.err)
	}
}

func TestTranscribeScanFloodTripwireRefused(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	_, root := scanFixtureRepo(t, nil)
	writeRulings(t, root, r7Armed)

	// 26 trusted-author issues → 26 CREATEs, one over the threshold → clause-6.
	var issues []ghIssue
	authorData := map[string]authorIdentity{}
	for n := 800; n < 826; n++ {
		issues = append(issues, ghIssue{Number: n, Title: "bug", Labels: lbl("bug")})
		authorData["example-org/tracker#"+strconv.Itoa(n)] = authorIdentity{Login: "ada", ID: 100001, Type: "User"}
	}
	list := fixtureLister(map[string][]ghIssue{"example-org/tracker": issues}, "")
	authors := fixtureAuthorResolver(authorData)

	out := captureRun(t, func() int {
		return runTranscribeScan(root, true, list, nilCommentLister, authors, blessNone, blessAuthorityResolver)
	})
	if out.code != 3 {
		t.Fatalf("26 CREATEs must trip the flood tripwire (exit 3), got %d:\n%s", out.code, out.log)
	}
	if !strings.Contains(out.log, "FLOOD") || !strings.Contains(out.log, "clause-6") {
		t.Errorf("flood refusal must name clause-6; log:\n%s", out.log)
	}
	// Exactly 25 would NOT trip.
	list25 := fixtureLister(map[string][]ghIssue{"example-org/tracker": issues[:25]}, "")
	out25 := captureRun(t, func() int {
		return runTranscribeScan(root, true, list25, nilCommentLister, authors, blessNone, blessAuthorityResolver)
	})
	if out25.code != 0 {
		t.Errorf("exactly 25 CREATEs must NOT trip the tripwire, got %d", out25.code)
	}
}

// --- authorizedByIdentity unit ------------------------------------------------

func TestAuthorizedByIdentity(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	cases := []struct {
		login string
		id    int64
		typ   string
		want  bool
		why   string
	}{
		{"ada", 100001, "User", true, "seeded bless authority is an authorized author"},
		{"dana", 200002, "User", true, "rostered authorized author, id matches"},
		{"dana", 999, "User", false, "authorized login, wrong id (recycled) → refused"},
		{"external-user", 5005, "User", false, "unrostered human refused"},
		{"assay-worker-app[bot]", 300000006, "Bot", true, "rostered desk App (bot rendering)"},
		{"assay-worker-app", 300000006, "Bot", false, "bare slug never trusted"},
		{"shared-agent", 100002, "User", false, "a general trusted login is NOT an authorized author"},
		{"", 0, "User", false, "empty login refused"},
	}
	for _, c := range cases {
		if got := authorizedByIdentity(c.login, c.id, c.typ); got != c.want {
			t.Errorf("authorizedByIdentity(%q,%d,%q)=%v, want %v — %s", c.login, c.id, c.typ, got, c.want, c.why)
		}
	}
}

func TestAuthorizedAuthorSetSeedsBlessAndDegrades(t *testing.T) {
	// No ASSAY_AUTHORIZED_AUTHORS set → the effective set degrades to {bless}.
	scanWithRoster(t, map[string]string{
		scanEnvBlessLogin:    "ada:100001",
		scanEnvTrustedLogins: "ada:100001",
	})
	set := authorizedAuthorSet()
	if set["ada"] != 100001 {
		t.Errorf("unset ASSAY_AUTHORIZED_AUTHORS must degrade to the seeded bless identity, got %v", set)
	}
	if len(set) != 1 {
		t.Errorf("seeded-default set should be {ada}, got %v", set)
	}
}
