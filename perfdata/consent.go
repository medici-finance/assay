package perfdata

// consent.go — the two-switch collection consent and the local collection policy.
//
// Local writing is armed only when ASSAY_TELEMETRY is exactly "1" AND a
// performance-event-v1 profile is explicitly selected (a --profile flag or
// ASSAY_PERFDATA_PROFILE). The profile selector is a separate variable on purpose: an
// existing telemetry-v1 or perf-v1 opt-in never becomes consent to this record class,
// and no profile is ever implied. Off creates nothing.

import (
	"encoding/json"
	"os"
)

// EnvTelemetry is the shared telemetry switch.
const EnvTelemetry = "ASSAY_TELEMETRY"

// EnvProfile selects the performance-event-v1 collection profile.
const EnvProfile = "ASSAY_PERFDATA_PROFILE"

// ConsentReason explains a consent resolution with a fixed token.
type ConsentReason string

const (
	ConsentArmed          ConsentReason = "armed"
	ConsentTelemetryOff   ConsentReason = "telemetry-off"
	ConsentNoProfile      ConsentReason = "no-profile"
	ConsentUnknownProfile ConsentReason = "unknown-profile"
)

// Consent is the resolved collection switch.
type Consent struct {
	Armed   bool          `json:"armed"`
	Profile Profile       `json:"profile,omitempty"`
	Reason  ConsentReason `json:"reason"`
}

// ResolveConsent resolves consent from the environment and an optional explicit flag
// value (the flag wins over the variable). An unknown profile name disarms; it is
// never widened to another profile.
func ResolveConsent(getenv func(string) string, flagProfile string) Consent {
	if getenv == nil {
		getenv = os.Getenv
	}
	if getenv(EnvTelemetry) != "1" {
		return Consent{Reason: ConsentTelemetryOff}
	}
	p := flagProfile
	if p == "" {
		p = getenv(EnvProfile)
	}
	switch Profile(p) {
	case "":
		return Consent{Reason: ConsentNoProfile}
	case ProfileTool, ProfileWorkflow:
		return Consent{Armed: true, Profile: Profile(p), Reason: ConsentArmed}
	}
	return Consent{Reason: ConsentUnknownProfile}
}

// Policy defaults and bounds.
const (
	DefaultSegmentBytes  = 1 << 20
	DefaultQueue         = 256
	DefaultFlushBudgetMs = 250
	MaxAgeCapDays        = 7
	MaxByteCap           = 1 << 30
	MaxDistributionDays  = 90
	MaxPolicyTools       = 256
)

// Policy is the selected local collection policy. Its revision is stamped on every
// event. ByteCap has no default: a policy without an explicit byte budget is refused.
type Policy struct {
	Schema           string    `json:"schema"`
	Revision         string    `json:"revision"`
	Profiles         []Profile `json:"profiles"`
	Tools            []string  `json:"tools"`
	AgeCapDays       int       `json:"ageCapDays"`
	ByteCap          int64     `json:"byteCap"`
	SegmentBytes     int64     `json:"segmentBytes,omitempty"`
	Queue            int       `json:"queue,omitempty"`
	FlushBudgetMs    int       `json:"flushBudgetMs,omitempty"`
	DistributionDays int       `json:"distributionDays,omitempty"`
}

// DecodePolicy strictly decodes, validates and defaults a policy document.
func DecodePolicy(b []byte) (Policy, error) {
	var p Policy
	if err := strictUnmarshal(b, &p); err != nil {
		return Policy{}, err
	}
	if err := p.Validate(); err != nil {
		return Policy{}, err
	}
	return p.withDefaults(), nil
}

func (p Policy) withDefaults() Policy {
	if p.SegmentBytes == 0 {
		p.SegmentBytes = DefaultSegmentBytes
	}
	if p.Queue == 0 {
		p.Queue = DefaultQueue
	}
	if p.FlushBudgetMs == 0 {
		p.FlushBudgetMs = DefaultFlushBudgetMs
	}
	return p
}

// Validate applies the policy bounds.
func (p Policy) Validate() error {
	if p.Schema != PolicySchema {
		return refuse(RefuseUnsupported, "schema", "unsupported policy schema")
	}
	if !revisionRE.MatchString(p.Revision) {
		return refuse(RefuseSchema, "revision", "policy revision is not a bounded token")
	}
	if len(p.Profiles) == 0 {
		return refuse(RefuseSchema, "profiles", "a policy names the profiles it admits")
	}
	for _, pr := range p.Profiles {
		if pr != ProfileTool && pr != ProfileWorkflow {
			return refuse(RefuseSchema, "profiles", "unknown profile")
		}
	}
	if len(p.Tools) == 0 || len(p.Tools) > MaxPolicyTools {
		return refuse(RefuseSchema, "tools", "a policy names a bounded tool vocabulary")
	}
	for _, t := range p.Tools {
		if !tokenRE.MatchString(t) {
			return refuse(RefuseSchema, "tools", "tool vocabulary entries are bounded tokens")
		}
	}
	if p.AgeCapDays < 1 || p.AgeCapDays > MaxAgeCapDays {
		return refuse(RefuseBounds, "ageCapDays", "age cap is 1 to 7 UTC days")
	}
	if p.ByteCap <= 0 || p.ByteCap > MaxByteCap {
		return refuse(RefuseBounds, "byteCap", "an explicit byte cap up to 1 GiB is required")
	}
	if p.SegmentBytes < 0 || (p.SegmentBytes > 0 && (p.SegmentBytes < 4*MaxEventBytes || p.SegmentBytes > p.ByteCap)) {
		return refuse(RefuseBounds, "segmentBytes", "segment size must hold several events and fit the byte cap")
	}
	if p.Queue < 0 || p.Queue > 65536 {
		return refuse(RefuseBounds, "queue", "queue bound out of range")
	}
	if p.FlushBudgetMs < 0 || p.FlushBudgetMs > 10_000 {
		return refuse(RefuseBounds, "flushBudgetMs", "flush budget out of range")
	}
	if p.DistributionDays < 0 || p.DistributionDays > MaxDistributionDays {
		return refuse(RefuseBounds, "distributionDays", "distribution retention is 0 to 90 days")
	}
	return nil
}

// Admits reports whether the policy admits profile pr.
func (p Policy) Admits(pr Profile) bool { return containsProfile(p.Profiles, pr) }

// AdmitsTool reports whether tool is in the policy's tool vocabulary.
func (p Policy) AdmitsTool(tool string) bool {
	for _, t := range p.Tools {
		if t == tool {
			return true
		}
	}
	return false
}

// CheckEvent applies the policy to a valid event: admitted profile, matching revision
// and, for tool completions, a tool inside the selected vocabulary (anything else is a
// privacy refusal, so a mistyped or free-form tool name never reaches the spool).
func (p Policy) CheckEvent(e Event) error {
	if !p.Admits(e.Profile) {
		return refuse(RefusePrivacy, "profile", "profile not admitted by the selected policy")
	}
	if e.Policy != p.Revision {
		return refuse(RefuseSchema, "policy", "event policy revision differs from the selected policy")
	}
	if e.Kind == KindTool {
		var tc ToolCompletion
		if err := json.Unmarshal(e.Payload, &tc); err != nil || !p.AdmitsTool(tc.Tool) {
			return refuse(RefusePrivacy, "payload.tool", "tool is outside the selected policy vocabulary")
		}
	}
	return nil
}
