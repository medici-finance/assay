package main

// briefdelivery.go — a merged PR the brief itself declares a PARTIAL delivery is not the witness
// that the brief is implemented (#2703).
//
// THE DEFECT CLASS. The `Brief:`-trailer fold (lifecycle.go) and the --backfill branch/body
// match (reconcilebackfill.go) take the latest merged PR that names a brief as the proof that
// the brief is implemented. briefauthoring.go removes the PRs that only WROTE the brief, by
// their file shape. A second kind of carrier has a shape no file list can classify: a skeleton
// (a report with every cell still "not yet run") or one slice of a multi-PR delivery. Its files
// are real deliverable files, yet the brief is not done.
//
// THE SUPPORTED PATH. The brief already has a structured, human-authored record of exactly this:
// its `delivery:` frontmatter list, `{in: "<alias>#<N>", covers: full|partial, note: ...}`
// (model.go DeliveryClaim). Until now only the sibling-merge detector read it. This file reads
// the same claims for the repo being reconciled: a `covers: partial` entry whose alias resolves
// (graph-repos.yaml) to the reconciled repo marks that merged PR Partial, and classifyPRs counts
// a Partial merge as work in progress, never as the implementation. A `full` entry for the same
// PR supersedes the partial (the list is append-only history). Entries naming another repo are
// the sibling-merge detector's business and are ignored here.
//
// IT ONLY REMOVES WITNESSES. A claim can turn a merge from `implemented` into `in-progress`; it
// can never create a witness. A brief with no `delivery:` list derives exactly as before.
//
// THREE-STATE. A partial claim whose `in:` cannot be resolved — a malformed ref, an alias the
// registry does not know, a withheld (unpublished) alias, or no readable registry — cannot be
// said to name this repo or not. The brief derives `unknown` with the reason, and the reason is
// disclosed as unread, which refuses --apply. It is never guessed in either direction.

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// declaredPartials is the result of reading every brief's `delivery:` claims against one repo.
type declaredPartials struct {
	// byKey maps a canonical brief key to the PR numbers it declares partial (and not full).
	byKey map[string]map[int]bool
	// undecided maps a brief ident id to the reason one of its partial claims could not be
	// resolved.
	undecided map[string]string
	// unread is the disclosure for each unresolved brief, sorted.
	unread []string
}

// partial reports whether the brief keyed by briefID declares PR number a partial delivery.
func (d declaredPartials) partial(number int, briefID string) bool {
	return d.byKey[canonicalBriefKey(briefID)][number]
}

// readDeclaredPartials reads the `delivery:` claims of every brief under boardRoot and resolves
// them against repo through the tree's graph-repos.yaml.
func readDeclaredPartials(boardRoot, repo string) declaredPartials {
	out := declaredPartials{byKey: map[string]map[int]bool{}, undecided: map[string]string{}}
	data, _, err := loadReconcileFoldData(boardRoot)
	if err != nil {
		// The fold reads fail in wireFoldInputs too, which already makes every brief unknown and
		// discloses why; nothing here can add to that.
		return out
	}
	var reg *graphRepos
	regWhy := ""
	if r, found, rerr := loadGraphRepos(boardRoot); rerr != nil {
		regWhy = fmt.Sprintf("the alias registry could not be read (%v)", rerr)
	} else if !found {
		regWhy = "the tree has no docs/streams/graph-repos.yaml to resolve the alias against"
	} else {
		reg = r
	}
	for _, d := range data {
		key := canonicalBriefKey(d.ident.ID)
		full := map[int]bool{}
		partial := map[int]bool{}
		for _, c := range d.delivery {
			cov := strings.TrimSpace(c.Covers)
			if cov != "full" && cov != "partial" {
				continue // not a claim either reader acts on
			}
			n, here, why := resolveDeliveryRef(c.In, repo, reg, regWhy)
			if why != "" {
				if cov == "partial" {
					if _, taken := out.undecided[d.ident.ID]; !taken {
						out.undecided[d.ident.ID] = fmt.Sprintf("could-not-check the delivery: claim %q of %s: %s — %s derives unknown, not a witnessed implemented",
							c.In, d.ident.ID, why, d.ident.ID)
					}
				}
				continue
			}
			if !here {
				continue
			}
			if cov == "full" {
				full[n] = true
			} else {
				partial[n] = true
			}
		}
		for n := range partial {
			if full[n] {
				continue
			}
			if out.byKey[key] == nil {
				out.byKey[key] = map[int]bool{}
			}
			out.byKey[key][n] = true
		}
	}
	for _, why := range out.undecided {
		out.unread = append(out.unread, why)
	}
	sort.Strings(out.unread)
	return out
}

// resolveDeliveryRef resolves one `in:` ref ("<alias>#<N>") against repo. here reports whether
// the alias names repo; why is non-empty when the question cannot be answered.
func resolveDeliveryRef(ref, repo string, reg *graphRepos, regWhy string) (n int, here bool, why string) {
	ref = strings.TrimSpace(ref)
	alias, num, ok := strings.Cut(ref, "#")
	if !ok || alias == "" {
		return 0, false, "it is not an <alias>#<N> ref"
	}
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 {
		return 0, false, "it is not an <alias>#<N> ref"
	}
	if reg == nil {
		return 0, false, regWhy
	}
	e, known := reg.Aliases[alias]
	switch {
	case !known:
		return 0, false, fmt.Sprintf("alias %q is not in the registry", alias)
	case e.Unpublished || e.Repo == "":
		return 0, false, fmt.Sprintf("alias %q is withheld in this tree's registry", alias)
	}
	return n, strings.EqualFold(e.Repo, repo), ""
}

// markDeclaredPartials sets Partial on every merged trailer PR the brief its trailer names
// declares a partial delivery.
func markDeclaredPartials(prs []PRRecord, d declaredPartials, stderr io.Writer) {
	for i := range prs {
		pr := &prs[i]
		if pr.State != prMerged || !d.partial(pr.Number, pr.BriefRef) {
			continue
		}
		pr.Partial = true
		if stderr != nil {
			fmt.Fprintf(stderr, "reconcile: NOTICE — %s names %s, but the brief's delivery: list declares it a partial delivery, "+
				"so it is work in progress, not a witness that the brief is implemented\n", pr.prLabel(), canonicalBriefKey(pr.BriefRef))
		}
	}
}
