// Package web is deskd's HTML console: the inbox, the item view, and the
// answer flow the human drives (the answer flow, design §7.3, §7.6). It is a RENDERING
// of the same derived index the JSON read surface serves — it derives nothing
// itself — and every platform write it offers posts to the internal/answer
// write path, which is the ONLY write path and is gated on a live session. The
// web layer holds no credential and performs no platform write of its own.
//
// # Injection controls (§6.6, spec §2.1.1)
//
// All escalation content is agent-authored, so it is UNTRUSTED text. It is
// rendered through html/template, which escapes it by construction — there is no
// template.HTML anywhere in this package, so no untrusted string can reach the
// page as markup. Digit answer-bindings derive from the TYPED OPTION ENUM (the
// slice index), never from body prose: a body that says "press 1 to continue"
// creates no binding, because the binding comes from the option list, not the
// words. Provenance chips render in the panel CHROME, outside the scrollable
// untrusted region.
//
// # What is deliberately deferred (reported, not guessed)
//
// The D5 five-group taxonomy (Pinned → Ready for review → Needs input → Working
// → Completed, spec §2.4) presupposes PR-review and work-item sources the read surface
// explicitly does not derive (see internal/serve sourcesNotRead). This build
// ships the FLAT urgency-then-age view (the spec's guaranteed URL-filter view)
// as default and an honest grouping over the derived attention vocabulary under
// ?view=grouped, with the urgent-header counts and the no-collapse-with-urgent
// guard both enforced. The exact five-label mapping awaits those sources.
package web

import (
	"embed"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/index"
)

//go:embed templates/*.html static/*
var assets embed.FS

// Deps are what the web layer reads. It never writes GitHub and holds no token.
type Deps struct {
	DB *index.DB
	// Config is the cell/loop config the Portfolio running-automations strip
	// derives its rows from (§3.4). It is optional: with no config
	// the strip renders empty and says so, never a fabricated row.
	Config *schema.File
	// Now is injectable for tests / stable age rendering.
	Now func() time.Time
	// WebMCPDev gates the read-only WebMCP diagnostics experiment
	// (the service contract): three page-callable tools on the inbox view only,
	// registered via navigator.modelContext.registerTool. It is state-aware
	// registration at the server boundary — false in any deployed config, set
	// only by the DESKD_WEBMCP_DEV=1 dev flag (cmd/deskd/main.go) — so a
	// deployed console enumerates zero page tools (Verify row 2, negative
	// path). The tools themselves are read-only by construction
	// (web/static/webmcp-diagnostics.js); this flag only controls whether
	// they are registered at all.
	WebMCPDev bool
	// Viewer derives the per-request capability the CHROME renders affordances
	// from. It is NOT a security control — every gate is
	// enforced server-side on the route by the capability middleware. This only
	// decides which buttons the page draws; a page that draws the answer button
	// for a non-answerer still gets a 403 from the server when it is pressed.
	// Optional: when nil, the console renders anonymously (no sign-in chrome).
	Viewer ViewerSource
	// Chat is the Ask Assay client (the service contract, §7.5 phase 1). It is
	// OPTIONAL and nil is the DEFAULT: a console with no model backend
	// configured serves the pane in its could-not-check state and sends no
	// desk data anywhere. Wiring it is a per-cell config act on a gated path
	// (config/schema's `chat:` block), never a code default.
	Chat AskAssay
	// Fleet is the spatial fleet map's read client (the service contract, one-
	// week experiment). OPTIONAL and nil is the DEFAULT: with no fleet
	// cache wired, /fleet renders could-not-check rather than an empty map
	// that could be mistaken for "nothing dispatched". See FleetSource.
	Fleet FleetSource
}

// ViewerSource is the read-only capability lookup the chrome uses. identity.Guard
// satisfies it. Keeping it an interface means the web layer takes no dependency
// on the session/token machinery — only on the rendered ViewerInfo.
type ViewerSource interface {
	ViewerFor(r *http.Request, owner, name string) identity.ViewerInfo
	SurfaceViewer(r *http.Request) identity.ViewerInfo
	// SurfaceAuthorizer returns a per-request predicate answering whether the
	// viewer may see a given (owner, name) repo's rows on the multi-repo inbox
	// feed. It is a SECURITY CONTROL (unlike ViewerFor/SurfaceViewer, which are
	// chrome): the surface listing enumerates rows across every cell/repo, so
	// each row is authorized per-repo here — a viewer with no read capability on
	// a cell's private repos is filtered out of that cell's rows, not merely
	// denied the item detail page.
	SurfaceAuthorizer(r *http.Request) func(owner, name string) bool
}

// Server renders the console.
type Server struct {
	deps Deps
	tmpl *template.Template
}

// New parses the templates and returns the server.
func New(deps Deps) (*Server, error) {
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	tmpl, err := template.New("").Funcs(funcMap()).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("web templates did not parse: %w", err)
	}
	return &Server{deps: deps, tmpl: tmpl}, nil
}

// Handler mounts the read/render routes. Writes are NOT here — they live on the
// internal/answer surface deskd mounts separately.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/inbox", http.StatusFound)
	})
	mux.HandleFunc("GET /inbox", s.handleInbox)
	mux.HandleFunc("GET /item/{id}", s.handleItem)
	// Altitude panes. Each carries the §5.2 ambient strip at its own
	// scope; all reads are over the derived index.
	mux.HandleFunc("GET /roadmap", s.handleRoadmap)
	mux.HandleFunc("GET /epic/{id}", s.handleEpic)
	mux.HandleFunc("GET /measures", s.handleMeasures)
	mux.HandleFunc("GET /portfolio", s.handlePortfolio)
	mux.HandleFunc("GET /teams", s.handleTeams)
	// Ask Assay (the service contract). GET only — a chat answer never becomes a
	// POST, and the pane has no write route to mount.
	mux.HandleFunc("GET /askassay", s.handleAskAssay)
	// Spatial fleet map (the service contract). GET only, by construction: the
	// page is pan/zoom/hover over already-existing read state, and this is
	// the only route this feature registers — see web/fleet_test.go's
	// TestFleetRouteAcceptsOnlyGET and internal/fleet's own AST readonly
	// audit for the two layers that pin it.
	mux.HandleFunc("GET /fleet", s.handleFleet)
	mux.Handle("GET /static/", http.FileServerFS(assets))
	return mux
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"humanAge":   humanAge,
		"humanLogin": humanid.Login,
		"add":        func(a, b int) int { return a + b },
	}
}

// ---- view models -----------------------------------------------------------

type optionVM struct {
	// Digit is the 1-based keyboard binding, derived from the option's POSITION
	// in the typed enum — never from body text (§6.6).
	Digit       int
	ID          string
	Label       string
	Consequence string
}

type blockVM struct {
	Kind  string
	Ident string
}

type rowVM struct {
	ID          string
	Title       string
	Cell        string
	Kind        string
	Urgency     string
	Gold        bool // gold/irreversible: no in-list answer, full item view only (D1)
	Glyph       string
	Word        string
	State       string
	AgeSeconds  int64
	Situation   string
	Options     []optionVM
	OptionsRev  string
	Provenance  string
	PlatformURL string
	Answered    bool
	Unparsed    bool // v2 body that did not parse: peek raw, no digit bindings
	RawBody     string
	Blocks      []blockVM
}

type groupVM struct {
	Name        string
	Rows        []rowVM
	HasUrgent   bool // a group holding an urgent item can never render collapsed
	Collapsible bool
}

type filterVM struct {
	Cell    string
	State   string
	Kind    string
	Urgency string
	View    string // "flat" | "grouped"
	Item    string
}

type inboxVM struct {
	Filter      filterVM
	Flat        bool
	Rows        []rowVM   // flat view
	Groups      []groupVM // grouped view
	Total       int
	UrgentCount int
	// QueryString echoes the applied filter, so links preserve URL state.
	QueryString string
	// WebMCPDev mirrors Deps.WebMCPDev (the service contract). The inbox template
	// is the ONLY template that reads it — the experiment is scoped to this
	// one page state; it must never be threaded into item.html or any view
	// offering the answer-write path (facts: read-only + no mutating view).
	WebMCPDev bool
	// Viewer is the sign-in chrome state: who is signed in. Not a
	// control — the server gates every route.
	Viewer identity.ViewerInfo
}

// ---- handlers --------------------------------------------------------------

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	f := filterVM{
		Cell:    r.URL.Query().Get("cell"),
		State:   r.URL.Query().Get("state"),
		Kind:    r.URL.Query().Get("kind"),
		Urgency: r.URL.Query().Get("urgency"),
		View:    r.URL.Query().Get("view"),
		Item:    r.URL.Query().Get("item"),
	}
	if f.View == "" {
		f.View = "flat" // flat urgency-then-age is the default & guaranteed view
	}

	rows, err := s.deps.DB.Inbox(f.Cell, true) // include closed so Completed is derivable
	if err != nil {
		http.Error(w, "inbox query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	vms := s.toRowVMs(rows)
	vms = s.filterByCapability(r, vms)
	vms = applyFilters(vms, f)

	vm := inboxVM{Filter: f, Total: len(vms), UrgentCount: urgentCount(vms), QueryString: r.URL.RawQuery, WebMCPDev: s.deps.WebMCPDev}
	if s.deps.Viewer != nil {
		vm.Viewer = s.deps.Viewer.SurfaceViewer(r)
	}
	if f.View == "grouped" {
		vm.Flat = false
		vm.Groups = groupRows(vms)
	} else {
		vm.Flat = true
		vm.Rows = vms
	}

	if err := s.tmpl.ExecuteTemplate(w, "inbox.html", vm); err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
	}
}

// filterByCapability drops from the inbox feed every row whose owning repo the
// viewer is not authorized to see (security control — the private-repo gate on
// the surface/listing path). It reuses the guard's per-repo derivation via
// SurfaceAuthorizer; a row whose repo cannot be resolved from its platform
// deep-link is dropped, fail-closed. With no Viewer wired (anonymous public
// deployment / tests) the feed is unfiltered, exactly as before — the surface
// login gate in the middleware still fronts private cells.
func (s *Server) filterByCapability(r *http.Request, rows []rowVM) []rowVM {
	if s.deps.Viewer == nil {
		return rows
	}
	authorized := s.deps.Viewer.SurfaceAuthorizer(r)
	out := make([]rowVM, 0, len(rows))
	for _, row := range rows {
		owner, name := ownerNameFromURL(row.PlatformURL)
		if authorized(owner, name) {
			out = append(out, row)
		}
	}
	return out
}

// itemVM carries one escalation's detail plus its artifact-trail escalation node.
type itemVM struct {
	Row rowVM
	// Trail is the item-detail artifact trail (spec §2.5/§2.6). This build
	// ingests only escalation issues (the read surface sourcesNotRead: no artifact
	// plane yet), so the trail carries the escalation's own node — dashed-gold
	// ⟡ while open, solid with its outcome colour once answered — and a standing
	// note that the surrounding automation artifacts arrive with the plane.
	Trail []trailNodeVM
	// Viewer conditions the affordances this page draws: the
	// comment box at commenter tier, the answer affordances at answerer tier.
	// The server enforces both regardless — this is chrome, NOT a control.
	Viewer identity.ViewerInfo
}

type trailNodeVM struct {
	Label        string
	Glyph        string
	Stroke       string // "dashed" (in-flight) | "solid" (settled)
	Outcome      string // outcome colour class when settled
	IsEscalation bool
}

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows, err := s.deps.DB.Inbox("", true)
	if err != nil {
		http.Error(w, "item query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var found *index.Row
	for i := range rows {
		if string(rows[i].ID) == id {
			found = &rows[i]
			break
		}
	}
	if found == nil {
		http.Error(w, "no escalation with that id", http.StatusNotFound)
		return
	}
	rvm := s.toRowVM(*found)
	vm := itemVM{Row: rvm, Trail: escalationTrail(rvm)}
	if s.deps.Viewer != nil {
		owner, name := ownerNameFromURL(rvm.PlatformURL)
		vm.Viewer = s.deps.Viewer.ViewerFor(r, owner, name)
	}
	if err := s.tmpl.ExecuteTemplate(w, "item.html", vm); err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
	}
}

// ownerNameFromURL extracts owner/name from a GitHub issue deep-link
// (https://github.com/<owner>/<name>/issues/<n>). It is how the chrome learns
// which repo an item belongs to so the viewer's tier can be derived; the ref a
// WRITE acts on is still resolved server-side from the index, never from this.
func ownerNameFromURL(platformURL string) (owner, name string) {
	const marker = "/issues/"
	i := strings.Index(platformURL, marker)
	if i < 0 {
		return "", ""
	}
	segs := strings.Split(strings.TrimPrefix(platformURL[:i], "https://"), "/")
	if len(segs) < 3 {
		return "", ""
	}
	return segs[len(segs)-2], segs[len(segs)-1]
}

// escalationTrail builds the artifact-trail nodes for one escalation. The
// escalation node uses the dashed/solid × colour grammar (spec §2.6): dashed +
// gold ⟡ while open, solid + outcome colour once answered/closed.
func escalationTrail(r rowVM) []trailNodeVM {
	node := trailNodeVM{Label: "escalation filed", Glyph: "⟡", IsEscalation: true}
	if r.Answered {
		node.Stroke = "solid"
		node.Outcome = "settled"
		node.Label = "escalation answered"
	} else {
		node.Stroke = "dashed"
		node.Outcome = "open"
	}
	return []trailNodeVM{node}
}

// ---- derivation helpers ----------------------------------------------------

func (s *Server) toRowVMs(rows []index.Row) []rowVM {
	out := make([]rowVM, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.toRowVM(r))
	}
	return out
}

func (s *Server) toRowVM(r index.Row) rowVM {
	vm := rowVM{
		ID:          string(r.ID),
		Title:       r.Title,
		Cell:        string(r.Cell),
		Kind:        string(r.Kind),
		Urgency:     r.Urgency.String(),
		Gold:        r.Urgency == domain.UrgencyGold,
		AgeSeconds:  r.AgeSeconds,
		Situation:   r.Situation,
		OptionsRev:  r.Rev,
		Provenance:  string(r.Provenance),
		PlatformURL: r.Links["platform"],
		Answered:    r.Answer != nil,
		RawBody:     r.RawBody,
	}
	if r.Attention != nil {
		vm.Glyph, vm.Word, vm.State = r.Attention.Glyph, r.Attention.Word, string(r.Attention.State)
	}
	// A v2 body that did not parse has raw_body and no typed options: peek it
	// raw, mark it unparsed, and bind no digits (§6.6).
	vm.Unparsed = r.RawBody != "" && r.Options == nil
	// Digit bindings derive from the typed enum — the option position — and
	// from nothing the body says.
	for i, o := range r.Options {
		vm.Options = append(vm.Options, optionVM{
			Digit:       i + 1,
			ID:          string(o.ID),
			Label:       o.Label,
			Consequence: o.Consequence,
		})
	}
	for _, b := range r.Blocks {
		vm.Blocks = append(vm.Blocks, blockVM{Kind: b.Kind, Ident: b.Ident})
	}
	return vm
}

func applyFilters(rows []rowVM, f filterVM) []rowVM {
	var out []rowVM
	for _, r := range rows {
		if f.State != "" && r.State != f.State {
			continue
		}
		if f.Kind != "" && r.Kind != f.Kind {
			continue
		}
		if f.Urgency != "" && r.Urgency != f.Urgency {
			continue
		}
		out = append(out, r)
	}
	return out
}

// urgentCount counts urgent-and-above rows. It renders in the inbox header,
// group-independent (D5 guard).
func urgentCount(rows []rowVM) int {
	n := 0
	for _, r := range rows {
		if r.Urgency == domain.UrgencyUrgent.String() || r.Urgency == domain.UrgencyGold.String() {
			n++
		}
	}
	return n
}

func isUrgent(r rowVM) bool {
	return r.Urgency == domain.UrgencyUrgent.String() || r.Urgency == domain.UrgencyGold.String()
}

// groupRows groups by the derived attention word (an honest grouping over what
// this build derives). A group holding an urgent item is forced non-collapsible
// (D5 no-collapse-with-urgent guard). Within-group order is preserved from the
// urgency-then-age sort DB.Inbox already applied.
func groupRows(rows []rowVM) []groupVM {
	order := []string{"waiting on you", "working", "idle", "silent", "failed-to-start"}
	byWord := map[string][]rowVM{}
	seen := map[string]bool{}
	for _, r := range rows {
		w := r.Word
		if w == "" {
			w = "unresolved"
		}
		byWord[w] = append(byWord[w], r)
		seen[w] = true
	}
	var groups []groupVM
	emit := func(word string) {
		rs := byWord[word]
		if len(rs) == 0 {
			return
		}
		g := groupVM{Name: word, Rows: rs, Collapsible: true}
		for _, r := range rs {
			if isUrgent(r) {
				g.HasUrgent = true
				g.Collapsible = false // never collapsed while it holds an urgent item
			}
		}
		groups = append(groups, g)
		delete(seen, word)
	}
	for _, w := range order {
		emit(w)
	}
	// Any words not in the fixed order, alphabetically, so the set is total.
	var rest []string
	for w := range seen {
		rest = append(rest, w)
	}
	sort.Strings(rest)
	for _, w := range rest {
		emit(w)
	}
	return groups
}

// humanAge renders an age the way the console does (compact, honest).
func humanAge(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
