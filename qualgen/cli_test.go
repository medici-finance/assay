package main

// Owner tests for qualgen's command line (desktools-v2/19, cli-contract.md §4). qualgen is a
// standalone module and cannot import the desk module, so the contract's fixture cases
// (tools/desk/internal/clicontract/fixtures.go) are copied here. Two Windows values are
// respelled with the same shape (see pathValues), so this public file names no user
// profile or network host.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// --- fixture cases copied from tools/desk/internal/clicontract/fixtures.go -------------

func opt(s string) *string { return &s }

var pathValues = []struct{ Name, Value string }{
	{"posix-abs", "/var/lib/assay/cell.env"},
	{"posix-space", "/tmp/a dir/with space.txt"},
	{"posix-rel", "./rel/../x"},
	// The contract's win-drive and win-unc values name a profile directory and a host; the
	// shapes are kept (drive-absolute with backslashes; a UNC host and share) with
	// placeholder components.
	{"win-drive", `C:\ProgramData\op\cell.env`},
	{"win-drive-space", `C:\Program Files\assay\x y.txt`},
	{"win-unc", `\\<host>\<share>\dir\f.md`},
	{"win-rel", `..\rel\x`},
	{"win-forward", "C:/mixed/sep/f"},
	{"dash-leading", "-not-a-flag"},
	{"equals-comma", "a=b,c=d"},
	{"unicode", "dossier/übersicht-ß.md"},
}

var optionForms = []struct {
	Name string
	Args func(flag, value string) []string
}{
	{"long-equals", func(f, v string) []string { return []string{"--" + f + "=" + v} }},
	{"long-space", func(f, v string) []string { return []string{"--" + f, v} }},
	// Go-flag spellings: qualgen's parser was Go's flag package, so they stay accepted.
	{"go-equals", func(f, v string) []string { return []string{"-" + f + "=" + v} }},
	{"go-space", func(f, v string) []string { return []string{"-" + f, v} }},
}

type precedenceCase struct {
	Name              string
	Flag, Env, Config *string
}

// precedence is the contract's matrix for order flag > env > config > default. Its
// expected values are recomputed per key below, from the sources that key declares.
var precedence = []precedenceCase{
	{Name: "nothing-set"},
	{Name: "config-only", Config: opt("cfg")},
	{Name: "env-over-config", Env: opt("env"), Config: opt("cfg")},
	{Name: "flag-over-all", Flag: opt("flg"), Env: opt("env"), Config: opt("cfg")},
	{Name: "empty-config-beats-default", Config: opt("")},
	{Name: "empty-env-beats-config", Env: opt(""), Config: opt("cfg")},
	{Name: "empty-flag-beats-env", Flag: opt(""), Env: opt("env")},
	{Name: "unset-env-falls-through", Config: opt("cfg")},
}

func helpForms(sub string) [][]string {
	return [][]string{
		{"-h"}, {"--help"}, {"help"}, {"help", sub},
		{sub, "-h"}, {sub, "--help"},
		{"-help"}, {sub, "-help"},
	}
}

var versionForms = [][]string{{"--version"}, {"-version"}, {"version"}}

var malformedConfigs = []struct{ Name, Content string }{
	{"absent", ""},
	{"not-key-value", "this is not a config\x00\n"},
	{"unterminated-quote", "KEY=\"open\n"},
}

// --- harness ---------------------------------------------------------------------------

var verbs = []string{"mine", "report", "pr", "check", "sweep", "init"}

// cliRec counts every effect one invocation reaches: handler calls, environment reads,
// and the settings each verb resolved.
type cliRec struct {
	calls   []string
	lookups []string
	opts    map[string]any
	values  map[string]*settingValues
}

func fakeCLI(rec *cliRec, env map[string]string) cliEnv {
	call := func(verb string, o any) int {
		rec.calls = append(rec.calls, verb)
		rec.opts[verb] = o
		return 0
	}
	return cliEnv{
		lookupEnv: func(k string) (string, bool) {
			rec.lookups = append(rec.lookups, k)
			v, ok := env[k]
			return v, ok
		},
		verbs: verbHandlers{
			mine:   func(o mineOptions, _, _ io.Writer) int { return call("mine", o) },
			report: func(o reportOptions, _, _ io.Writer) int { return call("report", o) },
			pr:     func(o prOptions, _, _ io.Writer) int { return call("pr", o) },
			check:  func(o checkOptions, _, _ io.Writer) int { return call("check", o) },
			sweep:  func(o sweepOptions, _, _ io.Writer) int { return call("sweep", o) },
			init:   func(o initOptions, _, _ io.Writer) int { return call("init", o) },
		},
		observe: func(verb string, v *settingValues) { rec.values[verb] = v },
	}
}

func runFake(args []string, env map[string]string) (int, string, string, *cliRec) {
	rec := &cliRec{opts: map[string]any{}, values: map[string]*settingValues{}}
	var out, errb bytes.Buffer
	rc := runCLI(args, &out, &errb, fakeCLI(rec, env))
	return rc, out.String(), errb.String(), rec
}

// --- TestCLIHelpOffline ----------------------------------------------------------------

// fileFlag is, per verb, a flag naming a file the verb's handler would read; help must
// print without reading it.
var fileFlag = map[string]string{
	"mine": "identity-map", "report": "baselines", "pr": "out",
	"check": "out", "sweep": "config", "init": "root",
}

// regression: #2111
func TestCLIHelpOffline(t *testing.T) {
	for _, mc := range malformedConfigs {
		dir := t.TempDir()
		cfg := filepath.Join(dir, "absent.conf")
		env := map[string]string{}
		if mc.Name != "absent" {
			cfg = filepath.Join(dir, mc.Name+".conf")
			if err := os.WriteFile(cfg, []byte(mc.Content), 0o644); err != nil {
				t.Fatal(err)
			}
			env[ciWriterEnv] = mc.Content
		}
		var cases [][]string
		for _, v := range verbs {
			for _, f := range helpForms(v) {
				if f[0] == v {
					// The verb's own file flag and required-argument gaps sit beside help.
					f = append(f, "--"+fileFlag[v]+"="+cfg)
				}
				cases = append(cases, f)
			}
		}
		for _, args := range append(cases, versionForms...) {
			rc, out, errs, rec := runFake(args, env)
			name := mc.Name + " " + strings.Join(args, " ")
			if rc != 0 {
				t.Errorf("%s: exit %d, want 0 (stderr %q)", name, rc, errs)
			}
			if len(rec.calls) != 0 || len(rec.lookups) != 0 || len(rec.values) != 0 {
				t.Errorf("%s: effects before help/version: calls=%v env reads=%v resolved=%d",
					name, rec.calls, rec.lookups, len(rec.values))
			}
			if out == "" || errs != "" {
				t.Errorf("%s: want output on stdout only, got stdout %q stderr %q", name, out, errs)
			}
			if args[0] == "--version" || args[0] == "-version" || args[0] == "version" {
				if out != qualgenVersion+"\n" {
					t.Errorf("%s: version printed %q, want the bare release tag", name, out)
				}
			}
		}
	}

	// The production entry point too: help for every verb, with every output root and
	// file naming a path that must still not exist afterwards.
	dir := t.TempDir()
	never := filepath.Join(dir, "never")
	t.Setenv(ciWriterEnv, ciWriterToken)
	for _, v := range verbs {
		var out, errb bytes.Buffer
		args := []string{v, "--help", "--" + fileFlag[v] + "=" + never}
		if rc := dispatch(args, &out, &errb); rc != 0 {
			t.Errorf("dispatch %v: exit %d (stderr %q)", args, rc, errb.String())
		}
		if !strings.Contains(out.String(), "qualgen "+v) {
			t.Errorf("dispatch %v: help does not name the verb:\n%s", args, out.String())
		}
	}
	if _, err := os.Stat(never); !os.IsNotExist(err) {
		t.Errorf("help created %s (stat err %v)", never, err)
	}
}

// --- TestCLIConfigFlow -----------------------------------------------------------------

// boundKey is one declared string setting, read back from the typed options its verb's
// handler received.
type boundKey struct {
	verb, key, flag, env string
	def                  *string
	read                 func(any) string
}

func boundKeys() []boundKey {
	dot, empty := opt("."), opt("")
	return []boundKey{
		{"mine", "repo", "repo", "", dot, func(o any) string { return o.(mineOptions).Repo }},
		{"mine", "out", "out", "", empty, func(o any) string { return o.(mineOptions).Out }},
		{"mine", "identity-map", "identity-map", "", empty, func(o any) string { return o.(mineOptions).IdentityMap }},
		{"mine", "instruction-globs", "instruction-globs", "", empty, func(o any) string { return o.(mineOptions).InstructionGlobs }},
		{"report", "out", "out", "", empty, func(o any) string { return o.(reportOptions).Out }},
		{"report", "baselines", "baselines", "", empty, func(o any) string { return o.(reportOptions).Baselines }},
		{"report", "quality-writer", "", ciWriterEnv, nil, func(o any) string { return o.(reportOptions).Writer }},
		{"pr", "out", "out", "", empty, func(o any) string { return o.(prOptions).Out }},
		{"pr", "repo", "repo", "", dot, func(o any) string { return o.(prOptions).Repo }},
		{"pr", "head", "head", "", empty, func(o any) string { return o.(prOptions).Head }},
		{"pr", "base", "base", "", empty, func(o any) string { return o.(prOptions).Base }},
		{"check", "out", "out", "", empty, func(o any) string { return o.(checkOptions).Out }},
		{"check", "repo", "repo", "", dot, func(o any) string { return o.(checkOptions).Repo }},
		{"sweep", "repo", "repo", "", empty, func(o any) string { return o.(sweepOptions).Repo }},
		{"sweep", "out", "out", "", empty, func(o any) string { return o.(sweepOptions).Out }},
		{"sweep", "config", "config", "", empty, func(o any) string { return o.(sweepOptions).Config }},
		{"init", "root", "root", "", dot, func(o any) string { return o.(initOptions).Root }},
	}
}

// regression: #2111
func TestCLIConfigFlow(t *testing.T) {
	for _, k := range boundKeys() {
		// An environment name the key does NOT declare: it must never be read.
		undeclared := "QUALGEN_" + strings.ToUpper(strings.ReplaceAll(k.key, "-", "_"))
		for _, row := range precedence {
			args := []string{k.verb}
			env := map[string]string{}
			want, wantSrc := "", "unset"
			if k.def != nil {
				want, wantSrc = *k.def, "default"
			}
			// qualgen has no configuration file: row.Config has no input channel, so the
			// config rows reduce to the rows below them (the value never arrives).
			if row.Env != nil {
				if k.env != "" {
					env[k.env] = *row.Env
					want, wantSrc = *row.Env, "env"
				} else {
					env[undeclared] = *row.Env
				}
			}
			if row.Flag != nil && k.flag != "" {
				args = append(args, "--"+k.flag+"="+*row.Flag)
				want, wantSrc = *row.Flag, "flag"
			}
			name := k.verb + "." + k.key + "/" + row.Name
			rc, _, errs, rec := runFake(args, env)
			if rc != 0 || len(rec.calls) != 1 {
				t.Errorf("%s: exit %d calls %v (stderr %q)", name, rc, rec.calls, errs)
				continue
			}
			if got := k.read(rec.opts[k.verb]); got != want {
				t.Errorf("%s: handler got %q, want %q", name, got, want)
			}
			if got := rec.values[k.verb].source(k.key).String(); got != wantSrc {
				t.Errorf("%s: source %s, want %s", name, got, wantSrc)
			}
			// Only declared environment names are ever looked up, and only by report.
			wantLookups := []string(nil)
			if k.verb == "report" {
				wantLookups = []string{ciWriterEnv}
			}
			if !reflect.DeepEqual(rec.lookups, wantLookups) {
				t.Errorf("%s: environment reads %v, want %v", name, rec.lookups, wantLookups)
			}
		}
	}

	// Typed bool and int settings: flag beats default, an explicit false is the flag's.
	typed := []struct {
		args    []string
		key     string
		want    any
		wantSrc string
	}{
		{[]string{"mine"}, "in-repo", false, "default"},
		{[]string{"mine", "--in-repo"}, "in-repo", true, "flag"},
		{[]string{"mine", "--block-min=7"}, "block-min", 7, "flag"},
		{[]string{"mine"}, "block-min", DefaultBlockMin, "default"},
		{[]string{"mine", "--churn-window-days", "0"}, "churn-window-days", 0, "flag"},
		{[]string{"mine"}, "churn-window-days", DefaultChurnWindowDays, "default"},
		{[]string{"report", "--write=false"}, "write", false, "flag"},
		{[]string{"report", "--write"}, "write", true, "flag"},
		{[]string{"sweep", "--reverify-all"}, "reverify-all", true, "flag"},
		{[]string{"init", "--dry-run"}, "dry-run", true, "flag"},
		{[]string{"init"}, "dry-run", false, "default"},
	}
	for _, c := range typed {
		rc, _, errs, rec := runFake(c.args, map[string]string{"QUALGEN_IN_REPO": "true", "QUALGEN_BLOCK_MIN": "99"})
		if rc != 0 {
			t.Errorf("%v: exit %d (stderr %q)", c.args, rc, errs)
			continue
		}
		v := rec.values[c.args[0]]
		got := v.vals[c.key].v
		if got != c.want || v.source(c.key).String() != c.wantSrc {
			t.Errorf("%v: %s = %v from %s, want %v from %s", c.args, c.key, got, v.source(c.key), c.want, c.wantSrc)
		}
	}

	// The CI-writer setting end to end through the real report handler: unset and
	// present-empty both refuse --write with exit 2; only the exact token writes.
	for _, c := range []struct {
		name string
		env  map[string]string
		rc   int
	}{
		{"writer-unset", map[string]string{}, 2},
		{"writer-empty", map[string]string{ciWriterEnv: ""}, 2},
		{"writer-other", map[string]string{ciWriterEnv: "CI"}, 2},
		{"writer-ci", map[string]string{ciWriterEnv: ciWriterToken}, 0},
	} {
		root := t.TempDir()
		var out, errb bytes.Buffer
		env := cliEnv{
			lookupEnv: func(k string) (string, bool) { v, ok := c.env[k]; return v, ok },
			verbs:     productionVerbs(),
		}
		rc := runCLI([]string{"report", "--out", root, "--write"}, &out, &errb, env)
		if rc != c.rc {
			t.Errorf("%s: exit %d, want %d (stderr %q)", c.name, rc, c.rc, errb.String())
		}
		_, err := os.Stat(filepath.Join(NewStore(root).dir(), qualityView))
		if wrote := err == nil; wrote != (c.rc == 0) {
			t.Errorf("%s: committed view written=%v, want %v", c.name, wrote, c.rc == 0)
		}
	}
}

// --- TestCLILegacyForms ----------------------------------------------------------------

// regression: #2111
func TestCLILegacyForms(t *testing.T) {
	// Every form the consumers write (workflows, docs, Verify rows, the init scaffold).
	consumer := []struct {
		args []string
		env  map[string]string
		want any
	}{
		{[]string{"report", "--out", "."}, nil, reportOptions{Out: "."}},
		{[]string{"report", "--out", ".", "--write"}, map[string]string{ciWriterEnv: "ci"},
			reportOptions{Out: ".", Write: true, Writer: "ci"}},
		{[]string{"mine", "--repo", "..", "--out", "/tmp/q"}, nil,
			mineOptions{Repo: "..", Out: "/tmp/q", BlockMin: DefaultBlockMin, ChurnWindowDays: DefaultChurnWindowDays, DefectLabels: "bug,defect,incident"}},
		{[]string{"mine", "--repo", ".", "--in-repo", "--block-min", "5", "--churn-window-days", "30",
			"--identity-map", "m.json", "--instruction-globs", "a/*.md,b.md"}, nil,
			mineOptions{Repo: ".", InRepo: true, BlockMin: 5, ChurnWindowDays: 30, IdentityMap: "m.json", InstructionGlobs: "a/*.md,b.md", DefectLabels: "bug,defect,incident"}},
		{[]string{"pr", "1", "--out", "/tmp/x"}, nil, prOptions{PR: "1", Out: "/tmp/x", Repo: "."}},
		{[]string{"pr", "105", "--out", "o", "--repo", "d"}, nil, prOptions{PR: "105", Out: "o", Repo: "d"}},
		{[]string{"pr", "7", "--out", "o", "--head", "h", "--base", "b"}, nil,
			prOptions{PR: "7", Out: "o", Repo: ".", Head: "h", Base: "b"}},
		{[]string{"check", "qualgen/main.go", "--out", "/tmp/x"}, nil,
			checkOptions{Paths: []string{"qualgen/main.go"}, Out: "/tmp/x", Repo: "."}},
		{[]string{"check", "hot.go", "b/*.go", "--out", "o", "--repo", "d"}, nil,
			checkOptions{Paths: []string{"hot.go", "b/*.go"}, Out: "o", Repo: "d"}},
		{[]string{"sweep", "--repo", "r", "--out", "o", "--config", "c", "--reverify-all"}, nil,
			sweepOptions{Repo: "r", Out: "o", Config: "c", ReverifyAll: true}},
		{[]string{"init"}, nil, initOptions{Root: "."}},
		{[]string{"init", "--root", "d", "--dry-run"}, nil, initOptions{Root: "d", DryRun: true}},
		// Go-flag spellings, a repeated scalar (last wins) and "--" termination.
		{[]string{"report", "-out", "a", "-write=false", "--out=b"}, nil, reportOptions{Out: "b"}},
		{[]string{"init", "-root=x", "-dry-run"}, nil, initOptions{Root: "x", DryRun: true}},
		{[]string{"check", "--out", "o", "--", "-a.go", "--repo", "x"}, nil,
			checkOptions{Paths: []string{"-a.go", "--repo", "x"}, Out: "o", Repo: "."}},
		{[]string{"pr", "--out", "-o", "9"}, nil, prOptions{PR: "9", Out: "-o", Repo: "."}},
		{[]string{"mine", "-out", "-repo", "-block-min", "0x10"}, nil,
			mineOptions{Repo: ".", Out: "-repo", BlockMin: 16, ChurnWindowDays: DefaultChurnWindowDays, DefectLabels: "bug,defect,incident"}},
	}
	for _, c := range consumer {
		rc, _, errs, rec := runFake(c.args, c.env)
		if rc != 0 || len(rec.calls) != 1 {
			t.Errorf("%q: exit %d calls %v (stderr %q)", c.args, rc, rec.calls, errs)
			continue
		}
		if got := rec.opts[rec.calls[0]]; !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: handler got %+v, want %+v", c.args, got, c.want)
		}
	}
	if rc, out, _, _ := runFake([]string{"--version"}, nil); rc != 0 || out != qualgenVersion+"\n" {
		t.Errorf("--version: exit %d, printed %q", rc, out)
	}

	// Every path value reaches the typed handler byte-identical through every spelling.
	for _, k := range boundKeys() {
		if k.flag == "" {
			continue
		}
		for _, form := range optionForms {
			for _, pv := range pathValues {
				args := append([]string{k.verb}, form.Args(k.flag, pv.Value)...)
				rc, _, errs, rec := runFake(args, nil)
				if rc != 0 || len(rec.calls) != 1 {
					t.Errorf("%s --%s %s/%s: exit %d (stderr %q)", k.verb, k.flag, form.Name, pv.Name, rc, errs)
					continue
				}
				if got := k.read(rec.opts[k.verb]); got != pv.Value {
					t.Errorf("%s --%s %s/%s: got %q, want %q", k.verb, k.flag, form.Name, pv.Name, got, pv.Value)
				}
			}
		}
	}
	for _, pv := range pathValues {
		rc, _, _, rec := runFake([]string{"check", "--out", "o", "--", pv.Value}, nil)
		if rc != 0 || !reflect.DeepEqual(rec.opts["check"], checkOptions{Paths: []string{pv.Value}, Out: "o", Repo: "."}) {
			t.Errorf("check path %s: exit %d, got %+v", pv.Name, rc, rec.opts["check"])
		}
	}

	// Unknown and malformed input exits 2 before any handler or environment read.
	refused := []struct {
		args []string
		say  string
	}{
		{[]string{}, "Usage:"},
		{[]string{"frobnicate"}, `qualgen: unknown mode "frobnicate" (want one of: mine, report, pr, check, sweep, init)`},
		{[]string{"mine", "--bogus"}, "unknown flag: --bogus"},
		{[]string{"mine", "-bogus"}, "unknown shorthand flag"},
		{[]string{"--bogus"}, "unknown flag: --bogus"},
		{[]string{"mine", "--block-min", "x"}, "block-min"},
		{[]string{"report", "--write=maybe"}, "write"},
		{[]string{"report", "--out"}, "needs an argument"},
		{[]string{"report", "--quality-writer=ci"}, "unknown flag: --quality-writer"},
		{[]string{"mine", "extra"}, "unknown command"},
		{[]string{"pr", "1", "2", "--out", "o"}, "accepts at most 1 arg"},
		{[]string{"version", "x"}, "unknown command"},
		{[]string{"__complete", "mine", ""}, "unknown mode"},
		{[]string{"__completeNoDesc", "m"}, "unknown mode"},
		{[]string{"completion", "bash"}, "unknown mode"},
	}
	for _, c := range refused {
		rc, _, errs, rec := runFake(c.args, map[string]string{ciWriterEnv: ciWriterToken})
		if rc != 2 {
			t.Errorf("%q: exit %d, want 2 (stderr %q)", c.args, rc, errs)
		}
		if len(rec.calls) != 0 || len(rec.lookups) != 0 {
			t.Errorf("%q: effects on refused input: calls=%v env reads=%v", c.args, rec.calls, rec.lookups)
		}
		if !strings.Contains(errs, c.say) {
			t.Errorf("%q: stderr %q does not say %q", c.args, errs, c.say)
		}
	}
}

// The pre-Cobra mode entry points the older tests call, now routed through the one parser.
func runReport(args []string, stdout, stderr io.Writer) int {
	return dispatch(append([]string{"report"}, args...), stdout, stderr)
}

func runPR(args []string, stdout, stderr io.Writer) int {
	return dispatch(append([]string{"pr"}, args...), stdout, stderr)
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	return dispatch(append([]string{"check"}, args...), stdout, stderr)
}

func runInit(args []string, stdout, stderr io.Writer) int {
	return dispatch(append([]string{"init"}, args...), stdout, stderr)
}
