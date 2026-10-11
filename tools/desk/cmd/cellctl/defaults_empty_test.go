package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// An assignment in the machine-wide defaults file whose value is empty sets nothing: the layer
// below it stands. The file outranks the process environment, so a line that set a key to the
// empty string would take a value the launching shell exported away from every cell under the
// root — and for the three keys that locate model policy, "empty" reads as "no policy named".
// The template prints `# KEY=` for every key with no compiled default, so uncommenting one of
// those lines unchanged is exactly that line.

// emptySpellings is every way the cell.env grammar spells a value of no bytes, and the one way a
// file saved with CRLF line endings spells it.
var emptySpellings = []struct{ name, rhs string }{
	{"nothing after the =", ""},
	{"empty double quotes", `""`},
	{"empty single quotes", `''`},
	{"a reference to a variable nothing sets", "$CELLCTL_TEST_NOT_SET"},
	{"the same reference, braced and quoted", `"${CELLCTL_TEST_NOT_SET}"`},
	{"only the carriage return of a CRLF line ending", "\r"},
}

// tryLoadCell loads a cell and returns it, or nil and what the refusal printed: a load that
// refuses is something for the caller to report, not a panic that ends the test binary.
func tryLoadCell(t *testing.T, name string) (c *Cell, refused string) {
	t.Helper()
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = wr
	done := make(chan string, 1)
	go func() { b, _ := io.ReadAll(rd); done <- string(b) }()
	func() {
		defer func() {
			os.Stderr = prev
			wr.Close()
			if r := recover(); r != nil {
				if _, ok := r.(exitCode); !ok {
					panic(r)
				}
				c = nil
			}
		}()
		c = loadCell(name)
	}()
	refused = <-done
	rd.Close()
	return c, refused
}

// TestCellDefaultsEmptyValueSetsNothing holds the rule at the reader and through loadCell, for a
// key cellctl reads, a key that locates model policy, and a key cellctl does not read at all.
func TestCellDefaultsEmptyValueSetsNothing(t *testing.T) {
	keys := map[string]string{
		"CELL_MODEL_POLICY":       "/example/policy.json",
		"CELL_PROVIDER_DEFAULTS":  "/example/team.json",
		"CELL_PROVIDER_OVERRIDES": "local.json",
		"CELL_GO_CACHE":           "on",
		"CELL_HARNESS":            "codex",
		"CELLCTL_TEST_PROBE":      "from the shell",
	}
	for key, shell := range keys {
		for _, sp := range emptySpellings {
			for _, prefix := range []string{"", "export "} {
				t.Run(key+"/"+sp.name+"/"+prefix, func(t *testing.T) {
					line := prefix + key + "=" + sp.rhs + "\n"
					for _, goos := range []string{"linux", "windows"} {
						// Under a value the process environment holds.
						e := emptyEnv()
						e.src = map[string]string{}
						e.putFrom(key, shell, layerProcess)
						got, err := overlayCellDefaultsText(goos, e, []byte("CELL_COCKPIT=tmux\n"+line), "defaults.env")
						if err != nil {
							t.Fatalf("%s: %q is refused: %v", goos, line, err)
						}
						if !reflect.DeepEqual(got, []string{"CELL_COCKPIT"}) {
							t.Errorf("%s: the file is reported to set %v, want CELL_COCKPIT only: a line with an empty value sets nothing", goos, got)
						}
						if e.Get(key) != shell || e.Source(key) != layerProcess {
							t.Errorf("%s: %q left %s=%q from %s, want the environment's %q", goos, line, key, e.Get(key), e.Source(key), shell)
						}
						// And with nothing below it: the key stays unset.
						e = emptyEnv()
						if _, err := overlayCellDefaultsText(goos, e, []byte(line), "defaults.env"); err != nil {
							t.Fatal(err)
						}
						if e.IsSet(key) || e.Source(key) != layerUnset {
							t.Errorf("%s: %q set %s (source %s); an empty value sets nothing", goos, line, key, e.Source(key))
						}
					}

					root := defaultsRoot(t)
					t.Setenv("CELLCTL_TEST_NOT_SET", "")
					os.Unsetenv("CELLCTL_TEST_NOT_SET")
					t.Setenv(key, shell)
					writeDefaults(t, root, "CELL_COCKPIT=tmux\n"+line)
					writeCell(t, root, "demo", demoCell)
					c, refused := tryLoadCell(t, "demo")
					if c == nil {
						t.Fatalf("the cell does not load under %q: %s", line, refused)
					}
					if c.Env.Get(key) != shell || c.Env.Source(key) != layerProcess {
						t.Errorf("the cell loads %s=%q from %s, want the environment's %q", key, c.Env.Get(key), c.Env.Source(key), shell)
					}
					if !reflect.DeepEqual(c.DefaultsKeys, []string{"CELL_COCKPIT"}) {
						t.Errorf("the cell reports the file set %v, want CELL_COCKPIT only", c.DefaultsKeys)
					}
				})
			}
		}
	}
}

// TestCellDefaultsEmptyValueEdges: what the rule does not reach. A value that is not empty still
// sets, however little it holds; a later empty line does not undo an earlier line; the strict
// reading comes first, so an empty value excuses neither a refused key nor a malformed line; and
// cell.env keeps the reading it always had, where an empty value IS an assignment.
func TestCellDefaultsEmptyValueEdges(t *testing.T) {
	overlay := func(body string) (*Env, []string, error) {
		e := emptyEnv()
		keys, err := overlayCellDefaultsText("linux", e, []byte(body), "defaults.env")
		return e, keys, err
	}
	if e, keys, err := overlay("CELLCTL_TEST_PROBE=\" \"\n"); err != nil || !reflect.DeepEqual(keys, []string{"CELLCTL_TEST_PROBE"}) || e.Get("CELLCTL_TEST_PROBE") != " " {
		t.Errorf("a value of one space is not empty and is assigned: keys=%v value=%q err=%v", keys, e.Get("CELLCTL_TEST_PROBE"), err)
	}
	if e, keys, err := overlay("CELL_HARNESS=codex\nCELL_HARNESS=\n"); err != nil || !reflect.DeepEqual(keys, []string{"CELL_HARNESS"}) || e.Get("CELL_HARNESS") != "codex" {
		t.Errorf("an empty line after a line that sets the key leaves the earlier value: keys=%v value=%q err=%v", keys, e.Get("CELL_HARNESS"), err)
	}
	if e, keys, err := overlay("CELL_HARNESS=\nCELL_HARNESS=codex\n"); err != nil || !reflect.DeepEqual(keys, []string{"CELL_HARNESS"}) || e.Get("CELL_HARNESS") != "codex" {
		t.Errorf("a line that sets the key after an empty one still sets it: keys=%v value=%q err=%v", keys, e.Get("CELL_HARNESS"), err)
	}
	// A variable an earlier line of the file set is not empty.
	if e, _, err := overlay("CELLCTL_TEST_PROBE=x\nCELL_HARNESS=$CELLCTL_TEST_PROBE\n"); err != nil || e.Get("CELL_HARNESS") != "x" {
		t.Errorf("a reference to a variable the file set above is assigned: %q %v", e.Get("CELL_HARNESS"), err)
	}
	for _, body := range []string{"CELL=\n", "CELLS_ROOT=\"\"\n", "HOME=''\n", "DRY_RUN=\n"} {
		if _, _, err := overlay(body); err == nil || !strings.Contains(err.Error(), "cannot be a machine-wide default") {
			t.Errorf("%q: a refused key is refused whatever its value, got %v", body, err)
		}
	}
	if _, _, err := overlay("=\n"); err == nil || !strings.Contains(err.Error(), "is not a KEY=VALUE assignment") {
		t.Errorf("a line with no key is malformed whatever follows the =, got %v", err)
	}

	// cell.env: unchanged. An empty value there is an assignment, and it outranks both layers
	// under it — which is how one cell removes a tier-map entry, or blanks a key for itself.
	root := defaultsRoot(t)
	t.Setenv("CELLCTL_TEST_PROBE", "from the shell")
	writeDefaults(t, root, "TIER_MODEL_TOP_CLAUDE=\nTIER_MODEL_MID_CLAUDE=example-mid\n")
	writeCell(t, root, "demo", demoCell)
	writeCell(t, root, "blank", strings.Replace(demoCell, "CELL=demo", "CELL=blank", 1)+"CELLCTL_TEST_PROBE=\nTIER_MODEL_TOP_CLAUDE=\nTIER_MODEL_MID_CLAUDE=\n")
	c, refused := tryLoadCell(t, "demo")
	if c == nil {
		t.Fatalf("the cell does not load: %s", refused)
	}
	if got := c.Env.Get("TIER_MODEL_TOP_CLAUDE"); got != tierModelDefaults["TIER_MODEL_TOP_CLAUDE"] {
		t.Errorf("an empty tier-map line in the defaults file leaves TIER_MODEL_TOP_CLAUDE=%q, want the compiled %q", got, tierModelDefaults["TIER_MODEL_TOP_CLAUDE"])
	}
	if got := c.Env.Get("TIER_MODEL_MID_CLAUDE"); got != "example-mid" || c.Env.Source("TIER_MODEL_MID_CLAUDE") != layerDefaults {
		t.Errorf("a tier-map line with a value in the defaults file gives %q from %s", got, c.Env.Source("TIER_MODEL_MID_CLAUDE"))
	}
	b := loadCell("blank")
	for _, k := range []string{"CELLCTL_TEST_PROBE", "TIER_MODEL_TOP_CLAUDE", "TIER_MODEL_MID_CLAUDE"} {
		if !b.Env.IsSet(k) || b.Env.Get(k) != "" || b.Env.Source(k) != layerCellEnv {
			t.Errorf("cell.env's empty %s line gives %q from %s (set=%v), want the empty string from cell.env", k, b.Env.Get(k), b.Env.Source(k), b.Env.IsSet(k))
		}
	}
}

// templateWith is the template cellctl writes with the commented-out lines for keys uncommented
// and nothing else changed — the edit the template invites.
func templateWith(t *testing.T, keys ...string) string {
	t.Helper()
	tpl := cellDefaultsTemplate()
	for _, k := range keys {
		line := "\n# " + k + "="
		if strings.Count(tpl, line) != 1 {
			t.Fatalf("the template has %d line(s) for %s, want one", strings.Count(tpl, line), k)
		}
		tpl = strings.Replace(tpl, line, "\n"+k+"=", 1)
	}
	return tpl
}

// TestCellDefaultsEmptyLineInShowAndSet: `show` does not credit the defaults file with a value
// an empty line there did not supply, and `set` resolves the cell as a launch does.
func TestCellDefaultsEmptyLineInShowAndSet(t *testing.T) {
	root := defaultsRoot(t)
	for _, k := range []string{"ASSAY_REPAIR_ADMISSION", "CELL_COCKPIT"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("CELL_HARNESS", "codex")
	writeDefaults(t, root, "CELL_HARNESS=\nCELL_COCKPIT=\"\"\nASSAY_REPAIR_ADMISSION=\n")
	envfile := writeCell(t, root, "demo", demoCell)
	out := captureStdout(t, func() { showArgv("demo") })
	if strings.Contains(out, cellDefaultsFile) {
		t.Errorf("show names the defaults file as the source of a value its empty lines did not set:\n%s", out)
	}
	if !strings.Contains(out, "[show] CELL_HARNESS=codex (default)\n") {
		t.Errorf("show should resolve CELL_HARNESS from the environment under an empty defaults line:\n%s", out)
	}
	eff := effectiveCellEnv(envfile, nil)
	for _, k := range []string{"CELL_HARNESS", "CELL_COCKPIT", "ASSAY_REPAIR_ADMISSION"} {
		if eff.IsSet(k) {
			t.Errorf("`set` resolves %s=%q from the files; an empty defaults line sets nothing", k, eff.Get(k))
		}
	}
}

// TestBinaryCellDefaultsEmptyLineKeepsEnvironmentPolicy is the launch itself, on the built
// binary: the three keys that locate model policy are set in the environment cellctl runs in,
// and the defaults file is the template with that one key's line uncommented, value unchanged.
// The policy the environment names must still be the one the cell resolves — for
// CELL_MODEL_POLICY, the launch must still install the enforcement hook.
func TestBinaryCellDefaultsEmptyLineKeepsEnvironmentPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's harness stubs are shell scripts")
	}
	t.Run("CELL_MODEL_POLICY", func(t *testing.T) {
		f := newPolicyFixture(t, "2.1.278")
		f.prepareLocalLaunch(t)
		recordingClaude(t, f)
		f.plainCell(t)
		policyEnv := "CELL_MODEL_POLICY=" + f.policy
		launch := func() (string, runResult) {
			r := f.run(t, []string{"CELLCTL_DESKWT=0", policyEnv}, "desk", "example", "pr-review-desk")
			if r.code != 0 {
				t.Fatalf("launch: exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
			}
			argv := launchArgv(r.stdout)
			for i, a := range argv {
				if a == "--settings" && i+1 < len(argv) {
					return argv[i+1], r
				}
			}
			return "", r
		}
		// Control: with no defaults file the environment's policy installs the hook.
		if settings, r := launch(); !strings.Contains(settings, "model-policy hook") {
			t.Fatalf("control: a launch with the policy in its environment installs no hook:\n%s\n%s", r.stdout, r.stderr)
		}
		if err := os.WriteFile(filepath.Join(f.cellsRoot, cellDefaultsFile), []byte(templateWith(t, "CELL_MODEL_POLICY")), 0o600); err != nil {
			t.Fatal(err)
		}
		settings, r := launch()
		if !strings.Contains(settings, "model-policy hook") {
			t.Fatalf("with the template's CELL_MODEL_POLICY line uncommented unchanged, the launch installs no model-policy hook: the empty line displaced the environment's policy\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
		}
		command := launchHookCommand(t, settings)
		if !strings.Contains(command, policySHA(t, f.policy)) {
			t.Errorf("the hook is not pinned to the policy the environment names:\n%s", command)
		}
		if code, stderr := runHookEnv(t, f, command, agentEvent("opus"), policyEnv); code != 0 {
			t.Errorf("a child the policy allows: exit %d, want 0; stderr: %s", code, stderr)
		}
		if code, stderr := runHookEnv(t, f, command, agentEvent("unknown-model"), policyEnv); code != wantBlock || !strings.Contains(stderr, "model-policy:") {
			t.Errorf("a child the policy refuses: exit %d, want %d; stderr: %s", code, wantBlock, stderr)
		}
		// `check` and `show` agree the file set nothing.
		chk := f.run(t, []string{policyEnv}, "check", "example")
		if !strings.Contains(chk.stdout, "cell defaults: read "+filepath.Join(f.cellsRoot, cellDefaultsFile)+" (it sets no keys;") {
			t.Errorf("check should say the file sets no keys:\n%s", chk.stdout)
		}
	})

	// The shared catalog and a cell's exceptions to it, each named only by the environment.
	for _, key := range []string{"CELL_PROVIDER_DEFAULTS", "CELL_PROVIDER_OVERRIDES"} {
		t.Run(key, func(t *testing.T) {
			f := catalogFixture(t)
			if err := os.Rename(filepath.Join(f.cellsRoot, "providers.json"), filepath.Join(f.cellsRoot, "team.json")); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(f.cellDir, "local.json")
			writeCatalogJSON(t, local, map[string]any{"default_provider": "kimi"})
			env := []string{"DRY_RUN=1", "CELL_PROVIDER_DEFAULTS=team.json", "CELL_PROVIDER_OVERRIDES=local.json"}
			want := "team.json + " + local
			resolve := func() runResult { return f.run(t, env, "desk", "example", "worker-desk") }
			if r := resolve(); r.code != 0 || !strings.Contains(r.stdout, "model=k3-256k") || !strings.Contains(r.stdout, want) {
				t.Fatalf("control: the catalog the environment names is not resolved: %+v", r)
			}
			if err := os.WriteFile(filepath.Join(f.cellsRoot, cellDefaultsFile), []byte(templateWith(t, key)), 0o600); err != nil {
				t.Fatal(err)
			}
			if r := resolve(); r.code != 0 || !strings.Contains(r.stdout, "model=k3-256k") || !strings.Contains(r.stdout, want) {
				t.Errorf("with the template's %s line uncommented unchanged, the cell no longer resolves the catalog the environment names (%s):\nexit %d\nstdout:\n%s\nstderr:\n%s", key, want, r.code, r.stdout, r.stderr)
			}
		})
	}
}

// TestCellDefaultsCreateFailureLeavesNoFile: a template write that fails part-way leaves nothing
// at the path — not half a template, which would be a defaults file that is there (so never
// written again) and possibly one cut off inside a line (so every cell under the root would
// refuse to load). `new` reports it as a notice, `defaults init` as a refusal, and the next
// attempt writes the whole file.
func TestCellDefaultsCreateFailureLeavesNoFile(t *testing.T) {
	root := defaultsRoot(t)
	path := filepath.Join(root, cellDefaultsFile)
	tpl := cellDefaultsTemplate()
	diskFull := errors.New("example: no space left on device")
	real := writeCellDefaultsTemplate
	t.Cleanup(func() { writeCellDefaultsTemplate = real })
	wrote := 0
	writeCellDefaultsTemplate = func(f *os.File) error {
		// Half the template, ending inside a line, has reached the file when the write fails.
		n, err := f.WriteString(tpl[:len(tpl)/2])
		if err != nil {
			t.Fatal(err)
		}
		wrote += n
		return diskFull
	}
	gone := func(after string) {
		t.Helper()
		if wrote == 0 {
			t.Fatalf("%s: the failing write was never reached", after)
		}
		wrote = 0
		if st, err := os.Lstat(path); !os.IsNotExist(err) {
			size := int64(-1)
			if st != nil {
				size = st.Size()
			}
			t.Errorf("%s: a write that failed part-way left a file at %s (%d of %d bytes; %v)", after, path, size, len(tpl), err)
			os.Remove(path)
		}
	}

	created, err := createCellDefaults(path)
	if created || !errors.Is(err, diskFull) {
		t.Errorf("createCellDefaults = %v, %v, want not created and the write's own error", created, err)
	}
	gone("createCellDefaults")

	msg := refusal(t, "defaults init on a write that fails", func() { cmdDefaults([]string{"init"}) })
	if !strings.Contains(msg, "cannot create "+path) || !strings.Contains(msg, diskFull.Error()) {
		t.Errorf("defaults init should refuse with the path and the error:\n%s", msg)
	}
	gone("defaults init")

	var out string
	notice, refused := tryRefusal(t, func() { out = captureStdout(t, seedCellDefaults) })
	if refused || !strings.Contains(notice, "NOTICE: no machine-wide defaults file written at "+path) || !strings.Contains(notice, diskFull.Error()) || strings.Contains(out, "[defaults]") {
		t.Errorf("`new` should report a failed write as a notice and claim nothing (refused=%v):\nstdout: %s\nstderr: %s", refused, out, notice)
	}
	gone("new")

	// With the write working again, nothing is in the way: the whole template is written.
	writeCellDefaultsTemplate = real
	if created, err := createCellDefaults(path); !created || err != nil {
		t.Fatalf("createCellDefaults after the failure = %v, %v, want created", created, err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != tpl {
		t.Errorf("the file written after a failed attempt is not the template (%v, %d of %d bytes)", err, len(got), len(tpl))
	}
}

// TestSeedCellDefaultsNotARegularFileIsANotice: `new` never replaces what is at the defaults
// path, and it stays quiet about a file that is there. Something there that is NOT a regular
// file — a directory, a symlink to nothing — is different: every cell under the root refuses to
// load until it is dealt with, so `new` says so instead of reporting a cell that will not boot.
func TestSeedCellDefaultsNotARegularFileIsANotice(t *testing.T) {
	seed := func(t *testing.T) (stdout, stderr string) {
		t.Helper()
		stderr = captureStderr(t, func() { stdout = captureStdout(t, seedCellDefaults) })
		return stdout, stderr
	}
	t.Run("a directory", func(t *testing.T) {
		root := defaultsRoot(t)
		path := filepath.Join(root, cellDefaultsFile)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		out, errOut := seed(t)
		if !strings.Contains(errOut, "NOTICE: ") || !strings.Contains(errOut, path) || !strings.Contains(errOut, "is not a regular file") {
			t.Errorf("a directory at the defaults path: want a notice naming it, got %q", errOut)
		}
		if out != "" {
			t.Errorf("nothing was created, and stdout says %q", out)
		}
		if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
			t.Errorf("the directory at the defaults path was touched (%v)", err)
		}
	})
	t.Run("a symlink to nothing", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need a privilege on Windows")
		}
		root := defaultsRoot(t)
		path := filepath.Join(root, cellDefaultsFile)
		if err := os.Symlink(filepath.Join(root, "absent"), path); err != nil {
			t.Fatal(err)
		}
		_, errOut := seed(t)
		if !strings.Contains(errOut, "NOTICE: ") || !strings.Contains(errOut, path) || !strings.Contains(errOut, "is not a regular file") {
			t.Errorf("a dangling symlink at the defaults path: want a notice naming it, got %q", errOut)
		}
		if _, err := os.Lstat(filepath.Join(root, "absent")); !os.IsNotExist(err) {
			t.Errorf("the seed wrote through the symlink (%v)", err)
		}
	})
	t.Run("a file is left alone in silence", func(t *testing.T) {
		root := defaultsRoot(t)
		writeDefaults(t, root, "CELL_COCKPIT=tmux\n")
		if out, errOut := seed(t); out != "" || errOut != "" {
			t.Errorf("a defaults file that is already there: want silence, got stdout %q stderr %q", out, errOut)
		}
	})
}

// TestBinaryCellDefaultsHelp: `cellctl defaults --help` prints the usage and exits 0, as
// `cellctl new --help` does; it is not the usage refusal a wrong operation gets.
func TestBinaryCellDefaultsHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		cmd := exec.Command(cellctlBinary(t), "defaults", flag)
		var stdout, stderr strings.Builder
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Errorf("defaults %s: %v\n%s", flag, err, stderr.String())
		}
		if !strings.Contains(stdout.String(), "defaults init|print") {
			t.Errorf("defaults %s: the usage on stdout should describe the verb:\n%s", flag, stdout.String())
		}
		if strings.Contains(stderr.String(), "cellctl: usage:") {
			t.Errorf("defaults %s is answered with the usage refusal: %s", flag, stderr.String())
		}
	}
	// A wrong operation is still refused.
	cmd := exec.Command(cellctlBinary(t), "defaults", "help-me")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "usage: cellctl defaults init|print") {
		t.Errorf("defaults help-me: want the usage refusal, got %v:\n%s", err, out)
	}
}
