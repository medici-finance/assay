package main

// transcripts.go — reading the operator's own session transcripts.
//
// THE PRIVACY LINE IS DRAWN HERE. This file is the only reader of
// ~/.claude/projects/<slug>/*.jsonl. Three rules it does not bend:
//
//  1. READ-ONLY. Nothing in this package opens a transcript for writing, moves one,
//     or deletes one. There is no code path that could.
//  2. Text leaves this file exactly once, into the classifier, which returns a LABEL.
//     No caller of ReadOperatorMessages holds a message body past classification, and
//     no emit struct has a field that could hold one (emit.go, allowlist test).
//  3. Every default is overridable so tests never touch the real home directory. A
//     test that only passes on one machine is not a test.
//
// WHAT COUNTS AS AN OPERATOR MESSAGE. A transcript interleaves the human's turns with
// tool results, sidechain (subagent) traffic, and harness bookkeeping — all of which
// arrive as type:"user" records. Counting those as operator messages would inflate
// the denominator by an order of magnitude and make the relay ratio meaningless. The
// filter is therefore restrictive and each clause is justified:
//
//	type == "user"                   the record shape that can carry a human turn
//	message.role == "user"           excludes harness records with no message
//	content has NO tool_result part  a tool result is the harness talking, not a human
//	isSidechain != true              subagent traffic is not the operator
//	isMeta != true                   harness-injected context, not a typed turn
//	userType == "external"           the harness's own marker for a human-supplied turn
//
// Records the filter cannot classify are DROPPED and COUNTED (see Read.Unparseable),
// never guessed at. A transcript directory that cannot be read at all is an error, so
// the caller reports could-not-check rather than a zero.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OperatorMessage is one human turn. Text is present ONLY so the classifier can label
// it; it is not carried into any emitted structure.
type OperatorMessage struct {
	Session string
	At      time.Time
	Text    string
}

// SessionSpan is one transcript file's wall-clock extent, used for session hygiene.
type SessionSpan struct {
	Session string
	First   time.Time
	Last    time.Time
}

// TranscriptRead is everything one pass over the transcript tree yields.
type TranscriptRead struct {
	Messages []OperatorMessage
	Spans    []SessionSpan
	// Files is how many .jsonl files were opened. Zero files with no error still
	// means could-not-check, not "the operator sent no messages" — an empty tree and
	// a mis-pointed --transcripts flag look identical from the inside.
	Files int
	// Unparseable counts lines that were not valid JSON. Reported so a silently
	// half-read transcript cannot masquerade as a quiet day.
	Unparseable int
}

// scanBufMax is the per-line ceiling for the transcript scanner. Transcript lines
// carry whole tool results and pasted files, so the default 64 KiB is far too small —
// a truncated line would be silently dropped and quietly deflate the count.
const scanBufMax = 16 << 20 // 16 MiB

type rawRecord struct {
	Type        string          `json:"type"`
	Timestamp   string          `json:"timestamp"`
	SessionID   string          `json:"sessionId"`
	IsSidechain *bool           `json:"isSidechain"`
	IsMeta      *bool           `json:"isMeta"`
	UserType    string          `json:"userType"`
	Message     json.RawMessage `json:"message"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type rawPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// operatorText returns the human-typed text of a record and whether the record is an
// operator turn at all. The bool is the whole filter described in the file comment.
func operatorText(r rawRecord) (string, bool) {
	if r.Type != "user" {
		return "", false
	}
	if r.IsSidechain != nil && *r.IsSidechain {
		return "", false
	}
	if r.IsMeta != nil && *r.IsMeta {
		return "", false
	}
	if r.UserType != "" && r.UserType != "external" {
		return "", false
	}
	if len(r.Message) == 0 {
		return "", false
	}
	var m rawMessage
	if err := json.Unmarshal(r.Message, &m); err != nil {
		return "", false
	}
	if m.Role != "user" {
		return "", false
	}

	// content: a bare string is always a typed turn.
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return s, true
	}
	// content: an array. Any tool_result part makes the record harness traffic.
	var parts []rawPart
	if err := json.Unmarshal(m.Content, &parts); err != nil {
		return "", false
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "tool_result" {
			return "", false
		}
		if p.Type == "text" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
	}
	// An image-only turn yields "" and is still an operator turn; the classifier
	// labels it ClassEmpty and it stays out of the ratio's denominator.
	return b.String(), true
}

// fileScan is one transcript file's contribution, returned by scanTranscriptFile
// so the same per-file parse serves both the single-root and multi-root readers.
type fileScan struct {
	Messages    []OperatorMessage
	Span        SessionSpan
	HasSpan     bool
	Unparseable int
	// SessionKey is the file's dedup key across roots — its records' own
	// sessionId when present, else the base name. Never emitted; used only to
	// count a session that was synced to two profiles ONCE.
	SessionKey string
}

// scanTranscriptFile parses one *.jsonl transcript into its operator turns on the
// given local day plus its whole-file span. It is strictly read-only.
func scanTranscriptFile(path string, day time.Time) (fileScan, error) {
	var fsr fileScan
	f, oerr := os.Open(path)
	if oerr != nil {
		return fsr, oerr
	}
	defer f.Close()

	fileKey := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	span := SessionSpan{Session: fileKey}
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), scanBufMax)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var r rawRecord
		if jerr := json.Unmarshal(line, &r); jerr != nil {
			fsr.Unparseable++
			continue
		}
		if fsr.SessionKey == "" && r.SessionID != "" {
			fsr.SessionKey = r.SessionID
		}
		ts, terr := time.Parse(time.RFC3339, r.Timestamp)
		if terr != nil {
			// A record with no usable timestamp cannot be attributed to a day.
			// Drop and count rather than assign it to today.
			if r.Timestamp != "" {
				fsr.Unparseable++
			}
			continue
		}
		ts = ts.In(day.Location())
		if span.First.IsZero() || ts.Before(span.First) {
			span.First = ts
		}
		if ts.After(span.Last) {
			span.Last = ts
		}

		text, ok := operatorText(r)
		if !ok {
			continue
		}
		if ts.Before(dayStart) || !ts.Before(dayEnd) {
			continue
		}
		key := r.SessionID
		if key == "" {
			key = fileKey
		}
		fsr.Messages = append(fsr.Messages, OperatorMessage{Session: key, At: ts, Text: text})
	}
	if serr := sc.Err(); serr != nil {
		return fsr, fmt.Errorf("reading %s: %w", path, serr)
	}
	if fsr.SessionKey == "" {
		fsr.SessionKey = fileKey
	}
	if !span.First.IsZero() {
		fsr.Span = span
		fsr.HasSpan = true
	}
	return fsr, nil
}

// readOneRoot walks one directory for *.jsonl transcripts and folds each file's
// scan into out. seen dedupes by file SessionKey across the whole read: a file
// whose session was already contributed (by an earlier root, or an earlier file)
// is skipped and NOT counted, so a session synced to two profiles is read once.
// Pass seen==nil to disable dedup (the single-root reader keeps its old
// every-file behaviour).
func readOneRoot(dir string, day time.Time, out *TranscriptRead, seen map[string]bool) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		fsr, serr := scanTranscriptFile(path, day)
		if serr != nil {
			return serr
		}
		if seen != nil {
			if seen[fsr.SessionKey] {
				return nil // a re-synced copy of a session already read — count it once
			}
			seen[fsr.SessionKey] = true
		}
		out.Files++
		out.Messages = append(out.Messages, fsr.Messages...)
		out.Unparseable += fsr.Unparseable
		if fsr.HasSpan {
			out.Spans = append(out.Spans, fsr.Span)
		}
		return nil
	})
}

// ReadOperatorMessages walks dir for *.jsonl transcripts and returns the operator's
// turns whose timestamp falls on the given local date, plus every session's span.
//
// Fail-closed: an unreadable directory is an error. The caller turns that into
// could-not-check — never into zero. This single-root form does NOT dedupe (it is
// the reader the unit tests drive directly); cross-root dedup is
// ReadOperatorMessagesMulti's job.
func ReadOperatorMessages(dir string, day time.Time) (TranscriptRead, error) {
	var out TranscriptRead
	info, err := os.Stat(dir)
	if err != nil {
		return out, fmt.Errorf("transcripts dir %s is not readable: %w", dir, err)
	}
	if !info.IsDir() {
		return out, fmt.Errorf("transcripts path %s is not a directory", dir)
	}
	if walkErr := readOneRoot(dir, day, &out, nil); walkErr != nil {
		return out, fmt.Errorf("walking transcripts dir %s: %w", dir, walkErr)
	}
	return out, nil
}

// ReadOperatorMessagesMulti reads EVERY root in dirs and merges the operator's
// turns, deduping by session id across roots so a session synced to two profiles
// is counted once. A root that cannot be read is SKIPPED (returned in skipped for
// the caller to NOTICE) rather than failing the whole read; the error is returned
// only when NOT ONE root was readable — the honest could-not-check, never a zero.
// readable is how many roots were actually walked.
func ReadOperatorMessagesMulti(dirs []string, day time.Time) (out TranscriptRead, skipped []string, readable int, err error) {
	seen := map[string]bool{}
	for _, dir := range dirs {
		info, serr := os.Stat(dir)
		if serr != nil || !info.IsDir() {
			skipped = append(skipped, dir)
			continue
		}
		if walkErr := readOneRoot(dir, day, &out, seen); walkErr != nil {
			return out, skipped, readable, fmt.Errorf("walking transcripts dir %s: %w", dir, walkErr)
		}
		readable++
	}
	if readable == 0 {
		return out, skipped, 0, fmt.Errorf("no readable transcripts root among %d candidate(s)", len(dirs))
	}
	return out, skipped, readable, nil
}
