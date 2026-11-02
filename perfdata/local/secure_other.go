//go:build !unix

package local

import "os"

// Ownership and mode bits are not modeled here (Windows uses ACLs); the link and
// file-kind checks and the directory handles still apply.
func checkOwner(string, os.FileInfo) error { return nil }

func checkStateDir(string, os.FileInfo) error { return nil }
