package main

// History (docs/streams/.history.jsonl) is an APPEND-ONLY, single-writer log
// of brief status transitions, one JSON object per line (JSONL):
//
//	{"ts":"2026-07-09T18:04:05Z","brief":"methodology-metrics/01","from":"implemented","to":"verified","sha":"a1b2c3d4e5f6..."}
//
// Fields:
//   - ts:    RFC3339 UTC timestamp of the regen run that observed the change.
//   - brief: typed id "<stream>/<NN>" — the same convention as BriefFile.Brief
//     and the depends/unblocks refs (brieffile.go).
//   - from:  the previously recorded status for this brief, or "" on the
//     FIRST record ever written for it (the seed entry: the log had no
//     prior knowledge, so it records current state with an empty origin
//     rather than fabricating a history that was never observed).
//   - to:    the newly observed status (current stream README table value).
//   - sha:   the commit sha statusgen ran against when it recorded the
//     transition ("" if git info is unavailable, e.g. a non-git checkout).
//
// Single-writer discipline (mirrors STATUS.md exactly, see main.go's run()):
// only mode == "record" appends, and that mode is wired into ONLY main's
// status-regen CI (.github/workflows/status-regen.yml), never the PR-side
// `--lint` gate. A branch/PR cannot forge a transition — the log only grows
// from commits main's CI has itself regenerated against.
//
// Idempotent: if current state matches the last recorded state for every
// brief, diffHistory returns no entries and appendHistory is a true no-op —
// re-running --record with no status changes leaves the file byte-identical.
//
// Format stability: this is the queryable substrate downstream statusgen
// subcommands (methodology-metrics/02 --dora, /03 --trend, /05 alarm KPIs)
// read via LoadHistory + LastRecordedStatus. Adding a field is safe (old
// readers ignore unknown keys); do not rename or repurpose an existing key.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// historyRelPath is the log's location relative to repo root — kept beside
// the stream sources it derives from (docs/streams/), git-tracked so it
// survives (methodology-metrics/01).
const historyRelPath = "docs/streams/.history.jsonl"

// HistoryEntry is one recorded brief status transition.
type HistoryEntry struct {
	Ts    string `json:"ts"`
	Brief string `json:"brief"`
	From  string `json:"from"`
	To    string `json:"to"`
	SHA   string `json:"sha"`
}

// nowFunc is the historian's clock; overridden in tests for determinism.
var nowFunc = func() time.Time { return time.Now().UTC() }

// LoadHistory reads the append-only transition log. A missing file is not an
// error — it means no transitions have ever been recorded (first run ever).
// Blank lines are skipped; a malformed line is a hard error (the log is a
// machine format with no free-text tolerance, unlike FINDINGS.md/INTAKE.md).
func LoadHistory(path string) ([]HistoryEntry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []HistoryEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("%s: malformed history line: %w", path, err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return entries, nil
}

// LastRecordedStatus replays the log in append order and returns each
// brief's most recently recorded "to" status. A brief absent from the
// returned map has never been recorded — the next record run seeds it with
// from="".
func LastRecordedStatus(entries []HistoryEntry) map[string]string {
	last := make(map[string]string, len(entries))
	for _, e := range entries {
		last[e.Brief] = e.To
	}
	return last
}

// LastTransitionTime returns each brief's most-recent recorded transition
// timestamp, parsed from the log's RFC3339 `ts` field. It is the per-brief
// staleness clock the Next-up score reads (methodology-metrics/14): a brief
// ages from its OWN last status change, so sibling activity elsewhere in the
// stream no longer resets its aging (the F-09 clock defect). A brief absent
// from the returned map has no recorded history — Next-up falls back to the
// stream's git LastTouch for it. Unparseable timestamps are skipped (the log
// is machine-written, but a bad row must not crash the scorer); the latest
// parseable ts per brief wins.
func LastTransitionTime(entries []HistoryEntry) map[string]time.Time {
	out := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if cur, ok := out[e.Brief]; !ok || ts.After(cur) {
			out[e.Brief] = ts
		}
	}
	return out
}

// diffHistory computes the new transition records to append: every brief
// whose current status differs from the last recorded one, including briefs
// never recorded before (seeded with from=""). Iteration order is
// deterministic — streams sorted by name (the loadStreams invariant),
// briefs in stream README table order — so re-runs against unchanged
// sources always compute the identical (empty) diff.
func diffHistory(streams []*Stream, last map[string]string, sha string, ts time.Time) []HistoryEntry {
	var out []HistoryEntry
	tsStr := ts.Format(time.RFC3339)
	for _, s := range streams {
		for _, b := range s.Briefs {
			id := s.Name + "/" + b.Num
			from, seen := last[id]
			if seen && from == b.Status {
				continue
			}
			out = append(out, HistoryEntry{
				Ts:    tsStr,
				Brief: id,
				From:  from, // "" when unseen — the seed entry
				To:    b.Status,
				SHA:   sha,
			})
		}
	}
	return out
}

// appendHistory appends entries to the log, creating it (and its parent dir)
// if absent. A nil/empty entries slice is a true no-op: the file is not
// opened, touched, or its mtime bumped — re-running with no status changes
// appends nothing (methodology-metrics/01 Task item 4).
func appendHistory(path string, entries []HistoryEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// recordHistory is the single-writer append path: diff current brief status
// against the log's last recorded state and append any transitions. Called
// ONLY from run()'s mode == "record" branch, which main's status-regen CI
// invokes after STATUS.md regenerates — never from --lint (see main.go).
func recordHistory(root string, streams []*Stream) int {
	path := filepath.Join(root, filepath.FromSlash(historyRelPath))
	existing, err := LoadHistory(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: history:", err)
		return 1
	}
	last := LastRecordedStatus(existing)
	sha := gitCurrentSHA(root)
	entries := diffHistory(streams, last, sha, nowFunc())
	if err := appendHistory(path, entries); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: history:", err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Println("history: no status changes — nothing appended")
		return 0
	}
	fmt.Printf("history: appended %d transition(s) to %s\n", len(entries), path)
	return 0
}
