package main

import (
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// perfVerbs is the verb roster main hands deskkit.PerfRun, so a perf record carries the verb's
// real name (desk, cache, show, model-policy ...) instead of "other". It is DERIVED from the
// command tree rather than listed by hand, so a verb added to buildRoot is recorded under its
// own name with no second edit to forget. Off (no ASSAY_TELEMETRY=1) it returns nil without
// building anything: PerfRun then never reads it.
//
// The two flag spellings are the introspection forms the tree answers without a verb.
func perfVerbs() []string {
	if !deskkit.PerfEnabled() {
		return nil
	}
	verbs := []string{"--version", "--help", "-h"}
	for _, c := range buildRoot().Commands() {
		verbs = append(verbs, c.Name())
	}
	return verbs
}

// launchStep times f as one pre-handover phase of a `cellctl desk` launch and writes it as a
// "step:<name>" perf row (a no-op unless ASSAY_TELEMETRY=1; deskkit.PerfStep owns that gate).
// The row is written BEFORE the launch hands over to the session, which is the whole point:
// the verb-level record only lands when the session ends, so the cost of getting there would
// otherwise be invisible. A die()/exitWith panic is recorded with its exit code and re-raised
// unchanged, so timing never alters what the launch does.
func launchStep(step string, f func() error) (err error) {
	began := time.Now()
	defer func() {
		code := deskkit.ExitOK
		r := recover()
		switch {
		case r != nil:
			if ec, ok := r.(exitCode); ok {
				code = ec.code
			} else {
				code = deskkit.ExitUnverifiable
			}
		case err != nil:
			code = deskkit.ExitCodeOf(err)
		}
		deskkit.PerfStep("cellctl", step, time.Since(began), code)
		if r != nil {
			panic(r)
		}
	}()
	return f()
}

// launchStepV is launchStep for a phase that reports no error (it dies instead).
func launchStepV(step string, f func()) {
	_ = launchStep(step, func() error { f(); return nil })
}
