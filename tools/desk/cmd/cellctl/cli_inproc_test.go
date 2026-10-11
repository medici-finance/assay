package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// inproc runs one invocation against a fresh command tree in this process and returns its exit
// code. Handlers refuse by panicking an exit code, which runTree recovers exactly as main does.
func inproc(args ...string) int { return runTree(args) }

// runTreeWith is inproc with a hook that may rewrite the freshly built tree (swap a handler for a
// probe), so a test can observe what the parser handed over without running the handler.
func runTreeWith(t *testing.T, args []string, mutate func(root *cobra.Command)) (code int) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			ec, ok := r.(exitCode)
			if !ok {
				panic(r)
			}
			code = ec.code
		}
	}()
	return cli.Run(func() *cobra.Command {
		root := buildRoot()
		mutate(root)
		return root
	}, args, cli.Options{UsageExit: 3, GoFlagCompat: true})
}

// newArgv runs `cellctl new <args...>` in this process and, like the handler it replaced, panics
// the exit code of a refusal to the caller (whose recover reads it as a refusal).
func newArgv(args []string) {
	if code := inproc(append([]string{"new"}, args...)...); code != 0 {
		panic(exitCode{code})
	}
}

// treeHelpText is every help text the command tree carries: the root's, then each command's long
// text and flag usages, hidden commands included. It is what the pre-migration usage text was.
func treeHelpText() string {
	var b strings.Builder
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		b.WriteString(c.Long + "\n" + c.Example + "\n" + c.Flags().FlagUsages() + "\n")
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(buildRoot())
	return b.String()
}

// argv runs `cellctl <verb> <cell> <args...>` in this process and panics a refusal's exit code to
// the caller, as the pre-migration handlers that took raw words did.
func argv(verb, cell string, args ...string) {
	if code := inproc(append([]string{verb, cell}, args...)...); code != 0 {
		panic(exitCode{code})
	}
}

func setArgv(cell string, args []string) { argv("set", cell, args...) }
func showArgv(cell string)               { argv("show", cell) }
