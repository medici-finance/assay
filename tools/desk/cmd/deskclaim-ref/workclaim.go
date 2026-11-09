package main

import (
	"os"
	"strings"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Only explicit reference inputs select this path. Opaque issue/PR/historical
// transport keys retain their grammar; arbitrary -- segments are never parsed.
func workClaimInput(id string) bool {
	return strings.ContainsAny(id, ":/@") || strings.Contains(id, "--b-") || strings.Contains(id, "--m-")
}

var loadClaimWork = func(repository string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
	root, err := os.Getwd()
	if err != nil {
		return deskkit.WorkCatalog{}, nil, false, err
	}
	catalog, err := deskkit.LocalWorkCatalog(root, repository)
	peers, complete := deskkit.LocalWorkPeers()
	return catalog, peers, complete, err
}

// subjectClaimStore is the lower effect boundary, not a new store. Every write
// validates selection/capabilities again. Existing resolver, credential custody,
// holder/TTL checks and exact-old server CAS remain in their existing owners.
type subjectClaimStore struct {
	deskkit.ClaimStore
	selected   deskkit.WorkSelection
	peers      []readercaps.Observation
	complete   bool
	repository string
	cause      string
}

func (s *subjectClaimStore) admitted() bool {
	err := s.selected.Validate()
	if err == nil && s.selected.Subject.Repository != s.repository {
		err = deskkit.Refused("claim repository differs from selected owning repository")
	}
	if err == nil {
		err = deskkit.CheckWorkOperation("claim", s.peers, s.complete)
	}
	if err != nil {
		s.cause = err.Error()
		return false
	}
	return true
}

func (s *subjectClaimStore) unique(id string) bool {
	if id != s.selected.ClaimKey {
		s.cause = "physical claim key mismatch"
		return false
	}
	ids, status := s.ClaimStore.List()
	if status == deskkit.ClaimReadUnverifiable {
		s.cause = "incomplete alias enumeration"
		return false
	}
	aliases := map[string]bool{}
	for _, alias := range s.selected.Aliases {
		if alias != id {
			aliases[alias] = true
		}
	}
	for _, other := range ids {
		if !aliases[other] {
			continue
		}
		ref, status := s.ClaimStore.Read(other)
		if status == deskkit.ClaimReadFree {
			continue
		}
		// An old physical key is never rewritten, reclaimed or deleted here. Even an
		// expired/unreadable alias holds; migration requires its separate admission.
		s.cause = "historical physical alias held: " + other + " " + fieldOf(ref.Msg, "owner")
		return false
	}
	return true
}

func (s *subjectClaimStore) Read(id string) (deskkit.ClaimStoreRecord, deskkit.ClaimReadStatus) {
	if !s.admitted() || !s.unique(id) {
		return deskkit.ClaimStoreRecord{}, deskkit.ClaimReadUnverifiable
	}
	ref, status := s.ClaimStore.Read(id)
	if status != deskkit.ClaimReadHeld {
		return ref, status
	}
	fields := strings.Fields(ref.Msg)
	if len(fields) < 2 || fields[1] != id {
		s.cause = "claim payload key mismatch"
		return ref, deskkit.ClaimReadUnverifiable
	}
	subject, present, err := deskkit.DecodeClaimSubject(ref.Msg)
	if err != nil || !present || subject != s.selected.Subject {
		s.cause = "existing claim subject is unknown, malformed or conflicting"
		return ref, deskkit.ClaimReadUnverifiable
	}
	return ref, status
}

func (s *subjectClaimStore) CreateIfAbsent(id, msg string) deskkit.ClaimWriteOutcome {
	if !s.admitted() || !s.unique(id) {
		return deskkit.ClaimWriteUnverifiable
	}
	msg, err := deskkit.EncodeClaimSubject(msg, s.selected.Subject)
	if err != nil {
		s.cause = err.Error()
		return deskkit.ClaimWriteUnverifiable
	}
	outcome := s.ClaimStore.CreateIfAbsent(id, msg)
	if outcome == deskkit.ClaimWriteApplied && !s.unique(id) {
		// Never Remove(id): a second owner could have advanced the newly created ref.
		// Retain the claim and hold dispatch; only exact-version cleanup is safe.
		return deskkit.ClaimWriteUnverifiable
	}
	return outcome
}

func (s *subjectClaimStore) UpdateFrom(id, old, msg string) deskkit.ClaimWriteOutcome {
	if !s.admitted() {
		return deskkit.ClaimWriteUnverifiable
	}
	ref, status := s.Read(id)
	if status != deskkit.ClaimReadHeld {
		return deskkit.ClaimWriteUnverifiable
	}
	if ref.Version != old {
		return deskkit.ClaimWriteRejected
	}
	msg, err := deskkit.EncodeClaimSubject(msg, s.selected.Subject)
	if err != nil {
		s.cause = err.Error()
		return deskkit.ClaimWriteUnverifiable
	}
	// Exact old token is passed through; alias equality never restores custody.
	return s.ClaimStore.UpdateFrom(id, old, msg)
}

func (s *subjectClaimStore) Remove(string) (deskkit.ClaimWriteOutcome, bool) {
	s.cause = "negotiated resolution does not authorize claim removal"
	return deskkit.ClaimWriteUnverifiable, false
}
func (s *subjectClaimStore) TransportCause() string {
	if s.cause != "" {
		return s.cause
	}
	return s.ClaimStore.TransportCause()
}

// An opaque physical-key invocation has no selected source context. It must not
// erase a negotiated subject by advancing/reclaiming through the legacy writer.
func legacySubjectHold(message string) bool {
	if _, selected := store.(*subjectClaimStore); selected {
		return false
	}
	reserved := false
	for _, field := range strings.Fields(message) {
		if strings.HasPrefix(field, "subject") || strings.Contains(field, "subject-v") {
			reserved = true
		}
	}
	if !reserved {
		return false
	}
	_, present, err := deskkit.DecodeClaimSubject(message)
	return present || err != nil
}
