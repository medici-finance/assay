package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCounterEntrypoint(t *testing.T) {
	if root := os.Getenv("GITLAUNCH_TEST_ROOT"); root != "" {
		os.Args = []string{"gitlaunch", root}
		main()
	}
}

// regression: #2395
func TestCounterTrust(t *testing.T) {
	git := func(t *testing.T, root string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("synthetic git %v: %s %v", args, out, err)
		}
	}
	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	inv := func(t *testing.T, root string) inventory {
		t.Helper()
		sites, err := scan(root)
		if err != nil {
			t.Fatal(err)
		}
		out := inventory{Reasons: map[string]reason{"fixture": {Operation: "synthetic Git read", Why: "test control", Owner: "test fixture"}}}
		for _, s := range sites {
			s.Disposition = "retained"
			s.Reason = "fixture"
			out.Sites = append(out.Sites, s)
			if strings.Contains(s.Key, "::direct#") {
				out.Direct++
			} else if strings.Contains(s.Key, "::dynamic#") {
				out.Dynamic++
			} else {
				out.Seams++
			}
		}
		b, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "git-launches.json"), string(b))
		return out
	}
	run := func(t *testing.T, root string, wantSuccess bool) {
		t.Helper()
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestCounterEntrypoint$")
		cmd.Env = append(os.Environ(), "GITLAUNCH_TEST_ROOT="+root)
		out, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("entrypoint success=%v, wanted %v: %s %v", err == nil, wantSuccess, out, err)
		}
		if !wantSuccess && !strings.Contains(string(out), "may only decrease") {
			t.Fatalf("wrong refusal: %s %v", out, err)
		}
		t.Logf("entrypoint success=%v: %s", err == nil, out)
	}
	setup := func(t *testing.T, seed bool) (string, string) {
		t.Helper()
		repo := t.TempDir()
		root := filepath.Join(repo, "statusgen")
		if err := os.Mkdir(root, 0755); err != nil {
			t.Fatal(err)
		}
		git(t, repo, "init", "-q")
		git(t, repo, "config", "user.name", "Fixture")
		git(t, repo, "config", "user.email", "fixture@example.invalid")
		git(t, repo, "config", "commit.gpgsign", "false")
		if seed {
			write(t, filepath.Join(root, "read.go"), `package p;import "os/exec";func read(){exec.Command("git","status")}`)
			inv(t, root)
		} else {
			write(t, filepath.Join(repo, "README"), "initial\n")
		}
		git(t, repo, "add", "-A")
		git(t, repo, "commit", "-qm", "immutable baseline")
		git(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
		return repo, root
	}
	t.Run("coordinated-source-inventory-cap-raise", func(t *testing.T) {
		_, root := setup(t, true)
		write(t, filepath.Join(root, "added.go"), `package p;import "os/exec";func added(){exec.Command("git","status")}`)
		raised := inv(t, root)
		sites, err := scan(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := verify(sites, raised); err != nil {
			t.Fatalf("current inventory positive control: %v", err)
		}
		run(t, root, false)
	})
	t.Run("retirement-lowers-cap", func(t *testing.T) {
		_, root := setup(t, true)
		if err := os.Remove(filepath.Join(root, "read.go")); err != nil {
			t.Fatal(err)
		}
		inv(t, root)
		run(t, root, true)
	})
	t.Run("initial-seed-proven-absence", func(t *testing.T) {
		_, root := setup(t, false)
		inv(t, root)
		run(t, root, true)
	})
}
