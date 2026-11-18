package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// verifyoutcomes.go — the READ side of #1338 part 2: a rotation-aware
// union reader for the docs/streams/verify-outcomes.jsonl append-only aggregate sidecar.
//
// WHY THIS EXISTS. #1338 raised deskevidence's per-file cap for verify-outcomes.jsonl itself
// (tools/desk/cmd/deskevidence/deskevidence.go, verifyOutcomesMaxBytes) because the sidecar
// grows monotonically fleet-wide with no natural per-write ceiling. That raise buys headroom,
// not permanence: the file will eventually need to shrink by ROTATING onto dated shards
// (verify-outcomes-2026-10.jsonl, …), and the forge write path refuses shrinking a file at
// all (the write_file_shrink_refused golden in tools/desk) — so a rotation can only ever ADD a
// new shard and leave the old one exactly as it stood. Nothing may then assume the canonical
// unsharded path is the only place a row can be, which means the READ side has to support a
// glob union BEFORE any rotation is attempted, or the moment one lands every existing reader
// goes silently blind to whichever rows moved into the new shard.
//
// This file is that read side. It does not implement rotation itself — no writer in this
// repo splits the file yet — it only makes sure a reader that wants "every verify-outcome row
// on record" never has to special-case a rotation once one exists.

// verifyOutcomesShardPaths returns the sorted, de-duplicated absolute paths of every
// verify-outcomes shard directly under <root>/docs/streams: the canonical unsharded file
// (verify-outcomes.jsonl) plus any dated rotation shard a future rotation has created
// (verify-outcomes-2026-10.jsonl, …) — one glob, the same pattern
// deskevidence's own write-side cap override matches against
// (tools/desk/cmd/deskevidence/deskevidence.go's verifyOutcomesGlobPattern). The two literals
// live in separate Go modules and so cannot share one constant; keep them byte-identical by
// hand — a drift here would mean the write side raises a shard's size cap while the read side
// silently stops unioning it, or vice versa.
//
// Lexical sort also reads chronologically for the YYYY-MM-shaped shard suffix this convention
// implies: "-" (0x2D) sorts before "." (0x2E), so every hyphenated dated shard
// (verify-outcomes-2026-10.jsonl, …) sorts AHEAD of the plain unsharded
// "verify-outcomes.jsonl" — oldest shard first, unsharded file last. Ordering across shards is
// not load-bearing for THIS function (see the no-ordering-needed note below); it is documented
// here only so a future caller that does care is not surprised by which position wins on a
// last-write-wins reduction. Per docs/streams/fresh-views/brief-04's own finding (recorded when
// verify-outcomes.jsonl was set merge=union), no known consumer needs strict ordering or
// de-duplication of this log, so this function does not attempt either; a future consumer
// that does needs it is responsible for imposing it on the returned rows itself.
func verifyOutcomesShardPaths(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "docs", "streams", verifyOutcomesGlob))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

// readVerifyOutcomesUnion returns the UNIONED content of every verify-outcomes shard under
// <root>/docs/streams, concatenated in verifyOutcomesShardPaths' sorted order. A missing
// docs/streams directory or zero shards is NOT an error — it returns (nil, nil), the "nothing
// to union yet" shape an adopter tree with no verify-outcomes history has.
//
// Every reader of verify-outcome rows should go through this function (or
// verifyOutcomesShardPaths directly, for a caller that needs the per-shard boundary) rather
// than opening docs/streams/verify-outcomes.jsonl by its bare path — that is precisely the
// assumption a future rotation breaks. Each shard's content is newline-terminated before the
// next is appended, so a shard file missing its own trailing newline (a manual edit, a
// truncated write) can never fuse its last row with the next shard's first.
func readVerifyOutcomesUnion(root string) ([]byte, error) {
	paths, err := verifyOutcomesShardPaths(root)
	if err != nil {
		return nil, err
	}
	var out []byte
	for _, p := range paths {
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil, rerr
		}
		if len(b) > 0 && b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}
		out = append(out, b...)
	}
	return out, nil
}

// --- verify-outcome record layer: an adapter over the shared protocol -----------------------
//
// The record codec, naming rule, vocabulary, per-brief reduction and the fs.FS reader live in
// statusgen/evaluation (outcome.go) — the ONE owner of how an outcome record is read
// (tool-refactoring/05). What remains here is the source adapter: it names a filesystem root
// and a wall clock and hands them to the shared reader as an io/fs port and a parameter.
// TestVerifyOutcomesSingleReader pins that this module has exactly one place that opens the
// verify-outcomes path, and TestOutcomeProtocolClassGuard pins that no second copy of the
// protocol's rules reappears.

// outcomeFS maps a checkout root to the read port the shared reader takes. The empty root means
// the current directory, as filepath.Join("", ...) always did.
func outcomeFS(root string) fs.FS {
	if root == "" {
		root = "."
	}
	return os.DirFS(root)
}

// readVerifyOutcomeRecords reads every verify-outcome record under root (see
// evaluation.ReadRecords for the three-state contract).
func readVerifyOutcomeRecords(root string) ([]evaluation.Record, error) {
	records, _, err := readVerifyOutcomeRecordsSkips(root)
	return records, err
}

// readVerifyOutcomeRecordsSkips is readVerifyOutcomeRecords that also returns every legacy log
// line the legacy tolerance skipped, so a gate can refuse to read past one (#6734).
func readVerifyOutcomeRecordsSkips(root string) ([]evaluation.Record, []evaluation.Skipped, error) {
	return evaluation.ReadRecords(outcomeFS(root), root)
}

// latestOutcomePerBrief reduces records to one winner per brief key at the wall clock now.
func latestOutcomePerBrief(records []evaluation.Record) (map[string]evaluation.Record, map[string]bool) {
	return evaluation.LatestPerBrief(records, time.Now())
}
