package service

import "time"

// These are transport DTOs for the existing review-observation-v1 schema at the
// pinned producer revision. Scalar pointers retain explicit zero/false/empty
// versus absent/null. Classification and PlanObservation remain with reviewflow;
// this package never computes or grants their conclusions.

type ReviewRepository struct {
	Provider   *string `json:"provider"`
	Host       *string `json:"host"`
	Repository *string `json:"repository"`
}

type ReviewIdentity struct {
	Repository ReviewRepository `json:"repository"`
	Number     *int             `json:"number"`
	Head       *string          `json:"head"`
	Base       *string          `json:"base"`
	BaseUsed   *bool            `json:"baseUsed"`
}

type ReviewCollection struct {
	Start *time.Time `json:"start"`
	End   *time.Time `json:"end"`
}

type ReviewProvenance struct {
	Source     *string    `json:"source"`
	Version    *string    `json:"version"`
	Revision   *string    `json:"revision"`
	ObservedAt *time.Time `json:"observedAt"`
	Freshness  *string    `json:"freshness"`
	Complete   *bool      `json:"complete"`
}

type ReviewPopulation struct {
	Repository ReviewRepository `json:"repository"`
	Input      ReviewProvenance `json:"input"`
	Count      *int             `json:"count"`
}

type ReviewApplicability struct {
	Head    *string `json:"head"`
	Base    *string `json:"base"`
	Body    *string `json:"body"`
	Reviews *string `json:"reviews"`
	Checks  *string `json:"checks"`
}

type ReviewInputSet struct {
	Subject    ReviewProvenance `json:"subject"`
	Body       ReviewProvenance `json:"body"`
	Reviews    ReviewProvenance `json:"reviews"`
	Checks     ReviewProvenance `json:"checks"`
	Enrichment ReviewProvenance `json:"enrichment"`
}

type ReviewFacts struct {
	Change                  ReviewChangeFact `json:"change"`
	Reviews                 ResolvedReview   `json:"reviews"`
	CIRequired              *bool            `json:"ciRequired"`
	ZeroCI                  *string          `json:"zeroCI"`
	OwnFilesChanged         *bool            `json:"ownFilesChanged"`
	RiskClassed             *bool            `json:"riskClassed"`
	RiskReason              *string          `json:"riskReason"`
	NonCommitResolution     *bool            `json:"nonCommitResolution"`
	NonCommitResolutionNote *string          `json:"nonCommitResolutionNote"`
}

type ReviewLaneConclusion struct {
	Name      *string             `json:"name"`
	State     *string             `json:"state"`
	AppliesTo ReviewApplicability `json:"appliesTo"`
}

type ReviewChange struct {
	Lanes     []ReviewLaneConclusion `json:"lanes"`
	Identity  ReviewIdentity         `json:"identity"`
	Inputs    ReviewInputSet         `json:"inputs"`
	AppliesTo ReviewApplicability    `json:"appliesTo"`
	Facts     ReviewFacts            `json:"facts"`
	Score     *int                   `json:"score"`
}

type ReviewInputs struct {
	Evaluator       *string            `json:"evaluator"`
	Policy          *string            `json:"policy"`
	ReviewerDisplay *string            `json:"reviewerDisplay"`
	Requested       []ReviewRepository `json:"requested"`
	Admitted        []ReviewRepository `json:"admitted"`
	Collection      ReviewCollection   `json:"collection"`
	Population      []ReviewPopulation `json:"population"`
	Changes         []ReviewChange     `json:"changes"`
}

type ReviewLane struct {
	Name      *string `json:"name"`
	Required  *bool   `json:"required"`
	Satisfied *bool   `json:"satisfied"`
}

type ReviewRow struct {
	Identity    ReviewIdentity      `json:"identity"`
	AppliesTo   ReviewApplicability `json:"appliesTo"`
	Title       *string             `json:"title"`
	Action      *string             `json:"action"`
	Disposition *string             `json:"disposition"`
	Note        *string             `json:"note"`
	Score       *int                `json:"score"`
	Rank        *int                `json:"rank"`
	Lanes       []ReviewLane        `json:"lanes"`
	Holds       []*string           `json:"holds"`
}

type ReviewDepths struct {
	Cutoff       *time.Time      `json:"cutoff"`
	Total        *int            `json:"total"`
	Actions      map[string]*int `json:"actions"`
	Dispositions map[string]*int `json:"dispositions"`
}

type ReviewResult struct {
	Rows   []ReviewRow  `json:"rows"`
	Depths ReviewDepths `json:"depths"`
}

type ReviewEnvelope struct {
	Schema *string      `json:"schema"`
	Inputs ReviewInputs `json:"inputs"`
	Result ReviewResult `json:"result"`
}

type ReviewChangeFact struct {
	Title, Body, MergeStateStatus *string
	IsDraft                       *bool
	Labels                        []*string
	Checks                        []ReviewCheckFact
}

type ResolvedReview struct {
	Ever         *bool
	AtHead       *bool
	Blocking     *bool
	Approved     *bool
	LastSHA      *string
	SecurityPass *bool
	ApprovedAt   *time.Time

	LastReviewAt *time.Time

	SuspectNoOp *bool

	ExternalPrereqDeclared *bool

	BodyEditReverified *bool
}

type ReviewCheckFact struct{ Status, Conclusion, State *string }
