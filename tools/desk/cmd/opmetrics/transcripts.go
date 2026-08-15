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

// ReadOperatorMessages walks dir for *.jsonl transcripts and returns the operator's
// turns whose timestamp falls on the given local date, plus every session's span.
//
// Fail-closed: an unreadable directory is an error. The caller turns that into
// could-not-check — never into zero.
func ReadOperatorMessages(dir string, day time.Time) (TranscriptRead, error) {
	var out TranscriptRead
	info, err := os.Stat(dir)
	if err != nil {
		return out, fmt.Errorf("transcripts dir %s is not readable: %w", dir, err)
	}
	if !info.IsDir() {
		return out, fmt.Errorf("transcripts path %s is not a directory", dir)
	}

	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		f, oerr := os.Open(path)
		if oerr != nil {
			return oerr
		}
		defer f.Close()
		out.Files++

		// The session key is the transcript's own sessionId when present, falling
		// back to the file's base name. It is used only to SCOPE duplicate detection
		// and to bound a span; it is never emitted.
		fileKey := strings.TrimSuffix(d.Name(), ".jsonl")
		span := SessionSpan{Session: fileKey}

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), scanBufMax)
		for sc.Scan() {
			line := sc.Bytes()
			if len(strings.TrimSpace(string(line))) == 0 {
				continue
			}
			var r rawRecord
			if jerr := json.Unmarshal(line, &r); jerr != nil {
				out.Unparseable++
				continue
			}
			ts, terr := time.Parse(time.RFC3339, r.Timestamp)
			if terr != nil {
				// A record with no usable timestamp cannot be attributed to a day.
				// Drop and count rather than assign it to today.
				if r.Timestamp != "" {
					out.Unparseable++
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
			out.Messages = append(out.Messages, OperatorMessage{Session: key, At: ts, Text: text})
		}
		if serr := sc.Err(); serr != nil {
			return fmt.Errorf("reading %s: %w", path, serr)
		}
		if !span.First.IsZero() {
			out.Spans = append(out.Spans, span)
		}
		return nil
	})
	if walkErr != nil {
		return out, fmt.Errorf("walking transcripts dir %s: %w", dir, walkErr)
	}
	return out, nil
}
