package perfdata

// alias.go — producer input (Draft) and the translation of its local, identifiable
// locators into scoped random aliases before anything reaches the spool, a preview or
// an upload. The private locator→alias mapping lives only in the AliasMapper (the local
// store keeps it in its own 0600 file per source epoch); an Event never contains a
// locator, and the mapping is never part of an event, batch or report.

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Draft is one producer observation before identity, aliasing and sealing.
type Draft struct {
	Kind       Kind            `json:"kind"`
	OccurredAt string          `json:"occurredAt,omitempty"`
	Ctx        *RawContext     `json:"ctx,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

// RawContext is local correlation input. Every locator is an opaque local value (a
// native harness ID, a launcher ID) and is only ever passed through the mapper.
type RawContext struct {
	Invocation       string    `json:"invocation,omitempty"`
	ParentInvocation string    `json:"parentInvocation,omitempty"`
	Session          string    `json:"session,omitempty"`
	Agent            string    `json:"agent,omitempty"`
	ParentAgent      string    `json:"parentAgent,omitempty"`
	Attempt          string    `json:"attempt,omitempty"`
	SharedRefresh    string    `json:"sharedRefresh,omitempty"`
	DeskRole         string    `json:"deskRole,omitempty"`
	Role             string    `json:"role,omitempty"`
	Work             []RawWork `json:"work,omitempty"`
}

// RawWork is a typed, qualified work locator: a brief is repository scope plus the
// qualified brief ID (and stable UUID when known); an issue is forge instance plus
// repository plus number, so identically numbered issues in different repositories or
// instances never share an alias.
type RawWork struct {
	Kind      string `json:"kind"`
	Repo      string `json:"repo,omitempty"`
	Brief     string `json:"brief,omitempty"`
	BriefUUID string `json:"briefUuid,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Number    int    `json:"number,omitempty"`
	Primary   bool   `json:"primary,omitempty"`
}

// MaxLocatorBytes bounds one raw locator component.
const MaxLocatorBytes = 256

// AliasMapper returns the stable alias of a qualified locator key within the current
// source epoch, minting a new random alias the first time.
type AliasMapper interface {
	Alias(key string) (string, error)
}

// NewAlias returns a fresh random alias (128 random bits).
func NewAlias() string { return "pa-" + random26() }

// NewSource returns a fresh random source identity (128 random bits).
func NewSource() string { return "src-" + random26() }

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func random26() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("perfdata: no randomness") // crypto/rand never fails on supported platforms
	}
	return strings.ToLower(b32.EncodeToString(b[:]))
}

func validLocator(s string) bool {
	return s != "" && len(s) <= MaxLocatorBytes && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// Translate maps a draft's raw context to an aliased Context for profile p. The tool
// profile admits only an invocation identity; any inherited session, agent, parent,
// attempt, role or work input is refused rather than silently widened or dropped.
// Unknown role input becomes the explicit unavailable value.
func Translate(p Profile, raw *RawContext, m AliasMapper) (*Context, error) {
	if raw == nil {
		return nil, nil
	}
	if p == ProfileTool {
		if raw.ParentInvocation != "" || raw.Session != "" || raw.Agent != "" || raw.ParentAgent != "" ||
			raw.Attempt != "" || raw.SharedRefresh != "" || raw.DeskRole != "" || raw.Role != "" || len(raw.Work) > 0 {
			return nil, refuse(RefusePrivacy, "ctx", "tool profile cannot carry workflow identity")
		}
	}
	c := &Context{}
	for _, f := range []struct {
		kind string
		in   string
		out  *string
	}{
		{"invocation", raw.Invocation, &c.Invocation},
		{"invocation", raw.ParentInvocation, &c.ParentInvocation},
		{"session", raw.Session, &c.Session},
		{"agent", raw.Agent, &c.Agent},
		{"agent", raw.ParentAgent, &c.ParentAgent},
		{"attempt", raw.Attempt, &c.Attempt},
		{"refresh", raw.SharedRefresh, &c.SharedRefresh},
	} {
		if f.in == "" {
			continue
		}
		if !validLocator(f.in) {
			return nil, refuse(RefuseBounds, "ctx", "locator is empty, too long or not text")
		}
		a, err := m.Alias(f.kind + "\x00" + f.in)
		if err != nil {
			return nil, refuse(RefuseSchema, "ctx", "alias mapping unavailable")
		}
		*f.out = a
	}
	if p == ProfileWorkflow {
		c.DeskRole = ParseDeskRole(raw.DeskRole)
		c.Role = ParseRole(raw.Role)
	}
	if len(raw.Work) > MaxWorkRefs {
		return nil, refuse(RefuseBounds, "ctx.work", "too many work references")
	}
	for _, w := range raw.Work {
		key, kind, ok := workKey(w)
		if !ok {
			return nil, refuse(RefuseSchema, "ctx.work", "work locator is not a typed brief or issue reference")
		}
		a, err := m.Alias(key)
		if err != nil {
			return nil, refuse(RefuseSchema, "ctx.work", "alias mapping unavailable")
		}
		c.Work = append(c.Work, WorkRef{Kind: kind, Ref: a, Primary: w.Primary})
	}
	return c, nil
}

func workKey(w RawWork) (string, WorkKind, bool) {
	switch w.Kind {
	case string(WorkBrief):
		if !validLocator(w.Repo) || !validLocator(w.Brief) || (w.BriefUUID != "" && !validLocator(w.BriefUUID)) {
			return "", "", false
		}
		return "brief\x00" + w.Repo + "\x00" + w.Brief + "\x00" + w.BriefUUID, WorkBrief, true
	case string(WorkIssue):
		if !validLocator(w.Instance) || !validLocator(w.Repo) || w.Number <= 0 {
			return "", "", false
		}
		return "issue\x00" + w.Instance + "\x00" + w.Repo + "\x00" + strconv.Itoa(w.Number), WorkIssue, true
	}
	return "", "", false
}

// Identity is the writer-owned identity assigned to one sealed event.
type Identity struct {
	Source string
	Epoch  uint32
	Seq    uint64
}

// Seal turns a draft into a validated Event and its canonical bytes. The payload is
// strictly decoded by kind and re-encoded canonically; an unset occurredAt takes
// recordedAt.
func Seal(d Draft, id Identity, p Profile, policy string, build Build, ctx *Context, recordedAt time.Time) (Event, []byte, error) {
	typed, err := DecodePayload(d.Kind, d.Payload)
	if err != nil {
		return Event{}, nil, err
	}
	payload, err := json.Marshal(typed)
	if err != nil {
		return Event{}, nil, refuse(RefuseSchema, "payload", "unencodable")
	}
	rec := FormatTime(recordedAt)
	occ := d.OccurredAt
	if occ == "" {
		occ = rec
	}
	e := Event{
		Schema: EventSchema, ID: EventID(id.Source, id.Epoch, id.Seq),
		Source: id.Source, Epoch: id.Epoch, Seq: id.Seq,
		Profile: p, Policy: policy,
		Instrument: Instrument{Name: InstrumentName, Version: InstrumentVersion},
		Build:      build, OccurredAt: occ, RecordedAt: rec,
		Kind: d.Kind, Ctx: ctx, Payload: payload,
	}
	b, err := e.Encode()
	if err != nil {
		return Event{}, nil, err
	}
	return e, b, nil
}
