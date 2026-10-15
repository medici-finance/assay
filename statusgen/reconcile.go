package main

// reconcile.go — the `statusgen reconcile` verb (derived-board/03).
//
// It derives each brief's PR-sourced lifecycle cell (todo / in-progress /
// implemented, or unknown) from the ONLY machine-readable PR→brief edge — the
// `Brief:` trailer — and prints WHY for every cell, so a negative is always paired
// with evidence the instrument looked (spec §2, the three-state invariant). The
// verify-witness / approval-at-head fold that lifts a cell to verified/done is the
// pure engine in lifecycle.go (composed here from the same inputs, exercised in
// full by the fixture matrix); the regen workflow and the generated-table brief
// (derived-board/04) wire those witness sources into the board render. This verb's
// job is the PR-dereference half: given a repo, read its PRs and derive.
//
// Modes:
//   - --offline: touch NO network. Every PR-derived cell is unknown(offline). This
//     is the arm --lint uses on a branch — it never renders a PR-derived todo.
//   - online (--repo owner/name): read the repo's PRs over REST (ghfetch.go). A
//     fetch failure is unknown WITH the HTTP status, never a clean board.
//
// --backfill and --report (derived-board/07, reconcilebackfill.go) layer a
// DECLARED, reviewable, history-only fallback and a drift report on top of the
// online mode above; see reconcilebackfill.go's header for the full contract.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// reconcileResult is the machine-readable output shape (--json). Its per-brief
// rows are BriefCell values (id/cell/source/witness/reason/version). Applied
// is populated only with --apply, and only with rows this run actually wrote
// (reconcileapply.go) — omitted (nil) otherwise. WouldApply is the --apply-free
// form (#2440): set on a --backfill run WITHOUT --apply that LOOKED (no
// whole-run refusal — the PR fetch, the raw pull-list fetch and every fold
// read made — and a docs/streams board found), to the rows --apply would write
// on the same tree, computed by the same function that writes
// (reconcileWrites). Present means looked: an empty list `[]` is "looked and
// found nothing", and the field is ABSENT whenever the run could not look
// (offline, no --repo, a refused or failed fetch, a failed fold read, no board
// under --root) so "found nothing" can never be read off a run that read
// nothing. Held lists the witnessed rows the write did NOT take (on the
// --apply-free form: would not take) because the move would add a lint PROBLEM
// (reconcilehold.go), decided by the same function — omitted (nil) when none.
// reportWrote and applyRun (not in the JSON) record that --report wrote the
// drift report in this run and that --apply was given, so the table output
// labels the run by what it did, never by which fields it carries (#2443).
type reconcileResult struct {
	Repo       string        `json:"repo"`
	LookedAt   bool          `json:"lookedAt"`
	Reason     string        `json:"reason"`
	Briefs     []BriefCell   `json:"briefs"`
	Applied    []appliedRow  `json:"applied,omitempty"`
	WouldApply *[]appliedRow `json:"wouldApply,omitempty"`
	// WouldRefuse is set on the read-only form when --apply on the same run
	// would refuse to write anything (exit 3): the reason, from the same
	// whole-run admission decision (reconcileWriteRefusal). WouldApply is then
	// ABSENT — the run could not look, so it lists no row, not `[]` (#2443) —
	// and Held is empty.
	WouldRefuse string    `json:"wouldRefuse,omitempty"`
	Held        []heldRow `json:"held,omitempty"`
	// Unread lists every fold read that could not be made this run (a failed
	// or truncated forge read, a provenance or version read that failed). The
	// briefs it touches derive `unknown`; --apply refuses while it is non-empty.
	Unread []string `json:"unread,omitempty"`

	reportWrote bool
	applyRun    bool
}

const (
	reconcileOK       = 0
	reconcileUsageErr = 2
	// reconcileCouldNotCheck is --apply's exit when the PR read did not happen
	// (HTTP error, no token, offline): a writer that looked at nothing must not
	// exit like one that looked and found nothing to write.
	reconcileCouldNotCheck = 3
)

// reconcileGHClient builds the read-only REST client runReconcile reads PRs
// through. It is a variable only so a test can point the whole verb at a local
// recorded-response server (the offline envelope); production always uses
// newGHClient. It is never a flag or an environment variable, so no run can
// send the token to another host.
var reconcileGHClient = newGHClient

func runReconcile(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	root := fs.String("root", ".", "repository root whose briefs to reconcile")
	repo := fs.String("repo", "", "owner/name of the repo to read PRs from (online mode)")
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	offline := fs.Bool("offline", false, "do not touch the network; every PR-derived cell is unknown")
	tokenFile := fs.String("token-file", "", "file holding the GitHub API token (else GITHUB_TOKEN)")
	backfill := fs.Bool("backfill", false, "declared history-only fallback (brief-07): a merged PR whose branch name or body names the brief in <stream>/<NN> or <stream>-<NN> form counts as a witness when no trailer links one; a hand-asserted implemented/verified/done with neither renders unknown, never a silent todo. Without --apply it also reports the rows --apply would write (JSON: wouldApply, present only when the run could look) and writes no stream README; it writes the drift report file only when --report is also given")
	report := fs.Bool("report", false, "with --backfill: write docs/streams/board-drift-<date>.md, one row per brief where the last hand-edited (pre-generation) README cell disagrees with what this run derives; requires --backfill")
	apply := fs.Bool("apply", false, "WRITE a witnessed todo|in-progress -> implemented cell back into the brief's stream README Status column (only, never Verified/Reviewed, never verified/done, never a demotion). Without --backfill only a `Brief:` trailer witness is written; with --backfill the declared branch/body match is written too. A row whose move would add a lint PROBLEM is held and reported, never written. Exits 3 when the PR read could not be done")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return reconcileOK
		}
		return reconcileUsageErr
	}
	if *report && !*backfill {
		fmt.Fprintln(stderr, "reconcile: --report requires --backfill (the report compares against the backfill-adjusted cells)")
		return reconcileUsageErr
	}
	if *backfill && hasNoGitDir(*root) {
		fmt.Fprintf(stderr, "could-not-check: %s has no .git — --backfill reads the hand-said history from git log\n", *root)
		return reconcileUsageErr
	}

	idents, err := reconcileBriefIdents(*root)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: %v\n", err)
		return reconcileUsageErr
	}

	in := LifecycleInput{Briefs: idents}
	var client *ghClient
	var classifier *authoringClassifier
	var authoringUndecided map[string]string
	var unread []string
	var allPulls []ghPull
	allPullsLookedAt := false
	allPullsReason := ""
	switch {
	case *offline:
		in.LookedAt = false
		in.Reason = "offline (--offline) — the PR fetch was not attempted"
	case *repo == "":
		// No repo and not --offline: there is nothing to dereference against. Report
		// it as a could-not-check, never a clean board.
		in.LookedAt = false
		in.Reason = "no --repo given and not --offline — nothing to dereference PR-derived cells against"
	default:
		token, terr := resolveGitHubToken(*tokenFile)
		if terr != nil {
			fmt.Fprintf(stderr, "could-not-check: %v\n", terr)
			return reconcileUsageErr
		}
		client = reconcileGHClient(token)
		// One paged read of every pull serves both the trailer fold (ListPRs'
		// filter) and the --backfill match below, which used to page the whole
		// list a second time.
		allPulls, allPullsLookedAt, allPullsReason = client.fetchAllPulls(*repo)
		if allPullsLookedAt {
			in.PRs = prRecordsFromPulls(allPulls)
		}
		in.LookedAt = allPullsLookedAt
		in.Reason = allPullsReason
		// A PR that only AUTHORED a brief is not a witness that it is implemented
		// (briefauthoring.go): drop it from the trailer fold before anything reads
		// in.PRs, so the cell, the approval fold and --apply all see the same set.
		var more []string
		classifier, authoringUndecided, more = admitPRWitnesses(&in, client, *repo, *root, stderr)
		unread = append(unread, more...)
		// The witness fold (#1787): populate the inputs DeriveLifecycle needs
		// to derive above `implemented`, each by its owner's rule (see
		// reconcilefold.go). Every read that could not be made is disclosed in
		// `unread` and leaves the briefs it touches `unknown`. Online only:
		// --offline keeps every map nil and every cell unknown.
		if boardRoot, found := findBoardRoot(*root); found {
			unread = append(unread, wireFoldInputs(&in, boardRoot, client, *repo, stderr)...)
		}
		mergeUndecided(&in, authoringUndecided)
	}

	cells := DeriveLifecycle(in)

	var lookup handSaidLookup
	pullsUnread := ""
	if *backfill {
		lookup = newHandSaidLookup(*root)
		var pulls []ghPull
		pullsLookedAt := false
		if client != nil {
			pulls, pullsLookedAt = allPulls, allPullsLookedAt
			if !pullsLookedAt {
				fmt.Fprintf(stderr, "reconcile --backfill: could-not-check the raw pull list: %s\n", allPullsReason)
				pullsUnread = allPullsReason
			}
		}
		var authoring backfillAuthoring
		if classifier != nil {
			authoring = classifier.authoringOnly
			if pullsLookedAt {
				// Overlap the match's file-list reads; the match below then
				// reads the same PRs, in the same order, from the cache.
				classifier.prefetchBackfill(pulls, backfillMatchBriefs(cells, in))
			}
		}
		var bfUnread []string
		cells, bfUnread = applyReconcileBackfillWith(cells, in, pulls, pullsLookedAt, lookup, authoring)
		for _, m := range bfUnread {
			fmt.Fprintf(stderr, "reconcile --backfill: %s\n", m)
		}
		unread = append(unread, bfUnread...)
	}

	// The whole-run admission decision, shared by both forms (#2472 C10):
	// --apply refuses on it, and the read-only form reports the same refusal
	// instead of rows, so `wouldApply` never lists a row the writer refuses.
	refusal := reconcileWriteRefusal(in, pullsUnread, unread)
	if *apply && refusal != "" {
		fmt.Fprintf(stderr, "reconcile --apply: could-not-check — %s; nothing written\n", refusal)
		return reconcileCouldNotCheck
	}

	var applied []appliedRow
	var wouldApply *[]appliedRow
	var held []heldRow
	wouldRefuse := ""
	if *backfill || *apply {
		// One decision path for both forms (reconcileWrites), lint hold
		// included: --apply writes the rows (trailer-only without --backfill);
		// --backfill without it reports the same rows as wouldApply and writes
		// no stream README (--report, when given, still writes its own drift
		// report file below). wouldApply is set only when the run looked — no
		// whole-run refusal (the PR list, the raw pull list and every fold read
		// made) and a board found — so an empty list always means "looked and
		// found nothing" (#2443).
		// boardRoot mirrors reconcileBriefIdents' own resolution (reconcile is
		// commonly run from a subdirectory), so the rows land on the same
		// docs/streams tree the briefs above were enumerated from.
		rows := []appliedRow{}
		boardRoot, found := findBoardRoot(*root)
		if refusal != "" {
			// Only the read-only form reaches here (--apply returned above):
			// --apply would write nothing, so neither is a row listed.
			wouldRefuse = refusal
			fmt.Fprintf(stderr, "reconcile --backfill: could-not-check — %s; --apply would exit %d and write nothing\n", refusal, reconcileCouldNotCheck)
		} else if found {
			var werr error
			// The lint hold's progress goes to stderr: --json's stdout is unchanged.
			opts := holdOptions{crossCheck: holdCrossCheck, progress: stderr}
			if rows, held, werr = reconcileWritesWith(boardRoot, cells, *apply, opts); werr != nil {
				label := "reconcile --backfill"
				if *apply {
					label = "reconcile --apply"
				}
				fmt.Fprintf(stderr, "%s: %v\n", label, werr)
				return reconcileUsageErr
			}
			if rows == nil {
				rows = []appliedRow{}
			}
		}
		if *apply {
			applied = rows
		} else if refusal == "" && found {
			wouldApply = &rows
		}
	}

	res := reconcileResult{
		Repo:        *repo,
		LookedAt:    in.LookedAt,
		Reason:      in.Reason,
		Briefs:      cells,
		Applied:     applied,
		WouldApply:  wouldApply,
		WouldRefuse: wouldRefuse,
		Held:        held,
		Unread:      unread,
		applyRun:    *apply,
	}

	if *report {
		rows := buildDriftRows(cells, lookup)
		path, werr := writeDriftReport(*root, *repo, rows, time.Now())
		if werr != nil {
			fmt.Fprintf(stderr, "reconcile --report: writing drift report: %v\n", werr)
			return reconcileUsageErr
		}
		fmt.Fprintf(stderr, "reconcile --report: wrote %s (%d drift row(s))\n", path, len(rows))
		res.reportWrote = true
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "reconcile: encoding JSON: %v\n", err)
			return reconcileUsageErr
		}
		return reconcileOK
	}
	printReconcileTable(stdout, res)
	return reconcileOK
}

// reconcileWriteRefusal is the whole-run admission decision: why no row may be
// written this run, or "" when the writer may proceed. --apply exits 3 on it;
// the read-only form reports it as wouldRefuse with no rows. Each refusal is a
// read that could not be made, and a write from it would pass for a complete
// run:
//   - the raw pull list (the --backfill witness set) was not read — writing the
//     trailer half alone would pass for a complete backfill;
//   - the PR read did not happen — a scheduled writer exiting 0 here reads as
//     "board current";
//   - a fold read failed — the cells it touches are `unknown`.
func reconcileWriteRefusal(in LifecycleInput, pullsUnread string, unread []string) string {
	switch {
	case pullsUnread != "":
		return "the raw pull list could not be read (" + pullsUnread + ")"
	case !in.LookedAt:
		return in.Reason
	case len(unread) > 0:
		return fmt.Sprintf("%d fold read(s) failed (see `unread`)", len(unread))
	}
	return ""
}

// reconcileBriefIdents enumerates every brief on the board and returns its
// identity for the fold: the id used in its `Brief:` trailer, its gate, and its
// version. An opted-in brief-v1/v2 file yields all three from its frontmatter; a
// legacy (no-frontmatter) brief still appears, keyed by its filename-derived id,
// so the board is complete rather than silently dropping the rows the derivation
// cannot enrich.
func reconcileBriefIdents(root string) ([]BriefIdent, error) {
	// Resolve the board root: reconcile is commonly run from a subdirectory (the
	// Verify table runs it from statusgen/ so `go run .` finds the module), so a
	// --root that has no docs/streams is walked UP to the first ancestor that does.
	// A tree with no docs/streams anywhere yields zero briefs (a valid empty board),
	// not an error — the lookedAt/reason still governs the run's honesty.
	boardRoot, found := findBoardRoot(root)
	if !found {
		return nil, nil
	}
	// The enumeration lives in loadReconcileFoldData (reconcilefold.go), which
	// keeps the fold's raw material alongside each ident; this is the ident-only
	// view of the same walk, so the two can never drift apart.
	data, _, err := loadReconcileFoldData(boardRoot)
	if err != nil {
		return nil, err
	}
	idents := make([]BriefIdent, 0, len(data))
	for _, d := range data {
		idents = append(idents, d.ident)
	}
	return idents, nil
}

// findBoardRoot returns the nearest ancestor of start (inclusive) that contains a
// docs/streams directory. It ascends until it finds one or reaches the filesystem
// root. found is false when none exists.
func findBoardRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, "docs", "streams")); err == nil && fi.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// reconcileDetail is a table row's evidence column. An `unknown` cell prints
// its reason first — why the board cannot say — and then any witness it rests
// on; the spec's `unknown` row has the board say why per cell, not only in the
// JSON and on stderr.
func reconcileDetail(b BriefCell) string {
	switch {
	case b.Witness == "":
		return b.Reason
	case b.Cell == "unknown" && b.Reason != "":
		return b.Reason + " — " + b.Witness
	default:
		return b.Witness
	}
}

func printReconcileTable(w *os.File, res reconcileResult) {
	if !res.LookedAt {
		fmt.Fprintf(w, "reconcile: could-not-check — %s\n", res.Reason)
	} else {
		fmt.Fprintf(w, "reconcile: %s (%d brief(s))\n", res.Repo, len(res.Briefs))
	}
	for _, b := range res.Briefs {
		fmt.Fprintf(w, "  %-40s %-12s %s\n", b.ID, b.Cell, reconcileDetail(b))
	}
	// The --apply-free label says what the run wrote: "read-only" is true only
	// when --report did not write the drift report file (#2443).
	label := "reconcile --backfill (read-only)"
	if res.reportWrote {
		label = "reconcile --backfill (no stream README written; --report wrote the drift report)"
	}
	if res.WouldRefuse != "" {
		fmt.Fprintf(w, "%s: --apply would refuse — %s; it would write nothing\n", label, res.WouldRefuse)
	}
	if res.WouldApply != nil && len(*res.WouldApply) > 0 {
		fmt.Fprintf(w, "%s: --apply would write %d row(s)\n", label, len(*res.WouldApply))
		for _, a := range *res.WouldApply {
			fmt.Fprintf(w, "  %-40s %s -> %s   %s\n", a.ID, a.From, a.To, a.Path)
		}
	}
	if len(res.Applied) > 0 {
		fmt.Fprintf(w, "reconcile --apply: wrote %d row(s)\n", len(res.Applied))
		for _, a := range res.Applied {
			fmt.Fprintf(w, "  %-40s %s -> %s   %s   [%s]\n", a.ID, a.From, a.To, a.Path, a.Witness)
		}
	}
	// Held rows print whether or not anything was written: a tick that holds
	// every row must still show each one. The label follows the run's mode,
	// never the presence of wouldApply: a run without --apply never prints the
	// --apply label, whatever it could read (#2443).
	if len(res.Held) > 0 {
		if !res.applyRun {
			fmt.Fprintf(w, "%s: --apply would hold %d row(s) — writing them would add a lint PROBLEM\n", label, len(res.Held))
		} else {
			fmt.Fprintf(w, "reconcile --apply: held %d row(s) — writing them would add a lint PROBLEM; not written\n", len(res.Held))
		}
		for _, h := range res.Held {
			fmt.Fprintf(w, "  %-40s %s -> %s   [%s]\n", h.ID, h.From, h.To, h.Witness)
			for _, p := range h.Problems {
				fmt.Fprintf(w, "      would add: %s\n", p)
			}
		}
	}
}
