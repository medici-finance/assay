package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
)

// A launch hands a cell to a later cellctl process in several places: the model-policy hook, the
// native container console, the comms and role pane commands, a scratch run's environment. Each
// must name the cells root the launch itself loaded the cell from. For a cell one level under
// the root that is the cell directory's parent; for a cell named `team/demo` it is not, and a
// re-entry that took the parent would read `<root>/team/defaults.env` in place of the machine's
// file — a different environment from the one the window launched on.

// loadAgain loads a cell the way a re-entered cellctl does: CELLS_ROOT from the command it was
// handed, the cell by the name it was handed. A load that refuses is reported, not a panic.
func loadAgain(t *testing.T, what, root, name string) *Cell {
	t.Helper()
	t.Setenv("CELLS_ROOT", root)
	var c *Cell
	prev := os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = devnull
	func() {
		defer func() {
			os.Stderr = prev
			devnull.Close()
			if r := recover(); r != nil {
				if _, ok := r.(exitCode); !ok {
					panic(r)
				}
				t.Errorf("%s: re-entering with CELLS_ROOT=%s and cell %q does not load the cell", what, root, name)
			}
		}()
		c = loadCell(name)
	}()
	return c
}

// TestNestedCellReentryNamesTheLaunchRoot: every re-entry command a nested cell builds names the
// cells root it was loaded from, and loading the cell again from that command reads the same
// defaults file — never the decoy one directory up from the cell. Both spellings of a nested
// cell are covered: the one `cellctl new team/demo` writes (CELL=team/demo) and a hand-made
// cell.env whose CELL= is the last path element only.
func TestNestedCellReentryNamesTheLaunchRoot(t *testing.T) {
	for _, cellName := range []string{"team/demo", "demo"} {
		t.Run("CELL="+cellName, func(t *testing.T) {
			root := defaultsRoot(t)
			rootFile := writeDefaults(t, root, "CELL_HARNESS=codex\n")
			writeCell(t, root, filepath.Join("team", "demo"), strings.Replace(demoCell, "CELL=demo", "CELL="+cellName, 1))
			if err := os.WriteFile(filepath.Join(root, "team", cellDefaultsFile), []byte("CELL_HARNESS=cursor\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			c := loadCell("team/demo")
			if c.DefaultsPath != rootFile || c.Harness != "codex" {
				t.Fatalf("the launch read %q (harness %s), want %q", c.DefaultsPath, c.Harness, rootFile)
			}
			same := func(what, gotRoot, gotName string) {
				t.Helper()
				again := loadAgain(t, what, gotRoot, gotName)
				t.Setenv("CELLS_ROOT", root)
				if again == nil {
					return
				}
				if again.Dir != c.Dir || again.DefaultsPath != rootFile || again.Harness != "codex" {
					t.Errorf("%s re-enters with CELLS_ROOT=%s and cell %q, which loads %s reading %q (harness %s); the launch loaded %s reading %q (harness codex)",
						what, gotRoot, gotName, again.Dir, again.DefaultsPath, again.Harness, c.Dir, rootFile)
				}
			}

			// The native container console: `env CELLS_ROOT=<root> cellctl container-run <cell> …`.
			argv := c.nativeConsoleArgv(&cellcontainer.Plan{Harness: "codex", Model: "example-model"}, "the-desk")
			if len(argv) < 5 || argv[0] != "env" || !strings.HasPrefix(argv[1], "CELLS_ROOT=") || argv[3] != "container-run" {
				t.Fatalf("container console argv: %q", argv)
			}
			same("the container console", strings.TrimPrefix(argv[1], "CELLS_ROOT="), argv[4])

			// The comms pane and the cadence-supervised role pane: `cellctl --cells-root <root> …`.
			wantRoot, wantName := "--cells-root "+shellPOSIX.quote(root), shellPOSIX.quote(filepath.Join("team", "demo"))
			if line := c.commsCmdIn(shellPOSIX, "/opt/example/cellctl"); !strings.Contains(line, wantRoot+" comms "+wantName+" run") {
				t.Errorf("comms pane command does not name the launch root and the cell under it: %s", line)
			}
			c.Cadence = &cadenceOptions{Interval: 5 * time.Minute, Budget: 10 * time.Minute}
			if line := c.roleCmdIn(shellPOSIX, "/opt/example/cellctl", "worker-desk", "", upOverrides{}); !strings.Contains(line, wantRoot+" desk "+wantName+" 'worker-desk'") {
				t.Errorf("role pane command does not name the launch root and the cell under it: %s", line)
			}
			// What the scratch environment carries, and what every site above is built from.
			r, n := c.reenter()
			same("reenter", r, n)
		})
	}
}

// nestedPolicyCell adds a house cell named team/demo to the fixture's cells root. Its cell.env
// names no model policy: the policy comes from the cells root's defaults.env alone, and a decoy
// defaults.env beside the cell's parent directory names none.
func nestedPolicyCell(t *testing.T, f *policyFixture) (cellDir, policy string) {
	t.Helper()
	cellDir = filepath.Join(f.cellsRoot, "team", "demo")
	cfg := filepath.Join(cellDir, "home", ".config", "assay")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "roster.env"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := "CELL=team/demo\nCELL_KIND=house\n" +
		"CELL_ROOTS=example-org/example-repo=" + f.repoDir + "\n" +
		"CELL_REPO=" + f.repoDir + "\n"
	if err := os.WriteFile(filepath.Join(cellDir, "cell.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	// The policy lives outside every cell directory, named by an absolute path.
	policy = filepath.Join(filepath.Dir(f.cellsRoot), "machine-policy.json")
	src, err := os.Open(f.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.cellsRoot, cellDefaultsFile), []byte("CELL_MODEL_POLICY="+policy+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.cellsRoot, "team", cellDefaultsFile), []byte("CELL_HARNESS=claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return cellDir, policy
}

// TestBinaryNestedCellHookReadsTheLaunchDefaults drives the whole path for a cell named
// team/demo whose model policy is named ONLY by the cells root's defaults.env: a real launch
// installs the hook, and the hook command that launch wrote — run as Claude Code runs it, with
// no CELLS_ROOT in its environment — must find the same policy and so allow what the policy
// allows and refuse what it refuses. A hook that took the cell directory's parent as the cells
// root reads the decoy file, finds no policy, and blocks every model: the window cannot be used.
func TestBinaryNestedCellHookReadsTheLaunchDefaults(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	recordingClaude(t, f)
	cellDir, policy := nestedPolicyCell(t, f)

	r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "team/demo", "pr-review-desk")
	if r.code != 0 {
		t.Fatalf("launch: exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	settings := ""
	argv := launchArgv(r.stdout)
	for i, a := range argv {
		if a == "--settings" && i+1 < len(argv) {
			settings = argv[i+1]
		}
	}
	command := launchHookCommand(t, settings)
	if want := "--cells-root " + bashQuote(f.cellsRoot) + " "; !strings.Contains(command, want) {
		t.Fatalf("the hook command does not name the cells root the launch used (%s):\n%s", want, command)
	}
	if !strings.Contains(command, bashQuote(cellDir)) {
		t.Fatalf("the hook command does not name the cell directory:\n%s", command)
	}

	if code, stderr := runHook(t, f, command, agentEvent("opus")); code != 0 {
		t.Errorf("a child the machine-wide policy allows: exit %d, want 0; stderr: %s", code, stderr)
	}
	if code, stderr := runHook(t, f, command, agentEvent("unknown-model")); code != wantBlock || !strings.Contains(stderr, "model-policy:") {
		t.Errorf("a child the machine-wide policy refuses: exit %d, want %d; stderr: %s", code, wantBlock, stderr)
	}
	// The root is the one on the command line, whatever the window's environment carries.
	if code, stderr := runHookEnv(t, f, command, agentEvent("opus"), "CELLS_ROOT="+filepath.Join(f.cellsRoot, "team")); code != 0 {
		t.Errorf("with another CELLS_ROOT in the hook's environment: exit %d, want 0; stderr: %s", code, stderr)
	}

	// A command that names a root the cell is not under is refused, not re-rooted.
	sha := policySHA(t, policy)
	hook := func(pre []string, dir string) string {
		parts := append([]string{cellctlBinary(t)}, pre...)
		parts = append(parts, "model-policy", "hook", dir, "pr-review-desk", "anthropic", "", "claude", sha)
		for i, p := range parts {
			parts[i] = bashQuote(p)
		}
		return strings.Join(parts, " ")
	}
	elsewhere := t.TempDir()
	if code, stderr := runHook(t, f, hook([]string{"--cells-root", elsewhere}, cellDir), agentEvent("opus")); code != wantBlock || !strings.Contains(stderr, "is not under the cells root") {
		t.Errorf("a cell directory outside the named root: exit %d, want %d; stderr: %s", code, wantBlock, stderr)
	}
	if code, stderr := runHook(t, f, hook([]string{"--cells-root", cellDir}, cellDir), agentEvent("opus")); code != wantBlock {
		t.Errorf("the cell directory named as its own root: exit %d, want %d; stderr: %s", code, wantBlock, stderr)
	}
	// The same explicit command with the right root is the control for the two refusals above.
	if code, stderr := runHook(t, f, hook([]string{"--cells-root", f.cellsRoot}, cellDir), agentEvent("opus")); code != 0 {
		t.Errorf("the hand-built command with the launch root: exit %d, want 0; stderr: %s", code, stderr)
	}
	// A command with no --cells-root at all (settings an older cellctl wrote) can only take the
	// parent directory. For this cell that finds no policy, and the hook blocks: it never allows
	// a model unchecked.
	if code, stderr := runHook(t, f, hook(nil, cellDir), agentEvent("opus")); code != wantBlock {
		t.Errorf("a flagless hook command for a nested cell: exit %d, want %d (blocking); stderr: %s", code, wantBlock, stderr)
	}
	// And for a cell one level down, the flagless form still resolves the root's file.
	if code, stderr := runHook(t, f, hook(nil, f.cellDir), agentEvent("opus")); code != 0 {
		t.Errorf("a flagless hook command for a top-level cell: exit %d, want 0; stderr: %s", code, stderr)
	}
}

// launchHookCommand is the PreToolUse hook command in a launch's --settings JSON.
func launchHookCommand(t *testing.T, settings string) string {
	t.Helper()
	var s launchSettings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		t.Fatalf("--settings is not JSON: %v\n%s", err, settings)
	}
	pre := s.Hooks["PreToolUse"]
	if len(pre) == 0 || len(pre[0].Hooks) == 0 {
		t.Fatalf("no PreToolUse hook in the launch's --settings: %s", settings)
	}
	return pre[0].Hooks[0].Command
}
