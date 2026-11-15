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
//	deskaudit -h          this text (generated from the command tree, desktools-v2/47)
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

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// helpLong is the root help's prose. The verbs are generated from the command tree
// (desktools-v2/47); this is what the tree cannot say on its own.
const helpLong = `deskaudit — maintenance for the shared audit log (~/.config/assay/audit.jsonl).

  deskaudit recover     quarantine any malformed line(s) into audit.jsonl.corrupt-<ts> and
                        carry every good entry forward, preserving the rate-limit counter and
                        the idempotency store (a plain file move resets both)
  deskaudit tail [N]    print the newest N entries (default 10, max 10000) as the raw
                        lines they are on disk, reading backwards across the daily
                        segments; reads only, locks nothing, writes nothing
  deskaudit --version   print the build stamp
  deskaudit -h | --help this text

exit: 0 ok · 3 disabled (kill switch/stop flag) · 5 refused (bad command line) · 6 unverifiable`

func main() {
	os.Exit(deskkit.PerfRun("deskaudit", nil, os.Args[1:], func(a []string) int { return run(a, os.Stdout, os.Stderr) }))
}

// run is one invocation: deskaudit has no process startup (it reads no roster), so the command
// tree is the whole of it.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr)
}

// guardFn, recoverFn and tailFn are what the command tree calls: the kill switch, then the
// verb's typed handler. The contract tests replace them to count what a command line reaches.
var (
	guardFn   = deskkit.Guard
	recoverFn = recoverAudit
	tailFn    = runTail
)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == deskkit.ExitOK {
		return nil
	}
	return exitErr(code)
}

// versionLine is what --version, -version and the legacy `version` word print.
func versionLine() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskaudit sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

// guard is the mandatory first call of every verb, like every desk tool: recovery and the read
// are refused while the kill switch or a stop flag is armed.
func guard(stderr io.Writer) error {
	if err := guardFn(); err != nil {
		fmt.Fprintln(stderr, "deskaudit:", err.Error())
		return exitErr(deskkit.ExitCodeOf(err))
	}
	return nil
}

// tailN parses tail's optional N: absent is the default, otherwise a positive integer up to
// the cap. It runs at parse time, so a bad N is refused before the kill switch or the read.
func tailN(args []string) (int, error) {
	switch len(args) {
	case 0:
		return tailDefault, nil
	case 1:
		v, err := strconv.Atoi(args[0])
		if err != nil || v <= 0 {
			return 0, fmt.Errorf("tail: N must be a positive integer, got %q", args[0])
		}
		if v > tailMax {
			return 0, fmt.Errorf("tail: N is capped at %d (asked for %d)", tailMax, v)
		}
		return v, nil
	default:
		return 0, fmt.Errorf("tail: takes at most one argument")
	}
}

// newTree builds deskaudit's command tree on the given streams. It declares no setting: no
// flag beyond help and version, no environment variable, no configuration key.
func newTree(stdout, stderr io.Writer) *cobra.Command {
	root := cli.NewRoot("deskaudit", "maintenance for the shared audit log")
	root.Long = helpLong
	// An unknown verb is refused before anything runs.
	root.Args = cobra.NoArgs
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		// A bare invocation prints the usage to stderr and refuses, as before.
		cmd.SetOut(stderr)
		_ = cmd.Help()
		return exitErr(deskkit.ExitRefused)
	}
	root.AddCommand(&cobra.Command{
		Use:   "recover",
		Short: "quarantine malformed line(s) and carry every good entry forward",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("recover: takes no arguments")
			}
			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			if err := guard(stderr); err != nil {
				return err
			}
			return codeErr(recoverFn(stdout, stderr))
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "tail [N]",
		Short: fmt.Sprintf("print the newest N entries (default %d, max %d) across segments", tailDefault, tailMax),
		Args: func(_ *cobra.Command, args []string) error {
			_, err := tailN(args)
			return err
		},
		RunE: func(_ *cobra.Command, args []string) error {
			n, _ := tailN(args)
			if err := guard(stderr); err != nil {
				return err
			}
			return codeErr(tailFn(n, stdout, stderr))
		},
	})
	// `deskaudit version` printed the build stamp before the migration; it still does.
	root.AddCommand(&cobra.Command{Use: "version", Short: "print the version", Hidden: true, Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			fmt.Fprintln(stdout, versionLine())
			return nil
		}})
	return root
}

// execute runs one invocation against a fresh command tree.
func execute(args []string, stdout, stderr io.Writer) int {
	return cli.Run(func() *cobra.Command { return newTree(stdout, stderr) }, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A command line deskaudit cannot parse (an unknown verb, a bad N) is refused (5).
		UsageExit: deskkit.ExitRefused,
		// -version was a legacy spelling and stays accepted.
		GoFlagCompat: true,
	})
}

// recoverAudit is `deskaudit recover`: deskkit.RecoverCorruptAudit, reported.
func recoverAudit(stdout, stderr io.Writer) int {
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
func runTail(n int, stdout, stderr io.Writer) int {
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
