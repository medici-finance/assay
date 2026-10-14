//go:build !unix

package cellcadence

// Windows opens no FIFOs through this path and follows no symlink silently past the identity
// check that follows the open.
const probeOpenFlags = 0
