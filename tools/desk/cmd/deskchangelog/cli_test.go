package main

import (
	"bytes"
	"errors"
	"fmt"
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

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
)

// The CLI contract tests (desktools-v2/52). Help and version are exercised on the BUILT binary
// with a hostile home, a private TMPDIR, a dead proxy and a PATH of recording stubs; the value
// tests run in process with the environment read (lookupEnv) and the resolved settings
// (onResolve) replaced by recorders.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskchangelog-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskchangelog")
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

// stubPATH writes recording stubs for every program the replaced scripts started.
func stubPATH(t *testing.T, binDir, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	for _, h := range []string{"git", "python3", "python", "go", "gh", "curl", "awk", "sed", "grep", "find", "sort", "mktemp"} {
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

// helpForms are every help and version spelling the contract names, for every verb.
func helpForms() [][]string {
	var forms [][]string
	for _, v := range verbs {
		forms = append(forms, clicontract.HelpForms(v)...)
	}
	forms = append(forms,
		[]string{"credit-identity", "classify", "-h"},
		[]string{"credit-identity", "classify", "--help"},
		[]string{"help", "credit-identity", "classify"},
	)
	return append(forms, clicontract.VersionForms...)
}

const fakeCredential = "credential-sentinel-must-not-echo"

// recorder replaces the environment read and records every resolved setting.
type recorder struct {
	env      map[string]string // the environment lookupEnv answers from
	envReads []string          // every name looked up
	resolves int               // handlers that reached their settings
	vals     map[string]string
	src      map[string]string
}

// allBindings are every binding any verb declares.
func allBindings() []cli.Binding {
	bs := append([]cli.Binding{creditsBinding(false)}, checkBindings()...)
	return append(bs, creditBindings()...)
}

// declared reads key from v when this verb declares it (Values panics on any other key).
func declared(v *cli.Values, key string) (val string, src cli.Source, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return v.String(key), v.Source(key), true
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{env: map[string]string{}}
	prevO, prevL := onResolve, lookupEnv
	lookupEnv = func(k string) (string, bool) {
		r.envReads = append(r.envReads, k)
		v, ok := r.env[k]
		return v, ok
	}
	onResolve = func(v *cli.Values) {
		r.resolves++
		r.vals, r.src = map[string]string{}, map[string]string{}
		for _, b := range allBindings() {
			if val, s, ok := declared(v, b.Key); ok && s != cli.Unset {
				r.vals[b.Key], r.src[b.Key] = val, s.String()
			}
		}
	}
	t.Cleanup(func() { onResolve, lookupEnv = prevO, prevL })
	return r
}

// exec runs one in-process invocation.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

// fixture is a fragment directory with one bullet, an empty-Unreleased changelog, a
// spec file naming v1.2.3 and a docs/streams tree with one clean gate: human brief.
type fixture struct{ root, frags, changelog, spec string }

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{root: root, frags: filepath.Join(root, "changelog"), changelog: filepath.Join(root, "CHANGELOG.md"),
		spec: filepath.Join(root, "spec.md")}
	files := map[string]string{
		filepath.Join(f.frags, "a.md"): "- `alpha` added.\n",
		f.changelog:                    "# Changelog\n\n## Unreleased\n\n## v0.1.0 — 2026-01-01\n\n### Added\n- seed\n",
		f.spec:                         "# Spec\n\nDescribes reference implementation: statusgen v1.2.3\n",
		filepath.Join(root, "docs", "streams", "s", "brief-01-x.md"): "---\ngate: human\n---\n# x\n\n## Human decision\n\nPick one.\nDefault if no answer: none — blocks until answered\n",
	}
	for p, body := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// regression: the scripts read the environment and started git, python3 and awk before
// looking at their arguments; help and version must answer before any environment, file or
// program is touched, under a hostile home with a credential in reach.
func TestCLIHelpOffline(t *testing.T) {
	bin := toolBinary(t)
	forms := helpForms()
	states := []struct {
		name  string
		extra []string
	}{
		{"clean", nil},
		{"credential-in-reach", []string{"GITHUB_TOKEN=" + fakeCredential, "ASSAY_TRUSTED_LOGINS=maintainer-example:1"}},
		{"dead-proxy", []string{"HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9", "GITHUB_TOKEN=" + fakeCredential}},
		{"unwritable-home", []string{"GITHUB_TOKEN=" + fakeCredential}},
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
				"PATH=" + binDir, "KUBECONFIG=/dev/null"}, st.extra...)
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
				if strings.Contains(r.stdout, fakeCredential) {
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
		r.env["ASSAY_TRUSTED_LOGINS"] = "maintainer-example:1"
		for _, args := range forms {
			if code, out, _ := r.exec(args...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", args, code, out)
			}
		}
		if len(r.envReads) != 0 || r.resolves != 0 {
			t.Errorf("help and version: environment reads %v, handlers %d; want none", r.envReads, r.resolves)
		}
		code, out, _ := r.exec("credit-identity", "classify", "octocat-example")
		want := "ASSAY_TRUSTED_LOGINS,ASSAY_TRUSTED_BOT_SLUGS,ASSAY_HUMAN_LOGIN_MAP"
		if code != 0 || out != "credit\n" || r.resolves != 1 || strings.Join(r.envReads, ",") != want {
			t.Errorf("a real run: exit %d stdout %q handlers %d environment reads %v; want 0, credit, 1 and exactly [%s]", code, out, r.resolves, r.envReads, want)
		}
	})

	t.Run("help-text", func(t *testing.T) {
		r := record(t)
		wants := map[string][]string{
			"highlights":             {"--credits", "Exit codes:"},
			"roll":                   {"--credits", "Exit codes:"},
			"check":                  {"--reconcile", "BASE_SHA", "HEAD_SHA", "PR_NUMBER", "Exit codes:"},
			"check-spec-header":      {"<vX.Y.Z>", "spec/brief-v1.md", "Exit codes:"},
			"lint-decision-defaults": {"docs/streams", "Exit codes:"},
			"unreleased-bullets":     {"<changelog>", "Exit codes:"},
			"credits":                {"<fragment-dir>", "Exit codes:"},
		}
		for verb, subs := range wants {
			_, out, _ := r.exec(verb, "--help")
			for _, s := range subs {
				if !strings.Contains(out, s) {
					t.Errorf("%s --help does not mention %s:\n%s", verb, s, out)
				}
			}
		}
		_, out, _ := r.exec("credit-identity", "classify", "--help")
		for _, s := range []string{"ASSAY_TRUSTED_LOGINS", "ASSAY_TRUSTED_BOT_SLUGS", "ASSAY_HUMAN_LOGIN_MAP", "<!-- changelog-credit: no -->"} {
			if !strings.Contains(out, s) {
				t.Errorf("credit-identity classify --help does not mention %s", s)
			}
		}
		if _, root, _ := r.exec("--help"); strings.Contains(root, "completion") {
			t.Errorf("root help offers the completion command, which the adapter disables:\n%s", root)
		}
	})
}

// regression: every value reached the scripts as a positional, `--credits V`, or an
// environment variable; the port must deliver each byte-identical, from exactly the source
// it declares, and read no other environment variable.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		f := newFixture(t)
		for _, c := range []struct {
			key  string
			args func(form []string) []string
		}{
			{"credits", func(form []string) []string { return append([]string{"highlights", f.frags, f.changelog}, form...) }},
			{"reconcile", func(form []string) []string { return append([]string{"check"}, form...) }},
		} {
			for _, form := range clicontract.OptionForms {
				for _, pv := range clicontract.PathValues {
					r := record(t)
					code, _, stderr := r.exec(c.args(form.Args(c.key, pv.Value))...)
					if form.GoFlagOnly {
						// The scripts never took a single-dash long flag; neither does this.
						if code != cli.ExitUsage || r.resolves != 0 {
							t.Errorf("%s %s/%s: exit %d, handlers %d; want usage %d before anything ran", c.key, form.Name, pv.Name, code, r.resolves, cli.ExitUsage)
						}
						continue
					}
					if r.resolves != 1 || r.vals[c.key] != pv.Value || r.src[c.key] != "flag" {
						t.Errorf("%s %s/%s: exit %d value %q (from %s), want %q from the flag\n%s", c.key, form.Name, pv.Name, code, r.vals[c.key], r.src[c.key], pv.Value, stderr)
					}
				}
			}
		}
	})

	t.Run("positionals", func(t *testing.T) {
		for _, pv := range clicontract.PathValues {
			r := record(t)
			var out, errb bytes.Buffer
			// The tree delivers the operand untouched: the domain check names it back.
			code := execute([]string{"check-spec-header", "--", "v1.2.3", pv.Value}, &out, &errb)
			if code != 2 || !strings.Contains(errb.String(), "cannot read "+pv.Value+"\n") {
				t.Errorf("%s: exit %d stderr %q, want could-not-check naming %q", pv.Name, code, errb.String(), pv.Value)
			}
			_ = r
		}
	})

	t.Run("precedence", func(t *testing.T) {
		cases := []struct {
			name      string
			env       map[string]string
			val, src  string
			wantPrint string
		}{
			{"nothing-set", nil, "", "unset", "skip:unknown\n"},
			{"env-sets-roster", map[string]string{"ASSAY_TRUSTED_LOGINS": "maintainer-example:1"}, "maintainer-example:1", "env", "skip:roster\n"},
			{"empty-env-is-set-but-empty", map[string]string{"ASSAY_TRUSTED_LOGINS": ""}, "", "env", "skip:unknown\n"},
		}
		for _, c := range cases {
			r := record(t)
			for k, v := range c.env {
				r.env[k] = v
			}
			code, out, stderr := r.exec("credit-identity", "classify", "maintainer-example")
			src := r.src["trusted-logins"]
			if src == "" {
				src = "unset"
			}
			if code != 0 || r.vals["trusted-logins"] != c.val || src != c.src || out != c.wantPrint {
				t.Errorf("%s: exit %d value %q (from %s) printed %q, want %q from %s printing %q\n%s", c.name, code, r.vals["trusted-logins"], src, out, c.val, c.src, c.wantPrint, stderr)
			}
		}
		// BASE_REF wins over GITHUB_BASE_REF; each is its own declared source.
		r := record(t)
		r.env["BASE_REF"], r.env["GITHUB_BASE_REF"], r.env["BASE_SHA"] = "rel", "main", "abc"
		if code, _, stderr := r.exec("check"); code != 1 || r.vals["base-ref"] != "rel" || r.vals["github-base-ref"] != "main" || r.src["base-sha"] != "env" ||
			!strings.Contains(stderr, "HEAD_SHA: HEAD_SHA is required") {
			t.Errorf("check inputs: exit %d values %v sources %v\n%s", code, r.vals, r.src, stderr)
		}
	})

	t.Run("env-is-not-a-source", func(t *testing.T) {
		f := newFixture(t)
		r := record(t)
		for _, b := range allBindings() {
			up := strings.ToUpper(strings.ReplaceAll(b.Key, "-", "_"))
			r.env["DESKCHANGELOG_"+up] = "from-env"
			if len(b.Env) == 0 {
				r.env[up] = "from-env"
			}
		}
		code, out, stderr := r.exec("highlights", f.frags, f.changelog)
		if code != 0 || r.src["credits"] != "" || strings.Contains(out, "thanks") {
			t.Errorf("an environment variable named like --credits changed the run: exit %d sources %v\n%s%s", code, r.src, out, stderr)
		}
		if len(r.envReads) != 0 {
			t.Errorf("highlights read the environment: %v", r.envReads)
		}
		r.envReads = nil
		_, _, _ = r.exec("check")
		want := "SKIP,BASE_SHA,HEAD_SHA,BASE_REF,GITHUB_BASE_REF,HEAD_REF,PR_NUMBER,CHANGELOG_AGG"
		if got := strings.Join(r.envReads, ","); got != want || r.src["reconcile"] != "" {
			t.Errorf("check: environment names read %s (reconcile from %q); want exactly %s", got, r.src["reconcile"], want)
		}
	})

	t.Run("declared-sources", func(t *testing.T) {
		envOnly := map[string]string{
			"skip": "SKIP", "base-sha": "BASE_SHA", "head-sha": "HEAD_SHA", "base-ref": "BASE_REF",
			"github-base-ref": "GITHUB_BASE_REF", "head-ref": "HEAD_REF", "pr-number": "PR_NUMBER",
			"changelog-agg": "CHANGELOG_AGG", "trusted-logins": "ASSAY_TRUSTED_LOGINS",
			"trusted-bot-slugs": "ASSAY_TRUSTED_BOT_SLUGS", "human-login-map": "ASSAY_HUMAN_LOGIN_MAP",
		}
		flagOnly := map[string]string{"credits": "credits", "reconcile": "reconcile"}
		for _, b := range allBindings() {
			if b.Config || b.Secret || b.Default != nil {
				t.Errorf("binding %q declares a config source, a secret or a default: %+v", b.Key, b)
			}
			if name, ok := envOnly[b.Key]; ok {
				if b.Flag != "" || strings.Join(b.Env, ",") != name {
					t.Errorf("binding %q must be environment-only (%s): %+v", b.Key, name, b)
				}
				continue
			}
			if name, ok := flagOnly[b.Key]; !ok || b.Flag != name || len(b.Env) > 0 {
				t.Errorf("binding %q must be a plain flag with no environment source: %+v", b.Key, b)
			}
		}
	})
}

// gitRepo makes a two-commit repository: base, then head adding changelog/x.md.
func gitRepo(t *testing.T) (dir, base, head string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		b, err := goquiet.Cmd(cmd).Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("# Changelog\n\n## Unreleased\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "CHANGELOG.md")
	git("commit", "-q", "-m", "base")
	base = git("rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(dir, "changelog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "changelog", "x.md"), []byte("- `x` changed.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "changelog/x.md")
	git("commit", "-q", "-m", "head")
	return dir, base, git("rev-parse", "HEAD")
}

// regression: aggregate.py's sys.argv dispatch, the scripts' "$1"/"$2" reads and check.sh's
// argument-free entry are gone; every form a consumer wrote (through the adapters, which
// pass arguments through after `--` where the script took operands) still reaches its verb,
// and an unknown flag, a stray operand or a bad value is refused before anything runs.
func TestCLILegacyForms(t *testing.T) {
	f := newFixture(t)
	creditsMap := filepath.Join(f.root, "credits.map")
	if err := os.WriteFile(creditsMap, []byte("a.md\t@octocat-example\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			name   string
			env    map[string]string
			args   []string
			code   int
			stdout string // substring
		}{
			{"unreleased-bullets", nil, []string{"unreleased-bullets", f.changelog}, 0, ""},
			{"highlights", nil, []string{"highlights", f.frags, f.changelog}, 0, "### Changed\n- `alpha` added."},
			{"highlights-credits-last", nil, []string{"highlights", f.frags, f.changelog, "--credits", creditsMap}, 0, "thanks @octocat-example"},
			{"highlights-credits-first", nil, []string{"highlights", "--credits=" + creditsMap, f.frags, f.changelog}, 0, "thanks @octocat-example"},
			{"highlights-missing-map", nil, []string{"highlights", f.frags, f.changelog, "--credits", filepath.Join(f.root, "nope")}, 0, "- `alpha` added.\n"},
			{"credits", nil, []string{"credits", f.frags}, 0, "a.md\tunresolved\n"},
			{"credits-ignored-map", nil, []string{"credits", f.frags, "--credits", creditsMap}, 0, "a.md\tunresolved\n"},
			{"classify", map[string]string{"ASSAY_TRUSTED_LOGINS": "maintainer-example:1"}, []string{"credit-identity", "classify", "octocat-example", "/dev/null"}, 0, "credit\n"},
			{"classify-adapter", map[string]string{"ASSAY_TRUSTED_LOGINS": "maintainer-example:1"}, []string{"credit-identity", "classify", "--", "-dash-login"}, 0, "credit\n"},
			{"classify-bot", map[string]string{"ASSAY_TRUSTED_LOGINS": "m:1"}, []string{"credit-identity", "classify", "renovate[bot]"}, 0, "skip:bot\n"},
			{"classify-no-login", map[string]string{"ASSAY_TRUSTED_LOGINS": "m:1"}, []string{"credit-identity", "classify"}, 0, "skip:unknown\n"},
			{"identity-no-verb", nil, []string{"credit-identity", "--"}, 0, "skip:unknown\n"},
			{"identity-other-verb", nil, []string{"credit-identity", "--", "lookup", "x"}, 0, "skip:unknown\n"},
			{"spec-header", nil, []string{"check-spec-header", "--", "v1.2.3", f.spec}, 0, "matches v1.2.3"},
			{"spec-header-direct", nil, []string{"check-spec-header", "v1.2.3", f.spec}, 0, "matches v1.2.3"},
			{"lint", nil, []string{"lint-decision-defaults", "--", f.root}, 0, "lint-human-decision-defaults: clean"},
			{"lint-direct", nil, []string{"lint-decision-defaults", f.root}, 0, "clean"},
		}
		for _, c := range cases {
			r := record(t)
			for k, v := range c.env {
				r.env[k] = v
			}
			code, out, stderr := r.exec(c.args...)
			if code != c.code || !strings.Contains(out, c.stdout) {
				t.Errorf("%s %v: exit %d stdout %q, want %d containing %q\n%s", c.name, c.args, code, out, c.code, c.stdout, stderr)
			}
		}

		// roll rewrites the changelog in place.
		g := newFixture(t)
		r := record(t)
		if code, _, stderr := r.exec("roll", g.frags, g.changelog, "v0.2.0", "2026-02-02", "--credits", creditsMap); code != 0 {
			t.Fatalf("roll: exit %d\n%s", code, stderr)
		}
		if b, _ := os.ReadFile(g.changelog); !strings.Contains(string(b), "## v0.2.0 — 2026-02-02") || !strings.Contains(string(b), "thanks @octocat-example") {
			t.Errorf("roll did not write the dated, credited section:\n%s", b)
		}

		// check, argument-free, in a checkout: the skip label passes a PR that adds a fragment.
		dir, base, head := gitRepo(t)
		t.Chdir(dir)
		r = record(t)
		r.env["BASE_SHA"], r.env["HEAD_SHA"], r.env["SKIP"] = base, head, "true"
		if code, out, stderr := r.exec("check"); code != 0 || !strings.Contains(out, "changelog:skip") {
			t.Errorf("check with SKIP=true: exit %d\n%s%s", code, out, stderr)
		}
		r = record(t)
		r.env["BASE_SHA"], r.env["HEAD_SHA"] = base, head
		if code, out, stderr := r.exec("check"); code != 0 || !strings.Contains(out, "changelog/x.md") {
			t.Errorf("check with a fragment: exit %d\n%s%s", code, out, stderr)
		}
	})

	t.Run("refusals-before-effects", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"bogus"}, `unknown command "bogus"`},
			{[]string{"highlights", "--bogus", f.frags, f.changelog}, "unknown flag: --bogus"},
			{[]string{"highlights", "-credits", "m", f.frags, f.changelog}, "unknown shorthand flag"},
			{[]string{"highlights", f.frags, f.changelog, "--credits"}, "flag needs an argument"},
			{[]string{"highlights", f.frags}, "usage: deskchangelog highlights <fragment-dir> <changelog>"},
			{[]string{"roll", f.frags, f.changelog, "v0.2.0"}, "usage: deskchangelog roll"},
			{[]string{"unreleased-bullets"}, "usage: deskchangelog unreleased-bullets"},
			{[]string{"credits", f.frags, "extra"}, "usage: deskchangelog credits"},
			{[]string{"check", "extra"}, "usage: deskchangelog check"},
			{[]string{"check", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"credit-identity", "classify", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"check-spec-header"}, "usage: deskchangelog check-spec-header"},
			{[]string{"check-spec-header", "v1.2.3", f.spec, "extra"}, "usage: deskchangelog check-spec-header"},
			{[]string{"lint-decision-defaults", "a", "b"}, "usage: deskchangelog lint-decision-defaults"},
		}
		for _, c := range cases {
			r := record(t)
			before := snapshotTree(f.root)
			code, out, stderr := r.exec(c.args...)
			if code != cli.ExitUsage || out != "" || r.resolves != 0 || len(r.envReads) != 0 {
				t.Errorf("%v: exit %d stdout %q handlers %d env reads %v; want %d, nothing printed, nothing run", c.args, code, out, r.resolves, r.envReads, cli.ExitUsage)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
			if snapshotTree(f.root) != before {
				t.Errorf("%v: a refusal changed the fixture tree", c.args)
			}
		}
	})

	// The domain checks, not the parser, own these refusals: each is refused through the
	// command tree AND with the tree bypassed (the verb's run function called directly).
	t.Run("domain-refusals", func(t *testing.T) {
		empty := filepath.Join(f.root, "empty")
		if err := os.MkdirAll(empty, 0o755); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name   string
			env    map[string]string
			args   []string
			direct func(out, errw *bytes.Buffer) int
			code   int
			msg    string
		}{
			{"malformed-tag", nil, []string{"check-spec-header", "1.2.3", f.spec},
				func(o, e *bytes.Buffer) int { return runSpecHeader("1.2.3", f.spec, o, e) }, 2, "FAIL usage: tag must be vMAJOR.MINOR.PATCH (got '1.2.3')"},
			{"stale-spec-header", nil, []string{"check-spec-header", "v9.9.9", f.spec},
				func(o, e *bytes.Buffer) int { return runSpecHeader("v9.9.9", f.spec, o, e) }, 1, "names statusgen v1.2.3, but this release is cutting v9.9.9"},
			{"nothing-to-aggregate", nil, []string{"highlights", empty, f.changelog},
				func(o, e *bytes.Buffer) int { return runHighlights(empty, f.changelog, "", o, e) }, 2, "nothing to aggregate"},
			{"roll-nothing-to-aggregate", nil, []string{"roll", empty, f.changelog, "v0.2.0", "2026-02-02"},
				func(_, e *bytes.Buffer) int { return runRoll(empty, f.changelog, "v0.2.0", "2026-02-02", "", e) }, 2, "nothing to aggregate"},
			{"no-docs-streams", nil, []string{"lint-decision-defaults", empty},
				func(o, e *bytes.Buffer) int { return runLintDecision(empty, o, e) }, 2, "refused: " + empty + "/docs/streams not found"},
			{"base-sha-required", nil, []string{"check"}, nil, 1, "BASE_SHA: BASE_SHA is required"},
		}
		for _, c := range cases {
			r := record(t)
			for k, v := range c.env {
				r.env[k] = v
			}
			code, _, stderr := r.exec(c.args...)
			if code != c.code || !strings.Contains(stderr, c.msg) {
				t.Errorf("%s through the tree: exit %d stderr %q, want %d with %q", c.name, code, stderr, c.code, c.msg)
			}
			if c.direct == nil {
				continue
			}
			var o, e bytes.Buffer
			if code := c.direct(&o, &e); code != c.code || !strings.Contains(e.String(), c.msg) {
				t.Errorf("%s tree bypassed: exit %d stderr %q, want %d with %q (the refusal must live below the parser)", c.name, code, e.String(), c.code, c.msg)
			}
		}
		if before, _ := os.ReadFile(f.changelog); !strings.Contains(string(before), "## Unreleased\n\n## v0.1.0") {
			t.Errorf("a refused roll changed the changelog:\n%s", before)
		}

		// The classifier fails closed: no roster at all credits nobody, through the tree and not.
		r := record(t)
		if code, out, _ := r.exec("credit-identity", "classify", "octocat-example"); code != 0 || out != "skip:unknown\n" {
			t.Errorf("no roster through the tree: exit %d stdout %q, want skip:unknown", code, out)
		}
		set := cli.Declare(&cobra.Command{Use: "classify"}, creditBindings()...)
		v, err := set.Resolve(cli.Inputs{LookupEnv: func(string) (string, bool) { return "", false }})
		if err != nil || classifyLogin("octocat-example", "", v) != "skip:unknown" {
			t.Errorf("no roster, tree bypassed: %v %q, want skip:unknown", err, classifyLogin("octocat-example", "", v))
		}
	})

	t.Run("version", func(t *testing.T) {
		r := record(t)
		if code, out, _ := r.exec("--version"); code != 0 || !strings.HasPrefix(out, "deskchangelog sourceSHA=") || !strings.HasSuffix(out, "\n") {
			t.Errorf("--version: exit %d stdout %q, want the version line", code, out)
		}
	})
}
