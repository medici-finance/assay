package attribution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ledger.go — the per-stage defect ledger writer (task item 4). It writes one
// APPEND-ONLY file per defect under the tracking root (`docs/quality/attribution/`),
// records the defect's stage call, and rolls the corpus up by stage / stream /
// window. Corrections are made ONLY by TOMBSTONE amendment — a new file that
// supersedes a prior entry — never by editing or deleting the prior file (brief
// ground rule: artifacts are append-only; amend by tombstone).
//
// # Ledger schema (downstream seam consumed by brief-12, kept stable)
//
// LedgerEntry is the frozen record brief-12's gate-yield and ritual joins read. One
// entry = one attribution of one defect at a point in time. A correction appends a
// SECOND entry with Supersedes set to the prior entry's hash and Tombstone true; the
// prior entry's file is unchanged on disk, so the ledger is a full audit trail, not
// a mutable current-state store. Rollup collapses the trail to current state
// (superseded entries excluded) for the by-stage counts brief-12 consumes.

// LedgerEntry is one append-only attribution record for one defect. Its fields are
// the frozen interface brief-12 reads; changing them is a breaking change.
type LedgerEntry struct {
	// DefectID is the defect this entry attributes (Trace.DefectID()).
	DefectID string `json:"defect_id"`
	// Stream and Window scope the entry for the per-stream / per-window rollup.
	Stream string `json:"stream"`
	Window string `json:"window,omitempty"`

	// Stage and ReviewEscape are the attribution outputs; DossierHash pins the exact
	// dossier the stage was called from (spot-audit).
	Stage         Stage        `json:"stage"`
	ReviewEscape  ReviewEscape `json:"review_escape"`
	DossierHash   string       `json:"dossier_hash"`
	Rationale     string       `json:"rationale,omitempty"`
	ModelAssisted bool         `json:"model_assisted"`

	// RecordedAt stamps when the entry was written (RFC3339, UTC). It is the ONLY
	// non-deterministic field and is excluded from EntryHash so a re-attribution of
	// unchanged evidence hashes identically.
	RecordedAt string `json:"recorded_at"`

	// Tombstone marks a correction entry; Supersedes names the EntryHash of the
	// entry this one corrects (empty for an original). A superseded entry's file is
	// never touched.
	Tombstone  bool   `json:"tombstone,omitempty"`
	Supersedes string `json:"supersedes,omitempty"`
}

// EntryHash is the content address of an entry's ATTRIBUTION (everything but the
// timestamp and tombstone bookkeeping): DefectID, Stream, Window, Stage,
// ReviewEscape, DossierHash. Two entries with the same attribution hash identically,
// so a correction that changes nothing is detectable and RecordedAt never perturbs
// the address.
func (e LedgerEntry) EntryHash() string {
	key := struct {
		DefectID     string       `json:"defect_id"`
		Stream       string       `json:"stream"`
		Window       string       `json:"window"`
		Stage        Stage        `json:"stage"`
		ReviewEscape ReviewEscape `json:"review_escape"`
		DossierHash  string       `json:"dossier_hash"`
	}{e.DefectID, e.Stream, e.Window, e.Stage, e.ReviewEscape, e.DossierHash}
	return jsonHash(key)
}

// Ledger writes and reads attribution entries under a tracking root. Root is the
// operator-chosen tracking root (the miner's, e.g. a repo's `docs/quality`); entries
// land under Root/attribution/<stream>/.
type Ledger struct {
	Root string
}

// NewLedger returns a ledger rooted at the given tracking root.
func NewLedger(root string) *Ledger { return &Ledger{Root: root} }

// attributionDir is the fixed subdirectory under the tracking root.
const attributionDir = "attribution"

func (l *Ledger) streamDir(stream string) string {
	return filepath.Join(l.Root, attributionDir, safeSegment(stream))
}

// entryPath is the on-disk path for an entry. An ORIGINAL entry is
// `<defect-id>.json`; a TOMBSTONE amendment is `<defect-id>.<superseded8>.tombstone.json`,
// a distinct name so writing it can NEVER overwrite the original (append-only).
func (l *Ledger) entryPath(e LedgerEntry) string {
	dir := l.streamDir(e.Stream)
	stem := safeSegment(e.DefectID)
	if e.Tombstone {
		sup := e.Supersedes
		if len(sup) > 8 {
			sup = sup[:8]
		}
		return filepath.Join(dir, fmt.Sprintf("%s.%s.tombstone.json", stem, sup))
	}
	return filepath.Join(dir, stem+".json")
}

// Write records an ORIGINAL attribution entry: one append-only file per defect. It
// stamps RecordedAt (UTC) and REFUSES to overwrite an existing original file — a
// second attribution of the same defect is a correction and must go through Amend,
// never a silent edit (append-only invariant). Writing is atomic (temp + rename) so
// a crash never leaves a half-written entry.
func (l *Ledger) Write(entry LedgerEntry) (LedgerEntry, error) {
	entry.Tombstone = false
	entry.Supersedes = ""
	if entry.RecordedAt == "" {
		entry.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	path := l.entryPath(entry)
	if _, err := os.Stat(path); err == nil {
		return LedgerEntry{}, fmt.Errorf("ledger: entry %s already exists; a correction must be a tombstone amendment (Amend), never a silent edit", path)
	} else if !os.IsNotExist(err) {
		return LedgerEntry{}, fmt.Errorf("ledger: stat %s: %w", path, err)
	}
	if err := writeJSONAtomic(path, entry); err != nil {
		return LedgerEntry{}, err
	}
	return entry, nil
}

// Amend records a CORRECTION as a tombstone amendment: a new entry superseding a
// prior one. The prior entry's file is NOT touched — this is the whole point of an
// append-only ledger, so the audit trail keeps both the original attribution and its
// correction. It returns an error if the prior original file is missing (nothing to
// amend). The corrected entry inherits the prior's DefectID/Stream/Window.
func (l *Ledger) Amend(prior, corrected LedgerEntry) (LedgerEntry, error) {
	priorPath := l.entryPath(LedgerEntry{DefectID: prior.DefectID, Stream: prior.Stream})
	if _, err := os.Stat(priorPath); err != nil {
		return LedgerEntry{}, fmt.Errorf("ledger: cannot amend, prior entry %s absent: %w", priorPath, err)
	}
	corrected.DefectID = prior.DefectID
	corrected.Stream = prior.Stream
	if corrected.Window == "" {
		corrected.Window = prior.Window
	}
	corrected.Tombstone = true
	corrected.Supersedes = prior.EntryHash()
	if corrected.RecordedAt == "" {
		corrected.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	path := l.entryPath(corrected)
	if _, err := os.Stat(path); err == nil {
		return LedgerEntry{}, fmt.Errorf("ledger: tombstone %s already exists (this correction was already recorded)", path)
	}
	if err := writeJSONAtomic(path, corrected); err != nil {
		return LedgerEntry{}, err
	}
	return corrected, nil
}

// ReadAll loads every ledger entry under the tracking root, in a deterministic
// (path-sorted) order. It reads originals and tombstones alike — the full trail.
func (l *Ledger) ReadAll() ([]LedgerEntry, error) {
	base := filepath.Join(l.Root, attributionDir)
	var paths []string
	err := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // an unpopulated tracking root is an empty ledger, not an error
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".json") {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ledger: walk %s: %w", base, err)
	}
	sort.Strings(paths)
	var out []LedgerEntry
	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec // paths are under the operator tracking root
		if err != nil {
			return nil, fmt.Errorf("ledger: read %s: %w", p, err)
		}
		var e LedgerEntry
		if err := json.Unmarshal(b, &e); err != nil {
			return nil, fmt.Errorf("ledger: decode %s: %w", p, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// StageCounts is the per-stage tally for one scope. All four stages (including
// untraceable) are always present — an untraceable count is a first-class output,
// never omitted (spec §3.2).
type StageCounts struct {
	Spec           int `json:"spec"`
	Brief          int `json:"brief"`
	Implementation int `json:"implementation"`
	Untraceable    int `json:"untraceable"`
}

func (s *StageCounts) add(stage Stage) {
	switch stage {
	case StageSpec:
		s.Spec++
	case StageBrief:
		s.Brief++
	case StageImplementation:
		s.Implementation++
	case StageUntraceable:
		s.Untraceable++
	}
}

// Rollup is the per-stage / per-stream / per-window summary brief-12 consumes (task
// item 4). Overall is the whole-corpus tally; ByStream and ByWindow are the scoped
// tallies; ReviewEscapeByLane is the review-escape distribution — how many defects
// each lane let through. A SUPERSEDED entry (one another entry's Supersedes names)
// is excluded: the rollup is current state, the files are the trail.
type Rollup struct {
	Overall            StageCounts            `json:"overall"`
	ByStream           map[string]StageCounts `json:"by_stream"`
	ByWindow           map[string]StageCounts `json:"by_window"`
	ReviewEscapeByLane map[string]int         `json:"review_escape_by_lane"`
}

// RollupOf computes the current-state rollup from a full entry trail. It first
// resolves each defect to its LATEST non-superseded entry (a tombstone supersedes
// the original it names), then tallies. The computation is deterministic: entries
// are pre-sorted by their content address before superseding is resolved.
func RollupOf(entries []LedgerEntry) Rollup {
	superseded := map[string]struct{}{}
	for _, e := range entries {
		if e.Supersedes != "" {
			superseded[e.Supersedes] = struct{}{}
		}
	}
	r := Rollup{
		ByStream:           map[string]StageCounts{},
		ByWindow:           map[string]StageCounts{},
		ReviewEscapeByLane: map[string]int{},
	}
	for _, e := range entries {
		if _, dead := superseded[e.EntryHash()]; dead {
			continue // a corrected entry does not count toward current state
		}
		r.Overall.add(e.Stage)
		bs := r.ByStream[e.Stream]
		bs.add(e.Stage)
		r.ByStream[e.Stream] = bs
		if e.Window != "" {
			bw := r.ByWindow[e.Window]
			bw.add(e.Stage)
			r.ByWindow[e.Window] = bw
		}
		for _, lane := range e.ReviewEscape.Lanes {
			r.ReviewEscapeByLane[lane]++
		}
	}
	return r
}

// -------- small deterministic helpers --------

// jsonHash is the sha256 hex of the canonical JSON of v (struct-field order is
// stable, and all values here are hash-safe scalars/slices).
func jsonHash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("attribution: hash marshal: %v", err))
	}
	return sha256Hex(b)
}

// writeJSONAtomic marshals v (indented for a human-diffable append-only artifact)
// and writes it atomically: a temp file in the same directory, then rename. The
// parent directory is created if absent.
func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ledger: mkdir %s: %w", filepath.Dir(path), err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("ledger: marshal %s: %w", path, err)
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".attribution-*.tmp")
	if err != nil {
		return fmt.Errorf("ledger: temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("ledger: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ledger: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ledger: rename into %s: %w", path, err)
	}
	return nil
}

// safeSegment sanitises a path segment (stream name, defect id) so it cannot escape
// the tracking root or collide across separators: path separators and any
// non-[A-Za-z0-9._-] become '-'. An empty result becomes "_".
func safeSegment(s string) string {
	if s == "" {
		return "_"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	// collapse a leading run of dots so a segment can never be "." / ".."
	out = strings.TrimLeft(out, ".")
	if out == "" {
		return "_"
	}
	return out
}
