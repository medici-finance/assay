package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/deskd/internal/chat"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// This file adds the altitude panes: Roadmap + epic
// drill-down, Measures, Portfolio, and Teams. Every pane carries the §5.2
// ambient metrics strip at its own scope (the binding ambient rule), and every
// pane renders honestly: the figures this build derives from the index are
// checked-clean; the roadmap/DORA/team substrate delegated to statusgen is
// could-not-check, naming its declared source, never a fabricated zero. This is
// the deliberate RENDERING scope of the brief — the Console renders, statusgen
// stays the single writer (§5).

// paletteFor maps a derived attention state to the render palette token per the
// D5/§3.5 rollup order:
//
//	working > waiting-in-SLA > idle > waiting-breached > silent > failed-to-start
//	healthy   healthy ⟡ gold   neutral  amber→escalating   amber-?  fault(red)
//
// `waiting` ranks HEALTHY: a loop that filed a well-formed escalation and moved
// on is the system working (§7.1), so it renders CALM (gold ⟡) — visual
// escalation is triggered by the CLOCK (an SLA breach), never by the state. And
// the SLA breach ladder ships BUILT-BUT-UNARMED in v1 (D3): no targets are
// configured, so no `waiting` row is ever classified breached here — it stays
// healthy until a target arrives later as a RetroAction from dogfood data. An
// inbox that renders every `waiting` item as alarming trains its operator to
// ignore alarm (§3.5).
func paletteFor(state domain.AttentionState) string {
	switch state {
	case domain.Working, domain.Waiting:
		return "healthy" // waiting is calm until an (unarmed) SLA breach — never the fault token
	case domain.Idle:
		return "neutral"
	case domain.Silent:
		return "amber" // amber-as-a-question, never green (§3.5, §10)
	case domain.FailedToStart:
		return "fault"
	}
	return "amber"
}

// paneScope maps a route to the metrics scope its ambient strip carries (§5.2:
// same desk__metrics_query, scope from context). `team` cuts across the
// hierarchy; the others cut down it (§3.1).
const (
	scopePortfolio = "portfolio"
	scopeProgram   = "program"
	scopeEpic      = "epic"
	scopeTeam      = "team"
)

// ---- Roadmap + epic drill-down (§7.3 pane 2) -------------------------------

// roadmapVM is the altitude view (§1.3). The wave/critical-path/Next-up
// substrate is statusgen's; this build does not ingest it, so the pane declares
// its delegated source and renders could-not-check for the stream data while
// still carrying the live ambient strip at program scope (§7.3 pane 2: the pane
// renders, statusgen remains the single writer).
type roadmapVM struct {
	Ambient     ambientVM
	Delegated   tileVM // the could-not-check stream/wave/Next-up panel
	EpicWaiting tileVM // epic-scoped waiting-on-you rail is index-native
}

func (s *Server) handleRoadmap(w http.ResponseWriter, r *http.Request) {
	src, err := s.newMetricsSource()
	if err != nil {
		http.Error(w, "roadmap metrics failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	vm := roadmapVM{
		Ambient: ambientFor(src, scopeProgram),
		Delegated: delegatedTile("roadmap_streams", "streams · waves · critical path · Next-up",
			"--json over the cell's streams_root", hardPlatform, scopeProgram),
		EpicWaiting: indexTile("waiting_on_you", "waiting on you (this program)", "waiting_on_you",
			hardPlatform, scopeProgram, fmt.Sprintf("%d", waitingOnYou(src.rows))),
	}
	s.render(w, "roadmap.html", vm)
}

// epicVM is the epic detail (frame 07): dependency waves, the critical path,
// blocked cards naming the escalation that holds them, and findings ⚠ flags —
// all statusgen semantics, delegated and could-not-check here — plus the
// epic-scoped waiting-on-you and recent-records rails, which ARE index-native.
type epicVM struct {
	EpicID       string
	Ambient      ambientVM
	Delegated    tileVM
	WaitingRail  tileVM
	RecentRecord tileVM
	BlockedOn    []blockedOnVM // escalations that hold this epic's work, linked into the inbox
}

// blockedOnVM is one "blocked-on" link — a waiting escalation the epic's cards
// point at (§7.3 pane 2: blocked cards name the escalation that holds them).
type blockedOnVM struct {
	Title       string
	Age         string
	InboxLink   string
	PlatformURL string
	State       string // attention state token (data-state), for the healthy/amber grammar
	Word        string
	Glyph       string
	Palette     string // healthy | neutral | amber | fault (data-palette) — §3.5/D5
}

func (s *Server) handleEpic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, err := s.newMetricsSource()
	if err != nil {
		http.Error(w, "epic metrics failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	vm := epicVM{
		EpicID:  id,
		Ambient: ambientFor(src, scopeEpic),
		Delegated: delegatedTile("epic_detail", "waves · critical path · findings ⚠",
			"--json over the epic's stream README", hardPlatform, scopeEpic),
		WaitingRail: indexTile("waiting_on_you", "waiting on you (this epic)", "waiting_on_you",
			hardPlatform, scopeEpic, fmt.Sprintf("%d", waitingOnYou(src.rows))),
		RecentRecord: delegatedTile("recent_records", "recent records",
			"--json stream register", hardPlatform, scopeEpic),
		// The blocked-on list emits each Waiting escalation's Title + item
		// deep-link + platform deep-link — the SAME withheld private-repo
		// metadata the inbox row carries, from the same DB.Inbox("", true)
		// source. So it is gated per-repo here exactly as the inbox feed is
		// (Security-Review sibling-route fix): rows on a repo the viewer holds
		// no read capability on are dropped before render, fail-closed.
		BlockedOn: blockedOnRows(s.authorizedRows(r, src.rows)),
	}
	s.render(w, "epic.html", vm)
}

// authorizedRows drops from a cross-cell index-row slice every row whose owning
// repo the viewer is not authorized to see on the surface (multi-repo) listing.
// It is the shared per-repo private-repo gate for the panes that derive
// PER-REPO facts from the unfiltered DB.Inbox("", true) source — the epic
// blocked-on list (row titles/deep-links) and the portfolio running-automations
// strip (a specific repo's escalation existence/age). It reuses the guard's
// SurfaceAuthorizer, the SAME deskd-side derivation the inbox fix and the item
// route use — never a parallel check — and ownerNameFromURL to resolve the repo
// from each row's platform deep-link; an unresolvable link fails closed (row
// dropped). Aggregate counts (the ambient strip's waiting-on-you total) are
// deliberately NOT gated through this — they name no repo and match the
// intended portfolio-metric visibility. With no Viewer wired (anonymous-public
// deployment / tests) the rows are returned unfiltered, exactly as before; the
// middleware's surface login gate still fronts private cells.
func (s *Server) authorizedRows(r *http.Request, rows []index.Row) []index.Row {
	if s.deps.Viewer == nil {
		return rows
	}
	authorized := s.deps.Viewer.SurfaceAuthorizer(r)
	out := make([]index.Row, 0, len(rows))
	for _, row := range rows {
		owner, name := ownerNameFromURL(row.Links["platform"])
		if authorized(owner, name) {
			out = append(out, row)
		}
	}
	return out
}

// blockedOnRows renders the open `waiting` escalations as blocked-on links. It
// is index-native: an open escalation IS a blocked-on card, and it links back
// into the inbox where it can be answered.
func blockedOnRows(rows []index.Row) []blockedOnVM {
	var out []blockedOnVM
	for _, r := range rows {
		if r.Attention == nil || r.Attention.State != domain.Waiting {
			continue
		}
		out = append(out, blockedOnVM{
			Title:       r.Title,
			Age:         humanAge(r.AgeSeconds),
			InboxLink:   "/item/" + string(r.ID),
			PlatformURL: r.Links["platform"],
			State:       string(r.Attention.State),
			Word:        r.Attention.Word,
			Glyph:       r.Attention.Glyph,
			Palette:     paletteFor(r.Attention.State),
		})
	}
	return out
}

// ---- Measures (§7.3 pane 5) ------------------------------------------------

func (s *Server) handleMeasures(w http.ResponseWriter, r *http.Request) {
	src, err := s.newMetricsSource()
	if err != nil {
		http.Error(w, "measures metrics failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = scopePortfolio
	}
	s.render(w, "measures.html", measuresFor(src, scope))
}

// ---- Portfolio + running-automations strip (§3.4, D2) ----------------------

// portfolioVM is the multi-cell view (§7.3 pane 7) with the D2 "Running
// automations" live strip. Every column is DERIVED — no self-reported field —
// and there is NO context-burn column (rejected for v1, §3.4/D2). Staleness is
// carried per §10.
type portfolioVM struct {
	Ambient ambientVM
	Cells   []cellStripVM
}

type cellStripVM struct {
	Cell        string
	Automations []automationRowVM
}

// automationRowVM is one row of the running-automations strip (§3.4). Columns:
// automation, attention state, working-on, age-in-state, oldest open
// escalation (links into the inbox), model, last-artifact age — every one
// derived from the index or config, none self-reported. The columns whose
// source this build does not ingest (open claim, artifact plane, observed
// model) render could-not-check, never a zero. Deliberately NO context-burn
// column (§1.2/D2: no derived source exists).
type automationRowVM struct {
	Automation      string // role@cell
	State           string // attention state word
	StateKey        string // attention state token (data-state) — drives the healthy/amber grammar
	Palette         string // healthy | neutral | amber | fault (data-palette) — §3.5/D5
	Glyph           string
	Silent          bool   // silent renders amber-as-a-question, never green (§3.4, §10)
	OldestEscAge    string // ● from FiledAt, or "—"
	OldestEscLink   string // links into the inbox
	WorkingOn       tileVM // ◐ open claim — not ingested
	AgeInState      tileVM // ● timestamp of the state-defining artifact — not ingested
	Model           tileVM // ◐ what actually ran — not ingested; reason names the configured ladder
	LastArtifactAge tileVM // ● liveness — not ingested
}

func (s *Server) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	src, err := s.newMetricsSource()
	if err != nil {
		http.Error(w, "portfolio metrics failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	vm := portfolioVM{Ambient: ambientFor(src, scopePortfolio)}
	vm.Cells = s.automationStrips(r, src)
	s.render(w, "portfolio.html", vm)
}

// automationStrips builds one running-automations strip per configured cell. A
// row per configured loop (§3.4): the strip's row count is the number of
// running automations in the cell, derived from config, not self-reported. When
// no config is wired the strip is empty and the pane says so — never a
// fabricated row.
func (s *Server) automationStrips(r *http.Request, src metricsSource) []cellStripVM {
	if s.deps.Config == nil {
		return nil
	}
	// The configured cell/role STRUCTURE (which automations exist) is deskd's
	// own operator config (cells.yaml), not private-repo metadata, so every
	// configured row still renders. But the per-automation escalation FACTS
	// below — whether a specific repo's automation has an open Waiting
	// escalation, and that escalation's age — ARE derived from private-repo
	// escalations, and are the same withheld-metadata class as the inbox/epic
	// leak. So the rows those facts derive from are gated per-repo first: a
	// viewer with no read on a cell's private repos sees the automation as
	// SILENT with no oldest-escalation age, never its private escalation
	// existence/age (Security-Review sibling-route audit).
	rows := s.authorizedRows(r, src.rows)
	// Index rows grouped by (cell, role) so per-automation escalation facts are
	// index-native.
	oldestByAuto := map[string]int64{}
	waitingByAuto := map[string]bool{}
	for _, row := range rows {
		if row.Automation.Zero() {
			continue
		}
		key := string(row.Automation.Cell) + "/" + string(row.Automation.Role)
		if row.AgeSeconds > oldestByAuto[key] {
			oldestByAuto[key] = row.AgeSeconds
		}
		if row.Attention != nil && row.Attention.State == domain.Waiting {
			waitingByAuto[key] = true
		}
	}

	var out []cellStripVM
	for _, c := range s.deps.Config.Cells {
		strip := cellStripVM{Cell: c.Name}
		for _, loop := range c.Loops {
			key := c.Name + "/" + string(loop.Role)
			ar := automationRowVM{Automation: key}

			// Attention state: `waiting` when this automation has an open
			// escalation (§1.2 rollup); otherwise SILENT — unknown, rendered
			// amber-as-a-question, never a green "idle" this build cannot
			// honestly derive without the artifact plane (§3.5, §1.2).
			if waitingByAuto[key] {
				st := domain.Waiting
				ar.State, ar.StateKey, ar.Glyph, ar.Palette = st.Word(), string(st), st.Glyph(), paletteFor(st)
			} else {
				st := domain.Silent
				ar.State, ar.StateKey, ar.Glyph, ar.Palette = st.Word(), string(st), st.Glyph(), paletteFor(st)
				ar.Silent = true
			}

			if age, ok := oldestByAuto[key]; ok && age > 0 {
				ar.OldestEscAge = humanAge(age)
				ar.OldestEscLink = "/inbox?cell=" + c.Name
			} else {
				ar.OldestEscAge = "—"
			}

			// Derived-but-not-ingested columns: could-not-check, never zero.
			ar.WorkingOn = notIngested("working_on", "open TTL claim lease", hardContent, key,
				"claims are deskd-local (desk__claim) and not surfaced on the read index in this build")
			ar.AgeInState = notIngested("age_in_state", "age in state", hardPlatform, key,
				"needs the timestamp of the state-defining artifact — no artifact plane is ingested in this build")
			ar.LastArtifactAge = notIngested("last_artifact_age", "last artifact age", hardPlatform, key,
				"liveness needs the artifact plane, not ingested in this build")
			model := notIngested("model", "model (routing drift)", hardContent, key,
				"§9 asks for what ACTUALLY ran; the observed-run signal is not ingested. Configured ladder: "+
					joinModels(loop.Models))
			ar.Model = model

			strip.Automations = append(strip.Automations, ar)
		}
		out = append(out, strip)
	}
	return out
}

func joinModels(models []string) string {
	if len(models) == 0 {
		return "(none configured)"
	}
	s := ""
	for i, m := range models {
		if i > 0 {
			s += " → "
		}
		s += m
	}
	return s
}

// notIngested builds a could-not-check tile for an automation-strip column
// whose source this build does not ingest.
func notIngested(key, label string, h hardness, scope, reason string) tileVM {
	return tileVM{
		Key:      key,
		Label:    label,
		Query:    "desk__portfolio (delegated) @ " + scope,
		Hardness: h,
		OK:       false,
		Reason:   reason,
		Scope:    scope,
	}
}

// ---- Teams (§7.3 pane 6, §3.1) ---------------------------------------------

// teamsVM is the orthogonal pivot — work grouped by WHO, not WHAT (§3.1). The
// owning-team dimension is not on the escalation index in this build, so the
// per-team breakdown and cross-team handoff waits are delegated and
// could-not-check, while the team-scoped ambient strip renders live.
type teamsVM struct {
	Ambient      ambientVM
	Delegated    tileVM
	HandoffWaits tileVM
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	src, err := s.newMetricsSource()
	if err != nil {
		http.Error(w, "teams metrics failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	vm := teamsVM{
		Ambient: ambientFor(src, scopeTeam),
		Delegated: delegatedTile("team_view", "per-team queue · WIP · gates by owning team",
			"--json per-gate owning-team", hardContent, scopeTeam),
		HandoffWaits: delegatedTile("handoff_waits", "cross-team handoff waits",
			"--trend cross-team handoff", hardPlatform, scopeTeam),
	}
	vm.Delegated.Reason = "the owning-team dimension is not on the escalation index in this build " +
		"(§3.1: no team dimension yet) — could-not-check, never an empty team list"
	s.render(w, "teams.html", vm)
}

// ---- shared render ---------------------------------------------------------

// render executes a template, or reports the failure. Panes share it so the
// error path is one place.
func (s *Server) render(w http.ResponseWriter, name string, vm any) {
	if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
	}
}

// ---- Ask Assay (§7.5 phase 1, the service contract) --------------------------

// AskAssay is the chat client this pane renders. It is an interface so the web
// layer takes no dependency on the model backend or the credential — only on
// the answer shape.
//
// Nil means the cell has no Ask Assay backend configured, which is the DEFAULT
// and the honest OFF state: the pane renders could-not-check and says why. It
// does not render an empty transcript that could be mistaken for "nothing to
// report", and it certainly does not fall back to answering from anything else.
type AskAssay interface {
	Ask(ctx context.Context, question string) (chat.Answer, error)
}

// askAssayVM is the Ask Assay pane (§7.5). Note what it does NOT carry: there
// is no field holding raw HTML, and no field a figure's value can be written
// into except through [figureChipVM], which is built from a chat.Figure.
type askAssayVM struct {
	Ambient  ambientVM
	Question string
	// State selects one of the pane's distinct renders (§7.6). It is a token,
	// not a set of booleans, so the template cannot draw two states at once:
	// resting | loading | unknown | answered.
	State string
	// Reason is the could-not-check explanation, shown in the `unknown` state.
	// A could-not-check with no reason is not a finding.
	Reason     string
	Transcript []turnVM
	Backend    string
	Model      string
	Escalated  []string
}

// turnVM is one turn of the transcript.
type turnVM struct {
	Role      string // "you" | "assay"
	Text      string // ESCAPED by html/template — never template.HTML
	Figures   []figureChipVM
	ToolCalls []toolCallVM
}

// figureChipVM is one cited figure as the §5.2 chip. Value is populated ONLY
// when the numbers rule says a value exists; otherwise OK is false and the
// template renders the could-not-check token. The view model never carries a
// digit the rule refused, so a template edit cannot resurrect one.
type figureChipVM struct {
	QueryKey   string
	Value      string
	OK         bool
	Href       string
	StateToken string
	Render     string // the rule's own one-line rendering, shown as the chip title
}

type toolCallVM struct {
	Tool    string
	OK      bool
	Refused string
}

func (s *Server) handleAskAssay(w http.ResponseWriter, r *http.Request) {
	src, err := s.newMetricsSource()
	if err != nil {
		http.Error(w, "ask assay metrics failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	question := strings.TrimSpace(r.URL.Query().Get("q"))
	vm := askAssayVM{Ambient: ambientFor(src, scopePortfolio), Question: question, State: "resting"}

	if question == "" {
		s.render(w, "askassay.html", vm)
		return
	}
	if s.deps.Chat == nil {
		vm.State = "unknown"
		vm.Reason = "no model backend is configured for this cell, so no question can be answered here. " +
			"This is the default state: Ask Assay is OFF until a cell's config names a ladder rung and a key " +
			"environment variable, which is a human's edit on a gated path."
		s.render(w, "askassay.html", vm)
		return
	}

	ans, err := s.deps.Chat.Ask(r.Context(), question)
	if err != nil {
		vm.State = "unknown"
		vm.Reason = "the question could not be answered: " + err.Error()
		s.render(w, "askassay.html", vm)
		return
	}
	vm.State = "answered"
	vm.Backend, vm.Model, vm.Escalated = ans.Backend, ans.Model, ans.Escalated
	vm.Transcript = []turnVM{
		{Role: "you", Text: question},
		{Role: "assay", Text: ans.Text, Figures: chipsFor(ans.Figures), ToolCalls: toolCallsFor(ans.ToolCalls)},
	}
	s.render(w, "askassay.html", vm)
}

// chipsFor turns the answer's rule-governed figures into chips. It reads the
// figure's STATE, never its field: a figure the rule refused yields OK=false
// and no Value at all, which is the same property askassay.Answer.Render holds
// one layer down, restated at the render boundary so neither layer alone is
// the whole guarantee.
func chipsFor(figs []chat.Figure) []figureChipVM {
	out := make([]figureChipVM, 0, len(figs))
	for _, f := range figs {
		chip := figureChipVM{
			QueryKey:   f.QueryKey(),
			Href:       f.MeasuresHref(),
			StateToken: string(f.State()),
			Render:     f.Render(),
		}
		if v, ok := f.Value(); ok {
			chip.OK = true
			chip.Value = strconv.FormatInt(v, 10)
		}
		out = append(out, chip)
	}
	return out
}

func toolCallsFor(calls []chat.ToolCall) []toolCallVM {
	out := make([]toolCallVM, 0, len(calls))
	for _, c := range calls {
		out = append(out, toolCallVM{Tool: c.Tool, OK: c.OK, Refused: c.Refused})
	}
	return out
}
