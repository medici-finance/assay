package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/packet"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

type preparationForge interface {
	reviewRoundForge
	reviewPacketForge
}

// A context lives for one dispatch only. Mutable reads are shared during assembly,
// never across the terminal check or across assignments. Errors are not memoized.
type preparedReview struct {
	preparationForge
	source           preparationForge
	packetStarted    bool
	omissions        map[rf.SourceKey]string
	repo             deskkit.ForgeRepo
	context          rf.ReviewContext
	cancel           context.CancelFunc
	local            *gitcore.Repo
	change           *deskkit.PullRequest
	reviews          []deskkit.Review
	reviewsRead      bool
	checks           *deskkit.ChecksAtHead
	files            []deskkit.ChangedFile
	filesRead        bool
	content          map[rf.SourceKey]*deskkit.FileContent
	comparisons      map[rf.SourceKey]*deskkit.RefComparison
	commits          map[rf.SourceKey]*deskkit.RepoCommit
	required         map[string][]string
	localDiffSkipped string
	localDiff        bool
	checked          bool
}

const reviewPreparationVersion = "review-prepare-v2"

var reviewStageBudget = reviewPacketReadBudget
var errPreparedInputsChanged = errors.New("prepared inputs changed")

func boundedPreparation(f preparationForge) (preparationForge, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), reviewStageBudget)
	if live, ok := f.(deskkit.Forge); ok {
		return deskkit.WithForgeReadContext(live, ctx).(preparationForge), cancel
	}
	return f, cancel
}
func (p *preparedReview) startPacket() {
	if p.packetStarted {
		return
	}
	p.cancel()
	p.preparationForge, p.cancel = boundedPreparation(p.source)
	p.packetStarted = true
}
func samePreparedSubject(a, b *deskkit.PullRequest) bool {
	return a != nil && b != nil && a.HeadSHA == b.HeadSHA && a.BaseSHA == b.BaseSHA && a.BaseRef == b.BaseRef && a.Body == b.Body && a.State == b.State && hasDecisionLabel(a.Labels) == hasDecisionLabel(b.Labels)
}

func newPreparedReview(o dispatchOpts, repo string) (*preparedReview, error) {
	if reviewRoundForgeFn == nil {
		if o.observation != "" {
			return nil, deskkit.Unverifiable("observation requires live review preparation", nil)
		}
		return nil, nil
	}
	fg, fr, err := reviewRoundForgeFn(repo)
	if err != nil || fg == nil {
		if o.observation != "" {
			return nil, deskkit.Unverifiable("observation subject could not be checked", err)
		}
		return nil, nil
	}
	full, ok := fg.(preparationForge)
	if !ok {
		if o.observation != "" {
			return nil, deskkit.Unverifiable("observation requires a complete preparation adapter", nil)
		}
		return nil, nil
	} // test/legacy adapter without a packet surface
	source := full
	full, cancel := boundedPreparation(full)
	p := &preparedReview{preparationForge: full, source: source, omissions: map[rf.SourceKey]string{}, repo: fr, cancel: cancel, content: map[rf.SourceKey]*deskkit.FileContent{}, comparisons: map[rf.SourceKey]*deskkit.RefComparison{}, commits: map[rf.SourceKey]*deskkit.RepoCommit{}, required: map[string][]string{}}
	pr, err := p.GetPullRequest(fr, o.pr)
	if err != nil || pr == nil || !isCommitID(pr.HeadSHA) {
		cancel()
		if o.observation != "" {
			return nil, deskkit.Unverifiable("observation subject could not be checked", err)
		}
		return nil, nil
	}
	live, ok := full.(deskkit.Forge)
	if !ok {
		cancel()
		if o.observation != "" {
			return nil, deskkit.Unverifiable("review backend has no qualified identity", nil)
		}
		return nil, nil
	}
	kind, host, err := deskkit.ForgeReadIdentity(live, fr)
	identity := rf.Repository{Provider: string(kind), Host: host, Repository: strings.ToLower(repo)}
	if err != nil {
		cancel()
		if o.observation != "" {
			return nil, deskkit.Unverifiable("observation identity could not be checked", err)
		}
		return nil, nil
	}
	p.context = rf.ReviewContext{Subject: rf.ChangeIdentity{Repository: identity, Number: o.pr, Head: pr.HeadSHA, Base: pr.BaseSHA, BaseUsed: pr.BaseSHA != ""}, AdapterVersion: reviewPreparationVersion}
	p.local, _ = gitcore.Open(o.root)
	if o.observation != "" {
		if err := p.checkObservation(o.observation); err != nil {
			cancel()
			return nil, deskkit.Unverifiable("review observation is not applicable", err)
		}
	}
	return p, nil
}
func (p *preparedReview) checkObservation(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 16<<20+1))
	if err != nil {
		return err
	}
	if len(raw) > 16<<20 {
		return fmt.Errorf("observation exceeds 16 MiB")
	}
	o, err := rf.DecodeObservation(raw)
	if err != nil {
		return err
	}
	// This validates structural/advisory consistency only. The embedded policy is
	// not authenticated or used for selection/admission; all mutable facts are live.
	_, err = rf.PlanObservation(o, rf.ObservationPolicy{Now: dispatchClock(), Oldest: o.Inputs.Collection.Start, Policy: o.Inputs.Policy})
	if err != nil {
		return err
	}
	for _, c := range o.Inputs.Changes {
		if c.Identity.Repository == p.context.Subject.Repository && c.Identity.Number == p.context.Subject.Number {
			if c.Identity.Head != p.change.HeadSHA || (c.Identity.BaseUsed && c.Identity.Base != p.change.BaseSHA) {
				return fmt.Errorf("observed head/base differs from current subject")
			}
			copy := c
			p.context.Advisory = &copy
			return nil
		}
	}
	return fmt.Errorf("assigned provider-qualified subject is absent")
}
func (p *preparedReview) GetPullRequest(r deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	if p.change != nil {
		return p.change, nil
	}
	v, e := p.preparationForge.GetPullRequest(r, n)
	if e == nil && v != nil {
		cp := *v
		p.change = &cp
	}
	return v, e
}
func (p *preparedReview) ReviewsAtHead(r deskkit.ForgeRepo, n int) ([]deskkit.Review, error) {
	if p.reviewsRead {
		return p.reviews, nil
	}
	v, e := p.preparationForge.ReviewsAtHead(r, n)
	if e == nil {
		p.reviews = v
		p.reviewsRead = true
	}
	return v, e
}
func (p *preparedReview) ChecksAtHead(r deskkit.ForgeRepo, h string) (*deskkit.ChecksAtHead, error) {
	if p.checks != nil && h == p.change.HeadSHA {
		return p.checks, nil
	}
	v, e := p.preparationForge.ChecksAtHead(r, h)
	if e == nil && h == p.change.HeadSHA {
		p.checks = v
	}
	return v, e
}
func (p *preparedReview) ListChangedFiles(r deskkit.ForgeRepo, n int) ([]deskkit.ChangedFile, error) {
	if p.filesRead {
		return p.files, nil
	}
	v, e := p.preparationForge.ListChangedFiles(r, n)
	if e == nil {
		p.files = v
		p.filesRead = true
	}
	return v, e
}
func (p *preparedReview) ReadFile(r deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	if in.Ref != p.context.Subject.Head || !reviewPacketSafePath(in.File) {
		return nil, fmt.Errorf("source must name the assigned exact head and a safe path")
	}
	key := p.context.SourceKey(in.File)
	if v, ok := p.content[key]; ok {
		return v, nil
	}
	var v *deskkit.FileContent
	if p.local != nil {
		if raw, large, e := p.local.FileAtBounded(in.Ref, in.File, packet.DefaultPerItemCap); e == nil {
			v = &deskkit.FileContent{Content: []byte(raw), Exists: true}
			if large {
				p.omissions[key] = "local exact blob exceeds the packet per-item byte cap"
				v.Content = nil
			}
		}
	}
	if v == nil {
		var e error
		v, e = p.preparationForge.ReadFile(r, in)
		if e != nil {
			return nil, e
		}
	}
	p.content[key] = v
	return v, nil
}
func (p *preparedReview) ChangeDiff(r deskkit.ForgeRepo, n int) (string, error) {
	if p.local != nil && isCommitID(p.change.BaseSHA) {
		if raw, large, e := p.local.DiffSymmetricBounded(p.change.BaseSHA, p.change.HeadSHA, 3, reviewPacketDiffInputCap); e == nil {
			if large {
				p.localDiffSkipped = "local diff input exceeds the byte budget or includes a non-regular entry"
				return "", fmt.Errorf("%s", p.localDiffSkipped)
			}
			p.localDiff = true
			return raw, nil
		}
	}
	return p.preparationForge.ChangeDiff(r, n)
}
func (p *preparedReview) CompareRefs(r deskkit.ForgeRepo, base, head string) (*deskkit.RefComparison, error) {
	key := rf.SourceKey{Repository: p.context.Subject.Repository, Head: head, Base: base, Version: reviewPreparationVersion + "/provider-compare"}
	immutable := isCommitID(base) && isCommitID(head)
	if immutable {
		if v, ok := p.comparisons[key]; ok {
			return v, nil
		}
	}
	v, e := p.preparationForge.CompareRefs(r, base, head)
	if e == nil && immutable {
		p.comparisons[key] = v
	}
	return v, e
}
func (p *preparedReview) GetCommit(r deskkit.ForgeRepo, head string) (*deskkit.RepoCommit, error) {
	key := rf.SourceKey{Repository: p.context.Subject.Repository, Head: head, Version: reviewPreparationVersion + "/provider-commit"}
	if isCommitID(head) {
		if v, ok := p.commits[key]; ok {
			return v, nil
		}
	}
	v, e := p.preparationForge.GetCommit(r, head)
	if e == nil && isCommitID(head) {
		p.commits[key] = v
	}
	return v, e
}
func (p *preparedReview) RequiredStatusChecks(r deskkit.ForgeRepo, branch string) ([]string, error) {
	v, e := p.preparationForge.RequiredStatusChecks(r, branch)
	if e == nil {
		p.required[branch] = v
	}
	return v, e
}
func (p *preparedReview) recheck() error {
	fresh, cancel := boundedPreparation(p.source)
	defer cancel()
	// Bypass the assembly memo: permission failures and edits invalidate it even
	// without a new head. No stale packet can certify current mutable facts.
	pr, e := fresh.GetPullRequest(p.repo, p.change.Number)
	if e != nil {
		return e
	}
	if pr == nil || !samePreparedSubject(pr, p.change) {
		return fmt.Errorf("%w: change head/base/body/access metadata moved during preparation", errPreparedInputsChanged)
	}
	if p.reviewsRead {
		v, e := fresh.ReviewsAtHead(p.repo, p.change.Number)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(v, p.reviews) {
			return fmt.Errorf("%w: reviews changed during preparation", errPreparedInputsChanged)
		}
	}
	if p.checks != nil {
		v, e := fresh.ChecksAtHead(p.repo, p.change.HeadSHA)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(v, p.checks) {
			return fmt.Errorf("%w: checks changed during preparation", errPreparedInputsChanged)
		}
	}
	for branch, want := range p.required {
		v, e := fresh.RequiredStatusChecks(p.repo, branch)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(v, want) {
			return fmt.Errorf("%w: required-check policy changed during preparation", errPreparedInputsChanged)
		}
	}
	p.checked = true
	return nil
}
