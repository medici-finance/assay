// Command desktick is the ONE executable form of the tick summary-line grammar, as a Go verb the
// release ships — the port of plugins/assay/scripts/tick-summary.sh, which stays in the plugin tree
// as the parity oracle until example-stream/14 retires it.
//
//	desktick regexp                print the published extended regular expression
//	desktick validate '<line>'     exit 0 iff that one line satisfies the grammar
//	desktick check [< input]       exit 0 iff the LAST non-blank line of stdin satisfies it
//	desktick --help | --version
//
// Exit codes: 0 the line satisfies the grammar · 1 it does not (the reason is on stderr) · 2 usage.
//
// No network, no credential, no state — and no roster: this verb reads no configured control
// surface, so it declares no tool class and echoes no config.
package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `desktick — the tick summary-line grammar (references/tick-contract.md).

A desk role invoked in tick mode prints exactly one machine-readable line, as the
LAST line of its output:

  tick role=<role> outcome=<outcome> swept=<n> acted=<n> filed=<n> duration=<s>

  ok               swept numeric, acted numeric and >= 1
  noop             swept numeric, acted == 0
  refused          swept == 0, acted == 0
  could-not-check  swept == -   (the sweep did not complete: its count is unknown)

A count that is genuinely unknown is written -, never 0.

usage:
  desktick regexp                print the published extended regular expression
  desktick validate '<line>'     exit 0 iff that one line satisfies the grammar
  desktick check [< input]       exit 0 iff the LAST line of stdin satisfies it
  desktick --help | --version

exit codes: 0 satisfies · 1 does not (reason on stderr) · 2 usage error
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "regexp":
		fmt.Fprintln(stdout, Grammar)
		return 0
	case "validate":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "desktick: validate takes exactly one argument, the candidate line")
			return 2
		}
		return report(stderr, Validate(args[1]))
	case "check":
		b, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "desktick: cannot read the pass output: %v\n", err)
			return 1
		}
		return report(stderr, Check(string(b)))
	case "--version", "version":
		fmt.Fprintf(stdout, "desktick %s\n", grammarVersion)
		return 0
	case "--help", "-h", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "desktick: unknown verb %s\n", args[0])
		return 2
	}
}

func report(stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "desktick: %s\n", err.Error())
	if ve, ok := err.(*ValidationError); ok {
		for _, d := range ve.Detail {
			fmt.Fprintln(stderr, d)
		}
	}
	return 1
}
