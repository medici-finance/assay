package service

// IssueProjectionEnvelope carries already-resolved issueboard rows. Selection,
// trust/blessing, positive closed reads, label history and escalation retain their
// existing owners. Inventory alone cannot produce this complete envelope.
type IssueProjectionEnvelope struct {
	Schema      *string            `json:"schema"`
	Projection  *string            `json:"projection"`
	To          *string            `json:"to"`
	Requested   []ReviewRepository `json:"requested"`
	Admitted    []ReviewRepository `json:"admitted"`
	Collection  ReviewCollection   `json:"collection"`
	Inputs      []ReviewPopulation `json:"inputs"`
	Rows        []IssueRow         `json:"rows"`
	Quarantined []QuarantinedIssue `json:"quarantined"`
}

type IssueRow struct {
	Repository             ReviewRepository `json:"repository"`
	Number                 *int             `json:"number"`
	Title                  *string          `json:"title"`
	Action                 *string          `json:"action"`
	AgeDays                *int             `json:"ageDays"`
	AddrRole               *string          `json:"addrRole"`
	EscalationUnverifiable *bool            `json:"escalationUnverifiable"`
	HistoryHold            *string          `json:"historyHold"`
	HistoryUnverifiable    *bool            `json:"historyUnverifiable"`
}

type QuarantinedIssue struct {
	Repository ReviewRepository `json:"repository"`
	Number     *int             `json:"number"`
	Title      *string          `json:"title"`
	Author     *string          `json:"author"`
}
