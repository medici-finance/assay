package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// witnessinputs_lint.go — the `--lint` half of the `inputs:` Context field
// (statusgen/36). The parse and the literal-path lower bound are the
// library's (evaluation.ParseInputsDeclaration, evaluation.InputsBoundViolations);
// this file only walks the briefs and reports.

// witnessInputsProblems reports, per brief:
//   - an `inputs:` declaration that cannot be read (scope falls back to
//     whole-tree, which the author should know) or that names bookkeeping;
//   - for a brief with a readable declaration, every Verify row whose Command
//     names a repo path literally that lies outside the declared set — the
//     same check the reader refuses as witness-input-undeclared.
func witnessInputsProblems(root string, streams []*Stream) []string {
	var problems []string
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed (reported elsewhere) or legacy/opted-out
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			id := s.Name + "/" + num
			declared, found, problem := evaluation.ParseInputsDeclaration(extractSectionByPrefix(bf.Body, "Context"))
			if problem != "" {
				problems = append(problems, fmt.Sprintf("%s: %s — inputs-undeclared", id, problem))
			}
			if !found {
				continue
			}
			for _, r := range briefVerifyRows(bf.Verify) {
				row := evaluation.VerifyRow{ID: r.ID, Command: r.Command, Shell: r.Shell}
				viol := evaluation.InputsBoundViolations(declared, row, func(p string) (bool, bool) {
					fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
					if err != nil {
						return false, false
					}
					return true, fi.IsDir()
				})
				for _, p := range viol {
					problems = append(problems, fmt.Sprintf(
						"%s: Verify row %s names %s, which lies outside the declared `inputs:` — a witness for it would not bind that path (witness-input-undeclared); add it to `inputs:` or drop the field",
						id, r.ID, p))
				}
			}
		}
	}
	sort.Strings(problems)
	return problems
}
