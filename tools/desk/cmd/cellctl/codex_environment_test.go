package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func codexEnvironmentCell(t *testing.T) *Cell {
	t.Helper()
	root := t.TempDir()
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	e.Put("HOME", filepath.Join(root, "operator"))
	e.Put("USERPROFILE", e.Get("HOME"))
	e.Put("PATH", os.Getenv("PATH"))
	e.Put("DESK_TOOLS_BIN", filepath.Join(root, "desk tools"))
	c := &Cell{Env: e, Dir: filepath.Join(root, "cell"), Home: filepath.Join(root, "cell home")}
	c.Config = filepath.Join(c.Home, ".config", "assay")
	for _, dir := range []string{c.Home, e.Get("HOME"), e.Get("DESK_TOOLS_BIN")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestCodexCommandEnvironmentSeparatesHomes(t *testing.T) {
	c := codexEnvironmentCell(t)
	env := []string{"HOME=" + c.Env.Get("HOME"), "DESK_LOOP=worker-desk", "DESK_ROOTS=example/repo=" + t.TempDir(), "GH_TOKEN=never-on-argv", "CUSTOM_SECRET=never-on-argv"}
	before := append([]string(nil), env...)
	args, err := c.codexEnvironmentArgs(env)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(env, before) {
		t.Fatal("harness environment was mutated")
	}
	values := map[string]string{}
	for _, arg := range args {
		if strings.Contains(arg, "never-on-argv") {
			t.Fatal("credential copied onto argv")
		}
		if rest, ok := strings.CutPrefix(arg, "shell_environment_policy.set."); ok {
			key, raw, _ := strings.Cut(rest, "=")
			var value string
			if err := json.Unmarshal([]byte(raw), &value); err != nil {
				t.Fatal(err)
			}
			values[key] = value
		}
	}
	for _, key := range []string{"HOME", "USERPROFILE", "ZDOTDIR"} {
		if values[key] != c.Home {
			t.Fatalf("%s=%q", key, values[key])
		}
	}
	if values["CODEX_HOME"] != filepath.Join(c.Env.Get("HOME"), ".codex") || values["GH_CONFIG_DIR"] != filepath.Join(c.Env.Get("HOME"), ghConfigRelPath) {
		t.Fatal("operator login homes lost")
	}
	wantPath := filepath.Join(c.Dir, "bin") + string(filepath.ListSeparator) + c.Env.Get("DESK_TOOLS_BIN") + string(filepath.ListSeparator) + c.Env.Get("PATH")
	if values["PATH"] != wantPath {
		t.Fatalf("cell binaries must precede native desk tools and operator PATH: got %q, want %q", values["PATH"], wantPath)
	}
	if values["DESK_LOOP"] != "worker-desk" {
		t.Fatal("role lost")
	}
	// Native Windows installations often carry USERPROFILE without HOME, and resolve. Off
	// windows os.UserHomeDir reads HOME alone, so a USERPROFILE-only environment refuses rather
	// than resolving a home every launched tool would then fail to find.
	c.Env.Put("HOME", "")
	_, err = c.codexEnvironmentArgs(env)
	if runtime.GOOS == "windows" && err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && (err == nil || !strings.Contains(err.Error(), "HOME is not set")) {
		t.Fatalf("USERPROFILE-only off windows must refuse, got %v", err)
	}
}

func TestCodexBashSkipsOperatorStartup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix Bash startup regression; Windows uses native shell")
	}
	c := codexEnvironmentCell(t)
	for _, name := range []string{".zshrc", ".zshenv", ".bashrc", ".bash_profile", "bash-env"} {
		if err := os.WriteFile(filepath.Join(c.Env.Get("HOME"), name), []byte("echo STARTUP_RAN; export PATH=/wrong\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	env := envSet(os.Environ(), "HOME", c.Env.Get("HOME"))
	env = envSet(env, "BASH_ENV", filepath.Join(c.Env.Get("HOME"), "bash-env"))
	values, err := c.codexCommandEnvironment(env)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		env = envSet(env, key, value)
	}
	bash, err := codexBashPath()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", `printf '%s\n%s\n%s\n' "$HOME" "$PATH" "$BASH_VERSION"`)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 3 || lines[0] != c.Home || lines[1] != values["PATH"] || lines[2] == "" || strings.Contains(string(out), "STARTUP_RAN") {
		t.Fatalf("startup isolation failed: %s", out)
	}
}

func TestCodexShellInstructionsArePlatformSpecific(t *testing.T) {
	unix := codexRolePrompt("worker-desk", "darwin", "/bin/bash")
	if !strings.Contains(unix, `shell="/bin/bash"`) || !strings.Contains(unix, "login=false") {
		t.Fatal(unix)
	}
	win := codexRolePrompt("worker-desk", "windows", "")
	if strings.Contains(win, "/bin/bash") || strings.Contains(win, "shell=\"\"") || !strings.Contains(win, "native Windows") {
		t.Fatal(win)
	}
}

func TestCodexNativeOperatorHomes(t *testing.T) {
	c := codexEnvironmentCell(t)
	// Windows-shaped, so the windows resolver's absolute-path check holds on any host.
	c.Env.Put("USERPROFILE", `C:\Profiles\native home`)
	if got, err := operatorHomeFor("windows", c.Env); err != nil || got != c.Env.Get("USERPROFILE") {
		t.Fatal("Windows used Git Bash HOME")
	}
	if got, err := operatorHomeFor("darwin", c.Env); err != nil || got != c.Env.Get("HOME") {
		t.Fatal("Unix home changed")
	}
	c.Env.Put("APPDATA", filepath.Join(t.TempDir(), "AppData", "Roaming"))
	if got, _ := ghConfigDirFor("windows", c.Env); got != filepath.Join(c.Env.Get("APPDATA"), "GitHub CLI") {
		t.Fatal(got)
	}
	c.Env.Put("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	if got, _ := ghConfigDirFor("windows", c.Env); got != filepath.Join(c.Env.Get("XDG_CONFIG_HOME"), "gh") {
		t.Fatal(got)
	}
	c.Env.Put("GH_CONFIG_DIR", filepath.Join(t.TempDir(), "gh override"))
	if got, _ := ghConfigDirFor("windows", c.Env); got != c.Env.Get("GH_CONFIG_DIR") {
		t.Fatal(got)
	}
}
