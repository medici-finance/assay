package main

// evidenceactor_test.go — PROOF THIS CHECK CAN FAIL.
//
// The rule the fleet applies to a new gate is that it ships with a captured red
// run against a POSITIVE CONTROL, mutation-tested. For an Evidence-actor check the
// control that matters is not "does it pass the existing corpus" — a check that
// accepts everything passes that too. It is A ROW THAT SELF-ATTESTS: a brief whose
// `## Evidence` section was committed end-to-end by the IMPLEMENTING identity while
// its README row reads `verified`. That is the F-verify-self-attest shape, and
// TestEvidenceActorRealRepoPositiveControl builds one in a REAL git repository —
// real commits, real author identities, real `git blame` — and requires the check
// to go red on it and stay green on the verifier-committed twin beside it.
//
// A stubbed blame would not have proved that: the shipped failure mode of a check
// like this is misreading git, not misapplying a policy to a struct.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The identities below MUST agree with fixtureRoster (rosterfixture_test.go),
// which is the roster every test in this package runs under.
const (
	fixtureVerifierEmail = "300000005+assay-verifier-app[bot]@users.noreply.github.com"
	fixtureVerifierName  = "assay-verifier-app[bot]"
	fixtureWorkerEmail   = "300000006+assay-worker-app[bot]@users.noreply.github.com"
	fixtureWorkerName    = "assay-worker-app[bot]"
	fixtureHumanEmail    = "100001+ada@users.noreply.github.com"
	fixtureHumanName     = "Ada"
)

func TestGithubIdentityFromEmail(t *testing.T) {
	cases := []struct {
		email string
		login string
		id    int64
		ok    bool
	}{
		{fixtureVerifierEmail, "assay-verifier-app", 300000005, true},
		{fixtureHumanEmail, "ada", 100001, true},
		{"  100001+ada@users.noreply.github.com  ", "ada", 100001, true},
		// Not the noreply form: pins no GitHub account, so it can never accept.
		{"someone@example.com", "", 0, false},
		{"ada@users.noreply.github.com", "", 0, false}, // no id half
		{"0+ada@users.noreply.github.com", "", 0, false},
		{"not.committed.yet", "", 0, false},
		{"", "", 0, false},
	}
	for _, c := range cases {
		login, id, ok := githubIdentityFromEmail(c.email)
		if ok != c.ok || login != c.login || id != c.id {
			t.Errorf("githubIdentityFromEmail(%q) = (%q, %d, %v), want (%q, %d, %v)",
				c.email, login, id, ok, c.login, c.id, c.ok)
		}
	}
}

// TestActorRefMatchesPrefersID pins the trusted-signal decision: when the roster
// pinned a numeric id, the ID is the comparison and the login is ignored. A login
// re-registered by somebody else must NOT match.
func TestActorRefMatchesPrefersID(t *testing.T) {
	pinned := actorRef{Login: "assay-verifier-app", ID: 300000005}
	if !pinned.matches("assay-verifier-app", 300000005) {
		t.Error("the pinned identity must match itself")
	}
	if pinned.matches("assay-verifier-app", 999) {
		t.Error("SAME LOGIN, DIFFERENT ACCOUNT must not match a pinned ref — that is the " +
			"login-re-registration case the id exists to refuse")
	}
	if !pinned.matches("anything-else", 300000005) {
		t.Error("the id is the comparison; the display login is not consulted when pinned")
	}

	unpinned := actorRef{Login: "assay-verifier-app"}
	if !unpinned.matches("ASSAY-VERIFIER-APP", 42) {
		t.Error("an unpinned ref falls back to a case-insensitive login match")
	}
	if unpinned.matches("someone-else", 42) {
		t.Error("an unpinned ref must still reject a different login")
	}
}

func TestEvidenceActorPolicyFromRoster(t *testing.T) {
	p := evidenceActorPolicyFromRoster()
	if p.Unavailable != "" {
		t.Fatalf("the fixture roster binds a verifier role; policy must be available, got %q", p.Unavailable)
	}
	if p.Verifier.Login != "assay-verifier-app" || p.Verifier.ID != 300000005 {
		t.Fatalf("verifier ref = %+v, want the fixture roster's verifier= binding", p.Verifier)
	}
	if !p.idPinned() {
		t.Error("the fixture binding carries an id, so matching must report id-pinned")
	}
	if len(p.Humans) == 0 {
		t.Error("roster-known humans must be accepted actors too — a human really can verify")
	}
}

// TestEvidenceActorPolicyUnavailableIsCouldNotCheck is the fail-CLOSED control. An
// unconfigured roster must produce could-not-check and ZERO findings — never a
// corpus-wide "unbacked" verdict derived from the checker's own ignorance.
func TestEvidenceActorPolicyUnavailableIsCouldNotCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // a config home with no roster.env in it
	scanReloadConfig()
	t.Cleanup(scanReloadConfig)

	p := evidenceActorPolicyFromRoster()
	if p.Unavailable == "" {
		t.Fatal("an unconfigured roster must make the policy unavailable, not empty-but-usable")
	}

	got := evidenceActorNotices(t.TempDir(), []*Stream{{Name: "x"}})
	if len(got) != 1 {
		t.Fatalf("want exactly one could-not-check notice, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "could-not-check") {
		t.Errorf("the notice must say could-not-check in those words; got: %s", got[0])
	}
	if strings.Contains(got[0], "Rows:") {
		t.Errorf("a could-not-check run must name no flagged rows at all; got: %s", got[0])
	}
	if !strings.Contains(got[0], "did not run") {
		t.Errorf("the notice must say the check did not run, not that it found nothing; got: %s", got[0])
	}
}

func TestEvidenceActorClassify(t *testing.T) {
	p := evidenceActorPolicy{
		Verifier: actorRef{Login: "assay-verifier-app", ID: 300000005},
		Humans:   []actorRef{{Login: "ada", ID: 100001}},
	}
	cases := []struct {
		name, email string
		want        actorVerdict
		reasonHas   string
	}{
		{fixtureVerifierName, fixtureVerifierEmail, actorVerifier, "verifier App"},
		{fixtureHumanName, fixtureHumanEmail, actorHuman, "human:ada"},
		// The finding itself: the implementer's own identity does not back a row.
		{fixtureWorkerName, fixtureWorkerEmail, actorRejected, "does not accept"},
		// Right login, wrong account — the login-re-registration shape.
		{fixtureVerifierName, "999+assay-verifier-app[bot]@users.noreply.github.com", actorImpostor, "pins the verifier role to id"},
		// The local spoof: display name dressed as the verifier, address pins nobody.
		{fixtureVerifierName, "me@example.com", actorImpostor, "free text"},
		{"Someone", "me@example.com", actorRejected, "pins no GitHub account"},
		// Uncommitted lines are owned by nobody and cannot back a row.
		{"Not Committed Yet", "not.committed.yet", actorRejected, "pins no GitHub account"},
	}
	for _, c := range cases {
		got, reason := p.classify(c.name, c.email)
		if got != c.want {
			t.Errorf("classify(%q, %q) = %v, want %v (reason: %s)", c.name, c.email, got, c.want, reason)
		}
		if !strings.Contains(reason, c.reasonHas) {
			t.Errorf("classify(%q, %q) reason must name %q so the NOTICE is actionable; got: %s",
				c.name, c.email, c.reasonHas, reason)
		}
	}
}

func TestEvidenceLineRange(t *testing.T) {
	raw := "---\nschema: brief-v1\n---\n" + // 1..3
		"# Brief\n" + // 4
		"\n" + // 5
		"## Verify\n" + // 6
		"| # | Command |\n" + // 7
		"\n" + // 8
		"## Evidence\n" + // 9
		"row a\n" + // 10
		"row b\n" + // 11
		"\n" + // 12
		"## Review\n" + // 13
		"gate\n" // 14
	start, end, ok := evidenceLineRange(raw)
	if !ok || start != 10 || end != 12 {
		t.Fatalf("evidenceLineRange = (%d, %d, %v), want (10, 12, true)", start, end, ok)
	}

	// No heading, and a heading with an empty body, are both "no range" — the
	// empty-Evidence case is already a hard PROBLEM in checkBriefFiles and must
	// not be double-reported here.
	if _, _, ok := evidenceLineRange("# Brief\n## Review\n"); ok {
		t.Error("a file with no ## Evidence heading has no range")
	}
	if _, _, ok := evidenceLineRange("## Evidence\n## Review\n"); ok {
		t.Error("an Evidence heading with an empty body has no range")
	}
	// A decorated heading is NOT the section, matching extractEvidence exactly.
	if _, _, ok := evidenceLineRange("## Evidence (notes)\nrow\n"); ok {
		t.Error("`## Evidence (notes)` is not the brief-v1 Evidence heading")
	}
}

// TestBlamePorcelainAuthorsIgnoresNonContentLines pins the fail-open the positive
// control caught: a blank line, or a line of Evidence-contract boilerplate inside
// an HTML comment, is structure rather than evidence and must not attribute the
// section to whoever last touched it.
func TestBlamePorcelainAuthorsIgnoresNonContentLines(t *testing.T) {
	entry := func(sha string, n int, name, email, content string) string {
		return fmt.Sprintf("%s %d %d 1\nauthor %s\nauthor-mail <%s>\n\t%s\n", sha, n, n, name, email, content)
	}
	out := entry("aaa", 1, fixtureVerifierName, fixtureVerifierEmail, "") + // blank line
		entry("aaa", 2, fixtureVerifierName, fixtureVerifierEmail, "<!-- appended by a NON-implementer. -->") +
		entry("bbb", 3, fixtureWorkerName, fixtureWorkerEmail, "| 1 | `true` | pass |") +
		entry("bbb", 4, fixtureWorkerName, fixtureWorkerEmail, "| 2 | `true` | pass |")

	got := blamePorcelainAuthors(out)
	if len(got) != 1 || got[0].Email != fixtureWorkerEmail {
		t.Fatalf("only the content-bearing rows count; got %+v, want just the worker", got)
	}

	// A multi-line HTML comment must stay excluded across all of its lines, and
	// real content after the closing marker must still count.
	multi := entry("aaa", 1, fixtureVerifierName, fixtureVerifierEmail, "<!-- contract") +
		entry("aaa", 2, fixtureVerifierName, fixtureVerifierEmail, "still inside the comment") +
		entry("aaa", 3, fixtureVerifierName, fixtureVerifierEmail, "--> | 1 | real row |")
	got = blamePorcelainAuthors(multi)
	if len(got) != 1 || got[0].Email != fixtureVerifierEmail {
		t.Fatalf("content after a comment closes must count; got %+v", got)
	}
}

// TestEvidenceActorJudgeBlameErrorIsCouldNotCheck: a row whose blame fails must
// come back with Err set, counted neither clean nor flagged. Three-state at the
// row level.
func TestEvidenceActorJudgeBlameErrorIsCouldNotCheck(t *testing.T) {
	p := evidenceActorPolicy{Verifier: actorRef{Login: "assay-verifier-app", ID: 300000005}}
	rows := []evidenceActorRow{{ID: "a/01"}, {ID: "b/02"}}
	got := evidenceActorJudge(p, rows, func(i int) ([]blameAuthor, error) {
		if i == 0 {
			return nil, fmt.Errorf("boom")
		}
		return []blameAuthor{{Name: fixtureVerifierName, Email: fixtureVerifierEmail}}, nil
	})
	if got[0].Err == nil {
		t.Error("a failed blame must be could-not-check, not a verdict")
	}
	if got[1].Err != nil || got[1].Verdict != actorVerifier {
		t.Errorf("the readable row must still be judged: %+v", got[1])
	}

	// An empty author set is also could-not-check: blame that returned nothing
	// establishes nobody, and calling that "no accepted actor" would fail open
	// into a finding the run did not earn.
	empty := evidenceActorJudge(p, []evidenceActorRow{{ID: "c/03"}},
		func(int) ([]blameAuthor, error) { return nil, nil })
	if empty[0].Err == nil {
		t.Error("an empty blame result must be could-not-check")
	}
}

// TestEvidenceActorAcceptAnywhereInSection: an editorial pass by a non-verifier
// over PART of a verifier-written section must not invalidate the verification —
// the measured alternative (attributing the section to its last-touching commit)
// re-attributed 93 of 133 rows in this repo to a single repo-wide scrub commit.
func TestEvidenceActorAcceptAnywhereInSection(t *testing.T) {
	p := evidenceActorPolicy{Verifier: actorRef{Login: "assay-verifier-app", ID: 300000005}}
	got := evidenceActorJudge(p, []evidenceActorRow{{ID: "a/01"}}, func(int) ([]blameAuthor, error) {
		return []blameAuthor{
			{Name: fixtureWorkerName, Email: fixtureWorkerEmail},
			{Name: fixtureVerifierName, Email: fixtureVerifierEmail},
		}, nil
	})
	if got[0].Verdict != actorVerifier {
		t.Errorf("one verifier-owned line backs the section; got %v (%s)", got[0].Verdict, got[0].Reason)
	}
}

// ---------------------------------------------------------------------------
// The real-git positive control
// ---------------------------------------------------------------------------

func evActorGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// gitCommitAs commits the whole worktree under a chosen author identity — the
// forgery this check is meant to make visible is exactly this call, which is why
// the test performs it rather than describing it.
func gitCommitAs(t *testing.T, dir, name, email, msg string) {
	t.Helper()
	evActorGit(t, dir, "add", "-A")
	evActorGit(t, dir,
		"-c", "user.name="+name, "-c", "user.email="+email,
		"-c", "commit.gpgsign=false",
		"commit", "--no-verify", "-m", msg)
}

func briefWithEvidence(evidence string) string {
	return "---\n" +
		"schema: brief-v1\n" +
		"brief: s/01\n" +
		"title: t\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"authored: 2026-07-08 by t\n" +
		"sources: [\"s\"]\n" +
		"---\n" +
		"# Brief\n\n" +
		"## Verify\n\n| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | 0 |\n\n" +
		"## Evidence\n\n" + evidence + "\n\n" +
		"## Review\ngate: model\n"
}

// TestEvidenceActorRealRepoPositiveControl is THE control: a row that
// self-attests, built in a real repository, must turn the check red.
//
// Two streams, identical in every respect a prose-reading check can see — same
// `verified` status, same dated runner cell, same Evidence table naming an
// independent runner:
//
//	selfattest/01  Evidence committed by the WORKER App (the implementer)
//	backed/01      Evidence committed by the VERIFIER App
//
// attributionProblems passes BOTH (asserted below, so the gap this brief closes is
// pinned rather than asserted in prose). The Evidence-actor check separates them.
func TestEvidenceActorRealRepoPositiveControl(t *testing.T) {
	root := t.TempDir()
	evActorGit(t, root, "init", "-q", "-b", "main")

	evidenceTable := "| # | Command | Result | Date | Runner |\n" +
		"|---|---------|--------|------|--------|\n" +
		"| 1 | `true` | pass exit=0 | 2026-08-13 | independent verifier |\n"

	mk := func(stream string) string {
		dir := filepath.Join(root, "docs", "streams", stream)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief-01-x.md"),
			[]byte(briefWithEvidence(evidenceTable)), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	selfDir := mk("selfattest")
	backedDir := mk("backed")

	// One commit per identity. The whole difference between the two rows is which
	// identity authored the commit that carries its Evidence lines.
	gitCommitAs(t, root, fixtureWorkerName, fixtureWorkerEmail, "worker writes both briefs")
	if err := os.WriteFile(filepath.Join(backedDir, "brief-01-x.md"),
		[]byte(briefWithEvidence(evidenceTable+"| 2 | `true` | pass exit=0 | 2026-08-13 | verify-desk |\n")),
		0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitAs(t, root, fixtureVerifierName, fixtureVerifierEmail, "verifier commits Evidence for backed/01")

	streams := []*Stream{
		{Name: "selfattest", Dir: selfDir, Briefs: []Brief{{
			Num: "01", Status: "verified", Verified: "2026-08-13 opus-verifier"}}},
		{Name: "backed", Dir: backedDir, Briefs: []Brief{{
			Num: "01", Status: "verified", Verified: "2026-08-13 opus-verifier"}}},
	}

	// The prose-reading control passes BOTH rows. That is F-verify-self-attest.
	if got := attributionProblems(streams); len(got) != 0 {
		t.Fatalf("the prose-reading check is expected to pass both rows (that is the gap this "+
			"brief closes); it reported: %v", got)
	}

	notices := evidenceActorNotices(root, streams)
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "selfattest/01") {
		t.Fatalf("RED RUN MISSING: the self-attesting row must be flagged.\nnotices:\n%s", joined)
	}
	if strings.Contains(joined, "backed/01") {
		t.Fatalf("FALSE POSITIVE: the verifier-committed row must stay clean.\nnotices:\n%s", joined)
	}
	if !strings.Contains(joined, "F-verify-self-attest") {
		t.Errorf("the NOTICE must name the finding it closes; got:\n%s", joined)
	}
	if !strings.Contains(joined, "1 of 2") {
		t.Errorf("the NOTICE must state the flagged/judged counts so severity stays measurable; got:\n%s", joined)
	}
}

// TestEvidenceActorRealRepoForgedAuthorIsVisible records what the check does NOT
// buy, as an executable statement rather than a caveat in prose: a local commit
// made under the verifier's exact noreply address IS accepted. The check is
// tamper-EVIDENT (the forgery is now a named act in commit metadata that a
// reviewer can diff), not tamper-PROOF. Whoever promotes this to a hard gate must
// read this test first.
func TestEvidenceActorRealRepoForgedAuthorIsVisible(t *testing.T) {
	root := t.TempDir()
	evActorGit(t, root, "init", "-q", "-b", "main")
	dir := filepath.Join(root, "docs", "streams", "forged")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-01-x.md"),
		[]byte(briefWithEvidence("| 1 | `true` | pass | 2026-08-13 | v |\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	// The implementer, wearing the verifier's address.
	gitCommitAs(t, root, fixtureVerifierName, fixtureVerifierEmail, "forged")

	streams := []*Stream{{Name: "forged", Dir: dir, Briefs: []Brief{{
		Num: "01", Status: "verified", Verified: "2026-08-13 opus-verifier"}}}}
	if got := evidenceActorNotices(root, streams); len(got) != 0 {
		t.Fatalf("the residual is that a locally-forged author IS accepted; if this now fails, the "+
			"check gained a stronger anchor and the file header must be rewritten. got: %v", got)
	}
}

// TestEvidenceActorRealRepoUncommittedIsNotBacked: a row cannot be greened by
// editing the file. Blame runs against the working tree, so an uncommitted
// Evidence section is owned by nobody.
func TestEvidenceActorRealRepoUncommittedIsNotBacked(t *testing.T) {
	root := t.TempDir()
	evActorGit(t, root, "init", "-q", "-b", "main")
	dir := filepath.Join(root, "docs", "streams", "wip")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "brief-01-x.md")
	if err := os.WriteFile(path, []byte(briefWithEvidence("| 1 | `true` | pass | 2026-08-13 | v |\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitAs(t, root, fixtureVerifierName, fixtureVerifierEmail, "verifier commits the original")

	// Now the implementer rewrites the whole section without committing it.
	if err := os.WriteFile(path, []byte(briefWithEvidence(
		"| 1 | `true` | pass | 2026-08-13 | totally independent |\n"+
			"| 2 | `true` | pass | 2026-08-13 | also independent |\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	streams := []*Stream{{Name: "wip", Dir: dir, Briefs: []Brief{{
		Num: "01", Status: "verified", Verified: "2026-08-13 opus-verifier"}}}}
	joined := strings.Join(evidenceActorNotices(root, streams), "\n")
	if !strings.Contains(joined, "wip/01") {
		t.Fatalf("an uncommitted rewrite of the whole Evidence section must not read as backed; got:\n%s", joined)
	}
}
