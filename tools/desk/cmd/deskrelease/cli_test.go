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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLI contract tests (desktools-v2/46). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the cut handler (cutFn) and the process startup replaced by recorders on top of the package's
// fake-GitHub harness, and the refusal tests run the REAL cut parser through that harness, so
// nothing here reaches a network.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskrelease-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskrelease")
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

// helpForms are every help and version spelling deskrelease answers, root and nested. The
// tool never had Go flag-package spellings, so the single-dash long forms do not apply.
func helpForms() [][]string {
	forms := clicontract.HelpForms("cut")
	forms = append(forms, []string{"cut", "-h"}, []string{"cut", "--help"}, []string{"-v"}, []string{"version"})
	return append(forms, clicontract.VersionForms...)
}

// recorder replaces the cut handler and the startup for one test, on the package harness.
type recorder struct {
	h           *harness
	calls, pres int
	argv        []string
}

func record(t *testing.T) *recorder {
	t.Helper()
	r := &recorder{h: newHarness(t)}
	prevC, prevRepo := cutFn, repoSlug
	cutFn = func(argv []string) int {
		r.calls++
		r.argv = append([]string(nil), argv...)
		fmt.Fprintln(stdout, "deskrelease: cut recorded")
		return 0
	}
	t.Cleanup(func() { cutFn, repoSlug = prevC, prevRepo })
	return r
}

// exec runs one in-process invocation with a counting startup.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(io.Writer) bool { r.pres++; return true })
	return code, out.String(), errb.String()
}

// regression: help went to stderr, `deskrelease cut --help` was an audited refusal and the
// roster startup (class, config echo, activation check) ran before even `version` — so help
// was not a clean, offline, stdout answer.
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
					t.Errorf("%v: wrote to stderr (the startup or a cut ran): %q", args, r.stderr)
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
		if r.calls != 0 || r.pres != 0 || len(r.h.gh.calls()) != 0 || len(r.h.tok.all()) != 0 || len(r.h.auditLines(t)) != 0 {
			t.Errorf("help and version: cut %d, startup %d, api %v, mint %v, audit %d; want nothing",
				r.calls, r.pres, r.h.gh.calls(), r.h.tok.all(), len(r.h.auditLines(t)))
		}
		if code, _, _ := r.exec("cut", goodTag); code != 0 || r.calls != 1 || r.pres != 1 {
			t.Errorf("a real cut: exit %d, cut %d, startup %d; want 0, exactly 1, exactly 1", code, r.calls, r.pres)
		}
	})
}

// regression: deskrelease declares no setting at all — the release home comes from the roster
// file only — and the migration must not give it one: every word after `cut` reaches the cut
// parser verbatim, and no environment variable stands in for argv.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		words := []string{goodTag}
		for _, pv := range clicontract.PathValues {
			words = append(words, pv.Value)
		}
		for _, w := range words {
			for _, argv := range [][]string{{"cut", w}, {"cut", w, "--dry-run"}, {"cut", "--dry-run", w}} {
				r := record(t)
				code, _, stderr := r.exec(argv...)
				if code != 0 || r.calls != 1 || strings.Join(r.argv, "\x00") != strings.Join(argv, "\x00") {
					t.Errorf("%q: exit %d calls %d reached the cut parser as %q; want it verbatim\n%s", argv, code, r.calls, r.argv, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		for _, k := range []string{"TAG", "RELEASE_TAG", "DESKRELEASE_TAG", "DESKRELEASE_REPO", "ASSAY_RELEASE_REPO", "DRY_RUN", "DESKRELEASE_DRY_RUN"} {
			t.Setenv(k, "attacker/elsewhere")
		}
		r := record(t)
		repoSlug = defaultReleaseRepo
		code, _, stderr := r.exec("cut", goodTag)
		if code != 0 || strings.Join(r.argv, " ") != "cut "+goodTag || repoSlug != defaultReleaseRepo {
			t.Errorf("exit %d argv %q repo %q; want argv only and the shipped release home (the environment is never read)\n%s", code, r.argv, repoSlug, stderr)
		}
		r = record(t)
		if code, _, _ := r.exec("cut"); code != 0 || strings.Join(r.argv, " ") != "cut" {
			t.Errorf("bare cut: exit %d argv %q; want [cut] with nothing taken from the environment", code, r.argv)
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		var walk func(c *cobra.Command)
		var names []string
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
		walk(newTree(io.Discard, io.Discard, nil))
		if len(names) != 0 {
			t.Errorf("deskrelease declares settings %v; it has none (no --repo, no --sha, no environment)", names)
		}
	})
}

// regression: the hand-rolled dispatcher is gone; every form a consumer writes still reaches the
// one cut parser, which still refuses (5, audited, nothing minted, no API call) every word it does
// not recognise, while an unknown verb or root flag stays a usage error (2).
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		for _, argv := range [][]string{{"cut", goodTag}, {"cut", goodTag, "--dry-run"}, {"cut", "--dry-run", goodTag}} {
			r := record(t)
			code, out, stderr := r.exec(argv...)
			if code != 0 || r.calls != 1 || strings.Join(r.argv, " ") != strings.Join(argv, " ") || !strings.Contains(out, "cut recorded") {
				t.Errorf("%v: exit %d calls %d argv %q; want 0, 1, verbatim\n%s", argv, code, r.calls, r.argv, stderr)
			}
		}
	})
	t.Run("handler-codes-pass-through", func(t *testing.T) {
		for _, want := range []int{deskkit.ExitRateLimited, deskkit.ExitRefused, deskkit.ExitUnverifiable} {
			r := record(t)
			cutFn = func([]string) int { r.calls++; return want }
			if code, _, _ := r.exec("cut", goodTag); code != want || r.calls != 1 {
				t.Errorf("handler %d: exit %d calls %d; want the handler's code", want, code, r.calls)
			}
		}
	})
	t.Run("cut-refusals-stay-with-the-cut-parser", func(t *testing.T) {
		cases := []struct {
			argv []string
			msg  string
		}{
			{[]string{"cut", goodTag, "extra"}, `extra operand "extra"`},
			{[]string{"cut", goodTag, "--help"}, `unrecognised flag "--help"`},
			{[]string{"cut", goodTag, "-h"}, `unrecognised flag "-h"`},
			{[]string{"cut", goodTag, "--version"}, `unrecognised flag "--version"`},
			{[]string{"cut", "--", goodTag}, `unrecognised flag "--"`},
			{[]string{"cut", goodTag, "--repo", "attacker/elsewhere"}, `unrecognised flag "--repo"`},
			{[]string{"cut", goodTag, "--dry-run", "--dry-run"}, "--dry-run given twice"},
			{[]string{"cut", "-dry-run", goodTag}, `unrecognised flag "-dry-run"`},
			// A word before the verb is carried into cut's argv, where the cut parser refuses it.
			{[]string{"--repo", "attacker/elsewhere", "cut", goodTag}, `unrecognised flag "--repo"`},
		}
		for _, c := range cases {
			h := newHarness(t)
			code := run(c.argv)
			if code != deskkit.ExitRefused {
				t.Errorf("%q: exit %d, want 5 (refused)\n%s", c.argv, code, h.errb.String())
			}
			if h.out.Len() != 0 || len(h.gh.calls()) != 0 || len(h.tok.all()) != 0 {
				t.Errorf("%q: stdout %q api %v mint %v; want nothing printed, called or minted", c.argv, h.out.String(), h.gh.calls(), h.tok.all())
			}
			if !strings.Contains(h.errb.String(), c.msg) {
				t.Errorf("%q: stderr %q, want it to contain %q", c.argv, h.errb.String(), c.msg)
			}
			if n := len(h.auditLines(t)); n != 1 {
				t.Errorf("%q: %d audit lines, want exactly 1 (every cut refusal is audited)", c.argv, n)
			}
		}
	})
	t.Run("usage-errors", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "usage:"},
			{[]string{"delete", goodTag}, `unknown command "delete"`},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"version", "extra"}, `unknown command "extra"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 2 || out != "" || r.calls != 0 || r.pres != 0 || len(r.h.gh.calls()) != 0 {
				t.Errorf("%v: exit %d stdout %q cut %d startup %d; want 2, nothing printed, nothing run", c.args, code, out, r.calls, r.pres)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-v"}, {"version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskrelease sourceSHA=") || !strings.HasSuffix(out, "\n") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
