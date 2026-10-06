package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// All subprocess tools are local stubs; the check never contacts a service.
func TestLaunchedHouseCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executables are Unix shell scripts")
	}
	f := newPolicyFixture(t, "2.1.278")
	operator := filepath.Join(t.TempDir(), "operator")
	cfg := filepath.Join(operator, ".config", "assay")
	claude := filepath.Join(operator, ".claude")
	cellHome := filepath.Join(f.cellDir, "home")
	cellConfig := filepath.Join(cellHome, ".config", "assay")
	for _, p := range []string{cfg, claude, filepath.Join(cellHome, ghConfigRelPath), filepath.Join(f.repoDir, "docs", "streams"), filepath.Join(f.repoDir, ".agents", "skills", "example")} {
		must(t, os.MkdirAll(p, 0700))
	}
	must(t, os.Remove(filepath.Join(cellConfig, "roster.env")))
	must(t, os.Remove(cellConfig))
	must(t, os.Symlink(cfg, cellConfig))
	must(t, os.WriteFile(filepath.Join(cfg, "roster.env"), []byte("ASSAY_BLESS_LOGIN=example-human:1\nASSAY_TRUSTED_LOGINS=example-human:1\nASSAY_ALLOWED_REPOS=example-org/example-repo:no-ci:public\n"), 0600))
	must(t, os.WriteFile(filepath.Join(cellHome, ".gitconfig"), nil, 0600))
	must(t, os.WriteFile(filepath.Join(f.repoDir, "AGENTS.md"), []byte("Assay resident operating rules"), 0600))
	cmd := exec.Command("git", "init", "-q", f.repoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, name := range strings.Fields(houseVerbs + " tmux") {
		must(t, os.WriteFile(filepath.Join(f.binDir, name), []byte("#!/bin/sh\nexit 0\n"), 0700))
	}
	must(t, os.WriteFile(filepath.Join(f.binDir, "codex"), []byte("#!/bin/sh\necho 'multi_agent stable true'\n"), 0700))
	must(t, os.WriteFile(filepath.Join(f.binDir, "claude"), []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo 2.1.278; else echo '[{\"id\":\"assay@assay\",\"enabled\":true}]'; fi\n"), 0700))
	file, err := os.OpenFile(filepath.Join(f.cellDir, "cell.env"), os.O_APPEND|os.O_WRONLY, 0600)
	must(t, err)
	_, err = file.WriteString("CELL_HARNESS=codex\nCELL_COCKPIT=tmux\nDESKD=0\n")
	must(t, err)
	must(t, file.Close())
	e := envWith(map[string]string{"HOME": operator, "USERPROFILE": operator, "PATH": f.binDir + ":/usr/bin:/bin", "DESK_TOOLS_BIN": f.binDir})
	c := &Cell{Env: e, Dir: f.cellDir, Home: cellHome, Config: cellConfig}
	values, err := c.codexCommandEnvironment(nil)
	must(t, err)
	run := func(env map[string]string, arg string) (string, error) {
		args := []string{"check", "example"}
		if arg != "" {
			args = append(args, arg)
		}
		cmd := exec.Command(cellctlBinary(t), args...)
		cmd.Env = []string{"CELLS_ROOT=" + f.cellsRoot, "DESK_TOOLS_BIN=" + f.binDir, "KUBECONFIG=/dev/null", "ZAI_API_KEY=fixture-zai", "KIMI_API_KEY=fixture-kimi"}
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	t.Run("host", func(t *testing.T) {
		out, err := run(e.vals, "")
		if err != nil {
			t.Fatalf("host check: %v\n%s", err, out)
		}
	})
	t.Run("launched", func(t *testing.T) {
		out, err := run(values, "")
		if err != nil {
			t.Fatalf("launched check: %v\n%s", err, out)
		}
	})
	t.Run("explicit-claude", func(t *testing.T) {
		out, err := run(values, claude)
		if err != nil {
			t.Fatalf("positional check: %v\n%s", err, out)
		}
	})
	t.Run("missing-claude", func(t *testing.T) {
		out, err := run(values, filepath.Join(operator, "missing"))
		if err == nil || !strings.Contains(out, "CLAUDE_CONFIG_DIR not a directory") {
			t.Fatalf("missing Claude not refused: %v\n%s", err, out)
		}
	})
	t.Run("misdirected-config", func(t *testing.T) {
		env := map[string]string{}
		for k, v := range values {
			env[k] = v
		}
		env["ASSAY_CONFIG_HOME"] = t.TempDir()
		out, err := run(env, claude)
		if err == nil || !strings.Contains(out, "MISS  config home linked") {
			t.Fatalf("misdirected config not refused: %v\n%s", err, out)
		}
	})
	t.Run("broken-config", func(t *testing.T) {
		must(t, os.Remove(cellConfig))
		must(t, os.Symlink(filepath.Join(operator, "missing"), cellConfig))
		out, err := run(values, claude)
		if err == nil || !strings.Contains(out, "MISS  config home linked") {
			t.Fatalf("broken config not refused: %v\n%s", err, out)
		}
		must(t, os.Remove(cellConfig))
		must(t, os.Symlink(cfg, cellConfig))
	})
	t.Run("relative-alias", func(t *testing.T) {
		must(t, os.Remove(cellConfig))
		rel, err := filepath.Rel(filepath.Dir(cellConfig), cfg)
		must(t, err)
		must(t, os.Symlink(rel, cellConfig))
		out, err := run(values, "")
		if err != nil {
			t.Fatalf("relative config link refused: %v\n%s", err, out)
		}
	})
	t.Run("plain-directory", func(t *testing.T) {
		must(t, os.Remove(cellConfig))
		must(t, os.Mkdir(cellConfig, 0700))
		out, err := run(values, claude)
		if err == nil || !strings.Contains(out, "MISS  config home linked") {
			t.Fatalf("plain directory not refused: %v\n%s", err, out)
		}
		must(t, os.Remove(cellConfig))
		must(t, os.Symlink(cfg, cellConfig))
	})
	t.Run("policy-settings", func(t *testing.T) {
		must(t, os.WriteFile(filepath.Join(claude, "settings.json"), []byte(`{"modelOverrides":{"opus":"x"}}`), 0600))
		out, err := run(values, "")
		if err == nil || !strings.Contains(out, "modelOverrides must be removed") {
			t.Fatalf("operator policy conflict missed: %v\n%s", err, out)
		}
	})
}

func TestCodexConfigLaunchOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable is a Unix shell script")
	}
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	must(t, os.WriteFile(filepath.Join(f.binDir, "codex"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0700))
	positional := t.TempDir()
	r := f.run(t, []string{"CELLCTL_DESKWT=0", "PATH=" + f.binDir + ":/usr/bin:/bin"}, "desk", "example", "intake-desk", "--cadence", "off", positional)
	if r.code != 0 {
		t.Fatalf("launch: %+v", r)
	}
	if !strings.Contains(r.stdout, "shell_environment_policy.set.CLAUDE_CONFIG_DIR=\""+positional+"\"") {
		t.Fatalf("positional config lost at command boundary:\n%s", r.stdout)
	}
}
