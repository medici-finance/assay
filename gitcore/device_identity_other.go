//go:build !unix

package gitcore

import "os"

// Unknown platform identities are an explicit native discovery capability,
// rather than an assumed same-filesystem answer.
func platformDeviceIdentity(os.FileInfo) (uint64, bool) { return 0, false }
