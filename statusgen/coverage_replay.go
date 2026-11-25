package main

import (
	"encoding/json"
	"fmt"
	"io"
)

type coverageReplaySide struct {
	Revision string   `json:"revision"`
	Coverage Coverage `json:"coverage"`
}

type coverageReplayReport struct {
	Advisory bool               `json:"advisory"`
	Head     coverageReplaySide `json:"head"`
	Replay   coverageReplaySide `json:"replay"`
}

// runCoverageReplay judges the same checkout brief twice with the existing
// coverage adapter. The supplied commit is a diagnostic input; this path does
// not resolve a trusted delivery or advance the brief.
func runCoverageReplay(root, brief, revision string, jsonMode bool, out, diagnostic io.Writer) int {
	fail := func(err any) int {
		fmt.Fprintf(diagnostic, "statusgen: coverage replay: %v\n", err)
		return coverageExitCouldNot
	}
	stream, num, ok := briefStreamNum(brief)
	if !ok || brief != stream+"/"+num {
		return fail("--brief must be in <stream>/<NN> form")
	}
	if !fullObjectIDRe.MatchString(revision) {
		return fail("--coverage-replay-at requires a full local commit SHA")
	}
	if activeGitReads == nil {
		defer beginGitReadSession()()
	}
	repo := coverageRepo{root: root}
	head, ok := repo.ResolveCommit("HEAD")
	if !ok {
		return fail("checkout HEAD could not be read")
	}
	target, ok := repo.ResolveCommit(revision)
	if !ok || target != revision {
		return fail("replay commit does not resolve to that commit in local history")
	}
	if ancestor, err := historyAncestor(repo, target, head); err != nil || !ancestor {
		return fail("replay commit must exist in local history and be an ancestor of checkout HEAD")
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		return fail(err)
	}
	var selected *Stream
	var path string
	for _, s := range streams {
		if s.Name != stream {
			continue
		}
		for _, candidate := range briefFilePaths(s) {
			if id, _, ok := expectedBriefID(candidate); ok && id == brief {
				if path != "" {
					return fail("brief resolves to multiple files")
				}
				selected, path = s, candidate
			}
		}
	}
	if path == "" {
		return fail("brief not found in the checkout")
	}
	bf, ok, err := parseBriefFile(path)
	if err != nil || !ok {
		return fail("brief could not be read as structured coverage input")
	}
	if unboundCrossAttestation(bf.Evidence) {
		return fail("cross-repository Evidence requires a separate source revision")
	}
	patterns, _ := loadPatternDocs(root)
	selection := &coverageSelection{path: path, file: bf}
	evaluate := func(commit string) coverageReplaySide {
		opts := coverageOptionsFor(root, patterns)
		opts.Revision, opts.selection = commit, selection
		return coverageReplaySide{commit, evaluateCoverage(root, []*Stream{selected}, opts)[brief]}
	}
	report := coverageReplayReport{Advisory: true, Head: evaluate(head), Replay: evaluate(target)}
	if report.Head.Coverage.BriefID == "" || report.Replay.Coverage.BriefID == "" {
		return fail("coverage observations could not be loaded")
	}
	if jsonMode {
		if err := json.NewEncoder(out).Encode(report); err != nil {
			return fail(err)
		}
		return coverageExitOK
	}
	fmt.Fprintln(out, "Coverage replay uses the checkout's Verify and Evidence sections. Results are advisory and grant no closure credit.")
	for _, row := range []struct {
		label string
		side  coverageReplaySide
	}{{"HEAD", report.Head}, {"Replay", report.Replay}} {
		fmt.Fprintf(out, "%s %s  %s\n", row.label, row.side.Revision, coverageLine(row.side.Coverage))
		for _, claim := range row.side.Coverage.Claims {
			fmt.Fprintf(out, "  %s  %s  %s\n", claim.Claim, claim.Result, claim.Reason)
		}
	}
	return coverageExitOK
}
