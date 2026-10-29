package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitcore/gittest"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #3327
func TestHeadsRoleTransport(t *testing.T) {
	work := newRepo(t, allowedSlug)
	withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk")
	raw := mustGit(t, work, "remote", "get-url", "origin")
	mustGit(t, raw, "update-ref", "refs/heads/feat/queue-01", mustGit(t, work, "rev-parse", "HEAD"))
	mustGit(t, work, "remote", "set-url", "origin", "https://github.com:443/"+allowedSlug+".git")
	project := filepath.Dir(filepath.Dir(raw))
	requests := gittest.RoleHTTP(t, project, "heads-role-fixture")
	mints := 0
	restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mints++
		if role != "worker" || owner != "example-org" {
			t.Fatalf("wrong custody %s/%s", role, owner)
		}
		path := filepath.Join(os.Getenv("HOME"), "heads.token")
		err := os.WriteFile(path, []byte("heads-role-fixture"), 0600)
		return path, "", err
	})
	defer restore()
	// A read audit must not consume the remaining outward-write slot. Exercise
	// the real handler, persisted ledger and AllowWriteAt, not a copied predicate.
	now := time.Now().UTC()
	for i := 0; i < deskkit.RateLimitPerPRPerHour-1; i++ {
		if err := deskkit.Log(deskkit.Entry{Tool: "deskgit", Verb: "push", Repo: allowedSlug, Result: deskkit.ResultOK, TS: now.Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
	}
	var code int
	out := captureStdoutStderr(t, func() { code = run([]string{"heads", "--repo", allowedSlug}) })
	if code != 0 || !strings.Contains(out, "feat/queue-01") || !strings.Contains(out, `"complete":true`) {
		t.Fatalf("complete role heads: exit %d: %s", code, out)
	}
	if requests.Requests.Load() == 0 || mints != 1 {
		t.Fatalf("wire custody not exercised: requests=%d mints=%d", requests.Requests.Load(), mints)
	}
	if err := deskkit.AllowWriteAt("deskgit", allowedSlug, 0, now); err != nil {
		t.Fatalf("real heads consumed the remaining write budget: %v", err)
	}
	seen := false
	for _, e := range readAudit(t) {
		if e.Tool == "deskgit" && e.Verb == "heads" && e.Result == deskkit.ResultOK && e.Repo == allowedSlug {
			seen = true
		}
	}
	if !seen {
		t.Fatal("read classification removed the real heads audit")
	}
	requests.Broken.Store(true)
	out = captureStdoutStderr(t, func() { code = run([]string{"heads", "--repo", allowedSlug}) })
	if code == 0 || strings.Contains(out, `"complete":true`) {
		t.Fatalf("partial wire became complete: exit %d: %s", code, out)
	}
}

// regression: #3327
func TestHeadsRefusals(t *testing.T) {
	for _, kind := range []string{"host", "port", "repo", "multi", "role", "unset", "unknown", "forge-role", "custody"} {
		t.Run(kind, func(t *testing.T) {
			work := newRepo(t, allowedSlug)
			withEnv(t, work)
			t.Setenv("DESK_LOOP", "worker-desk")
			owner := "example-" + kind
			slug := owner + "/tracker"
			cfg := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
			if err := os.WriteFile(cfg, []byte(strings.ReplaceAll(fixtureRoster, "example-org", owner)), 0600); err != nil {
				t.Fatal(err)
			}
			if kind == "forge-role" {
				b, e := os.ReadFile(cfg)
				if e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(cfg, []byte(strings.ReplaceAll(string(b), "worker=assay-worker-app:300000006", "worker=gitlab:example-worker:300000006")), 0600); e != nil {
					t.Fatal(e)
				}
			}
			deskkit.ReloadConfig()
			gittest.RoleHTTP(t, t.TempDir(), "refusal-role-fixture")
			origin := "https://github.com/" + slug + ".git"
			switch kind {
			case "host":
				origin = "https://foreign.invalid/" + slug + ".git"
			case "port":
				origin = "https://github.com:444/" + slug + ".git"
			case "repo":
				origin = "https://github.com/" + owner + "/agents.git"
			case "unset":
				t.Setenv("DESK_LOOP", "")
			case "unknown":
				t.Setenv("DESK_LOOP", "unsupported")
			}
			mustGit(t, work, "remote", "set-url", "origin", origin)
			if kind == "multi" {
				mustGit(t, work, "config", "--add", "remote.origin.url", origin)
			}
			mints := 0
			restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
				mints++
				return filepath.Join(os.Getenv("HOME"), "missing.token"), "", nil
			})
			defer restore()
			var err error
			if kind == "role" {
				_, err = deskkit.CheckoutGitRead(work, slug, "desk")
			} else {
				err = cmdHeads([]string{"--repo", slug})
			}
			if err == nil {
				t.Fatal("unsafe role read admitted")
			}
			want := 0
			if kind == "custody" {
				want = 1
			}
			if mints != want {
				t.Fatalf("credential read count %d want %d", mints, want)
			}
		})
	}
}

// Test-only executable bridge: the subprocess still runs the real heads handler,
// custody file reader and go-git HTTP auth. Only the dial is confined to loopback.
// regression: #3327
func TestHeadsBridge(t *testing.T) {
	if os.Getenv("ASSAY_HEADS_TEST") != "1" {
		return
	}
	getwd = func() (string, error) { return os.Getenv("ASSAY_HEADS_ROOT"), nil }
	deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		if role != "worker" || owner != "example-org" {
			return "", "", fmt.Errorf("wrong role custody")
		}
		return os.Getenv("ASSAY_HEADS_CUSTODY"), "", nil
	})
	gittest.RoleHTTP(t, os.Getenv("ASSAY_HEADS_PROJECT"), "bridge-role-fixture")
	args := []string{}
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	os.Exit(run(args))
}
