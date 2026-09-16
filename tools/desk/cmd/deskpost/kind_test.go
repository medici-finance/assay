package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// `deskpost comment --kind issue|mr` (#1091, the sibling of assay#1087's
// `deskfile attach --kind`). GitLab numbers issues and merge requests in SEPARATE
// sequences, so #N and !N routinely both exist; resolveTarget's automatic PR-first
// ordering silently lands on whichever one resolves first, with no error to notice the
// other was meant instead. --kind forces resolveTargetKind (target.go) for the READ and
// PostCommentTyped for the WRITE, bypassing that ambiguity entirely.
//
// The fake server here is GitHub-shaped (one number sequence), so these tests exercise
// the FLAG PLUMBING and the kind-mismatch refusal shape (ghClient.getIssueTyped mirrors
// GitHubForge.GetIssueTyped's own validation) — the GitLab both-kinds-exist case itself is
// already golden-pinned forge-side in forge_gitlab_typed_test.go (PR#1094).

func commentArgsKind(repo, pr, bodyFile, kind string) []string {
	return append(commentArgs(repo, pr, bodyFile), "--kind", kind)
}

// TestCommentKindMRUsesOnlyPRRead: --kind mr resolves via getPR alone — the issues
// endpoint is never touched, proving the automatic (ambiguous) resolution was bypassed
// entirely rather than merely reordered.
func TestCommentKindMRUsesOnlyPRRead(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "targeted at the change explicitly")
	code := run(commentArgsKind(exampleRepo, "1", bf, "mr"))
	if code != 0 {
		t.Fatalf("comment --kind mr exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
	if n := f.hitCount("GET", "/issues/1"); n != 0 {
		t.Fatalf("--kind mr made %d issues-endpoint reads, want 0 (forced-kind must skip it)", n)
	}
	e := lastAudit(t)
	if e.Verb != "comment:"+deskkit.Sha256Hex([]byte("targeted at the change explicitly")) {
		t.Fatalf("--kind mr must key the PR verb/idempotency exactly as auto-resolution does, got %q", e.Verb)
	}
}

// TestCommentKindIssueUsesTypedRead: --kind issue on a real issue number resolves via
// getIssueTyped and posts under the issue verb, same shape as automatic resolution.
func TestCommentKindIssueUsesTypedRead(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true
	bf := writeBody(t, "i.md", "targeted at the issue explicitly")
	code := run(commentArgsKind("example-org/org-slides", "21", bf, "issue"))
	if code != 0 {
		t.Fatalf("comment --kind issue exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
	e := lastAudit(t)
	wantVerb := "comment:issue:" + deskkit.Sha256Hex([]byte("targeted at the issue explicitly"))
	if e.Verb != wantVerb {
		t.Fatalf("--kind issue verb = %q, want %q", e.Verb, wantVerb)
	}
}

// TestCommentKindMRMismatchRefuses: --kind mr on a number that is actually an ISSUE (not
// a PR) must not silently fall through to the issue path — it is a clean could-not-check
// naming the mismatch, and nothing is posted. This is the guard-rail half: forcing a kind
// must fail closed on a wrong guess, never quietly resolve the OTHER kind instead.
func TestCommentKindMRMismatchRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.issueNums[21] = true // 21 is an issue: GET /pulls/21 404s
	bf := writeBody(t, "i.md", "would be wrong to post as a PR comment")
	code := run(commentArgsKind("example-org/org-slides", "21", bf, "mr"))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("--kind mr on an issue number exit = %d, want 6", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment may be posted when the forced kind does not match the number")
	}
	d := lastAudit(t).Detail
	if !strings.Contains(d, "--kind mr") || !strings.Contains(d, "not a merge request") {
		t.Fatalf("refusal must name the mismatch: %q", d)
	}
}

// TestCommentKindIssueMismatchRefuses: the inverse — --kind issue on a number that is
// actually a pull request is a could-not-check, never a silent PR comment.
func TestCommentKindIssueMismatchRefuses(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "would be wrong to post as an issue comment")
	code := run(commentArgsKind(exampleRepo, "1", bf, "issue"))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("--kind issue on a PR number exit = %d, want 6", code)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment may be posted when the forced kind does not match the number")
	}
	d := lastAudit(t).Detail
	if !strings.Contains(d, "pull request") {
		t.Fatalf("refusal must name the mismatch: %q", d)
	}
}

// TestCommentKindUnknownRejectedExit2NoNetwork: an unparseable --kind value is a plain
// usage error (exit 2) with zero forge reads — the same class as a malformed --head — and
// must not silently fall back to automatic resolution.
func TestCommentKindUnknownRejectedExit2NoNetwork(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "x")
	code := run(commentArgsKind(exampleRepo, "1", bf, "merge-request"))
	if code != 2 {
		t.Fatalf("--kind merge-request exit = %d, want 2", code)
	}
	if len(f.hits) != 0 {
		t.Fatalf("an unparseable --kind must reach zero network calls, got %v", f.hits)
	}
	if f.postedCmt != 0 {
		t.Fatal("no comment may be posted for an unparseable --kind")
	}
}

// TestCommentKindEmptyUnchanged: omitting --kind is byte-identical to the pre-existing
// behaviour — a regression guard that the flag's presence in the flag set does not, by
// itself, change resolution when unused.
func TestCommentKindEmptyUnchanged(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "no --kind given")
	code := run(commentArgs(exampleRepo, "1", bf))
	if code != 0 {
		t.Fatalf("comment with no --kind exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
}

// TestCommentKindPRAliasAcceptsMR: "pr" is accepted as an alias of "mr" (ParseTargetKind,
// forge.go) — the same alias deskfile attach's --kind accepts.
func TestCommentKindPRAliasAcceptsMR(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "c.md", "alias check")
	code := run(commentArgsKind(exampleRepo, "1", bf, "pr"))
	if code != 0 {
		t.Fatalf("comment --kind pr exit = %d, want 0", code)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
}
