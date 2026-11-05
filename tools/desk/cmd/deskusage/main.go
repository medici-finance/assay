// Command deskusage stores only reported usage counts locally and prints aggregates.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/spf13/cobra"
)

func main() {
	args := os.Args[1:]
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	// Cobra selects execution before recording; opted-in help/version remain effect-free.
	record := func(operation func() int) int {
		return deskkit.PerfRunSelected("deskusage", []string{"record", "summary"}, args, selected, func([]string) int { return operation() })
	}
	os.Exit(runRecorded(args, os.Stdout, os.Stderr, record, func(c perfrec.Context) { selected = c }))
}

func run(args []string, out, errout io.Writer) int {
	return runRecorded(args, out, errout, func(operation func() int) int { return operation() }, nil)
}

func runRecorded(args []string, out, errout io.Writer, record func(func() int) int, selected func(perfrec.Context)) int {
	handled := false
	// All operational paths, including parse refusals, share guard and audit ownership.
	perform := func(operation func() error, code int, verb string) int {
		return record(func() int {
			err := deskkit.Guard()
			if err != nil {
				fmt.Fprintln(errout, "deskusage:", err)
				code = deskkit.ExitCodeOf(err)
				if code == deskkit.ExitDisabled {
					return code
				} // Guard wrote this audit entry.
			} else if operation != nil {
				err = operation()
				code = deskkit.ExitCodeOf(err)
				if err != nil {
					fmt.Fprintln(errout, "deskusage:", err)
				}
			}
			result := deskkit.ResultLocal
			if code == deskkit.ExitRefused {
				result = deskkit.ResultRefused
			} else if code != 0 {
				result = deskkit.ResultUnverifiable
			}
			if err := deskkit.Log(deskkit.Entry{Tool: "deskusage", Verb: verb, Result: result, Detail: "local usage counts"}); err != nil {
				fmt.Fprintln(errout, "deskusage: cannot audit local usage action")
				return deskkit.ExitUnverifiable
			}
			return code
		})
	}
	rc := cli.Run(func() *cobra.Command {
		root := cli.NewRoot("deskusage", "Local counts only; never reads transcripts, prompts, stdin or tool output")
		sha, built := deskkit.Version()
		root.Version = fmt.Sprintf("deskusage sourceSHA=%s builtAt=%s", sha, built)
		root.Args = cobra.NoArgs
		root.RunE = func(*cobra.Command, []string) error { return deskkit.Refused("choose record or summary") }
		children := []*cobra.Command{recordCommand(), summaryCommand()}
		root.AddCommand(children...)
		for _, cmd := range append(children, root) {
			handler := cmd.RunE
			verb := cmd.Name()
			if cmd == root {
				verb = "other"
			}
			cmd.RunE = func(cmd *cobra.Command, argv []string) error {
				handled = true
				code := perform(func() error { return handler(cmd, argv) }, 0, verb)
				if code != 0 {
					return &deskkit.DeskError{Code: code}
				}
				return nil
			}
		}
		return root
	}, args, cli.Options{Performance: selected, IO: cli.IO{Out: out, Err: errout}, UsageExit: deskkit.ExitRefused, ExitCode: deskkit.ExitCodeOf})
	if !handled && rc != 0 {
		return perform(nil, rc, "other")
	}
	return rc
}
