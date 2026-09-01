// Command deskcomms is the house desks' CLIENT surface onto the local cell
// gateway: a fail-fast preflight, then send / poll / ack over the desk-comms
// backbone (internal/comms).
//
// WHY A CLIENT-SIDE PREFLIGHT AT ALL, when the gateway enforces. Because a
// round trip to refuse the obvious — an out-of-lane triple, a reserved (human-gate)
// verb, an oversize or credential-shaped body — is a round trip wasted, and a desk
// gets a one-line refusal with a distinct exit code instead. The preflight is
// convenience, NOT the boundary: ENFORCEMENT IS THE GATEWAY'S, because not every
// participating agent is a desk running these verbs. So the preflight calls the
// SAME internal/comms parse and lane-ACL the gateway re-runs authoritatively — it
// is behaviour-identical by construction, never a second implementation that could
// drift permissive.
//
// IDENTITY IS NEVER SELF-CLAIMED. The sender's {cell, desk-role} come from the
// session context (DESK_CELL / DESK_ROLE, established at token-mint time), never
// from an argument — a caller cannot name themselves on the command line. The
// signed assertion (minted with the desk-role's custody-managed key) is what the
// gateway authenticates; the preflight identity is only there to build the message.
//
// A REFUSAL IS A STOP. Every refusal exits non-zero with a distinct code and a
// one-line reason; the desk reports it, it never routes around it. There is no
// local-spool fallback: an unreachable gateway is could-not-submit (fail closed),
// never a silent on-disk queue reported as delivered.
//
// Inherits deskkit: kill switch first (DISABLED > STOP > STOP.<loop>), one audit
// line per invocation naming which layer refused, fail-closed. Exit: 0 ok · 3
// disabled · 4 rate-limited · 5 refused · 6 unverifiable.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskcomms — client preflight onto the local cell gateway.

USAGE:
  deskcomms send --to <role> [--to-cell <cell>] --verb <verb> [--class routine|sensitive] [--ref <id>]...
                 < payload            # preflight, sign, and submit one message (payload on stdin)
  deskcomms poll                      # read this session's own per-role mailbox
  deskcomms ack <id>                  # acknowledge one notice (moves, never deletes)
  deskcomms --version

The sender's cell and desk-role come from the session context (DESK_CELL /
DESK_ROLE), never from an argument — you address who a message is FOR (--to /
--to-cell), never who it is FROM. The gateway's loopback socket is DESK_COMMS_GATEWAY;
the signing key is DESK_COMMS_KEY (mode 0600, never committed).

send preflight order: reserved-verb -> identity -> parse -> lane-ACL -> bodycheck ->
ratelimit -> mint -> submit. Each failed check refuses with a distinct exit code; the
audit line records which layer refused. Cross-cell reach is the-desk <-> the-desk and
its verb set ships empty, so every cross-cell message is refused until a recorded ruling.

Exit: 0 ok · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

// outcome carries the per-invocation audit facts back to run(): the rate-limit
// bucket a send charged (its destination lane) and a human detail. Keeping the
// audit to ONE line per invocation (the desk-tools audit contract) means the verb
// reports these rather than logging a second line of its own.
type outcome struct {
	bucket string
	detail string
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskcomms sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Kill switch first — the MANDATORY first gate of every desk tool (DISABLED >
	// STOP > STOP.<loop> > stale HEARTBEAT). An armed switch stops the flow before
	// any identity is resolved or any message is built; Guard writes its own
	// disabled audit line.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]

	// Every verb needs the session identity; build the real dependencies once.
	// A missing identity is a refusal buildDeps surfaces — except that `send`
	// reports the reserved-verb refusal even before identity, so it resolves its
	// own dependencies after that fast check (see below).
	var (
		oc  *outcome
		err error
	)
	switch sub {
	case "send":
		oc, err = dispatchSend(rest)
	case "poll":
		var d *deps
		if d, err = buildDeps(os.Stdin, os.Stdout); err == nil {
			oc, err = cmdPoll(d, rest)
		}
	case "ack":
		var d *deps
		if d, err = buildDeps(os.Stdin, os.Stdout); err == nil {
			oc, err = cmdAck(d, rest)
		}
	default:
		fmt.Fprintf(os.Stderr, "deskcomms: unknown subcommand %q\n\n%s\n", sub, usage)
		auditLine("unknown", "", deskkit.ResultRefused, "unknown subcommand")
		return deskkit.ExitRefused
	}

	if oc == nil {
		oc = &outcome{}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "deskcomms:", err.Error())
	}
	auditLine(sub, oc.bucket, resultFor(sub, err), detailFor(oc, err))
	return deskkit.ExitCodeOf(err)
}

// dispatchSend runs send's identity-independent reserved-verb check before it
// resolves session identity, so a reserved verb is reported as such even from a
// session that has no identity established. It parses the arguments twice only in
// the reserved-verb case; the common path parses once inside cmdSend.
func dispatchSend(args []string) (*outcome, error) {
	// Resolve dependencies; on a missing identity, still let cmdSend run so a
	// reserved verb refuses with its own message first. buildDeps returns a refusal
	// error for a missing identity, but we defer to cmdSend's ordering by passing a
	// best-effort deps: if identity is absent, cmdSend's reserved-verb branch fires
	// before its identity branch.
	d, derr := buildDeps(os.Stdin, os.Stdout)
	if derr != nil {
		// No identity in context. Build a minimal deps so the reserved-verb check
		// (which needs no identity) can still fire; if the verb is not reserved,
		// cmdSend returns the identity refusal.
		d = &deps{stdin: os.Stdin, stdout: os.Stdout, now: time.Now}
	}
	return cmdSend(d, args)
}

// resultFor maps a verb's terminal error to exactly one audit result. A successful
// send is a real outward write (ResultOK, charged to the rate meter); a successful
// poll/ack is a read (ResultDryRun, uncharged).
func resultFor(sub string, err error) string {
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitOK:
		if sub == "send" {
			return deskkit.ResultOK
		}
		return deskkit.ResultDryRun
	case deskkit.ExitDisabled:
		return deskkit.ResultDisabled
	case deskkit.ExitRateLimited:
		return deskkit.ResultRateLimited
	case deskkit.ExitRefused:
		return deskkit.ResultRefused
	default:
		return deskkit.ResultUnverifiable
	}
}

func detailFor(oc *outcome, err error) string {
	if err == nil {
		if oc != nil && oc.detail != "" {
			return oc.detail
		}
		return "ok"
	}
	if oc != nil && oc.detail != "" {
		return deskkit.StripControl(oc.detail)
	}
	return deskkit.StripControl(err.Error())
}

func auditLine(verb, bucket, result, detail string) {
	sha, built := deskkit.Version()
	_ = deskkit.Log(deskkit.Entry{
		TS:         time.Now().UTC().Format(time.RFC3339),
		Tool:       "deskcomms",
		Verb:       verb,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
		Repo:       bucket,
		Result:     result,
		Detail:     detail,
		SourceSHA:  sha,
		BuiltAt:    built,
		SessionTag: deskkit.SessionTag(),
	})
}
