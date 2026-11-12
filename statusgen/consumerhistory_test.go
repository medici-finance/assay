package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The post-merge shape #6099 names: a verifier home detached at origin/main,
// the brief's committed Evidence carries no Verification-Table marker, and no
// --delivery/--base is passed. The delivery is then read from the checked-out
// history: the one first-parent commit that names `Brief: <stream>/<NN>` in its
// own message (a squash) or in a commit it merged (a merge commit).

type historyShape struct {
	merge    bool     // land the work through a --no-ff merge commit
	messages []string // one implementation commit per message, in order
}

// historyFixture lands the work for alpha/01 on the mainline, adds a verifier
// Evidence commit that carries no marker, points origin/main at it and detaches
// HEAD there. It returns the root, the delivery's first parent and the delivery.
func historyFixture(t *testing.T, routes []string, shape historyShape) (root, base, delivery string) {
	t.Helper()
	root = consumersFixture(t, routes, map[string]string{"web/fixed.go": "before\n", "web/untouched.go": "before\n", "web/excluded.go": "before\n"})
	mustGitInit(t, root)
	runGit(t, root, "commit", "--allow-empty", "-qm", "parent anchor")
	base = mustGitOut(t, root, "rev-parse", "HEAD")
	mainline := mustGitOut(t, root, "symbolic-ref", "--short", "HEAD")
	if shape.merge {
		runGit(t, root, "checkout", "-qb", "side")
	}
	for i, msg := range shape.messages {
		mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivered "+string(rune('a'+i))+"\n")
		runGit(t, root, "add", "-A")
		runGit(t, root, "-c", "commit.gpgsign=false", "commit", "-q", "-m", msg)
	}
	if shape.merge {
		runGit(t, root, "checkout", "-q", mainline)
		runGit(t, root, "-c", "commit.gpgsign=false", "merge", "--no-ff", "-q", "-m", "Merge pull request #7 from fixture/side", "-m", "A title-only merge message", "side")
	}
	delivery = mustGitOut(t, root, "rev-parse", "HEAD")
	appendBrief(t, root, "\n## Evidence\n\n| # | Result |\n|---|--------|\n| 1 | PASS |\n")
	mustGitCommitAll(t, root, "verifier Evidence commit")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	runGit(t, root, "checkout", "-q", "--detach", "HEAD")
	return root, base, delivery
}

func trailer(subject, brief string) string { return subject + "\n\nBrief: " + brief }

func wantDerivedPass(t *testing.T, root, base, delivery string) {
	t.Helper()
	bin := buildStatusgenBinary(t)
	code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01")
	if code != 0 || !strings.Contains(stdout, "summary: 1 corroborated, 0 disproved, 0 unchecked") {
		t.Fatalf("bare --consumers --brief, delivery in history: exit %d\n%s\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, base+".."+delivery+" (immutable delivered snapshot)") {
		t.Fatalf("the derived run must name its pinned range %s..%s:\n%s", base, delivery, stdout)
	}
	if !strings.Contains(stderr, "derived from history: "+delivery) {
		t.Fatalf("the derived delivery and its source must be named on stderr:\n%s", stderr)
	}
}

func wantCouldNotCheck(t *testing.T, root string, needles ...string) {
	t.Helper()
	bin := buildStatusgenBinary(t)
	code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01")
	if code != 2 || strings.Contains(stdout, "summary:") {
		t.Fatalf("want COULD-NOT-CHECK (exit 2, no summary): exit %d\n%s\n%s", code, stdout, stderr)
	}
	for _, n := range needles {
		if !strings.Contains(stderr, n) {
			t.Fatalf("stderr must say %q:\n%s", n, stderr)
		}
	}
}

// regression: #6099
func TestDerivedHistoryMergeCommit(t *testing.T) {
	// GitHub's merge message is title-only: the trailer lives on the commits
	// the merge introduced. The delivery is the merge, pinned to its first parent.
	for name, msgs := range map[string][]string{
		"one commit":  {trailer("implement", "alpha/01")},
		"two commits": {trailer("implement", "alpha/01"), trailer("address review", "alpha/01")},
	} {
		t.Run(name, func(t *testing.T) {
			root, base, delivery := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{merge: true, messages: msgs})
			if got := mustGitOut(t, root, "rev-parse", delivery+"^1"); got != base {
				t.Fatalf("fixture: merge first parent %s, want %s", got, base)
			}
			wantDerivedPass(t, root, base, delivery)
		})
	}
}

// regression: #6099
func TestDerivedHistorySquash(t *testing.T) {
	root, base, delivery := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{trailer("implement (#7)", "alpha/01")}})
	wantDerivedPass(t, root, base, delivery)
}

// regression: #6099
func TestDerivedHistoryNoMatch(t *testing.T) {
	// Nothing in history names the brief: could-not-check, naming both the
	// missing marker and what the history search covered.
	for name, msg := range map[string]string{
		"no trailer":         "implement",
		"another brief":      trailer("implement", "alpha/02"),
		"a longer number":    trailer("implement", "alpha/010"),
		"trailer in a fence": "implement\n\n```\nBrief: alpha/01\n```",
	} {
		t.Run(name, func(t *testing.T) {
			root, _, _ := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{msg}})
			wantCouldNotCheck(t, root, "no Verification-Table marker", "no first-parent commit of HEAD names `Brief: alpha/01`")
		})
	}
}

// regression: #6099
func TestDerivedHistoryAmbiguous(t *testing.T) {
	// Two first-parent commits name the brief: the instrument never picks one.
	root, _, _ := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{trailer("implement", "alpha/01"), trailer("follow-up", "alpha/01")}})
	wantCouldNotCheck(t, root, "no Verification-Table marker", "ambiguous", "2 first-parent commits")
}

// regression: #6099
func TestDerivedHistoryFlagsWin(t *testing.T) {
	// Precedence: explicit --delivery/--base, then the marker, then history.
	// The history here is ambiguous, so only a higher source can pass.
	root, _, _ := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{trailer("implement", "alpha/01"), trailer("follow-up", "alpha/01")}})
	first := mustGitOut(t, root, "rev-parse", "HEAD~2")
	firstBase := mustGitOut(t, root, "rev-parse", first+"^1")
	bin := buildStatusgenBinary(t)
	code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01", "--delivery", first, "--base", firstBase)
	if code != 0 || strings.Contains(stderr, "derived from history") {
		t.Fatalf("explicit flags must win over the history search: exit %d\n%s\n%s", code, stdout, stderr)
	}
	appendBrief(t, root, "\n"+goodMarker(firstBase, first))
	mustGitCommitAll(t, root, "verifier records the delivery")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	code, stdout, stderr = runDerivedCLI(t, bin, root, "--brief", "alpha/01")
	if code != 0 || strings.Contains(stderr, "derived from history") || !strings.Contains(stdout, firstBase+".."+first) {
		t.Fatalf("the marker must win over the history search: exit %d\n%s\n%s", code, stdout, stderr)
	}
}

// regression: #6099
func TestDerivedHistoryDisproved(t *testing.T) {
	// A route the derived delivery does not make true stays DISPROVED.
	for _, merge := range []bool{true, false} {
		root, _, _ := historyFixture(t, []string{"web/fixed.go: fixed-here", "web/untouched.go: fixed-here"}, historyShape{merge: merge, messages: []string{trailer("implement", "alpha/01")}})
		bin := buildStatusgenBinary(t)
		code, stdout, stderr := runDerivedCLI(t, bin, root, "--brief", "alpha/01")
		if code != 1 || !strings.Contains(stdout, "1 disproved") {
			t.Fatalf("merge=%v: a disproved route on the derived delivery: exit %d, want 1\n%s\n%s", merge, code, stdout, stderr)
		}
	}
}

// regression: #6099
func TestDerivedHistoryNotLanded(t *testing.T) {
	// A candidate the fetched mainline ref does not contain is not a delivery.
	root, _, _ := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{trailer("implement", "alpha/01")}})
	runGit(t, root, "update-ref", "-d", remoteMainRef)
	wantCouldNotCheck(t, root, "not reachable from the fetched mainline ref")
}

// regression: #6099
func TestDerivedHistoryShallow(t *testing.T) {
	// A shallow clone cannot prove there is exactly one candidate.
	src, _, _ := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{trailer("implement", "alpha/01")}})
	runGit(t, src, "branch", "-f", "landed", "HEAD")
	clone := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "--branch", "landed", "file://"+src, clone)
	cmd.Env = fixtureGitEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shallow clone: %v\n%s", err, out)
	}
	runGit(t, clone, "update-ref", remoteMainRef, "HEAD")
	runGit(t, clone, "checkout", "-q", "--detach", "HEAD")
	wantCouldNotCheck(t, clone, "complete local Git history")
}

// regression: #6099
func TestDerivedHistoryBadMarker(t *testing.T) {
	// Only a MISSING marker falls through to history. A malformed one is still
	// a refusal, even when the history alone would have named the delivery. (A
	// line wholly inside a code span is no marker at all, so it does fall through.)
	for name, c := range map[string]struct {
		marker func(base, delivery string) string
		want   string
	}{
		"trailing code span": {func(b, d string) string { return strings.TrimSpace(goodMarker(b, d)) + " `note`\n" }, "literal live text"},
		"three fields":       {func(b, d string) string { return "Verification-Table: delivery=" + d + " base=" + b + "\n" }, "expected Verification-Table"},
	} {
		t.Run(name, func(t *testing.T) {
			root, base, delivery := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{trailer("implement", "alpha/01")}})
			appendBrief(t, root, "\n"+c.marker(base, delivery))
			mustGitCommitAll(t, root, "verifier writes a malformed marker")
			runGit(t, root, "update-ref", remoteMainRef, "HEAD")
			runGit(t, root, "checkout", "-q", "--detach", "HEAD")
			wantCouldNotCheck(t, root, c.want)
		})
	}
}

// regression: #6099
func TestDerivedHistoryOctopus(t *testing.T) {
	// An octopus merge has no single side to attribute: refused, not pinned.
	root := consumersFixture(t, []string{"web/fixed.go: fixed-here"}, map[string]string{"web/fixed.go": "before\n", "web/untouched.go": "before\n", "web/excluded.go": "before\n"})
	mustGitInit(t, root)
	runGit(t, root, "commit", "--allow-empty", "-qm", "parent anchor")
	mainline := mustGitOut(t, root, "symbolic-ref", "--short", "HEAD")
	for _, side := range []string{"one", "two"} {
		runGit(t, root, "checkout", "-qb", side, mainline)
		mustWrite(t, filepath.Join(root, "web/"+side+".go"), side+"\n")
		if side == "one" {
			mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivered\n")
		}
		runGit(t, root, "add", "-A")
		runGit(t, root, "-c", "commit.gpgsign=false", "commit", "-q", "-m", trailer("implement "+side, "alpha/01"))
	}
	runGit(t, root, "checkout", "-q", mainline)
	runGit(t, root, "-c", "commit.gpgsign=false", "merge", "--no-ff", "-q", "-m", "octopus", "one", "two")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	runGit(t, root, "checkout", "-q", "--detach", "HEAD")
	wantCouldNotCheck(t, root, "not a single- or two-parent commit")
}

// regression: #6099
func TestDerivedHistoryDiffFails(t *testing.T) {
	// The second miss site: the branch diff cannot be taken at all. Both sites
	// reach the one derivation, so the history source pins this one too.
	root, base, delivery := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{merge: true, messages: []string{trailer("implement", "alpha/01")}})
	orig := changedPathsSince
	changedPathsSince = func(root, base string) ([]string, error) { return nil, os.ErrNotExist }
	defer func() { changedPathsSince = orig }()
	if got := runConsumersDerived(root, "alpha/01"); got != 0 {
		t.Fatalf("untakeable diff, delivery %s..%s in history: got %d, want 0", base, delivery, got)
	}
}

// regression: #6099
func TestDerivedHistoryNativeMessage(t *testing.T) {
	// A commit whose message the library cannot render exactly (here a declared
	// non-UTF-8 encoding) is refused, never searched natively or skipped.
	root, _, _ := historyFixture(t, []string{"web/fixed.go: fixed-here"}, historyShape{messages: []string{trailer("implement", "alpha/01")}})
	runGit(t, root, "-c", "commit.gpgsign=false", "-c", "i18n.commitEncoding=ISO-8859-1", "commit", "--allow-empty", "-qm", "an encoded commit")
	runGit(t, root, "update-ref", remoteMainRef, "HEAD")
	runGit(t, root, "checkout", "-q", "--detach", "HEAD")
	wantCouldNotCheck(t, root, "cannot be read through the library read path", "record the Verification-Table marker or pass --delivery and --base")
}
