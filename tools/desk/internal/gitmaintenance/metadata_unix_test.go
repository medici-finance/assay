//go:build unix

package gitmaintenance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// regression: #2513
func TestMetadataFailureHelper(t *testing.T) {
	repo := os.Getenv("ASSAY_FIXTURE_METADATA_REPO")
	if repo == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Run(ctx, repo, Options{DryRun: os.Getenv("ASSAY_FIXTURE_METADATA_DRY") == "1"}); err == nil {
		t.Fatal("uncertain metadata accepted")
	}
}

// regression: #2513
func TestMetadataPathsBounded(t *testing.T) {
	for _, kind := range []string{"alternates-fifo", "alternates-large", "head-fifo", "commondir-fifo", "gitdir-fifo", "head-large"} {
		t.Run(kind, func(t *testing.T) {
			repo := fixtureRepo(t)
			requireNativeHooks(t, repo)
			path := filepath.Join(repo, ".git", "objects", "info", "alternates")
			if !strings.HasPrefix(kind, "alternates-") {
				linked := filepath.Join(t.TempDir(), "linked")
				fixtureGit(t, repo, "worktree", "add", "--detach", linked)
				admin := fixtureGit(t, linked, "rev-parse", "--absolute-git-dir")
				field, _, _ := strings.Cut(kind, "-")
				if field == "head" {
					field = "HEAD"
				}
				path = filepath.Join(admin, field)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasSuffix(kind, "fifo") {
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(path, []byte(strings.Repeat("x", (1<<20)+1)), 0600); err != nil {
					t.Fatal(err)
				}
			}
			// Apply may open both known coordinator locks before rejecting metadata.
			for _, name := range []string{"assay-maintenance.lock", "assay-maintenance-temprefs.lock"} {
				if err := os.WriteFile(filepath.Join(repo, ".git", name), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := diskImage(t, repo)
			t.Setenv("ASSAY_FIXTURE_METADATA_REPO", repo)
			for _, dry := range []string{"0", "1"} {
				t.Setenv("ASSAY_FIXTURE_METADATA_DRY", dry)
				// Allow subprocess/race-runtime startup outside the helper's own 1s task budget.
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMetadataFailureHelper$")
				out, err := cmd.CombinedOutput()
				cancel()
				if ctx.Err() == context.DeadlineExceeded {
					t.Fatal("metadata read escaped the runtime bound")
				}
				if err != nil {
					t.Fatalf("bounded metadata helper failed: %v\n%s", err, out)
				}
			}
			if !reflect.DeepEqual(before, diskImage(t, repo)) {
				t.Fatalf("incomplete metadata led to mutation: %v", snapshotChanges(before, diskImage(t, repo)))
			}
		})
	}
}
