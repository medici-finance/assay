//go:build !unix

package container

import (
	"os"
	"testing"
)

func makeFIFO(t *testing.T, _ string) {
	t.Helper()
	t.Skip("named pipes need a unix host")
}

func serveFIFO(*testing.T, string, string, <-chan struct{}) bool { return false }

func awaitFIFOReader(*testing.T, string, <-chan struct{}) *os.File { return nil }
