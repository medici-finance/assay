package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLI contract tests (desktools-v2/23). Help and version are exercised on the BUILT binary
// with a hostile home, a private TMPDIR, a dead proxy and a PATH of recording stubs; the value
// tests run in process with the typed handlers (provisionFn, labelsFn) and the environment read
// replaced by recorders, and an HTTP transport that fails the test on any request, so nothing
// here reaches a forge or reads a credential.

var (
	fleetBinOnce sync.Once
	fleetBinPath string
	fleetBinErr  error
)

// fleetBinary builds this package's binary once per test process.
func fleetBinary(t *testing.T) string {
	t.Helper()
	fleetBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskfleet-bin-")
		if err != nil {
			fleetBinErr = err
			return
		}
		out := filepath.Join(dir, "deskfleet")
		if runtime.GOOS == "windows" {
			out += ".exe"
		}
		if b, err := goquiet.Cmd(exec.Command("go", "build", "-o", out, ".")).CombinedOutput(); err != nil {
			fleetBinErr = fmt.Errorf("go build: %v\n%s", err, b)
			return
		}
		fleetBinPath = out
	})
	if fleetBinErr != nil {
		t.Fatalf("building the binary under test: %v", fleetBinErr)
	}
	return fleetBinPath
}

// treeSnapshot fingerprints every path under dir: name, mode, size and mtime.
func treeSnapshot(dir string) string {
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

// recordingPATH writes recording stubs for every program a forge client might start.
func recordingPATH(t *testing.T, binDir, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	for _, h := range []string{"curl", "jq", "git", "glab", "gh", "mktemp"} {
		script := "#!/bin/sh\necho \"RAN $0 $*\" >> '" + logPath + "'\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, h), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

type fleetRun struct {
	code           int
	stdout, stderr string
}

// runBinary runs bin in dir with EXACTLY env as its environment.
func runBinary(t *testing.T, bin, dir string, env []string, args ...string) fleetRun {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir, cmd.Env = dir, env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return fleetRun{code, out.String(), errb.String()}
}

// fleetHelpForms are every help and version spelling the contract names, for both verbs,
// including the single-dash Go-flag forms deskfleet always accepted.
func fleetHelpForms() [][]string {
	var forms [][]string
	for _, verb := range []string{"provision", "labels"} {
		forms = append(forms, clicontract.HelpForms(verb)...)
		forms = append(forms, clicontract.GoFlagHelpForms(verb)...)
	}
	forms = append(forms, clicontract.VersionForms...)
	return append(forms, clicontract.GoFlagVersionForms...)
}

// fleetRecorder replaces the environment read and both typed handlers for one test.
type fleetRecorder struct {
	env      map[string]string // the environment lookupEnv answers from
	envReads []string          // every name looked up
	calls    int
	p        provisionOpts
	l        labelsOpts
	src      map[string]string
	net      *failingTransport
	out, err bytes.Buffer
}

func recordFleet(t *testing.T) *fleetRecorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &fleetRecorder{env: map[string]string{}, net: &failingTransport{t: t}}
	prevP, prevL, prevO := provisionFn, labelsFn, onResolve
	provisionFn = func(o provisionOpts, e *env) int {
		r.calls++
		r.p = o
		fmt.Fprintln(e.stdout, "provisioned")
		return exitOK
	}
	labelsFn = func(o labelsOpts, e *env) int {
		r.calls++
		r.l = o
		fmt.Fprintln(e.stdout, "labelled")
		return exitOK
	}
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range append(provisionBindings(), labelsBindings()...) {
			if s, ok := sourceOf(v, b.Key); ok {
				r.src[b.Key] = s
			}
		}
	}
	t.Cleanup(func() { provisionFn, labelsFn, onResolve = prevP, prevL, prevO })
	return r
}

// sourceOf is v.Source(key) for a key the running verb declared; ok is false otherwise.
func sourceOf(v *cli.Values, key string) (s string, ok bool) {
	defer func() {
		if recover() != nil {
			s, ok = "", false
		}
	}()
	return v.Source(key).String(), true
}

// exec runs one in-process invocation.
func (r *fleetRecorder) exec(args ...string) (int, string, string) {
	r.out.Reset()
	r.err.Reset()
	e := &env{
		stdout: &r.out,
		stderr: &r.err,
		http:   &http.Client{Transport: r.net, Timeout: 5 * time.Second},
		lookupEnv: func(k string) (string, bool) {
			r.envReads = append(r.envReads, k)
			v, ok := r.env[k]
			return v, ok
		},
		now:              time.Now,
		createRestricted: createRestricted,
		classifyCustody:  deskkit.ClassifyCustodyOwnerOnly,
		githubAPIBase:    "https://api.github.invalid",
	}
	code := run(args, e)
	return code, r.out.String(), r.err.String()
}

// pBase is the smallest valid provision invocation; lBase the smallest valid labels one.
// Neither needs a credential.
func pBase(extra ...string) []string {
	return append([]string{"provision", "--group", "g", "--prefix", "p", "--dry-run"}, extra...)
}

func lBase(extra ...string) []string {
	return append([]string{"labels", "--forge", "github", "--repo", "o/r", "--dry-run"}, extra...)
}

// regression: the Go flag parsers printed usage on stderr and exited 2 for `<verb> -h`; help
// must now answer on stdout, exit 0, before any environment, file or network read, under a
// hostile home with a credential and an API base in reach.
func TestCLIHelpOffline(t *testing.T) {
	bin := fleetBinary(t)
	forms := fleetHelpForms()
	type state struct {
		name, config string
		extra        []string
	}
	states := []state{
		{"clean", "", nil},
		{"credential-in-reach", "", []string{"GITLAB_API_BASE=https://gitlab.example.invalid/api/v4"}},
		{"dead-proxy", "", []string{"HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9", "GITLAB_API_BASE=https://gitlab.example.invalid/api/v4"}},
		{"unwritable-home", "", []string{"GITLAB_API_BASE=https://gitlab.example.invalid/api/v4"}},
	}
	for _, mc := range clicontract.MalformedConfigs {
		states = append(states, state{"malformed-config-" + mc.Name, mc.Content, nil})
	}
	for _, st := range states {
		st := st
		t.Run(st.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home, binDir, work, tmp := filepath.Join(root, "home"), filepath.Join(root, "bin"), filepath.Join(root, "work"), filepath.Join(root, "tmp")
			cfgDir := filepath.Join(home, ".config", "assay")
			logPath := filepath.Join(root, "ran.log")
			for _, d := range []string{home, binDir, work, tmp, cfgDir} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			// A credential where provisioning would look for one, in every state.
			if err := os.WriteFile(filepath.Join(cfgDir, "gitlab-owner.token"), []byte(fakeOwnerToken+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if st.config != "" {
				for _, name := range []string{"deskfleet.env", "config.env", "deskfleet.yaml"} {
					if err := os.WriteFile(filepath.Join(cfgDir, name), []byte(st.config), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			recordingPATH(t, binDir, logPath)
			if st.name == "unwritable-home" && runtime.GOOS != "windows" {
				if err := os.Chmod(home, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(home, 0o755) })
			}
			env := append([]string{"HOME=" + home, "USERPROFILE=" + home, "TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp,
				"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"), "KUBECONFIG=/dev/null"}, st.extra...)
			if runtime.GOOS == "windows" {
				env = append(env, "SystemRoot="+os.Getenv("SystemRoot"))
			}
			before := treeSnapshot(root)
			for _, args := range forms {
				r := runBinary(t, bin, work, env, args...)
				if r.code != 0 {
					t.Errorf("%v: exit %d, want 0 (stderr %q)", args, r.code, r.stderr)
				}
				if r.stderr != "" {
					t.Errorf("%v: wrote to stderr: %q", args, r.stderr)
				}
				if strings.TrimSpace(r.stdout) == "" {
					t.Errorf("%v: printed nothing", args)
				}
				if strings.Contains(r.stdout, fakeOwnerToken) {
					t.Errorf("%v: printed the credential", args)
				}
			}
			if b, err := os.ReadFile(logPath); err == nil {
				t.Errorf("help or version started a program:\n%s", b)
			}
			if after := treeSnapshot(root); after != before {
				t.Errorf("help or version changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}

	t.Run("effect-counters", func(t *testing.T) {
		r := recordFleet(t)
		r.env["GITLAB_API_BASE"] = "https://gitlab.example.invalid/api/v4"
		for _, args := range forms {
			if code, out, _ := r.exec(args...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", args, code, out)
			}
		}
		if len(r.envReads) != 0 || r.calls != 0 || r.net.calls != 0 {
			t.Errorf("help and version: environment reads %v, handler %d, network %d; want none", r.envReads, r.calls, r.net.calls)
		}
		for _, args := range [][]string{pBase(), lBase()} {
			r.envReads, r.calls = nil, 0
			if code, _, _ := r.exec(args...); code != 0 || r.calls != 1 || strings.Join(r.envReads, ",") != "GITLAB_API_BASE" {
				t.Errorf("%v: exit %d, handler %d, environment reads %v; want 0, exactly 1 and exactly [GITLAB_API_BASE]", args, code, r.calls, r.envReads)
			}
		}
	})

	t.Run("help-text", func(t *testing.T) {
		r := recordFleet(t)
		_, pout, _ := r.exec("provision", "--help")
		for _, flag := range []string{"--group", "--prefix", "--project", "--owner-token-file", "--out-dir", "--pat-expiry-days",
			"--dry-run", "--avatars-dir", "--avatars-only", "--no-avatars"} {
			if !strings.Contains(pout, flag) {
				t.Errorf("provision --help does not list %s", flag)
			}
		}
		_, lout, _ := r.exec("labels", "--help")
		for _, flag := range []string{"--forge", "--repo", "--project", "--token-file", "--dry-run"} {
			if !strings.Contains(lout, flag) {
				t.Errorf("labels --help does not list %s", flag)
			}
		}
		for name, out := range map[string]string{"provision": pout, "labels": lout} {
			if strings.Contains(out, "--gitlab-api-base") {
				t.Errorf("%s --help offers the environment-only API base as a flag", name)
			}
		}
		if _, root, _ := r.exec("--help"); strings.Contains(root, "completion") {
			t.Errorf("root help offers the completion command, which the adapter disables:\n%s", root)
		}
	})
}

// regression: every value reached deskfleet only through a Go flag set; the port must deliver
// each path byte-identical in every spelling, read GITLAB_API_BASE as its one environment
// source, and read no other environment variable and no config file.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		type pathKey struct {
			key  string
			base func(...string) []string
			got  func(r *fleetRecorder) string
		}
		keys := []pathKey{
			{"out-dir", pBase, func(r *fleetRecorder) string { return r.p.outDir }},
			{"avatars-dir", pBase, func(r *fleetRecorder) string { return r.p.avatarsDir }},
			{"owner-token-file", pBase, func(r *fleetRecorder) string { return r.p.ownerTokenFile }},
			{"token-file", lBase, func(r *fleetRecorder) string { return r.l.tokenFile }},
		}
		for _, k := range keys {
			for _, form := range clicontract.OptionForms {
				for _, pv := range clicontract.PathValues {
					r := recordFleet(t)
					// GoFlagOnly forms included: deskfleet always took -flag and -flag=value.
					code, _, stderr := r.exec(k.base(form.Args(k.key, pv.Value)...)...)
					if got := k.got(r); code != 0 || r.calls != 1 || got != pv.Value || r.src[k.key] != "flag" {
						t.Errorf("%s %s/%s: exit %d value %q (from %s), want %q from the flag\n%s", k.key, form.Name, pv.Name, code, got, r.src[k.key], pv.Value, stderr)
					}
				}
			}
		}
	})

	t.Run("precedence", func(t *testing.T) {
		// GITLAB_API_BASE is declared from the environment only; every other key from a flag and
		// its default. The rows are the ones those sources can express.
		const apiBase = "https://gitlab.example.invalid/api/v4"
		cases := []struct {
			name           string
			env            map[string]string
			args           []string
			api, apiSrc    string
			days           int
			daysSrc        string
			project, prSrc string
		}{
			{"nothing-set", nil, pBase(), "", "unset", 7, "default", "", "unset"},
			{"env-sets-api-base", map[string]string{"GITLAB_API_BASE": apiBase}, pBase(), apiBase, "env", 7, "default", "", "unset"},
			{"empty-env-is-set-but-empty", map[string]string{"GITLAB_API_BASE": ""}, pBase(), "", "env", 7, "default", "", "unset"},
			{"flag-over-default", nil, pBase("--pat-expiry-days", "30", "--project", "g/x"), "", "unset", 30, "flag", "g/x", "flag"},
			{"scalar-last-wins", nil, pBase("--pat-expiry-days", "3", "--pat-expiry-days", "9"), "", "unset", 9, "flag", "", "unset"},
			{"go-int-base-prefix", nil, pBase("--pat-expiry-days", "0x10"), "", "unset", 16, "flag", "", "unset"},
			{"empty-flag-is-set", nil, pBase("--project="), "", "unset", 7, "default", "", "flag"},
		}
		for _, c := range cases {
			r := recordFleet(t)
			for k, v := range c.env {
				r.env[k] = v
			}
			code, _, stderr := r.exec(c.args...)
			got := fmt.Sprintf("%q|%s|%d|%s|%q|%s", r.p.apiBase, r.src["gitlab-api-base"], r.p.patExpiryDays, r.src["pat-expiry-days"], r.p.project, r.src["project"])
			want := fmt.Sprintf("%q|%s|%d|%s|%q|%s", c.api, c.apiSrc, c.days, c.daysSrc, c.project, c.prSrc)
			if code != 0 || got != want {
				t.Errorf("%s: exit %d got %s, want exit 0 %s\n%s", c.name, code, got, want, stderr)
			}
		}
		// The labels verb resolves the same environment source.
		r := recordFleet(t)
		r.env["GITLAB_API_BASE"] = apiBase
		if code, _, stderr := r.exec("labels", "--forge", "gitlab", "--project", "g/x", "--dry-run"); code != 0 || r.l.apiBase != apiBase || r.src["gitlab-api-base"] != "env" {
			t.Errorf("labels: exit %d api base %q (from %s), want %q from env\n%s", code, r.l.apiBase, r.src["gitlab-api-base"], apiBase, stderr)
		}
	})

	t.Run("env-is-not-a-source", func(t *testing.T) {
		r := recordFleet(t)
		for _, b := range append(provisionBindings(), labelsBindings()...) {
			up := strings.ToUpper(strings.ReplaceAll(b.Key, "-", "_"))
			if up == "GITLAB_API_BASE" {
				continue
			}
			r.env[up] = "from-env"
			r.env["DESKFLEET_"+up] = "from-env"
		}
		r.env["DRY_RUN"], r.env["DESKFLEET_DRY_RUN"] = "true", "true"
		r.env["PAT_EXPIRY_DAYS"], r.env["DESKFLEET_PAT_EXPIRY_DAYS"] = "99", "99"
		code, _, stderr := r.exec("provision", "--group", "g", "--prefix", "p")
		o := r.p
		if code != 0 || o.group != "g" || o.prefix != "p" || o.project != "" || o.ownerTokenFile != "" || o.avatarsDir != "" ||
			o.avatarsOnly || o.noAvatars || o.dryRun || o.patExpiryDays != 7 || o.outDir != deskkit.ConfigHomeDirs()[0] || o.apiBase != "" {
			t.Errorf("an environment variable named like a key changed a setting: exit %d %+v\n%s", code, o, stderr)
		}
		if strings.Join(r.envReads, ",") != "GITLAB_API_BASE" {
			t.Errorf("environment names read: %v; want exactly [GITLAB_API_BASE]", r.envReads)
		}
		r.envReads = nil
		code, _, stderr = r.exec("labels", "--forge", "github", "--repo", "o/r")
		if l := r.l; code != 0 || l.forge != "github" || l.repo != "o/r" || l.project != "" || l.tokenFile != "" || l.dryRun {
			t.Errorf("labels: an environment variable named like a key changed a setting: exit %d %+v\n%s", code, l, stderr)
		}
		if strings.Join(r.envReads, ",") != "GITLAB_API_BASE" {
			t.Errorf("labels: environment names read: %v; want exactly [GITLAB_API_BASE]", r.envReads)
		}
	})

	t.Run("declared-sources", func(t *testing.T) {
		for verb, bs := range map[string][]cli.Binding{"provision": provisionBindings(), "labels": labelsBindings()} {
			for _, b := range bs {
				if b.Config {
					t.Errorf("%s binding %q declares a config source; deskfleet reads no config file", verb, b.Key)
				}
				if b.Secret {
					t.Errorf("%s binding %q is a secret value; credentials are read from files only", verb, b.Key)
				}
				if b.Key == "gitlab-api-base" {
					if b.Flag != "" || strings.Join(b.Env, ",") != "GITLAB_API_BASE" {
						t.Errorf("the API base must be environment-only (GITLAB_API_BASE): %+v", b)
					}
					continue
				}
				if len(b.Env) > 0 || b.Flag == "" || b.Flag != b.Key {
					t.Errorf("%s binding %q must be a plain flag of the same name with no environment source: %+v", verb, b.Key, b)
				}
			}
		}
	})
}

// regression: the Go flag sets (flag.NewFlagSet per verb) are gone; every spelling a consumer
// wrote still reaches the handler, and an unknown flag, a missing or bad value, a stray word or
// a domain refusal exits 2 before anything is read or written.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args  []string
			check func(r *fleetRecorder) bool
		}{
			// docs/adopting-assay*.md and the install skill.
			{[]string{"provision", "--group", "mygroup", "--prefix", "myorg", "--project", "mygroup/myproject", "--owner-token-file", `owner\gitlab-owner.token`},
				func(r *fleetRecorder) bool {
					return r.p.group == "mygroup" && r.p.prefix == "myorg" && r.p.project == "mygroup/myproject" &&
						r.p.ownerTokenFile == `owner\gitlab-owner.token` && !r.p.dryRun && r.p.patExpiryDays == 7
				}},
			{[]string{"labels", "--forge", "github", "--repo", "o/r", "--token-file", "t.token"},
				func(r *fleetRecorder) bool {
					return r.l.forge == "github" && r.l.target == "o/r" && r.l.tokenFile == "t.token" && !r.l.dryRun
				}},
			{[]string{"labels", "--forge", "github", "--repo", "o/r", "--token-file", "t.token", "--dry-run"},
				func(r *fleetRecorder) bool { return r.l.dryRun && r.l.target == "o/r" }},
			{[]string{"labels", "--forge", "gitlab", "--project", "g/p", "--token-file", "t.token", "--dry-run"},
				func(r *fleetRecorder) bool { return r.l.forge == "gitlab" && r.l.target == "g/p" && r.l.dryRun }},
			{[]string{"provision", "--avatars-only", "--avatars-dir", "icons", "--prefix", "myorg", "--out-dir", "d"},
				func(r *fleetRecorder) bool {
					return r.p.avatarsOnly && r.p.avatarsDir == "icons" && r.p.prefix == "myorg" && r.p.outDir == "d"
				}},
			{pBase("--no-avatars"), func(r *fleetRecorder) bool { return r.p.noAvatars }},
			{pBase("--out-dir", "/tmp/fleet-tokens"), func(r *fleetRecorder) bool { return r.p.outDir == "/tmp/fleet-tokens" }},
			{pBase(), func(r *fleetRecorder) bool { return r.p.outDir == deskkit.ConfigHomeDirs()[0] }},
			// Go-flag single-dash forms.
			{[]string{"provision", "-group", "g", "-prefix", "p", "-dry-run", "-pat-expiry-days=30"},
				func(r *fleetRecorder) bool { return r.p.group == "g" && r.p.dryRun && r.p.patExpiryDays == 30 }},
			{[]string{"labels", "-forge=gitlab", "-project", "g/p", "-dry-run"},
				func(r *fleetRecorder) bool { return r.l.forge == "gitlab" && r.l.project == "g/p" && r.l.dryRun }},
			{pBase("--dry-run=true"), func(r *fleetRecorder) bool { return r.p.dryRun }},
			{[]string{"provision", "--group", "g", "--prefix", "p", "--dry-run=false"}, func(r *fleetRecorder) bool { return !r.p.dryRun }},
			// Flags in any order, a trailing -- with nothing after it, and dash-leading values.
			{[]string{"provision", "--dry-run", "--prefix", "p", "--group", "g", "--"}, func(r *fleetRecorder) bool { return r.p.group == "g" && r.p.dryRun }},
			{pBase("--project", "-odd/name"), func(r *fleetRecorder) bool { return r.p.project == "-odd/name" }},
		}
		for _, c := range cases {
			r := recordFleet(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || !c.check(r) {
				t.Errorf("%v: exit %d handler %d provision %+v labels %+v\n%s", c.args, code, r.calls, r.p, r.l, stderr)
			}
			if r.net.calls != 0 {
				t.Errorf("%v: the parser reached the network", c.args)
			}
		}
	})

	t.Run("paths", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := recordFleet(t)
			if code, _, _ := r.exec(pBase("--out-dir", pv.Value, "--avatars-dir", pv.Value, "--owner-token-file", pv.Value)...); code != 0 ||
				r.p.outDir != pv.Value || r.p.avatarsDir != pv.Value || r.p.ownerTokenFile != pv.Value {
				t.Errorf("%s: exit %d %+v, want %q byte-identical", pv.Name, code, r.p, pv.Value)
			}
			if code, _, _ := r.exec(lBase("--token-file", pv.Value)...); code != 0 || r.l.tokenFile != pv.Value {
				t.Errorf("labels %s: exit %d token-file %q, want %q byte-identical", pv.Name, code, r.l.tokenFile, pv.Value)
			}
		}
	})

	t.Run("refusals-before-effects", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{}, ""},
			{[]string{"provision", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"labels", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"provision", "--group"}, "flag needs an argument"},
			{[]string{"provision", "--group", "g", "--prefix", "p", "extra"}, `unexpected argument "extra"`},
			{[]string{"labels", "--forge", "github", "--repo", "o/r", "extra"}, `unexpected argument "extra"`},
			{[]string{"bogus"}, `unknown command "bogus"`},
			{[]string{"--version", "extra"}, `unknown command "extra"`},
			{pBase("--pat-expiry-days", "seven"), "invalid argument"},
			{[]string{"provision", "--prefix", "p", "--dry-run"}, "deskfleet provision: --group and --prefix are required"},
			{[]string{"provision", "--group", "g", "--dry-run"}, "--group and --prefix are required"},
			{pBase("--no-avatars", "--avatars-dir", "d"), "--no-avatars contradicts --avatars-only and --avatars-dir"},
			{[]string{"provision", "--avatars-only", "--no-avatars", "--avatars-dir", "d", "--prefix", "p"}, "--no-avatars contradicts"},
			{[]string{"provision", "--avatars-only", "--prefix", "p"}, "--avatars-only requires --avatars-dir"},
			{[]string{"provision", "--avatars-only", "--avatars-dir", "d"}, "--avatars-only requires --prefix"},
			{[]string{"provision", "--avatars-only", "--avatars-dir", "d", "--prefix", "p", "--project", "g/x"}, "it takes no --project and no --owner-token-file"},
			{[]string{"provision", "--group", "g", "--prefix", "bad prefix", "--dry-run"}, `--prefix "bad prefix" must be letters`},
			{pBase("--pat-expiry-days", "0"), "--pat-expiry-days must be 1-365 (got 0)"},
			{pBase("--pat-expiry-days", "366"), "--pat-expiry-days must be 1-365 (got 366)"},
			{[]string{"labels", "--forge", "github", "--repo", "bad"}, "deskfleet labels: --forge github takes --repo <owner/name> (and no --project)"},
			{[]string{"labels", "--forge", "github", "--repo", "o/r", "--project", "g/x"}, "--forge github takes --repo"},
			{[]string{"labels", "--forge", "gitlab", "--repo", "o/r"}, "--forge gitlab takes --project <path-or-id> (and no --repo)"},
			{[]string{"labels", "--forge", "svn"}, "--forge must be github or gitlab"},
			{[]string{"labels"}, "--forge must be github or gitlab"},
		}
		for _, c := range cases {
			r := recordFleet(t)
			code, out, stderr := r.exec(c.args...)
			if code != exitUsage || out != "" || r.calls != 0 || r.net.calls != 0 {
				t.Errorf("%v: exit %d stdout %q handler %d network %d; want %d, nothing printed, nothing run", c.args, code, out, r.calls, r.net.calls, exitUsage)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
	})

	t.Run("version", func(t *testing.T) {
		r := recordFleet(t)
		for _, args := range [][]string{{"--version"}, {"-version"}} {
			if code, out, _ := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskfleet sourceSHA=") || !strings.HasSuffix(out, "\n") {
				t.Errorf("%v: exit %d stdout %q, want the version line", args, code, out)
			}
		}
	})

	t.Run("usage-exit", func(t *testing.T) {
		if exitUsage != cli.ExitUsage {
			t.Errorf("deskfleet's usage exit %d differs from the adapter's %d", exitUsage, cli.ExitUsage)
		}
	})
}
