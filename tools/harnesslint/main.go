package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func usage() {
	fmt.Fprintf(os.Stderr, `harnesslint — neutrality + vocabulary-closure lint (harness-portability/04)

usage:
  harnesslint [--vocab <readme>] bodies   <skillsDir>
  harnesslint [--vocab <readme>] bindings <referencesDir>

  bodies    scan <skillsDir>/*/SKILL.md for banned harness tokens and for
            capability references outside the closed vocabulary.
  bindings  assert every capability resolves in every <referencesDir>/*.md and
            every skill has a degradation cell in each binding file.

  --vocab   path to the stream README carrying the closed capability block
            (default %q).

exit: 0 checked-clean · 1 checked-failed · 2 could-not-check / usage.
`, defaultReadmePath)
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("harnesslint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	vocabPath := fs.String("vocab", defaultReadmePath, "path to the stream README with the closed capability block")
	fs.Usage = usage
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	rest := fs.Args()
	if len(rest) != 2 {
		usage()
		return exitUsage
	}
	mode, dir := rest[0], rest[1]

	vocab, err := loadVocabulary(*vocabPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could-not-check: %v\n", err)
		return exitCannot
	}

	switch mode {
	case "bodies":
		banned, err := loadBanned()
		if err != nil {
			fmt.Fprintf(os.Stderr, "could-not-check: %v\n", err)
			return exitCannot
		}
		violations, err := checkBodies(dir, vocab, banned)
		return report("bodies", err, violations)
	case "bindings":
		skillsDir := filepath.Join(dir, "..", "skills")
		violations, err := checkBindings(dir, skillsDir, vocab)
		return report("bindings", err, violations)
	default:
		fmt.Fprintf(os.Stderr, "could-not-check: unknown mode %q (want bodies|bindings)\n", mode)
		usage()
		return exitUsage
	}
}

func report(mode string, err error, violations []string) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "could-not-check: %v\n", err)
		return exitCannot
	}
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, v)
		}
		fmt.Fprintf(os.Stderr, "checked-failed: %s — %d violation(s)\n", mode, len(violations))
		return exitFailed
	}
	fmt.Printf("checked-clean: %s — no violations\n", mode)
	return exitClean
}
