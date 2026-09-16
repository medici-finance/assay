package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
)

// refusal_test.go — the finer contracts of the refusal journal (#1165) that
// refusal_journal_test.go's fail-first drill does not pin: classification
// is exhaustive over the gateway's typed refusals, the peer-unauthenticated
// line reads nothing from the bytes, the gateway cell is stamped, advisory
// sender metadata is stripped, and a journal write failure surfaces on the
// receipt while the refusal stands.

// TestClassifyRefusalExhaustive maps every typed refusal the gateway can
// return (precheck.go's Err* and the comms identity errors it wraps) onto a
// DISTINCT vocabulary member — never RefusalOther, which is reserved for a
// refusal nobody anticipated.
func TestClassifyRefusalExhaustive(t *testing.T) {
	cases := map[commsqueue.RefusalKind]error{
		commsqueue.RefusalPeerUnauthenticated:     ErrPeerUnauthenticated,
		commsqueue.RefusalCarrierMalformed:        fmt.Errorf("%w: no parts", errCarrierMalformed),
		commsqueue.RefusalEnvelopeParse:           fmt.Errorf("%w: %w", ErrEnvelopeParse, comms.ErrBadSchema),
		commsqueue.RefusalUnknownCell:             fmt.Errorf("%w: %w", ErrAssertionInvalid, comms.ErrUnknownCell),
		commsqueue.RefusalBadSignature:            fmt.Errorf("%w: %w", ErrAssertionInvalid, comms.ErrBadSignature),
		commsqueue.RefusalExpired:                 fmt.Errorf("%w: %w", ErrAssertionInvalid, comms.ErrExpired),
		commsqueue.RefusalNotYetValid:             fmt.Errorf("%w: %w", ErrAssertionInvalid, comms.ErrNotYetValid),
		commsqueue.RefusalReplay:                  fmt.Errorf("%w: %w", ErrAssertionInvalid, comms.ErrReplay),
		commsqueue.RefusalIdentityMismatch:        fmt.Errorf("%w: %w", ErrAssertionInvalid, comms.ErrIdentityMismatch),
		commsqueue.RefusalAssertionInvalid:        fmt.Errorf("%w: something new", ErrAssertionInvalid),
		commsqueue.RefusalLaneDenied:              fmt.Errorf("%w: (a -> b)", ErrLaneDenied),
		commsqueue.RefusalCrossCellPair:           fmt.Errorf("%w: (a -> b)", ErrCrossCellPair),
		commsqueue.RefusalCrossCellVerb:           fmt.Errorf("%w: handoff", ErrCrossCellVerb),
		commsqueue.RefusalDuplicate:               fmt.Errorf("%w: id", ErrDuplicateMessage),
		commsqueue.RefusalBudgetExhausted:         fmt.Errorf("%w: a/b", ErrBudgetExhausted),
		commsqueue.RefusalRateLimiterUnconfigured: ErrRateLimiterUnconfigured,
		commsqueue.RefusalKillSwitch:              fmt.Errorf("%w: STOP", ErrKillSwitch),
		commsqueue.RefusalOther:                   errors.New("commsgw: something nobody typed"),
	}
	seen := map[commsqueue.RefusalKind]bool{}
	for want, err := range cases {
		got := ClassifyRefusal(err)
		if got != want {
			t.Errorf("ClassifyRefusal(%v) = %q, want %q", err, got, want)
		}
		seen[got] = true
	}
	for kind := range commsqueue.KnownRefusalKinds {
		if !seen[kind] {
			t.Errorf("vocabulary member %q has no refusal that classifies to it — add the case above or retire the member", kind)
		}
	}
	if len(seen) != len(commsqueue.KnownRefusalKinds) {
		t.Fatalf("classification covers %d kinds, vocabulary has %d", len(seen), len(commsqueue.KnownRefusalKinds))
	}
}

// TestRefusalRecordPeerUnauthenticatedReadsNothing: precheck.go refuses an
// unauthenticated peer "before a single byte of it is parsed", and the
// journal line honours that — even bytes that WOULD present an id and a
// sender are not read for one; the line carries only the digest, the cell
// and the kind.
func TestRefusalRecordPeerUnauthenticatedReadsNothing(t *testing.T) {
	raw := []byte(`{"id":"i-claim-to-be","from":{"cell":"cell-x","role":"the-desk"},"to":{"cell":"cell-a","role":"the-desk"},"verb":"status"}`)
	rec := RefusalRecordFor("cell-a", raw, ErrPeerUnauthenticated, time.Unix(0, 0).UTC())
	if rec.Refusal != commsqueue.RefusalPeerUnauthenticated {
		t.Fatalf("kind = %q", rec.Refusal)
	}
	if rec.From != (comms.SenderID{}) || rec.To != (comms.Lane{}) || rec.Verb != "" {
		t.Fatalf("an unauthenticated peer's bytes must not be read for attribution, got %+v", rec)
	}
	if !strings.HasPrefix(rec.ID, "refused-") || rec.ID == "refused-" {
		t.Fatalf("synthetic id expected, got %q", rec.ID)
	}
	if rec.Cell != "cell-a" || rec.RawDigest == "" {
		t.Fatalf("cell + digest must be stamped, got %+v", rec)
	}
}

// TestRefusalRecordStripsAdvisoryMetadata: SenderID.App / .Session are
// unsigned, forgeable metadata (envelope.go) — never journalled, even as
// "presented".
func TestRefusalRecordStripsAdvisoryMetadata(t *testing.T) {
	raw := []byte(`{"schema":"x","id":"m","from":{"cell":"cell-a","role":"the-desk","app":"forged-app","session":"forged-session"},"to":{"cell":"cell-a","role":"worker-desk"},"verb":"handoff"}`)
	rec := RefusalRecordFor("cell-a", raw, fmt.Errorf("%w: %w", ErrEnvelopeParse, comms.ErrBadSchema), time.Now())
	if rec.From.App != "" || rec.From.Session != "" {
		t.Fatalf("advisory app/session must be stripped, got %+v", rec.From)
	}
	if rec.From.Cell != "cell-a" || rec.From.Role != "the-desk" || rec.ID != "m" {
		t.Fatalf("presented cell/role/id must be kept, got %+v", rec)
	}
}

// TestRefusalJournalCarriesGatewayCell: the line is stamped with the
// GATEWAY's cell so that cell's sweep scopes it even when the presented
// addressing is empty (unparseable bytes).
func TestRefusalJournalCarriesGatewayCell(t *testing.T) {
	f := newFixture(t)
	root := t.TempDir()
	s := SocketServer{Root: root, Cell: "cell-a", Deps: f.deps, Now: func() time.Time { return f.now }}
	resp := s.handleSubmit(gwRequest{Op: "submit", Message: []byte(`this is not json at all`)})
	if resp.Receipt == nil || resp.Receipt.Accepted {
		t.Fatalf("garbage must be refused, got %+v", resp)
	}
	_, recs := readRefusalJournal(t, root)
	if len(recs) != 1 {
		t.Fatalf("want 1 line, got %d", len(recs))
	}
	if recs[0].Cell != "cell-a" || recs[0].Refusal != string(commsqueue.RefusalEnvelopeParse) {
		t.Fatalf("line must carry the gateway cell + envelope-parse kind, got %+v", recs[0])
	}
	if recs[0].From != (comms.SenderID{}) || !strings.HasPrefix(recs[0].ID, "refused-") {
		t.Fatalf("unparseable bytes present no sender and get a synthetic id, got %+v", recs[0])
	}
}

// TestRefusalJournalWriteFailureSurfaces: the refusal is never conditional
// on the journal write. With an unwritable root the receipt still refuses
// with the ORIGINAL refusal, and additionally says the journal write failed
// — surfaced, never silently swallowed.
func TestRefusalJournalWriteFailureSurfaces(t *testing.T) {
	f := newFixture(t)
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := SocketServer{Root: notADir, Cell: "cell-a", Deps: f.deps, Now: func() time.Time { return f.now }}
	raw := f.envelope(t, "unwritable", func(we *wireEnvelope) { we.To.Role = "janitor" })
	resp := s.handleSubmit(gwRequest{Op: "submit", Message: raw})
	if resp.Receipt == nil || resp.Receipt.Accepted {
		t.Fatalf("must still be refused, got %+v", resp)
	}
	if !strings.Contains(resp.Receipt.Detail, ErrLaneDenied.Error()) {
		t.Fatalf("the original refusal must lead the detail, got %q", resp.Receipt.Detail)
	}
	if !strings.Contains(resp.Receipt.Detail, "refusal journal write failed") {
		t.Fatalf("the journal write failure must be surfaced on the receipt, got %q", resp.Receipt.Detail)
	}
}

// TestRefusalJournalSocketCarrierMalformed: a socket request line that does
// not decode as a gwRequest is journalled as carrier-malformed through the
// real connection handler.
func TestRefusalJournalSocketCarrierMalformed(t *testing.T) {
	f := newFixture(t)
	root := t.TempDir()
	s := SocketServer{Root: root, Cell: "cell-a", Deps: f.deps, Now: func() time.Time { return f.now }}
	// A short path: Unix socket paths are capped at ~104 bytes on macOS, and
	// t.TempDir() names can exceed that.
	dir, err := os.MkdirTemp("", "gwr")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "gw.sock")
	go func() { _ = s.ListenAndServe(path) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("unix", path); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	resp := socketRawLine(t, path, "{{{ not a request\n")
	if !strings.Contains(resp, "undecodable request") {
		t.Fatalf("want an undecodable-request error, got %q", resp)
	}
	_, recs := readRefusalJournal(t, root)
	if len(recs) != 1 || recs[0].Refusal != string(commsqueue.RefusalCarrierMalformed) || recs[0].Cell != "cell-a" {
		t.Fatalf("want one carrier-malformed line stamped with the cell, got %+v", recs)
	}
}

// socketRawLine dials the Unix socket, writes one raw line and returns the
// one-line reply — the wire-level client the carrier-malformed drill needs
// (a typed client could never send a line that does not decode).
func socketRawLine(t *testing.T, path, line string) string {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(line)); err != nil {
		t.Fatalf("write: %v", err)
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	return reply
}
