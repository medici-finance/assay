// Package outcomerecord is the verify-outcome record codec: it parses a record's bytes, names
// and digests it, picks the latest record per brief and reads both on-disk layouts through an
// io/fs.FS port. It is the decoding half of the outcome protocol whose meaning (vocabulary,
// disposition, row and run reduction, brief-key grammar) is owned by the pure evaluation
// library, statusgen/evaluation/outcome.go. It lives in its own package because decoding needs
// encoding/json, io/fs and time, which the evaluation library's dependency boundary excludes.
//
// It opens no file by path, runs no process and reads no clock or environment: the caller hands
// it an fs.FS and an explicit "now".
package outcomerecord

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// --- record codec, naming and per-brief reduction ----------------------------------------

// OutcomeRecordsDir is the repo-relative directory verify-outcome records live under, one file
// per outcome, per-stream subdirectory (docs/streams/verify-outcomes/<stream>/*.json).
const OutcomeRecordsDir = "docs/streams/verify-outcomes"

// MaxClockSkew bounds how far ahead of "now" a record's `ts` may be before it is untrustworthy
// rather than merely the newest (#1803 SR-1803-2).
const MaxClockSkew = 5 * time.Minute

// Record is one verify outcome, read generically enough that a caller can unmarshal Raw into its
// own richer type. Raw is the CANONICAL line: the record's single JSON object, trimmed of
// surrounding whitespace, with NO trailing newline.
type Record struct {
	Brief  string // the "brief" field: "<stream>/<NN>"
	TS     string // the "ts" field, RFC 3339
	Raw    []byte // the canonical JSON line
	Name   string // RecordName's basename for this record's bytes (the tie-break key)
	Source string // "record:<path>" or "legacy:<path>", diagnostics only
	Digest string // RecordDigest(Raw)
	// Path is a per-file record's repo-relative slash path, the place it was READ from; empty for
	// a legacy log line.
	Path string
}

// Skipped is a legacy log line the legacy tolerance dropped (a malformed line, or one with no
// computable record name). A reader that may not inherit that tolerance judges these itself.
type Skipped struct {
	Source string
	Line   []byte
}

// ParseRecord parses one record's raw bytes (a record file's content or one legacy log line)
// and returns it with Raw holding the canonical line. A record with no `brief` key, or bytes
// that are not one JSON object, is refused.
func ParseRecord(raw []byte) (Record, error) {
	line := bytes.TrimSpace(raw)
	if len(line) == 0 {
		return Record{}, errors.New("empty verify-outcome record")
	}
	var head struct {
		Brief string `json:"brief"`
		TS    string `json:"ts"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return Record{}, fmt.Errorf("invalid verify-outcome record JSON: %w", err)
	}
	if strings.TrimSpace(head.Brief) == "" {
		return Record{}, errors.New("verify-outcome record has no brief key")
	}
	canon := append([]byte{}, line...)
	return Record{Brief: head.Brief, TS: head.TS, Raw: canon, Digest: RecordDigest(canon)}, nil
}

// RecordDigest is the first 12 hex digits of the SHA-256 of canonicalLine plus a trailing
// newline: the record file's exact on-disk bytes.
func RecordDigest(canonicalLine []byte) string {
	sum := sha256.Sum256(CanonicalBytes(canonicalLine))
	return hex.EncodeToString(sum[:])[:12]
}

// CanonicalBytes returns the exact bytes a record file holds for canonicalLine.
func CanonicalBytes(canonicalLine []byte) []byte {
	out := make([]byte, 0, len(canonicalLine)+1)
	out = append(out, canonicalLine...)
	return append(out, '\n')
}

// RecordName computes the repo-relative path a record's bytes land at:
//
//	docs/streams/verify-outcomes/<stream>/<NN>-<YYYYMMDDTHHMMSSZ>-<digest12>.json
//
// A pure function of the bytes, so two concurrent landings only share a path when they carry
// byte-identical content.
func RecordName(recordBytes []byte) (string, error) {
	rec, err := ParseRecord(recordBytes)
	if err != nil {
		return "", err
	}
	stream, num, err := evaluation.SplitBriefKey(rec.Brief)
	if err != nil {
		return "", err
	}
	ts, err := time.Parse(time.RFC3339, rec.TS)
	if err != nil {
		return "", fmt.Errorf("invalid verify-outcome record ts %q for brief %s: %w", rec.TS, rec.Brief, err)
	}
	compact := ts.UTC().Format("20060102T150405Z")
	return path.Join(OutcomeRecordsDir, stream, fmt.Sprintf("%s-%s-%s.json", num, compact, rec.Digest)), nil
}

// FutureTS reports whether ts lands more than MaxClockSkew ahead of now. An unparsable ts is
// never "future": the reduction already sorts it as the oldest possible instant.
func FutureTS(ts string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	return t.After(now.Add(MaxClockSkew))
}

// LatestPerBrief reduces records to one winner per brief key: the newest `ts` wins; ties are
// broken by the lexically greatest record NAME, never by line position or slice order. A record
// whose ts is more than MaxClockSkew ahead of now is never a candidate: it is excluded and its
// brief key reported in the second result so the caller can say could-not-check.
func LatestPerBrief(records []Record, now time.Time) (latest map[string]Record, futureByBrief map[string]bool) {
	out := make(map[string]Record, len(records))
	future := map[string]bool{}
	tsOf := func(r Record) time.Time {
		t, err := time.Parse(time.RFC3339, r.TS)
		if err != nil {
			return time.Time{}
		}
		return t
	}
	for _, rec := range records {
		if FutureTS(rec.TS, now) {
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

// ReadRecords reads every verify-outcome record under the repository root fsys serves: every
// per-file record under docs/streams/verify-outcomes/<stream>/*.json AND every line of any
// legacy docs/streams/verify-outcomes*.jsonl, deduped by canonical-bytes digest. label is the
// path prefix diagnostics (Source, error text) are rendered with; it never affects what is read.
//
// Three-state: an absent records directory and an absent legacy log together are an empty set; an
// unreadable file or directory is an error (could-not-check), never a skipped record. The legacy
// log keeps its tolerance for a malformed LINE (returned in the skipped list), not for an
// unreadable FILE.
func ReadRecords(fsys fs.FS, label string) ([]Record, []Skipped, error) {
	var records []Record
	var skipped []Skipped
	seen := map[string]bool{}
	shown := func(rel string) string { return path.Join(label, rel) }

	streamDirs, err := fs.ReadDir(fsys, OutcomeRecordsDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("could-not-check: cannot read %s: %w", shown(OutcomeRecordsDir), err)
		}
		streamDirs = nil
	}
	sort.Slice(streamDirs, func(i, j int) bool { return streamDirs[i].Name() < streamDirs[j].Name() })
	for _, sd := range streamDirs {
		if !sd.IsDir() {
			continue
		}
		streamDir := path.Join(OutcomeRecordsDir, sd.Name())
		files, ferr := fs.ReadDir(fsys, streamDir)
		if ferr != nil {
			return nil, nil, fmt.Errorf("could-not-check: cannot read %s: %w", shown(streamDir), ferr)
		}
		names := make([]string, 0, len(files))
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
				names = append(names, f.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			rel := path.Join(streamDir, name)
			raw, rerr := fs.ReadFile(fsys, rel)
			if rerr != nil {
				return nil, nil, fmt.Errorf("could-not-check: cannot read record %s: %w", shown(rel), rerr)
			}
			rec, perr := ParseRecord(raw)
			if perr != nil {
				return nil, nil, fmt.Errorf("could-not-check: malformed verify-outcome record %s: %w", shown(rel), perr)
			}
			rec.Name = name
			rec.Source = "record:" + shown(rel)
			rec.Path = rel
			if !seen[rec.Digest] {
				seen[rec.Digest] = true
				records = append(records, rec)
			}
		}
	}

	matches, gerr := fs.Glob(fsys, path.Join("docs", "streams", evaluation.VerifyOutcomesGlob))
	if gerr != nil {
		return nil, nil, fmt.Errorf("could-not-check: bad legacy verify-outcomes glob: %w", gerr)
	}
	sort.Strings(matches)
	for _, rel := range matches {
		raw, rerr := fs.ReadFile(fsys, rel)
		if rerr != nil {
			if errors.Is(rerr, fs.ErrNotExist) {
				continue
			}
			return nil, nil, fmt.Errorf("could-not-check: cannot read %s: %w", shown(rel), rerr)
		}
		for _, line := range bytes.Split(raw, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			skip := func() {
				skipped = append(skipped, Skipped{Source: "legacy:" + shown(rel), Line: append([]byte{}, line...)})
			}
			rec, perr := ParseRecord(line)
			if perr != nil {
				skip() // legacy tolerance: a malformed line is skipped, never fatal
				continue
			}
			name, nerr := RecordName(line)
			if nerr != nil {
				skip() // legacy tolerance: a line RecordName cannot name is skipped, never fatal
				continue
			}
			rec.Name = path.Base(name)
			rec.Source = "legacy:" + shown(rel)
			if !seen[rec.Digest] {
				seen[rec.Digest] = true
				records = append(records, rec)
			}
		}
	}
	return records, skipped, nil
}
