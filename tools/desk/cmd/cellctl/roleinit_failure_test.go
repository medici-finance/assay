package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRoleInitFixtureProcess(t *testing.T) {
	if os.Getenv("CELLCTL_ROLEINIT_FIXTURE") != "1" {
		return
	}
	if os.Getenv("CELLCTL_ASSERT_CUSTODY") == "1" {
		for key, want := range map[string]string{
			"HOME": os.Getenv("CELLCTL_EXPECT_HOME"), "USERPROFILE": os.Getenv("CELLCTL_EXPECT_HOME"),
			"ASSAY_CONFIG_HOME": os.Getenv("CELLCTL_EXPECT_CONFIG"),
			"DESK_LOOP":         "worker-desk", "DESK_SESSION": "demo-worker-codex",
			"DESK_ROOTS": "example-org/project=/cell/project",
		} {
			if got := os.Getenv(key); got != want {
				fmt.Fprintf(os.Stderr, "%s = %q, want %q\n", key, got, want)
				os.Exit(7)
			}
		}
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
			assertDies(t, "role-init failure", func() { c.worktreeViaDeskwt("verifier", "sample-verifier") })
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
				assertDies(t, "invalid worktree", func() { c.worktreeViaDeskwt("verifier", "sample-verifier") })
				return
			}
			if got, ok := c.worktreeViaDeskwt("verifier", "sample-verifier"); !ok || got != path {
				t.Fatalf("got %s, %v", got, ok)
			}
		})
	}
	deskwtLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, ok := c.worktreeViaDeskwt("verifier", "sample-verifier"); ok {
		t.Fatal("missing tool did not select fallback")
	}
}

func TestRoleInitReceivesCellSessionAndCustody(t *testing.T) {
	oldCmd, oldLookup := deskwtCommand, deskwtLookPath
	t.Cleanup(func() { deskwtCommand, deskwtLookPath = oldCmd, oldLookup })
	t.Setenv("DESK_SESSION", "parent/unsafe-session")
	t.Setenv("DESK_LOOP", "the-desk")
	t.Setenv("DESK_ROOTS", "parent/repo=/parent")
	t.Setenv("ASSAY_CONFIG_HOME", t.TempDir())
	c := &Cell{Name: "demo", Repo: t.TempDir(), Home: t.TempDir(), Config: t.TempDir(),
		Env: envWith(map[string]string{"CELL_ROOTS": "example-org/project=/cell/project"})}
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".git"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	deskwtLookPath = func(string) (string, error) { return "fixture", nil }
	calls := 0
	deskwtCommand = func(_ string, args ...string) *exec.Cmd {
		calls++
		if args[len(args)-1] != "--help" {
			want := "role-init worker-desk --session demo-worker-codex"
			if got := strings.Join(args, " "); got != want {
				t.Fatalf("args %q, want %q", got, want)
			}
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestRoleInitFixtureProcess$")
		cmd.Env = append(os.Environ(), "CELLCTL_ROLEINIT_FIXTURE=1", "CELLCTL_ROLEINIT_PATH="+wt,
			"CELLCTL_ASSERT_CUSTODY=1", "CELLCTL_EXPECT_HOME="+c.Home, "CELLCTL_EXPECT_CONFIG="+c.Config)
		return cmd
	}
	if got, ok := c.worktreeViaDeskwt("worker-desk", "demo-worker-codex"); !ok || got != wt {
		t.Fatalf("got %q, %v", got, ok)
	}
	if calls != 2 {
		t.Fatalf("helper calls = %d", calls)
	}
	if os.Getenv("DESK_SESSION") != "parent/unsafe-session" {
		t.Fatal("parent environment changed")
	}
}
