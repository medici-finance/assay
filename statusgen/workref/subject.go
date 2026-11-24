package workref

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Subject is the exact work a ruling, a piece of evidence or a handoff is about:
// a logical key and an exact revision. Revision zero means no revision was
// supplied; it never equals a positive revision and is never filled in from the
// current brief. A Subject is built only from a resolved Result.
type Subject struct {
	LogicalKey string
	Revision   uint64
	kind       Kind
}

// SubjectOf retains the resolved logical key, kind and exact revision selector.
func SubjectOf(r Result) Subject {
	return Subject{LogicalKey: r.LogicalKey, Revision: r.Revision, kind: r.Kind}
}

// SameSubject is the one subject comparison: equal non-empty logical keys, the
// same work kind (a milestone never equals a brief) and equal revisions. A caller
// builds both sides under one revision regime; zero against a positive revision
// is a different subject here, and callers that must report it as unknown do so
// before comparing.
func SameSubject(approved, presented Subject) bool {
	return approved.LogicalKey != "" && approved.kind != "" &&
		approved.LogicalKey == presented.LogicalKey &&
		approved.kind == presented.kind &&
		approved.Revision == presented.Revision
}

// BriefSubject resolves a brief's declared identity (its board id or brief:
// value, with no revision of its own) and attaches the brief's declared version.
// version zero means the declaration carries no explicit version; it stays zero.
func BriefSubject(declared string, version uint64, declaring *Context, registry Registry) (Subject, error) {
	r, err := Resolve(declared, declaring, registry, TypedBrief)
	if err != nil {
		return Subject{}, err
	}
	if r.Revision != 0 {
		return Subject{}, fmt.Errorf("a declared brief identity carries no revision selector")
	}
	s := SubjectOf(r)
	s.Revision = version
	return s, nil
}

// RulingSubjectPrefix opens the one ruling-subject line a revision-bound human
// ruling carries, for example:
//
//	Ruling-Subject: assay:assay:milestone-support:b-09@1 card=none
const RulingSubjectPrefix = "Ruling-Subject:"

// RulingSubjectCutover is the fixed rollout instant of the ruling-subject bridge.
// Only an unedited ruling comment with no subject line, created by the forge
// strictly before this instant, keeps its historical logical-key-only meaning.
// Both the statusgen decision-gate hold and the desk-tools gate read this value.
const RulingSubjectCutover = "2026-12-01T00:00:00Z"

var (
	rulingSubjectRe = regexp.MustCompile(`^Ruling-Subject: (\S+) card=(none|sha256:[0-9a-f]{64})$`)
	cardDigestRe    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// RulingSubjectLine is the parsed subject line: the typed brief reference the
// human ruled on and the card they saw ("none" or "sha256:<hex>").
type RulingSubjectLine struct {
	Reference string
	Card      string
}

// ParseRulingSubject reads the comment's one subject line. No line returns nil,
// nil. A malformed line, or more than one line, is an error; it never falls back
// to the legacy meaning.
func ParseRulingSubject(comment string) (*RulingSubjectLine, error) {
	var found *RulingSubjectLine
	for _, line := range strings.Split(strings.ReplaceAll(comment, "\r\n", "\n"), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), RulingSubjectPrefix) {
			continue
		}
		m := rulingSubjectRe.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("malformed %s line", RulingSubjectPrefix)
		}
		if found != nil {
			return nil, fmt.Errorf("more than one %s line", RulingSubjectPrefix)
		}
		found = &RulingSubjectLine{Reference: m[1], Card: m[2]}
	}
	return found, nil
}

// CardDigest is the digest a subject line records for a typed card: sha256 over
// the exact UTF-8 bytes of the card block, delimiters included.
func CardDigest(block string) string {
	sum := sha256.Sum256([]byte(block))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// BeforeCutover reports whether a forge creation time is strictly before the
// cutover. known is false when the time is absent or unreadable.
func BeforeCutover(createdAt string) (before, known bool) {
	created, err := time.Parse(time.RFC3339, strings.TrimSpace(createdAt))
	if err != nil {
		return false, false
	}
	cut, err := time.Parse(time.RFC3339, RulingSubjectCutover)
	if err != nil {
		return false, false
	}
	return created.Before(cut), true
}

// CardObservation is what the caller's own read of the decision issue found.
// Block is the exact card block; Brief and Version are its parsed fields.
// Err reports a card that is present but malformed, ambiguous or unreadable.
type CardObservation struct {
	Present bool
	Block   string
	Brief   string
	Version uint64
	Err     error
}

// RulingEvidence is the fetched, unedited human ruling comment, its forge
// creation time and the caller's card observation of the current issue.
type RulingEvidence struct {
	Comment   string
	CreatedAt string
	Card      CardObservation
}

// BindingState is three-state: bound, refused (checked-failed) or could-not-check.
type BindingState string

const (
	Bound         BindingState = "bound"
	Refused       BindingState = "checked-failed"
	CouldNotCheck BindingState = "could-not-check"
)

// Binding reason codes. Each names one failed condition.
const (
	ReasonSubjectLine  = "subject-line"  // malformed, duplicate or unrevisioned line
	ReasonSubject      = "subject"       // the ruled subject is not the presented one
	ReasonBriefVersion = "brief-version" // the brief declares no explicit version
	ReasonCard         = "card"          // the card the human saw is not the card now
	ReasonCutover      = "cutover"       // no line, and not eligible for the legacy meaning
)

// RulingBinding is the result of binding a ruling to a presented brief subject.
// Legacy is true when the comment keeps the historical logical-key-only meaning.
type RulingBinding struct {
	State  BindingState
	Reason string
	Detail string
	Legacy bool
}

// BindRulingSubject applies the bridge's subject, card and cutover rules to an
// already admitted ruling: the caller has independently checked the comment's
// issue, edit timestamps, actor and human mapping, and the issue's marker.
// presented is the judged brief's subject: its resolved key and explicit version
// (zero when the brief declares none). Nothing is fetched or inferred here; the
// result can only refuse or bind, never grant.
func BindRulingSubject(e RulingEvidence, presented Subject, declaring *Context, registry Registry) RulingBinding {
	fail := func(state BindingState, reason, format string, a ...any) RulingBinding {
		return RulingBinding{State: state, Reason: reason, Detail: fmt.Sprintf(format, a...)}
	}
	line, err := ParseRulingSubject(e.Comment)
	if err != nil {
		return fail(Refused, ReasonSubjectLine, "the ruling comment's subject line is not usable: %v", err)
	}
	if line == nil {
		before, known := BeforeCutover(e.CreatedAt)
		switch {
		case !known:
			return fail(CouldNotCheck, ReasonCutover, "the ruling comment carries no %s line and its creation time %q could not be read, so whether it predates the %s cutover could not be checked", RulingSubjectPrefix, e.CreatedAt, RulingSubjectCutover)
		case !before:
			return fail(Refused, ReasonCutover, "the ruling comment carries no %s line and was created at or after the %s cutover — a ruling now names the subject it rules on", RulingSubjectPrefix, RulingSubjectCutover)
		}
		return RulingBinding{State: Bound, Legacy: true, Detail: "historical ruling: no subject line, created before the cutover"}
	}
	ruled, err := Resolve(line.Reference, declaring, registry, TypedBrief)
	if err != nil {
		return fail(Refused, ReasonSubject, "the ruling comment's subject %q does not resolve: %v", line.Reference, err)
	}
	if ruled.Revision == 0 {
		return fail(Refused, ReasonSubjectLine, "the ruling comment's subject %q names no revision", line.Reference)
	}
	ruledSubject := SubjectOf(ruled)
	if presented.Revision == 0 {
		return fail(Refused, ReasonBriefVersion, "the judged brief declares no explicit version:, so it cannot carry a ruling on revision %d", ruled.Revision)
	}
	if !SameSubject(ruledSubject, presented) {
		return fail(Refused, ReasonSubject, "the ruling comment rules on %s revision %d, not on the judged brief %s revision %d", ruled.LogicalKey, ruled.Revision, presented.LogicalKey, presented.Revision)
	}
	c := e.Card
	if c.Err != nil {
		return fail(CouldNotCheck, ReasonCard, "the decision issue's card could not be read: %v", c.Err)
	}
	if line.Card == "none" {
		if c.Present {
			return fail(CouldNotCheck, ReasonCard, "the ruling comment declared card=none, but the decision issue now carries a card")
		}
		return RulingBinding{State: Bound}
	}
	if !c.Present {
		return fail(CouldNotCheck, ReasonCard, "the ruling comment declared card %s, but the decision issue carries no card", line.Card)
	}
	if !cardDigestRe.MatchString(line.Card) || CardDigest(c.Block) != line.Card {
		return fail(CouldNotCheck, ReasonCard, "the decision issue's card is not the card the ruling declared (%s)", line.Card)
	}
	cardSubject, err := BriefSubject(c.Brief, c.Version, declaring, registry)
	if err != nil {
		return fail(CouldNotCheck, ReasonCard, "the decision issue's card brief %q does not resolve: %v", c.Brief, err)
	}
	if c.Version == 0 || !SameSubject(cardSubject, ruledSubject) {
		return fail(CouldNotCheck, ReasonCard, "the decision issue's card names %s revision %d, not the ruled subject", cardSubject.LogicalKey, c.Version)
	}
	return RulingBinding{State: Bound}
}

// SubjectString renders a subject for diagnostics.
func (s Subject) String() string {
	if s.Revision == 0 {
		return s.LogicalKey
	}
	return s.LogicalKey + "@" + strconv.FormatUint(s.Revision, 10)
}
