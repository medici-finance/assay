package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// regression: #3327
func TestClaimBridgeFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX subprocess wrapper; wire owner coverage is platform neutral")
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "heads-test")
	cmd := exec.Command("go", "test", "-c", "-o", helper, "./cmd/deskgit")
	cmd.Dir = "../tools/desk"
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build bridge: %s %v", out, err)
	}
	root := t.TempDir()
	initRepoAt(t, root, fixedDate)
	runGit(t, root, "branch", "feat/queue-01")
	project := t.TempDir()
	bare := filepath.Join(project, "example-org", "tracker.git")
	if err := os.MkdirAll(filepath.Dir(bare), 0700); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "clone", "--bare", root, bare)
	runGit(t, root, "remote", "add", "origin", "https://github.com:443/example-org/tracker.git")
	home := t.TempDir()
	cfg := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(cfg, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "roster.env"), []byte("ASSAY_REPOS=example-org/tracker\nASSAY_REPO_FORGES=example-org/tracker=github\nASSAY_TRUSTED_BOT_SLUGS=worker=github:example-worker\n"), 0600); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(home, "role.token")
	if err := os.WriteFile(token, []byte("bridge-role-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nexec '" + helper + "' -test.run '^TestHeadsBridge$' -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "deskgit"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	realGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	trap := "#!/bin/sh\ncase \"$*\" in *ls-remote*|*fetch*) echo native-network-forbidden >&2; exit 99;; esac\nexec '" + realGit + "' \"$@\"\n"
	if e := os.WriteFile(filepath.Join(bin, "git"), []byte(trap), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("ASSAY_HEADS_TEST", "1")
	t.Setenv("ASSAY_HEADS_ROOT", root)
	t.Setenv("ASSAY_HEADS_PROJECT", project)
	t.Setenv("ASSAY_HEADS_CUSTODY", token)
	branches, err := listRemoteBranches(root)
	if err != nil || strings.Join(branches, ",") != "feat/queue-01,main" {
		t.Fatalf("complete custody bridge: branches=%v err=%v", branches, err)
	}
	claimed := claimedBriefs([]*Stream{{Name: "queue"}}, branches)
	if !claimed["queue/01"] {
		t.Fatalf("advertised claim not filtered: %v", claimed)
	}
	old := roleClaimReads
	roleClaimReads = true
	t.Cleanup(func() { roleClaimReads = old })
	for _, loop := range []string{"", "unsupported"} {
		t.Setenv("DESK_LOOP", loop)
		if branches, err := listRemoteBranches(root); err == nil || branches != nil {
			t.Fatalf("missing/unsupported role became known: %v %v", branches, err)
		}
	}
	t.Setenv("DESK_LOOP", "worker-desk")
	t.Setenv("ASSAY_HEADS_CUSTODY", filepath.Join(home, "missing.token"))
	if branches, err := listRemoteBranches(root); err == nil || branches != nil {
		t.Fatalf("unreadable role custody became known claims: %v %v", branches, err)
	}
}
