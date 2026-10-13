//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func openMaintenanceDirectory(path string) (*os.File, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	volume := filepath.VolumeName(path)
	// Keep the initial Root open on a drive root, never a caller-selected device or
	// named-pipe namespace. Other namespaces fail closed for offline maintenance.
	if len(volume) != 2 || volume[1] != ':' {
		return nil, fmt.Errorf("maintenance directory requires a drive-rooted path")
	}
	root, err := os.OpenRoot(volume + `\`)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name := strings.TrimLeft(path[len(volume):], `\`)
	if name == "" {
		name = "."
	}
	// Root.OpenRoot uses directory-only open semantics while resolving components,
	// unlike ordinary os.Open followed by a type check. Open the verified handle's
	// own directory rather than reopening the original replaceable path.
	dir, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.Open(".")
}
