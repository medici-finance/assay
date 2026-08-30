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

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// reconcileResult is the machine-readable output shape (--json). Its per-brief
// rows are BriefCell values (id/cell/source/witness/reason/version).
type reconcileResult struct {
	Repo     string      `json:"repo"`
	LookedAt bool        `json:"lookedAt"`
	Reason   string      `json:"reason"`
	Briefs   []BriefCell `json:"briefs"`
}

const (
	reconcileOK       = 0
	reconcileUsageErr = 2
)

func runReconcile(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	root := fs.String("root", ".", "repository root whose briefs to reconcile")
	repo := fs.String("repo", "", "owner/name of the repo to read PRs from (online mode)")
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	offline := fs.Bool("offline", false, "do not touch the network; every PR-derived cell is unknown")
	tokenFile := fs.String("token-file", "", "file holding the GitHub API token (else GITHUB_TOKEN)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return reconcileOK
		}
		return reconcileUsageErr
	}

	idents, err := reconcileBriefIdents(*root)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: %v\n", err)
		return reconcileUsageErr
	}

	in := LifecycleInput{Briefs: idents}
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
		prs, lookedAt, reason := newGHClient(token).ListPRs(*repo)
		in.PRs = prs
		in.LookedAt = lookedAt
		in.Reason = reason
	}

	res := reconcileResult{
		Repo:     *repo,
		LookedAt: in.LookedAt,
		Reason:   in.Reason,
		Briefs:   DeriveLifecycle(in),
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
	streams, _, err := loadStreams(boardRoot)
	if err != nil {
		return nil, err
	}
	var idents []BriefIdent
	seen := map[string]bool{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, perr := parseBriefFile(path)
			var id, gate string
			version := 1
			if perr == nil && ok {
				id, gate, version = bf.Brief, bf.Gate, bf.Version
				if version == 0 {
					version = 1
				}
			} else {
				derived, _, okName := expectedBriefID(path)
				if !okName {
					continue // not a brief file shape we can key on
				}
				id = derived
			}
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			idents = append(idents, BriefIdent{ID: id, Gate: gate, Version: version})
		}
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

func printReconcileTable(w *os.File, res reconcileResult) {
	if !res.LookedAt {
		fmt.Fprintf(w, "reconcile: could-not-check — %s\n", res.Reason)
	} else {
		fmt.Fprintf(w, "reconcile: %s (%d brief(s))\n", res.Repo, len(res.Briefs))
	}
	for _, b := range res.Briefs {
		detail := b.Witness
		if detail == "" {
			detail = b.Reason
		}
		fmt.Fprintf(w, "  %-40s %-12s %s\n", b.ID, b.Cell, detail)
	}
}
