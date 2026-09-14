package deskkit

// helprequest.go — the one place the suite decides that an invocation is a request for
// the HELP SCREEN rather than a request to do something.
//
// WHY THIS EXISTS. A help screen is not a refusal, and until this file it was recorded as
// one. Every verb's subcommand builds a `flag.FlagSet` with `flag.ContinueOnError` and turns
// ANY parse error into `Refused("bad flags: " + err)`. Go's flag package reports `-h` /
// `--help` as the sentinel error `flag.ErrHelp`, so `deskpr create --help` produced
// `refused: bad flags: flag: help requested`, exit 5, and — because the verb's `auditCtx`
// arms its `defer finalize(err)` BEFORE `fs.Parse` runs — one appended row in a ledger that
// `ratelimit.go` counts for the write budget and the circuit breaker and that `audit.go`
// never rotates. Measured on one operating desk host over 32 days: 1,043 such rows.
//
// The TOP-LEVEL form already proves none of that is necessary: `cmd/deskpr/main.go`'s `run`
// handles a bare `-h`/`--help`/`help` before `Guard()`, prints usage, returns `ExitOK` and
// writes nothing. This file generalises that to the subcommand form, in two tiers, because
// the two tiers answer different questions and neither alone is sufficient.
//
// TIER ONE — HelpOnly, before the kill-switch gate. It matches ONE shape and only one: the
// arguments that follow the leading subcommand token are EXACTLY one token, and that token
// is exactly `-h`, `-help` or `--help`. That restriction is the whole safety argument. A
// scanner that merely looked for `--help` anywhere in argv would match it as another flag's
// VALUE — `deskpr create --title --help` names a PR whose title is the string `--help` — and
// silently turn a real invocation into a help screen. A LONE token cannot be a value, because
// there is no preceding flag left for it to belong to, so this form has no false positive to
// reason about. Everything wider is left to tier two, where the flag package's own grammar
// decides.
//
// TIER TWO — ErrHelpRequested / IsHelpRequest, at the parse. Any other spelling reaches
// `fs.Parse`, which returns `flag.ErrHelp`; the verb recognises it, prints its subcommand
// usage, returns `ExitOK`, and its audit finalizer writes NO row. Tier two necessarily runs
// AFTER `Guard()`, and that is deliberate rather than a compromise: a kill-switched session
// still records its own `disabled` row, which is the one row about a help screen worth
// having.
//
// WHAT THIS IS NOT. Neither tier is a way past a gate. Tier one performs no work: it opens
// nothing, reads no configuration, mints no token and consults no roster — it returns a bool
// so the caller can print the usage string already compiled into the binary. A verb that
// wanted to do anything else on the help path would not use this.

import (
	"errors"
	"flag"
)

// helpTokens is the closed set of spellings tier one recognises. It is deliberately NOT
// extended with `help`, `-?`, `/?` or any other alias: `help` as a bare subcommand is
// already handled by each verb's own top-level dispatch (where it is unambiguous), and
// widening this set widens the surface on which a token could be mistaken for a value.
var helpTokens = map[string]bool{
	"-h":     true,
	"-help":  true,
	"--help": true,
}

// HelpOnly reports whether `args` is a bare request for a subcommand's help screen —
// a leading subcommand token followed by EXACTLY one token that is exactly `-h`, `-help`
// or `--help`.
//
// It is false for every other shape, including each of these, all of which must continue to
// parse normally:
//
//	nil / []                       — no subcommand at all
//	["create"]                     — a subcommand with no arguments
//	["create", "--help", "--json"] — more than one following token
//	["create", "--title", "--help"]— the token is another flag's VALUE
//	["create", "--", "--help"]     — after the terminator it is a positional
//	["--help"]                     — the top-level form, which each verb's own `run`
//	                                 already handles before this is reached
//
// The caller is expected to print its usage to stderr and return ExitOK, writing no audit
// row. HelpOnly itself has no side effect of any kind.
func HelpOnly(args []string) bool {
	if len(args) != 2 {
		return false
	}
	if helpTokens[args[0]] {
		// `--help create` is not a subcommand help request; the leading token is the
		// top-level form and belongs to the verb's own dispatch.
		return false
	}
	return helpTokens[args[1]]
}

// ErrHelpRequested is the sentinel a verb returns from its subcommand when `fs.Parse`
// reported `flag.ErrHelp`. It carries ExitOK — a help screen is a successful read, not a
// refusal — and verbs test for it with IsHelpRequest before writing any audit row.
//
// It is a DeskError so that `ExitCodeOf` maps it without a special case at every call site.
var ErrHelpRequested = &DeskError{Code: ExitOK, Msg: "help requested"}

// IsHelpRequest reports whether `err` is a help request — either this package's
// ErrHelpRequested sentinel or the flag package's own flag.ErrHelp, so a verb may hand
// through whatever `fs.Parse` returned without translating it first.
//
// An audit finalizer consults this and writes NO row when it is true. That is the whole
// point: a help screen must not be recorded as an invocation of the verb, because the
// ledger it would land in is counted for the write budget and is never rotated.
func IsHelpRequest(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, flag.ErrHelp) {
		return true
	}
	var de *DeskError
	if errors.As(err, &de) {
		return de == ErrHelpRequested
	}
	return false
}
