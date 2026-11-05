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
// the typed handler (rebaselineFn) and the process startup replaced by recorders, so nothing
// here reads a brief or runs git.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskrebaseline-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskrebaseline")
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

// helpForms are every help and version spelling, alone and after the arguments a run takes.
// deskrebaseline has no verbs, so the contract's "help <sub>" forms do not apply.
func helpForms() [][]string {
	forms := [][]string{{"-h"}, {"--help"}, {"help"}}
	for _, f := range clicontract.GoFlagHelpForms("") {
		if len(f) == 1 {
			forms = append(forms, f)
		}
	}
	forms = append(forms,
		[]string{"verify-integrity/05", "-h"},
		[]string{"verify-integrity/05", "--row", "2", "--open", "--help"},
		[]string{"-row", "2", "-root", "/tmp/x", "-help"},
	)
	forms = append(forms, clicontract.VersionForms...)
	return append(forms, clicontract.GoFlagVersionForms...)
}

// recorder replaces the handler and the startup for one test.
type recorder struct {
	calls, pres int
	got         options
	vals        *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevR, prevO := rebaselineFn, onResolve
	rebaselineFn = func(o options) int {
		r.calls++
		r.got = o
		fmt.Fprintln(stdout, "verdict: recorded")
		return 0
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { rebaselineFn, onResolve = prevR, prevO })
	return r
}

// exec runs one in-process invocation with a counting startup.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(io.Writer) { r.pres++ })
	return code, out.String(), errb.String()
}

// regression: help went to stderr and only as the first word — `deskrebaseline <brief> -h` was
// an unknown flag (exit 5) — so help was not a clean, offline, stdout answer from every position.
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
					t.Errorf("%v: wrote to stderr (the startup or a run happened): %q", args, r.stderr)
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
		if r.calls != 0 || r.pres != 0 {
			t.Errorf("help and version: handler ran %d times, startup %d; want 0 and 0", r.calls, r.pres)
		}
		if code, _, _ := r.exec("s/01", "--row", "1"); code != 0 || r.calls != 1 || r.pres != 1 {
			t.Errorf("a real run: exit %d, handler %d, startup %d; want 0, exactly 1, exactly 1", code, r.calls, r.pres)
		}
	})
}

// regression: --root and --repo reached the handler only in the two spellings the hand-rolled
// reader knew, and no source was declared for any setting.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				args := append([]string{"s/01", "--row", "1"}, form.Args("root", pv.Value)...)
				code, _, stderr := r.exec(args...)
				if code != 0 || r.calls != 1 || r.got.root != pv.Value || r.vals.Source("root") != cli.Flag {
					t.Errorf("--root %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.got.root, r.vals.Source("root"), pv.Value, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		cases := []struct {
			name    string
			args    []string
			repo    string
			repoSrc cli.Source
		}{
			{"default", []string{"s/01", "--row", "1"}, "", cli.Default},
			{"flag-over-default", []string{"s/01", "--row", "1", "--repo", "o/n"}, "o/n", cli.Flag},
			{"explicit-empty", []string{"s/01", "--row", "1", "--repo="}, "", cli.Flag},
			{"last-wins", []string{"s/01", "--row", "1", "--repo", "a/b", "--repo=c/d"}, "c/d", cli.Flag},
		}
		for _, env := range []string{"ROW", "ROOT", "REPO", "OPEN", "DESKREBASELINE_REPO", "DESKREBASELINE_OPEN", "ASSAY_REPO"} {
			t.Setenv(env, "env-value")
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.got.repo != c.repo || r.vals.Source("repo") != c.repoSrc || r.got.open || r.vals.Source("open") != cli.Default || r.got.row != 1 {
				t.Errorf("%s: exit %d got %+v (repo from %s, open from %s); want repo %q (%s), open false from the default\n%s",
					c.name, code, r.got, r.vals.Source("repo"), r.vals.Source("open"), c.repo, c.repoSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskrebaseline has flags and defaults only", b.Key)
			}
		}
	})
}

// regression: the hand-rolled argument loop is gone; every form a consumer writes still reaches
// the handler with the same options, and a command line the verb cannot use is refused (5)
// before the startup or the handler runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args []string
			want options
		}{
			{[]string{"verify-integrity/05", "--row", "3"}, options{brief: "verify-integrity/05", row: 3}},
			{[]string{"docs/streams/s/01-x.md", "--row=2", "--root", "/r", "--repo", "o/n"}, options{brief: "docs/streams/s/01-x.md", row: 2, root: "/r", repo: "o/n"}},
			{[]string{"--row", "4", "s/02", "--open"}, options{brief: "s/02", row: 4, open: true}},
			{[]string{"s/02", "--row", "1", "--root=/r", "--repo=o/n", "--open"}, options{brief: "s/02", row: 1, root: "/r", repo: "o/n", open: true}},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.got != c.want || !strings.Contains(out, "verdict:") {
				t.Errorf("%v: exit %d calls %d got %+v stdout %q; want 0, 1, %+v\n%s", c.args, code, r.calls, r.got, out, c.want, stderr)
			}
		}
	})
	t.Run("handler-codes-pass-through", func(t *testing.T) {
		for _, want := range []int{5, 6} {
			r := record(t)
			rebaselineFn = func(options) int { r.calls++; return want }
			if code, _, _ := r.exec("s/01", "--row", "1"); code != want || r.calls != 1 {
				t.Errorf("handler %d: exit %d calls %d; want the handler's code", want, code, r.calls)
			}
		}
	})
	t.Run("refusals", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "Usage:"},
			{[]string{"s/01"}, "--row K is required"},
			{[]string{"--row", "1"}, "a <brief> (path or <stream>/<NN>) is required"},
			{[]string{"s/01", "--row", "0"}, `--row must be a positive integer, got "0"`},
			{[]string{"s/01", "--row", "-2"}, `--row must be a positive integer, got "-2"`},
			{[]string{"s/01", "--row", "abc"}, `invalid argument "abc"`},
			{[]string{"s/01", "--row"}, "flag needs an argument"},
			{[]string{"s/01", "s/02", "--row", "1"}, `unexpected extra argument "s/02" (one <brief> only)`},
			{[]string{"s/01", "--row", "1", "--bogus"}, "unknown flag: --bogus"},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.calls != 0 || r.pres != 0 {
				t.Errorf("%v: exit %d stdout %q handler %d startup %d; want 5, nothing printed, nothing run", c.args, code, out, r.calls, r.pres)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskrebaseline sourceSHA=") || !strings.HasSuffix(out, "\n") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
