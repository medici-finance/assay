package main

import (
	"sort"
	"time"
)

// Default change-coupling baseline (spec §4.5): a pair must co-change in at
// least MinCoChanges commits, AND the co-change count must be at least
// MinRatio of the rarer of the two files' own change counts, before the pair
// is flagged coupled. Both configurable via CouplingParams.
const (
	DefaultCouplingMinRatio     = 0.5
	DefaultCouplingMinCoChanges = 2
	// DefaultMaxFilesPerCommit excludes "shotgun" commits — a bulk rename,
	// restage, or repo-wide reformat that touches many files at once — from
	// pairwise coupling counting. code-maat and Tornhill's coupling analyses
	// apply the same filter: a commit touching hundreds of files co-occurs
	// with everything and contributes combinatorial noise, not a genuine
	// logical relationship (a commit touching k files contributes C(k,2)
	// pairs — quadratic in k). The file still counts toward its own
	// change-frequency denominator; only its PAIR contributions are skipped.
	DefaultMaxFilesPerCommit = 30
)

// CouplingParams configures the change-coupling baseline (spec §4.5).
type CouplingParams struct {
	MinRatio     float64
	MinCoChanges int
	// MaxFilesPerCommit excludes a commit from pairwise co-occurrence
	// counting once it touches more than this many distinct files (a
	// "shotgun commit" — see DefaultMaxFilesPerCommit). Zero (or negative)
	// defaults to DefaultMaxFilesPerCommit.
	MaxFilesPerCommit int
}

// DefaultCouplingParams returns the spec-default baseline.
func DefaultCouplingParams() CouplingParams {
	return CouplingParams{
		MinRatio:          DefaultCouplingMinRatio,
		MinCoChanges:      DefaultCouplingMinCoChanges,
		MaxFilesPerCommit: DefaultMaxFilesPerCommit,
	}
}

// CouplingRecord is one file-pair change-coupling row (spec §4.5).
type CouplingRecord struct {
	Metric    string    `json:"metric"` // "coupling"
	PathA     string    `json:"path_a"`
	PathB     string    `json:"path_b"`
	CoChanges int       `json:"co_changes"`
	ChangesA  int       `json:"changes_a"`
	ChangesB  int       `json:"changes_b"`
	Ratio     float64   `json:"ratio"`
	Coupled   bool      `json:"coupled"`
	MinRatio  float64   `json:"min_ratio"`
	MinedAt   time.Time `json:"mined_at"`
}

// MissingPartnerRecord is one instance of the INVERSE coupling signal (spec
// §4.5, §9.1): a commit touched Path without touching its established
// coupling Partner — the strongest cheap brittleness predictor in the
// coupling literature.
type MissingPartnerRecord struct {
	Metric    string    `json:"metric"` // "missing_coupling_partner"
	CommitSHA string    `json:"commit_sha"`
	Path      string    `json:"path"`
	Partner   string    `json:"partner"`
	Ratio     float64   `json:"ratio"`
	MinedAt   time.Time `json:"mined_at"`
}

// ComputeCoupling computes the change-coupling family from the mined commit
// and diff tables: the per-pair coupling table, plus the inverse
// missing-partner signal for every historical instance where a coupled pair
// changed apart (spec §4.5).
//
// Coupling is measured at COMMIT granularity — the diff table's atomic
// changeset — since PR metadata is not yet mined (a later brief may refine
// this to PR granularity once it is). Files that never co-occur in any
// commit are never scored as a pair at all: they raise neither the coupled
// nor the missing-partner signal, which is the correct "independent files"
// answer, not a could-not-measure — the instrument looked and found no
// relationship.
func ComputeCoupling(commits []Commit, diffs []FileDiff, params CouplingParams, now time.Time) ([]CouplingRecord, []MissingPartnerRecord) {
	if params.MinCoChanges <= 0 {
		params.MinCoChanges = DefaultCouplingMinCoChanges
	}
	if params.MinRatio <= 0 {
		params.MinRatio = DefaultCouplingMinRatio
	}
	if params.MaxFilesPerCommit <= 0 {
		params.MaxFilesPerCommit = DefaultMaxFilesPerCommit
	}

	bySHA := make(map[string]Commit, len(commits))
	for _, c := range commits {
		bySHA[c.SHA] = c
	}

	// Reduce each commit to its set of distinct touched paths, in commit
	// order — the atomic changeset coupling is measured over.
	touchedByCommit := map[string]map[string]bool{}
	var order []string
	for _, fd := range diffs {
		if _, ok := bySHA[fd.CommitSHA]; !ok {
			continue
		}
		p := fd.NewPath
		if p == "" {
			p = fd.OldPath
		}
		if p == "" {
			continue
		}
		set, seen := touchedByCommit[fd.CommitSHA]
		if !seen {
			set = map[string]bool{}
			touchedByCommit[fd.CommitSHA] = set
			order = append(order, fd.CommitSHA)
		}
		set[p] = true
	}
	// order accumulates as diffs are scanned; diffs are append-only in
	// commit-chronological order, but sort defensively so behaviour never
	// depends on the caller's diff ordering.
	sort.Slice(order, func(i, j int) bool {
		return bySHA[order[i]].AuthorWhen.Before(bySHA[order[j]].AuthorWhen)
	})

	changeCount := map[string]int{}
	coCount := map[[2]string]int{}
	for _, sha := range order {
		paths := make([]string, 0, len(touchedByCommit[sha]))
		for p := range touchedByCommit[sha] {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			changeCount[p]++
		}
		// A shotgun commit still counts toward each file's own
		// change-frequency denominator above; it just contributes no
		// pairwise co-occurrence evidence (DefaultMaxFilesPerCommit).
		if len(paths) > params.MaxFilesPerCommit {
			continue
		}
		for i := 0; i < len(paths); i++ {
			for j := i + 1; j < len(paths); j++ {
				coCount[[2]string{paths[i], paths[j]}]++
			}
		}
	}

	pairKeys := make([][2]string, 0, len(coCount))
	for k := range coCount {
		pairKeys = append(pairKeys, k)
	}
	sort.Slice(pairKeys, func(i, j int) bool {
		if pairKeys[i][0] != pairKeys[j][0] {
			return pairKeys[i][0] < pairKeys[j][0]
		}
		return pairKeys[i][1] < pairKeys[j][1]
	})

	records := make([]CouplingRecord, 0, len(pairKeys))
	coupledPairs := map[[2]string]CouplingRecord{}
	for _, k := range pairKeys {
		a, b := k[0], k[1]
		co := coCount[k]
		ca, cb := changeCount[a], changeCount[b]
		minCount := ca
		if cb < minCount {
			minCount = cb
		}
		ratio := 0.0
		if minCount > 0 {
			ratio = float64(co) / float64(minCount)
		}
		rec := CouplingRecord{
			Metric:    "coupling",
			PathA:     a,
			PathB:     b,
			CoChanges: co,
			ChangesA:  ca,
			ChangesB:  cb,
			Ratio:     ratio,
			Coupled:   co >= params.MinCoChanges && ratio >= params.MinRatio,
			MinRatio:  params.MinRatio,
			MinedAt:   now,
		}
		records = append(records, rec)
		if rec.Coupled {
			coupledPairs[k] = rec
		}
	}

	var missing []MissingPartnerRecord
	for _, sha := range order {
		touched := touchedByCommit[sha]
		for k, rec := range coupledPairs {
			a, b := k[0], k[1]
			switch {
			case touched[a] && !touched[b]:
				missing = append(missing, MissingPartnerRecord{
					Metric: "missing_coupling_partner", CommitSHA: sha, Path: a, Partner: b, Ratio: rec.Ratio, MinedAt: now,
				})
			case touched[b] && !touched[a]:
				missing = append(missing, MissingPartnerRecord{
					Metric: "missing_coupling_partner", CommitSHA: sha, Path: b, Partner: a, Ratio: rec.Ratio, MinedAt: now,
				})
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].CommitSHA != missing[j].CommitSHA {
			return missing[i].CommitSHA < missing[j].CommitSHA
		}
		return missing[i].Path < missing[j].Path
	})

	return records, missing
}
