package web

import (
	"context"
	"net/http"
	"sort"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/fleet"
)

// ---- Spatial fleet map (the service contract, one-week experiment) -----------
//
// A read-only canvas view over the same sources the sweep-based reports
// read: refs/dispatch/* claims and open PRs, grouped by repo lane, state-
// colored by their existing state. It is a RENDERING — it derives nothing itself, the
// same doctrine every other pane in this package follows — and it ships no
// write route: this file registers no route but GET, and every field a
// template can reach is either a token count/age/state string or html/
// template-escaped text, never template.HTML (§6.6 doctrine, this
// package's doc comment).
//
// Staleness is the load-bearing property (brief §single-point-of-failure):
// [fleet.Cache] is what tells a killed fetch from a live one, so this
// handler never invents freshness — it renders exactly the Stale/FetchedAt
// [fleet.RepoLane] gives it.

// FleetSource is the fleet-map read client. It is an interface for the same
// reason [AskAssay] is: the web layer takes no dependency on the GitHub
// token or the per-org routing table that builds it, only on the rendered
// [fleet.RepoLane] shape. Nil — the default — renders the page in its
// could-not-check state, same as an unconfigured Chat.
type FleetSource interface {
	Snapshot(ctx context.Context, repos []string) []fleet.RepoLane
}

// fleetVM is the /fleet page.
type fleetVM struct {
	OK    bool
	Lanes []fleetLaneVM
}

type fleetLaneVM struct {
	Repo      string
	OK        bool
	Reason    string
	Stale     bool
	FetchedAt string // RFC3339, for the age badge's title attribute
	FetchAge  int64  // seconds, for humanAge
	Tokens    []fleetTokenVM
}

type fleetTokenVM struct {
	Kind    string // "claim" | "pr"
	Key     string
	Title   string
	Holder  string
	AgeSecs int64
	State   string // the four-bucket token, for data-state
	Palette string // healthy | neutral | amber | fault, for data-palette
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	if s.deps.Fleet == nil {
		s.render(w, "fleet.html", fleetVM{OK: false})
		return
	}
	repos := configuredRepoSlugs(s.deps.Config)
	if len(repos) == 0 {
		s.render(w, "fleet.html", fleetVM{OK: false})
		return
	}

	now := s.deps.Now()
	lanes := s.authorizedLanes(r, s.deps.Fleet.Snapshot(r.Context(), repos))

	vm := fleetVM{OK: true, Lanes: make([]fleetLaneVM, 0, len(lanes))}
	for _, l := range lanes {
		lvm := fleetLaneVM{
			Repo: l.Repo, OK: l.OK, Reason: l.Reason, Stale: l.Stale,
			FetchAge: int64(l.FetchAge(now).Seconds()),
		}
		if !l.FetchedAt.IsZero() {
			lvm.FetchedAt = l.FetchedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		for _, t := range l.Tokens {
			lvm.Tokens = append(lvm.Tokens, fleetTokenVM{
				Kind: string(t.Kind), Key: t.Key, Title: t.Title, Holder: t.Holder,
				AgeSecs: int64(t.Age.Seconds()), State: string(t.State), Palette: t.Palette(),
			})
		}
		vm.Lanes = append(vm.Lanes, lvm)
	}
	s.render(w, "fleet.html", vm)
}

// authorizedLanes drops from the fleet map every lane whose repo the viewer is
// not authorized to see. The map is a surface-scoped multi-repo listing
// rendered under the desk's per-org read token (which sees every private repo),
// and the /fleet route is ItemScoped:false, so the middleware does only a
// login-presence check — per-repo authorization on this listing is the
// handler's job. So, exactly like the inbox feed (filterByCapability) and the
// epic/portfolio panes (authorizedRows), each lane is gated per-repo through the
// viewer's capability, dropping any lane whose PR titles/authors or
// refs/dispatch/* claim keys+holders the viewer holds no read on — a cross-tenant
// leak otherwise. It reuses the guard's SurfaceAuthorizer, the SAME deskd-side
// derivation those panes use — never a parallel check — and adapter.ParseRepo to
// resolve owner/name from the lane's "owner/name" slug; a lane whose slug will
// not parse is dropped, fail-closed. With no Viewer wired (anonymous-public
// deployment / tests) the lanes are returned unfiltered, exactly as before — the
// middleware's surface login gate still fronts private cells.
func (s *Server) authorizedLanes(r *http.Request, lanes []fleet.RepoLane) []fleet.RepoLane {
	if s.deps.Viewer == nil {
		return lanes
	}
	authorized := s.deps.Viewer.SurfaceAuthorizer(r)
	out := make([]fleet.RepoLane, 0, len(lanes))
	for _, l := range lanes {
		repo, err := adapter.ParseRepo(l.Repo)
		if err != nil {
			continue // unresolvable slug — fail closed
		}
		if authorized(repo.Owner, repo.Name) {
			out = append(out, l)
		}
	}
	return out
}

// configuredRepoSlugs collects every repo named across every cell, deduped
// and sorted — the SAME repo set the escalation poller and the pr-review /
// verify sweeps already read (brief §facts: "the map reads the same
// sources the sweep reports read (no second source of truth)").
func configuredRepoSlugs(cfg *schema.File) []string {
	if cfg == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range cfg.Cells {
		for _, slug := range c.Repos {
			if _, err := adapter.ParseRepo(slug); err != nil {
				continue // not a valid owner/name slug; skip rather than guess
			}
			if !seen[slug] {
				seen[slug] = true
				out = append(out, slug)
			}
		}
	}
	sort.Strings(out)
	return out
}
