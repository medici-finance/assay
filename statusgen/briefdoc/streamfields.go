package briefdoc

import "gopkg.in/yaml.v3"

// StreamFields is the existing stream-index YAML shape, shared by discovery
// and register classification. It carries no effect or authorization policy.
type StreamFields struct {
	Stream        string  `yaml:"stream"`
	Status        string  `yaml:"status"`
	Priority      string  `yaml:"priority"`
	Track         string  `yaml:"track"`
	Issues        []int   `yaml:"issues"`
	External      string  `yaml:"external"`
	Tiering       *string `yaml:"tiering"`        // optional; nil when absent, non-nil (incl. "") when present.
	MaxConcurrent *int    `yaml:"max-concurrent"` // optional; nil when absent; 1..perStreamCap when present.
	Serves        string  `yaml:"serves"`         // optional; example-app | example-service | assay | platform | "" (untagged).
	Theme         string  `yaml:"theme"`          // optional render-style selector for cadenced artifacts; unmapped values render as a visible marker.
	Owner         string  `yaml:"owner"`          // optional stream owner; "" when absent — renders "—".
	Repo          string  `yaml:"repo"`           // optional owning repo, <owner>/<name>; "" when absent.
	Board         string  `yaml:"board"`          // optional; "generated" opts the Briefs table into the marker-wrapped generated region (derived-board/04).
	Traced        *bool   `yaml:"traced"`         // optional; true opts the stream INTO the untraced-brief traceability check (registers-v1 §6.5). nil/false = out (the default): the check never fires over a corpus that has not opted in.
	Spec          string  `yaml:"spec"`           // optional; the repo-relative scoping doc this stream was scaffolded FROM (attention-budget/04). An active stream must cite one whose §8.1 header is `**Status:** approved` (the `stream-source` lint); a parked stream may cite a draft.
	// Mission is the optional authored mission block, kept as a raw node so a
	// malformed block never fails the README parse (the board must keep
	// working); parseMissionBlock validates it and returns diagnostics.
	// Kind == 0 when the key is absent.
	Mission yaml.Node `yaml:"mission"`
}
