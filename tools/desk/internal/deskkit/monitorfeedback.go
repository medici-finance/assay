package deskkit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MonitorFeedback contains revision tokens for advisory wakeups. An empty token means
// this dimension was not observed. No review body, check name or verdict is published.
type MonitorFeedback struct {
	Reviews string `json:"reviews,omitempty"`
	Checks  string `json:"checks,omitempty"`
}

// MonitorBatch accompanies one complete review-queue observation, binding its rows
// to the same collection and provider-qualified change identities.
type MonitorBatch struct {
	Generation string                     `json:"generation"`
	Policy     string                     `json:"policy"`
	Rows       map[string]MonitorFeedback `json:"rows"`
}

func MonitorSubject(repo string, number int, head string) string {
	return repo + "#" + strconv.Itoa(number) + "@" + head
}

func feedbackRevision(entries [][]string) string {
	// Sorting encoded tuples treats forge ordering as transport. Length framing in
	// JSON preserves field boundaries even for names containing a control character.
	rows := make([]string, 0, len(entries))
	for _, entry := range entries {
		b, _ := json.Marshal(entry)
		rows = append(rows, string(b))
	}
	sort.Strings(rows)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(rows, "\n"))))
}

func (c QueuedChange) MonitorFeedback(reviewer string) MonitorFeedback {
	if c.HeadSHA == "" {
		return MonitorFeedback{}
	}
	return MonitorFeedback{Reviews: c.monitorReviews(reviewer), Checks: c.monitorChecks()}
}

func (c QueuedChange) monitorReviews(reviewer string) string {
	if !c.ReviewsComplete || reviewer == "" {
		return ""
	}
	var reviews [][]string
	for _, r := range c.Reviews {
		if r.Author.Login != reviewer || r.CommitID != c.HeadSHA {
			continue
		}
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
		case "COMMENTED":
			if !hasSecurityVerdictMarker(r.Body) {
				continue
			}
		default:
			continue
		}
		if r.ID <= 0 {
			return ""
		}
		reviews = append(reviews, []string{reviewer, strconv.FormatInt(r.ID, 10), r.State, strconv.FormatBool(HasSecurityReviewPass(r.Body)), strconv.FormatBool(HasSecurityReviewFail(r.Body))})
	}
	return feedbackRevision(reviews)
}

func (c QueuedChange) monitorChecks() string {
	if !c.MonitorChecksComplete {
		return ""
	}
	var terminal [][]string
	for _, check := range c.Rollup {
		var record CICheckRecord
		var ok bool
		switch check.Typename {
		case "CheckRun":
			switch strings.ToUpper(check.Status) {
			case "QUEUED", "WAITING", "PENDING", "REQUESTED", "IN_PROGRESS":
				continue
			case "COMPLETED":
				if check.Conclusion == "" {
					return ""
				}
				record, ok = ciRunRecord(CICheckRecord{}, check.ID, check.Name, check.Status, check.Conclusion, check.StartedAt, check.CompletedAt)
			}
		case "StatusContext":
			switch strings.ToUpper(check.State) {
			case "EXPECTED", "PENDING":
				continue
			case "SUCCESS", "FAILURE", "ERROR":
				record, ok = ciStatusRecord(CICheckRecord{}, check.Context, check.State, check.CreatedAt)
			}
		}
		if !ok {
			return ""
		}
		terminal = append(terminal, []string{record.Kind, record.Name, record.Attempt, record.Conclusion})
	}
	return feedbackRevision(terminal)
}
