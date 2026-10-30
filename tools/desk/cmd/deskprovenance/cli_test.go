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

// The CLI contract tests (desktools-v2/46). Help and version are exercised on the BUILT binary
// with a hostile config home and a PATH of recording stubs; the value tests run in-process with
// the typed handler (provenanceFn) replaced by a recorder, and the gate tests run the REAL
// handler with only the forge read (gatherFn) replaced, so nothing here reaches a forge.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

// toolBinary builds this package's binary once per test process.
func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskprovenance-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskprovenance")
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

// helpForms are every help and version spelling, alone and after the flags a run takes.
// deskprovenance has no verbs, so the contract's "help <sub>" forms do not apply.
func helpForms() [][]string {
	forms := [][]string{{"-h"}, {"--help"}, {"help"}}
	for _, f := range clicontract.GoFlagHelpForms("") {
		if len(f) == 1 {
			forms = append(forms, f)
		}
	}
	forms = append(forms,
		[]string{"--dry-run", "-h"},
		[]string{"--repo", "o/n", "--pr", "7", "--post", "--help"},
		[]string{"-dry-run", "-fixture", "testdata/ordinary-author.json", "-help"},
	)
	forms = append(forms, clicontract.VersionForms...)
	return append(forms, clicontract.GoFlagVersionForms...)
}

// recorder replaces the handler for one test.
type recorder struct {
	calls int
	got   provOpts
	vals  *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevP, prevO, prevG := provenanceFn, onResolve, gatherFn
	provenanceFn = func(o provOpts, stdout, _ io.Writer) int {
		r.calls++
		r.got = o
		fmt.Fprintln(stdout, "card")
		return 0
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { provenanceFn, onResolve, gatherFn = prevP, prevO, prevG })
	return r
}

// realHandler puts the real handler back for one test and replaces only the forge read, which
// it counts.
func (r *recorder) realHandler() *int {
	reads := 0
	provenanceFn = func(o provOpts, stdout, stderr io.Writer) int {
		r.calls++
		r.got = o
		return provenance(o, stdout, stderr)
	}
	gatherFn = func(string, int, string) (deskkit.ProvenanceInput, error) {
		reads++
		body := "an ordinary change"
		paths := []string{"README.md"}
		return deskkit.ProvenanceInput{ThisBody: &body, ChangedPaths: &paths}, nil
	}
	return &reads
}

// exec runs one in-process invocation.
func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

// regression: a help request went through the flag package's parse error — usage on stderr,
// exit 2 — so help was not a clean, offline, stdout answer from any position.
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
					t.Errorf("%v: wrote to stderr (a gather or a read ran): %q", args, r.stderr)
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
		reads := r.realHandler()
		for _, args := range forms {
			if code, out, _ := r.exec(args...); code != 0 || out == "" {
				t.Errorf("%v: exit %d in-process, stdout %q", args, code, out)
			}
		}
		if r.calls != 0 || *reads != 0 {
			t.Errorf("help and version: handler ran %d times, forge read %d; want 0 and 0", r.calls, *reads)
		}
		// A live read today leaves most signals unread, so the card's verdict is could-not-check (6).
		if code, _, _ := r.exec("--repo", "o/n", "--pr", "7"); code != 6 || r.calls != 1 || *reads != 1 {
			t.Errorf("a real run: exit %d, handler %d, forge read %d; want 6, exactly 1, exactly 1", code, r.calls, *reads)
		}
	})
}

// regression: the flags reached the handler only through the flag package's spellings, and no
// source was declared for any of them.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				r := record(t)
				args := append([]string{"--dry-run"}, form.Args("fixture", pv.Value)...)
				code, _, stderr := r.exec(args...)
				if code != 0 || r.calls != 1 || r.got.fixture != pv.Value || r.vals.Source("fixture") != cli.Flag {
					t.Errorf("--fixture %s/%s: exit %d got %q (from %s), want %q from the flag\n%s", form.Name, pv.Name, code, r.got.fixture, r.vals.Source("fixture"), pv.Value, stderr)
				}
			}
		}
	})
	t.Run("precedence", func(t *testing.T) {
		cases := []struct {
			name    string
			args    []string
			role    string
			roleSrc cli.Source
		}{
			{"default", []string{"--repo", "o/n", "--pr", "7"}, "worker", cli.Default},
			{"flag-over-default", []string{"--repo", "o/n", "--pr", "7", "--role", "reviewer"}, "reviewer", cli.Flag},
			{"explicit-empty-wins-over-default", []string{"--repo", "o/n", "--pr", "7", "--role="}, "", cli.Flag},
			{"last-wins", []string{"--repo", "o/n", "--pr", "7", "--role", "a", "--role=b"}, "b", cli.Flag},
		}
		for _, env := range []string{"ROLE", "REPO", "PR", "POST", "DRY_RUN", "DESKPROVENANCE_ROLE", "DESKPROVENANCE_POST", "ASSAY_ROLE", "GH_TOKEN"} {
			t.Setenv(env, "env-value")
		}
		for _, c := range cases {
			r := record(t)
			code, _, stderr := r.exec(c.args...)
			if code != 0 || r.got.role != c.role || r.vals.Source("role") != c.roleSrc ||
				r.got.repo != "o/n" || r.got.pr != 7 || r.got.post || r.vals.Source("post") != cli.Default {
				t.Errorf("%s: exit %d got %+v (role from %s, post from %s); want role %q (%s), post false from the default\n%s",
					c.name, code, r.got, r.vals.Source("role"), r.vals.Source("post"), c.role, c.roleSrc, stderr)
			}
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; deskprovenance has flags and defaults only", b.Key)
			}
		}
	})
}

// regression: the flag package is gone; every form a consumer writes still reaches the handler
// with the same values, and both posting gates still refuse a live write on the real handler.
func TestCLILegacyForms(t *testing.T) {
	t.Run("consumer-forms", func(t *testing.T) {
		cases := []struct {
			args []string
			want provOpts
		}{
			{[]string{"--dry-run", "--fixture", "testdata/ordinary-author.json"}, provOpts{dryRun: true, fixture: "testdata/ordinary-author.json", role: "worker"}},
			{[]string{"--dry-run", "--repo", "o/n", "--pr", "12"}, provOpts{dryRun: true, repo: "o/n", pr: 12, role: "worker"}},
			{[]string{"--repo", "o/n", "--pr", "12", "--role", "reviewer"}, provOpts{repo: "o/n", pr: 12, role: "reviewer"}},
			{[]string{"--repo", "o/n", "--pr", "12", "--post"}, provOpts{repo: "o/n", pr: 12, post: true, role: "worker"}},
			{[]string{"-dry-run", "-repo", "o/n", "-pr", "12"}, provOpts{dryRun: true, repo: "o/n", pr: 12, role: "worker"}},
			{[]string{"--repo=o/n", "--pr=12", "--force-scan-override", "a reason long enough"}, provOpts{repo: "o/n", pr: 12, role: "worker", override: "a reason long enough"}},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.got != c.want || !strings.Contains(out, "card") {
				t.Errorf("%v: exit %d calls %d got %+v stdout %q; want 0, 1, %+v\n%s", c.args, code, r.calls, r.got, out, c.want, stderr)
			}
		}
	})
	t.Run("gates-on-the-real-handler", func(t *testing.T) {
		cases := []struct {
			name    string
			args    []string
			code    int
			reads   int
			card    bool
			message string
		}{
			{"dry-run-fixture-prints", []string{"--dry-run", "--fixture", "testdata/ordinary-author.json"}, 0, 0, true, ""},
			{"dry-run-flagged-fixture-keeps-its-verdict", []string{"--dry-run", "--fixture", "testdata/bulk-sweep-author.json"}, 1, 0, true, ""},
			{"fixture-requires-dry-run", []string{"--fixture", "testdata/ordinary-author.json"}, 2, 0, false, "--fixture requires --dry-run"},
			{"fixture-requires-dry-run-even-with-post", []string{"--fixture", "testdata/ordinary-author.json", "--post"}, 2, 0, false, "--fixture requires --dry-run"},
			{"gate-1-live-without-post-only-prints", []string{"--repo", "o/n", "--pr", "7"}, 6, 1, true, "--post was not given"},
			{"gate-1-dry-run-wins-over-post", []string{"--dry-run", "--repo", "o/n", "--pr", "7", "--post"}, 6, 1, true, ""},
			{"gate-2-post-is-refused", []string{"--repo", "o/n", "--pr", "7", "--post"}, 2, 1, false, liveCardRefusalReason},
			{"repo-without-pr", []string{"--repo", "o/n"}, 2, 0, false, "--repo and --pr are both required"},
			{"short-override-reason", []string{"--repo", "o/n", "--pr", "7", "--force-scan-override", "short"}, 5, 0, false, "at least"},
		}
		for _, c := range cases {
			r := record(t)
			reads := r.realHandler()
			code, out, stderr := r.exec(c.args...)
			if code != c.code || *reads != c.reads || (strings.TrimSpace(out) != "") != c.card {
				t.Errorf("%s: exit %d forge reads %d stdout %q; want %d, %d, card printed %v\n%s", c.name, code, *reads, out, c.code, c.reads, c.card, stderr)
			}
			if c.message != "" && !strings.Contains(stderr, c.message) {
				t.Errorf("%s: stderr %q, want it to contain %q", c.name, stderr, c.message)
			}
		}
	})
	t.Run("refusals", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{[]string{"--bogus"}, "unknown flag: --bogus"},
			{[]string{"--pr", "seven"}, `invalid argument "seven"`},
			{[]string{"--repo"}, "flag needs an argument"},
			{[]string{"--dry-run", "extra"}, `unexpected argument "extra"`},
			{[]string{"version"}, `unexpected argument "version"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 2 || out != "" || r.calls != 0 {
				t.Errorf("%v: exit %d stdout %q handler %d; want 2, nothing printed, nothing run", c.args, code, out, r.calls)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want it to contain %q", c.args, stderr, c.msg)
			}
		}
		r := record(t)
		reads := r.realHandler()
		if code, out, stderr := r.exec(); code != 2 || out != "" || *reads != 0 || !strings.Contains(stderr, "deskprovenance --dry-run --fixture") {
			t.Errorf("bare: exit %d stdout %q reads %d stderr %q; want 2 and the usage on stderr", code, out, *reads, stderr)
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		for _, args := range [][]string{{"--version"}, {"-version"}} {
			if code, out, stderr := r.exec(args...); code != 0 || !strings.HasPrefix(out, "deskprovenance sourceSHA=") || !strings.HasSuffix(out, "\n") || stderr != "" {
				t.Errorf("%v: exit %d stdout %q stderr %q, want the version line", args, code, out, stderr)
			}
		}
	})
}
