package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLI contract tests (the desktools-v2/47 shape). Help and version are exercised on the BUILT
// binary under hostile config homes with a PATH of recording stubs; the value tests run
// in-process with the typed handler (ensureFn) replaced by a recorder. The gate's own outcomes
// are proven in deskkit (decisionensure_test.go); TestEnsureBillsTheAuditByOutcome proves this
// command's wiring of them, against a fake forge.

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

func toolBinary(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deskdecision-bin-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(dir, "deskdecision")
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

func stubPATH(t *testing.T, binDir, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	for _, h := range []string{"git", "gh", "glab", "curl"} {
		script := "#!/bin/sh\necho \"RAN $0 $*\" >> '" + logPath + "'\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, h), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func execIn(t *testing.T, bin, dir string, env []string, args ...string) (int, string, string) {
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
	return code, out.String(), errb.String()
}

func helpForms() [][]string {
	forms := clicontract.HelpForms("ensure")
	forms = append(forms,
		[]string{"ensure", "brief.md", "--repo", "o/r", "-h"},
		[]string{"ensure", "brief.md", "--dry-run", "--help"},
	)
	return append(forms, clicontract.VersionForms...)
}

// regression: #6131 — help and version must answer offline: no roster read, no program
// started, no file touched, under every malformed or unreadable config home.
func TestCLIHelpOffline(t *testing.T) {
	bin := toolBinary(t)
	states := map[string]string{"roster-unreadable": "unreadable"}
	for _, m := range clicontract.MalformedConfigs {
		states["roster-"+m.Name] = m.Content
	}
	for name, content := range states {
		content := content
		t.Run(name, func(t *testing.T) {
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
			switch content {
			case "":
			case "unreadable":
				for _, f := range []string{"roster.env", "apps.env"} {
					if err := os.WriteFile(filepath.Join(cfg, f), []byte("ASSAY_DESKS_ENABLED=0\n"), 0o000); err != nil {
						t.Fatal(err)
					}
				}
			default:
				if err := os.WriteFile(filepath.Join(cfg, "roster.env"), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"), "KUBECONFIG=/dev/null"}
			before := snapshotTree(root)
			for _, args := range helpForms() {
				code, out, stderr := execIn(t, bin, work, env, args...)
				if code != 0 || stderr != "" || strings.TrimSpace(out) == "" {
					t.Errorf("%v: exit %d stdout %q stderr %q; want 0, help on stdout only", args, code, out, stderr)
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

type recorder struct {
	calls, pres int
	opts        ensureOptions
	vals        *cli.Values
}

func record(t *testing.T) *recorder {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r := &recorder{}
	prevE, prevO := ensureFn, onResolve
	ensureFn = func(o ensureOptions, stdout, _ io.Writer) int {
		r.calls++
		r.opts = o
		fmt.Fprintln(stdout, "ensured")
		return 0
	}
	onResolve = func(v *cli.Values) { r.vals = v }
	t.Cleanup(func() { ensureFn, onResolve = prevE, prevO })
	return r
}

func (r *recorder) exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := execute(args, &out, &errb, func(io.Writer) bool { r.pres++; return true })
	return code, out.String(), errb.String()
}

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

// regression: #6131 — every value reaches the gate byte-identical in each spelling, defaults
// match the helper's (--at start, the dispatcher role), and no environment variable stands in
// for a flag.
func TestCLIConfigFlow(t *testing.T) {
	t.Run("spellings-and-values", func(t *testing.T) {
		for _, flag := range []string{"repo", "at", "as"} {
			for _, form := range clicontract.OptionForms {
				if form.GoFlagOnly {
					continue
				}
				for _, pv := range clicontract.PathValues {
					r := record(t)
					code, _, stderr := r.exec(append([]string{"ensure", "brief.md"}, form.Args(flag, pv.Value)...)...)
					got := map[string]string{"repo": r.opts.repo, "at": r.opts.at, "as": r.opts.as}[flag]
					if code != 0 || r.calls != 1 || got != pv.Value || r.vals.Source(flag) != cli.Flag {
						t.Errorf("--%s %s/%s: exit %d got %q, want %q from the flag\n%s", flag, form.Name, pv.Name, code, got, pv.Value, stderr)
					}
				}
			}
		}
	})
	t.Run("defaults-and-no-env", func(t *testing.T) {
		for _, env := range []string{"REPO", "AT", "AS", "DESKDECISION_REPO", "DESKDECISION_AT", "DESKDECISION_AS", "DESK_LOOP"} {
			t.Setenv(env, "env-value")
		}
		r := record(t)
		code, _, stderr := r.exec("ensure", "b.md")
		want := ensureOptions{brief: "b.md", at: "start", as: deskkit.DispatcherRole}
		if code != 0 || r.opts != want {
			t.Errorf("defaults: exit %d got %+v want %+v\n%s", code, r.opts, want, stderr)
		}
	})
	t.Run("declared-sources", func(t *testing.T) {
		declared := map[string]bool{}
		for _, b := range bindings() {
			if b.Config || b.Secret || len(b.Env) > 0 {
				t.Errorf("binding %q declares a config, secret or environment source; flags and defaults only", b.Key)
			}
			declared["ensure --"+b.Flag] = true
		}
		for _, f := range treeFlags(newTree(io.Discard, io.Discard, nil)) {
			if !declared[f] {
				t.Errorf("%s is registered on the tree but not declared as a binding", f)
			}
		}
	})
}

// regression: #6131 — the helper's own argv (`ensure <brief> --repo R --at start`, as
// deskdispatch and the board tools invoke it) reaches the gate unchanged; a command line it
// cannot parse is refused (5) before any startup; the gate's code passes through.
func TestCLILegacyForms(t *testing.T) {
	t.Run("helper-argv", func(t *testing.T) {
		cases := []struct {
			args []string
			want ensureOptions
		}{
			{[]string{"ensure", "docs/s/brief-07.md", "--repo", "o/r", "--at", "start"}, ensureOptions{brief: "docs/s/brief-07.md", repo: "o/r", at: "start", as: deskkit.DispatcherRole}},
			{[]string{"ensure", "b.md", "--held"}, ensureOptions{brief: "b.md", at: "start", as: deskkit.DispatcherRole, held: true}},
			{[]string{"ensure", "b.md", "--dry-run", "--at", "creation"}, ensureOptions{brief: "b.md", at: "creation", as: deskkit.DispatcherRole, dryRun: true}},
			{[]string{"ensure", "--repo", "o/r", "b.md", "--as", "worker"}, ensureOptions{brief: "b.md", repo: "o/r", at: "start", as: "worker"}},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 0 || r.calls != 1 || r.pres != 1 || r.opts != c.want || !strings.Contains(out, "ensured") {
				t.Errorf("%v: exit %d calls %d startup %d got %+v; want %+v\n%s", c.args, code, r.calls, r.pres, r.opts, c.want, stderr)
			}
		}
	})
	t.Run("states-pass-through", func(t *testing.T) {
		for _, want := range []int{0, 4, 5, 6} {
			r := record(t)
			ensureFn = func(ensureOptions, io.Writer, io.Writer) int { r.calls++; return want }
			if code, _, _ := r.exec("ensure", "b.md"); code != want || r.calls != 1 {
				t.Errorf("state exit %d: got exit %d calls %d", want, code, r.calls)
			}
		}
	})
	t.Run("inactive-verb-before-the-gate", func(t *testing.T) {
		r := record(t)
		var out, errb bytes.Buffer
		code := execute([]string{"ensure", "b.md"}, &out, &errb, func(io.Writer) bool { r.pres++; return false })
		if code != 6 || r.pres != 1 || r.calls != 0 {
			t.Errorf("inactive verb: exit %d startup %d gate %d; want 6, 1, 0", code, r.pres, r.calls)
		}
	})
	t.Run("refusals-before-the-gate", func(t *testing.T) {
		cases := []struct {
			args []string
			msg  string
		}{
			{nil, "choose a verb"},
			{[]string{"bogus"}, `unknown command "bogus"`},
			{[]string{"ensure"}, "ensure needs a brief file"},
			{[]string{"ensure", "a.md", "b.md"}, `unknown argument "b.md"`},
			{[]string{"ensure", "a.md", "--bogus"}, "unknown flag: --bogus"},
			{[]string{"ensure", "a.md", "--repo"}, "flag needs an argument"},
			{[]string{"ensure", "a.md", "--held=maybe"}, `invalid argument "maybe"`},
		}
		for _, c := range cases {
			r := record(t)
			code, out, stderr := r.exec(c.args...)
			if code != 5 || out != "" || r.calls != 0 || r.pres != 0 {
				t.Errorf("%v: exit %d stdout %q gate %d startup %d; want 5, nothing printed, nothing run", c.args, code, out, r.calls, r.pres)
			}
			if !strings.Contains(stderr, c.msg) {
				t.Errorf("%v: stderr %q, want %q", c.args, stderr, c.msg)
			}
		}
	})
	t.Run("version", func(t *testing.T) {
		r := record(t)
		if code, out, stderr := r.exec("--version"); code != 0 || !strings.HasPrefix(out, "deskdecision sourceSHA=") || stderr != "" {
			t.Errorf("--version: exit %d stdout %q stderr %q", code, out, stderr)
		}
	})
}

// emptyForge answers every lookup with nothing and files issue #7.
type emptyForge struct {
	role   string
	filed  int
	failOn string
}

func (f *emptyForge) GetIssue(deskkit.ForgeRepo, int) (*deskkit.Issue, error) {
	return nil, errors.New("no such issue")
}
func (f *emptyForge) ListIssues(deskkit.ForgeRepo, deskkit.IssueListQuery) (*deskkit.IssueList, error) {
	return &deskkit.IssueList{}, nil
}
func (f *emptyForge) SearchIssues(deskkit.ForgeRepo, deskkit.SearchIssuesInput) ([]deskkit.IssueSearchResult, error) {
	if f.failOn == "search" {
		return nil, errors.New("search unavailable")
	}
	return nil, nil
}
func (f *emptyForge) ListLabels(deskkit.ForgeRepo) ([]string, error) { return nil, nil }
func (f *emptyForge) FileIssue(deskkit.ForgeRepo, deskkit.IssueInput) (*deskkit.IssueRef, error) {
	f.filed++
	if f.failOn == "file" {
		return nil, errors.New("connection reset")
	}
	return &deskkit.IssueRef{Number: 7, URL: "https://forge.example.invalid/o/r/issues/7"}, nil
}
func (f *emptyForge) ApplyLabels(deskkit.ForgeRepo, int, deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	return &deskkit.LabelOutcome{}, nil
}

const gatedBrief = `---
brief: assay:ex:sample:07
title: "Pick the retention window"
gate: human
decision-trigger: start
why: Logs grow without bound.
---

# Sample

## Human decision

Context: logs grow.

1. Keep 30 days.
2. Keep 90 days.

Default if no answer: option 1 after 2026-12-01

## Deliverables

none
`

// lastAudit returns the last audit line deskdecision wrote under HOME.
func lastAudit(t *testing.T) map[string]any {
	t.Helper()
	var last map[string]any
	_ = filepath.WalkDir(os.Getenv("HOME"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(p) != "audit.jsonl" {
			return nil
		}
		fh, e := os.Open(p)
		if e != nil {
			return nil
		}
		defer fh.Close()
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal(sc.Bytes(), &m) == nil && m["tool"] == toolName {
				last = m
			}
		}
		return nil
	})
	if last == nil {
		t.Fatal("deskdecision wrote no audit line")
	}
	return last
}

// regression: #6131 — the command runs the gate under the --as role's forge, charges the
// write budget only for a real filing, and audits each outcome so only a filing (or a filing
// that may have reached the forge) is billed.
func TestEnsureBillsTheAuditByOutcome(t *testing.T) {
	cases := []struct {
		name, failOn string
		args         []string
		budget       error
		code         int
		result       string
		filed        int
	}{
		{"created", "", nil, nil, 0, deskkit.ResultOK, 1},
		{"dry-run", "", []string{"--dry-run"}, nil, 0, deskkit.ResultDryRun, 0},
		{"rate-limited", "", nil, deskkit.RateLimited("budget spent"), 4, deskkit.ResultRateLimited, 0},
		{"lookup-failed", "search", nil, nil, 6, deskkit.ResultUnwritten, 0},
		{"filing-failed", "file", nil, nil, 6, deskkit.ResultUnverifiable, 1},
		{"refused", "", []string{"--at", "bogus"}, nil, 5, deskkit.ResultRefused, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("DESK_TOOLS_DISABLED", "")
			t.Setenv("DESK_LOOP", "")
			deskkit.ReloadConfig()
			t.Cleanup(deskkit.ReloadConfig)
			work := t.TempDir()
			if err := os.WriteFile(filepath.Join(work, "brief.md"), []byte(gatedBrief), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(work)
			f := &emptyForge{failOn: c.failOn}
			charged := 0
			prevF, prevA := forgeFor, allowCreate
			forgeFor = func(_ deskkit.ForgeRepo, role string) (deskkit.DecisionEnsureForge, error) {
				f.role = role
				return f, nil
			}
			allowCreate = func(string) error { charged++; return c.budget }
			t.Cleanup(func() { forgeFor, allowCreate = prevF, prevA })

			var out, errb bytes.Buffer
			args := append([]string{"ensure", "brief.md", "--repo", "o/r", "--as", "worker"}, c.args...)
			code := execute(args, &out, &errb, nil)
			if code != c.code || f.filed != c.filed {
				t.Fatalf("exit %d filed %d; want %d, %d\nstdout %s\nstderr %s", code, f.filed, c.code, c.filed, out.String(), errb.String())
			}
			if c.name != "refused" && f.role != "worker" {
				t.Errorf("forge resolved as %q, want the --as role", f.role)
			}
			if wantCharged := map[bool]int{true: 1, false: 0}[c.filed > 0 || c.budget != nil]; charged != wantCharged {
				t.Errorf("write budget consulted %d times, want %d", charged, wantCharged)
			}
			a := lastAudit(t)
			if a["result"] != c.result || a["verb"] != "ensure" {
				t.Errorf("audit %v; want result %s", a, c.result)
			}
			if c.name == "created" && (!strings.Contains(fmt.Sprint(a["detail"]), "#7") || a["repo"] != "o/r") {
				t.Errorf("created audit names %v", a)
			}
		})
	}
}
