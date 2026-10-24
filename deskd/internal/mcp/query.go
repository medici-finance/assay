package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// registry is the complete §7.1 tool table. It is a function, not a var, so the
// handlers close over nothing and the authority model can enumerate a fresh
// copy. The two deliberate absences (desk__answer, desk__adopt) are absent by
// simply not being here — and the authority test asserts they stay absent.
func registry() []Tool {
	return []Tool{
		{
			Name:        "desk__escalate",
			Description: "File an escalation (kind, situation, options, urgency, blocks-what). Returns immediately; the loop continues. Options are formatted into the on-issue v2 grammar and validated at filing; provenance is derived from caller identity, never the arguments.",
			Mutates:     true,
			AuthorsAs:   AppIdentity,
			InputSchema: escalateSchema(),
			Handler:     handleEscalate,
		},
		{
			Name:        "desk__report_file",
			Description: "The analysis automation files a classified report (good/bad/ugly, evidence links, suggested cluster) — the §5.3 signal intake, same shape as desk__escalate.",
			Mutates:     true,
			AuthorsAs:   AppIdentity,
			Handler:     handleReportFile,
		},
		{
			Name:        "desk__claim",
			Description: "Claim a work item under a TTL lease (§6.6). Renew before expiry or it releases on its own. Lease state is deskd-local; claiming coordinates the fleet and writes nothing on the platform.",
			Mutates:     false,
			Handler:     handleClaim,
		},
		{
			Name:        "desk__inbox_list",
			Description: "Read open escalations for a cell (filter: cell, kind, urgency). Each carries provenance and its parsed options.",
			Mutates:     false,
			Handler:     handleInboxList,
		},
		{
			Name:        "desk__overrides",
			Description: "Read a cell's operator override table (dispatch_width, hold_merges, pause_verify, model_floor, quiet_hours) with as_of — the knobs the Drive tab sets and the loops read each tick. Read-only; authors nothing.",
			Mutates:     false,
			InputSchema: overridesSchema(),
			Handler:     handleOverrides,
		},
		{
			Name:        "desk__item_get",
			Description: "Read one escalation by its domain id, with full parsed options and per-field confidence.",
			Mutates:     false,
			Handler:     handleItemGet,
		},
		{
			Name:        "desk__status",
			Description: "What this cell looks like right now: open-escalation count, unresolved rows, urgency mix, index age, and per-repo health.",
			Mutates:     false,
			Handler:     handleStatus,
		},
		{
			Name:        "desk__metrics_query",
			Description: "Deterministic index queries (counts, ages) at epic|program|portfolio|team scope — the substrate for Ask Assay's numbers (§5.2, §7.5).",
			Mutates:     false,
			Handler:     handleMetricsQuery,
		},
		{
			Name:        "desk__roadmap",
			Description: "Epics, in-flight work, blocked counts, Next-up. Delegated to statusgen; not ingested in this build.",
			Mutates:     false,
			Scope:       ScopeNone,
			Handler:     delegatedRead("desk__roadmap", "statusgen --json over each cell's streams_root"),
		},
		{
			Name:        "desk__portfolio",
			Description: "Every program: flow counts, waiting-on-human, last-record age, model routing. Delegated to statusgen; not ingested in this build.",
			Mutates:     false,
			Scope:       ScopeNone,
			Handler:     delegatedRead("desk__portfolio", "statusgen --json / --dora across programs"),
		},
		{
			Name:        "desk__team_view",
			Description: "The Teams pane as data (§3.1): per-team queue/WIP, gates by owning team, handoff waits. The owning-team dimension is not ingested in this build.",
			Mutates:     false,
			Scope:       ScopeNone,
			Handler:     delegatedRead("desk__team_view", "per-gate owning-team (not ingested — no team dimension on the escalation index yet)"),
		},
		{
			Name:        "desk__desks",
			Description: "The desk registry for a cell (the service contract): one row per configured desk (group: master/desk/custom/peer) joining cells.yaml loops, App identity and live terminal-registry state — the parity twin of both GET /v1/desks and GET /v1/desks/{id}/terminal. A desk with no attachable target, or an unreachable one, is a could-not-check row, never omitted.",
			Mutates:     false,
			InputSchema: desksSchema(),
			Handler:     handleDesks,
		},
		{
			Name:        "desk__improve_list",
			Description: "Read reports, clusters, open proposals, and RetroActions with their before/after — the sharpen-the-saw read surface (§5.3).",
			Mutates:     false,
			Scope:       ScopeNone,
			Handler:     delegatedRead("desk__improve_list", "the §5.3 report/cluster/proposal store (arrives with the Improve pane)"),
		},
	}
}

func escalateSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"cell", "kind", "title", "situation"},
		"properties": map[string]any{
			"cell":      map[string]any{"type": "string"},
			"repo":      map[string]any{"type": "string", "description": "owner/name; defaults to the cell's first repo"},
			"kind":      map[string]any{"type": "string", "enum": []string{"needs-decision", "question", "help-wanted", "ask"}},
			"title":     map[string]any{"type": "string"},
			"situation": map[string]any{"type": "string"},
			"urgency":   map[string]any{"type": "string", "enum": []string{"routine", "elevated", "urgent", "gold"}},
			"options": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"label", "consequence"},
					"properties": map[string]any{
						"id":          map[string]any{"type": "string", "description": "slug [a-z0-9][a-z0-9-]{1,31}; omit to get option-1..n"},
						"label":       map[string]any{"type": "string"},
						"consequence": map[string]any{"type": "string"},
					},
				},
			},
			"blocks": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
}

// defaultRepo returns a cell's first configured repo, or "" if unknown.
func (s *Server) defaultRepo(cell string) string {
	if s.Config == nil {
		return ""
	}
	for _, c := range s.Config.Cells {
		if c.Name == cell && len(c.Repos) > 0 {
			return c.Repos[0]
		}
	}
	return ""
}

// ---- read tools over the index --------------------------------------------

// InboxListArgs filters the inbox read.
type InboxListArgs struct {
	Cell    string `json:"cell"`
	Kind    string `json:"kind"`
	Urgency string `json:"urgency"`
}

func handleInboxList(_ context.Context, s *Server, call Call) (any, error) {
	var a InboxListArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__inbox_list arguments did not decode: %w", err)
	}
	rows, err := s.inbox(a.Cell)
	if err != nil {
		return nil, err
	}
	var out []index.Row
	for _, r := range rows {
		if a.Kind != "" && string(r.Kind) != strings.ToLower(a.Kind) {
			continue
		}
		if a.Urgency != "" && r.Urgency.String() != strings.ToLower(a.Urgency) {
			continue
		}
		out = append(out, r)
	}
	if out == nil {
		out = []index.Row{}
	}
	return map[string]any{
		"cell":             a.Cell,
		"items":            out,
		"count":            len(out),
		"sources_not_read": inboxSourcesNotRead,
	}, nil
}

// ItemGetArgs names one escalation.
type ItemGetArgs struct {
	Cell string `json:"cell"`
	ID   string `json:"id"`
}

func handleItemGet(_ context.Context, s *Server, call Call) (any, error) {
	var a ItemGetArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__item_get arguments did not decode: %w", err)
	}
	if a.ID == "" {
		return nil, fmt.Errorf("desk__item_get: id is required")
	}
	rows, err := s.inbox(a.Cell)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if string(r.ID) == a.ID {
			return r, nil
		}
	}
	return nil, fmt.Errorf("desk__item_get: no open escalation %q in cell %q (it may be closed, or in a repo that could not be read)", a.ID, a.Cell)
}

// StatusResult is desk__status's cell snapshot.
type StatusResult struct {
	Cell            string               `json:"cell"`
	Open            int                  `json:"open_escalations"`
	Unresolved      int                  `json:"unresolved_rows"`
	ByUrgency       map[string]int       `json:"by_urgency"`
	IndexAge        domain.Assessment    `json:"index_age"`
	IndexAgeSeconds *int64               `json:"index_age_seconds"`
	RepoHealth      []adapter.RepoHealth `json:"repo_health"`
	SourcesNotRead  []string             `json:"sources_not_read"`
}

func handleStatus(_ context.Context, s *Server, call Call) (any, error) {
	var a struct {
		Cell string `json:"cell"`
	}
	_ = json.Unmarshal(nonNull(call.Args), &a)
	rows, err := s.inbox(a.Cell)
	if err != nil {
		return nil, err
	}
	byUrg := map[string]int{}
	unresolved := 0
	for _, r := range rows {
		byUrg[r.Urgency.String()]++
		if len(r.Fields.Unresolved()) > 0 {
			unresolved++
		}
	}
	res := StatusResult{
		Cell: a.Cell, Open: len(rows), Unresolved: unresolved, ByUrgency: byUrg,
		SourcesNotRead: inboxSourcesNotRead,
	}
	if s.DB != nil {
		age, assessment := s.DB.IndexAge(s.Now())
		res.IndexAge = assessment
		if assessment.Confidence != domain.CouldNotCheck {
			secs := int64(age.Seconds())
			res.IndexAgeSeconds = &secs
		}
		if h, herr := s.DB.Health(); herr == nil {
			// Only the authorized cell's repositories: the health table is
			// index-wide, and another cell's repo list is not this caller's.
			own := cellRepos(s.findCell(a.Cell))
			res.RepoHealth = []adapter.RepoHealth{}
			for _, rh := range h {
				if own[strings.ToLower(rh.Repo)] {
					res.RepoHealth = append(res.RepoHealth, rh)
				}
			}
		}
	}
	return res, nil
}

// MetricsArgs is the desk__metrics_query argument shape.
type MetricsArgs struct {
	Scope  string `json:"scope"`  // epic|program|portfolio|team
	Metric string `json:"metric"` // open_escalations|unresolved_rows|oldest_age_seconds|by_urgency
	Cell   string `json:"cell"`
}

var metricScopes = map[string]bool{"epic": true, "program": true, "portfolio": true, "team": true}

// MetricsResult carries the deterministic index value plus an honest note on
// the hierarchy scoping this build cannot yet perform.
type MetricsResult struct {
	Scope           string            `json:"scope"`
	Metric          string            `json:"metric"`
	Value           any               `json:"value"`
	Assessment      domain.Assessment `json:"assessment"`
	ScopeAssessment domain.Assessment `json:"scope_assessment"`
}

func handleMetricsQuery(_ context.Context, s *Server, call Call) (any, error) {
	var a MetricsArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__metrics_query arguments did not decode: %w", err)
	}
	a.Scope = strings.ToLower(strings.TrimSpace(a.Scope))
	if !metricScopes[a.Scope] {
		return nil, fmt.Errorf("desk__metrics_query: scope %q is not one of epic|program|portfolio|team (§5.2)", a.Scope)
	}
	metric := strings.ToLower(strings.TrimSpace(a.Metric))
	if metric == "" {
		metric = "open_escalations"
	}
	rows, err := s.inbox(a.Cell)
	if err != nil {
		return nil, err
	}

	probe := domain.Probe{
		Source: "derived_escalations (SQLite) via desk__metrics_query — the index only",
		Method: domain.MethodLocalTable,
		Window: "open escalations of cell " + orAll(a.Cell),
		Limit:  0,
	}
	res := MetricsResult{Scope: a.Scope, Metric: metric, Assessment: domain.Clean(probe)}

	switch metric {
	case "open_escalations":
		res.Value = len(rows)
	case "unresolved_rows":
		n := 0
		for _, r := range rows {
			if len(r.Fields.Unresolved()) > 0 {
				n++
			}
		}
		res.Value = n
	case "oldest_age_seconds":
		var oldest int64
		for _, r := range rows {
			if r.AgeSeconds > oldest {
				oldest = r.AgeSeconds
			}
		}
		res.Value = oldest
	case "by_urgency":
		m := map[string]int{}
		for _, r := range rows {
			m[r.Urgency.String()]++
		}
		res.Value = m
	default:
		return nil, fmt.Errorf("desk__metrics_query: metric %q is not index-native "+
			"(known: open_escalations, unresolved_rows, oldest_age_seconds, by_urgency); "+
			"DORA percentiles are delegated to statusgen and not ingested in this build", metric)
	}

	// The number is a CELL-WIDE index count. The epic/program/portfolio/team
	// hierarchy roll-up needs statusgen's stream/DORA substrate (and, for team,
	// the owning-team dimension), which this build does not ingest — so the
	// SCOPING is could-not-check even though the cell total is checked-clean.
	res.ScopeAssessment = domain.Unchecked(domain.Probe{
		Source: "statusgen --json / --dora over the cell's streams_root (the declared source for scoped roll-ups)",
		Method: domain.MethodDelegated,
		Window: a.Scope + "-scoped roll-up within cell " + orAll(a.Cell),
		Limit:  0,
	}, "the value is the cell-wide index count; the "+a.Scope+"-scoped breakdown is not derived here — "+
		"deskd renders scoped metrics from statusgen rather than computing a rival roll-up, and that substrate is not ingested in this build")
	return res, nil
}

// desksSchema is desk__desks' tool schema (the service contract): a single
// required cell argument, exactly as desk__overrides and desk__status take
// one.
func desksSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cell": map[string]any{"type": "string", "description": "the cell whose desk registry to read"},
		},
		"required": []string{"cell"},
	}
}

// findCell resolves a cell by name from the loaded config, or nil.
func (s *Server) findCell(name string) *schema.Cell {
	if s.Config == nil {
		return nil
	}
	for i := range s.Config.Cells {
		if s.Config.Cells[i].Name == name {
			return &s.Config.Cells[i]
		}
	}
	return nil
}

// handleDesks is the desk__desks read (the service contract): the MCP twin of
// BOTH GET /v1/desks and GET /v1/desks/{id}/terminal — the parity map keys on
// path, not tool name, so one tool legitimately answers for two routes. It
// returns the SAME []contract.DeskRow the HTTP read serves for the named
// cell, joining cells.yaml loops, App identity and the terminal registry's
// live state. s.Terminal may be nil (no registry wired): [terminal.Registry]
// nil-receiver DeskRows still returns one could-not-check row per configured
// loop, never a fabricated empty list (three-state, §10).
func handleDesks(_ context.Context, s *Server, call Call) (any, error) {
	var a struct {
		Cell string `json:"cell"`
	}
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__desks arguments did not decode: %w", err)
	}
	if a.Cell == "" {
		return nil, fmt.Errorf("desk__desks: cell is required")
	}
	cell := s.findCell(a.Cell)
	if cell == nil {
		return nil, fmt.Errorf("desk__desks: no such cell %q is configured", a.Cell)
	}
	rows := s.Terminal.DeskRows(cell)
	return map[string]any{
		"cell":  a.Cell,
		"desks": rows,
		"count": len(rows),
	}, nil
}

// ---- report_file (agent may file reports — design §5.3, §7.1) --------------

// ReportArgs is the desk__report_file argument shape.
type ReportArgs struct {
	Cell             string   `json:"cell"`
	Repo             string   `json:"repo"`
	Classification   string   `json:"classification"` // good|bad|ugly
	Summary          string   `json:"summary"`
	Evidence         []string `json:"evidence"`
	SuggestedCluster string   `json:"suggested_cluster"`
}

var reportClasses = map[string]bool{"good": true, "bad": true, "ugly": true}

func handleReportFile(ctx context.Context, s *Server, call Call) (any, error) {
	if s.Filer == nil {
		return nil, fmt.Errorf("desk__report_file has no write path configured (no %s write token) — could-not-check, never a silent no-op", AppIdentity)
	}
	var a ReportArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__report_file arguments did not decode: %w", err)
	}
	a.Cell = strings.TrimSpace(a.Cell)
	a.Summary = strings.TrimSpace(a.Summary)
	class := strings.ToLower(strings.TrimSpace(a.Classification))
	if a.Cell == "" || a.Summary == "" {
		return nil, fmt.Errorf("desk__report_file: cell and summary are required")
	}
	if !reportClasses[class] {
		return nil, fmt.Errorf("desk__report_file: classification %q is not one of good|bad|ugly", a.Classification)
	}
	repo := strings.TrimSpace(a.Repo)
	if repo == "" {
		repo = s.defaultRepo(a.Cell)
	}
	if repo == "" {
		return nil, fmt.Errorf("desk__report_file: no repo given and cell %q has no configured repo", a.Cell)
	}
	var b strings.Builder
	b.WriteString(a.Summary)
	if len(a.Evidence) > 0 {
		b.WriteString("\n\n### Evidence\n")
		for _, e := range a.Evidence {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	if a.SuggestedCluster != "" {
		fmt.Fprintf(&b, "\nSuggested cluster: %s\n", a.SuggestedCluster)
	}
	ref, err := s.Filer.FileEscalation(ctx, NewEscalation{
		Repo: repo, Cell: a.Cell, Title: fmt.Sprintf("[report/%s] %s", class, firstLine(a.Summary)),
		Body: b.String(), Labels: []string{"report", "report:" + class},
	})
	if err != nil {
		return nil, fmt.Errorf("desk__report_file: filing failed: %w", err)
	}
	return map[string]any{"outcome": "filed", "ref": ref, "classification": class,
		"note": "a report is agent-filed; adopting any change it suggests is the human's (§5.3, §6.2)"}, nil
}

// ---- delegated (could-not-check) reads ------------------------------------

// delegatedRead builds a read handler that returns an honest could-not-check
// naming its delegated source, exactly as the HTTP roadmap skeleton does. A
// route whose data this build does not derive serves an explicit unchecked
// verdict rather than an empty list a consumer would read as "nothing here".
func delegatedRead(tool, source string) func(context.Context, *Server, Call) (any, error) {
	return func(_ context.Context, _ *Server, _ Call) (any, error) {
		return map[string]any{
			"assessment": domain.Unchecked(domain.Probe{
				Source: source,
				Method: domain.MethodDelegated,
				Window: "the delegated source for " + tool,
				Limit:  0,
			}, tool+" lands the tool, not the derivation: this build ingests no substrate for it, "+
				"so it is could-not-check rather than empty — an empty result would assert there is nothing to show"),
		}, nil
	}
}

// inbox reads the cell's rows from the index, or errors if there is no index.
func (s *Server) inbox(cell string) ([]index.Row, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("no index available: this server was built without a derived index to read")
	}
	rows, err := s.DB.Inbox(cell, false)
	if err != nil {
		return nil, fmt.Errorf("index read failed (BLIND, not empty): %w", err)
	}
	return rows, nil
}

// inboxSourcesNotRead is the standing disclosure the read tools carry, so a
// consumer never infers coverage from an empty list.
var inboxSourcesNotRead = []string{
	"pull requests, their reviews and CI checks — no PR state is derived here",
	"closed escalations — the read tools serve open rows; a closed one is out of window",
	"statusgen roadmap/DORA and per-team owning gates — delegated, not ingested in this build",
}

func orAll(cell string) string {
	if cell == "" {
		return "(all cells)"
	}
	return cell
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return strings.TrimSpace(s)
}
