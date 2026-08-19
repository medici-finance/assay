package deskkit

// loopnames.go — the known-loop-name roster behind the per-loop stop flag `STOP.<name>`.
//
// THE DEFECT THIS CLOSES. `stopFlagState` used to build the per-loop flag path by bare
// string concatenation — `flagFileState(dir, "STOP."+os.Getenv("DESK_LOOP"))` — with no
// validation of the loop name whatsoever. Two failures followed, and BOTH were silent,
// which is the worst available mode for a safety stop:
//
//  1. A RENAME ORPHANS A HELD FLAG. A human holding `STOP.batch-fanout` gets no
//     protection the moment the loop starts calling itself `worker-desk`.
//     Nothing errors. The flag simply never matches and the loop
//     runs on, while `deskboard` still shows the flag sitting there looking armed.
//  2. A TYPO IS INDISTINGUISHABLE FROM "NO FLAG HELD". `DESK_LOOP=btach-fanout` reads a
//     path that can never exist and reports, cleanly, "not stopped".
//
// Both are the house's `could-not-check` state being reported as `checked-clean`. A
// checker that could not look must never come back green.
//
// THE FIX, IN TWO PARTS.
//
//   - An unrecognised `DESK_LOOP` is UNVERIFIABLE (exit 6), not clean. See
//     `resolveLoopIdentity` and its caller in killswitch.go.
//   - A loop's identity is an EQUIVALENCE CLASS of names, not one string. A stop flag
//     held on ANY name in the class arms the loop, so a rename cannot orphan a flag a
//     human is already holding — in either direction.
//
// WHY EXIT 6 (UNVERIFIABLE) AND NOT EXIT 5 (REFUSED). Exit 5 is reserved for a
// deliberate constraint refusal, and — by house rule — is never routed around. An
// unrecognised loop name is not a human deciding to stop anything; it is the tool being
// unable to establish whether a stop is held. That is exactly what exit 6 means here
// already: `Guard` returns Unverifiable when the DISABLED file is unreadable or HOME is
// missing. This is the same class of ignorance, so it reuses the same code rather than
// spending exit 5's meaning on a configuration error. Diluting exit 5 into "also
// misconfiguration" is how operators learn to route around exit 5 — which would be a far
// worse outcome than the bug being fixed here.
//
// BLAST RADIUS — DELIBERATELY SCOPED. The house has already been burned once by a
// fail-closed check with an unbounded blast radius: an unknown key in `roster.env`
// fail-closed the ENTIRE trust roster and took write-verbs down fleet-wide.
// Fail-closed on a safety stop is right; fail-closed in a way that
// bricks every loop at once is not. Three choices bound the radius here:
//
//   - The roster is COMPILED IN, never read from a runtime file. There is no
//     `loops.env` whose corruption could refuse every loop on the machine at once. The
//     cost is that adding a loop is a PR; that is the intended price.
//   - Only a session that SETS `DESK_LOOP` to an unrecognised value is affected. A
//     session with `DESK_LOOP` unset is untouched (plain tool invocations keep their
//     documented behaviour), and every loop with a recognised name is untouched. One
//     mis-spelled loop cannot stop its neighbours.
//   - The roster is GENEROUS about what it recognises and strict only about SURFACING
//     what it does not. Every extra known name costs nothing but one more `STOP.<name>`
//     path being honoured; a missing name costs a bricked loop. So retired names are
//     kept forever and successor names are registered BEFORE the rename lands.
//
// THE DECLARED SOURCE. Loop identities are declared in exactly one place in this repo:
// the `export DESK_LOOP=<name>` line in each desk skill's SKILL.md (step 0 of its boot
// sequence), under both skills roots. This file does not restate that roster as a second
// copy of record — `loopnames_test.go` DIFFS it against those declarations and against
// the `Loop-name registry` bullet in tools/desk/README.md, and goes red if a loop can
// present a name the kill switch would not recognise.

import "sort"

// canonicalLoopNames maps each canonical loop name to the retired names that still
// resolve to it. A canonical name is the name a loop presents in `DESK_LOOP` today; a
// retired name is one it USED to present, and which a human may still be holding a stop
// flag on.
//
// RETIRED NAMES ARE NEVER REMOVED. That is the whole alias decision (see
// LoopFlagNames): a rename must not be able to orphan a flag, so the old name keeps
// working rather than the rename being refused. Refusing renames is not a real option —
// this roster cannot gate another repo's or another PR's rename, and a rename that is
// merely blocked here would simply land with the roster stale, which is the silent
// failure again. Honouring both names is strictly safer: the worst case is that one
// extra flag path is checked.
var canonicalLoopNames = map[string][]string{
	// The four desks named in the tools/desk/README.md loop-name registry, plus the
	// intake desk, which declares a loop identity of its own.
	"the-desk":       nil,
	"pr-review-desk": nil,
	"verify-desk":    nil,

	// worker-desk — REGISTERED AHEAD OF THE RENAME. A pending rename moves
	// `.claude/skills/batch-fanout/` to `.claude/skills/worker-desk/`. Registering the
	// successor name before that lands is what makes this file compatible with the
	// rename instead of broken by it: on the day the skill starts exporting
	// `DESK_LOOP=worker-desk`, the name is already known AND a `STOP.batch-fanout` a
	// human is holding still halts it.
	"worker-desk": {"batch-fanout"},

	// intake-desk — renamed from issue-loop.
	"intake-desk": {"issue-loop"},
}

// preRegisteredLoopNames records canonical names that are deliberately in the roster
// BEFORE any skill declares them, with the reason. Without this, the drift test in
// loopnames_test.go could not tell a forward-registration apart from a stale entry
// nobody removed — and a roster that silently accumulates junk is a roster nobody
// trusts. An entry here is a promise that a declaration is coming; when it arrives, the
// entry should be deleted.
// `worker-desk` is deliberately NOT listed: the loop it names is already declared today,
// under its retired name (`export DESK_LOOP=batch-fanout`), and resolves through the alias
// above. That is what lets this roster be correct both before and after the rename lands.
var preRegisteredLoopNames = map[string]string{
	"intake-desk": "the issue-loop loop was renamed to intake-desk; the skill exists " +
		"but does not yet carry an `export DESK_LOOP=` line of its own",
}

// retiredLoopNames is the reverse index of canonicalLoopNames: retired name -> canonical.
// Built once at init so lookup is not a scan.
var retiredLoopNames = func() map[string]string {
	m := map[string]string{}
	for canonical, retired := range canonicalLoopNames {
		for _, r := range retired {
			m[r] = canonical
		}
	}
	return m
}()

// resolveLoopIdentity maps a raw DESK_LOOP value to its canonical loop name.
//
// known=false means the name matches no canonical name and no retired name. The caller
// MUST treat that as could-not-check (Unverifiable, exit 6) and must NOT treat it as
// "no stop flag held" — that conflation is the bug this file exists to close.
func resolveLoopIdentity(raw string) (canonical string, known bool) {
	if _, ok := canonicalLoopNames[raw]; ok {
		return raw, true
	}
	if c, ok := retiredLoopNames[raw]; ok {
		return c, true
	}
	return "", false
}

// LoopFlagNames returns every flag-name suffix that halts the loop presenting `raw`,
// canonical name first, then its retired names in a stable order. known=false when the
// name is unrecognised, in which case the returned slice is nil.
//
// The full class is returned — not just the name the loop presented — so the rename is
// covered in BOTH directions:
//
//	a loop presenting `worker-desk`  is halted by STOP.worker-desk AND STOP.batch-fanout
//	a loop presenting `batch-fanout` is halted by STOP.batch-fanout AND STOP.worker-desk
//
// The first case is the one that matters in practice: it is a human who armed the flag
// BEFORE the rename and would otherwise have silently lost their stop. The second keeps
// a human who armed the NEW name from silently losing it against a not-yet-updated
// session.
func LoopFlagNames(raw string) (names []string, known bool) {
	canonical, ok := resolveLoopIdentity(raw)
	if !ok {
		return nil, false
	}
	retired := append([]string(nil), canonicalLoopNames[canonical]...)
	sort.Strings(retired)
	return append([]string{canonical}, retired...), true
}

// KnownLoopNames returns every recognised loop name — canonical and retired — sorted.
// It backs the refusal message (an operator who mistyped needs to see the real set) and
// deskboard's marking of stop flags that halt nothing.
func KnownLoopNames() []string {
	out := make([]string, 0, len(canonicalLoopNames)+len(retiredLoopNames))
	for c := range canonicalLoopNames {
		out = append(out, c)
	}
	for r := range retiredLoopNames {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// IsKnownLoopName reports whether raw names a loop this roster recognises.
func IsKnownLoopName(raw string) bool {
	_, ok := resolveLoopIdentity(raw)
	return ok
}
