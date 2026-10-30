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

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
)

// The CLI contract tests (desktools-v2/46). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the typed handler (pinsFn) replaced by a recorder, so nothing here reads a pin file.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskpins-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskpins")
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
	for _, h := range []string{"git", "gh", "glab", "statusgen", "deskboard"} {
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

// helpForms are every help and version spelling, alone and after the flags a run takes.
// deskpins has no verbs, so the contract's "help <sub>" forms do not apply.
func helpForms() [][]string {
	forms := [][]string{{"-h"}, {"--help"}, {"help"}}
	for _, f := range clicontract.GoFlagHelpForms("") {
		if len(f) == 1 {
			forms = append(forms, f)
		}
	}
	forms = append(forms,
		[]string{"--check", "-h"},
		[]string{"--check", "--root", "/tmp/x", "--help"},
		[]string{"-check", "-root", "/tmp/x", "-help"},
	)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, clicontract.GoFlagVersionForms...)
	return append(forms, []string{"version"})
}

// recorder replaces the check for one test.
type recorder struct {
	calls int
	root  string
	vals  *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevP, prevO := pinsFn, onResolve
	pinsFn = func(root string, stdout, _ io.Writer) int {
		r.calls++
		r.root = root
		fmt.Fprintln(stdout, "checked-clean")
		return 0
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { pinsFn, onResolve = prevP, prevO })
	return r
}

// exec runs one in-process invocation.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

// regression: help went to stderr through the hand-rolled reader and only as the first word —
// `deskpins --check -h` was an unknown argument (exit 5) — so help was not a clean, offline,
// stdout answer from every position.
func TestCLIHelpOffline(t *testing.T) {
	bin := toolBinary(t)
	forms := helpForms()
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
					t.Errorf("%v: wrote to stderr (a check or a read ran): %q", args, r.stderr)
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

	t.Run("effect-counters", func(t *testing.T) {
		r := record(t)
		for _, args := range forms {
			if code, out, _ := r.exec(args...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", args, code, out)
			}
		}
		if r.calls != 0 {
			t.Errorf("help and version: check ran %d times; want 0", r.calls)
		}
		if code, _, _ := r.exec("--check"); code != 0 || r.calls != 1 {
			t.Errorf("a real run: exit %d, check %d; want 0 and exactly 1", code, r.calls)
		}
	})
}

// regression: --root reached the check only in the `--root <dir>` spelling; `--root=<dir>` was an
// unknown argument, and no source was declared for either setting.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				args := append([]string{"--check"}, form.Args("root", pv.Value)...)
				code, _, stderr := r.exec(args...)
				if code != 0 || r.calls != 1 || r.root != pv.Value || r.vals.Source("root") != cli.Flag {
					t.Errorf("--root %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.root, r.vals.Source("root"), pv.Value, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		cases := []struct {
			name    string
			args    []string
			root    string
			rootSrc cli.Source
		}{
			{"default", []string{"--check"}, ".", cli.Default},
			{"flag-over-default", []string{"--check", "--root", "/r"}, "/r", cli.Flag},
			{"explicit-empty-wins-over-default", []string{"--check", "--root="}, "", cli.Flag},
			{"last-wins", []string{"--check", "--root", "/a", "--root=/b"}, "/b", cli.Flag},
		}
		for _, env := range []string{"ROOT", "CHECK", "DESKPINS_ROOT", "DESKPINS_CHECK", "ASSAY_ROOT"} {
			t.Setenv(env, "/env-value")
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.root != c.root || r.vals.Source("root") != c.rootSrc || r.vals.Source("check") != cli.Flag {
				t.Errorf("%s: exit %d root %q (from %s), check from %s; want %q (%s), check from the flag\n%s",
					c.name, code, r.root, r.vals.Source("root"), r.vals.Source("check"), c.root, c.rootSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskpins has flags and defaults only", b.Key)
			}
		}
	})
}

// regression: the hand-rolled argument loop is gone; every form a consumer writes still reaches
// the check with the same root, and a command line deskpins cannot parse is refused (5) before
// the check runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args []string
			root string
		}{
			{[]string{"--check"}, "."},
			{[]string{"--check", "--root", "/tmp/av"}, "/tmp/av"},
			{[]string{"--root", "/tmp/av", "--check"}, "/tmp/av"},
			{[]string{"--check", "--root", "-dash-dir"}, "-dash-dir"},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.root != c.root || !strings.Contains(out, "checked-clean") {
				t.Errorf("%v: exit %d calls %d root %q stdout %q; want 0, 1, %q\n%s", c.args, code, r.calls, r.root, out, c.root, stderr)
			}
		}
	})
	// NOT legacy: the hand-rolled loop refused every one of these with 5 ("unknown argument").
	// The shared adapter's parsing accepts them, and the widening is declared on the PR
	// (Desk-decided) rather than passed off as a preserved spelling.
	t.Run("declared-new-forms", func(t *testing.T) {
		cases := []struct {
			args []string
			root string
		}{
			{[]string{"-check", "-root", "/tmp/av"}, "/tmp/av"},
			{[]string{"--check=true", "--root=../.."}, "../.."},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.root != c.root || !strings.Contains(out, "checked-clean") {
				t.Errorf("%v: exit %d calls %d root %q stdout %q; want 0, 1, %q\n%s", c.args, code, r.calls, r.root, out, c.root, stderr)
			}
		}
	})
	t.Run("verdict-codes-pass-through", func(t *testing.T) {
		for _, want := range []int{exitCheckedFailed, exitCouldNotCheck, exitMalformed} {
			r := record(t)
			pinsFn = func(string, io.Writer, io.Writer) int { r.calls++; return want }
			if code, _, _ := r.exec("--check"); code != want || r.calls != 1 {
				t.Errorf("verdict %d: exit %d calls %d; want the verdict as the exit code", want, code, r.calls)
			}
		}
	})
	t.Run("refusals-before-the-check", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "Usage:"},
			{[]string{"--root", "/tmp/av"}, "nothing to do — pass --check"},
			{[]string{"extra"}, `unknown argument "extra"`},
			{[]string{"--check", "extra"}, `unknown argument "extra"`},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"--check", "--root"}, "flag needs an argument"},
			{[]string{"--check=maybe"}, `invalid argument "maybe"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q check %d; want 5, nothing printed, nothing run", c.args, code, out, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}, {"version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskpins sourceSHA=") || !strings.HasSuffix(out, "\n") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
