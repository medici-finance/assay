package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
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

// regression: #3324
// Exercise both prompt-producing paths: scrubbed cells bypass codexRolePrompt.
// This checks emitted instructions, not whether a live model follows them.
func TestCodexSupervisionAcrossLaunchPaths(t *testing.T) {
	scrubbed := harnessArgv("codex", "worker-desk", "selected-model", "session", "/wt")
	for name, prompt := range map[string]string{
		"host-unix":    codexRolePrompt("worker-desk", "darwin", "/bin/bash"),
		"host-windows": codexRolePrompt("worker-desk", "windows", ""),
		"scrubbed":     scrubbed[len(scrubbed)-1],
	} {
		t.Run(name, func(t *testing.T) {
			for _, instruction := range []string{
				"Block on existing child/command handles until the next required check",
				"After an early timeout, re-arm the wait",
				"operator-selected outer cadence after a clean handoff",
				`fork_turns="none"`,
				"independent reviews, stop checks and required freshness checks",
			} {
				if strings.Count(prompt, instruction) != 1 {
					t.Errorf("launch must carry %q exactly once", instruction)
				}
			}
		})
	}
}

// stampModeProblems reads a Codex launch prompt's dispatch-admission reminder
// sentence by sentence and names every mode whose stamp timing is wrong. It is
// the oracle the launch-path test and the mutants below share.
func stampModeProblems(prompt string) []string {
	var problems []string
	if strings.Contains(prompt, "Check the dispatch receipt for a successful model stamp before spending work on that dispatch") {
		problems = append(problems, "unconditional pre-work stamp requirement survives")
	}
	var strict, fresh, postOpen []string
	for _, line := range strings.Split(prompt, "\n") {
		for _, s := range strings.Split(line, ". ") {
			switch {
			case strings.HasPrefix(s, "Admission by dispatch mode:"):
				strict = append(strict, s)
			case strings.HasPrefix(s, "A fresh worker dispatch"):
				fresh = append(fresh, s)
			case strings.HasPrefix(s, "After that worker's draft PR opens"):
				postOpen = append(postOpen, s)
			}
		}
	}
	if !strings.Contains(prompt, "must include --model with the actual selected model and keep its dispatch receipt") {
		problems = append(problems, "every dispatch: actual --model and retained receipt not required")
	}
	if len(strict) != 1 || len(fresh) != 1 || len(postOpen) != 1 {
		return append(problems, fmt.Sprintf("want one sentence per mode group, got strict=%d fresh=%d post-open=%d", len(strict), len(fresh), len(postOpen)))
	}
	// Review, verifier, explicit --attest-worker and existing-PR stamps keep the readback.
	for mode, word := range map[string]string{
		"review":          "a review dispatch",
		"verifier":        "verifier admission",
		"attest-worker":   "an explicit --attest-worker dispatch",
		"existing PR":     "any stamp on an existing PR (--pr, including --stamp-only)",
		"readback":        "require a successful, read-back model stamp in the receipt before spending work",
		"pending refused": "PENDING",
		"failure refused": "failed, mismatched or unreadable stamp is not a successful admission",
		"skipped refused": "a skipped",
	} {
		if !strings.Contains(strict[0], word) {
			problems = append(problems, mode+": strict-mode readback lost "+strconv.Quote(word))
		}
	}
	// A fresh worker proceeds on PENDING, never called attested, never asked for the readback.
	for _, word := range []string{"may start while its receipt reports the stamp PENDING", "PENDING is not attested and is never reported as attested", "--attest-worker is opt-in"} {
		if !strings.Contains(fresh[0], word) {
			problems = append(problems, "fresh worker: lost "+strconv.Quote(word))
		}
	}
	for _, word := range []string{"successful", "read-back", "readback", "before spending work", "requires", "must"} {
		if strings.Contains(fresh[0], word) {
			problems = append(problems, "fresh worker: demands a pre-work stamp ("+strconv.Quote(word)+")")
		}
	}
	// The coordinator stamps the actual selection post-open and reads it back.
	for _, word := range []string{"the coordinator desk runs the receipt's --stamp-only command", "with the actual selected model and tier", "reads the stamp back before any downstream step relies on model-attested authoring"} {
		if !strings.Contains(postOpen[0], word) {
			problems = append(problems, "post-open: lost "+strconv.Quote(word))
		}
	}
	return problems
}

// regression: #6706
// The host and scrubbed launch prompts each split stamp timing by dispatch mode:
// fresh worker, existing PR, review, verifier, pending and failure results.
func TestCodexStampModesAcrossPaths(t *testing.T) {
	scrubbed := harnessArgv("codex", "worker-desk", "selected-model", "session", "/wt")
	prompts := map[string]string{
		"host-unix":    codexRolePrompt("worker-desk", "darwin", "/bin/bash"),
		"host-windows": codexRolePrompt("worker-desk", "windows", ""),
		"scrubbed":     scrubbed[len(scrubbed)-1],
	}
	for _, role := range []string{"the-desk", "pr-review-desk", "verify-desk", "intake-desk"} {
		prompts["host-"+role] = codexRolePrompt(role, "linux", "/bin/bash")
		argv := harnessArgv("codex", role, "selected-model", "session", "/wt")
		prompts["scrubbed-"+role] = argv[len(argv)-1]
	}
	for name, prompt := range prompts {
		t.Run(name, func(t *testing.T) {
			if n := strings.Count(prompt, codexStampPrompt); n != 1 {
				t.Errorf("launch must carry the stamp reminder exactly once, got %d", n)
			}
			for _, p := range stampModeProblems(prompt) {
				t.Error(p)
			}
		})
	}
}

// regression: #6706
// Each mutant re-breaks one mode of the split; the oracle must catch every one,
// so a green TestCodexStampModesAcrossPaths is not a vacuous pass.
func TestCodexStampModeMutants(t *testing.T) {
	good := codexRolePrompt("worker-desk", "darwin", "/bin/bash")
	if p := stampModeProblems(good); len(p) != 0 {
		t.Fatalf("baseline must be clean: %v", p)
	}
	for name, mutate := range map[string]func(string) string{
		"fresh worker demands readback again": func(s string) string {
			return strings.Replace(s, "may start while its receipt reports the stamp PENDING", "requires a successful, read-back model stamp before spending work; it may start while its receipt reports the stamp PENDING", 1)
		},
		"review drops readback":   func(s string) string { return strings.Replace(s, "a review dispatch, ", "", 1) },
		"verifier drops readback": func(s string) string { return strings.Replace(s, "verifier admission, ", "", 1) },
		"attest-worker drops readback": func(s string) string {
			return strings.Replace(s, "an explicit --attest-worker dispatch, ", "", 1)
		},
		"existing PR drops readback": func(s string) string {
			return strings.Replace(s, ", and any stamp on an existing PR (--pr, including --stamp-only)", "", 1)
		},
		"pending admits strict modes": func(s string) string { return strings.Replace(s, "a skipped, PENDING, failed", "a skipped, failed", 1) },
		"failure admits strict modes": func(s string) string {
			return strings.Replace(s, "failed, mismatched or unreadable stamp is not a successful admission", "skipped stamp is not a successful admission", 1)
		},
		"pending called attested": func(s string) string {
			return strings.Replace(s, "PENDING is not attested and is never reported as attested", "PENDING counts as attested", 1)
		},
		"post-open readback dropped": func(s string) string {
			return strings.Replace(s, " and reads the stamp back before any downstream step relies on model-attested authoring", "", 1)
		},
		"pre-fix unconditional line": func(s string) string {
			return strings.Replace(s, codexStampPrompt, "\nEvery deskdispatch that will post a model-attested result must include --model with the actual selected worker model. Check the dispatch receipt for a successful model stamp before spending work on that dispatch; a skipped stamp is not a successful review admission.", 1)
		},
	} {
		mutant := mutate(good)
		if mutant == good {
			t.Errorf("%s: mutation did not apply", name)
			continue
		}
		if len(stampModeProblems(mutant)) == 0 {
			t.Errorf("%s: mutant survived", name)
		}
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
