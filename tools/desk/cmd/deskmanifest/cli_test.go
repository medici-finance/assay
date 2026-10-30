package main

import (
	"bytes"
	"fmt"
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
// the typed handler (lintFn) replaced by a recorder, so nothing here reads a manifest tree.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskmanifest-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskmanifest")
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

// helpForms are every help and version spelling the contract names, for the lint verb.
func helpForms() [][]string {
	forms := append([][]string{}, clicontract.HelpForms("lint")...)
	forms = append(forms, clicontract.GoFlagHelpForms("lint")...)
	forms = append(forms,
		[]string{"lint", "--root", "/tmp/x", "--activation", "-h"},
		[]string{"lint", "-root", "/tmp/x", "-help"},
		[]string{"help", "version"},
	)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, clicontract.GoFlagVersionForms...)
	return append(forms, []string{"version"})
}

// recorder replaces the lint for one test.
type recorder struct {
	calls      int
	root       string
	activation bool
	verdict    int
	vals       *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevL, prevO := lintFn, onResolve
	lintFn = func(root string, activation bool) (string, int) {
		r.calls++
		r.root, r.activation = root, activation
		return "checked-clean\n", r.verdict
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { lintFn, onResolve = prevL, prevO })
	return r
}

// exec runs one in-process invocation.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

// regression: help was a first-word switch only — `deskmanifest lint -h` went to the flag
// package, which printed the usage to stderr and exited 2 — so help was not a clean, offline,
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
					t.Errorf("%v: wrote to stderr (a lint or a read ran): %q", args, r.stderr)
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
			t.Errorf("help and version: lint ran %d times; want 0", r.calls)
		}
		if code, _, _ := r.exec("lint"); code != 0 || r.calls != 1 {
			t.Errorf("a real run: exit %d, lint %d; want 0 and exactly 1", code, r.calls)
		}
	})
}

// regression: lint's settings lived on a hand-built flag set with no declared sources; every
// spelling of --root still reaches the lint byte-identical, from the flag, and nothing else is a
// source.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				args := append([]string{"lint"}, form.Args("root", pv.Value)...)
				code, _, stderr := r.exec(args...)
				if code != 0 || r.calls != 1 || r.root != pv.Value || r.vals.Source("root") != cli.Flag {
					t.Errorf("--root %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.root, r.vals.Source("root"), pv.Value, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		cases := []struct {
			name       string
			args       []string
			root       string
			rootSrc    cli.Source
			activation bool
			actSrc     cli.Source
		}{
			{"defaults", []string{"lint"}, ".", cli.Default, false, cli.Default},
			{"flags-over-defaults", []string{"lint", "--root", "/r", "--activation"}, "/r", cli.Flag, true, cli.Flag},
			{"explicit-empty-wins-over-default", []string{"lint", "--root="}, "", cli.Flag, false, cli.Default},
			{"explicit-false", []string{"lint", "--activation=false"}, ".", cli.Default, false, cli.Flag},
			{"last-wins", []string{"lint", "--root", "/a", "--root=/b"}, "/b", cli.Flag, false, cli.Default},
		}
		for _, env := range []string{"ROOT", "ACTIVATION", "DESKMANIFEST_ROOT", "DESKMANIFEST_ACTIVATION"} {
			t.Setenv(env, "1")
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.root != c.root || r.vals.Source("root") != c.rootSrc || r.activation != c.activation || r.vals.Source("activation") != c.actSrc {
				t.Errorf("%s: exit %d root %q (%s) activation %v (%s); want %q (%s) %v (%s)\n%s",
					c.name, code, r.root, r.vals.Source("root"), r.activation, r.vals.Source("activation"), c.root, c.rootSrc, c.activation, c.actSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range lintBindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskmanifest has flags and defaults only", b.Key)
			}
		}
	})
}

// regression: the first-word switch and the lint flag set are gone; every form a consumer writes
// still reaches the lint with the same values, and a command line deskmanifest cannot parse
// exits could-not-check (2) before the lint runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args       []string
			root       string
			activation bool
		}{
			{[]string{"lint"}, ".", false},
			{[]string{"lint", "--root", "."}, ".", false},
			{[]string{"lint", "--root", "/nonexistent"}, "/nonexistent", false},
			{[]string{"lint", "--root", "testdata/fixture", "--activation"}, "testdata/fixture", true},
			{[]string{"lint", "--activation"}, ".", true},
			{[]string{"lint", "--root", "../.."}, "../..", false},
			{[]string{"lint", "-root", "../..", "-activation"}, "../..", true},
			{[]string{"lint", "-root=x", "-activation=true"}, "x", true},
			{[]string{"lint", "--activation", "--root=-dash"}, "-dash", true},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.root != c.root || r.activation != c.activation || out != "checked-clean\n" {
				t.Errorf("%v: exit %d calls %d root %q activation %v stdout %q; want 0, 1, %q, %v\n%s",
					c.args, code, r.calls, r.root, r.activation, out, c.root, c.activation, stderr)
			}
		}
	})
	t.Run("verdict-codes-pass-through", func(t *testing.T) {
		for _, want := range []int{exitProblems, exitCouldNotCheck} {
			r := record(t)
			r.verdict = want
			if code, out, _ := r.exec("lint"); code != want || r.calls != 1 || out != "checked-clean\n" {
				t.Errorf("verdict %d: exit %d calls %d stdout %q; want the verdict as the exit code and the report printed", want, code, r.calls, out)
			}
		}
	})
	t.Run("refusals-before-the-lint", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "Usage:"},
			{[]string{"bogus"}, `unknown command "bogus"`},
			{[]string{"lint", "extra"}, `unknown command "extra"`},
			{[]string{"lint", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"lint", "--root"}, "flag needs an argument"},
			{[]string{"lint", "--activation=maybe"}, `invalid argument "maybe"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != exitCouldNotCheck || out != "" || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q lint %d; want 2, nothing printed, nothing run", c.args, code, out, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}, {"version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || out != versionText+"\n" || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
