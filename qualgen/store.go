package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Store is the single-writer artifact store (frozen interface contract item 4).
// It writes ONLY under the operator-chosen tracking root and never into the
// mined target repo (spec §3.1, Profile-B: no in-repo writes). quality/02 and
// quality/03 read the commit/diff tables through this Store and append their own
// aggregates through it; they never parse the JSONL by hand.
//
// The two tables (commits.jsonl, diffs.jsonl) are APPEND-ONLY: Append never
// rewrites a prior line, which is what makes incremental mining extend-never-
// replace (a second mine adds lines, it does not touch the ones already there).
// The header (mine.json) is a manifest, not a table, and is rewritten each run.
type Store struct {
	root string
}

// artifact paths relative to the tracking root (spec §9.4).
const (
	qualityDir   = "docs/quality"
	mineHeader   = "mine.json"
	commitsTable = "commits.jsonl"
	diffsTable   = "diffs.jsonl"
	// defectsTable is the M2 defect-lineage table (spec §9.4): quality/06
	// appends DefectFix records here; quality/07's B-SZZ trace reads them.
	// quality/05, when it lands, is expected to fold this path into its
	// formalized artifacts.go schema module — this brief pioneers the table
	// under the Store's existing append-only convention rather than block on
	// a schema module that does not yet exist (dependency-wave ordering puts
	// quality/06 in wave 1, ahead of quality/05's wave 2).
	defectsTable = "defects.jsonl"
)

// Kind selects which append-only table Append writes to.
type Kind string

const (
	KindCommit Kind = "commits"
	KindDiff   Kind = "diffs"
	KindDefect Kind = "defects"
)

// schemaVersion pins the artifact schema so a later reader can detect a stale
// layout rather than mis-parse it.
const schemaVersion = "qualgen-mine-v1"

// NewStore returns a Store rooted at the tracking root. It does not touch the
// filesystem until a write.
func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) dir() string        { return filepath.Join(s.root, qualityDir) }
func (s *Store) headerPath() string { return filepath.Join(s.dir(), mineHeader) }

func (s *Store) tablePath(k Kind) (string, error) {
	switch k {
	case KindCommit:
		return filepath.Join(s.dir(), commitsTable), nil
	case KindDiff:
		return filepath.Join(s.dir(), diffsTable), nil
	case KindDefect:
		return filepath.Join(s.dir(), defectsTable), nil
	default:
		return "", fmt.Errorf("qualgen: unknown table kind %q", k)
	}
}

// ensureDir creates the tracking-root quality directory if absent.
func (s *Store) ensureDir() error {
	return os.MkdirAll(s.dir(), 0o755)
}

// Append writes one record as a single JSONL line to the append-only table for
// kind. It opens the file O_APPEND so it can never rewrite a prior line, which
// is the byte-level guarantee the extend-never-replace invariant rests on.
func (s *Store) Append(kind Kind, record any) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	path, err := s.tablePath(kind)
	if err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// StreamCommits streams the commit table back as typed records, one call to fn
// per line. A malformed line stops the stream with an error rather than being
// skipped — a corrupt table is a loud failure, not a silent gap.
func (s *Store) StreamCommits(fn func(Commit) error) error {
	path, _ := s.tablePath(KindCommit)
	return streamJSONL(path, fn)
}

// StreamDiffs streams the diff table back as typed records.
func (s *Store) StreamDiffs(fn func(FileDiff) error) error {
	path, _ := s.tablePath(KindDiff)
	return streamJSONL(path, fn)
}

// StreamDefects streams the defects table (quality/06's DefectFix records)
// back as typed records.
func (s *Store) StreamDefects(fn func(DefectFix) error) error {
	path, _ := s.tablePath(KindDefect)
	return streamJSONL(path, fn)
}

// ReadDefects collects the whole defects table.
func (s *Store) ReadDefects() ([]DefectFix, error) {
	var out []DefectFix
	err := s.StreamDefects(func(d DefectFix) error {
		out = append(out, d)
		return nil
	})
	return out, err
}

// ReadCommits collects the whole commit table. Convenience over StreamCommits
// for callers that want a slice; large mines should prefer the streamer.
func (s *Store) ReadCommits() ([]Commit, error) {
	var out []Commit
	err := s.StreamCommits(func(c Commit) error {
		out = append(out, c)
		return nil
	})
	return out, err
}

// ReadDiffs collects the whole diff table.
func (s *Store) ReadDiffs() ([]FileDiff, error) {
	var out []FileDiff
	err := s.StreamDiffs(func(fd FileDiff) error {
		out = append(out, fd)
		return nil
	})
	return out, err
}

// streamJSONL is the shared typed-line reader. A missing file streams nothing
// (an unmined root is empty, not an error).
func streamJSONL[T any](path string, fn func(T) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	// JSONL records (a commit with many diffs, a diff with many lines) can be
	// large; raise the scanner's line cap well above the 64 KiB default.
	sc.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		var rec T
		if err := json.Unmarshal(raw, &rec); err != nil {
			return fmt.Errorf("qualgen: %s line %d: %w", filepath.Base(path), line, err)
		}
		if err := fn(rec); err != nil {
			return err
		}
	}
	return sc.Err()
}

// MineHeader is the header/manifest (mine.json). It records what the mine could
// and could not see, so a consumer knows the horizon and the coverage of the
// tables it is about to read (spec §3.1 records the horizon and discontinuities;
// §3.2 the per-state coverage).
type MineHeader struct {
	SchemaVersion string    `json:"schema_version"`
	MinedAt       time.Time `json:"mined_at"`

	// TipSHA is HEAD at mine time — the point incremental runs extend from.
	TipSHA string `json:"tip_sha"`
	// Horizon is the earliest reachable commit the mine could see; renames,
	// rewritten history and shallow clones floor it (spec §3.1).
	Horizon string `json:"horizon"`

	// Discontinuities are gaps that floor what backfill can see.
	Discontinuities []Discontinuity `json:"discontinuities"`

	// Coverage is the per-state count across all mined FileDiff line records
	// (spec §3.2) — how much of the history was measurable.
	Coverage Coverage `json:"coverage"`

	CommitCount int `json:"commit_count"`
	DiffCount   int `json:"diff_count"`
}

// Discontinuity names a gap in what the mine could reach.
type Discontinuity struct {
	Kind   string `json:"kind"` // e.g. "shallow-clone-floor", "rewritten-history", "rename-gap"
	Detail string `json:"detail"`
}

// Coverage counts FileDiff line records by their three-state outcome.
type Coverage struct {
	Measured        int `json:"measured"`
	MeasuredZero    int `json:"measured_zero"`
	CouldNotMeasure int `json:"could_not_measure"`
}

// ReadHeader reads mine.json. A missing header returns (nil, nil): the tracking
// root has never been mined, which is the signal mine uses to do a full run.
func (s *Store) ReadHeader() (*MineHeader, error) {
	f, err := os.Open(s.headerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var h MineHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, fmt.Errorf("qualgen: %s: %w", mineHeader, err)
	}
	return &h, nil
}

// WriteHeader writes mine.json, pretty-printed so it stays diffable in review
// (the committed-artifact model, spec §3). The header is a manifest, not an
// append-only table, so rewriting it each run is correct.
func (s *Store) WriteHeader(h MineHeader) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	h.SchemaVersion = schemaVersion
	raw, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.headerPath(), append(raw, '\n'), 0o644)
}
