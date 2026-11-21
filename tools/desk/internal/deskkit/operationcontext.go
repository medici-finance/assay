package deskkit

import (
	"errors"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/statusgen/domain"
)

// OperationTargets are the coordinates a caller observed outside a checkout: the
// pull request under review and the delivered candidate. Either may be zero.
type OperationTargets struct {
	Review    domain.ReviewTarget
	Candidate *domain.Candidate
}

// OperationContext is the observation adapter for the statusgen/domain operation
// subject. It reads the tracking checkout's origin and HEAD, and the delivery
// checkout's origin, and hands them to domain.NewOperationSubject unchanged.
//
// The decision target and the Evidence destination are the tracking checkout's
// observed origin and the selected brief's path. They are never taken from the
// delivery checkout, a flag, an alias or a basename. An empty deliveryRoot
// leaves the delivery coordinate unobserved; it is not replaced by the tracking
// one. The result authorizes nothing: every effect still checks its own
// permission, custody and current state.
func OperationContext(work WorkSelection, trackingRoot, deliveryRoot string, targets OperationTargets) (domain.OperationSubject, error) {
	if err := work.Validate(); err != nil {
		return domain.OperationSubject{}, err
	}
	tracking, err := observeTracking(trackingRoot)
	if err != nil {
		return domain.OperationSubject{}, err
	}
	var delivery domain.DeliveryTarget
	if deliveryRoot != "" {
		slug, err := OriginRepoSlug(deliveryRoot)
		if err != nil {
			return domain.OperationSubject{}, Unverifiable("delivery checkout origin unavailable", err)
		}
		delivery = domain.DeliveryTarget{Repository: slug, Candidate: targets.Candidate}
	} else if targets.Candidate != nil {
		return domain.OperationSubject{}, Unverifiable("delivery candidate supplied without an observed delivery checkout", nil)
	}
	subject, err := domain.NewOperationSubject(domain.OperationObservations{
		Work:     work.Reference,
		Brief:    work.OperationBrief(),
		Tracking: tracking,
		Delivery: delivery,
		Review:   targets.Review,
		Decision: domain.DecisionTarget{Repository: tracking.Repository},
		Evidence: domain.EvidenceDestination{Repository: tracking.Repository, Path: work.Subject.Path},
	})
	return subject, operationError(err)
}

func observeTracking(root string) (domain.TrackingSource, error) {
	top, err := gitcore.Toplevel(root)
	if err != nil {
		return domain.TrackingSource{}, Unverifiable("tracking checkout unreadable", err)
	}
	repo, err := gitcore.Open(top)
	if err != nil {
		return domain.TrackingSource{}, Unverifiable("tracking checkout unreadable", err)
	}
	head, err := repo.HeadID()
	if err != nil {
		return domain.TrackingSource{}, Unverifiable("tracking checkout commit unavailable", err)
	}
	slug, err := OriginRepoSlug(top)
	if err != nil {
		return domain.TrackingSource{}, Unverifiable("tracking checkout origin unavailable", err)
	}
	return domain.TrackingSource{Repository: slug, Commit: head}, nil
}

// operationError maps the domain's three-state refusal onto the desk exit
// contract: could-not-check is unverifiable (6), checked-failed is refused (5).
func operationError(err error) error {
	if err == nil {
		return nil
	}
	var r *domain.Refusal
	if !errors.As(err, &r) {
		return Unverifiable("operation subject", err)
	}
	if r.State == "could-not-check" {
		return Unverifiable("operation subject "+r.Code, err)
	}
	return Refused("operation subject " + r.Code)
}
