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
// the typed handler (disableFn) replaced by a recorder, so nothing here reads a manifest.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskdisable-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskdisable")
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

// helpForms are every help and version spelling, alone, after the component and after the flags
// a disable takes.
func helpForms() [][]string {
	forms := [][]string{{"-h"}, {"--help"}, {"help"}}
	for _, f := range clicontract.GoFlagHelpForms("comp") {
		forms = append(forms, f)
	}
	forms = append(forms,
		[]string{"comp", "-h"},
		[]string{"comp", "--help"},
		[]string{"comp", "--dry-run", "--root", "/tmp/x", "--help"},
		[]string{"--root", "/tmp/x", "comp", "--yes", "-h"},
		[]string{"comp", "-dry-run", "-cascade", "a,b", "-help"},
		[]string{"comp", "--version"},
	)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, clicontract.GoFlagVersionForms...)
	return append(forms, []string{"version"})
}

// recorder replaces the disable for one test.
type recorder struct {
	calls int
	opts  Options
	vals  *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevD, prevO := disableFn, onResolve
	disableFn = func(o Options, stdout, _ io.Writer) int {
		r.calls++
		r.opts = o
		fmt.Fprintln(stdout, "disable plan")
		return 0
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { disableFn, onResolve = prevD, prevO })
	return r
}

// src is where a setting came from, or Unset when the handler was never reached.
func (r *recorder) src(key string) cli.Source {
	if r.vals == nil {
		return cli.Unset
	}
	return r.vals.Source(key)
}

// exec runs one in-process invocation.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

func same(a, b Options) bool {
	return a.Root == b.Root && a.Target == b.Target && a.DryRun == b.DryRun &&
		strings.Join(a.Cascade, "\x00") == strings.Join(b.Cascade, "\x00") && len(a.Cascade) == len(b.Cascade)
}

// regression: help reached only the first word through the hand-rolled reader and went to
// stderr; `deskdisable comp --help` was a flag-set usage error (exit 5) and `--root x comp -h`
// was a dash-leading component — help was not a clean stdout answer from every position.
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
		if r.calls != 0 {
			t.Errorf("help and version: disable ran %d times; want 0", r.calls)
		}
		if code, _, _ := r.exec("comp", "--dry-run"); code != 0 || r.calls != 1 {
			t.Errorf("a real run: exit %d, disable %d; want 0 and exactly 1", code, r.calls)
		}
	})
}

// regression: --root and --cascade had no declared source and were read straight off a flag
// set; the values must reach the disable byte-identical in every spelling, and no environment
// variable may stand in for a flag.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				code, _, stderr := r.exec(append([]string{"comp", "--dry-run"}, form.Args("root", pv.Value)...)...)
				if code != 0 || r.calls != 1 || r.opts.Root != pv.Value || r.src("root") != cli.Flag {
					t.Errorf("--root %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.opts.Root, r.src("root"), pv.Value, stderr)
				}
			}
			r := record(t)
			code, _, stderr := r.exec(append([]string{"comp", "--yes"}, form.Args("cascade", "b, a,,c")...)...)
			if code != 0 || strings.Join(r.opts.Cascade, "|") != "b|a|c" || r.src("cascade") != cli.Flag {
				t.Errorf("--cascade %s: exit %d got %q (from %s), want [b a c] in order\n%s", form.Name, code, r.opts.Cascade, r.src("cascade"), stderr)
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		for _, env := range []string{"ROOT", "DRY_RUN", "YES", "CASCADE", "DESKDISABLE_ROOT", "DESKDISABLE_DRY_RUN", "DESKDISABLE_YES", "DESKDISABLE_CASCADE", "ASSAY_ROOT"} {
			t.Setenv(env, "true")
		}
		cases := []struct {
			name    string
			args    []string
			want    Options
			rootSrc cli.Source
		}{
			{"defaults", []string{"comp", "--yes"}, Options{Root: ".", Target: "comp"}, cli.Default},
			{"flag-over-default", []string{"comp", "--dry-run", "--root", "/r", "--cascade", "x"}, Options{Root: "/r", Target: "comp", Cascade: []string{"x"}, DryRun: true}, cli.Flag},
			{"explicit-empty-wins-over-default", []string{"comp", "--yes", "--root="}, Options{Root: "", Target: "comp"}, cli.Flag},
			{"last-wins", []string{"comp", "--yes", "--root", "/a", "--root=/b"}, Options{Root: "/b", Target: "comp"}, cli.Flag},
			{"bool-explicit-false", []string{"comp", "--yes", "--dry-run", "--dry-run=false"}, Options{Root: ".", Target: "comp"}, cli.Default},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || !same(r.opts, c.want) || r.src("root") != c.rootSrc {
				t.Errorf("%s: exit %d got %+v (root from %s); want %+v (root from %s)\n%s", c.name, code, r.opts, r.src("root"), c.want, c.rootSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		declared := map[string]bool{}
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskdisable has flags and defaults only", b.Key)
			}
			declared["deskdisable --"+b.Flag] = true
		}
		for _, f := range treeFlags(newTree(io.Discard, io.Discard)) {
			if !declared[f] {
				t.Errorf("%s is registered on the tree but not declared as a binding", f)
			}
		}
	})
}

// regression: the first-word component read and the flag set are gone; every form a consumer
// writes still reaches the disable with the same options, a command line deskdisable cannot
// parse is refused (5) before the disable, and its code passes through unchanged.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args []string
			want Options
		}{
			{[]string{"comp", "--dry-run"}, Options{Root: ".", Target: "comp", DryRun: true}},
			{[]string{"comp", "--yes"}, Options{Root: ".", Target: "comp"}},
			{[]string{"comp", "--root", "/tmp/t", "--dry-run"}, Options{Root: "/tmp/t", Target: "comp", DryRun: true}},
			{[]string{"comp", "--yes", "--cascade", "a,b,c"}, Options{Root: ".", Target: "comp", Cascade: []string{"a", "b", "c"}}},
			{[]string{"comp", "-root", "/tmp/t", "-dry-run", "-cascade=a"}, Options{Root: "/tmp/t", Target: "comp", Cascade: []string{"a"}, DryRun: true}},
			{[]string{"comp", "--root", "-dash-dir", "--yes"}, Options{Root: "-dash-dir", Target: "comp"}},
			{[]string{"comp", "--yes", "--cascade", ""}, Options{Root: ".", Target: "comp"}},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || !same(r.opts, c.want) || !strings.Contains(out, "disable plan") {
				t.Errorf("%v: exit %d calls %d got %+v stdout %q; want 0, 1, %+v\n%s", c.args, code, r.calls, r.opts, out, c.want, stderr)
			}
		}
	})
	// NOT legacy: the first-word reader refused a dash-leading first word, so a flag could not
	// precede the component. The shared adapter accepts it; the widening is declared on the PR.
	t.Run("declared-new-forms", func(t *testing.T) {
		for _, c := range []struct {
			args   []string
			target string
		}{
			{[]string{"--dry-run", "comp"}, "comp"},
			{[]string{"--root", "/tmp/t", "comp", "--yes"}, "comp"},
			{[]string{"--yes", "--", "-dash-comp"}, "-dash-comp"},
		} {
			r := record(t)
			if code, _, stderr := r.exec(c.args...); code != 0 || r.calls != 1 || r.opts.Target != c.target {
				t.Errorf("%v: exit %d calls %d target %q, want %q\n%s", c.args, code, r.calls, r.opts.Target, c.target, stderr)
			}
		}
	})
	t.Run("codes-pass-through", func(t *testing.T) {
		for _, want := range []int{0, 5, 6} {
			r := record(t)
			disableFn = func(Options, io.Writer, io.Writer) int { r.calls++; return want }
			if code, _, _ := r.exec("comp", "--dry-run"); code != want || r.calls != 1 {
				t.Errorf("disable exit %d: got exit %d calls %d; want the disable's code", want, code, r.calls)
			}
		}
	})
	t.Run("refusals-before-the-disable", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "Usage:"},
			{[]string{"--dry-run"}, "expected a component id"},
			{[]string{"comp"}, "refusing to run without --dry-run or --yes"},
			{[]string{"comp", "--root", "/r"}, "refusing to run without --dry-run or --yes"},
			{[]string{"comp", "extra", "--yes"}, "unexpected extra argument(s) [extra]"},
			{[]string{"comp", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"comp", "--root"}, "flag needs an argument"},
			{[]string{"comp", "--yes=maybe"}, `invalid argument "maybe"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q disable %d; want 5, nothing printed, nothing run", c.args, code, out, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}, {"version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskdisable sourceSHA=") || !strings.HasSuffix(out, "\n") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
