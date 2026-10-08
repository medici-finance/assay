package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The fixture world shared by the CLI contract tests (desktools-v2/16). Every world is built
// from files only: DRY_RUN=1 never fetches, never opens a worktree and never contacts a forge,
// a model endpoint or a cluster, and a stub harness answers --version and nothing else.

// cliWorld is one disposable cells registry with a "house" cell named example, a
// "scrubbed" cell named scrub, a stub claude and a stub codex on PATH.
type cliWorld struct {
	root      string
	cellsRoot string
	cellDir   string
	cfgDir    string
	binDir    string
	repoDir   string
	policy    string
}

const cliWorldHouseEnv = "CELL=example\nCELL_KIND=house\nCELL_ROOTS=example-org/example-repo=%REPO%\nCELL_REPO=%REPO%\n"

// newCLIWorld builds the world. extraEnv is appended to the house cell's cell.env.
func newCLIWorld(t *testing.T, extraEnv string) *cliWorld {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := &cliWorld{
		root:      root,
		cellsRoot: filepath.Join(root, "cells"),
		cfgDir:    filepath.Join(root, "claude-config"),
		binDir:    filepath.Join(root, "bin"),
		repoDir:   filepath.Join(root, "repo"),
	}
	w.cellDir = filepath.Join(w.cellsRoot, "example")
	scrubDir := filepath.Join(w.cellsRoot, "scrub")
	for _, d := range []string{
		filepath.Join(w.cellDir, "home", ".config", "assay"), filepath.Join(w.cellDir, ".config", "assay"), w.cfgDir, w.binDir, w.repoDir,
		filepath.Join(scrubDir, "home", ".config", "assay"), filepath.Join(scrubDir, "worktrees"),
		filepath.Join(scrubDir, "tmp"), filepath.Join(scrubDir, "run"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{w.cellDir, scrubDir} {
		if err := os.WriteFile(filepath.Join(d, "home", ".config", "assay", "roster.env"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gi := exec.Command("git", "init", "-q", w.repoDir)
	gi.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	if out, err := gi.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", w.repoDir, err, out)
	}
	w.writeHouseEnv(t, extraEnv)
	scrub := "CELL=scrub\nCELL_KIND=scrubbed\nCELL_REPO=" + w.repoDir + "\nCELL_REPO_SLUG=example-org/example-repo\n" +
		"CELL_ROOTS=example-org/example-repo=" + w.repoDir + "\nROLES=\"the-desk worker-desk\"\nCELL_HARNESS=claude\n" +
		"DESK_MODEL_DEFAULT=sonnet\nDESK_MODEL_the_desk=fable\nDESKD=0\n"
	if err := os.WriteFile(filepath.Join(scrubDir, "cell.env"), []byte(scrub), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"claude", "codex"} {
		script := "#!/bin/sh\ncase \"$1\" in\n--version) echo '2.1.278 (stub)'; exit 0;;\nesac\necho \"[stub " + h + "] $*\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(w.binDir, h), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w.policy = filepath.Join(w.cellDir, "model-policy.json")
	return w
}

func (w *cliWorld) writeHouseEnv(t *testing.T, extra string) {
	t.Helper()
	body := strings.ReplaceAll(cliWorldHouseEnv, "%REPO%", w.repoDir) + extra
	if err := os.WriteFile(filepath.Join(w.cellDir, "cell.env"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// installPolicy copies the shipped example policy into the house cell and selects it.
func (w *cliWorld) installPolicy(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile(examplePolicyPath)
	if err != nil {
		t.Skipf("example policy not readable from this checkout (%v)", err)
	}
	if err := os.WriteFile(w.policy, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	env, err := os.ReadFile(filepath.Join(w.cellDir, "cell.env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.cellDir, "cell.env"), append(env, []byte("CELL_MODEL_POLICY=model-policy.json\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// baseEnv is the complete process environment of every fixture run: nothing is inherited, so
// the host's own cells, credentials and cluster configuration cannot reach the binary.
func (w *cliWorld) baseEnv() []string {
	return []string{
		"CELLS_ROOT=" + w.cellsRoot,
		"CLAUDE_CONFIG_DIR=" + w.cfgDir,
		"PATH=" + w.binDir + ":" + os.Getenv("PATH"),
		"HOME=" + w.cellDir,
		"KUBECONFIG=/dev/null",
		"ZAI_API_KEY=fixture-zai",
		"KIMI_API_KEY=fixture-kimi",
	}
}

// cliRun is one finished invocation.
type cliRun struct {
	Args   []string `json:"args"`
	Code   int      `json:"code"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
}

// runBin runs bin with the world's base environment plus env (later entries win, as in exec).
func (w *cliWorld) runBin(t *testing.T, bin string, env []string, args ...string) cliRun {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(w.baseEnv(), env...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running cellctl %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return cliRun{Args: args, Code: code, Stdout: out.String(), Stderr: errb.String()}
}

func (w *cliWorld) run(t *testing.T, env []string, args ...string) cliRun {
	t.Helper()
	return w.runBin(t, cellctlBinary(t), env, args...)
}

var stampRE = regexp.MustCompile(`\d{8}T\d{6}Z`)

// binPathRE matches the quoted path of the binary under test (a dry-run plan re-invokes
// itself) and buildStampRE its version stamp, which differs between the baseline and a rebuild.
var (
	binPathRE    = regexp.MustCompile(`'[^'\n]*/cellctl(-base)?'`)
	buildStampRE = regexp.MustCompile(`dev-[0-9a-f]{12}(-dirty)?`)
)

// volatileRE matches the cache report's clock reading, its world-derived domain digest and the
// host's free-space figure: three values that differ between two runs of the same case.
var volatileRE = regexp.MustCompile(`"(at|domain)":"[^"]*"|"available_bytes":\d+`)

// norm replaces everything that legitimately differs between two runs of the same case: the
// world's directory and a UTC boot stamp.
func (w *cliWorld) norm(s string) string {
	s = strings.ReplaceAll(s, w.root, "<ROOT>")
	s = binPathRE.ReplaceAllString(s, "'<BIN>'")
	s = buildStampRE.ReplaceAllString(s, "dev-<BUILD>")
	s = volatileRE.ReplaceAllString(s, `"volatile":"<V>"`)
	return stampRE.ReplaceAllString(s, "<STAMP>")
}
