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

func TestRoleEndpointPreserved(t *testing.T) {
	for _, mode := range []string{"fresh", "reuse", "add"} {
		for _, port := range []string{"", ":443", ":8443", ":0", ":bad"} {
			t.Run(mode+port, func(t *testing.T) {
				work := newRepo(t)
				withEnv(t, work)
				gitlabRepoRoster(t, work)
				credCalls := 0
				prevCredential := roleCredential
				roleCredential = func(role string, repo deskkit.ForgeRepo, origin string) (deskkit.RoleCredential, error) {
					credCalls++
					return prevCredential(role, repo, origin)
				}
				origin := "https://" + gitlabFixtureHost + port + "/example-org/tracker.git"
				want := origin
				if port == "" {
					want = "https://" + gitlabFixtureHost + ":443/example-org/tracker.git"
				}
				mustGit(t, work, "remote", "set-url", "origin", origin)
				mustGit(t, work, "config", "remote.origin.pushurl", operatorSentinel)
				mustGit(t, work, "config", "extensions.worktreeConfig", "true")
				mustGit(t, work, "branch", "--track", "verify-desk/endpoint", "origin/main")
				shared, err := os.ReadFile(filepath.Join(work, ".git", "config"))
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(tmpBaseDir, "tracker-verify-desk-endpoint")
				if mode == "reuse" {
					mustGit(t, work, "worktree", "add", target, "verify-desk/endpoint")
					writeFile(t, filepath.Join(target, "keep.txt"), "keep\n")
				}
				checkURLs := func() error {
					for _, push := range []bool{false, true} {
						got, err := originURLs(target, push)
						if err != nil || len(got) != 1 || got[0] != want {
							return fmt.Errorf("push=%v endpoint=%v, want %s; error=%v", push, got, want, err)
						}
					}
					return nil
				}
				pfCalls := 0
				roleInitPreflight = func(deskkit.PreflightRequest) error { pfCalls++; return checkURLs() }
				args := []string{"role-init", "verifier", "--session", "endpoint", "--no-fetch"}
				if mode == "add" {
					args = []string{"add", "endpoint", "--role", "verifier", "--detach"}
					target = filepath.Join(tmpBaseDir, "tracker-endpoint")
				}
				rc, stderr := runCapErr(t, args)
				unsupported := port != "" && port != ":443"
				if unsupported {
					if rc != deskkit.ExitRefused {
						t.Fatalf("rc=%d, want unsupported endpoint refusal: %s", rc, stderr)
					}
					if pfCalls != 0 || credCalls != 0 {
						t.Fatalf("refusal reached preflight=%d credential resolver=%d", pfCalls, credCalls)
					}
					if mode == "add" {
						if _, err := os.Stat(target); !os.IsNotExist(err) {
							t.Fatalf("add failed to roll back: %v", err)
						}
					} else {
						if got := gitLines(t, target, "config", "--get", "remote.origin.url"); len(got) != 1 || got[0] != origin {
							t.Fatalf("original URL changed: %v", got)
						}
						if got := gitLines(t, target, "remote", "get-url", "--push", "--all", "origin"); len(got) != 1 || got[0] != operatorSentinel {
							t.Fatalf("original push destination changed: %v", got)
						}
					}
				} else {
					if rc != deskkit.ExitOK {
						t.Fatalf("rc=%d: %s", rc, stderr)
					}
					if err := checkURLs(); err != nil {
						t.Fatal(err)
					}
					if mode != "add" && pfCalls != 1 {
						t.Fatalf("preflight calls=%d, want1", pfCalls)
					}
					host := gitlabFixtureHost + port
					if got := credentialFill(t, target, "https", host); !strings.Contains(got, "username=oauth2") || !strings.Contains(got, "password="+fixtureTokenValue) {
						t.Fatalf("expected role credential at original HTTPS endpoint: %s", got)
					}
				}
				after, err := os.ReadFile(filepath.Join(work, ".git", "config"))
				if err != nil || !bytes.Equal(shared, after) {
					t.Fatalf("parent config changed: %v", err)
				}
				if mode == "reuse" {
					if got, err := os.ReadFile(filepath.Join(target, "keep.txt")); err != nil || string(got) != "keep\n" {
						t.Fatalf("existing file changed: %q %v", got, err)
					}
				}
			})
		}
	}
}
