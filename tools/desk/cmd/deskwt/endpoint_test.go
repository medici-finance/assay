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
		for _, tc := range []struct {
			scheme, port string
			refused      bool
		}{
			{"https", "", false}, {"https", ":443", false},
			{"https", ":8443", true}, {"https", ":80", true},
			{"https", ":0", true}, {"https", ":bad", true}, {"https", ":", true},
			{"http", "", false}, {"http", ":80", true}, {"http", ":443", true},
			{"http", ":8443", true}, {"http", ":0", true}, {"http", ":bad", true}, {"http", ":", true},
			{"HTTP", ":8443", true}, {"HTTPS", ":443", false},
		} {
			t.Run(mode+"/"+tc.scheme+tc.port, func(t *testing.T) {
				work := newRepo(t)
				withEnv(t, work)
				gitlabRepoRoster(t, work)
				credCalls := 0
				prevCredential := roleCredential
				roleCredential = func(role string, repo deskkit.ForgeRepo, origin string) (deskkit.RoleCredential, error) {
					credCalls++
					return prevCredential(role, repo, origin)
				}
				origin := tc.scheme + "://" + gitlabFixtureHost + tc.port + "/example-org/tracker.git"
				want := "https://" + gitlabFixtureHost + ":443/example-org/tracker.git"
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
				if tc.refused {
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
					host := gitlabFixtureHost + ":443"
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

// TestTransportAuthority covers parsing forms without invoking credential or preflight seams.
func TestTransportAuthority(t *testing.T) {
	for _, tc := range []struct {
		origin             string
		refused, networked bool
	}{
		{"hTtP://github.com:443/example/repo.git", true, true},
		{"hTtPs://github.com:443/example/repo.git", false, true},
		{"http://git@github.com:8443/example/repo.git", true, true},
		{"https://git@github.com:8443/example/repo.git", true, true},
		{"http://github.com:65536/example/repo.git", true, true},
		{"https://github.com:65536/example/repo.git", true, true},
		{"http://github.com:-1/example/repo.git", true, true},
		{"https://github.com:-1/example/repo.git", true, true},
		{"http://github.com:/example/repo.git", true, true},
		{"https://github.com:/example/repo.git", true, true},
		// These existing non-HTTP migrations have a separate transport contract.
		{"ssh://git@github.com:22/example/repo.git", false, true},
		{"git+ssh://git@github.com:22/example/repo.git", false, true},
		{"ssh+git://git@github.com:22/example/repo.git", false, true},
		{"git@github.com:example/repo.git", false, true},
		{"git://github.com:9418/example/repo.git", false, true},
		{"file:///tmp/example.git", false, false},
		{"/tmp/example.git", false, false},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			host, networked, err := transportHost(tc.origin)
			if (err != nil) != tc.refused || networked != tc.networked {
				t.Fatalf("host=%q networked=%v err=%v; want refusal=%v networked=%v", host, networked, err, tc.refused, tc.networked)
			}
			if !tc.refused && tc.networked && host != "github.com" {
				t.Fatalf("host=%q, want github.com", host)
			}
		})
	}
}
