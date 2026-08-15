package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// HeadUnresolved is the sentinel head SHA for a board row whose head could not be read.
// It is NEVER the empty string: an empty head silently satisfies string comparisons and
// would let a coalescing key or an AlreadyDone probe look answered when it was not.
const HeadUnresolved = "UNRESOLVED"

// ---------------------------------------------------------------------------
// The consumer-side view of deskboard's JSON.
// ---------------------------------------------------------------------------
//
// deskboard's report structs are unexported in package main under cmd/deskboard, so they
// cannot be imported. These are the consumer-side mirror of the fields this reactor reads
// — deliberately a SUBSET, and every field left out is named in surfaces.go rather than
// dropped silently.

// BoardScope mirrors deskboard's in-band coverage statement (scope.go:61).
type BoardScope struct {
	Repos  []string `json:"repos"`
	Count  int      `json:"count"`
	Source string   `json:"source"`
}

// PRPopulation mirrors deskboard's truncation statement (board.go:256). ABSENT means the
// verb read no PR list — never "the population was complete".
type PRPopulation struct {
	Complete       bool     `json:"complete"`
	Cap            int      `json:"cap"`
	TruncatedRepos []string `json:"truncatedRepos,omitempty"`
}

// ActionRow mirrors the fields of deskboard's actionRow (board.go:1291) this reactor
// reads. NOTE the absence of a head SHA: `deskboard actions` does not emit one.
type ActionRow struct {
	Repo        string `json:"repo"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Action      string `json:"action"`
	Draft       bool   `json:"draft"`
	RiskClassed bool   `json:"riskClassed"`
	Score       int    `json:"score"`
	OwningBrief string `json:"owningBrief"`
	Note        string `json:"note"`
}

// ActionsReport mirrors deskboard's actionsReport (board.go:1337) plus the header fields
// the idle gate rests on.
type ActionsReport struct {
	AsOf         string        `json:"asOf"`
	Stale        bool          `json:"stale"`
	StaleState   string        `json:"staleState,omitempty"`
	StaleDetail  string        `json:"staleDetail,omitempty"`
	Scope        *BoardScope   `json:"scope,omitempty"`
	PRPopulation *PRPopulation `json:"prPopulation,omitempty"`
	Rows         []ActionRow   `json:"rows"`
}

// PRRow mirrors deskboard's prRow (board.go:1147). It is read for ONE field the actions
// verb does not carry: the head SHA.
type PRRow struct {
	Repo    string `json:"repo"`
	Number  int    `json:"number"`
	HeadSHA string `json:"headSHA"`
}

// PRsReport mirrors deskboard's prsReport (board.go:1214).
type PRsReport struct {
	Rows []PRRow `json:"prs"`
}

// ---------------------------------------------------------------------------
// Board — the joined, three-state read
// ---------------------------------------------------------------------------

// Board is one sweep, joined across the two deskboard verbs it takes to see both halves
// of a row.
//
// THE JOIN IS NOT AN IMPLEMENTATION DETAIL, it is a measured gap. `deskboard actions`
// carries the ACTION and no head SHA (board.go:1291); `deskboard prs` carries the head
// SHA and no review state, and the pr-review-desk skill forbids substituting it for the
// classified sweep for exactly that reason. Neither verb alone can produce the
// (repo, pr, head, verb) idempotency key every outward write is keyed on. So the reactor
// reads BOTH and joins on (repo, number) — and a row present in `actions` but absent from
// `prs` gets HeadUnresolved, which is could-not-check for its outward verb, never a
// silently-empty key.
type Board struct {
	AsOf   time.Time
	Rows   []Row
	Scope  *BoardScope
	Popn   *PRPopulation
	Stale  bool
	Detail string
}

// Row is one board row with its head joined in and its disposition resolved.
type Row struct {
	Repo        string
	Number      int
	Title       string
	Action      string
	Head        string // HeadUnresolved when `prs` carried no matching row
	RiskClassed bool
	Score       int
	Rule        rule
}

// ID is the stable per-PR identity used in output and in coalescing keys.
func (r Row) ID() string { return fmt.Sprintf("%s#%d", r.Repo, r.Number) }

// ReadBoard decodes the two deskboard payloads and joins them. It is the reactor's ONLY
// entry to board state, and it is three-state by construction: it returns either a Board
// it could positively read, or a deskkit error carrying the exit code. There is no
// "empty board" success path — every way of failing to read lands on Unverifiable (6).
//
// The empty-result trap this closes: a secondary-rate-limit 403 on the shared token makes
// `deskboard actions` exit 6 with no rows, and ~16+ concurrent gh-hitting agents produce
// exactly that. A reader that decoded an empty rows array into "0 actionable" would report
// all-clear from an outage. Here an empty payload cannot become a Board at all.
func ReadBoard(actionsJSON, prsJSON []byte) (*Board, error) {
	if len(strings.TrimSpace(string(actionsJSON))) == 0 {
		return nil, deskkit.Unverifiable(
			"reviewloop: `deskboard actions` produced no output — BLIND, not idle "+
				"(a rate-limited or refused sweep is empty for the same reason a quiet board is)", nil)
	}
	var ar ActionsReport
	if err := json.Unmarshal(actionsJSON, &ar); err != nil {
		return nil, deskkit.Unverifiable("reviewloop: cannot decode `deskboard actions` JSON — BLIND, not idle", err)
	}
	if strings.TrimSpace(ar.AsOf) == "" {
		return nil, deskkit.Unverifiable(
			"reviewloop: `deskboard actions` payload carries no asOf timestamp — the sweep's own liveness "+
				"heartbeat is missing, so its freshness cannot be checked; BLIND, not idle", nil)
	}
	asOf, err := time.Parse(time.RFC3339, ar.AsOf)
	if err != nil {
		return nil, deskkit.Unverifiable("reviewloop: cannot parse the board's asOf timestamp "+ar.AsOf, err)
	}

	heads := map[string]string{}
	if len(strings.TrimSpace(string(prsJSON))) > 0 {
		var pr PRsReport
		if err := json.Unmarshal(prsJSON, &pr); err != nil {
			return nil, deskkit.Unverifiable("reviewloop: cannot decode `deskboard prs` JSON — the head-SHA half of the join is BLIND", err)
		}
		for _, p := range pr.Rows {
			if strings.TrimSpace(p.HeadSHA) != "" {
				heads[fmt.Sprintf("%s#%d", p.Repo, p.Number)] = p.HeadSHA
			}
		}
	}

	b := &Board{
		AsOf:   asOf,
		Scope:  ar.Scope,
		Popn:   ar.PRPopulation,
		Stale:  ar.Stale,
		Detail: strings.TrimSpace(ar.StaleState + " " + ar.StaleDetail),
	}
	for _, a := range ar.Rows {
		ru, err := LookupAction(a.Action)
		if err != nil {
			return nil, err // an unknown ACTION poisons the whole read: fail closed
		}
		head := HeadUnresolved
		if h, ok := heads[fmt.Sprintf("%s#%d", a.Repo, a.Number)]; ok {
			head = h
		}
		b.Rows = append(b.Rows, Row{
			Repo: a.Repo, Number: a.Number, Title: a.Title, Action: a.Action,
			Head: head, RiskClassed: a.RiskClassed, Score: a.Score, Rule: ru,
		})
	}
	sort.SliceStable(b.Rows, func(i, j int) bool {
		if b.Rows[i].Score != b.Rows[j].Score {
			return b.Rows[i].Score > b.Rows[j].Score
		}
		return b.Rows[i].ID() < b.Rows[j].ID()
	})
	return b, nil
}

// CountByDisposition tallies the board by disposition, for the plan header.
func (b *Board) CountByDisposition() map[Disposition]int {
	out := map[Disposition]int{}
	for _, r := range b.Rows {
		out[r.Rule.Disposition]++
	}
	return out
}

// CountAction tallies rows carrying one ACTION.
func (b *Board) CountAction(action string) int {
	n := 0
	for _, r := range b.Rows {
		if r.Action == action {
			n++
		}
	}
	return n
}

// UnresolvedHeads lists rows whose head could not be joined. They are named in the plan
// output; an outward verb is refused for each.
func (b *Board) UnresolvedHeads() []string {
	var out []string
	for _, r := range b.Rows {
		if r.Head == HeadUnresolved {
			out = append(out, r.ID())
		}
	}
	return out
}

// RenderRows writes one line per board row — including every WAIT and NO-OP row. The
// waiting states are printed on purpose: a row that vanishes from the reactor's output
// because there was nothing to do about it is indistinguishable, downstream, from a row
// that was never on the board.
func (b *Board) RenderRows(w io.Writer) {
	for _, r := range b.Rows {
		fmt.Fprintf(w, "%-10s %-24s %-9s head=%-12s score=%-4d %s\n",
			r.ID(), r.Action, r.Rule.Disposition, short(r.Head), r.Score, r.Rule.Why)
	}
}

func short(sha string) string {
	if sha == HeadUnresolved {
		return sha
	}
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
