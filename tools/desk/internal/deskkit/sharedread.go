package deskkit

// sharedread.go — the trusted composition that hands the shared read adapters
// (forgeread/adapters/*) their Access (forge-neutral brief 36).
//
// The adapters own the request shape, the pagination walk and the reduction to the shared issue
// type. They own NOTHING about credentials: they read through an adapters.Access, and this file is
// where an Access is built from a backend that has ALREADY resolved its credential, validated its
// host, and been admitted to read (a minted App token through ForgeFor, or the CI workflow token
// through ReadOnlyForgeForCIToken). So the credential, the host and repository validation, and the
// activation decision all stay at the boundary that owns them.
//
// THE LOWER-LAYER SCOPE CHECK. scopedAccess is the second layer behind every consumer-side repo
// check. It validates the repository coordinate and, when the credential was admitted for ONE
// repository (the CI workflow token is the job's own repository), refuses any other repository
// BEFORE a request is built — so a caller that skips the consumer-side check, or hands the shared
// adapter a repository the credential was never admitted for, reaches the provider zero times.
// TestSharedReadLowerScope proves it and fails when this check is removed.

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/adapters"
)

// scopedAccess narrows an inner page getter to the repositories its credential was admitted for.
type scopedAccess struct {
	// only, when non-empty, is the one owner/name this credential may read (case-insensitive).
	// Empty means the credential carries no narrower scope than its own installation, which the
	// provider enforces.
	only string
	next func(ctx context.Context, repo ForgeRepo, rel string, q url.Values) (adapters.Page, error)
}

var _ adapters.Access = scopedAccess{}

func (s scopedAccess) GetPage(ctx context.Context, repo, rel string, q url.Values) (adapters.Page, error) {
	if !ValidRepoSlug(repo) {
		return adapters.Page{}, Refused(fmt.Sprintf("shared read: %q is not one owner/name repository coordinate", repo))
	}
	if s.only != "" && !strings.EqualFold(repo, s.only) {
		return adapters.Page{}, Refused(fmt.Sprintf("shared read: the credential was admitted for %q only; "+
			"%q is outside its scope", s.only, repo))
	}
	owner, name, _ := strings.Cut(repo, "/")
	return s.next(ctx, ForgeRepo{Owner: owner, Name: name}, rel, q)
}

// sharedIssueSkip is the composition's listing policy, injected into the adapters: the desk's own
// pre-work attestation records (verifier and worker) are not open work and never listed.
func sharedIssueSkip(title, authorLogin string) bool {
	return IsAttestationRecord(title, authorLogin)
}

// issueFromShared converts the shared issue type to the interface's IssueSummary.
func issueFromShared(is forgeread.Issue) IssueSummary {
	return IssueSummary{
		Number: is.Number, Title: is.Title,
		Author:    Account{Login: is.AuthorLogin, ID: is.AuthorID},
		Labels:    is.Labels,
		CreatedAt: is.CreatedAt, UpdatedAt: is.UpdatedAt, URL: is.URL,
	}
}

// sharedIssuesToSummaries maps an adapter's answer (or its refusal) to the Forge method's
// contract: a truncation becomes the could-not-check refusal the interface has always returned.
func sharedIssuesToSummaries(issues []forgeread.Issue, err error) ([]IssueSummary, error) {
	if err != nil {
		if adapters.IsTruncated(err) {
			de := Unverifiable(err.Error(), nil)
			de.ReadClass = string(forgeread.ClassTruncated)
			return nil, de
		}
		return nil, err
	}
	out := make([]IssueSummary, 0, len(issues))
	for _, is := range issues {
		out = append(out, issueFromShared(is))
	}
	return out, nil
}
