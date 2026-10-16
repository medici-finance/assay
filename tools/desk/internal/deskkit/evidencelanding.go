package deskkit

import (
	"net/http"
)

// EvidenceLandingForge scopes read reuse to ONE evidence publication. It retains
// the checked forge and its custody; other tools and read-only transports are
// unchanged. GitHub batches admission and destination reads. GitLab reuses the
// admitted issue and destination while retaining its native provenance reads and
// protected-default-branch/MR path. Nothing is cached across publications.
//
// The first read still belongs to admission: local verifier checks run before it,
// and destination bytes are not consumed until the attestation has been admitted.
func EvidenceLandingForge(f Forge, repo ForgeRepo, file, branch string) Forge {
	o, ok := f.(*outboundForge)
	if !ok {
		return f // In particular, never unwrap a read-only credential fence.
	}
	switch o.Forge.(type) {
	case *GitHubForge, *GitLabForge:
		return OutboundChecked(&evidenceLandingForge{Forge: o.Forge, repo: repo, file: file, branch: branch}, o.role)
	default:
		return f
	}
}

type evidenceLandingForge struct {
	Forge
	repo         ForgeRepo
	file, branch string
	issue        *Issue
	prior        *FileContent
	absent       bool
	snapshot     *githubEvidenceSnapshot
	gitlab       *gitlabEvidenceSnapshot
	attempted    bool
}

func (e *evidenceLandingForge) GetIssueTyped(repo ForgeRepo, number int, kind TargetKind) (*Issue, error) {
	if repo != e.repo || kind != TargetIssue {
		return e.Forge.GetIssueTyped(repo, number, kind)
	}
	if e.issue != nil && e.issue.Number == number {
		copy := *e.issue
		copy.Labels = append([]string(nil), e.issue.Labels...)
		return &copy, nil
	}
	if e.issue == nil && !e.attempted {
		if g, ok := e.Forge.(*GitHubForge); ok {
			s, err := g.evidenceSnapshot(repo, number, e.file, e.branch)
			if err != nil {
				return nil, err
			}
			e.snapshot, e.issue, e.prior, e.absent = s, s.issue, s.file, s.file == nil
			return e.GetIssueTyped(repo, number, kind)
		}
	}
	issue, err := e.Forge.GetIssueTyped(repo, number, kind)
	if err == nil && issue != nil && e.issue == nil {
		copy := *issue
		copy.Labels = append([]string(nil), issue.Labels...)
		e.issue = &copy
	}
	return issue, err
}

func (e *evidenceLandingForge) IssueTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error) {
	if e.snapshot != nil && repo == e.repo && number == e.issue.Number {
		copy := e.snapshot.trust
		copy.Events = append([]ContentEvent(nil), copy.Events...)
		return &copy, nil
	}
	return e.Forge.IssueTrustEvents(repo, number)
}

func (e *evidenceLandingForge) ListIssueLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error) {
	if e.snapshot != nil && repo == e.repo && number == e.issue.Number && !e.snapshot.moreEvents {
		return append([]LabelEvent(nil), e.snapshot.events...), nil
	}
	// An overflowing timeline is walked by the existing complete reader, never
	// silently truncated to meet a request-count target.
	return e.Forge.ListIssueLabelEvents(repo, number)
}

func (e *evidenceLandingForge) RepoVisibility(repo ForgeRepo) (string, error) {
	if e.snapshot != nil && repo == e.repo {
		return e.snapshot.visibility, nil
	}
	if g, ok := e.Forge.(*GitLabForge); ok && repo == e.repo && !e.attempted {
		if e.gitlab == nil {
			s, err := g.evidenceProject(repo)
			if err != nil {
				return "", err
			}
			e.gitlab = s
		}
		return string(e.gitlab.project.Visibility), nil
	}
	return e.Forge.RepoVisibility(repo)
}

func (e *evidenceLandingForge) ReadFile(repo ForgeRepo, in ReadFileInput) (*FileContent, error) {
	if !e.attempted && repo == e.repo && in.File == e.file && in.Ref == e.branch {
		if e.absent {
			return nil, &ForgeAPIError{Status: http.StatusNotFound, Method: http.MethodGet, Path: e.file}
		}
		if e.prior != nil {
			return cloneLandingFile(e.prior), nil
		}
		f, err := e.Forge.ReadFile(repo, in)
		if IsForgeNotFound(err) {
			e.absent = true
		}
		if err == nil && f != nil {
			if f.Exists && f.SHA == "" {
				return nil, Unverifiable("evidence destination has no conditional-write revision", nil)
			}
			e.prior = cloneLandingFile(f)
		}
		return f, err
	}
	return e.Forge.ReadFile(repo, in)
}

func cloneLandingFile(f *FileContent) *FileContent {
	copy := *f
	copy.Content = append([]byte(nil), f.Content...)
	return &copy
}

func (e *evidenceLandingForge) WriteFile(repo ForgeRepo, in WriteFileInput) (*WriteFileResult, error) {
	if e.attempted {
		return nil, Refused("evidence publication already attempted; obtain a fresh snapshot and reconcile the previous outcome")
	}
	// The cached bytes were read at e.branch, so they serve only a write whose
	// probe ref is exactly that branch: a direct write to it, or a side branch
	// started from it. Any other start branch keeps the native probe.
	if g, ok := e.Forge.(*GitLabForge); ok && e.gitlab != nil && repo == e.repo && in.File == e.file &&
		gitlabWriteProbeRef(in) == e.branch && (e.prior != nil || e.absent) {
		if e.prior != nil && e.prior.Exists {
			if err := expectedSHAPrecondition(in, true, e.prior.SHA); err != nil {
				return nil, err
			}
			in.ExpectedSHA = e.prior.SHA
		}
		// The default-branch sentinel makes no request and consumes no write.
		// Preserve the snapshot for the tool's existing side-branch/MR path.
		if in.Branch != e.gitlab.project.DefaultBranch {
			e.attempted = true
		}
		return g.writeEvidenceFile(repo, in, e.gitlab.project, e.prior, true)
	}
	if e.snapshot == nil || repo != e.repo || in.File != e.file || in.Branch != e.branch || in.StartBranch != "" {
		// GitLab retains its native write path, but every update must remain
		// conditional on the destination the shared validators actually saw.
		if repo == e.repo && in.File == e.file && (in.Branch == e.branch || in.StartBranch == e.branch) && e.prior != nil && e.prior.Exists {
			if err := expectedSHAPrecondition(in, true, e.prior.SHA); err != nil {
				return nil, err
			}
			in.ExpectedSHA = e.prior.SHA
		}
		return e.Forge.WriteFile(repo, in)
	}
	// A write or lost acknowledgement consumes this snapshot. Never replay it.
	e.attempted = true
	g := e.Forge.(*GitHubForge)
	res, err := g.writeEvidenceFileFromSnapshot(repo, in, e.prior)
	if err != nil {
		return nil, Unverifiable("conditional evidence write not confirmed; reconcile the destination before retrying", err)
	}
	return res, nil
}
