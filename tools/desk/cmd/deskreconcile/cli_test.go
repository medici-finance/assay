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
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLI contract tests (desktools-v2/44). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the startup, the kill switch, the loop-identity check and the reconcile cycle replaced by
// recorders, so nothing here fetches, commits or opens a PR.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskreconcile-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskreconcile")
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

// stubPATH writes recording stubs for every program a real run would start.
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
// deskreconcile has no subcommands, so the contract's "help <sub>" forms do not apply.
func helpForms() [][]string {
	return [][]string{
		{"-h"}, {"--help"}, {"-help"}, {"help"},
		{"--worktree", "/tmp/wt", "-h"},
		{"-root", ".", "-worktree", "/tmp/wt", "-dry-run", "--help"},
		{"--dry-run", "-help"},
		{"--version"}, {"-version"},
	}
}

// recorder replaces every effect a run has, for one test.
type recorder struct {
	starts, guards, loops, calls int
	got                          Options
	src                          map[string]string
	runErr                       error
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevG, prevL, prevR, prevO := guard, loopIdentity, reconcileFn, onResolve
	guard = func() error { r.guards++; return nil }
	loopIdentity = func(string) error { r.loops++; return nil }
	reconcileFn = func(o Options) (Result, error) {
		r.calls++
		r.got = o
		r.got.Now = r.got.Now.Truncate(0)
		return Result{NoOp: true, DryRun: o.DryRun}, r.runErr
	}
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range bindings() {
			r.src[b.Key] = v.Source(b.Key).String()
		}
	}
	t.Cleanup(func() { guard, loopIdentity, reconcileFn, onResolve = prevG, prevL, prevR, prevO })
	return r
}

func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(io.Writer) int { r.starts++; return 0 })
	return code, out.String(), errb.String()
}

// want is the recorded Options with the clock removed.
type want struct {
	root, worktree, branch, repo, tokenFile string
	issue                                   int
	dryRun                                  bool
}

func (r *recorder) view() want {
	o := r.got
	return want{o.Root, o.Worktree, o.Branch, o.Repo, o.TokenFile, o.Issue, o.DryRun}
}

// regression: the kill switch, the effective-config echo and the activation check all ran
// before the hand-rolled parser, so even -h read the roster and printed to stderr.
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
			env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
				"KUBECONFIG=/dev/null"}
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
		if r.starts != 0 || r.guards != 0 || r.loops != 0 || r.calls != 0 {
			t.Errorf("help and version: startup %d, guard %d, loop identity %d, reconcile %d; want all 0", r.starts, r.guards, r.loops, r.calls)
		}
		if code, _, stderr := r.exec("--worktree", "/tmp/wt", "--dry-run"); code != 0 || r.starts != 1 || r.guards != 1 || r.loops != 1 || r.calls != 1 {
			t.Errorf("a real run: exit %d, startup %d, guard %d, loop identity %d, reconcile %d; want 0 and exactly 1 each\n%s",
				code, r.starts, r.guards, r.loops, r.calls, stderr)
		}
	})
}

// regression: the flags lived on a hand-built flag set with no declared sources; every
// setting is flag-or-default, and no environment variable named like a key is read.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, flag := range []string{"root", "worktree", "token-file"} {
				for _, pv := range clicontract.PathValues {
					r := record(t)
					code, _, stderr := r.exec(form.Args(flag, pv.Value)...)
					got := map[string]string{"root": r.got.Root, "worktree": r.got.Worktree, "token-file": r.got.TokenFile}[flag]
					if code != 0 || got != pv.Value || r.src[flag] != "flag" {
						t.Errorf("--%s %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", flag, form.Name, pv.Name, code, got, r.src[flag], pv.Value, stderr)
					}
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
			want want
			src  map[string]string
		}{
			{"defaults", nil, want{root: ".", branch: defaultBranch},
				map[string]string{"root": "default", "branch": "default", "issue": "default", "dry-run": "default", "worktree": "unset", "repo": "unset", "token-file": "unset"}},
			{"flags", []string{"--root", "r", "--branch", "b", "--issue", "9", "--repo", "o/n", "--dry-run"},
				want{root: "r", branch: "b", repo: "o/n", issue: 9, dryRun: true},
				map[string]string{"root": "flag", "branch": "flag", "issue": "flag", "dry-run": "flag", "repo": "flag"}},
			{"explicit-empty-wins-over-default", []string{"--root=", "--branch="}, want{},
				map[string]string{"root": "flag", "branch": "flag"}},
			{"explicit-false", []string{"--dry-run=false"}, want{root: ".", branch: defaultBranch},
				map[string]string{"dry-run": "flag"}},
		}
		for _, env := range []string{"ROOT", "WORKTREE", "BRANCH", "ISSUE", "REPO", "TOKEN_FILE", "DRY_RUN", "DESKRECONCILE_ROOT", "DESKRECONCILE_DRY_RUN"} {
			t.Setenv(env, "env-value")
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.view() != c.want {
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
				t.Errorf("binding %q declares an environment, config or secret source; deskreconcile is flag-or-default only", b.Key)
			}
		}
	})
}

// regression: the flag package's single-dash spellings were the documented form; every one of
// them still reaches the reconcile cycle with the same values, and a command line deskreconcile
// cannot parse still exits 5 before the kill switch or anything else runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		base := want{root: ".", worktree: "/tmp/wt", branch: defaultBranch}
		with := func(f func(*want)) want { w := base; f(&w); return w }
		cases := []struct {
			args []string
			want want
		}{
			{[]string{"--worktree", "/tmp/wt"}, base},
			{[]string{"-worktree", "/tmp/wt"}, base},
			{[]string{"-worktree=/tmp/wt"}, base},
			{[]string{"--root", "/r", "--worktree", "/tmp/wt", "--branch", "board/reconcile", "--issue", "12", "--dry-run"},
				with(func(w *want) { w.root, w.issue, w.dryRun = "/r", 12, true })},
			{[]string{"-root", "/r", "-worktree", "/tmp/wt", "-branch", "b", "-issue", "12", "-dry-run"},
				with(func(w *want) { w.root, w.branch, w.issue, w.dryRun = "/r", "b", 12, true })},
			{[]string{"-root=/r", "-worktree=/tmp/wt", "-issue=12", "-dry-run=true", "-repo=o/n", "-token-file=/t"},
				with(func(w *want) { w.root, w.issue, w.dryRun, w.repo, w.tokenFile = "/r", 12, true, "o/n", "/t" })},
			{[]string{"-worktree", "/tmp/wt", "-dry-run=false"}, base},
			{[]string{"-worktree", "/tmp/wt", "-dry-run=1"}, with(func(w *want) { w.dryRun = true })},
			// The flag package read --issue as a base-prefixed integer.
			{[]string{"-worktree", "/tmp/wt", "-issue", "0x10"}, with(func(w *want) { w.issue = 16 })},
			{[]string{"-worktree", "/tmp/wt", "-issue", "-3"}, with(func(w *want) { w.issue = -3 })},
			// Repeated flags: the last one wins.
			{[]string{"-worktree", "/a", "-worktree", "/tmp/wt"}, base},
			// A dash-leading value is the flag's value, never a flag.
			{[]string{"-worktree", "-wt"}, with(func(w *want) { w.worktree = "-wt" })},
			// Everything after -- is ignored.
			{[]string{"-worktree", "/tmp/wt", "--"}, base},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.view() != c.want {
				t.Errorf("%v: exit %d calls %d got %+v; want 0, 1, %+v\n%s", c.args, code, r.calls, r.view(), c.want, stderr)
			}
		}
	})
	t.Run("paths", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := record(t)
			if code, _, _ := r.exec("-worktree", pv.Value, "-root", pv.Value); code != 0 || r.got.Worktree != pv.Value || r.got.Root != pv.Value {
				t.Errorf("%s: exit %d got %q/%q, want it byte-identical", pv.Name, code, r.got.Worktree, r.got.Root)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}} {
			code, out, stderr := r.exec(args...)
			if code != 0 || !strings.HasPrefix(out, "deskreconcile sourceSHA=") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q", args, code, out, stderr)
			}
		}
	})
	t.Run("run-errors-keep-their-code", func(t *testing.T) {
		r := record(t)
		r.runErr = deskkit.Refused("worktree must be absolute")
		if code, _, stderr := r.exec("-worktree", "wt"); code != deskkit.ExitRefused || !strings.Contains(stderr, "worktree must be absolute") {
			t.Errorf("a refused cycle: exit %d stderr %q, want 5 and the reason", code, stderr)
		}
	})
	t.Run("refusals-exit-5", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{"extra"}, "takes no positional arguments"},
			{[]string{"-worktree", "/tmp/wt", "extra"}, "takes no positional arguments"},
			{[]string{"--", "help"}, "takes no positional arguments"},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"-bogus"}, "in -bogus"},
			{[]string{"--worktree"}, "flag needs an argument"},
			{[]string{"-issue", "x"}, `invalid argument "x"`},
			{[]string{"-dry-run=maybe"}, `invalid argument "maybe"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != deskkit.ExitRefused || out != "" || r.starts != 0 || r.guards != 0 || r.loops != 0 || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q startup %d guard %d loop %d reconcile %d; want 5 with nothing run",
					c.args, code, out, r.starts, r.guards, r.loops, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
}
