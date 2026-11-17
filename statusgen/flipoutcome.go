package main

// flipoutcome.go — the verify-outcome cross-check of `statusgen verifyflip`
// (#6734).
//
// THE GAP IT CLOSES. verifyflip takes its verdict from the latest strict
// `**VERIFY: PASS**` in Evidence. The same verify run also writes a
// verify-outcome record (one file per run under outcomeRecordsDir), and the
// two can disagree: an Evidence entry can read PASS while the run's
// own record says `blocked`. Without this check the flip acted on the
// Evidence alone.
//
// THE RULE. The flip is allowed only when the brief's LATEST verify-outcome
// record (latestOutcomePerBrief: newest ts, the record layer's own reduction)
// names the revision being flipped AND says `verified`. Each disagreement is a
// refusal that names which of the three it was:
//
//   - outcome record absent: no record names the brief;
//   - outcome record revision mismatch: the record names another revision;
//   - outcome record result mismatch: the record's outcome is not `verified`.
//
// The record's revision is `source_sha`, the commit the verify run observed
// (the one its Runner cells name as `<login> @ <sha>`). A record that carries
// no `source_sha` is an older shape whose only revision is `sha`, so `sha` is
// read then. A record store that cannot be read, a record that is not valid
// JSON, a record with no revision, or a record dated beyond the clock-skew
// tolerance (it never wins the reduction, so the latest record is unknown) is
// could-not-check: the flip refuses, fail-closed, and never reads it as
// agreeing.

import (
	"encoding/json"
	"fmt"
	"strings"
)

// flipOutcomeCheck cross-checks the brief's latest verify-outcome record under
// root against a flip of key to verified at wantSHA. A *flipRefusal refuses;
// any other error is could-not-check.
func flipOutcomeCheck(root, key, wantSHA string) error {
	records, err := readVerifyOutcomeRecords(root)
	if err != nil {
		return fmt.Errorf("outcome record: verify-outcome records unreadable: %v", err)
	}
	latest, future := latestOutcomePerBrief(records)
	if future[key] {
		return fmt.Errorf("outcome record: a verify-outcome record for %s is dated beyond the clock-skew tolerance — it never wins, so the latest record is unknown", key)
	}
	rec, ok := latest[key]
	if !ok {
		return refuseFlip("outcome record absent: no verify-outcome record names %s — the flip needs the record its verify run wrote", key)
	}
	var o struct {
		Outcome   string `json:"outcome"`
		SHA       string `json:"sha"`
		SourceSHA string `json:"source_sha"`
	}
	if err := json.Unmarshal(rec.Raw, &o); err != nil {
		return fmt.Errorf("outcome record: %s cannot be read: %v", rec.Source, err)
	}
	rev := strings.TrimSpace(o.SourceSHA)
	if rev == "" {
		rev = strings.TrimSpace(o.SHA)
	}
	if !flipShaRe.MatchString(rev) {
		return fmt.Errorf("outcome record: %s names no readable revision (source_sha/sha %q)", rec.Source, rev)
	}
	r, w := strings.ToLower(rev), strings.ToLower(wantSHA)
	if !strings.HasPrefix(r, w) && !strings.HasPrefix(w, r) {
		return refuseFlip("outcome record revision mismatch: the latest verify-outcome record for %s (%s) names %s, the flip is at %s", key, rec.Source, rev, wantSHA)
	}
	if classifyOutcome(o.Outcome) != outcomePass {
		return refuseFlip("outcome record result mismatch: the latest verify-outcome record for %s (%s) says outcome %q, the flip requests verified", key, rec.Source, o.Outcome)
	}
	return nil
}
