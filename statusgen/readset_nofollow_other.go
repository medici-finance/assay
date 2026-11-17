//go:build !unix

package main

// readSetNoFollow is zero where the OS has no O_NOFOLLOW; writeReadSetFile's
// Lstat checks and O_EXCL on a fresh name still refuse a planted link.
const readSetNoFollow = 0
