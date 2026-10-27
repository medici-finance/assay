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
)

// The CLI contract tests (desktools-v2/45). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the typed handler (readFn) and the process startup replaced by recorders, so nothing here
// reads a forge or a roster.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskread-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskread")
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

// helpForms are every help and version spelling the contract names, for every read kind.
func helpForms() [][]string {
	forms := append([][]string{}, clicontract.HelpForms("issues")...)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, []string{"version"})
	for _, k := range sortedKinds() {
		forms = append(forms, []string{k, "--help"}, []string{k, "-h"}, []string{"help", k})
	}
	return forms
}

// recorder replaces the startup and the typed handler for one test.
type recorder struct {
	starts, calls int
	ciAtStart     bool
	kind          string
	o             readOpts
	vals          *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevR, prevO := readFn, onResolve
	readFn = func(kind string, o readOpts, stdout, _ io.Writer) int {
		r.calls++
		r.kind, r.o = kind, o
		fmt.Fprintln(stdout, "{}")
		return 0
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { readFn, onResolve = prevR, prevO })
	return r
}

// exec runs one in-process invocation with the recording startup.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(_ io.Writer, ci bool) int { r.starts++; r.ciAtStart = ci; return 0 })
	return code, out.String(), errb.String()
}

// regression: help went to stderr with exit 5 after the roster echo and activation check (main
// did both before parsing), so help under a malformed or unreadable config home was not offline.
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
		if r.starts != 0 || r.calls != 0 {
			t.Errorf("help and version: startup %d, handler %d; want both 0", r.starts, r.calls)
		}
		if code, _, _ := r.exec("issues", "--repo", "o/r"); code != 0 || r.starts != 1 || r.calls != 1 {
			t.Errorf("a real run: exit %d, startup %d, handler %d; want 0 and exactly 1 each", code, r.starts, r.calls)
		}
	})
}

// regression: --label and --state reached the handler only in the --flag V spelling;
// --label=V was an unknown flag, and no source was declared for any setting.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, flag := range []string{"label", "state"} {
			for _, form := range clicontract.OptionForms {
				for _, pv := range clicontract.PathValues {
					r := record(t)
					args := append([]string{"issue-list", "--repo", "o/r"}, form.Args(flag, pv.Value)...)
					code, _, stderr := r.exec(args...)
					if form.GoFlagOnly {
						// No caller writes the Go-flag spelling; it was an unknown flag before.
						if code != 5 || r.starts != 0 || r.calls != 0 {
							t.Errorf("--%s %s/%s: exit %d, startup %d, handler %d; want the refusal 5 before anything ran", flag, form.Name, pv.Name, code, r.starts, r.calls)
						}
						continue
					}
					got, set := r.o.label, r.o.labelSet
					if flag == "state" {
						got, set = r.o.state, r.o.stateSet
					}
					if code != 0 || !set || got != pv.Value || r.vals.Source(flag) != cli.Flag {
						t.Errorf("--%s %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", flag, form.Name, pv.Name, code, got, r.vals.Source(flag), pv.Value, stderr)
					}
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		// deskread declares flag and default only (the max-parallel default is the one default
		// it has), so the matrix's rows are the ones those two sources express, plus the proof
		// that an environment variable named like a key is never read.
		cases := []struct {
			name   string
			env    []string
			args   []string
			par    int
			parSrc cli.Source
			label  string
			lblSet bool
		}{
			{"nothing-set", nil, []string{"issue-list", "--repo", "o/r"}, 6, cli.Default, "", false},
			{"flag-over-default", nil, []string{"issue-list", "--repo", "o/r", "--max-parallel", "2", "--label", "bug"}, 2, cli.Flag, "bug", true},
			{"empty-label-is-set", nil, []string{"issue-list", "--repo", "o/r", "--label="}, 6, cli.Default, "", true},
			{"scalar-last-wins", nil, []string{"issue-list", "--repo", "o/r", "--max-parallel", "2", "--max-parallel=3"}, 3, cli.Flag, "", false},
			{"env-is-not-a-source", []string{"REPO", "DESKREAD_REPO", "MAX_PARALLEL", "DESKREAD_MAX_PARALLEL", "LABEL", "DESKREAD_LABEL", "STATE"},
				[]string{"issue-list", "--repo", "o/r"}, 6, cli.Default, "", false},
		}
		for _, c := range cases {
			r := record(t)
			for _, e := range c.env {
				t.Setenv(e, "9")
			}
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.o.maxParallel != c.par || r.vals.Source("max-parallel") != c.parSrc || r.o.label != c.label || r.o.labelSet != c.lblSet ||
				!reflect.DeepEqual(r.o.repos, []string{"o/r"}) {
				t.Errorf("%s: exit %d max-parallel %d (%s) label %q set %v repos %q; want %d (%s) %q %v [o/r]\n%s",
					c.name, code, r.o.maxParallel, r.vals.Source("max-parallel"), r.o.label, r.o.labelSet, r.o.repos, c.par, c.parSrc, c.label, c.lblSet, stderr)
			}
		}
		// An explicitly empty --max-parallel beats the default and is then refused as a value.
		r := record(t)
		if code, _, stderr := r.exec("issues", "--repo", "o/r", "--max-parallel="); code != 5 || r.starts != 0 || !strings.Contains(stderr, `bad --max-parallel "" — expected a positive integer`) {
			t.Errorf("--max-parallel=: exit %d startup %d stderr %q; want the value refusal", code, r.starts, stderr)
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskread has flags and defaults only", b.Key)
			}
		}
	})
}

// regression: the hand-rolled parseFlags is gone; every form a consumer writes still reaches
// the kind it named, and an unknown flag or a bad value is refused (exit 5) before startup.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args     []string
			kind     string
			repos    []string
			issues   []issueTarget
			changes  []issueTarget
			state    string
			ci       bool
			parallel int
		}{
			{[]string{"issues", "--repo", "o/r"}, "issues", []string{"o/r"}, nil, nil, "", false, 6},
			{[]string{"issues", "--repo", "o/r", "--repo", "o2/r2"}, "issues", []string{"o/r", "o2/r2"}, nil, nil, "", false, 6},
			{[]string{"issues", "--repo", " o/r ", "--repo=o/r"}, "issues", []string{"o/r"}, nil, nil, "", false, 6},
			{[]string{"comments", "--issue", "o/r#5"}, "comments", nil, []issueTarget{{Repo: "o/r", Number: 5}}, nil, "", false, 6},
			{[]string{"label-events", "--issue", "o/r#3"}, "label-events", nil, []issueTarget{{Repo: "o/r", Number: 3}}, nil, "", false, 6},
			{[]string{"comments", "--change", "o/r#7"}, "comments", nil, nil, []issueTarget{{Repo: "o/r", Number: 7}}, "", false, 6},
			{[]string{"trust", "--issue", "o/r#1", "--issue", "o/r#2", "--issue", "o/r#1"}, "trust", nil, []issueTarget{{Repo: "o/r", Number: 1}, {Repo: "o/r", Number: 2}}, nil, "", false, 6},
			// Flags written before the kind now reach it (the hand-rolled reader refused them).
			{[]string{"--repo", "o/r", "issues"}, "issues", []string{"o/r"}, nil, nil, "", false, 6},
			{[]string{"changes", "--repo", "o/r", "--state", " merged "}, "changes", []string{"o/r"}, nil, nil, "merged", false, 6},
			{[]string{"issues", "--ci-workflow-token", "--repo", "o/r"}, "issues", []string{"o/r"}, nil, nil, "", true, 6},
			{[]string{"issues", "--repo", "o/r", "--max-parallel", "2"}, "issues", []string{"o/r"}, nil, nil, "", false, 2},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.kind != c.kind || !reflect.DeepEqual(r.o.repos, c.repos) || !reflect.DeepEqual(r.o.issues, c.issues) ||
				!reflect.DeepEqual(r.o.changes, c.changes) || r.o.state != c.state || r.o.ci != c.ci || r.ciAtStart != c.ci || r.o.maxParallel != c.parallel {
				t.Errorf("%v: exit %d kind %q repos %q issues %v changes %v state %q ci %v/%v par %d\n%s",
					c.args, code, r.kind, r.o.repos, r.o.issues, r.o.changes, r.o.state, r.o.ci, r.ciAtStart, r.o.maxParallel, stderr)
			}
		}
	})
	t.Run("refusals-before-startup", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "Usage:"},
			{[]string{"bogus"}, `refused: unknown read kind "bogus" — the kind set is closed`},
			{[]string{"bogus-kind"}, `refused: unknown read kind "bogus-kind"`},
			{[]string{"bogus-kind", "--issue", "o/r#3"}, "unknown flag: --issue"},
			{[]string{"issues", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"issues", "--repo"}, "flag needs an argument"},
			{[]string{"issues", "--repo", "o/r", "extra"}, `unknown command "extra"`},
			{[]string{"issues", "--repo", "not-a-slug"}, `deskread: refused: bad --repo "not-a-slug" — expected owner/name`},
			{[]string{"issue", "--issue", "o/r#0"}, "the number must be a positive integer"},
			{[]string{"issue", "--issue", "o/r"}, `bad --issue "o/r" — expected owner/name#N`},
			{[]string{"changes", "--repo", "o/r", "--state", "merged", "--state", "all"}, "deskread: refused: --state needs exactly one value"},
			{[]string{"issue-list", "--repo", "o/r", "--label", "a", "--label", "b"}, "deskread: refused: --label needs exactly one value"},
			{[]string{"issues", "--repo", "o/r", "--max-parallel", "0"}, `bad --max-parallel "0" — expected a positive integer`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.starts != 0 || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q startup %d handler %d; want 5, nothing printed, nothing run", c.args, code, out, r.starts, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"version"}} {
			if code, out, _ := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskread sourceSHA=") || !strings.HasSuffix(out, "\n") {
				t.Errorf("%v: exit %d stdout %q, want the version line", args, code, out)
			}
		}
	})
}

// regression: F-label-events-kind — label-events is a real per-issue kind: --issue addresses it, and
// a repo or change address is refused by the kind's own addressing check (the CLI table above
// stubs the read, so this test calls the check the read runs).
func TestLabelEventsAddressing(t *testing.T) {
	issue := []issueTarget{{Repo: "o/r", Number: 3}}
	o := readOpts{issues: issue}
	if err := checkAddressing("label-events", &o); err != nil || len(o.items) != 1 {
		t.Errorf("label-events --issue: err %v items %v; want it addressed", err, o.items)
	}
	for _, c := range []struct {
		name string
		o    readOpts
		want string
	}{
		{"repo", readOpts{repos: []string{"o/r"}}, `--repo does not address the "label-events" kind`},
		{"change", readOpts{changes: issue}, `--change does not address the "label-events" kind`},
	} {
		o := c.o
		if err := checkAddressing("label-events", &o); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("label-events with %s: err %v; want it to contain %q", c.name, err, c.want)
		}
	}
}
