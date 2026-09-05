// Package commsqueue is the durable on-disk queue SHARED between cmd/commsgw
// (the gateway, which writes it) and cmd/commsloop (the drain consumer, which
// reads and retires it). The two are SEPARATE binaries/processes ("copy
// scanloop's adapter shape") — so they agree on disk, never over an
// in-process channel, and this package is the ONE place that shape is
// defined, so the writer and the reader can never drift apart.
//
// Every write here is ATOMIC: build the full bytes, write to a sibling temp
// file, then rename into place, so a reader can never observe a partial file.
package commsqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// AcceptedItem is one message durably queued for commsloop to drain.
type AcceptedItem struct {
	Envelope   comms.Envelope `json:"envelope"`
	AcceptedAt time.Time      `json:"acceptedAt"`
}

// HeldItem is one quarantined message: accepted at the pre-check layer (or
// refused later, defense-in-depth, at the routing boundary) but never dropped.
type HeldItem struct {
	Envelope comms.Envelope `json:"envelope"`
	Reason   string         `json:"reason"`
	HeldAt   time.Time      `json:"heldAt"`
}

// Notice is one message a role's mailbox holds, mirroring
// cmd/deskcomms/gateway.go's wire shape field-for-field.
type Notice struct {
	ID      string          `json:"id"`
	From    comms.SenderID  `json:"from"`
	Verb    string          `json:"verb"`
	Class   string          `json:"class"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Sent    time.Time       `json:"sent"`
}

func AcceptedDir(root string) string { return filepath.Join(root, "accepted") }
func HeldDir(root string) string     { return filepath.Join(root, "held") }
func MailboxDir(root, cell, role string) string {
	return filepath.Join(root, "mailbox", safeSeg(cell), safeSeg(role))
}
func MailboxAckedDir(root, cell, role string) string {
	return filepath.Join(MailboxDir(root, cell, role), "acked")
}

// safeSeg keeps a cell/role/id name to a single path segment, mirroring
// deskkit.claimPath's sanitizer, so a hostile or malformed field can never
// escape the queue root.
func safeSeg(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "_"
	}
	return string(out)
}

func atomicWriteJSON(dir, name string, v any) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("commsqueue: cannot create %s: %w", dir, err)
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("commsqueue: cannot marshal %s/%s: %w", dir, name, err)
	}
	final := filepath.Join(dir, safeSeg(name)+".json")
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("commsqueue: cannot write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commsqueue: cannot rename %s -> %s: %w", tmp, final, err)
	}
	return nil
}

// WriteAccepted durably queues env for commsloop to drain.
func WriteAccepted(root string, env comms.Envelope, now time.Time) error {
	return atomicWriteJSON(AcceptedDir(root), env.ID, AcceptedItem{Envelope: env, AcceptedAt: now})
}

// ListAccepted reads every queued item, sorted by id for determinism. A
// missing accepted/ directory is an EMPTY queue (nothing accepted yet), not an
// error. That is a DIFFERENT condition from the queue SOURCE itself being
// absent/unreachable (commsloop's Monitor: "nil/unreachable Monitor = refuse,
// never empty queue" binds the source being missing, not an empty result from
// a healthy, present one).
func ListAccepted(root string) ([]AcceptedItem, error) {
	return listJSON[AcceptedItem](AcceptedDir(root))
}

// RemoveAccepted removes an item from the accepted-queue once commsloop has
// landed it (moved to done+journal, or to held).
func RemoveAccepted(root, id string) error {
	return removeJSON(AcceptedDir(root), id)
}

// WriteHeld durably quarantines env. A second hold of the same id updates the
// reason/time in place — the message is still held either way.
func WriteHeld(root string, env comms.Envelope, reason string, now time.Time) error {
	return atomicWriteJSON(HeldDir(root), env.ID, HeldItem{Envelope: env, Reason: reason, HeldAt: now})
}

// ListHeld reads every quarantined item, sorted by id.
func ListHeld(root string) ([]HeldItem, error) {
	return listJSON[HeldItem](HeldDir(root))
}

// DeliverToMailbox places notice in (cell, role)'s mailbox for poll/ack.
func DeliverToMailbox(root, cell, role string, notice Notice) error {
	return atomicWriteJSON(MailboxDir(root, cell, role), notice.ID, notice)
}

// PollMailbox returns every unacked notice in (cell, role)'s mailbox, sorted
// by id.
func PollMailbox(root, cell, role string) ([]Notice, error) {
	return listJSON[Notice](MailboxDir(root, cell, role))
}

// AckMailbox moves id from (cell, role)'s mailbox to its acked partition.
// Acknowledgement MOVES, it never deletes.
func AckMailbox(root, cell, role, id string) error {
	dir := MailboxDir(root, cell, role)
	src := filepath.Join(dir, safeSeg(id)+".json")
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("commsqueue: ack %s: not in mailbox (%s/%s): %w", id, cell, role, err)
	}
	ackedDir := MailboxAckedDir(root, cell, role)
	if err := os.MkdirAll(ackedDir, 0o700); err != nil {
		return fmt.Errorf("commsqueue: ack %s: cannot create acked dir: %w", id, err)
	}
	dst := filepath.Join(ackedDir, safeSeg(id)+".json")
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		return fmt.Errorf("commsqueue: ack %s: cannot write acked copy: %w", id, err)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("commsqueue: ack %s: acked copy written but original not removed: %w", id, err)
	}
	return nil
}

// listJSON reads every *.json file directly under dir (never recursing into
// a subdirectory such as mailbox's acked/), sorted by filename, decoding each
// into T. A missing dir is an empty, nil-error result.
func listJSON[T any](dir string) ([]T, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("commsqueue: cannot list %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	out := make([]T, 0, len(names))
	for _, n := range names {
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, fmt.Errorf("commsqueue: cannot read %s/%s: %w", dir, n, err)
		}
		var v T
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("commsqueue: %s/%s does not parse: %w", dir, n, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func removeJSON(dir, id string) error {
	p := filepath.Join(dir, safeSeg(id)+".json")
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("commsqueue: cannot remove %s: %w", p, err)
	}
	return nil
}

// appendLine appends one line (plus a trailing newline) to root/name,
// creating both root and the file as needed. It is an append-only O_APPEND
// write — never a read-modify-write of the whole file — so two writers never
// race a truncate.
func appendLine(root, name, line string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("commsqueue: cannot create %s: %w", root, err)
	}
	f, err := os.OpenFile(filepath.Join(root, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("commsqueue: cannot open %s/%s: %w", root, name, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("commsqueue: cannot append to %s/%s: %w", root, name, err)
	}
	return nil
}
