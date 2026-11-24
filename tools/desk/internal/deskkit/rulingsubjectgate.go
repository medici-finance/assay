package deskkit

// rulingsubjectgate.go — the desk-tools half of the ruling-subject bridge
// (design point 5 of the ruling-subject bridge).
//
// RulingSubjectGate is pure: the caller fetches the ruling comment and the current
// decision issue and passes what it read. The gate never fetches, never accepts
// another module's approval in place of these observations, and can only refuse:
// a bound result grants no permission, claim, dispatch, status move or acceptance.
// It checks the comment's issue, edit timestamps, actor type and human mapping
// itself, then the issue's marker, and applies the shared subject, card and cutover
// rules from statusgen/workref (BindRulingSubject) to its own card read.

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/statusgen/workref"
)

// Distinct refusal reasons of the ruling-subject gate.
const (
	RulingSubjectIssue   = "issue"            // the comment is not on the expected decision issue, or the issue does not identify the brief
	RulingSubjectEdited  = "edited"           // the comment was edited, or its edit state is unknown
	RulingSubjectActor   = "actor"            // the author is not a personal account mapped as a human
	RulingSubjectSubject = "subject"          // the ruled subject is not the presented brief and revision
	RulingSubjectCard    = "card-unavailable" // the card the human saw is not established
)

// RulingCommentObservation is the fetched ruling comment as the forge reported it.
type RulingCommentObservation struct {
	Repo      string // repository the comment was read from (owner/name)
	Issue     int    // issue the comment is on
	ID        int64  // the forge's numeric comment id
	Author    Account
	Body      string
	CreatedAt string // forge creation time (RFC3339); "" = not reported
	UpdatedAt string // forge last-update time (RFC3339); "" = not reported
}

// RulingIssueObservation is the current decision issue as the forge reported it.
type RulingIssueObservation struct {
	Repo   string
	Number int
	Body   string
}

// RulingSubjectInput is everything the gate judges. Comment or Issue nil means
// that read failed.
type RulingSubjectInput struct {
	Comment       *RulingCommentObservation
	Issue         *RulingIssueObservation
	ExpectedRepo  string            // the repository whose decision issue the brief records
	ExpectedIssue int               // the brief's decision-issue number
	ExpectedID    int64             // the ruling link's comment id (0 = not named)
	Humans        map[string]string // configured human name → login
	// The judged brief: its declared identity (brief: value or board id), its
	// explicit version (0 = none; never defaulted), and the resolved context and
	// registry at the judged base.
	Declared  string
	Version   uint64
	Declaring *workref.Context
	Registry  workref.Registry
}

// RulingSubjectResult is three-state. Reason is one of the RulingSubject*
// constants when State is not Bound; Detail names the failed condition.
type RulingSubjectResult struct {
	State  workref.BindingState
	Reason string
	Detail string
	Legacy bool // a historical, pre-cutover ruling with no subject line
}

var (
	rulingGateMarkerRe = regexp.MustCompile(`<!--\s*decision-gate:\s*(\S+?)\s*-->`)
	rulingCardPrefix   = "<!-- assay-card: "
)

// RulingSubjectGate judges one ruling for one brief subject.
func RulingSubjectGate(in RulingSubjectInput) RulingSubjectResult {
	res := func(state workref.BindingState, reason, format string, a ...any) RulingSubjectResult {
		return RulingSubjectResult{State: state, Reason: reason, Detail: fmt.Sprintf(format, a...)}
	}
	c, iss := in.Comment, in.Issue
	if c == nil {
		return res(workref.CouldNotCheck, RulingSubjectIssue, "the ruling comment could not be read")
	}
	if iss == nil {
		return res(workref.CouldNotCheck, RulingSubjectIssue, "the decision issue could not be read")
	}
	// Issue: the comment and the issue are the brief's recorded decision issue.
	if in.ExpectedIssue <= 0 || in.ExpectedRepo == "" {
		return res(workref.Refused, RulingSubjectIssue, "the brief records no decision issue")
	}
	if !strings.EqualFold(c.Repo, in.ExpectedRepo) || c.Issue != in.ExpectedIssue {
		return res(workref.Refused, RulingSubjectIssue, "the ruling comment is on %s#%d, not on the brief's decision issue %s#%d", c.Repo, c.Issue, in.ExpectedRepo, in.ExpectedIssue)
	}
	if in.ExpectedID != 0 && c.ID != in.ExpectedID {
		return res(workref.Refused, RulingSubjectIssue, "the comment read is %d, not the linked ruling comment %d", c.ID, in.ExpectedID)
	}
	if !strings.EqualFold(iss.Repo, in.ExpectedRepo) || iss.Number != in.ExpectedIssue {
		return res(workref.Refused, RulingSubjectIssue, "the issue read is %s#%d, not the brief's decision issue %s#%d", iss.Repo, iss.Number, in.ExpectedRepo, in.ExpectedIssue)
	}
	// Edited: an edited comment no longer stands; an unknown edit state holds.
	if c.CreatedAt == "" || c.UpdatedAt == "" {
		return res(workref.CouldNotCheck, RulingSubjectEdited, "the forge did not report the ruling comment's creation and update times")
	}
	if c.CreatedAt != c.UpdatedAt {
		return res(workref.Refused, RulingSubjectEdited, "the ruling comment was edited (created %s, updated %s)", c.CreatedAt, c.UpdatedAt)
	}
	// Actor: a personal account mapped as a human.
	switch {
	case c.Author.Type == "":
		return res(workref.CouldNotCheck, RulingSubjectActor, "the forge did not report the ruling author's account type")
	case c.Author.Type != "User" || strings.HasSuffix(strings.ToLower(c.Author.Login), "[bot]"):
		return res(workref.Refused, RulingSubjectActor, "the ruling author is a %s account, not a person", c.Author.Type)
	}
	mapped := false
	for _, login := range in.Humans {
		if login != "" && strings.EqualFold(login, c.Author.Login) {
			mapped = true
			break
		}
	}
	if !mapped {
		return res(workref.Refused, RulingSubjectActor, "the ruling author is not mapped as a human")
	}
	// The presented subject: the judged brief's resolved key and explicit version.
	presented, err := workref.BriefSubject(in.Declared, in.Version, in.Declaring, in.Registry)
	if err != nil {
		return res(workref.CouldNotCheck, RulingSubjectSubject, "the judged brief's identity %q does not resolve: %v", in.Declared, err)
	}
	// The issue still identifies the record: some marker resolves to the same key.
	body := strings.ReplaceAll(iss.Body, "\r\n", "\n")
	if !rulingMarkerNames(body, presented.LogicalKey, in.Declaring, in.Registry) {
		return res(workref.Refused, RulingSubjectIssue, "the decision issue carries no decision-gate marker that resolves to %s", presented.LogicalKey)
	}
	ev := workref.RulingEvidence{Comment: c.Body, CreatedAt: c.CreatedAt, Card: readRulingCard(body)}
	b := workref.BindRulingSubject(ev, presented, in.Declaring, in.Registry)
	switch {
	case b.State == workref.Bound:
		return RulingSubjectResult{State: workref.Bound, Detail: b.Detail, Legacy: b.Legacy}
	case b.Reason == workref.ReasonCard:
		return RulingSubjectResult{State: b.State, Reason: RulingSubjectCard, Detail: b.Detail}
	default:
		return RulingSubjectResult{State: b.State, Reason: RulingSubjectSubject, Detail: b.Reason + ": " + b.Detail}
	}
}

func rulingMarkerNames(body, key string, declaring *workref.Context, registry workref.Registry) bool {
	for _, m := range rulingGateMarkerRe.FindAllStringSubmatch(body, -1) {
		r, err := workref.Resolve(strings.TrimSpace(m[1]), declaring, registry, workref.TypedBrief)
		if err == nil && r.LogicalKey == key {
			return true
		}
	}
	return false
}

// readRulingCard is this module's own read of the typed card on a decision
// issue: exactly one card line outside fenced code, ending in " -->", carrying
// a decision card's brief and positive version. Anything else present is an error.
func readRulingCard(body string) workref.CardObservation {
	var lines []string
	count := 0
	fenced := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		count += strings.Count(line, rulingCardPrefix)
		if strings.HasPrefix(line, rulingCardPrefix) {
			lines = append(lines, line)
		}
	}
	switch {
	case count == 0:
		return workref.CardObservation{}
	case len(lines) != 1 || count != 1:
		return workref.CardObservation{Present: true, Err: fmt.Errorf("ambiguous card")}
	case !strings.HasSuffix(lines[0], " -->"):
		return workref.CardObservation{Present: true, Err: fmt.Errorf("malformed card")}
	}
	var card struct {
		Schema  string `json:"schema"`
		Purpose string `json:"purpose"`
		Brief   string `json:"brief"`
		Version int64  `json:"version"`
	}
	dec := json.NewDecoder(strings.NewReader(strings.TrimSuffix(strings.TrimPrefix(lines[0], rulingCardPrefix), " -->")))
	if err := dec.Decode(&card); err != nil {
		return workref.CardObservation{Present: true, Err: fmt.Errorf("malformed card: %v", err)}
	}
	if dec.Decode(new(any)) != io.EOF {
		return workref.CardObservation{Present: true, Err: fmt.Errorf("trailing card data")}
	}
	if card.Schema != "gate-card/v1" || card.Purpose != "decision" || card.Brief == "" || card.Version < 1 {
		return workref.CardObservation{Present: true, Err: fmt.Errorf("invalid decision card")}
	}
	return workref.CardObservation{Present: true, Block: lines[0], Brief: card.Brief, Version: uint64(card.Version)}
}
