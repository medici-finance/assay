package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRoleInitFixtureProcess(t *testing.T) {
	if os.Getenv("CELLCTL_ROLEINIT_FIXTURE") != "1" {
		return
	}
	n, _ := strconv.Atoi(os.Getenv("CELLCTL_ROLEINIT_EXIT"))
	if n != 0 {
		fmt.Fprintln(os.Stderr, "fixture role initialization refused")
		os.Exit(n)
	}
	fmt.Println(os.Getenv("CELLCTL_ROLEINIT_PATH"))
	os.Exit(0)
}

func TestRoleInitFailureNeverFallsBack(t *testing.T) {
	oldCmd, oldLookup := deskwtCommand, deskwtLookPath
	t.Cleanup(func() { deskwtCommand, deskwtLookPath = oldCmd, oldLookup })
	deskwtLookPath = func(string) (string, error) { return "fixture", nil }
	c := &Cell{Repo: t.TempDir(), Env: &Env{vals: map[string]string{}, set: map[string]bool{}}}
	for _, code := range []int{1, 3, 4, 5, 6} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			deskwtCommand = func(_ string, args ...string) *exec.Cmd {
				rc := code
				if args[len(args)-1] == "--help" {
					rc = 0
				}
				cmd := exec.Command(os.Args[0], "-test.run=^TestRoleInitFixtureProcess$")
				cmd.Env = append(os.Environ(), "CELLCTL_ROLEINIT_FIXTURE=1", "CELLCTL_ROLEINIT_EXIT="+strconv.Itoa(rc))
				return cmd
			}
			assertDies(t, "role-init failure", func() { c.worktreeViaDeskwt("verifier") })
		})
	}
	for _, valid := range []bool{false, true} {
		t.Run(fmt.Sprintf("valid=%v", valid), func(t *testing.T) {
			path := t.TempDir()
			if valid {
				if err := os.WriteFile(filepath.Join(path, ".git"), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			deskwtCommand = func(_ string, _ ...string) *exec.Cmd {
				cmd := exec.Command(os.Args[0], "-test.run=^TestRoleInitFixtureProcess$")
				cmd.Env = append(os.Environ(), "CELLCTL_ROLEINIT_FIXTURE=1", "CELLCTL_ROLEINIT_EXIT=0", "CELLCTL_ROLEINIT_PATH="+path)
				return cmd
			}
			if !valid {
				assertDies(t, "invalid worktree", func() { c.worktreeViaDeskwt("verifier") })
				return
			}
			if got, ok := c.worktreeViaDeskwt("verifier"); !ok || got != path {
				t.Fatalf("got %s, %v", got, ok)
			}
		})
	}
	deskwtLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, ok := c.worktreeViaDeskwt("verifier"); ok {
		t.Fatal("missing tool did not select fallback")
	}
}
