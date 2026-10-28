package main

import (
	"bytes"
	"fmt"
	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
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
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
)

// The CLI contract tests (desktools-v2/45). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the typed handler (dispatchFn) and the process startup replaced by recorders, so nothing here
// reads a forge, a roster or the kill switch.

// visibleVerbs is every lane the root help lists.
var visibleVerbs = []string{"board", "issues", "intake"}

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "issueboard-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "issueboard")
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

// helpForms are every help and version spelling the contract names, for every lane.
func helpForms() [][]string {
	forms := append([][]string{}, clicontract.HelpForms("issues")...)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, []string{"version"})
	for _, v := range visibleVerbs {
		forms = append(forms, []string{v, "--help"}, []string{v, "-h"}, []string{"help", v})
	}
	return forms
}

// recorder replaces the startup, the kill switch and the typed handler for one test.
type recorder struct {
	starts, guards, calls int
	root, to              string
	sla                   int
	sub                   string
	src                   map[string]string
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevD, prevG, prevO := dispatchFn, guard, onResolve
	dispatchFn = func(sub, root string, _ time.Time, sla int, to string, _ ...deskkit.RequestObserver) (*Report, error) {
		r.calls++
		r.sub, r.root, r.sla, r.to = sub, root, sla, to
		return &Report{render: func(w io.Writer) { fmt.Fprintln(w, "board") }, detail: "stub"}, nil
	}
	guard = func() error { r.guards++; return nil }
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range bindings() {
			r.src[b.Key] = v.Source(b.Key).String()
		}
	}
	t.Cleanup(func() { dispatchFn, guard, onResolve = prevD, prevG, prevO })
	return r
}

// exec runs one in-process invocation with the recording startup.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(io.Writer) int { r.starts++; return 0 })
	return code, out.String(), errb.String()
}

// regression: help ran after the roster echo and activation check (the hand-rolled main did
// both before parsing), so help under a malformed or unreadable config home was not offline.
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
		if code, _, _ := r.exec("issues"); code != 0 || r.starts != 1 || r.guards != 1 || r.calls != 1 {
			t.Errorf("a real run: exit %d, startup %d, kill switch %d, handler %d; want 0 and exactly 1 each", code, r.starts, r.guards, r.calls)
		}
	})
}

// regression: --root reached the handler only in the --root V spelling; --root=V was an unknown
// lane word, and no source was declared for any setting.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				code, _, stderr := r.exec(append(form.Args("root", pv.Value), "issues")...)
				if form.GoFlagOnly {
					// No caller writes the Go-flag spelling; it was a refused lane word before.
					if code != 5 || r.starts != 0 || r.calls != 0 {
						t.Errorf("%s/%s: exit %d, startup %d, handler %d; want the refusal 5 before anything ran", form.Name, pv.Name, code, r.starts, r.calls)
					}
					continue
				}
				if code != 0 || r.root != pv.Value || r.src["root"] != "flag" {
					t.Errorf("%s/%s: exit %d root %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.root, r.src["root"], pv.Value, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		// issueboard declares flag and default only, so the matrix's rows are the ones those two
		// sources can express, plus the proof that an environment variable named like a key is
		// never read.
		cases := []struct {
			name          string
			env           []string
			args          []string
			root, rootSrc string
			sla           int
			to, toSrc     string
		}{
			{"nothing-set", nil, []string{"issues"}, ".", "default", escalateSLADays, "", "unset"},
			{"flag-over-default", nil, []string{"--root", "flg", "--sla-days", "2", "--to", "worker", "issues"}, "flg", "flag", 2, "worker", "flag"},
			{"empty-flag-beats-default", nil, []string{"--root=", "--to=", "issues"}, "", "flag", escalateSLADays, "", "flag"},
			{"scalar-last-wins", nil, []string{"--root", "a", "--root", "b", "issues"}, "b", "flag", escalateSLADays, "", "unset"},
			{"env-is-not-a-source", []string{"ROOT", "ISSUEBOARD_ROOT", "SLA_DAYS", "ISSUEBOARD_SLA_DAYS", "TO", "ISSUEBOARD_TO"}, []string{"issues"}, ".", "default", escalateSLADays, "", "unset"},
		}
		for _, c := range cases {
			r := record(t)
			for _, e := range c.env {
				t.Setenv(e, "from-env")
				if strings.Contains(e, "SLA") {
					t.Setenv(e, "1")
				}
			}
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.root != c.root || r.src["root"] != c.rootSrc || r.sla != c.sla || r.to != c.to || r.src["to"] != c.toSrc {
				t.Errorf("%s: exit %d root %q (%s) sla %d to %q (%s); want root %q (%s) sla %d to %q (%s)\n%s",
					c.name, code, r.root, r.src["root"], r.sla, r.to, r.src["to"], c.root, c.rootSrc, c.sla, c.to, c.toSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; issueboard has flags and defaults only", b.Key)
			}
		}
	})
}

// regression: the hand-rolled loop is gone; every form a consumer writes still reaches the lane
// it named, and an unknown flag or a bad value is refused (exit 5) before anything is read.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args    []string
			sub, to string
			root    string
			sla     int
		}{
			{nil, "board", "", ".", escalateSLADays},
			{[]string{"board"}, "board", "", ".", escalateSLADays},
			{[]string{"issues"}, "issues", "", ".", escalateSLADays},
			{[]string{"intake"}, "intake", "", ".", escalateSLADays},
			{[]string{"issues", "--to", "worker"}, "issues", "worker", ".", escalateSLADays},
			{[]string{"issues", "--to=worker"}, "issues", "worker", ".", escalateSLADays},
			// Flags before the lane word (the package tests' own form) and after it.
			{[]string{"--root", "R", "--sla-days", "3", "--to", "worker", "issues"}, "issues", "worker", "R", 3},
			{[]string{"issues", "--root", "R", "--sla-days", "3"}, "issues", "", "R", 3},
			{[]string{"--root", "R", "intake", "--sla-days=0"}, "intake", "", "R", 0},
			// Words after the lane were ignored and still are; after -- they are words.
			{[]string{"issues", "extra"}, "issues", "", ".", escalateSLADays},
			{[]string{"issues", "--", "--sla-days"}, "issues", "", ".", escalateSLADays},
			// Atoi semantics kept: a leading zero is decimal.
			{[]string{"--sla-days", "010", "board"}, "board", "", ".", 10},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.sub != c.sub || r.to != c.to || r.root != c.root || r.sla != c.sla {
				t.Errorf("%v: exit %d sub %q to %q root %q sla %d; want %q %q %q %d\n%s", c.args, code, r.sub, r.to, r.root, r.sla, c.sub, c.to, c.root, c.sla, stderr)
			}
		}
	})
	t.Run("paths", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := record(t)
			if code, _, _ := r.exec("--root", pv.Value, "intake"); code != 0 || r.root != pv.Value {
				t.Errorf("%s: exit %d root %q, want %q byte-identical", pv.Name, code, r.root, pv.Value)
			}
		}
	})
	t.Run("refusals-before-effects", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"issues", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"-root", "R"}, "unknown shorthand flag"},
			{[]string{"--root"}, "flag needs an argument"},
			{[]string{"bogus"}, `refused: unknown subcommand "bogus" (see --help)`},
			{[]string{"--root", "R", "bogus"}, `refused: unknown subcommand "bogus"`},
			{[]string{"--sla-days", "-1"}, `refused: --sla-days must be a non-negative integer, got "-1"`},
			{[]string{"--sla-days", "x", "issues"}, `refused: --sla-days must be a non-negative integer, got "x"`},
			{[]string{"--to", "worker"}, "refused: --to <role> filters the `issues` lane only; it has no meaning for `board`"},
			{[]string{"intake", "--to", "worker"}, "has no meaning for `intake`"},
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
			if code, out, _ := r.exec(args...); code != 0 || !strings.HasPrefix(out, "issueboard sourceSHA=") || !strings.HasSuffix(out, "\n") {
				t.Errorf("%v: exit %d stdout %q, want the version line", args, code, out)
			}
		}
	})
}
