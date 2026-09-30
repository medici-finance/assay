// Command skillslint runs four offline checks over the plugin tree:
//
//	structural       every plugins/assay/skills/*/SKILL.md (lint.go), including
//	                 the per-skill frontmatter conformance limits — description
//	                 length, name length/pattern — plus their advisory body/
//	                 bundle budget NOTICEs (conformance.go)
//	hidden chars     byte-level invisible-character / Trojan-Source lint over the
//	                 instruction surfaces, plus an advisory context-budget NOTICE
//	                 (hidden.go)
//	house values     EVERY *.md under plugins/, at any depth (housevalue.go)
//	guardrails       derive-or-diff of every shared-guardrail copy (guardrail.go)
//
// The house-value half is deliberately wider than the other two: the references
// and READMEs under plugins/ are as adopter-facing as a skill body, and a
// resolved house value used to pass lint by sitting in one (#236, #238).
// See README.md and each file's header for what it checks and why.
//
//	go run ./tools/skillslint                 # lint the plugin tree under the cwd
//	go run ./tools/skillslint --root ..       # lint a sibling checkout
//	go run ./tools/skillslint --sync          # REGENERATE every guardrail copy
//	go run ./tools/skillslint --skills-dir <dir>  # adopter reach: structural +
//	                                               # conformance ONLY, over
//	                                               # <dir>/*/SKILL.md (repeatable)
//	make skillslint                           # the check form
//	make guardrail-sync                       # the --sync form
//
// Exit codes: 0 clean; 1 a real violation (a skill file breaks a rule, a house
// value is unresolved, or a guardrail copy has drifted); 2 the check itself
// could not run (bad root, no skills found, no plugin tree, unreadable declared
// guardrail source). 2 is could-not-check — it is a failure, never a quiet pass.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// stringList is a repeatable flag.Value: each --skills-dir on the command line
// appends rather than overwrites, so `--skills-dir a --skills-dir b` checks
// both directories in one run.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	root := flag.String("root", ".", "path to the repo root holding plugins/assay/skills/")
	sync := flag.Bool("sync", false, "regenerate every guardrail copy from "+guardrailSourcePath+" instead of checking")
	var skillsDirs stringList
	flag.Var(&skillsDirs, "skills-dir", "path to a directory of <dir>/*/SKILL.md to run ONLY the structural + conformance checks over (repeatable); when given, --root's other checks (house values, hidden chars, guardrails, enforcement block, posix-token) do not run")
	flag.Parse()

	if *sync {
		os.Exit(runSync(*root))
	}

	if len(skillsDirs) > 0 {
		os.Exit(runSkillsDirs([]string(skillsDirs)))
	}

	exit := 0

	checked, issues, err := LintSkills(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillslint: %v\n", err)
		os.Exit(2)
	}
	for _, is := range issues {
		fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", is.Path, is.Msg)
	}
	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "SKILLSLINT: FAIL — %d issue(s) across %d skill file(s)\n", len(issues), checked)
		exit = 1
	} else {
		fmt.Printf("SKILLSLINT: PASS — %d skill file(s) under %s, all frontmatter loads as a YAML mapping (name==dir, description a non-empty string) and no bare unforgeable/tamper-evident claims\n", checked, *root)
	}

	// Conformance soft budgets (advisory, never exit-affecting): per-skill body
	// bytes/lines/approx-tokens and the one bundle-wide summed-description-chars
	// line (conformance.go). The hard per-skill limits (description length,
	// name length/pattern) are exit-code-bearing and already folded into the
	// Issues LintSkills returned above — this half is the soft budgets only.
	cfChecked, cfNotices, cfErr := ConformanceNotices(*root)
	switch {
	case cfErr != nil:
		fmt.Fprintf(os.Stderr, "skillslint: %v\n", cfErr)
		fmt.Fprintf(os.Stderr, "CONFORMANCE-BUDGET: COULD-NOT-CHECK — the skill tree could not be read; a check that read nothing proved nothing (advisory: does not affect exit)\n")
	case len(cfNotices) > 0:
		for _, n := range cfNotices {
			fmt.Fprintln(os.Stderr, n)
		}
		fmt.Fprintf(os.Stderr, "CONFORMANCE-BUDGET: NOTICE — %d soft-budget notice(s) across %d skill file(s) (advisory: does not affect exit)\n", len(cfNotices), cfChecked)
	default:
		fmt.Printf("CONFORMANCE-BUDGET: PASS — %d skill file(s), no soft body/bundle budget crossed\n", cfChecked)
	}

	// Invisible-character / Trojan-Source lint + context-budget NOTICE over the
	// instruction surfaces (byte-level; hidden.go). The hidden-character half is a
	// HARD check — a bidi override, a zero-width splice, a stray control or an
	// invalid UTF-8 byte in a skill/instruction file is exit 1, because it is the
	// exact payload human review of the rendered text cannot see. The budget half
	// is ADVISORY: an over-budget file prints a NOTICE to stderr and never moves
	// the exit code, per the house convention for judgment-shaped checks.
	scChecked, scIssues, scNotices, scErr := ScanInstructionSurfaces(*root)
	for _, n := range scNotices {
		fmt.Fprintln(os.Stderr, n)
	}
	switch {
	case scErr != nil:
		fmt.Fprintf(os.Stderr, "skillslint: %v\n", scErr)
		fmt.Fprintf(os.Stderr, "HIDDEN-CHARS: COULD-NOT-CHECK — an instruction surface could not be walked; a check that read nothing proved nothing\n")
		if exit < 2 {
			exit = 2
		}
	case scChecked == 0:
		fmt.Fprintf(os.Stderr, "HIDDEN-CHARS: COULD-NOT-CHECK — 0 instruction surfaces found under %s; a check that read nothing proved nothing\n", *root)
		if exit < 2 {
			exit = 2
		}
	case len(scIssues) > 0:
		for _, is := range scIssues {
			fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", is.Path, is.Msg)
		}
		fmt.Fprintf(os.Stderr, "HIDDEN-CHARS: FAIL — %d invisible/hidden-character violation(s) across %d instruction file(s)\n", len(scIssues), scChecked)
		if exit < 1 {
			exit = 1
		}
	default:
		fmt.Printf("HIDDEN-CHARS: PASS — %d instruction file(s) scanned, no bidi/zero-width/control/invalid-UTF-8 payload\n", scChecked)
	}

	// The unresolved-house-value check reads the WHOLE plugin tree, not just the
	// skill homes above: a resolved house value in a reference file is as adopter-
	// facing as one in a SKILL.md (#236).
	hvChecked, hvIssues, hvErr := LintPluginTree(*root)
	switch {
	case hvErr != nil:
		fmt.Fprintf(os.Stderr, "skillslint: %v\n", hvErr)
		fmt.Fprintf(os.Stderr, "HOUSE-VALUES: COULD-NOT-CHECK — the plugin tree could not be read; a check that read nothing proved nothing\n")
		if exit < 2 {
			exit = 2
		}
	case len(hvIssues) > 0:
		for _, is := range hvIssues {
			fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", is.Path, is.Msg)
		}
		fmt.Fprintf(os.Stderr, "HOUSE-VALUES: FAIL — %d unresolved house value(s) across %d markdown file(s) under plugins/\n", len(hvIssues), hvChecked)
		if exit < 1 {
			exit = 1
		}
	default:
		fmt.Printf("HOUSE-VALUES: PASS — %d markdown file(s) under plugins/, no proper-name-shaped token in a driver position\n", hvChecked)
	}

	rep := CheckGuardrails(*root)
	for _, is := range rep.Unchecked {
		fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", is.Path, is.Msg)
	}
	for _, is := range rep.Failed {
		fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", is.Path, is.Msg)
	}
	switch {
	case len(rep.Unchecked) > 0:
		// could-not-check. Distinct exit code, and never printed as a pass.
		fmt.Fprintf(os.Stderr, "GUARDRAILS: COULD-NOT-CHECK — %d site(s) unreadable or unlocatable, %d compared, %d drifted\n",
			len(rep.Unchecked), rep.Compared, len(rep.Failed))
		if exit < 2 {
			exit = 2
		}
	case len(rep.Failed) > 0:
		fmt.Fprintf(os.Stderr, "GUARDRAILS: FAIL — %d copy/copies drifted from %s (%d compared). Fix the SOURCE and run `make guardrail-sync`; do not hand-edit the copy.\n",
			len(rep.Failed), guardrailSourcePath, rep.Compared)
		if exit < 1 {
			exit = 1
		}
	case rep.Compared == 0:
		fmt.Fprintf(os.Stderr, "GUARDRAILS: COULD-NOT-CHECK — 0 comparisons made; a check that compared nothing proved nothing\n")
		if exit < 2 {
			exit = 2
		}
	default:
		fmt.Printf("GUARDRAILS: PASS — %d guardrail copy/copies byte-match %s\n", rep.Compared, guardrailSourcePath)
	}

	// Enforcement-status block parity: the generated block in the authoring
	// guidance must byte-match `statusgen enforcement-status` (mistake-proofing/04).
	// This is the ONLY plugin-tree gate CI runs on pull requests, so the byte-diff
	// that binds the guidance copy to the lint's rule registry rides here. Derive
	// shells out to the statusgen emitter; a failure to derive is could-not-check,
	// never a quiet pass.
	erep := CheckEnforcementBlock(*root, func() (string, error) { return deriveEnforcementBlock(*root) })
	switch {
	case erep.Unchecked != nil:
		fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", erep.Unchecked.Path, erep.Unchecked.Msg)
		fmt.Fprintf(os.Stderr, "ENFORCEMENT-BLOCK: COULD-NOT-CHECK — the generated block could not be compared; a check that compared nothing proved nothing\n")
		if exit < 2 {
			exit = 2
		}
	case erep.Failed != nil:
		fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", erep.Failed.Path, erep.Failed.Msg)
		fmt.Fprintf(os.Stderr, "ENFORCEMENT-BLOCK: FAIL — the generated enforcement-status block drifted from the lint registry. Fix the registry in statusgen and run `go run ./tools/skillslint --sync`; do not hand-edit the block.\n")
		if exit < 1 {
			exit = 1
		}
	default:
		fmt.Printf("ENFORCEMENT-BLOCK: PASS — the generated block in %s byte-matches `statusgen enforcement-status`\n", enforcementSitePath)
	}

	// posix-token: advisory (never exit-affecting, per the lint-debt cadence a hard
	// row would need first — windows-port/12) NOTICE for a skill body that spells a
	// POSIX-only mktemp/tmp/config-home literal instead of naming the mechanism
	// (desk-shell.md §Scratch files / §Config home). See posixtoken.go.
	ptChecked, ptNotices, ptErr := PosixTokenIssues(*root)
	switch {
	case ptErr != nil:
		fmt.Fprintf(os.Stderr, "skillslint: %v\n", ptErr)
		fmt.Fprintf(os.Stderr, "POSIX-TOKEN: COULD-NOT-CHECK — the skill tree could not be read; a check that read nothing proved nothing (advisory: does not affect exit)\n")
	case len(ptNotices) > 0:
		for _, n := range ptNotices {
			fmt.Fprintf(os.Stderr, "skillslint: NOTICE: %s:%d: %s\n", n.Path, n.Line, n.Text)
		}
		fmt.Fprintf(os.Stderr, "POSIX-TOKEN: NOTICE — %d POSIX-only literal(s) across %d skill file(s) (advisory: does not affect exit)\n", len(ptNotices), ptChecked)
	default:
		fmt.Printf("POSIX-TOKEN: PASS — %d skill file(s), no POSIX-only mktemp/tmp/config-home literal outside a fenced unix-example block\n", ptChecked)
	}

	os.Exit(exit)
}

func runSync(root string) int {
	changed, rep, err := SyncGuardrails(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillslint --sync: %v\n", err)
		return 2
	}
	for _, is := range rep.Unchecked {
		fmt.Fprintf(os.Stderr, "skillslint --sync: %s: %s\n", is.Path, is.Msg)
	}
	for _, p := range changed {
		fmt.Printf("regenerated: %s\n", p)
	}
	if len(rep.Unchecked) > 0 {
		fmt.Fprintf(os.Stderr, "GUARDRAIL-SYNC: COULD-NOT-CHECK — %d site(s) not rewritten\n", len(rep.Unchecked))
		return 2
	}
	fmt.Printf("GUARDRAIL-SYNC: %d file(s) regenerated from %s\n", len(changed), guardrailSourcePath)

	// Also regenerate the enforcement-status block from the statusgen emitter
	// (mistake-proofing/04). Same regenerate-from-source discipline: the fix for a
	// drift finding is to change the registry and re-run this, never to hand-edit
	// the copy.
	echanged, erep := SyncEnforcementBlock(root, func() (string, error) { return deriveEnforcementBlock(root) })
	if erep.Unchecked != nil {
		fmt.Fprintf(os.Stderr, "skillslint --sync: %s: %s\n", erep.Unchecked.Path, erep.Unchecked.Msg)
		return 2
	}
	if echanged {
		fmt.Printf("regenerated: %s (enforcement-status block)\n", enforcementSitePath)
	}
	return 0
}

// runSkillsDirs is the --skills-dir adopter-reach entry point: it runs ONLY
// the structural + conformance checks (LintSkillsDir, conformance.go) over
// every given directory's <dir>/*/SKILL.md, plus the same checks' advisory
// soft-budget NOTICEs (ConformanceNoticesDir) — never the house-value,
// hidden-character, guardrail or enforcement-block halves, which check THIS
// repo's own fixed layout and do not apply to an adopter's directory.
//
// Every given directory is checked independently, and EVERY directory that
// matches zero files is reported by name and forces exit 2 — regardless of
// what any other directory in the same invocation found. A typo'd or moved
// path in a multi-directory run must never be silently dropped from the
// gate: README §1a promises "zero matched files is exit 2, never a quiet
// pass", and that promise binds per directory, not just to the union of all
// of them (#1663 SEC-1663-1 / F1).
func runSkillsDirs(dirs []string) int {
	var allIssues []Issue
	totalChecked := 0
	matchedAny := false
	anyUnmatched := false

	for _, d := range dirs {
		checked, issues, err := LintSkillsDir(d)
		if err != nil {
			anyUnmatched = true
			fmt.Fprintf(os.Stderr, "skillslint: %s: %v\n", d, err)
			continue
		}
		matchedAny = true
		totalChecked += checked
		allIssues = append(allIssues, issues...)
	}
	for _, is := range allIssues {
		fmt.Fprintf(os.Stderr, "skillslint: %s: %s\n", is.Path, is.Msg)
	}
	if anyUnmatched {
		fmt.Fprintf(os.Stderr, "SKILLSLINT: COULD-NOT-CHECK — at least one --skills-dir matched zero skill files; a check that read nothing proved nothing for that directory, whatever the others found\n")
		return 2
	}
	if !matchedAny {
		// Unreachable given the loop above (anyUnmatched would be true, so
		// !matchedAny already returned above), but kept as an explicit
		// belt-and-braces fail-closed default rather than falling through to
		// a PASS over zero directories.
		fmt.Fprintf(os.Stderr, "SKILLSLINT: COULD-NOT-CHECK — 0 skill file(s) matched under --skills-dir; a check that read nothing proved nothing\n")
		return 2
	}
	exit := 0
	if len(allIssues) > 0 {
		fmt.Fprintf(os.Stderr, "SKILLSLINT: FAIL — %d issue(s) across %d skill file(s)\n", len(allIssues), totalChecked)
		exit = 1
	} else {
		fmt.Printf("SKILLSLINT: PASS — %d skill file(s) under --skills-dir, structural + conformance checks clean\n", totalChecked)
	}

	for _, d := range dirs {
		cfChecked, cfNotices, cfErr := ConformanceNoticesDir(d)
		if cfErr != nil {
			// Already surfaced above via LintSkillsDir for this same directory
			// when it matched nothing — and an unmatched directory anywhere in
			// dirs already returned exit 2 above, so this branch only runs
			// when every directory matched. Kept for defensive symmetry with
			// LintSkillsDir's own error shape.
			continue
		}
		for _, n := range cfNotices {
			fmt.Fprintln(os.Stderr, n)
		}
		if len(cfNotices) > 0 {
			fmt.Fprintf(os.Stderr, "CONFORMANCE-BUDGET: NOTICE — %d soft-budget notice(s) across %d skill file(s) under %s (advisory: does not affect exit)\n", len(cfNotices), cfChecked, d)
		} else {
			fmt.Printf("CONFORMANCE-BUDGET: PASS — %d skill file(s) under %s, no soft body/bundle budget crossed\n", cfChecked, d)
		}
	}
	return exit
}
