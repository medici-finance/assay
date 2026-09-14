// Command deskaudit is the maintenance surface for the shared audit log
// (~/.config/assay/audit.jsonl), which is load-bearing state: the rate-limit counter, the
// circuit breaker, and the idempotency store all derive from it.
//
// Its one verb today is `recover`, the SANCTIONED, non-destructive replacement for the old
// "move the whole file aside" remedy a corruption refusal used to print. A single malformed
// line (a partial append from `kill -9`, a disk-full write, a sync-tool rewrite) makes every
// desk tool refuse (exit 6). Moving the whole file aside clears the corruption but RESETS the
// state — budgets return to full and the idempotency store forgets every prior write, so
// re-runs post duplicates. `deskaudit recover` instead quarantines only the malformed LINE
// into an audit.jsonl.corrupt-<ts> sidecar and carries every good entry forward, so the
// counter and idempotency state survive the recovery (deskkit.RecoverCorruptAudit).
//
// Its second verb is `tail`, the read the ledger never had. Operators reached for `tail`/`jq`
// on a file whose location and (since #1035) daily rotation the tool owns; `deskaudit tail N`
// prints the newest N entries across the segment boundary as the RAW lines exactly as they sit
// on disk — the same no-reserialisation principle recovery applies, for the same reason. It
// reads backwards and stops at N, so it costs a bounded read whatever the ledger's size.
//
//	deskaudit recover     quarantine bad lines, carry good entries forward
//	deskaudit tail [N]    print the newest N entries (default 10) across segments
//	deskaudit --version   print the build stamp
//	deskaudit -h          this text
//
// Guard() runs first, as for every desk tool: recovery is refused while the kill switch or a
// stop flag is armed. Recovery takes the shared audit flock, so it cannot race a live write.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskaudit — maintenance for the shared audit log (~/.config/assay/audit.jsonl).

usage:
  deskaudit recover     quarantine any malformed line(s) into audit.jsonl.corrupt-<ts> and
                        carry every good entry forward, preserving the rate-limit counter and
                        the idempotency store (a plain file move resets both)
  deskaudit tail [N]    print the newest N entries (default 10, max 10000) as the raw
                        lines they are on disk, reading backwards across the daily
                        segments; reads only, locks nothing, writes nothing
  deskaudit --version   print the build stamp
  deskaudit -h | --help this text

exit: 0 ok · 3 disabled (kill switch/stop flag) · 6 unverifiable`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskaudit sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Mandatory first call, like every desk tool.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, "deskaudit:", err.Error())
		return deskkit.ExitCodeOf(err)
	}

	switch args[0] {
	case "recover":
		if len(args) != 1 {
			fmt.Fprintf(stderr, "deskaudit recover: takes no arguments\n%s\n", usage)
			return deskkit.ExitRefused
		}
		res, err := deskkit.RecoverCorruptAudit()
		if err != nil {
			fmt.Fprintln(stderr, "deskaudit recover:", err.Error())
			return deskkit.ExitCodeOf(err)
		}
		if !res.Rewrote {
			fmt.Fprintf(stdout, "deskaudit recover: audit log already clean — %d entries, nothing quarantined\n", res.Carried)
			return deskkit.ExitOK
		}
		fmt.Fprintf(stdout,
			"deskaudit recover: carried %d entries forward, quarantined %d malformed line(s) to %s "+
				"(rate-limit counter and idempotency store preserved)\n",
			res.Carried, res.Quarantined, res.QuarantinePath)
		return deskkit.ExitOK
	case "tail":
		return runTail(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "deskaudit: unknown verb %q\n%s\n", args[0], usage)
		return deskkit.ExitRefused
	}
}

// tailDefault / tailMax bound the read verb. The cap is not a policy about how much history
// is interesting — it is what keeps `tail` a bounded read: an unbounded N is `LoadEntries`
// with extra steps.
const (
	tailDefault = 10
	tailMax     = 10000
)

// runTail prints the newest N ledger entries as the RAW lines they are on disk.
//
// Three states, reported distinctly (an instrument that did not look has not cleared
// anything): an ABSENT ledger is empty history and exits 0 saying so; an existing ledger it
// cannot read exits 6 with the reason; and a line that is not valid JSON is PRINTED, with a
// note naming the recovery verb — never dropped, because a reader silently skipping the one
// line that broke every other tool is the opposite of what this verb is for.
func runTail(args []string, stdout, stderr io.Writer) int {
	n := tailDefault
	switch len(args) {
	case 0:
	case 1:
		v, err := strconv.Atoi(args[0])
		if err != nil || v <= 0 {
			fmt.Fprintf(stderr, "deskaudit tail: N must be a positive integer, got %q\n", args[0])
			return deskkit.ExitRefused
		}
		if v > tailMax {
			fmt.Fprintf(stderr, "deskaudit tail: N is capped at %d (asked for %d)\n", tailMax, v)
			return deskkit.ExitRefused
		}
		n = v
	default:
		fmt.Fprintf(stderr, "deskaudit tail: takes at most one argument\n%s\n", usage)
		return deskkit.ExitRefused
	}

	lines, present, err := deskkit.TailLines(n)
	if err != nil {
		fmt.Fprintln(stderr, "deskaudit tail:", err.Error())
		return deskkit.ExitCodeOf(err)
	}
	if !present {
		fmt.Fprintln(stdout, "deskaudit tail: no audit history — the ledger does not exist yet")
		return deskkit.ExitOK
	}
	if len(lines) == 0 {
		fmt.Fprintln(stdout, "deskaudit tail: audit history present but empty — 0 entries")
		return deskkit.ExitOK
	}
	bad := 0
	for _, l := range lines {
		if !json.Valid([]byte(l)) {
			bad++
		}
		fmt.Fprintln(stdout, l)
	}
	if bad > 0 {
		fmt.Fprintf(stderr, "deskaudit tail: %d of the %d line(s) above are not valid JSON — run `deskaudit recover`\n", bad, len(lines))
	}
	return deskkit.ExitOK
}
