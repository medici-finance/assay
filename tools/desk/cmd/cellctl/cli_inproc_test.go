package main

import (
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
