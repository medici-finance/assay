package telemetry

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// FileAdapter is the file-based TelemetrySource reference adapter (spec
// §7.3, quality/13's OSS-ability proof): it reads a documented telemetry
// JSONL layout from an operator-supplied path and answers TelemetrySource
// lookups against it. It is the only concrete TelemetrySource this package
// ships — a house or other operator's own data source is configuration
// (point --telemetry at a file this shape, or write a second adapter against
// the same interface), never a change to this one.
//
// # Telemetry JSONL layout
//
// One JSON object per line (blank lines and lines starting with a leading
// '#' are skipped as comments). Each object carries the PRKey fields plus the
// five TelemetryRecord fields, all required:
//
//	{"pr_number": 42, "merge_sha": "<full commit SHA>", "stream_task_id": "quality/13",
//	 "retries": 7, "context_length": 18000, "tool_call_churn": 3, "interruptions": 1, "refusals": 0}
//
// A record missing any of the five metric fields, or one that fails to parse
// at all, is unreadable for its key — Telemetry reports could-not-measure
// for that key, never zero (spec §3.2). A line whose JSON is malformed
// enough that even the PRKey fields cannot be recovered cannot be attributed
// to a key at all; it is counted as a load-time parse failure and skipped,
// which FileAdapter surfaces via LoadErrors() so a caller can distinguish "no
// malformed lines" from "never checked."
//
// A key repeated across multiple lines: the LAST line wins, so an operator's
// append-only telemetry log can be corrected by appending a newer record
// for the same key.
type FileAdapter struct {
	path       string
	openErr    error
	records    map[PRKey]Measure[TelemetryRecord]
	loadErrors []string
}

// fileRow is the on-disk JSONL row shape. The five metric fields are
// pointers so FileAdapter can tell "field absent" (nil) apart from "field
// present and explicitly zero" (non-nil, *0) — the same distinction the
// three-state invariant exists to preserve.
type fileRow struct {
	PRNumber      int    `json:"pr_number"`
	MergeSHA      string `json:"merge_sha"`
	StreamTaskID  string `json:"stream_task_id"`
	Retries       *int   `json:"retries"`
	ContextLength *int   `json:"context_length"`
	ToolCallChurn *int   `json:"tool_call_churn"`
	Interruptions *int   `json:"interruptions"`
	Refusals      *int   `json:"refusals"`
}

// NewFileAdapter builds a FileAdapter over path. It does NOT return an error
// for a missing or unreadable file — that is itself a legitimate
// could-not-measure condition (spec §3.2), reported per key by Telemetry,
// not a construction-time fatal. Loading happens eagerly here (not lazily on
// first Telemetry call) so a caller can inspect LoadErrors() once, up front.
func NewFileAdapter(path string) *FileAdapter {
	a := &FileAdapter{path: path, records: map[PRKey]Measure[TelemetryRecord]{}}
	f, err := os.Open(path)
	if err != nil {
		a.openErr = err
		return a
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// The default bufio.Scanner token limit (64KiB) is generous for one JSON
	// telemetry row but not unbounded; raise it modestly for headroom without
	// accepting an arbitrarily large hostile line.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || trimmed[0] == '#' {
			continue
		}
		var row fileRow
		if err := json.Unmarshal(trimmed, &row); err != nil {
			a.loadErrors = append(a.loadErrors, fmt.Sprintf("%s:%d: %v", path, line, err))
			continue
		}
		key := PRKey{PRNumber: row.PRNumber, MergeSHA: row.MergeSHA, StreamTaskID: row.StreamTaskID}
		if row.Retries == nil || row.ContextLength == nil || row.ToolCallChurn == nil ||
			row.Interruptions == nil || row.Refusals == nil {
			a.records[key] = CouldNotMeasure[TelemetryRecord](
				fmt.Sprintf("%s:%d: telemetry record for %s is missing one or more required fields (retries, context_length, tool_call_churn, interruptions, refusals)", path, line, key))
			continue
		}
		rec := TelemetryRecord{
			Retries:       *row.Retries,
			ContextLength: *row.ContextLength,
			ToolCallChurn: *row.ToolCallChurn,
			Interruptions: *row.Interruptions,
			Refusals:      *row.Refusals,
		}
		if rec == (TelemetryRecord{}) {
			// A genuine all-zero session (a flawless one-shot: no retries, no
			// churn, no interruptions, no refusals) — measured-zero, not
			// could-not-measure. ContextLength==0 is unusual for a real
			// session but not impossible for a planted or degenerate
			// fixture; it does not change the classification.
			a.records[key] = MeasuredZero[TelemetryRecord]()
		} else {
			a.records[key] = Measured(rec)
		}
	}
	if err := scanner.Err(); err != nil {
		a.loadErrors = append(a.loadErrors, fmt.Sprintf("%s: scan error: %v", path, err))
	}
	return a
}

// LoadErrors reports lines that failed to parse at all (so the key could not
// even be recovered) during construction — a could-not-check on the FILE
// itself, distinct from a per-key could-not-measure. Empty means "no
// malformed lines were seen," not "never checked": callers that need the
// distinction should also check OpenError().
func (a *FileAdapter) LoadErrors() []string { return a.loadErrors }

// OpenError reports the error from opening path, if any (including "file
// does not exist"). Every Telemetry lookup on an adapter with a non-nil
// OpenError returns could-not-measure naming this error.
func (a *FileAdapter) OpenError() error { return a.openErr }

// Telemetry implements TelemetrySource. A missing file, a missing key, or an
// unreadable record all yield could-not-measure for that key — never a
// silent zero (spec §3.2, quality/13 Task item 2).
func (a *FileAdapter) Telemetry(key PRKey) Measure[TelemetryRecord] {
	if a.openErr != nil {
		return CouldNotMeasure[TelemetryRecord](fmt.Sprintf("telemetry file %q: %v", a.path, a.openErr))
	}
	rec, ok := a.records[key]
	if !ok {
		return CouldNotMeasure[TelemetryRecord](fmt.Sprintf("no telemetry record for %s in %q", key, a.path))
	}
	return rec
}

var _ TelemetrySource = (*FileAdapter)(nil)
