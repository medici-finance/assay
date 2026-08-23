package main

// dispatch.go — the cross-repo DISPATCH queue (#321).
//
// `deskboard dispatch` (aliases: todo, next, next-up) answers the question the
// misleadingly-named `awaiting`/`nextup` verb does NOT: what work can a dispatcher
// START right now? It runs the PINNED statusgen's `--next-up` emitter once per
// configured root, reading the SAME Next-up selection statusgen's STATUS.md board
// uses — briefs at `todo`/`in-progress`, eligible, UNCLAIMED (no open-branch
// claim), per-stream + span-of-control capped — and merges the rows into one
// repo-attributed queue.
//
// It is the mirror image of cmdAwaiting (nextup.go), and shares its two hard
// constraints:
//
//  1. FAIL-CLOSED. Any root error — unreadable root, missing statusgen, a non-zero
//     statusgen exit, unparseable JSON, or a root whose rows claim a repo it was
//     not configured under — aborts the whole run naming the root. A partial
//     dispatch board is worse than none: a short queue reads as "nothing to
//     start", and a dispatcher acts on that.
//
//  2. The PINNED BINARY, never the frozen in-repo tree — the same `statusgen`
//     resolveStatusgen() finds for `awaiting`. `--next-up` is a newer statusgen
//     flag; against a pinned release that predates it, gateScoresForRoot's sibling
//     nextUpForRoot fails closed with statusgen's own "flag provided but not
//     defined" on stderr, which is the honest signal to cut/​install a newer pin.
//
// The ONE population difference from `awaiting` is the entire point: `awaiting`
// returns FINISHED work (implemented/verified) and `dispatch` returns STARTABLE
// work (todo/in-progress), and the two sets are close to disjoint. Dispatching from
// `awaiting` sends workers at work that is already done (#321).
//
// Honest about caps (brief requirement 3): the report carries the aggregate
// held-back decomposition (N by per-stream caps, M by the span cap) and the
// claim-filtering state per root, so an empty queue is DISTINGUISHABLE from a
// throttled or a degraded one — never a bare "0 / drained".

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// dispatchView is the shape `statusgen --next-up` emits per root (mirrors the
// statusgen-side struct of the same name). The held-back counts travel WITH the
// rows so an empty `rows` is never bare — see statusgen/dispatchqueue.go.
type dispatchView struct {
	Repo              string             `json:"repo"`
	Rows              []statusgenDispRow `json:"rows"`
	Eligible          int                `json:"eligible"`
	Shown             int                `json:"shown"`
	Span              int                `json:"span"`
	HeldByStreamCap   int                `json:"heldByStreamCap"`
	HeldBySpan        int                `json:"heldBySpan"`
	HeldByDriveCap    int                `json:"heldByDriveCap"`
	ClaimsKnown       bool               `json:"claimsKnown"`
	ClaimsReason      string             `json:"claimsReason"`
	SerializedUnknown []string           `json:"serializedUnknown"`
	MeasuresGated     []string           `json:"measuresGated"`
	MeasuresUnknown   []string           `json:"measuresUnknown"`
}

// statusgenDispRow is one row of `statusgen --next-up` before repo re-attribution.
type statusgenDispRow struct {
	Brief        string `json:"brief"`
	Stream       string `json:"stream"`
	Status       string `json:"status"`
	Score        int    `json:"score"`
	BlockedCount int    `json:"blockedCount"`
	Repo         string `json:"repo"`
	CriticalArm  string `json:"criticalArm"`
	DriveSlug    string `json:"driveSlug"`
}

// dispatchRow is one merged, repo-attributed row of the cross-repo dispatch queue.
type dispatchRow struct {
	Repo         string `json:"repo"`
	Root         string `json:"root"`
	Brief        string `json:"brief"`
	Stream       string `json:"stream"`
	Status       string `json:"status"`
	Score        int    `json:"score"`
	BlockedCount int    `json:"blockedCount"`
	CriticalArm  string `json:"criticalArm,omitempty"`
	DriveSlug    string `json:"driveSlug,omitempty"`
}

// dispatchDegraded names a root whose claim filtering did NOT run: its rows are an
// unfiltered superset and must not be dispatched from until a run with a reachable
// origin regenerates them.
type dispatchDegraded struct {
	Repo   string `json:"repo"`
	Root   string `json:"root"`
	Reason string `json:"reason"`
}

const (
	populationDispatch = "dispatch"
	dispatchNote       = "briefs at todo/in-progress that are eligible and UNCLAIMED — the queue a " +
		"dispatcher can START from. This is the mirror of `awaiting`: `awaiting` is the " +
		"implemented/verified verification backlog (finished work), `dispatch` is startable work, " +
		"and the two sets are close to disjoint (#321)."
)

type dispatchReport struct {
	Header
	// Population / PopulationStatuses / PopulationNote state WHICH set these rows
	// are, in-band, on every run — the same anti-conflation discipline cmdAwaiting
	// carries. VerbUsed records which spelling reached this report.
	Population         string   `json:"population"`
	PopulationStatuses []string `json:"populationStatuses"`
	PopulationNote     string   `json:"populationNote"`
	VerbUsed           string   `json:"verbUsed"`
	// Roots is every root that was READ, carrying the RESOLVED absolute path — so a
	// repo with zero dispatchable briefs and a repo never read are distinguishable.
	Roots []deskkit.RootConfig `json:"roots"`
	// StatusgenPinned / StatusgenVersion / StatusgenSkew mirror the awaiting report:
	// the pin from .assay-versions, the version the binary reports, and whether they
	// differ. StatusgenPinRepo is which root supplied the pin.
	StatusgenPinned  string `json:"statusgenPinned"`
	StatusgenPinRepo string `json:"statusgenPinRepo"`
	StatusgenVersion string `json:"statusgenVersion"`
	StatusgenSkew    bool   `json:"statusgenSkew"`
	// Aggregate held-back decomposition, summed across every root. shown is
	// len(Rows); the three Held* counts explain the eligible briefs NOT shown, so an
	// empty queue is never mistaken for a drained one.
	Eligible        int `json:"eligible"`
	Shown           int `json:"shown"`
	HeldByStreamCap int `json:"heldByStreamCap"`
	HeldBySpan      int `json:"heldBySpan"`
	HeldByDriveCap  int `json:"heldByDriveCap"`
	// ClaimsDegraded names roots whose claim read did not run — their rows are an
	// unfiltered superset. Non-empty means: do not dispatch from this board.
	ClaimsDegraded []dispatchDegraded `json:"claimsDegraded"`
	Rows           []dispatchRow      `json:"rows"`
}

// nextUpForRoot runs `statusgen --next-up` against ONE root and returns its view.
// Every failure is Unverifiable and names the root — the caller aborts (fail-closed).
func nextUpForRoot(bin, absRoot, repo string) (dispatchView, error) {
	cmd := exec.Command(bin, "--next-up", "--root", absRoot)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			detail = ": " + detail
		}
		return dispatchView{}, deskkit.Unverifiable(
			fmt.Sprintf("statusgen --next-up failed for %s (root %s)%s", repo, absRoot, detail), err)
	}
	var v dispatchView
	if err := json.Unmarshal(out, &v); err != nil {
		return dispatchView{}, deskkit.Unverifiable(
			fmt.Sprintf("cannot parse statusgen --next-up output for %s (root %s)", repo, absRoot), err)
	}
	return v, nil
}

// mergeDispatch is the PURE half of the dispatch verb: it merges the per-root
// statusgen --next-up views (parallel to `resolved`) into one repo-attributed,
// score-sorted queue with the aggregate held-back decomposition. No IO — so the
// attribution fail-closed rule and the held-back accounting are unit-testable
// without a statusgen binary. `views[i]` MUST be the view read from `resolved[i]`.
func mergeDispatch(hdr Header, verbUsed string, resolved []deskkit.RootConfig, views []dispatchView,
	pinnedTag, pinRepo, running string) (*dispatchReport, error) {
	rep := dispatchReport{
		Header:             hdr,
		Population:         populationDispatch,
		PopulationStatuses: []string{"todo", "in-progress"},
		PopulationNote:     dispatchNote,
		VerbUsed:           verbUsed,
		Roots:              resolved,
		StatusgenPinned:    pinnedTag,
		StatusgenPinRepo:   pinRepo,
		StatusgenVersion:   running,
		StatusgenSkew:      running != pinnedTag,
		ClaimsDegraded:     []dispatchDegraded{},
		Rows:               []dispatchRow{},
	}
	for i, r := range resolved {
		v := views[i]
		// Attribute the whole view to a single repo. Same rule as cmdAwaiting: a
		// root that DECLARED a repo disagreeing with the repo it was CONFIGURED
		// under is a fail-closed misattribution, not a silent re-home.
		repo := v.Repo
		switch {
		case repo == "":
			repo = r.Repo // assay (statusgen's source repo) carries no repo: frontmatter
		case repo != r.Repo:
			return nil, deskkit.Unverifiable(fmt.Sprintf(
				"root %s is configured for %s but statusgen --next-up reports repo %s — "+
					"refusing to attribute briefs to a repo the root was not configured under "+
					"(fix the root path in %s, or the stream's repo: frontmatter)",
				r.Path, r.Repo, repo, deskkit.RootsEnv), nil)
		}
		rep.Eligible += v.Eligible
		rep.HeldByStreamCap += v.HeldByStreamCap
		rep.HeldBySpan += v.HeldBySpan
		rep.HeldByDriveCap += v.HeldByDriveCap
		if !v.ClaimsKnown {
			rep.ClaimsDegraded = append(rep.ClaimsDegraded, dispatchDegraded{
				Repo: repo, Root: r.Path, Reason: v.ClaimsReason,
			})
		}
		for _, row := range v.Rows {
			rep.Rows = append(rep.Rows, dispatchRow{
				Repo:         repo,
				Root:         r.Path,
				Brief:        row.Brief,
				Stream:       row.Stream,
				Status:       row.Status,
				Score:        row.Score,
				BlockedCount: row.BlockedCount,
				CriticalArm:  row.CriticalArm,
				DriveSlug:    row.DriveSlug,
			})
		}
	}
	rep.Shown = len(rep.Rows)
	// Score descending, then repo/stream/brief for a deterministic board — same
	// ordering as cmdAwaiting.
	sort.SliceStable(rep.Rows, func(i, j int) bool {
		a, b := rep.Rows[i], rep.Rows[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		if a.Stream != b.Stream {
			return a.Stream < b.Stream
		}
		return a.Brief < b.Brief
	})
	return &rep, nil
}

// cmdDispatch renders the cross-repo dispatch queue. verbUsed is the spelling the
// caller invoked (dispatch / todo / next / next-up).
func cmdDispatch(hdr Header, verbUsed string) (*Report, error) {
	roots, err := deskkit.ConfiguredRoots()
	if err != nil {
		return nil, err
	}
	bin, err := resolveStatusgen()
	if err != nil {
		return nil, err
	}
	// Resolve EVERY configured root up front, before reading any of them — same
	// discipline as cmdAwaiting: fail-closed stays fail-closed, and the report can
	// carry the resolved absolute path so the coverage lines name the directory the
	// rows actually came from.
	resolved := make([]deskkit.RootConfig, 0, len(roots))
	for _, r := range roots {
		abs, rerr := deskkit.ResolveRoot(r)
		if rerr != nil {
			return nil, rerr // fail-closed: never a partial board
		}
		resolved = append(resolved, deskkit.RootConfig{Repo: r.Repo, Path: abs})
	}
	pinnedTag, pinRepo, err := resolveStatusgenPin(resolved)
	if err != nil {
		return nil, err
	}
	running := statusgenVersionOf(bin)

	// IO half: run the pinned statusgen once per root, fail-closed on any error.
	views := make([]dispatchView, 0, len(resolved))
	for _, r := range resolved {
		v, rerr := nextUpForRoot(bin, r.Path, r.Repo)
		if rerr != nil {
			return nil, rerr // fail-closed
		}
		views = append(views, v)
	}

	// Pure half: merge/attribute/sum. Split out so the merge, repo-attribution
	// fail-closed rule, and held-back accounting are unit-testable without a binary.
	rep, err := mergeDispatch(hdr, verbUsed, resolved, views, pinnedTag, pinRepo, running)
	if err != nil {
		return nil, err
	}

	return &Report{value: *rep, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s  (DISPATCH queue across %d root(s); statusgen %s, pinned %s from %s)\n",
			hdr.AsOf, len(rep.Roots), rep.StatusgenVersion, rep.StatusgenPinned, shortRepo(rep.StatusgenPinRepo))
		fmt.Fprintf(w, "population: %s (todo/in-progress, unclaimed) — the queue to START from; NOT `awaiting` (implemented/verified) (#321)\n",
			rep.Population)
		if rep.StatusgenSkew {
			fmt.Fprintf(w, "WARN statusgen %s does not match the .assay-versions pin %s — "+
				"reinstall the pinned binary (tools/statusgen/README.md)\n",
				rep.StatusgenVersion, rep.StatusgenPinned)
		}
		for _, r := range rep.Roots {
			fmt.Fprintf(w, "  root %-24s %s\n", shortRepo(r.Repo), r.Path)
		}
		// Claim degradation is the loudest line: a degraded root's rows are an
		// unfiltered superset and must not be dispatched from.
		for _, d := range rep.ClaimsDegraded {
			fmt.Fprintf(w, "DEGRADED %s (%s): claim filtering did not run (%s) — rows are an UNFILTERED "+
				"superset; do not dispatch from this root until a run with a reachable origin regenerates it\n",
				shortRepo(d.Repo), d.Root, d.Reason)
		}
		// Held-back decomposition: printed on EVERY run so an empty queue is never a
		// bare "0". heldBackLine names which cap fired.
		fmt.Fprintf(w, "queue: %d shown of %d eligible; %s\n", rep.Shown, rep.Eligible, dispatchHeldBackLine(rep))
		if len(rep.Rows) == 0 {
			if rep.Eligible == 0 {
				fmt.Fprintf(w, "(no dispatchable briefs — all %d configured root(s) read successfully; nothing at todo/in-progress is eligible + unclaimed)\n", len(rep.Roots))
			} else {
				fmt.Fprintf(w, "(0 shown but %d eligible — everything is held back by caps, NOT drained; see the queue line above)\n", rep.Eligible)
			}
			return
		}
		fmt.Fprintf(w, "%-8s %-24s %-6s %-12s %-3s %s\n", "REPO", "STREAM", "SCORE", "STATUS", "UNB", "BRIEF")
		for _, r := range rep.Rows {
			fmt.Fprintf(w, "%-8s %-24s %-6d %-12s %-3d %s\n",
				shortRepo(r.Repo), trunc(r.Stream, 24), r.Score, r.Status, r.BlockedCount, r.Brief)
		}
	}}, nil
}

// dispatchHeldBackLine explains the eligible briefs NOT shown, naming which cap
// fired. It is the dispatch analog of statusgen's heldBackReason and is printed on
// every run so "0 shown" is always accompanied by WHY.
func dispatchHeldBackLine(rep *dispatchReport) string {
	held := rep.HeldByStreamCap + rep.HeldBySpan + rep.HeldByDriveCap
	if held == 0 {
		return "0 held back"
	}
	parts := []string{}
	if rep.HeldByStreamCap > 0 {
		parts = append(parts, fmt.Sprintf("%d by per-stream caps", rep.HeldByStreamCap))
	}
	if rep.HeldBySpan > 0 {
		parts = append(parts, fmt.Sprintf("%d by the span-of-control cap", rep.HeldBySpan))
	}
	if rep.HeldByDriveCap > 0 {
		parts = append(parts, fmt.Sprintf("%d by the drive anti-starvation floor", rep.HeldByDriveCap))
	}
	return fmt.Sprintf("%d held back (%s)", held, strings.Join(parts, ", "))
}
