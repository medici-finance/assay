package main

import (
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cellPathCheck is shared by roots and executable launchers. Network and device paths
// are refused on every host: roots cause SMB authentication and launchers execute remotely.
func cellPathCheck(goos, p string) error {
	sep := func(c byte) bool { return c == '/' || c == '\\' }
	if len(p) >= 2 && sep(p[0]) && sep(p[1]) {
		return fmt.Errorf("UNC or device path refused: network roots authenticate over SMB and remote launchers execute code")
	}
	if !deskkit.IsAbsFor(goos, p) {
		return fmt.Errorf("path must be absolute")
	}
	return nil
}
