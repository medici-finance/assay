// Package tree is the filesystem host adapter for reader preflight. The pure
// readercaps package does not import it. No credential or effect authority lives
// here; consumers retain their own current-subject and authorization checks.
package tree

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/readercaps/record"
)

// recordParts follows the existing canonical reader layout: each stream's
// index and direct briefs, plus its done briefs, under active or archived roots.
// Prose/example subdirectories are not alternative sources of operational records.
func recordParts(parts []string) bool {
	brief := strings.HasPrefix(parts[len(parts)-1], "brief-") && strings.HasSuffix(parts[len(parts)-1], ".md")
	return len(parts) == 2 && (parts[1] == "README.md" || brief) || len(parts) == 3 && parts[1] == "done" && brief
}

// Check reads requirements on stream indices and brief records, including the
// archive. Missing stream directories are legitimate legacy roots; an unreadable
// directory/file is unknown, never evidence that it declares no requirement.
func Check(roots []string, reader readercaps.Advertisement) readercaps.Result {
	registerNames := briefdoc.ReservedRegisterNames()
	for _, root := range roots {
		for _, base := range []string{"streams", "archive"} {
			dir := filepath.Join(root, "docs", base)
			info, err := os.Lstat(dir)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return readercaps.Result{State: readercaps.CouldNotCheck, Reason: err.Error()}
			}
			if !info.IsDir() {
				return readercaps.Result{State: readercaps.CouldNotCheck, Reason: "requirements directory is not a regular directory: " + dir}
			}
			result := readercaps.Result{State: readercaps.Supported}
			err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if entry.Type()&os.ModeSymlink != 0 {
					if len(parts) == 2 && parts[1] == "done" || recordParts(parts) {
						return fmt.Errorf("cannot establish requirements through symlink: %s", path)
					}
					return nil
				}
				if entry.IsDir() {
					if len(parts) == 1 && rel != "." {
						if registerNames[entry.Name()] {
							return fs.SkipDir
						}
						readme := filepath.Join(path, "README.md")
						if info, err := os.Lstat(readme); err == nil && info.Mode().IsRegular() {
							raw, err := os.ReadFile(readme)
							if err != nil {
								return err
							}
							if briefdoc.IsRegisterIndex(raw) {
								return fs.SkipDir
							}
						}
					}
					if rel == "." || len(parts) == 1 || len(parts) == 2 && parts[1] == "done" {
						return nil
					}
					return fs.SkipDir
				}
				if !recordParts(parts) {
					return nil
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				required, err := record.Requirements(raw)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				result = readercaps.Assess(required, readercaps.Observation{State: readercaps.Current, Advertisement: reader})
				if result.State != readercaps.Supported {
					result.Reason = path + ": " + result.Reason
					return fs.SkipAll
				}
				return nil
			})
			if err != nil {
				return readercaps.Result{State: readercaps.CouldNotCheck, Reason: err.Error()}
			}
			if result.State != readercaps.Supported {
				return result
			}
		}
	}
	return readercaps.Result{State: readercaps.Supported}
}
