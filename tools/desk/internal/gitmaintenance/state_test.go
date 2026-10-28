package gitmaintenance

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// regression: #2513
func TestStampFailuresStop(t *testing.T) {
	for _, body := range []string{
		"not JSON", `{}`, `{"version":1}`, `{"version":2,"success_at":"2000-01-01T00:00:00Z"}`,
		`{"version":1,"success_at":"2999-01-01T00:00:00Z"}`,
		`{"version":1,"success_at":"2000-01-01T00:00:00Z","unknown":true}`,
		`{"version":1,"success_at":"2000-01-01T00:00:00Z"} {}`,
	} {
		t.Run(body, func(t *testing.T) {
			repo := fixtureRepo(t)
			requireNativeHooks(t, repo)
			inv, err := Inspect(context.Background(), repo)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(inv.CommonDir, "assay-maintenance.json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			before := diskImage(t, repo)
			if _, err := Run(context.Background(), repo, Options{}); err == nil {
				t.Fatal("invalid success record accepted")
			}
			if !reflect.DeepEqual(before, diskImage(t, repo)) {
				t.Fatal("invalid stamp led to repository writes")
			}
		})
	}
}

// regression: #2513
func TestUnreadableStateStops(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	inv, err := Inspect(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inv.CommonDir, "assay-maintenance.json")
	// A directory is reliably unreadable as a stamp even on privileged test runners.
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	before := diskImage(t, repo)
	if _, err := Run(context.Background(), repo, Options{}); err == nil {
		t.Fatal("unreadable stamp became a success")
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatal("unreadable state led to repository writes")
	}
}

// regression: #2513
func TestCanceledRunDoesNotStamp(t *testing.T) {
	repo := fixtureRepo(t)
	before := diskImage(t, repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, repo, Options{Timeout: time.Second}); err == nil {
		t.Fatal("canceled work reported success")
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatal("canceled work changed bookkeeping")
	}
}

// regression: #2513
func TestAmbientGitVarsIgnored(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "unrelated"))
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(t.TempDir(), "wrong-objects"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "extensions.partialclone")
	t.Setenv("GIT_CONFIG_VALUE_0", "untrusted")
	if _, err := Run(context.Background(), repo, Options{}); err != nil {
		t.Fatalf("ambient git state escaped into coordinator: %v", err)
	}
}

// regression: #2513
func TestConfigIncludesRefused(t *testing.T) {
	repo := fixtureRepo(t)
	external := filepath.Join(t.TempDir(), "included")
	if err := os.WriteFile(external, []byte("[maintenance]\n prefetch = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, repo, "config", "include.path", external)
	// Both cooperative locks precede inventory; refusal may open these existing files.
	for _, name := range []string{"assay-maintenance.lock", "assay-maintenance-temprefs.lock"} {
		if err := os.WriteFile(filepath.Join(repo, ".git", name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := diskImage(t, repo)
	if _, err := Run(context.Background(), repo, Options{}); err == nil {
		t.Fatal("included config accepted into maintenance")
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatalf("refused include still changed repository: %v", snapshotChanges(before, diskImage(t, repo)))
	}
}

// regression: #2513
func TestMissingAlternateStops(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	inv, err := Inspect(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	alt := filepath.Join(inv.CommonDir, "objects", "info", "alternates")
	if err := os.WriteFile(alt, []byte(filepath.Join(t.TempDir(), "missing-objects")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Lock-before-inventory is intentional; refusal may open this coordinator file.
	if err := os.WriteFile(filepath.Join(repo, ".git", "assay-maintenance.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	before := diskImage(t, repo)
	if _, err := Run(context.Background(), repo, Options{}); err == nil {
		t.Fatal("unreadable alternate store was treated as complete")
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatal("unreadable alternate led to maintenance writes")
	}
}

// regression: #2513
func TestActiveMarkerHeld(t *testing.T) {
	repo := fixtureRepo(t)
	marker := filepath.Join(repo, ".git", "assay-maintenance-active.json")
	// Even an ancient marker is custody, never an age-based reclamation grant.
	if err := os.WriteFile(marker, []byte(`{"version":1,"started_at":"2000-01-01T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "assay-maintenance.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	before := diskImage(t, repo)
	if _, err := Inspect(context.Background(), repo); err == nil {
		t.Fatal("outstanding process custody was reported clean")
	}
	for _, dry := range []bool{true, false} {
		if _, err := Run(context.Background(), repo, Options{DryRun: dry}); err == nil {
			t.Fatal("outstanding process custody permitted a new run")
		}
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatal("outstanding custody was cleared or mutated")
	}
}

// regression: #2513
func TestAttemptFailuresStop(t *testing.T) {
	for _, body := range []string{"not JSON", `{}`, `{"version":1}`, `{"version":2,"attempted_at":"2000-01-01T00:00:00Z"}`, `{"version":1,"attempted_at":"2999-01-01T00:00:00Z"}`, `{"version":1,"attempted_at":"2000-01-01T00:00:00Z","unknown":true}`, `{"version":1,"attempted_at":"2000-01-01T00:00:00Z"} {}`} {
		t.Run(body, func(t *testing.T) {
			repo := fixtureRepo(t)
			requireNativeHooks(t, repo)
			if err := os.WriteFile(filepath.Join(repo, ".git", "assay-maintenance-attempt.json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			before := diskImage(t, repo)
			for _, dry := range []bool{true, false} {
				if _, err := Run(context.Background(), repo, Options{DryRun: dry}); err == nil {
					t.Fatal("invalid attempt record accepted")
				}
			}
			if !reflect.DeepEqual(before, diskImage(t, repo)) {
				t.Fatal("invalid attempt record permitted bookkeeping or tasks")
			}
		})
	}
}
