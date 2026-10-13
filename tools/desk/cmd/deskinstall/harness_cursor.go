package main

// Cursor placement is shared with upgrade-assay. Binary acquisition and its
// mandatory Sigstore verification remain in this command's independent mode.
import "github.com/medici-finance/assay/tools/desk/internal/cursorbundle"

type HarnessOptions = cursorbundle.Options
type DriftFinding = cursorbundle.DriftFinding

var HarnessCursorPlace = cursorbundle.Place
var HarnessCursorCheck = cursorbundle.Check

const (
	cursorPackagingMarker    = "<!-- assay:cursor-packaging"
	bindingsBegin            = "<!-- assay:bindings:begin -->"
	driftContentDiffers      = cursorbundle.DriftContentDiffers
	exitHarnessClean         = 0
	exitHarnessDrift         = 1
	exitHarnessCouldNotCheck = 2
)
