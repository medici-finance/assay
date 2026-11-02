package deskkit

// Verification attempts (desk-supervision/37). A verifier dispatch lands ONE outcome record,
// but inside that dispatch the Verify table may be run more than once: a run on locally built
// tools is never credit, an environmental fault can throw a run away, the credit check can
// fail. The optional `attempts` list on the record keeps every one of those runs with its
// disposition, so "brief N needed four runs and three earned nothing" is a query rather than a
// paragraph of Evidence prose.
//
// An attempt carries no cost measure of its own (driver correction 2026-10-22): its window
// (`started`/`finished`) is the join key to the workflow perf rows the desk already writes,
// and `deskperf reruns` does the join. CheckAttempts is the one validator, run by the writer
// (through ParseJoinInput) and again by the reader on every stored record, so a hand-landed
// or writer-bypassing record is reported, never counted.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// Attempt dispositions. Unrecorded is never valid on a record: it is what the rollup calls the
// single run a record WITHOUT attempts stands for, whose internal re-runs cannot be checked.
const (
	AttemptCredited   = "credited"
	AttemptUncredited = "uncredited"
	AttemptDiscarded  = "discarded"
	AttemptUnrecorded = "unrecorded"
)

// Attempt reasons, one closed set per disposition; a credited attempt carries none.
const (
	ReasonLocalBuild      = "local-build"
	ReasonNotCredited     = "not-credited"
	ReasonCreditUnchecked = "credit-unchecked"
	ReasonEnvFault        = "env-fault"
	ReasonSuperseded      = "superseded"
)

// MaxAttempts bounds one record's attempts list.
const MaxAttempts = 16

var attemptReasons = map[string][]string{
	AttemptUncredited: {ReasonLocalBuild, ReasonNotCredited, ReasonCreditUnchecked},
	AttemptDiscarded:  {ReasonEnvFault, ReasonSuperseded},
}

// attemptKeys is the exact key set of one entry.
var attemptKeys = []string{"attempt", "disposition", "reason", "cause_ref", "started", "finished", "rows_run"}

// VerifyAttempt is one validated entry of a record's attempts list.
type VerifyAttempt struct {
	Attempt     int
	Disposition string
	Reason      string
	CauseRef    string
	Started     time.Time
	Finished    time.Time
	RowsRun     int
}

// attemptRefusal is the one refusal shape: every refusal names the attempt and the rule.
func attemptRefusal(n int, rule, detail string) error {
	return Refused(fmt.Sprintf("refused: attempts: attempt %d: rule %s: %s", n, rule, detail))
}

// attemptUTC parses an RFC3339 UTC timestamp (a trailing Z, no offset).
func attemptUTC(v string) (time.Time, bool) {
	if !strings.HasSuffix(v, "Z") {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	return t, err == nil
}

// CheckAttempts validates the `attempts` key of one record's top-level fields and returns the
// parsed entries. A record without the key passes with nil. Every refusal is exit 5 and names
// the attempt number and the rule. It reads `outcome`, `started`, `finished` and `rows_total`
// from the same record, because attempts are ordered inside the record's window and bounded
// by its row count.
func CheckAttempts(f map[string]json.RawMessage) ([]VerifyAttempt, error) {
	raw, has := f["attempts"]
	if !has {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, attemptRefusal(1, "shape", "attempts must be a JSON list of attempt objects")
	}
	if len(entries) == 0 {
		return nil, attemptRefusal(1, "count", "attempts lists no entry — omit the key instead (1 to 16 entries)")
	}
	if len(entries) > MaxAttempts {
		return nil, attemptRefusal(MaxAttempts+1, "count",
			fmt.Sprintf("attempts lists %d entries; at most %d", len(entries), MaxAttempts))
	}

	recStr := func(key string) (string, bool) {
		v, ok := f[key]
		if !ok {
			return "", false
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			return "", false
		}
		return s, true
	}
	recStartRaw, okS := recStr("started")
	recFinishRaw, okF := recStr("finished")
	if !okS || !okF {
		return nil, attemptRefusal(1, "record-window", "a record with attempts carries its own started and finished")
	}
	recStart, errS := time.Parse(time.RFC3339, recStartRaw)
	recFinish, errF := time.Parse(time.RFC3339, recFinishRaw)
	if errS != nil || errF != nil {
		return nil, attemptRefusal(1, "record-window", "the record's started/finished are not RFC3339 timestamps")
	}
	totalLex := strings.TrimSpace(string(f["rows_total"]))
	rowsTotal, terr := strconv.Atoi(totalLex)
	if _, ok := f["rows_total"]; !ok || terr != nil || !intLexemeRe.MatchString(totalLex) {
		return nil, attemptRefusal(1, "rows-total", "a record with attempts carries rows_total (a JSON integer)")
	}

	out := make([]VerifyAttempt, 0, len(entries))
	credited := 0
	for i, e := range entries {
		n := i + 1
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(e, &obj); err != nil || obj == nil {
			return nil, attemptRefusal(n, "shape", "an attempt must be a JSON object")
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if want, bad := caseVariantOf(k, attemptKeys); bad {
				return nil, attemptRefusal(n, "key-case", fmt.Sprintf("key %q must be spelled %q", k, want))
			}
			known := false
			for _, a := range attemptKeys {
				known = known || k == a
			}
			if !known {
				return nil, attemptRefusal(n, "unknown-key",
					fmt.Sprintf("%q is not an attempt field (%s)", k, strings.Join(attemptKeys, ", ")))
			}
		}
		a := VerifyAttempt{}
		numLex := strings.TrimSpace(string(obj["attempt"]))
		num, nerr := strconv.Atoi(numLex)
		if !intLexemeRe.MatchString(numLex) || nerr != nil || num != n {
			return nil, attemptRefusal(n, "numbering", fmt.Sprintf("attempt must be %d (the list is numbered 1..n contiguously); got %s", n, strconv.Quote(numLex)))
		}
		a.Attempt = num
		str := func(key string) (string, bool, error) {
			v, ok := obj[key]
			if !ok {
				return "", false, nil
			}
			var s string
			if json.Unmarshal(v, &s) != nil {
				return "", true, attemptRefusal(n, "type", key+" must be a JSON string")
			}
			return s, true, nil
		}
		disp, hasDisp, err := str("disposition")
		if err != nil {
			return nil, err
		}
		if !hasDisp || (disp != AttemptCredited && disp != AttemptUncredited && disp != AttemptDiscarded) {
			return nil, attemptRefusal(n, "disposition", "disposition must be one of credited, uncredited, discarded")
		}
		a.Disposition = disp
		reason, hasReason, err := str("reason")
		if err != nil {
			return nil, err
		}
		if disp == AttemptCredited {
			if hasReason {
				return nil, attemptRefusal(n, "credited-no-reason", "a credited attempt carries no reason")
			}
		} else {
			allowed := attemptReasons[disp]
			ok := false
			for _, r := range allowed {
				ok = ok || reason == r
			}
			if !hasReason || !ok {
				return nil, attemptRefusal(n, "reason", fmt.Sprintf("a %s attempt names its reason: one of %s", disp, strings.Join(allowed, ", ")))
			}
			a.Reason = reason
		}
		cause, hasCause, err := str("cause_ref")
		if err != nil {
			return nil, err
		}
		if hasCause {
			if _, perr := ParseBlockerRef(cause, "", ""); perr != nil {
				return nil, attemptRefusal(n, "cause-ref", "cause_ref takes blocker_ref's grammar: #<N>, <owner>/<repo>#<N>, or a forge issue/PR/run URL")
			}
			a.CauseRef = strings.TrimSpace(cause)
		}
		if reason == ReasonEnvFault && !hasCause {
			return nil, attemptRefusal(n, "cause-ref-required", "an env-fault attempt names its cause in cause_ref")
		}
		for _, key := range []string{"started", "finished"} {
			v, has, err := str(key)
			if err != nil {
				return nil, err
			}
			t, ok := attemptUTC(v)
			if !has || !ok {
				return nil, attemptRefusal(n, "timestamp", key+" must be an RFC3339 UTC timestamp ending in Z")
			}
			if key == "started" {
				a.Started = t
			} else {
				a.Finished = t
			}
		}
		if a.Started.After(a.Finished) {
			return nil, attemptRefusal(n, "window", "started is after finished")
		}
		rowsLex := strings.TrimSpace(string(obj["rows_run"]))
		rows, rerr := strconv.Atoi(rowsLex)
		if _, ok := obj["rows_run"]; !ok || rerr != nil || !intLexemeRe.MatchString(rowsLex) || rows < 1 || rows > rowsTotal {
			return nil, attemptRefusal(n, "rows-run", fmt.Sprintf("rows_run must be an integer from 1 to rows_total (%d)", rowsTotal))
		}
		a.RowsRun = rows

		if n == 1 && a.Started.Before(recStart) {
			return nil, attemptRefusal(n, "record-window", "attempt 1 starts before the record's started")
		}
		if n > 1 && a.Started.Before(out[n-2].Finished) {
			return nil, attemptRefusal(n, "overlap", fmt.Sprintf("attempt %d starts before attempt %d finished (attempts ascend and never overlap)", n, n-1))
		}
		if a.Finished.After(recFinish) {
			return nil, attemptRefusal(n, "record-window", "the attempt finishes after the record's finished")
		}
		if disp == AttemptCredited {
			credited++
			if credited > 1 {
				return nil, attemptRefusal(n, "single-credited", "at most one attempt is credited")
			}
		}
		out = append(out, a)
	}
	last := len(out)
	for _, a := range out {
		if a.Disposition == AttemptCredited && a.Attempt != last {
			return nil, attemptRefusal(a.Attempt, "credited-last", "the credited attempt is the last one")
		}
	}
	if outcome, ok := recStr("outcome"); ok && (outcome == "verified" || outcome == "pass") && out[last-1].Disposition != AttemptCredited {
		return nil, attemptRefusal(last, "verified-ends-credited", "a "+outcome+" record's attempts end in a credited attempt")
	}
	return out, nil
}

// AttemptRun is one verification run of a brief as the rollup counts it: an attempt of a
// record, or (Attempt 0, disposition unrecorded) the one run a record without attempts
// stands for. HasWindow is false when that record carries no started/finished.
type AttemptRun struct {
	Source      string
	Attempt     int
	Disposition string
	Reason      string
	HasWindow   bool
	Started     time.Time
	Finished    time.Time
}

// AttemptRollup is the per-brief count over every record. Elapsed is the observed
// finished-started per disposition (not a new measure); ElapsedUnknown counts runs whose
// window the record does not carry, which are left out of Elapsed rather than counted as 0.
type AttemptRollup struct {
	Brief          string
	Runs           int
	ByDisposition  map[string]int
	ByReason       map[string]int
	Elapsed        map[string]time.Duration
	ElapsedUnknown map[string]int
	Attempts       []AttemptRun
	// Undecodable lists records whose attempts could not even be decoded, by source.
	Undecodable []string
}

// RollupAttempts counts every record it is given (all records of ONE brief, from
// ReadVerifyOutcomes, never LatestPerBrief's winner). It does not re-validate: the reader
// does that with CheckAttempts first and passes only the records that pass. It decodes each
// entry's disposition, reason and window; a record whose attempts cannot be decoded at all is
// listed in Undecodable and not counted.
func RollupAttempts(brief string, records []OutcomeRecord) AttemptRollup {
	r := AttemptRollup{Brief: brief, ByDisposition: map[string]int{}, ByReason: map[string]int{},
		Elapsed: map[string]time.Duration{}, ElapsedUnknown: map[string]int{}}
	sorted := append([]OutcomeRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TS != sorted[j].TS {
			return sorted[i].TS < sorted[j].TS
		}
		return sorted[i].Name < sorted[j].Name
	})
	for _, rec := range sorted {
		var f map[string]json.RawMessage
		if json.Unmarshal(rec.Raw, &f) != nil {
			r.Undecodable = append(r.Undecodable, rec.Source)
			continue
		}
		raw, has := f["attempts"]
		if !has {
			run := AttemptRun{Source: rec.Source, Disposition: AttemptUnrecorded}
			var s, e string
			if json.Unmarshal(f["started"], &s) == nil && json.Unmarshal(f["finished"], &e) == nil {
				st, err1 := time.Parse(time.RFC3339, s)
				fi, err2 := time.Parse(time.RFC3339, e)
				if err1 == nil && err2 == nil && !st.After(fi) {
					run.HasWindow, run.Started, run.Finished = true, st.UTC(), fi.UTC()
				}
			}
			r.add(run)
			continue
		}
		var entries []struct {
			Attempt     int    `json:"attempt"`
			Disposition string `json:"disposition"`
			Reason      string `json:"reason"`
			Started     string `json:"started"`
			Finished    string `json:"finished"`
		}
		if json.Unmarshal(raw, &entries) != nil || len(entries) == 0 {
			r.Undecodable = append(r.Undecodable, rec.Source)
			continue
		}
		for _, e := range entries {
			run := AttemptRun{Source: rec.Source, Attempt: e.Attempt, Disposition: e.Disposition, Reason: e.Reason}
			st, ok1 := attemptUTC(e.Started)
			fi, ok2 := attemptUTC(e.Finished)
			if ok1 && ok2 && !st.After(fi) {
				run.HasWindow, run.Started, run.Finished = true, st, fi
			}
			r.add(run)
		}
	}
	return r
}

func (r *AttemptRollup) add(run AttemptRun) {
	r.Runs++
	r.ByDisposition[run.Disposition]++
	if run.Reason != "" {
		r.ByReason[run.Reason]++
	}
	if run.HasWindow {
		r.Elapsed[run.Disposition] += run.Finished.Sub(run.Started)
	} else {
		r.ElapsedUnknown[run.Disposition]++
	}
	r.Attempts = append(r.Attempts, run)
}

// perfBriefInRe splits a frontmatter brief id into its three qualifying segments and number.
var perfBriefInRe = regexp.MustCompile(`^([a-z][a-z0-9-]{0,39}:[a-z][a-z0-9-]{0,39}:[a-z][a-z0-9-]{0,39}):([0-9]{1,12})$`)

// PerfBriefRef maps a brief file's frontmatter `brief:` value (<a>:<b>:<stream>:<NN>) to the
// qualified id a perf work ref carries: the number's leading zeros dropped, and the result
// accepted only when perfrec itself accepts it as a brief work ref. It is the single mapping
// from a brief to its perf work ref — the verify-desk dispatch value and the deskperf reruns
// join both use it, so the writer and reader of the join key cannot drift. Pure.
func PerfBriefRef(frontmatterID string) (string, error) {
	m := perfBriefInRe.FindStringSubmatch(frontmatterID)
	if m == nil {
		return "", Refused("refused: brief id " + strconv.Quote(frontmatterID) +
			" is not <a>:<b>:<stream>:<NN> (a <stream>/<NN> key is never a perf work ref)")
	}
	num := strings.TrimLeft(m[2], "0")
	if num == "" {
		return "", Refused("refused: brief id " + strconv.Quote(frontmatterID) + " has number 0, which no perf work ref accepts")
	}
	id := m[1] + ":" + num
	// perfrec's own validator decides: a work filter it would refuse is a work ref it would drop.
	probe := perfrec.WorkRef{Kind: "brief", Repo: "example/example", Brief: id}
	if _, _, err := perfrec.SelectV3(nil, perfrec.ProfileWorkflow, perfrec.V3Filter{Work: &probe}); err != nil {
		return "", Refused("refused: brief id " + strconv.Quote(frontmatterID) + " maps to " + strconv.Quote(id) +
			", which perfrec does not accept as a brief work ref")
	}
	return id, nil
}
