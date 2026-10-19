package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeRecipe returns the shell text make would run for target, so the recipe is
// exercised on a runner with no make. Any make expansion other than vars fails
// closed rather than letting the emulation drift from what make would run.
func makeRecipe(t *testing.T, makefile, target string, vars map[string]string) string {
	t.Helper()
	var lines []string
	in := false
	for _, line := range strings.Split(makefile, "\n") {
		switch {
		case strings.HasPrefix(line, target+":"):
			in = true
		case in && strings.HasPrefix(line, "\t"):
			lines = append(lines, strings.TrimPrefix(line, "\t"))
		case in:
			in = false
		}
	}
	if len(lines) == 0 {
		t.Fatalf("no %s recipe in Makefile", target)
	}
	body := strings.ReplaceAll(strings.TrimPrefix(strings.Join(lines, "\n"), "@"), "$$", "\x00")
	for k, v := range vars {
		body = strings.ReplaceAll(body, "$("+k+")", v)
	}
	if strings.Contains(body, "$") {
		t.Fatalf("recipe uses an unsupported make expansion:\n%s", body)
	}
	return strings.ReplaceAll(body, "\x00", "$")
}

// hookEntrypointRuns names how the Makefile entrypoint is run: always its recipe
// under /bin/sh (CI's runner has no make), plus real make wherever it exists.
func hookEntrypointRuns(t *testing.T) []string {
	if _, err := exec.LookPath("make"); err != nil {
		t.Logf("make not on PATH: Makefile recipe exercised under /bin/sh only")
		return []string{"recipe"}
	}
	return []string{"recipe", "make"}
}

func hookEntrypointCmd(t *testing.T, dir, run string, force bool) *exec.Cmd {
	t.Helper()
	vars := map[string]string{"DIST_DIR": "dist", "FORCE": ""}
	args := []string{"--no-print-directory", "desk-hook-install", "DIST_DIR=dist"}
	if force {
		vars["FORCE"] = "1"
		args = append(args, "FORCE=1")
	}
	cmd := exec.Command("make", args...)
	if run == "recipe" {
		recipe := makeRecipe(t, hookContent(t, filepath.Join(dir, "Makefile")), "desk-hook-install", vars)
		cmd = exec.Command("/bin/sh", "-c", recipe)
	}
	cmd.Dir = dir
	return cmd
}

// regression: #2801
func TestHookInstallMakeEntrypoint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Make entrypoint")
	}
	makefile := hookContent(t, "../../../../Makefile")
	for _, force := range []bool{false, true} {
		for _, name := range hookEntrypointRuns(t) {
			dir := t.TempDir()
			hookFile(t, filepath.Join(dir, "Makefile"), makefile)
			for _, sub := range []string{"dist", ".githooks"} {
				if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, ".githooks/pre-push")
			old := hookContent(t, "testdata/legacy-pre-push")
			hookFile(t, path, old)
			out, err := hookEntrypointCmd(t, dir, name, force).CombinedOutput()
			if _, ok := err.(*exec.ExitError); !ok {
				t.Fatalf("%s: missing built guard did not refuse: %v %s", name, err, out)
			}
			if hookContent(t, path) != old {
				t.Fatalf("%s: missing guard changed a hook", name)
			}
			if !strings.Contains(string(out), "desk-build") {
				t.Fatalf("%s: missing build guidance: %v %s", name, err, out)
			}
			// Recording stub: only the owned fixture executable can receive delegation.
			guard := filepath.Join(dir, "dist/deskpushguard")
			hookFile(t, guard, "#!/bin/sh\nprintf '%s\\n' \"$@\" > called\nexit 17\n")
			out, err = hookEntrypointCmd(t, dir, name, force).CombinedOutput()
			if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() == 0 {
				t.Fatalf("%s: guard refusal not propagated: %v %s", name, err, out)
			}
			want := "hook-install\n--dir\n.githooks\n"
			if force {
				want += "--force\n"
			}
			if got := hookContent(t, filepath.Join(dir, "called")); got != want {
				t.Fatalf("%s: delegation=%q want %q", name, got, want)
			}
			if hookContent(t, path) != old {
				t.Fatalf("%s: entrypoint changed hook outside guard", name)
			}
		}
	}
}

func windowsHookEntrypoint(t *testing.T, raw string) string {
	t.Helper()
	start := strings.Index(raw, "function Target-DeskHookInstall {")
	end := strings.Index(raw, "\nfunction Target-DeskTest {")
	if start < 0 || end <= start {
		t.Fatal("cannot locate Windows hook entrypoint")
	}
	return raw[start:end]
}
func singleWindowsHookWriter(body string) bool {
	return !strings.Contains(body, "Copy-Item") && !strings.Contains(body, "$HookSrc") &&
		strings.Count(body, "& $guard @hookArgs") == 1 && strings.Contains(body, "if ($Force)") &&
		strings.Contains(body, "$LASTEXITCODE -ne 0") && strings.Contains(body, "throw") &&
		strings.Contains(body, "desk-build")
}

// regression: #2801
func TestHookInstallWindowsEntrypoint(t *testing.T) {
	body := windowsHookEntrypoint(t, hookContent(t, "../../../../scripts/build-windows.ps1"))
	if !singleWindowsHookWriter(body) {
		t.Fatal("Windows installer retains a second hook writer or lacks guarded delegation")
	}
	// A planted second copy path must be detected, even if delegation remains.
	if singleWindowsHookWriter(body + "\nCopy-Item -Force legacy-hook .githooks/pre-push\n") {
		t.Fatal("planted second writer escaped")
	}
}

// regression: #2801
func TestHookInstallStaticAsset(t *testing.T) {
	// The published asset keeps the absolute guard anchor, and is the one Unix form
	// hook-install recognises and keeps without --force.
	if got := hookContent(t, "../../hooks/pre-push"); got != anchoredUnixShim {
		t.Fatal("published static shim differs from the anchored Unix hook")
	}
}
