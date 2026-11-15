package perfdata

// batch.go — the performance-batch-v1 transport envelope and the performance-receipt-v1
// acknowledgment. No sender or server exists in this module: Admit is the pure
// admission rule a collector applies, published with fixtures (testdata/api) so the
// database, server and sender can be built independently.
//
// An acknowledgment means committed to the configured store, never "received in
// memory": if the commit fails, nothing in the batch is acknowledged and every event
// that would have been is returned as retry. Identical duplicates succeed
// idempotently; the same identity with a different digest conflicts.

import (
	"encoding/json"
	"time"
)

// Batch bounds.
const (
	MaxBatchEvents = 500
	MaxBatchBytes  = 1 << 20
)

// Batch is one transport batch of stored events, byte-for-byte as spooled.
type Batch struct {
	Schema          string            `json:"schema"`
	Source          string            `json:"source"`
	ConsentRevision string            `json:"consentRevision"`
	Profile         Profile           `json:"profile"`
	Events          []json.RawMessage `json:"events"`
}

// Receipt and result statuses.
const (
	ReceiptCommitted = "committed" // every event acknowledged or duplicate
	ReceiptPartial   = "partial"   // committed, with some events refused or conflicting
	ReceiptRejected  = "rejected"  // nothing admitted; batch-level reason
	ReceiptRetry     = "retry"     // nothing committed; resend unchanged

	ResultAcknowledged   = "acknowledged"
	ResultDuplicate      = "duplicate"
	ResultConflicting    = "conflicting"
	ResultRejected       = "rejected"
	ResultOutsideHorizon = "outside-horizon"
	ResultRetry          = "retry"
)

// Batch-level rejection reasons.
const (
	ReasonSchema          = "schema"
	ReasonBounds          = "bounds"
	ReasonSourceBinding   = "source-not-authenticated"
	ReasonConsentRevision = "consent-revision"
	ReasonProfile         = "profile-not-allowed"
	ReasonCommitFailed    = "commit-failed"
	ReasonNoneAdmitted    = "none-admitted"
)

// Result is the per-event outcome. ID and Digest are present when the event decoded
// far enough to have them; Class names the refusal class, never input text.
type Result struct {
	Index  int          `json:"index"`
	ID     string       `json:"id,omitempty"`
	Status string       `json:"status"`
	Class  RefusalClass `json:"class,omitempty"`
	Digest string       `json:"digest,omitempty"`
}

// Receipt is the collector's answer to one batch.
type Receipt struct {
	Schema      string   `json:"schema"`
	BatchDigest string   `json:"batchDigest"`
	Committed   bool     `json:"committed"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason,omitempty"`
	Results     []Result `json:"results"`
}

// AdmitContext is the collector-side state Admit needs. AuthenticatedSource is the
// source bound to the caller's credential; the batch's claimed source must equal it.
// Known maps already-committed event IDs to their digests. Commit durably stores the
// accepted events (nil or an error means nothing was committed).
type AdmitContext struct {
	AuthenticatedSource string
	ConsentRevision     string
	AllowedProfiles     []Profile
	Now                 time.Time
	ReplayHorizon       time.Duration
	Known               map[string]string
	Commit              func(accepted []Event) error
}

// Admit applies the admission rules to one encoded batch and returns its receipt.
func Admit(raw []byte, ac AdmitContext) Receipt {
	rc := Receipt{Schema: ReceiptSchema, BatchDigest: Digest(raw), Results: []Result{}}
	reject := func(reason string) Receipt {
		rc.Status, rc.Reason = ReceiptRejected, reason
		return rc
	}
	if len(raw) > MaxBatchBytes {
		return reject(ReasonBounds)
	}
	var b Batch
	if err := strictUnmarshal(raw, &b); err != nil || b.Schema != BatchSchema {
		return reject(ReasonSchema)
	}
	if len(b.Events) == 0 || len(b.Events) > MaxBatchEvents {
		return reject(ReasonBounds)
	}
	if !sourceRE.MatchString(b.Source) || b.Source != ac.AuthenticatedSource {
		return reject(ReasonSourceBinding)
	}
	if b.ConsentRevision == "" || b.ConsentRevision != ac.ConsentRevision {
		return reject(ReasonConsentRevision)
	}
	if !containsProfile(ac.AllowedProfiles, b.Profile) {
		return reject(ReasonProfile)
	}
	var accepted []Event
	inBatch := map[string]string{}
	for i, line := range b.Events {
		r := Result{Index: i}
		e, err := DecodeEvent(line)
		if err != nil {
			r.Status, r.Class = ResultRejected, AsRefusal(err).Class
			rc.Results = append(rc.Results, r)
			continue
		}
		r.ID, r.Digest = e.ID, Digest(line)
		recorded, _ := ParseTime(e.RecordedAt)
		switch {
		case e.Source != b.Source:
			r.Status, r.Class = ResultRejected, RefusePrivacy
		case e.Profile != b.Profile:
			r.Status, r.Class = ResultRejected, RefusePrivacy
		case ac.ReplayHorizon > 0 && recorded.Before(ac.Now.Add(-ac.ReplayHorizon)):
			r.Status = ResultOutsideHorizon
		case ac.Known[e.ID] != "" || inBatch[e.ID] != "":
			prev := ac.Known[e.ID]
			if prev == "" {
				prev = inBatch[e.ID]
			}
			if prev == r.Digest {
				r.Status = ResultDuplicate
			} else {
				r.Status = ResultConflicting
			}
		default:
			r.Status = ResultAcknowledged
			inBatch[e.ID] = r.Digest
			accepted = append(accepted, e)
		}
		rc.Results = append(rc.Results, r)
	}
	if len(accepted) > 0 {
		if ac.Commit == nil || ac.Commit(accepted) != nil {
			for i := range rc.Results {
				if rc.Results[i].Status == ResultAcknowledged {
					rc.Results[i].Status = ResultRetry
				}
			}
			rc.Status, rc.Reason = ReceiptRetry, ReasonCommitFailed
			return rc
		}
	}
	ok, refused := 0, 0
	for _, r := range rc.Results {
		if r.Status == ResultAcknowledged || r.Status == ResultDuplicate {
			ok++
		} else {
			refused++
		}
	}
	if ok == 0 {
		rc.Status, rc.Reason = ReceiptRejected, ReasonNoneAdmitted
		return rc
	}
	rc.Committed = true
	rc.Status = ReceiptCommitted
	if refused > 0 {
		rc.Status = ReceiptPartial
	}
	return rc
}
