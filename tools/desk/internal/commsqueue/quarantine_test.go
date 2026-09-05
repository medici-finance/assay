package commsqueue

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// fakeFiler is a controllable IssueFiler: err (if non-nil) is returned from
// File without ever shelling out to the real `deskfile` CLI.
type fakeFiler struct {
	err   error
	calls int
	last  struct {
		env    comms.Envelope
		reason string
	}
}

func (f *fakeFiler) File(env comms.Envelope, reason string) error {
	f.calls++
	f.last.env = env
	f.last.reason = reason
	return f.err
}

// --- Never-dropped invariant: Quarantine always holds, whatever the filer does --

// TestQuarantineHeldEvenWhenFilingFails is the central never-dropped
// invariant: a quarantine issue filing failure must NEVER lose the message.
// The held-mailbox write happens first and unconditionally (quarantine.go's
// own doc); a failing filer only changes what the RETURNED error says, never
// whether the message survives.
func TestQuarantineHeldEvenWhenFilingFails(t *testing.T) {
	root := t.TempDir()
	env := sampleEnvelope("q-1")
	now := time.Now().UTC()
	filer := &fakeFiler{err: errors.New("simulated deskfile failure")}

	err := Quarantine(root, env, "test reason", now, filer)
	if err == nil {
		t.Fatalf("Quarantine with a failing filer must still return a non-nil error (ErrQuarantined, filing failed)")
	}
	if !errors.Is(err, ErrQuarantined) {
		t.Fatalf("error = %v, want it to satisfy errors.Is(_, ErrQuarantined) even though filing failed — the message IS held", err)
	}
	if filer.calls != 1 {
		t.Fatalf("filer.calls = %d, want 1", filer.calls)
	}

	// The never-dropped assertion itself: the message must be sitting in the
	// held mailbox regardless of the filing failure.
	held, lerr := ListHeld(root)
	if lerr != nil {
		t.Fatalf("ListHeld: %v", lerr)
	}
	if len(held) != 1 || held[0].Envelope.ID != "q-1" {
		t.Fatalf("held = %v, want exactly one item q-1 — a filing failure must never lose the message", held)
	}
	if held[0].Reason != "test reason" {
		t.Fatalf("held reason = %q, want %q", held[0].Reason, "test reason")
	}
}

// TestQuarantineHeldWithNilFiler covers the OTHER "never a silent drop" edge:
// a nil filer (no issue-filing configured at all) must still hold the
// message and report ErrQuarantined — never silently succeed with no
// record and never panic on the nil interface.
func TestQuarantineHeldWithNilFiler(t *testing.T) {
	root := t.TempDir()
	env := sampleEnvelope("q-nil-filer")
	now := time.Now().UTC()

	err := Quarantine(root, env, "no filer configured", now, nil)
	if !errors.Is(err, ErrQuarantined) {
		t.Fatalf("error = %v, want ErrQuarantined even with a nil filer", err)
	}

	held, lerr := ListHeld(root)
	if lerr != nil {
		t.Fatalf("ListHeld: %v", lerr)
	}
	if len(held) != 1 || held[0].Envelope.ID != "q-nil-filer" {
		t.Fatalf("held = %v, want exactly one item q-nil-filer", held)
	}
}

// TestQuarantineSucceedsRecordsFilerCall is the happy-path complement: when
// filing succeeds the filer sees the right envelope + reason, and the error
// returned is STILL ErrQuarantined (an informational return, per its own
// doc — a quarantine is never a silent success either, so a caller can log
// or count it).
func TestQuarantineSucceedsRecordsFilerCall(t *testing.T) {
	root := t.TempDir()
	env := sampleEnvelope("q-ok")
	now := time.Now().UTC()
	filer := &fakeFiler{}

	err := Quarantine(root, env, "awaiting router", now, filer)
	if !errors.Is(err, ErrQuarantined) {
		t.Fatalf("error = %v, want ErrQuarantined (informational, even on a successful filing)", err)
	}
	if filer.calls != 1 {
		t.Fatalf("filer.calls = %d, want 1", filer.calls)
	}
	if filer.last.env.ID != "q-ok" || filer.last.reason != "awaiting router" {
		t.Fatalf("filer saw env=%v reason=%q, want id=q-ok reason=%q", filer.last.env, filer.last.reason, "awaiting router")
	}
}

// TestQuarantineHeldWriteFailureIsNotErrQuarantined is the boundary case
// quarantine.go's own doc calls out explicitly: when the held-mailbox WRITE
// itself fails (as opposed to the filing step), the message really may be
// lost — that must be a DISTINCT, non-ErrQuarantined error so a caller never
// mistakes "durably held" for "write failed".
func TestQuarantineHeldWriteFailureIsNotErrQuarantined(t *testing.T) {
	root := t.TempDir()
	// Make HeldDir(root) unusable: pre-create "held" as a REGULAR FILE so
	// os.MkdirAll(HeldDir(root), ...) fails (a path component exists and is
	// not a directory) — forcing WriteHeld itself to fail before it ever
	// reaches the filer.
	if err := os.WriteFile(filepath.Join(root, "held"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	env := sampleEnvelope("q-lost")
	filer := &fakeFiler{}
	err := Quarantine(root, env, "held-write will fail", time.Now().UTC(), filer)
	if err == nil {
		t.Fatalf("Quarantine must error when the held-mailbox write itself fails")
	}
	if errors.Is(err, ErrQuarantined) {
		t.Fatalf("error = %v, must NOT satisfy errors.Is(_, ErrQuarantined) — the held write failed, so the message may be genuinely lost, which is a different condition than 'held, filing failed'", err)
	}
	if filer.calls != 0 {
		t.Fatalf("filer.calls = %d, want 0 — filing must never be attempted once the held write itself failed", filer.calls)
	}
}

// --- AppendJournal: append-only, never truncates ------------------------------

func TestAppendJournalNeverTruncatesPriorLines(t *testing.T) {
	root := t.TempDir()
	if err := AppendJournal(root, "line one"); err != nil {
		t.Fatalf("AppendJournal: %v", err)
	}
	if err := AppendJournal(root, "line two"); err != nil {
		t.Fatalf("AppendJournal: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "journal.log"))
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	want := "line one\nline two\n"
	if string(raw) != want {
		t.Fatalf("journal.log = %q, want %q (append-only, in order, never truncated)", string(raw), want)
	}
}
