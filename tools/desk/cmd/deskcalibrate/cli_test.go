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
// the typed handlers (sampleFn, reportFn) and the process startup replaced by recorders, so
// nothing here reads a roster or a results file.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskcalibrate-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskcalibrate")
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

// helpForms are every help and version spelling the contract names, for both verbs, alone and
// after the flags a verb requires.
func helpForms() [][]string {
	forms := append([][]string{}, clicontract.HelpForms("sample")...)
	forms = append(forms, clicontract.GoFlagHelpForms("sample")...)
	forms = append(forms, []string{"help", "report"}, []string{"report", "-h"}, []string{"report", "--help"}, []string{"report", "-help"})
	forms = append(forms, clicontract.VersionForms...)
	forms = append(forms, clicontract.GoFlagVersionForms...)
	return append(forms,
		[]string{"version"},
		[]string{"sample", "--month", "2026-09", "-h"},
		[]string{"sample", "-month", "2026-09", "-seed", "1", "--help"},
		[]string{"report", "--results", "r.json", "-h"},
	)
}

// recorder replaces the startup and the typed handlers for one test.
type recorder struct {
	starts, samples, reports int
	sample                   sampleOpts
	results, roster          string
	src                      map[string]string
}

// sourceOf reports key's source, or "" when the verb does not declare it.
func sourceOf(v *cli.Values, key string) (s string) {
	defer func() {
		if recover() != nil {
			s = ""
		}
	}()
	return v.Source(key).String()
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv(deskkit.EnvReviewerVendor, "")
	os.Unsetenv(deskkit.EnvReviewerVendor)
	r := &recorder{}
	prevS, prevR, prevO := sampleFn, reportFn, onResolve
	sampleFn = func(o sampleOpts, _ io.Writer) error {
		r.samples++
		r.sample = o
		return nil
	}
	reportFn = func(results, roster string, _ io.Writer) error {
		r.reports++
		r.results, r.roster = results, roster
		return nil
	}
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range append(sampleBindings(), reportBindings()...) {
			if s := sourceOf(v, b.Key); s != "" {
				r.src[b.Key] = s
			}
		}
	}
	t.Cleanup(func() { sampleFn, reportFn, onResolve = prevS, prevR, prevO })
	return r
}

// exec runs one in-process invocation with the recording startup.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(io.Writer) int { r.starts++; return 0 })
	return code, out.String(), errb.String()
}

// sampleArgs is a complete `sample` command line plus extra.
func sampleArgs(extra ...string) []string {
	return append([]string{"sample", "--month", "2026-09", "--seed", "7", "--vendor", "openai", "--prs", "1,2"}, extra...)
}

// regression: help ran after the roster echo and activation check (the hand-rolled main did
// both before parsing), and help went through two hand-written usage paths.
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
					if err := os.WriteFile(filepath.Join(cfg, f), []byte("ASSAY_REVIEWER_VENDOR=openai\n"), 0o000); err != nil {
						t.Fatal(err)
					}
				}
			default:
				if err := os.WriteFile(filepath.Join(cfg, "roster.env"), []byte(st.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
				"KUBECONFIG=/dev/null", deskkit.EnvReviewerVendor + "=openai"}
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
		if r.starts != 0 || r.samples != 0 || r.reports != 0 {
			t.Errorf("help and version: startup %d, sample %d, report %d; want all 0", r.starts, r.samples, r.reports)
		}
		if code, _, stderr := r.exec(sampleArgs("--reviewer-vendor", "anthropic")...); code != 0 || r.starts != 1 || r.samples != 1 {
			t.Errorf("a real sample: exit %d, startup %d, handler %d; want 0 and exactly 1 each\n%s", code, r.starts, r.samples, stderr)
		}
		if code, _, stderr := r.exec("report", "--results", "r.json"); code != 0 || r.starts != 2 || r.reports != 1 {
			t.Errorf("a real report: exit %d, startup %d, handler %d; want 0, 2 and 1\n%s", code, r.starts, r.reports, stderr)
		}
	})
}

// regression: the flags lived on per-verb flag sets with no declared sources; the reviewer
// role's vendor was a bare environment read inside the handler.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				code, _, stderr := r.exec(append(sampleArgs(), form.Args("candidates-file", pv.Value)...)...)
				if code != 0 || r.sample.candidatesFile != pv.Value || r.src["candidates-file"] != "flag" {
					t.Errorf("sample %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.sample.candidatesFile, r.src["candidates-file"], pv.Value, stderr)
				}
				code, _, stderr = r.exec(append([]string{"report"}, form.Args("results", pv.Value)...)...)
				if code != 0 || r.results != pv.Value || r.src["results"] != "flag" {
					t.Errorf("report %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.results, r.src["results"], pv.Value, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		// The reviewer role's vendor is the one setting with two sources. It keeps the legacy
		// per-key rule: a non-blank --reviewer-vendor wins, and a blank or whitespace one falls
		// through to ASSAY_REVIEWER_VENDOR (two keys, so the adapter's "explicit empty wins"
		// rule cannot override that).
		cases := []struct {
			name      string
			flag, env *string
			want      string
			flagSrc   string
			envSrc    string
		}{
			{"nothing-set", nil, nil, "", "unset", "unset"},
			{"env-only", nil, clicontract.Opt("openai"), "openai", "unset", "env"},
			{"flag-over-env", clicontract.Opt("anthropic"), clicontract.Opt("openai"), "anthropic", "flag", "env"},
			{"flag-only", clicontract.Opt("anthropic"), nil, "anthropic", "flag", "unset"},
			{"empty-env-is-set", nil, clicontract.Opt(""), "", "unset", "env"},
			{"empty-flag-falls-through", clicontract.Opt(""), clicontract.Opt("openai"), "openai", "flag", "env"},
			{"blank-flag-falls-through", clicontract.Opt("  "), clicontract.Opt("openai"), "openai", "flag", "env"},
		}
		for _, c := range cases {
			r := record(t)
			if c.env != nil {
				t.Setenv(deskkit.EnvReviewerVendor, *c.env)
			}
			args := sampleArgs()
			if c.flag != nil {
				args = append(args, "--reviewer-vendor="+*c.flag)
			}
			code, _, stderr := r.exec(args...)
			if code != 0 || r.sample.reviewerVendor != c.want || r.src["reviewer-vendor"] != c.flagSrc || r.src["roster-vendor"] != c.envSrc {
				t.Errorf("%s: exit %d got %q (flag %s, env %s); want %q (%s, %s)\n%s", c.name, code, r.sample.reviewerVendor,
					r.src["reviewer-vendor"], r.src["roster-vendor"], c.want, c.flagSrc, c.envSrc, stderr)
			}
			// report takes the same environment fallback for a results file without a vendor.
			if code, _, _ := r.exec("report", "--results", "r.json"); code != 0 || (c.env != nil && r.roster != *c.env) || (c.env == nil && r.roster != "") {
				t.Errorf("%s: report exit %d roster vendor %q", c.name, code, r.roster)
			}
			os.Unsetenv(deskkit.EnvReviewerVendor)
		}
		// Single-source settings: the default, an explicit value, and an explicit empty.
		r := record(t)
		if code, _, _ := r.exec(sampleArgs()...); code != 0 || r.sample.n != DefaultSampleSize || r.src["n"] != "default" || r.src["human"] != "unset" {
			t.Errorf("defaults: exit %d n %d (%s) human %s", code, r.sample.n, r.src["n"], r.src["human"])
		}
		if code, _, _ := r.exec(sampleArgs("--n", "3", "--human=")...); code != 0 || r.sample.n != 3 || r.src["n"] != "flag" || r.src["human"] != "flag" || r.sample.human != "" {
			t.Errorf("explicit: exit %d n %d (%s) human %q (%s)", code, r.sample.n, r.src["n"], r.sample.human, r.src["human"])
		}
		// An environment variable named like a key is never read.
		for _, e := range []string{"MONTH", "SEED", "VENDOR", "N", "RESULTS", "DESKCALIBRATE_MONTH", "DESKCALIBRATE_RESULTS"} {
			t.Setenv(e, "9")
		}
		if code, _, _ := r.exec("sample", "--seed", "1"); code != deskkit.ExitRefused || r.samples != 2 {
			t.Errorf("an undeclared environment variable reached sample: exit %d", code)
		}
		if code, _, _ := r.exec("report"); code != deskkit.ExitRefused || r.reports != 0 {
			t.Errorf("an undeclared environment variable reached report: exit %d", code)
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range append(sampleBindings(), reportBindings()...) {
			if b.Config || b.Secret {
				t.Errorf("binding %q declares a config or secret source; deskcalibrate has none", b.Key)
			}
			want := ""
			if b.Key == "roster-vendor" {
				want = deskkit.EnvReviewerVendor
				if b.Flag != "" {
					t.Errorf("roster-vendor is environment-only; it declares flag %q", b.Flag)
				}
			}
			if got := strings.Join(b.Env, ","); got != want {
				t.Errorf("binding %q reads environment %q, want %q", b.Key, got, want)
			}
		}
	})
}

// regression: the documented forms and the flag package's single-dash spellings were the only
// parser; every one of them still reaches the same handler with the same values, and a command
// line deskcalibrate cannot parse still exits 5 before anything runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		base := sampleOpts{month: "2026-09", seed: 7, vendor: "openai", prs: "1,2,3", n: DefaultSampleSize}
		with := func(f func(*sampleOpts)) sampleOpts { o := base; f(&o); return o }
		cases := []struct {
			args []string
			want sampleOpts
		}{
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--vendor", "openai", "--prs", "1,2,3"}, base},
			{[]string{"sample", "-month", "2026-09", "-seed", "7", "-vendor", "openai", "-prs", "1,2,3"}, base},
			{[]string{"sample", "-month=2026-09", "-seed=7", "-vendor=openai", "-prs=1,2,3"}, base},
			{[]string{"sample", "--month=2026-09", "--seed=7", "--vendor=openai", "--prs=1,2,3"}, base},
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--vendor", "openai", "--prs", "1,2,3", "--n", "4"}, with(func(o *sampleOpts) { o.n = 4 })},
			{[]string{"sample", "-month", "2026-09", "-seed", "7", "-vendor", "openai", "-prs", "1,2,3", "-n", "4"}, with(func(o *sampleOpts) { o.n = 4 })},
			{[]string{"sample", "-month", "2026-09", "-seed", "7", "-vendor", "openai", "-prs", "1,2,3", "-n=4"}, with(func(o *sampleOpts) { o.n = 4 })},
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--human", "alice", "--candidates-file", "c.txt"},
				with(func(o *sampleOpts) { o.vendor, o.prs, o.human, o.candidatesFile = "", "", "alice", "c.txt" })},
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--vendor", "openai", "--prs", "1,2,3", "--reviewer-vendor", "anthropic"},
				with(func(o *sampleOpts) { o.reviewerVendor = "anthropic" })},
			// The flag package read the seed as a base-prefixed signed 64-bit integer.
			{[]string{"sample", "-month", "2026-09", "-seed", "-7", "-vendor", "openai", "-prs", "1,2,3"}, with(func(o *sampleOpts) { o.seed = -7 })},
			{[]string{"sample", "-month", "2026-09", "-seed", "0x10", "-vendor", "openai", "-prs", "1,2,3"}, with(func(o *sampleOpts) { o.seed = 16 })},
			{[]string{"sample", "-month", "2026-09", "-seed", "9223372036854775807", "-vendor", "openai", "-prs", "1,2,3"},
				with(func(o *sampleOpts) { o.seed = 9223372036854775807 })},
			// Repeated flags: the last one wins, as before.
			{[]string{"sample", "--month", "2026-08", "--month", "2026-09", "--seed", "1", "--seed", "7", "--vendor", "openai", "--prs", "9", "--prs", "1,2,3"}, base},
			// A dash-leading value is the flag's value, never a flag.
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--vendor", "openai", "--candidates-file", "-c.txt"},
				with(func(o *sampleOpts) { o.prs, o.candidatesFile = "", "-c.txt" })},
			// Words after the flags, and everything after --, are ignored.
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--vendor", "openai", "--prs", "1,2,3", "extra"}, base},
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--vendor", "openai", "--prs", "1,2,3", "--", "--n", "1"}, base},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.samples != 1 || r.sample != c.want {
				t.Errorf("%v: exit %d calls %d got %+v; want 0, 1, %+v\n%s", c.args, code, r.samples, r.sample, c.want, stderr)
			}
		}
		for _, args := range [][]string{{"report", "--results", "r.json"}, {"report", "-results", "r.json"}, {"report", "-results=r.json"}, {"report", "--results=r.json"}} {
			r := record(t)
			if code, _, stderr := r.exec(args...); code != 0 || r.reports != 1 || r.results != "r.json" {
				t.Errorf("%v: exit %d calls %d results %q\n%s", args, code, r.reports, r.results, stderr)
			}
		}
	})
	t.Run("paths", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := record(t)
			if code, _, _ := r.exec("sample", "-month", "2026-09", "-seed", "1", "-vendor", "v", "-candidates-file", pv.Value); code != 0 || r.sample.candidatesFile != pv.Value {
				t.Errorf("sample %s: exit %d got %q, want it byte-identical", pv.Name, code, r.sample.candidatesFile)
			}
			if code, _, _ := r.exec("report", "-results", pv.Value); code != 0 || r.results != pv.Value {
				t.Errorf("report %s: exit %d got %q, want it byte-identical", pv.Name, code, r.results)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}, {"version"}} {
			code, out, stderr := r.exec(args...)
			if code != 0 || !strings.HasPrefix(out, "deskcalibrate sourceSHA=") || !strings.Contains(out, " releaseTag=") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q", args, code, out, stderr)
			}
		}
	})
	t.Run("refusals-exit-5", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "USAGE:"},
			{[]string{"bogus"}, `unknown subcommand "bogus"`},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"sample", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"sample", "-bogus", "1"}, "unknown shorthand flag"},
			{[]string{"sample", "--month"}, "flag needs an argument"},
			{[]string{"sample", "--month", "2026-09", "--seed", "7", "--n", "x"}, `invalid argument "x"`},
			{[]string{"sample", "--seed", "7"}, "--month YYYY-MM is required"},
			{[]string{"sample", "--month", "2026-09"}, "--seed is required"},
			{[]string{"sample", "--month", "2026-09", "--seed", "x"}, `invalid value "x" for flag --seed`},
			{[]string{"sample", "--month", "2026-09", "--seed", "9223372036854775808"}, "for flag --seed: value out of range"},
			{[]string{"report"}, "--results <file> is required"},
			{[]string{"report", "--results", "  "}, "--results <file> is required"},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != deskkit.ExitRefused || out != "" || r.starts != 0 || r.samples != 0 || r.reports != 0 {
				t.Errorf("%v: exit %d stdout %q startup %d handlers %d/%d; want 5 with nothing run", c.args, code, out, r.starts, r.samples, r.reports)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
}
