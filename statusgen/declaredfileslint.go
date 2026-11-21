package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

// Lint both readers at their shared authoring boundary. The permissive reader
// still serves risk diagnostics on syntax the strict input grammar refuses.
func declaredFilesNotices(root, path string, bf *BriefFile, reg *graphRepos, tags []string) []string {
	var notices []string
	seenTags := map[string]bool{}
	for _, tag := range tags {
		if seenTags[tag] {
			continue
		}
		seenTags[tag] = true
		if reg == nil {
			notices = append(notices, fmt.Sprintf("[declared-file-alias] %s: COULD-NOT-CHECK repository alias `%s` without a readable graph-repos.yaml", path, tag))
		} else if _, ok := reg.Aliases[tag]; !ok {
			notices = append(notices, fmt.Sprintf("[declared-file-alias] %s: unknown repository alias `%s` in Context files; use an alias from graph-repos.yaml", path, tag))
		}
	}
	entries, err := briefdoc.DeclaredFileLocations(bf.Body)
	if err != nil {
		var refusal *briefdoc.Refusal
		if errors.As(err, &refusal) {
			notices = append(notices, fmt.Sprintf("%s: [declared-files] line %d: %s", path, bf.bodyLineOffset+refusal.Line, refusal.Msg))
		}
		return notices
	}
	var strictPaths []string
	for _, decl := range entries {
		strictPaths = append(strictPaths, decl.Path)
		if decl.Planned || strings.ContainsAny(decl.Path, "/?*[") {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, decl.Path)); os.IsNotExist(err) {
			notices = append(notices, fmt.Sprintf("%s: [declared-files] line %d: missing root path %q; use its repository-relative path or mark a new file (planned)", path, bf.bodyLineOffset+decl.Line, decl.Path))
		} else if err != nil {
			notices = append(notices, fmt.Sprintf("%s: [declared-files] line %d: COULD-NOT-CHECK root path %q: %v", path, bf.bodyLineOffset+decl.Line, decl.Path, err))
		}
	}
	diagnosticPaths := slices.Clone(bf.DeclaredPaths)
	slices.Sort(diagnosticPaths)
	slices.Sort(strictPaths)
	diagnosticPaths = slices.Compact(diagnosticPaths)
	if !slices.Equal(diagnosticPaths, strictPaths) {
		notices = append(notices, fmt.Sprintf("%s: [declared-files] Context files resolve to different paths for declared inputs %q and authoring diagnostics %q", path, strictPaths, diagnosticPaths))
	}
	return notices
}
