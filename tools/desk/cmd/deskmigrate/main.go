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
//
// A no-op is only CLEAN when there is nothing to migrate. When no migration
// covers the requested span but the tree still carries `schema: brief-v1` files
// under docs/streams, the runner REFUSES (exit 5) instead of printing a no-op at
// exit 0 — an unmigrated tree that reports success is indistinguishable from a
// migrated one, and the cause is almost always that the release's migrations/
// directory was never vendored into the tree.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskmigrate — the version migration runner.

USAGE:
  deskmigrate --from vX.Y.Z --to vX.Y.Z --root <dir> [--dry-run] [--notes]
  deskmigrate --version

Selects the ordered migrations under <root>/migrations whose [from,to] span lies
within [--from,--to] and applies them idempotently (or previews under --dry-run).

Every run prints the number of migrations SELECTED and the number of planned file
actions, so an empty selection is legible rather than silent.

Exit codes:
  0  applied, or a clean no-op (nothing to do, or already migrated)
  5  refused (bad version range; or no migration covers the span while the tree
     still carries schema: brief-v1 files — the migrations were never vendored in)
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

	migrationsDir := filepath.Join(*root, deskkit.MigrationsDir)
	migs, err := deskkit.LoadMigrations(migrationsDir)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	selected, err := deskkit.SelectMigrations(migs, *from, *to)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	mode := "apply"
	if *dryRun {
		mode = "dry-run"
	}
	// Counts on EVERY run, before any branch: an operator reading the output can
	// tell "0 selected" (nothing matched the span) from "3 selected, 0 planned"
	// (matched but idempotently already applied) without re-deriving either.
	fmt.Fprintf(stdout, "%s: %d migration(s) selected for %s -> %s (migrations dir %s)\n",
		mode, len(selected), *from, *to, migrationsDir)

	if len(selected) == 0 {
		// A SILENT no-op is the failure this guards. On a tree that still carries
		// brief-v1 files, "no migrations for vX -> vY (clean no-op)" at exit 0 is
		// indistinguishable from a successful migration — the operator reads
		// success and moves on with an unmigrated tree. An empty migration set on
		// a tree with brief-v1 content is a determinate precondition failure: the
		// migration files were never vendored into this tree.
		v1, err := briefV1Files(*root)
		if err != nil {
			fmt.Fprintf(stderr, "deskmigrate: could-not-check whether %s carries brief-v1 files: %v\n", *root, err)
			return deskkit.ExitUnverifiable
		}
		if len(v1) > 0 {
			fmt.Fprintf(stdout, "  0 planned file action(s)\n")
			fmt.Fprintf(stderr,
				"deskmigrate: refusing a silent no-op — no migration covers %s -> %s, but %s carries %d file(s) still on `schema: brief-v1` (e.g. %s).\n"+
					"  %s holds no migration for this span (vendor the release's migrations/ directory into the tree — copy the migration files shipped with the target version under %s — then re-run).\n"+
					"  Exiting non-zero so this is not read as a completed migration.\n",
				*from, *to, *root, len(v1), relOrAbs(*root, v1[0]), migrationsDir, migrationsDir)
			return deskkit.ExitRefused
		}
		fmt.Fprintf(stdout, "no migrations for %s -> %s (clean no-op) — 0 planned file action(s)\n", *from, *to)
		return deskkit.ExitOK
	}

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
	fmt.Fprintf(stdout, "  %d planned file action(s)\n", len(actions))
	return deskkit.ExitOK
}

// briefV1SchemaLine is the frontmatter line that marks a brief as still on the
// pre-migration schema. Matched on the whole trimmed line so a `schema:` value of
// some other document kind never counts.
var briefV1SchemaLine = regexp.MustCompile(`(?m)^schema:\s*"?brief-v1"?\s*$`)

// briefV1Files lists the markdown files under <root>/docs/streams whose
// frontmatter is still `schema: brief-v1`. An ABSENT docs/streams is a legitimate
// empty (nil, nil) — a tree with no board carries no briefs to migrate. An
// UNREADABLE one is an error, never rounded down to "no brief-v1 files": a
// could-not-check must not be laundered into a clean no-op, which is the exact
// class of silence this whole check exists to end.
func briefV1Files(root string) ([]string, error) {
	base := filepath.Join(root, "docs", "streams")
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if briefV1SchemaLine.Match(raw) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// relOrAbs renders path relative to root for a readable message, falling back to
// the path as given.
func relOrAbs(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
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
