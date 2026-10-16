package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
)

// The CLI contract tests (desktools-v2/45). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the typed handlers (queueFn, flowFn) and the process startup replaced by recorders, so nothing
// here reads a forge, a roster or the kill switch.

// visibleVerbs is every subcommand the root help lists.
var visibleVerbs = []string{"walk", "html", "flow"}

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskinbox-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskinbox")
		if runtime.GOOS == "windows" {
			out += ".exe"
		}
		if b, err := exec.Command("go", "build", "-o", out, ".").CombinedOutput(); err != nil {
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

// helpForms are every help and version spelling the contract names, for every subcommand.
func helpForms() [][]string {
	forms := append([][]string{}, clicontract.HelpForms("walk")...)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, []string{"version"})
	for _, v := range visibleVerbs {
		forms = append(forms, []string{v, "--help"}, []string{v, "-h"}, []string{"help", v})
	}
	return forms
}

// same compares two string lists, treating nil and empty alike.
func same(a, b []string) bool { return len(a) == len(b) && (len(a) == 0 || reflect.DeepEqual(a, b)) }

// recorder replaces the startup, the kill switch and the typed handlers for one test.
type recorder struct {
	starts, guards, calls int
	handler               string // "queue" or "flow"
	mode, htmlOut         string
	repos                 []string
	item                  int
	roots                 []string
	since, html           string
	vals                  *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevQ, prevF, prevG, prevO := queueFn, flowFn, guard, onResolve
	queueFn = func(stdout, _ io.Writer, mode, htmlOut string, repos []string, item int, _ time.Time) int {
		r.calls++
		r.handler, r.mode, r.htmlOut, r.repos, r.item = "queue", mode, htmlOut, repos, item
		fmt.Fprintln(stdout, "queue")
		return 0
	}
	flowFn = func(stdout, _ io.Writer, roots []string, since, html string, _ func() string) int {
		r.calls++
		r.handler, r.roots, r.since, r.html = "flow", roots, since, html
		fmt.Fprintln(stdout, "flow")
		return 0
	}
	guard = func() error { r.guards++; return nil }
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { queueFn, flowFn, guard, onResolve = prevQ, prevF, prevG, prevO })
	return r
}

// exec runs one in-process invocation with the recording startup.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, time.Now(), func(io.Writer) int { r.starts++; return 0 })
	return code, out.String(), errb.String()
}

// regression: help ran after the kill switch and only as the sole argument, so `flow --help`
// under a disabled or unreadable config home was not offline.
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
					if err := os.WriteFile(filepath.Join(cfg, f), []byte("ASSAY_SCAN_REPOS=o/r\n"), 0o000); err != nil {
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
					t.Errorf("%v: wrote to stderr (a config echo or read ran): %q", args, r.stderr)
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
		if r.starts != 0 || r.guards != 0 || r.calls != 0 {
			t.Errorf("help and version: startup %d, kill switch %d, handler %d; want all 0", r.starts, r.guards, r.calls)
		}
		for _, real := range [][]string{{"o/r"}, {"flow"}} {
			r := record(t)
			if code, _, _ := r.exec(real...); code != 0 || r.starts != 1 || r.guards != 1 || r.calls != 1 {
				t.Errorf("%v: exit %d, startup %d, kill switch %d, handler %d; want 0 and exactly 1 each", real, code, r.starts, r.guards, r.calls)
			}
		}
	})
}

// regression: flow's --root and --html reached the handler only in the --flag V spelling;
// --root=V was an unknown option, and no source was declared for any setting.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, flag := range []string{"root", "html"} {
			for _, form := range clicontract.OptionForms {
				for _, pv := range clicontract.PathValues {
					r := record(t)
					code, _, stderr := r.exec(append([]string{"flow"}, form.Args(flag, pv.Value)...)...)
					if form.GoFlagOnly {
						// No caller writes the Go-flag spelling; it was an unknown option before.
						if code != 5 || r.starts != 0 || r.calls != 0 {
							t.Errorf("--%s %s/%s: exit %d, startup %d, handler %d; want the refusal 5 before anything ran", flag, form.Name, pv.Name, code, r.starts, r.calls)
						}
						continue
					}
					got := r.html
					if flag == "root" {
						got = strings.Join(r.roots, "\x00")
						if len(r.roots) != 1 {
							got = fmt.Sprintf("%q", r.roots)
						}
					}
					if code != 0 || got != pv.Value || r.vals.Source(flag) != cli.Flag {
						t.Errorf("--%s %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", flag, form.Name, pv.Name, code, got, r.vals.Source(flag), pv.Value, stderr)
					}
				}
			}
		}
		// The html subcommand's output path is a positional: byte-identical too, after -- when
		// it starts with a dash.
		for _, pv := range clicontract.PathValues {
			args := []string{"html", pv.Value, "o/r"}
			if strings.HasPrefix(pv.Value, "-") {
				args = []string{"html", "--", pv.Value, "o/r"}
			}
			r := record(t)
			if code, _, stderr := r.exec(args...); code != 0 || r.htmlOut != pv.Value || !reflect.DeepEqual(r.repos, []string{"o/r"}) {
				t.Errorf("html %s: exit %d out %q repos %q, want %q [o/r]\n%s", pv.Name, code, r.htmlOut, r.repos, pv.Value, stderr)
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		// deskinbox declares flag and (absent) default only, so the matrix's rows are the ones a
		// flag can express, plus the proof that an environment variable named like a key is never
		// read.
		cases := []struct {
			name     string
			env      []string
			args     []string
			roots    []string
			since    string
			sinceSrc cli.Source
			rootsSrc cli.Source
		}{
			{"nothing-set", nil, []string{"flow"}, nil, "", cli.Unset, cli.Unset},
			{"flag-set", nil, []string{"flow", "--root", "a", "--since", "2026-01-02"}, []string{"a"}, "2026-01-02", cli.Flag, cli.Flag},
			{"empty-flag-is-set", nil, []string{"flow", "--root=", "--since="}, []string{""}, "", cli.Flag, cli.Flag},
			{"scalar-last-wins", nil, []string{"flow", "--since", "2026-01-02", "--since", "2026-02-03"}, nil, "2026-02-03", cli.Flag, cli.Unset},
			{"array-accumulates", nil, []string{"flow", "--root", "a", "--root=b"}, []string{"a", "b"}, "", cli.Unset, cli.Flag},
			{"env-is-not-a-source", []string{"ROOT", "DESKINBOX_ROOT", "SINCE", "DESKINBOX_SINCE", "HTML", "DESKINBOX_HTML"}, []string{"flow"}, nil, "", cli.Unset, cli.Unset},
		}
		for _, c := range cases {
			r := record(t)
			for _, e := range c.env {
				t.Setenv(e, "2026-03-04")
			}
			code, _, stderr := r.exec(c.args...)
			if code != 0 || !same(r.roots, c.roots) || r.since != c.since || r.vals.Source("since") != c.sinceSrc || r.vals.Source("root") != c.rootsSrc || r.html != "" {
				t.Errorf("%s: exit %d roots %q (%s) since %q (%s) html %q; want roots %q (%s) since %q (%s)\n%s",
					c.name, code, r.roots, r.vals.Source("root"), r.since, r.vals.Source("since"), r.html, c.roots, c.rootsSrc, c.since, c.sinceSrc, stderr)
			}
		}
		for _, e := range []string{"ITEM", "DESKINBOX_ITEM", "WALK", "DESKINBOX_WALK"} {
			t.Setenv(e, "3")
		}
		r := record(t)
		if code, _, _ := r.exec("o/r"); code != 0 || r.mode != "table" || r.item != 1 {
			t.Errorf("environment named like --item/--walk: exit %d mode %q item %d, want the table untouched", code, r.mode, r.item)
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range append(queueBindings(), flowBindings()...) {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskinbox has flags only", b.Key)
			}
		}
	})
}

// regression: the hand-rolled loops are gone; every form a consumer writes still reaches the
// handler it named, and an unknown flag or a bad value is refused (exit 5) before anything runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("queue-forms", func(t *testing.T) {
		cases := []struct {
			args          []string
			mode, htmlOut string
			repos         []string
			item          int
		}{
			{nil, "table", "", nil, 1},
			{[]string{"o/r"}, "table", "", []string{"o/r"}, 1},
			{[]string{"o/r", "o2/r2"}, "table", "", []string{"o/r", "o2/r2"}, 1},
			{[]string{"o/r", "walk"}, "table", "", []string{"o/r", "walk"}, 1},
			{[]string{"walk"}, "walk", "", nil, 1},
			{[]string{"walk", "o/r"}, "walk", "", []string{"o/r"}, 1},
			{[]string{"walk", "--item", "1", "o/r"}, "walk", "", []string{"o/r"}, 1},
			{[]string{"walk", "--item", "3"}, "walk", "", nil, 3},
			{[]string{"walk", "o/r", "--item", "2"}, "walk", "", []string{"o/r"}, 2},
			{[]string{"walk", "--item=2"}, "walk", "", nil, 2},
			{[]string{"--item", "2", "o/r"}, "walk", "", []string{"o/r"}, 2},
			{[]string{"--walk", "o/r"}, "walk", "", []string{"o/r"}, 1},
			{[]string{"html", "OUT.html", "o/r"}, "html", "OUT.html", []string{"o/r"}, 1},
			{[]string{"html", "OUT.html"}, "html", "OUT.html", nil, 1},
			{[]string{"html", "OUT.html", "o/r", "--item", "2"}, "walk", "OUT.html", []string{"o/r"}, 2},
			{[]string{"--", "-x/repo"}, "table", "", []string{"-x/repo"}, 1},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.handler != "queue" || r.mode != c.mode || r.htmlOut != c.htmlOut || !same(r.repos, c.repos) || r.item != c.item {
				t.Errorf("%v: exit %d handler %q mode %q out %q repos %q item %d; want %q %q %q %d\n%s",
					c.args, code, r.handler, r.mode, r.htmlOut, r.repos, r.item, c.mode, c.htmlOut, c.repos, c.item, stderr)
			}
		}
	})
	t.Run("flow-forms", func(t *testing.T) {
		cases := []struct {
			args        []string
			roots       []string
			since, html string
		}{
			{[]string{"flow"}, nil, "", ""},
			{[]string{"flow", "o/r"}, nil, "", ""},
			{[]string{"flow", "--root", "../app", "--root", "../service"}, []string{"../app", "../service"}, "", ""},
			{[]string{"flow", "--html", "OUT.html", "--root", "a", "--since", "2026-01-02"}, []string{"a"}, "2026-01-02", "OUT.html"},
			{[]string{"flow", "--since", "2026-01-02"}, nil, "2026-01-02", ""},
			{[]string{"flow", "--root", `C:\cells\a b`, "--root", `..\cells\b`}, []string{`C:\cells\a b`, `..\cells\b`}, "", ""},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.handler != "flow" || !same(r.roots, c.roots) || r.since != c.since || r.html != c.html {
				t.Errorf("%v: exit %d handler %q roots %q since %q html %q; want %q %q %q\n%s",
					c.args, code, r.handler, r.roots, r.since, r.html, c.roots, c.since, c.html, stderr)
			}
		}
	})
	t.Run("refusals-before-effects", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"walk", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"flow", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"--root", "x"}, "unknown flag: --root"},
			{[]string{"flow", "--item", "2"}, "unknown flag: --item"},
			{[]string{"--item"}, "flag needs an argument"},
			{[]string{"flow", "--root"}, "flag needs an argument"},
			{[]string{"--item", "0"}, `deskinbox: --item must be a positive integer (got "0")`},
			{[]string{"walk", "--item", "x"}, `deskinbox: --item must be a positive integer (got "x")`},
			{[]string{"walk", "--item", "-1"}, `deskinbox: --item must be a positive integer (got "-1")`},
			{[]string{"html"}, "html needs an output path"},
			{[]string{"flow", "--since", "2026-1-2"}, `deskinbox: --since must be YYYY-MM-DD (got "2026-1-2")`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.starts != 0 || r.guards != 0 || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q startup %d guard %d handler %d; want 5, nothing printed, nothing run", c.args, code, out, r.starts, r.guards, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"version"}} {
			if code, out, _ := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskinbox sourceSHA=") || !strings.HasSuffix(out, "\n") {
				t.Errorf("%v: exit %d stdout %q, want the version line", args, code, out)
			}
		}
	})
}
