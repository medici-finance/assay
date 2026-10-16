package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// The cell-operation adapter mechanics (contract spec/cell-op-v1.md, how-to
// docs/cellctl-machine-seam.md). This file is what a verb brief adopts: it annotates its
// state-changing invocations, emits cellop results with --json, writes plans with --plan, and
// applies them only through cellopApply. No verb in the default build adopts it yet; the fixture
// verb and the stand-ins in cellop_fixture.go (build tag cellopfixture) exercise it end to end.

// stateChanging is the cobra annotation every state-changing invocation registers: on the command
// (every form of it except --plan changes state) or on the flag (only the form with that flag
// does). The root's pre-run gate refuses an annotated invocation off a terminal and in a desk
// session, and cellopApply refuses to run for one that is not annotated.
const stateChanging = "cellop:state-changing"

// cellopHonestLimit is appended to the help of every state-changing verb: the spec's stated limit.
const cellopHonestLimit = "Applying a plan is an interactive confirmation at the operator's own terminal: stdin and stdout must\n" +
	"be terminals, the plan and its confirm code are shown, and the code must be typed. Every precondition is then\n" +
	"re-measured and the plan is consumed once before anything runs. This confirm guards against mistakes and races;\n" +
	"it is NOT an authorization control. A process running under the operator's account can drive a terminal and\n" +
	"can delete the consumed marker, which allows another apply of the same plan before it expires (still subject\n" +
	"to the re-measure). Desk sessions (ASSAY_CELL, DESK_LOOP or DESK_SESSION set) are refused as a mistake guard\n" +
	"only. There is no flag that skips the confirm. See spec/cell-op-v1.md."

// markStateChanging annotates every form of cmd except --plan as state-changing.
func markStateChanging(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[stateChanging] = "1"
	appendHonestLimit(cmd)
}

// markStateChangingFlag annotates the form of cmd that sets flag name as state-changing.
func markStateChangingFlag(cmd *cobra.Command, name string) {
	if err := cmd.Flags().SetAnnotation(name, stateChanging, []string{"1"}); err != nil {
		panic(fmt.Sprintf("cellop: annotating --%s of %s: %v", name, cmd.Name(), err))
	}
	appendHonestLimit(cmd)
}

func appendHonestLimit(cmd *cobra.Command) {
	if strings.Contains(cmd.Long, cellopHonestLimit) {
		return
	}
	long := cmd.Long
	if long == "" {
		long = cmd.Short
	}
	cmd.Long = strings.TrimRight(long, "\n") + "\n\n" + cellopHonestLimit
}

func commandStateChanging(cmd *cobra.Command) bool { return cmd.Annotations[stateChanging] == "1" }

func flagStateChanging(f *pflag.Flag) bool { return len(f.Annotations[stateChanging]) > 0 }

// stateChangingInvocation reports whether this parsed invocation can change state: the command is
// annotated, or a flag given on the line is. A --plan line never is; the verb writes a plan and
// changes nothing else, and cellopApply refuses to run for it.
func stateChangingInvocation(cmd *cobra.Command) bool {
	if f := cmd.Flags().Lookup("plan"); f != nil && f.Changed {
		return false
	}
	if commandStateChanging(cmd) {
		return true
	}
	found := false
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if flagStateChanging(f) {
			found = true
		}
	})
	return found
}

// inheritedDeskMarkers are the desk-session markers this process started with. They are read at
// package initialisation because main removes an inherited ASSAY_CELL (dropInheritedCellMarker)
// before the command tree runs.
var inheritedDeskMarkers = cellop.SetMarkers(os.LookupEnv)

// deskMarkers is every desk-session marker set at start or now.
func deskMarkers() []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range append(append([]string{}, inheritedDeskMarkers...), cellop.SetMarkers(os.LookupEnv)...) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// interactive is the contract's terminal test: stdin AND stdout are terminals.
func interactive() bool { return isTTY(os.Stdin) && isTTY(os.Stdout) }

// gateStateChanging is the root pre-run gate: an annotated invocation is refused not-interactive
// off a terminal and desk-session when a marker is set, before the verb loads anything. It runs
// for every command, the no-echo ones included. cellop.Session.Apply makes both checks again.
func gateStateChanging(cmd *cobra.Command, args []string) {
	if !stateChangingInvocation(cmd) {
		return
	}
	reason, detail := "", ""
	switch {
	case !interactive():
		reason = cellop.ReasonNotInteractive
		detail = "this invocation changes state and needs a terminal on stdin and stdout; the --plan form works anywhere"
	case len(deskMarkers()) > 0:
		reason = cellop.ReasonDeskSession
		detail = "refused in a desk session (" + strings.Join(deskMarkers(), ", ") + " set); a mistake guard, not a control: clear them in an operator shell"
	default:
		return
	}
	cell := ""
	if len(args) > 0 {
		cell = args[0]
	}
	now := time.Now()
	r := cellop.NewResult(cmd.Name(), cell, now)
	r.Finish(cellop.Refused, reason, detail, now)
	if cell != "" && cmd.Flags().Lookup("plan") != nil {
		r.Next = []string{"cellctl " + cmd.Name() + " " + cell + " --plan"}
	}
	emitResult(cmd, r)
	exitWith(r.ExitCode())
}

// jsonOutput reports whether the line asked for the machine form.
func jsonOutput(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("json")
	return f != nil && f.Changed && f.Value.String() == "true"
}

// emitResult prints a result: the cell-op-result document on stdout with --json, else one human
// line (stdout when ok, stderr otherwise) and one line per action.
func emitResult(cmd *cobra.Command, r cellop.Result) {
	if jsonOutput(cmd) {
		raw, err := r.JSON()
		if err != nil {
			die("%s: encoding the result: %v", r.Verb, err)
		}
		fmt.Fprintln(os.Stdout, string(raw))
		return
	}
	out := os.Stdout
	if r.Outcome != cellop.OK {
		out = os.Stderr
	}
	line := fmt.Sprintf("cellctl: %s %s: %s", r.Verb, r.Cell, r.Outcome)
	if r.Reason != nil {
		line += " (" + *r.Reason + ")"
	}
	if r.Detail != nil {
		line += ": " + *r.Detail
	}
	fmt.Fprintln(out, line)
	for _, a := range r.Actions {
		d := ""
		if a.Detail != nil {
			d = ": " + *a.Detail
		}
		fmt.Fprintf(out, "  [%s] %s %s%s\n", a.Cell, a.ID, a.Outcome, d)
	}
	for _, n := range r.Next {
		fmt.Fprintf(out, "  next: %s\n", n)
	}
}

// cellopVerb is what a verb hands the adapter: its name, and how it measures and runs an action.
type cellopVerb struct {
	Verb    string
	Measure func(c *Cell, a cellop.Action) ([]cellop.Precondition, error)
	Run     func(c *Cell, a cellop.Action) (string, error)
}

// cellopNewPlan makes and stores a plan for c. It changes nothing but the plan store.
func cellopNewPlan(c *Cell, verb string, actions []cellop.Action) cellop.Plan {
	id, err := cellop.NewID()
	if err != nil {
		die("%s: %v", verb, err)
	}
	p, err := cellop.NewPlan(id, verb, []string{c.Name}, actions, nil, time.Now())
	if err != nil {
		die("%s: %v", verb, err)
	}
	if _, err := cellop.StoreFor(c.Dir).Save(p); err != nil {
		die("%s: storing the plan: %v", verb, err)
	}
	return p
}

// cellopPlan is `<verb> --plan [--json]`: it stores a plan for c and prints the plan document
// with --json, else its rendering. It changes nothing but the plan store.
func cellopPlan(cmd *cobra.Command, c *Cell, verb string, actions []cellop.Action) cellop.Plan {
	p := cellopNewPlan(c, verb, actions)
	if jsonOutput(cmd) {
		raw, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			die("%s: %v", p.Verb, err)
		}
		fmt.Fprintln(os.Stdout, string(raw))
	} else {
		fmt.Fprint(os.Stdout, cellop.Render(p, nil))
		fmt.Fprintf(os.Stdout, "apply on a terminal: cellctl %s %s --apply %s\n", p.Verb, c.Name, p.ID)
	}
	return p
}

// cellopApply is the only path from cellctl to an apply. It refuses unless the invocation is
// annotated state-changing (the gate already ran for it), then hands the stored plan to
// cellop.Session.Apply on this terminal and exits with the result's code.
func cellopApply(cmd *cobra.Command, c *Cell, v cellopVerb, id string) {
	cellopFixtureTrace(cmd)
	if !stateChangingInvocation(cmd) {
		die("%s: an apply was reached from an invocation not annotated %s", cmd.Name(), stateChanging)
	}
	sess := cellop.Session{
		Store:       cellop.StoreFor(c.Dir),
		Cell:        c.Name,
		Verb:        v.Verb,
		Interactive: interactive(),
		DeskMarkers: deskMarkers(),
		Confirm: func(p cellop.Plan, children []cellop.Plan) (string, error) {
			return cellop.ReadConfirm(os.Stdin, os.Stderr, cellop.Render(p, children))
		},
		Measure: func(a cellop.Action) ([]cellop.Precondition, error) { return v.Measure(c, a) },
		Run:     func(a cellop.Action) (string, error) { return v.Run(c, a) },
		Now:     time.Now,
	}
	r := sess.Apply(id)
	emitResult(cmd, r)
	exitWith(r.ExitCode())
}
