// toolvalidation — assemble the tool-validation evidence pack from the muhar
// mutation reports the release already produces.
//
// Every release, the `test` job mutates each release-gated refusal control and
// requires the suite to catch the injected error — the strongest available
// answer to the auditor's question about an automated gate: "how do you know it
// fires, and when did it last stop something?" (docs/mistake-proofing.md §3 D1).
// That demonstration runs on every release and is then discarded to an expiring
// job log. This tool captures it: it reads the DECLARED set of release-gated
// controls (declaredControls, below), the captured muhar reports for each, and
// a release tag, and writes the evidence pack in two formats from one model —
// tool-validation-<tag>.md for a human and tool-validation-<tag>.json for a
// parser.
//
// It is a RECORD of demonstrations, not a compliance claim: see header.go for
// the non-claim register the generated .md carries (the same register as
// docs/evidence-bundle.md — not an audit opinion, states no conformance).
//
// Exit codes (the docs/evidence-bundle.md contract, copied in spirit):
//
//	0  complete — every declared control had a trustworthy report.
//	3  emitted but INCOMPLETE — one or more declared controls had a report that
//	   was missing, unparseable, or a HARNESS BROKEN discard (muhar exit 2). The
//	   pack still writes, and its `omitted` array names each gap and why. A
//	   silently incomplete evidence pack is a worse outcome than a failed export,
//	   so this is deliberately non-zero and the release step gates on it.
//	1  usage / IO error (bad flags, unreadable spec, unwritable out dir).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// control is one release-gated refusal gate whose mutation demonstration this
// pack records. The set is DECLARED in source, not inferred from whichever
// mutation specs happen to be on disk: a declared set is a list a reviewer can
// read and argue with, and a control silently dropped because its capture went
// missing must show up as a positive `omitted` statement and a non-zero exit,
// never as a quietly smaller pack (the failure mode exit 3 exists to forbid).
type control struct {
	// Gate is the human name of the refusal gate.
	Gate string
	// Spec is the repo-relative path to its muhar mutation spec.
	Spec string
	// ReportKey is the basename the release workflow captures this gate's muhar
	// report under (<ReportKey>.report, or <ReportKey>.<i>.report per shard).
	ReportKey string
	// Shards is 0 for a single-report gate, or N when the sweep is sharded
	// across N release legs (deskmerge). All N shard reports are unioned; any
	// one missing or broken makes the whole gate could-not-check.
	Shards int
	// Why is the one-line rationale for treating this as a release control.
	Why string
}

// declaredControls is THE judgement of this tool: the release-gated refusal
// gates whose "an injected error reddens it" demonstration the release runs and
// this pack records. It is kept in lockstep with the `Gate bar — mutate …`
// steps in .github/workflows/release.yml — each entry here has one such step,
// and each such step has one entry here. assembleDrift() compares this set
// against the mutation specs actually on disk and reports any divergence in
// BOTH directions, so adding a gate spec to the tree without declaring it here
// (or declaring one whose file has gone) is a visible line in the pack, never a
// silent omission.
//
// Scope: this set is the desk-tools refusal gates the RELEASE mutates. It is
// deliberately NOT every *mutations*.json in tools/desk — several such specs
// are exercised elsewhere (PR-time CI, internal fixtures) and are not
// release-blocking controls; assembleDrift() lists those as not-declared so the
// boundary is visible rather than assumed.
var declaredControls = []control{
	{
		Gate:      "bodycheck refusals",
		Spec:      "tools/desk/internal/deskkit/mutations.json",
		ReportKey: "bodycheck",
		Why:       "the shared outward-write body scanner; every desk verb's secret/leak refusal rides on it.",
	},
	{
		Gate:      "desksourceguard refusals",
		Spec:      "tools/desk/cmd/desksourceguard/mutations.json",
		ReportKey: "desksourceguard",
		Why:       "the supply-chain gate between a repointed release tag and a consumer compiling an unreviewed tree.",
	},
	{
		Gate:      "loopengine dedupe-at-start gate",
		Spec:      "tools/desk/internal/loopengine/mutations.json",
		ReportKey: "loopengine-dedupe",
		Why:       "Claim is the single mandatory dispatch entry point; its dedupe-at-start guarantee must be shown to fail.",
	},
	{
		Gate:      "loopengine retry taxonomy",
		Spec:      "tools/desk/internal/loopengine/mutations-retry.json",
		ReportKey: "loopengine-retry",
		Why:       "the retryable / non-retryable / could-not-check split; a mis-sorted failure re-runs a rejected write or drops work.",
	},
	{
		Gate:      "deskmerge refusals and authority gate",
		Spec:      "tools/desk/cmd/deskmerge/mutations.json",
		ReportKey: "deskmerge",
		Shards:    3,
		Why:       "the only tool that rewrites another PR's head; its zero-authorship boundary is entirely negative and unobservable from a green suite.",
	},
	{
		Gate:      "reviewloop idle gate and coalescing",
		Spec:      "tools/desk/cmd/reviewloop/mutations.json",
		ReportKey: "reviewloop",
		Why:       "the board reactor's three-state idle gate; a board it could not read must report could-not-check, never \"nothing to do\".",
	},
	{
		Gate:      "deskclose superseded two-role lane",
		Spec:      "tools/desk/cmd/deskclose/mutations.json",
		ReportKey: "deskclose",
		Why:       "the propose/confirm/dispute boundary; one identity may never both propose and close on the same item.",
	},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses flags, assembles the pack, writes both formats, and returns the
// process exit code. It is factored out of main so tests drive it without an
// os.Exit call, exactly like tools/freshness.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("toolvalidation", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "path to the assay repo root (spec paths resolve here)")
	reports := fs.String("reports", "", "directory of captured muhar reports (<key>.report / <key>.<i>.report)")
	tag := fs.String("tag", "", "release tag this pack records (e.g. v0.13.0)")
	out := fs.String("out", "", "output directory for tool-validation-<tag>.{md,json}")
	date := fs.String("date", "", "run date YYYY-MM-DD stamped on every row (default: today)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *reports == "" || *tag == "" || *out == "" {
		fmt.Fprintln(stderr, "toolvalidation: -reports, -tag and -out are all required")
		return 1
	}

	runDate := time.Now().UTC().Format("2006-01-02")
	if *date != "" {
		if _, err := time.Parse("2006-01-02", *date); err != nil {
			fmt.Fprintf(stderr, "toolvalidation: invalid -date %q: %v\n", *date, err)
			return 1
		}
		runDate = *date
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "toolvalidation: cannot resolve -root: %v\n", err)
		return 1
	}

	pack, err := assemble(absRoot, *reports, *tag, runDate)
	if err != nil {
		fmt.Fprintf(stderr, "toolvalidation: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "toolvalidation: cannot create -out %s: %v\n", *out, err)
		return 1
	}
	mdPath := filepath.Join(*out, "tool-validation-"+*tag+".md")
	jsonPath := filepath.Join(*out, "tool-validation-"+*tag+".json")
	if err := os.WriteFile(mdPath, []byte(pack.renderMarkdown()), 0o644); err != nil {
		fmt.Fprintf(stderr, "toolvalidation: write %s: %v\n", mdPath, err)
		return 1
	}
	jsonBytes, err := pack.renderJSON()
	if err != nil {
		fmt.Fprintf(stderr, "toolvalidation: render json: %v\n", err)
		return 1
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "toolvalidation: write %s: %v\n", jsonPath, err)
		return 1
	}

	if len(pack.Omitted) > 0 {
		fmt.Fprintf(stdout, "toolvalidation: pack emitted INCOMPLETE — %d of %d declared controls omitted; see the omitted list in %s\n",
			len(pack.Omitted), len(declaredControls), jsonPath)
		return 3
	}
	fmt.Fprintf(stdout, "toolvalidation: pack complete — %d declared controls recorded at %s\n", len(declaredControls), *tag)
	return 0
}
