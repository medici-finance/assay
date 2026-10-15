//go:build cellopfixture

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// The cell-operation FIXTURE (desk-containers/15). It is compiled only with
// `-tags cellopfixture`, so the end-to-end rows run before any real verb adopts the contract, and
// a release build reports `cellop-fixture` as an unknown command (cellop_fixture_off.go).
//
// It registers:
//   - `cellop-fixture <cell> [--plan [--json] | --apply <plan-id>]`: bare, it reads; --plan writes
//     a plan; --apply (annotated) applies one;
//   - a STAND-IN for each verb of the enumerated state-changing list whose verb brief has not
//     landed (drain, pause, resume, restart, doctor, gc, upgrade), annotated exactly as the
//     contract says. A real verb, once registered, replaces its stand-in, and the flag-walk test
//     then checks the real verb's annotations instead;
//   - --plan, --json and an annotated --apply on the real `up`, until brief 17 lands its own.
//
// The fixture's one precondition is the content of <cell>/run/cellop-fixture/state ("absent" when
// there is none); its one effect appends a line to <cell>/run/cellop-fixture/effects. With
// CELLOP_FIXTURE_TRACE set, every arrival at the apply path appends the command path and the flags
// given to that file: the behavioural arm of TestEveryStateChangingInvocationRequiresTTY.

const cellopFixtureBuild = true

// cellopStandIn is one verb of the enumerated list: whether every form is state-changing, and
// its variant flags (booleans, each annotated, each choosing a variant of the verb's action).
type cellopStandIn struct {
	verb     string
	command  bool
	variants []string
}

var cellopStandIns = []cellopStandIn{
	{verb: "drain", command: true, variants: []string{"now"}},
	{verb: "pause", command: true},
	{verb: "resume", command: true, variants: []string{"foreign"}},
	{verb: "restart", command: true, variants: []string{"now"}},
	{verb: "doctor"},
	{verb: "gc", variants: []string{"run"}},
	{verb: "upgrade"},
}

func cellopFixtureCommands(root *cobra.Command) {
	root.AddCommand(cellopFixtureCmd("cellop-fixture", "the cell-operation fixture verb (cellopfixture build only)", false, nil))
	for _, s := range cellopStandIns {
		if verbNamed(root, s.verb) != nil {
			continue // the real verb replaces its stand-in
		}
		root.AddCommand(cellopFixtureCmd(s.verb, "fixture stand-in for "+s.verb+" (cellopfixture build only)", s.command, s.variants))
	}
	if up := verbNamed(root, "up"); up != nil && up.Flags().Lookup("apply") == nil {
		declare(up, bBool("plan", "write a reconcile plan and change nothing else (fixture)"),
			bBool("json", "print the plan or result as cell-op-v1 JSON (fixture)"),
			bStr("apply", "apply a stored reconcile plan on this terminal (fixture)"))
		markStateChangingFlag(up, "apply")
		real := up.RunE
		h := cellopFixtureRun("up", nil)
		up.RunE = func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("plan") || cmd.Flags().Changed("apply") {
				return h(cmd, args)
			}
			return real(cmd, args)
		}
	}
}

func cellopFixtureCmd(verb, short string, command bool, variants []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:         verb + " <cell> [--plan [--json] | --apply <plan-id>]",
		Short:       short,
		Annotations: map[string]string{leadingArgsKey: "1"},
		Args:        cobra.ExactArgs(1),
	}
	bs := []cli.Binding{
		bBool("plan", "write a plan and change nothing else; works off a terminal"),
		bBool("json", "print the plan or the result as cell-op-v1 JSON"),
		bStr("apply", "apply the stored plan with this id, on this terminal, after the typed confirm"),
	}
	for _, v := range variants {
		bs = append(bs, bBool(v, "the --"+v+" variant of "+verb))
	}
	declare(cmd, bs...)
	if command {
		// Every form except --plan is state-changing: the command annotation covers each flag.
		markStateChanging(cmd)
	} else {
		markStateChangingFlag(cmd, "apply")
		for _, v := range variants {
			markStateChangingFlag(cmd, v)
		}
	}
	cmd.RunE = cellopFixtureRun(verb, variants)
	return cmd
}

func cellopFixtureRun(verb string, variants []string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		c := loadCell(cellArg(args))
		var chosen []string
		for _, v := range variants {
			if cmd.Flags().Changed(v) {
				chosen = append(chosen, v)
			}
		}
		plan, apply := cmd.Flags().Changed("plan"), cmd.Flags().Changed("apply")
		fv := cellopVerb{Verb: verb, Measure: cellopFixtureMeasure, Run: cellopFixtureEffect}
		switch {
		case plan && apply:
			die("%s: --plan and --apply are exclusive", verb)
		case plan:
			cellopPlan(cmd, c, verb, cellopFixtureActions(c, verb, chosen))
		case apply:
			cellopApply(cmd, c, fv, cmd.Flags().Lookup("apply").Value.String())
		case commandStateChanging(cmd) || len(chosen) > 0:
			p := cellopNewPlan(c, verb, cellopFixtureActions(c, verb, chosen))
			cellopApply(cmd, c, fv, p.ID)
		default:
			// Bare doctor, gc, upgrade and cellop-fixture never apply and never write: they say
			// what they would do.
			fmt.Printf("%s %s: would %s (state %q); read-only, no plan written\n", verb, c.Name, verb, cellopFixtureState(c))
		}
		return nil
	}
}

func cellopFixtureDir(c *Cell) string { return filepath.Join(c.Dir, "run", "cellop-fixture") }

func cellopFixtureState(c *Cell) string {
	raw, err := os.ReadFile(filepath.Join(cellopFixtureDir(c), "state"))
	if errors.Is(err, fs.ErrNotExist) {
		return "absent"
	}
	if err != nil {
		die("cellop-fixture: %v", err)
	}
	return strings.TrimSpace(string(raw))
}

func cellopFixtureActions(c *Cell, verb string, variants []string) []cellop.Action {
	id := strings.Join(append([]string{verb}, variants...), "-")
	return []cellop.Action{{
		ID: id, Cell: c.Name, Summary: "fixture: " + id + " on " + c.Name,
		Preconditions: []cellop.Precondition{{Key: "fixture/state", Value: cellopFixtureState(c)}},
	}}
}

func cellopFixtureMeasure(c *Cell, a cellop.Action) ([]cellop.Precondition, error) {
	return []cellop.Precondition{{Key: "fixture/state", Value: cellopFixtureState(c)}}, nil
}

func cellopFixtureEffect(c *Cell, a cellop.Action) (string, error) {
	if err := os.MkdirAll(cellopFixtureDir(c), 0o700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(filepath.Join(cellopFixtureDir(c), "effects"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, a.ID); err != nil {
		return "", err
	}
	return "fixture effect recorded", nil
}

// cellopFixtureTrace appends "<command path>\t<flags given, comma-separated>" to the file named by
// CELLOP_FIXTURE_TRACE each time an invocation reaches the apply path.
func cellopFixtureTrace(cmd *cobra.Command) {
	path := os.Getenv("CELLOP_FIXTURE_TRACE")
	if path == "" {
		return
	}
	var names []string
	cmd.Flags().Visit(func(f *pflag.Flag) { names = append(names, f.Name) })
	sort.Strings(names)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		die("cellop-fixture: trace: %v", err)
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\t%s\n", cmd.CommandPath(), strings.Join(names, ","))
}
