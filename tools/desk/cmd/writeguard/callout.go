package main

// The adopter DANGEROUS-COMMAND callout.
//
// WHAT IT IS FOR. The compiled indicators in guard.go recognise write-shaped
// constructs that are dangerous in ANY tree — redirection, tee, the file-mutation
// family, sed -i, mutating git subcommands. They are generic by construction, and
// they ship. What they cannot recognise is a command that is dangerous only because
// of what a PARTICULAR organisation's build system does with it: a build tool whose
// name means nothing here writes artefacts into the tree it is invoked from, and only
// the adopter knows which invocations those are.
//
// Compiling that knowledge into a shared guard puts one organisation's toolchain in
// everybody's binary — and worse, makes it look like a general rule when it is not.
// Putting it in a CONFIG LIST is not better: a list of patterns is a policy the guard
// must then interpret, and a pattern language on a write gate grows an escape.
//
// So the adopter supplies an EXECUTABLE (ASSAY_WRITEGUARD_CALLOUT) and the guard asks
// it. This is deliberately the SAME extension-point shape the risk classifier uses —
// one pattern for both gates, so a reader who has understood one has understood the
// other, and the exec/timeout/refusal plumbing is shared (deskkit.Callout) rather
// than written twice with two different sets of failure modes.
//
// THE TWO RULES, and where each is enforced:
//
//	ONLY-WIDENS   Enforced STRUCTURALLY, in CheckBash: the compiled indicators are
//	              consulted FIRST and return their block before this code runs at
//	              all. There is no output a callout can produce that reaches a
//	              compiled verdict, so "allow" is not a word this protocol has to
//	              defend against — the callout is only ever asked about commands the
//	              compiled indicators have already declined to block.
//	FAIL-CLOSED   Enforced HERE, and for a write guard fail-closed means BLOCK. Every
//	              way the question can go unanswered — the callout cannot be
//	              executed, exits non-zero, exceeds the timeout, says nothing, or
//	              says something this guard does not recognise — is a BLOCK. An
//	              adopter who configured a callout asked for it to be consulted;
//	              allowing on "I could not consult it" answers a question nobody put.
//
// UNSET IS NOT A FAILURE. No callout configured means the compiled generic
// indicators alone, which is the shipped behaviour byte for byte. The fail-closed
// rule starts the moment a callout is configured, not before — otherwise publishing
// the guard would block every write in every tree that has not adopted one.
//
// RESIDUAL, recorded rather than taken silently: the callout path is one field of a
// roster Config, and a roster that fails validation collapses to unconfigured as a
// whole (deskkit/rosterconfig.go's fail-closed rule) — so a typo in an UNRELATED
// roster key leaves this guard running compiled-indicators-only while the operator
// believes a callout is in force. That is the shipped behaviour, not a silent
// weakening of a compiled check, and the collapse is announced loudly in the P3 echo
// this guard now emits on every run. Making it BLOCK instead would mean one bad
// character in roster.env refusing every file write on the machine.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// calloutRequest is what the guard puts on the callout's STDIN, as one JSON object.
//
// Stdin rather than argv, on purpose: a command string is arbitrary text of arbitrary
// length, and argv has a size limit and a quoting story. JSON rather than the raw
// command, also on purpose: the callout needs the cwd and the shared root to judge
// where a write lands, and a self-describing envelope can gain a field without every
// existing callout having to be rewritten to count positional arguments.
type calloutRequest struct {
	// Version lets a callout refuse a request shape it does not know, instead of
	// guessing. It is bumped only if a field's MEANING changes; adding a field does
	// not bump it.
	Version int `json:"version"`
	// Tool is the tool surface the call came through. "Bash" is the only value today
	// — the compiled indicator this replaces was a command check — but a callout can
	// switch on it rather than assuming.
	Tool string `json:"tool"`
	// Command is the VERBATIM command string, exactly as the guard received it. It is
	// never shell-interpreted here: deskkit.Callout execs the callout directly, so
	// this text is data all the way down.
	Command string `json:"command"`
	// Cwd is the session's current directory as the hook payload reported it ("" when
	// unknown), and SharedRoot the checkout the guard is protecting.
	Cwd        string `json:"cwd"`
	SharedRoot string `json:"shared_root"`
}

// calloutRequestVersion is the current request-shape version.
const calloutRequestVersion = 1

// The callout's ANSWER vocabulary, read from the first whitespace-separated token of
// its stdout. Exactly two words are recognised, and anything else is a BLOCK:
//
//	allow   this callout has no objection to the command. It does NOT clear any
//	        compiled indicator — by the time the callout is asked, none matched.
//	block   refuse the command. Any remaining text on the line is the reason, shown
//	        to the caller so the adopter's own words explain the refusal.
//
// A deliberately TINY vocabulary. A protocol with a rich answer space is a protocol
// where an unfamiliar answer has to be mapped onto a decision, and every such mapping
// is a place a guard learns to allow something it did not understand.
const (
	calloutAllow = "allow"
	calloutBlock = "block"
)

// checkCallout asks the adopter's callout about command.
//
// It returns blocked=false ONLY in the two cases where a refusal would be wrong: no
// callout is configured, or a configured callout ran cleanly and said "allow".
// Everything else blocks, with a reason naming which of the failure modes happened —
// an operator who has just been refused has to be able to tell "my callout said no"
// from "my callout is broken", because the fix is different.
func (c Config) checkCallout(command string) (reason string, blocked bool) {
	if c.Callout == "" {
		return "", false // unset: compiled generic indicators only, the shipped behaviour
	}

	req := calloutRequest{
		Version:    calloutRequestVersion,
		Tool:       "Bash",
		Command:    command,
		Cwd:        c.Cwd,
		SharedRoot: c.SharedRoot,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		// Unreachable for a struct of strings and an int, but a guard does not get to
		// have an unreachable branch that falls through to "allow".
		return calloutFailure(c.Callout, "the request could not be encoded: "+err.Error()), true
	}

	res, err := deskkit.Callout{Path: c.Callout, Timeout: c.CalloutTimeout}.Run(string(payload))
	if err != nil {
		detail := err.Error()
		if res.Stderr != "" {
			detail += "\n  callout stderr: " + firstLines(res.Stderr, 5)
		}
		return calloutFailure(c.Callout, detail), true
	}

	verdict, rest, _ := strings.Cut(strings.TrimSpace(res.Stdout), " ")
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case calloutAllow:
		return "", false
	case calloutBlock:
		why := strings.TrimSpace(rest)
		if why == "" {
			why = "(the callout gave no reason)"
		}
		return fmt.Sprintf(`writeguard: BLOCKED by the adopter's dangerous-command callout.
Callout: %s
Reason:  %s
Command: %s
This is adopter policy, not a rule of the shared guard — the callout named this command as one this organisation refuses to run against the shared checkout. Work in your own worktree instead:
  git worktree add ../<repo>-<name> -b <branch> origin/main   # absolute sibling path
If you believe the callout is wrong, that is a conversation with whoever owns it; this guard has no waiver.`,
			c.Callout, why, command), true
	default:
		// An answer outside the vocabulary is the same class of event as no answer at
		// all: the guard did not learn anything about this command.
		got := strings.TrimSpace(res.Stdout)
		if got == "" {
			got = "(nothing)"
		}
		return calloutFailure(c.Callout, fmt.Sprintf(
			"it printed %q, which is neither %q nor %q", firstLines(got, 3), calloutAllow, calloutBlock)), true
	}
}

// calloutFailure renders the FAIL-CLOSED refusal: the callout was configured and
// could not answer, so the command is blocked and the message says plainly that this
// is the guard failing closed rather than the callout objecting.
func calloutFailure(path, detail string) string {
	return fmt.Sprintf(`writeguard: BLOCKED — the configured dangerous-command callout did not answer.
Callout: %s
Problem: %s
A write guard fails CLOSED: a callout that cannot be consulted is refused, not waived. Nothing about this command was judged — it is blocked because the check configured for it did not run.
Fix the callout (it must be an executable, not group- or world-writable, exiting 0 and printing %q or %q), or unset %s to fall back to the shared guard's compiled indicators alone.`,
		path, detail, calloutAllow, calloutBlock, deskkit.EnvWriteguardCallout)
}

// firstLines trims a multi-line diagnostic down to n lines so a block message stays
// readable when a callout dumps a stack trace.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + "\n  …"
}
