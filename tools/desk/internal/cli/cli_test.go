package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/clicontract"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// effects instruments everything a fixture handler can do past parsing. Help and version
// must leave every counter at zero.
type effects struct {
	preRuns    int // persistent hooks may load config or credentials too
	postRuns   int
	loads      int // config loader calls (stands in for cell/credential reads)
	admissions int // calls into the existing admission layer
	acts       int // external effects (a forge write, a file write)
	got        *opts
}

// opts is the typed handler input: what Viper resolved crosses here, nothing else.
type opts struct {
	Repo, Path, Token, Cell string
	RepoSrc, PathSrc        Source
	CellSrc                 Source
	Count                   int
	Dry                     bool
	Wait                    time.Duration
	Labels                  []string
	Args                    []string
}

type codedErr struct {
	code int
	msg  string
}

func (c codedErr) Error() string { return c.msg }
func (c codedErr) ExitCode() int { return c.code }

// fixture is one invocation's non-argv world.
type fixture struct {
	env    map[string]string
	config map[string]any
	cfgErr error // a malformed config: the loader fails if it is ever called
	fx     *effects
}

func (f *fixture) lookup(name string) (string, bool) { v, ok := f.env[name]; return v, ok }

// admit stands in for the existing admission layer (roster/custody/repo validation): the
// adapter cannot make a value admissible, so a rejected repo stops here with its own code.
func admit(o *opts) error {
	if !strings.Contains(o.Repo, "/") {
		return codedErr{3, "refused: repo must be owner/name"}
	}
	return nil
}

func (f *fixture) build() *cobra.Command {
	root := NewRoot("fixt", "fixture tool for the adapter contract")
	root.PersistentPreRunE = func(*cobra.Command, []string) error { f.fx.preRuns++; return nil }
	root.PersistentPostRunE = func(*cobra.Command, []string) error { f.fx.postRuns++; return nil }
	run := &cobra.Command{
		Use:   "run [target]",
		Short: "resolve settings and act",
		Args:  cobra.MaximumNArgs(1),
	}
	set := Declare(run,
		Binding{Key: "repo", Kind: String, Flag: "repo", Usage: "owner/name", Env: []string{"FIXT_REPO"}, Config: true, Default: clicontract.PrecedenceDefault + "/x"},
		Binding{Key: "path", Kind: String, Flag: "path", Usage: "a path", Env: []string{"FIXT_PATH"}, Config: true, Default: clicontract.PrecedenceDefault},
		Binding{Key: "count", Kind: Int, Flag: "count", Usage: "how many", Env: []string{"FIXT_COUNT"}, Default: 1},
		Binding{Key: "dry", Kind: Bool, Flag: "dry-run", Short: "n", Usage: "do nothing"},
		Binding{Key: "wait", Kind: Duration, Flag: "wait", Usage: "timeout", Config: true},
		Binding{Key: "label", Kind: StringArray, Flag: "label", Usage: "repeatable", Env: []string{"FIXT_LABEL"}},
		Binding{Key: "token", Kind: String, Secret: true, Env: []string{"FIXT_TOKEN"}},
		// cell.env overlays the inherited value: config outranks env for this key only.
		Binding{Key: "cell", Kind: String, Env: []string{"FIXT_CELL"}, Config: true, Order: []Source{Config, Env}},
	)
	run.RunE = func(cmd *cobra.Command, args []string) error {
		f.fx.loads++
		if f.cfgErr != nil {
			return codedErr{2, "config: " + f.cfgErr.Error()}
		}
		v, err := set.Resolve(Inputs{Config: f.config, LookupEnv: f.lookup})
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "fixt:", err)
			return codedErr{2, err.Error()}
		}
		o := &opts{
			Repo: v.String("repo"), RepoSrc: v.Source("repo"),
			Path: v.String("path"), PathSrc: v.Source("path"),
			Cell: v.String("cell"), CellSrc: v.Source("cell"),
			Token: v.String("token"), Count: v.Int("count"), Dry: v.Bool("dry"),
			Wait: v.Duration("wait"), Labels: v.Strings("label"), Args: args,
		}
		f.fx.got = o
		f.fx.admissions++
		if err := admit(o); err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			return err
		}
		f.fx.acts++
		fmt.Fprintf(cmd.OutOrStdout(), "acted on %s\n", o.Repo)
		return nil
	}
	root.AddCommand(run)
	return root
}

func (f *fixture) run(t *testing.T, compat bool, args ...string) (int, string, string) {
	t.Helper()
	if f.fx == nil {
		f.fx = &effects{}
	}
	var out, errb bytes.Buffer
	code := Run(f.build, args, Options{
		IO: IO{Out: &out, Err: &errb}, Version: "v9.9.9-fixt", GoFlagCompat: compat,
	})
	return code, out.String(), errb.String()
}

func TestCLIHelpNoEffects(t *testing.T) {
	type form struct {
		args   []string
		compat bool
	}
	var forms []form
	for _, a := range clicontract.HelpForms("run") {
		forms = append(forms, form{a, false}, form{a, true})
	}
	for _, a := range clicontract.GoFlagHelpForms("run") {
		forms = append(forms, form{a, true})
	}
	if len(forms) == 0 {
		t.Fatal("no help forms selected")
	}
	for _, mc := range clicontract.MalformedConfigs {
		for _, fm := range forms {
			name := fmt.Sprintf("%s/%v/compat=%v", mc.Name, fm.args, fm.compat)
			t.Run(name, func(t *testing.T) {
				f := &fixture{cfgErr: errors.New("malformed: " + mc.Name), env: map[string]string{"FIXT_COUNT": "not-an-int"}}
				code, out, errs := f.run(t, fm.compat, fm.args...)
				if code != 0 {
					t.Fatalf("exit %d, want 0; stderr=%q", code, errs)
				}
				if f.fx.preRuns+f.fx.postRuns+f.fx.loads+f.fx.admissions+f.fx.acts != 0 {
					t.Fatalf("help reached effects: %+v", *f.fx)
				}
				// Generated from the tree: the usage line and a flag or subcommand it defines.
				if !strings.Contains(out, "Usage:") || !(strings.Contains(out, "--repo") || strings.Contains(out, "run")) {
					t.Fatalf("help not generated from the command tree:\n%s", out)
				}
			})
		}
	}
	versions := [][]string{}
	for _, a := range clicontract.VersionForms {
		versions = append(versions, a)
	}
	for _, a := range clicontract.GoFlagVersionForms {
		versions = append(versions, a)
	}
	for _, a := range versions {
		t.Run(fmt.Sprintf("version/%v", a), func(t *testing.T) {
			f := &fixture{cfgErr: errors.New("malformed")}
			code, out, _ := f.run(t, true, a...)
			if code != 0 || !strings.Contains(out, "v9.9.9-fixt") {
				t.Fatalf("exit %d out %q", code, out)
			}
			if f.fx.preRuns+f.fx.postRuns+f.fx.loads+f.fx.admissions+f.fx.acts != 0 {
				t.Fatalf("version reached effects: %+v", *f.fx)
			}
		})
	}
	t.Run("no-v-shorthand", func(t *testing.T) {
		f := &fixture{}
		if code, _, _ := f.run(t, false, "-v"); code != ExitUsage {
			t.Fatalf("-v exit %d, want %d: version owns no shorthand", code, ExitUsage)
		}
	})
	t.Run("no-completion-command", func(t *testing.T) {
		f := &fixture{}
		if code, _, _ := f.run(t, false, "completion", "bash"); code != ExitUsage {
			t.Fatalf("completion exit %d, want %d", code, ExitUsage)
		}
	})
	for _, traverse := range []bool{false, true} {
		for _, errorHooks := range []bool{false, true} {
			t.Run(fmt.Sprintf("persistent-hooks/traverse=%v/errors=%v", traverse, errorHooks), func(t *testing.T) {
				previous := cobra.EnableTraverseRunHooks
				cobra.EnableTraverseRunHooks = traverse
				t.Cleanup(func() { cobra.EnableTraverseRunHooks = previous })
				for _, args := range append(clicontract.HelpForms("run"), []string{"--version"}, []string{"run"}) {
					f := &fixture{fx: &effects{}}
					build := func() *cobra.Command {
						root := f.build()
						if !errorHooks {
							root.PersistentPreRunE, root.PersistentPostRunE = nil, nil
							root.PersistentPreRun = func(*cobra.Command, []string) { f.fx.preRuns++ }
							root.PersistentPostRun = func(*cobra.Command, []string) { f.fx.postRuns++ }
						}
						return root
					}
					if code := Run(build, args, Options{Version: "fixture"}); code != 0 {
						t.Fatalf("%v: exit %d", args, code)
					}
					want := 0
					if reflect.DeepEqual(args, []string{"run"}) {
						want = 1 // The fix must retain hooks for ordinary execution.
					}
					if f.fx.preRuns != want || f.fx.postRuns != want {
						t.Fatalf("%v: pre=%d post=%d, want %d each", args, f.fx.preRuns, f.fx.postRuns, want)
					}
				}
			})
		}
	}
}

func TestCLIConfigFlow(t *testing.T) {
	t.Run("string-array-never-splits", func(t *testing.T) {
		for _, source := range []Source{Env, Config} {
			t.Run(source.String(), func(t *testing.T) {
				set := Declare(NewRoot("array", ""), Binding{Key: "labels", Kind: StringArray, Env: []string{"LABELS"}, Config: true})
				in := Inputs{LookupEnv: func(string) (string, bool) { return "", false }}
				if source == Env {
					in.LookupEnv = func(string) (string, bool) { return "one,two three", true }
				} else {
					in.Config = map[string]any{"labels": "one,two three"}
				}
				v, err := set.Resolve(in)
				if err != nil {
					t.Fatal(err)
				}
				if got := v.Strings("labels"); !reflect.DeepEqual(got, []string{"one,two three"}) || v.Source("labels") != source {
					t.Fatalf("labels=%#v from %s, want one unsplit %s element", got, v.Source("labels"), source)
				}
			})
		}
	})
	t.Run("uncoded-handler-error", func(t *testing.T) {
		build := func() *cobra.Command {
			root := NewRoot("plain", "")
			root.RunE = func(*cobra.Command, []string) error { return errors.New("handler failed") }
			return root
		}
		var stderr bytes.Buffer
		if code := Run(build, nil, Options{IO: IO{Err: &stderr}}); code != 1 {
			t.Fatalf("uncoded handler error: exit %d, want 1", code)
		}
		if stderr.Len() != 0 {
			t.Fatalf("handler error was printed as a parse error: %q", stderr.String())
		}
	})
	t.Run("precedence", func(t *testing.T) {
		if len(clicontract.Precedence) == 0 {
			t.Fatal("empty precedence matrix")
		}
		for _, pc := range clicontract.Precedence {
			t.Run(pc.Name, func(t *testing.T) {
				f := &fixture{env: map[string]string{}, config: map[string]any{}}
				args := []string{"run"}
				if pc.Flag != nil {
					args = append(args, "--path="+*pc.Flag)
				}
				if pc.Env != nil {
					f.env["FIXT_PATH"] = *pc.Env
				}
				if pc.Config != nil {
					f.config["path"] = *pc.Config
				}
				code, _, errs := f.run(t, false, args...)
				if code != 0 {
					t.Fatalf("exit %d stderr %q", code, errs)
				}
				g := f.fx.got
				if g.Path != pc.Want || g.PathSrc.String() != pc.WantSource {
					t.Fatalf("path=%q from %s, want %q from %s", g.Path, g.PathSrc, pc.Want, pc.WantSource)
				}
			})
		}
	})
	t.Run("per-key-order", func(t *testing.T) {
		f := &fixture{env: map[string]string{"FIXT_CELL": "inherited"}, config: map[string]any{"cell": "from-cell-env"}}
		if code, _, e := f.run(t, false, "run"); code != 0 {
			t.Fatal(e)
		}
		if f.fx.got.Cell != "from-cell-env" || f.fx.got.CellSrc != Config {
			t.Fatalf("cell=%q from %s; config must outrank env for this key", f.fx.got.Cell, f.fx.got.CellSrc)
		}
		f = &fixture{env: map[string]string{"FIXT_CELL": "inherited"}, config: map[string]any{}}
		f.run(t, false, "run")
		if f.fx.got.Cell != "inherited" || f.fx.got.CellSrc != Env {
			t.Fatalf("cell=%q from %s, want inherited env", f.fx.got.Cell, f.fx.got.CellSrc)
		}
		f = &fixture{config: map[string]any{"cell": ""}, env: map[string]string{"FIXT_CELL": "inherited"}}
		f.run(t, false, "run")
		if f.fx.got.Cell != "" || f.fx.got.CellSrc != Config {
			t.Fatalf("explicit empty cell.env value must win: %q from %s", f.fx.got.Cell, f.fx.got.CellSrc)
		}
	})
	t.Run("unset-vs-empty", func(t *testing.T) {
		f := &fixture{env: map[string]string{}}
		f.run(t, false, "run")
		if f.fx.got.Token != "" || f.fx.got.Wait != 0 || f.fx.got.Labels != nil {
			t.Fatalf("unset keys leaked values: %+v", *f.fx.got)
		}
		f = &fixture{env: map[string]string{"FIXT_LABEL": ""}}
		f.run(t, false, "run")
		if !reflect.DeepEqual(f.fx.got.Labels, []string{""}) {
			t.Fatalf("explicit empty env list = %#v, want one empty element", f.fx.got.Labels)
		}
		f = &fixture{}
		f.run(t, false, "run", "--label=")
		if !reflect.DeepEqual(f.fx.got.Labels, []string{""}) {
			t.Fatalf("--label= = %#v, want one empty element", f.fx.got.Labels)
		}
	})
	t.Run("option-forms-and-paths", func(t *testing.T) {
		n := 0
		for _, form := range clicontract.OptionForms {
			for _, pv := range clicontract.PathValues {
				compat := form.GoFlagOnly
				t.Run(form.Name+"/"+pv.Name, func(t *testing.T) {
					f := &fixture{}
					args := append([]string{"run"}, form.Args("path", pv.Value)...)
					code, _, errs := f.run(t, compat, args...)
					if code != 0 {
						t.Fatalf("exit %d stderr %q", code, errs)
					}
					if f.fx.got.Path != pv.Value || f.fx.got.PathSrc != Flag {
						t.Fatalf("path=%q (%s), want %q byte-identical", f.fx.got.Path, f.fx.got.PathSrc, pv.Value)
					}
				})
				t.Run("env+config/"+pv.Name, func(t *testing.T) {
					f := &fixture{env: map[string]string{"FIXT_PATH": pv.Value}}
					f.run(t, false, "run")
					if f.fx.got.Path != pv.Value {
						t.Fatalf("env path=%q want %q", f.fx.got.Path, pv.Value)
					}
					f = &fixture{config: map[string]any{"path": pv.Value}}
					f.run(t, false, "run")
					if f.fx.got.Path != pv.Value {
						t.Fatalf("config path=%q want %q", f.fx.got.Path, pv.Value)
					}
				})
				n++
			}
		}
		if n == 0 {
			t.Fatal("no option-form cases selected")
		}
	})
	t.Run("typed-values", func(t *testing.T) {
		f := &fixture{env: map[string]string{"FIXT_COUNT": "7"}, config: map[string]any{"wait": "90s"}}
		code, out, errs := f.run(t, false, "run", "-n", "--label", "a", "--label=b,c", "--repo", "o/r", "tgt")
		if code != 0 {
			t.Fatalf("exit %d %q", code, errs)
		}
		g := f.fx.got
		want := opts{Repo: "o/r", RepoSrc: Flag, Path: clicontract.PrecedenceDefault, PathSrc: Default, Count: 7, Dry: true,
			Wait: 90 * time.Second, Labels: []string{"a", "b,c"}, Args: []string{"tgt"}}
		if !reflect.DeepEqual(*g, want) {
			t.Fatalf("got %+v\nwant %+v", *g, want)
		}
		if f.fx.admissions != 1 || f.fx.acts != 1 || !strings.Contains(out, "acted on o/r") {
			t.Fatalf("handler did not reach admission and effect: %+v out=%q", *f.fx, out)
		}
	})
	t.Run("secret-from-env-only", func(t *testing.T) {
		f := &fixture{env: map[string]string{"FIXT_TOKEN": "s3cr3t"}}
		f.run(t, false, "run")
		if f.fx.got.Token != "s3cr3t" {
			t.Fatalf("token not resolved from env")
		}
		if code, _, _ := (&fixture{}).run(t, false, "run", "--token=x"); code != ExitUsage {
			t.Fatalf("a secret must have no flag: exit %d", code)
		}
		defer func() {
			if recover() == nil {
				t.Fatal("Declare accepted a Secret flag")
			}
		}()
		Declare(&cobra.Command{Use: "x"}, Binding{Key: "token", Flag: "token", Secret: true})
	})
	t.Run("dash-and-double-dash", func(t *testing.T) {
		f := &fixture{}
		f.run(t, false, "run", "--path", "-x")
		if f.fx.got.Path != "-x" {
			t.Fatalf("dash-leading separate value = %q", f.fx.got.Path)
		}
		f = &fixture{}
		f.run(t, false, "run", "--", "--repo")
		if !reflect.DeepEqual(f.fx.got.Args, []string{"--repo"}) || f.fx.got.RepoSrc != Default {
			t.Fatalf("-- did not end flags: %+v", *f.fx.got)
		}
		f = &fixture{}
		f.run(t, true, "run", "--path", "-repo")
		if f.fx.got.Path != "-repo" {
			t.Fatalf("Go-flag compat rewrote a value: %q", f.fx.got.Path)
		}
		f = &fixture{}
		f.run(t, true, "run", "--", "-repo")
		if !reflect.DeepEqual(f.fx.got.Args, []string{"-repo"}) {
			t.Fatalf("Go-flag compat rewrote after --: %+v", f.fx.got.Args)
		}
	})
	t.Run("usage-errors", func(t *testing.T) {
		for _, args := range [][]string{
			{"run", "--nope"}, {"run", "--repo"}, {"run", "a", "b"}, {"nosuch"}, {"run", "--count=x"}, {"run", "-repo", "x"},
		} {
			f := &fixture{}
			code, _, errs := f.run(t, false, args...)
			if code != ExitUsage || f.fx.loads != 0 || !strings.Contains(errs, "fixt:") {
				t.Fatalf("%v: exit %d effects %+v stderr %q", args, code, *f.fx, errs)
			}
		}
	})
	t.Run("admission-keeps-its-code", func(t *testing.T) {
		f := &fixture{config: map[string]any{"repo": "noslash"}}
		code, _, errs := f.run(t, false, "run")
		if code != 3 || f.fx.admissions != 1 || f.fx.acts != 0 || !strings.Contains(errs, "refused") {
			t.Fatalf("exit %d effects %+v stderr %q", code, *f.fx, errs)
		}
	})
	t.Run("source-restriction-and-redaction", func(t *testing.T) {
		f := &fixture{config: map[string]any{"count": 5}}
		code, _, errs := f.run(t, false, "run")
		if code != 2 || f.fx.admissions != 0 || !strings.Contains(errs, `"count" cannot be set from config`) {
			t.Fatalf("config for an env/flag-only key: exit %d stderr %q", code, errs)
		}
		f = &fixture{env: map[string]string{"FIXT_COUNT": "zz-secret-value"}}
		code, _, errs = f.run(t, false, "run")
		if code != 2 || strings.Contains(errs, "zz-secret-value") || !strings.Contains(errs, `"count" from env`) {
			t.Fatalf("malformed env: exit %d stderr %q (must name key+source, never value)", code, errs)
		}
	})
	t.Run("env-case-by-platform", func(t *testing.T) {
		t.Setenv("FIXT_CASE_PROBE", "v")
		root := &cobra.Command{Use: "x"}
		set := Declare(root, Binding{Key: "probe", Kind: String, Env: []string{"fixt_case_probe"}})
		v, err := set.Resolve(Inputs{}) // nil LookupEnv: the real process environment
		if err != nil {
			t.Fatal(err)
		}
		if got, want := v.IsSet("probe"), clicontract.EnvFoldsCase(runtime.GOOS); got != want {
			t.Fatalf("GOOS=%s: lower-case name found=%v, want %v", runtime.GOOS, got, want)
		}
	})
}

func TestCLIReentrant(t *testing.T) {
	type step struct {
		args   []string
		env    map[string]string
		config map[string]any
		check  func(*opts) string
	}
	steps := []step{
		{args: []string{"run", "--repo=a/one", "--label", "x", "-n", "--count", "5"}, env: map[string]string{"FIXT_TOKEN": "t1"}, config: map[string]any{"wait": "1s", "cell": "c1"},
			check: func(o *opts) string {
				if o.Repo != "a/one" || o.Count != 5 || !o.Dry || o.Token != "t1" || o.Wait != time.Second || o.Cell != "c1" {
					return "first run lost its own inputs"
				}
				return ""
			}},
		{args: []string{"run"},
			check: func(o *opts) string {
				if o.RepoSrc != Default || o.Labels != nil || o.Dry || o.Count != 1 || o.Token != "" || o.Wait != 0 || o.CellSrc != Unset {
					return fmt.Sprintf("second run saw the first run's state: %+v", *o)
				}
				return ""
			}},
		{args: []string{"run", "--label=y"}, env: map[string]string{"FIXT_REPO": "b/two"},
			check: func(o *opts) string {
				if o.Repo != "b/two" || !reflect.DeepEqual(o.Labels, []string{"y"}) {
					return fmt.Sprintf("third run: %+v (labels must not accumulate)", *o)
				}
				return ""
			}},
	}
	for round := 0; round < 3; round++ {
		for i, s := range steps {
			f := &fixture{env: s.env, config: s.config}
			code, _, errs := f.run(t, round%2 == 1, s.args...)
			if code != 0 {
				t.Fatalf("round %d step %d: exit %d %q", round, i, code, errs)
			}
			if msg := s.check(f.fx.got); msg != "" {
				t.Fatalf("round %d step %d: %s", round, i, msg)
			}
		}
	}
	if keys := viper.AllKeys(); len(keys) != 0 {
		t.Fatalf("the global Viper was written: %v", keys)
	}
	if _, ok := os.LookupEnv("FIXT_REPO"); ok {
		t.Fatal("fixture environment leaked into the process")
	}
}

// TestCLIOpaqueArgv pins the OpaqueArgv boundary: every word past the declared positionals of a
// command that runs a command line reaches its handler untouched, whatever flag it spells, while
// the command's own flags before that word still parse.
func TestCLIOpaqueArgv(t *testing.T) {
	build := func(got *[]string, src *string) func() *cobra.Command {
		return func() *cobra.Command {
			root := NewRoot("tool", "fixture")
			root.PersistentFlags().String("root", "", "a persistent selector")
			grp := &cobra.Command{Use: "grp", Short: "a group"}
			run := &cobra.Command{Use: "run <a> [-- command [args]]", Args: cobra.ArbitraryArgs,
				RunE: func(cmd *cobra.Command, args []string) error {
					*got = args
					*src, _ = cmd.Flags().GetString("source")
					return nil
				}}
			run.Flags().String("source", "", "a value flag")
			run.Flags().BoolP("all", "a", false, "a bool flag")
			OpaqueArgv(run, 1)
			grp.AddCommand(run)
			root.AddCommand(grp)
			return root
		}
	}
	for _, tc := range []struct {
		args    []string
		src     string
		command []string
	}{
		{[]string{"grp", "run", "x", "--source", "s", "child", "--source", "t", "--all", "--help"}, "s", []string{"child", "--source", "t", "--all", "--help"}},
		{[]string{"--root", "/r", "grp", "--source=s", "run", "-a", "x", "child", "-source", "t", "-a", "--root", "/q"}, "s", []string{"child", "-source", "t", "-a", "--root", "/q"}},
		{[]string{"grp", "run", "-source", "s", "x", "--all", "child", "-h", "--version"}, "s", []string{"child", "-h", "--version"}},
		{[]string{"grp", "run", "x", "--", "child", "--source", "t"}, "", []string{"child", "--source", "t"}},
		{[]string{"grp", "run", "x", "--source", "s"}, "s", []string{}},
	} {
		var got []string
		var src string
		code := Run(build(&got, &src), tc.args, Options{GoFlagCompat: true, Version: "v0"})
		if code != 0 || src != tc.src {
			t.Errorf("%v: exit %d, --source %q; want exit 0, --source %q", tc.args, code, src, tc.src)
			continue
		}
		if len(got) < 1 || !reflect.DeepEqual(append([]string{}, got[1:]...), tc.command) {
			t.Errorf("%v: handler got %q; want positional x then the command %q untouched", tc.args, got, tc.command)
		}
	}
}

// TestCLIOpaqueArgvVerbResolvedLikeTheParser: a flag the walk does not know at the level it sits
// (here a flag of the opaque command, written before that command's name) makes the parser skip
// the next word as its value, while the walk would count that word as a positional and never
// reach the opaque command, so no "--" would be inserted and the command's own words would be
// parsed as the tool's flags. Wherever the walk and the parser disagree on the command, Run
// refuses with the usage exit and no handler runs.
func TestCLIOpaqueArgvVerbResolvedLikeTheParser(t *testing.T) {
	ran := false
	build := func() *cobra.Command {
		root := NewRoot("tool", "fixture")
		root.PersistentFlags().String("root", "", "a persistent selector")
		grp := &cobra.Command{Use: "grp", Short: "a group"}
		run := &cobra.Command{Use: "run <a> [-- command [args]]", Args: cobra.ArbitraryArgs,
			RunE: func(*cobra.Command, []string) error { ran = true; return nil }}
		run.Flags().String("source", "", "a value flag")
		OpaqueArgv(run, 1)
		grp.AddCommand(run)
		root.AddCommand(grp)
		return root
	}
	for _, args := range [][]string{
		{"--source", "s", "grp", "run", "x", "child", "--source", "t"},
		{"grp", "--source", "s", "run", "x", "child", "--help"},
		{"--root", "/r", "grp", "--source", "s", "run", "x", "child", "--root", "/q"},
	} {
		ran = false
		var errb strings.Builder
		code := Run(build, args, Options{IO: IO{Err: &errb}, GoFlagCompat: true, UsageExit: 3})
		if code != 3 || ran {
			t.Errorf("%v: exit %d, handler ran %v; want the usage exit 3 and no handler\n%s", args, code, ran, errb.String())
		}
	}
}

// TestCLIRefusesCompletionRequests pins that the hidden completion entrypoints Cobra adds on its
// own are refused like any unknown word: no handler of the tool runs (the shell-completion
// request would otherwise execute a handler's validation code, which may print or read state),
// and the refusal is the usage exit code with the unknown-command wording.
func TestCLIRefusesCompletionRequests(t *testing.T) {
	ran := false
	build := func() *cobra.Command {
		root := NewRoot("tool", "fixture")
		root.AddCommand(&cobra.Command{Use: "do <x>", Args: cobra.ExactArgs(1),
			ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
				ran = true
				return []string{"leak"}, cobra.ShellCompDirectiveNoFileComp
			},
			RunE: func(*cobra.Command, []string) error { ran = true; return nil }})
		return root
	}
	for _, args := range [][]string{
		{cobra.ShellCompRequestCmd, "do", ""},
		{cobra.ShellCompNoDescRequestCmd, "do", "x"},
		{cobra.ShellCompRequestCmd},
	} {
		ran = false
		var out, errb bytes.Buffer
		code := Run(build, args, Options{GoFlagCompat: true, Version: "v0", IO: IO{Out: &out, Err: &errb}})
		if code != ExitUsage || ran || out.Len() != 0 || !strings.Contains(errb.String(), "unknown command") {
			t.Errorf("%v: exit %d, handler ran %v, stdout %q, stderr %q; want the unknown-command refusal and no handler", args, code, ran, out.String(), errb.String())
		}
	}
}
