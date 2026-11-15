package perfdata

// snapshot.go — the per-source coverage manifest, the daily distribution rollup and the
// frozen three-report input snapshot. These are contract types: the local store fills
// coverage and rollups, the report package reads them, and future readers (on-demand and
// daily reports, a database adapter) consume the same shapes.

import "sort"

// Coverage states of one source.
const (
	CoverageMissing       = "missing"        // no store: unknown, never zero
	CoverageEmptyComplete = "empty-complete" // store present and clean, no events in window
	CoverageComplete      = "complete"       // events present, no unexplained loss
	CoveragePartial       = "partial"        // events present with gaps, drops, partial tails or unclean ends
	CoverageCorrupt       = "corrupt"        // undecodable records or metadata
)

// SeqRange is an inclusive range of sequence numbers with a fixed reason token.
type SeqRange struct {
	From   uint64 `json:"from"`
	To     uint64 `json:"to"`
	Reason string `json:"reason"`
}

// Gap reasons.
const (
	GapRetiredAge   = "retired-age"
	GapRetiredBytes = "retired-bytes"
	GapWriteFailed  = "write-failed"
	GapUnexplained  = "unexplained"
)

// Retirement records data removed by policy. Unsent is true for every retirement in
// this module: no uploader exists, so every retired event was never sent.
type Retirement struct {
	Reason   string `json:"reason"`
	FromSeq  uint64 `json:"fromSeq"`
	ToSeq    uint64 `json:"toSeq"`
	Events   int    `json:"events"`
	Bytes    int64  `json:"bytes"`
	FromDay  string `json:"fromDay,omitempty"`
	ToDay    string `json:"toDay,omitempty"`
	Unsent   bool   `json:"unsent"`
	RolledUp bool   `json:"rolledUp"`
	At       string `json:"at"`
}

// Drops counts observations lost before or during writing, as far as they could be
// recorded. A loss that also lost its marker remains unknown.
type Drops struct {
	QueueFull    int64 `json:"queueFull"`
	WriteFailed  int64 `json:"writeFailed"`
	FlushTimeout int64 `json:"flushTimeout"`
	Refused      int64 `json:"refused"`
}

// Add accumulates d2 into d.
func (d *Drops) Add(d2 Drops) {
	d.QueueFull += d2.QueueFull
	d.WriteFailed += d2.WriteFailed
	d.FlushTimeout += d2.FlushTimeout
	d.Refused += d2.Refused
}

// EpochReset records why a source started a new epoch.
type EpochReset struct {
	Epoch  uint32 `json:"epoch"`
	Reason string `json:"reason"`
	At     string `json:"at"`
}

// Epoch reset reasons.
const (
	ResetSourceLost   = "source-metadata-lost"
	ResetAliasMapLost = "alias-map-lost"
	ResetSeqBehind    = "source-metadata-behind"
)

// SourceCoverage is the coverage manifest of one source as a reader found it.
type SourceCoverage struct {
	Source              string       `json:"source,omitempty"`
	State               string       `json:"state"`
	Epochs              []uint32     `json:"epochs"`
	Events              int          `json:"events"`
	FirstRecordedAt     string       `json:"firstRecordedAt,omitempty"`
	LastRecordedAt      string       `json:"lastRecordedAt,omitempty"`
	LastSeq             uint64       `json:"lastSeq"`
	CorruptLines        int          `json:"corruptLines"`
	CorruptMetadata     int          `json:"corruptMetadata"`
	PartialTails        int          `json:"partialTails"`
	ExplainedGaps       []SeqRange   `json:"explainedGaps"`
	UnexplainedGaps     []SeqRange   `json:"unexplainedGaps"`
	Retired             []Retirement `json:"retired"`
	Drops               Drops        `json:"drops"`
	UncleanTerminations int          `json:"uncleanTerminations"`
	LiveWriters         int          `json:"liveWriters"`
	EpochResets         []EpochReset `json:"epochResets"`
}

// RollupSampleCap bounds the exact samples one rollup distribution keeps.
const RollupSampleCap = 4096

// RollupRow is one day's pooled tool-completion distributions for one key.
type RollupRow struct {
	Tool        string       `json:"tool"`
	Verb        string       `json:"verb,omitempty"`
	DeskRole    DeskRole     `json:"deskRole,omitempty"`
	Version     string       `json:"version"`
	OS          string       `json:"os"`
	Arch        string       `json:"arch"`
	Child       bool         `json:"child,omitempty"`
	Runs        int64        `json:"runs"`
	WallUnknown int64        `json:"wallUnknown"`
	CPUUnknown  int64        `json:"cpuUnknown"`
	Wall        Distribution `json:"wall"`
	CPUOwn      Distribution `json:"cpuOwn"`
}

func (r RollupRow) key() string {
	c := "0"
	if r.Child {
		c = "1"
	}
	return r.Tool + "\x00" + r.Verb + "\x00" + string(r.DeskRole) + "\x00" + r.Version + "\x00" + r.OS + "\x00" + r.Arch + "\x00" + c
}

// DailyRollup is one UTC day of tool-completion distributions, the retained form of
// raw events after their age cap when distribution retention is configured.
type DailyRollup struct {
	Schema string      `json:"schema"`
	Day    string      `json:"day"`
	Rows   []RollupRow `json:"rows"`
}

// NewRollup returns an empty rollup for day (YYYY-MM-DD).
func NewRollup(day string) DailyRollup {
	return DailyRollup{Schema: RollupSchema, Day: day, Rows: []RollupRow{}}
}

// DayOf returns the UTC day of a canonical timestamp.
func DayOf(ts string) string {
	if len(ts) < 10 {
		return ""
	}
	return ts[:10]
}

// AddEvent folds one tool-completion event into the rollup; other kinds are ignored.
// The caller deduplicates and applies corrections first.
func (r *DailyRollup) AddEvent(e Event, tc ToolCompletion) {
	row := RollupRow{Tool: tc.Tool, Verb: tc.Verb, Version: e.Build.Version, OS: e.Build.OS, Arch: e.Build.Arch,
		Child: e.Ctx != nil && e.Ctx.ParentInvocation != ""}
	if e.Ctx != nil {
		row.DeskRole = e.Ctx.DeskRole
	}
	row.Runs = 1
	row.Wall, row.CPUOwn = NewDistribution(), NewDistribution()
	if tc.Wall.Known() {
		row.Wall.Add(*tc.Wall.Ms)
	} else {
		row.WallUnknown = 1
	}
	if tc.CPU.Own.Known() {
		row.CPUOwn.Add(*tc.CPU.Own.Ms)
	} else {
		row.CPUUnknown = 1
	}
	r.addRow(row)
}

func (r *DailyRollup) addRow(row RollupRow) {
	k := row.key()
	for i := range r.Rows {
		if r.Rows[i].key() == k {
			cur := &r.Rows[i]
			cur.Runs += row.Runs
			cur.WallUnknown += row.WallUnknown
			cur.CPUUnknown += row.CPUUnknown
			cur.Wall, _ = Merge(cur.Wall, row.Wall, RollupSampleCap)
			cur.CPUOwn, _ = Merge(cur.CPUOwn, row.CPUOwn, RollupSampleCap)
			return
		}
	}
	r.Rows = append(r.Rows, row)
	sort.Slice(r.Rows, func(i, j int) bool { return r.Rows[i].key() < r.Rows[j].key() })
}

// MergeRollup folds o (same day) into r.
func (r *DailyRollup) MergeRollup(o DailyRollup) error {
	if o.Schema != RollupSchema || o.Day != r.Day {
		return refuse(RefuseUnsupported, "rollup", "rollup schema or day differs")
	}
	for _, row := range o.Rows {
		if err := row.Wall.ValidateStored(RollupSampleCap); err != nil {
			return err
		}
		if err := row.CPUOwn.ValidateStored(RollupSampleCap); err != nil {
			return err
		}
		r.addRow(row)
	}
	return nil
}

// DecodeRollup strictly decodes and validates a stored rollup.
func DecodeRollup(b []byte) (DailyRollup, error) {
	var r DailyRollup
	if err := strictUnmarshal(b, &r); err != nil {
		return DailyRollup{}, err
	}
	if _, ok := ParseTime(r.Day + "T00:00:00.000Z"); !ok || r.Schema != RollupSchema {
		return DailyRollup{}, refuse(RefuseSchema, "rollup", "not a performance-rollup-v1 day")
	}
	out := NewRollup(r.Day)
	if err := out.MergeRollup(r); err != nil {
		return DailyRollup{}, err
	}
	return out, nil
}

// Report identities of the three operating reports.
const (
	ReportToolPerformance = "tool-performance"
	ReportCompletedBriefs = "completed-briefs"
	ReportStreamProgress  = "stream-progress"
)

// ReportInput states whether this snapshot supplies one operating report's input.
type ReportInput struct {
	Report string `json:"report"`
	State  string `json:"state"` // available | unavailable
	Source string `json:"source,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Window is an inclusive UTC day window.
type Window struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// Snapshot is the frozen input of the three operating reports: the cutoff, the policy
// and instrument versions, the per-source coverage manifest and which reports it can
// feed. Rerunning a report over the same snapshot gives equivalent output.
type Snapshot struct {
	Schema         string           `json:"schema"`
	Cutoff         string           `json:"cutoff"`
	Timezone       string           `json:"timezone"`
	Window         Window           `json:"window"`
	PolicyRevision string           `json:"policyRevision,omitempty"`
	Instrument     Instrument       `json:"instrument"`
	Sources        []SourceCoverage `json:"sources"`
	Reports        []ReportInput    `json:"reports"`
}

// SnapshotReports returns the fixed report-input table of a local spool snapshot: tool
// performance is fed by this spool; completed-briefs and stream-progress are owned by
// lifecycle/flow sources this collector does not observe, so they are unavailable here
// rather than zero.
func SnapshotReports() []ReportInput {
	return []ReportInput{
		{Report: ReportToolPerformance, State: "available", Source: "performance-events"},
		{Report: ReportCompletedBriefs, State: "unavailable", Reason: "lifecycle-source-not-collected"},
		{Report: ReportStreamProgress, State: "unavailable", Reason: "flow-source-not-collected"},
	}
}
