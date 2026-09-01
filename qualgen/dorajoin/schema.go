package dorajoin

import "time"

// schema.go — the DORA join's output field naming (spec §8). Field names
// FOLLOW Apache DevLake's domain-layer schema
// (https://devlake.apache.org/docs/DataModels/DevLakeDomainLayerSchema) where
// a concept matches — "commits", "pull_requests", "incidents" — PURELY so a
// downstream reader comparing this join's numbers to a DevLake-backed
// dashboard needs no translation table. This is naming only: no DevLake code,
// schema import, or runtime dependency is introduced anywhere in this
// package or module; the join runs standalone against its own
// DeliveryMetricsSource (source.go) and the caller-supplied M1/M2 inputs
// (denominator.go, cfr.go).

// Record is one emitted row of the DORA join: one grain (window, and
// optionally stream / identity-class) carrying the quality denominator, the
// incident-vs-traced CFR pair, and the join-key resolution it came from.
type Record struct {
	Window        string `json:"window"`
	Stream        string `json:"stream,omitempty"`
	IdentityClass string `json:"identity_class,omitempty"`

	// DurableChangeVolume is the quality denominator (denominator.go):
	// landed_lines - churn_14d - copied.
	DurableChangeVolume Measure[float64] `json:"durable_change_volume"`

	// CFR carries the incident-based and traced change-failure rates side by
	// side (cfr.go), the traced number never emitted without its trace-rate
	// and tier composition.
	CFR CFRRecord `json:"cfr"`

	// PullRequests / Commits / Incidents follow DevLake's domain-layer naming
	// (package doc, above) for comparability: the window's join-key PR count,
	// its distinct merge-SHA (commit) count, and the delivery source's
	// incident count respectively — three-state, since a window with no
	// resolvable delivery records for one of these is could-not-measure, not
	// a silent zero.
	PullRequests Measure[float64] `json:"pull_requests"`
	Commits      Measure[float64] `json:"commits"`
	Incidents    Measure[float64] `json:"incidents"`

	// JoinState summarizes this row's key resolution (joinkeys.go):
	// matched / no-match / could-not-join.
	JoinState MatchState `json:"join_state"`

	MinedAt time.Time `json:"mined_at"`
}
