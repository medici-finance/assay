package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// artifacts.go formalizes the committed-artifact CONTRACT for the three families
// every quality layer writes into and the report view reads back (spec §9.4):
//
//   - docs/quality/metrics.jsonl   — M1 aggregates (append-only; MetricRecord +
//     the sibling hotspot/ownership/coupling family records, each discriminated
//     by its own "metric" field). Written by quality/02–03; schema frozen in
//     m1agg.go / hotspot.go / ownership.go / coupling.go.
//   - docs/quality/defects.jsonl   — M2 defect-lineage traces (append-only;
//     DefectFix records carrying evidence tier + a three-state confidence).
//     Populated by quality/06–07; the record shape is DECLARED here.
//   - docs/quality/attribution/    — M3 per-defect dossiers, one file per defect
//     with tombstone amendments (never a silent edit). Populated by quality/10;
//     the record shape is DECLARED here.
//
// This brief (quality/05) computes NO new metric. It (a) declares the two
// not-yet-populated shapes so quality/06–07 and /10 have a frozen target, and
// (b) supplies the industry-baseline table the report view renders beside every
// local number under the honest-claims discipline (spec §10).

// ---------------------------------------------------------------------------
// Industry-comparable baselines (spec §9.3, §10)
// ---------------------------------------------------------------------------

// Baseline is one published industry reference for a metric. Value is a
// three-state Measure so a metric with NO directly comparable published figure
// renders as could-not-measure (with the reason stating why) rather than a
// fabricated number — the same three-state honesty the local numbers carry.
//
// Source names the published methodology; WindowNote states any window/threshold
// difference between the local computation and the published definition, which
// spec §10 requires whenever they differ ("where windows/thresholds differ the
// artifact states both"). A baseline is NEVER labelled a "GitClear-equivalent"
// value — only "computed per GitClear's published definitions" (honestClaimsNote).
type Baseline struct {
	Value      Measure[float64] `json:"value"`
	Source     string           `json:"source"`
	WindowNote string           `json:"window_note,omitempty"`
}

// BaselineSet maps a metric name (the MetricXxx constants) to its published
// baseline. It is loaded from an operator/CI-supplied JSON file (--baselines) or
// falls back to the built-in set. Keeping baselines DATA — not hard-coded render
// strings — is what lets a reviewer vet or update a single external figure
// without touching the renderer.
type BaselineSet map[string]Baseline

// BuiltinBaselines is the honest built-in reference set.
//
// The ONLY figure pinned as measured is code churn, because GitClear's churn
// definition (code revised/deleted within a ~2-week window) matches this tool's
// default churn window (DefaultChurnWindowDays = 14) EXACTLY, so the two are
// directly comparable. Copy/paste ratio and duplicate-block rate are pinned as
// could-not-measure: GitClear publishes copy/paste as a share of changed lines,
// which is not the copied/(moved+copied) block ratio computed here — the
// definitions differ, so no directly comparable published figure is asserted.
// An operator with a vetted, definition-matched figure supplies it via
// --baselines; the render then shows it beside the local number automatically.
func BuiltinBaselines() BaselineSet {
	return BaselineSet{
		MetricChurnRate: {
			Value:      Measured(0.07),
			Source:     "GitClear code churn (published ~2-week window; 2024 report projection)",
			WindowNote: "local churn window is 14 days = GitClear's 2-week window — directly comparable",
		},
		MetricCopyPasteRatio: {
			Value: CouldNotMeasure[float64](
				"GitClear publishes copy/paste as a share of changed lines; the local metric is copied/(moved+copied) over blocks — definitions differ, no directly comparable published figure pinned"),
			Source: "GitClear published definitions",
		},
		MetricDuplicateBlockRate: {
			Value: CouldNotMeasure[float64](
				"no directly comparable published block-level duplicate-rate figure pinned in this release"),
			Source: "GitClear published definitions",
		},
	}
}

// LoadBaselines reads a baseline set from a JSON file. An empty path returns the
// built-in set. The file is a JSON object of the same shape as BaselineSet; its
// entries OVERRIDE the built-in ones metric-by-metric (an operator can pin a
// vetted copy/paste figure without discarding the built-in churn baseline).
func LoadBaselines(path string) (BaselineSet, error) {
	set := BuiltinBaselines()
	if path == "" {
		return set, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("qualgen report: read baselines %s: %w", path, err)
	}
	var loaded BaselineSet
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("qualgen report: parse baselines %s: %w", path, err)
	}
	for metric, b := range loaded {
		set[metric] = b
	}
	return set, nil
}

// ---------------------------------------------------------------------------
// M2 defects.jsonl — shape declared here, populated by quality/06–07
// ---------------------------------------------------------------------------
//
// The record itself is DefectFix (fixlinkage.go), extended by this brief with a
// three-state Confidence field so the append-only defects table carries both the
// evidence TIER (which classifier rung matched) and a CONFIDENCE (how strongly
// the trace holds), the two fields Verify #8 pins. Confidence is a pointer so an
// unpopulated record (quality/06 emits tier without a confidence yet) omits it
// entirely and round-trips byte-for-byte, exactly as before this brief.

// ---------------------------------------------------------------------------
// M3 attribution/ — one file per defect, tombstone amendments (spec §9.4)
// ---------------------------------------------------------------------------

// attributionSubdir is the per-defect dossier directory under the tracking
// root's quality dir (docs/quality/attribution/). One file per defect, so two
// defects never contend on one append-only line and a dossier can be amended
// in place without rewriting a shared table.
const attributionSubdir = "attribution"

// attributionSchemaVersion pins the dossier layout so a later reader detects a
// stale shape rather than mis-parsing it.
const attributionSchemaVersion = "qualgen-attribution-v1"

// AttributionRecord is one M3 per-defect stage-attribution dossier (spec §9.4,
// filled by quality/10). It attributes a defect to the pipeline STAGE that
// introduced it, with a three-state confidence. Both Stage and Confidence are
// three-state Measures: a defect whose introducing stage cannot be determined
// serializes as could-not-measure, never as a fabricated stage.
//
// Amendments are the tombstone trail (spec §9.4: "tombstone amendments, never
// silent edits"): a correction APPENDS an Amendment recording the old→new
// change with its reason and time; the base fields are updated in the same
// write, so the current value and its full revision history both live in the
// one file. AmendAttribution is the only path that mutates an existing dossier.
type AttributionRecord struct {
	DefectID          string           `json:"defect_id"`
	FixCommitSHA      string           `json:"fix_commit_sha"`
	InducingCommitSHA string           `json:"inducing_commit_sha,omitempty"`
	Stage             Measure[string]  `json:"stage"`
	Confidence        Measure[float64] `json:"confidence"`
	SchemaVersion     string           `json:"schema_version"`
	Amendments        []Amendment      `json:"amendments,omitempty"`
}

// Amendment is one tombstoned correction to an AttributionRecord.
type Amendment struct {
	AmendedAt time.Time `json:"amended_at"`
	Reason    string    `json:"reason"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
}

func (s *Store) attributionDir() string { return filepath.Join(s.dir(), attributionSubdir) }

// attributionIDPattern constrains a defect id to filesystem-safe characters, so
// the id can never escape the attribution dir (a "../" id, a path separator) or
// collide across case-folding filesystems in a surprising way.
var attributionIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func (s *Store) attributionPath(defectID string) (string, error) {
	if !attributionIDPattern.MatchString(defectID) {
		return "", fmt.Errorf("qualgen: unsafe attribution defect id %q (allowed: letters, digits, dot, dash, underscore)", defectID)
	}
	return filepath.Join(s.attributionDir(), defectID+".json"), nil
}

// WriteAttribution creates a new per-defect dossier. It REFUSES to overwrite an
// existing one — a correction goes through AmendAttribution, which records a
// tombstone — so a dossier can never be silently rewritten (spec §9.4).
func (s *Store) WriteAttribution(rec AttributionRecord) error {
	path, err := s.attributionPath(rec.DefectID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.attributionDir(), 0o755); err != nil {
		return err
	}
	rec.SchemaVersion = attributionSchemaVersion
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	// O_EXCL: fail rather than clobber an existing dossier.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("qualgen: attribution dossier for %q already exists; corrections go through AmendAttribution (tombstone, never silent overwrite)", rec.DefectID)
		}
		return err
	}
	defer f.Close()
	_, err = f.Write(append(raw, '\n'))
	return err
}

// ReadAttribution reads one per-defect dossier. A missing dossier returns
// (nil, nil): the defect has no M3 attribution yet.
func (s *Store) ReadAttribution(defectID string) (*AttributionRecord, error) {
	path, err := s.attributionPath(defectID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rec AttributionRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("qualgen: attribution %s: %w", defectID, err)
	}
	return &rec, nil
}

// AmendAttribution applies a tombstoned correction to an existing dossier: it
// reads the current record, lets apply mutate the base fields, appends am to the
// amendment trail, and rewrites the file. It is the ONLY path that mutates an
// existing dossier, which is what keeps every change tombstoned.
func (s *Store) AmendAttribution(defectID string, am Amendment, apply func(*AttributionRecord)) error {
	rec, err := s.ReadAttribution(defectID)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("qualgen: no attribution dossier for %q to amend", defectID)
	}
	if apply != nil {
		apply(rec)
	}
	if am.AmendedAt.IsZero() {
		am.AmendedAt = time.Now().UTC()
	}
	rec.Amendments = append(rec.Amendments, am)
	rec.SchemaVersion = attributionSchemaVersion
	path, err := s.attributionPath(defectID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// ListAttribution returns the defect ids of every dossier under the tracking
// root, sorted for a diffable artifact. A missing directory lists nothing (no
// M3 attribution has been written yet), not an error.
func (s *Store) ListAttribution() ([]string, error) {
	entries, err := os.ReadDir(s.attributionDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".json" {
			continue
		}
		ids = append(ids, name[:len(name)-len(".json")])
	}
	sort.Strings(ids)
	return ids, nil
}

// ---------------------------------------------------------------------------
// Family readers — the heterogeneous metrics.jsonl table, decoded per family
// ---------------------------------------------------------------------------

// metricName is a cheap pre-decode of only the discriminator field, so a line
// belonging to a different family is skipped without a failed full-decode into
// the wrong struct.
type metricName struct {
	Metric string `json:"metric"`
}

// readFamily streams metrics.jsonl and full-decodes only the lines whose
// "metric" discriminator is in want, into T. It is the typed lens each report
// section reads its family through; a non-matching line is skipped, never an
// error, because the table is deliberately heterogeneous.
func readFamily[T any](s *Store, want map[string]bool) ([]T, error) {
	path, _ := s.tablePath(KindMetric)
	var out []T
	err := streamRawJSONL(path, func(raw []byte) error {
		var nm metricName
		if err := json.Unmarshal(raw, &nm); err != nil {
			return err
		}
		if !want[nm.Metric] {
			return nil
		}
		var rec T
		if err := json.Unmarshal(raw, &rec); err != nil {
			return err
		}
		out = append(out, rec)
		return nil
	})
	return out, err
}

// ReadHotspots reads the hotspot family from the latest mine snapshot onward
// (all snapshots; the caller picks the latest by MinedAt).
func (s *Store) ReadHotspots() ([]HotspotRecord, error) {
	return readFamily[HotspotRecord](s, map[string]bool{"hotspot": true})
}

// ReadOwnership reads the ownership family.
func (s *Store) ReadOwnership() ([]OwnershipRecord, error) {
	return readFamily[OwnershipRecord](s, map[string]bool{"ownership": true})
}

// ReadReferenceValidity reads the instruction reference-validity trend family
// (spec §4.6, quality/04) — one record per history window, plus the single
// could-not-measure marker an unconfigured mine emits.
func (s *Store) ReadReferenceValidity() ([]ReferenceValidityRecord, error) {
	return readFamily[ReferenceValidityRecord](s, map[string]bool{MetricReferenceValidity: true})
}

// ReadDocCodeStaleness reads the doc↔code co-change staleness family (spec §4.6).
func (s *Store) ReadDocCodeStaleness() ([]DocCodeStalenessRecord, error) {
	return readFamily[DocCodeStalenessRecord](s, map[string]bool{MetricDocCodeStaleness: true})
}

// streamRawJSONL streams a JSONL file's non-empty lines as raw bytes, one call
// to fn per line, with the same large-record scanner budget streamJSONL uses. A
// missing file streams nothing (an unmined root is empty, not an error). It is
// the untyped sibling of streamJSONL, used where the caller decodes each line
// into a family-specific type chosen from the line's own discriminator.
func streamRawJSONL(path string, fn func([]byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		// Copy: Scanner reuses its buffer across Scan calls, and fn full-decodes
		// lazily; hand it a stable slice.
		buf := make([]byte, len(raw))
		copy(buf, raw)
		if err := fn(buf); err != nil {
			return fmt.Errorf("qualgen: %s line %d: %w", filepath.Base(path), line, err)
		}
	}
	return sc.Err()
}
