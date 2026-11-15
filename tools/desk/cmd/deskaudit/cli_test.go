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
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLI contract tests (desktools-v2/47). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the kill switch and both verbs (guardFn, recoverFn, tailFn) replaced by a recorder, so nothing
// here touches an audit log.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskaudit-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskaudit")
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

// helpForms are every help and version spelling, for the root and each verb.
func helpForms() [][]string {
	var forms [][]string
	for _, sub := range []string{"recover", "tail"} {
		forms = append(forms, clicontract.HelpForms(sub)...)
		forms = append(forms, clicontract.GoFlagHelpForms(sub)...)
	}
	forms = append(forms,
		[]string{"tail", "5", "--help"},
		[]string{"tail", "5", "-h"},
		[]string{"help", "version"},
	)
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, clicontract.GoFlagVersionForms...)
	return append(forms, []string{"version"})
}

// recorder replaces the kill switch and both verbs for one test.
type recorder struct {
	guards, recovers, tails int
	n                       int
	guardErr                error
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevG, prevR, prevT := guardFn, recoverFn, tailFn
	guardFn = func() error { r.guards++; return r.guardErr }
	recoverFn = func(stdout, _ io.Writer) int { r.recovers++; fmt.Fprintln(stdout, "recovered"); return 0 }
	tailFn = func(n int, stdout, _ io.Writer) int { r.tails++; r.n = n; fmt.Fprintln(stdout, "tailed"); return 0 }
	t.Cleanup(func() { guardFn, recoverFn, tailFn = prevG, prevR, prevT })
	return r
}

func (r *recorder) effects() int { return r.guards + r.recovers + r.tails }

// exec runs one in-process invocation.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

// regression: help went to stderr through the hand-rolled dispatcher, and `deskaudit tail --help`
// was a bad N (exit 5) — help was not a clean stdout answer from every position.
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
		if r.effects() != 0 {
			t.Errorf("help and version: guard %d, recover %d, tail %d; want none", r.guards, r.recovers, r.tails)
		}
		if code, _, _ := r.exec("tail"); code != 0 || r.guards != 1 || r.tails != 1 {
			t.Errorf("a real read: exit %d, guard %d, tail %d; want 0, exactly 1, exactly 1", code, r.guards, r.tails)
		}
	})
}

// regression: N was parsed by hand after the kill switch; it is now tail's one positional
// argument, validated at parse time, and deskaudit declares no other setting — no flag, no
// environment variable, no configuration key.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("positional-n", func(t *testing.T) {
		cases := []struct {
			args []string
			n    int
		}{
			{[]string{"tail"}, 10},
			{[]string{"tail", "7"}, 7},
			{[]string{"tail", "--", "7"}, 7},
			{[]string{"tail", "007"}, 7},
			{[]string{"tail", "1"}, 1},
			{[]string{"tail", "10000"}, 10000},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.tails != 1 || r.n != c.n || !strings.Contains(out, "tailed") {
				t.Errorf("%v: exit %d tail %d n %d; want 0, 1, %d\n%s", c.args, code, r.tails, r.n, c.n, stderr)
			}
		}
	})
	t.Run("environment-ignored", func(t *testing.T) {
		for _, env := range []string{"N", "TAIL", "DESKAUDIT_N", "DESKAUDIT_TAIL", "ASSAY_AUDIT_TAIL"} {
			t.Setenv(env, "3")
		}
		r := record(t)
		if code, _, _ := r.exec("tail"); code != 0 || r.n != 10 {
			t.Errorf("tail with N-shaped environment set: exit %d n %d; want the default 10", code, r.n)
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		if names := treeFlags(newTree(io.Discard, io.Discard)); len(names) != 0 {
			t.Errorf("deskaudit declares settings %v; it has none beyond help and version", names)
		}
	})
}

// regression: the hand-rolled dispatcher is gone; every form a consumer writes still reaches the
// same verb after the kill switch, and a command line deskaudit cannot parse — an unknown verb, a
// bad N — is refused (5) before the kill switch or the verb runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		for _, c := range []struct {
			args            []string
			recovers, tails int
			out             string
		}{
			{[]string{"recover"}, 1, 0, "recovered"},
			{[]string{"tail"}, 0, 1, "tailed"},
			{[]string{"tail", "50"}, 0, 1, "tailed"},
		} {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.guards != 1 || r.recovers != c.recovers || r.tails != c.tails || !strings.Contains(out, c.out) {
				t.Errorf("%v: exit %d guard %d recover %d tail %d stdout %q\n%s", c.args, code, r.guards, r.recovers, r.tails, out, stderr)
			}
		}
	})
	t.Run("kill-switch-first", func(t *testing.T) {
		for _, args := range [][]string{{"recover"}, {"tail"}, {"tail", "3"}} {
			r := record(t)
			r.guardErr = deskkit.Disabled("kill switch armed")
			code, out, stderr := r.exec(args...)
			if code != 3 || r.guards != 1 || r.recovers+r.tails != 0 || out != "" || !strings.Contains(stderr, "kill switch armed") {
				t.Errorf("%v disabled: exit %d guard %d verb %d stdout %q stderr %q; want 3, 1, 0", args, code, r.guards, r.recovers+r.tails, out, stderr)
			}
		}
	})
	t.Run("codes-pass-through", func(t *testing.T) {
		for _, want := range []int{0, 5, 6} {
			r := record(t)
			recoverFn = func(io.Writer, io.Writer) int { r.recovers++; return want }
			tailFn = func(int, io.Writer, io.Writer) int { r.tails++; return want }
			for _, args := range [][]string{{"recover"}, {"tail"}} {
				if code, _, _ := r.exec(args...); code != want {
					t.Errorf("%v returning %d: exit %d; want the verb's code", args, want, code)
				}
			}
		}
	})
	t.Run("refusals-before-the-kill-switch", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "Usage:"},
			{[]string{"frob"}, `unknown command "frob"`},
			{[]string{"recover", "x"}, "recover: takes no arguments"},
			{[]string{"tail", "0"}, "N must be a positive integer"},
			{[]string{"tail", "abc"}, "N must be a positive integer"},
			{[]string{"tail", "10001"}, "N is capped at 10000"},
			{[]string{"tail", "1", "2"}, "takes at most one argument"},
			{[]string{"tail", "-1"}, "unknown shorthand flag"},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"tail", "--bogus"}, "unknown flag: --bogus"},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.effects() != 0 {
				t.Errorf("%v: exit %d stdout %q guard %d verbs %d; want 5, nothing printed, nothing run", c.args, code, out, r.guards, r.recovers+r.tails)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}, {"version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskaudit sourceSHA=") || !strings.HasSuffix(out, "\n") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
