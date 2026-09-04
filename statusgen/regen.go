package main

// regen.go — the `statusgen regen --readmes` verb (derived-board/04).
//
// It (re)writes the marker-wrapped Briefs region of every `board: generated`
// stream README from the brief frontmatter (readmetable.go), leaving everything
// outside the markers untouched, and is idempotent: a second run changes nothing.
// STATUS.md is NOT its business — that stays the default (no-subcommand) regen's
// single-writer job; this verb only touches stream READMEs.
//
// When a reconcile READ is available (`--repo owner/name`, online), it also folds
// the PR witnesses through DeriveLifecycle and prints a NOTICE for every board
// cell that DISAGREES with the derived state — the interim drift comparator the
// assay-toolkit#2080 Q3 ruling asks for (assertedVsDerivedNotices). Offline
// (`--offline`, or no `--repo`) every PR-derived cell is `unknown`, so no drift
// NOTICE fires — a could-not-check is never rendered as a drift.

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func runRegen(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("regen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	readmes := fs.Bool("readmes", false, "regenerate the marker-wrapped Briefs table in every board: generated stream README")
	root := fs.String("root", ".", "repository root whose stream READMEs to regenerate")
	offline := fs.Bool("offline", false, "do not touch the network; PR-derived cells are unknown, so no drift NOTICE fires")
	repo := fs.String("repo", "", "owner/name to read PRs from for the drift comparator (online mode)")
	tokenFile := fs.String("token-file", "", "file holding the GitHub API token (else GITHUB_TOKEN)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return reconcileOK
		}
		return reconcileUsageErr
	}
	if !*readmes {
		fmt.Fprintln(stderr, "statusgen regen: nothing to do — pass --readmes to regenerate stream README tables")
		return reconcileUsageErr
	}

	boardRoot, found := findBoardRoot(*root)
	if !found {
		// A tree with no docs/streams is a valid empty board — nothing to regen.
		return reconcileOK
	}
	streams, _, err := loadStreams(boardRoot)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen regen: could-not-check: %v\n", err)
		return reconcileUsageErr
	}

	rc := reconcileOK
	for _, s := range streams {
		if s.Board != "generated" {
			// Per brief-04: an ungenerated board is rendered as today (hand table)
			// and NOTICEd — the per-stream migration signal brief 06 flips fleet-wide.
			fmt.Fprintf(stderr, "NOTICE: %s README: ungenerated board (no board: generated) — its Briefs table stays hand-maintained until the stream is wrapped in markers\n", s.Name)
			continue
		}
		path := s.Dir + "/README.md"
		changed, err := rewriteReadmeRegion(s, path)
		if err != nil {
			fmt.Fprintf(stderr, "PROBLEM: %v\n", err)
			rc = reconcileUsageErr
			continue
		}
		if changed {
			fmt.Fprintf(stdout, "regenerated %s\n", path)
		}
	}

	// Drift comparator — only when a reconcile read is available.
	for _, n := range regenDriftNotices(boardRoot, streams, *repo, *offline, *tokenFile, stderr) {
		fmt.Fprintf(stderr, "NOTICE: %s\n", n)
	}
	return rc
}

// regenDriftNotices folds the PR witnesses through DeriveLifecycle and returns the
// board-vs-witness drift NOTICEs. Offline (or no --repo) it returns nothing: every
// PR-derived cell is `unknown`, which assertedVsDerivedNotices never treats as a
// drift. The reconcile plumbing (idents, ghfetch, DeriveLifecycle) is shared with
// the `reconcile` verb so the two agree by construction.
func regenDriftNotices(boardRoot string, streams []*Stream, repo string, offline bool, tokenFile string, stderr *os.File) []string {
	idents, err := reconcileBriefIdents(boardRoot)
	if err != nil {
		fmt.Fprintf(stderr, "NOTICE: drift comparator skipped — could-not-check briefs: %v\n", err)
		return nil
	}
	in := LifecycleInput{Briefs: idents}
	switch {
	case offline || repo == "":
		in.LookedAt = false
		in.Reason = "offline — the PR fetch was not attempted"
	default:
		token, terr := resolveGitHubToken(tokenFile)
		if terr != nil {
			fmt.Fprintf(stderr, "NOTICE: drift comparator skipped — could-not-check token: %v\n", terr)
			return nil
		}
		prs, lookedAt, reason := newGHClient(token).ListPRs(repo)
		in.PRs = prs
		in.LookedAt = lookedAt
		in.Reason = reason
	}
	derived := DeriveLifecycle(in)
	byStream := map[string][]BriefCell{}
	for _, c := range derived {
		if i := strings.IndexByte(c.ID, '/'); i > 0 {
			byStream[c.ID[:i]] = append(byStream[c.ID[:i]], c)
		}
	}
	var notices []string
	for _, s := range streams {
		if s.Board != "generated" {
			continue
		}
		notices = append(notices, assertedVsDerivedNotices(s, byStream[s.Name])...)
	}
	return notices
}
