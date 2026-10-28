package reviewflow

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const ObservationSchema = "review-observation-v1"
const ObservationEvaluator = "reviewflow-v1"

// Repository is a canonical, provider-qualified identity. Collectors resolve aliases
// before entry; this boundary never guesses that two spellings name the same repo.
type Repository struct {
	Provider   string `json:"provider"`
	Host       string `json:"host"`
	Repository string `json:"repository"`
}

func (r Repository) String() string { return r.Provider + "://" + r.Host + "/" + r.Repository }

type ChangeIdentity struct {
	Repository Repository `json:"repository"`
	Number     int        `json:"number"`
	Head       string     `json:"head"`
	Base       string     `json:"base"` // empty only when BaseUsed is false
	BaseUsed   bool       `json:"baseUsed"`
}
type CollectionInterval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// InputProvenance describes a read, not authentication. Revision identifies the
// observed input set even when its contents are empty. Unknown freshness and
// incomplete coverage are representable but cannot support a successful plan.
type InputProvenance struct {
	Source     string    `json:"source"`
	Version    string    `json:"version"`
	Revision   string    `json:"revision"`
	ObservedAt time.Time `json:"observedAt"`
	Freshness  string    `json:"freshness"`
	Complete   bool      `json:"complete"`
}
type PopulationCoverage struct {
	Repository Repository      `json:"repository"`
	Input      InputProvenance `json:"input"`
	Count      int             `json:"count"` // uncapped number of admitted changes in this repo
}

// Applicability binds the resolved facts to every mutable input they used. A head
// alone cannot identify an observation of edited text, reviews, or rerun checks.
type Applicability struct {
	Head    string `json:"head"`
	Base    string `json:"base"`
	Body    string `json:"body"`
	Reviews string `json:"reviews"`
	Checks  string `json:"checks"`
}
type ReviewInputs struct {
	Subject    InputProvenance `json:"subject"`
	Body       InputProvenance `json:"body"`
	Reviews    InputProvenance `json:"reviews"`
	Checks     InputProvenance `json:"checks"`
	Enrichment InputProvenance `json:"enrichment"`
}

// ClassificationFacts are the existing command's resolved facts. Review identity,
// stale-grant filtering, newest-check selection, body-edit re-verification and
// non-commit resolution stay with their canonical operational owners. They are
// not reimplemented by the envelope and convey no write authority.
type ClassificationFacts struct {
	Change                  ChangeFact  `json:"change"`
	Reviews                 ReviewState `json:"reviews"`
	CIRequired              bool        `json:"ciRequired"`
	ZeroCI                  string      `json:"zeroCI"`
	OwnFilesChanged         bool        `json:"ownFilesChanged"`
	RiskClassed             bool        `json:"riskClassed"`
	RiskReason              string      `json:"riskReason"`
	NonCommitResolution     bool        `json:"nonCommitResolution"`
	NonCommitResolutionNote string      `json:"nonCommitResolutionNote"`
}

func (f ClassificationFacts) ClassifyInput() ClassifyInput {
	in := BuildClassifyInput(f.Change, f.Reviews, f.CIRequired, f.ZeroCI)
	in.OwnFilesChanged, in.RiskClassed, in.RiskReason = f.OwnFilesChanged, f.RiskClassed, f.RiskReason
	in.NonCommitResolution, in.NonCommitResolutionNote = f.NonCommitResolution, f.NonCommitResolutionNote
	return in
}

// LaneConclusion is independently resolved by the canonical lane owner. The
// legacy advisory aggregate is deliberately not relabeled as a correctness grant.
type LaneConclusion struct {
	Name      string        `json:"name"`
	State     string        `json:"state"` // approved, blocked, missing, stale, unknown
	AppliesTo Applicability `json:"appliesTo"`
}
type ObservedChange struct {
	Lanes     []LaneConclusion    `json:"lanes"`
	Identity  ChangeIdentity      `json:"identity"`
	Inputs    ReviewInputs        `json:"inputs"`
	AppliesTo Applicability       `json:"appliesTo"`
	Facts     ClassificationFacts `json:"facts"`
	Score     int                 `json:"score"`
}

// ObservationInputs is the full population, never a display prefix. Evaluator and
// policy revisions are checked against caller expectations, not trusted from JSON.
type ObservationInputs struct {
	Evaluator       string               `json:"evaluator"`
	Policy          string               `json:"policy"`
	ReviewerDisplay string               `json:"reviewerDisplay"`
	Requested       []Repository         `json:"requested"`
	Admitted        []Repository         `json:"admitted"`
	Collection      CollectionInterval   `json:"collection"`
	Population      []PopulationCoverage `json:"population"`
	Changes         []ObservedChange     `json:"changes"`
}

// ObservationPolicy is caller-owned. Oldest is an inclusive acceptance cutoff for
// EACH input; no package-global TTL is imposed on this contract.
type ObservationPolicy struct {
	Now    time.Time
	Oldest time.Time
	Policy string
}
type ReviewLane struct {
	Name      string `json:"name"`
	Required  bool   `json:"required"`
	Satisfied bool   `json:"satisfied"`
}
type ObservationRow struct {
	Identity    ChangeIdentity `json:"identity"`
	AppliesTo   Applicability  `json:"appliesTo"`
	Title       string         `json:"title"`
	Action      string         `json:"action"`
	Disposition string         `json:"disposition"`
	Note        string         `json:"note"`
	Score       int            `json:"score"`
	Rank        int            `json:"rank"`
	Lanes       []ReviewLane   `json:"lanes"`
	Holds       []string       `json:"holds"`
}
type ObservationDepths struct {
	Cutoff       time.Time      `json:"cutoff"`
	Total        int            `json:"total"`
	Actions      map[string]int `json:"actions"`
	Dispositions map[string]int `json:"dispositions"`
}
type ObservationResult struct {
	Rows   []ObservationRow  `json:"rows"`
	Depths ObservationDepths `json:"depths"`
}
type Observation struct {
	Schema string            `json:"schema"`
	Inputs ObservationInputs `json:"inputs"`
	Result ObservationResult `json:"result"`
}

// ObservationPlan contains advisory proposals only. Effect boundaries must re-read
// current facts and enforce trust, claims, budgets, audit, and ready permissions.
type ObservationPlan struct {
	Result ObservationResult
	Verbs  []PlannedVerb
	Idle   IdleVerdict
}

var fullObjectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var providerID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var hostID = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
var repoID = regexp.MustCompile(`^[a-z0-9_.-]+(?:/[a-z0-9_.-]+)+$`)

func validRepository(r Repository) bool {
	if !providerID.MatchString(r.Provider) || !hostID.MatchString(r.Host) || !repoID.MatchString(r.Repository) {
		return false
	}
	for _, part := range strings.Split(r.Repository, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return !strings.Contains(r.Host, "..")
}
func inputError(kind ErrorKind, detail string) error { return &InputError{Kind: kind, Detail: detail} }
func (p ObservationPolicy) validate() error {
	if p.Now.IsZero() || p.Oldest.IsZero() || p.Oldest.After(p.Now) || strings.TrimSpace(p.Policy) == "" {
		return invalid("explicit now, oldest cutoff and policy revision are required", nil)
	}
	return nil
}
func validateProvenance(p InputProvenance, interval CollectionInterval, policy ObservationPolicy) error {
	if strings.TrimSpace(p.Source) == "" || strings.TrimSpace(p.Version) == "" || strings.TrimSpace(p.Revision) == "" || p.ObservedAt.IsZero() {
		return invalid("input provenance is missing source/version/revision/time", nil)
	}
	if p.ObservedAt.After(interval.End) {
		return invalid("input was observed after the collection ended", nil)
	}
	if p.Freshness != "fresh" && p.Freshness != "stale" && p.Freshness != "unknown" {
		return invalid("unknown freshness value", nil)
	}
	if !p.Complete {
		return inputError(PartialError, "input enumeration is incomplete")
	}
	if p.Freshness != "fresh" || p.ObservedAt.Before(policy.Oldest) {
		return inputError(StaleError, "input freshness does not satisfy the caller cutoff")
	}
	return nil
}
func validateObservationInputs(in ObservationInputs, policy ObservationPolicy) error {
	if err := policy.validate(); err != nil {
		return err
	}
	if in.Evaluator != ObservationEvaluator || in.Policy != policy.Policy || strings.TrimSpace(in.ReviewerDisplay) == "" {
		return invalid("unsupported evaluator or policy revision, or missing reviewer display", nil)
	}
	c := in.Collection
	if c.Start.IsZero() || c.End.IsZero() || c.Start.After(c.End) || c.End.After(policy.Now) {
		return invalid("invalid collection interval", nil)
	}
	if c.Start.Before(policy.Oldest) {
		return inputError(StaleError, "collection began before the caller cutoff")
	}
	if len(in.Requested) == 0 || len(in.Admitted) == 0 {
		return inputError(PartialError, "requested and admitted scope must be nonempty")
	}
	requested, admitted := map[Repository]bool{}, map[Repository]bool{}
	for _, r := range in.Requested {
		if !validRepository(r) || requested[r] {
			return invalid("noncanonical or duplicate requested repository", nil)
		}
		requested[r] = true
	}
	for _, r := range in.Admitted {
		if !validRepository(r) || admitted[r] || !requested[r] {
			return invalid("duplicate, noncanonical or unrequested admitted repository", nil)
		}
		admitted[r] = true
	}
	if len(requested) != len(admitted) {
		return inputError(PartialError, "requested scope was not fully admitted")
	}
	population := map[Repository]int{}
	for _, p := range in.Population {
		if !admitted[p.Repository] || p.Count < 0 {
			return invalid("invalid population coverage", nil)
		}
		if _, ok := population[p.Repository]; ok {
			return invalid("duplicate population coverage", nil)
		}
		if err := validateProvenance(p.Input, c, policy); err != nil {
			return err
		}
		population[p.Repository] = p.Count
	}
	if len(population) != len(admitted) {
		return inputError(PartialError, "missing population coverage")
	}
	counts := map[Repository]int{}
	seen := map[string]bool{}
	if in.Changes == nil {
		return inputError(PartialError, "missing change population; an empty array is required for measured zero")
	}
	for _, v := range in.Changes {
		id := v.Identity
		if !admitted[id.Repository] || id.Number < 1 || !fullObjectID.MatchString(id.Head) || (id.BaseUsed && !fullObjectID.MatchString(id.Base)) || (!id.BaseUsed && id.Base != "") {
			return invalid("invalid or out-of-scope change identity", nil)
		}
		key := fmt.Sprintf("%s#%d", id.Repository, id.Number)
		if seen[key] {
			return invalid("duplicate change in interval observation", nil)
		}
		seen[key] = true
		counts[id.Repository]++
		for _, p := range []InputProvenance{v.Inputs.Subject, v.Inputs.Body, v.Inputs.Reviews, v.Inputs.Checks, v.Inputs.Enrichment} {
			if err := validateProvenance(p, c, policy); err != nil {
				return err
			}
		}
		want := Applicability{Head: id.Head, Base: id.Base, Body: v.Inputs.Body.Revision, Reviews: v.Inputs.Reviews.Revision, Checks: v.Inputs.Checks.Revision}
		if v.AppliesTo != want {
			return inputError(StaleError, "classification applicability differs from the observed subject/body/reviews/checks")
		}
		if len(v.Lanes) != 2 {
			return invalid("both independent review lanes must be explicit", nil)
		}
		laneSeen := map[string]bool{}
		for _, l := range v.Lanes {
			if (l.Name != "correctness" && l.Name != "security") || laneSeen[l.Name] {
				return invalid("unknown or duplicate review lane", nil)
			}
			laneSeen[l.Name] = true
			switch l.State {
			case "approved", "blocked", "missing", "stale", "unknown":
			default:
				return invalid("invalid lane state", nil)
			}
			if l.AppliesTo != want {
				return inputError(StaleError, "independent lane applicability differs from observed inputs")
			}
			if l.Name == "security" && (l.State == "approved") != v.Facts.Reviews.SecurityPass {
				return invalid("security lane contradicts resolved security fact", nil)
			}
		}
		if v.Facts.Change.Labels == nil || v.Facts.Change.Checks == nil {
			return invalid("labels and checks require explicit arrays, including measured empty", nil)
		}
		switch v.Facts.ZeroCI {
		case "", ZeroCINeverRan, ZeroCIUnverified, ZeroCINoChecks:
		default:
			return invalid("unknown zero-CI resolution", nil)
		}
		rs := v.Facts.Reviews
		if (rs.AtHead != (rs.Ever && SameHead(rs.LastSHA, id.Head))) || (!rs.Ever && rs.LastSHA != "") || (rs.AtHead && !rs.Approved && !rs.Blocking) || (rs.Approved && (!rs.AtHead || rs.Blocking)) || (rs.Blocking && !rs.AtHead) || (rs.SuspectNoOp && !rs.Blocking) || (rs.BodyEditReverified && (!rs.Approved || rs.SuspectNoOp)) {
			return invalid("contradictory resolved review facts", nil)
		}
		if rs.Ever && !fullObjectID.MatchString(rs.LastSHA) {
			return invalid("resolved review lacks a full revision", nil)
		}
		if rs.ApprovedAt.After(v.Inputs.Reviews.ObservedAt) || rs.LastReviewAt.After(v.Inputs.Reviews.ObservedAt) {
			return invalid("review verdict postdates its observation", nil)
		}
	}
	for r, n := range population {
		if counts[r] != n {
			return inputError(PartialError, "population count does not match the full change set")
		}
	}
	return nil
}

// BuildObservation classifies the complete resolved input set using the same
// classifier and action table as deskboard and reviewloop. No provider is called.
func BuildObservation(in ObservationInputs, policy ObservationPolicy) (Observation, error) {
	if err := validateObservationInputs(in, policy); err != nil {
		return Observation{}, err
	}
	rows := make([]ObservationRow, 0, len(in.Changes))
	depths := ObservationDepths{Cutoff: in.Collection.End, Total: len(in.Changes), Actions: map[string]int{}, Dispositions: map[string]int{}}
	for _, v := range in.Changes {
		action, note := Classify(v.Facts.ClassifyInput(), in.ReviewerDisplay)
		note += BodyEditReverificationNote(v.Facts.Reviews.BodyEditReverified)
		rule, err := LookupAction(action)
		if err != nil {
			return Observation{}, err
		}
		holds := []string{}
		if rule.Verb == "" {
			holds = append(holds, rule.Why)
		}
		rows = append(rows, ObservationRow{Identity: v.Identity, AppliesTo: v.AppliesTo, Title: v.Facts.Change.Title, Action: action, Disposition: rule.Disposition.String(), Note: note, Score: v.Score,
			Lanes: []ReviewLane{{Name: "correctness", Required: true, Satisfied: laneApproved(v.Lanes, "correctness")}, {Name: "security", Required: v.Facts.RiskClassed, Satisfied: v.Facts.Reviews.SecurityPass}}, Holds: holds})
		depths.Actions[action]++
		depths.Dispositions[rule.Disposition.String()]++
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return observationRowID(rows[i]) < observationRowID(rows[j])
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return Observation{Schema: ObservationSchema, Inputs: in, Result: ObservationResult{Rows: rows, Depths: depths}}, nil
}
func laneApproved(lanes []LaneConclusion, name string) bool {
	for _, l := range lanes {
		if l.Name == name {
			return l.State == "approved"
		}
	}
	return false
}

// laneRequired reports whether the named lane is required on a row. A planned
// Row carries it as RiskClassed so observation plans match ReadBoard rows.
func laneRequired(lanes []ReviewLane, name string) bool {
	for _, l := range lanes {
		if l.Name == name {
			return l.Required
		}
	}
	return false
}

func observationRowID(r ObservationRow) string {
	return fmt.Sprintf("%s#%d", r.Identity.Repository, r.Identity.Number)
}

// PlanObservation validates untrusted supplied results by recomputing them. It
// cannot turn a partial/stale/schema error into an empty board or idle success.
func PlanObservation(o Observation, policy ObservationPolicy) (ObservationPlan, error) {
	if o.Schema != ObservationSchema {
		return ObservationPlan{}, invalid("unsupported review observation schema", nil)
	}
	recomputed, err := BuildObservation(o.Inputs, policy)
	if err != nil {
		return ObservationPlan{}, err
	}
	if !reflect.DeepEqual(o.Result, recomputed.Result) {
		return ObservationPlan{}, invalid("observation rows/depths contradict the full input population", nil)
	}
	b := &Board{AsOf: o.Inputs.Collection.End, Scope: &BoardScope{Count: len(o.Inputs.Admitted)}, Popn: &PRPopulation{Complete: true}}
	for _, r := range recomputed.Result.Rows {
		rule, _ := LookupAction(r.Action)
		b.Rows = append(b.Rows, Row{Provider: r.Identity.Repository.Provider, Host: r.Identity.Repository.Host, Repo: r.Identity.Repository.Repository, Number: r.Identity.Number, Head: r.Identity.Head, Title: r.Title, Action: r.Action, RiskClassed: laneRequired(r.Lanes, "security"), Score: r.Score, Rule: rule})
	}
	// Freshness and coverage were checked above against the explicit caller policy.
	// Share the legacy idle count decision without importing its cadence TTL.
	plan := assemblePlan(b, idleCounts(b))
	return ObservationPlan{Result: recomputed.Result, Verbs: plan.Verbs, Idle: plan.Idle}, nil
}
