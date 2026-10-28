package gitmaintenance

import (
	"context"
	"path/filepath"
	"testing"
)

// regression: #2513
func TestOfflineTasksPinned(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	remote := filepath.Join(t.TempDir(), "remote.git")
	fixtureGit(t, repo, "clone", "--bare", repo, remote)
	fixtureGit(t, repo, "remote", "add", "origin", remote)
	for key, value := range map[string]string{
		"maintenance.gc.enabled": "true", "maintenance.prefetch.enabled": "true",
		"maintenance.reflog-expire.enabled": "true", "maintenance.worktree-prune.enabled": "true",
		"gc.pruneExpire": "now", "gc.reflogExpire": "now", "gc.reflogExpireUnreachable": "now",
		"maintenance.loose-objects.batchSize": "0", "maintenance.incremental-repack.batchSize": "0",
	} {
		fixtureGit(t, repo, "config", key, value)
	}
	previous := fixtureGit(t, repo, "rev-parse", "HEAD")
	fixtureCommit(t, repo, "retained reflog commit")
	fixtureGit(t, repo, "reset", "--hard", previous)
	reflog := fixtureGit(t, repo, "reflog", "show", "--format=%H %gs", "HEAD")
	if _, err := Run(context.Background(), repo, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := fixtureGit(t, repo, "for-each-ref", "refs/prefetch/"); got != "" {
		t.Fatal("offline maintenance prefetched a configured remote")
	}
	if got := fixtureGit(t, repo, "reflog", "show", "--format=%H %gs", "HEAD"); got != reflog {
		t.Fatal("maintenance expired reflog recovery")
	}
}
