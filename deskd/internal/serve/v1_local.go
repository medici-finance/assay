// v1_local.go — the `/v1` LOCAL-STATE read marker (the service contract increment 4).
//
// # What this closes
//
// internal/contract declares `POST /v1/escalations/{id}/read` as a
// [contract.KindLocalState] route (ruling D7: deskd's own SQLite, no platform
// write), the generated OpenAPI documents it, and `contract.Client.Read` calls
// it — but until this increment [NewV1] never mounted it, so the
// no-general-write catch-all answered every Client.Read with a 404. The
// declaration and the mounted table diverged, the same shape the 2026-09-06
// audit bounced for the named actions.
//
// # Gated exactly like the un-versioned marker, never looser
//
// The un-versioned `POST /escalations/{id}/read` mounts in deskd's protected
// table as an ITEM-scoped route with Min=Commenter and no anonymous shortcut:
// an unauthenticated caller is refused (401) even on a cell whose repos are a
// verified anonymous-read opt-in, and an authenticated one needs Commenter on
// the item's repo. `/v1/` is mounted SURFACE-scoped at the tree (the per-item
// decision is each handler's), so the surface login gate alone would let an
// anonymous caller write the marker on an anonymous-opt-in cell. This handler
// therefore asks the per-item question itself through [LocalStateAuthorizer] —
// the same derivation the item-scoped gate runs — and the route is not mounted
// at all when no such authorizer is wired: absent (a 404), never present and
// ungated.
package serve

import (
	"net/http"
	"strings"

	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/local"
)

// LocalStateAuthorizer answers the per-item question a local-state write on the
// versioned seam needs: is this request's viewer AUTHENTICATED, and does it
// derive at least Commenter on owner/name? It is deliberately stricter than
// [SurfaceAuthorizer]'s read predicate, which also admits an anonymous viewer on
// a verified anonymous-read repo — a read marker is a write, and no write is
// ever reachable anonymously. identity.Guard implements it.
type LocalStateAuthorizer interface {
	LocalStateAllowed(r *http.Request, owner, name string) (authenticated, allowed bool)
}

// mountV1Read registers POST /v1/escalations/{id}/read when — and only when —
// the index and a [LocalStateAuthorizer] are both wired. authz is the same
// value [NewV1] received; a deployment whose authorizer cannot answer the
// per-item write question leaves the route to the no-general-write catch-all.
func mountV1Read(mux *http.ServeMux, db *index.DB, authz SurfaceAuthorizer) {
	la, ok := authz.(LocalStateAuthorizer)
	if !ok || la == nil || db == nil {
		return
	}
	mux.HandleFunc("POST "+contract.Prefix+"/escalations/{id}/read", handleV1Read(db, la, local.New(db)))
}

// handleV1Read marks one escalation read in deskd's local table. The item is
// resolved against the TRUSTED index (open or closed rows — the same lookup
// the item-scoped gate's repo resolver makes); the repo it is authorized on
// comes from that row's platform link, never from the request. It answers 204
// with no body, as the contract documents; nothing is written on the platform.
func handleV1Read(db *index.DB, authz LocalStateAuthorizer, wr *local.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := domain.EscalationID(r.PathValue("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "escalation id is required in the path"})
			return
		}
		// A failed index read leaves owner/name empty, which the authorizer
		// fails closed on (never allowed); the failure itself is reported only
		// AFTER authentication, so an unauthenticated caller gets the same 401
		// as every other and never sees index state.
		rows, idxErr := db.Inbox("", true)
		var owner, name string
		found := false
		for _, row := range rows {
			if row.ID != id {
				continue
			}
			if ref, ok := answer.ParseRef(row.Links["platform"]); ok {
				owner, name, found = strings.Cut(ref.Repo, "/")
			}
			break
		}
		authenticated, allowed := authz.LocalStateAllowed(r, owner, name)
		switch {
		case !authenticated:
			// Same order as the item-scoped gate: an unauthenticated caller
			// learns nothing about whether the id exists, nor whether the
			// index could be read.
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "sign in with GitHub to mark this read", "login_at": "/auth/login"})
			return
		case idxErr != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "item index read failed (BLIND, not empty): " + idxErr.Error()})
			return
		case !found:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such item in the index"})
			return
		case !allowed:
			writeJSON(w, http.StatusForbidden, map[string]string{"error": `marking an item read needs "commenter" on its repo`})
			return
		}
		if err := wr.MarkRead(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "the local read write failed: " + err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
