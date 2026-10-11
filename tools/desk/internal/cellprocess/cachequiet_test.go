package cellprocess

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cachequiet"
)

// TestMain clears the cell's storage-admission variables before any test runs, so a test
// that launches through withCache with an inherited environment never takes a lease on
// the operator's live Go-cache root (see package cachequiet; #2491). Tests that exercise
// the managed cache build their own policy under t.TempDir().
func TestMain(m *testing.M) { cachequiet.Isolate(); os.Exit(m.Run()) }
