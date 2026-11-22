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
		} else {
			notices = append(notices, fmt.Sprintf("%s: [declared-files] COULD-NOT-CHECK declaration: %v", path, err))
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
	// Compare the paths the readers denote. The diagnostic reader preserves
	// brace patterns, directory slashes and annotations in its raw strings.
	var diagnosticPaths []string
	for _, raw := range bf.DeclaredPaths {
		expanded, err := briefdoc.ExpandDeclaration(briefdoc.TrimScopeAnnotation(raw))
		if err != nil {
			diagnosticPaths = append(diagnosticPaths, raw)
		} else {
			diagnosticPaths = append(diagnosticPaths, expanded...)
		}
	}
	slices.Sort(diagnosticPaths)
	diagnosticPaths = slices.Compact(diagnosticPaths)
	slices.Sort(strictPaths)
	// Report the first disagreement, in stable order; dumping both lists can
	// otherwise print thousands of entries for a single expanded declaration.
	for i := 0; i < max(len(strictPaths), len(diagnosticPaths)); i++ {
		if i < len(strictPaths) && i < len(diagnosticPaths) && strictPaths[i] == diagnosticPaths[i] {
			continue
		}
		reader := "authoring diagnostics"
		var entry string
		if i == len(diagnosticPaths) || i < len(strictPaths) && strictPaths[i] < diagnosticPaths[i] {
			reader, entry = "declared inputs", strictPaths[i]
		} else {
			entry = diagnosticPaths[i]
		}
		notices = append(notices, fmt.Sprintf("%s: [declared-files] Context files resolve to different paths: only %s includes %q", path, reader, entry))
		break
	}
	return notices
}
