// Package forgeread is the importable fact reader: the narrow, read-only shape of a forge fact
// (today: the open issues of an explicit repository set) and the three-state envelope that carries
// it between the desk read verb and a consumer.
//
// THE ROOT PACKAGE IS THE OFFLINE AND FROZEN SURFACE. It holds types, the wire envelope with its
// fail-closed parser, an offline reader and a frozen-result reader, and nothing else. It imports
// no subpackage, no net/http and no os/exec, so a consumer that links only this package cannot
// start a request or a process, and cannot be handed a credential. Every authenticated read lives
// in a subpackage under adapters/, takes only an already-admitted Access from its trusted
// composition, and is linked by that composition alone (the desk-tools read verb). A consumer such
// as statusgen links this package and nothing below it; its online reads stay on the read verb.
//
// THREE STATES, NEVER TWO. Every requested repository appears in exactly one of two places in a
// Result: Issues (read, possibly known-empty) or Unavailable (could not be read, with a reason and
// a class). A repository that was not looked at can never be mistaken for one that was looked at
// and had nothing.
//
// TWO CONTRACTS, VERSIONED SEPARATELY. APIVersion is the Go surface (this package's exported
// identifiers); EnvelopeSchema is the JSON wire shape. Either can move without the other.
package forgeread

import (
	"context"
	"sort"
	"time"
)

const (
	// APIVersion is the version of this package's Go contract. Additive changes bump the minor
	// part; a removed or re-typed identifier bumps the major part.
	APIVersion = "1.0.0"

	// EnvelopeSchema is the JSON wire contract of the issues envelope. A document declaring any
	// other value is refused (ErrUnsupportedSchema), never best-effort parsed.
	EnvelopeSchema = 1
)

// Issue is one open issue, reduced to the fields every provider can serve. The JSON tags are the
// wire shape of the read verb's `issues` kind.
type Issue struct {
	Number      int      `json:"number"`
	Title       string   `json:"title,omitempty"`
	State       string   `json:"state"`
	AuthorLogin string   `json:"authorLogin,omitempty"`
	AuthorID    int64    `json:"authorId,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	// UpdatedAt is part of the Go API only (json:"-"): the in-process desk-tools consumers read
	// it, and the wire contract of the read verb does not carry it.
	UpdatedAt  string `json:"-"`
	URL        string `json:"url,omitempty"`
	ClosedAt   string `json:"closedAt,omitempty"`
	AuthorType string `json:"authorType,omitempty"`
}

// Created parses CreatedAt as RFC 3339. A stamp that does not parse yields the zero time, which a
// caller reads as "no credible age", never as the epoch.
func (i Issue) Created() time.Time {
	t, err := time.Parse(time.RFC3339, i.CreatedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Class says why a repository is unavailable. The classes are distinct states a consumer may act
// on differently; none of them is an empty answer.
type Class string

const (
	// ClassUnreadable: the read failed or was refused. This is also the meaning of an absent class
	// on the wire.
	ClassUnreadable Class = "unreadable"
	// ClassTruncated: the provider kept paginating past the walk ceiling, so any list in hand
	// would be a PARTIAL set; none is returned.
	ClassTruncated Class = "truncated"
	// ClassOffline: no read was attempted (the offline reader).
	ClassOffline Class = "offline"
	// ClassMissing: the producer answered without a result or a reason for this repository.
	ClassMissing Class = "missing"
)

// Unavailable is one repository that could not be read, and why.
type Unavailable struct {
	Repo   string `json:"repo"`
	Reason string `json:"reason"`
	Class  Class  `json:"class,omitempty"`
}

// Result is the three-state answer for a repository set. Issues holds each repository that WAS
// read; a repository with no open issues maps to a non-nil empty slice (known-empty). Unavailable
// holds each repository that was not read. The two sets are disjoint and together cover the
// requested set exactly once.
type Result struct {
	Issues      map[string][]Issue
	Unavailable []Unavailable
}

// NewResult returns an empty Result whose Issues map is non-nil.
func NewResult() Result { return Result{Issues: map[string][]Issue{}} }

// Reader reads the open issues of an explicit repository set.
//
// err is for a fault in the reader itself (a malformed or unsupported document). A repository the
// provider would not serve is never an err: it is an Unavailable entry. Every requested repository
// appears in exactly one of the two result sets, once.
type Reader interface {
	OpenIssues(ctx context.Context, repos []string) (Result, error)
}

// Normalize returns repos with empty entries and duplicates removed, order preserved. It is the
// request set every reader reconciles against.
func Normalize(repos []string) []string {
	seen := make(map[string]bool, len(repos))
	out := make([]string, 0, len(repos))
	for _, r := range repos {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// Reconcile enforces the exactly-once invariant against the requested set. A repository present
// in both sets is treated as unavailable (the conservative reading); a repository present in
// neither becomes unavailable with ClassMissing; an entry for a repository nobody asked for is
// dropped; a repeated unavailable entry keeps the first. The returned Result is a new value.
func Reconcile(requested []string, in Result, missingReason string) Result {
	want := Normalize(requested)
	asked := make(map[string]bool, len(want))
	for _, r := range want {
		asked[r] = true
	}
	unavail := map[string]Unavailable{}
	for _, u := range in.Unavailable {
		if !asked[u.Repo] {
			continue
		}
		if _, dup := unavail[u.Repo]; dup {
			continue
		}
		if u.Class == "" {
			u.Class = ClassUnreadable
		}
		unavail[u.Repo] = u
	}
	out := NewResult()
	for _, r := range want {
		if u, bad := unavail[r]; bad {
			out.Unavailable = append(out.Unavailable, u)
			continue
		}
		if is, ok := in.Issues[r]; ok {
			if is == nil {
				is = []Issue{}
			}
			out.Issues[r] = is
			continue
		}
		out.Unavailable = append(out.Unavailable, Unavailable{Repo: r, Reason: missingReason, Class: ClassMissing})
	}
	return out
}

// SortedRepos returns the repositories of a Result's Issues in sorted order.
func (r Result) SortedRepos() []string {
	out := make([]string, 0, len(r.Issues))
	for k := range r.Issues {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
