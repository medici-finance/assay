// Command leaksweep is the final pre-publication leak sweep: a positive-control
// grep over a staged tree that gates the one-way copy into the public
// medici-finance/assay repo.
//
//	leaksweep run --tree DIR [--tokens PATH]
//
// It reports in three states per token (docs/three-state-instrument-rule.md):
// checked-clean, checked-failed, could-not-check. It exits 0 ONLY when every
// token is checked-clean; a leak or an entry that could not be checked exits
// non-zero and names the state and the reason. It never publishes anything,
// changes any repo setting, or deletes anything — its output informs a human's
// decision, it never takes it.
//
// The tool bans the word-boundary regex escape outright — in its own code, in
// the token file, and in every check — because that escape is the reason the
// sweep exists: in the engine `git grep` reaches on the reference machine it
// matches nothing and exits 1, which a checklist reads as "clean" over a dirty
// tree. Matching is literal-fixed or explicit character classes only.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultTokensPath = "docs/leak-sweep-tokens.yaml"

func usage() {
	fmt.Fprint(os.Stderr, `leaksweep — final pre-publication leak sweep (positive-control gate)

usage:
  leaksweep run --tree DIR [--tokens PATH]

run   Sweep the staged tree DIR for every withheld token in the token file
      (default `+defaultTokensPath+`). For each token: find its control first,
      then the token. Exit 0 only if EVERY entry is checked-clean. A leak
      (checked-failed) or an unchecked entry (could-not-check) exits non-zero.
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = runSweep(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "leaksweep: unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaksweep: %v\n", err)
		os.Exit(codeOf(err))
	}
}

// defaultEngines is the production engine set: both external scanners, both
// required to certify. `legacy` is available by name for the detection-parity
// evidence the human gate requires (Verify row 9) and for nothing else — it is
// staged for deletion, see DELETION-STAGED.md.
const defaultEngines = "gitleaks,trufflehog"

func runSweep(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	tree := fs.String("tree", "", "the staged tree to sweep (required)")
	tokens := fs.String("tokens", "", "token file (default "+defaultTokensPath+")")
	engineNames := fs.String("engines", defaultEngines,
		"comma-separated engines: gitleaks, trufflehog, legacy (parity evidence only)")
	gitleaksBin := fs.String("gitleaks-bin", "", "path to the pinned gitleaks binary (default: $LEAKSWEEP_GITLEAKS_BIN, then PATH)")
	trufflehogBin := fs.String("trufflehog-bin", "", "path to the pinned trufflehog binary (default: $LEAKSWEEP_TRUFFLEHOG_BIN, then PATH)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *tree == "" {
		return withExit(exitError, "run requires --tree DIR")
	}
	tokPath := *tokens
	if tokPath == "" {
		tokPath = defaultTokensPath
	}

	tl, err := loadTokens(tokPath)
	if err != nil {
		return err
	}
	// The legacy in-process engine is selected as a WHOLE run, not mixed in with
	// the external engines: it exists to produce the old engine's caught-set for
	// the parity diff, and blending the two sets would defeat the comparison.
	if strings.TrimSpace(*engineNames) == "legacy" {
		return runLegacy(tl, *tree)
	}
	engines, err := selectEngines(*engineNames, *gitleaksBin, *trufflehogBin)
	if err != nil {
		return err
	}
	c, err := buildCorpus(*tree)
	if err != nil {
		// A missing/unreadable tree is could-not-check — printed as such so the
		// output is never silent, then propagated as a non-zero exit.
		// The error line already carries the `could-not-check` marker (row 11
		// asserts the output says it); the summary uses the neutral count labels
		// so the marker is counted only where it means an entry state.
		fmt.Fprintf(os.Stdout, "%s\n", err)
		fmt.Fprintf(os.Stdout, "SUMMARY: entries=%d clean=0 failed=0 unchecked=%d files-swept=0 unreadable=0 unsearchable=0 non-regular=0 tree=%s\n",
			len(tl.Entries), len(tl.Entries), *tree)
		return err
	}
	out, err := orchestrate(tl, c, engines)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err)
		return err
	}
	out.provenance = readProvenance(*tree)
	out.write(os.Stdout)

	code := out.worstExit()
	if code == exitClean {
		return nil
	}
	abs, _ := filepath.Abs(*tree)
	return withExit(code, "sweep of %s did not certify — see the states above", abs)
}

// selectEngines builds the engine list from the flag. An unknown name is an
// ERROR, never a silently-skipped engine: a typo that quietly halved the engine
// set would leave a run that still prints a certificate.
func selectEngines(spec, gitleaksBin, trufflehogBin string) ([]engine, error) {
	var out []engine
	seen := map[string]bool{}
	for _, n := range strings.Split(spec, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if seen[n] {
			return nil, withExit(exitError, "engine %q listed twice", n)
		}
		seen[n] = true
		switch n {
		case "gitleaks":
			out = append(out, &gitleaksEngine{binOverride: gitleaksBin})
		case "trufflehog":
			out = append(out, &trufflehogEngine{binOverride: trufflehogBin})
		case "legacy":
			return nil, withExit(exitError,
				"engine %q cannot be combined with the external engines — run `--engines legacy` alone to produce the parity evidence", n)
		default:
			return nil, withExit(exitError, "unknown engine %q (want gitleaks, trufflehog, or legacy)", n)
		}
	}
	if len(out) == 0 {
		return nil, withExit(exitError, "no engines selected — a sweep with no engine is a gate that cannot fail")
	}
	return out, nil
}

// runLegacy is the retained in-process engine, kept ONLY so the human gate's
// detection-parity row can produce the old engine's caught-set from the same
// build as the new one. It is staged for deletion (DELETION-STAGED.md) and is
// never the default.
func runLegacy(tl *TokenList, tree string) error {
	c, err := buildCorpus(tree)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err)
		return err
	}
	rep := sweep(tl, c)
	rep.provenance = readProvenance(tree)
	fmt.Fprintln(os.Stdout, "ENGINE legacy: the in-process matcher, retained for parity evidence only (staged for deletion)")
	rep.write(os.Stdout)
	code := rep.worstExit()
	if code == exitClean {
		return nil
	}
	abs, _ := filepath.Abs(tree)
	return withExit(code, "sweep of %s did not certify — see the states above", abs)
}
