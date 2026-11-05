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

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLI contract tests (desktools-v2/44). Help and version are exercised on the BUILT binary
// with a hostile config home, an armed kill switch and a PATH of recording stubs; the value
// tests run in-process with the kill switch and the collector replaced by recorders, so nothing
// here reads a transcript or writes a day-file.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "opmetrics-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "opmetrics")
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
// opmetrics has no subcommands, so the contract's "help <sub>" forms do not apply.
func helpForms() [][]string {
	return [][]string{
		{"-h"}, {"--help"}, {"-help"}, {"help"},
		{"--root", ".", "-h"},
		{"-root", ".", "-date", "2026-07-22", "--help"},
		{"--stdout", "-help"},
		{"--version"}, {"-version"},
	}
}

// recorder replaces the kill switch and the collector for one test.
type recorder struct {
	guards, calls int
	got           options
	src           map[string]string
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevG, prevC, prevO := guard, collectFn, onResolve
	guard = func() error { r.guards++; return nil }
	collectFn = func(o options, _, _ io.Writer) int {
		r.calls++
		r.got = o
		return deskkit.ExitOK
	}
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range bindings() {
			r.src[b.Key] = v.Source(b.Key).String()
		}
	}
	t.Cleanup(func() { guard, collectFn, onResolve = prevG, prevC, prevO })
	return r
}

func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// view is the recorded options without the clock-derived fields.
type view struct {
	root, date, deskTools, ghFixture string
	transcripts                      []string
	trend                            int
	toStdout                         bool
}

func (r *recorder) view() view {
	o := r.got
	return view{o.root, o.date, o.deskTools, o.ghFixture, o.transcripts, o.trend, o.toStdout}
}

// base pins the clock so date-derived values are stable.
var base = []string{"--tz", "UTC", "--now", "2026-07-22T12:00:00Z"}

func args(extra ...string) []string { return append(append([]string{}, base...), extra...) }

// regression: help went to stderr and only a lone -h was help; -h after a flag was a
// parse refusal (exit 5), and the kill switch could turn a help request into a refusal.
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
					if err := os.WriteFile(filepath.Join(cfg, f), []byte("X=1\n"), 0o000); err != nil {
						t.Fatal(err)
					}
				}
			default:
				if err := os.WriteFile(filepath.Join(cfg, "roster.env"), []byte(st.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			// The kill switch is armed: help and version never consult it.
			env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
				"KUBECONFIG=/dev/null", "DESK_TOOLS_DISABLED=1"}
			before := snapshotTree(root)
			for _, a := range forms {
				r := execIn(t, bin, work, env, a...)
				if r.code != 0 {
					t.Errorf("%v: exit %d, want 0 (stderr %q)", a, r.code, r.stderr)
				}
				if r.stderr != "" {
					t.Errorf("%v: wrote to stderr: %q", a, r.stderr)
				}
				if strings.TrimSpace(r.stdout) == "" {
					t.Errorf("%v: printed nothing to stdout", a)
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
		for _, a := range forms {
			if code, out, _ := r.exec(a...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", a, code, out)
			}
		}
		if r.guards != 0 || r.calls != 0 {
			t.Errorf("help and version: guard %d, collect %d; want both 0", r.guards, r.calls)
		}
		if code, _, stderr := r.exec(args("--stdout")...); code != 0 || r.guards != 1 || r.calls != 1 {
			t.Errorf("a real run: exit %d, guard %d, collect %d; want 0 and exactly 1 each\n%s", code, r.guards, r.calls, stderr)
		}
	})
}

// regression: the flags lived on a hand-built flag set (with a home-made repeatable type for
// --transcripts) and no declared sources; every setting is flag-or-default.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, flag := range []string{"root", "desk-tools", "gh-fixture", "transcripts"} {
				for _, pv := range clicontract.PathValues {
					r := record(t)
					code, _, stderr := r.exec(append(args("--stdout"), form.Args(flag, pv.Value)...)...)
					got := map[string]string{"root": r.got.root, "desk-tools": r.got.deskTools, "gh-fixture": r.got.ghFixture}[flag]
					if flag == "transcripts" {
						got = strings.Join(r.got.transcripts, "|")
					}
					if code != 0 || got != pv.Value || r.src[flag] != "flag" {
						t.Errorf("--%s %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", flag, form.Name, pv.Name, code, got, r.src[flag], pv.Value, stderr)
					}
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		for _, env := range []string{"ROOT", "DATE", "TREND", "STDOUT", "TRANSCRIPTS", "GH_FIXTURE", "OPMETRICS_ROOT", "OPMETRICS_TREND"} {
			t.Setenv(env, "7")
		}
		cases := []struct {
			name string
			args []string
			want view
			src  map[string]string
		}{
			{"defaults", args("--stdout"), view{date: "2026-07-22", toStdout: true},
				map[string]string{"trend": "default", "stdout": "flag", "root": "unset", "transcripts": "unset", "gh-json": "unset"}},
			{"flags", args("--root", "r", "--date", "2026-07-01", "--trend", "3", "--desk-tools", "d"),
				view{root: "r", date: "2026-07-01", deskTools: "d", trend: 3},
				map[string]string{"trend": "flag", "root": "flag", "date": "flag", "stdout": "default"}},
			// --gh-json is the alias: it fills --gh-fixture only when that is unset or empty.
			{"alias-alone", args("--stdout", "--gh-json", "a.json"), view{date: "2026-07-22", ghFixture: "a.json", toStdout: true}, nil},
			{"fixture-over-alias", args("--stdout", "--gh-fixture", "f.json", "--gh-json", "a.json"), view{date: "2026-07-22", ghFixture: "f.json", toStdout: true}, nil},
			// --transcripts is repeatable and never split on a comma.
			{"repeatable", args("--stdout", "--transcripts", "a,b", "--transcripts", "c"),
				view{date: "2026-07-22", transcripts: []string{"a,b", "c"}, toStdout: true}, map[string]string{"transcripts": "flag"}},
			{"explicit-false", args("--root", "r", "--stdout=false"), view{root: "r", date: "2026-07-22"}, map[string]string{"stdout": "flag"}},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || !reflect.DeepEqual(r.view(), c.want) {
				t.Errorf("%s: exit %d got %+v, want %+v\n%s", c.name, code, r.view(), c.want, stderr)
			}
			for k, s := range c.src {
				if r.src[k] != s {
					t.Errorf("%s: %s came from %s, want %s", c.name, k, r.src[k], s)
				}
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if len(b.Env) != 0 || b.Config || b.Secret {
				t.Errorf("binding %q declares an environment, config or secret source; opmetrics is flag-or-default only", b.Key)
			}
		}
	})
}

// regression: the documented forms and the flag package's single-dash spellings were the only
// parser; every one still reaches the collector with the same values, and a command line
// opmetrics cannot parse still exits 5 before the kill switch runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		readme := view{root: ".", date: "2026-07-22", ghFixture: "/tmp/opm-gh.json", trend: 7}
		cases := []struct {
			args []string
			want view
		}{
			// The README's documented invocation.
			{[]string{"--root", ".", "--date", "2026-07-22", "--gh-fixture", "/tmp/opm-gh.json", "--trend", "7"}, readme},
			{[]string{"-root", ".", "-date", "2026-07-22", "-gh-fixture", "/tmp/opm-gh.json", "-trend", "7"}, readme},
			{[]string{"-root=.", "-date=2026-07-22", "-gh-fixture=/tmp/opm-gh.json", "-trend=7"}, readme},
			{[]string{"--root=.", "--date=2026-07-22", "--gh-json=/tmp/opm-gh.json", "--trend=7"}, readme},
			{[]string{"-root", ".", "-date", "2026-07-22", "-gh-json", "/tmp/opm-gh.json", "-trend", "0x7"}, readme},
			{[]string{"-stdout", "-date", "2026-07-22", "-transcripts", "/a", "-transcripts=/b"},
				view{date: "2026-07-22", transcripts: []string{"/a", "/b"}, toStdout: true}},
			{[]string{"-stdout=true", "-date", "2026-07-22", "-desk-tools", "-state"},
				view{date: "2026-07-22", deskTools: "-state", toStdout: true}},
			// Words after the flags, and everything after --, are ignored.
			{[]string{"--root", ".", "--date", "2026-07-22", "--gh-fixture", "/tmp/opm-gh.json", "--trend", "7", "extra"}, readme},
			{[]string{"--root", ".", "--date", "2026-07-22", "--gh-fixture", "/tmp/opm-gh.json", "--trend", "7", "--", "--trend", "9"}, readme},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(append(append([]string{}, base...), c.args...)...)
			if code != 0 || r.calls != 1 || !reflect.DeepEqual(r.view(), c.want) {
				t.Errorf("%v: exit %d calls %d got %+v; want 0, 1, %+v\n%s", c.args, code, r.calls, r.view(), c.want, stderr)
			}
		}
	})
	t.Run("paths", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := record(t)
			if code, _, _ := r.exec(args("-root", pv.Value, "-transcripts", pv.Value)...); code != 0 || r.got.root != pv.Value || len(r.got.transcripts) != 1 || r.got.transcripts[0] != pv.Value {
				t.Errorf("%s: exit %d got %q/%q, want it byte-identical", pv.Name, code, r.got.root, r.got.transcripts)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, a := range [][]string{{"--version"}, {"-version"}} {
			code, out, stderr := r.exec(a...)
			if code != 0 || !strings.HasPrefix(out, "opmetrics sourceSHA=") || !strings.Contains(out, " classifier=") || !strings.Contains(out, " schema=") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q", a, code, out, stderr)
			}
		}
	})
	t.Run("refusals-exit-5", func(t *testing.T) {
		cases := []struct {
			args   []string
			msg    string
			guards int
		}{
			{[]string{"--bogus"}, "unknown flag: --bogus", 0},
			{[]string{"-bogus"}, "in -bogus", 0},
			{[]string{"--root"}, "flag needs an argument", 0},
			{[]string{"-trend", "x"}, `invalid argument "x"`, 0},
			{[]string{"-stdout=maybe"}, `invalid argument "maybe"`, 0},
			// Value refusals run after the kill switch, as before.
			{[]string{"--stdout", "--tz", "Not/AZone"}, "--tz is not a known IANA location", 1},
			{[]string{"--stdout", "--now", "yesterday"}, "--now is not RFC3339", 1},
			{[]string{"--stdout", "--date", "22-07-2026"}, "--date is not YYYY-MM-DD", 1},
			{[]string{"--date", "2026-07-22"}, "--root is required (or pass --stdout)", 1},
			{[]string{"--stdout", "--trend", "-1"}, "--trend must be >= 0", 1},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != deskkit.ExitRefused || out != "" || r.guards != c.guards || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q guard %d collect %d; want 5, guard %d, nothing collected", c.args, code, out, r.guards, r.calls, c.guards)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
}
