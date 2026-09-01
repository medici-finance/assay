package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// pr.go is the `pr <n>` mode (spec §9.1): the per-PR risk-feature feed. For
// the file set a PR touches, it joins each file to the shared FileFeatures
// assembly (features.go, authored by brief 09; this mode reuses it rather
// than forking) and emits a per-file feature record as generic JSON — this
// mode assigns no weight of its own and computes no combined outcome; every
// consumer decides that in its own configuration (spec §9.1). It is
// READ-ONLY against both the tracking root (--out) and the target repo
// (--repo) and writes NO artifact of its own: the feed goes to stdout for a
// caller to capture.
//
// Touched-file-set resolution (Context: "the same repo/history access the
// miner uses") stays OFFLINE — neither path below makes a network call, both
// read only commits/refs already present in the local repository:
//   - default: search the mined repo's own history for the two-parent merge
//     commit a non-squash GitHub merge writes for PR #n
//     ("Merge pull request #n from ...") and diff it against its own first
//     parent — the same first-parent diff mine.go's extractFileDiffs already
//     uses, which for an ordinary (non-conflicted) merge is exactly the PR's
//     full changed-file set.
//   - --head/--base: an open PR's caller (e.g. a CI job that already
//     resolved the PR's own head/base from its own checkout) supplies them
//     directly rather than this mode guessing a pull-ref naming convention.
var mergedPRPattern = regexp.MustCompile(`(?im)^Merge pull request #(\d+) from `)

// PRFeatureFeed is the `pr <n>` mode's JSON output (spec §9.1): a generic
// per-touched-file feature feed.
type PRFeatureFeed struct {
	PR    int             `json:"pr"`
	Files []PRFileFeature `json:"files"`
}

// PRFileFeature is one touched file's feature record. Every numeric feature
// is a pointer, present only when its own Measure state is measured or
// measured-zero — a could-not-measure feature is never fabricated as a 0
// (Context: "never a silent 0"). Measured carries the three-state string per
// feature (task 3), sourced from the underlying M1/M2 record's own state.
type PRFileFeature struct {
	Path              string            `json:"path"`
	HotspotPercentile *float64          `json:"hotspot_percentile,omitempty"`
	DefectDensity     *float64          `json:"defect_density,omitempty"`
	DefectTraceRate   *float64          `json:"defect_trace_rate,omitempty"`
	OwnershipTop      *float64          `json:"ownership_top,omitempty"`
	CouplingMissing   []string          `json:"coupling_missing"`
	Measured          map[string]string `json:"measured"`
}

// runPR is the `pr <n>` mode entry point.
func runPR(args []string, stdout, stderr io.Writer) int {
	// The PR number is a leading positional before any flags (`pr 1 --out x`).
	var prArg string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		prArg = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("pr", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "tracking root to read mined artifacts from (required)")
	repo := fs.String("repo", ".", "path to the git repository the PR belongs to (read-only)")
	head := fs.String("head", "", "an open PR's head ref/commit — used together with --base, overrides the merged-PR history search")
	base := fs.String("base", "", "an open PR's base ref/commit — used together with --head")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" {
		fmt.Fprintln(stderr, "qualgen pr: --out <dir> is required (the tracking root to read mined artifacts from)")
		return 2
	}
	if prArg == "" {
		fmt.Fprintln(stderr, "qualgen pr: a PR number is required (qualgen pr <n> --out <dir>)")
		return 2
	}
	prNum, convErr := strconv.Atoi(prArg)
	if convErr != nil {
		fmt.Fprintf(stderr, "qualgen pr: %q is not a valid PR number\n", prArg)
		return 2
	}
	if (*head == "") != (*base == "") {
		fmt.Fprintln(stderr, "qualgen pr: --head and --base must be given together")
		return 2
	}

	r, err := git.PlainOpenWithOptions(*repo, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		fmt.Fprintln(stderr, "qualgen pr: open repository:", err)
		return 1
	}

	var paths []string
	if *head != "" {
		paths, err = touchedFilesHeadBase(r, *head, *base)
	} else {
		paths, err = touchedFilesMergedPR(r, prNum)
	}
	if err != nil {
		fmt.Fprintln(stderr, "qualgen pr:", err)
		return 1
	}

	store := NewStore(*out)
	feed, err := buildPRFeed(store, prNum, paths)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen pr:", err)
		return 1
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(feed); err != nil {
		fmt.Fprintln(stderr, "qualgen pr: encode feed:", err)
		return 1
	}
	return 0
}

// touchedFilesMergedPR searches repo history reachable from HEAD for the
// two-parent merge commit a non-squash GitHub merge writes for PR #prNum,
// then returns the file set that merge introduced: the diff between the
// merge commit and its own first parent (the base branch tip immediately
// before the merge landed).
func touchedFilesMergedPR(r *git.Repository, prNum int) ([]string, error) {
	head, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	iter, err := r.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("walk log: %w", err)
	}
	var merge *object.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		if c.NumParents() < 2 {
			return nil
		}
		m := mergedPRPattern.FindStringSubmatch(c.Message)
		if m == nil {
			return nil
		}
		n, convErr := strconv.Atoi(m[1])
		if convErr == nil && n == prNum {
			merge = c
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search merge history for PR #%d: %w", prNum, err)
	}
	if merge == nil {
		return nil, fmt.Errorf("no merged PR #%d found in this repository's history reachable from HEAD (no \"Merge pull request #%d from\" commit) — pass --head/--base for an open PR", prNum, prNum)
	}
	parent, err := merge.Parent(0)
	if err != nil {
		return nil, fmt.Errorf("resolve merge commit's first parent: %w", err)
	}
	return diffPaths(parent, merge)
}

// touchedFilesHeadBase resolves head/base (branch names, tags, or raw SHAs)
// and returns the file set changed between them — an open PR's own
// head-vs-base diff, supplied by the caller rather than guessed from a
// pull-ref naming convention.
func touchedFilesHeadBase(r *git.Repository, head, base string) ([]string, error) {
	headC, err := resolveCommit(r, head)
	if err != nil {
		return nil, fmt.Errorf("resolve --head %q: %w", head, err)
	}
	baseC, err := resolveCommit(r, base)
	if err != nil {
		return nil, fmt.Errorf("resolve --base %q: %w", base, err)
	}
	return diffPaths(baseC, headC)
}

// resolveCommit resolves a revision string (branch, tag, or SHA) to its
// commit object.
func resolveCommit(r *git.Repository, rev string) (*object.Commit, error) {
	hash, err := r.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, err
	}
	return r.CommitObject(*hash)
}

// diffPaths returns the deduplicated, sorted set of repo-relative paths that
// changed between from's tree and to's tree (added, modified, or deleted).
// from may be nil (a root commit with no parent), in which case every path
// in to's tree counts as touched.
func diffPaths(from, to *object.Commit) ([]string, error) {
	var fromTree *object.Tree
	if from != nil {
		t, err := from.Tree()
		if err != nil {
			return nil, err
		}
		fromTree = t
	}
	toTree, err := to.Tree()
	if err != nil {
		return nil, err
	}
	changes, err := object.DiffTree(fromTree, toTree)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, ch := range changes {
		add(ch.From.Name)
		add(ch.To.Name)
	}
	sort.Strings(out)
	return out, nil
}

// buildPRFeed joins every touched path to the shared FileFeatures assembly
// and computes each path's coupling_missing set (that path's historical
// coupling partners NOT also present in paths — the inverse signal, Context:
// "the strongest cheap brittleness predictor").
func buildPRFeed(store *Store, prNum int, paths []string) (PRFeatureFeed, error) {
	feed := PRFeatureFeed{PR: prNum, Files: []PRFileFeature{}}
	touched := map[string]bool{}
	for _, p := range paths {
		touched[p] = true
	}
	for _, p := range paths {
		features, err := AssembleFileFeatures(store, p)
		if err != nil {
			return PRFeatureFeed{}, err
		}
		feed.Files = append(feed.Files, prFileFeatureFrom(features, touched))
	}
	return feed, nil
}

// prFileFeatureFrom translates one FileFeatures join into its feed record,
// wiring the per-feature measured map from each field's own Measure state
// (task 3) — never a fabricated value for a feature that could not be
// measured.
func prFileFeatureFrom(f FileFeatures, touched map[string]bool) PRFileFeature {
	rec := PRFileFeature{Path: f.Path, Measured: map[string]string{}}

	rec.HotspotPercentile, rec.Measured["hotspot_percentile"] = measureFloatFeature(f.HotspotPercentile)
	rec.DefectDensity, rec.Measured["defect_density"] = measureFloatFeature(f.DefectDensity)
	rec.DefectTraceRate, rec.Measured["defect_trace_rate"] = measureFloatFeature(f.DefectTraceRate)
	rec.OwnershipTop, rec.Measured["ownership_top"] = measureFloatFeature(f.OwnershipTop)

	var missing []string
	for _, partner := range f.CouplingPartners {
		if !touched[partner] {
			missing = append(missing, partner)
		}
	}
	sort.Strings(missing)
	rec.CouplingMissing = missing
	if len(missing) > 0 {
		rec.Measured["coupling_missing"] = string(StateMeasured)
	} else {
		rec.Measured["coupling_missing"] = string(StateMeasuredZero)
	}
	return rec
}

// measureFloatFeature unwraps a Measure[float64] into the feed's flat-value +
// per-feature-state shape: a pointer present (and non-nil) only for measured
// or measured-zero, nil for could-not-measure — a caller reading the numeric
// field alone never mistakes an unmeasured feature for a real 0.
func measureFloatFeature(m Measure[float64]) (*float64, string) {
	switch m.State {
	case StateMeasured, StateMeasuredZero:
		v := m.Value
		return &v, string(m.State)
	default:
		return nil, string(StateCouldNotMeasure)
	}
}
