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

// The CLI contract tests (desktools-v2/44). Help is exercised on the BUILT binary with a
// hostile config home, a private TMPDIR and a PATH of recording stubs; the value tests run
// in-process with the typed handler (sweepFn) replaced by a recorder, so nothing here runs a
// suite or plants a mutation.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "muhar-cli-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "muhar")
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

// helpForms are every help spelling the contract names for a root with no verbs, alone and
// after the flags a caller writes.
func helpForms() [][]string {
	forms := [][]string{{"-h"}, {"--help"}}
	for _, f := range clicontract.GoFlagHelpForms("") {
		if len(f) == 1 {
			forms = append(forms, f)
		}
	}
	return append(forms,
		[]string{"-spec", "mutations.json", "-h"},
		[]string{"-j", "0", "-spec", "mutations.json", "--help"},
		[]string{"-shard", "1/2", "-help"},
		[]string{"--version"}, []string{"-version"},
	)
}

// recorder replaces the typed handler for one test.
type recorder struct {
	calls int
	got   options
	src   map[string]string
}

func record(t *testing.T) *recorder {
	t.Helper()
	r := &recorder{}
	prevS, prevO := sweepFn, onResolve
	sweepFn = func(o options, _, _ io.Writer) int {
		r.calls++
		r.got = o
		return 0
	}
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range bindings() {
			r.src[b.Key] = v.Source(b.Key).String()
		}
	}
	t.Cleanup(func() { sweepFn, onResolve = prevS, prevO })
	return r
}

func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// regression: help was the flag package's usage on stderr, and a command line with help in it
// was never checked against a run that reads the spec, creates the build cache or starts a suite.
func TestCLIHelpOffline(t *testing.T) {
	bin := toolBinary(t)
	forms := helpForms()
	states := []struct{ name, content string }{}
	for _, m := range clicontract.MalformedConfigs {
		states = append(states, struct{ name, content string }{"spec-" + m.Name, m.Content})
	}
	for _, st := range states {
		st := st
		t.Run(st.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home, binDir, work, tmp := filepath.Join(root, "home"), filepath.Join(root, "bin"), filepath.Join(root, "work"), filepath.Join(root, "tmp")
			logPath := filepath.Join(root, "ran.log")
			for _, d := range []string{home, binDir, work, tmp} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if runtime.GOOS != "windows" {
				for _, h := range []string{"go", "git", "sh", "bash"} {
					script := "#!/bin/sh\necho \"RAN $0 $*\" >> '" + logPath + "'\nexit 0\n"
					if err := os.WriteFile(filepath.Join(binDir, h), []byte(script), 0o755); err != nil {
						t.Fatal(err)
					}
				}
			}
			// The spec is muhar's only configuration: present-but-malformed or absent.
			if st.content != "" {
				if err := os.WriteFile(filepath.Join(work, "mutations.json"), []byte(st.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			env := []string{"HOME=" + home, "USERPROFILE=" + home, "TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp,
				"PATH=" + binDir, "KUBECONFIG=/dev/null"}
			before := snapshotTree(root)
			for _, args := range forms {
				cmd := exec.Command(bin, args...)
				cmd.Dir, cmd.Env = work, env
				var out, errb bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &errb
				if err := cmd.Run(); err != nil {
					t.Errorf("%v: %v, want exit 0 (stderr %q)", args, err, errb.String())
				}
				if errb.Len() != 0 {
					t.Errorf("%v: wrote to stderr (a spec read or run started): %q", args, errb.String())
				}
				want := "--spec" // help lists the flags; version names the binary
				if strings.HasSuffix(args[len(args)-1], "version") {
					want = "muhar sourceSHA="
				}
				if !strings.Contains(out.String(), want) {
					t.Errorf("%v: stdout %q does not carry %q", args, out.String(), want)
				}
			}
			if b, err := os.ReadFile(logPath); err == nil {
				t.Errorf("help started a program:\n%s", b)
			}
			if after := snapshotTree(root); after != before {
				t.Errorf("help changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
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
			t.Errorf("help reached the sweep %d times; want 0", r.calls)
		}
		if code, _, _ := r.exec("-spec", "x.json"); code != 0 || r.calls != 1 {
			t.Errorf("a real run: exit %d, sweep %d; want 0 and exactly 1", code, r.calls)
		}
	})
}

// regression: the flags lived on the process-global flag set, so no source was declared and a
// second parse in one process saw the first one's values.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				code, _, stderr := r.exec(form.Args("spec", pv.Value)...)
				if code != 0 || r.got.spec != pv.Value || r.src["spec"] != "flag" {
					t.Errorf("%s/%s: exit %d spec %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.got.spec, r.src["spec"], pv.Value, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		// muhar declares flag and default only, so the matrix's rows are the ones those two
		// sources can express, plus the proof that an environment variable named like a key is
		// never read.
		cases := []struct {
			name     string
			env      []string
			args     []string
			want     options
			specSrc  string
			jobsSrc  string
			shardSrc string
		}{
			{"nothing-set", nil, nil, options{jobs: 1}, "unset", "default", "unset"},
			{"flag-over-default", nil, []string{"-spec", "s.json", "-j", "0", "-shard", "0/2", "-select"},
				options{spec: "s.json", jobs: 0, shard: "0/2", useSelect: true}, "flag", "flag", "flag"},
			{"empty-flag-beats-unset", nil, []string{"--spec=", "--shard="}, options{jobs: 1}, "flag", "default", "flag"},
			{"scalar-last-wins", nil, []string{"-j", "2", "-j", "3", "-spec", "a", "-spec", "b"}, options{spec: "b", jobs: 3}, "flag", "flag", "unset"},
			{"env-is-not-a-source", []string{"SPEC", "MUHAR_SPEC", "J", "MUHAR_J", "SHARD", "MUHAR_SHARD", "SELECT", "MUHAR_SELECT"},
				nil, options{jobs: 1}, "unset", "default", "unset"},
		}
		for _, c := range cases {
			r := record(t)
			for _, e := range c.env {
				t.Setenv(e, "1")
			}
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.got != c.want || r.src["spec"] != c.specSrc || r.src["j"] != c.jobsSrc || r.src["shard"] != c.shardSrc {
				t.Errorf("%s: exit %d got %+v (spec %s, j %s, shard %s); want %+v (%s, %s, %s)\n%s",
					c.name, code, r.got, r.src["spec"], r.src["j"], r.src["shard"], c.want, c.specSrc, c.jobsSrc, c.shardSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; muhar has flags and defaults only", b.Key)
			}
		}
	})
}

// regression: every caller writes the flag package's single-dash spellings (release.yml,
// truth-suite.yml, the muhar action, the brief Verify rows); the command tree keeps them, and a
// command line muhar cannot parse still exits 2 before anything runs.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args []string
			want options
		}{
			{[]string{"-j", "0", "-spec", "internal/deskkit/mutations.json"}, options{spec: "internal/deskkit/mutations.json"}},
			{[]string{"-j", "0", "-shard", "1/3", "-spec", "cmd/deskmerge/mutations.json"}, options{spec: "cmd/deskmerge/mutations.json", shard: "1/3"}},
			{[]string{"-spec", "specs/clean.json", "-j", "1"}, options{spec: "specs/clean.json", jobs: 1}},
			{[]string{"-spec", "specs/shard.json", "-j", "0", "-shard", "1/3"}, options{spec: "specs/shard.json", shard: "1/3"}},
			{[]string{"-j", "1", "-spec", "x.json"}, options{spec: "x.json", jobs: 1}},
			{[]string{"-j", "2", "-spec", "x.json"}, options{spec: "x.json", jobs: 2}},
			{[]string{"-spec", "mutations.json"}, options{spec: "mutations.json", jobs: 1}},
			{[]string{"-spec=x.json", "-j=0"}, options{spec: "x.json"}},
			{[]string{"--spec", "x.json", "--j", "4"}, options{spec: "x.json", jobs: 4}},
			{[]string{"-spec", "x.json", "-select"}, options{spec: "x.json", jobs: 1, useSelect: true}},
			{[]string{"-spec", "x.json", "-select=false"}, options{spec: "x.json", jobs: 1}},
			{[]string{"-spec", "x.json", "--select=true"}, options{spec: "x.json", jobs: 1, useSelect: true}},
			// The flag package read 0x and 0 prefixes for -j; pflag's base-0 parse keeps them.
			{[]string{"-spec", "x.json", "-j", "0x2"}, options{spec: "x.json", jobs: 2}},
			// A dash-leading value is the flag's value, never a flag.
			{[]string{"-spec", "-mutations.json"}, options{spec: "-mutations.json", jobs: 1}},
			// Words after the flags were ignored, and after -- they are words.
			{[]string{"-spec", "x.json", "extra"}, options{spec: "x.json", jobs: 1}},
			{[]string{"-spec", "x.json", "--", "-j", "9"}, options{spec: "x.json", jobs: 1}},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.got != c.want {
				t.Errorf("%v: exit %d calls %d got %+v; want 0, 1, %+v\n%s", c.args, code, r.calls, r.got, c.want, stderr)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}} {
			code, out, stderr := r.exec(args...)
			if code != 0 || !strings.HasPrefix(out, "muhar sourceSHA=") || stderr != "" || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q stderr %q sweep %d", args, code, out, stderr, r.calls)
			}
		}
	})
	t.Run("paths", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := record(t)
			if code, _, _ := r.exec("-spec", pv.Value, "-j", "0"); code != 0 || r.got.spec != pv.Value {
				t.Errorf("%s: exit %d spec %q, want %q byte-identical", pv.Name, code, r.got.spec, pv.Value)
			}
		}
	})
	t.Run("parse-refusals-exit-2", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"-bogus", "-spec", "x.json"}, "unknown shorthand flag"},
			{[]string{"-j", "x", "-spec", "x.json"}, `invalid argument "x"`},
			{[]string{"-spec"}, "flag needs an argument"},
			{[]string{"-spec", "x.json", "-select=maybe"}, `invalid argument "maybe"`},
			// A trailing help word is the adapter's help command, which knows no -spec.
			{[]string{"-spec", "x.json", "help"}, "unknown flag: --spec"},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 2 || out != "" || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q sweep %d; want 2, nothing printed, nothing run", c.args, code, out, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("run-refusals-exit-1", func(t *testing.T) {
		// The real sweep: each refusal lands before the build cache, a suite or a mutation.
		dir := t.TempDir()
		spec := filepath.Join(dir, "s.json")
		writeFixture(t, spec, `{"test": "false", "control": {"name": "c", "file": "f", "old": "a", "new": "b"}}`)
		t.Setenv("TMPDIR", dir)
		before := snapshotTree(dir)
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "muhar: -spec is required"},
			{[]string{"-spec", filepath.Join(dir, "absent.json")}, "muhar: read spec:"},
			{[]string{"-spec", spec, "-j", "-1"}, "muhar: -j must be >= 0"},
			{[]string{"-spec", spec, "-shard", "3/2"}, "shard index must satisfy"},
		}
		for _, c := range cases {
			var out, errb bytes.Buffer
			code := run(c.args, &out, &errb)
			if code != 1 || out.Len() != 0 || !strings.Contains(errb.String(), c.msg) {
				t.Errorf("%v: exit %d stdout %q stderr %q; want 1 and %q", c.args, code, out.String(), errb.String(), c.msg)
			}
		}
		if after := snapshotTree(dir); after != before {
			t.Errorf("a refused run changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
}
