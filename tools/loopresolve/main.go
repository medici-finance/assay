// Command loopresolve answers the one question the loop/worker entrypoint has
// to ask of cells.yaml: "for cell C and role R, what is the schedule, the model
// ladder, the App identity, and the repo set?"
//
// It prints shell-sourceable KEY=VALUE lines so `images/loop-base/entrypoint.sh`
// can `eval` the answer without carrying a YAML parser (or a `yq` pin) into the
// image. Parsing and validation are the config/schema package's — the same code
// `cellsvalidate` runs — so the image cannot drift from the schema.
//
// It is a PURE READ: it opens one file, prints to stdout, and exits. It makes no
// network call, mints no credential and writes nothing, which is what lets
// `entrypoint.sh --smoke` be inert while still genuinely parsing config.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/cellconfig/config/schema"
)

// Exit codes. Kept deliberately narrow and documented because entrypoint.sh
// branches on them: 2 is "you called me wrong or named something that does not
// exist" (a deploy defect — the pod should crash, not quietly file), 1 is
// "the config is there but does not parse/validate".
const (
	exitOK      = 0
	exitBadFile = 1
	exitUsage   = 2
)

func main() {
	fs := flag.NewFlagSet("loopresolve", flag.ContinueOnError)
	cells := fs.String("cells", "", "path to cells.yaml (required)")
	cell := fs.String("cell", "", "cell name to resolve (required)")
	role := fs.String("role", "", "loop role to resolve within that cell (required)")
	prefix := fs.String("prefix", "ASSAY_", "prefix for the emitted variable names")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: loopresolve --cells <cells.yaml> --cell <name> --role <role> [--prefix P]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(exitUsage)
	}
	if *cells == "" || *cell == "" || *role == "" {
		fs.Usage()
		os.Exit(exitUsage)
	}

	f, err := schema.Load(*cells)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loopresolve: %v\n", err)
		os.Exit(exitBadFile)
	}

	out, err := Resolve(f, *cell, *role, *prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loopresolve: %v\n", err)
		os.Exit(exitUsage)
	}
	fmt.Print(out)
}

// Resolve renders the (cell, role) pair as shell-sourceable assignments, or
// reports which of the two did not exist. Naming the alternatives in the error
// is deliberate: a CronJob that names a role the cell does not run is a config
// typo, and "role %q not found (cell runs: review verify)" is the difference
// between a one-line fix and a kubectl archaeology session.
func Resolve(f *schema.File, cellName, roleName, prefix string) (string, error) {
	var c *schema.Cell
	names := make([]string, 0, len(f.Cells))
	for i := range f.Cells {
		names = append(names, f.Cells[i].Name)
		if f.Cells[i].Name == cellName {
			c = &f.Cells[i]
		}
	}
	if c == nil {
		return "", fmt.Errorf("no cell named %q (file defines: %s)", cellName, strings.Join(names, " "))
	}

	var l *schema.Loop
	roles := make([]string, 0, len(c.Loops))
	for i := range c.Loops {
		roles = append(roles, string(c.Loops[i].Role))
		if string(c.Loops[i].Role) == roleName {
			l = &c.Loops[i]
		}
	}
	if l == nil {
		return "", fmt.Errorf("cell %q runs no %q loop (it runs: %s)", cellName, roleName, strings.Join(roles, " "))
	}

	var b strings.Builder
	emit := func(k, v string) {
		fmt.Fprintf(&b, "%s%s=%s\n", prefix, k, shellQuote(v))
	}
	emit("CELL", c.Name)
	emit("ROLE", string(l.Role))
	emit("SCHEDULE", l.Schedule)
	// Space-separated, cheapest-first: the ladder's ORDER is its meaning
	// (run the cheap default, escalate when the item is judged hard),
	// so it is passed through as a list rather than collapsed to one name.
	emit("MODELS", strings.Join(l.Models, " "))
	emit("MODEL", first(l.Models))
	emit("APP", l.App)
	emit("REPOS", strings.Join(c.Repos, " "))
	emit("STREAMS_ROOT", c.StreamsRoot)
	emit("SILENCE_THRESHOLD", c.SilenceThreshold)
	return b.String(), nil
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// shellQuote renders v as a single-quoted POSIX shell word, so that `eval` of
// the output cannot execute anything the config file happens to contain. A cron
// expression alone (`*/10 * * * *`) would glob and word-split unquoted; anything
// worse would be an injection through a config file into a pod's shell.
func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
