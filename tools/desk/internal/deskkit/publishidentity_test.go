package deskkit

// Property tests for the publish-identity gate (issue #1490 lane B).
//
// Each test pins a property whose failure mode is SILENT in exactly the way the class is:
// a commit authored under a stale worktree identity publishes under the wrong actor, the
// push succeeds, and nothing red appears at the moment it happens. The gate is what turns
// that silent misattribution into an exit-5 refusal at the push boundary.
//
// FAIL-FIRST. Before this gate existed there was no check at all: every case below that
// ends in a refusal would have let the push proceed (the deskpr/deskevidence wiring tests
// prove the push is what the gate stands in front of). The single-line mutation that
// re-opens the hole is `emailMatchesRole` returning true unconditionally — do that and
// TestPublishIdentityForeignAuthorRefused / ForeignCommitterRefused / ForgeMergeExempt's
// non-merge arm all go green-would-push, which is the pre-gate state.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture roster (installed by this package's TestMain) binds
// worker=github:assay-worker-app:300000006 and verifier=github:assay-verifier-app:300000005.
const (
	workerEmail   = "300000006+assay-worker-app[bot]@users.noreply.github.com"
	verifierEmail = "300000005+assay-verifier-app[bot]@users.noreply.github.com"
	// issueLoopEmail is another role's bot — the exact cross-role misattribution #1490
	// observed (a verifier worktree carrying the issue-loop bot's identity).
	issueLoopEmail = "300000003+assay-issue-loop-app[bot]@users.noreply.github.com"
)

// seam returns a Commits reader that yields the given commits verbatim, so the identity
// logic is exercised without building a repository per case.
func seam(commits ...PublishCommit) func(string, string) ([]PublishCommit, error) {
	return func(string, string) ([]PublishCommit, error) { return commits, nil }
}

func workerCommit(sha, subject, authorEmail, committerEmail string) PublishCommit {
	return PublishCommit{
		SHA: sha, Subject: subject,
		AuthorName: "Assay Worker", AuthorEmail: authorEmail,
		CommitterName: "Assay Worker", CommitterEmail: committerEmail,
	}
}

func TestPublishIdentityForeignAuthorRefused(t *testing.T) {
	err := PublishIdentityMatchesRole(PublishIdentityInput{
		Role: "worker",
		Commits: seam(
			workerCommit("aaaaaaaaaaaa1111", "good one", workerEmail, workerEmail),
			workerCommit("bbbbbbbbbbbb2222", "authored elsewhere", issueLoopEmail, workerEmail),
		),
	})
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("foreign author: exit = %d, want 5 (refused); err = %v", ExitCodeOf(err), err)
	}
	msg := err.Error()
	// The refusal must NAME the offending commit and the found/expected identities.
	for _, want := range []string{"bbbbbbbbbbbb", "authored elsewhere", "assay-issue-loop-app", "assay-worker-app[bot]"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal does not name %q: %s", want, msg)
		}
	}
}

func TestPublishIdentityForeignCommitterRefused(t *testing.T) {
	// Author correct, committer foreign — both halves are checked, not just the author.
	err := PublishIdentityMatchesRole(PublishIdentityInput{
		Role:    "worker",
		Commits: seam(workerCommit("cccccccccccc3333", "committed elsewhere", workerEmail, issueLoopEmail)),
	})
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("foreign committer: exit = %d, want 5 (refused); err = %v", ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "committed by") {
		t.Errorf("committer refusal should say it was 'committed by' the wrong identity: %s", err.Error())
	}
}

func TestPublishIdentityCorrectPasses(t *testing.T) {
	if err := PublishIdentityMatchesRole(PublishIdentityInput{
		Role: "worker",
		Commits: seam(
			workerCommit("dddddddddddd4444", "one", workerEmail, workerEmail),
			workerCommit("eeeeeeeeeeee5555", "two", workerEmail, workerEmail),
		),
	}); err != nil {
		t.Fatalf("correctly-attributed commits must pass, got: %v", err)
	}
}

func TestPublishIdentityEmptyRangeClean(t *testing.T) {
	if err := PublishIdentityMatchesRole(PublishIdentityInput{Role: "worker", Commits: seam()}); err != nil {
		t.Fatalf("an empty range publishes nothing and must be clean, got: %v", err)
	}
}

// TestPublishIdentityForgeMergeExempt — a forge-authored merge commit (>=2 parents,
// authored/committed as noreply@github.com) is EXEMPT, but an ORDINARY commit carrying the
// same forge address is NOT — the exemption is scoped to real merges so it cannot be used
// to smuggle a mis-attributed ordinary commit through.
func TestPublishIdentityForgeMergeExempt(t *testing.T) {
	const forgeEmail = "noreply@github.com"

	t.Run("merge_by_forge_is_exempt", func(t *testing.T) {
		merge := PublishCommit{
			SHA: "ffffffffffff6666", Subject: "Merge branch 'main' into feat",
			Parents:    []string{"p1", "p2"},
			AuthorName: "GitHub", AuthorEmail: forgeEmail,
			CommitterName: "GitHub", CommitterEmail: forgeEmail,
		}
		if err := PublishIdentityMatchesRole(PublishIdentityInput{
			Role:    "worker",
			Commits: seam(workerCommit("1111111111117777", "real work", workerEmail, workerEmail), merge),
		}); err != nil {
			t.Fatalf("a forge-authored merge must be exempt, got: %v", err)
		}
	})

	t.Run("non_merge_with_forge_address_is_still_checked", func(t *testing.T) {
		// Same forge address, but ONE parent — an ordinary commit, not a merge. It must NOT
		// be exempted, or the exemption becomes a bypass.
		ordinary := PublishCommit{
			SHA: "2222222222228888", Subject: "sneaky", Parents: []string{"p1"},
			AuthorName: "GitHub", AuthorEmail: forgeEmail,
			CommitterName: "GitHub", CommitterEmail: forgeEmail,
		}
		err := PublishIdentityMatchesRole(PublishIdentityInput{Role: "worker", Commits: seam(ordinary)})
		if ExitCodeOf(err) != ExitRefused {
			t.Fatalf("a NON-merge commit carrying the forge address must be refused, got exit %d (%v)", ExitCodeOf(err), err)
		}
	})
}

func TestPublishIdentityUnboundRoleRefused(t *testing.T) {
	// A role the roster binds no identity to cannot attribute any commit — refuse rather
	// than let an unbindable role publish anything.
	err := PublishIdentityMatchesRole(PublishIdentityInput{
		Role:    "nonesuch",
		Commits: seam(workerCommit("3333333333339999", "x", workerEmail, workerEmail)),
	})
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("unbound role: exit = %d, want 5 (refused); err = %v", ExitCodeOf(err), err)
	}
}

func TestPublishIdentityUnpinnedGitHubIDUnverifiable(t *testing.T) {
	// A GitHub role with no pinned bot USER id yields no address to compare against —
	// could-not-check (exit 6), never rounded up to a pass.
	withRoster(t, map[string]string{
		EnvBlessLogin:      "ada:2001",
		EnvTrustedBotSlugs: "worker=github:assay-worker-app", // no :id
	})
	err := PublishIdentityMatchesRole(PublishIdentityInput{
		Role:    "worker",
		Commits: seam(workerCommit("4444444444440000", "x", workerEmail, workerEmail)),
	})
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("unpinned bot USER id: exit = %d, want 6 (could-not-check); err = %v", ExitCodeOf(err), err)
	}
}

// TestPublishIdentityGitLab — the GitLab two-identity path (#643): a trusted session
// address is accepted, the service-account noreply SHAPE is accepted, and a GitHub-shaped
// address for a GitLab role is refused (the cross-forge case).
func TestPublishIdentityGitLab(t *testing.T) {
	const sessionEmail = "ih-bot@example.org"
	const saEmail = "service_account_group_9619193_abcdef@noreply.gitlab.example.org"
	withRoster(t, map[string]string{
		EnvBlessLogin:          "ada:2001",
		EnvTrustedBotSlugs:     "verifier=gitlab:assay-verifier-bot:41987965",
		EnvGitLabSessionEmails: sessionEmail,
	})

	t.Run("trusted_session_address_accepted", func(t *testing.T) {
		if err := PublishIdentityMatchesRole(PublishIdentityInput{
			Role:    "verifier",
			Commits: seam(workerCommit("5555555555551111", "gl work", sessionEmail, sessionEmail)),
		}); err != nil {
			t.Fatalf("a trusted GitLab session address must be accepted, got: %v", err)
		}
	})

	t.Run("service_account_shape_accepted", func(t *testing.T) {
		if err := PublishIdentityMatchesRole(PublishIdentityInput{
			Role:    "verifier",
			Commits: seam(workerCommit("6666666666662222", "gl sa work", saEmail, saEmail)),
		}); err != nil {
			t.Fatalf("the GitLab service-account noreply shape must be accepted, got: %v", err)
		}
	})

	t.Run("github_address_for_gitlab_role_refused", func(t *testing.T) {
		err := PublishIdentityMatchesRole(PublishIdentityInput{
			Role:    "verifier",
			Commits: seam(workerCommit("7777777777773333", "wrong forge", verifierEmail, verifierEmail)),
		})
		if ExitCodeOf(err) != ExitRefused {
			t.Fatalf("a GitHub-shaped address for a GitLab role must be refused, got exit %d (%v)", ExitCodeOf(err), err)
		}
	})
}

// --- the default git reader ---------------------------------------------------------------

// TestDefaultPublishCommitsReadsRange builds a real repository with an origin/main remote
// ref and a feature commit ahead of it, then proves defaultPublishCommits returns exactly
// that commit with its author/committer — and that PublishIdentityMatchesRole run over the
// real range refuses when the feature commit is authored by the wrong bot.
func TestDefaultPublishCommitsReadsRange(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-b", "main")
	git("config", "commit.gpgsign", "false")
	// Seed main under a neutral identity, record it as origin/main.
	git("config", "user.email", "seed@example.org")
	git("config", "user.name", "Seed")
	writeFile(t, filepath.Join(dir, "README.md"), "seed\n")
	git("add", "README.md")
	git("commit", "-m", "init")
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	// One feature commit authored as ANOTHER role's bot — the #1490 misattribution.
	git("config", "user.email", issueLoopEmail)
	git("config", "user.name", "Assay Issue Loop")
	git("checkout", "-b", "feat")
	writeFile(t, filepath.Join(dir, "feature.txt"), "work\n")
	git("add", "feature.txt")
	git("commit", "-m", "feature work")

	commits, err := defaultPublishCommits(dir, "main")
	if err != nil {
		t.Fatalf("defaultPublishCommits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("range origin/main..HEAD returned %d commits, want 1: %+v", len(commits), commits)
	}
	if commits[0].AuthorEmail != issueLoopEmail || commits[0].Subject != "feature work" {
		t.Fatalf("read commit = %+v, want the feature commit authored as the issue-loop bot", commits[0])
	}

	// The whole gate over the real range: worker role, issue-loop-authored commit → refused.
	err = PublishIdentityMatchesRole(PublishIdentityInput{Dir: dir, Base: "main", Role: "worker"})
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("gate over the real range: exit = %d, want 5 (refused); err = %v", ExitCodeOf(err), err)
	}

	// An unresolvable base is could-not-check, never clean.
	err = PublishIdentityMatchesRole(PublishIdentityInput{Dir: dir, Base: "does-not-exist", Role: "worker"})
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("unresolvable base: exit = %d, want 6 (could-not-check); err = %v", ExitCodeOf(err), err)
	}
}
