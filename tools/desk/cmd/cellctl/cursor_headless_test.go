package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A compiled fixture models the documented print-mode distinction: without
// force an edit is proposed; with force it is applied except for explicit deny
// entries. This proves cellctl's route, not a live provider's implementation.
func init() {
	if os.Getenv("CELLCTL_CURSOR_PERMISSION_FIXTURE") != "1" {
		return
	}
	for _, arg := range os.Args[1:] {
		if arg == "--help" {
			if os.Getenv("CELLCTL_CURSOR_UNSUPPORTED_FIXTURE") == "1" {
				fmt.Print("--print --output-format --force bypass every restriction\n")
			} else {
				fmt.Print("--print Print response\n--output-format text\n-f, --force Force allow commands unless explicitly denied\n")
			}
			os.Exit(0)
		}
	}
	force, print, wt := false, false, ""
	for i, arg := range os.Args[1:] {
		switch arg {
		case "--force":
			force = true
		case "--print":
			print = true
		case "--workspace":
			if i+2 < len(os.Args) {
				wt = os.Args[i+2]
			}
		case "--yolo", "--trust", "--sandbox", "--approve-mcps":
			os.Exit(46)
		}
	}
	if !print || wt == "" {
		os.Exit(47)
	}
	if !force {
		fmt.Println("proposed changes only")
		os.Exit(0)
	}
	var cfg struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	data, err := os.ReadFile(filepath.Join(wt, ".cursor", "cli.json"))
	if err != nil || json.Unmarshal(data, &cfg) != nil {
		os.Exit(48)
	}
	for _, file := range []string{"allowed.txt", "denied.txt"} {
		denied := false
		for _, rule := range cfg.Permissions.Deny {
			if rule == "Write("+file+")" {
				denied = true
			}
		}
		if !denied {
			if err := os.WriteFile(filepath.Join(wt, file), []byte("fixture edit\n"), 0600); err != nil {
				os.Exit(49)
			}
		}
	}
	fmt.Print("tick role=worker-desk outcome=ok swept=1 acted=1 filed=0 duration=1\n")
	os.Exit(0)
}

func TestCursorHeadlessRouteAppliesAllowedEditsAndKeepsDenials(t *testing.T) {
	t.Setenv("CELLCTL_CURSOR_PERMISSION_FIXTURE", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = cursorHeadlessPreflight(exe); err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	writeCursorWorkspace(t, wt, "worker-desk")
	config := []byte(`{"permissions":{"allow":["Write(allowed.txt)"],"deny":["Write(denied.txt)"]}}`)
	path := filepath.Join(wt, ".cursor", "cli.json")
	if err = os.WriteFile(path, config, 0600); err != nil {
		t.Fatal(err)
	}
	argv := cursorLaunchArgv("worker-desk", "explicit-model", "session", wt)
	argv[0] = exe
	args, env, err := prepareTickLaunch("cursor", argv, os.Environ(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Dir = wt
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), "outcome=ok") {
		t.Fatalf("headless route failed: %v %s", err, out)
	}
	if _, err = os.Stat(filepath.Join(wt, "allowed.txt")); err != nil {
		t.Fatal("Cursor fixture only proposed the required edit")
	}
	if _, err = os.Stat(filepath.Join(wt, "denied.txt")); !os.IsNotExist(err) {
		t.Fatal("explicit denial was lost")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(config) {
		t.Fatal("headless route rewrote permissions")
	}
}

func TestCursorHeadlessRouteRefusesUnprovenPermissionSemantics(t *testing.T) {
	t.Setenv("CELLCTL_CURSOR_PERMISSION_FIXTURE", "1")
	t.Setenv("CELLCTL_CURSOR_UNSUPPORTED_FIXTURE", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = cursorHeadlessPreflight(exe); err == nil {
		t.Fatal("unknown force behavior accepted")
	}
	if err = cursorHeadlessPreflight(filepath.Join(t.TempDir(), "missing-agent")); err == nil {
		t.Fatal("missing Cursor runner accepted")
	}
}
