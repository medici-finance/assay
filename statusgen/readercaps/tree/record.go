package tree

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/readercaps/record"
)

// ReadRecord admits a named record, including paths outside discovery's glob.
// The host supplies its admitted root and consumes these exact checked bytes.
// Observed symlink components refuse. The opened root contains the read even
// during path changes; root/file identity changes while opening also refuse.
func ReadRecord(root, name string, reader readercaps.Advertisement) ([]byte, readercaps.Result) {
	refuse := func(reason string) ([]byte, readercaps.Result) {
		return nil, readercaps.Result{State: readercaps.CouldNotCheck, Reason: readercaps.Diagnostic(reason)}
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.VolumeName(clean) != "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return refuse("named record path escapes its admitted root")
	}
	if root == "" {
		root = "."
	}
	current := root
	info, err := os.Lstat(current)
	if err != nil {
		return refuse(fmt.Sprint(err))
	}
	if !info.IsDir() {
		return refuse("admitted record root is not a regular directory: " + root)
	}
	rootInfo := info
	parts := strings.Split(clean, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err = os.Lstat(current)
		if err != nil {
			return refuse(fmt.Sprint(err))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return refuse("cannot admit named record through symlink: " + current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return refuse("named record parent is not a directory: " + current)
		}
	}
	if !info.Mode().IsRegular() {
		return refuse("named record is not a regular file: " + current)
	}
	openedRoot, err := os.OpenRoot(root)
	if err != nil {
		return refuse(fmt.Sprint(err))
	}
	defer openedRoot.Close()
	openedRootInfo, err := openedRoot.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedRootInfo) {
		return refuse("admitted record root changed while opening")
	}
	file, err := openedRoot.Open(clean)
	if err != nil {
		return refuse(fmt.Sprint(err))
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return refuse("named record changed while opening: " + current)
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return refuse(fmt.Sprint(err))
	}
	result := record.Check(raw, reader)
	if result.State != readercaps.Supported {
		result.Reason = readercaps.Diagnostic(current + ": " + result.Reason)
		return nil, result
	}
	return raw, result
}
