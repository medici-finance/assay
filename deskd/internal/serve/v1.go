// v1.go — the VERSIONED read surface (the service contract, increments 1-3).
//
// The service contract task 2: mount the `/v1` route table declared in
// internal/contract on deskd's server, so a client host reads the inbox and
// item views — and now streams events and nudges a refresh — over the typed
// HTTP/JSON seam ruled on #1819 instead of a shared *index.DB.
//
//   - increment 1 landed the READ routes.
//   - increment 2 landed the enumerated named-action WRITE handlers
//     (`answer`/`ack`), the identity bearer gate, and the at-rest custody
//     removal.
//   - increment 3 lands `GET /v1/events` (the SSE ring buffer, [EventBus]) and
//     `POST /v1/escalations/{id}/refresh` (the App-identity nudge,
//     [Refresher]), plus wiring internal/adapter's answer-record parse into
//     the item view's Outcome (see [handleV1Escalation]).
//
// A POST to a READ route on this build is still a 405 (the read tree's
// methodGuard); a POST to an undeclared item sub-route is still a 404 (no
// general write route exists) — increment 3 changes neither guarantee.
//
// # Read-only, structurally — the same two guarantees as the un-versioned surface
//
// [NewV1] reuses the exact [Server] the read surface surface uses (no GitHub
// client on the serving path) and wraps its mux in the same [methodGuard], so
// the versioned surface cannot smuggle in a write any more than the
// un-versioned one can. The per-repo private-repo filter ([Server.Authz]) is
// applied on the listing (`/v1/inbox`) and per item (`/v1/escalations/{id}`),
// exactly as it is on the un-versioned inbox — the split changes the SEAM, not
// the security posture.
//
// # Staleness + ETag on every response
//
// Design §10: every view carries its staleness, displayed not hidden. Each
// `/v1` read response carries the contract [contract.Meta] envelope (`as_of` +
// `index_age_seconds`, a POINTER so a cold index serialises null not 0) in the
// body AND an `ETag` in the header. The ETag is derived from the index
// GENERATATION marker — the poll `as_of` — plus the request path and query, so
// it changes when the served representation changes (a new poll) and a client
// that sends `If-None-Match` with the current tag gets a 304 rather than the
// body again. A cold index (never polled) has no generation, so its ETag is
// pinned to a "cold" seed and every cold response validates alike.
package serve

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// NewV1 builds the `/v1` read surface handler (the service contract task 2, read
// half). It shares the [Server] shape with [New]: db is the derived index, cfg
// the cells, and authz the per-repo private-repo gate (the capability guard in
// the deskd wiring; nil leaves the feed unfiltered exactly as [New] does — the
// surface login gate in the middleware still fronts private cells).
//
// Every route registered here is a GET; the returned handler is wrapped in
// [methodGuard], so a POST to a `/v1` path — including a named-action write
// route this increment does not yet implement — is 405, never a silent accept.
//
// events and refresh are increment 3's additions, both nil-safe: a nil
// *EventBus leaves `/v1/events` mounted but answering 503 (could-not-check,
// never a hung stream); a nil Refresher leaves `/v1/escalations/{id}/refresh`
// UNMOUNTED (a 404), the same "absent, not refusing" shape [NamedActions]
// already takes when its wiring is incomplete.
func NewV1(db *index.DB, cfg *schema.File, authz SurfaceAuthorizer, actions *NamedActions, overrides *Overrides, term *terminal.Registry, events *EventBus, refresh Refresher) http.Handler {
	s := &Server{DB: db, Config: cfg, Now: func() time.Time { return time.Now().UTC() }, Authz: authz, Terminal: term}
	reads := http.NewServeMux()
	reads.HandleFunc(contract.Prefix+"/cells", s.handleV1Cells)
	reads.HandleFunc(contract.Prefix+"/inbox", s.handleV1Inbox)
	// The trailing slash matches the item view AND every write sub-route the
	// contract does not declare; the handler serves only GET {id} and 404s the
	// rest, which is what makes "there is no general write route" a 404 rather
	// than a 405 (Verify row 4: the route does not exist).
	reads.HandleFunc(contract.Prefix+"/escalations/", s.handleV1Escalation)
	reads.HandleFunc(contract.Prefix+"/roadmap", s.handleV1Roadmap)
	reads.HandleFunc(contract.Prefix+"/portfolio", s.handleV1Portfolio)
	reads.HandleFunc(contract.Prefix+"/metrics", s.handleV1Metrics)
	// The desk registry read (the service contract), registered as the EXACT
	// literal path (no trailing slash) so it does not fall into the
	// "/desks/" subtree redirect below — Go's ServeMux (1.22+) treats an exact
	// pattern and a subtree pattern sharing a prefix as distinct, non-ambiguous
	// registrations, so GET /v1/desks is served directly rather than 301'd to
	// the subtree.
	reads.HandleFunc(contract.Prefix+"/desks", s.handleV1Desks)
	// The terminal handle read (the service contract). Registered as a subtree
	// (like /escalations/ above) because a desk id is "<cell>/<role>" — it
	// carries a slash, so a single "{id}" path wildcard cannot capture it; the
	// handler parses the id between /desks/ and /terminal and 404s any other
	// shape. It is a READ only: no write sub-route is added here (the send route owns
	// send), so a POST is still 405 via the read tree's method guard.
	reads.HandleFunc(contract.Prefix+"/desks/", s.handleV1Terminal)

	// The read tree keeps its structural read-only guard: a POST to any READ
	// route is 405 before a handler sees it, exactly as before.
	guarded := methodGuard(reads)

	top := http.NewServeMux()
	// The no-general-write catch-all. Any POST to an item sub-route the
	// contract does not declare is a 404 — "the route does not exist" — and
	// NOT the 405 the read tree's method guard would otherwise give it. The
	// distinction is the whole of Verify row 4: a 405 says a route is there and
	// declined the verb; a 404 says there is no general comment/close/react
	// route to decline. Go's pattern precedence puts the two named actions
	// registered below ahead of this wildcard.
	top.HandleFunc("POST "+contract.Prefix+"/escalations/{id}/{action}", noSuchWriteRoute)
	// The named-action routes: exactly the KindNamedAction entries the contract
	// declares — {answer, ack} — and nothing else can be added here without
	// adding it to the contract's closed allow-list first.
	mountNamedActions(top, actions)
	// The local-state read marker (the service contract increment 4): the one
	// KindLocalState escalation route the contract declares. Ruling D7 —
	// deskd's own SQLite, no platform write — and gated per item exactly as the
	// un-versioned marker is (see v1_local.go); unmounted, a 404, when the
	// authorizer cannot answer that per-item question.
	mountV1Read(top, db, authz)
	// The event stream (the service contract increment 3, task 2): mounted
	// unconditionally so a build with no producer wired answers a clear 503
	// rather than a 404 that reads as "this route was never going to exist".
	top.HandleFunc("GET "+contract.Prefix+"/events", s.handleV1Events(events))
	// The refresh nudge (increment 3, task 2): a KindNudge route, not a
	// KindNamedAction — it authors nothing, so it is mounted only when wired
	// and otherwise absent (a 404), never present-but-refusing.
	if refresh != nil {
		top.HandleFunc("POST "+contract.Prefix+"/escalations/{id}/refresh", s.handleV1Refresh(refresh))
	}
	// The cell-override routes (the service contract): a GET read and a POST that
	// writes deskd's OWN SQLite (KindLocalState) and files a record PR — NOT a
	// human-authored platform write, so it is not a named action. Registered on
	// top with explicit methods so the POST bypasses the read tree's methodGuard.
	mountOverrides(top, overrides, s.cellGate)
	// The terminal named actions (the service contract): send and session — the
	// other two entries of the closed allow-list {answer, ack, send, session}
	// — from the same contract walk, mounted only when their own wiring
	// (viewer identity, answerer tier, the terminal registry) is complete.
	mountTerminalActions(top, actions)
	// Everything else is the read tree, structurally read-only as before.
	top.Handle("/", guarded)
	return top
}

// noSuchWriteRoute answers a POST to an undeclared item sub-route. It names the
// closed allow-list so a caller reading the error learns the surface's shape
// rather than guessing at it.
func noSuchWriteRoute(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{
		"error": "no such route: the service performs only the enumerated named actions on the person's behalf, and " +
			r.PathValue("action") + " is not one of them (custody model (b), #1819)",
		"named_actions": contract.NamedActions,
	})
}

// meta builds the staleness envelope every v1 read carries. index_age_seconds
// and as_of are nil on a cold index (never polled) — a zero age would read as
// "fresh", the single worst thing a blank board can say (design §10).
func (s *Server) meta() contract.Meta {
	var m contract.Meta
	if s.DB == nil {
		return m
	}
	now := s.Now()
	age, assessment := s.DB.IndexAge(now)
	if assessment.Confidence != domain.CouldNotCheck {
		secs := int64(age.Seconds())
		asOf := now.Add(-age)
		m.IndexAgeSeconds = &secs
		m.AsOf = &asOf
	}
	return m
}

// handleV1Inbox serves GET /v1/inbox as [contract.InboxResponse]. Rows are
// filtered per-repo by the capability authorizer (the private-repo gate on the
// listing) before their domain escalations are lifted onto the wire.
func (s *Server) handleV1Inbox(w http.ResponseWriter, r *http.Request) {
	cell := r.URL.Query().Get("cell")
	m := s.meta()
	rows, err := s.DB.Inbox(cell, false)
	if err != nil {
		s.writeV1Err(w, http.StatusInternalServerError, "inbox index read failed (BLIND, not empty): "+err.Error())
		return
	}
	rows = s.filterByCapability(r, rows)
	items := make([]domain.Escalation, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.Escalation)
	}
	s.writeV1(w, r, contract.InboxResponse{
		Meta:  m,
		Cell:  cell,
		Items: items,
		Total: len(items),
	})
}

// handleV1Escalation serves GET /v1/escalations/{id} as
// [contract.EscalationResponse]. Unlike increment 2, the read is no longer
// scoped to the OPEN inbox: the service contract increment 3 task 6 wired
// internal/adapter to derive an answer off a closed issue's polled comments,
// so a served item now includes CLOSED rows too — an open escalation still
// carries a nil Outcome (it is still being decided), and so does a closed one
// the adapter found no answer marker for; a closed, marked row carries the
// derived Outcome (see [v1Outcome]).
//
// A GET on a write sub-route (`/read`, `/answer`, `/ack`, `/refresh`) or on the
// bare collection is a 404 here: those are not read routes.
func (s *Server) handleV1Escalation(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, contract.Prefix+"/escalations/")
	if rest == "" || strings.Contains(rest, "/") {
		s.writeV1Err(w, http.StatusNotFound, "no such v1 read route: only GET /v1/escalations/{id} is served on this build")
		return
	}
	id := rest
	m := s.meta()
	rows, err := s.DB.Inbox("", true)
	if err != nil {
		s.writeV1Err(w, http.StatusInternalServerError, "item index read failed (BLIND, not empty): "+err.Error())
		return
	}
	for _, row := range rows {
		if string(row.ID) != id {
			continue
		}
		// Per-repo gate: an authenticated viewer with no read capability on the
		// item's repo gets a 404 (an unknown item, never a permission leak),
		// exactly as the item-scoped middleware and the listing filter decide.
		if !s.itemAuthorized(r, row) {
			s.writeV1Err(w, http.StatusNotFound, "no such item in the index")
			return
		}
		s.writeV1(w, r, contract.EscalationResponse{
			Meta:    m,
			Item:    row.Escalation,
			Outcome: v1Outcome(row),
		})
		return
	}
	s.writeV1Err(w, http.StatusNotFound, "no escalation "+id+" in the index (it may be unknown, or in a repo that could not be read)")
}

// v1Outcome builds the wire Outcome for a served item row (task 6): nil for
// an open escalation (still being decided, §4: GitHub is the record and the
// record is not written yet) and nil for a closed one the adapter found no
// answer marker for (today's prose-only close — deriving an outcome from free
// text is not this increment's scope, see the PR's Fail-first/Notes section).
// Otherwise it is the answer [adapter.DeriveAnswer] parsed off GitHub on poll,
// with Stale set when its options_rev no longer matches the escalation's
// CURRENT options_rev — design §10: an answer to a since-changed option set is
// displayed as stale, never hidden and never silently treated as current.
func v1Outcome(row index.Row) *contract.Outcome {
	if !row.Closed || row.Answer == nil {
		return nil
	}
	out := &contract.Outcome{OptionsRev: row.Answer.OptionsRev, AnsweredBy: row.Answer.By.Name}
	if row.Answer.Unverified {
		// answered-unverified: the real author, no option, no staleness
		// claim — there is no verified answer to be stale.
		out.Unverified = true
		return out
	}
	if row.Answer.Option != nil {
		out.Option = *row.Answer.Option
	}
	if row.Rev != "" && row.Answer.OptionsRev != "" && row.Answer.OptionsRev != row.Rev {
		out.Stale = true
	}
	return out
}

// handleV1Terminal serves GET /v1/desks/{id}/terminal as a
// [contract.TerminalHandle] (the service contract). The desk id is "<cell>/<role>"
// and carries a slash, so the id is parsed between /desks/ and /terminal rather
// than from a single path wildcard.
//
// The security property (SPOF layer (b)): the id names a DESK, and the registry
// resolves the target from cells.yaml — the request carries nothing that names
// a target. A configured desk with no target, or an unreachable one, is a
// could-not-check ROW (200), never a 404 (spec §5.2); only an id that names no
// configured desk is a 404. A GET on any other /desks/ shape is a 404.
//
// Authorization (issue #132): the handle carries live runtime state once a
// driver attaches — OSC title, agent identity, derived state, cursor — so it
// is gated per CELL exactly as the stream is; a viewer with no read capability
// on the desk's cell gets 403 and nothing else.
func (s *Server) handleV1Terminal(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, contract.Prefix+"/desks/")
	// The terminal SSE stream (the service contract) shares this subtree; it is
	// matched FIRST because its suffix ends the longer path (v1_terminal.go).
	if id, ok := strings.CutSuffix(rest, terminalStreamSuffix); ok && id != "" {
		s.handleV1TerminalStream(w, r, id)
		return
	}
	id, ok := strings.CutSuffix(rest, "/terminal")
	if !ok || id == "" {
		s.writeV1Err(w, http.StatusNotFound, "no such v1 read route: only GET /v1/desks/{id}/terminal and GET /v1/desks/{id}/terminal/stream are served on this build")
		return
	}
	// The per-cell gate (issue #132), shared with the stream and decided BEFORE
	// any desk lookup — see [Server.deskAuthorized]. It also runs before the
	// not-wired branch below, so that branch can no longer answer 200 for an id
	// whose cell is not configured at all.
	if !s.deskAuthorized(w, r, id) {
		return
	}
	if s.Terminal == nil {
		// The registry is not wired on this build: could-not-check, never a
		// crash and never a fabricated live handle (§10, three-state).
		s.writeV1(w, r, contract.TerminalHandle{DeskID: id, State: string(terminal.StateCouldNotCheck), Attach: terminal.NotAttachedStatus(terminal.ReasonDisabled, nil)})
		return
	}
	h, found := s.Terminal.Resolve(id)
	if !found {
		s.writeV1Err(w, http.StatusNotFound, "no such desk "+id+" in the registry (a configured desk with no target is a could-not-check row, not a 404 — this id names no configured desk)")
		return
	}
	s.writeV1(w, r, h)
}

// handleV1Desks serves GET /v1/desks?cell= as [contract.DesksResponse]
// (the service contract): the join of the cell's configured loops, App
// identities and live terminal-registry state into one row per desk — never
// dropping a row for an unreachable target (spec §5.2, design §10's silent-
// cell failure mode).
//
// `?cell=` is required: unlike /v1/inbox (which defaults to every cell), a
// desk registry with no cell named has no well-defined row set to return.
// An unknown cell is a 404; a cell the viewer has no read capability on is a
// 403 — never an empty list, which would read as "configured with zero
// desks", a different fact (facts).
func (s *Server) handleV1Desks(w http.ResponseWriter, r *http.Request) {
	cell := r.URL.Query().Get("cell")
	if cell == "" {
		s.writeV1Err(w, http.StatusBadRequest, "cell is required (?cell=<name>) — GET /v1/desks reads one cell's desk registry")
		return
	}
	c := s.findCell(cell)
	if c == nil {
		s.writeV1Err(w, http.StatusNotFound, "no such cell "+cell+" is configured")
		return
	}
	if !s.cellAuthorized(r, c) {
		s.writeV1Err(w, http.StatusForbidden, "the viewer has no read capability on cell "+cell+"'s repos")
		return
	}
	var rows []contract.DeskRow
	if s.Terminal != nil {
		rows = s.Terminal.DeskRows(c)
	} else {
		// No terminal registry wired on this build: build the row set from
		// config alone, exactly as [terminal.Registry.DeskRows] would with a
		// nil registry (every row could-not-check, never a crash and never an
		// empty list — three-state, §10).
		rows = (*terminal.Registry)(nil).DeskRows(c)
	}
	s.writeV1(w, r, contract.DesksResponse{Meta: s.meta(), Cell: cell, Desks: rows})
}

// cellGate decides a cell-addressed route: 404 for a cell this build does not
// configure, 403 for one the viewer has no read capability on. It writes the
// refusal and returns false; true means the handler may run.
func (s *Server) cellGate(w http.ResponseWriter, r *http.Request, name string) bool {
	c := s.findCell(name)
	if c == nil {
		s.writeV1Err(w, http.StatusNotFound, "no such cell "+name+" is configured")
		return false
	}
	if !s.cellAuthorized(r, c) {
		s.writeV1Err(w, http.StatusForbidden, "the viewer has no read capability on cell "+name+"'s repos")
		return false
	}
	return true
}

// findCell resolves a cell by name from the loaded config, or nil if this
// build has no config or no cell of that name.
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

// cellAuthorized reports whether the request's viewer may read a CELL's desk
// registry: fail-closed over every repo the cell declares, mirroring
// [itemAuthorized]'s per-repo check but applied to the whole set rather than
// one item's single repo (facts: "filtered by the viewer's permission on the
// cell's repos"). With no Authz wired every cell is served — the middleware's
// surface login gate still fronts private cells, exactly as elsewhere on this
// surface.
func (s *Server) cellAuthorized(r *http.Request, c *schema.Cell) bool {
	if s.Authz == nil {
		return true
	}
	authorized := s.Authz.SurfaceAuthorizer(r)
	for _, repo := range c.Repos {
		owner, name, ok := strings.Cut(repo, "/")
		if !ok || !authorized(owner, name) {
			return false
		}
	}
	return true
}

// itemAuthorized reports whether the request's viewer may see this row's repo.
// With no Authz wired the item is served (the middleware's surface login gate
// still fronts private cells); otherwise the row's repo is resolved from its
// trusted platform deep-link and asked of the per-repo authorizer, fail-closed.
func (s *Server) itemAuthorized(r *http.Request, row index.Row) bool {
	if s.Authz == nil {
		return true
	}
	authorized := s.Authz.SurfaceAuthorizer(r)
	ref, ok := answer.ParseRef(row.Links["platform"])
	if !ok {
		return false
	}
	owner, name, ok := strings.Cut(ref.Repo, "/")
	if !ok {
		return false
	}
	return authorized(owner, name)
}

// v1CellsResponse is GET /v1/cells: the configured cells with the staleness
// envelope. The client decodes this route with the generic typed read
// ([contract.Client.Get]), so the shape is owned here; it mirrors the
// un-versioned /cells body with Meta added.
type v1CellsResponse struct {
	contract.Meta
	Cells []CellSummary `json:"cells"`
}

// handleV1Cells lists the cells the viewer may read: a cell whose repositories
// the viewer cannot all read is omitted, so a private cell's name and
// repositories are not disclosed to every signed-in viewer.
func (s *Server) handleV1Cells(w http.ResponseWriter, r *http.Request) {
	var out []CellSummary
	if s.Config != nil {
		for i := range s.Config.Cells {
			c := s.Config.Cells[i]
			if !s.cellAuthorized(r, &c) {
				continue
			}
			cs := CellSummary{Name: c.Name, Repos: c.Repos, SilenceThreshold: c.SilenceThreshold}
			for _, l := range c.Loops {
				cs.Loops = append(cs.Loops, fmt.Sprintf("%s→%s", l.Role, l.App))
			}
			out = append(out, cs)
		}
	}
	s.writeV1(w, r, v1CellsResponse{Meta: s.meta(), Cells: out})
}

// v1DelegatedResponse is a could-not-check read: a route whose data this build
// does not derive serves an explicit unchecked verdict naming its delegated
// source, never an empty list a consumer would read as "nothing here" (§10,
// three-state). It matches the un-versioned roadmap skeleton and the MCP
// delegatedRead tools.
type v1DelegatedResponse struct {
	contract.Meta
	Streams    []any             `json:"streams"`
	Assessment domain.Assessment `json:"assessment"`
}

func (s *Server) handleV1Roadmap(w http.ResponseWriter, r *http.Request) {
	s.writeV1(w, r, v1DelegatedResponse{
		Meta:    s.meta(),
		Streams: nil,
		Assessment: domain.Unchecked(domain.Probe{
			Source: "statusgen --json over each cell's streams_root (the single writer of status; deskd renders, never re-derives)",
			Method: domain.MethodDelegated,
			Window: "every stream README under the cell's streams_root",
			Limit:  0,
		}, "the service contract mounts the v1 route, not the derivation: nothing in this build invokes the delegated source, "+
			"so the roadmap is could-not-check rather than empty. An empty stream list would assert there is no work."),
	})
}

func (s *Server) handleV1Portfolio(w http.ResponseWriter, r *http.Request) {
	s.writeV1(w, r, v1DelegatedResponse{
		Meta:    s.meta(),
		Streams: nil,
		Assessment: domain.Unchecked(domain.Probe{
			Source: "statusgen --json / --dora across programs (the declared source for the portfolio strip)",
			Method: domain.MethodDelegated,
			Window: "every program's stream/DORA substrate",
			Limit:  0,
		}, "the service contract mounts the v1 route, not the derivation: this build ingests no portfolio substrate, "+
			"so it is could-not-check rather than empty — an empty strip would assert there are no programs."),
	})
}

// v1MetricsResponse is GET /v1/metrics: a deterministic index-native count with
// its probe, plus an honest could-not-check on the scoped roll-up this build
// cannot perform. It mirrors the MCP desk__metrics_query result (§5.2, §7.5).
type v1MetricsResponse struct {
	contract.Meta
	Scope           string            `json:"scope"`
	Metric          string            `json:"metric"`
	Value           any               `json:"value"`
	Assessment      domain.Assessment `json:"assessment"`
	ScopeAssessment domain.Assessment `json:"scope_assessment"`
}

var v1MetricScopes = map[string]bool{"epic": true, "program": true, "portfolio": true, "team": true}

func (s *Server) handleV1Metrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	scope := strings.ToLower(strings.TrimSpace(q.Get("scope")))
	if scope == "" {
		scope = "portfolio"
	}
	if !v1MetricScopes[scope] {
		s.writeV1Err(w, http.StatusBadRequest, "scope "+scope+" is not one of epic|program|portfolio|team (§5.2)")
		return
	}
	metric := strings.ToLower(strings.TrimSpace(q.Get("metric")))
	if metric == "" {
		metric = "open_escalations"
	}
	cell := q.Get("cell")
	rows, err := s.DB.Inbox(cell, false)
	if err != nil {
		s.writeV1Err(w, http.StatusInternalServerError, "metrics index read failed (BLIND, not empty): "+err.Error())
		return
	}
	// Per-repo capability gate: the aggregate must be computed over ONLY the rows
	// this viewer may read, exactly as handleV1Inbox filters its listing. Without
	// this a viewer denied a cell's private repo still sees that repo's escalation
	// count, urgency histogram and ages (amplified across cells when cell==""),
	// re-opening the authorization class Security-Review 5091037270 closed.
	rows = s.filterByCapability(r, rows)
	probe := domain.Probe{
		Source: "derived_escalations (SQLite) via /v1/metrics — the index only; the serving path makes no network call",
		Method: domain.MethodLocalTable,
		Window: "open escalations of cell " + orAll(cell),
		Limit:  0,
	}
	res := v1MetricsResponse{Meta: s.meta(), Scope: scope, Metric: metric, Assessment: domain.Clean(probe)}
	switch metric {
	case "open_escalations":
		res.Value = len(rows)
	case "unresolved_rows":
		n := 0
		for _, row := range rows {
			if len(row.Fields.Unresolved()) > 0 {
				n++
			}
		}
		res.Value = n
	case "oldest_age_seconds":
		var oldest int64
		for _, row := range rows {
			if row.AgeSeconds > oldest {
				oldest = row.AgeSeconds
			}
		}
		res.Value = oldest
	case "by_urgency":
		mm := map[string]int{}
		for _, row := range rows {
			mm[row.Urgency.String()]++
		}
		res.Value = mm
	default:
		s.writeV1Err(w, http.StatusBadRequest, "metric "+metric+" is not index-native "+
			"(known: open_escalations, unresolved_rows, oldest_age_seconds, by_urgency); "+
			"DORA percentiles are delegated to statusgen and not ingested in this build")
		return
	}
	// The number is a CELL-WIDE index count; the epic/program/portfolio/team
	// roll-up needs statusgen's substrate this build does not ingest — so the
	// SCOPING is could-not-check even though the cell total is checked-clean.
	res.ScopeAssessment = domain.Unchecked(domain.Probe{
		Source: "statusgen --json / --dora over the cell's streams_root (the declared source for scoped roll-ups)",
		Method: domain.MethodDelegated,
		Window: scope + "-scoped roll-up within cell " + orAll(cell),
		Limit:  0,
	}, "the value is the cell-wide index count; the "+scope+"-scoped breakdown is not derived here — "+
		"deskd renders scoped metrics from statusgen rather than computing a rival roll-up, and that substrate is not ingested in this build")
	s.writeV1(w, r, res)
}

// writeV1 marshals a v1 read body, sets the ETag header from the index
// generation marker, and answers 304 to a matching If-None-Match. The ETag is
// derived from the poll as_of (the generation the served data belongs to) plus
// the request path and query — so it changes on a new poll and a repeated read
// of an unchanged generation validates as Not Modified.
func (s *Server) writeV1(w http.ResponseWriter, r *http.Request, body any) {
	etag := s.etagFor(r, body)
	w.Header().Set("ETag", etag)
	if inm := r.Header.Get("If-None-Match"); inm != "" && etagMatches(inm, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	buf, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		s.writeV1Err(w, http.StatusInternalServerError, "response encoding failed: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf)
	_, _ = w.Write([]byte("\n"))
}

// etagFor builds a strong ETag for a v1 read. The seed is the index generation
// (the poll as_of, or "cold" before the first poll) plus the route and query,
// which together identify the representation: the served data is a pure
// function of the index generation, so two reads of the same generation of the
// same route return the same tag.
func (s *Server) etagFor(r *http.Request, body any) string {
	gen := "cold"
	if m, ok := metaOf(body); ok && m.AsOf != nil {
		gen = m.AsOf.UTC().Format(time.RFC3339Nano)
	}
	seed := gen + "\x00" + r.URL.Path + "\x00" + r.URL.RawQuery
	sum := sha256.Sum256([]byte(seed))
	return `"` + hex.EncodeToString(sum[:])[:32] + `"`
}

// metaOf extracts the contract.Meta a v1 body carries, whether it is a named
// response type ([contract.InboxResponse], [contract.EscalationResponse]) or one
// of the local types that embed [contract.Meta].
func metaOf(body any) (contract.Meta, bool) {
	switch b := body.(type) {
	case contract.InboxResponse:
		return b.Meta, true
	case contract.EscalationResponse:
		return b.Meta, true
	case v1CellsResponse:
		return b.Meta, true
	case v1DelegatedResponse:
		return b.Meta, true
	case v1MetricsResponse:
		return b.Meta, true
	case contract.DesksResponse:
		return b.Meta, true
	}
	return contract.Meta{}, false
}

// etagMatches reports whether an If-None-Match header value matches the current
// tag. It handles the "*" wildcard and a comma-separated list, and ignores a
// weak-validator "W/" prefix (this surface only ever emits strong tags).
func etagMatches(ifNoneMatch, current string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "*" {
		return true
	}
	for _, part := range strings.Split(ifNoneMatch, ",") {
		p := strings.TrimSpace(part)
		p = strings.TrimPrefix(p, "W/")
		if p == current {
			return true
		}
	}
	return false
}

// writeV1Err writes a v1 error body. It carries no staleness envelope — an
// error is not a view of the index — but stays JSON so a client decodes it.
func (s *Server) writeV1Err(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
