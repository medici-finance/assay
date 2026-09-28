package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
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
const verifyOutcomesGlob = "verify-outcomes*.jsonl"

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

// --- #882 per-file verify-outcome record layer — statusgen's OWN copy ----------------------
//
// tools/desk/internal/deskkit carries the desk-side twin of everything below
// (RecordName/ParseRecord/ReadVerifyOutcomes/LatestPerBrief). statusgen is a SEPARATE Go
// module (this file's own header explains why: it cannot import tools/desk) so it keeps an
// independent copy of the pure record-naming/reading rules rather than sharing the code — the
// two are kept byte-identical BY HAND, and TestVerifyOutcomesSingleReader in each module pins
// that its own module has exactly one place that opens this path.

// outcomeRecordsDir is the repo-relative directory verify-outcome records live under, one file
// per outcome, per-stream subdirectory (docs/streams/verify-outcomes/<stream>/*.json).
const outcomeRecordsDir = "docs/streams/verify-outcomes"

var (
	outcomeStreamRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	outcomeNumRe    = regexp.MustCompile(`^[0-9]+$`)
)

// outcomeRecord is one verify outcome, read generically enough that a caller can unmarshal Raw
// into its own richer type. Raw is the CANONICAL line — the record's single JSON object,
// trimmed of surrounding whitespace, with NO trailing newline.
type outcomeRecord struct {
	Brief  string
	TS     string
	Raw    []byte
	Name   string // outcomeRecordName's basename for this record's bytes
	Source string // "record:<path>" or "legacy:<path>", diagnostics only
	Digest string
}

// parseOutcomeRecord parses one verify-outcome record's raw bytes and returns it with Raw
// holding the canonical line. A record with no `brief` key, or bytes that are not one JSON
// object, is refused.
func parseOutcomeRecord(raw []byte) (outcomeRecord, error) {
	line := bytes.TrimSpace(raw)
	if len(line) == 0 {
		return outcomeRecord{}, errors.New("empty verify-outcome record")
	}
	var head struct {
		Brief string `json:"brief"`
		TS    string `json:"ts"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return outcomeRecord{}, fmt.Errorf("invalid verify-outcome record JSON: %w", err)
	}
	if strings.TrimSpace(head.Brief) == "" {
		return outcomeRecord{}, errors.New("verify-outcome record has no brief key")
	}
	canon := append([]byte{}, line...)
	return outcomeRecord{Brief: head.Brief, TS: head.TS, Raw: canon, Digest: outcomeRecordDigest(canon)}, nil
}

// outcomeRecordDigest is the first 12 hex digits of the SHA-256 of canonicalLine plus a
// trailing newline — the record file's exact on-disk bytes.
func outcomeRecordDigest(canonicalLine []byte) string {
	buf := make([]byte, 0, len(canonicalLine)+1)
	buf = append(buf, canonicalLine...)
	buf = append(buf, '\n')
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])[:12]
}

// outcomeCanonicalBytes returns the exact bytes a record file holds for canonicalLine.
func outcomeCanonicalBytes(canonicalLine []byte) []byte {
	out := make([]byte, 0, len(canonicalLine)+1)
	out = append(out, canonicalLine...)
	return append(out, '\n')
}

// splitOutcomeBriefKey splits a "<stream>/<NN>" brief key, refusing (never sanitising) any
// other shape.
func splitOutcomeBriefKey(brief string) (stream, num string, err error) {
	parts := strings.SplitN(brief, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid brief key %q: want <stream>/<NN>", brief)
	}
	stream, num = parts[0], parts[1]
	if !outcomeStreamRe.MatchString(stream) {
		return "", "", fmt.Errorf("invalid brief stream %q in key %q", stream, brief)
	}
	if !outcomeNumRe.MatchString(num) {
		return "", "", fmt.Errorf("invalid brief number %q in key %q", num, brief)
	}
	return stream, num, nil
}

// outcomeRecordName computes the repo-relative path a verify-outcome record's bytes land at:
// docs/streams/verify-outcomes/<stream>/<NN>-<YYYYMMDDTHHMMSSZ>-<digest12>.json. A pure
// function of recordBytes — see deskkit.RecordName's doc for the full rationale (collision-free
// across concurrent PRs).
func outcomeRecordName(recordBytes []byte) (string, error) {
	rec, err := parseOutcomeRecord(recordBytes)
	if err != nil {
		return "", err
	}
	stream, num, err := splitOutcomeBriefKey(rec.Brief)
	if err != nil {
		return "", err
	}
	ts, err := time.Parse(time.RFC3339, rec.TS)
	if err != nil {
		return "", fmt.Errorf("invalid verify-outcome record ts %q for brief %s: %w", rec.TS, rec.Brief, err)
	}
	compact := ts.UTC().Format("20060102T150405Z")
	return path.Join(outcomeRecordsDir, stream, fmt.Sprintf("%s-%s-%s.json", num, compact, rec.Digest)), nil
}

// readVerifyOutcomeRecords reads every verify-outcome record under root: every per-file record
// under docs/streams/verify-outcomes/<stream>/*.json AND every line of any legacy
// docs/streams/verify-outcomes*.jsonl, deduped by canonical-bytes digest. An absent records
// directory and an absent legacy log together are an empty set, never an error; an unreadable
// file is an error (could-not-check), never a skipped record — the legacy log keeps its
// pre-existing tolerance for a malformed LINE (skipped) but not for an unreadable FILE.
func readVerifyOutcomeRecords(root string) ([]outcomeRecord, error) {
	var records []outcomeRecord
	seen := map[string]bool{}

	recordsDir := filepath.Join(root, filepath.FromSlash(outcomeRecordsDir))
	streamDirs, err := os.ReadDir(recordsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("could-not-check: cannot read %s: %w", recordsDir, err)
		}
		streamDirs = nil
	}
	sort.Slice(streamDirs, func(i, j int) bool { return streamDirs[i].Name() < streamDirs[j].Name() })
	for _, sd := range streamDirs {
		if !sd.IsDir() {
			continue
		}
		streamDir := filepath.Join(recordsDir, sd.Name())
		files, ferr := os.ReadDir(streamDir)
		if ferr != nil {
			return nil, fmt.Errorf("could-not-check: cannot read %s: %w", streamDir, ferr)
		}
		names := make([]string, 0, len(files))
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
				names = append(names, f.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			p := filepath.Join(streamDir, name)
			raw, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil, fmt.Errorf("could-not-check: cannot read record %s: %w", p, rerr)
			}
			rec, perr := parseOutcomeRecord(raw)
			if perr != nil {
				return nil, fmt.Errorf("could-not-check: malformed verify-outcome record %s: %w", p, perr)
			}
			rec.Name = name
			rec.Source = "record:" + filepath.ToSlash(p)
			if !seen[rec.Digest] {
				seen[rec.Digest] = true
				records = append(records, rec)
			}
		}
	}

	matches, gerr := verifyOutcomesShardPaths(root)
	if gerr != nil {
		return nil, gerr
	}
	for _, p := range matches {
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				continue
			}
			return nil, fmt.Errorf("could-not-check: cannot read %s: %w", p, rerr)
		}
		for _, line := range bytes.Split(raw, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			rec, perr := parseOutcomeRecord(line)
			if perr != nil {
				continue // legacy tolerance: a malformed line is skipped, never fatal
			}
			if name, nerr := outcomeRecordName(line); nerr == nil {
				rec.Name = filepath.Base(name)
			} else {
				continue
			}
			rec.Source = "legacy:" + filepath.ToSlash(p)
			if !seen[rec.Digest] {
				seen[rec.Digest] = true
				records = append(records, rec)
			}
		}
	}

	return records, nil
}

// maxClockSkew mirrors deskkit.MaxClockSkew (#1803 SR-1803-2) — kept in sync by hand, like every
// other pure rule in this file's own copy of the record layer (see the header comment).
const maxClockSkew = 5 * time.Minute

// futureTS mirrors deskkit.FutureTS.
func futureTS(ts string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	return t.After(now.Add(maxClockSkew))
}

// latestOutcomePerBrief reduces records to one winner per brief key: the newest `ts` wins, ties
// broken by the lexically greatest record name. A record whose `ts` is more than maxClockSkew
// ahead of now is never a candidate winner (#1803 SR-1803-2): it is excluded from the reduction
// and its brief key is reported in the second return value.
func latestOutcomePerBrief(records []outcomeRecord) (latest map[string]outcomeRecord, futureByBrief map[string]bool) {
	return latestOutcomePerBriefAt(records, time.Now())
}

// latestOutcomePerBriefAt is latestOutcomePerBrief with an explicit "now", for tests.
func latestOutcomePerBriefAt(records []outcomeRecord, now time.Time) (latest map[string]outcomeRecord, futureByBrief map[string]bool) {
	out := make(map[string]outcomeRecord, len(records))
	future := map[string]bool{}
	tsOf := func(r outcomeRecord) time.Time {
		t, err := time.Parse(time.RFC3339, r.TS)
		if err != nil {
			return time.Time{}
		}
		return t
	}
	for _, rec := range records {
		if futureTS(rec.TS, now) {
			future[rec.Brief] = true
			continue
		}
		cur, ok := out[rec.Brief]
		if !ok {
			out[rec.Brief] = rec
			continue
		}
		rt, ct := tsOf(rec), tsOf(cur)
		switch {
		case rt.After(ct):
			out[rec.Brief] = rec
		case rt.Equal(ct) && rec.Name > cur.Name:
			out[rec.Brief] = rec
		}
	}
	return out, future
}
