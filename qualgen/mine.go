package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// runMine is the `mine` mode: walk the target repo with go-git and extract its
// history into the append-only artifact tables under the tracking root.
//
// Full history by default; a repeat run reads the prior mine.json and extracts
// ONLY the commits that postdate the recorded tip, appending — never rewriting —
// and advancing the tip/horizon (extend-never-replace, spec §3.1). Every mode is
// READ-ONLY against the target repo; the only writes land under --out.
func runMine(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mine", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "path to the git repository to mine (read-only)")
	out := fs.String("out", "", "tracking root for artifacts (required unless --in-repo)")
	inRepo := fs.Bool("in-repo", false, "opt in to writing artifacts into the mined repo itself")
	blockMin := fs.Int("block-min", DefaultBlockMin, "minimum identical-line run to count as a moved/copied block (M1 §4.1)")
	churnDays := fs.Int("churn-window-days", DefaultChurnWindowDays, "churn window in days: a line revised/deleted within this of landing is churned (M1 §4.2)")
	identityMap := fs.String("identity-map", "", "path to a JSON author-identity class map (unmapped authors fall into an explicit 'unclassified' class)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	idMap, err := LoadIdentityMap(*identityMap)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen mine:", err)
		return 2
	}
	cfg := DefaultM1Config()
	cfg.BlockMin = *blockMin
	cfg.ChurnWindowDays = *churnDays
	cfg.Identity = idMap

	trackingRoot := *out
	if trackingRoot == "" {
		if *inRepo {
			// Explicit opt-in: default the tracking root to the mined repo.
			trackingRoot = *repo
		} else {
			fmt.Fprintln(stderr, "qualgen mine: --out <dir> is required (or pass --in-repo to write into the mined repo)")
			return 2
		}
	}

	if err := mineWithConfig(*repo, trackingRoot, stdout, cfg); err != nil {
		fmt.Fprintln(stderr, "qualgen mine:", err)
		return 1
	}
	return 0
}

// mine performs the extraction with the comparable-defaults M1 configuration.
// Separated from flag handling so tests drive it directly; it wraps
// mineWithConfig so existing callers keep the default block/churn/identity
// behaviour.
func mine(repoPath, trackingRoot string, stdout io.Writer) error {
	return mineWithConfig(repoPath, trackingRoot, stdout, DefaultM1Config())
}

// mineWithConfig extracts history and then aggregates the M1 metrics under the
// supplied configuration. The extraction is unchanged from the wave-0 skeleton;
// the aggregation passes read the freshly-mined commit + diff tables through the
// Store and append the derived metric families. Every step is READ-ONLY against
// the target repo; the only writes land under trackingRoot.
func mineWithConfig(repoPath, trackingRoot string, stdout io.Writer, cfg M1Config) error {
	// EnableDotGitCommonDir: the desk's own operating model dispatches every
	// worker into its OWN linked worktree (a `.git` file pointing at
	// `<main>/.git/worktrees/<name>`, common branch refs living in the main
	// repo's `.git`, per C1). Plain go-git PlainOpen does not follow a linked
	// worktree's `commondir` file, so `r.Head()` fails to resolve a branch ref
	// that lives in the common dir with "reference not found" — this was
	// caught running Verify #7 from inside a real dispatched worktree.
	r, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}
	tip := head.Hash().String()

	store := NewStore(trackingRoot)
	prior, err := store.ReadHeader()
	if err != nil {
		return fmt.Errorf("read prior header: %w", err)
	}

	priorTip := ""
	if prior != nil {
		priorTip = prior.TipSHA
	}

	// Walk newest-first from HEAD, stopping at the previously-mined tip so an
	// incremental run collects only the commits that postdate it.
	iter, err := r.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return fmt.Errorf("walk log: %w", err)
	}
	var collected []*object.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		if priorTip != "" && c.Hash.String() == priorTip {
			return storer.ErrStop // reached the mined frontier — exclude it and everything older
		}
		collected = append(collected, c)
		return nil
	})
	if err != nil {
		return fmt.Errorf("iterate commits: %w", err)
	}

	// Append oldest-first so the table reads chronologically; append-only means
	// an incremental run adds these lines after the ones already present without
	// touching them.
	reverse(collected)

	// Coverage and counts accumulate on top of what the prior mine recorded, so
	// the header always describes the WHOLE table, not just this run's delta.
	cov := Coverage{}
	commitCount, diffCount := 0, 0
	if prior != nil {
		cov = prior.Coverage
		commitCount = prior.CommitCount
		diffCount = prior.DiffCount
	}

	for _, c := range collected {
		com := commitRecord(c)
		fileDiffs, err := extractFileDiffs(c)
		if err != nil {
			return fmt.Errorf("diff commit %s: %w", c.Hash.String(), err)
		}
		for _, fd := range fileDiffs {
			com.FileDiffKeys = append(com.FileDiffKeys, fd.Key())
			if err := store.Append(KindDiff, fd); err != nil {
				return fmt.Errorf("append diff: %w", err)
			}
			diffCount++
			switch fd.Lines.State {
			case StateMeasured:
				cov.Measured++
			case StateMeasuredZero:
				cov.MeasuredZero++
			case StateCouldNotMeasure:
				cov.CouldNotMeasure++
			}
		}
		if err := store.Append(KindCommit, com); err != nil {
			return fmt.Errorf("append commit: %w", err)
		}
		commitCount++
	}

	// Horizon is the earliest reachable commit ever seen. A full mine sets it to
	// the oldest commit reached now; an incremental run keeps the original floor.
	horizon := ""
	if prior != nil && prior.Horizon != "" {
		horizon = prior.Horizon
	} else if len(collected) > 0 {
		horizon = collected[0].Hash.String() // oldest, after the reverse above
	}

	runAt := time.Now().UTC()
	header := MineHeader{
		MinedAt:         runAt,
		TipSHA:          tip,
		Horizon:         horizon,
		Discontinuities: detectDiscontinuities(repoPath, prior),
		Coverage:        cov,
		CommitCount:     commitCount,
		DiffCount:       diffCount,
	}
	if err := store.WriteHeader(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// M1 metric families over the full mined tables (append-only fresh snapshot):
	// the hotspot / ownership / coupling families (quality/03) and the
	// line-operation taxonomy + churn/rework family (quality/02).
	if err := appendM1Metrics(store, r, head.Hash(), runAt); err != nil {
		return fmt.Errorf("compute M1 hotspot/ownership/coupling metrics: %w", err)
	}
	if err := aggregateM1(store, cfg); err != nil {
		return fmt.Errorf("aggregate M1 taxonomy/churn metrics: %w", err)
	}

	fmt.Fprintf(stdout, "qualgen mine: %d new commit(s) extracted; tip %s; %d commit(s) in table\n",
		len(collected), shortSHA(tip), commitCount)
	return nil
}

// appendM1Metrics computes the hotspot, ownership and change-coupling
// families (spec §4.3-§4.5) over the FULL mined commit/diff tables — not just
// this run's incremental delta, since decay, ownership and co-change
// baselines are all whole-history aggregates — and appends the result to
// metrics.jsonl. Each `mine` run appends a fresh full snapshot; the tables
// stay append-only (extend, never rewrite), so trend consumers (quality/05)
// read the most recent snapshot per family.
func appendM1Metrics(store *Store, r *git.Repository, tip plumbing.Hash, runAt time.Time) error {
	allCommits, err := store.ReadCommits()
	if err != nil {
		return fmt.Errorf("read commits: %w", err)
	}
	allDiffs, err := store.ReadDiffs()
	if err != nil {
		return fmt.Errorf("read diffs: %w", err)
	}

	tipCommit, err := r.CommitObject(tip)
	if err != nil {
		return fmt.Errorf("resolve tip commit: %w", err)
	}
	allPaths, err := treePaths(tipCommit)
	if err != nil {
		return fmt.Errorf("list tip tree: %w", err)
	}

	for _, rec := range ComputeHotspots(allCommits, allDiffs, allPaths, HotspotParams{Now: runAt}) {
		if err := store.Append(KindMetric, rec); err != nil {
			return fmt.Errorf("append hotspot metric: %w", err)
		}
	}
	for _, rec := range ComputeOwnership(allCommits, allDiffs, DefaultIdentityClassifier, DefaultBusFactorThresholdPct, runAt) {
		if err := store.Append(KindMetric, rec); err != nil {
			return fmt.Errorf("append ownership metric: %w", err)
		}
	}
	pairs, missing := ComputeCoupling(allCommits, allDiffs, DefaultCouplingParams(), runAt)
	for _, rec := range pairs {
		if err := store.Append(KindMetric, rec); err != nil {
			return fmt.Errorf("append coupling metric: %w", err)
		}
	}
	for _, rec := range missing {
		if err := store.Append(KindMetric, rec); err != nil {
			return fmt.Errorf("append missing-coupling-partner metric: %w", err)
		}
	}
	return nil
}

// treePaths lists every regular file path in a commit's tree — the full path
// universe ComputeHotspots uses for its measured-zero case (a file present at
// the tip but never touched in the mined window, spec §4.3).
func treePaths(c *object.Commit) ([]string, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}
	var paths []string
	iter := tree.Files()
	defer iter.Close()
	err = iter.ForEach(func(f *object.File) error {
		paths = append(paths, f.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// commitRecord builds the internal Commit record from a go-git commit. Author
// identity is recorded raw; classification is a later brief's job.
func commitRecord(c *object.Commit) Commit {
	com := Commit{
		SHA:            c.Hash.String(),
		AuthorRaw:      fmt.Sprintf("%s <%s>", c.Author.Name, c.Author.Email),
		AuthorName:     c.Author.Name,
		AuthorEmail:    c.Author.Email,
		AuthorWhen:     c.Author.When.UTC(),
		CommitterRaw:   fmt.Sprintf("%s <%s>", c.Committer.Name, c.Committer.Email),
		CommitterName:  c.Committer.Name,
		CommitterEmail: c.Committer.Email,
		CommitterWhen:  c.Committer.When.UTC(),
		Message:        c.Message,
	}
	for _, p := range c.ParentHashes {
		com.ParentSHAs = append(com.ParentSHAs, p.String())
	}
	return com
}

// extractFileDiffs computes the per-file diffs for a commit against its first
// parent (or the empty tree for a root commit). Each file flows through the
// three-state rule via fileDiffFromChange.
func extractFileDiffs(c *object.Commit) ([]FileDiff, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}
	var parentTree *object.Tree
	if c.NumParents() > 0 {
		// First-parent diff: the mainline change this commit introduced. Merge
		// commits against other parents are a later-brief refinement.
		p, err := c.Parent(0)
		if err != nil {
			return nil, err
		}
		parentTree, err = p.Tree()
		if err != nil {
			return nil, err
		}
	}
	changes, err := object.DiffTree(parentTree, tree)
	if err != nil {
		return nil, err
	}
	out := make([]FileDiff, 0, len(changes))
	for _, ch := range changes {
		out = append(out, fileDiffFromChange(c.Hash.String(), ch))
	}
	return out, nil
}

// fileDiffFromChange turns one go-git change into a FileDiff, applying the
// three-state rule resiliently: a patch that cannot be computed (unreadable
// blob) is could-not-measure with a reason, a change with no content patch
// (mode-only) is measured-zero, and everything else goes through
// fileDiffFromPatch.
func fileDiffFromChange(sha string, ch *object.Change) FileDiff {
	patch, err := ch.Patch()
	if err != nil {
		fd := baseFileDiff(sha, ch)
		fd.Lines = CouldNotMeasure[[]Hunk]("patch could not be computed for this blob")
		return fd
	}
	fps := patch.FilePatches()
	if len(fps) == 0 {
		fd := baseFileDiff(sha, ch)
		fd.Lines = MeasuredZero[[]Hunk]()
		return fd
	}
	return fileDiffFromPatch(sha, fps[0])
}

// baseFileDiff builds the path/kind skeleton of a FileDiff from a change,
// without line data — used on the could-not-measure and empty-patch paths.
func baseFileDiff(sha string, ch *object.Change) FileDiff {
	fd := FileDiff{CommitSHA: sha, OldPath: ch.From.Name, NewPath: ch.To.Name}
	switch {
	case ch.From.Name == "" && ch.To.Name != "":
		fd.Kind = ChangeAdded
	case ch.From.Name != "" && ch.To.Name == "":
		fd.Kind = ChangeDeleted
	default:
		fd.Kind = ChangeModified
	}
	return fd
}

// detectDiscontinuities records gaps that floor what the mine could see. It
// carries forward any the prior header recorded and adds a shallow-clone floor
// if the repository is shallow.
func detectDiscontinuities(repoPath string, prior *MineHeader) []Discontinuity {
	seen := map[string]bool{}
	var out []Discontinuity
	add := func(d Discontinuity) {
		if seen[d.Kind] {
			return
		}
		seen[d.Kind] = true
		out = append(out, d)
	}
	if prior != nil {
		for _, d := range prior.Discontinuities {
			add(d)
		}
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git", "shallow")); err == nil {
		add(Discontinuity{
			Kind:   "shallow-clone-floor",
			Detail: "repository is a shallow clone; history before the shallow floor is unreachable",
		})
	}
	return out
}

// reverse flips a commit slice in place (newest-first → oldest-first).
func reverse(cs []*object.Commit) {
	for i, j := 0, len(cs)-1; i < j; i, j = i+1, j-1 {
		cs[i], cs[j] = cs[j], cs[i]
	}
}

// shortSHA abbreviates a SHA for log output.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
