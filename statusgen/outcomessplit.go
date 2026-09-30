package main

// outcomessplit.go — `statusgen outcomes split`: the #882 migration from the shared appended
// docs/streams/verify-outcomes*.jsonl log(s) to the per-file layout under
// docs/streams/verify-outcomes/<stream>/.
//
// Writes one record file per legacy log line (verbatim bytes plus a trailing newline — a
// migrated record's digest therefore agrees with its source line's, so the reader in
// verifyoutcomes.go counts it once whichever layout is present). Idempotent: a record file
// that already exists at the path the line's own bytes derive is left untouched, so a second
// run writes nothing. The log itself is NEVER modified or deleted by this command — it stays in
// place, frozen (deskevidence refuses ever appending to it again), until every open PR that
// still touches it has landed (see the brief's Task step 8).
//
// `--check` audits without writing: it exits 1 and names every legacy line that has no record
// file yet, or 0 when every line is already split.

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const outcomesUsage = `statusgen outcomes split — migrate the shared verify-outcomes log to one file per record.

Usage:
  statusgen outcomes split --root <dir> [--check]

Writes one record file under docs/streams/verify-outcomes/<stream>/ per line of any
docs/streams/verify-outcomes*.jsonl (legacy log and rotation shards). Idempotent: a record
already present at the path its own bytes derive is left untouched. The log itself is never
modified or deleted.

--check audits only: writes nothing, exits 1 and names every line with no record file yet
(exit 0 when every line already has one).

Flags:
  --root <dir>   repo root to read/write (default: cwd)
  --check        audit only, no write
`

// runOutcomes is the `statusgen outcomes` entry point. args excludes the "outcomes" token
// main() already stripped.
func runOutcomes(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "split" {
		fmt.Fprint(stderr, outcomesUsage)
		return 2
	}

	fs := flag.NewFlagSet("outcomes split", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	rootDir := fs.String("root", ".", "repo root to read/write (default: cwd)")
	check := fs.Bool("check", false, "audit only: exit 1 naming any legacy line with no record file, write nothing")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, outcomesUsage)
			return 0
		}
		fmt.Fprint(stderr, outcomesUsage)
		return 2
	}
	root := *rootDir

	shards, gerr := verifyOutcomesShardPaths(root)
	if gerr != nil {
		fmt.Fprintf(stderr, "statusgen outcomes split: %v\n", gerr)
		return 1
	}

	var missing []string
	written := 0
	for _, p := range shards {
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			fmt.Fprintf(stderr, "statusgen outcomes split: cannot read %s: %v\n", p, rerr)
			return 1
		}
		for i, line := range bytes.Split(raw, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			rec, perr := parseOutcomeRecord(line)
			if perr != nil {
				fmt.Fprintf(stderr, "statusgen outcomes split: %s line %d: %v\n", p, i+1, perr)
				return 1
			}
			name, nerr := outcomeRecordName(line)
			if nerr != nil {
				fmt.Fprintf(stderr, "statusgen outcomes split: %s line %d (brief %s): %v\n", p, i+1, rec.Brief, nerr)
				return 1
			}
			target := filepath.Join(root, filepath.FromSlash(name))
			if _, serr := os.Stat(target); serr == nil {
				continue // already split — idempotent
			} else if !os.IsNotExist(serr) {
				fmt.Fprintf(stderr, "statusgen outcomes split: cannot stat %s: %v\n", target, serr)
				return 1
			}
			if *check {
				missing = append(missing, fmt.Sprintf("%s line %d (brief %s) -> %s", p, i+1, rec.Brief, name))
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				fmt.Fprintf(stderr, "statusgen outcomes split: %v\n", err)
				return 1
			}
			if err := os.WriteFile(target, outcomeCanonicalBytes(rec.Raw), 0o644); err != nil {
				fmt.Fprintf(stderr, "statusgen outcomes split: %v\n", err)
				return 1
			}
			written++
		}
	}

	if *check {
		if len(missing) > 0 {
			for _, m := range missing {
				fmt.Fprintln(stdout, "MISSING RECORD:", m)
			}
			return 1
		}
		fmt.Fprintln(stdout, "outcomes split --check: every legacy line has its record file")
		return 0
	}
	fmt.Fprintf(stdout, "outcomes split: wrote %d new record file(s)\n", written)
	return 0
}
