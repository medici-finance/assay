package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The machine-wide defaults file (defaults.go). Every test here builds its own cells root under
// t.TempDir() and points CELLS_ROOT at it, so none reads a cells root or a config home that
// belongs to the machine running the test.

// defaultsRoot is an empty cells root with CELLS_ROOT pointed at it, and the keys these tests
// assert on removed from the inherited environment so a developer's shell cannot decide a row.
func defaultsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("CELLS_ROOT", root)
	for _, k := range []string{
		"CELL", "CELL_KIND", "CELL_KIND_OVERRIDE_INTERNAL", "CELL_HARNESS", "ROLES", "DESKD", "DRY_RUN",
		"CELL_REPO", "CELL_REPO_SLUG", "CELL_ROOTS", "TMUX_SESSION", "DESK_MODEL_DEFAULT",
		"CELL_GO_CACHE", "CELL_GO_CACHE_ROOT", "CELL_GO_CACHE_BYTES", "CELL_GO_CACHE_MIN_FREE",
		"CELL_PATH", "CELL_PROVIDER", "CELL_PROVIDER_DEFAULTS", "CELL_PROVIDER_OVERRIDES",
		"CELL_MODEL_POLICY", "CELL_ROLE_CONTEXT", "CELLCTL_TEST_PROBE",
	} {
		t.Setenv(k, "") // registers the restore
		os.Unsetenv(k)
	}
	return root
}

func writeDefaults(t *testing.T, root, body string) string {
	t.Helper()
	p := filepath.Join(root, cellDefaultsFile)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// demoCell is the smallest cell.env loadCell accepts for a house cell — the kind every harness
// value is valid on, which the layering rows need three distinct values of.
const demoCell = "CELL=demo\nCELL_KIND=house\nCELL_ROOTS=example-org/example-repo=/example/repo\nCELL_REPO=/example/repo\n"

// writeCell writes <root>/<name>/cell.env and returns the file's path.
func writeCell(t *testing.T, root, name, body string) string {
	t.Helper()
	d := filepath.Join(root, name)
	if err := os.MkdirAll(d, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "cell.env")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// refusal runs fn, which must refuse (die), and returns what it printed on stderr.
func refusal(t *testing.T, what string, fn func()) string {
	t.Helper()
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = wr
	done := make(chan string, 1)
	go func() { b, _ := io.ReadAll(rd); done <- string(b) }()
	refused := false
	func() {
		defer func() {
			os.Stderr = prev
			wr.Close()
			if r := recover(); r != nil {
				if _, ok := r.(exitCode); !ok {
					panic(r)
				}
				refused = true
			}
		}()
		fn()
	}()
	msg := <-done
	rd.Close()
	if !refused {
		t.Fatalf("%s: expected a refusal, got none", what)
	}
	return msg
}

// TestCellDefaultsLayering is the order: compiled default < process environment < defaults file
// < the cell's own cell.env. Each row states which layers carry the key and which must win, both
// for a plain key and for one loadCell turns into a field of the Cell.
func TestCellDefaultsLayering(t *testing.T) {
	const absent = "\x00"
	cases := []struct {
		name                       string
		process, defaults, cellEnv string
		want, wantSource           string
	}{
		{"a key only the defaults file sets applies to a cell that does not set it", absent, "codex", absent, "codex", layerDefaults},
		{"cell.env wins over the defaults file", absent, "codex", "cursor", "cursor", layerCellEnv},
		{"the defaults file wins over the process environment", "cursor", "codex", absent, "codex", layerDefaults},
		{"cell.env wins over both", "cursor", "codex", "claude", "claude", layerCellEnv},
		{"the process environment still applies when neither file sets the key", "codex", absent, absent, "codex", layerProcess},
		{"a defaults file that does not set the key leaves the process environment in force", "codex", "", absent, "codex", layerProcess},
		{"with no layer setting it the compiled default stands", absent, absent, absent, "claude", layerCompiled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := defaultsRoot(t)
			if tc.process != absent {
				t.Setenv("CELL_HARNESS", tc.process)
				t.Setenv("CELLCTL_TEST_PROBE", tc.process)
			}
			if tc.defaults != absent {
				body := "# machine-wide\nUNRELATED_DEFAULT=1\n"
				if tc.defaults != "" {
					body += "CELL_HARNESS=" + tc.defaults + "\nexport CELLCTL_TEST_PROBE='" + tc.defaults + "'\n"
				}
				writeDefaults(t, root, body)
			}
			cell := demoCell
			if tc.cellEnv != absent {
				cell += "CELL_HARNESS=" + tc.cellEnv + "\nCELLCTL_TEST_PROBE=" + tc.cellEnv + "\n"
			}
			writeCell(t, root, "demo", cell)

			c := loadCell("demo")
			if c.Harness != tc.want {
				t.Errorf("Cell.Harness = %q, want %q", c.Harness, tc.want)
			}
			if got := c.Env.Source("CELL_HARNESS"); got != tc.wantSource && !(tc.wantSource == layerCompiled && got == layerUnset) {
				t.Errorf("Source(CELL_HARNESS) = %q, want %q", got, tc.wantSource)
			}
			// The probe is a key cellctl never touches, so its value and source are the layers alone.
			wantProbe, wantProbeSource := tc.want, tc.wantSource
			if tc.wantSource == layerCompiled {
				wantProbe, wantProbeSource = "", layerUnset
			}
			if got := c.Env.Get("CELLCTL_TEST_PROBE"); got != wantProbe {
				t.Errorf("probe = %q, want %q", got, wantProbe)
			}
			if got := c.Env.Source("CELLCTL_TEST_PROBE"); got != wantProbeSource {
				t.Errorf("Source(probe) = %q, want %q", got, wantProbeSource)
			}
			if wantRead := tc.defaults != absent; c.DefaultsRead != wantRead {
				t.Errorf("DefaultsRead = %v, want %v", c.DefaultsRead, wantRead)
			}
			if want := filepath.Join(root, cellDefaultsFile); c.DefaultsPath != want {
				t.Errorf("DefaultsPath = %q, want %q", c.DefaultsPath, want)
			}
		})
	}
}

// TestCellDefaultsKeysAreReportedInFileOrder: the keys the file set, once each, in the order the
// file names them — what `check` prints.
func TestCellDefaultsKeysAreReportedInFileOrder(t *testing.T) {
	root := defaultsRoot(t)
	writeDefaults(t, root, "\n  # indented comment\nCELL_GO_CACHE=on\n\t\nexport DESK_MODEL_DEFAULT=example-model\nCELL_GO_CACHE=off\n\r\n")
	writeCell(t, root, "demo", demoCell)
	c := loadCell("demo")
	if want := []string{"CELL_GO_CACHE", "DESK_MODEL_DEFAULT"}; !reflect.DeepEqual(c.DefaultsKeys, want) {
		t.Errorf("DefaultsKeys = %v, want %v", c.DefaultsKeys, want)
	}
	// A later line for the same key wins, exactly as it does in cell.env.
	if got := c.Env.Get("CELL_GO_CACHE"); got != "off" {
		t.Errorf("CELL_GO_CACHE = %q, want the file's last line", got)
	}
}

// TestCellDefaultsMissingFileChangesNothing: with no defaults file a cell resolves exactly as the
// pre-existing two layers resolve it — the process environment with cell.env overlaid.
func TestCellDefaultsMissingFileChangesNothing(t *testing.T) {
	root := defaultsRoot(t)
	t.Setenv("CELLCTL_TEST_PROBE", "from-process")
	envfile := writeCell(t, root, "demo", demoCell+"CELL_HARNESS=codex\nDESK_MODEL_DEFAULT=example-model\n")

	// The reference is the loader as it was before the defaults layer existed.
	ref := newEnvFromProcess()
	if err := parseCellEnv(ref, envfile); err != nil {
		t.Fatal(err)
	}
	c := loadCell("demo")
	if c.DefaultsRead || c.DefaultsKeys != nil {
		t.Errorf("no file was there to read: DefaultsRead=%v DefaultsKeys=%v", c.DefaultsRead, c.DefaultsKeys)
	}
	for k, v := range ref.vals {
		if v == "" {
			continue // loadCell fills an empty key with its compiled default, as it always has
		}
		if got := c.Env.Get(k); got != v {
			t.Errorf("%s = %q, want %q (the value before the defaults layer existed)", k, got, v)
		}
	}
	for k := range c.Env.vals {
		if c.Env.Source(k) == layerDefaults {
			t.Errorf("%s is attributed to a defaults file that does not exist", k)
		}
	}
	// `set`'s view is the cell.env alone, key for key.
	only := &Env{vals: map[string]string{}, set: map[string]bool{}}
	if err := parseCellEnv(only, envfile); err != nil {
		t.Fatal(err)
	}
	if got := effectiveCellEnv(envfile, nil); !reflect.DeepEqual(got.vals, only.vals) || !reflect.DeepEqual(got.set, only.set) {
		t.Errorf("effectiveCellEnv with no defaults file = %v, want cell.env alone %v", got.vals, only.vals)
	}
}

// TestCellDefaultsRefusesPerCellKeys: every key that names or scopes one cell is refused in the
// defaults file, by the loader and by `set`, and the refusal names the key and the file. So is
// every key cellctl takes from the command that runs it: a switch for one run, a location it
// follows from the launching shell (the cells root and what locates it among them), a variable
// of the host, a carrier of its own.
func TestCellDefaultsRefusesPerCellKeys(t *testing.T) {
	// The keys the design names outright. Removing one from the refused list must fail here,
	// not just shrink the loop below.
	listed := map[string]bool{}
	for _, r := range cellDefaultsRefused {
		if listed[r.key] {
			t.Errorf("%s is listed twice", r.key)
		}
		if r.why == "" {
			t.Errorf("%s is refused without a reason", r.key)
		}
		listed[r.key] = true
	}
	named := []string{
		// One cell's identity, scope or bound resource.
		"CELL", "CELL_KIND", "CELL_REPO", "CELL_REPO_SLUG", "CELL_ROOTS", "ROLES",
		"CELLS_CONFIG", "CELL_COMMS_CONFIG",
		"DESKD", "DESKD_ADDR", "DESKD_INDEX", "TMUX_SESSION", "CELL_GO_CACHE_ROOT",
		"CELL_CONTAINER_CONFIG", "CELL_CONTAINER_LAUNCHER",
		"CELL_FORGE", "GITHUB_HOST", "FORGE_API_BASE", "GITLAB_API_BASE", "GITLAB_GROUP",
		"GITLAB_TOKEN_STORE", "DESKD_GITLAB_TOKEN_FILE", "DESKD_APP_PEM", "DESKD_APP_ID_VAR", "ORGS",
		// A switch for one run of one command.
		"CELLS_ROOT", "DRY_RUN", "CELL_ATTENDED", "DESK_MODEL_OVERRIDE",
		// What locates the cells root when CELLS_ROOT is unset, and the config homes.
		"HOME", "USERPROFILE", "XDG_DATA_HOME",
		"ASSAY_CONFIG_HOME", "XDG_CONFIG_HOME", "GH_CONFIG_DIR", "CLAUDE_CONFIG_DIR", "CODEX_HOME",
		"APPDATA", "LOCALAPPDATA",
		// The launching shell's own.
		"PATH", "TERM", "LANG",
		// Carriers cellctl sets for itself or a child.
		"CELL_KIND_OVERRIDE_INTERNAL", "ASSAY_SCRATCH_ID", "DESK_SESSION", "CELLCTL_PARITY_MUTATE",
	}
	for _, k := range named {
		if !listed[k] {
			t.Errorf("%s must be refused in the defaults file and is not listed", k)
		}
	}
	// Both ways: a key refused without being named here is a refusal nobody decided on.
	if len(listed) != len(named) {
		t.Errorf("the defaults file refuses %d key(s) and this test names %d: name the new one here, with the others of its kind", len(listed), len(named))
	}

	for _, r := range cellDefaultsRefused {
		for _, line := range []string{r.key + "=value", "export " + r.key + "='value'"} {
			t.Run(line, func(t *testing.T) {
				root := defaultsRoot(t)
				file := writeDefaults(t, root, "# shared\nCELL_GO_CACHE=on\n"+line+"\n")
				envfile := writeCell(t, root, "demo", demoCell)
				before, _ := os.ReadFile(envfile)

				msg := refusal(t, "loadCell", func() { loadCell("demo") })
				for _, want := range []string{file, "sets " + r.key + ",", "line 3", r.why} {
					if !strings.Contains(msg, want) {
						t.Errorf("load refusal does not carry %q:\n%s", want, msg)
					}
				}
				msg = refusal(t, "set", func() { cmdSet("demo", []string{"DESK_MODEL_DEFAULT=example-model"}) })
				for _, want := range []string{"cellctl: set: ", file, "sets " + r.key + ",", "nothing written"} {
					if !strings.Contains(msg, want) {
						t.Errorf("set refusal does not carry %q:\n%s", want, msg)
					}
				}
				if after, _ := os.ReadFile(envfile); string(after) != string(before) {
					t.Errorf("a refused set wrote cell.env:\n%s", after)
				}
				if baks, _ := filepath.Glob(envfile + ".bak-*"); len(baks) != 0 {
					t.Errorf("a refused set left a backup: %v", baks)
				}
			})
		}
	}

	// The same key in the cell's OWN cell.env is not this refusal's business.
	root := defaultsRoot(t)
	writeDefaults(t, root, "CELL_GO_CACHE=on\n")
	writeCell(t, root, "demo", demoCell+"ROLES='the-desk'\nTMUX_SESSION=example-session\n")
	if c := loadCell("demo"); c.Session != "example-session" || !reflect.DeepEqual(c.Roles, []string{"the-desk"}) {
		t.Errorf("per-cell keys in cell.env must still load: session=%q roles=%v", c.Session, c.Roles)
	}
}

// TestCellDefaultsRefusesMalformedFile: a defaults file is read strictly. A line that is not an
// assignment, or a path that is there and is not a readable regular file, stops the verb and
// names the file — it is never skipped the way a stray line in cell.env is.
func TestCellDefaultsRefusesMalformedFile(t *testing.T) {
	for _, tc := range []struct{ name, body, wantLine string }{
		{"a bare word", "CELL_GO_CACHE=on\nCELL_GO_CACHE\n", "line 2"},
		{"prose", "these are the machine defaults\n", "line 1"},
		{"no key", "\n=on\n", "line 2"},
		{"a key starting with a digit", "1CELL=on\n", "line 1"},
		{"a key with a space", "# ok\n\nCELL GO CACHE=on\n", "line 3"},
		{"a shell command", "CELL_GO_CACHE=on\nsource other.env\n", "line 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := defaultsRoot(t)
			file := writeDefaults(t, root, tc.body)
			writeCell(t, root, "demo", demoCell)
			msg := refusal(t, "loadCell", func() { loadCell("demo") })
			for _, want := range []string{file, tc.wantLine, "not a KEY=VALUE assignment"} {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal does not carry %q:\n%s", want, msg)
				}
			}
			msg = refusal(t, "effectiveCellEnv", func() { effectiveCellEnv(filepath.Join(root, "demo", "cell.env"), nil) })
			if !strings.Contains(msg, file) || !strings.Contains(msg, "cellctl: set: ") {
				t.Errorf("set-side refusal does not name the file:\n%s", msg)
			}
		})
	}

	t.Run("a directory at the path", func(t *testing.T) {
		root := defaultsRoot(t)
		file := filepath.Join(root, cellDefaultsFile)
		if err := os.Mkdir(file, 0o700); err != nil {
			t.Fatal(err)
		}
		writeCell(t, root, "demo", demoCell)
		if msg := refusal(t, "loadCell", func() { loadCell("demo") }); !strings.Contains(msg, "cannot read cell defaults file "+file) {
			t.Errorf("refusal does not name the file:\n%s", msg)
		}
	})
	t.Run("a symlink to nothing", func(t *testing.T) {
		root := defaultsRoot(t)
		file := filepath.Join(root, cellDefaultsFile)
		if err := os.Symlink(filepath.Join(root, "gone.env"), file); err != nil {
			t.Skipf("cannot create a symlink here: %v", err)
		}
		writeCell(t, root, "demo", demoCell)
		if msg := refusal(t, "loadCell", func() { loadCell("demo") }); !strings.Contains(msg, "cannot read cell defaults file "+file) {
			t.Errorf("a dangling defaults file must refuse, not read as absent:\n%s", msg)
		}
	})
	t.Run("a file that cannot be opened", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("needs a mode bit that denies the owner")
		}
		root := defaultsRoot(t)
		file := writeDefaults(t, root, "CELL_GO_CACHE=on\n")
		if err := os.Chmod(file, 0); err != nil {
			t.Fatal(err)
		}
		writeCell(t, root, "demo", demoCell)
		if msg := refusal(t, "loadCell", func() { loadCell("demo") }); !strings.Contains(msg, "cannot read cell defaults file "+file) {
			t.Errorf("refusal does not name the file:\n%s", msg)
		}
	})
	t.Run("blank lines and comments alone are a file that sets nothing", func(t *testing.T) {
		root := defaultsRoot(t)
		writeDefaults(t, root, "# nothing yet\n\n   \n\t# indented\n\r\n")
		writeCell(t, root, "demo", demoCell)
		if c := loadCell("demo"); !c.DefaultsRead || len(c.DefaultsKeys) != 0 {
			t.Errorf("DefaultsRead=%v DefaultsKeys=%v, want read with no keys", c.DefaultsRead, c.DefaultsKeys)
		}
	})
}

// TestCellDefaultsSetWritesOnlyCellEnv: `cellctl set` resolves the cell through the same layers
// loadCell does, and writes the cell's own cell.env and nothing else.
func TestCellDefaultsSetWritesOnlyCellEnv(t *testing.T) {
	root := defaultsRoot(t)
	const shared = "# machine-wide\nCELL_HARNESS=codex\nCELL_GO_CACHE=on\n"
	defaults := writeDefaults(t, root, shared)
	envfile := writeCell(t, root, "demo", demoCell)
	otherCell := strings.Replace(demoCell, "CELL=demo", "CELL=other", 1) + "CELL_HARNESS=claude\n"
	other := writeCell(t, root, "other", otherCell)

	// The path `set` derives from a cell.env is the one loadCell reads.
	if got, want := cellDefaultsFor(envfile), loadCell("demo").DefaultsPath; got != want {
		t.Fatalf("set would read %q, loadCell reads %q", got, want)
	}
	if got := activeHarnessOf(envfile); got != "codex" {
		t.Fatalf("activeHarnessOf = %q, want the defaults file's codex", got)
	}

	// The role form picks its key from the ACTIVE harness. Only the defaults file says codex.
	captureStdout(t, func() { cmdSet("demo", []string{"worker-desk", "--model", "example-codex-model"}) })
	// A cell that sets its own harness is not moved by the machine's.
	captureStdout(t, func() { cmdSet("other", []string{"worker-desk", "--model", "example-claude-model"}) })
	// The KEY=VALUE form overrides a machine default for this one cell.
	captureStdout(t, func() { cmdSet("demo", []string{"CELL_HARNESS=claude"}) })

	read := func(p string) string {
		t.Helper()
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read(defaults); got != shared {
		t.Errorf("set wrote the defaults file:\n%s", got)
	}
	if got, want := read(envfile), demoCell+"CODEX_MODEL_worker_desk=example-codex-model\nCELL_HARNESS=claude\n"; got != want {
		t.Errorf("demo cell.env =\n%s\nwant\n%s", got, want)
	}
	if got, want := read(other), otherCell+"DESK_MODEL_worker_desk=example-claude-model\n"; got != want {
		t.Errorf("other cell.env =\n%s\nwant\n%s", got, want)
	}
	// Nothing appears in the cells root but the two cells and the one defaults file, and each
	// cell directory holds its cell.env, that file's backups, and the pin record a model pin writes.
	if names, want := mustReadDir(t, root), []string{cellDefaultsFile, "demo", "other"}; !reflect.DeepEqual(names, want) {
		t.Errorf("cells root holds %v, want %v", names, want)
	}
	for _, cell := range []string{"demo", "other"} {
		for _, n := range mustReadDir(t, filepath.Join(root, cell)) {
			if n != "cell.env" && n != modelPinsFile && !strings.HasPrefix(n, "cell.env.bak-") {
				t.Errorf("set left %s in cell %s", n, cell)
			}
		}
	}
	// And the cell now loads with its own override in force.
	if c := loadCell("demo"); c.Harness != "claude" || c.Env.Source("CELL_HARNESS") != layerCellEnv || c.Env.Source("CELL_GO_CACHE") != layerDefaults {
		t.Errorf("after set: harness=%q from %s, CELL_GO_CACHE from %s", c.Harness, c.Env.Source("CELL_HARNESS"), c.Env.Source("CELL_GO_CACHE"))
	}
}

// TestCellDefaultsNestedCell: a cell named `team/demo` lives two levels under the cells root and
// still takes the ROOT's defaults file — from loadCell and from `set` alike — never a file that
// happens to sit beside it in `team/`.
func TestCellDefaultsNestedCell(t *testing.T) {
	root := defaultsRoot(t)
	rootFile := writeDefaults(t, root, "CELL_HARNESS=codex\n")
	envfile := writeCell(t, root, filepath.Join("team", "demo"), demoCell)
	decoy := filepath.Join(root, "team", cellDefaultsFile)
	if err := os.WriteFile(decoy, []byte("CELL_HARNESS=cursor\nCELL_KIND=scrubbed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := loadCell("team/demo")
	if c.DefaultsPath != rootFile || c.Harness != "codex" {
		t.Errorf("loadCell read %q (harness %s), want the cells root's %q", c.DefaultsPath, c.Harness, rootFile)
	}
	if got := cellDefaultsFor(envfile); got != rootFile {
		t.Errorf("set would read %q, want the cells root's %q", got, rootFile)
	}
	if got := activeHarnessOf(envfile); got != "codex" {
		t.Errorf("activeHarnessOf = %q, want codex from the cells root's file", got)
	}
	captureStdout(t, func() { cmdSet("team/demo", []string{"worker-desk", "--model", "example-codex-model"}) })
	if got, _ := os.ReadFile(envfile); string(got) != demoCell+"CODEX_MODEL_worker_desk=example-codex-model\n" {
		t.Errorf("nested cell.env =\n%s", got)
	}
	// A cell.env that is not under the cells root at all never picks up that root's file.
	outside := filepath.Join(t.TempDir(), "elsewhere", "cell.env")
	if got := cellDefaultsFor(outside); got == rootFile || got != filepath.Join(filepath.Dir(filepath.Dir(outside)), cellDefaultsFile) {
		t.Errorf("a cell.env outside the cells root resolved to %q", got)
	}
}

func mustReadDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// TestCellDefaultsReservedCellName: a cell directory at the defaults file's path would stop every
// cell on the machine from loading, so `new` does not create one.
func TestCellDefaultsReservedCellName(t *testing.T) {
	root := defaultsRoot(t)
	for _, name := range []string{"defaults.env", "Defaults.ENV", "defaults.env/inner"} {
		msg := refusal(t, "new "+name, func() { cmdNew([]string{name, "--kind", "house"}) })
		if !strings.Contains(msg, "not available as a cell name") || !strings.Contains(msg, filepath.Join(root, cellDefaultsFile)) {
			t.Errorf("new %s: %s", name, msg)
		}
		if _, err := os.Lstat(filepath.Join(root, cellDefaultsFile)); !os.IsNotExist(err) {
			t.Errorf("new %s created something in the cells root", name)
		}
	}
}

// TestBinaryCheckReportsCellDefaults is `check`'s account of the layer, on the built binary: the
// row that says a defaults file was read, and the managed Go cache's state with the layer that
// supplied it. A cell that uses neither prints neither, and no row changes the exit code.
func TestBinaryCheckReportsCellDefaults(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the fixture's stubs are shell scripts, and the managed Go cache is macOS/Linux only")
	}
	f := newPolicyFixture(t, "2.1.295")
	for _, n := range []string{"tmux", "codex"} {
		f.stub(t, n, "#!/bin/sh\nexit 0\n")
	}
	file := filepath.Join(f.cellsRoot, cellDefaultsFile)
	rows := func(r runResult) []string {
		var out []string
		for _, l := range strings.Split(r.stdout, "\n") {
			if strings.Contains(l, "cell defaults") || strings.Contains(l, "managed Go cache") {
				out = append(out, l)
			}
		}
		return out
	}
	check := func(t *testing.T, env ...string) runResult {
		t.Helper()
		return f.run(t, append([]string{f.hermeticPath()}, env...), "check", "example")
	}
	on := func(supply string) string { return "  ok    managed Go cache: on (" + supply + ")" }

	f.plainCell(t)
	base := check(t)
	if got := rows(base); got != nil {
		t.Fatalf("a cell with no defaults file and no CELL_GO_CACHE prints no row for either, got %q", got)
	}

	for _, tc := range []struct {
		name     string
		defaults string // "" = no file
		cellEnv  []string
		env      []string
		want     []string
	}{
		{"the defaults file turns the cache on", "# machine-wide\nCELL_GO_CACHE=on\nCELL_GO_CACHE_BYTES=1073741824\n", nil, nil, []string{
			"  ok    cell defaults: read " + file + " (2 key(s): CELL_GO_CACHE CELL_GO_CACHE_BYTES; this cell's cell.env overrides each)",
			on("CELL_GO_CACHE=on from defaults file"),
		}},
		{"cell.env overrides it off", "CELL_GO_CACHE=on\n", []string{"CELL_GO_CACHE=off"}, nil, []string{
			"  ok    cell defaults: read " + file + " (1 key(s): CELL_GO_CACHE; this cell's cell.env overrides each)",
			"  n/a   managed Go cache: off (CELL_GO_CACHE=off from cell.env)",
		}},
		{"the defaults file overrides the process environment", "CELL_GO_CACHE=off\n", nil, []string{"CELL_GO_CACHE=on"}, []string{
			"  ok    cell defaults: read " + file + " (1 key(s): CELL_GO_CACHE; this cell's cell.env overrides each)",
			"  n/a   managed Go cache: off (CELL_GO_CACHE=off from defaults file)",
		}},
		{"a defaults file that leaves the cache off", "CELL_GO_CACHE_BYTES=1073741824\n", nil, nil, []string{
			"  ok    cell defaults: read " + file + " (1 key(s): CELL_GO_CACHE_BYTES; this cell's cell.env overrides each)",
			"  n/a   managed Go cache: off (CELL_GO_CACHE unset)",
		}},
		{"an empty defaults file", "# nothing yet\n", nil, nil, []string{
			"  ok    cell defaults: read " + file + " (it sets no keys; this cell's cell.env overrides each)",
			"  n/a   managed Go cache: off (CELL_GO_CACHE unset)",
		}},
		{"no defaults file, the process environment turns the cache on", "", nil, []string{"CELL_GO_CACHE=on"}, []string{
			"  n/a   cell defaults: none read — no " + file,
			on("CELL_GO_CACHE=on from process environment"),
		}},
		{"no defaults file, cell.env turns the cache on", "", []string{"CELL_GO_CACHE=on"}, nil, []string{
			"  n/a   cell defaults: none read — no " + file,
			on("CELL_GO_CACHE=on from cell.env"),
		}},
		{"a value the cache refuses is a warn, never a MISS", "CELL_GO_CACHE=sometimes\n", nil, nil, []string{
			"  ok    cell defaults: read " + file + " (1 key(s): CELL_GO_CACHE; this cell's cell.env overrides each)",
			"  warn  managed Go cache: unusable — CELL_GO_CACHE must be on or off (CELL_GO_CACHE=sometimes from defaults file); a launch refuses on it",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Remove(file)
			if tc.defaults != "" {
				if err := os.WriteFile(file, []byte(tc.defaults), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f.plainCell(t, tc.cellEnv...)
			r := check(t, tc.env...)
			if got := rows(r); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rows =\n  %s\nwant\n  %s\nstderr: %s", strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "), r.stderr)
			}
			if r.code != base.code {
				t.Errorf("exit %d, want the %d the same cell exits with before the rows existed", r.code, base.code)
			}
			// The rows sit directly under the header, and are the ONLY difference from the
			// output the same cell prints without them.
			lines := strings.Split(r.stdout, "\n")
			if len(lines) < 3 || !strings.HasPrefix(lines[0], "[check] cell=example ") || lines[1] != tc.want[0] || lines[2] != tc.want[1] {
				t.Errorf("rows are not directly under the header:\n%s", r.stdout)
			}
			if rest := strings.Join(append(lines[:1:1], lines[3:]...), "\n"); rest != base.stdout {
				t.Errorf("check output differs beyond the two rows:\n--- with\n%s\n--- without\n%s", rest, base.stdout)
			}
		})
	}

	// An unusable file stops check before it prints a row, and says which file.
	if err := os.WriteFile(file, []byte("CELL_KIND=scrubbed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.plainCell(t)
	if r := check(t); r.code != 3 || r.stdout != "" || !strings.Contains(r.stderr, file) || !strings.Contains(r.stderr, "sets CELL_KIND,") {
		t.Errorf("check on a refused defaults file: %+v", r)
	}
}

// TestCellDefaultsDoNotWidenAScrubbedLaunch is the isolation decision for the scrubbed kind. The
// file applies there — a default the kind's allowlist already carries (the managed Go cache,
// CELL_PATH) arrives — and it adds NOTHING else: a key in the file reaches the composed child
// environment only where the same key in the process environment already would.
func TestCellDefaultsDoNotWidenAScrubbedLaunch(t *testing.T) {
	root := defaultsRoot(t)
	stubHarnessesOnPath(t)
	repo := filepath.Join(t.TempDir(), "repo")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Skipf("git init: %v: %s", err, out)
	}
	cacheSupported := runtime.GOOS == "darwin" || runtime.GOOS == "linux"
	body := "LEAK_PROBE=host-default-value\nGH_TOKEN=host-default-value\nSSH_AUTH_SOCK=/host-default-value/agent\n" +
		"ANTHROPIC_API_KEY=host-default-value\nKUBECONFIG=/host-default-value/kube\nCELL_PATH=/opt/example/bin\n" +
		"DESK_TOOLS_BIN=/opt/example/tools\n"
	// TERM and LANG reach the child from the launching shell and from nowhere else: the file
	// refuses both (TestCellDefaultsRefusesPerCellKeys), so these are the values it must carry.
	t.Setenv("TERM", "example-term")
	t.Setenv("LANG", "example.UTF-8")
	if cacheSupported {
		body += "CELL_GO_CACHE=on\n"
	}
	writeDefaults(t, root, body)
	writeCell(t, root, "demo", "CELL=demo\nCELL_KIND=scrubbed\nCELL_REPO="+cellEnvPathFor(runtime.GOOS, repo)+
		"\nCELL_REPO_SLUG=example-org/example-repo\nCELL_ROOTS=example-org/example-repo="+cellEnvPathFor(runtime.GOOS, repo)+"\n")

	c := loadCell("demo")
	if c.Kind != "scrubbed" || c.Env.Source("LEAK_PROBE") != layerDefaults {
		t.Fatalf("fixture: kind=%s, LEAK_PROBE from %s", c.Kind, c.Env.Source("LEAK_PROBE"))
	}
	composed := c.scrubbedComposeEnv("worker-desk", "claude", "demo-worker-desk")

	allowed := map[string]bool{}
	for _, k := range scrubbedEnvKeys {
		allowed[k] = true
	}
	p, err := c.cachePolicy()
	if err != nil {
		t.Fatal(err)
	}
	if cacheSupported {
		if p == nil {
			t.Fatal("the defaults file's CELL_GO_CACHE=on did not reach a scrubbed cell")
		}
		for _, kv := range p.Env() {
			k, _, _ := strings.Cut(kv, "=")
			allowed[k] = true
			if !valueIn(kv, composed.Pairs) {
				t.Errorf("scrubbed launch lost the managed cache variable %s", k)
			}
		}
	}
	for _, kv := range composed.Pairs {
		k, v, _ := strings.Cut(kv, "=")
		if !allowed[k] {
			t.Errorf("composed environment carries %s, which the scrubbed allowlist does not name", k)
		}
		if strings.Contains(v, "host-default-value") {
			t.Errorf("a defaults-file value reached the scrubbed child through %s", k)
		}
	}
	// CELL_PATH is the one named route for a path default, and it is the same route the
	// process environment has always had: the trailing system part of the composed PATH.
	if got := envValue(composed.Pairs, "PATH"); !strings.HasSuffix(got, ":/opt/example/bin") {
		t.Errorf("PATH = %q, want the CELL_PATH tail", got)
	}
	// DESK_TOOLS_BIN is the other: the tool directory, second in the composed PATH, directly
	// after the cell's own shim directory — which no layer can displace.
	if got, want := envValue(composed.Pairs, "PATH"), filepath.Join(c.Dir, "shim")+":/opt/example/tools:"; !strings.HasPrefix(got, want) {
		t.Errorf("PATH = %q, want it to start %q", got, want)
	}
	if got := envValue(composed.Pairs, "PATH"); strings.Count(got, "/opt/example/tools") != 1 || strings.Count(got, "/opt/example/bin") != 1 {
		t.Errorf("PATH = %q: each file-supplied directory belongs in it exactly once", got)
	}
	for k, want := range map[string]string{"TERM": "example-term", "LANG": "example.UTF-8"} {
		if got := envValue(composed.Pairs, k); got != want {
			t.Errorf("%s = %q in the scrubbed child, want the launching shell's %q", k, got, want)
		}
	}

	// Host model policy stays refused on an isolated kind whichever layer names it.
	for _, tc := range []struct{ line, want string }{
		{"CELL_PROVIDER_DEFAULTS=providers.json", "provider defaults require a house or k8s cell"},
		{"CELL_PROVIDER_OVERRIDES=providers.json", "provider defaults require a house or k8s cell"},
	} {
		writeDefaults(t, root, tc.line+"\n")
		if _, _, err := loadCell("demo").cellModelPolicy(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s in the defaults file on a scrubbed cell: err = %v, want %q", tc.line, err, tc.want)
		}
	}
	writeDefaults(t, root, "CELL_ROLE_CONTEXT=role-context.json\n")
	if _, err := loadCell("demo").cellRoleContext(); err == nil || !strings.Contains(err.Error(), "requires a house or k8s cell") {
		t.Errorf("CELL_ROLE_CONTEXT in the defaults file on a scrubbed cell: err = %v", err)
	}
}

// TestBinaryCellDefaultsDoNotReachAContainerLauncher is the same decision for the container kind:
// the launcher is handed its fixed variables and nothing the defaults file sets, the managed Go
// cache included (a container's own runtime configures its caches).
func TestBinaryCellDefaultsDoNotReachAContainerLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the launcher stub is a shell script")
	}
	f := newPolicyFixture(t, "2.1.295")
	dump := filepath.Join(f.cellsRoot, "..", "launcher-env")
	launcher := f.stub(t, "fake-launcher", "#!/bin/sh\nenv > '"+dump+"'\necho container-check-ran\nexit 0\n")
	f.plainCell(t)
	raw, err := os.ReadFile(filepath.Join(f.cellDir, "cell.env"))
	if err != nil {
		t.Fatal(err)
	}
	env := strings.Replace(string(raw), "CELL_KIND=house", "CELL_KIND=container", 1) + "CELL_CONTAINER_LAUNCHER=" + launcher + "\n"
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(f.cellsRoot, cellDefaultsFile)
	if err := os.WriteFile(file, []byte("LEAK_PROBE=host-default-value\nGH_TOKEN=host-default-value\nCELL_GO_CACHE=on\nGOFLAGS=-host-default-value\nCELL_HARNESS=codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := f.run(t, []string{f.hermeticPath()}, "check", "example")
	if r.code != 0 || !strings.Contains(r.stdout, "container-check-ran") {
		t.Fatalf("container check: %+v", r)
	}
	for _, want := range []string{
		"  ok    cell defaults: read " + file + " (5 key(s): LEAK_PROBE GH_TOKEN CELL_GO_CACHE GOFLAGS CELL_HARNESS; this cell's cell.env overrides each)\n",
		"  n/a   managed Go cache: not applied to a container cell — its own runtime configures its caches (CELL_GO_CACHE=on from defaults file)\n",
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("check does not print %q:\n%s", want, r.stdout)
		}
	}
	got, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("the launcher did not run: %v", err)
	}
	fixed := map[string]bool{"HOME": true, "PATH": true, "TERM": true, "CELL": true, "CELL_KIND": true, "CELL_DIR": true,
		"CELL_REPO": true, "CELL_ROOTS": true, "CELL_HARNESS": true, "ROLES": true,
		// what a POSIX shell adds to its own environment
		"PWD": true, "OLDPWD": true, "SHLVL": true, "_": true}
	handed := map[string]string{}
	for _, kv := range strings.Split(strings.TrimSpace(string(got)), "\n") {
		k, v, _ := strings.Cut(kv, "=")
		handed[k] = v
		if !fixed[k] {
			t.Errorf("the container launcher was handed %s, which is not one of its fixed variables", k)
		}
		if strings.Contains(v, "host-default-value") {
			t.Errorf("a defaults-file value reached the container launcher through %s", k)
		}
	}
	// One of the fixed variables is a lever the file may set, and it arrives as the same key in
	// the shell would. The three that are the host's own (HOME, PATH, TERM) the file refuses, so
	// they are the launching process's: PATH is the one this run controls.
	if handed["CELL_HARNESS"] != "codex" {
		t.Errorf("the launcher was handed CELL_HARNESS=%q, want the defaults file's codex", handed["CELL_HARNESS"])
	}
	if want := strings.TrimPrefix(f.hermeticPath(), "PATH="); handed["PATH"] != want {
		t.Errorf("the launcher was handed PATH=%q, want the launching process's %q", handed["PATH"], want)
	}
}

// TestShowNamesTheDefaultsFileAsASource: `show` says where each effective choice came from, and
// a value the machine-wide file supplied is labelled with that file — never `cell.env`, which it
// is not in, and never `default`, which would hide that a file decided it. The same value moved
// into the cell's own file is labelled `cell.env`.
func TestShowNamesTheDefaultsFileAsASource(t *testing.T) {
	root := defaultsRoot(t)
	for _, k := range []string{"ASSAY_REPAIR_ADMISSION", "CELL_COCKPIT", "CELL_PROVIDER_GLM_BASE_URL", "CELL_PROVIDER_GLM_TOKEN_ENV", "CELL_PROVIDER_GLM_MODEL"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	lines := "CELL_HARNESS=claude\nCELL_COCKPIT=tmux\nDESK_MODEL_DEFAULT=example-model\nASSAY_REPAIR_ADMISSION=on\n" +
		"CELL_PROVIDER=glm\nCELL_PROVIDER_GLM_BASE_URL=https://example.invalid/api\n"
	show := func() string { return captureStdout(t, func() { cmdShow("demo", nil) }) }

	writeDefaults(t, root, lines)
	writeCell(t, root, "demo", demoCell)
	fromFile := show()
	for _, want := range []string{
		"[show] ASSAY_REPAIR_ADMISSION=on (defaults.env; composed into every desk launch)\n",
		"[show] CELL_COCKPIT=tmux (defaults.env)\n",
		"[show] CELL_HARNESS=claude (defaults.env)\n",
		"(defaults.env DESK_MODEL_DEFAULT)\n",
		"https://example.invalid/api (defaults.env)",
	} {
		if !strings.Contains(fromFile, want) {
			t.Errorf("show, with the value in the defaults file, does not print %q:\n%s", want, fromFile)
		}
	}
	for _, l := range strings.Split(fromFile, "\n") {
		// CELL_KIND is the one of these lines the cell's own file does set.
		if strings.Contains(l, "(cell.env") && !strings.HasPrefix(l, "[show] CELL_KIND=") {
			t.Errorf("show labels a value cell.env that the cell's file does not set: %s", l)
		}
	}

	if err := os.Remove(filepath.Join(root, cellDefaultsFile)); err != nil {
		t.Fatal(err)
	}
	writeCell(t, root, "demo", demoCell+lines)
	fromCell := show()
	for _, want := range []string{
		"[show] ASSAY_REPAIR_ADMISSION=on (cell.env; composed into every desk launch)\n",
		"[show] CELL_COCKPIT=tmux (cell.env)\n",
		"[show] CELL_HARNESS=claude (cell.env)\n",
		"(cell.env DESK_MODEL_DEFAULT)\n",
		"https://example.invalid/api (cell.env)",
	} {
		if !strings.Contains(fromCell, want) {
			t.Errorf("show, with the value in cell.env, does not print %q:\n%s", want, fromCell)
		}
	}
	if strings.Contains(fromCell, cellDefaultsFile) {
		t.Errorf("show names the defaults file when there is none:\n%s", fromCell)
	}
}
