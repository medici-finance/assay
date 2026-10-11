//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestMaintenanceSpecialDirectoryHelper(t *testing.T) {
	work := os.Getenv("ASSAY_MAINTENANCE_SPECIAL_DIRECTORY_REPO")
	if work == "" {
		return
	}
	withEnv(t, work)
	args := []string{"--repo", work, "--timeout", "1s"}
	if os.Getenv("ASSAY_MAINTENANCE_SPECIAL_DIRECTORY_APPLY") == "1" {
		args = append(args, "--apply")
	}
	if err := cmdMaintain(args); err == nil {
		t.Fatal("special directory was accepted")
	}
}

func TestMaintenanceSpecialDirectoryBounded(t *testing.T) {
	for _, kind := range []string{"direct", "symlink"} {
		for _, mode := range []string{"plan", "apply"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				work, _ := maintenanceFixture(t)
				path := filepath.Join(t.TempDir(), "special")
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
				if kind == "symlink" {
					link := filepath.Join(t.TempDir(), "linked")
					if err := os.Symlink(path, link); err != nil {
						t.Fatal(err)
					}
					path = link
				}
				mustGit(t, work, "config", "core.hooksPath", path)
				before := mustGit(t, work, "show-ref")
				t.Setenv("ASSAY_MAINTENANCE_SPECIAL_DIRECTORY_REPO", work)
				if mode == "apply" {
					t.Setenv("ASSAY_MAINTENANCE_SPECIAL_DIRECTORY_APPLY", "1")
				} else {
					t.Setenv("ASSAY_MAINTENANCE_SPECIAL_DIRECTORY_APPLY", "")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMaintenanceSpecialDirectoryHelper$")
				out, err := cmd.Output()
				if ctx.Err() != nil {
					t.Fatal("preflight exceeded its work bound")
				}
				if err != nil {
					t.Fatalf("helper failed: %v output=%s", err, out)
				}
				var report maintenanceReport
				if err := json.NewDecoder(bytes.NewReader(out)).Decode(&report); err != nil {
					t.Fatalf("report: %v: %s", err, out)
				}
				if report.Status != "could-not-check" {
					t.Fatalf("bad status: %+v", report)
				}
				if _, err := os.Stat(filepath.Join(work, ".git", "assay-maintenance.lock")); !os.IsNotExist(err) {
					t.Fatal("preflight wrote attempt stamp")
				}
				if _, err := os.Stat(filepath.Join(work, ".git", "packed-refs")); !os.IsNotExist(err) {
					t.Fatal("preflight packed refs")
				}
				if _, err := os.Stat(filepath.Join(work, ".git", "objects", "info", "commit-graphs")); !os.IsNotExist(err) {
					t.Fatal("preflight wrote commit graph")
				}
				if mustGit(t, work, "show-ref") != before {
					t.Fatal("preflight changed refs")
				}
			})
		}
	}
}
