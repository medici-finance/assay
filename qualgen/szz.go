package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	fdiff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// szz.go — the B-SZZ inducing-commit trace engine and the derived defect metrics
// (spec §5.2, §5.3). For each identified DefectFix (quality/06), it blames the
// fix's deleted/modified lines at the fix's PARENT, applies the standard
// refinements (refine.go), and emits a DefectTrace record; from the trace corpus it
// computes the defect-inducing change rate, per-file defect density, fix latency,
// and the traced-CFR input. It is READ-ONLY over the mined repository and writes
// only diffable JSONL under the operator-chosen tracking root.
//
// Honest-claims discipline (spec §10): NO derived number is emitted without its
// trace-rate and evidence-tier composition beside it — a bare rate is a bug, not a
// formatting choice. The three-state invariant (spec §3.2) governs every outcome: a
// blame that failed, a squash-collapsed pre-image, an omission bug — each is a
// distinct could-not-trace, never a silent zero.

// TraceState is the three-state trace outcome for one fix (spec §3.2). Every fix
// resolves to exactly one.
type TraceState string

const (
	// TraceTraced — ≥1 inducing commit survived refinement.
	TraceTraced TraceState = "traced"
	// TraceTracedNone — blame ran and every candidate was filtered out; a real,
	// measured zero, distinct from could-not-trace.
	TraceTracedNone TraceState = "traced-none"
	// TraceCouldNotTrace — blame was unreachable (blameless/omission, multi-hop
	// history, squash-merge/history floor, or a blame error). Never a silent zero.
	TraceCouldNotTrace TraceState = "could-not-trace"
)

// CouldNotTraceReason enumerates why a could-not-trace fix was unreachable
// (frozen: consumed by downstream briefs). Empty for a traced / traced-none fix.
type CouldNotTraceReason string

const (
	// ReasonBlameless — an omission bug: the fix only adds lines, so there is no
	// deleted/modified pre-image line to blame.
	ReasonBlameless CouldNotTraceReason = "blameless"
	// ReasonMultiHop — the blame landed on a merge/squash commit; the true inducing
	// change is one hop deeper than a single blame can attribute.
	ReasonMultiHop CouldNotTraceReason = "multi-hop"
	// ReasonSquashFloor — the fix's pre-image state is unreachable on the mined
	// history (a squash merge collapsed the inducing commits away, or a shallow /
	// grafted history floor cut them off).
	ReasonSquashFloor CouldNotTraceReason = "squash-floor"
	// ReasonBlameError — blame itself failed (an unreadable blob, a diff error).
	ReasonBlameError CouldNotTraceReason = "blame-error"
)

// DefectTrace is the per-fix B-SZZ trace record — one per DefectFix, written to
// defects.jsonl (spec §9.4: "M2 traces with tier + confidence"). Its field names
// are the FROZEN interface contract consumed by briefs 08/10/11/14/15; changing
// them later is a breaking change.
type DefectTrace struct {
	// FixCommit / FixPR carry the fix identity through from the consumed DefectFix.
	FixCommit string `json:"fix_commit"`
	FixPR     int    `json:"fix_pr,omitempty"`

	// InducingCommits / InducingPRs are the resolved inducers; empty (never null)
	// when traced-none or could-not-trace. InducingPRs stays empty for a bare-git
	// mine that has no PR metadata to resolve a commit to its PR — an empty list is
	// honest, an invented PR is not.
	InducingCommits []string `json:"inducing_commits"`
	InducingPRs     []string `json:"inducing_prs"`

	// EvidenceTier is carried through from quality/06 (1/2/3), NEVER re-derived here.
	EvidenceTier EvidenceTier `json:"evidence_tier,omitempty"`

	// Confidence is this brief's blame-agreement score (refine.go). It is a recorded
	// field for consumers to weight, never a gate. A could-not-trace carries a
	// could-not-measure confidence with the reason; a traced-none carries a
	// measured-zero.
	Confidence Measure[float64] `json:"confidence"`

	// TraceState is the three-state outcome; CouldNotTraceReason names why when the
	// state is could-not-trace (empty otherwise).
	TraceState          TraceState          `json:"trace_state"`
	CouldNotTraceReason CouldNotTraceReason `json:"could_not_trace_reason,omitempty"`
}

// ReportDateResolver resolves the defect-report date for a fix — the date the bug
// was reported, against which the postdating refinement (spec §5.2) excludes
// inducers introduced AFTER the bug already existed.
//
// It is a pluggable seam, exactly as quality/06's LinkageAdapter is: the report
// date lives in an issue tracker, not in git, so resolving it is an adapter's job
// and never hardcoded here. A fix with no resolvable report date (no closed issue,
// or the tracker unreachable) returns ok=false, and the postdating refinement is
// then INAPPLICABLE for that fix — recorded as such (the candidate is not dropped
// on a date the run could not read), never silently applied as if the date were
// zero. A nil resolver disables the refinement entirely (a bare-git run with no
// tracker access); confidence and trace-state still travel honestly.
type ReportDateResolver interface {
	ReportDate(fix DefectFix) (t time.Time, ok bool)
}

// TraceCorpus is the result of tracing a fix set: the per-fix DefectTrace records
// plus the aggregates the derived metrics (spec §5.3) read. could-not-trace fixes
// are recorded in Traces (with their reason) but EXCLUDED from the per-file and
// inducing aggregates — they are unreachable, never counted as a zero-inducer fix.
type TraceCorpus struct {
	Traces []DefectTrace

	// perFileInducers: file → set of distinct inducing commit SHAs (defect density).
	perFileInducers map[string]map[string]struct{}
	// perFileFixes: file → set of TRACEABLE fix commits (traced or traced-none) that
	// touched the file's blameable surface — the per-file trace-rate denominator.
	perFileFixes map[string]map[string]struct{}
	// perFileTraced: file → set of fix commits that resolved `traced` touching it —
	// the per-file trace-rate numerator.
	perFileTraced map[string]map[string]struct{}

	// inducingDate: inducing commit SHA → its author date (fix latency).
	inducingDate map[string]time.Time
	// fixDate: fix commit SHA → its author date (fix latency).
	fixDate map[string]time.Time
	// inducingByFix: fix commit SHA → set of its inducing commits (latency pairing).
	inducingByFix map[string]map[string]struct{}

	tiers  TierComposition
	total  int // total identified fixes traced
	traced int // count that resolved `traced`
}

// TraceDefects runs the B-SZZ trace over an identified fix set against repo,
// returning the trace corpus. Only fixes that quality/06 IDENTIFIED (Identified ==
// measured) are traced; a could-not-identify DefectFix has no confirmed fix to
// trace and never enters the corpus. repo is opened read-only by the caller.
func TraceDefects(repo *git.Repository, fixes []DefectFix, resolver ReportDateResolver) TraceCorpus {
	c := TraceCorpus{
		perFileInducers: map[string]map[string]struct{}{},
		perFileFixes:    map[string]map[string]struct{}{},
		perFileTraced:   map[string]map[string]struct{}{},
		inducingDate:    map[string]time.Time{},
		fixDate:         map[string]time.Time{},
		inducingByFix:   map[string]map[string]struct{}{},
	}
	var identified []DefectFix
	for _, fix := range fixes {
		if fix.Identified.State != StateMeasured {
			continue
		}
		identified = append(identified, fix)

		tr, detail := traceFix(repo, fix, resolver)
		c.Traces = append(c.Traces, tr)
		c.total++
		if tr.TraceState == TraceTraced {
			c.traced++
		}

		if fc, err := repo.CommitObject(plumbing.NewHash(fix.FixCommitSHA)); err == nil {
			c.fixDate[fix.FixCommitSHA] = fc.Author.When.UTC()
		}
		for _, f := range detail.touchedFiles {
			addToSet(c.perFileFixes, f, tr.FixCommit)
			if tr.TraceState == TraceTraced {
				addToSet(c.perFileTraced, f, tr.FixCommit)
			}
		}
		for f, inds := range detail.perFileInducers {
			for _, ind := range inds {
				addToSet(c.perFileInducers, f, ind)
			}
		}
		for ind, d := range detail.inducingDates {
			c.inducingDate[ind] = d
		}
		for _, ind := range tr.InducingCommits {
			addToSet(c.inducingByFix, fix.FixCommitSHA, ind)
		}
	}
	c.tiers = ComputeTierComposition(identified)
	return c
}

// fixTraceDetail carries the per-fix attribution the corpus aggregates: which files
// the fix's inducers were blamed in, and each inducer's date.
type fixTraceDetail struct {
	perFileInducers map[string][]string
	inducingDates   map[string]time.Time
	// touchedFiles is the set of files this fix touched on its blameable surface —
	// populated ONLY for a traceable fix (traced / traced-none); a could-not-trace
	// fix touches nothing for aggregation purposes (it is excluded from rates).
	touchedFiles []string
}

// traceFix traces one identified fix: blame its deleted/modified lines at the
// parent, refine, and assign the three-state outcome.
func traceFix(repo *git.Repository, fix DefectFix, resolver ReportDateResolver) (DefectTrace, fixTraceDetail) {
	tr := DefectTrace{
		FixCommit:       fix.FixCommitSHA,
		FixPR:           fix.FixPRNumber,
		EvidenceTier:    fix.Tier,
		InducingCommits: []string{},
		InducingPRs:     []string{},
	}
	detail := fixTraceDetail{
		perFileInducers: map[string][]string{},
		inducingDates:   map[string]time.Time{},
	}

	get := func(h string) (*object.Commit, error) { return repo.CommitObject(plumbing.NewHash(h)) }

	fixCommit, err := repo.CommitObject(plumbing.NewHash(fix.FixCommitSHA))
	if err != nil {
		return couldNotTrace(tr, ReasonBlameError, fmt.Sprintf("resolve fix commit: %v", err)), detail
	}
	if fixCommit.NumParents() == 0 {
		return couldNotTrace(tr, ReasonBlameless, "fix is a root commit: no pre-existing line to blame"), detail
	}
	parent, err := fixCommit.Parent(0)
	if err != nil {
		// The pre-image the fix modified is not reachable on the mined history — a
		// squash merge collapsed the inducing commits, or a shallow/grafted floor
		// cut the parent off. Recorded, never a silent zero.
		return couldNotTrace(tr, ReasonSquashFloor, fmt.Sprintf("fix parent unreachable on the mined history: %v", err)), detail
	}
	changed, err := changedOldLines(parent, fixCommit)
	if err != nil {
		return couldNotTrace(tr, ReasonBlameError, fmt.Sprintf("diff fix against parent: %v", err)), detail
	}
	totalOld := 0
	for _, ls := range changed {
		totalOld += len(ls)
	}
	if totalOld == 0 {
		// Pure addition: an omission bug with no deleted/modified line to blame.
		return couldNotTrace(tr, ReasonBlameless, "fix only adds lines: an omission with no deleted/modified line to blame"), detail
	}

	var report time.Time
	haveReport := false
	if resolver != nil {
		if t, ok := resolver.ReportDate(fix); ok {
			report, haveReport = t.UTC(), true
		}
	}

	inducerSet := map[string]struct{}{}
	candidateCount := 0
	sawCandidate := false
	sawBlameError := false
	sawUnattributable := false

	for _, file := range sortedFileKeys(changed) {
		blamed, err := blameLines(parent, file, changed[file])
		if err != nil {
			sawBlameError = true
			continue
		}
		for _, n := range sortedBlameLineNums(blamed) {
			li := blamed[n]
			candidateCount++
			sawCandidate = true

			inducer, reason := resolveInducer(get, file, li)
			switch reason {
			case reasonInducerMultiHop, reasonInducerUnreachable:
				sawUnattributable = true
				continue
			case reasonInducerBlameError:
				sawBlameError = true
				continue
			}
			if postdatesReport(inducer.Date, report, haveReport) {
				continue // filtered: the inducer postdates the defect report
			}
			inducerSet[inducer.Commit] = struct{}{}
			detail.perFileInducers[file] = appendUniqueStr(detail.perFileInducers[file], inducer.Commit)
			detail.inducingDates[inducer.Commit] = inducer.Date
		}
	}

	survivors := sortedStrSet(inducerSet)
	if len(survivors) >= 1 {
		tr.TraceState = TraceTraced
		tr.InducingCommits = survivors
		tr.Confidence = scoreConfidence(candidateCount, len(survivors))
		detail.touchedFiles = sortedMapKeysStrSlice(detail.perFileInducers)
		return tr, detail
	}

	// No survivors. Distinguish a real measured-zero (blame ran, every candidate
	// filtered) from could-not-trace.
	if sawCandidate && !sawBlameError && !sawUnattributable {
		tr.TraceState = TraceTracedNone
		tr.Confidence = MeasuredZero[float64]()
		detail.touchedFiles = sortedFileKeys(changed) // traceable: blame ran on these files
		return tr, detail
	}
	if sawBlameError {
		return couldNotTrace(tr, ReasonBlameError, "blame failed for the fix's changed lines"), detail
	}
	if sawUnattributable {
		return couldNotTrace(tr, ReasonMultiHop, "inducing history is multi-hop (merge/squash) — not attributable in one blame hop"), detail
	}
	return couldNotTrace(tr, ReasonBlameless, "no blameable inducing line resolved"), detail
}

// couldNotTrace stamps a trace as could-not-trace with a reason and a
// could-not-measure confidence carrying that reason (spec §3.2) — the confidence is
// never rounded to zero.
func couldNotTrace(tr DefectTrace, reason CouldNotTraceReason, detail string) DefectTrace {
	tr.TraceState = TraceCouldNotTrace
	tr.CouldNotTraceReason = reason
	tr.InducingCommits = []string{}
	tr.InducingPRs = []string{}
	tr.Confidence = CouldNotMeasure[float64](fmt.Sprintf("%s: %s", reason, detail))
	return tr
}

// changedOldLines returns, per file, the 1-based OLD-side line numbers the fix
// deleted or modified — the lines to blame at the parent. It diffs the fix against
// its parent tree (first-parent, matching the miner's convention). A file the fix
// only ADDED contributes nothing (there is no old-side line); a binary blob
// contributes nothing (no text to blame).
func changedOldLines(parent, fix *object.Commit) (map[string][]int, error) {
	parentTree, err := parent.Tree()
	if err != nil {
		return nil, fmt.Errorf("parent tree: %w", err)
	}
	fixTree, err := fix.Tree()
	if err != nil {
		return nil, fmt.Errorf("fix tree: %w", err)
	}
	changes, err := object.DiffTree(parentTree, fixTree)
	if err != nil {
		return nil, fmt.Errorf("diff trees: %w", err)
	}
	out := map[string][]int{}
	for _, ch := range changes {
		patch, err := ch.Patch()
		if err != nil {
			return nil, fmt.Errorf("patch: %w", err)
		}
		for _, fp := range patch.FilePatches() {
			from, _ := fp.Files()
			if from == nil {
				continue // added file: no old-side pre-image to blame
			}
			if fp.IsBinary() {
				continue // binary: no text lines to blame
			}
			dels := oldSideDeletedLines(fp)
			if len(dels) > 0 {
				out[from.Path()] = append(out[from.Path()], dels...)
			}
		}
	}
	return out, nil
}

// oldSideDeletedLines walks a file patch's chunks tracking the OLD-side line cursor
// and returns the 1-based line numbers of the deleted lines. A modified line is a
// delete + add pair in go-git's chunk model, so its old-side line number surfaces
// here as a delete — exactly the pre-image line to blame.
func oldSideDeletedLines(fp fdiff.FilePatch) []int {
	oldLine := 0
	var dels []int
	for _, ch := range fp.Chunks() {
		segs := splitChunkLines(ch.Content())
		switch ch.Type() {
		case fdiff.Equal:
			oldLine += len(segs)
		case fdiff.Delete:
			for range segs {
				oldLine++
				dels = append(dels, oldLine)
			}
		case fdiff.Add:
			// Added lines advance only the new-side cursor; no old-side line.
		}
	}
	return dels
}

// splitChunkLines splits a chunk's content into individual lines, dropping the
// trailing empty segment a content string ending in "\n" produces (mirrors
// diff.go's chunksToLines).
func splitChunkLines(content string) []string {
	segs := strings.Split(content, "\n")
	if len(segs) > 0 && segs[len(segs)-1] == "" {
		segs = segs[:len(segs)-1]
	}
	return segs
}

// -------- derived metrics (spec §5.3) --------

// Metric-name constants for the defect-metrics family on the metrics table — the
// on-disk contract downstream readers key on. brief 08 reads MetricDefectDensity.
const (
	MetricDefectTraceRate    = "defect_trace_rate"
	MetricDefectInducingRate = "defect_inducing_rate"
	MetricDefectDensity      = "defect_density"
	MetricFixLatency         = "fix_latency"
	MetricTracedCFRInput     = "traced_cfr_input"
)

// Honest-claims labels for the SZZ family (spec §10): every derived number ships
// its trace-rate and evidence-tier composition; a bare number is never emitted.
const (
	basisTraceDisclosed = "trace-rate + evidence-tier disclosed"
	szzHonestNote       = "B-SZZ with standard refinements; trace-rate and evidence-tier composition travel beside every number (spec §10) — never a bare rate"
)

// DefectMetricRecord is one derived defect-metric value on the metrics table. Like
// the sibling M1 families it carries a "metric" discriminator and a mined-at stamp;
// UNLIKE them it ALSO carries TraceRate and TierComposition beside the value — the
// honest-claims contract (spec §10) that a defect number is never published bare.
type DefectMetricRecord struct {
	Metric string           `json:"metric"`
	Grain  string           `json:"grain,omitempty"`
	Key    string           `json:"key,omitempty"`
	Value  Measure[float64] `json:"value"`

	// TraceRate and TierComposition travel beside EVERY value (honest-claims).
	TraceRate       Measure[float64] `json:"trace_rate"`
	TierComposition TierComposition  `json:"tier_composition"`

	Basis   string    `json:"basis"`
	Note    string    `json:"note"`
	MinedAt time.Time `json:"mined_at"`
}

// DefectDensity is the per-file rollup (frozen contract, read by brief 08 as
// `defect_density`): the inducing-commit count for a file, WITH the file's
// trace-rate beside it — a density computed over a low trace-rate is a floor, not a
// count, so the rate travels with it.
type DefectDensity struct {
	File            string           `json:"file"`
	InducingCommits int              `json:"inducing_commits"`
	DefectDensity   float64          `json:"defect_density"`
	TraceRate       Measure[float64] `json:"trace_rate"`
	TierComposition TierComposition  `json:"tier_composition"`
}

// TraceRate is the run's overall trace-rate: traced / total identified fixes (spec
// §4/§5.2). could-not-trace fixes ARE in the denominator — publishing the rate low
// when history is unreachable is the whole point (~40% is unreachable by blame).
func (c TraceCorpus) TraceRate() Measure[float64] {
	if c.total == 0 {
		return CouldNotMeasure[float64]("no identified fixes to trace")
	}
	if c.traced == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(c.traced) / float64(c.total))
}

// Partition returns the three-state split of the trace corpus.
func (c TraceCorpus) Partition() (traced, tracedNone, couldNotTrace int) {
	for _, tr := range c.Traces {
		switch tr.TraceState {
		case TraceTraced:
			traced++
		case TraceTracedNone:
			tracedNone++
		case TraceCouldNotTrace:
			couldNotTrace++
		}
	}
	return
}

// DistinctInducingCommits is the numerator of the defect-inducing rate: the count
// of distinct inducing commits across all traced fixes. could-not-trace fixes
// contribute none (they never entered perFileInducers), so they are EXCLUDED from
// the numerator — never counted as a zero.
func (c TraceCorpus) DistinctInducingCommits() int {
	set := map[string]struct{}{}
	for _, inds := range c.perFileInducers {
		for ind := range inds {
			set[ind] = struct{}{}
		}
	}
	return len(set)
}

// DefectInducingRate is the defect-inducing change rate (spec §5.3):
// distinct-inducing-changes / merged-PRs over the window. mergedPRs is supplied by
// the caller (a bare-git mine has no PR metadata); a non-positive denominator makes
// the rate could-not-measure rather than an invented number. could-not-trace fixes
// are excluded from the numerator (DistinctInducingCommits) — they do not dilute
// the rate as a false zero.
func (c TraceCorpus) DefectInducingRate(mergedPRs int) Measure[float64] {
	if mergedPRs <= 0 {
		return CouldNotMeasure[float64]("merged-PR denominator not supplied: defect-inducing rate is undefined for a bare-git mine")
	}
	inducing := c.DistinctInducingCommits()
	if inducing == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(inducing) / float64(mergedPRs))
}

// TracedCFRInput is the traced defect-inducing rate that brief 11's DORA join
// re-bases change-failure-rate on (spec §5.3, §8): distinct traced-defect-inducing
// changes / merged-PRs. Same shape as DefectInducingRate but named for its DORA
// consumer, so the join reads an explicitly traced input, not a heuristic one.
func (c TraceCorpus) TracedCFRInput(mergedPRs int) Measure[float64] {
	return c.DefectInducingRate(mergedPRs)
}

// FixLatency is the mean inducing-merge → fix-merge time in DAYS across traced
// fixes (spec §5.3). Each traced fix's latency is measured from its EARLIEST
// inducer's date to the fix's date. A corpus with no datable traced fix is
// could-not-measure, never a zero.
func (c TraceCorpus) FixLatency() Measure[float64] {
	var sumDays float64
	n := 0
	for fixSHA, inds := range c.inducingByFix {
		fd, ok := c.fixDate[fixSHA]
		if !ok {
			continue
		}
		var earliest time.Time
		for ind := range inds {
			id, ok := c.inducingDate[ind]
			if !ok {
				continue
			}
			if earliest.IsZero() || id.Before(earliest) {
				earliest = id
			}
		}
		if earliest.IsZero() {
			continue
		}
		sumDays += fd.Sub(earliest).Hours() / 24.0
		n++
	}
	if n == 0 {
		return CouldNotMeasure[float64]("no traced fix with a datable inducer: fix latency undefined")
	}
	return Measured(sumDays / float64(n))
}

// fileTraceRate is a single file's trace-rate: traced fixes touching it / all
// traceable fixes touching it. could-not-trace fixes never entered perFileFixes, so
// the denominator excludes them.
func (c TraceCorpus) fileTraceRate(file string) Measure[float64] {
	total := len(c.perFileFixes[file])
	if total == 0 {
		return CouldNotMeasure[float64]("no traceable fix touched this file")
	}
	traced := len(c.perFileTraced[file])
	if traced == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(traced) / float64(total))
}

// DefectDensities is the per-file defect-density rollup (spec §5.3), one row per
// file that carried at least one resolved inducer, each with its own trace-rate.
func (c TraceCorpus) DefectDensities() []DefectDensity {
	var out []DefectDensity
	for _, file := range sortedSetKeys(c.perFileInducers) {
		n := len(c.perFileInducers[file])
		out = append(out, DefectDensity{
			File:            file,
			InducingCommits: n,
			DefectDensity:   float64(n),
			TraceRate:       c.fileTraceRate(file),
			TierComposition: c.tiers,
		})
	}
	return out
}

// DerivedMetrics renders the whole derived-metrics family as metrics-table records,
// EVERY one carrying its trace-rate and tier-composition (honest-claims, spec §10).
// mergedPRs is the window's merged-PR denominator for the rate metrics; minedAt
// stamps the snapshot.
func (c TraceCorpus) DerivedMetrics(mergedPRs int, minedAt time.Time) []DefectMetricRecord {
	overall := c.TraceRate()
	mk := func(metric, grain, key string, v Measure[float64], rate Measure[float64]) DefectMetricRecord {
		return DefectMetricRecord{
			Metric: metric, Grain: grain, Key: key, Value: v,
			TraceRate: rate, TierComposition: c.tiers,
			Basis: basisTraceDisclosed, Note: szzHonestNote, MinedAt: minedAt,
		}
	}

	recs := []DefectMetricRecord{
		mk(MetricDefectTraceRate, GrainRepo, "", overall, overall),
		mk(MetricDefectInducingRate, GrainWindow, "", c.DefectInducingRate(mergedPRs), overall),
		mk(MetricFixLatency, GrainRepo, "", c.FixLatency(), overall),
		mk(MetricTracedCFRInput, GrainWindow, "", c.TracedCFRInput(mergedPRs), overall),
	}
	for _, d := range c.DefectDensities() {
		recs = append(recs, mk(MetricDefectDensity, GrainPackage, d.File, intCountMeasure(d.InducingCommits), d.TraceRate))
	}
	return recs
}

// intCountMeasure wraps a non-negative count as a measured value (a genuine zero is
// measured-zero — distinct from could-not-measure).
func intCountMeasure(n int) Measure[float64] {
	if n == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(n))
}

// WriteTo emits the corpus: one DefectTrace per fix to defects.jsonl and the derived
// metrics to metrics.jsonl, both append-only under the tracking root (task items
// 4/6). It never writes the mined repo and never a single-writer view.
func (c TraceCorpus) WriteTo(store *Store, mergedPRs int, minedAt time.Time) error {
	for _, tr := range c.Traces {
		if err := store.Append(KindDefect, tr); err != nil {
			return fmt.Errorf("append defect trace: %w", err)
		}
	}
	for _, rec := range c.DerivedMetrics(mergedPRs, minedAt) {
		if err := store.Append(KindMetric, rec); err != nil {
			return fmt.Errorf("append defect metric: %w", err)
		}
	}
	return nil
}

// -------- small deterministic helpers --------

func addToSet(m map[string]map[string]struct{}, k, v string) {
	if m[k] == nil {
		m[k] = map[string]struct{}{}
	}
	m[k][v] = struct{}{}
}

func appendUniqueStr(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func sortedFileKeys(m map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedBlameLineNums(m map[int]LineInducer) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func sortedStrSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSetKeys(m map[string]map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedMapKeysStrSlice(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
