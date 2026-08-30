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
// the M1 taxonomy and churn, and writes the aggregate metrics into the derived
// metrics table (reset-then-rewritten, never accumulated). It is READ-ONLY over
// the raw tables and writes ONLY the metrics table under the tracking root.
func aggregateM1(store *Store, cfg M1Config) error {
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
	var repoTax CommitTaxonomy
	repoTax.LineClasses = map[LineClass]int{}
	perPkg := map[string]*CommitTaxonomy{}
	for _, com := range commits {
		ct := classifyCommit(com.SHA, diffsByCommit[com.SHA], cfg.BlockMin)
		mergeTaxonomy(&repoTax, ct)
		// Attribute this commit's blocks/lines to the packages its files touch.
		for _, pkg := range commitPackages(diffsByCommit[com.SHA]) {
			pt := perPkg[pkg]
			if pt == nil {
				pt = &CommitTaxonomy{LineClasses: map[LineClass]int{}}
				perPkg[pkg] = pt
			}
			mergeTaxonomy(pt, ct)
		}
	}

	// --- Churn roll-up. ---
	classOf := func(c Commit) string { return cfg.Identity.Classify(c) }
	churn := computeChurn(commits, diffsByCommit, classOf, time.Duration(cfg.ChurnWindowDays)*24*time.Hour)

	// --- Emit. Reset first so the derived table is a coherent snapshot. ---
	if err := store.ResetMetrics(); err != nil {
		return err
	}
	var records []MetricRecord
	emit := func(metric, grain, key string, v Measure[float64]) {
		records = append(records, MetricRecord{
			Metric: metric, Grain: grain, Key: key, Value: v,
			Basis: basisPublishedDefinitions, Note: honestClaimsNote,
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

// commitPackages returns the distinct package (directory) paths a commit's file
// diffs touch — the per-package aggregation grain (spec §4). The repo root is
// the "." package.
func commitPackages(diffs []FileDiff) []string {
	seen := map[string]bool{}
	var out []string
	for _, fd := range diffs {
		p := fd.NewPath
		if p == "" {
			p = fd.OldPath
		}
		if p == "" {
			continue
		}
		pkg := path.Dir(p)
		if !seen[pkg] {
			seen[pkg] = true
			out = append(out, pkg)
		}
	}
	return out
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
