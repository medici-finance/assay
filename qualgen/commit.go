package main

import "time"

// Commit is the internal commit record — the commit table (spec §4 aggregation
// grains all derive from this). One Commit marshals to one line in commits.jsonl.
//
// This is part of the frozen interface contract (contract item 2): later briefs
// READ Commit records via the Store and must not redefine this shape.
//
// Author identity is kept RAW here (the un-parsed "Name <email>" line plus the
// split name/email as git recorded them). The human / agent / automation
// classification (spec §4.2, §4.4) is a LATER brief's job; this skeleton only
// records the ground truth so that classification has something to classify.
type Commit struct {
	SHA        string   `json:"sha"`
	ParentSHAs []string `json:"parent_shas"`

	// Author is who wrote the change; Committer is who applied it. Both are
	// recorded raw — later briefs classify identity, they do not re-mine it.
	AuthorRaw   string    `json:"author_raw"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	AuthorWhen  time.Time `json:"author_when"`

	CommitterRaw   string    `json:"committer_raw"`
	CommitterName  string    `json:"committer_name"`
	CommitterEmail string    `json:"committer_email"`
	CommitterWhen  time.Time `json:"committer_when"`

	Message string `json:"message"`

	// FileDiffKeys references the per-(commit,file) diff records in diffs.jsonl
	// (FileDiff.Key). The diff bodies live in the diff table, not inline, so the
	// commit table stays a compact spine an aggregator can stream cheaply.
	FileDiffKeys []string `json:"file_diff_keys"`
}

// IsMerge reports whether this commit has more than one parent. Squash/merge
// granularity is load-bearing for later SZZ tracing (spec §5.2); the skeleton
// records the parent set so that reasoning is possible, without acting on it.
func (c Commit) IsMerge() bool {
	return len(c.ParentSHAs) > 1
}

// IsRoot reports whether this commit has no parents (the initial commit). A root
// commit is diffed against the empty tree during mining.
func (c Commit) IsRoot() bool {
	return len(c.ParentSHAs) == 0
}
