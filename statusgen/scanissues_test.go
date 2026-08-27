package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// nilCommentLister returns no comments — safe default for tests that don't
// exercise the un-block path.
var nilCommentLister commentLister = func(repo string, issue int) ([]issueComment, error) {
	return nil, nil
}

// fixtureCommentLister returns a canned commentLister keyed by "repo#issue".
func fixtureCommentLister(data map[string][]issueComment, failKey string) commentLister {
	return func(repo string, issue int) ([]issueComment, error) {
		key := repo + "#" + strconv.Itoa(issue)
		if key == failKey {
			return nil, errFixture
		}
		return data[key], nil
	}
}

// fixtureCommentID resolves the numeric database id a login legitimately carries
// under the active test roster (bless authority + authorized authors), mirroring
// what GitHub returns on a real comment's .user.id. It lets buildComments fixtures
// pin the bless authority's id automatically so the id-aware unblock gate
// (isBlessAuthorityID) accepts them; unrostered logins resolve to 0 (as GitHub
// would still return a real id, but the unblock gate rejects them on login anyway).
func fixtureCommentID(login string) int64 {
	return authorizedAuthorSet()[strings.ToLower(login)]
}

// buildComments constructs a []issueComment fixture from (login, type, createdAt) triples.
// Use buildCommentsWithBody for tests that need comment bodies (e.g. marker detection).
func buildComments(entries ...[3]string) []issueComment {
	var out []issueComment
	for _, e := range entries {
		out = append(out, issueComment{
			User:      issueCommentUser{Login: e[0], ID: fixtureCommentID(e[0]), Type: e[1]},
			CreatedAt: e[2],
		})
	}
	return out
}

// buildCommentsWithBody constructs a []issueComment fixture from
// (login, type, createdAt, body) quads for tests that need body content,
// e.g. the desk-automation marker detection (Blocker 2).
func buildCommentsWithBody(entries ...[4]string) []issueComment {
	var out []issueComment
	for _, e := range entries {
		out = append(out, issueComment{
			User:      issueCommentUser{Login: e[0], ID: fixtureCommentID(e[0]), Type: e[1]},
			CreatedAt: e[2],
			Body:      e[3],
		})
	}
	return out
}

// fixtureLister returns a canned issueLister: data per repo, plus an optional
// repo whose lookup fails (to exercise the degrade-to-NOTICE path).
func fixtureLister(data map[string][]ghIssue, failRepo string) issueLister {
	return func(repo string) ([]ghIssue, error) {
		if repo == failRepo {
			return nil, errFixture
		}
		return data[repo], nil
	}
}

type fixtureErr string

func (e fixtureErr) Error() string { return string(e) }

const errFixture = fixtureErr("simulated gh failure")

// lbl builds a []ghLabel from names.
func lbl(names ...string) []ghLabel {
	out := make([]ghLabel, len(names))
	for i, n := range names {
		out[i] = ghLabel{Name: n}
	}
	return out
}

func findPlan(plans []scanPlan, repo string, issue int) *scanPlan {
	for i := range plans {
		if plans[i].Repo == repo && plans[i].Issue == issue {
			return &plans[i]
		}
	}
	return nil
}

func findCloseOut(closeOuts []closeOutPlan, repo string, issue int) *closeOutPlan {
	for i := range closeOuts {
		if closeOuts[i].Repo == repo && closeOuts[i].Issue == issue {
			return &closeOuts[i]
		}
	}
	return nil
}

// closedPlaceholderFile renders a placeholder file that is already retired
// (status: done, resolved: issue-close) for testing reactivation.
func closedPlaceholderFile(stream string, issue int, labels string, extra ...string) string {
	b := "---\n" +
		"schema: placeholder-v1\n" +
		"brief: " + stream + "/issue-" + strconv.Itoa(issue) + "\n" +
		"issue: " + strconv.Itoa(issue) + "\n" +
		"repo: example-org/tracker\n" +
		"wave: 0\n" +
		"status: done\n"
	if labels != "" {
		b += "labels: [" + labels + "]\n"
	}
	b += "resolved: issue-close\n"
	for _, e := range extra {
		b += e + "\n"
	}
	b += "---\nSee issue #" + strconv.Itoa(issue) + " — the issue body is the spec.\n"
	return b
}

// scanFixtureRepo copies goodrepo, drops the given placeholder files into the
// alpha stream (so attachPlaceholders picks them up for idempotency), and returns
// the loaded+attached streams and root.
func scanFixtureRepo(t *testing.T, placeholders map[string]string) ([]*Stream, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	for name, body := range placeholders {
		writeTemp(t, filepath.Join(root, "docs/streams/alpha"), name, body)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	attachPlaceholders(streams)
	return streams, root
}

// TestScanIssuesPlan covers the Task-2 cases: emits for an unhandled bug issue;
// skips one with an existing placeholder; skips a verify-gate/review-request
// issue (dispatch-ticket class); derived gate correct; a one-repo gh failure
// degrades to a NOTICE (not an abort). It also covers foreign-repo filename
// derivation.
func TestScanIssuesPlan(t *testing.T) {
	// Existing placeholder for home issue 300 → must be skipped (idempotency).
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-300.md": placeholderFile("alpha", 300, "bug"),
	})

	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 300, Title: "already handled", Labels: lbl("bug")},            // skip: existing
			{Number: 301, Title: "frontend keydown bubbles", Labels: lbl("bug")},   // emit: gate model
			{Number: 302, Title: "funds settlement drift", Labels: lbl("bug")},     // emit: gate human (title trigger)
			{Number: 303, Title: "sign off brief", Labels: lbl("verify-gate")},     // skip: excluded label
			{Number: 304, Title: "live check", Labels: lbl("live-verify")},         // skip: excluded label
			{Number: 305, Title: "leak funds path", Labels: lbl("security")},       // emit: gate human (risk label)
			{Number: 306, Title: "dispatch review", Labels: lbl("review-request")}, // skip: excluded dispatch label
		},
		"example-org/agents": {
			{Number: 7, Title: "agent retry loop", Labels: lbl("bug")}, // emit: foreign filename
		},
		// example-org/examples is the failing repo (below) — its data is never read.
	}

	plans, closeOuts, notices := planScan(root, streams, fixtureLister(data, "example-org/examples"), blessAll)

	// A one-repo gh failure is a NOTICE, and the rest of the scan still ran.
	if !hasProblem(notices, "examples", "skipped") {
		t.Errorf("expected a NOTICE for the failed examples repo, got: %v", notices)
	}

	// No close-outs: all placeholders are todo and their issues are open.
	if len(closeOuts) != 0 {
		t.Errorf("expected no close-outs, got %d: %+v", len(closeOuts), closeOuts)
	}

	// Exactly the four unhandled, non-excluded issues are planned.
	if len(plans) != 4 {
		t.Fatalf("expected 4 plans, got %d: %+v", len(plans), plans)
	}
	if findPlan(plans, scanHomeRepo(), 300) != nil {
		t.Error("issue 300 has an existing placeholder — must be skipped")
	}
	if findPlan(plans, scanHomeRepo(), 303) != nil || findPlan(plans, scanHomeRepo(), 304) != nil ||
		findPlan(plans, scanHomeRepo(), 306) != nil {
		t.Error("verify-gate / live-verify / review-request issues must be excluded")
	}

	// Derived gate: bug-only → model; a risk keyword in the title or a risk label → human.
	if p := findPlan(plans, scanHomeRepo(), 301); p == nil || p.Gate != "model" {
		t.Errorf("issue 301 (plain bug) should derive gate:model, got %+v", p)
	}
	if p := findPlan(plans, scanHomeRepo(), 302); p == nil || p.Gate != "human" {
		t.Errorf("issue 302 (risk keyword in title) should derive gate:human, got %+v", p)
	}
	if p := findPlan(plans, scanHomeRepo(), 305); p == nil || p.Gate != "human" {
		t.Errorf("issue 305 (security label) should derive gate:human, got %+v", p)
	}

	// Home placeholder uses the bare filename + brief id.
	p301 := findPlan(plans, scanHomeRepo(), 301)
	if got := filepath.Base(p301.Path); got != "issue-301.md" {
		t.Errorf("home placeholder filename = %q, want issue-301.md", got)
	}
	if !strings.Contains(p301.Content, "brief: issue-loop/issue-301") ||
		!strings.Contains(p301.Content, "gate: model") ||
		!strings.Contains(p301.Content, "repo: "+scanHomeRepo()) ||
		!strings.Contains(p301.Content, "See issue #301 — the issue body is the spec.") {
		t.Errorf("home placeholder content wrong:\n%s", p301.Content)
	}

	// Foreign placeholder is prefixed by the repo name.
	p7 := findPlan(plans, "example-org/agents", 7)
	if got := filepath.Base(p7.Path); got != "agents-issue-7.md" {
		t.Errorf("foreign placeholder filename = %q, want agents-issue-7.md", got)
	}
	if !strings.Contains(p7.Content, "brief: issue-loop/agents-issue-7") ||
		!strings.Contains(p7.Content, "repo: example-org/agents") {
		t.Errorf("foreign placeholder content wrong:\n%s", p7.Content)
	}
}

// TestScanIssuesEmittedFilesRoundTrip — files the scanner WRITES must satisfy the
// brief-01 parser and checker (valid placeholder-v1, derived gate preserved), and
// re-scanning is idempotent (a second run creates nothing).
func TestScanIssuesEmittedFilesRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	// The scanner writes into docs/streams/issue-loop/ — create that stream so the
	// emitted files are loadable and lint-checkable.
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)

	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 401, Title: "plain bug", Labels: lbl("bug")},
			{Number: 402, Title: "auth token bug", Labels: lbl("bug")}, // title → human
		},
	}
	list := fixtureLister(data, "")

	if code := runScanIssues(root, false, list, nilCommentLister, blessAll); code != 0 {
		t.Fatalf("first scan exited %d, want 0", code)
	}
	// Both files were written.
	for _, n := range []string{"issue-401.md", "issue-402.md"} {
		if _, err := os.Stat(filepath.Join(issueLoopDir, n)); err != nil {
			t.Fatalf("expected %s written: %v", n, err)
		}
	}

	// Emitted files parse as valid placeholders with the derived gate preserved.
	ph401, ok, err := parsePlaceholderFile(filepath.Join(issueLoopDir, "issue-401.md"))
	if err != nil || !ok {
		t.Fatalf("issue-401 did not parse: ok=%v err=%v", ok, err)
	}
	if ph401.Gate != "model" || ph401.Issue != 401 || ph401.Repo != scanHomeRepo() {
		t.Errorf("issue-401 parsed wrong: %+v", ph401)
	}
	ph402, ok, err := parsePlaceholderFile(filepath.Join(issueLoopDir, "issue-402.md"))
	if err != nil || !ok || ph402.Gate != "human" {
		t.Errorf("issue-402 should parse with derived gate:human, got ok=%v err=%v ph=%+v", ok, err, ph402)
	}

	// The whole --lint path stays green with the emitted placeholders present, and
	// the reduced-schema checker flags nothing.
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if problems, _ := checkPlaceholderFiles(streams); len(problems) != 0 {
		t.Fatalf("emitted placeholders tripped checkPlaceholderFiles: %v", problems)
	}
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint with emitted placeholders exited %d, want 0", code)
	}

	// Idempotency: a second scan (same issues, now with placeholders on disk)
	// creates nothing.
	streams2, _, _ := loadStreams(root)
	attachPlaceholders(streams2)
	plans2, _, _ := planScan(root, streams2, list, blessAll)
	if len(plans2) != 0 {
		t.Errorf("re-scan should be idempotent, but planned %d: %+v", len(plans2), plans2)
	}
}

// TestScanIssuesDryRunWritesNothing — --dry-run lists plans but writes no files.
func TestScanIssuesDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)

	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 501, Title: "bug", Labels: lbl("bug")}},
	}
	if code := runScanIssues(root, true, fixtureLister(data, ""), nilCommentLister, blessAll); code != 0 {
		t.Fatalf("dry-run exited %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(issueLoopDir, "issue-501.md")); !os.IsNotExist(err) {
		t.Error("dry-run must not write placeholder files")
	}
}

// TestScanIssuesDryRunUnblockNoWrites is the #852 regression guard: the un-block
// lane used to os.WriteFile (strip blocked:/blockedAt:) BEFORE the dry-run branch,
// so a flag documented "without writing" still mutated placeholder frontmatter on
// disk. Under --dry-run the scan must now perform ZERO filesystem writes on a
// cleared-block placeholder — content AND mtime unchanged — while still REPORTING
// the un-block ("would unblock …"). A non-dry-run run is the positive control: the
// same fixture is actually written (block stripped).
func TestScanIssuesDryRunUnblockNoWrites(t *testing.T) {
	// A blocked placeholder whose issue (still open) received a human answer
	// AFTER blockedAt — the classic un-block case.
	blockedBody := placeholderFile("alpha", 500, "bug",
		"blocked: awaiting-issue-response",
		"blockedAt: 2026-07-10T12:00:00Z",
	)
	// "ada" (the configured blessing authority) answered after the block.
	comments := fixtureCommentLister(map[string][]issueComment{
		scanHomeRepo() + "#500": buildComments(
			[3]string{"ada", "User", "2026-07-10T13:00:00Z"},
		),
	}, "")
	// Issue 500 is OPEN (so it is neither created — a placeholder exists — nor
	// retired), keeping the scan focused on the un-block lane.
	issues := fixtureLister(map[string][]ghIssue{
		scanHomeRepo(): {{Number: 500, Title: "bug", Labels: lbl("bug")}},
	}, "")

	newFixture := func() (root, phPath string) {
		root = t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		phPath = filepath.Join(root, "docs/streams/alpha", "issue-500.md")
		writeTemp(t, filepath.Dir(phPath), "issue-500.md", blockedBody)
		return root, phPath
	}

	// --- dry-run: no write, but the un-block is reported ---
	root, phPath := newFixture()
	before, err := os.ReadFile(phPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(phPath)
	if err != nil {
		t.Fatal(err)
	}

	var code int
	out := captureStdout(t, func() {
		code = runScanIssues(root, true, issues, comments, blessAll)
	})
	if code != 0 {
		t.Fatalf("dry-run exited %d, want 0", code)
	}

	after, err := os.ReadFile(phPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("dry-run mutated placeholder content (#852):\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	afterInfo, err := os.Stat(phPath)
	if err != nil {
		t.Fatal(err)
	}
	if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		t.Errorf("dry-run rewrote the placeholder file (mtime changed %v → %v) — must be read-only (#852)",
			beforeInfo.ModTime(), afterInfo.ModTime())
	}
	// The block state must be intact on disk after a dry-run.
	if ph, ok, _ := parsePlaceholderFile(phPath); !ok || ph.Blocked == "" {
		t.Error("dry-run must leave the placeholder blocked; blocked field was cleared")
	}
	// …yet the un-block is still reported.
	if !strings.Contains(out, "would unblock") {
		t.Errorf("dry-run must report the un-block (\"would unblock …\"); got:\n%s", out)
	}

	// --- positive control: non-dry-run DOES write (block stripped) ---
	root2, phPath2 := newFixture()
	if code := runScanIssues(root2, false, issues, comments, blessAll); code != 0 {
		t.Fatalf("non-dry-run exited %d, want 0", code)
	}
	ph2, ok, err := parsePlaceholderFile(phPath2)
	if err != nil || !ok {
		t.Fatalf("unblocked file must still parse: ok=%v err=%v", ok, err)
	}
	if ph2.Blocked != "" || ph2.BlockedAt != "" {
		t.Errorf("non-dry-run must clear the block; got blocked=%q blockedAt=%q", ph2.Blocked, ph2.BlockedAt)
	}
}

// TestScanExcludedLabels documents the exclusion constant is complete for the
// system-state labels (including the review-request dispatch label) and
// matches case-insensitively.
// TestScanReposCoversOwnedSet locks the widened repo set. The scanner's
// scope is meant to be auditable from source alone, and the intake-desk SKILL's
// boot-step-1 list names this slice as its code authority — so the set is asserted
// here rather than left to drift. It also guards the property the filename scheme
// depends on: every foreign repo's bare NAME must be unique, or two repos' issue
// #7 would both want `<name>-issue-7.md`.
func TestScanReposCoversOwnedSet(t *testing.T) {
	// Every owned repo EXCEPT the ones deliberately held out of the scan
	// set (see the scanRepos() comment on held-out repos), plus the three shared-agent repos.
	want := []string{
		"example-org/tracker",
		"example-org/agents",
		"example-org/examples",
		"example-org/console",
		"example-org/team-slides",
		"example-org/site",
		"example-org/toolkit",
		"example-org/org-slides",
		"example-org/docs-slides",
		"example-org/platform",
		"example-org/website",
		"example-org/proposals",
		"example-org/example-service",
		"example-org/example-service-slides",
	}
	have := map[string]bool{}
	for _, r := range scanRepos() {
		if have[r] {
			t.Errorf("scanRepos() lists %q twice — it would be scanned (and NOTICEd) twice", r)
		}
		have[r] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("scanRepos() is missing owned repo %q — issues filed there go unmonitored", w)
		}
	}
	if len(scanRepos()) != len(want) {
		t.Errorf("scanRepos() has %d entries, want %d — if a repo was added, add it to the intake-desk SKILL's boot-step-1 list too (one authority)", len(scanRepos()), len(want))
	}

	// Filename-collision guard: placeholderStem uses the bare repo name.
	stems := map[string]string{}
	for _, r := range scanRepos() {
		stem := placeholderStem(r, 7)
		if prev, dup := stems[stem]; dup {
			t.Errorf("repos %q and %q both derive placeholder stem %q — issue numbers would collide on disk", prev, r, stem)
		}
		stems[stem] = r
	}
}

func TestScanExcludedLabels(t *testing.T) {
	for _, l := range []string{
		"verify-gate", "VERIFY-GATE",
		"live-verify", "Live-Verify",
		"needs-decision", "Needs-Decision",
		"review-request", "Review-Request",
	} {
		if !hasExcludedLabel([]string{"bug", l}) {
			t.Errorf("%q should be excluded", l)
		}
	}
	if hasExcludedLabel([]string{"bug", "enhancement"}) {
		t.Error("ordinary labels must not be excluded")
	}
}

// scanStreamREADME is a minimal issue-loop stream README for the write tests. The
// single brief row is a plain (non-link) title with status todo so it trips
// neither linkcheck nor the done-requires-Verified/Reviewed gate.
const scanStreamREADME = `---
stream: issue-loop
status: active
priority: P1
track: platform
issues: []
---

# Issue-Loop Stream

Test fixture stream.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | Schema | 0 | M | todo | — | — |
`

// TestBlockedPlaceholderParseAndCheck — a blocked placeholder parses correctly
// with the blocked/blockedAt fields, and the checker validates the values.
func TestBlockedPlaceholderParseAndCheck(t *testing.T) {
	dir := t.TempDir()

	// Valid blocked placeholder.
	ph, ok, err := parsePlaceholderFile(writeTemp(t, dir, "issue-400.md", placeholderFile("issue-loop", 400, "bug",
		"blocked: awaiting-issue-response",
		"blockedAt: 2026-07-10T12:00:00Z",
	)))
	if err != nil || !ok {
		t.Fatalf("valid blocked placeholder: ok=%v err=%v", ok, err)
	}
	if ph.Blocked != "awaiting-issue-response" || ph.BlockedAt != "2026-07-10T12:00:00Z" {
		t.Errorf("blocked fields wrong: blocked=%q blockedAt=%q", ph.Blocked, ph.BlockedAt)
	}

	// Invalid blocked value.
	bad := "---\nschema: placeholder-v1\nbrief: issue-loop/issue-401\nissue: 401\nrepo: o/r\nblocked: bad-state\nblockedAt: 2026-07-10T12:00:00Z\n---\nbody\n"
	streams, _ := placeholderRepo(t, map[string]string{"issue-401.md": bad})
	problems, _ := checkPlaceholderFiles(streams)
	if !hasProblem(problems, "invalid blocked") {
		t.Errorf("want a problem for invalid blocked value, got: %v", problems)
	}

	// Blocked set but blockedAt missing.
	noAt := "---\nschema: placeholder-v1\nbrief: issue-loop/issue-402\nissue: 402\nrepo: o/r\nblocked: awaiting-issue-response\n---\nbody\n"
	streams2, _ := placeholderRepo(t, map[string]string{"issue-402.md": noAt})
	problems2, _ := checkPlaceholderFiles(streams2)
	if !hasProblem(problems2, "blockedAt is missing") {
		t.Errorf("want a problem for missing blockedAt, got: %v", problems2)
	}
}

// TestBlockedPlaceholderNextUpExclusion — a blocked placeholder is excluded from
// Next-up (Verify item 2).
func TestBlockedPlaceholderNextUpExclusion(t *testing.T) {
	streams, _ := placeholderRepo(t, map[string]string{
		"issue-77.md": placeholderFile("alpha", 77, "bug"),
		"issue-88.md": placeholderFile("alpha", 88, "bug",
			"blocked: awaiting-issue-response",
			"blockedAt: 2026-07-10T12:00:00Z",
		),
	})

	// Unblocked placeholder appears in Next-up.
	picks := nextUp(streams, ClaimView{}, nil).Picks
	found77 := false
	found88 := false
	for _, p := range picks {
		if p.Brief.Num == "issue-77" {
			found77 = true
		}
		if p.Brief.Num == "issue-88" {
			found88 = true
		}
	}
	if !found77 {
		t.Error("unblocked placeholder issue-77 should be in Next-up")
	}
	if found88 {
		t.Error("blocked placeholder issue-88 must be excluded from Next-up")
	}

	// The blocked placeholder's title surfaces the blocked state.
	for _, s := range streams {
		for _, b := range s.Briefs {
			if b.Num == "issue-88" {
				if !strings.Contains(b.Title, "[blocked:awaiting-issue-response]") {
					t.Errorf("blocked placeholder title must surface blocked state: %q", b.Title)
				}
			}
		}
	}
}

// TestBlockedPlaceholderUnblockDetection covers the Task-3 un-block cases:
// non-bot comment after blockAt → cleared; bot comment after blockAt → still
// blocked; no comments → still blocked.
func TestBlockedPlaceholderUnblockDetection(t *testing.T) {
	// Build a fixture with one blocked placeholder.
	blockedBody := placeholderFile("alpha", 500, "bug",
		"blocked: awaiting-issue-response",
		"blockedAt: 2026-07-10T12:00:00Z",
	)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	phPath := filepath.Join(root, "docs/streams/alpha", "issue-500.md")
	writeTemp(t, filepath.Dir(phPath), "issue-500.md", blockedBody)

	// --- subtest: non-bot comment after blockedAt → unblocked ---
	streams1, _, _ := loadStreams(root)
	attachPlaceholders(streams1)
	comments1 := fixtureCommentLister(map[string][]issueComment{
		scanHomeRepo() + "#500": buildComments(
			[3]string{"bot-user", "Bot", "2026-07-10T11:00:00Z"}, // bot, before block
			[3]string{"ada", "User", "2026-07-10T13:00:00Z"},     // human, after block → unblock
		),
	}, "")
	notices := unblockPlaceholders(root, streams1, comments1)
	if len(notices) != 0 {
		t.Logf("unblock notices: %v", notices)
	}
	// Re-read the file — blocked lines should be gone.
	ph, ok, err := parsePlaceholderFile(phPath)
	if err != nil || !ok {
		t.Fatalf("unblocked file must still parse: ok=%v err=%v", ok, err)
	}
	if ph.Blocked != "" {
		t.Errorf("blocked field should be cleared after unblock; got %q", ph.Blocked)
	}
	if ph.BlockedAt != "" {
		t.Errorf("blockedAt field should be cleared after unblock; got %q", ph.BlockedAt)
	}

	// --- subtest: bot-only comments → still blocked ---
	root2 := t.TempDir()
	if err := os.CopyFS(root2, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	phPath2 := filepath.Join(root2, "docs/streams/alpha", "issue-500.md")
	writeTemp(t, filepath.Dir(phPath2), "issue-500.md", blockedBody)
	streams2, _, _ := loadStreams(root2)
	attachPlaceholders(streams2)
	comments2 := fixtureCommentLister(map[string][]issueComment{
		scanHomeRepo() + "#500": buildComments(
			[3]string{"example-reviewer-app[bot]", "Bot", "2026-07-10T14:00:00Z"}, // bot, after block
		),
	}, "")
	_ = unblockPlaceholders(root2, streams2, comments2)
	ph2, ok, _ := parsePlaceholderFile(phPath2)
	if !ok || ph2.Blocked == "" {
		t.Error("bot-only comment must NOT unblock; blocked should still be set")
	}

	// --- subtest: no comments → still blocked ---
	root3 := t.TempDir()
	if err := os.CopyFS(root3, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	phPath3 := filepath.Join(root3, "docs/streams/alpha", "issue-500.md")
	writeTemp(t, filepath.Dir(phPath3), "issue-500.md", blockedBody)
	streams3, _, _ := loadStreams(root3)
	attachPlaceholders(streams3)
	comments3 := fixtureCommentLister(map[string][]issueComment{}, "") // no comments for any issue
	_ = unblockPlaceholders(root3, streams3, comments3)
	ph3, ok, _ := parsePlaceholderFile(phPath3)
	if !ok || ph3.Blocked == "" {
		t.Error("no comments must NOT unblock; blocked should still be set")
	}

	// --- subtest: non-bot [bot]-suffix user → treated as bot ---
	if !isBotComment(issueCommentUser{Login: "some-tool[bot]", Type: "User"}, "") {
		t.Error("[bot] suffix must be treated as a bot")
	}
	if !isBotComment(issueCommentUser{Login: "x", Type: "Bot"}, "") {
		t.Error("Bot type must be treated as a bot")
	}
	if isBotComment(issueCommentUser{Login: "alex", Type: "User"}, "") {
		t.Error("human User must not be treated as a bot")
	}
	// Body marker: desk-automation marker → treated as bot (Blocker 2)
	if !isBotComment(issueCommentUser{Login: "shared-agent", Type: "User"}, "status update <!-- desk-automation -->") {
		t.Error("comment with desk-automation marker must be treated as a bot even with type:User")
	}
	if isBotComment(issueCommentUser{Login: "shared-agent", Type: "User"}, "yes, the fix looks good") {
		t.Error("comment without desk-automation marker and type:User must NOT be treated as a bot")
	}
}

// TestBlockedPlaceholderLintGreen — --lint stays green with a blocked placeholder present.
func TestBlockedPlaceholderLintGreen(t *testing.T) {
	streams, root := placeholderRepo(t, map[string]string{
		"issue-77.md": placeholderFile("alpha", 77, "bug"),
		"issue-99.md": placeholderFile("alpha", 99, "bug",
			"blocked: awaiting-issue-response",
			"blockedAt: 2026-07-10T12:00:00Z",
		),
	})
	_ = streams
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint with blocked placeholder exited %d, want 0", code)
	}
}

// TestBlockedPlaceholderMarkerCommentDoesNotUnblock — a comment with the
// desk-automation marker (shared-agent/type:User with <!-- desk-automation --> in the
// body) after blockedAt must NOT un-block (Blocker 2: the
// loop's own automation shares the shared-agent identity with human:<name>, so login alone
// cannot distinguish the answer from the question).
func TestBlockedPlaceholderMarkerCommentDoesNotUnblock(t *testing.T) {
	blockedBody := placeholderFile("alpha", 500, "bug",
		"blocked: awaiting-issue-response",
		"blockedAt: 2026-07-10T12:00:00Z",
	)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	phPath := filepath.Join(root, "docs/streams/alpha", "issue-500.md")
	writeTemp(t, filepath.Dir(phPath), "issue-500.md", blockedBody)

	streams, _, _ := loadStreams(root)
	attachPlaceholders(streams)

	// A comment from shared-agent/type:User with the desk-automation marker
	// after blockedAt — this is the desk posting "still waiting?" and it
	// must NOT un-block.
	comments := fixtureCommentLister(map[string][]issueComment{
		scanHomeRepo() + "#500": buildCommentsWithBody(
			[4]string{"shared-agent", "User", "2026-07-10T14:00:00Z", "still waiting on this <!-- desk-automation -->"},
		),
	}, "")
	_ = unblockPlaceholders(root, streams, comments)
	ph, ok, _ := parsePlaceholderFile(phPath)
	if !ok || ph.Blocked == "" {
		t.Error("desk-automation marker comment must NOT un-block; blocked should still be set")
	}

	// Now a real ada answer (no marker) — should un-block. shared-agent comments
	// no longer un-block (T1: only ada, the numeric-id-verified human, un-blocks).
	root2 := t.TempDir()
	if err := os.CopyFS(root2, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	phPath2 := filepath.Join(root2, "docs/streams/alpha", "issue-500.md")
	writeTemp(t, filepath.Dir(phPath2), "issue-500.md", blockedBody)
	streams2, _, _ := loadStreams(root2)
	attachPlaceholders(streams2)
	comments2 := fixtureCommentLister(map[string][]issueComment{
		scanHomeRepo() + "#500": buildCommentsWithBody(
			[4]string{"ada", "User", "2026-07-10T15:00:00Z", "yes, the fix looks good — go ahead"},
		),
	}, "")
	_ = unblockPlaceholders(root2, streams2, comments2)
	ph2, ok2, _ := parsePlaceholderFile(phPath2)
	if !ok2 || ph2.Blocked != "" {
		t.Errorf("ada answer (User, no marker) should un-block; blocked=%q", ph2.Blocked)
	}

	// shared-agent comment (User, no marker) must NOT un-block (T1: only ada un-blocks).
	root3 := t.TempDir()
	if err := os.CopyFS(root3, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	phPath3 := filepath.Join(root3, "docs/streams/alpha", "issue-500.md")
	writeTemp(t, filepath.Dir(phPath3), "issue-500.md", blockedBody)
	streams3, _, _ := loadStreams(root3)
	attachPlaceholders(streams3)
	comments3 := fixtureCommentLister(map[string][]issueComment{
		scanHomeRepo() + "#500": buildCommentsWithBody(
			[4]string{"shared-agent", "User", "2026-07-10T15:00:00Z", "yes, the fix looks good — go ahead"},
		),
	}, "")
	_ = unblockPlaceholders(root3, streams3, comments3)
	ph3, ok3, _ := parsePlaceholderFile(phPath3)
	if !ok3 || ph3.Blocked == "" {
		t.Errorf("shared-agent comment must NOT un-block (T1: only ada un-blocks); blocked=%q", ph3.Blocked)
	}
}

// TestBlockedPlaceholderManyComments — newestNonBotCommentTime correctly identifies
// the newest non-bot comment in a large set (validates the logic handles >30
// comments; the production pagination fix is in issueCommentLister).
func TestBlockedPlaceholderManyComments(t *testing.T) {
	// Build 35 comments: the first 34 are bot comments (old), the last is
	// a human answer (new). The newest non-bot should be the last one.
	var entries [][4]string
	for i := 0; i < 34; i++ {
		entries = append(entries, [4]string{
			"bot-" + strconv.Itoa(i), "Bot",
			"2026-07-10T" + strconv.Itoa(10+i/60) + ":" + strconv.Itoa(i%60) + ":00Z",
			"",
		})
	}
	entries = append(entries, [4]string{"ada", "User", "2026-07-10T15:00:00Z", "lgtm"})

	comments := buildCommentsWithBody(entries...)
	newest, ok := newestNonBotCommentTime(comments)
	if !ok {
		t.Fatal("expected to find a non-bot comment")
	}
	want, _ := time.Parse(time.RFC3339, "2026-07-10T15:00:00Z")
	if !newest.Equal(want) {
		t.Errorf("newest non-bot = %v, want %v", newest, want)
	}
}

// TestBlockedPlaceholderLintRejectsBadBlockedAt — --lint rejects an unparseable
// blockedAt timestamp (non-blocking finding).
func TestBlockedPlaceholderLintRejectsBadBlockedAt(t *testing.T) {
	bad := "---\nschema: placeholder-v1\nbrief: issue-loop/issue-401\nissue: 401\nrepo: o/r\nblocked: awaiting-issue-response\nblockedAt: yesterday\n---\nbody\n"
	streams, _ := placeholderRepo(t, map[string]string{"issue-401.md": bad})
	problems, _ := checkPlaceholderFiles(streams)
	if !hasProblem(problems, "not a valid RFC 3339 timestamp") {
		t.Errorf("want a problem for unparseable blockedAt, got: %v", problems)
	}
}

// --- close-out tests ---

// TestCloseOutRetire — an existing placeholder whose issue is NOT in the open list
// (→ closed) is planned for retirement (action: retire).
func TestCloseOutRetire(t *testing.T) {
	// Existing todo placeholder for issue 600.
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-600.md": placeholderFile("alpha", 600, "bug"),
	})

	// Issue 600 is NOT in the open data (simulating it was closed).
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 700, Title: "some other open bug", Labels: lbl("bug")},
		},
	}

	plans, closeOuts, notices := planScan(root, streams, fixtureLister(data, ""), blessAll)
	_ = notices

	// Issue 700 is unhandled → created.
	if p := findPlan(plans, scanHomeRepo(), 700); p == nil {
		t.Error("issue 700 should be planned for creation")
	}

	// Issue 600 placeholder exists, issue not open → retire.
	c := findCloseOut(closeOuts, scanHomeRepo(), 600)
	if c == nil {
		t.Fatalf("issue 600 placeholder should be retired (issue closed)")
	}
	if c.Action != "retire" {
		t.Errorf("action = %q, want retire", c.Action)
	}
}

// TestCloseOutReactivate — a retired placeholder (status: done + resolved) whose
// issue reappears in the open list is planned for reactivation.
func TestCloseOutReactivate(t *testing.T) {
	// Existing retired placeholder for issue 601.
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-601.md": closedPlaceholderFile("alpha", 601, "bug"),
	})

	// Issue 601 is back in the open list (reopened).
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 601, Title: "reopened bug", Labels: lbl("bug")},
		},
	}

	plans, closeOuts, notices := planScan(root, streams, fixtureLister(data, ""), blessAll)
	_ = notices

	// Issue 601 already has a placeholder → no creation plan.
	if findPlan(plans, scanHomeRepo(), 601) != nil {
		t.Error("issue 601 already has a placeholder — must not be re-created")
	}

	// But the placeholder is done while the issue is open → reactivate.
	c := findCloseOut(closeOuts, scanHomeRepo(), 601)
	if c == nil {
		t.Fatalf("issue 601 placeholder should be reactivated (issue reopened)")
	}
	if c.Action != "reactivate" {
		t.Errorf("action = %q, want reactivate", c.Action)
	}
}

// TestCloseOutCreateAndRetireOneSweep — a single scan both creates a new
// placeholder for an open issue AND retires an existing placeholder for a now-
// closed issue ("one --scan-issues both adds new placeholders
// and retires resolved ones").
func TestCloseOutCreateAndRetireOneSweep(t *testing.T) {
	// Existing todo placeholder for issue 602 (going to be closed).
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-602.md": placeholderFile("alpha", 602, "bug"),
	})

	// Issue 602 is closed (not open). Issue 603 is a new open issue.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 603, Title: "new bug", Labels: lbl("bug")},
		},
	}

	plans, closeOuts, _ := planScan(root, streams, fixtureLister(data, ""), blessAll)

	// Creation: issue 603 → new placeholder.
	if findPlan(plans, scanHomeRepo(), 603) == nil {
		t.Error("issue 603 should be planned for creation")
	}

	// Retirement: issue 602 → retire.
	c := findCloseOut(closeOuts, scanHomeRepo(), 602)
	if c == nil || c.Action != "retire" {
		t.Fatalf("issue 602 placeholder should be retired in the same sweep, got %+v", c)
	}

	if len(plans) != 1 {
		t.Errorf("expected 1 creation plan, got %d", len(plans))
	}
	if len(closeOuts) != 1 {
		t.Errorf("expected 1 close-out plan, got %d", len(closeOuts))
	}
}

// TestCloseOutAlreadyDoneSweepsToArchive — a placeholder already at status: done
// but still sitting at the stream ROOT is not re-retired (idempotent on the
// frontmatter) but IS swept into done/ (D3: the ghost drain).
func TestCloseOutAlreadyDoneSweepsToArchive(t *testing.T) {
	// Existing done placeholder (already retired) for issue 604, at the root.
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-604.md": closedPlaceholderFile("alpha", 604, "bug"),
	})

	// Issue 604 is NOT open.
	data := map[string][]ghIssue{
		scanHomeRepo(): {},
	}

	_, closeOuts, _ := planScan(root, streams, fixtureLister(data, ""), blessAll)

	// Already done + at the root → sweep it into done/ (no frontmatter change).
	c := findCloseOut(closeOuts, scanHomeRepo(), 604)
	if c == nil || c.Action != "sweep" {
		t.Fatalf("already-done root placeholder should be swept into done/, got %+v", c)
	}
	if filepath.Base(filepath.Dir(c.Dest)) != archiveDirName {
		t.Errorf("sweep dest = %q, want a %s/ path", c.Dest, archiveDirName)
	}
}

// TestCloseOutAlreadyTodoSkipsReactivate — a placeholder already at status: todo
// with an open issue is not re-activated (idempotent).
func TestCloseOutAlreadyTodoSkipsReactivate(t *testing.T) {
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-605.md": placeholderFile("alpha", 605, "bug"),
	})

	// Issue 605 is open.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 605, Title: "still open bug", Labels: lbl("bug")},
		},
	}

	_, closeOuts, _ := planScan(root, streams, fixtureLister(data, ""), blessAll)

	// Already todo with open issue → no close-out action.
	if findCloseOut(closeOuts, scanHomeRepo(), 605) != nil {
		t.Error("already-todo placeholder with open issue must not be re-activated")
	}
}

// TestCloseOutDryRun — --dry-run reports retire and reactivate actions without
// modifying files.
func TestCloseOutDryRun(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)

	// Write a todo placeholder whose issue will be "closed".
	writeTemp(t, issueLoopDir, "issue-610.md", placeholderFile("issue-loop", 610, "bug"))

	// Only issue 611 is open.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 611, Title: "new bug", Labels: lbl("bug")},
		},
	}

	code := runScanIssues(root, true, fixtureLister(data, ""), nilCommentLister, blessAll)
	if code != 0 {
		t.Fatalf("dry-run exited %d, want 0", code)
	}

	// Dry-run must not modify existing placeholder files.
	ph, ok, err := parsePlaceholderFile(filepath.Join(issueLoopDir, "issue-610.md"))
	if err != nil || !ok {
		t.Fatalf("issue-610 must still be parseable after dry-run: ok=%v err=%v", ok, err)
	}
	if ph.Status != "todo" {
		t.Errorf("dry-run must not modify files; issue-610 status = %q, want todo", ph.Status)
	}
	// Dry-run must not create new files.
	if _, err := os.Stat(filepath.Join(issueLoopDir, "issue-611.md")); !os.IsNotExist(err) {
		t.Error("dry-run must not create placeholder files")
	}
}

// TestCloseOutFileModificationRetire — applyCloseOut actually rewrites the
// placeholder file: status → done, adds resolved: issue-close.
func TestCloseOutFileModificationRetire(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)
	rootPath := filepath.Join(issueLoopDir, "issue-620.md")
	writeTemp(t, issueLoopDir, "issue-620.md", placeholderFile("issue-loop", 620, "bug"))

	// Run scan with issue 620 NOT open → retire.
	data := map[string][]ghIssue{
		scanHomeRepo(): {},
	}
	code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll)
	if code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}

	// (D3): retire MOVES the placeholder into done/; the root copy
	// is gone.
	if _, err := os.Stat(rootPath); !os.IsNotExist(err) {
		t.Errorf("retired placeholder must be moved out of the stream root (stat err=%v)", err)
	}
	phPath := filepath.Join(issueLoopDir, archiveDirName, "issue-620.md")

	// File must now be status: done + resolved: issue-close, at its done/ path.
	ph, ok, err := parsePlaceholderFile(phPath)
	if err != nil || !ok {
		t.Fatalf("retired file must parse: ok=%v err=%v", ok, err)
	}
	if ph.Status != "done" {
		t.Errorf("retired placeholder status = %q, want done", ph.Status)
	}

	// Re-read raw file to confirm resolved: issue-close is present.
	raw, err := os.ReadFile(phPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "resolved: issue-close") {
		t.Error("retired placeholder file must contain 'resolved: issue-close'")
	}
	if !strings.Contains(string(raw), "status: done") {
		t.Error("retired placeholder file must contain 'status: done'")
	}

	// Lint stays green.
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after retire exited %d, want 0", code)
	}
}

// TestCloseOutFileModificationReactivate — applyCloseOut rewrites a retired
// placeholder back to todo and removes resolved.
func TestCloseOutFileModificationReactivate(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)
	phPath := filepath.Join(issueLoopDir, "issue-621.md")
	writeTemp(t, issueLoopDir, "issue-621.md", closedPlaceholderFile("issue-loop", 621, "bug"))

	// Run scan with issue 621 open → reactivate.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 621, Title: "reopened bug", Labels: lbl("bug")},
		},
	}
	code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll)
	if code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}

	// File must now be status: todo and resolved removed.
	ph, ok, err := parsePlaceholderFile(phPath)
	if err != nil || !ok {
		t.Fatalf("reactivated file must parse: ok=%v err=%v", ok, err)
	}
	if ph.Status != "todo" {
		t.Errorf("reactivated placeholder status = %q, want todo", ph.Status)
	}

	raw, err := os.ReadFile(phPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "resolved:") {
		t.Error("reactivated placeholder file must NOT contain 'resolved:'")
	}
	if !strings.Contains(string(raw), "status: todo") {
		t.Error("reactivated placeholder file must contain 'status: todo'")
	}

	// Lint stays green.
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after reactivate exited %d, want 0", code)
	}
}

// TestCloseOutSweepRootGhostToArchive — a status: done placeholder still at the
// stream ROOT (a ghost) is swept into done/ by a scan, unchanged in content, with
// the root copy removed and lint green (D3 — the backlog drain).
func TestCloseOutSweepRootGhostToArchive(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)
	rootPath := filepath.Join(issueLoopDir, "issue-650.md")
	writeTemp(t, issueLoopDir, "issue-650.md", closedPlaceholderFile("issue-loop", 650, "bug"))

	// Issue 650 is NOT open (already closed and already retired at the root).
	data := map[string][]ghIssue{scanHomeRepo(): {}}
	code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll)
	if code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}

	if _, err := os.Stat(rootPath); !os.IsNotExist(err) {
		t.Errorf("swept ghost must be moved out of the stream root (stat err=%v)", err)
	}
	archived := filepath.Join(issueLoopDir, archiveDirName, "issue-650.md")
	raw, err := os.ReadFile(archived)
	if err != nil {
		t.Fatalf("swept ghost must live in done/: %v", err)
	}
	// Sweep does not touch frontmatter — still done + resolved: issue-close.
	if !strings.Contains(string(raw), "status: done") || !strings.Contains(string(raw), "resolved: issue-close") {
		t.Errorf("swept ghost frontmatter changed: %s", raw)
	}
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after sweep exited %d, want 0", code)
	}
}

// TestCloseOutReactivateFromArchive — a placeholder sitting in done/ whose issue
// REOPENS is reactivated (status: todo) AND moved back to the stream root
// (D3 — reactivation checks done/ and moves the file back).
func TestCloseOutReactivateFromArchive(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	doneDir := filepath.Join(issueLoopDir, archiveDirName)
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)
	// The retired placeholder already lives in done/.
	archivedPath := filepath.Join(doneDir, "issue-651.md")
	writeTemp(t, doneDir, "issue-651.md", closedPlaceholderFile("issue-loop", 651, "bug"))

	// Issue 651 is back open (reopened).
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 651, Title: "reopened bug", Labels: lbl("bug")},
		},
	}
	code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll)
	if code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}

	// Moved back to the root, and the done/ copy is gone.
	if _, err := os.Stat(archivedPath); !os.IsNotExist(err) {
		t.Errorf("reactivated placeholder must be moved out of done/ (stat err=%v)", err)
	}
	rootPath := filepath.Join(issueLoopDir, "issue-651.md")
	ph, ok, err := parsePlaceholderFile(rootPath)
	if err != nil || !ok {
		t.Fatalf("reactivated file must parse at the root: ok=%v err=%v", ok, err)
	}
	if ph.Status != "todo" {
		t.Errorf("reactivated placeholder status = %q, want todo", ph.Status)
	}
	// A reopened issue must NOT also get a fresh placeholder created.
	raw, _ := os.ReadFile(rootPath)
	if strings.Contains(string(raw), "resolved:") {
		t.Error("reactivated placeholder must NOT retain a resolved: marker")
	}
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after reactivate-from-archive exited %d, want 0", code)
	}
}

// TestCloseOutBlockedPlaceholderRetired — a blocked placeholder whose issue is
// closed is still retired (the block is moot when the issue is closed).
func TestCloseOutBlockedPlaceholderRetired(t *testing.T) {
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-630.md": placeholderFile("alpha", 630, "bug",
			"blocked: awaiting-issue-response",
			"blockedAt: 2026-07-10T12:00:00Z",
		),
	})

	// Issue 630 is NOT open → closed.
	data := map[string][]ghIssue{
		scanHomeRepo(): {},
	}

	_, closeOuts, _ := planScan(root, streams, fixtureLister(data, ""), blessAll)

	c := findCloseOut(closeOuts, scanHomeRepo(), 630)
	if c == nil || c.Action != "retire" {
		t.Fatalf("blocked placeholder with closed issue must be retired, got %+v", c)
	}
}

// labelExcludedPlaceholderFile renders a placeholder file already retired via the
// label-exclusion path (status: done, resolved: label-excluded) — the
// counterpart to closedPlaceholderFile's issue-close retire, for testing
// reactivation once the excluded label is removed.
func labelExcludedPlaceholderFile(stream string, issue int, labels string) string {
	return "---\n" +
		"schema: placeholder-v1\n" +
		"brief: " + stream + "/issue-" + strconv.Itoa(issue) + "\n" +
		"issue: " + strconv.Itoa(issue) + "\n" +
		"repo: example-org/tracker\n" +
		"wave: 0\n" +
		"status: done\n" +
		"labels: [" + labels + "]\n" +
		"resolved: label-excluded\n" +
		"---\nSee issue #" + strconv.Itoa(issue) + " — the issue body is the spec.\n"
}

// TestCloseOutRetireLabelGained — an OPEN issue whose placeholder
// already exists gains an excluded label (needs-decision) → the placeholder is
// retired (action: retire-label), not left dangling until issue-close.
func TestCloseOutRetireLabelGained(t *testing.T) {
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-640.md": placeholderFile("alpha", 640, "bug"),
	})

	// Issue 640 is still OPEN but now carries needs-decision.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 640, Title: "some bug", Labels: lbl("bug", "needs-decision")},
		},
	}

	plans, closeOuts, _ := planScan(root, streams, fixtureLister(data, ""), blessAll)

	// Still excluded from creation (would only matter if no placeholder
	// existed yet) and, since one DOES exist, no re-creation either.
	if findPlan(plans, scanHomeRepo(), 640) != nil {
		t.Error("issue 640 must not be planned for creation (excluded label)")
	}

	c := findCloseOut(closeOuts, scanHomeRepo(), 640)
	if c == nil {
		t.Fatalf("issue 640 placeholder should be retired (open issue gained an excluded label)")
	}
	if c.Action != "retire-label" {
		t.Errorf("action = %q, want retire-label", c.Action)
	}
}

// TestCloseOutRetireLabelGainedIdempotentSweeps — a placeholder already retired
// via the label-exclusion path (status: done) is not re-retired while the label
// is still present, but a copy still at the stream ROOT is swept into done/
// (D3: ANY done placeholder at the root is a ghost to archive).
func TestCloseOutRetireLabelGainedIdempotentSweeps(t *testing.T) {
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-641.md": labelExcludedPlaceholderFile("alpha", 641, "bug, needs-decision"),
	})

	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 641, Title: "still excluded", Labels: lbl("bug", "needs-decision")},
		},
	}

	_, closeOuts, _ := planScan(root, streams, fixtureLister(data, ""), blessAll)

	c := findCloseOut(closeOuts, scanHomeRepo(), 641)
	if c == nil || c.Action != "sweep" {
		t.Fatalf("done label-excluded root placeholder should be swept into done/, got %+v", c)
	}
	if filepath.Base(filepath.Dir(c.Dest)) != archiveDirName {
		t.Errorf("sweep dest = %q, want a %s/ path", c.Dest, archiveDirName)
	}
}

// TestCloseOutReactivateAfterLabelRemoved — symmetric half: a
// placeholder retired via label-exclusion (resolved: label-excluded), whose
// issue stays OPEN but has the excluded label removed, is reactivated the same
// way a reopen is — via the plain "reactivate" action.
func TestCloseOutReactivateAfterLabelRemoved(t *testing.T) {
	streams, root := scanFixtureRepo(t, map[string]string{
		"issue-642.md": labelExcludedPlaceholderFile("alpha", 642, "bug, needs-decision"),
	})

	// Issue 642 is still open, but needs-decision was removed.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 642, Title: "decision resolved", Labels: lbl("bug")},
		},
	}

	plans, closeOuts, _ := planScan(root, streams, fixtureLister(data, ""), blessAll)

	if findPlan(plans, scanHomeRepo(), 642) != nil {
		t.Error("issue 642 already has a placeholder — must not be re-created")
	}

	c := findCloseOut(closeOuts, scanHomeRepo(), 642)
	if c == nil {
		t.Fatalf("issue 642 placeholder should be reactivated (excluded label removed, issue still open)")
	}
	if c.Action != "reactivate" {
		t.Errorf("action = %q, want reactivate", c.Action)
	}
}

// TestCloseOutFileModificationRetireLabel — applyCloseOut actually rewrites the
// placeholder file for a retire-label action: status → done, resolved →
// label-excluded (distinct from the issue-close marker).
func TestCloseOutFileModificationRetireLabel(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)
	rootPath := filepath.Join(issueLoopDir, "issue-643.md")
	writeTemp(t, issueLoopDir, "issue-643.md", placeholderFile("issue-loop", 643, "bug"))

	// Issue 643 is open but now carries review-request.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 643, Title: "under review", Labels: lbl("bug", "review-request")},
		},
	}
	code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll)
	if code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}

	// (D3): retire-label MOVES the placeholder into done/ too.
	if _, err := os.Stat(rootPath); !os.IsNotExist(err) {
		t.Errorf("label-retired placeholder must be moved out of the stream root (stat err=%v)", err)
	}
	phPath := filepath.Join(issueLoopDir, archiveDirName, "issue-643.md")

	ph, ok, err := parsePlaceholderFile(phPath)
	if err != nil || !ok {
		t.Fatalf("retired file must parse: ok=%v err=%v", ok, err)
	}
	if ph.Status != "done" {
		t.Errorf("retired placeholder status = %q, want done", ph.Status)
	}

	raw, err := os.ReadFile(phPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "resolved: label-excluded") {
		t.Error("label-retired placeholder file must contain 'resolved: label-excluded'")
	}
	if strings.Contains(string(raw), "resolved: issue-close") {
		t.Error("label-retired placeholder file must NOT contain 'resolved: issue-close'")
	}
	if !strings.Contains(string(raw), "status: done") {
		t.Error("label-retired placeholder file must contain 'status: done'")
	}

	// Lint stays green.
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after label-retire exited %d, want 0", code)
	}
}

// TestCloseOutFileModificationReactivateAfterLabelRemoved — applyCloseOut
// rewrites a label-excluded-retired placeholder back to todo and removes
// resolved, same as an issue-close reactivate (symmetric half).
func TestCloseOutFileModificationReactivateAfterLabelRemoved(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	issueLoopDir := filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(issueLoopDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, issueLoopDir, "README.md", scanStreamREADME)
	phPath := filepath.Join(issueLoopDir, "issue-644.md")
	writeTemp(t, issueLoopDir, "issue-644.md", labelExcludedPlaceholderFile("issue-loop", 644, "bug, needs-decision"))

	// Issue 644 stays open; needs-decision was removed.
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 644, Title: "decision resolved", Labels: lbl("bug")},
		},
	}
	code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll)
	if code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}

	ph, ok, err := parsePlaceholderFile(phPath)
	if err != nil || !ok {
		t.Fatalf("reactivated file must parse: ok=%v err=%v", ok, err)
	}
	if ph.Status != "todo" {
		t.Errorf("reactivated placeholder status = %q, want todo", ph.Status)
	}

	raw, err := os.ReadFile(phPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "resolved:") {
		t.Error("reactivated placeholder file must NOT contain 'resolved:'")
	}
	if !strings.Contains(string(raw), "status: todo") {
		t.Error("reactivated placeholder file must contain 'status: todo'")
	}

	// Lint stays green.
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after reactivate exited %d, want 0", code)
	}
}

// ---------------------------------------------------------------------------
// Trust gate (trustgate.go)
// ---------------------------------------------------------------------------

// blessAll is the fixture blessChecker for pre-gate tests: every issue counts as
// blessed, so those tests keep exercising their original concern (labels, gates,
// idempotency, close-out) without also arranging trusted authors.
func blessAll(string, int) (bool, error) { return true, nil }

// blessNone quarantines everything (no ada blessing anywhere).
func blessNone(string, int) (bool, error) { return false, nil }

func TestTrustedAuthorTable(t *testing.T) {
	cases := map[string]bool{
		"ada":                        true,
		"Ada":                        true, // same account, case-insensitively unique
		"shared-agent":               true,
		"app/assay-desk-app":         true,
		"assay-desk-app[bot]":        true,
		"app/assay-issue-loop-app":   true,
		"assay-intake-loop-app[bot]": true,
		"":                           false,
		"external-user":              false,
		"assay-desk-app":             false, // bare slug = registrable username, rejected
		"ada2":                       false,
		"dependabot[bot]":            false,
	}
	for login, want := range cases {
		if got := trustedAuthor(login); got != want {
			t.Errorf("trustedAuthor(%q) = %v, want %v", login, got, want)
		}
	}
}

// TestScanTrustGateQuarantine: an untrusted-author issue with no blessing gets NO
// placeholder and a loud quarantine NOTICE; a trusted-author issue in the same
// sweep still gets its placeholder, and the bless checker runs ONLY for the
// untrusted author (bounded fetch).
func TestScanTrustGateQuarantine(t *testing.T) {
	streams, root := scanFixtureRepo(t, nil)
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 401, Title: "trusted work item", Author: ghAuthor{Login: "shared-agent"}, Labels: lbl("bug")},
			{Number: 402, Title: "external drive-by", Author: ghAuthor{Login: "external-user"}, Labels: lbl("bug")},
		},
	}
	var blessCalls []int
	bless := func(repo string, issue int) (bool, error) {
		blessCalls = append(blessCalls, issue)
		return false, nil
	}

	plans, _, notices := planScan(root, streams, fixtureLister(data, ""), bless)

	if len(plans) != 1 || plans[0].Issue != 401 {
		t.Fatalf("expected exactly the trusted issue 401 planned, got %+v", plans)
	}
	if len(blessCalls) != 1 || blessCalls[0] != 402 {
		t.Fatalf("bless checker must run ONLY for the untrusted author; calls=%v", blessCalls)
	}
	if !hasProblem(notices, "402", "quarantined") {
		t.Fatalf("expected a quarantine NOTICE for #402; got %v", notices)
	}
}

// TestScanTrustGateBlessedCreates: the same external-author issue WITH a ada
// blessing gets its placeholder.
func TestScanTrustGateBlessedCreates(t *testing.T) {
	streams, root := scanFixtureRepo(t, nil)
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 402, Title: "external but blessed", Author: ghAuthor{Login: "external-user"}, Labels: lbl("bug")},
		},
	}
	plans, _, notices := planScan(root, streams, fixtureLister(data, ""), blessAll)
	if len(plans) != 1 || plans[0].Issue != 402 {
		t.Fatalf("blessed external issue must be planned; plans=%+v notices=%v", plans, notices)
	}
}

// TestScanTrustGateUnverifiableFailsClosed: a failing blessing read means NO
// placeholder (fail closed) and a NOTICE — never a guess in either direction.
func TestScanTrustGateUnverifiableFailsClosed(t *testing.T) {
	streams, root := scanFixtureRepo(t, nil)
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 402, Title: "external", Author: ghAuthor{Login: "external-user"}, Labels: lbl("bug")},
		},
	}
	bless := func(string, int) (bool, error) { return false, fmt.Errorf("simulated network failure") }
	plans, _, notices := planScan(root, streams, fixtureLister(data, ""), bless)
	if len(plans) != 0 {
		t.Fatalf("unverifiable blessing must not create a placeholder; got %+v", plans)
	}
	if !hasProblem(notices, "402", "unverifiable") {
		t.Fatalf("expected an unverifiable NOTICE for #402; got %v", notices)
	}
}

// TestScanTrustGateEmptyAuthorQuarantined: a missing/empty author (deleted
// account, unexpected payload) is untrusted — fail closed.
func TestScanTrustGateEmptyAuthorQuarantined(t *testing.T) {
	streams, root := scanFixtureRepo(t, nil)
	data := map[string][]ghIssue{
		scanHomeRepo(): {
			{Number: 403, Title: "ghost author", Labels: lbl("bug")},
		},
	}
	plans, _, notices := planScan(root, streams, fixtureLister(data, ""), blessNone)
	if len(plans) != 0 {
		t.Fatalf("empty-author issue must be quarantined; got %+v", plans)
	}
	if !hasProblem(notices, "403", "quarantined") {
		t.Fatalf("expected a quarantine NOTICE for #403; got %v", notices)
	}
}

// TestEvalIssueBlessing covers the bless-then-edit rule on the raw GraphQL
// payload (duplicate of deskkit.BlessedByOctocat — keep the two in sync).
func TestEvalIssueBlessing(t *testing.T) {
	payload := func(bodyEdited string, comments ...string) []byte {
		le := "null"
		if bodyEdited != "" {
			le = `"` + bodyEdited + `"`
		}
		return []byte(`{"data":{"repository":{"issue":{"lastEditedAt":` + le +
			`,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` + strings.Join(comments, ",") + `]}}}}}`)
	}
	ada := `{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":null,"author":{"login":"ada","__typename":"User","databaseId":100001}}`
	fakeOctocat := `{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":null,"author":{"login":"ada","__typename":"User","databaseId":31337}}`
	lateExternal := `{"createdAt":"2026-07-22T10:00:00Z","lastEditedAt":null,"author":{"login":"someone","__typename":"User","databaseId":7}}`

	if ok, err := evalIssueBlessing(payload("", ada)); err != nil || !ok {
		t.Errorf("ada comment must bless: ok=%v err=%v", ok, err)
	}
	if ok, _ := evalIssueBlessing(payload("", fakeOctocat)); ok {
		t.Error("ada login with wrong databaseId must NOT bless (recycled-login defense)")
	}
	if ok, _ := evalIssueBlessing(payload("2026-07-22T10:00:00Z", ada)); ok {
		t.Error("body edited AFTER the blessing must re-quarantine")
	}
	if ok, _ := evalIssueBlessing(payload("", ada, lateExternal)); ok {
		t.Error("untrusted comment AFTER the blessing must re-quarantine")
	}
	// T10 wiring guard: a comment from login "ada" carrying the WRONG
	// numeric databaseId, posted AFTER the real ada blessing, must void
	// the blessing — trustedAuthorID returns false for wrong-id ada, so
	// the late comment is untrusted and re-quarantines. Reverting the call
	// site to trustedAuthor(e.login) would make this case PASS (green),
	// which is the untested hole this assertion covers.
	fakeOctocatLate := `{"createdAt":"2026-07-22T10:00:00Z","lastEditedAt":null,"author":{"login":"ada","__typename":"User","databaseId":31337}}`
	if ok, _ := evalIssueBlessing(payload("", ada, fakeOctocatLate)); ok {
		t.Error("wrong-id ada comment AFTER the real ada blessing must void it (trustedAuthorID wiring — reverting to trustedAuthor would make this PASS)")
	}
	if ok, err := evalIssueBlessing(payload("")); err != nil || ok {
		t.Errorf("no comments -> not blessed, no error: ok=%v err=%v", ok, err)
	}
	if _, err := evalIssueBlessing([]byte(`{not json`)); err == nil {
		t.Error("malformed payload must error (fail closed upstream)")
	}
}

// TestEvalIssueBlessingTie — T3: a same-second edit voids the blessing (fail-closed
// on ties). The old After check meant a body edit at the exact same timestamp as
// the blessing was a PASS; the fix (Before check) makes it fail.
func TestEvalIssueBlessingTie(t *testing.T) {
	payload := func(bodyEdited string, comments ...string) []byte {
		le := "null"
		if bodyEdited != "" {
			le = `"` + bodyEdited + `"`
		}
		return []byte(`{"data":{"repository":{"issue":{"lastEditedAt":` + le +
			`,"comments":{"pageInfo":{"hasNextPage":false},"nodes":[` + strings.Join(comments, ",") + `]}}}}}`)
	}
	// Same timestamp as the blessing — must now fail (T3: tie is fail-closed).
	ada := `{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":"2026-07-21T10:00:00Z","author":{"login":"ada","__typename":"User","databaseId":100001}}`
	if ok, _ := evalIssueBlessing(payload("2026-07-21T10:00:00Z", ada)); ok {
		t.Error("T3: body edited at same timestamp as blessing must void it (tie fail-closed)")
	}
	// Same timestamp untrusted comment edit — must now fail.
	comment := `{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":"2026-07-21T10:00:00Z","author":{"login":"ada","__typename":"User","databaseId":100001}}`
	untrusted := `{"createdAt":"2026-07-21T10:00:00Z","lastEditedAt":"2026-07-21T10:00:00Z","author":{"login":"external","__typename":"User","databaseId":99}}`
	if ok, _ := evalIssueBlessing(payload("", comment, untrusted)); ok {
		t.Error("T3: untrusted comment at same timestamp as blessing must void it (tie fail-closed)")
	}
}

// TestTrustedAuthorID — T10: trustedAuthorID requires numeric ID match for human
// accounts as recycled-login defense; bot accounts trusted by login alone.
func TestTrustedAuthorID(t *testing.T) {
	// ada with correct numeric ID → trusted.
	if !trustedAuthorID("ada", 100001) {
		t.Error("ada with id 100001 must be trusted")
	}
	// ada with wrong numeric ID → NOT trusted (recycled-login defense).
	if trustedAuthorID("ada", 31337) {
		t.Error("ada with wrong id must NOT be trusted")
	}
	// ada with zero id (caller has no numeric id) → NOT trusted (fail closed).
	if trustedAuthorID("ada", 0) {
		t.Error("ada with id 0 must NOT be trusted (fail closed without numeric id)")
	}
	// Bot accounts: trusted by login alone (id ignored).
	if !trustedAuthorID("assay-worker-app[bot]", 0) {
		t.Error("bot account with id 0 must be trusted (bots cannot be recycled)")
	}
	if !trustedAuthorID("app/assay-worker-app", 0) {
		t.Error("app/ prefixed bot with id 0 must be trusted")
	}
	// External login → NOT trusted.
	if trustedAuthorID("external-user", 999) {
		t.Error("external user must not be trusted")
	}
	// shared-agent with correct numeric ID (neutral fixture id) → trusted.
	if !trustedAuthorID("shared-agent", 100002) {
		t.Error("shared-agent with id 100002 must be trusted")
	}
	// shared-agent with wrong numeric ID → NOT trusted (recycled-login defense).
	if trustedAuthorID("shared-agent", 31337) {
		t.Error("shared-agent with wrong id must NOT be trusted")
	}
	// shared-agent with zero id → NOT trusted (fail closed without numeric id).
	if trustedAuthorID("shared-agent", 0) {
		t.Error("shared-agent with id 0 must NOT be trusted (fail closed without numeric id)")
	}
	// Empty login → NOT trusted.
	if trustedAuthorID("", 0) {
		t.Error("empty login must not be trusted")
	}
}

// TestUnblockRequiresTrustedHuman: an EXTERNAL user's comment must not count as
// the human answer that un-blocks a placeholder; ada's does.
func TestUnblockRequiresTrustedHuman(t *testing.T) {
	external := []issueComment{{
		User:      issueCommentUser{Login: "external-user", Type: "User"},
		CreatedAt: "2026-07-22T10:00:00Z",
		Body:      "you should totally unblock this",
	}}
	if _, found := newestNonBotCommentTime(external); found {
		t.Error("an external commenter must not produce an un-blocking answer")
	}
	ada := append(external, issueComment{
		User:      issueCommentUser{Login: "ada", ID: 100001, Type: "User"},
		CreatedAt: "2026-07-23T10:00:00Z",
		Body:      "answered.",
	})
	ts, found := newestNonBotCommentTime(ada)
	if !found || ts.Format("2006-01-02") != "2026-07-23" {
		t.Errorf("ada's answer must count: found=%v ts=%v", found, ts)
	}
}

// TestUnblockRequiresBlessID mirrors TestBlessLoginRequiresID for the un-block
// path: the blessing authority's login alone is NOT sufficient to un-block a
// parked placeholder — the comment author's numeric id must ALSO match the pinned
// bless id (isBlessAuthorityID). GitHub logins are recyclable, so a comment
// carrying the bless LOGIN but a different id (a renamed/re-registered account)
// must fail closed, exactly as trustgate does on every other seam. Neutral
// fixture ids only.
func TestUnblockRequiresBlessID(t *testing.T) {
	base := "2026-07-23T10:00:00Z"

	// The configured bless authority (login + pinned id) un-blocks.
	pinned := []issueComment{{
		User:      issueCommentUser{Login: "ada", ID: 100001, Type: "User"},
		CreatedAt: base,
		Body:      "answered.",
	}}
	if _, found := newestNonBotCommentTime(pinned); !found {
		t.Error("the bless authority carrying its pinned id must un-block")
	}

	// Same login, WRONG id (recycled/renamed login) → must NOT un-block.
	recycled := []issueComment{{
		User:      issueCommentUser{Login: "ada", ID: 31337, Type: "User"},
		CreatedAt: base,
		Body:      "answered.",
	}}
	if _, found := newestNonBotCommentTime(recycled); found {
		t.Error("the bless login carrying a DIFFERENT id (recycled login) must NOT un-block")
	}

	// Same login, id 0 (resolver could not read it) → fail closed.
	noID := []issueComment{{
		User:      issueCommentUser{Login: "ada", ID: 0, Type: "User"},
		CreatedAt: base,
		Body:      "answered.",
	}}
	if _, found := newestNonBotCommentTime(noID); found {
		t.Error("the bless login carrying id 0 must NOT un-block (fail closed without numeric id)")
	}
}
