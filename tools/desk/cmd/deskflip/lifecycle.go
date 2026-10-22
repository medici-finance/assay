package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lifecycleDelivery classifies only whether the PR declares delivery. Statusgen owns
// the lifecycle rules and declared-partial interpretation: a partial delivery still
// needs its successful preflight (which can report that no transition is proposed).
// The caller supplies the complete diff already reconciled by security-verdict.
func lifecycleDelivery(body string, files []fileInfo) bool {
	trailers, err := deskkit.ParseTrailers([]byte(body))
	if err != nil {
		return true
	} // malformed/mixed declarations never grant an exemption
	for _, tr := range trailers {
		switch tr.Kind {
		case deskkit.TrailerBrief:
			return !deskkit.BriefAuthoringOnly(tr.Value, asChangedFiles(files))
		case deskkit.TrailerCrossRepoBrief:
			return true
		}
	}
	return false
}

// checks comes exclusively from the complete rollup fetched by commit SHA in flip.
// A previous-head run is absent from this observation, never evidence for this head.
// Unlike checks-green, skipped/neutral are not successes: this mandatory check must run.
func checkLifecyclePreflight(body string, files []fileInfo, checks []rollupEntry, head string, editedAt func() (string, error)) error {
	if !lifecycleDelivery(body, files) {
		return nil
	}
	for _, c := range latestPerRollupName(checks) {
		if c.groupKey() != condLifecyclePreflight {
			continue
		}
		if (strings.EqualFold(c.Status, "completed") && strings.EqualFold(c.Conclusion, "success")) ||
			(c.Name == "" && strings.EqualFold(c.State, "success")) {
			if editedAt != nil {
				stamp, err := editedAt()
				if err != nil {
					return deskkit.Unverifiable(fmt.Sprintf("condition %s: cannot read the current PR body edit time", condLifecyclePreflight), err)
				}
				if stamp != "" {
					edited, err := time.Parse(time.RFC3339Nano, stamp)
					startedStamp := c.StartedAt
					if c.Name == "" {
						startedStamp = c.CreatedAt
					}
					started, startErr := time.Parse(time.RFC3339Nano, startedStamp)
					if err != nil || startErr != nil || started.Before(edited) {
						return deskkit.Unverifiable(fmt.Sprintf("condition %s: the successful check predates the current delivery declaration, or its time cannot be proven; rerun lifecycle-preflight after the PR body edit", condLifecyclePreflight), nil)
					}
				}
			}
			return nil
		}
		return deskkit.Unverifiable(fmt.Sprintf("condition %s: the mandatory lifecycle check at %s has not completed successfully; statusgen must validate the proposed implementation transition before ready-flip", condLifecyclePreflight, short(head)), nil)
	}
	return deskkit.Unverifiable(fmt.Sprintf("condition %s: delivering Brief: PR has no successful lifecycle-preflight check at %s; an absent or previous-head result is not a pass", condLifecyclePreflight, short(head)), nil)
}
