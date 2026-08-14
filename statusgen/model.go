package main

import "time"

type Stream struct {
	Name string
	Dir  string // absolute path to the stream directory
	// Root is the --root this stream was discovered under.
	// In a single-root run every stream carries the same value; in a multi-root run
	// it is what routes a stream to its own STATUS.md and its own registers.
	Root string
	// Repo is the optional `repo:` frontmatter field — the owning repo in
	// <owner>/<name> form, "" when absent. A stream lives in exactly ONE repo:
	// statusgen validates the form, requires every stream under one root to agree,
	// hard-errors when two roots claim the same repo, and surfaces the value in
	// STATUS.md and `--gate-scores` output.
	Repo     string
	Status   string // active | paused | done
	Priority string // P0 | P1 | P2
	Track    string // product | platform | ecosystem | ""
	Issues   []int
	External string
	// Tiering: free-text dispatcher guidance (e.g. "implement=sonnet verify=fable"),
	// nil when the frontmatter field is absent, non-nil (incl. "") when present.
	// Never enforced, never a Next-up score input (F-09 scope note).
	Tiering *string
	// MaxConcurrent: per-stream cap overrides the global perStreamCap. Optional;
	// nil when absent (uses perStreamCap). When present, 1 <= value <= perStreamCap
	// — it restricts the cap, never widens it.
	MaxConcurrent *int
	// Serves: the business goal this stream serves — example-app | example-service |
	// assay | platform | "" (untagged, roadmap deck).
	Serves string
	// Owner: optional stream owner, from the README frontmatter; "" when absent.
	Owner  string
	Briefs []Brief
	// Placeholders are the issue-loop placeholder rows (schema: placeholder-v1)
	// parsed from this stream's issue-<NN>.md files. Each is also appended to
	// Briefs as a synthetic row so the whole Next-up pipeline treats it as a
	// first-class candidate; the separate slice is retained for
	// placeholder-specific machinery — claim-awareness keyed on the issue number.
	// nil when the stream has no placeholders.
	Placeholders []Placeholder
	LastTouch    time.Time
}

type Brief struct {
	Num      string // "01", "12a"
	Title    string
	Wave     int
	Effort   string
	Status   string // todo | in-progress | implemented | verified | done | blocked
	Verified string // "" = unset
	Reviewed string // "" = unset
	// StaleRef is the finding ID when THIS brief is named by an unresolved
	// finding's affects: entry (`<stream>/<NN>` or `<stream>/brief-<NN>`). It is
	// both a display marker (`⚠ <id>` in the Incomplete-briefs listing) and a
	// hard Next-up exclusion, so only a brief-SPECIFIC affects entry may set it:
	// a bare-stream entry (`affects: <stream>`) is a stream-level annotation and
	// must never be broadcast to every brief in the stream — it would
	// silently lock the whole stream out of dispatch. Stream-level findings are
	// surfaced by their own machinery instead (Unresolved-findings table, the
	// roadmap top-blocker cell and health rules, DORA findings-per-group).
	StaleRef string
	Depends  []string // typed deps from brief-v1 frontmatter ("<stream>/<NN>"); nil for legacy
	Schema   string   // "brief-v1" from frontmatter; "" for legacy (non-brief-v1)
	// Value is the optional brief-v1 `value:` field — low | med | high, "" when
	// absent (treated as med). A Next-up score input:
	// the explicit worth of a brief, separate from priority and staleness.
	Value string
	// Blocked is the optional await/unblock state for placeholder-v1 briefs.
	// "" = not blocked; "awaiting-issue-response" = worker
	// has a blocking question posted on the GitHub issue. statusgen excludes
	// blocked placeholders from Next-up; the scanner (brief 02, wired in 04)
	// clears the field when a non-bot comment answers the question.
	Blocked string
	// ExecTier is the optional brief-v1 `exec-tier:` field — "any" or
	// "strong"; "" when absent (treated as "any"). A Next-up marker, NEVER a
	// score input (F-09 scope note). Wired from the brief
	// file's frontmatter into the README row by checkBriefFiles, same pattern
	// as Value/Depends/Schema.
	ExecTier string
	// Gate is the brief-v1 `gate:` field — "model" or "human"; "" for legacy
	// (no brief-v1 frontmatter). Wired from the brief file's frontmatter into
	// the README row by checkBriefFiles (needed at
	// render time for Awaiting-board segmentation by blocker owner).
	Gate string
	// BlockedBy is the optional brief-v1 `blocked-by:` field — "env" when the
	// brief is blocked on infrastructure/environment; "" otherwise or for
	// legacy (marks the env-blocked segment in the
	// segmented Awaiting board).
	BlockedBy string
	// Measures is the optional brief-v1 `measures:` field — the name of the
	// process queue this brief instruments (a metric, alarm or report ABOUT that
	// queue). nil when the field is absent, which is the neutral default: an
	// ordinary brief is not an instrumentation brief and this field never
	// touches it. Present (incl. "") means the drain-before-instrument gate
	// applies — see eligible()/queueBlocks in nextup.go. An eligibility
	// exclusion only: NEVER a Next-up score input (F-09 scope note).
	Measures *string
	// Evidence is the body of the `## Evidence` section from the brief file;
	// "" when absent or legacy. Wired from BriefFile into the README row by
	// checkBriefFiles (read at render time to
	// classify VERIFY:PASS / VERIFY:FAIL for Awaiting-board segmentation).
	Evidence string
}

type Finding struct {
	ID       string // "F-01" (legacy numeric) or "F-ws-token-expiry" (slug)
	Date     string // "2026-07-08"
	Title    string
	Affects  []string // "stream" or "stream/brief-01"
	Ack      string   // "" = unacked; else "YYYY-MM-DD <who>" (desk acknowledgement, F-09)
	Resolved bool
}
