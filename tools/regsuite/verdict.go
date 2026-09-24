package main

import (
	"fmt"
	"sort"
	"strings"
)

// result is what `gate` prints and exits with.
type result struct {
	ExitCode int // 0 clean, 1 fail, 2 could-not-check
	Report   string
}

// computeVerdict is the pure rule this whole tool exists to apply: a
// function over per-module listed/executed facts, with no subprocess, no
// filesystem access and no `go` toolchain involved, so it is unit-testable
// on its own (the "layering" note in the brief). It never returns exit 0
// when any module carries a BuildError ("Never exit 0" — Task step 2).
//
// Priority when more than one condition applies: could-not-check outranks
// everything else, because a module this gate could not build or list makes
// every other count on this run untrustworthy. Below that, fail / count-drop
// / vacuous are independent reasons and all three are reported together when
// they co-occur; the exit code is 1 either way.
func computeVerdict(mods []moduleResult) result {
	var b strings.Builder

	var buildErrors []string
	for _, m := range mods {
		if m.BuildError != "" {
			buildErrors = append(buildErrors, fmt.Sprintf("%s: %s", m.Path, m.BuildError))
		}
	}
	if len(buildErrors) > 0 {
		fmt.Fprintln(&b, "could-not-check")
		for _, e := range buildErrors {
			fmt.Fprintf(&b, "  %s\n", e)
		}
		return result{ExitCode: 2, Report: b.String()}
	}

	// Shape NOTICEs — informational only (Task step 2's last bullet), never
	// change the exit code.
	notices := shapeNotices(mods)

	var failing []string
	for _, m := range mods {
		for _, e := range m.Executed {
			if e.Outcome == "fail" {
				failing = append(failing, m.Path+"."+e.Name)
			}
		}
	}
	sort.Strings(failing)

	totalHead, totalBase := 0, 0
	var missing []string
	for _, m := range mods {
		totalHead += len(m.HeadListed)
		totalBase += len(m.BaseListed)
		headSet := toSet(m.HeadListed)
		for _, n := range m.BaseListed {
			if !headSet[n] {
				missing = append(missing, m.Path+"."+n)
			}
		}
	}
	sort.Strings(missing)
	countDrop := totalHead < totalBase

	vacuous := vacuousModules(mods)

	fmt.Fprintln(&b, "regsuite gate report")
	fmt.Fprintf(&b, "modules examined: %d\n", len(mods))
	writeModuleTable(&b, mods)
	writeExecutedList(&b, mods)

	var reasons []string
	if len(failing) > 0 {
		reasons = append(reasons, "fail")
		fmt.Fprintln(&b, "FAILING regression tests:")
		for _, n := range failing {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}
	if countDrop {
		reasons = append(reasons, "count-drop")
		fmt.Fprintf(&b, "COUNT DROP: head listed %d < base listed %d\n", totalHead, totalBase)
		fmt.Fprintln(&b, "missing from head (present in base, absent in head):")
		for _, n := range missing {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}
	if len(vacuous) > 0 {
		reasons = append(reasons, "vacuous")
		fmt.Fprintln(&b, "VACUOUS: listed tests exist but were not all executed:")
		for _, m := range vacuous {
			fmt.Fprintf(&b, "  %s\n", m)
		}
	}
	for _, n := range notices {
		fmt.Fprintf(&b, "NOTICE: %s\n", n)
	}

	if len(reasons) == 0 {
		fmt.Fprintln(&b, "clean")
		return result{ExitCode: 0, Report: b.String()}
	}
	fmt.Fprintf(&b, "verdict: fail (%s)\n", strings.Join(reasons, ", "))
	return result{ExitCode: 1, Report: b.String()}
}

// vacuousModules returns the modules where the selector executed fewer
// top-level regression tests than were listed, in either of the two
// independent ways Task step 2 names:
//  1. the module's head or base listed count is > 0, but zero tests were
//     executed (pass+fail = 0) — the plain "-run matched nothing" case;
//  2. the executed set (pass+fail+skip) is a strict, non-empty-listed subset
//     of the head listed set — a selector that matched SOME but not all of
//     the module's regression tests.
//
// A module with zero listed tests on both sides is reported as measured-zero
// (never vacuous) — Task step 2's clean-verdict bullet.
func vacuousModules(mods []moduleResult) []string {
	var vacuous []string
	for _, m := range mods {
		if len(m.HeadListed) == 0 && len(m.BaseListed) == 0 {
			continue
		}
		passFail := 0
		executedNames := map[string]bool{}
		for _, e := range m.Executed {
			if e.Outcome == "pass" || e.Outcome == "fail" {
				passFail++
			}
			executedNames[e.Name] = true
		}
		if passFail == 0 {
			vacuous = append(vacuous, m.Path)
			continue
		}
		headSet := toSet(m.HeadListed)
		subset := true
		for n := range executedNames {
			if !headSet[n] {
				subset = false
				break
			}
		}
		if subset && len(executedNames) < len(headSet) {
			vacuous = append(vacuous, m.Path)
		}
	}
	sort.Strings(vacuous)
	return vacuous
}

// shapeNotices flags every distinct (module, name) pair whose name carries
// the prefix's -list/-run match but not the full TestRegression_<repo>_
// <issue>[_Desc] shape. It never changes the exit code — Task step 2's last
// bullet.
func shapeNotices(mods []moduleResult) []string {
	seen := map[string]bool{}
	var notices []string
	note := func(mod, name string) {
		key := mod + "\x00" + name
		if seen[key] {
			return
		}
		seen[key] = true
		if !regressionNameShape.MatchString(name) {
			notices = append(notices, fmt.Sprintf("%s: %s does not match the naming shape %s", mod, name, regressionNameShapePattern))
		}
	}
	for _, m := range mods {
		for _, n := range m.HeadListed {
			note(m.Path, n)
		}
		for _, n := range m.BaseListed {
			note(m.Path, n)
		}
	}
	sort.Strings(notices)
	return notices
}

func writeModuleTable(b *strings.Builder, mods []moduleResult) {
	sorted := make([]moduleResult, len(mods))
	copy(sorted, mods)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, m := range sorted {
		passFail, skipped := 0, 0
		for _, e := range m.Executed {
			switch e.Outcome {
			case "pass", "fail":
				passFail++
			case "skip":
				skipped++
			}
		}
		fmt.Fprintf(b, "  %-40s listed(head=%d base=%d) executed=%d skipped=%d\n",
			m.Path, len(m.HeadListed), len(m.BaseListed), passFail, skipped)
	}
}

// writeExecutedList names every top-level regression test this run actually
// executed on HEAD, with its outcome — the row-8 obligation: proof a seeded
// test was found and RUN, not merely listed, and the same detail the module
// table's bare counts cannot carry on their own.
func writeExecutedList(b *strings.Builder, mods []moduleResult) {
	sorted := make([]moduleResult, len(mods))
	copy(sorted, mods)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	any := false
	for _, m := range sorted {
		if len(m.Executed) > 0 {
			any = true
			break
		}
	}
	if !any {
		return
	}

	fmt.Fprintln(b, "EXECUTED regression tests:")
	for _, m := range sorted {
		execs := make([]execution, len(m.Executed))
		copy(execs, m.Executed)
		sort.Slice(execs, func(i, j int) bool { return execs[i].Name < execs[j].Name })
		for _, e := range execs {
			fmt.Fprintf(b, "  %s.%s: %s\n", m.Path, e.Name, e.Outcome)
		}
	}
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}
