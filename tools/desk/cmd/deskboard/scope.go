package main

// scope.go — what the board COVERS, stated in-band on every sweep, plus the
// reconciliation verb that checks the compiled-in repo set against the owners'
// ACTUAL repos.
//
// The defect this closes: deskboard builds its watch list from a compiled-in set and
// renders a confident, complete-looking board of a subset. "No PRs in that repo" and
// "that repo is not read at all" produce the same output — an empty row set — so a
// 12-day-old approved PR in an unwatched repo is not missed, it is structurally
// invisible, and nothing knows to alert.
//
// Two halves, per the issue's own ranking (its point 2 matters more than its point 1,
// because the list WILL drift again and the failure mode is "nobody can tell"):
//
//  1. Every verb that SWEEPS THE REPO SET carries `scope` in its header and prints one
//     scope line on the table path. Verbs that do not sweep the repo set OMIT the field
//     entirely, so an absent `scope` means "this verb covered no REPO set", never "the
//     set was empty" (the #377 three-state rule: a field that was not probed is absent,
//     not green).
//
//     `scope` is the REPO axis specifically, and two different kinds of verb omit it
//     (#400 N3 — the rule as first written read as if only the first kind existed):
//
//     - reviews / diff / files take an explicit repo and sweep nothing at all;
//     - awaiting / nextup DO sweep, but they sweep the configured statusgen ROOTS, not
//     the repo set. A repo-axis `scope` there would claim a coverage they do not have,
//     so they state their coverage on their own axis instead: `roots` carries every
//     root that was read, in full and resolved, and the table path prints one line per
//     root. An empty board from those verbs is therefore still distinguishable from a
//     root that was never read.
//
//     TestDispatch_VerbInventory holds every dispatcher verb to whichever of these
//     three classes it is declared to be in, and fails on a verb declared in none.
//
//  2. `deskboard scope` reconciles: it lists the watched set, then asks GitHub which
//     repos under the SAME owners actually have open PRs, and reports every repo with
//     open PRs that the board does not watch. A gap becomes a row instead of a silence.
//     A read it could not perform is exit 6 (could-not-check), never "no gaps".

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// searchLimit caps `gh search prs` explicitly. gh's own default is THIRTY, which on a
// reconciliation read is the same silent-truncation class the board already fixed for
// `gh pr list` (#80). A result set at the cap is reported as possibly truncated rather
// than treated as complete.
const searchLimit = 200

// BoardScope is the in-band statement of what a sweeping verb covered. It rides in the
// header of prs / actions / queue and is ABSENT everywhere else.
type BoardScope struct {
	Repos  []string `json:"repos"`
	Count  int      `json:"count"`
	Source string   `json:"source"`
}

// boardScope builds the scope statement from the one compiled-in set every sweeping
// verb iterates (deskkit.AllowedRepos). It is derived, never a second hand-kept list —
// a scope footer that could disagree with the loop it describes would be worse than
// none.
func boardScope() *BoardScope {
	repos := deskkit.AllowedRepos()
	return &BoardScope{
		Repos:  repos,
		Count:  len(repos),
		Source: "compiled-in fixed set (deskkit); no flag or env widens it",
	}
}

// renderScopeLine prints the one-line scope footer on the table path. It is
// unconditional for a sweeping verb — the whole point is that a reader can never infer
// coverage from the rows — but it is exactly ONE line, so a healthy board pays one line
// for it (the #377 noise budget). A nil scope prints nothing: the verb did not sweep.
//
// That nil arm is UNREACHABLE from any current caller — every sweeping verb passes
// boardScope(), built from a non-empty compiled-in map — and nothing pins it. It exists
// so a verb added later that legitimately does not sweep prints nothing rather than an
// empty coverage claim; it is a guard, not a tested guarantee.
func renderScopeLine(w io.Writer, s *BoardScope) {
	if s == nil {
		return
	}
	shorts := make([]string, 0, len(s.Repos))
	for _, r := range s.Repos {
		shorts = append(shorts, shortRepo(r))
	}
	fmt.Fprintf(w, "scope: %d watched repo(s) — %s — repos OUTSIDE this set are not read by this board "+
		"(reconcile with `deskboard scope`)\n", s.Count, strings.Join(shorts, ", "))
}

// ---------------------------------------------------------------------------
// `deskboard scope` — the reconciliation verb
// ---------------------------------------------------------------------------

// searchPR is one row of `gh search prs --json`. repository.nameWithOwner is the field
// the reconciliation keys on; a row that arrives without it fails the run (exit 6)
// rather than being attributed to nothing and silently dropped — an unattributable
// result is precisely the "could not see it" case this verb exists to surface.
type searchPR struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	CreatedAt  string `json:"createdAt"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

// unwatchedRepo is a repo with open PRs that the board does not watch — the gap made
// into a row.
type unwatchedRepo struct {
	Repo      string `json:"repo"`
	OpenPRs   int    `json:"openPRs"`
	OldestAge string `json:"oldestOpenAge,omitempty"`
	OldestPR  int    `json:"oldestPR,omitempty"`
	Sample    []int  `json:"samplePRs,omitempty"`
}

type scopeReport struct {
	Header
	Owners        []string        `json:"owners"`
	Watched       []string        `json:"watched"`
	ObservedRepos int             `json:"observedRepos"`
	ObservedPRs   int             `json:"observedPRs"`
	Truncated     bool            `json:"truncated"`
	Gap           bool            `json:"gap"`
	Unwatched     []unwatchedRepo `json:"unwatched"`
	// ObservabilityBound states what this reconciliation could NOT have seen. gap=false
	// means "no unwatched repo was observed", which is a strictly weaker claim than "no
	// unwatched repo exists"; a machine reader deciding off `gap` has to be told the
	// difference in-band, the same way the table line is.
	ObservabilityBound string `json:"observabilityBound"`
}

// observabilityBound is the constant bound on what `deskboard scope` can see. It is
// derived from the mechanism (a `gh search prs` under the caller's token, owner-scoped),
// so it cannot drift away from the search it describes.
const observabilityBound = "observed via `gh search prs` under the caller's token and scoped to the owners " +
	"listed in `owners`; a repo this token cannot read (private, App not installed, org SSO not authorised) " +
	"returns no rows and no error, and a repo under any other owner is not asked about at all — neither is " +
	"counted in `unwatched`, so gap=false means NO GAP WAS OBSERVED, not NO GAP EXISTS"

// ownersOf derives the owner set from the watched repos themselves. Deriving it means
// there is no second list to drift: the verb asks about exactly the accounts the board
// already claims to cover.
func ownersOf(repos []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range repos {
		owner, _, ok := strings.Cut(r, "/")
		if !ok || owner == "" {
			continue
		}
		if !seen[owner] {
			seen[owner] = true
			out = append(out, owner)
		}
	}
	sort.Strings(out)
	return out
}

// searchOpenPRs asks GitHub for every open PR under one owner. GET-only (`gh search`
// is a read). Any failure is Unverifiable (exit 6) naming the owner — a search that did
// not run must never render as "this owner has no unwatched repos".
var searchOpenPRs = func(owner string) ([]searchPR, error) {
	out, err := ghRun("search", "prs", "--owner", owner, "--state", "open",
		"--limit", strconv.Itoa(searchLimit), "--json", "number,title,createdAt,repository")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot search open PRs for owner "+owner, err)
	}
	var rows []searchPR
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, deskkit.Unverifiable("cannot parse open-PR search for owner "+owner, err)
	}
	return rows, nil
}

func cmdScope(hdr Header) (*Report, error) {
	watched := deskkit.AllowedRepos()
	watchedSet := map[string]bool{}
	for _, r := range watched {
		watchedSet[r] = true
	}
	owners := ownersOf(watched)
	if len(owners) == 0 {
		return nil, deskkit.Unverifiable("no owners could be derived from the watched set", nil)
	}

	hdr.Scope = boardScope()
	rep := scopeReport{
		Header: hdr, Owners: owners, Watched: watched, Unwatched: []unwatchedRepo{},
		ObservabilityBound: observabilityBound,
	}

	now := time.Now()
	byRepo := map[string][]searchPR{}
	for _, owner := range owners {
		rows, err := searchOpenPRs(owner)
		if err != nil {
			return nil, err // exit 6, owner named — never a partial reconciliation
		}
		if len(rows) >= searchLimit {
			rep.Truncated = true
		}
		for _, r := range rows {
			slug := r.Repository.NameWithOwner
			if strings.TrimSpace(slug) == "" {
				return nil, deskkit.Unverifiable(fmt.Sprintf(
					"a search result under owner %s carries no repository.nameWithOwner — "+
						"it cannot be attributed, and an unattributable PR must not be dropped silently", owner), nil)
			}
			rep.ObservedPRs++
			byRepo[slug] = append(byRepo[slug], r)
		}
	}
	rep.ObservedRepos = len(byRepo)

	for slug, prs := range byRepo {
		if watchedSet[slug] {
			continue
		}
		u := unwatchedRepo{Repo: slug, OpenPRs: len(prs)}
		var oldest time.Time
		for _, p := range prs {
			if len(u.Sample) < 3 {
				u.Sample = append(u.Sample, p.Number)
			}
			if t, err := time.Parse(time.RFC3339, p.CreatedAt); err == nil {
				if oldest.IsZero() || t.Before(oldest) {
					oldest = t
					u.OldestPR = p.Number
				}
			}
		}
		if !oldest.IsZero() {
			u.OldestAge = formatDuration(now.Sub(oldest))
		}
		sort.Ints(u.Sample)
		rep.Unwatched = append(rep.Unwatched, u)
	}
	sort.SliceStable(rep.Unwatched, func(i, j int) bool {
		if rep.Unwatched[i].OpenPRs != rep.Unwatched[j].OpenPRs {
			return rep.Unwatched[i].OpenPRs > rep.Unwatched[j].OpenPRs
		}
		return rep.Unwatched[i].Repo < rep.Unwatched[j].Repo
	})
	rep.Gap = len(rep.Unwatched) > 0

	detail := fmt.Sprintf("watched=%d observed=%d unwatched=%d", len(watched), rep.ObservedRepos, len(rep.Unwatched))
	return &Report{value: rep, detail: detail, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s\n", hdr.AsOf)
		// The gap prints FIRST and per-repo: an unwatched repo with open PRs is the
		// one thing this verb exists to say out loud.
		for _, u := range rep.Unwatched {
			age := u.OldestAge
			if age == "" {
				age = "unknown"
			}
			fmt.Fprintf(w, "UNWATCHED: %s has %d open PR(s) the board does not read — oldest #%d, open %s\n",
				u.Repo, u.OpenPRs, u.OldestPR, displayDuration(age))
		}
		if rep.Truncated {
			fmt.Fprintf(w, "WARNING: a search hit the --limit %d cap — the reconciliation may be INCOMPLETE; "+
				"an unwatched repo could still be unreported\n", searchLimit)
		}
		// The bound on this verb's own sight, stated in-band. `gh search prs` returns no
		// rows AND no error for a repo the caller's token cannot see (private repo, App
		// not installed, org SSO not authorised), so without this clause an invisible
		// repo with open PRs renders identically to a clean reconciliation — the #359
		// defect one level up, in the verb that exists to close it. One clause on a line
		// that already prints, so a healthy board pays nothing extra for it.
		fmt.Fprintf(w, "scope-check: %d watched · %d repo(s) with open PRs observed under %s · %d UNWATCHED "+
			"· observed via `gh search prs` under the CALLER'S token: repos it cannot see, and any owner "+
			"outside the list above, are NOT observable here and are not counted in UNWATCHED\n",
			len(watched), rep.ObservedRepos, strings.Join(rep.Owners, ", "), len(rep.Unwatched))
		fmt.Fprintf(w, "%-40s %s\n", "WATCHED REPO", "STATUS")
		for _, r := range watched {
			fmt.Fprintf(w, "%-40s %s\n", r, "watched")
		}
	}}, nil
}
