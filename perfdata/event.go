// Package perfdata is the performance-event-v1 contract: the frozen, versioned record of
// one bounded performance observation, its strict validation, its batch/receipt envelope
// and the pure reducers every downstream reader shares.
//
// The package is standard-library only and performs no I/O. Storage lives in
// perfdata/local, reporting in perfdata/report and the thin commands in
// perfdata/cmd/perfdata. The specification is spec/performance-event-v1.md.
//
// Three rules shape every type here:
//
//   - Missing is not zero. Every measurement carries an explicit basis; "unavailable"
//     carries no value and a measured value is never absent.
//   - Nothing free-form crosses the boundary. Every string is an enum, a bounded token,
//     a random alias or a UTC timestamp. Refusals name the rule that failed, never the
//     refused input.
//   - Nothing is counted twice. Request counters are own-process only, inclusive CPU is
//     never added to its children, and cumulative usage snapshots are never summed.
package perfdata

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Schema identifiers. A reader refuses any other value for the field it checks; a new
// meaning gets a new identifier rather than a reinterpretation of an old one.
const (
	EventSchema    = "performance-event-v1"
	BatchSchema    = "performance-batch-v1"
	ReceiptSchema  = "performance-receipt-v1"
	SnapshotSchema = "performance-snapshot-v1"
	PolicySchema   = "performance-policy-v1"
	RollupSchema   = "performance-rollup-v1"

	// InstrumentName and InstrumentVersion identify this writer on every event.
	InstrumentName    = "perfdata"
	InstrumentVersion = "1"

	// MaxEventBytes bounds one canonical event line (without its newline).
	MaxEventBytes = 8192

	// TimeLayout is the only accepted timestamp form: UTC, millisecond precision.
	TimeLayout = "2006-01-02T15:04:05.000Z"
)

// Profile is the selected collection profile.
type Profile string

const (
	// ProfileTool carries fixed-vocabulary measurements and an unlinked invocation
	// identity only. It never carries session, agent, parent, attempt or work context.
	ProfileTool Profile = "tool"
	// ProfileWorkflow additionally carries aliased correlation context.
	ProfileWorkflow Profile = "workflow"
)

// Kind is the observation kind. The v1 set is closed: an unknown kind is refused.
type Kind string

const (
	KindTool       Kind = "tool.completion"
	KindPhase      Kind = "phase"
	KindAttempt    Kind = "attempt"
	KindUsage      Kind = "usage"
	KindHTTP       Kind = "http"
	KindCall       Kind = "service.call"
	KindHandler    Kind = "service.handler"
	KindHealth     Kind = "health"
	KindCorrection Kind = "correction"
)

var kindProfiles = map[Kind][]Profile{
	KindTool:       {ProfileTool, ProfileWorkflow},
	KindPhase:      {ProfileTool, ProfileWorkflow},
	KindHTTP:       {ProfileTool, ProfileWorkflow},
	KindCall:       {ProfileTool, ProfileWorkflow},
	KindHandler:    {ProfileTool, ProfileWorkflow},
	KindHealth:     {ProfileTool, ProfileWorkflow},
	KindCorrection: {ProfileTool, ProfileWorkflow},
	KindAttempt:    {ProfileWorkflow},
	KindUsage:      {ProfileWorkflow},
}

// Instrument names the writer and its contract version.
type Instrument struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Build is the producer build and platform, bounded tokens only.
type Build struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// Event is one stored performance-event-v1 record. Its canonical encoding is
// json.Marshal of this struct; DecodeEvent refuses any other byte form.
type Event struct {
	Schema     string          `json:"schema"`
	ID         string          `json:"id"`
	Source     string          `json:"source"`
	Epoch      uint32          `json:"epoch"`
	Seq        uint64          `json:"seq"`
	Profile    Profile         `json:"profile"`
	Policy     string          `json:"policy"`
	Instrument Instrument      `json:"instrument"`
	Build      Build           `json:"build"`
	OccurredAt string          `json:"occurredAt"`
	RecordedAt string          `json:"recordedAt"`
	Kind       Kind            `json:"kind"`
	Ctx        *Context        `json:"ctx,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

// EventID is the immutable identity derived from source, epoch and sequence.
func EventID(source string, epoch uint32, seq uint64) string {
	return source + "-e" + strconv.FormatUint(uint64(epoch), 10) + "-s" + strconv.FormatUint(seq, 10)
}

var (
	sourceRE   = regexp.MustCompile(`^src-[a-z2-7]{26}$`)
	aliasRE    = regexp.MustCompile(`^pa-[a-z2-7]{26}$`)
	tokenRE    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	revisionRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)
	platformRE = regexp.MustCompile(`^[a-z0-9]{1,16}$`)
	eventIDRE  = regexp.MustCompile(`^src-[a-z2-7]{26}-e[1-9][0-9]{0,9}-s[1-9][0-9]{0,19}$`)
	digestRE   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	currencyRE = regexp.MustCompile(`^[A-Z]{3}$`)
)

// IsAlias reports whether s has the random alias form.
func IsAlias(s string) bool { return aliasRE.MatchString(s) }

// IsSource reports whether s has the random source identity form.
func IsSource(s string) bool { return sourceRE.MatchString(s) }

// IsToken reports whether s is a bounded lowercase vocabulary token.
func IsToken(s string) bool { return tokenRE.MatchString(s) }

// ParseTime parses the canonical timestamp form.
func ParseTime(s string) (time.Time, bool) {
	t, err := time.Parse(TimeLayout, s)
	if err != nil || t.Format(TimeLayout) != s {
		return time.Time{}, false
	}
	return t, true
}

// FormatTime renders t in the canonical UTC millisecond form.
func FormatTime(t time.Time) string { return t.UTC().Truncate(time.Millisecond).Format(TimeLayout) }

// Encode returns the canonical bytes of e (no trailing newline) after validating it.
func (e Event) Encode() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e.encode()
}

func (e Event) encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, refuse(RefuseSchema, "", "unencodable")
	}
	b := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if len(b) > MaxEventBytes {
		return nil, refuse(RefuseBounds, "", "event exceeds MaxEventBytes")
	}
	return b, nil
}

// Digest is the hex SHA-256 of the canonical bytes; identical duplicates share it.
func Digest(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// DecodeEvent strictly decodes one stored event: unknown fields, trailing data,
// non-canonical bytes (including duplicate keys and reordered fields) and every
// Validate rule are refused.
func DecodeEvent(b []byte) (Event, error) {
	b = bytes.TrimSuffix(b, []byte("\n"))
	if len(b) > MaxEventBytes {
		return Event{}, refuse(RefuseBounds, "", "event exceeds MaxEventBytes")
	}
	var e Event
	if err := strictUnmarshal(b, &e); err != nil {
		return Event{}, err
	}
	if err := e.Validate(); err != nil {
		return Event{}, err
	}
	canon, err := e.encode()
	if err != nil {
		return Event{}, err
	}
	if !bytes.Equal(canon, b) {
		return Event{}, refuse(RefuseSchema, "", "non-canonical encoding")
	}
	return e, nil
}

// strictUnmarshal decodes exactly one JSON value with unknown fields refused. The
// decoder's own error text can quote input (an unknown field's name), so it is replaced
// by a fixed rule and never surfaced.
func strictUnmarshal(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return refuse(RefuseSchema, "", "malformed or unknown field")
	}
	if _, err := dec.Token(); err != io.EOF {
		return refuse(RefuseSchema, "", "trailing data")
	}
	return nil
}

// Validate applies every v1 rule to e and its typed payload.
func (e Event) Validate() error {
	if e.Schema != EventSchema {
		return refuse(RefuseUnsupported, "schema", "unsupported event schema")
	}
	if !sourceRE.MatchString(e.Source) {
		return refuse(RefusePrivacy, "source", "source is not a random source identity")
	}
	if e.Epoch == 0 || e.Seq == 0 {
		return refuse(RefuseSchema, "seq", "epoch and seq start at 1")
	}
	if e.ID != EventID(e.Source, e.Epoch, e.Seq) || !eventIDRE.MatchString(e.ID) {
		return refuse(RefuseSchema, "id", "id does not derive from source, epoch and seq")
	}
	if e.Profile != ProfileTool && e.Profile != ProfileWorkflow {
		return refuse(RefuseSchema, "profile", "unknown profile")
	}
	if !revisionRE.MatchString(e.Policy) {
		return refuse(RefusePrivacy, "policy", "policy revision is not a bounded token")
	}
	if e.Instrument.Name != InstrumentName || e.Instrument.Version != InstrumentVersion {
		return refuse(RefuseUnsupported, "instrument", "unsupported instrument version")
	}
	if !revisionRE.MatchString(e.Build.Version) || !platformRE.MatchString(e.Build.OS) || !platformRE.MatchString(e.Build.Arch) {
		return refuse(RefusePrivacy, "build", "build fields are not bounded tokens")
	}
	if _, ok := ParseTime(e.OccurredAt); !ok {
		return refuse(RefuseSchema, "occurredAt", "not a canonical UTC timestamp")
	}
	if _, ok := ParseTime(e.RecordedAt); !ok {
		return refuse(RefuseSchema, "recordedAt", "not a canonical UTC timestamp")
	}
	profiles, ok := kindProfiles[e.Kind]
	if !ok {
		return refuse(RefuseUnsupported, "kind", "unsupported kind")
	}
	if !containsProfile(profiles, e.Profile) {
		return refuse(RefusePrivacy, "kind", "kind requires the workflow profile")
	}
	if err := e.Ctx.validate(e.Profile, e.Kind); err != nil {
		return err
	}
	typed, err := DecodePayload(e.Kind, e.Payload)
	if err != nil {
		return err
	}
	if c, ok := typed.(*Correction); ok {
		if !strings.HasPrefix(c.Corrects, e.Source+"-e") || c.Corrects == e.ID {
			return refuse(RefuseSchema, "payload.corrects", "a correction names an earlier event of the same source")
		}
	}
	canon, err := json.Marshal(typed)
	if err != nil || !bytes.Equal(canon, e.Payload) {
		return refuse(RefuseSchema, "payload", "non-canonical payload")
	}
	return nil
}

func containsProfile(ps []Profile, p Profile) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

// RefusalClass is the bounded class of a refused record.
type RefusalClass string

const (
	// RefuseSchema: malformed, unknown field, non-canonical, inconsistent identity.
	RefuseSchema RefusalClass = "schema"
	// RefuseUnsupported: unknown schema, instrument version or kind.
	RefuseUnsupported RefusalClass = "unsupported"
	// RefusePrivacy: free text, raw locator, profile escalation.
	RefusePrivacy RefusalClass = "privacy"
	// RefuseMeasurement: false zero, unknown-as-zero, negative, inconsistent or
	// double-counted values.
	RefuseMeasurement RefusalClass = "measurement"
	// RefuseBounds: size or count limits.
	RefuseBounds RefusalClass = "bounds"
)

// Refusal is a refused record. Field is a fixed JSON path and Rule a fixed sentence
// from this package; neither ever contains the refused input.
type Refusal struct {
	Class RefusalClass `json:"class"`
	Field string       `json:"field,omitempty"`
	Rule  string       `json:"rule"`
}

func (r *Refusal) Error() string {
	if r.Field == "" {
		return fmt.Sprintf("perfdata: refused (%s): %s", r.Class, r.Rule)
	}
	return fmt.Sprintf("perfdata: refused (%s) %s: %s", r.Class, r.Field, r.Rule)
}

func refuse(c RefusalClass, field, rule string) *Refusal {
	return &Refusal{Class: c, Field: field, Rule: rule}
}

// AsRefusal extracts a *Refusal from err, or reports a schema refusal for any other
// error so a caller never prints foreign error text.
func AsRefusal(err error) *Refusal {
	if err == nil {
		return nil
	}
	if r, ok := err.(*Refusal); ok {
		return r
	}
	return refuse(RefuseSchema, "", "invalid")
}
