package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// stdout/stderr are package vars so tests can capture them.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// expandHome expands a leading "~/" to the user's home directory. No-op otherwise.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
