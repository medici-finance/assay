package issueflow

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/domain"
)

const HandoffSchema = "verification-handoff-v1"

type Disposition string

const (
	Unresolved   Disposition = "unresolved"
	NeedsScoping Disposition = "needs-scoping"
	Repair       Disposition = "repair"
	External     Disposition = "external-prerequisite"
	Duplicate    Disposition = "duplicate"
)

// Handoff is a companion observation, not an intake exit or a lifecycle record.
// CaseID is supplied by the existing repair/case owner; this library mints no ID.
// Source and Rows bind one failed observation, never an entire brief's acceptance.
type HandoffSource struct {
	Repo           string `json:"repo"`
	Brief          string `json:"brief"`
	SourceVersion  string `json:"source_version"`
	SourceRevision string `json:"source_revision"`
	ReceiptID      string `json:"receipt_id"`
	CaseID         string `json:"case_id"`
	Rows           []int  `json:"rows"`
	BlockerRef     string `json:"blocker_ref"`
}

type Handoff struct {
	Schema         string        `json:"schema"`
	Source         HandoffSource `json:"source"`
	Disposition    Disposition   `json:"disposition"`
	DecidingRole   string        `json:"deciding_role"`
	Successor      string        `json:"successor"`
	ExternalRole   string        `json:"external_role"`
	RequiredAction string        `json:"required_action"`
	ObservedAt     time.Time     `json:"observed_at"`
	FreshUntil     time.Time     `json:"fresh_until"`
}

// HandoffFacts are adapter observations. Complete includes a complete source,
// successor and disposition read, not merely a successful local JSON decode.
// Now is passed explicitly; assessment never consults a clock or an environment.
type HandoffFacts struct {
	Source         HandoffSource
	Complete       bool
	Now            time.Time
	HumanHold      bool
	ActivationHold bool
}

type Assessment struct {
	Disposition Disposition
	State       string // unknown, held or observed; never ready/accepted/authorized
	Reason      string
}

// SupportsHandoff is the exact-version reader/writer negotiation. Existing
// intake-exit-v1 writers remain unchanged and do not imply this capability.
func SupportsHandoff(schema string) bool { return schema == HandoffSchema }

func DecodeHandoff(data []byte) (Handoff, error) {
	var h Handoff
	if err := domain.DecodeStrict(data, &h); err != nil {
		return h, err
	}
	if !SupportsHandoff(h.Schema) {
		return h, fmt.Errorf("handoff: unsupported schema %q", h.Schema)
	}
	return h, nil
}

var (
	repoRef  = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	briefRef = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[0-9]+$`)
	issueRef = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)?#[1-9][0-9]*$`)
)

func knownRole(r string) bool {
	switch r {
	case "the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk", "driver":
		return true
	}
	return false
}

// AssessHandoff describes a routing observation only. Trust, label history,
// admission, claims, permissions and independent acceptance stay at effect owners.
// Even an observed repair does not schedule or resolve work.
func AssessHandoff(h Handoff, f HandoffFacts) Assessment {
	unknown := func(reason string) Assessment { return Assessment{Unresolved, "unknown", reason} }
	if !SupportsHandoff(h.Schema) {
		return unknown("absent or unsupported handoff version")
	}
	if !f.Complete {
		return unknown("incomplete or unreadable observation")
	}
	if !repoRef.MatchString(h.Source.Repo) || !briefRef.MatchString(h.Source.Brief) || h.Source.SourceVersion == "" || h.Source.SourceRevision == "" || h.Source.ReceiptID == "" || h.Source.CaseID == "" || !issueRef.MatchString(h.Source.BlockerRef) || len(h.Source.Rows) == 0 {
		return unknown("missing or invalid source/case binding")
	}
	seen := map[int]bool{}
	for _, row := range h.Source.Rows {
		if row < 1 || seen[row] {
			return unknown("invalid held rows")
		}
		seen[row] = true
	}
	if !sameSource(h.Source, f.Source) {
		return unknown("source binding differs")
	}
	if f.Now.IsZero() || h.ObservedAt.IsZero() || h.FreshUntil.IsZero() || h.FreshUntil.Before(h.ObservedAt) || f.Now.Before(h.ObservedAt) || f.Now.After(h.FreshUntil) {
		return unknown("stale or unknown freshness")
	}
	if f.HumanHold {
		return Assessment{Unresolved, "held", "existing human hold"}
	}
	if f.ActivationHold {
		return Assessment{Unresolved, "held", "existing activation hold"}
	}
	if !knownRole(h.DecidingRole) {
		return unknown("missing accountable deciding role")
	}
	switch h.Disposition {
	case NeedsScoping, Repair, Duplicate:
		if !briefRef.MatchString(h.Successor) && !issueRef.MatchString(h.Successor) {
			return unknown("missing typed successor")
		}
		if h.ExternalRole != "" || h.RequiredAction != "" {
			return unknown("conflicting external owner")
		}
	case External:
		switch h.ExternalRole {
		case "human", "operator", "brief-author", "verifier":
		default:
			return unknown("missing external owner")
		}
		if strings.TrimSpace(h.RequiredAction) == "" || h.Successor != "" {
			return unknown("missing action or conflicting successor")
		}
	case Unresolved, "":
		return unknown("no accountable disposition")
	default:
		return unknown("unsupported disposition")
	}
	return Assessment{h.Disposition, "observed", "routing evidence only; effect and acceptance gates remain required"}
}

func sameSource(a, b HandoffSource) bool {
	if a.Repo != b.Repo || a.Brief != b.Brief || a.SourceVersion != b.SourceVersion || a.SourceRevision != b.SourceRevision || a.ReceiptID != b.ReceiptID || a.CaseID != b.CaseID || a.BlockerRef != b.BlockerRef || len(a.Rows) != len(b.Rows) {
		return false
	}
	rows := map[int]bool{}
	for _, row := range a.Rows {
		rows[row] = true
	}
	for _, row := range b.Rows {
		if !rows[row] {
			return false
		}
		delete(rows, row)
	}
	return true
}
