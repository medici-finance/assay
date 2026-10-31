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

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/readercaps/record"
)

// Check reads requirements on stream indices and brief records, including the
// archive. Missing stream directories are legitimate legacy roots; an unreadable
// directory/file is unknown, never evidence that it declares no requirement.
func Check(roots []string, reader readercaps.Advertisement) readercaps.Result {
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
				if entry.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("cannot establish requirements through symlink: %s", path)
				}
				if entry.IsDir() || !(entry.Name() == "README.md" || strings.HasPrefix(entry.Name(), "brief-") && strings.HasSuffix(entry.Name(), ".md")) {
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
