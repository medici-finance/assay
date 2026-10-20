package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #3327
func TestDeskClaimsNoFallback(t *testing.T) {
	root := t.TempDir()
	initRepoAt(t, root, fixedDate)
	runGit(t, root, "remote", "add", "origin", "https://github.com/example/project.git")
	bin := t.TempDir()
	marker := filepath.Join(bin, "raw-called")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	// Raw Git would falsely succeed. A desk read must refuse the failed bridge.
	for name, body := range map[string]string{"git": "case \"$*\" in *ls-remote*) echo raw >'" + marker + "'; exit 0;; esac\nexec '" + realGit + "' \"$@\"", "deskgit": "echo unreadable-role-custody >&2\nexit 6"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DESK_LOOP", "worker-desk")
	branches, err := listRemoteBranches(root)
	if err == nil || !strings.Contains(err.Error(), "unreadable-role-custody") || branches != nil {
		t.Fatalf("failed role read became known claims: %v %v", branches, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("desk read fell through to ambient raw Git")
	}
}

// regression: #3327
func TestRoleClaimsPartial(t *testing.T) {
	root := t.TempDir()
	initRepoAt(t, root, fixedDate)
	runGit(t, root, "remote", "add", "origin", "https://github.com/example/project.git")
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	realGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\ncase \"$*\" in *ls-remote*) exit 0;; esac\nexec '"+realGit+"' \"$@\"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("DESK_LOOP", "")
	old := roleClaimReads
	roleClaimReads = true
	t.Cleanup(func() { roleClaimReads = old })
	for _, body := range []string{
		`{"schema":1,"repo":"example/project","complete":false,"heads":["refs/heads/feat/queue-01"]}`,
		`{"schema":1,"repo":"example/project","heads":[]}`,
		`{"schema":1,"repo":"example/other","complete":true,"heads":[]}`,
		`{"schema":99,"repo":"example/project","complete":true,"heads":[]}`,
		`{"schema":1,"repo":"example/project","complete":true,"heads":null}`,
		`{"schema":1,"repo":"example/project","complete":true,"heads":["refs/heads/main","refs/heads/main"]}`,
		`{"schema":1,"repo":"example/project","complete":true,"heads":["refs/tags/v1"]}`,
		`{`,
	} {
		script := "#!/bin/sh\necho '" + body + "'\n"
		if err := os.WriteFile(filepath.Join(bin, "deskgit"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		if branches, err := listRemoteBranches(root); err == nil || branches != nil {
			t.Fatalf("partial or ambiguous heads accepted: %s -> %v %v", body, branches, err)
		}
	}
}
