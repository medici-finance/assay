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
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLI contract tests (desktools-v2/35). Help and version run on the BUILT binary under a
// hostile home, a dead proxy and a PATH of recording stubs; the value tests run in process with
// the typed handler (dispatchFn) replaced by a recorder. The admission test drives a tampered
// artifact through the command tree and directly into Install, with an injected fetcher, so
// nothing here reaches the network.

var (
	instBinOnce sync.Once
	instBinPath string
	instBinErr  error
)

func instBinary(t *testing.T) string {
	t.Helper()
	instBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskinstall-bin-")
		if err != nil {
			instBinErr = err
			return
		}
		out := filepath.Join(dir, "deskinstall")
		if runtime.GOOS == "windows" {
			out += ".exe"
		}
		if b, err := goquiet.Cmd(exec.Command("go", "build", "-o", out, ".")).CombinedOutput(); err != nil {
			instBinErr = fmt.Errorf("go build: %v\n%s", err, b)
			return
		}
		instBinPath = out
	})
	if instBinErr != nil {
		t.Fatalf("building the binary under test: %v", instBinErr)
	}
	return instBinPath
}

// fsPrint fingerprints every path under dir: name, mode, size and mtime.
func fsPrint(dir string) string {
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

// stubPATH writes recording stubs for every program an installer might start.
func stubPATH(t *testing.T, binDir, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	for _, h := range []string{"curl", "wget", "git", "gh", "glab", "tar", "pwsh"} {
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

func execBin(t *testing.T, bin, dir string, env []string, args ...string) binRun {
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

// instHelpForms are the help and version spellings deskinstall answers: the root forms of the
// contract (it has no verbs), the bare `help` and `version` words, and -version.
func instHelpForms() [][]string {
	return [][]string{{"-h"}, {"--help"}, {"help"}, {"--version"}, {"-version"}, {"version"}}
}

// instRecorder replaces the typed handler for one test.
type instRecorder struct {
	calls    int
	o        options
	src      map[string]string
	out, err bytes.Buffer
}

func recordInst(t *testing.T) *instRecorder {
	t.Helper()
	r := &instRecorder{}
	prevD, prevO := dispatchFn, onResolve
	dispatchFn = func(o options, stdout, _ io.Writer) int {
		r.calls++
		r.o = o
		fmt.Fprintln(stdout, "dispatched")
		return deskkit.ExitOK
	}
	onResolve = func(v *cli.Values) {
		r.src = map[string]string{}
		for _, b := range bindings() {
			r.src[b.Key] = v.Source(b.Key).String()
		}
	}
	t.Cleanup(func() { dispatchFn, onResolve = prevD, prevO })
	return r
}

func (r *instRecorder) exec(args ...string) (int, string, string) {
	r.out.Reset()
	r.err.Reset()
	code := run(args, &r.out, &r.err)
	return code, r.out.String(), r.err.String()
}

// regression: the hand-rolled parser printed usage on stderr; help must now answer on stdout,
// exit 0, before any file, network or program effect, under a hostile environment.
func TestCLIHelpOffline(t *testing.T) {
	bin := instBinary(t)
	forms := instHelpForms()
	type state struct {
		name, config string
		extra        []string
	}
	states := []state{
		{"clean", "", nil},
		{"dead-proxy", "", []string{"HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9"}},
		{"unwritable-home", "", nil},
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
			// A manifest where a careless default would look for one.
			if err := os.WriteFile(filepath.Join(work, manifestName), []byte("schema: paired-versions-v1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if st.config != "" {
				for _, name := range []string{"deskinstall.env", "config.env", "deskinstall.yaml"} {
					if err := os.WriteFile(filepath.Join(cfgDir, name), []byte(st.config), 0o600); err != nil {
						t.Fatal(err)
					}
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
			before := fsPrint(root)
			for _, args := range forms {
				r := execBin(t, bin, work, env, args...)
				if r.code != 0 || r.stderr != "" || strings.TrimSpace(r.stdout) == "" {
					t.Errorf("%v: exit %d, stdout %d bytes, stderr %q; want 0, output, no stderr", args, r.code, len(r.stdout), r.stderr)
				}
			}
			if b, err := os.ReadFile(logPath); err == nil {
				t.Errorf("help or version started a program:\n%s", b)
			}
			if after := fsPrint(root); after != before {
				t.Errorf("help or version changed the filesystem:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}

	t.Run("effect-counters", func(t *testing.T) {
		r := recordInst(t)
		for _, args := range forms {
			if code, out, _ := r.exec(args...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", args, code, out)
			}
		}
		if r.calls != 0 {
			t.Errorf("help and version reached the handler %d times; want none", r.calls)
		}
	})

	t.Run("help-text", func(t *testing.T) {
		r := recordInst(t)
		_, out, _ := r.exec("--help")
		for _, b := range bindings() {
			if !strings.Contains(out, "--"+b.Flag) {
				t.Errorf("--help does not list --%s", b.Flag)
			}
		}
		for _, want := range []string{"no flag skips it", "MODE 1", "MODE 2", "5 refused"} {
			if !strings.Contains(out, want) {
				t.Errorf("--help lost %q", want)
			}
		}
		if strings.Contains(out, "completion") {
			t.Errorf("help offers the completion command, which the adapter disables")
		}
	})

	t.Run("bare-root-refuses", func(t *testing.T) {
		r := recordInst(t)
		code, out, errOut := r.exec()
		if code != deskkit.ExitRefused || out != "" || !strings.Contains(errOut, "MODE 1") || r.calls != 0 {
			t.Errorf("no arguments: exit %d stdout %q handler %d; want 5, help on stderr, no handler", code, out, r.calls)
		}
	})
}

// regression: every value reached deskinstall through a hand-rolled loop; each path must
// arrive byte-identical through every spelling, and nothing may bind from the environment.
func TestCLIConfigFlow(t *testing.T) {
	type pathKey struct {
		key string
		got func(o options) string
	}
	keys := []pathKey{
		{"manifest", func(o options) string { return o.manifest }},
		{"dest", func(o options) string { return o.dest }},
		{"repo", func(o options) string { return o.repo }},
		{"bundle", func(o options) string { return o.bundle }},
		{"releases", func(o options) string { return o.releases }},
		{"pack", func(o options) string { return o.pack }},
	}

	t.Run("spellings-and-values", func(t *testing.T) {
		for _, k := range keys {
			for _, form := range clicontract.OptionForms {
				for _, pv := range clicontract.PathValues {
					r := recordInst(t)
					code, _, stderr := r.exec(form.Args(k.key, pv.Value)...)
					if form.GoFlagOnly {
						// deskinstall never took single-dash flags; they stay unknown arguments.
						if code != deskkit.ExitRefused || r.calls != 0 || !strings.Contains(stderr, "unknown argument") {
							t.Errorf("%s %s/%s: exit %d handler %d, want 5 unknown argument\n%s", k.key, form.Name, pv.Name, code, r.calls, stderr)
						}
						continue
					}
					if got := k.got(r.o); code != 0 || r.calls != 1 || got != pv.Value || r.src[k.key] != "flag" {
						t.Errorf("%s %s/%s: exit %d value %q (from %s), want %q from the flag\n%s", k.key, form.Name, pv.Name, code, got, r.src[k.key], pv.Value, stderr)
					}
				}
			}
		}
	})

	t.Run("precedence", func(t *testing.T) {
		// Every key is declared from a flag and its zero default only, so the matrix rows those
		// sources can express are: nothing set, flag set, empty flag, and last flag wins.
		cases := []struct {
			name, want, src string
			args            []string
		}{
			{"nothing-set", "", "unset", []string{"--check"}},
			{"flag-sets", "flg", "flag", []string{"--release", "flg"}},
			{"empty-flag-is-set", "", "flag", []string{"--release="}},
			{"last-flag-wins", "b", "flag", []string{"--release", "a", "--release", "b"}},
		}
		for _, c := range cases {
			r := recordInst(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.o.release != c.want || r.src["release"] != c.src {
				t.Errorf("%s: exit %d release %q (from %s), want %q from %s\n%s", c.name, code, r.o.release, r.src["release"], c.want, c.src, stderr)
			}
		}
	})

	t.Run("env-is-not-a-source", func(t *testing.T) {
		for _, b := range bindings() {
			up := strings.ToUpper(strings.ReplaceAll(b.Key, "-", "_"))
			t.Setenv(up, "from-env")
			t.Setenv("DESKINSTALL_"+up, "from-env")
		}
		t.Setenv("CHECK", "true")
		t.Setenv("DESKINSTALL_DRY_RUN", "true")
		r := recordInst(t)
		code, _, stderr := r.exec("--manifest", "m")
		if code != 0 || r.o != (options{manifest: "m"}) {
			t.Errorf("an environment variable named like a key changed a setting: exit %d %+v\n%s", code, r.o, stderr)
		}
	})

	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 || b.Flag != b.Key || b.Default != nil {
				t.Errorf("binding %q must be a plain flag of the same name, no env/config/default: %+v", b.Key, b)
			}
		}
	})
}

// regression: the hand-rolled loop is gone; every spelling a consumer writes still reaches the
// handler, and an unknown word, a missing value, a stray word or `--` refuses with exit 5.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		forms := []struct {
			args []string
			want options
		}{
			{[]string{"--manifest", "plugins/assay/paired-versions.yaml", "--dest", `C:\Tools\Assay\bin`},
				options{manifest: "plugins/assay/paired-versions.yaml", dest: `C:\Tools\Assay\bin`}},
			{[]string{"--manifest", "m.yaml", "--dest", "d", "--platform", "windows-arm64"},
				options{manifest: "m.yaml", dest: "d", platform: "windows-arm64"}},
			{[]string{"--harness", "cursor", "--forge", "github", "--repo", "../..", "--bundle", "../../plugins/assay", "--check"},
				options{harness: "cursor", forge: "github", repo: "../..", bundle: "../../plugins/assay", check: true}},
			{[]string{"--harness", "cursor", "--bundle", "../../plugins/assay", "--repo", "../..", "--forge", "github"},
				options{harness: "cursor", forge: "github", repo: "../..", bundle: "../../plugins/assay"}},
			{[]string{"--harness", "cursor", "--bundle", "../../plugins/assay", "--release", "v1.2.3", "--pack", "../../cursor-bundle.tar.gz"},
				options{harness: "cursor", bundle: "../../plugins/assay", release: "v1.2.3", pack: "../../cursor-bundle.tar.gz"}},
			{[]string{"--harness", "cursor", "--release", "v1.2.3", "--forge", "gitlab", "--repo", `C:\src\myrepo`, "--fetch", "--dry-run"},
				options{harness: "cursor", release: "v1.2.3", forge: "gitlab", repo: `C:\src\myrepo`, fetch: true, dryRun: true}},
			{[]string{"--harness", "cursor", "--release", "v1.2.3", "--releases", "rel", "--release-home", "o/r", "--repo", "x", "--forge", "github"},
				options{harness: "cursor", release: "v1.2.3", releases: "rel", releaseHome: "o/r", repo: "x", forge: "github"}},
			{[]string{"--harness", "cursor", "--repo", "/absolute/cell/worktrees/worker-desk"},
				options{harness: "cursor", repo: "/absolute/cell/worktrees/worker-desk"}},
			// A value that looks like a flag is still the value, as the loop took the next word.
			{[]string{"--manifest", "-x", "--dest", "--check"}, options{manifest: "-x", dest: "--check"}},
		}
		for _, f := range forms {
			r := recordInst(t)
			code, _, stderr := r.exec(f.args...)
			if code != 0 || r.calls != 1 || r.o != f.want {
				t.Errorf("%v: exit %d handler %d options %+v, want %+v\n%s", f.args, code, r.calls, r.o, f.want, stderr)
			}
		}
	})

	t.Run("refusals-exit-5", func(t *testing.T) {
		cases := []struct {
			args []string
			want string
		}{
			{[]string{"--bogus"}, `unknown argument "--bogus"`},
			{[]string{"--no-attest=true", "--manifest", "m"}, `unknown argument "--no-attest=true"`},
			{[]string{"-manifest", "m"}, `unknown argument "-manifest"`},
			{[]string{"stray"}, `unknown argument "stray"`},
			{[]string{"--manifest", "m", "--dest", "d", "--"}, `unknown argument "--"`},
			{[]string{"--manifest", "m", "--", "--dest", "d"}, `unknown argument "--"`},
			{[]string{"--version", "--check"}, `unknown argument "--version"`},
			{[]string{"version", "--check"}, `unknown argument "version"`},
			// help answers only as the sole word; after (or before) flags it is refused
			// naming "help", never a declared flag.
			{[]string{"--manifest", "m", "help"}, `unknown argument "help"`},
			{[]string{"--check", "help"}, `unknown argument "help"`},
			{[]string{"help", "--manifest", "m"}, `unknown argument "help"`},
			{[]string{"--manifest"}, "--manifest"},
			{[]string{"--dest"}, "--dest"},
			{[]string{"--release"}, "--release"},
		}
		for _, c := range cases {
			r := recordInst(t)
			code, _, stderr := r.exec(c.args...)
			if code != deskkit.ExitRefused || r.calls != 0 || !strings.Contains(stderr, c.want) {
				t.Errorf("%v: exit %d handler %d stderr %q; want 5, no handler, naming %q", c.args, code, r.calls, stderr, c.want)
			}
		}
	})

	t.Run("mode-rules-unchanged", func(t *testing.T) {
		// The real dispatch, not the recorder: each refusal the loop made still exits 5
		// before any effect.
		cases := []struct {
			args []string
			want string
		}{
			{[]string{"--harness", "cursor", "--forge", "github", "--repo", "r", "--manifest", "m"}, "two install modes"},
			{[]string{"--harness", "cursor", "--forge", "github", "--repo", "r", "--platform", "windows-amd64"}, "--platform is only valid"},
			{[]string{"--manifest", "m", "--dest", "d", "--fetch"}, "require --harness cursor"},
			{[]string{"--manifest", "m", "--dest", "d", "--check"}, "--check is only meaningful"},
			{[]string{"--manifest", "m"}, "both required"},
			{[]string{"--harness", "vscode", "--forge", "github", "--repo", "r"}, "unknown --harness"},
		}
		for _, c := range cases {
			var out, errb bytes.Buffer
			code := run(c.args, &out, &errb)
			if code != deskkit.ExitRefused || !strings.Contains(errb.String(), c.want) {
				t.Errorf("%v: exit %d stderr %q; want 5 naming %q", c.args, code, errb.String(), c.want)
			}
		}
	})
}

// regression: hash verification is the single control between a tampered download and
// placement; it must refuse with the command tree in front AND with the adapter bypassed.
func TestCLIAdmissionBoundary(t *testing.T) {
	f := newAttestFixture(t)
	tampered := append([]byte(nil), f.sg...)
	tampered[0] ^= 0xff
	tamperFetch := func(url string) ([]byte, error) {
		if url == f.sgURL {
			return tampered, nil
		}
		return f.fetch(url)
	}
	inject := func(t *testing.T, fetch Fetcher) {
		prev := installFn
		installFn = func(o Options) error {
			o.Fetch, o.TrustedRoot = fetch, f.trust
			return Install(o)
		}
		t.Cleanup(func() { installFn = prev })
	}
	args := func(dest string) []string {
		return []string{"--manifest", f.manifest, "--dest", dest, "--platform", "windows-amd64"}
	}

	t.Run("control-clean-installs", func(t *testing.T) {
		inject(t, f.fetch)
		dest := t.TempDir()
		var out, errb bytes.Buffer
		if code := run(args(dest), &out, &errb); code != 0 {
			t.Fatalf("clean artifact through the command tree: exit %d\n%s", code, errb.String())
		}
		if _, err := os.Stat(filepath.Join(dest, "statusgen-windows-amd64.exe")); err != nil {
			t.Fatalf("clean artifact not placed: %v", err)
		}
	})

	t.Run("tampered-through-cobra", func(t *testing.T) {
		inject(t, tamperFetch)
		dest := t.TempDir()
		var out, errb bytes.Buffer
		code := run(args(dest), &out, &errb)
		if code != deskkit.ExitRefused || !strings.Contains(errb.String(), "mismatch") {
			t.Fatalf("SECURITY: tampered artifact through the command tree: exit %d stderr %q; want 5 naming the mismatch", code, errb.String())
		}
		assertNothingPlaced(t, dest)
	})

	t.Run("tampered-direct", func(t *testing.T) {
		dest := t.TempDir()
		err := Install(Options{ManifestPath: f.manifest, DestDir: dest, Platform: "windows-amd64", Fetch: tamperFetch, TrustedRoot: f.trust})
		if err == nil || !strings.Contains(err.Error(), "mismatch") {
			t.Fatalf("SECURITY: tampered artifact with the adapter bypassed was not refused: %v", err)
		}
		assertNothingPlaced(t, dest)
	})

	t.Run("mode-refusal-direct", func(t *testing.T) {
		var out, errb bytes.Buffer
		o := options{harness: "cursor", forge: "github", repo: t.TempDir(), manifest: f.manifest, dest: t.TempDir()}
		if code := dispatch(o, &out, &errb); code != deskkit.ExitRefused {
			t.Fatalf("two install modes with the adapter bypassed: exit %d, want 5", code)
		}
	})
}
