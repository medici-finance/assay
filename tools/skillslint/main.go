// Command skillslint validates the desk-role skill homes under
// plugins/assay/skills/ and enforces the shared-guardrail derive-or-diff
// contract. See lint.go and guardrail.go for what each half checks and why.
//
//	go run ./tools/skillslint                 # lint plugins/assay/skills/ under the cwd
//	go run ./tools/skillslint --root ..       # lint a sibling checkout's skills
//	go run ./tools/skillslint --sync          # REGENERATE every guardrail copy
//	make skillslint                           # the check form
//	make guardrail-sync                       # the --sync form
//
// Exit codes: 0 clean; 1 one or more skill files violate a rule OR a guardrail
// copy has drifted; 2 the check itself could not run (bad root, no skills found,
// unreadable declared guardrail source). 2 is could-not-check — it is a
// failure, never a quiet pass.
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
