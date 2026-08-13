package main

// nextup.go — the multi-repo Next-up board.
//
// statusgen emits one board per repo root. `deskboard nextup` runs the PINNED
// statusgen binary once per configured root, reading its `--gate-scores` JSON,
// and merges the rows into a single repo-attributed queue — the read side of the
// multi-repo workflow.
//
// Two constraints shape this file:
//
//  1. FAIL-CLOSED. ANY root error — unreadable root, missing statusgen,
//     a non-zero statusgen exit, unparseable JSON, or a root whose rows claim a
//     repo it was not configured under — aborts the whole run with a non-zero
//     exit naming the root. It never warns-and-continues. A partial board is
//     worse than no board here: an empty or short queue reads as "nothing open",
//     and the desk acts on that. (PR #1303's version warned and `continue`d on
//     all three error paths; that is what this rewrite fixes.)
//
//  2. The PINNED BINARY, never the frozen tree. tracker's tools/statusgen is a frozen
//     copy — `.github/workflows/statusgen.yml` blocks PRs that touch its Go
//     files, and the canonical source lives in assay. So this shells out
//     to the installed `statusgen` binary that `.assay-versions` pins, and NEVER
//     `go run`s the local tree. (#1303 ran `go run .` in tools/statusgen, which
//     would silently execute the frozen copy instead of the pinned release.)
//
// The pinned-vs-running version is REPORTED, not enforced: it rides in the JSON
// header as `statusgenSkew` and as a WARN line on the --table view.
// `--gate-scores` is per-root either way, so a version-skewed statusgen still
// yields correct rows; hard-failing here would couple the board's availability
// to release timing for no correctness gain. The mismatch stays visible so it
// can't rot unnoticed. (The warning rides on the report, not stderr, because the
// JSON path deliberately writes nothing to stderr — see main.go.)

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// statusgenBinEnv overrides which statusgen binary to run. Without it the
// installed `statusgen` on PATH is used — the one tools/statusgen/README.md's
// install step puts there from the pinned release.
const statusgenBinEnv = "STATUSGEN_BIN"

// gateScoreRow is the shape `statusgen --gate-scores` emits. `repo` arrived with
// multi-root statusgen and is absent from older releases;
// when absent the configured repo key is used instead.
type gateScoreRow struct {
	Brief        string `json:"brief"`
	Score        int    `json:"score"`
	BlockedCount int    `json:"blockedCount"`
	Stream       string `json:"stream"`
	Status       string `json:"status"`
	Repo         string `json:"repo,omitempty"`
}

// nextupRow is one merged, repo-attributed row of the cross-repo queue.
type nextupRow struct {
	Repo         string `json:"repo"`
	Root         string `json:"root"`
	Brief        string `json:"brief"`
	Stream       string `json:"stream"`
	Status       string `json:"status"`
	Score        int    `json:"score"`
	BlockedCount int    `json:"blockedCount"`
}

// Population labelling. Measured on the live board: this verb
// returned 20 rows — 19 `implemented`, 1 `verified`, ZERO `todo` — and eight real
// Next-up briefs grepped against its output scored a hit count of 0. The command named
// for the dispatch queue returns the awaiting-VERIFICATION backlog, and the two sets
// are close to disjoint: work at `implemented` is exactly the work a dispatcher must
// NOT hand out. It runs, exits 0, and returns a well-formed board of the wrong
// population — which nearly became a review recommendation before someone measured it.
//
// The source is `statusgen --gate-scores`, whose own selection is
// `b.Status != "implemented" && b.Status != "verified" → skip`. So the population is
// not a bug in this file; the NAME was the lie. It is fixed by naming what it returns
// (`deskboard awaiting`), keeping `nextup` as an alias so nothing breaks, and having
// every report state its population in-band so a consumer cannot mistake it again.
const (
	populationAwaiting = "awaiting-verification"
	populationNote     = "briefs at implemented/verified awaiting verification. This is NOT the " +
		"dispatch queue: it contains no `todo` brief by construction (statusgen --gate-scores " +
		"selects implemented/verified only), so dispatching from it sends workers at work that " +
		"is already done."
)

type nextupReport struct {
	Header
	// Population / PopulationStatuses / PopulationNote say WHICH set these rows are,
	// on every run, in-band. AliasUsed records that the caller reached this report
	// through the misleading `nextup` spelling.
	Population         string   `json:"population"`
	PopulationStatuses []string `json:"populationStatuses"`
	PopulationNote     string   `json:"populationNote"`
	AliasUsed          string   `json:"aliasUsed,omitempty"`
	// Roots is every root that was READ, in full, carrying the RESOLVED absolute
	// path rather than the configured spelling (the tracker default is literally
	// "."). It is emitted so a consumer can see the board's coverage rather than
	// infer it from the rows: a repo with zero open briefs and a repo that was
	// never read produce the same (empty) row set, and only this field tells them
	// apart — which it can only do if it names the directory the rows actually
	// came from, so `roots` and every row's `root` agree.
	Roots []deskkit.RootConfig `json:"roots"`
	// StatusgenPinned / StatusgenVersion are the pin from .assay-versions and the
	// version the binary actually reports. StatusgenSkew is true when they differ.
	// StatusgenPinRepo is WHICH configured root's .assay-versions supplied the pin
	// (#511: it is discovered, not a compiled-in repo — see resolveStatusgenPin).
	StatusgenPinned  string      `json:"statusgenPinned"`
	StatusgenPinRepo string      `json:"statusgenPinRepo"`
	StatusgenVersion string      `json:"statusgenVersion"`
	StatusgenSkew    bool        `json:"statusgenSkew"`
	Rows             []nextupRow `json:"rows"`
}

// resolveStatusgen returns the path of the statusgen binary to run. Fail-closed:
// an unresolvable binary is Unverifiable (exit 6) with the install recipe, never
// a fallback to `go run` against the frozen in-repo copy.
func resolveStatusgen() (string, error) {
	if bin := strings.TrimSpace(os.Getenv(statusgenBinEnv)); bin != "" {
		path, err := exec.LookPath(bin)
		if err != nil {
			return "", deskkit.Unverifiable(statusgenBinEnv+"="+bin+" is not an executable", err)
		}
		return path, nil
	}
	path, err := exec.LookPath("statusgen")
	if err != nil {
		return "", deskkit.Unverifiable(
			"statusgen is not on PATH — install the pinned release "+
				"(see tools/statusgen/README.md) or set "+statusgenBinEnv+"; "+
				"the in-repo tools/statusgen copy is FROZEN and must not be run", err)
	}
	return path, nil
}

// statusgenVersionOf asks the binary which release it is. An older statusgen has
// no --version flag, which is information rather than an error: report it as
// "unknown" so the skew is visible without blocking the board.
func statusgenVersionOf(bin string) string {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "unknown"
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "unknown"
	}
	return v
}

// gateScoresForRoot runs statusgen --gate-scores against ONE root and returns its
// rows. Every failure is Unverifiable and names the root — the caller aborts.
func gateScoresForRoot(bin, absRoot, repo string) ([]gateScoreRow, error) {
	cmd := exec.Command(bin, "--gate-scores", "--root", absRoot)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			detail = ": " + detail
		}
		return nil, deskkit.Unverifiable(
			fmt.Sprintf("statusgen --gate-scores failed for %s (root %s)%s", repo, absRoot, detail), err)
	}
	var rows []gateScoreRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, deskkit.Unverifiable(
			fmt.Sprintf("cannot parse statusgen --gate-scores output for %s (root %s)", repo, absRoot), err)
	}
	return rows, nil
}

// resolveStatusgenPin finds the statusgen pin among the resolved roots and
// reports which repo's root supplied it.
//
// #511: this used to require a compiled-in repo constant (example-org's
// tracker) to be among the configured roots, on the theory
// that ITS `.assay-versions` is where the pin lives. That made the board
// unrunnable for any adopter whose roster names only their own repo — the
// only way to satisfy the check was naming a third party's repo in DESK_ROOTS,
// which in turn required widening ASSAY_ALLOWED_REPOS (the desk's WRITE
// authority) just to read a version pin. There is nothing special about that
// one repo: `.assay-versions` lives "at the consumer repo root"
// for WHICHEVER repo is
// actually pinning statusgen, so this discovers it instead of naming it.
//
// It walks the resolved roots in their already-deterministic order (sorted by
// repo) and uses the first one that carries a `.assay-versions` file. A root
// with NO pin file is not an error — assay itself, the statusgen
// SOURCE repo, never carries one — but a root whose pin file IS present and
// unreadable or malformed fails closed immediately, naming that root, rather
// than silently falling through to the next one.
func resolveStatusgenPin(resolved []deskkit.RootConfig) (tag, repo string, err error) {
	for _, r := range resolved {
		if _, statErr := os.Stat(filepath.Join(r.Path, ".assay-versions")); statErr != nil {
			continue
		}
		pinnedTag, _, perr := deskkit.StatusgenPin(r.Path)
		if perr != nil {
			return "", "", perr // fail-closed: a present-but-bad pin is never skipped
		}
		return pinnedTag, r.Repo, nil
	}
	names := make([]string, len(resolved))
	for i, r := range resolved {
		names[i] = r.Repo
	}
	return "", "", deskkit.Unverifiable(
		"none of the configured roots ("+strings.Join(names, ", ")+") carry a readable "+
			".assay-versions — the statusgen pin file must live at one of them; add the repo that pins statusgen to "+
			deskkit.RootsEnv, nil)
}

// cmdAwaiting renders the cross-repo awaiting-verification queue. verbUsed is the
// spelling the caller invoked — `awaiting` (canonical) or `nextup` (deprecated alias).
func cmdAwaiting(hdr Header, verbUsed string) (*Report, error) {
	roots, err := deskkit.ConfiguredRoots()
	if err != nil {
		return nil, err
	}
	bin, err := resolveStatusgen()
	if err != nil {
		return nil, err
	}

	// Resolve EVERY configured root up front, before reading any of them. Two
	// reasons: fail-closed stays fail-closed (a bad root aborts before a single
	// row is collected), and the report can carry the resolved absolute path, so
	// the coverage lines name the directory the rows actually came from instead
	// of the configured spelling.
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

	rep := nextupReport{
		Header:             hdr,
		Population:         populationAwaiting,
		PopulationStatuses: []string{"implemented", "verified"},
		PopulationNote:     populationNote,
		Roots:              resolved,
		StatusgenPinned:    pinnedTag,
		StatusgenPinRepo:   pinRepo,
		StatusgenVersion:   running,
		StatusgenSkew:      running != pinnedTag,
		Rows:               []nextupRow{},
	}
	if verbUsed == "nextup" {
		rep.AliasUsed = "nextup"
	}

	for _, r := range resolved {
		rows, rerr := gateScoresForRoot(bin, r.Path, r.Repo)
		if rerr != nil {
			return nil, rerr // fail-closed
		}
		for _, row := range rows {
			repo := row.Repo
			switch {
			case repo == "":
				repo = r.Repo // pre-multi-root statusgen emits no repo field
			case repo != r.Repo:
				// The root DECLARED a repo (statusgen's `repo:` stream frontmatter)
				// that disagrees with the repo it was CONFIGURED under. Preferring
				// the declaration would let a checkout re-attribute its briefs at
				// will — including to a repo roots.go would have refused outright
				// and silent misattribution is the exact failure this board
				// exists to eliminate. So: fail-closed, naming both sides.
				return nil, deskkit.Unverifiable(fmt.Sprintf(
					"root %s is configured for %s but statusgen reports its stream %s as %s — "+
						"refusing to attribute briefs to a repo the root was not configured under "+
						"(fix the root path in %s, or the stream's repo: frontmatter)",
					r.Path, r.Repo, row.Stream, repo, deskkit.RootsEnv), nil)
			}
			rep.Rows = append(rep.Rows, nextupRow{
				Repo:         repo,
				Root:         r.Path,
				Brief:        row.Brief,
				Stream:       row.Stream,
				Status:       row.Status,
				Score:        row.Score,
				BlockedCount: row.BlockedCount,
			})
		}
	}

	// Score descending, then repo/stream/brief for a deterministic board.
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

	return &Report{value: rep, render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s  (AWAITING-VERIFICATION queue across %d root(s); statusgen %s, pinned %s from %s)\n",
			hdr.AsOf, len(rep.Roots), rep.StatusgenVersion, rep.StatusgenPinned, shortRepo(rep.StatusgenPinRepo))
		// One line, always, naming the population — the defect was a well-formed board
		// of the wrong set, so the set has to be stated where the rows are read.
		fmt.Fprintf(w, "population: %s (implemented/verified) — NOT the dispatch queue; it holds no `todo` brief (#321)\n",
			rep.Population)
		if rep.AliasUsed != "" {
			fmt.Fprintln(w, "NOTE: `deskboard nextup` is a deprecated alias for `deskboard awaiting` — "+
				"the name says dispatch queue, the rows are the verification backlog (#321)")
		}
		if rep.StatusgenSkew {
			fmt.Fprintf(w, "WARN statusgen %s does not match the .assay-versions pin %s — "+
				"reinstall the pinned binary (tools/statusgen/README.md)\n",
				rep.StatusgenVersion, rep.StatusgenPinned)
		}
		for _, r := range rep.Roots {
			fmt.Fprintf(w, "  root %-24s %s\n", shortRepo(r.Repo), r.Path)
		}
		if len(rep.Rows) == 0 {
			// Say WHICH roots were read. "no rows" after a fail-closed run means
			// every configured root really is empty — the sentence has to carry
			// that, or it reads like the silent-partial-board failure.
			fmt.Fprintf(w, "(no awaiting briefs — all %d configured root(s) read successfully)\n", len(rep.Roots))
			return
		}
		fmt.Fprintf(w, "%-8s %-24s %-6s %-14s %s\n", "REPO", "STREAM", "SCORE", "STATUS", "BRIEF")
		for _, r := range rep.Rows {
			fmt.Fprintf(w, "%-8s %-24s %-6d %-14s %s\n",
				shortRepo(r.Repo), trunc(r.Stream, 24), r.Score, r.Status, r.Brief)
		}
	}}, nil
}
