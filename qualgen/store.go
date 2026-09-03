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
	metricsTable = "metrics.jsonl"
	// defectsTable is the M2 defect-lineage table (spec §9.4): quality/06
	// appends DefectFix records here; quality/07's B-SZZ trace reads them.
	// quality/05, when it lands, is expected to fold this path into its
	// formalized artifacts.go schema module — this brief pioneers the table
	// under the Store's existing append-only convention rather than block on
	// a schema module that does not yet exist (dependency-wave ordering puts
	// quality/06 in wave 1, ahead of quality/05's wave 2).
	defectsTable = "defects.jsonl"

	// sweepSubdir is the code-slop forensic sweep lane's own subdirectory under
	// the quality dir (spec §3.1/§9.4 committed-artifact model; quality/16). Its
	// two append-only tables live here, kept out of the M1–M4 history-mining
	// tables' namespace since the sweep reads the CURRENT tree, not history.
	sweepSubdir   = "sweep"
	suspectsTable = "suspects.jsonl"
	verdictsTable = "verdicts.jsonl"
)

// Kind selects which append-only table Append writes to.
type Kind string

const (
	KindCommit Kind = "commits"
	KindDiff   Kind = "diffs"
	// KindMetric is the M1 aggregate table (spec §9.4): heterogeneous
	// records — one shape per metric family (hotspot, ownership, coupling,
	// missing-coupling-partner, ...) — each discriminated by its own
	// "metric" field. quality/05 is expected to formalize this into a
	// dedicated schema module (artifacts.go); until then, families append
	// here through the same generic Store.Append seam quality/01 shipped.
	KindMetric Kind = "metrics"
	KindDefect Kind = "defects"
	// KindSweepSuspect / KindSweepVerdict are the sweep lane's two append-only
	// tables (quality/16), written through the SAME Store.Append seam quality/01
	// shipped — the lane extends the store, it does not re-invent artifact
	// plumbing. Both are append-once-per-fingerprint: a suspect fingerprint is
	// appended the first run it appears, a verdict the run it is adjudicated, so
	// a rerun over an unchanged tree appends nothing.
	KindSweepSuspect Kind = "sweep-suspects"
	KindSweepVerdict Kind = "sweep-verdicts"
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
	case KindMetric:
		return filepath.Join(s.dir(), metricsTable), nil
	case KindDefect:
		return filepath.Join(s.dir(), defectsTable), nil
	case KindSweepSuspect:
		return filepath.Join(s.dir(), sweepSubdir, suspectsTable), nil
	case KindSweepVerdict:
		return filepath.Join(s.dir(), sweepSubdir, verdictsTable), nil
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
	// Tables may live in a subdirectory of the quality dir (the sweep lane's
	// tables do); ensure the immediate parent exists so a first append to a
	// nested table does not fail. ensureDir above covers the top-level tables.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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

// StreamDefects streams the defects table's DefectFix records (quality/06) back as
// typed records. The defects table is HETEROGENEOUS (spec §9.4): quality/06 seeds
// it with DefectFix rows and quality/07's B-SZZ trace appends DefectTrace rows to
// the same append-only file. A DefectTrace line decodes here into a DefectFix whose
// Identified state is empty (it carries no `identified` field); such a line is
// skipped so this reader returns ONLY genuine DefectFix rows — mirroring how
// StreamMetrics filters the heterogeneous metrics table by metric name.
func (s *Store) StreamDefects(fn func(DefectFix) error) error {
	path, _ := s.tablePath(KindDefect)
	return streamJSONL(path, func(d DefectFix) error {
		if d.Identified.State == "" {
			return nil // a DefectTrace row (or any non-DefectFix line): not ours
		}
		return fn(d)
	})
}

// ReadDefects collects the whole defects table's DefectFix records.
func (s *Store) ReadDefects() ([]DefectFix, error) {
	var out []DefectFix
	err := s.StreamDefects(func(d DefectFix) error {
		out = append(out, d)
		return nil
	})
	return out, err
}

// StreamTraces streams the defects table's DefectTrace records (quality/07's B-SZZ
// traces) back as typed records. The counterpart to StreamDefects on the same
// heterogeneous table: a DefectFix line decodes here into a DefectTrace whose
// TraceState is empty and is skipped, so this reader returns ONLY genuine
// DefectTrace rows.
func (s *Store) StreamTraces(fn func(DefectTrace) error) error {
	path, _ := s.tablePath(KindDefect)
	return streamJSONL(path, func(t DefectTrace) error {
		if t.TraceState == "" {
			return nil // a DefectFix row (or any non-DefectTrace line): not ours
		}
		return fn(t)
	})
}

// ReadTraces collects the whole defects table's DefectTrace records.
func (s *Store) ReadTraces() ([]DefectTrace, error) {
	var out []DefectTrace
	err := s.StreamTraces(func(t DefectTrace) error {
		out = append(out, t)
		return nil
	})
	return out, err
}

// StreamMetrics streams the metrics table back through the generic MetricRecord
// view (the M1 line-taxonomy / churn family this brief emits). The metrics table
// is heterogeneous — the hotspot / ownership / coupling families (quality/03)
// write their own record shapes to the same append-only table — so a line that
// is not a MetricRecord decodes with an empty Metric and is filtered by metric
// name at the call site. Like the other raw tables, metrics is append-only: each
// mine appends a fresh full snapshot (extend, never rewrite), so a trend consumer
// reads the most recent snapshot per metric.
func (s *Store) StreamMetrics(fn func(MetricRecord) error) error {
	path, _ := s.tablePath(KindMetric)
	return streamJSONL(path, fn)
}

// ReadMetrics collects the whole metrics table through the MetricRecord view.
func (s *Store) ReadMetrics() ([]MetricRecord, error) {
	var out []MetricRecord
	err := s.StreamMetrics(func(m MetricRecord) error {
		out = append(out, m)
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
