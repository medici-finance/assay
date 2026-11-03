package deskkit

import (
	"context"
	"strings"

	"github.com/medici-finance/assay/forgeread/adapters"
	ghupdates "github.com/medici-finance/assay/forgeread/adapters/github"
	glupdates "github.com/medici-finance/assay/forgeread/adapters/gitlab"
)

// MirrorRecordKind selects one enumerated ingestion operation, never an endpoint.
type MirrorRecordKind string

const (
	MirrorIssues  MirrorRecordKind = "issues"
	MirrorChanges MirrorRecordKind = "changes"
)

// MirrorReadSource binds the incremental reads to an already-admitted backend
// and repository resolution supplied by trusted composition. It resolves no
// credential and implements no mirror coverage, retention or refresh policy.
type MirrorReadSource struct {
	forge    Forge
	repo     ForgeRepo
	provider ForgeKind
	instance string
}

// NewMirrorReadSource takes the existing resolution; a read request cannot choose
// another repository, role, host, endpoint or credential. Backend admission still
// independently rejects an out-of-scope repo even if this caller is bypassed.
func NewMirrorReadSource(f Forge, resolved ForgeResolution) (*MirrorReadSource, error) {
	if f == nil || !ValidRepoSlug(resolved.Repo.Slug()) || (resolved.Kind != ForgeGitHub && resolved.Kind != ForgeGitLab) {
		return nil, Refused("incremental mirror source requires an admitted backend and explicit repository/provider resolution")
	}
	// Identity inspection may unwrap the explicit read-only decorator; the
	// operational source retains f, so every read still passes through the fence.
	identityBackend := f
	if ro, ok := f.(*readOnlyForge); ok {
		identityBackend = ro.inner
	}
	kind, instance, err := ForgeReadIdentity(identityBackend, resolved.Repo)
	if err != nil || kind != resolved.Kind {
		return nil, Refused("incremental mirror source resolution does not match its admitted backend")
	}
	return &MirrorReadSource{forge: f, repo: resolved.Repo, provider: kind, instance: instance}, nil
}

// MirrorReadResult retains the exact query/source and only the requested endpoint
// population. Complete listing coverage does not imply complete reviews/checks.
type MirrorReadResult struct {
	Kind     MirrorRecordKind
	Instance string
	adapters.UpdateCoverage
	Issues  []adapters.IssueUpdate
	Changes []adapters.ChangeUpdate
}

// Read concretely consumes the two incremental Forge methods. It validates
// returned source provenance before any observation escapes to later integration.
func (s *MirrorReadSource) Read(ctx context.Context, kind MirrorRecordKind, q adapters.UpdateQuery) (MirrorReadResult, error) {
	if s == nil || s.forge == nil {
		return MirrorReadResult{}, Refused("incremental mirror source is not configured")
	}
	out := MirrorReadResult{Kind: kind, Instance: s.instance}
	switch kind {
	case MirrorIssues:
		r, err := s.forge.IssuesUpdatedSince(ctx, s.repo, q)
		if err != nil {
			return MirrorReadResult{}, err
		}
		out.UpdateCoverage = r.UpdateCoverage
		out.Issues = r.Records
	case MirrorChanges:
		r, err := s.forge.ChangesUpdatedSince(ctx, s.repo, q)
		if err != nil {
			return MirrorReadResult{}, err
		}
		out.UpdateCoverage = r.UpdateCoverage
		out.Changes = r.Records
	default:
		return MirrorReadResult{}, Refused("incremental mirror source record kind is unsupported")
	}
	valid := func(provider, repo string, n int) bool {
		return provider == string(s.provider) && strings.EqualFold(repo, s.repo.Slug()) && n > 0
	}
	if out.Provider != string(s.provider) || !strings.EqualFold(out.Repository, s.repo.Slug()) || !out.Since.Equal(q.Since) || out.PageCount < 1 || out.ConditionalSupported {
		return MirrorReadResult{}, Unverifiable("incremental mirror source returned incompatible source/query provenance", nil)
	}
	for _, r := range out.Issues {
		if !valid(r.Provider, r.Repository, r.Number) {
			return MirrorReadResult{}, Unverifiable("incremental mirror issue has incompatible provenance", nil)
		}
	}
	for _, r := range out.Changes {
		wantKind := "pull-request"
		if s.provider == ForgeGitLab {
			wantKind = "merge-request"
		}
		if !valid(r.Provider, r.Repository, r.Number) || r.Kind != wantKind {
			return MirrorReadResult{}, Unverifiable("incremental mirror change has incompatible provenance", nil)
		}
	}
	return out, nil
}

// These methods delegate to the existing admitted Access; the adapters own the
// fixed request shape, pagination and reduction for both providers.
func (g *GitHubForge) ChangesUpdatedSince(ctx context.Context, repo ForgeRepo, q adapters.UpdateQuery) (adapters.ChangeUpdates, error) {
	return ghupdates.New(WithForgeReadContext(g, ctx).(*GitHubForge).sharedAccess(), nil).ChangesUpdatedSince(ctx, repo.Slug(), q)
}

func (g *GitHubForge) IssuesUpdatedSince(ctx context.Context, repo ForgeRepo, q adapters.UpdateQuery) (adapters.IssueUpdates, error) {
	return ghupdates.New(WithForgeReadContext(g, ctx).(*GitHubForge).sharedAccess(), nil).IssuesUpdatedSince(ctx, repo.Slug(), q)
}

func (g *GitLabForge) ChangesUpdatedSince(ctx context.Context, repo ForgeRepo, q adapters.UpdateQuery) (adapters.ChangeUpdates, error) {
	return glupdates.New(WithForgeReadContext(g, ctx).(*GitLabForge).sharedAccess(), nil).ChangesUpdatedSince(ctx, repo.Slug(), q)
}

func (g *GitLabForge) IssuesUpdatedSince(ctx context.Context, repo ForgeRepo, q adapters.UpdateQuery) (adapters.IssueUpdates, error) {
	return glupdates.New(WithForgeReadContext(g, ctx).(*GitLabForge).sharedAccess(), nil).IssuesUpdatedSince(ctx, repo.Slug(), q)
}
