package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
)

// The CLI contract tests (desktools-v2/16). They exercise the BUILT binary through the fixture
// world (cli_world_test.go); nothing here contacts a forge, a model endpoint or a cluster.

// visibleVerbs is every documented verb. The four internal entrypoints (model-policy,
// cache-run, container-run, providers) are covered by the legacy-forms and boundary tests.
var visibleVerbs = []string{
	"ls", "version", "new", "set", "show", "check", "deskd", "desk", "up", "down",
	"smoke", "status", "cadence", "scratch", "cache", "comms",
}

// snapshotTree is a stable fingerprint of every path under dir: name, mode, size and mtime.
func snapshotTree(t *testing.T, dir string) string {
	t.Helper()
	var rows []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			rows = append(rows, "ERR "+p)
			return nil
		}
		info, e := d.Info()
		if e != nil {
			rows = append(rows, "ERR "+p)
			return nil
		}
		rows = append(rows, fmt.Sprintf("%s %v %d %d", p, info.Mode(), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	sort.Strings(rows)
	return strings.Join(rows, "\n")
}

// effectStub writes a claude and a codex that record every invocation in logPath.
func effectStub(t *testing.T, binDir, logPath string) {
	t.Helper()
	for _, h := range []string{"claude", "codex"} {
		script := "#!/bin/sh\n{ printf 'RAN %s' \"$0\"; for a in \"$@\"; do printf ' [%s]' \"$a\"; done; echo; } >> '" + logPath + "'\necho READY\n"
		if err := os.WriteFile(filepath.Join(binDir, h), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// execIn runs bin with EXACTLY env as its environment (nothing inherited).
func execIn(t *testing.T, bin string, env []string, args ...string) cliRun {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
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

// TestCLIHelpOffline: help and version are pure introspection. They run against a missing
// registry, a malformed cell, unreadable cell and roster files, and a PATH holding only
// recording harness stubs, and in every one: exit 0, text on stdout, nothing on stderr (no
// roster echo, which is a read of the effective configuration), no harness started, and not one
// byte of the filesystem changed. In-process, the roster echo is counted directly.
func TestCLIHelpOffline(t *testing.T) {
	bin := cellctlBinary(t)
	type state struct {
		name  string
		build func(t *testing.T, cells, home string)
	}
	states := []state{{"absent-registry", func(t *testing.T, cells, home string) {}}}
	for _, m := range clicontract.MalformedConfigs {
		m := m
		states = append(states, state{"cell-env-" + m.Name, func(t *testing.T, cells, home string) {
			if m.Content == "" {
				return
			}
			dir := filepath.Join(cells, "example")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "cell.env"), []byte(m.Content), 0o644); err != nil {
				t.Fatal(err)
			}
		}})
	}
	states = append(states, state{"unreadable-credentials", func(t *testing.T, cells, home string) {
		dir := filepath.Join(cells, "example")
		cfg := filepath.Join(home, ".config", "assay")
		for _, d := range []string{dir, cfg} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for _, f := range []string{filepath.Join(dir, "cell.env"), filepath.Join(cfg, "roster.env"), filepath.Join(cfg, "apps.env")} {
			if err := os.WriteFile(f, []byte("CELL=example\n"), 0o000); err != nil {
				t.Fatal(err)
			}
		}
	}})
	forms := [][]string{}
	forms = append(forms, clicontract.HelpForms("desk")...)
	forms = append(forms, clicontract.GoFlagHelpForms("desk")...)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, clicontract.GoFlagVersionForms...)
	forms = append(forms, []string{"version"})
	for _, v := range visibleVerbs {
		forms = append(forms, []string{v, "--help"}, []string{"help", v})
	}
	forms = append(forms, []string{"--cells-root", "relative-is-never-applied", "--help"})

	for _, st := range states {
		st := st
		t.Run(st.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			cells, home, binDir := filepath.Join(root, "cells"), filepath.Join(root, "home"), filepath.Join(root, "bin")
			logPath := filepath.Join(root, "harness.log")
			for _, d := range []string{home, binDir} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			effectStub(t, binDir, logPath)
			st.build(t, cells, home)
			env := []string{"HOME=" + home, "CELLS_ROOT=" + cells, "PATH=" + binDir + ":" + os.Getenv("PATH"), "KUBECONFIG=/dev/null"}
			before := snapshotTree(t, root)
			for _, args := range forms {
				r := execIn(t, bin, env, args...)
				if r.Code != 0 {
					t.Errorf("%v: exit %d, want 0 (stderr %q)", args, r.Code, r.Stderr)
				}
				if r.Stderr != "" {
					t.Errorf("%v: wrote to stderr (a roster echo or config read ran): %q", args, r.Stderr)
				}
				if strings.TrimSpace(r.Stdout) == "" {
					t.Errorf("%v: printed nothing", args)
				}
			}
			if _, err := os.Stat(logPath); err == nil {
				t.Errorf("a harness was started by help or version")
			}
			if after := snapshotTree(t, root); after != before {
				t.Errorf("help or version changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}

	t.Run("roster-echo-count", func(t *testing.T) {
		n := 0
		old := echoRoster
		echoRoster = func() { n++ }
		defer func() { echoRoster = old }()
		t.Setenv("CELLS_ROOT", filepath.Join(t.TempDir(), "absent"))
		captureStdout(t, func() {
			for _, args := range forms {
				if code := inproc(args...); code != 0 {
					t.Errorf("%v: exit %d in-process", args, code)
				}
			}
		})
		if n != 0 {
			t.Errorf("roster echo ran %d times across help and version forms; want 0", n)
		}
		captureStdout(t, func() {
			if code := inproc("ls"); code != 0 || n != 1 {
				t.Errorf("a real command: exit %d, echo count %d; want 0 and exactly 1", code, n)
			}
		})
	})
}

// deskPlan runs `desk example worker-desk <extra>` as a dry run and returns the plan header.
var planModelRE = regexp.MustCompile(`(?m)^\[dry-run\] cell=\S+ kind=\S+ role=worker-desk model=(.*?)( \(override\))? effort=`)
var cadenceRE = regexp.MustCompile(`(?m)^\[cadence\] interval=(\S+) budget=(\S+)`)

func (w *cliWorld) deskPlan(t *testing.T, env []string, extra ...string) (cliRun, string, string) {
	t.Helper()
	args := append([]string{"desk", "example", "worker-desk"}, extra...)
	r := w.run(t, append([]string{"DRY_RUN=1"}, env...), args...)
	model, interval := "", ""
	if m := planModelRE.FindStringSubmatch(r.Stdout); m != nil {
		model = m[1]
	}
	if m := cadenceRE.FindStringSubmatch(r.Stdout); m != nil {
		interval = m[1]
	}
	return r, model, interval
}

// TestCLIConfigFlow: a value reaches the typed launch code byte-identical through every
// spelling the contract defines, and each declared setting resolves from its sources in the
// documented order. The resolved value is observed in the real dry-run plan of the binary.
func TestCLIConfigFlow(t *testing.T) {
	w := newCLIWorld(t, "")
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r, model, _ := w.deskPlan(t, nil, form.Args("model", pv.Value)...)
				if r.Code != 0 || model != pv.Value {
					t.Errorf("%s/%s: exit %d, model %q, want %q\n%s", form.Name, pv.Name, r.Code, model, pv.Value, r.Stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		pin := "sonnet" // the cell's own pin is the default when nothing overrides
		w.writeHouseEnv(t, "DESK_MODEL_DEFAULT="+pin+"\n")
		cases := []struct {
			name  string
			env   []string
			args  []string
			want  string
			wantC int
		}{
			{"nothing-set", nil, nil, pin, 0},
			{"env-over-default", []string{"DESK_MODEL_OVERRIDE=from-env"}, nil, "from-env", 0},
			{"flag-over-env", []string{"DESK_MODEL_OVERRIDE=from-env"}, []string{"--model", "from-flag"}, "from-flag", 0},
			{"empty-env-is-unset", []string{"DESK_MODEL_OVERRIDE="}, nil, pin, 0},
			{"empty-flag-refused", []string{"DESK_MODEL_OVERRIDE=from-env"}, []string{"--model="}, "", 3},
		}
		for _, c := range cases {
			r, model, _ := w.deskPlan(t, c.env, c.args...)
			if r.Code != c.wantC || model != c.want {
				t.Errorf("%s: exit %d model %q, want exit %d model %q\n%s", c.name, r.Code, model, c.wantC, c.want, r.Stderr)
			}
		}
		// cell.env is overlaid on the process environment, exactly as before the migration.
		w.writeHouseEnv(t, "DESK_MODEL_DEFAULT="+pin+"\nDESK_MODEL_OVERRIDE=from-cell-env\nCELL_CADENCE=45m\n")
		if _, model, _ := w.deskPlan(t, []string{"DESK_MODEL_OVERRIDE=from-proc"}); model != "from-cell-env" {
			t.Errorf("cell.env over process env: model %q, want from-cell-env", model)
		}
		for _, c := range []struct {
			name string
			args []string
			want string
		}{{"cadence-from-cell-env", nil, "45m0s"}, {"cadence-flag-over-cell-env", []string{"--cadence", "5m"}, "5m0s"}, {"cadence-equals-form", []string{"--cadence=9m"}, "9m0s"}} {
			if r, _, iv := w.deskPlan(t, nil, c.args...); r.Code != 0 || iv != c.want {
				t.Errorf("%s: exit %d interval %q, want %q\n%s", c.name, r.Code, iv, c.want, r.Stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		// Only the sources the contract names may feed a setting: no config-map source at all
		// (cellctl's config is cell.env, which is the Env source's overlay), no secret, and an
		// environment name only from this closed list.
		allowed := map[string]bool{"DESK_MODEL_OVERRIDE": true, "CELL_CADENCE": true, "CELL_TICK_BUDGET": true,
			"DESK_SESSION": true, "ASSAY_SCRATCH_ID": true}
		buildRoot()
		if len(declared) == 0 {
			t.Fatal("no bindings recorded")
		}
		for _, b := range declared {
			if b.Config {
				t.Errorf("binding %q opens the config-map source, which the contract does not declare", b.Key)
			}
			if b.Secret {
				t.Errorf("binding %q is a secret; cellctl declares none", b.Key)
			}
			for _, e := range b.Env {
				if !allowed[e] {
					t.Errorf("binding %q reads undeclared environment name %q", b.Key, e)
				}
			}
		}
	})
	t.Run("environment-names-follow-the-platform", func(t *testing.T) {
		w.writeHouseEnv(t, "DESK_MODEL_DEFAULT=sonnet\n")
		_, model, _ := w.deskPlan(t, []string{"desk_model_override=lower"})
		want := "sonnet"
		if clicontract.EnvFoldsCase(runtime.GOOS) {
			want = "lower"
		}
		if model != want {
			t.Errorf("lower-case environment name on %s: model %q, want %q", runtime.GOOS, model, want)
		}
	})
}

// logStub replaces the world's claude with one that records its argv and the launch-relevant
// environment, then answers READY (the smoke probe's expected reply).
func (w *cliWorld) logStub(t *testing.T) string {
	t.Helper()
	logPath := filepath.Join(w.root, "claude.log")
	script := "#!/bin/sh\ncase \"$1\" in\n--version) echo '2.1.278 (stub)'; exit 0;;\nesac\n" +
		"{ echo '--run'; for a in \"$@\"; do printf 'ARG=%s\\n' \"$a\"; done; " +
		"env | grep -E '^(CLAUDE_CONFIG_DIR|DESK_MODEL_OVERRIDE|ANTHROPIC_MODEL)=' | sort; } >> '" + logPath + "'\necho READY\n"
	if err := os.WriteFile(filepath.Join(w.binDir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return logPath
}

func readLog(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// TestCLIConfigLaunch: resolved settings reach the launch path exactly. Separate sequential
// invocations do not leak into each other, a persisted --set is what the next run resolves, a
// value the fake harness receives is the one the flag carried, and an environment name is
// matched by the platform's own rule.
func TestCLIConfigLaunch(t *testing.T) {
	t.Run("fake-harness-argv", func(t *testing.T) {
		w := newCLIWorld(t, "")
		logPath := w.logStub(t)
		values := []string{"m-one", "/tmp/a dir/with space.txt", `D:\op\m`, "a=b,c=d", "-dash-leading"}
		for _, v := range values {
			r := w.run(t, nil, "smoke", "scrub", "--harness", "claude", "--model", v)
			if r.Code != 0 {
				t.Fatalf("smoke --model %q: exit %d\n%s%s", v, r.Code, r.Stdout, r.Stderr)
			}
		}
		got := readLog(t, logPath)
		if n := strings.Count(got, "--run"); n != len(values) {
			t.Fatalf("harness ran %d times, want %d:\n%s", n, len(values), got)
		}
		runs := strings.Split(strings.TrimPrefix(got, "--run\n"), "--run\n")
		for i, v := range values {
			if !strings.Contains(runs[i], "ARG=-p\nARG=--model\nARG="+v+"\n") {
				t.Errorf("run %d: argv does not carry --model %q byte-identical:\n%s", i, v, runs[i])
			}
			if !strings.Contains(runs[i], "CLAUDE_CONFIG_DIR="+filepath.Join(w.cellsRoot, "scrub", "home", ".claude")+"\n") {
				t.Errorf("run %d: CLAUDE_CONFIG_DIR is not the scrubbed cell's own config dir:\n%s", i, runs[i])
			}
		}
	})

	t.Run("sequential-invocations-and-set-round-trip", func(t *testing.T) {
		w := newCLIWorld(t, "DESK_MODEL_DEFAULT=sonnet\nCELL_CADENCE=45m\n")
		plan := func(extra ...string) (string, string) {
			r, model, iv := w.deskPlan(t, nil, extra...)
			if r.Code != 0 {
				t.Fatalf("desk %v: exit %d\n%s", extra, r.Code, r.Stderr)
			}
			return model, iv
		}
		if m, iv := plan("--model", "run-a", "--cadence", "3m"); m != "run-a" || iv != "3m0s" {
			t.Fatalf("first run: %q %q", m, iv)
		}
		// the next invocation sees nothing of the previous one's flags
		if m, iv := plan(); m != "sonnet" || iv != "45m0s" {
			t.Fatalf("second run leaked state: %q %q", m, iv)
		}
		envFile := filepath.Join(w.cellDir, "cell.env")
		before, _ := os.ReadFile(envFile)
		// a dry run never writes
		if _, _ = plan("--model", "dry", "--set"); string(readFileOrEmpty(envFile)) != string(before) {
			t.Fatal("a dry-run --set wrote cell.env")
		}
		// the real --set persists, then the launch (which cannot fetch from a repo with no
		// remote, and so stops before any harness) is refused: the persistence still happened.
		r := w.run(t, nil, "desk", "example", "worker-desk", "--model", "pinned", "--cadence", "7m", "--set")
		if r.Code == 0 {
			t.Fatalf("a launch with no reachable origin must stop, got exit 0\n%s", r.Stdout)
		}
		after := string(readFileOrEmpty(envFile))
		for _, want := range []string{"DESK_MODEL_worker_desk=pinned\n", "CELL_CADENCE=7m\n"} {
			if !strings.Contains(after, want) {
				t.Errorf("--set did not persist %q:\n%s", want, after)
			}
		}
		// the persisted pin is what the next plain run resolves, and a later override does not edit it
		if m, iv := plan(); m != "pinned" || iv != "7m0s" {
			t.Errorf("after --set: model %q interval %q, want pinned 7m0s", m, iv)
		}
		persisted := string(readFileOrEmpty(envFile))
		if m, _ := plan("--model", "one-off"); m != "one-off" {
			t.Errorf("override over a persisted pin: %q", m)
		}
		if string(readFileOrEmpty(envFile)) != persisted {
			t.Error("a one-off override edited cell.env")
		}
		show := w.run(t, nil, "show", "example")
		if !strings.Contains(show.Stdout, "model worker-desk=pinned (cell.env DESK_MODEL_worker_desk)") {
			t.Errorf("show does not report the persisted pin:\n%s", show.Stdout)
		}
	})

	t.Run("nothing-to-persist-is-refused", func(t *testing.T) {
		w := newCLIWorld(t, "")
		if r := w.run(t, []string{"DRY_RUN=1"}, "desk", "example", "worker-desk", "--set"); r.Code == 0 || !strings.Contains(r.Stderr, "nothing to persist") {
			t.Errorf("--set with nothing to persist: exit %d\n%s", r.Code, r.Stderr)
		}
	})
}

func readFileOrEmpty(p string) []byte {
	b, _ := os.ReadFile(p)
	return b
}

// TestCLILegacyForms: every spelling an operator, a script or cellctl's own generated command
// line used before the migration still works. The strongest witness is the tool's own output:
// each per-role command `up` prints is re-executed verbatim and must reproduce the plan.
func TestCLILegacyForms(t *testing.T) {
	w := newCLIWorld(t, "")
	lineRE := regexp.MustCompile(`(?m)^\[dry-run\] ([a-z-]+): (.+)$`)

	t.Run("up-generated-lines-re-execute", func(t *testing.T) {
		r := w.run(t, []string{"DRY_RUN=1"}, "up", "example", "--model", "gen-model", "--cadence", "11m")
		if r.Code != 0 {
			t.Fatalf("up: exit %d\n%s", r.Code, r.Stderr)
		}
		n := 0
		for _, m := range lineRE.FindAllStringSubmatch(r.Stdout, -1) {
			role, cmdline := m[1], m[2]
			if !strings.Contains(cmdline, " desk 'example' '"+role+"' ") {
				continue // the cell line is an echo, not a window
			}
			n++
			c := exec.Command("sh", "-c", cmdline)
			c.Env = append(w.baseEnv(), "DRY_RUN=1")
			var out, errb bytes.Buffer
			c.Stdout, c.Stderr = &out, &errb
			if err := c.Run(); err != nil {
				t.Errorf("%s: generated command failed: %v\n%s", role, err, errb.String())
				continue
			}
			if !strings.Contains(out.String(), "role="+role+" model=gen-model (override)") || !strings.Contains(out.String(), "[cadence] interval=11m0s") {
				t.Errorf("%s: re-executed plan lost the model or cadence:\n%s", role, out.String())
			}
		}
		if n < 4 {
			t.Errorf("only %d generated window commands found", n)
		}
	})

	t.Run("go-style-and-leading-global", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			if !form.GoFlagOnly {
				continue
			}
			r, model, _ := w.deskPlan(t, nil, form.Args("model", "go-style")...)
			if r.Code != 0 || model != "go-style" {
				t.Errorf("%s: exit %d model %q\n%s", form.Name, r.Code, model, r.Stderr)
			}
		}
		for _, pre := range [][]string{{"--cells-root", w.cellsRoot}, {"-cells-root", w.cellsRoot}, {"--cells-root=" + w.cellsRoot}} {
			args := append(append([]string{}, pre...), "ls")
			env := []string{"CELLS_ROOT=" + filepath.Join(w.root, "elsewhere")}
			if r := w.run(t, env, args...); r.Code != 0 || !strings.Contains(r.Stdout, "example") {
				t.Errorf("%v: exit %d, cell list %q\n%s", pre, r.Code, r.Stdout, r.Stderr)
			}
		}
		// the same flag may also follow the verb
		if r := w.run(t, []string{"CELLS_ROOT=" + filepath.Join(w.root, "elsewhere")}, "ls", "--cells-root", w.cellsRoot); r.Code != 0 || !strings.Contains(r.Stdout, "example") {
			t.Errorf("ls --cells-root: exit %d %q", r.Code, r.Stdout)
		}
	})

	t.Run("raw-verbs-pass-argv-untouched", func(t *testing.T) {
		// A flag-shaped token after a raw verb belongs to the verb, never to the parser.
		for _, args := range [][]string{{"model-policy", "--bogus"}, {"model-policy", "-bogus"}, {"model-policy", "hook", "--help"}} {
			r := w.run(t, nil, args...)
			if r.Code != 2 {
				t.Errorf("%v: exit %d, want the verb's own 2\n%s", args, r.Code, r.Stderr)
			}
			if strings.Contains(r.Stderr, "unknown flag") || strings.Contains(r.Stderr, "unknown shorthand") {
				t.Errorf("%v: the parser, not the verb, rejected a token:\n%s", args, r.Stderr)
			}
		}
		if r := w.run(t, nil, "container-run", "--bogus"); strings.Contains(r.Stderr, "unknown flag") {
			t.Errorf("container-run: parser rejected a verb token:\n%s", r.Stderr)
		}
		if r := w.run(t, nil, "cache-run", "--bogus"); strings.Contains(r.Stderr, "unknown flag") {
			t.Errorf("cache-run: parser rejected a verb token:\n%s", r.Stderr)
		}
	})

	// A rejection message proves the parser stayed out; it does not prove the verb got the words.
	// This runs a child that records its own argv and requires exactly the words typed, flag-shaped
	// or not, with the selector in front of the verb in each spelling stripped and nothing else.
	t.Run("raw-verb-child-receives-every-word", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("the argv probe is a POSIX shell script")
		}
		out := filepath.Join(w.root, "raw-probe.out")
		probe := "#!/bin/sh\n{ for a in \"$@\"; do printf 'arg=%s\\n' \"$a\"; done; } > " + out + "\n"
		if err := os.WriteFile(filepath.Join(w.binDir, "raw-probe"), []byte(probe), 0o755); err != nil {
			t.Fatal(err)
		}
		words := []string{"--help", "-h", "--version", "-source", "x", "--cells-root=/nope", "--", "--model=y"}
		want := ""
		for _, a := range words {
			want += "arg=" + a + "\n"
		}
		for _, pre := range [][]string{
			nil,
			{"--cells-root", w.cellsRoot},
			{"--cells-root=" + w.cellsRoot},
			{"-cells-root", w.cellsRoot},
			{"-cells-root=" + w.cellsRoot},
		} {
			_ = os.Remove(out)
			args := append(append(append([]string{}, pre...), "cache-run", "raw-probe"), words...)
			r := w.run(t, nil, args...)
			if got := string(readFileOrEmpty(out)); r.Code != 0 || got != want {
				t.Errorf("%v: exit %d, child argv\n%s\nwant\n%s\nstderr:\n%s", pre, r.Code, got, want, r.Stderr)
			}
		}
	})

	// The same selector spellings in front of the hook: it must see its own tokens, whichever
	// spelling, and a bad token is its own refusal (2), never the parser's usage exit.
	t.Run("hook-selector-spellings", func(t *testing.T) {
		for _, pre := range [][]string{
			{"--cells-root", w.cellsRoot},
			{"--cells-root=" + w.cellsRoot},
			{"-cells-root", w.cellsRoot},
			{"-cells-root=" + w.cellsRoot},
		} {
			args := append(append([]string{}, pre...), "model-policy", "hook", "relative", "worker-desk", "anthropic", "", "claude", "abc")
			r := w.run(t, nil, args...)
			if r.Code != 2 || !strings.Contains(r.Stderr, "cell directory must be absolute: relative") {
				t.Errorf("%v: exit %d, want the hook's own refusal (2) of its first token\n%s", pre, r.Code, r.Stderr)
			}
		}
	})

	// The parser's hidden completion entrypoints are not commands of cellctl: they refuse as any
	// unknown word does (usage exit, no roster echo, nothing printed on stdout) in every position.
	t.Run("completion-entrypoints-refuse", func(t *testing.T) {
		for _, args := range [][]string{
			{"__complete", "desk", "x"}, {"__completeNoDesc", "desk"},
			{"--cells-root", w.cellsRoot, "__complete", ""}, {"--cells-root=" + w.cellsRoot, "__completeNoDesc", "d"},
		} {
			r := w.run(t, nil, args...)
			if r.Code != 3 || r.Stdout != "" || !strings.Contains(r.Stderr, "unknown command") || strings.Contains(r.Stderr, "assay-config:") {
				t.Errorf("%v: exit %d, stdout %q\n%s", args, r.Code, r.Stdout, r.Stderr)
			}
		}
	})

	// The role is the first thing `desk` checks after the cell: a bad role is refused as such
	// whatever flags follow it, as the hand-written parser ordered it.
	t.Run("desk-role-checked-before-flags", func(t *testing.T) {
		r := w.run(t, []string{"DRY_RUN=1"}, "desk", "example", "badrole", "--model", "")
		if r.Code != 3 || !strings.Contains(r.Stderr, "unknown role 'badrole'") {
			t.Errorf("exit %d, want the unknown-role refusal\n%s", r.Code, r.Stderr)
		}
	})
}

// dieRun runs fn and reports the exit code it ended with (-1 when it returned normally).
func dieRun(fn func()) (code int) {
	defer func() {
		if r := recover(); r != nil {
			ec, ok := r.(exitCode)
			if !ok {
				panic(r)
			}
			code = ec.code
		}
	}()
	fn()
	return -1
}

// TestCLIAdmissionBoundary: what the tool refuses, it refuses in the domain code, not in the
// parser. Each rejected request is rejected (1) through the command tree and (2) with the
// parser bypassed and the typed handler called directly, and the cell's files are untouched
// either way. A parse failure causes no effect at all.
func TestCLIAdmissionBoundary(t *testing.T) {
	w := newCLIWorld(t, "")
	w.installPolicy(t)
	envFile := filepath.Join(w.cellDir, "cell.env")

	type refusal struct {
		name string
		args []string   // through the command tree
		in   deskInputs // the same request with the parser bypassed
		role string
		pol  bool
	}
	cases := []refusal{
		{"the-desk-opus", []string{"--model", "opus"}, deskInputs{Model: "opus"}, "the-desk", false},
		{"unknown-harness", []string{"--harness", "bogus"}, deskInputs{Harness: "bogus"}, "worker-desk", false},
		{"unknown-cockpit", []string{"--cockpit", "bogus"}, deskInputs{Cockpit: "bogus"}, "worker-desk", false},
		{"persist-nothing", []string{"--set"}, deskInputs{Persist: true}, "worker-desk", false},
		{"persist-with-policy", []string{"--set", "--model", "x"}, deskInputs{Persist: true, Model: "x"}, "worker-desk", true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			w.writeHouseEnv(t, "")
			if c.pol {
				w.installPolicy(t)
			}
			before := string(readFileOrEmpty(envFile))
			args := append([]string{"desk", "example", c.role}, c.args...)
			r := w.run(t, []string{"DRY_RUN=1"}, args...)
			if r.Code != 3 {
				t.Errorf("through the tree: exit %d, want 3\n%s%s", r.Code, r.Stdout, r.Stderr)
			}
			if strings.Contains(r.Stdout, "[dry-run] cell=") {
				t.Errorf("a refused request still printed a launch plan:\n%s", r.Stdout)
			}
			// parser bypassed: the handler is called with the typed values directly
			t.Setenv("CELLS_ROOT", w.cellsRoot)
			t.Setenv("DRY_RUN", "1")
			t.Setenv("PATH", w.binDir+":"+os.Getenv("PATH"))
			t.Setenv("HOME", w.cellDir)
			var got int
			captureStdout(t, func() { got = dieRun(func() { cmdDesk(loadCell("example"), c.role, c.in) }) })
			if got != 3 {
				t.Errorf("parser bypassed: exit %d, want 3 (the refusal must live below the adapter)", got)
			}
			if after := string(readFileOrEmpty(envFile)); after != before {
				t.Errorf("a refused request changed cell.env:\n%s", after)
			}
		})
	}

	t.Run("override-from-environment-is-checked-too", func(t *testing.T) {
		w.writeHouseEnv(t, "")
		r := w.run(t, []string{"DRY_RUN=1", "DESK_MODEL_OVERRIDE=opus"}, "desk", "example", "the-desk")
		if r.Code != 3 {
			t.Errorf("env override of the-desk to opus: exit %d, want 3\n%s", r.Code, r.Stderr)
		}
	})

	t.Run("parse-failure-has-no-effect", func(t *testing.T) {
		w.writeHouseEnv(t, "")
		logPath := w.logStub(t)
		before := snapshotTree(t, w.root)
		for _, args := range [][]string{
			{"desk", "example", "worker-desk", "--no-such-flag"},
			{"desk", "example", "worker-desk", "--set", "--model"},
			{"up", "example", "--cadence"},
			{"smoke", "scrub", "--harness"},
			{"set", "example", "--bogus"},
			{"nosuchverb", "example"},
		} {
			r := w.run(t, []string{"DRY_RUN=1"}, args...)
			if r.Code != 3 {
				t.Errorf("%v: exit %d, want the usage exit 3\n%s", args, r.Code, r.Stderr)
			}
		}
		if _, err := os.Stat(logPath); err == nil {
			t.Error("a parse failure started a harness")
		}
		if after := snapshotTree(t, w.root); after != before {
			t.Errorf("a parse failure changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("hook-parse-failures-exit-2", func(t *testing.T) {
		for _, args := range [][]string{
			{"model-policy"}, {"model-policy", "hook"}, {"model-policy", "--bogus"},
			{"model-policy", "--cells-root", "relative"},
		} {
			if r := w.run(t, nil, args...); r.Code != 2 {
				t.Errorf("%v: exit %d, want 2 (a hook must never exit with the usage code)\n%s", args, r.Code, r.Stderr)
			}
		}
	})
}
