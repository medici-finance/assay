//go:build unix

package main

import "syscall"

// readSetNoFollow makes the read-set temp-file open refuse a final-component
// symlink (writeReadSetFile).
const readSetNoFollow = syscall.O_NOFOLLOW
