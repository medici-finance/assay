package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/gitcore"
)

// runPreviewStamps is a local authoring aid. A candidate is a question for the
// online corroborator. This command never looks up an approval or emits one.
func runPreviewStamps(args []string, out, stderr io.Writer) int {
	flags := flag.NewFlagSet("preview-stamps", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "checkout root")
	base := flags.String("base-ref", remoteMainRef, "exact local ref for the intended PR base")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "usage: statusgen preview-stamps [--root DIR] [--base-ref refs/remotes/origin/main]")
		return 2
	}
	top, err := gitcore.Toplevel(*root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "preview-stamps: could not resolve checkout root: %v\n", err)
		return 2
	}
	repo, err := gitcore.Open(top)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "preview-stamps: could not open local object database: %v\n", err)
		return 2
	}
	// The exact ref and all base blobs are read from this object store only.
	// A missing promisor object must never prompt a lazy fetch.
	if !strings.HasPrefix(*base, "refs/") || plumbing.ReferenceName(*base).Validate() != nil {
		_, _ = fmt.Fprintf(stderr, "preview-stamps: invalid fully qualified base ref %q\n", *base)
		return 2
	}
	baseID, found, err := repo.LocalRefHash(*base)
	if err != nil || !found || !isObjectID(baseID) {
		_, _ = fmt.Fprintf(stderr, "preview-stamps: could not read exact local ref %q\n", *base)
		return 2
	}
	head, err := repo.HeadID()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "preview-stamps: could not resolve local HEAD: %v\n", err)
		return 2
	}
	mb, err := repo.MergeBase(baseID, head)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "preview-stamps: could not resolve the local merge base: %v\n", err)
		return 2
	}
	patch, err := repo.Diff(mb, head, 3)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "preview-stamps: could not read the local diff: %v\n", err)
		return 2
	}
	markers := map[string]bool{}
	exists := func(marker string) bool {
		if found, seen := markers[marker]; seen {
			return found
		}
		_, err := repo.TreeishID(head, marker)
		markers[marker] = err == nil
		return err == nil
	}
	added, _ := walkAddedDiffLines(patch, func(file string) bool {
		return strings.HasPrefix(file, corroborateExcludedFixturePrefix) || fixtureUnderMarker(file, exists)
	})
	stamps, _ := diffStampClaims(added, func(file string) decisionRecordStamp {
		content, err := repo.FileAt(head, file)
		return parseDecisionRecordStamp(file, content, err)
	})
	markBaseStampCells(stamps, func(file string) (map[string]string, map[string]int) {
		content, err := repo.FileAt(mb, file)
		if err != nil {
			return nil, nil // no proven identical base cell; retain the candidate
		}
		return briefRowsByKey(content), briefHeaderIndex(content)
	})
	_, _ = fmt.Fprintf(out, "Base %s (merge base %s) to committed HEAD %s. Commit staged, unstaged and new files to include them.\n", *base, mb, head)
	claims := 0
	for _, s := range stamps {
		if s.PreExisting {
			continue
		}
		claims++
		if len(s.Locations) == 0 {
			_, _ = fmt.Fprintf(out, "%s (decided-by): human:%s needs corroboration\n", s.File, s.Name)
		}
		for _, line := range s.Locations {
			_, _ = fmt.Fprintf(out, "%s:%d: human:%s needs corroboration\n", s.File, line, s.Name)
		}
	}
	_, _ = fmt.Fprintf(out, "%d claim(s) for online corroboration. PR approvals and other CI gates are checked separately.\n", claims)
	if claims != 0 {
		return 1
	}
	return 0
}
