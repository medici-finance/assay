// Command deskmigrate is the migration runner: given a from-version and a
// to-version, it selects the ordered applicable migrations from <root>/migrations
// and applies them idempotently, or previews them under --dry-run. It ships with
// FIXTURE migrations only — no real migration exists yet, and this tool has no live
// caller; the `upgrade-assay` verb an adopter actually runs wraps it.
//
// The migration format is defined by deskkit/migrate.go, its executable half.
//
// USAGE:
//
//	deskmigrate --from vX.Y.Z --to vX.Y.Z --root <dir> [--dry-run] [--notes]
//	deskmigrate --version
//
// --dry-run computes exactly what an apply would do and writes NOTHING.
// --notes additionally prints each selected migration's human "what changed" body
// (the release-note prose upgrade-assay surfaces).
//
// Re-running the same command on an already-migrated tree is a clean no-op.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskmigrate — the version migration runner.

USAGE:
  deskmigrate --from vX.Y.Z --to vX.Y.Z --root <dir> [--dry-run] [--notes]
  deskmigrate --version

Selects the ordered migrations under <root>/migrations whose [from,to] span lies
within [--from,--to] and applies them idempotently (or previews under --dry-run).

Exit codes:
  0  applied, or a clean no-op (nothing to do, or already migrated)
  5  refused (bad version range)
  6  could-not-run (a migration file is unreadable or malformed)`

func main() {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("deskmigrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		from    = fs.String("from", "", "from-version (vX.Y.Z)")
		to      = fs.String("to", "", "to-version (vX.Y.Z)")
		root    = fs.String("root", ".", "adopter repo root to migrate")
		dryRun  = fs.Bool("dry-run", false, "preview only; write nothing")
		notes   = fs.Bool("notes", false, "also print each migration's human `what changed` body")
		version = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}

	if *version {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskmigrate sourceSHA=%s builtAt=%s releaseTag=%s\n",
			sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if *from == "" || *to == "" {
		fmt.Fprintln(stderr, "deskmigrate: --from and --to are required")
		fmt.Fprintln(stderr, usage)
		return deskkit.ExitRefused
	}

	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(stderr)

	migs, err := deskkit.LoadMigrations(filepath.Join(*root, deskkit.MigrationsDir))
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	selected, err := deskkit.SelectMigrations(migs, *from, *to)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	if len(selected) == 0 {
		fmt.Fprintf(stdout, "no migrations for %s -> %s (clean no-op)\n", *from, *to)
		return deskkit.ExitOK
	}

	mode := "apply"
	if *dryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(stdout, "%s: %d migration(s) for %s -> %s\n", mode, len(selected), *from, *to)
	for _, m := range selected {
		fmt.Fprintf(stdout, "  migration %s (%s -> %s)\n", m.ID, m.From, m.To)
		if *notes {
			fmt.Fprintf(stdout, "    what changed:\n%s\n", indent(m.Notes, "      "))
		}
	}

	actions, err := deskkit.RunMigrations(*root, selected, *dryRun)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	for _, a := range actions {
		fmt.Fprintf(stdout, "  [%s] %s\n", a.Migration, a.Desc)
	}
	return deskkit.ExitOK
}

// indent prefixes every line of s with pre.
func indent(s, pre string) string {
	out := ""
	for _, line := range splitLines(s) {
		out += pre + line + "\n"
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	lines = append(lines, cur)
	return lines
}
