package deskkit

// claimstore_conformance_test.go — ONE table every ClaimStore backend passes (spec §14 V1).
//
// The table is written against the seam's contract, not against any backend: a later backend
// (the file store, the served store) is added by appending one entry to claimStoreBackendsUnderTest
// and nothing else — a backend, not a test design. It drives the seven storage methods through
// every transition the claim verbs make: acquire, refuse-live, reclaim-stale, progress,
// release, steal, and the record and listing the `show` / `list` output is rendered from.
//
// SHOW / LIST BYTE PARITY. The claim tool renders `show` as
// `HELD <id> — <Msg> at=<Date> age=<n>m` and `list` as one `show` per listed id; both are a
// pure function of the record a store returns (and of the clock). So byte parity across
// backends is parity of the RECORD: Msg returned byte-for-byte as written, Date in exactly
// one form (UTC, RFC3339, whole seconds), and List naming exactly the held ids. The table pins
// those, and the claim tool's own suite pins the rendering above them.

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// memForgeClaimStore is the in-memory double of the forge-ref store: a claim is a ref whose
// value is a tag object id, placed with a server-side compare-and-swap — a create is accepted
// only from "absent", an update only from the exact tag id last read. It models the forge's
// semantics, not its transport (the go-git transport is proven against a local git server in
// internal/gitcore/claimref_test.go).
type memForgeClaimStore struct {
	refs     map[string]ClaimStoreRecord
	branches map[string]bool
	n        int
	clock    func() time.Time
}

func newMemForgeClaimStore(clock func() time.Time) *memForgeClaimStore {
	if clock == nil {
		clock = time.Now
	}
	return &memForgeClaimStore{refs: map[string]ClaimStoreRecord{}, branches: map[string]bool{}, clock: clock}
}

var _ ClaimStore = (*memForgeClaimStore)(nil)

func (m *memForgeClaimStore) mint(msg string) ClaimStoreRecord {
	m.n++
	return ClaimStoreRecord{
		Version: fmt.Sprintf("tag%040d", m.n),
		Msg:     msg,
		Date:    m.clock().UTC().Format(time.RFC3339),
	}
}

func (m *memForgeClaimStore) Read(id string) (ClaimStoreRecord, ClaimReadStatus) {
	if r, ok := m.refs[id]; ok {
		return r, ClaimReadHeld
	}
	return ClaimStoreRecord{}, ClaimReadFree
}

func (m *memForgeClaimStore) CreateIfAbsent(id, msg string) ClaimWriteOutcome {
	if _, ok := m.refs[id]; ok {
		return ClaimWriteRejected
	}
	m.refs[id] = m.mint(msg)
	return ClaimWriteApplied
}

func (m *memForgeClaimStore) UpdateFrom(id, oldVersion, msg string) ClaimWriteOutcome {
	r, ok := m.refs[id]
	if !ok || r.Version != oldVersion {
		return ClaimWriteRejected
	}
	m.refs[id] = m.mint(msg)
	return ClaimWriteApplied
}

func (m *memForgeClaimStore) Remove(id string) (ClaimWriteOutcome, bool) {
	_, existed := m.refs[id]
	delete(m.refs, id)
	return ClaimWriteApplied, existed
}

func (m *memForgeClaimStore) List() ([]string, ClaimReadStatus) {
	ids := make([]string, 0, len(m.refs))
	for id := range m.refs {
		ids = append(ids, id)
	}
	return ids, ClaimReadHeld
}

func (m *memForgeClaimStore) BranchExists(branch string) (bool, bool) {
	if branch == "" || branch == "-" {
		return false, true
	}
	return m.branches[branch], true
}

func (m *memForgeClaimStore) TransportCause() string { return "" }

// conformanceBackend is one backend under the table: a constructor taking the clock the store
// stamps claims with, and a hook that makes a branch exist on the backend's remote.
type conformanceBackend struct {
	name       string
	open       func(t *testing.T, clock func() time.Time) ClaimStore
	pushBranch func(s ClaimStore, branch string)
}

// claimStoreBackendsUnderTest — append a backend here; the table does the rest.
var claimStoreBackendsUnderTest = []conformanceBackend{
	{
		name: ClaimStoreForgeRef + " (in-memory double)",
		open: func(_ *testing.T, clock func() time.Time) ClaimStore { return newMemForgeClaimStore(clock) },
		pushBranch: func(s ClaimStore, branch string) {
			s.(*memForgeClaimStore).branches[branch] = true
		},
	},
}

// testClock is a settable clock for the table.
type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// holder builds a holder encoding exactly as the claim tool does.
func holder(id, owner, state, branch, note string) string {
	if branch == "" {
		branch = "-"
	}
	msg := "dispatch-claim " + id + " owner=" + owner + " state=" + state + " branch=" + branch
	if note != "" {
		msg += " note=" + note
	}
	return msg
}

func TestClaimStoreConformance(t *testing.T) {
	const id = "example--stream--07"
	start := time.Date(2026, 9, 17, 10, 0, 0, 123456789, time.FixedZone("east", 3*3600))

	for _, b := range claimStoreBackendsUnderTest {
		b := b
		t.Run(b.name, func(t *testing.T) {
			fresh := func(t *testing.T) (ClaimStore, *testClock) {
				clk := &testClock{now: start}
				return b.open(t, clk.Now), clk
			}
			mustRead := func(t *testing.T, s ClaimStore, id string) ClaimStoreRecord {
				t.Helper()
				r, st := s.Read(id)
				if st != ClaimReadHeld {
					t.Fatalf("Read(%s) status = %v, want held", id, st)
				}
				return r
			}

			t.Run("acquire places the claim and returns the record byte-for-byte", func(t *testing.T) {
				s, clk := fresh(t)
				if _, st := s.Read(id); st != ClaimReadFree {
					t.Fatalf("a fresh store reads %v for %s, want free", st, id)
				}
				msg := holder(id, "sess-A", "claimed", "feat/x", "")
				if got := s.CreateIfAbsent(id, msg); got != ClaimWriteApplied {
					t.Fatalf("CreateIfAbsent on a free key = %v, want applied", got)
				}
				r := mustRead(t, s, id)
				if r.Msg != msg {
					t.Errorf("Msg = %q, want %q (byte-for-byte)", r.Msg, msg)
				}
				if want := clk.Now().UTC().Format(time.RFC3339); r.Date != want {
					t.Errorf("Date = %q, want %q (UTC, RFC3339, whole seconds — the show line's at=)", r.Date, want)
				}
				if r.Version == "" {
					t.Error("a held claim carries no compare-and-swap version")
				}
				if c := s.TransportCause(); c != "" {
					t.Errorf("TransportCause after clean operations = %q, want empty", c)
				}
			})

			t.Run("refuse-live: a second create is rejected and the holder is untouched", func(t *testing.T) {
				s, _ := fresh(t)
				first := holder(id, "sess-A", "claimed", "", "")
				s.CreateIfAbsent(id, first)
				before := mustRead(t, s, id)
				if got := s.CreateIfAbsent(id, holder(id, "sess-B", "claimed", "", "")); got != ClaimWriteRejected {
					t.Fatalf("CreateIfAbsent over a live holder = %v, want rejected", got)
				}
				if after := mustRead(t, s, id); after != before {
					t.Fatalf("a rejected create changed the holder: %+v -> %+v", before, after)
				}
			})

			t.Run("reclaim-stale: the age is readable and a reclaim from the version read wins", func(t *testing.T) {
				s, clk := fresh(t)
				s.CreateIfAbsent(id, holder(id, "dead-sess", "claimed", "", ""))
				clk.Advance(25 * time.Minute)
				r := mustRead(t, s, id)
				placed, err := time.Parse(time.RFC3339, r.Date)
				if err != nil {
					t.Fatalf("Date %q does not parse: %v", r.Date, err)
				}
				if age := clk.Now().Sub(placed); age < 25*time.Minute || age >= 26*time.Minute {
					t.Fatalf("claim age computed from Date = %v, want 25m — the TTL cannot be judged", age)
				}
				reclaim := holder(id, "sess-N", "claimed", "", "TTL:_state=claimed_age=25m_>=_20m")
				if got := s.UpdateFrom(id, r.Version, reclaim); got != ClaimWriteApplied {
					t.Fatalf("reclaim from the version read = %v, want applied", got)
				}
				if got := mustRead(t, s, id); got.Msg != reclaim || got.Date != clk.Now().UTC().Format(time.RFC3339) {
					t.Fatalf("after reclaim the record is %+v", got)
				}
			})

			t.Run("progress: the holder advances from its version and the version moves", func(t *testing.T) {
				s, clk := fresh(t)
				s.CreateIfAbsent(id, holder(id, "sess-A", "claimed", "", ""))
				r := mustRead(t, s, id)
				clk.Advance(time.Minute)
				adv := holder(id, "sess-A", "dispatched", "feat/x", "")
				if got := s.UpdateFrom(id, r.Version, adv); got != ClaimWriteApplied {
					t.Fatalf("progress = %v, want applied", got)
				}
				r2 := mustRead(t, s, id)
				if r2.Msg != adv || r2.Version == r.Version {
					t.Fatalf("after progress: %+v (was %+v) — msg not replaced or version not moved", r2, r)
				}
				if got := s.UpdateFrom(id, r.Version, holder(id, "sess-A", "dispatched", "feat/y", "")); got != ClaimWriteRejected {
					t.Fatalf("an update from a superseded version = %v, want rejected (the compare-and-swap)", got)
				}
			})

			t.Run("steal: a takeover from the version read wins, a racing one loses", func(t *testing.T) {
				s, _ := fresh(t)
				s.CreateIfAbsent(id, holder(id, "old", "dispatched", "", ""))
				r := mustRead(t, s, id)
				if got := s.UpdateFrom(id, r.Version, holder(id, "thief-1", "claimed", "", "TTL_dead")); got != ClaimWriteApplied {
					t.Fatalf("steal = %v, want applied", got)
				}
				if got := s.UpdateFrom(id, r.Version, holder(id, "thief-2", "claimed", "", "also_dead")); got != ClaimWriteRejected {
					t.Fatalf("a racing steal from the same stale version = %v, want rejected", got)
				}
				if got := mustRead(t, s, id); fieldOfHolder(got.Msg, "owner") != "thief-1" {
					t.Fatalf("the losing steal clobbered the winner: %q", got.Msg)
				}
				if got := s.UpdateFrom("example--issue-9", "anything", holder("example--issue-9", "x", "claimed", "", "")); got != ClaimWriteRejected {
					t.Fatalf("an update of a key with no claim = %v, want rejected", got)
				}
			})

			t.Run("release: remove reports it existed, then is an idempotent no-op", func(t *testing.T) {
				s, _ := fresh(t)
				s.CreateIfAbsent(id, holder(id, "sess-A", "dispatched", "", ""))
				if out, existed := s.Remove(id); out != ClaimWriteApplied || !existed {
					t.Fatalf("Remove of a held claim = (%v, %t), want (applied, true)", out, existed)
				}
				if _, st := s.Read(id); st != ClaimReadFree {
					t.Fatalf("after release Read = %v, want free", st)
				}
				if out, existed := s.Remove(id); out != ClaimWriteApplied || existed {
					t.Fatalf("Remove of an absent claim = (%v, %t), want (applied, false)", out, existed)
				}
				if got := s.CreateIfAbsent(id, holder(id, "sess-B", "claimed", "", "")); got != ClaimWriteApplied {
					t.Fatalf("a create after release = %v, want applied", got)
				}
			})

			t.Run("list names exactly the held claims", func(t *testing.T) {
				s, _ := fresh(t)
				if ids, st := s.List(); st != ClaimReadHeld || len(ids) != 0 {
					t.Fatalf("an empty store lists (%v, %v), want (none, verified)", ids, st)
				}
				held := []string{"example--stream--07", "example--issue-5", "other--stream--01"}
				for _, k := range held {
					s.CreateIfAbsent(k, holder(k, "s", "claimed", "", ""))
				}
				s.Remove("other--stream--01")
				ids, st := s.List()
				if st != ClaimReadHeld {
					t.Fatalf("List status = %v, want verified", st)
				}
				sort.Strings(ids)
				want := []string{"example--issue-5", "example--stream--07"}
				if fmt.Sprint(ids) != fmt.Sprint(want) {
					t.Fatalf("List = %v, want %v", ids, want)
				}
			})

			t.Run("branch-as-claim is readable", func(t *testing.T) {
				s, _ := fresh(t)
				if exists, ok := s.BranchExists(""); exists || !ok {
					t.Fatalf("BranchExists(\"\") = (%t, %t), want (false, true)", exists, ok)
				}
				if exists, ok := s.BranchExists("feat/live"); exists || !ok {
					t.Fatalf("BranchExists on an absent branch = (%t, %t), want (false, true)", exists, ok)
				}
				b.pushBranch(s, "feat/live")
				if exists, ok := s.BranchExists("feat/live"); !exists || !ok {
					t.Fatalf("BranchExists on a pushed branch = (%t, %t), want (true, true)", exists, ok)
				}
			})
		})
	}
}

// fieldOfHolder pulls key=value out of a holder encoding.
func fieldOfHolder(msg, key string) string {
	for _, tok := range strings.Fields(msg) {
		if v, ok := strings.CutPrefix(tok, key+"="); ok {
			return v
		}
	}
	return ""
}
