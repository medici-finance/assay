package deskkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/gitcore"
)

// ForgeClaimLease exposes the two git operations needed to release a claim.
// Both resolve only the exact, unpeeled ref in the active dispatch namespace.
type ForgeClaimLease struct {
	Endpoint gitcore.ListOpts
	Trace    func(gitcore.RefReceipt)
	cause    error
}

func (s *ForgeClaimLease) TransportCause() string {
	if s.cause == nil {
		return ""
	}
	return s.cause.Error()
}

func (s *ForgeClaimLease) Read(key string) (ClaimStoreRecord, ClaimReadStatus) {
	ref := plumbing.ReferenceName(DispatchClaimActiveRefsPrefix + key)
	if err := ref.Validate(); err != nil {
		s.cause = err
		return ClaimStoreRecord{}, ClaimReadUnverifiable
	}
	refs, err := gitcore.List(s.Endpoint)
	if err != nil {
		s.cause = err
		return ClaimStoreRecord{}, ClaimReadUnverifiable
	}
	for _, r := range refs {
		if r.Name() != ref {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		p, err := gitcore.FetchTagPayload(ctx, s.Endpoint.URL, s.Endpoint.Auth, r.Hash())
		if err != nil {
			s.cause = err
			return ClaimStoreRecord{}, ClaimReadUnverifiable
		}
		return ClaimStoreRecord{Version: r.Hash().String(), Msg: strings.TrimRight(p.Message, "\n"), Date: p.When.UTC().Format(time.RFC3339)}, ClaimReadHeld
	}
	return ClaimStoreRecord{}, ClaimReadFree
}

func (s *ForgeClaimLease) RemoveFrom(key, expected string) ClaimWriteOutcome {
	ref := plumbing.ReferenceName(DispatchClaimActiveRefsPrefix + key)
	if err := ref.Validate(); err != nil {
		s.cause = err
		return ClaimWriteUnverifiable
	}
	if !plumbing.IsHash(expected) || plumbing.NewHash(expected).IsZero() {
		s.cause = fmt.Errorf("invalid expected claim revision")
		return ClaimWriteUnverifiable
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, reason, err := gitcore.PushRefUpdateDetail(ctx, gitcore.RefUpdate{
		URL: s.Endpoint.URL, Auth: s.Endpoint.Auth, Ref: ref, Old: plumbing.NewHash(expected), New: plumbing.ZeroHash, Trace: s.Trace,
	})
	if err != nil {
		s.cause = err
		return ClaimWriteUnverifiable
	}
	if result == gitcore.RefUpdateApplied {
		return ClaimWriteApplied
	}
	// A policy or transport refusal alone does not establish that a successor exists.
	current, status := s.Read(key)
	s.cause = fmt.Errorf("server refused %s: %s", ref, reason)
	if status == ClaimReadFree || (status == ClaimReadHeld && current.Version != expected) {
		return ClaimWriteRejected
	}
	return ClaimWriteUnverifiable
}
