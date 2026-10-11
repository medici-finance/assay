package covered

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cachequiet"
)

func TestMain(m *testing.M) { cachequiet.Isolate(); os.Exit(m.Run()) }
