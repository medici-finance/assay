package deskkit

// loopnames_test.go — proof for the per-loop stop-flag roster (loopnames.go).
//
// Every check here is written so that it CAN fail, and each has been observed failing
// against a mutated source before being trusted. Two shapes are deliberately avoided:
// a scan whose "no violations" result is indistinguishable from "scanned nothing" (every
// scan asserts a non-zero read count), and a loop over an empty list (every corpus test
// asserts a floor on what it found).

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The scenario this whole change exists for.
// ---------------------------------------------------------------------------

// TestHeldFlagSurvivesTheBatchFanoutRename is the POSITIVE CONTROL for the imminent
// rename: `.claude/skills/batch-fanout/` becomes
// `.claude/skills/worker-desk/`, so the loop starts presenting DESK_LOOP=worker-desk
// while a human may still be holding STOP.batch-fanout.
//
// Before loopnames.go, this test FAILS: stopFlagState built "STOP."+"worker-desk" by bare
// concatenation, found no such file, and returned armed=false — the human's stop flag was
// silently orphaned and the loop ran on.
func TestHeldFlagSurvivesTheBatchFanoutRename(t *testing.T) {
	dir := setup(t)
	// The human armed the flag under the OLD name, before the rename.
	mkFlag(t, dir, "STOP.batch-fanout", "human halted the fanout loop")
	// The loop now identifies itself by the NEW name.
	t.Setenv(loopEnv, "worker-desk")

	err := Guard()
	t.Logf("guard refused: %v", err)
	if !IsDisabled(err) {
		t.Fatalf("Guard() = %v (exit %d); want Disabled (exit %d) — a held STOP.batch-fanout "+
			"MUST still halt the loop after it renames itself worker-desk, or the rename "+
			"silently orphans a human's safety stop",
			err, ExitCodeOf(err), ExitDisabled)
	}
	if !strings.Contains(err.Error(), "STOP.batch-fanout") {
		t.Fatalf("refusal must name the exact flag file the human is holding, got %q", err.Error())
	}
}

// TestHeldFlagUnderNewNameHaltsOldNamedSession is the same guarantee in the other
// direction: a human who arms the NEW name must not lose their stop against a session
// that has not yet been updated.
func TestHeldFlagUnderNewNameHaltsOldNamedSession(t *testing.T) {
	dir := setup(t)
	mkFlag(t, dir, "STOP.worker-desk", "human halted the worker desk")
	t.Setenv(loopEnv, "batch-fanout")

	err := Guard()
	if !IsDisabled(err) {
		t.Fatalf("Guard() = %v (exit %d); want Disabled — STOP.worker-desk must halt a session "+
			"still presenting the pre-rename name", err, ExitCodeOf(err))
	}
	if !strings.Contains(err.Error(), "STOP.worker-desk") {
		t.Fatalf("refusal must name the exact flag file, got %q", err.Error())
	}
}

// TestIssueLoopRenameAliasHolds covers the other completed rename.
func TestIssueLoopRenameAliasHolds(t *testing.T) {
	dir := setup(t)
	mkFlag(t, dir, "STOP.issue-loop", "human halted intake")
	t.Setenv(loopEnv, "intake-desk")

	if err := Guard(); !IsDisabled(err) {
		t.Fatalf("Guard() = %v; want Disabled — STOP.issue-loop must still halt intake-desk", err)
	}
}

// ---------------------------------------------------------------------------
// could-not-check is not checked-clean.
// ---------------------------------------------------------------------------

// TestUnknownLoopNameIsUnverifiableNotClean is the typo half of the defect. With no flag
// files present at all, an unrecognised DESK_LOOP must NOT come back "not stopped": the
// tool cannot know which file would speak for that loop, so it has not checked anything.
func TestUnknownLoopNameIsUnverifiableNotClean(t *testing.T) {
	setup(t)
	t.Setenv(loopEnv, "btach-fanout") // transposed, as a human would mistype it

	err := Guard()
	t.Logf("guard refused: %v", err)
	if err == nil {
		t.Fatal("Guard() = nil for an unrecognised DESK_LOOP — reporting 'no stop flag held' " +
			"for a loop whose flag file cannot even be named is could-not-check reported as clean")
	}
	if !IsUnverifiable(err) {
		t.Fatalf("Guard() = %v (exit %d); want Unverifiable (exit %d)",
			err, ExitCodeOf(err), ExitUnverifiable)
	}
	// The operator must be able to act on it: the bad value and the real set, both named.
	if !strings.Contains(err.Error(), "btach-fanout") {
		t.Fatalf("refusal must quote the offending value, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "worker-desk") {
		t.Fatalf("refusal must list the known loop names so the operator can correct it, got %q", err.Error())
	}
}

// TestUnknownLoopNameBlastRadiusIsOneSession bounds the fail-closed behaviour. The house
// has been burned by a fail-closed check with an unbounded radius (an unknown key in
// roster.env fail-closed the whole trust roster, taking write-verbs down fleet-wide).
// These are the two escape valves that keep that from recurring here.
func TestUnknownLoopNameBlastRadiusIsOneSession(t *testing.T) {
	t.Run("DESK_LOOP unset is untouched", func(t *testing.T) {
		setup(t)
		t.Setenv(loopEnv, "")
		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v; want nil — a plain tool invocation with no loop identity "+
				"must keep its documented behaviour and cannot be collateral of a roster miss", err)
		}
	})

	t.Run("a registered neighbour is untouched", func(t *testing.T) {
		dir := setup(t)
		// A flag exists for some OTHER, mis-spelled loop. A correctly-named loop must
		// still run: one bad name cannot stop its neighbours.
		mkFlag(t, dir, "STOP.btach-fanout", "typo'd flag")
		t.Setenv(loopEnv, "verify-desk")
		if err := Guard(); err != nil {
			t.Fatalf("Guard() = %v; want nil — a registered loop must not be stopped by "+
				"another loop's unrecognised flag", err)
		}
	})
}

// TestPrecedenceSurvivesAnUnknownLoopName proves the documented chain
// DISABLED > STOP > STOP.<name> is preserved, including the case that matters most: a
// mis-spelled DESK_LOOP must never MASK a broader halt by turning it into an exit-6.
func TestPrecedenceSurvivesAnUnknownLoopName(t *testing.T) {
	t.Run("DISABLED wins over an unknown loop name", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "DISABLED", "whole suite halted")
		t.Setenv(loopEnv, "btach-fanout")
		err := Guard()
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v (exit %d); want Disabled (exit 3) — the kill switch outranks "+
				"a loop-name problem", err, ExitCodeOf(err))
		}
		if !strings.Contains(err.Error(), "whole suite halted") {
			t.Fatalf("DISABLED reason must surface, got %q", err.Error())
		}
	})

	t.Run("STOP wins over an unknown loop name", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "all loops halted")
		t.Setenv(loopEnv, "btach-fanout")
		err := Guard()
		if !IsDisabled(err) {
			t.Fatalf("Guard() = %v (exit %d); want Disabled (exit 3) — an all-loops STOP is "+
				"resolved before the loop name is consulted", err, ExitCodeOf(err))
		}
		if !strings.Contains(err.Error(), "all loops halted") {
			t.Fatalf("STOP reason must surface, got %q", err.Error())
		}
	})

	t.Run("STOP wins over a per-loop flag", func(t *testing.T) {
		dir := setup(t)
		mkFlag(t, dir, "STOP", "all loops halted")
		mkFlag(t, dir, "STOP.worker-desk", "just the worker")
		t.Setenv(loopEnv, "worker-desk")
		err := Guard()
		if !IsDisabled(err) || !strings.Contains(err.Error(), "all loops halted") {
			t.Fatalf("Guard() = %v; want the all-loops STOP to take precedence", err)
		}
	})
}

// TestInertStopFlagIsMarkedOnTheBoard covers the human-visible half of the same silent
// failure: a flag whose name matches no loop halts NOTHING, but deskboard used to list it
// beside flags that really are armed, so a human could believe they held a stop they did
// not.
func TestInertStopFlagIsMarkedOnTheBoard(t *testing.T) {
	dir := setup(t)
	mkFlag(t, dir, "STOP.btach-fanout", "meant to stop the fanout")
	mkFlag(t, dir, "STOP.verify-desk", "really stopping verify")

	flags := ActiveStopFlags()
	if len(flags) != 2 {
		t.Fatalf("ActiveStopFlags() returned %d flags, want 2 — the enumeration is broken, "+
			"which is never a pass: %+v", len(flags), flags)
	}
	byName := map[string]ActiveStopFlag{}
	for _, f := range flags {
		byName[f.Name] = f
	}
	inert, ok := byName["STOP.btach-fanout"]
	if !ok {
		t.Fatalf("STOP.btach-fanout missing from %+v", flags)
	}
	if !inert.Inert {
		t.Error("a STOP flag naming no known loop must be marked Inert — unmarked, it reads " +
			"to a human as a stop that is being honoured when nothing will ever read it")
	}
	if !strings.Contains(inert.Reason, "halts nothing") {
		t.Errorf("the inert flag's reason must say so plainly, got %q", inert.Reason)
	}
	live, ok := byName["STOP.verify-desk"]
	if !ok {
		t.Fatalf("STOP.verify-desk missing from %+v", flags)
	}
	if live.Inert {
		t.Error("a STOP flag for a registered loop must NOT be marked Inert (false positive)")
	}
}

// TestLoopFlagNamesClass pins the resolution table itself.
func TestLoopFlagNamesClass(t *testing.T) {
	cases := []struct {
		raw   string
		want  []string
		known bool
	}{
		{"worker-desk", []string{"worker-desk", "batch-fanout"}, true},
		{"batch-fanout", []string{"worker-desk", "batch-fanout"}, true},
		{"intake-desk", []string{"intake-desk", "issue-loop"}, true},
		{"issue-loop", []string{"intake-desk", "issue-loop"}, true},
		{"verify-desk", []string{"verify-desk"}, true},
		{"the-desk", []string{"the-desk"}, true},
		{"pr-review-desk", []string{"pr-review-desk"}, true},
		{"btach-fanout", nil, false},
		{"", nil, false},
		{"STOP", nil, false},
	}
	for _, c := range cases {
		got, known := LoopFlagNames(c.raw)
		if known != c.known {
			t.Errorf("LoopFlagNames(%q): known = %v, want %v", c.raw, known, c.known)
			continue
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("LoopFlagNames(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// derive-or-diff: the roster is checked against the ONE declared source.
// ---------------------------------------------------------------------------

// loopSkillRoots are the two homes a desk skill has in this repo, relative to this
// package. Both are covered by the `.claude/**` and `plugins/**` path filters of
// .github/workflows/tools.yml, so a diff to either runs this test.
var loopSkillRoots = []string{
	"../../../../.claude/skills",
	"../../../../plugins/assay/skills",
}

// deskLoopDeclRe matches the loop-identity declaration a desk skill carries at step 0 of
// its boot sequence: `export DESK_LOOP=<name>`. This is THE declared source for loop
// names — it is what a loop actually presents at runtime. It deliberately does not match
// `$DESK_LOOP` reads or `DESK_LOOP=<canonical-name>` placeholders.
var deskLoopDeclRe = regexp.MustCompile(`DESK_LOOP=([a-z][a-z0-9-]*)`)

// scanDeclaredLoopNames walks one skills root and returns every loop identity declared in
// a SKILL.md, plus the number of SKILL.md files actually read. A zero read count is a
// broken scan, never a pass.
func scanDeclaredLoopNames(root string) (decls map[string][]string, scanned int, err error) {
	decls = map[string][]string{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(root, e.Name(), "SKILL.md")
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // not every skill directory carries a SKILL.md
		}
		scanned++
		for _, m := range deskLoopDeclRe.FindAllStringSubmatch(string(raw), -1) {
			decls[m[1]] = append(decls[m[1]], path)
		}
	}
	return decls, scanned, nil
}

// TestEveryDeclaredLoopIdentityIsKnown is the load-bearing diff: if a skill can present a
// DESK_LOOP value the roster does not recognise, that loop runs with no per-loop stop
// protection. This test goes red on exactly that, which is what makes the roster a
// derived check rather than a second, drifting copy of the loop list.
func TestEveryDeclaredLoopIdentityIsKnown(t *testing.T) {
	skipIfFixtureAbsent(t, loopSkillRoots[0],
		".claude/ is not part of this repository's published file set; no loop-declaring skills ship")
	totalScanned, totalDecls := 0, 0

	for _, root := range loopSkillRoots {
		decls, scanned, err := scanDeclaredLoopNames(root)
		if err != nil {
			t.Fatalf("cannot read skills root %s: %v", root, err)
		}
		if scanned == 0 {
			t.Fatalf("read no SKILL.md under %s — the scan is broken, which is never a pass", root)
		}
		totalScanned += scanned
		for name, files := range decls {
			totalDecls += len(files)
			if !IsKnownLoopName(name) {
				t.Errorf("%s declares DESK_LOOP=%s, which the kill-switch roster does not know "+
					"(loopnames.go). That loop would run with NO per-loop stop protection and "+
					"Guard would refuse it as unverifiable. Declared in: %s",
					root, name, strings.Join(files, ", "))
			}
		}
	}

	// Corpus-identity guards. Without these, a moved or mis-rooted tree yields "no
	// unknown names" — a clean result that proves nothing.
	if totalScanned < len(loopSkillRoots) {
		t.Fatalf("scanned only %d SKILL.md across %d roots", totalScanned, len(loopSkillRoots))
	}
	if totalDecls < 4 {
		t.Fatalf("found only %d `DESK_LOOP=` declarations across the skills corpus; the desk "+
			"skills declare at least 4, so this scan did not read the real tree", totalDecls)
	}
}

// TestReadmeLoopRegistryIsKnown diffs the roster against the human-facing registry in
// tools/desk/README.md, so the documented canonical names and the enforced ones cannot
// drift apart in either direction without a red test.
func TestReadmeLoopRegistryIsKnown(t *testing.T) {
	const readme = "../../README.md"
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("cannot read %s: %v", readme, err)
	}
	lines := strings.Split(string(raw), "\n")

	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "- Loop-name registry") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("no `- Loop-name registry` bullet in %s — the registry this roster is diffed "+
			"against has moved or been deleted; a passing result here would be vacuous", readme)
	}

	// The bullet runs until the first blank line.
	var block []string
	for i := start; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
		block = append(block, lines[i])
	}

	// Backticked tokens that are plausibly loop names: lowercase, digits, hyphens only.
	// This filter excludes `DESK_LOOP`, `STOP.<name>`, `/loop` and `DESK_LOOP=<canonical-name>`.
	tokenRe := regexp.MustCompile("`([a-z][a-z0-9-]*)`")
	var names []string
	for _, m := range tokenRe.FindAllStringSubmatch(strings.Join(block, "\n"), -1) {
		names = append(names, m[1])
	}
	if len(names) < 4 {
		t.Fatalf("parsed only %d loop names from the registry bullet (%q) — the bullet's format "+
			"changed and this check is no longer reading it", len(names), strings.Join(block, " "))
	}
	for _, n := range names {
		if !IsKnownLoopName(n) {
			t.Errorf("%s documents %q as a canonical loop name but the kill-switch roster does "+
				"not know it (loopnames.go) — a human following the README would arm a "+
				"STOP.%s that halts nothing", readme, n, n)
		}
	}
}

// TestRosterCarriesNoUndeclaredCanonicalName is the reverse-drift guard. Extra known
// names are SAFE (each only means one more STOP.<name> path is honoured), so this is not
// an equality check — but a canonical name that no skill declares must be a deliberate,
// reasoned forward-registration, not a stale entry nobody removed.
func TestRosterCarriesNoUndeclaredCanonicalName(t *testing.T) {
	skipIfFixtureAbsent(t, loopSkillRoots[0],
		".claude/ is not part of this repository's published file set; no loop-declaring skills ship")
	declared := map[string]bool{}
	scannedAny := 0
	for _, root := range loopSkillRoots {
		decls, scanned, err := scanDeclaredLoopNames(root)
		if err != nil {
			t.Fatalf("cannot read skills root %s: %v", root, err)
		}
		scannedAny += scanned
		for name := range decls {
			// A skill may still declare a retired name; credit its canonical.
			if c, ok := resolveLoopIdentity(name); ok {
				declared[c] = true
			}
		}
	}
	if scannedAny == 0 {
		t.Fatal("read no SKILL.md at all — the scan is broken, which is never a pass")
	}
	if len(declared) == 0 {
		t.Fatal("no loop identities resolved from the skills corpus — vacuous pass guarded")
	}

	for canonical := range canonicalLoopNames {
		if declared[canonical] {
			continue
		}
		reason, ok := preRegisteredLoopNames[canonical]
		if !ok {
			t.Errorf("canonical loop name %q is in the roster but NO skill declares it and it "+
				"is not listed in preRegisteredLoopNames — either a skill's `export DESK_LOOP=` "+
				"line was removed, or this is a stale entry", canonical)
			continue
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("preRegisteredLoopNames[%q] has an empty reason; a forward-registration "+
				"must say what it is waiting for", canonical)
		}
	}
}

// TestPreRegistrationsAreRetiredWhenDeclared keeps preRegisteredLoopNames from becoming
// permanent: once a skill actually declares the name, the exemption must be deleted.
func TestPreRegistrationsAreRetiredWhenDeclared(t *testing.T) {
	skipIfFixtureAbsent(t, loopSkillRoots[0],
		".claude/ is not part of this repository's published file set; no loop-declaring skills ship")
	declared := map[string]bool{}
	for _, root := range loopSkillRoots {
		decls, _, err := scanDeclaredLoopNames(root)
		if err != nil {
			t.Fatalf("cannot read skills root %s: %v", root, err)
		}
		for name := range decls {
			if c, ok := resolveLoopIdentity(name); ok {
				declared[c] = true
			}
		}
	}
	for name := range preRegisteredLoopNames {
		if declared[name] {
			t.Errorf("%q is now declared by a skill, so its preRegisteredLoopNames entry is "+
				"spent and must be deleted (the roster entry itself stays)", name)
		}
	}
}
