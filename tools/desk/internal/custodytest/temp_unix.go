//go:build !windows

// Package custodytest provides owner-only fixture directories for _test.go files only.
// Its permissive-elsewhere temporary directory is never a production custody decision.
package custodytest

import "testing"

func PrivateTempDir(t *testing.T) string { t.Helper(); return t.TempDir() }
