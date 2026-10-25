package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func provisionScrubbedToken(t *testing.T, c *Cell) string {
	t.Helper()
	dir := c.Dir
	for _, part := range []string{"home", ".config", "assay", "providers"} {
		dir = filepath.Join(dir, part)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := c.scrubbedTokenPath("kimi")
	if err := os.WriteFile(path, []byte("cell-only-fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newScrubbedPolicyFixture(t *testing.T) *policyFixture {
	t.Helper()
	// This fixture must survive a module-only mutation snapshot. It needs no
	// model-policy example from a sibling directory, and must never skip.
	root := t.TempDir()
	f := &policyFixture{cellsRoot: filepath.Join(root, "cells"), cfgDir: filepath.Join(root, "claude-config"), binDir: filepath.Join(root, "bin"), repoDir: filepath.Join(root, "repo")}
	f.cellDir = filepath.Join(f.cellsRoot, "example")
	for _, dir := range []string{f.cellDir, f.cfgDir, f.binDir, filepath.Join(f.cellDir, "home", ".config", "assay")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.cellDir, "home", ".config", "assay", "roster.env"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte("#!/bin/sh\necho '2.1.278 (stub)'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestScrubbedProviderNeverFallsBackToShell(t *testing.T) {
	c := scrubbedFixture(t)
	c.Env.Put("KIMI_API_KEY", "host-canary")
	c.Env.putFrom("CELL_PROVIDER_KIMI_BASE_URL", "https://host-redirect.invalid", layerProcess)
	if _, err := c.providerCredential("kimi"); err == nil {
		t.Fatal("missing cell credential accepted host token")
	}
	provisionScrubbedToken(t, c)
	p, err := c.providerCredential("kimi")
	if err != nil || p.TokenVal != "cell-only-fixture" {
		t.Fatalf("cell credential: %v", err)
	}
	if p.BaseURL != "https://api.kimi.com/coding" {
		t.Fatal("host redirected cell credential")
	}
	env := scrubbedProviderEnv(c.scrubbedComposeEnv("the-desk", "claude", "s").Pairs, p, "k3[1m]")
	if envValue(env, "ANTHROPIC_AUTH_TOKEN") != "cell-only-fixture" {
		t.Fatal("cell token not delivered")
	}
	for _, key := range []string{"GH_TOKEN", "SSH_AUTH_SOCK", "KIMI_API_KEY", "ANTHROPIC_API_KEY", "CODEX_HOME"} {
		if _, ok := envLookup(env, key); ok {
			t.Fatalf("unexpected inherited %s", key)
		}
	}
	if envValue(env, "HOME") != c.Home || envValue(env, "KUBECONFIG") != "/dev/null" {
		t.Fatal("isolation lost")
	}
	for _, tier := range []string{"OPUS", "SONNET", "HAIKU"} {
		if envValue(env, "ANTHROPIC_DEFAULT_"+tier+"_MODEL") != "k3[1m]" {
			t.Fatal("provider alias missing")
		}
	}
	plan := captureStdout(t, func() {
		c.printPlan(c.scrubbedComposeEnv("the-desk", "claude", "s"), "/wt", c.scrubbedLaunchArgv("the-desk", "claude", "k3[1m]", "s", "/wt", "kimi"))
	})
	if strings.Contains(plan, "cell-only-fixture") || strings.Contains(plan, "host-canary") || !strings.Contains(plan, "scrubbed-run") {
		t.Fatal("unsafe launch plan")
	}
}

func TestScrubbedProviderCustodyRefusals(t *testing.T) {
	for _, kind := range []string{"mode", "empty", "multiline", "oversize", "file-link", "directory-link", "name", "endpoint"} {
		t.Run(kind, func(t *testing.T) {
			c := scrubbedFixture(t)
			path := provisionScrubbedToken(t, c)
			name := "kimi"
			switch kind {
			case "mode":
				os.Chmod(path, 0644)
			case "empty":
				os.WriteFile(path, nil, 0600)
			case "multiline":
				os.WriteFile(path, []byte("one\ntwo"), 0600)
			case "oversize":
				os.WriteFile(path, []byte(strings.Repeat("x", 16385)), 0600)
			case "file-link":
				os.Rename(path, path+".real")
				if err := os.Symlink(filepath.Base(path)+".real", path); err != nil {
					t.Fatal(err)
				}
			case "directory-link":
				d := filepath.Dir(path)
				os.Rename(d, d+".real")
				if err := os.Symlink(filepath.Base(d)+".real", d); err != nil {
					t.Fatal(err)
				}
			case "name":
				name = "../kimi"
			case "endpoint":
				c.Env.putFrom("CELL_PROVIDER_KIMI_BASE_URL", "http://provider.invalid", layerCellEnv)
			}
			if _, err := c.providerCredential(name); err == nil {
				t.Fatalf("accepted %s", kind)
			}
		})
	}
}

func TestScrubbedKimiBinarySmokeAndRuntime(t *testing.T) {
	f := newScrubbedPolicyFixture(t)
	if err := os.WriteFile(filepath.Join(f.binDir, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	c := &Cell{Dir: f.cellDir, Home: filepath.Join(f.cellDir, "home")}
	provisionScrubbedToken(t, c)
	env := "CELL=example\nCELL_KIND=scrubbed\nCELL_HARNESS=claude\nCELL_PROVIDER=kimi\nCELL_REPO=" + f.repoDir + "\nCELL_REPO_SLUG=example-org/example-repo\nROLES=the-desk\n"
	if err := os.MkdirAll(f.repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	init := exec.Command("git", "init", "--quiet", f.repoDir)
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("fixture init: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(env), 0600); err != nil {
		t.Fatal(err)
	}
	// The stub asserts custody in the ACTUAL child process. It never contacts a model.
	script := `#!/bin/sh
test "$ANTHROPIC_AUTH_TOKEN" = cell-only-fixture || exit 20
test "$ANTHROPIC_BASE_URL" = https://api.kimi.com/coding || exit 21
test "$ANTHROPIC_MODEL" = 'k3[1m]' || exit 22
test "$KUBECONFIG" = /dev/null || exit 23
test -z "$GH_TOKEN$KIMI_API_KEY$ANTHROPIC_API_KEY$SSH_AUTH_SOCK" || exit 24
case "$HOME" in */example/home) ;; *) exit 25;; esac
echo READY
`
	if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	r := f.run(t, []string{"GH_TOKEN=host-canary", "ANTHROPIC_API_KEY=host-native", "SSH_AUTH_SOCK=/host/agent"}, "smoke", "example")
	if r.code != 0 || !strings.HasSuffix(r.stdout, "READY\n") {
		t.Fatalf("smoke: %+v", r)
	}
	if !strings.Contains(r.stdout, "base_url=https://api.kimi.com/coding") {
		t.Fatal("smoke did not name its network target before contact")
	}
	wt := filepath.Join(f.cellDir, "worktrees", "the-desk")
	if err := os.MkdirAll(wt, 0700); err != nil {
		t.Fatal(err)
	}
	r = f.run(t, []string{"GH_TOKEN=host-canary"}, "--cells-root", f.cellsRoot, "scrubbed-run", "example", "the-desk", "claude", "k3[1m]", "s", wt, "kimi")
	if r.code != 0 || !strings.Contains(r.stdout, "READY") {
		t.Fatalf("runtime: %+v", r)
	}
	// Compare the actual provider child with the same validated metadata a direct
	// scrubbed launch carries. No ambient prefix-wide copying is permitted.
	value := func(s string) perfrec.Value { return perfrec.Value{Value: s, Present: true} }
	context := perfrec.ResolveContext(perfrec.ContextInput{
		Profile: value("workflow"), SessionID: value(strings.Repeat("a", 32)), AgentID: value(strings.Repeat("b", 32)),
		ParentAgentID: value(strings.Repeat("c", 32)), ParentInvocationID: value(strings.Repeat("d", 32)), Role: value("cellctl"),
		LookupEnv: func(k string) (string, bool) {
			if k == "ASSAY_TELEMETRY" {
				return "1", true
			}
			return "", false
		},
	})
	direct := deskkit.PerfContextEnv([]string{"ASSAY_TELEMETRY=1"}, context, false)
	checks := ""
	for _, pair := range direct {
		key, val, _ := strings.Cut(pair, "=")
		checks += "test \"$" + key + "\" = " + cockpitQuote(val) + " || exit 26\n"
	}
	checks += "test -z \"$ASSAY_PERF_UNRECOGNIZED\" || exit 27\n"
	perfScript := strings.Replace(script, "echo READY", checks+"echo READY", 1)
	if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(perfScript), 0700); err != nil {
		t.Fatal(err)
	}
	r = f.run(t, append(direct, "ASSAY_PERF_UNRECOGNIZED=host-canary"), "scrubbed-run", "example", "the-desk", "claude", "k3[1m]", "s", wt, "kimi")
	if r.code != 0 || !strings.Contains(r.stdout, "READY") {
		t.Fatalf("provider perf context differs from direct launch: %+v", r)
	}
	if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	r = f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "the-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "scrubbed-run") || !strings.Contains(r.stdout, "token-file=") {
		t.Fatalf("plan: %+v", r)
	}
	if strings.Contains(r.stdout+r.stderr, "cell-only-fixture") {
		t.Fatal("token in diagnostics")
	}
	// No fallback to the host preset variable when the private token is removed.
	if err := os.Remove(c.scrubbedTokenPath("kimi")); err != nil {
		t.Fatal(err)
	}
	r = f.run(t, nil, "smoke", "example")
	if r.code == 0 {
		t.Fatal("missing token accepted host credential")
	}
}

func TestScrubbedCockpitPreservesPrivateSession(t *testing.T) {
	c := scrubbedFixture(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if got := c.scrubbedCockpit(""); got.Cockpit != "tmux" || got.Err != "" {
		t.Fatalf("auto: %+v", got)
	}
	if got := c.scrubbedCockpit("orca"); got.Err == "" {
		t.Fatal("unsupported cockpit silently accepted")
	}
	env := scrubbedDisplayEnv(c.scrubbedComposeEnv("the-desk", "claude", "s"), "herdr")
	plan := captureStdout(t, func() { c.printPlan(env, "/wt", []string{"claude"}) })
	if envValue(env.Pairs, envAssayCockpit) != "tmux" || !strings.Contains(plan, "[plan] env ASSAY_COCKPIT=tmux") {
		t.Fatal("plan and runtime disagree about child worktree handling")
	}
	// A surface is explicit: installing Herdr never changes existing scrubbed cells.
	if err := os.WriteFile(filepath.Join(bin, "herdr"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if got := c.scrubbedCockpit(""); got.Cockpit != "tmux" || got.Err != "" {
		t.Fatalf("auto changed after Herdr installation: %+v", got)
	}
	if got := c.scrubbedCockpit("herdr"); got.Cockpit != "herdr" || got.Err != "" {
		t.Fatalf("Herdr: %+v", got)
	}
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Fatal(err)
	}
}

func TestScrubbedAutoDoesNotProbeOrca(t *testing.T) {
	c := scrubbedFixture(t)
	bin := t.TempDir()
	marker := filepath.Join(bin, "probed")
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "orca"), []byte("#!/bin/sh\necho probed > "+cockpitQuote(marker)+"\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	if got := c.scrubbedCockpit(""); got.Cockpit != "tmux" || got.Err != "" {
		t.Fatalf("auto: %+v", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("scrubbed auto probed orca", err)
	}
}

func TestScrubbedCheckDoesNotProbeHostCockpits(t *testing.T) {
	f := newScrubbedPolicyFixture(t)
	if err := os.WriteFile(filepath.Join(f.binDir, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "--quiet", f.repoDir).CombinedOutput(); err != nil {
		t.Fatalf("fixture init: %v %s", err, out)
	}
	config := "CELL=example\nCELL_KIND=scrubbed\nCELL_HARNESS=claude\nCELL_REPO=" + f.repoDir + "\nCELL_REPO_SLUG=example-org/example-repo\nROLES=the-desk\n"
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(f.cellDir, "cockpit-probed")
	stub := []byte("#!/bin/sh\necho probed > " + cockpitQuote(marker) + "\nexit 0\n")
	for _, name := range []string{"herdr", "orca"} {
		if err := os.WriteFile(filepath.Join(f.binDir, name), stub, 0700); err != nil {
			t.Fatal(err)
		}
	}
	r := f.run(t, nil, "check", "example")
	if !strings.Contains(r.stdout, "cockpit: tmux") {
		t.Fatalf("check did not resolve scrubbed cockpit: %+v", r)
	}
	if strings.Contains(r.stderr, "probing orca") {
		t.Fatal("check probed host cockpit")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("check contacted host cockpit", err)
	}
}

func TestScrubbedProviderLaunchKeepsTokenOutOfTmuxAndRetainsLock(t *testing.T) {
	c := scrubbedFixture(t)
	provisionScrubbedToken(t, c)
	bin := t.TempDir()
	log := filepath.Join(bin, "tmux-calls")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\ncase \"$3\" in has-session) exit 1;; list-panes) echo %d;; esac\n", log, os.Getpid())
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	launch := func() {
		c.scrubbedDeskLaunchWithOptions("the-desk", "claude", "k3[1m]", "s", filepath.Join(c.Dir, "worktrees", "the-desk"), "kimi", cockpitResolution{Cockpit: "tmux"})
	}
	launch()
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "scrubbed-run") || !strings.Contains(text, "env -i") || !strings.Contains(text, "KUBECONFIG=/dev/null") {
		t.Fatal("launch lost composed runtime", text)
	}
	if strings.Contains(text, "cell-only-fixture") {
		t.Fatal("token in tmux argv")
	}
	assertDies(t, "second session owner", launch)
	raw, _ = os.ReadFile(log)
	if strings.Count(string(raw), "new-session") != 1 {
		t.Fatal("second harness launched")
	}
}
