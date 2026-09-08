//go:build unix

package deskkit

// supportsPOSIXRosterModes reports whether os.Chmod on this platform produces the
// group/world-writable mode bits the POSIX permission test asserts on. It is true
// on unix and false on Windows, where os.FileMode is synthetic; the shared
// TestConfigHomePermissionsEnforced gates its chmod loop on it.
const supportsPOSIXRosterModes = true

// secureTestRosterPaths locks a test roster path down to its owner. On unix a file
// written at 0600 in a 0700 directory already satisfies the check, so this is a
// no-op; the Windows build supplies a real DACL lockdown.
func secureTestRosterPaths(_ ...string) error { return nil }
