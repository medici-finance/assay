package main

// topology.go — registry-resolved cross-repo aliasing for the human done-close and for the
// citation lint (the fleet topology contract, item (d)).
//
// THE DEFECT CLASS IT CLOSES. A reference that names a repo by ALIAS — `<alias>:<stream>/<NN>`,
// `<alias>#<NNN>`, `<cell>:<alias>:<stream>/<NN>`, or a brief-v2 id `<cell>:<alias>:<stream>:<NN>`
// — was reduced to its local `<stream>/<NN>` by dropping the alias, with no look at the alias
// registry (docs/streams/graph-repos.yaml, schema graph-repos-v1). For a flip that is a misroute:
// `--close-verify at:…:06` (a brief tracked in ANOTHER repo) flipped THIS tree's same-numbered
// brief to `done` under a human sign-off nobody gave for it. For a citation it is a silent pass:
// a `sources:`/`consumers:` entry or an Evidence claim naming an alias nobody registered read as
// resolved because nothing tried to resolve it.
//
// THE RULE. The registry is the ONLY resolution path, never a hardcoded map and never a guess
// from a name's resemblance to an alias:
//
//   - resolveLocalBriefRef (the close side) reduces an aliased ref to this tree's local
//     `<stream>/<NN>` ONLY when the alias resolves through the registry to this tree's own
//     alias. A registered alias that is some other repo REFUSES (exit 5) naming both; an alias
//     the registry does not define REFUSES (exit 5); no registry, an unreadable one, or one
//     that cannot say which alias this tree is, is could-not-check (exit 6). Nothing is written
//     on any of those paths.
//   - citationAliasProblems (the lint side) finds every alias-shaped ref in a brief's
//     `sources:`, `consumers:` and `## Evidence` section and resolves it through the same
//     registry with validGraphRef — the grammar validator the dependency-graph edges already
//     use, so the two can never disagree about what a ref means.
//
// The class guard (topology_guard_test.go) pins that no other code path can reach the
// alias-dropping reducer.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Exit codes shared by the two verbs in this file.
const (
	topoExitOK            = 0
	topoExitProblem       = 1 // lint: a ref failed to resolve; close: statusgen refused the flip
	topoExitUsage         = 2
	topoExitRefuse        = 5 // the ref names an alias the registry refuses (unknown, other repo, other cell)
	topoExitCouldNotCheck = 6
)

// refResolveError is a resolution failure with the exit code it maps to. Code is topoExitRefuse
// (the registry answered, and the answer is no) or topoExitCouldNotCheck (the registry could not
// be read or cannot answer).
type refResolveError struct {
	Code int
	Msg  string
}

func (e *refResolveError) Error() string { return e.Msg }

// topoRegistryRel is the registry's tree-relative path, for messages.
const topoRegistryRel = "docs/streams/graph-repos.yaml"

// resolveLocalBriefRef reduces a close ref to the `<stream>/<NN>` it names IN THIS TREE.
//
// A ref with no alias segment (`<stream>/<NN>`) needs no registry and is returned unchanged. A
// ref in none of the recognised shapes is ALSO returned unchanged, so the caller's own grammar
// check refuses it on its own terms. Every aliased shape resolves through the registry, and only
// an alias that is this tree's own yields a local id.
func resolveLocalBriefRef(root, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	var cell, alias, local string
	switch {
	case refInRepoBriefRe.MatchString(ref):
		return ref, nil
	case refCrossCellBriefRe.MatchString(ref):
		parts := strings.SplitN(ref, ":", 3)
		cell, alias, local = parts[0], parts[1], parts[2]
	case refCrossRepoBriefRe.MatchString(ref):
		parts := strings.SplitN(ref, ":", 2)
		alias, local = parts[0], parts[1]
	default:
		c, a, stream, num, ok := parseBriefV2ID(ref)
		if !ok {
			return ref, nil
		}
		cell, alias, local = c, a, stream+"/"+num
	}

	reg, present, err := loadGraphRepos(root)
	if err != nil {
		return "", &refResolveError{topoExitCouldNotCheck, fmt.Sprintf("ref %q names repo alias %q but the alias registry cannot be read: %v", ref, alias, err)}
	}
	if !present {
		return "", &refResolveError{topoExitCouldNotCheck, fmt.Sprintf("ref %q names repo alias %q but this tree carries no %s — the alias cannot be resolved, and it is never guessed", ref, alias, topoRegistryRel)}
	}
	entry, known := reg.Aliases[alias]
	if !known {
		return "", &refResolveError{topoExitRefuse, fmt.Sprintf("ref %q names repo alias %q, which is not in %s — refusing to guess which repo it means; nothing was flipped", ref, alias, topoRegistryRel)}
	}
	if cell != "" && cell != reg.Cell {
		return "", &refResolveError{topoExitRefuse, fmt.Sprintf("ref %q names cell %q, but this tree's %s is cell %q — a cross-cell item closes in its own cell; nothing was flipped", ref, cell, topoRegistryRel, reg.Cell)}
	}
	self, err := loadMigrateRegistry(root)
	if err != nil {
		return "", &refResolveError{topoExitCouldNotCheck, fmt.Sprintf("ref %q names repo alias %q, but this tree's own alias cannot be determined: %v", ref, alias, err)}
	}
	if alias != self.self {
		selfEntry := reg.Aliases[self.self]
		return "", &refResolveError{topoExitRefuse, fmt.Sprintf(
			"ref %q resolves through %s to alias %q (%s), but this tree is alias %q (%s) — a cross-repo item closes in the tree of the repo that tracks it, never here; nothing was flipped",
			ref, topoRegistryRel, alias, describeRegistryRepo(entry), self.self, describeRegistryRepo(selfEntry))}
	}
	return local, nil
}

// describeRegistryRepo renders an entry's repo for a message: the published <owner>/<name>, or
// a statement that it is withheld (never an invented name).
func describeRegistryRepo(e graphRepoEntry) string {
	if e.Unpublished || e.Repo == "" {
		return "repo unpublished in this registry"
	}
	return e.Repo
}

// runVerifyGateClose is `statusgen verify-gate-close --root DIR --ref REF [--dry-run]`: the human
// done-close by a ref in ANY of the reference grammar's brief forms, resolved through the alias
// registry before anything is read or written. It shares closeVerifyPlan with --close-verify, so
// every refusal that path makes (not verified, not gate:human, a held or failed Evidence record,
// a runner below the verifier floor) applies here unchanged. --dry-run computes the flip and
// writes nothing.
//
// Exit: 0 flipped (or, with --dry-run, would flip); 1 the close itself was refused; 2 usage;
// 5 the ref's alias is refused by the registry; 6 could-not-check (no/unreadable registry, or this
// tree's own alias cannot be determined).
func runVerifyGateClose(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify-gate-close", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "tree root carrying docs/streams/")
	ref := fs.String("ref", "", "the item to close: <stream>/<NN>, <alias>:<stream>/<NN>, <cell>:<alias>:<stream>/<NN> or <cell>:<alias>:<stream>:<NN>")
	dryRun := fs.Bool("dry-run", false, "resolve and compute the flip, write nothing")
	if err := fs.Parse(args); err != nil {
		return topoExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "statusgen verify-gate-close: unexpected argument %q (the item is passed as --ref)\n", fs.Arg(0))
		return topoExitUsage
	}
	if strings.TrimSpace(*ref) == "" {
		fmt.Fprintln(stderr, "statusgen verify-gate-close: --ref is required")
		return topoExitUsage
	}
	readme, updated, local, err := closeVerifyPlan(*root, *ref, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "statusgen verify-gate-close:", err)
		var rerr *refResolveError
		if errors.As(err, &rerr) {
			return rerr.Code
		}
		return topoExitProblem
	}
	if local != strings.TrimSpace(*ref) {
		fmt.Fprintf(stdout, "resolved %s -> %s through %s\n", strings.TrimSpace(*ref), local, topoRegistryRel)
	}
	if *dryRun {
		fmt.Fprintf(stdout, "would mark %s done (Reviewed: %s) — dry run, nothing written\n", local, verifyReviewer)
		return topoExitOK
	}
	if err := os.WriteFile(readme, updated, 0o644); err != nil {
		fmt.Fprintln(stderr, "statusgen verify-gate-close:", err)
		return topoExitProblem
	}
	fmt.Fprintf(stdout, "marked %s done (Reviewed: %s)\n", local, verifyReviewer)
	return topoExitOK
}

// ---- lint side -------------------------------------------------------------------------------

// citationRefRe finds an alias-shaped ref inside free text: `<alias>:<stream>/<NN>`,
// `<cell>:<alias>:<stream>/<NN>` or `<alias>#<NNN>`. Group 1 is the ref. The leading group keeps
// a ref from starting mid-token (a path's `.md#12`, a URL's `host:port/…`, the tail of a longer
// colon chain), and the stream segment must start with a letter so a `host:8080/1` port never
// reads as a ref.
var citationRefRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./:#-])((?:[a-z0-9]+:)?[a-z0-9]+:[a-z][a-z0-9-]*/[0-9]+[a-z]?|[a-z0-9]+#[0-9]+)\b`)

// citationRefs returns every alias-shaped ref in s, in order.
func citationRefs(s string) []string {
	var out []string
	for _, m := range citationRefRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// citationAliasReport is the three-state result of the citation-alias-resolution check.
type citationAliasReport struct {
	Problems  []string // a ref whose alias did not resolve through the registry
	Unchecked []string // a brief file that could not be parsed, so its refs were not read
	Refs      int      // aliased refs examined
	Files     int      // brief files whose fields were read
}

// citationAliasProblems runs the check over every stream under root. Only aliased refs are
// judged: an in-repo `<stream>/<NN>` or `#<NNN>` needs no registry and is not this check's
// concern. A brief with no `schema:` (legacy) carries none of the fields and is skipped; one that
// declares a schema but fails to parse is reported Unchecked — never counted clean.
func citationAliasProblems(root string) (citationAliasReport, error) {
	var rep citationAliasReport
	streams, _, err := loadStreams(root)
	if err != nil {
		return rep, err
	}
	reg, _, regErr := loadGraphRepos(root)
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, perr := parseBriefFile(path)
			if perr != nil {
				rep.Unchecked = append(rep.Unchecked, fmt.Sprintf("%s: not parsed, its refs were not checked: %v", path, perr))
				continue
			}
			if !ok || bf == nil {
				continue
			}
			rep.Files++
			fields := []struct {
				name string
				text []string
			}{
				{"sources", bf.Sources},
				{"consumers", append(append([]string{}, bf.Consumers...), bf.ConsumersProse)},
				{"Evidence", []string{bf.Evidence}},
			}
			for _, f := range fields {
				for _, text := range f.text {
					for _, ref := range citationRefs(text) {
						rep.Refs++
						if regErr != nil {
							rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s ref %q cannot be resolved — the alias registry is unreadable: %v", path, f.name, ref, regErr))
							continue
						}
						if ok, reason := validGraphRef(ref, reg); !ok {
							rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s ref %s", path, f.name, reason))
						}
					}
				}
			}
		}
	}
	sort.Strings(rep.Problems)
	sort.Strings(rep.Unchecked)
	return rep, nil
}

// lintChecks is the named-check registry of `statusgen lint --check`. A name not listed here is
// refused with exit 2, so an acceptance command naming a check that does not exist yet fails
// loudly instead of passing.
var lintChecks = map[string]func(root string, stdout, stderr io.Writer) int{
	"citation-alias-resolution": runCitationAliasCheck,
}

func lintCheckNames() string {
	names := make([]string, 0, len(lintChecks))
	for n := range lintChecks {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// checkList is a repeatable --check flag.
type checkList []string

func (c *checkList) String() string     { return strings.Join(*c, ",") }
func (c *checkList) Set(v string) error { *c = append(*c, v); return nil }

// runLintNamed is `statusgen lint --root DIR --check NAME [--check NAME …]`: run the named
// checks only. The whole-corpus gate stays `statusgen --lint`; this verb exists so one contract
// check can be asserted on its own. Exit: 1 when any check found a problem (never masked by
// another check's could-not-check), else 6 when any check could not look, else 0.
func runLintNamed(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "tree root carrying docs/streams/")
	var checks checkList
	fs.Var(&checks, "check", "a named check to run (repeatable): "+lintCheckNames())
	if err := fs.Parse(args); err != nil {
		return topoExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "statusgen lint: unexpected argument %q\n", fs.Arg(0))
		return topoExitUsage
	}
	if len(checks) == 0 {
		fmt.Fprintf(stderr, "statusgen lint: --check is required (known: %s); the whole-corpus lint is `statusgen --lint`\n", lintCheckNames())
		return topoExitUsage
	}
	for _, c := range checks {
		if _, ok := lintChecks[c]; !ok {
			fmt.Fprintf(stderr, "statusgen lint: unknown check %q (known: %s) — nothing was checked\n", c, lintCheckNames())
			return topoExitUsage
		}
	}
	problem, couldNotCheck := false, false
	for _, c := range checks {
		switch lintChecks[c](*root, stdout, stderr) {
		case topoExitOK:
		case topoExitCouldNotCheck:
			couldNotCheck = true
		default:
			problem = true
		}
	}
	switch {
	case problem:
		return topoExitProblem
	case couldNotCheck:
		return topoExitCouldNotCheck
	}
	return topoExitOK
}

// runCitationAliasCheck prints the citation-alias-resolution result: PROBLEM and UNCHECKED lines
// to stderr, one summary line to stdout. Exit 0 clean, 1 on any problem, 6 when nothing failed
// but some brief could not be read (could-not-check, never rounded up to clean).
func runCitationAliasCheck(root string, stdout, stderr io.Writer) int {
	rep, err := citationAliasProblems(root)
	if err != nil {
		fmt.Fprintf(stderr, "citation-alias-resolution: could-not-check: %v\n", err)
		return topoExitCouldNotCheck
	}
	for _, p := range rep.Problems {
		fmt.Fprintln(stderr, "PROBLEM:", p)
	}
	for _, u := range rep.Unchecked {
		fmt.Fprintln(stderr, "UNCHECKED:", u)
	}
	fmt.Fprintf(stdout, "citation-alias-resolution: %d aliased ref(s) in %d brief file(s) resolved through %s — %d problem(s), %d unchecked file(s)\n",
		rep.Refs, rep.Files, topoRegistryRel, len(rep.Problems), len(rep.Unchecked))
	switch {
	case len(rep.Problems) > 0:
		return topoExitProblem
	case len(rep.Unchecked) > 0:
		return topoExitCouldNotCheck
	}
	return topoExitOK
}
