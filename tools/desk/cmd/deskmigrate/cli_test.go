package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
)

// The CLI contract tests (desktools-v2/47). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the typed handler (migrateFn) replaced by a recorder, so nothing here reads a migration.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskmigrate-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskmigrate")
		if runtime.GOOS == "windows" {
			out += ".exe"
		}
		if b, err := goquiet.Cmd(exec.Command("go", "build", "-o", out, ".")).CombinedOutput(); err != nil {
			binErr = fmt.Errorf("go build: %v\n%s", err, b)
			return
		}
		binPath = out
	})
	if binErr != nil {
		t.Fatalf("building the binary under test: %v", binErr)
	}
	return binPath
}

// snapshotTree fingerprints every path under dir: name, mode, size and mtime.
func snapshotTree(dir string) string {
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

// stubPATH writes recording stubs for every program a real run could start.
func stubPATH(t *testing.T, binDir, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	for _, h := range []string{"git", "gh", "glab", "statusgen", "deskboard", "curl"} {
		script := "#!/bin/sh\necho \"RAN $0 $*\" >> '" + logPath + "'\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, h), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

type binRun struct {
	code           int
	stdout, stderr string
}

// execIn runs bin in dir with EXACTLY env as its environment.
func execIn(t *testing.T, bin, dir string, env []string, args ...string) binRun {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir, cmd.Env = dir, env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return binRun{code, out.String(), errb.String()}
}

// helpOfflineOnBinary runs every form on the built binary under each hostile config home —
// every malformed roster, and a roster and apps file it cannot read — with a PATH of recording
// stubs, and requires exit 0, stdout only, no program started and no file touched.
func helpOfflineOnBinary(t *testing.T, forms [][]string) {
	bin := toolBinary(t)
	type state struct{ name, content string }
	states := []state{}
	for _, m := range clicontract.MalformedConfigs {
		states = append(states, state{"roster-" + m.Name, m.Content})
	}
	states = append(states, state{"roster-unreadable", "unreadable"})
	for _, st := range states {
		st := st
		t.Run(st.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home, binDir, work := filepath.Join(root, "home"), filepath.Join(root, "bin"), filepath.Join(root, "work")
			logPath := filepath.Join(root, "ran.log")
			cfg := filepath.Join(home, ".config", "assay")
			for _, d := range []string{cfg, binDir, work} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stubPATH(t, binDir, logPath)
			switch st.content {
			case "":
			case "unreadable":
				for _, f := range []string{"roster.env", "apps.env"} {
					if err := os.WriteFile(filepath.Join(cfg, f), []byte("ASSAY_DESKS_ENABLED=0\n"), 0o000); err != nil {
						t.Fatal(err)
					}
				}
			default:
				if err := os.WriteFile(filepath.Join(cfg, "roster.env"), []byte(st.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"), "KUBECONFIG=/dev/null"}
			before := snapshotTree(root)
			for _, args := range forms {
				r := execIn(t, bin, work, env, args...)
				if r.code != 0 {
					t.Errorf("%v: exit %d, want 0 (stderr %q)", args, r.code, r.stderr)
				}
				if r.stderr != "" {
					t.Errorf("%v: wrote to stderr (a startup, a read or a check ran): %q", args, r.stderr)
				}
				if strings.TrimSpace(r.stdout) == "" {
					t.Errorf("%v: printed nothing", args)
				}
			}
			if b, err := os.ReadFile(logPath); err == nil {
				t.Errorf("help or version started a program:\n%s", b)
			}
			if after := snapshotTree(root); after != before {
				t.Errorf("help or version changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

// treeFlags lists every setting flag the command tree registers (help, version and the shared
// adapter's optional --perf-* attribution flags excepted), as "<command> --<flag>".
func treeFlags(root *cobra.Command) []string {
	var names []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		visit := func(f *pflag.Flag) {
			if f.Name != "help" && f.Name != "version" && !strings.HasPrefix(f.Name, "perf-") {
				names = append(names, c.Name()+" --"+f.Name)
			}
		}
		c.Flags().VisitAll(visit)
		c.PersistentFlags().VisitAll(visit)
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(root)
	sort.Strings(names)
	return names
}

// helpForms are every help and version spelling, alone and after the flags a migration takes.
// deskmigrate has no verbs, so the contract's "help <sub>" forms do not apply.
func helpForms() [][]string {
	forms := [][]string{{"-h"}, {"--help"}, {"help"}}
	for _, f := range clicontract.GoFlagHelpForms("") {
		if len(f) == 1 {
			forms = append(forms, f)
		}
	}
	forms = append(forms,
		[]string{"--from", "v1.0.0", "-h"},
		[]string{"--from", "v1.0.0", "--to", "v2.0.0", "--root", "/tmp/x", "--help"},
		[]string{"-from", "v1.0.0", "-to", "v2.0.0", "-dry-run", "-help"},
		[]string{"--from", "v1.0.0", "--to", "v2.0.0", "--version"},
	)
	forms = append(forms, clicontract.VersionForms...)
	return append(forms, clicontract.GoFlagVersionForms...)
}

// recorder replaces the runner and counts the process startup for one test.
type recorder struct {
	calls, pres int
	opts        migrateOptions
	vals        *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevM, prevO := migrateFn, onResolve
	migrateFn = func(o migrateOptions, stdout, _ io.Writer) int {
		r.calls++
		r.opts = o
		fmt.Fprintln(stdout, "migrated")
		return 0
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { migrateFn, onResolve = prevM, prevO })
	return r
}

// src is where a setting came from, or Unset when the handler was never reached.
func (r *recorder) src(key string) cli.Source {
	if r.vals == nil {
		return cli.Unset
	}
	return r.vals.Source(key)
}

// exec runs one in-process invocation with a counting startup.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(io.Writer) bool { r.pres++; return true })
	return code, out.String(), errb.String()
}

// rng is the minimum a run needs.
var rng = []string{"--from", "v1.0.0", "--to", "v2.0.0"}

// regression: the config echo and activation check ran before the flag set parsed, so
// `deskmigrate --help` read the roster and wrote to stderr, and `-h` exited 5 through the flag
// package's usage path — help was neither offline nor a clean stdout answer.
func TestCLIHelpOffline(t *testing.T) {
	forms := helpForms()
	helpOfflineOnBinary(t, forms)
	t.Run("effect-counters", func(t *testing.T) {
		r := record(t)
		for _, args := range forms {
			if code, out, _ := r.exec(args...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", args, code, out)
			}
		}
		if r.calls != 0 || r.pres != 0 {
			t.Errorf("help and version: runner %d times, startup %d times; want 0 and 0", r.calls, r.pres)
		}
		if code, _, _ := r.exec(rng...); code != 0 || r.calls != 1 || r.pres != 1 {
			t.Errorf("a real run: exit %d, runner %d, startup %d; want 0, exactly 1, exactly 1", code, r.calls, r.pres)
		}
	})
}

// regression: the five settings had no declared source and were read straight off a flag set;
// the values must reach the runner byte-identical in every spelling, and no environment variable
// may stand in for a flag.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				code, _, stderr := r.exec(append(append([]string{}, rng...), form.Args("root", pv.Value)...)...)
				if code != 0 || r.calls != 1 || r.opts.root != pv.Value || r.src("root") != cli.Flag {
					t.Errorf("--root %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.opts.root, r.src("root"), pv.Value, stderr)
				}
			}
			for _, flag := range []string{"from", "to"} {
				r := record(t)
				args := append(append([]string{}, rng...), form.Args(flag, "v9.9.9-rc.1")...)
				code, _, stderr := r.exec(args...)
				got := r.opts.from
				if flag == "to" {
					got = r.opts.to
				}
				if code != 0 || got != "v9.9.9-rc.1" || r.src(flag) != cli.Flag {
					t.Errorf("--%s %s: exit %d got %q (from %s)\n%s", flag, form.Name, code, got, r.src(flag), stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		for _, env := range []string{"FROM", "TO", "ROOT", "DRY_RUN", "NOTES", "DESKMIGRATE_FROM", "DESKMIGRATE_TO", "DESKMIGRATE_ROOT", "DESKMIGRATE_DRY_RUN", "DESKMIGRATE_NOTES", "ASSAY_ROOT"} {
			t.Setenv(env, "true")
		}
		cases := []struct {
			name    string
			args    []string
			want    migrateOptions
			rootSrc cli.Source
		}{
			{"defaults", rng, migrateOptions{from: "v1.0.0", to: "v2.0.0", root: "."}, cli.Default},
			{"flag-over-default", append(append([]string{}, rng...), "--root", "/r", "--dry-run", "--notes"), migrateOptions{"v1.0.0", "v2.0.0", "/r", true, true}, cli.Flag},
			{"explicit-empty-wins-over-default", append(append([]string{}, rng...), "--root="), migrateOptions{from: "v1.0.0", to: "v2.0.0"}, cli.Flag},
			{"last-wins", append(append([]string{}, rng...), "--root", "/a", "--root=/b", "--to", "v3.0.0"), migrateOptions{from: "v1.0.0", to: "v3.0.0", root: "/b"}, cli.Flag},
			{"bool-explicit-false", append(append([]string{}, rng...), "--dry-run", "--dry-run=false"), migrateOptions{from: "v1.0.0", to: "v2.0.0", root: "."}, cli.Default},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.opts != c.want || r.src("root") != c.rootSrc {
				t.Errorf("%s: exit %d got %+v (root from %s); want %+v (root from %s)\n%s", c.name, code, r.opts, r.src("root"), c.want, c.rootSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		declared := map[string]bool{}
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskmigrate has flags and defaults only", b.Key)
			}
			declared["deskmigrate --"+b.Flag] = true
		}
		for _, f := range treeFlags(newTree(io.Discard, io.Discard, nil)) {
			if !declared[f] {
				t.Errorf("%s is registered on the tree but not declared as a binding", f)
			}
		}
	})
}

// regression: the flag set is gone; every form a consumer writes (upgrade-assay's dry-run, then
// apply) still reaches the runner with the same values, a command line deskmigrate cannot parse
// or a missing range is refused (5) before the runner, and the runner's code passes through.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args []string
			want migrateOptions
		}{
			{[]string{"--from", "v0.12.0", "--to", "v0.13.0", "--root", "/tmp/a", "--dry-run"}, migrateOptions{"v0.12.0", "v0.13.0", "/tmp/a", true, false}},
			{[]string{"--from", "v0.12.0", "--to", "v0.13.0", "--root", "/tmp/a", "--dry-run", "--notes"}, migrateOptions{"v0.12.0", "v0.13.0", "/tmp/a", true, true}},
			{[]string{"--from", "v0.12.0", "--to", "v0.13.0", "--root", "/tmp/a"}, migrateOptions{"v0.12.0", "v0.13.0", "/tmp/a", false, false}},
			{[]string{"-from", "v0.12.0", "-to=v0.13.0", "-root", "/tmp/a", "-dry-run"}, migrateOptions{"v0.12.0", "v0.13.0", "/tmp/a", true, false}},
			{[]string{"--from=v0.12.0", "--to=v0.13.0", "--root", "-dash-dir"}, migrateOptions{"v0.12.0", "v0.13.0", "-dash-dir", false, false}},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.opts != c.want || !strings.Contains(out, "migrated") {
				t.Errorf("%v: exit %d calls %d got %+v stdout %q; want 0, 1, %+v\n%s", c.args, code, r.calls, r.opts, out, c.want, stderr)
			}
		}
	})
	t.Run("codes-pass-through", func(t *testing.T) {
		for _, want := range []int{0, 5, 6} {
			r := record(t)
			migrateFn = func(migrateOptions, io.Writer, io.Writer) int { r.calls++; return want }
			if code, _, _ := r.exec(rng...); code != want || r.calls != 1 {
				t.Errorf("runner exit %d: got exit %d calls %d; want the runner's code", want, code, r.calls)
			}
		}
	})
	t.Run("inactive-verb-before-the-runner", func(t *testing.T) {
		r := record(t)
		var out, errb bytes.Buffer
		code := execute(rng, &out, &errb, func(io.Writer) bool { r.pres++; return false })
		if code != 6 || r.pres != 1 || r.calls != 0 {
			t.Errorf("inactive verb: exit %d startup %d runner %d; want 6, 1, 0", code, r.pres, r.calls)
		}
	})
	t.Run("refusals-before-the-runner", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "--from and --to are required"},
			{[]string{"--from", "v1.0.0"}, "--from and --to are required"},
			{[]string{"--to", "v1.0.0", "--dry-run"}, "--from and --to are required"},
			{[]string{"--from=", "--to", "v1.0.0"}, "--from and --to are required"},
			{append(append([]string{}, rng...), "extra"), `unknown argument "extra"`},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"--from"}, "flag needs an argument"},
			{append(append([]string{}, rng...), "--notes=maybe"), `invalid argument "maybe"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.calls != 0 || r.pres != 0 {
				t.Errorf("%v: exit %d stdout %q runner %d startup %d; want 5, nothing printed, nothing run", c.args, code, out, r.calls, r.pres)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskmigrate sourceSHA=") || !strings.HasSuffix(out, "\n") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
