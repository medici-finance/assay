package evaluation

import (
	"fmt"
	"regexp"
	"strings"
)

// VerifiedStampPattern is the one dated-runner grammar used by all closure adapters.
const VerifiedStampPattern = `^\d{4}-\d{2}-\d{2} \S`

var verifiedStampRE = regexp.MustCompile(VerifiedStampPattern)

func HasHumanReviewer(stamp string) bool {
	for _, tok := range strings.Fields(stamp) {
		if strings.HasPrefix(tok, "human:") {
			return true
		}
	}
	return false
}

// ClosureStructureError retains the underlying parser refusal for CLI projections.
type ClosureStructureError struct{ Detail string }

func (e *ClosureStructureError) Error() string {
	return "verified outcome requires well-formed Verify and Evidence sections: " + e.Detail
}

// ClosureClaim is the supplied current witness judgment for one applicable row.
type ClosureClaim struct{ ID, State, Detail string }

// SelectedClosure is a frozen observation, not authority to collect or publish evidence.
type SelectedClosure struct {
	Status, Stamp, Malformed string
	HumanRows                []string
	Claims                   []ClosureClaim
	FailFirstProblems        []string
}

func ValidateSelectedStamp(status, stamp string) error {
	if status != "verified" && status != "done" {
		return fmt.Errorf("verified outcome requires status verified or done, got %q", status)
	}
	if !verifiedStampRE.MatchString(stamp) {
		return fmt.Errorf("verified outcome requires a dated runner in the Verified cell (YYYY-MM-DD <runner>)")
	}
	return nil
}
func ValidateSelectedClosure(s SelectedClosure) error {
	if e := ValidateSelectedStamp(s.Status, s.Stamp); e != nil {
		return e
	}
	if s.Malformed != "" {
		return &ClosureStructureError{Detail: s.Malformed}
	}
	applicable := 0
	for _, f := range s.Claims {
		if f.State != "author-added" {
			applicable++
		}
	}
	if applicable == 0 {
		return fmt.Errorf("verified outcome requires Verify rows with passing execution witnesses")
	}
	if !HasHumanReviewer(s.Stamp) && len(s.HumanRows) > 0 {
		return fmt.Errorf("verified outcome: Verify row %s is classed gate:human but the Verified cell %q names no human:<name>", s.HumanRows[0], s.Stamp)
	}
	var failures []string
	for _, f := range s.Claims {
		if f.State != "pass" && f.State != "author-added" {
			failures = append(failures, fmt.Sprintf("row %s: %s — %s", f.ID, f.State, f.Detail))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("verified outcome requires passing execution witnesses: %s", strings.Join(failures, "; "))
	}
	if len(s.FailFirstProblems) > 0 {
		return fmt.Errorf("verified outcome requires fail-first witnesses: %s", strings.Join(s.FailFirstProblems, "; "))
	}
	return nil
}

// FailFirst is a row's already-observed provenance, supplied by the Git adapter.
type FailFirst struct {
	Required, Found, Stale              bool
	State, AncestryProblem, HeldProblem string
}

// FailFirstProblem returns a stable code; adapters render existing CLI diagnostics.
func FailFirstProblem(f FailFirst) string {
	if !f.Required {
		return ""
	}
	switch {
	case f.Stale:
		return "stale"
	case !f.Found:
		return "missing"
	case f.State == "unproven" || (f.State != "red" && f.State != "green"):
		return "unproven"
	case f.AncestryProblem != "":
		return "ancestry"
	case f.HeldProblem != "":
		return "held"
	case f.State == "green":
		return "green"
	}
	return ""
}
