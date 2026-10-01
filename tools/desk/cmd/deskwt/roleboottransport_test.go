package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Exercise the boot entry point with real Git, including its reused-worktree arm.
// A stubbed preflight still observes the actual URLs at the instant it is called.
func TestRoleInitAppTransport(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reuse=%v", reuse), func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			sshOperatorCheckout(t, work, sharedSSHOrigin)
			mustGit(t, work, "config", "extensions.worktreeConfig", "true")
			mustGit(t, work, "branch", "--track", "verify-desk/transport", "origin/main")
			shared, err := os.ReadFile(filepath.Join(work, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(tmpBaseDir, "tracker-verify-desk-transport")
			if reuse {
				mustGit(t, work, "worktree", "add", target, "verify-desk/transport")
				writeFile(t, filepath.Join(target, "keep.txt"), "existing work\n")
			}
			pfCalls := 0
			roleInitPreflight = func(req deskkit.PreflightRequest) error {
				pfCalls++
				if req.Root != target || req.Role != "verifier" {
					return fmt.Errorf("wrong preflight target: %+v", req)
				}
				for _, push := range []bool{false, true} {
					got, err := originURLs(req.Root, push)
					if err != nil || len(got) != 1 || got[0] != wantAppURL {
						return fmt.Errorf("preflight push=%v URLs=%v error=%v; want [%s]", push, got, err, wantAppURL)
					}
				}
				return nil
			}
			rc, stderr := runCapErr(t, []string{"role-init", "verifier", "--session", "transport", "--no-fetch"})
			if rc != deskkit.ExitOK {
				t.Fatalf("role-init rc=%d: %s", rc, stderr)
			}
			if pfCalls != 1 {
				t.Fatalf("preflight calls=%d, want 1", pfCalls)
			}
			for _, args := range [][]string{{"fetch", "origin"}, {"push", "--dry-run", "origin", "HEAD:refs/heads/probe"}} {
				if got := offlineTransport(t, target, args...); !strings.Contains(got, "transport 'https' not allowed") {
					t.Fatalf("%v did not select HTTPS: %s", args, got)
				}
			}
			if got := credentialFill(t, target, "https", "github.com:443"); !strings.Contains(got, "password="+fixtureTokenValue) {
				t.Fatalf("App transport has no role credential: %s", got)
			}
			for _, tc := range [][2]string{{"https", "example.invalid"}, {"http", "github.com"}} {
				if got := credentialFill(t, target, tc[0], tc[1]); strings.Contains(got, fixtureTokenValue) {
					t.Fatalf("credential escaped origin HTTPS: %v", tc)
				}
			}
			after, err := os.ReadFile(filepath.Join(work, ".git", "config"))
			if err != nil || !bytes.Equal(shared, after) {
				t.Fatalf("shared configuration changed: %v", err)
			}
			if got := gitLines(t, work, "remote", "get-url", "--push", "--all", "origin"); len(got) != 1 || got[0] != operatorSentinel {
				t.Fatalf("parent sentinel changed: %v", got)
			}
			if reuse {
				got, err := os.ReadFile(filepath.Join(target, "keep.txt"))
				if err != nil || string(got) != "existing work\n" {
					t.Fatalf("reuse changed existing work: %q %v", got, err)
				}
			}
		})
	}
}

func TestRoleInitBadTransport(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reuse=%v", reuse), func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			sshOperatorCheckout(t, work, sharedSSHOrigin)
			writeFile(t, filepath.Join(os.Getenv("HOME"), ".gitconfig"), "[url \"ssh://git@github.com/\"]\n insteadOf = https://github.com:443/\n")
			target := filepath.Join(tmpBaseDir, "tracker-verify-desk-badtransport")
			if reuse {
				mustGit(t, work, "worktree", "add", "--track", "-b", "verify-desk/badtransport", target, "origin/main")
				writeFile(t, filepath.Join(target, "keep.txt"), "existing work\n")
			}
			pfRan := false
			roleInitPreflight = func(deskkit.PreflightRequest) error { pfRan = true; return nil }
			rc, stderr := runCapErr(t, []string{"role-init", "verifier", "--session", "badtransport", "--no-fetch"})
			if rc != deskkit.ExitRefused || !strings.Contains(stderr, "git still resolves") {
				t.Fatalf("rc=%d, want transport refusal: %s", rc, stderr)
			}
			if pfRan {
				t.Fatal("preflight ran after transport provisioning failed")
			}
			if strings.Contains(stderr, "ROLLED BACK") {
				t.Fatal("role-init falsely claimed a rollback")
			}
			if reuse {
				if got, err := os.ReadFile(filepath.Join(target, "keep.txt")); err != nil || string(got) != "existing work\n" {
					t.Fatalf("refusal destroyed existing work: %q %v", got, err)
				}
			}
		})
	}
}
