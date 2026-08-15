package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verbs.go — flag parsing and the two verb bodies.

type opts struct {
	repo     string
	pr       int
	repoRoot string
	rulings  string
	probe    bool
	jsonOut  bool
	dryRun   bool
}

// parseArgs is shared by both verbs so their flag surfaces cannot drift. There is no
// --force, no --yes and no environment override: every refusal below is reachable only
// by satisfying it.
func parseArgs(verb string, args []string, allowDryRun bool) (opts, error) {
	var o opts
	fs := flag.NewFlagSet(toolName+" "+verb, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.repo, "R", "", "owner/repo")
	fs.StringVar(&o.repoRoot, "repo-root", "", "local checkout to compute the merge in")
	fs.StringVar(&o.rulings, "rulings", defaultRulingsPath, "path to the rulings register")
	fs.BoolVar(&o.probe, "probe", false, "build the merged tree and report the result")
	fs.BoolVar(&o.jsonOut, "json", false, "emit the report as JSON")
	if allowDryRun {
		fs.BoolVar(&o.dryRun, "dry-run", false, "determine everything, write nothing")
	}
	// flag.Parse stops at the FIRST non-flag token, and the documented usage puts the
	// PR number before the optional flags (`deskmerge check -R <repo> <pr> --probe`).
	// Parsing once would therefore silently drop every flag after the number — the
	// shape where `--dry-run` is accepted, ignored, and the write happens anyway. So
	// the leftovers are collected and parsing resumes past each of them.
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return o, deskkit.Refused("refused: " + deskkit.StripControl(err.Error()))
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
	if strings.TrimSpace(o.repo) == "" {
		return o, deskkit.Refused("refused: -R <owner/repo> is required")
	}
	if len(positional) != 1 {
		return o, deskkit.Refused(fmt.Sprintf(
			"refused: %s takes exactly one PR number, got %d positional arguments", verb, len(positional)))
	}
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(positional[0]), "#"))
	if err != nil || n <= 0 {
		return o, deskkit.Refused("refused: PR must be a positive number, got " +
			deskkit.StripControl(positional[0]))
	}
	o.pr = n

	// The repo set is checked HERE, before any network or filesystem act, for the read
	// verb as well as the write verb. deskmerge fetches from the repo it is pointed at,
	// and a fetch is a reach into someone else's infrastructure whether or not it
	// writes.
	if !deskkit.IsAllowedRepo(o.repo) {
		return o, deskkit.Refused(fmt.Sprintf(
			"refused: %s is outside the configured repo set — deskmerge does not widen it, and an "+
				"UNCONFIGURED set answers no for every repo", deskkit.StripControl(o.repo)))
	}
	return o, nil
}

// cmdCheck is the read verb. It writes NOTHING — no push, no comment, no label, no
// audit-charged act — so it is not ruling-gated: reading is not authorship. This is the
// half of deskmerge that is useful today, while R-5 is unsigned.
func cmdCheck(args []string, out io.Writer) error {
	o, err := parseArgs(verbCheck, args, false)
	if err != nil {
		return err
	}
	root, err := resolveRepoRoot(o.repo, o.repoRoot)
	if err != nil {
		return err
	}
	p, err := fetchPR(o.repo, o.pr)
	if err != nil {
		return err
	}
	defer dropPRHeadRef(root, o.pr)

	t, aerr := assess(root, o.repo, p, o.probe)
	defer t.close()
	if aerr != nil {
		// A could-not-check determination still prints the partial report: which
		// dimensions WERE established is exactly what the reader needs in order to
		// know what remains unknown.
		emit(out, t.rep, o.jsonOut)
		return aerr
	}

	// State the eligibility answer alongside the currency answer. A PR that is
	// perfectly mergeable but already flipped ready is not deskmerge's to fix, and a
	// report that omitted that would send a caller to a verb that will refuse.
	if elig := eligibleForMerge(o.repo, p); elig != nil {
		t.rep.note("deskmerge merge would refuse this PR: " + elig.Error())
	}
	if t.UpToDate {
		t.rep.note("already current with " + p.BaseRefName + " — nothing to merge")
	}
	emit(out, t.rep, o.jsonOut)

	switch code := t.rep.exitCode(); code {
	case deskkit.ExitOK:
		return nil
	case deskkit.ExitRefused:
		return deskkit.Refused("checked-failed: " + failureSummary(t.rep))
	default:
		return deskkit.Unverifiable("could-not-check: at least one dimension could not be examined; "+
			"see the report above", nil)
	}
}

func failureSummary(r *report) string {
	var parts []string
	if r.Mergeability == mergeConflicted {
		parts = append(parts, "conflicts outside the compiled-in regenerable list ("+
			regenerableList()+"): "+strings.Join(r.ConflictedOther, " ")+
			" — this is worker work, not merge-currency work")
	}
	if r.SemanticValidity == stateFailed {
		parts = append(parts, r.ProbeDetail)
	}
	return strings.Join(parts, "; ")
}

func emit(out io.Writer, r *report, asJSON bool) {
	if asJSON {
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			fmt.Fprintf(out, "{\"error\":\"could-not-check: the report did not marshal\"}\n")
			return
		}
		fmt.Fprintln(out, string(b))
		return
	}
	var sb strings.Builder
	r.render(&sb)
	fmt.Fprint(out, sb.String())
}
