//go:build !unix && !windows

package commstransport

import (
	"fmt"
	"os"
)

// Ownership cannot be established on this platform, so the endpoint is refused.
func ownedByCurrentUser(path string, _ os.FileInfo) error {
	return fmt.Errorf("comms endpoint owner cannot be established on this platform: %s", path)
}
