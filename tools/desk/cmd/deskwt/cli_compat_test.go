package main

import (
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Compatibility entrypoints for direct-domain regression tests. Each enters the same Cobra
// tree; none keeps a Go FlagSet or a second parser. They skip activation and the kill switch
// (guardedExecute), as the pre-port per-verb entrypoints did, so they live in a test file: a
// production caller cannot reach a handler this way, and must go through parseWT with
// guardedExecute as main does.
func cmdVerb(argv []string) error {
	var result error
	env := wtCLI{handle: func(c wtCall) error { result = execute(c); return result }}
	code := parseWT(argv, env, cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
	if result != nil {
		return result
	}
	if code != 0 {
		return &deskkit.DeskError{Code: code, Msg: "deskwt: invalid command line"}
	}
	return nil
}

func cmdAdd(args []string) error       { return cmdVerb(append([]string{"add"}, args...)) }
func cmdRemove(args []string) error    { return cmdVerb(append([]string{"remove"}, args...)) }
func cmdPrune(args []string) error     { return cmdVerb(append([]string{"prune"}, args...)) }
func cmdMaintain(args []string) error  { return cmdVerb(append([]string{"maintain"}, args...)) }
func cmdRoleInit(args []string) error  { return cmdVerb(append([]string{"role-init"}, args...)) }
func cmdRoleClean(args []string) error { return cmdVerb(append([]string{"role-clean"}, args...)) }
func cmdObjectMaintenance(args []string) error {
	return cmdVerb(append([]string{"maintain", "objects"}, args...))
}
