// Command skillslint runs four offline checks over the plugin tree:
//
//	structural       every plugins/assay/skills/*/SKILL.md (lint.go)
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
)

func main() {
	root := flag.String("root", ".", "path to the repo root holding plugins/assay/skills/")
	sync := flag.Bool("sync", false, "regenerate every guardrail copy from "+guardrailSourcePath+" instead of checking")
	flag.Parse()

	if *sync {
		os.Exit(runSync(*root))
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
		fmt.Printf("SKILLSLINT: PASS — %d skill file(s) under %s, all frontmatter valid (name==dir, description present) and no bare unforgeable/tamper-evident claims\n", checked, *root)
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
	return 0
}
