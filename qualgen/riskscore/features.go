package riskscore

import (
	"math"
	"path"
	"time"
)

// AuthorClass is the identity class of who authored a change (spec §4.2/§4.4):
// a human, an autonomous agent, or non-agent automation (a bot/CI). It is a
// first-class JIT feature because in an agent-fleet repo the identity class is a
// defect signal in its own right.
type AuthorClass string

const (
	AuthorHuman      AuthorClass = "human"
	AuthorAgent      AuthorClass = "agent"
	AuthorAutomation AuthorClass = "automation"
	AuthorUnknown    AuthorClass = "unknown"
)

// Change is one scored unit — a commit or a merged PR — carrying the metadata
// the JIT feature extractor and the §9.1 heuristic both draw on. It is the
// package's corpus grain: quality/07's M2 trace + M1 aggregates populate these
// fields for real, and a fixture populates them directly in tests. The package
// never re-mines git; it consumes an already-mined, already-traced record.
type Change struct {
	ID string `json:"id"` // stable change id (commit SHA or PR key)

	// CommitTime is when the change was authored/committed; MergeTime is when it
	// landed on the mainline. The TEMPORAL split (learned.go) orders the corpus
	// by MergeTime when present, falling back to CommitTime — a change is only
	// ever training data for predictions AFTER it landed.
	CommitTime time.Time `json:"commit_time"`
	MergeTime  time.Time `json:"merge_time"`

	// Diffusion inputs.
	Files      []string `json:"files"`      // paths the change touched
	Subsystems []string `json:"subsystems"` // pre-derived subsystems, if any

	// Size inputs.
	LinesAdded   int `json:"lines_added"`
	LinesDeleted int `json:"lines_deleted"`
	// LinesTotal is the total size (LoC) of the touched files BEFORE the change
	// — Kamei's LT. Zero means unknown, handled as measured-zero downstream.
	LinesTotal int `json:"lines_total"`
	// ChangedLinesPerFile is the per-file changed-line distribution used for the
	// diffusion ENTROPY feature. When empty, entropy falls back to a uniform
	// split of LinesAdded+LinesDeleted across Files.
	ChangedLinesPerFile []int `json:"changed_lines_per_file"`

	// History inputs (from the M1/M2 corpus, as-of the change's landing).
	PriorChangesToFiles int     `json:"prior_changes_to_files"` // Kamei NUC
	PriorDefectsToFiles int     `json:"prior_defects_to_files"` // traced prior defects on this surface
	RecentChurn         int     `json:"recent_churn"`           // churned lines on the surface in the window
	NDev                int     `json:"ndev"`                   // distinct identities that touched the files
	AgeDays             float64 `json:"age_days"`               // avg days since the touched files last changed

	// Author-class inputs.
	AuthorClass AuthorClass `json:"author_class"`
	AuthorExp   int         `json:"author_exp"` // Kamei EXP: prior changes by this identity

	// Heuristic inputs — the §9.1 hand-weighted features for this change, each
	// three-state. These are what the heuristic layer weights and what every
	// learned score must carry as its explanation.
	Heuristic HeuristicFeatures `json:"heuristic"`
}

// landedAt returns the change's ordering time for the temporal split: MergeTime
// when set, else CommitTime. A change with neither is not temporally placeable
// and is treated as the zero time (sorts first / excluded by any as-of cutoff).
func (c Change) landedAt() time.Time {
	if !c.MergeTime.IsZero() {
		return c.MergeTime
	}
	return c.CommitTime
}

// JITFeatures is the Kamei-style just-in-time feature vector for a change,
// grouped into the four families the JIT-quality literature (spec §12) uses:
// diffusion, size, history, and author-class/experience.
type JITFeatures struct {
	// Diffusion — how spread out the change is.
	NS      int     `json:"ns"`      // number of modified subsystems
	ND      int     `json:"nd"`      // number of modified directories
	NF      int     `json:"nf"`      // number of modified files
	Entropy float64 `json:"entropy"` // distribution of the change across files, in [0,1]

	// Size.
	LA int `json:"la"` // lines added
	LD int `json:"ld"` // lines deleted
	LT int `json:"lt"` // lines of code in the touched files before the change

	// History.
	NDEV int     `json:"ndev"` // distinct identities that previously touched the files
	AGE  float64 `json:"age"`  // avg days since the files last changed
	NUC  int     `json:"nuc"`  // number of unique prior changes to the files
	NPD  int     `json:"npd"`  // number of prior traced defects on the files
	RC   int     `json:"rc"`   // recent churn on the surface

	// Author-class / experience.
	Class AuthorClass `json:"class"`
	EXP   int         `json:"exp"` // prior changes by this identity
}

// ExtractJIT computes the Kamei JIT feature vector for a change. It is a pure
// function of the change record — no git access — so it is deterministic and
// unit-testable against a fixture.
func ExtractJIT(c Change) JITFeatures {
	return JITFeatures{
		NS:      countSubsystems(c),
		ND:      countDirs(c.Files),
		NF:      len(c.Files),
		Entropy: changeEntropy(c),
		LA:      c.LinesAdded,
		LD:      c.LinesDeleted,
		LT:      c.LinesTotal,
		NDEV:    c.NDev,
		AGE:     c.AgeDays,
		NUC:     c.PriorChangesToFiles,
		NPD:     c.PriorDefectsToFiles,
		RC:      c.RecentChurn,
		Class:   normClass(c.AuthorClass),
		EXP:     c.AuthorExp,
	}
}

// Vector is the numeric feature vector fed to the learned model, in a FIXED
// order. The author class is one-hot encoded (agent, automation; human/unknown
// is the reference level). The order here is the model's feature contract — it
// must match FeatureNames.
func (f JITFeatures) Vector() []float64 {
	isAgent, isAuto := 0.0, 0.0
	switch f.Class {
	case AuthorAgent:
		isAgent = 1
	case AuthorAutomation:
		isAuto = 1
	}
	return []float64{
		float64(f.NS),
		float64(f.ND),
		float64(f.NF),
		f.Entropy,
		float64(f.LA),
		float64(f.LD),
		float64(f.LT),
		float64(f.NDEV),
		f.AGE,
		float64(f.NUC),
		float64(f.NPD),
		float64(f.RC),
		float64(f.EXP),
		isAgent,
		isAuto,
	}
}

// FeatureNames names each column of Vector, in the same fixed order. It is the
// human-readable index into the learned model's weights.
func FeatureNames() []string {
	return []string{
		"ns", "nd", "nf", "entropy",
		"la", "ld", "lt",
		"ndev", "age", "nuc", "npd", "rc",
		"exp", "class_agent", "class_automation",
	}
}

// changeEntropy is the diffusion-entropy feature: the Shannon entropy of the
// change's line distribution across the touched files, normalized to [0,1] by
// the maximum entropy for that file count (log NF). A change concentrated in one
// file has entropy 0; one spread evenly across many files approaches 1. Kamei
// finds a spread-out change more defect-prone than a concentrated one of equal
// size.
func changeEntropy(c Change) float64 {
	weights := c.ChangedLinesPerFile
	if len(weights) == 0 {
		// No per-file breakdown: assume the change spread uniformly across its
		// files. Uniform over n files has entropy exactly 1 after normalization.
		n := len(c.Files)
		if n <= 1 {
			return 0
		}
		return 1
	}
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += float64(w)
		}
	}
	if total == 0 {
		return 0
	}
	var h float64
	nonzero := 0
	for _, w := range weights {
		if w <= 0 {
			continue
		}
		p := float64(w) / total
		h -= p * math.Log(p)
		nonzero++
	}
	if nonzero <= 1 {
		return 0
	}
	return h / math.Log(float64(nonzero)) // normalize by max entropy
}

// countDirs counts the distinct directories the change's files live in.
func countDirs(files []string) int {
	seen := map[string]struct{}{}
	for _, f := range files {
		seen[path.Dir(f)] = struct{}{}
	}
	return len(seen)
}

// countSubsystems counts subsystems: the explicit Subsystems list when the
// corpus supplies one, else the distinct top-level path component of each file
// (Kamei's "subsystem" = root directory).
func countSubsystems(c Change) int {
	if len(c.Subsystems) > 0 {
		seen := map[string]struct{}{}
		for _, s := range c.Subsystems {
			seen[s] = struct{}{}
		}
		return len(seen)
	}
	seen := map[string]struct{}{}
	for _, f := range c.Files {
		seen[topComponent(f)] = struct{}{}
	}
	return len(seen)
}

// topComponent returns the first path segment of a cleaned path ("a/b/c" -> "a").
func topComponent(p string) string {
	p = path.Clean(p)
	for {
		dir := path.Dir(p)
		if dir == "." || dir == "/" || dir == p {
			return p
		}
		p = dir
	}
}

func normClass(c AuthorClass) AuthorClass {
	switch c {
	case AuthorHuman, AuthorAgent, AuthorAutomation:
		return c
	default:
		return AuthorUnknown
	}
}
