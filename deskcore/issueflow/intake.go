package issueflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/domain"
)

// IntakeHandoff is a separately negotiated companion to intake-exit-v1.
// It records a judgment about a source event, not permission to claim, execute,
// close an issue, or accept verification. Verification retains its /01 binding.
const IntakeHandoffSchema = "intake-handoff-v1"

type IntakeHandoff struct {
	Schema         string      `json:"schema"`
	Item           string      `json:"item"`
	Opened         string      `json:"opened"`
	Revision       int         `json:"revision"`
	DecidingRole   string      `json:"deciding_role"`
	Disposition    Disposition `json:"disposition"`
	Delivered      bool        `json:"delivered"`
	Route          string      `json:"route"` // authoring, repair, qualification, duplicate, external
	Successor      string      `json:"successor"`
	AuthorTier     string      `json:"author_tier"`
	ExternalRole   string      `json:"external_role"`
	RequiredAction string      `json:"required_action"`
	WakeCondition  string      `json:"wake_condition"`
	Evidence       string      `json:"evidence"`
	Verification   *Handoff    `json:"verification"`
}

type IntakeFacts struct {
	Item, Opened string
	Complete     bool
	Candidate    Candidate
	Verification HandoffFacts
}

type IntakeAssessment struct {
	DirectRepair bool   // the source issue itself is the scoped repair target
	State        string // classification-needed, held, successor-owed, observed, external-wait
	Route        string
	Reason       string
}

func SupportsIntakeHandoff(schema string) bool { return schema == IntakeHandoffSchema }

func DecodeIntakeHandoff(raw []byte) (IntakeHandoff, error) {
	var h IntakeHandoff
	if err := domain.DecodeStrict(raw, &h); err != nil {
		return h, err
	}
	if !SupportsIntakeHandoff(h.Schema) {
		return h, fmt.Errorf("unsupported intake handoff schema %q", h.Schema)
	}
	return h, nil
}

// AssessIntakeHandoff shares disposition semantics for the issue and register
// lanes. Complete means the adapter read the whole current record. Successor
// existence, scope, trust, freshness, claims and acceptance must still be checked
// at their effect owners; an observed judgment is not a completed transfer.
func AssessIntakeHandoff(h IntakeHandoff, f IntakeFacts) IntakeAssessment {
	result := func(state, why string) IntakeAssessment {
		return IntakeAssessment{State: state, Route: h.Route, Reason: why}
	}
	if !SupportsIntakeHandoff(h.Schema) || !f.Complete {
		return result("classification-needed", "legacy or unreadable disposition")
	}
	if h.Item == "" || !strings.EqualFold(h.Item, f.Item) || h.Opened == "" || h.Opened != f.Opened || h.Revision < 1 {
		return result("classification-needed", "source event or revision differs")
	}
	if _, err := time.Parse(time.RFC3339, h.Opened); err != nil {
		return result("classification-needed", "unknown source event time")
	}
	if !knownRole(h.DecidingRole) || strings.TrimSpace(h.Evidence) == "" {
		return result("classification-needed", "accountable role and evidence required")
	}
	c := f.Candidate
	// Representation is deliberately ignored here: a placeholder is not an
	// admission and cannot override a decision, addressee, exclusion or hold.
	c.HasPlaceholder = false
	if !Select(WorkReady, c, "") {
		return result("held", "independent decision, addressee or exclusion")
	}
	if h.Verification != nil {
		a := AssessHandoff(*h.Verification, f.Verification)
		if a.State != "observed" {
			return result("classification-needed", a.Reason)
		}
		if h.Disposition != h.Verification.Disposition || h.DecidingRole != h.Verification.DecidingRole || h.Successor != h.Verification.Successor || h.ExternalRole != h.Verification.ExternalRole || h.RequiredAction != h.Verification.RequiredAction {
			return result("classification-needed", "verification and intake judgments differ")
		}
	}
	if h.Delivered && h.Route != "qualification" {
		return result("successor-owed", "delivered source requires installation or qualification")
	}
	if h.Disposition == External {
		if h.Route != "external" || h.Successor != "" || strings.TrimSpace(h.RequiredAction) == "" || strings.TrimSpace(h.WakeCondition) == "" {
			return result("successor-owed", "external owner, action and wake required")
		}
		switch h.ExternalRole {
		case "human", "operator", "brief-author", "verifier":
		default:
			return result("successor-owed", "unknown external owner")
		}
		return result("external-wait", "explicit external prerequisite; no repair dispatch")
	}
	if h.ExternalRole != "" || h.RequiredAction != "" || h.WakeCondition != "" {
		return result("classification-needed", "conflicting external obligation")
	}
	if !briefRef.MatchString(h.Successor) && !issueRef.MatchString(h.Successor) {
		return result("successor-owed", "linked successor required")
	}
	switch h.Disposition {
	case NeedsScoping:
		if h.Route != "authoring" || h.AuthorTier != "strong" {
			return result("successor-owed", "linked strong authoring task required")
		}
	case Repair:
		if h.Route != "repair" && h.Route != "qualification" {
			return result("classification-needed", "repair or delivered-source qualification required")
		}
	case Duplicate:
		if h.Route != "duplicate" {
			return result("classification-needed", "duplicate owner required")
		}
	default:
		return result("classification-needed", "accountable disposition owed")
	}
	a := result("observed", "judgment recorded; successor admission and completion remain independent")
	// A routed successor owns its own admission and claim. Do not also offer
	// the source placeholder as a second implementation of the same repair.
	successor := h.Successor
	if repo, _, ok := strings.Cut(h.Item, "#"); ok && strings.HasPrefix(successor, "#") {
		successor = repo + successor
	}
	a.DirectRepair = h.Disposition == Repair && h.Route == "repair" && strings.EqualFold(h.Item, successor)
	return a
}

// RepairDispositionSettled permits consideration by the ordinary work reader.
// It is one prerequisite, never a substitute for its other admission checks.
func RepairDispositionSettled(a IntakeAssessment) bool {
	return a.State == "observed" && a.Route == "repair" && a.DirectRepair
}
