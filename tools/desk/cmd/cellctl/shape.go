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
// pre-migration parser did not accept and the recorded decision (docs/streams/decisions/
// DR-cellctl-cobra.md) does not name. That shape is:
//
//	[--cells-root <abs>] <verb> <fixed positionals...> [flags and further words]
//
// The selector is the separated `--cells-root <abs>` and only as the first word. A verb's fixed
// positionals (its leadingArgsKey count: the cell, a role, an action) come before any flag. A
// single-dash long flag is the Go flag package's spelling, which only `scratch` accepted. The
// verb word itself is a verb, `-h`/`--help`, or a sole `--version`/`version`; anything else in
// that place was an unknown verb, and is refused in the parser's words (decision entry 2).
//
// The internal raw entrypoints take their argv verbatim and are not checked past the selector.
// A refusal is a usage error: exit 3, nothing on stdout, no roster echo, nothing run.
func legacyShape(root *cobra.Command, args []string) error {
	rest := args
	if len(args) > 0 && args[0] == "--cells-root" {
		if len(args) < 3 {
			return nil // the selector's own refusals (missing value, missing command) apply
		}
		rest = args[2:]
	}
	if len(rest) == 0 {
		return nil
	}
	verb := rest[0]
	if strings.HasPrefix(verb, "-") && verb != "-" {
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
	cmd := verbNamed(root, verb)
	if cmd == nil || cmd.DisableFlagParsing {
		return nil // an unknown verb is the parser's refusal; a raw verb's argv is its own
	}
	if verb == "version" && len(rest) > 1 {
		return fmt.Errorf("unknown command %q for %q", strings.Join(rest, " "), root.Name())
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
			return fmt.Errorf("%s: flag %q stands before the command's positional arguments (%s); flags follow them", verb, w, cmd.Use)
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

// scratchLine reports whether args (after a leading selector) run the scratch verb, the one verb
// whose flags the legacy parser read with the Go flag package (single-dash long forms included).
func scratchLine(args []string) bool {
	a := args
	if len(a) > 0 && a[0] == "--cells-root" {
		if len(a) < 3 {
			return false
		}
		a = a[2:]
	}
	return len(a) > 0 && a[0] == "scratch"
}
