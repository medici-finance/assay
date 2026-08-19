package main

// emit.go — the day-file: what it may contain, and what it may never contain.
//
// THE ONE RULE. Every field below is a COUNT, a RATIO, a PERCENTILE, a fixed-vocabulary
// status, or a date. There is no field that can hold message text, a prompt excerpt, a
// file path from a transcript, a session id, a branch name, a PR title, or a person's
// name — and that is enforced three ways, not asserted once:
//
//  1. STRUCTURALLY. TestEmitStringFieldsAreAllowlisted reflects over the whole Report
//     type and fails if any string-typed leaf appears whose JSON path is not on a
//     pinned allowlist. Adding `SampleMessage string` breaks the build's tests before
//     it can ever run against a real transcript.
//  2. BY VOCABULARY. Every string that IS allowed is drawn from a closed enum (status,
//     reason, basis) or is a date/version literal. TestEmitStringValuesAreEnumerated
//     walks the EMITTED json and fails on any string outside those vocabularies.
//  3. BY SENTINEL. TestLeakNoTranscriptContent plants secrets and distinctive strings
//     in fixture transcripts, runs the real emit path end to end, and fails if any of
//     them — or ANY 12-character substring of ANY fixture message — reaches the file.
//
// A "reason" is a CODE, never a sentence. Sentences are where paths leak: "cannot read
// /Users/<name>/.claude/projects/..." is a home-directory disclosure committed to a
// repo. Human-readable detail goes to stderr, which nobody commits.
//
// THREE STATES, NOT TWO. A block whose inputs could not be read reports
// status=could-not-check with a reason code and leaves its numbers null. It NEVER
// reports zero. Zero and could-not-check are different findings: one says the operator
// sent no relays, the other says the collector is blind. Collapsing them is how a
// broken collector reads as a perfect day.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SchemaVersion identifies the day-file shape. Downstream consumers select on it.
// Bump on any field REMOVAL or meaning change; additive fields do
// not require a bump.
const SchemaVersion = "opmetrics/1"

// Status is the three-state marker every block carries.
type Status string

const (
	StatusOK            Status = "ok"
	StatusCouldNotCheck Status = "could-not-check"
	StatusNoPriorData   Status = "no-prior-data"
)

// Reason is a CLOSED vocabulary of machine codes. Never a sentence, never a path.
type Reason string

const (
	ReasonNone               Reason = ""
	ReasonTranscriptsUnread  Reason = "transcripts-unreadable"
	ReasonNoTranscriptFiles  Reason = "no-transcript-files"
	ReasonGHNotSupplied      Reason = "gh-json-not-supplied"
	ReasonGHUnreadable       Reason = "gh-json-unreadable"
	ReasonNoMergedPRs        Reason = "no-merged-prs"
	ReasonNoLatencySamples   Reason = "no-latency-samples"
	ReasonRosterUnreadable   Reason = "roster-dir-unreadable"
	ReasonClaimsUnreadable   Reason = "claims-dir-unreadable"
	ReasonNoPriorDayFiles    Reason = "no-prior-day-files"
	ReasonNoOperatorMessages Reason = "no-operator-messages"
)

// AllReasons is the closed set, exported for the vocabulary test.
var AllReasons = []Reason{
	ReasonNone, ReasonTranscriptsUnread, ReasonNoTranscriptFiles, ReasonGHNotSupplied,
	ReasonGHUnreadable, ReasonNoMergedPRs, ReasonNoLatencySamples, ReasonRosterUnreadable,
	ReasonClaimsUnreadable, ReasonNoPriorDayFiles, ReasonNoOperatorMessages,
}

// Unmeasured codes. Stated, not silently omitted: a metric this collector deliberately
// does not compute is a fact a reader needs, and prompt blockage was asked for.
const (
	UnmeasuredPromptBlockage = "prompt-blockage"
	UnmeasuredThinkTime      = "operator-think-time"
)

// AllUnmeasured is the closed set, exported for the vocabulary test.
var AllUnmeasured = []string{UnmeasuredPromptBlockage, UnmeasuredThinkTime}

// RelayFamilies is the fixed relay-family breakdown. Fixed keys, so the JSON has no
// dynamic map keys anywhere — a dynamic key is a string the allowlist cannot pin.
type RelayFamilies struct {
	Sync      int `json:"sync"`
	StateEcho int `json:"state_echo"`
	Poke      int `json:"poke"`
	Lookup    int `json:"lookup"`
	Duplicate int `json:"duplicate"`
}

// OperatorBlock is the relay-ratio metric.
type OperatorBlock struct {
	Status              Status        `json:"status"`
	Reason              Reason        `json:"reason,omitempty"`
	MessagesTotal       *int          `json:"messages_total"`
	MessagesClassified  *int          `json:"messages_classified"`
	RelayMessages       *int          `json:"relay_messages"`
	SubstantiveMessages *int          `json:"substantive_messages"`
	EmptyMessages       *int          `json:"empty_messages"`
	RelayRatio          *float64      `json:"relay_ratio"`
	RelayFamilies       RelayFamilies `json:"relay_families"`
	TranscriptFiles     int           `json:"transcript_files"`
	UnparseableLines    int           `json:"unparseable_lines"`
}

// InterventionBlock is operator messages per merged PR.
type InterventionBlock struct {
	Status              Status   `json:"status"`
	Reason              Reason   `json:"reason,omitempty"`
	OperatorMessages    *int     `json:"operator_messages"`
	MergedPRs           *int     `json:"merged_prs"`
	MessagesPerMergedPR *float64 `json:"messages_per_merged_pr"`
}

// LatencyBasis records which clock start each sample used — the honesty field. See
// latency.go: a createdAt-based sample is an UPPER BOUND on decision latency.
type LatencyBasis struct {
	ReadyFlip       int `json:"ready_flip"`
	DecisionRequest int `json:"decision_request"`
	CreatedAt       int `json:"created_at"`
}

// LatencyBlock is decision latency percentiles.
type LatencyBlock struct {
	Status     Status       `json:"status"`
	Reason     Reason       `json:"reason,omitempty"`
	Samples    *int         `json:"samples"`
	P50Minutes *int         `json:"p50_minutes"`
	P90Minutes *int         `json:"p90_minutes"`
	Basis      LatencyBasis `json:"basis"`
}

// HygieneBlock is session hygiene: long sessions, zombies, claim volume.
type HygieneBlock struct {
	Status           Status `json:"status"`
	Reason           Reason `json:"reason,omitempty"`
	SessionsObserved *int   `json:"sessions_observed"`
	SessionsOver24h  *int   `json:"sessions_over_24h"`
	RosterBeacons    *int   `json:"roster_beacons"`
	ZombieAgents     *int   `json:"zombie_agents"`
	ClaimsFiled      *int   `json:"claims_filed"`
	ClaimsOpen       *int   `json:"claims_open"`
	SessionsActive   *int   `json:"sessions_active"`
	UnparseableFiles int    `json:"unparseable_files"`
}

// CorrectionBlock counts recurring-correction CANDIDATES. Candidates, not findings —
// the heuristic cannot tell "the same rule twice" from "two rules sharing vocabulary".
type CorrectionBlock struct {
	Status              Status `json:"status"`
	Reason              Reason `json:"reason,omitempty"`
	CorrectiveMessages  *int   `json:"corrective_messages"`
	RecurrenceCandidate *int   `json:"recurrence_candidates"`
}

// TrendDelta is one headline number's movement against the prior window.
type TrendDelta struct {
	PriorMean *float64 `json:"prior_mean"`
	Delta     *float64 `json:"delta"`
}

// TrendBlock is the optional --trend N comparison against prior day-files.
type TrendBlock struct {
	Status              Status     `json:"status"`
	Reason              Reason     `json:"reason,omitempty"`
	WindowDays          int        `json:"window_days"`
	PriorFilesFound     int        `json:"prior_files_found"`
	ClassifierMismatch  int        `json:"prior_files_other_classifier"`
	RelayRatio          TrendDelta `json:"relay_ratio"`
	MessagesPerMergedPR TrendDelta `json:"messages_per_merged_pr"`
}

// Report is the whole day-file. Every string leaf in this type is pinned by
// TestEmitStringFieldsAreAllowlisted.
type Report struct {
	Schema            string `json:"schema"`
	Date              string `json:"date"`
	GeneratedAt       string `json:"generatedAt"`
	ClassifierVersion string `json:"classifierVersion"`

	// RelayRatio is the headline, mirrored to the top level so a consumer can read
	// it without walking into a block. Null when the operator block could not be
	// computed — never 0, which would read as "a perfect day".
	RelayRatio *float64 `json:"relay_ratio"`

	Operator             OperatorBlock     `json:"operator"`
	Intervention         InterventionBlock `json:"intervention"`
	DecisionLatency      LatencyBlock      `json:"decision_latency"`
	SessionHygiene       HygieneBlock      `json:"session_hygiene"`
	CorrectionRecurrence CorrectionBlock   `json:"correction_recurrence"`

	// Unmeasured lists, by code, what this collector deliberately does not measure.
	Unmeasured []string    `json:"unmeasured"`
	Trend      *TrendBlock `json:"trend,omitempty"`
}

func iptr(v int) *int         { return &v }
func fptr(v float64) *float64 { return &v }

// DayFilePath is where a report for date lands under a repo root.
func DayFilePath(root, date string) string {
	return filepath.Join(root, "docs", "reports", "daily", date, "opmetrics.json")
}

// marshalReport is the single serialiser. --stdout and the day-file writer both go
// through it, so the bytes the leak test scans are byte-identical to the bytes a
// reader would see either way. Two serialisers would be two things to keep leak-free.
func marshalReport(r Report) ([]byte, error) {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling report: %w", err)
	}
	return append(raw, '\n'), nil
}

// WriteReport marshals r and writes it to the day-file path under root, creating the
// directory. The bytes written are exactly the bytes returned, so the leak test can
// scan the same artifact the repo would receive.
func WriteReport(root, date string, r Report) (string, []byte, error) {
	raw, err := marshalReport(r)
	if err != nil {
		return "", nil, err
	}
	path := DayFilePath(root, date)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", nil, fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", nil, fmt.Errorf("writing %s: %w", path, err)
	}
	return path, raw, nil
}
