package main

import (
	"path"
	"sort"
	"time"
)

// M1 aggregation (spec §4). Rolls the per-commit line-operation taxonomy and the
// cross-commit churn computation up to the spec's aggregation grains and emits
// the headline comparability metrics — copy/paste ratio and duplicate-block rate
// (§4.1) and churn / rework rate (§4.2) — as three-state Measures into the
// derived metrics table, each labelled per the honest-claims discipline (§10).

// Honest-claims labels (spec §10): numbers are "computed per GitClear's
// PUBLISHED definitions," never "GitClear-equivalent." basisPublishedDefinitions
// is the machine token an artifact consumer keys on; it is emitted on every M1
// metric so no downstream reader can mistake a published-methodology number for
// an equivalence claim.
const (
	basisPublishedDefinitions = "published definitions"
	honestClaimsNote          = "computed per GitClear's published definitions; not a GitClear-equivalence claim"
)

// Metric-name constants — the on-disk contract a downstream reader keys on.
const (
	MetricCopyPasteRatio     = "copy_paste_ratio"
	MetricDuplicateBlockRate = "duplicate_block_rate"
	MetricChurnRate          = "churn_rate"
	MetricNewLines           = "new_lines"
	MetricChurnedLines       = "churned_lines"
)

// Aggregation grains (spec §4). PR and stream grains require issue/PR linkage a
// bare-git mine does not have; they are supplied by later adapter-gated briefs,
// not silently emitted as an unlabeled zero here.
const (
	GrainRepo     = "repo"
	GrainPackage  = "package"
	GrainIdentity = "identity"
	GrainWindow   = "window"
)

// MetricRecord is one aggregated metric value at one grain. One MetricRecord
// marshals to one line in metrics.jsonl. Value is a three-state Measure so a
// could-not-measure grain is never rounded to a misleading zero.
type MetricRecord struct {
	Metric string `json:"metric"`
	Grain  string `json:"grain"`
	// Key identifies the grain instance (a package path, an identity class, a
	// window label); empty for the repo grain.
	Key string `json:"key,omitempty"`

	Value Measure[float64] `json:"value"`

	// Basis is the honest-claims comparability token (spec §10) — the machine
	// key; "published definitions" means computed per the source tool's
	// published methodology, never an equivalence claim.
	Basis string `json:"basis"`
	// Note is the human-readable honest-claims label.
	Note string `json:"note"`

	// MinedAt stamps the run that produced this snapshot. The metrics table is
	// append-only, so each mine appends a fresh full M1 snapshot; MinedAt is the
	// ordering key that lets a trend consumer select the LATEST snapshot per
	// (metric, grain, key) rather than the table-position first-match. It is the
	// same stamp the sibling hotspot / ownership / coupling families carry on the
	// same table (their `mined_at`), so the whole metrics table orders uniformly.
	MinedAt time.Time `json:"mined_at"`
}

// M1Config parameterizes the aggregation: the block-match threshold (§4.1), the
// churn window (§4.2), the window bucket size for the per-window grain, and the
// author-identity partition (§3.1).
type M1Config struct {
	BlockMin        int
	ChurnWindowDays int
	// WindowDays buckets the per-window churn grain (calendar bucketing of
	// landing dates). 0 disables the per-window grain.
	WindowDays int
	Identity   *IdentityMap
	// Instruction configures the instruction-layer brittleness pass (spec §4.6,
	// quality/04). Its zero value (empty Globs) is UNCONFIGURED — the pass emits
	// a could-not-measure marker rather than a silent zero (fact 1), so a mine
	// with no instruction-doc glob set is honest about not having looked.
	Instruction InstructionBrittleConfig
}

// DefaultM1Config is the comparable-defaults configuration: GitClear's published
// block granularity and churn window, monthly windows, and an empty identity map
// (every author unclassified until a target supplies a map).
func DefaultM1Config() M1Config {
	return M1Config{
		BlockMin:        DefaultBlockMin,
		ChurnWindowDays: DefaultChurnWindowDays,
		WindowDays:      30,
		Identity:        &IdentityMap{},
	}
}

// aggregateM1 reads the mined commit + diff tables through the Store, computes
// the M1 taxonomy and churn, and appends the aggregate metrics as one fresh
// full snapshot to the append-only metrics table (extend, never rewrite — the
// same way the sibling hotspot / ownership / coupling families append theirs).
// minedAt stamps every record in this snapshot so a trend consumer can pick the
// latest snapshot per metric. It is READ-ONLY over the raw tables and writes
// ONLY the metrics table under the tracking root.
func aggregateM1(store *Store, cfg M1Config, minedAt time.Time) error {
	if cfg.BlockMin < 1 {
		cfg.BlockMin = DefaultBlockMin
	}
	if cfg.ChurnWindowDays < 1 {
		cfg.ChurnWindowDays = DefaultChurnWindowDays
	}
	if cfg.Identity == nil {
		cfg.Identity = &IdentityMap{}
	}

	commits, err := store.ReadCommits()
	if err != nil {
		return err
	}
	diffs, err := store.ReadDiffs()
	if err != nil {
		return err
	}
	diffsByCommit := map[string][]FileDiff{}
	for _, fd := range diffs {
		diffsByCommit[fd.CommitSHA] = append(diffsByCommit[fd.CommitSHA], fd)
	}

	// --- Taxonomy roll-up: repo total + per package. ---
	//
	// classifyCommitByFile attributes each commit's classified lines and detected
	// blocks to the FILE that carries them, so the per-package grain credits a
	// copied/moved block only to the package that gained its added lines — a
	// commit spanning packages A and B never inflates B with A's blocks. The repo
	// total is the sum over every file's taxonomy, identical to the whole-commit
	// classification.
	var repoTax CommitTaxonomy
	repoTax.LineClasses = map[LineClass]int{}
	perPkg := map[string]*CommitTaxonomy{}
	for _, com := range commits {
		for filePath, ft := range classifyCommitByFile(com.SHA, diffsByCommit[com.SHA], cfg.BlockMin) {
			mergeTaxonomy(&repoTax, *ft)
			pkg := path.Dir(filePath)
			pt := perPkg[pkg]
			if pt == nil {
				pt = &CommitTaxonomy{LineClasses: map[LineClass]int{}}
				perPkg[pkg] = pt
			}
			mergeTaxonomy(pt, *ft)
		}
	}

	// --- Churn roll-up. ---
	classOf := func(c Commit) string { return cfg.Identity.Classify(c) }
	churn := computeChurn(commits, diffsByCommit, classOf, time.Duration(cfg.ChurnWindowDays)*24*time.Hour)

	// --- Emit. The metrics table is append-only: each mine appends a fresh full
	// snapshot of the M1 family (extend, never rewrite), alongside the hotspot /
	// ownership / coupling snapshots quality/03 appends in the same run. A trend
	// consumer (quality/05) reads the most recent snapshot per metric. ---
	var records []MetricRecord
	emit := func(metric, grain, key string, v Measure[float64]) {
		records = append(records, MetricRecord{
			Metric: metric, Grain: grain, Key: key, Value: v,
			Basis: basisPublishedDefinitions, Note: honestClaimsNote,
			MinedAt: minedAt,
		})
	}

	// Headline copy/paste ratio + duplicate-block rate at the repo grain.
	emit(MetricCopyPasteRatio, GrainRepo, "", copyPasteRatio(repoTax))
	emit(MetricDuplicateBlockRate, GrainRepo, "", duplicateBlockRate(repoTax))

	// Per-package copy/paste ratio (deterministic order for a diffable artifact).
	for _, pkg := range sortedKeys(perPkg) {
		emit(MetricCopyPasteRatio, GrainPackage, pkg, copyPasteRatio(*perPkg[pkg]))
	}

	// Churn / rework rate: repo total + per author-identity class.
	emit(MetricChurnRate, GrainRepo, "", churn.Overall.Rate())
	emit(MetricNewLines, GrainRepo, "", intMeasure(churn.Overall.NewLines))
	emit(MetricChurnedLines, GrainRepo, "", intMeasure(churn.Overall.ChurnedLines))
	for _, class := range sortedChurnClasses(churn.ByClass) {
		emit(MetricChurnRate, GrainIdentity, class, churn.ByClass[class].Rate())
	}

	for _, r := range records {
		if err := store.Append(KindMetric, r); err != nil {
			return err
		}
	}
	return nil
}

// copyPasteRatio is the headline comparability metric (spec §4.1):
// copied / (moved + copied), over BLOCKS. A commit history with no move/copy
// blocks is measured-zero (the instrument ran, no duplication observed), never a
// could-not-measure or a misleading absent value.
func copyPasteRatio(ct CommitTaxonomy) Measure[float64] {
	denom := ct.MovedBlocks + ct.CopiedBlocks
	if denom == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(ct.CopiedBlocks) / float64(denom))
}

// duplicateBlockRate is the share of classified added lines that fall in a
// duplicated (copied) block (spec §4.1's duplicate-block rate). Measured-zero
// when there were added lines but none duplicated; could-not-measure when there
// were no classified added lines at all.
func duplicateBlockRate(ct CommitTaxonomy) Measure[float64] {
	added := ct.LineClasses[ClassAdded] + ct.LineClasses[ClassUpdated] +
		ct.LineClasses[ClassMoved] + ct.LineClasses[ClassCopied]
	if added == 0 {
		return CouldNotMeasure[float64]("no classified added lines: duplicate-block rate is undefined")
	}
	if ct.LineClasses[ClassCopied] == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(ct.LineClasses[ClassCopied]) / float64(added))
}

// intMeasure wraps a non-negative count as a measured value (a genuine zero is
// measured-zero, distinct from could-not-measure).
func intMeasure(n int) Measure[float64] {
	if n == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(n))
}

// mergeTaxonomy accumulates src into dst.
func mergeTaxonomy(dst *CommitTaxonomy, src CommitTaxonomy) {
	if dst.LineClasses == nil {
		dst.LineClasses = map[LineClass]int{}
	}
	for k, v := range src.LineClasses {
		dst.LineClasses[k] += v
	}
	dst.MovedBlocks += src.MovedBlocks
	dst.CopiedBlocks += src.CopiedBlocks
	dst.CouldNotMeasureLines += src.CouldNotMeasureLines
}

func sortedKeys(m map[string]*CommitTaxonomy) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedChurnClasses(m map[string]*ChurnCounts) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
