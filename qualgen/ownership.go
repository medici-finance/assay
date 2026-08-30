package main

import (
	"path"
	"sort"
	"time"
)

// DefaultBusFactorThresholdPct is the default "K%" bus-factor threshold (spec
// §4.4): the minimum identity/role set owning MORE than this share of a file
// or package's surviving lines. Configurable per call.
const DefaultBusFactorThresholdPct = 50.0

// IdentityClassifier maps a mined Commit to the two ownership axes spec §4.4
// requires: the author-identity class, and the DISPATCHING ROLE. In an
// agent-fleet repo a single role (e.g. "the worker desk") can commit under
// many different bot accounts or session identities, so identity alone
// under-counts a process SPOF — concentration in one role is a SPOF even when
// line-authors vary (spec §4.4).
//
// A real role signal (a commit trailer, a bot-account naming convention, a
// roster lookup) is house-specific and repo-agnostic qualgen ships no guess
// at it — the exact author-identity-class partition is an open spec question
// (§13.3). Callers that have one inject it here; DefaultIdentityClassifier is
// the honest fallback: role equals identity absent one.
type IdentityClassifier func(Commit) (identity, role string)

// DefaultIdentityClassifier treats the raw commit author (email, falling back
// to name when email is empty) as both the identity and the role.
func DefaultIdentityClassifier(c Commit) (identity, role string) {
	id := c.AuthorEmail
	if id == "" {
		id = c.AuthorName
	}
	return id, id
}

// OwnershipRecord is one per-file or per-package ownership row (spec §4.4).
type OwnershipRecord struct {
	Metric            string             `json:"metric"` // "ownership"
	Grain             string             `json:"grain"`  // "file" | "package"
	Path              string             `json:"path"`
	SurvivingLines    int                `json:"surviving_lines"`
	IdentityShares    map[string]float64 `json:"identity_shares"`
	RoleShares        map[string]float64 `json:"role_shares"`
	BusFactorIdentity Measure[int]       `json:"bus_factor_identity"`
	BusFactorRole     Measure[int]       `json:"bus_factor_role"`
	// RoleSPOF is set when a single dispatching role owns above threshold
	// even though more than one identity contributed (spec §4.4) — the
	// process-SPOF signal identity concentration alone would miss.
	RoleSPOF     bool      `json:"role_spof"`
	ThresholdPct float64   `json:"threshold_pct"`
	MinedAt      time.Time `json:"mined_at"`
}

// lineOwner is one surviving line's attribution, tracked per path.
type lineOwner struct {
	content, identity, role string
}

// ComputeOwnership computes per-file and per-package ownership concentration
// and bus factor (spec §4.4) from the mined commit and diff tables.
//
// Ownership is tracked over SURVIVING lines: an added line increments its
// author's tally for that path; a later deletion decrements it, matched by
// content (falling back to the oldest still-surviving entry when no exact
// content match remains, e.g. a reflowed line) so the count never goes stale.
// This is a diff-derived approximation of blame rather than a true git-blame
// read — it stays within quality/01's diff-only Store contract (the brief's
// declared READ-ONLY interface) rather than re-opening the live repository.
// A file's diffs that are could-not-measure (binary/unreadable) contribute no
// line-level attribution, honestly, rather than a guess.
func ComputeOwnership(commits []Commit, diffs []FileDiff, classify IdentityClassifier, thresholdPct float64, now time.Time) []OwnershipRecord {
	if classify == nil {
		classify = DefaultIdentityClassifier
	}
	if thresholdPct <= 0 {
		thresholdPct = DefaultBusFactorThresholdPct
	}

	bySHA := make(map[string]Commit, len(commits))
	for _, c := range commits {
		bySHA[c.SHA] = c
	}

	surv := map[string][]lineOwner{}

	for _, fd := range diffs {
		c, ok := bySHA[fd.CommitSHA]
		if !ok {
			continue
		}
		p := fd.NewPath
		if p == "" {
			p = fd.OldPath
		}
		if p == "" {
			continue
		}
		if fd.Kind == ChangeDeleted {
			delete(surv, p)
			continue
		}
		if fd.Lines.State != StateMeasured {
			continue // binary/unreadable: no line-level attribution possible
		}
		identity, role := classify(c)
		for _, hunk := range fd.Lines.Value {
			for _, lc := range hunk.Lines {
				switch lc.Op {
				case OpAdd:
					surv[p] = append(surv[p], lineOwner{content: lc.Content, identity: identity, role: role})
				case OpDel:
					surv[p] = removeSurvivor(surv[p], lc.Content)
				}
			}
		}
	}

	var out []OwnershipRecord
	pkgLines := map[string][]lineOwner{}
	filePaths := make([]string, 0, len(surv))
	for p, lines := range surv {
		if len(lines) == 0 {
			continue
		}
		filePaths = append(filePaths, p)
		pkgLines[path.Dir(p)] = append(pkgLines[path.Dir(p)], lines...)
	}
	sort.Strings(filePaths)
	for _, p := range filePaths {
		out = append(out, ownershipRecord("file", p, surv[p], thresholdPct, now))
	}

	pkgPaths := make([]string, 0, len(pkgLines))
	for p := range pkgLines {
		pkgPaths = append(pkgPaths, p)
	}
	sort.Strings(pkgPaths)
	for _, p := range pkgPaths {
		out = append(out, ownershipRecord("package", p, pkgLines[p], thresholdPct, now))
	}

	return out
}

// removeSurvivor removes one surviving entry matching content — the line the
// diff records as deleted. If no exact content match remains, it falls back
// to the oldest still-surviving entry (index 0) so the surviving count never
// drifts out of sync with the mined delete events.
func removeSurvivor(lines []lineOwner, content string) []lineOwner {
	for i, l := range lines {
		if l.content == content {
			return append(lines[:i:i], lines[i+1:]...)
		}
	}
	if len(lines) > 0 {
		return lines[1:]
	}
	return lines
}

func ownershipRecord(grain, p string, lines []lineOwner, thresholdPct float64, now time.Time) OwnershipRecord {
	identityCounts := map[string]int{}
	roleCounts := map[string]int{}
	for _, l := range lines {
		identityCounts[l.identity]++
		roleCounts[l.role]++
	}
	total := len(lines)
	busIdentity := busFactor(identityCounts, total, thresholdPct)
	busRole := busFactor(roleCounts, total, thresholdPct)

	roleSPOF := busRole.State == StateMeasured && busRole.Value == 1 &&
		busIdentity.State == StateMeasured && busIdentity.Value > 1

	return OwnershipRecord{
		Metric:            "ownership",
		Grain:             grain,
		Path:              p,
		SurvivingLines:    total,
		IdentityShares:    shares(identityCounts, total),
		RoleShares:        shares(roleCounts, total),
		BusFactorIdentity: busIdentity,
		BusFactorRole:     busRole,
		RoleSPOF:          roleSPOF,
		ThresholdPct:      thresholdPct,
		MinedAt:           now,
	}
}

// shares turns raw counts into fractional shares of total.
func shares(counts map[string]int, total int) map[string]float64 {
	out := make(map[string]float64, len(counts))
	if total == 0 {
		return out
	}
	for k, v := range counts {
		out[k] = float64(v) / float64(total)
	}
	return out
}

// busFactor is the minimum identity/role set owning MORE than thresholdPct%
// of the surviving lines (spec §4.4): sort by descending share (ties broken
// by key for determinism) and accumulate until the running share exceeds the
// threshold. Zero surviving lines is a genuine measured-zero — there is
// nothing to own — never conflated with an unmeasurable input.
func busFactor(counts map[string]int, total int, thresholdPct float64) Measure[int] {
	if total == 0 {
		return MeasuredZero[int]()
	}
	type kv struct {
		k string
		v int
	}
	kvs := make([]kv, 0, len(counts))
	for k, v := range counts {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		if kvs[i].v != kvs[j].v {
			return kvs[i].v > kvs[j].v
		}
		return kvs[i].k < kvs[j].k
	})
	running := 0.0
	for i, e := range kvs {
		running += float64(e.v) / float64(total) * 100
		if running > thresholdPct {
			return Measured(i + 1)
		}
	}
	return Measured(len(kvs))
}
