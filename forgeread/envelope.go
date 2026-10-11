package forgeread

import (
	"encoding/json"
	"errors"
	"fmt"
)

// IssuesKind is the envelope kind of an open-issues read.
const IssuesKind = "issues"

// ErrUnsupportedSchema is returned (wrapped) for an envelope that declares a schema other than
// EnvelopeSchema. It is a fault in the document, not an unavailable repository.
var ErrUnsupportedSchema = errors.New("forgeread: unsupported envelope schema")

// RepoIssues is one repository's issues in the envelope. Its Issues member is always present on the
// wire, so a known-empty repository is distinguishable from an absent one.
type RepoIssues struct {
	Repo   string  `json:"repo"`
	Issues []Issue `json:"issues"`
}

// IssuesEnvelope is the JSON document of an open-issues read. A producer may add members; this
// parser ignores what it does not know, but never accepts a different Schema.
type IssuesEnvelope struct {
	Schema  int           `json:"schema"`
	Kind    string        `json:"kind"`
	Repos   []RepoIssues  `json:"repos"`
	Partial []Unavailable `json:"partial"`
}

// ParseIssuesEnvelope parses data and reconciles it against the requested repositories. An unknown
// schema or a different kind is refused; a malformed document is an error. Every requested
// repository comes back exactly once, as data or as unavailable.
func ParseIssuesEnvelope(data []byte, requested []string) (Result, error) {
	var env IssuesEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Result{}, fmt.Errorf("forgeread: parsing the issues envelope: %w", err)
	}
	if env.Schema != EnvelopeSchema {
		return Result{}, fmt.Errorf("%w: the envelope declares schema %d, this build understands %d",
			ErrUnsupportedSchema, env.Schema, EnvelopeSchema)
	}
	if env.Kind != "" && env.Kind != IssuesKind {
		return Result{}, fmt.Errorf("forgeread: the envelope is kind %q, not %q", env.Kind, IssuesKind)
	}
	in := NewResult()
	for _, r := range env.Repos {
		if _, dup := in.Issues[r.Repo]; dup {
			// A repeated repository entry cannot be both answers; refuse to guess which is right.
			in.Unavailable = append(in.Unavailable, Unavailable{Repo: r.Repo,
				Reason: "the envelope carries this repository more than once", Class: ClassUnreadable})
			continue
		}
		is := r.Issues
		if is == nil {
			is = []Issue{}
		}
		in.Issues[r.Repo] = is
	}
	in.Unavailable = append(in.Unavailable, env.Partial...)
	return Reconcile(requested, in, "the producer returned no result and no reason for this repository"), nil
}

// NewIssuesEnvelope renders a Result as an envelope in the order of requested.
func NewIssuesEnvelope(requested []string, res Result) IssuesEnvelope {
	rec := Reconcile(requested, res, "no result and no reason for this repository")
	env := IssuesEnvelope{Schema: EnvelopeSchema, Kind: IssuesKind, Repos: []RepoIssues{}, Partial: []Unavailable{}}
	for _, r := range Normalize(requested) {
		if is, ok := rec.Issues[r]; ok {
			env.Repos = append(env.Repos, RepoIssues{Repo: r, Issues: is})
		}
	}
	env.Partial = append(env.Partial, rec.Unavailable...)
	return env
}
