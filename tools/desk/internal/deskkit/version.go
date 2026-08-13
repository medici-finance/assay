package deskkit

import (
	"fmt"
	"io"
)

// Version stamping. SourceSHA and BuiltAt are injected at build time
// by the Makefile's desk-install / desk-build targets via:
//
//	-ldflags "-X github.com/medici-finance/assay/tools/desk/internal/deskkit.SourceSHA=<short-sha> \
//	          -X github.com/medici-finance/assay/tools/desk/internal/deskkit.BuiltAt=<rfc3339>"
//
// A `go run` / unstamped build leaves them empty, which Version() reports as
// "unpinned" and WarnIfUnpinned announces on stderr — so running from source (drift
// risk) is visible in every transcript.
var (
	SourceSHA = ""
	BuiltAt   = ""
)

const unpinned = "unpinned"

// Version returns (sourceSHA, builtAt), substituting "unpinned" for either value
// that was not stamped in at build time. Both are echoed in every audit record and
// in each tool's --version output.
func Version() (sourceSHA, builtAt string) {
	s, b := SourceSHA, BuiltAt
	if s == "" {
		s = unpinned
	}
	if b == "" {
		b = unpinned
	}
	return s, b
}

// IsPinned reports whether the binary was stamped (installed via `sudo make
// desk-install`) rather than run from source.
func IsPinned() bool { return SourceSHA != "" && BuiltAt != "" }

// WarnIfUnpinned writes a one-line WARNING to w when the binary is unpinned, so an
// operator sees drift risk in the transcript. It is a no-op for a stamped binary.
func WarnIfUnpinned(w io.Writer) {
	if IsPinned() {
		return
	}
	fmt.Fprintln(w, "desk-tools WARNING: running UNPINNED (go run / unstamped build) — "+
		"sourceSHA/builtAt not embedded, drift is invisible. Install via `sudo make desk-install`.")
}
