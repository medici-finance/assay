package domain

import (
	"path"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/statusgen/workref"
)

// An operation subject names the separate repositories one piece of work touches:
// the declaring brief revision, its tracking source, the delivery candidate, the
// review target, the decision target and the Evidence destination. In
// cross-repository work these differ, and an adapter that re-derives one from
// another sends a read, prompt or write to the wrong home (#5525).
//
// Every coordinate is a supplied observation. This package performs no alias,
// file, forge or credential read, and no projection authorizes an effect: each
// effect boundary still checks its own permission, custody and current state.
// A missing coordinate is could-not-check, never a fallback to another one. A
// path is a locator inside an already-bound repository, never authorization.

// BriefRevision is the declaring executable record at one exact source commit.
// It mirrors the adapter's observed brief subject; it is not a second identity.
type BriefRevision struct {
	UUID       string
	Version    uint64
	Repository string
	Commit     string
	Path       string
}

// TrackingSource is the observed checkout the brief was read from.
type TrackingSource struct {
	Repository string
	Commit     string
}

// DeliveryTarget is where implementation lands. Candidate is optional: a
// dispatch precedes any delivered commit, an Evidence publication needs one.
type DeliveryTarget struct {
	Repository string
	Candidate  *Candidate
}

const (
	ReviewDelivery  = "delivery"
	ReviewAuthoring = "authoring"
)

// ReviewTarget is one pull request at one observed head.
type ReviewTarget struct {
	Kind       string
	Repository string
	Number     uint64
	Head       string
}

// DecisionTarget is where the brief's human decision is filed and read.
type DecisionTarget struct{ Repository string }

// EvidenceDestination is the brief file whose Evidence section receives rows.
type EvidenceDestination struct {
	Repository string
	Path       string
}

// OperationObservations is the adapter's input. Zero-valued coordinates are
// unobserved; nothing here is inferred from a sibling coordinate.
type OperationObservations struct {
	Work     workref.Result
	Brief    BriefRevision
	Tracking TrackingSource
	Delivery DeliveryTarget
	Review   ReviewTarget
	Decision DecisionTarget
	Evidence EvidenceDestination
}

// OperationSubject is immutable after NewOperationSubject: its fields are
// unexported and every read is a named projection that returns a copy.
type OperationSubject struct {
	work     workref.Result
	brief    BriefRevision
	tracking TrackingSource
	delivery DeliveryTarget
	review   ReviewTarget
	decision DecisionTarget
	evidence EvidenceDestination
}

var (
	operationUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func sameRepo(a, b string) bool { return strings.EqualFold(a, b) }

func locator(p string) bool {
	return p != "" && path.Clean(p) == p && !path.IsAbs(p) && !strings.HasPrefix(p, "../") && p != ".." && !strings.ContainsAny(p, "\\\t\r\n :")
}

// NewOperationSubject is the only constructor. A coordinate that is supplied
// and disagrees with the exact observed subject is refused here; a coordinate
// that is absent stays absent and each projection that needs it says so.
func NewOperationSubject(in OperationObservations) (OperationSubject, error) {
	b := in.Brief
	if !operationUUID.MatchString(b.UUID) || b.Version == 0 || !repoRE.MatchString(b.Repository) || !objectRE.MatchString(b.Commit) || !locator(b.Path) {
		return OperationSubject{}, unknown("brief-revision-unavailable")
	}
	// The work reference resolves to this record; a declared alias or selector
	// cannot point the subject at another one.
	if in.Work.LogicalKey != b.UUID {
		return OperationSubject{}, refuse("work-subject-mismatch")
	}
	if in.Work.Revision != 0 && in.Work.Revision != b.Version {
		return OperationSubject{}, refuse("work-revision-mismatch")
	}
	if in.Tracking.Repository == "" || in.Tracking.Commit == "" {
		return OperationSubject{}, unknown("tracking-source-unavailable")
	}
	if !sameRepo(in.Tracking.Repository, b.Repository) || in.Tracking.Commit != b.Commit {
		return OperationSubject{}, refuse("tracking-source-mismatch")
	}
	if d := in.Delivery; d.Repository != "" || d.Candidate != nil {
		if !repoRE.MatchString(d.Repository) {
			return OperationSubject{}, refuse("delivery-target-shape")
		}
		if d.Candidate != nil {
			if err := d.Candidate.Validate(); err != nil {
				return OperationSubject{}, err
			}
			if !sameRepo(d.Candidate.Repository, d.Repository) {
				return OperationSubject{}, refuse("delivery-candidate-mismatch")
			}
			c := *d.Candidate
			in.Delivery.Candidate = &c
		}
	}
	if dec := in.Decision.Repository; dec != "" && !sameRepo(dec, b.Repository) {
		// #5525: the decision belongs to the tracking repository, never the delivery one.
		return OperationSubject{}, refuse("decision-target-mismatch")
	}
	if e := in.Evidence; e.Repository != "" || e.Path != "" {
		if !sameRepo(e.Repository, b.Repository) || e.Path != b.Path {
			return OperationSubject{}, refuse("evidence-destination-mismatch")
		}
	}
	if r := in.Review; r != (ReviewTarget{}) {
		if r.Number == 0 || !repoRE.MatchString(r.Repository) || (r.Head != "" && !objectRE.MatchString(r.Head)) {
			return OperationSubject{}, refuse("review-target-shape")
		}
		switch r.Kind {
		case ReviewDelivery:
			if in.Delivery.Repository == "" {
				return OperationSubject{}, unknown("delivery-target-unavailable")
			}
			if !sameRepo(r.Repository, in.Delivery.Repository) {
				return OperationSubject{}, refuse("review-target-mismatch")
			}
			if c := in.Delivery.Candidate; c != nil && r.Head != "" && r.Head != c.Commit {
				return OperationSubject{}, refuse("review-head-mismatch")
			}
		case ReviewAuthoring:
			if !sameRepo(r.Repository, b.Repository) {
				return OperationSubject{}, refuse("review-target-mismatch")
			}
		default:
			return OperationSubject{}, refuse("review-kind")
		}
	}
	return OperationSubject{in.Work, b, in.Tracking, in.Delivery, in.Review, in.Decision, in.Evidence}, nil
}

// Work returns the resolved reference, including its legacy compatibility id.
func (s OperationSubject) Work() workref.Result { return s.work }

// Brief returns the declaring brief revision.
func (s OperationSubject) Brief() BriefRevision { return s.brief }

// DispatchProjection is what a worker dispatch reads: where the brief lives,
// where its decision is asked, and where implementation lands.
type DispatchProjection struct {
	Brief              BriefRevision
	TrackingRepository string
	DecisionRepository string
	DeliveryRepository string
	CrossRepository    bool
}

func (s OperationSubject) Dispatch() (DispatchProjection, error) {
	if s.delivery.Repository == "" {
		return DispatchProjection{}, unknown("delivery-target-unavailable")
	}
	if s.decision.Repository == "" {
		return DispatchProjection{}, unknown("decision-target-unavailable")
	}
	return DispatchProjection{s.brief, s.tracking.Repository, s.decision.Repository, s.delivery.Repository, !sameRepo(s.tracking.Repository, s.delivery.Repository)}, nil
}

// ReviewProjection is what a reviewer reads: one pull request at one head, and
// the brief it is reviewed against.
type ReviewProjection struct {
	Brief      BriefRevision
	Kind       string
	Repository string
	Number     uint64
	Head       string
}

func (s OperationSubject) Review() (ReviewProjection, error) {
	if s.review == (ReviewTarget{}) {
		return ReviewProjection{}, unknown("review-target-unavailable")
	}
	if s.review.Head == "" {
		return ReviewProjection{}, unknown("review-head-unavailable")
	}
	r := s.review
	return ReviewProjection{s.brief, r.Kind, r.Repository, r.Number, r.Head}, nil
}

// PRPublication is what opening a delivery pull request reads. CrossRepository
// says the trailer names a brief tracked in another repository.
type PRPublication struct {
	Repository         string
	Work               string
	TrackingRepository string
	CrossRepository    bool
}

func (s OperationSubject) PRPublication() (PRPublication, error) {
	if s.delivery.Repository == "" {
		return PRPublication{}, unknown("delivery-target-unavailable")
	}
	return PRPublication{s.delivery.Repository, s.work.Spelling, s.tracking.Repository, !sameRepo(s.tracking.Repository, s.delivery.Repository)}, nil
}

// EvidencePublication is what landing Evidence rows reads: the brief file in
// the tracking repository, bound to the delivered candidate it verifies.
type EvidencePublication struct {
	Repository string
	Path       string
	Brief      BriefRevision
	Delivery   Candidate
}

func (s OperationSubject) EvidencePublication() (EvidencePublication, error) {
	if s.evidence.Repository == "" {
		return EvidencePublication{}, unknown("evidence-destination-unavailable")
	}
	if s.delivery.Candidate == nil {
		return EvidencePublication{}, unknown("delivery-candidate-unavailable")
	}
	return EvidencePublication{s.evidence.Repository, s.evidence.Path, s.brief, *s.delivery.Candidate}, nil
}
