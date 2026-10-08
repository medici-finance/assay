package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The supervised command of `scratch <cell> run` is an opaque argv. The legacy parser stopped at
// the first word past the action and handed everything after it to the child untouched, with or
// without `--`; these tests pin that boundary end to end (a child that records its own argv) and
// over the whole tree (no command that runs an argv lets the flag parser into it).

// commitFixtureRepo turns dir into a Git checkout with one commit holding f.txt.
func commitFixtureRepo(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", dir},
		{"-C", dir, "add", "f.txt"},
		{"-C", dir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture"},
	} {
		g := exec.Command("git", args...)
		g.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
		if out, err := g.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestScratchRunArgvBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the argv probe is a POSIX shell script")
	}
	w := newCLIWorld(t, "")
	srcA, srcB := filepath.Join(w.root, "src-a"), filepath.Join(w.root, "src-b")
	commitFixtureRepo(t, srcA, "a\n")
	commitFixtureRepo(t, srcB, "b\n")
	out := filepath.Join(w.root, "probe.out")
	// The probe records which checkout the run was admitted against, whether a declared input
	// was materialised into its workspace, and its own argv, one word per line.
	probe := "#!/bin/sh\n{ printf 'source=%s\\n' \"$ASSAY_SOURCE_ROOT\"; [ -e f.txt ] && echo input=yes || echo input=no\n" +
		"for a in \"$@\"; do printf 'arg=%s\\n' \"$a\"; done; } > " + out + "\n"
	if err := os.WriteFile(filepath.Join(w.binDir, "argv-probe"), []byte(probe), 0o755); err != nil {
		t.Fatal(err)
	}
	head := []string{"scratch", "example", "run", "--source", srcA, "--session", "s", "--task", "t"}
	cases := []struct {
		name string
		pre  []string // wrapper words after head, before the command
		tail []string // the probe's own arguments
		snap bool     // the wrapper itself asked for a snapshot, so f.txt is materialised
	}{
		{"source-and-snapshot", nil, []string{"--source", srcB, "--snapshot", "x"}, false},
		{"input", nil, []string{"--input", "f.txt", "x"}, false},
		{"apply-and-task", nil, []string{"--apply", "--task", "hijack", "x"}, false},
		{"help", nil, []string{"--help"}, false},
		{"short-help", nil, []string{"-h"}, false},
		{"unknown-short", nil, []string{"-n", "x"}, false},
		{"single-dash-long", nil, []string{"-source", srcB, "-task=hijack"}, false},
		{"selector", nil, []string{"--cells-root", w.root, "--version"}, false},
		{"equals-forms", []string{"--revision=HEAD", "--snapshot"}, []string{"--source=" + srcB, "--input=f.txt"}, true},
		{"explicit-dash", []string{"--"}, []string{"--apply", "--task", "keep", "x"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(out)
			args := append(append(append(append([]string{}, head...), tc.pre...), "argv-probe"), tc.tail...)
			r := w.run(t, nil, args...)
			if r.Code != 0 {
				t.Fatalf("cellctl %v: exit %d, want 0 (the child must run)\nstdout:\n%s\nstderr:\n%s", args, r.Code, r.Stdout, r.Stderr)
			}
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("cellctl %v: the child never ran (%v)\nstdout:\n%s\nstderr:\n%s", args, err, r.Stdout, r.Stderr)
			}
			input := "no"
			if tc.snap {
				input = "yes"
			}
			want := "source=" + srcA + "\ninput=" + input + "\n"
			for _, a := range tc.tail {
				want += "arg=" + a + "\n"
			}
			if string(raw) != want {
				t.Errorf("cellctl %v: the child saw\n%s\nwant (source A, only what the wrapper asked for materialised, argv untouched)\n%s", args, raw, want)
			}
		})
	}
}

// argvVerbRE matches a Use line that advertises a command line the verb runs.
var argvVerbRE = regexp.MustCompile(`command \[args|\[args\.\.\.\]|<executable>`)

// unboundedArgvVerbs is the class guard: every command whose Use advertises a command line it runs
// must keep that argv out of the flag parser, either wholesale (DisableFlagParsing) or past a
// declared number of positionals (cli.OpaqueArgv). It returns the commands that do neither.
func unboundedArgvVerbs(root *cobra.Command) []string {
	var bad []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if argvVerbRE.MatchString(c.Use) && !c.DisableFlagParsing {
			if _, ok := c.Annotations["cli/opaque-argv-after"]; !ok {
				bad = append(bad, c.CommandPath())
			}
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(root)
	return bad
}

func TestArgvVerbsKeepTheirArgv(t *testing.T) {
	if bad := unboundedArgvVerbs(buildRoot()); len(bad) != 0 {
		t.Errorf("commands that run a command line but let the flag parser into it: %v", bad)
	}
	// Positive control: a planted verb that runs a command line with neither guard is flagged, so
	// a matcher that silently stops matching fails here instead of reporting the tree clean.
	planted := buildRoot()
	planted.AddCommand(&cobra.Command{Use: "plant <cell> [-- command [args]]", Run: func(*cobra.Command, []string) {}})
	if bad := unboundedArgvVerbs(planted); !reflect.DeepEqual(bad, []string{"cellctl plant"}) {
		t.Errorf("planted unguarded argv verb: guard reported %v, want [cellctl plant]", bad)
	}
	// Every bounded verb, driven through the parser with a probe in place of its handler: the
	// words past its positionals spell each of its own flags, the selector, help and version,
	// and must arrive as the command, word for word.
	saved := echoRoster
	echoRoster = func() {}
	defer func() { echoRoster = saved }()
	root := buildRoot()
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if n, ok := c.Annotations["cli/opaque-argv-after"]; ok {
			t.Run(c.Name(), func(t *testing.T) { checkOpaqueTail(t, c, n) })
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(root)
}

func checkOpaqueTail(t *testing.T, c *cobra.Command, n string) {
	t.Helper()
	tail := []string{"child", "--help", "-h", "--version", "--cells-root", "/elsewhere", "-n"}
	c.Flags().VisitAll(func(f *pflag.Flag) {
		tail = append(tail, "--"+f.Name, "v", "-"+f.Name, "--"+f.Name+"=v")
	})
	k, err := strconv.Atoi(n)
	if err != nil {
		t.Fatalf("%s: annotation %q is not a positional count", c.Name(), n)
	}
	pos := strings.Repeat("p ", k)
	args := append(append([]string{c.Name()}, strings.Fields(pos)...), tail...)
	var got []string
	dash := -2
	code := runTreeWith(t, args, func(root *cobra.Command) {
		for _, s := range root.Commands() {
			if s.Name() == c.Name() {
				s.RunE = func(cmd *cobra.Command, a []string) error {
					got, dash = a, cmd.ArgsLenAtDash()
					return nil
				}
			}
		}
	})
	if code != 0 || dash < 0 || !reflect.DeepEqual(got[dash:], tail) {
		t.Errorf("%s: exit %d, command %v (dash at %d); want exit 0 and the command %v untouched", c.Name(), code, got, dash, tail)
	}
}
