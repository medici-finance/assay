package main

import (
	"bytes"
	"errors"
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

// The CLI contract tests (desktools-v2/30). Help and version are exercised on the BUILT binary
// with a hostile home, a private TMPDIR, a dead proxy and a PATH of recording stubs; the value
// tests run in process with the typed handler (provisionFn) and the environment read
// (lookupEnv) replaced by recorders, so nothing here reaches GitLab or reads a credential.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskgitlab-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskgitlab")
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

// stubPATH writes recording stubs for every program the script this replaces started.
func stubPATH(t *testing.T, binDir, logPath string) {
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
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return binRun{code, out.String(), errb.String()}
}

// helpForms are every help and version spelling the contract names.
func helpForms() [][]string {
	forms := append([][]string{}, clicontract.HelpForms("create")...)
	return append(forms, clicontract.VersionForms...)
}

// recorder replaces the environment read and the typed handler for one test.
type recorder struct {
	env      map[string]string // the environment lookupEnv answers from
	envReads []string          // every name looked up
	calls    int
	o        options
	src      map[string]string
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{env: map[string]string{}}
	prevP, prevO, prevL := provisionFn, onResolve, lookupEnv
	lookupEnv = func(k string) (string, bool) {
		r.envReads = append(r.envReads, k)
		v, ok := r.env[k]
		return v, ok
	}
	provisionFn = func(o options, stdout, _ io.Writer) int {
		r.calls++
		r.o = o
		fmt.Fprintln(stdout, "provisioned")
		return 0
	}
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range bindings() {
			r.src[b.Key] = v.Source(b.Key).String()
		}
	}
	t.Cleanup(func() { provisionFn, onResolve, lookupEnv = prevP, prevO, prevL })
	return r
}

// exec runs one in-process invocation.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

// base is the smallest valid invocation; it never needs a credential.
func base(extra ...string) []string {
	return append([]string{"create", "--group", "g", "--prefix", "p", "--dry-run"}, extra...)
}

// regression: the script parsed its arguments after sourcing the role table and computing its
// paths; help must answer before any environment, file or network read, under a hostile home
// with a credential in reach.
func TestCLIHelpOffline(t *testing.T) {
	bin := toolBinary(t)
	forms := helpForms()
	states := []struct {
		name  string
		extra []string
	}{
		{"clean", nil},
		{"credential-in-reach", []string{"GITLAB_TOKEN=" + fakeOwnerToken}},
		{"dead-proxy", []string{"HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9", "GITLAB_TOKEN=" + fakeOwnerToken}},
		{"unwritable-home", []string{"GITLAB_TOKEN=" + fakeOwnerToken}},
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
			stubPATH(t, binDir, logPath)
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
			before := snapshotTree(root)
			for _, args := range forms {
				r := execIn(t, bin, work, env, args...)
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
			if after := snapshotTree(root); after != before {
				t.Errorf("help or version changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}

	t.Run("effect-counters", func(t *testing.T) {
		r := record(t)
		r.env["GITLAB_TOKEN"] = fakeOwnerToken
		for _, args := range forms {
			if code, out, _ := r.exec(args...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", args, code, out)
			}
		}
		if len(r.envReads) != 0 || r.calls != 0 {
			t.Errorf("help and version: environment reads %v, handler %d; want none", r.envReads, r.calls)
		}
		if code, _, _ := r.exec(base()...); code != 0 || r.calls != 1 || len(r.envReads) != 1 {
			t.Errorf("a real run: exit %d, handler %d, environment reads %v; want 0, exactly 1 and exactly [GITLAB_TOKEN]", code, r.calls, r.envReads)
		}
	})

	t.Run("help-text", func(t *testing.T) {
		r := record(t)
		_, out, _ := r.exec("create", "--help")
		for _, flag := range []string{"--group", "--prefix", "--project", "--gitlab-url", "--pat-expiry-days", "--out-dir",
			"--avatars-dir", "--no-avatars", "--avatars-only", "--tier", "--status-check-url", "--status-check-name", "--dry-run"} {
			if !strings.Contains(out, flag) {
				t.Errorf("create --help does not list %s", flag)
			}
		}
		if strings.Contains(out, "--gitlab-token") {
			t.Errorf("create --help offers the credential as a flag")
		}
		if _, root, _ := r.exec("--help"); strings.Contains(root, "completion") {
			t.Errorf("root help offers the completion command, which the adapter disables:\n%s", root)
		}
	})
}

// regression: every value reached the script only as `--flag V`; the port must deliver each
// path byte-identical in both long spellings, read the credential from GITLAB_TOKEN alone, and
// read no other environment variable.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, key := range []string{"out-dir", "avatars-dir"} {
			for _, form := range clicontract.OptionForms {
				for _, pv := range clicontract.PathValues {
					r := record(t)
					code, _, stderr := r.exec(base(form.Args(key, pv.Value)...)...)
					if form.GoFlagOnly {
						// The script never accepted a single-dash long flag; neither does this.
						if code != cli.ExitUsage || r.calls != 0 {
							t.Errorf("%s %s/%s: exit %d, handler %d; want usage %d before anything ran", key, form.Name, pv.Name, code, r.calls, cli.ExitUsage)
						}
						continue
					}
					got := r.o.outDir
					if key == "avatars-dir" {
						got = r.o.avatarsDir
					}
					if code != 0 || got != pv.Value || r.src[key] != "flag" {
						t.Errorf("%s %s/%s: exit %d value %q (from %s), want %q from the flag\n%s", key, form.Name, pv.Name, code, got, r.src[key], pv.Value, stderr)
					}
				}
			}
		}
	})

	t.Run("precedence", func(t *testing.T) {
		// The credential is declared from the environment only; every other key from a flag and
		// its default. The rows are the ones those sources can express.
		cases := []struct {
			name           string
			env            map[string]string
			args           []string
			code           int
			token, tokSrc  string
			url, urlSrc    string
			days, daysSrc  string
			check, checkSr string
		}{
			{"nothing-set", nil, base(), 0, "", "unset", defaultGitLabURL, "default", "7", "default", defaultStatusCheckName, "default"},
			{"env-sets-credential", map[string]string{"GITLAB_TOKEN": fakeOwnerToken}, base(), 0, fakeOwnerToken, "env", defaultGitLabURL, "default", "7", "default", defaultStatusCheckName, "default"},
			{"empty-env-is-set-but-empty", map[string]string{"GITLAB_TOKEN": ""}, base(), 0, "", "env", defaultGitLabURL, "default", "7", "default", defaultStatusCheckName, "default"},
			{"flag-over-default", nil, base("--gitlab-url", "https://gitlab.example.invalid", "--pat-expiry-days", "30", "--status-check-name", "n"), 0,
				"", "unset", "https://gitlab.example.invalid", "flag", "30", "flag", "n", "flag"},
			{"empty-flag-beats-default", nil, base("--status-check-name="), 0, "", "unset", defaultGitLabURL, "default", "7", "default", "", "flag"},
			{"scalar-last-wins", nil, base("--pat-expiry-days", "3", "--pat-expiry-days", "9"), 0, "", "unset", defaultGitLabURL, "default", "9", "flag", defaultStatusCheckName, "default"},
		}
		for _, c := range cases {
			r := record(t)
			for k, v := range c.env {
				r.env[k] = v
			}
			code, _, stderr := r.exec(c.args...)
			got := []string{r.o.token, r.src["gitlab-token"], r.o.gitlabURL, r.src["gitlab-url"], r.o.patExpiryDays, r.src["pat-expiry-days"], r.o.statusCheckName, r.src["status-check-name"]}
			want := []string{c.token, c.tokSrc, c.url, c.urlSrc, c.days, c.daysSrc, c.check, c.checkSr}
			if code != c.code || strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("%s: exit %d got %q, want exit %d %q\n%s", c.name, code, got, c.code, want, stderr)
			}
		}
		// An empty GITLAB_TOKEN counts as unset for a live run, as the script's -z did.
		r := record(t)
		r.env["GITLAB_TOKEN"] = ""
		if code, _, stderr := r.exec("create", "--group", "g", "--prefix", "p"); code != cli.ExitUsage || r.calls != 0 || !strings.Contains(stderr, "GITLAB_TOKEN must be set") {
			t.Errorf("live run with an empty GITLAB_TOKEN: exit %d handler %d\n%s", code, r.calls, stderr)
		}
	})

	t.Run("env-is-not-a-source", func(t *testing.T) {
		r := record(t)
		for _, b := range bindings() {
			up := strings.ToUpper(strings.ReplaceAll(b.Key, "-", "_"))
			r.env[up] = "from-env"
			r.env["DESKGITLAB_"+up] = "from-env"
		}
		r.env["GITLAB_TOKEN"] = fakeOwnerToken
		r.env["GITLAB_URL"] = "https://elsewhere.example.invalid"
		code, _, stderr := r.exec(base()...)
		if code != 0 || r.o.group != "g" || r.o.prefix != "p" || r.o.project != "" || r.o.gitlabURL != defaultGitLabURL || r.o.outDir != "" || r.o.tier != "" {
			t.Errorf("an environment variable named like a key changed a setting: exit %d %+v\n%s", code, r.o, stderr)
		}
		if strings.Join(r.envReads, ",") != "GITLAB_TOKEN" {
			t.Errorf("environment names read: %v; want exactly [GITLAB_TOKEN]", r.envReads)
		}
	})

	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config {
				t.Errorf("binding %q declares a config source; deskgitlab reads no config file", b.Key)
			}
			if b.Key == "gitlab-token" {
				if !b.Secret || b.Flag != "" || strings.Join(b.Env, ",") != "GITLAB_TOKEN" {
					t.Errorf("the credential must be environment-only (GITLAB_TOKEN) and secret: %+v", b)
				}
				continue
			}
			if b.Secret || len(b.Env) > 0 || b.Flag == "" {
				t.Errorf("binding %q must be a plain flag with no environment source: %+v", b.Key, b)
			}
		}
	})
}

// regression: the hand-rolled `while [ $# -gt 0 ]` loop is gone; every flag a consumer wrote
// (through the adapter, which prepends `create`) still reaches the provisioner, and an unknown
// flag or a bad value is refused (exit 2) before anything is read or written.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args  []string
			check func(o options) bool
		}{
			{[]string{"create", "--group", "grp", "--prefix", "pre", "--dry-run"}, func(o options) bool { return o.group == "grp" && o.prefix == "pre" && o.dryRun }},
			{[]string{"create", "--group=grp", "--prefix=pre", "--dry-run"}, func(o options) bool { return o.group == "grp" && o.prefix == "pre" }},
			{base("--project", "grp/proj"), func(o options) bool { return o.project == "grp/proj" }},
			{base("--gitlab-url", "https://gitlab.example.invalid"), func(o options) bool { return o.gitlabURL == "https://gitlab.example.invalid" }},
			{base("--gitlab-url", "http://127.0.0.1:8080"), func(o options) bool { return o.gitlabURL == "http://127.0.0.1:8080" }},
			{base("--pat-expiry-days", "30"), func(o options) bool { return o.patExpiryDays == "30" }},
			{base("--out-dir", "/tmp/fleet-tokens"), func(o options) bool { return o.outDir == "/tmp/fleet-tokens" }},
			{base("--avatars-dir", "./icons"), func(o options) bool { return o.avatarsDir == "./icons" && !o.noAvatars }},
			{base("--no-avatars"), func(o options) bool { return o.noAvatars }},
			{[]string{"create", "--avatars-only", "--prefix", "pre", "--out-dir", "d"}, func(o options) bool { return o.avatarsOnly && o.outDir == "d" && !o.dryRun }},
			{[]string{"create", "--avatars-only", "--prefix", "pre", "--out-dir", "d", "--avatars-dir", "i"}, func(o options) bool { return o.avatarsOnly && o.avatarsDir == "i" }},
			{base("--tier", "free"), func(o options) bool { return o.tier == "free" }},
			{base("--tier", "premium"), func(o options) bool { return o.tier == "premium" }},
			{base("--tier", "ultimate", "--status-check-url", "https://verdict.example.invalid/hook", "--status-check-name", "lane"),
				func(o options) bool {
					return o.tier == "ultimate" && o.statusCheckURL == "https://verdict.example.invalid/hook" && o.statusCheckName == "lane"
				}},
			// Flags in any order, and a trailing -- with nothing after it.
			{[]string{"create", "--dry-run", "--prefix", "pre", "--group", "grp", "--"}, func(o options) bool { return o.group == "grp" && o.dryRun }},
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || !c.check(r.o) {
				t.Errorf("%v: exit %d handler %d options %+v\n%s", c.args, code, r.calls, r.o, stderr)
			}
		}
	})

	t.Run("paths", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := record(t)
			if code, _, _ := r.exec(base("--out-dir", pv.Value, "--avatars-dir", pv.Value)...); code != 0 || r.o.outDir != pv.Value || r.o.avatarsDir != pv.Value {
				t.Errorf("%s: exit %d out-dir %q avatars-dir %q, want %q byte-identical", pv.Name, code, r.o.outDir, r.o.avatarsDir, pv.Value)
			}
		}
	})

	t.Run("refusals-before-effects", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{"create", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"create", "-group", "g"}, "unknown shorthand flag"},
			{[]string{"create", "--group"}, "flag needs an argument"},
			{[]string{"create", "--group", "g", "--prefix", "p", "extra"}, `unknown command "extra"`},
			{[]string{"bogus"}, `unknown command "bogus"`},
			{[]string{"create", "--gitlab-token", fakeOwnerToken}, "unknown flag: --gitlab-token"},
			{[]string{"create", "--prefix", "p", "--dry-run"}, "error: --group and --prefix are required"},
			{[]string{"create", "--group", "g", "--dry-run"}, "error: --group and --prefix are required"},
			{[]string{"create", "--avatars-only", "--no-avatars", "--prefix", "p", "--out-dir", "d"}, "error: --avatars-only and --no-avatars are contradictory"},
			{[]string{"create", "--avatars-only", "--prefix", "p"}, "error: --avatars-only requires --prefix and --out-dir"},
			{base("--tier", "gold"), "error: --tier must be one of: free, premium, ultimate (got 'gold')"},
			{base("--avatars-dir", "i", "--no-avatars"), "error: --avatars-dir and --no-avatars are contradictory"},
			{base("--pat-expiry-days", "0"), "error: --pat-expiry-days must be a whole number of days, 1 or more (got '0')"},
			{base("--pat-expiry-days", "-1"), "(got '-1')"},
			{base("--pat-expiry-days", "+3"), "(got '+3')"},
			{base("--pat-expiry-days", "7d"), "(got '7d')"},
			{base("--gitlab-url", "http://gitlab.example.invalid"), "is not https"},
			{base("--gitlab-url", "gitlab.example.invalid"), "is not an absolute base URL"},
			{[]string{"create", "--group", "g", "--prefix", "p"}, "error: GITLAB_TOKEN must be set (a group-owner PAT) unless --dry-run or --avatars-only"},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != cli.ExitUsage || out != "" || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q handler %d; want %d, nothing printed, nothing run", c.args, code, out, r.calls, cli.ExitUsage)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
			if strings.Contains(stderr, fakeOwnerToken) {
				t.Errorf("%v: a refusal printed the credential", c.args)
			}
		}
	})

	t.Run("version", func(t *testing.T) {
		r := record(t)
		if code, out, _ := r.exec("--version"); code != 0 || !strings.HasPrefix(out, "deskgitlab sourceSHA=") || !strings.HasSuffix(out, "\n") {
			t.Errorf("--version: exit %d stdout %q, want the version line", code, out)
		}
	})
}

// The domain check, not the parser, owns every refusal: each refused request is refused
// through the command tree AND with the tree bypassed (validate called with the typed options),
// and in neither case does the fake GitLab see a single request or the token directory appear.
func TestCLIAdmissionBoundary(t *testing.T) {
	f := newFakeGitLab(t)
	cases := []struct {
		name  string
		args  []string
		o     options
		token bool
		want  string
	}{
		{"unknown-tier", []string{"--tier", "gold"}, options{tier: "gold"}, true, "--tier must be one of"},
		{"plaintext-url", []string{"--gitlab-url", "http://gitlab.example.invalid"}, options{gitlabURL: "http://gitlab.example.invalid"}, true, "is not https"},
		{"zero-day-pat", []string{"--pat-expiry-days", "0"}, options{patExpiryDays: "0"}, true, "--pat-expiry-days must be a whole number"},
		{"open-ended-pat", []string{"--pat-expiry-days", "forever"}, options{patExpiryDays: "forever"}, true, "--pat-expiry-days must be a whole number"},
		{"avatars-dir-and-none", []string{"--avatars-dir", "i", "--no-avatars"}, options{avatarsDir: "i", noAvatars: true}, true, "--avatars-dir and --no-avatars are contradictory"},
		{"no-credential", nil, options{}, false, "GITLAB_TOKEN must be set"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), "tokens")
			tok := ""
			if c.token {
				tok = fakeOwnerToken
			}
			args := append([]string{"--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", outDir}, c.args...)
			r := runAgainst(t, f, tok, args...)
			if r.code != cli.ExitUsage || !strings.Contains(r.out, c.want) {
				t.Errorf("through the tree: exit %d, want %d with %q\n%s", r.code, cli.ExitUsage, c.want, r.out)
			}

			// The tree bypassed: the same request, typed, straight into the domain check.
			o := options{group: fakeGroup, prefix: fakePrefix, outDir: outDir, gitlabURL: f.URL(),
				patExpiryDays: "7", statusCheckName: defaultStatusCheckName, token: tok}
			mergeOptions(&o, c.o)
			err := validate(o)
			var ref *refusal
			if !errors.As(err, &ref) || ref.ExitCode() != cli.ExitUsage || !strings.Contains(err.Error(), c.want) {
				t.Errorf("tree bypassed: validate = %v, want a usage refusal containing %q (the refusal must live below the parser)", err, c.want)
			}
			if n := f.calls(); n != 0 {
				t.Errorf("a refused request reached GitLab %d times: %v", n, f.requests())
			}
			if _, err := os.Stat(outDir); !os.IsNotExist(err) {
				t.Errorf("a refused request created the token directory (%v)", err)
			}
		})
	}

	t.Run("parse-failure-has-no-effect", func(t *testing.T) {
		for _, args := range [][]string{{"--bogus"}, {"--group"}, {"--gitlab-token", fakeOwnerToken}, {"extra"}} {
			outDir := filepath.Join(t.TempDir(), "tokens")
			r := runAgainst(t, f, fakeOwnerToken, append([]string{"--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", outDir}, args...)...)
			if r.code != cli.ExitUsage || f.calls() != 0 {
				t.Errorf("%v: exit %d, %d GitLab calls; want %d and none", args, r.code, f.calls(), cli.ExitUsage)
			}
			if _, err := os.Stat(outDir); !os.IsNotExist(err) {
				t.Errorf("%v: a parse failure created the token directory", args)
			}
		}
	})

	// Positive control: the same fake, a valid request, and the run does reach it — so the
	// zero counts above are refusals, not a fixture that never connects.
	t.Run("valid-request-reaches-gitlab", func(t *testing.T) {
		g := newFakeGitLab(t)
		r := runAgainst(t, g, fakeOwnerToken, "--group", fakeGroup, "--prefix", fakePrefix, "--out-dir", t.TempDir(), "--no-avatars")
		if r.code != 0 || g.calls() == 0 {
			t.Errorf("a valid request: exit %d, %d GitLab calls; want 0 and some\n%s", r.code, g.calls(), r.out)
		}
	})
}

// mergeOptions overlays the set fields of over onto o.
func mergeOptions(o *options, over options) {
	if over.tier != "" {
		o.tier = over.tier
	}
	if over.gitlabURL != "" {
		o.gitlabURL = over.gitlabURL
	}
	if over.patExpiryDays != "" {
		o.patExpiryDays = over.patExpiryDays
	}
	if over.avatarsDir != "" {
		o.avatarsDir = over.avatarsDir
	}
	if over.noAvatars {
		o.noAvatars = true
	}
}
