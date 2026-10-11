package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// legacyShape refuses, before the tree parses anything, a command line whose SHAPE the
// pre-migration parser did not accept and the recorded decision (DR-cellctl-cobra) does not
// name. That shape is:
//
//	[--cells-root <abs>] <verb> <fixed positionals...> [flags and further words]
//
// The selector is the separated `--cells-root <abs>` and only as the first word. A verb's fixed
// positionals (its leadingArgsKey count: the cell, a role, an action) come before any flag. A
// single-dash long flag is the Go flag package's spelling, which only `scratch` accepted. The
// verb word itself is a verb, `-h`/`--help`, or a sole `--version`/`version`; anything else in
// that place, a lone `-` included, was an unknown verb, and is refused in the parser's words
// (decision entry 2).
//
// The internal raw entrypoints take their argv verbatim and are not checked past the selector.
// The flag-less verbs (check, deskd) read their words by position as the legacy parser did, so
// only the cell's place is checked: past it every word is theirs.
// A refusal is a usage error: exit 3, nothing on stdout, no roster echo, nothing run.
func legacyShape(root *cobra.Command, args []string) error {
	i := verbIndex(args)
	if i >= len(args) {
		return nil // no verb: the selector's own refusals (missing value, missing command) apply
	}
	rest := args[i:]
	verb := rest[0]
	if strings.HasPrefix(verb, "-") {
		switch {
		case verb == "-h" || verb == "--help":
			return nil
		case verb == "--version" && len(rest) == 1:
			return nil
		}
		return fmt.Errorf("unknown command %q for %q", verb, root.Name())
	}
	if verb == "help" {
		switch len(rest) {
		case 1:
			return nil
		case 2:
			if c := verbNamed(root, rest[1]); c != nil && !c.Hidden {
				return nil
			}
		}
		return fmt.Errorf("unknown help topic %q", strings.Join(rest[1:], " "))
	}
	if verb == "new" {
		// The legacy parser read new's help word on its whole line before anything else (a bare
		// `--` and a value position included), so a help line is new's usage (wholeLineScans)
		// whatever shape the other words have, a --cells-root after the verb included.
		for _, w := range rest[1:] {
			if w == "-h" || w == "--help" {
				return nil
			}
		}
	}
	cmd := verbNamed(root, verb)
	if cmd == nil || rawCommand(cmd) {
		return nil // an unknown verb is the parser's refusal; a raw verb's argv is its own
	}
	flagless := cmd.DisableFlagParsing
	if verb == "version" && len(rest) > 1 && !(len(rest) == 2 && (rest[1] == "-h" || rest[1] == "--help")) {
		return fmt.Errorf("unknown command %q for %q", rest[1], root.Name()+" version")
	}
	n, err := strconv.Atoi(cmd.Annotations[leadingArgsKey])
	if err != nil {
		return fmt.Errorf("%s: no declared positional count", verb) // the class guard's case
	}
	_, opaque := cli.OpaqueAfter(cmd)
	pos := 0
	for i := 1; i < len(rest); i++ {
		w := rest[i]
		if w == "--" {
			return nil
		}
		if flagless && pos >= n {
			return nil // a flag-less verb's words past its cell are read by position
		}
		if !flagShaped(w) {
			if opaque && pos >= n {
				return nil // where the supervised command starts
			}
			pos++
			continue
		}
		if w == "-h" || w == "--help" {
			continue
		}
		if pos < n {
			return fmt.Errorf("%s: flag %q comes before the command's positional arguments; flags follow them", verb, w)
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(w, "-"), "=")
		if name == "cells-root" {
			return fmt.Errorf("%s: --cells-root selects the registry only as the first word, before the command", verb)
		}
		if !strings.HasPrefix(w, "--") && len(w) > 2 && verb != "scratch" {
			return fmt.Errorf("%s: single-dash flag %q (only scratch takes that spelling; write -%s)", verb, w, w)
		}
		if f := lookupFlag(cmd, name); f != nil && f.NoOptDefVal == "" && !hasValue {
			i++ // the flag's separate value
		}
	}
	return nil
}

func flagShaped(w string) bool { return len(w) > 1 && w[0] == '-' }

// rawCommand reports an internal entrypoint: no flags parsed and no declared positionals.
func rawCommand(cmd *cobra.Command) bool {
	return cmd.DisableFlagParsing && cmd.Annotations[leadingArgsKey] == ""
}

func verbNamed(root *cobra.Command, name string) *cobra.Command {
	for _, c := range root.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func lookupFlag(cmd *cobra.Command, name string) *pflag.Flag {
	if f := cmd.Flags().Lookup(name); f != nil {
		return f
	}
	return cmd.InheritedFlags().Lookup(name)
}

// verbIndex is the index of the verb word in args: past the one accepted selector when a verb
// follows it, else 0. It is len(args) when there is no word there.
func verbIndex(args []string) int {
	if len(args) >= 3 && args[0] == "--cells-root" {
		return 2
	}
	if len(args) > 0 && args[0] == "--cells-root" {
		return len(args)
	}
	return 0
}

// scratchLine reports whether args (after a leading selector) run the scratch verb, the one verb
// whose flags the legacy parser read with the Go flag package (single-dash long forms included).
func scratchLine(args []string) bool {
	i := verbIndex(args)
	return i < len(args) && args[i] == "scratch"
}
