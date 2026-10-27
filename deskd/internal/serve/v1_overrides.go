// v1_overrides.go — the cell-override routes on the `/v1` seam
// (the service contract, ruling §10.5).
//
// # What these routes are, and what they are NOT
//
// GET /v1/cells/{id}/overrides is a READ (its MCP twin is desk__overrides).
// POST /v1/cells/{id}/overrides is KindLocalState: it writes deskd's OWN
// SQLite override table and files a record PR under a SEPARATE App — it is NOT
// a human-authored platform write, so it is not in contract.NamedActions and
// the closed allow-list (TestNamedActionSetIsClosed) is untouched.
//
// # The tier gate
//
// The POST requires the ANSWERER tier — the cell's answer_as login, the highest
// tier the ladder has (internal/identity has no `admin`, resolved on the brief).
// The tier is named in ONE place ([overridesPostTier]) so promoting it to a
// future admin tier is a one-line change. The gate itself is injected by
// cmd/deskd ([Overrides.Answerer]) from the identity session store; this package
// stays free of the session machinery, exactly as it takes the surface
// authorizer as an interface rather than importing it.
//
// # The record PR degrades honestly
//
// The record PR is filed only when the write-issues App is configured
// ([mcp.RecordFiler.RecordIdentity] non-empty). When it is not, the table is
// still written, NO PR is attempted, and the response says
// `record_pr: could-not-check` — the filer NEVER falls back to the read App or
// a person's token (the brief's row 7 negative path).
package serve

import (
	"encoding/json"
	"mime"
	"net/http"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/local"
	"github.com/medici-finance/assay/deskd/internal/mcp"
)

// overridesPostTier is the SINGLE place the POST's required tier is named. It is
// the answerer tier today (internal/identity.Answerer, the cell's answer_as);
// promoting it to a future admin tier is a one-line change here plus the gate
// cmd/deskd supplies.
const overridesPostTier = "answerer"

// Overrides is the wiring the cell-override routes need. nil (or a zero Store)
// leaves the routes UNMOUNTED — a 404, the read-only-deployment shape, never a
// route that exists and refuses.
type Overrides struct {
	// Store is the local override table (deskd's own SQLite).
	Store *local.OverrideStore
	// Filer files the record PR mirroring an override to cells.yaml. It may be
	// nil or unconfigured (RecordIdentity()==""), which is could-not-check.
	Filer mcp.RecordFiler
	// Answerer resolves the request's authenticated viewer login and whether it
	// is the cell's answerer. login=="" means no authenticated session (401);
	// login!="" && !isAnswerer means an authenticated non-answerer (403).
	Answerer func(r *http.Request, cell string) (login string, isAnswerer bool)
	// Cell resolves a cell by name — for its model ladder (POST validation) and
	// nothing else here.
	Cell func(cell string) (*schema.Cell, bool)
	// ConfigRepo (owner/name) and ConfigPath name where the cell's cells.yaml
	// lives: the record PR's target.
	ConfigRepo string
	ConfigPath string
}

func (o *Overrides) ready() bool { return o != nil && o.Store != nil }

// mountOverrides registers the two override routes on the v1 top mux. Both are
// registered with an explicit method so the read tree's methodGuard (which
// 405s non-GET) never sees the POST, exactly as the named-action writes are
// registered.
//
// Both run behind gate, the per-cell access decision the rest of the /v1 cell
// routes use (404 unknown cell, 403 no read capability on its repos): the
// answerer tier on the POST is a SECOND check, not the only one, and the GET
// had none before it.
func mountOverrides(mux *http.ServeMux, o *Overrides, gate func(http.ResponseWriter, *http.Request, string) bool) {
	if !o.ready() {
		return
	}
	gated := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if cell := r.PathValue("id"); cell != "" && !gate(w, r, cell) {
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET "+contract.Prefix+"/cells/{id}/overrides", gated(o.handleGet))
	mux.HandleFunc("POST "+contract.Prefix+"/cells/{id}/overrides", gated(o.handlePost))
}

// handleGet serves the cell's current override table plus its as_of.
func (o *Overrides) handleGet(w http.ResponseWriter, r *http.Request) {
	cell := r.PathValue("id")
	if cell == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cell id is required in the path"})
		return
	}
	ov, asOf, err := o.Store.Get(cell)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "reading the override table failed (BLIND, not empty): " + err.Error()})
		return
	}
	resp := contract.OverridesResponse{Cell: cell, Overrides: ov}
	resp.AsOf = asOf
	writeJSON(w, http.StatusOK, resp)
}

// handlePost sets a cell's overrides. It gates on the answerer tier, validates
// with the SAME validator the schema uses, writes the local table, then files
// the record PR (could-not-check when the write-issues App is unconfigured).
func (o *Overrides) handlePost(w http.ResponseWriter, r *http.Request) {
	cell := r.PathValue("id")
	if cell == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cell id is required in the path"})
		return
	}

	// ── the body must be declared JSON ────────────────────────────────────
	// A form or text/plain body is a "simple" cross-site request a browser
	// sends with no preflight; application/json is not.
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "the overrides body must be sent as Content-Type: application/json"})
		return
	}

	// ── the answerer gate ────────────────────────────────────────────────
	var login string
	var isAnswerer bool
	if o.Answerer != nil {
		login, isAnswerer = o.Answerer(r, cell)
	}
	if login == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "setting cell overrides requires an authenticated session; this request carried no identity",
		})
		return
	}
	if !isAnswerer {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "setting cell overrides requires the " + overridesPostTier + " tier — the cell's answer_as login; a commenter may read overrides but not set them",
		})
		return
	}

	// ── decode + validate (one validator, shared with the schema) ─────────
	var in domain.Overrides
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body did not decode as an overrides object: " + err.Error()})
		return
	}
	if in.IsEmpty() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no override keys set — the body must carry at least one of dispatch_width, hold_merges, pause_verify, model_floor, quiet_hours"})
		return
	}
	var models []string
	if o.Cell != nil {
		if c, ok := o.Cell(cell); ok {
			models = c.ModelLadderIDs()
		}
	}
	if err := domain.ValidateOverrides(in, models); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// ── write the local table (set_by = the answerer login) ───────────────
	merged, asOf, err := o.Store.Set(cell, in, login)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "writing the override table failed: " + err.Error()})
		return
	}

	// ── file the record PR, or record could-not-check ─────────────────────
	recordStatus := "could-not-check (write-issues App not configured)"
	if o.Filer != nil && o.Filer.RecordIdentity() != "" {
		res, ferr := o.Filer.FileRecordPR(r.Context(), mcp.RecordPRRequest{
			Cell: cell, Repo: o.ConfigRepo, ConfigPath: o.ConfigPath, Overrides: merged,
		})
		if ferr != nil {
			recordStatus = "record PR filing failed: " + ferr.Error()
		} else {
			recordStatus = res.Status
		}
	}

	resp := contract.OverridesResponse{Cell: cell, Overrides: merged, RecordPR: recordStatus}
	resp.AsOf = asOf
	writeJSON(w, http.StatusOK, resp)
}
