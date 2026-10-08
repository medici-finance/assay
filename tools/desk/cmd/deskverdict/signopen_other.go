//go:build !unix

package main

import (
	"errors"
	"os"
)

// errPayloadIsLink is never produced off unix; it keeps sign.go platform-neutral.
var errPayloadIsLink = errors.New("payload path is a link")

// checkPayloadDir checks nothing off unix and returns no handle: Go derives a
// non-unix file mode from attributes, not ACLs, so every writable directory reads
// as 0777 and a write-bit check would refuse every --payload. There, host
// contract H5's directory property is wholly the host's job.
func checkPayloadDir(dir string) (*os.File, error) { return nil, nil }

// openPayloadFile is a plain read-only open off unix (dir is always nil there);
// the caller's Lstat, regular-file and SameFile checks still apply.
func openPayloadFile(dir *os.File, path string) (*os.File, error) { return os.Open(path) }
