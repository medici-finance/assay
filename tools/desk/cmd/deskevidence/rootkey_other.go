//go:build !unix

package main

import "strings"

// rootIdentity names a landing root by its case-folded resolved path: Windows paths are
// case-insensitive, so a re-cased spelling of one directory maps to one preparation lock.
func rootIdentity(abs string) string {
	return "path:" + strings.ToLower(abs)
}
