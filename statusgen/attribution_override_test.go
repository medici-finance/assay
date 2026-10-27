package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// --- measured-status/05: the recorded, human-authorized override path ---------------
//
// The committer-identity cross-check's hard PROBLEM stays the default; these tests pin
// the ONE sanctioned way through it (an `independence-override:` frontmatter value) and
// every way that override is refused. The fixture roster (rosterfixture_test.go) maps the
// CURRENT human `alex` and the FORMER human `bob`.

const overrideHuman = "alex" // a current human in the fixture roster

// overrideRepo lays down the multi-identity fixture the identity tests use: brief-01 is
// the brief under test (authored, and its Evidence landed, by alice), brief-02 is the
// control whose distinct last toucher (bob) makes identity discriminating in the repo.
// It returns the root and brief-01's path.
func overrideRepo(t *testing.T) (root, b1 string) {
	t.Helper()
	gitAvailable(t)
	root = t.TempDir()
	writeIdentStream(t, root, []struct{ num, file string }{
		{"01", "brief-01-t.md"},
		{"02", "brief-02-t.md"},
	})
	attrGitInit(t, root)
	gitCommitAs(t, root, "Alice", "alice@example.com", "alice authors both briefs and lands their Evidence")
	b1 = filepath.Join(root, "docs", "streams", "gitattr", "brief-01-t.md")
	b2 := filepath.Join(root, "docs", "streams", "gitattr", "brief-02-t.md")
	appendLine(t, b2, "<!-- bob's independent touch on the control brief -->")
	gitCommitAs(t, root, "Bob", "bob@example.com", "bob touches brief-02 (control)")
	return root, b1
}

// setOverride writes `independence-override: <value>` into path's frontmatter (just
// before the closing ---), replacing any existing override line.
func setOverride(t *testing.T, path, value string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	var out []string
	closed := false
	for i, ln := range lines {
		if strings.HasPrefix(ln, "independence-override:") {
			continue
		}
		if i > 0 && ln == "---" && !closed {
			out = append(out, "independence-override: "+value)
			closed = true
		}
		out = append(out, ln)
	}
	if !closed {
		t.Fatal("fixture assumption broke: no closing frontmatter delimiter")
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func problemFor(problems []string, label string) string {
	for _, p := range problems {
		if strings.Contains(p, label) && strings.Contains(p, "committer-identity cross-check") {
			return p
		}
	}
	return ""
}

func validOverride() string {
	return `"human:` + overrideHuman + ` 2026-10-25 post-verification typo fix re-touched by the author identity"`
}

// TestAttributionIdentityOverrideHonored: a valid override — a current human, not the
// author, committed under a git author identity OTHER than the one excused, and the
// Evidence unchanged since it was recorded — turns the hard PROBLEM into a visible NOTICE. A later same-
// identity commit OUTSIDE the Evidence section (a mechanical title touch, which would
// otherwise re-arm the whole-file same-identity signal) does not invalidate it.
func TestAttributionIdentityOverrideHonored(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the human-authorized override")

	body, err := os.ReadFile(b1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b1, []byte(strings.Replace(string(body), "# Brief 01", "# Brief 01 (title touch)", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitAs(t, root, "Alice", "alice@example.com", "alice: unrelated mechanical title touch")

	problems, notices := attrProblemsAndNotices(t, root)
	if p := problemFor(problems, "gitattr/brief-01"); p != "" {
		t.Fatalf("a valid recorded override must lift the committer-identity PROBLEM; got:\n%s", p)
	}
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "gitattr/brief-01") || !strings.Contains(joined, "OVERRIDDEN") ||
		!strings.Contains(joined, "human:"+overrideHuman) {
		t.Fatalf("an honored override must be reported as a NOTICE naming the brief and the human; got:\n%s", joined)
	}
	// Offline, lint knows only that a different git author identity committed a line
	// naming the human; the NOTICE must say so, not assert the human's authorization.
	if !strings.Contains(joined, "does not corroborate") || strings.Contains(joined, "recorded human authorization") {
		t.Fatalf("the honored NOTICE must state that the human's authorization is not corroborated offline; got:\n%s", joined)
	}
}

// TestAttributionIdentityOverrideVisibleWhenRecorderClearsWholeFileSignal: the override's
// own commit (by a distinct identity) makes the whole-file last toucher differ from the
// author, so the fast pre-filter alone would never look at the brief again. A brief that
// CARRIES an override is still judged precisely, so the override is reported, never
// silently moot.
func TestAttributionIdentityOverrideVisibleWhenRecorderClearsWholeFileSignal(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the human-authorized override")

	problems, notices := attrProblemsAndNotices(t, root)
	if p := problemFor(problems, "gitattr/brief-01"); p != "" {
		t.Fatalf("a valid recorded override must not leave a PROBLEM; got:\n%s", p)
	}
	if joined := strings.Join(notices, "\n"); !strings.Contains(joined, "OVERRIDDEN") || !strings.Contains(joined, "gitattr/brief-01") {
		t.Fatalf("a brief carrying an override must be judged and the honored override reported; got notices:\n%s", joined)
	}
}

// TestAttributionIdentityOverrideSelfServeRefused: an override committed under the same
// git author identity as the one being excused is refused — even naming a real current
// human.
// The hard PROBLEM stands, and says why the override was not honored.
func TestAttributionIdentityOverrideSelfServeRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Alice", "alice@example.com", "alice records an override excusing herself")

	problems, notices := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if p == "" {
		t.Fatalf("an override recorded by the excused identity itself must NOT lift the PROBLEM; problems:\n%s\nnotices:\n%s",
			strings.Join(problems, "\n"), strings.Join(notices, "\n"))
	}
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "same git identity") {
		t.Errorf("the PROBLEM must say the override was refused as self-serve; got:\n%s", p)
	}
	if strings.Contains(strings.Join(notices, "\n"), "OVERRIDDEN") {
		t.Errorf("a refused override must not also be reported as honored; notices:\n%s", strings.Join(notices, "\n"))
	}
}

// TestAttributionIdentityOverrideSelfEditRefused: a distinct identity records a valid
// override, then the excused identity edits the override line (e.g. widens the
// reason). The line is now last written by the excused identity, so it is refused.
func TestAttributionIdentityOverrideSelfEditRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override")
	setOverride(t, b1, `"human:`+overrideHuman+` 2026-10-25 any future re-touch at all"`)
	gitCommitAs(t, root, "Alice", "alice@example.com", "alice rewrites the override")

	problems, _ := attrProblemsAndNotices(t, root)
	if p := problemFor(problems, "gitattr/brief-01"); !strings.Contains(p, "same git identity") {
		t.Fatalf("an override line last written by the excused identity must be refused; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestAttributionIdentityOverrideStaleRefused: the override covers the Evidence the
// human saw. A same-identity Evidence edit after it re-opens the PROBLEM.
func TestAttributionIdentityOverrideStaleRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override")
	appendLine(t, b1, "| 2 | `go vet ./...` | 0 | ok | 2026-10-26 | opus-verifier |")
	gitCommitAs(t, root, "Alice", "alice@example.com", "alice adds an Evidence row after the override")

	problems, _ := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "stale") {
		t.Fatalf("an override whose Evidence section changed after it was recorded must be refused as stale; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestAttributionIdentityOverrideStaleByUncommittedEvidenceRefused: staleness is judged
// on CONTENT, not on commit history, so an Evidence edit that was never committed also
// leaves the override behind.
func TestAttributionIdentityOverrideStaleByUncommittedEvidenceRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override")
	appendLine(t, b1, "| 2 | `go vet ./...` | 0 | ok | 2026-10-26 | opus-verifier |")

	problems, _ := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "stale") {
		t.Fatalf("an override whose Evidence changed since it was recorded (even uncommitted) must be refused as stale; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestAttributionIdentityOverrideUncommittedRefused: an override present only in the
// working tree has no committer, so who recorded it is unknown — refused.
func TestAttributionIdentityOverrideUncommittedRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())

	problems, _ := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "not committed") {
		t.Fatalf("an uncommitted override must be refused; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestAttributionIdentityOverrideWorkingTreeEditRefused: a distinct identity commits a
// valid override, then the working tree's override value is changed without a commit.
// git attributes the COMMITTED line, so the value being judged has no recorder of its
// own — refused, never honored on the committed line's provenance.
func TestAttributionIdentityOverrideWorkingTreeEditRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override")
	setOverride(t, b1, `"human:`+overrideHuman+` 2026-10-25 a wider reason nobody committed"`)

	problems, _ := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "matching the working tree") {
		t.Fatalf("an override value that differs from the committed line must be refused; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestAttributionIdentityNoOverrideStaysHard pins the default: with no override the
// same-identity pair is still the hard PROBLEM, and its remedy names the override.
func TestAttributionIdentityNoOverrideStaysHard(t *testing.T) {
	root, _ := overrideRepo(t)
	problems, notices := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if p == "" {
		t.Fatalf("no override: the same-identity pair must stay a hard PROBLEM; notices:\n%s", strings.Join(notices, "\n"))
	}
	if strings.Contains(p, "NOT honored") {
		t.Errorf("no override recorded, so no refusal clause is owed; got:\n%s", p)
	}
	if !strings.Contains(p, "independence-override") {
		t.Errorf("the PROBLEM's remedy must name the sanctioned override path; got:\n%s", p)
	}
}

// TestIndependenceOverrideRefusalNonGit covers the conditions decided before any git
// read: grammar, a CURRENT human, and not the author. None of these needs a repo, so a
// refusal here proves the check refuses before it ever consults history.
func TestIndependenceOverrideRefusalNonGit(t *testing.T) {
	const authored = "2026-07-08 by fable session (test)"
	cases := []struct {
		name, raw, authored, want string
	}{
		{"no-date", "human:alex because", authored, "does not read as"},
		{"no-reason", "human:alex 2026-10-25", authored, "does not read as"},
		{"not-a-calendar-date", "human:alex 2026-02-30 no such day", authored, "not a calendar date"},
		{"separator-only-reason", "human:alex 2026-10-25 — ", authored, "gives no reason"},
		{"relay-is-not-authorization", "on-behalf-of human:alex 2026-10-25 relayed", authored, "does not read as"},
		{"prose-mention", "per human:alex 2026-10-25 ok", authored, "does not read as"},
		{"homoglyph-name", "human:аlex 2026-10-25 confusable", authored, "does not read as"},
		{"former-human", "human:bob 2026-10-25 departed", authored, "former human"},
		{"unknown-name", "human:nobody_here 2026-10-25 not mapped", authored, "does not resolve"},
		{"author-excusing-own-work", "human:alex 2026-10-25 my own brief", "2026-07-08 by alex session", "own author"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, why := independenceOverrideRefusal(t.TempDir(), "x.md", "alice@example.com", c.raw, c.authored)
			if !strings.Contains(why, c.want) {
				t.Fatalf("override %q: want refusal containing %q, got %q", c.raw, c.want, why)
			}
		})
	}
	if ov, why := parseIndependenceOverride("human:Alex 2026-10-25 — a real reason"); why != "" || ov.name != "alex" || ov.date != "2026-10-25" || ov.reason != "a real reason" {
		t.Fatalf("a well-formed override must parse; got %+v why=%q", ov, why)
	}
}

// TestIndependenceOverrideIsCorroboratedOnline: the online half, as a unit test of the
// stamp scanner only. The `human:<name>` on an ADDED override line is a stamp that
// `--corroborate`, when it runs on the PR, must corroborate against that human's own act
// on the PR — it is not on a board row, so it can never be exempted as pre-existing.
// This test does not show that any CI lane runs `--corroborate` on such a PR.
func TestIndependenceOverrideIsCorroboratedOnline(t *testing.T) {
	diff := "diff --git a/docs/streams/s/brief-01-x.md b/docs/streams/s/brief-01-x.md\n" +
		"--- a/docs/streams/s/brief-01-x.md\n" +
		"+++ b/docs/streams/s/brief-01-x.md\n" +
		"@@ -1,3 +1,4 @@\n" +
		" ---\n" +
		" brief: s/01\n" +
		"+independence-override: \"human:alex 2026-10-25 post-verification re-touch\"\n" +
		" ---\n"
	stamps := stampsInDiff("", diff)
	for _, s := range stamps {
		if s.Name == "alex" && s.File == "docs/streams/s/brief-01-x.md" {
			if !s.Unresolved {
				t.Fatalf("an override stamp is not a board row and must never be exempted as pre-existing; got %+v", s)
			}
			return
		}
	}
	t.Fatalf("the added override's human:alex must be a stamp --corroborate checks; got %+v", stamps)
}

// writeOverrideFixtureFile is a small fixture helper: it writes content to root/rel, creating parents.
func writeOverrideFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAttributionIdentityOverrideRecorderIsCommitAuthor: the recorder of the override line
// is the author recorded in the commit object that last wrote it, read the same way as the
// identity it would excuse. A repository .mailmap changes neither side: an override the
// excused identity committed stays refused, and an override a distinct identity committed
// stays honored, whatever the mailmap folds together.
func TestAttributionIdentityOverrideRecorderIsCommitAuthor(t *testing.T) {
	t.Run("excused-identity-commit-stays-refused", func(t *testing.T) {
		root, b1 := overrideRepo(t)
		setOverride(t, b1, validOverride())
		writeOverrideFixtureFile(t, root, ".mailmap", "Carol <carol@example.com> <alice@example.com>\n")
		gitCommitAs(t, root, "Alice", "alice@example.com", "alice records an override and a mailmap entry")

		problems, notices := attrProblemsAndNotices(t, root)
		p := problemFor(problems, "gitattr/brief-01")
		if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "same git identity") {
			t.Fatalf("an override committed by the excused identity must stay refused under a .mailmap; problems:\n%s\nnotices:\n%s",
				strings.Join(problems, "\n"), strings.Join(notices, "\n"))
		}
		if strings.Contains(strings.Join(notices, "\n"), "OVERRIDDEN") {
			t.Fatalf("a refused override must not also read as honored; notices:\n%s", strings.Join(notices, "\n"))
		}
	})
	t.Run("distinct-identity-commit-stays-honored", func(t *testing.T) {
		root, b1 := overrideRepo(t)
		setOverride(t, b1, validOverride())
		writeOverrideFixtureFile(t, root, ".mailmap", "Alice <alice@example.com> <carol@example.com>\n")
		gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override and a mailmap entry")

		problems, notices := attrProblemsAndNotices(t, root)
		if p := problemFor(problems, "gitattr/brief-01"); p != "" {
			t.Fatalf("an override committed by a distinct identity must be honored whatever the .mailmap says; got:\n%s", p)
		}
		if !strings.Contains(strings.Join(notices, "\n"), "OVERRIDDEN") {
			t.Fatalf("the honored override must be reported; notices:\n%s", strings.Join(notices, "\n"))
		}
	})
}

// TestAttributionIdentityOverrideRecorderIndependentOfRunnerConfig: which commit last
// wrote the override line is read the same way on every runner. User-level git
// configuration that tells blame to skip a commit does not change the recorder, so an
// override line the excused identity last rewrote stays refused.
func TestAttributionIdentityOverrideRecorderIndependentOfRunnerConfig(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override")
	setOverride(t, b1, `"human:`+overrideHuman+` 2026-10-25 any future re-touch at all"`)
	gitCommitAs(t, root, "Alice", "alice@example.com", "alice rewrites the override")

	tmp := t.TempDir()
	revs, cfg := filepath.Join(tmp, "revs"), filepath.Join(tmp, "gitconfig")
	if err := os.WriteFile(revs, []byte(vfRev(t, root, "HEAD")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("[blame]\n\tignoreRevsFile = "+filepath.ToSlash(revs)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	// Positive control: the configuration is live in this git, and plain blame hands
	// the override line to the earlier commit.
	line := 0
	for i, l := range strings.Split(mustRead(t, b1), "\n") {
		if strings.HasPrefix(l, "independence-override:") {
			line = i + 1
		}
	}
	if line == 0 {
		t.Fatal("fixture assumption broke: no override line")
	}
	rel, err := filepath.Rel(root, b1)
	if err != nil {
		t.Fatal(err)
	}
	L := strconv.Itoa(line) + "," + strconv.Itoa(line)
	out, err := exec.Command("git", "-C", root, "blame", "--porcelain", "-L", L, "HEAD", "--", filepath.ToSlash(rel)).Output()
	if err != nil {
		t.Fatal(err)
	}
	if f := strings.Fields(string(out)); len(f) == 0 || f[0] != vfRev(t, root, "HEAD^") {
		t.Fatalf("positive control: the configuration must hand the line to the earlier commit; got %q", string(out))
	}

	problems, notices := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "same git identity") {
		t.Fatalf("an override line the excused identity last rewrote must stay refused whatever the runner's git configuration; problems:\n%s\nnotices:\n%s",
			strings.Join(problems, "\n"), strings.Join(notices, "\n"))
	}
	if strings.Contains(strings.Join(notices, "\n"), "OVERRIDDEN") {
		t.Fatalf("a refused override must not also read as honored; notices:\n%s", strings.Join(notices, "\n"))
	}
}

// TestAttributionIdentityOverrideInFixtureCorpusRefused: a brief inside a declared fixture
// corpus has its human:<name> stamps skipped by --corroborate, so an override there could
// never be corroborated online. It is refused, and the refusal says why.
func TestAttributionIdentityOverrideInFixtureCorpusRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, validOverride())
	writeOverrideFixtureFile(t, root, "docs/streams/gitattr/"+fixtureCorpusMarkerName, "")
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override; the stream declares a fixture corpus")

	problems, notices := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "fixture corpus") {
		t.Fatalf("an override inside a declared fixture corpus must be refused; problems:\n%s\nnotices:\n%s",
			strings.Join(problems, "\n"), strings.Join(notices, "\n"))
	}
}

// TestAttributionIdentityOverrideDateAfterRecordingRefused: the override's date is when the
// human authorized it, so it cannot be later than the commit that recorded it.
func TestAttributionIdentityOverrideDateAfterRecordingRefused(t *testing.T) {
	root, b1 := overrideRepo(t)
	setOverride(t, b1, `"human:`+overrideHuman+` 2999-01-01 dated after the commit that records it"`)
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records an override dated in the future")

	problems, _ := attrProblemsAndNotices(t, root)
	p := problemFor(problems, "gitattr/brief-01")
	if !strings.Contains(p, "NOT honored") || !strings.Contains(p, "later than the commit") {
		t.Fatalf("an override dated after its recording commit must be refused; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestAttributionIdentityOverrideForcedInUnreadableEvidenceNotice: a brief that reaches the
// precise pass only because it carries an override (its whole-file authoring and newest
// commit are under different identities) and whose Evidence history cannot be read is
// reported as an override that could not be judged. It is not described as a brief whose
// whole-file authoring and most-recent commit share one identity.
func TestAttributionIdentityOverrideForcedInUnreadableEvidenceNotice(t *testing.T) {
	root, b1 := overrideRepo(t)
	body, err := os.ReadFile(b1)
	if err != nil {
		t.Fatal(err)
	}
	// A heading the brief parser reads as Evidence but the line-range history walk
	// does not locate, so the Evidence section's history is unreadable.
	if err := os.WriteFile(b1, []byte(strings.Replace(string(body), "## Evidence\n", "## Evidence \n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitAs(t, root, "Alice", "alice@example.com", "alice touches the Evidence heading")
	setOverride(t, b1, validOverride())
	gitCommitAs(t, root, "Carol", "carol@example.com", "carol records the override")

	_, notices := attrProblemsAndNotices(t, root)
	joined := strings.Join(notices, "\n")
	for _, n := range notices {
		if strings.Contains(n, "gitattr/brief-01") && strings.Contains(n, "whose whole-file authoring and most-recent commit share one identity") {
			t.Fatalf("a brief in the precise pass only because of its override must not be described as same-identity whole-file; got:\n%s", n)
		}
	}
	if !strings.Contains(joined, "gitattr/brief-01") || !strings.Contains(joined, "could not be judged") {
		t.Fatalf("the unjudged override must be reported by name; notices:\n%s", joined)
	}
}
