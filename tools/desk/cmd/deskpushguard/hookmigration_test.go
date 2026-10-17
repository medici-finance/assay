package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func hookFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
}
func hookContent(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// regression: #2801
func TestHookInstallLegacyMigration(t *testing.T) {
	legacy := hookContent(t, "testdata/legacy-pre-push")
	for _, target := range []string{"linux", "windows"} {
		for _, force := range []bool{false, true} {
			for _, crlf := range []bool{false, true} {
				name := target
				if force {
					name += "/force"
				}
				if crlf {
					name += "/crlf"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					old := legacy
					if crlf {
						old = strings.ReplaceAll(old, "\n", "\r\n")
					}
					path := filepath.Join(dir, "pre-push")
					hookFile(t, path, old)
					installed, err := writeHooks(dir, target, force)
					if err != nil {
						t.Fatal(err)
					}
					want := 1
					if target == "windows" {
						want = 2
					}
					if len(installed) != want || hookContent(t, path) != unixShim {
						t.Fatalf("legacy hook not migrated: installed=%v content=%q", installed, hookContent(t, path))
					}
					if target == "windows" && hookContent(t, filepath.Join(dir, "pre-push.cmd")) != windowsCmdShim {
						t.Fatal("missing current Windows pair")
					}
					again, err := writeHooks(dir, target, force)
					if err != nil || len(again) != 0 {
						t.Fatalf("current pair not idempotent: %v %v", again, err)
					}
				})
			}
		}
	}
}

// regression: #2801
func TestHookInstallForeignAndPartialPair(t *testing.T) {
	legacy := hookContent(t, "testdata/legacy-pre-push")
	for _, foreign := range []string{"#!/bin/sh\necho custom\n", "#!/bin/sh\n# deskpushguard customization\nexit 0\n", legacy + "echo custom\n"} {
		dir := t.TempDir()
		path := filepath.Join(dir, "pre-push")
		hookFile(t, path, foreign)
		if _, err := writeHooks(dir, "linux", false); err == nil {
			t.Errorf("accepted foreign hook %q", foreign)
		}
		if hookContent(t, path) != foreign {
			t.Fatal("refusal modified foreign hook")
		}
		if _, err := writeHooks(dir, "linux", true); err != nil {
			t.Fatal(err)
		}
		if hookContent(t, path) != unixShim {
			t.Fatal("explicit force did not replace foreign hook")
		}
	}
	for _, first := range []string{"", legacy, unixShim} {
		dir := t.TempDir()
		path := filepath.Join(dir, "pre-push")
		if first != "" {
			hookFile(t, path, first)
		}
		partner := filepath.Join(dir, "pre-push.cmd")
		foreign := "@echo off\nrem deskpushguard custom\nexit /b 0\n"
		hookFile(t, partner, foreign)
		if _, err := writeHooks(dir, "windows", false); err == nil {
			t.Error("accepted foreign Windows partner")
		}
		if first == "" {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("refused pair created first hook")
			}
		} else if hookContent(t, path) != first {
			t.Fatal("refused pair changed first hook")
		}
		if hookContent(t, partner) != foreign {
			t.Fatal("refused pair changed foreign partner")
		}
		if _, err := writeHooks(dir, "windows", true); err != nil {
			t.Fatal(err)
		}
		if hookContent(t, path) != unixShim || hookContent(t, partner) != windowsCmdShim {
			t.Fatal("force did not complete pair")
		}
	}
	// A current Unix half plus a missing Windows half must complete the pair.
	dir := t.TempDir()
	hookFile(t, filepath.Join(dir, "pre-push"), unixShim)
	installed, err := writeHooks(dir, "windows", false)
	if err != nil || len(installed) != 1 || hookContent(t, filepath.Join(dir, "pre-push.cmd")) != windowsCmdShim {
		t.Fatalf("partial owned pair: %v %v", installed, err)
	}
}

// regression: #2801
func TestHookInstallGuardInvocation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX hook execution; Windows pair bytes covered separately")
	}
	dir := t.TempDir()
	hook := filepath.Join(dir, "pre-push")
	hookFile(t, hook, hookContent(t, "testdata/legacy-pre-push"))
	if _, err := writeHooks(dir, "linux", false); err != nil {
		t.Fatal(err)
	}
	if hookContent(t, hook) != unixShim {
		t.Fatal("legacy hook not migrated; refusing to execute it")
	}
	guard := filepath.Join(dir, "deskpushguard")
	hookFile(t, guard, "#!/bin/sh\nprintf 'args=<%s><%s>\\n' \"$1\" \"$2\"\nwhile IFS= read -r line; do printf 'stdin=<%s>\\n' \"$line\"; done\nexit 23\n")
	cmd := exec.Command("/bin/sh", hook, "origin", "ssh://example.invalid/repo with space")
	cmd.Env = append(os.Environ(), "PATH="+dir)
	cmd.Stdin = strings.NewReader("refs/heads/topic abc refs/heads/topic def\n")
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 23 {
		t.Fatalf("guard status not propagated: %v %s", err, out)
	}
	if string(out) != "args=<origin><ssh://example.invalid/repo with space>\nstdin=<refs/heads/topic abc refs/heads/topic def>\n" {
		t.Fatalf("guard argv/stdin changed: %q", out)
	}
	if err := os.Remove(guard); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("/bin/sh", hook)
	cmd.Env = append(os.Environ(), "PATH="+dir)
	if err := cmd.Run(); err == nil {
		t.Fatal("missing guard allowed hook success")
	}
}
