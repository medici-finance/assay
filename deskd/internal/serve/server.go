// Package serve is deskd's HTTP read surface.
//
// # Read-only, structurally
//
// The service contract ships no write path of any kind. That is enforced
// here in two places, neither of which is a convention:
//
//  1. [Routes] is the ONE route table. Every entry is a [Route] whose only
//     verb field is a bool that is not settable to anything but GET — the
//     type has no method field at all, so a handler cannot be registered
//     against POST/PUT/PATCH/DELETE because there is nowhere to say so.
//  2. [New] wraps the mux in [methodGuard], which answers 405 to anything
//     but GET/HEAD before a handler sees it.
//
// A sibling decision on this stream refused a dormant-but-built write path
// because *the gap between a flag and a grant is one commit*. The same
// reasoning is why there is no disabled-by-default writable route here: a
// route that exists can be enabled, and a route type that cannot express a
// write cannot be enabled into one. The answer path is the service contract; the
// agent write path is the service contract.
//
// `TestSurfaceIsReadOnly` walks [Routes] and the live mux and fails on any
// non-GET reachable path, so the property is checked on every CI run rather
// than argued in a review.
package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// Route is a registered read endpoint. Note what this struct does NOT have:
// a method field. Every route is a GET, by the shape of the type.
type Route struct {
	Path    string
	Summary string
	handler func(*Server, http.ResponseWriter, *http.Request)
}

// Routes is the complete HTTP surface of this build.
var Routes = []Route{
	{Path: "/inbox", Summary: "escalations for a cell, urgency-then-age, with index age and per-field confidence",
		handler: (*Server).handleInbox},
	{Path: "/cells", Summary: "configured cells and their repos, with per-repo health",
		handler: (*Server).handleCells},
	{Path: "/roadmap", Summary: "SKELETON — declares its delegated source and renders could-not-check until wired",
		handler: (*Server).handleRoadmap},
	{Path: "/healthz", Summary: "per-repo three-state health and the last poll outcome",
		handler: (*Server).handleHealth},
}

// SurfaceAuthorizer returns a per-request predicate answering whether the
// request's viewer may see a given (owner, name) repo's rows on the multi-repo
// inbox feed. identity.Guard satisfies it. It is the JSON surface's private-repo
// gate on the listing path: the machine surface enumerates rows across every
// cell/repo, so each row is authorized per-repo — a viewer with no read
// capability on a cell's private repos is filtered out of that cell's rows.
// Keeping it an interface means this package takes no dependency on the
// session/token machinery, only on the per-request predicate.
type SurfaceAuthorizer interface {
	SurfaceAuthorizer(r *http.Request) func(owner, name string) bool
}

// Server serves the derived index. It holds no GitHub client: the serving
// path cannot reach the network, so it cannot answer from anywhere but the
// index it claims to be serving.
type Server struct {
	DB     *index.DB
	Config *schema.File
	// Now is injectable so tests can pin index age.
	Now func() time.Time
	// Authz is the surface/listing private-repo gate (the capability model security fix):
	// it filters the inbox feed to only the repos the request's viewer may see.
	// Optional: nil leaves the feed unfiltered (the middleware's surface login
	// gate still fronts private cells), which is the anonymous-public / test
	// shape; the deskd wiring always sets it to the capability guard.
	Authz SurfaceAuthorizer
	// Terminal is the cockpit terminal registry (the service contract): desk id →
	// handle, resolved from cells.yaml. Only the versioned surface reads it, and
	// only for GET /v1/desks/{id}/terminal. Optional: nil means the terminal
	// read is could-not-check on this build (never a crash).
	Terminal *terminal.Registry
}

// New builds the handler. authz is the per-repo surface authorizer (the
// capability guard in the deskd wiring); pass nil for an unfiltered feed.
//
// These un-versioned paths are DEPRECATED aliases as of the service contract: the
// versioned `/v1` surface ([NewV1]) is the contract a client host reads. They
// keep serving for one release — with a Deprecation disclosure header on every
// response (see [deprecatedAlias]) — so an existing reader is not broken by the
// split, then are removed in a later increment.
func New(db *index.DB, cfg *schema.File, authz SurfaceAuthorizer) http.Handler {
	s := &Server{DB: db, Config: cfg, Now: func() time.Time { return time.Now().UTC() }, Authz: authz}
	mux := http.NewServeMux()
	for _, r := range Routes {
		h := r.handler
		// The pattern is registered WITHOUT a method prefix and the guard
		// below rejects non-GET, so http.ServeMux's method routing cannot
		// be used to smuggle one in.
		mux.HandleFunc(r.Path, func(w http.ResponseWriter, req *http.Request) { h(s, w, req) })
	}
	return methodGuard(deprecatedAlias(mux))
}

// deprecatedAlias stamps the standard Deprecation disclosure on every
// un-versioned read response (the service contract). RFC 8594 "Deprecation" +
// "Link" tell a reader where the supported surface is; the split keeps these
// paths serving for one release so no existing client breaks the day the
// service is cut, and the header makes the deprecation visible rather than a
// silent future 404.
func deprecatedAlias(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Link", `</v1`+r.URL.Path+`>; rel="successor-version"`)
		next.ServeHTTP(w, r)
	})
}

// methodGuard rejects every verb that is not a read, before routing.
func methodGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "deskd (the service contract) is read-only: no endpoint on this build accepts a write. " +
					"The human-gate answer path is the service contract; the agent write path is the service contract.",
				"method": r.Method,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// envelope wraps every response with the staleness disclosure design §10
// requires: "every view carries as-of <t>. Staleness is displayed, not
// hidden — the watermelon defense applies to the Console itself."
//
// index_age_seconds is a POINTER so a cold index serialises as null rather
// than 0. A zero age on an index that has never polled reads as "fresh",
// which is the single worst thing a blank board can say.
type envelope struct {
	AsOf            *time.Time           `json:"as_of"`
	IndexAgeSeconds *int64               `json:"index_age_seconds"`
	IndexAge        domain.Assessment    `json:"index_age_confidence"`
	Health          []adapter.RepoHealth `json:"repo_health"`
	// SourcesNotRead is the standing disclosure of what this index does NOT
	// look at, so a consumer never infers coverage from silence.
	SourcesNotRead []string `json:"sources_not_read"`
	Data           any      `json:"data"`
}

// sourcesNotRead is the honest boundary of this build.
//
// Every entry is a source a reader might reasonably ASSUME the inbox
// consults. Listing them is cheaper than the alternative — a consumer
// deciding from an empty inbox that nothing is wrong anywhere.
var sourcesNotRead = []string{
	"pull requests, their reviews, and their review decisions — no PR state is derived here at all",
	"CI check runs and statuses — a 0-check PR is not visible to this index",
	"`statusgen --json` / `--dora` / `--gate-scores` — the roadmap route declares them as its delegated source and is a skeleton in this build",
	"`deskboard actions|prs|health|awaiting` — no board verb is consulted; nothing here re-derives what they already derive",
	"issue comments — only the issue body and labels are read, so an answer left in a comment is not seen until the answer flow",
	"closed escalations — the inbox serves open rows; a closed one is out of window, not absent",
	"Kubernetes pod phase — the platform-assigned liveness sensor arrives with §9.3 phase-2 pods; the attention derivation prefers it where a pod record exists (the service contract upgrade path), but this build ingests none",
	"any artifact or dispatch stream other than escalation issues — the service contract lands the attention derivation and its first-artifact deadline as a closed pure function, but only escalation issues are ingested here, so a live inbox row derives `waiting` (open escalation) or unresolved, and the richer working/idle/silent/failed-to-start states arrive with the Portfolio strip and the pod substrate",
}

func (s *Server) envelope(data any) envelope {
	now := s.Now()
	age, assessment := s.DB.IndexAge(now)
	env := envelope{IndexAge: assessment, SourcesNotRead: sourcesNotRead, Data: data}
	if assessment.Confidence != domain.CouldNotCheck {
		secs := int64(age.Seconds())
		asOf := now.Add(-age)
		env.IndexAgeSeconds = &secs
		env.AsOf = &asOf
	}
	if h, err := s.DB.Health(); err == nil {
		env.Health = h
	}
	return env
}

// InboxPayload is the /inbox body.
type InboxPayload struct {
	Cell  string      `json:"cell"`
	Items []index.Row `json:"items"`
	// Total is a domain.Count, so it carries its probe and refuses to be a
	// bare number. It counts SERVED rows — rows from a repo that could not
	// be reached this cycle are the previous cycle's, and the repo_health
	// block says which.
	Total domain.Count `json:"total"`
	// Unresolved counts served rows carrying at least one could-not-check
	// field. A queue with unresolved rows is not a clean queue.
	Unresolved domain.Count `json:"unresolved"`
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	cell := r.URL.Query().Get("cell")
	rows, err := s.DB.Inbox(cell, false)
	probe := domain.Probe{
		Source: "derived_escalations (SQLite) — the index only; the serving path makes no network call",
		Method: domain.MethodLocalTable,
		Window: "open escalations of cell " + orAll(cell),
		Limit:  0,
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, s.envelope(InboxPayload{
			Cell:       cell,
			Items:      []index.Row{},
			Total:      domain.NoCount(probe, "the index query failed: "+err.Error()),
			Unresolved: domain.NoCount(probe, "the index query failed: "+err.Error()),
		}))
		return
	}
	if rows == nil {
		rows = []index.Row{}
	}
	rows = s.filterByCapability(r, rows)
	unresolved := 0
	for _, row := range rows {
		if len(row.Fields.Unresolved()) > 0 {
			unresolved++
		}
	}
	writeJSON(w, http.StatusOK, s.envelope(InboxPayload{
		Cell:       cell,
		Items:      rows,
		Total:      domain.NewCount(len(rows), probe),
		Unresolved: domain.NewCount(unresolved, probe),
	}))
}

// filterByCapability drops from the inbox feed every row whose owning repo the
// request's viewer is not authorized to see (the private-repo gate on the
// surface/listing path). It resolves each row's repo from its platform
// deep-link against the same trusted parse the write paths use
// ([answer.ParseRef]) and asks the capability authorizer; a row whose repo
// cannot be resolved is dropped, fail-closed. With no Authz wired the feed is
// unfiltered, exactly as before — the middleware's surface login gate still
// fronts private cells.
func (s *Server) filterByCapability(r *http.Request, rows []index.Row) []index.Row {
	if s.Authz == nil {
		return rows
	}
	authorized := s.Authz.SurfaceAuthorizer(r)
	out := make([]index.Row, 0, len(rows))
	for _, row := range rows {
		ref, ok := answer.ParseRef(row.Links["platform"])
		if !ok {
			continue // repo not resolvable: fail closed
		}
		owner, name, ok := strings.Cut(ref.Repo, "/")
		if !ok || !authorized(owner, name) {
			continue
		}
		out = append(out, row)
	}
	return out
}

// CellsPayload is the /cells body.
type CellsPayload struct {
	Cells []CellSummary `json:"cells"`
}

// CellSummary describes one configured cell.
type CellSummary struct {
	Name             string   `json:"name"`
	Repos            []string `json:"repos"`
	Loops            []string `json:"loops"`
	SilenceThreshold string   `json:"silence_threshold"`
}

func (s *Server) handleCells(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, s.envelope(CellsPayload{Cells: out}))
}

// RoadmapPayload is the /roadmap skeleton.
//
// The route exists because the read surface asks for the skeleton; the DATA does
// not, because this build derives no roadmap. Rather than serve an empty
// array — which reads as "no work" — it serves an explicit could-not-check
// naming the delegated source it will consume. derive-or-diff: statusgen is
// the single declared source for stream status, and deskd renders rather
// than computing a rival board (design §5).
type RoadmapPayload struct {
	Streams    []any             `json:"streams"`
	Assessment domain.Assessment `json:"assessment"`
}

func (s *Server) handleRoadmap(w http.ResponseWriter, r *http.Request) {
	p := domain.Probe{
		Source: "statusgen --json over each cell's streams_root (the single writer of status; deskd renders, never re-derives)",
		Method: domain.MethodDelegated,
		Window: "every stream README under the cell's streams_root",
		Limit:  0,
	}
	writeJSON(w, http.StatusOK, s.envelope(RoadmapPayload{
		Streams: nil,
		Assessment: domain.Unchecked(p,
			"the service contract lands the route, not the derivation: nothing in this build invokes the delegated source, "+
				"so the roadmap is could-not-check rather than empty. An empty stream list would assert there is no work."),
	}))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.envelope(map[string]string{"surface": "read-only"}))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		// The status line is already written; the best remaining act is to
		// make the truncation visible rather than serve half a document
		// that parses.
		fmt.Fprintf(w, "\n{\"error\":\"response encoding failed: %v\"}\n", err)
	}
}

func orAll(cell string) string {
	if cell == "" {
		return "(all cells)"
	}
	return cell
}
