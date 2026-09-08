package askassay_test

// This is an EXTERNAL test package (package askassay_test, not askassay). It may
// reference only EXPORTED identifiers, so it compiles exactly the surface a
// consumer in a DIFFERENT Go module can reach. Its job is to fail the public
// repo's OWN `go test`/`go vet` — at build time, before any downstream sees it —
// if a consumer-facing identifier is renamed, unexported, or has its signature
// changed. A green downstream build is not the first thing to notice such a
// break; this file is.
//
// Every reference below is a compile-time binding to a typed target. If the
// target loses its exported name or changes shape, this file stops compiling.

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

// The answer type and its single rendering entry point. Render is the one path a
// consumer calls to turn an answer into a line; it must stay exported with this
// signature, so it is bound here as a method expression rather than merely named.
var (
	_ askassay.Answer              // the answer value type
	_ func(askassay.Answer) string = askassay.Answer.Render
)

// The registry entry points a consumer uses to discover and resolve questions.
var (
	_ func(string) (askassay.Question, bool) = askassay.Lookup
	_ func() []askassay.Question             = askassay.Questions
)

// The answer constructors a consumer builds results with.
var (
	_ func(askassay.Question, int64, askassay.Stamp) askassay.Answer         = askassay.Computed
	_ func(askassay.Question, int64, string, askassay.Stamp) askassay.Answer = askassay.Failed
	_ func(askassay.Question, string, askassay.Stamp) askassay.Answer        = askassay.Unavailable
	_ func(string, string, askassay.Stamp) askassay.Answer                   = askassay.Unanswerable
)

// The stamp value type and its clock-only constructor.
var (
	_ askassay.Stamp                         = askassay.Stamp{}
	_ func(time.Time, string) askassay.Stamp = askassay.ClockOnly
)

// The supporting value types a consumer names in signatures.
var (
	_ askassay.Source
	_ askassay.Question
	_ askassay.Class
	_ askassay.State
)

// The state constants a consumer switches on, and the sentinel figure token.
var (
	_ askassay.State = askassay.Checked
	_ askassay.State = askassay.CheckedFailed
	_ askassay.State = askassay.CouldNotCheck
	_ string         = askassay.FigureField
)

// TestConsumerFacingSurfaceRenders exercises the two identifiers a consumer most
// depends on together — a constructor and the render entry point — so the guard
// is a real test with a behavioural assertion, not only a bank of compile-time
// bindings. The un-export the negative control injects (renaming Render to lower
// case) breaks the binding above at COMPILE time; this body then never runs,
// which is exactly the "fails to build" signal the guard exists to produce.
func TestConsumerFacingSurfaceRenders(t *testing.T) {
	q, ok := askassay.Lookup(askassay.Questions()[0].ID)
	if !ok {
		t.Fatal("Lookup could not resolve a registered question by its own ID")
	}
	a := askassay.Unanswerable(q.ID, "export-guard probe", askassay.ClockOnly(time.Now(), "guard test: no measurement"))
	if got := a.Render(); got == "" {
		t.Fatal("Answer.Render returned an empty line for a well-formed answer")
	}
}
