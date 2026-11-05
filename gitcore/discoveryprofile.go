package gitcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NativeFilesystemReadReason observes only filesystem identities on an already
// resolved physical cwd-to-canonical-root chain. It discovers no repository,
// parses no mount table and changes no Toplevel behavior for existing consumers.
// Native Git owns discovery whenever a boundary or unsupported identity platform
// is positively observed; filesystem observation errors remain uncertainty.
func NativeFilesystemReadReason(physicalDir, canonicalRoot string) (string, error) {
	return filesystemReadReason(physicalDir, canonicalRoot, os.Stat, platformDeviceIdentity)
}

func filesystemReadReason(dir, root string, stat func(string) (os.FileInfo, error), device func(os.FileInfo) (uint64, bool)) (string, error) {
	if !filepath.IsAbs(dir) || !filepath.IsAbs(root) {
		return "", fmt.Errorf("filesystem observation requires absolute physical paths")
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("filesystem observation root does not contain cwd")
	}
	current, err := stat(dir)
	if err != nil {
		return "", err
	}
	if !current.IsDir() {
		return "", fmt.Errorf("repository command context is not a directory")
	}
	for dir != root {
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("filesystem observation did not reach canonical root")
		}
		next, err := stat(parent)
		if err != nil {
			return "", err
		}
		if !next.IsDir() {
			return "", fmt.Errorf("filesystem observation encountered a non-directory")
		}
		a, aok := device(current)
		b, bok := device(next)
		if !aok || !bok {
			return "filesystem boundary identity is unavailable on this platform; preserve native discovery", nil
		}
		if a != b {
			return "filesystem boundary occurs before canonical root; preserve native discovery", nil
		}
		dir, current = parent, next
	}
	return "", nil
}
