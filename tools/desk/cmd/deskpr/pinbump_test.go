package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// pinbump_test.go — #328.
//
// `.assay-versions` is how every consumer repo pins a verified statusgen / desk-tools
// binary, so a pin bump is the single most routine edit made to that file. deskpr
// refused every one of them, and the cause was NOT the file's content: git's hunk-header
// funcname is a TRUNCATED echo of a line above the hunk, and a 64-hex sha256 cut to 53
// hex is neither 40 nor 64, so deskkit's git-SHA exemption misses it and the run scans as
// a secret.
//
// These tests drive real git — a real repo, a real commit, a real `git diff`, a real
// `git push` to a local bare origin — because the failure lives in the outgoing path. A
// hand-written diff string would only test the string I chose to write; git's truncation
// (80 bytes of funcname, kept verbatim from the file) is the thing under test.

// assayVersionsBefore is a verbatim slice of the real
// example-org/tracker `.assay-versions` at statusgen/v0.6.0, including
// the line-16 `statusgen …` pin that git adopts as the funcname for every hunk below it
// and the per-platform block ~15 lines further down that a bump actually edits. The
// distance between them is what makes the bug structural: at the default -U3 the two can
// never merge into one headerless hunk, so no pin bump can avoid the header.
const assayVersionsBefore = `# Pinned assay release artifacts -- tag + sha256 per artifact.
# Consumers verify the downloaded binary against the hash committed here,
# so a re-tagged release cannot silently substitute a binary.
#
# Format: <artifact> <tag> <sha256>
#
# Cross-org access to a private release repo (here medici-finance/assay) is via a
# runtime-generated GitHub App installation token, minted per CI run rather than
# stored. A repo secret holds the App private key; each workflow generates a
# short-lived installation token before downloading the pinned binary, so no
# long-lived cross-org credential sits in the tree or the environment.
#
# For local ` + "`sudo make desk-install`" + `, the invoking user's ` + "`gh`" + ` auth handles
# credentials (a developer with read access to the release repo).

statusgen statusgen/v0.6.0 a9145f3aa48deb0d85a2b42944381a02aea10a7d7b9154592d26b8337a74779b  # linux-amd64 (CI-built)

# Per-platform statusgen pins, same three-field shape as desk-tools below.
# The bare ` + "`statusgen`" + ` line ABOVE stays exactly as it is: all four workflows that
# download the binary select their pin with ` + "`grep '^statusgen '`" + ` (trailing space),
# which never matches ` + "`statusgen-<platform>`" + ` — so these lines are purely additive
# and change no CI behaviour.
#
# They exist because ` + "`make statusgen-install`" + ` (root Makefile) selects by DETECTED
# platform. Until they were added, only linux-amd64 had a committed hash, so a mac
# install could only have "verified" against the release's own checksums.txt.
statusgen-linux-amd64  statusgen/v0.6.0 a9145f3aa48deb0d85a2b42944381a02aea10a7d7b9154592d26b8337a74779b  # CI runners
statusgen-darwin-arm64 statusgen/v0.6.0 59854068373e13b41b61b920224327e5d2cf91a16d4ab5920dddad8e2f7bae8d  # primary dev
statusgen-darwin-amd64 statusgen/v0.6.0 b32a96393c26c8e224598832a4c39ff9c4c9f463f378af20c6b7f0e159e61b01  # intel mac
`

// Credential-SHAPED fixtures are assembled from SPLIT fragments, and must stay that way.
// deskpr secret-scans the branch diff before it pushes, so a contiguous credential literal
// on an ADDED line refuses the very PR that adds the test — the guard doing its job on its
// own repo. Runtime values are exact; deskkit's bodycheck_test.go splits the AWS example
// key for the same reason.
const (
	// fixtureSecret40 is a 40-char base64ish run with no `/`: nothing exempts it.
	fixtureSecret40 = "Qx7pLk2wZt9mNc4bYf6RhVs8" + "Ju3XoAeG5idWn1Dz"
	// truncatedPinHex is what git actually emits as the funcname for a `.assay-versions`
	// pin bump: the line-16 sha256 cut at 80 bytes, leaving 53 of its 64 hex characters.
	truncatedPinHex = "a9145f3aa48deb0d85a2b42944381a0" + "2aea10a7d7b9154592d26b"
)

// The outgoing pin — a real 64-hex sha256 shape that must survive the whole path.
const bumpedDarwinARM64 = "4c1d2e3f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d"

// assayVersionsAfter is assayVersionsBefore with the darwin-arm64 pin bumped
// v0.6.0 → v0.7.0 — an ordinary, minimal pin bump.
var assayVersionsAfter = strings.Replace(assayVersionsBefore,
	"statusgen-darwin-arm64 statusgen/v0.6.0 59854068373e13b41b61b920224327e5d2cf91a16d4ab5920dddad8e2f7bae8d",
	"statusgen-darwin-arm64 statusgen/v0.7.0 "+bumpedDarwinARM64, 1)

// newPinBumpFixture builds the same offline fixture as newBaseFixture (origin URL parses
// to an allowed repo, pushes route to a local bare repo) but seeds `.assay-versions` on
// MAIN and commits a pin bump on the feature branch — so the branch diff deskpr scans is
// a real pin-bump diff carrying git's real truncated funcname. Returns the worktree and
// the bare origin path.
func newPinBumpFixture(t *testing.T) (work, bare string) {
	t.Helper()
	root := t.TempDir()
	bare = filepath.Join(root, "origin.git")
	work = filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "main", bare)
	mustGit(t, "", "init", "-b", "main", work)
	mustGit(t, work, "config", "user.email", "t@e.st")
	mustGit(t, work, "config", "user.name", "Test")
	mustGit(t, work, "config", "commit.gpgsign", "false")
	mustGit(t, work, "remote", "add", "origin", ghURL)
	mustGit(t, work, "remote", "set-url", "--push", "origin", "file://"+bare)

	writeFile(t, filepath.Join(work, ".assay-versions"), assayVersionsBefore)
	mustGit(t, work, "add", ".assay-versions")
	mustGit(t, work, "commit", "-m", "seed pins at statusgen/v0.6.0")
	mainSHA := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mainSHA)
	mustGit(t, work, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	mustGit(t, work, "checkout", "-b", "feature/test-branch")
	writeFile(t, filepath.Join(work, ".assay-versions"), assayVersionsAfter)
	mustGit(t, work, "add", ".assay-versions")
	mustGit(t, work, "commit", "-m", "chore(pins): bump statusgen darwin-arm64 to v0.7.0")
	return work, bare
}

// TestGitTruncatesPinFuncname is the PREMISE check for the two tests below: it asserts
// that git really does emit a TRUNCATED copy of the line-16 statusgen pin as
// the hunk-header funcname, and that the truncated hex is neither 40 nor 64 characters —
// i.e. that it falls outside deskkit's git-SHA exemption by construction.
//
// It exists so that a future git whose funcname behaviour changed would fail HERE with a
// clear message, rather than leaving the fix tests passing vacuously against a diff that
// no longer contains the thing they were written to cover.
func TestGitTruncatesPinFuncname(t *testing.T) {
	work, _ := newPinBumpFixture(t)
	diff := mustGit(t, work, "diff", "origin/main...HEAD")

	var header string
	for _, ln := range strings.Split(diff, "\n") {
		if strings.HasPrefix(ln, "@@ ") {
			header = ln
			break
		}
	}
	if header == "" {
		t.Fatalf("no hunk header in the pin-bump diff:\n%s", diff)
	}
	const marker = "@@ statusgen statusgen/v0.6.0 "
	i := strings.Index(header, marker)
	if i < 0 {
		t.Fatalf("hunk header does not carry the line-16 statusgen pin as its funcname —\n"+
			"the #328 premise no longer holds; header was:\n%s", header)
	}
	hex := header[i+len(marker):]
	if len(hex) == 40 || len(hex) == 64 {
		t.Fatalf("funcname hex is %d chars — NOT truncated, so deskkit's git-SHA exemption "+
			"would already cover it and #328 would not reproduce; header:\n%s", len(hex), header)
	}
	t.Logf("git truncated the funcname sha256 to %d hex chars (full value is 64): %s", len(hex), header)
}

// TestCreatePinBumpIsNotRefused is the #328 regression test, end to end: a real
// `.assay-versions` pin bump must go through `deskpr create`.
//
// It asserts on the outgoing artefact, not just the exit code — the pushed branch in the
// bare origin must carry the bumped file BYTE FOR BYTE. A fix that let the command
// succeed while mangling what it sent would pass an exit-code-only test.
func TestCreatePinBumpIsNotRefused(t *testing.T) {
	work, bare := newPinBumpFixture(t)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "chore(pins): bump statusgen to v0.7.0",
		"--body-min", "Re-pins statusgen to the newly cut release."})
	if rc != deskkit.ExitOK {
		t.Fatalf("pin-bump create rc = %d, want 0 — deskpr is still refusing "+
			"the most routine edit made to .assay-versions (#328)", rc)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("no push happened; git calls: %v", gitCalls(*calls))
	}

	// The content ARRIVED, intact: read it back out of the pushed bare repo.
	got := mustGit(t, "", "-C", bare, "show", "feature/test-branch:.assay-versions")
	if got != strings.TrimRight(assayVersionsAfter, "\n") {
		t.Fatalf("pushed .assay-versions differs from what was committed;\ngot:\n%s", got)
	}
	if !strings.Contains(got, "statusgen-darwin-arm64 statusgen/v0.7.0 "+bumpedDarwinARM64) {
		t.Fatalf("the bumped 64-hex pin did not arrive intact in the pushed branch:\n%s", got)
	}
	t.Logf("pushed pin line: statusgen-darwin-arm64 statusgen/v0.7.0 %s", bumpedDarwinARM64)
}

// TestPinBumpScansAddedLines is the counterweight: the #328 fix drops only git-generated
// funcname text from hunk headers, so a secret introduced on a CONTENT line of the very
// same pin-bump diff must still be refused, and nothing may be pushed.
func TestPinBumpScansAddedLines(t *testing.T) {
	work, _ := newPinBumpFixture(t)
	calls := withEnv(t, work)

	writeFile(t, filepath.Join(work, ".assay-versions"), assayVersionsAfter+"# token "+fixtureSecret40+"\n")
	mustGit(t, work, "add", ".assay-versions")
	mustGit(t, work, "commit", "-m", "oops")

	rc := run([]string{"create", "--title", "chore(pins): bump", "--body-min", "bump"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want 5 — a secret on an ADDED line must still refuse", rc)
	}
	assertNoPushNoCreate(t, *calls)
}

// TestRefusalNamesItsSurface covers the issue's second ask. Every refusal used
// to say "body", whichever surface tripped it, so two diagnostic rounds went into
// discovering that the trigger was the branch diff while the first two attempts rewrote a
// body that was never the problem. A refusal that misidentifies its own surface converts
// a one-line fix into an investigation.
func TestRefusalNamesItsSurface(t *testing.T) {

	t.Run("diff", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		writeFile(t, filepath.Join(work, "leak.txt"), fixtureSecret40+"\n")
		mustGit(t, work, "add", "leak.txt")
		mustGit(t, work, "commit", "-m", "leak")

		err := cmdCreate([]string{"--title", "t", "--body-min", "clean body"})
		if err == nil {
			t.Fatal("want a refusal")
		}
		if !strings.Contains(err.Error(), "branch diff") {
			t.Fatalf("a diff-triggered refusal must name the diff, got: %v", err)
		}
		if strings.Contains(err.Error(), "body contains") {
			t.Fatalf("a diff-triggered refusal still blames the body, got: %v", err)
		}
	})

	t.Run("title", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		err := cmdCreate([]string{"--title", "t " + fixtureSecret40, "--body-min", "clean body"})
		if err == nil {
			t.Fatal("want a refusal")
		}
		if !strings.Contains(err.Error(), "PR title") {
			t.Fatalf("a title-triggered refusal must name the title, got: %v", err)
		}
	})

	t.Run("body", func(t *testing.T) {
		work := newBaseFixture(t)
		withEnv(t, work)
		err := cmdCreate([]string{"--title", "t", "--body-min", "here it is " + fixtureSecret40})
		if err == nil {
			t.Fatal("want a refusal")
		}
		if !strings.Contains(err.Error(), "PR body") {
			t.Fatalf("a body-triggered refusal must name the body, got: %v", err)
		}
	})
}

// TestStripHunkFuncname pins the strip at line granularity: the `@@ … @@` RANGE survives
// (it is deterministic, carries no file content, and keeping it preserves the diff's
// shape), the funcname after it is dropped, and a hunk header inside a doc fragment —
// one with no `diff --git` line, so not git output at all — is left alone.
func TestStripHunkFuncname(t *testing.T) {
	in := strings.Join([]string{
		"diff --git a/.assay-versions b/.assay-versions",
		"index 9c77ca2..f23d684 100644",
		"--- a/.assay-versions",
		"+++ b/.assay-versions",
		"@@ -27,5 +27,5 @@ statusgen statusgen/v0.6.0 " + truncatedPinHex,
		"-statusgen-darwin-arm64 statusgen/v0.6.0 59854068373e13b41b61b920224327e5d2cf91a16d4ab5920dddad8e2f7bae8d",
		"+statusgen-darwin-arm64 statusgen/v0.7.0 " + bumpedDarwinARM64,
	}, "\n")

	out := stripDiffMetaLines(in)

	if strings.Contains(out, truncatedPinHex) {
		t.Errorf("truncated funcname hex survived the strip:\n%s", out)
	}
	if !strings.Contains(out, "@@ -27,5 +27,5 @@") {
		t.Errorf("the hunk RANGE must survive; out:\n%s", out)
	}
	// Content survives with its leading -/+ diff marker stripped (#781).
	for _, mustHave := range []string{
		"statusgen-darwin-arm64 statusgen/v0.6.0 59854068373e13b41b61b920224327e5d2cf91a16d4ab5920dddad8e2f7bae8d",
		"statusgen-darwin-arm64 statusgen/v0.7.0 " + bumpedDarwinARM64,
	} {
		if !strings.Contains(out, mustHave) {
			t.Errorf("stripDiffMetaLines dropped a content line it must keep: %q\nout:\n%s", mustHave, out)
		}
	}
	if err := deskkit.BodyCheck([]byte(out)); err != nil {
		t.Errorf("the stripped pin-bump diff must scan clean, got: %v", err)
	}

	// A doc fragment is not git output: the scanner never enters header mode, so its
	// hunk header (funcname and all) is kept and scanned like any other prose.
	doc := "The diff looked like:\n@@ -1,2 +1,2 @@ func Example() {\n-old\n+new\n"
	if got := stripDiffMetaLines(doc); got != doc {
		t.Errorf("a doc fragment must pass through unchanged;\nwant:\n%s\ngot:\n%s", doc, got)
	}
}

// TestStripFuncnameOnlyTail is the security-facing half of the strip: only the funcname
// tail goes, and only on a well-formed `@@ … @@` header. A malformed
// header line is kept WHOLE (scan more, not less), and a content line that merely starts
// with `@@` inside a hunk — which the unified diff renders with a leading marker — is
// untouched.
func TestStripFuncnameOnlyTail(t *testing.T) {
	in := strings.Join([]string{
		"diff --git a/notes.txt b/notes.txt",
		"--- a/notes.txt",
		"+++ b/notes.txt",
		"@@ -1,3 +1,3 @@ func Example() {",
		"+@@ not a header, just content " + fixtureSecret40,
		" @@ context line that begins with at-at " + fixtureSecret40,
		"@@ malformed header " + fixtureSecret40,
	}, "\n")

	out := stripDiffMetaLines(in)

	if n := strings.Count(out, fixtureSecret40); n != 3 {
		t.Fatalf("want all 3 secret-bearing lines kept for scanning, kept %d:\n%s", n, out)
	}
	if strings.Contains(out, "func Example() {") {
		t.Errorf("the funcname on the well-formed header should have been dropped:\n%s", out)
	}
	if err := deskkit.BodyCheck([]byte(out)); err == nil {
		t.Error("the surviving content lines must still refuse — they carry a 40-char secret")
	}
}

// TestPinBumpFixtureShape guards the fixture itself: if the real `.assay-versions` ever
// loses the line-16 `statusgen ` pin or the per-platform block, the tests above would
// still pass while no longer covering the real file.
func TestPinBumpFixtureShape(t *testing.T) {
	lines := strings.Split(assayVersionsBefore, "\n")
	if len(lines) < 16 || !strings.HasPrefix(lines[15], "statusgen statusgen/") {
		t.Fatalf("fixture line 16 is no longer the bare statusgen pin: %q", lines[15])
	}
	for _, want := range []string{"statusgen-linux-amd64", "statusgen-darwin-arm64", "statusgen-darwin-amd64"} {
		if !strings.Contains(assayVersionsBefore, want) {
			t.Errorf("fixture lost the %s pin line", want)
		}
	}
}
