package viahelper

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cachequiet"
)

func setup() { cachequiet.Isolate() }

func TestMain(m *testing.M) { setup(); os.Exit(m.Run()) }
