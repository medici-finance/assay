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
//	deskaudit recover     quarantine bad lines, carry good entries forward
//	deskaudit --version   print the build stamp
//	deskaudit -h          this text
//
// Guard() runs first, as for every desk tool: recovery is refused while the kill switch or a
// stop flag is armed. Recovery takes the shared audit flock, so it cannot race a live write.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskaudit — maintenance for the shared audit log (~/.config/assay/audit.jsonl).

usage:
  deskaudit recover     quarantine any malformed line(s) into audit.jsonl.corrupt-<ts> and
                        carry every good entry forward, preserving the rate-limit counter and
                        the idempotency store (a plain file move resets both)
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
	default:
		fmt.Fprintf(stderr, "deskaudit: unknown verb %q\n%s\n", args[0], usage)
		return deskkit.ExitRefused
	}
}
